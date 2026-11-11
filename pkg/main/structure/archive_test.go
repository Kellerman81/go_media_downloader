package structure

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsArchiveFile(t *testing.T) {
	for in, want := range map[string]bool{
		"movie.rar": true, "MOVIE.ZIP": true, "a.tar.gz": true, "a.tgz": true, "a.7z": true,
		"a.xz": true, "movie.mkv": false, "a.r00": false, "rar": false,
	} {
		if got := IsArchiveFile(in); got != want {
			t.Errorf("IsArchiveFile(%q) = %t, want %t", in, got, want)
		}
	}
}

func TestIsMainArchiveFile(t *testing.T) {
	for in, want := range map[string]bool{
		"movie.rar":         true,
		"movie.part1.rar":   true,
		"movie.part01.rar":  true,
		"movie.PART001.RAR": true,
		"movie.part2.rar":   false,
		"movie.part02.rar":  false,
		"movie.zip":         true,
		"movie.7z":          true,
		"movie.7z.001":      true,
		"movie.7z.002":      false,
		"movie.001":         true,
		"movie.002":         false,
		"movie.r00":         false,
		"movie.mkv":         false,
		"movie.tar.gz":      true,
		// ".Part" in the title is not a volume number.
		"Dune.Part.Two.2024.2160p.rar":                true,
		"Harry.Potter.Hallows.Part.1.2010.1080p.rar":  true,
		"Harry.Potter.Hallows.Part.1.2010.part01.rar": true,
		"Harry.Potter.Hallows.Part.1.2010.part02.rar": false,
		"movie.part10.rar":                            false,
		"movie.part.rar":                              true,
	} {
		if got := IsMainArchiveFile(in); got != want {
			t.Errorf("IsMainArchiveFile(%q) = %t, want %t", in, got, want)
		}
	}
}

func TestIsAllDigits(t *testing.T) {
	for in, want := range map[string]bool{"001": true, "0": true, "": false, "01a": false, "-1": false} {
		if got := isAllDigits(in); got != want {
			t.Errorf("isAllDigits(%q) = %t, want %t", in, got, want)
		}
	}
}

func TestUnpackInProgress(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour)

	mk := func(t *testing.T, unpackDir string, modified time.Time) (folder, unpack string) {
		t.Helper()

		folder = t.TempDir()
		if err := os.WriteFile(filepath.Join(folder, "book.rar"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		unpack = filepath.Join(folder, unpackDir)
		if err := os.Mkdir(unpack, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.Chtimes(unpack, modified, modified); err != nil {
			t.Fatal(err)
		}

		return folder, unpack
	}

	// An old _unpack folder, empty: no reason to wait (goes to no_files).
	folder, _ := mk(t, "book_unpack", old)
	if unpackInProgress(folder, nil) {
		t.Error("old empty _unpack folder counted as in progress")
	}

	// Just created by another program (NZBGet names it _unpack): wait.
	folder, _ = mk(t, "_unpack", time.Now())
	if !unpackInProgress(folder, nil) {
		t.Error("new _unpack folder not counted as in progress")
	}

	// Just finished by this run: organize it now.
	folder, unpack := mk(t, "book_unpack", time.Now())
	if unpackInProgress(folder, []string{unpack}) {
		t.Error("_unpack folder this run extracted counted as in progress")
	}

	// Being extracted by this process, whatever its time: wait.
	folder, unpack = mk(t, "book_unpack", old)
	activeUnpacks.Store(unpack, struct{}{})
	t.Cleanup(func() { activeUnpacks.Delete(unpack) })

	if !unpackInProgress(folder, nil) {
		t.Error("_unpack folder being extracted not counted as in progress")
	}

	if unpackInProgress(t.TempDir(), nil) {
		t.Error("folder without _unpack counted as in progress")
	}
}
