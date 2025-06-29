package apiexternal

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

// Client is a type for interacting with a newznab or torznab api
// It contains fields for the api key, base API URL, debug mode,
// and a pointer to the rate limited HTTP client.
type client struct {
	Client        rlHTTPClient // pointer to the rate limited HTTP client
	Lim           slidingwindow.Limiter
	apikey        string // the API key for authentication
	aPIBaseURL    string // the base URL of the API
	aPIBaseURLStr string // the base URL as a string
	aPIUserID     string // the user ID for the API
	debug         bool   // whether to enable debug logging
}

type searchResponseJSON1 struct {
	Title   string `json:"title,omitempty"`
	Channel struct {
		Item []struct {
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
			Title string `json:"title,omitempty"`
			// Link      string `json:"link,omitempty"`
			GUID string `json:"guid,omitempty"`
			Size int64  `json:"size,omitempty"`
			// Date      string `json:"pubDate,omitempty"`
		} `json:"item"`
	} `json:"channel"`
}
type searchResponseJSON2 struct {
	Item []struct {
		Attributes []struct {
			Name  string `json:"_name"`
			Value string `json:"_value"`
		} `json:"newznab:attr,omitempty"`
		Attributes2 []struct {
			Name  string `json:"_name"`
			Value string `json:"_value"`
		} `json:"nntmux:attr,omitempty"`
		GUID struct {
			GUID string `json:"text,omitempty"`
		} `json:"guid,omitempty"`
		Enclosure struct {
			URL string `json:"_url"`
		} `json:"enclosure,omitempty"`
		Title string `json:"title,omitempty"`
		// Link  string `json:"link,omitempty"`
		Size int64 `json:"size,omitempty"`
		// Date      string `json:"pubDate,omitempty"`
	} `json:"item"`
}

const (
	brss       = "/rss"
	bapi       = "/api"
	bqlimit    = "&limit="
	bmovieimdb = "&t=movie&imdbid="
	bquotes    = "%22"
)

var (
	errQualityConfig = errors.New("error getting quality config")
	errBroke         = errors.New("broke")
)

