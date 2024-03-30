package apiexternal

import (
	"net/url"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

type theMovieDBSearch struct {
	//TotalPages   int                          `json:"total_pages"`
	//TotalResults int                          `json:"total_results"`
	//Page         int                          `json:"page"`
	Results []theMovieDBFindMovieresults `json:"results"`
}

type theMovieDBSearchTV struct {
	//TotalPages   int                       `json:"total_pages"`
	//TotalResults int                       `json:"total_results"`
	//Page         int                       `json:"page"`
	Results []theMovieDBFindTvresults `json:"results"`
}

type theMovieDBFind struct {
	MovieResults []theMovieDBFindMovieresults `json:"movie_results"`
	TvResults    []theMovieDBFindTvresults    `json:"tv_results"`
}

type theMovieDBFindMovieresults struct {
	VoteAverage      float32 `json:"vote_average"`
	Overview         string  `json:"overview"`
	ReleaseDate      string  `json:"release_date"`
	Adult            bool    `json:"adult"`
	VoteCount        int     `json:"vote_count"`
	Title            string  `json:"title"`
	OriginalLanguage string  `json:"original_language"`
	OriginalTitle    string  `json:"original_title"`
	ID               int     `json:"id"`
	Popularity       float32 `json:"popularity"`
}
type theMovieDBFindTvresults struct {
	//ID               int      `json:"id"`
	OriginalLanguage string   `json:"original_language"`
	FirstAirDate     string   `json:"first_air_date"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	VoteAverage      float32  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	Overview         string   `json:"overview"`
	OriginCountry    []string `json:"origin_Country"`
	Popularity       float32  `json:"popularity"`
}

type theMovieDBMovieGenres struct {
	//ID   int    `json:"id"`
	Name string `json:"name"`
}

type theMovieDBMovieSpokenLanguages struct {
	EnglishName string `json:"english_name"`
	Name        string `json:"name"`
	Iso6391     string `json:"iso_639_1"`
}

type theMovieDBMovie struct {
	Adult            bool                             `json:"adult"`
	Budget           int                              `json:"budget"`
	Genres           []theMovieDBMovieGenres          `json:"genres"`
	ID               int                              `json:"id"`
	ImdbID           string                           `json:"imdb_id"`
	OriginalLanguage string                           `json:"original_language"`
	OriginalTitle    string                           `json:"original_title"`
	Overview         string                           `json:"overview"`
	Popularity       float32                          `json:"popularity"`
	ReleaseDate      string                           `json:"release_date"`
	Revenue          int                              `json:"revenue"`
	Runtime          int                              `json:"runtime"`
	SpokenLanguages  []theMovieDBMovieSpokenLanguages `json:"spoken_languages"`
	Status           string                           `json:"status"`
	Tagline          string                           `json:"tagline"`
	Title            string                           `json:"title"`
	VoteAverage      float32                          `json:"vote_average"`
	VoteCount        int                              `json:"vote_count"`
	Backdrop         string                           `json:"backdrop_path"`
	Poster           string                           `json:"poster_path"`
}

type theMovieDBMovieTitles struct {
	//ID     int                         `json:"id"`
	Titles []theMovieDBMovieTitlesList `json:"titles"`
}

type theMovieDBMovieTitlesList struct {
	TmdbType string `json:"type"`
	Title    string `json:"title"`
	Iso31661 string `json:"iso_3166_1"`
}

type theMovieDBTVExternal struct {
	ID          int    `json:"id"`
	ImdbID      string `json:"imdb_id"`
	FreebaseMID string `json:"freebase_mid"`
	FreebaseID  string `json:"freebase_id"`
	TvdbID      int    `json:"tvdb_id"`
	TvrageID    int    `json:"tvrage_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

// tmdbClient is a struct for interacting with the TMDB API
// It contains fields for the API key, query parameter API key,
// and a pointer to the rate limited HTTP client
type tmdbClient struct {
	APIKey  string       // The TMDB API key
	QAPIKey string       // The query parameter API key
	Client  rlHTTPClient // Pointer to the rate limited HTTP client
}

// Close releases the resources held by a TheMovieDBMovieTitles struct.
// It sets the Titles field to nil if it has a capacity >= 1 to release the
// underlying array. It also calls logger.Clear on the struct to release any
// resources held by the logger.
func (t *theMovieDBMovieTitles) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = theMovieDBMovieTitles{}
}

func (t *theMovieDBTVExternal) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = theMovieDBTVExternal{}
}

// Close releases the memory allocated to the TheMovieDBMovie struct.
// It checks if the struct is nil and returns immediately if so.
// It then checks if the Genres and SpokenLanguages slices have a
// capacity >= 1, and sets them to nil if so to release the backing
// array memory.
// Finally it calls logger.Clear() on the struct to clear any logged errors.
func (t *theMovieDBMovie) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = theMovieDBMovie{}
}

// Close releases the memory allocated to the TheMovieDBFind struct by
// setting slices to nil and clearing pointers. This should be called after
// the data from the struct is no longer needed.
func (t *theMovieDBFind) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = theMovieDBFind{}
}

// Close releases the resources held by a TheMovieDBSearchTV struct by
// setting the Results field to nil if it has a capacity >= 1.
// It also calls logger.Clear on the struct to release any resources held by the logger.
func (t *theMovieDBSearchTV) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = theMovieDBSearchTV{}
}

// Close releases the memory allocated to the TheMovieDBSearch struct.
// It checks if the struct is nil and returns immediately if so.
// It then checks if the Results slice has a capacity >= 1, and sets it to nil if so
// to release the backing array memory.
// Finally it calls logger.Clear() on the struct to clear any logged errors.
func (t *theMovieDBSearch) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = theMovieDBSearch{}
}

