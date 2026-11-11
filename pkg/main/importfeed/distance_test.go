package importfeed

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
)

// TestAudiobookSortWithRuntimeVerification tests that matchTracksByDistance correctly
// orders multi-disc audiobook files using runtime+title+index from Audnex chapters.
//
//  1. Collect and enrich tracks from the real folder
//  2. Fetch chapters from Audnex API → build DbtrackWithArtist
//  3. Run matchTracksByDistance (isAudiobook=true)
//  4. Verify all chapters matched and result is in disc-sequential file order
//
// Uses: V:\completed\Audiobook\David Baldacci - Die Sammler, ASIN: B004UW5QSK
// Run with: go test -v -run TestAudiobookSortWithRuntimeVerification -timeout 120s
func TestAudiobookSortWithRuntimeVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("manual test requiring a real local audiobook folder and live Audnex access")
	}

	folder := `V:\completed\Audiobook\David Baldacci - Die Sammler`
	asin := "B004UW5QSK"
	region := "de" // German audiobook

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// === Step 1: Collect and enrich local files ===
	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil {
		t.Fatalf("CollectFilesOnly failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("No audio files found in folder")
	}
	localTracks := parser_v2.CollectTracksFromFiles(files)
	localTracks = parser_v2.EnrichTracksWithTags(localTracks)
	t.Logf("Found %d audio files", len(localTracks))

	// === Step 2: Fetch chapters from Audnex → build DbtrackWithArtist ===
	audnexProvider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
	})
	providers.SetAudnex(audnexProvider)

	chapters, err := audnexProvider.GetChaptersByASIN(ctx, asin, region)
	if err != nil {
		t.Fatalf("GetChaptersByASIN failed: %v", err)
	}
	if len(chapters) == 0 {
		t.Fatal("No chapters returned from Audnex")
	}
	t.Logf("Fetched %d chapters from Audnex for ASIN %s", len(chapters), asin)

	dbTracks := make([]database.DbtrackWithArtist, len(chapters))
	for i, ch := range chapters {
		dbTracks[i] = database.DbtrackWithArtist{
			Dbtrack: database.Dbtrack{
				Title:       ch.Title,
				TrackNumber: uint16(i + 1),
				RuntimeMs:   ch.LengthMs,
			},
		}
		t.Logf("  Chapter %2d: %s (%.1fs)", i+1, ch.Title, float64(ch.LengthMs)/1000)
	}

	if len(localTracks) != len(dbTracks) {
		t.Fatalf("Local track count %d != chapter count %d", len(localTracks), len(dbTracks))
	}

	// === Step 3: Run matchTracksByDistance ===
	result, matched, _ := matchTracksByDistance(localTracks, dbTracks, false, true, nil)

	// === Step 4: Verify all chapters matched and in disc-sequential file order ===
	t.Log("\n=== MATCH RESULT ===")
	var errors []string
	for i, track := range result {
		basename := filepath.Base(track.Filepath)
		status := "OK"
		if !matched[i] {
			status = "UNMATCHED"
			errors = append(
				errors,
				fmt.Sprintf("chapter %d (%s) has no match", i+1, dbTracks[i].Title),
			)
		}
		t.Logf("  Chapter %2d (%s) <- %-50s [%s]", i+1, dbTracks[i].Title, basename, status)
	}

	if !verifyDiscSequentialOrder(result) {
		errors = append(errors, "matched result is NOT in disc-sequential file order")
	}

	if len(localTracks) != 69 {
		t.Logf("WARNING: Expected 69 files, got %d", len(localTracks))
	}

	if len(errors) > 0 {
		t.Errorf("FAILED:\n  %s", strings.Join(errors, "\n  "))
	} else {
		t.Logf("\nSUCCESS: all %d chapters matched in disc-sequential order", len(result))
	}
}

// verifyDiscSequentialOrder checks that tracks are in disc-sequential order
// by verifying the numeric filename prefix is monotonically increasing.
func verifyDiscSequentialOrder(tracks []parser_v2.TrackInfo) bool {
	prevPrefix := ""
	for _, track := range tracks {
		basename := filepath.Base(track.Filepath)
		numPrefix := ""
		for _, c := range basename {
			if c >= '0' && c <= '9' {
				numPrefix += string(c)
			} else {
				break
			}
		}
		if numPrefix != "" && numPrefix < prevPrefix {
			return false
		}
		prevPrefix = numPrefix
	}
	return true
}

