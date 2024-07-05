package apiexternal

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	//"encoding/json"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"github.com/goccy/go-json"
	"golang.org/x/oauth2"
)

// rlHTTPClient is a rate limited HTTP client struct.
// It contains fields for the underlying http.Client, name, timeouts,
// rate limiters, and enabling daily rate limiting.
type rlHTTPClient struct {
	client              *http.Client           // The underlying HTTP client
	Clientname          string                 // The name of the client
	Timeout             time.Duration          // The timeout duration
	DailyLimiterEnabled bool                   // Whether daily rate limiting is enabled
	Ratelimiter         *slidingwindow.Limiter // The per-request rate limiter
	DailyRatelimiter    *slidingwindow.Limiter // The daily rate limiter
	Ctx                 context.Context
}

type NzbSlice struct {
	Arr []Nzbwithprio
	Mu  sync.Mutex
}

type apidata struct {
	apikey         string
	apikeyq        string
	seconds        uint8
	calls          int
	disabletls     bool
	timeoutseconds uint16
	clientID       string // The client ID for OAuth2
	clientSecret   string
	limiter        slidingwindow.Limiter
	dailylimiter   slidingwindow.Limiter
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
)

var (
	errDailyLimit = errors.New("daily limit reached")
	// traktAPI is a client for interacting with the Trakt API
	//traktAPI     traktClient
	traktApidata apidata
	pltrakt      = pool.NewPool[traktClient](100, 0, func(p *traktClient) {
		*p = traktClient{
			APIKey:       traktApidata.apikey,
			ClientID:     traktApidata.clientID,
			ClientSecret: traktApidata.clientSecret,
			Client: NewClient(
				"trakt",
				traktApidata.disabletls,
				true,
				&traktApidata.limiter,
				false, &traktApidata.dailylimiter, traktApidata.timeoutseconds),
			Auth: oauth2.Config{
				ClientID:     traktApidata.clientID,
				ClientSecret: traktApidata.clientSecret,
				RedirectURL:  "http://localhost:9090",
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://api.trakt.tv/oauth/authorize",
					TokenURL: "https://api.trakt.tv/oauth/token",
				},
			},
			Token:          config.GetTrakt(),
			DefaultHeaders: []string{"Content-Type", "application/json", "trakt-api-version", "2", "trakt-api-key", traktApidata.clientID}}
		if p.Token.AccessToken != "" {
			p.DefaultHeaders = append(p.DefaultHeaders, "Authorization", "Bearer "+p.Token.AccessToken)
		}
	}, func(p *traktClient) {
		p.Client = NewClient(
			"trakt",
			traktApidata.disabletls,
			true,
			&traktApidata.limiter,
			false, &traktApidata.dailylimiter, traktApidata.timeoutseconds)
	})

	// tvdbAPI is a client for interacting with the TVDB API
	//tvdbAPI tvdbClient
	tvdbApidata apidata
	pltvdb      = pool.NewPool[tvdbClient](100, 0, func(p *tvdbClient) {
		*p = tvdbClient{
			Client: NewClient(
				"tvdb",
				tvdbApidata.disabletls,
				true,
				&tvdbApidata.limiter,
				false, &tvdbApidata.dailylimiter, tvdbApidata.timeoutseconds)}
	}, func(p *tvdbClient) {
		p.Client = NewClient(
			"tvdb",
			tvdbApidata.disabletls,
			true,
			&tvdbApidata.limiter,
			false, &tvdbApidata.dailylimiter, tvdbApidata.timeoutseconds)
	})

	// tmdbAPI is a client for interacting with the TMDB API
	//tmdbAPI tmdbClient
	tmdbApidata apidata

	pltmdb = pool.NewPool[tmdbClient](100, 0, func(p *tmdbClient) {
		*p = tmdbClient{
			APIKey:         tmdbApidata.apikey,
			QAPIKey:        tmdbApidata.apikeyq,
			DefaultHeaders: []string{"accept", "application/json", "Authorization", "Bearer " + tmdbApidata.apikey},
			Client: NewClient(
				"tmdb",
				tmdbApidata.disabletls,
				true,
				&tmdbApidata.limiter,
				false, &tmdbApidata.dailylimiter, tmdbApidata.timeoutseconds)}
	}, func(p *tmdbClient) {
		p.Client = NewClient(
			"tmdb",
			tmdbApidata.disabletls,
			true,
			&tmdbApidata.limiter,
			false, &tmdbApidata.dailylimiter, tmdbApidata.timeoutseconds)
	})

	// pushoverAPI is a client for sending Pushover notifications
	//pushoverAPI pushOverClient
	//pushoverApidata apidata

	// omdbAPI is a client for interacting with the OMDb API
	//omdbAPI omdbClient
	omdbApidata apidata

	plomdb = pool.NewPool[omdbClient](100, 0, func(p *omdbClient) {
		*p = omdbClient{
			OmdbAPIKey: omdbApidata.apikey,
			QAPIKey:    omdbApidata.apikeyq,
			Client: NewClient(
				"omdb",
				omdbApidata.disabletls,
				true,
				&omdbApidata.limiter,
				false, &omdbApidata.dailylimiter, omdbApidata.timeoutseconds)}
	}, func(p *omdbClient) {
		*p = omdbClient{}
	})

	// newznabClients is a slice of newznab client structs
	newznabClients = logger.NewSynchedMap[*client](10)

	// cl is a default HTTP client with rate limiting and timeouts
	defaultLimiter = slidingwindow.NewLimiter(1*time.Second, 10)
	cl             = NewClient("defaultdownloader", true, true, &defaultLimiter, false, &defaultLimiter, 30)

	tlsinsecure = tls.Config{InsecureSkipVerify: true}

	// fieldmap maps XML element numbers to field names
	fieldmap = []string{"",
		strtitle,
		strlink,
		strguid,
		strsize,
	}
	bytesRequestLimitReached = []byte("Request limit reached")
	strtimefound             = "time found"
)

