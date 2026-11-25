package api

import (
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Outside apps and devices see the library through the protocols they
// speak: WebDAV (serve_webdav.go), the Jellyfin API (serve_jellyfin*.go),
// the Subsonic API (serve_subsonic.go), OPDS (serve_opds.go) and DLNA
// (serve_dlna*.go). Each is switched on in the general config (serve_*),
// logs in with the accounts of the web interface and shows the media
// groups of the account.

// defaultServerName is the name apps show without server_name.
const defaultServerName = "Media Downloader"

func serverName() string {
	if n := strings.TrimSpace(config.GetSettingsGeneral().ServerName); n != "" {
		return n
	}

	return defaultServerName
}

// serveEnabled answers 404 while the protocol is switched off.
func serveEnabled(on func(*config.GeneralConfig) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !on(config.GetSettingsGeneral()) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		c.Next()
	}
}

// clientLoginTTL is how long a checked login of an app is trusted: the
// apps send it with every request, and checking a password hash takes a
// moment.
const clientLoginTTL = 5 * time.Minute

type clientLogin struct {
	acc   *userAccess
	until time.Time
}

var clientLogins sync.Map // sha256(user, password) -> clientLogin

// The reasons an app's login is refused.
var (
	errClientLogin = errors.New("wrong username or password")
	// errClientNoApps: the login is right, but the account may not use apps
	// (the "apps" feature). Told apart so the app doesn't report a wrong
	// password.
	errClientNoApps = errors.New("this account may not use apps and devices: an admin can allow it under Users")
)

// clientAuthenticate checks the login of an app, with the web login's
// lockout after repeated failures; accounts without the apps feature are
// refused (errClientNoApps, only after the password matched).
func clientAuthenticate(c *gin.Context, user, password string) (*userAccess, error) {
	if user == "" {
		return nil, errClientLogin
	}

	key := sha256.Sum256([]byte(user + "\x00" + password))
	if v, ok := clientLogins.Load(key); ok {
		if l := v.(clientLogin); time.Now().Before(l.until) {
			return l.acc, nil
		}

		clientLogins.Delete(key)
	}

	ip, now := c.ClientIP(), time.Now()
	if loginLimiter.blockedFor(ip, now) > 0 {
		return nil, errClientLogin
	}

	acc, ok := authenticateUser(user, password)
	if !ok {
		loginLimiter.fail(ip, now)
		return nil, errClientLogin
	}

	if !acc.CanUseApps() {
		return nil, errClientNoApps
	}

	loginLimiter.reset(ip)
	clientLogins.Store(key, clientLogin{acc: acc, until: now.Add(clientLoginTTL)})

	return acc, nil
}

// setClientSession makes the app's account the request's, so the library
// helpers (accessOf, requestLanguage, streamGuard) work as for the web
// interface.
func setClientSession(c *gin.Context, acc *userAccess) {
	c.Set("session", &Session{UserID: acc.Name, Role: acc.Role, Groups: acc.Groups, Features: acc.Features})
}

// basicAuth logs in with HTTP Basic authentication (WebDAV, OPDS); it
// asks for the login and aborts without a valid one.
func basicAuth(realm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, password, ok := c.Request.BasicAuth()

		acc, err := clientAuthenticate(c, user, password)

		switch {
		case ok && errors.Is(err, errClientNoApps):
			// No new password prompt: the password was right.
			c.String(http.StatusForbidden, "Forbidden: "+err.Error())
			c.Abort()

			return
		case !ok || err != nil:
			c.Header("WWW-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
			c.AbortWithStatus(http.StatusUnauthorized)

			return
		}

		setClientSession(c, acc)
		c.Next()
	}
}

// groupsOf returns the media groups of the library types the account may
// see (all types without one).
func groupsOf(acc *userAccess, types ...string) []libGroup {
	var out []libGroup

	for _, g := range libraryGroups() {
		if (len(types) == 0 || slices.Contains(types, g.Kind.Type)) && acc.AllowsGroup(g.Name) {
			out = append(out, g)
		}
	}

	return out
}