// NewTmdbClient creates a new TMDb client for making API requests.
// It takes the TMDb API key, rate limiting settings, TLS setting, and timeout.
// Returns a tmdbClient instance configured with the provided settings.
func NewTmdbClient(apikey string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	tmdbAPI = tmdbClient{
		APIKey:  apikey,
		QAPIKey: "?api_key=" + apikey,
		Client: NewClient(
			"tmdb",
			disabletls,
			true,
			slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
			false, slidingwindow.NewLimiter(10*time.Second, 10), timeoutseconds)}
}

// SearchTmdbMovie searches for movies on TheMovieDB API by movie name.
// It takes a movie name string as input and returns a pointer to a TheMovieDBSearch struct containing the search results,
// or an error if the name is empty or the API call fails.
func SearchTmdbMovie(name string) ([]theMovieDBFindMovieresults, error) {
	if name == "" || tmdbAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	//return DoJSONType[theMovieDBSearch](tmdbAPI.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/movie", tmdbAPI.QAPIKey, "&query=", url.QueryEscape(name)), nil)
	arr, err := DoJSONType[theMovieDBSearch](&tmdbAPI.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/movie", tmdbAPI.QAPIKey, "&query=", url.QueryEscape(name)))
	return arr.Results, err
}

// SearchTmdbTV searches for TV shows on TheMovieDB API.
// It takes a search query string and returns a TheMovieDBSearchTV struct containing the search results.
// Returns ErrNotFound error if the search query is empty.
func SearchTmdbTV(name string) ([]theMovieDBFindTvresults, error) {
	if name == "" || tmdbAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}

	//return DoJSONType[theMovieDBSearchTV](tmdbAPI.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/tv", tmdbAPI.QAPIKey, "&query=", url.QueryEscape(name)), nil)
	arr, err := DoJSONType[theMovieDBSearchTV](&tmdbAPI.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/tv", tmdbAPI.QAPIKey, "&query=", url.QueryEscape(name)))
	return arr.Results, err
}

// FindTmdbImdb searches TheMovieDB API to find a movie based on its IMDb ID.
// It takes an IMDb ID string as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the IMDb ID is empty.
func FindTmdbImdb(imdbid string) ([]theMovieDBFindMovieresults, error) {
	if imdbid == "" || tmdbAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	arr, err := DoJSONType[theMovieDBFind](&tmdbAPI.Client, logger.JoinStrings(logger.URLJoinPath("https://api.themoviedb.org/3/find/", imdbid), tmdbAPI.QAPIKey, "&language=en-US&external_source=imdb_id"))
	return arr.MovieResults, err
}

// FindTmdbTvdb searches TheMovieDB API to find a TV show based on its TheTVDB ID.
// It takes a TheTVDB ID int as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the TheTVDB ID is 0.
func FindTmdbTvdb(thetvdbid int) ([]theMovieDBFindTvresults, error) {
	if thetvdbid == 0 || tmdbAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	arr, err := DoJSONType[theMovieDBFind](&tmdbAPI.Client, logger.JoinStrings(logger.URLJoinPath("https://api.themoviedb.org/3/find/", strconv.Itoa(thetvdbid)), tmdbAPI.QAPIKey, "&language=en-US&external_source=tvdb_id"))
	return arr.TvResults, err
}

// GetTmdbMovie retrieves movie details from TheMovieDB API by movie ID.
// It takes an integer movie ID as input and returns a TheMovieDBMovie struct containing the movie details.
// Returns an error if the ID is invalid or the API call fails.
func GetTmdbMovie(id int) (theMovieDBMovie, error) {
	if id == 0 || tmdbAPI.Client.checklimiterwithdaily() {
		return theMovieDBMovie{}, logger.ErrNotFound
	}
	return DoJSONType[theMovieDBMovie](&tmdbAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurltmdbmovies, strconv.Itoa(id)), tmdbAPI.QAPIKey))
}

// GetTmdbMovieTitles retrieves the alternative titles for a TMDb movie by ID.
// It returns a TheMovieDBMovieTitles struct containing the titles,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieTitles(id int) ([]theMovieDBMovieTitlesList, error) {
	if id == 0 || tmdbAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	arr, err := DoJSONType[theMovieDBMovieTitles](&tmdbAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurltmdbmovies, strconv.Itoa(id), "alternative_titles"), tmdbAPI.QAPIKey))
	return arr.Titles, err
}

// GetTmdbMovieExternal retrieves the external IDs for a TMDb movie by ID.
// It returns a TheMovieDBTVExternal struct containing the external IDs,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieExternal(id int) (theMovieDBTVExternal, error) {
	if id == 0 || tmdbAPI.Client.checklimiterwithdaily() {
		return theMovieDBTVExternal{}, logger.ErrNotFound
	}
	return DoJSONType[theMovieDBTVExternal](&tmdbAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurltmdbmovies, strconv.Itoa(id), "external_ids"), tmdbAPI.QAPIKey))
}

// GetTVExternal retrieves the external IDs for a TV show from TheMovieDB.
// It takes the ID of the TV show and returns a pointer to a TheMovieDBTVExternal struct containing the external IDs.
// Returns an error if the ID is invalid or the API call fails.
func GetTVExternal(id int) (theMovieDBTVExternal, error) {
	if id == 0 || tmdbAPI.Client.checklimiterwithdaily() {
		return theMovieDBTVExternal{}, logger.ErrNotFound
	}
	return DoJSONType[theMovieDBTVExternal](&tmdbAPI.Client, logger.JoinStrings(logger.URLJoinPath("https://api.themoviedb.org/3/tv/", strconv.Itoa(id), "external_ids"), tmdbAPI.QAPIKey))
}
