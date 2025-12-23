package htmlxpath

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// Config holds the configuration for the HTML/XPath scraper.
type Config struct {
	SiteName        string
	StartURL        string
	BaseURL         string
	SerieName       string
	FirstPageDBOnly bool

	// XPath selectors for extracting data
	SceneNodeXPath string // XPath to select each scene/video container
	TitleXPath     string // XPath relative to scene node for title
	URLXPath       string // XPath relative to scene node for URL
	DateXPath      string // XPath relative to scene node for date
	ActorsXPath    string // XPath relative to scene node for actors (can select multiple)
	TitleAttribute string // Optional: HTML attribute to extract title from (default: innerText)
	URLAttribute   string // Optional: HTML attribute to extract URL from (default: href)

	// Pagination settings
	PaginationType string // "sequential" (page 1, 2, 3) or "offset" (0, 12, 24)
	PageIncrement  int    // For offset pagination: how much to increment per page
	PageURLPattern string // URL pattern with {page} placeholder

	// Date parsing
	DateFormat string // Go time format string (e.g., "Jan 2, 2006" or "2006-01-02")

	// Optional settings
	WaitSeconds int // Seconds to wait between requests (default: 2)
}

// Scraper implements the HTML/XPath content scraper.
type Scraper struct {
	config    *Config
	dbserieID uint
	client    *http.Client
}

// NewScraper creates a new HTML/XPath scraper instance.
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

	if cfg.SceneNodeXPath == "" {
		return nil, fmt.Errorf("scene_node_xpath is required")
	}

	if cfg.TitleXPath == "" {
		return nil, fmt.Errorf("title_xpath is required")
	}

	if cfg.DateXPath == "" {
		return nil, fmt.Errorf("date_xpath is required")
	}

	// Set defaults
	if cfg.BaseURL == "" {
		cfg.BaseURL = cfg.StartURL
	}

	if cfg.PaginationType == "" {
		cfg.PaginationType = "sequential"
	}

	if cfg.PageIncrement == 0 && cfg.PaginationType == "offset" {
		cfg.PageIncrement = 12
	}

	if cfg.WaitSeconds == 0 {
		cfg.WaitSeconds = 2
	}

	if cfg.URLAttribute == "" {
		cfg.URLAttribute = "href"
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &Scraper{
		config: cfg,
		client: client,
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

// fetchPage fetches and parses an HTML page.
//
// Parameters:
//   - ctx: Context for cancellation
//   - pageNum: Page number (0-indexed)
//
// Returns:
//   - *html.Node: Parsed HTML document
//   - error: Any errors during fetch or parse
func (s *Scraper) fetchPage(ctx context.Context, pageNum int) (*html.Node, error) {
	// Build URL based on pagination type
	var url string
	if s.config.PageURLPattern != "" {
		// Use custom pattern
		pageValue := pageNum
		if s.config.PaginationType == "offset" {
			pageValue = pageNum * s.config.PageIncrement
		} else {
			pageValue = pageNum + 1 // Convert 0-indexed to 1-indexed
		}

		url = strings.ReplaceAll(s.config.PageURLPattern, "{page}", fmt.Sprintf("%d", pageValue))
	} else {
		url = s.config.StartURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

// extractText extracts text from a node using XPath.
//
// Parameters:
//   - node: Base HTML node
//   - xpath: XPath expression
//   - attribute: Optional attribute to extract instead of text
//
// Returns:
//   - string: Extracted text value
func (s *Scraper) extractText(node *html.Node, xpath, attribute string) string {
	targetNode := htmlquery.FindOne(node, xpath)
	if targetNode == nil {
		return ""
	}

	if attribute != "" {
		for _, attr := range targetNode.Attr {
			if attr.Key == attribute {
				return strings.TrimSpace(attr.Val)
			}
		}

		return ""
	}

	return strings.TrimSpace(htmlquery.InnerText(targetNode))
}

// extractActors extracts actor names from nodes using XPath.
//
// Parameters:
//   - node: Base HTML node
//   - xpath: XPath expression
//
// Returns:
//   - string: Comma-separated actor names
func (s *Scraper) extractActors(node *html.Node, xpath string) string {
	if xpath == "" {
		return ""
	}

	actorNodes := htmlquery.Find(node, xpath)
	if len(actorNodes) == 0 {
		return ""
	}

	var actors []string
	for _, actorNode := range actorNodes {
		actor := strings.TrimSpace(htmlquery.InnerText(actorNode))
		if actor != "" {
			actors = append(actors, actor)
		}
	}

	return strings.Join(actors, ", ")
}

// parseDate parses a date string using the configured format.
//
// Parameters:
//   - dateStr: Date string to parse
//
// Returns:
//   - time.Time: Parsed date
//   - error: Parsing errors
func (s *Scraper) parseDate(dateStr string) (time.Time, error) {
	if s.config.DateFormat == "" {
		// Try to parse as YYYY-MM-DD
		return time.Parse("2006-01-02", dateStr)
	}

	return time.Parse(s.config.DateFormat, dateStr)
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
	title, url string,
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

	totalProcessed := 0
	maxPages := 1000

	if firstpageonly {
		maxPages = 2 // Process up to 2 pages for first_page_db_only
	}

	for page := range maxPages {
		time.Sleep(time.Duration(s.config.WaitSeconds) * time.Second)
		logger.Logtype(logger.StatusInfo, 0).
			Str("site", s.config.SiteName).
			Int("page", page).
			Msg("Fetching page")

		doc, err := s.fetchPage(ctx, page)
		if err != nil {
			return totalProcessed, fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		// Find all scene nodes
		sceneNodes := htmlquery.Find(doc, s.config.SceneNodeXPath)
		if len(sceneNodes) == 0 {
			logger.Logtype(logger.StatusInfo, 0).
				Str("site", s.config.SiteName).
				Int("page", page).
				Msg("No scenes found, stopping")

			break
		}

		logger.Logtype(logger.StatusInfo, 0).
			Str("site", s.config.SiteName).
			Int("page", page).
			Int("scene_count", len(sceneNodes)).
			Msg("Processing scenes")

		for _, sceneNode := range sceneNodes {
			// Extract data from scene node
			title := s.extractText(sceneNode, s.config.TitleXPath, s.config.TitleAttribute)
			url := s.extractText(sceneNode, s.config.URLXPath, s.config.URLAttribute)
			dateStr := s.extractText(sceneNode, s.config.DateXPath, "")
			actors := s.extractActors(sceneNode, s.config.ActorsXPath)

			// Validate required fields
			if title == "" || dateStr == "" {
				logger.Logtype(logger.StatusDebug, 0).
					Str("site", s.config.SiteName).
					Str("title", title).
					Str("date", dateStr).
					Msg("Skipping scene with missing data")

				continue
			}

			// Parse date
			date, err := s.parseDate(dateStr)
			if err != nil {
				logger.Logtype(logger.StatusWarning, 0).
					Err(err).
					Str("site", s.config.SiteName).
					Str("date_str", dateStr).
					Msg("Failed to parse date")

				continue
			}

			// Create episode
			if err := s.createEpisode(ctx, title, url, date, actors); err != nil {
				logger.Logtype(logger.StatusError, 0).
					Err(err).
					Str("site", s.config.SiteName).
					Str("title", title).
					Msg("Failed to create episode")

				continue
			}

			totalProcessed++
		}
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("site", s.config.SiteName).
		Int("total_processed", totalProcessed).
		Msg("Scraping completed")

	return totalProcessed, nil
}
