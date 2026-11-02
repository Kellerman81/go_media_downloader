package api

import (
	"testing"
)

func TestFeedTypeValue(t *testing.T) {
	tests := map[string]string{
		"IMDB CSV - Import from IMDB CSV list": "imdbcsv",
		"imdbcsv":                              "imdbcsv",
		"Music Charts":                         "musiccharts",
		"unknown":                              "unknown",
	}

	for in, want := range tests {
		if got := feedTypeValue(in); got != want {
			t.Errorf("feedTypeValue(%q) = %q, want %q", in, got, want)
		}
	}
}
