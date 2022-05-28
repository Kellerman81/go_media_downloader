package apiexternal

import (
	"bytes"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/time/rate"
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
	MaxAge                  int
}

// QueryNewznabMovieImdb searches Indexers for imbid - strip tt at beginning!
func QueryNewznabMovieImdb(row NzbIndexer, imdbid string, categories string) (resultsadd []NZB, failedindexers string, erradd error) {
	if imdbid == "" {
		erradd = errors.New("no imdbid")
		return
	}
	var client *Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	var buildurl bytes.Buffer
	defer buildurl.Reset()
	if len(row.Customurl) >= 1 {
		buildurl.WriteString(row.Customurl)
	} else {
		buildurl.WriteString(client.ApiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(client.Apikey)
	}
	buildurl.WriteString("&imdbid=")
	buildurl.WriteString(imdbid)
	buildurl.WriteString("&cat=")
	buildurl.WriteString(categories)
	buildurl.WriteString("&dl=1&t=movie")
	if row.OutputAsJson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(row.Additional_query_params)

	resultsadd, _, erradd = client.processurl(buildurl.String(), "", row.MaxAge, row.OutputAsJson)

	if erradd != nil {
		failedindexers = row.URL
	}
	return
}

// QueryNewznabTvTvdb searches Indexers for tvdbid using season and episodes
func QueryNewznabTvTvdb(row NzbIndexer, tvdbid int, categories string, season int, episode int, useseason bool, useepisode bool) (resultsadd []NZB, failedindexers string, erradd error) {
	if tvdbid == 0 {
		erradd = errors.New("no tvdbid")
		return
	}
	var client *Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}

	var buildurl bytes.Buffer
	defer buildurl.Reset()
	if len(row.Customurl) >= 1 {
		buildurl.WriteString(row.Customurl)
	} else {
		buildurl.WriteString(client.ApiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(client.Apikey)
	}
	buildurl.WriteString("&tvdbid=")
	buildurl.WriteString(strconv.Itoa(tvdbid))
	if useseason {
		buildurl.WriteString("&season=")
		buildurl.WriteString(strconv.Itoa(season))
	}
	if useepisode {
		buildurl.WriteString("&ep=")
		buildurl.WriteString(strconv.Itoa(episode))
	}
	if !useepisode || !useseason {
		buildurl.WriteString("&limit=")
		buildurl.WriteString("100")
	}
	buildurl.WriteString("&cat=")
	buildurl.WriteString(categories)
	buildurl.WriteString("&dl=1&t=tvsearch")
	if row.OutputAsJson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(row.Additional_query_params)

	resultsadd, _, erradd = client.processurl(buildurl.String(), "", row.MaxAge, row.OutputAsJson)

	if erradd != nil {
		failedindexers = row.URL
	}
	return
}

// QueryNewznabQuery searches Indexers for string
func QueryNewznabQuery(row NzbIndexer, query string, categories string, searchtype string) (resultsadd []NZB, failedindexers string, erradd error) {
	defer logger.ClearVar(&categories)
	var client *Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	var buildurl bytes.Buffer
	defer buildurl.Reset()
	if len(row.Customurl) >= 1 {
		buildurl.WriteString(row.Customurl)
	} else {
		buildurl.WriteString(client.ApiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(client.Apikey)
	}
	buildurl.WriteString("&q=")
	if row.Addquotesfortitlequery {
		buildurl.WriteString("%22")
	}
	buildurl.WriteString(url.PathEscape(query))
	if row.Addquotesfortitlequery {
		buildurl.WriteString("%22")
	}
	buildurl.WriteString("&cat=")
	buildurl.WriteString(categories)
	buildurl.WriteString("&dl=1&t=")
	buildurl.WriteString(searchtype)
	if row.OutputAsJson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(row.Additional_query_params)

	resultsadd, _, erradd = client.processurl(buildurl.String(), "", row.MaxAge, row.OutputAsJson)

	if erradd != nil {
		failedindexers = row.URL
	}
	return
}

type Clients struct {
	Name   string
	Client Client
}

var NewznabClients []Clients

func checkclient(find string) bool {
	for idx := range NewznabClients {
		if NewznabClients[idx].Name == find {
			return true
		}
	}
	return false
}

func getclient(find string) *Client {
	for idx := range NewznabClients {
		if NewznabClients[idx].Name == find {
			return &NewznabClients[idx].Client
		}
	}
	return nil
}

// QueryNewznabRSS returns x entries of given category
func QueryNewznabRSS(row NzbIndexer, maxitems int, categories string) (resultsadd []NZB, failedindexers string, erradd error) {
	var client *Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	resultsadd, _, erradd = client.processurl(client.buildRssUrl(row.Customrssurl, row.Customrsscategory, row.Customapi, row.Additional_query_params, maxitems, categories, 0, false), "", 0, false)

	if erradd != nil {
		failedindexers = row.URL
	}
	return
}

// QueryNewznabRSS returns entries of given category up to id
func QueryNewznabRSSLast(row NzbIndexer, maxitems int, categories string, maxrequests int) (resultsadd []NZB, failedindexers string, lastid string, err error) {
	var client *Client
	if checkclient(row.URL) {
		client = getclient(row.URL)
	} else {
		client = NewNewznab(row.URL, row.Apikey, row.UserID, row.SkipSslCheck, true, row.Limitercalls, row.Limiterseconds)
		NewznabClients = append(NewznabClients, Clients{Name: row.URL, Client: *client})
	}
	count := 0
	var broke bool
	var nzbs_temp []NZB
	defer logger.ClearVar(&nzbs_temp)

	for {
		nzbs_temp = nil
		nzbs_temp, broke, err = client.processurl(client.buildRssUrl(row.Customrssurl, row.Customrsscategory, row.Customapi, row.Additional_query_params, maxitems, categories, (maxitems*maxrequests), false), row.LastRssId, 0, false)
		if err != nil {
			break
		}
		if count == 0 {
			resultsadd = nzbs_temp
		} else {
			resultsadd = append(resultsadd, nzbs_temp...)
		}
		nzbs_temp = nil
		count++
		if maxrequests == 0 || count >= maxrequests || broke || len((resultsadd)) == 0 {
			break
		}
	}

	if err != nil {
		failedindexers = row.URL
	} else {
		if len((resultsadd)) >= 1 {
			lastid = (resultsadd)[0].ID
		}
	}
	return
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
func NewNewznab(baseURL string, apikey string, userID int, insecure bool, debug bool, limitercalls int, limiterseconds int) *Client {
	if limitercalls == 0 {
		limitercalls = 3
	}
	if limiterseconds == 0 {
		limiterseconds = 10
	}

	ret := Client{
		Apikey:     apikey,
		ApiBaseURL: baseURL,
		ApiUserID:  userID,
		Debug:      debug,
		Client: NewClient(
			insecure,
			rate.NewLimiter(rate.Every(time.Duration(limiterseconds)*time.Second), limitercalls),
			slidingwindow.NewLimiterNoStop(time.Duration(limiterseconds)*time.Second, int64(limitercalls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })),
	}
	return &ret
}

// LoadRSSFeedUntilNZBID fetches NZBs until a given NZB id is reached.
func (c *Client) SearchWithQueryUntilNZBID(categories string, query string, searchType string, addquotes bool, id string, additional_query_params string, customurl string, maxage int, outputasjson bool) ([]NZB, error) {
	var buildurl bytes.Buffer
	defer buildurl.Reset()
	if len(customurl) >= 1 {
		buildurl.WriteString(customurl)
	} else {
		buildurl.WriteString(c.ApiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(c.Apikey)
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
	buildurl.WriteString(categories)
	buildurl.WriteString("&dl=1&t=")
	buildurl.WriteString(searchType)
	if outputasjson {
		buildurl.WriteString("&o=json")
	}
	buildurl.WriteString(additional_query_params)

	outnzbs, _, err := c.processurl(buildurl.String(), id, maxage, outputasjson)
	defer logger.ClearVar(&outnzbs)
	return outnzbs, err
}

func (c *Client) buildRssUrl(customrssurl string, customrsscategory string, customapi string, additional_query_params string, num int, categories string, offset int, outputasjson bool) string {
	defer logger.ClearVar(&categories)
	var buildurl bytes.Buffer
	defer buildurl.Reset()
	if len(customrssurl) >= 1 {
		buildurl.WriteString(customrssurl)
	} else if len(customapi) >= 1 {
		buildurl.WriteString(c.ApiBaseURL)
		buildurl.WriteString(rssPath)
		buildurl.WriteString("?")
		buildurl.WriteString(customapi)
		buildurl.WriteString("=")
		buildurl.WriteString(c.Apikey)
	} else {
		buildurl.WriteString(c.ApiBaseURL)
		buildurl.WriteString(rssPath)
		buildurl.WriteString("?r=")
		buildurl.WriteString(c.Apikey)
		buildurl.WriteString("&i=")
		buildurl.WriteString(strconv.Itoa(c.ApiUserID))
	}
	buildurl.WriteString("&num=")
	buildurl.WriteString(strconv.Itoa(num))
	if len(customrsscategory) >= 1 {
		buildurl.WriteString("&")
		buildurl.WriteString(customrsscategory)
		buildurl.WriteString("=")
		buildurl.WriteString(categories)
	} else {
		buildurl.WriteString("&t=")
		buildurl.WriteString(categories)
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

func (c *Client) processurl(url string, tillid string, maxage int, outputasjson bool) ([]NZB, bool, error) {
	scantime := time.Now()
	if maxage != 0 {
		scantime = scantime.AddDate(0, 0, 0-maxage)
	}
	breakatid := false
	if outputasjson {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, false, err
		}
		var result SearchResponseJson1
		defer logger.ClearVar(&result)
		err = c.Client.DoJson(req, &result)
		if err != nil {
			if err == errors.New("429") {
				config.Slepping(false, 60)
				err = c.Client.DoJson(req, &result)
				if err != nil {
					return nil, false, err
				}
			} else if err == errors.New("please wait") {
				config.Slepping(false, 10)
				err = c.Client.DoJson(req, &result)
				if err != nil {
					return nil, false, err
				}
			} else if err == nil {
				defer logger.ClearVar(&result.Channel.Item)
				entries := make([]NZB, 0, len((result.Channel.Item)))
				defer logger.ClearVar(&entries)
				var newEntry NZB
				defer logger.ClearVar(&newEntry)
				for idx := range result.Channel.Item {
					if len(result.Channel.Item[idx].Enclosure.Attributes.URL) == 0 {
						continue
					}
					newEntry = NZB{}
					var err error
					if strings.Contains(result.Channel.Item[idx].Title, "&") || strings.Contains(result.Channel.Item[idx].Title, "%") {
						newEntry.Title = html.UnescapeString(result.Channel.Item[idx].Title)
					} else {
						if strings.Contains(result.Channel.Item[idx].Title, "\\u") {
							newEntry.Title, err = strconv.Unquote("\"" + result.Channel.Item[idx].Title + "\"")
							if err != nil {
								newEntry.Title = result.Channel.Item[idx].Title
							}
						} else {
							newEntry.Title = result.Channel.Item[idx].Title
						}
					}
					if strings.Contains(result.Channel.Item[idx].Enclosure.Attributes.URL, "&amp") || strings.Contains(result.Channel.Item[idx].Enclosure.Attributes.URL, "%") {
						newEntry.DownloadURL = html.UnescapeString(result.Channel.Item[idx].Enclosure.Attributes.URL)
					} else {
						newEntry.DownloadURL = result.Channel.Item[idx].Enclosure.Attributes.URL
					}
					newEntry.SourceEndpoint = c.ApiBaseURL
					newEntry.IsTorrent = false
					if strings.Contains(result.Channel.Item[idx].Enclosure.Attributes.URL, ".torrent") || strings.Contains(result.Channel.Item[idx].Enclosure.Attributes.URL, "magnet:?") {
						newEntry.IsTorrent = true
					}

					for idx2 := range result.Channel.Item[idx].Attributes {
						saveAttributes(&newEntry, result.Channel.Item[idx].Attributes[idx2].Attribute.Name, result.Channel.Item[idx].Attributes[idx2].Attribute.Value)
					}
					if newEntry.Size == 0 && result.Channel.Item[idx].Size != 0 {
						newEntry.Size = result.Channel.Item[idx].Size
					}
					if newEntry.ID == "" && result.Channel.Item[idx].Guid != "" {
						newEntry.ID = result.Channel.Item[idx].Guid
					} else if newEntry.ID == "" {
						newEntry.ID = result.Channel.Item[idx].Enclosure.Attributes.URL
					}
					entries = append(entries, newEntry)
					if tillid == newEntry.ID && tillid != "" {
						breakatid = true
						break
					}
				}
				return entries, breakatid, nil
			} else {
				var result SearchResponseJson2
				defer logger.ClearVar(&result)
				err = c.Client.DoJson(req, &result)
				if err == errors.New("429") {
					config.Slepping(false, 60)
					err = c.Client.DoJson(req, &result)
					if err != nil {
						return nil, false, err
					}
				} else if err == errors.New("please wait") {
					config.Slepping(false, 10)
					err = c.Client.DoJson(req, &result)
					if err != nil {
						return nil, false, err
					}
				} else if err == nil {
					defer logger.ClearVar(&result.Item)

					entries := make([]NZB, 0, len((result.Item)))
					defer logger.ClearVar(&entries)
					var newEntry NZB
					defer logger.ClearVar(&newEntry)
					for idx := range result.Item {
						if len(result.Item[idx].Enclosure.URL) == 0 {
							continue
						}
						newEntry = NZB{}
						if strings.Contains(result.Item[idx].Title, "&") || strings.Contains(result.Item[idx].Title, "%") {
							newEntry.Title = html.UnescapeString(result.Item[idx].Title)
						} else {
							if strings.Contains(result.Item[idx].Title, "\\u") {
								var err error
								newEntry.Title, err = strconv.Unquote("\"" + result.Item[idx].Title + "\"")
								if err != nil {
									newEntry.Title = result.Item[idx].Title
								}
							} else {
								newEntry.Title = result.Item[idx].Title
							}
						}
						if strings.Contains(result.Item[idx].Enclosure.URL, "&amp") || strings.Contains(result.Item[idx].Enclosure.URL, "%") {
							newEntry.DownloadURL = html.UnescapeString(result.Item[idx].Enclosure.URL)
						} else {
							newEntry.DownloadURL = result.Item[idx].Enclosure.URL
						}
						newEntry.SourceEndpoint = c.ApiBaseURL
						newEntry.IsTorrent = false
						if strings.Contains(result.Item[idx].Enclosure.URL, ".torrent") || strings.Contains(result.Item[idx].Enclosure.URL, "magnet:?") {
							newEntry.IsTorrent = true
						}

						for idx2 := range result.Item[idx].Attributes {
							saveAttributes(&newEntry, result.Item[idx].Attributes[idx2].Name, result.Item[idx].Attributes[idx2].Value)
						}
						for idx2 := range result.Item[idx].Attributes2 {
							saveAttributes(&newEntry, result.Item[idx].Attributes[idx2].Name, result.Item[idx].Attributes[idx2].Value)
						}
						if newEntry.Size == 0 && result.Item[idx].Size != 0 {
							newEntry.Size = result.Item[idx].Size
						}
						if newEntry.ID == "" && result.Item[idx].GUID.GUID != "" {
							newEntry.ID = result.Item[idx].GUID.GUID
						} else if newEntry.ID == "" {
							newEntry.ID = result.Item[idx].Enclosure.URL
						}
						entries = append(entries, newEntry)
						if tillid == newEntry.ID && tillid != "" {
							breakatid = true
							break
						}
					}
					return entries, breakatid, nil
				}
			}
		}

		return nil, false, nil
	} else {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, false, err
		}
		var feed SearchResponse
		defer logger.ClearVar(&feed)
		err = c.Client.DoXml(req, &feed)
		if err != nil {
			if err == errors.New("429") {
				config.Slepping(false, 60)
				err = c.Client.DoXml(req, &feed)
				if err != nil {
					return nil, false, err
				}
			}
			if err == errors.New("please wait") {
				config.Slepping(false, 10)
				err = c.Client.DoXml(req, &feed)
				if err != nil {
					return nil, false, err
				}
			}
		}
		if c.Debug {
			logger.Log.Debug("url: ", url, " results ", len(feed.NZBs))
		}
		entries := make([]NZB, 0, len((feed.NZBs)))
		defer logger.ClearVar(&entries)
		var newEntry NZB
		defer logger.ClearVar(&newEntry)
		for idx := range feed.NZBs {
			newEntry = NZB{}
			if strings.Contains(feed.NZBs[idx].Title, "&") || strings.Contains(feed.NZBs[idx].Title, "%") {
				newEntry.Title = html.UnescapeString(feed.NZBs[idx].Title)
			} else {
				var err error
				if strings.Contains(feed.NZBs[idx].Title, "\\u") {
					newEntry.Title, err = strconv.Unquote("\"" + feed.NZBs[idx].Title + "\"")
					if err != nil {
						newEntry.Title = feed.NZBs[idx].Title
					}
				} else {
					newEntry.Title = feed.NZBs[idx].Title
				}
			}
			if strings.Contains(feed.NZBs[idx].Enclosure.URL, "&amp") || strings.Contains(feed.NZBs[idx].Enclosure.URL, "%") {
				newEntry.DownloadURL = html.UnescapeString(feed.NZBs[idx].Enclosure.URL)
			} else {
				newEntry.DownloadURL = feed.NZBs[idx].Enclosure.URL
			}
			newEntry.SourceEndpoint = c.ApiBaseURL
			newEntry.IsTorrent = false
			if strings.Contains(feed.NZBs[idx].Enclosure.URL, ".torrent") || strings.Contains(feed.NZBs[idx].Enclosure.URL, "magnet:?") {
				newEntry.IsTorrent = true
			}

			for idx2 := range feed.NZBs[idx].Attributes {
				saveAttributes(&newEntry, feed.NZBs[idx].Attributes[idx2].Name, feed.NZBs[idx].Attributes[idx2].Value)
			}
			if newEntry.Size == 0 && feed.NZBs[idx].Size != 0 {
				newEntry.Size = feed.NZBs[idx].Size
			}
			if newEntry.ID == "" && feed.NZBs[idx].GUID.GUID != "" {
				newEntry.ID = feed.NZBs[idx].GUID.GUID
			} else if newEntry.ID == "" {
				newEntry.ID = feed.NZBs[idx].Source.URL
			}
			entries = append(entries, newEntry)
			if tillid == newEntry.ID && tillid != "" {
				breakatid = true
				break
			}
		}
		return entries, breakatid, nil
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

// parseDate attempts to parse a date string
func parseDate(date string) (time.Time, error) {
	formats := []string{time.RFC3339, time.RFC1123Z}
	for idx := range formats {
		if parsedTime, err := time.Parse(formats[idx], date); err == nil {
			return parsedTime, nil
		}
	}
	return time.Time{}, errors.New("failed to parse date as one of " + date + " " + strings.Join(formats, ", "))
}

// NZB represents an NZB found on the index
type NZB struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
	//Description string    `json:"description,omitempty"`
	Size int64 `json:"size,omitempty"`
	//AirDate     time.Time `json:"air_date,omitempty"`
	//PubDate time.Time `json:"pub_date,omitempty"`
	//UsenetDate  time.Time `json:"usenet_date,omitempty"`
	//NumGrabs    int       `json:"num_grabs,omitempty"`

	SourceEndpoint string `json:"source_endpoint"`
	//SourceAPIKey   string `json:"source_apikey"`

	//Category []string `json:"category,omitempty"`
	//Info     string   `json:"info,omitempty"`
	//Genre    string   `json:"genre,omitempty"`

	//Resolution string `json:"resolution,omitempty"`
	//Poster     string `json:"poster,omitempty"`
	//Group      string `json:"group,omitempty"`

	// TV Specific stuff
	TVDBID  string `json:"tvdbid,omitempty"`
	Season  string `json:"season,omitempty"`
	Episode string `json:"episode,omitempty"`
	//TVTitle string `json:"tvtitle,omitempty"`
	//Rating  int    `json:"rating,omitempty"`

	// Movie Specific stuff
	IMDBID string `json:"imdb,omitempty"`
	//IMDBTitle string  `json:"imdbtitle,omitempty"`
	//IMDBYear  int     `json:"imdbyear,omitempty"`
	//IMDBScore float32 `json:"imdbscore,omitempty"`
	//CoverURL  string  `json:"coverurl,omitempty"`

	// Torznab specific stuff
	//Seeders     int    `json:"seeders,omitempty"`
	//Peers       int    `json:"peers,omitempty"`
	//InfoHash    string `json:"infohash,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	IsTorrent   bool   `json:"is_torrent,omitempty"`
}

// SearchResponse is a RSS version of the response.
type SearchResponse struct {
	NZBs []RawNZB `xml:"channel>item"`
}

type SearchResponseJson1 struct {
	Title   string `json:"title,omitempty"`
	Channel struct {
		Item []RawNZBJson1 `json:"item"`
	} `json:"channel"`
}
type SearchResponseJson2 struct {
	Item []RawNZBJson2 `json:"item"`
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
