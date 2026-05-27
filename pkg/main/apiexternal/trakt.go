package apiexternal

import (
	"context"
	"fmt"
	"iter"
	"slices"
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
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disabletls,
	}
	if provider := trakt.NewProviderWithConfig(
		traktConfig,
		clientid,
		clientsecret,
		redirecturl,
	); provider != nil {
		providers.SetTrakt(provider)
		logger.Logtype(logger.StatusDebug, 0).Msg("Registered Trakt metadata provider with OAuth2")
	}
}

// GetTraktMoviePopular retrieves popular movies from the Trakt API, yielding each entry
// directly without an intermediate slice allocation.
func GetTraktMoviePopular(limit *string, extraParams string) iter.Seq[TraktMovie] {
	return func(yield func(TraktMovie) bool) {
		provider := providers.GetTrakt()
		if provider == nil {
			return
		}

		results, err := provider.GetPopularMovies(context.Background(), 1, extraParams)
		if err != nil || results == nil {
			return
		}

		for i := range results.Results {
			var m TraktMovie

			m.Title = results.Results[i].Title
			m.Year = results.Results[i].Year
			m.IDs.Trakt = results.Results[i].ID

			m.IDs.Imdb = results.Results[i].IMDbID
			if !yield(m) {
				return
			}
		}
	}
}

// GetTraktMovieTrending retrieves trending movies from the Trakt API, yielding each
// TraktMovie directly (the Trending wrapper is omitted since callers only need the inner Movie).
func GetTraktMovieTrending(limit *string, extraParams string) iter.Seq[TraktMovie] {
	return func(yield func(TraktMovie) bool) {
		provider := providers.GetTrakt()
		if provider == nil {
			return
		}

		results, err := provider.GetTrendingMovies(context.Background(), 1, extraParams)
		if err != nil || results == nil {
			return
		}

		for i := range results.Results {
			var m TraktMovie

			m.Title = results.Results[i].Title
			m.Year = results.Results[i].Year
			m.IDs.Trakt = results.Results[i].ID

			m.IDs.Imdb = results.Results[i].IMDbID
			if !yield(m) {
				return
			}
		}
	}
}

// GetTraktMovieAnticipated retrieves anticipated movies from the Trakt API, yielding each
// TraktMovie directly (the Anticipated wrapper is omitted since callers only need the inner Movie).
func GetTraktMovieAnticipated(limit *string, extraParams string) iter.Seq[TraktMovie] {
	return func(yield func(TraktMovie) bool) {
		provider := providers.GetTrakt()
		if provider == nil {
			return
		}

		results, err := provider.GetUpcomingMovies(context.Background(), 1, extraParams)
		if err != nil || results == nil {
			return
		}

		for i := range results.Results {
			var m TraktMovie

			m.Title = results.Results[i].Title
			m.Year = results.Results[i].Year
			m.IDs.Trakt = results.Results[i].ID

			m.IDs.Imdb = results.Results[i].IMDbID
			if !yield(m) {
				return
			}
		}
	}
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
					details, err = provider.GetMovieByID(
						context.Background(),
						findResult.MovieResults[0].ID,
					)
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
			for i := range details.Genres {
				movie.Genres = append(movie.Genres, details.Genres[i].Name)
			}

			return movie, nil
		}
	}

	return nil, errNoClient
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
			if !strings.HasPrefix(showid, "tt") {
				return nil, fmt.Errorf("invalid show ID format: %s", showid)
			}

			// It's an IMDb ID, look it up first
			findResult, findErr := provider.FindSeriesByIMDbID(context.Background(), showid)
			if findErr == nil && findResult != nil && len(findResult.TVResults) > 0 {
				// Get full details using the Trakt ID from search results
				details, err = provider.GetSeriesByID(
					context.Background(),
					findResult.TVResults[0].ID,
				)
			} else {
				err = findErr
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
			for i := range details.Genres {
				serie.Genres = append(serie.Genres, details.Genres[i].Name)
			}

			return serie, nil
		}

		if err != nil {
			return nil, err
		}
	}

	return nil, errNoClient
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

	return nil, errClientEmpty
}

