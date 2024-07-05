package apiexternal

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

// Client is a type for interacting with a newznab or torznab api
// It contains fields for the api key, base API URL, debug mode,
// and a pointer to the rate limited HTTP client
type client struct {
	apikey        string       // the API key for authentication
	aPIBaseURL    string       // the base URL of the API
	aPIBaseURLStr string       // the base URL as a string
	aPIUserID     string       // the user ID for the API
	debug         bool         // whether to enable debug logging
	Client        rlHTTPClient // pointer to the rate limited HTTP client
}

type searchResponseJSON1 struct {
	Title   string                     `json:"title,omitempty"`
	Channel searchResponseJSON1Channel `json:"channel"`
}
type searchResponseJSON1Channel struct {
	Item []rawNZBJson1 `json:"item"`
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
	Enclosure enclosureJson1 `json:"enclosure,omitempty"`

	Attributes []attributesJson1 `json:"attr,omitempty"`
}

type enclosureJson1 struct {
	Attributes enclosureJson1Attribute `json:"@attributes,omitempty"`
}
type enclosureJson1Attribute struct {
	URL string `json:"url"`
}
type attributesJson1 struct {
	Attribute attributesJson1Attribute `json:"@attributes,omitempty"`
}
type attributesJson1Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type rawNZBJson2 struct {
	Title string `json:"title,omitempty"`
	//Link  string `json:"link,omitempty"`
	Size int64     `json:"size,omitempty"`
	GUID guidJson2 `json:"guid,omitempty"`
	//Date      string `json:"pubDate,omitempty"`
	Enclosure enclosureJson2Attribute `json:"enclosure,omitempty"`

	Attributes  []attributesJson2Attribute `json:"newznab:attr,omitempty"`
	Attributes2 []attributesJson2Attribute `json:"nntmux:attr,omitempty"`
}

type guidJson2 struct {
	GUID string `json:"text,omitempty"`
}
type enclosureJson2Attribute struct {
	URL string `json:"_url"`
}
type attributesJson2Attribute struct {
	Name  string `json:"_name"`
	Value string `json:"_value"`
}

const (
	brss       = "/rss"
	bapi       = "/api"
	bqlimit    = "&limit="
	bmovieimdb = "&t=movie&imdbid="
	bquotes    = "%22"
)

// buildURLNew constructs the API URL to query the Newznab indexer based
// on the given parameters. It handles building the base URL, API key,
// custom URLs, categories, quality settings, output format, etc.
func (c *client) buildURLNew(rss bool, categories *config.QualityIndexerConfig, cfgqual *config.QualityConfig, row *config.IndexersConfig, addb []byte) string {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)
	if rss && row.Customrssurl != "" {
		bld.WriteString(row.Customrssurl)
	} else if !rss && row.Customurl != "" {
		bld.WriteString(row.Customurl)
	} else if row.Customapi != "" {
		bld.WriteString(c.aPIBaseURL)
		if rss {
			bld.WriteString(brss)
		} else {
			bld.WriteString(bapi)
		}
		bld.WriteRune('?')
		bld.WriteString(row.Customapi)
		bld.WriteRune('=')
		bld.WriteString(c.apikey)
	} else {
		bld.WriteString(c.aPIBaseURL)
		if rss {
			bld.WriteString(brss)
		} else {
			bld.WriteString(bapi)
		}
		if rss {
			bld.WriteString("?r=")
			bld.WriteString(c.apikey)
			bld.WriteString("&i=")
			bld.WriteString(c.aPIUserID)
		} else {
			bld.WriteString("?apikey=")
			bld.WriteString(c.apikey)
		}
	}
	if categories != nil && categories.CategoriesIndexer != "" {
		if rss {
			if row.Customrsscategory != "" {
				bld.WriteRune('&')
				bld.WriteString(row.Customrsscategory)
				bld.WriteRune('=')
				bld.WriteString(categories.CategoriesIndexer)
			} else {
				bld.WriteString("&t=")
				bld.WriteString(categories.CategoriesIndexer)
			}
		} else {
			bld.WriteString("&cat=")
			bld.WriteString(categories.CategoriesIndexer)
		}
	}
	if row.OutputAsJSON {
		bld.WriteString("&o=json")
	}
	if row.MaxAge != 0 {
		bld.WriteString("&maxage=")
		bld.WriteUInt16(row.MaxAge)
	}
	bld.WriteString("&dl=1")

	for idx := range cfgqual.Indexer {
		if strings.EqualFold(cfgqual.Indexer[idx].TemplateIndexer, row.Name) {
			bld.WriteString(cfgqual.Indexer[idx].AdditionalQueryParams)
			break
		}
	}
	if addb != nil {
		bld.Write(addb)
	}
	return bld.String()
}