// buildURLNew constructs the API URL to query the Newznab indexer based
// on the given parameters. It handles building the base URL, API key,
// custom URLs, categories, quality settings, output format, etc.
func buildURLNew(
	rss bool,
	indexerid int,
	cfgqual *config.QualityConfig,
	row *config.IndexersConfig,
	addb []byte,
) string {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)
	switch {
	case rss && row.Customrssurl != "":
		bld.WriteString(row.Customrssurl)
	case !rss && row.Customurl != "":
		bld.WriteString(row.Customurl)
	case row.Customapi != "":
		bld.WriteString(row.URL)
		if rss {
			bld.WriteString(brss)
		} else {
			bld.WriteString(bapi)
		}
		bld.WriteByte('?')
		bld.WriteString(row.Customapi)
		bld.WriteByte('=')
		bld.WriteString(row.Apikey)
	default:
		bld.WriteString(row.URL)
		if rss {
			bld.WriteString(brss)
			bld.WriteString("?r=")
			bld.WriteString(row.Apikey)
			bld.WriteString("&i=")
			bld.WriteString(row.Userid)
		} else {
			bld.WriteString(bapi)
			bld.WriteString("?apikey=")
			bld.WriteString(row.Apikey)
		}
	}
	if indexerid != -1 && cfgqual.Indexer[indexerid].CategoriesIndexer != "" {
		if rss {
			if row.Customrsscategory != "" {
				bld.WriteByte('&')
				bld.WriteString(row.Customrsscategory)
				bld.WriteByte('=')
				bld.WriteString(cfgqual.Indexer[indexerid].CategoriesIndexer)
			} else {
				bld.WriteString("&t=")
				bld.WriteString(cfgqual.Indexer[indexerid].CategoriesIndexer)
			}
		} else {
			bld.WriteString("&cat=")
			bld.WriteString(cfgqual.Indexer[indexerid].CategoriesIndexer)
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
	return bld.String() // uses RAM
}

// processurl processes a URL to fetch search results from a Newznab-compatible indexer.
// It handles both JSON and XML-based search responses, extracting the relevant details
// into Nzbwithprio structs and adding them to the provided results slice. It returns
// a boolean indicating whether more results are available, the first ID for continuation,
// and any error that occurred during processing.
func processurl(
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
	urlv string,
	tillid string,
	results *NzbSlice,
	idsearched bool,
) (bool, string, error) {
	c := Getnewznabclient(ind)
	if ind.OutputAsJSON {
		if !strings.Contains(urlv, "://") {
			logger.LogDynamicany1String(
				"error",
				"failed to get url",
				logger.StrURL,
				urlv,
			) // nopointer
			return false, "", logger.Errnoresults
		}
		result, err := doJSONTypeP[searchResponseJSON1](&c.Client, urlv, nil)
		if err == nil {
			if len(result.Channel.Item) == 0 {
				return false, "", logger.Errnoresults
			}
			return c.processjson1(result, ind, qual, tillid, results, idsearched)
		}
		result2, err := doJSONTypeP[searchResponseJSON2](&c.Client, urlv, nil)
		if err != nil {
			return false, "", err
		}
		if len(result2.Item) == 0 {
			return false, "", logger.Errnoresults
		}
		return c.processjson2(result2, ind, qual, tillid, results, idsearched)
	}
	if len(urlv) == 0 {
		return false, "", logger.ErrNotFound
	}

	var (
		b                 Nzbwithprio
		lastfield         int8
		firstid           string
		nameidx, valueidx int
		apiBaseURL        = c.aPIBaseURLStr
	)

	if !strings.Contains(urlv, "://") {
		logger.LogDynamicany1String("error", "failed to get url", logger.StrURL, urlv) // nopointer
		return false, "", logger.Errnoresults
	}
	if apiBaseURL == "" {
		apiBaseURL = urlv
	}
	b.IDSearched = idsearched

	err := ProcessHTTP(&c.Client, urlv, true, func(_ context.Context, r *http.Response) error {
		d := xml.NewDecoder(r.Body)
		d.Strict = false

		for {
			t, err := d.RawToken()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}

			switch tt := t.(type) {
			case xml.StartElement:
				lastfield = -1
				switch tt.Name.Local {
				case "item":
					b.NZB = Nzb{Indexer: ind, Quality: qual, SourceEndpoint: apiBaseURL}
				case strtitle:
					lastfield = 1
				case strlink:
					lastfield = 2
				case strguid:
					lastfield = 3
				case strsize:
					lastfield = 4
				case "enclosure", "source":
					for idx := range tt.Attr {
						b.NZB.setfieldstr(tt.Attr[idx].Name.Local, tt.Attr[idx].Value)
					}
				case "attr":
					nameidx = -1
					valueidx = -1
					for idx := range tt.Attr {
						switch tt.Attr[idx].Name.Local {
						case "name":
							nameidx = idx
						case "value":
							valueidx = idx
						}
					}
					if nameidx == -1 || valueidx == -1 || tt.Attr[valueidx].Value == "" {
						continue
					}
					b.NZB.setfieldstr(tt.Attr[nameidx].Value, tt.Attr[valueidx].Value)
				}
			case xml.CharData:
				if lastfield <= 0 || lastfield >= 5 || len(tt) == 0 {
					continue
				}
				switch fieldmap[lastfield] {
				case strtitle:
					if b.NZB.Title != "" {
						lastfield = 0
					}
				case strlink, "url":
					if b.NZB.DownloadURL != "" {
						lastfield = 0
					}
				case strguid:
					if b.NZB.ID != "" {
						lastfield = 0
					}
				case strsize, "length":
					if b.NZB.Size != 0 {
						lastfield = 0
					}
				case logger.StrImdb:
					if b.NZB.IMDBID != "" {
						lastfield = 0
					}
				case "tvdbid":
					if b.NZB.TVDBID != 0 {
						lastfield = 0
					}
				case "season":
					if b.NZB.Season != "" {
						lastfield = 0
					}
				case "episode":
					if b.NZB.Episode != "" {
						lastfield = 0
					}
				default:
					lastfield = 0
				}
				if lastfield > 0 {
					b.NZB.setfield(fieldmap[lastfield], tt)
				}
			case xml.EndElement:
				if tt.Name.Local != "item" {
					continue
				}
				if b.NZB.ID == "" {
					b.NZB.ID = b.NZB.DownloadURL
				}
				if firstid == "" {
					firstid = b.NZB.ID
				}
				results.Add(&b)
				if tillid != "" && tillid == b.NZB.ID {
					return errBroke
				}
			}
		}
		return nil
	}, nil)
	if errors.Is(err, errBroke) {
		return true, firstid, nil
	}
	if err != nil {
		blockinterval := 5
		if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
			blockinterval = config.SettingsGeneral.FailedIndexerBlockTime
		}
		c.Client.logwait(logger.TimeGetNow().Add(time.Minute*time.Duration(blockinterval)), nil)
		return false, "", err
	}
	return false, firstid, err
}

