package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
)

// useMediaConfig loads a config with one media group per given media type.
func useMediaConfig(t *testing.T, media config.MediaConfig) {
	t.Helper()
	useTestConfig(t, config.MainConfig{Media: media})
}

// testSchemaDir is the migrations folder, resolved at package start so a
// test may change its working directory first (t.Chdir).
var testSchemaDir, _ = filepath.Abs("../schema/db")

// useTestConfig loads cfg, with the general, list, path, quality and
// scheduler entries the media groups of mediaGroup refer to, and a migrated
// test DB.
func useTestConfig(t *testing.T, cfg config.MainConfig) {
	t.Helper()

	restore, err := database.UseMigratedTestDB(testSchemaDir)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() { config.Configfile = oldFile })

	cfg.General.WorkerFiles, cfg.General.WorkerParse, cfg.General.WorkerSearch = 1, 1, 1
	cfg.Lists = append(cfg.Lists, config.ListsConfig{Name: "listA"})
	cfg.Paths = append(cfg.Paths, config.PathsConfig{Name: "alpha"})
	cfg.Quality = append(cfg.Quality, config.QualityConfig{Name: "HD"})
	cfg.Scheduler = append(cfg.Scheduler, config.SchedulerConfig{Name: "Default", IntervalImdb: "3d"})

	cnt, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(config.Configfile, cnt, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := config.Loadallsettings(true); err != nil {
		t.Fatal(err)
	}
}

func mediaGroup(name string) []config.MediaTypeConfig {
	return []config.MediaTypeConfig{{
		Name: name, TemplateQuality: "HD", TemplateScheduler: "Default",
		Data:  []config.MediaDataConfig{{TemplatePath: "alpha"}},
		Lists: []config.MediaListsConfig{{Name: name + " List", TemplateList: "listA", TemplateQuality: "HD", Enabled: true}},
	}}
}

var metadataSearchTabRe = regexp.MustCompile(`<a class="(nav-link[^"]*)" href="/api/admin/metadata-search\?type=(\w+)"`)

func metadataSearchBody(t *testing.T, query string) (tabs []string, active, body string) {
	t.Helper()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	asAdmin(ctx)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/metadata-search"+query, nil)

	MetadataSearchPage(ctx)

	body = w.Body.String()
	for _, m := range metadataSearchTabRe.FindAllStringSubmatch(body, -1) {
		tabs = append(tabs, m[2])
		if strings.Contains(m[1], "active") {
			active = m[2]
		}
	}

	return tabs, active, body
}

func TestMetadataSearchPage(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{
		Movies: mediaGroup("EN"), Series: mediaGroup("TV"), Books: mediaGroup("Books"),
		AudioBooks: mediaGroup("Audio"), Music: mediaGroup("Music"),
	})

	all := []string{"movies", "series", "books", "audiobooks", "music"}
	for _, tc := range []struct{ query, active, header string }{
		{"", "movies", "Movie"},
		{"?type=series", "series", "Series"},
		{"?type=books", "books", "Add Books"},
		{"?type=audiobooks", "audiobooks", "Add Audiobooks"},
		{"?type=music", "music", "Add Music"},
		{"?type=nope", "movies", "Movie"},
	} {
		tabs, active, body := metadataSearchBody(t, tc.query)
		if !slices.Equal(tabs, all) || active != tc.active || !strings.Contains(body, tc.header) {
			t.Errorf("%q: tabs %v, active %q, header %q found %v", tc.query, tabs, active, tc.header, strings.Contains(body, tc.header))
		}

		// One form at a time: the other types' content isn't on the page.
		if tc.active != "music" && strings.Contains(body, "Add Music") {
			t.Errorf("%q: music form rendered too", tc.query)
		}
	}
}

func TestMetadataSearchPageOnlyConfiguredTypes(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Music: mediaGroup("Music"), AudioBooks: mediaGroup("Audio")})

	tabs, active, _ := metadataSearchBody(t, "?type=movies")
	if !slices.Equal(tabs, []string{"audiobooks", "music"}) || active != "audiobooks" {
		t.Errorf("tabs %v, active %q; want audiobooks and music, audiobooks active", tabs, active)
	}
}

// TestAddMovieExistingByImdb: Add Media reuses a movie imported from a list
// before TMDB linked its IMDb id (no TMDB id yet) instead of adding it a
// second time.
func TestAddMovieExistingByImdb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: append(mediaGroup("EN"), mediaGroup("DE")...)})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":1592019,"imdb_id":"tt39096136","title":"Missing the Boat","release_date":"2026-02-14"}`))
	}))
	t.Cleanup(srv.Close)

	providers.SetTMDB(tmdb.NewProviderWithConfig(base.ClientConfig{Name: "tmdb", BaseURL: srv.URL + "/3"}, "key"))
	t.Cleanup(func() { providers.SetTMDB(nil) })

	for _, q := range []string{
		"insert into dbmovies (id, title, imdb_id, moviedb_id) values (9719, 'Missing the Boat', 'tt39096136', 0)",
		"insert into movies (dbmovie_id, listname) values (9719, 'DE List')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("tmdb_id=1592019&movie_list=EN+List&movie_quality_profile=HD"))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	AddMovieToDatabase(c)

	if w.Code != http.StatusOK {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}

	if n := database.Getdatarow[int](false, "select count() from dbmovies"); n != 1 {
		t.Errorf("%d dbmovies rows, want the existing one only", n)
	}

	if lists := database.GetrowsN[string](false, 5, "select listname from movies where dbmovie_id = 9719 order by listname"); !slices.Equal(lists, []string{"DE List", "EN List"}) {
		t.Errorf("lists of the existing movie: %v", lists)
	}
}
