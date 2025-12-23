package scrapers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/algolia"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/csrfapi"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/htmlxpath"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/project1service"
	"github.com/pelletier/go-toml/v2"
)

// ScraperType represents the type of scraper.
type ScraperType string

const (
	// ScraperTypeProject1Service is the Project1Service API scraper type.
	ScraperTypeProject1Service ScraperType = "project1service"
	// ScraperTypeAlgolia is the Algolia search API scraper type.
	ScraperTypeAlgolia ScraperType = "algolia"
	// ScraperTypeHTMLXPath is the HTML/XPath scraper type.
	ScraperTypeHTMLXPath ScraperType = "htmlxpath"
	// ScraperTypeCSRFAPI is the CSRF API scraper type.
	ScraperTypeCSRFAPI ScraperType = "csrfapi"
)

// Scraper is the common interface that all scrapers must implement.
type Scraper interface {
	// Scrape executes the scraping process and returns the number of episodes processed.
	Scrape(ctx context.Context, firstpageonly bool) (int, error)
}

// Service provides centralized scraper operations.
type Service struct {
	config         *config.MainConfig
	scraperMutexes map[ScraperType]*sync.Mutex
	mutexLock      sync.Mutex
}

// NewService creates a new scraper service instance.
//
// Parameters:
//   - cfg: Main application configuration
//
// Returns:
//   - *Service: Initialized service instance
func NewService(cfg *config.MainConfig) *Service {
	return &Service{
		config:         cfg,
		scraperMutexes: make(map[ScraperType]*sync.Mutex),
	}
}

// getScraperMutex returns the mutex for a specific scraper type, creating it if necessary.
// This ensures that only one scraper of each type can run at a time, preventing rate limit issues.
func (s *Service) getScraperMutex(scraperType ScraperType) *sync.Mutex {
	s.mutexLock.Lock()
	defer s.mutexLock.Unlock()

	if s.scraperMutexes[scraperType] == nil {
		s.scraperMutexes[scraperType] = &sync.Mutex{}
	}

	return s.scraperMutexes[scraperType]
}

// GetScrapersForSeries returns all configured scrapers that match the given series name.
// This method reads scraper configuration from the SerieConfig inline fields in series TOML files.
//
// Parameters:
//   - serieName: The series name to match against series configurations
//
// Returns:
//   - []Scraper: Array of initialized scraper instances
//   - error: Any errors during scraper initialization
func (s *Service) GetScrapersForSeries(serieName string, firstPageDBOnly bool) ([]Scraper, error) {
	if serieName == "" {
		return nil, fmt.Errorf("series name is required")
	}

	// Find the series configuration by loading TOML files
	seriesCfg, err := s.findSeriesConfig(serieName)
	if err != nil {
		return nil, err
	}

	// Check if series has scraper source
	if !strings.EqualFold(seriesCfg.Source, "scraper") {
		return nil, fmt.Errorf(
			"series %s does not use scraper source (source = %s)",
			serieName,
			seriesCfg.Source,
		)
	}

	// Validate scraper type is configured
	if seriesCfg.ScraperType == "" {
		return nil, fmt.Errorf(
			"series %s has scraper source but no scraper_type configured",
			serieName,
		)
	}

	// Create scraper based on the series configuration
	scraper, err := s.createScraperFromSerieConfig(seriesCfg, firstPageDBOnly)
	if err != nil {
		logger.Logtype(logger.StatusError, 0).
			Err(err).
			Str("scraper_type", seriesCfg.ScraperType).
			Str("series", serieName).
			Msg("Failed to create scraper")

		return nil, fmt.Errorf("failed to create scraper for series %s: %w", serieName, err)
	}

	var scrapers []Scraper

	scrapers = append(scrapers, scraper)

	return scrapers, nil
}

