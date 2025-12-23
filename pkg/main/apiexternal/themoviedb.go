package apiexternal

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

type TheMovieDBSearch struct {
	// TotalPages   int                          `json:"total_pages"`
	// TotalResults int                          `json:"total_results"`
	// Page         int                          `json:"page"`
	Results []TheMovieDBFindMovieresults `json:"results"`
}

type TheMovieDBSearchTV struct {
	// TotalPages   int                       `json:"total_pages"`
	// TotalResults int                       `json:"total_results"`
	// Page         int                       `json:"page"`
	Results []TheMovieDBFindTvresults `json:"results"`
}

type TheMovieDBList struct {
	// TotalPages   int                       `json:"total_pages"`
	// TotalResults int                       `json:"total_results"`
	ItemCount int                       `json:"item_count"`
	Items     []TheMovieDBFindTvresults `json:"items"`
}

type TheMovieDBFind struct {
	MovieResults []TheMovieDBFindMovieresults `json:"movie_results"`
	TvResults    []TheMovieDBFindTvresults    `json:"tv_results"`
}

type TheMovieDBFindMovieresults struct {
	Overview         string  `json:"overview"`
	ReleaseDate      string  `json:"release_date"`
	Title            string  `json:"title"`
	OriginalLanguage string  `json:"original_language"`
	OriginalTitle    string  `json:"original_title"`
	VoteAverage      float32 `json:"vote_average"`
	Popularity       float32 `json:"popularity"`
	VoteCount        int     `json:"vote_count"`
	ID               int     `json:"id"`
	Adult            bool    `json:"adult"`
}
type TheMovieDBFindTvresults struct {
	ID               int     `json:"id"`
	OriginalLanguage string  `json:"original_language"`
	FirstAirDate     string  `json:"first_air_date"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	Overview         string  `json:"overview"`
	VoteAverage      float32 `json:"vote_average"`
	Popularity       float32 `json:"popularity"`
	VoteCount        int     `json:"vote_count"`
	// OriginCountry    []string `json:"origin_Country"`
}

type TheMovieDBMovieGenres struct {
	Name string `json:"name"`
}
type TheMovieDBMovieLanguages struct {
	EnglishName string `json:"english_name"`
	Name        string `json:"name"`
	Iso6391     string `json:"iso_639_1"`
}
type TheMovieDBMovie struct {
	SpokenLanguages  []TheMovieDBMovieLanguages `json:"spoken_languages"`
	Genres           []TheMovieDBMovieGenres    `json:"genres"`
	Backdrop         string                     `json:"backdrop_path"`
	Poster           string                     `json:"poster_path"`
	Status           string                     `json:"status"`
	Tagline          string                     `json:"tagline"`
	Title            string                     `json:"title"`
	ImdbID           string                     `json:"imdb_id"`
	OriginalLanguage string                     `json:"original_language"`
	OriginalTitle    string                     `json:"original_title"`
	Overview         string                     `json:"overview"`
	ReleaseDate      string                     `json:"release_date"`
	Popularity       float32                    `json:"popularity"`
	VoteAverage      float32                    `json:"vote_average"`
	Revenue          int                        `json:"revenue"`
	Runtime          int                        `json:"runtime"`
	Budget           int                        `json:"budget"`
	ID               int                        `json:"id"`
	VoteCount        int32                      `json:"vote_count"`
	Adult            bool                       `json:"adult"`
}

type TheMovieDBMovieTitles struct {
	// ID     int                         `json:"id"`
	Titles []struct {
		TmdbType string `json:"type"`
		Title    string `json:"title"`
		Iso31661 string `json:"iso_3166_1"`
	} `json:"titles"`
}

type TheMovieDBTVExternal struct {
	ImdbID      string `json:"imdb_id"`
	FreebaseMID string `json:"freebase_mid"`
	FreebaseID  string `json:"freebase_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
	ID          int    `json:"id"`
	TvdbID      int    `json:"tvdb_id"`
	TvrageID    int    `json:"tvrage_id"`
}

