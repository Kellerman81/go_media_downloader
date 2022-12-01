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
	Name                   string
	URL                    string
	Apikey                 string
	UserID                 int
	SkipSslCheck           bool
	Addquotesfortitlequery bool
	AdditionalQueryParams  string
	LastRssId              string
	Customapi              string
	Customurl              string
	Customrssurl           string
	Customrsscategory      string
	RssDownloadAll         bool
	OutputAsJson           bool
	Limitercalls           int
	Limiterseconds         int
	LimitercallsDaily      int
	MaxAge                 int
	TimeoutSeconds         int
}

func (n *NzbIndexer) Close() {
	if n != nil {
		n = nil
	}
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

		waituntil := (time.Duration(1) * time.Second)
		waituntilmax := (time.Duration(10) * time.Second)
		rand.Seed(time.Now().UnixNano())
		waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
		for i := 0; i < 10; i++ {
			ok, waitfor = client.Client.Ratelimiter.Check()
			if ok {
				return true, 0
			}
			if waitfor > waituntilmax {
				//logger.Log.GlobalLogger.Warn("Hit rate limit - Should wait for (dont retry)", zap.Duration("waitfor", waitfor), zap.String("Url", url))
				return false, waitfor
			}
			if waitfor == 0 {
				waitfor = waituntil
			} else {
				waitfor = waitfor + waitincrease
			}
			time.Sleep(waitfor)
		}
	}
	return true, 0
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
	outputAsJson           bool
	customurl              string
	customrsscategory      string
	customapi              string
	additionalQueryParams  string
	limit                  string
	num                    int
	categories             string
	offset                 int
}

