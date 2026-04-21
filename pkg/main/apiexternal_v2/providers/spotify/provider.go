package spotify

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
)

//
// Spotify Provider - Music streaming service metadata
// API: https://developer.spotify.com/documentation/web-api
// Authentication: Client Credentials Flow
// Rate limit: Varies, typically several thousand requests per day
//

const (
	defaultBaseURL = "https://api.spotify.com/v1"
	authURL        = "https://accounts.spotify.com/api/token"
	defaultLimit   = 10
	maxLimit       = 50
	tokenCacheFile = "spotify_token_cache.json"
)

// Provider implements the music metadata provider for Spotify.
type Provider struct {
	*base.BaseClient
	clientID     string
	clientSecret string
	tokenMutex   sync.RWMutex
	accessToken  string
	tokenExpiry  time.Time
	region       string // ISO 3166-1 alpha-2 country code for market filtering
	tiebreak     string // "first" or "popularity" (default)
}

// NewProviderWithConfig creates a new Spotify provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig, clientID, clientSecret, region string) *Provider {
	cfg.Name = "spotify"
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Spotify has generous rate limits, no need for aggressive limiting
	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 10
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 1
	}

	if cfg.UserAgent == "" {
		cfg.UserAgent = config.GetSettingsGeneral().UserAgent
	}

	return &Provider{
		BaseClient:   base.NewBaseClient(cfg),
		clientID:     clientID,
		clientSecret: clientSecret,
		region:       region,
		tiebreak:     "popularity", // Default to most popular
	}
}

// NewProvider creates a new Spotify provider with client credentials.
func NewProvider(clientID, clientSecret string) *Provider {
	return NewProviderWithConfig(base.ClientConfig{}, clientID, clientSecret, "")
}

// SetRegion sets the market/region for filtering results.
func (p *Provider) SetRegion(region string) {
	p.region = region
}

// SetTiebreak sets the tiebreak strategy: "first" or "popularity".
func (p *Provider) SetTiebreak(tiebreak string) {
	if tiebreak == "first" || tiebreak == "popularity" {
		p.tiebreak = tiebreak
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderSpotify
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "spotify"
}

// authenticate obtains an access token using Client Credentials Flow.
func (p *Provider) authenticate(ctx context.Context) error {
	p.tokenMutex.Lock()
	defer p.tokenMutex.Unlock()

	// Check if token is still valid
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return nil
	}

	// Try to load cached token
	if p.loadCachedToken() {
		return nil
	}

	// Request new token
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	// Set Authorization header with Base64 encoded client credentials
	auth := base64.StdEncoding.EncodeToString([]byte(p.clientID + ":" + p.clientSecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth failed with status %d", resp.StatusCode)
	}

	var authResp spotifyAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	p.accessToken = authResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)

	// Cache the token
	p.saveCachedToken()

	return nil
}

// loadCachedToken attempts to load a cached token from disk.
func (p *Provider) loadCachedToken() bool {
	cacheDir := config.GetConfigDir()
	cachePath := filepath.Join(cacheDir, tokenCacheFile)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return false
	}

	var cache tokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return false
	}

	// Check if token is still valid (with 5 minute buffer)
	if time.Now().Add(5 * time.Minute).Before(cache.ExpiresAt) {
		p.accessToken = cache.AccessToken
		p.tokenExpiry = cache.ExpiresAt
		return true
	}

	return false
}

// saveCachedToken saves the current token to disk.
func (p *Provider) saveCachedToken() {
	cacheDir := config.GetConfigDir()
	cachePath := filepath.Join(cacheDir, tokenCacheFile)

	cache := tokenCache{
		AccessToken: p.accessToken,
		ExpiresAt:   p.tokenExpiry,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(cachePath, data, 0o600)
}

// makeAuthenticatedRequest makes an authenticated request to Spotify API.
func (p *Provider) makeAuthenticatedRequest(
	ctx context.Context,
	method, endpoint string,
	params url.Values,
	result any,
) error {
	// Ensure we have a valid token
	if err := p.authenticate(ctx); err != nil {
		return err
	}

	headers := map[string]string{
		"Authorization": "Bearer " + p.accessToken,
	}

	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	return p.MakeRequestWithHeaders(ctx, method, endpoint, nil, &result, nil, headers)
}

//
// Search Methods
//

// SearchAlbums searches for albums using Spotify's search API.
// Implements the beets plugin search pattern.
func (p *Provider) SearchAlbums(
	ctx context.Context,
	artist string,
	album string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}

	query := p.constructSearchQuery(artist, album, "")

	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "album")
	params.Set("limit", fmt.Sprintf("%d", limit))

	if p.region != "" {
		params.Set("market", p.region)
	}

	var response spotifySearchResponse
	if err := p.makeAuthenticatedRequest(ctx, "GET", "/search", params, &response); err != nil {
		return nil, err
	}

	if response.Albums == nil || len(response.Albums.Items) == 0 {
		return nil, fmt.Errorf("no albums found")
	}

	return p.filterAndConvertAlbums(response.Albums.Items), nil
}

