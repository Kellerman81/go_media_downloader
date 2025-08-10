package apiexternal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
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

// tmdbClient is a struct for interacting with the TMDB API
// It contains fields for the API key, query parameter API key,
// and a pointer to the rate limited HTTP client.
type tmdbClient struct {
	Client         rlHTTPClient // Pointer to the rate limited HTTP client
	Lim            slidingwindow.Limiter
	DefaultHeaders map[string][]string // Default headers to send with requests
	APIKey         string              // The TMDB API key
	QAPIKey        string              // The query parameter API key
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
	tmdbAPI = tmdbClient{
		APIKey: apikey,
		DefaultHeaders: map[string][]string{
			"accept":        {"application/json"},
			"Authorization": {"Bearer " + apikey},
		},
		Lim: slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		Client: newClient(
			"tmdb",
			disabletls,
			true,
			&tmdbAPI.Lim,
			false, nil, timeoutseconds),
	}
}

// SearchTmdbMovie searches for movies on TheMovieDB API by movie name.
// It takes a movie name string as input and returns a pointer to a TheMovieDBSearch struct containing the search results,
// or an error if the name is empty or the API call fails.
func SearchTmdbMovie(name string) (*TheMovieDBSearch, error) {
	if name == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBSearch](
		&tmdbAPI.Client,
		logger.JoinStrings(
			"https://api.themoviedb.org/3/search/movie?query=",
			url.QueryEscape(name),
		),
		tmdbAPI.DefaultHeaders,
	)
}

// SearchTmdbTV searches for TV shows on TheMovieDB API.
// It takes a search query string and returns a TheMovieDBSearchTV struct containing the search results.
// Returns ErrNotFound error if the search query is empty.
func SearchTmdbTV(name string) (*TheMovieDBSearchTV, error) {
	if name == "" {
		return nil, logger.ErrNotFound
	}

	return doJSONTypeP[TheMovieDBSearchTV](
		&tmdbAPI.Client,
		logger.JoinStrings("https://api.themoviedb.org/3/search/tv?query=", url.QueryEscape(name)),
		tmdbAPI.DefaultHeaders,
	)
}

// DiscoverTmdbMovie searches for movies on TheMovieDB API using the provided query string.
// It takes a query string as input and returns a pointer to a TheMovieDBSearch struct containing the search results,
// or an error if the query is empty or the API call fails.
func DiscoverTmdbMovie(query string) (*TheMovieDBSearch, error) {
	if query == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBSearch](
		&tmdbAPI.Client,
		logger.JoinStrings("https://api.themoviedb.org/3/discover/movie?", query),
		tmdbAPI.DefaultHeaders,
	)
}

// DiscoverTmdbSerie searches for TV shows on TheMovieDB API using the provided query string.
// It takes a query string as input and returns a pointer to a TheMovieDBSearchTV struct containing the search results,
// or an error if the query is empty or the API call fails.
func DiscoverTmdbSerie(query string) (*TheMovieDBSearchTV, error) {
	if query == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBSearchTV](
		&tmdbAPI.Client,
		logger.JoinStrings("https://api.themoviedb.org/3/discover/tv?", query),
		tmdbAPI.DefaultHeaders,
	)
}

// GetTmdbList retrieves a list of items from TheMovieDB API by the given list ID.
// It takes an integer list ID as input and returns a TheMovieDBList struct containing the list data.
// If the initial API response does not contain all the list items, it will make additional requests to fetch the remaining pages.
// Returns an error if the API call fails.
func GetTmdbList(listid int) (TheMovieDBList, error) {
	retdata, err := doJSONType[TheMovieDBList](
		&tmdbAPI.Client,
		logger.JoinStrings("https://api.themoviedb.org/3/list/", strconv.Itoa(listid), "&page=1"),
		tmdbAPI.DefaultHeaders,
	)
	if err != nil {
		return TheMovieDBList{}, err
	}
	if len(retdata.Items) < 20 {
		return retdata, err
	}
	pagesize := len(retdata.Items)
	totalPages := retdata.ItemCount / pagesize
	for i := 2; i <= totalPages; i++ {
		listadd, err := doJSONTypeNoLimit[TheMovieDBList](
			&tmdbAPI.Client,
			logger.JoinStrings(
				"https://api.themoviedb.org/3/list/",
				strconv.Itoa(listid),
				"?page=",
				strconv.Itoa(i),
			),
			tmdbAPI.DefaultHeaders,
		)
		if err != nil {
			continue
		}
		retdata.Items = append(retdata.Items, listadd.Items...)
	}
	return retdata, err
}

