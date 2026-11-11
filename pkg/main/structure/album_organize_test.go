package structure

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
)

func TestMatchReportDuration(t *testing.T) {
	for in, want := range map[int64]string{
		-5: "0:00", 0: "0:00", 999: "0:00", 1000: "0:01", 65_000: "1:05",
		3_599_000: "59:59", 3_600_000: "1:00:00", 36_061_000: "10:01:01",
	} {
		if got := matchReportDuration(in); got != want {
			t.Errorf("matchReportDuration(%d) = %q, want %q", in, got, want)
		}
	}
}

func albumOrganizer(lib, dl string, replaceLower, moveReplaced bool) *Organizer {
	target := &config.PathsConfig{
		Name: "music", Path: lib, Replacelower: replaceLower, MoveReplaced: moveReplaced,
		MoveReplacedTargetPath: filepath.Join(filepath.Dir(lib), "replaced"),
	}

	return &Organizer{
		Cfgp: &config.MediaTypeConfig{IsType: config.MediaTypeMusic, DataMap: map[int]*config.MediaDataConfig{0: {CfgPath: target}}},
		sourcepathCfg: &config.PathsConfig{
			Name: "music nzb", Path: dl,
			AllowedAudioExtensions: []string{".flac", ".mp3"}, AllowedAudioExtensionsLen: 2,
		},
		targetpathCfg: target,
	}
}

// TestMoveAlbumIntoPlaceReplacesWholeAlbum: the new release goes into a new
// folder; every old track, its leftovers and the emptied old folders go.
func TestMoveAlbumIntoPlaceReplacesWholeAlbum(t *testing.T) {
	for _, moveReplaced := range []bool{false, true} {
		useTestDB(t)

		base := t.TempDir()
		lib, dl := filepath.Join(base, "Music"), filepath.Join(base, "dl")
		oldDir := filepath.Join(lib, "G", "General Base", "In Trance (1992)")
		oldTracks := []string{filepath.Join(oldDir, "CD1", "01 - In Trance.mp3"), filepath.Join(oldDir, "CD1", "02 - Base Of Love.mp3")}
		writeFiles(t, append(oldTracks, filepath.Join(oldDir, "cover.jpg"), filepath.Join(oldDir, "CD1", "album.cue"))...)

		rel := filepath.Join(dl, "General_Base-In_Trance-WEB-FLAC")
		newTracks := []string{filepath.Join(rel, "01-in_trance.flac"), filepath.Join(rel, "02-base_of_love.flac")}
		writeFiles(t, newTracks...)

		var old []database.AudioFilePrio
		for _, f := range oldTracks {
			old = append(old, database.AudioFilePrio{Location: f})
		}

		s := albumOrganizer(lib, dl, !moveReplaced, moveReplaced)
		target := filepath.Join(lib, "G", "General Base", "In Trance")
		o := &Organizerdata{Folder: rel, MediaFiles: newTracks, Filenames: []string{"01 - In Trance", "02 - Base Of Love"}, Foldername: filepath.Join("G", "General Base", "In Trance")}

		if _, err := s.moveAlbumIntoPlace(context.Background(), rel, o, old, true); err != nil {
			t.Fatal(err)
		}

		if got := listDir(t, target); !slices.Equal(got, []string{"01 - In Trance.flac", "02 - Base Of Love.flac"}) {
			t.Errorf("move=%t: new album = %v", moveReplaced, got)
		}

		if exists(oldDir) {
			t.Errorf("move=%t: old album folder still there: %v", moveReplaced, listDir(t, oldDir))
		}

		if !exists(filepath.Join(lib, "G", "General Base")) {
			t.Errorf("move=%t: artist folder holding the new album was removed", moveReplaced)
		}

		replaced := filepath.Join(base, "replaced")
		if moveReplaced {
			if got := listDir(t, replaced); len(got) != 4 {
				t.Errorf("replaced folder = %v, want old tracks, cover and cue", got)
			}
		} else if exists(replaced) {
			t.Error("replace_lower moved files instead of deleting them")
		}
	}
}

func TestLockAlbumSerializesSameAlbum(t *testing.T) {
	unlock := lockAlbum(config.MediaTypeMusic, 42)

	acquired := make(chan struct{})
	go func() {
		release := lockAlbum(config.MediaTypeMusic, 42)
		close(acquired)
		release()
	}()

	// Another album, or the same id of another media type, is not blocked.
	lockAlbum(config.MediaTypeMusic, 43)()
	lockAlbum(config.MediaTypeAudiobook, 42)()

	select {
	case <-acquired:
		t.Fatal("second import of the same album ran while the first held the lock")
	case <-time.After(50 * time.Millisecond):
	}

	unlock()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("waiting import never got the lock")
	}
}

func TestTruncateUTF8(t *testing.T) {
	s := "Ärger über Öl"
	for n := 0; n <= len(s)+1; n++ {
		got := truncateUTF8(s, n)
		if !utf8.ValidString(got) || len(got) > n || !strings.HasPrefix(s, got) {
			t.Fatalf("truncateUTF8(%q, %d) = %q", s, n, got)
		}
	}
}

