package audible

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Audible Provider - Audiobook metadata from Audible catalog
// Fully typed implementation with BaseClient infrastructure
//

// Region represents an Audible marketplace region.
type Region string

const (
	RegionUS Region = "us"
	RegionUK Region = "uk"
	RegionCA Region = "ca"
	RegionAU Region = "au"
	RegionDE Region = "de"
	RegionFR Region = "fr"
	RegionIT Region = "it"
	RegionES Region = "es"
	RegionIN Region = "in"
	RegionJP Region = "jp"
)

// regionURLs maps regions to their Audible API base URLs.
var regionURLs = map[Region]string{
	RegionUS: "https://api.audible.com",
	RegionUK: "https://api.audible.co.uk",
	RegionCA: "https://api.audible.ca",
	RegionAU: "https://api.audible.com.au",
	RegionDE: "https://api.audible.de",
	RegionFR: "https://api.audible.fr",
	RegionIT: "https://api.audible.it",
	RegionES: "https://api.audible.es",
	RegionIN: "https://api.audible.in",
	RegionJP: "https://api.audible.co.jp",
}

// Provider implements the audiobook metadata provider for Audible.
type Provider struct {
	*base.BaseClient
	region Region
}

// NewProviderWithConfig creates a new Audible provider with custom config.
func NewProviderWithConfig(config base.ClientConfig, region Region) *Provider {
	config.Name = "audible"
	if config.BaseURL == "" {
		if baseURL, ok := regionURLs[region]; ok {
			config.BaseURL = baseURL
		} else {
			config.BaseURL = regionURLs[RegionUS]
		}
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.RateLimitCalls == 0 {
		config.RateLimitCalls = 5
	}

	if config.RateLimitSeconds == 0 {
		config.RateLimitSeconds = 1
	}

	// Important: User-Agent to avoid blocking
	if config.UserAgent == "" {
		config.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
		region:     region,
	}
}

// NewProvider creates a new Audible provider for the US region.
func NewProvider() *Provider {
	return NewProviderWithConfig(base.ClientConfig{}, RegionUS)
}

// NewProviderWithRegion creates a new Audible provider for a specific region.
func NewProviderWithRegion(region Region) *Provider {
	return NewProviderWithConfig(base.ClientConfig{}, region)
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderAudible
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "audible"
}

// GetRegion returns the current region.
func (p *Provider) GetRegion() Region {
	return p.region
}

// SetRegion sets the current region.
func (p *Provider) SetRegion(region Region) {
	p.region = region
	if baseURL, ok := regionURLs[region]; ok {
		p.SetBaseURL(baseURL)
	} else {
		p.SetBaseURL(regionURLs[RegionUS])
	}
}

//
// Search Methods
//

// SearchAudiobooks searches for audiobooks by keywords.
func (p *Provider) SearchAudiobooks(
	ctx context.Context,
	keywords string,
	numResults int,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	if numResults <= 0 {
		numResults = 10
	}

	if numResults > 50 {
		numResults = 50
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/1.0/catalog/products?keywords=")
	buf.WriteURL(keywords)
	buf.WriteString("&num_results=")
	buf.WriteInt(numResults)
	buf.WriteString(
		"&products_sort_by=Relevance&response_groups=product_desc%2Ccontributors%2Cmedia%2Cproduct_attrs%2Cseries",
	)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audibleSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertSearchResults(response.Products), nil
}

// SearchByASIN searches for an audiobook by ASIN.
func (p *Provider) SearchByASIN(
	ctx context.Context,
	asin string,
) (*apiexternal_v2.AudiobookDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/1.0/catalog/products/")
	buf.WriteString(asin)
	buf.WriteString(
		"?response_groups=product_desc%2Ccontributors%2Cmedia%2Cproduct_attrs%2Cseries%2Cchapter_info",
	)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audibleProductResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertProductToDetails(&response.Product), nil
}

// SearchByAuthor searches for audiobooks by author name.
func (p *Provider) SearchByAuthor(
	ctx context.Context,
	author string,
	numResults int,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	if numResults <= 0 {
		numResults = 10
	}

	if numResults > 50 {
		numResults = 50
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/1.0/catalog/products?author=")
	buf.WriteURL(author)
	buf.WriteString("&num_results=")
	buf.WriteInt(numResults)
	buf.WriteString(
		"&products_sort_by=Relevance&response_groups=product_desc%2Ccontributors%2Cmedia%2Cproduct_attrs%2Cseries",
	)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audibleSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertSearchResults(response.Products), nil
}

// SearchByNarrator searches for audiobooks by narrator name.
func (p *Provider) SearchByNarrator(
	ctx context.Context,
	narrator string,
	numResults int,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	if numResults <= 0 {
		numResults = 10
	}

	if numResults > 50 {
		numResults = 50
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/1.0/catalog/products?narrator=")
	buf.WriteURL(narrator)
	buf.WriteString("&num_results=")
	buf.WriteInt(numResults)
	buf.WriteString(
		"&products_sort_by=Relevance&response_groups=product_desc%2Ccontributors%2Cmedia%2Cproduct_attrs%2Cseries",
	)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audibleSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertSearchResults(response.Products), nil
}

// SearchByTitle searches for audiobooks by title.
func (p *Provider) SearchByTitle(
	ctx context.Context,
	title string,
	numResults int,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	if numResults <= 0 {
		numResults = 10
	}

	if numResults > 50 {
		numResults = 50
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/1.0/catalog/products?title=")
	buf.WriteURL(title)
	buf.WriteString("&num_results=")
	buf.WriteInt(numResults)
	buf.WriteString(
		"&products_sort_by=Relevance&response_groups=product_desc%2Ccontributors%2Cmedia%2Cproduct_attrs%2Cseries",
	)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audibleSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertSearchResults(response.Products), nil
}