// checkLimiter checks if the rate limiter allows a request. It handles retrying with increasing backoff if rate limited.
// allow forces an allowance if true. retrycount is the max number of retries.
// retryafterseconds is the initial backoff duration.
// Returns true if allowed, false if rate limited after retries.
func (c *rlHTTPClient) checkLimiter(allow bool, retrycount int, retryafterseconds int64) (bool, error) {
	if c.DailyLimiterEnabled {
		if ok, _ := c.DailyRatelimiter.Check(); !ok {
			return false, errDailyLimit
		}
	}
	// ok, waitfor := c.Ratelimiter.CheckDaily()
	// if !ok {
	// }
	waituntil := (time.Duration(retryafterseconds) * time.Second)
	waituntilmax := (time.Duration(retryafterseconds*int64(retrycount)) * time.Second)
	rand.New(rand.NewSource(time.Now().UnixNano()))
	waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
	var ok bool
	var waitfor time.Duration
	for range retrycount {
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
	logger.LogDynamicany("warn", "Hit rate limit - retrys failed", &logger.StrURL, &c.Clientname)

	return false, logger.ErrToWait
}

// checklimiterwithdaily checks if the rate limiter and daily rate limiter
// allow a request. It attempts to check the rate limiter up to the
// specified number of retries and increasing backoff. If the daily rate
// limiter is hit, it returns true to indicate the daily limit was reached.
func (c *rlHTTPClient) checklimiterwithdaily() bool {
	ok, _ := c.checkLimiter(true, 20, 1)
	return !ok
}

// Getdo makes an HTTP GET request to the given URL with the optional headers.
// It returns the http.Response and any error.
// It handles rate limiting by calling addwait.
func (c *rlHTTPClient) Getdo(ctx context.Context, urlv string, checkhtml bool, headers []string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlv, nil)
	if err != nil {
		logger.LogDynamicany("error", "failed to get url", err, &logger.StrURL, urlv) //logpointerr
		return nil, err
	}

	if len(headers) >= 1 {
		var key string
		for idx := range headers {
			if key == "" {
				key = headers[idx]
				continue
			}
			req.Header.Add(key, headers[idx])
			key = ""
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		logger.LogDynamicany("error", "failed to process url", err, &logger.StrURL, urlv) //logpointerr
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		if c.addwait(req, resp) {
			return nil, logger.ErrToWait
		}
		return nil, errors.New("http status error " + resp.Status)
	}
	if !checkhtml {
		return resp.Body, nil
	}
	if resp.Header.Get("Content-Type") == "text/html" {
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return nil, logger.ErrNotAllowed
	}
	return resp.Body, nil
}

// DoXMLItem makes an HTTP request to the specified URL, decodes the XML response, and populates the provided results slice with the extracted NZB items. It supports rate limiting, timeouts, and handling various HTTP response codes. The function returns a boolean indicating whether the processing was interrupted (e.g. due to reaching a specific item ID), the ID of the first item processed, and any errors encountered.
func (c *rlHTTPClient) DoXMLItem(ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, apiBaseURL string, urlv string, results *NzbSlice) (bool, string, error) {
	if urlv == "" {
		return false, "", logger.ErrNotFound
	}
	ok, err := c.checkLimiter(true, 20, 1)
	if !ok {
		if err == nil {
			return false, "", logger.ErrToWait
		}
		return false, "", err
	}
	ctx, ctxcancel := context.WithTimeoutCause(c.Ctx, c.Timeout*5, err)
	defer ctxcancel()

	resp, err := c.Getdo(ctx, urlv, false, nil)
	if err != nil {
		return false, "", err
	}
	defer resp.Close()
	if err := logger.CheckContextEnded(ctx); err != nil {
		return false, "", err
	}
	// readersize := 65000
	// if resp.ContentLength < int64(readersize) {
	// 	readersize = int(resp.ContentLength)
	// }
	d := xml.NewDecoder(resp) //testing
	//d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	//d.AutoClose = xmlccloser

	var (
		b         Nzbwithprio
		lastfield int8
		firstid   string
	)
	var nameidx, valueidx int
	for {
		// if err = logger.CheckContextEnded(ctx); err != nil {
		// 	d = nil
		// 	return false, firstid, err
		// }
		t, err := d.RawToken()
		if err != nil {
			break
		}
		switch tt := t.(type) {
		case xml.StartElement:
			lastfield = -1
			switch tt.Name.Local {
			case "item":
				b.NZB.Clear(ind, qual, apiBaseURL) //= Nzb{Indexer: ind, Quality: qual, SourceEndpoint: apiBaseURL}
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
					b.NZB.setfield(tt.Attr[idx].Name.Local, tt.Attr[idx].Value)
				}
				//clear(tt.Attr)
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
				//clear(tt.Attr)
				if nameidx == -1 || valueidx == -1 || tt.Attr[valueidx].Value == "" {
					break
				}
				b.NZB.setfield(tt.Attr[nameidx].Value, tt.Attr[valueidx].Value)
			}
		case xml.CharData:
			if lastfield > 0 && lastfield < 5 && len(tt) > 0 {
				b.NZB.setfield(fieldmap[lastfield], tt)
			}
			//clear(tt)
		case xml.EndElement:
			if tt.Name.Local != "item" {
				break
			}
			if b.NZB.ID == "" {
				b.NZB.ID = b.NZB.DownloadURL
			}
			if firstid == "" {
				firstid = b.NZB.ID
			}
			results.Mu.Lock()
			results.Arr = append(results.Arr, b)
			results.Mu.Unlock()
			if tillid != "" && tillid == b.NZB.ID {
				d = nil
				return true, firstid, nil
			}
		}
		t = nil
	}
	d = nil
	return false, firstid, nil
}

