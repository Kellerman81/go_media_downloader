package csrfapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeImdbID(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"tt1234567", "tt1234567", true},
		{"  tt0123456789 ", "", false},
		{"tt012345678", "tt012345678", true},
		{"TT1234567", "tt1234567", true},
		{"1234567", "tt1234567", true},
		{"/title/tt1234567/", "tt1234567", true},
		{"https://www.imdb.com/title/tt7654321/?ref_=nv_sr_1", "tt7654321", true},
		{"https://m.imdb.com/de/title/tt7654321#cast", "tt7654321", true},
		{"https://example.com/movie/1234567", "", false},
		{"tt123", "", false},
		{"tt12345a7", "", false},
		{"nm1234567", "", false},
		{"", "", false},
		{"/", "", false},
	}

	for _, tt := range tests {
		got, ok := normalizeImdbID(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("normalizeImdbID(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestAppendMovieImdbIDNormalizes(t *testing.T) {
	s := &MovieScraper{}
	seen := map[string]bool{}

	var ids []string
	for _, raw := range []string{"/title/tt1234567/", "tt1234567", "1234567", "garbage"} {
		ids = s.appendMovieImdbID(ids, seen, &MovieData{ImdbID: raw})
	}

	if len(ids) != 1 || ids[0] != "tt1234567" {
		t.Fatalf("ids = %v, want [tt1234567]", ids)
	}
}

func TestMovieScrapeErrors(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.SetCookie(w, &http.Cookie{Name: "_csrf", Value: "tok", Path: "/"})
		case "/api":
			switch r.URL.Query().Get("page") {
			case "1":
				calls.Add(1)
				// Duplicate id must not be emitted twice (and must not trigger a TMDB search).
				_, _ = w.Write([]byte(`{"movies":[{"title":"A","imdb":"tt1234567"},{"title":"A again","imdb":"tt1234567","year":2001}]}`))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}))
	defer srv.Close()

	newScraper := func(start int) *MovieScraper {
		s, err := NewMovieScraper(&MovieConfig{
			SiteName:         "t",
			StartURL:         srv.URL + "/start",
			APIURLPattern:    srv.URL + "/api?page={page}",
			PageStartIndex:   start,
			ResultsArrayPath: "movies",
			TitleField:       "title",
			ImdbIDField:      "imdb",
			YearField:        "year",
			WaitSeconds:      -1,
		})
		if err != nil {
			t.Fatal(err)
		}

		return s
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Page 2 fails: partial result plus error.
	ids, err := newScraper(1).Scrape(ctx, 3)
	if err == nil {
		t.Error("expected error for failing page 2")
	}

	if len(ids) != 1 || ids[0] != "tt1234567" {
		t.Errorf("expected partial [tt1234567], got %v", ids)
	}

	// First page fails: no ids, error.
	ids, err = newScraper(5).Scrape(ctx, 3)
	if err == nil || ids != nil {
		t.Errorf("expected (nil, error) when page 1 fails, got (%v, %v)", ids, err)
	}
}

// TestMovieScrapeCancelDuringPageWait: the inter-page wait (sitethrottle) must
// end as soon as ctx is cancelled instead of sleeping out WaitSeconds.
func TestMovieScrapeCancelDuringPageWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.SetCookie(w, &http.Cookie{Name: "_csrf", Value: "tok", Path: "/"})
		case "/api":
			_, _ = w.Write([]byte(`{"movies":[{"title":"A","imdb":"tt7654321"}]}`))
		}
	}))
	defer srv.Close()

	s, err := NewMovieScraper(&MovieConfig{
		SiteName:         "t",
		StartURL:         srv.URL + "/start",
		APIURLPattern:    srv.URL + "/api?page={page}",
		ResultsArrayPath: "movies",
		TitleField:       "title",
		ImdbIDField:      "imdb",
		WaitSeconds:      30,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	ids, err := s.Scrape(ctx, 3)

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Scrape took %v after cancellation", elapsed)
	}

	if err == nil {
		t.Error("expected a context error")
	}

	if len(ids) != 1 || ids[0] != "tt7654321" {
		t.Errorf("expected first-page ids [tt7654321], got %v", ids)
	}
}
