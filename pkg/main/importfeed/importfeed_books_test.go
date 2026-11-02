package importfeed

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

func TestAudiobookLanguageAllowed(t *testing.T) {
	tests := []struct {
		lang    string
		allowed []string
		want    bool
	}{
		{"english", []string{"en", "de"}, true},
		{"English", []string{"us"}, true},
		{"german", []string{"de"}, true},
		{"german", []string{"en"}, false},
		{"japanese", []string{"jp"}, true},
		{"en-US", []string{"en"}, true},
		{"de", []string{"de"}, true},
		{"french", nil, true},
		{"", []string{"en"}, true},
		{"klingon", []string{"en"}, false},
	}

	for _, tt := range tests {
		if got := audiobookLanguageAllowed(tt.lang, tt.allowed); got != tt.want {
			t.Errorf("audiobookLanguageAllowed(%q, %v) = %v, want %v",
				tt.lang, tt.allowed, got, tt.want)
		}
	}
}

// TestImportAudiobooksByAuthor_DanBrown tests audiobook import for author "Dan Brown"
// to debug why most dbaudiobooks columns are empty and why no chapters are imported.
//
// This test does NOT require a database - it only tests the API calls and data retrieval.
// Run with: go test -v -run TestImportAudiobooksByAuthor_DanBrown
func TestImportAudiobooksByAuthor_DanBrown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Initialize providers
	audibleProvider := audible.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}, audible.RegionUS)

	audnexProvider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
	})

	// Register providers so they can be used by importAndMergeAudiobook
	providers.SetAudible("us", audibleProvider)
	providers.SetAudnex(audnexProvider)

	authorName := "Dan Brown"

	t.Logf("=== Testing Audible SearchByAuthor for '%s' ===", authorName)

	// Step 1: Search Audible for audiobooks by author
	audiobooks, err := audibleProvider.SearchByAuthor(ctx, authorName, 10)
	if err != nil {
		t.Logf("ERROR: Audible SearchByAuthor failed: %v", err)
	} else {
		t.Logf("SUCCESS: Found %d audiobooks from Audible", len(audiobooks))

		for i, ab := range audiobooks {
			t.Logf("\n--- Audiobook %d ---", i+1)
			t.Logf("  Title:          %s", ab.Title)
			t.Logf("  ASIN:           %s", ab.ASIN)
			t.Logf("  ID:             %s", ab.ID)
			t.Logf("  Authors:        %v", ab.Authors)
			t.Logf("  Narrators:      %v", ab.Narrators)
			t.Logf("  Description:    %s", truncateString(ab.Description, 100))
			t.Logf("  CoverURL:       %s", ab.CoverURL)
			t.Logf("  Series:         %s", ab.Series)
			t.Logf("  SeriesPosition: %s", ab.SeriesPosition)
			t.Logf("  RuntimeMinutes: %d", ab.RuntimeMinutes)
			t.Logf("  ReleaseYear:    %d", ab.ReleaseYear)
			t.Logf("  ProviderType:   %s", ab.ProviderType)

			// Step 2: For each audiobook, test getting full details from Audible
			if ab.ASIN != "" {
				t.Logf("\n  >>> Testing Audible GetByASIN for ASIN: %s", ab.ASIN)
				details, err := audibleProvider.SearchByASIN(ctx, ab.ASIN)
				if err != nil {
					t.Logf("  ERROR: Audible SearchByASIN failed: %v", err)
				} else if details != nil {
					t.Logf("  --- Audible Details ---")
					t.Logf("    Title:          %s", details.Title)
					t.Logf("    Subtitle:       %s", details.Subtitle)
					t.Logf("    ASIN:           %s", details.ASIN)
					t.Logf("    Authors:        %v", details.Authors)
					t.Logf("    AuthorIDs:      %v", details.AuthorIDs)
					t.Logf("    Narrators:      %v", details.Narrators)
					t.Logf("    NarratorIDs:    %v", details.NarratorIDs)
					t.Logf("    Description:    %s", truncateString(details.Description, 100))
					t.Logf("    Summary:        %s", truncateString(details.Summary, 100))
					t.Logf("    CoverURL:       %s", details.CoverURL)
					t.Logf("    Series:         %s", details.Series)
					t.Logf("    SeriesASIN:     %s", details.SeriesASIN)
					t.Logf("    SeriesPosition: %s", details.SeriesPosition)
					t.Logf("    RuntimeMinutes: %d", details.RuntimeMinutes)
					t.Logf("    Duration:       %v", details.Duration)
					t.Logf("    Publisher:      %s", details.Publisher)
					t.Logf("    Language:       %s", details.Language)
					t.Logf("    ReleaseYear:    %d", details.ReleaseYear)
					t.Logf("    ReleaseDate:    %v", details.ReleaseDate)
					t.Logf("    Genres:         %v", details.Genres)
					t.Logf("    Categories:     %v", details.Categories)
					t.Logf("    Rating:         %.2f", details.Rating)
					t.Logf("    AverageRating:  %.2f", details.AverageRating)
					t.Logf("    RatingsCount:   %d", details.RatingsCount)
					t.Logf("    ISBN:           %s", details.ISBN)
					t.Logf("    Chapters:       %d chapters from Audible", len(details.Chapters))
					for j, ch := range details.Chapters {
						if j < 5 { // Only show first 5 chapters
							t.Logf(
								"      Ch %d: %s (start: %dms, length: %dms)",
								ch.ChapterNumber,
								ch.Title,
								ch.StartOffsetMs,
								ch.LengthMs,
							)
						}
					}
				} else {
					t.Logf("  WARNING: Audible SearchByASIN returned nil details")
				}

				// Step 3: Test getting chapters from Audnex
				t.Logf("\n  >>> Testing Audnex GetChaptersByASIN for ASIN: %s", ab.ASIN)
				chapters, err := audnexProvider.GetChaptersByASIN(ctx, ab.ASIN, "us")
				if err != nil {
					t.Logf("  ERROR: Audnex GetChaptersByASIN failed: %v", err)
				} else {
					t.Logf("  SUCCESS: Got %d chapters from Audnex", len(chapters))
					for j, ch := range chapters {
						if j < 5 { // Only show first 5 chapters
							t.Logf(
								"    Ch %d: %s (start: %dms, length: %dms)",
								ch.ChapterNumber,
								ch.Title,
								ch.StartOffsetMs,
								ch.LengthMs,
							)
						}
					}
				}

				// Step 4: Test getting book details from Audnex
				t.Logf("\n  >>> Testing Audnex GetBookByASIN for ASIN: %s", ab.ASIN)
				audnexDetails, err := audnexProvider.GetBookByASIN(ctx, ab.ASIN, "us")
				if err != nil {
					t.Logf("  ERROR: Audnex GetBookByASIN failed: %v", err)
				} else if audnexDetails != nil {
					t.Logf("  --- Audnex Details ---")
					t.Logf("    Title:          %s", audnexDetails.Title)
					t.Logf("    ASIN:           %s", audnexDetails.ASIN)
					t.Logf("    Authors:        %v", audnexDetails.Authors)
					t.Logf("    Narrators:      %v", audnexDetails.Narrators)
					t.Logf("    Description:    %s", truncateString(audnexDetails.Description, 100))
					t.Logf("    CoverURL:       %s", audnexDetails.CoverURL)
					t.Logf("    RuntimeMinutes: %d", audnexDetails.RuntimeMinutes)
					t.Logf("    Publisher:      %s", audnexDetails.Publisher)
					t.Logf("    Language:       %s", audnexDetails.Language)
					t.Logf("    ReleaseYear:    %d", audnexDetails.ReleaseYear)
				} else {
					t.Logf("  WARNING: Audnex GetBookByASIN returned nil details")
				}
			}

			// Only test first 3 audiobooks to avoid rate limiting
			if i >= 2 {
				t.Logf("\n... Stopping after 3 audiobooks to avoid rate limiting ...")
				break
			}
		}
	}

	t.Logf("\n\n=== ANALYSIS ===")
	t.Logf("Check the above output to see:")
	t.Logf("1. Which fields are empty in Audible search results")
	t.Logf("2. Which fields are empty in Audible details (SearchByASIN)")
	t.Logf("3. Whether Audnex returns chapters (GetChaptersByASIN)")
	t.Logf("4. What additional data Audnex provides (GetBookByASIN)")
	t.Logf("")
	t.Logf("If chapters are empty, check:")
	t.Logf("- Does the ASIN exist in Audnex database?")
	t.Logf("- Is the Audnex API returning data correctly?")
	t.Logf("- Are there network/rate-limit issues?")
}

