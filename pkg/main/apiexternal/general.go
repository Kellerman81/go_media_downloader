package apiexternal

import (
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/net/html/charset"
	"golang.org/x/time/rate"
)

//RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	insecure      bool
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

//Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoXml(req *http.Request, xmlobj interface{}) error {
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

	d := xml.NewDecoder(resp.Body)
	defer logger.ClearVar(&d)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	errd := d.Decode(&xmlobj)
	if errd != nil {
		logger.Log.Error("Err Decode ", req.RequestURI, " error ", errd)
		return errd
	}
	return nil
}

//NewClient return http client with a ratelimiter
func NewClient(skiptlsverify bool, rl *rate.Limiter, rl2 *slidingwindow.Limiter) *RLHTTPClient {
	c := &RLHTTPClient{
		client: &http.Client{Timeout: 10 * time.Second,
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: skiptlsverify},
				MaxIdleConns:          20,
				MaxConnsPerHost:       10,
				DisableCompression:    false,
				DisableKeepAlives:     false,
				IdleConnTimeout:       120 * time.Second}},
		Ratelimiter:   rl,
		LimiterWindow: rl2,
	}
	return c
}
