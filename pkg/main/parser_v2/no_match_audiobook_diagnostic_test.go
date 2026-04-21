package parser_v2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
)

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

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100.0 / float64(total)
}

// TestNoMatchAddFoundDiagnostic examines WHY each no_match folder was not rescued
// by the addFound path in matchAudiobookFolder, even when AddFound=true in config.
//
// matchAudiobookFolder addFound has four gates that must ALL pass:
//
//	Gate 1: album.ASIN != ""     ASIN must be extractable from folder path or file tags
//	Gate 2: listid != -1         AddFoundList config must resolve to a valid list ID
//	Gate 3: Audnex pre-flight    chapter count from Audnex must equal local file count
//	Gate 4: import success       JobImportAudiobooks must return a valid DB ID
//
// This test verifies Gate 1 (ASIN) for each folder and reports file count for
// Gate 3 context. Gates 2 and 4 require live config/API and cannot be checked here.
//
// Run with: go test -v -run TestNoMatchAddFoundDiagnostic ./parser_v2/
func TestNoMatchAddFoundDiagnostic(t *testing.T) {
	diagAddFoundGates(t, 0, 30)
}

// TestNoMatchAddFoundDiagnosticMiddle is the same but starts from the middle of the list.
// Run with: go test -v -run TestNoMatchAddFoundDiagnosticMiddle ./parser_v2/
func TestNoMatchAddFoundDiagnosticMiddle(t *testing.T) {
	diagAddFoundGates(t, 1020, 30)
}

