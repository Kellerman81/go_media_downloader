package apiexternal

import (
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/net/html/charset"
)

// RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	client *http.Client
	//Ratelimiter         rate.Limiter
	//DailyRatelimiter    rate.Limiter
	DailyLimiterEnabled bool
	Ratelimiter         *slidingwindow.Limiter
	DailyRatelimiter    *slidingwindow.Limiter
}

type addHeader struct {
	key string
	val string
}

const errorCalling = "Error calling"

var (
	WebClient = &http.Client{Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:          20,
			MaxConnsPerHost:       10,
			DisableCompression:    false,
			DisableKeepAlives:     true,
			IdleConnTimeout:       120 * time.Second}}
)

func (c *RLHTTPClient) checkLimiter(allow bool, retrycount int, retryafterseconds int64) (bool, error) {
	waituntil := (time.Duration(retryafterseconds) * time.Second)
	waituntilmax := (time.Duration(retryafterseconds*int64(retrycount)) * time.Second)
	rand.New(rand.NewSource(time.Now().UnixNano()))
	waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
	var waitfor time.Duration
	var ok bool
	if c.DailyLimiterEnabled {
		if ok, waitfor = c.DailyRatelimiter.Check(); !ok {
			logger.Log.Debug().Dur("waitfor", waitfor).Msg("Hit rate limit - Daily limit reached (dont retry)")
			//logger.LogAnyDebug("Hit rate limit - Daily limit reached (dont retry)", logger.LoggerValue{Name: "waitfor", Value: waitfor})
			return false, logger.ErrDailyLimit
		}
	}
	// ok, waitfor := c.Ratelimiter.CheckDaily()
	// if !ok {

	// }
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
			logger.Log.Debug().Dur("waitfor", waitfor).Msg("Hit rate limit - Should wait for (dont retry)")
			//logger.LogAnyDebug("Hit rate limit - limit reached (dont retry)", logger.LoggerValue{Name: "waitfor", Value: waitfor})
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
	logger.Log.Error().Msg("Hit rate limit - retrys failed")

	return false, logger.ErrToWait
}

func do(c *RLHTTPClient, url *string, headers *[]addHeader) (*http.Response, error) {
	return c.client.Do(getrequest(url, headers))
}

// Do dispatches the HTTP request to the network
func DoJSONType[T any](c *RLHTTPClient, url string, headers ...addHeader) (*T, error) {
	defer logger.Clear(&headers)
	if url == "" {
		return nil, logger.ErrNotFound
	}
	defer func() { // recovers panic
		if e := recover(); e != nil {
			logger.Log.Error().Msgf("Recovered from panic (json) %v", e)
		}
	}()
	ok, err := c.checkLimiter(true, 20, 1)
	if !ok {
		logerror(err, &url)
		if err == logger.ErrDailyLimit {
			return nil, nil
		}
		return nil, err
	}

	resp, err := do(c, &url, &headers)
	if err != nil {
		logerror(err, &url)
		c.addwait(&url, resp)
		return nil, err
	}

	defer resp.Body.Close()
	if c.addwait(&url, resp) {
		return nil, logger.ErrToWait
	}
	var u T
	return &u, json.NewDecoder(resp.Body).Decode(&u)
}

func logerror(err error, urlv *string) {
	if err != nil {
		if err != logger.ErrToWait && err != logger.ErrDailyLimit {
			logger.Log.Error().Err(err).CallerSkipFrame(1).Str(logger.StrURL, *urlv).Msg(errorCalling)
		}
	}
}

func QueryEscape(name *string) string {
	return url.QueryEscape(*name)
}

type xmlrow struct {
	results    *[]NZB
	startv     int64
	name       string
	b          NZB
	i          int
	j          int
	indexer    string
	quality    string
	apiBaseURL string
	tillid     string
}

// Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoXML(indexer string, quality string, tillid string, apiBaseURL string, url string) (*[]NZB, bool, error) {
	if url == "" {
		return nil, false, logger.ErrNotFound
	}
	ok, err := c.checkLimiter(true, 20, 1)
	if !ok {
		if err == nil {
			return nil, false, logger.ErrToWait
		}
		return nil, false, err
	}

	resp, err := do(c, &url, nil)
	if err != nil {
		logerror(err, &url)
		c.addwait(&url, resp)
		return nil, false, err
	}
	defer resp.Body.Close()
	if c.addwait(&url, resp) {
		return nil, false, logger.ErrToWait
	}
	d := xml.NewDecoder(resp.Body)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false

	nzbs := make([]NZB, 0, 50)
	row := xmlrow{tillid: tillid, indexer: indexer, quality: quality, apiBaseURL: apiBaseURL, results: &nzbs}
	var t xml.Token

	for {
		t, err = d.RawToken()
		if err != nil {
			break
		}
		switch tt := t.(type) {
		case xml.StartElement:
			startelement(d, &tt, &row)
		case xml.CharData:
			chardata(&tt, &row)
		case xml.EndElement:
			if endelement(&tt, &row) {
				logger.ClearVar(d)
				logger.ClearVar(&row)
				return &nzbs, false, nil
			}
		}
	}
	logger.ClearVar(d)
	logger.ClearVar(&row)
	return &nzbs, false, nil
}

