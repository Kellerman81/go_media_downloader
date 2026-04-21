package parser_v2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoMatchMusicDiagnostic parses a sample of folder names from the
// no_match music folder and reports what the parser extracts.
// Purpose: understand why matching fails so the parser can be improved.
//
// Run with: go test -v -run TestNoMatchMusicDiagnostic ./parser_v2/
const noMatchMusicDir = `Q:\_tocheck_Music\no_match`

// musicDiagResult holds parsed data for a single no_match music folder.
type musicDiagResult struct {
	folder     string
	artist     string
	album      string
	year       int
	confidence float64
	issues     []string
}

func TestNoMatchMusicDiagnostic(t *testing.T) {
	entries, err := os.ReadDir(noMatchMusicDir)
	if err != nil {
		t.Skipf("Cannot read %s: %v (skip if drive not mounted)", noMatchMusicDir, err)
	}

	mp := NewMusicParser()

	var results []musicDiagResult
	var noArtistCount, noAlbumCount, lowConfCount int

	limit := maxDiagnosticSamples
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if limit <= 0 {
			break
		}
		limit--

		name := e.Name()
		pr := mp.ParseAlbumTitle(name)

		var issues []string
		if pr.Artist == "" {
			issues = append(issues, "NO_ARTIST")
			noArtistCount++
		}
		if pr.Album == "" {
			issues = append(issues, "NO_ALBUM")
			noAlbumCount++
		}
		if pr.Confidence < 0.25 {
			issues = append(issues, fmt.Sprintf("LOW_CONF(%.2f)", pr.Confidence))
			lowConfCount++
		}
		// Detect when full folder name was kept as album unchanged
		cleanedFolder := strings.ReplaceAll(name, "_", " ")
		if strings.EqualFold(strings.TrimSpace(pr.Album), strings.TrimSpace(cleanedFolder)) {
			issues = append(issues, "ALBUM=FULL_FOLDER_NAME")
		}
		// Detect special/weird prefixes
		if len(name) > 0 && (name[0] == '(' || name[0] == '[' || name[0] == ' ' || name[0] == '-') {
			issues = append(issues, "WEIRD_PREFIX")
		}

		results = append(results, musicDiagResult{
			folder:     name,
			artist:     pr.Artist,
			album:      pr.Album,
			year:       pr.Year,
			confidence: pr.Confidence,
			issues:     issues,
		})
	}

	// Print report
	t.Logf("\n=== MUSIC NO_MATCH DIAGNOSTIC (%d samples) ===\n", len(results))
	t.Logf("%-50s | %-25s | %-25s | %-6s | %s",
		"FOLDER (truncated)", "ARTIST", "ALBUM", "CONF", "ISSUES")
	t.Logf("%s", strings.Repeat("-", 140))

	for _, r := range results {
		folder := r.folder
		if len(folder) > 50 {
			folder = folder[:47] + "..."
		}
		artist := r.artist
		if len(artist) > 25 {
			artist = artist[:22] + "..."
		}
		album := r.album
		if len(album) > 25 {
			album = album[:22] + "..."
		}
		t.Logf("%-50s | %-25s | %-25s | %.2f   | %s",
			folder, artist, album, r.confidence, strings.Join(r.issues, ", "))
	}

	t.Logf("\n=== SUMMARY ===")
	t.Logf("  Samples:    %d", len(results))
	t.Logf("  No artist:  %d (%.0f%%)", noArtistCount, pct(noArtistCount, len(results)))
	t.Logf("  No album:   %d (%.0f%%)", noAlbumCount, pct(noAlbumCount, len(results)))
	t.Logf("  Low conf:   %d (%.0f%%)", lowConfCount, pct(lowConfCount, len(results)))

	t.Logf("\n=== PATTERN CATEGORIES ===")
	categorizeMusicFolders(t, results)
}

// categorizeMusicFolders identifies common naming patterns that confuse the parser.
func categorizeMusicFolders(t *testing.T, results []musicDiagResult) {
	var (
		weirdPrefix    int // starts with (, [, space, -, special chars
		artistDash     int // "Artist - Album (Year)"
		nzbStyle       int // (-_-) TOP40 = _filename_ style
		plainAlbum     int // no artist separator at all
		numberedSeries int // numbered folders
	)

	for _, r := range results {
		f := r.folder
		switch {
		case strings.HasPrefix(f, "(-_-)") || strings.HasPrefix(f, "(-") || strings.Contains(f, "= _"):
			nzbStyle++
		case len(f) > 0 && (f[0] == '(' || f[0] == '[' || f[0] == ' ' || f[0] == '-'):
			weirdPrefix++
		case len(f) > 2 && f[0] >= '0' && f[0] <= '9':
			numberedSeries++
		case strings.Contains(f, " - ") && r.artist != "":
			artistDash++
		default:
			plainAlbum++
		}
	}

	t.Logf("  NZB-style ((-_-), = _..._):   %d", nzbStyle)
	t.Logf("  Weird/special prefix:          %d", weirdPrefix)
	t.Logf("  Numbered series:               %d", numberedSeries)
	t.Logf("  Artist - Album (detected):     %d", artistDash)
	t.Logf("  Plain album / other:           %d", plainAlbum)
}

