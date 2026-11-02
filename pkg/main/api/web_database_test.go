package api

import (
	"testing"
)

func TestConvertValueForColumn(t *testing.T) {
	types := map[string]string{
		"runtime":     "INTEGER",
		"rating":      "REAL",
		"first_aired": "DATETIME",
		"title":       "TEXT",
		"missing":     "BOOLEAN",
	}

	tests := []struct {
		col  string
		in   any
		want any
	}{
		{"runtime", "42", 42},
		{"runtime", "", 0},
		{"rating", "7.5", 7.5},
		{"first_aired", "", nil},
		{"first_aired", "2024-01-02", "2024-01-02"},
		{"title", "", ""},
		{"missing", "on", 1},
		{"missing", "0", 0},
	}

	for _, tt := range tests {
		if got := convertValueForColumn(tt.in, tt.col, types); got != tt.want {
			t.Errorf("convertValueForColumn(%q, %q) = %#v, want %#v", tt.in, tt.col, got, tt.want)
		}
	}
}

func TestUnknownAdminColumn(t *testing.T) {
	types := map[string]string{"id": "INTEGER", "title": "TEXT"}

	if col := unknownAdminColumn(map[string]any{"Title": "x", "csrf_token": "t"}, types); col != "" {
		t.Errorf("known columns reported unknown: %q", col)
	}

	if col := unknownAdminColumn(map[string]any{"title = 1; drop table x; --": "x"}, types); col == "" {
		t.Error("injected column name was accepted")
	}
}
