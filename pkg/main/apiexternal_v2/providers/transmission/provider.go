package transmission

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Transmission Provider - Transmission Torrent Client
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the DownloadProvider interface for Transmission.
type Provider struct {
	*base.BaseClient
	baseURL   string
	username  string
	password  string
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
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		baseURL:    baseURL + "/transmission/rpc",
		username:   username,
		password:   password,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderTransmission
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "transmission"
}

// AddTorrent adds a torrent for downloading.
func (p *Provider) AddTorrent(
	ctx context.Context,
	request apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	args := map[string]any{
		"download-dir": request.SavePath,
	}

	// Determine if we're adding by URL or raw data
	if len(request.TorrentData) > 0 {
		args["metainfo"] = string(request.TorrentData)
	} else {
		args["filename"] = request.URL
	}

	if request.Paused {
		args["paused"] = true
	}

	if request.Priority > 0 {
		args["bandwidthPriority"] = request.Priority
	}

	response, err := p.makeRPCCall(ctx, "torrent-add", args)
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("failed to add torrent: %s", response.Result)
	}

	// Extract hash from response
	hash := ""
	if args, ok := response.Arguments.(map[string]any); ok {
		if addedData, ok := args["torrent-added"].(map[string]any); ok {
			if hashStr, ok := addedData["hashString"].(string); ok {
				hash = hashStr
			}
		}
	}

	return &apiexternal_v2.TorrentAddResponse{
		Hash:    hash,
		Success: true,
	}, nil
}

// GetTorrentInfo retrieves information about a specific torrent.
func (p *Provider) GetTorrentInfo(
	ctx context.Context,
	hash string,
) (*apiexternal_v2.TorrentInfo, error) {
	fields := []string{
		"id", "name", "hashString", "status", "percentDone", "totalSize",
		"downloadedEver", "uploadedEver", "rateDownload", "rateUpload", "eta", "downloadDir", "addedDate",
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
		return nil, fmt.Errorf("failed to get torrent: %s", response.Result)
	}

	arguments, ok := response.Arguments.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	torrentsData, ok := arguments["torrents"].([]any)
	if !ok || len(torrentsData) == 0 {
		return nil, fmt.Errorf("torrent not found")
	}

	if torrentMap, ok := torrentsData[0].(map[string]any); ok {
		return parseTorrentInfo(torrentMap), nil
	}

	return nil, fmt.Errorf("invalid torrent data")
}

// ListTorrents retrieves the list of active torrents.
func (p *Provider) ListTorrents(
	ctx context.Context,
	filter string,
) (*apiexternal_v2.TorrentListResponse, error) {
	fields := []string{
		"id", "name", "hashString", "status", "percentDone", "totalSize",
		"downloadedEver", "uploadedEver", "rateDownload", "rateUpload", "eta", "downloadDir", "addedDate",
	}

	args := map[string]any{
		"fields": fields,
	}

	response, err := p.makeRPCCall(ctx, "torrent-get", args)
	if err != nil {
		return nil, err
	}

	if response.Result != "success" {
		return nil, fmt.Errorf("failed to list torrents: %s", response.Result)
	}

	arguments, ok := response.Arguments.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	torrentsData, ok := arguments["torrents"].([]any)
	if !ok {
		torrentsData = []any{}
	}

	torrents := make([]apiexternal_v2.TorrentInfo, 0, len(torrentsData))
	for _, data := range torrentsData {
		if torrentMap, ok := data.(map[string]any); ok {
			torrent := parseTorrentInfo(torrentMap)
			// Apply filter if specified
			if filter == "" ||
				strings.Contains(strings.ToLower(torrent.State), strings.ToLower(filter)) {
				torrents = append(torrents, *torrent)
			}
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
		return fmt.Errorf("failed to pause torrent: %s", response.Result)
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
		return fmt.Errorf("failed to resume torrent: %s", response.Result)
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
		return fmt.Errorf("failed to remove torrent: %s", response.Result)
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
		return nil, fmt.Errorf("failed to get status: %s", response.Result)
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
		return fmt.Errorf("test failed: %s", response.Result)
	}

	return nil
}

// Helper methods

func (p *Provider) makeRPCCall(
	ctx context.Context,
	method string,
	args any,
) (*transmissionRPCResponse, error) {
	// Get session ID if needed
	if p.sessionID == "" {
		if err := p.getSessionID(ctx); err != nil {
			return nil, err
		}
	}

	request := transmissionRPCRequest{
		Method:    method,
		Arguments: args,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	if p.sessionID != "" {
		headers["X-Transmission-Session-Id"] = p.sessionID
	}

	if p.username != "" && p.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))

		headers["Authorization"] = "Basic " + auth
	}

	var rpcResponse transmissionRPCResponse

	err = p.MakeRequestWithHeaders(
		ctx,
		"POST",
		p.baseURL,
		strings.NewReader(string(jsonData)),
		nil,
		func(resp *http.Response) error {
			// Handle CSRF - retry with new session ID
			if resp.StatusCode == http.StatusConflict {
				if sessionID := resp.Header.Get("X-Transmission-Session-Id"); sessionID != "" {
					p.sessionID = sessionID
					// Don't decode response, will retry
					return fmt.Errorf("CSRF retry needed")
				}
			}

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("HTTP %d", resp.StatusCode)
			}

			if decodeErr := json.NewDecoder(resp.Body).Decode(&rpcResponse); decodeErr != nil {
				return decodeErr
			}

			return nil
		},
		headers,
	)

	// Handle CSRF retry
	if err != nil && err.Error() == "CSRF retry needed" {
		return p.makeRPCCall(ctx, method, args)
	}

	if err != nil {
		return nil, err
	}

	return &rpcResponse, nil
}

func (p *Provider) getSessionID(ctx context.Context) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	if p.username != "" && p.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))

		headers["Authorization"] = "Basic " + auth
	}

	err := p.MakeRequestWithHeaders(
		ctx,
		"POST",
		p.baseURL,
		strings.NewReader("{}"),
		nil,
		func(resp *http.Response) error {
			if resp.StatusCode == http.StatusConflict {
				if sessionID := resp.Header.Get("X-Transmission-Session-Id"); sessionID != "" {
					p.sessionID = sessionID
					return nil
				}
			}

			return fmt.Errorf("failed to get session ID")
		},
		headers,
	)

	return err
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