func TestLapAssignPaddedRectangular(t *testing.T) {
	tests := []struct {
		name string
		cost [][]float64
		want []int
	}{
		{
			name: "square identity",
			cost: [][]float64{{0, 1, 1}, {1, 0, 1}, {1, 1, 0}},
			want: []int{0, 1, 2},
		},
		{
			name: "greedy would be wrong",
			cost: [][]float64{{0.1, 0.2}, {0.2, 0.9}},
			want: []int{1, 0},
		},
		{
			// 3 DB rows, 2 local columns, padded with a dummy column of 1.0.
			name: "more db than local",
			cost: [][]float64{{0.5, 0.0, 1}, {0.0, 0.5, 1}, {0.4, 0.4, 1}},
			want: []int{1, 0, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lapAssign(tt.cost)
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("lapAssign = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRecommendationThresholds(t *testing.T) {
	tests := []struct {
		dists []float64
		want  albumRecommendation
	}{
		{nil, recNone},
		{[]float64{0.01}, recStrong},
		{[]float64{0.2, 0.21}, recMedium},
		{[]float64{0.2, 0.5}, recStrong},
		{[]float64{0.4}, recLow},
		{[]float64{0.4, 0.7}, recLow},
		{[]float64{0.4, 0.45}, recNone},
	}

	for _, tt := range tests {
		if got := recommendation(tt.dists, nil); got != tt.want {
			t.Errorf("recommendation(%v) = %d, want %d", tt.dists, got, tt.want)
		}
	}
}

// rejectedPairFixture: local track 2 is far outside the hard runtime limit of
// every DB track, so its LAP pair is rejected (cost 1.0 ≥ maxTD).
func rejectedPairFixture() ([]parser_v2.TrackInfo, []database.DbtrackWithArtist, *config.MediaDataConfig) {
	local := []parser_v2.TrackInfo{
		{Title: "Alpha", TrackNumber: 1, RuntimeMS: 200_000},
		{Title: "Zulu", TrackNumber: 2, RuntimeMS: 100_000},
	}
	db := []database.DbtrackWithArtist{
		{Dbtrack: database.Dbtrack{Title: "Alpha", TrackNumber: 1, RuntimeMs: 200_000}},
		{Dbtrack: database.Dbtrack{Title: "Bravo", TrackNumber: 2, RuntimeMs: 300_000}},
	}

	return local, db, &config.MediaDataConfig{PerTrackToleranceSeconds: 5}
}

func TestDistanceWithTracksRejectedPairCountsBothSides(t *testing.T) {
	local, db, data := rejectedPairFixture()

	// One perfect pair (0), one DB track missing and one local file unmatched:
	// raw = 0.9 + 0.6; max = artist 3 + album 3 + tracks 3 + 0.9 + 0.6.
	const want = 1.5 / 10.5

	album := albumDistanceWithTracks(
		&database.AlbumSearchResult{Title: "Album", Artist: "Artist"},
		"Artist", "Album", "", 0, local, db, false, data,
	)
	if math.Abs(album-want) > 1e-9 {
		t.Errorf("albumDistanceWithTracks = %.6f, want %.6f", album, want)
	}

	book := audiobookDistanceWithTracks(
		&database.AudiobookSearchResult{Title: "Album", Author: "Artist"},
		"Album", "Artist", local, db, data,
	)
	if math.Abs(book-want) > 1e-9 {
		t.Errorf("audiobookDistanceWithTracks = %.6f, want %.6f", book, want)
	}
}

// abChapters builds n audiobook chapters with distinct runtimes, except that
// chapters 19 and 20 are only 10 s apart.
func abChapters(n int) []database.DbtrackWithArtist {
	db := make([]database.DbtrackWithArtist, n)
	for i := range db {
		db[i].TrackNumber = uint16(i + 1)
		db[i].DiscNumber = 1
		db[i].RuntimeMs = int64(300_000 + i*60_000)
	}

	db[18].RuntimeMs = 600_000
	db[19].RuntimeMs = 610_000

	return db
}

func abLocal(db []database.DbtrackWithArtist) []parser_v2.TrackInfo {
	local := make([]parser_v2.TrackInfo, len(db))
	for i := range local {
		local[i].TrackNumber = i + 1
		local[i].RuntimeMS = db[i].RuntimeMs
	}

	// Each of files 19/20 is 9 s off its own chapter but 1 s off the other one.
	local[18].RuntimeMS = 609_000
	local[19].RuntimeMS = 601_000

	return local
}

func TestMatchTracksByDistanceAudiobookKeepsFileOrder(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}
	db := abChapters(20)
	local := abLocal(db)

	// Precondition: the unconstrained assignment swaps files 19 and 20.
	cost := make([][]float64, len(db))
	for i := range cost {
		cost[i] = make([]float64, len(local))
		for j := range cost[i] {
			cost[i][j] = trackDistance(&local[j], &db[i], false, true, data)
		}
	}

	if lap := lapAssign(cost); lap[18] != 19 || lap[19] != 18 {
		t.Fatalf("test setup: lapAssign did not swap 19/20: %v", lap)
	}

	result, matched, used := matchTracksByDistance(local, db, false, true, data)
	for i := range db {
		if !matched[i] || !used[i] || result[i].TrackNumber != i+1 {
			t.Fatalf("chapter %d got file %d (matched=%v)", i+1, result[i].TrackNumber, matched[i])
		}
	}
}

func TestMatchTracksByDistanceAudiobookMissingChapterKeepsOrder(t *testing.T) {
	data := &config.MediaDataConfig{
		PerTrackToleranceSeconds:    3,
		PerTrackToleranceSecondsMax: 30,
		AllowMissingTracks:          true,
	}
	db := abChapters(20)
	local := abLocal(db)

	// Chapter 5 is missing locally; the files keep their original numbers.
	local = append(local[:4:4], local[5:]...)

	result, matched, _ := matchTracksByDistance(local, db, false, true, data)
	for i := range db {
		switch {
		case i == 4:
			if matched[i] {
				t.Fatalf("missing chapter 5 matched file %d", result[i].TrackNumber)
			}
		case !matched[i] || result[i].TrackNumber != i+1:
			t.Fatalf("chapter %d got file %d (matched=%v)", i+1, result[i].TrackNumber, matched[i])
		}
	}
}

func TestAudiobookLocalOrder(t *testing.T) {
	tracks := []parser_v2.TrackInfo{
		{DiscNumber: 2, TrackNumber: 1},
		{DiscNumber: 1, TrackNumber: 2},
		{DiscNumber: 1, TrackNumber: 1},
	}

	got := audiobookLocalOrder(tracks, true)
	if len(got) != 3 || got[0] != 2 || got[1] != 1 || got[2] != 0 {
		t.Fatalf("audiobookLocalOrder = %v, want [2 1 0]", got)
	}

	if audiobookLocalOrder(tracks, false) != nil {
		t.Fatal("music tracks must not get an order constraint")
	}

	tracks[0] = parser_v2.TrackInfo{DiscNumber: 1, TrackNumber: 2}
	if audiobookLocalOrder(tracks, true) != nil {
		t.Fatal("duplicate track numbers must leave the order unknown")
	}

	tracks[0] = parser_v2.TrackInfo{}
	if audiobookLocalOrder(tracks, true) != nil {
		t.Fatal("an unnumbered track must leave the order unknown")
	}
}

func TestSingleFileTotalMatch(t *testing.T) {
	prod := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}

	tests := []struct {
		name       string
		localMs    int64
		chaptersMs int64
		chapters   int
		data       *config.MediaDataConfig
		want       bool
		wantTolMs  int64
	}{
		// 038 Benjamin Blümchen - Der Zoo zieht um: 1 file vs 9 Audnex chapters.
		{"zoo zieht um", 2_545_544, 2_631_147, 9, prod, true, 270_000},
		{"beyond chapters x hard limit", 2_545_544, 2_831_147, 9, prod, false, 270_000},
		{"total override", 2_545_544, 2_631_147, 9,
			&config.MediaDataConfig{PerTrackToleranceSeconds: 3, MaxTotalDifferenceSeconds: 60}, false, 60_000},
		{"grace is the hard limit", 2_545_544, 2_560_000, 9,
			&config.MediaDataConfig{PerTrackToleranceSeconds: 3}, true, 27_000},
		{"beets default", 2_545_544, 2_631_147, 9, nil, true, 270_000},
		{"no chapter runtimes", 2_545_544, 0, 9, prod, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, _, tol := singleFileTotalMatch(tt.localMs, tt.chaptersMs, tt.chapters, tt.data)
			if ok != tt.want || tol != tt.wantTolMs {
				t.Fatalf("singleFileTotalMatch = %v (tol %d), want %v (tol %d)", ok, tol, tt.want, tt.wantTolMs)
			}
		})
	}
}

func TestSingleFileAudiobookAgainstChapters(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}
	c := &database.AudiobookSearchResult{Title: "Der Zoo zieht um", Author: "Elfie Donnelly", ChapterCount: 9}

	chapterMs := []int64{54614, 441876, 389399, 238608, 148329, 471481, 243577, 313330, 329933}
	db := make([]database.DbtrackWithArtist, len(chapterMs))
	for i := range db {
		db[i].TrackNumber = uint16(i + 1)
		db[i].RuntimeMs = chapterMs[i]
	}

	local := []parser_v2.TrackInfo{{TrackNumber: 38, RuntimeMS: 2_545_544, Title: "der Zoo zieht um"}}

	if d := audiobookMatchDistance(c, "der Zoo zieht um", "Elfie Donnelly", 1); d > mediumRecThresh {
		t.Fatalf("album distance %.3f: missing chapters must not count against a single file", d)
	}

	tr, ok := singleFileAudiobookTrack(c, &local[0], db, data)
	if !ok || tr.TrackNumber != 1 || tr.Title != c.Title || tr.ExpectedRuntimeMS != 2_631_147 {
		t.Fatalf("singleFileAudiobookTrack = %+v, %v", tr, ok)
	}

	if d := audiobookDistanceWithTracks(c, "der Zoo zieht um", "Elfie Donnelly", local, db, data); d > mediumRecThresh {
		t.Fatalf("full distance %.3f, want ≤ %.2f", d, mediumRecThresh)
	}

	local[0].RuntimeMS = 2_000_000
	if _, ok := singleFileAudiobookTrack(c, &local[0], db, data); ok {
		t.Fatal("a file 10 minutes short must not match")
	}
}

