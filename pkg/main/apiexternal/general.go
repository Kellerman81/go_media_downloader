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
	Ratelimiter         *rate.Limiter
	DailyRatelimiter    *rate.Limiter
	DailyLimiterEnabled bool
}

type addHeader struct {
	key string
	val string
}

const errorCalling = "Error calling"

var errPleaseWait = errors.New("please wait")
var errDailyLimit = errors.New("daily limit reached")

func (c *RLHTTPClient) checkLimiter(retrycount int, retryafterseconds int64, url string) (bool, error) {
	waituntil := (time.Duration(retryafterseconds) * time.Second)
	waituntilmax := (time.Duration(retryafterseconds*int64(retrycount)) * time.Second)
	rand.Seed(time.Now().UnixNano())
	waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
	_, dailyok, waitfor := c.Ratelimiter.Check(false, true)
	if !dailyok {
		return false, errDailyLimit
	}
	var ok bool
	for i := 0; i < retrycount; i++ {
		ok, _, waitfor = c.Ratelimiter.Check(true, false)
		if ok {
			c.Ratelimiter.AllowForce()
			return true, nil
		}
		if waitfor > waituntilmax {
			time.Sleep(waituntilmax)
			//logger.Log.GlobalLogger.Warn("Hit rate limit - Should wait for (dont retry)", zap.Duration("waitfor", waitfor), zap.String("Url", url))
			return false, errPleaseWait
		}
		if waitfor == 0 {
			waitfor = waituntil
		} else {
			waitfor += waitincrease
		}
		time.Sleep(waitfor)
	}

	if waitfor < (5 * time.Minute) {
		//logger.Log.GlobalLogger.Warn("Hit rate limit - retrys failed for (add 5 minutes to wait) ", zap.String("Url", url))
		c.Ratelimiter.WaitTill(time.Now().Add(5 * time.Minute))
	}
	return false, errPleaseWait
}

func (c *RLHTTPClient) getResponse(url string, headers []addHeader) (*http.Response, error) {
	if len(headers) >= 1 {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			// headers = nil
			return nil, err
		}
		for idx := range headers {
			req.Header.Add((headers)[idx].key, (headers)[idx].val)
		}
		// headers = nil
		return c.client.Do(req)
	} else {
		// headers = nil
		return c.client.Get(url)
	}
}

// Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoJSON(url string, jsonobj interface{}, headers []addHeader) (int, error) {
	// Comment out the below 5 lines to turn off ratelimiting
	ok, err := c.checkLimiter(10, 1, url)
	if !ok {
		if err == nil {
			err = errPleaseWait
		}
		if err == errDailyLimit {
			// headers = nil
			return 0, nil
		}
		// headers = nil
		return 0, err
	}
	resp, err := c.getResponse(url, headers)
	if err != nil {
		if resp == nil {
			return 404, err
		}
		c.addwait(url, resp)
		return resp.StatusCode, err
	}
	defer resp.Body.Close()
	if c.addwait(url, resp) {
		return 429, errPleaseWait
	}
	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(jsonobj)
}

// Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoXML(url string, headers []addHeader, feed *searchResponse) error {
	ok, err := c.checkLimiter(10, 1, url)
	if !ok {
		if err == nil {
			err = errPleaseWait
		}
		return err
	}

	resp, err := c.getResponse(url, headers)
	if err != nil {
		if resp == nil {
			return err
		}
		c.addwait(url, resp)
		return err
	}
	defer resp.Body.Close()
	if c.addwait(url, resp) {
		return errPleaseWait
	}
	d := xml.NewDecoder(resp.Body)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	err = d.Decode(feed)
	d = nil
	if err != nil {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))

		logger.Log.GlobalLogger.Error("Err Decode ", zap.Stringp("Url", &url), zap.Error(err))
		feed = nil
		return err
	}
	return nil
}

func (c *RLHTTPClient) testsleep(s string) (bool, string) {
	var errstr string
	if sleep, err := strconv.ParseInt(s, 10, 64); err == nil {
		c.Ratelimiter.WaitTill(time.Now().Add(time.Second * time.Duration(sleep)))
		return true, errstr
	} else {
		errstr = err.Error()
		if sleeptime, err := time.Parse(time.RFC1123, s); err == nil {
			c.Ratelimiter.WaitTill(sleeptime)
			return true, errstr
		} else {
			return false, err.Error()
		}
	}
}
func (c *RLHTTPClient) addwait(url string, resp *http.Response) bool {
	blockinterval := 5
	if config.Cfg.General.FailedIndexerBlockTime != 0 {
		blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusInternalServerError || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == 521 || resp.StatusCode == 522 || resp.StatusCode == 524 || resp.StatusCode == 204 {
		//408 Timeout
		//404 not found
		//500 Internal Server Error
		//503 Service Unavailable
		//522 Connection Timed Out
		//521 Web Server Is Down
		//524 A Timeout Occurred
		//204 No Content

		//Trakt responds with 404 if media not found
		if resp.StatusCode != http.StatusNotFound {
			c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
		}
		logger.Log.GlobalLogger.Error("error get response url: " + url + " status: " + resp.Status)

		return true
	} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusBadRequest {
		//403 Forbidden
		//401 unauthorized
		//429 too many requests
		//400 bad request
		var hdr, errstr string
		for key := range resp.Header {
			hdr += "Header Key: " + key + " values: " + strings.Join(resp.Header[key], ",")
		}
		var limitincreased bool
		if s, ok := resp.Header["Retry-After"]; ok {
			limitincreased, errstr = c.testsleep(s[0])
		} else if s, ok := resp.Header["X-Retry-After"]; ok {
			limitincreased, errstr = c.testsleep(s[0])
		} else if resp.StatusCode == 400 && resp.Body != nil {
			b, _ := io.ReadAll(resp.Body)
			if strings.Contains(string(b), "Request limit reached") {
				c.Ratelimiter.WaitTill(time.Now().Add(3 * time.Hour))
				limitincreased = true
			}
		}
		if !limitincreased {
			c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
		}
		logger.Log.GlobalLogger.Error("error get response url: " + url + " status: " + resp.Status + " headers: " + hdr + " error: " + errstr)
		return true
	}
	return false
}

// NewClient return http client with a ratelimiter
func NewClient(skiptlsverify bool, disablecompression bool, rl *rate.Limiter, timeoutseconds int) *RLHTTPClient {
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}
	return &RLHTTPClient{
		client: &http.Client{Timeout: time.Duration(timeoutseconds) * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout:   time.Duration(timeoutseconds) * time.Second,
				ResponseHeaderTimeout: time.Duration(timeoutseconds) * time.Second,
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: skiptlsverify},
				MaxIdleConns:          20,
				MaxConnsPerHost:       10,
				DisableCompression:    disablecompression,
				DisableKeepAlives:     true,
				IdleConnTimeout:       120 * time.Second}},
		Ratelimiter: rl,
	}
}
