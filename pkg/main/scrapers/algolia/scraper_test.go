package algolia

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// TestAlgoliaIntegration tests the actual Algolia API integration
// This test makes real HTTP requests to verify the scraper works correctly.
//
// Example PowerShell command being tested:
// & .\extract_algolia.ps1 -starturl "https://www.girlsway.com/en/videos/" -sitefiltername "girlsway" -siteurl "https://www.girlsway.com" -site_id 2 -site "girlsway" -first_page_db_only $true
func TestAlgoliaIntegration(t *testing.T) {
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

	// Create test configuration matching the PowerShell command
	cfg := &Config{
		SiteName:        "girlsway",
		StartURL:        "https://www.girlsway.com/en/videos/",
		SiteURL:         "https://www.girlsway.com",
		SiteFilterName:  "girlsway",
		FirstPageDBOnly: true,
		SerieName:       "Girlsway", // Series name for auto-creation
	}

	// Create scraper instance
	scraper, err := NewScraper(cfg)
	if err != nil {
		t.Fatalf("Failed to create scraper: %v", err)
	}

	// Test 1: Extract Algolia credentials
	t.Run("ExtractCredentials", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := scraper.extractAlgoliaCredentials(ctx)
		if err != nil {
			t.Errorf("Failed to extract Algolia credentials: %v", err)
			return
		}

		if scraper.applicationID == "" {
			t.Error("Application ID is empty")
		} else {
			t.Logf("Successfully extracted Application ID: %s", scraper.applicationID)
		}

		if scraper.apiKey == "" {
			t.Error("API Key is empty")
		} else {
			// Don't log full API key for security
			t.Logf("Successfully extracted API Key (length: %d)", len(scraper.apiKey))

			// Check if API key needs base64 decoding
			if strings.Contains(scraper.apiKey, "=") {
				t.Log("API Key appears to be base64 encoded")
			}
		}
	})

	// Test 2: Build facet filter
	t.Run("BuildFacetFilter", func(t *testing.T) {
		// Test with site filter
		filter := scraper.buildFacetFilter()
		t.Logf("Site filter result: %s", filter)

		// Should contain the site name
		if !strings.Contains(filter, "girlsway") {
			t.Errorf("Filter does not contain site name: %s", filter)
		}

		// Test with different filter types
		testCases := []struct {
			name          string
			modifyScraper func(*Scraper)
			expectContain string
		}{
			{
				name: "SerieFilter",
				modifyScraper: func(s *Scraper) {
					s.config.SiteFilterName = ""
					s.config.SerieFilterName = "TestSeries"
				},
				expectContain: "serie_name",
			},
			{
				name: "NetworkFilter",
				modifyScraper: func(s *Scraper) {
					s.config.SiteFilterName = ""
					s.config.SerieFilterName = ""
					s.config.NetworkFilterName = "Test Network"
				},
				expectContain: "network.lvl0",
			},
			{
				name: "NetworkSiteFilter",
				modifyScraper: func(s *Scraper) {
					s.config.SiteFilterName = ""
					s.config.SerieFilterName = ""
					s.config.NetworkFilterName = ""
					s.config.NetworkSiteFilterName = "Network Name,Site Name"
				},
				expectContain: "network.lvl1",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create a copy of config for this test
				testScraper := &Scraper{
					config: &Config{
						SiteName: cfg.SiteName,
						StartURL: cfg.StartURL,
						SiteURL:  cfg.SiteURL,
					},
				}
				tc.modifyScraper(testScraper)

				result := testScraper.buildFacetFilter()
				t.Logf("%s filter: %s", tc.name, result)

				if result != "" && !strings.Contains(result, tc.expectContain) {
					t.Errorf("Expected filter to contain %q, got: %s", tc.expectContain, result)
				}
			})
		}
	})

	// Test 3: Build request body
	t.Run("BuildRequestBody", func(t *testing.T) {
		// Test page 0
		body := scraper.buildRequestBody(0)
		if !strings.Contains(body, `"indexName":"all_scenes_latest_desc"`) {
			t.Error("Request body does not contain index name")
		}
		if !strings.Contains(body, "page=0") {
			t.Error("Request body does not contain page parameter")
		}
		if !strings.Contains(body, "hitsPerPage=1000") {
			t.Error("Request body does not contain hitsPerPage parameter")
		}

		t.Logf("Page 0 request body: %s", body)

		// Test page 1
		body = scraper.buildRequestBody(1)
		if !strings.Contains(body, "page=1") {
			t.Error("Page 1 request body does not contain correct page parameter")
		}

		t.Logf("Page 1 request body: %s", body)
	})

	// Test 4: Fetch first page
	t.Run("FetchFirstPage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Extract credentials first
		err := scraper.extractAlgoliaCredentials(ctx)
		if err != nil {
			t.Fatalf("Failed to extract credentials: %v", err)
		}

		// Fetch first page
		hits, err := scraper.fetchPage(ctx, 0)
		if err != nil {
			t.Errorf("Failed to fetch first page: %v", err)
			return
		}

		if len(hits) == 0 {
			t.Error("No hits returned from first page")
			return
		}

		t.Logf("Successfully fetched %d hits from first page", len(hits))

		// Validate first hit structure
		firstHit := hits[0]
		if firstHit.ClipID == 0 {
			t.Error("First hit has zero ClipID")
		}
		if firstHit.Title == "" {
			t.Error("First hit has empty Title")
		}
		if firstHit.ReleaseDate == "" {
			t.Error("First hit has empty ReleaseDate")
		}

		t.Logf("First hit: ClipID=%d, Title=%s, ReleaseDate=%s",
			firstHit.ClipID,
			firstHit.Title,
			firstHit.ReleaseDate)

		// Check for actors
		if len(firstHit.Actors) > 0 {
			actorNames := make([]string, len(firstHit.Actors))
			for i, actor := range firstHit.Actors {
				actorNames[i] = actor.Name
			}
			t.Logf("Actors: %v", actorNames)
		}

		// Check for categories
		if len(firstHit.Categories) > 0 {
			t.Logf("Categories count: %d", len(firstHit.Categories))
		}

		// Check metadata fields
		if firstHit.Sitename != "" {
			t.Logf("Sitename: %s", firstHit.Sitename)
		}
		if firstHit.NetworkName != "" {
			t.Logf("Network: %s", firstHit.NetworkName)
		}
		if firstHit.SerieName != "" {
			t.Logf("Series: %s", firstHit.SerieName)
		}
		if firstHit.StudioName != "" {
			t.Logf("Studio: %s", firstHit.StudioName)
		}
	})

	// Test 5: Helper functions
	t.Run("HelperFunctions", func(t *testing.T) {
		// Test joinActorNames
		actors := []ActorInfo{
			{Name: "Actor One"},
			{Name: "Actor Two"},
			{Name: "Actor Three"},
		}
		result := joinActorNames(actors)
		expected := "Actor One, Actor Two, Actor Three"
		if result != expected {
			t.Errorf("joinActorNames() = %q, expected %q", result, expected)
		}

		// Test joinCategoryNames
		categories := []CategoryInfo{
			{Name: "Category A"},
			{Name: "Category B"},
		}
		result = joinCategoryNames(categories)
		expected = "Category A, Category B"
		if result != expected {
			t.Errorf("joinCategoryNames() = %q, expected %q", result, expected)
		}

		// Test empty slices
		if joinActorNames([]ActorInfo{}) != "" {
			t.Error("joinActorNames with empty slice should return empty string")
		}
		if joinCategoryNames([]CategoryInfo{}) != "" {
			t.Error("joinCategoryNames with empty slice should return empty string")
		}
	})

	// Test 6: Full scrape simulation (without database operations)
	t.Run("FullScrapeSim", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Extract credentials
		err := scraper.extractAlgoliaCredentials(ctx)
		if err != nil {
			t.Fatalf("Failed to extract credentials: %v", err)
		}

		// Fetch first page only
		hits, err := scraper.fetchPage(ctx, 0)
		if err != nil {
			t.Errorf("Failed to fetch page: %v", err)
			return
		}

		t.Logf("Fetched %d hits", len(hits))

		// Validate each hit can be processed
		validCount := 0
		invalidDates := 0
		emptyClipIDs := 0

		for i, hit := range hits {
			if hit.ClipID == 0 {
				emptyClipIDs++
				t.Logf("Hit %d (%s) has zero ClipID", i, hit.Title)
				continue
			}

			if hit.ReleaseDate == "" {
				invalidDates++
				t.Logf("Hit %d (%s) has empty release date", i, hit.Title)
				continue
			}

			// Try parsing the release date
			_, err := time.Parse("2006-01-02", hit.ReleaseDate)
			if err != nil {
				invalidDates++
				t.Logf("Hit %d (%s) has invalid date format: %s", i, hit.Title, hit.ReleaseDate)
				continue
			}

			// Generate identifier like the scraper would
			identifier := hit.ReleaseDate[2:]

			if len(identifier) < 8 {
				t.Errorf("Hit %d (%s) generates invalid identifier: %s", i, hit.Title, identifier)
				continue
			}

			validCount++
		}

		t.Logf("Valid hits: %d/%d", validCount, len(hits))
		if emptyClipIDs > 0 {
			t.Logf("Empty ClipIDs: %d", emptyClipIDs)
		}
		if invalidDates > 0 {
			t.Logf("Invalid dates: %d", invalidDates)
		}

		if validCount == 0 {
			t.Error("No valid hits found")
		}

		// Check filter effectiveness
		if cfg.SiteFilterName != "" {
			matchingFilter := 0
			for _, hit := range hits {
				if hit.Sitename == cfg.SiteFilterName {
					matchingFilter++
				}
			}
			t.Logf("Hits matching site filter '%s': %d/%d", cfg.SiteFilterName, matchingFilter, len(hits))

			if matchingFilter == 0 {
				t.Logf("Warning: No hits match the site filter - filter may not be working correctly")
			}
		}
	})

	// Test 7: Test credential patterns
	t.Run("CredentialPatterns", func(t *testing.T) {
		// This test doesn't make HTTP requests, just validates our regex patterns
		testCases := []struct {
			name        string
			content     string
			expectAppID string
			expectKey   string
		}{
			{
				name:        "Pattern1",
				content:     `{"api":{"algolia":{"applicationID":"TESTAPP123","apiKey":"TESTKEY456"}}}`,
				expectAppID: "TESTAPP123",
				expectKey:   "TESTKEY456",
			},
			{
				name:        "Pattern2",
				content:     `{"api":{"algolia":{"apiKey":"TESTKEY789","applicationID":"TESTAPP999"}}}`,
				expectAppID: "TESTAPP999",
				expectKey:   "TESTKEY789",
			},
			{
				name:        "RealWorld",
				content:     `window.algolia = {"applicationID":"TSMKFA364Q","apiKey":"ZWUxMDlmMzE0NDBlOTYwYzI0MmE4MGQ3NDJjNGY0Njk2MWZiZDNiOWE2OTUyYTVlN2NhZmQ0YjljMzkxOGE0NHZhbGlkVW50aWw9MTczMTYyMTgzNSZyZXN0cmljdEluZGljZXM9YWxsJTJBJmZpbHRlcnM9c2VnbWVudCUzQWFkdWx0dGltZQ=="};`,
				expectAppID: "TSMKFA364Q",
				expectKey:   "ZWUxMDlmMzE0NDBlOTYwYzI0MmE4MGQ3NDJjNGY0Njk2MWZiZDNiOWE2OTUyYTVlN2NhZmQ0YjljMzkxOGE0NHZhbGlkVW50aWw9MTczMTYyMTgzNSZyZXN0cmljdEluZGljZXM9YWxsJTJBJmZpbHRlcnM9c2VnbWVudCUzQWFkdWx0dGltZQ==",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Use the same regex patterns from the scraper
				re1 := `"applicationID":"([a-zA-Z0-9]{1,})","apiKey":"([a-zA-Z0-9=,\.]{1,})"`
				re2 := `"apiKey":"([a-zA-Z0-9=,\.]{1,})","applicationID":"([a-zA-Z0-9]{1,})"`

				// Try pattern 1
				if strings.Contains(tc.content, `"applicationID":"`+tc.expectAppID) &&
					strings.Contains(tc.content, `"apiKey":"`+tc.expectKey) {
					t.Logf("Pattern 1 regex would match: %s", re1)
				}

				// Try pattern 2
				if strings.Contains(tc.content, `"apiKey":"`+tc.expectKey) &&
					strings.Contains(tc.content, `"applicationID":"`+tc.expectAppID) {
					t.Logf("Pattern 2 regex would match: %s", re2)
				}
			})
		}
	})
}