// TestImportAndMergeAudiobook_SingleBook tests the full merge process for a single audiobook.
// This simulates what importAudiobooksByAuthor does but for a single known ASIN.
func TestImportAndMergeAudiobook_SingleBook(t *testing.T) {
	if testing.Short() {
		t.Skip("manual test hitting live Audible/Audnex APIs with a hardcoded ASIN that can go stale")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Initialize providers
	audibleProvider := audible.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}, audible.RegionUS)

	audnexProvider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
	})

	// Test with a known Dan Brown audiobook ASIN
	// "The Da Vinci Code" - this is a well-known audiobook that should have chapters
	// You may need to replace this ASIN with an actual valid one
	testASIN := "B000A14OMA" // Example ASIN - replace with a valid Dan Brown audiobook ASIN

	t.Logf("=== Testing importAndMergeAudiobook simulation for ASIN: %s ===", testASIN)

	// Simulate what importAndMergeAudiobook does
	merged := &apiexternal_v2.AudiobookDetails{
		ASIN: testASIN,
		ID:   testASIN,
	}
	var chapters []apiexternal_v2.AudiobookChapter

	// Step 1: Get data from Audible
	t.Logf("\n--- Step 1: Getting data from Audible ---")
	details, err := audibleProvider.SearchByASIN(ctx, testASIN)
	if err != nil {
		t.Logf("ERROR: Audible SearchByASIN failed: %v", err)
	} else if details != nil {
		t.Logf("Before merge - merged.Title: '%s'", merged.Title)
		mergeAudiobookDetails(merged, details)
		t.Logf("After merge - merged.Title: '%s'", merged.Title)
		t.Logf("Merged Audible data successfully")
		logMergedDetails(t, "After Audible merge", merged)
	}

	// Step 2: Get chapters from Audnex
	t.Logf("\n--- Step 2: Getting chapters from Audnex ---")
	audnexChapters, err := audnexProvider.GetChaptersByASIN(ctx, testASIN, "us")
	if err != nil {
		t.Logf("ERROR: Audnex GetChaptersByASIN failed: %v", err)
	} else {
		chapters = audnexChapters
		t.Logf("Got %d chapters from Audnex", len(chapters))
	}

	// Step 3: Get additional metadata from Audnex if needed
	if merged.Title == "" || merged.Description == "" {
		t.Logf("\n--- Step 3: Getting additional metadata from Audnex ---")
		audnexDetails, err := audnexProvider.GetBookByASIN(ctx, testASIN, "us")
		if err != nil {
			t.Logf("ERROR: Audnex GetBookByASIN failed: %v", err)
		} else if audnexDetails != nil {
			mergeAudiobookDetails(merged, audnexDetails)
			t.Logf("Merged Audnex data successfully")
			logMergedDetails(t, "After Audnex merge", merged)
		}
	}

	// Final summary
	t.Logf("\n\n=== FINAL MERGED RESULT ===")
	logMergedDetails(t, "Final", merged)
	t.Logf("Total chapters: %d", len(chapters))

	// Analyze what would be inserted into the database
	t.Logf("\n=== DATABASE INSERT ANALYSIS ===")
	t.Logf("The following values would be inserted into dbaudiobooks:")
	t.Logf("  title:           '%s' (empty: %v)", merged.Title, merged.Title == "")
	t.Logf("  asin:            '%s' (empty: %v)", merged.ASIN, merged.ASIN == "")
	t.Logf("  audible_id:      '%s' (empty: %v)", merged.ID, merged.ID == "")
	t.Logf("  year:            %d (zero: %v)", merged.ReleaseYear, merged.ReleaseYear == 0)
	t.Logf(
		"  description:     '%s' (empty: %v)",
		truncateString(merged.Description, 50),
		merged.Description == "",
	)
	t.Logf(
		"  cover_url:       '%s' (empty: %v)",
		truncateString(merged.CoverURL, 50),
		merged.CoverURL == "",
	)
	t.Logf("  language:        '%s' (empty: %v)", merged.Language, merged.Language == "")
	t.Logf("  runtime_minutes: %d (zero: %v)", merged.RuntimeMinutes, merged.RuntimeMinutes == 0)
	t.Logf("  chapter_count:   %d (zero: %v)", len(chapters), len(chapters) == 0)
	t.Logf("  publisher:       '%s' (empty: %v)", merged.Publisher, merged.Publisher == "")
	t.Logf("  average_rating:  %.2f (zero: %v)", merged.AverageRating, merged.AverageRating == 0)
	t.Logf("  ratings_count:   %d (zero: %v)", merged.RatingsCount, merged.RatingsCount == 0)

	if merged.Title == "" {
		t.Error("PROBLEM: Title is empty - audiobook won't be saved!")
	}

	if len(chapters) == 0 {
		t.Log("WARNING: No chapters found - chapters won't be imported")
	}
}

