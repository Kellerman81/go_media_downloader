package apiexternal

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

var AppriseClients = syncops.NewSyncMap[syncops.SyncAny](5)

// SendAppriseMessage sends a message via Apprise API server with the given message, title, and URLs.
// The message must not be empty.
// The function returns an error if there is an error sending the message.
func SendAppriseMessage(serverURL, message, title, urls string) error {
	if serverURL == "" {
		return errors.New("server URL empty")
	}
	if message == "" {
		return errors.New("message empty")
	}
	if urls == "" {
		return errors.New("notification URLs empty")
	}

	c := GetAppriseClient(serverURL)
	ctx, ctxcancel := context.WithTimeout(c.Client.Ctx, c.Client.Timeout5)
	defer ctxcancel()
	ok, err := c.Client.checkLimiter(ctx, true)
	if !ok {
		if err == nil {
			return logger.ErrToWait
		}
		return err
	}

	// Prepare form data
	data := url.Values{}
	data.Set("body", message)
	data.Set("urls", urls)
	if title != "" {
		data.Set("title", title)
	}

	// Build the API URL
	apiURL := strings.TrimSuffix(serverURL, "/") + "/notify"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, apiURL).
			Err(err).
			Msg("failed to create Apprise request")
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Client.client.Do(req)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, apiURL).
			Err(err).
			Msg("failed to send Apprise message")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.Client.addwait(req, resp)
		logger.Logtype("error", 1).
			Str(logger.StrURL, apiURL).
			Msg("failed to send Apprise message")
		return logger.ErrToWait
	}
	return nil
}

// GetAppriseClient returns an Apprise client instance for the given server URL.
// If a client for the server URL does not exist, it creates a new one and adds it to the cache.
func GetAppriseClient(serverURL string) *LimitedAPIClient {
	if !AppriseClients.Check(serverURL) {
		lim := slidingwindow.NewLimiter(10*time.Second, 20) // 20 requests per 10 seconds
		d := LimitedAPIClient{
			apikey: "", // No API key needed for Apprise
			Lim:    &lim,
		}
		d.Client = newClient("apprise", true, false, &lim, false, nil, 30)
		syncops.QueueSyncMapAdd(syncops.MapTypeApprise, serverURL, syncops.SyncAny{Value: &d}, 0, false, 0)
		return &d
	}
	clt := AppriseClients.GetVal(serverURL)
	if clt.Value == nil {
		logger.Logtype("debug", 0).Str("Apikey", serverURL).Msg("NIL Client")
		return nil
	}
	if c, ok := clt.Value.(*LimitedAPIClient); ok {
		return c
	}
	logger.Logtype("debug", 0).Str("Apikey", serverURL).Msg("Empty Client")
	return nil
}

// GetAppriseClients returns the apprise clients SyncMap for syncops registration
func GetAppriseClients() *syncops.SyncMap[syncops.SyncAny] {
	return AppriseClients
}