// TestNoMatchMusicDetailedSamples shows full parser + tag output and simulates
// the matchMusicFolder search-pair logic for the first 10 no_match folders.
//
// Run with: go test -v -run TestNoMatchMusicDetailedSamples ./parser_v2/
func TestNoMatchMusicDetailedSamples(t *testing.T) {
	diagMusicSamples(t, 0, 10)
}

// TestNoMatchMusicMiddleSamples is the same as TestNoMatchMusicDetailedSamples
// but starts from the middle of the folder list (~folder 200 of 399).
// Run with: go test -v -run TestNoMatchMusicMiddleSamples ./parser_v2/
func TestNoMatchMusicMiddleSamples(t *testing.T) {
	diagMusicSamples(t, 195, 10)
}

// diagMusicSamples is the shared detailed diagnostic logic for music folders.
// offset = how many dirs to skip before sampling; n = how many to show.
func diagMusicSamples(t *testing.T, offset, n int) {
	entries, err := os.ReadDir(noMatchMusicDir)
	if err != nil {
		t.Skipf("Cannot read %s: %v (skip if drive not mounted)", noMatchMusicDir, err)
	}

	mp := NewMusicParser()

	// Collect only directories
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}

	if offset >= len(dirs) {
		t.Skipf("offset %d >= total folders %d", offset, len(dirs))
	}

	t.Logf("\n=== DETAILED MUSIC PARSE RESULTS (folders %d–%d of %d) ===\n",
		offset+1, min2(offset+n, len(dirs)), len(dirs))

	for i := offset; i < len(dirs) && i < offset+n; i++ {
		e := dirs[i]
		name := e.Name()
		pr := mp.ParseAlbumTitle(name)

		folderArtist := pr.Artist
		folderAlbum := pr.Album

		// Raw first-hyphen split (same as matchMusicFolder)
		rawArtist, rawAlbum := rawHyphenSplit(name, folderArtist)

		// Get tags from first audio file
		subPath := filepath.Join(noMatchMusicDir, name)
		audioFiles := listAudioFilesIn(subPath)
		audioCount := len(audioFiles)

		var tagArtist, tagAlbumArtist, tagAlbum string
		if audioCount > 0 {
			tagData := readTagsForFirstFileCtx(t.Context(), audioFiles)
			if tagData != nil {
				tagArtist = tagData.Artist
				tagAlbumArtist = tagData.AlbumArtist
				tagAlbum = tagData.Album
			}
		}

		strippedEp := simulateStripEpisodePrefix(folderAlbum)

		anyArtist := folderArtist != "" || tagArtist != "" || tagAlbumArtist != "" ||
			rawArtist != ""

		num := i - offset + 1
		t.Logf("[%2d] #%-4d Folder: %q", num, i+1, name)
		t.Logf(
			"      folder→  artist=%q  album=%q  conf=%.2f",
			folderArtist,
			folderAlbum,
			pr.Confidence,
		)
		t.Logf("      year: %d  mbid: %q", pr.Year, pr.MusicBrainzReleaseID)
		t.Logf("      raw→     artist=%q  album=%q", rawArtist, rawAlbum)
		t.Logf(
			"      tags→    artist=%q  albumArtist=%q  album=%q",
			tagArtist,
			tagAlbumArtist,
			tagAlbum,
		)
		t.Logf("      files: %d", audioCount)
		t.Logf("      strip(folder): %q", strippedEp)

		if anyArtist {
			firedPairs := simulateMusicSearchPairs(
				folderArtist, folderAlbum,
				rawArtist, rawAlbum,
				tagArtist, tagAlbumArtist, tagAlbum,
				strippedEp,
			)
			if len(firedPairs) > 0 {
				t.Logf("      search pairs that fire:")
				for _, p := range firedPairs {
					t.Logf("        ✓ artist=%q  album=%q  [%s]", p[0], p[1], p[2])
				}
			} else {
				t.Logf("      ✗ No valid pairs despite having artist sources")
			}
		} else {
			t.Logf("      ✗ ALL artists empty — addPair calls SKIPPED")
			if folderAlbum != "" {
				t.Logf("      ⚠ title-only fallback needed for %q", folderAlbum)
			}
		}

		if strippedEp != "" {
			if idx := strings.Index(strippedEp, " - "); idx > 0 {
				t.Logf("      ✓ re-parse stripped: artist=%q  album=%q  [stripped-ep-reparsed]",
					strings.TrimSpace(strippedEp[:idx]), strings.TrimSpace(strippedEp[idx+3:]))
			}
		}
		t.Logf("")
	}
}