func (client *Client) buildUrl(builder urlbuilder) string {
	var bld strings.Builder
	defer bld.Reset()
	path := apiPath
	if builder.rss {
		path = rssPath
	}
	if builder.customurl != "" {
		bld.WriteString(builder.customurl)
	} else if builder.customapi != "" {
		bld.WriteString(logger.StringBuild(client.ApiBaseURL, path, "?", builder.customapi, "=", client.Apikey))
	} else {
		if builder.rss {
			bld.WriteString(logger.StringBuild(client.ApiBaseURL, path, "?r=", client.Apikey, "&i=", strconv.FormatInt(int64(client.ApiUserID), 10)))
		} else {
			bld.WriteString(logger.StringBuild(client.ApiBaseURL, path, "?apikey=", client.Apikey))
		}
	}
	if builder.useseason {
		bld.WriteString("&season=" + strconv.FormatInt(int64(builder.season), 10))
	}
	if builder.useepisode {
		bld.WriteString("&ep=" + strconv.FormatInt(int64(builder.episode), 10))
	}
	if builder.limit != "0" && builder.limit != "" {
		bld.WriteString("&limit=" + builder.limit)
	}
	if builder.imdbid != "" {
		bld.WriteString("&imdbid=" + builder.imdbid)
	}
	if builder.tvdbid != 0 {
		bld.WriteString("&tvdbid=" + strconv.Itoa(builder.tvdbid))
	}
	if builder.categories != "" {
		if builder.rss {
			if builder.customrsscategory != "" {
				bld.WriteString("&" + builder.customrsscategory + "=" + builder.categories)
			} else {
				bld.WriteString("&t=" + builder.categories)
			}
		} else {
			bld.WriteString("&cat=" + builder.categories)
		}
	}

	if builder.offset != 0 {
		bld.WriteString("&offset=" + strconv.FormatInt(int64(builder.offset), 10))
	}

	if builder.num != 0 {
		bld.WriteString("&num=" + strconv.Itoa(builder.num))
	}

	if builder.searchtype != "" {
		bld.WriteString("&t=" + builder.searchtype)
	}

	if builder.query != "" {
		if builder.addquotesfortitlequery {
			bld.WriteString("&q=%22" + url.PathEscape(builder.query) + "%22")
		} else {
			bld.WriteString("&q=" + url.PathEscape(builder.query))
		}
	}
	if builder.outputAsJson {
		bld.WriteString("&o=json")
	}
	bld.WriteString("&dl=1" + builder.additionalQueryParams)
	return bld.String()
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(row *NzbIndexer, imdbid string, categories string) (*NZBArr, bool, error) {
	if imdbid == "" {
		return nil, false, errors.New("no imdbid")
	}

	client := getnewznabclient(row)
	return client.processurl(client.buildUrl(urlbuilder{searchtype: "movie", imdbid: imdbid, outputAsJson: row.OutputAsJson, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}), "", row.MaxAge, row.OutputAsJson, nil)
}

func getnewznabclient(row *NzbIndexer) *Client {
	if checkclient(row.URL) {
		return getclient(row.URL)
	} else {
		client := NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, limiterconfig{row.Limitercalls, row.Limiterseconds, row.LimitercallsDaily, row.TimeoutSeconds})
		newznabClients = append(newznabClients, Clients{Name: row.URL, Client: client})
		return client
	}
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(row *NzbIndexer, tvdbid int, categories string, season int, episode int, useseason bool, useepisode bool) (*NZBArr, bool, error) {
	if tvdbid == 0 {
		return nil, false, errors.New("no tvdbid")
	}
	client := getnewznabclient(row)

	limitstr := ""
	if !useepisode || !useseason {
		limitstr = "100"
	}

	return client.processurl(client.buildUrl(urlbuilder{searchtype: "tvsearch", tvdbid: tvdbid, useseason: useseason, season: season, useepisode: useepisode, episode: episode, outputAsJson: row.OutputAsJson, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: limitstr, categories: categories}), "", row.MaxAge, row.OutputAsJson, nil)
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(row *NzbIndexer, query string, categories string, searchtype string) (*NZBArr, bool, error) {
	client := getnewznabclient(row)

	return client.processurl(client.buildUrl(urlbuilder{searchtype: searchtype, query: query, addquotesfortitlequery: row.Addquotesfortitlequery, outputAsJson: row.OutputAsJson, customurl: row.Customurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}), "", row.MaxAge, row.OutputAsJson, nil)
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
func QueryNewznabRSS(row *NzbIndexer, maxitems int, categories string) (*NZBArr, bool, error) {

	client := getnewznabclient(row)
	return client.processurl(client.buildRssUrl(row.Customrssurl, row.Customrsscategory, row.Customapi, row.AdditionalQueryParams, maxitems, categories, 0, false), "", 0, false, nil)
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(row *NzbIndexer, maxitems int, categories string, maxrequests int) (*NZBArr, string, string, error) {

	client := getnewznabclient(row)
	count := 0
	baseurl := client.buildRssUrl(row.Customrssurl, row.Customrsscategory, row.Customapi, row.AdditionalQueryParams, maxitems, categories, 0, false)

	resultsadd, broke, erradd := client.processurl(baseurl, row.LastRssId, 0, false, nil)
	if erradd != nil {
		return nil, "", "", erradd
	}
	if broke || len((resultsadd.Arr)) == 0 {
		return resultsadd, "", "", erradd
	}
	count++
	stroffset := baseurl + "&offset="
	for {
		_, broke, erradd = client.processurl(stroffset+strconv.Itoa(maxitems*count), row.LastRssId, 0, false, resultsadd)
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

type limiterconfig struct {
	limitercalls      int
	limiterseconds    int
	limitercallsdaily int
	timeoutseconds    int
}

// New returns a new instance of Client
func NewNewznab(baseURL string, apikey string, userID int, insecure bool, debug bool, lmtconfig limiterconfig) *Client {
	if lmtconfig.limitercalls == 0 {
		lmtconfig.limitercalls = 3
	}
	if lmtconfig.limiterseconds == 0 {
		lmtconfig.limiterseconds = 10
	}
	return &Client{
		Apikey:     apikey,
		ApiBaseURL: baseURL,
		ApiUserID:  userID,
		Debug:      debug,
		Client: NewClient(
			insecure,
			rate.New(lmtconfig.limitercalls, lmtconfig.limitercallsdaily, time.Duration(lmtconfig.limiterseconds)*time.Second), lmtconfig.timeoutseconds,
		),
	}
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (c *Client) SearchWithQueryUntilNZBID(categories string, query string, searchType string, addquotes bool, id string, additionalQueryParams string, customurl string, maxage int, outputasjson bool) (resultsadd *NZBArr, broke bool, erradd error) {
	return c.processurl(c.buildUrl(urlbuilder{searchtype: searchType, query: query, addquotesfortitlequery: addquotes, outputAsJson: outputasjson, customurl: customurl, additionalQueryParams: additionalQueryParams, limit: "0", categories: categories}), id, maxage, outputasjson, nil)
}

func (c *Client) buildRssUrl(customrssurl string, customrsscategory string, customapi string, additionalQueryParams string, num int, categories string, offset int, outputasjson bool) string {
	return c.buildUrl(urlbuilder{rss: true, outputAsJson: outputasjson, customurl: customrssurl, customrsscategory: customrsscategory, customapi: customapi, additionalQueryParams: additionalQueryParams, limit: "0", num: num, categories: categories, offset: offset}) //buildurl.String()
}

func addentrySearchResponseXml(tillid string, apiBaseURL string, item *RawNZB, entries *NZBArr) (bool, bool) {
	if item.Enclosure.URL == "" {
		return false, true
	}
	var newEntry Nzbwithprio
	newEntry.NZB.Title = logger.HtmlUnescape(item.Title)
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
	entries.Arr = append(entries.Arr, newEntry)
	if tillid == newEntry.NZB.ID && tillid != "" {
		return true, false
	}
	return false, false
}
func addentrySearchResponseJson1(tillid string, apiBaseURL string, item *RawNZBJson1, entries *NZBArr) {
	itemconvert := RawNZB{}
	itemconvert.Enclosure.URL = item.Enclosure.Attributes.URL
	itemconvert.Title = item.Title
	itemconvert.Size = item.Size
	itemconvert.GUID.GUID = item.Guid
	itemconvert.Attributes = make([]struct {
		Name  string "xml:\"name,attr\""
		Value string "xml:\"value,attr\""
	}, len(item.Attributes))
	for idx2 := range item.Attributes {
		itemconvert.Attributes[idx2].Name = item.Attributes[idx2].Attribute.Name
		itemconvert.Attributes[idx2].Value = item.Attributes[idx2].Attribute.Value
	}
	addentrySearchResponseXml(tillid, apiBaseURL, &itemconvert, entries)
}
func addentrySearchResponseJson2(tillid string, apiBaseURL string, item *RawNZBJson2, entries *NZBArr) {
	itemconvert := RawNZB{}
	itemconvert.Enclosure.URL = item.Enclosure.URL
	itemconvert.Title = item.Title
	itemconvert.Size = item.Size
	itemconvert.GUID.GUID = item.GUID.GUID
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
	addentrySearchResponseXml(tillid, apiBaseURL, &itemconvert, entries)
}
func (c *Client) processurl(url string, tillid string, maxage int, outputasjson bool, entries *NZBArr) (*NZBArr, bool, error) {
	breakatid := false

	if entries == nil {
		entries = new(NZBArr)
	}
	if outputasjson {
		result := new(SearchResponseJson1)
		_, err := c.Client.DoJson(url, result, nil)
		defer result.Close()
		if err == nil {
			entries.Arr = logger.GrowSliceBy(entries.Arr, len(result.Channel.Item))

			for idx := range result.Channel.Item {
				addentrySearchResponseJson1(tillid, c.ApiBaseURL, &result.Channel.Item[idx], entries)
			}
			return entries, breakatid, nil
		} else {
			result := new(SearchResponseJson2)
			_, err = c.Client.DoJson(url, result, nil)
			defer result.Close()
			if err == nil {
				entries.Arr = logger.GrowSliceBy(entries.Arr, len(result.Item))

				for idx := range result.Item {
					addentrySearchResponseJson2(tillid, c.ApiBaseURL, &result.Item[idx], entries)
				}
				return entries, breakatid, nil
			} else {
				logger.Log.GlobalLogger.Error("nzb process error", zap.Error(err))
				return nil, false, err
			}
		}
	} else {
		feed := new(SearchResponse)
		_, err := c.Client.DoXml(url, feed, nil)
		defer feed.Close()
		if err == nil {
			var breakid, skip bool
			//if tillid == "" && len(entries.Arr) == 0 {
			//	entries.Arr = make([]Nzbwithprio, 0, len(feed.NZBs))
			//}
			//if len(entries.Arr) >= 1 {
			entries.Arr = logger.GrowSliceBy(entries.Arr, len(feed.NZBs))
			//}

			for idx := range feed.NZBs {
				breakid, skip = addentrySearchResponseXml(tillid, c.ApiBaseURL, &feed.NZBs[idx], entries)
				if skip {
					continue
				}
				if breakid {

					return entries, breakatid, nil
				}
			}
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
