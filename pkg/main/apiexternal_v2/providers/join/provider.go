// Package join sends notifications to devices through Join by joaoapps.
package join

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

const apiURL = "https://joinjoaomgcd.appspot.com/_ah/api"

// Provider pushes each notification to the configured devices.
type Provider struct {
	*base.BaseClient
	apiKey, deviceIDs string
}

// NewProvider returns a provider for apiKey. deviceIDs is a comma-separated
// list of device IDs; empty sends to all devices.
func NewProvider(apiKey, deviceIDs string) *Provider {
	return &Provider{BaseClient: base.NewBaseClient(base.ServiceConfig("join", apiURL)), apiKey: apiKey, deviceIDs: deviceIDs}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationJoin
}

func (*Provider) GetProviderName() string { return "join" }

type apiResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
}

func (p *Provider) call(ctx context.Context, endpoint string, q url.Values) error {
	q.Set("apikey", p.apiKey)

	var resp apiResponse
	if err := p.MakeRequest(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil, &resp, nil); err != nil {
		return err
	}

	if !resp.Success {
		return errors.New("join: " + resp.ErrorMessage)
	}

	return nil
}

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	q := url.Values{"title": {req.Title}, "text": {req.Message}}
	if p.deviceIDs != "" {
		q.Set("deviceIds", p.deviceIDs)
	} else {
		q.Set("deviceId", "group.all")
	}

	if err := p.call(ctx, "/messaging/v1/sendPush", q); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationJoin), nil
}

// TestConnection checks the API key by listing the devices.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.call(ctx, "/registration/v1/listDevices", url.Values{})
}
