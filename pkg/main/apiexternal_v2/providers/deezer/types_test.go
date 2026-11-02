package deezer

import (
	"testing"
	"time"
)

func TestParseYear(t *testing.T) {
	year, at, ok := parseYear("2001-03-12")
	if !ok || year != 2001 || !at.Equal(time.Date(2001, 3, 12, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("full date = %d %v %t", year, at, ok)
	}

	if year, at, ok := parseYear("2001"); !ok || year != 2001 || !at.IsZero() {
		t.Errorf("year only = %d %v %t", year, at, ok)
	}

	for _, bad := range []string{"", "0000"[:3], "n/a-01-01"} {
		if _, _, ok := parseYear(bad); ok {
			t.Errorf("parseYear(%q) accepted", bad)
		}
	}
}
