package tvmaze

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestRequestsAndIDs(t *testing.T) {
	var gotURI string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()

		switch r.URL.Path {
		case "/search/shows":
			_, _ = w.Write([]byte(`[{"score":1,"show":{"id":1,"name":"S","externals":{"thetvdb":81189,"imdb":"tt0903747"}}}]`))
		default:
			_, _ = w.Write([]byte(`{"id":1,"name":"S"}`))
		}
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second})

	res, err := p.SearchSeries(context.Background(), "s", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 1 || res[0].TVDbID != 81189 || res[0].IMDbID != "tt0903747" {
		t.Errorf("unexpected results %+v", res)
	}

	if _, err := p.GetSeriesByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotURI != "/shows/1" {
		t.Errorf("GetSeriesByID requested %q, want /shows/1", gotURI)
	}
}
