package rtorrent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// XML-RPC structures for rTorrent
//

type xmlRPCRequest struct {
	XMLName xml.Name   `xml:"methodCall"`
	Method  string     `xml:"methodName"`
	Params  *xmlParams `xml:"params,omitempty"`
}

type xmlParams struct {
	Params []xmlParam `xml:"param"`
}

type xmlParam struct {
	Value xmlValue `xml:"value"`
}

type xmlValue struct {
	String string    `xml:"string,omitempty"`
	Int    int       `xml:"int,omitempty"`
	Bool   bool      `xml:"boolean,omitempty"`
	Array  *xmlArray `xml:"array,omitempty"`
}

type xmlArray struct {
	Data []xmlValue `xml:"data>value"`
}

type xmlRPCResponse struct {
	XMLName xml.Name   `xml:"methodResponse"`
	Params  *xmlParams `xml:"params,omitempty"`
	Fault   *xmlFault  `xml:"fault,omitempty"`
}

type xmlFault struct {
	Value xmlValue `xml:"value"`
}

//
// Provider Implementation
//

// Provider implements the DownloadProvider interface for rTorrent.
type Provider struct {
	*base.BaseClient
	xmlrpcURL string
	username  string
	password  string
}

// NewProvider creates a new rTorrent download provider.
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
		port = 80
		if useSSL {
			port = 443
		}
	}

	xmlrpcPath := "/RPC2"
	if urlBase != "" {
		xmlrpcPath = strings.TrimSuffix(urlBase, "/") + "/RPC2"
	}

	xmlrpcURL := fmt.Sprintf("%s://%s:%d%s", scheme, host, port, xmlrpcPath)

	config := base.ClientConfig{
		Name:                    "rtorrent",
		BaseURL:                 xmlrpcURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthNone, // Uses Basic auth in requests
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
		xmlrpcURL:  xmlrpcURL,
		username:   username,
		password:   password,
	}, nil
}

// GetProviderType returns the download provider type.
func (p *Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderRTorrent
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "rtorrent"
}