// selectBestAudiobookMatches must apply the configured medium threshold, not
// the built-in default, to non-exact candidates.
func TestSelectBestAudiobookMatchesUsesConfiguredMediumThresh(t *testing.T) {
	// Runtime set and no local runtime: not a release cut differently than
	// the chapters, so only the threshold decides.
	m := &database.AudiobookSearchResult{Title: "The Hobbitt", Author: "J.R.R. Tolkien", ChapterCount: 12, Runtime: 600}

	d := audiobookMatchDistance(m, "The Hobbit", "J.R.R. Tolkien", 10)
	if d <= 0 || d >= 0.7 {
		t.Fatalf("distance %v unsuitable for the test", d)
	}

	strict := &config.MediaDataConfig{MediumRecThresh: d / 2}
	if got := selectBestAudiobookMatches([]*database.AudiobookSearchResult{m}, 10, 0, strict, "The Hobbit", "J.R.R. Tolkien"); len(got) != 0 {
		t.Errorf("MediumRecThresh %v: got %d candidates, want 0 (distance %v)", strict.MediumRecThresh, len(got), d)
	}

	lenient := &config.MediaDataConfig{MediumRecThresh: d + 0.01}
	if got := selectBestAudiobookMatches([]*database.AudiobookSearchResult{m}, 10, 0, lenient, "The Hobbit", "J.R.R. Tolkien"); len(got) != 1 {
		t.Errorf("MediumRecThresh %v: got %d candidates, want 1 (distance %v)", lenient.MediumRecThresh, len(got), d)
	}
}

