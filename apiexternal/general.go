package apiexternal

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"golang.org/x/time/rate"
)

//RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	client        *http.Client
	Ratelimiter   *rate.Limiter
	LimiterWindow *slidingwindow.Limiter
}

//Do dispatches the HTTP request to the network
func (c *RLHTTPClient) Do(req *http.Request) (*http.Response, []byte, error) {
	if !c.LimiterWindow.Allow() {
		isok := false
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			if c.LimiterWindow.Allow() {
				isok = true
				break
			}
		}
		if !isok {
			return nil, nil, errors.New("please wait")
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, body, nil
}

//NewClient return http client with a ratelimiter
func NewClient(rl *rate.Limiter, rl2 *slidingwindow.Limiter) *RLHTTPClient {
	c := &RLHTTPClient{
		client:        &http.Client{Timeout: 5 * time.Second},
		Ratelimiter:   rl,
		LimiterWindow: rl2,
	}
	return c
}
