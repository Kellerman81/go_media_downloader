package api

import (
	"testing"
)

func TestTableJSONOrderBy(t *testing.T) {
	cols := []string{"id", "dbmovies.title as title", "year"}

	tests := []struct {
		col, dir, def, want string
	}{
		{"", "asc", " order by id desc", "order by id desc"},
		{"", "asc", "", "order by id asc"},
		{"1", "desc", " order by id desc", "order by title desc"},
		{"2", "DESC", "", "order by year desc"},
		{"2", "asc; drop table movies", "", "order by year asc"},
		{"9", "asc", "", "order by id asc"},
		{"-1", "desc", "", "order by id desc"},
	}

	for _, tt := range tests {
		if got := tableJSONOrderBy(cols, tt.col, tt.dir, tt.def); got != tt.want {
			t.Errorf("tableJSONOrderBy(%q,%q,%q) = %q, want %q", tt.col, tt.dir, tt.def, got, tt.want)
		}
	}
}

func TestTableJSONPaging(t *testing.T) {
	tests := []struct {
		length, start       string
		wantSize, wantStart int
	}{
		{"10", "0", 10, 0},
		{"-1", "20", tableJSONMaxPageSize, 20},
		{"999999", "0", tableJSONMaxPageSize, 0},
		{"abc", "-5", tableJSONMaxPageSize, 0},
	}

	for _, tt := range tests {
		size, start := tableJSONPaging(tt.length, tt.start)
		if size != tt.wantSize || start != tt.wantStart {
			t.Errorf("tableJSONPaging(%q,%q) = %d,%d want %d,%d", tt.length, tt.start, size, start, tt.wantSize, tt.wantStart)
		}
	}
}

func TestTableJSONWhere(t *testing.T) {
	if got := tableJSONWhere(nil); got != "" {
		t.Errorf("empty = %q", got)
	}

	got := tableJSONWhere([]string{"a like ? or b like ?", "c = ?"})
	if want := "WHERE (a like ? or b like ?) AND (c = ?)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