// addwait checks the HTTP response status code and waits an appropriate amount of time before retrying the request.
// If the response indicates a rate limit or error condition, it will wait the specified time before allowing the request to be retried.
// If the response is successful, it will return false to indicate the request can proceed.
// The Response will be invalid after this call
func (c *rlHTTPClient) addwait(req *http.Request, resp *http.Response) bool {
	blockinterval := 5
	if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
		blockinterval = config.SettingsGeneral.FailedIndexerBlockTime
	}
	waitfor := logger.TimeGetNow()
	if resp == nil {
		waitfor = waitfor.Add(time.Minute * time.Duration(blockinterval))
		c.Ratelimiter.WaitTill(waitfor)
		logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)

		return true
	}

	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusServiceUnavailable, 521, 522, 524, http.StatusNoContent:
		if resp.StatusCode != http.StatusNotFound {
			waitfor = waitfor.Add(time.Minute * time.Duration(blockinterval))
			c.Ratelimiter.WaitTill(waitfor)
			logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)
		}
		logger.LogDynamicany("error", "error get response url", &logger.StrURL, req.URL.String(), &logger.StrStatus, &resp.Status)
		io.Copy(io.Discard, resp.Body)
		return true

	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadRequest:
		s, ok := resp.Header["Retry-After"]
		if !ok {
			s, ok = resp.Header["X-Retry-After"]
		}
		if ok {
			if sleep, err := strconv.Atoi(s[0]); err == nil {
				waitfor = waitfor.Add(time.Second*time.Duration(sleep) - c.Ratelimiter.Interval())
				c.Ratelimiter.WaitTill(waitfor)
				logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)
			} else if sleeptime, err := time.Parse(time.RFC1123, s[0]); err == nil {
				waitfor = sleeptime.Add(-c.Ratelimiter.Interval())
				c.Ratelimiter.WaitTill(waitfor)
				logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)
				return true
			} else if sleeptime, err := time.Parse(time.RFC3339, s[0]); err == nil {
				waitfor = sleeptime.Add(-c.Ratelimiter.Interval())
				c.Ratelimiter.WaitTill(waitfor)
				logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)
				return true
			} else if strings.Contains(s[0], "Request limit reached. Retry in ") {
				str := strings.Replace(s[0], "Request limit reached. Retry in ", "", 1)
				str = strings.Trim(str, ".")
				a := strings.Split(str, " ")
				if len(a) == 2 {
					switch a[1] {
					case "minutes":
						i, _ := strconv.Atoi(a[0])
						waitfor = waitfor.Add(time.Minute * time.Duration(i))
						c.Ratelimiter.WaitTill(waitfor)
						logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)
						return true
					case "hours":
						i, _ := strconv.Atoi(a[0])
						waitfor = waitfor.Add(time.Hour * time.Duration(i))
						c.Ratelimiter.WaitTill(waitfor)
						logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)
						return true
					default:
						waitfor = waitfor.Add(time.Minute * time.Duration(blockinterval))
						c.Ratelimiter.WaitTill(waitfor)
						logger.LogDynamicany("debug", "Set Waittill parse failed", &logger.StrWaitfor, &waitfor, &strtimefound, &s[0], &logger.StrURL, &c.Clientname)
						return true
					}
				} else {
					waitfor = waitfor.Add(time.Minute * time.Duration(blockinterval))
					c.Ratelimiter.WaitTill(waitfor)
					logger.LogDynamicany("debug", "Set Waittill parse failed", &logger.StrWaitfor, &waitfor, &strtimefound, &s[0], &logger.StrURL, &c.Clientname)
					return true
				}
			} else {
				waitfor = waitfor.Add(time.Minute * time.Duration(blockinterval))
				c.Ratelimiter.WaitTill(waitfor)
				logger.LogDynamicany("debug", "Set Waittill parse failed", &logger.StrWaitfor, &waitfor, &strtimefound, &s[0], &logger.StrURL, &c.Clientname)
				return true
			}
		} else if resp.StatusCode == 400 && resp.Body != nil {
			b, err := io.ReadAll(resp.Body)
			if err == nil && logger.ContainsByteI(b, bytesRequestLimitReached) {
				waitfor = waitfor.Add(3 * time.Hour)
				c.Ratelimiter.WaitTill(waitfor)
				logger.LogDynamicany("debug", "Set Waittill", &logger.StrWaitfor, &waitfor, &logger.StrURL, &c.Clientname)
				return true
			}
		}
		logger.LogDynamicany("error", "error get response url", &logger.StrURL, req.URL.String(), &logger.StrStatus, &resp.Status)
		io.Copy(io.Discard, resp.Body)
		return true
	}

	io.Copy(io.Discard, resp.Body)
	return false
}

