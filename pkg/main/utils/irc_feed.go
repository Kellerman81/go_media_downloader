package utils

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Sentinel errors for the IRC announce feed.
var (
	errIRCServerEmpty   = errors.New("irc_server is required for irc list type")
	errIRCNickEmpty     = errors.New("irc_nick is required for irc list type")
	errIRCChannelsEmpty = errors.New("irc_channels is required for irc list type")
	errIRCRegexEmpty    = errors.New("irc_announce_regex is required for irc list type")
)

const (
	// defaultIRCReadSeconds is how long the first run waits for initial
	// announcements after a session is created, when irc_read_seconds is unset.
	defaultIRCReadSeconds = 30
	// ircBufferMax caps the number of buffered announce lines per session so a
	// busy channel cannot grow memory without bound between drains.
	ircBufferMax = 5000
	// ircIdleStop closes a background session whose buffer has not been drained
	// for this long (e.g. the list was disabled or removed).
	ircIdleStop = time.Hour
	// ircReconnectMin / ircReconnectMax bound the reconnect backoff.
	ircReconnectMin = 5 * time.Second
	ircReconnectMax = 5 * time.Minute
	// ircDialTimeout bounds establishing a single connection.
	ircDialTimeout = 30 * time.Second
	// ircReadIdle is the per-read deadline; on expiry the session sends a
	// keepalive PING and re-checks for shutdown.
	ircReadIdle = 90 * time.Second
)

// ircSession is a long-lived background connection to an IRC network that
// continuously buffers matching announce lines. The poll-based Feeds() pipeline
// drains the buffer on each scheduled run.
type ircSession struct {
	listName    string
	fingerprint string
	cancel      context.CancelFunc

	mu        sync.Mutex
	buffer    []string
	lastDrain time.Time
}

var (
	ircSessions   = make(map[string]*ircSession)
	ircSessionsMu sync.Mutex
)

// getirc ensures a background IRC session for the list, drains the
// announcements buffered since the last run, parses each with the configured
// regexp and appends entries to the appropriate feedResults slice based on the
// parent config's media type:
//   - movies:           IMDB id (from the announce or resolved via TMDB) -> d.Movies
//   - series:           name + tvdb id (resolved via TMDB when absent)   -> d.Series
//   - music:            album + artist                                   -> d.Albums
//   - audiobooks/books: title + author                                   -> d.Audiobooks / d.Books
func (d *feedResults) getirc(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	cfglist *config.MediaListsConfig,
) error {
	cl := cfglist.CfgList
	switch {
	case cl.IRCServer == "":
		return errIRCServerEmpty
	case cl.IRCNick == "":
		return errIRCNickEmpty
	case len(cl.IRCChannels) == 0:
		return errIRCChannelsEmpty
	case cl.IRCAnnounceRegex == "":
		return errIRCRegexEmpty
	}

	re, err := regexp.Compile(cl.IRCAnnounceRegex)
	if err != nil {
		return errors.New("invalid irc_announce_regex: " + err.Error())
	}

	session, created := ensureIRCSession(cfglist.Name, cl)

	// On the very first run the connection was just opened, so give it a brief
	// window to receive initial announcements before draining.
	lines := session.drain()
	if created && len(lines) == 0 {
		readSeconds := cl.IRCReadSeconds
		if readSeconds <= 0 {
			readSeconds = defaultIRCReadSeconds
		}

		lines = session.waitDrain(ctx, time.Duration(readSeconds)*time.Second)
	}

	logger.Logtype("info", 2).
		Str(logger.StrListname, cfglist.Name).
		Int("lines", len(lines)).
		Msg("irc announce lines collected")

	seen := make(map[string]struct{}, len(lines))
	for idx := range lines {
		m := namedSubmatches(re, lines[idx])
		if len(m) == 0 {
			continue
		}

		d.addIRCMatch(cfgp.IsType, m, cfglist, seen)
	}

	return nil
}