// diagAddFoundGates is the shared diagnostic logic for addFound gate analysis.
func diagAddFoundGates(t *testing.T, offset, n int) {
	entries, err := os.ReadDir(noMatchAudiobookDir)
	if err != nil {
		t.Skipf("Cannot read %s: %v (skip if drive not mounted)", noMatchAudiobookDir, err)
	}

	ap := NewAudiobookParser()
	audibleProv := audible.NewProviderWithRegion(audible.RegionDE)
	ctx := context.Background()

	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}

	if offset >= len(dirs) {
		t.Skipf("offset %d >= total folders %d", offset, len(dirs))
	}

	var (
		noASIN          int // Gate 1 failed: no ASIN anywhere (including Audible fallback)
		asinFromFolder  int // Gate 1 passed via folder/path name
		asinFromTag     int // Gate 1 passed via file tag
		asinFromAudible int // Gate 1 passed via Audible title search (Step 4b fallback)
		noFiles         int // no audio files found at all
	)

	t.Logf("\n=== ADDFOUND GATE DIAGNOSTIC (folders %d–%d of %d) ===",
		offset+1, min2(offset+n, len(dirs)), len(dirs))
	t.Logf("\nGates checked here:")
	t.Logf("  Gate 1a/b/c: ASIN from folder/path/tags  (local check)")
	t.Logf("  Gate 1d:     ASIN via Audible title search (live API — Step 4b fallback)")
	t.Logf("  Gate 2: listid != -1        SKIPPED — needs live config")
	t.Logf("  Gate 3: Audnex pre-flight   SKIPPED — needs live Audnex API")
	t.Logf("  Gate 4: import success      SKIPPED — runtime only, check app logs\n")

	t.Logf("%-6s  %-52s  %-14s  %-5s  %s",
		"#", "FOLDER (truncated)", "ASIN", "FILES", "ADDFOUND STATUS")
	t.Logf("%s", strings.Repeat("-", 140))

	for i := offset; i < len(dirs) && i < offset+n; i++ {
		e := dirs[i]
		name := e.Name()
		subPath := filepath.Join(noMatchAudiobookDir, name)

		// Gate 1a: ASIN from AudiobookParser (parses folder name)
		pr := ap.Parse(name)
		asin := pr.ASIN
		asinSource := ""
		if asin != "" {
			asinSource = "folder-parser"
		}

		// Gate 1b: ASIN from ParseASINFromPath (full path scan, what matchAudiobookFolder uses)
		if asin == "" {
			if a := ParseASINFromPath(subPath); a != "" {
				asin = a
				asinSource = "path-scan"
			}
		}

		// Gate 1c: ASIN from file tags (final fallback in matchAudiobookFolder)
		audioFiles := listAudioFilesIn(subPath)
		fileCount := len(audioFiles)

		var tagArtist, tagAlbumArtist, tagAlbum string
		if asin == "" && fileCount > 0 {
			tagData := readTagsForFirstFileCtx(t.Context(), audioFiles)
			if tagData != nil {
				tagArtist = tagData.Artist
				tagAlbumArtist = tagData.AlbumArtist
				tagAlbum = tagData.Album
				if tagData.ASIN != "" {
					asin = tagData.ASIN
					asinSource = "file-tag"
				}
			}
		}

		// Gate 1d: Audible title+author search (Step 4b fallback in matchAudiobookFolder)
		// Mirrors matchAudiobookFolder's addPair logic: raw title, stripped episode prefix,
		// and the re-parsed second part after stripping (e.g. "001 - Der Super - Papagei"
		// → stripped "Der Super - Papagei" → second part "Papagei" which IS a substring
		// of Audible's "Superpapagei").
		var audibleMatchedTitle, audibleMatchedAuthor, audibleSearchedTitle string
		var triedTitles []string
		if asin == "" && fileCount > 0 {
			// Collect all title candidates (mirrors matchAudiobookFolder's addPair + strip logic)
			seenCandidates := make(map[string]bool)
			addCandidate := func(tt string) {
				key := strings.ToLower(strings.TrimSpace(tt))
				if key != "" && !seenCandidates[key] {
					seenCandidates[key] = true
					triedTitles = append(triedTitles, tt)
				}
			}
			for _, tt := range []string{pr.Title, tagAlbum} {
				if tt == "" {
					continue
				}
				addCandidate(tt)
				if stripped := simulateStripEpisodePrefix(tt); stripped != "" {
					addCandidate(stripped)
					// Also try the second part after " - " in the stripped title
					// (e.g. "Der Super - Papagei" → "Papagei"), which matches
					// compound words like Audible's "Superpapagei".
					if idx := strings.Index(stripped, " - "); idx > 0 {
						addCandidate(strings.TrimSpace(stripped[idx+3:]))
					}
				}
			}
			// Collect all author candidates
			authors := []string{pr.Author, tagAlbumArtist, tagArtist}

			// Cache Audible results by title to avoid duplicate API calls.
			// Every (title, author) combination is checked — same as findASINByTitleAuthor.
			type audibleCache struct {
				results []apiexternal_v2.AudiobookSearchResult
			}
			cache := make(map[string]*audibleCache)

		outerBreak:
			for _, tt := range triedTitles {
				titleKey := strings.ToLower(tt)
				if _, seen := cache[titleKey]; !seen {
					results, err := audibleProv.SearchByTitle(ctx, tt, 10)
					if err != nil {
						cache[titleKey] = &audibleCache{}
						continue
					}
					cache[titleKey] = &audibleCache{results: results}
				}
				for _, r := range cache[titleKey].results {
					if r.ASIN == "" {
						continue
					}
					rTitle := strings.ToLower(r.Title)
					if !strings.Contains(rTitle, titleKey) && !strings.Contains(titleKey, rTitle) {
						continue
					}
					for _, a := range authors {
						if a == "" {
							continue
						}
						aLower := strings.ToLower(a)
						matched := false
						// Exact substring match against Authors.
						for _, ra := range r.Authors {
							raLower := strings.ToLower(ra)
							if strings.Contains(raLower, aLower) || strings.Contains(aLower, raLower) {
								matched = true
								break
							}
						}
						// Fallback: word-level partial match against Authors + SeriesName/Series.
						if !matched {
							for word := range strings.FieldsSeq(aLower) {
								if len(word) < 4 {
									continue
								}
								for _, ra := range r.Authors {
									if strings.Contains(strings.ToLower(ra), word) {
										matched = true
										break
									}
								}
								if !matched {
									for _, sn := range []string{r.SeriesName, r.Series} {
										if strings.Contains(strings.ToLower(sn), word) {
											matched = true
											break
										}
									}
								}
								if matched {
									break
								}
							}
						}
						if matched {
							asin = r.ASIN
							asinSource = "audible-search"
							audibleMatchedTitle = r.Title
							audibleMatchedAuthor = a
							audibleSearchedTitle = tt
							break outerBreak
						}
					}
				}
			}
		}

		// Classify
		var status string
		switch {
		case fileCount == 0:
			status = "NO_AUDIO_FILES"
			noFiles++
		case asinSource == "audible-search":
			status = fmt.Sprintf(
				"Gate1d PASS via Audible: ASIN=%s  searched=%q → matched=%q / author=%q",
				asin,
				audibleSearchedTitle,
				audibleMatchedTitle,
				audibleMatchedAuthor,
			)
			asinFromAudible++
		case asin == "":
			if len(triedTitles) == 0 {
				status = "Gate1 FAIL: no ASIN, no title — cannot search Audible"
			} else {
				// Show all tried titles so it's clear what was attempted
				tried := make([]string, len(triedTitles))
				for k, tt := range triedTitles {
					tried[k] = fmt.Sprintf("%q", tt)
				}
				fallbackAuthor := ""
				for _, a := range []string{pr.Author, tagAlbumArtist, tagArtist} {
					if a != "" {
						fallbackAuthor = a
						break
					}
				}
				status = fmt.Sprintf("Gate1 FAIL: Audible miss — tried [%s] author=%q",
					strings.Join(tried, ", "), fallbackAuthor)
			}
			noASIN++
		case asinSource == "file-tag":
			status = fmt.Sprintf(
				"Gate1c OK [tag=%q] — Gate2/3/4 may have blocked; check app logs",
				asin,
			)
			asinFromTag++
		default:
			status = fmt.Sprintf(
				"Gate1 OK [%s=%q] — Gate2/3/4 may have blocked; check app logs",
				asinSource,
				asin,
			)
			asinFromFolder++
		}

		folderDisp := name
		if len(folderDisp) > 52 {
			folderDisp = folderDisp[:49] + "..."
		}
		asinDisp := asin
		if asinDisp == "" {
			asinDisp = "(none)"
		}

		t.Logf("%-6d  %-52s  %-14s  %-5d  %s", i+1, folderDisp, asinDisp, fileCount, status)
	}

	total := min2(n, len(dirs)-offset)
	t.Logf("\n=== ADDFOUND SUMMARY (%d folders sampled) ===", total)
	t.Logf(
		"  Gate1 FAIL — no ASIN anywhere:         %3d  (%.0f%%)  ← Audible search also missed; check title/author quality",
		noASIN,
		pct(noASIN, total),
	)
	t.Logf(
		"  Gate1 PASS — ASIN from folder/path:    %3d  (%.0f%%)  ← Gate2/3/4 blocked or import failed; check app debug logs",
		asinFromFolder,
		pct(asinFromFolder, total),
	)
	t.Logf(
		"  Gate1 PASS — ASIN from file tag:       %3d  (%.0f%%)  ← Gate2/3/4 blocked or import failed; check app debug logs",
		asinFromTag,
		pct(asinFromTag, total),
	)
	t.Logf(
		"  Gate1d PASS — ASIN via Audible search: %3d  (%.0f%%)  ← Step 4b fallback would rescue these folders",
		asinFromAudible,
		pct(asinFromAudible, total),
	)
	t.Logf("  No audio files at all:                 %3d  (%.0f%%)",
		noFiles, pct(noFiles, total))
	t.Logf("")
	t.Logf("  If Gate1d PASS is high → Step 4b Audible fallback is working well")
	t.Logf("  If Gate1 FAIL is still dominant → titles/authors too noisy for Audible matching")
	t.Logf("  If Gate1 OK but still no_match → check app logs for:")
	t.Logf("    'Chapter count mismatch - skipping addFound import'  (Gate3 pre-flight blocked)")
	t.Logf("    'AddFoundList' config resolves to listid=-1           (Gate2 blocked)")
	t.Logf("    'Successfully imported audiobook' absent              (Gate4 import failed)")
}
