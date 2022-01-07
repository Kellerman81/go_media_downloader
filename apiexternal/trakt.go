package apiexternal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

type traktMovieAnticipated struct {
	ListCount int        `json:"list_count"`
	Movie     traktMovie `json:"movie"`
}

type traktMovieTrending struct {
	Watchers int        `json:"watchers"`
	Movie    traktMovie `json:"movie"`
}

type traktUserList struct {
	Rank  int        `json:"rank"`
	Id    int        `json:"id"`
	Notes string     `json:"notes"`
	Type  string     `json:"type"`
	Movie traktMovie `json:"movie"`
	Serie traktSerie `json:"show"`
}

type traktMovie struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   struct {
		Trakt int    `json:"trakt"`
		Slug  string `json:"slug"`
		Imdb  string `json:"imdb"`
		Tmdb  int    `json:"tmdb"`
	} `json:"ids"`
}
type traktSerieTrending struct {
	Watchers int        `json:"watchers"`
	Serie    traktSerie `json:"show"`
}
type traktSerieAnticipated struct {
	ListCount int        `json:"list_count"`
	Serie     traktSerie `json:"show"`
}
type traktSerieSeason struct {
	Number int `json:"number"`
	Ids    struct {
		Trakt  int    `json:"trakt"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
		Imdb   string `json:"imdb"`
		Tmdb   int    `json:"tmdb"`
	} `json:"ids"`
}
type traktSerieSeasonEpisodes struct {
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

type traktSerie struct {
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

type traktSerieData struct {
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
type traktShowAliases struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type traktMovieAliases struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type traktMovieExtend struct {
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

type traktClient struct {
	ApiKey       string
	ClientID     string
	ClientSecret string
	Client       *RLHTTPClient
	Auth         *oauth2.Config
	Token        *oauth2.Token
}

var TraktApi *traktClient

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
	TraktApi = &traktClient{
		ApiKey:       clientid,
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Client:       NewClient(rl, limiter),
		Auth:         conf,
		Token:        &token}
}

func (t *traktClient) GetMoviePopular(limit int) ([]traktMovie, error) {
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

	var result []traktMovie
	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktMovie{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetMovieTrending(limit int) ([]traktMovieTrending, error) {
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

	var result []traktMovieTrending

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktMovieTrending{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetMovieAnticipated(limit int) ([]traktMovieAnticipated, error) {
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

	var result []traktMovieAnticipated

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktMovieAnticipated{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetMovieAliases(movieid string) ([]traktMovieAliases, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "/aliases"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktMovieAliases

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktMovieAliases{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t *traktClient) GetMovie(movieid string) (traktMovieExtend, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "?extended=full"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result traktMovieExtend

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return traktMovieExtend{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t *traktClient) GetSerie(movieid string) (traktSerieData, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "?extended=full"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result traktSerieData

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return traktSerieData{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetSerieAliases(movieid string) ([]traktShowAliases, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/aliases"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktShowAliases

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktShowAliases{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t *traktClient) GetSerieSeasons(movieid string) ([]traktSerieSeason, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/seasons"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktSerieSeason

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktSerieSeason{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
func (t *traktClient) GetSerieSeasonEpisodes(movieid string, season int) ([]traktSerieSeasonEpisodes, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/seasons/" + strconv.Itoa(season) + "?extended=full"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktSerieSeasonEpisodes

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktSerieSeasonEpisodes{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetUserList(username string, listname string, listtype string, limit int) ([]traktUserList, error) {
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

	var result []traktUserList

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktUserList{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetSeriePopular(limit int) ([]traktSerie, error) {
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

	var result []traktSerie

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktSerie{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetSerieTrending(limit int) ([]traktSerieTrending, error) {
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

	var result []traktSerieTrending

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktSerieTrending{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetSerieAnticipated(limit int) ([]traktSerieAnticipated, error) {
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

	var result []traktSerieAnticipated

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktSerieAnticipated{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}

func (t *traktClient) GetAuthUrl() string {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	url := t.Auth.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("Visit the URL for the auth dialog: %v", url)

	return url
}
func (t *traktClient) GetAuthToken(clientcode string) *oauth2.Token {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	ctx := context.Background()
	tok, err := t.Auth.Exchange(ctx, clientcode)
	if err != nil {
		log.Fatal(err)
	}
	return tok
}

func (t *traktClient) GetUserListAuth(username string, listname string, listtype string, limit int) ([]traktUserList, error) {
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

	var result []traktUserList

	err := t.Client.DoJson(req, &result)
	if err != nil {
		return []traktUserList{}, err
	}
	//json.Unmarshal(responseData, &result)
	return result, nil
}
