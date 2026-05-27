package nzbget

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

//
// NZBGet JSON-RPC structures
//

type jsonRPCRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	ID     int    `json:"id"`
}

type jsonRPCResponse struct {
	Version string          `json:"version"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error"`
	ID      int             `json:"id"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type nzbGetVersion struct {
	Version string `json:"version"`
}

type nzbGetQueueItem struct {
	NZBID            int    `json:"NZBID"`
	NZBFilename      string `json:"NZBFilename"`
	NZBName          string `json:"NZBName"`
	Category         string `json:"Category"`
	FileSizeMB       int64  `json:"FileSizeMB"`
	RemainingSizeMB  int64  `json:"RemainingSizeMB"`
	PausedSizeMB     int64  `json:"PausedSizeMB"`
	Status           string `json:"Status"`
	Priority         int    `json:"Priority"`
	DownloadedSizeMB int64  `json:"DownloadedSizeMB"`
	DownloadRate     int    `json:"DownloadRate"`
	PostTime         int64  `json:"PostTime"`
}

//
// Provider Implementation
//

// Provider implements the DownloadProvider interface for NZBGet
type Provider struct {
	*base.BaseClient
	host     string
	port     int
	username string
	password string
	baseURL  string
}

// NewProvider creates a new NZBGet download provider
func NewProvider(host string, port int, username, password string, useSSL bool) (*Provider, error) {
	if port == 0 {
		port = 6789 // Default NZBGet port
	}

	// Build base URL for JSON-RPC
	scheme := "http"
	if useSSL {
		scheme = "https"
	}

	baseURL := fmt.Sprintf("%s://%s:%d/jsonrpc", scheme, host, port)

	config := base.ClientConfig{
		Name:                    "nzbget",
		BaseURL:                 baseURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthNone, // Uses Basic auth in JSON-RPC requests
		Username:                username,
		Password:                password,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		baseURL:    baseURL,
	}, nil
}

// GetProviderType returns the download provider type
func (p *Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderNZBGet
}

// GetProviderName returns the provider name
func (p *Provider) GetProviderName() string {
	return "nzbget"
}

// GetTorrentInfo retrieves information about a specific NZB download
func (p *Provider) GetTorrentInfo(
	ctx context.Context,
	hash string,
) (*apiexternal_v2.TorrentInfo, error) {
	// Parse hash as NZBID
	nzbID, err := strconv.Atoi(hash)
	if err != nil {
		return nil, errors.New(logger.JoinStrings("invalid NZBID: ", hash))
	}

	// Get all downloads and find the specific one
	list, err := p.ListTorrents(ctx, "")
	if err != nil {
		return nil, err
	}

	for _, torrent := range list.Torrents {
		if torrent.Hash == hash {
			return &torrent, nil
		}
	}

	// Not found in queue, might be in history
	historyItems, err := p.getHistory(ctx, false)
	if err != nil {
		return nil, err
	}

	for _, item := range historyItems {
		if item.NZBID == nzbID {
			return p.convertHistoryItemToTorrentInfo(item), nil
		}
	}

	return nil, errors.New(logger.JoinStrings("download not found: ", hash))
}

// ListTorrents lists all NZB downloads in the queue
func (p *Provider) ListTorrents(
	ctx context.Context,
	filter string,
) (*apiexternal_v2.TorrentListResponse, error) {
	var queueItems []nzbGetQueueItem

	result, err := p.makeJSONRPCCall(ctx, "listgroups", []any{})
	if err != nil {
		return nil, errors.New(logger.JoinStrings("failed to list downloads: ", err.Error()))
	}

	if err := json.Unmarshal(result, &queueItems); err != nil {
		return nil, errors.New(logger.JoinStrings("failed to decode queue items: ", err.Error()))
	}

	torrents := make([]apiexternal_v2.TorrentInfo, 0, len(queueItems))
	for _, item := range queueItems {
		torrent := p.convertQueueItemToTorrentInfo(item)
		torrents = append(torrents, torrent)
	}

	return &apiexternal_v2.TorrentListResponse{
		Torrents: torrents,
		Total:    len(torrents),
	}, nil
}

// PauseTorrent pauses a download
func (p *Provider) PauseTorrent(ctx context.Context, hash string) error {
	nzbID, err := strconv.Atoi(hash)
	if err != nil {
		return errors.New(logger.JoinStrings("invalid NZBID: ", hash))
	}

	_, err = p.makeJSONRPCCall(ctx, "editqueue", []any{"GroupPause", 0, "", []int{nzbID}})
	if err != nil {
		return errors.New(logger.JoinStrings("failed to pause download: ", err.Error()))
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Msg("Download paused")

	return nil
}

// ResumeTorrent resumes a download
func (p *Provider) ResumeTorrent(ctx context.Context, hash string) error {
	nzbID, err := strconv.Atoi(hash)
	if err != nil {
		return errors.New(logger.JoinStrings("invalid NZBID: ", hash))
	}

	_, err = p.makeJSONRPCCall(ctx, "editqueue", []any{"GroupResume", 0, "", []int{nzbID}})
	if err != nil {
		return errors.New(logger.JoinStrings("failed to resume download: ", err.Error()))
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Msg("Download resumed")

	return nil
}

// RemoveTorrent removes a download from the queue
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	nzbID, err := strconv.Atoi(hash)
	if err != nil {
		return errors.New(logger.JoinStrings("invalid NZBID: ", hash))
	}

	action := "GroupDelete"
	if deleteFiles {
		action = "GroupFinalDelete" // Deletes from history and files
	}

	_, err = p.makeJSONRPCCall(ctx, "editqueue", []any{action, 0, "", []int{nzbID}})
	if err != nil {
		return errors.New(logger.JoinStrings("failed to remove download: ", err.Error()))
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Bool("delete_files", deleteFiles).
		Msg("Download removed")

	return nil
}

// GetStatus retrieves the download client status
func (p *Provider) GetStatus(ctx context.Context) (*apiexternal_v2.DownloadClientStatus, error) {
	result, err := p.makeJSONRPCCall(ctx, "version", []any{})
	if err != nil {
		return &apiexternal_v2.DownloadClientStatus{
			Connected: false,
			Message:   fmt.Sprintf("Failed to connect: %v", err),
		}, nil
	}

	var version string
	if err := json.Unmarshal(result, &version); err != nil {
		return &apiexternal_v2.DownloadClientStatus{
			Connected: false,
			Message:   "Failed to parse version",
		}, nil
	}

	return &apiexternal_v2.DownloadClientStatus{
		Connected: true,
		Message:   fmt.Sprintf("Connected to NZBGet %s", version),
		Version:   version,
	}, nil
}

// TestConnection tests the connection to NZBGet
func (p *Provider) TestConnection(ctx context.Context) error {
	status, err := p.GetStatus(ctx)
	if err != nil {
		return errors.New(logger.JoinStrings("connection test failed: ", err.Error()))
	}

	if !status.Connected {
		return errors.New(logger.JoinStrings("not connected: ", status.Message))
	}

	return nil
}

//
// Helper Methods
//

// GetHTTPClient returns an HTTP client for this provider
func (p *Provider) GetHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// makeJSONRPCCall makes a JSON-RPC call to NZBGet
func (p *Provider) makeJSONRPCCall(
	ctx context.Context,
	method string,
	params []any,
) (json.RawMessage, error) {
	request := jsonRPCRequest{
		Method: method,
		Params: params,
		ID:     1,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, errors.New(logger.JoinStrings("failed to marshal JSON-RPC request: ", err.Error()))
	}

	// Build custom headers
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "apiexternal_v2/1.0",
	}

	// Add authentication if provided
	if p.username != "" && p.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))
		headers["Authorization"] = "Basic " + auth
	}

	// Use BaseClient's MakeRequest with custom response handler for JSON-RPC
	var response jsonRPCResponse
	err = p.MakeRequestWithHeaders(
		ctx,
		"POST",
		p.baseURL,
		bytes.NewReader(jsonData),
		nil,
		func(resp *http.Response) error {
			if decodeErr := json.NewDecoder(resp.Body).Decode(&response); decodeErr != nil {
				return errors.New(logger.JoinStrings("failed to decode JSON-RPC response: ", decodeErr.Error()))
			}

			if response.Error != nil {
				return errors.New(logger.JoinStrings("JSON-RPC error ", strconv.Itoa(response.Error.Code), ": ", response.Error.Message))
			}

			return nil
		},
		headers,
	)
	if err != nil {
		return nil, err
	}

	return response.Result, nil
}

// convertQueueItemToTorrentInfo converts NZBGet queue item to TorrentInfo
func (p *Provider) convertQueueItemToTorrentInfo(item nzbGetQueueItem) apiexternal_v2.TorrentInfo {
	torrent := apiexternal_v2.TorrentInfo{
		Hash: strconv.Itoa(item.NZBID),
		Name: item.NZBName,
	}

	// Set size in bytes (NZBGet returns MB)
	torrent.Size = item.FileSizeMB * 1024 * 1024
	torrent.Downloaded = item.DownloadedSizeMB * 1024 * 1024

	// Calculate progress
	if torrent.Size > 0 {
		torrent.Progress = float64(torrent.Downloaded) / float64(torrent.Size) * 100
	}

	// Set download speed (bytes per second)
	torrent.DownloadSpeed = int64(item.DownloadRate)

	// Calculate ETA
	if item.DownloadRate > 0 && item.RemainingSizeMB > 0 {
		remainingBytes := item.RemainingSizeMB * 1024 * 1024
		torrent.ETA = int(remainingBytes / int64(item.DownloadRate))
	}

	// Convert status
	switch strings.ToUpper(item.Status) {
	case "DOWNLOADING":
		torrent.State = "downloading"
	case "PAUSED":
		torrent.State = "paused"
	case "QUEUED":
		torrent.State = "queued"
	default:
		torrent.State = strings.ToLower(item.Status)
	}

	// Set priority
	torrent.Priority = item.Priority

	// Set category as label
	torrent.Label = item.Category

	// Set added date
	if item.PostTime > 0 {
		torrent.AddedDate = time.Unix(item.PostTime, 0)
	}

	return torrent
}

// getHistory retrieves NZBGet download history
func (p *Provider) getHistory(ctx context.Context, hidden bool) ([]nzbGetQueueItem, error) {
	result, err := p.makeJSONRPCCall(ctx, "history", []any{hidden})
	if err != nil {
		return nil, errors.New(logger.JoinStrings("failed to get history: ", err.Error()))
	}

	var historyItems []nzbGetQueueItem
	if err := json.Unmarshal(result, &historyItems); err != nil {
		return nil, errors.New(logger.JoinStrings("failed to decode history: ", err.Error()))
	}

	return historyItems, nil
}

// convertHistoryItemToTorrentInfo converts history item to TorrentInfo
func (p *Provider) convertHistoryItemToTorrentInfo(
	item nzbGetQueueItem,
) *apiexternal_v2.TorrentInfo {
	torrent := &apiexternal_v2.TorrentInfo{
		Hash: strconv.Itoa(item.NZBID),
		Name: item.NZBName,
	}

	torrent.Size = item.FileSizeMB * 1024 * 1024
	torrent.Downloaded = item.DownloadedSizeMB * 1024 * 1024
	torrent.State = "completed"
	torrent.Progress = 100.0
	torrent.Label = item.Category

	if item.PostTime > 0 {
		torrent.AddedDate = time.Unix(item.PostTime, 0)
	}

	return torrent
}

// AddNZB implements the DownloadProvider interface for NZB support
// This is the interface-compliant version that calls AddNZBExtended with default parameters
func (p *Provider) AddNZB(ctx context.Context, nzbURL, category string, priority int) error {
	_, err := p.AddNZBExtended(ctx, nzbURL, category, priority, false)
	return err
}

// AddNZBExtended adds an NZB download to NZBGet with full control
//
// This is an extended method with additional parameters not in the DownloadProvider interface,
// but useful for clients that need direct NZB support with more options
func (p *Provider) AddNZBExtended(
	ctx context.Context,
	nzbURL, category string,
	priority int,
	addPaused bool,
) (int, error) {
	// Download the NZB file using BaseClient infrastructure
	var buf bytes.Buffer
	var contentDisposition string

	err := p.MakeRequest(
		ctx,
		"GET",
		nzbURL,
		nil,
		nil,
		func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK {
				return errors.New(logger.JoinStrings("failed to download NZB: HTTP ", strconv.Itoa(resp.StatusCode)))
			}

			// Store Content-Disposition header for later
			contentDisposition = resp.Header.Get("Content-Disposition")

			// Read NZB content
			if _, err := buf.ReadFrom(resp.Body); err != nil {
				return errors.New(logger.JoinStrings("failed to read NZB content: ", err.Error()))
			}

			return nil
		},
	)
	if err != nil {
		return 0, err
	}

	// Encode NZB content to base64
	nzbContent := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Extract filename - try multiple methods
	filename := ""

	// 1. Try Content-Disposition header
	if contentDisposition != "" {
		// Parse Content-Disposition: attachment; filename="something.nzb"
		if parts := strings.Split(contentDisposition, "filename="); len(parts) > 1 {
			filename = strings.Trim(parts[1], "\"")
		}
	}

	// 2. If no Content-Disposition, try to extract from URL path (before query params)
	if filename == "" {
		parsedURL := nzbURL
		// Remove query parameters
		if idx := strings.Index(parsedURL, "?"); idx != -1 {
			parsedURL = parsedURL[:idx]
		}
		parts := strings.Split(parsedURL, "/")
		filename = parts[len(parts)-1]
	}

	// 3. If still no good filename, use a generic one
	if filename == "" || strings.Contains(filename, "=") {
		filename = fmt.Sprintf("download_%d", time.Now().Unix())
	}

	// Ensure .nzb extension
	if !strings.HasSuffix(filename, ".nzb") {
		filename = filename + ".nzb"
	}

	// Append NZB file
	params := []any{
		filename,   // NZB filename
		nzbContent, // NZB content (base64)
		category,   // Category
		priority,   // Priority
		false,      // Add to top of queue
		addPaused,  // Add paused
		"",         // Duplicate key (empty for auto-generate)
		0,          // Duplicate score
		"SCORE",    // Duplicate mode
		nil,        // Post-processing parameters
	}

	result, err := p.makeJSONRPCCall(ctx, "append", params)
	if err != nil {
		return 0, errors.New(logger.JoinStrings("failed to add NZB: ", err.Error()))
	}

	var nzbID int
	if err := json.Unmarshal(result, &nzbID); err != nil {
		return 0, errors.New(logger.JoinStrings("failed to decode NZB ID: ", err.Error()))
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("url", nzbURL).
		Int("nzb_id", nzbID).
		Msg("NZB added successfully")

	return nzbID, nil
}
