package pushover

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

//
// Pushover Provider - Push notification service
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the NotificationProvider interface for Pushover.
type Provider struct {
	*base.BaseClient
	apiToken string
	userKey  string
}

// NewProviderWithConfig creates a new Pushover provider with custom config.
func NewProviderWithConfig(config base.ClientConfig, apiToken, userKey string) *Provider {
	config.Name = "pushover"
	if config.BaseURL == "" {
		config.BaseURL = "https://api.pushover.net/1"
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		apiToken:   apiToken,
		userKey:    userKey,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationPushover
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "pushover"
}

// SendNotification sends a notification via Pushover.
func (p *Provider) SendNotification(
	ctx context.Context,
	request apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	// Check for dynamic credentials in Options
	useAPIToken := p.apiToken
	useUserKey := p.userKey

	if request.Options != nil {
		// Override API token if provided (for dynamic credentials)
		if apiToken, ok := request.Options["api_token"]; ok && apiToken != "" {
			useAPIToken = apiToken
		}

		// Override user key if provided (for dynamic credentials)
		if userKey, ok := request.Options["user_key"]; ok && userKey != "" {
			useUserKey = userKey
		}
	}

	// Build form data
	params := url.Values{
		"token":   {useAPIToken},
		"user":    {useUserKey},
		"title":   {request.Title},
		"message": {request.Message},
	}

	// Set priority
	priority := int(request.Priority)
	params.Set("priority", strconv.Itoa(priority))

	// Apply additional options
	if request.Options != nil {
		// Device
		if device, ok := request.Options["device"]; ok && device != "" {
			params.Set("device", device)
		}

		// Sound
		if sound, ok := request.Options["sound"]; ok && sound != "" {
			params.Set("sound", sound)
		}

		// URL
		if notifURL, ok := request.Options["url"]; ok && notifURL != "" {
			params.Set("url", notifURL)

			if urlTitle, ok := request.Options["url_title"]; ok && urlTitle != "" {
				params.Set("url_title", urlTitle)
			}
		}

		// HTML formatting
		if html, ok := request.Options["html"]; ok && html == "1" {
			params.Set("html", "1")
		}

		// Timestamp
		if timestamp, ok := request.Options["timestamp"]; ok && timestamp != "" {
			params.Set("timestamp", timestamp)
		}

		// Emergency priority specific options
		if priority == int(apiexternal_v2.PriorityEmergency) {
			if retry, ok := request.Options["retry"]; ok && retry != "" {
				params.Set("retry", retry)
			} else {
				params.Set("retry", "60") // Default retry interval
			}

			if expire, ok := request.Options["expire"]; ok && expire != "" {
				params.Set("expire", expire)
			} else {
				params.Set("expire", "3600") // Default expiration
			}

			if callback, ok := request.Options["callback"]; ok && callback != "" {
				params.Set("callback", callback)
			}
		}
	}

	// Make request using BaseClient with MakeRequestWithHeaders for stats tracking
	// Must use MakeRequestWithHeaders to set proper Content-Type for form data
	endpoint := "/messages.json"

	var pushoverResp pushoverResponse

	err := p.MakeRequestWithHeaders(
		ctx,
		"POST",
		endpoint,
		strings.NewReader(params.Encode()),
		nil, // Don't use automatic JSON decoding
		func(resp *http.Response) error {
			// Custom response handler for form-encoded request
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return errors.New(logger.JoinStrings("failed to read response: ", err.Error()))
			}

			if err := json.Unmarshal(body, &pushoverResp); err != nil {
				return errors.New(logger.JoinStrings("failed to decode response: ", err.Error()))
			}

			return nil
		},
		map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
	)
	if err != nil {
		return nil, err
	}

	if pushoverResp.Status != 1 {
		errorMsg := logger.JoinStringsSep(pushoverResp.Errors, ", ")

		return &apiexternal_v2.NotificationResponse{
			Success:   false,
			Timestamp: time.Now(),
			Provider:  "pushover",
			Error:     errorMsg,
		}, errors.New(logger.JoinStrings("pushover notification failed: ", errorMsg))
	}

	return &apiexternal_v2.NotificationResponse{
		Success:   true,
		MessageID: pushoverResp.Request,
		Timestamp: time.Now(),
		Provider:  "pushover",
	}, nil
}

// TestConnection validates the Pushover credentials.
func (p *Provider) TestConnection(ctx context.Context) error {
	params := url.Values{
		"token": {p.apiToken},
		"user":  {p.userKey},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		p.GetBaseURL()+"/users/validate.json",
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return errors.New(logger.JoinStrings("failed to create request: ", err.Error()))
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.BaseClient.GetHTTPClient().Do(req)
	if err != nil {
		return errors.New(logger.JoinStrings("request failed: ", err.Error()))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New(logger.JoinStrings("failed to read response: ", err.Error()))
	}

	var pushoverResp pushoverResponse
	if err := json.Unmarshal(body, &pushoverResp); err != nil {
		return errors.New(logger.JoinStrings("failed to decode response: ", err.Error()))
	}

	if pushoverResp.Status != 1 {
		errorMsg := logger.JoinStringsSep(pushoverResp.Errors, ", ")
		return errors.New(logger.JoinStrings("pushover validation failed: ", errorMsg))
	}

	return nil
}

// GetSounds retrieves available notification sounds.
func (p *Provider) GetSounds(ctx context.Context) (map[string]string, error) {
	type soundsResponse struct {
		Status int               `json:"status"`
		Sounds map[string]string `json:"sounds"`
		Errors []string          `json:"errors"`
	}

	endpoint := "/sounds.json?token=" + p.apiToken

	var soundsResp soundsResponse

	err := p.MakeRequest(
		ctx,
		"GET",
		endpoint,
		nil,
		&soundsResp,
		nil,
	)
	if err != nil {
		return nil, err
	}

	if soundsResp.Status != 1 {
		errorMsg := logger.JoinStringsSep(soundsResp.Errors, ", ")
		return nil, errors.New(logger.JoinStrings("pushover sounds request failed: ", errorMsg))
	}

	return soundsResp.Sounds, nil
}

// GetBaseURL returns the base URL for the Pushover API.
func (p *Provider) GetBaseURL() string {
	return "https://api.pushover.net/1"
}

//
// Internal types
//

type pushoverResponse struct {
	Status  int      `json:"status"`
	Request string   `json:"request"`
	Errors  []string `json:"errors"`
}
