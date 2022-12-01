package apiexternal

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/rate"
)

type pushOverClient struct {
	ApiKey  string
	Limiter *rate.RateLimiter
}

var PushoverApi *pushOverClient

func NewPushOverClient(apikey string) {
	rl := rate.New(3, 0, 10*time.Second) // 3 request every 10 seconds
	PushoverApi = &pushOverClient{ApiKey: apikey, Limiter: rl}
}

const pushover_message_max = 512
const pushover_url_max = 500
const pushover_url_title_max = 50
const pushover_api_url = "https://api.pushover.net/1/messages.json"

type Pushover_Identity struct {
	Token string
	User  string
}

type Pushover_Message struct {
	token     string
	user      string
	text      string
	device    string
	title     string
	url       string
	url_title string
	priority  string
	timestamp string
}

// returns a boolean indicating whether the message was valid. if the
// message was invalid, the offending struct member(s) was/were
// truncated.
func validatemessage(message Pushover_Message) error {
	if message.token == "" {
		return errors.New("missing authentication token")
	}

	if message.user == "" {
		return errors.New("missing user key")
	}

	if message.text == "" {
		return errors.New("missing message")
	}

	messagelen := len(message.text) + len(message.title)
	if messagelen > pushover_message_max {
		return errors.New("message length longer than " + strconv.Itoa(pushover_message_max) + " currently " + strconv.Itoa(messagelen))
	}

	if len(message.url) > pushover_url_max {
		return errors.New("url length longer than " + strconv.Itoa(pushover_url_max) + " currently " + strconv.Itoa(len(message.url)))
	}

	if len(message.url_title) > pushover_url_title_max {
		return errors.New("url title length longer than " + strconv.Itoa(pushover_url_title_max) + " currently " + strconv.Itoa(len(message.url_title)))
	}

	return nil
}

func getbody(message Pushover_Message) url.Values {
	body := url.Values{}

	body.Add("token", message.token)
	body.Add("user", message.user)
	body.Add("message", message.text)

	if len(message.device) > 0 {
		body.Add("device", message.device)
	}

	if len(message.title) > 0 {
		body.Add("title", message.title)
	}

	if len(message.url) > 0 {
		body.Add("url", message.url)
	}

	if len(message.url_title) > 0 {
		body.Add("url_title", message.url_title)
	}

	if len(message.priority) > 0 {
		body.Add("priority", message.priority)
	}

	if len(message.timestamp) > 0 {
		body.Add("timestamp", message.timestamp)
	}

	return body
}

func notify(message Pushover_Message) error {
	err := validatemessage(message)
	if err != nil {
		return err
	}

	resp, err := http.PostForm(pushover_api_url, getbody(message))
	if err != nil {
		return errors.New("POST request failed")
	} else {
		defer resp.Body.Close()
	}

	if resp.StatusCode != 200 {
		return errors.New("server returned " + resp.Status)
	}
	return nil
}

func Authenticate(token string, user string) Pushover_Identity {
	return Pushover_Identity{token, user}
}

func (p *pushOverClient) SendMessage(messagetext string, title string, recipientkey string) error {
	if isok, waitfor := p.Limiter.Check(); !isok {
		for i := 0; i < 10; i++ {
			if waitfor == 0 {
				waitfor = time.Duration(5) * time.Second
			}
			if waitfor > time.Duration(5) {
				break
			}
			time.Sleep(waitfor)
			if isok, waitfor = p.Limiter.Check(); isok {
				p.Limiter.AllowForce()
				break
			}
		}
		if !isok {
			if waitfor < time.Duration(5) {
				p.Limiter.WaitTill(time.Now().Add(5 * time.Second))
			}
			return errPleaseWait
		} else {
			p.Limiter.AllowForce()
		}
	}

	msg := Pushover_Message{
		token: p.ApiKey,
		user:  recipientkey,
		text:  messagetext,
		title: title}
	return notify(msg)
}
