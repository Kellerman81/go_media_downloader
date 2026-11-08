package utorrent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestAddTorrent(t *testing.T) {
	var actions []url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, _ := r.BasicAuth(); u != "admin" || p != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/gui/token.html" {
			http.SetCookie(w, &http.Cookie{Name: "GUID", Value: "g", Path: "/"})
			io.WriteString(w, "<html><div id='token' style='display:none;'>TOK</div></html>")

			return
		}

		if c, err := r.Cookie("GUID"); err != nil || c.Value != "g" || r.URL.Query().Get("token") != "TOK" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if r.URL.Query().Get("list") == "1" {
			// The torrent URL shows up only after its add.
			if len(actions) > 0 && actions[len(actions)-1].Get("s") == "http://t/x.torrent" {
				io.WriteString(w, `{"torrents":[["AAAA"],["BBBB"]]}`)
				return
			}

			io.WriteString(w, `{"torrents":[["AAAA"]]}`)

			return
		}

		actions = append(actions, r.URL.Query())
		io.WriteString(w, `{"build":44632}`)
	}))
	defer srv.Close()

	p, err := NewProvider(srv.URL, "admin", "pw")
	if err != nil {
		t.Fatal(err)
	}

	magnet := "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"
	if _, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{URL: magnet, Category: "tv", Paused: true}); err != nil {
		t.Fatal(err)
	}

	if len(actions) != 3 || actions[0].Get("action") != "add-url" || actions[0].Get("s") != magnet ||
		actions[1].Get("v") != "tv" || actions[1].Get("hash") != "0123456789ABCDEF0123456789ABCDEF01234567" ||
		actions[2].Get("action") != "stop" {
		t.Errorf("actions %v", actions)
	}

	actions = nil

	resp, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{URL: "http://t/x.torrent", Category: "movies"})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Hash != "BBBB" || len(actions) != 2 || actions[1].Get("hash") != "BBBB" || actions[1].Get("v") != "movies" {
		t.Errorf("torrent URL: hash %q, actions %v", resp.Hash, actions)
	}

	bad, _ := NewProvider(srv.URL, "admin", "x")
	if err := bad.TestConnection(context.Background()); err == nil {
		t.Error("wrong password accepted")
	}
}
