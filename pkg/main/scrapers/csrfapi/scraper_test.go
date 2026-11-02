package csrfapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

// TestCSRFAPIIntegration tests the actual CSRF API scraper integration
// This test makes real HTTP requests to verify the scraper works correctly.
//
// Example config from series_x.toml:
// name = "vivthomas"
// scraper_type = "csrfapi"
// start_url = "https://www.vivthomas.com/movies"
// site_url = "https://www.vivthomas.com"
// csrf_cookie_name = "_csrf"
// csrf_header_name = "csrf-token"
// api_url_pattern = "https://www.vivthomas.com/api/updates?tab=STREAM&page={page}&order=DATE&direction=DESC"
// page_start_index = 1
// results_array_path = "galleries"
// title_field = "name"
// date_field = "publishedAt"
// url_field = "path"
// actors_field = "models"
// actor_name_field = "name"
// runtime_field = "runtime"
// date_format = ""
// wait_seconds = 2
func TestCSRFAPIIntegration(t *testing.T) {
	// Skip in short mode since this makes real HTTP requests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Initialize logger for tests
	logger.InitLogger(logger.Config{
		LogLevel:      "info",
		LogFileSize:   0, // Disable file logging for tests
		LogToFileOnly: false,
		LogColorize:   false,
		LogCompress:   false,
	})

	// Create test configuration matching the series_x.toml config for vivthomas
	cfg := &Config{
		SiteName:        "vivthomas",
		StartURL:        "https://www.vivthomas.com/movies",
		BaseURL:         "https://www.vivthomas.com",
		SerieName:       "vivthomas",
		FirstPageDBOnly: true,

		// CSRF settings
		CSRFCookieName: "_csrf",
		CSRFHeaderName: "csrf-token",

		// API settings
		APIURLPattern:  "https://www.vivthomas.com/api/updates?tab=STREAM&page={page}&order=DATE&direction=DESC",
		PageStartIndex: 1,

		// JSON field paths
		ResultsArrayPath: "galleries",
		TitleField:       "name",
		DateField:        "publishedAt",
		URLField:         "path",
		ActorsField:      "models",
		ActorNameField:   "name",
		RuntimeField:     "runtime",

		// Date format (empty means use ISO 8601)
		DateFormat: "",

		// Wait time
		WaitSeconds: 2,
	}

	// Create scraper instance
	scraper, err := NewScraper(cfg)
	if err != nil {
		t.Fatalf("Failed to create scraper: %v", err)
	}

	// Test 1: Validate configuration
	t.Run("ValidateConfiguration", func(t *testing.T) {
		if scraper.config.CSRFCookieName == "" {
			t.Error("CSRFCookieName is empty")
		}
		if scraper.config.CSRFHeaderName == "" {
			t.Error("CSRFHeaderName is empty")
		}
		if scraper.config.APIURLPattern == "" {
			t.Error("APIURLPattern is empty")
		}
		if scraper.config.ResultsArrayPath == "" {
			t.Error("ResultsArrayPath is empty")
		}
		if scraper.config.PageStartIndex != 1 {
			t.Errorf("Expected page start index 1, got: %d", scraper.config.PageStartIndex)
		}

		t.Logf("Configuration validated: site=%s, cookie=%s, header=%s, start_index=%d",
			scraper.config.SiteName,
			scraper.config.CSRFCookieName,
			scraper.config.CSRFHeaderName,
			scraper.config.PageStartIndex)
	})

	// Test 2: Build API URLs
	t.Run("BuildAPIURLs", func(t *testing.T) {
		testCases := []struct {
			page        int
			expectedURL string
		}{
			{
				1,
				"https://www.vivthomas.com/api/updates?tab=STREAM&page=1&order=DATE&direction=DESC",
			},
			{
				2,
				"https://www.vivthomas.com/api/updates?tab=STREAM&page=2&order=DATE&direction=DESC",
			},
			{
				10,
				"https://www.vivthomas.com/api/updates?tab=STREAM&page=10&order=DATE&direction=DESC",
			},
		}

		for _, tc := range testCases {
			url := strings.ReplaceAll(cfg.APIURLPattern, "{page}", fmt.Sprintf("%d", tc.page))

			if url != tc.expectedURL {
				t.Errorf("Page %d: expected URL %q, got %q", tc.page, tc.expectedURL, url)
			} else {
				t.Logf("Page %d URL built correctly: %s", tc.page, url)
			}
		}
	})

	// Test 3: Extract CSRF token
	t.Run("ExtractCSRFToken", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := scraper.extractCSRFToken(ctx)
		if err != nil {
			t.Skipf("Skipping - website may be unavailable or blocking requests: %v", err)
			return
		}

		if scraper.csrfToken == "" {
			t.Error("CSRF token is empty")
		} else {
			t.Logf("Successfully extracted CSRF token (length: %d)", len(scraper.csrfToken))
		}

		// Verify cookie jar has cookies
		if scraper.cookieJar == nil {
			t.Error("Cookie jar is nil")
		}
	})

	// Test 4: Fetch first API page
	t.Run("FetchFirstAPIPage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Extract CSRF token first
		err := scraper.extractCSRFToken(ctx)
		if err != nil {
			t.Skipf("Skipping - website may be unavailable or blocking requests: %v", err)
			return
		}

		// Fetch first page
		data, err := scraper.fetchAPIPage(ctx, cfg.PageStartIndex)
		if err != nil {
			t.Skipf("Skipping - API may be unavailable: %v", err)
			return
		}

		if data == nil {
			t.Error("API response data is nil")
			return
		}

		if obj, ok := data.(map[string]any); ok {
			t.Logf("Successfully fetched API page, response has %d top-level keys", len(obj))
		} else {
			t.Log("Successfully fetched API page")
		}

		// Check for results array
		results := scraper.extractResults(data)
		if results == nil {
			t.Error("Results array is nil")
			return
		}

		t.Logf("Found %d results in first page", len(results))

		// Validate first result structure if available
		if len(results) > 0 {
			firstResult, ok := results[0].(map[string]any)
			if !ok {
				t.Error("First result is not a map")
				return
			}

			title := scraper.extractStringField(firstResult, cfg.TitleField)
			if title == "" {
				t.Log("Warning: First result has empty title")
			} else {
				t.Logf("First result title: %s", title)
			}

			// Check for date field
			if dateVal := firstResult[cfg.DateField]; dateVal != nil {
				t.Logf("First result has date field: %v", dateVal)
			}

			// Check for actors
			actors := scraper.extractActors(firstResult)
			if actors != "" {
				t.Logf("First result actors: %s", actors)
			}
		}
	})

	// Test 5: Test JSON field extraction
	t.Run("JSONFieldExtraction", func(t *testing.T) {
		// Test extractStringField
		testObj := map[string]any{
			"name":        "Test Title",
			"path":        "/video/12345",
			"publishedAt": "2024-03-15T10:30:00Z",
		}

		title := scraper.extractStringField(testObj, "name")
		if title != "Test Title" {
			t.Errorf("Expected title 'Test Title', got: %s", title)
		}

		path := scraper.extractStringField(testObj, "path")
		if path != "/video/12345" {
			t.Errorf("Expected path '/video/12345', got: %s", path)
		}

		// Test extractActors
		testObjWithActors := map[string]any{
			"models": []any{
				map[string]any{"name": "Actor One"},
				map[string]any{"name": "Actor Two"},
			},
		}

		actors := scraper.extractActors(testObjWithActors)
		expected := "Actor One, Actor Two"
		if actors != expected {
			t.Errorf("Expected actors %q, got: %q", expected, actors)
		} else {
			t.Logf("Successfully extracted actors: %s", actors)
		}
	})

	// Test 6: Test date parsing
	t.Run("ParseDateFormats", func(t *testing.T) {
		testCases := []struct {
			name       string
			dateVal    any
			shouldFail bool
		}{
			{
				name:       "ISO8601String",
				dateVal:    "2024-03-15T10:30:00Z",
				shouldFail: false,
			},
			{
				name:       "ISO8601WithOffset",
				dateVal:    "2024-03-15T10:30:00+01:00",
				shouldFail: false,
			},
			{
				name:       "TimeObject",
				dateVal:    time.Now(),
				shouldFail: false,
			},
			{
				// JSON numbers decode as float64 (e.g. goccy/go-json unmarshaling
				// into `any`); this is what a Unix-timestamp date field like
				// Bellesa's "posted_on" looks like once parsed.
				name:       "UnixTimestampFloat64",
				dateVal:    float64(1710498600),
				shouldFail: false,
			},
			{
				name:       "InvalidString",
				dateVal:    "not a date",
				shouldFail: true,
			},
			{
				name:       "InvalidType",
				dateVal:    12345,
				shouldFail: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				parsed, err := scraper.parseDate(tc.dateVal)
				if tc.shouldFail {
					if err == nil {
						t.Errorf("Expected error for %v, but got none", tc.dateVal)
					} else {
						t.Logf("Correctly failed to parse: %v", tc.dateVal)
					}
					return
				}

				if err != nil {
					t.Errorf("Failed to parse date %v: %v", tc.dateVal, err)
					return
				}

				t.Logf(
					"Successfully parsed date: %v -> %s",
					tc.dateVal,
					parsed.Format(time.RFC3339),
				)
			})
		}
	})

	// Test 7: Test cookie jar functionality
	t.Run("CookieJarFunctionality", func(t *testing.T) {
		if scraper.cookieJar == nil {
			t.Fatal("Cookie jar is nil")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Extract token to populate cookie jar
		err := scraper.extractCSRFToken(ctx)
		if err != nil {
			t.Skipf("Skipping - website may be unavailable: %v", err)
			return
		}

		t.Log("Cookie jar successfully stores and manages cookies")
	})

	// Test 8: Configuration validation errors
	t.Run("ConfigurationValidation", func(t *testing.T) {
		invalidConfigs := []struct {
			name   string
			config *Config
			errMsg string
		}{
			{
				name: "MissingSiteName",
				config: &Config{
					StartURL:         "https://example.com",
					SerieName:        "test",
					APIURLPattern:    "https://api.example.com/{page}",
					ResultsArrayPath: "data",
				},
				errMsg: "site_name is required",
			},
			{
				name: "MissingStartURL",
				config: &Config{
					SiteName:         "test",
					SerieName:        "test",
					APIURLPattern:    "https://api.example.com/{page}",
					ResultsArrayPath: "data",
				},
				errMsg: "start_url is required",
			},
			{
				name: "MissingSerieName",
				config: &Config{
					SiteName:         "test",
					StartURL:         "https://example.com",
					APIURLPattern:    "https://api.example.com/{page}",
					ResultsArrayPath: "data",
				},
				errMsg: "serie_name is required",
			},
			{
				name: "MissingAPIURLPattern",
				config: &Config{
					SiteName:         "test",
					StartURL:         "https://example.com",
					SerieName:        "test",
					ResultsArrayPath: "data",
				},
				errMsg: "api_url_pattern is required",
			},
		}

		for _, tc := range invalidConfigs {
			t.Run(tc.name, func(t *testing.T) {
				_, err := NewScraper(tc.config)
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
					return
				}

				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("Expected error containing %q, got: %v", tc.errMsg, err)
				} else {
					t.Logf("Correctly rejected invalid config: %s", tc.errMsg)
				}
			})
		}
	})

	// Test 9: Test default values
	t.Run("DefaultValues", func(t *testing.T) {
		minimalCfg := &Config{
			SiteName:         "test",
			StartURL:         "https://example.com",
			SerieName:        "test",
			APIURLPattern:    "https://api.example.com/{page}",
			ResultsArrayPath: "data",
		}

		testScraper, err := NewScraper(minimalCfg)
		if err != nil {
			t.Fatalf("Failed to create scraper with minimal config: %v", err)
		}

		// Check defaults
		if testScraper.config.CSRFCookieName != "_csrf" {
			t.Errorf(
				"Expected default CSRF cookie name '_csrf', got: %s",
				testScraper.config.CSRFCookieName,
			)
		}

		if testScraper.config.CSRFHeaderName != "csrf-token" {
			t.Errorf(
				"Expected default CSRF header name 'csrf-token', got: %s",
				testScraper.config.CSRFHeaderName,
			)
		}

		if testScraper.config.PageStartIndex != 1 {
			t.Errorf(
				"Expected default page start index 1, got: %d",
				testScraper.config.PageStartIndex,
			)
		}

		if testScraper.config.PaginationStyle != "page" {
			t.Errorf(
				"Expected default pagination style 'page', got: %s",
				testScraper.config.PaginationStyle,
			)
		}

		if testScraper.config.WaitSeconds != 2 {
			t.Errorf("Expected default wait seconds 2, got: %d", testScraper.config.WaitSeconds)
		}

		if testScraper.config.BaseURL != minimalCfg.StartURL {
			t.Errorf("Expected BaseURL to default to StartURL, got: %s", testScraper.config.BaseURL)
		}

		t.Log("All default values are correctly set")
	})

	// Test 10: Test runtime field filtering
	t.Run("RuntimeFieldFiltering", func(t *testing.T) {
		testCases := []struct {
			name         string
			result       map[string]any
			shouldSkip   bool
			runtimeField string
		}{
			{
				name:         "ValidRuntime",
				result:       map[string]any{"runtime": 1200},
				shouldSkip:   false,
				runtimeField: "runtime",
			},
			{
				name:         "InvalidRuntimeNegative",
				result:       map[string]any{"runtime": -1},
				shouldSkip:   true,
				runtimeField: "runtime",
			},
			{
				// What JSON decoding actually produces for -1.
				name:         "InvalidRuntimeNegativeFloat",
				result:       map[string]any{"runtime": float64(-1)},
				shouldSkip:   true,
				runtimeField: "runtime",
			},
			{
				name:         "ValidRuntimeFloat",
				result:       map[string]any{"runtime": float64(1200)},
				shouldSkip:   false,
				runtimeField: "runtime",
			},
			{
				name:         "InvalidRuntimeString",
				result:       map[string]any{"runtime": "-1"},
				shouldSkip:   true,
				runtimeField: "runtime",
			},
			{
				name:         "MissingRuntime",
				result:       map[string]any{},
				shouldSkip:   true,
				runtimeField: "runtime",
			},
			{
				name:         "NoRuntimeCheck",
				result:       map[string]any{"runtime": -1},
				shouldSkip:   false,
				runtimeField: "",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				shouldSkip := false
				if tc.runtimeField != "" {
					shouldSkip = isMissingRuntime(tc.result[tc.runtimeField])
				}

				if shouldSkip != tc.shouldSkip {
					t.Errorf(
						"Expected shouldSkip=%v, got %v for runtime check",
						tc.shouldSkip,
						shouldSkip,
					)
				} else {
					t.Logf("Runtime filtering correctly determined: skip=%v", shouldSkip)
				}
			})
		}
	})
}