// addIRCMatch dispatches a single parsed announcement to the correct
// feedResults slice based on media type. seen deduplicates entries within a run.
func (d *feedResults) addIRCMatch(
	mediaType uint,
	m map[string]string,
	cfglist *config.MediaListsConfig,
	seen map[string]struct{},
) {
	title := strings.TrimSpace(m["title"])
	year, _ := strconv.Atoi(strings.TrimSpace(m["year"]))

	switch mediaType {
	case config.MediaTypeMovie:
		imdb := normaliseImdbID(m["imdb"])
		if imdb == "" {
			imdb = resolveMovieImdb(title, year)
		}

		if imdb == "" {
			return
		}

		if _, ok := seen[imdb]; ok {
			return
		}

		seen[imdb] = struct{}{}
		checkaddimdbfeed(&imdb, cfglist, d)

	case config.MediaTypeSeries:
		if title == "" {
			return
		}

		if _, ok := seen[title]; ok {
			return
		}

		seen[title] = struct{}{}

		tvdb, _ := strconv.Atoi(strings.TrimSpace(m["tvdb"]))
		if tvdb == 0 {
			tvdb = resolveSeriesTvdb(title, year)
		}

		d.Series = append(d.Series, config.ManualConfig{Name: title, TvdbID: tvdb})

	case config.MediaTypeMusic:
		artist := strings.TrimSpace(m["artist"])
		if title == "" || seenAdd(seen, artist+"\x00"+title) {
			return
		}

		d.Albums = append(d.Albums, config.ManualConfig{Name: title, ArtistName: artist})

	case config.MediaTypeAudiobook:
		author := strings.TrimSpace(m["author"])
		if title == "" || seenAdd(seen, author+"\x00"+title) {
			return
		}

		d.Audiobooks = append(d.Audiobooks, config.ManualConfig{Name: title, AuthorName: author})

	case config.MediaTypeBook:
		author := strings.TrimSpace(m["author"])
		if title == "" || seenAdd(seen, author+"\x00"+title) {
			return
		}

		d.Books = append(d.Books, config.ManualConfig{Name: title, AuthorName: author})
	}
}

// seenAdd reports whether key was already present, recording it otherwise.
func seenAdd(seen map[string]struct{}, key string) bool {
	if _, ok := seen[key]; ok {
		return true
	}

	seen[key] = struct{}{}

	return false
}

// resolveMovieImdb resolves a release title (and optional year) to an IMDB id
// via TMDB. Returns "" when no confident match is found.
func resolveMovieImdb(title string, year int) string {
	if title == "" {
		return ""
	}

	imdb, err := apiexternal.SearchTMDBMovieImdbID(title, year)
	if err != nil || imdb == "" {
		return ""
	}

	return imdb
}

// resolveSeriesTvdb resolves a series name (and optional year) to a TVDB id via
// TMDB. Returns 0 when no confident name match is found, in which case the
// series is still monitored by name only.
func resolveSeriesTvdb(name string, year int) int {
	if name == "" {
		return 0
	}

	res, err := apiexternal.SearchTmdbTV(name)
	if err != nil || res == nil {
		return 0
	}

	best := 0
	for i := range res.Results {
		r := &res.Results[i]
		if !strings.EqualFold(r.Name, name) && !strings.EqualFold(r.OriginalName, name) {
			continue
		}

		if year > 0 && len(r.FirstAirDate) >= 4 {
			if y, _ := strconv.Atoi(r.FirstAirDate[:4]); y != year {
				continue
			}
		}

		best = r.ID

		break
	}

	if best == 0 {
		return 0
	}

	ext, err := apiexternal.GetTVExternal(best)
	if err != nil || ext == nil {
		return 0
	}

	return ext.TvdbID
}

// normaliseImdbID trims whitespace and ensures a leading "tt" on a numeric IMDB id.
func normaliseImdbID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		return ""
	}

	if !strings.HasPrefix(id, "tt") {
		id = "tt" + id
	}

	return id
}

// namedSubmatches runs re against s and returns a map of named capture group
// to value. Returns nil when the line does not match.
func namedSubmatches(re *regexp.Regexp, s string) map[string]string {
	match := re.FindStringSubmatch(s)
	if match == nil {
		return nil
	}

	names := re.SubexpNames()

	out := make(map[string]string, len(names))
	for i, name := range names {
		if name != "" && i < len(match) {
			out[name] = match[i]
		}
	}

	return out
}

