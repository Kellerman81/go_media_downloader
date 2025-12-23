package apiexternal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"golang.org/x/oauth2"
)

type TraktMovieAnticipated struct {
	// ListCount int        `json:"list_count"`
	Movie TraktMovie `json:"movie"`
}

type TraktMovieTrending struct {
	// Watchers int        `json:"watchers"`
	Movie TraktMovie `json:"movie"`
}

type TraktUserList struct {
	// Rank      int        `json:"rank"`
	// ID        int        `json:"id"`
	// Notes     string     `json:"notes"`
	Movie     TraktMovie `json:"movie"`
	Serie     TraktSerie `json:"show"`
	TraktType string     `json:"type"`
}

type TraktMovie struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

type TraktSerieTrending struct {
	// Watchers int        `json:"watchers"`
	Serie TraktSerie `json:"show"`
}

type TraktSerieAnticipated struct {
	// ListCount int        `json:"list_count"`
	Serie TraktSerie `json:"show"`
}

type TraktSerieSeason struct {
	Number string `json:"number"`
	// Ids    Ids `json:"ids"`
}

type TraktSerieSeasonEpisodes struct {
	Title      string    `json:"title"`
	Overview   string    `json:"overview"`
	FirstAired time.Time `json:"first_aired"`
	Season     int       `json:"season"`
	Episode    int       `json:"number"`
	Runtime    int       `json:"runtime"`
	// Ids                   Ids       `json:"ids"`
	// EpisodeAbs            int       `json:"number_abs"`
	// Rating                float32   `json:"rating"`
	// Votes                 int       `json:"votes"`
	// Comments              int       `json:"comment_count"`
	// AvailableTranslations []string  `json:"available_translations"`
}

type TraktSerie struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

type TraktSerieData struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Genres     []string  `json:"genres"`
	Title      string    `json:"title"`
	Overview   string    `json:"overview"`
	Network    string    `json:"network"`
	Country    string    `json:"country"`
	Status     string    `json:"status"`
	Language   string    `json:"language"`
	FirstAired time.Time `json:"first_aired"`
	Rating     float32   `json:"rating"`
	Year       int       `json:"year"`
	Runtime    int       `json:"runtime"`
	// AvailableTranslations []string  `json:"available_translations"`
	// Certification         string    `json:"certification"`
	// Trailer               string    `json:"trailer"`
	// Homepage              string    `json:"homepage"`
	// Votes                 int       `json:"votes"`
	// Comments              int       `json:"comment_count"`
	// AiredEpisodes         int       `json:"aired_episodes"`
}

type TraktAlias struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type TraktMovieExtend struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Genres   []string `json:"genres"`
	Title    string   `json:"title"`
	Tagline  string   `json:"tagline"`
	Overview string   `json:"overview"`
	Released string   `json:"released"`
	Status   string   `json:"status"`
	Language string   `json:"language"`
	Rating   float32  `json:"rating"`
	Runtime  int      `json:"runtime"`
	Comments int      `json:"comment_count"`
	Votes    int32    `json:"votes"`
	Year     uint16   `json:"year"`
	// AvailableTranslations []string `json:"available_translations"`
	// Country               string   `json:"country"`
	// Trailer               string   `json:"trailer"`
	// Homepage              string   `json:"homepage"`
	// Certification         string   `json:"certification"`
}

// NewTraktClient initializes a new traktClient instance for making requests to
// the Trakt API. It takes in credentials and rate limiting settings and sets up
// the OAuth2 configuration.
func NewTraktClient(
	clientid, clientsecret string,
	seconds uint8,
	calls int,
	disabletls bool,
	timeoutseconds uint16,
	redirecturl string,
) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	// Create v2 provider
	traktConfig := base.ClientConfig{
		Name:                      "trakt",
		BaseURL:                   "https://api.trakt.tv",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthNone, // Trakt provider handles OAuth internally via EnsureValidToken
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		EnableStats:               true,
		UserAgent:                 "go-media-downloader/2.0",
		DisableTLSVerify:          disabletls,
	}
	if provider := trakt.NewProviderWithConfig(traktConfig, clientid, clientsecret, "http://localhost:9090"); provider != nil {
		providers.SetTrakt(provider)
		logger.Logtype(logger.StatusDebug, 0).Msg("Registered Trakt metadata provider with OAuth2")
	}
}

