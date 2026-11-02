package transmission

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

//
// Transmission Provider - Transmission Torrent Client
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the DownloadProvider interface for Transmission.
type Provider struct {
	*base.BaseClient
	baseURL  string
	username string
	password string
	// sessionMu guards sessionID - Provider instances are cached singletons
	// in providers/registry.go, reused across every concurrent operation
	// against a given Transmission instance for the app's lifetime, so this
	// field is read/written from multiple goroutines.
	sessionMu sync.Mutex
	sessionID string
}

// NewProvider creates a new Transmission download provider.
func NewProvider(baseURL, username, password string) *Provider {
	config := base.ClientConfig{
		Name:                    "transmission",
		BaseURL:                 baseURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthBasic,
		Username:                username,
		Password:                password,
		RateLimitCalls:          1000, // 1000 calls per hour
		RateLimitSeconds:        3600, // 1 hour
		RateLimitPer24h:         10000,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		baseURL:    rpcURL(baseURL),
		username:   username,
		password:   password,
	}
}

// rpcURL appends Transmission's RPC path to baseURL unless it already ends with it.
func rpcURL(baseURL string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(baseURL, "/transmission/rpc") {
		return baseURL
	}

	return baseURL + "/transmission/rpc"
}

// GetProviderType returns the provider type.
func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderTransmission
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "transmission"
}

// bandwidthPriority maps the downloader priority scale (-2 very low .. 2 very
// high) onto Transmission's bandwidthPriority, which only knows -1/0/1.
func bandwidthPriority(priority int) int {
	switch {
	case priority < 0:
		return -1
	case priority > 0:
		return 1
	default:
		return 0
	}
}

// AddTorrent adds a torrent for downloading.
func (p *Provider) AddTorrent(
	ctx context.Context,
	request apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	args := map[string]any{}

	// An empty download-dir is not "use the default" for Transmission - it is
	// taken literally, so only send it when a path is configured.
	if request.SavePath != "" {
		args["download-dir"] = request.SavePath
	}

	// metainfo is the base64-encoded content of a .torrent file.
	if len(request.TorrentData) > 0 {
		args["metainfo"] = base64.StdEncoding.EncodeToString(request.TorrentData)
	} else {
		args["filename"] = request.URL
	}

	if request.Paused {
		args["paused"] = true
	}

	if bp := bandwidthPriority(request.Priority); bp != 0 {
		args["bandwidthPriority"] = bp
	}

	response, err := p.makeRPCCall(ctx, "torrent-add", args)
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, errors.New(logger.JoinStrings("failed to add torrent: ", response.Result))
	}

	// A new torrent comes back under "torrent-added"; one already in the
	// session under "torrent-duplicate". Both mean the torrent is present, so
	// both report success with the torrent's hash.
	hash := ""
	message := ""

	if args, ok := response.Arguments.(map[string]any); ok {
		if addedData, ok := args["torrent-added"].(map[string]any); ok {
			hash, _ = addedData["hashString"].(string)
		} else if dupData, ok := args["torrent-duplicate"].(map[string]any); ok {
			hash, _ = dupData["hashString"].(string)
			message = "torrent already added"
		}
	}

	return &apiexternal_v2.TorrentAddResponse{
		Hash:     hash,
		Success:  true,
		Message:  message,
		Provider: "transmission",
	}, nil
}

// GetTorrentInfo retrieves information about a specific torrent.
func (p *Provider) GetTorrentInfo(
	ctx context.Context,
	hash string,
) (*apiexternal_v2.TorrentInfo, error) {
	fields := []string{
		"id",
		"name",
		"hashString",
		"status",
		"percentDone",
		"totalSize",
		"downloadedEver",
		"uploadedEver",
		"rateDownload",
		"rateUpload",
		"eta",
		"downloadDir",
		"addedDate",
	}

	args := map[string]any{
		"fields": fields,
		"ids":    []string{hash},
	}

	response, err := p.makeRPCCall(ctx, "torrent-get", args)
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, errors.New(logger.JoinStrings("failed to get torrent: ", response.Result))
	}

	arguments, ok := response.Arguments.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected response format")
	}

	torrentsData, ok := arguments["torrents"].([]any)
	if !ok || len(torrentsData) == 0 {
		return nil, errors.New("torrent not found")
	}

	if torrentMap, ok := torrentsData[0].(map[string]any); ok {
		return parseTorrentInfo(torrentMap), nil
	}

	return nil, errors.New("invalid torrent data")
}