// processurl processes a URL from the Newznab API, handling both XML and JSON responses.
// It takes in indexer and quality configuration, the URL, latest ID, whether to output as JSON,
// result limit, mutex, result set size, and result set pointer.
// It returns a flag indicating if processing broke early, the latest ID, and an error if one occurred.
// For XML responses it calls DoXMLItemSub on the client.
// For JSON responses it calls processjson1 or processjson2 depending on the response structure.
func (c *client) processurl(ind *config.IndexersConfig, qual *config.QualityConfig, urlv string, tillid string, results *NzbSlice) (bool, string, error) {
	if !ind.OutputAsJSON {
		if len(c.aPIBaseURLStr) == 0 {
			return c.Client.DoXMLItem(ind, qual, tillid, urlv, urlv, results)
		}
		return c.Client.DoXMLItem(ind, qual, tillid, c.aPIBaseURLStr, urlv, results)
	}
	if c.Client.checklimiterwithdaily() {
		return false, "", logger.Errnoresults
	}
	result, err := doJSONType[searchResponseJSON1](&c.Client, urlv)
	if err == nil {
		if len(result.Channel.Item) == 0 {
			return false, "", logger.Errnoresults
		}
		return c.processjson1(&result, ind, qual, tillid, results)
	}
	result2, err := doJSONType[searchResponseJSON2](&c.Client, urlv)
	if err != nil {
		return false, "", err
	}
	if len(result2.Item) == 0 {
		return false, "", logger.Errnoresults
	}
	return c.processjson2(&result2, ind, qual, tillid, results)
}

// processjson1 processes the JSON search response in the searchResponseJSON1 format.
// It extracts the search results into a slice of Nzbwithprio structs that contains
// the NZB details. It handles looping through the search results, extracting the relevant
// fields into the NZB struct, handling special cases like missing fields, and closing the
// response when done. It returns bools indicating more results and if it hit the tillid,
// the first id for more search continuation, and any error.
func (c *client) processjson1(result *searchResponseJSON1, ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, results *NzbSlice) (bool, string, error) {
	//entries := make([]NZB, 0, len(result.Channel.Item))
	var firstid string
	for idx := range result.Channel.Item {
		if result.Channel.Item[idx].Enclosure.Attributes.URL == "" {
			continue
		}
		var newEntry Nzbwithprio
		newEntry.NZB.Indexer = ind
		newEntry.NZB.Quality = qual
		newEntry.NZB.Title = result.Channel.Item[idx].Title
		newEntry.NZB.Title = logger.UnquoteUnescape(newEntry.NZB.Title)
		newEntry.NZB.DownloadURL = result.Channel.Item[idx].Enclosure.Attributes.URL
		newEntry.NZB.SourceEndpoint = c.aPIBaseURLStr
		if logger.ContainsI(newEntry.NZB.DownloadURL, ".torrent") || logger.ContainsI(newEntry.NZB.DownloadURL, "magnet:?") {
			newEntry.NZB.IsTorrent = true
		}

		for idx2 := range result.Channel.Item[idx].Attributes {
			newEntry.NZB.saveAttributes(result.Channel.Item[idx].Attributes[idx2].Attribute.Name, result.Channel.Item[idx].Attributes[idx2].Attribute.Value)
		}
		if newEntry.NZB.Size == 0 && result.Channel.Item[idx].Size != 0 {
			newEntry.NZB.Size = result.Channel.Item[idx].Size
		}
		newEntry.NZB.ID = result.Channel.Item[idx].GUID
		if newEntry.NZB.ID == "" {
			newEntry.NZB.ID = result.Channel.Item[idx].Enclosure.Attributes.URL
		}
		if firstid == "" {
			firstid = newEntry.NZB.ID
		}
		results.Mu.Lock()
		results.Arr = append(results.Arr, newEntry)
		results.Mu.Unlock()
		if tillid != "" && tillid == newEntry.NZB.ID {
			result.close()
			return true, firstid, nil
		}
	}
	result.close()
	return false, firstid, nil
}

