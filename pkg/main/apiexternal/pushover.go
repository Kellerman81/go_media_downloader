package apiexternal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

// pushOverClient is a struct for interacting with the Pushover API.
// It contains the API key and a rate limiter.
type pushOverClient struct {
	// APIKey is the Pushover API key.
	APIKey string
	// Limiter limits requests to the Pushover API.
	//Client     *pushover.Pushover
	httpClient rlHTTPClient
}

var pushOverClients = logger.NewSynchedMap[*pushOverClient](5)
var pushovermessagesapi = "https://api.pushover.net/1/messages.json"
var strresponse = "response"

func (c *pushOverClient) SendPushoverMessage(message string, title string, recipient string) error {
	if len(message) == 0 {
		return errors.New("message empty")
	}
	if len(message) > 1024 {
		return errors.New("message too long")
	}
	if len(title) > 250 {
		return errors.New("title too long")
	}

	ok, err := c.httpClient.checkLimiter(true, 20, 1)
	if !ok {
		if err == nil {
			return logger.ErrToWait
		}
		return err
	}
	ctx, ctxcancel := context.WithTimeoutCause(c.httpClient.Ctx, c.httpClient.Timeout*5, err)
	defer ctxcancel()

	data := url.Values{}
	data.Set("token", c.APIKey)
	data.Set("user", recipient)
	data.Set("message", message)
	if title != "" {
		data.Set("title", title)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pushovermessagesapi, strings.NewReader(data.Encode()))
	if err != nil {
		logger.LogDynamicany("error", "failed to get url", err, &logger.StrURL, &pushovermessagesapi)
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.client.Do(req)
	if err != nil {
		logger.LogDynamicany("error", "failed to process url", err, &logger.StrURL, &pushovermessagesapi)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var b []byte
		if c.httpClient.addwait(req, resp) {
			b, _ = io.ReadAll(resp.Body)
			logger.LogDynamicany("error", "failed to process url", &logger.StrURL, &pushovermessagesapi, &strresponse, b)
		} else {
			logger.LogDynamicany("error", "failed to process url", &logger.StrURL, &pushovermessagesapi)
		}

		return logger.ErrToWait
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

// Getnewznabclient returns a Client for the given IndexersConfig.
// It checks if a client already exists for the given URL,
// and returns it if found. Otherwise creates a new client and caches it.
func GetPushoverclient(apikey string) *pushOverClient {
	if pushOverClients.Check(apikey) {
		return pushOverClients.Get(apikey)
	}
	limiter := slidingwindow.NewLimiter(10*time.Second, 3)
	d := pushOverClient{APIKey: apikey, httpClient: NewClient("pushover", true, false, &limiter, false, nil, 30)} //Client: pushover.New(apikey)}
	pushOverClients.Set(apikey, &d)
	return &d
}

// SendPushoverMessage sends a pushover message to the specified recipient.
// It handles rate limiting and retries before returning an error.
// messagetext is the message text, title is the message title, and
// recipientkey is the recipient's pushover key.
// func (p *pushOverClient) SendPushoverMessage(messagetext string, title string, recipientkey string) error {
// 	if isok, waitfor := p.Limiter.Check(); !isok {
// 		waittime := 5 * time.Second
// 		for i := 0; i < 10; i++ {
// 			if waitfor == 0 {
// 				waitfor = waittime
// 			}
// 			if waitfor > waittime {
// 				break
// 			}
// 			time.Sleep(waitfor)
// 			if isok, waitfor = p.Limiter.Check(); isok {
// 				p.Limiter.AllowForce()
// 				break
// 			}
// 		}
// 		if !isok {
// 			if waitfor < waittime {
// 				p.Limiter.WaitTill(time.Now().Add(waittime))
// 			}
// 			return logger.ErrToWait
// 		}
// 		p.Limiter.AllowForce()
// 	}

// 	// Send the message to the recipient
// 	_, err := p.Client.SendMessage(pushover.NewMessageWithTitle(messagetext, title), pushover.NewRecipient(recipientkey))
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
