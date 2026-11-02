package algolia

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/goccy/go-json"
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
	Lesbian         any            `json:"lesbian"` // Can be bool or string
	Bisex           any            `json:"bisex"`   // Can be bool or string
	ClipType        string         `json:"clip_type"`
	ClipLength      any            `json:"clip_length"` // Can be string or int
	MovieID         any            `json:"movie_id"`    // Can be string or int
	MovieTitle      string         `json:"movie_title"`
	MovieDesc       string         `json:"movie_desc"`
	Compilation     any            `json:"compilation"` // Can be bool or string
	SerieID         any            `json:"serie_id"`    // Can be string or int
	SerieName       string         `json:"serie_name"`
	StudioID        any            `json:"studio_id"` // Can be string or int
	StudioName      string         `json:"studio_name"`
	NetworkID       any            `json:"network_id"` // Can be string or int
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
	Hits   []AlgoliaHit `json:"hits"`
	NbHits int          `json:"nbHits"`
}

// hitsPerPage must stay well below Algolia's default paginationLimitedTo
// (1000): with hitsPerPage=1000 page 0 already exhausts the window and every
// later page comes back empty, silently truncating large series.
const (
	hitsPerPage         = 100
	paginationHitsLimit = 1000
)

// errDBWrite marks createEpisode failures caused by the database rather than
// by the hit's own data, so Scrape can tell "every write failed" apart from
// "every hit was unusable".
var errDBWrite = errors.New("database write failed")

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
		return nil, errors.New("config cannot be nil")
	}

	if cfg.StartURL == "" {
		return nil, errors.New("start URL is required")
	}

	if cfg.SiteURL == "" {
		return nil, errors.New("site URL is required")
	}

	if cfg.SerieName == "" {
		return nil, errors.New("series name is required")
	}

	if (&Scraper{config: cfg}).buildFacetFilter() == "" {
		if cfg.NetworkSiteFilterName != "" {
			return nil, fmt.Errorf(
				"network_site_filter_name %q must have the form \"network,site\"",
				cfg.NetworkSiteFilterName,
			)
		}

		// No filter is documented as allowed (the extracted API key may already
		// be restricted to the site), but on an unrestricted key it pulls the
		// whole index into this one series - make that visible.
		logger.Logtype(logger.StatusWarning, 0).
			Str("site", cfg.SiteName).
			Msg("Algolia scraper has no facet filter configured; every hit the site's API key can see will be imported")
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
	re1 := database.GetCachedRegexp(
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
	re2 := database.GetCachedRegexp(
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

// warnIfMultipleFacetFilters logs a warning when more than one facet filter
// field is configured. buildFacetFilter only ever applies the first non-empty
// field in its SiteFilterName > SerieFilterName > NetworkFilterName >
// NetworkSiteFilterName priority order, so an accidental second value would
// otherwise be silently dropped with no indication anything was ignored.
func (s *Scraper) warnIfMultipleFacetFilters() {
	fields := []struct {
		name  string
		value string
	}{
		{"site_filter_name", s.config.SiteFilterName},
		{"serie_filter_name", s.config.SerieFilterName},
		{"network_filter_name", s.config.NetworkFilterName},
		{"network_site_filter_name", s.config.NetworkSiteFilterName},
	}

	var configured []string

	for _, f := range fields {
		if f.value != "" {
			configured = append(configured, f.name)
		}
	}

	if len(configured) > 1 {
		logger.Logtype(logger.StatusWarning, 0).
			Str("site", s.config.SiteName).
			Str("used", configured[0]).
			Str("ignored", logger.JoinStringsSep(configured[1:], ", ")).
			Msg("Multiple Algolia facet filter fields configured; only the first is applied")
	}
}

// buildFacetFilter constructs the facet filter string for Algolia query.
//
// Returns:
//   - string: URL-encoded facet filter string
func (s *Scraper) buildFacetFilter() string {
	s.warnIfMultipleFacetFilters()

	if s.config.SiteFilterName != "" {
		return url.QueryEscape(fmt.Sprintf(`[["sitename:%s"]]`, s.config.SiteFilterName))
	}

	if s.config.SerieFilterName != "" {
		return url.QueryEscape(fmt.Sprintf(`[["serie_name:%s"]]`, s.config.SerieFilterName))
	}

	if s.config.NetworkFilterName != "" {
		// url.QueryEscape below already encodes spaces correctly - a manual
		// " " -> "%20" substitution before it double-encodes the "%" into
		// "%25", producing the literal text "%2520" instead of a space once
		// Algolia URL-decodes the query, so a network name containing a
		// space (e.g. "Sinn Sage") never matched anything.
		return url.QueryEscape(fmt.Sprintf(`[["network.lvl0:%s"]]`, s.config.NetworkFilterName))
	}

	if s.config.NetworkSiteFilterName != "" {
		// Cut on the first comma only, so a site name containing a comma survives.
		network, site, ok := strings.Cut(s.config.NetworkSiteFilterName, ",")
		network = strings.TrimSpace(network)
		site = strings.TrimSpace(site)

		if ok && network != "" && site != "" {
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
		"query=&hitsPerPage=%d&maxValuesPerFacet=2000&tagFilters=&facets=availableOnSite&page=%d",
		hitsPerPage,
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
//   - int: Total number of hits matching the query (nbHits)
//   - error: Any errors during fetch
func (s *Scraper) fetchPage(ctx context.Context, page int) ([]AlgoliaHit, int, error) {
	// Build API URL
	apiURL := fmt.Sprintf(
		"https://tsmkfa364q-dsn.algolia.net/1/indexes/*/queries?x-algolia-application-id=%s&x-algolia-api-key=%s",
		url.QueryEscape(s.applicationID),
		url.QueryEscape(s.apiKey),
	)

	requestBody := s.buildRequestBody(page)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(requestBody),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
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
		return nil, 0, fmt.Errorf("failed to fetch page %d: %w", page, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("algolia API returned status %d for page %d", resp.StatusCode, page)
	}

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, 0, fmt.Errorf("failed to read response: %w", readErr)
	}

	var apiResp AlgoliaResponse
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Results) == 0 {
		return nil, 0, nil
	}

	return apiResp.Results[0].Hits, apiResp.Results[0].NbHits, nil
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

	return logger.JoinStringsSep(names, ", ")
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

	return logger.JoinStringsSep(names, ", ")
}

// createEpisode creates or updates an episode in the database.
//
// Parameters:
//   - ctx: Context for database operations
//   - hit: Algolia scene hit data
//
// Returns:
//   - error: Any errors during episode creation
func (s *Scraper) createEpisode(_ context.Context, hit *AlgoliaHit) error {
	if hit.ClipID == 0 {
		return errors.New("clip_id is zero")
	}

	if hit.ReleaseDate == "" {
		return fmt.Errorf("release date is empty for clip: %s", hit.Title)
	}

	// Matches htmlxpath/csrfapi's guard: the existing-row lookup below keys
	// on (dbserie_id, identifier, title), so a blank title would make every
	// hit on the same release date collide into one DB row, each silently
	// overwriting the last one's overview/scraper_url.
	if hit.Title == "" {
		return fmt.Errorf("title is empty for clip_id %d", hit.ClipID)
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

	// Scraper provenance for later re-scraping: the source clip id and the item
	// page URL (built the same way as the original scraping script).
	scraperID := strconv.Itoa(hit.ClipID)
	scraperURL := s.config.SiteURL + "/en/video/" + hit.Sitename + "/" + hit.URL + "/" + scraperID

	episode := database.DbserieEpisode{
		Identifier: identifier,
		Title:      hit.Title,
		FirstAired: sql.NullTime{
			Time:  releaseDate,
			Valid: true,
		},
		DbserieID:  s.dbserieID,
		Overview:   hit.Description,
		ScraperID:  scraperID,
		ScraperURL: scraperURL,
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
		// Episode exists, update it. Backfill scraper fields only when empty.
		err = database.ExecNErr(`
			UPDATE dbserie_episodes SET
				overview = ?,
				scraper_id = CASE WHEN scraper_id = '' OR scraper_id IS NULL THEN ? ELSE scraper_id END,
				scraper_url = CASE WHEN scraper_url = '' OR scraper_url IS NULL THEN ? ELSE scraper_url END,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			episode.Overview,
			episode.ScraperID,
			episode.ScraperURL,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update episode '%s': %w: %w", hit.Title, errDBWrite, err)
		}

		logger.Logtype(logger.StatusDebug, 0).
			Str("title", hit.Title).
			Str("identifier", identifier).
			Msg("Updated existing episode")
	} else {
		// Episode doesn't exist, insert new
		err = database.ExecNErr(`
			INSERT INTO dbserie_episodes (
				identifier, title, first_aired, dbserie_id, overview, scraper_id, scraper_url, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			episode.Identifier,
			episode.Title,
			episode.FirstAired,
			episode.DbserieID,
			episode.Overview,
			episode.ScraperID,
			episode.ScraperURL,
		)
		if err != nil {
			return fmt.Errorf("failed to insert episode '%s': %w: %w", hit.Title, errDBWrite, err)
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

		hits, nbHits, err := s.fetchPage(ctx, page)
		if err != nil {
			return totalProcessed, fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		if page == 0 && nbHits > paginationHitsLimit && maxPages > 1 {
			logger.Logtype(logger.StatusWarning, 0).
				Str("site", s.config.SiteName).
				Int("nb_hits", nbHits).
				Int("limit", paginationHitsLimit).
				Msg("Algolia query matches more hits than the index pagination limit; older scenes beyond the limit will not be scraped")
		}

		if len(hits) == 0 {
			logger.Logtype(logger.StatusInfo, 0).
				Str("site", s.config.SiteName).
				Int("page", page).
				Msg("No more scenes found")

			break
		}

		dbFailures := 0
		for i := range hits {
			if err := s.createEpisode(ctx, &hits[i]); err != nil {
				logger.Logtype(logger.StatusError, 0).
					Err(err).
					Str("title", hits[i].Title).
					Msg("Failed to create episode")

				if errors.Is(err, errDBWrite) {
					dbFailures++
				}

				continue
			}

			totalProcessed++
		}

		if dbFailures == len(hits) {
			return totalProcessed, fmt.Errorf(
				"all %d episodes on page %d failed to save: %w",
				len(hits),
				page,
				errDBWrite,
			)
		}

		// Break if first page only
		if s.config.FirstPageDBOnly {
			break
		}

		// Sleep between pages to avoid rate limiting
		if page < maxPages-1 {
			select {
			case <-ctx.Done():
				return totalProcessed, ctx.Err()
			case <-time.After(10 * time.Second): // Algolia script uses 10 seconds
			}
		}
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("site", s.config.SiteName).
		Int("total", totalProcessed).
		Msg("Scraping completed")

	return totalProcessed, nil
}
