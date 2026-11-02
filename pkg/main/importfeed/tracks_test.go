package importfeed

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

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
	titleTokenQuery := fmt.Sprintf(
		`artist:"%s" AND %s`,
		artist,
		strings.Join(releaseTokens, " AND "),
	)

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

func TestMajorityReleaseTieIsDeterministic(t *testing.T) {
	counts := map[string]int{"rel-b": 3, "rel-a": 3, "rel-c": 1}
	for range 50 {
		if got := majorityRelease(counts, 5); got != "rel-a" {
			t.Fatalf("got %q, want rel-a", got)
		}
	}
}

func TestBuildArtistAlbumSearchBindsAllTitleWords(t *testing.T) {
	got := string(BuildArtistAlbumSearch("Greatest Hits (Live)", "AC/DC"))
	want := `(release:"Greatest Hits \(Live\)"^4 OR release:(Greatest Hits \(Live\))) AND artist:"AC\/DC"~2`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if got := string(BuildArtistAlbumSearch("Hits", VariousArtistsName)); got != `(release:"Hits"^4 OR release:(Hits))` {
		t.Fatalf("VA query: %s", got)
	}
}

func TestTracksFilterDiagnostic(t *testing.T) {
	const (
		targetTitle  = "Bravo Hits 130"
		targetArtist = "Various Artists"
		targetTracks = 48
		targetID     = "8f894280-a181-443b-826b-d64075c400b7"
	)

	mbProvider := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   1,
		RateLimitSeconds: 1,
		UserAgent:        "GoMediaDownloader/1.0 (diagnostic)",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	type searchQ struct{ label, query string }
	queries := []searchQ{
		{
			"without tracks filter",
			string(BuildArtistAlbumSearch(targetTitle, targetArtist)),
		},
		{
			"with tracks:48 (total)",
			string(BuildArtistAlbumSearch(targetTitle, targetArtist)) + " AND tracks:48",
		},
		{
			// MB tracks: field counts per-medium on some releases (2xCD → tracks:24)
			"with tracks:24 (per-disc, 2xCD)",
			string(BuildArtistAlbumSearch(targetTitle, targetArtist)) + " AND tracks:24",
		},
	}

	for _, q := range queries {
		t.Logf("\n--- %s ---", q.label)
		t.Logf("  Query: %s", q.query)
		results, _, err := mbProvider.SearchReleases(ctx, q.query, 10, 0)
		if err != nil {
			t.Logf("  ERROR: %v", err)
			continue
		}
		t.Logf("  Results: %d", len(results))
		for i, r := range results {
			targetMark := ""
			if r.MusicBrainzID == targetID {
				targetMark = " *** TARGET ***"
			}
			t.Logf("  [%d] %q | tracks=%d | year=%d | id=%s%s",
				i+1, r.Title, r.TrackCount, r.ReleaseYear, r.MusicBrainzID, targetMark)
		}
	}
}

const (
	vdggFolder = `Q:\_tocheck_Music\wrong_runtime\Van Der Graaf Generator-1977-The Quiet Zone, The Pleasure Dome NMR`
	// Known 9-track original release (example — may not be this exact MBID)
	vdggReleaseGroup = "4bb9cda8-0742-30ff-ae73-955c6860fc25"
)

