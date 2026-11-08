package tribler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestAddTorrent(t *testing.T) {
	var method, path, key, body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, path, key, body = r.Method, r.URL.Path, r.Header.Get("X-Api-Key"), string(b)

		io.WriteString(w, `{"started":true,"infohash":"abcd"}`)
	}))
	defer srv.Close()

	resp, err := NewProvider(srv.URL, "k").AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{URL: "magnet:?x"})
	if err != nil {
		t.Fatal(err)
	}

	if method != http.MethodPut || path != "/api/downloads" || key != "k" || resp.Hash != "ABCD" ||
		body != `{"uri":"magnet:?x","anon_hops":1,"safe_seeding":true}` {
		t.Errorf("got %s %s %q %s", method, path, key, body)
	}
}
