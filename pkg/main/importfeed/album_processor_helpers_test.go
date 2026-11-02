package importfeed

import (
	"context"
	"math"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

func TestRuntimeDiffWithinTolerance(t *testing.T) {
	def := &config.MediaDataConfig{}
	perTrack := &config.MediaDataConfig{PerTrackToleranceSeconds: 10}
	maxTotal := &config.MediaDataConfig{PerTrackToleranceSeconds: 10, MaxTotalDifferenceSeconds: 5}

	tests := []struct {
		name            string
		local, expected int64
		files           int
		data            *config.MediaDataConfig
		want            bool
	}{
		{"nil config", 1000, 1000, 1, nil, false},
		{"unknown runtime", 1000, 0, 1, def, false},
		{"default 3s per track, within", 100_000, 106_000, 2, def, true},
		{"default 3s per track, outside", 100_000, 106_001, 2, def, false},
		{"local longer counts too", 106_000, 100_000, 2, def, true},
		{"custom per track", 100_000, 120_000, 2, perTrack, true},
		{"max total overrides per track", 100_000, 106_000, 2, maxTotal, false},
		{"max total within", 100_000, 105_000, 2, maxTotal, true},
	}

	for _, tt := range tests {
		if got := runtimeDiffWithinTolerance(tt.local, tt.expected, tt.files, tt.data); got != tt.want {
			t.Errorf("%s: got %t, want %t", tt.name, got, tt.want)
		}
	}
}

func TestReleaseTitleName(t *testing.T) {
	local := "Stef Bos - In Een Ander Licht"

	if got := releaseTitleName("In Een Ander Licht", &local); got != "In Een Ander Licht" {
		t.Errorf("API title not preferred: %q", got)
	}

	if got := releaseTitleName("", &local); got != local {
		t.Errorf("local fallback: %q", got)
	}

	if got := releaseTitleName("", nil); got != "" {
		t.Errorf("no title: %q", got)
	}
}

func pairSet(pairs []musicSearchPair) map[string]string {
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		m[p.artist+"|"+p.album] = p.source
	}

	return m
}

func TestBuildMusicSearchPairs(t *testing.T) {
	pairs := buildMusicSearchPairs(folderMetadata{
		FolderArtist:   "Frank Zappa & The Mothers Of Invention",
		FolderAlbum:    "Absolutely Free",
		FilenameArtist: "frank zappa & the mothers of invention",
		FilenameAlbum:  "ABSOLUTELY FREE",
		TagArtist:      "Frank Zappa",
		TagAlbum:       "Absolutely Free",
	})
	set := pairSet(pairs)

	if len(set) != len(pairs) {
		t.Errorf("duplicate pairs returned: %d pairs, %d unique", len(pairs), len(set))
	}

	// The case-only filename variant is a duplicate of the folder pair.
	if got := set["Frank Zappa & The Mothers Of Invention|Absolutely Free"]; got != "folder" {
		t.Errorf("folder pair source = %q, want folder", got)
	}

	if _, ok := set["frank zappa & the mothers of invention|ABSOLUTELY FREE"]; ok {
		t.Error("case-insensitive duplicate was not removed")
	}

	if got := set["Frank Zappa|Absolutely Free"]; got != "tag-artist" {
		t.Errorf("tag pair source = %q, want tag-artist", got)
	}

	if got := set["The Mothers Of Invention|Absolutely Free"]; got != "compound-alt" {
		t.Errorf("compound alternative source = %q, want compound-alt", got)
	}
}

func TestBuildMusicSearchPairsVAAndEpisodes(t *testing.T) {
	set := pairSet(buildMusicSearchPairs(folderMetadata{
		FolderArtist: "VA",
		FolderAlbum:  "001 - Bravo Hits",
	}))

	if got := set["Various Artists|001 - Bravo Hits"]; got != "va-expanded+folder" {
		t.Errorf("VA expansion source = %q", got)
	}

	if got := set["VA|Bravo Hits"]; got != "stripped-ep+folder" {
		t.Errorf("stripped episode source = %q", got)
	}

	if len(buildMusicSearchPairs(folderMetadata{FolderAlbum: "No Artist"})) != 0 {
		t.Error("pairs without an artist must be skipped")
	}
}

