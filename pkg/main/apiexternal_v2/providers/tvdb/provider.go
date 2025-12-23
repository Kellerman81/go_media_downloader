package tvdb

import (
	"context"
	"fmt"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// TVDB Provider - The TV Database
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the MetadataProvider interface for TVDB.
type Provider struct {
	*base.BaseClient
	apiKey      string
	userKey     string
	username    string
	jwtToken    string
	tokenExpiry time.Time
}

// NewProviderWithConfig creates a new TVDB provider with custom config.
func NewProviderWithConfig(config base.ClientConfig, apiKey, userKey, username string) *Provider {
	config.Name = "tvdb"
	if config.BaseURL == "" {
		config.BaseURL = "https://api.thetvdb.com"
	}

	provider := &Provider{
		BaseClient: base.NewBaseClient(config),
		apiKey:     apiKey,
		userKey:    userKey,
		username:   username,
	}

	provider.refreshToken(context.Background())

	return provider
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderTVDb
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "tvdb"
}

// refreshToken gets a new JWT token from TVDB.
func (p *Provider) refreshToken(ctx context.Context) error {
	// TVDB v4 uses JWT authentication
	// Note: This would need a special request without auth for login
	// For now, this is a placeholder
	_ = ctx         // Unused for now
	p.jwtToken = "" // Token would be stored here
	p.tokenExpiry = time.Now().Add(24 * time.Hour)

	return nil
}

//
// Movie Methods (TVDB is primarily for TV, limited movie support)
//

// FindMovieByIMDbID - TVDB focuses on TV series.
func (p *Provider) FindMovieByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	// TVDB v2 can search by IMDb ID
	endpoint := fmt.Sprintf("/search/series?imdbId=%s", imdbID)

	var response tvdbSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	result := &apiexternal_v2.FindByIMDbResult{
		TVResults: convertSearchResults(response.Data, "tvdb"),
	}

	return result, nil
}

//
// TV Series Methods (TVDB's primary focus)
//

// SearchSeries searches for TV series by title.
func (p *Provider) SearchSeries(
	ctx context.Context,
	query string,
	year int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	endpoint := fmt.Sprintf("/search/series?name=%s", query)
	if year > 0 {
		endpoint += fmt.Sprintf("&year=%d", year)
	}

	var response tvdbSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertSearchResults(response.Data, "tvdb"), nil
}

// GetSeriesByID retrieves detailed series information by TVDB ID.
func (p *Provider) GetSeriesByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.SeriesDetails, error) {
	endpoint := fmt.Sprintf("/series/%d", id)

	var response tvdbV2SeriesResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertV2SeriesToDetails(&response.Data), nil
}

// FindSeriesByIMDbID finds TV series by IMDb ID.
func (p *Provider) FindSeriesByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	return p.FindMovieByIMDbID(ctx, imdbID)
}

// FindSeriesByTVDbID finds TV series by TVDb ID (direct lookup).
func (p *Provider) FindSeriesByTVDbID(
	ctx context.Context,
	tvdbID int,
) (*apiexternal_v2.SeriesDetails, error) {
	return p.GetSeriesByID(ctx, tvdbID)
}

