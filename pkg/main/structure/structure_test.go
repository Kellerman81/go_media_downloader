package structure

import (
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
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
