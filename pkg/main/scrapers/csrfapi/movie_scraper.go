package csrfapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/sitethrottle"
	"github.com/goccy/go-json"
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
	PageStartIndex   int    // Starting page index: 0/unset = 1, negative = 0-based (see Config)
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
		return nil, errors.New("site_name is required")
	}

	if cfg.StartURL == "" {
		return nil, errors.New("start_url is required")
	}

	// Default the same way NewScraper (the episode variant) does, instead of
	// hard-erroring - there's no functional reason for movie scraping to be
	// stricter than episode scraping for the exact same config fields.
	if cfg.CSRFCookieName == "" {
		cfg.CSRFCookieName = "_csrf"
	}

	if cfg.CSRFHeaderName == "" {
		cfg.CSRFHeaderName = "csrf-token"
	}

	if cfg.APIURLPattern == "" {
		return nil, errors.New("api_url_pattern is required")
	}

	if cfg.ResultsArrayPath == "" {
		return nil, errors.New("results_array_path is required")
	}

	if cfg.TitleField == "" {
		return nil, errors.New("title_field is required")
	}

	// Set defaults
	if cfg.BaseURL == "" {
		cfg.BaseURL = cfg.StartURL
	}

	cfg.PageStartIndex = normalizePageStartIndex(cfg.PageStartIndex)

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
	for page := range maxPages {
		// Build page URL
		pageNum := s.config.PageStartIndex + page
		apiURL := strings.ReplaceAll(s.config.APIURLPattern, "{page}", fmt.Sprintf("%d", pageNum))

		logger.Logtype("debug", 1).
			Int("page", page+1).
			Str("url", apiURL).
			Msg("Scraping movie API page")

		movies, err := s.scrapePage(ctx, apiURL)
		if err != nil {
			if page == 0 {
				return nil, fmt.Errorf("failed to scrape page 1: %w", err)
			}

			// Hand back what earlier pages produced; the caller decides
			// whether a partial list is usable.
			return imdbIDs, fmt.Errorf(
				"failed to scrape page %d (%d ids from earlier pages kept): %w",
				page+1,
				len(imdbIDs),
				err,
			)
		}

		if len(movies) == 0 {
			logger.Logtype("debug", 1).
				Int("page", page+1).
				Msg("No movies found, stopping pagination")
			break
		}

		for idx := range movies {
			imdbIDs = s.appendMovieImdbID(imdbIDs, seenIDs, &movies[idx])
		}

		// Page spacing is enforced per site in getAPI (sitethrottle).
		if err := ctx.Err(); err != nil {
			return imdbIDs, err
		}
	}

	logger.Logtype("info", 1).
		Int("count", len(imdbIDs)).
		Msg("CSRF API movie scrape completed")

	return imdbIDs, nil
}

// normalizeImdbID turns a scraped IMDb id - "tt1234567", a bare 7-9 digit
// number, or a URL/path such as "/title/tt1234567/?ref_=x" - into
// "tt"+digits. Anything else is rejected (ok=false, empty id).
func normalizeImdbID(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}

	s = strings.TrimRight(s, "/")

	hadPath := false
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
		hadPath = true
	}

	digits := s
	if len(s) >= 2 && strings.EqualFold(s[:2], "tt") {
		digits = s[2:]
	} else if hadPath {
		// A bare number in some other URL is not an IMDb title id.
		return "", false
	}

	if len(digits) < 7 || len(digits) > 9 {
		return "", false
	}

	for i := range len(digits) {
		if digits[i] < '0' || digits[i] > '9' {
			return "", false
		}
	}

	return "tt" + digits, true
}

