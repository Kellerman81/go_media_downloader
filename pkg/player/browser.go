package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// browser is a Chromium-based browser that can open app windows.
type browser struct {
	Exe string // what to run
	// Sandbox is the folder a Flatpak or Snap browser may write to (its
	// profile goes there); empty for a normal install.
	Sandbox string
}

// Linux installs, in order of preference: on the PATH, in /opt, as Snap
// and as Flatpak (system and user installation).
var (
	linuxPathNames = []string{
		"microsoft-edge", "microsoft-edge-stable", "google-chrome", "google-chrome-stable",
		"chromium", "chromium-browser", "brave-browser", "brave", "vivaldi", "vivaldi-stable",
	}
	linuxOptPaths = []string{
		"/opt/microsoft/msedge/msedge", "/opt/google/chrome/chrome", "/opt/brave.com/brave/brave", "/opt/vivaldi/vivaldi",
	}
	snapNames  = []string{"chromium", "brave", "vivaldi"}
	flatpakIDs = []string{
		"com.microsoft.Edge", "com.google.Chrome", "org.chromium.Chromium", "com.brave.Browser",
		"io.github.ungoogled_software.ungoogled_chromium", "com.vivaldi.Vivaldi",
	}
	snapBin = "/snap/bin"
	// flatpakBins are the folders Flatpak puts its launchers in.
	flatpakBins = func() []string {
		dirs := []string{"/var/lib/flatpak/exports/bin"}
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, ".local", "share", "flatpak", "exports", "bin"))
		}

		return dirs
	}
)

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// findBrowser returns a Chromium-based browser (Edge, Chrome, Chromium,
// Brave, Vivaldi), or one with an empty Exe.
func findBrowser() browser {
	if runtime.GOOS == "windows" {
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
			base := os.Getenv(env)
			if base == "" {
				continue
			}

			for _, c := range []string{
				filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(base, "Chromium", "Application", "chrome.exe"),
				filepath.Join(base, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			} {
				if isFile(c) {
					return browser{Exe: c}
				}
			}
		}

		return browser{}
	}

	for _, name := range linuxPathNames {
		if p, err := exec.LookPath(name); err == nil {
			return browserAt(p)
		}
	}

	for _, p := range linuxOptPaths {
		if isFile(p) {
			return browser{Exe: p}
		}
	}

	for _, name := range snapNames {
		if p := filepath.Join(snapBin, name); isFile(p) {
			return browserAt(p)
		}
	}

	for _, dir := range flatpakBins() {
		for _, id := range flatpakIDs {
			if p := filepath.Join(dir, id); isFile(p) {
				return browserAt(p)
			}
		}
	}

	return browser{}
}

// browserAt is the browser at path, with the sandbox folder of a Flatpak
// (its launcher is named after the app ID) or Snap install.
func browserAt(path string) browser {
	b := browser{Exe: path}

	home, err := os.UserHomeDir()
	if err != nil {
		return b
	}

	dir, name := filepath.Dir(path), filepath.Base(path)

	switch {
	case strings.HasSuffix(filepath.ToSlash(dir), "flatpak/exports/bin"):
		b.Sandbox = filepath.Join(home, ".var", "app", name)
	case dir == snapBin:
		b.Sandbox = filepath.Join(home, "snap", name, "common")
	}

	return b
}

// profileDir is the player's own browser profile: in the player's folder,
// or inside the sandbox of a Flatpak or Snap browser, which can't write
// anywhere else.
func (b browser) profileDir(appDir string) string {
	if b.Sandbox != "" {
		return filepath.Join(b.Sandbox, "go-media-player-profile")
	}

	return filepath.Join(appDir, "profile")
}

// appWindow starts the browser with url in an app window (no tabs or
// address bar) and the player's own profile, so its login stays apart
// from the normal browser.
func appWindow(b browser, appDir, url string) *exec.Cmd {
	return exec.Command(b.Exe,
		"--app="+url,
		"--user-data-dir="+b.profileDir(appDir),
		"--no-first-run",
		"--no-default-browser-check",
		"--window-size=1366,860",
		"--class=GoMediaPlayer",
		// The next track starts without a click.
		"--autoplay-policy=no-user-gesture-required",
	)
}

// defaultBrowser opens url in the system's default browser.
func defaultBrowser(url string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}

	return exec.Command("xdg-open", url)
}
