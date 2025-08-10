package apiexternal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

type OmDBMovie struct {
	Title string `json:"Title"`
	Year  string `json:"Year"`
	// Rated    string `json:"Rated"`
	// Released string `json:"Released"`
	Genre    string `json:"Genre"`
	Language string `json:"Language"`
	Country  string `json:"Country"`
	// Awards     string `json:"Awards"`
	// Metascore  string `json:"Metascore"`
	ImdbRating string `json:"imdbRating"`
	ImdbVotes  string `json:"imdbVotes"`
	ImdbID     string `json:"imdbID"`
	// OmdbType   string `json:"Type"`
	// DVD        string `json:"DVD"`
	Plot string `json:"Plot"`
	// BoxOffice  string `json:"BoxOffice"`
	// Production string `json:"Production"`
	Website string `json:"Website"`
	Runtime string `json:"Runtime"`
	// Director  string `json:"Director"`
	// Writer string `json:"Writer"`
	// Actors string `json:"Actors"`
	// Poster string `json:"Poster"`
}

type OmDBMovieSearchGlobal struct {
	Search []struct {
		Title  string `json:"Title"`
		Year   string `json:"Year"`
		ImdbID string `json:"imdbID"`
		// OmdbType string `json:"Type"`
		// Poster   string `json:"Poster"`
	} `json:"Search"`
	// TotalResults int               `json:"TotalResults"`
	// Response     bool              `json:"Response"`
}

// omdbClient is a struct for interacting with the OMDb API.
// It contains fields for the API key, query parameter API key,
// and a pointer to the rate limited HTTP client.
type omdbClient struct {
	Client     rlHTTPClient // Pointer to the rate limited HTTP client
	Lim        slidingwindow.Limiter
	OmdbAPIKey string // The OMDb API key
	QAPIKey    string // The query parameter API key
}

// NewOmdbClient creates a new omdbClient instance for making requests to the
// OMDb API. It takes the API key, rate limit seconds, rate limit calls per
// second, whether to disable TLS, and request timeout in seconds.
// It sets sane defaults for the rate limiting if 0 values are passed.
func NewOmdbClient(
	apikey string,
	seconds uint8,
	calls int,
	disabletls bool,
	timeoutseconds uint16,
) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	omdbAPI = omdbClient{
		OmdbAPIKey: apikey,
		QAPIKey:    "&apikey=" + apikey,
		Lim:        slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		Client: newClient(
			"omdb",
			disabletls,
			true,
			&omdbAPI.Lim,
			false, nil, timeoutseconds),
	}
}

// GetOmdbMovie retrieves movie details from the OMDb API by imdbid.
// It returns a pointer to an OmDBMovie struct and an error.
// The imdbid parameter specifies the imdbid to look up.
// It returns logger.ErrNotFound if the imdbid is empty.
func GetOmdbMovie(imdbid string) (*OmDBMovie, error) {
	if imdbid == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[OmDBMovie](
		&omdbAPI.Client,
		logger.JoinStrings("http://www.omdbapi.com/?i=", imdbid, omdbAPI.QAPIKey),
		nil,
	)
}

// SearchOmdbMovie searches the OMDb API for movies matching the given title and release year.
// The title parameter specifies the movie title to search for.
// The year parameter optionally specifies a release year to filter by.
// It returns a pointer to an OmDBMovieSearchGlobal struct containing search results,
// and an error.
func SearchOmdbMovie(title, yearin string) (*OmDBMovieSearchGlobal, error) {
	if title == "" {
		return nil, logger.ErrNotFound
	}
	if yearin != "" && yearin != "0" {
		return doJSONTypeP[OmDBMovieSearchGlobal](
			&omdbAPI.Client,
			logger.JoinStrings(
				"http://www.omdbapi.com/?s=",
				url.QueryEscape(title),
				"&y=",
				yearin,
				omdbAPI.QAPIKey,
			),
			nil,
		)
	}
	return doJSONTypeP[OmDBMovieSearchGlobal](
		&omdbAPI.Client,
		logger.JoinStrings("http://www.omdbapi.com/?s=", url.QueryEscape(title), omdbAPI.QAPIKey),
		nil,
	)
}

// TestOMDBConnectivity tests the connectivity to the OMDB API
// Returns status code and error if any
func TestOMDBConnectivity(timeout time.Duration) (int, error) {
	// Check if client is initialized
	if omdbAPI.OmdbAPIKey == "" {
		return 0, fmt.Errorf("OMDB API client not initialized or missing API key")
	}

	statusCode := 0
	err := ProcessHTTPNoRateCheck(
		&omdbAPI.Client,
		"http://www.omdbapi.com/?s=test",
		func(ctx context.Context, resp *http.Response) error {
			statusCode = resp.StatusCode
			return nil
		},
		nil,
	)
	return statusCode, err
}
