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
	APIKey  string
	Limiter *rate.Limiter
}
type PushoverIdentity struct {
	Token string
	User  string
}

type PushoverMessage struct {
	token     string
	user      string
	text      string
	device    string
	title     string
	url       string
	urlTitle  string
	priority  string
	timestamp string
}

const pushoverMessageMax = 512
const pushoverURLMax = 500
const pushoverURLTitleMax = 50
const pushoverAPIURL = "https://api.pushover.net/1/messages.json"

var PushoverAPI *pushOverClient

func NewPushOverClient(apikey string) {
	rl := rate.New(3, 0, 10*time.Second) // 3 request every 10 seconds
	PushoverAPI = &pushOverClient{APIKey: apikey, Limiter: rl}
}

// returns a boolean indicating whether the message was valid. if the
// message was invalid, the offending struct member(s) was/were
// truncated.
func validatemessage(message *PushoverMessage) error {
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
	if messagelen > pushoverMessageMax {
		return errors.New("message length longer than " + strconv.Itoa(pushoverMessageMax) + " currently " + strconv.Itoa(messagelen))
	}

	if len(message.url) > pushoverURLMax {
		return errors.New("url length longer than " + strconv.Itoa(pushoverURLMax) + " currently " + strconv.Itoa(len(message.url)))
	}

	if len(message.urlTitle) > pushoverURLTitleMax {
		return errors.New("url title length longer than " + strconv.Itoa(pushoverURLTitleMax) + " currently " + strconv.Itoa(len(message.urlTitle)))
	}

	return nil
}

func getbody(message *PushoverMessage) url.Values {
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

	if len(message.urlTitle) > 0 {
		body.Add("url_title", message.urlTitle)
	}

	if len(message.priority) > 0 {
		body.Add("priority", message.priority)
	}

	if len(message.timestamp) > 0 {
		body.Add("timestamp", message.timestamp)
	}

	return body
}

func notify(message *PushoverMessage) error {
	err := validatemessage(message)
	if err != nil {
		message = nil
		return err
	}

	resp, err := http.PostForm(pushoverAPIURL, getbody(message))
	message = nil
	if err != nil {
		return errors.New("POST request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("server returned " + resp.Status)
	}
	return nil
}

func Authenticate(token string, user string) PushoverIdentity {
	return PushoverIdentity{token, user}
}

func (p *pushOverClient) SendMessage(messagetext string, title string, recipientkey string) error {
	if isok, _, waitfor := p.Limiter.Check(true, false); !isok {
		for i := 0; i < 10; i++ {
			if waitfor == 0 {
				waitfor = time.Duration(5) * time.Second
			}
			if waitfor > time.Duration(5) {
				break
			}
			time.Sleep(waitfor)
			if isok, _, waitfor = p.Limiter.Check(true, false); isok {
				p.Limiter.AllowForce()
				break
			}
		}
		if !isok {
			if waitfor < time.Duration(5) {
				p.Limiter.WaitTill(time.Now().Add(5 * time.Second))
			}
			return errPleaseWait
		}
		p.Limiter.AllowForce()
	}

	return notify(&PushoverMessage{
		token: p.APIKey,
		user:  recipientkey,
		text:  messagetext,
		title: title})
}
