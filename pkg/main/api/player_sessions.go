package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// What the player sends out: every stream goes through streamGuard, which
// shares player_max_bandwidth among all streams and lets an admin cancel
// them; the player reports its position (HandlePlayerProgress), which
// feeds the play history, resume positions and the Now Playing page.

// --- total bandwidth -------------------------------------------------------

// bandwidthLimiter hands out send time at a fixed byte rate, first come
// first served, so all streams share the rate.
type bandwidthLimiter struct {
	mu   sync.Mutex
	next time.Time
}

var playerBandwidth bandwidthLimiter

const (
	bandwidthBurst = 250 * time.Millisecond // unused rate a stream may catch up on
	bandwidthChunk = 64 << 10               // bytes sent per reservation
)

// playerBytesPerSecond is player_max_bandwidth in bytes per second (0:
// unlimited).
func playerBytesPerSecond() float64 {
	return float64(max(config.GetSettingsGeneral().PlayerMaxBandwidth, 0)) * 1e6 / 8
}

// wait blocks until n bytes may be sent.
func (l *bandwidthLimiter) wait(ctx context.Context, n int) error {
	rate := playerBytesPerSecond()
	if rate <= 0 {
		return nil
	}

	l.mu.Lock()

	now := time.Now()
	if l.next.Before(now.Add(-bandwidthBurst)) {
		l.next = now.Add(-bandwidthBurst)
	}

	at := l.next
	l.next = at.Add(time.Duration(float64(n) / rate * float64(time.Second)))
	l.mu.Unlock()

	d := time.Until(at)
	if d <= 0 {
		return nil
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// --- streams ---------------------------------------------------------------

// activeStream is a player response being sent.
type activeStream struct {
	user, kind, id string
	cancel         context.CancelFunc
	bytes          atomic.Int64
}

var (
	activeStreams sync.Map // *activeStream -> struct{}
	// blockedStreams are files an admin stopped for an account (user|kind|id
	// -> until): the player's automatic restarts don't bring them back.
	blockedStreams sync.Map
)

const streamBlockTime = 30 * time.Second

// streamWriter sends a stream within the shared bandwidth.
type streamWriter struct {
	gin.ResponseWriter
	ctx context.Context //nolint:containedctx // the stream's own context
	st  *activeStream
}

func (w *streamWriter) Write(b []byte) (int, error) {
	written := 0

	for len(b) > 0 {
		n := min(len(b), bandwidthChunk)
		if err := playerBandwidth.wait(w.ctx, n); err != nil {
			return written, err
		}

		m, err := w.ResponseWriter.Write(b[:n])
		written += m
		w.st.bytes.Add(int64(m))

		if err != nil {
			return written, err
		}

		b = b[n:]
	}

	return written, nil
}

func (w *streamWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func streamKey(user, kind, id string) string { return strings.ToLower(user) + "|" + kind + "|" + id }

// streamGuard registers the request as a stream of kind/id: its writes
// share the total bandwidth and an admin can cancel it. ok is false (and
// the reply sent) for a file an admin just stopped. Call done when the
// stream ends.
func streamGuard(c *gin.Context, kind, id string) (done func(), ok bool) {
	user := accessOf(c).Name

	if streamBlocked(user, kind, id) {
		c.String(http.StatusGone, t("stopped by an admin"))
		return nil, false
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	st := &activeStream{user: user, kind: kind, id: id, cancel: cancel}
	activeStreams.Store(st, struct{}{})

	c.Request = c.Request.WithContext(ctx)
	c.Writer = &streamWriter{ResponseWriter: c.Writer, ctx: ctx, st: st}

	return func() {
		activeStreams.Delete(st)
		cancel()
	}, true
}

// streamBlocked reports whether an admin stopped the file for the account
// a moment ago.
func streamBlocked(user, kind, id string) bool {
	until, blocked := blockedStreams.Load(streamKey(user, kind, id))
	if !blocked {
		return false
	}

	if time.Now().Before(until.(time.Time)) {
		return true
	}

	blockedStreams.Delete(streamKey(user, kind, id))

	return false
}

// streamCount is the number of streams being sent.
func streamCount() int {
	n := 0

	activeStreams.Range(func(k, _ any) bool {
		if k.(*activeStream).kind != "speedtest" {
			n++
		}

		return true
	})

	return n
}

// stopStreams cancels the account's streams of a file, its HLS
// conversions too, and keeps new ones away for a while.
func stopStreams(user, kind, id string) {
	blockedStreams.Store(streamKey(user, kind, id), time.Now().Add(streamBlockTime))

	activeStreams.Range(func(k, _ any) bool {
		if st := k.(*activeStream); strings.EqualFold(st.user, user) && st.kind == kind && st.id == id {
			st.cancel()
		}

		return true
	})

	hlsSessions.Range(func(k, v any) bool {
		if s := v.(*hlsSession); strings.EqualFold(s.owner, user) && s.kind == kind && s.id == id {
			hlsStop(k.(string))
		}

		return true
	})
}

// serverShareKbps is what one more stream gets of the total bandwidth now
// (0: unlimited).
func serverShareKbps() int {
	rate := playerBytesPerSecond()
	if rate <= 0 {
		return 0
	}

	return int(rate * 8 / 1000 / float64(streamCount()+1))
}

// --- now playing and history -----------------------------------------------

// nowPlaying is a player that reported recently.
type nowPlaying struct {
	SID      string
	User     string
	Kind     string
	ID       string
	Title    string
	State    string // start, play, pause, end
	Quality  string
	Client   string
	Position float64
	Duration float64
	Mode     string // direct, remux, transcode, hls
	Started  time.Time
	Updated  time.Time
	stopMsg  string // set by an admin: the player stops and shows it

	session   uint      // its play_sessions row (0 until counted)
	lastState string    // the state of the previous report
	lastAt    time.Time // when it came
	played    float64   // seconds played of the current file
	counted   bool      // the file played long enough to be recorded
}

// minPlayedShare is how much of a file has to be played before it counts
// as played for the account (history, Continue, watched, statistics):
// skipping through a shuffled list leaves no trace. minPlayedSeconds when
// the length is unknown.
const (
	minPlayedShare   = 0.05
	minPlayedSeconds = 30
)

// maxReportGap caps the time counted as played between two reports (they
// come every 10 s while playing).
const maxReportGap = 60 * time.Second

var (
	nowPlayingMu  sync.Mutex
	nowPlayingMap = map[string]*nowPlaying{}
	playerSIDRe   = regexp.MustCompile(`^[a-z0-9]{8,32}$`)
)

// nowPlayingTimeout: a player that didn't report for this long is gone.
const nowPlayingTimeout = 90 * time.Second

// nowPlayingList returns the active players, longest playing first.
func nowPlayingList() []nowPlaying {
	nowPlayingMu.Lock()
	defer nowPlayingMu.Unlock()

	out := make([]nowPlaying, 0, len(nowPlayingMap))

	for sid, p := range nowPlayingMap {
		if time.Since(p.Updated) > nowPlayingTimeout {
			delete(nowPlayingMap, sid)
			continue
		}

		out = append(out, *p)
	}

	slices.SortFunc(out, func(a, b nowPlaying) int { return a.Started.Compare(b.Started) })

	return out
}

// HandlePlayerProgress takes the player's position (/admin/library/play/progress:
// sid, kind, id, pos, dur, state, title, quality) and answers whether to
// stop (an admin stopped it, with a message).
func HandlePlayerProgress(c *gin.Context) {
	sid, kind, id := c.PostForm("sid"), c.PostForm("kind"), c.PostForm("id")
	state := c.PostForm("state")

	fileID, err := strconv.ParseUint(id, 10, 64)
	k := libKindByType(playLibKinds[kind])
	acc := accessOf(c)

	if !playerSIDRe.MatchString(sid) || err != nil || k == nil || !slices.Contains([]string{"start", "play", "pause", "end", "stop"}, state) ||
		!acc.allowsFile(k, &fileID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid progress"})
		return
	}

	pos, _ := strconv.ParseFloat(c.PostForm("pos"), 64)
	dur, _ := strconv.ParseFloat(c.PostForm("dur"), 64)

	stopMsg, err := recordPlayProgress(acc.Name, playReport{
		SID: sid, Kind: kind, FileID: fileID, State: state, Pos: pos, Dur: dur, Title: c.PostForm("title"),
		Mode: c.PostForm("mode"), Quality: c.PostForm("quality"), Client: c.ClientIP(),
	})
	if errors.Is(err, errForeignSID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid progress"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stop": stopMsg != "", "message": stopMsg})
}

// playReport is a player's report of where it is in a file: the Library
// player's and those of outside apps (Jellyfin, Subsonic).
type playReport struct {
	SID     string
	Kind    string // movie, episode, track, audiobook
	FileID  uint64
	State   string // start, play, pause, end, stop
	Pos     float64
	Dur     float64
	Title   string
	Mode    string
	Quality string
	Client  string
}

// errForeignSID: the player session belongs to another account.
var errForeignSID = errors.New("player session of another account")

// recordPlayProgress records a report of an account's player: Now Playing,
// the history and the statistics. It returns the message of an admin who
// stopped the player ("" to go on).
func recordPlayProgress(user string, r playReport) (string, error) {
	title := r.Title[:min(len(r.Title), 200)]
	id := strconv.FormatUint(r.FileID, 10)
	pos, dur, state := r.Pos, r.Dur, r.State

	nowPlayingMu.Lock()

	p := nowPlayingMap[r.SID]
	if p != nil && !strings.EqualFold(p.User, user) {
		nowPlayingMu.Unlock()
		return "", errForeignSID
	}

	if p == nil {
		p = &nowPlaying{SID: r.SID, User: user, Started: time.Now()}
		nowPlayingMap[r.SID] = p
	}

	stopMsg := p.stopMsg
	now := time.Now()

	// Time played of this file; a new file starts over.
	delta := 0.0
	if state == "start" || p.Kind != r.Kind || p.ID != id {
		p.Started, p.played, p.counted, p.session = now, 0, false, 0
	} else if p.lastState == "start" || p.lastState == "play" {
		delta = min(now.Sub(p.lastAt), maxReportGap).Seconds()
		p.played += delta
	}

	// The account's history and the statistics only from 5% played on (Now
	// Playing shows the player at once).
	enough := p.played >= minPlayedSeconds
	if dur > 0 {
		enough = p.played >= minPlayedShare*dur
	}

	newPlay := !p.counted && enough
	record := p.counted || newPlay

	// Playing on in an unfinished file continues that play: no new one.
	fid := uint(r.FileID)
	resumed := newPlay && database.ContinuesPlay(user, r.Kind, &fid)

	switch {
	case newPlay:
		p.counted = true
		p.session = database.StartPlaySession(user, r.Kind, fid, title, r.Mode, r.Quality, resumed)
		database.AddPlaySeconds(p.session, p.played, r.Mode)
	case record && delta > 0:
		database.AddPlaySeconds(p.session, delta, r.Mode)
	}

	p.lastState, p.lastAt = state, now
	p.Kind, p.ID, p.Title, p.State, p.Position, p.Duration = r.Kind, id, title, state, pos, dur
	p.Quality, p.Mode, p.Client, p.Updated = r.Quality, r.Mode, r.Client, now

	if state == "stop" || stopMsg != "" {
		delete(nowPlayingMap, r.SID)
	}

	nowPlayingMu.Unlock()

	if state == "end" && dur > 0 {
		pos = dur
	}

	if record {
		h := database.PlayHistory{Username: user, Kind: r.Kind, FileID: fid, Title: title, Position: pos, Duration: dur}
		if err := database.PlayProgress(&h, newPlay && !resumed); err != nil {
			return stopMsg, err
		}

		// Finishing the last part finishes the book, parts skipped or not.
		if r.Kind == "audiobook" && dur > 0 && pos/dur >= database.Watched {
			database.FinishAudiobook(user, &fid)
		}
	}

	return stopMsg, nil
}

// NowPlayingPage shows who plays what, with a stop button
// (/admin/nowplaying, admins).
func NowPlayingPage(c *gin.Context) {
	renderPageIn(c, "Now Playing", true, html.Div(html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-circle-play", t("Now Playing"), t("Who is playing what in the Library player. Stop ends the playback; the account sees your message.")),
		html.Div(html.ID("nowPlaying"), hx.Get("/api/admin/nowplaying/list"), hx.Trigger("load, every 5s"), hx.Swap("innerHTML")),
		// The play history and viewing stats of the Statistics page, loaded once.
		html.Div(html.Class("mt-4"), hx.Get("/api/admin/statistics/plays"), hx.Trigger("load"), hx.Swap("innerHTML"),
			html.P(html.Class("text-muted small"), gomponents.Text(t("Loading the play history..."))))))
}

// HandleNowPlayingList renders the active players and the latest plays.
func HandleNowPlayingList(c *gin.Context) {
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, renderComponentToString(renderNowPlaying(getCSRFToken(c), "")))
}

// HandleNowPlayingStop stops a player (/admin/nowplaying/stop: sid, message).
func HandleNowPlayingStop(c *gin.Context) {
	msg := strings.TrimSpace(c.PostForm("message"))
	if msg == "" {
		msg = t("Playback was stopped by an admin.")
	}

	nowPlayingMu.Lock()
	p := nowPlayingMap[c.PostForm("sid")]

	var stopped nowPlaying
	if p != nil {
		p.stopMsg = msg[:min(len(msg), 300)]
		stopped = *p
	}

	nowPlayingMu.Unlock()

	alert := renderAlert(t("That player is no longer playing."), "warning")
	if p != nil {
		stopStreams(stopped.User, stopped.Kind, stopped.ID)
		alert = renderAlert(tf("Stopped %s for %s.", stopped.Title, stopped.User), "success")
	}

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, renderComponentToString(renderNowPlaying(getCSRFToken(c), alert)))
}

func renderNowPlaying(csrf, alert string) gomponents.Node {
	players := nowPlayingList()
	rows := make([]gomponents.Node, 0, len(players))

	for i := range players {
		p := &players[i]

		pct := 0
		if p.Duration > 0 {
			pct = int(min(p.Position/p.Duration, 1) * 100)
		}

		state := map[string]string{"pause": t("Paused"), "end": t("Next file")}[p.State]
		if state == "" {
			state = t("Playing")
		}

		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(p.User)), html.Div(html.Class("small text-muted"), gomponents.Text(p.Client))),
			html.Td(gomponents.Text(p.Title), html.Div(html.Class("small text-muted"), gomponents.Text(p.Kind+" · "+orDash(p.Quality)+" · "+orDash(p.Mode)))),
			html.Td(html.Style("min-width:12rem"),
				html.Div(html.Class("progress"), html.Style("height:.5rem"),
					html.Div(html.Class("progress-bar"), html.Style("width:"+strconv.Itoa(pct)+"%"))),
				html.Div(html.Class("small text-muted"), gomponents.Text(fmtClock(p.Position)+" / "+fmtClock(p.Duration)+" · "+state))),
			html.Td(html.Class("small text-muted text-nowrap"), gomponents.Text(p.Started.Format("15:04"))),
			html.Td(html.Form(html.Class("d-flex gap-1"),
				hx.Post("/api/admin/nowplaying/stop"), hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#nowPlaying"), hx.Swap("innerHTML"),
				html.Input(html.Type("hidden"), html.Name("sid"), html.Value(p.SID)),
				html.Input(html.Type("text"), html.Class("form-control form-control-sm"), html.Name("message"),
					html.Placeholder(t("Message (optional)")), gomponents.Attr("maxlength", "300")),
				html.Button(html.Type("submit"), html.Class("btn btn-sm btn-danger text-nowrap"),
					html.I(html.Class("fa-solid fa-stop me-1")), gomponents.Text(t("Stop"))))),
		))
	}

	current := gomponents.Node(html.P(html.Class("text-muted mb-0"), gomponents.Text(t("Nobody is playing anything."))))
	if len(rows) > 0 {
		current = html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(html.Th(gomponents.Text(t("Account"))), html.Th(gomponents.Text(t("Playing"))),
				html.Th(gomponents.Text(t("Progress"))), html.Th(gomponents.Text(t("Since"))), html.Th())),
			html.TBody(gomponents.Group(rows))))
	}

	limit := "unlimited"
	if mbit := config.GetSettingsGeneral().PlayerMaxBandwidth; mbit > 0 {
		limit = strconv.Itoa(mbit) + " Mbit/s"
	}

	return html.Div(gomponents.Raw(alert),
		html.Div(html.Class("card mb-3"), html.Div(html.Class("card-body"), current)),
		html.P(html.Class("small text-muted"), gomponents.Text(strconv.Itoa(streamCount())+" streams sending · total bandwidth "+limit+
			" (player_max_bandwidth).")))
}