func TestSdNormalizeUnicode(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"abc123", "abc123"},
		{"The Wall!", "thewall"},
		{"Dvořák", "dvorak"},
		{"Café Société", "cafesociete"},
		{"Война и мир", "войнаимир"},
		{"千と千尋の神隠し", "千と千尋の神隠し"},
		{"Ελληνικά", "ελληνικά"},
		{"Йога", "йога"},
		{"!!!", ""},
	}

	for _, tt := range tests {
		if got := sdNormalize(tt.in); got != tt.want {
			t.Errorf("sdNormalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStringDistBasicNonASCII(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		wantMin float64
		wantMax float64
	}{
		{"identical cyrillic", "Война и мир", "война и мир", 0, 0},
		{"different cyrillic", "Война и мир", "Преступление и наказание", 0.5, 1},
		{"different cjk", "千と千尋の神隠し", "もののけ姫", 0.5, 1},
		{"diacritics folded", "Dvořák", "Dvorak", 0, 0},
		{"one rune substitution", "Ελληνικά", "Ελληνικο", 0.1, 0.2},
		{"both punctuation only differ", "!!!", "???", 1, 1},
		{"both punctuation only equal", "...", "...", 0, 0},
	}

	for _, tt := range tests {
		got := stringDistBasic(tt.a, tt.b)
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("%s: stringDistBasic(%q, %q) = %v, want [%v, %v]",
				tt.name, tt.a, tt.b, got, tt.wantMin, tt.wantMax)
		}
	}
}

func TestLevenshteinIntRunes(t *testing.T) {
	if d := levenshteinInt("kitten", "sitting"); d != 3 {
		t.Errorf("ascii distance = %d, want 3", d)
	}

	if d := levenshteinInt("мир", "мор"); d != 1 {
		t.Errorf("cyrillic distance = %d, want 1", d)
	}

	if d := levenshteinInt("神隠し", "神"); d != 2 {
		t.Errorf("cjk distance = %d, want 2", d)
	}
}

