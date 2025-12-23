package trakt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Trakt Provider - Social tracking and discovery
// Fully typed implementation with BaseClient infrastructure
//

const (
	// tokenFileName is the name of the file where the Trakt OAuth token is stored.
	tokenFileName = "trakt_token.json"
)

// Provider implements both MetadataProvider and OAuthProvider interfaces for Trakt.
type Provider struct {
	*base.BaseClient
	clientID     string
	clientSecret string
	redirectURI  string
	token        *apiexternal_v2.OAuthToken
	tokenMutex   sync.RWMutex
}

// NewProviderWithConfig creates a new Trakt provider with custom config.
func NewProviderWithConfig(
	config base.ClientConfig,
	clientID, clientSecret, redirectURI string,
) *Provider {
	config.Name = "trakt"
	if config.BaseURL == "" {
		config.BaseURL = "https://api.trakt.tv"
	}

	p := &Provider{
		BaseClient:   base.NewBaseClient(config),
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
	}

	// Load token from disk on initialization
	p.loadTokenFromDisk()

	return p
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderTrakt
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "trakt"
}

//
// Request Method Override for Auto Token Refresh
//

// MakeRequest overrides the BaseClient MakeRequest to automatically refresh tokens before requests.
func (p *Provider) MakeRequest(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
	target any,
) error {
	// For authenticated requests, ensure token is valid before making the request
	// This will automatically refresh the token if it's expired or about to expire
	if err := p.EnsureValidToken(ctx); err != nil {
		// Fail silently without logging - authentication errors are expected when not authenticated
		return err
	}

	// Add Trakt-specific headers
	return p.makeRequestWithHeaders(ctx, method, endpoint, body, target)
}

// makeRequestWithHeaders makes a request with Trakt-specific headers.
func (p *Provider) makeRequestWithHeaders(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
	target any,
) error {
	// Build custom headers for Trakt API
	headers := map[string]string{
		"trakt-api-version": "2",
		"trakt-api-key":     p.clientID,
		"Content-Type":      "application/json",
		"User-Agent":        "go-media-downloader/2.0",
	}

	// Add authorization header if we have a token
	token := p.GetCurrentToken()
	if token != nil && token.AccessToken != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", token.AccessToken)
	}

	// Use BaseClient's MakeRequestWithHeaders for full infrastructure support
	return p.MakeRequestWithHeaders(
		ctx,
		method,
		endpoint,
		body,
		target,
		nil,
		headers,
	)
}

//
// Movie Methods
//

// SearchMovies searches for movies by title and optional year.
func (p *Provider) SearchMovies(
	ctx context.Context,
	query string,
	year int,
) ([]apiexternal_v2.MovieSearchResult, error) {
	endpoint := fmt.Sprintf("/search/movie?query=%s", query)
	if year > 0 {
		endpoint += fmt.Sprintf("&years=%d", year)
	}

	var response traktSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertSearchResults(response, "trakt"), nil
}

// GetMovieByID retrieves detailed movie information by Trakt ID.
func (p *Provider) GetMovieByID(ctx context.Context, id int) (*apiexternal_v2.MovieDetails, error) {
	endpoint := fmt.Sprintf("/movies/%d?extended=full", id)

	var response traktMovieResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertMovieToDetails(&response), nil
}

// FindMovieByIMDbID finds movies by IMDb ID.
func (p *Provider) FindMovieByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	endpoint := fmt.Sprintf("/search/imdb/%s?type=movie", imdbID)

	var response traktSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	result := &apiexternal_v2.FindByIMDbResult{
		MovieResults: convertSearchResults(response, "trakt"),
	}

	return result, nil
}

// GetMovieExternalIDs retrieves external IDs for a movie.
func (p *Provider) GetMovieExternalIDs(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ExternalIDs, error) {
	endpoint := fmt.Sprintf("/movies/%d?extended=full", id)

	var response traktMovieResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertIDsToExternalIDs(response.IDs), nil
}

// GetMovieAlternativeTitles retrieves alternative titles (aliases) for a movie.
func (p *Provider) GetMovieAlternativeTitles(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.AlternativeTitle, error) {
	endpoint := fmt.Sprintf("/movies/%d/aliases", id)

	var response []traktAlias
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	// Convert Trakt aliases to AlternativeTitle format
	results := make([]apiexternal_v2.AlternativeTitle, len(response))
	for i, alias := range response {
		results[i] = apiexternal_v2.AlternativeTitle{
			Title:     alias.Title,
			ISO3166_1: alias.Country,
			Type:      "",
		}
	}

	return results, nil
}