// NewTmdbClient creates a new TMDb client for making API requests.
// It takes the TMDb API key, rate limiting settings, TLS setting, and timeout.
// Returns a tmdbClient instance configured with the provided settings.
func NewTmdbClient(
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
	tmdbConfig := base.ClientConfig{
		Name:                      "tmdb",
		BaseURL:                   "https://api.themoviedb.org/3",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthNone, // TMDB handles Bearer token auth internally
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		RateLimitPer24h:           40000,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		EnableStats:               true,
		UserAgent:                 "go-media-downloader/2.0",
		DisableTLSVerify:          disabletls,
	}
	if provider := tmdb.NewProviderWithConfig(tmdbConfig, apikey); provider != nil {
		providers.SetTMDB(provider)
		logger.Logtype(logger.StatusDebug, 0).
			Msg("Registered TMDB metadata provider with rate limiting")
	}
}

// SearchTmdbMovie searches for movies on TheMovieDB API by movie name.
// It takes a movie name string as input and returns a pointer to a TheMovieDBSearch struct containing the search results,
// or an error if the name is empty or the API call fails.
func SearchTmdbMovie(name string) (*TheMovieDBSearch, error) {
	if name == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		results, err := provider.SearchMovies(context.Background(), name, 0)
		if err != nil {
			return nil, err
		}
		// Convert v2 results to old format
		search := &TheMovieDBSearch{
			Results: make([]TheMovieDBFindMovieresults, len(results)),
		}
		for i, r := range results {
			releaseDate := ""
			if !r.ReleaseDate.IsZero() {
				releaseDate = r.ReleaseDate.Format("2006-01-02")
			}

			search.Results[i] = TheMovieDBFindMovieresults{
				ID:               r.ID,
				Title:            r.Title,
				OriginalTitle:    r.OriginalTitle,
				Overview:         r.Overview,
				ReleaseDate:      releaseDate,
				OriginalLanguage: "",
				VoteAverage:      float32(r.VoteAverage),
				VoteCount:        r.VoteCount,
				Popularity:       float32(r.Popularity),
				Adult:            r.Adult,
			}
		}

		return search, nil
	}

	return nil, logger.ErrNotFound
}

// SearchTmdbTV searches for TV shows on TheMovieDB API.
// It takes a search query string and returns a TheMovieDBSearchTV struct containing the search results.
// Returns ErrNotFound error if the search query is empty.
func SearchTmdbTV(name string) (*TheMovieDBSearchTV, error) {
	if name == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		results, err := provider.SearchSeries(context.Background(), name, 0)
		if err != nil {
			return nil, err
		}
		// Convert v2 results to old format
		search := &TheMovieDBSearchTV{
			Results: make([]TheMovieDBFindTvresults, len(results)),
		}
		for i, r := range results {
			firstAirDate := ""
			if !r.FirstAirDate.IsZero() {
				firstAirDate = r.FirstAirDate.Format("2006-01-02")
			}

			search.Results[i] = TheMovieDBFindTvresults{
				ID:               r.ID,
				Name:             r.Name,
				OriginalName:     r.OriginalName,
				Overview:         r.Overview,
				FirstAirDate:     firstAirDate,
				OriginalLanguage: "",
				VoteAverage:      float32(r.VoteAverage),
				VoteCount:        r.VoteCount,
				Popularity:       float32(r.Popularity),
			}
		}

		return search, nil
	}

	return nil, logger.ErrNotFound
}

// DiscoverTmdbMovie searches for movies on TheMovieDB API using the provided query string.
// It takes a query string as input and returns a pointer to a TheMovieDBSearch struct containing the search results,
// or an error if the query is empty or the API call fails.
func DiscoverTmdbMovie(query string) (*TheMovieDBSearch, error) {
	if query == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		results, err := provider.DiscoverMovies(context.Background(), query, 1)
		if err != nil {
			return nil, err
		}
		// Convert v2 results to old format
		search := &TheMovieDBSearch{
			Results: make([]TheMovieDBFindMovieresults, len(results.Results)),
		}
		for i, r := range results.Results {
			releaseDate := ""
			if !r.ReleaseDate.IsZero() {
				releaseDate = r.ReleaseDate.Format("2006-01-02")
			}

			search.Results[i] = TheMovieDBFindMovieresults{
				ID:            r.ID,
				Title:         r.Title,
				OriginalTitle: r.OriginalTitle,
				Overview:      r.Overview,
				ReleaseDate:   releaseDate,
				Popularity:    float32(r.Popularity),
				VoteAverage:   float32(r.VoteAverage),
				VoteCount:     r.VoteCount,
				Adult:         r.Adult,
			}
		}

		return search, nil
	}

	return nil, logger.ErrNotFound
}

