// Package sendgrid sends notifications as email through the SendGrid API.
package sendgrid

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/mail"
	"slices"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

const apiURL = "https://api.sendgrid.com/v3"

type address struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// Provider mails each notification from one verified sender.
type Provider struct {
	*base.BaseClient
	from address
	to   []address
}

// NewProvider returns a provider for the API key, the verified sender and
// the recipients.
func NewProvider(apiKey, from string, to []string) (*Provider, error) {
	f, err := mail.ParseAddress(from)
	if err != nil {
		return nil, err
	}

	p := &Provider{from: address{Email: f.Address, Name: f.Name}}
	for _, t := range to {
		a, err := mail.ParseAddress(t)
		if err != nil {
			return nil, err
		}

		p.to = append(p.to, address{Email: a.Address, Name: a.Name})
	}

	cfg := base.ServiceConfig("sendgrid", apiURL)
	cfg.AuthType, cfg.APIKeyHeader, cfg.APIKey = base.AuthAPIKeyHeader, "Authorization", "Bearer "+apiKey
	p.BaseClient = base.NewBaseClient(cfg)

	return p, nil
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationSendGrid
}

func (*Provider) GetProviderName() string { return "sendgrid" }

type content struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type personalization struct {
	To []address `json:"to"`
}

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	body, err := json.Marshal(map[string]any{
		"personalizations": []personalization{{To: p.to}},
		"from":             p.from,
		"subject":          req.Title,
		"content":          []content{{Type: "text/plain", Value: req.Message}},
	})
	if err != nil {
		return nil, err
	}

	if err := p.MakeRequest(ctx, http.MethodPost, "/mail/send", bytes.NewReader(body), nil, nil); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationSendGrid), nil
}

var errNoSendScope = errors.New("the sendgrid API key lacks the mail.send permission")

// TestConnection checks that the API key may send mail.
func (p *Provider) TestConnection(ctx context.Context) error {
	var resp struct {
		Scopes []string `json:"scopes"`
	}

	if err := p.MakeRequest(ctx, http.MethodGet, "/scopes", nil, &resp, nil); err != nil {
		return err
	}

	if !slices.Contains(resp.Scopes, "mail.send") {
		return errNoSendScope
	}

	return nil
}
