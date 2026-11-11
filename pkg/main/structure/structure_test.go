package structure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/books"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

func TestTrimStringInclAfterString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		search   string
		expected string
	}{
		{
			name:     "Empty strings",
			input:    "",
			search:   "",
			expected: "",
		},
		{
			name:     "Search string not found",
			input:    "hello world",
			search:   "xyz",
			expected: "hello world",
		},
		{
			name:     "Search string at start",
			input:    "test string content",
			search:   "test",
			expected: "",
		},
		{
			name:     "Search string in middle",
			input:    "hello test world",
			search:   "test",
			expected: "hello ",
		},
		{
			name:     "Search string at end",
			input:    "hello world test",
			search:   "test",
			expected: "hello world ",
		},
		{
			name:     "Case insensitive search",
			input:    "Hello TEST World",
			search:   "test",
			expected: "Hello ",
		},
		{
			name:     "Multiple occurrences",
			input:    "test one test two test",
			search:   "test",
			expected: "",
		},
		{
			name:     "Search string longer than input",
			input:    "test",
			search:   "testing",
			expected: "test",
		},
		{
			name:     "Unicode characters",
			input:    "Hello 世界 World",
			search:   "世界",
			expected: "Hello ",
		},
		{
			name:     "Special characters",
			input:    "Hello!@#$%^&* World",
			search:   "!@#$%^&*",
			expected: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimStringInclAfterString(tt.input, tt.search)
			if result != tt.expected {
				t.Errorf(
					"trimStringInclAfterString(%q, %q) = %q, expected %q",
					tt.input,
					tt.search,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestStringRemoveAllRunes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		remove   byte
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			remove:   'a',
			expected: "",
		},
		{
			name:     "No matching runes",
			input:    "hello",
			remove:   'x',
			expected: "hello",
		},
		{
			name:     "Single character removal",
			input:    "hello",
			remove:   'l',
			expected: "heo",
		},
		{
			name:     "Multiple character removal",
			input:    "mississippi",
			remove:   'i',
			expected: "msssspp",
		},
		{
			name:     "All same characters",
			input:    "aaaaa",
			remove:   'a',
			expected: "",
		},
		{
			name:     "Special characters",
			input:    "!@#$%^&*",
			remove:   '@',
			expected: "!#$%^&*",
		},
		{
			name:     "Numbers",
			input:    "123454321",
			remove:   '3',
			expected: "1245421",
		},
		{
			name:     "Mixed content",
			input:    "a1b2c3d4",
			remove:   '2',
			expected: "a1bc3d4",
		},
		{
			name:     "First character",
			input:    "testing",
			remove:   't',
			expected: "esing",
		},
		{
			name:     "Last character",
			input:    "testing",
			remove:   'g',
			expected: "testin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringRemoveAllRunes(tt.input, tt.remove)
			if result != tt.expected {
				t.Errorf(
					"stringRemoveAllRunes(%q, %q) = %q, expected %q",
					tt.input,
					tt.remove,
					result,
					tt.expected,
				)
			}
		})
	}
}

// cfgpWithPath builds a minimal *config.MediaTypeConfig whose DataMap has a
// single entry pointing at the given storage path, for exercising
// computeRootpath without depending on any real, loaded config snapshot.
func cfgpWithPath(path string) *config.MediaTypeConfig {
	return &config.MediaTypeConfig{
		DataMap: map[int]*config.MediaDataConfig{
			0: {CfgPath: &config.PathsConfig{Path: path}},
		},
	}
}

