package apiexternal

import (
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/rate"
	"go.uber.org/zap"
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
	OmdbType   string `json:"Type"`
	DVD        string `json:"DVD"`
	Plot       string `json:"Plot"`
	BoxOffice  string `json:"BoxOffice"`
	Production string `json:"Production"`
	Website    string `json:"Website"`
}

type OmDBMovieSearch struct {
	Title    string `json:"Title"`
	Year     string `json:"Year"`
	ImdbID   string `json:"imdbID"`
	OmdbType string `json:"Type"`
	Poster   string `json:"Poster"`
}
type OmDBMovieSearchGlobal struct {
	Search       []OmDBMovieSearch `json:"Search"`
	TotalResults int               `json:"TotalResults"`
	Response     bool              `json:"Reponse"`
}

type omdbClient struct {
	OmdbApiKey string
	Client     *RLHTTPClient
}

var OmdbApi omdbClient

func NewOmdbClient(apikey string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	OmdbApi = omdbClient{
		OmdbApiKey: apikey,
		Client: NewClient(
			disabletls,
			rate.New(calls, 0, time.Duration(seconds)*time.Second),
			timeoutseconds)}
}

func (o *omdbClient) GetMovie(imdbid string, result *OmDBMovie) error {
	url := "http://www.omdbapi.com/?i=" + imdbid + "&apikey=" + o.OmdbApiKey
	_, err := o.Client.DoJson(url, result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return err
	}

	return nil
}

func (o *omdbClient) SearchMovie(title string, year string, result *OmDBMovieSearchGlobal) error {
	yearstr := ""
	if year != "" && year != "0" {
		yearstr = "&y=" + year
	}
	url := "http://www.omdbapi.com/?s=" + url.PathEscape(title) + yearstr + "&apikey=" + o.OmdbApiKey

	_, err := o.Client.DoJson(url, result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return err
	}

	return nil
}
