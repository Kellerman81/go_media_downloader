package deluge

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"regexp"
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
// Deluge Provider - Deluge Torrent Client
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the DownloadProvider interface for Deluge.
type Provider struct {
	*base.BaseClient
	baseURL  string
	username string
	password string
	// sessionMu guards sessionID - Provider instances are cached singletons
	// in providers/registry.go, reused across every concurrent operation
	// against a given Deluge instance for the app's lifetime, so this field
	// is read/written from multiple goroutines.
	sessionMu sync.Mutex
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
		baseURL:    strings.TrimSuffix(baseURL, "/") + "/json",
		username:   username,
		password:   password,
		cookieJar:  jar,
	}
}

// GetProviderType returns the provider type.
func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderDeluge
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "deluge"
}

// AddTorrent adds a torrent for downloading.
func (p *Provider) AddTorrent(
	ctx context.Context,
	request apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	options := map[string]any{}

	// An empty download_location would override Deluge's default path with "".
	if request.SavePath != "" {
		options["download_location"] = request.SavePath
	}

	if request.Paused {
		options["add_paused"] = true
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
		// core.add_torrent_file(filename, filedump, options): filedump is the
		// base64-encoded .torrent content.
		method = "core.add_torrent_file"
		params = []any{
			"",
			base64.StdEncoding.EncodeToString(request.TorrentData),
			options,
		}
	} else {
		method = "core.add_torrent_url"
		params = []any{request.URL, options}
	}

	result, err := p.call(ctx, method, params)
	if err != nil {
		// Deluge 2.x raises AddTorrentError("Torrent already in session
		// (<hash>).") for a duplicate add; the torrent is present, which is
		// what the caller asked for.
		if hash, ok := alreadyInSessionHash(err); ok {
			return &apiexternal_v2.TorrentAddResponse{
				Hash:     hash,
				Success:  true,
				Message:  "torrent already added",
				Provider: "deluge",
			}, nil
		}

		return nil, err
	}

	// Deluge returns the torrent hash (a non-empty string) on success, and
	// null (no RPC-level error) when the daemon silently rejects the add -
	// most commonly a torrent already present in the session, but also a
	// rejected/invalid magnet. Treating any non-string result as success
	// made a real, silent add-failure indistinguishable from success.
	hash, ok := result.(string)
	if !ok || hash == "" {
		return &apiexternal_v2.TorrentAddResponse{
			Success: false,
			Error:   "deluge rejected the add (already present in session, or invalid torrent/magnet)",
		}, errors.New("deluge rejected the torrent add - no hash returned")
	}

	label := request.Label
	if label == "" {
		label = request.Category
	}

	if label != "" {
		if err := p.setLabel(ctx, hash, label); err != nil {
			logger.Logtype("warn", 0).
				Err(err).
				Str("hash", hash).
				Str("label", label).
				Msg("deluge: torrent added but label could not be set")
		}
	}

	return &apiexternal_v2.TorrentAddResponse{
		Hash:    hash,
		Success: true,
	}, nil
}

// setLabel assigns label to the torrent via Deluge's Label plugin, creating
// the label first if needed. Without the plugin enabled it does nothing.
// Deluge has no per-torrent priority in its core API, so the downloader
// priority setting is not applied to Deluge.
func (p *Provider) setLabel(ctx context.Context, hash, label string) error {
	plugins, err := p.call(ctx, "core.get_enabled_plugins", []any{})
	if err != nil {
		return err
	}

	if !containsString(plugins, "Label") {
		return nil
	}

	// The Label plugin only accepts lowercase label ids.
	label = strings.ToLower(label)

	labels, err := p.call(ctx, "label.get_labels", []any{})
	if err != nil {
		return err
	}

	if !containsString(labels, label) {
		if _, err := p.call(ctx, "label.add", []any{label}); err != nil {
			return err
		}
	}

	_, err = p.call(ctx, "label.set_torrent", []any{hash, label})

	return err
}

// containsString reports whether a decoded JSON array contains s.
func containsString(list any, s string) bool {
	items, _ := list.([]any)
	for _, item := range items {
		if v, ok := item.(string); ok && v == s {
			return true
		}
	}

	return false
}

