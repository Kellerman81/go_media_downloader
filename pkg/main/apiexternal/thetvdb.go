package apiexternal

import (
	"strconv"
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

var TvdbApi tvdbClient

func NewTvdbClient(seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TvdbApi = tvdbClient{
		Client: NewClient(
			disabletls,
			rate.New(calls, 0, time.Duration(seconds)*time.Second), timeoutseconds)}

}

func (t *tvdbClient) GetSeries(id int, language string) (TheTVDBSeries, error) {
	url := "https://api.thetvdb.com/series/" + strconv.FormatInt(int64(id), 10)
	var result TheTVDBSeries
	_, err := t.Client.DoJson(url, &result, []AddHeader{{Key: "Accept-Language", Val: language}})

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheTVDBSeries{}, err
	}
	return result, nil
}
func (t *tvdbClient) GetSeriesEpisodes(id int, language string) (*TheTVDBEpisodes, error) {
	url := "https://api.thetvdb.com/series/" + strconv.FormatInt(int64(id), 10) + "/episodes"
	var result TheTVDBEpisodes
	_, err := t.Client.DoJson(url, &result, []AddHeader{{Key: "Accept-Language", Val: language}})

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}

	if result.Links.Last >= 2 {
		var resultadd TheTVDBEpisodes
		for k := 2; k <= result.Links.Last; k++ {
			resultadd = TheTVDBEpisodes{}
			_, err = t.Client.DoJson(url+"?page="+strconv.Itoa(k), &resultadd, []AddHeader{{Key: "Accept-Language", Val: language}})
			if err != nil {
				logger.Log.GlobalLogger.Error("Error calling: " + url + "?page=" + strconv.Itoa(k) + " error: " + err.Error())
				break
			}

			if len(result.Data) >= 1 {
				result.Data = logger.GrowSliceBy(result.Data, len(resultadd.Data))
			}
			result.Data = append(result.Data, resultadd.Data...)
		}
	}
	return &result, nil
}
