package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// config is what the player remembers (config.json in its folder).
type config struct {
	URL     string `json:"url"`               // the portal, e.g. https://media.example.com
	Browser string `json:"browser,omitempty"` // path of the browser to use (empty: found automatically)
}

// appDir is the player's folder: config and browser profile.
func appDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "go-media-player"), nil
}

func loadConfig(dir string) config {
	var c config

	if b, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		_ = json.Unmarshal(b, &c)
	}

	return c
}

func saveConfig(dir string, c config) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.json"), b, 0o600)
}

var errBadURL = errors.New("enter the portal address, e.g. https://media.example.com or http://192.168.1.10:9090")

// normalizeURL checks a portal address and returns it without a trailing
// slash or the web interface's own path; http:// is assumed without a
// scheme.
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errBadURL
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errBadURL
	}

	// Pasted from the address bar: keep only what comes before /api.
	if i := strings.Index(u.Path, "/api"); i >= 0 {
		u.Path = u.Path[:i]
	}

	u.RawQuery, u.Fragment = "", ""

	return strings.TrimRight(u.String(), "/"), nil
}
