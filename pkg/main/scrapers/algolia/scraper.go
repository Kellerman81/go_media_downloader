package algolia

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// AlgoliaHit represents a single scene from Algolia search results.
type AlgoliaHit struct {
	ClipID          int            `json:"clip_id"` // Changed from string to int - API returns numeric ID
	Title           string         `json:"title"`
	URL             string         `json:"url_title"`
	Sitename        string         `json:"sitename"`
	ReleaseDate     string         `json:"release_date"`
	SiteID          int            `json:"site_id"`
	Description     string         `json:"description"`
	Actors          []ActorInfo    `json:"actors"`
	Categories      []CategoryInfo `json:"categories"`
	AvailableOnSite []string       `json:"availableOnSite"`
	ContentTags     []string       `json:"content_tags"`
	Lesbian         interface{}    `json:"lesbian"` // Can be bool or string
	Bisex           interface{}    `json:"bisex"`   // Can be bool or string
	ClipType        string         `json:"clip_type"`
	ClipLength      interface{}    `json:"clip_length"` // Can be string or int
	MovieID         interface{}    `json:"movie_id"`    // Can be string or int
	MovieTitle      string         `json:"movie_title"`
	MovieDesc       string         `json:"movie_desc"`
	Compilation     interface{}    `json:"compilation"` // Can be bool or string
	SerieID         interface{}    `json:"serie_id"`    // Can be string or int
	SerieName       string         `json:"serie_name"`
	StudioID        interface{}    `json:"studio_id"` // Can be string or int
	StudioName      string         `json:"studio_name"`
	NetworkID       interface{}    `json:"network_id"` // Can be string or int
	NetworkName     string         `json:"network_name"`
	URLMovieTitle   string         `json:"url_movie_title"`
}

// ActorInfo represents actor information.
type ActorInfo struct {
	Name string `json:"name"`
}

// CategoryInfo represents category information.
type CategoryInfo struct {
	Name string `json:"name"`
}

// AlgoliaResult represents Algolia search result structure.
type AlgoliaResult struct {
	Hits []AlgoliaHit `json:"hits"`
}

// AlgoliaResponse represents the full Algolia API response.
type AlgoliaResponse struct {
	Results []AlgoliaResult `json:"results"`
}

// Config holds the configuration for the Algolia scraper.
type Config struct {
	SiteName              string
	StartURL              string
	SiteURL               string
	SiteFilterName        string
	SerieFilterName       string
	NetworkFilterName     string
	NetworkSiteFilterName string
	FirstPageDBOnly       bool
	SerieName             string
}

// Scraper handles scraping from Algolia search API.
type Scraper struct {
	config        *Config
	client        *http.Client
	applicationID string
	apiKey        string
	dbserieID     uint
}