// GetSeriesExternalIDs retrieves external IDs for a series.
func (p *Provider) GetSeriesExternalIDs(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ExternalIDs, error) {
	endpoint := fmt.Sprintf("/series/%d", id)

	var response tvdbV2SeriesResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	// v2 API provides IMDb ID and Zap2it ID directly
	return &apiexternal_v2.ExternalIDs{
		IMDbID: response.Data.ImdbID,
		TVDbID: id,
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
	// TVDB v2 episodes endpoint
	endpoint := fmt.Sprintf("/series/%d/episodes", seriesID)

	var response tvdbV2EpisodesResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	// Filter episodes for this season
	var seasonEpisodes []tvdbV2Episode
	for _, ep := range response.Data {
		if ep.AiredSeason == seasonNumber {
			seasonEpisodes = append(seasonEpisodes, ep)
		}
	}

	if len(seasonEpisodes) == 0 {
		return nil, fmt.Errorf("season %d not found", seasonNumber)
	}

	return &apiexternal_v2.Season{
		ID:           seriesID*1000 + seasonNumber, // Generate ID
		SeasonNumber: seasonNumber,
		EpisodeCount: len(seasonEpisodes),
	}, nil
}

// GetEpisodeDetails retrieves detailed information about an episode.
func (p *Provider) GetEpisodeDetails(
	ctx context.Context,
	seriesID int,
	seasonNumber int,
	episodeNumber int,
) (*apiexternal_v2.Episode, error) {
	// Get all episodes for the series, then filter
	endpoint := fmt.Sprintf("/series/%d/episodes", seriesID)

	var response tvdbV2EpisodesResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	// Find the specific episode
	for _, ep := range response.Data {
		if ep.AiredSeason == seasonNumber && ep.AiredEpisodeNumber == episodeNumber {
			return convertV2EpisodeToDetails(&ep), nil
		}
	}

	return nil, fmt.Errorf("episode S%02dE%02d not found", seasonNumber, episodeNumber)
}

// GetAllEpisodes retrieves ALL episodes for a series, handling pagination.
// TVDB API returns ~100 episodes per page, so we need to fetch all pages.
func (p *Provider) GetAllEpisodes(
	ctx context.Context,
	seriesID int,
) ([]apiexternal_v2.Episode, error) {
	allEpisodes := make([]apiexternal_v2.Episode, 0, 200) // Pre-allocate for typical series
	page := 1

	for {
		endpoint := fmt.Sprintf("/series/%d/episodes?page=%d", seriesID, page)

		var response tvdbV2EpisodesResponse
		if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
			return nil, err
		}

		// Convert episodes from this page
		for i := range response.Data {
			allEpisodes = append(allEpisodes, *convertV2EpisodeToDetails(&response.Data[i]))
		}

		// Check if there are more pages
		if response.Links.Next == nil {
			break // No more pages
		}

		page++

		// Safety check to prevent infinite loops (max 50 pages = ~5000 episodes)
		if page > 50 {
			break
		}
	}

	return allEpisodes, nil
}

//
// Alternative Titles
//

// GetSeriesAlternativeTitles retrieves alternative titles for a series.
func (p *Provider) GetSeriesAlternativeTitles(
	ctx context.Context,
	id int,
) ([]apiexternal_v2.AlternativeTitle, error) {
	endpoint := fmt.Sprintf("/series/%d", id)

	var response tvdbV2SeriesResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	// v2 API provides aliases as string array
	titles := make([]apiexternal_v2.AlternativeTitle, len(response.Data.Aliases))
	for i, alias := range response.Data.Aliases {
		titles[i] = apiexternal_v2.AlternativeTitle{
			Title: alias,
			Type:  "alias",
		}
	}

	return titles, nil
}

//
// Media Resources
//

// GetSeriesImages retrieves images for a series.
func (p *Provider) GetSeriesImages(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ImageCollection, error) {
	endpoint := fmt.Sprintf("/series/%d", id)

	var response tvdbV2SeriesResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	// v2 API provides basic image paths in series response
	images := &apiexternal_v2.ImageCollection{
		Posters:   []apiexternal_v2.Image{},
		Backdrops: []apiexternal_v2.Image{},
		Logos:     []apiexternal_v2.Image{},
	}

	if response.Data.Poster != "" {
		images.Posters = append(images.Posters, apiexternal_v2.Image{
			FilePath: response.Data.Poster,
		})
	}

	if response.Data.Fanart != "" {
		images.Backdrops = append(images.Backdrops, apiexternal_v2.Image{
			FilePath: response.Data.Fanart,
		})
	}

	if response.Data.Banner != "" {
		images.Logos = append(images.Logos, apiexternal_v2.Image{
			FilePath: response.Data.Banner,
		})
	}

	return images, nil
}
