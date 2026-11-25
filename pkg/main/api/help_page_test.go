package api

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestHelpSections: the page explains only what the account can do. t()
// marks its texts for the translation of the page; has compares without.
func TestHelpSections(t *testing.T) {
	ids := func(acc *userAccess) (out []string) {
		for _, s := range helpSections(acc, "") {
			out = append(out, s.id)
		}

		return out
	}

	has := func(acc *userAccess, title string) bool {
		for _, s := range helpSections(acc, "") {
			if slices.ContainsFunc(s.items, func(it helpItem) bool { return strings.Trim(it.title, "") == title }) {
				return true
			}
		}

		return false
	}

	user := &userAccess{Name: "u", Role: database.RoleUser}
	requester := &userAccess{Name: "r", Role: database.RoleRequestingUser, Features: []string{database.FeatureRequest}}
	admin := &userAccess{Name: "a", Role: database.RoleAdmin}

	if got := ids(user); !slices.Equal(got, []string{"help-library", "help-calendar", "help-account"}) {
		t.Errorf("user sections: %v", got)
	}

	for _, title := range []string{"Requests", "Download", "Report a file", "Search at the indexers", "Search missing", "Apps and devices"} {
		if has(user, title) {
			t.Errorf("user is told about %q", title)
		}
	}

	if !slices.Contains(ids(requester), "help-add") || !has(requester, "Requests") || has(requester, "Whole catalogues and manual entries") {
		t.Errorf("requester: %v", ids(requester))
	}

	if has(admin, "Requests") || !has(admin, "Whole catalogues and manual entries") || !has(admin, "Download") || !has(admin, "Apps and devices") {
		t.Error("admin items")
	}
}

func TestHelpPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/help", nil)
	asUser(c, database.RoleUser, "movie_DE")

	HelpPage(c)

	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, `href="#help-calendar"`) || !strings.Contains(body, `id="help-account"`) ||
		strings.Contains(body, "help-add") {
		t.Fatalf("help page: %d %s", w.Code, body)
	}
}

// TestHelpApps: the per-app guides cover the protocols that are on, with
// their addresses, and only for accounts with the apps feature.
func TestHelpApps(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	g := config.GetSettingsGeneral()
	g.ServeJellyfin, g.ServeWebDAV, g.ServeSubsonic, g.ServeOPDS, g.ServeDLNA = true, true, false, true, false

	apps := &userAccess{Name: "a", Role: database.RoleUser, Features: []string{database.FeatureApps}}

	s := helpApps(apps, "https://media.example")
	if s == nil {
		t.Fatal("no apps section")
	}

	addresses := map[string]bool{}
	for _, it := range s.items {
		addresses[it.address] = true
	}

	for _, a := range []string{"https://media.example/jellyfin", "https://media.example/dav", "https://media.example/opds"} {
		if !addresses[a] {
			t.Errorf("no guide with %s: %v", a, addresses)
		}
	}

	if addresses["https://media.example/subsonic"] {
		t.Error("guide for Subsonic, which is off")
	}

	if helpApps(&userAccess{Name: "u", Role: database.RoleUser}, "https://media.example") != nil {
		t.Error("apps section for an account without the apps feature")
	}
}