// GetTraktMoviePopular retrieves a list of popular movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovie structs containing the movie data,
// or nil if there was an error.
func GetTraktMoviePopular(limit *string) []TraktMovie {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		// Convert limit to page number (Trakt v2 uses pagination)
		page := 1

		results, err := provider.GetPopularMovies(context.Background(), page)
		if err == nil && results != nil && len(results.Results) > 0 {
			movies := make([]TraktMovie, len(results.Results))
			for i, r := range results.Results {
				movies[i] = TraktMovie{
					Title: r.Title,
					Year:  r.Year,
				}
				movies[i].IDs.Trakt = r.ID
				movies[i].IDs.Imdb = r.IMDbID
			}

			return movies
		}
	}

	return nil
}

// GetTraktMovieTrending retrieves a list of trending movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieTrending structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieTrending(limit *string) []TraktMovieTrending {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		page := 1

		results, err := provider.GetTrendingMovies(context.Background(), page)
		if err == nil && results != nil && len(results.Results) > 0 {
			movies := make([]TraktMovieTrending, len(results.Results))
			for i, r := range results.Results {
				movies[i] = TraktMovieTrending{
					Movie: TraktMovie{
						Title: r.Title,
						Year:  r.Year,
					},
				}
				movies[i].Movie.IDs.Trakt = r.ID
				movies[i].Movie.IDs.Imdb = r.IMDbID
			}

			return movies
		}
	}

	return nil
}

// GetTraktMovieAnticipated retrieves a list of anticipated movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieAnticipated structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieAnticipated(limit *string) []TraktMovieAnticipated {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		page := 1

		results, err := provider.GetUpcomingMovies(context.Background(), page)
		if err == nil && results != nil && len(results.Results) > 0 {
			movies := make([]TraktMovieAnticipated, len(results.Results))
			for i, r := range results.Results {
				movies[i] = TraktMovieAnticipated{
					Movie: TraktMovie{
						Title: r.Title,
						Year:  r.Year,
					},
				}
				movies[i].Movie.IDs.Trakt = r.ID
				movies[i].Movie.IDs.Imdb = r.IMDbID
			}

			return movies
		}
	}

	return nil
}

// GetTraktMovieAliases retrieves alias data from the Trakt API for the given movie ID.
// It takes a Trakt movie ID string as a parameter.
// Returns a slice of TraktAlias structs containing the alias data,
// or nil if there is an error or no aliases found.
func GetTraktMovieAliases(movieid string) []TraktAlias {
	if movieid == "" {
		return nil
	}

	// Use v2 provider if available and movieid is numeric (Trakt ID)
	if provider := providers.GetTrakt(); provider != nil {
		if id, err := strconv.Atoi(movieid); err == nil {
			titles, err := provider.GetMovieAlternativeTitles(context.Background(), id)
			if err == nil && len(titles) > 0 {
				aliases := make([]TraktAlias, len(titles))
				for i, t := range titles {
					aliases[i] = TraktAlias{
						Title:   t.Title,
						Country: t.ISO3166_1,
					}
				}

				return aliases
			}
		}
	}

	return nil
}

// GetTraktMovie retrieves extended data for a Trakt movie by ID.
// It takes a movie ID string as input.
// Returns a TraktMovieExtend struct containing the movie data,
// or nil and an error if the movie is not found or there is an error fetching data.
func GetTraktMovie(movieid string) (*TraktMovieExtend, error) {
	if movieid == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		var (
			details *apiexternal_v2.MovieDetails
			err     error
		)

		// Try to parse as integer for Trakt ID first

		if id, parseErr := strconv.Atoi(movieid); parseErr == nil {
			details, err = provider.GetMovieByID(context.Background(), id)
		} else {
			// Not a number, check if it's an IMDb ID
			if strings.HasPrefix(movieid, "tt") {
				// It's an IMDb ID, look it up first
				findResult, findErr := provider.FindMovieByIMDbID(context.Background(), movieid)
				if findErr == nil && findResult != nil && len(findResult.MovieResults) > 0 {
					// Get full details using the Trakt ID from search results
					details, err = provider.GetMovieByID(context.Background(), findResult.MovieResults[0].ID)
				} else {
					err = findErr
				}
			} else {
				err = parseErr
			}
		}

		if err == nil && details != nil {
			// Convert MovieDetails to TraktMovieExtend
			movie := &TraktMovieExtend{
				Title:    details.Title,
				Tagline:  details.Tagline,
				Overview: details.Overview,
				Status:   details.Status,
				Language: details.OriginalLanguage,
				Runtime:  details.Runtime,
				Year:     uint16(details.Year),
			}

			movie.IDs.Trakt = details.ID
			movie.IDs.Imdb = details.IMDbID
			movie.Rating = float32(details.VoteAverage)
			movie.Votes = int32(details.VoteCount)

			if !details.ReleaseDate.IsZero() {
				movie.Released = details.ReleaseDate.Format("2006-01-02")
			}

			// Convert genres
			for _, g := range details.Genres {
				movie.Genres = append(movie.Genres, g.Name)
			}

			return movie, nil
		}
	}

	return nil, errors.New("no client")
}

