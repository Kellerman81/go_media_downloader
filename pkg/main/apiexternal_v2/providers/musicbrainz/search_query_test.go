package musicbrainz_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
)

// newTestProvider creates a MusicBrainz provider suitable for live API tests.
// MusicBrainz is free and requires no API key, just a descriptive User-Agent.
func newTestProvider() *musicbrainz.Provider {
	return musicbrainz.NewProviderWithConfig(base.ClientConfig{
		UserAgent:        "go_media_downloader/test (github.com/Kellerman81/go_media_downloader)",
		RateLimitCalls:   1,
		RateLimitSeconds: 2,
	})
}

// searchCase defines one search scenario.
type searchCase struct {
	name        string
	artist      string
	album       string
	trackCount  int
	wantAnyMBID string // if set, at least one result must have this MBID
}

// buildQuery mirrors the current and proposed query logic so we can compare both.
func buildQueryTitleOnly(album string) string {
	return "release:" + album
}

// luceneEscape escapes all Lucene special characters so the value is safe inside
// a quoted phrase. Mirrors importfeed.luceneEscape.
// Special characters: + - && || ! ( ) { } [ ] ^ " ~ * ? : \ /
func luceneEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		switch c {
		case '+', '-', '!', '(', ')', '{', '}', '[', ']', '^', '"', '~', '*', '?', ':', '\\', '/':
			b.WriteByte('\\')
		case '&', '|':
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func buildQueryWithArtist(album, artist string) string {
	if strings.EqualFold(artist, "Various Artists") || artist == "" {
		return buildQueryTitleOnly(album)
	}
	return `release:` + album + ` AND artist:"` + luceneEscape(artist) + `"~1`
}

// buildQueryWithArtistSlop uses a phrase-slop query: artist:"Status Quo"~1
// Allows words to be slightly out of order or with one word gap.
func buildQueryWithArtistSlop(album, artist string, slop int) string {
	if strings.EqualFold(artist, "Various Artists") || artist == "" {
		return buildQueryTitleOnly(album)
	}
	return fmt.Sprintf(`release:%s AND artist:"%s"~%d`, album, luceneEscape(artist), slop)
}

// buildQueryWithArtistPerWord splits the artist into words and requires each
// word to appear in the artist field: artist:Status AND artist:Quo
func buildQueryWithArtistPerWord(album, artist string) string {
	if strings.EqualFold(artist, "Various Artists") || artist == "" {
		return buildQueryTitleOnly(album)
	}
	words := strings.Fields(artist)
	if len(words) == 1 {
		return `release:` + album + ` AND artist:` + luceneEscape(words[0])
	}
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = `artist:` + luceneEscape(w)
	}
	return `release:` + album + ` AND ` + strings.Join(parts, ` AND `)
}

