package apiexternal

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
func (c *RLHTTPClient) DoJson(req *http.Request, jsonobj interface{}) error {
	// Comment out the below 5 lines to turn off ratelimiting
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
			return errors.New("please wait")
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 408 || resp.StatusCode == 500 || resp.StatusCode == 503 || resp.StatusCode == 204 || resp.StatusCode == 522 {
		return errors.New(strconv.Itoa(resp.StatusCode))
	}
	errd := json.NewDecoder(resp.Body).Decode(&jsonobj)
	if errd != nil {
		return errd
	}
	return nil
}

//NewClient return http client with a ratelimiter
func NewClient(rl *rate.Limiter, rl2 *slidingwindow.Limiter) *RLHTTPClient {
	c := &RLHTTPClient{
		client: &http.Client{Timeout: 5 * time.Second,
			Transport: &http.Transport{MaxIdleConns: 20, MaxConnsPerHost: 10, DisableCompression: false, IdleConnTimeout: 20 * time.Second}},
		Ratelimiter:   rl,
		LimiterWindow: rl2,
	}
	return c
}
