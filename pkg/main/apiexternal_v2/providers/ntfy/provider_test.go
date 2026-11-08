package ntfy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var path, auth, body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, auth, body = r.URL.Path, r.Header.Get("Authorization"), string(b)

		io.WriteString(w, `{"healthy":true}`)
	}))
	defer srv.Close()

	p := NewProvider(srv.URL+"/", "news", "tk_1")
	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "Grüße", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if path != "/" || auth != "Bearer tk_1" || body != `{"message":"M","title":"Grüße","topic":"news"}` {
		t.Errorf("got %s %q %s", path, auth, body)
	}

	if err := p.TestConnection(context.Background()); err != nil || path != "/v1/health" {
		t.Errorf("test connection: %v at %s", err, path)
	}

	NewProvider(srv.URL, "news", "user:pw").SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Message: "M"})

	if auth != "Basic dXNlcjpwdw==" {
		t.Errorf("user:password token sent as %q", auth)
	}
}
