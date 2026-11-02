package scanner_test

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

func testPathCfg() *config.PathsConfig {
	return &config.PathsConfig{
		AllowedVideoExtensions: []string{".mkv"}, AllowedVideoExtensionsLen: 1,
		AllowedVideoExtensionsNoRename: []string{".iso"}, AllowedVideoExtensionsNoRenameLen: 1,
		AllowedOtherExtensions: []string{".srt"}, AllowedOtherExtensionsLen: 1,
	}
}

func TestCheckExtensions(t *testing.T) {
	pc := testPathCfg()

	tests := []struct {
		video, other bool
		ext          string
		ok, norename bool
	}{
		{true, false, ".mkv", true, false},
		{true, false, ".iso", true, true},
		{true, false, ".srt", false, false},
		{true, true, ".srt", true, false}, // falls back to other extensions
		{false, true, ".mkv", false, false},
		{false, false, ".mkv", false, false},
	}

	for _, tt := range tests {
		ok, nr := scanner.CheckExtensions(tt.video, tt.other, pc, tt.ext)
		if ok != tt.ok || nr != tt.norename {
			t.Errorf("scanner.CheckExtensions(%t, %t, %q) = %t, %t", tt.video, tt.other, tt.ext, ok, nr)
		}
	}

	if ok, _ := scanner.CheckExtensionsType(config.MediaTypeMovie, false, pc, ".mkv"); !ok {
		t.Error("scanner.CheckExtensionsType(movie, .mkv) rejected")
	}

	if ok, _ := scanner.CheckExtensionsType(config.MediaTypeMovie, true, pc, ".mkv"); ok {
		t.Error("scanner.CheckExtensionsType(movie, other, .mkv) accepted")
	}
}

func TestPlannedMovePath(t *testing.T) {
	pc := testPathCfg()
	target := filepath.Join("/media", "Movies", "Heat (1995)")
	opts := scanner.MoveFileOptions{MediaType: config.MediaTypeMovie}

	tests := []struct {
		file, newname string
		opts          scanner.MoveFileOptions
		want          string
	}{
		{"/dl/heat.1995.mkv", "Heat (1995) [1080p]", opts, "Heat (1995) [1080p].mkv"},
		{"/dl/heat.1995.mkv", "Heat (1995).MKV", opts, "Heat (1995).MKV"},
		{"/dl/heat.1995.iso", "Heat (1995)", opts, "heat.1995.iso"}, // no-rename extension
		{"/dl/heat.1995.mkv", "", opts, "heat.1995.mkv"},
		{"/dl/heat.1995.mkv", "Heat", scanner.MoveFileOptions{UseNil: true}, "heat.1995.mkv"},
		{"/dl/heat.srt", "Heat", scanner.MoveFileOptions{UseOther: true, MediaType: config.MediaTypeMovie}, "Heat.srt"},
	}

	for _, tt := range tests {
		if got := scanner.PlannedMovePath(tt.file, pc, target, tt.newname, tt.opts); got != filepath.Join(target, tt.want) {
			t.Errorf("scanner.PlannedMovePath(%q, %q) = %q, want %q", tt.file, tt.newname, got, filepath.Join(target, tt.want))
		}
	}

	// Without a path config every extension is allowed and nothing renamed.
	if got := scanner.PlannedMovePath("/dl/x.mkv", nil, target, "New", opts); got != filepath.Join(target, "x.mkv") {
		t.Errorf("nil path config = %q", got)
	}
}

func TestPlannedMovePathCapsLongNames(t *testing.T) {
	title := strings.Repeat("Improvisation (Don't Worry Kyoko) ", 12) + "Maxwell's Silver Hammer"
	got := filepath.Base(scanner.PlannedMovePath("/dl/x.mp3", nil, "/m", "", scanner.MoveFileOptions{UseNil: true}))

	if got != "x.mp3" {
		t.Errorf("short name changed: %q", got)
	}

	pc := &config.PathsConfig{AllowedVideoExtensions: []string{".mkv"}, AllowedVideoExtensionsLen: 1}
	long := filepath.Base(scanner.PlannedMovePath("/dl/x.mkv", pc, "/m", "06-06 - "+title, scanner.MoveFileOptions{MediaType: config.MediaTypeMovie}))

	if len(long) > 200 || !strings.HasSuffix(long, ".mkv") || !strings.HasPrefix(long, "06-06 - Improvisation") {
		t.Errorf("long name = %q (%d bytes)", long, len(long))
	}

	// Multi-byte characters are never cut in half.
	umlauts := filepath.Base(scanner.PlannedMovePath("/dl/x.mkv", pc, "/m", strings.Repeat("ä", 150), scanner.MoveFileOptions{MediaType: config.MediaTypeMovie}))
	if !utf8.ValidString(umlauts) || len(umlauts) > 200 {
		t.Errorf("umlaut name = %d bytes, valid %t", len(umlauts), utf8.ValidString(umlauts))
	}
}
