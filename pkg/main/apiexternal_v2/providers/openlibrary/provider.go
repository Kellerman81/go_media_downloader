package openlibrary

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

//
// OpenLibrary Provider - Free book metadata API
// Fully typed implementation with BaseClient infrastructure
//

// Provider implements the book metadata provider for OpenLibrary.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a new OpenLibrary provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig) *Provider {
	cfg.Name = "openlibrary"
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openlibrary.org"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 10 // OpenLibrary is generous
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 1
	}

	if cfg.UserAgent == "" {
		cfg.UserAgent = config.GetSettingsGeneral().UserAgent
	}

	return &Provider{
		BaseClient: base.NewBaseClient(cfg),
	}
}

// NewProvider creates a new OpenLibrary provider with default config.
func NewProvider() *Provider {
	return NewProviderWithConfig(base.ClientConfig{})
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderOpenLibrary
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "openlibrary"
}

//
// Search Methods
//

// SearchBooks searches for books by title and optional author.
func (p *Provider) SearchBooks(
	ctx context.Context,
	query string,
	author string,
	limit int,
) ([]apiexternal_v2.BookSearchResult, error) {
	// Build search URL
	searchURL := fmt.Sprintf("/search.json?q=%s", url.QueryEscape(query))
	if author != "" {
		searchURL += fmt.Sprintf("&author=%s", url.QueryEscape(author))
	}

	if limit > 0 {
		searchURL += fmt.Sprintf("&limit=%d", limit)
	}

	var response olSearchResponse
	if err := p.MakeRequest(ctx, "GET", searchURL, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertSearchResults(response.Docs), nil
}

// SearchByISBN searches for a book by ISBN.
func (p *Provider) SearchByISBN(
	ctx context.Context,
	isbn string,
) (*apiexternal_v2.BookDetails, error) {
	endpoint := fmt.Sprintf("/isbn/%s.json", isbn)

	var response olEditionResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertEditionToBookDetails(&response), nil
}

//
// Lookup Methods
//

// GetBookByID retrieves book details by OpenLibrary ID.
// ID format: /works/OL123W or /books/OL123M.
func (p *Provider) GetBookByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.BookDetails, error) {
	endpoint := fmt.Sprintf("%s.json", id)

	var response olWorkResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertWorkToBookDetails(&response), nil
}

// GetAuthorByID retrieves author details by OpenLibrary author ID.
// ID format: /authors/OL123A.
func (p *Provider) GetAuthorByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.AuthorDetails, error) {
	endpoint := fmt.Sprintf("%s.json", id)

	var response olAuthorResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertToAuthorDetails(&response), nil
}

// GetBooksByAuthor retrieves all books by an author.
func (p *Provider) GetBooksByAuthor(
	ctx context.Context,
	authorID string,
	limit int,
) ([]apiexternal_v2.BookSearchResult, error) {
	// Extract the author key if full path given
	endpoint := fmt.Sprintf("/authors/%s/works.json", authorID)
	if limit > 0 {
		endpoint += fmt.Sprintf("?limit=%d", limit)
	}

	var response olAuthorWorksResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertAuthorWorksToSearchResults(response.Entries), nil
}

// GetEditionsByWork retrieves all editions of a work.
func (p *Provider) GetEditionsByWork(
	ctx context.Context,
	workID string,
	limit int,
) ([]apiexternal_v2.BookSearchResult, error) {
	endpoint := fmt.Sprintf("/works/%s/editions.json", workID)
	if limit > 0 {
		endpoint += fmt.Sprintf("?limit=%d", limit)
	}

	var response olEditionsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertEditionsToSearchResults(response.Entries), nil
}

// GetCoverURL returns the URL for a book cover.
// coverType: "id" (cover ID), "isbn", "oclc", "lccn", "olid"
// size: "S" (small), "M" (medium), "L" (large)
func (p *Provider) GetCoverURL(coverType, value, size string) string {
	return fmt.Sprintf("https://covers.openlibrary.org/b/%s/%s-%s.jpg", coverType, value, size)
}

// GetAuthorPhotoURL returns the URL for an author photo.
// size: "S" (small), "M" (medium), "L" (large)
func (p *Provider) GetAuthorPhotoURL(authorID, size string) string {
	return fmt.Sprintf("https://covers.openlibrary.org/a/olid/%s-%s.jpg", authorID, size)
}