// GetSeriesAlternativeTitles retrieves alternative titles (aliases) for a TV series.
func (p *Provider) GetSeriesAlternativeTitles(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.AlternativeTitle, error) {
	endpoint := fmt.Sprintf("/shows/%d/aliases", id)

	var response []traktAlias
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	// Convert Trakt aliases to AlternativeTitle format
	results := make([]apiexternal_v2.AlternativeTitle, len(response))
	for i, alias := range response {
		results[i] = apiexternal_v2.AlternativeTitle{
			Title:     alias.Title,
			ISO3166_1: alias.Country,
			Type:      "",
		}
	}

	return results, nil
}

//
// TV Series Methods
//

// SearchSeries searches for TV series by title and optional year.
func (p *Provider) SearchSeries(
	ctx context.Context,
	query string,
	year int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	endpoint := fmt.Sprintf("/search/show?query=%s", query)
	if year > 0 {
		endpoint += fmt.Sprintf("&years=%d", year)
	}

	var response traktSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertSearchToSeriesResults(response, "trakt"), nil
}

// GetSeriesByID retrieves detailed series information by Trakt ID.
func (p *Provider) GetSeriesByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.SeriesDetails, error) {
	endpoint := fmt.Sprintf("/shows/%d?extended=full", id)

	var response traktShowResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertShowToDetails(&response), nil
}

// FindSeriesByIMDbID finds TV series by IMDb ID.
func (p *Provider) FindSeriesByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	endpoint := fmt.Sprintf("/search/imdb/%s?type=show", imdbID)

	var response traktSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	result := &apiexternal_v2.FindByIMDbResult{
		TVResults: convertSearchToSeriesResults(response, "trakt"),
	}

	return result, nil
}

// FindSeriesByTVDbID finds TV series by TVDb ID.
func (p *Provider) FindSeriesByTVDbID(
	ctx context.Context,
	tvdbID int,
) (*apiexternal_v2.SeriesDetails, error) {
	endpoint := fmt.Sprintf("/search/tvdb/%d?type=show", tvdbID)

	var response traktSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	if len(response) == 0 || response[0].Show == nil {
		return nil, fmt.Errorf("show not found")
	}

	// Get full details using Trakt ID
	return p.GetSeriesByID(ctx, response[0].Show.IDs.Trakt)
}

// FindByTraktID finds movies or series by Trakt ID
//
// This method searches for both movies and TV series using the Trakt ID.
// Returns a FindByTraktIDResult containing both movie and series results.
func (p *Provider) FindByTraktID(
	ctx context.Context,
	traktID int,
) (*apiexternal_v2.FindByTraktIDResult, error) {
	result := &apiexternal_v2.FindByTraktIDResult{}

	// Try to get as a movie first
	movieDetails, movieErr := p.GetMovieByID(ctx, traktID)
	if movieErr == nil && movieDetails != nil {
		result.MovieResult = &apiexternal_v2.MovieSearchResult{
			ID:           movieDetails.ID,
			Title:        movieDetails.Title,
			Year:         movieDetails.Year,
			ReleaseDate:  movieDetails.ReleaseDate,
			Overview:     movieDetails.Overview,
			VoteAverage:  movieDetails.VoteAverage,
			ProviderName: "trakt",
		}
	}

	// Try to get as a series
	seriesDetails, seriesErr := p.GetSeriesByID(ctx, traktID)
	if seriesErr == nil && seriesDetails != nil {
		result.SeriesResult = &apiexternal_v2.SeriesSearchResult{
			ID:           seriesDetails.ID,
			Name:         seriesDetails.Name,
			FirstAirDate: seriesDetails.FirstAirDate,
			Overview:     seriesDetails.Overview,
			VoteAverage:  seriesDetails.VoteAverage,
			ProviderName: "trakt",
		}
	}

	// If both failed, return error
	if result.MovieResult == nil && result.SeriesResult == nil {
		return nil, fmt.Errorf(
			"content not found with Trakt ID %d: movie error: %w, series error: %w",
			traktID,
			movieErr,
			seriesErr,
		)
	}

	return result, nil
}

