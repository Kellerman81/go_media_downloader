package api

import (
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// dlnaBrowse sends a Browse action and returns the DIDL-Lite result.
func dlnaBrowse(t *testing.T, f *clientFixture, id, flag string) (string, string) {
	t.Helper()

	body := `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>` +
		`<u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><ObjectID>` + id + `</ObjectID><BrowseFlag>` + flag +
		`</BrowseFlag><Filter>*</Filter><StartingIndex>0</StartingIndex><RequestedCount>0</RequestedCount><SortCriteria></SortCriteria>` +
		`</u:Browse></s:Body></s:Envelope>`

	h := http.Header{}
	h.Set("SOAPAction", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)

	w := f.do(t, http.MethodPost, "/dlna/cd/control", strings.NewReader(body), h)
	if w.Code != http.StatusOK {
		return "", w.Body.String()
	}

	m := regexp.MustCompile(`<Result>(.*)</Result>.*<TotalMatches>(\d+)</TotalMatches>`).FindStringSubmatch(w.Body.String())
	if m == nil {
		t.Fatalf("browse %s: %s", id, w.Body)
	}

	return html.UnescapeString(m[1]), m[2]
}

func TestDLNA(t *testing.T) {
	f := clientTestData(t)

	w := f.do(t, http.MethodGet, "/dlna/device.xml", nil, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "uuid:"+dlnaUUID()) || !strings.Contains(w.Body.String(), "MediaServer:1") {
		t.Fatalf("device: %d %s", w.Code, w.Body)
	}

	if w := f.do(t, http.MethodGet, "/dlna/cd.xml", nil, nil); !strings.Contains(w.Body.String(), "<name>Browse</name>") {
		t.Fatalf("service description: %s", w.Body)
	}

	root, total := dlnaBrowse(t, f, "0", "BrowseDirectChildren")
	if total != "4" || !strings.Contains(root, ">Movies EN<") || strings.Contains(root, "Books") {
		t.Fatalf("root (%s): %s", total, root)
	}

	movies, _ := dlnaBrowse(t, f, "g/"+dlnaGroupKey("movie_EN"), "BrowseDirectChildren")
	if !strings.Contains(movies, `id="m/1"`) || !strings.Contains(movies, "object.item.videoItem.movie") ||
		!strings.Contains(movies, "/dlna/res/movie/11/movie.mkv") || strings.Contains(movies, "Missing") {
		t.Fatalf("movies: %s", movies)
	}

	seasons, _ := dlnaBrowse(t, f, "s/1", "BrowseDirectChildren")
	if !strings.Contains(seasons, `id="se/1/1"`) {
		t.Fatalf("seasons: %s", seasons)
	}

	eps, total := dlnaBrowse(t, f, "se/1/1", "BrowseDirectChildren")
	if total != "2" || !strings.Contains(eps, "S01E01 Pilot") || !strings.Contains(eps, `duration="0:20:00.000"`) {
		t.Fatalf("episodes: %s", eps)
	}

	artists, _ := dlnaBrowse(t, f, "g/"+dlnaGroupKey("music_M"), "BrowseDirectChildren")
	if !strings.Contains(artists, ">Band<") {
		t.Fatalf("artists: %s", artists)
	}

	if albums, _ := dlnaBrowse(t, f, "ar/"+dlnaGroupKey("music_M")+"/1", "BrowseDirectChildren"); !strings.Contains(albums, `id="al/1"`) {
		t.Fatalf("albums: %s", albums)
	}

	if tracks, _ := dlnaBrowse(t, f, "al/1", "BrowseDirectChildren"); !strings.Contains(tracks, "musicTrack") || !strings.Contains(tracks, ">Song<") {
		t.Fatalf("tracks: %s", tracks)
	}

	if parts, _ := dlnaBrowse(t, f, "b/1", "BrowseDirectChildren"); !strings.Contains(parts, "audioBook") || !strings.Contains(parts, ">part1<") {
		t.Fatalf("audiobook parts: %s", parts)
	}

	if meta, total := dlnaBrowse(t, f, "m/1", "BrowseMetadata"); total != "1" || !strings.Contains(meta, ">Alien<") {
		t.Fatalf("metadata: %s", meta)
	}

	if _, fault := dlnaBrowse(t, f, "m/2", "BrowseMetadata"); !strings.Contains(fault, "<errorCode>701</errorCode>") {
		t.Fatalf("movie without a file: %s", fault)
	}

	// The DLNA account's playlists.
	pl := database.Playlist{Username: "admin", Name: "Evening"}
	if err := database.SavePlaylist(&pl); err != nil {
		t.Fatal(err)
	}

	if _, err := database.AddPlaylistItems("admin", &pl.ID, []database.PlaylistItem{{Kind: "series", EntryID: 1, Title: "Show"}}); err != nil {
		t.Fatal(err)
	}

	if root, total := dlnaBrowse(t, f, "0", "BrowseDirectChildren"); total != "5" || !strings.Contains(root, `id="pl"`) {
		t.Fatalf("root with playlists (%s): %s", total, root)
	}

	if lists, _ := dlnaBrowse(t, f, "pl", "BrowseDirectChildren"); !strings.Contains(lists, ">Evening<") {
		t.Fatalf("playlists: %s", lists)
	}

	if items, total := dlnaBrowse(t, f, "p/"+strconv.FormatUint(uint64(pl.ID), 10), "BrowseDirectChildren"); total != "2" || !strings.Contains(items, "S01E02 Second") {
		t.Fatalf("playlist items (%s): %s", total, items)
	}

	w = f.do(t, http.MethodGet, "/dlna/res/movie/11/movie.mkv", nil, nil)
	if w.Code != http.StatusOK || w.Body.String() != "data of movie.mkv" || w.Header().Get("contentFeatures.dlna.org") == "" {
		t.Fatalf("resource: %d %q", w.Code, w.Body)
	}

	// Not answered from outside the local network or through a proxy.
	req := httptest.NewRequest(http.MethodGet, "/dlna/device.xml", nil)
	req.RemoteAddr = "203.0.113.5:4000"
	out := httptest.NewRecorder()
	f.router.ServeHTTP(out, req)

	if out.Code != http.StatusForbidden {
		t.Fatalf("from outside: %d", out.Code)
	}

	if w := f.do(t, http.MethodGet, "/dlna/res/movie/11/x.mkv", nil, http.Header{"X-Forwarded-For": {"192.168.1.9"}}); w.Code != http.StatusForbidden {
		t.Fatalf("through a proxy: %d", w.Code)
	}

	// dlna_user narrows it to that account's groups.
	config.GetSettingsGeneral().DLNAUser = "anna"
	t.Cleanup(func() { config.GetSettingsGeneral().DLNAUser = "" })

	if root, total := dlnaBrowse(t, f, "0", "BrowseDirectChildren"); total != "1" || !strings.Contains(root, "Movies") {
		t.Fatalf("anna's root (%s): %s", total, root)
	}

	if w := f.do(t, http.MethodGet, "/dlna/res/episode/21/e1.mkv", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("anna's DLNA plays another group: %d", w.Code)
	}
}

func TestDLNADuration(t *testing.T) {
	for s, want := range map[float64]string{0: "0:00:00.000", 61.5: "0:01:01.500", 3725.25: "1:02:05.250"} {
		if got := dlnaDuration(s); got != want {
			t.Errorf("%v: %q, want %q", s, got, want)
		}
	}
}
