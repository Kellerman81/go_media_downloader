package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestWebDAV(t *testing.T) {
	f := clientTestData(t)
	admin := basicHeader(DefaultUsername, "adminpass")
	depth1 := func(h http.Header) http.Header {
		out := h.Clone()
		out.Set("Depth", "1")

		return out
	}

	// Windows asks the root and the share without a login first.
	for _, p := range []string{"/", "/dav", "/dav/movie_EN/"} {
		w := f.do(t, http.MethodOptions, p, nil, nil)
		if w.Code != http.StatusOK || w.Header().Get("DAV") != "1, 2" || !strings.Contains(w.Header().Get("Allow"), "PROPFIND") ||
			w.Header().Get("MS-Author-Via") != "DAV" {
			t.Errorf("OPTIONS %s: %d %v", p, w.Code, w.Header())
		}
	}

	if w := f.do(t, "PROPFIND", "/dav/", nil, nil); w.Code != http.StatusUnauthorized || w.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("without login: %d", w.Code)
	}

	w := f.do(t, "PROPFIND", "/dav/", nil, depth1(admin))
	if w.Code != http.StatusMultiStatus {
		t.Fatalf("root: %d %s", w.Code, w.Body)
	}

	// A group's folder is named after the group, not its folder on disk.
	if !strings.Contains(w.Body.String(), "<D:displayname>movie_EN</D:displayname>") {
		t.Errorf("group folder's name: %s", w.Body)
	}

	for _, g := range []string{"/dav/movie_EN/", "/dav/serie_X/", "/dav/music_M/", "/dav/audiobook_A/", "/dav/book_B/"} {
		if !strings.Contains(w.Body.String(), g) {
			t.Errorf("root lacks %s: %s", g, w.Body)
		}
	}

	if w := f.do(t, "PROPFIND", "/dav/movie_EN/", nil, depth1(admin)); w.Code != http.StatusMultiStatus || !strings.Contains(w.Body.String(), "movie.mkv") {
		t.Fatalf("group folder: %d %s", w.Code, w.Body)
	}

	if w := f.do(t, http.MethodGet, "/dav/movie_EN/movie.mkv", nil, admin); w.Code != http.StatusOK || w.Body.String() != "data of movie.mkv" {
		t.Fatalf("file: %d %q", w.Code, w.Body)
	}

	// A folder answers GET with a playlist for players that only GET (VLC).
	w = f.do(t, http.MethodGet, "/dav", nil, admin)
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "audio/x-mpegurl") ||
		!strings.Contains(w.Body.String(), "#EXTINF:-1,movie_EN\n/dav/movie_EN/\n") {
		t.Fatalf("root playlist: %d %q", w.Code, w.Body)
	}

	w = f.do(t, http.MethodGet, "/dav/movie_EN/", nil, admin)
	if body := w.Body.String(); w.Code != http.StatusOK || !strings.HasPrefix(body, "#EXTM3U\n") ||
		!strings.Contains(body, "\n/dav/movie_EN/movie.mkv\n") || !strings.Contains(body, "\n/dav/movie_EN/01%20-%20Song.mp3\n") ||
		strings.Contains(body, "novel.epub") {
		t.Fatalf("group playlist: %d %q", w.Code, body)
	}

	if w := f.do(t, http.MethodGet, "/dav/movie_EN/", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("playlist without login: %d", w.Code)
	}

	for _, m := range []string{http.MethodPut, http.MethodDelete, "MKCOL", "MOVE"} {
		if w := f.do(t, m, "/dav/movie_EN/movie.mkv", strings.NewReader("x"), admin); w.Code < 400 {
			t.Errorf("%s allowed: %d", m, w.Code)
		}
	}

	if w := f.do(t, "PROPFIND", "/dav/", nil, func() http.Header { h := admin.Clone(); h.Set("Depth", "infinity"); return h }()); w.Code != http.StatusForbidden {
		t.Errorf("Depth infinity: %d", w.Code)
	}

	// anna sees only her group.
	anna := basicHeader("anna", "secret12")
	if w := f.do(t, "PROPFIND", "/dav/", nil, depth1(anna)); !strings.Contains(w.Body.String(), "movie_EN") || strings.Contains(w.Body.String(), "serie_X") {
		t.Fatalf("anna's root: %s", w.Body)
	}

	if w := f.do(t, http.MethodGet, "/dav/serie_X/e1.mkv", nil, anna); w.Code != http.StatusNotFound {
		t.Fatalf("anna reads another group: %d", w.Code)
	}

	// No way out of the group's folder.
	if w := f.do(t, http.MethodGet, "/dav/movie_EN/../../etc/passwd", nil, admin); w.Code == http.StatusOK && strings.Contains(w.Body.String(), "root:") {
		t.Fatal("left the media folder")
	}
}
