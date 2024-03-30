package apiexternal

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
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
	Ids   ids    `json:"ids"`
}

type TraktSerieTrending struct {
	//Watchers int        `json:"watchers"`
	Serie TraktSerie `json:"show"`
}

type TraktSerieAnticipated struct {
	//ListCount int        `json:"list_count"`
	Serie TraktSerie `json:"show"`
}

type traktSerieSeason struct {
	Number int `json:"number"`
	//Ids    Ids `json:"ids"`
}

type traktSerieSeasonEpisodes struct {
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
	Ids   ids    `json:"ids"`
}

type traktSerieData struct {
	Title                 string    `json:"title"`
	Year                  int       `json:"year"`
	Ids                   ids       `json:"ids"`
	Overview              string    `json:"overview"`
	FirstAired            time.Time `json:"first_aired"`
	Runtime               int       `json:"runtime"`
	Network               string    `json:"network"`
	Country               string    `json:"country"`
	Status                string    `json:"status"`
	Rating                float32   `json:"rating"`
	Language              string    `json:"language"`
	AvailableTranslations []string  `json:"available_translations"`
	Genres                []string  `json:"genres"`
	//Certification         string    `json:"certification"`
	//Trailer               string    `json:"trailer"`
	//Homepage              string    `json:"homepage"`
	//Votes                 int       `json:"votes"`
	//Comments              int       `json:"comment_count"`
	//AiredEpisodes         int       `json:"aired_episodes"`
}

type traktAlias struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type ids struct {
	Trakt  int    `json:"trakt"`
	Slug   string `json:"slug"`
	Imdb   string `json:"imdb"`
	Tmdb   int    `json:"tmdb"`
	Tvdb   int    `json:"tvdb"`
	Tvrage int    `json:"tvrage"`
}

type traktMovieExtend struct {
	Title                 string   `json:"title"`
	Year                  int      `json:"year"`
	Ids                   ids      `json:"ids"`
	Tagline               string   `json:"tagline"`
	Overview              string   `json:"overview"`
	Released              string   `json:"released"`
	Runtime               int      `json:"runtime"`
	Status                string   `json:"status"`
	Rating                float32  `json:"rating"`
	Votes                 int      `json:"votes"`
	Comments              int      `json:"comment_count"`
	Language              string   `json:"language"`
	AvailableTranslations []string `json:"available_translations"`
	Genres                []string `json:"genres"`
	//Country               string   `json:"country"`
	//Trailer               string   `json:"trailer"`
	//Homepage              string   `json:"homepage"`
	//Certification         string   `json:"certification"`
}

type keyval struct {
	Key   string
	Value string
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
	DefaultHeaders []keyval      // Default headers to send with requests
}

// Close clears slices in TraktMovieExtend to free memory.
func (t *traktMovieExtend) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = traktMovieExtend{}
}

// Close releases the resources used by the TraktSerieData struct if
// they are no longer needed. It sets any slices to nil to allow garbage
// collection.
func (t *traktSerieData) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || t == nil {
		return
	}
	*t = traktSerieData{}
}

// NewTraktClient initializes a new traktClient instance for making requests to
// the Trakt API. It takes in credentials and rate limiting settings and sets up
// the OAuth2 configuration.
func NewTraktClient(clientid string, clientsecret string, seconds int, calls int, disabletls bool, timeoutseconds int) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	traktAPI = traktClient{
		APIKey:       clientid,
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Client: NewClient(
			"trakt",
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
		Token:          config.GetTrakt(),
		DefaultHeaders: []keyval{{"Content-Type", "application/json"}, {"trakt-api-version", "2"}, {"trakt-api-key", clientid}}}
	if traktAPI.Token.AccessToken != "" {
		traktAPI.DefaultHeaders = append(traktAPI.DefaultHeaders, keyval{"Authorization", "Bearer " + traktAPI.Token.AccessToken})
	}
}

// GetTraktMoviePopular retrieves a list of popular movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovie structs containing the movie data,
// or nil if there was an error.
func GetTraktMoviePopular(limit string) []TraktMovie {
	if traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[TraktMovie](&traktAPI.Client, traktaddlimit("https://api.trakt.tv/movies/popular", limit), traktAPI.DefaultHeaders...)
	if err != nil {
		return nil
	}
	return result
}

