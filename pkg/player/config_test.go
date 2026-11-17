package main

import "testing"

func TestNormalizeURL(t *testing.T) {
	for in, want := range map[string]string{
		"media.example.com":                                 "http://media.example.com",
		"https://media.example.com/":                        "https://media.example.com",
		" http://192.168.1.10:9090 ":                        "http://192.168.1.10:9090",
		"https://media.example.com/api/admin/library?app=1": "https://media.example.com",
		"https://example.com/media/api/admin":               "https://example.com/media",
	} {
		if got, err := normalizeURL(in); err != nil || got != want {
			t.Errorf("%q: %q, %v; want %q", in, got, err, want)
		}
	}

	for _, bad := range []string{"", "ftp://x", "http://"} {
		if _, err := normalizeURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}

	if got := appURL("https://m.example.com"); got != "https://m.example.com/api/admin/library?app=1" {
		t.Errorf("app URL %q", got)
	}

	dir := t.TempDir()
	if err := saveConfig(dir, config{URL: "https://m.example.com", Browser: "/usr/bin/chromium"}); err != nil {
		t.Fatal(err)
	}

	if c := loadConfig(dir); c.URL != "https://m.example.com" || c.Browser != "/usr/bin/chromium" {
		t.Errorf("loaded %+v", c)
	}
}
