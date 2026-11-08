// Package slack sends notifications to a Slack incoming webhook.
package slack

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// Provider posts each notification as one message with a bold title.
type Provider struct {
	*base.BaseClient
	webhookURL string
}

// NewProvider returns a provider for the webhook URL
// (https://hooks.slack.com/services/...).
func NewProvider(webhookURL string) *Provider {
	return &Provider{
		BaseClient: base.NewBaseClient(base.ServiceConfig("slack", "")),
		webhookURL: webhookURL,
	}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationSlack
}

func (*Provider) GetProviderName() string { return "slack" }

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// messageText is the mrkdwn text of a notification.
func messageText(title, message string) string {
	if title == "" {
		return escaper.Replace(message)
	}

	return "*" + escaper.Replace(title) + "*\n" + escaper.Replace(message)
}

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	body, err := json.Marshal(map[string]string{"text": messageText(req.Title, req.Message)})
	if err != nil {
		return nil, err
	}

	if err := p.MakeRequest(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body), nil, nil); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationSlack), nil
}

var errInvalidWebhook = errors.New("slack rejected the webhook")

// TestConnection posts an empty message. A valid webhook refuses it with
// "no_text" and posts nothing; an invalid one answers with another error.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.MakeRequestAcceptStatus(ctx, http.MethodPost, p.webhookURL, strings.NewReader("{}"),
		func(resp *http.Response) error {
			if resp.StatusCode < http.StatusBadRequest {
				return nil
			}

			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			if strings.Contains(string(b), "no_text") {
				return nil
			}

			return errors.Join(errInvalidWebhook, errors.New(strings.TrimSpace(string(b))))
		}, nil, http.StatusBadRequest)
}
