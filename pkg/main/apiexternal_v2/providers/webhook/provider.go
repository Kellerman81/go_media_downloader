// Package webhook posts notifications as JSON to any URL.
package webhook

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// Payload is the JSON body of every webhook call.
type Payload struct {
	EventType string    `json:"eventType"` // "Notification", or "Test" from TestConnection
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Provider posts a Payload to the configured URL.
type Provider struct {
	*base.BaseClient
	url string
}

// NewProvider returns a provider for rawURL. Credentials in the URL
// ("https://user:pass@host/hook") are sent as basic auth.
func NewProvider(rawURL string) *Provider {
	cfg, clean := base.WithURLCredentials(base.ServiceConfig("webhook", ""), rawURL)

	return &Provider{BaseClient: base.NewBaseClient(cfg), url: clean}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationWebhook
}

func (*Provider) GetProviderName() string { return "webhook" }

func (p *Provider) post(ctx context.Context, payload Payload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return p.MakeRequest(ctx, http.MethodPost, p.url, bytes.NewReader(body), nil, nil)
}

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	err := p.post(ctx, Payload{EventType: "Notification", Title: req.Title, Message: req.Message, Timestamp: time.Now()})
	if err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationWebhook), nil
}

// TestConnection posts a payload with eventType "Test", as a generic URL
// has no other way to check it.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.post(ctx, Payload{EventType: "Test", Title: "Test", Message: "Connection test", Timestamp: time.Now()})
}