// DiscoverTmdbSerie searches for TV shows on TheMovieDB API using the provided query string.
// It takes a query string as input and returns a pointer to a TheMovieDBSearchTV struct containing the search results,
// or an error if the query is empty or the API call fails.
func DiscoverTmdbSerie(query string) (*TheMovieDBSearchTV, error) {
	if query == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		results, err := provider.DiscoverTV(context.Background(), query, 1)
		if err != nil {
			return nil, err
		}
		// Convert v2 results to old format
		search := &TheMovieDBSearchTV{
			Results: make([]TheMovieDBFindTvresults, len(results.Results)),
		}
		for i, r := range results.Results {
			firstAirDate := ""
			if !r.FirstAirDate.IsZero() {
				firstAirDate = r.FirstAirDate.Format("2006-01-02")
			}

			search.Results[i] = TheMovieDBFindTvresults{
				ID:           r.ID,
				Name:         r.Name,
				OriginalName: r.OriginalName,
				Overview:     r.Overview,
				FirstAirDate: firstAirDate,
				Popularity:   float32(r.Popularity),
				VoteAverage:  float32(r.VoteAverage),
				VoteCount:    r.VoteCount,
			}
		}

		return search, nil
	}

	return nil, logger.ErrNotFound
}

// GetTmdbList retrieves a list of items from TheMovieDB API by the given list ID.
// It takes an integer list ID as input and returns a TheMovieDBList struct containing the list data.
// If the initial API response does not contain all the list items, it will make additional requests to fetch the remaining pages.
// Returns an error if the API call fails.
func GetTmdbList(listid int) (TheMovieDBList, error) {
	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		response, err := provider.GetTMDBList(context.Background(), listid)
		if err != nil {
			return TheMovieDBList{}, err
		}

		// Convert TMDBListResponse to TheMovieDBList
		list := TheMovieDBList{
			ItemCount: response.ItemCount,
			Items:     make([]TheMovieDBFindTvresults, len(response.Items)),
		}

		for i, item := range response.Items {
			list.Items[i] = TheMovieDBFindTvresults{
				ID:           item.ID,
				FirstAirDate: item.ReleaseDate,
				Name:         item.Title,
				OriginalName: item.OriginalTitle,
				Overview:     item.Overview,
				VoteAverage:  float32(item.VoteAverage),
				Popularity:   float32(item.Popularity),
				VoteCount:    item.VoteCount,
			}
		}

		return list, nil
	}

	return TheMovieDBList{}, logger.ErrNotFound
}

// RemoveFromTmdbList removes an item from a TheMovieDB list by the given list ID and item ID.
// It takes the list ID and the ID of the item to remove as input, and returns an error if the operation fails.
func RemoveFromTmdbList(listid int, remove int) error {
	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		return provider.RemoveFromTMDBList(context.Background(), listid, remove)
	}
	return logger.ErrNotFound
}

// FindTmdbImdb searches TheMovieDB API to find a movie based on its IMDb ID.
// It takes an IMDb ID string as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the IMDb ID is empty.
func FindTmdbImdb(imdbid string) (*TheMovieDBFind, error) {
	if imdbid == "" {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		result, err := provider.FindMovieByIMDbID(context.Background(), imdbid)
		if err != nil {
			return nil, err
		}
		// Convert v2 result to old format
		find := &TheMovieDBFind{
			MovieResults: make([]TheMovieDBFindMovieresults, len(result.MovieResults)),
			TvResults:    make([]TheMovieDBFindTvresults, len(result.TVResults)),
		}
		for i, r := range result.MovieResults {
			releaseDate := ""
			if !r.ReleaseDate.IsZero() {
				releaseDate = r.ReleaseDate.Format("2006-01-02")
			}

			find.MovieResults[i] = TheMovieDBFindMovieresults{
				ID:               r.ID,
				Title:            r.Title,
				OriginalTitle:    r.OriginalTitle,
				Overview:         r.Overview,
				ReleaseDate:      releaseDate,
				OriginalLanguage: "",
				VoteAverage:      float32(r.VoteAverage),
				VoteCount:        r.VoteCount,
				Popularity:       float32(r.Popularity),
				Adult:            r.Adult,
			}
		}

		for i, r := range result.TVResults {
			firstAirDate := ""
			if !r.FirstAirDate.IsZero() {
				firstAirDate = r.FirstAirDate.Format("2006-01-02")
			}

			find.TvResults[i] = TheMovieDBFindTvresults{
				ID:               r.ID,
				Name:             r.Name,
				OriginalName:     r.OriginalName,
				Overview:         r.Overview,
				FirstAirDate:     firstAirDate,
				OriginalLanguage: "",
				VoteAverage:      float32(r.VoteAverage),
				VoteCount:        r.VoteCount,
				Popularity:       float32(r.Popularity),
			}
		}

		return find, nil
	}

	return nil, logger.ErrNotFound
}

