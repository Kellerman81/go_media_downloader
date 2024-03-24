package apiexternal

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"github.com/gregdel/pushover"
)

// pushOverClient is a struct for interacting with the Pushover API.
// It contains the API key and a rate limiter.
type pushOverClient struct {
	// APIKey is the Pushover API key.
	APIKey string
	// Limiter limits requests to the Pushover API.
	Limiter *slidingwindow.Limiter
}

// NewPushOverClient initializes a new pushOverClient instance.
// apikey is the Pushover API key to use for sending messages.
// It initializes a rate limiter to limit messages to 3 per 10 seconds.
// If pushoverAPI is already initialized, it does nothing.
func NewPushOverClient(apikey string) {
	if pushoverAPI != nil {
		return
	}
	pushoverAPI = &pushOverClient{APIKey: apikey, Limiter: slidingwindow.NewLimiter(10*time.Second, 3)}
}

// GetPushOverKey returns the API key for the pushover client.
// It returns an empty string if the client has not been initialized.
func GetPushOverKey() string {
	if pushoverAPI == nil {
		return ""
	}
	return pushoverAPI.APIKey
}

// SendPushoverMessage sends a pushover message to the specified recipient.
// It handles rate limiting and retries before returning an error.
// messagetext is the message text, title is the message title, and
// recipientkey is the recipient's pushover key.
func SendPushoverMessage(messagetext string, title string, recipientkey string) error {
	if isok, waitfor := pushoverAPI.Limiter.Check(); !isok {
		waittime := 5 * time.Second
		for i := 0; i < 10; i++ {
			if waitfor == 0 {
				waitfor = waittime
			}
			if waitfor > waittime {
				break
			}
			time.Sleep(waitfor)
			if isok, waitfor = pushoverAPI.Limiter.Check(); isok {
				pushoverAPI.Limiter.AllowForce()
				break
			}
		}
		if !isok {
			if waitfor < waittime {
				pushoverAPI.Limiter.WaitTill(time.Now().Add(waittime))
			}
			return logger.ErrToWait
		}
		pushoverAPI.Limiter.AllowForce()
	}

	// Send the message to the recipient
	_, err := pushover.New(pushoverAPI.APIKey).SendMessage(pushover.NewMessageWithTitle(messagetext, title), pushover.NewRecipient(recipientkey))
	if err != nil {
		return err
	}
	return nil
}
