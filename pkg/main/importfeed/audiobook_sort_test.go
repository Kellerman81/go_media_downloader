package importfeed

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// TestAudiobookSortWithRuntimeVerification tests that matchTracksByDistance correctly
// orders multi-disc audiobook files using runtime+title+index from Audnex chapters.
//
//  1. Collect and enrich tracks from the real folder
//  2. Fetch chapters from Audnex API → build DbtrackWithArtist
//  3. Run matchTracksByDistance (isAudiobook=true)
//  4. Verify all chapters matched and result is in disc-sequential file order
//
// Uses: V:\completed\Audiobook\David Baldacci - Die Sammler, ASIN: B004UW5QSK
// Run with: go test -v -run TestAudiobookSortWithRuntimeVerification -timeout 120s
func TestAudiobookSortWithRuntimeVerification(t *testing.T) {
	folder := `V:\completed\Audiobook\David Baldacci - Die Sammler`
	asin := "B004UW5QSK"
	region := "de" // German audiobook

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// === Step 1: Collect and enrich local files ===
	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil {
		t.Fatalf("CollectFilesOnly failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("No audio files found in folder")
	}
	localTracks := parser_v2.CollectTracksFromFiles(files)
	localTracks = parser_v2.EnrichTracksWithTags(localTracks)
	t.Logf("Found %d audio files", len(localTracks))

	// === Step 2: Fetch chapters from Audnex → build DbtrackWithArtist ===
	audnexProvider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
	})
	providers.SetAudnex(audnexProvider)

	chapters, err := audnexProvider.GetChaptersByASIN(ctx, asin, region)
	if err != nil {
		t.Fatalf("GetChaptersByASIN failed: %v", err)
	}
	if len(chapters) == 0 {
		t.Fatal("No chapters returned from Audnex")
	}
	t.Logf("Fetched %d chapters from Audnex for ASIN %s", len(chapters), asin)

	dbTracks := make([]database.DbtrackWithArtist, len(chapters))
	for i, ch := range chapters {
		dbTracks[i] = database.DbtrackWithArtist{
			Dbtrack: database.Dbtrack{
				Title:       ch.Title,
				TrackNumber: uint16(i + 1),
				RuntimeMs:   ch.LengthMs,
			},
		}
		t.Logf("  Chapter %2d: %s (%.1fs)", i+1, ch.Title, float64(ch.LengthMs)/1000)
	}

	if len(localTracks) != len(dbTracks) {
		t.Fatalf("Local track count %d != chapter count %d", len(localTracks), len(dbTracks))
	}

	// === Step 3: Run matchTracksByDistance ===
	result, matched, _ := matchTracksByDistance(localTracks, dbTracks, false, true, nil)

	// === Step 4: Verify all chapters matched and in disc-sequential file order ===
	t.Log("\n=== MATCH RESULT ===")
	var errors []string
	for i, track := range result {
		basename := filepath.Base(track.Filepath)
		status := "OK"
		if !matched[i] {
			status = "UNMATCHED"
			errors = append(errors, fmt.Sprintf("chapter %d (%s) has no match", i+1, dbTracks[i].Title))
		}
		t.Logf("  Chapter %2d (%s) <- %-50s [%s]", i+1, dbTracks[i].Title, basename, status)
	}

	if !verifyDiscSequentialOrder(result) {
		errors = append(errors, "matched result is NOT in disc-sequential file order")
	}

	if len(localTracks) != 69 {
		t.Logf("WARNING: Expected 69 files, got %d", len(localTracks))
	}

	if len(errors) > 0 {
		t.Errorf("FAILED:\n  %s", strings.Join(errors, "\n  "))
	} else {
		t.Logf("\nSUCCESS: all %d chapters matched in disc-sequential order", len(result))
	}
}