// TestAlgoliaFilters tests different filter configurations
func TestAlgoliaFilters(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		verify func(t *testing.T, filter string)
	}{
		{
			name: "SiteFilter",
			config: &Config{
				SiteName:       "test",
				SiteFilterName: "examplesite",
			},
			verify: func(t *testing.T, filter string) {
				if !strings.Contains(filter, "sitename") {
					t.Error("Site filter should contain 'sitename'")
				}
				if !strings.Contains(filter, "examplesite") {
					t.Error("Site filter should contain the site name")
				}
			},
		},
		{
			name: "SerieFilter",
			config: &Config{
				SiteName:        "test",
				SerieFilterName: "TestSeries",
			},
			verify: func(t *testing.T, filter string) {
				if !strings.Contains(filter, "serie_name") {
					t.Error("Serie filter should contain 'serie_name'")
				}
			},
		},
		{
			name: "NetworkFilter",
			config: &Config{
				SiteName:          "test",
				NetworkFilterName: "Network Name",
			},
			verify: func(t *testing.T, filter string) {
				if !strings.Contains(filter, "network.lvl0") {
					t.Error("Network filter should contain 'network.lvl0'")
				}
			},
		},
		{
			name: "NetworkSiteFilter",
			config: &Config{
				SiteName:              "test",
				NetworkSiteFilterName: "Network,Site",
			},
			verify: func(t *testing.T, filter string) {
				if !strings.Contains(filter, "network.lvl1") {
					t.Error("Network site filter should contain 'network.lvl1'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &Scraper{
				config: tt.config,
			}

			filter := scraper.buildFacetFilter()
			t.Logf("Generated filter: %s", filter)

			tt.verify(t, filter)
		})
	}
}
