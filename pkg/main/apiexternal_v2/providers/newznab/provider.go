package newznab

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Newznab Provider - Newznab/Torznab indexer
// Fully typed implementation with BaseClient infrastructure
//

// ProviderConfig contains configuration for creating a Newznab provider.
type ProviderConfig struct {
	IndexerName       string // Name of the indexer (for logging and identification)
	BaseURL           string
	APIKey            string
	Categories        []string
	OutputAsJSON      bool
	TimeoutSeconds    uint16
	CustomAPI         string // Custom API parameter name (default: "apikey")
	CustomURL         string // Custom API URL path (default: "/api")
	CustomRSSURL      string // Custom RSS URL path (default: "/rss")
	CustomRSSCategory string // Custom RSS category parameter (default: "&t=")
	MaxAge            uint16 // Maximum age of releases in days
	MaxEntries        uint16 // Maximum number of entries per request
	AddQuotesForTitle bool   // Add quotes around title queries
	LimiterCalls      int    // Number of API calls allowed within LimiterSeconds (default: 200)
	LimiterSeconds    uint8  // Time window in seconds for rate limiting (default: 3600)
	LimiterCallsDaily int    // Maximum number of API calls allowed per day (default: 2000)
}

// Provider implements the IndexerProvider interface for Newznab/Torznab.
type Provider struct {
	*base.BaseClient
	DownloadClient    *base.BaseClient // Separate client for tracking download statistics
	baseURL           string
	apiKey            string
	categories        []string
	isTorznab         bool
	outputAsJSON      bool   // When true, request and parse JSON responses instead of XML
	customAPI         string // Custom API parameter name (if not "apikey")
	customURL         string // Custom API URL path (if not "/api")
	customRSSURL      string // Custom RSS URL path (if not "/rss")
	customRSSCategory string // Custom RSS category parameter (if not "&t=")
	maxAge            uint16 // Maximum age of releases in days
	maxEntries        uint16 // Maximum number of entries per request
	addQuotesForTitle bool   // Add quotes around title queries
}

var ErrBroke = errors.New("broke")

