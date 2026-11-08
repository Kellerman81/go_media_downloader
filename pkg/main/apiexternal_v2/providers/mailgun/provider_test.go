package mailgun

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var (
		path, user, key string
		to              []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		path, to = r.URL.Path, r.PostForm["to"]
		user, key, _ = r.BasicAuth()
	}))
	defer srv.Close()

	if _, err := NewProvider("k", "", "no-domain", nil); err == nil {
		t.Error("from address without domain accepted")
	}

	p, err := NewProvider("k", srv.URL+"/v3/", "Media <media@mg.example.com>", []string{"a@x.de", "b@x.de"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if path != "/v3/mg.example.com/messages" || user != "api" || key != "k" || !slices.Equal(to, []string{"a@x.de", "b@x.de"}) {
		t.Errorf("got %s as %s:%s to %v", path, user, key, to)
	}

	if err := p.TestConnection(context.Background()); err != nil || path != "/v3/domains/mg.example.com" {
		t.Errorf("test connection: %v at %s", err, path)
	}
}