// MergeTraktIntoCollectedEpisodes merges Trakt episode data into the collected episodes map.
// TVDB data is preferred - Trakt data only fills in gaps (empty/zero values) and adds missing episodes.
// The showid should be the IMDB ID (tt...) or Trakt ID for the series.
func MergeTraktIntoCollectedEpisodes(showid string, episodes map[string]*CollectedEpisode) {
	if showid == "" {
		return
	}

	// Use v2 provider if available
	provider := providers.GetTrakt()
	if provider == nil {
		return
	}

	// Try to parse as integer for Trakt ID first, otherwise use IMDB ID lookup
	var seriesID int
	if id, err := strconv.Atoi(showid); err == nil {
		seriesID = id
	} else if strings.HasPrefix(showid, "tt") {
		// It's an IMDb ID, look it up first
		findResult, err := provider.FindSeriesByIMDbID(context.Background(), showid)
		if err != nil || findResult == nil || len(findResult.TVResults) == 0 {
			return
		}

		seriesID = findResult.TVResults[0].ID
	} else {
		return
	}

	seasons, err := provider.GetAllSeasons(context.Background(), seriesID)
	if err != nil {
		return
	}

	for i := range seasons {
		seasonEpisodes, err := provider.GetSeasonEpisodes(
			context.Background(),
			seriesID,
			seasons[i].SeasonNumber,
		)
		if err != nil {
			continue
		}

		for j := range seasonEpisodes {
			ep := seasonEpisodes[j]
			key := strconv.Itoa(ep.SeasonNumber) + "-" + strconv.Itoa(ep.EpisodeNumber)

			if existing, ok := episodes[key]; ok {
				// Episode exists from TVDB - only fill in empty fields
				if existing.Title == "" && ep.Name != "" {
					existing.Title = ep.Name
				}

				if existing.FirstAired.IsZero() && !ep.AirDate.IsZero() {
					existing.FirstAired = ep.AirDate
				}

				if existing.Overview == "" && ep.Overview != "" {
					existing.Overview = ep.Overview
				}

				if existing.AbsoluteNumber == 0 && ep.AbsoluteNumber != 0 {
					existing.AbsoluteNumber = ep.AbsoluteNumber
				}

				// Note: Trakt doesn't provide poster/still images, so we don't update Poster
			} else {
				// Episode doesn't exist from TVDB - add it from Trakt
				episodes[key] = &CollectedEpisode{
					Season:         ep.SeasonNumber,
					Episode:        ep.EpisodeNumber,
					AbsoluteNumber: ep.AbsoluteNumber,
					Title:          ep.Name,
					FirstAired:     ep.AirDate,
					Overview:       ep.Overview,
					Poster:         "", // Trakt doesn't provide poster images
				}
			}
		}
	}
}