// fmtClock shows seconds as h:mm:ss or m:ss.
func fmtClock(s float64) string {
	t := int(max(s, 0))
	if t >= 3600 {
		return strconv.Itoa(t/3600) + ":" + twoDigits(t/60%60) + ":" + twoDigits(t%60)
	}

	return strconv.Itoa(t/60) + ":" + twoDigits(t%60)
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}

	return strconv.Itoa(n)
}

// --- settings --------------------------------------------------------------

const maxSettingsBytes = 64 << 10

// HandleAccountSettings stores the account's web UI settings, a JSON
// object (/admin/account/settings).
func HandleAccountSettings(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxSettingsBytes+1))

	var obj map[string]any
	if err != nil || len(body) > maxSettingsBytes || json.Unmarshal(body, &obj) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": t("settings must be a JSON object of at most 64 KB")})
		return
	}

	if err := database.SaveUserSettings(accName(accessOf(c)), string(body)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// accountSettingsJS is the account's settings as a JavaScript object
// literal for the page ("{}" when unset or broken).
func accountSettingsJS(acc *userAccess) string {
	s := database.GetUserSettings(accName(acc))

	var obj map[string]any
	if json.Unmarshal([]byte(s), &obj) != nil {
		return "{}"
	}

	// The JSON goes into a script element: no "</script>" may end it early.
	return strings.ReplaceAll(s, "</", `<\/`)
}
