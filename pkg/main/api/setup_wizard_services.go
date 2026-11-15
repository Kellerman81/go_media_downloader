package api

// setup_wizard_services.go is the wizard's Services & API Keys step: the web
// API key (also the admin password), the metadata services' keys the chosen
// media groups use, and Plex/Jellyfin, each with a live test.

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// wizardWeakWebKeys are API keys that also open the web interface to anyone.
var wizardWeakWebKeys = []string{"", "mysecure", "admin"}

// wizardService is a setting of the services step.
type wizardService struct {
	key      string // form field suffix
	label    string
	section  string
	kinds    []string // media kinds that use it (none: always shown)
	level    string   // "recommended" or "optional"
	why      string
	link     string // where to get it
	secret   bool
	restart  bool // takes effect after a restart (providers set up at start)
	get      func(*config.GeneralConfig) string
	set      func(*config.GeneralConfig, string)
	test     func(ctx context.Context, v func(string) string) (bool, string)
	testWith string // the setting whose test covers this one
}

var wizardServices = []wizardService{
	{
		key: "tmdb", label: "TMDb API key", section: "Movies", kinds: []string{"video"}, level: "recommended",
		why:  "Movie metadata: titles, release dates, posters, alternate titles. Without it movie lookups fail.",
		link: "https://www.themoviedb.org/settings/api", secret: true,
		get: func(g *config.GeneralConfig) string { return g.TheMovieDBApiKey },
		set: func(g *config.GeneralConfig, s string) { g.TheMovieDBApiKey = s },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			return wizardHTTPCheck(ctx, "https://api.themoviedb.org/3/configuration?api_key="+url.QueryEscape(v("tmdb")), nil, nil)
		},
	},
	{
		key: "omdb", label: "OMDb API key", section: "Movies", kinds: []string{"video"}, level: "optional",
		why:  "Ratings and a second source for movie data. Without it OMDb lookups fail.",
		link: "https://www.omdbapi.com/apikey.aspx", secret: true,
		get: func(g *config.GeneralConfig) string { return g.OmdbAPIKey },
		set: func(g *config.GeneralConfig, s string) { g.OmdbAPIKey = s },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			return wizardHTTPCheck(ctx, "https://www.omdbapi.com/?i=tt0111161&apikey="+url.QueryEscape(v("omdb")), nil,
				func(body []byte) (bool, string) {
					var r struct{ Response, Error string }
					if json.Unmarshal(body, &r) != nil || r.Response != "True" {
						return false, cmp.Or(r.Error, "unexpected answer")
					}

					return true, ""
				})
		},
	},
	{
		key: "traktid", label: "Trakt client ID", section: "Movies & series", kinds: []string{"video"}, level: "optional",
		why:  "Trakt lists (watchlists, popular, trending) and Trakt metadata. Trakt currently requires a paid account (Trakt VIP) for API access. Create an app at Trakt; authorize it afterwards under Settings → Trakt.",
		link: "https://trakt.tv/oauth/applications",
		get:  func(g *config.GeneralConfig) string { return g.TraktClientID },
		set:  func(g *config.GeneralConfig, s string) { g.TraktClientID = s },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			return wizardHTTPCheck(ctx, "https://api.trakt.tv/movies/trending?limit=1",
				map[string]string{"trakt-api-version": "2", "trakt-api-key": v("traktid"), "Content-Type": "application/json"}, nil)
		},
	},
	{
		key: "traktsecret", label: "Trakt client secret", section: "Movies & series", kinds: []string{"video"}, level: "optional",
		why: "Needed with the client ID to authorize Trakt (checked when you authorize).", secret: true,
		get: func(g *config.GeneralConfig) string { return g.TraktClientSecret },
		set: func(g *config.GeneralConfig, s string) { g.TraktClientSecret = s },
	},
	{
		key: "lastfm", label: "Last.fm API key", section: "Music", kinds: []string{"music"}, level: "recommended",
		why: "Last.fm chart and artist lists, and artist/album information.", link: "https://www.last.fm/api/account/create",
		secret: true, restart: true,
		get: func(g *config.GeneralConfig) string { return g.LastFMAPIKey },
		set: func(g *config.GeneralConfig, s string) { g.LastFMAPIKey = s },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			return wizardHTTPCheck(ctx, "https://ws.audioscrobbler.com/2.0/?method=chart.gettopartists&limit=1&format=json&api_key="+url.QueryEscape(v("lastfm")), nil,
				func(body []byte) (bool, string) {
					var r struct {
						Error   int    `json:"error"`
						Message string `json:"message"`
					}
					if json.Unmarshal(body, &r) == nil && r.Error != 0 {
						return false, r.Message
					}

					return true, ""
				})
		},
	},
	{
		key: "acoustid", label: "AcoustID API key", section: "Music", kinds: []string{"music"}, level: "optional",
		why: "Identifies tracks by their sound (with fpcalc from the tools step).", link: "https://acoustid.org/new-application",
		secret: true, restart: true,
		get: func(g *config.GeneralConfig) string { return g.AcoustIDAPIKey },
		set: func(g *config.GeneralConfig, s string) { g.AcoustIDAPIKey = s },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			// A wrong fingerprint is fine; only error 4 means the key is wrong.
			return wizardHTTPCheck(ctx, "https://api.acoustid.org/v2/lookup?format=json&duration=10&fingerprint=AQAA&client="+url.QueryEscape(v("acoustid")),
				nil, func(body []byte) (bool, string) {
					var r struct {
						Error struct {
							Code    int    `json:"code"`
							Message string `json:"message"`
						} `json:"error"`
					}
					if json.Unmarshal(body, &r) == nil && r.Error.Code == 4 {
						return false, r.Error.Message
					}

					return true, ""
				}, http.StatusBadRequest)
		},
	},
	{
		key: "discogs", label: "Discogs token", section: "Music", kinds: []string{"music"}, level: "optional",
		why: "Higher Discogs limits for release data (Discogs also works without it).", link: "https://www.discogs.com/settings/developers",
		secret: true, restart: true,
		get: func(g *config.GeneralConfig) string { return g.DiscogsToken },
		set: func(g *config.GeneralConfig, s string) { g.DiscogsToken = s },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			return wizardHTTPCheck(ctx, "https://api.discogs.com/oauth/identity", map[string]string{"Authorization": "Discogs token=" + v("discogs")}, nil)
		},
	},
	{
		key: "theaudiodb", label: "TheAudioDB API key", section: "Music", kinds: []string{"music"}, level: "optional",
		why: "Artist images; the free public key is used when empty.", link: "https://www.theaudiodb.com/api_guide.php",
		secret: true, restart: true,
		get: func(g *config.GeneralConfig) string { return g.TheAudioDBAPIKey },
		set: func(g *config.GeneralConfig, s string) { g.TheAudioDBAPIKey = s },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			return wizardHTTPCheck(ctx, "https://www.theaudiodb.com/api/v1/json/"+url.PathEscape(v("theaudiodb"))+"/search.php?s=coldplay", nil, nil)
		},
	},
	{
		key: "goodreads", label: "Goodreads API key", section: "Books", kinds: []string{"book"}, level: "optional",
		why: "Only if you still have one: Goodreads no longer issues keys. Books use OpenLibrary without a key.", secret: true, restart: true,
		get: func(g *config.GeneralConfig) string { return g.GoodreadsAPIKey },
		set: func(g *config.GeneralConfig, s string) { g.GoodreadsAPIKey = s },
	},
	{
		key: "plexurl", label: "Plex server URL", section: "Media servers", kinds: []string{"video", "music"}, level: "optional",
		why: "\"Open in Plex\" links, Plex watchlists and Plex as a metadata source. Example: http://192.168.1.10:32400",
		get: func(g *config.GeneralConfig) string { return g.PlexServerURL },
		set: func(g *config.GeneralConfig, s string) { g.PlexServerURL = strings.TrimRight(s, "/") },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			if v("plextoken") == "" {
				return false, "enter the token too"
			}

			return wizardHTTPCheck(ctx, strings.TrimRight(v("plexurl"), "/")+"/library/sections",
				map[string]string{"X-Plex-Token": v("plextoken"), "Accept": "application/json"}, nil)
		},
	},
	{
		key: "plextoken", label: "Plex token", section: "Media servers", kinds: []string{"video", "music"}, level: "optional",
		why:  "Find it via \"View XML\" of any item in Plex Web (X-Plex-Token in the address).",
		link: "https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/", secret: true, testWith: "plexurl",
		get: func(g *config.GeneralConfig) string { return g.PlexToken },
		set: func(g *config.GeneralConfig, s string) { g.PlexToken = s },
	},
	{
		key: "jellyfinurl", label: "Jellyfin server URL", section: "Media servers", kinds: []string{"video", "music"}, level: "optional",
		why: "\"Open in Jellyfin\" links, Jellyfin watchlists and Jellyfin as a metadata source. Example: http://192.168.1.10:8096",
		get: func(g *config.GeneralConfig) string { return g.JellyfinServerURL },
		set: func(g *config.GeneralConfig, s string) { g.JellyfinServerURL = strings.TrimRight(s, "/") },
		test: func(ctx context.Context, v func(string) string) (bool, string) {
			if v("jellyfintoken") == "" {
				return false, "enter the API key too"
			}

			return wizardHTTPCheck(ctx, strings.TrimRight(v("jellyfinurl"), "/")+"/System/Info",
				map[string]string{"X-Emby-Token": v("jellyfintoken")}, nil)
		},
	},
	{
		key: "jellyfintoken", label: "Jellyfin API key", section: "Media servers", kinds: []string{"video", "music"}, level: "optional",
		why: "Create one in Jellyfin under Dashboard → API Keys.", secret: true, testWith: "jellyfinurl",
		get: func(g *config.GeneralConfig) string { return g.JellyfinToken },
		set: func(g *config.GeneralConfig, s string) { g.JellyfinToken = s },
	},
}

