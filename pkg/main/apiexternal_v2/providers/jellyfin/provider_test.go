package jellyfin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestWatchlistItemIDs(t *testing.T) {
	item := WatchlistItem{ProviderIds: map[string]string{"Imdb": "tt0944947", "Tvdb": "121361", "Tmdb": "x"}}

	if got := item.IMDbID(); got != "tt0944947" {
		t.Errorf("IMDB = %q", got)
	}

	if got := item.TVDbID(); got != 121361 {
		t.Errorf("TVDB = %d", got)
	}

	if got := item.TMDbID(); got != 0 {
		t.Errorf("unparsable TMDB = %d", got)
	}

	var empty WatchlistItem
	if empty.IMDbID() != "" || empty.TVDbID() != 0 || empty.TMDbID() != 0 {
		t.Error("item without provider ids returned an id")
	}

	withOriginal := WatchlistItem{Name: "Dark", OriginalTitle: "Dark (DE)"}
	if got := withOriginal.Title(); got != "Dark (DE)" {
		t.Errorf("title = %q", got)
	}

	nameOnly := WatchlistItem{Name: "Dark"}
	if got := nameOnly.Title(); got != "Dark" {
		t.Errorf("title fallback = %q", got)
	}

	if !(&WatchlistItem{Type: "Movie"}).IsMovie() || !(&WatchlistItem{Type: "Series"}).IsSeries() {
		t.Error("type checks")
	}
}

func TestGetWatchlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/Users" && r.URL.Query().Get("api_key") == "key":
			_, _ = w.Write([]byte(`[{"Name":"Other","Id":"u1"},{"Name":"Kellers","Id":"u2"}]`))
		case r.URL.Path == "/Users/u2/Items" && r.URL.Query().Get("IsFavorite") == "true":
			_, _ = w.Write([]byte(`{"Items":[{"Name":"Dark","Type":"Series","ProviderIds":{"Tvdb":"334824"}}]}`))
		default:
			http.Error(w, "bad request "+r.URL.String(), http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{Timeout: 5 * time.Second, RateLimitCalls: 10, RateLimitSeconds: 1})

	items, err := p.GetWatchlist(context.Background(), srv.URL, "key", "kellers")
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 || items[0].TVDbID() != 334824 || !items[0].IsSeries() {
		t.Fatalf("items = %+v", items)
	}

	if _, err := p.GetWatchlist(context.Background(), srv.URL, "key", "nobody"); err == nil {
		t.Error("unknown user accepted")
	}
}
