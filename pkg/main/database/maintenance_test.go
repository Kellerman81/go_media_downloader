package database

import (
	"testing"
)

func TestCleanupOrphans(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbalbums (id, title, slug) values (1, 'Kept', 'kept')")
	mustExec(t, db, "insert into albums (id, dbalbum_id, listname) values (1, 1, 'Rock')")
	mustExec(t, db, "insert into albums (id, dbalbum_id, listname) values (2, 99, 'Rock')") // parent missing
	mustExec(t, db, "insert into album_files (location, album_id, dbalbum_id) values ('/a/1.flac', 1, 1)")
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