func TestNamesMatchAndBestNameMatch(t *testing.T) {
	if !namesMatch("The Beatles", "the beatles") {
		t.Error("case-insensitive names should match")
	}

	if !namesMatch("Antonín Dvořák", "Antonin Dvorak") {
		t.Error("diacritic variants should match")
	}

	if namesMatch("Metallica", "Megadeth") {
		t.Error("different artists must not match")
	}

	if namesMatch("Война и мир", "Анна Каренина") {
		t.Error("different cyrillic titles must not match")
	}

	cands := []string{"Iron Maidens", "Iron Maiden", "Maiden United"}
	get := func(i int) string { return cands[i] }

	if got := bestNameMatch("Iron Maiden", len(cands), get, nil); got != 1 {
		t.Errorf("bestNameMatch picked %d, want 1", got)
	}

	if got := bestNameMatch("Slayer", len(cands), get, nil); got != -1 {
		t.Errorf("bestNameMatch for unrelated name = %d, want -1", got)
	}

	accept := func(i int) bool { return i != 1 }
	if got := bestNameMatch("Iron Maiden", len(cands), get, accept); got != 0 {
		t.Errorf("bestNameMatch with filter = %d, want 0", got)
	}
}

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
		{
			"folderAlbum+folderArtist",
			fmt.Sprintf(`release:"%s" AND artist:"%s"`, folderAlbum, folderArtist),
		},
		{"tagAlbum+tagArtist", fmt.Sprintf(`release:"%s" AND artist:"%s"`, tagAlbum, tagArtist)},
		{
			"tagAlbum+tagAlbumArtist",
			fmt.Sprintf(`release:"%s" AND artist:"%s"`, tagAlbum, tagAlbumArtist),
		},
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
		dist := albumMatchDistance(
			candidate,
			artist,
			albumTitle,
			tagMBID,
			"",
			"",
			year,
			fileCount,
			data,
		)
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
			rec := recommendation([]float64{fullDist}, nil)
			t.Logf("  recommendation: %v  (need >= recMedium=%v)", rec, recMedium)
		}
	}
}

func TestRecommendationConfiguredThresholds(t *testing.T) {
	tests := []struct {
		name  string
		dists []float64
		data  *config.MediaDataConfig
		want  albumRecommendation
	}{
		{"default medium rejects 0.35", []float64{0.35}, nil, recLow},
		{"raised medium accepts 0.35", []float64{0.35}, &config.MediaDataConfig{MediumRecThresh: 0.4}, recStrong},
		{"lowered medium rejects 0.2", []float64{0.2, 0.3}, &config.MediaDataConfig{MediumRecThresh: 0.1}, recNone},
		{"default strong: close pair is medium", []float64{0.05, 0.06}, nil, recMedium},
		{"raised strong: close pair is strong", []float64{0.05, 0.06}, &config.MediaDataConfig{StrongRecThresh: 0.1}, recStrong},
		{"zero means default", []float64{0.05, 0.06}, &config.MediaDataConfig{}, recMedium},
	}

	for _, tt := range tests {
		if got := recommendation(tt.dists, tt.data); got != tt.want {
			t.Errorf("%s: recommendation(%v) = %d, want %d", tt.name, tt.dists, got, tt.want)
		}
	}
}

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
				t.Logf(
					"       → MB has no duration data (relRuntimeMs=0): FAIL-OPEN → WOULD IMPORT%s",
					targetMark,
				)
			} else if runtimeDiff <= toleranceMs {
				t.Logf(
					"       → runtime OK (diff=%dms <= tol=%dms) → WOULD IMPORT%s",
					runtimeDiff,
					toleranceMs,
					targetMark,
				)
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
		dist := albumMatchDistance(
			candidate,
			artist,
			albumTitle,
			tagMBID,
			"",
			"",
			year,
			fileCount,
			data,
		)
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
			rec := recommendation([]float64{fullDist}, nil)
			t.Logf("  recommendation: %v  (need >= recMedium=%v)", rec, recMedium)
		}
	}
}

// chaptersOf returns n chapters of ms each.
func chaptersOf(n int, ms int64) []database.DbtrackWithArtist {
	out := make([]database.DbtrackWithArtist, n)
	for i := range out {
		out[i] = database.DbtrackWithArtist{Dbtrack: database.Dbtrack{Title: "Kapitel", TrackNumber: uint16(i + 1), RuntimeMs: ms}}
	}

	return out
}

// filesOf returns n local files of ms each.
func filesOf(n int, ms int64) []parser_v2.TrackInfo {
	out := make([]parser_v2.TrackInfo, n)
	for i := range out {
		out[i] = parser_v2.TrackInfo{Filename: "part.mp3", TrackNumber: i + 1, RuntimeMS: ms}
	}

	return out
}

func TestRepackedTotalMatch(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}

	// 60 chapters of 5 min = 300 min.
	const chapters, chapterMs = 60, int64(300_000)

	cases := []struct {
		name  string
		local []parser_v2.TrackInfo
		want  bool
	}{
		{"more files, same total", filesOf(150, 120_000), true},
		{"fewer files, same total", filesOf(30, 600_000), true},
		{"fewer files, two chapters short", filesOf(29, 600_000), false},
		{"same count is not a repack", filesOf(60, 300_000), false},
		{"single file is handled elsewhere", filesOf(1, 18_000_000), false},
		{"file without runtime", append(filesOf(29, 620_000), parser_v2.TrackInfo{}), false},
	}

	for _, tc := range cases {
		if ok, _, _ := repackedTotalMatch(tc.local, chapters*chapterMs, chapters, data); ok != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, ok, tc.want)
		}
	}
}

