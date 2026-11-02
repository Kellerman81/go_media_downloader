package api

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
)

// TestAPIEndpointDocsMatchRoutes checks that every endpoint the API testing
// page offers is actually registered.
func TestAPIEndpointDocsMatchRoutes(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	g := r.Group("/api")
	AddWebRoutes(g)
	AddGeneralRoutes(g)
	AddAllRoutes(g.Group("/all"))
	AddMoviesRoutes(g.Group("/movies"))
	AddSeriesRoutes(g.Group("/series"))
	AddBooksRoutes(g.Group("/books"))
	AddAudiobooksRoutes(g.Group("/audiobooks"))
	AddMusicRoutes(g.Group("/music"))

	param := regexp.MustCompile(`(:|\*)\w+|\{\w+\}`)

	// gin redirects a missing trailing slash, so "/api/movies" reaches "/api/movies/".
	key := func(method, path string) string {
		return method + " " + strings.TrimSuffix(param.ReplaceAllString(path, "{}"), "/")
	}

	registered := make(map[string]bool)
	for _, rt := range r.Routes() {
		registered[key(rt.Method, rt.Path)] = true
	}

	for _, list := range [][]apiEndpointDoc{
		apiEndpointsAll, apiEndpointsMovies, apiEndpointsSeries, apiEndpointsBooks,
		apiEndpointsAudiobooks, apiEndpointsMusic, apiEndpointsGeneral, apiEndpointsAdmin,
	} {
		for _, ep := range list {
			if !registered[key(ep.Method, ep.Path)] {
				t.Errorf("API testing page lists unregistered endpoint %s %s", ep.Method, ep.Path)
			}
		}
	}
}

func TestRedactConfigSecrets(t *testing.T) {
	in := map[string]any{
		"general":         config.GeneralConfig{WebAPIKey: "webkey", TheMovieDBApiKey: "tmdb", WebPort: "9090"},
		"downloader_qbit": config.DownloaderConfig{Name: "qbit", Password: "pw", Username: "user"},
		"indexer_x":       config.IndexersConfig{Name: "x", Apikey: "idxkey", URL: "http://x"},
	}

	out, ok := redactConfigSecrets(in).(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", redactConfigSecrets(in))
	}

	field := func(section, name string) any { return out[section].(map[string]any)[name] }

	for _, tc := range []struct {
		section, name string
		want          any
	}{
		{"general", "WebAPIKey", configRedacted},
		{"general", "TheMovieDBApiKey", configRedacted},
		{"general", "WebPort", "9090"},
		{"downloader_qbit", "Password", configRedacted},
		{"downloader_qbit", "Username", "user"},
		{"indexer_x", "Apikey", configRedacted},
		{"indexer_x", "URL", "http://x"},
	} {
		if got := field(tc.section, tc.name); got != tc.want {
			t.Errorf("%s.%s = %v, want %v", tc.section, tc.name, got, tc.want)
		}
	}
}

func TestMergeRedactedRestoresStoredSecrets(t *testing.T) {
	cur := map[string]any{"Name": "x", "Password": configRedacted, "Nested": []any{map[string]any{"Apikey": configRedacted}}}
	old := map[string]any{"Name": "x", "Password": "real", "Nested": []any{map[string]any{"Apikey": "k1"}}}
	if !mergeRedacted(cur, old) {
		t.Fatal("expected a replacement")
	}
	if cur["Password"] != "real" || cur["Nested"].([]any)[0].(map[string]any)["Apikey"] != "k1" {
		t.Fatalf("not restored: %#v", cur)
	}
	plain := map[string]any{"Password": "new"}
	if mergeRedacted(plain, old) || plain["Password"] != "new" {
		t.Fatalf("explicit new value must be kept: %#v", plain)
	}
}

func TestAffectsScheduler(t *testing.T) {
	for name, want := range map[string]bool{
		"general":           true,
		"scheduler_Default": true,
		"serie_EN":          true,
		"movie_EN":          true,
		"book_X":            true,
		"audiobook_X":       true,
		"music_X":           true,
		"downloader_qbit":   false,
		"indexer_nzb":       false,
		"quality_SD":        false,
		"list_trakt":        false,
		"imdb":              false,
	} {
		if got := affectsScheduler(name); got != want {
			t.Errorf("affectsScheduler(%q) = %v, want %v", name, got, want)
		}
	}
}