// wizardHTTPCheck calls a service; a 2xx answer (or one of okCodes) passes,
// check looks at the body when given. 401/403 mean a wrong key.
func wizardHTTPCheck(ctx context.Context, rawURL string, headers map[string]string,
	check func([]byte) (bool, string), okCodes ...int,
) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, wizardTestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false, err.Error()
	}

	req.Header.Set("User-Agent", "go_media_downloader setup wizard")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "not reachable: " + err.Error()
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return false, fmt.Sprintf("rejected (HTTP %d): wrong key or token", resp.StatusCode)
	case resp.StatusCode/100 != 2 && !slices.Contains(okCodes, resp.StatusCode):
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	if check != nil {
		return check(body)
	}

	return true, ""
}

// wizardServicesFor are the services the chosen media kinds use.
func wizardServicesFor(kinds map[string]bool) []wizardService {
	out := make([]wizardService, 0, len(wizardServices))
	for _, s := range wizardServices {
		if len(s.kinds) == 0 || wizardAnyKind(kinds, s.kinds...) {
			out = append(out, s)
		}
	}

	return out
}

func renderWizardStepServices(csrfToken string, state wizardState) gomponents.Node {
	g := config.GetSettingsGeneral()
	if g == nil {
		g = &config.GeneralConfig{}
	}

	kinds := wizardMediaKinds(state)

	weak := slices.Contains(wizardWeakWebKeys, g.WebAPIKey)

	web := html.Div(html.Class("card mb-3"), html.Div(html.Class("card-body"),
		html.H5(html.Class("card-title"), gomponents.Text("Web access")),
		gomponents.If(weak, gomponents.Raw(renderAlert("The web API key is still the default. It is also the password of the web interface (user \"admin\") and of the REST API: change it.", "warning"))),
		html.Label(html.Class("form-label"), html.For("wizardWebKey"), gomponents.Text("Web API key = admin password")),
		html.Div(html.Class("input-group"),
			html.Input(html.Type("text"), html.Class("form-control font-monospace"), html.ID("wizardWebKey"),
				html.Name("wizard_svc_webapikey"), html.Value(g.WebAPIKey), gomponents.Attr("autocomplete", "off")),
			html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), gomponents.Text("Generate"),
				gomponents.Attr("onclick", "wizardGenerateKey()"))),
		html.P(html.Class("small text-muted mt-1 mb-0"),
			gomponents.Text("Log in with user \"admin\" and this key. A change applies at once and signs out remembered logins: note the new key before saving.")),
	))

	sections := []gomponents.Node{web}
	order := []string{}
	bySection := map[string][]gomponents.Node{}

	for _, s := range wizardServicesFor(kinds) {
		if _, ok := bySection[s.section]; !ok {
			order = append(order, s.section)
		}

		badge := map[string]string{"recommended": "bg-warning text-dark", "optional": "bg-secondary"}[s.level]
		typ := "text"
		if s.secret {
			typ = "password"
		}

		bySection[s.section] = append(bySection[s.section], html.Div(html.Class("col-md-6 mb-3"),
			html.Label(html.Class("form-label mb-1"), html.For("wizardSvc"+s.key), gomponents.Text(s.label+" "),
				html.Span(html.Class("badge "+badge), gomponents.Text(s.level))),
			html.Input(html.Type(typ), html.Class("form-control"), html.ID("wizardSvc"+s.key), html.Name("wizard_svc_"+s.key),
				html.Value(s.get(g)), gomponents.Attr("autocomplete", "off")),
			html.Div(html.Class("small text-muted mt-1"), gomponents.Text(s.why+" "),
				gomponents.If(s.link != "", html.A(html.Href(s.link), html.Target("_blank"), html.Rel("noopener"), gomponents.Text("Get it here")))),
		))
	}

	keyless := map[string]string{
		"Series":     "TheTVDB and TVmaze need no key.",
		"Music":      "MusicBrainz, Deezer and iTunes need no key.",
		"Audiobooks": "Audible and Audnexus need no key.",
		"Books":      "OpenLibrary needs no key.",
	}

	for _, sec := range order {
		sections = append(sections, html.Div(html.Class("card mb-3"), html.Div(html.Class("card-body"),
			html.H5(html.Class("card-title"), gomponents.Text(sec)),
			html.Div(html.Class("row"), gomponents.Group(bySection[sec])))))
	}

	if kinds["video"] {
		sections = append(sections, renderWizardLimits(g), renderWizardImdb(state))
	}

	// Per chosen group (all when none were chosen): movies and series share
	// the video kind, the note is about series only.
	chosen := map[string]bool{}
	for _, d := range wizardEnabledGroups(state) {
		chosen[d.label] = true
	}

	var notes []string

	for _, k := range []string{"Series", "Audiobooks", "Books", "Music"} {
		if len(chosen) == 0 || chosen[k] {
			notes = append(notes, k+": "+keyless[k])
		}
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-key", fmt.Sprintf("Step %d: Services & API Keys", wizStepServices),
			"The web interface's password and the online services your media groups use. Test checks every entered key live."),
		html.Form(
			html.Class("config-form"),
			html.ID("wizardStepForm"),
			gomponents.Group(sections),
			gomponents.If(len(notes) > 0, html.P(html.Class("small text-muted"), gomponents.Text(strings.Join(notes, " · ")))),
			html.Div(
				html.Class("form-group submit-group mt-3"),
				html.Button(html.Class("btn btn-outline-info me-2"), html.Type("button"), gomponents.Text("Test entered keys"),
					hx.Post("/api/admin/wizard/test/services"), hx.Target("#wizardTestResult"), hx.Swap("innerHTML"),
					hx.Include("#wizardStepForm, #wizardCarry"), hx.Headers(wizardCSRFHeader(csrfToken))),
				wizardSaveButton(wizStepServices, csrfToken),
				wizardNextButton(wizStepServices, "Skip"),
			),
			html.Div(html.ID("wizardTestResult"), html.Class("mt-2")),
		),
		html.Script(gomponents.Raw(`function wizardGenerateKey() {
	// 24 characters of 62, rejecting bytes that would bias the choice.
	var abc = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789', out = '';
	while (out.length < 24) {
		var a = new Uint8Array(32); crypto.getRandomValues(a);
		for (var i = 0; i < a.length && out.length < 24; i++) { if (a[i] < 248) { out += abc[a[i] % 62]; } }
	}
	document.getElementById('wizardWebKey').value = out;
}`)),
	)
}