// AddTorrent adds a torrent to rTorrent.
func (p *Provider) AddTorrent(
	ctx context.Context,
	request apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	var (
		method string
		params []any
	)

	if strings.HasPrefix(request.URL, "magnet:") {
		// Handle magnet links
		method = "load.start_verbose"
		params = []any{"", request.URL}

		if request.SavePath != "" {
			params = append(params, fmt.Sprintf("d.directory.set=%s", request.SavePath))
		}
	} else if len(request.TorrentData) > 0 {
		// Handle torrent file data
		method = "load.raw_start"
		params = []any{"", request.TorrentData}

		if request.SavePath != "" {
			params = append(params, fmt.Sprintf("d.directory.set=%s", request.SavePath))
		}
	} else {
		// Handle torrent URL
		method = "load.start"
		params = []any{"", request.URL}

		if request.SavePath != "" {
			params = append(params, fmt.Sprintf("d.directory.set=%s", request.SavePath))
		}
	}

	// Apply labels if provided
	if request.Label != "" {
		params = append(params, fmt.Sprintf("d.custom1.set=%s", request.Label))
	}

	if err := p.makeXMLRPCCall(ctx, method, params); err != nil {
		return &apiexternal_v2.TorrentAddResponse{
			Success:  false,
			Provider: "rtorrent",
			Error:    err.Error(),
		}, err
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("url", request.URL).
		Str("save_path", request.SavePath).
		Msg("Torrent added successfully")

	return &apiexternal_v2.TorrentAddResponse{
		Success:  true,
		Message:  "Torrent added successfully",
		Provider: "rtorrent",
	}, nil
}

// GetTorrentInfo retrieves information about a specific torrent.
func (p *Provider) GetTorrentInfo(
	ctx context.Context,
	hash string,
) (*apiexternal_v2.TorrentInfo, error) {
	// rTorrent multicall to get torrent info
	// This is a simplified version - full implementation would use d.multicall
	return &apiexternal_v2.TorrentInfo{
		Hash:     hash,
		Name:     "Unknown", // Would need multicall to get actual name
		State:    "unknown",
		Provider: "rtorrent",
	}, nil
}

// ListTorrents lists all torrents in rTorrent.
func (p *Provider) ListTorrents(
	ctx context.Context,
	filter string,
) (*apiexternal_v2.TorrentListResponse, error) {
	// Get list of torrent hashes
	// This is simplified - full implementation would use d.multicall2 for efficiency
	var hashes []string

	if err := p.makeXMLRPCCall(ctx, "download_list", []any{}); err != nil {
		return nil, err
	}

	torrents := make([]apiexternal_v2.TorrentInfo, 0, len(hashes))
	for _, hash := range hashes {
		torrents = append(torrents, apiexternal_v2.TorrentInfo{
			Hash:     hash,
			Name:     "Unknown",
			State:    "unknown",
			Provider: "rtorrent",
		})
	}

	return &apiexternal_v2.TorrentListResponse{
		Torrents: torrents,
		Total:    len(torrents),
	}, nil
}

// PauseTorrent pauses a torrent.
func (p *Provider) PauseTorrent(ctx context.Context, hash string) error {
	if err := p.makeXMLRPCCall(ctx, "d.pause", []any{hash}); err != nil {
		return err
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Msg("Torrent paused")

	return nil
}

// ResumeTorrent resumes a torrent.
func (p *Provider) ResumeTorrent(ctx context.Context, hash string) error {
	if err := p.makeXMLRPCCall(ctx, "d.resume", []any{hash}); err != nil {
		return err
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Msg("Torrent resumed")

	return nil
}

// RemoveTorrent removes a torrent from rTorrent.
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	if deleteFiles {
		// Remove torrent and delete files
		if err := p.makeXMLRPCCall(ctx, "d.erase", []any{hash}); err != nil {
			return err
		}
	} else {
		// Stop and close torrent
		if err := p.makeXMLRPCCall(ctx, "d.stop", []any{hash}); err != nil {
			return err
		}

		if err := p.makeXMLRPCCall(ctx, "d.close", []any{hash}); err != nil {
			return err
		}
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
		Bool("delete_files", deleteFiles).
		Msg("Torrent removed")

	return nil
}

// GetStatus retrieves the download client status.
func (p *Provider) GetStatus(ctx context.Context) (*apiexternal_v2.DownloadClientStatus, error) {
	// Test connectivity by getting API version
	if err := p.makeXMLRPCCall(ctx, "system.api_version", []any{}); err != nil {
		return &apiexternal_v2.DownloadClientStatus{
			Connected: false,
			Message:   fmt.Sprintf("Failed to connect: %v", err),
			Provider:  "rtorrent",
		}, nil
	}

	return &apiexternal_v2.DownloadClientStatus{
		Connected: true,
		Message:   "Connected to rTorrent",
		Provider:  "rtorrent",
	}, nil
}

// TestConnection tests the connection to rTorrent.
func (p *Provider) TestConnection(ctx context.Context) error {
	status, err := p.GetStatus(ctx)
	if err != nil {
		return err
	}

	if !status.Connected {
		return fmt.Errorf("not connected: %s", status.Message)
	}

	return nil
}

//
// Helper Methods
//

// GetHTTPClient returns an HTTP client for this provider.
func (p *Provider) GetHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// makeXMLRPCCall makes an XML-RPC call to rTorrent.
func (p *Provider) makeXMLRPCCall(ctx context.Context, method string, params []any) error {
	// Build XML-RPC request
	request := xmlRPCRequest{
		Method: method,
	}

	if len(params) > 0 {
		request.Params = &xmlParams{}
		for _, param := range params {
			var v xmlValue
			switch t := param.(type) {
			case string:
				v.String = t
			case int:
				v.Int = t
			case bool:
				v.Bool = t
			case []byte:
				v.String = string(t)
			default:
				v.String = fmt.Sprintf("%v", t)
			}

			request.Params.Params = append(request.Params.Params, xmlParam{Value: v})
		}
	}

	// Marshal to XML
	xmlData, err := xml.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal XML-RPC request: %w", err)
	}

	// Prepare headers with authentication
	headers := map[string]string{
		"Content-Type": "text/xml",
		"User-Agent":   "apiexternal_v2/1.0",
	}

	if p.username != "" && p.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))

		headers["Authorization"] = "Basic " + auth
	}

	// Make the request using BaseClient infrastructure
	return p.MakeRequestWithHeaders(
		ctx,
		"POST",
		p.xmlrpcURL,
		bytes.NewReader(xmlData),
		nil,
		func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)

				return fmt.Errorf(
					"HTTP request failed with status %d: %s",
					resp.StatusCode,
					string(body),
				)
			}

			// Read and parse XML response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}

			var response xmlRPCResponse
			if err := xml.Unmarshal(body, &response); err != nil {
				return fmt.Errorf("failed to unmarshal XML-RPC response: %w", err)
			}

			// Check for fault
			if response.Fault != nil {
				return fmt.Errorf("XML-RPC fault: %v", response.Fault.Value)
			}

			return nil
		},
		headers,
	)
}
