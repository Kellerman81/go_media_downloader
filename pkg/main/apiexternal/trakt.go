package apiexternal

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"golang.org/x/oauth2"
)

type TraktMovieAnticipated struct {
	//ListCount int        `json:"list_count"`
	Movie TraktMovie `json:"movie"`
}

type TraktMovieTrending struct {
	//Watchers int        `json:"watchers"`
	Movie TraktMovie `json:"movie"`
}

type TraktUserList struct {
	//Rank      int        `json:"rank"`
	//ID        int        `json:"id"`
	//Notes     string     `json:"notes"`
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
	//Watchers int        `json:"watchers"`
	Serie TraktSerie `json:"show"`
}

type TraktSerieAnticipated struct {
	//ListCount int        `json:"list_count"`
	Serie TraktSerie `json:"show"`
}

type TraktSerieSeason struct {
	Number string `json:"number"`
	//Ids    Ids `json:"ids"`
}

type TraktSerieSeasonEpisodes struct {
	Season     int       `json:"season"`
	Episode    int       `json:"number"`
	Title      string    `json:"title"`
	Overview   string    `json:"overview"`
	Runtime    int       `json:"runtime"`
	FirstAired time.Time `json:"first_aired"`
	//Ids                   Ids       `json:"ids"`
	//EpisodeAbs            int       `json:"number_abs"`
	//Rating                float32   `json:"rating"`
	//Votes                 int       `json:"votes"`
	//Comments              int       `json:"comment_count"`
	//AvailableTranslations []string  `json:"available_translations"`
}

type TraktSerie struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	Ids   Ids    `json:"ids"`
}

type TraktSerieData struct {
	Title      string    `json:"title"`
	Year       int       `json:"year"`
	Ids        Ids       `json:"ids"`
	Overview   string    `json:"overview"`
	FirstAired time.Time `json:"first_aired"`
	Runtime    int       `json:"runtime"`
	Network    string    `json:"network"`
	Country    string    `json:"country"`
	Status     string    `json:"status"`
	Rating     float32   `json:"rating"`
	Language   string    `json:"language"`
	//AvailableTranslations []string  `json:"available_translations"`
	Genres []string `json:"genres"`
	//Certification         string    `json:"certification"`
	//Trailer               string    `json:"trailer"`
	//Homepage              string    `json:"homepage"`
	//Votes                 int       `json:"votes"`
	//Comments              int       `json:"comment_count"`
	//AiredEpisodes         int       `json:"aired_episodes"`
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
	Title    string  `json:"title"`
	Year     uint16  `json:"year"`
	Ids      Ids     `json:"ids"`
	Tagline  string  `json:"tagline"`
	Overview string  `json:"overview"`
	Released string  `json:"released"`
	Runtime  int     `json:"runtime"`
	Status   string  `json:"status"`
	Rating   float32 `json:"rating"`
	Votes    int32   `json:"votes"`
	Comments int     `json:"comment_count"`
	Language string  `json:"language"`
	//AvailableTranslations []string `json:"available_translations"`
	Genres []string `json:"genres"`
	//Country               string   `json:"country"`
	//Trailer               string   `json:"trailer"`
	//Homepage              string   `json:"homepage"`
	//Certification         string   `json:"certification"`
}

// traktClient is a struct for interacting with the Trakt API
// It contains fields for the API key, client ID, client secret, HTTP client,
// OAuth2 config, access token, and default headers
type traktClient struct {
	APIKey         string        // The API key for authentication
	ClientID       string        // The client ID for OAuth2
	ClientSecret   string        // The client secret for OAuth2
	Client         rlHTTPClient  // The HTTP client for requests
	Auth           oauth2.Config // The OAuth2 config
	Token          *oauth2.Token // The OAuth2 access token
	DefaultHeaders []string      // Default headers to send with requests
}

// NewTraktClient initializes a new traktClient instance for making requests to
// the Trakt API. It takes in credentials and rate limiting settings and sets up
// the OAuth2 configuration.
func NewTraktClient(clientid string, clientsecret string, seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	traktApidata = apidata{
		apikey:         clientid,
		clientID:       clientid,
		clientSecret:   clientsecret,
		disabletls:     disabletls,
		seconds:        seconds,
		calls:          calls,
		timeoutseconds: timeoutseconds,
		limiter:        slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		dailylimiter:   slidingwindow.NewLimiter(10*time.Second, 10),
	}
}