// ListTorrents retrieves the list of active torrents.
func (p *Provider) ListTorrents(
	ctx context.Context,
	filter string,
) (*apiexternal_v2.TorrentListResponse, error) {
	fields := []string{
		"id",
		"name",
		"hashString",
		"status",
		"percentDone",
		"totalSize",
		"downloadedEver",
		"uploadedEver",
		"rateDownload",
		"rateUpload",
		"eta",
		"downloadDir",
		"addedDate",
	}

	args := map[string]any{
		"fields": fields,
	}

	response, err := p.makeRPCCall(ctx, "torrent-get", args)
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, errors.New(logger.JoinStrings("failed to list torrents: ", response.Result))
	}

	arguments, ok := response.Arguments.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected response format")
	}

	torrentsData, ok := arguments["torrents"].([]any)
	if !ok {
		torrentsData = []any{}
	}

	torrents := make([]apiexternal_v2.TorrentInfo, 0, len(torrentsData))
	for _, data := range torrentsData {
		torrentMap, ok := data.(map[string]any)
		if !ok {
			continue
		}

		torrent := parseTorrentInfo(torrentMap)
		// Apply filter if specified
		if filter == "" ||
			logger.ContainsI(torrent.State, filter) {
			torrents = append(torrents, *torrent)
		}
	}

	return &apiexternal_v2.TorrentListResponse{
		Torrents: torrents,
		Total:    len(torrents),
	}, nil
}

// PauseTorrent pauses a torrent.
func (p *Provider) PauseTorrent(ctx context.Context, hash string) error {
	response, err := p.makeRPCCall(ctx, "torrent-stop", map[string]any{"ids": []string{hash}})
	if err != nil {
		return err
	}

	if response.Result != "success" {
		return errors.New(logger.JoinStrings("failed to pause torrent: ", response.Result))
	}

	return nil
}

// ResumeTorrent resumes a torrent.
func (p *Provider) ResumeTorrent(ctx context.Context, hash string) error {
	response, err := p.makeRPCCall(ctx, "torrent-start", map[string]any{"ids": []string{hash}})
	if err != nil {
		return err
	}

	if response.Result != "success" {
		return errors.New(logger.JoinStrings("failed to resume torrent: ", response.Result))
	}

	return nil
}

// RemoveTorrent removes a torrent.
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	args := map[string]any{
		"ids":               []string{hash},
		"delete-local-data": deleteFiles,
	}

	response, err := p.makeRPCCall(ctx, "torrent-remove", args)
	if err != nil {
		return err
	}

	if response.Result != "success" {
		return errors.New(logger.JoinStrings("failed to remove torrent: ", response.Result))
	}

	return nil
}

// GetStatus retrieves the client status.
func (p *Provider) GetStatus(ctx context.Context) (*apiexternal_v2.DownloadClientStatus, error) {
	response, err := p.makeRPCCall(ctx, "session-stats", nil)
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, errors.New(logger.JoinStrings("failed to get status: ", response.Result))
	}

	status := &apiexternal_v2.DownloadClientStatus{
		Connected: true,
		Version:   "Transmission",
		Provider:  "transmission",
	}

	if args, ok := response.Arguments.(map[string]any); ok {
		if upSpeed, ok := args["uploadSpeed"].(float64); ok {
			status.TotalUpload = int64(upSpeed)
		}

		if downSpeed, ok := args["downloadSpeed"].(float64); ok {
			status.TotalDownload = int64(downSpeed)
		}
	}

	return status, nil
}

// TestConnection validates connectivity.
func (p *Provider) TestConnection(ctx context.Context) error {
	response, err := p.makeRPCCall(ctx, "session-get", nil)
	if err != nil {
		return err
	}

	if response.Result != "success" {
		return errors.New(logger.JoinStrings("test failed: ", response.Result))
	}

	return nil
}

// Helper methods

// loadSessionID returns the current session ID under sessionMu.
func (p *Provider) loadSessionID() string {
	p.sessionMu.Lock()
	defer p.sessionMu.Unlock()

	return p.sessionID
}

// storeSessionID sets the session ID under sessionMu.
func (p *Provider) storeSessionID(id string) {
	p.sessionMu.Lock()
	defer p.sessionMu.Unlock()

	p.sessionID = id
}