// Helper functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func logMergedDetails(t *testing.T, label string, d *apiexternal_v2.AudiobookDetails) {
	t.Logf("\n%s:", label)
	t.Logf("  Title:          %s", d.Title)
	t.Logf("  ASIN:           %s", d.ASIN)
	t.Logf("  ID:             %s", d.ID)
	t.Logf("  Authors:        %v", d.Authors)
	t.Logf("  Narrators:      %v", d.Narrators)
	t.Logf("  Description:    %s", truncateString(d.Description, 50))
	t.Logf("  CoverURL:       %s", truncateString(d.CoverURL, 50))
	t.Logf("  RuntimeMinutes: %d", d.RuntimeMinutes)
	t.Logf("  Publisher:      %s", d.Publisher)
	t.Logf("  Language:       %s", d.Language)
	t.Logf("  ReleaseYear:    %d", d.ReleaseYear)
	t.Logf("  AverageRating:  %.2f", d.AverageRating)
	t.Logf("  RatingsCount:   %d", d.RatingsCount)
}

// TestImportAudiobooksByAuthorFlow tests the complete flow that importAudiobooksByAuthor uses.
// This does NOT touch the database - it only shows what data would be collected.
func TestImportAudiobooksByAuthorFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create mock config that doesn't require database
	mockCfgp := &config.MediaTypeConfig{
		Name:       "test_audiobooks",
		NamePrefix: "audiobook_test",
		Lists: []config.MediaListsConfig{
			{
				Name: "test_list",
				CfgQuality: &config.QualityConfig{
					Name: "test_quality",
				},
			},
		},
	}

	mockBook := &config.ManualConfig{
		AuthorName: "Dan Brown",
	}

	// Initialize providers
	audibleProvider := audible.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}, audible.RegionUS)

	audnexProvider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
	})

	// This won't actually save to database, but will show the flow
	t.Logf("=== Simulating importAudiobooksByAuthor flow for '%s' ===", mockBook.AuthorName)

	// Search Audible
	audiobooks, err := audibleProvider.SearchByAuthor(ctx, mockBook.AuthorName, 100)
	if err != nil {
		t.Fatalf("Audible search failed: %v", err)
	}

	t.Logf("Found %d audiobooks", len(audiobooks))

	// For each audiobook, simulate the merge process
	for i, ab := range audiobooks {
		if i >= 3 {
			t.Logf("\n... Stopping after 3 audiobooks ...")
			break
		}

		t.Logf(
			"\n\n========== Processing Audiobook %d: %s (ASIN: %s) ==========",
			i+1,
			ab.Title,
			ab.ASIN,
		)

		if ab.ASIN == "" {
			t.Logf("SKIP: No ASIN available")
			continue
		}

		// Simulate importAndMergeAudiobook
		merged := &apiexternal_v2.AudiobookDetails{
			ASIN: ab.ASIN,
			ID:   ab.ASIN,
		}
		var chapters []apiexternal_v2.AudiobookChapter

		// Get from Audible
		details, err := audibleProvider.SearchByASIN(ctx, ab.ASIN)
		if err == nil && details != nil {
			mergeAudiobookDetails(merged, details)
			t.Logf("Merged Audible data: title='%s', runtime=%d, language='%s'",
				merged.Title, merged.RuntimeMinutes, merged.Language)
		}

		// Get chapters from Audnex
		audnexChapters, err := audnexProvider.GetChaptersByASIN(ctx, ab.ASIN, "us")
		if err == nil && len(audnexChapters) > 0 {
			chapters = audnexChapters
			t.Logf("Got %d chapters from Audnex", len(chapters))
		} else {
			t.Logf("No chapters from Audnex: %v", err)
		}

		// Get additional from Audnex if needed
		if merged.Title == "" || merged.Description == "" {
			audnexDetails, err := audnexProvider.GetBookByASIN(ctx, ab.ASIN, "us")
			if err == nil && audnexDetails != nil {
				mergeAudiobookDetails(merged, audnexDetails)
				t.Logf("Merged additional Audnex data")
			}
		}

		// Show what would be saved
		t.Logf("\n--- Would save to database ---")
		t.Logf("  title:           '%s'", merged.Title)
		t.Logf("  asin:            '%s'", merged.ASIN)
		t.Logf("  description:     '%s'", truncateString(merged.Description, 50))
		t.Logf("  cover_url:       '%s'", truncateString(merged.CoverURL, 50))
		t.Logf("  runtime_minutes: %d", merged.RuntimeMinutes)
		t.Logf("  language:        '%s'", merged.Language)
		t.Logf("  publisher:       '%s'", merged.Publisher)
		t.Logf("  chapter_count:   %d", len(chapters))
		t.Logf("  average_rating:  %.2f", merged.AverageRating)

		// Highlight problems
		if merged.Title == "" {
			t.Error("  PROBLEM: title is empty!")
		}
		if merged.RuntimeMinutes == 0 {
			t.Log("  WARNING: runtime_minutes is 0")
		}
		if merged.Language == "" {
			t.Log("  WARNING: language is empty")
		}
		if len(chapters) == 0 {
			t.Log("  WARNING: no chapters")
		}
	}

	_ = mockCfgp // suppress unused warning
}