func TestBuildMusicSearchPairsRawAndDisc(t *testing.T) {
	set := pairSet(buildMusicSearchPairs(folderMetadata{
		RawArtist:         "Artist",
		RawAlbum:          "Album",
		ParentAlbum:       "Parent Album",
		GrandparentArtist: "Grand Artist",
		IsDiscFolder:      true,
	}))

	if set["Artist|Album"] != "raw-folder" || set["Album|Artist"] != "raw-folder-swapped" {
		t.Errorf("raw pairs missing: %v", set)
	}

	if set["Grand Artist|Parent Album"] != "grandparent+parent" {
		t.Errorf("grandparent pair missing: %v", set)
	}
}

func TestParseFolderMetadata(t *testing.T) {
	walk := "/music/incoming"
	data := &config.MediaDataConfig{CfgPath: &config.PathsConfig{Path: walk}}

	m := parseFolderMetadata(walk+"/Pink Floyd - The Wall (1979)", nil, data)

	if m.FolderBase != "Pink Floyd - The Wall (1979)" {
		t.Errorf("FolderBase = %q", m.FolderBase)
	}

	if m.FolderArtist != "Pink Floyd" || m.FolderAlbum != "The Wall" || m.Year != 1979 {
		t.Errorf("folder parse = %q / %q / %d", m.FolderArtist, m.FolderAlbum, m.Year)
	}

	// The configured walk path is never treated as an artist folder.
	if m.ParentArtist != "" || m.ParentAlbum != "" {
		t.Errorf("parent of walk path parsed: %q / %q", m.ParentArtist, m.ParentAlbum)
	}

	disc := parseFolderMetadata("/music/Pink Floyd - The Wall/CD1", nil, nil)
	if !disc.IsDiscFolder {
		t.Error("CD1 not detected as disc folder")
	}

	if disc.ParentArtist != "Pink Floyd" || disc.ParentAlbum != "The Wall" {
		t.Errorf("disc parent = %q / %q", disc.ParentArtist, disc.ParentAlbum)
	}
}

