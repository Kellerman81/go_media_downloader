package importfeed

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

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
