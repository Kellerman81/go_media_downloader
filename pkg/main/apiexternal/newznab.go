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
	"golang.org/x/exp/slices"
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

type searchResponseJSON1 struct {
	Title   string `json:"title,omitempty"`
	Channel struct {
		Item []rawNZBJson1 `json:"item"`
	} `json:"channel"`
}
type searchResponseJSON2 struct {
	Item []rawNZBJson2 `json:"item"`
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

var newznabClients []*Clients
var Errnoresults = errors.New("no results")

func NewznabCheckLimiter(url string) (bool, time.Duration) {
	intid := slices.IndexFunc(newznabClients, func(e *Clients) bool { return e.Name == url })
	if intid == -1 {

		return true, 0
	}
	var ok bool
	var waitfor time.Duration
	if newznabClients[intid].Client.Client.DailyLimiterEnabled {
		_, ok, waitfor = newznabClients[intid].Client.Client.Ratelimiter.Check(false, true)
		if !ok {
			logger.Log.GlobalLogger.Debug("Daily not ok ", zap.Stringp("url", &url))
			return ok, waitfor
		}
	}

	waituntil := (time.Duration(1) * time.Second)
	waituntilmax := (time.Duration(10) * time.Second)
	rand.New(rand.NewSource(time.Now().UnixNano()))
	waitincrease := (time.Duration(rand.Intn(500)+10) * time.Millisecond)
	for i := 0; i < 10; i++ {
		ok, _, waitfor = newznabClients[intid].Client.Client.Ratelimiter.Check(true, false)
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

func (client *Client) buildURL(builder *urlbuilder, maxage int) string {
	var bld strings.Builder
	bld.Grow(200)

	path := apiPath
	if builder.rss {
		path = rssPath
	}
	if builder.customurl != "" {
		bld.WriteString(builder.customurl)
	} else if builder.customapi != "" {
		bld.WriteString(client.APIBaseURL)
		bld.WriteString(path)
		bld.WriteString("?")
		bld.WriteString(builder.customapi)
		bld.WriteString("=")
		bld.WriteString(client.Apikey)
	} else {
		bld.WriteString(client.APIBaseURL)
		bld.WriteString(path)
		if builder.rss {
			bld.WriteString("?r=")
			bld.WriteString(client.Apikey)
			bld.WriteString("&i=")
			bld.WriteString(client.APIUserID)
		} else {
			bld.WriteString("?apikey=")
			bld.WriteString(client.Apikey)
		}
	}
	if builder.useseason {
		bld.WriteString("&season=")
		bld.WriteString(logger.IntToString(builder.season))
	}
	if builder.useepisode {
		bld.WriteString("&ep=")
		bld.WriteString(logger.IntToString(builder.episode))
	}
	if builder.limit != "0" && builder.limit != "" {
		bld.WriteString("&limit=")
		bld.WriteString(builder.limit)
	}
	if builder.imdbid != "" {
		bld.WriteString("&imdbid=")
		bld.WriteString(builder.imdbid)
	}
	if builder.tvdbid != 0 {
		bld.WriteString("&tvdbid=")
		bld.WriteString(logger.IntToString(builder.tvdbid))
	}
	if builder.categories != "" {
		if builder.rss {
			if builder.customrsscategory != "" {
				bld.WriteString("&")
				bld.WriteString(builder.customrsscategory)
				bld.WriteString("=")
				bld.WriteString(builder.categories)
			} else {
				bld.WriteString("&t=")
				bld.WriteString(builder.categories)
			}
		} else {
			bld.WriteString("&cat=")
			bld.WriteString(builder.categories)
		}
	}

	if builder.offset != 0 {
		bld.WriteString("&offset=")
		bld.WriteString(logger.IntToString(builder.offset))
	}

	if builder.num != 0 {
		bld.WriteString("&num=")
		bld.WriteString(logger.IntToString(builder.num))
	}

	if builder.searchtype != "" {
		bld.WriteString("&t=")
		bld.WriteString(builder.searchtype)
	}

	if builder.query != "" {
		if builder.addquotesfortitlequery {
			quotes := "%22"
			bld.WriteString("&q=")
			bld.WriteString(quotes)
			bld.WriteString(url.QueryEscape(builder.query))
			bld.WriteString(quotes)
		} else {
			bld.WriteString("&q=")
			bld.WriteString(url.QueryEscape(builder.query))
		}
	}
	if builder.outputAsJSON {
		bld.WriteString("&o=json")
	}
	if maxage != 0 {
		bld.WriteString("&maxage=")
		bld.WriteString(logger.IntToString(maxage))
	}
	bld.WriteString("&dl=1")
	bld.WriteString(builder.additionalQueryParams)
	defer bld.Reset()
	return bld.String()
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(row *NzbIndexer, imdbid string, categories string) (*NZBArr, bool, error) {
	if imdbid == "" {
		return nil, false, errors.New("no imdbid")
	}
	entries := new(NZBArr)
	broke, err := getnewznabclient(row).processurl(&urlbuilder{searchtype: "movie", imdbid: imdbid, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}, "", row.MaxAge, row.OutputAsJSON, entries)
	return entries, broke, err
}

func getnewznabclient(row *NzbIndexer) *Client {
	intid := slices.IndexFunc(newznabClients, func(e *Clients) bool { return e.Name == row.URL })
	if intid != -1 {
		return newznabClients[intid].Client
	}
	client := NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, row.DisableCompression, true, limiterconfig{row.Limitercalls, row.Limiterseconds, row.LimitercallsDaily, row.TimeoutSeconds})
	newznabClients = append(newznabClients, &Clients{Name: row.URL, Client: client})
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
	entries := new(NZBArr)
	broke, err := getnewznabclient(row).processurl(&urlbuilder{searchtype: "tvsearch", tvdbid: tvdbid, useseason: useseason, season: season, useepisode: useepisode, episode: episode, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: limitstr, categories: categories}, "", row.MaxAge, row.OutputAsJSON, entries)
	return entries, broke, err
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(row *NzbIndexer, query string, categories string, searchtype string) (*NZBArr, bool, error) {
	entries := new(NZBArr)
	broke, err := getnewznabclient(row).processurl(&urlbuilder{searchtype: searchtype, query: query, addquotesfortitlequery: row.Addquotesfortitlequery, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}, "", row.MaxAge, row.OutputAsJSON, entries)
	return entries, broke, err
}

// QueryNewznabRSS returns x entries of given category
func QueryNewznabRSS(row *NzbIndexer, maxitems int, categories string) (*NZBArr, bool, error) {
	entries := new(NZBArr)
	broke, err := getnewznabclient(row).processurl(&urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories}, "", 0, false, entries)
	return entries, broke, err
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(row *NzbIndexer, maxitems int, categories string, maxrequests int) (*NZBArr, string, string, error) {

	count := 0
	results := new(NZBArr)
	broke, erradd := getnewznabclient(row).processurl(&urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories}, row.LastRssID, 0, false, results)
	if erradd != nil {
		results = nil
		return nil, "", "", erradd
	}
	if broke || len((results.Arr)) == 0 {
		return results, "", "", nil
	}
	count++
	for {
		broke, erradd = getnewznabclient(row).processurl(&urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories, offset: (maxitems * count)}, row.LastRssID, 0, false, results)
		if erradd != nil {
			break
		}
		count++
		if maxrequests == 0 || count >= maxrequests || broke {
			break
		}
	}

	if erradd != nil {
		results = nil
		return nil, row.URL, "", erradd
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

func (client *Client) processurl(urlb *urlbuilder, tillid string, maxage int, outputasjson bool, entries *NZBArr) (bool, error) {
	url := client.buildURL(urlb, maxage)
	urlb = nil
	var err error
	//logger.Log.GlobalLogger.Debug("call url", zap.String("url", url))
	if outputasjson {
		var result searchResponseJSON1
		_, err = client.Client.DoJSON(url, &result)
		if err == nil {
			if len(result.Channel.Item) == 0 {
				return false, Errnoresults
			}
			if entries == nil || len(entries.Arr) == 0 {
				entries.Arr = make([]Nzbwithprio, 0, len(result.Channel.Item))
			}
			var newEntry *Nzbwithprio
			for idx := range result.Channel.Item {
				if result.Channel.Item[idx].Enclosure.Attributes.Url == "" {
					continue
				}
				newEntry = new(Nzbwithprio)
				newEntry.NZB.Title = result.Channel.Item[idx].Title

				if logger.StringContainsRune(newEntry.NZB.Title, '&') || logger.StringContainsRune(newEntry.NZB.Title, '%') {
					newEntry.NZB.Title = html.UnescapeString(newEntry.NZB.Title)
				}
				if strings.Contains(newEntry.NZB.Title, "\\u") {
					unquote, err := strconv.Unquote("\"" + newEntry.NZB.Title + "\"")
					if err == nil {
						newEntry.NZB.Title = unquote
					}
				}
				newEntry.NZB.DownloadURL = result.Channel.Item[idx].Enclosure.Attributes.Url
				if strings.Contains(newEntry.NZB.DownloadURL, "&amp") || logger.StringContainsRune(newEntry.NZB.DownloadURL, '%') {
					newEntry.NZB.DownloadURL = html.UnescapeString(newEntry.NZB.DownloadURL)
				}
				newEntry.NZB.SourceEndpoint = client.APIBaseURL
				if strings.Contains(newEntry.NZB.DownloadURL, ".torrent") || strings.Contains(newEntry.NZB.DownloadURL, "magnet:?") {
					newEntry.NZB.IsTorrent = true
				}

				for key := range result.Channel.Item[idx].Attributes {
					saveAttributes(&newEntry.NZB, result.Channel.Item[idx].Attributes[key].Attribute.Name, result.Channel.Item[idx].Attributes[key].Attribute.Value)
				}
				if newEntry.NZB.Size == 0 && result.Channel.Item[idx].Size != 0 {
					newEntry.NZB.Size = result.Channel.Item[idx].Size
				}
				newEntry.NZB.ID = result.Channel.Item[idx].Guid
				if newEntry.NZB.ID == "" {
					newEntry.NZB.ID = result.Channel.Item[idx].Enclosure.Attributes.Url
				}
				entries.Arr = append(entries.Arr, *newEntry)
				if tillid != "" && tillid == newEntry.NZB.ID {
					newEntry = nil
					return false, nil
				}
				newEntry = nil
			}
			result.close()
			return false, nil
		}
		var result2 searchResponseJSON2
		_, err = client.Client.DoJSON(url, &result2)
		if err != nil {
			return false, err
		}
		if len(result2.Item) == 0 {
			return false, Errnoresults
		}
		if entries == nil || len(entries.Arr) == 0 {
			entries.Arr = make([]Nzbwithprio, 0, len(result2.Item))
		}
		var newEntry *Nzbwithprio
		for idx := range result2.Item {
			if result2.Item[idx].Enclosure.Url == "" {
				continue
			}
			newEntry = new(Nzbwithprio)
			newEntry.NZB.Title = result2.Item[idx].Title

			if logger.StringContainsRune(newEntry.NZB.Title, '&') || logger.StringContainsRune(newEntry.NZB.Title, '%') {
				newEntry.NZB.Title = html.UnescapeString(newEntry.NZB.Title)
			}
			if strings.Contains(newEntry.NZB.Title, "\\u") {
				unquote, err := strconv.Unquote("\"" + newEntry.NZB.Title + "\"")
				if err == nil {
					newEntry.NZB.Title = unquote
				}
			}
			newEntry.NZB.DownloadURL = result2.Item[idx].Enclosure.Url
			if strings.Contains(newEntry.NZB.DownloadURL, "&amp") || logger.StringContainsRune(newEntry.NZB.DownloadURL, '%') {
				newEntry.NZB.DownloadURL = html.UnescapeString(newEntry.NZB.DownloadURL)
			}
			newEntry.NZB.SourceEndpoint = client.APIBaseURL
			if strings.Contains(newEntry.NZB.DownloadURL, ".torrent") || strings.Contains(newEntry.NZB.DownloadURL, "magnet:?") {
				newEntry.NZB.IsTorrent = true
			}

			for key := range result2.Item[idx].Attributes {
				saveAttributes(&newEntry.NZB, result2.Item[idx].Attributes[key].Name, result2.Item[idx].Attributes[key].Value)
			}
			for key := range result2.Item[idx].Attributes2 {
				saveAttributes(&newEntry.NZB, result2.Item[idx].Attributes2[key].Name, result2.Item[idx].Attributes2[key].Value)
			}
			if newEntry.NZB.Size == 0 && result2.Item[idx].Size != 0 {
				newEntry.NZB.Size = result2.Item[idx].Size
			}
			newEntry.NZB.ID = result2.Item[idx].Guid.Guid
			if newEntry.NZB.ID == "" {
				newEntry.NZB.ID = result2.Item[idx].Enclosure.Url
			}
			entries.Arr = append(entries.Arr, *newEntry)
			if tillid != "" && tillid == newEntry.NZB.ID {
				newEntry = nil
				return false, nil
			}
			newEntry = nil
		}
		result2.close()
		return false, nil

	}
	var broke bool
	broke, err = client.Client.DoXML(tillid, client.APIBaseURL, url, entries)
	if err != nil {
		//logger.Log.GlobalLogger.Debug("call url error", zap.Error(err))
		return false, err
	}
	if len(entries.Arr) == 0 {
		return false, Errnoresults
	}
	return broke, nil
}

func saveAttributes(newEntry *NZB, name string, value string) {
	switch name {

	case "guid":
		newEntry.ID = value
	// case "genre":
	// 	newEntry.Genre = value
	case "tvdbid":
		newEntry.TVDBID = logger.StringToInt(value)
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
