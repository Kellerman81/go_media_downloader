package apiexternal

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/net/html/charset"
)

// rlHTTPClient is a rate limited HTTP client struct.
// It contains fields for the underlying http.Client, name, timeouts,
// rate limiters, and enabling daily rate limiting.
type rlHTTPClient struct {
	client              *http.Client           // The underlying HTTP client
	clientname          string                 // The name of the client
	Timeout             time.Duration          // The timeout duration
	DailyLimiterEnabled bool                   // Whether daily rate limiting is enabled
	Ratelimiter         *slidingwindow.Limiter // The per-request rate limiter
	DailyRatelimiter    *slidingwindow.Limiter // The daily rate limiter
}

const (
	apiurltmdbmovies = "https://api.themoviedb.org/3/movie/"
	apiurlshows      = "https://api.trakt.tv/shows/"
	apiurlmovies     = "https://api.trakt.tv/movies/"
	extendedfull     = "?extended=full"
	strguid          = "guid"
	strsize          = "size"
	strlink          = "link"
	strtitle         = "title"
	strurl           = "url"
)

// traktAPI is a client for interacting with the Trakt API
var traktAPI *traktClient

// tvdbAPI is a client for interacting with the TVDB API
var tvdbAPI *tvdbClient

// tmdbAPI is a client for interacting with the TMDB API
var tmdbAPI *tmdbClient

// pushoverAPI is a client for sending Pushover notifications
var pushoverAPI *pushOverClient

// omdbAPI is a client for interacting with the OMDb API
var omdbAPI *omdbClient

// newznabClients is a slice of newznab client structs
var newznabClients = make([]clients, 0, 10)

// cl is a default HTTP client with rate limiting and timeouts
var cl = NewClient("defaultdownloader", true, true, slidingwindow.NewLimiter(1*time.Second, 10), false, slidingwindow.NewLimiter(10*time.Second, 10), 30)

// dialer is a net.Dialer with timeout and keepalive options set
var dialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
	DualStack: true,
}

// fieldmap maps XML element numbers to field names
var fieldmap = map[int]string{
	1: strtitle,
	2: strlink,
	3: strguid,
	4: strsize,
}

// WebGetByte performs a GET request to the provided URL and returns
// the response body as a byte slice using a customized HTTP client
// with timeouts and other options set.
func WebGet(url string) (*http.Response, error) {
	return cl.GetdoResp(url)
}

// checkLimiter checks if the rate limiter allows a request. It handles retrying with increasing backoff if rate limited.
// allow forces an allowance if true. retrycount is the max number of retries.
// retryafterseconds is the initial backoff duration.
// Returns true if allowed, false if rate limited after retries.
func (c *rlHTTPClient) checkLimiter(allow bool, retrycount int, retryafterseconds int64) (bool, error) {
	waituntil := (time.Duration(retryafterseconds) * time.Second)
	waituntilmax := (time.Duration(retryafterseconds*int64(retrycount)) * time.Second)
	rand.New(rand.NewSource(time.Now().UnixNano()))
	waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
	if c.DailyLimiterEnabled {
		if ok, _ := c.DailyRatelimiter.Check(); !ok {
			return false, logger.ErrDailyLimit
		}
	}
	// ok, waitfor := c.Ratelimiter.CheckDaily()
	// if !ok {
	// }
	var ok bool
	var waitfor time.Duration
	for i := 0; i < retrycount; i++ {
		ok, waitfor = c.Ratelimiter.Check()
		if ok {
			if allow {
				c.Ratelimiter.AllowForce()
				if c.DailyLimiterEnabled {
					c.DailyRatelimiter.AllowForce()
				}
			}
			return true, nil
		}
		if waitfor > waituntilmax {
			//time.Sleep(waituntilmax)
			logger.LogDynamic("debug", "Hit rate limit - Should wait for (dont retry)", logger.NewLogField("waitfor", waitfor), logger.NewLogField("url", &c.clientname))
			return false, logger.ErrToWait
		}
		if waitfor == 0 {
			waitfor = waituntil
		} else {
			waitfor += waitincrease
		}
		time.Sleep(waitfor)
	}

	//logger.LogAnyError(nil, "Hit rate limit - retries failed")
	logger.LogDynamic("warn", "Hit rate limit - retrys failed", logger.NewLogField("url", c.clientname))

	return false, logger.ErrToWait
}

// DoJSONType makes a request to the url and unmarshals the JSON response into the given type S.
// It handles rate limiting and retries.
func DoJSONType[S any](c *rlHTTPClient, urlv string, headers ...keyval) (S, error) {
	var u S
	resp, err := c.GetdoResp(urlv, headers...)
	if err != nil {
		return u, err
	}
	//defer clear(resp)
	defer resp.Body.Close()
	return u, json.NewDecoder(resp.Body).Decode(&u)
	//return u, json.Unmarshal(resp, &u)
}