// processjson2 processes the search result from the JSON API version 2 format.
// It iterates through the items in the result and converts them into Nzbwithprio structs,
// populating the fields based on the attributes in the JSON. It returns whether more
// results are available, the first ID and any error.
func (c *client) processjson2(result2 *searchResponseJSON2, ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, results *NzbSlice) (bool, string, error) {
	//entries := make([]NZB, 0, len(result2.Item))
	var firstid string
	for idx := range result2.Item {
		if result2.Item[idx].Enclosure.URL == "" {
			continue
		}
		var newEntry Nzbwithprio
		newEntry.NZB.Indexer = ind
		newEntry.NZB.Quality = qual
		newEntry.NZB.Title = result2.Item[idx].Title
		newEntry.NZB.Title = logger.UnquoteUnescape(newEntry.NZB.Title)
		newEntry.NZB.DownloadURL = result2.Item[idx].Enclosure.URL
		newEntry.NZB.SourceEndpoint = c.aPIBaseURLStr
		if logger.ContainsI(newEntry.NZB.DownloadURL, ".torrent") || logger.ContainsI(newEntry.NZB.DownloadURL, "magnet:?") {
			newEntry.NZB.IsTorrent = true
		}

		for idx2 := range result2.Item[idx].Attributes {
			newEntry.NZB.saveAttributes(result2.Item[idx].Attributes[idx2].Name, result2.Item[idx].Attributes[idx2].Value)
		}
		for idx2 := range result2.Item[idx].Attributes2 {
			newEntry.NZB.saveAttributes(result2.Item[idx].Attributes2[idx2].Name, result2.Item[idx].Attributes2[idx2].Value)
		}
		if newEntry.NZB.Size == 0 && result2.Item[idx].Size != 0 {
			newEntry.NZB.Size = result2.Item[idx].Size
		}
		newEntry.NZB.ID = result2.Item[idx].GUID.GUID
		if newEntry.NZB.ID == "" {
			newEntry.NZB.ID = result2.Item[idx].Enclosure.URL
		}
		if firstid == "" {
			firstid = newEntry.NZB.ID
		}
		results.Mu.Lock()
		results.Arr = append(results.Arr, newEntry)
		results.Mu.Unlock()
		if tillid != "" && tillid == newEntry.NZB.ID {
			result2.close()
			return true, firstid, nil
		}
	}
	result2.close()
	return false, firstid, nil
}

// close cleans up the search response by setting the Item slice to nil.
// This allows the garbage collector to reclaim the memory unless
// config.SettingsGeneral.DisableVariableCleanup is true.
func (s *searchResponseJSON1) close() {
	if config.SettingsGeneral.DisableVariableCleanup || s == nil {
		return
	}
	s.Channel.Item = nil
}

// close releases resources associated with the searchResponseJSON2
// struct if the DisableVariableCleanup setting is false. It sets the
// Item field to nil to allow garbage collection.
func (s *searchResponseJSON2) close() {
	if config.SettingsGeneral.DisableVariableCleanup || s == nil {
		return
	}
	s.Item = nil
}

// NewznabCheckLimiter checks if the rate limiter is triggered for the given indexer config.
// It loops through the newznabClients slice to find the matching client by URL,
// and calls checkLimiter on it to check if the rate limit has been hit.
// Returns true if under limit, false if over limit, and error if there was a problem.
func NewznabCheckLimiter(cfgindexer *config.IndexersConfig) (bool, error) {
	if !newznabClients.Check(cfgindexer.URL) {
		return true, nil
	}
	return newznabClients.Get(cfgindexer.URL).Client.checkLimiter(false, 20, 1)
}

