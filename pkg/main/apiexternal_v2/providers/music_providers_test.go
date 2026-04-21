package providers_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/acoustid"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deezer"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/itunes"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/theaudiodb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/discogs"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/lastfm"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/spotify"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// artistNames returns a comma-separated list of artist names from an ArtistRef slice.
func artistNames(refs []apiexternal_v2.ArtistRef) string {
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}

const (
	// Well-known test album: The Dark Side of the Moon – Pink Floyd (1973)
	testMusicArtist    = "Pink Floyd"
	testMusicAlbum     = "The Dark Side of the Moon"
	testMusicAlbumYear = 1973

	// MusicBrainz release ID for The Dark Side of the Moon (UK original)
	testMBReleaseID = "b84ee12a-09ef-421b-82de-0441a926375b"

	// AcoustID track ID for "Wish You Were Here" by Pink Floyd
	testAcoustIDTrackID = "3d4961cb-8e88-4bca-b606-0e3c47e14d08"

	// Deezer album ID for The Dark Side of the Moon
	testDeezerAlbumID = 12114240

	// TheAudioDB album ID for The Dark Side of the Moon
	testTheAudioDBAlbumID = "2110073"

	// iTunes collection ID for The Dark Side of the Moon
	testITunesAlbumID = 1065973699
)

// loadMusicTestConfig reads music provider API keys from config.toml.
// Keys are optional per provider — missing keys cause individual tests to skip.
func loadMusicTestConfig(t *testing.T) (acoustIDKey, lastFMKey, discogsToken, spotifyID, spotifySecret string) {
	t.Helper()
	if config.Configfile == "" || config.Configfile == "./config/config.toml" {
		config.Configfile = "R:\\golang_ent\\config\\config.toml"
	}
	cfg, err := config.Readconfigtoml()
	if err != nil {
		t.Fatalf("Failed to read config.toml: %v", err)
	}
	return cfg.General.AcoustIDAPIKey,
		cfg.General.LastFMAPIKey,
		cfg.General.DiscogsToken,
		cfg.General.SpotifyClientID,
		cfg.General.SpotifyClientSecret
}

// testCtx returns a 30-second context for a single provider call.
func testCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// ---------------------------------------------------------------------------
// MusicBrainz — no API key required
// ---------------------------------------------------------------------------

func TestMusicBrainzReturnsData(t *testing.T) {
	p := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		UserAgent:        "go_media_downloader/test (github.com/Kellerman81/go_media_downloader)",
		RateLimitCalls:   1,
		RateLimitSeconds: 2,
	})

	t.Run("SearchReleases", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, total, err := p.SearchReleases(ctx,
			`release:"`+testMusicAlbum+`" AND artist:"`+testMusicArtist+`"`, 5, 0)
		if err != nil {
			t.Fatalf("SearchReleases error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchReleases returned no results")
		}
		t.Logf("SearchReleases: %d total, first title: %q MBID: %s", total, results[0].Title, results[0].MusicBrainzID)
		if results[0].Title == "" {
			t.Error("first result has empty title")
		}
	})

	t.Run("GetReleaseByID", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		details, err := p.GetReleaseByID(ctx, testMBReleaseID)
		if err != nil {
			t.Fatalf("GetReleaseByID error: %v", err)
		}
		if details == nil {
			t.Fatal("GetReleaseByID returned nil")
		}
		t.Logf("GetReleaseByID: %q (%s) tracks: %d", details.Title, details.ReleaseDate, details.TrackCount)
		if details.Title == "" {
			t.Error("release title is empty")
		}
	})
}

// ---------------------------------------------------------------------------
// AcoustID — requires API key
// ---------------------------------------------------------------------------