// NewClient creates a new HTTP client for making external API requests. It configures rate limiting, TLS verification, compression, timeouts etc. based on the provided parameters.
func NewClient(clientname string, skiptlsverify bool, disablecompression bool, rl *slidingwindow.Limiter, usedaily bool, rldaily *slidingwindow.Limiter, timeoutseconds uint16) rlHTTPClient {
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     false, //please don't
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
		ResponseHeaderTimeout: time.Duration(timeoutseconds) * time.Second,
	}

	if skiptlsverify {
		transport.TLSClientConfig = &tlsinsecure
	}
	if disablecompression {
		transport.DisableCompression = true
	}
	return rlHTTPClient{
		Timeout:             time.Duration(timeoutseconds) * time.Second,
		Clientname:          clientname,
		client:              &http.Client{Transport: transport},
		Ratelimiter:         rl,
		DailyLimiterEnabled: usedaily,
		DailyRatelimiter:    rldaily,
		Ctx:                 context.Background(),
	}
}

// GetCl returns the singleton instance of the rlHTTPClient.
func GetCl() *rlHTTPClient {
	return &cl
}

// doJSONType makes a request to the url and unmarshals the JSON response into the given type S.
// It handles rate limiting and retries.
func doJSONType[S any](c *rlHTTPClient, urlv string) (S, error) {
	var err error
	ctx, ctxcancel := context.WithTimeoutCause(c.Ctx, c.Timeout*5, err)
	defer ctxcancel()
	resp, err := c.Getdo(ctx, urlv, false, nil)
	if err != nil {
		var u S
		return u, err
	}
	defer resp.Close()
	return jsondecode[S](ctx, resp)
}

