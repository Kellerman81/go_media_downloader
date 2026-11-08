package telegram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestSendNotification(t *testing.T) {
	var path, body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, body = r.URL.Path, string(b)

		if strings.Contains(body, "fail") {
			io.WriteString(w, `{"ok":false,"description":"Bad Request: chat not found"}`)
			return
		}

		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	p := NewProvider("123:abc", "42")
	p.SetBaseURL(srv.URL + "/bot123:abc")

	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "<T>", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if path != "/bot123:abc/sendMessage" || !strings.Contains(body, `"chat_id":"42"`) ||
		!strings.Contains(body, `\u003cb\u003e\u0026lt;T\u0026gt;\u003c/b\u003e\nM`) {
		t.Errorf("got %s %s", path, body)
	}

	_, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Message: "fail"})
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("ok=false not reported: %v", err)
	}
}