// NewProvider creates a new Newznab indexer provider
//
// Parameters:
//   - config: Provider configuration with all indexer settings
//
// Returns:
//   - *Provider: Configured Newznab/Torznab provider instance
func NewProvider(config ProviderConfig) *Provider {
	// Use configured timeout or default to 60 seconds if not specified
	timeout := 60 * time.Second
	if config.TimeoutSeconds > 0 {
		timeout = time.Duration(config.TimeoutSeconds) * time.Second
	}

	// Use configured rate limits or defaults
	rateLimitCalls := config.LimiterCalls
	if rateLimitCalls <= 0 {
		rateLimitCalls = 5 // Default: 200 calls per hour
	}

	rateLimitSeconds := int(config.LimiterSeconds)
	if rateLimitSeconds == 0 {
		rateLimitSeconds = 20 // Default: 1 hour
	}

	rateLimitPer24h := config.LimiterCallsDaily

	// Use indexer name if provided, otherwise default to "newznab"
	clientName := config.IndexerName
	if clientName == "" {
		clientName = "newznab"
	}

	clientConfig := base.ClientConfig{
		Name:                    clientName,
		BaseURL:                 config.BaseURL,
		Timeout:                 timeout,
		AuthType:                base.AuthNone, // API key in URL
		APIKey:                  config.APIKey,
		RateLimitCalls:          rateLimitCalls,
		RateLimitSeconds:        rateLimitSeconds,
		RateLimitPer24h:         rateLimitPer24h,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	// Create separate client for download statistics tracking
	// Downloads are rare (1-2 per thousands of searches) so they should be tracked separately
	downloadClientConfig := base.ClientConfig{
		Name:                    clientName + "_download",
		BaseURL:                 config.BaseURL,
		Timeout:                 timeout,
		AuthType:                base.AuthNone,
		APIKey:                  config.APIKey,
		RateLimitCalls:          rateLimitCalls,
		RateLimitSeconds:        rateLimitSeconds,
		RateLimitPer24h:         rateLimitPer24h,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	return &Provider{
		BaseClient:        base.NewBaseClient(clientConfig),
		DownloadClient:    base.NewBaseClient(downloadClientConfig),
		baseURL:           config.BaseURL,
		apiKey:            config.APIKey,
		categories:        config.Categories,
		outputAsJSON:      config.OutputAsJSON,
		customAPI:         config.CustomAPI,
		customURL:         config.CustomURL,
		customRSSURL:      config.CustomRSSURL,
		customRSSCategory: config.CustomRSSCategory,
		maxAge:            config.MaxAge,
		maxEntries:        config.MaxEntries,
		addQuotesForTitle: config.AddQuotesForTitle,
		isTorznab: strings.Contains(strings.ToLower(config.BaseURL), "torznab") ||
			strings.Contains(strings.ToLower(config.BaseURL), "torrent"),
	}
}

// GetProviderType returns the provider type.
func (p *Provider) GetProviderType() apiexternal_v2.IndexerProviderType {
	if p.isTorznab {
		return apiexternal_v2.IndexerTorznab
	}
	return apiexternal_v2.IndexerNewznab
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	if p.isTorznab {
		return "torznab"
	}
	return "newznab"
}

// Search performs a generic search.
func (p *Provider) Search(
	ctx context.Context,
	request apiexternal_v2.IndexerSearchRequest,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	// Build URL parameters following the old buildurlnew logic
	params := url.Values{}

	// Add API key with custom parameter name if configured
	apiKeyParam := "apikey"
	if p.customAPI != "" {
		apiKeyParam = p.customAPI
	}

	params.Set(apiKeyParam, p.apiKey)

	// Determine search type from request or default to "search"
	searchType := "search"
	if request.SearchType != "" {
		searchType = request.SearchType
	}

	// Set search type based on what parameters are provided
	if request.IMDBID != "" {
		searchType = "movie"

		params.Set("imdbid", request.IMDBID)
	} else if request.TVDBID > 0 {
		searchType = "tvsearch"

		params.Set("tvdbid", strconv.Itoa(request.TVDBID))
	}

	// Set the search type parameter
	params.Set("t", searchType)

	// Add query if provided, with optional quotes
	if request.Query != "" {
		query := request.Query
		if p.addQuotesForTitle {
			query = `"` + query + `"`
		}

		params.Set("q", query)
	}

	// Add season/episode for TV searches
	if request.Season != "" {
		params.Set("season", request.Season)
	}

	if request.Episode != "" {
		params.Set("ep", request.Episode)
	}

	// Add categories
	if len(request.Categories) > 0 {
		params.Set("cat", strings.Join(request.Categories, ","))
	} else if len(p.categories) > 0 {
		params.Set("cat", strings.Join(p.categories, ","))
	}

	// Add limits - use provider default if not specified in request
	limit := request.Limit
	if limit == 0 && p.maxEntries > 0 {
		limit = int(p.maxEntries)
	}

	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	if request.Offset > 0 {
		params.Set("offset", strconv.Itoa(request.Offset))
	}

	// Add max age filter - use provider default if not specified in request
	maxAge := request.MaxAge
	if maxAge == 0 && p.maxAge > 0 {
		maxAge = int(p.maxAge)
	}

	if maxAge > 0 {
		params.Set("maxage", strconv.Itoa(maxAge))
	}

	// Add download flag (from old buildurlnew)
	params.Set("dl", "1")

	// Extract raw additional query params if provided
	// This handles AdditionalQueryParams from quality indexer config (format: "&param=value&param2=value2")
	rawParams := ""
	for k, v := range request.Options {
		if k == "_raw_params" || k == "AdditionalQueryParams" {
			rawParams = v
		} else {
			params.Set(k, v)
		}
	}

	return p.MakeSearchRequest(ctx, params, rawParams, ind, qual)
}

// SearchMovies searches for movies by title and year.
func (p *Provider) SearchMovies(
	ctx context.Context,
	query string,
	year int,
	additionalParams string,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	req := apiexternal_v2.IndexerSearchRequest{
		Query:      query,
		SearchType: "movie",
		Categories: p.categories,
	}

	// Add AdditionalQueryParams if provided
	if additionalParams != "" {
		if req.Options == nil {
			req.Options = make(map[string]string)
		}

		req.Options["AdditionalQueryParams"] = additionalParams
	}

	return p.Search(ctx, req, ind, qual)
}

// SearchTV searches for TV shows.
func (p *Provider) SearchTV(
	ctx context.Context,
	query string,
	season, episode int,
	additionalParams string,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	req := apiexternal_v2.IndexerSearchRequest{
		Query:      query,
		SearchType: "tvsearch",
		Categories: p.categories,
	}
	if season > 0 {
		req.Season = strconv.Itoa(season)
	}

	if episode > 0 {
		req.Episode = strconv.Itoa(episode)
	}

	// Add AdditionalQueryParams if provided
	if additionalParams != "" {
		if req.Options == nil {
			req.Options = make(map[string]string)
		}

		req.Options["AdditionalQueryParams"] = additionalParams
	}

	return p.Search(ctx, req, ind, qual)
}

// SearchByIMDB searches by IMDB ID.
func (p *Provider) SearchByIMDB(
	ctx context.Context,
	imdbID string,
	additionalParams string,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	req := apiexternal_v2.IndexerSearchRequest{
		IMDBID:     imdbID,
		SearchType: "movie",
		Categories: p.categories,
	}

	// Add AdditionalQueryParams if provided
	if additionalParams != "" {
		if req.Options == nil {
			req.Options = make(map[string]string)
		}

		req.Options["AdditionalQueryParams"] = additionalParams
	}

	return p.Search(ctx, req, ind, qual)
}

// SearchByTVDB searches by TVDB ID.
func (p *Provider) SearchByTVDB(
	ctx context.Context,
	tvdbID int,
	season, episode int,
	additionalParams string,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	req := apiexternal_v2.IndexerSearchRequest{
		TVDBID:     tvdbID,
		SearchType: "tvsearch",
		Categories: p.categories,
	}
	if season > 0 {
		req.Season = strconv.Itoa(season)
	}

	if episode > 0 {
		req.Episode = strconv.Itoa(episode)
	}

	// Add AdditionalQueryParams if provided
	if additionalParams != "" {
		if req.Options == nil {
			req.Options = make(map[string]string)
		}

		req.Options["AdditionalQueryParams"] = additionalParams
	}

	return p.Search(ctx, req, ind, qual)
}

// GetCapabilities retrieves indexer capabilities.
func (p *Provider) GetCapabilities(
	ctx context.Context,
) (*apiexternal_v2.IndexerCapabilities, error) {
	params := url.Values{
		"apikey": {p.apiKey},
		"t":      {"caps"},
	}

	endpoint := "/api?" + params.Encode()

	// Use BaseClient's MakeRequest with custom XML decoder
	var caps newznabCaps

	err := p.MakeRequest(
		ctx,
		"GET",
		endpoint,
		nil,
		nil,
		func(resp *http.Response) error {
			if decodeErr := xml.NewDecoder(resp.Body).Decode(&caps); decodeErr != nil {
				return decodeErr
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return convertCapabilities(&caps), nil
}

// GetRSSFeed retrieves RSS feed.
// additionalParams can be used to pass AdditionalQueryParams from quality config.
func (p *Provider) GetRSSFeed(
	ctx context.Context,
	categories []string,
	limit int,
	additionalParams string,
	tillid string,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	params := url.Values{}

	// Add API key with custom parameter name if configured
	apiKeyParam := "apikey"
	if p.customAPI != "" {
		apiKeyParam = p.customAPI
	}

	params.Set(apiKeyParam, p.apiKey)

	// Use custom RSS category parameter if configured (default is "t")
	categoryParam := "t"
	if p.customRSSCategory != "" {
		categoryParam = strings.TrimPrefix(p.customRSSCategory, "&")
		categoryParam = strings.TrimSuffix(categoryParam, "=")
	}

	params.Set(categoryParam, "search")

	// Add categories
	if len(categories) > 0 {
		params.Set("cat", strings.Join(categories, ","))
	} else if len(p.categories) > 0 {
		params.Set("cat", strings.Join(p.categories, ","))
	}

	// Add limit - use provider default if not specified
	if limit == 0 && p.maxEntries > 0 {
		limit = int(p.maxEntries)
	}

	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	// Add max age if configured
	if p.maxAge > 0 {
		params.Set("maxage", strconv.Itoa(int(p.maxAge)))
	}

	// Add tillid parameter if provided (only fetch entries newer than this ID)
	if tillid != "" {
		params.Set("tillid", tillid)
	}

	// Add download flag
	params.Set("dl", "1")

	// Extract additional params if provided
	rawParams := ""
	if len(additionalParams) > 0 {
		rawParams = additionalParams
	}

	// Use custom RSS URL or makeSearchRequest for RSS feeds
	return p.makeRSSRequest(ctx, params, rawParams, tillid, ind, qual)
}

// TestConnection validates connectivity.
func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.GetCapabilities(ctx)
	return err
}

// Helper methods

func (p *Provider) MakeSearchRequest(
	ctx context.Context,
	params url.Values,
	rawParams string,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	// Add JSON output parameter if configured
	if p.outputAsJSON {
		params.Set("o", "json")
	}

	// Build URL following the old buildurlnew logic with custom URL support
	var requestURL string
	if p.customURL != "" {
		// Use custom URL path if configured
		requestURL = p.baseURL + p.customURL + "?" + params.Encode()
	} else {
		// Default to /api
		requestURL = p.baseURL + "/api?" + params.Encode()
	}

	// Append raw additional query params if provided (from AdditionalQueryParams)
	// These are already formatted with & prefix like "&extended=1&maxsize=1572864000"
	if rawParams != "" {
		requestURL += rawParams
	}

	// Use shared request execution and parsing logic (no tillid for regular searches)
	arr, err := p.ExecuteRequest(ctx, requestURL, "", ind, qual)
	if errors.Is(err, ErrBroke) {
		return arr, nil
	}
	return arr, err
}

// makeRSSRequest makes an RSS feed request with custom RSS URL support.
func (p *Provider) makeRSSRequest(
	ctx context.Context,
	params url.Values,
	rawParams string,
	tillid string,
	ind *config.IndexersConfig, qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	// Add JSON output parameter if configured
	if p.outputAsJSON {
		params.Set("o", "json")
	}

	// Build URL following the old buildurlnew logic with custom RSS URL support
	var requestURL string
	if p.customRSSURL != "" {
		// Use custom RSS URL path if configured
		// Can be either a relative path or an absolute URL
		if strings.HasPrefix(p.customRSSURL, "http://") ||
			strings.HasPrefix(p.customRSSURL, "https://") {
			// Absolute URL
			requestURL = p.customRSSURL + "?" + params.Encode()
		} else {
			// Relative path
			requestURL = p.baseURL + p.customRSSURL + "?" + params.Encode()
		}
	} else {
		// Default to /rss
		requestURL = p.baseURL + "/rss?" + params.Encode()
	}

	// Append raw additional query params if provided (from AdditionalQueryParams)
	// These are already formatted with & prefix like "&extended=1&maxsize=1572864000"
	if rawParams != "" {
		requestURL += rawParams
	}

	// Make the HTTP request and parse response (reuse existing parsing logic)
	arr, err := p.ExecuteRequest(ctx, requestURL, tillid, ind, qual)
	if errors.Is(err, ErrBroke) {
		return arr, nil
	}
	return arr, err
}

// executeRequest performs the HTTP request and parses the response.
// This is shared by both makeSearchRequest and makeRSSRequest to avoid code duplication.
// tillid parameter is used for RSS feeds to stop parsing at a specific entry ID.
func (p *Provider) ExecuteRequest(
	ctx context.Context,
	requestURL string,
	tillid string,
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	var ret []apiexternal_v2.Nzbwithprio

	err := p.MakeRequest(ctx, "GET", requestURL, nil, nil, func(resp *http.Response) error {
		var err error
		// If JSON output is enabled, try JSON parsing with format fallback
		if p.outputAsJSON {
			ret, err = p.parseJSONResponse(resp.Body, tillid)
			return err
		}

		// Parse XML response
		ret, err = p.parseXMLResponse(resp.Body, tillid, ind, qual)

		return err
	})
	return ret, err
}

func (p *Provider) Download(ctx context.Context,
	requestURL string,
	targetpath string,
	filename string,
) error {
	return p.DownloadWithGracePeriod(ctx, requestURL, targetpath, filename, 120*time.Second)
}

// DownloadWithGracePeriod downloads a file with configurable grace period for rate limiting.
//
// Parameters:
//   - gracePeriod: Maximum time to wait for rate limit slots (e.g., 30s, 120s, 5m)
//
// Downloads use a separate DownloadClient for tracking download statistics separately from searches.
// Higher gracePeriod = higher priority (more willing to wait for execution).
func (p *Provider) DownloadWithGracePeriod(ctx context.Context,
	requestURL string,
	targetpath string,
	filename string,
	gracePeriod time.Duration,
) error {
	// Use DownloadClient's MakeRequestWithGracePeriod with configurable grace period
	// Downloads are rare and critical, so they typically get longer grace periods (e.g., 2 minutes)
	// This allows flexible priority handling for different download scenarios
	return p.DownloadClient.MakeRequestWithGracePeriod(
		ctx,
		"GET",
		requestURL,
		nil,
		nil,
		func(resp *http.Response) error {
			out, createErr := os.Create(filepath.Join(targetpath, filename))
			if createErr != nil {
				return createErr
			}
			defer out.Close()

			// Write the body to file
			_, copyErr := io.Copy(out, resp.Body)
			if copyErr != nil {
				return copyErr
			}

			return out.Sync()
		},
		gracePeriod,
	)
}

// parseXMLResponse parses XML response body into Nzbwithprio slice using RawToken.
// This matches the old processurl implementation's token-based parsing approach.
// If tillid is provided, stops parsing when it encounters an entry with that ID.
func (p *Provider) parseXMLResponse(
	body io.Reader,
	tillid string,
	ind *config.IndexersConfig,
	qual *config.QualityConfig,
) ([]apiexternal_v2.Nzbwithprio, error) {
	results := []apiexternal_v2.Nzbwithprio{}
	decoder := xml.NewDecoder(body)

	decoder.Strict = false

	var (
		currentNZB apiexternal_v2.Nzb
		inItem     bool
		lastfield  int8 // Tracks which field CharData should populate: 1=title, 2=link, 3=guid, 4=size
		nameidx    int
		valueidx   int
	)

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to parse XML: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			lastfield = -1
			switch t.Name.Local {
			case "item":
				// Initialize new NZB item
				currentNZB = apiexternal_v2.Nzb{
					SourceEndpoint: p.GetProviderName(),
					IsTorrent:      p.isTorznab,
					Indexer:        ind,
					Quality:        qual,
				}
				inItem = true

			case "title":
				lastfield = 1

			case "link":
				lastfield = 2

			case "guid":
				lastfield = 3

			case "size":
				lastfield = 4

			case "enclosure", "source":
				// Extract attributes directly from enclosure/source elements
				if inItem {
					for idx := range t.Attr {
						switch t.Attr[idx].Name.Local {
						case "url":
							if currentNZB.DownloadURL == "" {
								currentNZB.DownloadURL = t.Attr[idx].Value
							}

						case "length":
							if currentNZB.Size == 0 {
								if size, err := strconv.ParseInt(t.Attr[idx].Value, 10, 64); err == nil {
									currentNZB.Size = size
								}
							}
						}
					}
				}

			case "attr":
				// Newznab attribute element with name/value pairs
				if inItem {
					nameidx = -1

					valueidx = -1
					for idx := range t.Attr {
						switch t.Attr[idx].Name.Local {
						case "name":
							nameidx = idx
						case "value":
							valueidx = idx
						}
					}

					// Apply the attribute if both name and value are found
					if nameidx != -1 && valueidx != -1 && t.Attr[valueidx].Value != "" {
						switch t.Attr[nameidx].Value {
						case "imdb", "imdbid":
							if currentNZB.IMDBID == "" {
								currentNZB.IMDBID = t.Attr[valueidx].Value
							}

						case "tvdbid":
							if currentNZB.TVDBID == 0 {
								if tvdbID, err := strconv.Atoi(t.Attr[valueidx].Value); err == nil {
									currentNZB.TVDBID = tvdbID
								}
							}

						case "season":
							if currentNZB.Season == "" {
								currentNZB.Season = t.Attr[valueidx].Value
							}

						case "episode":
							if currentNZB.Episode == "" {
								currentNZB.Episode = t.Attr[valueidx].Value
							}
						}
					}
				}
			}

		case xml.CharData:
			// Only process CharData if we're in an item and lastfield is set
			if !inItem || lastfield <= 0 || lastfield >= 5 || len(t) == 0 {
				continue
			}

			data := strings.TrimSpace(string(t))
			if data == "" {
				continue
			}

			// Check if field is already populated; if so, reset lastfield to prevent overwriting
			switch lastfield {
			case 1: // title
				if currentNZB.Title != "" {
					lastfield = 0
					continue
				}

				currentNZB.Title = data

			case 2: // link/url
				if currentNZB.DownloadURL != "" {
					lastfield = 0
					continue
				}

				currentNZB.DownloadURL = data

			case 3: // guid
				if currentNZB.ID != "" {
					lastfield = 0
					continue
				}

				currentNZB.ID = data

			case 4: // size
				if currentNZB.Size != 0 {
					lastfield = 0
					continue
				}

				if size, err := strconv.ParseInt(data, 10, 64); err == nil {
					currentNZB.Size = size
				}
			}

		case xml.EndElement:
			if t.Name.Local == "item" && inItem {
				// Finalize the NZB item
				// Use DownloadURL as ID fallback if ID is empty
				if currentNZB.ID == "" {
					currentNZB.ID = currentNZB.DownloadURL
				}

				// Check if this is the tillid entry - if so, stop parsing here
				// This prevents processing old entries we've already seen
				if tillid != "" && currentNZB.ID == tillid {
					// logger.Logtype(logger.StatusDebug, 1).
					// 	Str("tillid", tillid).
					// 	Int("new_entries", len(results)).
					// 	Msg("Reached tillid entry - stopping RSS parsing")
					return results, ErrBroke
				}

				results = append(results, apiexternal_v2.Nzbwithprio{
					NZB:         currentNZB,
					WantedTitle: currentNZB.Title,
				})

				inItem = false
			}
		}
	}

	return results, nil
}

// parseJSONResponse parses JSON responses with fallback between two formats
//
// Some Newznab implementations return JSON in different formats:
// - Format 1: { "channel": { "item": [...] } }
// - Format 2: { "item": [...] }
//
// This function tries both formats automatically.
func (p *Provider) parseJSONResponse(
	body io.Reader,
	tillid string,
) ([]apiexternal_v2.Nzbwithprio, error) {
	// Read the body once since we may need to try multiple formats
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Try format 1 first (nested channel structure)
	var format1 searchResponseJSON1
	if err := json.Unmarshal(bodyBytes, &format1); err == nil {
		if len(format1.Channel.Item) > 0 {
			return p.convertJSON1Items(format1.Channel.Item, tillid), nil
		}
	}

	// Try format 2 (flat structure)
	var format2 searchResponseJSON2
	if err := json.Unmarshal(bodyBytes, &format2); err != nil {
		return nil, fmt.Errorf("failed to parse JSON in any supported format: %w", err)
	}

	if len(format2.Item) == 0 {
		return []apiexternal_v2.Nzbwithprio{}, nil
	}

	return p.convertJSON2Items(format2.Item, tillid), nil
}

// convertJSON1Items converts JSON format 1 items to Nzbwithprio.
func (p *Provider) convertJSON1Items(
	items []json1Item,
	tillid string,
) []apiexternal_v2.Nzbwithprio {
	results := make([]apiexternal_v2.Nzbwithprio, 0, len(items))

	for _, item := range items {
		nzb := apiexternal_v2.Nzb{
			ID:             item.GUID.Text,
			Title:          item.Title,
			SourceEndpoint: p.GetProviderName(),
			Size:           item.Size,
			IsTorrent:      p.isTorznab,
		}

		// Parse publish date
		if item.PubDate != "" {
			if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
				nzb.PubDate = t
			} else if t, err := time.Parse(time.RFC1123, item.PubDate); err == nil {
				nzb.PubDate = t
			}
		}

		// Extract download URL from enclosure or link
		if item.Enclosure.Attributes.URL != "" {
			nzb.DownloadURL = item.Enclosure.Attributes.URL

			// Parse size from enclosure if not in main item
			if nzb.Size == 0 && item.Enclosure.Attributes.Length != "" {
				if size, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64); err == nil {
					nzb.Size = size
				}
			}
		} else if item.Link != "" {
			nzb.DownloadURL = item.Link
		}

		// Process custom attributes to populate NZB fields
		for _, attr := range item.Attributes {
			switch attr.Attribute.Name {
			case "imdb", "imdbid":
				nzb.IMDBID = attr.Attribute.Value
			case "tvdbid":
				if id, err := strconv.Atoi(attr.Attribute.Value); err == nil {
					nzb.TVDBID = id
				}

			case "season":
				nzb.Season = attr.Attribute.Value
			case "episode":
				nzb.Episode = attr.Attribute.Value
			}
		}

		// Check if this is the tillid entry - if so, stop processing
		if tillid != "" && nzb.ID == tillid {
			logger.Logtype(logger.StatusDebug, 1).
				Str("tillid", tillid).
				Int("new_entries", len(results)).
				Msg("Reached tillid entry in JSON - stopping parsing")

			return results
		}

		results = append(results, apiexternal_v2.Nzbwithprio{
			NZB:         nzb,
			WantedTitle: item.Title,
		})
	}

	return results
}