// NewScraper creates a new Algolia scraper instance.
//
// Parameters:
//   - cfg: Configuration for the scraper
//
// Returns:
//   - *Scraper: Initialized scraper instance
//   - error: Any initialization errors
func NewScraper(cfg *Config) (*Scraper, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if cfg.StartURL == "" {
		return nil, fmt.Errorf("start URL is required")
	}

	if cfg.SiteURL == "" {
		return nil, fmt.Errorf("site URL is required")
	}

	if cfg.SerieName == "" {
		return nil, fmt.Errorf("series name is required")
	}

	return &Scraper{
		config: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// getOrCreateSerie gets the series ID by name, creating it if it doesn't exist.
//
// Parameters:
//   - ctx: Context for database operations
//
// Returns:
//   - error: Any errors during series lookup or creation
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

// extractAlgoliaCredentials extracts Algolia API credentials from the site HTML.
//
// Parameters:
//   - ctx: Context for HTTP request
//
// Returns:
//   - error: Any errors during credential extraction
func (s *Scraper) extractAlgoliaCredentials(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.StartURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch start URL: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)

	// Try pattern 1: "applicationID":"...","apiKey":"..."
	re1 := regexp.MustCompile(
		`"applicationID":"([a-zA-Z0-9]{1,})","apiKey":"([a-zA-Z0-9=,\.]{1,})"`,
	)
	matches := re1.FindStringSubmatch(content)

	if len(matches) == 3 {
		s.applicationID = matches[1]
		s.apiKey = matches[2]
		logger.Logtype(logger.StatusDebug, 0).
			Str("site", s.config.SiteName).
			Msg("Extracted Algolia credentials (pattern 1)")

		return nil
	}

	// Try pattern 2: "apiKey":"...","applicationID":"..."
	re2 := regexp.MustCompile(
		`"apiKey":"([a-zA-Z0-9=,\.]{1,})","applicationID":"([a-zA-Z0-9]{1,})"`,
	)

	matches = re2.FindStringSubmatch(content)

	if len(matches) == 3 {
		s.apiKey = matches[1]
		s.applicationID = matches[2]
		logger.Logtype(logger.StatusDebug, 0).
			Str("site", s.config.SiteName).
			Msg("Extracted Algolia credentials (pattern 2)")

		return nil
	}

	return fmt.Errorf("algolia API credentials not found for %s", s.config.StartURL)
}

// buildFacetFilter constructs the facet filter string for Algolia query.
//
// Returns:
//   - string: URL-encoded facet filter string
func (s *Scraper) buildFacetFilter() string {
	if s.config.SiteFilterName != "" {
		return url.QueryEscape(fmt.Sprintf(`[["sitename:%s"]]`, s.config.SiteFilterName))
	}

	if s.config.SerieFilterName != "" {
		return url.QueryEscape(fmt.Sprintf(`[["serie_name:%s"]]`, s.config.SerieFilterName))
	}

	if s.config.NetworkFilterName != "" {
		network := strings.ReplaceAll(s.config.NetworkFilterName, " ", "%20")
		return url.QueryEscape(fmt.Sprintf(`[["network.lvl0:%s"]]`, network))
	}

	if s.config.NetworkSiteFilterName != "" {
		parts := strings.Split(s.config.NetworkSiteFilterName, ",")
		if len(parts) == 2 {
			network := strings.TrimSpace(parts[0])
			site := strings.TrimSpace(parts[1])

			network = strings.ReplaceAll(network, " ", "%20")
			site = strings.ReplaceAll(site, " ", "%20")

			return url.QueryEscape(fmt.Sprintf(`[["network.lvl1:%s > %s"]]`, network, site))
		}
	}

	return ""
}

// buildRequestBody constructs the JSON request body for Algolia API.
//
// Parameters:
//   - page: Page number to fetch
//
// Returns:
//   - string: JSON request body
func (s *Scraper) buildRequestBody(page int) string {
	facetFilter := s.buildFacetFilter()

	params := fmt.Sprintf(
		"query=&hitsPerPage=1000&maxValuesPerFacet=2000&tagFilters=&facets=availableOnSite&page=%d",
		page,
	)

	if facetFilter != "" {
		params += "&facetFilters=" + facetFilter
	}

	return fmt.Sprintf(
		`{"requests":[{"indexName":"all_scenes_latest_desc","params":"%s"}]}`,
		params,
	)
}

// fetchPage fetches a single page of scenes from Algolia API.
//
// Parameters:
//   - ctx: Context for cancellation
//   - page: Page number to fetch
//
// Returns:
//   - []AlgoliaHit: Array of scene hits
//   - error: Any errors during fetch
func (s *Scraper) fetchPage(ctx context.Context, page int) ([]AlgoliaHit, error) {
	// Build API URL
	apiURL := fmt.Sprintf(
		"https://tsmkfa364q-dsn.algolia.net/1/indexes/*/queries?x-algolia-application-id=%s&x-algolia-api-key=%s",
		s.applicationID,
		s.apiKey,
	)

	requestBody := s.buildRequestBody(page)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	// Note: Don't set Accept-Encoding manually - Go's HTTP client handles gzip decompression automatically
	req.Header.Set("Accept", "application/json")
	// req.Header.Set("Origin", s.config.SiteURL)
	req.Header.Set("Referer", s.config.SiteURL+"/")
	// req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("x-algolia-api-key", s.apiKey)
	req.Header.Set("x-algolia-application-id", s.applicationID)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page %d: %w", page, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("algolia API returned status %d for page %d", resp.StatusCode, page)
	}

	var apiResp AlgoliaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Results) == 0 {
		return nil, nil
	}

	return apiResp.Results[0].Hits, nil
}

// joinActorNames concatenates actor names into a comma-separated string.
//
// Parameters:
//   - actors: Array of ActorInfo structs
//
// Returns:
//   - string: Comma-separated actor names
func joinActorNames(actors []ActorInfo) string {
	if len(actors) == 0 {
		return ""
	}

	names := make([]string, len(actors))
	for i, actor := range actors {
		names[i] = actor.Name
	}

	return strings.Join(names, ", ")
}

// joinCategoryNames concatenates category names into a comma-separated string.
//
// Parameters:
//   - categories: Array of CategoryInfo structs
//
// Returns:
//   - string: Comma-separated category names
func joinCategoryNames(categories []CategoryInfo) string {
	if len(categories) == 0 {
		return ""
	}

	names := make([]string, len(categories))
	for i, category := range categories {
		names[i] = category.Name
	}

	return strings.Join(names, ", ")
}

