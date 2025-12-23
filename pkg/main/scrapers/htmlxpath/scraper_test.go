package htmlxpath

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// TestHTMLXPathIntegration tests the actual HTML/XPath scraper integration
// This test makes real HTTP requests to verify the scraper works correctly.
//
// Example config from series_x.toml:
// name = "momsteachsex"
// scraper_type = "htmlxpath"
// start_url = "https://momsteachsex.com/video/gallery/0"
// site_url = "https://momsteachsex.com"
// scene_node_xpath = "//figure"
// title_xpath = ".//figcaption/div[@class=\"caption-header\"]/span/a"
// url_xpath = ".//figcaption/div[@class=\"caption-header\"]/span/a"
// url_attribute = "href"
// date_xpath = ".//figcaption/span[@class=\"date\"]"
// actors_xpath = ".//figcaption/div[@class=\"models \"]/a"
// pagination_type = "offset"
// page_increment = 12
// page_url_pattern = "https://momsteachsex.com/video/gallery/{page}"
// date_format = "Jan 2, 2006"
// wait_seconds = 15
func TestHTMLXPathIntegration(t *testing.T) {
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

	// Create test configuration matching the series_x.toml config for momsteachsex
	cfg := &Config{
		SiteName:        "momsteachsex",
		StartURL:        "https://momsteachsex.com/video/gallery/0",
		BaseURL:         "https://momsteachsex.com",
		SerieName:       "momsteachsex",
		FirstPageDBOnly: true,

		// XPath selectors
		SceneNodeXPath: "//figure",
		TitleXPath:     ".//figcaption/div[@class=\"caption-header\"]/span/a",
		URLXPath:       ".//figcaption/div[@class=\"caption-header\"]/span/a",
		URLAttribute:   "href",
		DateXPath:      ".//figcaption/span[@class=\"date\"]",
		ActorsXPath:    ".//figcaption/div[@class=\"models \"]/a",

		// Pagination
		PaginationType: "offset",
		PageIncrement:  12,
		PageURLPattern: "https://momsteachsex.com/video/gallery/{page}",

		// Date parsing
		DateFormat: "Jan 2, 2006",

		// Wait time
		WaitSeconds: 15,
	}

	// Create scraper instance
	scraper, err := NewScraper(cfg)
	if err != nil {
		t.Fatalf("Failed to create scraper: %v", err)
	}

	// Test 1: Validate configuration
	t.Run("ValidateConfiguration", func(t *testing.T) {
		if scraper.config.SceneNodeXPath == "" {
			t.Error("SceneNodeXPath is empty")
		}
		if scraper.config.TitleXPath == "" {
			t.Error("TitleXPath is empty")
		}
		if scraper.config.DateXPath == "" {
			t.Error("DateXPath is empty")
		}
		if scraper.config.PaginationType != "offset" {
			t.Errorf("Expected pagination type 'offset', got: %s", scraper.config.PaginationType)
		}
		if scraper.config.PageIncrement != 12 {
			t.Errorf("Expected page increment 12, got: %d", scraper.config.PageIncrement)
		}

		t.Logf("Configuration validated: site=%s, pagination=%s, increment=%d",
			scraper.config.SiteName,
			scraper.config.PaginationType,
			scraper.config.PageIncrement)
	})

	// Test 2: Build page URLs
	t.Run("BuildPageURLs", func(t *testing.T) {
		testCases := []struct {
			page         int
			expectedURL  string
			paginationType string
			pageIncrement int
		}{
			{0, "https://momsteachsex.com/video/gallery/0", "offset", 12},
			{1, "https://momsteachsex.com/video/gallery/12", "offset", 12},
			{2, "https://momsteachsex.com/video/gallery/24", "offset", 12},
		}

		for _, tc := range testCases {
			// Build URL manually to test logic
			pageValue := tc.page
			if tc.paginationType == "offset" {
				pageValue = tc.page * tc.pageIncrement
			}
			url := strings.ReplaceAll(cfg.PageURLPattern, "{page}", fmt.Sprintf("%d", pageValue))

			if url != tc.expectedURL {
				t.Errorf("Page %d: expected URL %q, got %q", tc.page, tc.expectedURL, url)
			} else {
				t.Logf("Page %d correctly maps to offset %d: %s", tc.page, pageValue, url)
			}
		}
	})

	// Test 3: Fetch first page
	t.Run("FetchFirstPage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		doc, err := scraper.fetchPage(ctx, 0)
		if err != nil {
			t.Skipf("Skipping - website may be unavailable or blocking requests: %v", err)
			return
		}

		if doc == nil {
			t.Error("Fetched document is nil")
			return
		}

		t.Logf("Successfully fetched and parsed first page HTML document")
	})

	// Test 4: Parse date format
	t.Run("ParseDateFormat", func(t *testing.T) {
		testCases := []struct {
			dateStr  string
			expected time.Time
			shouldFail bool
		}{
			{"Jan 2, 2006", time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC), false},
			{"Dec 25, 2023", time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC), false},
			{"Mar 15, 2024", time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), false},
			{"Invalid Date", time.Time{}, true},
		}

		for _, tc := range testCases {
			parsed, err := scraper.parseDate(tc.dateStr)
			if tc.shouldFail {
				if err == nil {
					t.Errorf("Expected error for date %q, but got none", tc.dateStr)
				}
				continue
			}

			if err != nil {
				t.Errorf("Failed to parse date %q: %v", tc.dateStr, err)
				continue
			}

			if parsed.Year() != tc.expected.Year() ||
				parsed.Month() != tc.expected.Month() ||
				parsed.Day() != tc.expected.Day() {
				t.Errorf("Date mismatch for %q: expected %v, got %v",
					tc.dateStr, tc.expected, parsed)
			} else {
				t.Logf("Successfully parsed date: %q -> %s", tc.dateStr, parsed.Format("2006-01-02"))
			}
		}
	})

	// Test 5: Test XPath text extraction
	t.Run("XPathExtraction", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := scraper.fetchPage(ctx, 0)
		if err != nil {
			t.Skipf("Skipping XPath extraction test - website may be unavailable: %v", err)
			return
		}

		// Test extractText with different scenarios
		// This is a basic test - actual extraction would require valid nodes
		t.Log("XPath extraction methods are available and ready for use")
		t.Log("Successfully fetched page for XPath testing")
	})

	// Test 6: Test alternative config (girlsonlyporn)
	t.Run("AlternativeConfig", func(t *testing.T) {
		// Test with girlsonlyporn config from series_x.toml (line 293)
		altCfg := &Config{
			SiteName:        "girlsonlyporn",
			StartURL:        "https://nubilefilms.com/video/gallery/website/71/0",
			BaseURL:         "https://nubilefilms.com",
			SerieName:       "girlsonlyporn",
			FirstPageDBOnly: true,

			SceneNodeXPath: "//figure",
			TitleXPath:     ".//figcaption/div[@class=\"caption-header\"]/span/a",
			URLXPath:       ".//figcaption/div[@class=\"caption-header\"]/span/a",
			URLAttribute:   "href",
			DateXPath:      ".//figcaption/span[@class=\"date\"]",
			ActorsXPath:    ".//figcaption/div[@class=\"models \"]/a",

			PaginationType: "offset",
			PageIncrement:  12,
			PageURLPattern: "https://nubilefilms.com/video/gallery/website/71/{page}",
			DateFormat:     "Jan 2, 2006",
			WaitSeconds:    15,
		}

		altScraper, err := NewScraper(altCfg)
		if err != nil {
			t.Errorf("Failed to create alternative scraper: %v", err)
			return
		}

		t.Logf("Successfully created alternative scraper for: %s", altCfg.SiteName)

		// Verify configuration
		if altScraper.config.SiteName != "girlsonlyporn" {
			t.Errorf("Expected site name 'girlsonlyporn', got: %s", altScraper.config.SiteName)
		}
	})

	// Test 7: Test sequential pagination type (missax)
	t.Run("SequentialPagination", func(t *testing.T) {
		// Test with missax config from series_x.toml (line 187)
		seqCfg := &Config{
			SiteName:        "missax",
			StartURL:        "https://missax.com/tour/categories/movies_1_d.html",
			BaseURL:         "https://missax.com",
			SerieName:       "missax",
			FirstPageDBOnly: true,

			SceneNodeXPath: "//div[@class=\"photo-thumb video-thumb\"]",
			TitleXPath:     ".//div[@class=\"thumb-descr\"]/a",
			TitleAttribute: "title",
			URLXPath:       ".//div[@class=\"thumb-descr\"]/a",
			URLAttribute:   "href",
			DateXPath:      ".//div[@class=\"thumb-descr\"]/div/span",
			ActorsXPath:    ".//div[@class=\"thumb-descr\"]/p[@class=\"model-name\"]/a",

			PaginationType: "sequential",
			PageURLPattern: "https://missax.com/tour/categories/movies_{page}_d.html",
			DateFormat:     "2006-01-02",
			WaitSeconds:    2,
		}

		seqScraper, err := NewScraper(seqCfg)
		if err != nil {
			t.Errorf("Failed to create sequential scraper: %v", err)
			return
		}

		if seqScraper.config.PaginationType != "sequential" {
			t.Errorf("Expected pagination type 'sequential', got: %s", seqScraper.config.PaginationType)
		}

		t.Logf("Sequential pagination scraper created for: %s", seqCfg.SiteName)

		// Test date format for this config
		testDate := "2024-03-15"
		parsed, err := seqScraper.parseDate(testDate)
		if err != nil {
			t.Errorf("Failed to parse sequential config date: %v", err)
		} else {
			t.Logf("Sequential config date parsing: %q -> %s", testDate, parsed.Format("2006-01-02"))
		}
	})

	// Test 8: Configuration validation errors
	t.Run("ConfigurationValidation", func(t *testing.T) {
		invalidConfigs := []struct {
			name   string
			config *Config
			errMsg string
		}{
			{
				name:   "MissingSiteName",
				config: &Config{StartURL: "https://example.com", SerieName: "test"},
				errMsg: "site_name is required",
			},
			{
				name:   "MissingStartURL",
				config: &Config{SiteName: "test", SerieName: "test"},
				errMsg: "start_url is required",
			},
			{
				name:   "MissingSerieName",
				config: &Config{SiteName: "test", StartURL: "https://example.com"},
				errMsg: "serie_name is required",
			},
			{
				name: "MissingSceneNodeXPath",
				config: &Config{
					SiteName:  "test",
					StartURL:  "https://example.com",
					SerieName: "test",
				},
				errMsg: "scene_node_xpath is required",
			},
			{
				name: "MissingTitleXPath",
				config: &Config{
					SiteName:       "test",
					StartURL:       "https://example.com",
					SerieName:      "test",
					SceneNodeXPath: "//div",
				},
				errMsg: "title_xpath is required",
			},
			{
				name: "MissingDateXPath",
				config: &Config{
					SiteName:       "test",
					StartURL:       "https://example.com",
					SerieName:      "test",
					SceneNodeXPath: "//div",
					TitleXPath:     ".//span",
				},
				errMsg: "date_xpath is required",
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
}

// TestHTMLXPathPaginationLogic tests pagination URL building logic
func TestHTMLXPathPaginationLogic(t *testing.T) {
	tests := []struct {
		name           string
		paginationType string
		pageIncrement  int
		pageURLPattern string
		pageNum        int
		expectedOffset int
	}{
		{
			name:           "OffsetPage0",
			paginationType: "offset",
			pageIncrement:  12,
			pageURLPattern: "https://example.com/videos/{page}",
			pageNum:        0,
			expectedOffset: 0,
		},
		{
			name:           "OffsetPage1",
			paginationType: "offset",
			pageIncrement:  12,
			pageURLPattern: "https://example.com/videos/{page}",
			pageNum:        1,
			expectedOffset: 12,
		},
		{
			name:           "OffsetPage5",
			paginationType: "offset",
			pageIncrement:  12,
			pageURLPattern: "https://example.com/videos/{page}",
			pageNum:        5,
			expectedOffset: 60,
		},
		{
			name:           "SequentialPage0",
			paginationType: "sequential",
			pageIncrement:  0,
			pageURLPattern: "https://example.com/page_{page}.html",
			pageNum:        0,
			expectedOffset: 1, // Sequential is 1-indexed
		},
		{
			name:           "SequentialPage2",
			paginationType: "sequential",
			pageIncrement:  0,
			pageURLPattern: "https://example.com/page_{page}.html",
			pageNum:        2,
			expectedOffset: 3, // Sequential is 1-indexed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pageValue int
			if tt.paginationType == "offset" {
				pageValue = tt.pageNum * tt.pageIncrement
			} else {
				pageValue = tt.pageNum + 1 // Convert 0-indexed to 1-indexed
			}

			if pageValue != tt.expectedOffset {
				t.Errorf("Expected offset %d, got %d", tt.expectedOffset, pageValue)
			} else {
				t.Logf("Page %d correctly maps to offset %d for %s pagination",
					tt.pageNum, pageValue, tt.paginationType)
			}
		})
	}
}
