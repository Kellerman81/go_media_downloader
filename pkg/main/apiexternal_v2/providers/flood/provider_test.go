package flood

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestAddTorrent(t *testing.T) {
	var added string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)

		switch r.URL.Path {
		case "/sub/api/auth/authenticate":
			if string(b) != `{"password":"pw","username":"u"}` {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			http.SetCookie(w, &http.Cookie{Name: "jwt", Value: "tok", Path: "/"})
			io.WriteString(w, `{"success":true}`)
		case "/sub/api/torrents/add-urls":
			if c, err := r.Cookie("jwt"); err != nil || c.Value != "tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			added = string(b)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p, err := NewProvider(srv.URL+"/sub/", "u", "pw")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		URL: "http://t/x.torrent", Category: "movies", SavePath: "/dl",
	}); err != nil {
		t.Fatal(err)
	}

	if added != `{"destination":"/dl","start":true,"tags":["movies"],"urls":["http://t/x.torrent"]}` {
		t.Errorf("added %s", added)
	}

	bad, _ := NewProvider(srv.URL+"/sub", "u", "wrong")
	if err := bad.TestConnection(context.Background()); err == nil {
		t.Error("wrong password accepted")
	}
}
