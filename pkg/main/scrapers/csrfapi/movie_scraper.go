package csrfapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// MovieConfig holds the configuration for the CSRF API movie scraper.
type MovieConfig struct {
	SiteName string
	StartURL string
	BaseURL  string

	// CSRF settings
	CSRFCookieName string // Name of cookie containing CSRF token
	CSRFHeaderName string // Name of header to send CSRF token in

	// API settings
	APIURLPattern    string // URL pattern with {page} placeholder
	PageStartIndex   int    // Starting page index (0 or 1)
	ResultsArrayPath string // JSON path to results array (e.g., "movies" or "data.results")

	// JSON field mappings
	TitleField       string // JSON field for title
	YearField        string // JSON field for year
	ImdbIDField      string // JSON field for IMDB ID
	URLField         string // JSON field for URL
	RatingField      string // JSON field for rating/score
	GenreField       string // JSON field for genre(s)
	ReleaseDateField string // JSON field for full release date

	// Date parsing
	DateFormat string // Go time format string

	// Optional settings
	WaitSeconds int // Seconds to wait between requests (default: 2)
}

// MovieScraper implements the CSRF API movie scraper.
type MovieScraper struct {
	config    *MovieConfig
	client    *http.Client
	csrfToken string
}

// MovieData represents scraped movie data from API.
type MovieData struct {
	Title       string
	Year        int
	ImdbID      string
	URL         string
	Rating      string
	Genre       string
	ReleaseDate string
}

// NewMovieScraper creates a new CSRF API movie scraper instance.
func NewMovieScraper(cfg *MovieConfig) (*MovieScraper, error) {
	// Validate required configuration
	if cfg.SiteName == "" {
		return nil, fmt.Errorf("site_name is required")
	}

	if cfg.StartURL == "" {
		return nil, fmt.Errorf("start_url is required")
	}

	if cfg.CSRFCookieName == "" {
		return nil, fmt.Errorf("csrf_cookie_name is required")
	}

	if cfg.CSRFHeaderName == "" {
		return nil, fmt.Errorf("csrf_header_name is required")
	}

	if cfg.APIURLPattern == "" {
		return nil, fmt.Errorf("api_url_pattern is required")
	}

	if cfg.ResultsArrayPath == "" {
		return nil, fmt.Errorf("results_array_path is required")
	}

	if cfg.TitleField == "" {
		return nil, fmt.Errorf("title_field is required")
	}

	// Set defaults
	if cfg.BaseURL == "" {
		cfg.BaseURL = cfg.StartURL
	}

	if cfg.PageStartIndex == 0 {
		cfg.PageStartIndex = 1 // Default to 1-based pagination
	}

	if cfg.WaitSeconds == 0 {
		cfg.WaitSeconds = 2
	}

	// Create HTTP client with cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
	}

	return &MovieScraper{
		config: cfg,
		client: client,
	}, nil
}

// Scrape executes the scraping process and returns a list of IMDB IDs.
func (s *MovieScraper) Scrape(ctx context.Context, maxPages int) ([]string, error) {
	var imdbIDs []string
	seenIDs := make(map[string]bool)

	logger.Logtype("info", 1).
		Str("site", s.config.SiteName).
		Str("url", s.config.StartURL).
		Msg("Starting CSRF API movie scrape")

	// First, extract CSRF token
	if err := s.extractCSRFToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to extract CSRF token: %w", err)
	}

	if maxPages == 0 {
		maxPages = 10 // Default to 10 pages
	}

	// Paginated scraping
	for page := 0; page < maxPages; page++ {
		// Build page URL
		pageNum := s.config.PageStartIndex + page
		apiURL := strings.ReplaceAll(s.config.APIURLPattern, "{page}", fmt.Sprintf("%d", pageNum))

		logger.Logtype("debug", 1).
			Int("page", page+1).
			Str("url", apiURL).
			Msg("Scraping movie API page")

		movies, err := s.scrapePage(ctx, apiURL)
		if err != nil {
			logger.Logtype("warn", 1).
				Err(err).
				Int("page", page+1).
				Msg("Failed to scrape page, stopping")
			break
		}

		if len(movies) == 0 {
			logger.Logtype("debug", 1).
				Int("page", page+1).
				Msg("No movies found, stopping pagination")
			break
		}

		// Process movies from this page
		for idx := range movies {
			// If IMDB ID is directly available, use it
			if movies[idx].ImdbID != "" && !seenIDs[movies[idx].ImdbID] {
				imdbIDs = append(imdbIDs, movies[idx].ImdbID)
				seenIDs[movies[idx].ImdbID] = true
			} else if movies[idx].Title != "" && movies[idx].Year > 0 {
				// Search for IMDB ID using title and year
				imdbID, err := s.searchIMDBID(movies[idx].Title, movies[idx].Year)
				if err == nil && imdbID != "" && !seenIDs[imdbID] {
					logger.Logtype("debug", 1).
						Str("title", movies[idx].Title).
						Int("year", movies[idx].Year).
						Str("imdb_id", imdbID).
						Msg("Found IMDB ID via search")
					imdbIDs = append(imdbIDs, imdbID)
					seenIDs[imdbID] = true
				} else {
					logger.Logtype("debug", 1).
						Str("title", movies[idx].Title).
						Int("year", movies[idx].Year).
						Msg("Could not find IMDB ID for movie")
				}
			}
		}

		// Wait between requests
		if page < maxPages-1 && s.config.WaitSeconds > 0 {
			time.Sleep(time.Duration(s.config.WaitSeconds) * time.Second)
		}
	}

	logger.Logtype("info", 1).
		Int("count", len(imdbIDs)).
		Msg("CSRF API movie scrape completed")

	return imdbIDs, nil
}

