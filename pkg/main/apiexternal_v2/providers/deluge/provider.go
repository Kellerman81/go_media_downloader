package deluge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Deluge Provider - Deluge Torrent Client
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the DownloadProvider interface for Deluge.
type Provider struct {
	*base.BaseClient
	baseURL   string
	username  string
	password  string
	sessionID string
	cookieJar http.CookieJar
}

// NewProvider creates a new Deluge download provider.
func NewProvider(baseURL, username, password string) *Provider {
	config := base.ClientConfig{
		Name:                    "deluge",
		BaseURL:                 baseURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthNone, // Custom JSON-RPC auth
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

	// Create cookie jar for session management
	jar, _ := cookiejar.New(nil)

	baseClient := base.NewBaseClient(config)

	// Configure BaseClient's HTTP client to use the cookie jar for session management
	baseClient.GetHTTPClient().Jar = jar

	return &Provider{
		BaseClient: baseClient,
		baseURL:    baseURL + "/json",
		username:   username,
		password:   password,
		cookieJar:  jar,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderDeluge
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "deluge"
}

// AddTorrent adds a torrent for downloading.
func (p *Provider) AddTorrent(
	ctx context.Context,
	request apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	// Prepare torrent options
	options := map[string]any{
		"download_location": request.SavePath,
	}

	if request.Paused {
		options["add_paused"] = true
	}

	if request.Priority > 0 {
		options["priority"] = request.Priority
	}

	// Apply additional options
	for key, value := range request.Options {
		switch key {
		case "max_connections", "max_upload_speed", "max_download_speed":
			if intVal, err := strconv.Atoi(value); err == nil {
				options[key] = intVal
			}

		case "move_completed":
			if boolVal, err := strconv.ParseBool(value); err == nil {
				options[key] = boolVal
			}

		case "move_completed_path":
			options[key] = value
		}
	}

	// Determine method based on URL type
	var (
		method string
		params []any
	)

	if strings.HasPrefix(request.URL, "magnet:") {
		method = "core.add_torrent_magnet"
		params = []any{request.URL, options}
	} else if len(request.TorrentData) > 0 {
		// For raw torrent data, encode as base64
		// Deluge expects base64-encoded torrent file
		method = "core.add_torrent_file"
		params = []any{"", string(request.TorrentData), options}
	} else {
		method = "core.add_torrent_url"
		params = []any{request.URL, options}
	}

	result, err := p.makeRPCCall(ctx, method, params)
	if err != nil {
		return nil, err
	}

	// Deluge returns the torrent hash on success
	hash, ok := result.(string)
	if !ok {
		hash = ""
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
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	// Request specific status keys
	keys := []string{
		"name", "state", "progress", "total_size", "total_done", "total_uploaded",
		"download_payload_rate", "upload_payload_rate", "eta", "save_path", "time_added",
	}

	result, err := p.makeRPCCall(ctx, "core.get_torrent_status", []any{hash, keys})
	if err != nil {
		return nil, err
	}

	data, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	return parseTorrentInfo(hash, data), nil
}

// ListTorrents retrieves the list of active torrents.
func (p *Provider) ListTorrents(
	ctx context.Context,
	filter string,
) (*apiexternal_v2.TorrentListResponse, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	// Request specific status keys
	keys := []string{
		"name", "state", "progress", "total_size", "total_done", "total_uploaded",
		"download_payload_rate", "upload_payload_rate", "eta", "save_path", "time_added",
	}

	// First parameter is filter dict (empty for all torrents), second is keys
	result, err := p.makeRPCCall(ctx, "core.get_torrents_status", []any{map[string]any{}, keys})
	if err != nil {
		return nil, err
	}

	torrentsData, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	torrents := make([]apiexternal_v2.TorrentInfo, 0, len(torrentsData))
	for hash, data := range torrentsData {
		if torrentMap, ok := data.(map[string]any); ok {
			torrent := parseTorrentInfo(hash, torrentMap)

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
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	_, err := p.makeRPCCall(ctx, "core.pause_torrent", []any{[]string{hash}})

	return err
}

// ResumeTorrent resumes a torrent.
func (p *Provider) ResumeTorrent(ctx context.Context, hash string) error {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	_, err := p.makeRPCCall(ctx, "core.resume_torrent", []any{[]string{hash}})

	return err
}

// RemoveTorrent removes a torrent.
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	_, err := p.makeRPCCall(ctx, "core.remove_torrent", []any{hash, deleteFiles})

	return err
}

// GetStatus retrieves the client status.
func (p *Provider) GetStatus(ctx context.Context) (*apiexternal_v2.DownloadClientStatus, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	result, err := p.makeRPCCall(
		ctx,
		"core.get_session_status",
		[]any{[]string{"upload_rate", "download_rate", "num_peers"}},
	)
	if err != nil {
		return nil, err
	}

	data, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	status := &apiexternal_v2.DownloadClientStatus{
		Connected: true,
		Version:   "Deluge",
		Provider:  "deluge",
	}

	if upRate, ok := data["upload_rate"].(float64); ok {
		status.TotalUpload = int64(upRate)
	}

	if downRate, ok := data["download_rate"].(float64); ok {
		status.TotalDownload = int64(downRate)
	}

	return status, nil
}

// TestConnection validates connectivity.
func (p *Provider) TestConnection(ctx context.Context) error {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	// Verify we're connected by checking connection status
	result, err := p.makeRPCCall(ctx, "web.connected", []any{})
	if err != nil {
		return err
	}

	if connected, ok := result.(bool); !ok || !connected {
		return fmt.Errorf("not connected to daemon")
	}

	return nil
}

// Helper methods

func (p *Provider) ensureAuthenticated(ctx context.Context) error {
	if p.sessionID != "" {
		return nil
	}

	return p.authenticate(ctx)
}

func (p *Provider) authenticate(ctx context.Context) error {
	// Authenticate with web interface
	if p.password != "" {
		result, err := p.makeRPCCall(ctx, "auth.login", []any{p.password})
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		if success, ok := result.(bool); !ok || !success {
			return fmt.Errorf("authentication failed: invalid credentials")
		}
	}

	// Check if we need to connect to a daemon
	// Get list of available hosts
	result, err := p.makeRPCCall(ctx, "web.get_hosts", []any{})
	if err != nil {
		// If this fails, we might already be connected or it's not needed
		p.sessionID = "authenticated"
		return nil
	}

	// Parse hosts list
	hosts, ok := result.([]any)
	if !ok || len(hosts) == 0 {
		p.sessionID = "authenticated"
		return nil
	}

	// Get the first host (usually the local daemon)
	firstHost, ok := hosts[0].([]any)
	if !ok || len(firstHost) == 0 {
		p.sessionID = "authenticated"
		return nil
	}

	// Extract host ID (first element)
	hostID, ok := firstHost[0].(string)
	if !ok {
		p.sessionID = "authenticated"
		return nil
	}

	// Check if already connected
	connResult, err := p.makeRPCCall(ctx, "web.connected", []any{})
	if err == nil {
		if connected, ok := connResult.(bool); ok && connected {
			p.sessionID = "authenticated"
			return nil
		}
	}

	// Connect to the daemon
	_, err = p.makeRPCCall(ctx, "web.connect", []any{hostID})
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	p.sessionID = "authenticated"

	return nil
}

func (p *Provider) makeRPCCall(ctx context.Context, method string, params []any) (any, error) {
	requestData := delugeRPCRequest{
		Method: method,
		Params: params,
		ID:     1,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	// Use BaseClient's MakeRequest with custom response handler for RPC
	var rpcResponse delugeRPCResponse

	err = p.MakeRequestWithHeaders(
		ctx,
		"POST",
		p.baseURL,
		strings.NewReader(string(jsonData)),
		nil,
		func(resp *http.Response) error {
			if resp.StatusCode == http.StatusUnauthorized {
				p.sessionID = "" // Clear session
			}

			if decodeErr := json.NewDecoder(resp.Body).Decode(&rpcResponse); decodeErr != nil {
				return decodeErr
			}

			if rpcResponse.Error != nil {
				return fmt.Errorf("RPC error: %v", rpcResponse.Error)
			}

			return nil
		},
		map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
	)
	if err != nil {
		return nil, err
	}

	return rpcResponse.Result, nil
}

//
// Internal types
//

type delugeRPCRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	ID     int    `json:"id"`
}

type delugeRPCResponse struct {
	Result any `json:"result"`
	Error  any `json:"error"`
	ID     int `json:"id"`
}

//
// Helper functions
//

func parseTorrentInfo(hash string, data map[string]any) *apiexternal_v2.TorrentInfo {
	info := &apiexternal_v2.TorrentInfo{
		Hash: hash,
	}

	if name, ok := data["name"].(string); ok {
		info.Name = name
	}

	if state, ok := data["state"].(string); ok {
		info.State = state
	}

	if progress, ok := data["progress"].(float64); ok {
		info.Progress = progress
	}

	if size, ok := data["total_size"].(float64); ok {
		info.Size = int64(size)
	}

	if downloaded, ok := data["total_done"].(float64); ok {
		info.Downloaded = int64(downloaded)
	}

	if uploaded, ok := data["total_uploaded"].(float64); ok {
		info.Uploaded = int64(uploaded)
	}

	if downSpeed, ok := data["download_payload_rate"].(float64); ok {
		info.DownloadSpeed = int64(downSpeed)
	}

	if upSpeed, ok := data["upload_payload_rate"].(float64); ok {
		info.UploadSpeed = int64(upSpeed)
	}

	if eta, ok := data["eta"].(float64); ok {
		info.ETA = int(eta)
	}

	if savePath, ok := data["save_path"].(string); ok {
		info.SavePath = savePath
	}

	if addedTime, ok := data["time_added"].(float64); ok {
		info.AddedDate = time.Unix(int64(addedTime), 0)
	}

	return info
}
