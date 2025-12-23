package apiexternal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
)

// jellyfinClient is a struct for interacting with Jellyfin API.
type jellyfinClient struct {
	Client rlHTTPClient
	Lim    *slidingwindow.Limiter
}

// NewJellyfinClient creates a new jellyfinClient instance.
func NewJellyfinClient(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	if timeoutseconds == 0 {
		timeoutseconds = 30
	}

	lim := slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls))
	client := &jellyfinClient{
		Lim: lim,
		Client: newClient(
			"jellyfin",
			disabletls,
			true,
			lim,
			false, nil, timeoutseconds,
		),
	}
	setJellyfinAPI(client)
}

// Helper function for thread-safe access to jellyfinAPI.
func getJellyfinClient() *rlHTTPClient {
	api := getJellyfinAPI()
	if api == nil {
		return nil
	}

	return &api.Client
}

// JellyfinWatchlistItem represents an item from a Jellyfin watchlist.
type JellyfinWatchlistItem struct {
	Name                    string            `json:"Name"`
	ServerId                string            `json:"ServerId"`
	Id                      string            `json:"Id"`
	RunTimeTicks            int64             `json:"RunTimeTicks,omitempty"`
	ProductionYear          int               `json:"ProductionYear,omitempty"`
	Type                    string            `json:"Type"`
	UserData                JellyfinUserData  `json:"UserData,omitempty"`
	PrimaryImageAspectRatio float64           `json:"PrimaryImageAspectRatio,omitempty"`
	OriginalTitle           string            `json:"OriginalTitle,omitempty"`
	Overview                string            `json:"Overview,omitempty"`
	CommunityRating         float64           `json:"CommunityRating,omitempty"`
	DateCreated             string            `json:"DateCreated,omitempty"`
	MediaType               string            `json:"MediaType,omitempty"`
	ProviderIds             map[string]string `json:"ProviderIds,omitempty"`
	IsFolder                bool              `json:"IsFolder,omitempty"`
	ParentId                string            `json:"ParentId,omitempty"`
	GenreItems              []JellyfinGenre   `json:"GenreItems,omitempty"`
	ImageTags               map[string]string `json:"ImageTags,omitempty"`
	BackdropImageTags       []string          `json:"BackdropImageTags,omitempty"`
}

// JellyfinUserData contains user-specific data for Jellyfin items.
type JellyfinUserData struct {
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	PlayCount             int    `json:"PlayCount"`
	IsFavorite            bool   `json:"IsFavorite"`
	Played                bool   `json:"Played"`
	Key                   string `json:"Key,omitempty"`
}

// JellyfinGenre represents a genre in Jellyfin.
type JellyfinGenre struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

// JellyfinWatchlistResponse represents the response from Jellyfin watchlist API.
type JellyfinWatchlistResponse struct {
	Items            []JellyfinWatchlistItem `json:"Items"`
	TotalRecordCount int                     `json:"TotalRecordCount"`
	StartIndex       int                     `json:"StartIndex"`
}

// JellyfinUser represents a Jellyfin user.
type JellyfinUser struct {
	Name                      string `json:"Name"`
	ServerId                  string `json:"ServerId"`
	Id                        string `json:"Id"`
	PrimaryImageTag           string `json:"PrimaryImageTag,omitempty"`
	HasPassword               bool   `json:"HasPassword"`
	HasConfiguredPassword     bool   `json:"HasConfiguredPassword"`
	HasConfiguredEasyPassword bool   `json:"HasConfiguredEasyPassword"`
	EnableAutoLogin           bool   `json:"EnableAutoLogin"`
	LastLoginDate             string `json:"LastLoginDate,omitempty"`
	LastActivityDate          string `json:"LastActivityDate,omitempty"`
}

// JellyfinUsersResponse represents the response from Jellyfin users API.
type JellyfinUsersResponse []JellyfinUser

