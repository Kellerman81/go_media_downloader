// Package mailgun sends notifications as email through the Mailgun API.
package mailgun

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

// DefaultAPI is the US region; EU domains use https://api.eu.mailgun.net/v3.
const DefaultAPI = "https://api.mailgun.net/v3"

// Provider mails each notification from one sending domain.
type Provider struct {
	*base.BaseClient
	domain, from string
	to           []string
}

var errNoDomain = errors.New("mailgun: the from address has no domain")

// NewProvider returns a provider for the API key. The sending domain is the
// domain of the from address. apiURL selects the region (DefaultAPI when
// empty).
func NewProvider(apiKey, apiURL, from string, to []string) (*Provider, error) {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return nil, err
	}

	_, domain, ok := strings.Cut(addr.Address, "@")
	if !ok || domain == "" {
		return nil, errNoDomain
	}

	if apiURL == "" {
		apiURL = DefaultAPI
	}

	cfg := base.ServiceConfig("mailgun", strings.TrimRight(apiURL, "/"))
	cfg.AuthType, cfg.Username, cfg.Password = base.AuthBasic, "api", apiKey

	return &Provider{BaseClient: base.NewBaseClient(cfg), domain: domain, from: from, to: to}, nil
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationMailgun
}

func (*Provider) GetProviderName() string { return "mailgun" }

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	form := url.Values{"from": {p.from}, "to": p.to, "subject": {req.Title}, "text": {req.Message}}

	err := p.MakeRequestWithHeaders(ctx, http.MethodPost, "/"+p.domain+"/messages", strings.NewReader(form.Encode()),
		nil, nil, map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationMailgun), nil
}

// TestConnection checks the API key and that the sending domain exists.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.MakeRequest(ctx, http.MethodGet, "/domains/"+url.PathEscape(p.domain), nil, nil, nil)
}
