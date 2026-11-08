package rqbit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestAddTorrent(t *testing.T) {
	var query, body, paused string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)

		if r.URL.Path == "/torrents/abcd/pause" {
			paused = r.Method
			return
		}

		query, body = r.URL.RawQuery, string(b)
		io.WriteString(w, `{"id":3,"details":{"info_hash":"abcd","name":"Movie"}}`)
	}))
	defer srv.Close()

	resp, err := NewProvider(srv.URL, "", "").AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		URL: "magnet:?xt=urn:btih:abcd", SavePath: "/dl", Paused: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if query != "is_url=true&output_folder=%2Fdl&overwrite=true" || body != "magnet:?xt=urn:btih:abcd" ||
		resp.Hash != "ABCD" || resp.Name != "Movie" || paused != http.MethodPost {
		t.Errorf("got %s %s, pause %q -> %+v", query, body, paused, resp)
	}
}
