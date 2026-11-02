package newznab

import (
	"bytes"
	"context"
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
	"github.com/goccy/go-json"
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
	OutputAsJSON      bool
	TimeoutSeconds    uint16
	CustomAPI         string // Custom API parameter name (default: "apikey")
	CustomURL         string // Custom API URL path (default: "/api")
	LimiterCalls      int    // Number of API calls allowed within LimiterSeconds (default: 200)
	LimiterSeconds    uint8  // Time window in seconds for rate limiting (default: 3600)
	LimiterCallsDaily int    // Maximum number of API calls allowed per day (default: 2000)
	Enabled           bool   // Whether this indexer is active; skips capability fetch if false
}

// Provider implements the IndexerProvider interface for Newznab/Torznab.
type Provider struct {
	*base.BaseClient
	DownloadClient *base.BaseClient // Separate client for tracking download statistics
	baseURL        string
	apiKey         string
	isTorznab      bool
	outputAsJSON   bool   // When true, request and parse JSON responses instead of XML
	customAPI      string // Custom API parameter name (if not "apikey")
	customURL      string // Custom API URL path (if not "/api")

	// SupportedCategories contains all category IDs (including subcategories) reported by the
	// indexer's caps endpoint. Populated once during NewProvider and used for resolution inference.
	SupportedCategories []string
}

var ErrBroke = errors.New("broke")

// ErrIndexerAPI wraps an error document returned by the indexer.
var ErrIndexerAPI = errors.New("indexer returned error")

// indexerQuotaBlock matches the base client's block for a
// "Request limit reached" HTTP 400 when no retry time is given.
const indexerQuotaBlock = 3 * time.Hour

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
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	p := &Provider{
		BaseClient:     base.NewBaseClient(clientConfig),
		DownloadClient: base.NewBaseClient(downloadClientConfig),
		baseURL:        config.BaseURL,
		apiKey:         config.APIKey,
		outputAsJSON:   config.OutputAsJSON,
		customAPI:      config.CustomAPI,
		customURL:      config.CustomURL,
		isTorznab: logger.ContainsI(config.BaseURL, "torznab") ||
			logger.ContainsI(config.BaseURL, "torrent"),
	}

	// Fetch indexer capabilities and cache supported category IDs for resolution inference.
	// Skip for disabled indexers to avoid unnecessary network requests.
	if config.Enabled {
		if caps, err := p.GetCapabilities(context.Background()); err == nil {
			p.SupportedCategories = flattenCategoryIDs(caps.Categories)
		}
	}

	return p
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

// GetCapabilities retrieves indexer capabilities.
func (p *Provider) GetCapabilities(
	ctx context.Context,
) (*apiexternal_v2.IndexerCapabilities, error) {
	var caps newznabCaps

	err := p.MakeRequest(
		ctx,
		"GET",
		p.capsURL(),
		nil,
		nil,
		func(resp *http.Response) error {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return readErr
			}

			if decodeErr := xml.Unmarshal(body, &caps); decodeErr != nil {
				var apiErr newznabXMLError
				if xml.Unmarshal(body, &apiErr) == nil {
					return p.indexerError(apiErr.Code, apiErr.Description)
				}

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

// CustomEndpoint returns an indexer's API endpoint with its API key
// parameter added, unless the endpoint already carries it or no key is set.
// customURL may be a path ("/newznab/api") or a full URL, optionally with its
// own query string; empty means "/api". customAPI names the key parameter
// (default "apikey"). Shared by capsURL and the legacy search URL builder
// (apiexternal.buildURLNew) so both read the setting the same way.
func CustomEndpoint(customURL, customAPI, apiKey string) string {
	apiKeyParam := "apikey"
	if customAPI != "" {
		apiKeyParam = customAPI
	}

	endpoint := "/api"
	if customURL != "" {
		endpoint = customURL
	}

	if apiKey == "" || strings.Contains(endpoint, apiKeyParam+"=") {
		return endpoint
	}

	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}

	return logger.JoinStrings(
		endpoint,
		sep,
		url.QueryEscape(apiKeyParam),
		"=",
		url.QueryEscape(apiKey),
	)
}

// capsURL builds the t=caps URL for the configured endpoint, see
// CustomEndpoint.
func (p *Provider) capsURL() string {
	endpoint := CustomEndpoint(p.customURL, p.customAPI, p.apiKey)

	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}

	return logger.JoinStrings(endpoint, sep, "t=caps")
}

// TestConnection validates connectivity.
func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.GetCapabilities(ctx)
	return err
}