// HandleWizardTestServices tests every entered key at once, without saving
// (/admin/wizard/test/services).
func HandleWizardTestServices(c *gin.Context) {
	_ = c.Request.ParseForm()

	v := func(key string) string { return strings.TrimSpace(c.PostForm("wizard_svc_" + key)) }

	type result struct {
		label, msg string
		ok         bool
	}

	var (
		results = make([]result, len(wizardServices))
		wg      sync.WaitGroup
	)

	for i, s := range wizardServices {
		if s.test == nil || v(s.key) == "" {
			continue
		}

		wg.Add(1)

		go func(i int, s wizardService) {
			defer wg.Done()

			ok, msg := s.test(c.Request.Context(), v)
			results[i] = result{label: s.label, ok: ok, msg: msg}
		}(i, s)
	}

	wg.Wait()

	items := make([]gomponents.Node, 0, len(results))
	for _, r := range results {
		if r.label == "" {
			continue
		}

		icon, text := "fa-circle-check text-success", "works"
		if !r.ok {
			icon, text = "fa-circle-xmark text-danger", r.msg
		}

		items = append(items, html.Li(html.I(html.Class("fa-solid "+icon+" me-2")), html.Strong(gomponents.Text(r.label+": ")), gomponents.Text(text)))
	}

	c.Header("Content-Type", "text/html")

	if len(items) == 0 {
		c.String(http.StatusOK, renderAlert("Enter a key first.", "warning"))
		return
	}

	c.String(http.StatusOK, renderComponentToString(html.Ul(html.Class("list-unstyled mb-0"), gomponents.Group(items))))
}

