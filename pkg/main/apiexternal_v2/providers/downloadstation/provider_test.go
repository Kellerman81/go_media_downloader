package downloadstation

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
	var (
		logins  int
		created url.Values
		dsm6    bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		switch {
		case r.URL.Path == "/webapi/query.cgi":
			if dsm6 {
				io.WriteString(w, `{"success":true,"data":{"SYNO.API.Auth":{"path":"auth.cgi","maxVersion":3},`+
					`"SYNO.DownloadStation.Task":{"path":"DownloadStation/task.cgi","maxVersion":3}}}`)

				return
			}

			io.WriteString(w, `{"success":true,"data":{"SYNO.API.Auth":{"path":"entry.cgi","maxVersion":7},`+
				`"SYNO.DownloadStation2.Task":{"path":"entry.cgi","maxVersion":2}}}`)
		case q.Get("api") == apiAuth:
			logins++

			if q.Get("passwd") != "pw" {
				io.WriteString(w, `{"success":false,"error":{"code":400}}`)
				return
			}

			io.WriteString(w, `{"success":true,"data":{"sid":"S`+string(rune('0'+logins))+`"}}`)
		default:
			// The first session is reported expired once.
			if q.Get("_sid") == "S1" {
				io.WriteString(w, `{"success":false,"error":{"code":119}}`)
				return
			}

			created = q
			io.WriteString(w, `{"success":true,"data":{}}`)
		}
	}))
	defer srv.Close()

	p := NewProvider(srv.URL, "u", "pw")
	if _, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{URL: "magnet:?x", SavePath: "dl/movies"}); err != nil {
		t.Fatal(err)
	}

	if logins != 2 || created.Get("api") != apiTask2 || created.Get("url") != "magnet:?x" ||
		created.Get("destination") != "dl/movies" || created.Get("_sid") != "S2" {
		t.Errorf("logins %d, created %v", logins, created)
	}

	dsm6 = true
	if _, err := NewProvider(srv.URL, "u", "pw").AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{URL: "magnet:?y"}); err != nil {
		t.Fatal(err)
	}

	if created.Get("api") != apiTask1 || created.Get("uri") != "magnet:?y" {
		t.Errorf("DSM 6 create %v", created)
	}

	if err := NewProvider(srv.URL, "u", "wrong").TestConnection(context.Background()); err == nil {
		t.Error("wrong password accepted")
	}
}