func TestIsDiscSet(t *testing.T) {
	tests := []struct {
		names []string
		want  bool
	}{
		{[]string{
			"The Beatles - Artifacts III disk1 - Not Fade Away1969-1971 (1995)(320kbps)",
			"The Beatles - Artifacts III disk2 - Sue Me Sue You 1972-1975 (1995)(320kbps)",
			"The Beatles - Artifacts III disk3 - As Time Goes By 1976-1980 (1995)(320kbps)",
		}, true},
		{[]string{"Box CD 1 (Hits)", "Box CD 2 (Rarities)", "Artwork"}, true},
		{[]string{"Box-Disc_01", "Box-Disc_02"}, true},
		{[]string{"45 RPM The Singles of The The (CD 1)", "45 RPM The Singles of The The (CD 2)"}, true},
		{[]string{"Album [Disc 1]", "Album [Disc 2]"}, true},
		// One disc folder, the same disc twice, or different boxes: no set.
		{[]string{"Box disk1 - A", "Other Album"}, false},
		{[]string{"Box disk1 - A", "Box disk01 - A (repack)"}, false},
		{[]string{"Box A disk1", "Box B disk2"}, false},
		// Plain disc names are left to IsDiscSubfolder; artist folders stay artists.
		{[]string{"CD1", "CD2"}, false},
		{[]string{"Discovery (2001)", "Homework (1997)"}, false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := IsDiscSet(tt.names); got != tt.want {
			t.Errorf("IsDiscSet(%q) = %t, want %t", tt.names, got, tt.want)
		}
	}
}

func TestIsDiscSubfolder(t *testing.T) {
	tests := []struct {
		name, parent string
		want         bool
	}{
		{"CD1", "Play (1999)", true},
		{"CD 1", "", true},
		{"cd01", "", true},
		{"CD_1", "", true},
		{"Disc 1", "", true},
		{"Disc.2", "", true},
		{"Disk2", "", true},
		{"Side A", "", true},
		{"Part 1", "", true},
		{"Vol. 2", "", true},
		{"Disc 1 - The Beginning", "", true},
		{"CD1 (Bonus)", "", true},
		// Trailing token: only when the rest repeats the album folder name.
		{"Play CD2", "Play (1999)", true},
		{"Play - Disc 2", "Play", true},
		{"Play [CD 2]", "Play (1999)", true},
		{"Play (Disc 1)", "Play (1999)", true},
		// Album folders directly below the artist (production examples).
		{"Moonflower (Disc 1)", "Santana", false},
		{"Cafe Europa (disc 4)", "Elvis Presley", false},
		{"Play CD2", "", false},
		// Album/audiobook folders the loose discFolderRe matched, which
		// turned their artist folder into an "album".
		{"Trance Tapes (2021)", "Joe Colley", false},
		{"The Truckstop Tapes", "At War With Shadows", false},
		{"Party", "", false},
		{"Western Party", "", false},
		{"Halloween-Party (B0B1QM6D5Z)", "", false},
		{"Im Griff ihrer Partner (1665018674)", "", false},
		{"Dark Side of the Moon", "", false},
		{"Side Effects", "", false},
		{"Disco", "", false},
		{"Disciples of the Void", "", false},
		{"Off the Books", "", false},
		{"Booklet", "", false},
		{"A Part of Me", "", false},
		{"Greatest Hits, Part 2", "Artist", false},
		{"Glee The Music, Volume 5", "Glee Cast", false},
		{"Nashville On The Record - Volume 2 (2015)", "Nashville Cast", false},
		{"Charleroi Pittsburgh Vol.2 (2016)", "William Fitzsimmons", false},
		{"Friday The 13th Part 2 (Music From The Motion Picture) (Also Includes Music From Friday The 13th Part 3) (2017)", "Harry Manfredini", false},
	}

	for _, tt := range tests {
		if got := IsDiscSubfolder(tt.name, tt.parent); got != tt.want {
			t.Errorf("IsDiscSubfolder(%q, %q) = %v, want %v", tt.name, tt.parent, got, tt.want)
		}
	}
}

func TestBuildMatchedTracksWithFallbackTrackDistUsesLocalValues(t *testing.T) {
	local := []parser_v2.TrackInfo{{Title: "Omega", TrackNumber: 5, RuntimeMS: 180_000}}
	db := []database.DbtrackWithArtist{
		{Dbtrack: database.Dbtrack{Title: "Alpha", TrackNumber: 1, DiscNumber: 1, RuntimeMs: 180_000}},
	}

	want := trackDistance(&local[0], &db[0], false, false, nil)
	if want == 0 {
		t.Fatal("fixture must have a non-zero title/index distance")
	}

	got := buildMatchedTracksWithFallback(
		local, db, make([]parser_v2.TrackInfo, 1), []bool{false}, []bool{false}, false, false, nil,
	)
	if len(got) != 1 {
		t.Fatalf("got %d tracks, want 1", len(got))
	}

	if got[0].Title != "Alpha" || got[0].TrackNumber != 1 {
		t.Errorf("DB overrides not applied: %+v", got[0])
	}

	if math.Abs(got[0].TrackDist-want) > 1e-9 {
		t.Errorf("TrackDist = %.4f, want %.4f (distance of the local file before overrides)",
			got[0].TrackDist, want)
	}
}

func useTestDB(t *testing.T) {
	t.Helper()

	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)
}

type dbTrack struct {
	title string
	ms    int64
}

// seedAlbum stores an album and its tracks in the test catalogue.
func seedAlbum(t *testing.T, id uint, title string, tracks ...dbTrack) *database.AlbumSearchResult {
	t.Helper()

	if err := database.ExecNErr("insert into dbalbums (id, title) values (?, ?)", id, title); err != nil {
		t.Fatal(err)
	}

	var total int64

	for i, tr := range tracks {
		if err := database.ExecNErr(
			"insert into dbtracks (dbalbum_id, title, disc_number, track_number, runtime_ms) values (?, ?, 1, ?, ?)",
			id, tr.title, i+1, tr.ms); err != nil {
			t.Fatal(err)
		}

		total += tr.ms
	}

	return &database.AlbumSearchResult{ID: id, Title: title, Artist: "Pink Floyd", TotalTracks: len(tracks), TotalRuntime: int(total)}
}