// createEpisode creates or updates an episode in the database.
//
// Parameters:
//   - ctx: Context for database operations
//   - hit: Algolia scene hit data
//
// Returns:
//   - error: Any errors during episode creation
func (s *Scraper) createEpisode(ctx context.Context, hit *AlgoliaHit) error {
	if hit.ClipID == 0 {
		return fmt.Errorf("clip_id is zero")
	}

	if hit.ReleaseDate == "" {
		return fmt.Errorf("release date is empty for clip: %s", hit.Title)
	}

	// Parse release date
	releaseDate, err := time.Parse("2006-01-02", hit.ReleaseDate)
	if err != nil {
		return fmt.Errorf(
			"invalid release date '%s' for clip '%s': %w",
			hit.ReleaseDate,
			hit.Title,
			err,
		)
	}

	// Create episode identifier from date (remove first 2 characters like PowerShell script)
	identifier := hit.ReleaseDate[2:] // Remove "20" prefix

	episode := database.DbserieEpisode{
		Identifier: identifier,
		Title:      hit.Title,
		FirstAired: sql.NullTime{
			Time:  releaseDate,
			Valid: true,
		},
		DbserieID: s.dbserieID,
		Overview:  hit.Description,
	}

	// Check if episode already exists
	existingID := database.Getdatarow[uint](
		false,
		"SELECT id FROM dbserie_episodes WHERE dbserie_id = ? AND identifier = ? AND title = ? LIMIT 1",
		s.dbserieID,
		identifier,
		hit.Title,
	)

	if existingID > 0 {
		// Episode exists, update it
		err = database.ExecNErr(`
			UPDATE dbserie_episodes SET
				overview = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			episode.Overview,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update episode '%s': %w", hit.Title, err)
		}

		logger.Logtype(logger.StatusDebug, 0).
			Str("title", hit.Title).
			Str("identifier", identifier).
			Msg("Updated existing episode")
	} else {
		// Episode doesn't exist, insert new
		err = database.ExecNErr(`
			INSERT INTO dbserie_episodes (
				identifier, title, first_aired, dbserie_id, overview, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			episode.Identifier,
			episode.Title,
			episode.FirstAired,
			episode.DbserieID,
			episode.Overview,
		)
		if err != nil {
			return fmt.Errorf("failed to insert episode '%s': %w", hit.Title, err)
		}

		logger.Logtype(logger.StatusInfo, 0).
			Str("title", hit.Title).
			Str("identifier", identifier).
			Str("date", hit.ReleaseDate).
			Msg("Created new episode")
	}

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

	// Extract Algolia credentials
	if err := s.extractAlgoliaCredentials(ctx); err != nil {
		return 0, fmt.Errorf("failed to extract Algolia credentials: %w", err)
	}

	// Decode base64 API key if needed
	// if strings.Contains(s.apiKey, "=") {
	// 	decoded, err := base64.StdEncoding.DecodeString(s.apiKey)
	// 	if err == nil {
	// 		s.apiKey = string(decoded)
	// 		logger.Logtype(logger.StatusDebug, 0).
	// 			Str("site", s.config.SiteName).
	// 			Msg("Decoded base64 API key")
	// 	}
	// }

	totalProcessed := 0
	maxPages := 1000

	if firstpageonly {
		maxPages = 1
	}

	for page := range maxPages {
		logger.Logtype(logger.StatusInfo, 0).
			Str("site", s.config.SiteName).
			Int("page", page).
			Msg("Fetching page")

		hits, err := s.fetchPage(ctx, page)
		if err != nil {
			return totalProcessed, fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		if len(hits) == 0 {
			logger.Logtype(logger.StatusInfo, 0).
				Str("site", s.config.SiteName).
				Int("page", page).
				Msg("No more scenes found")

			break
		}

		processedThisPage := 0
		for _, hit := range hits {
			if err := s.createEpisode(ctx, &hit); err != nil {
				logger.Logtype(logger.StatusError, 0).
					Err(err).
					Str("title", hit.Title).
					Msg("Failed to create episode")

				continue
			}

			processedThisPage++

			totalProcessed++
		}

		// If no episodes were processed this page, stop
		if processedThisPage == 0 {
			break
		}

		// Sleep between pages to avoid rate limiting
		if page < maxPages-1 && len(hits) > 0 {
			select {
			case <-ctx.Done():
				return totalProcessed, ctx.Err()
			case <-time.After(10 * time.Second): // Algolia script uses 10 seconds
			}
		}

		// Break if first page only
		if s.config.FirstPageDBOnly {
			break
		}
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("site", s.config.SiteName).
		Int("total", totalProcessed).
		Msg("Scraping completed")

	return totalProcessed, nil
}
