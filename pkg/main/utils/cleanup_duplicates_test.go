package utils

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func useTestDB(t *testing.T) {
	t.Helper()

	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)
}

func exec(t *testing.T, query string, args ...any) {
	t.Helper()

	if err := database.ExecNErr(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func count(t *testing.T, query string, args ...any) int {
	t.Helper()

	return database.Getdatarow[int](false, query, args...)
}

func twoListConfig(isType uint) *config.MediaTypeConfig {
	return &config.MediaTypeConfig{
		Name: "EN", IsType: isType, ListsLen: 2, ListsNames: []string{"A", "B"}, ListsQu: ",?",
	}
}

// TestDuplicateSpecsMatchSchema checks every table and column the cleanup
// builds DELETE/UPDATE statements from.
func TestDuplicateSpecsMatchSchema(t *testing.T) {
	useTestDB(t)

	var specs []dupTableSpec

	for _, mt := range []uint{config.MediaTypeMovie, config.MediaTypeSeries, config.MediaTypeBook, config.MediaTypeAudiobook, config.MediaTypeMusic} {
		spec, ok := specForType(mt)
		if !ok {
			t.Fatalf("no spec for type %d", mt)
		}

		specs = append(specs, spec)

		trackers, _ := trackerSpecForType(mt)
		specs = append(specs, trackers...)
	}

	specs = append(specs, episodeDupSpec)

	for _, s := range specs {
		cols := map[string][]string{s.table: {"id"}}
		if s.idCol != "" {
			// dedupTableForConfig groups by idCol within the config's lists.
			cols[s.table] = append(cols[s.table], s.idCol, "listname")
		}

		if s.fileTable != "" {
			// pickKeeper scores rows with files by their quality profile.
			cols[s.table] = append(cols[s.table], "quality_profile")
			cols[s.fileTable] = append(cols[s.fileTable], s.fileFK)
		}

		if s.histTable != "" {
			cols[s.histTable] = append(cols[s.histTable], s.histFK)
		}

		if s.childTable != "" {
			cols[s.childTable] = append(cols[s.childTable], s.childFK)
		}

		for _, ref := range s.refs {
			cols[ref[0]] = append(cols[ref[0]], ref[1])
		}

		for table, cs := range cols {
			for _, c := range cs {
				if err := database.ExecNErr("select " + c + " from " + table + " limit 0"); err != nil {
					t.Errorf("%s: %s.%s: %v", s.label, table, c, err)
				}
			}
		}
	}

	if _, ok := specForType(9999); ok {
		t.Error("spec for unknown type")
	}

	if _, ok := trackerSpecForType(config.MediaTypeMovie); ok {
		t.Error("movies have no tracker tables")
	}
}

func TestDedupMoviesKeepsRowWithFiles(t *testing.T) {
	useTestDB(t)

	exec(t, "insert into dbmovies (id, title) values (1, 'Heat')")
	exec(t, "insert into movies (id, dbmovie_id, listname) values (1, 1, 'A'), (2, 1, 'B'), (3, 1, 'C')")
	exec(t, "insert into movie_files (location, movie_id, dbmovie_id) values ('/m/heat.mkv', 2, 1)")
	exec(t, "insert into movie_histories (title, movie_id, dbmovie_id) values ('Heat', 1, 1)")

	spec, _ := specForType(config.MediaTypeMovie)
	cfgp := twoListConfig(config.MediaTypeMovie)

	// Dry run: reports, changes nothing.
	if g, d := dedupTableForConfig(spec, cfgp, config.MediaTypeMovie, false); g != 1 || d != 1 {
		t.Fatalf("dry run = %d groups, %d deleted", g, d)
	}

	if count(t, "select count() from movies") != 3 {
		t.Fatal("dry run deleted rows")
	}

	if g, d := dedupTableForConfig(spec, cfgp, config.MediaTypeMovie, true); g != 1 || d != 1 {
		t.Fatalf("apply = %d groups, %d deleted", g, d)
	}

	// Row 2 has the file and stays; row 1 and its history go; row 3 is in
	// another config's list and is left alone.
	if count(t, "select count() from movies where id = 2") != 1 || count(t, "select count() from movies where id = 1") != 0 ||
		count(t, "select count() from movies where id = 3") != 1 {
		t.Errorf("wrong rows kept: %d left", count(t, "select count() from movies"))
	}

	if count(t, "select count() from movie_histories") != 0 || count(t, "select count() from movie_files") != 1 {
		t.Error("children of the deleted row not removed, or keeper's file lost")
	}
}

func TestDedupSeriesDeletesEpisodes(t *testing.T) {
	useTestDB(t)

	exec(t, "insert into dbseries (id, seriename) values (1, 'Dark')")
	exec(t, "insert into series (id, dbserie_id, listname) values (1, 1, 'A'), (2, 1, 'B')")
	exec(t, "insert into serie_episodes (id, serie_id, dbserie_episode_id) values (10, 1, 100), (20, 2, 100)")

	spec, _ := specForType(config.MediaTypeSeries)

	if _, d := dedupTableForConfig(spec, twoListConfig(config.MediaTypeSeries), config.MediaTypeSeries, true); d != 1 {
		t.Fatalf("deleted %d", d)
	}

	// Without files the lowest id is kept, and the other row's episodes go with it.
	if count(t, "select count() from series where id = 1") != 1 || count(t, "select count() from serie_episodes where serie_id = 2") != 0 ||
		count(t, "select count() from serie_episodes where serie_id = 1") != 1 {
		t.Error("series dedupe kept or deleted the wrong rows")
	}
}

func TestDedupTrackerRepointsReferences(t *testing.T) {
	useTestDB(t)

	exec(t, "insert into dbauthors (id, name) values (1, 'Stephen Fry')")
	exec(t, "insert into authors (id, dbauthor_id, listname) values (1, 1, 'A'), (2, 1, 'B')")
	exec(t, "insert into audiobooks (id, listname, author_id) values (5, 'B', 2)")
	exec(t, "insert into book_series (id, listname, author_id) values (7, 'B', 2)")

	specs, _ := trackerSpecForType(config.MediaTypeAudiobook)

	if _, d := dedupTableForConfig(specs[0], twoListConfig(config.MediaTypeAudiobook), config.MediaTypeAudiobook, true); d != 1 {
		t.Fatalf("deleted %d", d)
	}

	if count(t, "select count() from authors") != 1 || count(t, "select author_id from audiobooks where id = 5") != 1 ||
		count(t, "select author_id from book_series where id = 7") != 1 {
		t.Error("references to the removed author were not repointed to the keeper")
	}
}

func TestPickKeeperWithoutQualityFallsBackToFirstWithFiles(t *testing.T) {
	useTestDB(t)

	exec(t, "insert into albums (id, dbalbum_id, listname, quality_profile) values (1, 1, 'A', 'missing'), (2, 1, 'B', ''), (3, 1, 'A', '')")
	exec(t, "insert into album_files (location, album_id) values ('/a/2.flac', 2), ('/a/3.flac', 3)")

	spec, _ := specForType(config.MediaTypeMusic)

	if got := pickKeeper(spec, []uint{1, 2, 3}, config.MediaTypeMusic); got != 2 {
		t.Errorf("pickKeeper = %d, want 2 (first row with files)", got)
	}

	if got := pickKeeper(spec, []uint{1}, config.MediaTypeMusic); got != 1 {
		t.Errorf("no files = %d, want lowest id", got)
	}
}

func TestCleanupDuplicateEpisodes(t *testing.T) {
	useTestDB(t)

	exec(t, "insert into serie_episodes (id, serie_id, dbserie_episode_id) values (10, 1, 100), (11, 1, 100), (12, 1, 101), (13, 2, 100)")
	exec(t, "insert into serie_episode_files (location, serie_episode_id, serie_id) values ('/tv/e1.mkv', 11, 1)")
	exec(t, "insert into serie_episode_histories (title, serie_episode_id, serie_id) values ('e1', 10, 1)")

	if g, d := cleanupDuplicateEpisodes(false); g != 1 || d != 1 || count(t, "select count() from serie_episodes") != 4 {
		t.Fatalf("dry run = %d %d", g, d)
	}

	if g, d := cleanupDuplicateEpisodes(true); g != 1 || d != 1 {
		t.Fatalf("apply = %d %d", g, d)
	}

	if count(t, "select count() from serie_episodes where id = 11") != 1 || count(t, "select count() from serie_episodes where id = 10") != 0 ||
		count(t, "select count() from serie_episode_histories") != 0 || count(t, "select count() from serie_episodes") != 3 {
		t.Error("episode dedupe kept the wrong row or left children")
	}
}
