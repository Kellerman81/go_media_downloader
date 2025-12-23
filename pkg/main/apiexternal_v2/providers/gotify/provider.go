package gotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Gotify Provider - Self-hosted push notification service
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the NotificationProvider interface for Gotify.
type Provider struct {
	*base.BaseClient
	host    string
	port    int
	token   string
	baseURL string
	useSSL  bool
}

// NewProviderWithConfig creates a new Gotify provider with custom config.
func NewProviderWithConfig(
	config base.ClientConfig,
	host string,
	port int,
	token string,
	useSSL bool,
) *Provider {
	config.Name = "gotify"

	scheme := "http"
	if useSSL {
		scheme = "https"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	config.BaseURL = baseURL

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		host:       host,
		port:       port,
		token:      token,
		baseURL:    baseURL,
		useSSL:     useSSL,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationGotify
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "gotify"
}

// SendNotification sends a notification via Gotify.
func (p *Provider) SendNotification(
	ctx context.Context,
	request apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	// Map priority (Pushover -2 to 2 scale to Gotify 0-10 scale)
	priority := mapPriorityToGotify(int(request.Priority))

	// Prepare message data
	messageData := gotifyMessage{
		Title:    request.Title,
		Message:  request.Message,
		Priority: priority,
	}

	// Check for dynamic credentials in Options
	useBaseURL := p.baseURL
	useToken := p.token

	// Apply additional options
	if request.Options != nil {
		// Override base URL if provided (for dynamic server selection)
		if serverURL, ok := request.Options["server_url"]; ok && serverURL != "" {
			useBaseURL = serverURL
		}

		// Override token if provided (for dynamic credentials)
		if token, ok := request.Options["token"]; ok && token != "" {
			useToken = token
		}

		// Override priority if specified
		if optPriority, ok := request.Options["priority"]; ok {
			if priorityInt, err := strconv.Atoi(optPriority); err == nil {
				messageData.Priority = priorityInt
			}
		}

		// Extras (additional data)
		if extras, ok := request.Options["extras"]; ok {
			// Parse extras if it's a JSON string
			var extrasMap map[string]any
			if err := json.Unmarshal([]byte(extras), &extrasMap); err == nil {
				messageData.Extras = extrasMap
			}
		}
	}

	jsonData, err := json.Marshal(messageData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Make request
	endpoint := "/message"
	params := url.Values{
		"token": {useToken},
	}

	// Build full endpoint with query parameters (for custom baseURL support)
	fullEndpoint := endpoint + "?" + params.Encode()
	if useBaseURL != p.baseURL {
		fullEndpoint = useBaseURL + fullEndpoint
	}

	// Use BaseClient's MakeRequest with custom response handler
	var notifResponse apiexternal_v2.NotificationResponse

	err = p.MakeRequestWithHeaders(
		ctx,
		"POST",
		fullEndpoint,
		strings.NewReader(string(jsonData)),
		nil,
		func(resp *http.Response) error {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return fmt.Errorf("failed to read response: %w", readErr)
			}

			if resp.StatusCode != http.StatusOK {
				notifResponse = apiexternal_v2.NotificationResponse{
					Success:   false,
					Timestamp: time.Now(),
					Provider:  "gotify",
					Error:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
				}

				return fmt.Errorf("gotify request failed with status %d", resp.StatusCode)
			}

			var gotifyResp gotifyMessageResponse
			if unmarshalErr := json.Unmarshal(body, &gotifyResp); unmarshalErr != nil {
				return fmt.Errorf("failed to decode response: %w", unmarshalErr)
			}

			notifResponse = apiexternal_v2.NotificationResponse{
				Success:   true,
				MessageID: strconv.Itoa(gotifyResp.ID),
				Timestamp: time.Now(),
				Provider:  "gotify",
			}

			return nil
		},
		map[string]string{"Content-Type": "application/json"},
	)

	return &notifResponse, err
}

// TestConnection validates the Gotify credentials and connectivity.
func (p *Provider) TestConnection(ctx context.Context) error {
	// Get server version to test connectivity
	endpoint := "/version"
	requestURL := p.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.BaseClient.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf(
			"gotify version check failed with status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return nil
}

// GetVersion retrieves the Gotify server version.
func (p *Provider) GetVersion(ctx context.Context) (*GotifyVersion, error) {
	endpoint := "/version"

	var version gotifyVersion

	err := p.MakeRequest(
		ctx,
		"GET",
		endpoint,
		nil,
		&version,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("gotify version request failed: %w", err)
	}

	return &GotifyVersion{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,
	}, nil
}

//
// Helper Functions
//

// mapPriorityToGotify maps standard notification priority (-2 to 2) to Gotify priority (0-10).
func mapPriorityToGotify(priority int) int {
	switch priority {
	case -2: // Lowest
		return 0
	case -1: // Low
		return 2
	case 0: // Normal
		return 5
	case 1: // High
		return 8
	case 2: // Emergency
		return 10
	default:
		return 5 // Default to normal
	}
}

//
// Internal types
//

type gotifyMessage struct {
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Priority int            `json:"priority"`
	Extras   map[string]any `json:"extras,omitempty"`
}

type gotifyMessageResponse struct {
	ID       int            `json:"id"`
	AppID    int            `json:"appid"`
	Message  string         `json:"message"`
	Title    string         `json:"title"`
	Priority int            `json:"priority"`
	Date     string         `json:"date"`
	Extras   map[string]any `json:"extras,omitempty"`
}

type gotifyVersion struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

//
// Public types
//

// GotifyVersion represents Gotify server version information.
type GotifyVersion struct {
	Version   string
	Commit    string
	BuildDate string
}
