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

	return convertAlbumToDetails(&album), nil
}
