package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestWatchlistItemIDs(t *testing.T) {
	item := WatchlistItem{
		GUID:  "plex://movie/5d776",
		GUIDS: []GUID{{ID: "tmdb://603"}, {ID: "tvdb://abc"}, {ID: "imdb://tt0133093"}, {ID: "tvdb://81189"}},
	}

	if got := item.IMDbID(); got != "tt0133093" {
		t.Errorf("IMDB = %q", got)
	}

	// An unparsable tvdb GUID is skipped in favour of the next one.
	if got := item.TVDbID(); got != 81189 {
		t.Errorf("TVDB = %d", got)
	}

	fallback := WatchlistItem{GUID: "imdb://tt0111161"}
	if got := fallback.IMDbID(); got != "tt0111161" {
		t.Errorf("IMDB fallback = %q", got)
	}

	tvdbFallback := WatchlistItem{GUID: "tvdb://121361"}
	if got := tvdbFallback.TVDbID(); got != 121361 {
		t.Errorf("TVDB fallback = %d", got)
	}

	var empty WatchlistItem
	if empty.IMDbID() != "" || empty.TVDbID() != 0 {
		t.Error("empty item returned an id")
	}

	if !(&WatchlistItem{Type: "movie"}).IsMovie() || !(&WatchlistItem{Type: "show"}).IsShow() || empty.IsMovie() {
		t.Error("type checks")
	}
}

func TestGetWatchlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlists/all" || r.URL.Query().Get("X-Plex-Token") != "tok" || r.URL.Query().Get("type") != "15" {
			http.Error(w, "bad request "+r.URL.String(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[{"type":"movie","title":"Heat","Guid":[{"id":"imdb://tt0113277"}]}]}}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{Timeout: 5 * time.Second, RateLimitCalls: 10, RateLimitSeconds: 1})

	items, err := p.GetWatchlist(context.Background(), srv.URL+"/", "tok")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 || items[0].IMDbID() != "tt0113277" || !items[0].IsMovie() {
		t.Fatalf("items = %+v", items)
	}

	if _, err := p.GetWatchlist(context.Background(), "", "tok"); err == nil {
		t.Error("missing server URL accepted")
	}
}
