package api

import (
	"database/sql"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// openMigratedSchema builds an in-memory SQLite database from the real
// migrations so filter SQL can be compiled (EXPLAIN) against the actual schema.
func openMigratedSchema(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	files, err := filepath.Glob(filepath.Join("..", "schema", "db", "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
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

	return db
}

func tableColumns(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()

	rows, err := db.Query("SELECT name, type FROM pragma_table_info(?)", table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	cols := map[string]string{}

	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			t.Fatal(err)
		}

		cols[strings.ToLower(name)] = strings.ToUpper(typ)
	}

	return cols
}

func explainOK(t *testing.T, db *sql.DB, label, query string, args ...any) {
	t.Helper()

	if _, err := db.Exec("EXPLAIN "+query, args...); err != nil {
		t.Errorf("%s: %v\n  query: %s", label, err, query)
	}
}

// TestTableFiltersCompileAgainstSchema applies every filter clause of every
// admin table-browser table to that table's GetTableDefaults query and checks
// SQLite accepts it. A clause that references a column missing from (or
// ambiguous in) the joined query otherwise fails silently as an empty grid.
func TestTableFiltersCompileAgainstSchema(t *testing.T) {
	db := openMigratedSchema(t)

	for table, defs := range filterFieldDefs {
		td := database.GetTableDefaults(table)
		if td.Table == "" {
			t.Errorf("%s: no GetTableDefaults entry", table)
			continue
		}

		base := "select " + td.DefaultColumns + " from " + td.Table + " WHERE "

		for _, def := range defs {
			for _, cl := range def.Clauses {
				for _, v := range []string{"1", "0"} {
					cond, args, ok := cl.apply(v)
					if !ok {
						continue
					}

					explainOK(t, db, table+"/"+def.Field, base+cond+td.DefaultOrderBy, args...)
				}
			}
		}
	}
}

// TestTableDefaultQueriesCompileAgainstSchema checks each browsable table's
// free-text search query and generic filter-*_id catch-all against the
// schema.
func TestTableDefaultQueriesCompileAgainstSchema(t *testing.T) {
	db := openMigratedSchema(t)

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		t.Fatal(err)
	}

	var tables []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}

		tables = append(tables, name)
	}

	rows.Close()

	for _, table := range tables {
		td := database.GetTableDefaults(table)
		if td.Table == "" {
			continue
		}

		query := "select " + td.DefaultColumns + " from " + td.Table + td.DefaultOrderBy
		explainOK(t, db, table+"/default", query)

		cols := tableColumns(t, db, table)

		q := url.Values{}
		for col := range cols {
			q.Set("filter-"+col, "1")
		}

		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/?"+q.Encode(), nil)

		_, _, legacyHandled := applyDefinedFilterClauses(table, ctx)

		conds, idArgs := genericIDFilters(table, cols, legacyHandled, ctx)
		for i, cond := range conds {
			explainOK(t, db, table+"/generic-id", "select "+td.DefaultColumns+" from "+td.Table+" WHERE "+cond, idArgs[i])
		}

		if td.DefaultQuery == "" {
			continue
		}

		args := make([]any, td.DefaultQueryParamCount)
		for i := range args {
			args[i] = "%x%"
		}

		explainOK(t, db, table+"/search", "select "+td.DefaultColumns+" from "+td.Table+td.DefaultQuery, args...)
	}
}
