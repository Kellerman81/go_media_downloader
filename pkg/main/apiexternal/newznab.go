package apiexternal

import (
	"bytes"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
)

// clients stores a name and client instance
type clients struct {
	name string  // name of the client
	c    *client // pointer to the client instance
}

// Client is a type for interacting with a newznab or torznab api
// It contains fields for the api key, base API URL, debug mode,
// and a pointer to the rate limited HTTP client
type client struct {
	apikey        string        // the API key for authentication
	aPIBaseURL    string        // the base URL of the API
	aPIBaseURLStr string        // the base URL as a string
	aPIUserID     string        // the user ID for the API
	debug         bool          // whether to enable debug logging
	Client        *rlHTTPClient // pointer to the rate limited HTTP client
}

func setfield(field string, value string, n *nzb) {
	switch field {
	case strtitle:
		if n.Title == "" {
			n.Title = value
		}
	case strlink:
		if n.DownloadURL == "" {
			n.DownloadURL = value
		}
	case strguid:
		if n.ID == "" {
			n.ID = value
		}
	case strsize:
		if n.Size == 0 {
			n.Size = logger.StringToInt64(value)
		}
	case logger.StrImdb:
		if n.IMDBID == "" {
			n.IMDBID = value
			if value != "" {
				n.IMDBID = logger.AddImdbPrefix(n.IMDBID)
			}
		}
	case "tvdbid":
		if n.TVDBID == 0 {
			n.TVDBID = logger.StringToInt(value)
		}
	case "season":
		if n.Season == "" {
			n.Season = value
		}
	case "episode":
		if n.Episode == "" {
			n.Episode = value
		}
	}
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

// qualityIndexerByQualityAndTemplate appends additional query parameters from the
// QualityConfig to the buffer based on matching the indexer template name.
func qualityIndexerByQualityAndTemplate(bld *bytes.Buffer, cfgqual *config.QualityConfig, cfgindexer *config.IndexersConfig) {
	if cfgindexer == nil {
		return
	}
	for idx := range cfgqual.Indexer {
		if strings.EqualFold(cfgqual.Indexer[idx].TemplateIndexer, cfgindexer.Name) {
			bld.WriteString(cfgqual.Indexer[idx].AdditionalQueryParams)
			return
		}
	}
}

// NewznabCheckLimiter checks if the rate limiter is triggered for the given indexer config.
// It loops through the newznabClients slice to find the matching client by URL,
// and calls checkLimiter on it to check if the rate limit has been hit.
// Returns true if under limit, false if over limit, and error if there was a problem.
func NewznabCheckLimiter(cfgindexer *config.IndexersConfig) (bool, error) {
	for idxi := range newznabClients {
		if newznabClients[idxi].name == cfgindexer.URL {
			return newznabClients[idxi].c.Client.checkLimiter(false, 20, 1)
		}
	}
	return true, nil
}

const (
	brss          = "/rss"
	bapi          = "/api"
	bqr           = "?r="
	bandi         = "&i="
	bqapi         = "?apikey="
	bandt         = "&t="
	bandcat       = "&cat="
	bjson         = "&o=json"
	bage          = "&maxage="
	bdl           = "&dl=1"
	bqlimit       = "&limit="
	bseason       = "&season="
	bepi          = "&ep="
	btvsearchtvdb = "&t=tvsearch&tvdbid="
	bmovieimdb    = "&t=movie&imdbid="
	bsearchq      = "&t=search&q="
	bquotes       = "%22"
)

// buildURLNew constructs the API URL to query the Newznab indexer based
// on the given parameters. It handles building the base URL, API key,
// custom URLs, categories, quality settings, output format, etc.
func (c *client) buildURLNew(rss bool, categories *config.QualityIndexerConfig, cfgqual *config.QualityConfig, row *config.IndexersConfig, varlen int, fn func(*bytes.Buffer)) string {
	bld := logger.PlBuffer.Get()
	//var bld bytes.Buffer
	i := 90 + varlen
	if rss && row.Customrssurl != "" {
		i += len(row.Customrssurl)
	} else if !rss && row.Customurl != "" {
		i += len(row.Customurl)
	} else if row.Customapi != "" {
		i += len(c.aPIBaseURL) + 6
		i += len(row.Customapi)
		i += len(c.apikey)
	} else {
		i += len(c.aPIBaseURL) + 4
		i += len(c.apikey)
		if rss {
			i += len(c.aPIUserID) + 6
		} else {
			i += 8
		}
	}
	if categories != nil {
		i += len(categories.CategoriesIndexer)
	}
	bld.Grow(i)
	//defer bld.Reset()

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
			bld.WriteString(bqr)
			bld.WriteString(c.apikey)
			bld.WriteString(bandi)
			bld.WriteString(c.aPIUserID)
		} else {
			bld.WriteString(bqapi)
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
				bld.WriteString(bandt)
				bld.WriteString(categories.CategoriesIndexer)
			}
		} else {
			bld.WriteString(bandcat)
			bld.WriteString(categories.CategoriesIndexer)
		}
	}
	if row.OutputAsJSON {
		bld.WriteString(bjson)
	}
	if row.MaxAge != 0 {
		bld.WriteString(bage)
		logger.BuilderAddInt(bld, row.MaxAge)
	}
	bld.WriteString(bdl)
	qualityIndexerByQualityAndTemplate(bld, cfgqual, row)
	if fn != nil {
		fn(bld)
	}
	defer logger.PlBuffer.Put(bld)
	return bld.String()
}

