package apiexternal

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"github.com/goccy/go-json"
)

// rlHTTPClient is a rate limited HTTP client struct.
// It contains fields for the underlying http.Client, name, timeouts,
// rate limiters, and enabling daily rate limiting.
type rlHTTPClient struct {
	Ctx                 context.Context
	Clientname          string                 // The name of the client
	Timeout             time.Duration          // The timeout duration
	Timeout5            time.Duration          // The timeout duration
	DailyLimiterEnabled bool                   // Whether daily rate limiting is enabled
	client              *http.Client           // The underlying HTTP client
	Ratelimiter         *slidingwindow.Limiter // The per-request rate limiter
	DailyRatelimiter    *slidingwindow.Limiter // The daily rate limiter
}

type NzbSlice struct {
	Arr []Nzbwithprio
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
	// traktAPI is a client for interacting with the Trakt API.
	traktAPI traktClient
	// tvdbAPI is a client for interacting with the TVDB API.
	tvdbAPI tvdbClient
	// tmdbAPI is a client for interacting with the TMDB API.
	tmdbAPI tmdbClient
	// omdbAPI is a client for interacting with the OMDb API.
	omdbAPI omdbClient
	// newznabClients is a slice of newznab client structs.
	newznabClients = logger.NewSyncMap[*client](10)

	// cl is a default HTTP client with rate limiting and timeouts.
	lim = slidingwindow.NewLimiter(1*time.Second, 10)
	cl  = newClient("defaultdownloader", true, true, &lim, false, nil, 30)

	tlsinsecure = tls.Config{InsecureSkipVerify: true}

	// fieldmap maps XML element numbers to field names.
	fieldmap = []string{
		"",
		strtitle,
		strlink,
		strguid,
		strsize,
	}
	bytesRequestLimitReached = []byte("Request limit reached")
	strtimefound             = "time found"
	nzbmu                    = sync.Mutex{}
)

// Add appends the given Nzbwithprio to the NzbSlice's Arr field, with synchronization
// to ensure thread-safety.
func (n *NzbSlice) Add(nzb *Nzbwithprio) {
	nzbmu.Lock()
	defer nzbmu.Unlock()
	n.Arr = append(n.Arr, *nzb)
}

// checkLimiter checks if the rate limiter allows a request. It handles retrying with increasing backoff if rate limited.
// allow forces an allowance if true. retrycount is the max number of retries.
// retryafterseconds is the initial backoff duration.
// Returns true if allowed, false if rate limited after retries.
func (c *rlHTTPClient) checkLimiter(_ context.Context, allow bool) (bool, error) {
	if c.DailyLimiterEnabled {
		if !c.DailyRatelimiter.CheckBool() {
			return false, errDailyLimit
		}
	}
	waituntil := (time.Duration(1) * time.Second)
	waituntilmax := (time.Duration(20) * time.Second)
	for range 20 {
		ok, waitfor := c.Ratelimiter.Check()
		if ok {
			if allow {
				c.Ratelimiter.AllowForce()
				if c.DailyLimiterEnabled {
					c.DailyRatelimiter.AllowForce()
				}
			}
			return true, nil
		}
		if waitfor == 0 {
			time.Sleep(waituntil)
			// break
		}
		time.Sleep(
			(time.Duration(rand.New(config.RandomizerSource).Intn(500)+10) * time.Millisecond),
		)
		if waitfor > waituntilmax {
			return false, logger.ErrToWait
		}
		time.Sleep(waitfor)
	}

	logger.LogDynamicany1String(
		"warn",
		"Hit rate limit - retrys failed",
		logger.StrURL,
		c.Clientname,
	)

	return false, logger.ErrToWait
}

// checklimiterwithdaily checks if the rate limiter and daily rate limiter
// allow a request. It attempts to check the rate limiter up to the
// specified number of retries and increasing backoff. If the daily rate
// limiter is hit, it returns true to indicate the daily limit was reached.
func (c *rlHTTPClient) checklimiterwithdaily(ctx context.Context) bool {
	ok, _ := c.checkLimiter(ctx, true)
	return !ok
}

// checkresperror checks the HTTP response status code and returns an error if the response indicates an error condition.
// If the response status code is not http.StatusOK, it calls addwait to handle the error and returns logger.ErrToWait.
// If the response Content-Type header is "text/html" and checkhtml is true, it returns logger.ErrNotAllowed.
// Otherwise, it returns nil.
func (c *rlHTTPClient) checkresperror(
	resp *http.Response,
	req *http.Request,
	checkhtml bool,
) error {
	if resp.StatusCode != http.StatusOK {
		if c.addwait(req, resp) {
			return logger.ErrToWait
		}
		return errors.New("http status error " + resp.Status)
	}
	if checkhtml && resp.Header.Get("Content-Type") == "text/html" {
		return logger.ErrNotAllowed
	}
	return nil
}

