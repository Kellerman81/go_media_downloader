package api

import (
	"database/sql"
	"testing"
)

func lookupIDs(t *testing.T, db *sql.DB, query string, args ...any) []int {
	t.Helper()

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()

	var ids []int

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}

		ids = append(ids, id)
	}

	return ids
}

func TestManualAddLookupsAgainstSchema(t *testing.T) {
	db := openMigratedSchema(t)

	for _, q := range []string{
		"INSERT INTO dbseries (id, seriename, thetvdb_id) VALUES (1, 'Known Show', 100), (2, 'Bare Show', 0)",
		"INSERT INTO dbauthors (id, name) VALUES (1, 'Jane Doe')",
		"INSERT INTO dbaudiobooks (id, title, asin) VALUES (1, 'Book One', 'B001'), (2, 'Book Two', '')",
		"INSERT INTO dbaudiobook_authors (dbaudiobook_id, dbauthor_id, role) VALUES (2, 1, 'author')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	tests := []struct {
		name  string
		query string
		args  []any
		want  int // 0 = no match
	}{
		{"serie by tvdb", manualSerieByTvdbQuery, []any{100}, 1},
		{"serie by name, no tvdb given", manualSerieByNameQuery, []any{"known show", 0}, 1},
		{"serie name of a different tvdb show", manualSerieByNameQuery, []any{"Known Show", 200}, 0},
		{"serie name of a row without tvdb", manualSerieByNameQuery, []any{"BARE SHOW", 300}, 2},
		{"audiobook by asin", manualAudiobookByASINQuery, []any{"B001"}, 1},
		{"audiobook by title, no asin given", manualAudiobookByTitleQuery, []any{"book one", ""}, 1},
		{"audiobook title of a different asin", manualAudiobookByTitleQuery, []any{"Book One", "B999"}, 0},
		{"audiobook by title and author", manualAudiobookByTitleAuthorQuery, []any{"book two", "", "jane doe"}, 2},
		{"audiobook title, other author", manualAudiobookByTitleAuthorQuery, []any{"Book Two", "", "John Roe"}, 0},
	}

	for _, tt := range tests {
		ids := lookupIDs(t, db, tt.query, tt.args...)

		got := 0
		if len(ids) > 0 {
			got = ids[0]
		}

		if got != tt.want {
			t.Errorf("%s: got id %d, want %d", tt.name, got, tt.want)
		}
	}
}
