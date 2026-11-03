package importfeed

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/jmoiron/sqlx"
)

func TestIsUPC(t *testing.T) {
	tests := map[string]bool{
		"123456789012":    true,
		"1234567890123":   true,
		"12345678901234":  true,
		"12345678901":     false,
		"123456789012345": false,
		"12345678901a":    false,
		"":                false,
	}

	for in, want := range tests {
		if got := isUPC(in); got != want {
			t.Errorf("isUPC(%q) = %t, want %t", in, got, want)
		}
	}
}

// TestMediaEntrySpecsMatchSchema checks every table and column the shared
// list-entry code builds SQL from against the migrated schema.
func TestMediaEntrySpecsMatchSchema(t *testing.T) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	files, _ := filepath.Glob(filepath.Join("..", "schema", "db", "*.up.sql"))
	if len(files) == 0 {
		t.Fatal("no migrations found")
	}

	sort.Strings(files)

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(f), err)
		}
	}

	for _, spec := range []mediaListEntrySpec{bookEntrySpec(), audiobookEntrySpec(), albumEntrySpec()} {
		for _, q := range []string{
			spec.itemInsertSQL,
			"select " + spec.idCol + " from " + spec.table,
			"select " + spec.joinParentCol + " from " + spec.joinTable,
			"select " + spec.parentIDCol + ", track_mode from " + spec.parentTable,
		} {
			stmt, err := db.Prepare(q)
			if err != nil {
				t.Errorf("%s: %q: %v", spec.logItemType, q, err)
				continue
			}

			stmt.Close()
		}

		if spec.ignoredErr == nil || spec.cacheKey == "" {
			t.Errorf("%s: incomplete spec %+v", spec.logItemType, spec)
		}
	}
}

func TestEnsureAlbumListEntry(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr("insert into dbalbums (id, title) values (1, 'Synthetic')"); err != nil {
		t.Fatal(err)
	}

	cfgp := &config.MediaTypeConfig{
		Lists:      []config.MediaListsConfig{{Name: "Music Library"}},
		ListsNames: []string{"Music Library"},
		ListsLen:   1,
	}

	id := uint(1)

	// listid -1 only looks up: an unlisted album stays unlisted.
	if got := EnsureAlbumListEntry(&id, cfgp, -1); got != 0 {
		t.Fatalf("lookup-only returned entry %d for an unlisted album", got)
	}

	entry := EnsureAlbumListEntry(&id, cfgp, 0)
	if entry == 0 {
		t.Fatal("no entry created in the add_found list")
	}

	if got := database.Getdatarow[string](false, "select listname from albums where id = ?", &entry); got != "Music Library" {
		t.Errorf("entry list = %q", got)
	}

	// A second call finds the entry instead of adding another one.
	if again := EnsureAlbumListEntry(&id, cfgp, 0); again != entry {
		t.Errorf("second call = %d, want existing entry %d", again, entry)
	}

	if got := database.Getdatarow[int](false, "select count() from albums where dbalbum_id = 1"); got != 1 {
		t.Errorf("list entries = %d, want 1", got)
	}
}

func TestEnsureAudiobookListEntry(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr("insert into dbaudiobooks (id, title) values (1, 'Unlisted')"); err != nil {
		t.Fatal(err)
	}

	cfgp := &config.MediaTypeConfig{
		Lists:      []config.MediaListsConfig{{Name: "Audiobook Library"}},
		ListsNames: []string{"Audiobook Library"},
		ListsLen:   1,
	}

	id := uint(1)

	if got := EnsureAudiobookListEntry(&id, cfgp, -1); got != 0 {
		t.Fatalf("lookup-only returned entry %d for an unlisted audiobook", got)
	}

	entry := EnsureAudiobookListEntry(&id, cfgp, 0)
	if entry == 0 {
		t.Fatal("no entry created in the add_found list")
	}

	if again := EnsureAudiobookListEntry(&id, cfgp, 0); again != entry {
		t.Errorf("second call = %d, want existing entry %d", again, entry)
	}

	if got := database.Getdatarow[string](false, "select listname from audiobooks where dbaudiobook_id = 1"); got != "Audiobook Library" {
		t.Errorf("entry list = %q", got)
	}
}
