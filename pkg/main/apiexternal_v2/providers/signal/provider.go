// Package signal sends notifications through a signal-cli-rest-api server
// (https://github.com/bbernhard/signal-cli-rest-api).
package signal

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/goccy/go-json"
)

// Provider sends each notification from the sender number to one recipient.
type Provider struct {
	*base.BaseClient
	sender, recipient string
}

// NewProvider returns a provider for the server at serverURL (credentials in
// the URL are sent as basic auth), the registered sender number and the
// recipient (a phone number or a group ID).
func NewProvider(serverURL, sender, recipient string) *Provider {
	cfg, clean := base.WithURLCredentials(base.ServiceConfig("signal", ""), serverURL)
	cfg.BaseURL = strings.TrimRight(clean, "/")

	return &Provider{BaseClient: base.NewBaseClient(cfg), sender: sender, recipient: recipient}
}

func (*Provider) GetProviderType() apiexternal_v2.NotificationProviderType {
	return apiexternal_v2.NotificationSignal
}

func (*Provider) GetProviderName() string { return "signal" }

func (p *Provider) SendNotification(
	ctx context.Context,
	req apiexternal_v2.NotificationRequest,
) (*apiexternal_v2.NotificationResponse, error) {
	text := req.Message
	if req.Title != "" {
		text = req.Title + "\n" + req.Message
	}

	body, err := json.Marshal(map[string]any{"message": text, "number": p.sender, "recipients": []string{p.recipient}})
	if err != nil {
		return nil, err
	}

	if err := p.MakeRequest(ctx, http.MethodPost, "/v2/send", bytes.NewReader(body), nil, nil); err != nil {
		return nil, err
	}

	return apiexternal_v2.SentNotification(apiexternal_v2.NotificationSignal), nil
}

var errSenderNotRegistered = errors.New("the sender number is not registered on the signal server")

// TestConnection checks that the server has the sender number registered.
func (p *Provider) TestConnection(ctx context.Context) error {
	var accounts []string
	if err := p.MakeRequest(ctx, http.MethodGet, "/v1/accounts", nil, &accounts, nil); err != nil {
		return err
	}

	if !slices.Contains(accounts, p.sender) {
		return errSenderNotRegistered
	}

	return nil
}
