package parser_v2

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
)

func TestDeriveQualityProfile(t *testing.T) {
	tests := []struct {
		info TrackInfo
		want string
	}{
		{TrackInfo{Format: "flac", BitDepth: 24}, "lossless-hires"},
		{TrackInfo{Format: "FLAC", BitDepth: 16}, "lossless"},
		{TrackInfo{Format: "mp3", Bitrate: 320}, "high"},
		{TrackInfo{Format: "mp3", Bitrate: 256}, "medium-high"},
		{TrackInfo{Format: "aac", Bitrate: 192}, "medium"},
		{TrackInfo{Format: "mp3", Bitrate: 128}, "standard"},
		{TrackInfo{Format: "mp3", Bitrate: 96}, "low"},
	}

	for _, tt := range tests {
		if got := deriveQualityProfile(&tt.info); got != tt.want {
			t.Errorf("deriveQualityProfile(%s/%d/%dbit) = %q, want %q",
				tt.info.Format, tt.info.Bitrate, tt.info.BitDepth, got, tt.want)
		}
	}
}

func TestParseFFProbeTags(t *testing.T) {
	var info TrackInfo
	parseFFProbeTags(&info, map[string]string{
		"title":               "Chapter 1",
		"Artist":              "Stephen Fry",
		"ALBUM":               "Mythos",
		"album_artist":        "Stephen Fry",
		"genre":               "Audiobook",
		"track":               "3/12",
		"disc":                "2/3",
		"date":                "2017-11-02",
		"composer":            "Stephen Fry (Narrator)",
		"grouping":            "Mythos Series",
		"series-part":         "1",
		"asin":                "B0759MNMD2",
		"MUSICBRAINZ_TRACKID": "mbid-1",
		"isrc":                "",
	})

	want := TrackInfo{
		Title: "Chapter 1", Artist: "Stephen Fry", Album: "Mythos", AlbumArtist: "Stephen Fry",
		Genre: "Audiobook", TrackNumber: 3, DiscNumber: 2, Year: 2017,
		Narrator: "Stephen Fry (Narrator)", Series: "Mythos Series", SeriesNum: "1",
		ASIN: "B0759MNMD2", MusicBrainzID: "mbid-1",
	}

	if info.Title != want.Title || info.Artist != want.Artist || info.Album != want.Album ||
		info.AlbumArtist != want.AlbumArtist || info.Genre != want.Genre ||
		info.TrackNumber != want.TrackNumber || info.DiscNumber != want.DiscNumber ||
		info.Year != want.Year || info.Narrator != want.Narrator || info.Series != want.Series ||
		info.SeriesNum != want.SeriesNum || info.ASIN != want.ASIN ||
		info.MusicBrainzID != want.MusicBrainzID || info.ISRC != "" {
		t.Errorf("parseFFProbeTags =\n%+v\nwant\n%+v", info, want)
	}
}

func TestParseFFProbeTagsEdgeCases(t *testing.T) {
	info := TrackInfo{Title: "keep"}
	parseFFProbeTags(&info, nil)

	if info.Title != "keep" {
		t.Error("nil tag map changed the track")
	}

	parseFFProbeTags(&info, map[string]string{"disc": "0", "date": "1850", "title": ""})

	if info.DiscNumber != 1 {
		t.Errorf("disc 0 = %d, want 1", info.DiscNumber)
	}

	if info.Year != 0 {
		t.Errorf("implausible year accepted: %d", info.Year)
	}

	if info.Title != "keep" {
		t.Errorf("empty tag overwrote title: %q", info.Title)
	}
}

func trackNums(tracks []TrackInfo) [][2]int {
	out := make([][2]int, len(tracks))
	for i := range tracks {
		out[i] = [2]int{tracks[i].DiscNumber, tracks[i].TrackNumber}
	}

	return out
}

