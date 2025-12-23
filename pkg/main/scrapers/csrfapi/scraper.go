package csrfapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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

	// API settings
	APIURLPattern   string // API URL with {page} placeholder
	PageStartIndex  int    // Starting page index (default: 1)
	PaginationStyle string // "page" or "offset"

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
		return nil, fmt.Errorf("site_name is required")
	}

	if cfg.StartURL == "" {
		return nil, fmt.Errorf("start_url is required")
	}

	if cfg.SerieName == "" {
		return nil, fmt.Errorf("serie_name is required")
	}

	if cfg.APIURLPattern == "" {
		return nil, fmt.Errorf("api_url_pattern is required")
	}

	if cfg.ResultsArrayPath == "" {
		return nil, fmt.Errorf("results_array_path is required")
	}

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

	if cfg.PageStartIndex == 0 {
		cfg.PageStartIndex = 1
	}

	if cfg.PaginationStyle == "" {
		cfg.PaginationStyle = "page"
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
func (s *Scraper) getOrCreateSerie(ctx context.Context) error {
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

	s.dbserieID = uint(lastID)
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
		if cookie.Name == s.config.CSRFCookieName {
			s.csrfToken = cookie.Value
			logger.Logtype(logger.StatusDebug, 0).
				Str("site", s.config.SiteName).
				Int("token_length", len(s.csrfToken)).
				Msg("Extracted CSRF token")

			return nil
		}
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
func (s *Scraper) fetchAPIPage(ctx context.Context, pageNum int) (map[string]interface{}, error) {
	// Build API URL
	url := strings.ReplaceAll(s.config.APIURLPattern, "{page}", fmt.Sprintf("%d", pageNum))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers including CSRF token
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set(s.config.CSRFHeaderName, s.csrfToken)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-dest", "empty")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return result, nil
}

// extractResults extracts the results array from the JSON response.
//
// Parameters:
//   - data: Parsed JSON response
//
// Returns:
//   - []interface{}: Array of results
func (s *Scraper) extractResults(data map[string]interface{}) []interface{} {
	resultsRaw, ok := data[s.config.ResultsArrayPath]
	if !ok {
		return nil
	}

	results, ok := resultsRaw.([]interface{})
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
func (s *Scraper) extractStringField(obj map[string]interface{}, fieldName string) string {
	if fieldName == "" {
		return ""
	}

	val, ok := obj[fieldName]
	if !ok {
		return ""
	}

	strVal, ok := val.(string)
	if ok {
		return strVal
	}

	return ""
}

// extractActors extracts actor names from a JSON array.
//
// Parameters:
//   - obj: JSON object
//
// Returns:
//   - string: Comma-separated actor names
func (s *Scraper) extractActors(obj map[string]interface{}) string {
	if s.config.ActorsField == "" {
		return ""
	}

	actorsRaw, ok := obj[s.config.ActorsField]
	if !ok {
		return ""
	}

	actors, ok := actorsRaw.([]interface{})
	if !ok {
		return ""
	}

	var names []string
	for _, actorRaw := range actors {
		actor, ok := actorRaw.(map[string]interface{})
		if !ok {
			continue
		}

		name := s.extractStringField(actor, s.config.ActorNameField)
		if name != "" {
			names = append(names, name)
		}
	}

	return strings.Join(names, ", ")
}

// parseDate parses a date string or time object.
//
// Parameters:
//   - dateVal: Date value from JSON (string or time.Time)
//
// Returns:
//   - time.Time: Parsed date
//   - error: Parsing errors
func (s *Scraper) parseDate(dateVal interface{}) (time.Time, error) {
	// Check if it's already a time.Time
	if t, ok := dateVal.(time.Time); ok {
		return t, nil
	}

	// Try as string
	dateStr, ok := dateVal.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("date is not a string or time.Time")
	}

	// Try ISO 8601 format first
	t, err := time.Parse(time.RFC3339, dateStr)
	if err == nil {
		return t, nil
	}

	// Try custom format if specified
	if s.config.DateFormat != "" {
		return time.Parse(s.config.DateFormat, dateStr)
	}

	return time.Time{}, fmt.Errorf("failed to parse date: %s", dateStr)
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
func (s *Scraper) createEpisode(
	ctx context.Context,
	title, urlPath string,
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
		// Episode exists, update it
		err := database.ExecNErr(`
			UPDATE dbserie_episodes
			SET first_aired = ?, overview = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			episode.FirstAired,
			episode.Overview,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update episode: %w", err)
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
			dbserie_id, identifier, title, first_aired, overview, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		episode.DbserieID,
		episode.Identifier,
		episode.Title,
		episode.FirstAired,
		episode.Overview,
	)
	if err != nil {
		return fmt.Errorf("failed to create episode: %w", err)
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

	// Extract CSRF token
	if err := s.extractCSRFToken(ctx); err != nil {
		return 0, fmt.Errorf("failed to extract CSRF token: %w", err)
	}

	totalProcessed := 0
	maxPages := 1000

	if firstpageonly {
		maxPages = 2 // Process up to 2 pages for first_page_db_only
	}

	for page := s.config.PageStartIndex; page < s.config.PageStartIndex+maxPages; page++ {
		time.Sleep(time.Duration(s.config.WaitSeconds) * time.Second)
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

		processedInPage := false
		for _, resultRaw := range results {
			result, ok := resultRaw.(map[string]interface{})
			if !ok {
				continue
			}

			// Check runtime filter if specified
			if s.config.RuntimeField != "" {
				runtime := result[s.config.RuntimeField]
				// Skip if runtime is -1 or missing
				if runtime == nil || runtime == -1 || runtime == "-1" {
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

				continue
			}

			totalProcessed++

			processedInPage = true
		}

		// Break if no items were processed (might indicate end)
		if !processedInPage {
			logger.Logtype(logger.StatusInfo, 0).
				Str("site", s.config.SiteName).
				Int("page", page).
				Msg("No valid items processed, stopping")

			break
		}
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("site", s.config.SiteName).
		Int("total_processed", totalProcessed).
		Msg("Scraping completed")

	return totalProcessed, nil
}