// processjson1 processes the JSON search response in the searchResponseJSON1 format.
// It extracts the search results into a slice of Nzbwithprio structs that contains
// the NZB details. It handles looping through the search results, extracting the relevant
// fields into the NZB struct, handling special cases like missing fields, and closing the
// response when done. It returns bools indicating more results and if it hit the tillid,
// the first id for more search continuation, and any error.
func (c *client) processjson1(
	result *searchResponseJSON1,
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
	tillid string,
	results *NzbSlice,
	idsearched bool,
) (bool, string, error) {
	var firstid string

	defer result.close()
	for idx := range result.Channel.Item {
		if result.Channel.Item[idx].Enclosure.Attributes.URL == "" {
			continue
		}
		var newEntry Nzbwithprio
		newEntry.IDSearched = idsearched
		newEntry.NZB.Indexer = ind
		newEntry.NZB.Quality = qual
		newEntry.NZB.Title = result.Channel.Item[idx].Title
		newEntry.NZB.Title = logger.UnquoteUnescape(newEntry.NZB.Title)
		newEntry.NZB.DownloadURL = result.Channel.Item[idx].Enclosure.Attributes.URL
		newEntry.NZB.SourceEndpoint = c.aPIBaseURLStr
		if logger.ContainsI(newEntry.NZB.DownloadURL, ".torrent") ||
			logger.ContainsI(newEntry.NZB.DownloadURL, "magnet:?") {
			newEntry.NZB.IsTorrent = true
		}

		for idx2 := range result.Channel.Item[idx].Attributes {
			newEntry.NZB.saveAttributes(
				result.Channel.Item[idx].Attributes[idx2].Attribute.Name,
				result.Channel.Item[idx].Attributes[idx2].Attribute.Value,
			)
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
		results.Add(&newEntry)
		if tillid != "" && tillid == newEntry.NZB.ID {
			return true, firstid, nil
		}
	}
	return false, firstid, nil
}

// processjson2 processes the search result from the JSON API version 2 format.
// It iterates through the items in the result and converts them into Nzbwithprio structs,
// populating the fields based on the attributes in the JSON. It returns whether more
// results are available, the first ID and any error.
func (c *client) processjson2(
	result2 *searchResponseJSON2,
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
	tillid string,
	results *NzbSlice,
	idsearched bool,
) (bool, string, error) {
	var firstid string
	defer result2.close()
	for idx := range result2.Item {
		if result2.Item[idx].Enclosure.URL == "" {
			continue
		}
		var newEntry Nzbwithprio
		newEntry.IDSearched = idsearched
		newEntry.NZB.Indexer = ind
		newEntry.NZB.Quality = qual
		newEntry.NZB.Title = result2.Item[idx].Title
		newEntry.NZB.Title = logger.UnquoteUnescape(newEntry.NZB.Title)
		newEntry.NZB.DownloadURL = result2.Item[idx].Enclosure.URL
		newEntry.NZB.SourceEndpoint = c.aPIBaseURLStr
		if logger.ContainsI(newEntry.NZB.DownloadURL, ".torrent") ||
			logger.ContainsI(newEntry.NZB.DownloadURL, "magnet:?") {
			newEntry.NZB.IsTorrent = true
		}

		for idx2 := range result2.Item[idx].Attributes {
			newEntry.NZB.saveAttributes(
				result2.Item[idx].Attributes[idx2].Name,
				result2.Item[idx].Attributes[idx2].Value,
			)
		}
		for idx2 := range result2.Item[idx].Attributes2 {
			newEntry.NZB.saveAttributes(
				result2.Item[idx].Attributes2[idx2].Name,
				result2.Item[idx].Attributes2[idx2].Value,
			)
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
		results.Add(&newEntry)
		if tillid != "" && tillid == newEntry.NZB.ID {
			return true, firstid, nil
		}
	}
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
func NewznabCheckLimiter(cfgindexer *config.IndexersConfig) bool {
	if !newznabClients.Check(cfgindexer.URL) {
		return true
	}
	ok, _ := newznabClients.GetVal(
		cfgindexer.URL,
	).Client.checkLimiter(
		newznabClients.GetVal(cfgindexer.URL).Client.Ctx,
		false,
	)
	return ok
}

// QueryNewznabMovieImdb queries the Newznab indexer for movies matching
// the given IMDB ID. It builds the query URL based on the config,
// quality, and other parameters, executes the query, and stores
// the results in the given slice. Returns an error if one occurs.
func QueryNewznabMovieImdb(
	cfgind *config.IndexersConfig,
	qual *config.QualityConfig,
	imdbid string,
	indexerid int,
	results *NzbSlice,
) (bool, string, error) {
	if imdbid == "" {
		return false, "", logger.ErrNoID
	}

	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)
	b.WriteString(bmovieimdb)
	b.WriteString(imdbid)
	if cfgind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteString(cfgind.MaxEntriesStr)
	}
	return processurl(
		cfgind,
		qual,
		buildURLNew(false, indexerid, qual, cfgind, b.Bytes()),
		"",
		results,
		true,
	)
}

// getnewznabclient returns a Client for the given IndexersConfig.
// It checks if a client already exists for the given URL,
// and returns it if found. Otherwise creates a new client and caches it.
func Getnewznabclient(row *config.IndexersConfig) *client {
	if !newznabClients.Check(row.URL) {
		newznabClients.Add(row.URL, newNewznab(true, row), 0, false, 0)
	}
	return newznabClients.GetVal(row.URL)
}

// DownloadNZB downloads an NZB file from the given URL and saves it to the specified target path.
// If the filename is empty, it uses the base name of the URL as the filename.
// It uses the provided IndexersConfig to get a Newznab client and downloads the file using ProcessHTTP.
// Returns an error if any step fails.
func DownloadNZB(
	filename string,
	targetpath string,
	urlv string,
	idxcfg *config.IndexersConfig,
) error {
	// Create the file
	if filename == "" {
		filename = filepath.Base(urlv)
	}

	return ProcessHTTP(
		&Getnewznabclient(idxcfg).Client,
		urlv,
		true,
		func(_ context.Context, r *http.Response) error {
			out, err := os.Create(filepath.Join(targetpath, filename))
			if err != nil {
				return err
			}
			defer out.Close()

			// Write the body to file
			_, err = io.Copy(out, r.Body)
			if err != nil {
				return err
			}
			return out.Sync()
		},
		nil,
	)
}

// QueryNewznabTvTvdb queries the Newznab indexer for TV episodes matching
// the given TVDB ID, season, and episode. It builds the query URL based on
// the config, quality, and other parameters, executes the query, and stores
// the results in the given slice. Returns an error if one occurs.
func QueryNewznabTvTvdb(
	cfgind *config.IndexersConfig,
	qual *config.QualityConfig,
	tvdbid, indexerid int,
	season, episode string,
	useseason, useepisode bool,
	results *NzbSlice,
) (bool, string, error) {
	if tvdbid == 0 {
		return false, "", logger.ErrNoID
	}
	if indexerid == -1 {
		return false, "", errQualityConfig
	}

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
	return processurl(
		cfgind,
		qual,
		buildURLNew(false, indexerid, qual, cfgind, b.Bytes()),
		"",
		results,
		true,
	)
}

// getaddstr constructs the search query string for a Newznab API request based on the
// given media type configuration, title, and Nzbwithprio information. If the media type
// configuration indicates the content is a series and the Nzbwithprio has a non-zero
// year, the year is appended to the title. Otherwise, if the Nzbwithprio has an
// identifier, it is appended to the title. Otherwise, the title is returned as-is.
func getaddstr(cfgp *config.MediaTypeConfig, title string, e *Nzbwithprio) string {
	if !cfgp.Useseries && e.Info.Year != 0 {
		return logger.JoinStrings(title, logger.StrSpace, logger.IntToString(e.Info.Year))
	} else if e.Info.Identifier != "" {
		return logger.JoinStrings(title, logger.StrSpace, e.Info.Identifier)
	}
	return title
}

// QueryNewznabQuery queries the Newznab API for the given search query, indexer
// configuration, quality config, categories, mutex, and result slice. It handles
// escaping the search query, adding quotes if configured, and limiting results.
// It returns any error that occurs.
func QueryNewznabQuery(
	cfgp *config.MediaTypeConfig,
	e *Nzbwithprio,
	cfgind *config.IndexersConfig,
	qual *config.QualityConfig,
	title string,
	indexerid int,
	results *NzbSlice,
) (bool, string, error) {
	if title == "" {
		return false, "", nil
	}
	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)
	b.WriteString("&t=search&q=")
	if cfgind.Addquotesfortitlequery {
		b.WriteString(bquotes)
	}

	b.WriteString(url.QueryEscape(getaddstr(cfgp, title, e)))

	if cfgind.Addquotesfortitlequery {
		b.WriteString(bquotes)
	}
	if cfgind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteUInt16(cfgind.MaxEntries)
	}

	return processurl(
		cfgind,
		qual,
		buildURLNew(false, indexerid, qual, cfgind, b.Bytes()),
		"",
		results,
		false,
	)
}

