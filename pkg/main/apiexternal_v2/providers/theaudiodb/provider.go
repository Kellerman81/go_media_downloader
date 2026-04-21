package theaudiodb

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// TheAudioDB Provider - Music metadata (free public API)
// API: https://www.theaudiodb.com/api.php
// Free key: "2" (limited to single-artist lookups)
// Rate limit: conservative 2 req/sec to avoid hammering the free tier
//

const defaultBaseURL = "https://www.theaudiodb.com/api/v1/json"

// Provider implements the music metadata provider for TheAudioDB.
type Provider struct {
	*base.BaseClient
	apiKey string
}

// NewProvider creates a new TheAudioDB provider using rate limits from the
// application config. If no API key is configured the free public key ("123")
// is used automatically.
func NewProvider() *Provider {
	cfg := config.GetSettingsGeneral()
	apiKey := cfg.TheAudioDBAPIKey
	rateSec := cfg.TheAudioDBLimiterSeconds
	rateCalls := cfg.TheAudioDBLimiterCalls
	if rateSec == 0 {
		rateSec = 1
	}
	if rateCalls == 0 {
		rateCalls = 2
	}
	return NewProviderWithConfig(base.ClientConfig{
		RateLimitSeconds: int(rateSec),
		RateLimitCalls:   rateCalls,
	}, apiKey)
}

// NewProviderWithConfig creates a new TheAudioDB provider with custom config.
// apiKey may be empty — the free public key "123" is used in that case.
func NewProviderWithConfig(cfg base.ClientConfig, apiKey string) *Provider {
	if apiKey == "" {
		apiKey = "123"
	}
	cfg.Name = "theaudiodb"
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 2
	}
	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 1
	}
	return &Provider{BaseClient: base.NewBaseClient(cfg), apiKey: apiKey}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderTheAudioDB
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "theaudiodb"
}

// SearchAlbums searches for albums by artist name and album title.
// TheAudioDB does not have a generic album search — it requires both artist
// and album name to narrow results reliably.
func (p *Provider) SearchAlbums(
	ctx context.Context,
	artist, album string,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/searchalbum.php?s=")
	buf.WriteURL(artist)
	buf.WriteString("&a=")
	buf.WriteURL(album)
	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var resp tadSearchAlbumResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertAlbumSearchToReleases(resp.Album), nil
}

// GetTracksByAlbumID fetches all tracks for the given TheAudioDB album ID.
func (p *Provider) GetTracksByAlbumID(
	ctx context.Context,
	albumID string,
) (*apiexternal_v2.ReleaseDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/")
	buf.WriteURL(p.apiKey)
	buf.WriteString("/track.php?m=")
	buf.WriteURL(albumID)
	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var resp tadTrackListResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	if len(resp.Track) == 0 {
		return nil, nil
	}

	return convertTracksToDetails(albumID, resp.Track), nil
}
