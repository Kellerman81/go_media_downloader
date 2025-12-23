package sabnzbd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// SABnzbd API response structures
//

type sabAddResult struct {
	Status bool     `json:"status"`
	NzoIDs []string `json:"nzo_ids"`
	Error  string   `json:"error"`
}

type sabBasicResult struct {
	Status bool `json:"status"`
}

type sabQueueResult struct {
	Queue sabQueue `json:"queue"`
}

type sabQueue struct {
	Status        string         `json:"status"`
	SpeedLimit    string         `json:"speedlimit"`
	SpeedLimitAbs string         `json:"speedlimit_abs"`
	Paused        bool           `json:"paused"`
	NoOfSlots     int            `json:"noofslots"`
	DiskSpace1    string         `json:"diskspace1"`
	DiskSpace2    string         `json:"diskspace2"`
	TimeLeft      string         `json:"timeleft"`
	MB            string         `json:"mb"`
	MBLeft        string         `json:"mbleft"`
	KBPerSec      string         `json:"kbpersec"`
	Slots         []sabQueueSlot `json:"slots"`
}

type sabQueueSlot struct {
	Status     string `json:"status"`
	Index      int    `json:"index"`
	ETA        string `json:"eta"`
	TimeLeft   string `json:"timeleft"`
	AvgAge     string `json:"avg_age"`
	Script     string `json:"script"`
	MsgID      string `json:"msgid"`
	Verbosity  string `json:"verbosity"`
	MB         string `json:"mb"`
	MBLeft     string `json:"mbleft"`
	Filename   string `json:"filename"`
	Priority   string `json:"priority"`
	Cat        string `json:"cat"`
	KBPerSec   string `json:"kbpersec"`
	Size       string `json:"size"`
	SizeLeft   string `json:"sizeleft"`
	Percentage string `json:"percentage"`
	NzoID      string `json:"nzo_id"`
}

//
// Provider Implementation
//

// Provider implements the DownloadProvider interface for SABnzbd
type Provider struct {
	*base.BaseClient
	host    string
	port    int
	apiKey  string
	baseURL string
}

// NewProvider creates a new SABnzbd download provider
func NewProvider(host string, port int, apiKey string, useSSL bool) (*Provider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("SABnzbd requires an API key")
	}

	if port == 0 {
		port = 8080 // Default SABnzbd port
	}

	// Build base URL
	scheme := "http"
	if useSSL {
		scheme = "https"
	}

	baseURL := fmt.Sprintf("%s://%s:%d/api", scheme, host, port)

	config := base.ClientConfig{
		Name:                    "sabnzbd",
		BaseURL:                 baseURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthNone, // Uses API key in URL params
		APIKey:                  apiKey,
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
		apiKey:     apiKey,
		baseURL:    baseURL,
	}, nil
}

// GetProviderType returns the download provider type
func (p *Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderSABnzbd
}

// GetProviderName returns the provider name
func (p *Provider) GetProviderName() string {
	return "sabnzbd"
}

// GetTorrentInfo retrieves information about a specific NZB download
func (p *Provider) GetTorrentInfo(ctx context.Context, hash string) (*apiexternal_v2.TorrentInfo, error) {
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

	return nil, fmt.Errorf("download not found: %s", hash)
}

// ListTorrents lists all NZB downloads in the queue
func (p *Provider) ListTorrents(ctx context.Context, filter string) (*apiexternal_v2.TorrentListResponse, error) {
	params := url.Values{
		"mode":   {"queue"},
		"output": {"json"},
		"apikey": {p.apiKey},
	}

	resp, err := p.makeRequest(ctx, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleHTTPError(resp)
	}

	var queueResult sabQueueResult
	if err := json.NewDecoder(resp.Body).Decode(&queueResult); err != nil {
		return nil, fmt.Errorf("failed to decode SABnzbd queue response: %w", err)
	}

	torrents := make([]apiexternal_v2.TorrentInfo, 0, len(queueResult.Queue.Slots))
	for _, slot := range queueResult.Queue.Slots {
		torrent := p.convertSlotToTorrentInfo(slot)
		torrents = append(torrents, torrent)
	}

	return &apiexternal_v2.TorrentListResponse{
		Torrents: torrents,
		Total:    len(torrents),
	}, nil
}

// PauseTorrent pauses a download
func (p *Provider) PauseTorrent(ctx context.Context, hash string) error {
	params := url.Values{
		"mode":   {"queue"},
		"name":   {"pause"},
		"value":  {hash},
		"output": {"json"},
		"apikey": {p.apiKey},
	}

	resp, err := p.makeRequest(ctx, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.handleHTTPError(resp)
	}

	var result sabBasicResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode SABnzbd response: %w", err)
	}

	if !result.Status {
		return fmt.Errorf("SABnzbd pause failed")
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Msg("Download paused")

	return nil
}

// ResumeTorrent resumes a download
func (p *Provider) ResumeTorrent(ctx context.Context, hash string) error {
	params := url.Values{
		"mode":   {"queue"},
		"name":   {"resume"},
		"value":  {hash},
		"output": {"json"},
		"apikey": {p.apiKey},
	}

	resp, err := p.makeRequest(ctx, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.handleHTTPError(resp)
	}

	var result sabBasicResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode SABnzbd response: %w", err)
	}

	if !result.Status {
		return fmt.Errorf("SABnzbd resume failed")
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Msg("Download resumed")

	return nil
}

