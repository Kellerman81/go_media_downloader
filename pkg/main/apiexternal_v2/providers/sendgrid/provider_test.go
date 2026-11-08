package sendgrid

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var auth, body string

	scopes := `{"scopes":["mail.send"]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		auth, body = r.Header.Get("Authorization"), string(b)

		if r.URL.Path == "/scopes" {
			io.WriteString(w, scopes)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	p, err := NewProvider("k", "Media <m@x.de>", []string{"a@x.de"})
	if err != nil {
		t.Fatal(err)
	}

	p.SetBaseURL(srv.URL)

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	want := `{"content":[{"type":"text/plain","value":"M"}],"from":{"email":"m@x.de","name":"Media"},` +
		`"personalizations":[{"to":[{"email":"a@x.de"}]}],"subject":"T"}`
	if auth != "Bearer k" || body != want {
		t.Errorf("got %q %s", auth, body)
	}

	if err := p.TestConnection(context.Background()); err != nil {
		t.Errorf("key with mail.send: %v", err)
	}

	scopes = `{"scopes":["alerts.read"]}`
	if err := p.TestConnection(context.Background()); err == nil {
		t.Error("key without mail.send accepted")
	}
}
