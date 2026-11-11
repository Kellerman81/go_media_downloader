package importfeed

import (
	"context"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
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

func TestMusicCandidatePassesCountMismatchTotalMatch(t *testing.T) {
	useTestDB(t)

	// The first two songs as one track: same music, one track fewer.
	merged := seedAlbum(t, 1, "The Wall",
		dbTrack{"In the Flesh? / The Thin Ice", 346_000}, dbTrack{"Another Brick in the Wall, Part 1", 191_000})
	// Same tags, but an hour longer.
	longer := seedAlbum(t, 2, "The Wall",
		dbTrack{"In the Flesh? / The Thin Ice", 346_000}, dbTrack{"Another Brick in the Wall, Part 1", 3_791_000})

	tracks, total := localTracks(studio...)
	// Past the relaxed cap only because of the track-count penalties.
	cands := []preScoredC{{merged, 0.7}, {longer, 0.7}}

	for _, data := range []*config.MediaDataConfig{
		{},
		{AllowMissingTracks: true},
		{AllowExtraTracks: true},
	} {
		if got := runMusicCandidatePasses(context.Background(), cands, tracks, total, false, "The Wall", "Pink Floyd", "", 0, len(tracks), data); len(got) != 0 {
			t.Errorf("%+v: matched %+v", data, got)
		}
	}

	data := &config.MediaDataConfig{AllowCountMismatchIfTotalMatch: true}

	got := runMusicCandidatePasses(context.Background(), cands, tracks, total, false, "The Wall", "Pink Floyd", "", 0, len(tracks), data)
	if len(got) != 1 || got[0].c.ID != 1 {
		t.Fatalf("matches = %+v, want only the release with the same total runtime", got)
	}

	if data.AllowMissingTracks || data.AllowExtraTracks {
		t.Error("the configured data was changed")
	}

	// Tags that don't match keep the release out.
	got = runMusicCandidatePasses(context.Background(), cands, tracks, total, false, "Animals", "Pink Floyd", "", 0, len(tracks), data)
	if len(got) != 0 {
		t.Errorf("matched with other tags: %+v", got)
	}
}

// countSettings are all combinations of allow_missing_tracks,
// allow_extra_tracks and allow_count_mismatch_if_total_match.
func countSettings(base config.MediaDataConfig) []config.MediaDataConfig {
	out := make([]config.MediaDataConfig, 0, 8)
	for i := range 8 {
		d := base
		d.AllowMissingTracks = i&1 != 0
		d.AllowExtraTracks = i&2 != 0
		d.AllowCountMismatchIfTotalMatch = i&4 != 0
		out = append(out, d)
	}

	return out
}

func countSettingsName(d *config.MediaDataConfig) string {
	return fmt.Sprintf("missing=%t/extra=%t/total=%t",
		d.AllowMissingTracks, d.AllowExtraTracks, d.AllowCountMismatchIfTotalMatch)
}

// countScenario is a folder whose files don't line up with the release's
// tracks; want says whether the release may match under data.
type countScenario struct {
	name  string
	local []dbTrack
	want  func(d *config.MediaDataConfig) bool
}

// TestMusicCountSettingsMatrix runs folders with fewer files, more files and
// differently cut files against one release under every combination of the
// track-count settings, with the production per-track tolerances (10 s grace,
// 30 s hard cutoff).
func TestMusicCountSettingsMatrix(t *testing.T) {
	// Release: the three studio tracks, 537 s.
	scenarios := []countScenario{
		{"one track missing", studio[:2],
			func(d *config.MediaDataConfig) bool { return d.AllowMissingTracks }},
		{"one bonus track", append(append([]dbTrack{}, studio...), dbTrack{"Bonus Demo", 60_000}),
			func(d *config.MediaDataConfig) bool { return d.AllowExtraTracks }},
		{"first track split in two, same total", append([]dbTrack{{"In the Flesh? (Part A)", 99_000}, {"In the Flesh? (Part B)", 100_000}}, studio[1:]...),
			func(d *config.MediaDataConfig) bool {
				return (d.AllowMissingTracks && d.AllowExtraTracks) || d.AllowCountMismatchIfTotalMatch
			}},
		// Total 486 s against 537 s: only accepting both missing and extra
		// tracks lets it through.
		{"track missing, shorter bonus track", append(append([]dbTrack{}, studio[:2]...), dbTrack{"Bonus Demo", 140_000}),
			func(d *config.MediaDataConfig) bool { return d.AllowMissingTracks && d.AllowExtraTracks }},
	}

	for _, sc := range scenarios {
		for _, data := range countSettings(config.MediaDataConfig{PerTrackToleranceSeconds: 10, PerTrackToleranceSecondsMax: 30}) {
			t.Run(sc.name+"/"+countSettingsName(&data), func(t *testing.T) {
				useTestDB(t)

				release := seedAlbum(t, 1, "The Wall", studio...)
				tracks, total := localTracks(sc.local...)
				d := albumMatchDistance(release, "Pink Floyd", "The Wall", "", "", "", 0, len(tracks), &data)

				got := runMusicCandidatePasses(context.Background(), []preScoredC{{release, d}}, tracks, total,
					false, "The Wall", "Pink Floyd", "", 0, len(tracks), &data)

				_, _, _, accepted := applyMusicRecommendation(got, total, len(tracks), &data)
				t.Logf("matched=%t accepted=%t", len(got) == 1, accepted)

				if want := sc.want(&data); (len(got) == 1) != want {
					t.Errorf("matched = %t, want %t", len(got) == 1, want)
				}
			})
		}
	}
}

// TestAudiobookCountSettingsMatrix is TestMusicCountSettingsMatrix for an
// audiobook with three chapters, with the production per-track tolerances
// (3 s grace, 30 s hard cutoff).
func TestAudiobookCountSettingsMatrix(t *testing.T) {
	chapters := []int64{1_140_000, 1_150_000, 1_130_000}
	part := func(ms ...int64) []dbTrack {
		out := make([]dbTrack, len(ms))
		for i, m := range ms {
			out[i] = dbTrack{fmt.Sprintf("Teil %d", i+1), m}
		}

		return out
	}

	scenarios := []countScenario{
		{"one chapter missing", part(1_140_000, 1_150_000),
			func(d *config.MediaDataConfig) bool { return d.AllowMissingTracks }},
		{"one extra file", part(1_140_000, 1_150_000, 1_130_000, 300_000),
			func(d *config.MediaDataConfig) bool { return d.AllowExtraTracks }},
		// Different file count, same total: the repack check accepts it under
		// any setting.
		{"cut into four files, same total", part(855_000, 855_000, 855_000, 855_000),
			func(*config.MediaDataConfig) bool { return true }},
		// Same file count, cut at other points: no file matches its chapter.
		{"three files cut elsewhere, same total", part(1_000_000, 1_300_000, 1_120_000),
			func(d *config.MediaDataConfig) bool {
				return (d.AllowMissingTracks && d.AllowExtraTracks) || d.AllowCountMismatchIfTotalMatch
			}},
	}

	cfgp := &config.MediaTypeConfig{IsType: config.MediaTypeAudiobook}

	for _, sc := range scenarios {
		for _, data := range countSettings(config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}) {
			t.Run(sc.name+"/"+countSettingsName(&data), func(t *testing.T) {
				useTestDB(t)

				book := seedAudiobook(t, 1, "Die Schatzinsel", chapters...)
				tracks, _ := localTracks(sc.local...)

				got := runAudiobookCandidatePass(context.Background(), []*database.AudiobookSearchResult{book}, tracks,
					false, "Die Schatzinsel", "Robert Louis Stevenson", len(tracks), &data, cfgp)

				_, _, _, accepted := applyAudiobookRecommendation(got, &data)
				t.Logf("matched=%t accepted=%t", len(got) == 1, accepted)

				if want := sc.want(&data); (len(got) == 1) != want {
					t.Errorf("matched = %t, want %t", len(got) == 1, want)
				}
			})
		}
	}
}