// checklimiterwithdaily checks if the rate limiter and daily rate limiter
// allow a request. It attempts to check the rate limiter up to the
// specified number of retries and increasing backoff. If the daily rate
// limiter is hit, it returns true to indicate the daily limit was reached.
func (c *rlHTTPClient) checklimiterwithdaily() bool {
	ok, err := c.checkLimiter(true, 20, 1)
	if !ok {
		if err == nil || errors.Is(err, logger.ErrDailyLimit) {
			return true
		}
		return true
	}
	return false
}

// DoJSONTypeG makes a request to the url and unmarshals the JSON response into
// the given generic slice type T. It handles rate limiting and retries.
func DoJSONTypeG[T any](c *rlHTTPClient, urlv string, headers ...keyval) ([]T, error) {
	resp, err := c.GetdoResp(urlv, headers...)

	if err != nil {
		return nil, err
	}
	//defer clear(resp)
	defer resp.Body.Close()
	var u []T
	//return u, json.Unmarshal(resp.Body, &u)
	return u, json.NewDecoder(resp.Body).Decode(&u)
}

// Getdo makes an HTTP GET request to the given URL with the optional headers.
// It returns the http.Response and any error.
// It handles rate limiting by calling addwait.
func (c *rlHTTPClient) Getdo(urlv string, headers []keyval, checkhtml bool) (*http.Response, error) {
	//ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	//defer cancel()
	req, err := http.NewRequest(http.MethodGet, urlv, nil)
	if err != nil {
		logger.LogDynamic("error", "failed to get url", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrURL, urlv))
		return nil, err
	}
	for _, h := range headers {
		req.Header.Add(h.Key, h.Value)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && c.addwait(req, resp) {
		defer resp.Body.Close()
		return nil, logger.ErrToWait
	}
	if checkhtml {
		if true && resp.Header.Get("Content-Type") == "text/html" {
			io.Copy(io.Discard, resp.Body)
			defer resp.Body.Close()
			return nil, logger.ErrNotAllowed
		}
	}
	return resp, nil
}

// GetdoByte makes an HTTP GET request to the given URL and returns
// the response body as a byte slice. It handles rate limiting by
// calling addwait.
func (c *rlHTTPClient) GetdoByte(urlv string, headers ...keyval) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()
	//ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlv, nil)
	if err != nil {
		logger.LogDynamic("error", "failed to get url", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrURL, urlv))
		return nil, err
	}
	for idx := range headers {
		req.Header.Add(headers[idx].Key, headers[idx].Value)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && c.addwait(req, resp) {
		return nil, logger.ErrToWait
	}
	return io.ReadAll(resp.Body)
}

func (c *rlHTTPClient) GetdoResp(urlv string, headers ...keyval) (*http.Response, error) {
	//ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	//defer cancel()
	req, err := http.NewRequest(http.MethodGet, urlv, nil)
	if err != nil {
		logger.LogDynamic("error", "failed to get url", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrURL, urlv))
		return nil, err
	}
	for idx := range headers {
		req.Header.Add(headers[idx].Key, headers[idx].Value)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && c.addwait(req, resp) {
		defer resp.Body.Close()
		return nil, logger.ErrToWait
	}
	return resp, nil
}

// XMLResponse represents the response from an XML API request.
// It contains fields for:
// - Err: Any error from the request
// - FirstID: The ID of the first result
// - BrokeLoop: Whether parsing broke out of the result loop early
type XMLResponse struct {
	Err       error
	FirstID   string
	BrokeLoop bool
}

