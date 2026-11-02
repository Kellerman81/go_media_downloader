package tmdb

import (
	"testing"
	"time"
)

func TestParseDateAndYear(t *testing.T) {
	for in, want := range map[string]time.Time{
		"":                     {},
		"2008-07-16":           time.Date(2008, 7, 16, 0, 0, 0, 0, time.UTC),
		"2008-07-16T10:00:00Z": time.Date(2008, 7, 16, 10, 0, 0, 0, time.UTC),
		"July 2008":            {},
	} {
		if got := parseDate(in); !got.Equal(want) {
			t.Errorf("parseDate(%q) = %v, want %v", in, got, want)
		}
	}

	for in, want := range map[string]int{"": 0, "2008-07-16": 2008, "2008": 2008, "08": 0, "TBA-01-01": 0} {
		if got := extractYear(in); got != want {
			t.Errorf("extractYear(%q) = %d, want %d", in, got, want)
		}
	}
}
