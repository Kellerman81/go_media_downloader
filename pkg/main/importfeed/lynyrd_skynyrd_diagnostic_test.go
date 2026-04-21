package importfeed

// TestLynyrdSkynyrdDiagnostic diagnoses why
//
//	Q:\_tocheck_Music\no_match\1993 - Lynyrd Skynyrd - The Last Rebel\
//
// could not be matched to MusicBrainz release 4f212e57-51ff-49da-9857-b329497ca135.
//
// Run with:
//
//	go test -v -run TestLynyrdSkynyrdDiagnostic ./importfeed/

import (
	"context"
	"fmt"
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
	lsFolder    = `Q:\_tocheck_Music\no_match\1993 - Lynyrd Skynyrd - The Last Rebel`
	lsMBRelease = "4f212e57-51ff-49da-9857-b329497ca135"
)

func TestLynyrdSkynyrdDiagnostic(t *testing.T) {
	// ── 1. Collect files ──────────────────────────────────────────────────────
	entries, err := os.ReadDir(lsFolder)
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
			files = append(files, filepath.Join(lsFolder, e.Name()))
		}
	}
	if len(files) == 0 {
		t.Skip("no audio files found in folder")
	}
	t.Logf("Found %d audio files", len(files))

	// ── 2. Parse folder name ──────────────────────────────────────────────────
	folderArtist, folderAlbum, year := parser_v2.ParseAudioFolder(lsFolder)
	t.Logf("\n=== Folder name parse ===")
	t.Logf("  folderArtist : %q", folderArtist)
	t.Logf("  folderAlbum  : %q", folderAlbum)
	t.Logf("  year         : %d", year)

	// ── 3. Read tags from files ───────────────────────────────────────────────
	t.Logf("\n=== Tags (first file with non-empty tags) ===")
	tagData := parser_v2.ReadTagsForFirstFile(files)
	var tagArtist, tagAlbumArtist, tagAlbum, tagMBID string
	if tagData != nil {
		tagArtist = tagData.Artist
		tagAlbumArtist = tagData.AlbumArtist
		tagAlbum = tagData.Album
		tagMBID = tagData.MusicBrainzID
		t.Logf("  Artist      : %q", tagArtist)
		t.Logf("  AlbumArtist : %q", tagAlbumArtist)
		t.Logf("  Album       : %q", tagAlbum)
		t.Logf("  MusicBrainzID: %q", tagMBID)
		t.Logf("  Title       : %q", tagData.Title)
		t.Logf("  Year        : %d", tagData.Year)
		t.Logf("  TrackNumber : %d", tagData.TrackNumber)
	} else {
		t.Logf("  (no tags found)")
	}

	// ── 4. Parse first filename ───────────────────────────────────────────────
	firstTrack := parser_v2.ParseAudioFilename(files[0])
	t.Logf("\n=== First filename parse (%s) ===", filepath.Base(files[0]))
	t.Logf("  Artist : %q", firstTrack.Artist)
	t.Logf("  Album  : %q", firstTrack.Album)
	t.Logf("  Title  : %q", firstTrack.Title)
	t.Logf("  Track# : %d", firstTrack.TrackNumber)

	// ── 5. Collect & enrich all tracks ───────────────────────────────────────
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
	t.Logf("\n=== Target MB release: %s ===", lsMBRelease)
	details, err := mbProvider.GetReleaseByID(ctx, lsMBRelease)
	if err != nil || details == nil {
		t.Logf("  ERROR fetching release: %v", err)
	} else {
		t.Logf("  Title   : %q", details.Title)
		var detailArtistNames []string
		for _, a := range details.Artists {
			detailArtistNames = append(detailArtistNames, a.Name)
		}
		t.Logf("  Artists : %v", detailArtistNames)
		t.Logf("  Tracks  : %d", details.TrackCount)
		t.Logf("  Year    : %d", details.ReleaseYear)
		t.Logf("  Country : %s", details.Country)
		t.Logf("  Format  : %s", details.Format)
		var mbTotalMs int64
		for i, tr := range details.Tracks {
			mbTotalMs += tr.Duration.Milliseconds()
			t.Logf("  [%02d] disc=%d track=%02d runtime=%6dms (%.1fs) title=%q",
				i+1, tr.DiscNumber, tr.TrackNumber, tr.Duration.Milliseconds(),
				tr.Duration.Seconds(), tr.Title)
		}
		t.Logf("  MB total runtime: %dms (%.1fs)", mbTotalMs, float64(mbTotalMs)/1000)
	}

	// ── 8. Search MB for artist/album combinations ────────────────────────────
	// Mirror the search pairs matchMusicFolder would build
	type searchQ struct {
		label, query string
	}
	artist := coalesceStr(tagAlbumArtist, tagArtist, folderArtist, firstTrack.Artist)
	albumTitle := coalesceStr(tagAlbum, folderAlbum, firstTrack.Album)

	t.Logf("\n=== Resolved artist/album for search ===")
	t.Logf("  artist     : %q", artist)
	t.Logf("  albumTitle : %q", albumTitle)

	queries := []searchQ{
		{"album+artist", fmt.Sprintf(`release:"%s" AND artist:"%s"`, albumTitle, artist)},
		{"album only", fmt.Sprintf(`release:"%s"`, albumTitle)},
		{"folderAlbum+folderArtist", fmt.Sprintf(`release:"%s" AND artist:"%s"`, folderAlbum, folderArtist)},
		{"tagAlbum+tagArtist", fmt.Sprintf(`release:"%s" AND artist:"%s"`, tagAlbum, tagArtist)},
		{"tagAlbum+tagAlbumArtist", fmt.Sprintf(`release:"%s" AND artist:"%s"`, tagAlbum, tagAlbumArtist)},
	}
	for _, q := range queries {
		if q.query == ` AND artist:""` || strings.TrimSpace(q.query) == "" {
			continue
		}
		t.Logf("\n--- MB search: %s ---", q.label)
		t.Logf("  Query: %s", q.query)
		results, _, serr := mbProvider.SearchReleases(ctx, q.query, 10, 0)
		if serr != nil {
			t.Logf("  ERROR: %v", serr)
			continue
		}
		t.Logf("  Results: %d", len(results))
		for i, r := range results {
			targetMark := ""
			if r.MusicBrainzID == lsMBRelease {
				targetMark = " *** TARGET ***"
			}
			var rArtists []string
			for _, a := range r.Artists {
				rArtists = append(rArtists, a.Name)
			}
			t.Logf("  [%d] %q | artist=%v | tracks=%d | year=%d | id=%s%s",
				i+1, r.Title, rArtists, r.TrackCount, r.ReleaseYear, r.MusicBrainzID, targetMark)
		}
	}

	// ── 9. Run albumMatchDistance against the target as a synthetic candidate ─
	if details != nil {
		t.Logf("\n=== albumMatchDistance against target release ===")

		var artistNames []string
		for _, a := range details.Artists {
			artistNames = append(artistNames, a.Name)
		}
		candidate := &database.AlbumSearchResult{
			ID:                   0, // not in DB
			MusicBrainzReleaseID: lsMBRelease,
			Title:                details.Title,
			Artist:               strings.Join(artistNames, ", "),
			TotalTracks:          details.TrackCount,
			Year:                 details.ReleaseYear,
		}

		data := &config.MediaDataConfig{} // default (no AllowMissingTracks etc.)
		dist := albumMatchDistance(candidate, artist, albumTitle, tagMBID, "", "", year, fileCount, data)
		t.Logf("  albumMatchDistance: %.4f  (threshold strongRec=%.2f mediumRec=%.2f)",
			dist, strongRecThresh, mediumRecThresh)

		// ── 10. Run track matching ────────────────────────────────────────────
		if len(details.Tracks) > 0 {
			t.Logf("\n=== Track matching ===")
			dbTracks := mbTracksToDbTracks(details.Tracks)
			isVA := DetectVA(artist, tracks)
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

			// Full distance
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
