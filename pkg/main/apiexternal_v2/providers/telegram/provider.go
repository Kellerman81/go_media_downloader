// Package telegram sends notifications through a Telegram bot.
package telegram

import (
	"bytes"
	"context"
	"errors"
	"html"
	"net/http"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// Provider sends each notification as one message to a chat.
type Provider struct {
	*base.BaseClient
	chatID string
}

// NewProvider returns a provider for the bot token from @BotFather and the
// chat, group or channel ID the bot posts to.
func NewProvider(botToken, chatID string) *Provider {
	return &Provider{
		BaseClient: base.NewBaseClient(base.ServiceConfig("telegram", "https://api.telegram.org/bot"+botToken)),
		chatID:     chatID,
	}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationTelegram
}

func (*Provider) GetProviderName() string { return "telegram" }

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (r *apiResponse) err() error {
	if r.OK {
		return nil
	}

	return errors.New("telegram: " + r.Description)
}

// messageText is the HTML text of a notification.
func messageText(title, message string) string {
	if title == "" {
		return html.EscapeString(message)
	}

	return "<b>" + html.EscapeString(title) + "</b>\n" + html.EscapeString(message)
}

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	body, err := json.Marshal(map[string]string{
		"chat_id":    p.chatID,
		"text":       messageText(req.Title, req.Message),
		"parse_mode": "HTML",
	})
	if err != nil {
		return nil, err
	}

	var resp apiResponse
	if err := p.MakeRequest(ctx, http.MethodPost, "/sendMessage", bytes.NewReader(body), &resp, nil); err != nil {
		return nil, err
	}

	if err := resp.err(); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationTelegram), nil
}

// TestConnection checks the bot token with getMe.
func (p *Provider) TestConnection(ctx context.Context) error {
	var resp apiResponse
	if err := p.MakeRequest(ctx, http.MethodGet, "/getMe", nil, &resp, nil); err != nil {
		return err
	}

	return resp.err()
}
