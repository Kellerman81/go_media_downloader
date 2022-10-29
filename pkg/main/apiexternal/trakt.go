package apiexternal

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/rate"
	"go.uber.org/zap"
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
	Id        int        `json:"id"`
	Notes     string     `json:"notes"`
	TraktType string     `json:"type"`
	Movie     TraktMovie `json:"movie"`
	Serie     TraktSerie `json:"show"`
}

type TraktUserListGroup struct {
	Entries []TraktUserList
}

type TraktMovie struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   Ids    `json:"ids"`
}
type TraktMovieGroup struct {
	Movies []TraktMovie
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
type TraktSerieSeasonGroup struct {
	Seasons []TraktSerieSeason
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

type TraktSerieSeasonEpisodeGroup struct {
	Episodes []TraktSerieSeasonEpisodes
}

type TraktSerie struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   Ids    `json:"ids"`
}
type TraktSerieGroup struct {
	Series []TraktSerie
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

type TraktAliases struct {
	Aliases []TraktAlias
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
	Title                 string   `json:"title"`
	Year                  int      `json:"year"`
	Ids                   Ids      `json:"ids"`
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
	ApiKey         string
	ClientID       string
	ClientSecret   string
	Client         *RLHTTPClient
	Auth           *oauth2.Config
	Token          *oauth2.Token
	DefaultHeaders []AddHeader
}

var TraktApi traktClient

func NewTraktClient(clientid string, clientsecret string, token oauth2.Token, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	TraktApi = traktClient{
		ApiKey:       clientid,
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Client: NewClient(
			disabletls,
			rate.New(calls, 0, time.Duration(seconds)*time.Second), timeoutseconds),
		Auth: &oauth2.Config{
			ClientID:     clientid,
			ClientSecret: clientsecret,
			RedirectURL:  "http://localhost:9090",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://api.trakt.tv/oauth/authorize",
				TokenURL: "https://api.trakt.tv/oauth/token",
			},
		},
		Token:          &token,
		DefaultHeaders: []AddHeader{{Key: "Content-Type", Val: "application/json"}, {Key: "trakt-api-version", Val: "2"}, {Key: "trakt-api-key", Val: clientid}}}
	if TraktApi.Token.AccessToken != "" {
		TraktApi.DefaultHeaders = append(TraktApi.DefaultHeaders, AddHeader{Key: "Authorization", Val: "Bearer " + TraktApi.Token.AccessToken})
	}
}

func (t *traktClient) GetMoviePopular(limit int) (*TraktMovieGroup, error) {
	url := "https://api.trakt.tv/movies/popular"
	var movies TraktMovieGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		movies.Movies = make([]TraktMovie, 0, limit)
	}

	_, err := t.Client.DoJson(url, &movies.Movies, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}

	return &movies, nil
}

func (t *traktClient) GetMovieTrending(limit int) (*TraktMovieGroup, error) {
	url := "https://api.trakt.tv/movies/trending"
	var result []TraktMovieTrending //= make([]TraktMovieTrending, 0, 20)
	var movies TraktMovieGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		result = make([]TraktMovieTrending, 0, limit)
	}

	_, err := t.Client.DoJson(url, &result, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}
	movies.Movies = make([]TraktMovie, len(result))
	for idx := range result {
		movies.Movies[idx] = result[idx].Movie
	}

	return &movies, nil
}

func (t *traktClient) GetMovieAnticipated(limit int) (*TraktMovieGroup, error) {
	url := "https://api.trakt.tv/movies/anticipated"
	var result []TraktMovieAnticipated //= make([]TraktMovieAnticipated, 0, 20)
	var movies TraktMovieGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		result = make([]TraktMovieAnticipated, 0, limit)
	}

	_, err := t.Client.DoJson(url, &result, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}
	movies.Movies = make([]TraktMovie, len(result))
	for idx := range result {
		movies.Movies[idx] = result[idx].Movie
	}

	return &movies, nil
}

