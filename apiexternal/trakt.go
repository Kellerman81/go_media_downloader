package apiexternal

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"golang.org/x/time/rate"
)

type TraktMovies struct {
}

type TraktMovieAnticipated struct {
	ListCount int        `json:"list_count"`
	Movie     TraktMovie `json:"movie"`
}

type TraktMovieTrending struct {
	Watchers int        `json:"watchers"`
	Movie    TraktMovie `json:"movie"`
}

type TraktMovie struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   struct {
		Trakt int    `json:"trakt"`
		Slug  string `json:"slug"`
		Imdb  string `json:"imdb"`
		Tmdb  int    `json:"tmdb"`
	} `json:"ids"`
}
type TraktSerieSeason struct {
	Number int `json:"number"`
	Ids    struct {
		Trakt  int    `json:"trakt"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
		Imdb   string `json:"imdb"`
		Tmdb   int    `json:"tmdb"`
	} `json:"ids"`
}
type TraktSerieSeasonEpisodes struct {
	Season  int    `json:"season"`
	Episode int    `json:"number"`
	Title   string `json:"title"`
	Ids     struct {
		Trakt  int    `json:"trakt"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
		Imdb   string `json:"imdb"`
		Tmdb   int    `json:"tmdb"`
	} `json:"ids"`
	EpisodeAbs            int       `json:"number_abs"`
	Overview              string    `json:"overview"`
	Rating                float32   `json:"rating"`
	Votes                 int       `json:"votes"`
	Comments              int       `json:"comment_count"`
	AvailableTranslations []string  `json:"available_translations"`
	Runtime               int       `json:"runtime"`
	FirstAired            time.Time `json:"first_aired"`
}

type TraktSerieData struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   struct {
		Trakt  int    `json:"trakt"`
		Slug   string `json:"slug"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
		Imdb   string `json:"imdb"`
		Tmdb   int    `json:"tmdb"`
	} `json:"ids"`
	Overview              string    `json:"overview"`
	FirstAired            time.Time `json:"first_aired"`
	Runtime               int       `json:"runtime"`
	Certification         string    `json:"certification"`
	Network               string    `json:"network"`
	Country               string    `json:"country"`
	Trailer               string    `json:"trailer"`
	Homepage              string    `json:"homepage"`
	Status                string    `json:"status"`
	Rating                float32   `json:"rating"`
	Votes                 int       `json:"votes"`
	Comments              int       `json:"comment_count"`
	Language              string    `json:"language"`
	AvailableTranslations []string  `json:"available_translations"`
	Genres                []string  `json:"genres"`
	AiredEpisodes         int       `json:"aired_episodes"`
}
type TraktShowAliases struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type TraktMovieAliases struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type TraktMovieExtend struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   struct {
		Trakt int    `json:"trakt"`
		Slug  string `json:"slug"`
		Imdb  string `json:"imdb"`
		Tmdb  int    `json:"tmdb"`
	} `json:"ids"`
	Tagline               string   `json:"tagline"`
	Overview              string   `json:"overview"`
	Released              string   `json:"released"`
	Runtime               int      `json:"runtime"`
	Country               string   `json:"country"`
	Trailer               string   `json:"trailer"`
	Homepage              string   `json:"homepage"`
	Status                string   `json:"status"`
	Rating                float32  `json:"rating"`
	Votes                 int      `json:"votes"`
	Comments              int      `json:"comment_count"`
	Language              string   `json:"language"`
	AvailableTranslations []string `json:"available_translations"`
	Genres                []string `json:"genres"`
	Certification         string   `json:"certification"`
}

type TraktClient struct {
	ApiKey string
	Client *RLHTTPClient
}

var TraktApi TraktClient

func NewTraktClient(apikey string, seconds int, calls int) {
	rl := rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls) // 50 request every 10 seconds
	limiter, _ := slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })
	TraktApi = TraktClient{ApiKey: apikey, Client: NewClient(rl, limiter)}
}

func (t TraktClient) GetMoviePopular(limit int) ([]TraktMovie, error) {
	url := "https://api.trakt.tv/movies/popular"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktMovie{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktMovie{}, err
	}
	result := make([]TraktMovie, 0, limit)
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetMovieTrending(limit int) ([]TraktMovieTrending, error) {
	url := "https://api.trakt.tv/movies/trending"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktMovieTrending{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktMovieTrending{}, err
	}
	result := make([]TraktMovieTrending, 0, limit)
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetMovieAnticipated(limit int) ([]TraktMovieAnticipated, error) {
	url := "https://api.trakt.tv/movies/anticipated"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktMovieAnticipated{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktMovieAnticipated{}, err
	}
	result := make([]TraktMovieAnticipated, 0, limit)
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetMovieAliases(movieid string) ([]TraktMovieAliases, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "/aliases"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktMovieAliases{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktMovieAliases{}, err
	}
	result := make([]TraktMovieAliases, 0, 10)
	json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TraktClient) GetMovie(movieid string) (TraktMovieExtend, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "?extended=full"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return TraktMovieExtend{}, err
	}
	if resp.StatusCode == 429 {
		return TraktMovieExtend{}, err
	}
	result := TraktMovieExtend{}
	json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TraktClient) GetSerie(movieid string) (TraktSerieData, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "?extended=full"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return TraktSerieData{}, err
	}
	if resp.StatusCode == 429 {
		return TraktSerieData{}, err
	}
	result := TraktSerieData{}
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetSerieAliases(movieid string) ([]TraktShowAliases, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/aliases"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktShowAliases{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktShowAliases{}, err
	}
	result := make([]TraktShowAliases, 0, 10)
	json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TraktClient) GetSerieSeasons(movieid string) ([]TraktSerieSeason, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/seasons"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktSerieSeason{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktSerieSeason{}, err
	}
	result := make([]TraktSerieSeason, 0, 10)
	json.Unmarshal(responseData, &result)
	return result, nil
}
func (t TraktClient) GetSerieSeasonEpisodes(movieid string, season int) ([]TraktSerieSeasonEpisodes, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/seasons/" + strconv.Itoa(season) + "?extended=full"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktSerieSeasonEpisodes{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktSerieSeasonEpisodes{}, err
	}
	result := make([]TraktSerieSeasonEpisodes, 0, 10)
	json.Unmarshal(responseData, &result)
	return result, nil
}
