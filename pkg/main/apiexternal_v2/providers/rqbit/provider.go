// Package rqbit adds torrents to rqbit (https://github.com/ikatson/rqbit)
// over its HTTP API.
package rqbit

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

// Provider talks to one rqbit instance.
type Provider struct {
	*base.BaseClient
}

// NewProvider returns a provider for rqbit at baseURL (e.g.
// "http://host:3030"). Username and password are only needed when rqbit
// runs with RQBIT_HTTP_BASIC_AUTH_USERPASS.
func NewProvider(baseURL, username, password string) *Provider {
	cfg := base.ServiceConfig("rqbit", strings.TrimRight(baseURL, "/"))
	// A magnet add only returns once rqbit has the metadata from peers.
	cfg.Timeout = 2 * time.Minute
	if username != "" {
		cfg.AuthType, cfg.Username, cfg.Password = base.AuthBasic, username, password
	}

	return &Provider{BaseClient: base.NewBaseClient(cfg)}
}

func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderRQBit
}

func (*Provider) GetProviderName() string { return "rqbit" }

// AddTorrent adds a magnet link or torrent URL, sent as the request body.
// rqbit has no paused add; a paused torrent is paused right after.
func (p *Provider) AddTorrent(
	ctx context.Context,
	req apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	q := url.Values{"overwrite": {"true"}, "is_url": {"true"}}
	if req.SavePath != "" {
		q.Set("output_folder", req.SavePath)
	}

	var resp struct {
		Details struct {
			InfoHash string `json:"info_hash"`
			Name     string `json:"name"`
		} `json:"details"`
	}

	err := p.MakeRequestWithHeaders(ctx, http.MethodPost, "/torrents?"+q.Encode(), strings.NewReader(req.URL), &resp, nil,
		map[string]string{"Content-Type": "text/plain"})
	if err != nil {
		return nil, err
	}

	if req.Paused && resp.Details.InfoHash != "" {
		err := p.MakeRequest(ctx, http.MethodPost, "/torrents/"+resp.Details.InfoHash+"/pause", nil, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("rqbit: added but not paused: %w", err)
		}
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Hash:     strings.ToUpper(resp.Details.InfoHash),
		Name:     resp.Details.Name,
		Provider: string(apiexternal_v2.DownloadProviderRQBit),
	}, nil
}

// TestConnection reads the API root.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.MakeRequest(ctx, http.MethodGet, "/", nil, nil, nil)
}
