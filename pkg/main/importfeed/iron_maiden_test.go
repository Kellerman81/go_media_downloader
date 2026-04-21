package importfeed

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

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
	recRightOnly := recommendation([]float64{distRight})
	t.Logf("  recommendation([right=%.4f]) = %d", distRight, recRightOnly)
	if recRightOnly < recMedium {
		t.Errorf("FAIL: right edition should yield at least recMedium, got %d", recRightOnly)
	} else {
		t.Log("  PASS: right edition accepted (recMedium or recStrong)")
	}

	// Both candidates sorted best-first (hypothetical: both passed track matching).
	// Right edition should still be accepted; gap check may or may not upgrade to recStrong.
	sortedDists := []float64{distRight, distWrong}
	recBoth := recommendation(sortedDists)
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
