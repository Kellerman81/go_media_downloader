package plex

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// Plex Provider - Watchlist integration
//

// Provider implements the WatchlistProvider interface for Plex.
type Provider struct {
	*base.BaseClient
	serverURL          string
	token              string
	insecureSkipVerify bool
}

// NewProvider creates a new Plex watchlist provider.
func NewProvider(serverURL, token string, insecureSkipVerify bool) *Provider {
	config := base.ClientConfig{
		Name:                    "plex",
		BaseURL:                 serverURL,
		Timeout:                 30 * time.Second,
		AuthType:                base.AuthAPIKeyHeader,
		APIKey:                  token,
		APIKeyHeader:            "X-Plex-Token",
		RateLimitPer24h:         0, // No specific rate limit for Plex
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	baseClient := base.NewBaseClient(config)

	// Configure BaseClient's HTTP client with optional TLS skip verification
	if insecureSkipVerify {
		baseClient.GetHTTPClient().Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return &Provider{
		BaseClient:         baseClient,
		serverURL:          serverURL,
		token:              token,
		insecureSkipVerify: insecureSkipVerify,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return "plex"
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "plex"
}

// PlexWatchlistResponse represents the JSON response from Plex watchlist.
type PlexWatchlistResponse struct {
	MediaContainer PlexMediaContainer `json:"MediaContainer"`
}

// PlexMediaContainer contains the actual watchlist items.
type PlexMediaContainer struct {
	Size     int                 `json:"size"`
	Metadata []PlexWatchlistItem `json:"Metadata"`
}

// PlexWatchlistItem represents a single item in the watchlist.
type PlexWatchlistItem struct {
	Type  string     `json:"type"`
	Title string     `json:"title"`
	Year  int        `json:"year"`
	Guid  string     `json:"guid"`
	Guids []PlexGuid `json:"Guid"`
}

// PlexGuid represents a GUID element in Plex.
type PlexGuid struct {
	ID string `json:"id"`
}

// GetWatchlist retrieves the watchlist for a Plex user.
func (p *Provider) GetWatchlist(
	ctx context.Context,
	username string,
) ([]apiexternal_v2.WatchlistItem, error) {
	// Build the watchlist URL with query parameters
	params := url.Values{}
	params.Set("X-Plex-Token", p.token)
	params.Set("type", "15") // Watchlist type
	params.Set("includeGuids", "1")

	endpoint := fmt.Sprintf("/playlists/all?%s", params.Encode())

	headers := map[string]string{
		"Accept":       "application/json",
		"X-Plex-Token": p.token,
	}

	var jsonResp PlexWatchlistResponse

	err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		nil,
		func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("plex API returned status %d", resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			if err := json.Unmarshal(body, &jsonResp); err != nil {
				return fmt.Errorf("failed to parse JSON: %w", err)
			}

			return nil
		},
		headers,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch watchlist: %w", err)
	}

	items := make([]apiexternal_v2.WatchlistItem, 0, len(jsonResp.MediaContainer.Metadata))
	for _, video := range jsonResp.MediaContainer.Metadata {
		item := apiexternal_v2.WatchlistItem{
			Type:         video.Type,
			Title:        video.Title,
			Year:         video.Year,
			ProviderName: "plex",
		}

		// Extract IMDb ID and TVDB ID from GUIDs
		item.IMDbID = extractIMDbIDFromGuids(video.Guids)
		item.TVDbID = extractTVDbIDFromGuids(video.Guids)

		items = append(items, item)
	}

	return items, nil
}

// extractIMDbIDFromGuids extracts IMDb ID from Plex GUIDs.
func extractIMDbIDFromGuids(guids []PlexGuid) string {
	imdbPattern := regexp.MustCompile(`imdb://([a-z0-9]+)`)
	for _, guid := range guids {
		if matches := imdbPattern.FindStringSubmatch(guid.ID); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractTVDbIDFromGuids extracts TVDB ID from Plex GUIDs.
func extractTVDbIDFromGuids(guids []PlexGuid) int {
	tvdbPattern := regexp.MustCompile(`tvdb://(\d+)`)
	for _, guid := range guids {
		if matches := tvdbPattern.FindStringSubmatch(guid.ID); len(matches) > 1 {
			if id, err := strconv.Atoi(matches[1]); err == nil {
				return id
			}
		}
	}

	return 0
}

// AddToWatchlist adds an item to the Plex watchlist.
func (p *Provider) AddToWatchlist(
	ctx context.Context,
	username string,
	itemType string,
	id int,
) error {
	return fmt.Errorf("add to watchlist not implemented for Plex")
}

// RemoveFromWatchlist removes an item from the Plex watchlist.
func (p *Provider) RemoveFromWatchlist(
	ctx context.Context,
	username string,
	itemType string,
	id int,
) error {
	return fmt.Errorf("remove from watchlist not implemented for Plex")
}
