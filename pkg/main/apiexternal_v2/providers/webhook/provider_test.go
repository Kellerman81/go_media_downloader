package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/goccy/go-json"
)

func TestProvider(t *testing.T) {
	var (
		got        Payload
		user, pass string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()

	p := NewProvider("http://bob:secret@" + srv.Listener.Addr().String() + "/hook")
	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if got.EventType != "Notification" || got.Title != "T" || got.Message != "M" || user != "bob" || pass != "secret" {
		t.Errorf("got %+v as %s:%s", got, user, pass)
	}

	if err := p.TestConnection(context.Background()); err != nil || got.EventType != "Test" {
		t.Errorf("test connection: %v, event %q", err, got.EventType)
	}
}
