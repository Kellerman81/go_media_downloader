package apiexternal

import (
	"context"
	"fmt"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/omdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
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

	// Create v2 provider
	omdbConfig := base.ClientConfig{
		Name:                      "omdb",
		BaseURL:                   "http://www.omdbapi.com",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthAPIKeyURL,
		APIKey:                    apikey,
		APIKeyParam:               "apikey",
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		EnableStats:               true,
		UserAgent:                 "go-media-downloader/2.0",
		DisableTLSVerify:          disabletls,
	}
	if provider := omdb.NewProviderWithConfig(omdbConfig); provider != nil {
		providers.SetOMDB(provider)
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

	// Use v2 provider if available
	if provider := providers.GetOMDB(); provider != nil {
		details, err := provider.GetDetailsByIMDb(context.Background(), imdbid)
		if err != nil {
			return nil, err
		}

		// Convert genres slice to comma-separated string
		genreNames := make([]string, len(details.Genres))
		for i, g := range details.Genres {
			genreNames[i] = g.Name
		}

		genre := ""
		if len(genreNames) > 0 {
			genre = logger.JoinStrings(genreNames...)
		}

		// Convert spoken languages to comma-separated string
		languageNames := make([]string, len(details.SpokenLanguages))
		for i, l := range details.SpokenLanguages {
			languageNames[i] = l.EnglishName
		}

		language := ""
		if len(languageNames) > 0 {
			language = logger.JoinStrings(languageNames...)
		}

		// Convert v2 result to old format
		return &OmDBMovie{
			Title:      details.Title,
			Year:       fmt.Sprintf("%d", details.Year),
			Genre:      genre,
			Language:   language,
			Country:    "", // Not available in v2 MovieDetails
			ImdbRating: fmt.Sprintf("%.1f", details.VoteAverage),
			ImdbVotes:  fmt.Sprintf("%d", details.VoteCount),
			ImdbID:     details.IMDbID,
			Plot:       details.Overview,
			Website:    details.Homepage,
			Runtime:    fmt.Sprintf("%d min", details.Runtime),
		}, nil
	}

	return nil, logger.ErrNotFound
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

	// Use v2 provider if available
	if provider := providers.GetOMDB(); provider != nil {
		year := 0
		if yearin != "" && yearin != "0" {
			if y, err := fmt.Sscanf(yearin, "%d", &year); err == nil && y == 1 {
				// year parsed successfully
			} else {
				year = 0
			}
		}

		results, err := provider.SearchByTitle(context.Background(), title, year, "")
		if err != nil {
			return nil, err
		}

		// Convert v2 results to old format
		search := &OmDBMovieSearchGlobal{
			Search: make([]struct {
				Title  string `json:"Title"`
				Year   string `json:"Year"`
				ImdbID string `json:"imdbID"`
			}, len(results)),
		}
		for i, r := range results {
			search.Search[i].Title = r.Title
			search.Search[i].Year = r.Year
			search.Search[i].ImdbID = r.ImdbID
		}

		return search, nil
	}

	return nil, logger.ErrNotFound
}

// TestOMDBConnectivity tests the connectivity to the OMDB API
// Returns status code and error if any.
func TestOMDBConnectivity(timeout time.Duration) (int, error) {
	// Use v2 provider if available
	if provider := providers.GetOMDB(); provider != nil {
		// Test with a simple search
		_, err := provider.SearchByTitle(context.Background(), "test", 0, "")
		if err != nil {
			return 0, err
		}

		return 200, nil
	}

	return 0, logger.ErrNotFound
}
