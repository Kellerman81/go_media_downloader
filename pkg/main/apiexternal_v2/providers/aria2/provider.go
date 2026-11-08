// Package aria2 adds torrents to aria2 over its JSON-RPC interface.
package aria2

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// DefaultRPCPath is used when the configured URL has no path.
const DefaultRPCPath = "/jsonrpc"

// Provider talks to one aria2 instance.
type Provider struct {
	*base.BaseClient
	secret string
}

// NewProvider returns a provider for aria2 at host:port. secret is the
// --rpc-secret, empty when none is set.
func NewProvider(host string, port int, secret string, useSSL bool, rpcPath string) *Provider {
	scheme := "http://"
	if useSSL {
		scheme = "https://"
	}

	if rpcPath == "" {
		rpcPath = DefaultRPCPath
	}

	cfg := base.ServiceConfig("aria2", scheme+host+":"+strconv.Itoa(port)+rpcPath)

	return &Provider{BaseClient: base.NewBaseClient(cfg), secret: secret}
}

func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderAria2
}

func (*Provider) GetProviderName() string { return "aria2" }

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *Provider) call(ctx context.Context, method string, result any, params ...any) error {
	if p.secret != "" {
		params = append([]any{"token:" + p.secret}, params...)
	}

	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "gmd", "method": method, "params": params})
	if err != nil {
		return err
	}

	// aria2 answers RPC errors with HTTP 400 and the error in the body.
	var resp rpcResponse

	err = p.MakeRequestAcceptStatus(ctx, http.MethodPost, "", bytes.NewReader(body), func(r *http.Response) error {
		return json.NewDecoder(r.Body).Decode(&resp)
	}, nil, http.StatusBadRequest, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return errors.New("aria2: " + resp.Error.Message)
	}

	if result != nil {
		return json.Unmarshal(resp.Result, result)
	}

	return nil
}

// AddTorrent adds a magnet link or torrent URL. aria2 downloads a torrent
// URL and follows it into the torrent (--follow-torrent, on by default).
func (p *Provider) AddTorrent(
	ctx context.Context,
	req apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	options := map[string]string{}
	if req.SavePath != "" {
		options["dir"] = req.SavePath
	}

	if req.Paused {
		options["pause"] = "true"
	}

	var gid string
	if err := p.call(ctx, "aria2.addUri", &gid, []string{req.URL}, options); err != nil {
		return nil, err
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Hash:     base.InfoHashFromURL(req.URL),
		Message:  "gid " + gid,
		Provider: string(apiexternal_v2.DownloadProviderAria2),
	}, nil
}

// TestConnection checks the RPC endpoint and secret with getVersion.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.call(ctx, "aria2.getVersion", nil)
}
