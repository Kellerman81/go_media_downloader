package structure

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
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
