package apiexternal

import (
	"errors"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"github.com/gregdel/pushover"
	"golang.org/x/time/rate"
)

type PushOverClient struct {
	ApiKey        string
	Limiter       *rate.Limiter
	LimiterWindow *slidingwindow.Limiter
}

var PushoverApi PushOverClient

func NewPushOverClient(apikey string) {
	limiter, _ := slidingwindow.NewLimiter(10*time.Second, 3, func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })
	rl := rate.NewLimiter(rate.Every(10*time.Second), 3) // 3 request every 10 seconds
	PushoverApi = PushOverClient{ApiKey: apikey, Limiter: rl, LimiterWindow: limiter}
}

func (p PushOverClient) SendMessage(messagetext string, title string, recipientkey string) error {
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
	// ctx := context.Background()
	// _, err := p.LimiterWindow.Limit(ctx)
	// if err == limiters.ErrLimitExhausted {
	// 	return errors.New("try again later")
	// } else if err != nil {
	// 	// The limiter failed. This error should be logged and examined.
	// 	log.Println(err)
	// 	return errors.New("internal error")
	// }
	// err := p.Limiter.Wait(ctx) // This is a blocking call. Honors the rate limit
	// if err != nil {
	// 	return err
	// }
	app := pushover.New(p.ApiKey)

	// Create a new recipient
	recipient := pushover.NewRecipient(recipientkey)

	// Create the message to send
	message := pushover.NewMessageWithTitle(messagetext, title)

	// Send the message to the recipient
	_, errp := app.SendMessage(message, recipient)
	if errp != nil {
		return errp
	}
	return nil
}
