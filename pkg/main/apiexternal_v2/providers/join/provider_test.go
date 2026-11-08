package join

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var (
		path  string
		query url.Values
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.Query()

		if query.Get("apikey") != "k" {
			io.WriteString(w, `{"success":false,"errorMessage":"bad key"}`)
			return
		}

		io.WriteString(w, `{"success":true}`)
	}))
	defer srv.Close()

	p := NewProvider("k", "")
	p.SetBaseURL(srv.URL)

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if path != "/messaging/v1/sendPush" || query.Get("deviceId") != "group.all" || query.Get("text") != "M" {
		t.Errorf("got %s %v", path, query)
	}

	bad := NewProvider("x", "d1,d2")
	bad.SetBaseURL(srv.URL)

	if err := bad.TestConnection(context.Background()); err == nil || path != "/registration/v1/listDevices" {
		t.Errorf("bad key accepted at %s: %v", path, err)
	}
}
