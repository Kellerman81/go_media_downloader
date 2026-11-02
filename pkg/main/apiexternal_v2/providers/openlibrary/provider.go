package openlibrary

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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

	// The configured OpenLibrary limits apply to every instance; the built-in
	// limits below are only the fallback for 0.
	general := config.GetSettingsGeneral()
	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = general.OpenLibraryLimiterCalls
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = int(general.OpenLibraryLimiterSeconds)
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
func (*Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderOpenLibrary
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
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
	// q is parsed as a Solr query; escape user text so e.g. "Title: Subtitle"
	// is not read as a field search.
	searchURL := fmt.Sprintf("/search.json?q=%s", url.QueryEscape(logger.LuceneEscape(query)))
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
	endpoint := fmt.Sprintf("/isbn/%s.json", url.PathEscape(isbn))

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
// ID format: /works/OL123W, /books/OL123M or the bare OL123W / OL123M.
func (p *Provider) GetBookByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.BookDetails, error) {
	endpoint := olKeyPath(id, "works") + ".json"

	var response olWorkResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertWorkToBookDetails(&response), nil
}

// GetAuthorByID retrieves author details by OpenLibrary author ID.
// ID format: /authors/OL123A or the bare OL123A.
func (p *Provider) GetAuthorByID(
	ctx context.Context,
	id string,
) (*apiexternal_v2.AuthorDetails, error) {
	endpoint := olKeyPath(id, "authors") + ".json"

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
	endpoint := fmt.Sprintf("/authors/%s/works.json", url.PathEscape(extractOLID(authorID)))
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
	endpoint := fmt.Sprintf("/works/%s/editions.json", url.PathEscape(extractOLID(workID)))
	if limit > 0 {
		endpoint += fmt.Sprintf("?limit=%d", limit)
	}

	var response olEditionsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertEditionsToSearchResults(response.Entries), nil
}

// olKeyPath turns an OpenLibrary key into its request path. Full keys
// ("/works/OL1W") are used as is; bare IDs as stored in the database ("OL1W")
// get the collection implied by their type suffix, or fallback.
func olKeyPath(id, fallback string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "/") {
		return id
	}

	collection := fallback
	if id != "" {
		switch id[len(id)-1] {
		case 'W':
			collection = "works"
		case 'M':
			collection = "books"
		case 'A':
			collection = "authors"
		}
	}

	return logger.JoinStrings("/", collection, "/", url.PathEscape(id))
}

// GetCoverURL returns the URL for a book cover.
// coverType: "id" (cover ID), "isbn", "oclc", "lccn", "olid"
// size: "S" (small), "M" (medium), "L" (large)
func (*Provider) GetCoverURL(coverType, value, size string) string {
	return fmt.Sprintf("https://covers.openlibrary.org/b/%s/%s-%s.jpg", coverType, value, size)
}

// GetAuthorPhotoURL returns the URL for an author photo.
// size: "S" (small), "M" (medium), "L" (large)
func (*Provider) GetAuthorPhotoURL(authorID, size string) string {
	return fmt.Sprintf("https://covers.openlibrary.org/a/olid/%s-%s.jpg", authorID, size)
}
