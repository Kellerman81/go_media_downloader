package htmlxpath

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// MovieConfig holds the configuration for the HTML/XPath movie scraper.
type MovieConfig struct {
	SiteName string
	StartURL string
	BaseURL  string

	// XPath selectors for extracting movie data
	SceneNodeXPath       string // XPath to select each movie container
	TitleXPath           string // XPath relative to movie node for title
	YearXPath            string // XPath relative to movie node for year
	ImdbIDXPath          string // XPath relative to movie node for IMDB ID
	URLXPath             string // XPath relative to movie node for URL
	RatingXPath          string // XPath relative to movie node for rating/score
	GenreXPath           string // XPath relative to movie node for genre(s)
	ReleaseDateXPath     string // XPath relative to movie node for full release date
	TitleAttribute       string // Optional: HTML attribute to extract title from
	URLAttribute         string // Optional: HTML attribute to extract URL from

	// Pagination settings
	PaginationType string // "sequential" (page 1, 2, 3) or "offset" (0, 12, 24)
	PageIncrement  int    // For offset pagination: how much to increment per page
	PageURLPattern string // URL pattern with {page} placeholder

	// Date parsing
	DateFormat string // Go time format string (e.g., "2006-01-02" or "Jan 2, 2006")

	// Optional settings
	WaitSeconds int // Seconds to wait between requests (default: 2)
}

// MovieScraper implements the HTML/XPath movie scraper.
type MovieScraper struct {
	config *MovieConfig
	client *http.Client
}

// MovieData represents scraped movie data.
type MovieData struct {
	Title       string
	Year        int
	ImdbID      string
	URL         string
	Rating      string
	Genre       string
	ReleaseDate string
}

var imdbIDRegex = regexp.MustCompile(`tt\d{7,}`)