// GetTraktSerie retrieves extended data for a Trakt TV show by its Trakt ID or IMDb ID.
// It takes the show ID as a string parameter (can be Trakt ID or IMDb ID).
// It returns a TraktSerieData struct containing the show data,
// or nil and an error if the show ID is invalid or there was an error retrieving data.
func GetTraktSerie(showid string) (*TraktSerieData, error) {
	if showid == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		var (
			details *apiexternal_v2.SeriesDetails
			err     error
		)

		// Try to parse as integer for Trakt ID first

		if id, parseErr := strconv.Atoi(showid); parseErr == nil {
			details, err = provider.GetSeriesByID(context.Background(), id)
		} else {
			// Not a number, check if it's an IMDb ID
			if strings.HasPrefix(showid, "tt") {
				// It's an IMDb ID, look it up first
				findResult, findErr := provider.FindSeriesByIMDbID(context.Background(), showid)
				if findErr == nil && findResult != nil && len(findResult.TVResults) > 0 {
					// Get full details using the Trakt ID from search results
					details, err = provider.GetSeriesByID(context.Background(), findResult.TVResults[0].ID)
				} else {
					err = findErr
				}
			} else {
				return nil, fmt.Errorf("invalid show ID format: %s", showid)
			}
		}

		if err == nil && details != nil {
			// Convert SeriesDetails to TraktSerieData
			serie := &TraktSerieData{
				Title:    details.Name,
				Overview: details.Overview,
				Network:  "",
				Country:  "",
				Status:   details.Status,
				Language: details.OriginalLanguage,
				Year:     details.FirstAirDate.Year(),
			}

			serie.IDs.Trakt = details.ID
			serie.IDs.Imdb = details.IMDbID
			serie.IDs.Tvdb = details.TVDbID
			serie.Rating = float32(details.VoteAverage)
			serie.FirstAired = details.FirstAirDate

			if len(details.EpisodeRunTime) > 0 {
				serie.Runtime = details.EpisodeRunTime[0]
			}

			// Extract network from Networks array
			if len(details.Networks) > 0 {
				serie.Network = details.Networks[0].Name
			}

			// Convert genres
			for _, g := range details.Genres {
				serie.Genres = append(serie.Genres, g.Name)
			}

			return serie, nil
		}

		if err != nil {
			return nil, err
		}
	}

	return nil, errors.New("no client")
}

// GetTraktSerieAliases retrieves alias data from the Trakt API for the given Dbserie.
// It first checks if there is a Trakt ID available and uses that to retrieve aliases.
// If no Trakt ID, it falls back to using the IMDb ID if available.
// Returns a slice of TraktAlias structs or nil if no aliases found.
func GetTraktSerieAliases(dbserie *database.Dbserie) []TraktAlias {
	// Use v2 provider if available and TraktID is set
	if provider := providers.GetTrakt(); provider != nil && dbserie.TraktID != 0 {
		titles, err := provider.GetSeriesAlternativeTitles(context.Background(), dbserie.TraktID)
		if err == nil && len(titles) > 0 {
			aliases := make([]TraktAlias, len(titles))
			for i, t := range titles {
				aliases[i] = TraktAlias{
					Title:   t.Title,
					Country: t.ISO3166_1,
				}
			}

			return aliases
		}
	}

	return nil
}