// GetTraktMoviePopular retrieves a list of popular movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovie structs containing the movie data,
// or nil if there was an error.
func GetTraktMoviePopular(limit *string) ([]TraktMovie, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	return doJSONTypeG[TraktMovie](&p.Client, traktaddlimit("https://api.trakt.tv/movies/popular", limit), p.DefaultHeaders)
}

// GetTraktMovieTrending retrieves a list of trending movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieTrending structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieTrending(limit *string) ([]TraktMovieTrending, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	return doJSONTypeG[TraktMovieTrending](&p.Client, traktaddlimit("https://api.trakt.tv/movies/trending", limit), p.DefaultHeaders)
}

// traktaddlimit adds a limit query parameter to the Trakt API URL if a limit is specified.
// It takes the limit as a string parameter.
// Returns a URL query string with the limit, or an empty string if no limit provided.
func traktaddlimit(urlv string, limit *string) string {
	if *limit != "" && *limit != "0" {
		return logger.JoinStrings(urlv, "?limit=", *limit)
	}
	return urlv
}

// GetTraktMovieAnticipated retrieves a list of anticipated movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieAnticipated structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieAnticipated(limit *string) ([]TraktMovieAnticipated, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	return doJSONTypeG[TraktMovieAnticipated](&p.Client, traktaddlimit("https://api.trakt.tv/movies/anticipated", limit), p.DefaultHeaders)
}

// GetTraktMovieAliases retrieves alias data from the Trakt API for the given movie ID.
// It takes a Trakt movie ID string as a parameter.
// Returns a slice of TraktAlias structs containing the alias data,
// or nil if there is an error or no aliases found.
func GetTraktMovieAliases(movieid string) ([]TraktAlias, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if movieid == "" || p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	urlv := logger.JoinStrings(apiurlmovies, movieid, "/aliases")
	return doJSONTypeG[TraktAlias](&p.Client, urlv, p.DefaultHeaders)
}

