package project1service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// SceneRelease represents a scene/release from the Project1Service API.
type SceneRelease struct {
	ID                int          `json:"id"` // Changed from string to int - API returns numeric ID
	Title             string       `json:"title"`
	Brand             string       `json:"brand"`
	Type              string       `json:"type"`
	Description       string       `json:"description"`
	DateReleased      time.Time    `json:"dateReleased"`
	Actors            []Actor      `json:"actors"`
	Tags              []Tag        `json:"tags"`
	Collections       []Collection `json:"collections"`
	Groups            []Group      `json:"groups"`
	SexualOrientation string       `json:"sexualOrientation"`
}

// Actor represents an actor in a scene.
type Actor struct {
	Name string `json:"name"`
}

// Tag represents a tag/category.
type Tag struct {
	Name string `json:"name"`
}

// Collection represents a collection.
type Collection struct {
	Name string `json:"name"`
}

// Group represents a group.
type Group struct {
	Name string `json:"name"`
}

// APIResponse represents the API response structure.
type APIResponse struct {
	Result []SceneRelease `json:"result"`
}

// Config holds the configuration for the Project1Service scraper.
type Config struct {
	SiteName           string
	StartURL           string
	SiteID             uint
	FilterCollectionID int
	FirstPageDBOnly    bool
	SerieName          string
}

// Scraper handles scraping from Project1Service API.
type Scraper struct {
	config        *Config
	client        *http.Client
	instanceToken string
	dbserieID     uint
}

// NewScraper creates a new Project1Service scraper instance.
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

// getInstanceToken fetches the instance_token cookie from the startURL.
//
// Returns:
//   - error: Any errors during token retrieval
func (s *Scraper) getInstanceToken(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.StartURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch start URL: %w", err)
	}
	defer resp.Body.Close()

	// Extract instance_token from cookies
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "instance_token" {
			s.instanceToken = cookie.Value
			logger.Logtype(logger.StatusDebug, 0).
				Str("site", s.config.SiteName).
				Msg("Retrieved instance token")

			return nil
		}
	}

	return fmt.Errorf("instance_token cookie not found")
}

// buildURL constructs the API URL for fetching releases.
//
// Parameters:
//   - page: Page number (0-indexed)
//
// Returns:
//   - string: Constructed API URL
func (s *Scraper) buildURL(page int) string {
	offset := page * 100
	baseURL := "https://site-api.project1service.com/v2/releases"

	params := url.Values{}
	params.Add("limit", "100")
	params.Add("orderBy", "-dateReleased")
	params.Add("type", "scene")

	if page > 0 {
		params.Add("offset", fmt.Sprintf("%d", offset))
	}

	if s.config.FilterCollectionID != 0 {
		params.Add("collectionId", fmt.Sprintf("%d", s.config.FilterCollectionID))
	}

	return baseURL + "?" + params.Encode()
}

// fetchPage fetches a single page of releases from the API.
//
// Parameters:
//   - ctx: Context for cancellation
//   - page: Page number to fetch
//
// Returns:
//   - []SceneRelease: Array of scene releases
//   - error: Any errors during fetch
func (s *Scraper) fetchPage(ctx context.Context, page int) ([]SceneRelease, error) {
	apiURL := s.buildURL(page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add instance token header
	if s.instanceToken != "" {
		req.Header.Set("Instance", s.instanceToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page %d: %w", page, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d for page %d", resp.StatusCode, page)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return apiResp.Result, nil
}

// cleanTitle cleans the title for URL generation.
//
// Parameters:
//   - title: Original title string
//
// Returns:
//   - string: Cleaned title suitable for URLs
func cleanTitle(title string) string {
	// Replace characters according to PowerShell script logic
	cleaned := strings.ReplaceAll(title, " ", "-")

	cleaned = strings.ReplaceAll(cleaned, "?", "")
	cleaned = strings.ReplaceAll(cleaned, "!", "")
	cleaned = strings.ReplaceAll(cleaned, ":", "")
	cleaned = strings.ReplaceAll(cleaned, "#", "-")
	cleaned = strings.ReplaceAll(cleaned, "&", "and")
	cleaned = strings.ReplaceAll(cleaned, "/", "-")

	// Replace multiple consecutive dashes with single dash
	re := regexp.MustCompile(`-+`)

	cleaned = re.ReplaceAllString(cleaned, "-")

	return cleaned
}

// createEpisode creates or updates an episode in the database.
//
// Parameters:
//   - ctx: Context for database operations
//   - release: Scene release data
//
// Returns:
//   - error: Any errors during episode creation
func (s *Scraper) createEpisode(ctx context.Context, release *SceneRelease) error {
	if release.DateReleased.IsZero() {
		return fmt.Errorf("invalid date for scene: %s", release.Title)
	}

	// Create episode identifier from date (remove first 2 characters like PowerShell script)
	dateStr := release.DateReleased.Format("2006-01-02")
	identifier := dateStr[2:] // Remove "20" prefix

	episode := database.DbserieEpisode{
		Identifier: identifier,
		Title:      release.Title,
		FirstAired: sql.NullTime{
			Time:  release.DateReleased,
			Valid: true,
		},
		DbserieID: s.dbserieID,
		Overview:  release.Description,
	}

	// Check if episode already exists
	existingID := database.Getdatarow[uint](
		false,
		"SELECT id FROM dbserie_episodes WHERE dbserie_id = ? AND identifier = ? AND title = ? LIMIT 1",
		s.dbserieID,
		identifier,
		release.Title,
	)

	if existingID > 0 {
		// Episode exists, update it
		err := database.ExecNErr(`
			UPDATE dbserie_episodes SET
				overview = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			episode.Overview,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update episode '%s': %w", release.Title, err)
		}

		logger.Logtype(logger.StatusDebug, 0).
			Str("title", release.Title).
			Str("identifier", identifier).
			Msg("Updated existing episode")
	} else {
		// Episode doesn't exist, insert new
		err := database.ExecNErr(`
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
			return fmt.Errorf("failed to insert episode '%s': %w", release.Title, err)
		}

		logger.Logtype(logger.StatusInfo, 0).
			Str("title", release.Title).
			Str("identifier", identifier).
			Str("date", dateStr).
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

	// Get instance token
	if err := s.getInstanceToken(ctx); err != nil {
		return 0, fmt.Errorf("failed to get instance token: %w", err)
	}

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

		releases, err := s.fetchPage(ctx, page)
		if err != nil {
			return totalProcessed, fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		if len(releases) == 0 {
			logger.Logtype(logger.StatusInfo, 0).
				Str("site", s.config.SiteName).
				Int("page", page).
				Msg("No more releases found")

			break
		}

		for _, release := range releases {
			if err := s.createEpisode(ctx, &release); err != nil {
				logger.Logtype(logger.StatusError, 0).
					Err(err).
					Str("title", release.Title).
					Msg("Failed to create episode")

				continue
			}

			totalProcessed++
		}

		// Sleep between pages to avoid rate limiting
		if page < maxPages-1 && len(releases) > 0 {
			select {
			case <-ctx.Done():
				return totalProcessed, ctx.Err()
			case <-time.After(2 * time.Second):
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