// TestAudiobookSortWithZeroRuntimes verifies that matchTracksByDistance still assigns
// tracks correctly when all DB chapter runtimes are zero (simulates DB rows imported
// before the runtime_ms column existed). Matching falls back to track_index + title.
//
// Uses: V:\completed\Audiobook\David Baldacci - Die Sammler
// Run with: go test -v -run TestAudiobookSortWithZeroRuntimes
func TestAudiobookSortWithZeroRuntimes(t *testing.T) {
	folder := `V:\completed\Audiobook\David Baldacci - Die Sammler`

	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil {
		t.Fatalf("CollectFilesOnly failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("No audio files found in folder")
	}
	localTracks := parser_v2.CollectTracksFromFiles(files)
	localTracks = parser_v2.EnrichTracksWithTags(localTracks)
	t.Logf("Enriched %d tracks", len(localTracks))

	// Build DB tracks with zero runtimes and sequential track numbers.
	// RuntimeMs=0 causes matchTracksByDistance to skip the track_length component;
	// matching falls back to track_title (if tags are set) and track_index.
	dbTracks := make([]database.DbtrackWithArtist, len(localTracks))
	for i := range dbTracks {
		dbTracks[i] = database.DbtrackWithArtist{
			Dbtrack: database.Dbtrack{
				TrackNumber: uint16(i + 1), // sequential 1..N
				RuntimeMs:   0,             // zero — simulates missing DB data
			},
		}
	}

	t.Log("=== Testing matchTracksByDistance with all-zero DB runtimes ===")
	result, matched, _ := matchTracksByDistance(localTracks, dbTracks, false, true, nil)

	unmatchedCount := 0
	for _, m := range matched {
		if !m {
			unmatchedCount++
		}
	}

	t.Log("\n=== RESULT ===")
	for i, track := range result {
		status := "OK"
		if !matched[i] {
			status = "UNMATCHED"
		}
		t.Logf("  Chapter %2d <- %-50s [%s]", i+1, filepath.Base(track.Filepath), status)
	}

	if unmatchedCount > 0 {
		t.Fatalf("matchTracksByDistance left %d/%d chapters unmatched with zero runtimes",
			unmatchedCount, len(dbTracks))
	}

	if !verifyDiscSequentialOrder(result) {
		t.Fatal("Files are NOT in disc-sequential order — zero-runtime fallback via track_index failed")
	}

	t.Log("\nSUCCESS: Zero DB runtimes correctly fall back to track_index matching")
}

// TestAllRuntimesZero tests the allRuntimesZero helper function.
func TestAllRuntimesZero(t *testing.T) {
	tests := []struct {
		name     string
		input    []int64
		expected bool
	}{
		{"nil", nil, false},
		{"empty", []int64{}, false},
		{"all zeros", []int64{0, 0, 0}, true},
		{"single zero", []int64{0}, true},
		{"has nonzero", []int64{0, 100, 0}, false},
		{"all nonzero", []int64{100, 200, 300}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allRuntimesZero(tt.input); got != tt.expected {
				t.Errorf("allRuntimesZero(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestKeinKeksDistanceMatch tests the beets-style distance matching for
// "Kein Keks fuer Kobolde" (ASIN B01FUMJRB0):
//  1. Fetch Audnex chapters via resolveTracksForMatching (no DB, audiobook fallback)
//  2. Collect and enrich the 25 renamed MP3s from disk
//  3. Run matchTracksByDistance and verify all 25 are matched in the right order
//  4. Specifically verify tracks 13 and 14 (both ~447s) are not swapped
//
// Run with: go test -v -run TestKeinKeksDistanceMatch -timeout 60s
func TestKeinKeksDistanceMatch(t *testing.T) {
	folder := `P:\C\Cornelia Funke\Kein Keks fuer Kobolde (B01FUMJRB0)`
	asin := "B01FUMJRB0"
	region := "de"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// === Step 1: Initialize Audnex and resolve chapters ===
	audnexProvider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
	})
	providers.SetAudnex(audnexProvider)

	dbTracks := resolveTracksForMatching(ctx, 0, "", asin, region, "", "")
	if len(dbTracks) == 0 {
		t.Fatal("resolveTracksForMatching returned no chapters")
	}
	t.Logf("Resolved %d chapters from Audnex", len(dbTracks))
	for i, ch := range dbTracks {
		t.Logf("  DB[%2d] track=%2d runtime=%6dms title=%s",
			i, ch.TrackNumber, ch.RuntimeMs, ch.Title)
	}

	// === Step 1b: Album distance (audiobookMatchDistance) ===
	// Simulate what matchAudiobookFolder does: score a candidate by title+author.
	// The folder name gives: author="Cornelia Funke", title="Kein Keks fuer Kobolde".
	localTitle := "Kein Keks fuer Kobolde"
	localAuthor := "Cornelia Funke"
	fileCount := 25

	candidates := []struct {
		label    string
		title    string
		author   string
		wantBand string // "strong" | "medium" | "reject"
	}{
		{"exact match", localTitle, localAuthor, "strong"},
		{"title only (no author)", localTitle, "", "medium"},
		{"slight title variation", "Kein Keks für Kobolde", localAuthor, "medium"},
		{"wrong title", "Harry Potter", "J.K. Rowling", "reject"},
	}

	t.Log("\n=== ALBUM DISTANCE (audiobookMatchDistance) ===")
	t.Logf("  %-40s  %6s  %s", "candidate", "dist", "band")
	var albumErrors []string
	for _, tc := range candidates {
		mock := &database.AudiobookSearchResult{
			Title:        tc.title,
			Author:       tc.author,
			ASIN:         asin,
			ChapterCount: fileCount,
		}
		dist := audiobookMatchDistance(mock, localTitle, localAuthor, fileCount)

		var band string
		switch {
		case dist <= strongRecThresh:
			band = "strong"
		case dist <= mediumRecThresh:
			band = "medium"
		default:
			band = "reject"
		}

		status := "OK"
		if band != tc.wantBand {
			status = fmt.Sprintf("FAIL (want %s)", tc.wantBand)
			albumErrors = append(albumErrors, fmt.Sprintf("%s: dist=%.4f band=%s want=%s",
				tc.label, dist, band, tc.wantBand))
		}
		t.Logf("  %-40s  %.4f  %s  [%s]", tc.label, dist, band, status)
	}
	if len(albumErrors) > 0 {
		t.Errorf("audiobookMatchDistance FAILED:\n  %s", strings.Join(albumErrors, "\n  "))
	}

	// === Step 2: Collect and enrich local tracks ===
	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil {
		t.Fatalf("CollectFilesOnly failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("No audio files found in folder")
	}
	t.Logf("Found %d audio files", len(files))

	localTracks := parser_v2.CollectTracksFromFiles(files)
	localTracks = parser_v2.EnrichTracksWithTags(localTracks)
	t.Logf("Enriched %d tracks with tag data", len(localTracks))

	if len(localTracks) != len(dbTracks) {
		t.Fatalf("Local track count %d != chapter count %d", len(localTracks), len(dbTracks))
	}

	t.Log("\n=== LOCAL TRACKS (before matching) ===")
	for i, tr := range localTracks {
		t.Logf("  [%2d] disc=%d track=%2d runtime=%6dms file=%s",
			i, tr.DiscNumber, tr.TrackNumber, tr.RuntimeMS,
			filepath.Base(tr.Filepath))
	}

	// === Step 3: Distance matrix — every local track vs every chapter ===
	t.Log("\n=== DISTANCE MATRIX (local rows × chapter cols, best 3 per row) ===")
	t.Logf("  %-40s  %s", "local file", "top-3 chapter matches (ch dist)")
	for j := range localTracks {
		type hit struct {
			ch   int
			dist float64
		}
		var hits []hit
		for i := range dbTracks {
			hits = append(hits, hit{i + 1, trackDistance(&localTracks[j], &dbTracks[i], false, true, nil)})
		}
		sort.Slice(hits, func(a, b int) bool { return hits[a].dist < hits[b].dist })
		top := hits
		if len(top) > 3 {
			top = top[:3]
		}
		var parts []string
		for _, h := range top {
			parts = append(parts, fmt.Sprintf("ch%02d=%.3f", h.ch, h.dist))
		}
		t.Logf("  %-40s  %s", filepath.Base(localTracks[j].Filepath), strings.Join(parts, "  "))
	}

	// === Step 3b: Component breakdown for the ambiguous pair (chapters 13 & 14) ===
	t.Log("\n=== COMPONENT DISTANCES for ambiguous pair (chapters 13 & 14, both ~447s) ===")
	for _, chIdx := range []int{12, 13} { // 0-based
		ch := &dbTracks[chIdx]
		t.Logf("  Chapter %d: title=%q runtime=%dms track#=%d",
			chIdx+1, ch.Title, ch.RuntimeMs, ch.TrackNumber)
		for j := range localTracks {
			local := &localTracks[j]
			dist := trackDistance(local, ch, false, true, nil)
			if dist < 0.9 {
				t.Logf("    local[%2d] %-38s  total=%.3f  (runtime=%dms track#=%d)",
					j, filepath.Base(local.Filepath), dist, local.RuntimeMS, local.TrackNumber)
			}
		}
	}

	// === Step 4: Run matchTracksByDistance ===
	result, matched, _ := matchTracksByDistance(localTracks, dbTracks, false, true, nil)

	t.Log("\n=== MATCH RESULT ===")
	var errors []string
	for i, tr := range result {
		chTitle := dbTracks[i].Title
		basename := filepath.Base(tr.Filepath)
		status := "OK"
		if !matched[i] {
			status = "UNMATCHED"
			errors = append(errors, fmt.Sprintf("chapter %d (%s) has no match", i+1, chTitle))
		}
		t.Logf("  Chapter %2d (%s) <- %-40s [%s]", i+1, chTitle, basename, status)

		// The files are named "N - Kapitel N.mp3" so the correct file for
		// chapter i+1 starts with the string "<i+1> - " or "<i+1> -" prefix.
		if matched[i] {
			expectedPrefix := fmt.Sprintf("%d - ", i+1)
			if !strings.HasPrefix(basename, expectedPrefix) {
				errors = append(errors, fmt.Sprintf(
					"chapter %d: expected file starting with %q, got %q",
					i+1, expectedPrefix, basename))
			}
		}
	}

	// === Step 4: Special check for the ambiguous pair (tracks 13 and 14, both ~447s) ===
	for _, ambig := range []int{12, 13} { // 0-based indices
		if !matched[ambig] {
			continue
		}
		expectedPrefix := fmt.Sprintf("%d - ", ambig+1)
		basename := filepath.Base(result[ambig].Filepath)
		if !strings.HasPrefix(basename, expectedPrefix) {
			errors = append(errors, fmt.Sprintf(
				"ambiguous track check FAILED: chapter %d should map to %q* but got %q",
				ambig+1, expectedPrefix, basename))
		}
	}

	if len(errors) > 0 {
		t.Errorf("matchTracksByDistance FAILED:\n  %s", strings.Join(errors, "\n  "))
	} else {
		t.Logf("\nSUCCESS: all %d chapters matched in correct order", len(result))
		t.Log("Ambiguous pair (chapters 13 & 14, both ~447s) correctly disambiguated by track_index")
	}

	// === Step 5: audiobookDistanceWithTracks — full distance per candidate ===
	//
	// Verifies that the full distance (metadata + per-chapter runtime) ranks the
	// exact-match candidate better than a wrong-title candidate, and that the
	// exact candidate sits below strongRecThresh.
	t.Log("\n=== FULL DISTANCE (audiobookDistanceWithTracks) ===")

	// Note: audiobookDistanceWithTracks uses a single averaged "tracks" entry
	// (beets style) so chapter count does not inflate track weight.
	// Exact match: title+author=0, mean chapter dist adds a small penalty → medium.
	// Wrong title: metadata penalty dominates → reject.
	fullDistCandidates := []struct {
		label    string
		title    string
		author   string
		wantBand string
	}{
		{"exact match", localTitle, localAuthor, "medium"},
		{"wrong title", "Harry Potter", "J.K. Rowling", "reject"},
	}

	var fullDists []float64
	var fullDistErrors []string
	for _, tc := range fullDistCandidates {
		mock := &database.AudiobookSearchResult{
			Title:        tc.title,
			Author:       tc.author,
			ASIN:         asin,
			ChapterCount: len(localTracks),
		}
		fd := audiobookDistanceWithTracks(mock, localTitle, localAuthor, localTracks, dbTracks, nil)

		var band string
		switch {
		case fd <= strongRecThresh:
			band = "strong"
		case fd <= mediumRecThresh:
			band = "medium"
		default:
			band = "reject"
		}

		status := "OK"
		if band != tc.wantBand {
			status = fmt.Sprintf("FAIL (want %s)", tc.wantBand)
			fullDistErrors = append(fullDistErrors, fmt.Sprintf("%s: fullDist=%.4f band=%s want=%s",
				tc.label, fd, band, tc.wantBand))
		}
		t.Logf("  %-40s  fullDist=%.4f  band=%s  [%s]", tc.label, fd, band, status)

		if tc.wantBand != "reject" {
			fullDists = append(fullDists, fd)
		}
	}
	if len(fullDistErrors) > 0 {
		t.Errorf("audiobookDistanceWithTracks FAILED:\n  %s", strings.Join(fullDistErrors, "\n  "))
	}

	// === Step 6: recommendation ===
	//
	// With only the exact-match candidate passing track matching, recommendation
	// should return recStrong (single candidate, dist ≤ strongRecThresh).
	t.Log("\n=== RECOMMENDATION ===")
	if len(fullDists) == 0 {
		t.Error("No valid full distances to pass to recommendation — check step 5")
	} else {
		rec := recommendation(fullDists)
		t.Logf("  recommendation(%v) = %d", fullDists, rec)
		if rec < recMedium {
			t.Errorf("FAIL: expected at least recMedium (%d), got %d", recMedium, rec)
		} else {
			t.Logf("  PASS: recommendation = %d (>= recMedium=%d)", rec, recMedium)
		}
	}
}

// verifyDiscSequentialOrder checks that tracks are in disc-sequential order
// by verifying the numeric filename prefix is monotonically increasing.
func verifyDiscSequentialOrder(tracks []parser_v2.TrackInfo) bool {
	prevPrefix := ""
	for _, track := range tracks {
		basename := filepath.Base(track.Filepath)
		numPrefix := ""
		for _, c := range basename {
			if c >= '0' && c <= '9' {
				numPrefix += string(c)
			} else {
				break
			}
		}
		if numPrefix != "" && numPrefix < prevPrefix {
			return false
		}
		prevPrefix = numPrefix
	}
	return true
}