// GetTraktMovieTrending retrieves a list of trending movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieTrending structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieTrending(limit string) []TraktMovieTrending {
	if traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[TraktMovieTrending](&traktAPI.Client, traktaddlimit("https://api.trakt.tv/movies/trending", limit), traktAPI.DefaultHeaders...)

	if err != nil {
		return nil
	}
	return result
}

// traktaddlimit adds a limit query parameter to the Trakt API URL if a limit is specified.
// It takes the limit as a string parameter.
// Returns a URL query string with the limit, or an empty string if no limit provided.
func traktaddlimit(urlv string, limit string) string {
	if limit != "" && limit != "0" {
		return logger.JoinStrings(urlv, "?limit=", limit)
	}
	return urlv
}

// GetTraktMovieAnticipated retrieves a list of anticipated movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieAnticipated structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieAnticipated(limit string) []TraktMovieAnticipated {
	if traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[TraktMovieAnticipated](&traktAPI.Client, traktaddlimit("https://api.trakt.tv/movies/anticipated", limit), traktAPI.DefaultHeaders...)

	if err != nil {
		return nil
	}
	return result
}

// GetTraktMovieAliases retrieves alias data from the Trakt API for the given movie ID.
// It takes a Trakt movie ID string as a parameter.
// Returns a slice of TraktAlias structs containing the alias data,
// or nil if there is an error or no aliases found.
func GetTraktMovieAliases(movieid string) []traktAlias {
	if movieid == "" || traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[traktAlias](&traktAPI.Client, logger.URLJoinPath(apiurlmovies, movieid, "aliases"), traktAPI.DefaultHeaders...)
	if err != nil {
		return nil
	}
	return result
}

// GetTraktMovie retrieves extended data for a Trakt movie by ID.
// It takes a movie ID string as input.
// Returns a TraktMovieExtend struct containing the movie data,
// or nil and an error if the movie is not found or there is an error fetching data.
func GetTraktMovie(movieid string) (traktMovieExtend, error) {
	if movieid == "" || traktAPI.Client.checklimiterwithdaily() {
		return traktMovieExtend{}, logger.ErrNotFound
	}
	//return DoJSONType[traktMovieExtend](traktAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurlmovies, movieid), extendedfull), &traktAPI.DefaultHeaders)
	return DoJSONType[traktMovieExtend](&traktAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurlmovies, movieid), extendedfull), traktAPI.DefaultHeaders...)
}

// GetTraktSerie retrieves extended data for a Trakt TV show by its Trakt ID.
// It takes the Trakt show ID as a string parameter.
// It returns a TraktSerieData struct containing the show data,
// or nil and an error if the show ID is invalid or there was an error retrieving data.
func GetTraktSerie(showid string) (traktSerieData, error) {
	if showid == "" || traktAPI.Client.checklimiterwithdaily() {
		return traktSerieData{}, logger.ErrNotFound
	}
	//return DoJSONType[traktSerieData](traktAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurlshows, showid), extendedfull), &traktAPI.DefaultHeaders)
	return DoJSONType[traktSerieData](&traktAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurlshows, showid), extendedfull), traktAPI.DefaultHeaders...)
}

// GetTraktSerieAliases retrieves alias data from the Trakt API for the given Dbserie.
// It first checks if there is a Trakt ID available and uses that to retrieve aliases.
// If no Trakt ID, it falls back to using the IMDb ID if available.
// Returns a slice of TraktAlias structs or nil if no aliases found.
func GetTraktSerieAliases(dbserie *database.Dbserie) []traktAlias {
	if traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	if dbserie.TraktID == 0 {
		if dbserie.ImdbID == "" {
			return nil
		}
		result, err := DoJSONTypeG[traktAlias](&traktAPI.Client, logger.URLJoinPath(apiurlshows, dbserie.ImdbID, "aliases"), traktAPI.DefaultHeaders...)
		if err != nil {
			return nil
		}
		return result
	}
	result, err := DoJSONTypeG[traktAlias](&traktAPI.Client, logger.URLJoinPath(apiurlshows, strconv.Itoa(dbserie.TraktID), "aliases"), traktAPI.DefaultHeaders...)
	if err != nil {
		return nil
	}
	return result
}

