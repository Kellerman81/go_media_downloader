// Package discord sends notifications to a Discord channel webhook.
package discord

import (
	"bytes"
	"context"
	"net/http"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// Provider posts each notification as one embed.
type Provider struct {
	*base.BaseClient
	webhookURL string
}

// NewProvider returns a provider for the webhook URL
// (https://discord.com/api/webhooks/<id>/<token>).
func NewProvider(webhookURL string) *Provider {
	return &Provider{
		BaseClient: base.NewBaseClient(base.ServiceConfig("discord", "")),
		webhookURL: webhookURL,
	}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationDiscord
}

func (*Provider) GetProviderName() string { return "discord" }

type embed struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description"`
}

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	body, err := json.Marshal(map[string][]embed{"embeds": {{Title: req.Title, Description: req.Message}}})
	if err != nil {
		return nil, err
	}

	if err := p.MakeRequest(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body), nil, nil); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationDiscord), nil
}

// TestConnection reads the webhook, which Discord answers without posting.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.MakeRequest(ctx, http.MethodGet, p.webhookURL, nil, nil, nil)
}
