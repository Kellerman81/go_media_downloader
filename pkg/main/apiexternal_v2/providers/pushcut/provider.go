// Package pushcut triggers a Pushcut notification on iOS devices.
package pushcut

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

const apiURL = "https://api.pushcut.io/v1"

// Provider sends to one notification defined in the Pushcut app.
type Provider struct {
	*base.BaseClient
	name string
}

// NewProvider returns a provider for the API key and the name of the
// notification defined in the Pushcut app.
func NewProvider(apiKey, notificationName string) *Provider {
	cfg := base.ServiceConfig("pushcut", apiURL)
	cfg.AuthType, cfg.APIKeyHeader, cfg.APIKey = base.AuthAPIKeyHeader, "API-Key", apiKey

	return &Provider{BaseClient: base.NewBaseClient(cfg), name: notificationName}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationPushcut
}

func (*Provider) GetProviderName() string { return "pushcut" }

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	body, err := json.Marshal(map[string]string{"title": req.Title, "text": req.Message})
	if err != nil {
		return nil, err
	}

	err = p.MakeRequest(ctx, http.MethodPost, "/notifications/"+url.PathEscape(p.name), bytes.NewReader(body), nil, nil)
	if err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationPushcut), nil
}

var errUnknownNotification = errors.New("pushcut has no notification with this name")

// TestConnection checks the API key and that the notification exists.
func (p *Provider) TestConnection(ctx context.Context) error {
	var defined []struct {
		ID string `json:"id"`
	}

	if err := p.MakeRequest(ctx, http.MethodGet, "/notifications", nil, &defined, nil); err != nil {
		return err
	}

	for _, n := range defined {
		if n.ID == p.name {
			return nil
		}
	}

	return errUnknownNotification
}