// GetSeriesExternalIDs retrieves external IDs for a series.
func (p *Provider) GetSeriesExternalIDs(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ExternalIDs, error) {
	endpoint := fmt.Sprintf("/shows/%d?extended=full", id)

	var response traktShowResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertIDsToExternalIDs(response.IDs), nil
}

//
// Episode Methods
//

// GetSeasonDetails retrieves detailed information about a season.
func (p *Provider) GetSeasonDetails(
	ctx context.Context,
	seriesID int,
	seasonNumber int,
) (*apiexternal_v2.Season, error) {
	endpoint := fmt.Sprintf("/shows/%d/seasons/%d?extended=full", seriesID, seasonNumber)

	var response traktSeasonResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertSeasonToDetails(&response), nil
}

// GetAllSeasons retrieves all seasons for a series.
func (p *Provider) GetAllSeasons(
	ctx context.Context,
	seriesID int,
) ([]*apiexternal_v2.Season, error) {
	endpoint := fmt.Sprintf("/shows/%d/seasons?extended=full", seriesID)

	var response []traktSeasonResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	seasons := make([]*apiexternal_v2.Season, 0, len(response))
	for i := range response {
		seasons = append(seasons, convertSeasonToDetails(&response[i]))
	}

	return seasons, nil
}

// GetEpisodeDetails retrieves detailed information about an episode.
func (p *Provider) GetEpisodeDetails(
	ctx context.Context,
	seriesID int,
	seasonNumber int,
	episodeNumber int,
) (*apiexternal_v2.Episode, error) {
	endpoint := fmt.Sprintf(
		"/shows/%d/seasons/%d/episodes/%d?extended=full",
		seriesID,
		seasonNumber,
		episodeNumber,
	)

	var response traktEpisode
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertEpisodeToDetails(&response), nil
}

// GetSeasonEpisodes retrieves all episodes for a specific season.
func (p *Provider) GetSeasonEpisodes(
	ctx context.Context,
	seriesID int,
	seasonNumber int,
) ([]*apiexternal_v2.Episode, error) {
	endpoint := fmt.Sprintf("/shows/%d/seasons/%d?extended=full,episodes", seriesID, seasonNumber)

	var response []traktEpisode
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	episodes := make([]*apiexternal_v2.Episode, 0, len(response))
	for i := range response {
		episodes = append(episodes, convertEpisodeToDetails(&response[i]))
	}

	return episodes, nil
}

//
// Popular/Trending Methods
//

// GetPopularMovies retrieves popular movies.
func (p *Provider) GetPopularMovies(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularMoviesResponse, error) {
	endpoint := fmt.Sprintf("/movies/popular?page=%d&limit=20&extended=full", page)

	var response traktPopularResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertPopularMovies(response, page), nil
}

// GetPopularSeries retrieves popular TV series.
func (p *Provider) GetPopularSeries(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularSeriesResponse, error) {
	endpoint := fmt.Sprintf("/shows/popular?page=%d&limit=20&extended=full", page)

	var response traktPopularResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertPopularSeries(response, page), nil
}

// GetTrendingMovies retrieves trending movies.
func (p *Provider) GetTrendingMovies(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularMoviesResponse, error) {
	endpoint := fmt.Sprintf("/movies/trending?page=%d&limit=20&extended=full", page)

	var response traktPopularResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertPopularMovies(response, page), nil
}

// GetTrendingSeries retrieves trending TV series.
func (p *Provider) GetTrendingSeries(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularSeriesResponse, error) {
	endpoint := fmt.Sprintf("/shows/trending?page=%d&limit=20&extended=full", page)

	var response traktPopularResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertPopularSeries(response, page), nil
}

// GetUpcomingMovies retrieves upcoming movies.
func (p *Provider) GetUpcomingMovies(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularMoviesResponse, error) {
	endpoint := fmt.Sprintf("/movies/anticipated?page=%d&limit=20&extended=full", page)

	var response traktPopularResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertPopularMovies(response, page), nil
}

//
// Credits
//

