package apiexternal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

var GotifyClients = syncops.NewSyncMap[syncops.SyncAny](5)

// SendGotifyMessage sends a Gotify message with the given message, title, and server URL.
// The message must not be empty and must be less than 4096 characters.
// The title must be less than 256 characters.
// The function returns an error if the message or title are too long, or if there is an error sending the message.
func SendGotifyMessage(serverURL, token, message, title string) error {
	if serverURL == "" {
		return errors.New("server URL empty")
	}
	if token == "" {
		return errors.New("token empty")
	}
	if message == "" {
		return errors.New("message empty")
	}
	if len(message) > 4096 {
		return errors.New("message too long")
	}
	if len(title) > 256 {
		return errors.New("title too long")
	}

	c := GetGotifyClient(serverURL, token)
	ctx, ctxcancel := context.WithTimeout(c.Client.Ctx, c.Client.Timeout5)
	defer ctxcancel()
	ok, err := c.Client.checkLimiter(ctx, true)
	if !ok {
		if err == nil {
			return logger.ErrToWait
		}
		return err
	}

	// Prepare the message payload
	payload := map[string]any{
		"message": message,
	}
	if title != "" {
		payload["title"] = title
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Build the API URL
	apiURL := strings.TrimSuffix(serverURL, "/") + "/message?token=" + token

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, apiURL).
			Err(err).
			Msg("failed to create Gotify request")
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.client.Do(req)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, apiURL).
			Err(err).
			Msg("failed to send Gotify message")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.Client.addwait(req, resp)
		logger.Logtype("error", 1).
			Str(logger.StrURL, apiURL).
			Msg("failed to send Gotify message")
		return logger.ErrToWait
	}
	return nil
}

// GetGotifyClient returns a Gotify client instance for the given server URL and token.
// If a client for the server URL and token combination does not exist, it creates a new one and adds it to the cache.
func GetGotifyClient(serverURL, token string) *LimitedAPIClient {
	clientKey := serverURL + ":" + token
	if !GotifyClients.Check(clientKey) {
		lim := slidingwindow.NewLimiter(10*time.Second, 10) // 10 requests per 10 seconds
		d := LimitedAPIClient{
			apikey: token,
			Lim:    &lim,
		}
		d.Client = newClient("gotify", true, false, &lim, false, nil, 30)
		syncops.QueueSyncMapAdd(syncops.MapTypeGotify, clientKey, syncops.SyncAny{Value: &d}, 0, false, 0)
		return &d
	}
	clt := GotifyClients.GetVal(clientKey)
	if clt.Value == nil {
		logger.Logtype("debug", 0).Str("Apikey", clientKey).Msg("NIL Client")
		return nil
	}
	if c, ok := clt.Value.(*LimitedAPIClient); ok {
		return c
	}
	logger.Logtype("debug", 0).Str("Apikey", clientKey).Msg("Empty Client")
	return nil
}
