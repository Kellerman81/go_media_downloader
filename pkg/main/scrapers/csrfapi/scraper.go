package csrfapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/sitethrottle"
	"github.com/goccy/go-json"
)

// Config holds the configuration for the CSRF API scraper.
type Config struct {
	SiteName        string
	StartURL        string
	BaseURL         string
	SerieName       string
	FirstPageDBOnly bool

	// CSRF settings
	CSRFCookieName string // Cookie name containing CSRF token (e.g., "_csrf")
	CSRFHeaderName string // Header name to send token in (e.g., "csrf-token")
	SkipCSRF       bool   // Skip CSRF token extraction entirely, for public APIs needing no auth

	// API settings
	APIURLPattern string // API URL with {page} placeholder
	// PageStartIndex is the first page number. 0 (unset) keeps the historical
	// default of 1; a negative value selects 0-based pages, because a plain 0
	// cannot be told apart from an unset field in existing configs.
	PageStartIndex int
	// PaginationStyle "offset" substitutes {page} with (page-start)*PageIncrement;
	// anything else substitutes the page number itself.
	PaginationStyle string
	PageIncrement   int // Items per page for PaginationStyle "offset"

	// JSON field paths for extracting data
	ResultsArrayPath string // JSON path to array of results (e.g., "galleries")
	TitleField       string // JSON field for title
	DateField        string // JSON field for date
	URLField         string // JSON field for URL path (will be prepended with BaseURL)
	ActorsField      string // JSON field for actors array
	ActorNameField   string // JSON field for actor name within actor object
	RuntimeField     string // Optional: field to check for valid runtime

	// Date parsing
	DateFormat string // Go time format string for parsing dates

	// Optional settings
	WaitSeconds int // Seconds to wait between requests (default: 2)
}

// Scraper implements the CSRF API content scraper.
type Scraper struct {
	config    *Config
	dbserieID uint
	client    *http.Client
	csrfToken string
	cookieJar *cookiejar.Jar
}

const (
	paginationOffset = "offset"

	// statusCSRFExpired is the non-standard status Laravel-style backends
	// return for an expired CSRF token ("419 Page Expired").
	statusCSRFExpired = 419
)

// errDBWrite marks createEpisode failures caused by the database rather than
// by the item's own data.
var errDBWrite = errors.New("database write failed")

// normalizePageStartIndex maps the configured start index to the real one:
// 0 means unset (default 1), negative means 0-based.
func normalizePageStartIndex(v int) int {
	switch {
	case v == 0:
		return 1
	case v < 0:
		return 0
	default:
		return v
	}
}

// isCSRFRejection reports whether status indicates the CSRF token (or the
// session it belongs to) was rejected and a fresh token may help.
func isCSRFRejection(status int) bool {
	return status == http.StatusForbidden || status == statusCSRFExpired
}

// isMissingRuntime reports whether a runtime value is absent or the -1
// "no runtime" marker. JSON numbers decode as float64, so a plain
// comparison against the int -1 never matched.
func isMissingRuntime(v any) bool {
	switch r := v.(type) {
	case nil:
		return true
	case float64:
		return r == -1
	case int:
		return r == -1
	case int64:
		return r == -1
	case json.Number:
		f, err := r.Float64()
		return err == nil && f == -1
	case string:
		return strings.TrimSpace(r) == "-1"
	default:
		return false
	}
}

