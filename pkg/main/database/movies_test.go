package database

import (
	"testing"
)

func TestChecknzbtitleB(t *testing.T) {
	tests := []struct {
		title, nzb string
		year       uint16
		pm1        bool
		want       bool
	}{
		{"The Matrix", "the matrix", 0, false, true},
		{"The Matrix", "The.Matrix.1999.1080p", 1999, false, true}, // dots: matched via the slug
		{"The Matrix", "The Matrix 1999 1080p", 1999, false, true},
		{"The Matrix", "The Matrix 2000 1080p", 1999, false, false},
		{"The Matrix", "The Matrix 2000 1080p", 1999, true, true},
		{"Amélie", "Amelie.2001.720p", 2001, false, true},
		{"Alien", "Aliens 1986", 1979, true, false},
		{"", "x", 0, false, false},
		{"x", "", 0, false, false},
	}

	for _, tt := range tests {
		if got := ChecknzbtitleB(tt.title, "", tt.nzb, tt.pm1, tt.year); got != tt.want {
			t.Errorf("ChecknzbtitleB(%q, %q, %d, %t) = %t, want %t", tt.title, tt.nzb, tt.year, tt.pm1, got, tt.want)
		}
	}
}
