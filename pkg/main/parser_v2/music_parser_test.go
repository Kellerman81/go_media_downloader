package parser_v2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listAudioFilesIn returns all audio file paths in a directory (non-recursive).
func listAudioFilesIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	audioExts := map[string]bool{
		".mp3": true, ".m4a": true, ".m4b": true, ".flac": true,
		".ogg": true, ".opus": true, ".wav": true, ".wma": true,
		".aac": true, ".alac": true, ".ape": true,
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			if audioExts[strings.ToLower(filepath.Ext(e.Name()))] {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	return files
}

// countAudioFilesIn returns the number of audio files in a directory (non-recursive).
func countAudioFilesIn(dir string) int {
	return len(listAudioFilesIn(dir))
}

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

func TestMusicParser_ParseAlbum(t *testing.T) {
	mp := NewMusicParser()

	tests := []struct {
		name    string
		dirName string
		want    struct {
			artist      string
			album       string
			year        int
			releaseType string
		}
	}{
		{
			name:    "Standard album format",
			dirName: "Pink Floyd - The Dark Side of the Moon (1973) [FLAC]",
			want: struct {
				artist      string
				album       string
				year        int
				releaseType string
			}{
				artist: "Pink Floyd",
				album:  "The Dark Side of the Moon",
				year:   1973,
			},
		},
		{
			name:    "Album with release type",
			dirName: "Radiohead - OK Computer [Album] (1997)",
			want: struct {
				artist      string
				album       string
				year        int
				releaseType string
			}{
				artist:      "Radiohead",
				year:        1997,
				releaseType: "album",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &MusicParseResult{}
			mp.parseAlbumName(tt.dirName, result)

			if tt.want.artist != "" && result.Artist != tt.want.artist {
				t.Errorf("Artist = %q, want %q", result.Artist, tt.want.artist)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
			if tt.want.releaseType != "" && result.ReleaseType != tt.want.releaseType {
				t.Errorf("ReleaseType = %q, want %q", result.ReleaseType, tt.want.releaseType)
			}
		})
	}
}

func BenchmarkMusicParser_Parse(b *testing.B) {
	mp := NewMusicParser()
	filename := "01 - Comfortably Numb.flac"

	for b.Loop() {
		mp.Parse(filename)
	}
}

func TestMusicParser_ParseAlbumTitle(t *testing.T) {
	mp := NewMusicParser()

	tests := []struct {
		name  string
		title string
		want  struct {
			artist string
			album  string
			year   int
			format string
		}
	}{
		{
			name:  "Standard format with spaces",
			title: "Alabama Shakes - At The Loveless Barn (2014) FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Alabama Shakes",
				album:  "At The Loveless Barn",
				year:   2014,
			},
		},
		{
			name:  "Scene release format",
			title: "Alan Menken And Howard Ashman-Little Shop Of Horrors-OST-CD-FLAC-1986-KINDA-AUD",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Alan Menken And Howard Ashman",
				album:  "Little Shop Of Horrors",
				year:   1986,
			},
		},
		{
			name:  "Scene release with WEB source",
			title: "Howard Ashman and Alan Menken-Little Shop Of Horrors-The New Off-Broadway Cast Album-OST-16BIT-WEB-FLAC-2019-KINDA",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				// Note: When multiple artists are detected (via "and"), Artist is set to the first one
				// The full artist string is stored in AlbumArtist
				artist: "Howard Ashman",
				album:  "Little Shop Of Horrors The New Off Broadway Cast Album",
				year:   2019,
			},
		},
		{
			name:  "Multiple artists with ampersand",
			title: "Daft Punk & Pharrell Williams - Get Lucky (2013) MP3",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Daft Punk",
				album:  "Get Lucky",
				year:   2013,
			},
		},
		{
			name:  "Album with year in brackets",
			title: "Radiohead - OK Computer [1997] FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Radiohead",
				album:  "OK Computer",
				year:   1997,
			},
		},
		{
			name:  "Scene release with underscore",
			title: "Pink_Floyd-The_Dark_Side_of_the_Moon-REMASTERED-CD-FLAC-2011-GROUP",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Pink Floyd",
				album:  "The Dark Side of the Moon",
				year:   2011,
			},
		},
		{
			name:  "Simple format no year",
			title: "The Beatles - Abbey Road FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "The Beatles",
				album:  "Abbey Road",
				year:   0,
			},
		},
		{
			name:  "24-bit hi-res release",
			title: "Miles Davis - Kind of Blue (1959) 24BIT-96KHZ FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Miles Davis",
				album:  "Kind of Blue",
				year:   1959,
			},
		},
		{
			name:  "Scene format with dots as separators",
			title: "Alan.Silvestri-Predator.2-OST-1990-EOS",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Alan Silvestri",
				album:  "Predator 2",
				year:   1990,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mp.ParseAlbumTitle(tt.title)

			if result.Artist != tt.want.artist {
				t.Errorf("Artist = %q, want %q", result.Artist, tt.want.artist)
			}
			if result.Album != tt.want.album {
				t.Errorf("Album = %q, want %q", result.Album, tt.want.album)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
		})
	}
}

func TestRegression_MusicTrackDisc(t *testing.T) {
	mp := NewMusicParser()
	tests := []struct {
		input string
		track int
		disc  int
		title string
	}{
		{"01 - Cloud 9.flac", 1, 0, "Cloud 9"},
		{"05 - Road 66.mp3", 5, 0, "Road 66"},
		{"03 - Studio 54.flac", 3, 0, "Studio 54"},
		{"01 - Song (Disc 2).flac", 1, 2, "Song"},
		{"01 - Song d2.flac", 1, 2, "Song"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tr := mp.Parse(tt.input).Tracks[0]
			if tr.TrackNumber != tt.track || tr.DiscNumber != tt.disc || tr.Title != tt.title {
				t.Errorf("got track=%d disc=%d title=%q, want track=%d disc=%d title=%q",
					tr.TrackNumber, tr.DiscNumber, tr.Title, tt.track, tt.disc, tt.title)
			}
		})
	}
}

func TestRegression_MusicAlbumTags(t *testing.T) {
	mp := NewMusicParser()
	tests := []struct {
		input       string
		artist      string
		album       string
		releaseType string
		format      string
	}{
		{"Magnum Opus", "", "Magnum Opus", "", ""},
		{"Paul McCartney - Live and Let Die", "Paul McCartney", "Live and Let Die", "", ""},
		{"Artist - Album (Live) FLAC", "Artist", "Album (Live)", "live", "flac"},
		{"Artist - Album [FLAC]", "Artist", "Album", "", "flac"},
		{"Artist - Album OPUS", "Artist", "Album", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r := mp.ParseAlbumTitle(tt.input)
			if r.Artist != tt.artist || r.Album != tt.album || r.ReleaseType != tt.releaseType ||
				r.Format != tt.format {
				t.Errorf("got artist=%q album=%q type=%q format=%q, want %q %q %q %q",
					r.Artist, r.Album, r.ReleaseType, r.Format,
					tt.artist, tt.album, tt.releaseType, tt.format)
			}
		})
	}
}
