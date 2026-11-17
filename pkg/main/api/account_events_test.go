package api

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
)

// TestAccountEventNotification: a report sends the media_reported
// notification of the file's media group, with the report's fields and a
// link to the Reports page; other groups' notifications stay quiet.
func TestAccountEventNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)

	csv := filepath.Join(t.TempDir(), "events.csv")
	de, en := mediaGroup("DE"), mediaGroup("EN")
	de[0].Notification = []config.MediaNotificationConfig{{MapNotification: "log", Event: providers.EventReported,
		Message: "{{.User}} reported {{.Title}} ({{.Reason}}: {{.Note}}) {{.File}} in {{.Configuration}}/{{.List}} {{.URL}}"}}
	en[0].Notification = []config.MediaNotificationConfig{{MapNotification: "log", Event: providers.EventReported, Message: "EN {{.Title}}"}}

	useTestConfig(t, config.MainConfig{
		Media:        config.MediaConfig{Movies: append(de, en...)},
		Notification: []config.NotificationConfig{{Name: "log", NotificationType: "csv", Outputto: csv}},
	})

	for _, s := range []string{
		"insert into dbmovies (id, title) values (2, 'Deutsch')",
		"insert into movies (id, dbmovie_id, listname) values (2, 2, 'DE List')",
		"insert into movie_files (id, movie_id, dbmovie_id, location) values (12, 2, 2, '/media/de.mkv')",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	reporter := &Session{UserID: "robin", Role: database.RoleUser, Groups: []string{"movie_DE"}, Features: []string{database.FeatureRequestDelete}}
	if w := callAs(reporter, HandleLibraryReport, url.Values{"type": {"movie"}, "id": {"12"}, "reason": {"language"}, "note": {"English audio"}}); w.Code != http.StatusOK {
		t.Fatalf("report: %d %s", w.Code, w.Body)
	}

	b, err := os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}

	want := "robin reported Deutsch (Wrong language: English audio) de.mkv in DE/DE List http://example.com/api/admin/reports"
	if got := strings.TrimSpace(string(b)); !strings.Contains(got, want) || strings.Contains(got, "EN ") {
		t.Errorf("notification %q, want %q", got, want)
	}
}
