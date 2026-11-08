// Package simplepush sends notifications through Simplepush.
package simplepush

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

const apiURL = "https://api.simplepush.io/send"

// Provider sends each notification to one Simplepush key.
type Provider struct {
	*base.BaseClient
	key string
}

// NewProvider returns a provider for the Simplepush key.
func NewProvider(key string) *Provider {
	return &Provider{BaseClient: base.NewBaseClient(base.ServiceConfig("simplepush", apiURL)), key: key}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationSimplepush
}

func (*Provider) GetProviderName() string { return "simplepush" }

func (p *Provider) send(ctx context.Context, title, message string) error {
	form := url.Values{"key": {p.key}, "title": {title}, "msg": {message}}

	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	err := p.MakeRequestWithHeaders(ctx, http.MethodPost, "", strings.NewReader(form.Encode()), &resp, nil,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if err != nil {
		return err
	}

	if !strings.EqualFold(resp.Status, "OK") {
		return errors.New("simplepush: " + resp.Message)
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

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationSimplepush), nil
}

// TestConnection sends a test message: Simplepush has no call that only
// checks the key.
func (p *Provider) TestConnection(ctx context.Context) error {
	return p.send(ctx, "Test", "Connection test")
}
