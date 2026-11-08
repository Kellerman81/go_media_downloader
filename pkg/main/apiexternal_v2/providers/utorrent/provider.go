// Package utorrent adds torrents to uTorrent through its Web UI API.
package utorrent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

var (
	tokenRe     = regexp.MustCompile(`<div[^>]*id=['"]token['"][^>]*>([^<]+)</div>`)
	errNoToken  = errors.New("utorrent: no token in token.html")
	anyAccepted = map[string]string{"Accept": "*/*"}
)

// Provider talks to one uTorrent Web UI. The token is bound to the GUID
// cookie kept in the client's cookie jar.
type Provider struct {
	*base.BaseClient
}

// NewProvider returns a provider for the Web UI at baseURL (e.g.
// "http://host:8080"); "/gui" is appended.
func NewProvider(baseURL, username, password string) (*Provider, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	cfg := base.ServiceConfig("utorrent", strings.TrimRight(baseURL, "/")+"/gui")
	cfg.AuthType, cfg.Username, cfg.Password = base.AuthBasic, username, password

	bc := base.NewBaseClient(cfg)
	bc.GetHTTPClient().Jar = jar

	return &Provider{BaseClient: bc}, nil
}

func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderUTorrent
}

func (*Provider) GetProviderName() string { return "utorrent" }

func (p *Provider) token(ctx context.Context) (string, error) {
	var token string

	err := p.MakeRequestWithHeaders(ctx, http.MethodGet, "/token.html", nil, nil, func(r *http.Response) error {
		b, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		if err != nil {
			return err
		}

		m := tokenRe.FindSubmatch(b)
		if m == nil {
			return errNoToken
		}

		token = string(m[1])

		return nil
	}, anyAccepted)

	return token, err
}

func (p *Provider) action(ctx context.Context, token string, q url.Values) error {
	q.Set("token", token)

	return p.MakeRequestWithHeaders(ctx, http.MethodGet, "/?"+q.Encode(), nil, nil, nil, anyAccepted)
}

// hashes returns the info hashes of all torrents in the client.
func (p *Provider) hashes(ctx context.Context, token string) (map[string]bool, error) {
	var list struct {
		Torrents [][]any `json:"torrents"`
	}

	err := p.MakeRequestWithHeaders(ctx, http.MethodGet, "/?"+url.Values{"list": {"1"}, "token": {token}}.Encode(),
		nil, &list, nil, anyAccepted)
	if err != nil {
		return nil, err
	}

	out := make(map[string]bool, len(list.Torrents))
	for _, t := range list.Torrents {
		if len(t) > 0 {
			if h, ok := t[0].(string); ok {
				out[strings.ToUpper(h)] = true
			}
		}
	}

	return out, nil
}

// newHash waits up to addWait for a torrent that is not in before, as
// uTorrent fetches a torrent URL in the background after add-url returns.
func (p *Provider) newHash(ctx context.Context, token string, before map[string]bool) string {
	deadline := time.Now().Add(addWait)
	for time.Now().Before(deadline) {
		now, err := p.hashes(ctx, token)
		if err != nil {
			return ""
		}

		for h := range now {
			if !before[h] {
				return h
			}
		}

		select {
		case <-ctx.Done():
			return ""
		case <-time.After(500 * time.Millisecond):
		}
	}

	return ""
}

// addWait bounds how long AddTorrent looks for the hash of a torrent URL.
const addWait = 15 * time.Second

// AddTorrent adds a magnet link or torrent URL. The Web UI returns no hash;
// a magnet's comes from the link, a torrent URL's from comparing the
// torrent list before and after, so label and pause can be applied.
func (p *Provider) AddTorrent(
	ctx context.Context,
	req apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	token, err := p.token(ctx)
	if err != nil {
		return nil, err
	}

	hash := base.InfoHashFromURL(req.URL)

	var before map[string]bool
	if hash == "" && (req.Category != "" || req.Paused) {
		if before, err = p.hashes(ctx, token); err != nil {
			return nil, err
		}
	}

	if err := p.action(ctx, token, url.Values{"action": {"add-url"}, "s": {req.URL}}); err != nil {
		return nil, err
	}

	if before != nil {
		hash = p.newHash(ctx, token, before)
	}
	if hash != "" && req.Category != "" {
		if err := p.action(ctx, token, url.Values{
			"action": {"setprops"}, "hash": {hash}, "s": {"label"}, "v": {req.Category},
		}); err != nil {
			return nil, err
		}
	}

	if hash != "" && req.Paused {
		if err := p.action(ctx, token, url.Values{"action": {"stop"}, "hash": {hash}}); err != nil {
			return nil, err
		}
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Hash:     hash,
		Provider: string(apiexternal_v2.DownloadProviderUTorrent),
	}, nil
}

// TestConnection fetches a token, which needs valid credentials.
func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.token(ctx)

	return err
}
