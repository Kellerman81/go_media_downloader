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
