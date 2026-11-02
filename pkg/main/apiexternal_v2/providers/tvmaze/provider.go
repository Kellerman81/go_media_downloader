package tvmaze

import (
	"context"
	"fmt"
	"net/url"

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
func (*Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderTVMaze
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "tvmaze"
}

// FindMovieByIMDbID - TVMaze doesn't support movies.
func (p *Provider) FindMovieByIMDbID(
	ctx context.Context,
	imdbID string,
) (*apiexternal_v2.FindByIMDbResult, error) {
	// TVMaze can search by IMDb ID for TV shows
	endpoint := fmt.Sprintf("/lookup/shows?imdb=%s", url.QueryEscape(imdbID))

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
				TVDbID:       response.Externals.TVDB,
				IMDbID:       response.Externals.IMDB,
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
	_ int,
) ([]apiexternal_v2.SeriesSearchResult, error) {
	endpoint := fmt.Sprintf("/search/shows?q=%s", url.QueryEscape(query))

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
	endpoint := fmt.Sprintf("/shows/%d", id)

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
