package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/pelletier/go-toml/v2"
)

func TestOmdbValue(t *testing.T) {
	tests := map[string]string{
		"":         "",
		"N/A":      "",
		" N/A ":    "",
		"0":        "0",
		"7.4":      "7.4",
		"120 min":  "120 min",
		"1,234":    "1,234",
		"Drama":    "Drama",
		"2010":     "2010",
		"10 min":   "10 min",
		"0.0 rate": "0.0 rate",
	}

	for in, want := range tests {
		if got := omdbValue(in); got != want {
			t.Errorf("omdbValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShouldUpdateRuntime(t *testing.T) {
	tests := []struct {
		current, next int
		overwrite     bool
		want          bool
	}{
		{0, 0, true, false},
		{0, 3, false, false}, // placeholder runtimes are never accepted
		{0, 95, false, true},
		{90, 95, false, false},
		{90, 95, true, true},
		{2, 95, false, true}, // a placeholder current value is replaced
		{90, 1, true, false},
	}

	for _, tt := range tests {
		if got := shouldUpdateRuntime(tt.current, tt.next, tt.overwrite); got != tt.want {
			t.Errorf("shouldUpdateRuntime(%d, %d, %t) = %t, want %t", tt.current, tt.next, tt.overwrite, got, tt.want)
		}
	}
}

func TestShouldUpdateSerieRuntime(t *testing.T) {
	tests := []struct {
		current   string
		next      int
		overwrite bool
		want      bool
	}{
		{"", 0, true, false},
		{"", 45, false, true},
		{"0", 45, false, true},
		{"3", 45, false, true},
		{"42", 45, false, false},
		{"42", 45, true, true},
		{"", 2, false, false},
		{"42", 2, true, false},
	}

	for _, tt := range tests {
		if got := shouldUpdateSerieRuntime(tt.current, tt.next, tt.overwrite); got != tt.want {
			t.Errorf("shouldUpdateSerieRuntime(%q, %d, %t) = %t, want %t", tt.current, tt.next, tt.overwrite, got, tt.want)
		}
	}
}

func TestBuildCommaSeparated(t *testing.T) {
	type genre struct{ Name string }

	name := func(g genre) string { return g.Name }

	if got := buildCommaSeparated(nil, name); got != "" {
		t.Errorf("empty = %q", got)
	}

	if got := buildCommaSeparated([]genre{{"Drama"}}, name); got != "Drama" {
		t.Errorf("single = %q", got)
	}

	if got := buildCommaSeparated([]genre{{"Drama"}, {"Crime"}, {"Thriller"}}, name); got != "Drama,Crime,Thriller" {
		t.Errorf("multiple = %q", got)
	}
}

// TestExtrasConversion: the TMDB details become the saved cast (at most
// database.MaxStoredCast) and related titles.
func TestExtrasConversion(t *testing.T) {
	credits := &apiexternal_v2.Credits{}
	for i := range database.MaxStoredCast + 3 {
		credits.Cast = append(credits.Cast, apiexternal_v2.CastMember{ID: i + 1, Name: "Actor", Character: "Role", ProfilePath: "/p.jpg"})
	}

	if c := CastFrom(credits); len(c) != database.MaxStoredCast || c[0].PersonID != 1 || c[0].Profile != "/p.jpg" {
		t.Fatalf("cast: %d %+v", len(c), c[:1])
	}

	if CastFrom(nil) != nil {
		t.Fatal("no credits")
	}

	r := RelatedMovies([]apiexternal_v2.MovieSearchResult{{ID: 584, Title: "2 Fast 2 Furious", ReleaseDate: time.Date(2003, 6, 5, 0, 0, 0, 0, time.UTC), VoteAverage: 6.4}})
	if len(r) != 1 || r[0].TMDB != 584 || r[0].Year != 2003 || r[0].Rating != 6.4 {
		t.Fatalf("related: %+v", r)
	}

	if s := RelatedSeries([]apiexternal_v2.SeriesSearchResult{{ID: 1, Name: "Show"}}); len(s) != 1 || s[0].Year != 0 || s[0].Title != "Show" {
		t.Fatalf("series: %+v", s)
	}
}

// TestTmdbMovieDetails: a movie without an English overview (a German
// production) gets the overview of the group's language; one with an
// English overview, or a group in English, costs no second request.
func TestTmdbMovieDetails(t *testing.T) {
	var requests []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("language")
		requests = append(requests, r.URL.Path+" "+lang)

		switch {
		case r.URL.Path == "/3/movie/1" && lang == "de":
			w.Write([]byte(`{"id":1,"title":"Terra X - Wilde Winterwelten","overview":"Mit Einbruch der Kälte","tagline":"Winter"}`))
		case r.URL.Path == "/3/movie/1":
			w.Write([]byte(`{"id":1,"title":"Terra X - Wilde Winterwelten","overview":""}`))
		default:
			w.Write([]byte(`{"id":2,"title":"Inception","overview":"A thief"}`))
		}
	}))
	t.Cleanup(srv.Close)

	providers.SetTMDB(tmdb.NewProviderWithConfig(base.ClientConfig{Name: "tmdb", BaseURL: srv.URL + "/3"}, "key"))
	t.Cleanup(func() { providers.SetTMDB(nil) })

	ctx := context.Background()

	d, err := TmdbMovieDetails(ctx, 1, "de")
	if err != nil || d.Overview != "Mit Einbruch der Kälte" || d.Tagline != "Winter" || d.Title != "Terra X - Wilde Winterwelten" {
		t.Fatalf("german only: %+v, %v", d, err)
	}

	if d, _ := TmdbMovieDetails(ctx, 1, "en"); d.Overview != "" {
		t.Errorf("english group got %q", d.Overview)
	}

	requests = nil

	if d, _ := TmdbMovieDetails(ctx, 2, "de"); d.Overview != "A thief" || len(requests) != 1 {
		t.Errorf("english overview: %q, requests %q", d.Overview, requests)
	}
}