func TestAcoustIDReturnsData(t *testing.T) {
	acoustIDKey, _, _, _, _ := loadMusicTestConfig(t)
	if acoustIDKey == "" {
		t.Skip("AcoustID API key not configured — skipping")
	}

	p := acoustid.NewProviderWithConfig(base.ClientConfig{
		RateLimitCalls:   1,
		RateLimitSeconds: 1,
	}, acoustIDKey)

	ctx, cancel := testCtx(t)
	defer cancel()

	results, err := p.LookupByTrackID(ctx, testAcoustIDTrackID)
	if err != nil {
		t.Fatalf("LookupByTrackID error: %v", err)
	}
	if len(results) == 0 {
		// AcoustID track IDs can become stale if the entry has no recordings attached.
		// The API responding without error means the provider works; skip rather than fail.
		t.Skip("LookupByTrackID returned no results — track ID may have no recordings in AcoustID database")
	}
	r := results[0]
	t.Logf("AcoustID result: AcoustID=%s score=%.3f title=%q artist=%q",
		r.AcoustID, r.Score, r.RecordingTitle, r.ArtistName)
	if r.AcoustID == "" {
		t.Error("result has empty AcoustID")
	}
}

// ---------------------------------------------------------------------------
// Last.fm — requires API key
// ---------------------------------------------------------------------------

func TestLastFMReturnsData(t *testing.T) {
	_, lastFMKey, _, _, _ := loadMusicTestConfig(t)
	if lastFMKey == "" {
		t.Skip("Last.fm API key not configured — skipping")
	}

	p := lastfm.NewProviderWithConfig(base.ClientConfig{
		APIKey:           lastFMKey,
		UserAgent:        "go_media_downloader/test (github.com/Kellerman81/go_media_downloader)",
		RateLimitCalls:   2,
		RateLimitSeconds: 1,
	})

	t.Run("SearchArtists", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchArtists(ctx, testMusicArtist, 5)
		if err != nil {
			t.Fatalf("SearchArtists error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchArtists returned no results")
		}
		t.Logf("SearchArtists first: %q MBID: %s", results[0].Name, results[0].MusicBrainzID)
		if results[0].Name == "" {
			t.Error("first artist has empty name")
		}
	})

	t.Run("SearchAlbums", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchAlbums(ctx, testMusicAlbum, 5)
		if err != nil {
			t.Fatalf("SearchAlbums error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchAlbums returned no results")
		}
		t.Logf("SearchAlbums first: %q by %q", results[0].Title, artistNames(results[0].Artists))
		if results[0].Title == "" {
			t.Error("first album has empty title")
		}
	})

	t.Run("GetAlbumInfo", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		details, err := p.GetAlbumInfo(ctx, testMusicArtist, testMusicAlbum, "")
		if err != nil {
			t.Fatalf("GetAlbumInfo error: %v", err)
		}
		if details == nil {
			t.Fatal("GetAlbumInfo returned nil")
		}
		t.Logf("GetAlbumInfo: %q by %q tracks: %d", details.Title, artistNames(details.Artists), details.TrackCount)
		if details.Title == "" {
			t.Error("album title is empty")
		}
	})
}

// ---------------------------------------------------------------------------
// Discogs — works without auth (lower rate limit), token optional
// ---------------------------------------------------------------------------

func TestDiscogsReturnsData(t *testing.T) {
	_, _, discogsToken, _, _ := loadMusicTestConfig(t)
	// Discogs works without a token; token just raises the rate limit.

	p := discogs.NewProviderWithConfig(base.ClientConfig{
		UserAgent:        "go_media_downloader/test (github.com/Kellerman81/go_media_downloader)",
		RateLimitCalls:   5,
		RateLimitSeconds: 60,
	}, discogsToken)

	t.Run("SearchReleases", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchReleases(ctx, testMusicArtist+" "+testMusicAlbum, 5)
		if err != nil {
			t.Fatalf("SearchReleases error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchReleases returned no results")
		}
		r := results[0]
		t.Logf("Discogs SearchReleases first: %q by %q year: %d", r.Title, artistNames(r.Artists), r.ReleaseYear)
		if r.Title == "" {
			t.Error("first result has empty title")
		}
	})

	t.Run("SearchArtists", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchArtists(ctx, testMusicArtist, 5)
		if err != nil {
			t.Fatalf("SearchArtists error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchArtists returned no results")
		}
		t.Logf("Discogs SearchArtists first: %q ID: %s", results[0].Name, results[0].ID)
		if results[0].Name == "" {
			t.Error("first artist has empty name")
		}
	})
}

// ---------------------------------------------------------------------------
// Deezer — public API, no auth required
// ---------------------------------------------------------------------------

