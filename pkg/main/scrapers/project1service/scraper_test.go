package project1service

import (
	"context"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// TestProject1ServiceIntegration tests the actual Project1Service API integration
// This test makes real HTTP requests to verify the scraper works correctly.
//
// Example PowerShell command being tested:
// & .\extract_project1service.ps1 -starturl "https://www.twistys.com/scenes/" -filter_collectionid 227 -site_id 178 -site_name "whengirlsplay" -first_page_db_only $true
func TestProject1ServiceIntegration(t *testing.T) {
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
		SiteName:           "whengirlsplay",
		StartURL:           "https://www.twistys.com/scenes/",
		SiteID:             178,
		FilterCollectionID: 227,
		FirstPageDBOnly:    true,
		SerieName:          "WhenGirlsPlay", // Series name for auto-creation
	}

	// Create scraper instance
	scraper, err := NewScraper(cfg)
	if err != nil {
		t.Fatalf("Failed to create scraper: %v", err)
	}

	// Test 1: Get instance token
	t.Run("GetInstanceToken", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := scraper.getInstanceToken(ctx)
		if err != nil {
			t.Errorf("Failed to get instance token: %v", err)
			return
		}

		if scraper.instanceToken == "" {
			t.Error("Instance token is empty")
		} else {
			t.Logf("Successfully retrieved instance token (length: %d)", len(scraper.instanceToken))
		}
	})

	// Test 2: Build URL
	t.Run("BuildURL", func(t *testing.T) {
		// Test page 0
		url := scraper.buildURL(0)
		expectedBase := "https://site-api.project1service.com/v2/releases"
		if !contains(url, expectedBase) {
			t.Errorf("URL does not contain expected base: %s", url)
		}

		if !contains(url, "collectionId=227") {
			t.Errorf("URL does not contain collection filter: %s", url)
		}

		if !contains(url, "limit=100") {
			t.Errorf("URL does not contain limit parameter: %s", url)
		}

		t.Logf("Page 0 URL: %s", url)

		// Test page 1
		url = scraper.buildURL(1)
		if !contains(url, "offset=100") {
			t.Errorf("Page 1 URL does not contain offset: %s", url)
		}

		t.Logf("Page 1 URL: %s", url)
	})

	// Test 3: Fetch first page
	t.Run("FetchFirstPage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Get instance token first
		err := scraper.getInstanceToken(ctx)
		if err != nil {
			t.Fatalf("Failed to get instance token: %v", err)
		}

		// Fetch first page
		releases, err := scraper.fetchPage(ctx, 0)
		if err != nil {
			t.Errorf("Failed to fetch first page: %v", err)
			return
		}

		if len(releases) == 0 {
			t.Error("No releases returned from first page")
			return
		}

		t.Logf("Successfully fetched %d releases from first page", len(releases))

		// Validate first release structure
		firstRelease := releases[0]
		if firstRelease.ID == 0 {
			t.Error("First release has zero ID")
		}
		if firstRelease.Title == "" {
			t.Error("First release has empty Title")
		}
		if firstRelease.Brand == "" {
			t.Error("First release has empty Brand")
		}
		if firstRelease.DateReleased.IsZero() {
			t.Error("First release has zero DateReleased")
		}

		t.Logf("First release: ID=%d, Title=%s, Brand=%s, Date=%s",
			firstRelease.ID,
			firstRelease.Title,
			firstRelease.Brand,
			firstRelease.DateReleased.Format("2006-01-02"))

		// Check for actors
		if len(firstRelease.Actors) > 0 {
			actorNames := make([]string, len(firstRelease.Actors))
			for i, actor := range firstRelease.Actors {
				actorNames[i] = actor.Name
			}
			t.Logf("Actors: %v", actorNames)
		}

		// Check for tags
		if len(firstRelease.Tags) > 0 {
			t.Logf("Tags count: %d", len(firstRelease.Tags))
		}

		// Check for collections
		if len(firstRelease.Collections) > 0 {
			t.Logf("Collections count: %d", len(firstRelease.Collections))
		}

		// Check for groups
		if len(firstRelease.Groups) > 0 {
			t.Logf("Groups count: %d", len(firstRelease.Groups))
		}
	})

	// Test 4: Helper functions
	t.Run("HelperFunctions", func(t *testing.T) {
		// Test cleanTitle
		tests := []struct {
			input    string
			expected string
		}{
			{"Simple Title", "Simple-Title"},
			{"Title: With Colon", "Title-With-Colon"},
			{"Question?", "Question"},
			{"Exclaim!", "Exclaim"},
			{"Hash#Tag", "Hash-Tag"},
			{"Spaces   Multiple", "Spaces-Multiple"},
			{"A & B", "A-and-B"},
			{"Path/Slash", "Path-Slash"},
			{"Multiple---Dashes", "Multiple-Dashes"},
		}

		for _, tt := range tests {
			result := cleanTitle(tt.input)
			if result != tt.expected {
				t.Errorf("cleanTitle(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		}
	})

	// Test 5: Full scrape simulation (without database operations)
	t.Run("FullScrapeSim", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Get instance token
		err := scraper.getInstanceToken(ctx)
		if err != nil {
			t.Fatalf("Failed to get instance token: %v", err)
		}

		// Fetch first page only
		releases, err := scraper.fetchPage(ctx, 0)
		if err != nil {
			t.Errorf("Failed to fetch page: %v", err)
			return
		}

		t.Logf("Fetched %d releases", len(releases))

		// Validate each release can be processed
		validCount := 0
		for i, release := range releases {
			if release.DateReleased.IsZero() {
				t.Logf("Release %d (%s) has invalid date", i, release.Title)
				continue
			}

			// Generate identifier like the scraper would
			dateStr := release.DateReleased.Format("2006-01-02")
			identifier := dateStr[2:]

			if len(identifier) < 8 {
				t.Errorf("Release %d (%s) generates invalid identifier: %s", i, release.Title, identifier)
				continue
			}

			validCount++
		}

		t.Logf("Valid releases: %d/%d", validCount, len(releases))

		if validCount == 0 {
			t.Error("No valid releases found")
		}
	})
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsRune(s, substr))
}

func containsRune(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
