package tmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// TMDB Provider - Fully Typed Implementation
// Zero interface{} or any in public methods
//

// Provider implements the MetadataProvider interface for TMDB.
type Provider struct {
	apiKey     string
	baseClient *base.BaseClient
	baseURL    string
}

// NewProviderWithConfig creates a new TMDB metadata provider with full configuration including
// rate limiting, timeout settings, circuit breaker, and TLS verification options.
func NewProviderWithConfig(config base.ClientConfig, apiKey string) *Provider {
	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// Set default base URL if not specified
	if config.BaseURL == "" {
		config.BaseURL = "https://api.themoviedb.org/3"
	}

	// Create base client with full infrastructure (rate limiting, circuit breaker, etc.)
	baseClient := base.NewBaseClient(config)

	return &Provider{
		apiKey:     apiKey,
		baseClient: baseClient,
		baseURL:    config.BaseURL,
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderTMDb
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "tmdb"
}

//
// Movie Methods - Fully Typed
//

// SearchMovies searches for movies by title and optional year.
func (p *Provider) SearchMovies(
	ctx context.Context,
	query string,
	year int,
) ([]apiexternal_v2.MovieSearchResult, error) {
	url := fmt.Sprintf("%s/search/movie?query=%s", p.baseURL, query)
	if year > 0 {
		url += fmt.Sprintf("&year=%d", year)
	}

	var response struct {
		Results []tmdbMovieSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertMovieSearchResults(response.Results), nil
}

// GetMovieByID retrieves detailed movie information by TMDB ID.
func (p *Provider) GetMovieByID(ctx context.Context, id int) (*apiexternal_v2.MovieDetails, error) {
	url := fmt.Sprintf("%s/movie/%d?append_to_response=alternative_titles,credits", p.baseURL, id)

	var response tmdbMovieDetails
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertMovieDetails(&response), nil
}

// FindMovieByIMDbID finds movies by IMDb ID.
func (p *Provider) FindMovieByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	url := fmt.Sprintf("%s/find/%s?external_source=imdb_id", p.baseURL, imdbID)

	var response struct {
		MovieResults []tmdbMovieSearchResult  `json:"movie_results"`
		TVResults    []tmdbSeriesSearchResult `json:"tv_results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.FindByIMDbResult{
		MovieResults: convertMovieSearchResults(response.MovieResults),
		TVResults:    convertSeriesSearchResults(response.TVResults),
	}, nil
}

// GetMovieExternalIDs retrieves external IDs for a movie.
func (p *Provider) GetMovieExternalIDs(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ExternalIDs, error) {
	url := fmt.Sprintf("%s/movie/%d/external_ids", p.baseURL, id)

	var response tmdbExternalIDs
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertExternalIDs(&response, id), nil
}

//
// TV Series Methods - Fully Typed
//

// SearchSeries searches for TV series by title and optional year.
func (p *Provider) SearchSeries(
	ctx context.Context,
	query string,
	year int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	url := fmt.Sprintf("%s/search/tv?query=%s", p.baseURL, query)
	if year > 0 {
		url += fmt.Sprintf("&first_air_date_year=%d", year)
	}

	var response struct {
		Results []tmdbSeriesSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertSeriesSearchResults(response.Results), nil
}

// GetSeriesByID retrieves detailed series information by TMDB ID.
func (p *Provider) GetSeriesByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.SeriesDetails, error) {
	url := fmt.Sprintf(
		"%s/tv/%d?append_to_response=alternative_titles,credits,external_ids",
		p.baseURL,
		id,
	)

	var response tmdbSeriesDetails
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertSeriesDetails(&response), nil
}

// FindSeriesByIMDbID finds TV series by IMDb ID.
func (p *Provider) FindSeriesByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	// Use the same find endpoint as movies
	return p.FindMovieByIMDbID(ctx, imdbID)
}

// FindSeriesByTVDbID finds TV series by TVDb ID.
func (p *Provider) FindSeriesByTVDbID(
	ctx context.Context,
	tvdbID int,
) (*apiexternal_v2.SeriesDetails, error) {
	url := fmt.Sprintf("%s/find/%d?external_source=tvdb_id", p.baseURL, tvdbID)

	var response struct {
		TVResults []tmdbSeriesSearchResult `json:"tv_results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	if len(response.TVResults) == 0 {
		return nil, fmt.Errorf("no series found with TVDb ID %d", tvdbID)
	}

	// Get full details for the first result
	return p.GetSeriesByID(ctx, response.TVResults[0].ID)
}

// GetSeriesExternalIDs retrieves external IDs for a TV series.
func (p *Provider) GetSeriesExternalIDs(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ExternalIDs, error) {
	url := fmt.Sprintf("%s/tv/%d/external_ids", p.baseURL, id)

	var response tmdbExternalIDs
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertExternalIDs(&response, id), nil
}

// TVExternalIDs represents external IDs for a TV show from TMDB.
type TVExternalIDs struct {
	ID          int    `json:"id"`
	IMDbID      string `json:"imdb_id"`
	TVDbID      int    `json:"tvdb_id"`
	FacebookID  string `json:"facebook_id,omitempty"`
	InstagramID string `json:"instagram_id,omitempty"`
	TwitterID   string `json:"twitter_id,omitempty"`
}

// GetTVExternal retrieves external IDs (IMDB, TVDB, etc.) for a TV show
// Maps to TMDB API v3 endpoint: /tv/{tv_id}/external_ids.
func (p *Provider) GetTVExternal(ctx context.Context, tvID int) (*TVExternalIDs, error) {
	url := fmt.Sprintf("%s/tv/%d/external_ids", p.baseURL, tvID)

	var response tmdbExternalIDs
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, fmt.Errorf("failed to get TV external IDs: %w", err)
	}

	return &TVExternalIDs{
		ID:          tvID,
		IMDbID:      response.IMDbID,
		TVDbID:      response.TVDbID,
		FacebookID:  response.FacebookID,
		InstagramID: response.InstagramID,
		TwitterID:   response.TwitterID,
	}, nil
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
	url := fmt.Sprintf("%s/tv/%d/season/%d", p.baseURL, seriesID, seasonNumber)

	var response tmdbSeason
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertSeason(&response), nil
}