// QueryNewznabMovieImdb queries the Newznab indexer for movies matching
// the given IMDB ID. It builds the query URL based on the config,
// quality, and other parameters, executes the query, and stores
// the results in the given slice. Returns an error if one occurs.
func QueryNewznabMovieImdb(cfgind *config.IndexersConfig, qual *config.QualityConfig, imdbid string, categories *config.QualityIndexerConfig, mu *sync.Mutex, results *[]Nzbwithprio) XMLResponse {
	if imdbid == "" {
		return XMLResponse{Err: logger.ErrNoID}
	}
	// if row.InitRows != 0 {
	// 	entries.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	client := Getnewznabclient(cfgind)
	bldstr := client.buildURLNew(false, categories, qual, cfgind, 0, func(b *bytes.Buffer) {
		b.WriteString(bmovieimdb)
		b.WriteString(imdbid)
		if cfgind.MaxEntries != 0 {
			b.WriteString(bqlimit)
			logger.BuilderAddInt(b, cfgind.MaxEntries)
		}
	})

	return client.processurlsimple(cfgind, qual, bldstr, mu, results)
}

// Getnewznabclient returns a Client for the given IndexersConfig.
// It checks if a client already exists for the given URL,
// and returns it if found. Otherwise creates a new client and caches it.
func Getnewznabclient(row *config.IndexersConfig) *client {
	for idxi := range newznabClients {
		if newznabClients[idxi].name == row.URL {
			return newznabClients[idxi].c
		}
	}
	c := newNewznab(true, row)
	newznabClients = append(newznabClients, clients{name: row.URL, c: c})
	return c
}

// QueryNewznabTvTvdb queries the Newznab indexer for TV episodes matching
// the given TVDB ID, season, and episode. It builds the query URL based on
// the config, quality, and other parameters, executes the query, and stores
// the results in the given slice. Returns an error if one occurs.
func QueryNewznabTvTvdb(cfgind *config.IndexersConfig, qual *config.QualityConfig, tvdbid int, categories *config.QualityIndexerConfig, season string, episode string, useseason bool, useepisode bool, mu *sync.Mutex, results *[]Nzbwithprio) XMLResponse {
	if tvdbid == 0 {
		return XMLResponse{Err: logger.ErrNoID}
	}
	if categories == nil {
		return XMLResponse{Err: errors.New("error getting quality config")}
	}
	// if row.InitRows != 0 {
	// 	entries.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	client := Getnewznabclient(cfgind)
	bldstr := client.buildURLNew(false, categories, qual, cfgind, 20, func(b *bytes.Buffer) {
		b.WriteString(btvsearchtvdb)
		logger.BuilderAddInt(b, tvdbid)
		if !useepisode || !useseason {
			b.WriteString(bqlimit)
			b.WriteString("100")
		} else if cfgind.MaxEntries != 0 {
			b.WriteString(bqlimit)
			logger.BuilderAddInt(b, cfgind.MaxEntries)
		}
		if useseason && season != "" {
			b.WriteString(bseason)
			b.WriteString(season)
		}
		if useepisode && episode != "" {
			b.WriteString(bepi)
			b.WriteString(episode)
		}
	})

	//urlv := client.buildURL(urlbuilder{searchtype: "tvsearch", tvdbid: tvdbid, useseason: useseason, season: season, useepisode: useepisode, episode: episode, limit: limitstr, categories: categories}, row)
	return client.processurlsimple(cfgind, qual, bldstr, mu, results)
}

