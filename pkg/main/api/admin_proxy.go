// Package api - admin_proxy.go lets the session-authenticated admin UI's own
// JavaScript trigger the small set of apikey-gated external API actions it
// needs (starting a search, cancelling a queue item, etc.) without ever
// embedding the real WebAPIKey in a rendered page. Previously wanted.go,
// calendar.go, and web_grids.go each concatenated
// config.GetSettingsGeneral().WebAPIKey directly into client-side <script>
// blocks - since that key doubles as the login-password fallback (auth.go),
// every page view leaked the effective admin credential into plaintext HTML.
package api

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
)

// adminProxyRoute pairs an allowlisted path pattern with the HTTP method the
// real external-API route actually requires. The method is decided here,
// server-side, never by the caller - forwarding whatever method the browser
// asked for would let a caller upgrade e.g. a GET into the DELETE that
// /api/queue/cancel/:id actually needs, defeating the point of an allowlist.
type adminProxyRoute struct {
	pattern *regexp.Regexp
	method  string
}

// adminProxyRoutes is a fixed allowlist of external-API path patterns this
// proxy will forward. This is NOT a general-purpose SSRF proxy - only the
// exact actions the admin UI's own JS already needs (see
// wanted.go/calendar_modals.go/web_grids.go) are reachable through it, each
// pinned to the real media types/routes/methods this app actually registers.
var adminProxyRoutes = []adminProxyRoute{
	{regexp.MustCompile(`^/api/movies/search/list/\d+$`), http.MethodGet},
	{regexp.MustCompile(`^/api/series/episodes/search/list/\d+$`), http.MethodGet},
	{regexp.MustCompile(`^/api/music/search/list/\d+$`), http.MethodGet},
	{regexp.MustCompile(`^/api/books/search/list/\d+$`), http.MethodGet},
	{regexp.MustCompile(`^/api/audiobooks/search/list/\d+$`), http.MethodGet},
	{regexp.MustCompile(`^/api/queue/cancel/[^/?]+$`), http.MethodDelete},
	{
		regexp.MustCompile(`^/api/(?:movies|series|books|audiobooks|music)/search/list/[^/?]+$`),
		http.MethodGet,
	},
	{
		regexp.MustCompile(`^/api/(?:movies|series|books|audiobooks|music)/search/[^/?]+/missing/[^/?]+$`),
		http.MethodGet,
	},
	{regexp.MustCompile(`^/api/music/discover/series/artist/\d+$`), http.MethodGet},
	{regexp.MustCompile(`^/api/(?:music|audiobooks|books)/job/refresh$`), http.MethodGet},
	{regexp.MustCompile(`^/api/(?:movies|series)/refresh/\d+$`), http.MethodGet},
}

// adminProxyClient is a short-timeout client for the self-loopback call -
// the target is always this same process's own listener, so a hang here
// would indicate the app deadlocking on itself, not real network latency.
var adminProxyClient = &http.Client{Timeout: 30 * time.Second}

// resolveProxyMethod returns the required HTTP method for path (no query
// string) and whether it's allowed through this proxy at all.
func resolveProxyMethod(path string) (string, bool) {
	for _, r := range adminProxyRoutes {
		if r.pattern.MatchString(path) {
			return r.method, true
		}
	}

	return "", false
}

// HandleAdminAPIProxy forwards an allowlisted external-API request to this
// same process's own listener, attaching the real WebAPIKey server-side.
// The caller (already authenticated via the admin session + CSRF, per
// protectAdminRoutes) never needs to know the real key.
func HandleAdminAPIProxy(c *gin.Context) {
	target := c.Query("path")
	if target == "" {
		sendBadRequest(c, "path query parameter is required")
		return
	}

	u, err := url.Parse(target)
	if err != nil || u.IsAbs() || u.Host != "" {
		sendBadRequest(c, "path must be a relative in-app API path")
		return
	}

	method, ok := resolveProxyMethod(u.Path)
	if !ok {
		sendForbidden(c, "path is not allowed through this proxy")
		return
	}

	q := u.Query()
	q.Set(StrApikey, config.GetSettingsGeneral().WebAPIKey)
	u.RawQuery = q.Encode()

	target = "http://127.0.0.1:" + config.GetSettingsGeneral().WebPort + u.String()

	req, err := http.NewRequestWithContext(c.Request.Context(), method, target, nil)
	if err != nil {
		sendBadRequest(c, "failed to build proxied request")
		return
	}

	resp, err := adminProxyClient.Do(req)
	if err != nil {
		sendBadRequest(c, "proxied request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}

	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
