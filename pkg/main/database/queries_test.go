package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestValidateImdbDBFile(t *testing.T) {
	dir := t.TempDir()

	mk := func(name string, stmts ...string) string {
		p := filepath.Join(dir, name)
		db, err := sqlx.Connect("sqlite", "file:"+p)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range stmts {
			if _, err := db.Exec(s); err != nil {
				t.Fatal(err)
			}
		}
		db.Close()
		return p
	}

	empty := filepath.Join(dir, "empty.db")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	garbage := filepath.Join(dir, "garbage.db")
	if err := os.WriteFile(garbage, []byte("not a sqlite database at all, just text padding"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"missing", filepath.Join(dir, "nope.db"), true},
		{"empty file", empty, true},
		{"garbage", garbage, true},
		{"no table", mk("notable.db", "create table x (id integer)"), true},
		{"empty table", mk("emptytable.db", "create table imdb_titles (tconst text)"), true},
		{"ok", mk("ok.db", "create table imdb_titles (tconst text)", "insert into imdb_titles values ('tt1')"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImdbDBFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateImdbDBFile() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueryHelpersRoundTrip(t *testing.T) {
	db := useMigratedMainDB(t)

	if _, err := InsertArray("dbartists", []string{"name", "slug"}, "Only One Value"); !errors.Is(err, errWrongNumberOfColumns) {
		t.Fatalf("column/value mismatch: err = %v", err)
	}

	if _, err := InsertArray("dbartists", []string{"name", "slug"}, "Pink Floyd", "pink-floyd"); err != nil {
		t.Fatal(err)
	}

	id, err := ExecNid("insert into dbartists (name, slug) values (?, ?)", "Queen", "queen")
	if err != nil || id != 2 {
		t.Fatalf("ExecNid = %d, %v", id, err)
	}

	res, err := UpdateArray("dbartists", []string{"name", "slug"}, "id = ?", "Queen + Adam Lambert", "queen-adam-lambert", id)
	if err != nil {
		t.Fatal(err)
	}

	if n, _ := res.RowsAffected(); n != 1 {
		t.Errorf("UpdateArray affected %d rows", n)
	}

	type artist struct {
		ID   uint   `db:"id"`
		Name string `db:"name"`
	}

	rows := StructscanT[artist](false, 0, "select id, name from dbartists order by id")
	if len(rows) != 2 || rows[0].Name != "Pink Floyd" || rows[1].Name != "Queen + Adam Lambert" {
		t.Errorf("StructscanT = %+v", rows)
	}

	one, err := Structscan[artist]("select id, name from dbartists where id = ?", false, id)
	if err != nil || one.Name != "Queen + Adam Lambert" {
		t.Errorf("Structscan = %+v, %v", one, err)
	}

	if _, err := Structscan[artist]("select id, name from dbartists where id = ?", false, 99); err == nil {
		t.Error("Structscan of a missing row returned no error")
	}

	if got := Getdatarow[string](false, "select slug from dbartists where id = ?", &id); got != "queen-adam-lambert" {
		t.Errorf("Getdatarow = %q", got)
	}

	if _, err := DeleteRow("dbartists", "id = ?", id); err != nil {
		t.Fatal(err)
	}

	var left int
	if err := db.Get(&left, "select count(*) from dbartists"); err != nil || left != 1 {
		t.Errorf("rows after DeleteRow = %d, %v", left, err)
	}

	if err := ExecNErr("insert into no_such_table values (1)"); err == nil {
		t.Error("ExecNErr on invalid SQL returned no error")
	}

	if _, err := ExecNid("insert into no_such_table values (1)"); err == nil {
		t.Error("ExecNid on invalid SQL returned no error")
	}
}

func TestCloseInterruptedJobHistories(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into job_histories (id, job_type, started) values (1, 'open-old', '2026-01-01 10:00:00')")
	mustExec(t, db, "insert into job_histories (id, job_type, started, ended) values (2, 'done', '2026-01-01 10:00:00', '2026-01-01 10:05:00')")
	mustExec(t, db, "insert into job_histories (id, job_type, started) values (3, 'open-future', '2999-01-01 10:00:00')")
	mustExec(t, db, "insert into job_histories (id, job_type) values (4, 'no-start')")

	closed, err := CloseInterruptedJobHistories()
	if err != nil {
		t.Fatal(err)
	}

	if closed != 1 {
		t.Fatalf("closed %d rows, want 1", closed)
	}

	var ended []struct {
		ID    int     `db:"id"`
		Ended *string `db:"ended"`
	}
	if err := db.Select(&ended, "select id, strftime('%Y-%m-%d %H:%M:%S', ended) as ended from job_histories order by id"); err != nil {
		t.Fatal(err)
	}

	want := map[int]*string{1: strp("2026-01-01 10:00:00"), 2: strp("2026-01-01 10:05:00"), 3: nil, 4: nil}
	for _, row := range ended {
		w := want[row.ID]
		if (w == nil) != (row.Ended == nil) || (w != nil && *w != *row.Ended) {
			t.Errorf("row %d: ended = %v, want %v", row.ID, deref(row.Ended), deref(w))
		}
	}

	// Duration as computed by the table browser must be 0 for a closed row.
	var dur float64
	if err := db.Get(&dur, "select ROUND((julianday(ended) - julianday(started)) * 86400) from job_histories where id = 1"); err != nil {
		t.Fatal(err)
	}

	if dur != 0 {
		t.Errorf("duration = %v, want 0", dur)
	}
}

func strp(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}

	return *s
}
