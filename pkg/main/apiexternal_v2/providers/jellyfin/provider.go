package jellyfin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Jellyfin Provider - Watchlist integration
//

// Provider implements the WatchlistProvider interface for Jellyfin.
type Provider struct {
	*base.BaseClient
	serverURL string
	token     string
	userID    string
}

// NewProvider creates a new Jellyfin watchlist provider.
func NewProvider(serverURL, token, userID string) *Provider {
	config := base.ClientConfig{
		Name:                    "jellyfin",
		BaseURL:                 serverURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthAPIKeyHeader,
		APIKey:                  token,
		APIKeyHeader:            "X-MediaBrowser-Token",
		RateLimitPer24h:         0, // No specific rate limit for Jellyfin
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		serverURL:  serverURL,
		token:      token,
		userID:     userID,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return "jellyfin"
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "jellyfin"
}

// JellyfinItem represents a Jellyfin library item.
type JellyfinItem struct {
	Name            string            `json:"Name"`
	Type            string            `json:"Type"`
	ProductionYear  int               `json:"ProductionYear"`
	Id              string            `json:"Id"`
	ProviderIds     map[string]string `json:"ProviderIds"`
	CommunityRating float64           `json:"CommunityRating"`
}

// JellyfinItemsResponse represents the response from Jellyfin Items endpoint.
type JellyfinItemsResponse struct {
	Items            []JellyfinItem `json:"Items"`
	TotalRecordCount int            `json:"TotalRecordCount"`
}

// GetWatchlist retrieves the watchlist for a Jellyfin user.
func (p *Provider) GetWatchlist(
	ctx context.Context,
	username string,
) ([]apiexternal_v2.WatchlistItem, error) {
	// Jellyfin uses Favorites as watchlist
	endpoint := fmt.Sprintf(
		"/Users/%s/Items?IsFavorite=true&IncludeItemTypes=Movie,Series&Recursive=true&Fields=ProviderIds&limit=1000",
		p.userID,
	)

	var response JellyfinItemsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	items := make([]apiexternal_v2.WatchlistItem, 0, len(response.Items))
	for _, item := range response.Items {
		itemType := "movie"
		if item.Type == "Series" {
			itemType = "tv"
		}

		watchlistItem := apiexternal_v2.WatchlistItem{
			Type:         itemType,
			Title:        item.Name,
			Year:         item.ProductionYear,
			ProviderName: "jellyfin",
		}

		// Extract IMDb and TVDB IDs from ProviderIds
		if imdbID, ok := item.ProviderIds["Imdb"]; ok {
			watchlistItem.IMDbID = imdbID
		}

		if tvdbIDStr, ok := item.ProviderIds["Tvdb"]; ok {
			if tvdbID, err := strconv.Atoi(tvdbIDStr); err == nil {
				watchlistItem.TVDbID = tvdbID
			}
		}

		items = append(items, watchlistItem)
	}

	return items, nil
}

// AddToWatchlist adds an item to favorites (Jellyfin's watchlist equivalent).
func (p *Provider) AddToWatchlist(
	ctx context.Context,
	username string,
	itemType string,
	id int,
) error {
	endpoint := fmt.Sprintf("/Users/%s/FavoriteItems/%d", p.userID, id)

	var response any
	if err := p.MakeRequest(ctx, "POST", endpoint, nil, &response, nil); err != nil {
		return fmt.Errorf("failed to add to favorites: %w", err)
	}

	return nil
}

// RemoveFromWatchlist removes an item from favorites.
func (p *Provider) RemoveFromWatchlist(
	ctx context.Context,
	username string,
	itemType string,
	id int,
) error {
	endpoint := fmt.Sprintf("/Users/%s/FavoriteItems/%d", p.userID, id)

	var response any
	if err := p.MakeRequest(ctx, "DELETE", endpoint, nil, &response, nil); err != nil {
		return fmt.Errorf("failed to remove from favorites: %w", err)
	}

	return nil
}
