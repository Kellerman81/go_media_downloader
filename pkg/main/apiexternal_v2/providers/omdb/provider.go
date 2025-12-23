package omdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// OMDB Provider - Open Movie Database
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the MetadataProvider interface for OMDB.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a new OMDB provider with custom config.
func NewProviderWithConfig(config base.ClientConfig) *Provider {
	config.Name = "omdb"
	if config.BaseURL == "" {
		config.BaseURL = "http://www.omdbapi.com"
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderOMDb
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "omdb"
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
	endpoint := fmt.Sprintf("/?s=%s&type=movie", query)
	if year > 0 {
		endpoint += fmt.Sprintf("&y=%d", year)
	}

	var response omdbSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if response.Response == "False" {
		return []apiexternal_v2.MovieSearchResult{}, nil
	}

	return convertSearchResults(response.Search, "omdb"), nil
}

// GetMovieByID retrieves detailed movie information by OMDB/IMDb ID.
func (p *Provider) GetMovieByID(ctx context.Context, id int) (*apiexternal_v2.MovieDetails, error) {
	// OMDB uses IMDb ID format, so convert if needed
	// For now, return error as OMDB doesn't use numeric IDs
	return nil, fmt.Errorf("OMDB requires IMDb ID (use FindMovieByIMDbID instead)")
}

// FindMovieByIMDbID finds movies by IMDb ID.
func (p *Provider) FindMovieByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	endpoint := fmt.Sprintf("/?i=%s&plot=full", imdbID)

	var response omdbDetailsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if response.Response == "False" {
		return &apiexternal_v2.FindByIMDbResult{}, nil
	}

	result := &apiexternal_v2.FindByIMDbResult{}

	if strings.ToLower(response.Type) == "movie" {
		result.MovieResults = []apiexternal_v2.MovieSearchResult{
			convertDetailsToSearchResult(&response),
		}
	} else if strings.ToLower(response.Type) == "series" {
		result.TVResults = []apiexternal_v2.SeriesSearchResult{
			convertDetailsToSeriesSearchResult(&response),
		}
	}

	return result, nil
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
	endpoint := fmt.Sprintf("/?s=%s&type=series", query)
	if year > 0 {
		endpoint += fmt.Sprintf("&y=%d", year)
	}

	var response omdbSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if response.Response == "False" {
		return []apiexternal_v2.SeriesSearchResult{}, nil
	}

	return convertSearchToSeriesResults(response.Search, "omdb"), nil
}

// GetSeriesByID retrieves detailed series information (OMDB requires IMDb ID).
func (p *Provider) GetSeriesByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.SeriesDetails, error) {
	return nil, fmt.Errorf("OMDB requires IMDb ID (use FindSeriesByIMDbID instead)")
}

// FindSeriesByIMDbID finds TV series by IMDb ID.
func (p *Provider) FindSeriesByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	return p.FindMovieByIMDbID(ctx, imdbID) // Same endpoint
}

//
// Episode Methods (Limited OMDB support)
//

// GetEpisodeDetailsByIMDb retrieves episode details using IMDb ID.
func (p *Provider) GetEpisodeDetailsByIMDb(
	ctx context.Context,
	imdbID string,
	seasonNumber int,
	episodeNumber int,
) (*apiexternal_v2.Episode, error) {
	endpoint := fmt.Sprintf("/?i=%s&Season=%d&Episode=%d", imdbID, seasonNumber, episodeNumber)

	var response omdbDetailsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if response.Response == "False" {
		return nil, fmt.Errorf("episode not found")
	}

	return convertDetailsToEpisode(&response, seasonNumber, episodeNumber), nil
}

//
// Credits (Limited OMDB support)
//

// GetMovieCreditsByIMDb retrieves basic cast info using IMDb ID.
func (p *Provider) GetMovieCreditsByIMDb(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.Credits, error) {
	endpoint := fmt.Sprintf("/?i=%s&plot=full", imdbID)

	var response omdbDetailsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertDetailsToCredits(&response), nil
}

//
// OMDB-specific convenience methods
//

// GetDetailsByIMDb retrieves full details using IMDb ID (recommended for OMDB).
func (p *Provider) GetDetailsByIMDb(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.MovieDetails, error) {
	endpoint := fmt.Sprintf("/?i=%s&plot=full", imdbID)

	var response omdbDetailsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if response.Response == "False" {
		return nil, fmt.Errorf("not found: %s", response.Error)
	}

	return convertDetailsToMovieDetails(&response), nil
}

// SearchByTitle is a convenience method for OMDB's primary search that returns raw OMDB results with IMDb IDs.
func (p *Provider) SearchByTitle(
	ctx context.Context,
	title string,
	year int,
	mediaType string,
) ([]OmdbSearchResult, error) {
	endpoint := fmt.Sprintf("/?s=%s", title)
	if year > 0 {
		endpoint += fmt.Sprintf("&y=%d", year)
	}

	if mediaType != "" {
		endpoint += fmt.Sprintf("&type=%s", mediaType) // movie, series, episode
	}

	var response omdbSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if response.Response == "False" {
		return []OmdbSearchResult{}, nil
	}

	return response.Search, nil
}

// Helper function to parse year from string.
func parseYear(yearStr string) int {
	// Handle formats like "2020", "2020-2023", "2020-"
	parts := strings.Split(yearStr, "-")
	if len(parts) > 0 && parts[0] != "" {
		if year, err := strconv.Atoi(parts[0]); err == nil {
			return year
		}
	}

	return 0
}

// Helper function to parse runtime.
func parseRuntime(runtimeStr string) int {
	// Format: "142 min"
	parts := strings.Fields(runtimeStr)
	if len(parts) > 0 {
		if runtime, err := strconv.Atoi(parts[0]); err == nil {
			return runtime
		}
	}

	return 0
}

// Helper function to parse rating.
func parseRating(ratingStr string) float64 {
	if rating, err := strconv.ParseFloat(ratingStr, 64); err == nil {
		return rating
	}
	return 0.0
}
