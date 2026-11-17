//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestFindBrowserLinux: browsers on the PATH come first, then /opt, Snap
// and Flatpak; a Flatpak or Snap browser keeps the profile in its sandbox.
func TestFindBrowserLinux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	exe := func(p string) string {
		t.Helper()

		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}

		return p
	}

	oldOpt, oldSnap, oldFlatpak := linuxOptPaths, snapBin, flatpakBins
	t.Cleanup(func() { linuxOptPaths, snapBin, flatpakBins = oldOpt, oldSnap, oldFlatpak })

	root := t.TempDir()
	flatpakDir := filepath.Join(home, ".local", "share", "flatpak", "exports", "bin")
	linuxOptPaths = []string{filepath.Join(root, "opt", "chrome")}
	snapBin = filepath.Join(root, "snap", "bin")
	flatpakBins = func() []string { return []string{filepath.Join(root, "var", "flatpak", "exports", "bin"), flatpakDir} }

	if b := findBrowser(); b.Exe != "" {
		t.Fatalf("found %+v without any browser", b)
	}

	brave := exe(filepath.Join(flatpakDir, "com.brave.Browser"))
	b := findBrowser()
	wantProfile := filepath.Join(home, ".var", "app", "com.brave.Browser", "go-media-player-profile")

	if b.Exe != brave || b.profileDir("/cfg") != wantProfile {
		t.Errorf("flatpak Brave: %+v, profile %q", b, b.profileDir("/cfg"))
	}

	if args := appWindow(b, "/cfg", "http://x/").Args; !slices.Contains(args, "--user-data-dir="+wantProfile) {
		t.Errorf("app window args %v", args)
	}

	snap := exe(filepath.Join(snapBin, "chromium"))
	if b := findBrowser(); b.Exe != snap || b.profileDir("/cfg") != filepath.Join(home, "snap", "chromium", "common", "go-media-player-profile") {
		t.Errorf("snap before flatpak: %+v", b)
	}

	opt := exe(linuxOptPaths[0])
	if b := findBrowser(); b.Exe != opt || b.profileDir("/cfg") != filepath.Join("/cfg", "profile") {
		t.Errorf("/opt before snap: %+v", b)
	}

	onPath := exe(filepath.Join(os.Getenv("PATH"), "chromium"))
	if b := findBrowser(); b.Exe != onPath {
		t.Errorf("PATH first: %+v", b)
	}
}
