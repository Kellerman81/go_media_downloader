package newznab

import (
	"bytes"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/RussellLuo/slidingwindow"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

//RLHTTPClient Rate Limited HTTP Client
type RLHTTPClient struct {
	client        *http.Client
	Ratelimiter   *rate.Limiter
	LimiterWindow *slidingwindow.Limiter
}

//Do dispatches the HTTP request to the network
func (c *RLHTTPClient) Do(req *http.Request) (*http.Response, []byte, error) {
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
			return nil, nil, errors.New("please wait")
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, body, nil
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
func (c Client) SearchWithTVDB(categories []int, tvDBID int, season int, episode int, additional_query_params string, customurl string, maxage int) ([]NZB, error) {
	buildurl := ""
	if len(customurl) >= 1 {
		buildurl = customurl
	} else {
		buildurl = c.apiBaseURL + apiPath + "?apikey=" + c.apikey
	}
	buildurl += "&tvdbid=" + strconv.Itoa(tvDBID)
	buildurl += "&season=" + strconv.Itoa(season)
	buildurl += "&ep=" + strconv.Itoa(episode)
	buildurl += "&cat=" + c.joinCats(categories)
	buildurl += "&dl=1"
	buildurl += "&t=tvsearch"
	buildurl += additional_query_params

	return c.processurl(buildurl, "", maxage)
}

// SearchWithIMDB returns NZBs for the given parameters
func (c Client) SearchWithIMDB(categories []int, imdbID string, additional_query_params string, customurl string, maxage int) ([]NZB, error) {
	buildurl := ""
	if len(customurl) >= 1 {
		buildurl = customurl
	} else {
		buildurl = c.apiBaseURL + apiPath + "?apikey=" + c.apikey
	}
	buildurl += "&imdbid=" + imdbID
	buildurl += "&cat=" + c.joinCats(categories)
	buildurl += "&dl=1"
	buildurl += "&t=movie"
	buildurl += additional_query_params

	return c.processurl(buildurl, "", maxage)
}

// SearchWithQuery returns NZBs for the given parameters
func (c Client) SearchWithQuery(categories []int, query string, searchType string, addquotes bool, additional_query_params string, customurl string, maxage int) ([]NZB, error) {
	buildurl := ""
	if len(customurl) >= 1 {
		buildurl = customurl
	} else {
		buildurl = c.apiBaseURL + apiPath + "?apikey=" + c.apikey
	}
	buildurl += "&q="
	if addquotes {
		buildurl += "%22"
	}
	buildurl += url.PathEscape(query)
	if addquotes {
		buildurl += "%22"
	}
	buildurl += "&cat=" + c.joinCats(categories)
	buildurl += "&dl=1"
	buildurl += "&t=" + searchType
	buildurl += additional_query_params

	return c.processurl(buildurl, "", maxage)
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (c Client) SearchWithQueryUntilNZBID(categories []int, query string, searchType string, addquotes bool, id string, additional_query_params string, customurl string, maxage int) ([]NZB, error) {
	buildurl := ""
	if len(customurl) >= 1 {
		buildurl = customurl
	} else {
		buildurl = c.apiBaseURL + apiPath + "?apikey=" + c.apikey
	}
	buildurl += "&q="
	if addquotes {
		buildurl += "%22"
	}
	buildurl += url.PathEscape(query)
	if addquotes {
		buildurl += "%22"
	}
	buildurl += "&cat=" + c.joinCats(categories)
	buildurl += "&dl=1"
	buildurl += "&t=" + searchType
	buildurl += additional_query_params

	partition, err := c.processurl(buildurl, id, maxage)

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
func (c Client) LoadRSSFeed(categories []int, num int, additional_query_params string, customapi string, customrssurl string, customrsscategory string, maxage int) ([]NZB, error) {
	buildurl := c.BuildRssUrl(customrssurl, customrsscategory, customapi, additional_query_params, num, categories, 0)
	return c.processurl(buildurl, "", maxage)
}

func (c Client) joinCats(cats []int) string {
	var catstemp []string
	for idx := range cats {
		if cats[idx] == 0 {
			continue
		}
		catstemp = append(catstemp, strconv.Itoa(cats[idx]))
	}
	return strings.Join(catstemp, ",")
}

func (c Client) BuildRssUrl(customrssurl string, customrsscategory string, customapi string, additional_query_params string, num int, categories []int, offset int) string {
	var buildurl string
	if len(customrssurl) >= 1 {
		buildurl = customrssurl
		buildurl += "&num=" + strconv.Itoa(num)
		if len(customrsscategory) >= 1 {
			buildurl += "&" + customrsscategory + "=" + c.joinCats(categories)
		} else {
			buildurl += "&t=" + c.joinCats(categories)
		}
		buildurl += "&dl=1"
		if offset != 0 {
			buildurl += "&offset=" + strconv.Itoa(offset)
		}
		buildurl += additional_query_params
	} else if len(customapi) >= 1 {
		buildurl = c.apiBaseURL + rssPath + "?" + customapi + "=" + c.apikey
		buildurl += "&num=" + strconv.Itoa(num)
		if len(customrsscategory) >= 1 {
			buildurl += "&" + customrsscategory + "=" + c.joinCats(categories)
		} else {
			buildurl += "&t=" + c.joinCats(categories)
		}
		buildurl += "&dl=1"
		if offset != 0 {
			buildurl += "&offset=" + strconv.Itoa(offset)
		}
		buildurl += additional_query_params
	} else {
		buildurl = c.apiBaseURL + rssPath + "?r=" + c.apikey + "&i=" + strconv.Itoa(c.apiUserID)
		buildurl += "&num=" + strconv.Itoa(num)
		if len(customrsscategory) >= 1 {
			buildurl += "&" + customrsscategory + "=" + c.joinCats(categories)
		} else {
			buildurl += "&t=" + c.joinCats(categories)
		}
		buildurl += "&dl=1"
		if offset != 0 {
			buildurl += "&offset=" + strconv.Itoa(offset)
		}
		buildurl += additional_query_params
	}
	return buildurl
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (c Client) LoadRSSFeedUntilNZBID(categories []int, num int, id string, maxRequests int, additional_query_params string, customapi string, customrssurl string, customrsscategory string, maxage int) ([]NZB, error) {
	count := 0
	// nzbcount := num
	// if maxRequests >= 1 {
	// 	nzbcount = nzbcount * num
	// }
	var nzbs []NZB

	for {
		buildurl := c.BuildRssUrl(customrssurl, customrsscategory, customapi, additional_query_params, num, categories, (num * count))

		partition, errp := c.processurl(buildurl, id, maxage)
		if errp == nil {
			for idx := range partition {
				if partition[idx].ID == id && id != "" {
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

func (c Client) processurl(url string, tillid string, maxage int) ([]NZB, error) {
	var feed SearchResponse
	body, err := c.getURL(url)
	if err != nil {
		logger.Log.Error("Err Download ", url, " error ", err)
		return []NZB{}, err
	}
	d := xml.NewDecoder(bytes.NewReader(body))
	d.Strict = false
	errd := d.Decode(&feed)
	if errd != nil {
		logger.Log.Error("Err Decode ", url, " error ", errd)
		return []NZB{}, errd
	}
	entries := make([]NZB, 0, len(feed.NZBs))
	for _, item := range feed.NZBs {
		var newEntry NZB
		newEntry.Title = item.Title
		newEntry.DownloadURL = item.Enclosure.URL
		newEntry.SourceEndpoint = c.apiBaseURL
		newEntry.SourceAPIKey = c.apikey
		if item.Date != "" {
			newEntry.PubDate, _ = parseDate(item.Date)
			if maxage != 0 {
				scantime := time.Now()
				scantime = scantime.AddDate(0, 0, 0-maxage)
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

			switch name {

			case "tvairdate":
				newEntry.AirDate, _ = parseDate(value)
			case "guid":
				newEntry.ID = value
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
			case "infohash":
				newEntry.InfoHash = value
				newEntry.IsTorrent = true
			case "category":
				newEntry.Category = append(newEntry.Category, value)
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
			case "rating":
				intValue, _ := strconv.ParseInt(value, 10, 64)
				newEntry.Rating = int(intValue)
			case "imdb":
				newEntry.IMDBID = value
			case "imdbtitle":
				newEntry.IMDBTitle = value
			case "imdbyear":
				intValue, _ := strconv.ParseInt(value, 10, 64)
				newEntry.IMDBYear = int(intValue)
			case "imdbscore":
				parsedFloat, _ := strconv.ParseFloat(value, 32)
				newEntry.IMDBScore = float32(parsedFloat)
			case "coverurl":
				newEntry.CoverURL = value
			case "usenetdate":
				newEntry.UsenetDate, _ = parseDate(value)
			case "resolution":
				newEntry.Resolution = value
			}
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
	if c.debug {
		logger.Log.Debug("url: ", url)
		logger.Log.Debug("Results: ", len(feed.NZBs))
	}
	return entries, nil
}

const (
	apiPath = "/api"
	rssPath = "/rss"
)

func (c Client) getURL(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	resp, responseData, err := c.client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	if resp.StatusCode == 429 {
		return []byte{}, err
	}
	return responseData, nil
}

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
