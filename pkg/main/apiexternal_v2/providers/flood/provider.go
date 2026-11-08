// Package flood adds torrents to Flood (https://flood.js.org), whichever
// torrent client it fronts.
package flood

import (
	"bytes"
	"context"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// Provider talks to one Flood instance; its session cookie lives in the
// client's cookie jar.
type Provider struct {
	*base.BaseClient
	username, password string
}

// NewProvider returns a provider for Flood at baseURL (e.g.
// "http://host:3000", sub-path included).
func NewProvider(baseURL, username, password string) (*Provider, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	bc := base.NewBaseClient(base.ServiceConfig("flood", strings.TrimRight(baseURL, "/")+"/api"))
	bc.GetHTTPClient().Jar = jar

	return &Provider{BaseClient: bc, username: username, password: password}, nil
}

func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderFlood
}

func (*Provider) GetProviderName() string { return "flood" }

func (p *Provider) post(ctx context.Context, endpoint string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return p.MakeRequestWithHeaders(ctx, http.MethodPost, endpoint, bytes.NewReader(body), nil, nil,
		map[string]string{"Accept": "*/*"})
}

// login (re)creates the session cookie. Adds are rare, so it runs before
// each one instead of tracking the session's expiry.
func (p *Provider) login(ctx context.Context) error {
	return p.post(ctx, "/auth/authenticate", map[string]string{"username": p.username, "password": p.password})
}

// AddTorrent adds a magnet link or torrent URL. The category becomes a tag.
func (p *Provider) AddTorrent(
	ctx context.Context,
	req apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	if err := p.login(ctx); err != nil {
		return nil, err
	}

	payload := map[string]any{"urls": []string{req.URL}, "start": !req.Paused}
	if req.SavePath != "" {
		payload["destination"] = req.SavePath
	}

	if req.Category != "" {
		payload["tags"] = []string{req.Category}
	}

	if err := p.post(ctx, "/torrents/add-urls", payload); err != nil {
		return nil, err
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Hash:     base.InfoHashFromURL(req.URL),
		Provider: string(apiexternal_v2.DownloadProviderFlood),
	}, nil
}

// TestConnection logs in.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.login(ctx)
}
