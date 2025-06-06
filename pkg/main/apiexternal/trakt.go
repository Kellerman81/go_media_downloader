package apiexternal

import (
	"context"
	"fmt"
	"net/url"
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
	// ListCount int        `json:"list_count"`
	Movie TraktMovie `json:"movie"`
}

type TraktMovieTrending struct {
	// Watchers int        `json:"watchers"`
	Movie TraktMovie `json:"movie"`
}

type TraktUserList struct {
	// Rank      int        `json:"rank"`
	// ID        int        `json:"id"`
	// Notes     string     `json:"notes"`
	Movie     TraktMovie `json:"movie"`
	Serie     TraktSerie `json:"show"`
	TraktType string     `json:"type"`
}

type TraktMovie struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

type TraktSerieTrending struct {
	// Watchers int        `json:"watchers"`
	Serie TraktSerie `json:"show"`
}

type TraktSerieAnticipated struct {
	// ListCount int        `json:"list_count"`
	Serie TraktSerie `json:"show"`
}

type TraktSerieSeason struct {
	Number string `json:"number"`
	// Ids    Ids `json:"ids"`
}

type TraktSerieSeasonEpisodes struct {
	Title      string    `json:"title"`
	Overview   string    `json:"overview"`
	FirstAired time.Time `json:"first_aired"`
	Season     int       `json:"season"`
	Episode    int       `json:"number"`
	Runtime    int       `json:"runtime"`
	// Ids                   Ids       `json:"ids"`
	// EpisodeAbs            int       `json:"number_abs"`
	// Rating                float32   `json:"rating"`
	// Votes                 int       `json:"votes"`
	// Comments              int       `json:"comment_count"`
	// AvailableTranslations []string  `json:"available_translations"`
}

type TraktSerie struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

type TraktSerieData struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Genres     []string  `json:"genres"`
	Title      string    `json:"title"`
	Overview   string    `json:"overview"`
	Network    string    `json:"network"`
	Country    string    `json:"country"`
	Status     string    `json:"status"`
	Language   string    `json:"language"`
	FirstAired time.Time `json:"first_aired"`
	Rating     float32   `json:"rating"`
	Year       int       `json:"year"`
	Runtime    int       `json:"runtime"`
	// AvailableTranslations []string  `json:"available_translations"`
	// Certification         string    `json:"certification"`
	// Trailer               string    `json:"trailer"`
	// Homepage              string    `json:"homepage"`
	// Votes                 int       `json:"votes"`
	// Comments              int       `json:"comment_count"`
	// AiredEpisodes         int       `json:"aired_episodes"`
}

type TraktAlias struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

type TraktMovieExtend struct {
	IDs struct {
		Slug   string `json:"slug"`
		Imdb   string `json:"imdb"`
		Trakt  int    `json:"trakt"`
		Tmdb   int    `json:"tmdb"`
		Tvdb   int    `json:"tvdb"`
		Tvrage int    `json:"tvrage"`
	} `json:"ids"`
	Genres   []string `json:"genres"`
	Title    string   `json:"title"`
	Tagline  string   `json:"tagline"`
	Overview string   `json:"overview"`
	Released string   `json:"released"`
	Status   string   `json:"status"`
	Language string   `json:"language"`
	Rating   float32  `json:"rating"`
	Runtime  int      `json:"runtime"`
	Comments int      `json:"comment_count"`
	Votes    int32    `json:"votes"`
	Year     uint16   `json:"year"`
	// AvailableTranslations []string `json:"available_translations"`
	// Country               string   `json:"country"`
	// Trailer               string   `json:"trailer"`
	// Homepage              string   `json:"homepage"`
	// Certification         string   `json:"certification"`
}

// traktClient is a struct for interacting with the Trakt API
// It contains fields for the API key, client ID, client secret, HTTP client,
// OAuth2 config, access token, and default headers.
type traktClient struct {
	Client         rlHTTPClient // The HTTP client for requests
	Lim            slidingwindow.Limiter
	DefaultHeaders map[string][]string // Default headers to send with requests
	Auth           oauth2.Config       // The OAuth2 config
	APIKey         string              // The API key for authentication
	ClientID       string              // The client ID for OAuth2
	ClientSecret   string              // The client secret for OAuth2
	Token          *oauth2.Token       // The OAuth2 access token
}