func TestTrackCountAllowed(t *testing.T) {
	cases := []struct {
		data          *config.MediaDataConfig
		missing, xtra int
		want          bool
	}{
		{nil, 0, 0, true},
		{nil, 1, 0, false},
		{&config.MediaDataConfig{AllowMissingTracks: true}, 2, 0, true},
		{&config.MediaDataConfig{AllowMissingTracks: true}, 2, 1, false},
		{&config.MediaDataConfig{AllowExtraTracks: true}, 0, 3, true},
		{&config.MediaDataConfig{AllowExtraTracks: true}, 1, 3, false},
		{&config.MediaDataConfig{AllowMissingTracks: true, AllowExtraTracks: true}, 1, 3, true},
	}

	for _, tc := range cases {
		if got := trackCountAllowed(tc.data, tc.missing, tc.xtra); got != tc.want {
			t.Errorf("%+v missing %d extra %d: got %v", tc.data, tc.missing, tc.xtra, got)
		}
	}
}

func TestCountMismatchTotalMatch(t *testing.T) {
	on := &config.MediaDataConfig{AllowCountMismatchIfTotalMatch: true}

	cases := []struct {
		name     string
		data     *config.MediaDataConfig
		metaDist float64
		local    int64
		want     bool
	}{
		{"off", &config.MediaDataConfig{}, 0, 600_000, false},
		{"total and tags match", on, 0, 600_000 + 20_000, true},
		// default 3 s per release track: 10 tracks = 30 s
		{"total off", on, 0, 600_000 + 31_000, false},
		{"tags off", on, 0.5, 600_000, false},
		{"absolute cap", &config.MediaDataConfig{AllowCountMismatchIfTotalMatch: true, MaxTotalDifferenceSeconds: 60}, 0, 600_000 + 50_000, true},
		{"per-track tolerance", &config.MediaDataConfig{AllowCountMismatchIfTotalMatch: true, PerTrackToleranceSeconds: 1}, 0, 600_000 + 20_000, false},
		{"no local runtime", on, 0, 0, false},
	}

	for _, tc := range cases {
		if got := countMismatchTotalMatch(tc.data, tc.metaDist, tc.local, 600_000, 10); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
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

// TestAudiobookCandidatePassCountMismatchTotalMatch: as many files as
// chapters, but cut at other points, so with a hard per-track cutoff no file
// matches its chapter.
func TestAudiobookCandidatePassCountMismatchTotalMatch(t *testing.T) {
	useTestDB(t)

	book := seedAudiobook(t, 1, "Die Schatzinsel", 1_140_000, 1_150_000, 1_130_000)

	local := []parser_v2.TrackInfo{
		{Title: "Teil 1", TrackNumber: 1, RuntimeMS: 1_000_000},
		{Title: "Teil 2", TrackNumber: 2, RuntimeMS: 1_300_000},
		{Title: "Teil 3", TrackNumber: 3, RuntimeMS: 1_121_000},
	}
	cfgp := &config.MediaTypeConfig{IsType: config.MediaTypeAudiobook}
	books := []*database.AudiobookSearchResult{book}

	if got := runAudiobookCandidatePass(context.Background(), books, local, false,
		"Die Schatzinsel", "Robert Louis Stevenson", len(local), &config.MediaDataConfig{PerTrackToleranceSeconds: 10}, cfgp); len(got) != 0 {
		t.Fatalf("matched without the option: %+v", got)
	}

	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 10, AllowCountMismatchIfTotalMatch: true}

	if got := runAudiobookCandidatePass(context.Background(), books, local, false,
		"Die Schatzinsel", "Robert Louis Stevenson", len(local), data, cfgp); len(got) != 1 {
		t.Errorf("matches = %+v, want the book (same total runtime)", got)
	}

	if got := runAudiobookCandidatePass(context.Background(), books, local, false,
		"Kidnapped", "Robert Louis Stevenson", len(local), data, cfgp); len(got) != 0 {
		t.Errorf("matched with another title: %+v", got)
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

func TestSyntheticAlbumsDropExcept(t *testing.T) {
	useTestDB(t)

	for _, q := range []string{
		"insert into dbalbums (id, title) values (1, 'Winner'), (2, 'Loser'), (3, 'Listed meanwhile')",
		"insert into dbtracks (dbalbum_id, title, disc_number, track_number) values (1, 'a', 1, 1), (2, 'b', 1, 1), (3, 'c', 1, 1)",
		"insert into dbalbum_artists (dbalbum_id, dbartist_id) values (1, 1), (2, 1), (3, 1)",
		"insert into albums (dbalbum_id, listname) values (3, 'Music Library')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	ctx, synthetic := withSyntheticAlbums(context.Background())
	for _, id := range []uint{1, 2, 3} {
		noteSyntheticAlbum(ctx, id)
	}

	noteSyntheticAlbum(context.Background(), 99) // no collector: ignored

	synthetic.dropExcept(1)

	for id, want := range map[int]int{1: 1, 2: 0, 3: 1} {
		if got := database.Getdatarow[int](false, "select count() from dbalbums where id = ?", id); got != want {
			t.Errorf("dbalbum %d rows = %d, want %d", id, got, want)
		}
	}

	if got := database.Getdatarow[int](false,
		"select (select count() from dbtracks where dbalbum_id = 2) + (select count() from dbalbum_artists where dbalbum_id = 2)",
	); got != 0 {
		t.Errorf("dropped album left %d child rows", got)
	}
}

func TestRepackedAudiobookTracks(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}
	c := &database.AudiobookSearchResult{Title: "Der Drachenflüsterer", ChapterCount: 29}
	db := chaptersOf(29, 600_000)

	// 27 files on two discs, read in shuffled order; one file has no title.
	local := filesOf(27, 29*600_000/27)
	for i := range local {
		local[i].DiscNumber = 1 + i/14
		local[i].TrackNumber = 1 + i%14
		local[i].Title = "Teil"
	}

	local[26].Title = ""
	local[0], local[20] = local[20], local[0]

	got, ok := repackedAudiobookTracks(c, local, db, data)
	if !ok {
		t.Fatal("27 files as long as the 29 chapters must match")
	}

	if len(got) != 27 {
		t.Fatalf("got %d tracks, want 27", len(got))
	}

	for i := range got {
		if got[i].TrackNumber != i+1 || got[i].DiscNumber != 1 {
			t.Errorf("track %d numbered %d/%d, want %d/1", i, got[i].TrackNumber, got[i].DiscNumber, i+1)
		}
	}

	if got[14].RuntimeMS != local[14].RuntimeMS || got[0].ExpectedRuntimeMS != 0 {
		t.Error("runtime changed or expected runtime set")
	}

	if got[26].Title != "Der Drachenflüsterer - 027" {
		t.Errorf("untitled file named %q", got[26].Title)
	}

	if local[0].TrackNumber != 7 {
		t.Error("input slice was modified")
	}

	if _, ok := repackedAudiobookTracks(c, filesOf(27, 300_000), db, data); ok {
		t.Error("27 files of half the length must not match")
	}
}

func TestSingleFileAudiobookTrackReleaseChapters(t *testing.T) {
	data := &config.MediaDataConfig{PerTrackToleranceSeconds: 3, PerTrackToleranceSecondsMax: 30}
	c := &database.AudiobookSearchResult{Title: "Buch", Author: "Autor", ChapterCount: 2}

	db := chaptersOf(2, 300_000)
	db[0].Title, db[1].Title = "Kapitel 1", "Kapitel 2"

	local := fileWithChapters(300_000, 300_000)

	tr, ok := singleFileAudiobookTrack(c, &local, db, data)
	if !ok {
		t.Fatal("whole-book file not accepted")
	}

	titles := func(chapters []tags.Chapter) []string {
		out := make([]string, len(chapters))
		for i := range chapters {
			out[i] = chapters[i].Title
		}

		return out
	}

	if tr.Title != "Buch" || !slices.Equal(titles(tr.ReleaseChapters), []string{"Kapitel 1", "Kapitel 2"}) {
		t.Errorf("title %q, chapters %+v", tr.Title, tr.ReleaseChapters)
	}

	// Chapters cut differently: accepted on the total, each titled after the
	// book chapter it starts in.
	split := fileWithChapters(200_000, 200_000, 200_000)

	tr, ok = singleFileAudiobookTrack(c, &split, db, data)
	if want := []string{"Kapitel 1", "Kapitel 1", "Kapitel 2"}; !ok || !slices.Equal(titles(tr.ReleaseChapters), want) {
		t.Errorf("differently cut chapters: ok %v, chapters %+v, want titles %q", ok, tr.ReleaseChapters, want)
	}
}
