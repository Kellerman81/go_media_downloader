package database

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/jmoiron/sqlx"
)

// seedMusic builds a small catalogue: Pink Floyd with "The Wall" (listed in
// "Rock") and an alternate title, an aliased artist, and a VA compilation.
func seedMusic(t *testing.T) {
	t.Helper()

	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbartists (id, name, sort_name, slug) values (1, 'Pink Floyd', 'Floyd, Pink', 'pink-floyd')")
	mustExec(t, db, "insert into dbartists (id, name, slug) values (2, 'Various Artists', 'various-artists')")
	mustExec(t, db, "insert into dbartists (id, name, slug) values (3, 'Prince', 'prince')")
	mustExec(t, db, "insert into dbartist_aliases (dbartist_id, alias, slug) values (3, 'The Artist Formerly Known as Prince', 'the-artist-formerly-known-as-prince')")

	mustExec(t, db, "insert into dbalbums (id, title, slug) values (10, 'The Wall', 'the-wall')")
	mustExec(t, db, "insert into dbalbums (id, title, slug) values (11, 'Wish You Were Here', 'wish-you-were-here')")
	mustExec(t, db, "insert into dbalbums (id, title, slug) values (12, 'Bravo Hits 58', 'bravo-hits-58')")
	mustExec(t, db, "insert into dbalbums (id, title, slug) values (13, 'Purple Rain', 'purple-rain')")
	mustExec(t, db, "insert into dbalbum_titles (dbalbum_id, title, slug) values (11, 'WYWH', 'wywh')")

	for _, link := range [][2]int{{10, 1}, {11, 1}, {12, 2}, {13, 3}} {
		mustExec(t, db, "insert into dbalbum_artists (dbalbum_id, dbartist_id) values (?, ?)", link[0], link[1])
	}

	mustExec(t, db, "insert into albums (id, dbalbum_id, listname, rootpath, quality_profile) values (100, 10, 'Rock', '/data/music/Pink Floyd/The Wall', 'lossless')")
}

func TestFindDbalbumByArtistFirst(t *testing.T) {
	seedMusic(t)

	tests := []struct {
		raw    string
		want   uint
		artist string
	}{
		{"Pink Floyd - The Wall (1979) FLAC", 10, "Pink Floyd"},
		{"Pink.Floyd-The.Wall-WEB-FLAC-2011-GRP", 10, "Pink Floyd"},
		{"Pink Floyd, Wish You Were Here", 11, "Pink Floyd"},
		{"Pink Floyd - WYWH", 11, "Pink Floyd"},                       // alternate title
		{"floyd, pink - the wall", 10, "floyd, pink"},                 // sort name
		{"Pink Floyd - The Wall Deluxe Experience", 10, "Pink Floyd"}, // prefix match
		{"The Artist Formerly Known as Prince - Purple Rain", 13, "The Artist Formerly Known as Prince"},
		{"VA - Bravo Hits Vol. 58", 12, "Various Artists"}, // VA is expanded
		// A later volume of a stored title is a different release.
		{"Pink Floyd - The Wall Part 2", 0, ""},
		{"Unknown Artist - The Wall", 0, ""},
		{"", 0, ""},
	}

	for _, tt := range tests {
		m := ParseInfo{File: tt.raw}
		m.FindDbalbumByArtistFirst()

		if m.DbalbumID != tt.want {
			t.Errorf("%q: DbalbumID = %d, want %d", tt.raw, m.DbalbumID, tt.want)
		}

		if tt.want != 0 && m.Artist != tt.artist {
			t.Errorf("%q: Artist = %q, want %q", tt.raw, m.Artist, tt.artist)
		}
	}
}

func TestFindDbalbumByArtistFirstFromWantedList(t *testing.T) {
	seedMusic(t)

	m := ParseInfo{File: "Pink Floyd - The Wall FLAC"}
	m.FindDbalbumByArtistFirstFromWantedList([]string{"Pop", "rock"})

	if m.DbalbumID != 10 || m.AlbumID != 100 {
		t.Errorf("listed album: DbalbumID = %d, AlbumID = %d, want 10, 100", m.DbalbumID, m.AlbumID)
	}

	// Known to the catalogue but not on any wanted list.
	m = ParseInfo{File: "Pink Floyd - Wish You Were Here"}
	m.FindDbalbumByArtistFirstFromWantedList([]string{"Rock"})

	if m.DbalbumID != 0 || m.AlbumID != 0 {
		t.Errorf("unlisted album matched: DbalbumID = %d, AlbumID = %d", m.DbalbumID, m.AlbumID)
	}

	m = ParseInfo{File: "Pink Floyd - The Wall"}
	m.FindDbalbumByArtistFirstFromWantedList(nil)

	if m.DbalbumID != 0 {
		t.Errorf("no lists matched DbalbumID = %d", m.DbalbumID)
	}
}

