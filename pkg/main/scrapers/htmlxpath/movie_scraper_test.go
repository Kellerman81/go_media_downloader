package htmlxpath

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMovieScrapeErrorsAndNoPattern(t *testing.T) {
	movies := `<html><body>
<div class="m"><span class="t">A</span><a class="imdb" href="https://www.imdb.com/title/tt1234567/">x</a></div>
<div class="m"><span class="t">A dup</span><a class="imdb" href="https://www.imdb.com/title/tt1234567/">x</a><span class="y">2001</span></div>
</body></html>`

	var startHits int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			startHits++

			_, _ = w.Write([]byte(movies))
		case "/page/1":
			_, _ = w.Write([]byte(movies))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	newScraper := func(pattern string) *MovieScraper {
		s, err := NewMovieScraper(&MovieConfig{
			SiteName:       "t",
			StartURL:       srv.URL + "/start",
			SceneNodeXPath: `//div[@class="m"]`,
			TitleXPath:     `.//span[@class="t"]`,
			ImdbIDXPath:    `.//a[@class="imdb"]/@href`,
			YearXPath:      `.//span[@class="y"]`,
			PageURLPattern: pattern,
			WaitSeconds:    -1,
		})
		if err != nil {
			t.Fatal(err)
		}

		return s
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// No pattern: exactly one request to StartURL, duplicate id emitted once.
	ids, err := newScraper("").Scrape(ctx, 5)
	if err != nil || len(ids) != 1 || startHits != 1 {
		t.Errorf("no-pattern scrape: ids=%v err=%v startHits=%d", ids, err, startHits)
	}

	// Page 2 fails: partial ids plus error.
	ids, err = newScraper(srv.URL+"/page/{page}").Scrape(ctx, 3)
	if err == nil || len(ids) != 1 {
		t.Errorf("expected partial ids plus error, got ids=%v err=%v", ids, err)
	}

	// Page 1 fails: no ids, error.
	ids, err = newScraper(srv.URL+"/missing/{page}").Scrape(ctx, 3)
	if err == nil || ids != nil {
		t.Errorf("expected (nil, error), got ids=%v err=%v", ids, err)
	}
}