// addwait checks the HTTP response status code and waits an appropriate amount of time before retrying the request.
// If the response indicates a rate limit or error condition, it will wait the specified time before allowing the request to be retried.
// If the response is successful, it will return false to indicate the request can proceed.
// The Response will be invalid after this call.
func (c *rlHTTPClient) addwait(req *http.Request, resp *http.Response) bool {
	blockinterval := 5
	if config.GetSettingsGeneral().FailedIndexerBlockTime != 0 {
		blockinterval = config.GetSettingsGeneral().FailedIndexerBlockTime
	}
	if resp == nil {
		c.logwait(logger.TimeGetNow().Add(time.Duration(blockinterval)*time.Minute), nil)
		return true
	}

	switch resp.StatusCode {
	case http.StatusNotFound,
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		521,
		522,
		524,
		http.StatusNoContent:
		if resp.StatusCode != http.StatusNotFound {
			c.logwait(logger.TimeGetNow().Add(time.Duration(blockinterval)*time.Minute), nil)
		}
		logger.LogDynamicany2Str(
			"error",
			"error get response url",
			logger.StrURL,
			req.URL.String(),
			logger.StrStatus,
			resp.Status,
		)
		return true

	case http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusBadRequest:
		s, ok := resp.Header["Retry-After"]
		if !ok {
			s, ok = resp.Header["X-Retry-After"]
		}
		if ok {
			if strings.Contains(s[0], "Request limit reached. Retry in ") {
				a := strings.Split(
					logger.Trim(
						strings.Replace(s[0], "Request limit reached. Retry in ", "", 1),
						'.',
					),
					" ",
				)
				if len(a) != 2 {
					c.logwait(
						logger.TimeGetNow().Add(time.Duration(blockinterval)*time.Minute),
						&s[0],
					)
					return true
				}
				switch a[1] {
				case "minutes":
					c.logwait(
						logger.TimeGetNow().
							Add(time.Duration(logger.StringToDuration(a[0]))*time.Minute),
						nil,
					)
					return true
				case "hours":
					c.logwait(
						logger.TimeGetNow().
							Add(time.Duration(logger.StringToDuration(a[0]))*time.Hour),
						nil,
					)
					return true
				default:
					c.logwait(
						logger.TimeGetNow().Add(time.Duration(blockinterval)*time.Minute),
						&s[0],
					)
					return true
				}
			} else if sleep, err := strconv.Atoi(s[0]); err == nil {
				c.logwait(logger.TimeGetNow().Add((time.Duration(sleep)*time.Second)-c.Ratelimiter.Interval()), nil)
				return true
			} else if strings.ContainsRune(s[0], ' ') && strings.ContainsRune(s[0], ':') {
				if sleeptime, ok := logger.TryTimeParse(time.RFC1123, s[0]); ok {
					c.logwait(sleeptime.Add(-c.Ratelimiter.Interval()), nil)
					return true
				}
			} else if strings.ContainsRune(s[0], 'T') && strings.ContainsRune(s[0], ':') {
				if sleeptime, ok := logger.TryTimeParse(time.RFC3339, s[0]); ok {
					c.logwait(sleeptime.Add(-c.Ratelimiter.Interval()), nil)
					return true
				}
			} else {
				c.logwait(logger.TimeGetNow().Add(time.Duration(blockinterval)*time.Minute), &s[0])
				return true
			}
		} else if resp.StatusCode == 400 && resp.Body != nil {
			b, err := io.ReadAll(resp.Body)
			if err == nil && logger.ContainsByteI(b, bytesRequestLimitReached) {
				c.logwait(logger.TimeGetNow().Add(3*time.Hour), nil)
				return true
			}
			logger.LogDynamicany2Str("error", "error get response url", logger.StrURL, req.URL.String(), logger.StrStatus, resp.Status)
			return true
		}
		logger.LogDynamicany2Str(
			"error",
			"error get response url",
			logger.StrURL,
			req.URL.String(),
			logger.StrStatus,
			resp.Status,
		)
		return true
	}
	return false
}

// logwait logs a debug message with the specified wait time and optional log message.
// It waits until the specified time before returning.
func (c *rlHTTPClient) logwait(waitfor time.Time, logfound *string) {
	c.Ratelimiter.WaitTill(waitfor)
	logv := logger.Logtype("debug", 1).
		Time(logger.StrWaitfor, waitfor).
		Str(logger.StrURL, c.Clientname)
	if logfound != nil {
		logv.Str(strtimefound, *logfound)
	}
	logv.Msg("Set Waittill")
}

