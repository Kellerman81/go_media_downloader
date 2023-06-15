package apiexternal

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
)

type TheMovieDBSearch struct {
	TotalPages   int                          `json:"total_pages"`
	TotalResults int                          `json:"total_results"`
	Page         int                          `json:"page"`
	Results      []TheMovieDBFindMovieresults `json:"results"`
}

type TheMovieDBSearchTV struct {
	TotalPages   int                       `json:"total_pages"`
	TotalResults int                       `json:"total_results"`
	Page         int                       `json:"page"`
	Results      []TheMovieDBFindTvresults `json:"results"`
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
	ID               int      `json:"id"`
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

type TheMovieDBMovieGenres struct {
	ID   int    `json:"id"`
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
	VoteCount        int                              `json:"vote_count"`
	Backdrop         string                           `json:"backdrop_path"`
	Poster           string                           `json:"poster_path"`
}

type TheMovieDBMovieTitles struct {
	ID     int                         `json:"id"`
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

type tmdbClient struct {
	APIKey string
	Client *RLHTTPClient
}

const (
	apiurltmdbmovies = "https://api.themoviedb.org/3/movie/"
	strAPIKey        = "?api_key="
)

var TmdbAPI *tmdbClient

func (t *TheMovieDBMovieTitles) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.Titles)
	logger.ClearVar(t)
}
func (t *TheMovieDBMovie) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.Genres)
	logger.Clear(&t.SpokenLanguages)
	logger.ClearVar(t)
}
func (t *TheMovieDBFind) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.MovieResults)
	logger.Clear(&t.TvResults)
	logger.ClearVar(t)
}
func (t *TheMovieDBSearchTV) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.Results)
	logger.ClearVar(t)
}
func (t *TheMovieDBSearch) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.Results)
	logger.ClearVar(t)
}

func NewTmdbClient(apikey string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TmdbAPI = &tmdbClient{
		APIKey: apikey,
		Client: NewClient(
			disabletls,
			true,
			slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
			false, slidingwindow.NewLimiter(10*time.Second, 10), timeoutseconds)}

}

func (t *tmdbClient) SearchMovie(name *string) (*TheMovieDBSearch, error) {
	if *name == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TheMovieDBSearch](t.Client, "https://api.themoviedb.org/3/search/movie?api_key="+t.APIKey+"&query="+QueryEscape(name))
}

func (t *tmdbClient) SearchTV(name string) (*TheMovieDBSearchTV, error) {
	if name == "" {
		return nil, logger.ErrNotFound
	}

	return DoJSONType[TheMovieDBSearchTV](t.Client, "https://api.themoviedb.org/3/search/tv?api_key="+t.APIKey+"&query="+QueryEscape(&name))
}

func (t *tmdbClient) FindImdb(imdbid string) (*TheMovieDBFind, error) {
	if imdbid == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TheMovieDBFind](t.Client, "https://api.themoviedb.org/3/find/"+imdbid+strAPIKey+t.APIKey+"&language=en-US&external_source=imdb_id")
}
func (t *tmdbClient) FindTvdb(thetvdbid int) (*TheMovieDBFind, error) {
	if thetvdbid == 0 {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TheMovieDBFind](t.Client, "https://api.themoviedb.org/3/find/"+logger.IntToString(thetvdbid)+strAPIKey+t.APIKey+"&language=en-US&external_source=tvdb_id")
}
func (t *tmdbClient) GetMovie(id int) (*TheMovieDBMovie, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TheMovieDBMovie](t.Client, apiurltmdbmovies+logger.IntToString(id)+strAPIKey+t.APIKey)
}
func (t *tmdbClient) GetMovieTitles(id int) (*TheMovieDBMovieTitles, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TheMovieDBMovieTitles](t.Client, apiurltmdbmovies+logger.IntToString(id)+"/alternative_titles?api_key="+t.APIKey)
}
func (t *tmdbClient) GetMovieExternal(id int) (*TheMovieDBTVExternal, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TheMovieDBTVExternal](t.Client, apiurltmdbmovies+logger.IntToString(id)+"/external_ids?api_key="+t.APIKey)
}
func (t *tmdbClient) GetTVExternal(id string) (*TheMovieDBTVExternal, error) {
	if id == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TheMovieDBTVExternal](t.Client, "https://api.themoviedb.org/3/tv/"+id+"/external_ids?api_key="+t.APIKey)
}
