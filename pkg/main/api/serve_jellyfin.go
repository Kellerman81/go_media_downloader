package api

import (
	"crypto/sha1" //nolint:gosec // only to shorten IDs
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
)

// The Jellyfin API (/jellyfin) lets Jellyfin apps (Infuse, Swiftfin,
// Findroid, the TV apps) browse and play the library: the part of the API
// these apps use to log in, list libraries and items, show images, play
// files as they are (no transcoding) and report progress, which feeds the
// account's history and Now Playing like the Library player. The items
// are in serve_jellyfin_items.go.

// jfVersion is the Jellyfin version the API answers as.
const jfVersion = "10.11.11"

// jfTokenDuration is how long a login of an app is valid; a new password
// ends it earlier.
const jfTokenDuration = 365 * 24 * time.Hour

// jfRoute is an API path: lower-case segments, "{name}" takes a segment,
// a final "*" any further ones.
type jfRoute struct {
	method  string // "" any
	pattern []string
	public  bool
	h       func(c *gin.Context, p map[string]string)
}

var jfRoutes []jfRoute

func jfRoute2(method, pattern string, public bool, h func(c *gin.Context, p map[string]string)) {
	jfRoutes = append(jfRoutes, jfRoute{method: method, pattern: strings.Split(strings.ToLower(pattern), "/"), public: public, h: h})
}

// match returns the captured segments when the route takes the path.
func (r *jfRoute) match(method string, segs, orig []string) (map[string]string, bool) {
	if r.method != "" && r.method != method && (r.method != http.MethodGet || method != http.MethodHead) {
		return nil, false
	}

	p := map[string]string{}

	for i, s := range r.pattern {
		if s == "*" {
			return p, true
		}

		if i >= len(segs) {
			return nil, false
		}

		if strings.HasPrefix(s, "{") {
			p[strings.Trim(s, "{}")] = orig[i]
			continue
		}

		if s != segs[i] {
			return nil, false
		}
	}

	return p, len(segs) == len(r.pattern)
}

// HandleJellyfin dispatches a Jellyfin API request (/jellyfin/*path).
func HandleJellyfin(c *gin.Context) {
	orig := strings.Split(strings.Trim(c.Param("path"), "/"), "/")
	if len(orig) > 0 && strings.EqualFold(orig[0], "emby") {
		orig = orig[1:]
	}

	segs := make([]string, len(orig))
	for i, s := range orig {
		segs[i] = strings.ToLower(s)

		// stream.mkv, Stream.srt: the container doesn't matter.
		if base, _, ok := strings.Cut(segs[i], "."); ok && i == len(orig)-1 && (base == "stream" || base == "css") {
			segs[i], orig[i] = base, base
		}
	}

	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusNoContent)
		return
	}

	for i := range jfRoutes {
		p, ok := jfRoutes[i].match(c.Request.Method, segs, orig)
		if !ok {
			continue
		}

		if !jfRoutes[i].public && !jfAuth(c) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		jfRoutes[i].h(c, p)

		return
	}

	logger.Logtype("debug", 0).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Msg("Jellyfin API call not supported")
	c.AbortWithStatus(http.StatusNotFound)
}

// --- login ------------------------------------------------------------------

// jfClient is the app's description of itself (the Authorization header).
type jfClient struct {
	Client, Device, DeviceID, Version, Token string
}

// jfClientOf reads the Authorization (or X-Emby-Authorization) header:
// MediaBrowser Client="Infuse", Device="iPhone", DeviceId="...",
// Version="7.8", Token="...".
func jfClientOf(c *gin.Context) jfClient {
	h := c.GetHeader("Authorization")
	if !strings.Contains(strings.ToLower(h), "mediabrowser") && !strings.Contains(strings.ToLower(h), "emby") {
		h = c.GetHeader("X-Emby-Authorization")
	}

	var cl jfClient

	if _, rest, ok := strings.Cut(h, " "); ok {
		for part := range strings.SplitSeq(rest, ",") {
			k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
			if !ok {
				continue
			}

			v, _ = url.QueryUnescape(strings.Trim(strings.TrimSpace(v), `"`))

			switch strings.ToLower(k) {
			case "client":
				cl.Client = v
			case "device":
				cl.Device = v
			case "deviceid":
				cl.DeviceID = v
			case "version":
				cl.Version = v
			case "token":
				cl.Token = v
			}
		}
	}

	for _, t := range []string{c.GetHeader("X-Emby-Token"), c.GetHeader("X-MediaBrowser-Token"), jfParam(c, "api_key"), jfParam(c, "apikey")} {
		if cl.Token == "" {
			cl.Token = t
		}
	}

	if cl.DeviceID == "" {
		cl.DeviceID = c.GetHeader("X-Emby-Device-Id")
	}

	return cl
}

// jfTokenKey is the signing key of an account's app logins: it changes
// with the password like the remember-me key.
func jfTokenKey(user string) ([]byte, bool) {
	key, ok := rememberKeyFor(user)
	if !ok {
		return nil, false
	}

	return rememberMAC(key, "jellyfin-token"), true
}

type jfTokenEntry struct {
	acc   *userAccess
	until time.Time
}

var jfTokens sync.Map // token -> jfTokenEntry

// jfAuth checks the app's token and makes its account the request's.
func jfAuth(c *gin.Context) bool {
	acc := jfTokenAccess(jfClientOf(c).Token)
	if acc == nil {
		return false
	}

	setClientSession(c, acc)

	return true
}

// jfTokenAccess is the account of a valid token (nil otherwise); checked
// tokens are trusted for a minute.
func jfTokenAccess(token string) *userAccess {
	if token == "" {
		return nil
	}

	if v, ok := jfTokens.Load(token); ok {
		if e := v.(jfTokenEntry); time.Now().Before(e.until) {
			return e.acc
		}

		jfTokens.Delete(token)
	}

	user := rememberTokenUser(token)

	key, ok := jfTokenKey(user)
	if !ok {
		return nil
	}

	if _, ok := parseRememberToken(key, token, time.Now()); !ok {
		return nil
	}

	acc := adminAccess
	if user != DefaultUsername {
		u, err := database.GetUserByName(&user)
		if err != nil || u.ID == 0 {
			return nil
		}

		acc = accessOfUser(u)
	}

	if !acc.CanUseApps() {
		return nil
	}

	jfTokens.Store(token, jfTokenEntry{acc: acc, until: time.Now().Add(time.Minute)})

	return acc
}

func jfServerID() string {
	sum := sha256.Sum256(append([]byte("jellyfin-server"), rememberServerSecret()...))
	return hex.EncodeToString(sum[:16])
}

// jfImageTag is an image's tag: apps load images without their login, so
// a valid tag stands for it.
func jfImageTag(id, typ string) string {
	return hex.EncodeToString(rememberMAC(rememberServerSecret(), "jfimg", strings.ToLower(id), strings.ToLower(typ))[:8])
}

