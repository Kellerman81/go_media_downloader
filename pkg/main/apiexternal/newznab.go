package apiexternal

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
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
	InitRows               int
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
	GUID string `json:"guid,omitempty"`
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

type rawNZBJson2 struct {
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

const (
	apiPath = "/api"
	rssPath = "/rss"
)

var (
	newznabClients []Clients = make([]Clients, 0, 10)
)

func NewznabCheckLimiter(urlv string) (bool, error) {
	intid := -1
	for idxi := range newznabClients {
		if newznabClients[idxi].Name == urlv {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(&newznabClients, func(e Clients) bool { return e.Name == urlv })
	if intid == -1 {
		return true, nil
	}
	return newznabClients[intid].Client.Client.checkLimiter(false, 20, 1)
}

func (client *Client) buildURL(builder *urlbuilder, maxage int) string {

	if builder.query == "" && builder.imdbid == "" && builder.tvdbid == 0 && !builder.rss {
		return ""
	}

	var bld strings.Builder
	bld.Grow(200 + len(builder.query) + len(client.Apikey))

	if builder.customurl != "" {
		bld.WriteString(builder.customurl)
	} else if builder.customapi != "" {
		var path string
		if builder.rss {
			path = rssPath
		} else {
			path = apiPath
		}
		bld.WriteString(client.APIBaseURL)
		bld.WriteString(path)
		bld.WriteString("?")
		bld.WriteString(builder.customapi)
		bld.WriteString("=")
		bld.WriteString(client.Apikey)
	} else {
		var path string
		if builder.rss {
			path = rssPath
		} else {
			path = apiPath
		}
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
	if builder.useseason && builder.tvdbid != 0 {
		bld.WriteString("&season=")
		bld.WriteString(logger.IntToString(builder.season))
	}
	if builder.useepisode && builder.tvdbid != 0 {
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
			bld.WriteString("&q=")
			bld.WriteString("%22")
			bld.WriteString(QueryEscape(&builder.query))
			bld.WriteString("%22")
		} else {
			bld.WriteString("&q=")
			bld.WriteString(QueryEscape(&builder.query))
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
	// if !logger.DisableVariableCleanup {
	// 	defer bld.Reset()
	// }
	defer bld.Reset()
	return bld.String()
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(indexer string, quality string, row *NzbIndexer, imdbid string, categories string) (*[]NZB, error) {
	if imdbid == "" {
		return nil, logger.ErrNoID
	}
	// if row.InitRows != 0 {
	// 	entries.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	return nzbs3to2(getnewznabclient(row).processurl(indexer, quality, &urlbuilder{searchtype: logger.StrMovie, imdbid: imdbid, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}, "", row.MaxAge, row.OutputAsJSON))
}

func nzbs3to2(nzbs *[]NZB, brk bool, err error) (*[]NZB, error) {
	return nzbs, err
}

func getnewznabclient(row *NzbIndexer) *Client {
	intid := -1
	for idxi := range newznabClients {
		if newznabClients[idxi].Name == row.URL {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(&newznabClients, func(e Clients) bool { return e.Name == row.URL })
	if intid != -1 {
		return newznabClients[intid].Client
	}
	newznabClients = append(newznabClients, Clients{Name: row.URL, Client: NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, row.DisableCompression, true, limiterconfig{row.Limitercalls, row.Limiterseconds, row.LimitercallsDaily, row.TimeoutSeconds})})
	intid = -1
	for idxi := range newznabClients {
		if newznabClients[idxi].Name == row.URL {
			intid = idxi
			break
		}
	}
	//intid = logger.IndexFunc(&newznabClients, func(e Clients) bool { return e.Name == row.URL })
	if intid != -1 {
		return newznabClients[intid].Client
	}
	return nil
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(indexer string, quality string, row *NzbIndexer, tvdbid int, categories string, season int, episode int, useseason bool, useepisode bool) (*[]NZB, error) {
	if tvdbid == 0 {
		return nil, logger.ErrNoID
	}
	var limitstr string
	if !useepisode || !useseason {
		limitstr = "100"
	}
	// if row.InitRows != 0 {
	// 	entries.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	return nzbs3to2(getnewznabclient(row).processurl(indexer, quality, &urlbuilder{searchtype: "tvsearch", tvdbid: tvdbid, useseason: useseason, season: season, useepisode: useepisode, episode: episode, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: limitstr, categories: categories}, "", row.MaxAge, row.OutputAsJSON))
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(indexer string, quality string, row *NzbIndexer, query string, categories string, searchtype string) (*[]NZB, error) {
	// if row.InitRows != 0 {
	// 	entries.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	return nzbs3to2(getnewznabclient(row).processurl(indexer, quality, &urlbuilder{searchtype: searchtype, query: query, addquotesfortitlequery: row.Addquotesfortitlequery, outputAsJSON: row.OutputAsJSON, customurl: row.Customurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", categories: categories}, "", row.MaxAge, row.OutputAsJSON))
}

// QueryNewznabRSS returns x entries of given category
func QueryNewznabRSS(indexer string, quality string, row *NzbIndexer, maxitems int, categories string) (*[]NZB, bool, error) {
	// if row.InitRows != 0 {
	// 	entries.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	return getnewznabclient(row).processurl(indexer, quality, &urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories}, "", 0, false)
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(indexer string, quality string, row *NzbIndexer, maxitems int, categories string, maxrequests int) (*[]NZB, error) {

	count := 0
	// if row.InitRows != 0 {
	// 	results.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	results, broke, erradd := getnewznabclient(row).processurl(indexer, quality, &urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories}, row.LastRssID, 0, false)
	if erradd != nil {
		logger.Clear(results)
		return nil, erradd
	}
	if broke || results == nil || len(*results) == 0 {
		return results, nil
	}
	count++
	var addresults *[]NZB
	for {
		addresults, broke, erradd = getnewznabclient(row).processurl(indexer, quality, &urlbuilder{rss: true, customurl: row.Customrssurl, customrsscategory: row.Customrsscategory, customapi: row.Customapi, additionalQueryParams: row.AdditionalQueryParams, limit: "0", num: maxitems, categories: categories, offset: (maxitems * count)}, row.LastRssID, 0, false)
		if erradd != nil {
			logger.Clear(addresults)
			break
		}
		logger.Grow(results, len(*addresults))
		*results = append(*results, *addresults...)
		logger.Clear(addresults)
		count++
		if maxrequests == 0 || count >= maxrequests || broke {
			break
		}
	}

	if erradd != nil {
		logger.Clear(results)
		return nil, erradd
	}
	return results, nil
}

func getdailylimiter(limit int64) *slidingwindow.Limiter {
	if limit != 0 {
		return slidingwindow.NewLimiter(24*time.Hour, limit)
	}
	return nil
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
			slidingwindow.NewLimiter(time.Duration(lmtconfig.limiterseconds)*time.Second, int64(lmtconfig.limitercalls)),
			lmtconfig.limitercallsdaily != 0,
			getdailylimiter(int64(lmtconfig.limitercallsdaily)), lmtconfig.timeoutseconds),

		//rate.New(lmtconfig.limitercalls, lmtconfig.limitercallsdaily, time.Duration(lmtconfig.limiterseconds)*time.Second), lmtconfig.timeoutseconds,
		//),
	}
}

func (client *Client) processurl(indexer string, quality string, urlb *urlbuilder, tillid string, maxage int, outputasjson bool) (*[]NZB, bool, error) {
	defer logger.ClearVar(urlb)
	if !outputasjson {
		return client.Client.DoXML(indexer, quality, tillid, client.APIBaseURL, client.buildURL(urlb, maxage))
	}
	result, err := DoJSONType[searchResponseJSON1](client.Client, client.buildURL(urlb, maxage))
	if err == nil {
		defer result.close()
		if len(result.Channel.Item) == 0 {
			return nil, false, logger.Errnoresults
		}
		return client.processjson1(result, indexer, quality, tillid)
	}
	result2, err := DoJSONType[searchResponseJSON2](client.Client, client.buildURL(urlb, maxage))
	if err != nil {
		return nil, false, err
	}
	defer result2.close()
	if len(result2.Item) == 0 {
		return nil, false, logger.Errnoresults
	}
	return client.processjson2(result2, indexer, quality, tillid)
}

func (client *Client) processjson1(result *searchResponseJSON1, indexer string, quality string, tillid string) (*[]NZB, bool, error) {
	entries := make([]NZB, 0, len(result.Channel.Item))
	for idx := range result.Channel.Item {
		if result.Channel.Item[idx].Enclosure.Attributes.URL == "" {
			continue
		}
		var newEntry NZB
		newEntry.Indexer = indexer
		newEntry.Quality = quality
		newEntry.Title = result.Channel.Item[idx].Title
		logger.HTMLUnescape(&newEntry.Title)
		logger.Unquote(&newEntry.Title)
		newEntry.DownloadURL = result.Channel.Item[idx].Enclosure.Attributes.URL
		logger.HTMLUnescape(&newEntry.DownloadURL)
		newEntry.SourceEndpoint = client.APIBaseURL
		if logger.ContainsI(newEntry.DownloadURL, ".torrent") || logger.ContainsI(newEntry.DownloadURL, "magnet:?") {
			newEntry.IsTorrent = true
		}

		for key := range result.Channel.Item[idx].Attributes {
			saveAttributes(&newEntry, result.Channel.Item[idx].Attributes[key].Attribute.Name, result.Channel.Item[idx].Attributes[key].Attribute.Value)
		}
		if newEntry.Size == 0 && result.Channel.Item[idx].Size != 0 {
			newEntry.Size = result.Channel.Item[idx].Size
		}
		newEntry.ID = result.Channel.Item[idx].GUID
		if newEntry.ID == "" {
			newEntry.ID = result.Channel.Item[idx].Enclosure.Attributes.URL
		}
		entries = append(entries, newEntry)
		if tillid != "" && tillid == newEntry.ID {
			return &entries, false, nil
		}
	}
	result.close()
	return &entries, false, nil
}

func (client *Client) processjson2(result2 *searchResponseJSON2, indexer string, quality string, tillid string) (*[]NZB, bool, error) {
	entries := make([]NZB, 0, len(result2.Item))
	for idx := range result2.Item {
		if result2.Item[idx].Enclosure.URL == "" {
			continue
		}
		var newEntry NZB
		newEntry.Indexer = indexer
		newEntry.Quality = quality
		newEntry.Title = result2.Item[idx].Title
		logger.HTMLUnescape(&newEntry.Title)
		logger.Unquote(&newEntry.Title)
		newEntry.DownloadURL = result2.Item[idx].Enclosure.URL
		logger.HTMLUnescape(&newEntry.DownloadURL)
		newEntry.SourceEndpoint = client.APIBaseURL
		if logger.ContainsI(newEntry.DownloadURL, ".torrent") || logger.ContainsI(newEntry.DownloadURL, "magnet:?") {
			newEntry.IsTorrent = true
		}

		for key := range result2.Item[idx].Attributes {
			saveAttributes(&newEntry, result2.Item[idx].Attributes[key].Name, result2.Item[idx].Attributes[key].Value)
		}
		for key := range result2.Item[idx].Attributes2 {
			saveAttributes(&newEntry, result2.Item[idx].Attributes2[key].Name, result2.Item[idx].Attributes2[key].Value)
		}
		if newEntry.Size == 0 && result2.Item[idx].Size != 0 {
			newEntry.Size = result2.Item[idx].Size
		}
		newEntry.ID = result2.Item[idx].GUID.GUID
		if newEntry.ID == "" {
			newEntry.ID = result2.Item[idx].Enclosure.URL
		}
		entries = append(entries, newEntry)
		if tillid != "" && tillid == newEntry.ID {
			return &entries, false, nil
		}
	}
	result2.close()
	return &entries, false, nil
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
		newEntry.Size = logger.StringToInt64(value)
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
	logger.Clear(&s.Channel.Item)
	logger.ClearVar(s)
}

func (s *searchResponseJSON2) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.Clear(&s.Item)
	logger.ClearVar(s)
}
