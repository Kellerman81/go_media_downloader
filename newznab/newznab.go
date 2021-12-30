package newznab

import (
	"encoding/json"
	"encoding/xml"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/RussellLuo/slidingwindow"
	"github.com/pkg/errors"
	"golang.org/x/net/html/charset"
	"golang.org/x/time/rate"
)

//RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	client        *http.Client
	Ratelimiter   *rate.Limiter
	LimiterWindow *slidingwindow.Limiter
}

//Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoXml(url string, xmlobj interface{}) error {
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
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return errors.New("429")
	}

	d := xml.NewDecoder(resp.Body)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	errd := d.Decode(&xmlobj)
	if errd != nil {
		logger.Log.Error("Err Decode ", url, " error ", errd)
		return errd
	}
	return nil
}

//Do dispatches the HTTP request to the network
func (c *RLHTTPClient) DoJson(url string) (string, interface{}, error) {
	var retint interface{}
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
			return "", retint, errors.New("please wait")
		}
	}
	resp, err := c.client.Get(url)
	if err != nil {
		return "", retint, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return "", retint, errors.New("429")
	}
	var json1 SearchResponseJson1
	errd := json.NewDecoder(resp.Body).Decode(&json1)
	if errd != nil {
		var json2 SearchResponseJson2
		errd2 := json.NewDecoder(resp.Body).Decode(&json2)
		if errd2 != nil {
			logger.Log.Error("Err Decode ", url, " error ", errd)
			return "", retint, errd
		}
		return "json2", json2, nil
	}
	if json1.Title != "" {
		var json2 SearchResponseJson2
		errd2 := json.NewDecoder(resp.Body).Decode(&json2)
		if errd2 != nil {
			return "json1", json1, nil
		}
		return "json2", json2, nil
	}
	return "json1", json1, nil
}

//NewClient return http client with a ratelimiter
func NewRlClient(rl *rate.Limiter, rl2 *slidingwindow.Limiter) *RLHTTPClient {
	c := &RLHTTPClient{
		client:        &http.Client{Timeout: 10 * time.Second},
		Ratelimiter:   rl,
		LimiterWindow: rl2,
	}
	return c
}

// Client is a type for interacting with a newznab or torznab api
type Client struct {
	apikey     string
	apiBaseURL string
	apiUserID  int
	debug      bool
	client     *RLHTTPClient
}

// New returns a new instance of Client
func New(baseURL string, apikey string, userID int, insecure bool, debug bool, limitercalls int, limiterseconds int) Client {
	if limitercalls == 0 {
		limitercalls = 3
	}
	if limiterseconds == 0 {
		limiterseconds = 10
	}
	rl := rate.NewLimiter(rate.Every(time.Duration(limiterseconds)*time.Second), limitercalls) // 3 request every 10 seconds
	limiter, _ := slidingwindow.NewLimiter(time.Duration(limiterseconds)*time.Second, int64(limitercalls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })

	ret := Client{
		apikey:     apikey,
		apiBaseURL: baseURL,
		apiUserID:  userID,
		debug:      debug,
		client:     NewRlClient(rl, limiter),
	}
	return ret
}