var studio = []dbTrack{{"In the Flesh?", 199_000}, {"The Thin Ice", 147_000}, {"Another Brick in the Wall, Part 1", 191_000}}

func TestMusicCandidatePassesPicksMatchingRelease(t *testing.T) {
	useTestDB(t)

	studioAlbum := seedAlbum(t, 1, "The Wall", studio...)
	liveAlbum := seedAlbum(t, 2, "The Wall (Live)", append([]dbTrack{{"MC: Atmos", 60_000}}, studio...)...)

	tracks, total := localTracks(studio...)
	cands := []preScoredC{{studioAlbum, 0.02}, {liveAlbum, 0.1}}

	got := runMusicCandidatePasses(context.Background(), cands, tracks, total, false, "The Wall", "Pink Floyd", "", 1979, len(tracks), &config.MediaDataConfig{})
	if len(got) != 1 || got[0].c.ID != 1 || len(got[0].matchedTracks) != 3 {
		t.Fatalf("valid matches = %+v, want only the 3-track studio album", got)
	}

	id, n, _, ok := applyMusicRecommendation(got, total, len(tracks), &config.MediaDataConfig{})
	if !ok || id != 1 || n != 3 {
		t.Errorf("recommendation = %d %d %t", id, n, ok)
	}

	// With AllowMissingTracks the live album qualifies too, but ranks lower.
	loose := &config.MediaDataConfig{AllowMissingTracks: true}

	got = runMusicCandidatePasses(context.Background(), cands, tracks, total, false, "The Wall", "Pink Floyd", "", 1979, len(tracks), loose)
	if len(got) != 2 || got[0].c.ID != 1 || got[0].fullDist > got[1].fullDist {
		t.Errorf("allow-missing matches = %d, first %d", len(got), got[0].c.ID)
	}
}

func TestMusicCandidatePassesRejectsWrongRelease(t *testing.T) {
	useTestDB(t)

	other := seedAlbum(t, 1, "Animals", dbTrack{"Pigs on the Wing 1", 85_000}, dbTrack{"Dogs", 1_023_000})

	tracks, total := localTracks(studio...)

	got := runMusicCandidatePasses(context.Background(), []preScoredC{{other, 0.2}}, tracks, total, false, "The Wall", "Pink Floyd", "", 1979, len(tracks), &config.MediaDataConfig{})
	if len(got) != 0 {
		t.Errorf("a different album matched: %+v", got)
	}

	// Candidates beyond the relaxed distance cap are never examined.
	far := seedAlbum(t, 2, "The Wall", studio...)

	if got := runMusicCandidatePasses(context.Background(), []preScoredC{{far, 0.95}}, tracks, total, false, "The Wall", "Pink Floyd", "", 1979, len(tracks), nil); len(got) != 0 {
		t.Errorf("candidate past the distance cap matched: %+v", got)
	}
}

func TestTierTwoRuntimeAccepted(t *testing.T) {
	c := &database.AlbumSearchResult{TotalRuntime: 537_000}
	data := &config.MediaDataConfig{ExceedToleranceIfTotalMatch: true}

	if !tierTwoRuntimeAccepted(c, nil, 540_000, 3, data) {
		t.Error("3 s off with 9 s tolerance rejected")
	}

	if tierTwoRuntimeAccepted(c, nil, 560_000, 3, data) {
		t.Error("23 s off accepted")
	}

	if tierTwoRuntimeAccepted(c, nil, 537_000, 3, &config.MediaDataConfig{}) {
		t.Error("accepted without ExceedToleranceIfTotalMatch")
	}

	// Without an album runtime the track runtimes are summed.
	noTotal := &database.AlbumSearchResult{}
	tracks := []database.DbtrackWithArtist{{}, {}}
	tracks[0].RuntimeMs, tracks[1].RuntimeMs = 100_000, 200_000

	if !tierTwoRuntimeAccepted(noTotal, tracks, 301_000, 2, data) {
		t.Error("summed track runtime not used")
	}
}