// NewTraktClient initializes a new traktClient instance for making requests to
// the Trakt API. It takes in credentials and rate limiting settings and sets up
// the OAuth2 configuration.
func NewTraktClient(
	clientid, clientsecret string,
	seconds uint8,
	calls int,
	disabletls bool,
	timeoutseconds uint16,
) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	traktAPI = traktClient{
		ClientID:     clientid,
		ClientSecret: clientsecret,
		Lim:          slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls)),
		Client: NewClient(
			"trakt",
			disabletls,
			true,
			&traktAPI.Lim,
			false, nil, timeoutseconds),
		Auth: oauth2.Config{
			ClientID:     clientid,
			ClientSecret: clientsecret,
			RedirectURL:  "http://localhost:9090",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://api.trakt.tv/oauth/authorize",
				TokenURL: "https://api.trakt.tv/oauth/token",
			},
		},
		Token: config.GetTrakt(),
		DefaultHeaders: map[string][]string{
			"Content-Type":      {"application/json"},
			"trakt-api-version": {"2"},
			"trakt-api-key":     {clientid},
		},
	}
	if traktAPI.Token.AccessToken != "" {
		traktAPI.DefaultHeaders["Authorization"] = []string{"Bearer " + traktAPI.Token.AccessToken}
	}
}