func TestDeezerReturnsData(t *testing.T) {
	p := deezer.NewProviderWithConfig(base.ClientConfig{
		RateLimitCalls:   10,
		RateLimitSeconds: 5,
	})

	t.Run("SearchAlbums", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchAlbums(ctx, testMusicArtist+" "+testMusicAlbum, 5)
		if err != nil {
			t.Fatalf("SearchAlbums error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchAlbums returned no results")
		}
		r := results[0]
		t.Logf("Deezer SearchAlbums first: %q by %q DeezerID: %d", r.Title, artistNames(r.Artists), r.DeezerID)
		if r.Title == "" {
			t.Error("first result has empty title")
		}
	})

	t.Run("GetAlbumByID", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		details, err := p.GetAlbumByID(ctx, testDeezerAlbumID)
		if err != nil {
			t.Fatalf("GetAlbumByID error: %v", err)
		}
		if details == nil {
			t.Fatal("GetAlbumByID returned nil")
		}
		t.Logf("Deezer GetAlbumByID: %q by %q tracks: %d", details.Title, artistNames(details.Artists), details.TrackCount)
		if details.Title == "" {
			t.Error("album title is empty")
		}
	})
}

// ---------------------------------------------------------------------------
// Spotify — requires client ID + secret
// ---------------------------------------------------------------------------

func TestSpotifyReturnsData(t *testing.T) {
	_, _, _, spotifyID, spotifySecret := loadMusicTestConfig(t)
	if spotifyID == "" || spotifySecret == "" {
		t.Skip("Spotify credentials not configured — skipping")
	}

	p := spotify.NewProviderWithConfig(base.ClientConfig{
		UserAgent:        "go_media_downloader/test (github.com/Kellerman81/go_media_downloader)",
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
	}, spotifyID, spotifySecret, "")

	t.Run("SearchAlbums", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchAlbums(ctx, testMusicArtist, testMusicAlbum, 5)
		if err != nil {
			if strings.Contains(err.Error(), "HTTP 403") {
				t.Skipf("Spotify returned 403 — credentials may lack required API permissions or token reuse is blocked: %v", err)
			}
			t.Fatalf("SearchAlbums error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchAlbums returned no results")
		}
		r := results[0]
		t.Logf("Spotify SearchAlbums first: %q by %q SpotifyID: %s", r.Title, artistNames(r.Artists), r.ID)
		if r.Title == "" {
			t.Error("first result has empty title")
		}
	})
}

// ---------------------------------------------------------------------------
// TheAudioDB — public API, free key "2" requires no configuration
// ---------------------------------------------------------------------------

func TestTheAudioDBReturnsData(t *testing.T) {
	p := theaudiodb.NewProviderWithConfig(base.ClientConfig{
		RateLimitCalls:   2,
		RateLimitSeconds: 1,
	}, "") // empty → uses free public key "2"

	// Search first so GetTracksByAlbumID uses a live ID rather than a hardcoded one.
	var foundAlbumID string

	t.Run("SearchAlbums", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchAlbums(ctx, testMusicArtist, testMusicAlbum)
		if err != nil {
			t.Fatalf("SearchAlbums error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchAlbums returned no results")
		}
		r := results[0]
		t.Logf("TheAudioDB SearchAlbums first: %q by %q TheAudioDBID: %s year: %d",
			r.Title, artistNames(r.Artists), r.TheAudioDBID, r.ReleaseYear)
		if r.Title == "" {
			t.Error("first result has empty title")
		}
		if r.TheAudioDBID == "" {
			t.Error("first result has empty TheAudioDBID")
		}
		foundAlbumID = r.TheAudioDBID
	})

	t.Run("GetTracksByAlbumID", func(t *testing.T) {
		albumID := foundAlbumID
		if albumID == "" {
			albumID = testTheAudioDBAlbumID // fallback to constant if search didn't run
		}

		ctx, cancel := testCtx(t)
		defer cancel()

		details, err := p.GetTracksByAlbumID(ctx, albumID)
		if err != nil {
			t.Fatalf("GetTracksByAlbumID error: %v", err)
		}
		if details == nil {
			t.Skipf("GetTracksByAlbumID returned nil for album %s — free-tier API may not return track listings", albumID)
		}
		t.Logf("TheAudioDB GetTracksByAlbumID: %q by %q tracks: %d",
			details.Title, artistNames(details.Artists), details.TrackCount)
		if details.TrackCount == 0 {
			t.Error("album has no tracks")
		}
		if len(details.Tracks) > 0 && details.Tracks[0].Duration == 0 {
			t.Log("warning: first track has zero duration (free-tier API may omit durations)")
		}
	})
}

