package importfeed

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

func TestImportQualityConfigAndYearCandidate(t *testing.T) {
	strict := &config.QualityConfig{Name: "strict"}
	lenient := &config.QualityConfig{Name: "lenient", CheckYear1: true}

	cfgp := &config.MediaTypeConfig{
		CfgQuality: strict,
		Lists: []config.MediaListsConfig{
			{Name: "a", CfgQuality: strict},
			{Name: "b", CfgQuality: lenient},
		},
	}

	m := &database.ParseInfo{ListID: 1, Year: 2000}
	if q := importQualityConfig(m, cfgp); q != lenient {
		t.Fatalf("importQualityConfig picked %v, want list quality", q)
	}

	m.ListID = -1
	if q := importQualityConfig(m, cfgp); q != strict {
		t.Fatalf("importQualityConfig fallback picked %v, want media default", q)
	}

	arr := []database.DbstaticOneStringOneInt{
		{Str: "tt1", Num: 2001},
		{Str: "tt2", Num: 2000},
	}

	m.ListID = 1
	if got := pickImdbYearCandidate(arr, m, cfgp); got != 1 {
		t.Errorf("exact year should win, got %d", got)
	}

	if got := pickImdbYearCandidate(arr[:1], m, cfgp); got != 0 {
		t.Errorf("year+1 with CheckYear1 should match, got %d", got)
	}

	m.ListID = 0
	if got := pickImdbYearCandidate(arr[:1], m, cfgp); got != -1 {
		t.Errorf("year+1 without CheckYear1 must not match, got %d", got)
	}

	m.Year = 0
	if got := pickImdbYearCandidate(arr[:1], m, cfgp); got != 0 {
		t.Errorf("single yearless candidate should match, got %d", got)
	}

	if got := pickImdbYearCandidate(arr, m, cfgp); got != -1 {
		t.Errorf("ambiguous yearless candidates must not match, got %d", got)
	}
}

func TestCheckaddmovieentryNotifiesAddedToList(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr("insert into dbmovies (id, title, year, imdb_id) values (1, 'Heat', 1995, 'tt0113277')"); err != nil {
		t.Fatal(err)
	}

	cfgp, file := csvNotifyConfig(t, config.MediaTypeMovie, providers.EventAddedToList)
	cfgp.Lists = []config.MediaListsConfig{{Name: "EN-watch"}}
	cfgp.ListsNames = []string{"EN-watch"}
	cfgp.ListsLen = 1
	cfgp.ListsQu = ""

	dbid := uint(1)
	for range 2 {
		if err := Checkaddmovieentry(&dbid, cfgp, &cfgp.Lists[0], "tt0113277"); err != nil {
			t.Fatal(err)
		}
	}

	if got := readFile(t, file); got != "added_to_list;movie;Heat;1995;tt0113277;EN-watch\n" {
		t.Errorf("csv %q, want one added_to_list line", got)
	}
}

// TestCheckaddmovieentryExclusion: an excluded movie isn't added to the
// lists of its media group, but still to another group's.
func TestCheckaddmovieentryExclusion(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr("insert into dbmovies (id, title, imdb_id) values (1, 'Old', 'tt0001')"); err != nil {
		t.Fatal(err)
	}

	if err := database.AddImportExclusion("movie", "movie_DE", new(uint(1)), "Old"); err != nil {
		t.Fatal(err)
	}

	count := func(list string) int {
		return database.Getdatarow[int](false, "select count() from movies where dbmovie_id = 1 and listname = ?", list)
	}

	de := &config.MediaTypeConfig{Name: "DE", NamePrefix: "movie_DE", Lists: []config.MediaListsConfig{{Name: "DE List"}}}
	en := &config.MediaTypeConfig{Name: "EN", NamePrefix: "movie_EN", Lists: []config.MediaListsConfig{{Name: "EN List"}}}

	if err := Checkaddmovieentry(new(uint(1)), de, &de.Lists[0], "tt0001"); err != nil || count("DE List") != 0 {
		t.Errorf("excluded group: %v, %d rows", err, count("DE List"))
	}

	if err := Checkaddmovieentry(new(uint(1)), en, &en.Lists[0], "tt0001"); err != nil || count("EN List") != 1 {
		t.Errorf("other group: %v, %d rows", err, count("EN List"))
	}
}

// TestMovieOfNewImdb: a list's new IMDb id of a film the library has under
// an old one (same TMDB id) finds that row and switches it to the new id;
// an IMDb id of an unknown film finds nothing.
func TestMovieOfNewImdb(t *testing.T) {
	useTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/find/tt35775356":
			w.Write([]byte(`{"movie_results":[{"id":1474447,"title":"To Barcelona, with Love"}]}`))
		case "/3/find/tt99999999":
			w.Write([]byte(`{"movie_results":[{"id":5}]}`))
		default:
			w.Write([]byte(`{"movie_results":[]}`))
		}
	}))
	t.Cleanup(srv.Close)

	providers.SetTMDB(tmdb.NewProviderWithConfig(base.ClientConfig{Name: "tmdb", BaseURL: srv.URL + "/3"}, "key"))
	t.Cleanup(func() { providers.SetTMDB(nil) })

	if err := database.ExecNErr("insert into dbmovies (id, title, year, imdb_id, moviedb_id) values (9538, 'To Barcelona, with Love', 2025, 'tt36778003', 1474447)"); err != nil {
		t.Fatal(err)
	}

	if id := movieOfNewImdb("tt35775356", "de"); id != 9538 {
		t.Fatalf("known film: %d, want 9538", id)
	}

	if imdb := database.Getdatarow[string](false, "select imdb_id from dbmovies where id = 9538"); imdb != "tt35775356" {
		t.Errorf("imdb id %q, want the new tt35775356", imdb)
	}

	for _, imdb := range []string{"tt99999999", "tt00000001"} {
		if id := movieOfNewImdb(imdb, ""); id != 0 {
			t.Errorf("%s: %d, want 0", imdb, id)
		}
	}

	if n := database.Getdatarow[int](false, "select count() from dbmovies"); n != 1 {
		t.Errorf("%d dbmovies rows", n)
	}
}
