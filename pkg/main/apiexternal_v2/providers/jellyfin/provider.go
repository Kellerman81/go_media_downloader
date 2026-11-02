package jellyfin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

var errMissingServer = errors.New("jellyfin server URL, API key, and username are required")

// Provider reads watchlists (favorites) from Jellyfin servers. The server is
// given per call, since every list config may point at another one.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a Jellyfin provider.
func NewProviderWithConfig(cfg base.ClientConfig) *Provider {
	cfg.Name = "jellyfin"

	return &Provider{BaseClient: base.NewBaseClient(cfg)}
}

// WatchlistItem is a favorite movie or series of a Jellyfin user.
type WatchlistItem struct {
	Name                    string            `json:"Name"`
	ServerId                string            `json:"ServerId"`
	Id                      string            `json:"Id"`
	RunTimeTicks            int64             `json:"RunTimeTicks,omitempty"`
	ProductionYear          int               `json:"ProductionYear,omitempty"`
	Type                    string            `json:"Type"`
	UserData                UserData          `json:"UserData"`
	PrimaryImageAspectRatio float64           `json:"PrimaryImageAspectRatio,omitempty"`
	OriginalTitle           string            `json:"OriginalTitle,omitempty"`
	Overview                string            `json:"Overview,omitempty"`
	CommunityRating         float64           `json:"CommunityRating,omitempty"`
	DateCreated             string            `json:"DateCreated,omitempty"`
	MediaType               string            `json:"MediaType,omitempty"`
	ProviderIds             map[string]string `json:"ProviderIds,omitempty"`
	IsFolder                bool              `json:"IsFolder,omitempty"`
	ParentId                string            `json:"ParentId,omitempty"`
	GenreItems              []Genre           `json:"GenreItems,omitempty"`
	ImageTags               map[string]string `json:"ImageTags,omitempty"`
	BackdropImageTags       []string          `json:"BackdropImageTags,omitempty"`
}

// UserData is the user-specific state of an item.
type UserData struct {
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	PlayCount             int    `json:"PlayCount"`
	IsFavorite            bool   `json:"IsFavorite"`
	Played                bool   `json:"Played"`
	Key                   string `json:"Key,omitempty"`
}

// Genre is a genre of an item.
type Genre struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

type watchlistResponse struct {
	Items []WatchlistItem `json:"Items"`
}

type user struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
}

// GetWatchlist retrieves the favorite movies and series of a user.
func (p *Provider) GetWatchlist(ctx context.Context, serverURL, apiKey, username string) ([]WatchlistItem, error) {
	if serverURL == "" || apiKey == "" || username == "" {
		return nil, errMissingServer
	}

	serverURL = strings.TrimSuffix(serverURL, "/")

	userID, err := p.userID(ctx, serverURL, apiKey, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get Jellyfin user ID: %w", err)
	}

	params := url.Values{}
	params.Set("api_key", apiKey)
	params.Set("IsFavorite", "true")
	params.Set("IncludeItemTypes", "Movie,Series")
	params.Set("Recursive", "true")
	params.Set("Fields", "ProviderIds,Overview,Genres,CommunityRating,DateCreated")
	params.Set("limit", "1000")

	var response watchlistResponse

	err = p.MakeRequest(
		ctx,
		http.MethodGet,
		serverURL+"/Users/"+url.PathEscape(userID)+"/Items?"+params.Encode(),
		nil,
		&response,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return response.Items, nil
}

func (p *Provider) userID(ctx context.Context, serverURL, apiKey, username string) (string, error) {
	var users []user

	err := p.MakeRequest(
		ctx,
		http.MethodGet,
		serverURL+"/Users?api_key="+url.QueryEscape(apiKey),
		nil,
		&users,
		nil,
	)
	if err != nil {
		return "", err
	}

	for _, u := range users {
		if strings.EqualFold(u.Name, username) {
			return u.Id, nil
		}
	}

	return "", fmt.Errorf("user '%s' not found on Jellyfin server", username)
}

func (item *WatchlistItem) providerInt(key string) int {
	n, _ := strconv.Atoi(item.ProviderIds[key])
	return n
}

// IMDbID returns the item's IMDB ID or "".
func (item *WatchlistItem) IMDbID() string { return item.ProviderIds["Imdb"] }

// TVDbID returns the item's TVDB ID or 0.
func (item *WatchlistItem) TVDbID() int { return item.providerInt("Tvdb") }

// TMDbID returns the item's TMDB ID or 0.
func (item *WatchlistItem) TMDbID() int { return item.providerInt("Tmdb") }

// IsMovie reports whether the item is a movie.
func (item *WatchlistItem) IsMovie() bool { return item.Type == "Movie" }

// IsSeries reports whether the item is a TV series.
func (item *WatchlistItem) IsSeries() bool { return item.Type == "Series" }

// Title returns the original title, or the name when there is none.
func (item *WatchlistItem) Title() string {
	if item.OriginalTitle != "" {
		return item.OriginalTitle
	}

	return item.Name
}