// FindTmdbTvdb searches TheMovieDB API to find a TV show based on its TheTVDB ID.
// It takes a TheTVDB ID int as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the TheTVDB ID is 0.
func FindTmdbTvdb(thetvdbid int) (*TheMovieDBFind, error) {
	if thetvdbid == 0 {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		result, err := provider.FindSeriesByTVDbID(context.Background(), thetvdbid)
		if err != nil {
			return nil, err
		}
		// Convert v2 result to old format
		find := &TheMovieDBFind{
			MovieResults: []TheMovieDBFindMovieresults{},
			TvResults:    make([]TheMovieDBFindTvresults, 0, 1),
		}
		if result != nil {
			firstAirDate := ""
			if !result.FirstAirDate.IsZero() {
				firstAirDate = result.FirstAirDate.Format("2006-01-02")
			}

			find.TvResults = append(find.TvResults, TheMovieDBFindTvresults{
				ID:               result.ID,
				Name:             result.Name,
				OriginalName:     result.OriginalName,
				Overview:         result.Overview,
				FirstAirDate:     firstAirDate,
				OriginalLanguage: result.OriginalLanguage,
				VoteAverage:      float32(result.VoteAverage),
				VoteCount:        result.VoteCount,
				Popularity:       float32(result.Popularity),
			})
		}

		return find, nil
	}

	return nil, logger.ErrNotFound
}

// GetTmdbMovie retrieves movie details from TheMovieDB API by movie ID.
// It takes an integer movie ID as input and returns a TheMovieDBMovie struct containing the movie details.
// Returns an error if the ID is invalid or the API call fails.
func GetTmdbMovie(id int) (*TheMovieDBMovie, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		details, err := provider.GetMovieByID(context.Background(), id)
		if err != nil {
			return nil, err
		}
		// Convert v2 details to old format
		releaseDate := ""
		if !details.ReleaseDate.IsZero() {
			releaseDate = details.ReleaseDate.Format("2006-01-02")
		}

		movie := &TheMovieDBMovie{
			ID:               details.ID,
			ImdbID:           details.IMDbID,
			Title:            details.Title,
			OriginalTitle:    details.OriginalTitle,
			Overview:         details.Overview,
			ReleaseDate:      releaseDate,
			Status:           details.Status,
			Tagline:          details.Tagline,
			Poster:           details.PosterPath,
			Backdrop:         details.BackdropPath,
			OriginalLanguage: details.OriginalLanguage,
			Popularity:       float32(details.Popularity),
			VoteAverage:      float32(details.VoteAverage),
			VoteCount:        int32(details.VoteCount),
			Runtime:          details.Runtime,
			Budget:           int(details.Budget),
			Revenue:          int(details.Revenue),
			Adult:            details.Adult,
			Genres:           make([]TheMovieDBMovieGenres, len(details.Genres)),
			SpokenLanguages:  []TheMovieDBMovieLanguages{},
		}
		for i, g := range details.Genres {
			movie.Genres[i] = TheMovieDBMovieGenres{Name: g.Name}
		}

		return movie, nil
	}

	return nil, logger.ErrNotFound
}

// GetTmdbMovieTitles retrieves the alternative titles for a TMDb movie by ID.
// It returns a TheMovieDBMovieTitles struct containing the titles,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieTitles(id int) (*TheMovieDBMovieTitles, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		titles, err := provider.GetMovieAlternativeTitles(context.Background(), id)
		if err != nil {
			return nil, err
		}
		// Convert v2 titles to old format
		result := &TheMovieDBMovieTitles{
			Titles: make([]struct {
				TmdbType string `json:"type"`
				Title    string `json:"title"`
				Iso31661 string `json:"iso_3166_1"`
			}, len(titles)),
		}
		for i, t := range titles {
			result.Titles[i].Title = t.Title
			result.Titles[i].Iso31661 = t.ISO3166_1
			result.Titles[i].TmdbType = t.Type
		}

		return result, nil
	}

	return nil, logger.ErrNotFound
}

