package simplepush

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		if r.Form.Get("key") != "k" || r.Form.Get("msg") == "" {
			io.WriteString(w, `{"status":"error","message":"invalid key"}`)
			return
		}

		io.WriteString(w, `{"status":"OK"}`)
	}))
	defer srv.Close()

	p := NewProvider("k")
	p.SetBaseURL(srv.URL)

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	bad := NewProvider("x")
	bad.SetBaseURL(srv.URL)

	if err := bad.TestConnection(context.Background()); err == nil {
		t.Error("error status not reported")
	}
}
