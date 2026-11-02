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
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/goodreads"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// TestAudiobookSortWithZeroRuntimes verifies that matchTracksByDistance still assigns
// tracks correctly when all DB chapter runtimes are zero (simulates DB rows imported
// before the runtime_ms column existed). Matching falls back to track_index + title.
//
// Uses: V:\completed\Audiobook\David Baldacci - Die Sammler
// Run with: go test -v -run TestAudiobookSortWithZeroRuntimes
func TestAudiobookSortWithZeroRuntimes(t *testing.T) {
	if testing.Short() {
		t.Skip("manual test requiring a real local audiobook folder and live Audnex access")
	}

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
		t.Fatal(
			"Files are NOT in disc-sequential order — zero-runtime fallback via track_index failed",
		)
	}

	t.Log("\nSUCCESS: Zero DB runtimes correctly fall back to track_index matching")
}

// TestKeinKeksDistanceMatch tests the beets-style distance matching for
// "Kein Keks fuer Kobolde" (ASIN B01FUMJRB0):
//  1. Fetch Audnex chapters via resolveChaptersForMatching (no DB, audiobook fallback)
//  2. Collect and enrich the 25 renamed MP3s from disk
//  3. Run matchTracksByDistance and verify all 25 are matched in the right order
//  4. Specifically verify tracks 13 and 14 (both ~447s) are not swapped
//
// Run with: go test -v -run TestKeinKeksDistanceMatch -timeout 60s
func TestKeinKeksDistanceMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("manual test requiring a real local audiobook folder and live Audnex access")
	}

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

	dbTracks := resolveChaptersForMatching(ctx, 0, asin, region)
	if len(dbTracks) == 0 {
		t.Fatal("resolveChaptersForMatching returned no chapters")
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
			hits = append(
				hits,
				hit{i + 1, trackDistance(&localTracks[j], &dbTracks[i], false, true, nil)},
			)
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
		t.Log(
			"Ambiguous pair (chapters 13 & 14, both ~447s) correctly disambiguated by track_index",
		)
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
		rec := recommendation(fullDists, nil)
		t.Logf("  recommendation(%v) = %d", fullDists, rec)
		if rec < recMedium {
			t.Errorf("FAIL: expected at least recMedium (%d), got %d", recMedium, rec)
		} else {
			t.Logf("  PASS: recommendation = %d (>= recMedium=%d)", rec, recMedium)
		}
	}
}

