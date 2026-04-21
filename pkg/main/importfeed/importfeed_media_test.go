package importfeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/goodreads"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

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

// TestAudibleSearchResults tests what data Audible returns in search results
// to understand why columns might be empty.
func TestAudibleSearchResults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	audibleProvider := audible.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}, audible.RegionUS)

	t.Log("=== Testing Audible search result fields ===")

	// Search for Dan Brown audiobooks
	results, err := audibleProvider.SearchByAuthor(ctx, "Dan Brown", 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Found %d results", len(results))

	// Analyze each result to see which fields are populated
	for i, r := range results {
		t.Logf("\n--- Result %d: %s ---", i+1, r.Title)

		// Check each field and report if empty
		checkField(t, "ID", r.ID)
		checkField(t, "ASIN", r.ASIN)
		checkField(t, "Title", r.Title)
		checkField(t, "Subtitle", r.Subtitle)
		checkFieldSlice(t, "Authors", r.Authors)
		checkFieldSlice(t, "Narrators", r.Narrators)
		checkField(t, "Description", truncateString(r.Description, 50))
		checkField(t, "CoverURL", r.CoverURL)
		checkField(t, "Series", r.Series)
		checkField(t, "SeriesPosition", r.SeriesPosition)
		checkFieldInt(t, "RuntimeMinutes", r.RuntimeMinutes)
		checkFieldInt(t, "ReleaseYear", r.ReleaseYear)
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

func checkField(t *testing.T, name, value string) {
	if value == "" {
		t.Logf("  EMPTY: %s", name)
	} else {
		t.Logf("  OK:    %s = %s", name, truncateString(value, 40))
	}
}

func checkFieldSlice(t *testing.T, name string, value []string) {
	if len(value) == 0 {
		t.Logf("  EMPTY: %s", name)
	} else {
		t.Logf("  OK:    %s = %v", name, value)
	}
}

func checkFieldInt(t *testing.T, name string, value int) {
	if value == 0 {
		t.Logf("  ZERO:  %s", name)
	} else {
		t.Logf("  OK:    %s = %d", name, value)
	}
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

// TestOpenLibraryFallback tests the OpenLibrary fallback path
// which is used when Audible/Audnex don't have data.
func TestOpenLibraryFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	olProvider := openlibrary.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
		UserAgent:        "general.UserAgent",
	})

	t.Log("=== Testing OpenLibrary search for Dan Brown ===")

	books, err := olProvider.SearchBooks(ctx, "", "Dan Brown", 10)
	if err != nil {
		t.Fatalf("OpenLibrary search failed: %v", err)
	}

	t.Logf("Found %d books", len(books))

	for i, b := range books {
		if i >= 3 {
			break
		}
		t.Logf("\n--- Book %d ---", i+1)
		t.Logf("  Title:       %s", b.Title)
		t.Logf("  ID:          %s", b.ID)
		t.Logf("  Authors:     %v", b.Authors)
		t.Logf("  ISBN13:      %s", b.ISBN13)
		t.Logf("  ISBN10:      %s", b.ISBN10)
		t.Logf("  Description: %s", truncateString(b.Description, 50))
		t.Logf("  CoverURL:    %s", b.CoverURL)
		t.Logf("  PublishYear: %d", b.PublishYear)
	}

	t.Log("\nNOTE: OpenLibrary data is used as fallback when Audible/Audnex don't work.")
	t.Log("OpenLibrary does NOT provide audiobook-specific data like chapters or narrators.")
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
		t.Logf("Author books endpoint not supported (status %d): %s", resp2.StatusCode, string(body))
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

// TestVAAlbumRuntimeMatching tests the runtime matching flow for a Various Artists album.
// This reads actual files from a local folder and shows what metadata is extracted,
// how it would match against a database, and where runtime mismatches occur.
//
// Run with: go test -v -run TestVAAlbumRuntimeMatching
func TestVAAlbumRuntimeMatching(t *testing.T) {
	folder := `Q:\_tocheck_Music\wrong_runtime\VA-Kuschelrock.38-2CD-2024-NOiCE\VA_-_Kuschelrock_38-2CD-2024-NOiCE`

	// Step 1: Collect files
	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil {
		t.Fatalf("CollectFilesOnly failed: %v", err)
	}
	t.Logf("Found %d audio files", len(files))
	if len(files) == 0 {
		t.Skip("No audio files found - is the folder accessible?")
	}

	// Step 2: Parse folder name
	folderArtist, folderAlbum, year := parser_v2.ParseAudioFolder(folder)
	t.Logf("\n=== Folder Parsing ===")
	t.Logf("  folderArtist: '%s'", folderArtist)
	t.Logf("  folderAlbum:  '%s'", folderAlbum)
	t.Logf("  year:         %d", year)

	// Step 3: Parse first filename
	firstTrack := parser_v2.ParseAudioFilename(files[0])
	t.Logf("\n=== First File Parsing (%s) ===", filepath.Base(files[0]))
	t.Logf("  Artist:      '%s'", firstTrack.Artist)
	t.Logf("  Album:       '%s'", firstTrack.Album)
	t.Logf("  Title:       '%s'", firstTrack.Title)
	t.Logf("  TrackNumber: %d", firstTrack.TrackNumber)
	t.Logf("  DiscNumber:  %d", firstTrack.DiscNumber)

	// Step 4: Read tags from first file
	tagData := parser_v2.ReadTagsForFirstFile(files)
	t.Logf("\n=== Tag Data (first file with tags) ===")
	if tagData != nil {
		t.Logf("  Artist:       '%s'", tagData.Artist)
		t.Logf("  AlbumArtist:  '%s'", tagData.AlbumArtist)
		t.Logf("  Album:        '%s'", tagData.Album)
		t.Logf("  Title:        '%s'", tagData.Title)
		t.Logf("  Genre:        '%s'", tagData.Genre)
		t.Logf("  MusicBrainzID:'%s'", tagData.MusicBrainzID)
		t.Logf("  RuntimeMS:    %d (%.1fs)", tagData.RuntimeMS, float64(tagData.RuntimeMS)/1000)
		t.Logf("  Bitrate:      %d", tagData.Bitrate)
		t.Logf("  Year:         %d", tagData.Year)
	} else {
		t.Logf("  No tags found")
	}

	// Step 5: Read all tracks and show their individual runtimes
	t.Logf("\n=== All Track Runtimes ===")
	tracks := parser_v2.CollectTracksFromFiles(files)
	tracks = parser_v2.EnrichTracksWithTags(tracks)

	var totalRuntimeMS int64
	var missingRuntime int
	discRuntimes := make(map[int]int64)
	discCounts := make(map[int]int)

	for i, track := range tracks {
		totalRuntimeMS += track.RuntimeMS
		if track.RuntimeMS == 0 {
			missingRuntime++
		}
		discRuntimes[track.DiscNumber] += track.RuntimeMS
		discCounts[track.DiscNumber]++

		// Show first 5, last 2, and disc boundaries
		showTrack := i < 5 || i >= len(tracks)-2
		if !showTrack {
			// Also show disc boundaries
			if i > 0 && tracks[i].DiscNumber != tracks[i-1].DiscNumber {
				showTrack = true
			}
		}
		if showTrack {
			t.Logf(
				"  [%02d] disc=%d track=%02d runtime=%6dms (%4.1fs) artist='%s' title='%s' album='%s'",
				i+1,
				track.DiscNumber,
				track.TrackNumber,
				track.RuntimeMS,
				float64(track.RuntimeMS)/1000,
				truncateString(track.Artist, 25),
				truncateString(track.Title, 30),
				truncateString(track.Album, 25),
			)
		} else if i == 5 {
			t.Logf("  ... (%d more tracks) ...", len(tracks)-7)
		}
	}

	t.Logf("\n=== Runtime Summary ===")
	t.Logf("  Total tracks:    %d", len(tracks))
	t.Logf("  Missing runtime: %d", missingRuntime)
	t.Logf("  Total runtime:   %dms (%.1f minutes)", totalRuntimeMS, float64(totalRuntimeMS)/60000)
	for disc, runtime := range discRuntimes {
		t.Logf(
			"  Disc %d: %d tracks, %dms (%.1f min)",
			disc,
			discCounts[disc],
			runtime,
			float64(runtime)/60000,
		)
	}

	// Step 6: Show what search pairs would be generated
	t.Logf("\n=== Search Pairs That Would Be Generated ===")
	artist := coalesceStr(folderArtist, firstTrack.Artist)
	albumTitle := coalesceStr(folderAlbum, firstTrack.Album)
	if tagData != nil {
		artist = coalesceStr(folderArtist, firstTrack.Artist, tagData.AlbumArtist, tagData.Artist)
		albumTitle = coalesceStr(folderAlbum, firstTrack.Album, tagData.Album)
	}
	t.Logf("  Best artist:  '%s'", artist)
	t.Logf("  Best album:   '%s'", albumTitle)
	t.Logf("  File count:   %d", len(files))

	// Step 7: Show the runtime tolerance that would be used
	toleranceMs := int64(len(tracks)) * 3000
	t.Logf("\n=== Runtime Verification Settings ===")
	t.Logf("  Per-track tolerance: 3000ms")
	t.Logf("  Total tolerance:     %dms (%.1f seconds) for %d tracks",
		toleranceMs, float64(toleranceMs)/1000, len(tracks))
	t.Logf("  Local total runtime: %dms (%.1f min)", totalRuntimeMS, float64(totalRuntimeMS)/60000)

	// Step 8: Analyze per-track runtime pattern
	t.Logf("\n=== Runtime Distribution Analysis ===")
	var minRT, maxRT int64
	if len(tracks) > 0 {
		minRT = tracks[0].RuntimeMS
		maxRT = tracks[0].RuntimeMS
	}
	for _, track := range tracks {
		if track.RuntimeMS > 0 {
			if track.RuntimeMS < minRT || minRT == 0 {
				minRT = track.RuntimeMS
			}
			if track.RuntimeMS > maxRT {
				maxRT = track.RuntimeMS
			}
		}
	}
	t.Logf("  Min track runtime: %dms (%.1fs)", minRT, float64(minRT)/1000)
	t.Logf("  Max track runtime: %dms (%.1fs)", maxRT, float64(maxRT)/1000)
	t.Logf(
		"  Avg track runtime: %dms (%.1fs)",
		totalRuntimeMS/int64(len(tracks)),
		float64(totalRuntimeMS)/float64(len(tracks))/1000,
	)

	// Step 9: MusicBrainz search and per-track runtime comparison
	t.Logf("\n=== MusicBrainz Search & Track Comparison ===")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mbProvider := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   1,
		RateLimitSeconds: 1,
		UserAgent:        "GoMediaDownloader/1.0 (test)",
	})

	// Try multiple search queries to find the right release
	searchQueries := []struct {
		label string
		query string
	}{
		{
			"album+artist",
			fmt.Sprintf(`release:"%s" AND artist:"%s"`, "Kuschelrock 38", "Various Artists"),
		},
		{"album only", fmt.Sprintf(`release:"%s"`, "Kuschelrock 38")},
		{
			"album+tracks+status",
			fmt.Sprintf(
				`release:"%s" AND tracks:%d AND status:official`,
				"Kuschelrock 38",
				len(tracks),
			),
		},
	}

	var bestRelease *apiexternal_v2.ReleaseDetails
	var bestReleaseScore string

	for _, sq := range searchQueries {
		t.Logf("\n--- MusicBrainz search: %s ---", sq.label)
		t.Logf("  Query: %s", sq.query)

		results, _, err := mbProvider.SearchReleases(ctx, sq.query, 10, 0)
		if err != nil {
			t.Logf("  ERROR: %v", err)
			continue
		}
		t.Logf("  Found %d results", len(results))

		for i, r := range results {
			trackMatch := ""
			if r.TrackCount == len(tracks) {
				trackMatch = " <-- TRACK COUNT MATCH"
			}
			t.Logf(
				"  [%d] '%s' by %v | tracks=%d | year=%d | country=%s | format=%s | type=%s | id=%s%s",
				i+1,
				r.Title,
				r.Artists,
				r.TrackCount,
				r.ReleaseYear,
				r.Country,
				r.Format,
				r.Type,
				r.ID,
				trackMatch,
			)

			// If we found a release with matching track count and haven't picked one yet
			if bestRelease == nil && r.TrackCount == len(tracks) {
				t.Logf("  >>> Fetching full release details for ID: %s", r.ID)
				details, err := mbProvider.GetReleaseByID(ctx, r.ID)
				if err != nil {
					t.Logf("  ERROR getting release details: %v", err)
					continue
				}
				bestRelease = details
				bestReleaseScore = sq.label
				t.Logf("  >>> Got release: '%s' with %d tracks across %d disc(s)",
					details.Title, details.TrackCount, details.DiscCount)
			}
		}

		if bestRelease != nil {
			break // Found a match, no need to try more queries
		}

		time.Sleep(1100 * time.Millisecond) // MusicBrainz rate limit
	}

	if bestRelease == nil {
		t.Logf("\nNo MusicBrainz release found with matching track count (%d tracks)", len(tracks))
		t.Logf(
			"Try searching manually at: https://musicbrainz.org/search?query=Kuschelrock+38&type=release",
		)
		return
	}

	t.Logf("\n=== Best MusicBrainz Match (via %s) ===", bestReleaseScore)
	t.Logf("  Title:     %s", bestRelease.Title)
	t.Logf("  Artists:   %v", bestRelease.Artists)
	t.Logf("  ID:        %s", bestRelease.ID)
	t.Logf("  Year:      %d", bestRelease.ReleaseYear)
	t.Logf("  Discs:     %d", bestRelease.DiscCount)
	t.Logf("  Tracks:    %d", bestRelease.TrackCount)
	t.Logf("  Format:    %s", bestRelease.Format)
	t.Logf("  Country:   %s", bestRelease.Country)
	t.Logf("  Barcode:   %s", bestRelease.Barcode)

	// Step 10: Per-track runtime comparison
	t.Logf("\n=== Per-Track Runtime Comparison (MusicBrainz vs Local) ===")

	// Sort MusicBrainz tracks by disc+position
	mbTracks := bestRelease.Tracks
	sort.Slice(mbTracks, func(i, j int) bool {
		if mbTracks[i].DiscNumber != mbTracks[j].DiscNumber {
			return mbTracks[i].DiscNumber < mbTracks[j].DiscNumber
		}
		return mbTracks[i].Position < mbTracks[j].Position
	})

	// Sort local tracks by disc+track
	localSorted := make([]parser_v2.TrackInfo, len(tracks))
	copy(localSorted, tracks)
	sort.Slice(localSorted, func(i, j int) bool {
		if localSorted[i].DiscNumber != localSorted[j].DiscNumber {
			return localSorted[i].DiscNumber < localSorted[j].DiscNumber
		}
		return localSorted[i].TrackNumber < localSorted[j].TrackNumber
	})

	if len(mbTracks) != len(localSorted) {
		t.Logf(
			"WARNING: Track count mismatch! MusicBrainz=%d, Local=%d",
			len(mbTracks),
			len(localSorted),
		)
	}

	matchCount := 0
	mismatchCount := 0
	var totalDiffMS int64
	var maxDiffMS int64
	worstTrack := -1

	compareLen := min(len(localSorted), len(mbTracks))

	t.Logf("\n  %-4s %-4s %-35s %8s %8s %8s %s",
		"Disc", "Trk", "Title", "MB(ms)", "Local(ms)", "Diff(ms)", "Status")
	t.Logf(
		"  %s",
		"----  ---- ----------------------------------- -------- --------- --------- ------",
	)

	for i := range compareLen {
		mbTrack := mbTracks[i]
		localTrack := localSorted[i]

		mbMS := mbTrack.Duration.Milliseconds()
		localMS := localTrack.RuntimeMS
		diffMS := localMS - mbMS
		absDiff := diffMS
		if absDiff < 0 {
			absDiff = -absDiff
		}

		status := "OK"
		if absDiff > 5000 {
			status = "MISMATCH"
			mismatchCount++
		} else if absDiff > 3000 {
			status = "WARN"
			mismatchCount++
		} else {
			matchCount++
		}

		totalDiffMS += absDiff
		if absDiff > maxDiffMS {
			maxDiffMS = absDiff
			worstTrack = i
		}

		// Show all tracks for full visibility
		mbTitle := truncateString(mbTrack.Title, 33)
		t.Logf("  D%-3d %02d   %-35s %8d %9d %+9d %s",
			mbTrack.DiscNumber, mbTrack.Position, mbTitle,
			mbMS, localMS, diffMS, status)
	}

	// Step 11: Summary statistics
	t.Logf("\n=== Runtime Comparison Summary ===")
	t.Logf("  Tracks compared: %d", compareLen)
	t.Logf("  Matching (<=3s):  %d", matchCount)
	t.Logf("  Mismatching:      %d", mismatchCount)
	if compareLen > 0 {
		avgDiff := float64(totalDiffMS) / float64(compareLen)
		t.Logf("  Avg abs diff:     %.0fms (%.1fs)", avgDiff, avgDiff/1000)
	}
	t.Logf("  Max abs diff:     %dms (%.1fs)", maxDiffMS, float64(maxDiffMS)/1000)
	if worstTrack >= 0 {
		t.Logf(
			"  Worst track:      D%d #%d '%s'",
			mbTracks[worstTrack].DiscNumber,
			mbTracks[worstTrack].Position,
			mbTracks[worstTrack].Title,
		)
	}

	// Step 12: Test progressive runtime matching tolerances
	t.Logf("\n=== Progressive Runtime Tolerance Analysis ===")
	tolerances := []int64{1000, 2000, 3000, 4000, 5000}
	for _, tol := range tolerances {
		matched := 0
		for i := range compareLen {
			mbMS := mbTracks[i].Duration.Milliseconds()
			localMS := localSorted[i].RuntimeMS
			if int64(math.Abs(float64(localMS-mbMS))) <= tol {
				matched++
			}
		}
		pct := float64(matched) / float64(compareLen) * 100
		t.Logf("  Tolerance %dms: %d/%d matched (%.0f%%)", tol, matched, compareLen, pct)
	}

	// Step 13: Total runtime comparison
	var mbTotalMS int64
	for _, tr := range mbTracks {
		mbTotalMS += tr.Duration.Milliseconds()
	}
	runtimeDiff := totalRuntimeMS - mbTotalMS
	t.Logf("\n=== Total Runtime Comparison ===")
	t.Logf("  MusicBrainz total: %dms (%.1f min)", mbTotalMS, float64(mbTotalMS)/60000)
	t.Logf("  Local total:       %dms (%.1f min)", totalRuntimeMS, float64(totalRuntimeMS)/60000)
	t.Logf("  Difference:        %+dms (%.1fs)", runtimeDiff, float64(runtimeDiff)/1000)

	verifyTolerance := int64(len(tracks)) * 3000
	if runtimeDiff < 0 {
		runtimeDiff = -runtimeDiff
	}
	if runtimeDiff <= verifyTolerance {
		t.Logf("  Total runtime: PASS (within %dms tolerance)", verifyTolerance)
	} else {
		t.Logf(
			"  Total runtime: FAIL (exceeds %dms tolerance by %dms)",
			verifyTolerance,
			runtimeDiff-verifyTolerance,
		)
	}

	t.Logf("\n=== CONCLUSION ===")
	t.Logf("If this album fails matching, check:")
	t.Logf(
		"1. Does the database have a matching album with artist='%s' title='%s'?",
		artist,
		albumTitle,
	)
	t.Logf("2. Does that DB album have %d tracks?", len(files))
	t.Logf(
		"3. Does the DB album's total runtime match ~%dms (%.1f min)?",
		totalRuntimeMS,
		float64(totalRuntimeMS)/60000,
	)
	t.Logf("4. Do individual track runtimes match within 5s tolerance?")
	t.Logf(
		"5. For VA albums, track artists differ per file - is the Album artist 'Various Artists' or '%s'?",
		artist,
	)
	t.Logf("6. MusicBrainz release ID for direct import: %s", bestRelease.ID)
}

