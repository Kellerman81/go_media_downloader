package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// clientFixture is a library for the apps' protocols: a movie, a series
// with two episodes, an album, an audiobook and a book, each with a file
// in dir; every protocol on; the account "anna" (password "secret12")
// sees only the movie group, "noapps" too, but without the apps feature.
type clientFixture struct {
	dir    string
	router *gin.Engine
}

func clientTestData(t *testing.T) *clientFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	group := func(name string) []config.MediaTypeConfig {
		g := mediaGroup(name)
		g[0].Data = []config.MediaDataConfig{{TemplatePath: "media"}}

		return g
	}

	cfg := config.MainConfig{Media: config.MediaConfig{
		Movies: group("EN"), Series: group("X"), Music: group("M"), AudioBooks: group("A"), Books: group("B"),
	}}
	cfg.General = config.GeneralConfig{
		ServeWebDAV: true, ServeJellyfin: true, ServeSubsonic: true, ServeOPDS: true, ServeDLNA: true, WebAPIKey: "adminpass",
	}
	cfg.Paths = []config.PathsConfig{{Name: "media", Path: dir}}

	useTestConfig(t, cfg)
	useRememberSecret(t)

	for _, f := range []string{"movie.mkv", "e1.mkv", "e2.mkv", "01 - Song.mp3", "part1.mp3", "novel.epub"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("data of "+f), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	loc := func(f string) string { return strings.ReplaceAll(filepath.Join(dir, f), "'", "''") }

	for _, s := range []string{
		"insert into dbmovies (id, title, year, overview, imdb_id, moviedb_id, genres) values (1, 'Alien', 1979, 'In space', 'tt0078748', 348, 'Horror,Sci-Fi'), (2, 'Missing', 2000, '', '', 0, '')",
		"insert into movies (id, dbmovie_id, listname) values (1, 1, 'EN List'), (2, 2, 'EN List')",
		"insert into movie_files (id, movie_id, dbmovie_id, location, file_size) values (11, 1, 1, '" + loc("movie.mkv") + "', 18)",
		"insert into dbseries (id, seriename, firstaired, overview) values (1, 'Show', '2020-01-01', 'A show')",
		"insert into series (id, dbserie_id, listname) values (1, 1, 'X List')",
		"insert into dbserie_episodes (id, dbserie_id, identifier, title, season, episode) values (1, 1, 'S01E01', 'Pilot', '1', '1'), (2, 1, 'S01E02', 'Second', '1', '2')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id) values (5, 1, 1, 1), (6, 1, 1, 2)",
		"insert into serie_episode_files (id, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, location, runtime_seconds) values " +
			"(21, 1, 5, 1, 1, '" + loc("e1.mkv") + "', 1200), (22, 1, 6, 2, 1, '" + loc("e2.mkv") + "', 1200)",
		"insert into dbartists (id, name) values (1, 'Band')",
		"insert into dbalbums (id, title, year, genres) values (1, 'Record', 2001, 'rock,pop')",
		"insert into dbalbum_artists (dbalbum_id, dbartist_id, position) values (1, 1, 0)",
		"insert into albums (id, dbalbum_id, listname) values (1, 1, 'M List')",
		"insert into album_files (id, album_id, dbalbum_id, location, track_number, runtime_ms, file_size) values (31, 1, 1, '" +
			loc("01 - Song.mp3") + "', 1, 180000, 22)",
		"insert into dbauthors (id, name) values (1, 'Writer')",
		"insert into dbaudiobooks (id, title) values (1, 'Tale')",
		"insert into dbaudiobook_authors (dbaudiobook_id, dbauthor_id, position) values (1, 1, 0)",
		"insert into audiobooks (id, dbaudiobook_id, listname) values (1, 1, 'A List')",
		"insert into audiobook_files (id, audiobook_id, dbaudiobook_id, location, runtime_ms) values (41, 1, 1, '" + loc("part1.mp3") + "', 600000)",
		"insert into dbbooks (id, title, dbauthor_id, year, description) values (1, 'Novel', 1, 2010, 'A story')",
		"insert into books (id, dbbook_id, listname) values (1, 1, 'B List')",
		"insert into book_files (id, book_id, dbbook_id, location, file_size) values (51, 1, 1, '" + loc("novel.epub") + "', 19)",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}

	if err := database.InsertUser(&database.User{Username: "anna", Role: database.RoleUser, MediaGroups: "movie_EN", Features: database.FeatureApps}, "secret12"); err != nil {
		t.Fatal(err)
	}

	if err := database.InsertUser(&database.User{Username: "noapps", Role: database.RoleUser, MediaGroups: "movie_EN"}, "secret12"); err != nil {
		t.Fatal(err)
	}

	clientLogins.Clear()
	jfTokens.Clear()
	loginLimiter.reset("192.0.2.1")
	loginLimiter.reset("192.168.1.20")

	r := gin.New()
	AddClientRoutes(r)

	return &clientFixture{dir: dir, router: r}
}

// do sends a request to the fixture's router from a local address.
func (f *clientFixture) do(t *testing.T, method, target string, body io.Reader, header http.Header) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, body)
	req.RemoteAddr = "192.168.1.20:5000"

	for k, v := range header {
		req.Header[k] = v
	}

	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)

	return w
}

func basicHeader(user, pass string) http.Header {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetBasicAuth(user, pass)

	return r.Header
}

