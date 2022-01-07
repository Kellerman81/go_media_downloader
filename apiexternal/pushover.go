package apiexternal

import (
	"errors"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"github.com/gregdel/pushover"
	"golang.org/x/time/rate"
)

type pushOverClient struct {
	ApiKey        string
	Limiter       *rate.Limiter
	LimiterWindow *slidingwindow.Limiter
}

var PushoverApi *pushOverClient

func NewPushOverClient(apikey string) {
	limiter, _ := slidingwindow.NewLimiter(10*time.Second, 3, func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })
	rl := rate.NewLimiter(rate.Every(10*time.Second), 3) // 3 request every 10 seconds
	PushoverApi = &pushOverClient{ApiKey: apikey, Limiter: rl, LimiterWindow: limiter}
}

func (p *pushOverClient) SendMessage(messagetext string, title string, recipientkey string) error {
	if !p.LimiterWindow.Allow() {
		isok := false
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			if p.LimiterWindow.Allow() {
				isok = true
				break
			}
		}
		if !isok {
			return errors.New("please wait")
		}
	}
	app := pushover.New(p.ApiKey)

	// Create a new recipient
	recipient := pushover.NewRecipient(recipientkey)

	// Create the message to send
	message := pushover.NewMessageWithTitle(messagetext, title)

	defer func() {
		message = nil
		recipient = nil
		app = nil
	}()
	// Send the message to the recipient
	_, errp := app.SendMessage(message, recipient)
	if errp != nil {
		return errp
	}
	return nil
}