// NewScraper creates a new CSRF API scraper instance.
//
// Parameters:
//   - cfg: Scraper configuration
//
// Returns:
//   - *Scraper: Initialized scraper instance
//   - error: Configuration validation errors
func NewScraper(cfg *Config) (*Scraper, error) {
	// Validate required configuration
	if cfg.SiteName == "" {
		return nil, errors.New("site_name is required")
	}

	if cfg.StartURL == "" {
		return nil, errors.New("start_url is required")
	}

	if cfg.SerieName == "" {
		return nil, errors.New("serie_name is required")
	}

	if cfg.APIURLPattern == "" {
		return nil, errors.New("api_url_pattern is required")
	}

	// ResultsArrayPath is optional: leave it empty when the API response
	// itself is a top-level JSON array rather than an object wrapping one.

	// Set defaults
	if cfg.BaseURL == "" {
		cfg.BaseURL = cfg.StartURL
	}

	if cfg.CSRFCookieName == "" {
		cfg.CSRFCookieName = "_csrf"
	}

	if cfg.CSRFHeaderName == "" {
		cfg.CSRFHeaderName = "csrf-token"
	}

	cfg.PageStartIndex = normalizePageStartIndex(cfg.PageStartIndex)

	if cfg.PaginationStyle == "" {
		cfg.PaginationStyle = "page"
	}

	if cfg.PaginationStyle == paginationOffset && cfg.PageIncrement <= 0 {
		return nil, errors.New("page_increment is required for pagination_type \"offset\"")
	}

	if cfg.WaitSeconds == 0 {
		cfg.WaitSeconds = 2
	}

	// Create cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Create HTTP client with cookie jar and timeout
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	return &Scraper{
		config:    cfg,
		client:    client,
		cookieJar: jar,
	}, nil
}

