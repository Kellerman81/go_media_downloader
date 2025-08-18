package apiexternal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

var (
	PushbulletClients     = syncops.NewSyncMap[syncops.SyncAny](5)
	pushbulletMessagesAPI = "https://api.pushbullet.com/v2/pushes"
)

// SendPushbulletMessage sends a Pushbullet push notification with the given message and title.
// The message must not be empty and must be less than 8192 characters.
// The title must be less than 140 characters.
// The function returns an error if the message or title are too long, or if there is an error sending the message.
func SendPushbulletMessage(token, message, title string) error {
	if token == "" {
		return errors.New("token empty")
	}
	if message == "" {
		return errors.New("message empty")
	}
	if len(message) > 8192 {
		return errors.New("message too long")
	}
	if len(title) > 140 {
		return errors.New("title too long")
	}

	c := GetPushbulletClient(token)
	ctx, ctxcancel := context.WithTimeout(c.Client.Ctx, c.Client.Timeout5)
	defer ctxcancel()
	ok, err := c.Client.checkLimiter(ctx, true)
	if !ok {
		if err == nil {
			return logger.ErrToWait
		}
		return err
	}

	// Prepare the push payload
	payload := map[string]any{
		"type": "note",
		"body": message,
	}
	if title != "" {
		payload["title"] = title
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		pushbulletMessagesAPI,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, pushbulletMessagesAPI).
			Err(err).
			Msg("failed to create Pushbullet request")
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Access-Token", token)

	resp, err := c.Client.client.Do(req)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, pushbulletMessagesAPI).
			Err(err).
			Msg("failed to send Pushbullet message")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.Client.addwait(req, resp)
		logger.Logtype("error", 1).
			Str(logger.StrURL, pushbulletMessagesAPI).
			Msg("failed to send Pushbullet message")
		return logger.ErrToWait
	}
	return nil
}

// GetPushbulletClient returns a Pushbullet client instance for the given token.
// If a client for the token does not exist, it creates a new one and adds it to the cache.
func GetPushbulletClient(token string) *LimitedAPIClient {
	if !PushbulletClients.Check(token) {
		lim := slidingwindow.NewLimiter(60*time.Second, 500) // 500 requests per minute
		d := LimitedAPIClient{
			apikey: token,
			Lim:    &lim,
		}
		d.Client = newClient("pushbullet", true, false, &lim, false, nil, 30)
		syncops.QueueSyncMapAdd(syncops.MapTypePushbullet, token, syncops.SyncAny{Value: &d}, 0, false, 0)
		return &d
	}
	clt := PushbulletClients.GetVal(token)
	if clt.Value == nil {
		logger.Logtype("debug", 0).Str("Apikey", token).Msg("NIL Client")
		return nil
	}
	if c, ok := clt.Value.(*LimitedAPIClient); ok {
		return c
	}
	logger.Logtype("debug", 0).Str("Apikey", token).Msg("Empty Client")
	return nil
}