// GetTraktSerieSeasons retrieves a list of season numbers for a Trakt TV show by ID.
// It returns a slice of season numbers as strings, or nil if there is an error.
func GetTraktSerieSeasons(showid string) ([]TraktSerieSeason, error) {
	if showid == "" {
		return nil, nil
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		id, err := strconv.Atoi(showid)
		if err != nil {
			return nil, err
		}

		seasons, err := provider.GetAllSeasons(context.Background(), id)
		if err != nil {
			return nil, err
		}

		// Convert to old format
		result := make([]TraktSerieSeason, len(seasons))
		for i, season := range seasons {
			result[i] = TraktSerieSeason{
				Number: strconv.Itoa(season.SeasonNumber),
			}
		}

		return result, nil
	}

	return nil, errors.New("client empty")
}

// GetTraktSerieSeasonsAndEpisodes retrieves all seasons and episodes for the given Trakt show ID from the Trakt API.
// It takes the show ID and database series ID as parameters.
// It queries the local database for existing episodes to avoid duplicates.
// For each season, it calls addtraktdbepisodes to insert any missing episodes into the database.
// Returns nothing.
func UpdateTraktSerieSeasonsAndEpisodes(showid string, id *uint) {
	if showid == "" {
		return
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		seriesID, err := strconv.Atoi(showid)
		if err != nil {
			return
		}

		seasons, err := provider.GetAllSeasons(context.Background(), seriesID)
		if err != nil {
			return
		}

		tbl := database.Getrowssize[database.DbstaticTwoString](
			false,
			database.QueryDbserieEpisodesCountByDBID,
			database.QueryDbserieEpisodesGetSeasonEpisodeByDBID,
			id,
		)

		for _, season := range seasons {
			episodes, err := provider.GetSeasonEpisodes(
				context.Background(),
				seriesID,
				season.SeasonNumber,
			)
			if err != nil {
				continue
			}

			for _, ep := range episodes {
				if checkdbtwostrings(tbl, ep.SeasonNumber, ep.EpisodeNumber) {
					continue
				}

				epi := strconv.Itoa(ep.EpisodeNumber)
				seas := strconv.Itoa(ep.SeasonNumber)
				ident := generateIdentifierStringFromInt(&ep.SeasonNumber, &ep.EpisodeNumber)
				database.ExecN(
					"insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
					&epi,
					&seas,
					&ident,
					&ep.Name,
					&ep.AirDate,
					&ep.Overview,
					id,
				)
			}
		}

		return
	}
}

func Testaddtraktdbepisodes() ([]TraktSerieSeasonEpisodes, error) {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		// Look up series by IMDB ID to get Trakt ID
		findResult, err := provider.FindSeriesByIMDbID(context.Background(), "tt1183865")
		if err != nil {
			return nil, err
		}

		if len(findResult.TVResults) == 0 {
			return nil, logger.ErrNotFound
		}

		// Get season 1 episodes using the first result's ID
		episodes, err := provider.GetSeasonEpisodes(
			context.Background(),
			findResult.TVResults[0].ID,
			1,
		)
		if err != nil {
			return nil, err
		}

		// Convert to old format
		result := make([]TraktSerieSeasonEpisodes, len(episodes))
		for i, ep := range episodes {
			result[i] = TraktSerieSeasonEpisodes{
				Title:      ep.Name,
				Overview:   ep.Overview,
				FirstAired: ep.AirDate,
				Season:     ep.SeasonNumber,
				Episode:    ep.EpisodeNumber,
				Runtime:    ep.Runtime,
			}
		}

		return result, nil
	}

	return nil, errors.New("client empty")
}

// checkdbtwostrings checks if the given integer values int1 and int2 exist as a pair in the provided slice of database.DbstaticTwoString.
// It returns true if the pair is found, false otherwise.
func checkdbtwostrings(tbl []database.DbstaticTwoString, int1, int2 int) bool {
	if len(tbl) == 0 {
		return false
	}

	v := database.DbstaticTwoString{Str1: strconv.Itoa(int1), Str2: strconv.Itoa(int2)}
	for idx := range tbl {
		if tbl[idx] == v {
			return true
		}
	}

	return false
}

// padNumberWithZero pads an integer value with leading zeros to ensure it is at least two digits.
// If the value is already two or more digits, it is returned unchanged as a string.
func padNumberWithZero(value *int) string {
	if *value == 0 {
		return "0"
	}

	if *value >= 10 {
		return strconv.Itoa(*value)
	}

	return ("0" + strconv.Itoa(*value))
}

