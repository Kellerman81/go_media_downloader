package structure

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// subtitleExts is the production allowed_other_extensions setting.
var subtitleExts = []string{".idx", ".sub", ".srt"}

func listDir(t *testing.T, dir string) []string {
	t.Helper()

	var out []string

	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, rel)
		}

		return nil
	})

	sort.Strings(out)

	return out
}

func companionOrganizer(isType uint, dlRoot, lib string) *Organizer {
	return &Organizer{
		Cfgp: &config.MediaTypeConfig{IsType: isType},
		sourcepathCfg: &config.PathsConfig{
			Name: "dl", Path: dlRoot,
			AllowedOtherExtensions: subtitleExts, AllowedOtherExtensionsLen: len(subtitleExts),
		},
		targetpathCfg: &config.PathsConfig{Name: "lib", Path: lib},
	}
}

func TestMoveCompanionFilesMovie(t *testing.T) {
	dlRoot, lib := t.TempDir(), t.TempDir()
	rel := filepath.Join(dlRoot, "Heat.1995.1080p-GRP")
	target := filepath.Join(lib, "Heat (1995)")
	writeFiles(t,
		filepath.Join(rel, "heat.1995.1080p.mkv"), // already moved in reality; ignored here
		filepath.Join(rel, "heat.1995.1080p.srt"),
		filepath.Join(rel, "heat.1995.1080p.en.srt"),
		filepath.Join(rel, "Subs", "VTS_01_0.idx"),
		filepath.Join(rel, "Subs", "VTS_01_0.sub"),
		filepath.Join(rel, "Subs", "German.srt"),
		filepath.Join(rel, "heat.nfo"),
	)

	s := companionOrganizer(config.MediaTypeMovie, dlRoot, lib)
	o := &Organizerdata{Folder: rel, MediaFile: filepath.Join(rel, "heat.1995.1080p.mkv"), TargetPath: target, Filename: "Heat (1995)"}

	s.moveCompanionFiles(o)

	// Exact and language-suffixed subtitles keep their relation to the video;
	// the lone idx/sub pair takes the plain name; a third .srt keeps its own
	// name so nothing is overwritten. Files outside the setting stay.
	want := []string{"Heat (1995).German.srt", "Heat (1995).en.srt", "Heat (1995).idx", "Heat (1995).srt", "Heat (1995).sub"}
	if got := listDir(t, target); !slices.Equal(got, want) {
		t.Errorf("target = %v, want %v", got, want)
	}

	if !exists(filepath.Join(rel, "heat.nfo")) {
		t.Error("file outside allowed_other_extensions was moved")
	}

	if len(o.RenamedFiles) != 5 {
		t.Errorf("rename log has %d entries", len(o.RenamedFiles))
	}
}

func TestMoveCompanionFilesSeriesOnlyTakesTheEpisode(t *testing.T) {
	dlRoot, lib := t.TempDir(), t.TempDir()
	pack := filepath.Join(dlRoot, "Dark.S01.1080p-GRP")
	writeFiles(t,
		filepath.Join(pack, "Dark.S01E01.1080p.srt"),
		filepath.Join(pack, "Dark.S01E01.1080p.de.srt"),
		filepath.Join(pack, "Dark.S01E02.1080p.srt"),
		filepath.Join(pack, "Subs", "English.srt"),
	)

	s := companionOrganizer(config.MediaTypeSeries, dlRoot, lib)
	o := &Organizerdata{Folder: pack, MediaFile: filepath.Join(pack, "Dark.S01E01.1080p.mkv"), TargetPath: lib, Filename: "Dark - S01E01"}

	s.moveCompanionFiles(o)

	if got := listDir(t, lib); !slices.Equal(got, []string{"Dark - S01E01.de.srt", "Dark - S01E01.srt"}) {
		t.Errorf("target = %v", got)
	}

	if !exists(filepath.Join(pack, "Dark.S01E02.1080p.srt")) || !exists(filepath.Join(pack, "Subs", "English.srt")) {
		t.Error("another episode's or an unrelated subtitle was taken")
	}
}

func TestMoveCompanionFilesInDownloadRoot(t *testing.T) {
	dlRoot, lib := t.TempDir(), t.TempDir()
	writeFiles(t, filepath.Join(dlRoot, "heat.srt"), filepath.Join(dlRoot, "other.movie.srt"))

	s := companionOrganizer(config.MediaTypeMovie, dlRoot, lib)
	o := &Organizerdata{Folder: dlRoot, MediaFile: filepath.Join(dlRoot, "heat.mkv"), TargetPath: lib, Filename: "Heat (1995)"}

	s.moveCompanionFiles(o)

	// A file lying directly in the download root only takes its own subtitles.
	if got := listDir(t, lib); !slices.Equal(got, []string{"Heat (1995).srt"}) || !exists(filepath.Join(dlRoot, "other.movie.srt")) {
		t.Errorf("target = %v", got)
	}
}

func TestCompanionsOf(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t,
		filepath.Join(dir, "Heat.srt"), filepath.Join(dir, "Heat.en.srt"), filepath.Join(dir, "Heat.forced.idx"),
		filepath.Join(dir, "Heat 2.srt"), filepath.Join(dir, "Heat.nfo"), filepath.Join(dir, "Heater.srt"),
	)

	got := companionsOf(filepath.Join(dir, "Heat.mkv"), subtitleExts)
	for i := range got {
		got[i] = filepath.Base(got[i])
	}

	sort.Strings(got)

	if want := []string{"Heat.en.srt", "Heat.forced.idx", "Heat.srt"}; !slices.Equal(got, want) {
		t.Errorf("companionsOf = %v, want %v", got, want)
	}
}

func TestIsDiscFolderName(t *testing.T) {
	for in, want := range map[string]bool{"CD1": true, "Disc 2": true, "disk_03": true, "CD": false, "Discovery": false, "Album": false} {
		if got := isDiscFolderName(in); got != want {
			t.Errorf("isDiscFolderName(%q) = %t, want %t", in, got, want)
		}
	}
}

func TestRetireOldAlbumFoldersKeepsFoldersWithAudio(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "Music")
	// Flat artist folder with another album's tracks: nothing may go.
	flat := filepath.Join(lib, "Artist")
	writeFiles(t, filepath.Join(flat, "other album 01.mp3"), filepath.Join(flat, "cover.jpg"))

	s := albumOrganizer(lib, filepath.Join(base, "dl"), true, false)
	o := &Organizerdata{TargetPath: filepath.Join(lib, "Artist", "New Album")}

	s.retireOldAlbumFolders(o, []string{filepath.Join(flat, "old album 01.mp3")}, false)

	if !exists(filepath.Join(flat, "cover.jpg")) || !exists(filepath.Join(flat, "other album 01.mp3")) {
		t.Error("folder still holding audio was cleaned")
	}

	// The library root itself is never removed.
	s.retireOldAlbumFolders(o, []string{filepath.Join(lib, "x.mp3")}, false)

	if !exists(lib) {
		t.Error("library root removed")
	}
}