func wizardSaveServicesStep(c *gin.Context) string {
	g := config.GetSettingsGeneral()
	if g == nil {
		return renderAlert("Configuration not loaded.", "danger")
	}

	updated := *g
	restart := false

	if k := strings.TrimSpace(c.PostForm("wizard_svc_webapikey")); k != "" {
		updated.WebAPIKey = k
	}

	for _, s := range wizardServices {
		v, ok := c.GetPostForm("wizard_svc_" + s.key)
		if !ok {
			continue
		}

		if v = strings.TrimSpace(v); v != s.get(g) && s.restart {
			restart = true
		}

		s.set(&updated, v)
	}

	for _, l := range wizardLimits {
		p := "wizard_limit_" + l.key + "_"
		if _, ok := c.GetPostForm(p + "calls"); !ok {
			continue
		}

		num := func(field string, lo, hi int) int {
			n, _ := strconv.Atoi(strings.TrimSpace(c.PostForm(p + field)))
			return min(max(n, lo), hi)
		}

		l.set(&updated, num("calls", 1, 1000), num("seconds", 1, 255), num("timeout", 1, 600))
	}

	if err := config.UpdateCfgEntryAny(&updated); err != nil {
		return renderAlert("Failed to save: "+err.Error(), "danger")
	}

	if err := wizardSaveImdb(c); err != nil {
		return renderAlert("Failed to save the IMDb settings: "+err.Error(), "danger")
	}

	// TMDb, OMDb, Trakt and the media servers pick their keys up now.
	providers.RegisterMetadataProviders()
	providers.RegisterMediaServerProviders()

	msg := "Services saved."
	if updated.WebAPIKey != g.WebAPIKey {
		msg += " The web password is now the new API key."
	}

	if restart {
		msg += " Restart the downloader once to use the changed music/book keys."
	}

	return renderAlert(msg, "success")
}