// GetTraktSerieSeasonEpisodes retrieves all episodes for the given show ID and season from the Trakt API.
// It takes the show ID and season number as parameters.
// Returns a slice of TraktSerieSeasonEpisodes structs containing the episode data,
// or nil if there is an error.
func GetTraktSerieSeasonEpisodes(showid, season string) ([]TraktSerieSeasonEpisodes, error) {
	if showid == "" || season == "" {
		return nil, errDailyLimit
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		seriesID, err := strconv.Atoi(showid)
		if err != nil {
			return nil, err
		}

		seasonNum, err := strconv.Atoi(season)
		if err != nil {
			return nil, err
		}

		episodes, err := provider.GetSeasonEpisodes(context.Background(), seriesID, seasonNum)
		if err != nil {
			return nil, err
		}

		// Convert to old format
		result := make([]TraktSerieSeasonEpisodes, len(episodes))
		for i, ep := range episodes {
			result[i] = TraktSerieSeasonEpisodes{
				Title:      ep.Name,
				Overview:   ep.Overview,
				FirstAired: ep.AirDate,
				Season:     ep.SeasonNumber,
				Episode:    ep.EpisodeNumber,
				Runtime:    ep.Runtime,
			}
		}

		return result, nil
	}

	return nil, errors.New("client empty")
}

// GetTraktUserList retrieves a Trakt user list by username, list name, list type,
// and optional limit. It returns a slice of TraktUserList structs containing
// the list data, and an error.
func GetTraktUserList(username, listname, listtype string, limit *string) ([]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		items, err := provider.GetTraktUserList(context.Background(), username, listname, listtype)
		if err != nil {
			return nil, err
		}

		// Convert to old format
		result := make([]TraktUserList, 0, len(items))
		for _, item := range items {
			userListItem := TraktUserList{
				TraktType: item.Type,
			}
			if item.Movie != nil {
				userListItem.Movie = TraktMovie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  item.Movie.IDs.Slug,
						Imdb:  item.Movie.IDs.IMDB,
						Trakt: item.Movie.IDs.Trakt,
						Tmdb:  item.Movie.IDs.TMDB,
						Tvdb:  item.Movie.IDs.TVDB,
					},
					Title: item.Movie.Title,
					Year:  item.Movie.Year,
				}
			}

			if item.Show != nil {
				userListItem.Serie = TraktSerie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  item.Show.IDs.Slug,
						Imdb:  item.Show.IDs.IMDB,
						Trakt: item.Show.IDs.Trakt,
						Tmdb:  item.Show.IDs.TMDB,
						Tvdb:  item.Show.IDs.TVDB,
					},
					Title: item.Show.Title,
					Year:  item.Show.Year,
				}
			}

			result = append(result, userListItem)
		}

		return result, nil
	}

	return nil, errors.New("client empty")
}

// RemoveMovieFromTraktUserList removes the specified movie from the given Trakt user list.
// It takes the username, list name, and the IMDB ID of the movie to remove as parameters.
// If the username or list name are empty, it returns an error.
func RemoveMovieFromTraktUserList(username, listname, remove string) error {
	if username == "" || listname == "" {
		return logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		err := provider.RemoveMovieFromTraktUserList(
			context.Background(),
			username,
			listname,
			remove,
		)
		if err == nil {
			return nil
		}
	}

	return errors.New("client empty")
}

// RemoveSerieFromTraktUserList removes the specified TV show from the given Trakt user list.
// It takes the username, list name, and the TVDB ID of the show to remove as parameters.
// If the username or list name are empty, it returns an error.
func RemoveSerieFromTraktUserList(username, listname string, remove int) error {
	if username == "" || listname == "" {
		return logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		err := provider.RemoveSerieFromTraktUserList(
			context.Background(),
			username,
			listname,
			remove,
		)
		if err == nil {
			return nil
		}
	}

	return errors.New("client empty")
}