// NewMovieScraper creates a new HTML/XPath movie scraper instance.
func NewMovieScraper(cfg *MovieConfig) (*MovieScraper, error) {
	// Validate required configuration
	if cfg.SiteName == "" {
		return nil, fmt.Errorf("site_name is required")
	}

	if cfg.StartURL == "" {
		return nil, fmt.Errorf("start_url is required")
	}

	if cfg.SceneNodeXPath == "" {
		return nil, fmt.Errorf("scene_node_xpath is required")
	}

	if cfg.TitleXPath == "" {
		return nil, fmt.Errorf("title_xpath is required")
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
		Msg("Starting movie scrape")

	// If no pagination pattern, just scrape the start URL
	if s.config.PageURLPattern == "" {
		movies, err := s.scrapePage(ctx, s.config.StartURL)
		if err != nil {
			return nil, err
		}

		for idx := range movies {
			if movies[idx].ImdbID != "" && !seenIDs[movies[idx].ImdbID] {
				imdbIDs = append(imdbIDs, movies[idx].ImdbID)
				seenIDs[movies[idx].ImdbID] = true
			}
		}

		return imdbIDs, nil
	}

	// Paginated scraping
	if maxPages == 0 {
		maxPages = 10 // Default to 10 pages
	}

	for page := 0; page < maxPages; page++ {
		// Build page URL
		pageNum := s.getPageNumber(page)
		url := strings.ReplaceAll(s.config.PageURLPattern, "{page}", fmt.Sprintf("%d", pageNum))

		logger.Logtype("debug", 1).
			Int("page", page+1).
			Str("url", url).
			Msg("Scraping movie page")

		movies, err := s.scrapePage(ctx, url)
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
		Msg("Movie scrape completed")

	return imdbIDs, nil
}

// scrapePage fetches and parses a single page, returning movie data.
func (s *MovieScraper) scrapePage(ctx context.Context, url string) ([]MovieData, error) {
	// Fetch the page
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract movie nodes
	movieNodes := htmlquery.Find(doc, s.config.SceneNodeXPath)
	if len(movieNodes) == 0 {
		return nil, nil // No movies found on this page
	}

	var movies []MovieData

	for _, node := range movieNodes {
		movie := s.extractMovieData(node)
		if movie.Title != "" {
			movies = append(movies, movie)
		}
	}

	return movies, nil
}

// extractMovieData extracts movie data from a single movie node.
func (s *MovieScraper) extractMovieData(node *html.Node) MovieData {
	movie := MovieData{}

	// Extract title
	if s.config.TitleXPath != "" {
		titleNode := htmlquery.FindOne(node, s.config.TitleXPath)
		if titleNode != nil {
			if s.config.TitleAttribute != "" {
				movie.Title = htmlquery.SelectAttr(titleNode, s.config.TitleAttribute)
			} else {
				movie.Title = strings.TrimSpace(htmlquery.InnerText(titleNode))
			}
		}
	}

	// Extract year
	if s.config.YearXPath != "" {
		yearNode := htmlquery.FindOne(node, s.config.YearXPath)
		if yearNode != nil {
			yearStr := strings.TrimSpace(htmlquery.InnerText(yearNode))
			if year, err := strconv.Atoi(yearStr); err == nil && year > 1800 && year < 2100 {
				movie.Year = year
			}
		}
	}

	// Extract IMDB ID
	if s.config.ImdbIDXPath != "" {
		imdbNode := htmlquery.FindOne(node, s.config.ImdbIDXPath)
		if imdbNode != nil {
			imdbText := htmlquery.InnerText(imdbNode)
			// Also check href attribute for URLs
			if imdbText == "" {
				imdbText = htmlquery.SelectAttr(imdbNode, "href")
			}
			// Extract IMDB ID using regex
			if matches := imdbIDRegex.FindString(imdbText); matches != "" {
				movie.ImdbID = matches
			}
		}
	}

	// Extract URL
	if s.config.URLXPath != "" {
		urlNode := htmlquery.FindOne(node, s.config.URLXPath)
		if urlNode != nil {
			if s.config.URLAttribute != "" {
				movie.URL = htmlquery.SelectAttr(urlNode, s.config.URLAttribute)
			} else {
				movie.URL = htmlquery.SelectAttr(urlNode, "href")
			}
		}
	}

	// Extract rating
	if s.config.RatingXPath != "" {
		ratingNode := htmlquery.FindOne(node, s.config.RatingXPath)
		if ratingNode != nil {
			movie.Rating = strings.TrimSpace(htmlquery.InnerText(ratingNode))
		}
	}

	// Extract genre
	if s.config.GenreXPath != "" {
		genreNodes := htmlquery.Find(node, s.config.GenreXPath)
		var genres []string
		for _, gNode := range genreNodes {
			genre := strings.TrimSpace(htmlquery.InnerText(gNode))
			if genre != "" {
				genres = append(genres, genre)
			}
		}
		if len(genres) > 0 {
			movie.Genre = strings.Join(genres, ", ")
		}
	}

	// Extract release date
	if s.config.ReleaseDateXPath != "" {
		dateNode := htmlquery.FindOne(node, s.config.ReleaseDateXPath)
		if dateNode != nil {
			movie.ReleaseDate = strings.TrimSpace(htmlquery.InnerText(dateNode))
			// Try to parse year from release date if year not already set
			if movie.Year == 0 && movie.ReleaseDate != "" {
				if s.config.DateFormat != "" {
					if t, err := time.Parse(s.config.DateFormat, movie.ReleaseDate); err == nil {
						movie.Year = t.Year()
					}
				} else {
					// Try common formats
					formats := []string{"2006-01-02", "Jan 2, 2006", "January 2, 2006", "2006"}
					for _, format := range formats {
						if t, err := time.Parse(format, movie.ReleaseDate); err == nil {
							movie.Year = t.Year()
							break
						}
					}
				}
			}
		}
	}

	return movie
}

// getPageNumber returns the page number/offset based on pagination type.
func (s *MovieScraper) getPageNumber(pageIndex int) int {
	if s.config.PaginationType == "offset" {
		return pageIndex * s.config.PageIncrement
	}
	return pageIndex + 1 // Sequential: 1, 2, 3, ...
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
