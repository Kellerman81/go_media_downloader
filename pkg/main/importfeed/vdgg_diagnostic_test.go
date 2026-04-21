package importfeed

// TestVDGGDiagnostic diagnoses why
//
//	Q:\_tocheck_Music\wrong_runtime\Van Der Graaf Generator-1977-The Quiet Zone, The Pleasure Dome NMR\
//
// is denied as wrong_runtime instead of being matched to a 9-track MB release.
// Release group: https://musicbrainz.org/release-group/4bb9cda8-0742-30ff-ae73-955c6860fc25
//
// Run with: go test -v -run TestVDGGDiagnostic ./importfeed/

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

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
	t.Logf("\n=== Local: %d tracks, %dms (%.1fs) ===", fileCount, localTotalMs, float64(localTotalMs)/1000)

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
		{"production query (BuildArtistAlbumSearch)", string(BuildArtistAlbumSearch(albumTitle, artist))},
		// fuzzy fallback (new code, only fires when primary returns 0):
		{"fuzzy fallback", func() string {
			buf := logger.PlAddBuffer.Get()
			defer logger.PlAddBuffer.Put(buf)
			buf.WriteString(`release:"`)
			buf.WriteString(LuceneEscape(albumTitle))
			buf.WriteString(`"~2`)
			if artist != "" && !IsVariousArtists(artist) {
				buf.WriteString(` AND artist:"`)
				buf.WriteString(LuceneEscape(artist))
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
				t.Logf("       → MB has no durations: FAIL-OPEN → WOULD IMPORT (id=%s)", r.MusicBrainzID)
			} else if diff <= toleranceMs {
				t.Logf("       → runtime OK (diff=%dms <= tol=%dms) → WOULD IMPORT (id=%s)", diff, toleranceMs, r.MusicBrainzID)
			} else {
				t.Logf("       → SKIP: runtime diff %dms > tol %dms (mbRuntime=%dms)", diff, toleranceMs, relRuntimeMs)
			}
		}
	}
}
