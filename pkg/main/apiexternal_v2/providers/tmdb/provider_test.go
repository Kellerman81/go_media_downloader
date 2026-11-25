package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestGetTMDBListMixedItemsAndStringID(t *testing.T) {
	tests := map[string]string{
		"string id": `"509ec17b19c2950a0600050d"`,
		"number id": `8204`,
	}

	for name, id := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id":` + id + `,"item_count":2,"page":1,"total_pages":1,"items":[
{"id":603,"media_type":"movie","title":"The Matrix","release_date":"1999-03-30"},
{"id":1396,"media_type":"tv","name":"Breaking Bad","first_air_date":"2008-01-20"}]}`))
			}))
			defer srv.Close()

			p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL + "/3", Timeout: 5 * time.Second}, "key")

			res, err := p.GetTMDBList(context.Background(), 1, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(res.Items) != 2 {
				t.Fatalf("got %d items", len(res.Items))
			}

			if res.Items[0].Title != "The Matrix" || res.Items[1].Name != "Breaking Bad" ||
				res.Items[1].FirstAirDate != "2008-01-20" || res.Items[1].MediaType != "tv" {
				t.Errorf("unexpected items %+v", res.Items)
			}
		})
	}
}

// TestWithLanguage: the language goes along with every request of the
// copy, also after other parameters, never twice, and not with the
// original provider.
func TestWithLanguage(t *testing.T) {
	var queries []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("language")+" "+r.URL.Query().Get("include_video_language"))
		w.Write([]byte(`{"id":1,"results":[]}`))
	}))
	t.Cleanup(srv.Close)

	p := NewProviderWithConfig(base.ClientConfig{Name: "tmdb", BaseURL: srv.URL + "/3"}, "key")
	de := p.WithLanguage("de")
	ctx := context.Background()

	de.GetMovieByID(ctx, 1)
	de.GetMovieVideos(ctx, 1, "de", "en")
	de.FindMovieByIMDbID(ctx, "tt1")
	p.GetMovieByID(ctx, 1)

	want := []string{"de ", "de de,en", "de ", " "}
	if len(queries) != len(want) {
		t.Fatalf("requests: %q", queries)
	}

	for i := range want {
		if queries[i] != want[i] {
			t.Errorf("request %d: %q, want %q", i, queries[i], want[i])
		}
	}
}

// TestWithFallbackLanguage: a 404 or an empty answer is asked again in the
// fallback language; a found one, English and an explicit language are not.
func TestWithFallbackLanguage(t *testing.T) {
	var requests []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("language")
		requests = append(requests, r.URL.Path+" "+lang)

		switch {
		case r.URL.Path == "/3/movie/1" && lang != "de":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"status_code":34}`))
		case r.URL.Path == "/3/movie/1":
			w.Write([]byte(`{"id":1,"title":"Terra X"}`))
		case r.URL.Path == "/3/find/tt1" && lang == "de":
			w.Write([]byte(`{"movie_results":[{"id":1,"title":"Terra X"}]}`))
		case r.URL.Path == "/3/find/tt1":
			w.Write([]byte(`{"movie_results":[],"tv_results":[]}`))
		default:
			w.Write([]byte(`{"id":2,"cast":[{"id":1,"name":"A"}],"results":[{"id":3}]}`))
		}
	}))
	t.Cleanup(srv.Close)

	p := NewProviderWithConfig(base.ClientConfig{Name: "tmdb", BaseURL: srv.URL + "/3"}, "key")
	de := p.WithFallbackLanguage("de")
	ctx := context.Background()

	if d, err := de.GetMovieByID(ctx, 1); err != nil || d.Title != "Terra X" {
		t.Errorf("404, then de: %+v, %v", d, err)
	}

	if f, err := de.FindMovieByIMDbID(ctx, "tt1"); err != nil || len(f.MovieResults) != 1 {
		t.Errorf("empty, then de: %+v, %v", f, err)
	}

	requests = nil

	if c, err := de.GetMovieCredits(ctx, 2); err != nil || len(c.Cast) != 1 || len(requests) != 1 {
		t.Errorf("found at once: %v, requests %q", err, requests)
	}

	for name, q := range map[string]*Provider{"english": p.WithFallbackLanguage("en"), "none": p, "explicit": de.WithLanguage("fr")} {
		requests = nil

		if _, err := q.GetMovieByID(ctx, 1); err == nil || len(requests) != 1 {
			t.Errorf("%s: %v, requests %q", name, err, requests)
		}
	}

	var nilProvider *Provider
	if nilProvider.WithFallbackLanguage("de") != nil || nilProvider.WithLanguage("de") != nil {
		t.Error("nil provider")
	}
}
