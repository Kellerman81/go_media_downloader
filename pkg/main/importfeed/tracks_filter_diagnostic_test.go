package importfeed

// TestTracksFilterDiagnostic checks whether MusicBrainz's tracks: Lucene field
// correctly filters by total track count, using "Bravo Hits 130" (48 tracks)
// as a known target.
//
// Run with: go test -v -run TestTracksFilterDiagnostic ./importfeed/

import (
	"context"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
)

func TestTracksFilterDiagnostic(t *testing.T) {
	const (
		targetTitle  = "Bravo Hits 130"
		targetArtist = "Various Artists"
		targetTracks = 48
		targetID     = "8f894280-a181-443b-826b-d64075c400b7"
	)

	mbProvider := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   1,
		RateLimitSeconds: 1,
		UserAgent:        "GoMediaDownloader/1.0 (diagnostic)",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	type searchQ struct{ label, query string }
	queries := []searchQ{
		{
			"without tracks filter",
			string(BuildArtistAlbumSearch(targetTitle, targetArtist)),
		},
		{
			"with tracks:48 (total)",
			string(BuildArtistAlbumSearch(targetTitle, targetArtist)) + " AND tracks:48",
		},
		{
			// MB tracks: field counts per-medium on some releases (2xCD → tracks:24)
			"with tracks:24 (per-disc, 2xCD)",
			string(BuildArtistAlbumSearch(targetTitle, targetArtist)) + " AND tracks:24",
		},
	}

	for _, q := range queries {
		t.Logf("\n--- %s ---", q.label)
		t.Logf("  Query: %s", q.query)
		results, _, err := mbProvider.SearchReleases(ctx, q.query, 10, 0)
		if err != nil {
			t.Logf("  ERROR: %v", err)
			continue
		}
		t.Logf("  Results: %d", len(results))
		for i, r := range results {
			targetMark := ""
			if r.MusicBrainzID == targetID {
				targetMark = " *** TARGET ***"
			}
			t.Logf("  [%d] %q | tracks=%d | year=%d | id=%s%s",
				i+1, r.Title, r.TrackCount, r.ReleaseYear, r.MusicBrainzID, targetMark)
		}
	}
}