func chardata(tt *xml.CharData, row *xmlrow) {
	if row.startv == 0 || row.name == "" { //endv is previous
		return
	}
	if strings.EqualFold(row.name, "title") {
		if row.b.Title == "" {
			row.b.Title = logger.UnquoteS(string(*tt))
			logger.HTMLUnescape(&row.b.Title)
		}
		return
	}
	if strings.EqualFold(row.name, "guid") {
		if row.b.ID == "" {
			row.b.ID = string(*tt)
		}
		return
	}
	if strings.EqualFold(row.name, "size") {
		if row.b.Size == 0 {
			row.b.Size = logger.StringToInt64(string(*tt))
		}
		return
	}
	// switch row.name {
	// case "title":
	// 	if row.b.Title == "" {
	// 		row.b.Title = logger.UnquoteS(string(*tt))
	// 		logger.HTMLUnescape(&row.b.Title)
	// 	}
	// case "guid":
	// 	if row.b.ID == "" {
	// 		row.b.ID = string(*tt)
	// 	}
	// case "size":
	// 	if row.b.Size == 0 {
	// 		row.b.Size = logger.StringToInt64(string(*tt))
	// 	}
	// }
}
func startelement(d *xml.Decoder, tt *xml.StartElement, row *xmlrow) {
	if tt.Name.Local == "item" {
		row.startv = d.InputOffset()
		row.b = NZB{Indexer: row.indexer, Quality: row.quality, SourceEndpoint: row.apiBaseURL}
		return
	}
	if row.startv <= 0 {
		return
	}
	row.name = tt.Name.Local //strings.ToLower(tt.Name.Local)
	if strings.EqualFold(row.name, "enclosure") {
		row.i = -1
		for idxi := range tt.Attr {
			if strings.EqualFold(tt.Attr[idxi].Name.Local, "url") {
				row.i = idxi
				break
			}
		}
		//i = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "url") })
		if row.i == -1 {
			return
		}
		row.b.DownloadURL = tt.Attr[row.i].Value
		logger.HTMLUnescape(&row.b.DownloadURL)
		if logger.ContainsI(row.b.DownloadURL, ".torrent") || logger.ContainsI(row.b.DownloadURL, "magnet:?") {
			row.b.IsTorrent = true
		}
		return
	}
	if strings.EqualFold(row.name, "attr") {
		row.i, row.j = -1, -1
		for idxi := range tt.Attr {
			if strings.EqualFold(tt.Attr[idxi].Name.Local, "name") {
				row.i = idxi
			}
			if strings.EqualFold(tt.Attr[idxi].Name.Local, "value") {
				row.j = idxi
			}
		}
		//i = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "name") })
		//j = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "value") })

		if row.i == -1 || row.j == -1 || tt.Attr[row.j].Value == "" {
			return
		}
		if strings.EqualFold(tt.Attr[row.i].Value, "size") {
			row.b.Size = logger.StringToInt64(tt.Attr[row.j].Value)
			return
		}
		if strings.EqualFold(tt.Attr[row.i].Value, "guid") {
			if row.b.ID == "" {
				row.b.ID = tt.Attr[row.j].Value
			}
			return
		}
		if strings.EqualFold(tt.Attr[row.i].Value, "tvdbid") {
			row.b.TVDBID = logger.StringToInt(tt.Attr[row.j].Value)
			return
		}
		if strings.EqualFold(tt.Attr[row.i].Value, "season") {
			row.b.Season = tt.Attr[row.j].Value
			return
		}
		if strings.EqualFold(tt.Attr[row.i].Value, "episode") {
			row.b.Episode = tt.Attr[row.j].Value
			return
		}
		if strings.EqualFold(tt.Attr[row.i].Value, "imdb") {
			row.b.IMDBID = tt.Attr[row.j].Value
			return
		}
		// switch strings.ToLower(tt.Attr[row.i].Value) {
		// case "size":
		// 	row.b.Size = logger.StringToInt64(tt.Attr[row.j].Value)
		// case "guid":
		// 	if row.b.ID == "" {
		// 		row.b.ID = tt.Attr[row.j].Value
		// 	}
		// case "tvdbid":
		// 	row.b.TVDBID = logger.StringToInt(tt.Attr[row.j].Value)
		// case "season":
		// 	row.b.Season = tt.Attr[row.j].Value
		// case "episode":
		// 	row.b.Episode = tt.Attr[row.j].Value
		// case "imdb":
		// 	row.b.IMDBID = tt.Attr[row.j].Value
		// }
		return
	}
	// switch row.name {
	// case "enclosure":
	// 	row.i = -1
	// 	for idxi := range tt.Attr {
	// 		if strings.EqualFold(tt.Attr[idxi].Name.Local, "url") {
	// 			row.i = idxi
	// 			break
	// 		}
	// 	}
	// 	//i = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "url") })
	// 	if row.i == -1 {
	// 		return
	// 	}
	// 	row.b.DownloadURL = tt.Attr[row.i].Value
	// 	logger.HTMLUnescape(&row.b.DownloadURL)
	// 	if logger.ContainsI(row.b.DownloadURL, ".torrent") || logger.ContainsI(row.b.DownloadURL, "magnet:?") {
	// 		row.b.IsTorrent = true
	// 	}
	// case "attr":
	// 	row.i, row.j = -1, -1
	// 	for idxi := range tt.Attr {
	// 		if strings.EqualFold(tt.Attr[idxi].Name.Local, "name") {
	// 			row.i = idxi
	// 		}
	// 		if strings.EqualFold(tt.Attr[idxi].Name.Local, "value") {
	// 			row.j = idxi
	// 		}
	// 	}
	// 	//i = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "name") })
	// 	//j = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "value") })

	// 	if row.i == -1 || row.j == -1 || tt.Attr[row.j].Value == "" {
	// 		return
	// 	}
	// 	switch strings.ToLower(tt.Attr[row.i].Value) {
	// 	case "size":
	// 		row.b.Size = logger.StringToInt64(tt.Attr[row.j].Value)
	// 	case "guid":
	// 		if row.b.ID == "" {
	// 			row.b.ID = tt.Attr[row.j].Value
	// 		}
	// 	case "tvdbid":
	// 		row.b.TVDBID = logger.StringToInt(tt.Attr[row.j].Value)
	// 	case "season":
	// 		row.b.Season = tt.Attr[row.j].Value
	// 	case "episode":
	// 		row.b.Episode = tt.Attr[row.j].Value
	// 	case "imdb":
	// 		row.b.IMDBID = tt.Attr[row.j].Value
	// 	}
	// }
}
func endelement(tt *xml.EndElement, row *xmlrow) bool {
	if tt.Name.Local == "item" {
		if row.startv == 0 || row.b.DownloadURL == "" {
			return false //Switch not for
		}
		row.startv = 0
		if row.b.ID == "" {
			row.b.ID = row.b.DownloadURL
		}
		*row.results = append(*row.results, row.b)
		row.b.DownloadURL = ""
		if row.tillid != "" && row.tillid == row.b.ID {
			return true
		}
	}
	return false
}