func TestAppendRenameLogKeepsEarlierEntries(t *testing.T) {
	p := filepath.Join(t.TempDir(), "_rename_log.txt")

	appendRenameLog(p, "first\n")
	appendRenameLog(p, "second\n")

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(b); got != "first\n\nsecond\n" {
		t.Errorf("rename log = %q", got)
	}
}

func TestBuildAlbumDataConfigCopiesRecThresholds(t *testing.T) {
	cfgp := &config.MediaTypeConfig{Data: []config.MediaDataConfig{{StrongRecThresh: 0.07, MediumRecThresh: 0.5}}}

	got := AlbumDataConfig(&config.MediaDataImportConfig{MediumRecThresh: 0.4}, cfgp)
	if got.MediumRecThresh != 0.4 || got.StrongRecThresh != 0.07 {
		t.Errorf("thresholds = %v / %v, want 0.07 / 0.4 (import wins, data fills gaps)", got.StrongRecThresh, got.MediumRecThresh)
	}
}

func TestAlbumUpgradeCheck(t *testing.T) {
	dir := t.TempDir()
	kept := filepath.Join(dir, "lib", "1 - Kapitel 1.mp3")
	source := filepath.Join(dir, "src", "a.mp3")

	for _, f := range []string{kept, source} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	album := &parser_v2.AlbumInfo{Tracks: []parser_v2.TrackInfo{{Filepath: source, Format: "mp3"}}}
	old := []database.AudioFilePrio{
		{Location: kept, Format: "mp3", Bitrate: 128},
		{Location: filepath.Join(dir, "lib", "gone.mp3"), Format: "mp3"}, // file deleted
		{Location: source, Format: "mp3"},                                // in-place reorganize
	}

	// No list / quality profile: the library files are reported but nothing
	// is compared, so the import goes ahead.
	check := albumUpgradeCheck(&config.MediaTypeConfig{}, 0, album, old)

	want := []AlbumLibraryFile{{Location: kept, Format: "mp3", Bitrate: 128}}
	if !slices.Equal(check.Existing, want) {
		t.Errorf("Existing = %+v, want %+v", check.Existing, want)
	}

	if !check.Allowed || check.OldPriority != 0 || check.NewPriority != 0 || check.NewFiles != 1 {
		t.Errorf("check = %+v, want allowed without priorities and 1 new file", check)
	}

	if check := albumUpgradeCheck(&config.MediaTypeConfig{}, 0, album, old[1:]); len(check.Existing) != 0 || !check.Allowed {
		t.Errorf("only stale rows: check = %+v, want no existing files and allowed", check)
	}
}

func TestAssignAudiobookChapters(t *testing.T) {
	chapters := func(lengths ...int64) []apiexternal_v2.AudiobookChapter {
		out := make([]apiexternal_v2.AudiobookChapter, len(lengths))
		for i, l := range lengths {
			out[i] = apiexternal_v2.AudiobookChapter{Title: "Kapitel " + strconv.Itoa(i+1), LengthMs: l}
		}

		return out
	}

	files := func(runtimes ...int64) []parser_v2.TrackInfo {
		out := make([]parser_v2.TrackInfo, len(runtimes))
		for i, r := range runtimes {
			out[i] = parser_v2.TrackInfo{RuntimeMS: r}
		}

		return out
	}

	cases := []struct {
		name     string
		chapters []apiexternal_v2.AudiobookChapter
		tracks   []parser_v2.TrackInfo
		want     []string
		spans    [][2]int
		note     string
	}{
		{
			"same count: by position",
			chapters(100_000, 200_000), files(150_000, 150_000),
			[]string{"Kapitel 1", "Kapitel 2"}, [][2]int{{1, 1}, {2, 2}},
			"one file per chapter",
		},
		{
			// Chapters of 10 and 5 min, files of 5 min: three files.
			"more files: chapter each file starts in",
			chapters(600_000, 300_000), files(300_000, 300_000, 300_000),
			[]string{"Kapitel 1", "Kapitel 1", "Kapitel 2"}, [][2]int{{1, 1}, {1, 1}, {2, 2}},
			"3 files for 2 chapters, mapped by runtime (files 0:15:00, chapters 0:15:00, difference 0.0s)",
		},
		{
			// Files run 1 s short of the chapters; scaling and the slack keep
			// each file on the two chapters it holds.
			"fewer files: first chapter of each file",
			chapters(300_000, 300_000, 300_000, 300_000), files(599_500, 599_500),
			[]string{"Kapitel 1", "Kapitel 3"}, [][2]int{{1, 2}, {3, 4}},
			"2 files for 4 chapters",
		},
		{
			"chapter with Duration only",
			[]apiexternal_v2.AudiobookChapter{{Title: "A", Duration: time.Minute}, {Title: "B", Duration: time.Minute}},
			files(30_000, 30_000, 60_000),
			[]string{"A", "A", "B"}, [][2]int{{1, 1}, {1, 1}, {2, 2}},
			"mapped by runtime",
		},
		{
			"single file: all chapters",
			chapters(100_000, 200_000), files(300_000),
			[]string{"Kapitel 1"}, [][2]int{{1, 2}},
			"one file for all 2 chapters",
		},
	}

	for _, tc := range cases {
		got, note := assignAudiobookChapters(tc.chapters, tc.tracks)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: titles %v, want %v", tc.name, got, tc.want)
		}

		for i := range tc.tracks {
			if span := [2]int{tc.tracks[i].ChapterFirst, tc.tracks[i].ChapterLast}; span != tc.spans[i] {
				t.Errorf("%s: track %d covers %v, want %v", tc.name, i+1, span, tc.spans[i])
			}
		}

		if !strings.Contains(note, tc.note) {
			t.Errorf("%s: note %q lacks %q", tc.name, note, tc.note)
		}
	}
}