// simulateMusicSearchPairs returns the non-empty (artist, album, source) triples
// that matchMusicFolder's addPair would actually fire for the given inputs.
func simulateMusicSearchPairs(folderArtist, folderAlbum, rawArtist, rawAlbum,
	tagArtist, tagAlbumArtist, tagAlbum, strippedEp string,
) [][3]string {
	seen := make(map[string]bool)
	var pairs [][3]string

	add := func(a, al, src string) {
		if a == "" || al == "" {
			return
		}
		key := strings.ToLower(a) + "|" + strings.ToLower(al)
		if !seen[key] {
			seen[key] = true
			pairs = append(pairs, [3]string{a, al, src})
		}
	}

	add(folderArtist, folderAlbum, "folder")
	add(rawArtist, rawAlbum, "raw-folder")
	add(rawAlbum, rawArtist, "raw-folder-swapped")
	add(tagAlbumArtist, tagAlbum, "tag-albumartist")
	add(tagArtist, tagAlbum, "tag-artist")
	add(tagAlbumArtist, folderAlbum, "tag-albumartist+folder")
	add(tagArtist, folderAlbum, "tag-artist+folder")

	// stripEpisodePrefix pairs (now implemented in matchMusicFolder):
	add(folderArtist, strippedEp, "folder+stripped-ep")
	add(tagArtist, strippedEp, "tag-artist+stripped-ep")
	add(tagAlbumArtist, strippedEp, "tag-albumartist+stripped-ep")
	// re-parse stripped " - " as "Artist - Album"
	if idx := strings.Index(strippedEp, " - "); idx > 0 {
		potArtist := strings.TrimSpace(strippedEp[:idx])
		potAlbum := strings.TrimSpace(strippedEp[idx+3:])
		add(potArtist, potAlbum, "stripped-ep-reparsed")
		add(tagArtist, potAlbum, "tag-artist+stripped-ep-reparsed")
		add(tagAlbumArtist, potAlbum, "tag-albumartist+stripped-ep-reparsed")
	}

	return pairs
}

// TestNoMatchMusicNZBStyleSamples specifically examines the NZB/Usenet-style
// folder names like "(-_-) TOP 40_International_1989 = _filename.mp3_ Group".
// Run with: go test -v -run TestNoMatchMusicNZBStyleSamples ./parser_v2/
func TestNoMatchMusicNZBStyleSamples(t *testing.T) {
	entries, err := os.ReadDir(noMatchMusicDir)
	if err != nil {
		t.Skipf("Cannot read %s: %v (skip if drive not mounted)", noMatchMusicDir, err)
	}

	mp := NewMusicParser()
	nzb := NewNZBPreprocessor()

	t.Logf("\n=== NZB/USENET-STYLE MUSIC FOLDER DIAGNOSTIC ===\n")
	t.Logf("These folders appear to contain raw Usenet subject lines as folder names.\n")
	t.Logf("Note: These are SINGLE-TRACK folders — they contain one mp3/file per folder.")
	t.Logf("They are fundamentally unmatchable as albums (no album structure).\n")

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()

		// Only process NZB-style names
		isNZBStyle := strings.HasPrefix(name, "(-") ||
			strings.Contains(name, "= _") ||
			strings.Contains(name, "yEnc")
		if !isNZBStyle {
			continue
		}
		if count >= 15 {
			break
		}
		count++

		// First try: parse raw folder name
		rawPR := mp.ParseAlbumTitle(name)

		// Second try: NZB-clean then parse
		cleaned := nzb.Clean(name)
		cleanedPR := mp.ParseAlbumTitle(cleaned)

		subPath := filepath.Join(noMatchMusicDir, name)
		audioCount := countAudioFilesIn(subPath)

		t.Logf("[%2d] Raw:     %q", count, name)
		t.Logf("     Cleaned: %q", cleaned)
		t.Logf("     Files:   %d  (single-track = not a matchable album)", audioCount)
		t.Logf("     Raw→     artist=%q  album=%q  conf=%.2f",
			rawPR.Artist, rawPR.Album, rawPR.Confidence)
		t.Logf("     Cleaned→ artist=%q  album=%q  conf=%.2f",
			cleanedPR.Artist, cleanedPR.Album, cleanedPR.Confidence)
		if audioCount == 1 {
			t.Logf("     ℹ Single-track folder — should be filtered before album matching")
		}
		t.Logf("")
	}

	if count == 0 {
		t.Log("No NZB-style folder names found in sample.")
	} else {
		t.Logf("Processed %d NZB-style folder names.", count)
	}
}
