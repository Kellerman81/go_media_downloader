package tmdb

import (
	"encoding/json"
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

// TestConvertMovieDetailsCollection: the movie details bring the collection
// along (saved by the metadata import).
func TestConvertMovieDetailsCollection(t *testing.T) {
	var in tmdbMovieDetails
	if err := json.Unmarshal([]byte(`{"id":584,"title":"2 Fast 2 Furious","belongs_to_collection":{"id":9485,"name":"The Fast and the Furious Collection","poster_path":"/x.jpg"}}`), &in); err != nil {
		t.Fatal(err)
	}

	if d := convertMovieDetails(&in); d.Collection == nil || d.Collection.ID != 9485 || d.Collection.Name != "The Fast and the Furious Collection" {
		t.Fatalf("collection: %+v", d.Collection)
	}

	var full tmdbMovieDetails
	if err := json.Unmarshal([]byte(`{"id":584,"credits":{"cast":[{"id":8167,"name":"Paul Walker","character":"Brian O'Conner"}]},`+
		`"recommendations":{"results":[{"id":9799,"title":"The Fast and the Furious"}]},"similar":{"results":[{"id":1,"title":"A"},{"id":2,"title":"B"}]}}`), &full); err != nil {
		t.Fatal(err)
	}

	if d := convertMovieDetails(&full); d.Credits == nil || d.Credits.Cast[0].Name != "Paul Walker" || len(d.Recommendations) != 1 ||
		d.Recommendations[0].ID != 9799 || len(d.Similar) != 2 {
		t.Fatalf("appended: %+v %+v %+v", d.Credits, d.Recommendations, d.Similar)
	}

	if d := convertMovieDetails(&tmdbMovieDetails{ID: 1}); d.Collection != nil {
		t.Fatalf("no collection: %+v", d.Collection)
	}
}
