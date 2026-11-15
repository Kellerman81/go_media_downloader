package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
	"github.com/gin-gonic/gin"
)

func coverRequest(t *testing.T, kind, id string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "type", Value: kind}, {Key: "id", Value: id}}
	HandleLibraryCover(c)

	return w
}

func TestLibraryCoverCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)
	t.Chdir(t.TempDir())

	loads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		loads++
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("\x89PNG fake"))
	}))
	defer srv.Close()

	if err := database.ExecNErr("update dbmovies set poster = ? where id = 1", srv.URL+"/p.png"); err != nil {
		t.Fatal(err)
	}

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		parts := strings.Split(path, "/")
		c.Params = gin.Params{{Key: "type", Value: parts[1]}, {Key: "id", Value: parts[2]}}
		HandleLibraryCover(c)

		return w
	}

	for range 2 {
		if w := get("/movie/1"); w.Body.String() != "\x89PNG fake" {
			t.Fatalf("cover = %q", w.Body.String())
		}
	}

	if loads != 1 {
		t.Errorf("cover loaded %d times", loads)
	}

	if files, _ := filepath.Glob(filepath.Join(defaultCoverCacheDir, "movie-1-*.png")); len(files) != 1 {
		t.Errorf("cache files = %v", files)
	}

	// No poster, unknown type: placeholder / 404.
	if w := get("/movie/2"); !strings.Contains(w.Body.String(), "<svg") {
		t.Error("no placeholder")
	}

	if w := get("/nope/1"); w.Code != http.StatusNotFound {
		t.Errorf("unknown type: %d", w.Code)
	}
}

func TestLibraryCoverFromPlex(t *testing.T) {
	gin.SetMode(gin.TestMode)

	posters := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("X-Plex-Token") != "secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/library/sections":
			w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie"}]}}`))
		case "/library/sections/1/all":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"10","title":"Inception","thumb":"/library/metadata/10/thumb/1",
				"Guid":[{"id":"imdb://tt1375666"}]}]}}`))
		case "/photo/:/transcode":
			if r.URL.Query().Get("url") != "/library/metadata/10/thumb/1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			posters++
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte("plex poster"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	useTestConfig(t, config.MainConfig{
		General: config.GeneralConfig{PlexServerURL: srv.URL, PlexToken: "secret-token", MovieMetaSourcePlex: true,
			MovieMetaSourcePriority: []string{"plex"}},
		Media: config.MediaConfig{Movies: mediaGroup("EN")},
	})
	providers.RegisterMediaServerProviders()
	t.Chdir(t.TempDir())

	if err := database.ExecNErr("insert into dbmovies (id, title, imdb_id) values (1, 'Inception', 'tt1375666'), (2, 'Other', 'tt0')"); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if w := coverRequest(t, "movie", "1"); w.Body.String() != "plex poster" {
			t.Fatalf("cover = %q", w.Body.String())
		}
	}

	if posters != 1 {
		t.Errorf("poster loaded %d times", posters)
	}

	files, _ := filepath.Glob(filepath.Join(defaultCoverCacheDir, "*"))
	for _, f := range files {
		if strings.Contains(f, "secret") {
			t.Errorf("token in cache file name: %s", f)
		}
	}

	// Not on the server: placeholder.
	if w := coverRequest(t, "movie", "2"); !strings.Contains(w.Body.String(), "<svg") {
		t.Error("no placeholder")
	}

}

func TestLibraryCoverFromFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)
	t.Chdir(t.TempDir())

	// Album 1: cover.jpg next to the tracks; album 2: only an embedded picture.
	withFolder, withTag := filepath.Join(t.TempDir(), "a1"), filepath.Join(t.TempDir(), "a2")
	for _, d := range []string{withFolder, withTag} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	os.WriteFile(filepath.Join(withFolder, "01.mp3"), []byte("not really audio"), 0o644)
	os.WriteFile(filepath.Join(withFolder, "Folder.JPG"), []byte("folder image"), 0o644)
	os.WriteFile(filepath.Join(withFolder, "cover.png"), []byte("cover image"), 0o644)

	tagged := filepath.Join(withTag, "01.mp3")
	os.WriteFile(tagged, []byte("not really audio"), 0o644)

	if err := tags.WriteTags(context.Background(), tagged, &tags.AudioTags{Title: "T", CoverData: []byte("\xff\xd8embedded"), CoverMIME: "image/jpeg"}); err != nil {
		t.Fatal(err)
	}

	for _, q := range []string{
		"insert into dbalbums (id, title, cover_url) values (1, 'One', ''), (2, 'Two', 'https://127.0.0.1:1/dead.jpg'), (3, 'Three', '')",
		"insert into album_files (id, location, dbalbum_id, track_number) values (1, '" + filepath.Join(withFolder, "01.mp3") + "', 1, 1), (2, '" + tagged + "', 2, 1)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	if w := coverRequest(t, "album", "1"); w.Body.String() != "cover image" {
		t.Errorf("folder cover = %q", w.Body.String())
	}

	if w := coverRequest(t, "album", "2"); w.Body.String() != "\xff\xd8embedded" {
		t.Errorf("embedded cover = %q", w.Body.String())
	}

	if w := coverRequest(t, "album", "3"); !strings.Contains(w.Body.String(), "<svg") {
		t.Error("album without files: no placeholder")
	}
}

func TestLibraryCoverFromFrame(t *testing.T) {
	gin.SetMode(gin.TestMode)

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	libraryTestData(t)
	t.Chdir(t.TempDir())

	video := filepath.Join(t.TempDir(), "ep.mkv")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=640x360:rate=25:duration=4",
		"-c:v", "libx264", video).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}

	if err := database.ExecNErr("insert into serie_episode_files (id, serie_id, serie_episode_id, dbserie_id, location) values (5, 1, 1, 1, ?)", video); err != nil {
		t.Fatal(err)
	}

	// Series 1 has no poster: its cover is a frame of the episode file.
	if w := coverRequest(t, "series", "1"); !strings.HasPrefix(w.Body.String(), "\xff\xd8") {
		t.Errorf("series cover is no JPEG frame: %.20q", w.Body.String())
	}

	thumb := func(id string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Params = gin.Params{{Key: "kind", Value: "episode"}, {Key: "id", Value: id}}
		HandleLibraryThumb(c)

		return w
	}

	if w := thumb("5"); !strings.HasPrefix(w.Body.String(), "\xff\xd8") {
		t.Errorf("episode thumb is no JPEG: %.20q", w.Body.String())
	}

	if w := thumb("99"); !strings.Contains(w.Body.String(), "<svg") {
		t.Error("unknown episode file: no placeholder")
	}
}
