package apiexternal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"golang.org/x/oauth2"
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

type TraktUserList struct {
	Rank  int        `json:"rank"`
	Id    int        `json:"id"`
	Notes string     `json:"notes"`
	Type  string     `json:"type"`
	Movie TraktMovie `json:"movie"`
	Serie TraktSerie `json:"show"`
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
type TraktSerieTrending struct {
	Watchers int        `json:"watchers"`
	Serie    TraktSerie `json:"show"`
}
type TraktSerieAnticipated struct {
	ListCount int        `json:"list_count"`
	Serie     TraktSerie `json:"show"`
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

type TraktSerie struct {
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
	ApiKey       string
	ClientID     string
	ClientSecret string
	Client       *RLHTTPClient
	Auth         *oauth2.Config
	Token        *oauth2.Token
}

var TraktApi TraktClient

func NewTraktClient(clientid string, clientsecret string, token oauth2.Token, seconds int, calls int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	rl := rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls) // 50 request every 10 seconds
	limiter, _ := slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })
	conf := &oauth2.Config{
		ClientID:     clientid,
		ClientSecret: clientsecret,
		RedirectURL:  "http://localhost:9090",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://api.trakt.tv/oauth/authorize",
			TokenURL: "https://api.trakt.tv/oauth/token",
		},
	}
	TraktApi = TraktClient{
		ApiKey:       clientid,
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Client:       NewClient(rl, limiter),
		Auth:         conf,
		Token:        &token}
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

func (t TraktClient) GetUserList(username string, listname string, listtype string, limit int) ([]TraktUserList, error) {
	url := "https://api.trakt.tv/users/" + username + "/lists/" + listname + "/items/" + listtype
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	if t.Token.AccessToken != "" {
		req.Header.Add("Authorization", "Bearer "+t.Token.AccessToken)
	}
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktUserList{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktUserList{}, err
	}
	var result []TraktUserList
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetSeriePopular(limit int) ([]TraktSerie, error) {
	url := "https://api.trakt.tv/shows/popular"
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
		return []TraktSerie{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktSerie{}, err
	}
	result := make([]TraktSerie, 0, limit)
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetSerieTrending(limit int) ([]TraktSerieTrending, error) {
	url := "https://api.trakt.tv/shows/trending"
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
		return []TraktSerieTrending{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktSerieTrending{}, err
	}
	result := make([]TraktSerieTrending, 0, limit)
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetSerieAnticipated(limit int) ([]TraktSerieAnticipated, error) {
	url := "https://api.trakt.tv/shows/anticipated"
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
		return []TraktSerieAnticipated{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktSerieAnticipated{}, err
	}
	result := make([]TraktSerieAnticipated, 0, limit)
	json.Unmarshal(responseData, &result)
	return result, nil
}

func (t TraktClient) GetAuthUrl() string {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	url := t.Auth.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("Visit the URL for the auth dialog: %v", url)

	return url
}
func (t TraktClient) GetAuthToken(clientcode string) *oauth2.Token {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	ctx := context.Background()
	tok, err := t.Auth.Exchange(ctx, clientcode)
	if err != nil {
		log.Fatal(err)
	}
	return tok
}

func (t TraktClient) GetUserListAuth(username string, listname string, listtype string, limit int) ([]TraktUserList, error) {
	url := "https://api.trakt.tv/users/" + username + "/lists/" + listname + "/items/" + listtype
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	if t.Token.AccessToken != "" {
		req.Header.Add("Authorization", "Bearer "+t.Token.AccessToken)
	}
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	resp, responseData, err := t.Client.Do(req)
	if err != nil {
		return []TraktUserList{}, err
	}
	if resp.StatusCode == 429 {
		return []TraktUserList{}, err
	}
	var result []TraktUserList
	json.Unmarshal(responseData, &result)
	return result, nil
}
