package omdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestParseYear(t *testing.T) {
	tests := map[string]int{
		"2020":      2020,
		"2020-2023": 2020,
		"2011–2019": 2011,
		"2011–":     2011,
		"N/A":       0,
		"":          0,
	}

	for in, want := range tests {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestGetDetailsByIMDbMapsFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Title":"M","Year":"1999","Genre":"N/A","Language":"English, French",
"Plot":"N/A","Poster":"N/A","imdbRating":"N/A","imdbVotes":"1,234","Website":"https://example.org",
"imdbID":"tt0133093","Type":"movie","Response":"True"}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})

	d, err := p.GetDetailsByIMDb(context.Background(), "tt0133093")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.VoteCount != 1234 || d.Homepage != "https://example.org" {
		t.Errorf("VoteCount=%d Homepage=%q", d.VoteCount, d.Homepage)
	}

	if d.Overview != "" || d.PosterPath != "" || len(d.Genres) != 0 || d.VoteAverage != 0 {
		t.Errorf("N/A leaked: overview=%q poster=%q genres=%v rating=%v",
			d.Overview, d.PosterPath, d.Genres, d.VoteAverage)
	}

	if len(d.SpokenLanguages) != 2 {
		t.Errorf("languages = %v", d.SpokenLanguages)
	}
}

func TestSearchSeriesKeepsIMDbID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Search":[{"Title":"S","Year":"2011–2019","imdbID":"tt0944947","Type":"series","Poster":"N/A"}],"Response":"True"}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})

	res, err := p.SearchSeries(context.Background(), "s", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 1 || res[0].IMDbID != "tt0944947" || res[0].PosterPath != "" {
		t.Errorf("unexpected results %+v", res)
	}
}
