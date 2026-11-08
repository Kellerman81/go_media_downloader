package providers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestNotifyEvent(t *testing.T) {
	csvFile := filepath.Join(t.TempDir(), "n.csv")

	got := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- string(b)
	}))
	defer srv.Close()

	hook := &config.NotificationConfig{Name: "hook", NotificationType: "webhook", ServerURL: srv.URL}

	p, err := NewNotificationProvider(hook)
	if err != nil {
		t.Fatal(err)
	}

	oldCM, _ := apiexternal_v2.GetGlobalClientManager()
	cm := apiexternal_v2.NewClientManager()
	cm.ReplaceNotificationProviders(map[string]apiexternal_v2.NotificationProvider{"hook": p})
	apiexternal_v2.SetGlobalClientManager(cm)
	t.Cleanup(func() { apiexternal_v2.SetGlobalClientManager(oldCM) })

	csv := &config.NotificationConfig{Name: "csv", NotificationType: "csv", Outputto: csvFile}
	cfgp := &config.MediaTypeConfig{Notification: []config.MediaNotificationConfig{
		{Event: EventAddedToList, CfgNotification: csv, Message: "{{.Title}};{{.Prefix}}", ReplacedPrefix: "p1"},
		{Event: EventAddedToDB, CfgNotification: csv, Message: "wrong event"},
		{Event: EventAddedToList, CfgNotification: hook, Title: "New: {{.Title}}", Message: "{{.Title}}"},
	}}

	if !HasEventNotification(cfgp, "ADDED_TO_LIST") || HasEventNotification(cfgp, EventAddedDownload) {
		t.Error("HasEventNotification wrong")
	}

	data := &struct{ Title, Prefix string }{Title: "Heat"}
	NotifyEvent(cfgp, EventAddedToList, data, func(n *config.MediaNotificationConfig) { data.Prefix = n.ReplacedPrefix })

	if b, _ := os.ReadFile(csvFile); string(b) != "Heat;p1\n" {
		t.Errorf("csv = %q", b)
	}

	select {
	case body := <-got:
		if want := `"title":"New: Heat","message":"Heat"`; !strings.Contains(body, want) {
			t.Errorf("webhook body %s, want %s", body, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook not sent")
	}
}