// ExecuteRequest performs the HTTP request and parses the response.
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
		data       string
	)

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, errors.New(logger.JoinStrings("failed to parse XML: ", err.Error()))
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

			case "category":
				lastfield = 5

			case "error":
				// A Newznab error document arrives with HTTP 200; without this
				// a bad API key or exhausted quota looked like "no results".
				if inItem {
					break
				}

				var code, description string

				for idx := range t.Attr {
					switch t.Attr[idx].Name.Local {
					case "code":
						code = t.Attr[idx].Value
					case "description":
						description = t.Attr[idx].Value
					}
				}

				return nil, p.indexerError(code, description)

			case "enclosure", "source":
				// Extract attributes directly from enclosure/source elements
				if !inItem {
					break
				}

				for idx := range t.Attr {
					switch t.Attr[idx].Name.Local {
					case "url":
						if currentNZB.DownloadURL == "" {
							currentNZB.DownloadURL = t.Attr[idx].Value
						}

					case "length":
						if currentNZB.Size != 0 {
							break
						}

						size, err := strconv.ParseInt(
							t.Attr[idx].Value,
							10,
							64,
						)
						if err == nil {
							currentNZB.Size = size
						}
					}
				}

			case "attr":
				// Newznab attribute element with name/value pairs
				if !inItem {
					break
				}

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
				if nameidx == -1 || valueidx == -1 || t.Attr[valueidx].Value == "" {
					break
				}

				switch t.Attr[nameidx].Value {
				case "imdb", "imdbid":
					if currentNZB.IMDBID == "" {
						currentNZB.IMDBID = normalizeImdbID(t.Attr[valueidx].Value)
					}

				case "tvdbid":
					if currentNZB.TVDBID != 0 {
						break
					}

					tvdbID, err := strconv.Atoi(t.Attr[valueidx].Value)
					if err == nil {
						currentNZB.TVDBID = tvdbID
					}

				case "season":
					if currentNZB.Season == "" {
						currentNZB.Season = t.Attr[valueidx].Value
					}

				case "episode":
					if currentNZB.Episode == "" {
						currentNZB.Episode = t.Attr[valueidx].Value
					}

				case "category":
					currentNZB.Category = moreSpecificCategory(
						currentNZB.Category,
						t.Attr[valueidx].Value,
					)

				case "size", "length":
					if currentNZB.Size != 0 {
						break
					}

					size, err := strconv.ParseInt(
						t.Attr[valueidx].Value,
						10,
						64,
					)
					if err == nil {
						currentNZB.Size = size
					}
				}
			}

		case xml.CharData:
			// Only process CharData if we're in an item and lastfield is set
			if !inItem || lastfield <= 0 || lastfield >= 6 || len(t) == 0 {
				continue
			}

			trimmed := bytes.TrimSpace(t)
			if len(trimmed) == 0 {
				continue
			}

			data = string(trimmed)

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

			case 5: // category - usually "Movies > HD"; only a numeric ID is usable
				currentNZB.Category = moreSpecificCategory(currentNZB.Category, data)
			}

		case xml.EndElement:
			if t.Name.Local != "item" || !inItem {
				break
			}

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
		return nil, errors.New(logger.JoinStrings("failed to read response body: ", err.Error()))
	}

	var apiErr jsonErrorDoc
	if json.Unmarshal(bodyBytes, &apiErr) == nil {
		if code, description, ok := apiErr.details(); ok {
			return nil, p.indexerError(code, description)
		}
	}

	// Try format 1 first (nested channel structure)
	var format1 searchResponseJSON1
	if err := json.Unmarshal(bodyBytes, &format1); err == nil {
		if len(format1.Channel.Item) > 0 {
			return p.convertJSON1Items(format1.Channel.Item, tillid)
		}
	}

	// Try format 2 (flat structure)
	var format2 searchResponseJSON2
	if err := json.Unmarshal(bodyBytes, &format2); err != nil {
		return nil, errors.New(
			logger.JoinStrings("failed to parse JSON in any supported format: ", err.Error()),
		)
	}

	if len(format2.Item) == 0 {
		return []apiexternal_v2.Nzbwithprio{}, nil
	}

	return p.convertJSON2Items(format2.Item, tillid)
}

