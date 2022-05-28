package apiexternal

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/slidingwindow"
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
type theMovieDBSearchTV struct {
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

type theMovieDBMovie struct {
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

type theMovieDBMovieTitles struct {
	ID     int                         `json:"id"`
	Titles []theMovieDBMovieTitlesList `json:"titles"`
}
type theMovieDBMovieTitlesList struct {
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

var TmdbApi *tmdbClient

func NewTmdbClient(apikey string, seconds int, calls int, disabletls bool) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TmdbApi = &tmdbClient{
		ApiKey: apikey,
		Client: NewClient(
			disabletls,
			rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls),
			slidingwindow.NewLimiterNoStop(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() }))}

}

func (t *tmdbClient) SearchMovie(name string) (TheMovieDBSearch, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/search/movie?api_key="+t.ApiKey+"&query="+url.PathEscape(name), nil)
	if err != nil {
		return TheMovieDBSearch{}, err
	}

	var result TheMovieDBSearch

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return TheMovieDBSearch{}, err
	}

	return result, nil
}

func (t *tmdbClient) SearchTV(name string) (theMovieDBSearchTV, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/search/tv?api_key="+t.ApiKey+"&query="+url.PathEscape(name), nil)
	if err != nil {
		return theMovieDBSearchTV{}, err
	}

	var result theMovieDBSearchTV

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return theMovieDBSearchTV{}, err
	}

	return result, nil
}

func (t *tmdbClient) FindImdb(imdbid string) (theMovieDBFind, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/find/"+imdbid+"?api_key="+t.ApiKey+"&language=en-US&external_source=imdb_id", nil)
	if err != nil {
		return theMovieDBFind{}, err
	}

	var result theMovieDBFind

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return theMovieDBFind{}, err
	}

	return result, nil
}
func (t *tmdbClient) FindTvdb(thetvdbid int) (theMovieDBFind, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/find/"+strconv.Itoa(thetvdbid)+"?api_key="+t.ApiKey+"&language=en-US&external_source=tvdb_id", nil)
	if err != nil {
		return theMovieDBFind{}, err
	}

	var result theMovieDBFind

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return theMovieDBFind{}, err
	}

	return result, nil
}
func (t *tmdbClient) GetMovie(id int) (theMovieDBMovie, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/movie/"+strconv.Itoa(id)+"?api_key="+t.ApiKey, nil)
	if err != nil {
		return theMovieDBMovie{}, err
	}

	var result theMovieDBMovie

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return theMovieDBMovie{}, err
	}

	return result, nil
}
func (t *tmdbClient) GetMovieTitles(id int) (theMovieDBMovieTitles, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/movie/"+strconv.Itoa(id)+"/alternative_titles?api_key="+t.ApiKey, nil)
	if err != nil {
		return theMovieDBMovieTitles{}, err
	}

	var result theMovieDBMovieTitles

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return theMovieDBMovieTitles{}, err
	}

	return result, nil
}
func (t *tmdbClient) GetMovieExternal(id int) (TheMovieDBTVExternal, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/movie/"+strconv.Itoa(id)+"/external_ids?api_key="+t.ApiKey, nil)
	if err != nil {
		return TheMovieDBTVExternal{}, err
	}

	var result TheMovieDBTVExternal
	err = t.Client.DoJson(req, &result)

	if err != nil {
		return TheMovieDBTVExternal{}, err
	}

	return result, nil
}
func (t *tmdbClient) GetTVExternal(id int) (TheMovieDBTVExternal, error) {
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/tv/"+strconv.Itoa(id)+"/external_ids?api_key="+t.ApiKey, nil)
	if err != nil {
		return TheMovieDBTVExternal{}, err
	}

	var result TheMovieDBTVExternal
	err = t.Client.DoJson(req, &result)

	if err != nil {
		return TheMovieDBTVExternal{}, err
	}

	return result, nil
}