func TestClientAuthenticate(t *testing.T) {
	clientTestData(t)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	if acc, err := clientAuthenticate(c, "anna", "secret12"); err != nil || acc.Name != "anna" || !acc.AllowsGroup("movie_EN") || acc.AllowsGroup("serie_X") {
		t.Fatalf("anna: %+v %v", acc, err)
	}

	if acc, err := clientAuthenticate(c, DefaultUsername, "adminpass"); err != nil || !acc.Admin() {
		t.Fatalf("admin: %+v %v", acc, err)
	}

	if _, err := clientAuthenticate(c, "anna", "wrong"); !errors.Is(err, errClientLogin) {
		t.Fatalf("wrong password: %v", err)
	}

	// The right password of an account without the apps feature: told apart,
	// a wrong one isn't.
	if _, err := clientAuthenticate(c, "noapps", "secret12"); !errors.Is(err, errClientNoApps) {
		t.Fatalf("account without the apps feature: %v", err)
	}

	if _, err := clientAuthenticate(c, "noapps", "wrong"); !errors.Is(err, errClientLogin) {
		t.Fatalf("account without the apps feature, wrong password: %v", err)
	}

	// Repeated failures lock the address out, even for the right password.
	for range 10 {
		clientAuthenticate(c, "nobody", "x")
	}

	clientLogins.Clear()

	if _, err := clientAuthenticate(c, "anna", "secret12"); err == nil {
		t.Fatal("login accepted during the lockout")
	}

	loginLimiter.reset(c.ClientIP())
}

// TestClientNoApps: an account without the apps feature is told so by every
// protocol, not that its password is wrong.
func TestClientNoApps(t *testing.T) {
	f := clientTestData(t)

	if w := f.do(t, "PROPFIND", "/dav/", nil, basicHeader("noapps", "secret12")); w.Code != http.StatusForbidden ||
		w.Header().Get("WWW-Authenticate") != "" || !strings.Contains(w.Body.String(), "may not use apps") {
		t.Errorf("WebDAV: %d %q %q", w.Code, w.Header().Get("WWW-Authenticate"), w.Body)
	}

	if w := f.do(t, "PROPFIND", "/dav/", nil, basicHeader("noapps", "wrong")); w.Code != http.StatusUnauthorized {
		t.Errorf("WebDAV, wrong password: %d", w.Code)
	}

	if w := f.do(t, http.MethodGet, "/opds", nil, basicHeader("noapps", "secret12")); w.Code != http.StatusForbidden {
		t.Errorf("OPDS: %d", w.Code)
	}

	if w := f.do(t, http.MethodGet, "/subsonic/rest/ping.view?u=noapps&p=secret12", nil, nil); !strings.Contains(w.Body.String(), `code="50"`) {
		t.Errorf("Subsonic: %s", w.Body)
	}

	h := http.Header{"Content-Type": {"application/json"}, "X-Emby-Authorization": {`MediaBrowser Client="t", Device="d", DeviceId="1", Version="1"`}}
	if w := f.do(t, http.MethodPost, "/jellyfin/Users/AuthenticateByName", strings.NewReader(`{"Username":"noapps","Pw":"secret12"}`), h); w.Code != http.StatusForbidden {
		t.Errorf("Jellyfin: %d %s", w.Code, w.Body)
	}

	loginLimiter.reset("192.0.2.1")
}

func TestLocalRequest(t *testing.T) {
	for addr, want := range map[string]bool{
		"192.168.1.5:1000": true, "10.0.0.1:1": true, "127.0.0.1:1": true, "[fe80::1]:1": true, "8.8.8.8:53": false, "bad": false,
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = addr

		if got := localRequest(r); got != want {
			t.Errorf("%s: %v, want %v", addr, got, want)
		}
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.5:1000"
	r.Header.Set("X-Forwarded-For", "192.168.1.6")

	if localRequest(r) {
		t.Error("a request through a proxy counted as local")
	}
}

func TestServeEnabled(t *testing.T) {
	f := clientTestData(t)

	config.GetSettingsGeneral().ServeOPDS = false
	t.Cleanup(func() { config.GetSettingsGeneral().ServeOPDS = true })

	if w := f.do(t, http.MethodGet, "/opds", nil, basicHeader("anna", "secret12")); w.Code != http.StatusNotFound {
		t.Fatalf("OPDS switched off: %d", w.Code)
	}
}

func TestMediaContentType(t *testing.T) {
	for name, want := range map[string]string{"a/b.MKV": "video/x-matroska", `c:\x\y.flac`: "audio/flac", "z.epub": "application/epub+zip", "q.xyz": ""} {
		if got := mediaContentType(name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}

}

// TestRenderAccountApps: My Account names the addresses of the protocols
// that are on, for accounts with the apps feature.
func TestRenderAccountApps(t *testing.T) {
	clientTestData(t)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "http://media.example/api/admin/account", nil)

	out := renderComponentToString(renderAccountApps(c, adminAccess))
	for _, want := range []string{"http://media.example/jellyfin", "http://media.example/dav", "http://media.example/subsonic", "http://media.example/opds", "DLNA"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %s: %s", want, out)
		}
	}

	if n := renderAccountApps(c, &userAccess{Name: "noapps", Role: database.RoleUser}); n != nil {
		t.Errorf("without the feature: %s", renderComponentToString(n))
	}
}
