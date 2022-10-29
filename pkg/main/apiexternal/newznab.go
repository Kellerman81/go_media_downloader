package apiexternal

import (
	"errors"
	"html"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/rate"
	"go.uber.org/zap"
)

// NzbIndexer defines the Indexers to query
type NzbIndexer struct {
	Name                    string
	URL                     string
	Apikey                  string
	UserID                  int
	SkipSslCheck            bool
	Addquotesfortitlequery  bool
	Additional_query_params string
	LastRssId               string
	Customapi               string
	Customurl               string
	Customrssurl            string
	Customrsscategory       string
	RssDownloadAll          bool
	OutputAsJson            bool
	Limitercalls            int
	Limiterseconds          int
	LimitercallsDaily       int
	MaxAge                  int
	TimeoutSeconds          int
}

func NewznabCheckLimiter(url string) (bool, time.Duration) {
	if checkclient(url) {
		client := getclient(url)
		var ok bool
		var waitfor time.Duration
		if client.Client.DailyLimiterEnabled {
			ok, waitfor = client.Client.DailyRatelimiter.Check()
			if !ok {
				return ok, waitfor
			}
		}

		for i := 0; i < 10; i++ {
			ok, waitfor = client.Client.Ratelimiter.Check()
			if ok {
				return true, 0
			}
			if waitfor > (10 * time.Second) {
				//logger.Log.GlobalLogger.Warn("Hit rate limit - Should wait for (dont retry)", zap.Duration("waitfor", waitfor), zap.String("Url", url))
				return false, waitfor
			}
			if waitfor == 0 {
				waitfor = (1 * time.Second)
			} else {
				rand.Seed(time.Now().UnixNano())
				waitfor = waitfor + (time.Duration(rand.Intn(500)+10) * time.Millisecond)
			}
			time.Sleep(waitfor)
		}
	}
	return true, 0
}