// groupTitle is the name an app shows for a media group: the type with
// the group's own name ("Movies EN"), the type alone when the group is
// named after it.
func groupTitle(g *libGroup, lang string) string {
	label := trText(lang, g.Kind.Label)

	suffix := strings.TrimPrefix(g.Name, g.Kind.Prefix)
	if suffix == "" || strings.EqualFold(suffix, label) || strings.EqualFold(suffix, trText("en", g.Kind.Label)) {
		return label
	}

	return label + " " + suffix
}

// trText translates a text marked with t() into lang (the responses of the
// apps aren't translated by i18nMiddleware).
func trText(lang, s string) string {
	return string(translateOutput([]byte(s), lang))
}

// listsOf is " IN (?, ...)" over the lists of the groups and its
// arguments; a condition that matches nothing without lists.
func listsOf(groups []libGroup) (string, []any) {
	var args []any

	for i := range groups {
		for j := range groups[i].Lists {
			args = append(args, groups[i].Lists[j])
		}
	}

	if len(args) == 0 {
		return " IN (NULL)", nil
	}

	return " COLLATE NOCASE IN (?" + strings.Repeat(", ?", len(args)-1) + ")", args
}

// serveMediaFile sends a file of a play kind (movie, episode, track,
// audiobook) as it is, with range requests, as a stream of the account
// (bandwidth, Now Playing's stop button).
func serveMediaFile(c *gin.Context, kind string, fileID uint64) {
	k := libKindByType(playLibKinds[kind])
	if k == nil || !accessOf(c).allowsFile(k, &fileID) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	loc, _, ok := playFile(kind, strconv.FormatUint(fileID, 10))
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	done, ok := streamGuard(c, kind, strconv.FormatUint(fileID, 10))
	if !ok {
		return
	}
	defer done()

	if ct := mediaContentType(loc); ct != "" {
		c.Header("Content-Type", ct)
	}

	http.ServeFile(c.Writer, c.Request, loc)
}

// mediaTypes are the content types of the media files by extension (the
// system's mime.types often lacks them in slim containers).
var mediaTypes = map[string]string{
	".mkv": "video/x-matroska", ".mp4": "video/mp4", ".m4v": "video/mp4", ".avi": "video/x-msvideo", ".ts": "video/mp2t",
	".m2ts": "video/mp2t", ".mov": "video/quicktime", ".wmv": "video/x-ms-wmv", ".webm": "video/webm", ".mpg": "video/mpeg",
	".mpeg": "video/mpeg", ".flv": "video/x-flv", ".ogv": "video/ogg",
	".mp3": "audio/mpeg", ".flac": "audio/flac", ".m4a": "audio/mp4", ".m4b": "audio/mp4", ".aac": "audio/aac",
	".ogg": "audio/ogg", ".oga": "audio/ogg", ".opus": "audio/ogg", ".wav": "audio/wav", ".wma": "audio/x-ms-wma",
	".alac": "audio/mp4", ".aiff": "audio/aiff", ".aif": "audio/aiff", ".dsf": "audio/x-dsf", ".ape": "audio/x-ape",
	".epub": "application/epub+zip", ".pdf": "application/pdf", ".mobi": "application/x-mobipocket-ebook",
	".azw": "application/vnd.amazon.ebook", ".azw3": "application/vnd.amazon.ebook", ".cbz": "application/vnd.comicbook+zip",
	".cbr": "application/vnd.comicbook-rar", ".fb2": "application/x-fictionbook+xml", ".djvu": "image/vnd.djvu",
	".txt": "text/plain; charset=utf-8", ".rtf": "application/rtf", ".doc": "application/msword",
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif",
	".srt": "application/x-subrip", ".vtt": "text/vtt", ".ass": "text/x-ssa", ".ssa": "text/x-ssa", ".nfo": "text/xml",
}

// mediaContentType is the content type of a media file ("" unknown).
func mediaContentType(name string) string {
	return mediaTypes[strings.ToLower(filepath.Ext(strings.ReplaceAll(name, "\\", "/")))]
}

// fileTitle is a file's name without folder and extension: the title of
// a file without metadata.
func fileTitle(loc string) string {
	base := path.Base(strings.ReplaceAll(loc, "\\", "/"))
	return strings.TrimSuffix(base, path.Ext(base))
}

