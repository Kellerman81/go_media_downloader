package metadata

import (
	"testing"
)

func TestOmdbValue(t *testing.T) {
	tests := map[string]string{
		"":         "",
		"N/A":      "",
		" N/A ":    "",
		"0":        "0",
		"7.4":      "7.4",
		"120 min":  "120 min",
		"1,234":    "1,234",
		"Drama":    "Drama",
		"2010":     "2010",
		"10 min":   "10 min",
		"0.0 rate": "0.0 rate",
	}

	for in, want := range tests {
		if got := omdbValue(in); got != want {
			t.Errorf("omdbValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShouldUpdateRuntime(t *testing.T) {
	tests := []struct {
		current, next int
		overwrite     bool
		want          bool
	}{
		{0, 0, true, false},
		{0, 3, false, false}, // placeholder runtimes are never accepted
		{0, 95, false, true},
		{90, 95, false, false},
		{90, 95, true, true},
		{2, 95, false, true}, // a placeholder current value is replaced
		{90, 1, true, false},
	}

	for _, tt := range tests {
		if got := shouldUpdateRuntime(tt.current, tt.next, tt.overwrite); got != tt.want {
			t.Errorf("shouldUpdateRuntime(%d, %d, %t) = %t, want %t", tt.current, tt.next, tt.overwrite, got, tt.want)
		}
	}
}

func TestShouldUpdateSerieRuntime(t *testing.T) {
	tests := []struct {
		current   string
		next      int
		overwrite bool
		want      bool
	}{
		{"", 0, true, false},
		{"", 45, false, true},
		{"0", 45, false, true},
		{"3", 45, false, true},
		{"42", 45, false, false},
		{"42", 45, true, true},
		{"", 2, false, false},
		{"42", 2, true, false},
	}

	for _, tt := range tests {
		if got := shouldUpdateSerieRuntime(tt.current, tt.next, tt.overwrite); got != tt.want {
			t.Errorf("shouldUpdateSerieRuntime(%q, %d, %t) = %t, want %t", tt.current, tt.next, tt.overwrite, got, tt.want)
		}
	}
}

func TestBuildCommaSeparated(t *testing.T) {
	type genre struct{ Name string }

	name := func(g genre) string { return g.Name }

	if got := buildCommaSeparated(nil, name); got != "" {
		t.Errorf("empty = %q", got)
	}

	if got := buildCommaSeparated([]genre{{"Drama"}}, name); got != "Drama" {
		t.Errorf("single = %q", got)
	}

	if got := buildCommaSeparated([]genre{{"Drama"}, {"Crime"}, {"Thriller"}}, name); got != "Drama,Crime,Thriller" {
		t.Errorf("multiple = %q", got)
	}
}