func (client *Client) buildUrl(rss bool, searchtype string, query string, addquotesfortitlequery bool, imdbid string, tvdbid int, useseason bool, season int, useepisode bool, episode int, outputAsJson bool, customurl string, customrsscategory string, customapi string, additional_query_params string, limit string, num int, categories string, offset int) string {
	path := apiPath
	if rss {
		path = rssPath
	}
	urlv := ""
	if len(customurl) >= 1 {
		urlv = customurl
	} else if len(customapi) >= 1 {
		urlv = client.ApiBaseURL + path + "?" + customapi + "=" + client.Apikey
	} else {
		if rss {
			urlv = client.ApiBaseURL + path + "?r=" + client.Apikey + "&i=" + strconv.FormatInt(int64(client.ApiUserID), 10)
		} else {
			urlv = client.ApiBaseURL + path + "?apikey=" + client.Apikey
		}
	}
	if useseason {
		urlv += "&season=" + strconv.FormatInt(int64(season), 10)
	}
	if useepisode {
		urlv += "&ep=" + strconv.FormatInt(int64(episode), 10)
	}
	if limit != "0" && limit != "" {
		urlv += "&limit=" + limit
	}
	if imdbid != "" {
		urlv += "&imdbid=" + imdbid
	}
	if tvdbid != 0 {
		urlv += "&tvdbid=" + strconv.Itoa(tvdbid)
	}
	if categories != "" {
		if rss {
			if len(customrsscategory) >= 1 {
				urlv += "&" + customrsscategory + "=" + categories
			} else {
				urlv += "&t=" + categories
			}
		} else {
			urlv += "&cat=" + categories
		}
	}

	if offset != 0 {
		urlv += "&offset=" + strconv.FormatInt(int64(offset), 10)
	}

	if num != 0 {
		urlv += "&num=" + strconv.Itoa(num)
	}

	if searchtype != "" {
		urlv += "&t=" + searchtype
	}

	if query != "" {
		if addquotesfortitlequery {
			urlv += "&q=%22" + url.PathEscape(query) + "%22"
		} else {
			urlv += "&q=" + url.PathEscape(query)
		}
	}
	if outputAsJson {
		urlv += "&o=json"
	}
	return urlv + "&dl=1" + additional_query_params
	// url := urlv + "&imdbid=" + imdbid + "&cat=" + categories + "&dl=1&t=movie" + json + additional_query_params //movie

	// url = urlv + "&tvdbid=" + strconv.Itoa(tvdbid) + seasonstr + episodestr + limitstr + "&cat=" + categories + "&dl=1&t=tvsearch" + json + additional_query_params //tv

	// url = urlv + "&q=" + query + "&cat=" + categories + "&dl=1&t=" + searchtype + json + additional_query_params //query
	// url = urlv + "&q=" + query + "&cat=" + categories + "&dl=1&t=" + searchtype + json + additional_query_params //queryuntil
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(row *NzbIndexer, imdbid string, categories string) (resultsadd *NZBArr, broke bool, erradd error) {
	if imdbid == "" {
		erradd = errors.New("no imdbid")
		return
	}

	client := getnewznabclient(row)
	return client.processurl(client.buildUrl(false, "movie", "", false, imdbid, 0, false, 0, false, 0, row.OutputAsJson, row.Customurl, "", row.Customapi, row.Additional_query_params, "0", 0, categories, 0), "", row.MaxAge, row.OutputAsJson, nil)
}

func getnewznabclient(row *NzbIndexer) *Client {
	if checkclient(row.URL) {
		return getclient(row.URL)
	} else {
		client := NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds, row.LimitercallsDaily, row.TimeoutSeconds)
		newznabClients = append(newznabClients, Clients{Name: row.URL, Client: client})
		return client
	}
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(row *NzbIndexer, tvdbid int, categories string, season int, episode int, useseason bool, useepisode bool) (resultsadd *NZBArr, broke bool, erradd error) {
	if tvdbid == 0 {
		erradd = errors.New("no tvdbid")
		return
	}
	client := getnewznabclient(row)

	limitstr := ""
	if !useepisode || !useseason {
		limitstr = "100"
	}

	return client.processurl(client.buildUrl(false, "tvsearch", "", false, "", tvdbid, useseason, season, useepisode, episode, row.OutputAsJson, row.Customurl, "", row.Customapi, row.Additional_query_params, limitstr, 0, categories, 0), "", row.MaxAge, row.OutputAsJson, nil)
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(row *NzbIndexer, query string, categories string, searchtype string) (resultsadd *NZBArr, broke bool, erradd error) {
	client := getnewznabclient(row)

	return client.processurl(client.buildUrl(false, searchtype, query, row.Addquotesfortitlequery, "", 0, false, 0, false, 0, row.OutputAsJson, row.Customurl, row.Customrsscategory, row.Customapi, row.Additional_query_params, "0", 0, categories, 0), "", row.MaxAge, row.OutputAsJson, nil)
}

type Clients struct {
	Name   string
	Client *Client
}

var newznabClients []Clients

func checkclient(find string) bool {
	for idx := range newznabClients {
		if newznabClients[idx].Name == find {
			return true
		}
	}
	return false
}

func getclient(find string) *Client {
	for idx := range newznabClients {
		if newznabClients[idx].Name == find {
			return newznabClients[idx].Client
		}
	}
	return nil
}

// QueryNewznabRSS returns x entries of given category
func QueryNewznabRSS(row *NzbIndexer, maxitems int, categories string) (resultsadd *NZBArr, broke bool, erradd error) {

	client := getnewznabclient(row)
	return client.processurl(client.buildRssUrl(row.Customrssurl, row.Customrsscategory, row.Customapi, row.Additional_query_params, maxitems, categories, 0, false), "", 0, false, nil)
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(row *NzbIndexer, maxitems int, categories string, maxrequests int) (resultsadd *NZBArr, failedindexers string, lastid string, erradd error) {

	client := getnewznabclient(row)
	count := 0
	baseurl := client.buildRssUrl(row.Customrssurl, row.Customrsscategory, row.Customapi, row.Additional_query_params, maxitems, categories, 0, false)

	var broke bool
	resultsadd, broke, erradd = client.processurl(baseurl, row.LastRssId, 0, false, nil)
	if erradd != nil {
		return
	}
	if broke || len((resultsadd.Arr)) == 0 {
		return
	}
	count++
	for {
		_, broke, erradd = client.processurl(baseurl+"&offset="+strconv.Itoa(maxitems*count), row.LastRssId, 0, false, resultsadd)
		if erradd != nil {
			break
		}
		count++
		if maxrequests == 0 || count >= maxrequests || broke || len((resultsadd.Arr)) == 0 {
			break
		}
	}

	if erradd != nil {
		return resultsadd, row.URL, "", erradd
	} else {
		if len(resultsadd.Arr) >= 1 {
			return resultsadd, "", resultsadd.Arr[0].NZB.ID, nil
		}
	}
	return resultsadd, "", "", nil
}

// Client is a type for interacting with a newznab or torznab api
type Client struct {
	Apikey     string
	ApiBaseURL string
	ApiUserID  int
	Debug      bool
	Client     *RLHTTPClient
}

// New returns a new instance of Client
func NewNewznab(baseURL string, apikey string, userID int, insecure bool, debug bool, limitercalls int, limiterseconds int, limitercallsdaily int, timeoutseconds int) *Client {
	if limitercalls == 0 {
		limitercalls = 3
	}
	if limiterseconds == 0 {
		limiterseconds = 10
	}
	return &Client{
		Apikey:     apikey,
		ApiBaseURL: baseURL,
		ApiUserID:  userID,
		Debug:      debug,
		Client: NewClient(
			insecure,
			rate.New(limitercalls, limitercallsdaily, time.Duration(limiterseconds)*time.Second), timeoutseconds,
		),
	}
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (c *Client) SearchWithQueryUntilNZBID(categories string, query string, searchType string, addquotes bool, id string, additional_query_params string, customurl string, maxage int, outputasjson bool) (resultsadd *NZBArr, broke bool, erradd error) {
	return c.processurl(c.buildUrl(false, searchType, query, addquotes, "", 0, false, 0, false, 0, outputasjson, customurl, "", "", additional_query_params, "0", 0, categories, 0), id, maxage, outputasjson, nil)
}

func (c *Client) buildRssUrl(customrssurl string, customrsscategory string, customapi string, additional_query_params string, num int, categories string, offset int, outputasjson bool) string {
	return c.buildUrl(true, "", "", false, "", 0, false, 0, false, 0, outputasjson, customrssurl, customrsscategory, customapi, additional_query_params, "0", num, categories, offset) //buildurl.String()
}

func addentrySearchResponseXml(tillid string, apiBaseURL string, item *RawNZB, entries *NZBArr) (breakatid bool, skip bool) {
	if len(item.Enclosure.URL) == 0 {
		skip = true
		return
	}
	var newEntry Nzbwithprio
	newEntry.NZB.Title = item.Title
	if strings.Contains(newEntry.NZB.Title, "&") || strings.Contains(newEntry.NZB.Title, "%") {
		newEntry.NZB.Title = html.UnescapeString(newEntry.NZB.Title)
	} else {
		if strings.Contains(newEntry.NZB.Title, "\\u") {
			unquote, err := strconv.Unquote("\"" + newEntry.NZB.Title + "\"")
			if err == nil {
				newEntry.NZB.Title = unquote
			}
		}
	}
	newEntry.NZB.DownloadURL = item.Enclosure.URL
	if strings.Contains(newEntry.NZB.DownloadURL, "&amp") || strings.Contains(newEntry.NZB.DownloadURL, "%") {
		newEntry.NZB.DownloadURL = html.UnescapeString(newEntry.NZB.DownloadURL)
	}
	newEntry.NZB.SourceEndpoint = apiBaseURL
	if strings.Contains(newEntry.NZB.DownloadURL, ".torrent") || strings.Contains(newEntry.NZB.DownloadURL, "magnet:?") {
		newEntry.NZB.IsTorrent = true
	}

	for idx2 := range item.Attributes {
		saveAttributes(&newEntry.NZB, item.Attributes[idx2].Name, item.Attributes[idx2].Value)
	}
	if newEntry.NZB.Size == 0 && item.Size != 0 {
		newEntry.NZB.Size = item.Size
	}
	newEntry.NZB.ID = item.GUID.GUID
	if newEntry.NZB.ID == "" {
		newEntry.NZB.ID = item.Enclosure.URL
	}
	if tillid == newEntry.NZB.ID && tillid != "" {
		breakatid = true
	}
	entries.Arr = append(entries.Arr, newEntry)
	return
}
func addentrySearchResponseJson1(tillid string, apiBaseURL string, item *RawNZBJson1, entries *NZBArr) (breakatid bool, skip bool) {
	item_convert := RawNZB{}
	item_convert.Enclosure.URL = item.Enclosure.Attributes.URL
	item_convert.Title = item.Title
	item_convert.Size = item.Size
	item_convert.GUID.GUID = item.Guid
	item_convert.Attributes = make([]struct {
		Name  string "xml:\"name,attr\""
		Value string "xml:\"value,attr\""
	}, len(item.Attributes))
	for idx2 := range item.Attributes {
		item_convert.Attributes[idx2].Name = item.Attributes[idx2].Attribute.Name
		item_convert.Attributes[idx2].Value = item.Attributes[idx2].Attribute.Value
	}
	addentrySearchResponseXml(tillid, apiBaseURL, &item_convert, entries)
	return
}
func addentrySearchResponseJson2(tillid string, apiBaseURL string, item *RawNZBJson2, entries *NZBArr) (breakatid bool, skip bool) {
	item_convert := RawNZB{}
	item_convert.Enclosure.URL = item.Enclosure.URL
	item_convert.Title = item.Title
	item_convert.Size = item.Size
	item_convert.GUID.GUID = item.GUID.GUID
	item_convert.Attributes = make([]struct {
		Name  string "xml:\"name,attr\""
		Value string "xml:\"value,attr\""
	}, len(item.Attributes)+len(item.Attributes2))
	for idx2 := range item.Attributes {
		item_convert.Attributes[idx2].Name = item.Attributes[idx2].Name
		item_convert.Attributes[idx2].Value = item.Attributes[idx2].Value
	}
	for idx2 := range item.Attributes2 {
		item_convert.Attributes[idx2].Name = item.Attributes2[idx2].Name
		item_convert.Attributes[idx2].Value = item.Attributes2[idx2].Value
	}
	addentrySearchResponseXml(tillid, apiBaseURL, &item_convert, entries)
	return
}
func (c *Client) processurl(url string, tillid string, maxage int, outputasjson bool, entries *NZBArr) (*NZBArr, bool, error) {

	breakatid := false

	if entries == nil {
		entries = &NZBArr{}
	}
	if outputasjson {
		var result SearchResponseJson1
		_, err := c.Client.DoJson(url, &result, nil)
		if err == nil {
			var breakid, skip bool
			if tillid == "" && len(entries.Arr) == 0 {
				entries.Arr = make([]Nzbwithprio, 0, len(result.Channel.Item))
			}
			if len(entries.Arr) >= 1 {
				entries.Arr = logger.GrowSliceBy(entries.Arr, len(result.Channel.Item))
			}

			for idx := range result.Channel.Item {
				breakid, skip = addentrySearchResponseJson1(tillid, c.ApiBaseURL, &result.Channel.Item[idx], entries)
				if skip {
					continue
				}
				if breakid {
					result.Close()
					return entries, breakatid, nil
				}
			}
			result.Close()
			return entries, breakatid, nil
		} else {
			var result SearchResponseJson2
			_, err = c.Client.DoJson(url, &result, nil)
			if err == nil {
				var breakid, skip bool
				if tillid == "" && len(entries.Arr) == 0 {
					entries.Arr = make([]Nzbwithprio, 0, len(result.Item))
				}
				if len(entries.Arr) >= 1 {
					entries.Arr = logger.GrowSliceBy(entries.Arr, len(result.Item))
				}

				for idx := range result.Item {
					breakid, skip = addentrySearchResponseJson2(tillid, c.ApiBaseURL, &result.Item[idx], entries)
					if skip {
						continue
					}
					if breakid {
						result.Close()
						return entries, breakatid, nil
					}
				}
				result.Close()
				return entries, breakatid, nil
			} else {
				logger.Log.GlobalLogger.Error("nzb process error", zap.Error(err))
				return nil, false, err
			}
		}
	} else {
		var feed SearchResponse
		_, err := c.Client.DoXml(url, &feed, nil)
		if err == nil {
			var breakid, skip bool
			if tillid == "" && len(entries.Arr) == 0 {
				entries.Arr = make([]Nzbwithprio, 0, len(feed.NZBs))
			}
			if len(entries.Arr) >= 1 {
				entries.Arr = logger.GrowSliceBy(entries.Arr, len(feed.NZBs))
			}

			for idx := range feed.NZBs {
				breakid, skip = addentrySearchResponseXml(tillid, c.ApiBaseURL, &feed.NZBs[idx], entries)
				if skip {
					continue
				}
				if breakid {
					feed.Close()
					return entries, breakatid, nil
				}
			}
			feed.Close()
			return entries, breakatid, nil
		} else {
			logger.Log.GlobalLogger.Error("nzb process error", zap.Error(err))
			return nil, false, err
		}
	}
}

func saveAttributes(newEntry *NZB, name string, value string) {
	switch name {

	case "guid":
		newEntry.ID = value
	// case "genre":
	// 	newEntry.Genre = value
	case "tvdbid":
		newEntry.TVDBID = value
	// case "info":
	// 	newEntry.Info = value
	case "season":
		newEntry.Season = value
	case "episode":
		newEntry.Episode = value
	// case "tvtitle":
	// 	newEntry.TVTitle = value
	case "imdb":
		newEntry.IMDBID = value
	// case "imdbtitle":
	// 	newEntry.IMDBTitle = value
	// case "coverurl":
	// 	newEntry.CoverURL = value
	// case "resolution":
	// 	newEntry.Resolution = value
	// case "poster":
	// 	newEntry.Poster = value
	// case "group":
	// 	newEntry.Group = value
	// case "infohash":
	// 	newEntry.InfoHash = value
	// 	newEntry.IsTorrent = true
	// case "category":
	// 	newEntry.Category = append(newEntry.Category, value)
	// case "tvairdate":
	// 	newEntry.AirDate, _ = parseDate(value)
	// case "usenetdate":
	// 	newEntry.UsenetDate, _ = parseDate(value)
	case "size":
		intValue, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			newEntry.Size = intValue
		}
		// case "grabs":
		// 	intValue, _ := strconv.ParseInt(value, 10, 64)
		// 	newEntry.NumGrabs = int(intValue)
		// case "seeders":
		// 	intValue, _ := strconv.ParseInt(value, 10, 64)
		// 	newEntry.Seeders = int(intValue)
		// 	newEntry.IsTorrent = true
		// case "peers":
		// 	intValue, _ := strconv.ParseInt(value, 10, 64)
		// 	newEntry.Peers = int(intValue)
		// 	newEntry.IsTorrent = true
		// case "rating":
		// 	intValue, _ := strconv.ParseInt(value, 10, 64)
		// 	newEntry.Rating = int(intValue)
		// case "imdbyear":
		// 	intValue, _ := strconv.ParseInt(value, 10, 64)
		// 	newEntry.IMDBYear = int(intValue)
		// case "imdbscore":
		// 	parsedFloat, _ := strconv.ParseFloat(value, 32)
		// 	newEntry.IMDBScore = float32(parsedFloat)
	}
}

const (
	apiPath = "/api"
	rssPath = "/rss"
)

// SearchResponse is a RSS version of the response.
type SearchResponse struct {
	NZBs []RawNZB `xml:"channel>item"`
}

func (s *SearchResponse) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		if len(s.NZBs) >= 1 {
			s.NZBs = nil
		}
		s = nil
	}
}

type SearchResponseJson1 struct {
	Title   string `json:"title,omitempty"`
	Channel struct {
		Item []RawNZBJson1 `json:"item"`
	} `json:"channel"`
}

func (s *SearchResponseJson1) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		if len(s.Channel.Item) >= 1 {
			s.Channel.Item = nil
		}
		logger.ClearVar(&s)
	}
}