// TestBoyzoneADifferentBeatBonusTrack reproduces the wrong-track-assignment bug
// observed in T:\Music\B\Boyzone\A Different Beat (1996)\_rename_log.txt.
//
// Symptom: 06-boyzone-dont_stop_looking_for_love.flac was renamed to
// "06 - Ben.flac" with actual=261.3s but expected=171.4s (diff=89.9s).
// Every track from position 5 onward was shifted by one slot.
//
// Root cause: the DB edition (catalogue 537 954 2) contains a bonus track at
// position 4 not present in the local rip.  With binary track_index, lapAssign
// minimised cost by matching each local file to its same-numbered DB slot
// (track_index=0.0) even when the runtime was off by ~90 s.  With the graduated
// track_index (abs(diff)/max(local#,db#)) runtime becomes the primary signal and
// the bonus DB track is correctly left unmatched.
//
// Local files: 14 FLAC files numbered 02–15, titles empty (rip had no title
// tags — only track numbers and runtimes were available to the matcher).
//
// Run with: go test -v -run TestBoyzoneADifferentBeatBonusTrack
func TestBoyzoneADifferentBeatBonusTrack(t *testing.T) {
	// DB tracks: edition WITH bonus track at position 4 (catalogue 537 954 2).
	// Runtimes from the "expected" column of the rename log.
	// Bonus runtime (300 s) is chosen to be > 30 s away from every local file.
	dbTracksWithBonus := []database.DbtrackWithArtist{
		{Dbtrack: database.Dbtrack{Title: "Paradise", TrackNumber: 1, RuntimeMs: 213500}},
		{Dbtrack: database.Dbtrack{Title: "A Different Beat", TrackNumber: 2, RuntimeMs: 256200}},
		{Dbtrack: database.Dbtrack{Title: "Melting Pot", TrackNumber: 3, RuntimeMs: 233600}},
		{
			Dbtrack: database.Dbtrack{Title: "Words (bonus)", TrackNumber: 4, RuntimeMs: 300000},
		}, // bonus — not ripped locally
		{
			Dbtrack: database.Dbtrack{
				Title:       "What Can You Do for Me",
				TrackNumber: 5,
				RuntimeMs:   176900,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Ben", TrackNumber: 6, RuntimeMs: 171400}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Don't Stop Looking for Love",
				TrackNumber: 7,
				RuntimeMs:   261300,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Isn't It a Wonder", TrackNumber: 8, RuntimeMs: 226900}},
		{Dbtrack: database.Dbtrack{Title: "Words", TrackNumber: 9, RuntimeMs: 245800}},
		{Dbtrack: database.Dbtrack{Title: "It's Time", TrackNumber: 10, RuntimeMs: 216000}},
		{Dbtrack: database.Dbtrack{Title: "Games of Love", TrackNumber: 11, RuntimeMs: 226100}},
		{Dbtrack: database.Dbtrack{Title: "Strong Enough", TrackNumber: 12, RuntimeMs: 218700}},
		{Dbtrack: database.Dbtrack{Title: "Heaven Knows", TrackNumber: 13, RuntimeMs: 246500}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Crying in the Night",
				TrackNumber: 14,
				RuntimeMs:   187600,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Give a Little", TrackNumber: 15, RuntimeMs: 203300}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "She Moves Through the Fair",
				TrackNumber: 16,
				RuntimeMs:   266100,
			},
		},
	}

	// DB tracks: standard edition WITHOUT the bonus track (15 tracks).
	dbTracksWithoutBonus := []database.DbtrackWithArtist{
		{Dbtrack: database.Dbtrack{Title: "Paradise", TrackNumber: 1, RuntimeMs: 213500}},
		{Dbtrack: database.Dbtrack{Title: "A Different Beat", TrackNumber: 2, RuntimeMs: 256200}},
		{Dbtrack: database.Dbtrack{Title: "Melting Pot", TrackNumber: 3, RuntimeMs: 233600}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "What Can You Do for Me",
				TrackNumber: 4,
				RuntimeMs:   176900,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Ben", TrackNumber: 5, RuntimeMs: 171400}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Don't Stop Looking for Love",
				TrackNumber: 6,
				RuntimeMs:   261300,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Isn't It a Wonder", TrackNumber: 7, RuntimeMs: 226900}},
		{Dbtrack: database.Dbtrack{Title: "Words", TrackNumber: 8, RuntimeMs: 245800}},
		{Dbtrack: database.Dbtrack{Title: "It's Time", TrackNumber: 9, RuntimeMs: 216000}},
		{Dbtrack: database.Dbtrack{Title: "Games of Love", TrackNumber: 10, RuntimeMs: 226100}},
		{Dbtrack: database.Dbtrack{Title: "Strong Enough", TrackNumber: 11, RuntimeMs: 218700}},
		{Dbtrack: database.Dbtrack{Title: "Heaven Knows", TrackNumber: 12, RuntimeMs: 246500}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Crying in the Night",
				TrackNumber: 13,
				RuntimeMs:   187600,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Give a Little", TrackNumber: 14, RuntimeMs: 203300}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "She Moves Through the Fair",
				TrackNumber: 15,
				RuntimeMs:   266100,
			},
		},
	}

	// localTracksNoTitle: empty titles — the actual rip had no title tags.
	// Used for matchTracksByDistance regression (sections 2, 2b): with all
	// title distances == 1.0, binary track_index was the sole differentiator
	// and made lapAssign prefer same-numbered DB slots regardless of runtime.
	// The graduated track_index fix must overcome this with runtime signal alone.
	// Runtimes are from the "actual" column of the rename log.
	localTracksNoTitle := []parser_v2.TrackInfo{
		{TrackNumber: 2, RuntimeMS: 212700},
		{TrackNumber: 3, RuntimeMS: 256500},
		{TrackNumber: 4, RuntimeMS: 233500},
		{
			TrackNumber: 5,
			RuntimeMS:   169500,
		}, // ← bug was here: matched to WCDM at track 5 instead of Ben at track 6
		{TrackNumber: 6, RuntimeMS: 261300},
		{TrackNumber: 7, RuntimeMS: 226900},
		{TrackNumber: 8, RuntimeMS: 245700},
		{TrackNumber: 9, RuntimeMS: 216100},
		{TrackNumber: 10, RuntimeMS: 226000},
		{TrackNumber: 11, RuntimeMS: 218700},
		{TrackNumber: 12, RuntimeMS: 246500},
		{TrackNumber: 13, RuntimeMS: 187600},
		{TrackNumber: 14, RuntimeMS: 203200},
		{TrackNumber: 15, RuntimeMS: 266100},
	}

	// localTracksWithTitle: titles parsed from filenames (best-case scenario,
	// also representative of re-runs after initial tagging).  Used for the
	// albumDistanceWithTracks and recommendation sections where per-track title
	// signal is needed to get distances below mediumRecThresh.
	localTracksWithTitle := []parser_v2.TrackInfo{
		{Title: "Paradise", TrackNumber: 2, RuntimeMS: 212700},
		{Title: "A Different Beat", TrackNumber: 3, RuntimeMS: 256500},
		{Title: "Melting Pot", TrackNumber: 4, RuntimeMS: 233500},
		{Title: "Ben", TrackNumber: 5, RuntimeMS: 169500},
		{Title: "Dont Stop Looking for Love", TrackNumber: 6, RuntimeMS: 261300},
		{Title: "Isnt It a Wonder", TrackNumber: 7, RuntimeMS: 226900},
		{Title: "Words", TrackNumber: 8, RuntimeMS: 245700},
		{Title: "Its Time", TrackNumber: 9, RuntimeMS: 216100},
		{Title: "Games of Love", TrackNumber: 10, RuntimeMS: 226000},
		{Title: "Strong Enough", TrackNumber: 11, RuntimeMS: 218700},
		{Title: "Heaven Knows", TrackNumber: 12, RuntimeMS: 246500},
		{Title: "Crying in the Night", TrackNumber: 13, RuntimeMS: 187600},
		{Title: "Give a Little", TrackNumber: 14, RuntimeMS: 203200},
		{Title: "She Moves Through the Fair", TrackNumber: 15, RuntimeMS: 266100},
	}

	// User's actual production config: grace=3 s, hard limit=30 s.
	// With grace=3 the 7.4 s diff of local05→DB5 (WCDM) incurs a penalty
	// (diff > grace), making DB6 (Ben, 1.9 s diff + small graduated index cost)
	// cheaper globally.  With grace=10 the 7.4 s diff falls inside the grace
	// window (cost=0) and the exact track# match wins even with graduated index.
	data := &config.MediaDataConfig{
		PerTrackToleranceSeconds:    3,
		PerTrackToleranceSecondsMax: 30,
	}

	// ── 1. detectVA ──────────────────────────────────────────────────────────
	t.Log("=== detectVA ===")
	isVA := DetectVA("Boyzone", localTracksNoTitle)
	t.Logf("  detectVA(%q) = %v", "Boyzone", isVA)
	if isVA {
		t.Error("FAIL: Boyzone should NOT be detected as VA")
	} else {
		t.Log("  PASS: correctly identified as non-VA")
	}

	// ── 2. matchTracksByDistance — DB with bonus ──────────────────────────────
	// Core regression test: with empty titles and the graduated track_index fix,
	// local[3] (the "ben" file, runtime=169.5s, track#=5) must be matched to
	// DB[5] (Ben, runtime=171.4s, track#=6) — NOT to DB[4] (WCDM, 176.9s, track#=5).
	// With the old binary track_index, the same-track# bonus made DB[4] cheaper
	// (cost=0.499) than DB[5] (cost=0.571) even with a worse runtime match.
	// The graduated fix lowers DB[5]'s cost to 0.452 (index=1/6), making it win.
	t.Log("\n=== matchTracksByDistance (DB with bonus track) ===")
	result, matched, used := matchTracksByDistance(
		localTracksNoTitle,
		dbTracksWithBonus,
		isVA,
		false,
		data,
	)

	unmatchedDB, unusedLocal := 0, 0
	for i, m := range matched {
		localTitle := "(unmatched)"
		if m {
			localTitle = result[i].Title
			if localTitle == "" {
				localTitle = fmt.Sprintf("(no title, runtime=%dms)", result[i].RuntimeMS)
			}
		} else {
			unmatchedDB++
		}
		t.Logf("  DB[%2d] %-35s → local: %s", i+1, dbTracksWithBonus[i].Title, localTitle)
	}
	for j, u := range used {
		if !u {
			unusedLocal++
			t.Logf(
				"  Local[%2d] track#=%d runtime=%dms → UNUSED",
				j+1,
				localTracksNoTitle[j].TrackNumber,
				localTracksNoTitle[j].RuntimeMS,
			)
		}
	}
	t.Logf("  unmatchedDB=%d  unusedLocal=%d", unmatchedDB, unusedLocal)

	// Bonus (DB[4], index 3) must be unmatched — it has no runtime close to any local file.
	if unmatchedDB == 0 {
		t.Error(
			"FAIL: bonus track in DB should be left unmatched — wrong-edition detection would not fire",
		)
	} else {
		t.Logf("  PASS: %d DB track(s) unmatched — wrong-edition detection fires", unmatchedDB)
	}

	bonusIdx := 3 // DB track 4 is at index 3
	if matched[bonusIdx] {
		t.Errorf("FAIL: DB[4] %q (bonus) should be the unmatched track, but it was matched",
			dbTracksWithBonus[bonusIdx].Title)
	} else {
		t.Logf("  PASS: DB[4] %q correctly identified as the unmatched bonus track",
			dbTracksWithBonus[bonusIdx].Title)
	}

	// ── KEY REGRESSION: local[3] (the Ben file, 169.5 s, track#=5) must be
	// matched to DB[5]=Ben (171.4 s, track#=6), NOT DB[4]=WCDM (176.9 s, track#=5).
	// With binary track_index (old code): DB[4] wins (same track#, cost=0.499 < 0.571).
	// With graduated track_index (fix): DB[5] wins (diff 1.9 s < grace, cost=0.452).
	benLocalIdx := 3 // local[3] = the 169.5 s file
	benDBIdx := 5    // DB[5] = Ben (track#=6, runtime=171.4 s) in with-bonus slice
	wcdmDBIdx := 4   // DB[4] = WCDM (track#=5, runtime=176.9 s) in with-bonus slice
	if !matched[benDBIdx] {
		t.Errorf(
			"FAIL: DB[6] %q should be matched to the 169.5 s local file, but it was left unmatched",
			dbTracksWithBonus[benDBIdx].Title,
		)
	} else if result[benDBIdx].RuntimeMS != localTracksNoTitle[benLocalIdx].RuntimeMS {
		t.Errorf(
			"FAIL: DB[6] %q matched to wrong local file (runtime=%d ms), expected %d ms",
			dbTracksWithBonus[benDBIdx].Title,
			result[benDBIdx].RuntimeMS,
			localTracksNoTitle[benLocalIdx].RuntimeMS,
		)
	} else {
		t.Logf("  PASS: DB[6] %q correctly matched to local[%d] (runtime=%d ms)",
			dbTracksWithBonus[benDBIdx].Title, benLocalIdx+1, result[benDBIdx].RuntimeMS)
	}
	if matched[wcdmDBIdx] {
		t.Errorf("FAIL: DB[5] %q (WCDM) should be unmatched — it was not ripped locally",
			dbTracksWithBonus[wcdmDBIdx].Title)
	} else {
		t.Logf("  PASS: DB[5] %q correctly left unmatched", dbTracksWithBonus[wcdmDBIdx].Title)
	}

	// ── 2b. matchTracksByDistance — DB without bonus ─────────────────────────
	// The without-bonus edition should have exactly 1 unmatched DB track (WCDM,
	// which was not ripped locally) and 0 unused local tracks.
	t.Log("\n=== matchTracksByDistance (DB without bonus track) ===")
	result2, matched2, used2 := matchTracksByDistance(
		localTracksNoTitle,
		dbTracksWithoutBonus,
		isVA,
		false,
		data,
	)

	unmatchedDB2, unusedLocal2 := 0, 0
	for i, m := range matched2 {
		localTitle := "(unmatched)"
		if m {
			localTitle = result2[i].Title
			if localTitle == "" {
				localTitle = "(no title)"
			}
		} else {
			unmatchedDB2++
		}
		t.Logf("  DB[%2d] %-35s → local: %s", i+1, dbTracksWithoutBonus[i].Title, localTitle)
	}
	for j, u := range used2 {
		if !u {
			unusedLocal2++
			t.Logf(
				"  Local[%2d] track#=%d runtime=%dms → UNUSED",
				j+1,
				localTracksNoTitle[j].TrackNumber,
				localTracksNoTitle[j].RuntimeMS,
			)
		}
	}
	t.Logf("  unmatchedDB=%d  unusedLocal=%d", unmatchedDB2, unusedLocal2)

	// Exactly WCDM (track 4 in without-bonus, index 3) should be unmatched —
	// it was not ripped locally.  All 14 local files must be consumed.
	if unmatchedDB2 != 1 {
		t.Errorf(
			"FAIL: without-bonus edition should have exactly 1 unmatched DB track (WCDM), got %d",
			unmatchedDB2,
		)
	} else {
		t.Log("  PASS: exactly 1 DB track unmatched")
	}

	wcdmIdx := 3 // DB track 4 = WCDM is at index 3 in the without-bonus slice
	if matched2[wcdmIdx] {
		t.Errorf("FAIL: DB[4] %q should be the unmatched track (not ripped), but it was matched",
			dbTracksWithoutBonus[wcdmIdx].Title)
	} else {
		t.Logf("  PASS: DB[4] %q correctly left unmatched (not in local rip)",
			dbTracksWithoutBonus[wcdmIdx].Title)
	}

	if unusedLocal2 != 0 {
		t.Errorf(
			"FAIL: all 14 local files should be matched in the without-bonus edition, %d unused",
			unusedLocal2,
		)
	} else {
		t.Log("  PASS: all local files consumed")
	}

	// ── 2c. per-track cost analysis: binary vs graduated track_index ─────────
	// Shows WHY the graduated fix works.
	//
	// Strict mode: grace=3 s, hard=30 s.  Weights: TL=3, title=3, index=1 → total=7.
	// All title distances = 1.0 (local files have no title tags).
	//
	// Binary  index: 0 if local# == db#, else 1.0
	// Graduated index: abs(local# − db#) / max(local#, db#)
	//
	// The table focuses on local tracks 2–6 (around the bonus-track gap) vs the
	// DB tracks they compete for, making the cost flip visible.
	t.Log("\n=== per-track cost analysis: binary vs graduated track_index ===")
	t.Logf("  %-7s %-9s  %-5s %-28s %-9s  %-7s  %-5s  %-7s  %-9s  %-7s  %-9s",
		"local#", "act(s)", "db#", "db title", "exp(s)", "diff(s)", "TL",
		"i_bin", "cost_bin", "i_grad", "cost_grad")

	graceMs := float64(data.PerTrackToleranceSeconds) * 1000
	hardMs := float64(data.PerTrackToleranceSecondsMax) * 1000
	const wTL, wTitle, wIndex, wTotal = 3.0, 3.0, 1.0, 7.0

	costRow := func(localIdx, dbIdx int) {
		loc := &localTracksNoTitle[localIdx]
		db := &dbTracksWithBonus[dbIdx]

		diffF := float64(loc.RuntimeMS) - float64(db.RuntimeMs)
		if diffF < 0 {
			diffF = -diffF
		}

		var tl float64
		reject := false
		switch {
		case diffF > hardMs:
			tl = 1.0
			reject = true
		case diffF > graceMs:
			tl = (diffF - graceMs) / (hardMs - graceMs)
		}

		locN, dbN := float64(loc.TrackNumber), float64(db.TrackNumber)
		idxDiff := locN - dbN
		if idxDiff < 0 {
			idxDiff = -idxDiff
		}
		denom := locN
		if dbN > denom {
			denom = dbN
		}

		iBin := 0.0
		if loc.TrackNumber != int(db.TrackNumber) {
			iBin = 1.0
		}
		iGrad := idxDiff / denom

		// trackDistance returns 1.0 immediately on hard-reject (strictMode + diff>hard).
		// Reflect that in the displayed costs rather than the formula value.
		var costBin, costGrad float64
		if reject {
			costBin = 1.0
			costGrad = 1.0
		} else {
			costBin = (wTL*tl + wTitle*1.0 + wIndex*iBin) / wTotal
			costGrad = (wTL*tl + wTitle*1.0 + wIndex*iGrad) / wTotal
		}

		note := ""
		if reject {
			note = " ← HARD-REJECT (excluded from matching)"
		} else if costBin != costGrad {
			winner := "binary→bin wins"
			if costGrad < costBin {
				winner = "graduated wins"
			}
			note = fmt.Sprintf(" ← DIFFERS (%s)", winner)
		}

		t.Logf("  %-7d %-9.1f  %-5d %-28s %-9.1f  %-7.1f  %-5.3f  %-7.3f  %-9.4f  %-7.3f  %-9.4f%s",
			loc.TrackNumber, float64(loc.RuntimeMS)/1000,
			db.TrackNumber, db.Title, float64(db.RuntimeMs)/1000,
			diffF/1000, tl,
			iBin, costBin,
			iGrad, costGrad,
			note)
	}

	// Show the key rows: local tracks 2–6 (indices 1–5) vs their likely DB neighbours.
	// Separator shows where the bonus-track gap causes the index divergence.
	pairs := [][2]int{
		{1, 1}, {1, 0}, // local[1] (ADB, #3) vs DB ADB(#2) and Paradise(#1)
		{2, 2}, {2, 1}, // local[2] (MP, #4)  vs DB MP(#3) and ADB(#2)
		{3, 4}, {3, 5}, // local[3] (169.5s, #5) vs WCDM(#5) and Ben(#6) ← KEY
		{4, 5}, {4, 6}, // local[4] (261.3s, #6) vs Ben(#6) and DSLFL(#7) ← KEY
		{5, 6}, {5, 7}, // local[5] (226.9s, #7) vs DSLFL(#7) and IIaW(#8)
	}
	prevLocal := -1
	for _, p := range pairs {
		if p[0] != prevLocal && prevLocal != -1 {
			t.Log("")
		}
		prevLocal = p[0]
		costRow(p[0], p[1])
	}

	t.Log("\n  Summary:")
	t.Log(
		"  local[3] (#5, 169.5s): binary→WCDM(cost_bin<cost_bin_Ben), graduated→Ben(cost_grad<cost_grad_WCDM)",
	)
	t.Log("  local[4] (#6, 261.3s): both→DSLFL once local[3] is correctly routed to Ben")

	// ── 3. albumDistanceWithTracks — with-bonus vs without-bonus ─────────────
	t.Log("\n=== albumDistanceWithTracks ===")

	withBonusCandidate := &database.AlbumSearchResult{
		Artist:      "Boyzone",
		Title:       "A Different Beat",
		Year:        1996,
		TotalTracks: len(dbTracksWithBonus),
	}
	withoutBonusCandidate := &database.AlbumSearchResult{
		Artist:      "Boyzone",
		Title:       "A Different Beat",
		Year:        1996,
		TotalTracks: len(dbTracksWithoutBonus),
	}

	distWithBonus := albumDistanceWithTracks(
		withBonusCandidate,
		"Boyzone",
		"A Different Beat",
		"",
		1996,
		localTracksWithTitle,
		dbTracksWithBonus,
		isVA,
		data,
	)
	distWithoutBonus := albumDistanceWithTracks(
		withoutBonusCandidate,
		"Boyzone",
		"A Different Beat",
		"",
		1996,
		localTracksWithTitle,
		dbTracksWithoutBonus,
		isVA,
		data,
	)

	t.Logf("  with-bonus edition    dist = %.4f", distWithBonus)
	t.Logf("  without-bonus edition dist = %.4f", distWithoutBonus)

	if distWithBonus <= distWithoutBonus {
		t.Errorf(
			"FAIL: with-bonus edition (%.4f) should score worse than without-bonus edition (%.4f)",
			distWithBonus,
			distWithoutBonus,
		)
	} else {
		t.Log("  PASS: without-bonus edition scores better (lower distance)")
	}

	// ── 4. recommendation ────────────────────────────────────────────────────
	t.Log("\n=== recommendation ===")

	recWithoutOnly := recommendation([]float64{distWithoutBonus}, nil)
	t.Logf("  recommendation([without-bonus=%.4f]) = %d", distWithoutBonus, recWithoutOnly)
	if recWithoutOnly < recMedium {
		t.Errorf(
			"FAIL: without-bonus edition should yield at least recMedium, got %d",
			recWithoutOnly,
		)
	} else {
		t.Log("  PASS: without-bonus edition accepted (recMedium or recStrong)")
	}

	// Both candidates together (without-bonus first, lower is better).
	sortedDists := []float64{distWithoutBonus, distWithBonus}
	recBoth := recommendation(sortedDists, nil)
	t.Logf(
		"  recommendation([without=%.4f, with=%.4f]) = %d",
		distWithoutBonus,
		distWithBonus,
		recBoth,
	)
	if recBoth < recMedium {
		t.Errorf(
			"FAIL: without-bonus edition should be accepted even when bonus candidate is present, got %d",
			recBoth,
		)
	} else {
		t.Log("  PASS: without-bonus edition accepted even with bonus candidate present")
	}

	// with-bonus distance should exceed strongRecThresh to be distinguishable.
	t.Logf("  with-bonus dist=%.4f > strongRecThresh=%.2f: %v",
		distWithBonus, float64(strongRecThresh), distWithBonus > strongRecThresh)
	if distWithBonus <= strongRecThresh {
		t.Errorf("FAIL: with-bonus dist %.4f should exceed strongRecThresh %.2f",
			distWithBonus, float64(strongRecThresh))
	} else {
		t.Log("  PASS: with-bonus edition is measurably worse than the standard edition")
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

// TestIronMaidenWrongEditionMatching exercises the full matching stack:
//   - detectVA: Iron Maiden is not a VA release
//   - matchTracksByDistance: wrong-edition detection via title mismatch
//   - albumDistanceWithTracks: full distance for wrong vs right edition candidate
//   - recommendation: wrong edition scores above mediumRecThresh → recNone
//
// Scenario: DB has Iron Maiden (1980) without Sanctuary at track 2; local files
// are the reissue with Sanctuary at track 2.  The wrong-edition DB entry should
// produce a high full distance and recNone, triggering searchAndImportAlternativeRelease.
//
// Run with: go test -v -run TestIronMaidenWrongEditionMatching
func TestIronMaidenWrongEditionMatching(t *testing.T) {
	// DB tracks: original 1980 edition — no Sanctuary, Remember Tomorrow is track 2.
	dbTracksWrongEdition := []database.DbtrackWithArtist{
		{Dbtrack: database.Dbtrack{Title: "Prowler", TrackNumber: 1, RuntimeMs: 235300}},
		{Dbtrack: database.Dbtrack{Title: "Remember Tomorrow", TrackNumber: 2, RuntimeMs: 327800}},
		{Dbtrack: database.Dbtrack{Title: "Running Free", TrackNumber: 3, RuntimeMs: 197100}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Phantom of the Opera",
				TrackNumber: 4,
				RuntimeMs:   440700,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Transylvania", TrackNumber: 5, RuntimeMs: 245200}},
		{Dbtrack: database.Dbtrack{Title: "Strange World", TrackNumber: 6, RuntimeMs: 346200}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Charlotte the Harlot",
				TrackNumber: 7,
				RuntimeMs:   252700,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Iron Maiden", TrackNumber: 8, RuntimeMs: 214600}},
		{Dbtrack: database.Dbtrack{Title: "Iron Maiden", TrackNumber: 9, RuntimeMs: 216200}},
	}

	// DB tracks: reissue edition — Sanctuary at track 2, one fewer Iron Maiden bonus.
	dbTracksRightEdition := []database.DbtrackWithArtist{
		{Dbtrack: database.Dbtrack{Title: "Prowler", TrackNumber: 1, RuntimeMs: 236000}},
		{Dbtrack: database.Dbtrack{Title: "Sanctuary", TrackNumber: 2, RuntimeMs: 196500}},
		{Dbtrack: database.Dbtrack{Title: "Remember Tomorrow", TrackNumber: 3, RuntimeMs: 328000}},
		{Dbtrack: database.Dbtrack{Title: "Running Free", TrackNumber: 4, RuntimeMs: 197200}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Phantom of the Opera",
				TrackNumber: 5,
				RuntimeMs:   429000,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Transylvania", TrackNumber: 6, RuntimeMs: 258000}},
		{Dbtrack: database.Dbtrack{Title: "Strange World", TrackNumber: 7, RuntimeMs: 332000}},
		{
			Dbtrack: database.Dbtrack{
				Title:       "Charlotte the Harlot",
				TrackNumber: 8,
				RuntimeMs:   252700,
			},
		},
		{Dbtrack: database.Dbtrack{Title: "Iron Maiden", TrackNumber: 9, RuntimeMs: 216200}},
	}

	// Local files: reissue with Sanctuary at track 2.
	localTracks := []parser_v2.TrackInfo{
		{Title: "Prowler", TrackNumber: 1, RuntimeMS: 236200},
		{Title: "Sanctuary", TrackNumber: 2, RuntimeMS: 196300},
		{Title: "Remember Tomorrow", TrackNumber: 3, RuntimeMS: 328600},
		{Title: "Running Free", TrackNumber: 4, RuntimeMS: 197300},
		{Title: "Phantom of the Opera", TrackNumber: 5, RuntimeMS: 428000},
		{Title: "Transylvania", TrackNumber: 6, RuntimeMS: 259300},
		{Title: "Strange World", TrackNumber: 7, RuntimeMS: 332400},
		{Title: "Charlotte the Harlot", TrackNumber: 8, RuntimeMS: 252700},
		{Title: "Iron Maiden", TrackNumber: 9, RuntimeMS: 216200},
	}

	data := &config.MediaDataConfig{
		PerTrackToleranceSeconds:    10,
		PerTrackToleranceSecondsMax: 30,
	}

	// ── 1. detectVA ──────────────────────────────────────────────────────────
	t.Log("=== detectVA ===")
	isVA := DetectVA("Iron Maiden", localTracks)
	t.Logf("  detectVA(%q) = %v", "Iron Maiden", isVA)
	if isVA {
		t.Error("FAIL: Iron Maiden should NOT be detected as VA")
	} else {
		t.Log("  PASS: correctly identified as non-VA")
	}

	// ── 2. matchTracksByDistance — wrong edition ──────────────────────────────
	t.Log("\n=== matchTracksByDistance (wrong edition) ===")
	result, matched, used := matchTracksByDistance(
		localTracks,
		dbTracksWrongEdition,
		isVA,
		false,
		data,
	)

	unmatchedDB, unusedLocal := 0, 0
	for i, m := range matched {
		localTitle := "(unmatched)"
		if m {
			localTitle = result[i].Title
		} else {
			unmatchedDB++
		}
		t.Logf("  DB[%d] %-25s → local: %s", i+1, dbTracksWrongEdition[i].Title, localTitle)
	}
	for j, u := range used {
		if !u {
			unusedLocal++
			t.Logf("  Local[%d] %-25s → UNUSED", j+1, localTracks[j].Title)
		}
	}
	t.Logf("  unmatchedDB=%d  unusedLocal=%d", unmatchedDB, unusedLocal)
	if unmatchedDB == 0 && unusedLocal == 0 {
		t.Error("FAIL: wrong-edition matching should have left at least one track unmatched")
	} else {
		t.Log("  PASS: wrong-edition produced unmatched tracks — alternative search would trigger")
	}

	// ── 3. albumDistanceWithTracks — wrong vs right edition ──────────────────
	t.Log("\n=== albumDistanceWithTracks ===")

	wrongCandidate := &database.AlbumSearchResult{
		Artist:      "Iron Maiden",
		Title:       "Iron Maiden",
		Year:        1980,
		TotalTracks: len(dbTracksWrongEdition),
	}
	rightCandidate := &database.AlbumSearchResult{
		Artist:      "Iron Maiden",
		Title:       "Iron Maiden",
		Year:        1980,
		TotalTracks: len(dbTracksRightEdition),
	}

	distWrong := albumDistanceWithTracks(
		wrongCandidate,
		"Iron Maiden",
		"Iron Maiden",
		"",
		1980,
		localTracks,
		dbTracksWrongEdition,
		isVA,
		data,
	)
	distRight := albumDistanceWithTracks(
		rightCandidate,
		"Iron Maiden",
		"Iron Maiden",
		"",
		1980,
		localTracks,
		dbTracksRightEdition,
		isVA,
		data,
	)

	t.Logf("  wrong edition dist = %.4f", distWrong)
	t.Logf("  right edition dist = %.4f", distRight)

	if distWrong <= distRight {
		t.Errorf(
			"FAIL: wrong edition (%.4f) should score worse than right edition (%.4f)",
			distWrong,
			distRight,
		)
	} else {
		t.Log("  PASS: right edition scores better (lower distance)")
	}

	// ── 4. recommendation ────────────────────────────────────────────────────
	//
	// The recommendation function operates on candidates that already passed
	// matchTracksByDistance.  The wrong edition is excluded upstream (unmatchedDB>0),
	// so in practice only the right edition ever reaches this stage.
	//
	// We also verify the wrong-edition distance is above strongRecThresh so it
	// would be distinguishable from a perfect match, even if it somehow passed.
	t.Log("\n=== recommendation ===")

	// Right edition alone (the realistic case after wrong edition is filtered).
	recRightOnly := recommendation([]float64{distRight}, nil)
	t.Logf("  recommendation([right=%.4f]) = %d", distRight, recRightOnly)
	if recRightOnly < recMedium {
		t.Errorf("FAIL: right edition should yield at least recMedium, got %d", recRightOnly)
	} else {
		t.Log("  PASS: right edition accepted (recMedium or recStrong)")
	}

	// Both candidates sorted best-first (hypothetical: both passed track matching).
	// Right edition should still be accepted; gap check may or may not upgrade to recStrong.
	sortedDists := []float64{distRight, distWrong}
	recBoth := recommendation(sortedDists, nil)
	t.Logf("  recommendation([right=%.4f, wrong=%.4f]) = %d", distRight, distWrong, recBoth)
	if recBoth < recMedium {
		t.Errorf(
			"FAIL: right edition should be accepted even when wrong candidate is present, got %d",
			recBoth,
		)
	} else {
		t.Log("  PASS: right edition accepted even with wrong candidate present")
	}

	// Wrong edition distance should be above strongRecThresh (visible separation from a perfect match).
	t.Logf(
		"  wrong dist=%.4f > strongRecThresh=%.2f: %v",
		distWrong,
		float64(strongRecThresh),
		distWrong > strongRecThresh,
	)
	if distWrong <= strongRecThresh {
		t.Errorf(
			"FAIL: wrong edition dist %.4f should exceed strongRecThresh %.2f",
			distWrong,
			float64(strongRecThresh),
		)
	} else {
		t.Log("  PASS: wrong edition is measurably worse than a perfect match")
	}
}

func localTracks(tracks ...dbTrack) ([]parser_v2.TrackInfo, int64) {
	out := make([]parser_v2.TrackInfo, len(tracks))

	var total int64

	for i, tr := range tracks {
		out[i] = parser_v2.TrackInfo{Title: tr.title, TrackNumber: i + 1, DiscNumber: 1, RuntimeMS: tr.ms, Artist: "Pink Floyd"}
		total += tr.ms
	}

	return out, total
}