// GetTraktSeriePopular retrieves popular TV shows from Trakt based on the
// number of watches and list additions. It takes an optional limit parameter
// to limit the number of results returned. Returns a slice of TraktSerie
// structs containing the popular show data.
func GetTraktSeriePopular(limit *string) []TraktSerie {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		page := 1

		results, err := provider.GetPopularSeries(context.Background(), page)
		if err == nil && results != nil && len(results.Results) > 0 {
			series := make([]TraktSerie, len(results.Results))
			for i, r := range results.Results {
				series[i] = TraktSerie{
					Title: r.Name,
				}
				if !r.FirstAirDate.IsZero() {
					series[i].Year = r.FirstAirDate.Year()
				}

				series[i].IDs.Trakt = r.ID
			}

			return series
		}
	}

	return nil
}

// GetTraktSerieTrending retrieves the trending TV shows from Trakt based on the limit parameter.
// It returns a slice of TraktSerieTrending structs containing the trending show data.
func GetTraktSerieTrending(limit *string) []TraktSerieTrending {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		page := 1

		results, err := provider.GetTrendingSeries(context.Background(), page)
		if err == nil && results != nil && len(results.Results) > 0 {
			series := make([]TraktSerieTrending, len(results.Results))
			for i, r := range results.Results {
				series[i] = TraktSerieTrending{
					Serie: TraktSerie{
						Title: r.Name,
					},
				}
				if !r.FirstAirDate.IsZero() {
					series[i].Serie.Year = r.FirstAirDate.Year()
				}

				series[i].Serie.IDs.Trakt = r.ID
			}

			return series
		}
	}

	return nil
}

// GetTraktSerieAnticipated retrieves the most anticipated TV shows from Trakt
// based on the number of list adds. It takes an optional limit parameter to limit
// the number of results returned. Returns a slice of TraktSerieAnticipated structs
// containing the anticipated show data.
func GetTraktSerieAnticipated(limit *string) []TraktSerieAnticipated {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		page := 1

		results, err := provider.GetTrendingSeries(context.Background(), page)
		if err == nil && results != nil && len(results.Results) > 0 {
			series := make([]TraktSerieAnticipated, len(results.Results))
			for i, r := range results.Results {
				series[i] = TraktSerieAnticipated{
					Serie: TraktSerie{
						Title: r.Name,
					},
				}
				if !r.FirstAirDate.IsZero() {
					series[i].Serie.Year = r.FirstAirDate.Year()
				}

				series[i].Serie.IDs.Trakt = r.ID
			}

			return series
		}
	}

	return nil
}

// GetTraktToken returns the token used to authenticate with Trakt. This is a wrapper around the traktAPI.
func GetTraktToken() *oauth2.Token {
	// Try to get token from v2 provider first
	if provider := providers.GetTrakt(); provider != nil {
		v2Token := provider.GetCurrentToken()
		if v2Token != nil {
			return &oauth2.Token{
				AccessToken:  v2Token.AccessToken,
				TokenType:    v2Token.TokenType,
				RefreshToken: v2Token.RefreshToken,
				Expiry:       v2Token.Expiry,
			}
		}
	}

	return nil
}

// SetTraktToken sets the OAuth 2.0 token used to authenticate
// with the Trakt API.
func SetTraktToken(tk *oauth2.Token) {
	// Update v2 provider token if available
	if provider := providers.GetTrakt(); provider != nil && tk != nil {
		v2Token := &apiexternal_v2.OAuthToken{
			AccessToken:  tk.AccessToken,
			TokenType:    tk.TokenType,
			RefreshToken: tk.RefreshToken,
			Expiry:       tk.Expiry,
		}
		provider.SetToken(v2Token)
	}
}

// GetTraktAuthURL generates an authorization URL that redirects the user
// to the Trakt consent page to request permission for the configured scopes.
// It returns the generated authorization URL.
func GetTraktAuthURL() string {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		urlv := provider.GetAuthorizationURL("state")
		fmt.Println("Visit the URL for the auth dialog: ", urlv)
		return urlv
	}

	return ""
}

// GetTraktAuthToken exchanges the authorization code for an OAuth 2.0 token
// for the Trakt API. It takes the client code and returns the token, or nil and an
// error if there was an issue exchanging the code.
func GetTraktAuthToken(clientcode string) *oauth2.Token {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		v2Token, err := provider.ExchangeCodeForToken(context.Background(), clientcode)
		if err != nil {
			logger.Logtype("error", 1).
				Err(err).
				Msg("Error getting token")
			return nil
		}
		// Convert v2 OAuthToken to oauth2.Token
		return &oauth2.Token{
			AccessToken:  v2Token.AccessToken,
			TokenType:    v2Token.TokenType,
			RefreshToken: v2Token.RefreshToken,
			Expiry:       v2Token.Expiry,
		}
	}

	return nil
}