// DoXMLItem parses an XML response from an indexer API endpoint to extract
// NZB items. It handles rate limiting and decoding the XML into Nzbwithprio
// structs, appending results to the provided slice. It can stop early if
// a matching ID is found in tillid. Returns false, error on failure.
func (c *rlHTTPClient) DoXMLItem(ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, apiBaseURL string, urlv string, mu *sync.Mutex, createsize int, results *[]Nzbwithprio) XMLResponse {
	var retval XMLResponse
	if urlv == "" {
		retval.Err = logger.ErrNotFound
		return retval
	}
	ok, err := c.checkLimiter(true, 20, 1)
	if !ok {
		if err == nil {
			retval.Err = logger.ErrToWait
			return retval
		}
		retval.Err = err
		return retval
	}
	resp, err := c.GetdoResp(urlv)
	if err != nil {
		retval.Err = err
		return retval
	}
	//defer clear(resp)
	defer resp.Body.Close()
	//d := xml.NewDecoder(resp)
	d := xml.NewDecoder(resp.Body)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false

	mu.Lock()
	if results == nil || cap(*results) == 0 {
		*results = make([]Nzbwithprio, 0, createsize*qual.IndexerLen)
	}
	mu.Unlock()

	//b := plnzbprio.Get().(*Nzbwithprio)
	//defer b.ClosePut()
	var nameidx, valueidx int
	var b Nzbwithprio
	var lastfield int
	for {
		t, err := d.RawToken()
		if err != nil {
			break
		}

		switch tt := t.(type) {
		case xml.StartElement:
			lastfield = -1
			switch tt.Name.Local {
			case "item":
				b.NZB = nzb{}
				b.NZB.Indexer = ind
				b.NZB.Quality = qual
				b.NZB.SourceEndpoint = apiBaseURL
			case strtitle:
				lastfield = 1
			case strlink:
				lastfield = 2
			case strguid:
				lastfield = 3
			case strsize:
				lastfield = 4
			case "enclosure", "source":
				for idx := range tt.Attr {
					switch tt.Attr[idx].Name.Local {
					case strurl:
						if b.NZB.DownloadURL == "" {
							b.NZB.DownloadURL = tt.Attr[idx].Value
						}
					case "length":
						if b.NZB.Size == 0 {
							b.NZB.Size = logger.StringToInt64(tt.Attr[idx].Value)
						}
					}
				}
			case "attr":
				nameidx = -1
				valueidx = -1
				for idx := range tt.Attr {
					switch tt.Attr[idx].Name.Local {
					case "name":
						nameidx = idx
					case "value":
						valueidx = idx
					}
				}
				if nameidx == -1 || valueidx == -1 || tt.Attr[valueidx].Value == "" {
					break
				}
				setfield(tt.Attr[nameidx].Value, tt.Attr[valueidx].Value, &b.NZB)
			}
		case xml.CharData:
			if lastfield > 0 && lastfield < 5 && len(tt) > 0 {
				setfield(fieldmap[lastfield], string(tt), &b.NZB)
			}
		case xml.EndElement:
			if tt.Name.Local == "item" {
				if b.NZB.ID == "" {
					b.NZB.ID = b.NZB.DownloadURL
				}
				if retval.FirstID == "" {
					retval.FirstID = b.NZB.ID
				}
				//mu.Lock()
				*results = append(*results, b)
				//mu.Unlock()
				if tillid != "" && tillid == b.NZB.ID {
					retval.BrokeLoop = true
					return retval
				}
			}
		}
	}
	return retval
}

// addwait handles rate limiting and blocking based on the HTTP response.
// It will wait and return true if the response indicates the request should be rate limited or blocked.
func (c *rlHTTPClient) addwait(req *http.Request, resp *http.Response) bool {
	blockinterval := 5
	if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
		blockinterval = config.SettingsGeneral.FailedIndexerBlockTime
	}
	if resp == nil {
		c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
		return true
	}

	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusServiceUnavailable, 521, 522, 524, http.StatusNoContent:
		if resp.StatusCode != http.StatusNotFound {
			c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
		}
		logger.LogDynamic("error", "error get response url", logger.NewLogField(strurl, req.URL.String()), logger.NewLogField("status", resp.Status))
		return true

	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadRequest:
		s, ok := resp.Header["Retry-After"]
		if !ok {
			s, ok = resp.Header["X-Retry-After"]
		}
		if ok {
			if sleep, err := strconv.Atoi(s[0]); err == nil {
				c.Ratelimiter.WaitTill(logger.TimeGetNow().Add(time.Second*time.Duration(sleep) - c.Ratelimiter.Interval()))
			} else if sleeptime, err := time.Parse(time.RFC1123, s[0]); err == nil {
				c.Ratelimiter.WaitTill(sleeptime.Add(-c.Ratelimiter.Interval()))
				io.Copy(io.Discard, resp.Body)
				return true
			} else {
				c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
			}
		} else if resp.StatusCode == 400 && resp.Body != nil {
			if b, _ := io.ReadAll(resp.Body); logger.ContainsI(string(b), "Request limit reached") {
				c.Ratelimiter.WaitTill(time.Now().Add(3 * time.Hour))
				return true
			}
		}

		logger.LogDynamic("error", "error get response url", logger.NewLogField(strurl, req.URL.String()), logger.NewLogField("status", resp.Status))
		io.Copy(io.Discard, resp.Body)
		return true
	}

	return false
}

// NewClient creates a new HTTP client for making external API requests. It configures rate limiting, TLS verification, compression, timeouts etc. based on the provided parameters.
func NewClient(clientname string, skiptlsverify bool, disablecompression bool, rl *slidingwindow.Limiter, usedaily bool, rldaily *slidingwindow.Limiter, timeoutseconds int) *rlHTTPClient {
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false, //please don't
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
	}

	transport.ResponseHeaderTimeout = time.Duration(timeoutseconds) * time.Second

	if skiptlsverify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if disablecompression {
		transport.DisableCompression = true
	}
	return &rlHTTPClient{
		Timeout:             time.Duration(timeoutseconds) * time.Second,
		clientname:          clientname,
		client:              &http.Client{Transport: transport},
		Ratelimiter:         rl,
		DailyLimiterEnabled: usedaily,
		DailyRatelimiter:    rldaily,
	}
}