func jfAuthenticate(c *gin.Context, _ map[string]string) {
	var body struct {
		Username string `json:"Username"`
		Pw       string `json:"Pw"`
		Password string `json:"Password"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if body.Pw == "" {
		body.Pw = body.Password
	}

	acc, err := clientAuthenticate(c, body.Username, body.Pw)
	if errors.Is(err, errClientNoApps) {
		c.String(http.StatusForbidden, err.Error())
		c.Abort()

		return
	}

	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	key, ok := jfTokenKey(acc.Name)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token := newRememberToken(key, acc.Name, time.Now().Add(jfTokenDuration))
	cl := jfClientOf(c)

	setClientSession(c, acc)
	c.JSON(http.StatusOK, gin.H{
		"User":        jfUserDto(acc),
		"SessionInfo": jfSessionInfo(acc, cl),
		"AccessToken": token,
		"ServerId":    jfServerID(),
	})
}

func jfSessionInfo(acc *userAccess, cl jfClient) gin.H {
	return gin.H{
		"Id": generateSecureToken(16), "UserId": jfUserID(acc.Name), "UserName": acc.Name, "Client": cl.Client,
		"DeviceName": cl.Device, "DeviceId": cl.DeviceID, "ApplicationVersion": cl.Version,
		"LastActivityDate": time.Now().UTC().Format(time.RFC3339), "IsActive": true, "SupportsMediaControl": false,
		"SupportsRemoteControl": false, "HasCustomDeviceName": false, "ServerId": jfServerID(), "PlayableMediaTypes": []string{},
		"AdditionalUsers": []any{}, "SupportedCommands": []string{}, "NowPlayingQueue": []any{}, "NowPlayingQueueFullItems": []any{},
		"Capabilities": gin.H{"PlayableMediaTypes": []string{}, "SupportedCommands": []string{}, "SupportsMediaControl": false,
			"SupportsPersistentIdentifier": true},
	}
}

// jfUserDto describes an account.
func jfUserDto(acc *userAccess) gin.H {
	return gin.H{
		"Name": acc.Name, "ServerId": jfServerID(), "ServerName": serverName(), "Id": jfUserID(acc.Name), "HasPassword": true,
		"HasConfiguredPassword": true, "HasConfiguredEasyPassword": false, "EnableAutoLogin": false,
		"LastLoginDate": time.Now().UTC().Format(time.RFC3339), "LastActivityDate": time.Now().UTC().Format(time.RFC3339),
		"Configuration": gin.H{
			"PlayDefaultAudioTrack": true, "SubtitleLanguagePreference": "", "DisplayMissingEpisodes": false, "GroupedFolders": []string{},
			"SubtitleMode": "Default", "DisplayCollectionsView": false, "EnableLocalPassword": false, "OrderedViews": []string{},
			"LatestItemsExcludes": []string{}, "MyMediaExcludes": []string{}, "HidePlayedInLatest": true,
			"RememberAudioSelections": true, "RememberSubtitleSelections": true, "EnableNextEpisodeAutoPlay": true,
			"AudioLanguagePreference": "", "CastReceiverId": "",
		},
		"Policy": gin.H{
			// No account administers this server through Jellyfin's
			// dashboard: the web client hides it.
			"IsAdministrator": false, "IsHidden": false, "IsDisabled": false, "EnableCollectionManagement": false,
			"EnableSubtitleManagement": false, "EnableLyricManagement": false, "BlockedTags": []string{}, "AllowedTags": []string{},
			"EnableUserPreferenceAccess": true, "AccessSchedules": []any{}, "BlockUnratedItems": []string{},
			"EnableRemoteControlOfOtherUsers": false, "EnableSharedDeviceControl": false, "EnableRemoteAccess": true,
			"EnableLiveTvManagement": false, "EnableLiveTvAccess": false, "EnableMediaPlayback": true,
			"EnableAudioPlaybackTranscoding": false, "EnableVideoPlaybackTranscoding": false, "EnablePlaybackRemuxing": false,
			"ForceRemoteSourceTranscoding": false, "EnableContentDeletion": false, "EnableContentDeletionFromFolders": []string{},
			"EnableContentDownloading": acc.CanDownload(), "EnableSyncTranscoding": false, "EnableMediaConversion": false,
			"EnabledDevices": []string{}, "EnableAllDevices": true, "EnabledChannels": []string{}, "EnableAllChannels": false,
			"EnabledFolders": []string{}, "EnableAllFolders": true, "InvalidLoginAttemptCount": 0, "LoginAttemptsBeforeLockout": -1,
			"MaxActiveSessions": 0, "EnablePublicSharing": false, "BlockedMediaFolders": []string{}, "BlockedChannels": []string{},
			"RemoteClientBitrateLimit": 0, "AuthenticationProviderId": "Jellyfin.Server.Implementations.Users.DefaultAuthenticationProvider",
			"PasswordResetProviderId": "Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider", "SyncPlayAccess": "None",
		},
	}
}

// --- system -----------------------------------------------------------------

func jfPublicInfo(c *gin.Context) gin.H {
	scheme := "http"
	if requestIsHTTPS(c) {
		scheme = "https"
	}

	return gin.H{
		"LocalAddress": scheme + "://" + c.Request.Host + "/jellyfin", "ServerName": serverName(), "Version": jfVersion,
		"ProductName": "Jellyfin Server", "OperatingSystem": runtime.GOOS, "Id": jfServerID(), "StartupWizardCompleted": true,
	}
}

func init() {
	empty := func(c *gin.Context, _ map[string]string) { c.JSON(http.StatusOK, []any{}) }
	emptyResult := func(c *gin.Context, _ map[string]string) {
		c.JSON(http.StatusOK, gin.H{"Items": []any{}, "TotalRecordCount": 0, "StartIndex": 0})
	}
	noContent := func(c *gin.Context, _ map[string]string) { c.Status(http.StatusNoContent) }
	me := func(c *gin.Context, _ map[string]string) { c.JSON(http.StatusOK, jfUserDto(accessOf(c))) }

	get, post, del := http.MethodGet, http.MethodPost, http.MethodDelete

	// Public.
	jfRoute2(get, "system/info/public", true, func(c *gin.Context, _ map[string]string) { c.JSON(http.StatusOK, jfPublicInfo(c)) })
	jfRoute2("", "system/ping", true, func(c *gin.Context, _ map[string]string) { c.JSON(http.StatusOK, "Jellyfin Server") })
	jfRoute2(get, "users/public", true, empty)
	jfRoute2(post, "users/authenticatebyname", true, jfAuthenticate)
	jfRoute2(get, "quickconnect/enabled", true, func(c *gin.Context, _ map[string]string) { c.JSON(http.StatusOK, false) })
	jfRoute2(get, "branding/configuration", true, func(c *gin.Context, _ map[string]string) {
		c.JSON(http.StatusOK, gin.H{"LoginDisclaimer": "", "CustomCss": "", "SplashscreenEnabled": false})
	})
	jfRoute2(get, "branding/css", true, func(c *gin.Context, _ map[string]string) { c.Data(http.StatusOK, "text/css", nil) })
	jfRoute2(get, "items/{id}/images/{type}/*", true, jfImage)

	// System and account.
	jfRoute2(get, "system/info", false, func(c *gin.Context, _ map[string]string) {
		info := jfPublicInfo(c)
		for k, v := range map[string]any{
			"OperatingSystemDisplayName": runtime.GOOS, "HasPendingRestart": false, "IsShuttingDown": false,
			"SupportsLibraryMonitor": false, "CanSelfRestart": false, "CanLaunchWebBrowser": false, "HasUpdateAvailable": false,
			"EncoderLocation": "System", "SystemArchitecture": runtime.GOARCH, "CompletedInstallations": []any{},
			"WebSocketPortNumber": 0, "PackageName": "",
		} {
			info[k] = v
		}

		c.JSON(http.StatusOK, info)
	})
	jfRoute2(get, "system/endpoint", false, func(c *gin.Context, _ map[string]string) {
		c.JSON(http.StatusOK, gin.H{"IsLocal": localRequest(c.Request), "IsInNetwork": localRequest(c.Request)})
	})
	jfRoute2(get, "users/me", false, me)
	jfRoute2(get, "users", false, func(c *gin.Context, _ map[string]string) { c.JSON(http.StatusOK, []any{jfUserDto(accessOf(c))}) })
	jfRoute2(get, "users/{uid}", false, me)
	jfRoute2(post, "sessions/logout", false, noContent)
	jfRoute2(post, "sessions/capabilities", false, noContent)
	jfRoute2(post, "sessions/capabilities/full", false, noContent)
	jfRoute2(get, "sessions", false, empty)
	jfRoute2(get, "displaypreferences/{id}", false, func(c *gin.Context, p map[string]string) {
		c.JSON(http.StatusOK, gin.H{
			"Id": p["id"], "SortBy": "SortName", "RememberIndexing": false, "PrimaryImageHeight": 250, "PrimaryImageWidth": 250,
			"CustomPrefs": gin.H{}, "ScrollDirection": "Horizontal", "ShowBackdrop": true, "RememberSorting": false,
			"SortOrder": "Ascending", "ShowSidebar": false, "Client": jfParam(c, "client"),
		})
	})
	jfRoute2(post, "displaypreferences/{id}", false, noContent)
	for _, p := range []string{"localization/cultures", "localization/countries", "localization/parentalratings", "localization/options",
		"plugins", "scheduledtasks", "users/{uid}/groupingoptions", "userviews/groupingoptions", "movies/recommendations",
		"web/configurationpages"} {
		jfRoute2(get, p, false, empty)
	}

	// Libraries and items.
	jfRoute2(get, "userviews", false, jfViews)
	jfRoute2(get, "users/{uid}/views", false, jfViews)
	jfRoute2(get, "items", false, jfGetItems)
	jfRoute2(get, "users/{uid}/items", false, jfGetItems)
	jfRoute2(get, "items/latest", false, jfLatest)
	jfRoute2(get, "users/{uid}/items/latest", false, jfLatest)
	jfRoute2(get, "useritems/resume", false, jfResume)
	jfRoute2(get, "users/{uid}/items/resume", false, jfResume)
	jfRoute2(get, "items/root", false, jfRoot)
	jfRoute2(get, "users/{uid}/items/root", false, jfRoot)
	jfRoute2(get, "items/counts", false, jfCounts)
	jfRoute2(get, "items/filters", false, func(c *gin.Context, _ map[string]string) {
		c.JSON(http.StatusOK, gin.H{"Genres": []string{}, "Tags": []string{}, "OfficialRatings": []string{}, "Years": []int{}})
	})
	jfRoute2(get, "items/filters2", false, func(c *gin.Context, _ map[string]string) {
		c.JSON(http.StatusOK, gin.H{"Genres": []any{}, "Tags": []string{}})
	})
	jfRoute2(get, "items/{id}", false, jfGetItem)
	jfRoute2(get, "users/{uid}/items/{id}", false, jfGetItem)
	jfRoute2(get, "artists", false, jfArtists)
	jfRoute2(get, "artists/albumartists", false, jfArtists)
	for _, p := range []string{"studios", "items/{id}/intros", "users/{uid}/items/{id}/intros", "videos/{id}/additionalparts",
		"mediasegments/{id}", "livetv/programs/recommended", "livetv/channels"} {
		jfRoute2(get, p, false, emptyResult)
	}
	for _, p := range []string{"items/{id}/localtrailers", "users/{uid}/items/{id}/localtrailers", "items/{id}/specialfeatures",
		"users/{uid}/items/{id}/specialfeatures", "items/{id}/ancestors"} {
		jfRoute2(get, p, false, empty)
	}
	jfRoute2(get, "items/{id}/thememedia", false, func(c *gin.Context, _ map[string]string) {
		e := gin.H{"Items": []any{}, "TotalRecordCount": 0, "StartIndex": 0, "OwnerId": ""}
		c.JSON(http.StatusOK, gin.H{"ThemeVideosResult": e, "ThemeSongsResult": e, "SoundtrackSongsResult": e})
	})
	jfRoute2(get, "shows/{id}/seasons", false, jfShowSeasons)
	jfRoute2(get, "shows/{id}/episodes", false, jfShowEpisodes)
	jfRoute2(get, "shows/nextup", false, jfNextUp)

	// Watched state.
	for _, p := range []string{"userplayeditems/{id}", "users/{uid}/playeditems/{id}"} {
		jfRoute2(post, p, false, func(c *gin.Context, p map[string]string) { jfSetPlayed(c, p["id"], true) })
		jfRoute2(del, p, false, func(c *gin.Context, p map[string]string) { jfSetPlayed(c, p["id"], false) })
	}
	for _, p := range []string{"userfavoriteitems/{id}", "users/{uid}/favoriteitems/{id}"} {
		jfRoute2(post, p, false, func(c *gin.Context, p map[string]string) { jfSetFavorite(c, p["id"], true) })
		jfRoute2(del, p, false, func(c *gin.Context, p map[string]string) { jfSetFavorite(c, p["id"], false) })
	}
	for _, p := range []string{"useritems/{id}/userdata", "users/{uid}/items/{id}/userdata"} {
		jfRoute2(get, p, false, func(c *gin.Context, p map[string]string) { jfUserDataReply(c, p["id"]) })
		jfRoute2(post, p, false, jfUpdateUserData)
	}
	jfRoute2(post, "useritems/{id}/rating", false, func(c *gin.Context, p map[string]string) { jfUserDataReply(c, p["id"]) })

	// Playing.
	jfRoute2("", "items/{id}/playbackinfo", false, jfPlaybackInfo)
	for _, p := range []string{"videos/{id}/stream", "audio/{id}/stream", "audio/{id}/universal", "videos/{id}/{source}/stream"} {
		jfRoute2(get, p, false, func(c *gin.Context, p map[string]string) { jfStream(c, p, false) })
	}
	for _, p := range []string{"items/{id}/download", "items/{id}/file"} {
		jfRoute2(get, p, false, func(c *gin.Context, p map[string]string) { jfStream(c, p, true) })
	}
	jfRoute2(post, "sessions/playing", false, func(c *gin.Context, _ map[string]string) { jfReport(c, "start") })
	jfRoute2(post, "sessions/playing/progress", false, func(c *gin.Context, _ map[string]string) { jfReport(c, "progress") })
	jfRoute2(post, "sessions/playing/stopped", false, func(c *gin.Context, _ map[string]string) { jfReport(c, "stop") })
	jfRoute2(post, "sessions/playing/ping", false, noContent)
}

// --- query parameters -------------------------------------------------------

// jfParam reads a query parameter by name, ignoring case (the apps differ).
func jfParam(c *gin.Context, name string) string {
	for k, v := range c.Request.URL.Query() {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}

	return ""
}

func jfInt(c *gin.Context, name string) int {
	n, _ := strconv.Atoi(jfParam(c, name))
	return n
}

func jfList(c *gin.Context, name string) []string {
	var out []string

	for s := range strings.SplitSeq(jfParam(c, name), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, strings.ToLower(s))
		}
	}

	return out
}

// jfResult answers a QueryResult.
func jfResult(c *gin.Context, items []jfItem, total, start int) {
	if items == nil {
		items = []jfItem{}
	}

	c.JSON(http.StatusOK, gin.H{"Items": items, "TotalRecordCount": total, "StartIndex": start})
}

// --- libraries --------------------------------------------------------------

// jfGroups are the account's groups the Jellyfin API shows (not books).
func jfGroups(c *gin.Context) []libGroup {
	return groupsOf(accessOf(c), "movie", "series", "album", "audiobook")
}

func jfViewItem(g *libGroup, lang string) jfItem {
	id := jfGroupID(g.Name)

	return jfItem{
		Name: groupTitle(g, lang), ServerID: jfServerID(), ID: id, Type: "CollectionFolder", IsFolder: true,
		CollectionType: jfCollectionTypes[g.Kind.Type], LocationType: "FileSystem", ImageTags: map[string]string{},
		BackdropImageTags: []string{}, PlayAccess: "Full", SortName: strings.ToLower(g.Name), Etag: id[24:],
	}
}

func jfViews(c *gin.Context, _ map[string]string) {
	lang := requestLanguage(c)
	groups := jfGroups(c)

	items := make([]jfItem, 0, len(groups)+1)
	for i := range groups {
		items = append(items, jfViewItem(&groups[i], lang))
	}

	if len(jfBoxSetsOf(jfMovieGroups(groups))) > 0 {
		items = append(items, jfBoxSetsView(lang))
	}

	if user := accName(accessOf(c)); len(database.GetPlaylists(user)) > 0 || len(savedViews(user)) > 0 {
		items = append(items, jfPlaylistsView(lang))
	}

	jfResult(c, items, len(items), 0)
}

func jfRoot(c *gin.Context, _ map[string]string) {
	id := jfID(jfServer, 1)
	c.JSON(http.StatusOK, jfItem{
		Name: "Media Folders", ServerID: jfServerID(), ID: id, Type: "UserRootFolder", IsFolder: true, LocationType: "FileSystem",
		ImageTags: map[string]string{}, BackdropImageTags: []string{},
	})
}

// jfGroupOf returns the group of a view ID.
func jfGroupOf(groups []libGroup, id string) *libGroup {
	for i := range groups {
		if strings.EqualFold(jfGroupID(groups[i].Name), strings.ReplaceAll(id, "-", "")) {
			return &groups[i]
		}
	}

	return nil
}

// jfDefaultKinds are the items a library lists without a type.
var jfDefaultKinds = map[string]byte{"movie": jfMovie, "series": jfSeries, "album": jfAlbum, "audiobook": jfBook}

// jfPart is one kind's part of an item query.
type jfPart struct {
	kind *jfKind
	q    jfQuery
}

// jfGetItems lists items: of a library or folder (ParentId), by type
// (IncludeItemTypes, Recursive), by ID (Ids), with search, filters, sort
// and paging.
func jfGetItems(c *gin.Context, _ map[string]string) {
	groups := jfGroups(c)
	start, limit := jfInt(c, "startIndex"), jfInt(c, "limit")
	types := jfList(c, "includeItemTypes")

	if ids := jfList(c, "ids"); len(ids) > 0 {
		items := jfItemsByID(c, groups, ids)
		jfResult(c, items, len(items), 0)

		return
	}

	parent := jfParam(c, "parentId")
	var parts []jfPart

	kindsOf := func(def byte, allowed ...byte) []byte {
		if len(types) == 0 {
			return []byte{def}
		}

		var out []byte

		for _, t := range types {
			for _, k := range jfTypeKinds[t] {
				if slices.Contains(allowed, k) && !slices.Contains(out, k) {
					out = append(out, k)
				}
			}
		}

		return out
	}

	switch code, n, ok := parseJFID(parent); {
	case slices.Equal(types, []string{"boxset"}) && (parent == "" || code == jfView || code == jfServer):
		if g := jfGroupOf(groups, parent); g != nil {
			groups = []libGroup{*g}
		}

		jfBoxSetList(c, groups)

		return
	case ok && code == jfBoxSet && n == 0:
		jfBoxSetList(c, groups)
		return
	case ok && code == jfBoxSet:
		jfBoxSetMovies(c, int(n)) //nolint:gosec // TMDB IDs fit

		return
	case ok && code == jfPlaylists, parent == "" && slices.Equal(types, []string{"playlist"}):
		items := jfPlaylistList(c)
		jfResult(c, items, len(items), 0)

		return
	case ok && code == jfSmart:
		if !jfSmartEntries(c, parent, start, limit) {
			jfResult(c, nil, 0, 0)
		}

		return
	case ok && code == jfPlaylist:
		if pl, ok := jfPlaylistOf(c, parent); ok {
			jfPlaylistEntries(c, pl, start, limit)
			return
		}

		jfResult(c, nil, 0, 0)

		return
	case parent == "" || code == jfServer:
		if len(types) == 0 && jfParam(c, "searchTerm") == "" {
			jfViews(c, nil)
			return
		}

		ks := kindsOf(0, jfMovie, jfSeries, jfEpisode, jfAlbum, jfTrack, jfArtist, jfBook, jfBookPart)
		if len(types) == 0 {
			ks = []byte{jfMovie, jfSeries, jfAlbum, jfArtist, jfBook}
		}

		for _, k := range ks {
			parts = append(parts, jfPart{jfKinds[k], jfQuery{Groups: groups}})
		}
	case ok && code == jfView:
		g := jfGroupOf(groups, parent)
		if g == nil {
			jfResult(c, nil, 0, 0)
			return
		}

		var allowed []byte

		switch g.Kind.Type {
		case "movie":
			allowed = []byte{jfMovie}
		case "series":
			allowed = []byte{jfSeries, jfEpisode}
		case "album":
			allowed = []byte{jfAlbum, jfTrack, jfArtist}
		case "audiobook":
			allowed = []byte{jfBook, jfBookPart}
		}

		for _, k := range kindsOf(jfDefaultKinds[g.Kind.Type], allowed...) {
			parts = append(parts, jfPart{jfKinds[k], jfQuery{Groups: []libGroup{*g}}})
		}
	case ok && code == jfSeries:
		if !slices.Contains(types, "episode") && jfParam(c, "recursive") != "true" {
			jfSeasonsReply(c, groups, n)
			return
		}

		parts = append(parts, jfPart{jfKinds[jfEpisode], jfQuery{Groups: groups, Cond: " AND x.parent_id = ?", Args: []any{n}, Order: "x.pidx, x.idx"}})
	case ok && code == jfSeason:
		parts = append(parts, jfPart{jfKinds[jfEpisode], jfQuery{
			Groups: groups, Cond: " AND x.parent_id = ? AND x.pidx = ?", Args: []any{n >> 16, n & 0xffff}, Order: "x.idx",
		}})
	case ok && code == jfAlbum:
		parts = append(parts, jfPart{jfKinds[jfTrack], jfQuery{Groups: groups, Cond: " AND x.parent_id = ?", Args: []any{n}, Order: "x.pidx, x.idx, x.loc"}})
	case ok && code == jfArtist:
		ks := kindsOf(jfAlbum, jfAlbum, jfTrack)
		for _, k := range ks {
			parts = append(parts, jfPart{jfKinds[k], jfQuery{Groups: groups, Cond: " AND x.artist_id = ?", Args: []any{n}, Order: "x.year, x.name"}})
		}
	case ok && code == jfBook:
		parts = append(parts, jfPart{jfKinds[jfBookPart], jfQuery{Groups: groups, Cond: " AND x.parent_id = ?", Args: []any{n}, Order: "x.pidx, x.idx, x.loc"}})
	default:
		jfResult(c, nil, 0, start)
		return
	}

	items, total := jfRunParts(c, parts, start, limit)
	jfResult(c, items, total, start)
}

// jfFilters adds the request's search and filters to a part's query.
func jfFilters(c *gin.Context, p *jfPart) {
	k, q := p.kind, &p.q
	user := accName(accessOf(c))

	if s := strings.TrimSpace(jfParam(c, "searchTerm")); s != "" {
		like := "%" + s + "%"
		cond := " AND (x.name LIKE ? OR x.original LIKE ? OR x.artist LIKE ?"
		q.Args = append(q.Args, like, like, like)

		if mt := map[byte]string{jfMovie: "dbmovie", jfSeries: "dbserie", jfEpisode: "dbserie_episode", jfBook: "dbaudiobook"}[k.Code]; mt != "" {
			cond += " OR EXISTS (SELECT 1 FROM metadata_translations mt WHERE mt.media_type = ? AND mt.media_id = x.db_id AND mt.translation LIKE ?)"
			q.Args = append(q.Args, mt, like)
		}

		q.Cond += cond + ")"
	}

	if ids := jfList(c, "personIds"); len(ids) > 0 {
		var persons []uint64

		for _, id := range ids {
			if code, n, ok := parseJFID(id); ok && code == jfPerson {
				persons = append(persons, n)
			}
		}

		cond, args := jfPersonCastCond(k.Code, persons)
		q.Cond += cond
		q.Args = append(q.Args, args...)
	}

	if s := jfParam(c, "nameStartsWith"); s != "" {
		q.Cond += " AND x.name LIKE ?"
		q.Args = append(q.Args, s+"%")
	}

	var genres []string
	if g := jfParam(c, "genres"); g != "" {
		genres = strings.Split(g, "|")
	}

	if ids := jfList(c, "genreIds"); len(ids) > 0 {
		genres = append(genres, jfGenreNamesOf(q.Groups, ids)...)
		if len(genres) == 0 {
			q.Cond += " AND 0"
		}
	}

	if len(genres) > 0 {
		var conds []string

		for _, name := range genres {
			conds = append(conds, "(',' || replace(x.genres, ', ', ',') || ',') LIKE ?")
			q.Args = append(q.Args, "%,"+strings.TrimSpace(name)+",%")
		}

		q.Cond += " AND (" + strings.Join(conds, " OR ") + ")"
	}

	if years := jfList(c, "years"); len(years) > 0 {
		q.Cond += " AND x.year IN (?" + strings.Repeat(", ?", len(years)-1) + ")"
		for _, y := range years {
			q.Args = append(q.Args, y)
		}
	}

	filters := jfList(c, "filters")
	if v := jfParam(c, "isPlayed"); v != "" {
		filters = append(filters, map[string]string{"true": "isplayed", "false": "isunplayed"}[strings.ToLower(v)])
	}

	if strings.EqualFold(jfParam(c, "isFavorite"), "true") {
		filters = append(filters, "isfavorite")
	}

	for _, f := range filters {
		switch f {
		case "isplayed", "isunplayed":
			cond, args := k.playedCond(user)
			if f == "isunplayed" {
				cond = "NOT " + cond
			}

			q.Cond += " AND " + cond
			q.Args = append(q.Args, args...)
		case "isresumable":
			cond, args := k.resumableCond(user)
			q.Cond += " AND " + cond
			q.Args = append(q.Args, args...)
		case "isfavorite", "likes":
			kind := jfPlaylistKinds[k.Code]
			if kind == "" {
				q.Cond += " AND 0"
				continue
			}

			cond, args := favoriteCond(user, kind, "x.id")
			q.Cond += " AND " + cond
			q.Args = append(q.Args, args...)
		}
	}

	q.Order = jfOrder(jfParam(c, "sortBy"), jfParam(c, "sortOrder"), q.Order)
}

// jfRunParts runs the parts of a query and returns the page of their
// items in order and the total.
func jfRunParts(c *gin.Context, parts []jfPart, start, limit int) ([]jfItem, int) {
	var (
		items []jfItem
		total int
	)

	for i := range parts {
		jfFilters(c, &parts[i])

		q := parts[i].q
		if len(parts) == 1 {
			q.Start, q.Limit = start, limit
		} else if limit > 0 {
			q.Limit = start + limit
		}

		rows, n := jfRows(parts[i].kind, q)
		total += n
		items = append(items, jfBuild(c, parts[i].kind, rows)...)
	}

	if len(parts) > 1 {
		items = items[min(start, len(items)):]
		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
	}

	return items, total
}

// jfBuild makes the items of rows, translated, with the account's state.
func jfBuild(c *gin.Context, k *jfKind, rows []jfRow) []jfItem {
	acc := accessOf(c)

	items := jfItems(k, rows, jfServerID(), requestLanguage(c))
	for i := range items {
		items[i].CanDownload = acc.CanDownload() && !k.Folder
	}

	user := accName(acc)
	favs := favoriteSet(user)

	for i := range rows {
		items[i].UserData = jfUserDataOf(user, k, &rows[i], items[i].ID)
		items[i].UserData.IsFavorite = jfFavoriteOf(favs, k.Code, rows[i].ID)
	}

	return items
}

// jfItemsByID loads items by their IDs, in that order.
func jfItemsByID(c *gin.Context, groups []libGroup, ids []string) []jfItem {
	var out []jfItem

	for _, id := range ids {
		if it, ok := jfLoad(c, groups, id); ok {
			out = append(out, it)
		}
	}

	return out
}

// jfLoad loads one item the account may see.
func jfLoad(c *gin.Context, groups []libGroup, id string) (jfItem, bool) {
	code, n, ok := parseJFID(id)
	if !ok {
		return jfItem{}, false
	}

	// A further version (file) of a movie or episode: that item.
	switch code {
	case jfMovieFile:
		code, n = jfMovie, database.Getdatarow[uint64](false, "SELECT movie_id FROM movie_files WHERE id = ?", &n)
		id = jfID(code, n)
	case jfEpisodeFile:
		code, n = jfEpisode, database.Getdatarow[uint64](false, "SELECT serie_episode_id FROM serie_episode_files WHERE id = ?", &n)
		id = jfID(code, n)
	}

	switch code {
	case jfPerson:
		if name, profile, ok := jfPersonOf(n); ok {
			return jfPersonItem(n, name, profile), true
		}

		return jfItem{}, false
	case jfPlaylists:
		return jfPlaylistsView(requestLanguage(c)), true
	case jfBoxSet:
		if n == 0 {
			return jfBoxSetsView(requestLanguage(c)), true
		}

		for _, g := range jfBoxSetsOf(jfMovieGroups(groups)) {
			if uint64(g.ID) == n { //nolint:gosec // positive
				return jfBoxSetItem(&g), true
			}
		}

		return jfItem{}, false
	case jfSmart:
		if v, ok := smartView(accName(accessOf(c)), n); ok {
			return jfSmartItem(v), true
		}

		return jfItem{}, false
	case jfPlaylist:
		if p, ok := jfPlaylistOf(c, id); ok {
			return jfPlaylistItemOf(p), true
		}

		return jfItem{}, false
	case jfView:
		if g := jfGroupOf(groups, id); g != nil {
			return jfViewItem(g, requestLanguage(c)), true
		}

		return jfItem{}, false
	case jfSeason:
		for _, s := range jfSeasons(groups, n>>16) {
			if uint64(s.Season) == n&0xffff { //nolint:gosec // season numbers are small
				it := jfSeasonItem(n>>16, &s, jfServerID())
				it.UserData = jfSeasonUserData(accName(accessOf(c)), n>>16, s.Season, s.Files, it.ID)
				jfTranslateSeason(&it, s.DbID, requestLanguage(c))

				return it, true
			}
		}

		return jfItem{}, false
	}

	k := jfKinds[code]
	if k == nil {
		return jfItem{}, false
	}

	rows, _ := jfRows(k, jfQuery{Groups: groups, Cond: " AND x.id = ?", Args: []any{n}})
	if len(rows) == 0 {
		return jfItem{}, false
	}

	return jfBuild(c, k, rows)[0], true
}

func jfTranslateSeason(it *jfItem, dbSerie uint64, lang string) {
	tr := database.TranslationsIn("dbserie", lang, []uint{uint(dbSerie)})
	it.SeriesName = firstText(tr[uint(dbSerie)]["title"], it.SeriesName)
}

func jfGetItem(c *gin.Context, p map[string]string) {
	it, ok := jfLoad(c, jfGroups(c), p["id"])
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if code, n, _ := parseJFID(p["id"]); code == jfMovie || code == jfSeries {
		db := database.Getdatarow[uint64](false, "SELECT "+map[byte]string{jfMovie: "dbmovie_id FROM movies", jfSeries: "dbserie_id FROM series"}[code]+
			" WHERE id = ?", &n)
		jfAddDetails(c, &it, code, db)
	}

	if !it.IsFolder && it.Type != "MusicArtist" {
		it.MediaSources = jfMediaSources(c, p["id"], nil, "")
		if len(it.MediaSources) > 0 {
			it.MediaStreams = it.MediaSources[0].MediaStreams
		}
	}

	c.JSON(http.StatusOK, it)
}

func jfArtists(c *gin.Context, _ map[string]string) {
	groups := jfGroups(c)
	if g := jfGroupOf(groups, jfParam(c, "parentId")); g != nil {
		groups = []libGroup{*g}
	}

	items, total := jfRunParts(c, []jfPart{{jfKinds[jfArtist], jfQuery{Groups: groups}}}, jfInt(c, "startIndex"), jfInt(c, "limit"))
	jfResult(c, items, total, jfInt(c, "startIndex"))
}

func jfCounts(c *gin.Context, _ map[string]string) {
	groups := jfGroups(c)
	count := func(code byte) int {
		_, n := jfRows(jfKinds[code], jfQuery{Groups: groups, Limit: 1})
		return n
	}

	c.JSON(http.StatusOK, gin.H{
		"MovieCount": count(jfMovie), "SeriesCount": count(jfSeries), "EpisodeCount": count(jfEpisode),
		"AlbumCount": count(jfAlbum), "SongCount": count(jfTrack), "ArtistCount": count(jfArtist), "BookCount": count(jfBook),
		"MusicVideoCount": 0, "BoxSetCount": 0, "ProgramCount": 0, "TrailerCount": 0, "ItemCount": 0,
	})
}

// --- series -----------------------------------------------------------------

func jfSeasonsReply(c *gin.Context, groups []libGroup, seriesID uint64) {
	user, sid, lang := accName(accessOf(c)), jfServerID(), requestLanguage(c)

	seasons := jfSeasons(groups, seriesID)
	items := make([]jfItem, 0, len(seasons))

	for i := range seasons {
		it := jfSeasonItem(seriesID, &seasons[i], sid)
		it.UserData = jfSeasonUserData(user, seriesID, seasons[i].Season, seasons[i].Files, it.ID)
		jfTranslateSeason(&it, seasons[i].DbID, lang)
		items = append(items, it)
	}

	jfResult(c, items, len(items), 0)
}

func jfShowSeasons(c *gin.Context, p map[string]string) {
	code, n, ok := parseJFID(p["id"])
	if !ok || code != jfSeries {
		jfResult(c, nil, 0, 0)
		return
	}

	jfSeasonsReply(c, jfGroups(c), n)
}

func jfShowEpisodes(c *gin.Context, p map[string]string) {
	code, n, ok := parseJFID(p["id"])
	if !ok || code != jfSeries {
		jfResult(c, nil, 0, 0)
		return
	}

	q := jfQuery{Groups: jfGroups(c), Cond: " AND x.parent_id = ?", Args: []any{n}, Order: "x.pidx, x.idx"}

	if sc, sn, ok := parseJFID(jfParam(c, "seasonId")); ok && sc == jfSeason {
		q.Cond += " AND x.pidx = ?"
		q.Args = append(q.Args, sn&0xffff)
	} else if s := jfParam(c, "season"); s != "" {
		q.Cond += " AND x.pidx = ?"
		q.Args = append(q.Args, s)
	}

	start, limit := jfInt(c, "startIndex"), jfInt(c, "limit")
	items, total := jfRunParts(c, []jfPart{{jfKinds[jfEpisode], q}}, start, limit)

	// startItemId: from that episode on.
	if from := jfParam(c, "startItemId"); from != "" {
		if i := slices.IndexFunc(items, func(it jfItem) bool { return strings.EqualFold(it.ID, strings.ReplaceAll(from, "-", "")) }); i > 0 {
			items = items[i:]
		}
	}

	jfResult(c, items, total, start)
}

// jfNextUp returns the next episode of each series the account watches.
func jfNextUp(c *gin.Context, _ map[string]string) {
	var series []uint64

	if code, n, ok := parseJFID(jfParam(c, "seriesId")); ok && code == jfSeries {
		series = []uint64{n}
	}

	limit := jfInt(c, "limit")
	if limit <= 0 {
		limit = 24
	}

	rows := nextEpisodes(jfGroups(c), accName(accessOf(c)), series, limit, false)
	items := jfBuild(c, jfKinds[jfEpisode], rows)

	jfResult(c, items, len(items), 0)
}

// jfLatest lists what was added last to a library (an array, not a
// QueryResult).
func jfLatest(c *gin.Context, _ map[string]string) {
	groups := jfGroups(c)
	limit := jfInt(c, "limit")

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	codes := []byte{jfMovie, jfEpisode}

	if g := jfGroupOf(groups, jfParam(c, "parentId")); g != nil {
		groups = []libGroup{*g}
		codes = []byte{map[string]byte{"movie": jfMovie, "series": jfEpisode, "album": jfAlbum, "audiobook": jfBook}[g.Kind.Type]}
	}

	if types := jfList(c, "includeItemTypes"); len(types) > 0 {
		var cs []byte

		for _, t := range types {
			for _, k := range jfTypeKinds[t] {
				if !slices.Contains(cs, k) {
					cs = append(cs, k)
				}
			}
		}

		codes = cs
	}

	var items []jfItem

	for _, code := range codes {
		order := "x.id DESC"
		if code == jfEpisode {
			order = "x.file_id DESC"
		}

		rows, _ := jfRows(jfKinds[code], jfQuery{Groups: groups, Order: order, Limit: limit})
		items = append(items, jfBuild(c, jfKinds[code], rows)...)
	}

	slices.SortStableFunc(items, func(a, b jfItem) int { return strings.Compare(b.DateCreated, a.DateCreated) })

	if len(items) > limit {
		items = items[:limit]
	}

	if items == nil {
		items = []jfItem{}
	}

	c.JSON(http.StatusOK, items)
}

// jfResume lists the files the account stopped in the middle of.
func jfResume(c *gin.Context, _ map[string]string) {
	groups := jfGroups(c)
	user := accName(accessOf(c))
	limit := jfInt(c, "limit")

	if limit <= 0 || limit > 100 {
		limit = 24
	}

	if g := jfGroupOf(groups, jfParam(c, "parentId")); g != nil {
		groups = []libGroup{*g}
	}

	kinds := []string{"movie", "episode"}

	switch types := jfList(c, "mediaTypes"); {
	case slices.Contains(types, "audio"):
		kinds = []string{"audiobook"}
	case slices.Contains(types, "book"):
		// Books aren't in the Jellyfin libraries.
		jfResult(c, nil, 0, 0)
		return
	}

	rows := database.StructscanT[struct {
		Kind   string `db:"kind"`
		FileID uint64 `db:"file_id"`
	}](false, 100, "SELECT kind, file_id FROM play_history WHERE username = ? AND position > 0 AND completion < "+
		strconv.FormatFloat(database.Watched, 'f', 2, 64)+" AND kind IN ('"+strings.Join(kinds, "', '")+"') "+
		"ORDER BY updated_at DESC LIMIT 100", user)

	var items []jfItem

	for _, r := range rows {
		var (
			k  *jfKind
			id uint64
		)

		switch r.Kind {
		case "movie":
			k, id = jfKinds[jfMovie], database.Getdatarow[uint64](false, "SELECT movie_id FROM movie_files WHERE id = ?", &r.FileID)
		case "episode":
			k, id = jfKinds[jfEpisode], database.Getdatarow[uint64](false, "SELECT serie_episode_id FROM serie_episode_files WHERE id = ?", &r.FileID)
		case "audiobook":
			k, id = jfKinds[jfBookPart], r.FileID
		}

		if k == nil || id == 0 || slices.ContainsFunc(items, func(it jfItem) bool { return it.ID == jfID(k.Code, id) }) {
			continue
		}

		found, _ := jfRows(k, jfQuery{Groups: groups, Cond: " AND x.id = ?", Args: []any{id}})
		items = append(items, jfBuild(c, k, found)...)

		if len(items) >= limit {
			break
		}
	}

	jfResult(c, items, len(items), 0)
}

// --- watched state ----------------------------------------------------------

// jfUserDataOf is the account's state of an item of a row.
func jfUserDataOf(user string, k *jfKind, r *jfRow, itemID string) *jfUserData {
	if k.Code == jfArtist {
		return &jfUserData{Key: itemID, ItemID: itemID}
	}

	hist := database.StructscanT[jfHist](false, 8, "SELECT completion, position, plays, updated_at FROM play_history WHERE username = ? AND kind = ? AND file_id IN ("+
		k.filesOf("?")+") ORDER BY updated_at DESC", user, k.PlayKind, r.ID)

	return jfUserDataFrom(hist, k.Folder, r.Children, k.Code == jfArtist, itemID)
}

func jfSeasonUserData(user string, seriesID uint64, season, files int, itemID string) *jfUserData {
	hist := database.StructscanT[jfHist](false, 32, "SELECT completion, position, plays, updated_at FROM play_history WHERE username = ? AND kind = 'episode' AND file_id IN ("+
		jfSeasonFiles+") ORDER BY updated_at DESC", user, seriesID, season)

	return jfUserDataFrom(hist, true, files, false, itemID)
}

// jfHist is the history of a file of an item.
type jfHist struct {
	Completion float64 `db:"completion"`
	Position   float64 `db:"position"`
	Plays      int     `db:"plays"`
	Updated    string  `db:"updated_at"`
}

func jfUserDataFrom(hist []jfHist, folder bool, files int, noCount bool, itemID string) *jfUserData {
	d := &jfUserData{Key: itemID, ItemID: itemID}
	watched := 0

	for i := range hist {
		h := &hist[i]

		d.PlayCount += h.Plays

		if d.LastPlayedDate == "" {
			d.LastPlayedDate = jfCreated(h.Updated)
		}

		if h.Completion >= database.Watched {
			watched++
		} else if !folder && d.PlaybackPositionTicks == 0 && h.Position > 0 {
			d.PlaybackPositionTicks, d.PlayedPercentage = ticks(h.Position), h.Completion*100
		}
	}

	if !folder {
		d.Played = watched > 0

		if d.Played {
			d.PlaybackPositionTicks, d.PlayedPercentage = 0, 0
		}

		return d
	}

	if noCount {
		return d
	}

	unplayed := max(files-watched, 0)
	d.UnplayedItemCount = &unplayed
	d.Played = files > 0 && unplayed == 0

	return d
}

// jfSetPlayed marks an item (with all its files) watched or not.
func jfSetPlayed(c *gin.Context, id string, played bool) {
	groups := jfGroups(c)
	if _, ok := jfLoad(c, groups, id); !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	user := accName(accessOf(c))
	code, n, _ := parseJFID(id)

	var err error

	switch code {
	case jfMovie:
		err = database.SetEntryWatched(user, "movie", "movie_files", "movie_id", &n, played)
	case jfSeries:
		err = database.SetEntryWatched(user, "episode", "serie_episode_files", "serie_id", &n, played)
	case jfEpisode:
		err = database.SetEntryWatched(user, "episode", "serie_episode_files", "serie_episode_id", &n, played)
	case jfSeason:
		for _, fid := range database.GetrowsN[uint](false, 256, jfSeasonFiles, n>>16, n&0xffff) {
			if err = database.SetWatched(user, "episode", &fid, played); err != nil {
				break
			}
		}
	case jfAlbum:
		err = database.SetEntryWatched(user, "track", "album_files", "album_id", &n, played)
	case jfBook:
		err = database.SetEntryWatched(user, "audiobook", "audiobook_files", "audiobook_id", &n, played)
	case jfTrack, jfBookPart:
		fid := uint(n)
		err = database.SetWatched(user, map[byte]string{jfTrack: "track", jfBookPart: "audiobook"}[code], &fid, played)
	default:
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	jfUserDataReply(c, id)
}

// jfUserDataReply answers the account's state of an item.
func jfUserDataReply(c *gin.Context, id string) {
	it, ok := jfLoad(c, jfGroups(c), id)
	if !ok || it.UserData == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, it.UserData)
}

func jfUpdateUserData(c *gin.Context, p map[string]string) {
	var body struct {
		Played     *bool `json:"Played"`
		IsFavorite *bool `json:"IsFavorite"`
	}

	_ = c.ShouldBindJSON(&body)

	if body.IsFavorite != nil {
		if it, ok := jfPlaylistItem(p["id"]); ok {
			_ = setFavorite(accessOf(c), it.Kind, it.EntryID, it.ItemID, *body.IsFavorite)
		}
	}

	if body.Played != nil {
		jfSetPlayed(c, p["id"], *body.Played)
		return
	}

	jfUserDataReply(c, p["id"])
}

// --- playing ----------------------------------------------------------------

// jfMediaSource is a file of an item.
type jfMediaSource struct {
	Protocol                string            `json:"Protocol"`
	ID                      string            `json:"Id"`
	Type                    string            `json:"Type"`
	Container               string            `json:"Container"`
	Size                    int64             `json:"Size,omitempty"`
	Name                    string            `json:"Name"`
	IsRemote                bool              `json:"IsRemote"`
	ETag                    string            `json:"ETag"`
	RunTimeTicks            int64             `json:"RunTimeTicks,omitempty"`
	ReadAtNativeFramerate   bool              `json:"ReadAtNativeFramerate"`
	IgnoreDts               bool              `json:"IgnoreDts"`
	IgnoreIndex             bool              `json:"IgnoreIndex"`
	GenPtsInput             bool              `json:"GenPtsInput"`
	SupportsTranscoding     bool              `json:"SupportsTranscoding"`
	SupportsDirectStream    bool              `json:"SupportsDirectStream"`
	SupportsDirectPlay      bool              `json:"SupportsDirectPlay"`
	IsInfiniteStream        bool              `json:"IsInfiniteStream"`
	RequiresOpening         bool              `json:"RequiresOpening"`
	RequiresClosing         bool              `json:"RequiresClosing"`
	RequiresLooping         bool              `json:"RequiresLooping"`
	SupportsProbing         bool              `json:"SupportsProbing"`
	VideoType               string            `json:"VideoType,omitempty"`
	MediaStreams            []jfMediaStream   `json:"MediaStreams"`
	MediaAttachments        []any             `json:"MediaAttachments"`
	Formats                 []string          `json:"Formats"`
	Bitrate                 int               `json:"Bitrate,omitempty"`
	RequiredHTTPHeaders     map[string]string `json:"RequiredHttpHeaders"`
	DefaultAudioStreamIndex *int              `json:"DefaultAudioStreamIndex,omitempty"`
	DirectStreamURL         string            `json:"DirectStreamUrl,omitempty"`
	TranscodingURL          string            `json:"TranscodingUrl,omitempty"`
	TranscodingSubProtocol  string            `json:"TranscodingSubProtocol,omitempty"`
	TranscodingContainer    string            `json:"TranscodingContainer,omitempty"`
}

type jfMediaStream struct {
	Codec                  string `json:"Codec"`
	Language               string `json:"Language,omitempty"`
	Title                  string `json:"Title,omitempty"`
	DisplayTitle           string `json:"DisplayTitle"`
	IsDefault              bool   `json:"IsDefault"`
	IsForced               bool   `json:"IsForced"`
	Type                   string `json:"Type"`
	Index                  int    `json:"Index"`
	IsExternal             bool   `json:"IsExternal"`
	IsTextSubtitleStream   bool   `json:"IsTextSubtitleStream"`
	SupportsExternalStream bool   `json:"SupportsExternalStream"`
	Height                 int    `json:"Height,omitempty"`
	Channels               int    `json:"Channels,omitempty"`
	IsInterlaced           bool   `json:"IsInterlaced"`
}

// jfFile is a playable file of an item.
type jfFile struct {
	Kind     string // play kind
	ID       uint64
	SourceID string
	Location string
	Size     int64
}

// jfFilesOf returns the files of a playable item, first the one played
// without a choice.
func jfFilesOf(id string) []jfFile {
	code, n, ok := parseJFID(id)
	if !ok {
		return nil
	}

	type row struct {
		ID       uint64 `db:"id"`
		Location string `db:"location"`
		Size     int64  `db:"size"`
	}

	var (
		rows       []row
		kind       string
		sourceCode byte
	)

	switch code {
	case jfMovie:
		kind, sourceCode = "movie", jfMovieFile
		rows = database.StructscanT[row](false, 2, "SELECT id, location, COALESCE(file_size, 0) AS size FROM movie_files WHERE movie_id = ? ORDER BY id", &n)
	case jfEpisode:
		kind, sourceCode = "episode", jfEpisodeFile
		rows = database.StructscanT[row](false, 2, "SELECT id, location, COALESCE(file_size, 0) AS size FROM serie_episode_files WHERE serie_episode_id = ? ORDER BY id", &n)
	case jfTrack:
		kind, sourceCode = "track", jfTrack
		rows = database.StructscanT[row](false, 1, "SELECT id, location, COALESCE(file_size, 0) AS size FROM album_files WHERE id = ?", &n)
	case jfBookPart:
		kind, sourceCode = "audiobook", jfBookPart
		rows = database.StructscanT[row](false, 1, "SELECT id, location, COALESCE(file_size, 0) AS size FROM audiobook_files WHERE id = ?", &n)
	}

	out := make([]jfFile, 0, len(rows))
	for i, r := range rows {
		// The first file's source is the item itself, as in Jellyfin: the
		// web client looks the source up as an item.
		source := jfID(sourceCode, r.ID)
		if i == 0 {
			source = jfID(code, n)
		}

		out = append(out, jfFile{Kind: kind, ID: r.ID, SourceID: source, Location: r.Location, Size: r.Size})
	}

	return out
}

// jfFileFor picks the file of a request: its MediaSourceId, else the first.
func jfFileFor(id, source string) (jfFile, bool) {
	files := jfFilesOf(id)
	if len(files) == 0 {
		return jfFile{}, false
	}

	source = strings.ToLower(strings.ReplaceAll(source, "-", ""))
	for _, f := range files {
		if f.SourceID == source {
			return f, true
		}
	}

	return files[0], true
}

// jfMediaSources describes the files of an item (probed with ffprobe);
// with a playback request, videos the app can't play as they are get the
// transcoding URL.
func jfMediaSources(c *gin.Context, id string, req *jfPlaybackRequest, playSession string) []jfMediaSource {
	var out []jfMediaSource

	for _, f := range jfFilesOf(id) {
		name := f.Location[strings.LastIndexAny(f.Location, `/\`)+1:]
		ext := strings.TrimPrefix(strings.ToLower(name[strings.LastIndex(name, ".")+1:]), ".")

		ms := jfMediaSource{
			Protocol: "File", ID: f.SourceID, Type: "Default", Container: ext, Size: f.Size, Name: fileTitle(name),
			ETag: f.SourceID[24:], SupportsDirectStream: true, SupportsDirectPlay: true, SupportsProbing: true,
			MediaStreams: []jfMediaStream{}, MediaAttachments: []any{}, Formats: []string{}, RequiredHTTPHeaders: map[string]string{},
		}

		if f.Kind == "movie" || f.Kind == "episode" {
			ms.VideoType = "VideoFile"
		}

		if p, err := probeMedia(c.Request.Context(), f.Location); err == nil {
			ms.RunTimeTicks, ms.Bitrate = ticks(p.Duration), p.Kbps*1000
			ms.MediaStreams = jfStreamsOf(p)

			if i := slices.IndexFunc(ms.MediaStreams, func(s jfMediaStream) bool { return s.Type == "Audio" }); i >= 0 {
				ms.DefaultAudioStreamIndex = intPtr(ms.MediaStreams[i].Index)
			}

			if req != nil {
				jfTranscodeInfo(c, &ms, id, playSession, p, req)
			}
		}

		out = append(out, ms)
	}

	return out
}

// jfStreamsOf lists a file's streams: video, audio tracks, subtitles.
func jfStreamsOf(p playProbe) []jfMediaStream {
	var out []jfMediaStream

	if p.VCodec != "" {
		out = append(out, jfMediaStream{Codec: p.VCodec, Type: "Video", Index: 0, Height: p.Height, IsDefault: true,
			DisplayTitle: strings.ToUpper(p.VCodec) + jfHeightLabel(p.Height)})
	}

	if len(p.AudioTracks) == 0 && p.ACodec != "" {
		out = append(out, jfMediaStream{Codec: p.ACodec, Type: "Audio", Index: len(out), IsDefault: true, DisplayTitle: strings.ToUpper(p.ACodec)})
	}

	for _, t := range p.AudioTracks {
		out = append(out, jfMediaStream{
			Codec: t.Codec, Language: t.Lang, Title: t.Title, Type: "Audio", Index: len(out), IsDefault: t.Default, Channels: t.Channels,
			DisplayTitle: strings.TrimSpace(strings.ToUpper(t.Lang) + " " + strings.ToUpper(t.Codec) + " " + t.Title),
		})
	}

	for _, t := range p.SubTracks {
		out = append(out, jfMediaStream{
			Codec: t.Codec, Language: t.Lang, Title: t.Title, Type: "Subtitle", Index: len(out), IsDefault: t.Default, IsForced: t.Forced,
			IsTextSubtitleStream: !imageSubCodecs[t.Codec],
			DisplayTitle:         strings.TrimSpace(strings.ToUpper(t.Lang) + " " + t.Title),
		})
	}

	return out
}

func jfHeightLabel(h int) string {
	if h <= 0 {
		return ""
	}

	return " " + strconv.Itoa(h) + "p"
}

func jfPlaybackInfo(c *gin.Context, p map[string]string) {
	if _, ok := jfLoad(c, jfGroups(c), p["id"]); !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var req jfPlaybackRequest
	if c.Request.Method == http.MethodPost {
		_ = c.ShouldBindJSON(&req)
	}

	if req.MaxStreamingBitrate == 0 {
		req.MaxStreamingBitrate, _ = strconv.ParseInt(jfParam(c, "maxStreamingBitrate"), 10, 64)
	}

	if req.AudioStreamIndex == nil {
		if a, err := strconv.Atoi(jfParam(c, "audioStreamIndex")); err == nil {
			req.AudioStreamIndex = &a
		}
	}

	playSession := generateSecureToken(16)

	sources := jfMediaSources(c, p["id"], &req, playSession)
	if len(sources) == 0 {
		c.JSON(http.StatusOK, gin.H{"MediaSources": []any{}, "ErrorCode": "NoCompatibleStream"})
		return
	}

	if want := strings.ToLower(strings.ReplaceAll(jfParam(c, "mediaSourceId"), "-", "")); want != "" {
		if i := slices.IndexFunc(sources, func(s jfMediaSource) bool { return s.ID == want }); i >= 0 {
			sources = []jfMediaSource{sources[i]}
		}
	}

	c.JSON(http.StatusOK, gin.H{"MediaSources": sources, "PlaySessionId": playSession})
}

// jfStream sends the file of an item as it is.
func jfStream(c *gin.Context, p map[string]string, download bool) {
	if _, ok := jfLoad(c, jfGroups(c), p["id"]); !ok || (download && !accessOf(c).CanDownload()) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	source := jfParam(c, "mediaSourceId")
	if source == "" {
		source = p["source"]
	}

	f, ok := jfFileFor(p["id"], source)
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if download {
		c.Header("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(f.Location[strings.LastIndexAny(f.Location, `/\`)+1:], `"`, "")+`"`)
	}

	serveMediaFile(c, f.Kind, f.ID)
}

var jfTitles sync.Map // player session -> title

// jfReport takes an app's playback report (start, progress, stop).
func jfReport(c *gin.Context, event string) {
	var body struct {
		ItemID        string `json:"ItemId"`
		MediaSourceID string `json:"MediaSourceId"`
		PositionTicks int64  `json:"PositionTicks"`
		IsPaused      bool   `json:"IsPaused"`
		PlaySessionID string `json:"PlaySessionId"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	it, ok := jfLoad(c, jfGroups(c), body.ItemID)
	if !ok {
		c.Status(http.StatusNoContent)
		return
	}

	f, ok := jfFileFor(body.ItemID, body.MediaSourceID)
	if !ok {
		c.Status(http.StatusNoContent)
		return
	}

	cl := jfClientOf(c)
	sum := sha1.Sum([]byte(cl.DeviceID + "|" + f.SourceID)) //nolint:gosec // only an ID
	sid := "jf" + hex.EncodeToString(sum[:])[:24]

	state := map[string]string{"start": "start", "progress": "play", "stop": "stop"}[event]
	if state == "play" && body.IsPaused {
		state = "pause"
	}

	title := it.Name
	if it.SeriesName != "" {
		title = it.SeriesName + " - " + title
	}

	jfTitles.Store(sid, title)

	if state == "stop" {
		jfTitles.Delete(sid)
		jfStopTranscode(body.PlaySessionID)
	}

	client := c.ClientIP()
	if cl.Client != "" {
		client = cl.Client + " (" + cl.Device + ", " + client + ")"
	}

	_, err := recordPlayProgress(accName(accessOf(c)), playReport{
		SID: sid, Kind: f.Kind, FileID: f.ID, State: state, Pos: float64(body.PositionTicks) / 1e7,
		Dur: fileSeconds(c, f.Kind, f.ID, f.Location), Title: title, Mode: "direct", Quality: "original", Client: client,
	})
	if err != nil {
		logger.Logtype("debug", 0).Err(err).Msg("Jellyfin playback report")
	}

	c.Status(http.StatusNoContent)
}

// --- images -----------------------------------------------------------------

// jfImage serves an item's image: with the app's login, or with the tag
// the item listed for it.
func jfImage(c *gin.Context, p map[string]string) {
	id, typ := p["id"], strings.ToLower(p["type"])

	code, n, ok := parseJFID(id)
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if tag := jfParam(c, "tag"); tag != "" {
		if tag != jfImageTag(id, typ) && tag != jfImageTag(strings.ReplaceAll(id, "-", ""), typ) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	} else if !jfAuth(c) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if _, ok := jfLoad(c, jfGroups(c), id); !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	width := max(jfInt(c, "maxWidth"), jfInt(c, "fillWidth"), jfInt(c, "width"))
	full := width == 0 || width > 450

	file, err := jfImageFile(code, n, typ)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if !full {
		file = coverThumb(file)
	}

	c.Header("Cache-Control", "public, max-age=604800")
	c.File(file)
}

const (
	jfTmdbBackdrop = "https://image.tmdb.org/t/p/w1280"
)

// jfImageFile is the cached file of an image of an item.
func jfImageFile(code byte, n uint64, typ string) (string, error) {
	backdrop := typ == "backdrop" || typ == "thumb" || typ == "art"

	switch code {
	case jfMovie:
		db := database.Getdatarow[uint64](false, "SELECT dbmovie_id FROM movies WHERE id = ?", &n)
		if backdrop {
			return cachedColumnImage("moviebg", db, "SELECT COALESCE(backdrop, '') FROM dbmovies WHERE id = ?", jfTmdbBackdrop)
		}

		return libraryCoverFile("movie", db)
	case jfSeries, jfSeason:
		if code == jfSeason {
			n >>= 16
		}

		db := database.Getdatarow[uint64](false, "SELECT dbserie_id FROM series WHERE id = ?", &n)

		switch {
		case backdrop:
			return cachedColumnImage("seriebg", db, "SELECT COALESCE(fanart, '') FROM dbseries WHERE id = ?", tvdbImageBase)
		case typ == "banner":
			return cachedColumnImage("seriebanner", db, "SELECT COALESCE(banner, '') FROM dbseries WHERE id = ?", tvdbImageBase)
		}

		return libraryCoverFile("series", db)
	case jfEpisode:
		fid := database.Getdatarow[uint64](false, "SELECT min(id) FROM serie_episode_files WHERE serie_episode_id = ?", &n)

		return episodeThumbFile(fid)
	case jfAlbum:
		return libraryCoverFile("album", database.Getdatarow[uint64](false, "SELECT dbalbum_id FROM albums WHERE id = ?", &n))
	case jfTrack:
		return libraryCoverFile("album", database.Getdatarow[uint64](false, "SELECT dbalbum_id FROM album_files WHERE id = ?", &n))
	case jfArtist:
		if img := database.Getdatarow[string](false, "SELECT COALESCE(image_url, '') FROM dbartists WHERE id = ?", &n); strings.HasPrefix(img, "http") {
			if p, err := cachedCover("artist", n, img, img); err == nil {
				return p, nil
			}
		}

		db := database.Getdatarow[uint64](false, "SELECT dbalbum_id FROM dbalbum_artists WHERE dbartist_id = ? ORDER BY position LIMIT 1", &n)

		return libraryCoverFile("album", db)
	case jfBook:
		return libraryCoverFile("audiobook", database.Getdatarow[uint64](false, "SELECT dbaudiobook_id FROM audiobooks WHERE id = ?", &n))
	case jfBookPart:
		return libraryCoverFile("audiobook", database.Getdatarow[uint64](false, "SELECT dbaudiobook_id FROM audiobook_files WHERE id = ?", &n))
	case jfBoxSet:
		db := jfBoxSetFirstMovie(int(n)) //nolint:gosec // TMDB IDs fit
		if backdrop {
			return cachedColumnImage("moviebg", db, "SELECT COALESCE(backdrop, '') FROM dbmovies WHERE id = ?", jfTmdbBackdrop)
		}

		return libraryCoverFile("movie", db)
	case jfPerson:
		if _, profile, ok := jfPersonOf(n); ok && profile != "" {
			u := tmdbImageBase + "w342" + profile
			return cachedCover("person", n, u, u)
		}
	}

	return "", errNoCover
}
