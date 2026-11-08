// Package pushsafer sends notifications through Pushsafer.
package pushsafer

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

const apiURL = "https://www.pushsafer.com/api"

// Provider sends each notification to the configured devices.
type Provider struct {
	*base.BaseClient
	privateKey, devices string
}

// NewProvider returns a provider for the private or alias key. devices is a
// device or group ID ("12|34" for several); empty sends to all devices.
func NewProvider(privateKey, devices string) *Provider {
	if devices == "" {
		devices = "a"
	}

	return &Provider{BaseClient: base.NewBaseClient(base.ServiceConfig("pushsafer", apiURL)), privateKey: privateKey, devices: devices}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationPushsafer
}

func (*Provider) GetProviderName() string { return "pushsafer" }

func (p *Provider) send(ctx context.Context, title, message string) error {
	form := url.Values{"k": {p.privateKey}, "d": {p.devices}, "t": {title}, "m": {message}}

	// Pushsafer reports errors with status 0 in a 200 response.
	var resp struct {
		Status int    `json:"status"`
		Error  string `json:"error"`
	}

	err := p.MakeRequestWithHeaders(ctx, http.MethodPost, "", strings.NewReader(form.Encode()), &resp, nil,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if err != nil {
		return err
	}

	if resp.Status != 1 {
		return errors.New("pushsafer: " + resp.Error)
	}

	return nil
}

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	if err := p.send(ctx, req.Title, req.Message); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationPushsafer), nil
}

// TestConnection sends a test message: Pushsafer has no call that only
// checks the key.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.send(ctx, "Test", "Connection test")
}
