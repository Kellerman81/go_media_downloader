package apiexternal

import (
	"errors"
	"fmt"
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
	Name                   string
	URL                    string
	Apikey                 string
	UserID                 string
	SkipSslCheck           bool
	DisableCompression     bool
	Addquotesfortitlequery bool
	AdditionalQueryParams  string
	LastRssID              string
	Customapi              string
	Customurl              string
	Customrssurl           string
	Customrsscategory      string
	OutputAsJSON           bool
	Limitercalls           int
	Limiterseconds         int
	LimitercallsDaily      int
	MaxAge                 int
	TimeoutSeconds         int
}
type urlbuilder struct {
	rss                    bool
	searchtype             string
	query                  string
	addquotesfortitlequery bool
	imdbid                 string
	tvdbid                 int
	useseason              bool
	season                 int
	useepisode             bool
	episode                int
	outputAsJSON           bool
	customurl              string
	customrsscategory      string
	customapi              string
	additionalQueryParams  string
	limit                  string
	num                    int
	categories             string
	offset                 int
}

type Clients struct {
	Name   string
	Client *Client
}

// Client is a type for interacting with a newznab or torznab api
type Client struct {
	Apikey     string
	APIBaseURL string
	APIUserID  string
	Debug      bool
	Client     *RLHTTPClient
}

type limiterconfig struct {
	limitercalls      int
	limiterseconds    int
	limitercallsdaily int
	timeoutseconds    int
}

// SearchResponse is a RSS version of the response.
type searchResponse struct {
	Nzbs []rawNZB `xml:"channel>item"`
}

type searchResponseJSON1 struct {
	Title   string `json:"title,omitempty"`
	Channel struct {
		Item []rawNZBJson1 `json:"item"`
	} `json:"channel"`
}
type searchResponseJSON2 struct {
	Item []rawNZBJson2 `json:"item"`
}

// RawNZB represents a single NZB item in search results.
type rawNZB struct {
	Title string `xml:"title,omitempty"`
	//Link  string `xml:"link,omitempty"`
	Size int64 `xml:"size,omitempty"`

	Guid struct {
		Guid string `xml:",chardata"`
	} `xml:"guid,omitempty"`

	Source struct {
		Url string `xml:"url,attr"`
	} `xml:"source,omitempty"`

	//Date string `xml:"pubDate,omitempty"`

	Enclosure struct {
		Url string `xml:"url,attr"`
	} `xml:"enclosure,omitempty"`

	Attributes []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	} `xml:"attr"`
}

