package apiexternal

import (
	"net/http"
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/time/rate"
)

type omDBMovie struct {
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
	OmdbType   string `json:"Type"`
	DVD        string `json:"DVD"`
	Plot       string `json:"Plot"`
	BoxOffice  string `json:"BoxOffice"`
	Production string `json:"Production"`
	Website    string `json:"Website"`
}

type omDBMovieSearch struct {
	Title    string `json:"Title"`
	Year     string `json:"Year"`
	ImdbID   string `json:"imdbID"`
	OmdbType string `json:"Type"`
	Poster   string `json:"Poster"`
}
type OmDBMovieSearchGlobal struct {
	Search       []omDBMovieSearch `json:"Search"`
	TotalResults int               `json:"TotalResults"`
	Response     bool              `json:"Reponse"`
}

type omdbClient struct {
	OmdbApiKey string
	Client     *RLHTTPClient
}

var OmdbApi *omdbClient

func NewOmdbClient(apikey string, seconds int, calls int, disabletls bool) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	OmdbApi = &omdbClient{
		OmdbApiKey: apikey,
		Client: NewClient(
			disabletls,
			rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls),
			slidingwindow.NewLimiterNoStop(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() }))}
}

func (o *omdbClient) GetMovie(imdbid string) (omDBMovie, error) {
	req, err := http.NewRequest("GET", "http://www.omdbapi.com/?i="+imdbid+"&apikey="+o.OmdbApiKey, nil)
	if err != nil {
		return omDBMovie{}, err
	}
	var result omDBMovie
	err = o.Client.DoJson(req, &result)

	if err != nil {
		return omDBMovie{}, err
	}
	return result, nil
}

func (o *omdbClient) SearchMovie(title string, year string) (OmDBMovieSearchGlobal, error) {
	url := "http://www.omdbapi.com/?s=" + url.PathEscape(title)
	if year != "" && year != "0" {
		url = url + "&y=" + year
	}
	url = url + "&apikey=" + o.OmdbApiKey

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return OmDBMovieSearchGlobal{}, err
	}
	var result OmDBMovieSearchGlobal

	err = o.Client.DoJson(req, &result)

	if err != nil {
		return OmDBMovieSearchGlobal{}, err
	}
	return result, nil
}
