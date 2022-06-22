package apiexternal

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/time/rate"
)

type theTVDBSeries struct {
	Data struct {
		ID              int      `json:"id"`
		SeriesID        int      `json:"seriesId"`
		SeriesName      string   `json:"seriesName"`
		Aliases         []string `json:"aliases"`
		Season          string   `json:"season"`
		Status          string   `json:"status"`
		FirstAired      string   `json:"firstAired"`
		Network         string   `json:"network"`
		NetworkID       string   `json:"networkId"`
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
	} `json:"data"`
}

type theTVDBEpisodes struct {
	Links struct {
		First int `json:"first"`
		Last  int `json:"last"`
	} `json:"links"`
	Data []struct {
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
	} `json:"data"`
}

type tvdbClient struct {
	Client *RLHTTPClient
}

var TvdbApi *tvdbClient

func NewTvdbClient(seconds int, calls int, disabletls bool) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TvdbApi = &tvdbClient{
		Client: NewClient(
			disabletls,
			rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls),
			slidingwindow.NewLimiterNoStop(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() }))}

}

func (t *tvdbClient) GetSeries(id int, language string) (theTVDBSeries, error) {
	req, err := http.NewRequest("GET", "https://api.thetvdb.com/series/"+strconv.Itoa(id), nil)
	if err != nil {
		return theTVDBSeries{}, err
	}

	if len(language) >= 1 {
		req.Header.Add("Accept-Language", language)
	}

	var result theTVDBSeries
	err = t.Client.DoJson(req, &result)

	if err != nil {
		return theTVDBSeries{}, err
	}
	req = nil
	return result, nil
}
func (t *tvdbClient) GetSeriesEpisodes(id int, language string) (theTVDBEpisodes, error) {
	req, err := http.NewRequest("GET", "https://api.thetvdb.com/series/"+strconv.Itoa(id)+"/episodes", nil)
	if err != nil {
		return theTVDBEpisodes{}, err
	}

	if len(language) >= 1 {
		req.Header.Add("Accept-Language", language)
	}
	var result theTVDBEpisodes
	err = t.Client.DoJson(req, &result)

	if err != nil {
		return theTVDBEpisodes{}, err
	}

	if result.Links.Last >= 2 {
		k := 2

		var resultadd theTVDBEpisodes
		for ; k <= result.Links.Last; k++ {
			req, err = http.NewRequest("GET", "https://api.thetvdb.com/series/"+strconv.Itoa(id)+"/episodes?page="+strconv.Itoa(k), nil)
			if err != nil {
				continue
			}
			if len(language) >= 1 {
				req.Header.Add("Accept-Language", language)
			}

			t.Client.DoJson(req, &resultadd)
			result.Data = append(result.Data, resultadd.Data...)
		}
	}
	req = nil
	return result, nil
}