func (c *RLHTTPClient) testsleep(s string) bool {
	if sleep, err := strconv.ParseInt(s, 10, 64); err == nil {
		c.Ratelimiter.WaitTill(logger.TimeGetNow().Add((time.Second * time.Duration(sleep)) - c.Ratelimiter.Interval()))
		return true
	}
	if sleeptime, err := time.Parse(time.RFC1123, s); err == nil {
		c.Ratelimiter.WaitTill(sleeptime.Add(-c.Ratelimiter.Interval()))
		return true
	}
	return false
}
func (c *RLHTTPClient) addwait(url *string, resp *http.Response) bool {
	if resp == nil {
		return true
	}
	blockinterval := 5
	if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
		blockinterval = 1 * config.SettingsGeneral.FailedIndexerBlockTime
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
			c.Ratelimiter.WaitTill(time.Now().Add((time.Minute * time.Duration(blockinterval))))
		}
		//logger.LogAnyError(nil, "error get response url", logger.LoggerValue{Name: "url", Value: url}, logger.LoggerValue{Name: "status", Value: resp.Status})
		logger.Log.Error().Str("url", *url).Str("status", resp.Status).Msg("error get response url")

		return true
	} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusBadRequest {
		//403 Forbidden
		//401 unauthorized
		//429 too many requests
		//400 bad request
		var limitincreased bool
		if s, ok := resp.Header["Retry-After"]; ok {
			limitincreased = c.testsleep(s[0])
		} else if s, ok := resp.Header["X-Retry-After"]; ok {
			limitincreased = c.testsleep(s[0])
		} else if resp.StatusCode == 400 && resp.Body != nil {
			b, _ := io.ReadAll(resp.Body)
			if logger.ContainsI(string(b), "Request limit reached") {
				c.Ratelimiter.WaitTill(time.Now().Add(3 * time.Hour))
				return true
			}
		}
		if !limitincreased {
			c.Ratelimiter.WaitTill(time.Now().Add(time.Minute * time.Duration(blockinterval)))
		}
		//logger.LogAnyError(nil, "error get response url", logger.LoggerValue{Name: "url", Value: url}, logger.LoggerValue{Name: "status", Value: resp.Status})
		logger.Log.Error().Str("url", *url).Str("status", resp.Status).Msg("error get response url")
		return true
	}
	return false
}

// NewClient return http client with a ratelimiter
func NewClient(skiptlsverify bool, disablecompression bool, rl *slidingwindow.Limiter, usedaily bool, rldaily *slidingwindow.Limiter, timeoutseconds int) *RLHTTPClient {
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
		Ratelimiter:         rl,
		DailyLimiterEnabled: usedaily,
		DailyRatelimiter:    rldaily,
	}
}

func getrequest(url *string, headers *[]addHeader) *http.Request {
	req := logger.HTTPGetRequest(url)
	if headers == nil {
		return req
	}
	for i := range *headers {
		req.Header.Add((*headers)[i].key, (*headers)[i].val)
	}
	return req
}
