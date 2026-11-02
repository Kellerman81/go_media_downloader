package openlibrary

import (
	"testing"
	"time"

	"github.com/goccy/go-json"
)

func TestParseOLDate(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

	for in, want := range map[string]time.Time{
		"":               {},
		"1965-08-01":     d(1965, 8, 1),
		"August 1, 1965": d(1965, 8, 1),
		"August 1965":    d(1965, 8, 1),
		"1965":           d(1965, 1, 1),
		"Aug 1, 1965":    d(1965, 8, 1),
		"01 Aug 1965":    d(1965, 8, 1),
		"1965?":          d(1965, 1, 1),
		"c1965":          {},
	} {
		if got := parseOLDate(in); !got.Equal(want) {
			t.Errorf("parseOLDate(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestExtractOLID(t *testing.T) {
	for in, want := range map[string]string{"/works/OL45804W": "OL45804W", "OL45804W": "OL45804W", "": ""} {
		if got := extractOLID(in); got != want {
			t.Errorf("extractOLID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractTextValue(t *testing.T) {
	for _, tt := range []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"plain description", "plain description"},
		{map[string]any{"type": "/type/text", "value": "typed description"}, "typed description"},
		{map[string]any{"type": "/type/text"}, ""},
		{42, ""},
	} {
		if got := extractTextValue(tt.in); got != tt.want {
			t.Errorf("extractTextValue(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWorkAuthorTypeLoose(t *testing.T) {
	data := []byte(`{"key":"/works/OL1W","title":"T","authors":[
		{"author":{"key":"/authors/OL1A"},"type":{"key":"/type/author_role"}},
		{"author":{"key":"/authors/OL2A"},"type":"/type/author_role"},
		{"author":{"key":"/authors/OL3A"}}
	]}`)

	var w olWorkResponse
	if err := json.Unmarshal(data, &w); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(w.Authors) != 3 {
		t.Fatalf("authors = %d, want 3", len(w.Authors))
	}

	for i, want := range []string{"/type/author_role", "/type/author_role", ""} {
		if w.Authors[i].Type.Key != want {
			t.Errorf("author %d type = %q, want %q", i, w.Authors[i].Type.Key, want)
		}
	}

	if w.Authors[1].Author.Key != "/authors/OL2A" {
		t.Errorf("author key lost: %q", w.Authors[1].Author.Key)
	}

	if err := json.Unmarshal([]byte(`{"authors":[{"type":42}]}`), &w); err == nil {
		t.Error("non-string, non-object type accepted")
	}
}