// TestCSRFAPIHelperFunctions tests individual helper functions
func TestCSRFAPIHelperFunctions(t *testing.T) {
	cfg := &Config{
		SiteName:         "test",
		StartURL:         "https://example.com",
		SerieName:        "test",
		APIURLPattern:    "https://api.example.com/{page}",
		ResultsArrayPath: "results",
		TitleField:       "title",
		ActorsField:      "actors",
		ActorNameField:   "name",
	}

	scraper, err := NewScraper(cfg)
	if err != nil {
		t.Fatalf("Failed to create scraper: %v", err)
	}

	t.Run("ExtractStringField", func(t *testing.T) {
		tests := []struct {
			obj      map[string]any
			field    string
			expected string
		}{
			{
				obj:      map[string]any{"title": "Test Title"},
				field:    "title",
				expected: "Test Title",
			},
			{
				obj:      map[string]any{"title": ""},
				field:    "title",
				expected: "",
			},
			{
				obj:      map[string]any{"other": "value"},
				field:    "title",
				expected: "",
			},
			{
				obj:      map[string]any{"title": 123},
				field:    "title",
				expected: "",
			},
			{
				// JSON numbers decode as float64 (e.g. Bellesa's numeric "id"
				// field, used to build the episode URL).
				obj:      map[string]any{"id": float64(13508)},
				field:    "id",
				expected: "13508",
			},
		}

		for _, tt := range tests {
			result := scraper.extractStringField(tt.obj, tt.field)
			if result != tt.expected {
				t.Errorf("extractStringField(%v, %q) = %q, expected %q",
					tt.obj, tt.field, result, tt.expected)
			}
		}
	})

	t.Run("ExtractResults", func(t *testing.T) {
		tests := []struct {
			name     string
			data     map[string]any
			expected int
		}{
			{
				name: "ValidResults",
				data: map[string]any{
					"results": []any{
						map[string]any{"id": 1},
						map[string]any{"id": 2},
					},
				},
				expected: 2,
			},
			{
				name:     "MissingResults",
				data:     map[string]any{},
				expected: 0,
			},
			{
				name: "WrongType",
				data: map[string]any{
					"results": "not an array",
				},
				expected: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				results := scraper.extractResults(tt.data)
				if len(results) != tt.expected {
					t.Errorf("Expected %d results, got %d", tt.expected, len(results))
				}
			})
		}
	})

	// A ResultsArrayPath of "" means the API response itself is a top-level
	// JSON array (e.g. Bellesa's videos endpoint), not an object wrapping one.
	t.Run("ExtractResults_RootArray", func(t *testing.T) {
		rootArrayScraper, err := NewScraper(&Config{
			SiteName:      "test",
			StartURL:      "https://example.com",
			SerieName:     "test",
			APIURLPattern: "https://api.example.com/videos",
		})
		if err != nil {
			t.Fatalf("Failed to create scraper: %v", err)
		}

		tests := []struct {
			name     string
			data     any
			expected int
		}{
			{
				name: "ValidRootArray",
				data: []any{
					map[string]any{"id": 1},
					map[string]any{"id": 2},
					map[string]any{"id": 3},
				},
				expected: 3,
			},
			{
				name:     "WrongTypeObject",
				data:     map[string]any{"results": []any{map[string]any{"id": 1}}},
				expected: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				results := rootArrayScraper.extractResults(tt.data)
				if len(results) != tt.expected {
					t.Errorf("Expected %d results, got %d", tt.expected, len(results))
				}
			})
		}
	})
}