// convertJSON2Items converts JSON format 2 items to Nzbwithprio.
func (p *Provider) convertJSON2Items(
	items []json2Item,
	tillid string,
) []apiexternal_v2.Nzbwithprio {
	results := make([]apiexternal_v2.Nzbwithprio, 0, len(items))

	for _, item := range items {
		nzb := apiexternal_v2.Nzb{
			ID:             item.GUID.Text,
			Title:          item.Title,
			SourceEndpoint: p.GetProviderName(),
			Size:           item.Size,
			IsTorrent:      p.isTorznab,
		}

		// Parse publish date
		if item.PubDate != "" {
			if t, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
				nzb.PubDate = t
			} else if t, err := time.Parse(time.RFC1123, item.PubDate); err == nil {
				nzb.PubDate = t
			}
		}

		// Extract download URL from enclosure or link
		if item.Enclosure.URL != "" {
			nzb.DownloadURL = item.Enclosure.URL

			// Parse size from enclosure if not in main item
			if nzb.Size == 0 && item.Enclosure.Length != "" {
				if size, err := strconv.ParseInt(item.Enclosure.Length, 10, 64); err == nil {
					nzb.Size = size
				}
			}
		} else if item.Link != "" {
			nzb.DownloadURL = item.Link
		}

		// Process custom attributes from both possible attribute fields to populate NZB fields
		for _, attr := range item.Attributes {
			p.processAttributeForNzb(&nzb, attr.Name, attr.Value)
		}

		for _, attr := range item.Attributes2 {
			p.processAttributeForNzb(&nzb, attr.Name, attr.Value)
		}

		// Check if this is the tillid entry - if so, stop processing
		if tillid != "" && nzb.ID == tillid {
			logger.Logtype(logger.StatusDebug, 1).
				Str("tillid", tillid).
				Int("new_entries", len(results)).
				Msg("Reached tillid entry in JSON format 2 - stopping parsing")

			return results
		}

		results = append(results, apiexternal_v2.Nzbwithprio{
			NZB:         nzb,
			WantedTitle: item.Title,
		})
	}

	return results
}