// convertJSON1Items converts JSON format 1 items to Nzbwithprio.
func (p *Provider) convertJSON1Items(
	items []json1Item,
	tillid string,
) ([]apiexternal_v2.Nzbwithprio, error) {
	results := make([]apiexternal_v2.Nzbwithprio, 0, len(items))

	for i := range items {
		nzb := apiexternal_v2.Nzb{
			ID:             items[i].GUID.Text,
			Title:          items[i].Title,
			SourceEndpoint: p.GetProviderName(),
			Size:           items[i].Size,
			IsTorrent:      p.isTorznab,
		}

		// Parse publish date
		if items[i].PubDate != "" {
			if t, err := time.Parse(time.RFC1123Z, items[i].PubDate); err == nil {
				nzb.PubDate = t
			} else if t, err := time.Parse(time.RFC1123, items[i].PubDate); err == nil {
				nzb.PubDate = t
			}
		}

		// Extract download URL from enclosure or link
		if items[i].Enclosure.Attributes.URL != "" {
			nzb.DownloadURL = items[i].Enclosure.Attributes.URL

			// Parse size from enclosure if not in main item
			if nzb.Size == 0 && items[i].Enclosure.Attributes.Length != "" {
				if size, err := strconv.ParseInt(
					items[i].Enclosure.Attributes.Length,
					10,
					64,
				); err == nil {
					nzb.Size = size
				}
			}
		} else if items[i].Link != "" {
			nzb.DownloadURL = items[i].Link
		}

		// Process custom attributes to populate NZB fields
		for j := range items[i].Attributes {
			switch items[i].Attributes[j].Attribute.Name {
			case "imdb", "imdbid":
				nzb.IMDBID = normalizeImdbID(items[i].Attributes[j].Attribute.Value)
			case "tvdbid":
				if id, err := strconv.Atoi(items[i].Attributes[j].Attribute.Value); err == nil {
					nzb.TVDBID = id
				}

			case "season":
				nzb.Season = items[i].Attributes[j].Attribute.Value
			case "episode":
				nzb.Episode = items[i].Attributes[j].Attribute.Value
			case "category":
				nzb.Category = moreSpecificCategory(
					nzb.Category,
					items[i].Attributes[j].Attribute.Value,
				)
			}
		}

		// Check if this is the tillid entry - if so, stop processing
		if tillid != "" && nzb.ID == tillid {
			logger.Logtype(logger.StatusDebug, 1).
				Str("tillid", tillid).
				Int("new_entries", len(results)).
				Msg("Reached tillid entry in JSON - stopping parsing")

			return results, ErrBroke
		}

		results = append(results, apiexternal_v2.Nzbwithprio{
			NZB:         nzb,
			WantedTitle: items[i].Title,
		})
	}

	return results, nil
}