// QueryNewznabQuery queries the Newznab API for the given search query, indexer
// configuration, quality config, categories, mutex, and result slice. It handles
// escaping the search query, adding quotes if configured, and limiting results.
// It returns any error that occurs.
func QueryNewznabQuery(cfgind *config.IndexersConfig, qual *config.QualityConfig, query string, categories *config.QualityIndexerConfig, mu *sync.Mutex, results *[]Nzbwithprio) XMLResponse {
	if query == "" {
		return XMLResponse{}
	}
	client := Getnewznabclient(cfgind)
	bldstr := client.buildURLNew(false, categories, qual, cfgind, len(query)+10, func(b *bytes.Buffer) {
		b.WriteString(bsearchq)
		if cfgind.Addquotesfortitlequery {
			b.WriteString(bquotes)
			writeescapequery(b, query)
			b.WriteString(bquotes)
			if cfgind.MaxEntries != 0 {
				b.WriteString(bqlimit)
				logger.BuilderAddInt(b, cfgind.MaxEntries)
			}
		} else {
			writeescapequery(b, query)
			if cfgind.MaxEntries != 0 {
				b.WriteString(bqlimit)
				logger.BuilderAddInt(b, cfgind.MaxEntries)
			}
		}
	})

	return client.processurlsimple(cfgind, qual, bldstr, mu, results)
}

// writeescapequery escapes the provided query string using url.QueryEscape and writes it to the provided bytes.Buffer.
// This allows properly escaping query strings to be placed in URLs.
func writeescapequery(bld *bytes.Buffer, query string) {
	bld.WriteString(url.QueryEscape(query))
}

// QueryNewznabRSS queries the Newznab RSS feed for the given indexer
// configuration, quality config, max items, categories, mutex, and result
// slice. It returns a bool indicating if the results were truncated, and
// an error if one occurred.
func QueryNewznabRSS(ind *config.IndexersConfig, qual *config.QualityConfig, maxitems int, categories *config.QualityIndexerConfig, mu *sync.Mutex, results *[]Nzbwithprio) XMLResponse {
	// if row.InitRows != 0 {
	// 	entries.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }
	client := Getnewznabclient(ind)
	bldstr := client.buildURLNew(true, categories, qual, ind, 0, func(b *bytes.Buffer) {
		if maxitems != 0 {
			b.WriteString(bqlimit)
			logger.BuilderAddInt(b, maxitems)
		}
	})

	return client.processurl(ind, qual, bldstr, "", mu, ind.MaxEntries, results)
}