// TestAudnexChaptersDirectly tests the Audnex API directly to see if it returns chapters.
func TestAudnexChaptersDirectly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	audnexProvider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
	})

	// Test with some known popular audiobook ASINs
	// These are example ASINs - you may need to replace with actual valid ones
	testASINs := []string{
		"B0036NLKE2", // "The Da Vinci Code" (example - may not be exact)
		"B000OIZUA4", // Another popular audiobook (example)
		"B017V4IMVQ", // Another popular audiobook (example)
	}

	for _, asin := range testASINs {
		t.Logf("\n=== Testing ASIN: %s ===", asin)

		chapters, err := audnexProvider.GetChaptersByASIN(ctx, asin, "us")
		if err != nil {
			t.Logf("ERROR: %v", err)
			continue
		}

		t.Logf("Got %d chapters", len(chapters))
		for i, ch := range chapters {
			if i < 3 {
				t.Logf("  Chapter %d: %s (start: %dms, length: %dms)",
					ch.ChapterNumber, ch.Title, ch.StartOffsetMs, ch.LengthMs)
			}
		}

		// Also test GetBookByASIN
		book, err := audnexProvider.GetBookByASIN(ctx, asin, "us")
		if err != nil {
			t.Logf("GetBookByASIN ERROR: %v", err)
		} else if book != nil {
			t.Logf("Book title: %s", book.Title)
			t.Logf("Book authors: %v", book.Authors)
		}
	}
}