// RunScrapersForSeries executes all scrapers configured for a series.
// This method ensures that only one scraper of each type runs at a time across all parallel
// jobImportDBSeries calls, preventing rate limit violations.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - serieName: The series name to scrape for
//   - firstpageonly: Whether to scrape only the first page
//
// Returns:
//   - int: Total number of episodes processed across all scrapers
//   - error: Any errors during scraping (returns first error encountered)
func (s *Service) RunScrapersForSeries(
	ctx context.Context,
	serieName string,
	firstpageonly bool,
) (int, error) {
	// First, get the series config to determine scraper type
	seriesCfg, err := s.findSeriesConfig(serieName)
	if err != nil {
		return 0, err
	}

	// Validate scraper configuration
	if !strings.EqualFold(seriesCfg.Source, "scraper") {
		return 0, fmt.Errorf(
			"series %s does not use scraper source (source = %s)",
			serieName,
			seriesCfg.Source,
		)
	}

	if seriesCfg.ScraperType == "" {
		return 0, fmt.Errorf(
			"series %s has scraper source but no scraper_type configured",
			serieName,
		)
	}

	// Get the scraper type
	scraperType := ScraperType(strings.ToLower(seriesCfg.ScraperType))

	// Get the mutex for this scraper type to ensure only one scraper of this type runs at a time
	mutex := s.getScraperMutex(scraperType)

	logger.Logtype(logger.StatusDebug, 0).
		Str("series", serieName).
		Str("scraper_type", string(scraperType)).
		Msg("Waiting for scraper type lock")

	// Lock the mutex for this scraper type - this blocks if another scraper of the same type is running
	mutex.Lock()
	defer mutex.Unlock()

	logger.Logtype(logger.StatusInfo, 0).
		Str("series", serieName).
		Str("scraper_type", string(scraperType)).
		Msg("Acquired scraper type lock, starting scraper")

	// Now proceed with getting and running the scrapers
	scrapers, err := s.GetScrapersForSeries(serieName, firstpageonly)
	if err != nil {
		return 0, err
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("series", serieName).
		Int("scraper_count", len(scrapers)).
		Msg("Running scrapers for series")

	totalProcessed := 0

	for idx, scraper := range scrapers {
		logger.Logtype(logger.StatusInfo, 0).
			Str("series", serieName).
			Int("scraper_index", idx+1).
			Int("total_scrapers", len(scrapers)).
			Msg("Executing scraper")

		count, err := scraper.Scrape(ctx, firstpageonly)
		if err != nil {
			logger.Logtype(logger.StatusError, 0).
				Err(err).
				Str("series", serieName).
				Int("scraper_index", idx+1).
				Msg("Scraper failed")

			return totalProcessed, fmt.Errorf("scraper %d failed: %w", idx+1, err)
		}

		totalProcessed += count

		logger.Logtype(logger.StatusInfo, 0).
			Str("series", serieName).
			Int("episodes_processed", count).
			Int("scraper_index", idx+1).
			Msg("Scraper completed")
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("series", serieName).
		Int("total_episodes", totalProcessed).
		Int("scraper_count", len(scrapers)).
		Str("scraper_type", string(scraperType)).
		Msg("All scrapers completed for series, releasing lock")

	return totalProcessed, nil
}

// findSeriesConfig searches for a series configuration by name in the config/series_*.toml files.
//
// Parameters:
//   - serieName: The series name to search for
//
// Returns:
//   - *config.SerieConfig: The found series configuration
//   - error: Any errors during search or file loading
func (s *Service) findSeriesConfig(serieName string) (*config.SerieConfig, error) {
	// Get the config directory path (hardcoded to ./config)
	configDir := "./config"

	// Search for series_*.toml files
	pattern := filepath.Join(configDir, "series_*.toml")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for series files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no series configuration files found in %s", configDir)
	}

	// Search through each series file
	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			logger.Logtype(logger.StatusWarning, 0).
				Err(err).
				Str("file", filePath).
				Msg("Failed to read series configuration file")

			continue
		}

		var seriesConfig config.MainSerieConfig
		if err := toml.Unmarshal(content, &seriesConfig); err != nil {
			logger.Logtype(logger.StatusWarning, 0).
				Err(err).
				Str("file", filePath).
				Msg("Failed to parse series configuration file")

			continue
		}

		// Search for the series by name
		for idx := range seriesConfig.Serie {
			if strings.EqualFold(seriesConfig.Serie[idx].Name, serieName) {
				logger.Logtype(logger.StatusDebug, 0).
					Str("series", serieName).
					Str("file", filePath).
					Msg("Found series configuration")

				return &seriesConfig.Serie[idx], nil
			}
		}
	}

	return nil, fmt.Errorf("series configuration not found for: %s", serieName)
}

// createScraperFromSerieConfig creates a scraper instance from a SerieConfig.
//
// Parameters:
//   - cfg: Series configuration
//
// Returns:
//   - Scraper: Initialized scraper instance
//   - error: Any errors during scraper creation
func (s *Service) createScraperFromSerieConfig(
	cfg *config.SerieConfig,
	firstPageDBOnly bool,
) (Scraper, error) {
	scraperType := ScraperType(strings.ToLower(cfg.ScraperType))

	switch scraperType {
	case ScraperTypeProject1Service:
		return s.createProject1ServiceScraperFromSerie(cfg, firstPageDBOnly)
	case ScraperTypeAlgolia:
		return s.createAlgoliaScraperFromSerie(cfg, firstPageDBOnly)
	case ScraperTypeHTMLXPath:
		return s.createHTMLXPathScraperFromSerie(cfg, firstPageDBOnly)
	case ScraperTypeCSRFAPI:
		return s.createCSRFAPIScraperFromSerie(cfg, firstPageDBOnly)
	default:
		return nil, fmt.Errorf("unsupported scraper type: %s", cfg.ScraperType)
	}
}