// appendMovieImdbID appends the movie's IMDb id - taken directly, or found via
// a TMDB title/year search when the item carries none - unless already seen.
func (s *MovieScraper) appendMovieImdbID(
	imdbIDs []string,
	seenIDs map[string]bool,
	movie *MovieData,
) []string {
	if movie.ImdbID != "" {
		id, ok := normalizeImdbID(movie.ImdbID)
		if ok {
			if !seenIDs[id] {
				imdbIDs = append(imdbIDs, id)
				seenIDs[id] = true
			}

			return imdbIDs
		}

		// Unusable id: treat the item as carrying none and try the search below.
		logger.Logtype("debug", 1).
			Str("imdb_id", movie.ImdbID).
			Str("title", movie.Title).
			Msg("Ignoring invalid scraped IMDB ID")
	}

	if movie.Title == "" || movie.Year <= 0 {
		return imdbIDs
	}

	imdbID, err := s.searchIMDBID(movie.Title, movie.Year)
	if err == nil {
		imdbID, _ = normalizeImdbID(imdbID)
	}

	if err != nil || imdbID == "" {
		logger.Logtype("debug", 1).
			Str("title", movie.Title).
			Int("year", movie.Year).
			Msg("Could not find IMDB ID for movie")

		return imdbIDs
	}

	if !seenIDs[imdbID] {
		logger.Logtype("debug", 1).
			Str("title", movie.Title).
			Int("year", movie.Year).
			Str("imdb_id", imdbID).
			Msg("Found IMDB ID via search")

		imdbIDs = append(imdbIDs, imdbID)
		seenIDs[imdbID] = true
	}

	return imdbIDs
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
		if cookie.Name != s.config.CSRFCookieName {
			continue
		}

		s.csrfToken = cookie.Value
		logger.Logtype("debug", 1).
			Str("cookie", s.config.CSRFCookieName).
			Msg("Extracted CSRF token")

		return nil
	}

	return fmt.Errorf("CSRF cookie '%s' not found", s.config.CSRFCookieName)
}

// scrapePage fetches and parses a single API page, returning movie data.
func (s *MovieScraper) scrapePage(ctx context.Context, apiURL string) ([]MovieData, error) {
	body, status, err := s.getAPI(ctx, apiURL)
	if err != nil {
		return nil, err
	}

	// The token is fetched once per scrape and can expire mid-run.
	if isCSRFRejection(status) {
		logger.Logtype("info", 1).
			Str("site", s.config.SiteName).
			Int("status", status).
			Msg("API rejected CSRF token, refreshing and retrying once")

		if err := s.extractCSRFToken(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh CSRF token after status %d: %w", status, err)
		}

		body, status, err = s.getAPI(ctx, apiURL)
		if err != nil {
			return nil, err
		}
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", status)
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract results array using path
	results, err := s.extractResultsArray(data)
	if err != nil {
		return nil, err
	}

	var movies []MovieData
	for i := range results {
		movie := s.extractMovieData(results[i])
		if movie.Title != "" {
			movies = append(movies, movie)
		}
	}

	return movies, nil
}

// getAPI performs one API GET and returns the body and status code.
func (s *MovieScraper) getAPI(ctx context.Context, apiURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set(s.config.CSRFHeaderName, s.csrfToken)

	// Enforce the configured delay per site (host), shared across all scraper
	// configs targeting the same site. Mirrors Scraper.getAPI.
	sitethrottle.Wait(ctx, req.URL.Host, time.Duration(s.config.WaitSeconds)*time.Second)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch API page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, resp.StatusCode, nil
}

// extractResultsArray extracts the results array from JSON data using the configured path.
func (s *MovieScraper) extractResultsArray(
	data map[string]any,
) ([]map[string]any, error) {
	// Handle simple path (e.g., "movies")
	if !strings.Contains(s.config.ResultsArrayPath, ".") {
		if arr, ok := data[s.config.ResultsArrayPath].([]any); ok {
			results := make([]map[string]any, 0, len(arr))
			for i := range arr {
				if m, ok := arr[i].(map[string]any); ok {
					results = append(results, m)
				}
			}

			return results, nil
		}

		return nil, fmt.Errorf("results array '%s' not found or invalid", s.config.ResultsArrayPath)
	}

	// Handle nested path (e.g., "data.results")
	parts := strings.Split(s.config.ResultsArrayPath, ".")

	var current any = data

	for i, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid path at '%s'", logger.JoinStringsSep(parts[:i+1], "."))
		}

		current = m[part]
	}

	if arr, ok := current.([]any); ok {
		results := make([]map[string]any, 0, len(arr))
		for i := range arr {
			if m, ok := arr[i].(map[string]any); ok {
				results = append(results, m)
			}
		}

		return results, nil
	}

	return nil, errors.New("results path does not point to an array")
}

// extractMovieData extracts movie data from a single JSON object.
func (s *MovieScraper) extractMovieData(data map[string]any) MovieData {
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
			case []any:
				var genres []string
				for i := range v {
					if gs, ok := v[i].(string); ok {
						genres = append(genres, gs)
					}
				}

				movie.Genre = logger.JoinStringsSep(genres, ", ")
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
func (*MovieScraper) searchIMDBID(title string, year int) (string, error) {
	logger.Logtype("debug", 1).
		Str("title", title).
		Int("year", year).
		Msg("Searching for IMDB ID via TMDB")

	imdbID, err := providers.TMDBMovieIMDbID(context.Background(), title, year)
	if err != nil {
		return "", err
	}

	return imdbID, nil
}