// makeRPCCall sends one Transmission RPC request. Transmission answers a
// request without a valid X-Transmission-Session-Id with HTTP 409 carrying the
// current id in that header; the id is stored and the request is sent once
// more. A second 409 means the server keeps rotating the id (or a proxy strips
// the header), so it is reported instead of looping.
func (p *Provider) makeRPCCall(
	ctx context.Context,
	method string,
	args any,
) (*transmissionRPCResponse, error) {
	jsonData, err := json.Marshal(transmissionRPCRequest{
		Method:    method,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	for range 2 {
		response, conflict, err := p.sendRPC(ctx, jsonData)
		if err != nil {
			return nil, err
		}

		if !conflict {
			return response, nil
		}
	}

	return nil, errors.New(
		"transmission kept answering HTTP 409: session id was not accepted after refresh",
	)
}

// sendRPC posts one RPC body. conflict reports an HTTP 409 whose new session
// id has been stored, meaning the request has to be re-sent.
func (p *Provider) sendRPC(
	ctx context.Context,
	jsonData []byte,
) (response *transmissionRPCResponse, conflict bool, err error) {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	if sessionID := p.loadSessionID(); sessionID != "" {
		headers["X-Transmission-Session-Id"] = sessionID
	}

	if p.username != "" && p.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))

		headers["Authorization"] = "Basic " + auth
	}

	var rpcResponse transmissionRPCResponse

	err = p.MakeRequestAcceptStatus(
		ctx,
		http.MethodPost,
		p.baseURL,
		bytes.NewReader(jsonData),
		func(resp *http.Response) error {
			if resp.StatusCode == http.StatusConflict {
				sessionID := resp.Header.Get("X-Transmission-Session-Id")
				if sessionID == "" {
					return errors.New("transmission answered HTTP 409 without a session id")
				}

				p.storeSessionID(sessionID)

				conflict = true

				return nil
			}

			if resp.StatusCode != http.StatusOK {
				return errors.New(logger.JoinStrings("HTTP ", strconv.Itoa(resp.StatusCode)))
			}

			return json.NewDecoder(resp.Body).Decode(&rpcResponse)
		},
		headers,
		http.StatusConflict,
	)
	if err != nil || conflict {
		return nil, conflict, err
	}

	return &rpcResponse, false, nil
}

//
// Internal types
//

type transmissionRPCRequest struct {
	Method    string `json:"method"`
	Arguments any    `json:"arguments,omitempty"`
}

type transmissionRPCResponse struct {
	Arguments any    `json:"arguments,omitempty"`
	Result    string `json:"result"`
}

//
// Helper functions
//

func parseTorrentInfo(data map[string]any) *apiexternal_v2.TorrentInfo {
	info := &apiexternal_v2.TorrentInfo{}

	if hash, ok := data["hashString"].(string); ok {
		info.Hash = hash
	}

	if name, ok := data["name"].(string); ok {
		info.Name = name
	}

	if status, ok := data["status"].(float64); ok {
		info.State = getStatusString(int(status))
	}

	if progress, ok := data["percentDone"].(float64); ok {
		info.Progress = progress * 100
	}

	if size, ok := data["totalSize"].(float64); ok {
		info.Size = int64(size)
	}

	if downloaded, ok := data["downloadedEver"].(float64); ok {
		info.Downloaded = int64(downloaded)
	}

	if uploaded, ok := data["uploadedEver"].(float64); ok {
		info.Uploaded = int64(uploaded)
	}

	if downSpeed, ok := data["rateDownload"].(float64); ok {
		info.DownloadSpeed = int64(downSpeed)
	}

	if upSpeed, ok := data["rateUpload"].(float64); ok {
		info.UploadSpeed = int64(upSpeed)
	}

	if eta, ok := data["eta"].(float64); ok && eta >= 0 {
		info.ETA = int(eta)
	}

	if savePath, ok := data["downloadDir"].(string); ok {
		info.SavePath = savePath
	}

	if addedTime, ok := data["addedDate"].(float64); ok {
		info.AddedDate = time.Unix(int64(addedTime), 0)
	}

	return info
}

func getStatusString(status int) string {
	switch status {
	case 0:
		return "stopped"
	case 1:
		return "check_waiting"
	case 2:
		return "checking"
	case 3:
		return "download_waiting"
	case 4:
		return "downloading"
	case 5:
		return "seed_waiting"
	case 6:
		return "seeding"
	default:
		return "unknown"
	}
}
