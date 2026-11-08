// Package tribler adds torrents to Tribler over its REST API.
package tribler

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// AnonymityHops is the number of proxy hops a download runs through, the
// default of Tribler and Sonarr.
const AnonymityHops = 1

// Provider talks to one Tribler instance.
type Provider struct {
	*base.BaseClient
}

// NewProvider returns a provider for Tribler at baseURL (e.g.
// "http://host:20100") with the API key from Tribler's settings.
func NewProvider(baseURL, apiKey string) *Provider {
	cfg := base.ServiceConfig("tribler", strings.TrimRight(baseURL, "/")+"/api")
	cfg.AuthType, cfg.APIKeyHeader, cfg.APIKey = base.AuthAPIKeyHeader, "X-Api-Key", apiKey

	return &Provider{BaseClient: base.NewBaseClient(cfg)}
}

func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderTribler
}

func (*Provider) GetProviderName() string { return "tribler" }

type addRequest struct {
	URI         string `json:"uri"`
	Destination string `json:"destination,omitempty"`
	AnonHops    int    `json:"anon_hops"`
	SafeSeeding bool   `json:"safe_seeding"`
}

// AddTorrent adds a magnet link or torrent URL with safe seeding on.
func (p *Provider) AddTorrent(
	ctx context.Context,
	req apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	body, err := json.Marshal(addRequest{URI: req.URL, Destination: req.SavePath, AnonHops: AnonymityHops, SafeSeeding: true})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Infohash string `json:"infohash"`
	}

	if err := p.MakeRequest(ctx, http.MethodPut, "/downloads", bytes.NewReader(body), &resp, nil); err != nil {
		return nil, err
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Hash:     strings.ToUpper(resp.Infohash),
		Provider: string(apiexternal_v2.DownloadProviderTribler),
	}, nil
}

// TestConnection reads the settings, which needs a valid API key.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.MakeRequest(ctx, http.MethodGet, "/settings", nil, nil, nil)
}