// QueryNewznabRSS queries the Newznab RSS feed for the given indexer
// configuration, quality config, max items, categories, mutex, and result
// slice. It returns a bool indicating if the results were truncated, and
// an error if one occurred.
func QueryNewznabRSS(
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
	maxitems, indexerid int,
	results *NzbSlice,
) (bool, string, error) {
	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)
	if maxitems != 0 {
		b.WriteString(bqlimit)
		b.WriteInt(maxitems)
	}
	return processurl(
		ind,
		qual,
		buildURLNew(true, indexerid, qual, ind, b.Bytes()),
		"",
		results,
		false,
	)
}

// QueryNewznabRSSLastCustom queries the Newznab RSS feed for the latest items
// matching the given configuration and quality parameters. It handles pagination
// to retrieve multiple pages of results if needed. It returns the ID of the first
// result and any error.
func QueryNewznabRSSLastCustom(
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
	tillid string,
	indexerid int,
	results *NzbSlice,
) (string, error) {
	if indexerid == -1 {
		return "", errQualityConfig
	}
	if ind.MaxEntries == 0 {
		ind.MaxEntries = 100
		ind.MaxEntriesStr = "100"
	}

	b := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(b)
	if ind.MaxEntries != 0 {
		b.WriteString(bqlimit)
		b.WriteString(ind.MaxEntriesStr)
	}
	bld := buildURLNew(true, indexerid, qual, ind, b.Bytes())
	maxloop := ind.RssEntriesloop
	if maxloop == 0 {
		maxloop = 2
	}
	brokeloop, firstid, err := processurl(ind, qual, bld, tillid, results, false)
	if err != nil || results == nil || len(results.Arr) == 0 || brokeloop || maxloop == 1 {
		return firstid, err
	}
	if maxloop == 0 {
		maxloop = 1
	}
	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)
	for count := range maxloop - 1 {
		buf.WriteString(bld)
		buf.WriteString("&offset=")
		buf.WriteUInt16(ind.MaxEntries * (uint16(count) + 1))
		brokeloop, _, err = processurl(ind, qual, buf.String(), tillid, results, false)
		buf.Reset()
		if err != nil || brokeloop || len(results.Arr) == 0 {
			break
		}
	}
	return firstid, err
}