// RemoveTorrent removes a download from the queue
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	params := url.Values{
		"mode":   {"queue"},
		"name":   {"delete"},
		"value":  {hash},
		"output": {"json"},
		"apikey": {p.apiKey},
	}

	// Note: SABnzbd automatically manages files, deleteFiles parameter is ignored

	resp, err := p.makeRequest(ctx, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.handleHTTPError(resp)
	}

	var result sabBasicResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode SABnzbd response: %w", err)
	}

	if !result.Status {
		return fmt.Errorf("SABnzbd remove failed")
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
	params := url.Values{
		"mode":   {"version"},
		"output": {"json"},
		"apikey": {p.apiKey},
	}

	resp, err := p.makeRequest(ctx, params)
	if err != nil {
		return &apiexternal_v2.DownloadClientStatus{
			Connected: false,
			Message:   fmt.Sprintf("Failed to connect: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &apiexternal_v2.DownloadClientStatus{
			Connected: false,
			Message:   fmt.Sprintf("HTTP error: %d", resp.StatusCode),
		}, nil
	}

	return &apiexternal_v2.DownloadClientStatus{
		Connected: true,
		Message:   "Connected to SABnzbd",
	}, nil
}

// TestConnection tests the connection to SABnzbd
func (p *Provider) TestConnection(ctx context.Context) error {
	status, err := p.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if !status.Connected {
		return fmt.Errorf("not connected: %s", status.Message)
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

// makeRequest makes an HTTP request to the SABnzbd API
func (p *Provider) makeRequest(ctx context.Context, params url.Values) (*http.Response, error) {
	requestURL := p.baseURL + "?" + params.Encode()

	headers := map[string]string{
		"User-Agent": "apiexternal_v2/1.0",
		"Accept":     "application/json",
	}

	var rawResp *http.Response
	err := p.MakeRequestWithHeaders(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
		nil,
		func(resp *http.Response) error {
			rawResp = resp
			return nil
		},
		headers,
	)

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return rawResp, nil
}

// convertSlotToTorrentInfo converts SABnzbd queue slot to TorrentInfo
func (p *Provider) convertSlotToTorrentInfo(slot sabQueueSlot) apiexternal_v2.TorrentInfo {
	torrent := apiexternal_v2.TorrentInfo{
		Hash: slot.NzoID,
		Name: slot.Filename,
	}

	// Convert status
	switch strings.ToLower(slot.Status) {
	case "downloading":
		torrent.State = "downloading"
	case "paused":
		torrent.State = "paused"
	case "queued":
		torrent.State = "queued"
	case "completed":
		torrent.State = "completed"
	default:
		torrent.State = slot.Status
	}

	// Parse size and progress
	if size, err := strconv.ParseFloat(slot.MB, 64); err == nil {
		torrent.Size = int64(size * 1024 * 1024) // Convert MB to bytes
	}

	if left, err := strconv.ParseFloat(slot.MBLeft, 64); err == nil {
		if torrent.Size > 0 {
			downloaded := torrent.Size - int64(left*1024*1024)
			torrent.Downloaded = downloaded
			torrent.Progress = float64(downloaded) / float64(torrent.Size) * 100
		}
	}

	// Parse speed (in KB/s)
	if speed, err := strconv.ParseFloat(slot.KBPerSec, 64); err == nil {
		torrent.DownloadSpeed = int64(speed * 1024) // Convert to bytes per second
	}

	// Parse ETA
	if slot.TimeLeft != "" && slot.TimeLeft != "0:00:00" {
		if eta := p.parseTimeLeft(slot.TimeLeft); eta > 0 {
			torrent.ETA = eta
		}
	}

	// Set priority
	if priority, err := strconv.Atoi(slot.Priority); err == nil {
		torrent.Priority = priority
	}

	// Set category as label
	torrent.Label = slot.Cat

	// Parse dates (SABnzbd doesn't provide add date in queue)
	torrent.AddedDate = time.Now()

	return torrent
}

// parseTimeLeft converts SABnzbd time format (HH:MM:SS) to seconds
func (p *Provider) parseTimeLeft(timeLeft string) int {
	parts := strings.Split(timeLeft, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	seconds, _ := strconv.Atoi(parts[2])

	return hours*3600 + minutes*60 + seconds
}

// handleHTTPError converts HTTP errors to meaningful error messages
func (p *Provider) handleHTTPError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid API key")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded")
	case http.StatusNotFound:
		return fmt.Errorf("endpoint not found")
	case http.StatusBadRequest:
		return fmt.Errorf("invalid request")
	default:
		return fmt.Errorf("HTTP error %d", resp.StatusCode)
	}
}

// AddNZB adds an NZB download to SABnzbd
//
// This is a SABnzbd-specific method not in the DownloadProvider interface,
// but useful for clients that need direct NZB support
func (p *Provider) AddNZB(ctx context.Context, nzbURL, category string, priority int) error {
	params := url.Values{
		"mode":   {"addurl"},
		"name":   {nzbURL},
		"output": {"json"},
		"apikey": {p.apiKey},
	}

	if category != "" {
		params.Set("cat", category)
	}

	if priority > 0 {
		params.Set("priority", strconv.Itoa(priority))
	}

	resp, err := p.makeRequest(ctx, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.handleHTTPError(resp)
	}

	var result sabAddResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode SABnzbd response: %w", err)
	}

	if !result.Status {
		return fmt.Errorf("SABnzbd add failed: %s", result.Error)
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("url", nzbURL).
		Strs("nzo_ids", result.NzoIDs).
		Msg("NZB added successfully")

	return nil
}
