package aria2

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestProvider(t *testing.T) {
	var path, body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, body = r.URL.Path, string(b)

		if r.URL.Path != "/jsonrpc" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"id":"gmd","jsonrpc":"2.0","error":{"code":1,"message":"Unauthorized"}}`)

			return
		}

		io.WriteString(w, `{"id":"gmd","jsonrpc":"2.0","result":"2089b05ecca3d829"}`)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	p := NewProvider(u.Hostname(), port, "s3cret", false, "")
	if _, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		URL: "magnet:?xt=urn:btih:x", SavePath: "/dl", Paused: true,
	}); err != nil {
		t.Fatal(err)
	}

	want := `{"id":"gmd","jsonrpc":"2.0","method":"aria2.addUri","params":["token:s3cret",["magnet:?xt=urn:btih:x"],{"dir":"/dl","pause":"true"}]}`
	if path != "/jsonrpc" || body != want {
		t.Errorf("got %s %s", path, body)
	}

	err := NewProvider(u.Hostname(), port, "", false, "/rpc").TestConnection(context.Background())
	if err == nil || err.Error() != "aria2: Unauthorized" {
		t.Errorf("RPC error not reported: %v", err)
	}
}