// TestAudnexAPIDirectHTTP tests the Audnex API directly using HTTP calls
// with the same parameters that beets-audible uses (region and update=1).
// This helps debug if the issue is with the provider implementation or the API itself.
func TestAudnexAPIDirectHTTP(t *testing.T) {
	// Test with known Dan Brown audiobook ASINs
	// These are real ASINs from Audible US store
	testCases := []struct {
		name string
		asin string
	}{
		{"The Da Vinci Code", "B0009JKV9W"},
		{"Angels & Demons", "B002V8KTYC"},
		{"Inferno", "B00BWV5NR8"},
	}

	region := "us"
	client := &http.Client{Timeout: 30 * time.Second}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test book endpoint with region parameter (like beets-audible)
			bookURL := "https://api.audnex.us/books/" + tc.asin + "?region=" + region + "&update=1"
			t.Logf("Fetching book: %s", bookURL)

			req, _ := http.NewRequest("GET", bookURL, nil)
			req.Header.Set(
				"User-Agent",
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.36",
			)

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("ERROR: Book request failed: %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				t.Logf("ERROR: Book request returned status %d: %s", resp.StatusCode, string(body))
				return
			}

			var bookData map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&bookData); err != nil {
				t.Logf("ERROR: Failed to decode book response: %v", err)
				return
			}

			t.Logf("SUCCESS: Got book data")
			t.Logf("  asin:            %v", bookData["asin"])
			t.Logf("  title:           %v", bookData["title"])
			t.Logf("  subtitle:        %v", bookData["subtitle"])
			t.Logf("  authors:         %v", bookData["authors"])
			t.Logf("  narrators:       %v", bookData["narrators"])
			t.Logf("  description:     %v", truncateInterface(bookData["description"], 80))
			t.Logf("  image:           %v", bookData["image"])
			t.Logf("  runtimeLengthMin:%v", bookData["runtimeLengthMin"])
			t.Logf("  releaseDate:     %v", bookData["releaseDate"])
			t.Logf("  language:        %v", bookData["language"])
			t.Logf("  publisherName:   %v", bookData["publisherName"])
			t.Logf("  rating:          %v", bookData["rating"])
			t.Logf("  seriesPrimary:   %v", bookData["seriesPrimary"])
			t.Logf("  genres:          %v", bookData["genres"])

			// Test chapters endpoint
			chaptersURL := "https://api.audnex.us/books/" + tc.asin + "/chapters?region=" + region + "&update=1"
			t.Logf("\nFetching chapters: %s", chaptersURL)

			req2, _ := http.NewRequest("GET", chaptersURL, nil)
			req2.Header.Set(
				"User-Agent",
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.36",
			)

			resp2, err := client.Do(req2)
			if err != nil {
				t.Logf("ERROR: Chapters request failed: %v", err)
				return
			}
			defer resp2.Body.Close()

			if resp2.StatusCode != 200 {
				body, _ := io.ReadAll(resp2.Body)
				t.Logf(
					"ERROR: Chapters request returned status %d: %s",
					resp2.StatusCode,
					string(body),
				)
				return
			}

			var chaptersData map[string]any
			if err := json.NewDecoder(resp2.Body).Decode(&chaptersData); err != nil {
				t.Logf("ERROR: Failed to decode chapters response: %v", err)
				return
			}

			t.Logf("SUCCESS: Got chapters data")
			t.Logf("  asin:               %v", chaptersData["asin"])
			t.Logf("  runtimeLengthMs:    %v", chaptersData["runtimeLengthMs"])
			t.Logf("  runtimeLengthSec:   %v", chaptersData["runtimeLengthSec"])

			if chapters, ok := chaptersData["chapters"].([]any); ok {
				t.Logf("  chapters count:     %d", len(chapters))
				for i, ch := range chapters {
					if i < 3 {
						if chMap, ok := ch.(map[string]any); ok {
							t.Logf("    Ch %d: %v (start: %vms, length: %vms)",
								i+1, chMap["title"], chMap["startOffsetMs"], chMap["lengthMs"])
						}
					}
				}
				if len(chapters) > 3 {
					t.Logf("    ... and %d more chapters", len(chapters)-3)
				}
			} else {
				t.Logf("  chapters:           NONE or invalid format")
			}

			// Add delay between requests to avoid rate limiting
			time.Sleep(500 * time.Millisecond)
		})
	}
}

