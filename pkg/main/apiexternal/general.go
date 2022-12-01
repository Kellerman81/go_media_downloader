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

const errorCalling string = "Error calling"
const pleaseWait string = "please wait"

var errPleaseWait = errors.New(pleaseWait)

func logerror(url string, err error) {
	logger.Log.GlobalLogger.Error(errorCalling, zap.String("url", url), zap.Error(err))
}
func (c *RLHTTPClient) CheckLimiter(retrycount int, retryafterseconds int64, url string) error {
	var ok bool
	var waitfor time.Duration
	waituntil := (time.Duration(retryafterseconds) * time.Second)
	waituntilmax := (time.Duration(retryafterseconds*int64(retrycount)) * time.Second)
	rand.Seed(time.Now().UnixNano())
	waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
	for i := 0; i < retrycount; i++ {
		ok, waitfor = c.Ratelimiter.Check()
		if ok {
			c.Ratelimiter.AllowForce()
			return nil
		}
		if waitfor > waituntilmax {
			//logger.Log.GlobalLogger.Warn("Hit rate limit - Should wait for (dont retry)", zap.Duration("waitfor", waitfor), zap.String("Url", url))
			return errPleaseWait
		}
		if waitfor == 0 {
			waitfor = waituntil
		} else {
			waitfor = waitfor + waitincrease
		}
		time.Sleep(waitfor)
	}

	if waitfor < (5 * time.Minute) {
		//logger.Log.GlobalLogger.Warn("Hit rate limit - retrys failed for (add 5 minutes to wait) ", zap.String("Url", url))
		c.Ratelimiter.WaitTill(time.Now().Add(5 * time.Minute))
	}
	return errPleaseWait
}

func (c *RLHTTPClient) PreCheck(retrycount int, retryafterseconds int64, url string) error {
	var ok bool
	var waitfor time.Duration
	waituntil := (time.Duration(retryafterseconds) * time.Second)
	waituntilmax := (time.Duration(retryafterseconds*int64(retrycount)) * time.Second)
	rand.Seed(time.Now().UnixNano())
	waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
	for i := 0; i < retrycount; i++ {
		ok, waitfor = c.Ratelimiter.Check()
		if ok {
			return nil
		}
		if waitfor > waituntilmax {
			//logger.Log.GlobalLogger.Warn("Hit rate limit - Should wait for (dont retry)", zap.Duration("waitfor", waitfor), zap.String("Url", url))
			return errPleaseWait
		}
		if waitfor == 0 {
			waitfor = waituntil
		} else {
			waitfor = waitfor + waitincrease
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
	var resp *http.Response
	if len(headers) >= 1 {
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return 0, errors.New("error get url: " + url + " error: " + err.Error())
		}
		for idx := range headers {
			req.Header.Add((headers)[idx].Key, (headers)[idx].Val)
		}
		headers = nil
		resp, err = c.client.Do(req)
	} else {
		resp, err = c.client.Get(url)
	}
	headers = nil
	if err != nil {
		if resp == nil {
			return 404, err
		}
		c.addwait(url, resp)
		defer resp.Body.Close()
		return resp.StatusCode, err
	}
	defer resp.Body.Close()
	if c.addwait(url, resp) {
		return 429, errPleaseWait
	}
	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(jsonobj)
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

	var resp *http.Response
	if len(headers) >= 1 {
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return 0, errors.New("error get url: " + url + " error: " + err.Error())
		}
		for idx := range headers {
			req.Header.Add((headers)[idx].Key, (headers)[idx].Val)
		}
		headers = nil
		resp, err = c.client.Do(req)
	} else {
		resp, err = c.client.Get(url)
	}
	if err != nil {
		if resp == nil {
			return 404, err
		}
		c.addwait(url, resp)
		defer resp.Body.Close()
		return resp.StatusCode, err
	}
	defer resp.Body.Close()
	if c.addwait(url, resp) {
		return 429, errPleaseWait
	}
	d := xml.NewDecoder(resp.Body)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	err = d.Decode(xmlobj)
	if err != nil {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))

		logger.Log.GlobalLogger.Error("Err Decode ", zap.String("Url", url), zap.Error(err))
		d = nil
		return resp.StatusCode, err
	}
	d = nil
	return resp.StatusCode, nil
}

// func (c *RLHTTPClient) getrespbody(url string, headers []AddHeader) (*http.Response, error) {
// 	if len(headers) >= 1 {
// 		req, err := http.NewRequest("GET", url, nil)
// 		if err != nil {
// 			return nil, errors.New("error get url: " + url + " error: " + err.Error())
// 		}
// 		for idx := range headers {
// 			req.Header.Add((headers)[idx].Key, (headers)[idx].Val)
// 		}
// 		headers = nil
// 		return c.client.Do(req)
// 	} else {
// 		return c.client.Get(url)
// 	}
// }

func (c *RLHTTPClient) testsleep(s string) (bool, string) {
	limitincreased := false
	errstr := ""
	if sleep, err := strconv.ParseInt(s, 10, 64); err == nil {
		c.Ratelimiter.WaitTill(time.Now().Add(time.Second * time.Duration(sleep)))
		limitincreased = true
	} else {
		errstr = err.Error()
		if sleeptime, err := time.Parse(time.RFC1123, s); err == nil {
			c.Ratelimiter.WaitTill(sleeptime)
			limitincreased = true
		} else {
			errstr = err.Error()
		}
	}
	return limitincreased, errstr
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
			limitincreased, errstr = c.testsleep(s[0])
		} else if s, ok := resp.Header["X-Retry-After"]; ok {
			limitincreased, errstr = c.testsleep(s[0])
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