func TestVDGGDiagnostic(t *testing.T) {
	// ── 1. Collect files ──────────────────────────────────────────────────────
	entries, err := os.ReadDir(vdggFolder)
	if err != nil {
		t.Skipf("folder not accessible: %v", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".mp3", ".flac", ".ogg", ".m4a", ".aac", ".wav", ".wv", ".ape", ".opus":
			files = append(files, filepath.Join(vdggFolder, e.Name()))
		}
	}
	if len(files) == 0 {
		t.Skip("no audio files found in folder")
	}
	t.Logf("Found %d audio files", len(files))

	// ── 2. Parse folder / tags ────────────────────────────────────────────────
	folderArtist, folderAlbum, folderYear := parser_v2.ParseAudioFolder(vdggFolder)
	t.Logf("\n=== Folder parse ===")
	t.Logf("  folderArtist : %q", folderArtist)
	t.Logf("  folderAlbum  : %q", folderAlbum)
	t.Logf("  year         : %d", folderYear)

	tagData := parser_v2.ReadTagsForFirstFile(files)
	var tagArtist, tagAlbumArtist, tagAlbum, tagMBID string
	var tagYear int
	if tagData != nil {
		tagArtist = tagData.Artist
		tagAlbumArtist = tagData.AlbumArtist
		tagAlbum = tagData.Album
		tagMBID = tagData.MusicBrainzID
		tagYear = tagData.Year
		t.Logf("\n=== Tags ===")
		t.Logf("  Artist       : %q", tagArtist)
		t.Logf("  AlbumArtist  : %q", tagAlbumArtist)
		t.Logf("  Album        : %q", tagAlbum)
		t.Logf("  MusicBrainzID: %q", tagMBID)
		t.Logf("  Year         : %d", tagYear)
	}

	// ── 3. Collect tracks ─────────────────────────────────────────────────────
	tracks := parser_v2.CollectTracksFromFiles(files)
	tracks = parser_v2.EnrichTracksWithTags(tracks)
	fileCount := len(tracks)
	var localTotalMs int64
	for _, tr := range tracks {
		localTotalMs += tr.RuntimeMS
	}
	t.Logf(
		"\n=== Local: %d tracks, %dms (%.1fs) ===",
		fileCount,
		localTotalMs,
		float64(localTotalMs)/1000,
	)

	// ── 4. Resolve artist/album as matchMusicFolder would ────────────────────
	artist := coalesceStr(tagAlbumArtist, tagArtist, folderArtist)
	albumTitle := coalesceStr(tagAlbum, folderAlbum)
	year := tagYear
	if year == 0 {
		year = folderYear
	}
	t.Logf("\n=== Resolved for search ===")
	t.Logf("  artist     : %q", artist)
	t.Logf("  albumTitle : %q", albumTitle)
	t.Logf("  year       : %d", year)

	// ── 5. MB provider ────────────────────────────────────────────────────────
	mbProvider := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   1,
		RateLimitSeconds: 1,
		UserAgent:        "GoMediaDownloader/1.0 (diagnostic)",
	})
	providers.SetMusicBrainz(mbProvider)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// ── 6. Simulate searchAndImportAlternativeRelease ─────────────────────────
	// This mirrors exactly what the production code does when tryMusicAlternativeRelease fires.
	t.Logf("\n=== Simulating searchAndImportAlternativeRelease ===")

	perTrackMs := int64(3000)
	toleranceMs := int64(fileCount) * perTrackMs

	type searchQ struct{ label, query string }
	queries := []searchQ{
		{
			"production query (BuildArtistAlbumSearch)",
			string(BuildArtistAlbumSearch(albumTitle, artist)),
		},
		// fuzzy fallback (new code, only fires when primary returns 0):
		{"fuzzy fallback", func() string {
			buf := logger.PlAddBuffer.Get()
			defer logger.PlAddBuffer.Put(buf)
			buf.WriteString(`release:"`)
			buf.WriteString(logger.LuceneEscape(albumTitle))
			buf.WriteString(`"~2`)
			if artist != "" && !IsVariousArtists(artist) {
				buf.WriteString(` AND artist:"`)
				buf.WriteString(logger.LuceneEscape(artist))
				buf.WriteString(`"~2`)
			}
			return buf.String()
		}()},
		// title with slash instead of comma (MB uses "/"):
		{"slash variant", string(BuildArtistAlbumSearch(
			strings.ReplaceAll(albumTitle, ",", " /"), artist))},
	}

	for _, q := range queries {
		t.Logf("\n--- %s ---", q.label)
		t.Logf("  Query: %s", q.query)
		results, _, serr := mbProvider.SearchReleases(ctx, q.query, 50, 0)
		if serr != nil {
			t.Logf("  ERROR: %v", serr)
			continue
		}
		t.Logf("  Raw results: %d", len(results))
		for i, r := range results {
			var rArtists []string
			for _, a := range r.Artists {
				rArtists = append(rArtists, a.Name)
			}
			t.Logf("  [%d] %q | tracks=%d | year=%d | id=%s",
				i+1, r.Title, r.TrackCount, r.ReleaseYear, r.MusicBrainzID)

			// Track count filter
			if r.TrackCount != fileCount {
				t.Logf("       → SKIP: tracks %d != fileCount %d", r.TrackCount, fileCount)
				continue
			}

			// Artist filter
			artistMatch := IsVariousArtists(artist)
			if !artistMatch {
				for _, a := range r.Artists {
					if strings.EqualFold(a.Name, artist) {
						artistMatch = true
						break
					}
				}
			}
			if !artistMatch {
				t.Logf("       → SKIP: no artist match for %q in %v", artist, rArtists)
				continue
			}

			// Fetch details and check runtime
			t.Logf("       → Fetching details ...")
			rd, derr := mbProvider.GetReleaseByID(ctx, r.MusicBrainzID)
			if derr != nil || rd == nil {
				t.Logf("       → SKIP: details error: %v", derr)
				continue
			}
			var relRuntimeMs int64
			for j := range rd.Tracks {
				relRuntimeMs += rd.Tracks[j].Duration.Milliseconds()
			}
			diff := localTotalMs - relRuntimeMs
			if diff < 0 {
				diff = -diff
			}
			if relRuntimeMs == 0 {
				t.Logf(
					"       → MB has no durations: FAIL-OPEN → WOULD IMPORT (id=%s)",
					r.MusicBrainzID,
				)
			} else if diff <= toleranceMs {
				t.Logf(
					"       → runtime OK (diff=%dms <= tol=%dms) → WOULD IMPORT (id=%s)",
					diff,
					toleranceMs,
					r.MusicBrainzID,
				)
			} else {
				t.Logf(
					"       → SKIP: runtime diff %dms > tol %dms (mbRuntime=%dms)",
					diff,
					toleranceMs,
					relRuntimeMs,
				)
			}
		}
	}
}
