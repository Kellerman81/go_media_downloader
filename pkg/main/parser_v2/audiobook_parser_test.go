package parser_v2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAudiobookRuntimeHelpers(t *testing.T) {
	ap := NewAudiobookParser()
	res := &AudiobookParseResult{Files: []AudiobookFileInfo{
		{Filename: "01.mp3", PartNumber: 1}, {Filename: "02.mp3", PartNumber: 2}, {Filename: "03.mp3", PartNumber: 3},
	}}

	ap.UpdateFileRuntimes(res, map[string]int64{"01.mp3": 600_000, "02.mp3": 400_000})

	if res.RuntimeMS != 1_000_000 || res.Files[0].RuntimeMS != 600_000 || res.Files[2].RuntimeMS != 0 {
		t.Errorf("runtimes = %d %+v", res.RuntimeMS, res.Files)
	}

	tracks := filesToTracks(res.Files)
	if len(tracks) != 3 || tracks[1].TrackNumber != 2 || tracks[1].RuntimeMS != 400_000 {
		t.Errorf("filesToTracks = %+v", tracks)
	}

	if ok, _ := ap.MatchByRuntime(1_000_000, res.Files); !ok {
		t.Error("exact total runtime not matched")
	}

	if ok, _ := ap.MatchByRuntime(2_000_000, res.Files); ok {
		t.Error("double runtime matched")
	}
}

// TestNoMatchAudiobookDiagnostic parses a sample of folder names from the
// no_match audiobook folder and reports what the parser extracts.
// Purpose: understand why matching fails so the parser can be improved.
//
// Run with: go test -v -run TestNoMatchAudiobookDiagnostic ./parser_v2/
const noMatchAudiobookDir = `Q:\_tocheck_Audiobook\no_match`

// maxDiagnosticSamples limits how many folders we inspect per run.
const maxDiagnosticSamples = 30

// audiobookDiagResult holds parsed data for a single no_match audiobook folder.
type audiobookDiagResult struct {
	folder     string
	title      string
	author     string
	asin       string
	year       int
	series     string
	confidence float64
	issues     []string
}

func TestNoMatchAudiobookDiagnostic(t *testing.T) {
	entries, err := os.ReadDir(noMatchAudiobookDir)
	if err != nil {
		t.Skipf("Cannot read %s: %v (skip if drive not mounted)", noMatchAudiobookDir, err)
	}

	ap := NewAudiobookParser()

	var results []audiobookDiagResult
	var noTitleCount, noAuthorCount, lowConfCount int

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
		pr := ap.Parse(name)

		var issues []string
		if pr.Title == "" {
			issues = append(issues, "NO_TITLE")
			noTitleCount++
		}
		if pr.Author == "" {
			issues = append(issues, "NO_AUTHOR")
			noAuthorCount++
		}
		if pr.Confidence < 0.35 {
			issues = append(issues, fmt.Sprintf("LOW_CONF(%.2f)", pr.Confidence))
			lowConfCount++
		}
		// Detect when full folder name was kept as title unchanged (parser gave up)
		cleanedFolder := strings.ReplaceAll(name, "_", " ")
		if strings.EqualFold(strings.TrimSpace(pr.Title), strings.TrimSpace(cleanedFolder)) {
			issues = append(issues, "TITLE=FULL_FOLDER_NAME")
		}
		// Detect numeric prefix left in title (e.g., "001 - ...")
		if len(pr.Title) > 0 && pr.Title[0] >= '0' && pr.Title[0] <= '9' {
			issues = append(issues, "TITLE_STARTS_WITH_DIGIT")
		}

		results = append(results, audiobookDiagResult{
			folder:     name,
			title:      pr.Title,
			author:     pr.Author,
			asin:       pr.ASIN,
			year:       pr.Year,
			series:     pr.Series,
			confidence: pr.Confidence,
			issues:     issues,
		})
	}

	// Print report
	t.Logf("\n=== AUDIOBOOK NO_MATCH DIAGNOSTIC (%d samples) ===\n", len(results))
	t.Logf("%-50s | %-30s | %-20s | %-6s | %s",
		"FOLDER (truncated)", "TITLE", "AUTHOR", "CONF", "ISSUES")
	t.Logf("%s", strings.Repeat("-", 130))

	for _, r := range results {
		folder := r.folder
		if len(folder) > 50 {
			folder = folder[:47] + "..."
		}
		title := r.title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		author := r.author
		if len(author) > 20 {
			author = author[:17] + "..."
		}
		t.Logf("%-50s | %-30s | %-20s | %.2f   | %s",
			folder, title, author, r.confidence, strings.Join(r.issues, ", "))
	}

	t.Logf("\n=== SUMMARY ===")
	t.Logf("  Samples:    %d", len(results))
	t.Logf("  No title:   %d (%.0f%%)", noTitleCount, pct(noTitleCount, len(results)))
	t.Logf("  No author:  %d (%.0f%%)", noAuthorCount, pct(noAuthorCount, len(results)))
	t.Logf("  Low conf:   %d (%.0f%%)", lowConfCount, pct(lowConfCount, len(results)))

	// Categorise pattern types in the folder names
	t.Logf("\n=== PATTERN CATEGORIES ===")
	categorizeAudiobookFolders(t, results)
}

