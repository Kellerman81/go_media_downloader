package apiexternal

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
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

var OmdbAPI *omdbClient

func (t *OmDBMovieSearchGlobal) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.Search)
	logger.ClearVar(t)
}
func NewOmdbClient(apikey string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	OmdbAPI = &omdbClient{
		OmdbAPIKey: apikey,
		Client: NewClient(
			disabletls,
			true,
			slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
			false, slidingwindow.NewLimiter(10*time.Second, 10), timeoutseconds)}
}

func (o *omdbClient) GetMovie(imdbid string) (*OmDBMovie, error) {
	if imdbid == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[OmDBMovie](o.Client, "http://www.omdbapi.com/?i="+imdbid+"&apikey="+o.OmdbAPIKey)
}

func (o *omdbClient) SearchMovie(title *string, year string) (*OmDBMovieSearchGlobal, error) {
	if *title == "" {
		return nil, logger.ErrNotFound
	}
	var yearstr string
	if year == "0" {
		year = ""
	}
	if year != "" {
		yearstr = "&y=" + year
	}
	return DoJSONType[OmDBMovieSearchGlobal](o.Client, "http://www.omdbapi.com/?s="+QueryEscape(title)+yearstr+"&apikey="+o.OmdbAPIKey)
}
