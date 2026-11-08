package pushcut

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var path, key string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, key = r.URL.EscapedPath(), r.Header.Get("API-Key")
		io.WriteString(w, `[{"id":"New Download","title":"x"}]`)
	}))
	defer srv.Close()

	p := NewProvider("k", "New Download")
	p.SetBaseURL(srv.URL)

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if path != "/notifications/New%20Download" || key != "k" {
		t.Errorf("got %s %q", path, key)
	}

	if err := p.TestConnection(context.Background()); err != nil {
		t.Errorf("existing notification: %v", err)
	}

	other := NewProvider("k", "Other")
	other.SetBaseURL(srv.URL)

	if err := other.TestConnection(context.Background()); err == nil {
		t.Error("unknown notification name accepted")
	}
}