// GetJellyfinWatchlist retrieves the watchlist from a Jellyfin server.
func GetJellyfinWatchlist(serverURL, apiKey, username string) ([]JellyfinWatchlistItem, error) {
	if serverURL == "" || apiKey == "" || username == "" {
		return nil, fmt.Errorf("jellyfin server URL, API key, and username are required")
	}

	// First, get the user ID from the username
	userID, err := getJellyfinUserID(serverURL, apiKey, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get Jellyfin user ID: %w", err)
	}

	// Build the watchlist URL - Jellyfin uses favorites as watchlist
	watchlistURL := fmt.Sprintf("%s/Users/%s/Items", strings.TrimSuffix(serverURL, "/"), userID)

	params := url.Values{}
	params.Set("api_key", apiKey)
	params.Set("IsFavorite", "true")
	params.Set("IncludeItemTypes", "Movie,Series")
	params.Set("Recursive", "true")
	params.Set("Fields", "ProviderIds,Overview,Genres,CommunityRating,DateCreated")
	params.Set("limit", "1000") // Reasonable limit for watchlists

	fullURL := fmt.Sprintf("%s?%s", watchlistURL, params.Encode())

	logger.Logtype("debug", 1).
		Str("url", serverURL).
		Msg("Fetching Jellyfin watchlist")

	var response JellyfinWatchlistResponse

	err = ProcessHTTP(
		getJellyfinClient(),
		fullURL,
		false,
		func(ctx context.Context, r *http.Response) error {
			return json.NewDecoder(r.Body).Decode(&response)
		},
		nil,
	)
	if err != nil {
		logger.Logtype("error", 1).
			Str("url", serverURL).
			Err(err).
			Msg("Failed to fetch Jellyfin watchlist")

		return nil, err
	}

	logger.Logtype("info", 1).
		Str("count", strconv.Itoa(len(response.Items))).
		Msg("Successfully fetched Jellyfin watchlist")

	return response.Items, nil
}

// getJellyfinUserID retrieves the user ID for a given username.
func getJellyfinUserID(serverURL, apiKey, username string) (string, error) {
	usersURL := fmt.Sprintf("%s/Users", strings.TrimSuffix(serverURL, "/"))

	params := url.Values{}
	params.Set("api_key", apiKey)

	fullURL := fmt.Sprintf("%s?%s", usersURL, params.Encode())

	var response JellyfinUsersResponse

	err := ProcessHTTP(
		getJellyfinClient(),
		fullURL,
		false,
		func(ctx context.Context, r *http.Response) error {
			return json.NewDecoder(r.Body).Decode(&response)
		},
		nil,
	)
	if err != nil {
		return "", err
	}

	// Find the user by username
	for _, user := range response {
		if strings.EqualFold(user.Name, username) {
			return user.Id, nil
		}
	}

	return "", fmt.Errorf("user '%s' not found on Jellyfin server", username)
}

// ExtractIMDBFromJellyfinItem extracts IMDB ID from a Jellyfin watchlist item.
func ExtractIMDBFromJellyfinItem(item JellyfinWatchlistItem) string {
	if item.ProviderIds != nil {
		if imdbID, exists := item.ProviderIds["Imdb"]; exists {
			return imdbID
		}
	}

	return ""
}

// ExtractTVDBFromJellyfinItem extracts TVDB ID from a Jellyfin watchlist item.
func ExtractTVDBFromJellyfinItem(item JellyfinWatchlistItem) int {
	if item.ProviderIds != nil {
		if tvdbIDStr, exists := item.ProviderIds["Tvdb"]; exists {
			if tvdbID, err := strconv.Atoi(tvdbIDStr); err == nil {
				return tvdbID
			}
		}
	}

	return 0
}

// ExtractTMDBFromJellyfinItem extracts TMDB ID from a Jellyfin watchlist item.
func ExtractTMDBFromJellyfinItem(item JellyfinWatchlistItem) int {
	if item.ProviderIds != nil {
		if tmdbIDStr, exists := item.ProviderIds["Tmdb"]; exists {
			if tmdbID, err := strconv.Atoi(tmdbIDStr); err == nil {
				return tmdbID
			}
		}
	}

	return 0
}

// IsJellyfinItemMovie determines if a Jellyfin item is a movie.
func IsJellyfinItemMovie(item JellyfinWatchlistItem) bool {
	return item.Type == "Movie"
}

// IsJellyfinItemSeries determines if a Jellyfin item is a TV series.
func IsJellyfinItemSeries(item JellyfinWatchlistItem) bool {
	return item.Type == "Series"
}

// GetJellyfinItemTitle returns the appropriate title for the item.
func GetJellyfinItemTitle(item JellyfinWatchlistItem) string {
	if item.OriginalTitle != "" {
		return item.OriginalTitle
	}
	return item.Name
}
