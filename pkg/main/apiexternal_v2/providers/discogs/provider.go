package discogs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Discogs Provider - Music database with detailed release information
// API: https://www.discogs.com/developers
// Rate limit: 60 requests per minute (authenticated), 25 without auth
//

const (
	defaultBaseURL = "https://api.discogs.com"
)

// Provider implements the music metadata provider for Discogs.
type Provider struct {
	*base.BaseClient
	token       string
	authHeaders map[string]string
}

// NewProviderWithConfig creates a new Discogs provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig, token string) *Provider {
	cfg.Name = "discogs"
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Discogs rate limits: 60/min authenticated, 25/min unauthenticated
	if cfg.RateLimitCalls == 0 {
		if token != "" {
			cfg.RateLimitCalls = 60
		} else {
			cfg.RateLimitCalls = 25
		}
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 60
	}

	// Discogs requires a User-Agent
	if cfg.UserAgent == "" {
		cfg.UserAgent = config.GetSettingsGeneral().UserAgent
	}

	p := &Provider{
		BaseClient: base.NewBaseClient(cfg),
		token:      token,
	}
	if token != "" {
		p.authHeaders = map[string]string{
			"Authorization": logger.JoinStrings("Discogs token=", token),
		}
	}

	return p
}

// NewProvider creates a new Discogs provider without authentication.
func NewProvider() *Provider {
	return NewProviderWithConfig(base.ClientConfig{}, "")
}

// NewProviderWithToken creates a new Discogs provider with authentication token.
func NewProviderWithToken(token string) *Provider {
	return NewProviderWithConfig(base.ClientConfig{}, token)
}

// GetProviderType returns the provider type.
func (*Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderDiscogs
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "discogs"
}

//
// Search Methods
//

// Search performs a general search across all types.
func (p *Provider) Search(
	ctx context.Context,
	query string,
	searchType string, // "release", "artist", "label", "master", or empty for all
	limit int,
	page int,
) (*discogsSearchResponse, error) {
	if limit <= 0 {
		limit = 50
	}

	if page <= 0 {
		page = 1
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/database/search")
	buf.WriteString("?q=")
	buf.WriteURL(query) // URL-encode the query

	if searchType != "" {
		buf.WriteString("&type=")
		buf.WriteString(searchType)
	}

	buf.WriteString("&per_page=")
	buf.WriteInt(limit)
	buf.WriteString("&page=")
	buf.WriteInt(page)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response discogsSearchResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return &response, nil
}

// SearchReleases searches for releases.
func (p *Provider) SearchReleases(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	response, err := p.Search(ctx, query, "release", limit, 1)
	if err != nil {
		return nil, err
	}

	return convertSearchResultsToReleases(response.Results), nil
}

// SearchArtists searches for artists.
func (p *Provider) SearchArtists(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.ArtistSearchResult, error) {
	response, err := p.Search(ctx, query, "artist", limit, 1)
	if err != nil {
		return nil, err
	}

	return convertSearchResultsToArtists(response.Results), nil
}

//
// Artist Methods
//

// GetArtistByID retrieves artist details by Discogs ID.
func (p *Provider) GetArtistByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ArtistDetails, error) {
	endpoint := fmt.Sprintf("/artists/%d", id)

	var response discogsArtistResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return convertArtistToDetails(&response), nil
}

// GetArtistReleases retrieves releases for an artist.
func (p *Provider) GetArtistReleases(
	ctx context.Context,
	id int,
	limit int,
	page int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	if page <= 0 {
		page = 1
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/artists/")
	buf.WriteInt(id)
	buf.WriteString("/releases?per_page=")
	buf.WriteInt(limit)
	buf.WriteString("&page=")
	buf.WriteInt(page)
	buf.WriteString("&sort=year&sort_order=desc")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response discogsArtistReleasesResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return convertArtistReleasesToResults(response.Releases), nil
}

//
// Release Methods
//

// GetReleaseByID retrieves release details by Discogs ID.
func (p *Provider) GetReleaseByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ReleaseDetails, error) {
	endpoint := fmt.Sprintf("/releases/%d", id)

	var response discogsReleaseResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return convertReleaseToDetails(&response), nil
}

// GetReleaseByBarcode searches for a release by barcode.
func (p *Provider) GetReleaseByBarcode(
	ctx context.Context,
	barcode string,
) (*apiexternal_v2.ReleaseDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/database/search?barcode=")
	buf.WriteURL(barcode)
	buf.WriteString("&type=release")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response discogsSearchResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, errors.New(logger.JoinStrings("no release found with barcode: ", barcode))
	}

	// Get the first result's ID and fetch full details
	return p.GetReleaseByID(ctx, response.Results[0].ID)
}

//
// Master Release Methods
//

// GetMasterByID retrieves master release details by Discogs ID.
// A master release groups all versions of the same album.
func (p *Provider) GetMasterByID(
	ctx context.Context,
	id int,
) (*apiexternal_v2.ReleaseDetails, error) {
	endpoint := logger.JoinStrings("/masters/", strconv.Itoa(id))

	var response discogsMasterResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return convertMasterToDetails(&response), nil
}

// GetMasterVersions retrieves all versions of a master release.
func (p *Provider) GetMasterVersions(
	ctx context.Context,
	id int,
	limit int,
	page int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	if page <= 0 {
		page = 1
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/masters/")
	buf.WriteInt(id)
	buf.WriteString("/versions?per_page=")
	buf.WriteInt(limit)
	buf.WriteString("&page=")
	buf.WriteInt(page)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response discogsMasterVersionsResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return convertVersionsToResults(response.Versions), nil
}

//
// Label Methods
//

// GetLabelByID retrieves label details by Discogs ID.
func (p *Provider) GetLabelByID(ctx context.Context, id int) (*discogsLabelResponse, error) {
	endpoint := logger.JoinStrings("/labels/", strconv.Itoa(id))

	var response discogsLabelResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetLabelReleases retrieves releases on a label.
func (p *Provider) GetLabelReleases(
	ctx context.Context,
	id int,
	limit int,
	page int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	if page <= 0 {
		page = 1
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/labels/")
	buf.WriteInt(id)
	buf.WriteString("/releases?per_page=")
	buf.WriteInt(limit)
	buf.WriteString("&page=")
	buf.WriteInt(page)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response discogsLabelReleasesResponse
	if err := p.MakeRequestWithHeaders(
		ctx,
		"GET",
		endpoint,
		nil,
		&response,
		nil,
		p.authHeaders,
	); err != nil {
		return nil, err
	}

	return convertLabelReleasesToResults(response.Releases), nil
}