func (t *traktClient) GetMovieAliases(movieid string) (*TraktAliases, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "/aliases"

	var aliases TraktAliases
	_, err := t.Client.DoJson(url, &aliases.Aliases, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}
	return &aliases, nil
}
func (t *traktClient) GetMovie(movieid string) (TraktMovieExtend, error) {
	url := "https://api.trakt.tv/movies/" + movieid + "?extended=full"

	var result TraktMovieExtend
	_, err := t.Client.DoJson(url, &result, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TraktMovieExtend{}, err
	}
	return result, nil
}
func (t *traktClient) GetSerie(showid string) (TraktSerieData, error) {
	url := "https://api.trakt.tv/shows/" + showid + "?extended=full"

	var result TraktSerieData
	_, err := t.Client.DoJson(url, &result, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TraktSerieData{}, err
	}

	return result, nil
}

func (t *traktClient) GetSerieAliases(showid string) (*TraktAliases, error) {
	url := "https://api.trakt.tv/shows/" + showid + "/aliases"

	var aliases TraktAliases
	_, err := t.Client.DoJson(url, &aliases.Aliases, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}

	return &aliases, nil
}
func (t *traktClient) GetSerieSeasons(showid string) (*TraktSerieSeasonGroup, error) {
	url := "https://api.trakt.tv/shows/" + showid + "/seasons"

	var seasons TraktSerieSeasonGroup
	_, err := t.Client.DoJson(url, &seasons.Seasons, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}

	return &seasons, nil
}
func (t *traktClient) GetSerieSeasonEpisodes(showid string, season int) (*TraktSerieSeasonEpisodeGroup, error) {
	url := "https://api.trakt.tv/shows/" + showid + "/seasons/" + strconv.FormatInt(int64(season), 10) + "?extended=full"

	var episodes TraktSerieSeasonEpisodeGroup
	_, err := t.Client.DoJson(url, &episodes.Episodes, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}

	return &episodes, nil
}

func (t *traktClient) GetUserList(username string, listname string, listtype string, limit int) (*TraktUserListGroup, error) {
	url := "https://api.trakt.tv/users/" + username + "/lists/" + listname + "/items/" + listtype
	var entries TraktUserListGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		entries.Entries = make([]TraktUserList, 0, limit)
	}

	_, err := t.Client.DoJson(url, &entries.Entries, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}

	return &entries, nil
}

func (t *traktClient) GetSeriePopular(limit int) (*TraktSerieGroup, error) {
	url := "https://api.trakt.tv/shows/popular"
	var series TraktSerieGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		series.Series = make([]TraktSerie, 0, limit)
	}

	_, err := t.Client.DoJson(url, &series.Series, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}

	return &series, nil
}

func (t *traktClient) GetSerieTrending(limit int) (*TraktSerieGroup, error) {
	url := "https://api.trakt.tv/shows/trending"
	var result []TraktSerieTrending //= make([]TraktSerieTrending, 0, 20)
	var series TraktSerieGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		result = make([]TraktSerieTrending, 0, limit)
	}

	_, err := t.Client.DoJson(url, &result, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}
	series.Series = make([]TraktSerie, len(result))
	for idx := range result {
		series.Series[idx] = result[idx].Serie
	}

	return &series, nil
}

func (t *traktClient) GetSerieAnticipated(limit int) (*TraktSerieGroup, error) {
	url := "https://api.trakt.tv/shows/anticipated"
	var result []TraktSerieAnticipated //= make([]TraktSerieAnticipated, 0, 20)
	var series TraktSerieGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		result = make([]TraktSerieAnticipated, 0, limit)
	}

	_, err := t.Client.DoJson(url, &result, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return nil, err
	}
	series.Series = make([]TraktSerie, len(result))
	for idx := range result {
		series.Series[idx] = result[idx].Serie
	}

	return &series, nil
}

func (t *traktClient) GetAuthUrl() string {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	url := t.Auth.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Println("Visit the URL for the auth dialog: " + url)

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
	ctx.Done()
	return tok
}

func (t *traktClient) GetUserListAuth(username string, listname string, listtype string, limit int) (TraktUserListGroup, error) {
	url := "https://api.trakt.tv/users/" + username + "/lists/" + listname + "/items/" + listtype
	var entries TraktUserListGroup
	if limit >= 1 {
		url = url + "?limit=" + strconv.Itoa(limit)
		entries.Entries = make([]TraktUserList, 0, limit)
	}

	_, err := t.Client.DoJson(url, &entries.Entries, t.DefaultHeaders)

	if err != nil {
		logger.Log.GlobalLogger.Error("Error calling", zap.String("url", url), zap.Error(err))
		return TraktUserListGroup{}, err
	}

	return entries, nil
}
