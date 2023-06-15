package apiexternal

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
)

type TheTVDBSeries struct {
	Data struct {
		ID              int         `json:"id"`
		SeriesID        interface{} `json:"seriesId"`
		SeriesName      string      `json:"seriesName"`
		Aliases         []string    `json:"aliases"`
		Season          string      `json:"season"`
		Status          string      `json:"status"`
		FirstAired      string      `json:"firstAired"`
		Network         string      `json:"network"`
		NetworkID       string      `json:"networkId"`
		Runtime         string      `json:"runtime"`
		Language        string      `json:"language"`
		Genre           []string    `json:"genre"`
		Overview        string      `json:"overview"`
		Rating          string      `json:"rating"`
		ImdbID          string      `json:"imdbId"`
		SiteRating      float32     `json:"siteRating"`
		SiteRatingCount int         `json:"siteRatingCount"`
		Slug            string      `json:"slug"`
		Banner          string      `json:"banner"`
		Poster          string      `json:"poster"`
		Fanart          string      `json:"fanart"`
	} `json:"data"`
}

type TheTVDBEpisodes struct {
	Links struct {
		First int `json:"first"`
		Last  int `json:"last"`
	} `json:"links"`
	Data []TheTVDBEpisode `json:"data"`
}

type TheTVDBEpisode struct {
	ID                 int    `json:"id"`
	AiredSeason        int    `json:"airedSeason"`
	AiredEpisodeNumber int    `json:"airedEpisodeNumber"`
	EpisodeName        string `json:"episodeName"`
	FirstAired         string `json:"firstAired"`
	Overview           string `json:"overview"`
	Language           struct {
		EpisodeName string `json:"episodeName"`
		Overview    string `json:"overview"`
	} `json:"language"`
	ProductionCode  string  `json:"productionCode"`
	ShowURL         string  `json:"showUrl"`
	SeriesID        int     `json:"seriesId"`
	ImdbID          string  `json:"imdbId"`
	ContentRating   string  `json:"contentRating"`
	SiteRating      float32 `json:"siteRating"`
	SiteRatingCount int     `json:"siteRatingCount"`
	IsMovie         int     `json:"isMovie"`
	Poster          string  `json:"filename"`
}

type tvdbClient struct {
	Client *RLHTTPClient
}

var TvdbAPI *tvdbClient

func (t *TheTVDBSeries) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.Data.Genre)
	logger.Clear(&t.Data.Aliases)
	logger.ClearVar(t)
}
func (t *TheTVDBEpisodes) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.Data)
	logger.ClearVar(t)
}
func NewTvdbClient(seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TvdbAPI = &tvdbClient{
		Client: NewClient(
			disabletls,
			true,
			slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
			false, slidingwindow.NewLimiter(10*time.Second, 10), timeoutseconds)}

}

func (t *tvdbClient) GetSeries(id int, language string) (*TheTVDBSeries, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	var add []addHeader
	if language != "" {
		add = append(add, addHeader{key: "Accept-Language", val: language})
	}
	return DoJSONType[TheTVDBSeries](t.Client, "https://api.thetvdb.com/series/"+logger.IntToString(id), add...)
}
func (t *tvdbClient) GetSeriesEpisodes(id int, language string) (*TheTVDBEpisodes, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	urlv := "https://api.thetvdb.com/series/" + logger.IntToString(id) + "/episodes"
	var add []addHeader
	if language != "" {
		add = append(add, addHeader{key: "Accept-Language", val: language})
	}
	result, err := DoJSONType[TheTVDBEpisodes](t.Client, urlv, add...)

	if err != nil {
		if err != logger.ErrToWait {
			logger.Log.Error().Err(err).Str(logger.StrURL, urlv).Msg(errorCalling)
		}
		return nil, err
	}

	if result.Links.Last >= 2 {
		logger.Grow(&result.Data, len(result.Data)*result.Links.Last)
		var resultadd *TheTVDBEpisodes
		for k := 2; k <= result.Links.Last; k++ {
			resultadd, err = DoJSONType[TheTVDBEpisodes](t.Client, urlv+"?page="+logger.IntToString(k), add...)
			if err != nil {
				break
			}
			if len(resultadd.Data) >= 1 {
				result.Data = append(result.Data, resultadd.Data...)
			}
			resultadd.Close()
			//result.Data = append(logger.GrowSliceBy(result.Data, len(resultadd.Data)), resultadd.Data...)

		}
	}
	return result, nil
}