func TestUpdateRootpath(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		cfgp       *config.MediaTypeConfig
		expected   string
		expectedOk bool
	}{
		{
			name:       "No matching path in config",
			file:       "/some/random/path/file.txt",
			cfgp:       &config.MediaTypeConfig{},
			expectedOk: false,
		},
		{
			// The file sits directly in the configured path with no
			// subfolder between them, so there is no "first folder" to
			// extract - computeRootpath falls back to joining the bare
			// filename back onto the config path.
			name:       "File directly in configured path (no subfolder)",
			file:       "/media/photos/vacation.jpg",
			cfgp:       cfgpWithPath("/media/photos"),
			expected:   "/media/photos/vacation.jpg",
			expectedOk: true,
		},
		{
			// Only the first subfolder below the configured path becomes
			// the rootpath - "summer" is dropped, matching getrootpath's
			// documented "first folder only" behavior.
			name:       "Nested directory match keeps only the first subfolder",
			file:       "/content/videos/2023/summer/video.mp4",
			cfgp:       cfgpWithPath("/content/videos"),
			expected:   "/content/videos/2023",
			expectedOk: true,
		},
		{
			// filepath.Join always uses this OS's separator ("/" on Linux,
			// where these tests run) regardless of the input path's own
			// separator style, so a Windows-style config path ends up with
			// a mixed-separator result here - this documents that
			// platform-dependent behavior rather than asserting a
			// Windows-only outcome.
			name:       "Windows style paths (joined with this OS's separator)",
			file:       "C:\\Users\\Media\\Pictures\\photo.jpg",
			cfgp:       cfgpWithPath("C:\\Users\\Media\\Pictures"),
			expected:   "C:\\Users\\Media\\Pictures/photo.jpg",
			expectedOk: true,
		},
		{
			name: "Multiple config entries - only one path matches",
			file: "/data/music/album/song.mp3",
			cfgp: &config.MediaTypeConfig{
				DataMap: map[int]*config.MediaDataConfig{
					0: {CfgPath: &config.PathsConfig{Path: "/data/books"}},
					1: {CfgPath: &config.PathsConfig{Path: "/data/music"}},
				},
			},
			expected:   "/data/music/album",
			expectedOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := computeRootpath(tt.file, tt.cfgp)
			if ok != tt.expectedOk {
				t.Fatalf("computeRootpath() ok = %v, want %v", ok, tt.expectedOk)
			}
			if ok && got != tt.expected {
				t.Errorf("computeRootpath() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestChecksplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected byte
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: ' ',
		},
		{
			name:     "Forward slash path",
			input:    "path/to/folder",
			expected: '/',
		},
		{
			name:     "Backslash path",
			input:    "path\\to\\folder",
			expected: '\\',
		},
		{
			name:     "No slashes",
			input:    "simple-folder-name",
			expected: ' ',
		},
		{
			name:     "Single forward slash",
			input:    "/",
			expected: '/',
		},
		{
			name:     "Single backslash",
			input:    "\\",
			expected: '\\',
		},
		{
			name:     "Mixed slashes with forward slash first",
			input:    "path/to\\folder",
			expected: '/',
		},
		{
			name:     "Mixed slashes with backslash first",
			input:    "path\\to/folder",
			expected: '\\',
		},
		{
			name:     "Special characters without slashes",
			input:    "folder!@#$%^&*()",
			expected: ' ',
		},
		{
			name:     "Unicode characters without slashes",
			input:    "文件夹",
			expected: ' ',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checksplit(tt.input)
			if result != tt.expected {
				t.Errorf("checksplit(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetRootPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple folder name without slashes",
			input:    "folder",
			expected: "folder",
		},
		{
			name:     "Forward slash path",
			input:    "root/subfolder/file",
			expected: "root",
		},
		{
			name:     "Backslash path",
			input:    "root\\subfolder\\file",
			expected: "root",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single forward slash",
			input:    "/",
			expected: "",
		},
		{
			name:     "Single backslash",
			input:    "\\",
			expected: "\\",
		},
		{
			name:     "Root folder with trailing slash",
			input:    "root/",
			expected: "root",
		},
		{
			name:     "Multiple forward slashes",
			input:    "root///subfolder",
			expected: "root",
		},
		{
			name:     "Mixed slashes",
			input:    "root/subfolder\\file",
			expected: "root",
		},
		{
			name:     "Path starting with slash",
			input:    "/root/folder",
			expected: "",
		},
		{
			name:     "Complex path with dots",
			input:    "../root/folder",
			expected: "..",
		},
		{
			name:     "Path with spaces",
			input:    "my folder/subfolder",
			expected: "my folder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getrootpath(tt.input)
			if result != tt.expected {
				t.Errorf("getrootpath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTestInputnotifier(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
		contains []string // strings that should be in the result
	}{
		{
			name:     "Simple movie template",
			template: "{{.Dbmovie.Title}} ({{.Dbmovie.Year}})",
			wantErr:  false,
			contains: []string{"Inception", "(2000)"},
		},
		{
			name:     "Series template",
			template: "{{.Dbserie.Seriename}} S{{.DbserieEpisode.Season}}E{{.DbserieEpisode.Episode}}",
			wantErr:  false,
			contains: []string{"Breaking Bad", "S1E1"},
		},
		{
			name:     "Template with source info",
			template: "{{.Source.Quality}} {{.Source.Resolution}} {{.Source.Codec}}",
			wantErr:  false,
			contains: []string{"bluray", "1080p", "x264"},
		},
		{
			name:     "Template with notification fields",
			template: "{{.Title}} from {{.SourcePath}} to {{.Targetpath}}",
			wantErr:  false,
			contains: []string{},
		},
		{
			name:     "Invalid template syntax",
			template: "{{.Invalid}",
			wantErr:  true,
			contains: []string{},
		},
		{
			name:     "Empty template",
			template: "",
			wantErr:  false,
			contains: []string{},
		},
		{
			name:     "Template with conditionals",
			template: "{{if .Dbmovie.Title}}{{.Dbmovie.Title}}{{end}}",
			wantErr:  false,
			contains: []string{"Inception"},
		},
		{
			name:     "Template with replaced files",
			template: "Replaced: {{range .Replaced}}{{.}} {{end}}",
			wantErr:  false,
			contains: []string{"Replaced:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TestInputnotifier(tt.template)

			if tt.wantErr && err == nil {
				t.Errorf("TestInputnotifier() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("TestInputnotifier() unexpected error: %v", err)
			}

			// Check if expected strings are contained in result
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf(
						"TestInputnotifier() result should contain %q, got: %q",
						expected,
						result,
					)
				}
			}
		})
	}
}

func TestTestParsertype(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
		contains []string
	}{
		{
			name:     "Movie parser template",
			template: "{{.Dbmovie.Title}} ({{.Dbmovie.Year}})",
			wantErr:  false,
			contains: []string{"Inception", "(2000)"},
		},
		{
			name:     "Series parser template",
			template: "{{.Dbserie.Seriename}}/Season {{.DbserieEpisode.Season}}",
			wantErr:  false,
			contains: []string{"Breaking Bad", "Season 1"},
		},
		{
			name:     "Source template with quality info",
			template: "[{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}}]",
			wantErr:  false,
			contains: []string{"[1080p", "bluray", "x264]"},
		},
		{
			name:     "Episode identifier template",
			template: "{{.Identifier}}",
			wantErr:  false,
			contains: []string{"S01E01"},
		},
		{
			name:     "Complex template with conditionals",
			template: "{{.TitleSource}}{{if .Source.Proper}} proper{{end}}",
			wantErr:  false,
			contains: []string{},
		},
		{
			name:     "Invalid template",
			template: "{{.NonExistent",
			wantErr:  true,
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TestParsertype(tt.template)

			if tt.wantErr && err == nil {
				t.Errorf("TestParsertype() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("TestParsertype() unexpected error: %v", err)
			}

			// Check if expected strings are contained in result
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("TestParsertype() result should contain %q, got: %q", expected, result)
				}
			}
		})
	}
}

// The music/audiobook rootpath must be the album folder that setTargetPath
// uses as the complete target, never the artist (or letter) folder above it.
func TestComputeRootpathAudioIsAlbumFolder(t *testing.T) {
	lib := t.TempDir()
	album := filepath.Join(lib, "E", "Elton John", "Wonderful Crazy Night (2016)")
	multi := filepath.Join(lib, "M", "Moby", "Play (1999)")

	for _, d := range []string{album, filepath.Join(multi, "CD1"), filepath.Join(multi, "CD2")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, typ := range []uint{config.MediaTypeMusic, config.MediaTypeAudiobook} {
		cfgp := &config.MediaTypeConfig{
			IsType:  typ,
			DataMap: map[int]*config.MediaDataConfig{0: {CfgPath: &config.PathsConfig{Path: lib}}},
		}

		tests := []struct {
			path   string
			want   string
			wantOK bool
		}{
			{filepath.Join(album, "01 - Track.flac"), album, true},
			// moveandcleanup passes the album folder itself.
			{album, album, true},
			// Files of a multi-disc layout belong to the folder above CD1.
			{filepath.Join(multi, "CD2", "01 - Track.mp3"), multi, true},
			{filepath.Join(multi, "CD1"), multi, true},
			{filepath.Join(lib, "loose.mp3"), "", false},
			{"/elsewhere/a.mp3", "", false},
		}

		for _, tt := range tests {
			got, ok := computeRootpath(tt.path, cfgp)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("type %d computeRootpath(%q) = %q, %v; want %q, %v", typ, tt.path, got, ok, tt.want, tt.wantOK)
			}
		}
	}
}

// A real album folder whose name merely contains a disc-like word is not
// collapsed into the artist folder.
func TestAudioRootpathKeepsAlbumNamedLikeDisc(t *testing.T) {
	lib := t.TempDir()

	for _, name := range []string{"Trance Tapes (2021)", "Nashville On The Record - Volume 2 (2015)", "Party im Blutschloss (B0C9JKXKYX)"} {
		dir := filepath.Join(lib, "Artist", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}

		got, ok := audioRootpath(filepath.Join(dir, "01.mp3"), lib)
		if !ok || got != dir {
			t.Errorf("audioRootpath(%q) = %q, %v; want %q", name, got, ok, dir)
		}
	}
}

// The book rootpath must be the folder holding the book, which naming and
// setTargetPath use as the complete target folder once it is set.
func TestComputeRootpathBookIsBookFolder(t *testing.T) {
	lib := t.TempDir()
	book := filepath.Join(lib, "Tolkien", "The Hobbit (1937)")

	if err := os.MkdirAll(book, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgp := &config.MediaTypeConfig{
		IsType:  config.MediaTypeBook,
		DataMap: map[int]*config.MediaDataConfig{0: {CfgPath: &config.PathsConfig{Path: lib}}},
	}

	tests := []struct {
		path   string
		want   string
		wantOK bool
	}{
		{filepath.Join(book, "The Hobbit.epub"), book, true},
		// moveandcleanup passes the target folder itself.
		{book, book, true},
		{filepath.Join(lib, "Author", "Single.epub"), filepath.Join(lib, "Author"), true},
		{filepath.Join(lib, "loose.epub"), "", false},
		{"/elsewhere/a.epub", "", false},
	}

	for _, tt := range tests {
		got, ok := computeRootpath(tt.path, cfgp)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("computeRootpath(%q) = %q, %v; want %q, %v", tt.path, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestReplacedFileTakesLanguageSubtitles(t *testing.T) {
	useTestDB(t)

	lib := t.TempDir()
	old := filepath.Join(lib, "Heat (1995)", "Heat (1995) [720p].mkv")
	writeFiles(t, old,
		filepath.Join(lib, "Heat (1995)", "Heat (1995) [720p].srt"),
		filepath.Join(lib, "Heat (1995)", "Heat (1995) [720p].en.srt"),
		filepath.Join(lib, "Heat (1995)", "Heat (1995) [1080p].srt"), // belongs to the new file
	)

	s := movieOrganizer(lib)
	s.sourcepathCfg.AllowedOtherExtensions = subtitleExts
	id := uint(1)

	if err := s.moveRemoveOldMediaFile(old, &old, &id, false); err != nil {
		t.Fatal(err)
	}

	if got := listDir(t, filepath.Join(lib, "Heat (1995)")); !slices.Equal(got, []string{"Heat (1995) [1080p].srt"}) {
		t.Errorf("left = %v", got)
	}
}

func TestOrganizeBookReplacesOldFile(t *testing.T) {
	for _, moveReplaced := range []bool{false, true} {
		useTestDB(t)

		base := t.TempDir()
		dl, lib := filepath.Join(base, "dl", "Dune-EPUB-GRP"), filepath.Join(base, "lib")
		newBook := filepath.Join(dl, "dune.epub")
		oldBook := filepath.Join(lib, "Dune", "Dune.pdf")
		writeFiles(t, newBook, oldBook)

		for _, q := range []string{
			"insert into dbbooks (id, title, slug) values (1, 'Dune', 'dune')",
			"insert into books (id, dbbook_id, listname, missing) values (1, 1, 'Books', 0)",
		} {
			if err := database.ExecNErr(q); err != nil {
				t.Fatal(err)
			}
		}

		if err := database.ExecNErr("insert into book_files (location, book_id, dbbook_id) values (?, 1, 1)", oldBook); err != nil {
			t.Fatal(err)
		}

		target := &config.PathsConfig{
			Name: "lib", Path: lib, Replacelower: !moveReplaced, MoveReplaced: moveReplaced,
			MoveReplacedTargetPath: filepath.Join(base, "replaced"),
		}
		s := &Organizer{
			Cfgp: &config.MediaTypeConfig{
				IsType: config.MediaTypeBook, Naming: "{{.Dbbook.Title}}/{{.Dbbook.Title}}",
				Lists:   []config.MediaListsConfig{{Name: "Books"}},
				DataMap: map[int]*config.MediaDataConfig{0: {CfgPath: target}},
			},
			sourcepathCfg: &config.PathsConfig{
				Name: "dl", CleanupsizeMB: 10,
				AllowedBookExtensions: []string{".epub", ".pdf"}, AllowedBookExtensionsLen: 2,
			},
			targetpathCfg: target,
		}

		o := &Organizerdata{Folder: dl, MediaFile: newBook, Rootpath: filepath.Join(lib, "Dune"), Listid: 0}
		m := &database.ParseInfo{DbbookID: 1, BookID: 1}

		if err := s.Organize(context.Background(), o, m, &config.QualityConfig{Name: "ebook"}, false, false); err != nil {
			t.Fatal(err)
		}

		if got := listDir(t, filepath.Join(lib, "Dune")); !slices.Equal(got, []string{"Dune.epub"}) {
			t.Errorf("move=%t: library = %v, want only the new book", moveReplaced, got)
		}

		if moveReplaced != exists(filepath.Join(base, "replaced", "Dune", "Dune.pdf")) {
			t.Errorf("move=%t: replaced folder state wrong", moveReplaced)
		}

		if rows(t, "select count() from book_files") != 1 || rows(t, "select count() from book_files where location = ?", oldBook) != 0 {
			t.Errorf("move=%t: book_files not updated", moveReplaced)
		}
	}
}

func TestBookLowerQuality(t *testing.T) {
	cfgp := &config.MediaTypeConfig{IsType: config.MediaTypeBook}
	qual := &config.QualityConfig{Name: "ebook"}

	for _, tc := range []struct {
		name, oldExt, newFormat string
		lower                   bool
	}{
		{"no library file", "", "pdf", false},
		{"epub over pdf", ".pdf", "epub", false},
		{"pdf over epub", ".epub", "pdf", true},
		{"same format", ".epub", "epub", true},
		{"unknown library format", ".xyz", "pdf", false},
	} {
		useTestDB(t)

		for _, q := range []string{
			"insert into dbbooks (id, title, slug) values (1, 'Dune', 'dune')",
			"insert into books (id, dbbook_id, listname, missing) values (1, 1, 'Books', 0)",
		} {
			if err := database.ExecNErr(q); err != nil {
				t.Fatal(err)
			}
		}

		if tc.oldExt != "" {
			if err := database.ExecNErr("insert into book_files (location, extension, book_id, dbbook_id) values ('/lib/Dune/Dune'||?, ?, 1, 1)",
				tc.oldExt, tc.oldExt); err != nil {
				t.Fatal(err)
			}
		}

		m := &database.ParseInfo{BookID: 1, DbbookID: 1, Quality: tc.newFormat}
		if _, lower := bookLowerQuality(m, cfgp, qual); lower != tc.lower {
			t.Errorf("%s: lower quality %t, want %t", tc.name, lower, tc.lower)
		}
	}
}

func TestOrganizeBookKeepsLowerQualityFile(t *testing.T) {
	useTestDB(t)

	base := t.TempDir()
	dl, lib := filepath.Join(base, "dl", "Dune-PDF-GRP"), filepath.Join(base, "lib")
	newBook := filepath.Join(dl, "dune.pdf")
	oldBook := filepath.Join(lib, "Dune", "Dune.epub")
	writeFiles(t, newBook, oldBook)

	for _, q := range []string{
		"insert into dbbooks (id, title, slug) values (1, 'Dune', 'dune')",
		"insert into books (id, dbbook_id, listname, missing) values (1, 1, 'Books', 0)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	if err := database.ExecNErr("insert into book_files (location, extension, book_id, dbbook_id) values (?, '.epub', 1, 1)", oldBook); err != nil {
		t.Fatal(err)
	}

	target := &config.PathsConfig{Name: "lib", Path: lib, Replacelower: true}
	s := &Organizer{
		Cfgp: &config.MediaTypeConfig{
			IsType: config.MediaTypeBook, Naming: "{{.Dbbook.Title}}/{{.Dbbook.Title}}",
			Lists:   []config.MediaListsConfig{{Name: "Books"}},
			DataMap: map[int]*config.MediaDataConfig{0: {CfgPath: target}},
		},
		sourcepathCfg: &config.PathsConfig{
			Name: "dl", CleanupsizeMB: 10,
			AllowedBookExtensions: []string{".epub", ".pdf"}, AllowedBookExtensionsLen: 2,
		},
		targetpathCfg: target,
	}

	o := &Organizerdata{Folder: dl, MediaFile: newBook, Rootpath: filepath.Join(lib, "Dune"), Listid: 0}
	m := &database.ParseInfo{DbbookID: 1, BookID: 1, Quality: "pdf"}

	err := s.Organize(context.Background(), o, m, &config.QualityConfig{Name: "ebook"}, false, false)
	if !errors.Is(err, errLowerQuality) {
		t.Fatalf("organize error %v, want lower quality", err)
	}

	if !exists(newBook) || !exists(oldBook) || rows(t, "select count() from book_files") != 1 {
		t.Error("a lower-quality book must leave the import file and the library untouched")
	}
}

func TestComputeRootpathBoundaryAndLongestPath(t *testing.T) {
	cfgp := &config.MediaTypeConfig{DataMap: map[int]*config.MediaDataConfig{
		0: {CfgPath: &config.PathsConfig{Path: "/data"}},
		1: {CfgPath: &config.PathsConfig{Path: "/data/EN_Series"}},
		2: {CfgPath: &config.PathsConfig{Path: "/other/"}},
	}}

	tests := []struct {
		file   string
		want   string
		wantOK bool
	}{
		{"/data/EN_Series/Show/Season 1/e1.mkv", "/data/EN_Series/Show", true},
		// Sibling folder sharing the prefix belongs to /data, not /data/EN_Series.
		{"/data/EN_Series_old/Show/e1.mkv", "/data/EN_Series_old", true},
		{"/other/Movie (2020)/m.mkv", "/other/Movie (2020)", true},
		{"/elsewhere/data/x.mkv", "", false},
	}

	for i := 0; i < 20; i++ { // map order is random; repeat
		for _, tt := range tests {
			got, ok := computeRootpath(tt.file, cfgp)
			if got != filepath.FromSlash(tt.want) || ok != tt.wantOK {
				t.Fatalf("computeRootpath(%q) = %q, %v; want %q, %v", tt.file, got, ok, tt.want, tt.wantOK)
			}
		}
	}
}

func TestRemoveOtherFilesOnlySwapsExtension(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Movie.mkv")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	video := filepath.Join(dir, "Movie.mkv")
	sub := filepath.Join(dir, "Movie.srt")

	for _, f := range []string{video, sub} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := &Organizer{sourcepathCfg: &config.PathsConfig{AllowedOtherExtensions: []string{".srt"}}}
	s.removeotherfiles(video)

	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Errorf("subtitle next to the video was not removed (err=%v)", err)
	}
}

func TestCleanUpFolderKeepsFolderAtThreshold(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.bin"), make([]byte, 1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Organizer{sourcepathCfg: &config.PathsConfig{CleanupsizeMB: 1}}
	if err := s.cleanUpFolder(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("folder holding exactly the threshold was removed: %v", err)
	}
}

// TestCleanUpAlbumFolderKeepsAudio: cleanupsize 0 removes any folder, so an
// album organize that left unmatched tracks behind must keep the folder.
func TestCleanUpAlbumFolderKeepsAudio(t *testing.T) {
	s := &Organizer{sourcepathCfg: &config.PathsConfig{}}

	withAudio := t.TempDir()
	if err := os.MkdirAll(filepath.Join(withAudio, "CD2"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{"cover.jpg", filepath.Join("CD2", "09 - Extra.FLAC")} {
		if err := os.WriteFile(filepath.Join(withAudio, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s.cleanUpAlbumFolder(withAudio)

	if _, err := os.Stat(withAudio); err != nil {
		t.Errorf("folder with a leftover track was removed: %v", err)
	}

	leftovers := t.TempDir()
	if err := os.WriteFile(filepath.Join(leftovers, "album.nfo"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s.cleanUpAlbumFolder(leftovers)

	if _, err := os.Stat(leftovers); !os.IsNotExist(err) {
		t.Errorf("folder without audio was not cleaned up (err=%v)", err)
	}
}

func TestIsForeignUnpackPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/downloads/Movie.2020/Movie.2020_unpack/movie.mkv", false},
		{"/downloads/Movie.2020/movie.mkv", false},
		{"/downloads/_UNPACK_Movie.2020/movie.mkv", true},
		{"/downloads/Movie.2020/_unpack_x/movie.mkv", true},
		{`C:\downloads\_UNPACK_Movie\movie.mkv`, true},
	}

	for _, tt := range tests {
		if got := isForeignUnpackPath(tt.path); got != tt.want {
			t.Errorf("isForeignUnpackPath(%q) = %v; want %v", tt.path, got, tt.want)
		}
	}
}

// TestPresortTargetPath: with presort the folder name must appear exactly once
// in the target for every media type.
func TestPresortTargetPath(t *testing.T) {
	presort := filepath.FromSlash("/presort")

	tests := []struct {
		name     string
		isType   uint
		rootpath string
		usePre   bool
		want     string
	}{
		{"movie presort", config.MediaTypeMovie, "", true, filepath.Join(presort, "Movie (2020)")},
		{"series presort", config.MediaTypeSeries, "", true, filepath.Join(presort, "Movie (2020)")},
		{"music presort", config.MediaTypeMusic, "", true, filepath.Join(presort, "Movie (2020)")},
		{"movie no presort", config.MediaTypeMovie, "", false, filepath.Join("/lib", "Movie (2020)")},
		{"movie rootpath", config.MediaTypeMovie, "/lib/Other", false, filepath.Join("/lib/Other", "Movie (2020)")},
		{"music rootpath", config.MediaTypeMusic, "/lib/Artist/Album", false, "/lib/Artist/Album"},
	}

	for _, tt := range tests {
		s := &Organizer{
			Cfgp: &config.MediaTypeConfig{IsType: tt.isType},
			targetpathCfg: &config.PathsConfig{
				Path:              "/lib",
				Usepresort:        tt.usePre,
				PresortFolderPath: presort,
			},
		}
		o := &Organizerdata{Rootpath: tt.rootpath, Foldername: "Movie (2020)"}

		s.applyPresort(o)
		s.setTargetPath(o)

		if o.TargetPath != tt.want {
			t.Errorf("%s: TargetPath = %q; want %q", tt.name, o.TargetPath, tt.want)
		}
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

func writeFiles(t *testing.T, paths ...string) {
	t.Helper()

	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func rows(t *testing.T, query string, args ...any) int {
	t.Helper()

	return database.Getdatarow[int](false, query, args...)
}

func movieOrganizer(lib string) *Organizer {
	return &Organizer{
		Cfgp:          &config.MediaTypeConfig{IsType: config.MediaTypeMovie},
		sourcepathCfg: &config.PathsConfig{Name: "dl", AllowedOtherExtensions: []string{".srt", ".nfo"}},
		targetpathCfg: &config.PathsConfig{Name: "lib", Path: lib, MoveReplacedTargetPath: filepath.Join(lib, "_replaced")},
	}
}

func TestMoveRemoveOldMediaFileDeletes(t *testing.T) {
	useTestDB(t)

	lib := t.TempDir()
	// A folder named like the file must not confuse the companion lookup.
	old := filepath.Join(lib, "Heat.mkv", "Heat.720p.mkv")
	sub := filepath.Join(lib, "Heat.mkv", "Heat.720p.srt")
	nfo := filepath.Join(lib, "Heat.mkv", "Heat.720p.nfo")
	writeFiles(t, old, sub, nfo)

	if err := database.ExecNErr("insert into movie_files (location, movie_id) values (?, 1), ('/other.mkv', 1)", old); err != nil {
		t.Fatal(err)
	}

	s := movieOrganizer(lib)
	id := uint(1)

	if err := s.moveRemoveOldMediaFile(old, &old, &id, false); err != nil {
		t.Fatal(err)
	}

	if exists(old) || exists(sub) || exists(nfo) {
		t.Errorf("left behind: video %t, srt %t, nfo %t", exists(old), exists(sub), exists(nfo))
	}

	if rows(t, "select count() from movie_files") != 1 {
		t.Error("the replaced file's row was not dropped (or the wrong one was)")
	}
}

func TestMoveRemoveOldMediaFileMovesToReplaced(t *testing.T) {
	useTestDB(t)

	lib := t.TempDir()
	old := filepath.Join(lib, "Heat (1995)", "Heat.720p.mkv")
	sub := filepath.Join(lib, "Heat (1995)", "Heat.720p.srt")
	writeFiles(t, old, sub)

	s := movieOrganizer(lib)
	id := uint(1)

	if err := s.moveRemoveOldMediaFile(old, &old, &id, true); err != nil {
		t.Fatal(err)
	}

	replaced := filepath.Join(lib, "_replaced", "Heat (1995)")
	if exists(old) || !exists(filepath.Join(replaced, "Heat.720p.mkv")) || !exists(filepath.Join(replaced, "Heat.720p.srt")) {
		t.Error("old file and companion not moved into the replaced folder")
	}
}

func TestMoveRemoveOldMediaFileMissingDropsRow(t *testing.T) {
	useTestDB(t)

	gone := filepath.Join(t.TempDir(), "gone.mkv")
	if err := database.ExecNErr("insert into movie_files (location, movie_id) values (?, 1)", gone); err != nil {
		t.Fatal(err)
	}

	s := movieOrganizer(t.TempDir())
	id := uint(1)

	if err := s.moveRemoveOldMediaFile(gone, &gone, &id, false); err != nil {
		t.Fatal(err)
	}

	if rows(t, "select count() from movie_files") != 0 {
		t.Error("stale row of a missing file kept")
	}

	if err := s.moveRemoveOldMediaFile("", nil, &id, false); err != nil {
		t.Errorf("empty path: %v", err)
	}
}

func TestMoveremoveoldfilesSkipsNewFile(t *testing.T) {
	useTestDB(t)

	lib := t.TempDir()
	target := filepath.Join(lib, "Heat (1995)")
	newFile := filepath.Join(target, "Heat (1995) [1080p].mkv")
	oldFile := filepath.Join(target, "Heat (1995) [720p].mkv")
	writeFiles(t, newFile, oldFile)

	s := movieOrganizer(lib)
	o := &Organizerdata{TargetPath: target, Filename: "Heat (1995) [1080p].mkv"}
	id := uint(1)

	// A failing entry (a directory) must not stop the batch.
	if err := s.moveremoveoldfiles(o, true, &id, false, []string{newFile, lib, oldFile}); err != nil {
		t.Fatal(err)
	}

	if !exists(newFile) || exists(oldFile) {
		t.Errorf("new kept %t (want true), old kept %t (want false)", exists(newFile), exists(oldFile))
	}
}

func TestStashAndRestoreReplacedFile(t *testing.T) {
	lib := t.TempDir()
	old := filepath.Join(lib, "Heat (1995)", "Heat (1995).mkv")
	writeFiles(t, old)

	s := movieOrganizer(lib)

	stashed, err := s.stashReplacedFile(old)
	if err != nil {
		t.Fatal(err)
	}

	if exists(old) || !exists(stashed) || filepath.Dir(stashed) != filepath.Join(lib, "_replaced", "Heat (1995)") {
		t.Fatalf("stash: old %t, stashed %q", exists(old), stashed)
	}

	s.restoreStashedFile(stashed, old)

	if !exists(old) || exists(stashed) {
		t.Errorf("restore: old %t, stashed %t", exists(old), exists(stashed))
	}

	// An occupied original path keeps the stashed copy instead of overwriting.
	stashed, _ = s.stashReplacedFile(old)
	writeFiles(t, old)
	s.restoreStashedFile(stashed, old)

	if !exists(stashed) {
		t.Error("stashed file overwrote an occupied original path")
	}
}

func TestFileCleanupRemovesDownloadFolder(t *testing.T) {
	dl := filepath.Join(t.TempDir(), "Heat.1995.1080p-GRP")
	video := filepath.Join(dl, "heat.mkv")
	sub := filepath.Join(dl, "Subs", "heat.srt")
	writeFiles(t, video, sub)

	s := movieOrganizer(t.TempDir())
	s.sourcepathCfg.CleanupsizeMB = 50
	s.sourcepathCfg.AllowedOtherExtensions = []string{".srt"}
	s.sourcepathCfg.AllowedOtherExtensionsLen = 1

	if err := s.fileCleanup(dl, video, dl); err != nil {
		t.Fatal(err)
	}

	if exists(dl) {
		t.Error("download folder below the cleanup size was not removed")
	}

	// Without a video file only the folder size rule applies.
	other := filepath.Join(t.TempDir(), "rel")
	writeFiles(t, filepath.Join(other, "x.nfo"))

	if err := s.fileCleanup(other, "", other); err != nil || exists(other) {
		t.Errorf("folder-only cleanup: %v, exists %t", err, exists(other))
	}
}

func TestWalkcleanupRemovesOtherFiles(t *testing.T) {
	dl := t.TempDir()
	writeFiles(t, filepath.Join(dl, "heat.srt"), filepath.Join(dl, "sample.mkv"), filepath.Join(dl, "blocked", "x.srt"))

	s := movieOrganizer(t.TempDir())
	s.sourcepathCfg.AllowedOtherExtensions = []string{".srt"}
	s.sourcepathCfg.AllowedOtherExtensionsLen = 1
	s.sourcepathCfg.Blocked = []string{"blocked"}
	s.sourcepathCfg.BlockedLen = 1

	if err := s.walkcleanup(dl); err != nil {
		t.Fatal(err)
	}

	if exists(filepath.Join(dl, "heat.srt")) || !exists(filepath.Join(dl, "sample.mkv")) || !exists(filepath.Join(dl, "blocked", "x.srt")) {
		t.Error("walkcleanup removed the wrong files")
	}
}

func TestFolderHasRecentFiles(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.mkv")
	writeFiles(t, f)

	if !folderHasRecentFiles(dir) {
		t.Error("fresh file not detected")
	}

	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(f, old, old); err != nil {
		t.Fatal(err)
	}

	if folderHasRecentFiles(dir) {
		t.Error("hour-old file counted as recent")
	}
}

// TestOrganizeBookEndToEnd runs Organize for a new book: the file is renamed
// into the library, the download folder is cleaned up, and the database
// records the file, the rootpath and that the book is no longer missing.
func TestOrganizeBookEndToEnd(t *testing.T) {
	useTestDB(t)

	dl := filepath.Join(t.TempDir(), "Dune-EPUB-GRP")
	lib := t.TempDir()
	newBook := filepath.Join(dl, "dune.epub")
	writeFiles(t, newBook, filepath.Join(dl, "dune.nfo"))

	for _, q := range []string{
		"insert into dbbooks (id, title, slug) values (1, 'Dune', 'dune')",
		"insert into books (id, dbbook_id, listname, quality_profile, missing) values (1, 1, 'Books', 'ebook', 1)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	target := &config.PathsConfig{Name: "lib", Path: lib, Replacelower: true}
	s := &Organizer{
		Cfgp: &config.MediaTypeConfig{
			IsType: config.MediaTypeBook, Naming: "{{.Dbbook.Title}}/{{.Dbbook.Title}}",
			Lists:   []config.MediaListsConfig{{Name: "Books"}},
			DataMap: map[int]*config.MediaDataConfig{0: {CfgPath: target}},
		},
		sourcepathCfg: &config.PathsConfig{
			Name: "dl", CleanupsizeMB: 10,
			AllowedBookExtensions: []string{".epub"}, AllowedBookExtensionsLen: 1,
			AllowedOtherExtensions: []string{".nfo"}, AllowedOtherExtensionsLen: 1,
		},
		targetpathCfg: target,
	}

	// A new item has no library rootpath yet.
	o := &Organizerdata{Folder: dl, MediaFile: newBook, Listid: 0}
	m := &database.ParseInfo{DbbookID: 1, BookID: 1}

	if err := s.Organize(context.Background(), o, m, &config.QualityConfig{Name: "ebook"}, false, false); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(lib, "Dune", "Dune.epub")
	if !exists(want) || exists(newBook) {
		t.Fatalf("book not moved to %s", want)
	}

	if exists(dl) {
		t.Error("download folder not cleaned up")
	}

	if rows(t, "select count() from book_files where location = ?", want) != 1 ||
		rows(t, "select missing from books where id = 1") != 0 {
		t.Error("new file not recorded or book still missing")
	}

	if rows(t, "select count() from books where id = 1 and rootpath = ?", filepath.Join(lib, "Dune")) != 1 {
		t.Errorf("rootpath not set: %q", database.Getdatarow[string](false, "select rootpath from books where id = 1"))
	}
}

func TestOrganizeRejectsMissingIDs(t *testing.T) {
	for _, mt := range []uint{config.MediaTypeMovie, config.MediaTypeSeries, config.MediaTypeBook, config.MediaTypeAudiobook, config.MediaTypeMusic} {
		s := &Organizer{Cfgp: &config.MediaTypeConfig{IsType: mt}, targetpathCfg: &config.PathsConfig{}, sourcepathCfg: &config.PathsConfig{}}
		if err := s.Organize(context.Background(), &Organizerdata{}, &database.ParseInfo{}, &config.QualityConfig{}, false, false); err == nil {
			t.Errorf("type %d: organize without a database id succeeded", mt)
		}
	}
}

// TestFileCleanupKeepsLibraryCompanions rejects a download for a movie that is
// already in the library: the library copy's subtitles and NFO must survive.
func TestFileCleanupKeepsLibraryCompanions(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "Heat (1995)")
	dl := filepath.Join(t.TempDir(), "Heat.1995.720p-GRP")
	libFiles := []string{filepath.Join(lib, "Heat (1995).mkv"), filepath.Join(lib, "Heat (1995).srt"), filepath.Join(lib, "Heat (1995).nfo")}
	writeFiles(t, append(libFiles, filepath.Join(dl, "heat.mkv"), filepath.Join(dl, "heat.srt"), filepath.Join(dl, "big.bin"))...)

	if err := os.WriteFile(filepath.Join(dl, "big.bin"), make([]byte, 2<<20), 0o644); err != nil {
		t.Fatal(err)
	}

	s := movieOrganizer(t.TempDir())
	s.sourcepathCfg.CleanupsizeMB = 1 // download folder stays because of big.bin
	s.sourcepathCfg.AllowedOtherExtensions = []string{".srt", ".nfo"}
	s.sourcepathCfg.AllowedOtherExtensionsLen = 2

	if err := s.fileCleanup(dl, filepath.Join(dl, "heat.mkv"), lib); err != nil {
		t.Fatal(err)
	}

	for _, f := range libFiles {
		if !exists(f) {
			t.Errorf("library file deleted: %s", f)
		}
	}

	if exists(filepath.Join(dl, "heat.mkv")) || exists(filepath.Join(dl, "heat.srt")) {
		t.Error("rejected download's video or subtitle left behind")
	}
}

func TestRuntimeMismatch(t *testing.T) {
	s := &Organizer{
		Cfgp:          &config.MediaTypeConfig{IsType: config.MediaTypeMovie},
		targetpathCfg: &config.PathsConfig{MaxRuntimeDifference: 15},
	}

	m := &database.ParseInfo{Runtime: 208 * 60}

	if wanted, found, wrong := s.runtimeMismatch(m, 149, true); !wrong || wanted != 149 || found != 208 {
		t.Errorf("208 vs 149 min: wanted %d found %d wrong %v", wanted, found, wrong)
	}

	if _, _, wrong := s.runtimeMismatch(m, 200, true); wrong {
		t.Error("8 min off with a 15 min tolerance was rejected")
	}

	if _, _, wrong := s.runtimeMismatch(m, 149, false); wrong {
		t.Error("rejected with check_runtime off")
	}

	s.targetpathCfg.MaxRuntimeDifference = 0
	if _, _, wrong := s.runtimeMismatch(m, 207, true); !wrong {
		t.Error("without a tolerance any difference must be rejected")
	}
}

func TestFolderTemplateSeparators(t *testing.T) {
	type author struct{ Name string }

	data := struct {
		Author author
		Series string
		Title  string
	}{author{"AC/DC"}, `Back\Black`, "Live / 1991"}

	tmpl := "{{.Author.Name}}/{{if .Series}}{{.Series}}/{{end}}{{.Title}}"

	_, got, err := logger.ParseStringTemplate(protectTemplateSeparators(tmpl), &data)
	if err != nil {
		t.Fatal(err)
	}

	if got = folderSepReplacer.Replace(got); got != "AC-DC/Back-Black/Live - 1991" {
		t.Errorf("got %q, want value separators as - and template ones as /", got)
	}

	if protectTemplateSeparators("no separators") != "no separators" {
		t.Error("template without separators changed")
	}
}

func TestCleanFolderPath(t *testing.T) {
	cases := map[string]string{
		"F/Frank Kodiak/Kantzius/1 - /Amissa - Die Verlorenen (3965191691)": "F/Frank Kodiak/Kantzius/1 -/Amissa - Die Verlorenen (3965191691)",
		"A/Arthur Conan Doyle/Der schwarze Peter / Das Glas (B07D3P24KJ)":   "A/Arthur Conan Doyle/Der schwarze Peter/Das Glas (B07D3P24KJ)",
		"C/Christiane F./Wir Kinder vom Bahnhof Zoo (B004SG7JWY)":           "C/Christiane F/Wir Kinder vom Bahnhof Zoo (B004SG7JWY)",
		`H/H.G. Francis/Gauner mit der "Goldenen Hand" (B0B1QL1P1H)`:        "H/H.G. Francis/Gauner mit der 'Goldenen Hand' (B0B1QL1P1H)",
		"Series. ./Season 1": "Series/Season 1",
		"a//b/ /c":           "a/b/c",
		"":                   "",
		"Movie (2020)":       "Movie (2020)",
	}

	for in, want := range cases {
		if got := cleanFolderPath(in); got != want {
			t.Errorf("cleanFolderPath(%q) = %q, want %q", in, got, want)
		}
	}

	if got := cleanPathName(` Track 1 - "Intro". `); got != "Track 1 - 'Intro'" {
		t.Errorf("cleanPathName = %q", got)
	}
}

func TestValidateAudiobookLanguage(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}

	// book writes a 3-chapter audiobook folder, tagged with language when set.
	book := func(t *testing.T, language string) *parser_v2.AlbumInfo {
		t.Helper()

		album := &parser_v2.AlbumInfo{SourceFolder: t.TempDir()}
		for i := range 3 {
			f := filepath.Join(album.SourceFolder, strconv.Itoa(i+1)+".m4a")

			args := []string{"-v", "error", "-f", "lavfi", "-i", "sine=duration=2", "-c:a", "aac"}
			if language != "" {
				args = append(args, "-metadata:s:a:0", "language="+language)
			}

			if out, err := exec.Command("ffmpeg", append(args, f)...).CombinedOutput(); err != nil {
				t.Fatalf("creating test file: %v: %s", err, out)
			}

			album.Tracks = append(album.Tracks, parser_v2.TrackInfo{Filepath: f})
		}

		return album
	}

	org := func(deleteWrong bool, allowed ...string) *Organizer {
		return &Organizer{
			deletewronglanguage: deleteWrong,
			targetpathCfg:       &config.PathsConfig{AllowedLanguages: allowed, AllowedLanguagesLen: len(allowed)},
		}
	}

	remaining := func(album *parser_v2.AlbumInfo) int {
		n := 0
		for i := range album.Tracks {
			if _, err := os.Stat(album.Tracks[i].Filepath); err == nil {
				n++
			}
		}

		return n
	}

	tests := []struct {
		name     string
		s        *Organizer
		language string
		wrong    bool
	}{
		{"allowed", org(true, "German", "ger"), "ger", false},
		{"wrong language deleted", org(true, "eng"), "ger", true},
		{"delete_wrong_language off", org(false, "eng"), "ger", false},
		{"no allowed_languages", org(true), "ger", false},
		// Untagged and nothing detected (no whisper_url): not checked.
		{"unknown passes", org(true, "eng"), "", false},
	}

	for _, tt := range tests {
		album := book(t, tt.language)

		err := tt.s.validateAudiobookLanguage(context.Background(), album)
		if got := errors.Is(err, errWrongLanguage); got != tt.wrong {
			t.Errorf("%s: err %v, want wrong language %v", tt.name, err, tt.wrong)
		}

		want := 3
		if tt.wrong {
			want = 0
		}

		if n := remaining(album); n != want {
			t.Errorf("%s: %d of 3 files left, want %d", tt.name, n, want)
		}
	}
}