// getOrCreateSerie gets the series ID by name, creating it if it doesn't exist.
func (s *Scraper) getOrCreateSerie(_ context.Context) error {
	// First, try to find existing series by name
	existingID := database.Getdatarow[uint](
		false,
		"SELECT id FROM dbseries WHERE seriename = ? LIMIT 1",
		s.config.SerieName,
	)

	if existingID > 0 {
		// Series exists
		s.dbserieID = existingID
		logger.Logtype(logger.StatusDebug, 0).
			Str("site", s.config.SiteName).
			Str("series", s.config.SerieName).
			Uint("id", existingID).
			Msg("Found existing series")

		return nil
	}

	// Series doesn't exist, create it
	lastID, err := database.ExecNid(`
		INSERT INTO dbseries (
			seriename, created_at, updated_at
		) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		s.config.SerieName,
	)
	if err != nil {
		return fmt.Errorf("failed to create series '%s': %w", s.config.SerieName, err)
	}

	s.dbserieID = uint(lastID) //nolint:gosec // safe: value within target type range
	logger.Logtype(logger.StatusInfo, 0).
		Str("site", s.config.SiteName).
		Str("series", s.config.SerieName).
		Uint("id", s.dbserieID).
		Msg("Created new series")

	return nil
}

// extractCSRFToken fetches the start URL and extracts the CSRF token from cookies.
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns:
//   - error: Any errors during token extraction
func (s *Scraper) extractCSRFToken(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.StartURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch start URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Extract CSRF token from cookies
	parsedURL, _ := url.Parse(s.config.StartURL)
	cookies := s.cookieJar.Cookies(parsedURL)

	for _, cookie := range cookies {
		if cookie.Name != s.config.CSRFCookieName {
			continue
		}

		s.csrfToken = cookie.Value
		logger.Logtype(logger.StatusDebug, 0).
			Str("site", s.config.SiteName).
			Int("token_length", len(s.csrfToken)).
			Msg("Extracted CSRF token")

		return nil
	}

	return fmt.Errorf("CSRF cookie '%s' not found", s.config.CSRFCookieName)
}

// fetchAPIPage fetches a page from the JSON API.
//
// Parameters:
//   - ctx: Context for cancellation
//   - pageNum: Page number
//
// Returns:
//   - map[string]interface{}: Parsed JSON response
//   - error: Any errors during fetch or parse
func (s *Scraper) fetchAPIPage(ctx context.Context, pageNum int) (any, error) {
	apiURL := strings.ReplaceAll(
		s.config.APIURLPattern,
		"{page}",
		strconv.Itoa(s.pageValue(pageNum)),
	)

	data, status, err := s.getAPI(ctx, apiURL)
	if err != nil {
		return nil, err
	}

	// The token is fetched once per scrape; a long backfill can outlive it.
	if !s.config.SkipCSRF && isCSRFRejection(status) {
		logger.Logtype(logger.StatusInfo, 0).
			Str("site", s.config.SiteName).
			Int("status", status).
			Msg("API rejected CSRF token, refreshing and retrying once")

		if err := s.extractCSRFToken(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh CSRF token after status %d: %w", status, err)
		}

		data, status, err = s.getAPI(ctx, apiURL)
		if err != nil {
			return nil, err
		}
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", status)
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return result, nil
}

// pageValue returns what {page} is replaced with for the given page number.
func (s *Scraper) pageValue(pageNum int) int {
	if s.config.PaginationStyle == paginationOffset {
		return (pageNum - s.config.PageStartIndex) * s.config.PageIncrement
	}

	return pageNum
}

// getAPI performs one throttled API GET and returns the body and status code.
func (s *Scraper) getAPI(ctx context.Context, apiURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers including CSRF token
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if !s.config.SkipCSRF {
		req.Header.Set(s.config.CSRFHeaderName, s.csrfToken)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-dest", "empty")

	// Enforce the configured delay per site (host), shared across all scraper
	// configs targeting the same site rather than only between this scrape's
	// pages. Mirrors htmlxpath.Scraper.fetchPage.
	sitethrottle.Wait(ctx, req.URL.Host, time.Duration(s.config.WaitSeconds)*time.Second)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch API page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, 0, fmt.Errorf("failed to read response: %w", readErr)
	}

	return data, resp.StatusCode, nil
}

// extractResults extracts the results array from the JSON response. When
// ResultsArrayPath is empty, the response itself is expected to be a
// top-level JSON array rather than an object wrapping one.
//
// Parameters:
//   - data: Parsed JSON response
//
// Returns:
//   - []interface{}: Array of results
func (s *Scraper) extractResults(data any) []any {
	if s.config.ResultsArrayPath == "" {
		results, ok := data.([]any)
		if !ok {
			return nil
		}

		return results
	}

	obj, ok := data.(map[string]any)
	if !ok {
		return nil
	}

	resultsRaw, ok := obj[s.config.ResultsArrayPath]
	if !ok {
		return nil
	}

	results, ok := resultsRaw.([]any)
	if !ok {
		return nil
	}

	return results
}

// extractStringField extracts a string field from a JSON object.
//
// Parameters:
//   - obj: JSON object
//   - fieldName: Field name to extract
//
// Returns:
//   - string: Extracted value
func (*Scraper) extractStringField(obj map[string]any, fieldName string) string {
	if fieldName == "" {
		return ""
	}

	val, ok := obj[fieldName]
	if !ok {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case float64:
		// JSON numbers decode as float64 (e.g. a numeric id used to build a
		// URL, like Bellesa's "id" field). Format without a decimal point.
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}

// extractActors extracts actor names from a JSON array.
//
// Parameters:
//   - obj: JSON object
//
// Returns:
//   - string: Comma-separated actor names
func (s *Scraper) extractActors(obj map[string]any) string {
	if s.config.ActorsField == "" {
		return ""
	}

	actorsRaw, ok := obj[s.config.ActorsField]
	if !ok {
		return ""
	}

	actors, ok := actorsRaw.([]any)
	if !ok {
		return ""
	}

	var names []string
	for i := range actors {
		actor, ok := actors[i].(map[string]any)
		if !ok {
			continue
		}

		name := s.extractStringField(actor, s.config.ActorNameField)
		if name != "" {
			names = append(names, name)
		}
	}

	return logger.JoinStringsSep(names, ", ")
}

// parseDate parses a date string or time object.
//
// Parameters:
//   - dateVal: Date value from JSON (string or time.Time)
//
// Returns:
//   - time.Time: Parsed date
//   - error: Parsing errors
func (s *Scraper) parseDate(dateVal any) (time.Time, error) {
	switch v := dateVal.(type) {
	case time.Time:
		return v, nil
	case float64:
		// JSON numbers decode as float64: a Unix timestamp, in milliseconds
		// when it is too large to be seconds (1e12 s is ~33,000 years out).
		if v > 1e12 || v < -1e12 {
			return time.UnixMilli(int64(v)).UTC(), nil
		}

		return time.Unix(int64(v), 0).UTC(), nil
	case string:
		v = strings.TrimSpace(v)

		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return t, nil
		}

		if s.config.DateFormat != "" {
			if t, err := time.Parse(s.config.DateFormat, v); err == nil {
				return t, nil
			}
		}

		for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t, nil
			}
		}

		return time.Time{}, fmt.Errorf("failed to parse date: %s", v)
	default:
		return time.Time{}, errors.New("date is not a string, number, or time.Time")
	}
}

// createEpisode creates or updates an episode in the database.
//
// Parameters:
//   - ctx: Context for database operations
//   - title: Episode title
//   - url: Episode URL
//   - date: Episode air date
//   - actors: Comma-separated actor names
//
// Returns:
//   - error: Any errors during database operations
//
// resolveURL turns a possibly-relative reference into an absolute URL using base.
// Returns "" for an empty reference and the reference unchanged on parse errors.
func resolveURL(base, ref string) string {
	if ref == "" {
		return ""
	}

	b, err := url.Parse(base)
	if err != nil {
		return ref
	}

	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}

	return b.ResolveReference(r).String()
}

func (s *Scraper) createEpisode(
	_ context.Context,
	title, episodeURL string,
	date time.Time,
	actors string,
) error {
	// Create episode identifier from date (YYMMDD format)
	identifier := date.Format("06-01-02")

	episode := database.DbserieEpisode{
		Identifier: identifier,
		Title:      title,
		FirstAired: sql.NullTime{
			Time:  date,
			Valid: true,
		},
		DbserieID: s.dbserieID,
		Overview:  actors, // Store actors in overview field
		// URLField is a site-relative path (e.g. "/model/.../movie/..."); resolve
		// it against BaseURL so the stored scraper URL is absolute.
		ScraperURL: resolveURL(s.config.BaseURL, episodeURL),
	}

	// Check if episode already exists
	existingID := database.Getdatarow[uint](
		false,
		"SELECT id FROM dbserie_episodes WHERE dbserie_id = ? AND identifier = ? AND title = ? LIMIT 1",
		s.dbserieID,
		identifier,
		title,
	)

	if existingID > 0 {
		// Episode exists, update it. Backfill scraper fields only when empty.
		err := database.ExecNErr(`
			UPDATE dbserie_episodes
			SET first_aired = ?, overview = ?,
				scraper_id = CASE WHEN scraper_id = '' OR scraper_id IS NULL THEN ? ELSE scraper_id END,
				scraper_url = CASE WHEN scraper_url = '' OR scraper_url IS NULL OR scraper_url NOT LIKE 'http%' THEN ? ELSE scraper_url END,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			episode.FirstAired,
			episode.Overview,
			episode.ScraperID,
			episode.ScraperURL,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update episode: %w: %w", errDBWrite, err)
		}

		logger.Logtype(logger.StatusDebug, 0).
			Str("site", s.config.SiteName).
			Str("title", title).
			Str("identifier", identifier).
			Msg("Updated existing episode")

		return nil
	}

	// Create new episode
	err := database.ExecNErr(`
		INSERT INTO dbserie_episodes (
			dbserie_id, identifier, title, first_aired, overview, scraper_id, scraper_url, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		episode.DbserieID,
		episode.Identifier,
		episode.Title,
		episode.FirstAired,
		episode.Overview,
		episode.ScraperID,
		episode.ScraperURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create episode: %w: %w", errDBWrite, err)
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("site", s.config.SiteName).
		Str("title", title).
		Str("identifier", identifier).
		Str("date", date.Format("2006-01-02")).
		Msg("Created new episode")

	return nil
}

// Scrape executes the scraping process.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//
// Returns:
//   - int: Number of episodes processed
//   - error: Any errors during scraping
func (s *Scraper) Scrape(ctx context.Context, firstpageonly bool) (int, error) {
	// Get or create the series first
	if err := s.getOrCreateSerie(ctx); err != nil {
		return 0, fmt.Errorf("failed to get or create series: %w", err)
	}

	// Extract CSRF token, unless this API needs no auth at all
	if !s.config.SkipCSRF {
		if err := s.extractCSRFToken(ctx); err != nil {
			return 0, fmt.Errorf("failed to extract CSRF token: %w", err)
		}
	}

	totalProcessed := 0
	maxPages := 1000

	// firstpageonly (per-call override) and config.FirstPageDBOnly (persisted
	// series setting) both mean "incremental: only the first page", matching
	// algolia/project1service's semantics and the documented first_page_db_only
	// behavior (see scrapers/README.md).
	if firstpageonly || s.config.FirstPageDBOnly {
		maxPages = 1
	}

	// Without a {page} placeholder every request returns the same page.
	if !strings.Contains(s.config.APIURLPattern, "{page}") {
		maxPages = 1
	}

	for page := s.config.PageStartIndex; page < s.config.PageStartIndex+maxPages; page++ {
		// Page spacing is enforced per site in getAPI (sitethrottle).
		if err := ctx.Err(); err != nil {
			return totalProcessed, err
		}

		logger.Logtype(logger.StatusInfo, 0).
			Str("site", s.config.SiteName).
			Int("page", page).
			Msg("Fetching API page")

		data, err := s.fetchAPIPage(ctx, page)
		if err != nil {
			return totalProcessed, fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		// Extract results array
		results := s.extractResults(data)
		if len(results) == 0 {
			logger.Logtype(logger.StatusInfo, 0).
				Str("site", s.config.SiteName).
				Int("page", page).
				Msg("No results found, stopping")

			break
		}

		logger.Logtype(logger.StatusInfo, 0).
			Str("site", s.config.SiteName).
			Int("page", page).
			Int("result_count", len(results)).
			Msg("Processing results")

		dbFailures := 0
		for i := range results {
			result, ok := results[i].(map[string]any)
			if !ok {
				continue
			}

			// Check runtime filter if specified
			if s.config.RuntimeField != "" {
				if isMissingRuntime(result[s.config.RuntimeField]) {
					continue
				}
			}

			// Extract fields
			title := s.extractStringField(result, s.config.TitleField)
			urlPath := s.extractStringField(result, s.config.URLField)
			dateVal := result[s.config.DateField]
			actors := s.extractActors(result)

			// Validate required fields
			if title == "" || dateVal == nil {
				logger.Logtype(logger.StatusDebug, 0).
					Str("site", s.config.SiteName).
					Str("title", title).
					Msg("Skipping result with missing data")

				continue
			}

			// Parse date
			date, err := s.parseDate(dateVal)
			if err != nil {
				logger.Logtype(logger.StatusWarning, 0).
					Err(err).
					Str("site", s.config.SiteName).
					Interface("date_val", dateVal).
					Msg("Failed to parse date")

				continue
			}

			// Create episode
			if err := s.createEpisode(ctx, title, urlPath, date, actors); err != nil {
				logger.Logtype(logger.StatusError, 0).
					Err(err).
					Str("site", s.config.SiteName).
					Str("title", title).
					Msg("Failed to create episode")

				if errors.Is(err, errDBWrite) {
					dbFailures++
				}

				continue
			}

			totalProcessed++
		}

		if dbFailures == len(results) {
			return totalProcessed, fmt.Errorf(
				"all %d episodes on page %d failed to save: %w",
				len(results),
				page,
				errDBWrite,
			)
		}

		// Break if first page only (defense in depth alongside maxPages above,
		// matching algolia/project1service).
		if s.config.FirstPageDBOnly {
			break
		}
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("site", s.config.SiteName).
		Int("total_processed", totalProcessed).
		Msg("Scraping completed")

	return totalProcessed, nil
}
