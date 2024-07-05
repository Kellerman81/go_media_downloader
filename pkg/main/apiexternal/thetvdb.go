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
	Data TheTVDBSeriesData `json:"data"`
}
type TheTVDBSeriesData struct {
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

type TheTVDBEpisodes struct {
	Links TheTVDBEpisodesLinks `json:"links"`
	Data  []TheTVDBEpisode     `json:"data"`
}
type TheTVDBEpisodesLinks struct {
	First int `json:"first"`
	Last  int `json:"last"`
	Next  int `json:"next"`
}

type TheTVDBEpisode struct {
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
	tvdbApidata = apidata{
		disabletls:     disabletls,
		seconds:        seconds,
		calls:          calls,
		timeoutseconds: timeoutseconds,
		limiter:        slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		dailylimiter:   slidingwindow.NewLimiter(10*time.Second, 10),
	}
}

// GetTvdbSeries retrieves TV series data from the TheTVDB API for the given series ID.
// If a non-empty language is provided, it will be set in the API request headers.
// Returns the TV series data, or an error if one occurs.
func GetTvdbSeries(id int, language string) (TheTVDBSeries, error) {
	p := pltvdb.Get()
	defer pltvdb.Put(p)
	if id == 0 || p.Client.checklimiterwithdaily() {
		return TheTVDBSeries{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings("https://api.thetvdb.com/series/", strconv.Itoa(id)) //JoinStrings
	if language != "" {
		return doJSONTypeHeader[TheTVDBSeries](&p.Client, urlv, []string{"Accept-Language", language})
	}
	return doJSONType[TheTVDBSeries](&p.Client, urlv)
}

// GetTvdbSeriesEpisodes retrieves all episodes for the given TV series ID from
// TheTVDB API. It accepts the series ID, preferred language, and database series
// ID. It retrieves the episode data, checks for existing episodes to avoid
// duplicates, and inserts any missing episodes into the database. If there are
// multiple pages of results, it fetches additional pages.
func UpdateTvdbSeriesEpisodes(id int, language string, dbid *uint) {
	p := pltvdb.Get()
	defer pltvdb.Put(p)
	if id == 0 || p.Client.checklimiterwithdaily() {
		return
	}
	urlv := logger.JoinStrings("https://api.thetvdb.com/series/", strconv.Itoa(id), "/episodes")
	var result TheTVDBEpisodes
	//defer result.Close()
	var err error
	var lang []string
	if language != "" {
		lang = []string{"Accept-Language", language}
		result, err = doJSONTypeHeader[TheTVDBEpisodes](&p.Client, urlv, lang)
	} else {
		result, err = doJSONType[TheTVDBEpisodes](&p.Client, urlv)
	}

	if err != nil {
		if !errors.Is(err, logger.ErrToWait) {
			logger.LogDynamicany("error", "Error calling", err, &logger.StrURL, &urlv) //logpointer
		}
		return
	}
	tbl := database.Getrows1size[database.DbstaticTwoString](false, database.QueryDbserieEpisodesCountByDBID, database.QueryDbserieEpisodesGetSeasonEpisodeByDBID, dbid)

	result.addthetvdbepisodes(dbid, tbl)
	urlv = logger.JoinStrings(urlv, "?page=")
	if result.Links.Next > 0 && (result.Links.First+1) < result.Links.Last {
		var resultadd TheTVDBEpisodes
		for k := result.Links.First + 1; k <= result.Links.Last; k++ {
			if language != "" {
				resultadd, err = doJSONTypeHeader[TheTVDBEpisodes](&p.Client, logger.JoinStrings(urlv, strconv.Itoa(k)), lang)
			} else {
				resultadd, err = doJSONType[TheTVDBEpisodes](&p.Client, logger.JoinStrings(urlv, strconv.Itoa(k)))
			}
			if err == nil {
				resultadd.addthetvdbepisodes(dbid, tbl)
				clear(resultadd.Data)
			}
		}
	}
	//clear(result.Data)
	//clear(tbl)
}

// addthetvdbepisodes iterates through the episodes in the given TheTVDBEpisodes
// result and inserts any missing episodes into the dbserie_episodes table for
// the series matching the given dbid. It returns false if no error occurs.
func (t *TheTVDBEpisodes) addthetvdbepisodes(dbid *uint, tbl []database.DbstaticTwoString) {
	for idx := range t.Data {
		if checkdbtwostrings(tbl, t.Data[idx].AiredSeason, t.Data[idx].AiredEpisodeNumber) {
			continue
		}
		database.ExecN("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			strconv.Itoa(t.Data[idx].AiredEpisodeNumber), strconv.Itoa(t.Data[idx].AiredSeason), generateIdentifierStringFromInt(t.Data[idx].AiredSeason, t.Data[idx].AiredEpisodeNumber), &t.Data[idx].EpisodeName, parseDateTime(t.Data[idx].FirstAired), &t.Data[idx].Overview, &t.Data[idx].Poster, dbid)
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
