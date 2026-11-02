package pushover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func newTestServer(t *testing.T, got *url.Values, path *string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		*got = r.PostForm
		*path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"request":"req"}`))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestEmergencyDefaultsWithoutOptionsAndTruncation(t *testing.T) {
	var (
		form url.Values
		path string
	)

	srv := newTestServer(t, &form, &path)
	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL}, "tok", "usr")

	_, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{
		Title:    strings.Repeat("ä", 300),
		Message:  strings.Repeat("x", 2000),
		Priority: apiexternal_v2.PriorityEmergency,
	})
	if err != nil {
		t.Fatal(err)
	}

	if form.Get("retry") != "60" || form.Get("expire") != "3600" {
		t.Errorf("retry/expire = %q/%q", form.Get("retry"), form.Get("expire"))
	}

	if n := utf8.RuneCountInString(form.Get("title")); n != maxTitleLen {
		t.Errorf("title length = %d", n)
	}

	if n := len(form.Get("message")); n != maxMessageLen {
		t.Errorf("message length = %d", n)
	}
}

func TestTestConnectionRespectsBaseURL(t *testing.T) {
	var (
		form url.Values
		path string
	)

	srv := newTestServer(t, &form, &path)
	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL}, "tok", "usr")

	if err := p.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}

	if path != "/users/validate.json" || form.Get("token") != "tok" {
		t.Errorf("path=%q token=%q", path, form.Get("token"))
	}
}