// TestMusicBrainzSearchQueryComparison compares title-only vs title+artist queries
// for cases where a common album name (e.g. "Live") drowns the real result.
//
// Run with: go test -v -run TestMusicBrainzSearchQueryComparison ./pkg/main/apiexternal_v2/providers/musicbrainz/
func TestMusicBrainzSearchQueryComparison(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	cases := []searchCase{
		{
			name:        "Status Quo - Live (single-word common title)",
			artist:      "Status Quo",
			album:       "Live",
			wantAnyMBID: "9e8c98da-02b3-4b74-9420-4df42d19e286", // Status Quo Live (12 tracks), top result with artist filter
		},
		{
			name:        "Status Quo - Rockin All Over The World (multi-word title)",
			artist:      "Status Quo",
			album:       "Rockin All Over The World",
			wantAnyMBID: "54940cb9-e5a2-45d7-92cd-8215be307a84", // top result with artist filter (13 tracks)
		},
		{
			name:        "Eagles - One of These Nights (multi-word title, specific artist)",
			artist:      "Eagles",
			album:       "One of These Nights",
			wantAnyMBID: "273f23df-2a82-40e8-9230-d217463b3eb5", // top result in both queries
		},
		{
			name:        "Deep Purple - Machine Head (multi-word title)",
			artist:      "Deep Purple",
			album:       "Machine Head",
			wantAnyMBID: "9b3bbe68-a4da-3c84-8ba1-f7a35af98597", // original 7-track edition
		},
		{
			name:        "Various Artists - no artist filter should apply",
			artist:      "Various Artists",
			album:       "Now That's What I Call Music",
			wantAnyMBID: "e2efdc06-0fa7-4e9d-835c-2a99b255b4b6", // 30-track edition, consistent in both queries
		},
		{
			name:        "Artist with special chars in name",
			artist:      `AC/DC`,
			album:       "Back in Black",
			wantAnyMBID: "4bbe3188-d626-4aff-82c0-87d455262f49", // top result in both queries
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			queryOld := buildQueryTitleOnly(tc.album)
			queryNew := buildQueryWithArtist(tc.album, tc.artist)

			t.Logf("Old query: %q", queryOld)
			t.Logf("New query: %q", queryNew)

			// --- title-only search ---
			time.Sleep(1500 * time.Millisecond) // respect rate limit
			oldResults, oldCount, err := p.SearchReleases(ctx, queryOld, 50, 0)
			if err != nil {
				t.Fatalf("title-only search failed: %v", err)
			}
			t.Logf("Title-only: total=%d returned=%d", oldCount, len(oldResults))

			oldArtistHits := 0
			for _, r := range oldResults {
				for _, a := range r.Artists {
					if strings.EqualFold(a.Name, tc.artist) {
						oldArtistHits++
						break
					}
				}
			}
			t.Logf(
				"Title-only: results with artist %q = %d / %d",
				tc.artist,
				oldArtistHits,
				len(oldResults),
			)

			if tc.wantAnyMBID != "" {
				found := false
				for _, r := range oldResults {
					if r.ID == tc.wantAnyMBID || r.MusicBrainzID == tc.wantAnyMBID {
						found = true
					}
				}
				t.Logf("Title-only: target MBID %s present = %v", tc.wantAnyMBID, found)
			}

			// --- title+artist search ---
			time.Sleep(1500 * time.Millisecond)
			newResults, newCount, err := p.SearchReleases(ctx, queryNew, 50, 0)
			if err != nil {
				t.Fatalf("title+artist search failed: %v", err)
			}
			t.Logf("Title+artist: total=%d returned=%d", newCount, len(newResults))

			newArtistHits := 0
			for _, r := range newResults {
				for _, a := range r.Artists {
					if strings.EqualFold(a.Name, tc.artist) {
						newArtistHits++
						break
					}
				}
			}
			t.Logf(
				"Title+artist: results with artist %q = %d / %d",
				tc.artist,
				newArtistHits,
				len(newResults),
			)

			if tc.wantAnyMBID != "" {
				found := false
				for _, r := range newResults {
					if r.ID == tc.wantAnyMBID || r.MusicBrainzID == tc.wantAnyMBID {
						found = true
					}
				}
				t.Logf("Title+artist: target MBID %s present = %v", tc.wantAnyMBID, found)
			}

			// Summary comparison
			improvement := newArtistHits - oldArtistHits
			t.Logf(
				"Artist-hit improvement: %+d (old=%d new=%d)",
				improvement,
				oldArtistHits,
				newArtistHits,
			)

			if improvement < 0 {
				t.Errorf(
					"title+artist query returned FEWER artist matches than title-only (%d vs %d)",
					newArtistHits,
					oldArtistHits,
				)
			}

			// Print top-5 results from each for visual inspection
			t.Log("--- Top 5 title-only results ---")
			for i, r := range oldResults {
				if i >= 5 {
					break
				}
				artistNames := make([]string, len(r.Artists))
				for j, a := range r.Artists {
					artistNames[j] = a.Name
				}
				t.Logf(
					"  [%d] %s - %s (tracks=%d, id=%s)",
					i+1,
					strings.Join(artistNames, ", "),
					r.Title,
					r.TrackCount,
					r.ID,
				)
			}
			t.Log("--- Top 5 title+artist results ---")
			for i, r := range newResults {
				if i >= 5 {
					break
				}
				artistNames := make([]string, len(r.Artists))
				for j, a := range r.Artists {
					artistNames[j] = a.Name
				}
				t.Logf(
					"  [%d] %s - %s (tracks=%d, id=%s)",
					i+1,
					strings.Join(artistNames, ", "),
					r.Title,
					r.TrackCount,
					r.ID,
				)
			}

			_ = fmt.Sprintf // suppress unused import
		})
	}
}

// TestMusicBrainzArtistQueryVariants compares three artist-filter strategies:
//   - quoted phrase:   artist:"Status Quo"
//   - phrase + slop:   artist:"Status Quo"~1
//   - per-word AND:    artist:Status AND artist:Quo
//
// Run with: go test -v -run TestMusicBrainzArtistQueryVariants ./apiexternal_v2/providers/musicbrainz/
func TestMusicBrainzArtistQueryVariants(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	cases := []struct {
		artist      string
		album       string
		wantAnyMBID string
	}{
		// Common single-word title — relies entirely on artist filter
		{"Status Quo", "Live", "04396c03-f730-4c52-a6f3-0dfe121dfc51"},
		// Multi-word title + multi-word artist
		{"Status Quo", "Rockin All Over The World", "54940cb9-e5a2-45d7-92cd-8215be307a84"},
		// Artist with special char (/ must survive escaping)
		{"AC/DC", "Back in Black", "4bbe3188-d626-4aff-82c0-87d455262f49"},
		// Multi-word artist, unique-ish title
		{"Deep Purple", "Machine Head", "9b3bbe68-a4da-3c84-8ba1-f7a35af98597"},
	}

	type variant struct {
		label string
		query func(album, artist string) string
	}
	variants := []variant{
		{
			"quoted-phrase",
			func(album, artist string) string { return buildQueryWithArtist(album, artist) },
		},
		{
			"slop-1",
			func(album, artist string) string { return buildQueryWithArtistSlop(album, artist, 1) },
		},
		{
			"per-word-AND",
			func(album, artist string) string { return buildQueryWithArtistPerWord(album, artist) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.artist+" - "+tc.album, func(t *testing.T) {
			for _, v := range variants {
				time.Sleep(1500 * time.Millisecond)
				q := v.query(tc.album, tc.artist)
				results, total, err := p.SearchReleases(ctx, q, 50, 0)
				if err != nil {
					t.Errorf("[%s] search failed: %v", v.label, err)
					continue
				}

				artistHits := 0
				wantFound := false
				for _, r := range results {
					for _, a := range r.Artists {
						if strings.EqualFold(a.Name, tc.artist) {
							artistHits++
							break
						}
					}
					if r.ID == tc.wantAnyMBID || r.MusicBrainzID == tc.wantAnyMBID {
						wantFound = true
					}
				}

				t.Logf("[%s] query=%q  total=%d returned=%d artistHits=%d wantFound=%v",
					v.label, q, total, len(results), artistHits, wantFound)

				if tc.wantAnyMBID != "" && !wantFound {
					t.Errorf("[%s] target MBID %s not found in results", v.label, tc.wantAnyMBID)
				}
			}
		})
	}
}
