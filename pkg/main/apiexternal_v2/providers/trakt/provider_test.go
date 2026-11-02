package trakt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()

	old := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")

	t.Cleanup(func() { config.Configfile = old })

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return NewProviderWithConfig(base.ClientConfig{
		BaseURL:        srv.URL,
		Timeout:        5 * time.Second,
		RateLimitCalls: 100, RateLimitSeconds: 1,
	}, "client-id", "secret", "")
}

func TestPublicEndpointWithoutToken(t *testing.T) {
	var gotKey, gotAuth string

	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotAuth = r.Header.Get("trakt-api-key"), r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[{"type":"show","show":{"title":"S","ids":{"trakt":1,"tvdb":42,"imdb":"tt1"}}}]`))
	})

	res, err := p.SearchSeries(context.Background(), "s", 0)
	if err != nil {
		t.Fatalf("public search without token failed: %v", err)
	}

	if gotKey != "client-id" || gotAuth != "" {
		t.Errorf("headers api-key=%q auth=%q", gotKey, gotAuth)
	}

	if len(res) != 1 || res[0].TVDbID != 42 || res[0].IMDbID != "tt1" {
		t.Errorf("unexpected results %+v", res)
	}
}

func TestAuthEndpointWithoutToken(t *testing.T) {
	called := false

	p := newTestProvider(t, func(http.ResponseWriter, *http.Request) { called = true })

	_, err := p.GetWatchlist(context.Background(), "me", "movies")
	if err == nil || !strings.Contains(err.Error(), "requires OAuth") {
		t.Fatalf("err = %v, want OAuth error", err)
	}

	if called {
		t.Error("request was sent without a token")
	}
}

func TestAuthEndpointSendsToken(t *testing.T) {
	var gotAuth string

	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	})

	_ = p.SetToken(&apiexternal_v2.OAuthToken{
		AccessToken: "tok",
		Expiry:      time.Now().Add(24 * time.Hour),
	})

	if _, err := p.GetWatchlist(context.Background(), "me", "movies"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestPopularAcceptsBareAndWrappedItems(t *testing.T) {
	tests := map[string]string{
		"bare":    `[{"title":"Show A","year":2020,"ids":{"trakt":7,"tvdb":70,"imdb":"tt7"}}]`,
		"wrapped": `[{"watchers":5,"show":{"title":"Show A","year":2020,"ids":{"trakt":7,"tvdb":70,"imdb":"tt7"}}}]`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			})

			res, err := p.GetPopularSeries(context.Background(), 1, 10, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(res.Results) != 1 || res.Results[0].ID != 7 || res.Results[0].TVDbID != 70 ||
				res.Results[0].Name != "Show A" {
				t.Errorf("unexpected results %+v", res.Results)
			}

			movies, err := p.GetPopularMovies(context.Background(), 1, 10, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if name == "bare" && (len(movies.Results) != 1 || movies.Results[0].ID != 7) {
				t.Errorf("unexpected movie results %+v", movies.Results)
			}
		})
	}
}