func TestResolveURL(t *testing.T) {
	base := "https://example.com/shows/list/page.html"

	for ref, want := range map[string]string{
		"":                          "",
		"episode/1":                 "https://example.com/shows/list/episode/1",
		"/img/poster.jpg":           "https://example.com/img/poster.jpg",
		"https://cdn.example.org/x": "https://cdn.example.org/x",
		"//cdn.example.org/y":       "https://cdn.example.org/y",
		"?page=2":                   "https://example.com/shows/list/page.html?page=2",
	} {
		if got := resolveURL(base, ref); got != want {
			t.Errorf("resolveURL(%q) = %q, want %q", ref, got, want)
		}
	}

	// An unparsable base leaves the reference unchanged.
	if got := resolveURL("http://[::1", "x/y"); got != "x/y" {
		t.Errorf("bad base = %q", got)
	}
}

func TestIsMissingRuntime(t *testing.T) {
	tests := []struct {
		v    any
		want bool
	}{
		{nil, true},
		{float64(-1), true},
		{-1, true},
		{json.Number("-1"), true},
		{"-1", true},
		{" -1 ", true},
		{float64(0), false},
		{float64(1200), false},
		{"1200", false},
		{json.Number("1200"), false},
	}

	for _, tt := range tests {
		if got := isMissingRuntime(tt.v); got != tt.want {
			t.Errorf("isMissingRuntime(%#v) = %v, want %v", tt.v, got, tt.want)
		}
	}
}

