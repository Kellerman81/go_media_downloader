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

var (
	PushOverClients     = syncops.NewSyncMap[syncops.SyncAny](5)
	pushovermessagesapi = "https://api.pushover.net/1/messages.json"
)

// SendPushoverMessage sends a Pushover message with the given message, title, and recipient.
// The message must not be empty and must be less than 1024 characters.
// The title must be less than 250 characters.
// The function returns an error if the message or title are too long, or if there is an error sending the message.
func SendPushoverMessage(apikey, message, title, recipient string) error {
	if apikey == "" {
		return errors.New("apikey empty")
	}
	if message == "" {
		return errors.New("message empty")
	}
	if len(message) > 1024 {
		return errors.New("message too long")
	}
	if len(title) > 250 {
		return errors.New("title too long")
	}
	c := GetPushoverclient(apikey)
	ctx, ctxcancel := context.WithTimeout(c.Client.Ctx, c.Client.Timeout5)
	defer ctxcancel()
	ok, err := c.Client.checkLimiter(ctx, true)
	if !ok {
		if err == nil {
			return logger.ErrToWait
		}
		return err
	}

	data := url.Values{}
	data.Set("token", c.apikey)
	data.Set("user", recipient)
	data.Set("message", message)
	if title != "" {
		data.Set("title", title)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		pushovermessagesapi,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, pushovermessagesapi).
			Err(err).
			Msg("failed to get url")
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Client.client.Do(req)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrURL, pushovermessagesapi).
			Err(err).
			Msg("failed to process url")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.Client.addwait(req, resp)
		logger.Logtype("error", 1).
			Str(logger.StrURL, pushovermessagesapi).
			Msg("failed to process url")
		return logger.ErrToWait
	}
	return nil
}

// getPushoverclient returns a Pushover client instance for the given API key. If a client for the API key does not exist, it creates a new one and adds it to the cache.
func GetPushoverclient(apikey string) *LimitedAPIClient {
	if !PushOverClients.Check(apikey) {
		lim := slidingwindow.NewLimiter(10*time.Second, 3)
		d := LimitedAPIClient{
			apikey: apikey,
			Lim:    &lim,
		} // Client: pushover.New(apikey)}
		d.Client = newClient("pushover", true, false, &lim, false, nil, 30)
		syncops.QueueSyncMapAdd(syncops.MapTypePushover, apikey, syncops.SyncAny{Value: &d}, 0, false, 0)
		return &d
	}
	clt := PushOverClients.GetVal(apikey)
	if clt.Value == nil {
		logger.Logtype("debug", 0).Str("Apikey", apikey).Msg("NIL Client")
		return nil
	}
	if c, ok := clt.Value.(*LimitedAPIClient); ok {
		return c
	}
	logger.Logtype("debug", 0).Str("Apikey", apikey).Msg("Empty Client")
	return nil
}