// doJSONTypeHeader is a helper function that makes a GET request to the provided URL,
// sets the specified headers, and decodes the JSON response into the provided type.
// The function uses a context with a timeout of 5 times the client's configured timeout.
// If the request fails, the function returns the zero value of the provided type and the error.
func doJSONTypeHeader[S any](c *rlHTTPClient, urlv string, headers []string) (S, error) {
	var err error
	ctx, ctxcancel := context.WithTimeoutCause(c.Ctx, c.Timeout*5, err)
	defer ctxcancel()
	resp, err := c.Getdo(ctx, urlv, false, headers)
	if err != nil {
		var u S
		return u, err
	}
	defer resp.Close()
	return jsondecode[S](ctx, resp)
}

// jsondecode decodes the response body into the provided struct.
// The function checks if the context has been canceled before decoding the response.
func jsondecode[S any](ctx context.Context, resp io.ReadCloser) (S, error) {
	if err := logger.CheckContextEnded(ctx); err != nil {
		var u S
		return u, err
	}
	var v S
	if err := json.NewDecoder(resp).DecodeContext(ctx, &v); err != nil {
		return v, err
	}
	return v, nil
}

// doJSONTypeG makes a request to the url and unmarshals the JSON response into
// the given generic slice type T. It handles rate limiting and retries.
func doJSONTypeG[T any](c *rlHTTPClient, urlv string, headers []string) ([]T, error) {
	var err error
	ctx, ctxcancel := context.WithTimeoutCause(c.Ctx, c.Timeout*5, err)
	defer ctxcancel()
	resp, err := c.Getdo(ctx, urlv, false, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Close()
	if err := logger.CheckContextEnded(ctx); err != nil {
		return nil, err
	}
	var v []T
	if err := json.NewDecoder(resp).DecodeContext(ctx, &v); err != nil {
		return nil, err
	}
	return v, nil
}