// QueryNewznabMovieImdb queries the Newznab indexer for movies matching
// the given IMDB ID. It builds the query URL based on the config,
// quality, and other parameters, executes the query, and stores
// the results in the given slice. Returns an error if one occurs.
func QueryNewznabMovieImdb(cfgind *config.IndexersConfig, qual *config.QualityConfig, imdbid string, categories *config.QualityIndexerConfig, results *NzbSlice) (bool, string, error) {
	if imdbid == "" {
		return false, "", logger.ErrNoID
	}
	client := Getnewznabclient(cfgind)

	var addbyte []byte
	if cfgind.MaxEntries != 0 {
		addbyte = logger.JoinStringsByte(bmovieimdb, imdbid, bqlimit, cfgind.MaxEntriesStr)
	} else {
		addbyte = logger.JoinStringsByte(bmovieimdb, imdbid)
	}

	return client.processurl(cfgind, qual, client.buildURLNew(false, categories, qual, cfgind, addbyte), "", results)
}

// Getnewznabclient returns a Client for the given IndexersConfig.
// It checks if a client already exists for the given URL,
// and returns it if found. Otherwise creates a new client and caches it.
func Getnewznabclient(row *config.IndexersConfig) *client {
	if newznabClients.Check(row.URL) {
		return newznabClients.Get(row.URL)
	}
	newznabClients.Set(row.URL, newNewznab(true, row))
	return newznabClients.Get(row.URL)
}

// QueryNewznabTvTvdb queries the Newznab indexer for TV episodes matching
// the given TVDB ID, season, and episode. It builds the query URL based on
// the config, quality, and other parameters, executes the query, and stores
// the results in the given slice. Returns an error if one occurs.
func QueryNewznabTvTvdb(cfgind *config.IndexersConfig, qual *config.QualityConfig, tvdbid int, categories *config.QualityIndexerConfig, season string, episode string, useseason bool, useepisode bool, results *NzbSlice) (bool, string, error) {
	if tvdbid == 0 {
		return false, "", logger.ErrNoID
	}
	if categories == nil {
		return false, "", errors.New("error getting quality config")
	}
	client := Getnewznabclient(cfgind)

	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)
	b.WriteString("&t=tvsearch&tvdbid=")
	b.WriteInt(tvdbid)
	if !useepisode || !useseason {
		b.WriteString(bqlimit)
		b.WriteString("100")
	} else if cfgind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteUInt16(cfgind.MaxEntries)
	}
	if useseason && season != "" {
		b.WriteString("&season=")
		b.WriteString(season)
	}
	if useepisode && episode != "" {
		b.WriteString("&ep=")
		b.WriteString(episode)
	}
	//urlv := client.buildURL(urlbuilder{searchtype: "tvsearch", tvdbid: tvdbid, useseason: useseason, season: season, useepisode: useepisode, episode: episode, limit: limitstr, categories: categories}, row)
	return client.processurl(cfgind, qual, client.buildURLNew(false, categories, qual, cfgind, b.Bytes()), "", results)
}

// QueryNewznabQuery queries the Newznab API for the given search query, indexer
// configuration, quality config, categories, mutex, and result slice. It handles
// escaping the search query, adding quotes if configured, and limiting results.
// It returns any error that occurs.
func QueryNewznabQuery(cfgind *config.IndexersConfig, qual *config.QualityConfig, query string, categories *config.QualityIndexerConfig, results *NzbSlice) (bool, string, error) {
	if query == "" {
		return false, "", nil
	}
	client := Getnewznabclient(cfgind)
	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)
	b.WriteString("&t=search&q=")
	if cfgind.Addquotesfortitlequery {
		b.WriteString(bquotes)
	}
	b.WriteUrl(query)

	if cfgind.Addquotesfortitlequery {
		b.WriteString(bquotes)
	}
	if cfgind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteUInt16(cfgind.MaxEntries)
	}

	return client.processurl(cfgind, qual, client.buildURLNew(false, categories, qual, cfgind, b.Bytes()), "", results)
}