// UpdateTraktSerieSeasonsAndEpisodes retrieves all seasons and episodes for the given Trakt show ID from the Trakt API.
// It takes the show ID and database series ID as parameters.
// It queries the local database for existing episodes to avoid duplicates.
// For each season, it inserts missing episodes or only fills empty fields in existing ones (does not overwrite TVDB data).
// Deprecated: Use CollectTvdbSeriesEpisodes + MergeTraktIntoCollectedEpisodes + WriteCollectedEpisodesToDB instead.
func UpdateTraktSerieSeasonsAndEpisodes(showid string, id *uint) {
	if showid == "" {
		return
	}

	// Use v2 provider if available
	provider := providers.GetTrakt()
	if provider == nil {
		return
	}

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

	for i := range seasons {
		episodes, err := provider.GetSeasonEpisodes(
			context.Background(),
			seriesID,
			seasons[i].SeasonNumber,
		)
		if err != nil {
			continue
		}

		for j := range episodes {
			ep := episodes[j]
			epi := strconv.Itoa(ep.EpisodeNumber)
			seas := strconv.Itoa(ep.SeasonNumber)
			ident := generateIdentifierStringFromInt(&ep.SeasonNumber, &ep.EpisodeNumber)

			if checkdbtwostrings(tbl, ep.SeasonNumber, ep.EpisodeNumber) {
				// Episode exists - only fill in empty fields (don't overwrite TVDB data)
				database.ExecN(
					`UPDATE dbserie_episodes SET
						title = CASE WHEN title IS NULL OR title = '' THEN ? ELSE title END,
						first_aired = CASE WHEN first_aired IS NULL OR first_aired = '' THEN ? ELSE first_aired END,
						overview = CASE WHEN overview IS NULL OR overview = '' THEN ? ELSE overview END,
						absolute_number = CASE WHEN absolute_number IS NULL OR absolute_number = 0 THEN ? ELSE absolute_number END,
						updated_at = CURRENT_TIMESTAMP
					WHERE dbserie_id = ? AND season = ? AND episode = ?`,
					&ep.Name,
					&ep.AirDate,
					&ep.Overview,
					&ep.AbsoluteNumber,
					id,
					&seas,
					&epi,
				)
			} else {
				// Episode doesn't exist - insert it
				database.ExecN(
					"INSERT INTO dbserie_episodes (episode, season, identifier, title, first_aired, overview, absolute_number, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
					&epi,
					&seas,
					&ident,
					&ep.Name,
					&ep.AirDate,
					&ep.Overview,
					&ep.AbsoluteNumber,
					id,
				)
			}
		}
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

	return nil, errClientEmpty
}

// checkdbtwostrings checks if the given integer values int1 and int2 exist as a pair in the provided slice of database.DbstaticTwoString.
// It returns true if the pair is found, false otherwise.
func checkdbtwostrings(tbl []database.DbstaticTwoString, int1, int2 int) bool {
	if len(tbl) == 0 {
		return false
	}

	v := database.DbstaticTwoString{Str1: strconv.Itoa(int1), Str2: strconv.Itoa(int2)}

	return slices.Contains(tbl, v)
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

	return nil, errClientEmpty
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
		var (
			items []trakt.TraktUserListItem
			err   error
		)

		if listname == "watchlist" {
			items, err = provider.GetWatchlist(context.Background(), username, listtype)
		} else {
			items, err = provider.GetTraktUserList(
				context.Background(),
				username,
				listname,
				listtype,
			)
		}

		if err != nil {
			return nil, err
		}

		// Convert to old format
		result := make([]TraktUserList, 0, len(items))
		for i := range items {
			userListItem := TraktUserList{
				TraktType: items[i].Type,
			}
			if items[i].Movie != nil {
				userListItem.Movie = TraktMovie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  items[i].Movie.IDs.Slug,
						Imdb:  items[i].Movie.IDs.IMDB,
						Trakt: items[i].Movie.IDs.Trakt,
						Tmdb:  items[i].Movie.IDs.TMDB,
						Tvdb:  items[i].Movie.IDs.TVDB,
					},
					Title: items[i].Movie.Title,
					Year:  items[i].Movie.Year,
				}
			}

			if items[i].Show != nil {
				userListItem.Serie = TraktSerie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  items[i].Show.IDs.Slug,
						Imdb:  items[i].Show.IDs.IMDB,
						Trakt: items[i].Show.IDs.Trakt,
						Tmdb:  items[i].Show.IDs.TMDB,
						Tvdb:  items[i].Show.IDs.TVDB,
					},
					Title: items[i].Show.Title,
					Year:  items[i].Show.Year,
				}
			}

			result = append(result, userListItem)
		}

		return result, nil
	}

	return nil, errClientEmpty
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

	return errClientEmpty
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

	return errClientEmpty
}

// GetTraktSeriePopular retrieves popular TV shows from Trakt, yielding each entry
// directly without an intermediate slice allocation.
func GetTraktSeriePopular(limit *string, extraParams string) iter.Seq[TraktSerie] {
	return func(yield func(TraktSerie) bool) {
		provider := providers.GetTrakt()
		if provider == nil {
			return
		}

		results, err := provider.GetPopularSeries(context.Background(), 1, extraParams)
		if err != nil || results == nil {
			return
		}

		for i := range results.Results {
			var s TraktSerie

			s.Title = results.Results[i].Name
			if !results.Results[i].FirstAirDate.IsZero() {
				s.Year = results.Results[i].FirstAirDate.Year()
			}

			s.IDs.Trakt = results.Results[i].ID
			if !yield(s) {
				return
			}
		}
	}
}

// GetTraktSerieTrending retrieves trending TV shows from Trakt, yielding each TraktSerie
// directly (the Trending wrapper is omitted since callers only need the inner Serie).
func GetTraktSerieTrending(limit *string, extraParams string) iter.Seq[TraktSerie] {
	return func(yield func(TraktSerie) bool) {
		provider := providers.GetTrakt()
		if provider == nil {
			return
		}

		results, err := provider.GetTrendingSeries(context.Background(), 1, extraParams)
		if err != nil || results == nil {
			return
		}

		for i := range results.Results {
			var s TraktSerie

			s.Title = results.Results[i].Name
			if !results.Results[i].FirstAirDate.IsZero() {
				s.Year = results.Results[i].FirstAirDate.Year()
			}

			s.IDs.Trakt = results.Results[i].ID
			if !yield(s) {
				return
			}
		}
	}
}

