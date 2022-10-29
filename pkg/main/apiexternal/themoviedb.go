package apiexternal

import (
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/rate"
	"go.uber.org/zap"
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

type TheMovieDBMovie struct {
	Adult  bool `json:"adult"`
	Budget int  `json:"budget"`
	Genres []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ID               int     `json:"id"`
	ImdbID           string  `json:"imdb_id"`
	OriginalLanguage string  `json:"original_language"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	Popularity       float32 `json:"popularity"`
	ReleaseDate      string  `json:"release_date"`
	Revenue          int     `json:"revenue"`
	Runtime          int     `json:"runtime"`
	SpokenLanguages  []struct {
		EnglishName string `json:"english_name"`
		Name        string `json:"name"`
		Iso6391     string `json:"iso_639_1"`
	} `json:"spoken_languages"`
	Status      string  `json:"status"`
	Tagline     string  `json:"tagline"`
	Title       string  `json:"title"`
	VoteAverage float32 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	Backdrop    string  `json:"backdrop_path"`
	Poster      string  `json:"poster_path"`
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
	ApiKey string
	Client *RLHTTPClient
}

var TmdbApi tmdbClient

func NewTmdbClient(apikey string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TmdbApi = tmdbClient{
		ApiKey: apikey,
		Client: NewClient(
			disabletls,
			rate.New(calls, 0, time.Duration(seconds)*time.Second), timeoutseconds)}

}

func (t *tmdbClient) SearchMovie(name string) (TheMovieDBSearch, error) {
	url := "https://api.themoviedb.org/3/search/movie?api_key=" + t.ApiKey + "&query=" + url.PathEscape(name)
	var result TheMovieDBSearch
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheMovieDBSearch{}, err
	}

	return result, nil
}

func (t *tmdbClient) SearchTV(name string) (TheMovieDBSearchTV, error) {
	url := "https://api.themoviedb.org/3/search/tv?api_key=" + t.ApiKey + "&query=" + url.PathEscape(name)
	var result TheMovieDBSearchTV
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheMovieDBSearchTV{}, err
	}

	return result, nil
}

func (t *tmdbClient) FindImdb(imdbid string) (TheMovieDBFind, error) {
	url := "https://api.themoviedb.org/3/find/" + imdbid + "?api_key=" + t.ApiKey + "&language=en-US&external_source=imdb_id"
	var result TheMovieDBFind
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheMovieDBFind{}, err
	}

	return result, nil
}
func (t *tmdbClient) FindTvdb(thetvdbid string) (TheMovieDBFind, error) {
	url := "https://api.themoviedb.org/3/find/" + thetvdbid + "?api_key=" + t.ApiKey + "&language=en-US&external_source=tvdb_id"
	var result TheMovieDBFind
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheMovieDBFind{}, err
	}

	return result, nil
}
func (t *tmdbClient) GetMovie(id string) (TheMovieDBMovie, error) {
	url := "https://api.themoviedb.org/3/movie/" + id + "?api_key=" + t.ApiKey
	var result TheMovieDBMovie
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheMovieDBMovie{}, err
	}
	return result, nil
}
func (t *tmdbClient) GetMovieTitles(id string) (*TheMovieDBMovieTitles, error) {
	url := "https://api.themoviedb.org/3/movie/" + id + "/alternative_titles?api_key=" + t.ApiKey
	var result TheMovieDBMovieTitles
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}
	return &result, nil
}
func (t *tmdbClient) GetMovieExternal(id string) (TheMovieDBTVExternal, error) {
	url := "https://api.themoviedb.org/3/movie/" + id + "/external_ids?api_key=" + t.ApiKey
	var result TheMovieDBTVExternal
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheMovieDBTVExternal{}, err
	}

	return result, nil
}
func (t *tmdbClient) GetTVExternal(id string) (TheMovieDBTVExternal, error) {
	url := "https://api.themoviedb.org/3/tv/" + id + "/external_ids?api_key=" + t.ApiKey
	var result TheMovieDBTVExternal
	_, err := t.Client.DoJson(url, &result, nil)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TheMovieDBTVExternal{}, err
	}

	return result, nil
}