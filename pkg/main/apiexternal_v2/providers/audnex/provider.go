package audnex

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Audnex Provider - Enhanced audiobook metadata including chapter info
// API: https://api.audnex.us
// Provides chapter data and enhanced metadata for Audible audiobooks
//

const (
	defaultBaseURL = "https://api.audnex.us"
)

// Provider implements the audiobook metadata provider for Audnex.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a new Audnex provider with custom config.
func NewProviderWithConfig(config base.ClientConfig) *Provider {
	config.Name = "audnex"
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.RateLimitCalls == 0 {
		config.RateLimitCalls = 10
	}

	if config.RateLimitSeconds == 0 {
		config.RateLimitSeconds = 1
	}

	return &Provider{
		BaseClient: base.NewBaseClient(config),
	}
}

// NewProvider creates a new Audnex provider with default configuration.
func NewProvider() *Provider {
	return NewProviderWithConfig(base.ClientConfig{})
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderAudnex
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "audnex"
}

//
// Book/Audiobook Methods
//

// GetBookByASIN retrieves book/audiobook metadata by ASIN.
func (p *Provider) GetBookByASIN(
	ctx context.Context,
	asin string,
	region string,
) (*apiexternal_v2.AudiobookDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/books/")
	buf.WriteURL(asin)
	buf.WriteString("?region=")
	buf.WriteURL(region)
	buf.WriteString("&update=1")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audnexBookResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertBookToDetails(&response), nil
}

// GetChaptersByASIN retrieves chapter information for an audiobook.
func (p *Provider) GetChaptersByASIN(
	ctx context.Context,
	asin string,
	region string,
) ([]apiexternal_v2.AudiobookChapter, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/books/")
	buf.WriteURL(asin)
	buf.WriteString("/chapters?region=")
	buf.WriteURL(region)
	buf.WriteString("&update=1")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audnexChaptersResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertChapters(response.Chapters), nil
}

//
// Author Methods
//

// GetAuthorByASIN retrieves author metadata by ASIN.
func (p *Provider) GetAuthorByASIN(
	ctx context.Context,
	asin string,
) (*apiexternal_v2.AuthorDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/authors/")
	buf.WriteURL(asin)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audnexAuthorResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertAuthorToDetails(&response), nil
}

// GetAuthorBooks retrieves all books by an author.
func (p *Provider) GetAuthorBooks(
	ctx context.Context,
	asin string,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/authors/")
	buf.WriteURL(asin)
	buf.WriteString("/books")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response audnexAuthorBooksResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertAuthorBooks(response.Books), nil
}

// SearchAuthorByName searches for authors by name.
// Returns a list of matching authors with their ASINs.
// API: GET /authors?name={name}.
func (p *Provider) SearchAuthorByName(
	ctx context.Context,
	name string,
) ([]apiexternal_v2.AuthorSearchResult, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/authors?name=")
	buf.WriteURL(name)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response []audnexAuthorSearchResult
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	results := make([]apiexternal_v2.AuthorSearchResult, 0, len(response))
	for i := range response {
		results = append(results, apiexternal_v2.AuthorSearchResult{
			ID:           response[i].ASIN,
			Name:         response[i].Name,
			ProviderType: apiexternal_v2.ProviderAudnex,
		})
	}

	return results, nil
}

//
// Series Methods
//