// A book whose chapter count differs from the file count stays a candidate
// when title and author fit and a stored runtime matches the files.
func TestSelectBestAudiobookMatchesRepacked(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}

	const localMs = int64(322 * 60_000)

	book := func(title string, runtime int) *database.AudiobookSearchResult {
		return &database.AudiobookSearchResult{
			Title: title, Author: "Andreas Schreiner", ChapterCount: 60, Runtime: runtime,
		}
	}

	cases := []struct {
		name string
		m    *database.AudiobookSearchResult
		want int
	}{
		{"runtime fits", book("Am Abgrund der Zeit", 322), 1},
		{"no stored runtime", book("Am Abgrund der Zeit", 0), 1},
		{"runtime off by an hour", book("Am Abgrund der Zeit", 262), 0},
		{"other title", book("Die Legende von Sleepy Hollow", 322), 0},
	}

	for _, tc := range cases {
		got := selectBestAudiobookMatches(
			[]*database.AudiobookSearchResult{tc.m}, 152, localMs, data,
			"Am Abgrund der Zeit", "Andreas Schreiner",
		)
		if len(got) != tc.want {
			t.Errorf("%s: got %d candidates, want %d", tc.name, len(got), tc.want)
		}
	}
}

// A repacked folder scores like a single file: by its total runtime, not by
// the per-chapter counts.
func TestAudiobookDistanceWithTracksRepacked(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}
	c := &database.AudiobookSearchResult{Title: "Keinhorn", Author: "Julian Leitloff", ChapterCount: 38}

	db := chaptersOf(38, 300_000)

	if d := audiobookDistanceWithTracks(c, "Keinhorn", "Julian Leitloff", filesOf(109, 38*300_000/109), db, data); d > mediumRecThresh {
		t.Errorf("repacked distance %.3f, want ≤ %.2f", d, mediumRecThresh)
	}

	if d := audiobookDistanceWithTracks(c, "Keinhorn", "Julian Leitloff", filesOf(109, 60_000), db, data); d <= mediumRecThresh {
		t.Errorf("half-length folder distance %.3f, want > %.2f", d, mediumRecThresh)
	}
}

func TestAudiobookDistanceWithTracksEmbeddedChapters(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}
	c := &database.AudiobookSearchResult{Title: "Ronja Räubertochter", Author: "Astrid Lindgren", ChapterCount: 10}

	// Two editions of the same length: chapters of 290/310 s and of 300 s.
	uneven := chaptersOf(10, 300_000)
	for i := range uneven {
		uneven[i].RuntimeMs = 290_000 + int64(i%2)*20_000
	}

	even := chaptersOf(10, 300_000)

	file := func(chapters []database.DbtrackWithArtist) []parser_v2.TrackInfo {
		f := filesOf(1, 3_000_000)
		for i := range chapters {
			f[0].Chapters = append(f[0].Chapters, tags.Chapter{
				Title: chapters[i].Title, RuntimeMS: chapters[i].RuntimeMs,
			})
		}

		return f
	}

	dist := func(local []parser_v2.TrackInfo, db []database.DbtrackWithArtist) float64 {
		return audiobookDistanceWithTracks(c, c.Title, c.Author, local, db, data)
	}

	plain := filesOf(1, 3_000_000)
	if a, b := dist(plain, uneven), dist(plain, even); a != b {
		t.Fatalf("without chapters the editions differ: %v vs %v", a, b)
	}

	local := file(uneven)
	for i := range local[0].Chapters {
		// The release's own titles must not count against it.
		local[0].Chapters[i].Title = fmt.Sprintf("%03d - Kapitel %d.1 - Ronja Räubertochter", i+1, i+1)
	}

	right, wrong := dist(local, uneven), dist(local, even)

	if right >= wrong {
		t.Errorf("matching chapters %.4f not better than other edition %.4f", right, wrong)
	}

	if right > dist(plain, uneven) {
		t.Errorf("matching chapters %.4f worse than the total alone %.4f", right, dist(plain, uneven))
	}

	// Chapters that do not all pair (or a different count) leave the total
	// to decide, as for a file without chapters.
	moved := file(even)
	moved[0].Chapters[0].RuntimeMS += 60_000 // past the 30 s per-chapter limit
	moved[0].Chapters[1].RuntimeMS -= 60_000

	if d := dist(moved, even); d != dist(plain, even) {
		t.Errorf("moved boundary: %v, want the total-only %v", d, dist(plain, even))
	}

	if d := dist(file(even[:9]), even); d != dist(plain, even) {
		t.Errorf("9 of 10 chapters: %v, want the total-only %v", d, dist(plain, even))
	}
}

