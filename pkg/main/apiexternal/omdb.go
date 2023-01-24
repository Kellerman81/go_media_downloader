package apiexternal

import (
	"fmt"
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
	OmdbAPIKey string
	Client     *RLHTTPClient
}

var OmdbAPI omdbClient

func (t *OmDBMovieSearchGlobal) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	t.Search = nil
	t = nil
}
func NewOmdbClient(apikey string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	OmdbAPI = omdbClient{
		OmdbAPIKey: apikey,
		Client: NewClient(
			disabletls,
			true,
			rate.New(calls, 0, time.Duration(seconds)*time.Second),
			timeoutseconds)}
}

func (o *omdbClient) GetMovie(imdbid string, result *OmDBMovie) error {
	url := fmt.Sprintf("http://www.omdbapi.com/?i=%s&apikey=%s", imdbid, o.OmdbAPIKey)
	_, err := o.Client.DoJSON(url, result, nil)

	if err != nil {
		if err != errPleaseWait {
			logger.Log.GlobalLogger.Error(errorCalling, zap.Stringp("url", &url), zap.Error(err))
		}
		return err
	}

	return nil
}

func (o *omdbClient) SearchMovie(title string, year string, result *OmDBMovieSearchGlobal) error {
	var yearstr string
	if year != "" && year != "0" {
		yearstr = "&y=" + year
	}
	url := fmt.Sprintf("http://www.omdbapi.com/?s=%s%s&apikey=%s", url.QueryEscape(title), yearstr, o.OmdbAPIKey)

	_, err := o.Client.DoJSON(url, result, nil)

	if err != nil {
		if err != errPleaseWait {
			logger.Log.GlobalLogger.Error(errorCalling, zap.Stringp("url", &url), zap.Error(err))
		}
		result.Close()
		return err
	}

	return nil
}