// ensureIRCSession returns the running background session for listName, starting
// one when absent or when the connection-relevant config changed. The second
// return value reports whether a new session was created on this call.
func ensureIRCSession(listName string, cl *config.ListsConfig) (*ircSession, bool) {
	fp := ircFingerprint(cl)

	ircSessionsMu.Lock()
	defer ircSessionsMu.Unlock()

	if s := ircSessions[listName]; s != nil {
		if s.fingerprint == fp {
			return s, false
		}

		s.cancel() // config changed: stop the old session and start fresh
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &ircSession{
		listName:    listName,
		fingerprint: fp,
		cancel:      cancel,
		lastDrain:   time.Now(),
	}

	ircSessions[listName] = s

	clCopy := *cl // snapshot so later config edits don't race the goroutine
	go s.run(ctx, &clCopy)

	return s, true
}

// ircFingerprint captures the connection-relevant settings so a changed config
// restarts the session.
func ircFingerprint(cl *config.ListsConfig) string {
	return strings.Join([]string{
		cl.IRCServer,
		strconv.FormatBool(cl.IRCUseTLS),
		cl.IRCNick,
		cl.IRCNickServPassword,
		cl.IRCInviteCommand,
		strings.Join(cl.IRCChannels, ","),
		strings.Join(cl.IRCAnnounceNicks, ","),
	}, "\x00")
}

// run is the background loop: connect, listen and buffer until the connection
// drops, then reconnect with capped exponential backoff. It self-terminates if
// the buffer has not been drained for ircIdleStop (list disabled/removed).
func (s *ircSession) run(ctx context.Context, cl *config.ListsConfig) {
	backoff := ircReconnectMin
	for {
		if ctx.Err() != nil {
			s.remove()
			return
		}

		err := s.connectAndListen(ctx, cl)
		if ctx.Err() != nil {
			s.remove()
			return
		}

		if err != nil {
			logger.Logtype("debug", 2).
				Str(logger.StrListname, s.listName).
				Err(err).
				Msg("irc connection dropped, will reconnect")
		}

		s.mu.Lock()
		idle := time.Since(s.lastDrain)
		s.mu.Unlock()

		if idle > ircIdleStop {
			logger.Logtype("info", 1).
				Str(logger.StrListname, s.listName).
				Msg("irc session idle, stopping")
			s.cancel()
			s.remove()

			return
		}

		if err == nil {
			backoff = ircReconnectMin
		}

		select {
		case <-ctx.Done():
			s.remove()
			return

		case <-time.After(backoff):
		}

		backoff = min(backoff*2, ircReconnectMax)
	}
}

// remove detaches this session from the registry (only if still the active one).
func (s *ircSession) remove() {
	ircSessionsMu.Lock()
	if ircSessions[s.listName] == s {
		delete(ircSessions, s.listName)
	}

	ircSessionsMu.Unlock()
}

// connectAndListen opens one connection, authenticates, joins the channels and
// buffers matching announce lines until the connection drops or ctx is done.
func (s *ircSession) connectAndListen(ctx context.Context, cl *config.ListsConfig) error {
	conn, err := dialIRC(ctx, cl.IRCServer, cl.IRCUseTLS, time.Now().Add(ircDialTimeout))
	if err != nil {
		return err
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	send := func(msg string) error {
		if _, werr := writer.WriteString(msg + "\r\n"); werr != nil {
			return werr
		}

		return writer.Flush()
	}

	if err = ircRegister(send, cl); err != nil {
		return err
	}

	announceNicks := lowerSet(cl.IRCAnnounceNicks)
	channels := lowerSet(cl.IRCChannels)
	joined := false

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Self-terminate a live connection whose buffer is no longer drained
		// (the list was disabled or removed).
		s.mu.Lock()
		idle := time.Since(s.lastDrain)
		s.mu.Unlock()

		if idle > ircIdleStop {
			s.cancel()
			return nil
		}

		_ = conn.SetReadDeadline(time.Now().Add(ircReadIdle))

		line, rerr := reader.ReadString('\n')
		if rerr != nil {
			// Idle read timeout: send a keepalive and keep listening; a failed
			// write means the connection is dead and we should reconnect.
			if isTimeout(rerr) && ctx.Err() == nil {
				if perr := send("PING :keepalive"); perr != nil {
					return perr
				}

				continue
			}

			return rerr
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		if cmd, arg, ok := ircPing(line); ok {
			_ = send("PONG :" + cmd + arg)
			continue
		}

		if !joined && ircIsWelcome(line) {
			s.onWelcome(send, cl)

			joined = true

			continue
		}

		msg, ok := ircPrivmsg(line)
		if !ok {
			continue
		}

		if _, want := channels[strings.ToLower(msg.channel)]; !want {
			continue
		}

		if len(announceNicks) > 0 {
			if _, want := announceNicks[strings.ToLower(msg.nick)]; !want {
				continue
			}
		}

		s.appendLine(msg.text)
	}
}

// onWelcome authenticates, requests invites and joins channels after the server
// has accepted registration (RPL_WELCOME).
func (*ircSession) onWelcome(send func(string) error, cl *config.ListsConfig) {
	if cl.IRCNickServPassword != "" {
		_ = send("PRIVMSG NickServ :IDENTIFY " + cl.IRCNickServPassword)
	}

	if cl.IRCInviteCommand != "" {
		_ = send(cl.IRCInviteCommand)
	}

	for i := range cl.IRCChannels {
		_ = send("JOIN " + cl.IRCChannels[i])
	}
}

// appendLine buffers one announce line, dropping the oldest entries when the
// buffer is full.
func (s *ircSession) appendLine(text string) {
	s.mu.Lock()
	if len(s.buffer) >= ircBufferMax {
		copy(s.buffer, s.buffer[1:])

		s.buffer = s.buffer[:ircBufferMax-1]
	}

	s.buffer = append(s.buffer, text)
	s.mu.Unlock()
}

// drain returns and clears the buffered announce lines.
func (s *ircSession) drain() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastDrain = time.Now()
	if len(s.buffer) == 0 {
		return nil
	}

	out := s.buffer

	s.buffer = nil

	return out
}

// waitDrain polls the buffer up to window, returning as soon as lines arrive.
func (s *ircSession) waitDrain(ctx context.Context, window time.Duration) []string {
	deadline := time.Now().Add(window)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return s.drain()
		case <-ticker.C:
			s.mu.Lock()
			n := len(s.buffer)
			s.mu.Unlock()

			if n > 0 {
				return s.drain()
			}
		}
	}

	return s.drain()
}

