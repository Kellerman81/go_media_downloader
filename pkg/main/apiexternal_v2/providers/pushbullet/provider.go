package pushbullet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Pushbullet Provider - Push notification service
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the NotificationProvider interface for Pushbullet.
type Provider struct {
	*base.BaseClient
	apiToken string
}

// NewProvider creates a new Pushbullet notification provider.
func NewProvider(apiToken string) *Provider {
	config := base.ClientConfig{
		Name:                    "pushbullet",
		BaseURL:                 "https://api.pushbullet.com/v2",
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthAPIKeyHeader,
		APIKeyHeader:            "Access-Token",
		APIKey:                  apiToken,
		RateLimitCalls:          500,  // 500 calls per hour
		RateLimitSeconds:        3600, // 1 hour
		RateLimitPer24h:         5000,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		apiToken:   apiToken,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationPushbullet
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "pushbullet"
}

// SendNotification sends a notification via Pushbullet.
func (p *Provider) SendNotification(
	ctx context.Context,
	request apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	// Check for dynamic credentials in Options
	useAPIToken := p.apiToken

	if request.Options != nil {
		// Override API token if provided (for dynamic credentials)
		if apiToken, ok := request.Options["api_token"]; ok && apiToken != "" {
			useAPIToken = apiToken
		}
	}

	pushReq := pushbulletPushRequest{
		Type:  "note",
		Title: request.Title,
		Body:  request.Message,
	}

	// Apply options
	if request.Options != nil {
		if deviceIden, ok := request.Options["device_iden"]; ok {
			pushReq.DeviceIden = deviceIden
		}

		if email, ok := request.Options["email"]; ok {
			pushReq.Email = email
		}

		if channelTag, ok := request.Options["channel_tag"]; ok {
			pushReq.ChannelTag = channelTag
		}
	}

	jsonData, err := json.Marshal(pushReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use BaseClient's MakeRequest with custom response handler
	var notifResponse apiexternal_v2.NotificationResponse

	err = p.MakeRequestWithHeaders(
		ctx,
		"POST",
		"https://api.pushbullet.com/v2/pushes",
		bytes.NewReader(jsonData),
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
					Provider:  "pushbullet",
					Error:     fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
				}

				return fmt.Errorf("pushbullet request failed with status %d", resp.StatusCode)
			}

			var pushResp pushbulletPushResponse
			if unmarshalErr := json.Unmarshal(body, &pushResp); unmarshalErr != nil {
				return fmt.Errorf("failed to decode response: %w", unmarshalErr)
			}

			notifResponse = apiexternal_v2.NotificationResponse{
				Success:   true,
				MessageID: pushResp.Iden,
				Timestamp: time.Now(),
				Provider:  "pushbullet",
			}

			return nil
		},
		map[string]string{
			"Content-Type": "application/json",
			"Access-Token": useAPIToken,
		},
	)

	return &notifResponse, err
}

// TestConnection validates the Pushbullet API token.
func (p *Provider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://api.pushbullet.com/v2/users/me",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Access-Token", p.apiToken)

	resp, err := p.BaseClient.GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf(
			"pushbullet auth check failed with status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return nil
}

//
// Internal types
//

type pushbulletPushRequest struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	DeviceIden string `json:"device_iden,omitempty"`
	Email      string `json:"email,omitempty"`
	ChannelTag string `json:"channel_tag,omitempty"`
}

type pushbulletPushResponse struct {
	Iden    string  `json:"iden"`
	Created float64 `json:"created"`
}