// GetTorrentInfo retrieves information about a specific torrent.
func (p *Provider) GetTorrentInfo(
	ctx context.Context,
	hash string,
) (*apiexternal_v2.TorrentInfo, error) {
	// Request specific status keys
	keys := []string{
		"name", "state", "progress", "total_size", "total_done", "total_uploaded",
		"download_payload_rate", "upload_payload_rate", "eta", "save_path", "time_added",
	}

	result, err := p.call(ctx, "core.get_torrent_status", []any{hash, keys})
	if err != nil {
		return nil, err
	}

	data, ok := result.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected response format")
	}

	return parseTorrentInfo(hash, data), nil
}

// ListTorrents retrieves the list of active torrents.
func (p *Provider) ListTorrents(
	ctx context.Context,
	filter string,
) (*apiexternal_v2.TorrentListResponse, error) {
	// Request specific status keys
	keys := []string{
		"name", "state", "progress", "total_size", "total_done", "total_uploaded",
		"download_payload_rate", "upload_payload_rate", "eta", "save_path", "time_added",
	}

	// First parameter is filter dict (empty for all torrents), second is keys
	result, err := p.call(ctx, "core.get_torrents_status", []any{map[string]any{}, keys})
	if err != nil {
		return nil, err
	}

	torrentsData, ok := result.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected response format")
	}

	torrents := make([]apiexternal_v2.TorrentInfo, 0, len(torrentsData))
	for hash, data := range torrentsData {
		torrentMap, ok := data.(map[string]any)
		if !ok {
			continue
		}

		torrent := parseTorrentInfo(hash, torrentMap)

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
	// The plural form takes a list of ids in Deluge 2.x; the singular
	// pause_torrent takes a single id string there.
	_, err := p.call(ctx, "core.pause_torrents", []any{[]string{hash}})

	return err
}

// ResumeTorrent resumes a torrent.
func (p *Provider) ResumeTorrent(ctx context.Context, hash string) error {
	_, err := p.call(ctx, "core.resume_torrents", []any{[]string{hash}})

	return err
}

// RemoveTorrent removes a torrent.
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	result, err := p.call(ctx, "core.remove_torrent", []any{hash, deleteFiles})
	if err != nil {
		return err
	}

	// remove_torrent returns false (not an RPC error) when nothing was removed.
	if removed, ok := result.(bool); ok && !removed {
		return errors.New(logger.JoinStrings("deluge did not remove torrent ", hash))
	}

	return nil
}

// GetStatus retrieves the client status.
func (p *Provider) GetStatus(ctx context.Context) (*apiexternal_v2.DownloadClientStatus, error) {
	result, err := p.call(
		ctx,
		"core.get_session_status",
		[]any{[]string{"upload_rate", "download_rate", "num_peers"}},
	)
	if err != nil {
		return nil, err
	}

	data, ok := result.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected response format")
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
	// Verify we're connected by checking connection status
	connected, err := p.daemonConnected(ctx)
	if err != nil {
		return err
	}

	if !connected {
		if err := p.connectDaemon(ctx); err != nil {
			return err
		}

		if connected, err = p.daemonConnected(ctx); err != nil {
			return err
		}
	}

	if !connected {
		return errors.New("not connected to daemon")
	}

	return nil
}