// GetTraktSerieSeasons retrieves a list of season numbers for a Trakt TV show by ID.
// It returns a slice of season numbers as strings, or nil if there is an error.
func GetTraktSerieSeasons(showid string) []string {
	if showid == "" || traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[traktSerieSeason](&traktAPI.Client, logger.URLJoinPath(apiurlshows, showid, "seasons"), traktAPI.DefaultHeaders...)
	if err != nil {
		return nil
	}
	ret := make([]string, len(result))
	for idx := range result {
		ret[idx] = strconv.Itoa(result[idx].Number)
	}
	clear(result)
	return ret
}

// GetTraktSerieSeasonsAndEpisodes retrieves all seasons and episodes for the given Trakt show ID from the Trakt API.
// It takes the show ID and database series ID as parameters.
// It queries the local database for existing episodes to avoid duplicates.
// For each season, it calls addtraktdbepisodes to insert any missing episodes into the database.
// Returns nothing.
func UpdateTraktSerieSeasonsAndEpisodes(showid string, id *uint) {
	if showid == "" || traktAPI.Client.checklimiterwithdaily() {
		return
	}
	result, err := DoJSONTypeG[traktSerieSeason](&traktAPI.Client, logger.URLJoinPath(apiurlshows, showid, "seasons"), traktAPI.DefaultHeaders...)
	if err != nil {
		return
	}
	baseurl := logger.URLJoinPath(apiurlshows, showid, "seasons")
	tbl := database.Getrows1size[database.DbstaticTwoString](false, database.QueryDbserieEpisodesCountByDBID, database.QueryDbserieEpisodesGetSeasonEpisodeByDBID, id)
	for idx := range result {
		//addtraktdbepisodes(&id, logger.JoinStrings(logger.URLJoinPath(apiurlshows, showid, "seasons", strconv.Itoa(result[idx].Number)), extendedfull), tbl)
		addtraktdbepisodes(id, logger.JoinStrings(logger.URLJoinPath(baseurl, strconv.Itoa(result[idx].Number)), extendedfull), tbl)
	}
	clear(tbl)
}

// addtraktdbepisodes retrieves Trakt episode data for a show and inserts any missing episodes into the dbserie_episodes table.
// It takes a dbserie ID, the Trakt API URL to retrieve episode data, and a slice of existing episodes.
// For each episode returned by Trakt that does not exist in the existing episode list, it inserts a new record into the database.
func addtraktdbepisodes(dbid *uint, urlv string, tbl []database.DbstaticTwoString) {
	if traktAPI.Client.checklimiterwithdaily() {
		return
	}
	data, err := DoJSONTypeG[traktSerieSeasonEpisodes](&traktAPI.Client, urlv, traktAPI.DefaultHeaders...)
	if err != nil {
		return
	}

	for idx := range data {
		strepisode := strconv.Itoa(data[idx].Episode)
		strseason := strconv.Itoa(data[idx].Season)

		if checkdbtwostrings(tbl, strseason, strepisode) {
			continue
		}
		stridentifier := GenerateIdentifierStringFromInt(data[idx].Season, data[idx].Episode)
		database.ExecN("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
			&strepisode, &strseason, &stridentifier, &data[idx].Title, &data[idx].FirstAired, &data[idx].Overview, dbid)
	}
	clear(data)
}

func checkdbtwostrings(tbl []database.DbstaticTwoString, str1, str2 string) bool {
	for idxepi := range tbl {
		if strings.EqualFold(tbl[idxepi].Str1, str1) && strings.EqualFold(tbl[idxepi].Str2, str2) {
			return true
		}
	}
	return false
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
	return "0" + strconv.Itoa(value)
}

// GetTraktSerieSeasonEpisodes retrieves all episodes for the given show ID and season from the Trakt API.
// It takes the show ID and season number as parameters.
// Returns a slice of TraktSerieSeasonEpisodes structs containing the episode data,
// or nil if there is an error.
func GetTraktSerieSeasonEpisodes(showid string, season string) []traktSerieSeasonEpisodes {
	if showid == "" || season == "" || traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	//result, err := DoJSONTypeG[traktSerieSeasonEpisodes](traktAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurlshows, showid, "seasons", season), extendedfull), &traktAPI.DefaultHeaders)
	result, err := DoJSONTypeG[traktSerieSeasonEpisodes](&traktAPI.Client, logger.JoinStrings(logger.URLJoinPath(apiurlshows, showid, "seasons", season), extendedfull), traktAPI.DefaultHeaders...)
	if err != nil {
		return nil
	}
	return result
}