func truncateInterface(v any, maxLen int) string {
	if v == nil {
		return "<nil>"
	}
	s, ok := v.(string)
	if !ok {
		return "<non-string>"
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestAudnexAuthorSearch tests searching for an author's books via Audnex
// Note: Audnex requires the author's ASIN, not name. We need to get this from Audible first.
func TestAudnexAuthorSearch(t *testing.T) {
	// Dan Brown's Audible author ASIN (you may need to find the correct one)
	// This can be found by searching on Audible and looking at the author page URL
	authorASIN := "B000AP9DSU" // Dan Brown's author ASIN on Audible

	region := "us"
	client := &http.Client{Timeout: 30 * time.Second}

	// Test author endpoint
	authorURL := "https://api.audnex.us/authors/" + authorASIN + "?region=" + region
	t.Logf("Fetching author: %s", authorURL)

	req, _ := http.NewRequest("GET", authorURL, nil)
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.36",
	)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Author request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Author request returned status %d: %s", resp.StatusCode, string(body))
	}

	var authorData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&authorData); err != nil {
		t.Fatalf("Failed to decode author response: %v", err)
	}

	t.Logf("Author info:")
	t.Logf("  asin:        %v", authorData["asin"])
	t.Logf("  name:        %v", authorData["name"])
	t.Logf("  description: %v", truncateInterface(authorData["description"], 100))
	t.Logf("  image:       %v", authorData["image"])

	// Test author books endpoint
	booksURL := "https://api.audnex.us/authors/" + authorASIN + "/books?region=" + region
	t.Logf("\nFetching author books: %s", booksURL)

	req2, _ := http.NewRequest("GET", booksURL, nil)
	req2.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.36",
	)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Author books request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		body, _ := io.ReadAll(resp2.Body)
		t.Logf(
			"Author books endpoint not supported (status %d): %s",
			resp2.StatusCode,
			string(body),
		)
		return
	}

	var booksData map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&booksData); err != nil {
		t.Fatalf("Failed to decode author books response: %v", err)
	}

	t.Logf("Author books:")
	if books, ok := booksData["books"].([]any); ok {
		t.Logf("  Total books: %d", len(books))
		for i, book := range books {
			if i < 5 {
				if bookMap, ok := book.(map[string]any); ok {
					t.Logf("  %d: %v (ASIN: %v)", i+1, bookMap["title"], bookMap["asin"])
				}
			}
		}
		if len(books) > 5 {
			t.Logf("  ... and %d more books", len(books)-5)
		}
	}
}

