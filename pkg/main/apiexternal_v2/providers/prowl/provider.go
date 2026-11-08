// Package prowl sends notifications to iOS devices through Prowl.
package prowl

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

const apiURL = "https://api.prowlapp.com/publicapi"

// Prowl answers in XML; only the status code is used.
var headers = map[string]string{"Accept": "*/*", "Content-Type": "application/x-www-form-urlencoded"}

// Provider sends each notification as one Prowl event.
type Provider struct {
	*base.BaseClient
	apiKey string
}

// NewProvider returns a provider for the Prowl API key.
func NewProvider(apiKey string) *Provider {
	return &Provider{BaseClient: base.NewBaseClient(base.ServiceConfig("prowl", apiURL)), apiKey: apiKey}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationProwl
}

func (*Provider) GetProviderName() string { return "prowl" }

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	form := url.Values{
		"apikey":      {p.apiKey},
		"application": {"go_media_downloader"},
		"event":       {req.Title},
		"description": {req.Message},
	}

	err := p.MakeRequestWithHeaders(ctx, http.MethodPost, "/add", strings.NewReader(form.Encode()), nil, nil, headers)
	if err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationProwl), nil
}

// TestConnection checks the API key with Prowl's verify call.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.MakeRequestWithHeaders(ctx, http.MethodGet, "/verify?apikey="+url.QueryEscape(p.apiKey), nil, nil, nil, headers)
}
