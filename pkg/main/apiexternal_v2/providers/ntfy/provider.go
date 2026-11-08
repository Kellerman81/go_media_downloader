// Package ntfy publishes notifications to an ntfy topic (ntfy.sh or
// self-hosted).
package ntfy

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// DefaultServer is used when no server URL is configured.
const DefaultServer = "https://ntfy.sh"

// Provider publishes each notification as one message.
type Provider struct {
	*base.BaseClient
	topic string
}

// NewProvider returns a provider for topic on serverURL (DefaultServer when
// empty). accessToken is optional; "user:password" is sent as basic auth,
// anything else as a bearer token.
func NewProvider(serverURL, topic, accessToken string) *Provider {
	if serverURL == "" {
		serverURL = DefaultServer
	}

	cfg := base.ServiceConfig("ntfy", strings.TrimRight(serverURL, "/"))
	if user, pass, ok := strings.Cut(accessToken, ":"); ok {
		cfg.AuthType, cfg.Username, cfg.Password = base.AuthBasic, user, pass
	} else if accessToken != "" {
		cfg.AuthType, cfg.APIKeyHeader, cfg.APIKey = base.AuthAPIKeyHeader, "Authorization", "Bearer "+accessToken
	}

	return &Provider{BaseClient: base.NewBaseClient(cfg), topic: topic}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationNtfy
}

func (*Provider) GetProviderName() string { return "ntfy" }

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	// JSON publishing keeps non-ASCII titles intact; the Title header would not.
	body, err := json.Marshal(map[string]string{"topic": p.topic, "title": req.Title, "message": req.Message})
	if err != nil {
		return nil, err
	}

	if err := p.MakeRequest(ctx, http.MethodPost, "/", bytes.NewReader(body), nil, nil); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationNtfy), nil
}

var errUnhealthy = errors.New("ntfy server reports itself unhealthy")

// TestConnection checks that the server is reachable and healthy. It does
// not check the token's access to the topic.
func (p *Provider) TestConnection(ctx context.Context) error {
	var health struct {
		Healthy bool `json:"healthy"`
	}

	if err := p.MakeRequest(ctx, http.MethodGet, "/v1/health", nil, &health, nil); err != nil {
		return err
	}

	if !health.Healthy {
		return errUnhealthy
	}

	return nil
}
