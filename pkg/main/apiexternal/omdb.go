package apiexternal

import (
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

type OmDBMovie struct {
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
	Runtime string `json:"Runtime"`
	//Director  string `json:"Director"`
	//Writer string `json:"Writer"`
	//Actors string `json:"Actors"`
	//Poster string `json:"Poster"`
}

type OmDBMovieSearch struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	//OmdbType string `json:"Type"`
	//Poster   string `json:"Poster"`
}
type OmDBMovieSearchGlobal struct {
	Search []OmDBMovieSearch `json:"Search"`
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

// NewOmdbClient creates a new omdbClient instance for making requests to the
// OMDb API. It takes the API key, rate limit seconds, rate limit calls per
// second, whether to disable TLS, and request timeout in seconds.
// It sets sane defaults for the rate limiting if 0 values are passed.
func NewOmdbClient(apikey string, seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	omdbApidata = apidata{
		apikey:         apikey,
		apikeyq:        "&apikey=" + apikey,
		disabletls:     disabletls,
		seconds:        seconds,
		calls:          calls,
		timeoutseconds: timeoutseconds,
		limiter:        slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		dailylimiter:   slidingwindow.NewLimiter(10*time.Second, 10),
	}
}

// GetOmdbMovie retrieves movie details from the OMDb API by imdbid.
// It returns a pointer to an OmDBMovie struct and an error.
// The imdbid parameter specifies the imdbid to look up.
// It returns logger.ErrNotFound if the imdbid is empty.
func GetOmdbMovie(imdbid string) (OmDBMovie, error) {
	p := plomdb.Get()
	defer plomdb.Put(p)
	if imdbid == "" || p.Client.checklimiterwithdaily() {
		return OmDBMovie{}, logger.ErrNotFound
	}
	return doJSONType[OmDBMovie](&p.Client, logger.JoinStrings("http://www.omdbapi.com/?i=", imdbid, p.QAPIKey))
}

// SearchOmdbMovie searches the OMDb API for movies matching the given title and release year.
// The title parameter specifies the movie title to search for.
// The year parameter optionally specifies a release year to filter by.
// It returns a pointer to an OmDBMovieSearchGlobal struct containing search results,
// and an error.
func SearchOmdbMovie(title string, year string) (OmDBMovieSearchGlobal, error) {
	p := plomdb.Get()
	defer plomdb.Put(p)
	if title == "" || p.Client.checklimiterwithdaily() {
		return OmDBMovieSearchGlobal{}, logger.ErrNotFound
	}
	if year != "" && year != "0" {
		year = logger.JoinStrings("&y=", year)
	}
	return doJSONType[OmDBMovieSearchGlobal](&p.Client, logger.JoinStrings("http://www.omdbapi.com/?s=", url.QueryEscape(title), year, p.QAPIKey))
}