type SearchResponseJson2 struct {
	Item []RawNZBJson2 `json:"item"`
}

func (s *SearchResponseJson2) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		if len(s.Item) >= 1 {
			s.Item = nil
		}
		logger.ClearVar(&s)
	}
}

// RawNZB represents a single NZB item in search results.
type RawNZB struct {
	Title string `xml:"title,omitempty"`
	//Link  string `xml:"link,omitempty"`
	Size int64 `xml:"size,omitempty"`

	GUID struct {
		GUID string `xml:",chardata"`
	} `xml:"guid,omitempty"`

	Source struct {
		URL string `xml:"url,attr"`
	} `xml:"source,omitempty"`

	//Date string `xml:"pubDate,omitempty"`

	Enclosure struct {
		URL string `xml:"url,attr"`
	} `xml:"enclosure,omitempty"`

	Attributes []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	} `xml:"attr"`
}

type RawNZBJson1 struct {
	Title string `json:"title,omitempty"`
	//Link      string `json:"link,omitempty"`
	Guid string `json:"guid,omitempty"`
	Size int64  `json:"size,omitempty"`
	//Date      string `json:"pubDate,omitempty"`
	Enclosure struct {
		Attributes struct {
			URL string `json:"url"`
		} `json:"@attributes,omitempty"`
	} `json:"enclosure,omitempty"`

	Attributes []struct {
		Attribute struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"@attributes,omitempty"`
	} `json:"attr,omitempty"`
}

type RawNZBJson2 struct {
	Title string `json:"title,omitempty"`
	//Link  string `json:"link,omitempty"`
	Size int64 `json:"size,omitempty"`
	GUID struct {
		GUID string `json:"text,omitempty"`
	} `json:"guid,omitempty"`
	//Date      string `json:"pubDate,omitempty"`
	Enclosure struct {
		URL string `json:"_url"`
	} `json:"enclosure,omitempty"`

	Attributes []struct {
		Name  string `json:"_name"`
		Value string `json:"_value"`
	} `json:"newznab:attr,omitempty"`
	Attributes2 []struct {
		Name  string `json:"_name"`
		Value string `json:"_value"`
	} `json:"nntmux:attr,omitempty"`
}