// GetMovieCredits retrieves cast and crew information for a movie.
func (p *Provider) GetMovieCredits(ctx context.Context, id int) (*apiexternal_v2.Credits, error) {
	endpoint := fmt.Sprintf("/movies/%d/people", id)

	var response traktCreditsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertCredits(&response), nil
}

// GetSeriesCredits retrieves cast and crew information for a series.
func (p *Provider) GetSeriesCredits(ctx context.Context, id int) (*apiexternal_v2.Credits, error) {
	endpoint := fmt.Sprintf("/shows/%d/people", id)

	var response traktCreditsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertCredits(&response), nil
}

//
// Media Resources (Not supported by Trakt)
//

// GetMovieImages - Trakt doesn't provide images.
func (p *Provider) GetMovieImages(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ImageCollection, error) {
	return nil, fmt.Errorf("trakt does not provide image listings")
}

// GetSeriesImages - Trakt doesn't provide images.
func (p *Provider) GetSeriesImages(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ImageCollection, error) {
	return nil, fmt.Errorf("trakt does not provide image listings")
}

// GetMovieVideos - Trakt doesn't provide videos.
func (p *Provider) GetMovieVideos(ctx context.Context, id int) ([]apiexternal_v2.Video, error) {
	return nil, fmt.Errorf("trakt does not provide video listings")
}

// GetSeriesVideos - Trakt doesn't provide videos.
func (p *Provider) GetSeriesVideos(ctx context.Context, id int) ([]apiexternal_v2.Video, error) {
	return nil, fmt.Errorf("trakt does not provide video listings")
}

//
// Recommendations
//

// GetSimilarMovies retrieves similar movies.
func (p *Provider) GetSimilarMovies(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.MovieSearchResult, error) {
	endpoint := fmt.Sprintf("/movies/%d/related?extended=full", id)

	var response []TraktMovie
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	results := make([]apiexternal_v2.MovieSearchResult, len(response))
	for i, m := range response {
		results[i] = apiexternal_v2.MovieSearchResult{
			ID:           m.IDs.Trakt,
			Title:        m.Title,
			Year:         m.Year,
			ReleaseDate:  parseTraktDate(m.Released),
			Overview:     m.Overview,
			VoteAverage:  m.Rating,
			ProviderName: "trakt",
		}
	}

	return results, nil
}

// GetSimilarSeries retrieves similar TV series.
func (p *Provider) GetSimilarSeries(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	endpoint := fmt.Sprintf("/shows/%d/related?extended=full", id)

	var response []TraktShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	results := make([]apiexternal_v2.SeriesSearchResult, len(response))
	for i, s := range response {
		results[i] = apiexternal_v2.SeriesSearchResult{
			ID:           s.IDs.Trakt,
			Name:         s.Title,
			FirstAirDate: parseTraktDate(s.FirstAired),
			Overview:     s.Overview,
			VoteAverage:  s.Rating,
			ProviderName: "trakt",
		}
	}

	return results, nil
}

// GetMovieRecommendations retrieves movie recommendations.
func (p *Provider) GetMovieRecommendations(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.MovieSearchResult, error) {
	endpoint := fmt.Sprintf("/movies/%d/recommendations?extended=full", id)

	var response []TraktMovie
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	results := make([]apiexternal_v2.MovieSearchResult, len(response))
	for i, m := range response {
		results[i] = apiexternal_v2.MovieSearchResult{
			ID:           m.IDs.Trakt,
			Title:        m.Title,
			Year:         m.Year,
			ReleaseDate:  parseTraktDate(m.Released),
			Overview:     m.Overview,
			VoteAverage:  m.Rating,
			ProviderName: "trakt",
		}
	}

	return results, nil
}

// GetSeriesRecommendations retrieves TV series recommendations.
func (p *Provider) GetSeriesRecommendations(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	endpoint := fmt.Sprintf("/shows/%d/recommendations?extended=full", id)

	var response []TraktShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	results := make([]apiexternal_v2.SeriesSearchResult, len(response))
	for i, s := range response {
		results[i] = apiexternal_v2.SeriesSearchResult{
			ID:           s.IDs.Trakt,
			Name:         s.Title,
			FirstAirDate: parseTraktDate(s.FirstAired),
			Overview:     s.Overview,
			VoteAverage:  s.Rating,
			ProviderName: "trakt",
		}
	}

	return results, nil
}

