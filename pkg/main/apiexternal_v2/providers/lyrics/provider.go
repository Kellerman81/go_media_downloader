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
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

const (
	lyricsOVHBase = "https://api.lyrics.ovh/v1"
	lrclibBase    = "https://lrclib.net/api"

	// Per-request timeout — kept short so a dead provider doesn't stall tagging.
	fetchTimeout = 10 * time.Second
)

var (
	ovhClient = sync.OnceValue(func() *base.BaseClient {
		return newClient("lyricsovh", lyricsOVHBase)
	})
	lrclibClient = sync.OnceValue(func() *base.BaseClient {
		return newClient("lrclib", lrclibBase)
	})
)

func newClient(name, baseURL string) *base.BaseClient {
	return base.NewBaseClient(base.ClientConfig{
		Name:             name,
		BaseURL:          baseURL,
		Timeout:          fetchTimeout,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
		UserAgent:        "go_media_downloader/lyrics (+https://github.com/Kellerman81/go_media_downloader)",
	})
}

// Fetch returns plain-text lyrics for the given track.
// It tries LyricsOVH first, then falls back to LRCLIB.
// Returns an empty string when no lyrics are found from either source.
func Fetch(ctx context.Context, artist, title, album string) string {
	if artist == "" || title == "" {
		return ""
	}

	if lyr := fromLyricsOVH(ctx, ovhClient(), artist, title); lyr != "" {
		return lyr
	}

	return fromLRCLIB(ctx, lrclibClient(), artist, title, album)
}

// ---------------------------------------------------------------------------
// LyricsOVH
// ---------------------------------------------------------------------------

type lyricsOVHResponse struct {
	Lyrics string `json:"lyrics"`
	Error  string `json:"error"`
}

func fromLyricsOVH(ctx context.Context, c *base.BaseClient, artist, title string) string {
	endpoint := logger.JoinStrings("/", url.PathEscape(artist), "/", url.PathEscape(title))

	var resp lyricsOVHResponse
	if err := c.MakeRequest(ctx, "GET", endpoint, nil, &resp, nil); err != nil {
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
	SyncedLyrics string `json:"syncedLyrics"`
	Instrumental bool   `json:"instrumental"`
}

func fromLRCLIB(ctx context.Context, c *base.BaseClient, artist, title, album string) string {
	params := url.Values{}
	params.Set("artist_name", artist)
	params.Set("track_name", title)

	if album != "" {
		params.Set("album_name", album)
	}

	var resp lrclibResponse
	if err := c.MakeRequest(ctx, "GET", "/get?"+params.Encode(), nil, &resp, nil); err != nil {
		return ""
	}

	if resp.Instrumental {
		return ""
	}

	if resp.PlainLyrics != "" {
		return cleanLyrics(resp.PlainLyrics)
	}

	if resp.SyncedLyrics != "" {
		return cleanLyrics(stripLRC(cleanLyrics(resp.SyncedLyrics)))
	}

	return ""
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stripLRC removes LRC time tags ("[01:23.45]") from the start of each line
// and drops ID-tag lines ("[ar:Artist]") that carry no lyric text.
func stripLRC(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)

		hadTag := false
		for strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				break
			}

			tag := line[1:end]
			if !isLRCTimeTag(tag) {
				if !hadTag && strings.TrimSpace(line[end+1:]) == "" && strings.IndexByte(tag, ':') > 0 {
					line = ""
				}

				break
			}

			hadTag = true
			line = strings.TrimSpace(line[end+1:])
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

// isLRCTimeTag reports whether tag looks like "mm:ss", "mm:ss.xx" or "mm:ss:xx".
func isLRCTimeTag(tag string) bool {
	colon := strings.IndexByte(tag, ':')
	if colon <= 0 || colon == len(tag)-1 {
		return false
	}

	for i := 0; i < len(tag); i++ {
		c := tag[i]
		if (c < '0' || c > '9') && c != ':' && c != '.' {
			return false
		}
	}

	return true
}

// cleanLyrics normalises line endings and trims surrounding whitespace.
func cleanLyrics(s string) string {
	// Fast path: no carriage returns to normalize.
	if strings.IndexByte(s, '\r') == -1 {
		return strings.TrimSpace(s)
	}

	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			buf.WriteByte('\n')

			if i+1 < len(s) && s[i+1] == '\n' {
				i++ // skip the \n in \r\n
			}
		} else {
			buf.WriteByte(s[i])
		}
	}

	return strings.TrimSpace(buf.String())
}