// extractCSRFToken visits the start URL and extracts the CSRF token from cookies.
func (s *MovieScraper) extractCSRFToken(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.config.StartURL, nil)
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
	parsedURL, err := url.Parse(s.config.StartURL)
	if err != nil {
		return fmt.Errorf("failed to parse start URL: %w", err)
	}

	cookies := s.client.Jar.Cookies(parsedURL)
	for _, cookie := range cookies {
		if cookie.Name == s.config.CSRFCookieName {
			s.csrfToken = cookie.Value
			logger.Logtype("debug", 1).
				Str("cookie", s.config.CSRFCookieName).
				Msg("Extracted CSRF token")
			return nil
		}
	}

	return fmt.Errorf("CSRF cookie '%s' not found", s.config.CSRFCookieName)
}

// scrapePage fetches and parses a single API page, returning movie data.
func (s *MovieScraper) scrapePage(ctx context.Context, apiURL string) ([]MovieData, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set(s.config.CSRFHeaderName, s.csrfToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract results array using path
	results, err := s.extractResultsArray(data)
	if err != nil {
		return nil, err
	}

	var movies []MovieData
	for _, result := range results {
		movie := s.extractMovieData(result)
		if movie.Title != "" {
			movies = append(movies, movie)
		}
	}

	return movies, nil
}

// extractResultsArray extracts the results array from JSON data using the configured path.
func (s *MovieScraper) extractResultsArray(data map[string]interface{}) ([]map[string]interface{}, error) {
	// Handle simple path (e.g., "movies")
	if !strings.Contains(s.config.ResultsArrayPath, ".") {
		if arr, ok := data[s.config.ResultsArrayPath].([]interface{}); ok {
			results := make([]map[string]interface{}, 0, len(arr))
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					results = append(results, m)
				}
			}
			return results, nil
		}
		return nil, fmt.Errorf("results array '%s' not found or invalid", s.config.ResultsArrayPath)
	}

	// Handle nested path (e.g., "data.results")
	parts := strings.Split(s.config.ResultsArrayPath, ".")
	var current interface{} = data

	for i, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil, fmt.Errorf("invalid path at '%s'", strings.Join(parts[:i+1], "."))
		}
	}

	if arr, ok := current.([]interface{}); ok {
		results := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				results = append(results, m)
			}
		}
		return results, nil
	}

	return nil, fmt.Errorf("results path does not point to an array")
}

// extractMovieData extracts movie data from a single JSON object.
func (s *MovieScraper) extractMovieData(data map[string]interface{}) MovieData {
	movie := MovieData{}

	// Extract title
	if s.config.TitleField != "" {
		if title, ok := data[s.config.TitleField].(string); ok {
			movie.Title = strings.TrimSpace(title)
		}
	}

	// Extract year
	if s.config.YearField != "" {
		if year, ok := data[s.config.YearField]; ok {
			switch v := year.(type) {
			case float64:
				if int(v) > 1800 && int(v) < 2100 {
					movie.Year = int(v)
				}
			case int:
				if v > 1800 && v < 2100 {
					movie.Year = v
				}
			case string:
				if y, err := strconv.Atoi(v); err == nil && y > 1800 && y < 2100 {
					movie.Year = y
				}
			}
		}
	}

	// Extract IMDB ID
	if s.config.ImdbIDField != "" {
		if imdbID, ok := data[s.config.ImdbIDField].(string); ok {
			movie.ImdbID = strings.TrimSpace(imdbID)
		}
	}

	// Extract URL
	if s.config.URLField != "" {
		if urlStr, ok := data[s.config.URLField].(string); ok {
			movie.URL = strings.TrimSpace(urlStr)
		}
	}

	// Extract rating
	if s.config.RatingField != "" {
		if rating, ok := data[s.config.RatingField]; ok {
			movie.Rating = fmt.Sprintf("%v", rating)
		}
	}

	// Extract genre
	if s.config.GenreField != "" {
		if genre, ok := data[s.config.GenreField]; ok {
			switch v := genre.(type) {
			case string:
				movie.Genre = v
			case []interface{}:
				var genres []string
				for _, g := range v {
					if gs, ok := g.(string); ok {
						genres = append(genres, gs)
					}
				}
				movie.Genre = strings.Join(genres, ", ")
			}
		}
	}

	// Extract release date
	if s.config.ReleaseDateField != "" {
		if date, ok := data[s.config.ReleaseDateField].(string); ok {
			movie.ReleaseDate = strings.TrimSpace(date)
			// Try to parse year from release date if year not already set
			if movie.Year == 0 && movie.ReleaseDate != "" {
				if s.config.DateFormat != "" {
					if t, err := time.Parse(s.config.DateFormat, movie.ReleaseDate); err == nil {
						movie.Year = t.Year()
					}
				} else {
					// Try ISO 8601 format
					if t, err := time.Parse("2006-01-02", movie.ReleaseDate); err == nil {
						movie.Year = t.Year()
					}
				}
			}
		}
	}

	return movie
}

// searchIMDBID searches for an IMDB ID using title and year via TMDB.
func (s *MovieScraper) searchIMDBID(title string, year int) (string, error) {
	logger.Logtype("debug", 1).
		Str("title", title).
		Int("year", year).
		Msg("Searching for IMDB ID via TMDB")

	// Use the apiexternal package to search TMDB
	imdbID, err := apiexternal.SearchTMDBMovieImdbID(title, year)
	if err != nil {
		return "", err
	}

	return imdbID, nil
}