func seedAudiobook(t *testing.T, id uint, title string, chapterMs ...int64) *database.AudiobookSearchResult {
	t.Helper()

	if err := database.ExecNErr("insert into dbaudiobooks (id, title) values (?, ?)", id, title); err != nil {
		t.Fatal(err)
	}

	for i, ms := range chapterMs {
		if err := database.ExecNErr(
			"insert into dbaudiobook_chapters (dbaudiobook_id, title, chapter_number, position, runtime_ms) values (?, ?, ?, ?, ?)",
			id, "Kapitel", i+1, i+1, ms); err != nil {
			t.Fatal(err)
		}
	}

	return &database.AudiobookSearchResult{ID: id, Title: title, Author: "Robert Louis Stevenson", ChapterCount: len(chapterMs)}
}

// TestAudiobookCandidatePassPicksEditionByRuntime: two editions share the
// title; only the one whose chapter runtimes fit the local files may match.
func TestAudiobookCandidatePassPicksEditionByRuntime(t *testing.T) {
	useTestDB(t)

	radioPlay := seedAudiobook(t, 1, "Die Schatzinsel", 1_140_000, 1_150_000, 1_130_000)
	reading := seedAudiobook(t, 2, "Die Schatzinsel", 9_600_000, 9_700_000, 9_500_000)

	local := []parser_v2.TrackInfo{
		{Title: "Teil 1", TrackNumber: 1, RuntimeMS: 1_141_000},
		{Title: "Teil 2", TrackNumber: 2, RuntimeMS: 1_149_000},
		{Title: "Teil 3", TrackNumber: 3, RuntimeMS: 1_131_000},
	}
	cfgp := &config.MediaTypeConfig{IsType: config.MediaTypeAudiobook}

	got := runAudiobookCandidatePass(context.Background(), []*database.AudiobookSearchResult{reading, radioPlay}, local, false,
		"Die Schatzinsel", "Robert Louis Stevenson", len(local), &config.MediaDataConfig{}, cfgp)

	if len(got) == 0 || got[0].c.ID != 1 {
		t.Fatalf("matches = %+v, want the radio play first", got)
	}

	for _, m := range got {
		if m.c.ID == 2 && m.fullDist <= got[0].fullDist {
			t.Errorf("8-hour reading ranks with or above the radio play: %v vs %v", m.fullDist, got[0].fullDist)
		}
	}

	id, chapters, _, ok := applyAudiobookRecommendation(got, &config.MediaDataConfig{})
	if !ok || id != 1 || chapters != 3 {
		t.Errorf("recommendation = %d %d %t", id, chapters, ok)
	}
}

func TestAudiobookCandidatePassSkipsUnverifiable(t *testing.T) {
	useTestDB(t)

	// No chapters with runtimes and no ASIN: nothing to verify against.
	bare := seedAudiobook(t, 1, "Die Schatzinsel")
	wrongTitle := seedAudiobook(t, 2, "Kidnapped", 1_140_000)

	local := []parser_v2.TrackInfo{{Title: "Teil 1", TrackNumber: 1, RuntimeMS: 1_141_000}}
	cfgp := &config.MediaTypeConfig{IsType: config.MediaTypeAudiobook}

	if got := runAudiobookCandidatePass(context.Background(), []*database.AudiobookSearchResult{bare, wrongTitle}, local, false,
		"Die Schatzinsel", "Robert Louis Stevenson", 1, nil, cfgp); len(got) != 0 {
		t.Errorf("unverifiable or wrong-title candidates matched: %+v", got)
	}
}

func TestAudiobookSearchPairsCoAuthors(t *testing.T) {
	pairs := buildAudiobookSearchPairs(folderMetadata{
		TagArtist:   "Terry Pratchett & Neil Gaiman",
		FolderAlbum: "Good Omens",
	})
	want := map[string]bool{"Terry Pratchett": false, "Neil Gaiman": false}
	for _, p := range pairs {
		if _, ok := want[p.author]; ok && p.title == "Good Omens" {
			want[p.author] = true
		}
	}
	for a, found := range want {
		if !found {
			t.Errorf("missing co-author pair for %q", a)
		}
	}
}

func TestCountUnmatched(t *testing.T) {
	db, local := countUnmatched([]bool{true, false, false}, []bool{true, true, false, false, false})
	if db != 2 || local != 3 {
		t.Errorf("countUnmatched = %d, %d, want 2, 3", db, local)
	}

	if db, local := countUnmatched(nil, nil); db != 0 || local != 0 {
		t.Errorf("empty = %d, %d", db, local)
	}
}

