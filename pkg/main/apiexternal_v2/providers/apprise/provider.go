package apprise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Apprise Provider - Universal notification gateway
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the NotificationProvider interface for Apprise.
type Provider struct {
	*base.BaseClient
	host    string
	port    int
	token   string
	baseURL string
	urls    []string // Notification service URLs
	useSSL  bool
}

// NewProvider creates a new Apprise notification provider.
func NewProvider(host string, port int, token string, urls []string, useSSL bool) *Provider {
	scheme := "http"
	if useSSL {
		scheme = "https"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	config := base.ClientConfig{
		Name:                    "apprise",
		BaseURL:                 baseURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthNone,
		RateLimitCalls:          10000, // 10000 calls per hour
		RateLimitSeconds:        3600,  // 1 hour
		RateLimitPer24h:         100000,
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
		token:      token,
		baseURL:    baseURL,
		urls:       urls,
		useSSL:     useSSL,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationApprise
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "apprise"
}

// SendNotification sends a notification via Apprise.
func (p *Provider) SendNotification(
	ctx context.Context,
	request apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	// Check for dynamic credentials in Options
	useBaseURL := p.baseURL

	appriseReq := appriseNotifyRequest{
		Title:  request.Title,
		Body:   request.Message,
		Type:   mapPriorityToType(int(request.Priority)),
		Format: "text",
	}

	// Use configured URLs or ones from options
	urls := p.urls
	if request.Options != nil {
		// Override base URL if provided (for dynamic server selection)
		if serverURL, ok := request.Options["server_url"]; ok && serverURL != "" {
			useBaseURL = serverURL
		}

		if optURLs, ok := request.Options["urls"]; ok {
			urls = strings.Split(optURLs, ",")
		}

		if tag, ok := request.Options["tag"]; ok {
			appriseReq.Tag = tag
		}

		if format, ok := request.Options["format"]; ok {
			appriseReq.Format = format
		}
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no notification URLs configured")
	}

	appriseReq.URLs = urls

	jsonData, err := json.Marshal(appriseReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := "/notify"
	if p.token != "" {
		endpoint = fmt.Sprintf("/notify/%s", p.token)
	}

	// If useBaseURL differs from p.baseURL, construct full URL as endpoint
	fullEndpoint := endpoint
	if useBaseURL != p.baseURL {
		fullEndpoint = useBaseURL + endpoint
	}

	// Use BaseClient's MakeRequest with custom response handler
	var appriseResponse apiexternal_v2.NotificationResponse

	err = p.MakeRequestWithHeaders(
		ctx,
		"POST",
		fullEndpoint,
		bytes.NewReader(jsonData),
		nil,
		func(resp *http.Response) error {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return fmt.Errorf("failed to read response: %w", readErr)
			}

			if resp.StatusCode != http.StatusOK {
				appriseResponse = apiexternal_v2.NotificationResponse{
					Success:   false,
					Timestamp: time.Now(),
					Provider:  "apprise",
					Error:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
				}

				return fmt.Errorf("apprise request failed with status %d", resp.StatusCode)
			}

			appriseResponse = apiexternal_v2.NotificationResponse{
				Success:   true,
				Timestamp: time.Now(),
				Provider:  "apprise",
			}

			return nil
		},
		map[string]string{"Content-Type": "application/json"},
	)

	return &appriseResponse, err
}

// TestConnection validates the Apprise server connectivity.
func (p *Provider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.BaseClient.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("apprise health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// mapPriorityToType converts priority to Apprise notification type.
func mapPriorityToType(priority int) string {
	switch priority {
	case 2:
		return "failure" // Emergency
	case 1:
		return "warning" // High
	case 0:
		return "info" // Normal
	case -1:
		return "success" // Low
	default:
		return "info"
	}
}

//
// Internal types
//

type appriseNotifyRequest struct {
	URLs   []string `json:"urls,omitempty"`
	Title  string   `json:"title,omitempty"`
	Body   string   `json:"body"`
	Type   string   `json:"type,omitempty"`
	Tag    string   `json:"tag,omitempty"`
	Format string   `json:"format,omitempty"`
}