// newClient creates a new HTTP client for making external API requests. It configures rate limiting, TLS verification, compression, timeouts etc. based on the provided parameters.
func newClient(
	clientname string,
	skiptlsverify, disablecompression bool,
	rl *slidingwindow.Limiter,
	usedaily bool,
	rldaily *slidingwindow.Limiter,
	timeoutseconds uint16,
) rlHTTPClient {
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}
	var insecure *tls.Config
	if skiptlsverify {
		insecure = &tlsinsecure
	}

	return rlHTTPClient{
		Timeout:    time.Duration(timeoutseconds) * time.Second,
		Timeout5:   time.Duration(timeoutseconds*5) * time.Second,
		Clientname: clientname,
		client: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     false,
			DisableCompression:    disablecompression,
			TLSClientConfig:       insecure,
			MaxIdleConns:          50,
			MaxConnsPerHost:       50,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
			ResponseHeaderTimeout: time.Duration(timeoutseconds) * time.Second,
			DisableKeepAlives:     false,
		}},
		Ratelimiter:         rl,
		DailyLimiterEnabled: usedaily,
		DailyRatelimiter:    rldaily,
		Ctx:                 context.Background(),
	}
}

// ProcessHTTP is a helper function that makes a GET request to the provided URL,
// sets the specified headers, and runs the provided function with the HTTP response.
// The function uses a context with a timeout of 5 times the client's configured timeout.
// If the request fails, the function returns the error.
func ProcessHTTP(
	c *rlHTTPClient,
	urlv string,
	checklimiter bool,
	run func(context.Context, *http.Response) error,
	headers map[string][]string,
	body ...io.Reader,
) error {
	if c == nil {
		c = &cl
	}
	ctx, ctxcancel := context.WithTimeout(c.Ctx, cl.Timeout5)
	defer ctxcancel()

	if checklimiter {
		ok, err := c.checkLimiter(ctx, true)
		if !ok {
			if err == nil {
				return logger.ErrToWait
			}
			return err
		}
	}
	var req *http.Request
	var err error
	if len(body) >= 1 {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, urlv, body[0])
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, urlv, http.NoBody)
	}
	if err != nil {
		logger.LogDynamicany1StringErr(
			"error",
			"failed to get url",
			err,
			logger.StrURL,
			urlv,
		) // nopointer
		return err
	}

	if len(headers) >= 1 {
		req.Header = headers
	}
	resp, err := c.client.Do(req)
	if err != nil {
		logger.LogDynamicany1StringErr(
			"error",
			"failed to process url",
			err,
			logger.StrURL,
			urlv,
		) // nopointer
		return err
	}
	defer resp.Body.Close()
	err = c.checkresperror(resp, req, false)
	if err != nil {
		logger.LogDynamicany1StringErr(
			"error",
			"failed to process url",
			err,
			logger.StrURL,
			urlv,
		) // nopointer
		return err
	}
	return run(ctx, resp)
}

// doJSONType is a helper function that makes a GET request to the provided URL,
// sets the specified headers, and decodes the JSON response into the provided type.
// The function uses a context with a timeout of 5 times the client's configured timeout.
// If the request fails, the function returns the provided type and the error.
func doJSONType[S any](c *rlHTTPClient, urlv string, headers map[string][]string) (S, error) {
	var v S
	err := ProcessHTTP(c, urlv, true, func(ctx context.Context, resp *http.Response) error {
		return json.NewDecoder(resp.Body).DecodeContext(ctx, &v)
	}, headers)
	return v, err
}

// doJSONTypeNoLimit is a helper function that makes a GET request to the provided URL,
// sets the specified headers, and decodes the JSON response into the provided type.
// The function does not use a context with a timeout, unlike doJSONType.
// If the request fails, the function returns the provided type and the error.
func doJSONTypeNoLimit[S any](
	c *rlHTTPClient,
	urlv string,
	headers map[string][]string,
) (S, error) {
	var v S
	err := ProcessHTTP(c, urlv, false, func(ctx context.Context, resp *http.Response) error {
		return json.NewDecoder(resp.Body).DecodeContext(ctx, &v)
	}, headers)
	return v, err
}

// doJSONTypeP is a helper function that makes a GET request to the provided URL,
// sets the specified headers, and decodes the JSON response into a pointer to the provided type.
// The function uses a context with a timeout of 5 times the client's configured timeout.
// If the request fails, the function returns a nil pointer to the provided type and the error.
func doJSONTypeP[S any](c *rlHTTPClient, urlv string, headers map[string][]string) (*S, error) {
	v, err := doJSONType[S](c, urlv, headers)
	if err != nil {
		return nil, err
	}
	return &v, err
}

// ProcessHTTPNoRateCheck is like ProcessHTTP but completely bypasses all rate limiting and response error checking
// This is specifically for connectivity testing purposes
func ProcessHTTPNoRateCheck(
	c *rlHTTPClient,
	urlv string,
	run func(context.Context, *http.Response) error,
	headers map[string][]string,
	body ...io.Reader,
) error {
	if c == nil {
		c = &cl
	}
	ctx, ctxcancel := context.WithTimeout(c.Ctx, cl.Timeout5)
	defer ctxcancel()
	
	var req *http.Request
	var err error
	if len(body) >= 1 {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, urlv, body[0])
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, urlv, http.NoBody)
	}
	if err != nil {
		return err
	}
	
	// Set headers
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	
	// Make the request directly without any rate limiting or error checking
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// Run the callback function
	return run(ctx, resp)
}
