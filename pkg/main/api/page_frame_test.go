package api

import "testing"

func TestCopyrightLine(t *testing.T) {
	if got := copyrightLine(2025); got != "© 2025 Go Media Downloader - Advanced Media Automation" {
		t.Errorf("first year: %q", got)
	}

	if got := copyrightLine(2026); got != "© 2025–2026 Go Media Downloader - Advanced Media Automation" {
		t.Errorf("later year: %q", got)
	}
}
