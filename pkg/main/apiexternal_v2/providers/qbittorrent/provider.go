package qbittorrent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// qBittorrent Provider - Torrent download client
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the DownloadProvider interface for qBittorrent.
type Provider struct {
	*base.BaseClient
	baseURL       string
	username      string
	password      string
	authenticated bool
	cookieJar     http.CookieJar
}

// NewProvider creates a new qBittorrent download provider.
func NewProvider(host string, port int, username, password string, useSSL bool) (*Provider, error) {
	scheme := "http"
	if useSSL {
		scheme = "https"
	}

	if port == 0 {
		port = 8080 // Default qBittorrent port
	}

	baseURL := fmt.Sprintf("%s://%s:%d/api/v2", scheme, host, port)

	config := base.ClientConfig{
		Name:                    "qbittorrent",
		BaseURL:                 baseURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthNone, // Uses session cookie
		Username:                username,
		Password:                password,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	cookieJar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	baseClient := base.NewBaseClient(config)

	// Configure BaseClient's HTTP client to use the cookie jar for session management
	baseClient.GetHTTPClient().Jar = cookieJar

	return &Provider{
		BaseClient: baseClient,
		baseURL:    baseURL,
		username:   username,
		password:   password,
		cookieJar:  cookieJar,
	}, nil
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderQBittorrent
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "qbittorrent"
}

// AddTorrent adds a torrent for downloading.
func (p *Provider) AddTorrent(
	ctx context.Context,
	request apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	formData := url.Values{
		"savepath": {request.SavePath},
		"category": {request.Category},
		"paused":   {strconv.FormatBool(request.Paused)},
	}

	if request.URL != "" {
		formData.Set("urls", request.URL)
	}

	if len(request.Tags) > 0 {
		formData.Set("tags", strings.Join(request.Tags, ","))
	}

	for k, v := range request.Options {
		formData.Set(k, v)
	}

	resp, err := p.makeFormRequest(ctx, "/torrents/add", formData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return &apiexternal_v2.TorrentAddResponse{
			Success:  false,
			Provider: "qbittorrent",
			Error:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}, fmt.Errorf("failed to add torrent: HTTP %d", resp.StatusCode)
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Provider: "qbittorrent",
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

	resp, err := p.makeRequest(ctx, "GET", "/torrents/info?hashes="+hash, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var torrents []qbtTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(torrents) == 0 {
		return nil, fmt.Errorf("torrent not found")
	}

	return convertTorrentInfo(&torrents[0]), nil
}

// ListTorrents retrieves a list of torrents.
func (p *Provider) ListTorrents(
	ctx context.Context,
	filter string,
) (*apiexternal_v2.TorrentListResponse, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	endpoint := "/torrents/info"
	if filter != "" {
		endpoint += "?filter=" + filter
	}

	resp, err := p.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var qbtTorrents []qbtTorrent
	if err := json.NewDecoder(resp.Body).Decode(&qbtTorrents); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	torrents := make([]apiexternal_v2.TorrentInfo, len(qbtTorrents))
	for i, qbt := range qbtTorrents {
		torrents[i] = *convertTorrentInfo(&qbt)
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

	formData := url.Values{"hashes": {hash}}

	resp, err := p.makeFormRequest(ctx, "/torrents/pause", formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to pause torrent: HTTP %d", resp.StatusCode)
	}

	return nil
}

// ResumeTorrent resumes a torrent.
func (p *Provider) ResumeTorrent(ctx context.Context, hash string) error {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	formData := url.Values{"hashes": {hash}}

	resp, err := p.makeFormRequest(ctx, "/torrents/resume", formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to resume torrent: HTTP %d", resp.StatusCode)
	}

	return nil
}

// RemoveTorrent removes a torrent.
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	formData := url.Values{
		"hashes":      {hash},
		"deleteFiles": {strconv.FormatBool(deleteFiles)},
	}

	resp, err := p.makeFormRequest(ctx, "/torrents/delete", formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove torrent: HTTP %d", resp.StatusCode)
	}

	return nil
}

// GetStatus retrieves the download client status.
func (p *Provider) GetStatus(ctx context.Context) (*apiexternal_v2.DownloadClientStatus, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	resp, err := p.makeRequest(ctx, "GET", "/transfer/info", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info qbtTransferInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Get version
	versionResp, err := p.makeRequest(ctx, "GET", "/app/version", nil)
	if err != nil {
		return nil, err
	}
	defer versionResp.Body.Close()

	version, _ := io.ReadAll(versionResp.Body)

	return &apiexternal_v2.DownloadClientStatus{
		Version:       strings.TrimSpace(string(version)),
		FreeSpace:     info.FreeSpaceOnDisk,
		TotalDownload: info.DlInfoSpeed,
		TotalUpload:   info.UpInfoSpeed,
		Provider:      "qbittorrent",
	}, nil
}

// TestConnection validates connectivity.
func (p *Provider) TestConnection(ctx context.Context) error {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	resp, err := p.makeRequest(ctx, "GET", "/app/version", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent test failed: HTTP %d", resp.StatusCode)
	}

	return nil
}

// Helper methods

func (p *Provider) ensureAuthenticated(ctx context.Context) error {
	if p.authenticated {
		return nil
	}

	formData := url.Values{
		"username": {p.username},
		"password": {p.password},
	}

	resp, err := p.makeFormRequest(ctx, "/auth/login", formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) == "Fails." {
		return fmt.Errorf("invalid username or password")
	}

	p.authenticated = true

	return nil
}

func (p *Provider) makeRequest(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
) (*http.Response, error) {
	// Custom headers for qBittorrent CSRF protection
	headers := map[string]string{
		"Referer": p.baseURL,
		"Origin":  p.baseURL,
	}

	// Use BaseClient with custom response handler to return the raw response
	var rawResp *http.Response

	err := p.MakeRequestWithHeaders(
		ctx,
		method,
		endpoint,
		body,
		nil,
		func(resp *http.Response) error {
			rawResp = resp
			return nil
		},
		headers,
	)

	return rawResp, err
}

func (p *Provider) makeFormRequest(
	ctx context.Context,
	endpoint string,
	formData url.Values,
) (*http.Response, error) {
	// Custom headers for qBittorrent CSRF protection and form data
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Referer":      p.baseURL,
		"Origin":       p.baseURL,
	}

	// Use BaseClient with custom response handler to return the raw response
	var rawResp *http.Response

	err := p.MakeRequestWithHeaders(
		ctx,
		"POST",
		endpoint,
		strings.NewReader(formData.Encode()),
		nil,
		func(resp *http.Response) error {
			rawResp = resp
			return nil
		},
		headers,
	)

	return rawResp, err
}

//
// Internal types
//

type qbtTorrent struct {
	Hash         string  `json:"hash"`
	Name         string  `json:"name"`
	Size         int64   `json:"size"`
	Progress     float64 `json:"progress"`
	DlSpeed      int64   `json:"dlspeed"`
	UpSpeed      int64   `json:"upspeed"`
	Downloaded   int64   `json:"downloaded"`
	Uploaded     int64   `json:"uploaded"`
	Ratio        float64 `json:"ratio"`
	ETA          int     `json:"eta"`
	State        string  `json:"state"`
	Category     string  `json:"category"`
	Tags         string  `json:"tags"`
	SavePath     string  `json:"save_path"`
	AddedOn      int64   `json:"added_on"`
	CompletionOn int64   `json:"completion_on"`
}

type qbtTransferInfo struct {
	DlInfoSpeed     int64 `json:"dl_info_speed"`
	UpInfoSpeed     int64 `json:"up_info_speed"`
	FreeSpaceOnDisk int64 `json:"free_space_on_disk"`
}

func convertTorrentInfo(qbt *qbtTorrent) *apiexternal_v2.TorrentInfo {
	tags := []string{}
	if qbt.Tags != "" {
		tags = strings.Split(qbt.Tags, ",")
	}

	return &apiexternal_v2.TorrentInfo{
		Hash:          qbt.Hash,
		Name:          qbt.Name,
		State:         qbt.State,
		Size:          qbt.Size,
		Progress:      qbt.Progress * 100,
		DownloadSpeed: qbt.DlSpeed,
		UploadSpeed:   qbt.UpSpeed,
		Downloaded:    qbt.Downloaded,
		Uploaded:      qbt.Uploaded,
		Ratio:         qbt.Ratio,
		ETA:           qbt.ETA,
		SavePath:      qbt.SavePath,
		Category:      qbt.Category,
		Tags:          tags,
		AddedOn:       time.Unix(qbt.AddedOn, 0),
		CompletionOn:  time.Unix(qbt.CompletionOn, 0),
		Provider:      "qbittorrent",
	}
}
