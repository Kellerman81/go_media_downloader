package goodreads

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

//
// Goodreads Provider - Book metadata and reviews
// Note: Goodreads API was deprecated in 2020, but some endpoints still work.
// This implementation focuses on available public endpoints.
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the book metadata provider for Goodreads.
type Provider struct {
	*base.BaseClient
	apiKey string
}

// NewProviderWithConfig creates a new Goodreads provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig, apiKey string) *Provider {
	cfg.Name = "goodreads"
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://www.goodreads.com"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 1 // Goodreads is strict
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 1
	}

	if cfg.UserAgent == "" {
		cfg.UserAgent = config.GetSettingsGeneral().UserAgent
	}

	return &Provider{
		BaseClient: base.NewBaseClient(cfg),
		apiKey:     apiKey,
	}
}

// NewProvider creates a new Goodreads provider with API key.
func NewProvider(apiKey string) *Provider {
	return NewProviderWithConfig(base.ClientConfig{}, apiKey)
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderGoodreads
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "goodreads"
}

//
// Search Methods
//

// SearchBooks searches for books by query.
func (p *Provider) SearchBooks(
	ctx context.Context,
	query string,
	page int,
) ([]apiexternal_v2.BookSearchResult, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("goodreads API key is required")
	}

	endpoint := fmt.Sprintf("/search/index.xml?key=%s&q=%s",
		p.apiKey, url.QueryEscape(query))
	if page > 1 {
		endpoint += fmt.Sprintf("&page=%d", page)
	}

	var response grSearchResponse

	err := p.MakeRequest(ctx, "GET", endpoint, nil, nil,
		func(resp *http.Response) error {
			return xml.NewDecoder(resp.Body).Decode(&response)
		},
	)
	if err != nil {
		return nil, err
	}

	return convertSearchResults(response.Search.Results.Work), nil
}

// SearchByISBN searches for a book by ISBN.
func (p *Provider) SearchByISBN(
	ctx context.Context,
	isbn string,
) (*apiexternal_v2.BookDetails, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("goodreads API key is required")
	}

	endpoint := fmt.Sprintf("/book/isbn/%s?key=%s", isbn, p.apiKey)

	var response grBookResponse

	err := p.MakeRequest(ctx, "GET", endpoint, nil, nil,
		func(resp *http.Response) error {
			return xml.NewDecoder(resp.Body).Decode(&response)
		},
	)
	if err != nil {
		return nil, err
	}

	return convertBookToDetails(&response.Book), nil
}

//
// Lookup Methods
//

// GetBookByID retrieves book details by Goodreads book ID.
func (p *Provider) GetBookByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.BookDetails, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("goodreads API key is required")
	}

	endpoint := fmt.Sprintf("/book/show/%s.xml?key=%s", id, p.apiKey)

	var response grBookResponse

	err := p.MakeRequest(ctx, "GET", endpoint, nil, nil,
		func(resp *http.Response) error {
			return xml.NewDecoder(resp.Body).Decode(&response)
		},
	)
	if err != nil {
		return nil, err
	}

	return convertBookToDetails(&response.Book), nil
}

// GetAuthorByID retrieves author details by Goodreads author ID.
func (p *Provider) GetAuthorByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.AuthorDetails, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("goodreads API key is required")
	}

	endpoint := fmt.Sprintf("/author/show/%s?key=%s", id, p.apiKey)

	var response grAuthorResponse

	err := p.MakeRequest(ctx, "GET", endpoint, nil, nil,
		func(resp *http.Response) error {
			return xml.NewDecoder(resp.Body).Decode(&response)
		},
	)
	if err != nil {
		return nil, err
	}

	return convertAuthorToDetails(&response.Author), nil
}

// GetBooksByAuthor retrieves books by an author.
func (p *Provider) GetBooksByAuthor(
	ctx context.Context,
	authorID string,
	page int,
) ([]apiexternal_v2.BookSearchResult, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("goodreads API key is required")
	}

	endpoint := fmt.Sprintf("/author/list/%s?key=%s&format=xml", authorID, p.apiKey)
	if page > 1 {
		endpoint += fmt.Sprintf("&page=%d", page)
	}

	var response grAuthorBooksResponse

	err := p.MakeRequest(ctx, "GET", endpoint, nil, nil,
		func(resp *http.Response) error {
			return xml.NewDecoder(resp.Body).Decode(&response)
		},
	)
	if err != nil {
		return nil, err
	}

	return convertAuthorBooksToResults(response.Author.Books.Book), nil
}

// GetSeriesByID retrieves series details by Goodreads series ID.
func (p *Provider) GetSeriesByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.BookSeriesDetails, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("goodreads API key is required")
	}

	endpoint := fmt.Sprintf("/series/%s?key=%s", id, p.apiKey)

	var response grSeriesResponse

	err := p.MakeRequest(ctx, "GET", endpoint, nil, nil,
		func(resp *http.Response) error {
			return xml.NewDecoder(resp.Body).Decode(&response)
		},
	)
	if err != nil {
		return nil, err
	}

	return convertSeriesToDetails(&response.Series), nil
}
