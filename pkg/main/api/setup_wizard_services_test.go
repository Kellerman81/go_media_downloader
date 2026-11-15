package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
)

func TestWizardHTTPCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/denied":
			w.WriteHeader(http.StatusUnauthorized)
		case "/bad":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":3}}`))
		default:
			if r.Header.Get("X-Key") != "k" {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			_, _ = w.Write([]byte(`{"Response":"True"}`))
		}
	}))
	defer srv.Close()

	ctx := context.Background()

	if ok, msg := wizardHTTPCheck(ctx, srv.URL+"/ok", map[string]string{"X-Key": "k"}, nil); !ok {
		t.Errorf("ok: %s", msg)
	}

	if ok, msg := wizardHTTPCheck(ctx, srv.URL+"/ok", nil, nil); ok || !strings.Contains(msg, "wrong key") {
		t.Errorf("missing header: %v %s", ok, msg)
	}

	if ok, msg := wizardHTTPCheck(ctx, srv.URL+"/denied", nil, nil); ok || !strings.Contains(msg, "401") {
		t.Errorf("401: %v %s", ok, msg)
	}

	// A 400 the service uses for "bad request, key fine" passes when allowed.
	if ok, _ := wizardHTTPCheck(ctx, srv.URL+"/bad", nil, nil); ok {
		t.Error("400 passed")
	}

	if ok, msg := wizardHTTPCheck(ctx, srv.URL+"/bad", nil, func([]byte) (bool, string) { return true, "" }, http.StatusBadRequest); !ok {
		t.Errorf("allowed 400: %s", msg)
	}

	if ok, msg := wizardHTTPCheck(ctx, srv.URL+"/ok", map[string]string{"X-Key": "k"},
		func(b []byte) (bool, string) { return strings.Contains(string(b), "False"), "no" }); ok || msg != "no" {
		t.Errorf("body check: %v %s", ok, msg)
	}
}

func TestWizardServicesFor(t *testing.T) {
	keys := func(kinds map[string]bool) string {
		var out []string
		for _, s := range wizardServicesFor(kinds) {
			out = append(out, s.key)
		}

		return strings.Join(out, ",")
	}

	if got := keys(map[string]bool{"music": true}); strings.Contains(got, "tmdb") || !strings.Contains(got, "lastfm") || !strings.Contains(got, "plexurl") {
		t.Errorf("music: %s", got)
	}

	if got := keys(map[string]bool{"book": true}); got != "goodreads" {
		t.Errorf("books: %s", got)
	}

	if got := keys(map[string]bool{"video": true}); !strings.Contains(got, "tmdb") || strings.Contains(got, "lastfm") {
		t.Errorf("video: %s", got)
	}
}

func TestWizardSaveServicesStep(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	save := func(form url.Values) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		return wizardSaveServicesStep(c)
	}

	msg := save(url.Values{"wizard_svc_webapikey": {"n3w-k3y"}, "wizard_svc_tmdb": {" abc "}, "wizard_svc_lastfm": {"lf"},
		"wizard_svc_plexurl": {"http://plex:32400/"}})
	g := config.GetSettingsGeneral()

	if g.WebAPIKey != "n3w-k3y" || g.TheMovieDBApiKey != "abc" || g.LastFMAPIKey != "lf" || g.PlexServerURL != "http://plex:32400" {
		t.Fatalf("saved: %+v", g)
	}

	if !strings.Contains(msg, "web password") || !strings.Contains(msg, "Restart") {
		t.Errorf("message: %s", msg)
	}

	// An empty web key keeps the old one; a field not sent stays.
	save(url.Values{"wizard_svc_webapikey": {""}, "wizard_svc_omdb": {"o"}})

	if g := config.GetSettingsGeneral(); g.WebAPIKey != "n3w-k3y" || g.OmdbAPIKey != "o" || g.TheMovieDBApiKey != "abc" {
		t.Fatalf("second save: key %q omdb %q tmdb %q", g.WebAPIKey, g.OmdbAPIKey, g.TheMovieDBApiKey)
	}
}

func TestWizardSaveServicesLimitsAndImdb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	form := url.Values{
		"wizard_limit_tmdb_calls": {"3"}, "wizard_limit_tmdb_seconds": {"2"}, "wizard_limit_tmdb_timeout": {"9999"},
		"wizard_limit_omdb_calls": {"0"}, "wizard_limit_omdb_seconds": {"x"}, "wizard_limit_omdb_timeout": {"20"},
		"wizard_imdb_types": {"movie, tvSeries,,"}, "wizard_imdb_regions": {"DE, \\N"}, "wizard_imdb_memory": {"1"},
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if msg := wizardSaveServicesStep(c); !strings.Contains(msg, "saved") {
		t.Fatalf("save: %s", msg)
	}

	g := config.GetSettingsGeneral()
	if g.TmdbLimiterCalls != 3 || g.TmdbLimiterSeconds != 2 || g.TmdbTimeoutSeconds != 600 {
		t.Errorf("tmdb: %d/%d/%d", g.TmdbLimiterCalls, g.TmdbLimiterSeconds, g.TmdbTimeoutSeconds)
	}

	// Out of range values are clamped, not saved as 0 (no limit at all).
	if g.OmdbLimiterCalls != 1 || g.OmdbLimiterSeconds != 1 {
		t.Errorf("omdb: %d/%d", g.OmdbLimiterCalls, g.OmdbLimiterSeconds)
	}

	imdb := config.GetSettingsImdb()
	if imdb == nil || strings.Join(imdb.Indexedtypes, "|") != "movie|tvSeries" || strings.Join(imdb.Indexedlanguages, "|") != "DE|\\N" ||
		imdb.Indexfull || !imdb.UseMemory || imdb.ImdbIDSize == 0 {
		t.Fatalf("imdb: %+v", imdb)
	}
}

func TestWizardRecommendedImdb(t *testing.T) {
	movies := wizardRecommendedImdb(wizardState{Groups: map[string]wizardGroupState{"movies": {Enabled: true, Name: "EN"}}})
	if slices.Contains(movies.Indexedtypes, "tvSeries") || !slices.Contains(movies.Indexedtypes, "movie") {
		t.Errorf("movies only: %v", movies.Indexedtypes)
	}

	both := wizardRecommendedImdb(wizardState{Groups: map[string]wizardGroupState{
		"movies": {Enabled: true, Name: "EN"}, "series": {Enabled: true, Name: "DE"},
	}})
	if !slices.Contains(both.Indexedtypes, "tvSeries") {
		t.Errorf("with series: %v", both.Indexedtypes)
	}
}