// SearchTracks searches for tracks using Spotify's search API.
func (p *Provider) SearchTracks(
	ctx context.Context,
	artist string,
	track string,
	album string,
	limit int,
) ([]apiexternal_v2.TrackSearchResult, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}

	query := p.constructSearchQuery(artist, album, track)

	params := url.Values{}
	params.Set("q", query)
	params.Set("type", "track")
	params.Set("limit", fmt.Sprintf("%d", limit))

	if p.region != "" {
		params.Set("market", p.region)
	}

	var response spotifySearchResponse
	if err := p.makeAuthenticatedRequest(ctx, "GET", "/search", params, &response); err != nil {
		return nil, err
	}

	if response.Tracks == nil || len(response.Tracks.Items) == 0 {
		return nil, fmt.Errorf("no tracks found")
	}

	return p.filterAndConvertTracks(response.Tracks.Items), nil
}

// constructSearchQuery builds a search query string from components.
// Mimics beets plugin's _construct_search_query method.
func (p *Provider) constructSearchQuery(artist, album, track string) string {
	var parts []string

	if artist != "" {
		parts = append(parts, fmt.Sprintf("artist:%s", artist))
	}

	if album != "" {
		parts = append(parts, fmt.Sprintf("album:%s", album))
	}

	if track != "" {
		parts = append(parts, fmt.Sprintf("track:%s", track))
	}

	return logger.JoinStringsSep(parts, " ")
}

// filterAndConvertAlbums filters results by region and converts to standard format.
func (p *Provider) filterAndConvertAlbums(
	albums []spotifyAlbum,
) []apiexternal_v2.ReleaseSearchResult {
	var results []apiexternal_v2.ReleaseSearchResult

	// Apply tiebreak strategy
	var selectedAlbums []spotifyAlbum
	if p.tiebreak == "first" && len(albums) > 0 {
		selectedAlbums = []spotifyAlbum{albums[0]}
	} else if p.tiebreak == "popularity" {
		// Sort by popularity and take top results
		selectedAlbums = albums
	} else {
		selectedAlbums = albums
	}

	for _, album := range selectedAlbums {
		// Filter by region if specified
		if p.region != "" && !containsMarket(album.AvailableMarkets, p.region) {
			continue
		}

		results = append(results, convertAlbumToSearchResult(&album))
	}

	return results
}

// filterAndConvertTracks filters results by region and converts to standard format.
func (p *Provider) filterAndConvertTracks(
	tracks []spotifyTrack,
) []apiexternal_v2.TrackSearchResult {
	var results []apiexternal_v2.TrackSearchResult

	for _, track := range tracks {
		// Filter by region if specified
		if p.region != "" && !containsMarket(track.AvailableMarkets, p.region) {
			continue
		}

		results = append(results, convertTrackToSearchResult(&track))
	}

	return results
}

// containsMarket checks if a market is in the available markets list.
func containsMarket(markets []string, market string) bool {
	for _, m := range markets {
		if strings.EqualFold(m, market) {
			return true
		}
	}

	return false
}

//
// Detailed Lookup Methods
//

// GetAlbumByID retrieves full album details by Spotify ID.
func (p *Provider) GetAlbumByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.ReleaseDetails, error) {
	endpoint := fmt.Sprintf("/albums/%s", id)

	params := url.Values{}
	if p.region != "" {
		params.Set("market", p.region)
	}

	var album spotifyAlbum
	if err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, params, &album); err != nil {
		return nil, err
	}

	return convertAlbumToDetails(&album), nil
}

// GetTrackByID retrieves track details by Spotify ID.
func (p *Provider) GetTrackByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.TrackDetails, error) {
	endpoint := fmt.Sprintf("/tracks/%s", id)

	params := url.Values{}
	if p.region != "" {
		params.Set("market", p.region)
	}

	var track spotifyTrack
	if err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, params, &track); err != nil {
		return nil, err
	}

	return convertTrackToDetails(&track), nil
}

// GetArtistByID retrieves artist details by Spotify ID.
func (p *Provider) GetArtistByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.ArtistDetails, error) {
	endpoint := fmt.Sprintf("/artists/%s", id)

	var artist spotifyArtist
	if err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil, &artist); err != nil {
		return nil, err
	}

	return convertArtistToDetails(&artist), nil
}

// GetAudioFeatures retrieves audio features for a track.
// Note: This endpoint may return 403 as it's being deprecated.
func (p *Provider) GetAudioFeatures(ctx context.Context, id string) (*spotifyAudioFeatures, error) {
	endpoint := fmt.Sprintf("/audio-features/%s", id)

	var features spotifyAudioFeatures
	if err := p.makeAuthenticatedRequest(ctx, "GET", endpoint, nil, &features); err != nil {
		// Gracefully handle 403 (deprecated endpoint)
		if strings.Contains(err.Error(), "403") {
			return nil, nil
		}

		return nil, err
	}

	return &features, nil
}
