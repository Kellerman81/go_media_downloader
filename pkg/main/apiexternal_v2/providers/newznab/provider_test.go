package newznab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestProvider(t *testing.T, cfg ProviderConfig, handler http.HandlerFunc) *Provider {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg.BaseURL = srv.URL
	cfg.IndexerName = "test-" + t.Name()

	return NewProvider(cfg)
}

const rssTwoItems = `<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:newznab="http://www.newznab.com/DTD/2010/feeds/attributes/">
<channel>
<item>
  <title>Movie.2020.1080p</title>
  <guid>id-1</guid>
  <link>http://x/1.nzb</link>
  <category>Movies &gt; HD</category>
  <newznab:attr name="category" value="2000"/>
  <newznab:attr name="category" value="2040"/>
  <newznab:attr name="imdb" value="0133093"/>
</item>
<item>
  <title>Old</title>
  <guid>id-2</guid>
  <link>http://x/2.nzb</link>
  <newznab:attr name="imdb" value="0000000"/>
</item>
</channel>
</rss>`

func TestParseXMLCategoryAndImdb(t *testing.T) {
	p := newTestProvider(t, ProviderConfig{}, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssTwoItems))
	})

	res, err := p.ExecuteRequest(context.Background(), p.baseURL+"/api?t=search", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("got %d results, want 2", len(res))
	}

	if res[0].NZB.Category != "2040" {
		t.Errorf("category = %q, want 2040", res[0].NZB.Category)
	}

	if res[0].NZB.IMDBID != "tt0133093" {
		t.Errorf("imdb = %q, want tt0133093", res[0].NZB.IMDBID)
	}

	if res[1].NZB.IMDBID != "" {
		t.Errorf("placeholder imdb = %q, want empty", res[1].NZB.IMDBID)
	}
}

func TestParseXMLTillID(t *testing.T) {
	p := newTestProvider(t, ProviderConfig{}, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssTwoItems))
	})

	res, err := p.ExecuteRequest(context.Background(), p.baseURL+"/rss", "id-2", nil, nil)
	if !errors.Is(err, ErrBroke) || len(res) != 1 {
		t.Fatalf("got %d results, err %v; want 1 and ErrBroke", len(res), err)
	}
}

func TestErrorDocuments(t *testing.T) {
	tests := []struct {
		name      string
		json      bool
		body      string
		wantCode  string
		ratelimit bool
	}{
		{"xml bad key", false, `<?xml version="1.0"?><error code="100" description="Incorrect user credentials"/>`, "100", false},
		{"xml request limit", false, `<error code="500" description="Request limit reached"/>`, "500", true},
		{"json nested", true, `{"error":{"code":"100","description":"Incorrect user credentials"}}`, "100", false},
		{"json attributes", true, `{"error":{"@attributes":{"code":"501","description":"Download limit reached"}}}`, "501", true},
		{"json flat numeric", true, `{"code":100,"description":"Incorrect user credentials"}`, "100", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestProvider(t, ProviderConfig{OutputAsJSON: tt.json}, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := p.ExecuteRequest(context.Background(), p.baseURL+"/api?t=search", "", nil, nil)
			if !errors.Is(err, ErrIndexerAPI) || !strings.Contains(err.Error(), "code "+tt.wantCode) {
				t.Fatalf("err = %v, want ErrIndexerAPI with code %s", err, tt.wantCode)
			}

			_, err = p.ExecuteRequest(context.Background(), p.baseURL+"/api?t=search", "", nil, nil)

			blocked := err != nil && strings.Contains(err.Error(), "server rate limit active")
			if blocked != tt.ratelimit {
				t.Errorf("second request blocked = %v, want %v (err %v)", blocked, tt.ratelimit, err)
			}
		})
	}
}

func TestJSONTillIDReturnsBroke(t *testing.T) {
	tests := map[string]string{
		"format1": `{"channel":{"item":[{"title":"a","guid":{"_text":"id-1"}},{"title":"b","guid":{"_text":"id-2"}}]}}`,
		"format2": `{"item":[{"title":"a","guid":{"_text":"id-1"},"newznab:attr":[{"_name":"imdb","_value":"0133093"},{"_name":"category","_value":"2000"},{"_name":"category","_value":"2045"}]},{"title":"b","guid":{"_text":"id-2"}}]}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			p := newTestProvider(t, ProviderConfig{OutputAsJSON: true}, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			})

			res, err := p.ExecuteRequest(context.Background(), p.baseURL+"/rss", "id-2", nil, nil)
			if !errors.Is(err, ErrBroke) || len(res) != 1 {
				t.Fatalf("got %d results, err %v; want 1 and ErrBroke", len(res), err)
			}

			if name == "format2" && (res[0].NZB.IMDBID != "tt0133093" || res[0].NZB.Category != "2045") {
				t.Errorf("imdb %q category %q", res[0].NZB.IMDBID, res[0].NZB.Category)
			}
		})
	}
}

func TestCapsURLUsesCustomSettings(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ProviderConfig
		wantPath  string
		wantQuery string
	}{
		{"default", ProviderConfig{APIKey: "k"}, "/api", "apikey=k&t=caps"},
		{"custom api param", ProviderConfig{APIKey: "k", CustomAPI: "token"}, "/api", "token=k&t=caps"},
		{"custom path", ProviderConfig{APIKey: "k", CustomURL: "/newznab/api"}, "/newznab/api", "apikey=k&t=caps"},
		{"custom path with key", ProviderConfig{APIKey: "k", CustomURL: "/x/api?apikey=k"}, "/x/api", "apikey=k&t=caps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotQuery string

			cfg := tt.cfg
			cfg.Enabled = true

			newTestProvider(t, cfg, func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
				_, _ = w.Write([]byte(`<caps><server title="t"/></caps>`))
			})

			if gotPath != tt.wantPath || gotQuery != tt.wantQuery {
				t.Errorf("caps request = %s?%s, want %s?%s", gotPath, gotQuery, tt.wantPath, tt.wantQuery)
			}
		})
	}
}

func TestCapabilitiesErrorDocument(t *testing.T) {
	p := newTestProvider(t, ProviderConfig{}, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<error code="100" description="Incorrect user credentials"/>`))
	})

	if err := p.TestConnection(context.Background()); !errors.Is(err, ErrIndexerAPI) {
		t.Fatalf("err = %v, want ErrIndexerAPI", err)
	}
}

func TestCustomEndpoint(t *testing.T) {
	tests := []struct {
		customURL, customAPI, apiKey, want string
	}{
		{"", "", "k", "/api?apikey=k"},
		{"/newznab/api", "", "k", "/newznab/api?apikey=k"},
		{"/newznab/api", "token", "k", "/newznab/api?token=k"},
		{"https://c/api?x=1", "", "k", "https://c/api?x=1&apikey=k"},
		{"https://c/api?apikey=zz", "", "k", "https://c/api?apikey=zz"},
		{"https://c/api", "", "", "https://c/api"},
		{"/api", "", "a&b", "/api?apikey=a%26b"},
	}

	for _, tt := range tests {
		if got := CustomEndpoint(tt.customURL, tt.customAPI, tt.apiKey); got != tt.want {
			t.Errorf("CustomEndpoint(%q, %q, %q) = %q, want %q",
				tt.customURL, tt.customAPI, tt.apiKey, got, tt.want)
		}
	}
}
