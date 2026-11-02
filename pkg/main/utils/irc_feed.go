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

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// Sentinel errors for the IRC announce feed.
var (
	errIRCServerEmpty   = errors.New("irc_server is required for irc list type")
	errIRCNickEmpty     = errors.New("irc_nick is required for irc list type")
	errIRCChannelsEmpty = errors.New("irc_channels is required for irc list type")
	errIRCRegexEmpty    = errors.New("irc_announce_regex is required for irc list type")
	errIRCNoResponse    = errors.New("irc server stopped responding")
	errIRCNotRegistered = errors.New("irc registration not accepted in time")
	errIRCNickInUse     = errors.New("irc nick and alternatives in use")
)

const (
	// defaultIRCReadSeconds is how long the first run waits for initial
	// announcements after a session is created, when irc_read_seconds is unset.
	defaultIRCReadSeconds = 30
	// ircBufferMax caps the number of buffered announce lines per session so a
	// busy channel cannot grow memory without bound between drains.
	ircBufferMax = 5000
	// ircMaxAttempts is how many failed runs a drained announce line survives
	// before it is dropped instead of requeued, so a line that breaks the
	// import every time is not retried forever.
	ircMaxAttempts = 3
	// ircIdleStop closes a background session whose buffer has not been drained
	// for this long (e.g. the list was disabled or removed).
	ircIdleStop = time.Hour
	// ircReconnectMin / ircReconnectMax bound the reconnect backoff.
	ircReconnectMin = 5 * time.Second
	ircReconnectMax = 5 * time.Minute
	// ircDialTimeout bounds establishing a single connection (TCP and TLS).
	ircDialTimeout = 30 * time.Second
	// ircHealthyConn is how long a connection must have lasted for the next
	// reconnect to start again from ircReconnectMin.
	ircHealthyConn = ircReconnectMax
	// ircMaxNickRetries bounds the alternative nicks tried on a collision.
	ircMaxNickRetries = 3
)

var (
	// ircReadIdle is the per-read deadline; on expiry the session sends a
	// keepalive PING. A second expiry in a row means the PING went
	// unanswered and the connection is treated as dead.
	ircReadIdle = 90 * time.Second
	// ircRegisterTimeout bounds the wait for RPL_WELCOME after NICK/USER.
	ircRegisterTimeout = 2 * time.Minute
	// ircIdentifyTimeout bounds the wait for NickServ to confirm IDENTIFY
	// before the channels are joined anyway.
	ircIdentifyTimeout = 15 * time.Second
)

// ircSession is a long-lived background connection to an IRC network that
// continuously buffers matching announce lines. The poll-based Feeds() pipeline
// drains the buffer on each scheduled run.
type ircSession struct {
	key         string // registry key: media config + list name
	listName    string
	fingerprint string
	cancel      context.CancelFunc

	mu        sync.Mutex
	buffer    []string
	lastDrain time.Time
	// attempts counts failed runs per requeued line.
	attempts map[string]uint8
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

	// Keyed per media config: same-named lists in different configs must not
	// share (and drain) one session's buffer.
	key := cfgp.NamePrefix + "\x00" + cfglist.Name
	session, created := ensureIRCSession(key, cfglist.Name, cl)

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

	// The caller settles the lines (commitIRC/requeueIRC) once it knows
	// whether the import went through.
	d.ircKey, d.ircLines = key, lines

	parsed := false

	defer func() {
		if !parsed { // panicking: hand the lines back, charged as a failure
			d.requeueIRC(true)
		}
	}()

	seen := make(map[string]struct{}, len(lines))
	for idx := range lines {
		m := namedSubmatches(re, lines[idx])
		if len(m) == 0 {
			continue
		}

		d.addIRCMatch(cfgp.IsType, m, cfglist, seen)
	}

	parsed = true

	return nil
}

// commitIRC forgets the IRC announce lines this feed drained, once they were
// imported.
func (d *feedResults) commitIRC() {
	if d == nil || d.ircKey == "" {
		return
	}

	if s := ircSessionFor(d.ircKey); s != nil {
		s.commit(d.ircLines)
	}

	d.ircKey, d.ircLines = "", nil
}

// requeueIRC hands the IRC announce lines this feed drained back to their
// session for the next run. failed charges each line one attempt.
func (d *feedResults) requeueIRC(failed bool) {
	if d == nil || d.ircKey == "" {
		return
	}

	// Looked up again: a config change may have replaced the session since.
	if s := ircSessionFor(d.ircKey); s != nil {
		s.requeue(d.ircLines, failed)
	}

	d.ircKey, d.ircLines = "", nil
}