// wizardLimit is a metadata service's request limit: calls per seconds and a
// timeout.
type wizardLimit struct {
	key, label string
	get        func(*config.GeneralConfig) (calls, seconds, timeout int)
	set        func(g *config.GeneralConfig, calls, seconds, timeout int)
}

// wizardLimits are the movie/series services' limits. 2 calls per second with
// a 20 s timeout is what long-running setups use; music and book services
// keep their built-in limits (MusicBrainz' 1 per second and so on).
var wizardLimits = []wizardLimit{
	{"tmdb", "TMDb",
		func(g *config.GeneralConfig) (int, int, int) {
			return g.TmdbLimiterCalls, int(g.TmdbLimiterSeconds), int(g.TmdbTimeoutSeconds)
		},
		func(g *config.GeneralConfig, c, s, t int) {
			g.TmdbLimiterCalls, g.TmdbLimiterSeconds, g.TmdbTimeoutSeconds = c, uint8(s), uint16(t)
		}},
	{"omdb", "OMDb",
		func(g *config.GeneralConfig) (int, int, int) {
			return g.OmdbLimiterCalls, int(g.OmdbLimiterSeconds), int(g.OmdbTimeoutSeconds)
		},
		func(g *config.GeneralConfig, c, s, t int) {
			g.OmdbLimiterCalls, g.OmdbLimiterSeconds, g.OmdbTimeoutSeconds = c, uint8(s), uint16(t)
		}},
	{"trakt", "Trakt",
		func(g *config.GeneralConfig) (int, int, int) {
			return g.TraktLimiterCalls, int(g.TraktLimiterSeconds), int(g.TraktTimeoutSeconds)
		},
		func(g *config.GeneralConfig, c, s, t int) {
			g.TraktLimiterCalls, g.TraktLimiterSeconds, g.TraktTimeoutSeconds = c, uint8(s), uint16(t)
		}},
	{"tvdb", "TheTVDB",
		func(g *config.GeneralConfig) (int, int, int) {
			return g.TvdbLimiterCalls, int(g.TvdbLimiterSeconds), int(g.TvdbTimeoutSeconds)
		},
		func(g *config.GeneralConfig, c, s, t int) {
			g.TvdbLimiterCalls, g.TvdbLimiterSeconds, g.TvdbTimeoutSeconds = c, uint8(s), uint16(t)
		}},
}

