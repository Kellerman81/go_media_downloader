package apiexternal

import (
	"net/http"
	"strconv"
	"time"

	"github.com/RussellLuo/slidingwindow"
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

var TvdbApi tvdbClient

func NewTvdbClient(seconds int, calls int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	rl := rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls) // 50 request every 10 seconds
	limiter, _ := slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })
	TvdbApi = tvdbClient{Client: NewClient(rl, limiter)}
}

func (t tvdbClient) GetSeries(id int, language string) (theTVDBSeries, error) {
	req, _ := http.NewRequest("GET", "https://api.thetvdb.com/series/"+strconv.Itoa(id), nil)
	if len(language) >= 1 {
		req.Header.Add("Accept-Language", language)
	}

	var result theTVDBSeries
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return theTVDBSeries{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t tvdbClient) GetSeriesEpisodes(id int, language string) (theTVDBEpisodes, error) {
	req, _ := http.NewRequest("GET", "https://api.thetvdb.com/series/"+strconv.Itoa(id)+"/episodes", nil)
	if len(language) >= 1 {
		req.Header.Add("Accept-Language", language)
	}
	var result theTVDBEpisodes
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return theTVDBEpisodes{}, err
	}
	//json.Unmarshal(responseData, &result)
	if result.Links.Last >= 2 {
		k := 2
		for ; k <= result.Links.Last; k++ {
			req, _ := http.NewRequest("GET", "https://api.thetvdb.com/series/"+strconv.Itoa(id)+"/episodes?page="+strconv.Itoa(k), nil)
			if len(language) >= 1 {
				req.Header.Add("Accept-Language", language)
			}

			var resultadd theTVDBEpisodes
			t.Client.DoJson(req, &resultadd)
			result.Data = append(result.Data, resultadd.Data...)
		}
	}
	return result, nil
}