// RefreshTraktToken manually refreshes the Trakt API token using the refresh token.
func RefreshTraktToken() (*oauth2.Token, error) {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		currentToken := GetTraktToken()
		if currentToken == nil || currentToken.RefreshToken == "" {
			return nil, fmt.Errorf("no refresh token available")
		}

		v2Token, err := provider.RefreshToken(context.Background(), currentToken.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Convert v2 OAuthToken to oauth2.Token
		newToken := &oauth2.Token{
			AccessToken:  v2Token.AccessToken,
			TokenType:    v2Token.TokenType,
			RefreshToken: v2Token.RefreshToken,
			Expiry:       v2Token.Expiry,
		}

		// Update the stored token
		SetTraktToken(newToken)

		// Save to config
		config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: newToken})

		logger.Logtype("info", 1).Msg("Trakt token manually refreshed")

		return newToken, nil
	}

	return nil, errors.New("client empty")
}

// IsTokenExpired checks if the current Trakt token is expired or about to expire.
func IsTokenExpired() bool {
	// Check v2 provider token first
	if provider := providers.GetTrakt(); provider != nil {
		v2Token := provider.GetCurrentToken()
		if v2Token == nil || v2Token.AccessToken == "" {
			return true
		}

		// Check if token is expired or expires within 5 minutes
		if v2Token.Expiry.IsZero() {
			return false // No expiry means token doesn't expire
		}

		return time.Until(v2Token.Expiry) < 5*time.Minute
	}

	return false
}

// GetTraktUserListAuth retrieves a Trakt user list with authentication.
// It takes the username, list name, list type, and optional limit parameters and returns
// the user list items as an array of TraktUserList structs and an error.
// Returns ErrNotFound if username, listname or listtype are empty.
func GetTraktUserListAuth(
	username, listname, listtype string,
	limit *string,
) ([]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		items, err := provider.GetTraktUserList(context.Background(), username, listname, listtype)
		if err != nil {
			return nil, err
		}

		// Convert to old format
		result := make([]TraktUserList, 0, len(items))
		for _, item := range items {
			userListItem := TraktUserList{
				TraktType: item.Type,
			}
			if item.Movie != nil {
				userListItem.Movie = TraktMovie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  item.Movie.IDs.Slug,
						Imdb:  item.Movie.IDs.IMDB,
						Trakt: item.Movie.IDs.Trakt,
						Tmdb:  item.Movie.IDs.TMDB,
						Tvdb:  item.Movie.IDs.TVDB,
					},
					Title: item.Movie.Title,
					Year:  item.Movie.Year,
				}
			}

			if item.Show != nil {
				userListItem.Serie = TraktSerie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  item.Show.IDs.Slug,
						Imdb:  item.Show.IDs.IMDB,
						Trakt: item.Show.IDs.Trakt,
						Tmdb:  item.Show.IDs.TMDB,
						Tvdb:  item.Show.IDs.TVDB,
					},
					Title: item.Show.Title,
					Year:  item.Show.Year,
				}
			}

			result = append(result, userListItem)
		}

		return result, nil
	}

	return nil, errors.New("client empty")
}

// TestTraktConnectivity tests the connectivity to the Trakt API
// Returns status code and error if any.
func TestTraktConnectivity(
	timeout time.Duration,
	limit *string,
) (int, []TraktMovieTrending, error) {
	// Use v2 provider if available
	if provider := providers.GetTrakt(); provider != nil {
		// Test with a simple trending movies request
		page := 1

		results, err := provider.GetTrendingMovies(context.Background(), page)
		if err != nil {
			return 0, nil, err
		}
		// Convert to old format
		if results != nil && len(results.Results) > 0 {
			movies := make([]TraktMovieTrending, len(results.Results))
			for i, r := range results.Results {
				movies[i] = TraktMovieTrending{
					Movie: TraktMovie{
						Title: r.Title,
						Year:  r.Year,
					},
				}
				movies[i].Movie.IDs.Trakt = r.ID
				movies[i].Movie.IDs.Imdb = r.IMDbID
			}

			return 200, movies, nil
		}

		return 200, []TraktMovieTrending{}, nil
	}

	return 400, nil, errors.New("client empty")
}
