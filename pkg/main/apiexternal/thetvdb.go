package apiexternal

import (
	"errors"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

type theTVDBSeries struct {
	Data theTVDBSeriesData `json:"data"`
}
type theTVDBSeriesData struct {
	ID              int      `json:"id"`
	SeriesName      string   `json:"seriesName"`
	Aliases         []string `json:"aliases"`
	Season          string   `json:"season"`
	Status          string   `json:"status"`
	FirstAired      string   `json:"firstAired"`
	Network         string   `json:"network"`
	Runtime         string   `json:"runtime"`
	Language        string   `json:"language"`
	Genre           []string `json:"genre"`
	Overview        string   `json:"overview"`
	Rating          string   `json:"rating"`
	ImdbID          string   `json:"imdbId"`
	SiteRating      float32  `json:"siteRating"`
	SiteRatingCount int      `json:"siteRatingCount"`
	Slug            string   `json:"slug"`
	Banner          string   `json:"banner"`
	Poster          string   `json:"poster"`
	Fanart          string   `json:"fanart"`
	//SeriesID        any      `json:"seriesId"`
	//NetworkID       string   `json:"networkId"`
}

type theTVDBEpisodes struct {
	Links theTVDBEpisodesLinks `json:"links"`
	Data  []theTVDBEpisode     `json:"data"`
}
type theTVDBEpisodesLinks struct {
	First int `json:"first"`
	Last  int `json:"last"`
}

type theTVDBEpisode struct {
	AiredSeason        int    `json:"airedSeason"`
	AiredEpisodeNumber int    `json:"airedEpisodeNumber"`
	EpisodeName        string `json:"episodeName"`
	FirstAired         string `json:"firstAired"`
	Overview           string `json:"overview"`
	Poster             string `json:"filename"`
	//ID                 int    `json:"id"`
	//Language           TheTVDBEpisodeLanguage `json:"language"`
	//ProductionCode     string                 `json:"productionCode"`
	//ShowURL            string                 `json:"showUrl"`
	//SeriesID           int                    `json:"seriesId"`
	//ImdbID string `json:"imdbId"`
	//ContentRating      string                 `json:"contentRating"`
	//SiteRating         float32                `json:"siteRating"`
	//SiteRatingCount    int                    `json:"siteRatingCount"`
	//IsMovie            int                    `json:"isMovie"`
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
}

// Close cleans up the theTVDBSeries object by setting all fields to their
// zero values. This is done to avoid keeping large objects in memory
// unnecessarily when they are no longer needed. The cleanup is skipped
// if the DisableVariableCleanup setting is true or if t is nil.
func (t *theTVDBSeries) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	//clear(t.Data.Aliases)
	//clear(t.Data.Genre)
	t.Data.Aliases = nil
	t.Data.Genre = nil
	*t = theTVDBSeries{}
}

// Close cleans up the theTVDBEpisodes struct by zeroing it out.
// This is done to avoid keeping large structs in memory when no longer needed.
func (t *theTVDBEpisodes) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	//clear(t.Data)
	t.Data = nil
	*t = theTVDBEpisodes{}
}

// NewTvdbClient creates a new tvdbClient instance for making requests to
// the TheTVDB API. It configures rate limiting and TLS based on the
// provided parameters.
func NewTvdbClient(seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	tvdbAPI = tvdbClient{
		Client: NewClient(
			"tvdb",
			disabletls,
			true,
			slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
			false, slidingwindow.NewLimiter(10*time.Second, 10), timeoutseconds)}
}

// GetTvdbSeries retrieves TV series data from the TheTVDB API for the given series ID.
// If a non-empty language is provided, it will be set in the API request headers.
// Returns the TV series data, or an error if one occurs.
func GetTvdbSeries(id int, language string) (theTVDBSeries, error) {
	if id == 0 || tvdbAPI.Client.checklimiterwithdaily() {
		return theTVDBSeries{}, logger.ErrNotFound
	}
	if language != "" {
		return DoJSONType[theTVDBSeries](&tvdbAPI.Client, logger.JoinStrings("https://api.thetvdb.com/series/", strconv.Itoa(id)), keyval{"Accept-Language", language})
	}
	return DoJSONType[theTVDBSeries](&tvdbAPI.Client, logger.JoinStrings("https://api.thetvdb.com/series/", strconv.Itoa(id)))
}

// GetTvdbSeriesEpisodes retrieves all episodes for the given TV series ID from
// TheTVDB API. It accepts the series ID, preferred language, and database series
// ID. It retrieves the episode data, checks for existing episodes to avoid
// duplicates, and inserts any missing episodes into the database. If there are
// multiple pages of results, it fetches additional pages.
func UpdateTvdbSeriesEpisodes(id int, language string, dbid uint) {
	if id == 0 || tvdbAPI.Client.checklimiterwithdaily() {
		return
	}
	urlv := logger.URLJoinPath("https://api.thetvdb.com/series/", strconv.Itoa(id), "episodes")
	var result theTVDBEpisodes
	defer result.Close()
	var err error
	var lang keyval
	if language != "" {
		lang = keyval{"Accept-Language", language}
		result, err = DoJSONType[theTVDBEpisodes](&tvdbAPI.Client, urlv, lang)
	} else {
		result, err = DoJSONType[theTVDBEpisodes](&tvdbAPI.Client, urlv)
	}

	if err != nil {
		if !errors.Is(err, logger.ErrToWait) {
			logger.LogDynamic("error", "Error calling", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrURL, &urlv))
		}
		return
	}
	tbl := database.Getrows1size[database.DbstaticTwoString](false, database.QueryDbserieEpisodesCountByDBID, database.QueryDbserieEpisodesGetSeasonEpisodeByDBID, &dbid)

	addthetvdbepisodes(&result, &dbid, tbl)
	urlv += "?page="
	if result.Links.Last >= 2 {
		var resultadd theTVDBEpisodes
		for k := 2; k <= result.Links.Last; k++ {
			if language != "" {
				resultadd, err = DoJSONType[theTVDBEpisodes](&tvdbAPI.Client, urlv+strconv.Itoa(k), lang)
			} else {
				resultadd, err = DoJSONType[theTVDBEpisodes](&tvdbAPI.Client, urlv+strconv.Itoa(k))
			}
			if err == nil {
				addthetvdbepisodes(&resultadd, &dbid, tbl)
			}
			resultadd.Close()
		}
	}
	//clear(tbl)
	tbl = nil
}

// addthetvdbepisodes iterates through the episodes in the given TheTVDBEpisodes
// result and inserts any missing episodes into the dbserie_episodes table for
// the series matching the given dbid. It returns false if no error occurs.
func addthetvdbepisodes(resultadd *theTVDBEpisodes, dbid *uint, tbl []database.DbstaticTwoString) bool {
	for idx := range resultadd.Data {
		if checkdbtwostrings(tbl, resultadd.Data[idx].AiredSeason, resultadd.Data[idx].AiredEpisodeNumber) {
			continue
		}
		dt := database.ParseDateTime(resultadd.Data[idx].FirstAired)
		strepisode := strconv.Itoa(resultadd.Data[idx].AiredEpisodeNumber)
		strseason := strconv.Itoa(resultadd.Data[idx].AiredSeason)
		stridentifier := GenerateIdentifierStringFromInt(resultadd.Data[idx].AiredSeason, resultadd.Data[idx].AiredEpisodeNumber)
		database.ExecN("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			&strepisode, &strseason, &stridentifier, &resultadd.Data[idx].EpisodeName, &dt, &resultadd.Data[idx].Overview, &resultadd.Data[idx].Poster, dbid)
	}
	return false
}