func TestFindDbaudiobookByAuthorFirst(t *testing.T) {
	seedAudiobooks(t)

	tests := []struct {
		raw  string
		want uint
	}{
		{"Stephen Fry - Mythos [Audiobook]", 10},
		{"Stephen.Fry-Mythos-AUDIOBOOK-2017-GRP", 10},
		{"Stephen Fry - Heroes: Mortals and Monsters", 11},
		{"Stephen Fry - The Fry Chronicles Unabridged Edition", 12},
		{"Stephen Fry - The Fry Chronicles Teil 2", 0},
		// Single-word titles never prefix-match, but bracketed suffixes are dropped.
		{"Stephen Fry - Mythos Read by the Author", 0},
		{"Stephen Fry - Mythos (Unabridged)", 10},
		{"Stephen Fry - Mythos [ungekürzt] (2017)", 10},
		{"Stephen Fry - Mythos (Teil 2)", 0},
		{"Someone Else - Mythos", 0},
	}

	for _, tt := range tests {
		m := ParseInfo{File: tt.raw}
		m.FindDbaudiobookByAuthorFirst()

		if m.DbaudiobookID != tt.want {
			t.Errorf("%q: DbaudiobookID = %d, want %d", tt.raw, m.DbaudiobookID, tt.want)
		}
	}

	m := ParseInfo{File: "Stephen Fry - Mythos"}
	m.FindDbaudiobookByAuthorFirstFromWantedList([]string{"fantasy"})

	if m.DbaudiobookID != 10 || m.AudiobookID != 100 {
		t.Errorf("wanted list: DbaudiobookID = %d, AudiobookID = %d, want 10, 100", m.DbaudiobookID, m.AudiobookID)
	}

	m = ParseInfo{File: "Stephen Fry - Mythos [ungekürzt]"}
	m.FindDbaudiobookByAuthorFirstFromWantedList([]string{"Fantasy"})

	if m.DbaudiobookID != 10 || m.AudiobookID != 100 {
		t.Errorf("wanted list brackets: DbaudiobookID = %d, AudiobookID = %d", m.DbaudiobookID, m.AudiobookID)
	}

	m = ParseInfo{File: "Stephen Fry - Heroes"}
	m.FindDbaudiobookByAuthorFirstFromWantedList([]string{"Fantasy"})

	if m.DbaudiobookID != 0 {
		t.Errorf("unlisted audiobook matched: %d", m.DbaudiobookID)
	}
}

