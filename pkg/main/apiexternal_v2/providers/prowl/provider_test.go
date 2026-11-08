package prowl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var path, event, key string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		path, event, key = r.URL.Path, r.Form.Get("event"), r.Form.Get("apikey")

		if key != "k" {
			w.WriteHeader(http.StatusUnauthorized)
		}

		w.Write([]byte(`<?xml version="1.0"?><prowl/>`))
	}))
	defer srv.Close()

	p := NewProvider("k")
	p.SetBaseURL(srv.URL)

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if path != "/add" || event != "T" {
		t.Errorf("got %s %q", path, event)
	}

	bad := NewProvider("x")
	bad.SetBaseURL(srv.URL)

	if err := bad.TestConnection(context.Background()); err == nil || path != "/verify" {
		t.Errorf("bad key accepted at %s", path)
	}
}