// convertJSON2Items converts JSON format 2 items to Nzbwithprio.
func (p *Provider) convertJSON2Items(
	items []json2Item,
	tillid string,
) ([]apiexternal_v2.Nzbwithprio, error) {
	results := make([]apiexternal_v2.Nzbwithprio, 0, len(items))

	for i := range items {
		nzb := apiexternal_v2.Nzb{
			ID:             items[i].GUID.Text,
			Title:          items[i].Title,
			SourceEndpoint: p.GetProviderName(),
			Size:           items[i].Size,
			IsTorrent:      p.isTorznab,
		}

		// Parse publish date
		if items[i].PubDate != "" {
			if t, err := time.Parse(time.RFC1123Z, items[i].PubDate); err == nil {
				nzb.PubDate = t
			} else if t, err := time.Parse(time.RFC1123, items[i].PubDate); err == nil {
				nzb.PubDate = t
			}
		}

		// Extract download URL from enclosure or link
		if items[i].Enclosure.URL != "" {
			nzb.DownloadURL = items[i].Enclosure.URL

			// Parse size from enclosure if not in main item
			if nzb.Size == 0 && items[i].Enclosure.Length != "" {
				if size, err := strconv.ParseInt(items[i].Enclosure.Length, 10, 64); err == nil {
					nzb.Size = size
				}
			}
		} else if items[i].Link != "" {
			nzb.DownloadURL = items[i].Link
		}

		// Process custom attributes from both possible attribute fields to populate NZB fields
		for j := range items[i].Attributes {
			p.processAttributeForNzb(
				&nzb,
				items[i].Attributes[j].Name,
				items[i].Attributes[j].Value,
			)
		}

		for j := range items[i].Attributes2 {
			p.processAttributeForNzb(
				&nzb,
				items[i].Attributes2[j].Name,
				items[i].Attributes2[j].Value,
			)
		}

		// Check if this is the tillid entry - if so, stop processing
		if tillid != "" && nzb.ID == tillid {
			logger.Logtype(logger.StatusDebug, 1).
				Str("tillid", tillid).
				Int("new_entries", len(results)).
				Msg("Reached tillid entry in JSON format 2 - stopping parsing")

			return results, ErrBroke
		}

		results = append(results, apiexternal_v2.Nzbwithprio{
			NZB:         nzb,
			WantedTitle: items[i].Title,
		})
	}

	return results, nil
}

// processAttributeForNzb processes a custom Newznab attribute and populates the NZB fields.
func (*Provider) processAttributeForNzb(
	nzb *apiexternal_v2.Nzb,
	name, value string,
) {
	switch name {
	case "imdb", "imdbid":
		nzb.IMDBID = normalizeImdbID(value)
	case "tvdbid":
		if id, err := strconv.Atoi(value); err == nil {
			nzb.TVDBID = id
		}

	case "season":
		nzb.Season = value
	case "episode":
		nzb.Episode = value
	case "category":
		nzb.Category = moreSpecificCategory(nzb.Category, value)

	case "size":
		if nzb.Size != 0 {
			break
		}

		size, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			nzb.Size = size
		}
	}
}

// moreSpecificCategory picks the category to keep for an item. Items list the
// parent (2000) and the sub-category (2040), and some send the display text
// ("Movies > HD"); consumers such as searcher.resolutionFromCategory only
// understand numeric sub-categories, so a numeric non-x000 ID wins.
func moreSpecificCategory(current, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if !isDigits(candidate) {
		return current
	}

	if !isDigits(current) {
		return candidate
	}

	if strings.HasSuffix(current, "000") && !strings.HasSuffix(candidate, "000") {
		return candidate
	}

	return current
}

// normalizeImdbID adds the "tt" prefix indexers omit (imdb="0133093"); the
// searcher only treats "tt"-prefixed IDs as IMDb IDs. All-zero placeholders
// mean "unknown".
func normalizeImdbID(value string) string {
	value = strings.TrimSpace(value)
	if !isDigits(value) {
		return value
	}

	if strings.Trim(value, "0") == "" {
		return ""
	}

	return logger.AddImdbPrefix(value)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

// indexerError converts a Newznab error document into an error and, for
// quota errors (500 request limit, 501 download limit), blocks the client the
// same way an HTTP 429 would.
func (p *Provider) indexerError(code, description string) error {
	if code == "500" || code == "501" || logger.ContainsI(description, "limit reached") {
		p.MarkServerRateLimited(description, indexerQuotaBlock)
	}

	return fmt.Errorf("%w: code %s: %s", ErrIndexerAPI, code, description)
}

type newznabXMLError struct {
	XMLName     xml.Name `xml:"error"`
	Code        string   `xml:"code,attr"`
	Description string   `xml:"description,attr"`
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

// flattenCategoryIDs recursively collects all category IDs (including subcategories)
// from an IndexerCapabilities category list into a flat string slice.
func flattenCategoryIDs(cats []apiexternal_v2.IndexerCategory) []string {
	ids := make([]string, 0, len(cats)*3)

	var collect func(c []apiexternal_v2.IndexerCategory)

	collect = func(c []apiexternal_v2.IndexerCategory) {
		for i := range c {
			ids = append(ids, c[i].ID)
			collect(c[i].Subcategories)
		}
	}
	collect(cats)

	return ids
}
