package itunes

import (
	"testing"
	"time"
)

func TestParseReleaseDate(t *testing.T) {
	year, at, ok := parseReleaseDate("1997-05-21T07:00:00Z")
	if !ok || year != 1997 || !at.Equal(time.Date(1997, 5, 21, 7, 0, 0, 0, time.UTC)) {
		t.Errorf("full date = %d %v %t", year, at, ok)
	}

	if year, at, ok := parseReleaseDate("1997"); !ok || year != 1997 || !at.IsZero() {
		t.Errorf("year only = %d %v %t", year, at, ok)
	}

	for _, bad := range []string{"", "97", "abcd-01-01"} {
		if _, _, ok := parseReleaseDate(bad); ok {
			t.Errorf("parseReleaseDate(%q) accepted", bad)
		}
	}
}

func TestArtworkURL(t *testing.T) {
	for in, want := range map[string]string{
		"": "",
		"https://is1-ssl.mzstatic.com/image/thumb/x/100x100bb.jpg": "https://is1-ssl.mzstatic.com/image/thumb/x/600x600bb.jpg",
		"https://example.com/cover.jpg":                            "https://example.com/cover.jpg",
	} {
		if got := artworkURL(in); got != want {
			t.Errorf("artworkURL(%q) = %q, want %q", in, got, want)
		}
	}
}