// processAttributeForNzb processes a custom Newznab attribute and populates the NZB fields.
func (p *Provider) processAttributeForNzb(
	nzb *apiexternal_v2.Nzb,
	name, value string,
) {
	switch name {
	case "imdb", "imdbid":
		nzb.IMDBID = value
	case "tvdbid":
		if id, err := strconv.Atoi(value); err == nil {
			nzb.TVDBID = id
		}

	case "season":
		nzb.Season = value
	case "episode":
		nzb.Episode = value
	case "size":
		if nzb.Size == 0 {
			if size, err := strconv.ParseInt(value, 10, 64); err == nil {
				nzb.Size = size
			}
		}
	}
}

type newznabCaps struct {
	XMLName    xml.Name          `xml:"caps"`
	Server     newznabServer     `xml:"server"`
	Limits     newznabLimits     `xml:"limits"`
	Searching  newznabSearching  `xml:"searching"`
	Categories []newznabCategory `xml:"categories>category"`
}

type newznabServer struct {
	Title   string `xml:"title,attr"`
	Version string `xml:"version,attr"`
}

type newznabLimits struct {
	Max     int `xml:"max,attr"`
	Default int `xml:"default,attr"`
}

type newznabSearching struct {
	Search      newznabSearch `xml:"search"`
	TVSearch    newznabSearch `xml:"tv-search"`
	MovieSearch newznabSearch `xml:"movie-search"`
}

