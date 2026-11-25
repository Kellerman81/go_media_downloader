package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/lyrics"
	"github.com/gin-gonic/gin"
)

// Lyrics of a track for the music apps (Subsonic, Jellyfin): the file's
// own lyrics tag, else LyricsOVH or LRCLIB (the tagger's sources), cached
// for a day, missing ones too.

const lyricsCacheTime = 24 * time.Hour

type lyricsEntry struct {
	text  string
	until time.Time
}

var lyricsCache sync.Map // file path or artist|title -> lyricsEntry

// tagLyrics is a file's lyrics tag ("" without one).
func tagLyrics(ctx context.Context, loc string) string {
	p, err := probeMedia(ctx, loc)
	if err != nil {
		return ""
	}

	for k, v := range p.Tags {
		if strings.HasPrefix(k, "lyrics") || k == "unsyncedlyrics" {
			return v
		}
	}

	return ""
}

// fetchLyrics returns cached lyrics or loads them.
func fetchLyrics(key string, load func() string) string {
	if v, ok := lyricsCache.Load(key); ok && time.Now().Before(v.(lyricsEntry).until) {
		return v.(lyricsEntry).text
	}

	text := strings.TrimSpace(load())
	lyricsCache.Store(key, lyricsEntry{text: text, until: time.Now().Add(lyricsCacheTime)})

	return text
}

// trackLyrics returns the lyrics of a track row (jfTrack).
func trackLyrics(ctx context.Context, r *jfRow, title string) string {
	return fetchLyrics("file|"+r.Location, func() string {
		if text := tagLyrics(ctx, r.Location); text != "" {
			return text
		}

		if r.ArtistID == 0 {
			return ""
		}

		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		return lyrics.Fetch(ctx, r.Artist, title, r.ParentName)
	})
}

// subGetLyrics answers lyrics by artist and title.
func subGetLyrics(c *gin.Context) {
	artist, title := subParam(c, "artist"), subParam(c, "title")

	text := ""
	if artist != "" && title != "" {
		text = fetchLyrics("song|"+strings.ToLower(artist+"|"+title), func() string {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
			defer cancel()

			return lyrics.Fetch(ctx, artist, title, "")
		})
	}

	r := newSubResp()
	r.Lyrics = &subLyrics{Artist: artist, Title: title, Text: text}
	writeSub(c, r)
}

// subGetLyricsBySongID answers the lyrics of a song (OpenSubsonic
// songLyrics), one line per line, not synced.
func subGetLyricsBySongID(c *gin.Context) {
	r := newSubResp()
	r.LyricsList = &subLyricsList{Structured: []subStructuredLyrics{}}

	if id, ok := subID(c, "tr"); ok {
		rows, items := subQuery(c, jfTrack, " AND x.id = ?", []any{id}, "", 0, 1)
		if len(rows) > 0 {
			if text := trackLyrics(c.Request.Context(), &rows[0], items[0].Name); text != "" {
				l := subStructuredLyrics{Lang: "xxx", DisplayArtist: rows[0].Artist, DisplayTitle: items[0].Name}
				for line := range strings.SplitSeq(text, "\n") {
					l.Lines = append(l.Lines, subLyricsLine{Value: strings.TrimRight(line, "\r")})
				}

				r.LyricsList.Structured = append(r.LyricsList.Structured, l)
			}
		}
	}

	writeSub(c, r)
}

// jfLyrics answers a track's lyrics (/Audio/{id}/Lyrics), 404 without.
func jfLyrics(c *gin.Context, p map[string]string) {
	code, n, ok := parseJFID(p["id"])
	if !ok || code != jfTrack {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	k := jfKinds[jfTrack]

	rows, _ := jfRows(k, jfQuery{Groups: jfGroups(c), Cond: " AND x.id = ?", Args: []any{n}, NoTotal: true})
	if len(rows) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	items := jfItems(k, rows, "", requestLanguage(c))

	text := trackLyrics(c.Request.Context(), &rows[0], items[0].Name)
	if text == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	lines := []gin.H{}
	for line := range strings.SplitSeq(text, "\n") {
		lines = append(lines, gin.H{"Text": strings.TrimRight(line, "\r")})
	}

	c.JSON(http.StatusOK, gin.H{"Metadata": gin.H{"Artist": rows[0].Artist, "Title": items[0].Name, "Album": rows[0].ParentName, "IsSynced": false},
		"Lyrics": lines})
}

func init() {
	jfRoute2(http.MethodGet, "audio/{id}/lyrics", false, jfLyrics)
}
