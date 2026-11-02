package rtorrent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// XML-RPC structures for rTorrent
//

type xmlParams struct {
	Params []xmlParam `xml:"param"`
}

type xmlParam struct {
	Value xmlValue `xml:"value"`
}

type xmlValue struct {
	String string     `xml:"string,omitempty"`
	Int    int        `xml:"int,omitempty"`
	I4     string     `xml:"i4,omitempty"`
	I8     string     `xml:"i8,omitempty"`
	Bool   bool       `xml:"boolean,omitempty"`
	Base64 string     `xml:"base64,omitempty"`
	Array  *xmlArray  `xml:"array,omitempty"`
	Struct *xmlStruct `xml:"struct,omitempty"`
}

type xmlStruct struct {
	Members []xmlMember `xml:"member"`
}

type xmlMember struct {
	Name  string   `xml:"name"`
	Value xmlValue `xml:"value"`
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

// String renders a fault as "faultCode <code>: <faultString>". rTorrent
// sends the code as <i4> or <i8>, other servers as <int>.
func (f *xmlFault) String() string {
	var code, msg string

	if f.Value.Struct != nil {
		for _, m := range f.Value.Struct.Members {
			switch m.Name {
			case "faultCode":
				switch {
				case m.Value.I4 != "":
					code = m.Value.I4
				case m.Value.I8 != "":
					code = m.Value.I8
				default:
					code = strconv.Itoa(m.Value.Int)
				}
			case "faultString":
				msg = m.Value.String
			}
		}
	}

	return logger.JoinStrings("faultCode ", code, ": ", msg)
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
func (*Provider) GetProviderType() apiexternal_v2.DownloadProviderType {
	return apiexternal_v2.DownloadProviderRTorrent
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
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
		Hash:     btihFromURL(request.URL),
		Message:  "Torrent added successfully",
		Provider: "rtorrent",
	}, nil
}

// btihFromURL returns the info hash of a magnet/torrent URL in rTorrent's
// uppercase hex format; rTorrent's load.* calls return no hash.
func btihFromURL(u string) string {
	return base.InfoHashFromURL(u)
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

// RemoveTorrent removes a torrent from rTorrent. d.erase removes the torrent
// from the session but leaves its data on disk; rTorrent offers no XML-RPC
// call that deletes the data short of running a shell command via execute, so
// deleteFiles=true is refused rather than silently keeping the files.
func (p *Provider) RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	if deleteFiles {
		return errors.New("rtorrent: removing a torrent together with its data is not supported")
	}

	if err := p.makeXMLRPCCall(ctx, "d.erase", []any{hash}); err != nil {
		return err
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("provider", p.GetProviderName()).
		Str("hash", hash).
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
		return errors.New(logger.JoinStrings("not connected: ", status.Message))
	}

	return nil
}

//
// Helper Methods
//

// GetHTTPClient returns an HTTP client for this provider.
func (*Provider) GetHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// makeXMLRPCCall makes an XML-RPC call to rTorrent.
// encodeXMLRPCRequest builds a methodCall document. Every value is written with
// its explicit type element: rTorrent rejects an empty <value></value> ("empty
// value element"), which is what an omitempty struct produces for the empty
// target string load.* calls take as their first parameter.
func encodeXMLRPCRequest(method string, params []any) ([]byte, error) {
	var b strings.Builder

	b.WriteString(xml.Header)
	b.WriteString("<methodCall><methodName>")

	if err := xml.EscapeText(&b, []byte(method)); err != nil {
		return nil, err
	}

	b.WriteString("</methodName><params>")

	for _, param := range params {
		b.WriteString("<param><value>")

		switch t := param.(type) {
		case int:
			b.WriteString("<i8>" + strconv.Itoa(t) + "</i8>")
		case bool:
			if t {
				b.WriteString("<boolean>1</boolean>")
			} else {
				b.WriteString("<boolean>0</boolean>")
			}
		case []byte:
			b.WriteString("<base64>" + base64.StdEncoding.EncodeToString(t) + "</base64>")
		default:
			b.WriteString("<string>")

			if err := xml.EscapeText(&b, []byte(fmt.Sprint(t))); err != nil {
				return nil, err
			}

			b.WriteString("</string>")
		}

		b.WriteString("</value></param>")
	}

	b.WriteString("</params></methodCall>")

	return []byte(b.String()), nil
}

func (p *Provider) makeXMLRPCCall(ctx context.Context, method string, params []any) error {
	xmlData, err := encodeXMLRPCRequest(method, params)
	if err != nil {
		return errors.New(logger.JoinStrings("failed to marshal XML-RPC request: ", err.Error()))
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

				return errors.New(
					logger.JoinStrings(
						"HTTP request failed with status ",
						strconv.Itoa(resp.StatusCode),
						": ",
						string(body),
					),
				)
			}

			// Read and parse XML response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return errors.New(logger.JoinStrings("failed to read response body: ", err.Error()))
			}

			var response xmlRPCResponse
			if err := xml.Unmarshal(body, &response); err != nil {
				return errors.New(
					logger.JoinStrings("failed to unmarshal XML-RPC response: ", err.Error()),
				)
			}

			// Check for fault
			if response.Fault != nil {
				return errors.New(
					logger.JoinStrings("XML-RPC fault: ", response.Fault.String()),
				)
			}

			return nil
		},
		headers,
	)
}
