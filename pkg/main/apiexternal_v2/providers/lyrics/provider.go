// Package lyrics fetches plain-text song lyrics from free public APIs.
// Two sources are tried in order:
//  1. LyricsOVH (api.lyrics.ovh) — no auth required
//  2. LRCLIB (lrclib.net)        — no auth required, also has synced lyrics
//
// Neither source requires an API key. Both are tried with a short timeout so a
// slow/unavailable provider does not delay the overall tag-writing pass.
package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	lyricsOVHBase = "https://api.lyrics.ovh/v1"
	lrclibBase    = "https://lrclib.net/api/get"

	// Per-request timeout — kept short so a dead provider doesn't stall tagging.
	fetchTimeout = 10 * time.Second
)

var httpClient = &http.Client{Timeout: fetchTimeout}

// Fetch returns plain-text lyrics for the given track.
// It tries LyricsOVH first, then falls back to LRCLIB.
// Returns an empty string when no lyrics are found from either source.
func Fetch(ctx context.Context, artist, title, album string) string {
	if artist == "" || title == "" {
		return ""
	}

	if lyr := fromLyricsOVH(ctx, artist, title); lyr != "" {
		return lyr
	}
	return fromLRCLIB(ctx, artist, title, album)
}

// ---------------------------------------------------------------------------
// LyricsOVH
// ---------------------------------------------------------------------------

type lyricsOVHResponse struct {
	Lyrics string `json:"lyrics"`
	Error  string `json:"error"`
}

func fromLyricsOVH(ctx context.Context, artist, title string) string {
	endpoint := fmt.Sprintf("%s/%s/%s",
		lyricsOVHBase,
		url.PathEscape(artist),
		url.PathEscape(title),
	)

	body, err := get(ctx, endpoint)
	if err != nil {
		return ""
	}

	var resp lyricsOVHResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}
	if resp.Error != "" || resp.Lyrics == "" {
		return ""
	}

	return cleanLyrics(resp.Lyrics)
}

// ---------------------------------------------------------------------------
// LRCLIB
// ---------------------------------------------------------------------------

type lrclibResponse struct {
	PlainLyrics  string `json:"plainLyrics"`
	Instrumental bool   `json:"instrumental"`
}

func fromLRCLIB(ctx context.Context, artist, title, album string) string {
	params := url.Values{}
	params.Set("artist_name", artist)
	params.Set("track_name", title)
	if album != "" {
		params.Set("album_name", album)
	}

	body, err := get(ctx, lrclibBase+"?"+params.Encode())
	if err != nil {
		return ""
	}

	var resp lrclibResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}
	if resp.Instrumental || resp.PlainLyrics == "" {
		return ""
	}

	return cleanLyrics(resp.PlainLyrics)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "go_media_downloader/lyrics (+https://github.com/Kellerman81/go_media_downloader)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lyrics: HTTP %d from %s", resp.StatusCode, rawURL)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // cap at 512 KB
}

// cleanLyrics normalises line endings and trims surrounding whitespace.
func cleanLyrics(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}
