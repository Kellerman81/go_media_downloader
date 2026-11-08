package discord

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestSendNotification(t *testing.T) {
	var method, body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, body = r.Method, string(b)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := NewProvider(srv.URL + "/api/webhooks/1/tok")
	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if method != http.MethodPost || body != `{"embeds":[{"title":"T","description":"M"}]}` {
		t.Errorf("got %s %s", method, body)
	}

	if err := p.TestConnection(context.Background()); err != nil || method != http.MethodGet {
		t.Errorf("test connection: %v, method %s", err, method)
	}
}
