package importfeed

import (
	"testing"

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
