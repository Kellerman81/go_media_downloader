package qbittorrent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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
// qBittorrent Provider - Torrent download client
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the DownloadProvider interface for qBittorrent.
type Provider struct {
	*base.BaseClient
	baseURL  string
	username string
	password string
	// authMu guards authenticated - Provider instances are cached singletons
	// in providers/registry.go, reused across every concurrent operation
	// against a given qBittorrent instance for the app's lifetime, so this
	// flag is read/written from multiple goroutines.
	authMu        sync.Mutex
	authenticated bool
	cookieJar     http.CookieJar
	// proxied is set when a urlBase puts the WebUI behind a reverse proxy.
	proxied bool
}

// NewProvider creates a new qBittorrent download provider. urlBase is an
// optional reverse-proxy sub-path ("/qbittorrent") in front of /api/v2.
func NewProvider(
	host string,
	port int,
	username, password string,
	useSSL bool,
	urlBase string,
) (*Provider, error) {
	scheme := "http"
	if useSSL {
		scheme = "https"
	}

	if port == 0 {
		port = 8080 // Default qBittorrent port
	}

	baseURL := fmt.Sprintf("%s://%s:%d%s/api/v2", scheme, host, port, strings.TrimSuffix(urlBase, "/"))

	config := base.ClientConfig{
		Name:                    "qbittorrent",
		BaseURL:                 baseURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthNone, // Uses session cookie
		Username:                username,
		Password:                password,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	cookieJar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, errors.New(logger.JoinStrings("failed to create cookie jar: ", err.Error()))
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
		proxied:    strings.Trim(urlBase, "/") != "",
	}, nil
}