func TestStripTrailingBrackets(t *testing.T) {
	for in, want := range map[string]string{
		"Mythos (Unabridged)":                    "Mythos",
		"Mythos [ungekürzt] (2017)":              "Mythos",
		"Dune (Part Two)":                        "Dune (Part Two)",
		"Helden der Schlacht (Die Rom-Serie 18)": "Helden der Schlacht (Die Rom-Serie 18)",
		"Plain Title":                            "Plain Title",
		"(Only Brackets)":                        "(Only Brackets)",
		"Odd ) Title":                            "Odd ) Title",
	} {
		if got := stripTrailingBrackets(in); got != want {
			t.Errorf("stripTrailingBrackets(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindDbbookByAuthorFirst(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbauthors (id, name, slug) values (1, 'Brandon Sanderson', 'brandon-sanderson')")
	mustExec(t, db, "insert into dbbooks (id, title, slug) values (10, 'The Way of Kings', 'the-way-of-kings')")
	mustExec(t, db, "insert into dbbooks (id, title, slug) values (11, 'Words of Radiance', 'words-of-radiance')")
	mustExec(t, db, "insert into dbbook_titles (dbbook_id, title, slug) values (11, 'Stormlight 2', 'stormlight-2')")
	mustExec(t, db, "insert into dbbooks (id, title, slug) values (12, 'Elantris', 'elantris')")
	mustExec(t, db, "insert into dbbook_authors (dbbook_id, dbauthor_id) values (10, 1), (11, 1), (12, 1)")
	mustExec(t, db, "insert into books (id, dbbook_id, listname) values (100, 10, 'Fantasy')")

	tests := []struct {
		raw  string
		want uint
	}{
		{"Brandon Sanderson - The Way of Kings EPUB", 10},
		{"Brandon.Sanderson-The.Way.of.Kings-2010-GRP", 10},
		{"Brandon Sanderson - Stormlight 2", 11},
		{"Brandon Sanderson - The Way of Kings Leatherbound Edition", 10},
		{"Brandon Sanderson - The Way of Kings Part 2", 0},
		{"Brandon Sanderson - Elantris (Tenth Anniversary)", 12},
		{"Brandon Sanderson - Elantris (Book 2)", 0},
		{"Robert Jordan - The Way of Kings", 0},
	}

	for _, tt := range tests {
		m := ParseInfo{File: tt.raw}
		m.FindDbbookByAuthorFirst()

		if m.DbbookID != tt.want {
			t.Errorf("%q: DbbookID = %d, want %d", tt.raw, m.DbbookID, tt.want)
		}
	}

	m := ParseInfo{File: "Brandon Sanderson - The Way of Kings"}
	m.FindDbbookByAuthorFirstFromWantedList([]string{"FANTASY"})

	if m.DbbookID != 10 || m.BookID != 100 {
		t.Errorf("wanted list: DbbookID = %d, BookID = %d, want 10, 100", m.DbbookID, m.BookID)
	}

	m = ParseInfo{File: "Brandon Sanderson - The Way of Kings Leatherbound Edition"}
	m.FindDbbookByAuthorFirstFromWantedList([]string{"Fantasy"})

	if m.DbbookID != 10 || m.BookID != 100 {
		t.Errorf("wanted list prefix: DbbookID = %d, BookID = %d, want 10, 100", m.DbbookID, m.BookID)
	}

	m = ParseInfo{File: "Brandon Sanderson - The Way of Kings Book 2"}
	m.FindDbbookByAuthorFirstFromWantedList([]string{"Fantasy"})

	if m.DbbookID != 0 {
		t.Errorf("wanted list volume matched: %d", m.DbbookID)
	}

	m = ParseInfo{File: "Brandon Sanderson - Words of Radiance"}
	m.FindDbbookByAuthorFirstFromWantedList([]string{"Fantasy"})

	if m.DbbookID != 0 {
		t.Errorf("unlisted book matched: %d", m.DbbookID)
	}
}

// migratedDB builds an in-memory SQLite database from the migrations in
// ../schema/<dir> so SQL can be checked against the real schema.
func migratedDB(t *testing.T, dir string) *sqlx.DB {
	t.Helper()

	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	files, err := filepath.Glob(filepath.Join("..", "schema", dir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found in %s: %v", dir, err)
	}

	sort.Strings(files)

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(f), err)
		}
	}

	return db
}

// useMigratedMainDB points the package at a migrated in-memory main database
// for the duration of the test.
func useMigratedMainDB(t *testing.T) *sqlx.DB {
	t.Helper()

	NewCache(time.Hour, time.Hour)

	db := migratedDB(t, "db")
	prev := dbData
	dbData = db

	t.Cleanup(func() {
		// Statements are cached by SQL text; drop the ones bound to this db.
		cache.ristrettoStmt.Clear()
		dbData = prev
	})

	return db
}

func mustExec(t *testing.T, db *sqlx.DB, query string, args ...any) {
	t.Helper()

	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// stmtSQL evaluates the SQL argument of an addStaticXStmt call. List
// placeholders built at runtime (IgnoreMapListsQu/ListsQu) are rendered as ",?".
func stmtSQL(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		s, _ := strconv.Unquote(v.Value)
		return s
	case *ast.BinaryExpr:
		return stmtSQL(v.X) + stmtSQL(v.Y)
	case *ast.CallExpr:
		var out string
		for _, a := range v.Args {
			out += stmtSQL(a)
		}

		return out
	default:
		return ",?"
	}
}

// TestSetVarsStatementsPrepare prepares every statement SetVars/SetVarsType
// registers against the migrated schemas. addStaticXStmt drops statements that
// fail to prepare silently, so a broken one would otherwise go unnoticed.
func TestSetVarsStatementsPrepare(t *testing.T) {
	mainDB := migratedDB(t, "db")
	imdbDB := migratedDB(t, "imdbdb")

	f, err := parser.ParseFile(token.NewFileSet(), "general.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	count := 0

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "addStaticXStmt" || len(call.Args) != 2 {
			return true
		}

		query := stmtSQL(call.Args[0])

		db := mainDB
		if id, ok := call.Args[1].(*ast.Ident); ok && id.Name == "true" {
			db = imdbDB
		}

		count++

		stmt, err := db.Preparex(query)
		if err != nil {
			t.Errorf("prepare failed: %v\n  query: %s", err, query)
			return true
		}

		stmt.Close()

		return true
	})

	if count < 100 {
		t.Fatalf("found only %d addStaticXStmt calls - parser out of sync with general.go?", count)
	}
}