// categorizeAudiobookFolders attempts to identify common naming patterns.
func categorizeAudiobookFolders(t *testing.T, results []audiobookDiagResult) {
	var (
		numberedPrefix  int // "001 - Title"
		authorDashTitle int // "Author - Title (Year)"
		isbnOrASIN      int // contains ISBN/ASIN
		weirdPrefix     int // starts with special chars
		plainTitle      int // just a plain title
	)

	for _, r := range results {
		f := r.folder
		switch {
		case len(f) > 2 && f[0] >= '0' && f[0] <= '9':
			numberedPrefix++
		case strings.Contains(f, " - ") && r.author != "":
			authorDashTitle++
		case strings.Contains(f, "B0") || strings.Contains(f, "978") || strings.Contains(f, "979"):
			isbnOrASIN++
		case len(f) > 0 && (f[0] < 'A' || f[0] > 'z'):
			weirdPrefix++
		default:
			plainTitle++
		}
	}

	t.Logf("  Numbered prefix (001 - ...):  %d", numberedPrefix)
	t.Logf("  Author - Title detected:      %d", authorDashTitle)
	t.Logf("  ISBN/ASIN present:            %d", isbnOrASIN)
	t.Logf("  Weird/special prefix:         %d", weirdPrefix)
	t.Logf("  Plain title:                  %d", plainTitle)
}

// TestNoMatchAudiobookDetailedSamples shows full parser + tag output and simulates
// the matchAudiobookFolder search-pair logic for the first 10 no_match folders.
//
// This test reveals exactly WHY each folder fails to match:
//   - What the folder-name parser extracts
//   - What file tags contain
//   - Which addPair combinations would actually fire (non-empty pairs)
//   - Which pairs are silently skipped (empty artist)
//   - Whether a title-only search would have helped
//
// Run with: go test -v -run TestNoMatchAudiobookDetailedSamples ./parser_v2/
func TestNoMatchAudiobookDetailedSamples(t *testing.T) {
	diagAudiobookSamples(t, 0, 10)
}

// TestNoMatchAudiobookMiddleSamples is the same as TestNoMatchAudiobookDetailedSamples
// but starts from the middle of the folder list (~folder 1023 of 2046).
// Run with: go test -v -run TestNoMatchAudiobookMiddleSamples ./parser_v2/
func TestNoMatchAudiobookMiddleSamples(t *testing.T) {
	diagAudiobookSamples(t, 1020, 10)
}

