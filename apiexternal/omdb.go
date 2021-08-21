package apiexternal

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"golang.org/x/time/rate"
)

type OmDBMovie struct {
	Title      string `json:"Title"`
	Year       string `json:"Year"`
	Rated      string `json:"Rated"`
	Released   string `json:"Released"`
	Genre      string `json:"Genre"`
	Language   string `json:"Language"`
	Country    string `json:"Country"`
	Awards     string `json:"Awards"`
	Metascore  string `json:"Metascore"`
	ImdbRating string `json:"imdbRating"`
	ImdbVotes  string `json:"imdbVotes"`
	ImdbID     string `json:"imdbID"`
	Type       string `json:"Type"`
	DVD        string `json:"DVD"`
	Plot       string `json:"Plot"`
	BoxOffice  string `json:"BoxOffice"`
	Production string `json:"Production"`
	Website    string `json:"Website"`
}

type OmDBMovieSearch struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	Type   string `json:"Type"`
	Poster string `json:"Poster"`
}
type OmDBMovieSearchGlobal struct {
	Search       []OmDBMovieSearch `json:"Search"`
	TotalResults int               `json:"TotalResults"`
	Response     bool              `json:"Reponse"`
}

type OmdbClient struct {
	OmdbApiKey string
	Client     *RLHTTPClient
}

var OmdbApi OmdbClient

func NewOmdbClient(apikey string, seconds int, calls int) {
	rl := rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls) // 50 request every 10 seconds
	limiter, _ := slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })
	OmdbApi = OmdbClient{OmdbApiKey: apikey, Client: NewClient(rl, limiter)}
}

func (o OmdbClient) GetMovie(imdbid string) (OmDBMovie, error) {
	req, _ := http.NewRequest("GET", "http://www.omdbapi.com/?i="+imdbid+"&apikey="+o.OmdbApiKey, nil)
	resp, responseData, err := o.Client.Do(req)
	if err != nil {
		return OmDBMovie{}, err
	}
	if resp.StatusCode == 429 {
		return OmDBMovie{}, err
	}
	var result OmDBMovie
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (o OmdbClient) SearchMovie(title string, year string) (OmDBMovieSearchGlobal, error) {
	url := "http://www.omdbapi.com/?s=" + url.PathEscape(title)
	if year != "" && year != "0" {
		url = url + "&y=" + year
	}
	url = url + "&apikey=" + o.OmdbApiKey

	req, _ := http.NewRequest("GET", url, nil)
	resp, responseData, err := o.Client.Do(req)
	if err != nil {
		return OmDBMovieSearchGlobal{}, err
	}
	if resp.StatusCode == 429 {
		return OmDBMovieSearchGlobal{}, err
	}
	var result OmDBMovieSearchGlobal
	json.Unmarshal(responseData, &result)
	return result, nil
}
