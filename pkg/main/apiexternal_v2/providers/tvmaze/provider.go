package tvmaze

import (
	"context"
	"fmt"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

//
// TVMaze Provider - TV show metadata and schedules
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the MetadataProvider interface for TVMaze.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a new TVMaze provider with custom config.
func NewProviderWithConfig(config base.ClientConfig) *Provider {
	config.Name = "tvmaze"
	if config.BaseURL == "" {
		config.BaseURL = "https://api.tvmaze.com"
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderTVMaze
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "tvmaze"
}

// FindMovieByIMDbID - TVMaze doesn't support movies.
func (p *Provider) FindMovieByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	// TVMaze can search by IMDb ID for TV shows
	endpoint := fmt.Sprintf("/lookup/shows?imdb=%s", imdbID)

	var response tvmazeShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	result := &apiexternal_v2.FindByIMDbResult{
		TVResults: []apiexternal_v2.SeriesSearchResult{
			{
				ID:           response.ID,
				Name:         response.Name,
				FirstAirDate: parseTVMazeDate(response.Premiered),
				Overview:     stripHTML(response.Summary),
				VoteAverage:  response.Rating.Average,
				ProviderName: "tvmaze",
			},
		},
	}

	return result, nil
}

//
// TV Series Methods
//

// SearchSeries searches for TV series by title.
func (p *Provider) SearchSeries(
	ctx context.Context,
	query string,
	year int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	endpoint := fmt.Sprintf("/search/shows?q=%s", query)

	var response tvmazeSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertSearchResults(response, "tvmaze"), nil
}

// GetSeriesByID retrieves detailed series information by TVMaze ID.
func (p *Provider) GetSeriesByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.SeriesDetails, error) {
	endpoint := fmt.Sprintf("/shows/%d?embed[]=seasons&embed[]=episodes", id)

	var response tvmazeShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertShowToDetails(&response), nil
}

// FindSeriesByIMDbID finds TV series by IMDb ID.
func (p *Provider) FindSeriesByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	return p.FindMovieByIMDbID(ctx, imdbID)
}

// FindSeriesByTVDbID finds TV series by TVDb ID.
func (p *Provider) FindSeriesByTVDbID(
	ctx context.Context,
	tvdbID int,
) (*apiexternal_v2.SeriesDetails, error) {
	endpoint := fmt.Sprintf("/lookup/shows?thetvdb=%d", tvdbID)

	var response tvmazeShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertShowToDetails(&response), nil
}

// GetSeriesExternalIDs retrieves external IDs for a series.
func (p *Provider) GetSeriesExternalIDs(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ExternalIDs, error) {
	endpoint := fmt.Sprintf("/shows/%d", id)

	var response tvmazeShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertExternalsToExternalIDs(response.Externals, id), nil
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
	endpoint := fmt.Sprintf("/shows/%d/seasons", seriesID)

	var response tvmazeSeasonResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	// Find the specific season
	for _, s := range response {
		if s.Number == seasonNumber {
			return convertSeasonToDetails(&s), nil
		}
	}

	return nil, fmt.Errorf("season %d not found", seasonNumber)
}

// GetSeasonDetails retrieves detailed information about a season.
func (p *Provider) GetEpisodes(
	ctx context.Context,
	seriesID int,
) ([]*apiexternal_v2.Episode, error) {
	endpoint := fmt.Sprintf("/shows/%d/episodes", seriesID)

	var response tvmazeEpisodeResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	episodes := make([]*apiexternal_v2.Episode, 0, len(response))
	for idx := range response {
		episodes = append(episodes, convertEpisodeToDetails(&response[idx]))
	}

	return episodes, nil
}

// GetSeasonDetails retrieves detailed information about a season.
func (p *Provider) GetSeasons(
	ctx context.Context,
	seriesID int,
) ([]*apiexternal_v2.Season, error) {
	endpoint := fmt.Sprintf("/shows/%d/seasons", seriesID)

	var response tvmazeSeasonResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	seasons := make([]*apiexternal_v2.Season, 0, len(response))
	for idx := range response {
		seasons = append(seasons, convertSeasonToDetails(&response[idx]))
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
		"/shows/%d/episodebynumber?season=%d&number=%d",
		seriesID,
		seasonNumber,
		episodeNumber,
	)

	var response tvmazeEpisode
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertEpisodeToDetails(&response), nil
}

