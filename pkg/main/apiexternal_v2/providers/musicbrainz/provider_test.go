package musicbrainz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestClampLimit(t *testing.T) {
	for in, want := range map[int]int{0: 25, -1: 25, 50: 50, 100: 100, 500: 100} {
		if got := clampLimit(in); got != want {
			t.Errorf("clampLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestSearchReleasesRetriesBusyServer(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error": "The MusicBrainz web server is currently busy. Please try again later."}`))

			return
		}

		_, _ = w.Write([]byte(`{"count": 1, "releases": [{"id": "abc", "title": "Album"}]}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{
		BaseURL:          srv.URL,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
		RetryBackoff:     10 * time.Millisecond,
	})

	results, total, err := p.SearchReleases(context.Background(), "release:Album", 5, 0)
	if err != nil {
		t.Fatalf("SearchReleases after one 503: %v", err)
	}

	if total != 1 || len(results) != 1 || results[0].Title != "Album" || calls.Load() != 2 {
		t.Errorf("total=%d results=%+v calls=%d, want one result after 2 calls", total, results, calls.Load())
	}
}
