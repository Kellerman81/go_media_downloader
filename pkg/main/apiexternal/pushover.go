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
)

var (
	pushOverClients     = logger.NewSyncMap[*client](5)
	pushovermessagesapi = "https://api.pushover.net/1/messages.json"
)

func (c *client) SendPushoverMessage(message, title, recipient string) error {
	if message == "" {
		return errors.New("message empty")
	}
	if len(message) > 1024 {
		return errors.New("message too long")
	}
	if len(title) > 250 {
		return errors.New("title too long")
	}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pushovermessagesapi, strings.NewReader(data.Encode()))
	if err != nil {
		logger.LogDynamicany1StringErr("error", "failed to get url", err, logger.StrURL, pushovermessagesapi)
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Client.client.Do(req)
	if err != nil {
		logger.LogDynamicany1StringErr("error", "failed to process url", err, logger.StrURL, pushovermessagesapi)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if c.Client.addwait(req, resp) {
			logger.LogDynamicany1String("error", "failed to process url", logger.StrURL, pushovermessagesapi)
		} else {
			logger.LogDynamicany1String("error", "failed to process url", logger.StrURL, pushovermessagesapi)
		}

		return logger.ErrToWait
	}
	return nil
}

// GetPushoverclient returns a Pushover client instance for the given API key. If a client for the API key does not exist, it creates a new one and adds it to the cache.
func GetPushoverclient(apikey string) *client {
	if !pushOverClients.Check(apikey) {
		d := &client{apikey: apikey, Lim: slidingwindow.NewLimiter(10*time.Second, 3)} // Client: pushover.New(apikey)}
		d.Client = NewClient("pushover", true, false, &d.Lim, false, nil, 30)
		pushOverClients.Add(apikey, d, 0, false, 0)
	}
	return pushOverClients.GetVal(apikey)
}