func TestNormalizeDiscTrackNumbersFromTrackNumbers(t *testing.T) {
	tracks := []TrackInfo{{TrackNumber: 101}, {TrackNumber: 102}, {TrackNumber: 201}, {TrackNumber: 202}}
	NormalizeDiscTrackNumbers(tracks)

	want := [][2]int{{1, 1}, {1, 2}, {2, 1}, {2, 2}}
	if got := trackNums(tracks); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNormalizeDiscTrackNumbersFromFilenames(t *testing.T) {
	tracks := []TrackInfo{
		{TrackNumber: 1, Filepath: "/a/101_artist_-_one.mp3"},
		{TrackNumber: 2, Filepath: "/a/102_artist_-_two.mp3"},
		{TrackNumber: 1, Filepath: "/a/201_artist_-_three.mp3"},
		{TrackNumber: 2, Filepath: "/a/202_artist_-_four.mp3"},
	}
	NormalizeDiscTrackNumbers(tracks)

	want := [][2]int{{1, 1}, {1, 2}, {2, 1}, {2, 2}}
	if got := trackNums(tracks); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNormalizeDiscTrackNumbersLeavesOthersAlone(t *testing.T) {
	cases := map[string][]TrackInfo{
		"tags already have discs": {
			{DiscNumber: 1, TrackNumber: 101}, {DiscNumber: 2, TrackNumber: 201},
		},
		"single encoded disc": {{TrackNumber: 101}, {TrackNumber: 102}},
		"disc with one track": {{TrackNumber: 101}, {TrackNumber: 102}, {TrackNumber: 201}},
		"plain numbering": {
			{TrackNumber: 1, Filepath: "01 - one.mp3"}, {TrackNumber: 2, Filepath: "02 - two.mp3"},
		},
		"too few tracks": {{TrackNumber: 101}},
	}

	for name, tracks := range cases {
		before := trackNums(tracks)
		NormalizeDiscTrackNumbers(tracks)

		if got := trackNums(tracks); !slices.Equal(got, before) {
			t.Errorf("%s: changed %v to %v", name, before, got)
		}
	}
}

func TestValidateTrackSequence(t *testing.T) {
	tests := []struct {
		name     string
		tracks   []TrackInfo
		complete bool
		missing  []int
	}{
		{"empty", nil, false, nil},
		{"complete", []TrackInfo{{TrackNumber: 2}, {TrackNumber: 1}, {TrackNumber: 3}}, true, nil},
		{"intro track 0", []TrackInfo{{TrackNumber: 0}, {TrackNumber: 1}}, true, nil},
		{"gap", []TrackInfo{{TrackNumber: 1}, {TrackNumber: 4}}, false, []int{102, 103}},
		{"missing start", []TrackInfo{{TrackNumber: 3}}, false, []int{101, 102}},
		{"duplicate", []TrackInfo{{TrackNumber: 1}, {TrackNumber: 1}}, false, []int{101}},
		{
			"per disc",
			[]TrackInfo{{DiscNumber: 1, TrackNumber: 1}, {DiscNumber: 1, TrackNumber: 2}, {DiscNumber: 2, TrackNumber: 2}},
			false, []int{201},
		},
	}

	for _, tt := range tests {
		album := AlbumInfo{Tracks: tt.tracks}
		complete, missing := validateTrackSequence(&album)
		slices.Sort(missing)

		if complete != tt.complete || !slices.Equal(missing, tt.missing) {
			t.Errorf("%s: got %t %v, want %t %v", tt.name, complete, missing, tt.complete, tt.missing)
		}

		if album.IsComplete != complete {
			t.Errorf("%s: album.IsComplete not updated", tt.name)
		}
	}
}

func TestHasExtension(t *testing.T) {
	exts := []string{".mp3", ".flac"}

	for in, want := range map[string]bool{"a.MP3": true, "b.flac": true, "c.m4b": false, ".mp3": false, "": false} {
		if got := HasExtension(in, exts); got != want {
			t.Errorf("HasExtension(%q) = %t, want %t", in, got, want)
		}
	}
}

func TestWalkFiles(t *testing.T) {
	root := t.TempDir()

	for _, f := range []string{"01.mp3", "02.FLAC", "cover.jpg", "CD2/03.mp3", "_unpack/x.mp3"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := WalkFiles(root, []string{".mp3", ".flac"})
	if err != nil {
		t.Fatal(err)
	}

	for i := range got {
		got[i], _ = filepath.Rel(root, got[i])
	}

	// "_"-prefixed subfolders are skipped; results come back in path order.
	want := []string{"01.mp3", "02.FLAC", filepath.Join("CD2", "03.mp3")}
	if !slices.Equal(got, want) {
		t.Errorf("WalkFiles = %v, want %v", got, want)
	}

	all, _ := WalkFiles(root, nil)
	if len(all) != 4 {
		t.Errorf("no extension filter = %d files, want 4", len(all))
	}

	// A root folder that itself starts with "_" is still walked.
	under := filepath.Join(root, "_unpack")
	if files, _ := WalkFiles(under, nil); len(files) != 1 {
		t.Errorf("underscore root = %v", files)
	}

	if _, err := WalkFiles("", nil); err == nil {
		t.Error("empty folder accepted")
	}

	if files, _ := CollectFilesOnly(root, nil); len(files) != 3 {
		t.Errorf("CollectFilesOnly defaults to audio extensions: %v", files)
	}
}

func TestCollectTracksFromFiles(t *testing.T) {
	tracks := CollectTracksFromFiles([]string{"/m/Pink Floyd - The Wall - 01 - In the Flesh.flac", "/m/02 - Thin Ice.flac"})

	if len(tracks) != 2 || tracks[0].Filepath != "/m/Pink Floyd - The Wall - 01 - In the Flesh.flac" ||
		tracks[1].TrackNumber != 2 || tracks[1].Title != "Thin Ice" {
		t.Errorf("CollectTracksFromFiles = %+v", tracks)
	}
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

func TestRegression_ParseAudioFilenameFallback(t *testing.T) {
	tests := []struct {
		input string
		track int
		title string
	}{
		{"1999.flac", 0, "1999"},
		{"2112.mp3", 0, "2112"},
		{"12_Song.mp3", 12, "Song"},
		{"05-Song.mp3", 5, "Song"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			info := ParseAudioFilename(tt.input)
			if info.TrackNumber != tt.track || info.Title != tt.title {
				t.Errorf("got track=%d title=%q, want track=%d title=%q",
					info.TrackNumber, info.Title, tt.track, tt.title)
			}
		})
	}
}

func TestRegression_ASINFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/x/BRAINSTORM", ""},
		{"/x/BCDEFGHIJK", ""},
		{"/x/1234567890", ""},
		{"/x/Title 0306406152", ""},
		{"/x/Title [B0ABCDEFGH]", "B0ABCDEFGH"},
		{"/x/B0ABCDEFGH", "B0ABCDEFGH"},
		{"/x/Title ASIN B0ABCDEFGH", "B0ABCDEFGH"},
		{"/x/0306406152", "0306406152"},
		{"/x/Title [0306406152]", "0306406152"},
		{"/x/Title asin: 0306406152", "0306406152"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := ParseASINFromPath(tt.path); got != tt.want {
				t.Errorf("ParseASINFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidateAlbum(t *testing.T) {
	tracks := []TrackInfo{{TrackNumber: 1}, {TrackNumber: 2}, {TrackNumber: 4}}

	album := AlbumInfo{Tracks: tracks, DiscCount: 1}
	complete, missing := ValidateAlbum(&album, 5)

	// The expected-count branch reports plain track numbers.
	if complete || !slices.Equal(missing, []int{3, 5}) || album.IsComplete {
		t.Errorf("expected 5: %t %v", complete, missing)
	}

	album = AlbumInfo{Tracks: tracks[:2], DiscCount: 1}
	if complete, missing := ValidateAlbum(&album, 2); !complete || missing != nil {
		t.Errorf("complete album: %t %v", complete, missing)
	}

	// Unknown count and multi-disc fall back to gap detection (disc*100+track).
	album = AlbumInfo{Tracks: tracks}
	if complete, missing := ValidateAlbum(&album, 0); complete || !slices.Equal(missing, []int{103}) {
		t.Errorf("gap detection: %t %v", complete, missing)
	}

	album = AlbumInfo{Tracks: []TrackInfo{{DiscNumber: 1, TrackNumber: 1}, {DiscNumber: 2, TrackNumber: 2}}, DiscCount: 2}
	if complete, missing := ValidateAlbum(&album, 10); complete || !slices.Equal(missing, []int{201}) {
		t.Errorf("multi-disc: %t %v", complete, missing)
	}
}

func TestNormalizeFromFilenames(t *testing.T) {
	mk := func(names ...string) []TrackInfo {
		tracks := make([]TrackInfo, len(names))
		for i, n := range names {
			tracks[i] = TrackInfo{Filepath: "/music/Album/" + n, TrackNumber: 1}
		}

		return tracks
	}

	tracks := mk("101_a.mp3", "102_b.mp3", "201 - c.mp3", "202-d.mp3")
	normalizeFromFilenames(tracks)

	want := [][2]int{{1, 1}, {1, 2}, {2, 1}, {2, 2}}
	for i, w := range want {
		if tracks[i].DiscNumber != w[0] || tracks[i].TrackNumber != w[1] {
			t.Errorf("%s: disc/track = %d/%d, want %d/%d",
				tracks[i].Filepath, tracks[i].DiscNumber, tracks[i].TrackNumber, w[0], w[1])
		}
	}

	// Anything that isn't a clean 3-digit disc+track encoding leaves tracks alone.
	for _, names := range [][]string{
		{"01_a.mp3", "02_b.mp3", "03_c.mp3", "04_d.mp3"},      // 2 digits
		{"101_a.mp3", "102_b.mp3", "103_c.mp3", "104_d.mp3"},  // one disc only
		{"101_a.mp3", "102_b.mp3", "201_c.mp3", "x_d.mp3"},    // no number
		{"101_a.mp3", "102_b.mp3", "201_c.mp3", "1000_d.mp3"}, // 4 digits
	} {
		tracks := mk(names...)
		normalizeFromFilenames(tracks)

		for _, tr := range tracks {
			if tr.DiscNumber != 0 || tr.TrackNumber != 1 {
				t.Errorf("%v: %s changed to disc/track %d/%d", names, tr.Filepath, tr.DiscNumber, tr.TrackNumber)
			}
		}
	}
}

func TestEnrichTracksWithTagsKeepsOrderAndFallback(t *testing.T) {
	// More tracks than workers, none readable: every slot must fall back to
	// its own parsed data, in input order.
	tracks := make([]TrackInfo, enrichTagWorkers*3+1)
	for i := range tracks {
		tracks[i] = TrackInfo{
			Filepath:    filepath.Join(t.TempDir(), fmt.Sprintf("missing-%d.mp3", i)),
			TrackNumber: i + 1,
		}
	}

	got := EnrichTracksWithTags(tracks)
	if len(got) != len(tracks) {
		t.Fatalf("got %d tracks, want %d", len(got), len(tracks))
	}

	for i := range got {
		if got[i].Filepath != tracks[i].Filepath || got[i].TrackNumber != tracks[i].TrackNumber {
			t.Errorf("slot %d = %q #%d, want %q #%d", i, got[i].Filepath, got[i].TrackNumber, tracks[i].Filepath, tracks[i].TrackNumber)
		}
	}

	if got := EnrichTracksWithTags(nil); len(got) != 0 {
		t.Errorf("nil input gave %d tracks", len(got))
	}
}

func TestReadEmbeddedChapters(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed - skipping", tool)
		}
	}

	dir := t.TempDir()
	meta := filepath.Join(dir, "chapters.txt")
	if err := os.WriteFile(meta, []byte(";FFMETADATA1\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=2500\ntitle=Kapitel 1\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=2500\nEND=6000\ntitle=Kapitel 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	withChapters := filepath.Join(dir, "book.m4b")
	plain := filepath.Join(dir, "plain.m4b")

	for _, args := range [][]string{
		{"-f", "lavfi", "-i", "sine=duration=6", "-i", meta, "-map_metadata", "1",
			"-map_chapters", "1", "-c:a", "aac", "-f", "mp4", withChapters},
		{"-f", "lavfi", "-i", "sine=duration=6", "-c:a", "aac", "-f", "mp4", plain},
	} {
		if out, err := exec.Command("ffmpeg", append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("ffmpeg cannot generate fixture (%v): %s", err, out)
		}
	}

	got, err := ReadEmbeddedChapters(context.Background(), withChapters)
	if err != nil {
		t.Fatal(err)
	}

	want := []tags.Chapter{
		{Title: "Kapitel 1", StartMS: 0, RuntimeMS: 2500},
		{Title: "Kapitel 2", StartMS: 2500, RuntimeMS: 3500},
	}
	if !slices.Equal(got, want) {
		t.Errorf("chapters = %+v, want %+v", got, want)
	}

	if got, err := ReadEmbeddedChapters(context.Background(), plain); err != nil || got != nil {
		t.Errorf("file without chapters: got %+v, %v; want nil, nil", got, err)
	}

	if _, err := ReadEmbeddedChapters(context.Background(), filepath.Join(dir, "missing.m4b")); err == nil {
		t.Error("missing file: want an error")
	}

	// Other formats go through ffprobe: ID3 CHAP frames in an MP3.
	mp3 := filepath.Join(dir, "book.mp3")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "sine=duration=6",
		"-i", meta, "-map_metadata", "1", "-map_chapters", "1", "-c:a", "libmp3lame", mp3).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot generate mp3 fixture (%v): %s", err, out)
	}

	got, err = ReadEmbeddedChapters(context.Background(), mp3)
	if err != nil || len(got) != 2 || got[1].Title != "Kapitel 2" || got[1].StartMS != 2500 {
		t.Errorf("mp3 chapters = %+v, %v", got, err)
	}
}