// GetTraktSerieAnticipated retrieves anticipated TV shows from Trakt, yielding each TraktSerie
// directly (the Anticipated wrapper is omitted since callers only need the inner Serie).
func GetTraktSerieAnticipated(limit *string, extraParams string) iter.Seq[TraktSerie] {
	return func(yield func(TraktSerie) bool) {
		provider := providers.GetTrakt()
		if provider == nil {
			return
		}

		results, err := provider.GetTrendingSeries(context.Background(), 1, extraParams)
		if err != nil || results == nil {
			return
		}

		for i := range results.Results {
			var s TraktSerie

			s.Title = results.Results[i].Name
			if !results.Results[i].FirstAirDate.IsZero() {
				s.Year = results.Results[i].FirstAirDate.Year()
			}

			s.IDs.Trakt = results.Results[i].ID
			if !yield(s) {
				return
			}
		}
	}
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
		logger.Logtype("info", 0).Str("url", urlv).Msg("Visit the URL for the auth dialog")
		return urlv
	}

	return ""
}

// GetTraktAuthToken exchanges the authorization code for an OAuth 2.0 token
// for the Trakt API. It takes the client code and returns the token and any error.
func GetTraktAuthToken(clientcode string) (*oauth2.Token, error) {
	if provider := providers.GetTrakt(); provider != nil {
		v2Token, err := provider.ExchangeCodeForToken(context.Background(), clientcode)
		if err != nil {
			logger.Logtype("error", 1).
				Err(err).
				Msg("Error getting Trakt token")
			return nil, err
		}

		return &oauth2.Token{
			AccessToken:  v2Token.AccessToken,
			TokenType:    v2Token.TokenType,
			RefreshToken: v2Token.RefreshToken,
			Expiry:       v2Token.Expiry,
		}, nil
	}

	return nil, errTraktNotInit
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

	return nil, errClientEmpty
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
		var (
			items []trakt.TraktUserListItem
			err   error
		)

		if listname == "watchlist" {
			items, err = provider.GetWatchlist(context.Background(), username, listtype)
		} else {
			items, err = provider.GetTraktUserList(
				context.Background(),
				username,
				listname,
				listtype,
			)
		}

		if err != nil {
			return nil, err
		}

		// Convert to old format
		result := make([]TraktUserList, 0, len(items))
		for i := range items {
			userListItem := TraktUserList{
				TraktType: items[i].Type,
			}
			if items[i].Movie != nil {
				userListItem.Movie = TraktMovie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  items[i].Movie.IDs.Slug,
						Imdb:  items[i].Movie.IDs.IMDB,
						Trakt: items[i].Movie.IDs.Trakt,
						Tmdb:  items[i].Movie.IDs.TMDB,
						Tvdb:  items[i].Movie.IDs.TVDB,
					},
					Title: items[i].Movie.Title,
					Year:  items[i].Movie.Year,
				}
			}

			if items[i].Show != nil {
				userListItem.Serie = TraktSerie{
					IDs: struct {
						Slug   string `json:"slug"`
						Imdb   string `json:"imdb"`
						Trakt  int    `json:"trakt"`
						Tmdb   int    `json:"tmdb"`
						Tvdb   int    `json:"tvdb"`
						Tvrage int    `json:"tvrage"`
					}{
						Slug:  items[i].Show.IDs.Slug,
						Imdb:  items[i].Show.IDs.IMDB,
						Trakt: items[i].Show.IDs.Trakt,
						Tmdb:  items[i].Show.IDs.TMDB,
						Tvdb:  items[i].Show.IDs.TVDB,
					},
					Title: items[i].Show.Title,
					Year:  items[i].Show.Year,
				}
			}

			result = append(result, userListItem)
		}

		return result, nil
	}

	return nil, errClientEmpty
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

		results, err := provider.GetTrendingMovies(context.Background(), page, "")
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

	return 400, nil, errClientEmpty
}