//
// Popular/Trending (Not directly supported by TVMaze)
//

// GetPopularSeries - TVMaze doesn't have a direct popular endpoint.
func (p *Provider) GetPopularSeries(
	ctx context.Context,
	page int,
) (*apiexternal_v2.PopularSeriesResponse, error) {
	// TVMaze has a "show index" that returns all shows
	// We can use this with pagination to simulate popular shows
	endpoint := fmt.Sprintf("/shows?page=%d", page)

	var response []tvmazeShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	results := make([]apiexternal_v2.SeriesSearchResult, len(response))
	for i, s := range response {
		results[i] = apiexternal_v2.SeriesSearchResult{
			ID:           s.ID,
			Name:         s.Name,
			FirstAirDate: parseTVMazeDate(s.Premiered),
			Overview:     stripHTML(s.Summary),
			VoteAverage:  s.Rating.Average,
			ProviderName: "tvmaze",
		}
	}

	return &apiexternal_v2.PopularSeriesResponse{
		Page:         page,
		Results:      results,
		TotalPages:   1, // TVMaze doesn't provide total pages
		TotalResults: len(results),
	}, nil
}

//
// Credits
//

// GetSeriesCredits retrieves cast information for a series.
func (p *Provider) GetSeriesCredits(ctx context.Context, id int) (*apiexternal_v2.Credits, error) {
	castEndpoint := fmt.Sprintf("/shows/%d/cast", id)
	crewEndpoint := fmt.Sprintf("/shows/%d/crew", id)

	var castResponse tvmazeCastResponse
	if err := p.MakeRequest(ctx, "GET", castEndpoint, nil, &castResponse, nil); err != nil {
		return nil, err
	}

	credits := convertCastToCredits(castResponse)

	// Get crew separately
	var crewResponse tvmazeCrewResponse
	if err := p.MakeRequest(ctx, "GET", crewEndpoint, nil, &crewResponse, nil); err == nil {
		credits.Crew = convertCrewToCredits(crewResponse)
	}

	return credits, nil
}

//
// Media Resources
//

// GetSeriesImages retrieves images for a series.
func (p *Provider) GetSeriesImages(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ImageCollection, error) {
	endpoint := fmt.Sprintf("/shows/%d", id)

	var response tvmazeShow
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertImagesToCollection(response.Image), nil
}

//
// TVMaze-specific methods
//

// GetSchedule retrieves the TV schedule for a specific country and date.
func (p *Provider) GetSchedule(
	ctx context.Context,
	countryCode string,
	date string,
) ([]apiexternal_v2.Episode, error) {
	endpoint := fmt.Sprintf("/schedule?country=%s&date=%s", countryCode, date)

	var response tvmazeEpisodeResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	episodes := make([]apiexternal_v2.Episode, len(response))
	for i, ep := range response {
		episodes[i] = *convertEpisodeToDetails(&ep)
	}

	return episodes, nil
}

// GetFullSchedule retrieves the full TV schedule (all countries).
func (p *Provider) GetFullSchedule(ctx context.Context) ([]apiexternal_v2.Episode, error) {
	endpoint := "/schedule/full"

	var response tvmazeEpisodeResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	episodes := make([]apiexternal_v2.Episode, len(response))
	for i, ep := range response {
		episodes[i] = *convertEpisodeToDetails(&ep)
	}

	return episodes, nil
}

// GetShowUpdates retrieves shows that have been updated since a specific timestamp.
func (p *Provider) GetShowUpdates(ctx context.Context, since string) (map[int]int64, error) {
	endpoint := fmt.Sprintf("/updates/shows?since=%s", since)

	var response map[int]int64
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return response, nil
}

// GetEpisodesByDate retrieves all episodes airing on a specific date.
func (p *Provider) GetEpisodesByDate(
	ctx context.Context,
	date string,
) ([]apiexternal_v2.Episode, error) {
	endpoint := fmt.Sprintf("/schedule?date=%s", date)

	var response tvmazeEpisodeResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	episodes := make([]apiexternal_v2.Episode, len(response))
	for i, ep := range response {
		episodes[i] = *convertEpisodeToDetails(&ep)
	}

	return episodes, nil
}
