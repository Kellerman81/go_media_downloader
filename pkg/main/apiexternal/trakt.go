package apiexternal

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"golang.org/x/oauth2"
)

type TraktMovieAnticipated struct {
	ListCount int        `json:"list_count"`
	Movie     TraktMovie `json:"movie"`
}

type TraktMovieTrending struct {
	Watchers int        `json:"watchers"`
	Movie    TraktMovie `json:"movie"`
}

type TraktUserList struct {
	Rank      int        `json:"rank"`
	ID        int        `json:"id"`
	Notes     string     `json:"notes"`
	TraktType string     `json:"type"`
	Movie     TraktMovie `json:"movie"`
	Serie     TraktSerie `json:"show"`
}

type TraktMovie struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   Ids    `json:"ids"`
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
	Ids   Ids    `json:"ids"`
}

type TraktSerieData struct {
	Title                 string    `json:"title"`
	Year                  int       `json:"year"`
	Ids                   Ids       `json:"ids"`
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

type TraktAlias struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type Ids struct {
	Trakt  int    `json:"trakt"`
	Slug   string `json:"slug"`
	Imdb   string `json:"imdb"`
	Tmdb   int    `json:"tmdb"`
	Tvdb   int    `json:"tvdb"`
	Tvrage int    `json:"tvrage"`
}

type TraktMovieExtend struct {
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Ids      Ids    `json:"ids"`
	Tagline  string `json:"tagline"`
	Overview string `json:"overview"`
	Released string `json:"released"`
	Runtime  int    `json:"runtime"`
	//Country               string   `json:"country"`
	//Trailer               string   `json:"trailer"`
	//Homepage              string   `json:"homepage"`
	Status                string   `json:"status"`
	Rating                float32  `json:"rating"`
	Votes                 int      `json:"votes"`
	Comments              int      `json:"comment_count"`
	Language              string   `json:"language"`
	AvailableTranslations []string `json:"available_translations"`
	Genres                []string `json:"genres"`
	//Certification         string   `json:"certification"`
}

type traktClient struct {
	APIKey         string
	ClientID       string
	ClientSecret   string
	Client         *RLHTTPClient
	Auth           oauth2.Config
	Token          oauth2.Token
	DefaultHeaders []addHeader
}

const (
	apiurlshows  = "https://api.trakt.tv/shows/"
	apiurlmovies = "https://api.trakt.tv/movies/"
	limitquery   = "?limit="
	extendedfull = "?extended=full"
)

var TraktAPI *traktClient

func (t *TraktMovieExtend) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.AvailableTranslations)
	logger.Clear(&t.Genres)
	logger.ClearVar(t)
}

func (t *TraktSerieData) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if t == nil {
		return
	}
	logger.Clear(&t.AvailableTranslations)
	logger.Clear(&t.Genres)
	logger.ClearVar(t)
}
func NewTraktClient(clientid string, clientsecret string, token oauth2.Token, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TraktAPI = &traktClient{
		APIKey:       clientid,
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Client: NewClient(
			disabletls,
			true,
			slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
			false, slidingwindow.NewLimiter(10*time.Second, 10), timeoutseconds),
		Auth: oauth2.Config{
			ClientID:     clientid,
			ClientSecret: clientsecret,
			RedirectURL:  "http://localhost:9090",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://api.trakt.tv/oauth/authorize",
				TokenURL: "https://api.trakt.tv/oauth/token",
			},
		},
		Token:          token,
		DefaultHeaders: []addHeader{{key: "Content-Type", val: "application/json"}, {key: "trakt-api-version", val: "2"}, {key: "trakt-api-key", val: clientid}}}
	if TraktAPI.Token.AccessToken != "" {
		TraktAPI.DefaultHeaders = append(TraktAPI.DefaultHeaders, addHeader{key: "Authorization", val: "Bearer " + TraktAPI.Token.AccessToken})
	}
}

func (t *traktClient) GetMoviePopular(limit int) (*[]TraktMovie, error) {
	return DoJSONType[[]TraktMovie](t.Client, "https://api.trakt.tv/movies/popular"+traktaddlimit(limit), t.DefaultHeaders...)
}

func (t *traktClient) GetMovieTrending(limit int) (*[]TraktMovie, error) {
	result, err := DoJSONType[[]TraktMovieTrending](t.Client, "https://api.trakt.tv/movies/trending"+traktaddlimit(limit), t.DefaultHeaders...)

	if err != nil || result == nil {
		return nil, err
	}
	if len(*result) == 0 {
		return nil, logger.Errnoresults
	}
	movies := make([]TraktMovie, len(*result))
	for idx := range *result {
		movies[idx] = (*result)[idx].Movie
	}
	logger.Clear(result)
	return &movies, nil
}