// diagAudiobookSamples is the shared detailed diagnostic logic for audiobook folders.
// offset = how many dirs to skip before sampling; n = how many to show.
func diagAudiobookSamples(t *testing.T, offset, n int) {
	entries, err := os.ReadDir(noMatchAudiobookDir)
	if err != nil {
		t.Skipf("Cannot read %s: %v (skip if drive not mounted)", noMatchAudiobookDir, err)
	}

	ap := NewAudiobookParser()

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

	t.Logf("\n=== DETAILED AUDIOBOOK PARSE RESULTS (folders %d–%d of %d) ===\n",
		offset+1, min2(offset+n, len(dirs)), len(dirs))

	for i := offset; i < len(dirs) && i < offset+n; i++ {
		e := dirs[i]
		name := e.Name()
		pr := ap.Parse(name)

		folderArtist := pr.Author
		folderAlbum := pr.Title
		asin := pr.ASIN

		rawArtist, rawAlbum := rawHyphenSplit(name, folderArtist)

		subPath := filepath.Join(noMatchAudiobookDir, name)
		audioFiles := listAudioFilesIn(subPath)
		audioCount := len(audioFiles)

		var tagArtist, tagAlbumArtist, tagAlbum, tagASIN string
		if audioCount > 0 {
			tagData := readTagsForFirstFileCtx(t.Context(), audioFiles)
			if tagData != nil {
				tagArtist = tagData.Artist
				tagAlbumArtist = tagData.AlbumArtist
				tagAlbum = tagData.Album
				tagASIN = tagData.ASIN
				if asin == "" {
					asin = tagASIN
				}
			}
		}

		strippedEp := simulateStripEpisodePrefix(folderAlbum)
		strippedTag := simulateStripEpisodePrefix(tagAlbum)

		allArtists := map[string]string{
			"folderArtist":   folderArtist,
			"tagArtist":      tagArtist,
			"tagAlbumArtist": tagAlbumArtist,
			"rawArtist":      rawArtist,
		}
		anyArtist := false
		for _, a := range allArtists {
			if a != "" {
				anyArtist = true
				break
			}
		}

		num := i - offset + 1
		t.Logf("[%2d] #%-4d Folder: %q", num, i+1, name)
		t.Logf("      ASIN: %q", asin)
		t.Logf(
			"      folder→  artist=%q  album=%q  conf=%.2f",
			folderArtist,
			folderAlbum,
			pr.Confidence,
		)
		t.Logf("      raw→     artist=%q  album=%q", rawArtist, rawAlbum)
		t.Logf(
			"      tags→    artist=%q  albumArtist=%q  album=%q",
			tagArtist,
			tagAlbumArtist,
			tagAlbum,
		)
		t.Logf("      files: %d", audioCount)
		t.Logf("      strip(folder): %q   strip(tag): %q", strippedEp, strippedTag)

		if anyArtist {
			// Show the best non-trivial pairs
			bestPairs := bestAudiobookPairs(folderArtist, folderAlbum, rawArtist, rawAlbum,
				tagArtist, tagAlbumArtist, tagAlbum, strippedEp, strippedTag)
			if len(bestPairs) > 0 {
				t.Logf("      search pairs that fire:")
				for _, p := range bestPairs {
					t.Logf("        ✓ author=%q  title=%q  [%s]", p[0], p[1], p[2])
				}
			}
		} else {
			t.Logf(
				"      ✗ all artists empty — addPair skipped, rawArtist=%q (numeric?)",
				rawArtist,
			)
			if strippedEp != "" {
				t.Logf("      ⚠ title-only fallback needed for %q", strippedEp)
			}
		}
		if strippedEp != "" {
			if idx := strings.Index(strippedEp, " - "); idx > 0 {
				t.Logf("      ✓ re-parse stripped: author=%q  title=%q  [stripped-ep-reparsed]",
					strings.TrimSpace(strippedEp[:idx]), strings.TrimSpace(strippedEp[idx+3:]))
			}
		}
		t.Logf("")
	}
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func bestAudiobookPairs(folderArtist, folderAlbum, rawArtist, rawAlbum,
	tagArtist, tagAlbumArtist, tagAlbum, strippedEp, strippedTag string,
) [][3]string {
	seen := make(map[string]bool)
	var pairs [][3]string
	add := func(a, t, src string) {
		if a == "" || t == "" {
			return
		}
		key := strings.ToLower(a) + "|" + strings.ToLower(t)
		if !seen[key] {
			seen[key] = true
			pairs = append(pairs, [3]string{a, t, src})
		}
	}
	add(folderArtist, folderAlbum, "folder")
	add(rawArtist, rawAlbum, "raw")
	add(tagArtist, tagAlbum, "tag-artist+tag-album")
	add(tagAlbumArtist, tagAlbum, "tag-albumartist+tag-album")
	add(tagArtist, folderAlbum, "tag-artist+folder")
	add(tagAlbumArtist, folderAlbum, "tag-albumartist+folder")
	add(tagArtist, strippedEp, "tag-artist+stripped-ep(folder)")
	add(tagAlbumArtist, strippedEp, "tag-albumartist+stripped-ep(folder)")
	add(tagArtist, strippedTag, "tag-artist+stripped-ep(tag)")
	add(tagAlbumArtist, strippedTag, "tag-albumartist+stripped-ep(tag)")
	// re-parse stripped
	for _, s := range []string{strippedEp, strippedTag} {
		if idx := strings.Index(s, " - "); idx > 0 {
			a := strings.TrimSpace(s[:idx])
			ti := strings.TrimSpace(s[idx+3:])
			add(a, ti, "stripped-ep-reparsed")
			add(tagArtist, ti, "tag-artist+stripped-ep-reparsed")
		}
	}
	return pairs
}

// simulateStripEpisodePrefix replicates the stripEpisodePrefix closure from matchAudiobookFolder.
func simulateStripEpisodePrefix(title string) string {
	for i, c := range title {
		if c >= '0' && c <= '9' {
			continue
		}
		if i > 0 {
			rest := strings.TrimLeft(title[i:], " .-_")
			if rest != "" && len(rest) < len(title) {
				return rest
			}
		}
		break
	}
	return ""
}

// rawHyphenSplit replicates the raw first-hyphen split from matchAudiobookFolder.
// Only done when folderArtist is empty.
func rawHyphenSplit(folderBase, folderArtist string) (rawArtist, rawAlbum string) {
	if folderArtist != "" {
		return "", ""
	}
	idx := strings.Index(folderBase, "-")
	if idx <= 0 || idx >= len(folderBase)-1 {
		return "", ""
	}
	rawArtist = strings.TrimSpace(folderBase[:idx])
	rawAlbum = strings.TrimSpace(folderBase[idx+1:])
	rawArtist = strings.ReplaceAll(strings.ReplaceAll(rawArtist, "_", " "), ".", " ")
	rawAlbum = strings.ReplaceAll(strings.ReplaceAll(rawAlbum, "_", " "), ".", " ")
	return
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100.0 / float64(total)
}

func TestAudiobookParser_Parse(t *testing.T) {
	ap := NewAudiobookParser()

	tests := []struct {
		name     string
		filename string
		want     struct {
			title    string
			author   string
			narrator string
			asin     string
			abridged bool
			format   string
		}
	}{
		{
			name:     "Simple audiobook",
			filename: "Stephen King - The Stand - Read by Grover Gardner.m4b",
			want: struct {
				title    string
				author   string
				narrator string
				asin     string
				abridged bool
				format   string
			}{
				author:   "Stephen King",
				narrator: "Grover Gardner",
				format:   "m4b",
			},
		},
		{
			name:     "Audiobook with ASIN",
			filename: "B0BX7F2P6G - Project Hail Mary.m4b",
			want: struct {
				title    string
				author   string
				narrator string
				asin     string
				abridged bool
				format   string
			}{
				asin:   "B0BX7F2P6G",
				format: "m4b",
			},
		},
		{
			name:     "Unabridged audiobook",
			filename: "Andy Weir - The Martian [Unabridged].mp3",
			want: struct {
				title    string
				author   string
				narrator string
				asin     string
				abridged bool
				format   string
			}{
				author:   "Andy Weir",
				abridged: false,
				format:   "mp3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ap.Parse(tt.filename)

			if tt.want.author != "" && result.Author != tt.want.author {
				t.Errorf("Author = %q, want %q", result.Author, tt.want.author)
			}
			if tt.want.narrator != "" && result.Narrator != tt.want.narrator {
				t.Errorf("Narrator = %q, want %q", result.Narrator, tt.want.narrator)
			}
			if tt.want.asin != "" && result.ASIN != tt.want.asin {
				t.Errorf("ASIN = %q, want %q", result.ASIN, tt.want.asin)
			}
			if result.Format != tt.want.format {
				t.Errorf("Format = %q, want %q", result.Format, tt.want.format)
			}
		})
	}
}

func TestAudiobookParser_Parse_NZBTitles(t *testing.T) {
	ap := NewAudiobookParser()

	tests := []struct {
		name  string
		title string
		want  struct {
			author   string
			title    string
			year     int
			narrator string
			series   string
			asin     string
		}
	}{
		{
			name:  "Standard format with spaces",
			title: "Stephen King - The Shining (1977)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Stephen King",
				title:  "The Shining",
				year:   1977,
			},
		},
		{
			name:  "With narrator",
			title: "Brandon Sanderson - Mistborn Read by Michael Kramer (2006)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author:   "Brandon Sanderson",
				title:    "Mistborn",
				year:     2006,
				narrator: "Michael Kramer",
			},
		},
		{
			name:  "With ASIN",
			title: "Terry Pratchett - Guards Guards [B00354ZSS2]",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Terry Pratchett",
				title:  "Guards Guards",
				asin:   "B00354ZSS2",
			},
		},
		{
			name:  "With series info",
			title: "Jim Butcher - Storm Front (Dresden Files Book 1) (2000)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Jim Butcher",
				title:  "Storm Front",
				year:   2000,
				series: "Dresden Files",
			},
		},
		{
			name:  "Scene format with dashes",
			title: "Neil Gaiman-American Gods-Audiobook-MP3-64kbps-2001",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Neil Gaiman",
				title:  "American Gods",
				year:   2001,
			},
		},
		{
			name:  "Lowercase author name",
			title: "stephen king - it (1986)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "stephen king",
				title:  "it",
				year:   1986,
			},
		},
		{
			name:  "Multiple authors",
			title: "Douglas Adams & Eoin Colfer - And Another Thing (2009)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Douglas Adams",
				title:  "And Another Thing",
				year:   2009,
			},
		},
		{
			name:  "Unabridged indicator",
			title: "Frank Herbert - Dune [Unabridged] (1965)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Frank Herbert",
				title:  "Dune",
				year:   1965,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ap.Parse(tt.title)

			if result.Author != tt.want.author {
				t.Errorf("Author = %q, want %q", result.Author, tt.want.author)
			}
			if result.Title != tt.want.title {
				t.Errorf("Title = %q, want %q", result.Title, tt.want.title)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
			if tt.want.narrator != "" && result.Narrator != tt.want.narrator {
				t.Errorf("Narrator = %q, want %q", result.Narrator, tt.want.narrator)
			}
			if tt.want.series != "" && result.Series != tt.want.series {
				t.Errorf("Series = %q, want %q", result.Series, tt.want.series)
			}
			if tt.want.asin != "" && result.ASIN != tt.want.asin {
				t.Errorf("ASIN = %q, want %q", result.ASIN, tt.want.asin)
			}
		})
	}
}

