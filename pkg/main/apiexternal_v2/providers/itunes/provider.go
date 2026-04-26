package itunes

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// iTunes Search API Provider - Music metadata (completely free, no auth required)
// API: https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI
// Rate limit: ~20 requests/min recommended; no official limit published
//

const defaultBaseURL = "https://itunes.apple.com"

// Provider implements the music metadata provider for the iTunes Search API.
type Provider struct {
	*base.BaseClient
}

// NewProvider creates a new iTunes provider using rate limits from the application config.
func NewProvider() *Provider {
	cfg := config.GetSettingsGeneral()
	rateSec := cfg.ITunesLimiterSeconds
	rateCalls := cfg.ITunesLimiterCalls

	if rateSec == 0 {
		rateSec = 60
	}

	if rateCalls == 0 {
		rateCalls = 20
	}

	return NewProviderWithConfig(base.ClientConfig{
		RateLimitSeconds: int(rateSec),
		RateLimitCalls:   rateCalls,
	})
}

// NewProviderWithConfig creates a new iTunes provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig) *Provider {
	cfg.Name = "itunes"
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 20
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 60
	}

	return &Provider{BaseClient: base.NewBaseClient(cfg)}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderITunes
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "itunes"
}

// SearchAlbums searches for albums by artist and album name.
// Returns up to limit results.
func (p *Provider) SearchAlbums(
	ctx context.Context,
	artist, album string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/search?term=")
	buf.WriteURL(artist)
	buf.WriteString(" ")
	buf.WriteURL(album)
	buf.WriteString("&entity=album")
	buf.WriteString("&limit=")
	buf.WriteInt(limit)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var resp itunesSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertSearchToReleases(resp.Results), nil
}

// GetAlbumTracks fetches the full track listing for a given iTunes collection ID.
func (p *Provider) GetAlbumTracks(
	ctx context.Context,
	collectionID int,
) (*apiexternal_v2.ReleaseDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/lookup?id=")
	buf.WriteInt(collectionID)
	buf.WriteString("&entity=song")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var resp itunesLookupResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertLookupToDetails(resp.Results), nil
}