func traktaddlimit(limit int) string {
	if limit >= 1 {
		return limitquery + logger.IntToString(limit)
	}
	return ""
}

func (t *traktClient) GetMovieAnticipated(limit int) (*[]TraktMovie, error) {
	result, err := DoJSONType[[]TraktMovieAnticipated](t.Client, "https://api.trakt.tv/movies/anticipated"+traktaddlimit(limit), t.DefaultHeaders...)

	if err != nil || result == nil {
		return nil, err
	}
	if len(*result) == 0 {
		return nil, logger.Errnoresults
	}
	movies := make([]TraktMovie, len(*result))
	for idx := range *result {
		movies[idx] = (*result)[idx].Movie
	}
	logger.Clear(result)
	return &movies, nil
}

func (t *traktClient) GetMovieAliases(movieid string) (*[]TraktAlias, error) {
	if movieid == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[[]TraktAlias](t.Client, apiurlmovies+movieid+"/aliases", t.DefaultHeaders...)
}
func (t *traktClient) GetMovie(movieid string) (*TraktMovieExtend, error) {
	if movieid == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TraktMovieExtend](t.Client, apiurlmovies+movieid+extendedfull, t.DefaultHeaders...)
}
func (t *traktClient) GetSerie(showid string) (*TraktSerieData, error) {
	if showid == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[TraktSerieData](t.Client, apiurlshows+showid+extendedfull, t.DefaultHeaders...)
}

func (t *traktClient) GetSerieAliases(showid string) (*[]TraktAlias, error) {
	if showid == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[[]TraktAlias](t.Client, apiurlshows+showid+"/aliases", t.DefaultHeaders...)
}
func (t *traktClient) GetSerieSeasons(showid string) (*[]TraktSerieSeason, error) {
	if showid == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[[]TraktSerieSeason](t.Client, apiurlshows+showid+"/seasons", t.DefaultHeaders...)
}
func (t *traktClient) GetSerieSeasonEpisodes(showid string, season int) (*[]TraktSerieSeasonEpisodes, error) {
	if showid == "" || season == 0 {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[[]TraktSerieSeasonEpisodes](t.Client, apiurlshows+showid+"/seasons/"+logger.IntToString(season)+extendedfull, t.DefaultHeaders...)
}

func (t *traktClient) GetUserList(username string, listname string, listtype string, limit int) (*[]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[[]TraktUserList](t.Client, "https://api.trakt.tv/users/"+username+"/lists/"+listname+"/items/"+listtype+traktaddlimit(limit), t.DefaultHeaders...)
}

func (t *traktClient) GetSeriePopular(limit int) (*[]TraktSerie, error) {
	return DoJSONType[[]TraktSerie](t.Client, "https://api.trakt.tv/shows/popular"+traktaddlimit(limit), t.DefaultHeaders...)
}

func (t *traktClient) GetSerieTrending(limit int) (*[]TraktSerie, error) {
	result, err := DoJSONType[[]TraktSerieTrending](t.Client, "https://api.trakt.tv/shows/trending"+traktaddlimit(limit), t.DefaultHeaders...)

	if err != nil || result == nil {
		return nil, err
	}
	if len(*result) == 0 {
		return nil, logger.Errnoresults
	}
	series := make([]TraktSerie, len(*result))
	for idx := range *result {
		series[idx] = (*result)[idx].Serie
	}
	logger.Clear(result)
	return &series, nil
}

func (t *traktClient) GetSerieAnticipated(limit int) (*[]TraktSerie, error) {
	result, err := DoJSONType[[]TraktSerieAnticipated](t.Client, "https://api.trakt.tv/shows/anticipated"+traktaddlimit(limit), t.DefaultHeaders...)

	if err != nil || result == nil {
		return nil, err
	}
	if len(*result) == 0 {
		return nil, logger.Errnoresults
	}
	series := make([]TraktSerie, len(*result))
	for idx := range *result {
		series[idx] = (*result)[idx].Serie
	}

	logger.Clear(result)
	return &series, nil
}

func (t *traktClient) GetAuthURL() string {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	urlv := t.Auth.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Println("Visit the URL for the auth dialog: " + urlv)

	return urlv
}
func (t *traktClient) GetAuthToken(clientcode string) oauth2.Token {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	ctx := context.Background()
	tok, err := t.Auth.Exchange(ctx, clientcode)
	if err != nil {
		log.Fatal(err)
	}
	ctx.Done()
	return *tok
}

func (t *traktClient) GetUserListAuth(username string, listname string, listtype string, limit int) (*[]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" {
		return nil, logger.ErrNotFound
	}
	return DoJSONType[[]TraktUserList](t.Client, "https://api.trakt.tv/users/"+username+"/lists/"+listname+"/items/"+listtype+traktaddlimit(limit), t.DefaultHeaders...)
}
