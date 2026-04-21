package importfeed

// TestUweJensenDiagnostic diagnoses why
//
//	Q:\_tocheck_Music\no_match\Uwe Jensen - Jubiläumsgold-2013-Samfie Man\
//
// could not be matched to MusicBrainz release 1d309abf-e408-4895-8289-a9a430da1682.
// Tags: Artist="Uwe Jensen", Album="Jubiläumsgold"
//
// Run with:
//
//	go test -v -run TestUweJensenDiagnostic ./importfeed/

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

const (
	ujFolder    = `Q:\_tocheck_Music\no_match\Uwe Jensen - Jubiläumsgold-2013-Samfie Man`
	ujMBRelease = "1d309abf-e408-4895-8289-a9a430da1682"
	// Known tags (from user report)
	ujTagArtist = "Uwe Jensen"
	ujTagAlbum  = "Jubiläumsgold"
)

func TestUweJensenDiagnostic(t *testing.T) {
	// ── 1. Collect files ──────────────────────────────────────────────────────
	entries, err := os.ReadDir(ujFolder)
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
			files = append(files, filepath.Join(ujFolder, e.Name()))
		}
	}
	if len(files) == 0 {
		t.Skip("no audio files found in folder")
	}
	t.Logf("Found %d audio files", len(files))

	// ── 2. Parse folder name ──────────────────────────────────────────────────
	folderArtist, folderAlbum, folderYear := parser_v2.ParseAudioFolder(ujFolder)
	t.Logf("\n=== Folder name parse ===")
	t.Logf("  folderArtist : %q", folderArtist)
	t.Logf("  folderAlbum  : %q", folderAlbum)
	t.Logf("  year         : %d", folderYear)

	// ── 3. Read tags from files ───────────────────────────────────────────────
	t.Logf("\n=== Tags (first file with non-empty tags) ===")
	tagData := parser_v2.ReadTagsForFirstFile(files)
	var tagArtist, tagAlbumArtist, tagAlbum, tagMBID string
	var tagYear int
	if tagData != nil {
		tagArtist = tagData.Artist
		tagAlbumArtist = tagData.AlbumArtist
		tagAlbum = tagData.Album
		tagMBID = tagData.MusicBrainzID
		tagYear = tagData.Year
		t.Logf("  Artist       : %q", tagArtist)
		t.Logf("  AlbumArtist  : %q", tagAlbumArtist)
		t.Logf("  Album        : %q", tagAlbum)
		t.Logf("  MusicBrainzID: %q", tagMBID)
		t.Logf("  Year         : %d", tagYear)
	} else {
		t.Logf("  (no tags found)")
		// Fall back to known tags
		tagArtist = ujTagArtist
		tagAlbum = ujTagAlbum
	}

	// ── 4. Collect & enrich all tracks ───────────────────────────────────────
	tracks := parser_v2.CollectTracksFromFiles(files)
	tracks = parser_v2.EnrichTracksWithTags(tracks)
	fileCount := len(tracks)
	t.Logf("\n=== Tracks (%d) ===", fileCount)
	var localTotalMs int64
	for i, tr := range tracks {
		localTotalMs += tr.RuntimeMS
		t.Logf("  [%02d] disc=%d track=%02d runtime=%6dms (%.1fs) title=%q artist=%q",
			i+1, tr.DiscNumber, tr.TrackNumber, tr.RuntimeMS,
			float64(tr.RuntimeMS)/1000, tr.Title, tr.Artist)
	}
	t.Logf("  Local total runtime: %dms (%.1fs)", localTotalMs, float64(localTotalMs)/1000)

	// ── 5. Resolve artist/album as matchMusicFolder would ────────────────────
	artist := coalesceStr(tagAlbumArtist, tagArtist, folderArtist)
	albumTitle := coalesceStr(tagAlbum, folderAlbum)
	year := tagYear
	if year == 0 {
		year = folderYear
	}

	t.Logf("\n=== Resolved artist/album for search ===")
	t.Logf("  artist     : %q", artist)
	t.Logf("  albumTitle : %q", albumTitle)
	t.Logf("  year       : %d", year)

	// ── 6. Set up MusicBrainz provider ───────────────────────────────────────
	mbProvider := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   1,
		RateLimitSeconds: 1,
		UserAgent:        "GoMediaDownloader/1.0 (diagnostic)",
	})
	providers.SetMusicBrainz(mbProvider)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// ── 7. Fetch the target release directly ─────────────────────────────────
	t.Logf("\n=== Target MB release: %s ===", ujMBRelease)
	details, err := mbProvider.GetReleaseByID(ctx, ujMBRelease)
	if err != nil || details == nil {
		t.Logf("  ERROR fetching release: %v", err)
	} else {
		var detailArtistNames []string
		for _, a := range details.Artists {
			detailArtistNames = append(detailArtistNames, a.Name)
		}
		t.Logf("  Title   : %q", details.Title)
		t.Logf("  Artists : %v", detailArtistNames)
		t.Logf("  Tracks  : %d", details.TrackCount)
		t.Logf("  Year    : %d", details.ReleaseYear)
		t.Logf("  Country : %s", details.Country)
		t.Logf("  Format  : %s", details.Format)
		var mbTotalMs int64
		for i, tr := range details.Tracks {
			mbTotalMs += tr.Duration.Milliseconds()
			t.Logf("  [%02d] disc=%d track=%02d runtime=%6dms title=%q",
				i+1, tr.DiscNumber, tr.TrackNumber, tr.Duration.Milliseconds(), tr.Title)
		}
		t.Logf("  MB total runtime: %dms (%.1fs)", mbTotalMs, float64(mbTotalMs)/1000)
	}

	// ── 8. Simulate searchAndImportAlternativeRelease text-search path ──────
	// This is the path that runs when the album is NOT in the database yet.
	// It mirrors exactly what the production code does: search, filter by track
	// count, fetch details, check runtime.
	t.Logf("\n=== Simulating searchAndImportAlternativeRelease (text search path) ===")
	prodQuery := string(BuildArtistAlbumSearch(albumTitle, artist))
	t.Logf("  Production query: %s", prodQuery)
	t.Logf("  fileCount: %d", fileCount)
	t.Logf("  localTotalMs: %d", localTotalMs)

	perTrackMs := int64(3000)
	toleranceMs := int64(fileCount) * perTrackMs
	t.Logf("  toleranceMs (default): %d", toleranceMs)

	searchResults, _, serr := mbProvider.SearchReleases(ctx, prodQuery, 50, 0)
	if serr != nil {
		t.Logf("  Search ERROR: %v", serr)
	} else {
		t.Logf("  Raw results: %d", len(searchResults))
		for i, r := range searchResults {
			targetMark := ""
			if r.MusicBrainzID == ujMBRelease {
				targetMark = " *** TARGET ***"
			}
			var rArtists []string
			for _, a := range r.Artists {
				rArtists = append(rArtists, a.Name)
			}
			t.Logf("  [%d] %q | artist=%v | tracks=%d | year=%d | id=%s%s",
				i+1, r.Title, rArtists, r.TrackCount, r.ReleaseYear, r.MusicBrainzID, targetMark)

			// Step A: track count filter
			if r.TrackCount != fileCount {
				t.Logf("       → SKIP: track count %d != fileCount %d", r.TrackCount, fileCount)
				continue
			}

			// Step B: artist filter (client-side)
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

			// Step C: fetch full details and check runtime
			t.Logf("       → Fetching full details for %s ...", r.MusicBrainzID)
			rd, derr := mbProvider.GetReleaseByID(ctx, r.MusicBrainzID)
			if derr != nil || rd == nil {
				t.Logf("       → SKIP: details fetch error: %v", derr)
				continue
			}

			var relRuntimeMs int64
			for j := range rd.Tracks {
				relRuntimeMs += rd.Tracks[j].Duration.Milliseconds()
			}
			runtimeDiff := localTotalMs - relRuntimeMs
			if runtimeDiff < 0 {
				runtimeDiff = -runtimeDiff
			}

			if relRuntimeMs == 0 {
				t.Logf("       → MB has no duration data (relRuntimeMs=0): FAIL-OPEN → WOULD IMPORT%s", targetMark)
			} else if runtimeDiff <= toleranceMs {
				t.Logf("       → runtime OK (diff=%dms <= tol=%dms) → WOULD IMPORT%s", runtimeDiff, toleranceMs, targetMark)
			} else {
				t.Logf("       → SKIP: runtime diff %dms > tolerance %dms (relRuntime=%dms)%s",
					runtimeDiff, toleranceMs, relRuntimeMs, targetMark)
			}
		}
	}

	// ── 8b. Typo / diacritic-stripped variant searches ───────────────────────
	// Verify whether MB accent-folding handles "Jubilaumsgold" (ä→a) and
	// whether a Lucene fuzzy term (~1) would catch other single-char typos.
	typoVariants := []struct{ label, query string }{
		{"diacritic stripped (ä→a)", string(BuildArtistAlbumSearch("Jubilaumsgold", artist))},
		{"fuzzy release term (~1)", "release:Jubilaumsgold~1 AND artist:\"Uwe Jensen\"~2"},
		{"artist only (no release filter)", "artist:\"Uwe Jensen\"~2"},
	}
	t.Logf("\n=== Typo / diacritic robustness checks ===")
	for _, v := range typoVariants {
		t.Logf("\n--- %s ---", v.label)
		t.Logf("  Query: %s", v.query)
		res, _, verr := mbProvider.SearchReleases(ctx, v.query, 5, 0)
		if verr != nil {
			t.Logf("  ERROR: %v", verr)
			continue
		}
		t.Logf("  Results: %d", len(res))
		for i, r := range res {
			targetMark := ""
			if r.MusicBrainzID == ujMBRelease {
				targetMark = " *** TARGET ***"
			}
			var rArtists []string
			for _, a := range r.Artists {
				rArtists = append(rArtists, a.Name)
			}
			t.Logf("  [%d] %q | tracks=%d | year=%d | id=%s%s",
				i+1, r.Title, r.TrackCount, r.ReleaseYear, r.MusicBrainzID, targetMark)
			_ = rArtists
		}
	}

	// ── 9. albumMatchDistance against the target as a synthetic candidate ─────
	if details != nil {
		t.Logf("\n=== albumMatchDistance against target release ===")
		var artistNames []string
		for _, a := range details.Artists {
			artistNames = append(artistNames, a.Name)
		}
		candidate := &database.AlbumSearchResult{
			ID:                   0,
			MusicBrainzReleaseID: ujMBRelease,
			Title:                details.Title,
			Artist:               strings.Join(artistNames, ", "),
			TotalTracks:          details.TrackCount,
			Year:                 details.ReleaseYear,
		}
		t.Logf("  candidate.Artist : %q", candidate.Artist)
		t.Logf("  candidate.Title  : %q", candidate.Title)
		t.Logf("  local artist     : %q", artist)
		t.Logf("  local album      : %q", albumTitle)

		data := &config.MediaDataConfig{}
		dist := albumMatchDistance(candidate, artist, albumTitle, tagMBID, "", "", year, fileCount, data)
		t.Logf("  albumMatchDistance: %.4f  (strong=%.2f medium=%.2f)",
			dist, strongRecThresh, mediumRecThresh)

		// ── 10. Full distance with track matching ─────────────────────────────
		if len(details.Tracks) > 0 {
			t.Logf("\n=== Track matching ===")
			dbTracks := mbTracksToDbTracks(details.Tracks)
			isVA := DetectVA(artist, tracks)
			t.Logf("  isVA: %v", isVA)
			result, matched, used := matchTracksByDistance(tracks, dbTracks, isVA, false, data)

			unmatchedDB, unusedLocal := 0, 0
			for _, m := range matched {
				if !m {
					unmatchedDB++
				}
			}
			for _, u := range used {
				if !u {
					unusedLocal++
				}
			}
			t.Logf("  unmatchedDB=%d  unusedLocal=%d", unmatchedDB, unusedLocal)
			for i, tr := range result {
				if !matched[i] {
					t.Logf("  DB[%02d] %q — UNMATCHED", i+1, dbTracks[i].Title)
					continue
				}
				d := trackDistance(&tr, &dbTracks[i], isVA, false, data)
				rtDiff := tr.RuntimeMS - dbTracks[i].RuntimeMs
				t.Logf("  DB[%02d] %q <- local %q  trackDist=%.4f  rtDiff=%+dms",
					i+1, dbTracks[i].Title, tr.Title, d, rtDiff)
			}

			fullDist := albumDistanceWithTracks(
				candidate, artist, albumTitle, tagMBID, year,
				tracks, dbTracks, isVA, data,
			)
			t.Logf("\n  fullDist (album+tracks): %.4f", fullDist)
			rec := recommendation([]float64{fullDist})
			t.Logf("  recommendation: %v  (need >= recMedium=%v)", rec, recMedium)
		}
	}
}