type newznabSearch struct {
	Available string `xml:"available,attr"`
}

type newznabCategory struct {
	ID            string            `xml:"id,attr"`
	Name          string            `xml:"name,attr"`
	Subcategories []newznabCategory `xml:"subcat"`
}

//
// Conversion functions
//

func convertCapabilities(caps *newznabCaps) *apiexternal_v2.IndexerCapabilities {
	searchModes := []string{}
	if caps.Searching.Search.Available == "yes" {
		searchModes = append(searchModes, "search")
	}

	if caps.Searching.TVSearch.Available == "yes" {
		searchModes = append(searchModes, "tvsearch")
	}

	if caps.Searching.MovieSearch.Available == "yes" {
		searchModes = append(searchModes, "movie")
	}

	categories := make([]apiexternal_v2.IndexerCategory, len(caps.Categories))
	for i, cat := range caps.Categories {
		categories[i] = convertCategory(&cat)
	}

	return &apiexternal_v2.IndexerCapabilities{
		ServerTitle:   caps.Server.Title,
		ServerVersion: caps.Server.Version,
		SearchModes:   searchModes,
		Categories:    categories,
		Limits: apiexternal_v2.IndexerLimits{
			MaxResults:   caps.Limits.Max,
			DefaultLimit: caps.Limits.Default,
		},
	}
}

func convertCategory(cat *newznabCategory) apiexternal_v2.IndexerCategory {
	subcats := make([]apiexternal_v2.IndexerCategory, len(cat.Subcategories))
	for i, sub := range cat.Subcategories {
		subcats[i] = convertCategory(&sub)
	}

	return apiexternal_v2.IndexerCategory{
		ID:            cat.ID,
		Name:          cat.Name,
		Subcategories: subcats,
	}
}