func TestRegression_Audiobook(t *testing.T) {
	ap := NewAudiobookParser()
	tests := []struct {
		input    string
		author   string
		title    string
		year     int
		series   string
		position string
		asin     string
	}{
		{"George Orwell - 1984.m4b", "George Orwell", "1984", 1984, "", "", ""},
		{"Author - Title (2019).m4b", "Author", "Title", 2019, "", "", ""},
		{"Author - Title 2019.m4b", "Author", "Title", 2019, "", "", ""},
		{"J.K. Rowling - Harry Potter [B012345678]", "J.K. Rowling", "Harry Potter", 0, "", "", "B012345678"},
		{"Author - Casino 2", "Author", "Casino 2", 0, "", "", ""},
		{"Author - Piano 3", "Author", "Piano 3", 0, "", "", ""},
		{"Author - Volcano 3", "Author", "Volcano 3", 0, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r := ap.Parse(tt.input)
			if r.Author != tt.author || r.Title != tt.title || r.Year != tt.year ||
				r.Series != tt.series || r.SeriesPosition != tt.position || r.ASIN != tt.asin {
				t.Errorf("got author=%q title=%q year=%d series=%q pos=%q asin=%q",
					r.Author, r.Title, r.Year, r.Series, r.SeriesPosition, r.ASIN)
			}
		})
	}
}