// GetTraktMovie retrieves extended data for a Trakt movie by ID.
// It takes a movie ID string as input.
// Returns a TraktMovieExtend struct containing the movie data,
// or nil and an error if the movie is not found or there is an error fetching data.
func GetTraktMovie(movieid string) (TraktMovieExtend, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if movieid == "" || p.Client.checklimiterwithdaily() {
		return TraktMovieExtend{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings(apiurlmovies, movieid, extendedfull)
	return doJSONTypeHeader[TraktMovieExtend](&p.Client, urlv, p.DefaultHeaders)
}

// GetTraktSerie retrieves extended data for a Trakt TV show by its Trakt ID.
// It takes the Trakt show ID as a string parameter.
// It returns a TraktSerieData struct containing the show data,
// or nil and an error if the show ID is invalid or there was an error retrieving data.
func GetTraktSerie(showid string) (TraktSerieData, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if showid == "" || p.Client.checklimiterwithdaily() {
		return TraktSerieData{}, logger.ErrNotFound
	}
	urlv := logger.JoinStrings(apiurlshows, showid, extendedfull)
	return doJSONTypeHeader[TraktSerieData](&p.Client, urlv, p.DefaultHeaders)
}

// GetTraktSerieAliases retrieves alias data from the Trakt API for the given Dbserie.
// It first checks if there is a Trakt ID available and uses that to retrieve aliases.
// If no Trakt ID, it falls back to using the IMDb ID if available.
// Returns a slice of TraktAlias structs or nil if no aliases found.
func GetTraktSerieAliases(dbserie *database.Dbserie) ([]TraktAlias, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	if dbserie.TraktID == 0 {
		if dbserie.ImdbID == "" {
			return nil, nil
		}
		urlv := logger.JoinStrings(apiurlshows, dbserie.ImdbID, "/aliases")
		return doJSONTypeG[TraktAlias](&p.Client, urlv, p.DefaultHeaders)
	}
	urlv := logger.JoinStrings(apiurlshows, strconv.Itoa(dbserie.TraktID), "/aliases")
	return doJSONTypeG[TraktAlias](&p.Client, urlv, p.DefaultHeaders)
}

// GetTraktSerieSeasons retrieves a list of season numbers for a Trakt TV show by ID.
// It returns a slice of season numbers as strings, or nil if there is an error.
func GetTraktSerieSeasons(showid string) ([]TraktSerieSeason, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if showid == "" || p.Client.checklimiterwithdaily() {
		return nil, nil
	}
	urlv := logger.JoinStrings(apiurlshows, showid, "/seasons")
	return doJSONTypeG[TraktSerieSeason](&p.Client, urlv, p.DefaultHeaders)
}

// GetTraktSerieSeasonsAndEpisodes retrieves all seasons and episodes for the given Trakt show ID from the Trakt API.
// It takes the show ID and database series ID as parameters.
// It queries the local database for existing episodes to avoid duplicates.
// For each season, it calls addtraktdbepisodes to insert any missing episodes into the database.
// Returns nothing.
func UpdateTraktSerieSeasonsAndEpisodes(showid string, id *uint) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if showid == "" || p.Client.checklimiterwithdaily() {
		return
	}
	baseurl := logger.JoinStrings(apiurlshows, showid, "/seasons")
	result, err := doJSONTypeG[TraktSerieSeason](&p.Client, baseurl, p.DefaultHeaders)
	if err != nil {
		return
	}
	baseurl = logger.JoinStrings(baseurl, "/")
	tbl := database.Getrows1size[database.DbstaticTwoString](false, database.QueryDbserieEpisodesCountByDBID, database.QueryDbserieEpisodesGetSeasonEpisodeByDBID, id)
	var data []TraktSerieSeasonEpisodes
	for idx := range result {
		if p.Client.checklimiterwithdaily() {
			//clear(tbl)
			//clear(result)
			return
		}
		data, err = doJSONTypeG[TraktSerieSeasonEpisodes](&p.Client, logger.JoinStrings(baseurl, result[idx].Number, extendedfull), p.DefaultHeaders)
		if err != nil {
			//clear(tbl)
			//clear(result)
			return
		}

		for idx2 := range data {
			if checkdbtwostrings(tbl, data[idx2].Season, data[idx2].Episode) {
				continue
			}
			database.ExecN("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
				strconv.Itoa(data[idx2].Episode), strconv.Itoa(data[idx2].Season), generateIdentifierStringFromInt(data[idx2].Season, data[idx2].Episode), &data[idx2].Title, &data[idx2].FirstAired, &data[idx2].Overview, id)
		}
		clear(data)
	}
	//clear(tbl)
	//clear(result)
}

func Testaddtraktdbepisodes() ([]TraktSerieSeasonEpisodes, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, nil
	}
	urlv, _ := url.JoinPath(apiurlshows, "tt1183865", "seasons", "1")
	return doJSONTypeG[TraktSerieSeasonEpisodes](&p.Client, urlv, p.DefaultHeaders)
}

func checkdbtwostrings(tbl []database.DbstaticTwoString, int1, int2 int) bool {
	if len(tbl) == 0 {
		return false
	}
	return database.ArrStructContains(tbl, database.DbstaticTwoString{Str1: strconv.Itoa(int1), Str2: strconv.Itoa(int2)})
}

// padNumberWithZero pads an integer value with leading zeros to ensure it is at least two digits.
// If the value is already two or more digits, it is returned unchanged as a string.
func padNumberWithZero(value int) string {
	if value == 0 {
		return "0"
	}
	if value >= 10 {
		return strconv.Itoa(value)
	}
	return logger.JoinStrings("0", strconv.Itoa(value))
}

// GetTraktSerieSeasonEpisodes retrieves all episodes for the given show ID and season from the Trakt API.
// It takes the show ID and season number as parameters.
// Returns a slice of TraktSerieSeasonEpisodes structs containing the episode data,
// or nil if there is an error.
func GetTraktSerieSeasonEpisodes(showid string, season string) ([]TraktSerieSeasonEpisodes, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if showid == "" || season == "" || p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	urlv := logger.JoinStrings(apiurlshows, showid, "/seasons/", season, extendedfull)
	return doJSONTypeG[TraktSerieSeasonEpisodes](&p.Client, urlv, p.DefaultHeaders)
}

