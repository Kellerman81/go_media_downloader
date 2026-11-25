package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStartPage: the start page is written into the folder and opened as a
// file:// address carrying the configured portal and the flags.
func TestStartPage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sandbox")

	page, err := startPage(dir, "https://media.example.com", true, false)
	if err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(page)
	if err != nil || u.Scheme != "file" || u.Path != filepath.ToSlash(filepath.Join(dir, setupFile)) {
		t.Fatalf("page address %q, %v", page, err)
	}

	if q := u.Query(); q.Get("url") != "https://media.example.com" || q.Get("force") != "1" || q.Has("setup") {
		t.Errorf("query %v", q)
	}

	b, err := os.ReadFile(filepath.Join(dir, setupFile))
	if err != nil || !strings.Contains(string(b), "/api/admin/library?app=1") || !strings.Contains(string(b), "localStorage") {
		t.Errorf("page content: %v", err)
	}

	if page, _ := startPage(dir, "", false, true); strings.Contains(page, "url=") || !strings.Contains(page, "setup=1") {
		t.Errorf("setup page address %q", page)
	}

	if got := fileURL("C:/Users/me/go-media-player/"+setupFile, url.Values{"url": {"http://x"}}); !strings.HasPrefix(got, "file:///C:/Users/me/") ||
		!strings.HasSuffix(got, "?url=http%3A%2F%2Fx") {
		t.Errorf("windows file URL %q", got)
	}
}
