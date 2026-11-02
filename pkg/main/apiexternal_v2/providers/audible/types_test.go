package audible

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConvertProductChaptersFillsBothFieldSets(t *testing.T) {
	p := &audibleProduct{ASIN: "B1", Chapters: audibleChapterInfo{Chapters: []audibleChapter{
		{Title: "One", StartOffsetMs: 0, LengthMs: 1000},
		{Title: "Two", StartOffsetMs: 1000, LengthMs: 2500},
	}}}

	ch := convertProductToDetails(p).Chapters
	if len(ch) != 2 {
		t.Fatalf("chapters = %d", len(ch))
	}

	c := ch[1]
	if c.Number != 2 || c.ChapterNumber != 2 || c.StartOffsetMs != 1000 || c.LengthMs != 2500 ||
		c.StartOffset != time.Second || c.Duration != 2500*time.Millisecond {
		t.Errorf("chapter = %+v", c)
	}
}

func TestAudibleRatingAndCategoriesDecode(t *testing.T) {
	raw := `{"product":{"asin":"B1","rating":{"num_reviews":2460,"overall_distribution":{"average_rating":4.44,"display_average_rating":"4.4","display_stars":4.5,"num_ratings":12450}},
	"category_ladders":[{"ladder":[{"id":"1","name":"Fiction"},{"id":"2","name":"Thriller"}],"root":"Genres"},{"ladder":[{"id":"1","name":"Fiction"},{"id":"3","name":"Suspense"}],"root":"Genres"}]}}`

	var resp audibleProductResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	d := convertProductToDetails(&resp.Product)
	if d.Rating != 4.44 || d.RatingsCount != 12450 {
		t.Errorf("rating = %v / %d", d.Rating, d.RatingsCount)
	}

	if strings.Join(d.Genres, "|") != "Fiction|Thriller|Suspense" {
		t.Errorf("genres = %q", d.Genres)
	}
}

func TestParseAudibleDate(t *testing.T) {
	day := time.Date(2017, 11, 2, 0, 0, 0, 0, time.UTC)

	for in, want := range map[string]time.Time{
		"2017-11-02":       day,
		"11-02-2017":       day,
		"November 2, 2017": day,
		"Nov 2, 2017":      day,
		"2017":             time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
		"2017-11-02T00:00": time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
		"1850":             time.Date(1850, 1, 1, 0, 0, 0, 0, time.UTC), // exact layout wins over the range check
		"18500":            {},
		"":                 {},
	} {
		if got := parseAudibleDate(in); !got.Equal(want) {
			t.Errorf("parseAudibleDate(%q) = %v, want %v", in, got, want)
		}
	}
}