// GetEpisodeDetails retrieves detailed information about an episode.
func (p *Provider) GetEpisodeDetails(
	ctx context.Context,
	seriesID int,
	seasonNumber int,
	episodeNumber int,
) (*apiexternal_v2.Episode, error) {
	url := fmt.Sprintf("%s/tv/%d/season/%d/episode/%d?append_to_response=credits",
		p.baseURL, seriesID, seasonNumber, episodeNumber)

	var response tmdbEpisode
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertEpisode(&response), nil
}

//
// Popular/Trending Methods
//

// GetPopularMovies retrieves popular movies.
func (p *Provider) GetPopularMovies(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularMoviesResponse, error) {
	url := fmt.Sprintf("%s/movie/popular?page=%d", p.baseURL, page)

	var response struct {
		Page         int                     `json:"page"`
		TotalPages   int                     `json:"total_pages"`
		TotalResults int                     `json:"total_results"`
		Results      []tmdbMovieSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.PopularMoviesResponse{
		Page:         response.Page,
		TotalPages:   response.TotalPages,
		TotalResults: response.TotalResults,
		Results:      convertMovieSearchResults(response.Results),
	}, nil
}

// GetPopularSeries retrieves popular TV series.
func (p *Provider) GetPopularSeries(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularSeriesResponse, error) {
	url := fmt.Sprintf("%s/tv/popular?page=%d", p.baseURL, page)

	var response struct {
		Page         int                      `json:"page"`
		TotalPages   int                      `json:"total_pages"`
		TotalResults int                      `json:"total_results"`
		Results      []tmdbSeriesSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.PopularSeriesResponse{
		Page:         response.Page,
		TotalPages:   response.TotalPages,
		TotalResults: response.TotalResults,
		Results:      convertSeriesSearchResults(response.Results),
	}, nil
}

// GetTrendingMovies retrieves trending movies.
func (p *Provider) GetTrendingMovies(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularMoviesResponse, error) {
	url := fmt.Sprintf("%s/trending/movie/week?page=%d", p.baseURL, page)

	var response struct {
		Page         int                     `json:"page"`
		TotalPages   int                     `json:"total_pages"`
		TotalResults int                     `json:"total_results"`
		Results      []tmdbMovieSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.PopularMoviesResponse{
		Page:         response.Page,
		TotalPages:   response.TotalPages,
		TotalResults: response.TotalResults,
		Results:      convertMovieSearchResults(response.Results),
	}, nil
}

// GetTrendingSeries retrieves trending TV series.
func (p *Provider) GetTrendingSeries(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularSeriesResponse, error) {
	url := fmt.Sprintf("%s/trending/tv/week?page=%d", p.baseURL, page)

	var response struct {
		Page         int                      `json:"page"`
		TotalPages   int                      `json:"total_pages"`
		TotalResults int                      `json:"total_results"`
		Results      []tmdbSeriesSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.PopularSeriesResponse{
		Page:         response.Page,
		TotalPages:   response.TotalPages,
		TotalResults: response.TotalResults,
		Results:      convertSeriesSearchResults(response.Results),
	}, nil
}

// GetUpcomingMovies retrieves upcoming movies.
func (p *Provider) GetUpcomingMovies(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularMoviesResponse, error) {
	url := fmt.Sprintf("%s/movie/upcoming?page=%d", p.baseURL, page)

	var response struct {
		Page         int                     `json:"page"`
		TotalPages   int                     `json:"total_pages"`
		TotalResults int                     `json:"total_results"`
		Results      []tmdbMovieSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.PopularMoviesResponse{
		Page:         response.Page,
		TotalPages:   response.TotalPages,
		TotalResults: response.TotalResults,
		Results:      convertMovieSearchResults(response.Results),
	}, nil
}

//
// Alternative Titles
//

// GetMovieAlternativeTitles retrieves alternative titles for a movie.
func (p *Provider) GetMovieAlternativeTitles(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.AlternativeTitle, error) {
	url := fmt.Sprintf("%s/movie/%d/alternative_titles", p.baseURL, id)

	var response struct {
		Titles []tmdbAlternativeTitle `json:"titles"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertAlternativeTitles(response.Titles), nil
}

// GetSeriesAlternativeTitles retrieves alternative titles for a TV series.
func (p *Provider) GetSeriesAlternativeTitles(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.AlternativeTitle, error) {
	url := fmt.Sprintf("%s/tv/%d/alternative_titles", p.baseURL, id)

	var response struct {
		Results []tmdbAlternativeTitle `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertAlternativeTitles(response.Results), nil
}

//
// Credits
//

// GetMovieCredits retrieves credits for a movie.
func (p *Provider) GetMovieCredits(ctx context.Context, id int) (*apiexternal_v2.Credits, error) {
	url := fmt.Sprintf("%s/movie/%d/credits", p.baseURL, id)

	var response tmdbCredits
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertCredits(&response), nil
}

// GetSeriesCredits retrieves credits for a TV series.
func (p *Provider) GetSeriesCredits(ctx context.Context, id int) (*apiexternal_v2.Credits, error) {
	url := fmt.Sprintf("%s/tv/%d/credits", p.baseURL, id)

	var response tmdbCredits
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertCredits(&response), nil
}

//
// Media Resources
//

// GetMovieImages retrieves images for a movie.
func (p *Provider) GetMovieImages(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ImageCollection, error) {
	url := fmt.Sprintf("%s/movie/%d/images", p.baseURL, id)

	var response tmdbImageCollection
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertImageCollection(&response), nil
}

// GetSeriesImages retrieves images for a TV series.
func (p *Provider) GetSeriesImages(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ImageCollection, error) {
	url := fmt.Sprintf("%s/tv/%d/images", p.baseURL, id)

	var response tmdbImageCollection
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertImageCollection(&response), nil
}

// GetMovieVideos retrieves videos for a movie.
func (p *Provider) GetMovieVideos(ctx context.Context, id int) ([]apiexternal_v2.Video, error) {
	url := fmt.Sprintf("%s/movie/%d/videos", p.baseURL, id)

	var response struct {
		Results []tmdbVideo `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertVideos(response.Results), nil
}

// GetSeriesVideos retrieves videos for a TV series.
func (p *Provider) GetSeriesVideos(ctx context.Context, id int) ([]apiexternal_v2.Video, error) {
	url := fmt.Sprintf("%s/tv/%d/videos", p.baseURL, id)

	var response struct {
		Results []tmdbVideo `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertVideos(response.Results), nil
}

//
// Recommendations and Similar
//

// GetSimilarMovies retrieves similar movies.
func (p *Provider) GetSimilarMovies(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.MovieSearchResult, error) {
	url := fmt.Sprintf("%s/movie/%d/similar", p.baseURL, id)

	var response struct {
		Results []tmdbMovieSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertMovieSearchResults(response.Results), nil
}

// GetSimilarSeries retrieves similar TV series.
func (p *Provider) GetSimilarSeries(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	url := fmt.Sprintf("%s/tv/%d/similar", p.baseURL, id)

	var response struct {
		Results []tmdbSeriesSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertSeriesSearchResults(response.Results), nil
}

// GetMovieRecommendations retrieves movie recommendations.
func (p *Provider) GetMovieRecommendations(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.MovieSearchResult, error) {
	url := fmt.Sprintf("%s/movie/%d/recommendations", p.baseURL, id)

	var response struct {
		Results []tmdbMovieSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertMovieSearchResults(response.Results), nil
}

// GetSeriesRecommendations retrieves TV series recommendations.
func (p *Provider) GetSeriesRecommendations(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	url := fmt.Sprintf("%s/tv/%d/recommendations", p.baseURL, id)

	var response struct {
		Results []tmdbSeriesSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return convertSeriesSearchResults(response.Results), nil
}

//
// Provider-Specific Methods (not in base MetadataProvider interface)
// These can be accessed via type assertion
//

// GetTMDBList retrieves a TMDB list by ID
// This is a provider-specific method not in the base interface.
func (p *Provider) GetTMDBList(ctx context.Context, listID int) (*TMDBListResponse, error) {
	url := fmt.Sprintf("%s/list/%d", p.baseURL, listID)

	var response TMDBListResponse
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// RemoveFromTMDBList removes a movie from a TMDB list
// This is a provider-specific method not in the base interface.
func (p *Provider) RemoveFromTMDBList(ctx context.Context, listID int, mediaID int) error {
	url := fmt.Sprintf("%s/list/%d/remove_item", p.baseURL, listID)

	// Create request body
	requestBody := map[string]any{
		"media_id": mediaID,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]string{
		"Authorization": "Bearer " + p.apiKey,
		"Accept":        "application/json",
		"Content-Type":  "application/json",
	}

	return p.baseClient.MakeRequestWithHeaders(
		ctx,
		"POST",
		url,
		bytes.NewReader(bodyBytes),
		nil,
		func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("TMDB API returned status %d", resp.StatusCode)
			}
			return nil
		},
		headers,
	)
}

//
// Discover Methods - Provider-specific methods for dynamic content discovery
//

// DiscoverMovies searches for movies using TMDB Discover API with filter parameters.
// The params string should be in URL query format (e.g., "sort_by=popularity.desc&vote_average.gte=7").
// Returns paginated results matching the discover criteria.
//
// Common discover parameters include:
//   - sort_by: popularity.desc, vote_average.desc, release_date.desc, etc.
//   - with_genres: Comma-separated genre IDs (28=Action, 12=Adventure, etc.)
//   - vote_average.gte/lte: Filter by vote average rating
//   - vote_count.gte: Minimum vote count
//   - release_date.gte/lte: Filter by release date
//   - primary_release_year: Filter by specific year
//
// See TMDB API docs: https://developer.themoviedb.org/reference/discover-movie
func (p *Provider) DiscoverMovies(
	ctx context.Context,
	params string,
	page int,
) (*apiexternal_v2.PopularMoviesResponse, error) {
	// Build the discover URL with pagination
	url := fmt.Sprintf("%s/discover/movie?page=%d", p.baseURL, page)

	// Append custom filter parameters if provided
	if params != "" {
		// Remove leading & or ? if present
		params = strings.TrimPrefix(params, "&")
		params = strings.TrimPrefix(params, "?")

		url += "&" + params
	}

	var response struct {
		Page         int                     `json:"page"`
		TotalPages   int                     `json:"total_pages"`
		TotalResults int                     `json:"total_results"`
		Results      []tmdbMovieSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.PopularMoviesResponse{
		Page:         response.Page,
		TotalPages:   response.TotalPages,
		TotalResults: response.TotalResults,
		Results:      convertMovieSearchResults(response.Results),
	}, nil
}

// DiscoverTV searches for TV shows using TMDB Discover API with filter parameters.
// The params string should be in URL query format (e.g., "sort_by=popularity.desc&vote_average.gte=7").
// Returns paginated results matching the discover criteria.
//
// Common discover parameters include:
//   - sort_by: popularity.desc, vote_average.desc, first_air_date.desc, etc.
//   - with_genres: Comma-separated genre IDs (28=Action, 12=Adventure, etc.)
//   - vote_average.gte/lte: Filter by vote average rating
//   - vote_count.gte: Minimum vote count
//   - first_air_date.gte/lte: Filter by first air date
//   - first_air_date_year: Filter by specific year
//
// See TMDB API docs: https://developer.themoviedb.org/reference/discover-tv
func (p *Provider) DiscoverTV(
	ctx context.Context,
	params string,
	page int,
) (*apiexternal_v2.PopularSeriesResponse, error) {
	// Build the discover URL with pagination
	url := fmt.Sprintf("%s/discover/tv?page=%d", p.baseURL, page)

	// Append custom filter parameters if provided
	if params != "" {
		// Remove leading & or ? if present
		params = strings.TrimPrefix(params, "&")
		params = strings.TrimPrefix(params, "?")

		url += "&" + params
	}

	var response struct {
		Page         int                      `json:"page"`
		TotalPages   int                      `json:"total_pages"`
		TotalResults int                      `json:"total_results"`
		Results      []tmdbSeriesSearchResult `json:"results"`
	}

	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return &apiexternal_v2.PopularSeriesResponse{
		Page:         response.Page,
		TotalPages:   response.TotalPages,
		TotalResults: response.TotalResults,
		Results:      convertSeriesSearchResults(response.Results),
	}, nil
}

//
// HTTP Helper
//

// makeRequest performs an HTTP GET request and decodes the JSON response
// Uses the base client for rate limiting, circuit breaker, and other infrastructure
// Automatically adds Bearer token authentication via Authorization header.
func (p *Provider) makeRequest(ctx context.Context, url string, target any) error {
	// Extract just the path from the full URL
	// TMDB URLs come in as full URLs like "https://api.themoviedb.org/3/search/movie?..."
	// We need to extract the path AFTER "/3/" since BaseURL already includes "/3"
	endpoint := url
	if idx := strings.Index(url, "/3/"); idx != -1 {
		// Extract everything AFTER "/3/", not including "/3/"
		endpoint = url[idx+3:] // +3 to skip over "/3/"
	}

	// Use BaseClient's MakeRequestWithHeaders with custom Bearer token
	return p.baseClient.MakeRequestWithHeaders(
		ctx,
		"GET",
		"/"+endpoint, // Add leading slash for proper URL construction
		nil,
		target,
		nil,
		map[string]string{
			"Authorization": "Bearer " + p.apiKey,
			"Accept":        "application/json",
		},
	)
}

// Helper function to convert year from release date.
func extractYear(dateStr string) int {
	if dateStr == "" {
		return 0
	}

	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}

	return 0
}