type fakeGRAuthor struct {
	pages map[int][]apiexternal_v2.BookSearchResult
	calls int
}

func (f *fakeGRAuthor) GetBooksByAuthor(
	_ context.Context,
	_ string,
	page int,
) ([]apiexternal_v2.BookSearchResult, error) {
	f.calls++
	if b, ok := f.pages[page]; ok {
		return b, nil
	}

	if page > 10 {
		return nil, errors.New("unexpected page")
	}

	return nil, nil
}

func TestFetchGoodreadsAuthorBooksPaginates(t *testing.T) {
	f := &fakeGRAuthor{pages: map[int][]apiexternal_v2.BookSearchResult{
		1: {{ID: "1"}, {ID: "2"}},
		2: {{ID: "3"}},
	}}

	if got := fetchGoodreadsAuthorBooks(context.Background(), f, "a"); len(got) != 3 || f.calls != 3 {
		t.Errorf("books=%d calls=%d, want 3/3", len(got), f.calls)
	}

	// A server that ignores the page parameter returns page 1 forever.
	same := []apiexternal_v2.BookSearchResult{{ID: "1"}}
	f = &fakeGRAuthor{pages: map[int][]apiexternal_v2.BookSearchResult{}}
	for i := 1; i <= 20; i++ {
		f.pages[i] = same
	}

	if got := fetchGoodreadsAuthorBooks(context.Background(), f, "a"); len(got) != 1 || f.calls != 2 {
		t.Errorf("repeating pages: books=%d calls=%d, want 1/2", len(got), f.calls)
	}
}