// dialIRC establishes a TCP or TLS connection honouring ctx and the deadline.
func dialIRC(
	ctx context.Context,
	server string,
	useTLS bool,
	deadline time.Time,
) (net.Conn, error) {
	dialer := &net.Dialer{Deadline: deadline}

	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, err
	}

	if !useTLS {
		return conn, nil
	}

	host, _, splitErr := net.SplitHostPort(server)
	if splitErr != nil {
		host = server
	}

	tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err = tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	return tlsConn, nil
}

// ircRegister performs the initial NICK/USER handshake.
func ircRegister(send func(string) error, cl *config.ListsConfig) error {
	if err := send("NICK " + cl.IRCNick); err != nil {
		return err
	}

	return send("USER " + cl.IRCNick + " 0 * :" + cl.IRCNick)
}

// ircPing reports whether line is a server PING and returns the token to echo.
func ircPing(line string) (prefix, token string, ok bool) {
	if !strings.HasPrefix(line, "PING") {
		return "", "", false
	}

	if idx := strings.IndexByte(line, ':'); idx >= 0 {
		return "", line[idx+1:], true
	}

	return strings.TrimSpace(strings.TrimPrefix(line, "PING")), "", true
}

// ircIsWelcome reports whether line is the RPL_WELCOME (001) numeric.
func ircIsWelcome(line string) bool {
	// Format: ":server 001 nick :Welcome..."
	if !strings.HasPrefix(line, ":") {
		return false
	}

	fields := strings.SplitN(line, " ", 3)

	return len(fields) >= 2 && fields[1] == "001"
}

// ircMessage holds the parsed parts of a PRIVMSG line.
type ircMessage struct {
	nick    string
	channel string
	text    string
}

// ircPrivmsg parses a PRIVMSG line into sender nick, target channel and text.
func ircPrivmsg(line string) (ircMessage, bool) {
	if !strings.HasPrefix(line, ":") {
		return ircMessage{}, false
	}

	// ":nick!user@host PRIVMSG #channel :message text"
	space := strings.IndexByte(line, ' ')
	if space < 0 {
		return ircMessage{}, false
	}

	prefix := line[1:space]
	rest := line[space+1:]

	if !strings.HasPrefix(rest, "PRIVMSG ") {
		return ircMessage{}, false
	}

	rest = strings.TrimPrefix(rest, "PRIVMSG ")

	target, text, found := strings.Cut(rest, " :")
	if !found {
		return ircMessage{}, false
	}

	nick := prefix
	if bang := strings.IndexByte(prefix, '!'); bang >= 0 {
		nick = prefix[:bang]
	}

	return ircMessage{nick: nick, channel: strings.TrimSpace(target), text: text}, true
}

// isTimeout reports whether err is a network timeout (idle read deadline).
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// lowerSet builds a lowercased lookup set from a slice, skipping empty entries.
func lowerSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for i := range values {
		v := strings.ToLower(strings.TrimSpace(values[i]))
		if v != "" {
			out[v] = struct{}{}
		}
	}

	return out
}

// StopAllIRCSessions cancels every running IRC background session. Intended for
// graceful application shutdown.
func StopAllIRCSessions() {
	ircSessionsMu.Lock()

	sessions := make([]*ircSession, 0, len(ircSessions))
	for _, s := range ircSessions {
		sessions = append(sessions, s)
	}

	ircSessionsMu.Unlock()

	for _, s := range sessions {
		s.cancel()
	}
}
