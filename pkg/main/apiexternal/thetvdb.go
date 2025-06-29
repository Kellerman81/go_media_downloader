package apiexternal

import (
	"errors"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

type TheTVDBSeries struct {
	Data struct {
		Aliases         []string `json:"aliases"`
		Genre           []string `json:"genre"`
		SeriesName      string   `json:"seriesName"`
		Season          string   `json:"season"`
		Status          string   `json:"status"`
		FirstAired      string   `json:"firstAired"`
		Network         string   `json:"network"`
		Runtime         string   `json:"runtime"`
		Language        string   `json:"language"`
		Overview        string   `json:"overview"`
		Rating          string   `json:"rating"`
		ImdbID          string   `json:"imdbId"`
		Slug            string   `json:"slug"`
		Banner          string   `json:"banner"`
		Poster          string   `json:"poster"`
		Fanart          string   `json:"fanart"`
		SiteRating      float32  `json:"siteRating"`
		ID              int      `json:"id"`
		SiteRatingCount int      `json:"siteRatingCount"`
		// SeriesID        any      `json:"seriesId"`
		// NetworkID       string   `json:"networkId"`
	} `json:"data"`
}

type TheTVDBEpisodes struct {
	Data []struct {
		EpisodeName        string `json:"episodeName"`
		FirstAired         string `json:"firstAired"`
		Overview           string `json:"overview"`
		Poster             string `json:"filename"`
		AiredSeason        int    `json:"airedSeason"`
		AiredEpisodeNumber int    `json:"airedEpisodeNumber"`
		// ID                 int    `json:"id"`
		// Language           TheTVDBEpisodeLanguage `json:"language"`
		// ProductionCode     string                 `json:"productionCode"`
		// ShowURL            string                 `json:"showUrl"`
		// SeriesID           int                    `json:"seriesId"`
		// ImdbID string `json:"imdbId"`
		// ContentRating      string                 `json:"contentRating"`
		// SiteRating         float32                `json:"siteRating"`
		// SiteRatingCount    int                    `json:"siteRatingCount"`
		// IsMovie            int                    `json:"isMovie"`
	} `json:"data"`
	Links struct {
		First int `json:"first"`
		Last  int `json:"last"`
		Next  int `json:"next"`
	} `json:"links"`
}

// type theTVDBEpisodeLanguage struct {
// 	EpisodeName string `json:"episodeName"`
// 	Overview    string `json:"overview"`
// }

// tvdbClient is a struct for interacting with TheTVDB API.
// It contains a field Client which is a pointer to a rate limited HTTP client.
type tvdbClient struct {
	// Client is a pointer to a rate limited HTTP client for making requests.
	Client rlHTTPClient
	Lim    slidingwindow.Limiter
}

// type tvdbHeader struct {
// 	Header map[string][]string
// }

// NewTvdbClient creates a new tvdbClient instance for making requests to
// the TheTVDB API. It configures rate limiting and TLS based on the
// provided parameters.
func NewTvdbClient(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	tvdbAPI = tvdbClient{
		Lim: slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		Client: newClient(
			"tvdb",
			disabletls,
			true,
			&tvdbAPI.Lim,
			false, nil, timeoutseconds),
	}
}

// var plheader = pool.NewPool(100, 5, func(t *tvdbHeader) { *t = tvdbHeader{Header: make(map[string][]string, 5)} }, func(b *tvdbHeader) bool {
// 	for idx := range b.Header {
// 		clear(b.Header[idx])
// 	}
// 	clear(b.Header)
// 	return false
// })

var maptvdblanguageheader = make(map[string]map[string][]string, 5)

// GetTvdbSeries retrieves TV series data from the TheTVDB API for the given series ID.
// If a non-empty language is provided, it will be set in the API request headers.
// Returns the TV series data, or an error if one occurs.
func GetTvdbSeries(id int, language string) (*TheTVDBSeries, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	urlv := logger.JoinStrings("https://api.thetvdb.com/series/", strconv.Itoa(id))
	if language != "" {
		_, ok := maptvdblanguageheader[language]
		if !ok {
			maptvdblanguageheader[language] = map[string][]string{"Accept-Language": {language}}
		}
		return doJSONTypeP[TheTVDBSeries](&tvdbAPI.Client, urlv, maptvdblanguageheader[language])
	}
	return doJSONTypeP[TheTVDBSeries](&tvdbAPI.Client, urlv, nil)
}

// GetTvdbSeriesEpisodes retrieves all episodes for the given TV series ID from
// TheTVDB API. It accepts the series ID, preferred language, and database series
// ID. It retrieves the episode data, checks for existing episodes to avoid
// duplicates, and inserts any missing episodes into the database. If there are
// multiple pages of results, it fetches additional pages.
func UpdateTvdbSeriesEpisodes(id int, language string, dbid *uint) {
	if id == 0 || tvdbAPI.Client.checklimiterwithdaily(tvdbAPI.Client.Ctx) {
		return
	}
	urlv := logger.JoinStrings("https://api.thetvdb.com/series/", strconv.Itoa(id), "/episodes")
	result, err := querytvdb(language, urlv)
	if err != nil {
		if !errors.Is(err, logger.ErrToWait) {
			logger.LogDynamicany1StringErr(
				"error",
				"Error calling",
				err,
				logger.StrURL,
				urlv,
			) // logpointer
		}
		return
	}
	tbl := database.Getrowssize[database.DbstaticTwoString](
		false,
		database.QueryDbserieEpisodesCountByDBID,
		database.QueryDbserieEpisodesGetSeasonEpisodeByDBID,
		dbid,
	)
	result.addthetvdbepisodes(dbid, tbl)
	if result.Links.Next > 0 && (result.Links.First+1) < result.Links.Last {
		for k := result.Links.First + 1; k <= result.Links.Last; k++ {
			resultadd, err := querytvdb(
				language,
				logger.JoinStrings(urlv, "?page=", strconv.Itoa(k)),
			)
			if err == nil {
				resultadd.addthetvdbepisodes(dbid, tbl)
			}
		}
	}
}

// querytvdb queries the TheTVDB API for episode data, using the provided language
// if specified. It returns the episode data or an error if one occurs.
func querytvdb(language, urlv string) (*TheTVDBEpisodes, error) {
	if language != "" {
		_, ok := maptvdblanguageheader[language]
		if !ok {
			maptvdblanguageheader[language] = map[string][]string{"Accept-Language": {language}}
		}
		return doJSONTypeP[TheTVDBEpisodes](&tvdbAPI.Client, urlv, maptvdblanguageheader[language])
	}
	return doJSONTypeP[TheTVDBEpisodes](&tvdbAPI.Client, urlv, nil)
}

// addthetvdbepisodes iterates through the episodes in the given TheTVDBEpisodes
// result and inserts any missing episodes into the dbserie_episodes table for
// the series matching the given dbid. It returns false if no error occurs.
func (t *TheTVDBEpisodes) addthetvdbepisodes(dbid *uint, tbl []database.DbstaticTwoString) {
	for idx := range t.Data {
		if checkdbtwostrings(tbl, t.Data[idx].AiredSeason, t.Data[idx].AiredEpisodeNumber) {
			continue
		}
		epi := strconv.Itoa(t.Data[idx].AiredEpisodeNumber)
		seas := strconv.Itoa(t.Data[idx].AiredSeason)
		ident := generateIdentifierStringFromInt(
			&t.Data[idx].AiredSeason,
			&t.Data[idx].AiredEpisodeNumber,
		)
		aired := parseDateTime(t.Data[idx].FirstAired)
		database.ExecN(
			"insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			&epi,
			&seas,
			&ident,
			&t.Data[idx].EpisodeName,
			&aired,
			&t.Data[idx].Overview,
			&t.Data[idx].Poster,
			dbid,
		)
	}
}

// ParseDate parses a date string in "2006-01-02" format and returns a sql.NullTime.
// Returns a null sql.NullTime if the date string is empty or fails to parse.
func parseDateTime(date string) time.Time {
	if date == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return t
	}
	return time.Time{}
}