// GetTraktUserList retrieves a Trakt user list by username, list name, list type,
// and optional limit. It returns a slice of TraktUserList structs containing
// the list data, and an error.
func GetTraktUserList(username string, listname string, listtype string, limit string) ([]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" || traktAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	return DoJSONTypeG[TraktUserList](&traktAPI.Client, traktaddlimit(logger.URLJoinPath("https://api.trakt.tv/users/", username, "lists", listname, "items", listtype), limit), traktAPI.DefaultHeaders...)
}

// GetTraktSeriePopular retrieves popular TV shows from Trakt based on the
// number of watches and list additions. It takes an optional limit parameter
// to limit the number of results returned. Returns a slice of TraktSerie
// structs containing the popular show data.
func GetTraktSeriePopular(limit string) []TraktSerie {
	if traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[TraktSerie](&traktAPI.Client, traktaddlimit("https://api.trakt.tv/shows/popular", limit), traktAPI.DefaultHeaders...)
	if err != nil {
		return nil
	}
	return result
}

// GetTraktSerieTrending retrieves the trending TV shows from Trakt based on the limit parameter.
// It returns a slice of TraktSerieTrending structs containing the trending show data.
func GetTraktSerieTrending(limit string) []TraktSerieTrending {
	if traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[TraktSerieTrending](&traktAPI.Client, traktaddlimit("https://api.trakt.tv/shows/trending", limit), traktAPI.DefaultHeaders...)

	if err != nil {
		return nil
	}
	return result
}

// GetTraktSerieAnticipated retrieves the most anticipated TV shows from Trakt
// based on the number of list adds. It takes an optional limit parameter to limit
// the number of results returned. Returns a slice of TraktSerieAnticipated structs
// containing the anticipated show data.
func GetTraktSerieAnticipated(limit string) []TraktSerieAnticipated {
	if traktAPI.Client.checklimiterwithdaily() {
		return nil
	}
	result, err := DoJSONTypeG[TraktSerieAnticipated](&traktAPI.Client, traktaddlimit("https://api.trakt.tv/shows/anticipated", limit), traktAPI.DefaultHeaders...)

	if err != nil {
		return nil
	}
	return result
}

// GetTraktToken returns the token used to authenticate with Trakt. This is a wrapper around the traktAPI
func GetTraktToken() *oauth2.Token {
	return traktAPI.Token
}

// SetTraktToken sets the OAuth 2.0 token used to authenticate
// with the Trakt API.
func SetTraktToken(tk *oauth2.Token) {
	traktAPI.Token = tk
}

// GetTraktAuthURL generates an authorization URL that redirects the user
// to the Trakt consent page to request permission for the configured scopes.
// It returns the generated authorization URL.
func GetTraktAuthURL() string {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	urlv := traktAPI.Auth.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Println("Visit the URL for the auth dialog: " + urlv)

	return urlv
}

// GetTraktAuthToken exchanges the authorization code for an OAuth 2.0 token
// for the Trakt API. It takes the client code and returns the token, or nil and an
// error if there was an issue exchanging the code.
func GetTraktAuthToken(clientcode string) *oauth2.Token {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	ctx := context.Background()
	tok, err := traktAPI.Auth.Exchange(ctx, clientcode)
	if err != nil {
		logger.LogDynamic("error", "Error getting token", logger.NewLogFieldValue(err))
	}
	ctx.Done()
	return tok
}

// GetTraktUserListAuth retrieves a Trakt user list with authentication.
// It takes the username, list name, list type, and optional limit parameters and returns
// the user list items as an array of TraktUserList structs and an error.
// Returns ErrNotFound if username, listname or listtype are empty.
func GetTraktUserListAuth(username string, listname string, listtype string, limit string) ([]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" || traktAPI.Client.checklimiterwithdaily() {
		return nil, logger.ErrNotFound
	}
	return DoJSONTypeG[TraktUserList](&traktAPI.Client, traktaddlimit(logger.URLJoinPath("https://api.trakt.tv/users/", username, "lists", listname, "items", listtype), limit), traktAPI.DefaultHeaders...)
}