// SearchWithTVDB returns NZBs for the given parameters
func (c Client) SearchWithTVDB(categories []int, tvDBID int, season int, episode int, additional_query_params string, customurl string, maxage int, outputasjson bool) ([]NZB, error) {
	var buildurl strings.Builder
	buildurl.Grow(150)
	if len(customurl) >= 1 {
		buildurl.WriteString(customurl)
	} else {
		buildurl.WriteString(c.apiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(c.apikey)
	}
	buildurl.WriteString("&tvdbid=")
	buildurl.WriteString(strconv.Itoa(tvDBID))
	buildurl.WriteString("&season=")
	buildurl.WriteString(strconv.Itoa(season))
	buildurl.WriteString("&ep=")
	buildurl.WriteString(strconv.Itoa(episode))
	buildurl.WriteString("&cat=")
	buildurl.WriteString(c.joinCats(categories))
	buildurl.WriteString("&dl=1&t=tvsearch")
	if outputasjson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(additional_query_params)

	return c.processurl(buildurl.String(), "", maxage, outputasjson)
}

// SearchWithIMDB returns NZBs for the given parameters
func (c Client) SearchWithIMDB(categories []int, imdbID string, additional_query_params string, customurl string, maxage int, outputasjson bool) ([]NZB, error) {
	var buildurl strings.Builder
	buildurl.Grow(150)
	if len(customurl) >= 1 {
		buildurl.WriteString(customurl)
	} else {
		buildurl.WriteString(c.apiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(c.apikey)
	}
	buildurl.WriteString("&imdbid=")
	buildurl.WriteString(imdbID)
	buildurl.WriteString("&cat=")
	buildurl.WriteString(c.joinCats(categories))
	buildurl.WriteString("&dl=1&t=movie")
	if outputasjson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(additional_query_params)

	return c.processurl(buildurl.String(), "", maxage, outputasjson)
}

// SearchWithQuery returns NZBs for the given parameters
func (c Client) SearchWithQuery(categories []int, query string, searchType string, addquotes bool, additional_query_params string, customurl string, maxage int, outputasjson bool) ([]NZB, error) {
	var buildurl strings.Builder
	buildurl.Grow(150)
	if len(customurl) >= 1 {
		buildurl.WriteString(customurl)
	} else {
		buildurl.WriteString(c.apiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(c.apikey)
	}
	buildurl.WriteString("&q=")
	if addquotes {
		buildurl.WriteString("%22")
	}
	buildurl.WriteString(url.PathEscape(query))
	if addquotes {
		buildurl.WriteString("%22")
	}
	buildurl.WriteString("&cat=")
	buildurl.WriteString(c.joinCats(categories))
	buildurl.WriteString("&dl=1&t=")
	buildurl.WriteString(searchType)
	if outputasjson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(additional_query_params)

	return c.processurl(buildurl.String(), "", maxage, outputasjson)
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (c Client) SearchWithQueryUntilNZBID(categories []int, query string, searchType string, addquotes bool, id string, additional_query_params string, customurl string, maxage int, outputasjson bool) ([]NZB, error) {
	var buildurl strings.Builder
	buildurl.Grow(150)
	if len(customurl) >= 1 {
		buildurl.WriteString(customurl)
	} else {
		buildurl.WriteString(c.apiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(c.apikey)
	}
	buildurl.WriteString("&q=")
	if addquotes {
		buildurl.WriteString("%22")
	}
	buildurl.WriteString(url.PathEscape(query))
	if addquotes {
		buildurl.WriteString("%22")
	}
	buildurl.WriteString("&cat=")
	buildurl.WriteString(c.joinCats(categories))
	buildurl.WriteString("&dl=1&t=")
	buildurl.WriteString(searchType)
	if outputasjson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(additional_query_params)

	partition, err := c.processurl(buildurl.String(), id, maxage, outputasjson)

	if err != nil {
		return nil, err
	}
	nzbs := make([]NZB, 0, len(partition))
	for idx := range partition {
		if partition[idx].ID == id && id != "" {
			return append(nzbs, partition[:idx]...), nil
		}
	}
	return append(nzbs, partition...), nil
}

// LoadRSSFeed returns up to <num> of the most recent NZBs of the given categories.
func (c Client) LoadRSSFeed(categories []int, num int, additional_query_params string, customapi string, customrssurl string, customrsscategory string, maxage int, outputasjson bool) ([]NZB, error) {
	return c.processurl(c.buildRssUrl(customrssurl, customrsscategory, customapi, additional_query_params, num, categories, 0, false), "", maxage, false)
}

func (c Client) joinCats(cats []int) string {
	var b strings.Builder
	b.Grow(30)
	for idx := range cats {
		if cats[idx] == 0 {
			continue
		}
		if b.Len() >= 1 {
			b.WriteString(",")
		}
		b.WriteString(strconv.Itoa(cats[idx]))
	}
	return b.String()
}

func (c Client) buildRssUrl(customrssurl string, customrsscategory string, customapi string, additional_query_params string, num int, categories []int, offset int, outputasjson bool) string {
	var buildurl strings.Builder
	buildurl.Grow(150)
	if len(customrssurl) >= 1 {
		buildurl.WriteString(customrssurl)
	} else if len(customapi) >= 1 {
		buildurl.WriteString(c.apiBaseURL)
		buildurl.WriteString(rssPath)
		buildurl.WriteString("?")
		buildurl.WriteString(customapi)
		buildurl.WriteString("=")
		buildurl.WriteString(c.apikey)
	} else {
		buildurl.WriteString(c.apiBaseURL)
		buildurl.WriteString(rssPath)
		buildurl.WriteString("?r=")
		buildurl.WriteString(c.apikey)
		buildurl.WriteString("&i=")
		buildurl.WriteString(strconv.Itoa(c.apiUserID))
	}
	buildurl.WriteString("&num=")
	buildurl.WriteString(strconv.Itoa(num))
	if len(customrsscategory) >= 1 {
		buildurl.WriteString("&")
		buildurl.WriteString(customrsscategory)
		buildurl.WriteString("=")
		buildurl.WriteString(c.joinCats(categories))
	} else {
		buildurl.WriteString("&t=")
		buildurl.WriteString(c.joinCats(categories))
	}
	buildurl.WriteString("&dl=1")
	if offset != 0 {
		buildurl.WriteString("&offset=")
		buildurl.WriteString(strconv.Itoa(offset))
	}
	if outputasjson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(additional_query_params)

	return buildurl.String()
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (c Client) LoadRSSFeedUntilNZBID(categories []int, num int, id string, maxRequests int, additional_query_params string, customapi string, customrssurl string, customrsscategory string, maxage int, outputasjson bool) ([]NZB, error) {
	count := 0
	//nzbcount := num
	//if maxRequests >= 1 {
	//	nzbcount = nzbcount * num
	//}
	var nzbs []NZB

	for {
		buildurl := c.buildRssUrl(customrssurl, customrsscategory, customapi, additional_query_params, num, categories, (num * count), false)

		partition, errp := c.processurl(buildurl, id, maxage, false)
		if errp == nil {
			for idx := range partition {
				if partition[idx].ID == id && id != "" {
					if count == 0 {
						return partition[:idx], nil
					}
					return append(nzbs, partition[:idx]...), nil
				}
			}
			nzbs = append(nzbs, partition...)
		} else {
			break
		}
		count++
		if maxRequests == 0 || count == maxRequests {
			break
		}
	}
	return nzbs, nil

}

func (c Client) processurl(url string, tillid string, maxage int, outputasjson bool) ([]NZB, error) {
	if outputasjson {
		rettype, feed, err := c.client.DoJson(url)
		if err != nil {
			return []NZB{}, err
		}
		if rettype == "json1" {
			return c.prepareNzbsJson1(feed.(SearchResponseJson1).Channel.Item, tillid, maxage), nil
		}
		if rettype == "json2" {
			return c.prepareNzbsJson2(feed.(SearchResponseJson2).Item, tillid, maxage), nil
		}
		return []NZB{}, nil
	} else {
		var feed SearchResponse
		err := c.client.DoXml(url, &feed)
		if err != nil {
			return []NZB{}, err
		}
		if c.debug {
			logger.Log.Debug("url: ", url)
			logger.Log.Debug("Results: ", len(feed.NZBs))
		}
		return c.prepareNzbs(feed.NZBs, tillid, maxage), nil
	}
}

func (c Client) prepareNzbs(nzbs []RawNZB, tillid string, maxage int) []NZB {
	scantime := time.Now()
	if maxage != 0 {
		scantime = scantime.AddDate(0, 0, 0-maxage)
	}
	entries := make([]NZB, 0, len(nzbs))
	for _, item := range nzbs {
		var newEntry NZB
		newEntry.Title = html.UnescapeString(item.Title)
		newEntry.DownloadURL = html.UnescapeString(item.Enclosure.URL)
		newEntry.SourceEndpoint = c.apiBaseURL
		newEntry.SourceAPIKey = c.apikey
		if item.Date != "" {
			newEntry.PubDate, _ = parseDate(item.Date)
			if maxage != 0 {
				if newEntry.PubDate.Before(scantime) {
					continue
				}
			}
		}
		newEntry.IsTorrent = false
		if strings.Contains(item.Enclosure.URL, ".torrent") || strings.Contains(item.Enclosure.URL, "magnet:?") {
			newEntry.IsTorrent = true
		}

		for idx := range item.Attributes {
			name := item.Attributes[idx].Name
			value := item.Attributes[idx].Value

			saveAttributes(&newEntry, name, value)
		}
		if newEntry.Size == 0 && item.Size != 0 {
			newEntry.Size = item.Size
		}
		if newEntry.ID == "" && item.GUID.GUID != "" {
			newEntry.ID = item.GUID.GUID
		} else if newEntry.ID == "" {
			newEntry.ID = item.Source.URL
		}
		entries = append(entries, newEntry)
		if tillid == newEntry.ID && tillid != "" {
			break
		}
	}
	return entries
}

func (c Client) prepareNzbsJson2(nzbs []RawNZBJson2, tillid string, maxage int) []NZB {
	scantime := time.Now()
	if maxage != 0 {
		scantime = scantime.AddDate(0, 0, 0-maxage)
	}
	entries := make([]NZB, 0, len(nzbs))
	for _, item := range nzbs {
		if len(item.Enclosure.URL) == 0 {
			continue
		}
		var newEntry NZB
		newEntry.Title = html.UnescapeString(item.Title)
		newEntry.DownloadURL = html.UnescapeString(item.Enclosure.URL)
		newEntry.SourceEndpoint = c.apiBaseURL
		newEntry.SourceAPIKey = c.apikey
		if item.Date != "" {
			newEntry.PubDate, _ = parseDate(item.Date)
			if maxage != 0 {
				if newEntry.PubDate.Before(scantime) {
					continue
				}
			}
		}
		newEntry.IsTorrent = false
		if strings.Contains(item.Enclosure.URL, ".torrent") || strings.Contains(item.Enclosure.URL, "magnet:?") {
			newEntry.IsTorrent = true
		}

		for idx := range item.Attributes {
			name := item.Attributes[idx].Name
			value := item.Attributes[idx].Value

			saveAttributes(&newEntry, name, value)
		}
		for idx := range item.Attributes2 {
			name := item.Attributes[idx].Name
			value := item.Attributes[idx].Value

			saveAttributes(&newEntry, name, value)
		}
		if newEntry.Size == 0 && item.Size != 0 {
			newEntry.Size = item.Size
		}
		if newEntry.ID == "" && item.GUID.GUID != "" {
			newEntry.ID = item.GUID.GUID
		} else if newEntry.ID == "" {
			newEntry.ID = item.Enclosure.URL
		}
		entries = append(entries, newEntry)
		if tillid == newEntry.ID && tillid != "" {
			break
		}
	}
	return entries
}

func (c Client) prepareNzbsJson1(nzbs []RawNZBJson1, tillid string, maxage int) []NZB {
	scantime := time.Now()
	if maxage != 0 {
		scantime = scantime.AddDate(0, 0, 0-maxage)
	}
	entries := make([]NZB, 0, len(nzbs))
	for _, item := range nzbs {
		if len(item.Enclosure.Attributes.URL) == 0 {
			continue
		}
		var newEntry NZB
		newEntry.Title = html.UnescapeString(item.Title)
		newEntry.DownloadURL = html.UnescapeString(item.Enclosure.Attributes.URL)
		newEntry.SourceEndpoint = c.apiBaseURL
		newEntry.SourceAPIKey = c.apikey
		if item.Date != "" {
			newEntry.PubDate, _ = parseDate(item.Date)
			if maxage != 0 {
				if newEntry.PubDate.Before(scantime) {
					continue
				}
			}
		}
		newEntry.IsTorrent = false
		if strings.Contains(item.Enclosure.Attributes.URL, ".torrent") || strings.Contains(item.Enclosure.Attributes.URL, "magnet:?") {
			newEntry.IsTorrent = true
		}

		for idx := range item.Attributes {
			name := item.Attributes[idx].Attribute.Name
			value := item.Attributes[idx].Attribute.Value

			saveAttributes(&newEntry, name, value)
		}
		if newEntry.Size == 0 && item.Size != 0 {
			newEntry.Size = item.Size
		}
		if newEntry.ID == "" && item.Guid != "" {
			newEntry.ID = item.Guid
		} else if newEntry.ID == "" {
			newEntry.ID = item.Enclosure.Attributes.URL
		}
		entries = append(entries, newEntry)
		if tillid == newEntry.ID && tillid != "" {
			break
		}
	}
	return entries
}

func saveAttributes(newEntry *NZB, name string, value string) {
	switch name {

	case "guid":
		newEntry.ID = value
	case "genre":
		newEntry.Genre = value
	case "tvdbid":
		newEntry.TVDBID = value
	case "info":
		newEntry.Info = value
	case "season":
		newEntry.Season = value
	case "episode":
		newEntry.Episode = value
	case "tvtitle":
		newEntry.TVTitle = value
	case "imdb":
		newEntry.IMDBID = value
	case "imdbtitle":
		newEntry.IMDBTitle = value
	case "coverurl":
		newEntry.CoverURL = value
	case "resolution":
		newEntry.Resolution = value
	case "poster":
		newEntry.Poster = value
	case "group":
		newEntry.Group = value
	case "infohash":
		newEntry.InfoHash = value
		newEntry.IsTorrent = true
	case "category":
		newEntry.Category = append(newEntry.Category, value)
	case "tvairdate":
		newEntry.AirDate, _ = parseDate(value)
	case "usenetdate":
		newEntry.UsenetDate, _ = parseDate(value)
	case "size":
		intValue, _ := strconv.ParseInt(value, 10, 64)
		newEntry.Size = intValue
	case "grabs":
		intValue, _ := strconv.ParseInt(value, 10, 64)
		newEntry.NumGrabs = int(intValue)
	case "seeders":
		intValue, _ := strconv.ParseInt(value, 10, 64)
		newEntry.Seeders = int(intValue)
		newEntry.IsTorrent = true
	case "peers":
		intValue, _ := strconv.ParseInt(value, 10, 64)
		newEntry.Peers = int(intValue)
		newEntry.IsTorrent = true
	case "rating":
		intValue, _ := strconv.ParseInt(value, 10, 64)
		newEntry.Rating = int(intValue)
	case "imdbyear":
		intValue, _ := strconv.ParseInt(value, 10, 64)
		newEntry.IMDBYear = int(intValue)
	case "imdbscore":
		parsedFloat, _ := strconv.ParseFloat(value, 32)
		newEntry.IMDBScore = float32(parsedFloat)
	}
}

const (
	apiPath = "/api"
	rssPath = "/rss"
)

// parseDate attempts to parse a date string
func parseDate(date string) (time.Time, error) {
	formats := []string{time.RFC3339, time.RFC1123Z}
	var parsedTime time.Time
	var err error
	for idx := range formats {
		if parsedTime, err = time.Parse(formats[idx], date); err == nil {
			return parsedTime, nil
		}
	}
	return parsedTime, errors.Errorf("failed to parse date %s as one of %s", date, strings.Join(formats, ", "))
}