// daemonConnected reports whether the web UI is attached to a daemon.
func (p *Provider) daemonConnected(ctx context.Context) (bool, error) {
	result, err := p.call(ctx, "web.connected", []any{})
	if err != nil {
		return false, err
	}

	connected, _ := result.(bool)

	return connected, nil
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

func (p *Provider) ensureAuthenticated(ctx context.Context) error {
	if p.loadSessionID() != "" {
		return nil
	}

	return p.authenticate(ctx)
}

func (p *Provider) authenticate(ctx context.Context) error {
	// Authenticate with web interface
	if p.password != "" {
		result, err := p.makeRPCCall(ctx, "auth.login", []any{p.password})
		if err != nil {
			return errors.New(logger.JoinStrings("authentication failed: ", err.Error()))
		}

		if success, ok := result.(bool); !ok || !success {
			return errors.New("authentication failed: invalid credentials")
		}
	}

	if err := p.connectDaemon(ctx); err != nil {
		return err
	}

	p.storeSessionID("authenticated")

	return nil
}

// connectDaemon attaches the web UI to the first configured daemon if it isn't
// already. deluge-web keeps login sessions across a restart but does not
// reconnect to the daemon by itself, so this must also run when a still-valid
// session reports itself disconnected - not only right after auth.login.
func (p *Provider) connectDaemon(ctx context.Context) error {
	connResult, err := p.makeRPCCall(ctx, "web.connected", []any{})
	if err == nil {
		if connected, ok := connResult.(bool); ok && connected {
			return nil
		}
	}

	result, err := p.makeRPCCall(ctx, "web.get_hosts", []any{})
	if err != nil {
		// Older web UIs without host management are connected implicitly.
		return nil
	}

	hosts, ok := result.([]any)
	if !ok || len(hosts) == 0 {
		return nil
	}

	firstHost, ok := hosts[0].([]any)
	if !ok || len(firstHost) == 0 {
		return nil
	}

	hostID, ok := firstHost[0].(string)
	if !ok {
		return nil
	}

	if _, err := p.makeRPCCall(ctx, "web.connect", []any{hostID}); err != nil {
		return errors.New(logger.JoinStrings("failed to connect to daemon: ", err.Error()))
	}

	return nil
}

// call runs an RPC method on an authenticated session. Deluge's web UI
// answers an expired session with HTTP 200 and an "error" member (code 1,
// "Not authenticated") rather than an HTTP status, so that is the signal to
// log in again; the call is then retried once.
func (p *Provider) call(ctx context.Context, method string, params []any) (any, error) {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	result, err := p.makeRPCCall(ctx, method, params)
	if err != nil && !errors.Is(err, errNotAuthenticated) && !strings.HasPrefix(method, "web.") {
		// A daemon (core.*) call fails while the web UI is detached from the
		// daemon, e.g. after a deluge restart: reattach and retry once.
		if connected, cerr := p.makeRPCCall(ctx, "web.connected", []any{}); cerr == nil {
			if ok, _ := connected.(bool); !ok {
				if p.connectDaemon(ctx) == nil {
					return p.makeRPCCall(ctx, method, params)
				}
			}
		}

		return result, err
	}

	if !errors.Is(err, errNotAuthenticated) {
		return result, err
	}

	p.storeSessionID("")

	if err := p.authenticate(ctx); err != nil {
		return nil, err
	}

	return p.makeRPCCall(ctx, method, params)
}

// errNotAuthenticated marks a request rejected because the web session is
// missing or expired.
var errNotAuthenticated = errors.New("deluge: not authenticated")

// delugeAuthErrorCode is the JSON-RPC error code Deluge's web API uses for
// "Not authenticated" (deluge/ui/web/json_api.py).
const delugeAuthErrorCode = 1

// alreadyInSessionRe matches Deluge's duplicate-add error text and captures
// the torrent's info hash.
var alreadyInSessionRe = regexp.MustCompile(`(?i)already in session \(([0-9a-f]{40})\)`)

func alreadyInSessionHash(err error) (string, bool) {
	m := alreadyInSessionRe.FindStringSubmatch(err.Error())
	if m == nil {
		return "", false
	}

	return m[1], true
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

	var rpcResponse delugeRPCResponse

	err = p.MakeRequestAcceptStatus(
		ctx,
		http.MethodPost,
		p.baseURL,
		bytes.NewReader(jsonData),
		func(resp *http.Response) error {
			if resp.StatusCode == http.StatusUnauthorized {
				return errNotAuthenticated
			}

			if decodeErr := json.NewDecoder(resp.Body).Decode(&rpcResponse); decodeErr != nil {
				return decodeErr
			}

			if rpcResponse.Error != nil {
				if rpcResponse.Error.Code == delugeAuthErrorCode {
					return errNotAuthenticated
				}

				return errors.New(logger.JoinStrings(
					"RPC error ",
					strconv.Itoa(rpcResponse.Error.Code),
					": ",
					rpcResponse.Error.Message,
				))
			}

			return nil
		},
		map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		http.StatusUnauthorized,
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
	Result any             `json:"result"`
	Error  *delugeRPCError `json:"error"`
	ID     int             `json:"id"`
}

type delugeRPCError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
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
