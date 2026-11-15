package database

import (
	"maps"
	"testing"
)

func TestCleanupOrphans(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbalbums (id, title, slug) values (1, 'Kept', 'kept')")
	mustExec(t, db, "insert into albums (id, dbalbum_id, listname) values (1, 1, 'Rock')")
	mustExec(t, db, "insert into albums (id, dbalbum_id, listname) values (2, 99, 'Rock')") // parent missing
	mustExec(t, db, "insert into album_files (location, extension, album_id, dbalbum_id) values ('/a/1.flac', '.flac', 1, 1)")
	mustExec(t, db, "insert into album_files (location, album_id, dbalbum_id) values ('/a/2.flac', 2, 99)") // orphaned in pass 2

	count := func(table string) int {
		var n int
		if err := db.Get(&n, "select count(*) from "+table); err != nil {
			t.Fatal(err)
		}

		return n
	}

	dry, err := CleanupOrphans(true)
	if err != nil {
		t.Fatal(err)
	}

	if dry["albums"] != 1 || count("albums") != 2 {
		t.Errorf("dry run = %v, albums left %d", dry, count("albums"))
	}

	deleted, err := CleanupOrphans(false)
	if err != nil {
		t.Fatal(err)
	}

	if deleted["albums"] != 1 || deleted["album_files"] != 1 {
		t.Errorf("deleted = %v, want 1 album and 1 cascaded album file", deleted)
	}

	if count("albums") != 1 || count("album_files") != 1 || count("dbalbums") != 1 {
		t.Errorf("left: albums %d, album_files %d, dbalbums %d", count("albums"), count("album_files"), count("dbalbums"))
	}

	if again, err := CleanupOrphans(false); err != nil || len(again) != 0 {
		t.Errorf("second run = %v, %v", again, err)
	}
}

func TestRunConsistencyCheck(t *testing.T) {
	db := useMigratedMainDB(t)

	// Movie 1 has its file recorded twice and a folder recorded as a file; one
	// file row belongs to no movie.
	mustExec(t, db, "insert into dbmovies (id, title) values (1, 'M')")
	mustExec(t, db, "insert into movies (id, dbmovie_id, listname, missing) values (1, 1, 'EN', 0)")
	mustExec(t, db, "insert into movie_files (id, location, extension, movie_id, dbmovie_id) values (1, '/m/a.mkv', '.mkv', 1, 1)")
	mustExec(t, db, "insert into movie_files (id, location, extension, movie_id, dbmovie_id) values (2, '/m/a.mkv', '.mkv', 1, 1)")
	mustExec(t, db, "insert into movie_files (id, location, extension, movie_id, dbmovie_id) values (3, '/m/b.mkv', '.mkv', 0, 0)")
	mustExec(t, db, "insert into movie_files (id, location, extension, movie_id, dbmovie_id) values (4, '/m/folder', '', 1, 1)")

	// Album 1 has files but is marked missing; album 2 has none, is not missing
	// and claims its quality is reached.
	mustExec(t, db, "insert into dbalbums (id, title, slug) values (1, 'A', 'a'), (2, 'B', 'b')")
	mustExec(t, db, "insert into albums (id, dbalbum_id, listname, missing) values (1, 1, 'Music', 1)")
	mustExec(t, db, "insert into albums (id, dbalbum_id, listname, missing, quality_reached) values (2, 2, 'Music', 0, 1)")
	mustExec(t, db, "insert into album_files (location, extension, album_id, dbalbum_id) values ('/a/1.flac', '.flac', 1, 1)")

	// Series 1 knows episodes 1-3 but only has rows for 1 and 2, which share
	// one multi-episode file (not a duplicate).
	mustExec(t, db, "insert into dbseries (id, seriename) values (1, 'S')")
	mustExec(t, db, "insert into dbserie_episodes (id, dbserie_id, season, episode, identifier) values (1, 1, '1', '1', 'S01E01'), (2, 1, '1', '2', 'S01E02'), (3, 1, '1', '3', 'S01E03')")
	mustExec(t, db, "insert into series (id, dbserie_id, listname, quality_profile) values (1, 1, 'EN', 'SD')")
	mustExec(t, db, "insert into serie_episodes (id, dbserie_id, serie_id, dbserie_episode_id, missing, quality_profile) values (1, 1, 1, 1, 0, 'SD'), (2, 1, 1, 2, 0, 'SD')")
	mustExec(t, db, "insert into serie_episode_files (location, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, extension) values ('/s/E01E02.mkv', 1, 1, 1, 1, '.mkv'), ('/s/E01E02.mkv', 1, 2, 2, 1, '.mkv')")

	want := map[string]map[string]int64{
		"emptyrefs":    {"movie_files": 1},
		"noextension":  {"movie_files": 1},
		"orphans":      {},
		"dupfiles":     {"movie_files": 1},
		"episodelinks": {"serie_episodes": 1},
		"missingflag":  {"albums": 2},
		"reachedflag":  {"albums": 1},
	}

	checks := ConsistencyChecks()
	if len(checks) != len(want) {
		t.Fatalf("%d checks, want %d", len(checks), len(want))
	}

	for _, repair := range []bool{false, true} {
		for _, c := range checks {
			res, err := RunConsistencyCheck(c.Key, repair, 5, nil)
			if err != nil {
				t.Fatalf("%s repair=%v: %v", c.Key, repair, err)
			}

			if !maps.Equal(res.Counts, want[c.Key]) {
				t.Errorf("%s repair=%v: counts %v, want %v", c.Key, repair, res.Counts, want[c.Key])
			}

			if !repair && res.Total() > 0 && len(res.Samples) == 0 {
				t.Errorf("%s: no samples", c.Key)
			}
		}
	}

	for _, c := range checks {
		if res, err := RunConsistencyCheck(c.Key, false, 5, nil); err != nil || res.Total() != 0 {
			t.Errorf("%s after repair: %v, %v", c.Key, res.Counts, err)
		}
	}

	var got []string
	if err := db.Select(&got, "select missing || '/' || quality_profile from serie_episodes where dbserie_episode_id = 3"); err != nil ||
		len(got) != 1 || got[0] != "1/SD" {
		t.Errorf("new episode row = %v, %v; want missing with the series quality", got, err)
	}

	if _, err := RunConsistencyCheck("nope", false, 5, nil); err == nil {
		t.Error("unknown check: no error")
	}
}
