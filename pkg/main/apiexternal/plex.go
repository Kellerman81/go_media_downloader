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

// plexClient is a struct for interacting with Plex API.
type plexClient struct {
	Client rlHTTPClient
	Lim    *slidingwindow.Limiter
}

// NewPlexClient creates a new plexClient instance.
func NewPlexClient(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
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
	client := &plexClient{
		Lim: lim,
		Client: newClient(
			"plex",
			disabletls,
			true,
			lim,
			false, nil, timeoutseconds,
		),
	}
	setPlexAPI(client)
}

// Helper function for thread-safe access to plexAPI.
func getPlexClient() *rlHTTPClient {
	api := getPlexAPI()
	if api == nil {
		return nil
	}

	return &api.Client
}

// PlexWatchlistItem represents an item from a Plex watchlist.
type PlexWatchlistItem struct {
	RatingKey             string     `json:"ratingKey"`
	Key                   string     `json:"key"`
	GUID                  string     `json:"guid"`
	Type                  string     `json:"type"`
	Title                 string     `json:"title"`
	OriginalTitle         string     `json:"originalTitle,omitempty"`
	Year                  int        `json:"year"`
	Summary               string     `json:"summary,omitempty"`
	Rating                string     `json:"rating,omitempty"`
	UserRating            string     `json:"userRating,omitempty"`
	Duration              int        `json:"duration,omitempty"`
	AddedAt               int64      `json:"addedAt"`
	UpdatedAt             int64      `json:"updatedAt"`
	Thumb                 string     `json:"thumb,omitempty"`
	Art                   string     `json:"art,omitempty"`
	ContentRating         string     `json:"contentRating,omitempty"`
	OriginallyAvailableAt string     `json:"originallyAvailableAt,omitempty"`
	GUIDS                 []PlexGUID `json:"Guid,omitempty"`
}

// PlexGUID represents external IDs for Plex items.
type PlexGUID struct {
	ID string `json:"id"`
}

// PlexWatchlistResponse represents the response from Plex watchlist API.
type PlexWatchlistResponse struct {
	MediaContainer PlexMediaContainer `json:"MediaContainer"`
}

// PlexMediaContainer contains the actual watchlist items.
type PlexMediaContainer struct {
	Size                int                 `json:"size"`
	AllowSync           int                 `json:"allowSync"`
	Identifier          string              `json:"identifier"`
	LibrarySectionID    int                 `json:"librarySectionID"`
	LibrarySectionTitle string              `json:"librarySectionTitle"`
	LibrarySectionUUID  string              `json:"librarySectionUUID"`
	MediaTagPrefix      string              `json:"mediaTagPrefix"`
	MediaTagVersion     int                 `json:"mediaTagVersion"`
	Metadata            []PlexWatchlistItem `json:"Metadata"`
}

// PlexUserResponse represents user account information from Plex.
type PlexUserResponse struct {
	MediaContainer struct {
		Size int        `json:"size"`
		User []PlexUser `json:"User"`
	} `json:"MediaContainer"`
}

// PlexUser represents a Plex user account.
type PlexUser struct {
	ID       int    `json:"id"`
	UUID     string `json:"uuid"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Email    string `json:"email"`
	Thumb    string `json:"thumb"`
}

// GetPlexWatchlist retrieves the watchlist from a Plex server.
func GetPlexWatchlist(serverURL, token, username string) ([]PlexWatchlistItem, error) {
	if serverURL == "" || token == "" || username == "" {
		return nil, fmt.Errorf("plex server URL, token, and username are required")
	}

	// Note: For now we're using the watchlist endpoint directly
	// In the future we might need user ID for more specific API calls

	// Build the watchlist URL
	watchlistURL := fmt.Sprintf("%s/playlists/all", strings.TrimSuffix(serverURL, "/"))

	params := url.Values{}
	params.Set("X-Plex-Token", token)
	params.Set("type", "15") // Watchlist type
	params.Set("includeGuids", "1")

	fullURL := fmt.Sprintf("%s?%s", watchlistURL, params.Encode())

	logger.Logtype("debug", 1).
		Str("url", serverURL).
		Msg("Fetching Plex watchlist")

	var response PlexWatchlistResponse

	err := ProcessHTTP(
		getPlexClient(),
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
			Msg("Failed to fetch Plex watchlist")

		return nil, err
	}

	logger.Logtype("info", 1).
		Str("count", strconv.Itoa(len(response.MediaContainer.Metadata))).
		Msg("Successfully fetched Plex watchlist")

	return response.MediaContainer.Metadata, nil
}

// getPlexUserID retrieves the user ID for a given username.
func getPlexUserID(serverURL, token, username string) (int, error) {
	usersURL := fmt.Sprintf("%s/accounts", strings.TrimSuffix(serverURL, "/"))

	params := url.Values{}
	params.Set("X-Plex-Token", token)

	fullURL := fmt.Sprintf("%s?%s", usersURL, params.Encode())

	var response PlexUserResponse

	err := ProcessHTTP(
		getPlexClient(),
		fullURL,
		false,
		func(ctx context.Context, r *http.Response) error {
			return json.NewDecoder(r.Body).Decode(&response)
		},
		nil,
	)
	if err != nil {
		return 0, err
	}

	// Find the user by username
	for _, user := range response.MediaContainer.User {
		if strings.EqualFold(user.Username, username) || strings.EqualFold(user.Email, username) {
			return user.ID, nil
		}
	}

	return 0, fmt.Errorf("user '%s' not found on Plex server", username)
}

// ExtractIMDBFromPlexItem extracts IMDB ID from a Plex watchlist item.
func ExtractIMDBFromPlexItem(item PlexWatchlistItem) string {
	// Check GUIDs for IMDB ID
	for _, guid := range item.GUIDS {
		if strings.HasPrefix(guid.ID, "imdb://") {
			return strings.TrimPrefix(guid.ID, "imdb://")
		}
	}

	// Fallback: check main GUID field
	if strings.HasPrefix(item.GUID, "imdb://") {
		return strings.TrimPrefix(item.GUID, "imdb://")
	}

	return ""
}

// ExtractTVDBFromPlexItem extracts TVDB ID from a Plex watchlist item.
func ExtractTVDBFromPlexItem(item PlexWatchlistItem) int {
	// Check GUIDs for TVDB ID
	for _, guid := range item.GUIDS {
		if strings.HasPrefix(guid.ID, "tvdb://") {
			tvdbStr := strings.TrimPrefix(guid.ID, "tvdb://")
			if tvdbID, err := strconv.Atoi(tvdbStr); err == nil {
				return tvdbID
			}
		}
	}

	// Fallback: check main GUID field
	if strings.HasPrefix(item.GUID, "tvdb://") {
		tvdbStr := strings.TrimPrefix(item.GUID, "tvdb://")
		if tvdbID, err := strconv.Atoi(tvdbStr); err == nil {
			return tvdbID
		}
	}

	return 0
}

// IsPlexItemMovie determines if a Plex item is a movie.
func IsPlexItemMovie(item PlexWatchlistItem) bool {
	return item.Type == "movie"
}

// IsPlexItemShow determines if a Plex item is a TV show.
func IsPlexItemShow(item PlexWatchlistItem) bool {
	return item.Type == "show"
}
