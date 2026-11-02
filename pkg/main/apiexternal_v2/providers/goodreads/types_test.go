package goodreads

import (
	"testing"
	"time"
)

func TestParseGRDate(t *testing.T) {
	for in, want := range map[string]time.Time{
		"2011/03/01":    time.Date(2011, 3, 1, 0, 0, 0, 0, time.UTC),
		"2011-03-01":    time.Date(2011, 3, 1, 0, 0, 0, 0, time.UTC),
		"March 1, 2011": time.Date(2011, 3, 1, 0, 0, 0, 0, time.UTC),
		"March 2011":    time.Date(2011, 3, 1, 0, 0, 0, 0, time.UTC),
		"2011":          time.Date(2011, 1, 1, 0, 0, 0, 0, time.UTC),
		"":              {},
		"soon":          {},
	} {
		if got := parseGRDate(in); !got.Equal(want) {
			t.Errorf("parseGRDate(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestStripHTMLTags(t *testing.T) {
	for in, want := range map[string]string{
		"<p>A <b>great</b> book.</p>": "A great book.",
		"  plain  ":                   "plain",
		"1 < 2":                       "1 < 2",
		"<br/>":                       "",
	} {
		if got := stripHTMLTags(in); got != want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", in, got, want)
		}
	}
}
