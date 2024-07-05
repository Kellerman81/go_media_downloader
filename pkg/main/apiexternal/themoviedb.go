package apiexternal

import (
	"net/url"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

type TheMovieDBSearch struct {
	//TotalPages   int                          `json:"total_pages"`
	//TotalResults int                          `json:"total_results"`
	//Page         int                          `json:"page"`
	Results []TheMovieDBFindMovieresults `json:"results"`
}

type TheMovieDBSearchTV struct {
	//TotalPages   int                       `json:"total_pages"`
	//TotalResults int                       `json:"total_results"`
	//Page         int                       `json:"page"`
	Results []TheMovieDBFindTvresults `json:"results"`
}

type TheMovieDBFind struct {
	MovieResults []TheMovieDBFindMovieresults `json:"movie_results"`
	TvResults    []TheMovieDBFindTvresults    `json:"tv_results"`
}

type TheMovieDBFindMovieresults struct {
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
type TheMovieDBFindTvresults struct {
	//ID               int      `json:"id"`
	OriginalLanguage string  `json:"original_language"`
	FirstAirDate     string  `json:"first_air_date"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	VoteAverage      float32 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	Overview         string  `json:"overview"`
	//OriginCountry    []string `json:"origin_Country"`
	Popularity float32 `json:"popularity"`
}

type TheMovieDBMovieGenres struct {
	//ID   int    `json:"id"`
	Name string `json:"name"`
}

type TheMovieDBMovieSpokenLanguages struct {
	EnglishName string `json:"english_name"`
	Name        string `json:"name"`
	Iso6391     string `json:"iso_639_1"`
}

type TheMovieDBMovie struct {
	Adult            bool                             `json:"adult"`
	Budget           int                              `json:"budget"`
	Genres           []TheMovieDBMovieGenres          `json:"genres"`
	ID               int                              `json:"id"`
	ImdbID           string                           `json:"imdb_id"`
	OriginalLanguage string                           `json:"original_language"`
	OriginalTitle    string                           `json:"original_title"`
	Overview         string                           `json:"overview"`
	Popularity       float32                          `json:"popularity"`
	ReleaseDate      string                           `json:"release_date"`
	Revenue          int                              `json:"revenue"`
	Runtime          int                              `json:"runtime"`
	SpokenLanguages  []TheMovieDBMovieSpokenLanguages `json:"spoken_languages"`
	Status           string                           `json:"status"`
	Tagline          string                           `json:"tagline"`
	Title            string                           `json:"title"`
	VoteAverage      float32                          `json:"vote_average"`
	VoteCount        int32                            `json:"vote_count"`
	Backdrop         string                           `json:"backdrop_path"`
	Poster           string                           `json:"poster_path"`
}

type TheMovieDBMovieTitles struct {
	//ID     int                         `json:"id"`
	Titles []TheMovieDBMovieTitlesList `json:"titles"`
}

type TheMovieDBMovieTitlesList struct {
	TmdbType string `json:"type"`
	Title    string `json:"title"`
	Iso31661 string `json:"iso_3166_1"`
}

type TheMovieDBTVExternal struct {
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
	APIKey         string       // The TMDB API key
	QAPIKey        string       // The query parameter API key
	Client         rlHTTPClient // Pointer to the rate limited HTTP client
	DefaultHeaders []string     // Default headers to send with requests
}

// NewTmdbClient creates a new TMDb client for making API requests.
// It takes the TMDb API key, rate limiting settings, TLS setting, and timeout.
// Returns a tmdbClient instance configured with the provided settings.
func NewTmdbClient(apikey string, seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	tmdbApidata = apidata{
		apikey:         apikey,
		disabletls:     disabletls,
		seconds:        seconds,
		calls:          calls,
		timeoutseconds: timeoutseconds,
		limiter:        slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		dailylimiter:   slidingwindow.NewLimiter(10*time.Second, 10),
	}
}

// SearchTmdbMovie searches for movies on TheMovieDB API by movie name.
// It takes a movie name string as input and returns a pointer to a TheMovieDBSearch struct containing the search results,
// or an error if the name is empty or the API call fails.
func SearchTmdbMovie(name string) (TheMovieDBSearch, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if name == "" || p.Client.checklimiterwithdaily() {
		return TheMovieDBSearch{}, logger.ErrNotFound
	}
	//return doJSONType[theMovieDBSearch](p.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/movie", p.QAPIKey, "&query=", url.QueryEscape(name)), nil)
	return doJSONTypeHeader[TheMovieDBSearch](&p.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/movie?query=", url.QueryEscape(name)), p.DefaultHeaders)
}

// SearchTmdbTV searches for TV shows on TheMovieDB API.
// It takes a search query string and returns a TheMovieDBSearchTV struct containing the search results.
// Returns ErrNotFound error if the search query is empty.
func SearchTmdbTV(name string) (TheMovieDBSearchTV, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if name == "" || p.Client.checklimiterwithdaily() {
		return TheMovieDBSearchTV{}, logger.ErrNotFound
	}

	//return doJSONType[theMovieDBSearchTV](p.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/tv", p.QAPIKey, "&query=", url.QueryEscape(name)), nil)
	return doJSONTypeHeader[TheMovieDBSearchTV](&p.Client, logger.JoinStrings("https://api.themoviedb.org/3/search/tv?query=", url.QueryEscape(name)), p.DefaultHeaders)
}

// FindTmdbImdb searches TheMovieDB API to find a movie based on its IMDb ID.
// It takes an IMDb ID string as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the IMDb ID is empty.
func FindTmdbImdb(imdbid string) (TheMovieDBFind, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if imdbid == "" || p.Client.checklimiterwithdaily() {
		return TheMovieDBFind{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings("https://api.themoviedb.org/3/find/", imdbid)
	return doJSONTypeHeader[TheMovieDBFind](&p.Client, logger.JoinStrings(urlv, "?language=en-US&external_source=imdb_id"), p.DefaultHeaders)
}

// FindTmdbTvdb searches TheMovieDB API to find a TV show based on its TheTVDB ID.
// It takes a TheTVDB ID int as input and returns a TheMovieDBFind struct containing the lookup result.
// Returns an ErrNotFound error if the TheTVDB ID is 0.
func FindTmdbTvdb(thetvdbid int) (TheMovieDBFind, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if thetvdbid == 0 || p.Client.checklimiterwithdaily() {
		return TheMovieDBFind{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings("https://api.themoviedb.org/3/find/", strconv.Itoa(thetvdbid))
	return doJSONTypeHeader[TheMovieDBFind](&p.Client, logger.JoinStrings(urlv, "?language=en-US&external_source=tvdb_id"), p.DefaultHeaders)
}

// GetTmdbMovie retrieves movie details from TheMovieDB API by movie ID.
// It takes an integer movie ID as input and returns a TheMovieDBMovie struct containing the movie details.
// Returns an error if the ID is invalid or the API call fails.
func GetTmdbMovie(id int) (TheMovieDBMovie, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if id == 0 || p.Client.checklimiterwithdaily() {
		return TheMovieDBMovie{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings(apiurltmdbmovies, strconv.Itoa(id))
	return doJSONTypeHeader[TheMovieDBMovie](&p.Client, urlv, p.DefaultHeaders)
}

// GetTmdbMovieTitles retrieves the alternative titles for a TMDb movie by ID.
// It returns a TheMovieDBMovieTitles struct containing the titles,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieTitles(id int) (TheMovieDBMovieTitles, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if id == 0 || p.Client.checklimiterwithdaily() {
		return TheMovieDBMovieTitles{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings(apiurltmdbmovies, strconv.Itoa(id), "/alternative_titles")
	return doJSONTypeHeader[TheMovieDBMovieTitles](&p.Client, urlv, p.DefaultHeaders)
}

// GetTmdbMovieExternal retrieves the external IDs for a TMDb movie by ID.
// It returns a TheMovieDBTVExternal struct containing the external IDs,
// or an error if the ID is invalid or the lookup fails.
func GetTmdbMovieExternal(id int) (TheMovieDBTVExternal, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if id == 0 || p.Client.checklimiterwithdaily() {
		return TheMovieDBTVExternal{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings(apiurltmdbmovies, strconv.Itoa(id), "/external_ids")
	return doJSONTypeHeader[TheMovieDBTVExternal](&p.Client, urlv, p.DefaultHeaders)
}

// GetTVExternal retrieves the external IDs for a TV show from TheMovieDB.
// It takes the ID of the TV show and returns a pointer to a TheMovieDBTVExternal struct containing the external IDs.
// Returns an error if the ID is invalid or the API call fails.
func GetTVExternal(id int) (TheMovieDBTVExternal, error) {
	p := pltmdb.Get()
	defer pltmdb.Put(p)
	if id == 0 || p.Client.checklimiterwithdaily() {
		return TheMovieDBTVExternal{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings("https://api.themoviedb.org/3/tv/", strconv.Itoa(id), "/external_ids")
	return doJSONTypeHeader[TheMovieDBTVExternal](&p.Client, urlv, p.DefaultHeaders)
}