func ircSessionFor(key string) *ircSession {
	ircSessionsMu.Lock()
	defer ircSessionsMu.Unlock()

	return ircSessions[key]
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

	imdb, err := providers.TMDBMovieIMDbID(context.Background(), title, year)
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

	provider := providers.GetTMDB()
	if provider == nil {
		return 0
	}

	res, err := provider.SearchSeries(context.Background(), name, 0)
	if err != nil {
		return 0
	}

	best := 0
	for i := range res {
		r := &res[i]
		if !strings.EqualFold(r.Name, name) && !strings.EqualFold(r.OriginalName, name) {
			continue
		}

		if year > 0 && !r.FirstAirDate.IsZero() && r.FirstAirDate.Year() != year {
			continue
		}

		best = r.ID

		break
	}

	if best == 0 {
		return 0
	}

	return tmdbSerieTVDbID(best)
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

// ensureIRCSession returns the running background session for key, starting
// one when absent or when the connection-relevant config changed. The second
// return value reports whether a new session was created on this call.
func ensureIRCSession(key, listName string, cl *config.ListsConfig) (*ircSession, bool) {
	fp := ircFingerprint(cl)

	ircSessionsMu.Lock()
	defer ircSessionsMu.Unlock()

	if s := ircSessions[key]; s != nil {
		if s.fingerprint == fp {
			return s, false
		}

		s.cancel() // config changed: stop the old session and start fresh
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &ircSession{
		key:         key,
		listName:    listName,
		fingerprint: fp,
		cancel:      cancel,
		lastDrain:   time.Now(),
	}

	ircSessions[key] = s

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

		started := time.Now()

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

		// Only a connection that held up resets the backoff: one that is
		// accepted and dropped right away (ban, throttle) must keep backing off.
		if err == nil || time.Since(started) >= ircHealthyConn {
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
	if ircSessions[s.key] == s {
		delete(ircSessions, s.key)
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

	// A blocked read only notices ctx at its next deadline; close the
	// connection on cancel so a stopped or replaced session leaves at once
	// (and frees the nick for its successor).
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

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
	nick := cl.IRCNick
	nickRetries := 0
	registerBy := time.Now().Add(ircRegisterTimeout)
	idleReads := 0

	// identifyBy is set while waiting for NickServ to confirm IDENTIFY;
	// channels (often +r) are joined only after that, or once it expires.
	var identifyBy time.Time

	identified := func(ok bool, reason string) {
		identifyBy = time.Time{}

		if !ok {
			logger.Logtype("warn", 1).
				Str(logger.StrListname, s.listName).
				Str("reason", reason).
				Msg("irc NickServ identification not confirmed, joining channels anyway")
		}

		s.joinChannels(send, cl)
	}

	// pending holds a line cut by a read deadline; the rest follows on the
	// next read.
	var pending string

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

		readBy := time.Now().Add(ircReadIdle)
		if !identifyBy.IsZero() && identifyBy.Before(readBy) {
			readBy = identifyBy
		}

		_ = conn.SetReadDeadline(readBy)

		line, rerr := reader.ReadString('\n')
		if rerr != nil {
			pending += line

			if isTimeout(rerr) && ctx.Err() == nil && !identifyBy.IsZero() &&
				!time.Now().Before(identifyBy) {
				identified(false, "timeout")
				continue
			}

			// Idle read timeout: send a keepalive and keep listening. A failed
			// write, or a second timeout without any data since the PING, means
			// the connection is dead and we should reconnect.
			if isTimeout(rerr) && ctx.Err() == nil {
				idleReads++
				if idleReads > 1 {
					return errIRCNoResponse
				}

				if !joined && time.Now().After(registerBy) {
					return errIRCNotRegistered
				}

				if perr := send("PING :keepalive"); perr != nil {
					return perr
				}

				continue
			}

			return rerr
		}

		idleReads = 0
		line = strings.TrimRight(pending+line, "\r\n")
		pending = ""

		if line == "" {
			continue
		}

		if cmd, arg, ok := ircPing(line); ok {
			_ = send("PONG :" + cmd + arg)
			continue
		}

		if !identifyBy.IsZero() {
			if done, ok := ircIdentifyResult(line); done {
				identified(ok, line)
				continue
			}

			if !time.Now().Before(identifyBy) {
				identified(false, "timeout")
			}
		}

		if !joined {
			if ircIsWelcome(line) {
				if cl.IRCNickServPassword != "" {
					_ = send("PRIVMSG NickServ :IDENTIFY " + cl.IRCNickServPassword)
					identifyBy = time.Now().Add(ircIdentifyTimeout)
				} else {
					s.joinChannels(send, cl)
				}

				joined = true

				continue
			}

			if ircIsNickInUse(line) {
				if nickRetries >= ircMaxNickRetries {
					return errIRCNickInUse
				}

				nickRetries++
				nick += "_"

				if err := send("NICK " + nick); err != nil {
					return err
				}

				continue
			}

			if time.Now().After(registerBy) {
				return errIRCNotRegistered
			}
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

		s.appendLine(stripIRCFormatting(msg.text))
	}
}

// joinChannels requests invites and joins the channels once the server has
// accepted registration and, with a NickServ password, NickServ has confirmed
// the identification (or its wait expired).
func (*ircSession) joinChannels(send func(string) error, cl *config.ListsConfig) {
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
		delete(s.attempts, s.buffer[0])
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

// requeue puts lines back at the front of the buffer, ahead of anything that
// arrived meanwhile, keeping at most ircBufferMax lines (oldest dropped first,
// as in appendLine). With failed set, each line is charged one attempt and a
// line reaching ircMaxAttempts is dropped.
func (s *ircSession) requeue(lines []string, failed bool) {
	if len(lines) == 0 {
		return
	}

	s.mu.Lock()

	merged := make([]string, 0, len(lines)+len(s.buffer))
	dropped := 0

	for _, l := range lines {
		if failed {
			if s.attempts == nil {
				s.attempts = make(map[string]uint8)
			}

			s.attempts[l]++
			if s.attempts[l] >= ircMaxAttempts {
				delete(s.attempts, l)

				dropped++

				continue
			}
		}

		merged = append(merged, l)
	}

	merged = append(merged, s.buffer...)
	if over := len(merged) - ircBufferMax; over > 0 {
		for _, l := range merged[:over] {
			delete(s.attempts, l)
		}

		merged = merged[over:]
	}

	s.buffer = merged
	s.mu.Unlock()

	if dropped > 0 {
		logger.Logtype("warn", 1).
			Str(logger.StrListname, s.listName).
			Int("lines", dropped).
			Int("attempts", ircMaxAttempts).
			Msg("irc announce lines dropped after repeated import failures")
	}
}

// commit clears the failure count of lines that were imported.
func (s *ircSession) commit(lines []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.attempts) == 0 {
		return
	}

	for _, l := range lines {
		delete(s.attempts, l)
	}
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

	// Dialer.Deadline covers only the TCP connect; without a deadline here a
	// server that accepts but never answers the handshake hangs the session.
	_ = conn.SetDeadline(deadline)

	tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err = tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	_ = conn.SetDeadline(time.Time{})

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

// ircIdentifyResult classifies line as the answer to NickServ IDENTIFY: done
// is true for RPL_LOGGEDIN (900) / a NickServ NOTICE confirming the login (ok)
// and for ERR_NICKLOCKED / SASL failures / a NickServ rejection (not ok).
func ircIdentifyResult(line string) (done, ok bool) {
	if !strings.HasPrefix(line, ":") {
		return false, false
	}

	fields := strings.SplitN(line, " ", 4)
	if len(fields) < 2 {
		return false, false
	}

	switch fields[1] {
	case "900", "903":
		return true, true
	case "902", "904":
		return true, false
	case "NOTICE":
	default:
		return false, false
	}

	if !logger.HasPrefixI(fields[0], ":NickServ!") || len(fields) < 4 {
		return false, false
	}

	text := strings.ToLower(fields[3])
	for _, s := range []string{"you are now identified", "you are now logged in", "password accepted", "you are now recognized"} {
		if strings.Contains(text, s) {
			return true, true
		}
	}

	for _, s := range []string{"invalid password", "password incorrect", "incorrect password", "is not registered", "isn't registered", "not a registered"} {
		if strings.Contains(text, s) {
			return true, false
		}
	}

	return false, false
}

// ircIsNickInUse reports whether line rejects the requested nick
// (ERR_NICKNAMEINUSE 433 / ERR_NICKCOLLISION 436).
func ircIsNickInUse(line string) bool {
	if !strings.HasPrefix(line, ":") {
		return false
	}

	fields := strings.SplitN(line, " ", 3)

	return len(fields) >= 2 && (fields[1] == "433" || fields[1] == "436")
}

// stripIRCFormatting removes mIRC bold/colour/italic/underline/reverse/reset
// control codes, which announce bots commonly use and which would otherwise
// sit between the words an announce regex expects.
func stripIRCFormatting(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return r < 0x20 }) < 0 {
		return s
	}

	var b strings.Builder

	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case 0x02, 0x0F, 0x11, 0x16, 0x1D, 0x1E, 0x1F:
		case 0x03: // colour: \x03[fg[,bg]] with up to two digits each
			fg := ircColorDigits(s, i+1)
			i += fg

			if fg > 0 && i+2 < len(s) && s[i+1] == ',' && isASCIIDigit(s[i+2]) {
				i++
				i += ircColorDigits(s, i+1)
			}
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// ircColorDigits counts the colour-number digits (at most two) at s[from:].
func ircColorDigits(s string, from int) int {
	n := 0
	for n < 2 && from+n < len(s) && isASCIIDigit(s[from+n]) {
		n++
	}

	return n
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

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