func TestStripReleaseTypeLeavesInputUnchanged(t *testing.T) {
	in := "Abbey Road Deluxe Edition 2019"
	orig := in

	got := stripReleaseType(&in)

	if in != orig {
		t.Fatalf("input mutated to %q", in)
	}

	if got != "Abbey Road" {
		t.Fatalf("stripReleaseType = %q, want %q", got, "Abbey Road")
	}
}

func TestTitleOrSlugEqual(t *testing.T) {
	tests := []struct {
		title, slug, wantTitle, wantSlug string
		want                             bool
	}{
		{"Dune", "dune", "dune", "dune", true},
		{"Other", "dune", "Dune Part", "dune", true},
		{"Other", "", "!!!", "", false}, // empty slug must not match an unpopulated slug
		{"!!!", "", "!!!", "", true},
		{"Other", "x", "Dune", "", false},
	}

	for _, tt := range tests {
		if got := titleOrSlugEqual(tt.title, tt.slug, tt.wantTitle, tt.wantSlug); got != tt.want {
			t.Errorf("titleOrSlugEqual(%q,%q,%q,%q) = %v, want %v",
				tt.title, tt.slug, tt.wantTitle, tt.wantSlug, got, tt.want)
		}
	}
}

// TestParseInfoPoolMissListID drains the pool past its preallocated objects:
// freshly created ones must carry the same "no list" sentinel as recycled ones.
func TestParseInfoPoolMissListID(t *testing.T) {
	got := make([]*ParseInfo, 0, 150)
	for range 150 {
		m := PLParseInfo.Get()
		if m.ListID != -1 {
			t.Fatalf("ParseInfo from pool has ListID %d, want -1", m.ListID)
		}

		m.ListID = 3
		m.Retail = true
		m.Episodes = append(m.Episodes, DbstaticTwoUint{Num1: 1})
		got = append(got, m)
	}

	for _, m := range got {
		m.Close()
	}

	m := PLParseInfo.Get()
	defer m.Close()

	if m.ListID != -1 || m.Retail || len(m.Episodes) != 0 {
		t.Fatalf("recycled ParseInfo not reset: ListID=%d Retail=%v Episodes=%v", m.ListID, m.Retail, m.Episodes)
	}
}

func TestSetDBEpisodeIDfromMIgnoresEmptyIdentifier(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbseries (id, seriename) values (1, 'Show')")
	mustExec(t, db, "insert into dbserie_episodes (id, dbserie_id, season, episode, identifier) values (1, 1, '', '', '')")
	mustExec(t, db, "insert into dbserie_episodes (id, dbserie_id, season, episode, identifier) values (2, 1, '1', '2', 'S01E02')")

	m := ParseInfo{DbserieID: 1, ListID: -1}
	m.SetDBEpisodeIDfromM()

	if m.DbserieEpisodeID != 0 {
		t.Fatalf("empty identifier matched episode %d", m.DbserieEpisodeID)
	}

	m = ParseInfo{DbserieID: 1, SeasonStr: "1", EpisodeStr: "2", ListID: -1}
	m.SetDBEpisodeIDfromM()

	if m.DbserieEpisodeID != 2 {
		t.Fatalf("season/episode lookup with empty identifier got %d, want 2", m.DbserieEpisodeID)
	}
}

func TestAlbumAndBookCreditChecks(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbartists (id, name, slug) values (1, 'Prince', 'prince')")
	mustExec(t, db, "insert into dbartist_aliases (dbartist_id, alias, slug) values (1, 'The Artist', 'the-artist')")
	mustExec(t, db, "insert into dbalbums (id, title, slug) values (1, 'Emancipation', 'emancipation')")
	mustExec(t, db, "insert into dbalbum_artists (dbalbum_id, dbartist_id) values (1, 1)")

	mustExec(t, db, "insert into dbauthors (id, name, slug) values (1, 'Main Author', 'main-author'), (2, 'Co Author', 'co-author')")
	mustExec(t, db, "insert into dbbooks (id, title, slug, dbauthor_id) values (1, 'Joint Work', 'joint-work', 1)")
	mustExec(t, db, "insert into dbbook_authors (dbbook_id, dbauthor_id) values (1, 2)")

	for _, tt := range []struct {
		artist string
		want   bool
	}{{"prince", true}, {"the artist", true}, {"Madonna", false}} {
		m := ParseInfo{Artist: tt.artist}
		if got := m.albumHasArtist(1); got != tt.want {
			t.Errorf("albumHasArtist(%q) = %v, want %v", tt.artist, got, tt.want)
		}
	}

	for _, tt := range []struct {
		author string
		want   bool
	}{{"main author", true}, {"Co Author", true}, {"Other", false}} {
		m := ParseInfo{Artist: tt.author}
		if got := m.bookHasAuthor(1); got != tt.want {
			t.Errorf("bookHasAuthor(%q) = %v, want %v", tt.author, got, tt.want)
		}
	}
}

