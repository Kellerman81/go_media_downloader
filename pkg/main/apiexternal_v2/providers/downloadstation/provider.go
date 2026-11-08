// Package downloadstation adds torrents to Synology Download Station
// through the DSM Web API.
package downloadstation

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

const (
	apiAuth   = "SYNO.API.Auth"
	apiTask2  = "SYNO.DownloadStation2.Task" // DSM 7
	apiTask1  = "SYNO.DownloadStation.Task"  // DSM 6
	authMaxV  = 6
	errNoAuth = 119 // "SID not found": the session expired
)

var errNoTaskAPI = errors.New("download station: no task API found - is Download Station installed?")

// Provider talks to one DiskStation.
type Provider struct {
	*base.BaseClient
	username, password string

	mu   sync.Mutex
	apis map[string]apiInfo
	sid  string
}

type apiInfo struct {
	Path       string `json:"path"`
	MaxVersion int    `json:"maxVersion"`
}

type apiResponse[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
	Error   struct {
		Code int `json:"code"`
	} `json:"error"`
}

type apiError int

func (e apiError) Error() string { return "download station: API error " + strconv.Itoa(int(e)) }

// NewProvider returns a provider for the DiskStation at baseURL (e.g.
// "http://nas:5000") and a DSM account allowed to use Download Station.
func NewProvider(baseURL, username, password string) *Provider {
	cfg := base.ServiceConfig("downloadstation", strings.TrimRight(baseURL, "/")+"/webapi")

	return &Provider{BaseClient: base.NewBaseClient(cfg), username: username, password: password}
}

func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderStation
}

func (*Provider) GetProviderName() string { return "downloadstation" }

func get[T any](ctx context.Context, p *Provider, path string, q url.Values) (T, error) {
	var resp apiResponse[T]
	if err := p.MakeRequest(ctx, http.MethodGet, "/"+path+"?"+q.Encode(), nil, &resp, nil); err != nil {
		return resp.Data, err
	}

	if !resp.Success {
		return resp.Data, apiError(resp.Error.Code)
	}

	return resp.Data, nil
}

// session returns the API table and a session ID, logging in when needed.
func (p *Provider) session(ctx context.Context) (map[string]apiInfo, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.apis == nil {
		apis, err := get[map[string]apiInfo](ctx, p, "query.cgi", url.Values{
			"api": {"SYNO.API.Info"}, "version": {"1"}, "method": {"query"},
			"query": {apiAuth + "," + apiTask2 + "," + apiTask1},
		})
		if err != nil {
			return nil, "", err
		}

		p.apis = apis
	}

	if p.sid == "" {
		auth, ok := p.apis[apiAuth]
		if !ok {
			return nil, "", apiError(102)
		}

		data, err := get[struct {
			SID string `json:"sid"`
		}](ctx, p, auth.Path, url.Values{
			"api": {apiAuth}, "version": {strconv.Itoa(min(auth.MaxVersion, authMaxV))}, "method": {"login"},
			"account": {p.username}, "passwd": {p.password}, "session": {"DownloadStation"}, "format": {"sid"},
		})
		if err != nil {
			return nil, "", err
		}

		p.sid = data.SID
	}

	return p.apis, p.sid, nil
}

func (p *Provider) dropSession() {
	p.mu.Lock()
	p.sid = ""
	p.mu.Unlock()
}

func (p *Provider) create(ctx context.Context, rawURL, destination string) error {
	apis, sid, err := p.session(ctx)
	if err != nil {
		return err
	}

	q := url.Values{"method": {"create"}, "_sid": {sid}}

	var path string

	if task, ok := apis[apiTask2]; ok {
		path = task.Path
		q.Set("api", apiTask2)
		q.Set("version", "2")
		q.Set("type", "url")
		q.Set("url", rawURL)
		q.Set("create_list", "false")
	} else if task, ok := apis[apiTask1]; ok {
		path = task.Path
		q.Set("api", apiTask1)
		q.Set("version", "1")
		q.Set("uri", rawURL)
	} else {
		return errNoTaskAPI
	}

	if destination != "" {
		q.Set("destination", destination)
	}

	_, err = get[any](ctx, p, path, q)

	return err
}

// AddTorrent adds a magnet link or torrent URL. destination is a shared
// folder path ("downloads/movies"); empty uses Download Station's default.
func (p *Provider) AddTorrent(
	ctx context.Context,
	req apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	err := p.create(ctx, req.URL, req.SavePath)
	if errors.Is(err, apiError(errNoAuth)) {
		p.dropSession()
		err = p.create(ctx, req.URL, req.SavePath)
	}

	if err != nil {
		return nil, err
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Hash:     base.InfoHashFromURL(req.URL),
		Provider: string(apiexternal_v2.DownloadProviderStation),
	}, nil
}

// TestConnection logs in and checks that a task API exists.
func (p *Provider) TestConnection(ctx context.Context) error {
	p.dropSession()

	apis, _, err := p.session(ctx)
	if err != nil {
		return err
	}

	if _, ok := apis[apiTask2]; ok {
		return nil
	}

	if _, ok := apis[apiTask1]; ok {
		return nil
	}

	return errNoTaskAPI
}