// QueryNewznabRSS queries the Newznab RSS feed for the given indexer
// configuration, quality config, max items, categories, mutex, and result
// slice. It returns a bool indicating if the results were truncated, and
// an error if one occurred.
func QueryNewznabRSS(ind *config.IndexersConfig, qual *config.QualityConfig, maxitems int, categories *config.QualityIndexerConfig, results *NzbSlice) (bool, string, error) {
	client := Getnewznabclient(ind)

	var addbyte []byte
	if maxitems != 0 {
		addbyte = logger.JoinStringsByte(bqlimit, strconv.Itoa(maxitems))
	}
	return client.processurl(ind, qual, client.buildURLNew(true, categories, qual, ind, addbyte), "", results)
}

// QueryNewznabRSSLastCustom queries the Newznab RSS feed for the latest items
// matching the given configuration and quality parameters. It handles pagination
// to retrieve multiple pages of results if needed. It returns the ID of the first
// result and any error.
func QueryNewznabRSSLastCustom(ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, categories *config.QualityIndexerConfig, results *NzbSlice) (bool, string, error) {
	if categories == nil {
		return false, "", errors.New("error getting quality config")
	}
	maxitems := ind.MaxEntries
	maxitemsstr := ind.MaxEntriesStr
	if maxitems == 0 {
		maxitems = 100
		maxitemsstr = "100"
	}
	client := Getnewznabclient(ind)

	var addbyte []byte
	if maxitems != 0 {
		addbyte = logger.JoinStringsByte(bqlimit, maxitemsstr)
	}
	bldstr := client.buildURLNew(true, categories, qual, ind, addbyte)
	maxloop := ind.RssEntriesloop
	if maxloop == 0 {
		maxloop = 2
	}

	brokeloop, firstid, err := client.processurl(ind, qual, bldstr, tillid, results)
	if err != nil {
		return brokeloop, firstid, err
	}
	if results == nil || len(results.Arr) == 0 {
		return brokeloop, firstid, err
	}
	if brokeloop || maxloop == 1 {
		return brokeloop, firstid, err
	}
	if maxloop == 0 {
		maxloop = 1
	}
	urlv := logger.JoinStrings(bldstr, "&offset=") //JoinStrings
	var i uint16
	for count := range maxloop - 1 {
		i = maxitems * (uint16(count) + 1)
		broke2, _, err := client.processurl(ind, qual, logger.JoinStrings(urlv, logger.IntToString(i)), tillid, results)
		if err != nil || len(results.Arr) == 0 {
			break
		}
		if broke2 {
			break
		}
	}
	// for count := 1; count <= int(maxloop); count++ {

	// }

	return brokeloop, firstid, err
}

// QueryNewznabRSSLast queries the Newznab RSS feed for the latest items matching
// the given configuration and quality parameters. It returns the latest item ID
// and any error.
func QueryNewznabRSSLast(cfgind *config.IndexersConfig, qual *config.QualityConfig, tillid string, categories *config.QualityIndexerConfig, results *NzbSlice) (bool, string, error) {
	return QueryNewznabRSSLastCustom(cfgind, qual, tillid, categories, results)
}

// newNewznab creates a new Newznab client instance.
// It takes in debug mode and indexer configuration row parameters.
// It sets up rate limiting and timeouts based on the configuration.
// It returns a pointer to the constructed Client instance.
func newNewznab(debug bool, row *config.IndexersConfig) *client {
	if row.Limitercalls == 0 {
		row.Limitercalls = 3
	}
	if row.Limiterseconds == 0 {
		row.Limiterseconds = 10
	}

	var limiter slidingwindow.Limiter
	if row.LimitercallsDaily != 0 {
		limiter = slidingwindow.NewLimiter(24*time.Hour, int64(row.LimitercallsDaily))
	}

	calllimiter := slidingwindow.NewLimiter(time.Duration(row.Limiterseconds)*time.Second, int64(row.Limitercalls))

	return &client{
		apikey:        row.Apikey,
		aPIBaseURL:    row.URL,
		aPIBaseURLStr: row.URL,
		aPIUserID:     row.Userid,
		debug:         debug,
		Client: NewClient(
			row.URL,
			row.DisableTLSVerify,
			row.DisableCompression,
			&calllimiter,
			row.LimitercallsDaily != 0,
			&limiter, row.TimeoutSeconds),
	}
}
