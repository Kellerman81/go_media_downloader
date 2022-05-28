package apiexternal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

type traktMovieAnticipated struct {
	ListCount int        `json:"list_count"`
	Movie     TraktMovie `json:"movie"`
}

type traktMovieTrending struct {
	Watchers int        `json:"watchers"`
	Movie    TraktMovie `json:"movie"`
}

type traktUserList struct {
	Rank      int        `json:"rank"`
	Id        int        `json:"id"`
	Notes     string     `json:"notes"`
	TraktType string     `json:"type"`
	Movie     TraktMovie `json:"movie"`
	Serie     TraktSerie `json:"show"`
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
type traktSerieTrending struct {
	Watchers int        `json:"watchers"`
	Serie    TraktSerie `json:"show"`
}
type traktSerieAnticipated struct {
	ListCount int        `json:"list_count"`
	Serie     TraktSerie `json:"show"`
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

func NewTraktClient(clientid string, clientsecret string, token oauth2.Token, seconds int, calls int, disabletls bool) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TraktApi = &traktClient{
		ApiKey:       clientid,
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Client: NewClient(
			disabletls,
			rate.NewLimiter(rate.Every(time.Duration(seconds)*time.Second), calls),
			slidingwindow.NewLimiterNoStop(time.Duration(seconds)*time.Second, int64(calls), func() (slidingwindow.Window, slidingwindow.StopFunc) { return slidingwindow.NewLocalWindow() })),
		Auth: &oauth2.Config{
			ClientID:     clientid,
			ClientSecret: clientsecret,
			RedirectURL:  "http://localhost:9090",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://api.trakt.tv/oauth/authorize",
				TokenURL: "https://api.trakt.tv/oauth/token",
			},
		},
		Token: &token}
}

func (t *traktClient) GetMoviePopular(limit int) ([]TraktMovie, error) {
	url := "https://api.trakt.tv/movies/popular"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []TraktMovie{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []TraktMovie
	defer logger.ClearVar(&result)
	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []TraktMovie{}, err
	}

	return result, nil
}

func (t *traktClient) GetMovieTrending(limit int) ([]TraktMovie, error) {
	url := "https://api.trakt.tv/movies/trending"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []TraktMovie{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktMovieTrending
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []TraktMovie{}, err
	}
	res := make([]TraktMovie, 0, len(result))
	for idx := range result {
		res = append(res, result[idx].Movie)
	}
	return res, nil
}

func (t *traktClient) GetMovieAnticipated(limit int) ([]TraktMovie, error) {
	url := "https://api.trakt.tv/movies/anticipated"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []TraktMovie{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktMovieAnticipated
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []TraktMovie{}, err
	}
	res := make([]TraktMovie, 0, len(result))
	for idx := range result {
		res = append(res, result[idx].Movie)
	}
	return res, nil
}

func (t *traktClient) GetMovieAliases(movieid string) ([]traktMovieAliases, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "/aliases"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []traktMovieAliases{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktMovieAliases
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []traktMovieAliases{}, err
	}

	return result, nil
}
func (t *traktClient) GetMovie(movieid string) (traktMovieExtend, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "?extended=full"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return traktMovieExtend{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result traktMovieExtend

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return traktMovieExtend{}, err
	}

	return result, nil
}
func (t *traktClient) GetSerie(movieid string) (traktSerieData, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "?extended=full"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return traktSerieData{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result traktSerieData

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return traktSerieData{}, err
	}

	return result, nil
}

func (t *traktClient) GetSerieAliases(movieid string) ([]traktShowAliases, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/aliases"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []traktShowAliases{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktShowAliases
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []traktShowAliases{}, err
	}

	return result, nil
}
func (t *traktClient) GetSerieSeasons(movieid string) ([]traktSerieSeason, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/seasons"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []traktSerieSeason{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktSerieSeason
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []traktSerieSeason{}, err
	}

	return result, nil
}
func (t *traktClient) GetSerieSeasonEpisodes(movieid string, season int) ([]TraktSerieSeasonEpisodes, error) {
	url := "https://api.trakt.tv/shows/" + movieid + "/seasons/" + strconv.Itoa(season) + "?extended=full"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []TraktSerieSeasonEpisodes{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []TraktSerieSeasonEpisodes
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []TraktSerieSeasonEpisodes{}, err
	}

	return result, nil
}

func (t *traktClient) GetUserList(username string, listname string, listtype string, limit int) ([]traktUserList, error) {
	url := "https://api.trakt.tv/users/" + username + "/lists/" + listname + "/items/" + listtype
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []traktUserList{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	if t.Token.AccessToken != "" {
		req.Header.Add("Authorization", "Bearer "+t.Token.AccessToken)
	}
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktUserList
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []traktUserList{}, err
	}

	return result, nil
}

func (t *traktClient) GetSeriePopular(limit int) ([]TraktSerie, error) {
	url := "https://api.trakt.tv/shows/popular"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []TraktSerie{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []TraktSerie
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []TraktSerie{}, err
	}

	return result, nil
}

func (t *traktClient) GetSerieTrending(limit int) ([]TraktSerie, error) {
	url := "https://api.trakt.tv/shows/trending"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []TraktSerie{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktSerieTrending
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []TraktSerie{}, err
	}
	res := make([]TraktSerie, 0, len(result))
	for idx := range result {
		res = append(res, result[idx].Serie)
	}
	return res, nil
}

func (t *traktClient) GetSerieAnticipated(limit int) ([]TraktSerie, error) {
	url := "https://api.trakt.tv/shows/anticipated"
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
	} else {
		limit = 10
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []TraktSerie{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktSerieAnticipated
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []TraktSerie{}, err
	}
	res := make([]TraktSerie, 0, len(result))
	for idx := range result {
		res = append(res, result[idx].Serie)
	}
	return res, nil
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

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []traktUserList{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	if t.Token.AccessToken != "" {
		req.Header.Add("Authorization", "Bearer "+t.Token.AccessToken)
	}
	req.Header.Add("trakt-api-version", "2")
	req.Header.Add("trakt-api-key", t.ApiKey)

	var result []traktUserList
	defer logger.ClearVar(&result)

	err = t.Client.DoJson(req, &result)

	if err != nil {
		return []traktUserList{}, err
	}

	return result, nil
}