// fileWithChapters returns one local file holding chapters of the given
// lengths, titled the way releases title them.
func fileWithChapters(ms ...int64) parser_v2.TrackInfo {
	f := parser_v2.TrackInfo{Filename: "book.m4b", TrackNumber: 1, DiscNumber: 1}

	var start int64
	for i, d := range ms {
		f.Chapters = append(f.Chapters, tags.Chapter{
			Title:   fmt.Sprintf("%03d - Kapitel %d.1 - Buch", i+1, i+1),
			StartMS: start, RuntimeMS: d,
		})
		start += d
		f.RuntimeMS += d
	}

	return f
}

func TestReleaseChapterList(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}
	replace := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30, ReplaceEmbeddedChapters: true}

	// The book: chapters of 200, 400 and 300 s (starts 0, 200, 600 s).
	db := chaptersOf(3, 0)
	for i, d := range []int64{200_000, 400_000, 300_000} {
		db[i].Title = fmt.Sprintf("Kapitel %d", i+1)
		db[i].RuntimeMs = d
	}

	ch := func(title string, start, length int64) tags.Chapter {
		return tags.Chapter{Title: title, StartMS: start, RuntimeMS: length}
	}

	// Same cuts: the book's titles, the file's times - with or without the option.
	same := fileWithChapters(201_000, 399_000, 300_000)
	for _, d := range []*config.MediaDataConfig{data, replace} {
		want := []tags.Chapter{ch("Kapitel 1", 0, 201_000), ch("Kapitel 2", 201_000, 399_000), ch("Kapitel 3", 600_000, 300_000)}
		if got := releaseChapterList(&same, db, d); !slices.Equal(got, want) {
			t.Errorf("same cuts (replace %v): %+v", d.ReplaceEmbeddedChapters, got)
		}
	}

	// Cut differently, 6 pieces of 150 s in a 900 s file.
	pieces := fileWithChapters(150_000, 150_000, 150_000, 150_000, 150_000, 150_000)

	// Option A: the file's cuts, each titled after the chapter it starts in.
	// The piece at 600 s starts exactly where Kapitel 3 does.
	wantA := []tags.Chapter{
		ch("Kapitel 1", 0, 150_000), ch("Kapitel 1", 150_000, 150_000), ch("Kapitel 2", 300_000, 150_000),
		ch("Kapitel 2", 450_000, 150_000), ch("Kapitel 3", 600_000, 150_000), ch("Kapitel 3", 750_000, 150_000),
	}
	if got := releaseChapterList(&pieces, db, data); !slices.Equal(got, wantA) {
		t.Errorf("titled by start: %+v", got)
	}

	// Option B: the book's chapters, scaled to a file 1% longer (909 s).
	long := pieces
	long.RuntimeMS = 909_000

	wantB := []tags.Chapter{ch("Kapitel 1", 0, 202_000), ch("Kapitel 2", 202_000, 404_000), ch("Kapitel 3", 606_000, 303_000)}
	if got := releaseChapterList(&long, db, replace); !slices.Equal(got, wantB) {
		t.Errorf("replaced: %+v", got)
	}

	// A file without chapters gets the book's only with the option.
	bare := parser_v2.TrackInfo{RuntimeMS: 909_000}
	if got := releaseChapterList(&bare, db, replace); !slices.Equal(got, wantB) {
		t.Errorf("no chapters, replace: %+v", got)
	}

	if got := releaseChapterList(&bare, db, data); got != nil {
		t.Errorf("no chapters, no option: %+v, want nil", got)
	}

	// Nothing to write.
	untitled := slices.Clone(db)
	untitled[1].Title = ""

	already := fileWithChapters(200_000, 400_000, 300_000)
	for i := range already.Chapters {
		already.Chapters[i].Title = db[i].Title
	}

	for name, c := range map[string]struct {
		local parser_v2.TrackInfo
		db    []database.DbtrackWithArtist
	}{
		"book chapter without title":   {same, untitled},
		"book chapter without runtime": {same, chaptersOf(3, 0)},
		"titles already the book's":    {already, db},
		"file without runtime":         {parser_v2.TrackInfo{Chapters: same.Chapters}, db},
	} {
		if got := releaseChapterList(&c.local, c.db, replace); got != nil {
			t.Errorf("%s: %+v, want nil", name, got)
		}
	}
}