// wizardLimitValues are a service's limits to show: the configured ones, the
// recommendation where unset.
func wizardLimitValues(l wizardLimit, g *config.GeneralConfig) (calls, seconds, timeout int) {
	calls, seconds, timeout = l.get(g)

	return cmp.Or(calls, 2), cmp.Or(seconds, 1), cmp.Or(timeout, 20)
}

func renderWizardLimits(g *config.GeneralConfig) gomponents.Node {
	num := func(name string, v int) gomponents.Node {
		return html.Input(html.Type("number"), html.Min("0"), html.Class("form-control form-control-sm"), html.Name(name),
			html.Value(strconv.Itoa(v)), html.Style("max-width:7rem"))
	}

	rows := make([]gomponents.Node, 0, len(wizardLimits))
	for _, l := range wizardLimits {
		c, s, t := wizardLimitValues(l, g)
		p := "wizard_limit_" + l.key + "_"
		rows = append(rows, html.Tr(html.Td(gomponents.Text(l.label)),
			html.Td(num(p+"calls", c)), html.Td(num(p+"seconds", s)), html.Td(num(p+"timeout", t))))
	}

	return html.Div(html.Class("card mb-3"), html.Div(html.Class("card-body"),
		html.H5(html.Class("card-title"), gomponents.Text("Request limits")),
		html.P(html.Class("small text-muted"), gomponents.Text(
			"How often the metadata services may be asked. Too many requests get you blocked; 2 calls per second with a 20 s timeout is a safe choice. "+
				"Music and book services use built-in limits (MusicBrainz allows 1 call per second).")),
		html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(html.Th(gomponents.Text("Service")), html.Th(gomponents.Text("Calls")),
				html.Th(gomponents.Text("Per seconds")), html.Th(gomponents.Text("Timeout (s)")))),
			html.TBody(gomponents.Group(rows)))))
}