// TestAddFoundNoMBID tests that AddFound text search works for albums without a MusicBrainz ID
// in their file tags, specifically verifying that tagAlbum is preferred over the year-prefixed
// folder name for the MB search query.
//
// Run with: go test -v -run TestAddFoundNoMBID
func TestAddFoundNoMBID(t *testing.T) {
	folder := `Q:\_tocheck_Music\no_match\1983 At Capolinea`

	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil || len(files) == 0 {
		t.Skip("No audio files found - is the folder accessible?")
	}
	t.Logf("Found %d audio files", len(files))

	// Mimic album_processor.go metadata extraction
	folderArtist, folderAlbum, year := parser_v2.ParseAudioFolder(folder)
	t.Logf("folderArtist=%q folderAlbum=%q year=%d", folderArtist, folderAlbum, year)

	tagData := parser_v2.ReadTagsForFirstFile(files)
	var tagArtist, tagAlbum string
	if tagData != nil {
		tagArtist = tagData.Artist
		tagAlbum = tagData.Album
		t.Logf("tagArtist=%q tagAlbum=%q", tagArtist, tagAlbum)
	}

	// Replicate coalesceStr priority used in album_processor.go
	artist := coalesceStr(folderArtist, tagArtist)
	albumTitle := coalesceStr(folderAlbum, tagAlbum) // folder first — may have year prefix

	// searchTitle selection (the fix)
	searchTitle := albumTitle
	if tagAlbum != "" {
		searchTitle = tagAlbum
	}

	t.Logf("\nalbumTitle (for DB search): %q", albumTitle)
	t.Logf("searchTitle (for MB search): %q", searchTitle)

	tracks := parser_v2.CollectTracksFromFiles(files)
	tracks = parser_v2.EnrichTracksWithTags(tracks)
	fileCount := len(tracks)
	t.Logf("fileCount: %d", fileCount)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mbProvider := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   1,
		RateLimitSeconds: 1,
		UserAgent:        "GoMediaDownloader/1.0 (test)",
	})
	providers.SetMusicBrainz(mbProvider)

	// Split album title into individual keyword tokens (no space issues)
	titleWords := strings.Fields(searchTitle)
	var releaseTokens []string
	for _, w := range titleWords {
		releaseTokens = append(releaseTokens, "release:"+w)
	}
	titleTokenQuery := fmt.Sprintf(`artist:"%s" AND %s`, artist, strings.Join(releaseTokens, " AND "))

	// Best working query: unquoted release tokens, no artist field
	bestQuery := fmt.Sprintf(`release:%s`, searchTitle)
	t.Logf("\n--- Best query: %s ---", bestQuery)
	results, _, serr := mbProvider.SearchReleases(ctx, bestQuery, 20, 0)
	if serr != nil || len(results) == 0 {
		t.Fatalf("No results for best query: %v", serr)
	}
	for i, r := range results {
		match := ""
		if r.TrackCount == fileCount {
			match = " <-- TRACK COUNT MATCH"
		}
		t.Logf("  [%d] %q artists=%v | tracks=%d | year=%d | id=%s%s",
			i+1, r.Title, r.Artists, r.TrackCount, r.ReleaseYear, r.MusicBrainzID, match)
	}

	// Fetch full details for all track-count-matching candidates
	t.Logf("\n--- Full release details for track-count matches ---")
	for _, r := range results {
		if r.TrackCount != fileCount {
			continue
		}
		details, err := mbProvider.GetReleaseByID(ctx, r.MusicBrainzID)
		if err != nil || details == nil {
			t.Logf("  [%s] fetch error: %v", r.MusicBrainzID, err)
			continue
		}
		t.Logf("  Release: %q | artist=%v | tracks=%d | id=%s",
			details.Title, details.Artists, details.TrackCount, details.ID)
		var totalMs int64
		for _, tr := range details.Tracks {
			totalMs += tr.Duration.Milliseconds()
		}
		t.Logf("    Total runtime: %dms (%.1fs)", totalMs, float64(totalMs)/1000)
	}

	_ = titleTokenQuery // used above for exploration only
}

// TestGoodreadsFallback tests the Goodreads fallback path
func TestGoodreadsFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Note: Goodreads API requires an API key and was deprecated in December 2020
	// This test requires a valid API key to work
	apiKey := "" // Set your Goodreads API key here for testing, or skip this test

	if apiKey == "" {
		t.Skip("Skipping Goodreads test - no API key configured")
		return
	}

	grProvider := goodreads.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
	}, apiKey)

	providers.SetGoodreads(grProvider)

	t.Log("=== Testing Goodreads (if API key configured) ===")

	books, err := grProvider.SearchBooks(ctx, "Dan Brown", 1)
	if err != nil {
		t.Logf("Goodreads search failed: %v", err)
		return
	}

	t.Logf("Found %d books from Goodreads", len(books))
	for i, b := range books {
		if i >= 3 {
			break
		}
		t.Logf("  %d: %s by %v", i+1, b.Title, b.Authors)
	}
}