func TestAlbumMatchDistanceCountPenalties(t *testing.T) {
	c := &database.AlbumSearchResult{Title: "The Wall", Artist: "Pink Floyd", TotalTracks: 10}

	exact := albumMatchDistance(c, "Pink Floyd", "The Wall", "", "", "", 0, 10, nil)
	extra := albumMatchDistance(c, "Pink Floyd", "The Wall", "", "", "", 0, 12, nil)
	missing := albumMatchDistance(c, "Pink Floyd", "The Wall", "", "", "", 0, 8, nil)

	if extra <= exact || missing <= exact {
		t.Fatalf("no count penalties: exact %v extra %v missing %v", exact, extra, missing)
	}

	allowExtra := &config.MediaDataConfig{AllowExtraTracks: true}
	if d := albumMatchDistance(c, "Pink Floyd", "The Wall", "", "", "", 0, 12, allowExtra); d != exact {
		t.Errorf("allow_extra_tracks: extra files still penalised: %v", d)
	}

	if d := albumMatchDistance(c, "Pink Floyd", "The Wall", "", "", "", 0, 8, allowExtra); d != missing {
		t.Errorf("allow_extra_tracks changed the missing-track penalty: %v", d)
	}

	if d := albumMetaDistance(c, "Pink Floyd", "The Wall", "", 0, nil); d != exact {
		t.Errorf("albumMetaDistance = %v, want %v", d, exact)
	}
}

func TestSelectBestAlbumMatchesCountMismatch(t *testing.T) {
	// 3 tracks against 12 local files: far over the medium threshold on the
	// count penalties alone.
	short := &database.AlbumSearchResult{ID: 1, Title: "The Wall", Artist: "Pink Floyd", TotalTracks: 3}
	other := &database.AlbumSearchResult{ID: 2, Title: "Animals", Artist: "Genesis", TotalTracks: 3}
	matches := []*database.AlbumSearchResult{short, other}

	if got := selectBestAlbumMatches(matches, 12, "Pink Floyd", "The Wall", "", "", "", 0, nil); len(got) != 0 {
		t.Fatalf("without the option: %d candidates", len(got))
	}

	data := &config.MediaDataConfig{AllowCountMismatchIfTotalMatch: true}

	got := selectBestAlbumMatches(matches, 12, "Pink Floyd", "The Wall", "", "", "", 0, data)
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("with the option: %+v, want only the release with matching tags", got)
	}
}

var benchPairs = [][2]string{
	{"Kapitel 12", "Kapitel 13"},
	{"Another Brick in the Wall, Part 1", "In the Flesh?"},
	{"Die Schatzinsel - Teil 03", "Kapitel 3: Der alte Seebär"},
	{"Müller & Söhne (Live)", "Mueller and Soehne"},
	{"01 - Vorspann", "Vorspann"},
}

func BenchmarkStringDist(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		for _, p := range benchPairs {
			stringDist(p[0], p[1])
		}
	}
}

// matrixTracks is a 40-file folder against a 15-chapter book, with titles
// that exercise folding, patterns and parentheticals.
func matrixTracks() ([]parser_v2.TrackInfo, []database.DbtrackWithArtist) {
	titles := []string{"Der alte Seebär", "Müller & Söhne (Live)", "Kapitel 3, The", "Another Brick in the Wall, Part 1",
		"Vorspann feat. Someone", "Straße (Remastered 2011)", "Das Ende [Bonus]", "Teil (#4)"}

	local := make([]parser_v2.TrackInfo, 40)
	for i := range local {
		local[i] = parser_v2.TrackInfo{Title: titles[i%len(titles)], Artist: "Robert", TrackNumber: i + 1, RuntimeMS: int64(100_000 + i*7_000)}
	}

	db := make([]database.DbtrackWithArtist, 15)
	for i := range db {
		db[i].Title = titles[(i*3)%len(titles)]
		db[i].Artist = "Robert Louis"
		db[i].TrackNumber = uint16(i + 1)
		db[i].RuntimeMs = int64(100_000 + i*19_000)
	}

	return local, db
}

// TestTrackScorerReuse: texts prepared once and reused across all pairs score
// exactly like trackDistance on each pair.
func TestTrackScorerReuse(t *testing.T) {
	local, db := matrixTracks()

	for _, data := range []*config.MediaDataConfig{nil, {PerTrackToleranceSeconds: 10, PerTrackToleranceSecondsMax: 30}} {
		for _, isVA := range []bool{false, true} {
			sc := newTrackScorer(isVA, true, data)
			lts, dts := sc.localTexts(local), sc.dbTexts(db)

			for i := range db {
				for j := range local {
					if got, want := sc.dist(&local[j], &db[i], &lts[j], &dts[i]), trackDistance(&local[j], &db[i], isVA, true, data); got != want {
						t.Fatalf("%q vs %q: reused %v, single %v", local[j].Title, db[i].Title, got, want)
					}
				}
			}
		}
	}
}

func BenchmarkMatchTracksByDistance(b *testing.B) {
	local, db := matrixTracks()

	b.ReportAllocs()

	for b.Loop() {
		matchTracksByDistance(local, db, false, false, nil)
	}
}