// GetTmdbMovieExternal retrieves the external IDs for a TMDb movie by ID.
// It returns a TheMovieDBTVExternal struct containing the external IDs,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieExternal(id int) (*TheMovieDBTVExternal, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		externalIDs, err := provider.GetMovieExternalIDs(context.Background(), id)
		if err != nil {
			return nil, err
		}
		// Convert v2 result to old format
		return &TheMovieDBTVExternal{
			ID:          externalIDs.TMDbID,
			ImdbID:      externalIDs.IMDbID,
			FacebookID:  externalIDs.FacebookID,
			InstagramID: externalIDs.InstagramID,
			TwitterID:   externalIDs.TwitterID,
			TvdbID:      0, // Movies don't have TVDB IDs
			TvrageID:    0,
			FreebaseMID: "",
			FreebaseID:  "",
		}, nil
	}

	return nil, logger.ErrNotFound
}

// GetTVExternal retrieves the external IDs for a TV show from TheMovieDB.
// It takes the ID of the TV show and returns a pointer to a TheMovieDBTVExternal struct containing the external IDs.
// Returns an error if the ID is invalid or the API call fails.
func GetTVExternal(id int) (*TheMovieDBTVExternal, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		externalIDs, err := provider.GetSeriesExternalIDs(context.Background(), id)
		if err != nil {
			return nil, err
		}
		// Convert v2 result to old format
		return &TheMovieDBTVExternal{
			ID:          externalIDs.TMDbID,
			ImdbID:      externalIDs.IMDbID,
			TvdbID:      externalIDs.TVDbID,
			FacebookID:  externalIDs.FacebookID,
			InstagramID: externalIDs.InstagramID,
			TwitterID:   externalIDs.TwitterID,
			TvrageID:    0,
			FreebaseMID: "",
			FreebaseID:  "",
		}, nil
	}

	return nil, logger.ErrNotFound
}

// TestTMDBConnectivity tests the connectivity to the TMDB API
// Returns status code and error if any.
func TestTMDBConnectivity(timeout time.Duration) (int, error) {
	// Use v2 provider if available
	if provider := providers.GetTMDB(); provider != nil {
		// Test with a simple search
		_, err := provider.SearchMovies(context.Background(), "test", 0)
		if err != nil {
			return 0, err
		}

		return 200, nil
	}

	return 0, logger.ErrNotFound
}

// SearchTMDBMovieImdbID searches for a movie by title and year, then retrieves its IMDB ID.
// Returns the IMDB ID if found, empty string and error otherwise.
func SearchTMDBMovieImdbID(title string, year int) (string, error) {
	if title == "" {
		return "", logger.ErrNotFound
	}

	// Search for the movie
	searchResults, err := SearchTmdbMovie(title)
	if err != nil {
		return "", err
	}

	if len(searchResults.Results) == 0 {
		return "", logger.ErrNotFound
	}

	// Find the best match based on title and year
	var bestMatch *TheMovieDBFindMovieresults

	for i := range searchResults.Results {
		result := &searchResults.Results[i]

		// Check if title matches (case insensitive)
		titleMatch := strings.EqualFold(result.Title, title) ||
			strings.EqualFold(result.OriginalTitle, title)

		// If year is provided, verify it matches
		if year > 0 && result.ReleaseDate != "" {
			if len(result.ReleaseDate) >= 4 {
				releaseYear, err := strconv.Atoi(result.ReleaseDate[:4])
				if err == nil && releaseYear == year && titleMatch {
					// Exact match on both title and year
					bestMatch = result
					break
				}
			}
		} else if titleMatch {
			// No year provided or no release date, match on title only
			bestMatch = result
			break
		}
	}

	// If no exact match found, return error
	if bestMatch == nil {
		return "", logger.ErrNotFound
	}

	// Get external IDs for the matched movie
	externalIDs, err := GetTmdbMovieExternal(bestMatch.ID)
	if err != nil {
		return "", err
	}

	if externalIDs.ImdbID == "" {
		return "", logger.ErrNotFound
	}

	return externalIDs.ImdbID, nil
}
