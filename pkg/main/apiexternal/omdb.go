package apiexternal

import (
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

type omDBMovie struct {
	Title string `json:"Title"`
	Year  string `json:"Year"`
	//Rated    string `json:"Rated"`
	//Released string `json:"Released"`
	Genre    string `json:"Genre"`
	Language string `json:"Language"`
	Country  string `json:"Country"`
	//Awards     string `json:"Awards"`
	//Metascore  string `json:"Metascore"`
	ImdbRating string `json:"imdbRating"`
	ImdbVotes  string `json:"imdbVotes"`
	ImdbID     string `json:"imdbID"`
	//OmdbType   string `json:"Type"`
	//DVD        string `json:"DVD"`
	Plot string `json:"Plot"`
	//BoxOffice  string `json:"BoxOffice"`
	//Production string `json:"Production"`
	Website string `json:"Website"`
}

type omDBMovieSearch struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	//OmdbType string `json:"Type"`
	//Poster   string `json:"Poster"`
}
type omDBMovieSearchGlobal struct {
	Search []omDBMovieSearch `json:"Search"`
	//TotalResults int               `json:"TotalResults"`
	//Response     bool              `json:"Reponse"`
}

// omdbClient is a struct for interacting with the OMDb API.
// It contains fields for the API key, query parameter API key,
// and a pointer to the rate limited HTTP client.
type omdbClient struct {
	OmdbAPIKey string       // The OMDb API key
	QAPIKey    string       // The query parameter API key
	Client     rlHTTPClient // Pointer to the rate limited HTTP client
}

// Close releases the resources used by the OmDBMovieSearchGlobal struct.
// It sets the Search field to nil and calls logger.Clear on the struct to
// release any accumulated log messages. This should be called when the
// struct is no longer needed.
func (t *omDBMovieSearchGlobal) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	//clear(t.Search)
	t.Search = nil
	*t = omDBMovieSearchGlobal{}
}

// NewOmdbClient creates a new omdbClient instance for making requests to the
// OMDb API. It takes the API key, rate limit seconds, rate limit calls per
// second, whether to disable TLS, and request timeout in seconds.
// It sets sane defaults for the rate limiting if 0 values are passed.
func NewOmdbClient(apikey string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	omdbAPI = omdbClient{
		OmdbAPIKey: apikey,
		QAPIKey:    "&apikey=" + apikey,
		Client: NewClient(
			"omdb",
			disabletls,
			true,
			slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
			false, slidingwindow.NewLimiter(10*time.Second, 10), timeoutseconds)}
}

// Close cleans up the OmDBMovie instance by clearing its logger.
// This is called automatically when the instance is no longer referenced.
func (s *omDBMovie) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || s == nil {
		return
	}
	*s = omDBMovie{}
}

// GetOmdbMovie retrieves movie details from the OMDb API by imdbid.
// It returns a pointer to an OmDBMovie struct and an error.
// The imdbid parameter specifies the imdbid to look up.
// It returns logger.ErrNotFound if the imdbid is empty.
func GetOmdbMovie(imdbid string) (omDBMovie, error) {
	if imdbid == "" || omdbAPI.Client.checklimiterwithdaily() {
		return omDBMovie{}, logger.ErrNotFound
	}
	//return DoJSONType[omDBMovie](omdbAPI.Client, logger.JoinStrings("http://www.omdbapi.com/?i=", imdbid, omdbAPI.QAPIKey), nil)
	return DoJSONType[omDBMovie](&omdbAPI.Client, logger.JoinStrings("http://www.omdbapi.com/?i=", imdbid, omdbAPI.QAPIKey))
}

// SearchOmdbMovie searches the OMDb API for movies matching the given title and release year.
// The title parameter specifies the movie title to search for.
// The year parameter optionally specifies a release year to filter by.
// It returns a pointer to an OmDBMovieSearchGlobal struct containing search results,
// and an error.
func SearchOmdbMovie(title string, year string) ([]omDBMovieSearch, error) {
	if title == "" || omdbAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	if year != "" && year != "0" {
		year = "&y=" + year
	}
	//return DoJSONType[omDBMovieSearchGlobal](omdbAPI.Client, logger.JoinStrings("http://www.omdbapi.com/?s=", url.QueryEscape(title), yearstr, omdbAPI.QAPIKey), nil)
	arr, err := DoJSONType[omDBMovieSearchGlobal](&omdbAPI.Client, logger.JoinStrings("http://www.omdbapi.com/?s=", url.QueryEscape(title), year, omdbAPI.QAPIKey))
	return arr.Search, err
}
