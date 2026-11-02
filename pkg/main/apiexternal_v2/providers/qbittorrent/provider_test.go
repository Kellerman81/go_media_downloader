package qbittorrent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func newTestProvider(t *testing.T, h http.Handler) *Provider {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	p, err := NewProvider(u.Hostname(), port, "u", "p", false, "")
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func TestAddTorrentFailsBody(t *testing.T) {
	var form url.Values

	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/add":
			form = r.PostForm
			_, _ = w.Write([]byte("Fails."))
		}
	}))

	_, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{URL: "magnet:?xt=urn:btih:x"})
	if err == nil {
		t.Fatal("expected error for \"Fails.\" body")
	}

	if _, ok := form["savepath"]; ok {
		t.Errorf("savepath sent for empty SavePath")
	}
}

func TestExpiredSessionIsRetriedAfterLogin(t *testing.T) {
	var (
		mu     sync.Mutex
		logins int
		adds   int
	)

	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch r.URL.Path {
		case "/api/v2/auth/login":
			logins++
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/add":
			adds++
			if adds == 1 {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("Forbidden"))

				return
			}

			_, _ = w.Write([]byte("Ok."))
		}
	}))

	resp, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{URL: "http://x/t.torrent", SavePath: "/dl"})
	if err != nil || !resp.Success {
		t.Fatalf("AddTorrent = %+v, %v", resp, err)
	}

	if logins != 2 || adds != 2 {
		t.Fatalf("logins=%d adds=%d, want 2/2", logins, adds)
	}
}

func TestPersistent403IsError(t *testing.T) {
	p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/login") {
			_, _ = w.Write([]byte("Ok."))
			return
		}

		w.WriteHeader(http.StatusForbidden)
	}))

	if err := p.PauseTorrent(context.Background(), "h"); err == nil {
		t.Fatal("expected error for persistent 403")
	}
}

func TestAddTorrentDataIsMultipartUpload(t *testing.T) {
	var (
		fields  url.Values
		file    []byte
		ctype   string
		subPath bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subPath = strings.HasPrefix(r.URL.Path, "/qbit/api/v2/")

		switch strings.TrimPrefix(r.URL.Path, "/qbit") {
		case "/api/v2/auth/login":
			w.WriteHeader(http.StatusNoContent)
		case "/api/v2/torrents/add":
			ctype = r.Header.Get("Content-Type")

			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
				return
			}

			fields = url.Values(r.MultipartForm.Value)

			f, _, err := r.FormFile("torrents")
			if err != nil {
				t.Errorf("torrents part: %v", err)
				return
			}

			file, _ = io.ReadAll(f)
			_, _ = w.Write([]byte("Ok."))
		}
	}))
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	p, err := NewProvider(u.Hostname(), port, "u", "p", false, "/qbit/")
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("d8:announce3:x:e")

	resp, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		TorrentData: data,
		SavePath:    "/dl",
		Category:    "tv",
		Tags:        []string{"a", "b"},
		Paused:      true,
	})
	if err != nil || !resp.Success {
		t.Fatalf("AddTorrent = %+v, %v", resp, err)
	}

	if !subPath {
		t.Error("sub-path not used")
	}

	if !strings.HasPrefix(ctype, "multipart/form-data") {
		t.Errorf("Content-Type %q, want multipart/form-data", ctype)
	}

	if string(file) != string(data) {
		t.Errorf("torrents part = %q, want %q", file, data)
	}

	for k, want := range map[string]string{
		"savepath": "/dl", "category": "tv", "tags": "a,b", "paused": "true", "stopped": "true",
	} {
		if got := fields.Get(k); got != want {
			t.Errorf("field %s = %q, want %q", k, got, want)
		}
	}

	if _, ok := fields["urls"]; ok {
		t.Error("urls sent for a raw torrent add")
	}
}
