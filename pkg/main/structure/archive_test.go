package structure

import (
	"testing"
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