// GetTraktMoviePopular retrieves a list of popular movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovie structs containing the movie data,
// or nil if there was an error.
func GetTraktMoviePopular(limit *string) []TraktMovie {
	arr, _ := doJSONType[[]TraktMovie](
		&traktAPI.Client,
		traktaddlimit("https://api.trakt.tv/movies/popular", limit),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// GetTraktMovieTrending retrieves a list of trending movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieTrending structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieTrending(limit *string) []TraktMovieTrending {
	arr, _ := doJSONType[[]TraktMovieTrending](
		&traktAPI.Client,
		traktaddlimit("https://api.trakt.tv/movies/trending", limit),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// traktaddlimit adds a limit query parameter to the Trakt API URL if a limit is specified.
// It takes the limit as a string parameter.
// Returns a URL query string with the limit, or an empty string if no limit provided.
func traktaddlimit(urlv string, limit *string) string {
	if limit != nil && *limit != "" && *limit != "0" {
		return (urlv + "?limit=" + *limit)
	}
	return urlv
}

// GetTraktMovieAnticipated retrieves a list of anticipated movies from the Trakt API.
// The limit parameter allows specifying the maximum number of movies to return.
// It returns a slice of TraktMovieAnticipated structs containing the movie data,
// or nil if there was an error.
func GetTraktMovieAnticipated(limit *string) []TraktMovieAnticipated {
	arr, _ := doJSONType[[]TraktMovieAnticipated](
		&traktAPI.Client,
		traktaddlimit("https://api.trakt.tv/movies/anticipated", limit),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// GetTraktMovieAliases retrieves alias data from the Trakt API for the given movie ID.
// It takes a Trakt movie ID string as a parameter.
// Returns a slice of TraktAlias structs containing the alias data,
// or nil if there is an error or no aliases found.
func GetTraktMovieAliases(movieid string) []TraktAlias {
	if movieid == "" {
		return nil
	}
	arr, _ := doJSONType[[]TraktAlias](
		&traktAPI.Client,
		logger.JoinStrings(apiurlmovies, movieid, "/aliases"),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// GetTraktMovie retrieves extended data for a Trakt movie by ID.
// It takes a movie ID string as input.
// Returns a TraktMovieExtend struct containing the movie data,
// or nil and an error if the movie is not found or there is an error fetching data.
func GetTraktMovie(movieid string) (*TraktMovieExtend, error) {
	if movieid == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TraktMovieExtend](
		&traktAPI.Client,
		logger.JoinStrings(apiurlmovies, movieid, extendedfull),
		traktAPI.DefaultHeaders,
	)
}

// GetTraktSerie retrieves extended data for a Trakt TV show by its Trakt ID.
// It takes the Trakt show ID as a string parameter.
// It returns a TraktSerieData struct containing the show data,
// or nil and an error if the show ID is invalid or there was an error retrieving data.
func GetTraktSerie(showid string) (*TraktSerieData, error) {
	if showid == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONTypeP[TraktSerieData](
		&traktAPI.Client,
		logger.JoinStrings(apiurlshows, showid, extendedfull),
		traktAPI.DefaultHeaders,
	)
}

// GetTraktSerieAliases retrieves alias data from the Trakt API for the given Dbserie.
// It first checks if there is a Trakt ID available and uses that to retrieve aliases.
// If no Trakt ID, it falls back to using the IMDb ID if available.
// Returns a slice of TraktAlias structs or nil if no aliases found.
func GetTraktSerieAliases(dbserie *database.Dbserie) []TraktAlias {
	urlpart := dbserie.ImdbID
	if dbserie.TraktID == 0 {
		if dbserie.ImdbID == "" {
			return nil
		}
	} else {
		urlpart = strconv.Itoa(dbserie.TraktID)
	}
	arr, _ := doJSONType[[]TraktAlias](
		&traktAPI.Client,
		logger.JoinStrings(apiurlshows, urlpart, "/aliases"),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// GetTraktSerieSeasons retrieves a list of season numbers for a Trakt TV show by ID.
// It returns a slice of season numbers as strings, or nil if there is an error.
func GetTraktSerieSeasons(showid string) ([]TraktSerieSeason, error) {
	if showid == "" {
		return nil, nil
	}
	return doJSONType[[]TraktSerieSeason](
		&traktAPI.Client,
		logger.JoinStrings(apiurlshows, showid, "/seasons"),
		traktAPI.DefaultHeaders,
	)
}

// GetTraktSerieSeasonsAndEpisodes retrieves all seasons and episodes for the given Trakt show ID from the Trakt API.
// It takes the show ID and database series ID as parameters.
// It queries the local database for existing episodes to avoid duplicates.
// For each season, it calls addtraktdbepisodes to insert any missing episodes into the database.
// Returns nothing.
func UpdateTraktSerieSeasonsAndEpisodes(showid string, id *uint) {
	if showid == "" {
		return
	}
	baseurl := (apiurlshows + showid + "/seasons")
	result, err := doJSONType[[]TraktSerieSeason](
		&traktAPI.Client,
		baseurl,
		traktAPI.DefaultHeaders,
	)
	if err != nil {
		return
	}
	tbl := database.Getrows1size[database.DbstaticTwoString](
		false,
		database.QueryDbserieEpisodesCountByDBID,
		database.QueryDbserieEpisodesGetSeasonEpisodeByDBID,
		id,
	)
	for idx := range result {
		data, _ := doJSONType[[]TraktSerieSeasonEpisodes](
			&traktAPI.Client,
			logger.JoinStrings(baseurl, "/", result[idx].Number, extendedfull),
			traktAPI.DefaultHeaders,
		)
		for idx2 := range data {
			if checkdbtwostrings(tbl, data[idx2].Season, data[idx2].Episode) {
				continue
			}
			epi := strconv.Itoa(data[idx2].Episode)
			seas := strconv.Itoa(data[idx2].Season)
			ident := generateIdentifierStringFromInt(&data[idx2].Season, &data[idx2].Episode)
			database.ExecN(
				"insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
				&epi,
				&seas,
				&ident,
				&data[idx2].Title,
				&data[idx2].FirstAired,
				&data[idx2].Overview,
				id,
			)
		}
	}
}

func Testaddtraktdbepisodes() ([]TraktSerieSeasonEpisodes, error) {
	urlv, _ := url.JoinPath(apiurlshows, "tt1183865", "seasons", "1")
	return doJSONType[[]TraktSerieSeasonEpisodes](&traktAPI.Client, urlv, traktAPI.DefaultHeaders)
}

// checkdbtwostrings checks if the given integer values int1 and int2 exist as a pair in the provided slice of database.DbstaticTwoString.
// It returns true if the pair is found, false otherwise.
func checkdbtwostrings(tbl []database.DbstaticTwoString, int1, int2 int) bool {
	if len(tbl) == 0 {
		return false
	}
	v := database.DbstaticTwoString{Str1: strconv.Itoa(int1), Str2: strconv.Itoa(int2)}
	for idx := range tbl {
		if tbl[idx] == v {
			return true
		}
	}
	return false
}

// padNumberWithZero pads an integer value with leading zeros to ensure it is at least two digits.
// If the value is already two or more digits, it is returned unchanged as a string.
func padNumberWithZero(value *int) string {
	if *value == 0 {
		return "0"
	}
	if *value >= 10 {
		return strconv.Itoa(*value)
	}
	return ("0" + strconv.Itoa(*value))
}

// GetTraktSerieSeasonEpisodes retrieves all episodes for the given show ID and season from the Trakt API.
// It takes the show ID and season number as parameters.
// Returns a slice of TraktSerieSeasonEpisodes structs containing the episode data,
// or nil if there is an error.
func GetTraktSerieSeasonEpisodes(showid, season string) ([]TraktSerieSeasonEpisodes, error) {
	if showid == "" || season == "" {
		return nil, errDailyLimit
	}
	return doJSONType[[]TraktSerieSeasonEpisodes](
		&traktAPI.Client,
		logger.JoinStrings(apiurlshows, showid, "/seasons/", season, extendedfull),
		traktAPI.DefaultHeaders,
	)
}

// GetTraktUserList retrieves a Trakt user list by username, list name, list type,
// and optional limit. It returns a slice of TraktUserList structs containing
// the list data, and an error.
func GetTraktUserList(username, listname, listtype string, limit *string) ([]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONType[[]TraktUserList](
		&traktAPI.Client,
		traktaddlimit(
			logger.JoinStrings(
				"https://api.trakt.tv/users/",
				username,
				"/lists/",
				listname,
				"/items/",
				listtype,
			),
			limit,
		),
		traktAPI.DefaultHeaders,
	)
}

// RemoveMovieFromTraktUserList removes the specified movie from the given Trakt user list.
// It takes the username, list name, and the IMDB ID of the movie to remove as parameters.
// If the username or list name are empty, it returns an error.
func RemoveMovieFromTraktUserList(username, listname, remove string) error {
	if username == "" || listname == "" {
		return logger.ErrNotFound
	}
	body := strings.NewReader(fmt.Sprintf(`{"movies": [{"ids": {"imdb": "%s"}}]}`, remove))
	return ProcessHTTP(
		&traktAPI.Client,
		logger.JoinStrings(
			"https://api.trakt.tv/users/",
			username,
			"/lists/",
			listname,
			"/items/remove",
		),
		true,
		nil,
		traktAPI.DefaultHeaders,
		body,
	)
}

// RemoveSerieFromTraktUserList removes the specified TV show from the given Trakt user list.
// It takes the username, list name, and the TVDB ID of the show to remove as parameters.
// If the username or list name are empty, it returns an error.
func RemoveSerieFromTraktUserList(username, listname string, remove int) error {
	if username == "" || listname == "" {
		return logger.ErrNotFound
	}
	body := strings.NewReader(fmt.Sprintf(`{"shows": [{"ids": {"tvdb": %d}}]}`, remove))
	return ProcessHTTP(
		&traktAPI.Client,
		logger.JoinStrings(
			"https://api.trakt.tv/users/",
			username,
			"/lists/",
			listname,
			"/items/remove",
		),
		true,
		nil,
		traktAPI.DefaultHeaders,
		body,
	)
}

// GetTraktSeriePopular retrieves popular TV shows from Trakt based on the
// number of watches and list additions. It takes an optional limit parameter
// to limit the number of results returned. Returns a slice of TraktSerie
// structs containing the popular show data.
func GetTraktSeriePopular(limit *string) []TraktSerie {
	arr, _ := doJSONType[[]TraktSerie](
		&traktAPI.Client,
		traktaddlimit("https://api.trakt.tv/shows/popular", limit),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// GetTraktSerieTrending retrieves the trending TV shows from Trakt based on the limit parameter.
// It returns a slice of TraktSerieTrending structs containing the trending show data.
func GetTraktSerieTrending(limit *string) []TraktSerieTrending {
	arr, _ := doJSONType[[]TraktSerieTrending](
		&traktAPI.Client,
		traktaddlimit("https://api.trakt.tv/shows/trending", limit),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// GetTraktSerieAnticipated retrieves the most anticipated TV shows from Trakt
// based on the number of list adds. It takes an optional limit parameter to limit
// the number of results returned. Returns a slice of TraktSerieAnticipated structs
// containing the anticipated show data.
func GetTraktSerieAnticipated(limit *string) []TraktSerieAnticipated {
	arr, _ := doJSONType[[]TraktSerieAnticipated](
		&traktAPI.Client,
		traktaddlimit("https://api.trakt.tv/shows/anticipated", limit),
		traktAPI.DefaultHeaders,
	)
	return arr
}

// GetTraktToken returns the token used to authenticate with Trakt. This is a wrapper around the traktAPI.
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
	fmt.Println("Visit the URL for the auth dialog: ", urlv)

	return urlv
}

// GetTraktAuthToken exchanges the authorization code for an OAuth 2.0 token
// for the Trakt API. It takes the client code and returns the token, or nil and an
// error if there was an issue exchanging the code.
func GetTraktAuthToken(clientcode string) *oauth2.Token {
	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	tok, err := traktAPI.Auth.Exchange(context.Background(), clientcode)
	if err != nil {
		logger.LogDynamicanyErr("error", "Error getting token", err)
	}
	return tok
}

// GetTraktUserListAuth retrieves a Trakt user list with authentication.
// It takes the username, list name, list type, and optional limit parameters and returns
// the user list items as an array of TraktUserList structs and an error.
// Returns ErrNotFound if username, listname or listtype are empty.
func GetTraktUserListAuth(
	username, listname, listtype string,
	limit *string,
) ([]TraktUserList, error) {
	if username == "" || listname == "" || listtype == "" {
		return nil, logger.ErrNotFound
	}
	return doJSONType[[]TraktUserList](
		&traktAPI.Client,
		traktaddlimit(
			logger.JoinStrings(
				"https://api.trakt.tv/users/",
				username,
				"/lists/",
				listname,
				"/items/",
				listtype,
			),
			limit,
		),
		traktAPI.DefaultHeaders,
	)
}