func TestRegression_SeriesKeepsAuthorTitle(t *testing.T) {
	type want struct{ author, title, series, position string }

	tests := []struct {
		input string
		want  want
	}{
		{"Author - Title Vol. 3", want{"Author", "Title", "Title", "3"}},
		{"Author - Series #2", want{"Author", "Series", "Series", "2"}},
		{"Author - Title, No. 4", want{"Author", "Title", "Title", "4"}},
		{"Brandon Sanderson - The Final Empire (Mistborn Book 1)", want{"Brandon Sanderson", "The Final Empire", "Mistborn", "1"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			b := NewBookParser().Parse(tt.input + ".epub")
			if got := (want{b.Author, b.Title, b.Series, b.SeriesPosition}); got != tt.want {
				t.Errorf("book = %+v, want %+v", got, tt.want)
			}

			a := NewAudiobookParser().Parse(tt.input + ".m4b")
			if got := (want{a.Author, a.Title, a.Series, a.SeriesPosition}); got != tt.want {
				t.Errorf("audiobook = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRegression_AudiobookISBN(t *testing.T) {
	tests := []struct{ input, isbn, title string }{
		{"Author - Title 9780306406157.m4b", "9780306406157", "Title"},
		{"Author - Title ISBN 978-0-306-40615-7.m4b", "9780306406157", "Title"},
		{"Author - Title 9781234567890.m4b", "", "Title 9781234567890"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r := NewAudiobookParser().Parse(tt.input)
			if r.ISBN13 != tt.isbn || r.Title != tt.title {
				t.Errorf("got isbn=%q title=%q, want isbn=%q title=%q", r.ISBN13, r.Title, tt.isbn, tt.title)
			}
		})
	}
}

func TestRegression_SeriesPositionKept(t *testing.T) {
	// "Vol. 3" used to be cut off as a fake ".3" file extension.
	if r := NewBookParser().Parse("Author - Title Vol. 3"); r.SeriesPosition != "3" {
		t.Errorf("book SeriesPosition = %q, want 3", r.SeriesPosition)
	}

	if r := NewAudiobookParser().Parse("Author - Title Vol. 3"); r.SeriesPosition != "3" {
		t.Errorf("audiobook SeriesPosition = %q, want 3", r.SeriesPosition)
	}
}