// ---------------------------------------------------------------------------
// iTunes Search API — public API, no auth required
// ---------------------------------------------------------------------------

func TestITunesReturnsData(t *testing.T) {
	p := itunes.NewProviderWithConfig(base.ClientConfig{
		RateLimitCalls:   20,
		RateLimitSeconds: 60,
	})

	var foundCollectionID int

	t.Run("SearchAlbums", func(t *testing.T) {
		ctx, cancel := testCtx(t)
		defer cancel()

		results, err := p.SearchAlbums(ctx, testMusicArtist, testMusicAlbum, 5)
		if err != nil {
			t.Fatalf("SearchAlbums error: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("SearchAlbums returned no results")
		}
		r := results[0]
		t.Logf("iTunes SearchAlbums first: %q by %q ITunesID: %d tracks: %d year: %d",
			r.Title, artistNames(r.Artists), r.ITunesID, r.TrackCount, r.ReleaseYear)
		if r.Title == "" {
			t.Error("first result has empty title")
		}
		if r.ITunesID == 0 {
			t.Error("first result has zero ITunesID")
		}
		foundCollectionID = r.ITunesID
	})

	t.Run("GetAlbumTracks", func(t *testing.T) {
		collectionID := foundCollectionID
		if collectionID == 0 {
			collectionID = testITunesAlbumID
		}

		ctx, cancel := testCtx(t)
		defer cancel()

		details, err := p.GetAlbumTracks(ctx, collectionID)
		if err != nil {
			t.Fatalf("GetAlbumTracks error: %v", err)
		}
		if details == nil {
			t.Fatal("GetAlbumTracks returned nil")
		}
		t.Logf("iTunes GetAlbumTracks: %q by %q tracks: %d",
			details.Title, artistNames(details.Artists), details.TrackCount)
		if details.TrackCount == 0 {
			t.Error("album has no tracks")
		}
		if len(details.Tracks) > 0 && details.Tracks[0].Duration == 0 {
			t.Error("first track has zero duration — trackTimeMillis missing from response")
		}
	})
}

// ---------------------------------------------------------------------------
// FallbackDiag — trace every fallback provider for a specific album
//
// Run with:
//   go test -v -run TestFallbackDiag ./pkg/main/apiexternal_v2/providers/
// ---------------------------------------------------------------------------

const (
	diagArtist    = "Stef Bos"
	diagAlbum     = "In Een Ander Licht"
	diagFileCount = 11 // change to the actual number of files in the folder
)

// logTracks dumps every track from a ReleaseDetails, flagging (disc,track) collisions.
func logTracks(t *testing.T, provider string, details *apiexternal_v2.ReleaseDetails) {
	t.Helper()
	seen := make(map[uint32]bool, len(details.Tracks))
	for i, tr := range details.Tracks {
		dn := tr.DiscNumber
		if dn == 0 {
			dn = 1
		}
		tn := tr.TrackNumber
		if tn == 0 {
			tn = tr.Position
		}
		if tn == 0 {
			tn = i + 1
		}
		key := uint32(dn)*10000 + uint32(tn)
		collision := ""
		if seen[key] {
			collision = " *** COLLISION ***"
		}
		seen[key] = true
		t.Logf("  [%s] track %d: disc=%d track=%d pos=%d dur=%s title=%q%s",
			provider, i+1, tr.DiscNumber, tr.TrackNumber, tr.Position,
			tr.Duration.Round(time.Second), tr.Title, collision)
	}
	if len(details.Artists) > 0 {
		t.Logf("  [%s] artists from API: %q", provider, artistNames(details.Artists))
	} else {
		t.Logf("  [%s] artists from API: (empty — fallback would use folder artist %q)", provider, diagArtist)
	}
}

func TestFallbackDiag(t *testing.T) {
	_, lastFMKey, discogsToken, _, _ := loadMusicTestConfig(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// ── Last.fm ────────────────────────────────────────────────────────────────
	t.Run("LastFM", func(t *testing.T) {
		if lastFMKey == "" {
			t.Skip("Last.fm API key not configured — skipping")
		}
		p := lastfm.NewProviderWithConfig(base.ClientConfig{
			APIKey:           lastFMKey,
			UserAgent:        "go_media_downloader/test (github.com/Kellerman81/go_media_downloader)",
			RateLimitCalls:   2,
			RateLimitSeconds: 1,
		})

		details, err := p.GetAlbumInfo(ctx, diagArtist, diagAlbum, "")
		if err != nil || details == nil {
			t.Logf("Last.fm GetAlbumInfo error or nil: %v", err)

			results, serr := p.SearchAlbums(ctx, diagArtist+" "+diagAlbum, 5)
			if serr != nil || len(results) == 0 {
				t.Logf("Last.fm SearchAlbums also failed: %v", serr)
				return
			}
			t.Logf("Last.fm SearchAlbums: %d results", len(results))
			for i, r := range results {
				t.Logf("  [%d] %q by %q MBID=%s tracks=%d", i+1, r.Title, artistNames(r.Artists), r.MusicBrainzID, r.TrackCount)
			}
			return
		}
		t.Logf("Last.fm GetAlbumInfo: %q by %q tracks=%d fileCount=%d MATCH=%v",
			details.Title, artistNames(details.Artists), details.TrackCount, diagFileCount, details.TrackCount == diagFileCount)
		logTracks(t, "lastfm", details)
	})

	// ── Discogs ────────────────────────────────────────────────────────────────
	t.Run("Discogs", func(t *testing.T) {
		p := discogs.NewProviderWithConfig(base.ClientConfig{
			UserAgent:        "go_media_downloader/test (github.com/Kellerman81/go_media_downloader)",
			RateLimitCalls:   5,
			RateLimitSeconds: 60,
		}, discogsToken)

		results, err := p.SearchReleases(ctx, diagArtist+" "+diagAlbum, 5)
		if err != nil || len(results) == 0 {
			t.Logf("Discogs SearchReleases failed: %v", err)
			return
		}
		t.Logf("Discogs SearchReleases: %d results", len(results))
		for i, r := range results {
			t.Logf("  [%d] %q by %q DiscogsID=%d year=%d tracks=%d",
				i+1, r.Title, artistNames(r.Artists), r.DiscogsID, r.ReleaseYear, r.TrackCount)
		}

		// Fetch full details for first candidate with matching track count.
		for i, r := range results {
			details, ferr := p.GetReleaseByID(ctx, r.DiscogsID)
			if ferr != nil || details == nil {
				t.Logf("  [%d] GetReleaseByID(%d) error: %v", i+1, r.DiscogsID, ferr)
				continue
			}
			t.Logf("  [%d] GetReleaseByID(%d): %q tracks=%d fileCount=%d MATCH=%v",
				i+1, r.DiscogsID, details.Title, len(details.Tracks), diagFileCount, len(details.Tracks) == diagFileCount)
			logTracks(t, "discogs", details)
			if len(details.Tracks) == diagFileCount {
				break // found a matching candidate
			}
		}
	})

	// ── Deezer ────────────────────────────────────────────────────────────────
	t.Run("Deezer", func(t *testing.T) {
		p := deezer.NewProviderWithConfig(base.ClientConfig{
			RateLimitCalls:   10,
			RateLimitSeconds: 5,
		})

		results, err := p.SearchAlbums(ctx, diagArtist+" "+diagAlbum, 5)
		if err != nil || len(results) == 0 {
			t.Logf("Deezer SearchAlbums failed: %v", err)
			return
		}
		t.Logf("Deezer SearchAlbums: %d results", len(results))
		for i, r := range results {
			t.Logf("  [%d] %q by %q DeezerID=%d tracks=%d",
				i+1, r.Title, artistNames(r.Artists), r.DeezerID, r.TrackCount)
		}

		for i, r := range results {
			details, ferr := p.GetAlbumByID(ctx, r.DeezerID)
			if ferr != nil || details == nil {
				t.Logf("  [%d] GetAlbumByID(%d) error: %v", i+1, r.DeezerID, ferr)
				continue
			}
			t.Logf("  [%d] GetAlbumByID(%d): %q tracks=%d fileCount=%d MATCH=%v",
				i+1, r.DeezerID, details.Title, len(details.Tracks), diagFileCount, len(details.Tracks) == diagFileCount)
			logTracks(t, "deezer", details)
			if len(details.Tracks) == diagFileCount {
				break
			}
		}
	})

	// ── TheAudioDB ─────────────────────────────────────────────────────────────
	t.Run("TheAudioDB", func(t *testing.T) {
		p := theaudiodb.NewProviderWithConfig(base.ClientConfig{
			RateLimitCalls:   2,
			RateLimitSeconds: 1,
		}, "")

		results, err := p.SearchAlbums(ctx, diagArtist, diagAlbum)
		if err != nil || len(results) == 0 {
			t.Logf("TheAudioDB SearchAlbums failed: %v", err)
			return
		}
		t.Logf("TheAudioDB SearchAlbums: %d results", len(results))
		for i, r := range results {
			t.Logf("  [%d] %q by %q TheAudioDBID=%s tracks=%d",
				i+1, r.Title, artistNames(r.Artists), r.TheAudioDBID, r.TrackCount)
		}

		for i, r := range results {
			details, ferr := p.GetTracksByAlbumID(ctx, r.TheAudioDBID)
			if ferr != nil || details == nil {
				t.Logf("  [%d] GetTracksByAlbumID(%s) error or nil: %v", i+1, r.TheAudioDBID, ferr)
				continue
			}
			t.Logf("  [%d] GetTracksByAlbumID(%s): %q tracks=%d fileCount=%d MATCH=%v",
				i+1, r.TheAudioDBID, details.Title, len(details.Tracks), diagFileCount, len(details.Tracks) == diagFileCount)
			logTracks(t, "theaudiodb", details)
			if len(details.Tracks) == diagFileCount {
				break
			}
		}
	})

	// ── iTunes ─────────────────────────────────────────────────────────────────
	t.Run("iTunes", func(t *testing.T) {
		p := itunes.NewProviderWithConfig(base.ClientConfig{
			RateLimitCalls:   20,
			RateLimitSeconds: 60,
		})

		results, err := p.SearchAlbums(ctx, diagArtist, diagAlbum, 5)
		if err != nil || len(results) == 0 {
			t.Logf("iTunes SearchAlbums failed: %v", err)
			return
		}
		t.Logf("iTunes SearchAlbums: %d results", len(results))
		for i, r := range results {
			t.Logf("  [%d] %q by %q ITunesID=%d tracks=%d year=%d",
				i+1, r.Title, artistNames(r.Artists), r.ITunesID, r.TrackCount, r.ReleaseYear)
		}

		for i, r := range results {
			details, ferr := p.GetAlbumTracks(ctx, r.ITunesID)
			if ferr != nil || details == nil {
				t.Logf("  [%d] GetAlbumTracks(%d) error: %v", i+1, r.ITunesID, ferr)
				continue
			}
			t.Logf("  [%d] GetAlbumTracks(%d): %q tracks=%d fileCount=%d MATCH=%v",
				i+1, r.ITunesID, details.Title, len(details.Tracks), diagFileCount, len(details.Tracks) == diagFileCount)
			logTracks(t, "itunes", details)
			if len(details.Tracks) == diagFileCount {
				break
			}
		}
	})
}

// ---------------------------------------------------------------------------
// AllMusicProviders — convenience runner
// ---------------------------------------------------------------------------

func TestAllMusicProviders(t *testing.T) {
	t.Run("MusicBrainz", func(t *testing.T) { TestMusicBrainzReturnsData(t) })
	t.Run("AcoustID", func(t *testing.T) { TestAcoustIDReturnsData(t) })
	t.Run("LastFM", func(t *testing.T) { TestLastFMReturnsData(t) })
	t.Run("Discogs", func(t *testing.T) { TestDiscogsReturnsData(t) })
	t.Run("Deezer", func(t *testing.T) { TestDeezerReturnsData(t) })
	t.Run("Spotify", func(t *testing.T) { TestSpotifyReturnsData(t) })
	t.Run("TheAudioDB", func(t *testing.T) { TestTheAudioDBReturnsData(t) })
	t.Run("iTunes", func(t *testing.T) { TestITunesReturnsData(t) })
}