// QueryNewznabRSSLastCustom queries the Newznab RSS feed for the latest items
// matching the given configuration and quality parameters. It handles pagination
// to retrieve multiple pages of results if needed. It returns the ID of the first
// result and any error.
func QueryNewznabRSSLastCustom(ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, categories *config.QualityIndexerConfig, mu *sync.Mutex, results *[]Nzbwithprio) XMLResponse {
	// if row.InitRows != 0 {
	// 	results.Arr = make(*[]Nzbwithprio, 0, row.InitRows)
	// }

	if categories == nil {
		return XMLResponse{Err: errors.New("error getting quality config")}
	}
	maxitems := ind.MaxEntries
	if maxitems == 0 {
		maxitems = 100
	}
	client := Getnewznabclient(ind)
	bldstr := client.buildURLNew(true, categories, qual, ind, 0, func(b *bytes.Buffer) {
		if maxitems != 0 {
			b.WriteString(bqlimit)
			logger.BuilderAddInt(b, maxitems)
		}
	})
	maxloop := ind.RssEntriesloop
	if maxloop == 0 {
		maxloop = 2
	}

	retval := client.processurl(ind, qual, bldstr, tillid, mu, maxitems*maxloop, results)
	if retval.Err != nil {
		return retval
	}
	if results == nil || len(*results) == 0 {
		return retval
	}
	if retval.BrokeLoop || maxloop == 1 {
		return retval
	}
	if maxloop == 0 {
		maxloop = 1
	}
	urlv := logger.JoinStrings(bldstr, "&offset=")

	var retloop XMLResponse
	for count := 1; count <= maxloop; count++ {
		//getid = len(*results)
		//broke, _, erradd = client.processurl(ind, qual, logger.JoinStrings(urlv, strconv.Itoa(maxitems*count)), tillid, mu, maxitems*maxloop, results)
		retloop = client.processurl(ind, qual, logger.JoinStrings(urlv, strconv.Itoa(maxitems*count)), tillid, mu, maxitems*maxloop, results)
		if retloop.Err != nil || len(*results) == 0 {
			break
		}
		if retloop.BrokeLoop {
			break
		}
	}

	return retval
}

// QueryNewznabRSSLast queries the Newznab RSS feed for the latest items matching
// the given configuration and quality parameters. It returns the latest item ID
// and any error.
func QueryNewznabRSSLast(cfgind *config.IndexersConfig, qual *config.QualityConfig, tillid string, categories *config.QualityIndexerConfig, mu *sync.Mutex, results *[]Nzbwithprio) XMLResponse {
	return QueryNewznabRSSLastCustom(cfgind, qual, tillid, categories, mu, results)
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

	var limiter *slidingwindow.Limiter
	if row.LimitercallsDaily != 0 {
		limiter = slidingwindow.NewLimiter(24*time.Hour, int64(row.LimitercallsDaily))
	}

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
			slidingwindow.NewLimiter(time.Duration(row.Limiterseconds)*time.Second, int64(row.Limitercalls)),
			row.LimitercallsDaily != 0,
			limiter, row.TimeoutSeconds),
	}
}

// processurl processes a URL from the Newznab API, handling both XML and JSON responses.
// It takes in indexer and quality configuration, the URL, latest ID, whether to output as JSON,
// result limit, mutex, result set size, and result set pointer.
// It returns a flag indicating if processing broke early, the latest ID, and an error if one occurred.
// For XML responses it calls DoXMLItemSub on the client.
// For JSON responses it calls processjson1 or processjson2 depending on the response structure.
func (c *client) processurl(ind *config.IndexersConfig, qual *config.QualityConfig, urlv string, tillid string, mu *sync.Mutex, createsize int, results *[]Nzbwithprio) XMLResponse {
	//*results = slices.Grow(*results, limit)
	if !ind.OutputAsJSON {
		if len(c.aPIBaseURLStr) == 0 {
			return c.Client.DoXMLItem(ind, qual, tillid, urlv, urlv, mu, createsize, results)
		}
		return c.Client.DoXMLItem(ind, qual, tillid, c.aPIBaseURLStr, urlv, mu, createsize, results)
	}
	if c.Client.checklimiterwithdaily() {
		return XMLResponse{Err: logger.Errnoresults}
	}
	result, err := DoJSONType[searchResponseJSON1](c.Client, urlv)
	if err == nil {
		if len(result.Channel.Item) == 0 {
			return XMLResponse{Err: logger.Errnoresults}
		}
		return c.processjson1(&result, ind, qual, tillid, results)
	}
	result2, err := DoJSONType[searchResponseJSON2](c.Client, urlv)
	if err != nil {
		return XMLResponse{Err: err}
	}
	if len(result2.Item) == 0 {
		return XMLResponse{Err: logger.Errnoresults}
	}
	return c.processjson2(&result2, ind, qual, tillid, results)
}