// RemoveFromTmdbList removes an item from a TheMovieDB list by the given list ID and item ID.
// It takes the list ID and the ID of the item to remove as input, and returns an error if the operation fails.
func RemoveFromTmdbList(listid int, remove int) error {
	body := strings.NewReader(fmt.Sprintf(`{"media_id":%d}`, remove))
	return ProcessHTTP(
		&tmdbAPI.Client,
		logger.JoinStrings(
			"https://api.themoviedb.org/3/list/",
			strconv.Itoa(listid),
			"/remove_item",
		),
		true,
		nil,
		tmdbAPI.DefaultHeaders,
		body,
	)
}

// FindTmdbImdb searches TheMovieDB API to find a movie based on its IMDb ID.
// It takes an IMDb ID string as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the IMDb ID is empty.
func FindTmdbImdb(imdbid string) (*TheMovieDBFind, error) {
	if imdbid == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBFind](
		&tmdbAPI.Client,
		logger.JoinStrings(
			"https://api.themoviedb.org/3/find/",
			imdbid,
			"?language=en-US&external_source=imdb_id",
		),
		tmdbAPI.DefaultHeaders,
	)
}

// FindTmdbTvdb searches TheMovieDB API to find a TV show based on its TheTVDB ID.
// It takes a TheTVDB ID int as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the TheTVDB ID is 0.
func FindTmdbTvdb(thetvdbid int) (*TheMovieDBFind, error) {
	if thetvdbid == 0 {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBFind](
		&tmdbAPI.Client,
		logger.JoinStrings(
			"https://api.themoviedb.org/3/find/",
			strconv.Itoa(thetvdbid),
			"?language=en-US&external_source=tvdb_id",
		),
		tmdbAPI.DefaultHeaders,
	)
}

// GetTmdbMovie retrieves movie details from TheMovieDB API by movie ID.
// It takes an integer movie ID as input and returns a TheMovieDBMovie struct containing the movie details.
// Returns an error if the ID is invalid or the API call fails.
func GetTmdbMovie(id int) (*TheMovieDBMovie, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBMovie](
		&tmdbAPI.Client,
		logger.JoinStrings(apiurltmdbmovies, strconv.Itoa(id)),
		tmdbAPI.DefaultHeaders,
	)
}

// GetTmdbMovieTitles retrieves the alternative titles for a TMDb movie by ID.
// It returns a TheMovieDBMovieTitles struct containing the titles,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieTitles(id int) (*TheMovieDBMovieTitles, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBMovieTitles](
		&tmdbAPI.Client,
		logger.JoinStrings(apiurltmdbmovies, strconv.Itoa(id), "/alternative_titles"),
		tmdbAPI.DefaultHeaders,
	)
}

// GetTmdbMovieExternal retrieves the external IDs for a TMDb movie by ID.
// It returns a TheMovieDBTVExternal struct containing the external IDs,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieExternal(id int) (*TheMovieDBTVExternal, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBTVExternal](
		&tmdbAPI.Client,
		logger.JoinStrings(apiurltmdbmovies, strconv.Itoa(id), "/external_ids"),
		tmdbAPI.DefaultHeaders,
	)
}

// GetTVExternal retrieves the external IDs for a TV show from TheMovieDB.
// It takes the ID of the TV show and returns a pointer to a TheMovieDBTVExternal struct containing the external IDs.
// Returns an error if the ID is invalid or the API call fails.
func GetTVExternal(id int) (*TheMovieDBTVExternal, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TheMovieDBTVExternal](
		&tmdbAPI.Client,
		logger.JoinStrings("https://api.themoviedb.org/3/tv/", strconv.Itoa(id), "/external_ids"),
		tmdbAPI.DefaultHeaders,
	)
}

// TestTMDBConnectivity tests the connectivity to the TMDB API
// Returns status code and error if any
func TestTMDBConnectivity(timeout time.Duration) (int, error) {
	// Check if client is initialized
	if tmdbAPI.APIKey == "" {
		return 0, fmt.Errorf("TMDB API client not initialized or missing API key")
	}

	statusCode := 0
	err := ProcessHTTPNoRateCheck(
		&tmdbAPI.Client,
		"https://api.themoviedb.org/3/search/movie?query=test",
		func(ctx context.Context, resp *http.Response) error {
			statusCode = resp.StatusCode
			return nil
		},
		tmdbAPI.DefaultHeaders,
	)
	return statusCode, err
}
