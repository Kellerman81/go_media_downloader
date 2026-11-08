package signal

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var path, body, user string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, body = r.URL.Path, string(b)
		user, _, _ = r.BasicAuth()

		io.WriteString(w, `["+491"]`)
	}))
	defer srv.Close()

	p := NewProvider("http://u:p@"+srv.Listener.Addr().String()+"/", "+491", "+492")
	if _, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "T", Message: "M"}); err != nil {
		t.Fatal(err)
	}

	if path != "/v2/send" || user != "u" || body != `{"message":"T\nM","number":"+491","recipients":["+492"]}` {
		t.Errorf("got %s as %q: %s", path, user, body)
	}

	if err := p.TestConnection(context.Background()); err != nil {
		t.Errorf("registered sender: %v", err)
	}

	if err := NewProvider(srv.URL, "+499", "+492").TestConnection(context.Background()); err == nil {
		t.Error("unregistered sender accepted")
	}
}