func TestParseresolution(t *testing.T) {
	tests := []struct {
		w, h int
		want string
	}{
		{1920, 1080, "1080p"},
		{1920, 1079, "720p"},
		{1920, 1081, "1080p"},
		{1280, 720, "720p"},
		{3840, 2160, "2160p"},
		{7680, 4320, "4320p"},
		{720, 576, "576p"},
		{720, 480, "480p"},
		{640, 360, "360p"},
		{320, 240, "240p"},
		{160, 120, "SD"},
		// Portrait video is classified like its landscape counterpart.
		{1080, 1920, "1080p"},
		// Cinema scope (2.4:1) stays under the ultra-wide threshold.
		{1920, 800, "720p"},
		// Ultra-wide (> 2.5:1) is classified by width.
		{3840, 1080, "2160p"},
		{5120, 1440, "2880p"},
		{2560, 1000, "1440p"},
		{1920, 700, "1080p"},
		{1280, 500, "720p"},
		{800, 300, "480p"},
		{600, 200, "SD"},
	}

	for _, tt := range tests {
		m := ParseInfo{Width: tt.w, Height: tt.h}
		if got := m.Parseresolution(); got != tt.want {
			t.Errorf("Parseresolution(%dx%d) = %q, want %q", tt.w, tt.h, got, tt.want)
		}
	}
}

func TestParsegroup(t *testing.T) {
	m := ParseInfo{Str: "Movie.Title.2020.PROPER.1080p.BluRay.x264.DTS-GRP"}
	m.Parsegroup("resolution", false, []string{"720p", "1080p"})
	m.Parsegroup("quality", false, []string{"bluray", "webdl"})
	m.Parsegroup("codec", false, []string{"x264", "x265"})
	m.Parsegroup("audio", false, []string{"dts", "ac3"})
	m.Parsegroup("proper", false, []string{"proper"})
	m.Parsegroup("repack", false, []string{"repack"})
	m.Parsegroup("extended", false, []string{"extended"})

	if m.Resolution != "1080p" || m.Quality != "BluRay" || m.Codec != "x264" || m.Audio != "DTS" {
		t.Errorf("got resolution=%q quality=%q codec=%q audio=%q", m.Resolution, m.Quality, m.Codec, m.Audio)
	}

	if !m.Proper || m.Repack || m.Extended {
		t.Errorf("got proper=%t repack=%t extended=%t", m.Proper, m.Repack, m.Extended)
	}

	if want := len("Movie.Title.2020."); m.FirstIDX != want {
		t.Errorf("FirstIDX = %d, want %d", m.FirstIDX, want)
	}
}

func TestParsegroupWordBoundaries(t *testing.T) {
	// "dts" inside "DTSX" and "ac3" glued to a letter must not match.
	m := ParseInfo{Str: "Title.DTSX.xAC3"}
	m.Parsegroup("audio", false, []string{"dts", "ac3"})

	if m.Audio != "" {
		t.Errorf("Audio = %q, want empty", m.Audio)
	}

	if m.FirstIDX != 0 {
		t.Errorf("FirstIDX = %d, want 0", m.FirstIDX)
	}
}

func TestParsegroupOnlyIfEmpty(t *testing.T) {
	m := ParseInfo{Str: "Title.720p", Resolution: "1080p"}
	m.Parsegroup("resolution", true, []string{"720p"})

	if m.Resolution != "1080p" {
		t.Errorf("onlyifempty overwrote Resolution: %q", m.Resolution)
	}

	m.Parsegroup("resolution", false, []string{"720p"})

	if m.Resolution != "720p" {
		t.Errorf("Resolution = %q, want 720p", m.Resolution)
	}
}

func TestCleanRawNZBTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Artist - Record FLAC", "Artist - Record"},
		{"Artist - Record-WEB-FLAC", "Artist - Record"},
		{"Artist - Record (2020)", "Artist - Record"},
		{"Artist - Record [2020] MP3", "Artist - Record"},
		{"Artist_-_Record_2019", "Artist - Record"},
		{"Simon &amp; Garfunkel - Bookends", "Simon & Garfunkel - Bookends"},
		{"Author - Book [Audiobook]", "Author - Book"},
		{"Bravo Hits Vol.58", "Bravo Hits 58"},
		{"Bravo Hits Volume 12 CD", "Bravo Hits 12"},
		{"Artist  -   Record", "Artist - Record"},
		// Words that are also title words are only stripped as dash-joined tags.
		{"Metallica - The Black Album", "Metallica - The Black Album"},
		{"Pearl Jam - Live", "Pearl Jam - Live"},
		{"Aerosmith - Classics Live (1986) MP3", "Aerosmith - Classics Live"},
		{"A$AP Rocky - Long Live A$AP (2012) MP3", "A$AP Rocky - Long Live A$AP"},
		{"2Pac-Dear.Mama-CDM-1995-GCP.INT", "2Pac-Dear Mama-CDM-1995-GCP"},
		{"ALIAS - Fever Dream WEB (2011) MP3", "ALIAS - Fever Dream"},
		{"Various - Bravo Hits 128", "Various - Bravo Hits 128"},
		{"Prince - The Black Album-WEB-FLAC-2016-GRP", "Prince - The Black Album"},
		{"Artist - Record-LIVE-WEB-FLAC", "Artist - Record"},
		{"Artist - Record-320-WEB", "Artist - Record"},
		{"Artist-Live", "Artist-Live"},
		{"Artist - Record Deluxe Edition FLAC", "Artist - Record"},
		// Tag words inside the title are kept.
		{"Weblife - Record", "Weblife - Record"},
	}

	for _, tt := range tests {
		if got := cleanRawNZBTitle(tt.in); got != tt.want {
			t.Errorf("cleanRawNZBTitle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCleanSceneGroupFromAlbum(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Album Title", "Album Title"},
		{"Album Title-WEB-2020-GRP", "Album Title"},
		{"Album Title-FLAC", "Album Title"},
		{"Album - Part Two-CD-FLAC", "Album - Part Two"},
		// The first segment is always kept, even if it looks like a tag.
		{"OST-WEB", "OST"},
		{"Album--Deluxe Mix", "Album - Deluxe Mix"},
	}

	for _, tt := range tests {
		if got := cleanSceneGroupFromAlbum(tt.in); got != tt.want {
			t.Errorf("cleanSceneGroupFromAlbum(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLooksLikeSceneTag(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"FLAC", true},
		{"flac", true},
		{"2020", true},
		{"1999", true},
		{"2CD", true},
		{"GRP", true},
		{" WEB ", true},
		{"1800", false},
		{"Grp", false},
		{"A", false},
		{"123", false},
		{"Part Two", false},
		{"VERYLONGGROUPNAME", false},
	}

	for _, tt := range tests {
		if got := looksLikeSceneTag(tt.in); got != tt.want {
			t.Errorf("looksLikeSceneTag(%q) = %t, want %t", tt.in, got, tt.want)
		}
	}
}

func TestExpandVANames(t *testing.T) {
	tests := []struct {
		in, want []string
	}{
		{[]string{"Artist"}, []string{"Artist"}},
		{[]string{"VA"}, []string{"VA", "Various Artists"}},
		{[]string{" v.a. "}, []string{" v.a. ", "Various Artists"}},
		{[]string{"various artists"}, []string{"various artists", "VA"}},
		{[]string{"Artist", "V.A"}, []string{"Artist", "V.A", "Various Artists"}},
		{nil, nil},
	}

	for _, tt := range tests {
		if got := expandVANames(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("expandVANames(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseEpisodeList(t *testing.T) {
	tests := []struct {
		in   string
		want []int
	}{
		{"E01", []int{1}},
		{"E01E02E03", []int{1, 2, 3}},
		{"E01-E03", []int{1, 2, 3}},
		{"e05-e06", []int{5, 6}},
		{"E01-03", []int{1, 2, 3}},
		{"x05", []int{5}},
		{"E100", []int{100}},
		{"E03-E01", []int{3, 1}},
		{"E01-E99", []int{1, 99}},
		{"", nil},
	}

	for _, tt := range tests {
		if got := parseEpisodeList(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("parseEpisodeList(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestPadEpisodeNumber(t *testing.T) {
	for n, want := range map[int]string{0: "00", 5: "05", 12: "12", 100: "100"} {
		if got := padEpisodeNumber(n); got != want {
			t.Errorf("padEpisodeNumber(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestPrefixTitleMatch(t *testing.T) {
	tests := []struct {
		release, stored string
		want            bool
	}{
		// Other volumes of a series must not match the shorter title.
		{"Bravo Hits 130", "Bravo Hits", false},
		{"BRAVO Hits 114-2CD", "Bravo Hits", false},
		{"Progressive Trance Awakening Vol. 15-16BIT-WEB-FLAC", "Progressive Trance", false},
		{"Trance Voices Vol. 23-2CD-FLAC", "Trance Voices", false},
		{"Memory Man 07 . Long Shadows", "Memory Man", false},
		{"Die Zeitmaschine Teil 2-DE-AUDIOBOOK", "Die Zeitmaschine", false},
		{"Cabin Fever 2 (2012) MP3", "Cabin Fever", false},
		// Edition, year, catalog and scene tags still match.
		{"Bravo Hits", "Bravo Hits", true},
		{"All Killer No Filler-25th Anniversary Edition-Remastered-CD-FLAC", "All Killer No Filler", true},
		{"Triple 9 (Original Motion Picture Soundtrack)-WEB", "Triple 9", true},
		{"Helden der Schlacht (Die Rom-Serie 18) [ungekürzt]", "Helden der Schlacht", true},
		{"Top 50 Rock Vibes 2024", "Top 50", true},
		{"The End of Eternity NMR 80 kbps", "The End of Eternity", true},
		// Slug fallback: catalog numbers are not volumes, short numbers are.
		{"Stop Loving Me Stop Loving You 659818 2 CDM", "Stop Loving Me, Stop Loving You", true},
		{"Prism Acoustic Sessions 320 kbps", "PRISM (Acoustic Sessions)", true},
		{"Bravo.Hits.130", "Bravo Hits", false},
	}

	for _, tt := range tests {
		got := prefixTitleMatch(
			tt.release,
			logger.StringToSlugCached(tt.release),
			tt.stored,
			logger.StringToSlugCached(tt.stored),
		)
		if got != tt.want {
			t.Errorf("prefixTitleMatch(%q, %q) = %v, want %v", tt.release, tt.stored, got, tt.want)
		}
	}
}

func TestLongestPrefixMatch(t *testing.T) {
	stored := []string{"The Dome", "Bravo Hits", "The Dome: Summer 2006", "Bravo Hits 130", "Dome"}
	get := func(i int) (string, string) {
		return stored[i], logger.StringToSlugCached(stored[i])
	}

	tests := []struct {
		release string
		want    int
	}{
		{"The Dome Summer 2006-(060249841313)-2CD-FLAC", 2},
		{"Bravo Hits 130-2CD", 3},
		{"Bravo Hits 131-2CD", -1},
		{"Dome Live", -1},
	}

	for _, tt := range tests {
		if got := longestPrefixMatch(tt.release, logger.StringToSlugCached(tt.release), len(stored), get); got != tt.want {
			t.Errorf("longestPrefixMatch(%q) = %d, want %d", tt.release, got, tt.want)
		}
	}
}

func TestStripTitlePrefixPostfixGetQual(t *testing.T) {
	qual := &config.QualityConfig{
		TitleStripSuffixForSearch: []string{"[shield]", "-proper"},
		TitleStripPrefixForSearch: []string{"[ www.example.org ]"},
	}

	tests := []struct{ in, want string }{
		{"", ""},
		{"Movie Title", "Movie Title"},
		{"Movie Title [SHIELD] 1080p", "Movie Title"},
		{"Movie.Title.-PROPER", "Movie.Title"},
		{"[ WWW.EXAMPLE.ORG ] - Movie Title", "Movie Title"},
		{"[ www.example.org ]Movie", "Movie"},
		// A match that would leave nothing keeps the title.
		{"[shield]", "[shield]"},
		{"[ www.example.org ]", "[ www.example.org ]"},
	}

	for _, tt := range tests {
		m := ParseInfo{Title: tt.in}
		m.StripTitlePrefixPostfixGetQual(qual)

		if m.Title != tt.want {
			t.Errorf("StripTitlePrefixPostfixGetQual(%q) = %q, want %q", tt.in, m.Title, tt.want)
		}
	}
}

func TestRegexGetMatchesStr1(t *testing.T) {
	db := useMigratedMainDB(t)

	SetStaticRegexp(strRegexSeriesTitle)
	SetStaticRegexp(strRegexSeriesTitleDate)

	mustExec(t, db, "insert into dbseries (id, seriename, slug) values (1, 'The Daily Show', 'the-daily-show')")
	mustExec(t, db, "insert into dbseries (id, seriename, slug) values (2, 'Grey''s Anatomy', 'greys-anatomy')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (2, 'Die jungen Ärzte', 'die-jungen-arzte')")

	series := &config.MediaTypeConfig{IsType: config.MediaTypeSeries}

	tests := []struct {
		file, title, date string
		want              uint
	}{
		{"/tv/The.Daily.Show.S29E10.720p.WEB.mkv", "", "", 1},
		{"/tv/The.Daily.Show.2024.03.14.720p.WEB.mkv", "", "2024-03-14", 1},
		{"/tv/Greys Anatomy - S20E01 - Episode.mkv", "", "", 2}, // slug fallback
		{"/tv/Die jungen Ärzte - S01E02.mkv", "", "", 2},        // alternate title
		{"/tv/Unknown.Show.S01E01.mkv", "", "", 0},
		{"/tv/no-episode-marker.mkv", "", "", 0},
		// Already parsed to the same title: no second lookup.
		{"/tv/The.Daily.Show.S29E10.mkv", "The Daily Show", "", 0},
	}

	for _, tt := range tests {
		m := ParseInfo{File: tt.file, Title: tt.title, Date: tt.date}
		m.RegexGetMatchesStr1(series)

		if m.DbserieID != tt.want {
			t.Errorf("RegexGetMatchesStr1(%q) DbserieID = %d, want %d", tt.file, m.DbserieID, tt.want)
		}
	}
}

func TestPreferOriginalDbserie(t *testing.T) {
	db := useMigratedMainDB(t)

	// Shaped like production: the 1998 show's main name is its German title,
	// so the bare "Charmed" name lookup hits the 2018 remake first.
	mustExec(t, db, "insert into dbseries (id, seriename, slug, firstaired) values (48, 'Charmed - Zauberhafte Hexen', 'charmed-zauberhafte-hexen', '1998-10-07')")
	mustExec(t, db, "insert into dbseries (id, seriename, slug, firstaired) values (49, 'Charmed', 'charmed', '2018-10-14')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (48, 'Charmed', 'charmed')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (49, 'Charmed (2018)', 'charmed-2018')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (48, 'Charmed (1998)', 'charmed-1998')")
	mustExec(t, db, "insert into dbseries (id, seriename, slug, firstaired) values (105, 'MacGyver', 'macgyver', '2016-09-23')")
	mustExec(t, db, "insert into dbseries (id, seriename, slug, firstaired) values (263, 'MacGyver', 'macgyver', '1985-09-29')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (105, 'MacGyver 2016', 'macgyver-2016')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (263, 'MacGyver (1985)', 'macgyver-1985')")
	// A title shared only through a stray alias: no year-qualified names, so
	// the main-name hit stands although the alias owner is older.
	mustExec(t, db, "insert into dbseries (id, seriename, slug, firstaired) values (144, 'Quantico', 'quantico', '2015-08-30')")
	mustExec(t, db, "insert into dbseries (id, seriename, slug, firstaired) values (246, 'Criminal Minds', 'criminal-minds', '2005-09-22')")
	mustExec(t, db, "insert into dbserie_alternates (dbserie_id, title, slug) values (246, 'Quantico', 'quantico')")

	tests := []struct {
		title string
		year  uint16
		want  uint
	}{
		{"Charmed", 0, 48},    // bare name is the original
		{"charmed", 0, 48},    // case-insensitive
		{"Charmed", 2018, 49}, // year picks the remake
		{"MacGyver", 0, 263},
		{"MacGyver", 2016, 105},
		{"MacGyver", 1990, 263}, // no year match: oldest
		{"Quantico", 0, 144},
		{"Charmed - Zauberhafte Hexen", 0, 48},
		{"Unknown Show", 0, 0},
	}

	for _, tt := range tests {
		m := ParseInfo{Year: tt.year}
		m.FindDbserieByNameWithSlug(tt.title)
		m.PreferOriginalDbserie(tt.title)

		if m.DbserieID != tt.want {
			t.Errorf("%q (%d): DbserieID = %d, want %d", tt.title, tt.year, m.DbserieID, tt.want)
		}
	}
}
