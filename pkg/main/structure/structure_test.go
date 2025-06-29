package structure

import (
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
			expected: "124541",
		},
		{
			name:     "Mixed content",
			input:    "a1b2c3d4",
			remove:   '2',
			expected: "a1b3c3d4",
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

func TestUpdateRootpath(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		objtype  string
		objid    uint
		cfgp     *config.MediaTypeConfig
		expected string
	}{
		{
			name:    "No matching path in config",
			file:    "/some/random/path/file.txt",
			objtype: "media",
			objid:   1,
			cfgp:    &config.MediaTypeConfig{},
		},
		{
			name:     "Single level directory match",
			file:     "/media/photos/vacation.jpg",
			objtype:  "photos",
			objid:    2,
			cfgp:     config.SettingsMedia["movie_EN"],
			expected: "/media/photos",
		},
		{
			name:     "Nested directory match",
			file:     "/content/videos/2023/summer/video.mp4",
			objtype:  "videos",
			objid:    3,
			cfgp:     config.SettingsMedia["movie_EN"],
			expected: "/content/videos",
		},
		{
			name:     "Windows style paths",
			file:     "C:\\Users\\Media\\Pictures\\photo.jpg",
			objtype:  "pictures",
			objid:    4,
			cfgp:     config.SettingsMedia["movie_EN"],
			expected: "C:\\Users\\Media\\Pictures",
		},
		{
			name:     "Multiple config entries",
			file:     "/data/music/album/song.mp3",
			objtype:  "music",
			objid:    5,
			cfgp:     config.SettingsMedia["movie_EN"],
			expected: "/data/music",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			var capturedID uint

			// database.ExecMock = func(query string, args ...interface{}) {
			// 	capturedPath = *args[0].(*string)
			// 	capturedID = *args[1].(*uint)
			// }
			// defer func() { database.ExecMock = nil }()

			UpdateRootpath(tt.file, tt.objtype, &tt.objid, tt.cfgp)

			if tt.expected != "" {
				if capturedPath != tt.expected {
					t.Errorf("Expected rootpath %s, got %s", tt.expected, capturedPath)
				}
				if capturedID != tt.objid {
					t.Errorf("Expected objid %d, got %d", tt.objid, capturedID)
				}
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