// TestListedLanguage: the fallback language of a title is the first
// non-English metadata language of the groups listing it.
func TestListedLanguage(t *testing.T) {
	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() { config.Configfile = oldFile })

	group := func(name, lang, list string) config.MediaTypeConfig {
		return config.MediaTypeConfig{
			Name: name, MetadataLanguage: lang, TemplateQuality: "HD", TemplateScheduler: "Default",
			Data:  []config.MediaDataConfig{{TemplatePath: "alpha"}},
			Lists: []config.MediaListsConfig{{Name: list, TemplateList: "listA", TemplateQuality: "HD", Enabled: true}},
		}
	}

	cfg := config.MainConfig{
		General: config.GeneralConfig{WorkerFiles: 1, WorkerParse: 1, WorkerSearch: 1},
		Media: config.MediaConfig{
			Movies: []config.MediaTypeConfig{group("EN", "en", "EN List"), group("DE", "de", "DE List")},
			Series: []config.MediaTypeConfig{group("X", "de", "X List")},
		},
		Lists:     []config.ListsConfig{{Name: "listA"}},
		Paths:     []config.PathsConfig{{Name: "alpha"}},
		Quality:   []config.QualityConfig{{Name: "HD"}},
		Scheduler: []config.SchedulerConfig{{Name: "Default", IntervalImdb: "3d"}},
	}

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

	for _, q := range []string{
		"INSERT INTO dbmovies (id, title) VALUES (1, 'a'), (2, 'b'), (3, 'c')",
		"INSERT INTO movies (dbmovie_id, listname) VALUES (1, 'EN List'), (2, 'DE List'), (3, 'EN List'), (3, 'DE List')",
		"INSERT INTO dbseries (id, seriename, thetvdb_id) VALUES (5, 's', 81189)",
		"INSERT INTO series (dbserie_id, listname) VALUES (5, 'X List')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	for _, tc := range []struct {
		typ  string
		id   uint
		tvdb int
		want string
	}{
		{"movie", 1, 0, ""}, {"movie", 2, 0, "de"}, {"movie", 3, 0, "de"}, {"movie", 9, 0, ""},
		{"series", 5, 0, "de"}, {"series", 0, 81189, "de"}, {"series", 0, 999, ""},
	} {
		if got := ListedLanguage(tc.typ, tc.id, tc.tvdb); got != tc.want {
			t.Errorf("%s %d/%d: %q, want %q", tc.typ, tc.id, tc.tvdb, got, tc.want)
		}
	}
}