// fileSeconds is the length of a media file in seconds: from its files
// row, else from ffprobe (cached).
func fileSeconds(c *gin.Context, kind string, fileID uint64, loc string) float64 {
	var secs float64

	switch kind {
	case "movie":
		secs = database.Getdatarow[float64](false, "SELECT COALESCE(runtime_seconds, 0) FROM movie_files WHERE id = ?", &fileID)
	case "episode":
		secs = database.Getdatarow[float64](false, "SELECT COALESCE(runtime_seconds, 0) FROM serie_episode_files WHERE id = ?", &fileID)
	case "track":
		secs = database.Getdatarow[float64](false, "SELECT COALESCE(runtime_ms, 0) / 1000.0 FROM album_files WHERE id = ?", &fileID)
	case "audiobook":
		secs = database.Getdatarow[float64](false, "SELECT COALESCE(runtime_ms, 0) / 1000.0 FROM audiobook_files WHERE id = ?", &fileID)
	}

	if secs > 0 || loc == "" {
		return secs
	}

	if p, err := probeMedia(c.Request.Context(), loc); err == nil {
		return p.Duration
	}

	return 0
}

// localRequest reports whether a request comes straight from the local
// network: DLNA has no login. Requests through a reverse proxy (with
// forwarding headers) never count, whatever address they name.
func localRequest(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-Ip") != "" || r.Header.Get("Forwarded") != "" {
		return false
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}

	ip := net.ParseIP(host)

	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast())
}

// renderAccountApps shows the account the addresses of the apps' protocols
// that are on (My Account).
func renderAccountApps(c *gin.Context, acc *userAccess) gomponents.Node {
	g := config.GetSettingsGeneral()
	if !acc.CanUseApps() || !(g.ServeJellyfin || g.ServeWebDAV || g.ServeSubsonic || g.ServeOPDS || g.ServeDLNA) {
		return nil
	}

	base := "http://" + c.Request.Host
	if requestIsHTTPS(c) {
		base = "https://" + c.Request.Host
	}

	var rows []gomponents.Node

	row := func(on bool, icon, name, address, text string) {
		if !on {
			return
		}

		rows = append(rows, html.Li(html.Class("list-group-item"),
			html.Div(html.Class("fw-semibold"), html.I(html.Class("fa-solid "+icon+" me-2 text-primary")), gomponents.Text(name)),
			gomponents.If(address != "", html.Div(html.Class("my-1"), html.Code(html.Class("user-select-all"), gomponents.Text(address)))),
			html.Div(html.Class("small text-muted"), gomponents.Text(text))))
	}

	row(g.ServeJellyfin, "fa-tv", "Jellyfin", base+"/jellyfin",
		t("Infuse, Swiftfin, Findroid and the Jellyfin apps for TVs: add a Jellyfin server with this address and log in as here."))
	row(g.ServeWebDAV, "fa-folder-open", "WebDAV", base+"/dav",
		t("Infuse, nPlayer, Finder or Explorer: the folders of your media, read only. In VLC, open the address as a network stream: a folder plays as a playlist. Log in as here. Windows Explorer refuses files over 50 MB (FileSizeLimitInBytes of the WebClient service): play videos with VLC instead."))
	row(g.ServeSubsonic, "fa-music", "Subsonic", base+"/subsonic",
		t("play:Sub, Substreamer, Amperfy, Symfonium: your music. Switch on the plain password (legacy) login in the app."))
	row(g.ServeOPDS, "fa-book", "OPDS", base+"/opds", t("KOReader, Moon+ Reader, Librera, Thorium: your books. Log in as here."))
	row(g.ServeDLNA, "fa-network-wired", "DLNA", "",
		tf("TVs and players in the home network find \"%s\" by themselves, without a login.", serverName()))

	return html.Div(html.Class("card mt-4"), html.Style("max-width: 48rem"),
		html.Div(html.Class("card-body"),
			html.H5(html.Class("card-title"), gomponents.Text(t("Apps and devices"))),
			html.P(html.Class("card-text small text-muted"), gomponents.Text(t("Watch and listen to the library in other apps. Your watched state and playlists are the same there."))),
			html.Ul(html.Class("list-group"), gomponents.Group(rows))))
}