// processurlsimple processes a search URL for the simple API.
// It handles both XML and JSON API responses, calling the appropriate
// processing functions based on the API format.
func (c *client) processurlsimple(ind *config.IndexersConfig, qual *config.QualityConfig, bldstr string, mu *sync.Mutex, results *[]Nzbwithprio) XMLResponse {
	return c.processurl(ind, qual, bldstr, "", mu, ind.MaxEntries, results)
}

// processjson1 processes the JSON search response in the searchResponseJSON1 format.
// It extracts the search results into a slice of Nzbwithprio structs that contains
// the NZB details. It handles looping through the search results, extracting the relevant
// fields into the NZB struct, handling special cases like missing fields, and closing the
// response when done. It returns bools indicating more results and if it hit the tillid,
// the first id for more search continuation, and any error.
func (c *client) processjson1(result *searchResponseJSON1, ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, results *[]Nzbwithprio) XMLResponse {
	//entries := make([]NZB, 0, len(result.Channel.Item))
	var retval XMLResponse
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

		for key := range result.Channel.Item[idx].Attributes {
			saveAttributes(&newEntry.NZB, result.Channel.Item[idx].Attributes[key].Attribute.Name, result.Channel.Item[idx].Attributes[key].Attribute.Value)
		}
		if newEntry.NZB.Size == 0 && result.Channel.Item[idx].Size != 0 {
			newEntry.NZB.Size = result.Channel.Item[idx].Size
		}
		newEntry.NZB.ID = result.Channel.Item[idx].GUID
		if newEntry.NZB.ID == "" {
			newEntry.NZB.ID = result.Channel.Item[idx].Enclosure.Attributes.URL
		}
		if retval.FirstID == "" {
			retval.FirstID = newEntry.NZB.ID
		}
		*results = append(*results, newEntry)
		if tillid != "" && tillid == newEntry.NZB.ID {
			result.close()
			retval.BrokeLoop = true
			return retval
		}
	}
	result.close()
	return retval
}

// processjson2 processes the search result from the JSON API version 2 format.
// It iterates through the items in the result and converts them into Nzbwithprio structs,
// populating the fields based on the attributes in the JSON. It returns whether more
// results are available, the first ID and any error.
func (c *client) processjson2(result2 *searchResponseJSON2, ind *config.IndexersConfig, qual *config.QualityConfig, tillid string, results *[]Nzbwithprio) XMLResponse {
	//entries := make([]NZB, 0, len(result2.Item))
	var retval XMLResponse
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

		for key := range result2.Item[idx].Attributes {
			saveAttributes(&newEntry.NZB, result2.Item[idx].Attributes[key].Name, result2.Item[idx].Attributes[key].Value)
		}
		for key := range result2.Item[idx].Attributes2 {
			saveAttributes(&newEntry.NZB, result2.Item[idx].Attributes2[key].Name, result2.Item[idx].Attributes2[key].Value)
		}
		if newEntry.NZB.Size == 0 && result2.Item[idx].Size != 0 {
			newEntry.NZB.Size = result2.Item[idx].Size
		}
		newEntry.NZB.ID = result2.Item[idx].GUID.GUID
		if newEntry.NZB.ID == "" {
			newEntry.NZB.ID = result2.Item[idx].Enclosure.URL
		}
		if retval.FirstID == "" {
			retval.FirstID = newEntry.NZB.ID
		}
		*results = append(*results, newEntry)
		if tillid != "" && tillid == newEntry.NZB.ID {
			result2.close()
			retval.BrokeLoop = true
			return retval
		}
	}
	result2.close()
	return retval
}

// saveAttributes populates the fields of the NZB struct from
// the name/value pairs passed in. It handles translating the
// values to the appropriate types for the NZB struct fields.
func saveAttributes(newEntry *nzb, name string, value string) {
	switch name {
	case strguid:
		newEntry.ID = value
	case "tvdbid":
		newEntry.TVDBID = logger.StringToInt(value)
	case "season":
		newEntry.Season = value
	case "episode":
		newEntry.Episode = value
	case logger.StrImdb:
		newEntry.IMDBID = value
	case strsize:
		newEntry.Size = logger.StringToInt64(value)
	}
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