// GetTraktUserList retrieves a Trakt user list by username, list name, list type,
// and optional limit. It returns a slice of TraktUserList structs containing
// the list data, and an error.
func GetTraktUserList(username string, listname string, listtype string, limit *string) ([]TraktUserList, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if username == "" || listname == "" || listtype == "" || p.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	urlv := logger.JoinStrings("https://api.trakt.tv/users/", username, "/lists/", listname, "/items/", listtype)
	urlv = traktaddlimit(urlv, limit)
	return doJSONTypeG[TraktUserList](&p.Client, urlv, p.DefaultHeaders)
}

// GetTraktSeriePopular retrieves popular TV shows from Trakt based on the
// number of watches and list additions. It takes an optional limit parameter
// to limit the number of results returned. Returns a slice of TraktSerie
// structs containing the popular show data.
func GetTraktSeriePopular(limit *string) ([]TraktSerie, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	return doJSONTypeG[TraktSerie](&p.Client, traktaddlimit("https://api.trakt.tv/shows/popular", limit), p.DefaultHeaders)
}

// GetTraktSerieTrending retrieves the trending TV shows from Trakt based on the limit parameter.
// It returns a slice of TraktSerieTrending structs containing the trending show data.
func GetTraktSerieTrending(limit *string) ([]TraktSerieTrending, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	return doJSONTypeG[TraktSerieTrending](&p.Client, traktaddlimit("https://api.trakt.tv/shows/trending", limit), p.DefaultHeaders)
}

// GetTraktSerieAnticipated retrieves the most anticipated TV shows from Trakt
// based on the number of list adds. It takes an optional limit parameter to limit
// the number of results returned. Returns a slice of TraktSerieAnticipated structs
// containing the anticipated show data.
func GetTraktSerieAnticipated(limit *string) ([]TraktSerieAnticipated, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if p.Client.checklimiterwithdaily() {
		return nil, errDailyLimit
	}
	return doJSONTypeG[TraktSerieAnticipated](&p.Client, traktaddlimit("https://api.trakt.tv/shows/anticipated", limit), p.DefaultHeaders)
}

// GetTraktToken returns the token used to authenticate with Trakt. This is a wrapper around the traktAPI
func GetTraktToken() *oauth2.Token {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	return p.Token
}

// SetTraktToken sets the OAuth 2.0 token used to authenticate
// with the Trakt API.
func SetTraktToken(tk *oauth2.Token) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	p.Token = tk
}

// GetTraktAuthURL generates an authorization URL that redirects the user
// to the Trakt consent page to request permission for the configured scopes.
// It returns the generated authorization URL.
func GetTraktAuthURL() string {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	urlv := p.Auth.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Println("Visit the URL for the auth dialog: " + urlv)

	return urlv
}

// GetTraktAuthToken exchanges the authorization code for an OAuth 2.0 token
// for the Trakt API. It takes the client code and returns the token, or nil and an
// error if there was an issue exchanging the code.
func GetTraktAuthToken(clientcode string) *oauth2.Token {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	ctx := context.Background()
	tok, err := p.Auth.Exchange(ctx, clientcode)
	if err != nil {
		logger.LogDynamicany("error", "Error getting token", err)
	}
	ctx.Done()
	return tok
}

// GetTraktUserListAuth retrieves a Trakt user list with authentication.
// It takes the username, list name, list type, and optional limit parameters and returns
// the user list items as an array of TraktUserList structs and an error.
// Returns ErrNotFound if username, listname or listtype are empty.
func GetTraktUserListAuth(username string, listname string, listtype string, limit *string) ([]TraktUserList, error) {
	p := pltrakt.Get()
	defer pltrakt.Put(p)
	if username == "" || listname == "" || listtype == "" || p.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	urlv := logger.JoinStrings("https://api.trakt.tv/users/", username, "/lists/", listname, "/items/", listtype)
	urlv = traktaddlimit(urlv, limit)
	return doJSONTypeG[TraktUserList](&p.Client, urlv, p.DefaultHeaders)
}