// wizardRecommendedImdb is the IMDb import suggestion: the title types the
// chosen groups need, English regions plus German for a German group, and
// the original titles (\N).
func wizardRecommendedImdb(state wizardState) config.ImdbConfig {
	cfg := config.ImdbConfig{
		Indexedtypes:     []string{"movie", "tvMovie", "video"},
		Indexedlanguages: []string{"US", "UK", "\\N"},
		Indexfull:        true,
		UseMemory:        true,
		ImdbIDSize:       12000000,
	}

	for _, d := range wizardEnabledGroups(state) {
		if d.key == "series" {
			cfg.Indexedtypes = append(cfg.Indexedtypes, "tvSeries")
		}

		if d.qualityKind == "video" && wizardDefaultLang(d, state.Groups[d.key].Name) == "de" && !slices.Contains(cfg.Indexedlanguages, "DE") {
			cfg.Indexedlanguages = append(cfg.Indexedlanguages, "DE")
		}
	}

	return cfg
}

func renderWizardImdb(state wizardState) gomponents.Node {
	rec := wizardRecommendedImdb(state)

	check := func(name, label, hint string, on bool) gomponents.Node {
		return html.Div(html.Class("form-check"),
			html.Input(html.Type("checkbox"), html.Class("form-check-input"), html.ID("wizImdb"+name), html.Name("wizard_imdb_"+name),
				html.Value("1"), gomponents.If(on, html.Checked())),
			html.Label(html.Class("form-check-label"), html.For("wizImdb"+name), gomponents.Text(label)),
			html.Div(html.Class("form-text mt-0"), gomponents.Text(hint)))
	}

	return html.Div(html.Class("card mb-3"), html.Div(html.Class("card-body"),
		html.H5(html.Class("card-title"), gomponents.Text("IMDb database (movies & series)")),
		html.P(html.Class("small text-muted"), gomponents.Text(
			"A local copy of IMDb's title lists matches releases to movies and series without asking a service. It is imported by init_imdb (System Tools) on its schedule.")),
		html.Div(html.Class("row"),
			html.Div(html.Class("col-md-6 mb-2"),
				html.Label(html.Class("form-label mb-1"), gomponents.Text("Title types")),
				html.Input(html.Type("text"), html.Class("form-control"), html.Name("wizard_imdb_types"), html.Value(strings.Join(rec.Indexedtypes, ", "))),
				html.Div(html.Class("form-text"), gomponents.Text("movie, tvMovie, tvSeries, video, short, tvEpisode …"))),
			html.Div(html.Class("col-md-6 mb-2"),
				html.Label(html.Class("form-label mb-1"), gomponents.Text("Regions of alternate titles")),
				html.Input(html.Type("text"), html.Class("form-control"), html.Name("wizard_imdb_regions"), html.Value(strings.Join(rec.Indexedlanguages, ", "))),
				html.Div(html.Class("form-text"), gomponents.Text("US, UK, DE, FR … — \\N keeps the original titles (no region)."))),
		),
		check("full", "Full index", "Also ratings and more details; bigger database.", rec.Indexfull),
		check("memory", "Import in memory", "Much faster import, but needs 1-4 GB RAM while it runs.", rec.UseMemory),
	))
}

// wizardSaveImdb stores the IMDb import settings of the form.
func wizardSaveImdb(c *gin.Context) error {
	if _, ok := c.GetPostForm("wizard_imdb_types"); !ok {
		return nil
	}

	split := func(field string) []string {
		var out []string
		for _, v := range strings.Split(c.PostForm(field), ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}

		return out
	}

	size := 12000000
	if old := config.GetSettingsImdb(); old != nil && old.ImdbIDSize > 0 {
		size = old.ImdbIDSize
	}

	return config.UpdateCfgEntryAny(config.ImdbConfig{
		Indexedtypes:     split("wizard_imdb_types"),
		Indexedlanguages: split("wizard_imdb_regions"),
		Indexfull:        c.PostForm("wizard_imdb_full") != "",
		UseMemory:        c.PostForm("wizard_imdb_memory") != "",
		ImdbIDSize:       size,
	})
}