func TestNormalizePageStartIndex(t *testing.T) {
	for in, want := range map[int]int{0: 1, 1: 1, 2: 2, -1: 0, -5: 0} {
		if got := normalizePageStartIndex(in); got != want {
			t.Errorf("normalizePageStartIndex(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestParseDateFormats(t *testing.T) {
	s := &Scraper{config: &Config{}}

	tests := []struct {
		in   any
		want string
	}{
		{"2024-03-15T10:30:00Z", "2024-03-15"},
		{"2024-03-15", "2024-03-15"},
		{"2024-03-15 10:30:00", "2024-03-15"},
		{float64(1710498600), "2024-03-15"},    // seconds
		{float64(1710498600000), "2024-03-15"}, // milliseconds
	}

	for _, tt := range tests {
		got, err := s.parseDate(tt.in)
		if err != nil {
			t.Errorf("parseDate(%v): %v", tt.in, err)
			continue
		}

		if got.Format("2006-01-02") != tt.want {
			t.Errorf("parseDate(%v) = %s, want %s", tt.in, got.Format("2006-01-02"), tt.want)
		}
	}

	custom := &Scraper{config: &Config{DateFormat: "02.01.2006"}}
	if got, err := custom.parseDate("15.03.2024"); err != nil || got.Format("2006-01-02") != "2024-03-15" {
		t.Errorf("custom DateFormat: got %v, %v", got, err)
	}

	// A configured DateFormat must not prevent the ISO fallbacks.
	if _, err := custom.parseDate("2024-03-15"); err != nil {
		t.Errorf("ISO fallback with DateFormat set: %v", err)
	}
}

func TestPageValueAndOffsetValidation(t *testing.T) {
	base := Config{
		SiteName:      "t",
		StartURL:      "https://example.com",
		SerieName:     "t",
		APIURLPattern: "https://example.com/api?offset={page}",
	}

	noSize := base
	noSize.PaginationStyle = "offset"

	if _, err := NewScraper(&noSize); err == nil {
		t.Error("expected error for offset pagination without page_increment")
	}

	off := base
	off.PaginationStyle = "offset"
	off.PageIncrement = 24

	s, err := NewScraper(&off)
	if err != nil {
		t.Fatal(err)
	}

	for page, want := range map[int]int{1: 0, 2: 24, 3: 48} {
		if got := s.pageValue(page); got != want {
			t.Errorf("offset pageValue(%d) = %d, want %d", page, got, want)
		}
	}

	seq := base
	seq.PaginationStyle = "sequential"

	s, err = NewScraper(&seq)
	if err != nil {
		t.Fatal(err)
	}

	if got := s.pageValue(3); got != 3 {
		t.Errorf("sequential pageValue(3) = %d, want 3", got)
	}

	zeroBased := base
	zeroBased.PageStartIndex = -1

	s, err = NewScraper(&zeroBased)
	if err != nil {
		t.Fatal(err)
	}

	if s.config.PageStartIndex != 0 {
		t.Errorf("PageStartIndex -1 should become 0, got %d", s.config.PageStartIndex)
	}
}

// csrfServer issues a new token on every start-page visit and rejects API
// calls carrying any token other than the latest one with rejectStatus.
func csrfServer(t *testing.T, rejectStatus int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var tokens atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/start":
			n := tokens.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "_csrf", Value: "tok" + string(rune('0'+n)), Path: "/"})
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/api"):
			want := "tok" + string(rune('0'+tokens.Load()))
			// First token is treated as expired.
			if tokens.Load() < 2 || r.Header.Get("csrf-token") != want {
				w.WriteHeader(rejectStatus)
				return
			}

			_, _ = w.Write([]byte(body))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, &tokens
}

func TestFetchAPIPageRefreshesCSRFToken(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, statusCSRFExpired} {
		srv, tokens := csrfServer(t, status, `{"results":[{"title":"a"}]}`)

		s, err := NewScraper(&Config{
			SiteName:         "t",
			StartURL:         srv.URL + "/start",
			SerieName:        "t",
			APIURLPattern:    srv.URL + "/api?page={page}",
			ResultsArrayPath: "results",
			WaitSeconds:      -1,
		})
		if err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		if err := s.extractCSRFToken(ctx); err != nil {
			cancel()
			t.Fatal(err)
		}

		data, err := s.fetchAPIPage(ctx, 1)
		cancel()

		if err != nil {
			t.Fatalf("status %d: expected retry with fresh token to succeed, got %v", status, err)
		}

		if got := len(s.extractResults(data)); got != 1 {
			t.Errorf("status %d: expected 1 result, got %d", status, got)
		}

		if tokens.Load() != 2 {
			t.Errorf("status %d: expected exactly one token refresh, got %d fetches", status, tokens.Load())
		}
	}
}
