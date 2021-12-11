package apiexternal

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"golang.org/x/time/rate"
)

type TheMovieDBSearch struct {
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
	Page         int `json:"page"`
	Results      []struct {
		OriginalTitle    string    `json:"original_title"`
		VoteAverage      string    `json:"vote_average"`
		Popularity       string    `json:"popularity"`
		VoteCount        int       `json:"vote_count"`
		ReleaseDate      time.Time `json:"release_date"`
		Title            string    `json:"title"`
		Adult            string    `json:"adult"`
		Overview         string    `json:"overview"`
		ID               int       `json:"id"`
		OriginalLanguage string    `json:"original_language"`
	} `json:"results"`
}
type TheMovieDBSearchTV struct {
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
	Page         int `json:"page"`
	Results      []struct {
		ID               int       `json:"id"`
		OriginalLanguage string    `json:"original_language"`
		FirstAirDate     time.Time `json:"first_air_date"`
		Name             string    `json:"name"`
		OriginalName     string    `json:"original_name"`
		VoteAverage      string    `json:"vote_average"`
		VoteCount        int       `json:"vote_count"`
		Overview         string    `json:"overview"`
		OriginCountry    []string  `json:"origin_Country"`
		Popularity       string    `json:"popularity"`
	} `json:"results"`
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
	ID               int       `json:"id"`
	OriginalLanguage string    `json:"original_language"`
	FirstAirDate     time.Time `json:"first_air_date"`
	Name             string    `json:"name"`
	OriginalName     string    `json:"original_name"`
	VoteAverage      int       `json:"vote_average"`
	VoteCount        int       `json:"vote_count"`
	Overview         string    `json:"overview"`
	OriginCountry    []string  `json:"origin_Country"`
	Popularity       string    `json:"popularity"`
}

type TheMovieDBMovie struct {
	Adult  bool `json:"adult"`
	Budget int  `json:"budget"`
	Genres []struct {
		ID   string `json:"id"`
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
	Type     string `json:"type"`
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

type TmdbClient struct {
	ApiKey string
	Client *RLHTTPClient
}

var TmdbApi TmdbClient

func NewTmdbClient(apikey string, seconds int, calls int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	rl := rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls) // 50 request every 10 seconds
	limiter, _ := slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })
	TmdbApi = TmdbClient{ApiKey: apikey, Client: NewClient(rl, limiter)}
}

func (t TmdbClient) SearchMovie(name string) (TheMovieDBSearch, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/search/movie?api_key="+t.ApiKey+"&query="+url.PathEscape(name), nil)

	var result TheMovieDBSearch
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBSearch{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TmdbClient) SearchTV(name string) (TheMovieDBSearchTV, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/search/tv?api_key="+t.ApiKey+"&query="+url.PathEscape(name), nil)

	var result TheMovieDBSearchTV
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBSearchTV{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TmdbClient) FindImdb(imdbid string) (TheMovieDBFind, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/find/"+imdbid+"?api_key="+t.ApiKey+"&language=en-US&external_source=imdb_id", nil)

	var result TheMovieDBFind
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBFind{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TmdbClient) FindTvdb(thetvdbid int) (TheMovieDBFind, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/find/"+strconv.Itoa(thetvdbid)+"?api_key="+t.ApiKey+"&language=en-US&external_source=tvdb_id", nil)

	var result TheMovieDBFind
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBFind{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TmdbClient) GetMovie(id int) (TheMovieDBMovie, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/movie/"+strconv.Itoa(id)+"?api_key="+t.ApiKey, nil)

	var result TheMovieDBMovie
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBMovie{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TmdbClient) GetMovieTitles(id int) (TheMovieDBMovieTitles, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/movie/"+strconv.Itoa(id)+"/alternative_titles?api_key="+t.ApiKey, nil)

	var result TheMovieDBMovieTitles
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBMovieTitles{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TmdbClient) GetMovieExternal(id int) (TheMovieDBTVExternal, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/movie/"+strconv.Itoa(id)+"/external_ids?api_key="+t.ApiKey, nil)

	var result TheMovieDBTVExternal
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBTVExternal{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TmdbClient) GetTVExternal(id int) (TheMovieDBTVExternal, error) {
	req, _ := http.NewRequest("GET", "https://api.themoviedb.org/3/tv/"+strconv.Itoa(id)+"/external_ids?api_key="+t.ApiKey, nil)

	var result TheMovieDBTVExternal
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return TheMovieDBTVExternal{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