// GetProviderType returns the provider type.
func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderQBittorrent
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
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

	// qBittorrent 5 renamed the "paused" add parameter to "stopped"; older
	// versions ignore the unknown one, so both are sent.
	formData := url.Values{
		"category": {request.Category},
		"paused":   {strconv.FormatBool(request.Paused)},
		"stopped":  {strconv.FormatBool(request.Paused)},
	}

	// An empty savepath is omitted so qBittorrent applies its default (or
	// the category's) save path.
	if request.SavePath != "" {
		formData.Set("savepath", request.SavePath)
	}

	if request.URL != "" {
		formData.Set("urls", request.URL)
	}

	if len(request.Tags) > 0 {
		formData.Set("tags", logger.JoinStringsSep(request.Tags, ","))
	}

	for k, v := range request.Options {
		formData.Set(k, v)
	}

	hash := strings.ToLower(base.InfoHashFromURL(request.URL))

	var (
		resp *http.Response
		err  error
	)

	if len(request.TorrentData) > 0 {
		// A raw .torrent can only be sent as a multipart file part.
		body, berr := multipartBody(formData, request.TorrentData)
		if berr != nil {
			return nil, berr
		}

		resp, err = p.doWithReauth(ctx, http.MethodPost, "/torrents/add", body)
	} else {
		resp, err = p.makeFormRequest(ctx, "/torrents/add", formData)
	}

	if err != nil {
		// qBittorrent 5 answers an add of a torrent it already has with 409.
		if strings.Contains(err.Error(), "HTTP 409") {
			return &apiexternal_v2.TorrentAddResponse{
				Success:  true,
				Hash:     hash,
				Message:  "torrent already added",
				Provider: "qbittorrent",
			}, nil
		}

		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)

		return &apiexternal_v2.TorrentAddResponse{
			Success:  false,
			Provider: "qbittorrent",
			Error:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}, errors.New(logger.JoinStrings("failed to add torrent: HTTP ", strconv.Itoa(resp.StatusCode)))
	}

	// qBittorrent (WebAPI v2 before 5.x) rejects an add - invalid URL or
	// torrent, unreachable link - with HTTP 200 and the body "Fails.".
	body, _ := io.ReadAll(resp.Body)
	if strings.EqualFold(strings.TrimSpace(string(body)), "Fails.") {
		return &apiexternal_v2.TorrentAddResponse{
			Success:  false,
			Provider: "qbittorrent",
			Error:    "qBittorrent rejected the torrent",
		}, errors.New("failed to add torrent: qBittorrent rejected it (invalid torrent/URL)")
	}

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Hash:     hash,
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

	resp, err := p.makeRequest(ctx, http.MethodGet, "/torrents/info?hashes="+url.QueryEscape(hash))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var torrents []qbtTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, errors.New(logger.JoinStrings("failed to decode response: ", err.Error()))
	}

	if len(torrents) == 0 {
		return nil, errors.New("torrent not found")
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

	resp, err := p.makeRequest(ctx, http.MethodGet, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var qbtTorrents []qbtTorrent
	if err := json.NewDecoder(resp.Body).Decode(&qbtTorrents); err != nil {
		return nil, errors.New(logger.JoinStrings("failed to decode response: ", err.Error()))
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
	if err != nil && strings.Contains(err.Error(), "HTTP 404") {
		// qBittorrent 5 renamed pause/resume to stop/start.
		resp, err = p.makeFormRequest(ctx, "/torrents/stop", formData)
	}

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return errors.New(
			logger.JoinStrings("failed to pause torrent: HTTP ", strconv.Itoa(resp.StatusCode)),
		)
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
	if err != nil && strings.Contains(err.Error(), "HTTP 404") {
		// qBittorrent 5 renamed pause/resume to stop/start.
		resp, err = p.makeFormRequest(ctx, "/torrents/start", formData)
	}

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return errors.New(
			logger.JoinStrings("failed to resume torrent: HTTP ", strconv.Itoa(resp.StatusCode)),
		)
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

	if resp.StatusCode/100 != 2 {
		return errors.New(
			logger.JoinStrings("failed to remove torrent: HTTP ", strconv.Itoa(resp.StatusCode)),
		)
	}

	return nil
}

// GetStatus retrieves the download client status.
func (p *Provider) GetStatus(ctx context.Context) (*apiexternal_v2.DownloadClientStatus, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	resp, err := p.makeRequest(ctx, http.MethodGet, "/transfer/info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info qbtTransferInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, errors.New(logger.JoinStrings("failed to decode response: ", err.Error()))
	}

	// Get version
	versionResp, err := p.makeRequest(ctx, http.MethodGet, "/app/version")
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

	resp, err := p.makeRequest(ctx, http.MethodGet, "/app/version")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return errors.New(
			logger.JoinStrings("qBittorrent test failed: HTTP ", strconv.Itoa(resp.StatusCode)),
		)
	}

	return nil
}

// Helper methods

func (p *Provider) ensureAuthenticated(ctx context.Context) error {
	p.authMu.Lock()
	defer p.authMu.Unlock()

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

	if resp.StatusCode/100 != 2 {
		return errors.New(
			logger.JoinStrings("authentication failed: HTTP ", strconv.Itoa(resp.StatusCode)),
		)
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) == "Fails." {
		return errors.New("invalid username or password")
	}

	p.authenticated = true

	return nil
}

func (p *Provider) makeRequest(
	ctx context.Context,
	method, endpoint string,
) (*http.Response, error) {
	return p.doWithReauth(ctx, method, endpoint, nil)
}

func (p *Provider) makeFormRequest(
	ctx context.Context,
	endpoint string,
	formData url.Values,
) (*http.Response, error) {
	return p.doWithReauth(ctx, http.MethodPost, endpoint, &requestBody{
		data:        []byte(formData.Encode()),
		contentType: "application/x-www-form-urlencoded",
	})
}

// requestBody is a buffered request body, so doWithReauth can re-send it
// after a re-login.
type requestBody struct {
	data        []byte
	contentType string
}

// multipartBody encodes fields plus torrentData as the "torrents" file part
// of a multipart/form-data body.
func multipartBody(fields url.Values, torrentData []byte) (*requestBody, error) {
	var buf bytes.Buffer

	mw := multipart.NewWriter(&buf)

	for k, vs := range fields {
		for _, v := range vs {
			if err := mw.WriteField(k, v); err != nil {
				return nil, fmt.Errorf("building multipart body: %w", err)
			}
		}
	}

	fw, err := mw.CreateFormFile("torrents", "upload.torrent")
	if err != nil {
		return nil, fmt.Errorf("building multipart body: %w", err)
	}

	if _, err := fw.Write(torrentData); err != nil {
		return nil, fmt.Errorf("building multipart body: %w", err)
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("building multipart body: %w", err)
	}

	return &requestBody{data: buf.Bytes(), contentType: mw.FormDataContentType()}, nil
}

// doWithReauth sends a request and, when qBittorrent answers 403 because the
// WebUI session cookie expired, logs in again and re-sends it once. The 403
// is rejected before anything is processed, so re-sending a POST (e.g. an
// add) cannot duplicate it.
func (p *Provider) doWithReauth(
	ctx context.Context,
	method, endpoint string,
	reqBody *requestBody,
) (*http.Response, error) {
	resp, forbidden, err := p.send(ctx, method, endpoint, reqBody)
	if err != nil || !forbidden {
		return resp, err
	}

	// ensureAuthenticated interprets the login response itself.
	if endpoint == "/auth/login" {
		return resp, nil
	}

	p.authMu.Lock()
	p.authenticated = false
	p.authMu.Unlock()

	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	resp, forbidden, err = p.send(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	if forbidden {
		return nil, errors.New(
			logger.JoinStrings("[qbittorrent] HTTP 403 on ", endpoint, " after re-login"),
		)
	}

	return resp, nil
}

// send performs one request with qBittorrent's CSRF headers; forbidden
// reports an HTTP 403, which is handed back instead of being an error so the
// caller can re-authenticate. A nil reqBody sends no body.
func (p *Provider) send(
	ctx context.Context,
	method, endpoint string,
	reqBody *requestBody,
) (resp *http.Response, forbidden bool, err error) {
	// qBittorrent rejects an Origin/Referer whose host differs from the Host
	// it receives, which behind a reverse proxy it usually does; requests
	// without either header pass its CSRF check.
	headers := map[string]string{}
	if !p.proxied {
		headers["Referer"] = p.baseURL
		headers["Origin"] = p.baseURL
	}

	var body io.Reader
	if reqBody != nil {
		headers["Content-Type"] = reqBody.contentType
		body = bytes.NewReader(reqBody.data)
	}

	err = p.MakeRequestAcceptStatus(
		ctx,
		method,
		endpoint,
		body,
		func(r *http.Response) error {
			if r.StatusCode == http.StatusForbidden {
				forbidden = true
			}

			resp = bufferResponseBody(r)

			return nil
		},
		headers,
		http.StatusForbidden,
	)
	if err != nil {
		return nil, false, err
	}

	return resp, forbidden, nil
}

// bufferResponseBody reads resp.Body into memory and replaces it with a
// fresh, independently-closable reader over the buffered bytes. The
// base.BaseClient request functions close resp.Body via a deferred call as
// soon as the callback passed to them returns - since makeRequest/
// makeFormRequest hand the *http.Response back to their own caller instead
// of reading it inside that callback, every field access to resp.Body
// afterward was reading an already-closed body. On a read failure, the
// original (still-open, since we haven't returned yet) response is
// returned unchanged so the caller's own error handling still applies.
func bufferResponseBody(resp *http.Response) *http.Response {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp
	}

	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	return resp
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