type rawNZBJson1 struct {
	Title string `json:"title,omitempty"`
	//Link      string `json:"link,omitempty"`
	Guid string `json:"guid,omitempty"`
	Size int64  `json:"size,omitempty"`
	//Date      string `json:"pubDate,omitempty"`
	Enclosure struct {
		Attributes struct {
			Url string `json:"url"`
		} `json:"@attributes,omitempty"`
	} `json:"enclosure,omitempty"`

	Attributes []struct {
		Attribute struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"@attributes,omitempty"`
	} `json:"attr,omitempty"`
}

type rawNZBJson2 struct {
	Title string `json:"title,omitempty"`
	//Link  string `json:"link,omitempty"`
	Size int64 `json:"size,omitempty"`
	Guid struct {
		Guid string `json:"text,omitempty"`
	} `json:"guid,omitempty"`
	//Date      string `json:"pubDate,omitempty"`
	Enclosure struct {
		Url string `json:"_url"`
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

const (
	apiPath = "/api"
	rssPath = "/rss"
)

var newznabClients []Clients
var Errnoresults = errors.New("no results")

func NewznabCheckLimiter(url string) (bool, time.Duration) {
	if checkclient(url) {
		client := getclient(url)

		var ok bool
		var waitfor time.Duration
		if client.Client.DailyLimiterEnabled {
			_, ok, waitfor = client.Client.Ratelimiter.Check(false, true)
			if !ok {
				logger.Log.GlobalLogger.Debug("Daily not ok ", zap.Stringp("url", &url))
				return ok, waitfor
			}
		}

		waituntil := (time.Duration(1) * time.Second)
		waituntilmax := (time.Duration(10) * time.Second)
		rand.Seed(time.Now().UnixNano())
		waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
		for i := 0; i < 10; i++ {
			ok, _, waitfor = client.Client.Ratelimiter.Check(true, false)
			if ok {
				return true, 0
			}

			if waitfor > waituntilmax {
				logger.Log.GlobalLogger.Warn("Hit rate limit - Should wait for (dont retry)", zap.Durationp("waitfor", &waitfor), zap.Stringp("Url", &url))
				return false, waitfor
			}
			if waitfor == 0 {
				waitfor = waituntil
			} else {
				waitfor += waitincrease
			}
			time.Sleep(waitfor)
		}
		logger.Log.GlobalLogger.Debug("Loops exceeded - still waiting", zap.Stringp("url", &url), zap.Durationp("waitfor", &waitfor))
		return false, waitfor
	}
	return true, 0
}

func (client *Client) buildURL(builder *urlbuilder, maxage int) string {
	var bld strings.Builder
	bld.Grow(200)

	if builder.customurl != "" {
		bld.WriteString(builder.customurl)
	} else if builder.customapi != "" {
		path := apiPath
		if builder.rss {
			path = rssPath
		}
		bld.WriteString(fmt.Sprintf("%s%s?%s=%s", client.APIBaseURL, path, builder.customapi, client.Apikey))
	} else {
		if builder.rss {
			bld.WriteString(fmt.Sprintf("%s%s?r=%s&i=%s", client.APIBaseURL, rssPath, client.Apikey, client.APIUserID))
		} else {
			bld.WriteString(fmt.Sprintf("%s%s?apikey=%s", client.APIBaseURL, apiPath, client.Apikey))
		}
	}
	if builder.useseason {
		bld.WriteString(fmt.Sprintf("&season=%d", builder.season))
	}
	if builder.useepisode {
		bld.WriteString(fmt.Sprintf("&ep=%d", builder.episode))
	}
	if builder.limit != "0" && builder.limit != "" {
		bld.WriteString(fmt.Sprintf("&limit=%s", builder.limit))
	}
	if builder.imdbid != "" {
		bld.WriteString(fmt.Sprintf("&imdbid=%s", builder.imdbid))
	}
	if builder.tvdbid != 0 {
		bld.WriteString(fmt.Sprintf("&tvdbid=%d", builder.tvdbid))
	}
	if builder.categories != "" {
		if builder.rss {
			if builder.customrsscategory != "" {
				bld.WriteString(fmt.Sprintf("&%s=%s", builder.customrsscategory, builder.categories))
			} else {
				bld.WriteString(fmt.Sprintf("&t=%s", builder.categories))
			}
		} else {
			bld.WriteString(fmt.Sprintf("&cat=%s", builder.categories))
		}
	}

	if builder.offset != 0 {
		bld.WriteString(fmt.Sprintf("&offset=%d", builder.offset))
	}

	if builder.num != 0 {
		bld.WriteString(fmt.Sprintf("&num=%d", builder.num))
	}

	if builder.searchtype != "" {
		bld.WriteString(fmt.Sprintf("&t=%s", builder.searchtype))
	}

	if builder.query != "" {
		if builder.addquotesfortitlequery {
			quotes := "%22"
			bld.WriteString(fmt.Sprintf("&q=%s%s%s", quotes, url.QueryEscape(builder.query), quotes))
		} else {
			bld.WriteString(fmt.Sprintf("&q=%s", url.QueryEscape(builder.query)))
		}
	}
	if builder.outputAsJSON {
		bld.WriteString("&o=json")
	}
	if maxage != 0 {
		bld.WriteString("&maxage=" + strconv.Itoa(maxage))
	}
	bld.WriteString(fmt.Sprintf("&dl=1%s", builder.additionalQueryParams))
	return bld.String()
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(row *NzbIndexer, imdbid string, categories string) (*NZBArr, bool, error) {
	if imdbid == "" {
		return nil, false, errors.New("no imdbid")
	}

	return getnewznabclient(row).processurl(&urlbuilder{searchtype: "movie", imdbid: imdbid, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}, "", row.MaxAge, row.OutputAsJSON)
}

func getnewznabclient(row *NzbIndexer) *Client {
	client := getclient(row.URL)
	if client != nil {
		return client
	}
	client = NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, row.DisableCompression, true, limiterconfig{row.Limitercalls, row.Limiterseconds, row.LimitercallsDaily, row.TimeoutSeconds})
	newznabClients = append(newznabClients, Clients{Name: row.URL, Client: client})
	return client
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(row *NzbIndexer, tvdbid int, categories string, season int, episode int, useseason bool, useepisode bool) (*NZBArr, bool, error) {
	if tvdbid == 0 {
		return nil, false, errors.New("no tvdbid")
	}
	var limitstr string
	if !useepisode || !useseason {
		limitstr = "100"
	}

	return getnewznabclient(row).processurl(&urlbuilder{searchtype: "tvsearch", tvdbid: tvdbid, useseason: useseason, season: season, useepisode: useepisode, episode: episode, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: limitstr, categories: categories}, "", row.MaxAge, row.OutputAsJSON)
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(row *NzbIndexer, query string, categories string, searchtype string) (*NZBArr, bool, error) {
	return getnewznabclient(row).processurl(&urlbuilder{searchtype: searchtype, query: query, addquotesfortitlequery: row.Addquotesfortitlequery, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}, "", row.MaxAge, row.OutputAsJSON)
}

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
func QueryNewznabRSS(row *NzbIndexer, maxitems int, categories string) (*NZBArr, bool, error) {
	return getnewznabclient(row).processurl(&urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories}, "", 0, false)
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(row *NzbIndexer, maxitems int, categories string, maxrequests int) (*NZBArr, string, string, error) {

	client := getnewznabclient(row)
	count := 0
	baseurl := urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories}

	results, broke, erradd := client.processurl(&baseurl, row.LastRssID, 0, false)
	if erradd != nil {
		return nil, "", "", erradd
	}
	if broke || len((results.Arr)) == 0 {
		return results, "", "", erradd
	}
	count++
	urlb := urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories}
	for {
		urlb.offset = (maxitems * count)
		_, broke, erradd = client.processurl(&urlb, row.LastRssID, 0, false, results)
		if erradd != nil {
			break
		}
		count++
		if maxrequests == 0 || count >= maxrequests || broke {
			break
		}
	}

	if erradd != nil {
		return results, row.URL, "", erradd
	}
	if len(results.Arr) >= 1 {
		return results, "", results.Arr[0].NZB.ID, nil
	}
	return results, "", "", nil
}

// New returns a new instance of Client
func NewNewznab(baseURL string, apikey string, userID string, insecure bool, disablecompression bool, debug bool, lmtconfig limiterconfig) *Client {
	if lmtconfig.limitercalls == 0 {
		lmtconfig.limitercalls = 3
	}
	if lmtconfig.limiterseconds == 0 {
		lmtconfig.limiterseconds = 10
	}
	return &Client{
		Apikey:     apikey,
		APIBaseURL: baseURL,
		APIUserID:  userID,
		Debug:      debug,
		Client: NewClient(
			insecure,
			disablecompression,
			rate.New(lmtconfig.limitercalls, lmtconfig.limitercallsdaily, time.Duration(lmtconfig.limiterseconds)*time.Second), lmtconfig.timeoutseconds,
		),
	}
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (client *Client) SearchWithQueryUntilNZBID(categories string, query string, searchType string, addquotes bool, id string, additionalQueryParams string, customurl string, maxage int, outputasjson bool) (resultsadd *NZBArr, broke bool, erradd error) {
	return client.processurl(&urlbuilder{searchtype: searchType, query: query, addquotesfortitlequery: addquotes, outputAsJSON: outputasjson, customurl: customurl, additionalQueryParams: additionalQueryParams, limit: "0", categories: categories}, id, maxage, outputasjson)
}

func addentrySearchResponseXML(tillid string, apiBaseURL string, item *rawNZB, entries *NZBArr) (bool, bool) {
	if item.Enclosure.Url == "" {
		return false, true
	}
	var newEntry Nzbwithprio
	newEntry.NZB.Title = item.Title

	if strings.Contains(newEntry.NZB.Title, "&") || strings.Contains(newEntry.NZB.Title, "%") {
		newEntry.NZB.Title = html.UnescapeString(newEntry.NZB.Title)
	}
	if strings.Contains(newEntry.NZB.Title, "\\u") {
		unquote, err := strconv.Unquote("\"" + newEntry.NZB.Title + "\"")
		if err == nil {
			newEntry.NZB.Title = unquote
		}
	}
	newEntry.NZB.DownloadURL = item.Enclosure.Url
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
	newEntry.NZB.ID = item.Guid.Guid
	if newEntry.NZB.ID == "" {
		newEntry.NZB.ID = item.Enclosure.Url
	}
	entries.Arr = append(entries.Arr, newEntry)
	if tillid != "" && tillid == newEntry.NZB.ID {
		return true, false
	}
	return false, false
}
func addentrySearchResponseJSON1(tillid string, apiBaseURL string, item *rawNZBJson1, entries *NZBArr) {
	itemconvert := rawNZB{}
	itemconvert.Enclosure.Url = item.Enclosure.Attributes.Url
	itemconvert.Title = item.Title
	itemconvert.Size = item.Size
	itemconvert.Guid.Guid = item.Guid
	itemconvert.Attributes = make([]struct {
		Name  string "xml:\"name,attr\""
		Value string "xml:\"value,attr\""
	}, len(item.Attributes))
	for idx2 := range item.Attributes {
		itemconvert.Attributes[idx2].Name = item.Attributes[idx2].Attribute.Name
		itemconvert.Attributes[idx2].Value = item.Attributes[idx2].Attribute.Value
	}
	addentrySearchResponseXML(tillid, apiBaseURL, &itemconvert, entries)
}
func addentrySearchResponseJSON2(tillid string, apiBaseURL string, item *rawNZBJson2, entries *NZBArr) {
	itemconvert := rawNZB{}
	itemconvert.Enclosure.Url = item.Enclosure.Url
	itemconvert.Title = item.Title
	itemconvert.Size = item.Size
	itemconvert.Guid.Guid = item.Guid.Guid
	itemconvert.Attributes = make([]struct {
		Name  string "xml:\"name,attr\""
		Value string "xml:\"value,attr\""
	}, len(item.Attributes)+len(item.Attributes2))
	for idx2 := range item.Attributes {
		itemconvert.Attributes[idx2].Name = item.Attributes[idx2].Name
		itemconvert.Attributes[idx2].Value = item.Attributes[idx2].Value
	}
	for idx2 := range item.Attributes2 {
		itemconvert.Attributes[idx2].Name = item.Attributes2[idx2].Name
		itemconvert.Attributes[idx2].Value = item.Attributes2[idx2].Value
	}
	addentrySearchResponseXML(tillid, apiBaseURL, &itemconvert, entries)
}

func (client *Client) processurl(urlb *urlbuilder, tillid string, maxage int, outputasjson bool, invar ...*NZBArr) (*NZBArr, bool, error) {
	url := client.buildURL(urlb, maxage)
	urlb = nil
	var breakatid bool
	//logger.Log.GlobalLogger.Debug("call url", zap.String("url", url))
	if outputasjson {
		var result searchResponseJSON1
		_, err := client.Client.DoJSON(url, &result, nil)
		if err == nil {
			if len(result.Channel.Item) == 0 {
				return nil, false, Errnoresults
			}
			var entries *NZBArr
			if len(invar) >= 1 {
				entries = invar[0]
			} else {
				entries = &NZBArr{Arr: make([]Nzbwithprio, 0, len(result.Channel.Item))}
			}
			for idx := range result.Channel.Item {
				addentrySearchResponseJSON1(tillid, client.APIBaseURL, &result.Channel.Item[idx], entries)
			}
			result.close()
			return entries, breakatid, nil
		}
		var result2 searchResponseJSON2
		_, err = client.Client.DoJSON(url, &result2, nil)
		if err != nil {
			return nil, false, err
		}
		if len(result2.Item) == 0 {
			return nil, false, Errnoresults
		}
		var entries *NZBArr
		if len(invar) >= 1 {
			entries = invar[0]
		} else {
			entries = &NZBArr{Arr: make([]Nzbwithprio, 0, len(result2.Item))}
		}
		for idx := range result2.Item {
			addentrySearchResponseJSON2(tillid, client.APIBaseURL, &result2.Item[idx], entries)
		}
		result2.close()
		return entries, breakatid, nil

	}
	var feed searchResponse
	err := client.Client.DoXML(url, nil, &feed)
	if err != nil {
		//logger.Log.GlobalLogger.Debug("call url error", zap.Error(err))
		return nil, false, err
	}
	if len(feed.Nzbs) == 0 {
		return nil, false, Errnoresults
	}
	var breakid, skip bool
	//if tillid == "" && len(entries.Arr) == 0 {
	//	entries.Arr = make([]Nzbwithprio, 0, len(feed.NZBs))
	//}
	//if len(entries.Arr) >= 1 {
	//}
	var entries *NZBArr
	if len(invar) >= 1 {
		entries = invar[0]
	} else {
		entries = &NZBArr{Arr: make([]Nzbwithprio, 0, len(feed.Nzbs))}
	}
	for idx := range feed.Nzbs {
		breakid, skip = addentrySearchResponseXML(tillid, client.APIBaseURL, &feed.Nzbs[idx], entries)
		if skip {
			continue
		}
		if breakid {

			feed.close()
			return entries, breakatid, nil
		}
	}
	feed.close()
	return entries, breakatid, nil
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

func (s *searchResponse) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	if len(s.Nzbs) >= 1 {
		s.Nzbs = nil
	}
	s = nil
}

func (s *searchResponseJSON1) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	if len(s.Channel.Item) >= 1 {
		s.Channel.Item = nil
	}
	s = nil
}

func (s *searchResponseJSON2) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	if len(s.Item) >= 1 {
		s.Item = nil
	}
	s = nil
}