func musicMatch(id uint, dist float64, runtime int, tracks ...int64) musicValidMatch {
	m := musicValidMatch{c: &database.AlbumSearchResult{ID: id, TotalTracks: len(tracks), TotalRuntime: runtime}, fullDist: dist}
	for _, ms := range tracks {
		m.matchedTracks = append(m.matchedTracks, parser_v2.TrackInfo{ExpectedRuntimeMS: ms})
	}

	return m
}

func TestApplyMusicRecommendation(t *testing.T) {
	if id, _, _, ok := applyMusicRecommendation(nil, 0, 0, nil); id != 0 || ok {
		t.Errorf("no matches = %d %t", id, ok)
	}

	// A close single candidate is accepted on distance alone.
	id, tracks, matched, ok := applyMusicRecommendation([]musicValidMatch{musicMatch(7, 0.01, 0, 1000, 2000)}, 0, 2, nil)
	if !ok || id != 7 || tracks != 2 || len(matched) != 2 {
		t.Errorf("strong match = %d %d %d %t", id, tracks, len(matched), ok)
	}

	// Two close competitors far from medium: rejected without the runtime fallback.
	contested := []musicValidMatch{musicMatch(1, 0.5, 600_000, 300_000, 300_000), musicMatch(2, 0.55, 0)}
	if _, _, _, ok := applyMusicRecommendation(contested, 600_000, 2, &config.MediaDataConfig{}); ok {
		t.Error("contested weak match accepted without ExceedToleranceIfTotalMatch")
	}

	// With ExceedToleranceIfTotalMatch the total runtime decides.
	data := &config.MediaDataConfig{ExceedToleranceIfTotalMatch: true}
	if id, _, _, ok := applyMusicRecommendation(contested, 605_000, 2, data); !ok || id != 1 {
		t.Errorf("runtime fallback (5s off, 6s tolerance) = %d %t", id, ok)
	}

	if _, _, _, ok := applyMusicRecommendation(contested, 607_000, 2, data); ok {
		t.Error("runtime fallback accepted 7s off with 6s tolerance")
	}

	// Without an album runtime the matched tracks' expected runtimes are summed.
	noTotal := []musicValidMatch{musicMatch(3, 0.5, 0, 200_000, 100_000), musicMatch(4, 0.55, 0)}
	if id, _, _, ok := applyMusicRecommendation(noTotal, 300_000, 2, data); !ok || id != 3 {
		t.Errorf("summed track runtime = %d %t", id, ok)
	}

	// No runtime at all: nothing to compare, rejected.
	none := []musicValidMatch{musicMatch(5, 0.5, 0), musicMatch(6, 0.55, 0)}
	if _, _, _, ok := applyMusicRecommendation(none, 300_000, 2, data); ok {
		t.Error("accepted without any expected runtime")
	}

	// MaxTotalDifferenceSeconds overrides the per-track tolerance.
	wide := &config.MediaDataConfig{ExceedToleranceIfTotalMatch: true, MaxTotalDifferenceSeconds: 60}
	if _, _, _, ok := applyMusicRecommendation(contested, 650_000, 2, wide); !ok {
		t.Error("MaxTotalDifferenceSeconds not applied")
	}
}

func TestApplyAudiobookRecommendation(t *testing.T) {
	if _, _, _, ok := applyAudiobookRecommendation(nil, nil); ok {
		t.Error("no matches accepted")
	}

	best := audiobookValidMatch{c: &database.AudiobookSearchResult{ID: 9, ChapterCount: 12}, fullDist: 0.02}
	if id, chapters, _, ok := applyAudiobookRecommendation([]audiobookValidMatch{best}, nil); !ok || id != 9 || chapters != 12 {
		t.Errorf("strong match = %d %d %t", id, chapters, ok)
	}

	weak := []audiobookValidMatch{
		{c: &database.AudiobookSearchResult{ID: 1}, fullDist: 0.5},
		{c: &database.AudiobookSearchResult{ID: 2}, fullDist: 0.52},
	}
	if _, _, _, ok := applyAudiobookRecommendation(weak, nil); ok {
		t.Error("contested weak audiobook match accepted")
	}
}
