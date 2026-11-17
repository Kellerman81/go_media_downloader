package api

import (
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestPageLayouts: admin pages without an account render (no nil access),
// with the sidebar mode script; app mode drops the footer and narrows the
// sidebar to the end-user pages.
func TestPageLayouts(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	admin := renderComponentToString(page("Config", true, false, false))
	if !strings.Contains(admin, "gmd_nav") || !strings.Contains(admin, "<footer") {
		t.Error("admin page without the sidebar mode script or footer")
	}

	user := &userAccess{Name: "anna", Role: database.RoleUser, Features: []string{database.FeatureRequestDelete}, App: true}
	app := renderComponentToString(pageAs(user, "Library", false, false, false))

	if strings.Contains(app, "<footer") || !strings.Contains(app, `class="gmd-app"`) ||
		!strings.Contains(app, "window.gmdCanReport = true") || strings.Contains(app, "/api/admin/config/") {
		t.Error("app page: footer, class, report flag or admin links wrong")
	}
}