func TestWriteChapterSpan(t *testing.T) {
	titles := []string{"Vorspann", "Prolog", "Kapitel 1"}

	cases := []struct {
		first, last int
		want        string
	}{
		{2, 2, ` chapter=2 "Prolog"`},
		{1, 3, ` chapters=1-3 "Vorspann" .. "Kapitel 1"`},
		{0, 0, ""},
		{2, 4, ""},
	}

	for _, tc := range cases {
		var b strings.Builder
		writeChapterSpan(&b, &parser_v2.TrackInfo{ChapterFirst: tc.first, ChapterLast: tc.last}, titles)

		if b.String() != tc.want {
			t.Errorf("%d-%d: got %q, want %q", tc.first, tc.last, b.String(), tc.want)
		}
	}
}

func TestEmbeddedChapterNote(t *testing.T) {
	own := []tags.Chapter{{Title: "001 - Kapitel 1.1", RuntimeMS: 1000}, {Title: "002 - Kapitel 1.2", StartMS: 1000, RuntimeMS: 1000}}
	titled := []tags.Chapter{{Title: "Kapitel 1", RuntimeMS: 1000}, {Title: "Kapitel 1", StartMS: 1000, RuntimeMS: 1000}}
	replaced := []tags.Chapter{{Title: "Kapitel 1", RuntimeMS: 2000}}

	tests := []struct {
		name string
		t    parser_v2.TrackInfo
		book int
		want string
	}{
		{"no chapters", parser_v2.TrackInfo{}, 3, ""},
		{"kept", parser_v2.TrackInfo{Chapters: own}, 3, "its 2 embedded chapters are kept"},
		{"same cuts", parser_v2.TrackInfo{Chapters: own, ReleaseChapters: titled}, 2,
			"its 2 embedded chapters get the book's chapter titles"},
		{"titled by start", parser_v2.TrackInfo{Chapters: own, ReleaseChapters: titled}, 1,
			"its 2 embedded chapters are kept, each titled after the book chapter it starts in"},
		{"replaced", parser_v2.TrackInfo{Chapters: own, ReleaseChapters: replaced}, 1,
			"its 2 embedded chapters are replaced by the book's 1, start times scaled to the file's runtime"},
	}

	for _, tt := range tests {
		if got := embeddedChapterNote(&tt.t, tt.book); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCollectAlbumFolders(t *testing.T) {
	cfgp := &config.MediaTypeConfig{IsType: config.MediaTypeAudiobook}
	pathCfg := &config.PathsConfig{AllowedAudioExtensions: []string{".mp3"}, AllowedAudioExtensionsLen: 1}

	mk := func(t *testing.T, files ...string) string {
		t.Helper()

		root := t.TempDir()
		for _, f := range files {
			p := filepath.Join(root, f)
			if strings.HasSuffix(f, "/") {
				if err := os.MkdirAll(p, 0o755); err != nil {
					t.Fatal(err)
				}

				continue
			}

			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		return root
	}

	rel := func(root string, dirs []string) []string {
		out := make([]string, len(dirs))
		for i := range dirs {
			r, _ := filepath.Rel(root, dirs[i])
			out[i] = filepath.ToSlash(r)
		}

		return out
	}

	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{"audio at the top", []string{"01.mp3", "cover.jpg"}, []string{"."}},
		{"artist/album nesting", []string{"A/01.mp3", "B/01.mp3", "C/info.txt"}, []string{"A", "B"}},
		{"disc set", []string{"CD1/01.mp3", "CD2/01.mp3"}, []string{"."}},
		// No album anywhere below: the folder itself, to be unpacked or
		// moved to move_unprocessed as no_files.
		{"leftover cover and info", []string{"Info-Datei.txt", "Sub/folder.jpg"}, []string{"."}},
		{"archive only", []string{"book.rar", "book_unpack/"}, []string{"."}},
		{"empty", nil, []string{"."}},
	}

	for _, tt := range tests {
		root := mk(t, tt.files...)
		if got := rel(root, CollectAlbumFolders(context.Background(), root, cfgp, pathCfg)); !slices.Equal(got, tt.want) {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}
