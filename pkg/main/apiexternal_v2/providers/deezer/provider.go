package deezer

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Deezer Provider - Music metadata (public API, no auth required)
// API: https://developers.deezer.com/api
// Rate limit: ~50 requests per 5 seconds (unauthenticated)
//

const defaultBaseURL = "https://api.deezer.com"

// Provider implements the music metadata provider for Deezer.
type Provider struct {
	*base.BaseClient
}

// NewProvider creates a new Deezer provider using rate limits from the application config.
func NewProvider() *Provider {
	cfg := config.GetSettingsGeneral()
	rateSec := cfg.DeezerLimiterSeconds
	rateCalls := cfg.DeezerLimiterCalls

	if rateSec == 0 {
		rateSec = 5
	}

	if rateCalls == 0 {
		rateCalls = 50
	}

	return NewProviderWithConfig(base.ClientConfig{
		RateLimitSeconds: int(rateSec),
		RateLimitCalls:   rateCalls,
	})
}

// NewProviderWithConfig creates a new Deezer provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig) *Provider {
	cfg.Name = "deezer"
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 50
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 5
	}

	return &Provider{BaseClient: base.NewBaseClient(cfg)}
}

// GetProviderType returns the provider type.
func (*Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderDeezer
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "deezer"
}

// SearchAlbums searches for albums by query string.
// Returns up to limit results.
func (p *Provider) SearchAlbums(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/search/album?q=")
	buf.WriteURL(query)
	buf.WriteString("&limit=")
	buf.WriteInt(limit)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var resp deezerSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}

	if err := resp.Error.toError(); err != nil {
		return nil, err
	}

	return convertSearchToReleases(resp.Data), nil
}

// GetAlbumByID fetches full album details including tracks.
func (p *Provider) GetAlbumByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ReleaseDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/album/")
	buf.WriteInt(id)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var album deezerAlbumDetail
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &album, nil); err != nil {
		return nil, err
	}

	if err := album.Error.toError(); err != nil {
		return nil, err
	}

	// The tracklist embedded in /album/{id} has no track_position,
	// disk_number or isrc; /album/{id}/tracks does. Keep the embedded list
	// if that call fails.
	if tracks, err := p.getAlbumTracks(ctx, id); err == nil && len(tracks) > 0 {
		album.Tracks.Data = tracks
	} else if err != nil {
		logger.Logtype("debug", 0).Err(err).Int("album_id", id).Msg("deezer album tracks unavailable")
	}

	return convertAlbumToDetails(&album), nil
}

// maxTrackPages bounds pagination of /album/{id}/tracks.
const maxTrackPages = 10

// getAlbumTracks fetches the full tracklist of an album, following Deezer's
// "next" links.
func (p *Provider) getAlbumTracks(ctx context.Context, id int) ([]deezerTrack, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/album/")
	buf.WriteInt(id)
	buf.WriteString("/tracks?limit=500")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var all []deezerTrack

	for range maxTrackPages {
		var page deezerTracksList
		if err := p.MakeRequest(ctx, "GET", endpoint, nil, &page, nil); err != nil {
			return nil, err
		}

		if err := page.Error.toError(); err != nil {
			return nil, err
		}

		all = append(all, page.Data...)

		if page.Next == "" || len(page.Data) == 0 {
			break
		}

		endpoint = page.Next
	}

	return all, nil
}