//
// Trakt-specific methods (not in base MetadataProvider interface)
// These can be accessed via type assertion
//

// GetWatchlist retrieves user's watchlist (requires OAuth).
func (p *Provider) GetWatchlist(
	ctx context.Context,
	userID string,
	mediaType string,
) ([]apiexternal_v2.MovieSearchResult, error) {
	endpoint := fmt.Sprintf("/users/%s/watchlist/%s?extended=full", userID, mediaType)

	var response traktPopularResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return convertPopularMovies(response, 1).Results, nil
}

// GetUserLists retrieves user's lists (requires OAuth).
func (p *Provider) GetUserLists(
	ctx context.Context,
	userID string,
) ([]apiexternal_v2.MovieSearchResult, error) {
	endpoint := fmt.Sprintf("/users/%s/lists", userID)

	var response []map[string]any
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	// Lists would require additional processing
	return nil, fmt.Errorf("list processing not implemented")
}

// GetTraktUserList retrieves items from a specific Trakt user list
// This is a provider-specific method not in the base interface.
func (p *Provider) GetTraktUserList(
	ctx context.Context,
	username, listname, listtype string,
) ([]TraktUserListItem, error) {
	endpoint := fmt.Sprintf("/users/%s/lists/%s/items/%s", username, listname, listtype)

	var response []TraktUserListItem
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// GetTraktSerieAnticipated retrieves anticipated TV series from Trakt
// This is a provider-specific method not in the base interface.
func (p *Provider) GetTraktSerieAnticipated(
	ctx context.Context,
	page int,
) ([]traktAnticipatedItem, error) {
	endpoint := fmt.Sprintf("/shows/anticipated?page=%d&limit=20&extended=full", page)

	var response []traktAnticipatedItem
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// RemoveMovieFromTraktUserList removes a movie from a Trakt user list
// This is a provider-specific method not in the base interface.
func (p *Provider) RemoveMovieFromTraktUserList(
	ctx context.Context,
	username, listname, imdbID string,
) error {
	endpoint := fmt.Sprintf("/users/%s/lists/%s/items/remove", username, listname)

	requestBody := map[string]any{
		"movies": []map[string]any{
			{
				"ids": map[string]any{
					"imdb": imdbID,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	var response map[string]any
	if err := p.MakeRequest(ctx, "POST", endpoint, bytes.NewReader(bodyBytes), &response); err != nil {
		return err
	}

	return nil
}

// RemoveSerieFromTraktUserList removes a TV series from a Trakt user list
// This is a provider-specific method not in the base interface.
func (p *Provider) RemoveSerieFromTraktUserList(
	ctx context.Context,
	username, listname string,
	tvdbID int,
) error {
	endpoint := fmt.Sprintf("/users/%s/lists/%s/items/remove", username, listname)

	requestBody := map[string]any{
		"shows": []map[string]any{
			{
				"ids": map[string]any{
					"tvdb": tvdbID,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	var response map[string]any
	if err := p.MakeRequest(ctx, "POST", endpoint, bytes.NewReader(bodyBytes), &response); err != nil {
		return err
	}

	return nil
}

//
// Token Persistence Methods
// These methods handle loading and saving tokens from/to disk
//

// getTokenFilePath returns the full path to the token file.
func (p *Provider) getTokenFilePath() string {
	// Store token file in the config directory (same directory as config.toml)
	configDir := filepath.Dir(config.Configfile)
	return filepath.Join(configDir, tokenFileName)
}

// loadTokenFromDisk loads the OAuth token from disk on startup.
func (p *Provider) loadTokenFromDisk() {
	tokenPath := p.getTokenFilePath()

	// Check if token file exists
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		logger.Logtype(logger.StatusDebug, 1).
			Str("path", tokenPath).
			Msg("No existing Trakt token file found")
		return
	}

	// Read token file
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		logger.Logtype(logger.StatusWarning, 0).
			Err(err).
			Str("path", tokenPath).
			Msg("Failed to read Trakt token file")

		return
	}

	// Parse token
	var token apiexternal_v2.OAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		logger.Logtype(logger.StatusWarning, 0).
			Err(err).
			Str("path", tokenPath).
			Msg("Failed to parse Trakt token file")

		return
	}

	// Validate token has required fields
	if token.AccessToken == "" {
		logger.Logtype(logger.StatusWarning, 0).
			Str("path", tokenPath).
			Msg("Trakt token file contains empty access token")
		return
	}

	// Store token in memory
	p.tokenMutex.Lock()

	p.token = &token
	p.tokenMutex.Unlock()

	if token.IsValid() {
		logger.Logtype(logger.StatusInfo, 0).
			Time("expiry", token.Expiry).
			Str("path", tokenPath).
			Msg("Loaded valid Trakt token from disk")
	} else {
		logger.Logtype(logger.StatusWarning, 0).
			Time("expiry", token.Expiry).
			Str("path", tokenPath).
			Msg("Loaded expired Trakt token from disk - will refresh on first use")
	}
}

// saveTokenToDisk saves the OAuth token to disk for persistence.
func (p *Provider) saveTokenToDisk(token *apiexternal_v2.OAuthToken) error {
	if token == nil {
		return fmt.Errorf("cannot save nil token")
	}

	tokenPath := p.getTokenFilePath()

	// Ensure config directory exists
	configDir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		logger.Logtype(logger.StatusError, 0).
			Err(err).
			Str("dir", configDir).
			Msg("Failed to create config directory for Trakt token")

		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal token to JSON with indentation for readability
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		logger.Logtype(logger.StatusError, 0).
			Err(err).
			Msg("Failed to marshal Trakt token to JSON")
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Write token to file with restricted permissions (0600 = owner read/write only)
	if err := os.WriteFile(tokenPath, data, 0o600); err != nil {
		logger.Logtype(logger.StatusError, 0).
			Err(err).
			Str("path", tokenPath).
			Msg("Failed to write Trakt token to disk")

		return fmt.Errorf("failed to write token file: %w", err)
	}

	logger.Logtype(logger.StatusDebug, 1).
		Time("expiry", token.Expiry).
		Str("path", tokenPath).
		Msg("Saved Trakt token to disk")

	return nil
}

//
// OAuth Provider Interface Implementation
// These methods implement the OAuthProvider interface
//

// GetAuthorizationURL returns the URL to initiate OAuth authorization
//
// The state parameter should be a random string to prevent CSRF attacks.
// Store this state and verify it matches when the callback is received.
func (p *Provider) GetAuthorizationURL(state string) string {
	params := make(map[string]string)

	params["response_type"] = "code"
	params["client_id"] = p.clientID

	params["redirect_uri"] = p.redirectURI
	if state != "" {
		params["state"] = state
	}

	queryString := ""
	for key, val := range params {
		if queryString != "" {
			queryString += "&"
		}

		queryString += fmt.Sprintf("%s=%s", key, val)
	}

	return fmt.Sprintf("https://trakt.tv/oauth/authorize?%s", queryString)
}

// ExchangeCodeForToken exchanges an authorization code for an access token.
func (p *Provider) ExchangeCodeForToken(
	ctx context.Context,
	code string,
) (*apiexternal_v2.OAuthToken, error) {
	requestBody := map[string]string{
		"code":          code,
		"client_id":     p.clientID,
		"client_secret": p.clientSecret,
		"redirect_uri":  p.redirectURI,
		"grant_type":    "authorization_code",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		CreatedAt    int64  `json:"created_at"`
	}

	// Use makeRequestWithHeaders directly to avoid token validation during token exchange
	if err := p.makeRequestWithHeaders(ctx, "POST", "/oauth/token", bytes.NewReader(bodyBytes), &tokenResp); err != nil {
		return nil, err
	}

	// Calculate expiry time
	var expiry time.Time
	if tokenResp.ExpiresIn > 0 {
		expiry = time.Unix(tokenResp.CreatedAt, 0).
			Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	token := &apiexternal_v2.OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       expiry,
		Scope:        tokenResp.Scope,
	}

	// Store the token in memory
	if err := p.SetToken(token); err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	// Save token to disk for persistence
	if err := p.saveTokenToDisk(token); err != nil {
		logger.Logtype(logger.StatusWarning, 0).
			Err(err).
			Msg("Failed to save Trakt token to disk after exchange")
		// Don't fail the exchange if save fails - token is still valid in memory
	}

	logger.Logtype(logger.StatusInfo, 0).
		Time("expiry", token.Expiry).
		Msg("Trakt token exchanged and saved successfully")

	return token, nil
}

// RefreshToken refreshes an expired access token using the refresh token.
func (p *Provider) RefreshToken(
	ctx context.Context,
	refreshToken string,
) (*apiexternal_v2.OAuthToken, error) {
	requestBody := map[string]string{
		"refresh_token": refreshToken,
		"client_id":     p.clientID,
		"client_secret": p.clientSecret,
		"redirect_uri":  p.redirectURI,
		"grant_type":    "refresh_token",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		CreatedAt    int64  `json:"created_at"`
	}

	// Use makeRequestWithHeaders directly to avoid token validation during token refresh
	if err := p.makeRequestWithHeaders(ctx, "POST", "/oauth/token", bytes.NewReader(bodyBytes), &tokenResp); err != nil {
		return nil, err
	}

	// Calculate expiry time
	var expiry time.Time
	if tokenResp.ExpiresIn > 0 {
		expiry = time.Unix(tokenResp.CreatedAt, 0).
			Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	token := &apiexternal_v2.OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       expiry,
		Scope:        tokenResp.Scope,
	}

	// Store the new token in memory
	if err := p.SetToken(token); err != nil {
		return nil, fmt.Errorf("failed to store refreshed token: %w", err)
	}

	// Save token to disk for persistence
	if err := p.saveTokenToDisk(token); err != nil {
		logger.Logtype(logger.StatusWarning, 0).
			Err(err).
			Msg("Failed to save refreshed Trakt token to disk")
		// Don't fail the refresh if save fails - token is still valid in memory
	}

	logger.Logtype(logger.StatusInfo, 0).
		Time("expiry", token.Expiry).
		Msg("Trakt token refreshed and saved successfully")

	return token, nil
}

// RevokeToken revokes an access token.
func (p *Provider) RevokeToken(ctx context.Context, token string) error {
	// Trakt doesn't have a dedicated revoke endpoint in their public API
	// Clear the token locally
	p.tokenMutex.Lock()

	p.token = nil
	p.tokenMutex.Unlock()

	// Remove token file from disk
	tokenPath := p.getTokenFilePath()
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		logger.Logtype(logger.StatusWarning, 0).
			Err(err).
			Str("path", tokenPath).
			Msg("Failed to remove Trakt token file")
		// Don't fail the revoke if file removal fails
	} else {
		logger.Logtype(logger.StatusDebug, 1).
			Str("path", tokenPath).
			Msg("Removed Trakt token file")
	}

	logger.Logtype(logger.StatusInfo, 0).Msg("Trakt token revoked (cleared locally)")

	return nil
}

// GetCurrentToken returns the current OAuth token.
func (p *Provider) GetCurrentToken() *apiexternal_v2.OAuthToken {
	p.tokenMutex.RLock()
	defer p.tokenMutex.RUnlock()
	return p.token
}

// SetToken sets the OAuth token.
func (p *Provider) SetToken(token *apiexternal_v2.OAuthToken) error {
	p.tokenMutex.Lock()
	defer p.tokenMutex.Unlock()

	p.token = token

	return nil
}

// IsAuthenticated checks if the provider has a valid token.
func (p *Provider) IsAuthenticated() bool {
	token := p.GetCurrentToken()
	return token != nil && token.IsValid()
}

// EnsureValidToken checks if the current token is valid and refreshes if needed
//
// This is a helper method that should be called before making authenticated API requests.
func (p *Provider) EnsureValidToken(ctx context.Context) error {
	token := p.GetCurrentToken()
	if token == nil {
		return fmt.Errorf("no token available - please authenticate first")
	}

	if !token.IsValid() {
		if token.RefreshToken == "" {
			return fmt.Errorf("token expired and no refresh token available")
		}

		// Token is expired, try to refresh
		_, err := p.RefreshToken(ctx, token.RefreshToken)

		return err
	}

	// Check if token needs refresh soon
	if token.NeedsRefresh() && token.RefreshToken != "" {
		// Attempt to refresh proactively
		_, err := p.RefreshToken(ctx, token.RefreshToken)
		if err != nil {
			logger.Logtype(logger.StatusWarning, 0).
				Err(err).
				Msg("Failed to proactively refresh Trakt token")

			// Don't fail - current token is still valid
		}
	}

	return nil
}
