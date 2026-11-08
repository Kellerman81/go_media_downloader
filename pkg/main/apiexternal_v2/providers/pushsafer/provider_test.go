package pushsafer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var device string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		device = r.Form.Get("d")

		if r.Form.Get("k") != "k" {
			io.WriteString(w, `{"status":0,"error":"invalid key"}`)
			return
		}

		io.WriteString(w, `{"status":1,"success":"message transmitted"}`)
	}))
	defer srv.Close()

	p := NewProvider("k", "")
	p.SetBaseURL(srv.URL)

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if device != "a" {
		t.Errorf("empty device list sent as %q, want all (a)", device)
	}

	bad := NewProvider("x", "12")
	bad.SetBaseURL(srv.URL)

	if err := bad.TestConnection(context.Background()); err == nil {
		t.Error("status 0 not reported")
	}
}