// QueryNewznabRSSLast queries the Newznab RSS feed for the latest items matching
// the given configuration and quality parameters. It returns the latest item ID
// and any error.
func QueryNewznabRSSLast(
	cfgind *config.IndexersConfig,
	qual *config.QualityConfig,
	tillid string,
	indexerid int,
	results *NzbSlice,
) (string, error) {
	return QueryNewznabRSSLastCustom(cfgind, qual, tillid, indexerid, results)
}

// newNewznab creates a new Newznab client instance.
// It takes in debug mode and indexer configuration row parameters.
// It sets up rate limiting and timeouts based on the configuration.
// It returns a pointer to the constructed Client instance.
func newNewznab(debug bool, row *config.IndexersConfig) *client {
	if row.Limitercalls == 0 {
		row.Limitercalls = 5
	}
	if row.Limiterseconds == 0 {
		row.Limiterseconds = 20
	}

	var limiter slidingwindow.Limiter
	if row.LimitercallsDaily != 0 {
		limiter = slidingwindow.NewLimiter(24*time.Hour, int64(row.LimitercallsDaily))
	}

	d := client{
		apikey:        row.Apikey,
		aPIBaseURL:    row.URL,
		aPIBaseURLStr: row.URL,
		aPIUserID:     row.Userid,
		debug:         debug,
		Lim: slidingwindow.NewLimiter(
			time.Duration(row.Limiterseconds)*time.Second,
			int64(row.Limitercalls),
		),
	}
	d.Client = newClient(
		row.URL,
		row.DisableTLSVerify,
		row.DisableCompression,
		&d.Lim,
		row.LimitercallsDaily != 0,
		&limiter, row.TimeoutSeconds)
	return &d
}
