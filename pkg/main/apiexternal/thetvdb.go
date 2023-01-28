package apiexternal

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/rate"
	"go.uber.org/zap"
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

var TvdbAPI tvdbClient

func (t *TheTVDBSeries) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	t.Data.Aliases = nil
	t.Data.Genre = nil
	t = nil
}
func (t *TheTVDBEpisodes) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	t.Data = nil
	t = nil
}
func NewTvdbClient(seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TvdbAPI = tvdbClient{
		Client: NewClient(
			disabletls,
			true,
			rate.New(calls, 0, time.Duration(seconds)*time.Second), timeoutseconds)}

}

func (t *tvdbClient) GetSeries(id int, language string) (*TheTVDBSeries, error) {
	url := "https://api.thetvdb.com/series/" + logger.IntToString(id)
	var result TheTVDBSeries
	_, err := t.Client.DoJSON(url, &result, []addHeader{{key: "Accept-Language", val: language}})

	if err != nil {
		if err != errPleaseWait {
			logger.Log.GlobalLogger.Error(errorCalling, zap.Stringp("url", &url), zap.Error(err))
		}
		result.Close()
		return nil, err
	}
	return &result, nil
}
func (t *tvdbClient) GetSeriesEpisodes(id int, language string) (*TheTVDBEpisodes, error) {
	url := "https://api.thetvdb.com/series/" + logger.IntToString(id) + "/episodes"
	var result TheTVDBEpisodes
	_, err := t.Client.DoJSON(url, &result, []addHeader{{key: "Accept-Language", val: language}})

	if err != nil {
		if err != errPleaseWait {
			logger.Log.GlobalLogger.Error(errorCalling, zap.Stringp("url", &url), zap.Error(err))
		}
		result.Close()
		return nil, err
	}

	if result.Links.Last >= 2 {
		var resultadd TheTVDBEpisodes
		urlbase := url + "?page="
		for k := 2; k <= result.Links.Last; k++ {
			resultadd.Data = []TheTVDBEpisode{}
			_, err = t.Client.DoJSON(urlbase+logger.IntToString(k), &resultadd, []addHeader{{key: "Accept-Language", val: language}})
			if err != nil {
				logger.Log.GlobalLogger.Error(errorCalling, zap.String("Url", urlbase+logger.IntToString(k)), zap.Error(err))
				break
			} else if len(resultadd.Data) >= 1 {
				result.Data = append(logger.GrowSliceBy(result.Data, len(resultadd.Data)), resultadd.Data...)
			}
		}
		resultadd.Close()
	}
	return &result, nil
}
