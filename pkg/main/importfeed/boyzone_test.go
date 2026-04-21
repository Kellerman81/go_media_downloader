package importfeed

import (
	"fmt"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

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

	recWithoutOnly := recommendation([]float64{distWithoutBonus})
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
	recBoth := recommendation(sortedDists)
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
