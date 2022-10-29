package apiexternal

import (
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/rate"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
)

// RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	client              *http.Client
	Ratelimiter         *rate.RateLimiter
	DailyRatelimiter    *rate.RateLimiter
	DailyLimiterEnabled bool
}

var errPleaseWait = errors.New("please wait")

func (c *RLHTTPClient) CheckLimiter(retrycount int, retryafterseconds int64, url string) error {
	var ok bool
	var waitfor time.Duration
	for i := 0; i < retrycount; i++ {
		ok, waitfor = c.Ratelimiter.Allow()
		if ok {
			return nil
		}
		if waitfor > (time.Duration(retryafterseconds*int64(retrycount)) * time.Second) {
			//logger.Log.GlobalLogger.Warn("Hit rate limit - Should wait for (dont retry)", zap.Duration("waitfor", waitfor), zap.String("Url", url))
			return errPleaseWait
		}
		if waitfor == 0 {
			waitfor = time.Duration(retryafterseconds) * time.Second
		} else {
			rand.Seed(time.Now().UnixNano())
			waitfor = waitfor + (time.Duration(rand.Intn(500)+10) * time.Millisecond)
		}
		time.Sleep(waitfor)
	}

	if waitfor < (5 * time.Minute) {
		//logger.Log.GlobalLogger.Warn("Hit rate limit - retrys failed for (add 5 minutes to wait) ", zap.String("Url", url))
		c.Ratelimiter.WaitTill(time.Now().Add(5 * time.Minute))
	}
	return errPleaseWait
}

var errDailyLimit = errors.New("daily limit reached")

// Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoJson(url string, jsonobj interface{}, headers []AddHeader) (int, error) {
	// Comment out the below 5 lines to turn off ratelimiting
	err := c.CheckLimiter(10, 1, url)
	if err != nil {
		if err == errDailyLimit {
			return 0, nil
		}
		return 0, err
	}
	resp, err := c.getrespbody(url, headers)
	if err != nil {
		if resp == nil {
			return 404, err
		}
		c.addwait(url, resp)
		defer func() {
			resp.Body.Close()
			resp = nil
		}()
		return resp.StatusCode, err
	}
	defer func() {
		resp.Body.Close()
		resp = nil
	}()
	if c.addwait(url, resp) {
		return 429, errPleaseWait
	}

	dc := json.NewDecoder(resp.Body)
	err = dc.Decode(jsonobj)
	dc = nil
	return resp.StatusCode, err
}

type AddHeader struct {
	Key string
	Val string
}

// Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoXml(url string, xmlobj interface{}, headers []AddHeader) (int, error) {
	err := c.CheckLimiter(10, 1, url)
	if err != nil {
		if err == errDailyLimit {
			return 0, nil
		}
		return 0, err
	}

	resp, err := c.getrespbody(url, headers)
	if err != nil {
		if resp == nil {
			return 404, err
		}
		c.addwait(url, resp)
		defer func() {
			resp.Body.Close()
			resp = nil
		}()
		return resp.StatusCode, err
	}
	defer func() {
		resp.Body.Close()
		resp = nil
	}()
	if c.addwait(url, resp) {
		return 429, errPleaseWait
	}

	dc := getxmldecoder(resp.Body)
	err = dc.Decode(xmlobj)
	if err != nil {
		dc = nil
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))

		logger.Log.GlobalLogger.Error("Err Decode ", zap.String("Url", url), zap.Error(err))
		return resp.StatusCode, err
	}
	dc = nil
	return resp.StatusCode, nil
}

func getxmldecoder(respbody io.ReadCloser) *xml.Decoder {
	d := xml.NewDecoder(respbody)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	return d
}

func (c *RLHTTPClient) getrespbody(url string, headers []AddHeader) (*http.Response, error) {
	if len(headers) >= 1 {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, errors.New("error get url: " + url + " error: " + err.Error())
		}
		for idx := range headers {
			req.Header.Add(headers[idx].Key, headers[idx].Val)
		}

		resp, err := c.client.Do(req)
		req = nil
		return resp, err
	} else {
		return c.client.Get(url)
	}
}

func (c *RLHTTPClient) addwait(url string, resp *http.Response) bool {
	blockinterval := 5
	if config.Cfg.General.FailedIndexerBlockTime != 0 {
		blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 408 {
		c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
		logger.Log.GlobalLogger.Error("error get response url: " + url + " status: " + resp.Status)

		resp.Body.Close()
		return true
	} else if resp.StatusCode == 429 || resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 408 || resp.StatusCode == 500 || resp.StatusCode == 503 || resp.StatusCode == 204 || resp.StatusCode == 522 {
		hdr := ""
		errstr := ""
		for key := range resp.Header {
			hdr += "Header Key: " + key + " values: " + strings.Join(resp.Header[key], ",")
		}
		limitincreased := false
		if s, ok := resp.Header["Retry-After"]; ok {
			if sleep, err := strconv.ParseInt(s[0], 10, 64); err == nil {
				c.Ratelimiter.WaitTill(time.Now().Add(time.Second * time.Duration(sleep)))
				limitincreased = true
			} else {
				errstr = err.Error()
				if sleeptime, err := time.Parse(time.RFC1123, s[0]); err == nil {
					c.Ratelimiter.WaitTill(sleeptime)
					limitincreased = true
				} else {
					errstr = err.Error()
				}
			}
		} else if s, ok := resp.Header["X-Retry-After"]; ok {
			if sleep, err := strconv.ParseInt(s[0], 10, 64); err == nil {
				c.Ratelimiter.WaitTill(time.Now().Add(time.Second * time.Duration(sleep)))
				limitincreased = true
			} else {
				errstr = err.Error()
				if sleeptime, err := time.Parse(time.RFC1123, s[0]); err == nil {
					c.Ratelimiter.WaitTill(sleeptime)
					limitincreased = true
				} else {
					errstr = err.Error()
				}
			}
		} else {
			if resp.StatusCode == 400 && resp.Body != nil {
				b, _ := io.ReadAll(resp.Body)
				if strings.Contains(string(b), "Request limit reached") {
					c.Ratelimiter.WaitTill(time.Now().Add(3 * time.Hour))
					limitincreased = true
				}
			}
		}
		if !limitincreased {
			c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
		}
		logger.Log.GlobalLogger.Error("error get response url: " + url + " status: " + resp.Status + " headers: " + hdr + " error: " + errstr)
		resp.Body.Close()
		return true
	}
	return false
}

// NewClient return http client with a ratelimiter
func NewClient(skiptlsverify bool, rl *rate.RateLimiter, timeoutseconds int) *RLHTTPClient {
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}
	c := &RLHTTPClient{
		client: &http.Client{Timeout: time.Duration(timeoutseconds) * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout:   time.Duration(timeoutseconds) * time.Second,
				ResponseHeaderTimeout: time.Duration(timeoutseconds) * time.Second,
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: skiptlsverify},
				MaxIdleConns:          20,
				MaxConnsPerHost:       10,
				DisableCompression:    false,
				DisableKeepAlives:     true,
				IdleConnTimeout:       120 * time.Second}},
		Ratelimiter: rl,
	}
	return c
}