// createProject1ServiceScraperFromSerie creates a Project1Service scraper from SerieConfig.
//
// Parameters:
//   - cfg: Series configuration
//
// Returns:
//   - Scraper: Initialized Project1Service scraper
//   - error: Any errors during scraper creation
func (s *Service) createProject1ServiceScraperFromSerie(
	cfg *config.SerieConfig,
	firstPageDBOnly bool,
) (Scraper, error) {
	scraperCfg := &project1service.Config{
		SiteName:           cfg.Name,
		StartURL:           cfg.StartURL,
		SiteID:             cfg.SiteID,
		FilterCollectionID: cfg.FilterCollectionID,
		FirstPageDBOnly:    firstPageDBOnly,
		SerieName:          cfg.Name,
	}

	return project1service.NewScraper(scraperCfg)
}

// createAlgoliaScraperFromSerie creates an Algolia scraper from SerieConfig.
//
// Parameters:
//   - cfg: Series configuration
//
// Returns:
//   - Scraper: Initialized Algolia scraper
//   - error: Any errors during scraper creation
func (s *Service) createAlgoliaScraperFromSerie(
	cfg *config.SerieConfig,
	firstPageDBOnly bool,
) (Scraper, error) {
	scraperCfg := &algolia.Config{
		SiteName:              cfg.Name,
		StartURL:              cfg.StartURL,
		SiteURL:               cfg.SiteURL,
		SiteFilterName:        cfg.SiteFilterName,
		SerieFilterName:       cfg.SerieFilterName,
		NetworkFilterName:     cfg.NetworkFilterName,
		NetworkSiteFilterName: cfg.NetworkSiteFilterName,
		FirstPageDBOnly:       firstPageDBOnly,
		SerieName:             cfg.Name,
	}

	return algolia.NewScraper(scraperCfg)
}

// createHTMLXPathScraperFromSerie creates an HTML/XPath scraper from SerieConfig.
//
// Parameters:
//   - cfg: Series configuration
//
// Returns:
//   - Scraper: Initialized HTML/XPath scraper
//   - error: Any errors during scraper creation
func (s *Service) createHTMLXPathScraperFromSerie(
	cfg *config.SerieConfig,
	firstPageDBOnly bool,
) (Scraper, error) {
	scraperCfg := &htmlxpath.Config{
		SiteName:        cfg.Name,
		StartURL:        cfg.StartURL,
		BaseURL:         cfg.SiteURL,
		SerieName:       cfg.Name,
		FirstPageDBOnly: firstPageDBOnly,

		// XPath selectors
		SceneNodeXPath: cfg.SceneNodeXPath,
		TitleXPath:     cfg.TitleXPath,
		URLXPath:       cfg.URLXPath,
		DateXPath:      cfg.DateXPath,
		ActorsXPath:    cfg.ActorsXPath,
		TitleAttribute: cfg.TitleAttribute,
		URLAttribute:   cfg.URLAttribute,

		// Pagination
		PaginationType: cfg.PaginationType,
		PageIncrement:  cfg.PageIncrement,
		PageURLPattern: cfg.PageURLPattern,

		// Date parsing
		DateFormat: cfg.DateFormat,

		// Optional
		WaitSeconds: cfg.WaitSeconds,
	}

	return htmlxpath.NewScraper(scraperCfg)
}

// createCSRFAPIScraperFromSerie creates a CSRF API scraper from SerieConfig.
//
// Parameters:
//   - cfg: Series configuration
//
// Returns:
//   - Scraper: Initialized CSRF API scraper
//   - error: Any errors during scraper creation
func (s *Service) createCSRFAPIScraperFromSerie(
	cfg *config.SerieConfig,
	firstPageDBOnly bool,
) (Scraper, error) {
	scraperCfg := &csrfapi.Config{
		SiteName:        cfg.Name,
		StartURL:        cfg.StartURL,
		BaseURL:         cfg.SiteURL,
		SerieName:       cfg.Name,
		FirstPageDBOnly: firstPageDBOnly,

		// CSRF settings
		CSRFCookieName: cfg.CSRFCookieName,
		CSRFHeaderName: cfg.CSRFHeaderName,

		// API settings
		APIURLPattern:   cfg.APIURLPattern,
		PageStartIndex:  cfg.PageStartIndex,
		PaginationStyle: cfg.PaginationType,

		// JSON field paths
		ResultsArrayPath: cfg.ResultsArrayPath,
		TitleField:       cfg.TitleField,
		DateField:        cfg.DateField,
		URLField:         cfg.URLField,
		ActorsField:      cfg.ActorsField,
		ActorNameField:   cfg.ActorNameField,
		RuntimeField:     cfg.RuntimeField,

		// Date parsing
		DateFormat: cfg.DateFormat,

		// Optional
		WaitSeconds: cfg.WaitSeconds,
	}

	return csrfapi.NewScraper(scraperCfg)
}
