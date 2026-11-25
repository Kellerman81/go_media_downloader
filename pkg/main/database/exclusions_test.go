package database

import "testing"

// TestImportExclusions: an exclusion holds for its media group only, by
// the row and by its external id (a re-created row still matches), and
// can be removed.
func TestImportExclusions(t *testing.T) {
	useUsersTestDB(t)

	for _, q := range []string{
		"insert into dbmovies (id, title, imdb_id) values (1, 'Old', 'tt0001'), (2, 'Other', 'tt0002'), (3, 'Old again', 'tt0001')",
	} {
		if err := ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	if err := AddImportExclusion("movie", "movie_DE", new(uint(1)), "Old"); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		group string
		id    uint
		want  bool
	}{
		"same row":     {"movie_DE", 1, true},
		"same IMDb ID": {"movie_de", 3, true},
		"other movie":  {"movie_DE", 2, false},
		"other group":  {"movie_EN", 1, false},
		"no row":       {"movie_DE", 0, false},
	} {
		if got := ImportExcluded("movie", tc.group, &tc.id); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}

	ex := GetImportExclusions()
	if len(ex) != 1 || ex[0].ExtID != "tt0001" || ex[0].Title != "Old" {
		t.Fatalf("exclusions: %+v", ex)
	}

	if err := DeleteImportExclusion(&ex[0].ID); err != nil || ImportExcluded("movie", "movie_DE", new(uint(1))) {
		t.Errorf("still excluded after removing: %v", err)
	}
}

// TestMonitoredCollections: switching a collection on and off for a list.
func TestMonitoredCollections(t *testing.T) {
	useUsersTestDB(t)

	if err := MonitorCollection(10, "Saga", "EN List", true); err != nil || !CollectionMonitored(10, "en list") {
		t.Fatalf("monitored: %v", err)
	}

	if err := MonitorCollection(10, "Saga Collection", "EN List", true); err != nil {
		t.Fatal(err)
	}

	if m := GetMonitoredCollections(); len(m) != 1 || m[0].Name != "Saga Collection" {
		t.Errorf("collections: %+v", m)
	}

	if err := MonitorCollection(10, "", "EN List", false); err != nil || CollectionMonitored(10, "EN List") {
		t.Errorf("still monitored: %v", err)
	}
}

// TestSearchExclusionCandidates: by title part or whole external id,
// exact titles first, with whether the row is excluded in the group or in
// one of its lists; ExclusionTitle tells a missing row apart.
func TestSearchExclusionCandidates(t *testing.T) {
	useUsersTestDB(t)

	for _, q := range []string{
		"insert into dbmovies (id, title, year, imdb_id) values (1, 'The Matrix Reloaded', 2003, 'tt0234215'), " +
			"(2, 'The Matrix', 1999, 'tt0133093'), (3, 'Other', 2000, 'tt0000003'), (4, 'Matrix copy', 0, 'tt0133093')",
		"insert into movies (dbmovie_id, listname) values (1, 'EN Watch')",
		"insert into dbseries (id, seriename, firstaired, thetvdb_id) values (5, 'The Matrix Show', '2010-01-01', 77)",
	} {
		if err := ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	if err := AddImportExclusion("movie", "movie_EN", new(uint(2)), "The Matrix"); err != nil {
		t.Fatal(err)
	}

	got := SearchExclusionCandidates("movie", "movie_EN", []string{"en watch"}, "the matrix", 10)
	if len(got) != 2 || got[0].DBID != 2 || !got[0].Excluded || got[0].Year != 1999 || got[0].Listed ||
		got[1].DBID != 1 || got[1].Excluded || !got[1].Listed {
		t.Errorf("by title: %+v", got)
	}

	// The copy shares the excluded IMDb ID; another group has no exclusion.
	if got := SearchExclusionCandidates("movie", "movie_DE", nil, "tt0133093", 10); len(got) != 2 || got[0].Excluded || got[1].Excluded {
		t.Errorf("by id in another group: %+v", got)
	}

	if got := SearchExclusionCandidates("movie", "movie_EN", nil, "tt0133093", 10); len(got) != 2 || !got[0].Excluded || !got[1].Excluded {
		t.Errorf("by id: %+v", got)
	}

	if got := SearchExclusionCandidates("series", "serie_EN", nil, "77", 10); len(got) != 1 || got[0].Title != "The Matrix Show" ||
		got[0].Year != 2010 || got[0].ExtID != "77" {
		t.Errorf("series by TVDB ID: %+v", got)
	}

	if got := SearchExclusionCandidates("movie", "movie_EN", nil, "  ", 10); got != nil {
		t.Errorf("empty search: %+v", got)
	}

	if title, ok := ExclusionTitle("movie", new(uint(3))); !ok || title != "Other" {
		t.Errorf("title of 3: %q %v", title, ok)
	}

	if _, ok := ExclusionTitle("movie", new(uint(99))); ok {
		t.Error("title of a missing row")
	}
}
