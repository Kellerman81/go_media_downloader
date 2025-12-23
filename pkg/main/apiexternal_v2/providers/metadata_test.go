package providers_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/omdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

const (
	// Test IMDb ID for Nosferatu (2024) - the movie that's failing in production
	testIMDbIDNosferatu = "tt37263154"

	// Test IMDb ID for The Shawshank Redemption (1994) - well-known movie for baseline testing
	testIMDbIDShawshank = "tt0111161"

	// Test IMDb IDs for TV shows (TVDB is primarily for TV series)
	testIMDbIDBreakingBad = "tt0903747" // Breaking Bad
	testIMDbIDGameOfThrones = "tt0944947" // Game of Thrones

	// Test TVDB IDs
	testTVDBIDBreakingBad = 81189 // Breaking Bad
)

// loadTestConfig loads API keys from the config.toml file
func loadTestConfig(t *testing.T) (tmdbKey, traktID, traktSecret, omdbKey, tokenPath string) {
	// Set config file path if not already set
	if config.Configfile == "" || config.Configfile == "./config/config.toml" {
		// Try to find config file in the repository
		config.Configfile = "R:\\golang_ent\\config\\config.toml"
	}

	// Read config file
	cfg, err := config.Readconfigtoml()
	if err != nil {
		t.Fatalf("Failed to read config.toml: %v", err)
	}

	// Extract API keys from config
	tmdbKey = cfg.General.TheMovieDBApiKey
	traktID = cfg.General.TraktClientID
	traktSecret = cfg.General.TraktClientSecret
	omdbKey = cfg.General.OmdbAPIKey

	// Construct Trakt token path from config file location
	if config.Configfile != "" {
		tokenPath = filepath.Join(filepath.Dir(config.Configfile), "trakt_token.json")
	} else {
		tokenPath = filepath.Join("R:\\golang_ent\\config", "trakt_token.json")
	}

	// Validate that required keys are present
	if tmdbKey == "" {
		t.Skip("TMDB API key not configured in config.toml - skipping TMDB tests")
	}
	if traktID == "" || traktSecret == "" {
		t.Skip("Trakt credentials not configured in config.toml - skipping Trakt tests")
	}
	if omdbKey == "" {
		t.Skip("OMDB API key not configured in config.toml - skipping OMDB tests")
	}

	return
}

func TestTMDBFindMovieByIMDbID(t *testing.T) {
	// Load API keys from config
	tmdbKey, _, _, _, _ := loadTestConfig(t)

	// Use official NewTmdbClient constructor to initialize the global provider
	apiexternal.NewTmdbClient(tmdbKey, 1, 1, false, 30)

	// Get the provider from global registry
	provider := providers.GetTMDB()
	if provider == nil {
		t.Fatal("TMDB provider not initialized by NewTmdbClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with both Nosferatu (the failing case) and Shawshank (baseline)
	t.Run("Nosferatu-2024", func(t *testing.T) {
		t.Logf("Testing TMDB FindMovieByIMDbID for IMDb ID: %s (Nosferatu 2024)", testIMDbIDNosferatu)
		testTMDBMovieLookup(t, provider, ctx, testIMDbIDNosferatu)
	})

	t.Run("Shawshank-Redemption", func(t *testing.T) {
		t.Logf("Testing TMDB FindMovieByIMDbID for IMDb ID: %s (Shawshank Redemption)", testIMDbIDShawshank)
		testTMDBMovieLookup(t, provider, ctx, testIMDbIDShawshank)
	})
}

func testTMDBMovieLookup(t *testing.T, provider *tmdb.Provider, ctx context.Context, imdbID string) {
	result, err := provider.FindMovieByIMDbID(ctx, imdbID)
	if err != nil {
		t.Fatalf("TMDB FindMovieByIMDbID failed: %v", err)
	}

	if result == nil {
		t.Fatal("TMDB returned nil result")
	}

	if len(result.MovieResults) == 0 {
		t.Fatal("TMDB returned no movie results")
	}

	movie := result.MovieResults[0]
	t.Logf("TMDB Movie found:")
	t.Logf("  - ID: %d", movie.ID)
	t.Logf("  - Title: %s", movie.Title)
	t.Logf("  - Year: %d", movie.Year)
	t.Logf("  - Release Date: %s", movie.ReleaseDate)
	t.Logf("  - Overview: %s", movie.Overview)
	t.Logf("  - Vote Average: %.2f", movie.VoteAverage)
	t.Logf("  - Provider: %s", movie.ProviderName)

	if movie.Title == "" {
		t.Error("TMDB movie title is empty")
	}

	if movie.ProviderName != "tmdb" {
		t.Errorf("TMDB provider name incorrect: got %s, want tmdb", movie.ProviderName)
	}
}

func TestTraktFindMovieByIMDbID(t *testing.T) {
	// Load API keys from config
	_, traktID, traktSecret, _, tokenPath := loadTestConfig(t)

	// Load Trakt token from file
	token, err := loadTraktToken(tokenPath)
	if err != nil {
		t.Fatalf("Failed to load Trakt token: %v", err)
	}

	// Use official NewTraktClient constructor to initialize the global provider
	apiexternal.NewTraktClient(traktID, traktSecret, 1, 1, false, 30, "http://localhost:9090")

	// Get the provider from global registry
	provider := providers.GetTrakt()
	if provider == nil {
		t.Fatal("Trakt provider not initialized by NewTraktClient")
	}

	// Set the loaded token
	if err := provider.SetToken(token); err != nil {
		t.Fatalf("Failed to set Trakt token: %v", err)
	}

	// Verify token is valid
	if !provider.IsAuthenticated() {
		t.Fatal("Trakt provider is not authenticated - token may be expired")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with both Nosferatu (the failing case) and Shawshank (baseline)
	t.Run("Nosferatu-2024", func(t *testing.T) {
		t.Logf("Testing Trakt FindMovieByIMDbID for IMDb ID: %s (Nosferatu 2024)", testIMDbIDNosferatu)
		testTraktMovieLookup(t, provider, ctx, testIMDbIDNosferatu)
	})

	t.Run("Shawshank-Redemption", func(t *testing.T) {
		t.Logf("Testing Trakt FindMovieByIMDbID for IMDb ID: %s (Shawshank Redemption)", testIMDbIDShawshank)
		testTraktMovieLookup(t, provider, ctx, testIMDbIDShawshank)
	})
}

func testTraktMovieLookup(t *testing.T, provider *trakt.Provider, ctx context.Context, imdbID string) {
	result, err := provider.FindMovieByIMDbID(ctx, imdbID)
	if err != nil {
		t.Fatalf("Trakt FindMovieByIMDbID failed: %v", err)
	}

	if result == nil {
		t.Fatal("Trakt returned nil result")
	}

	if len(result.MovieResults) == 0 {
		t.Fatal("Trakt returned no movie results")
	}

	movie := result.MovieResults[0]
	t.Logf("Trakt Movie found:")
	t.Logf("  - ID: %d", movie.ID)
	t.Logf("  - Title: %s", movie.Title)
	t.Logf("  - Year: %d", movie.Year)
	t.Logf("  - Release Date: %s", movie.ReleaseDate)
	t.Logf("  - Overview: %s", movie.Overview)
	t.Logf("  - Vote Average: %.2f", movie.VoteAverage)
	t.Logf("  - Provider: %s", movie.ProviderName)

	if movie.Title == "" {
		t.Error("Trakt movie title is empty")
	}

	if movie.ProviderName != "trakt" {
		t.Errorf("Trakt provider name incorrect: got %s, want trakt", movie.ProviderName)
	}
}

// loadTraktToken loads the Trakt OAuth token from the specified file path
func loadTraktToken(path string) (*apiexternal_v2.OAuthToken, error) {
	// Check if token file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	// Read token file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse token
	var token apiexternal_v2.OAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	// Validate token has required fields
	if token.AccessToken == "" {
		return nil, os.ErrInvalid
	}

	return &token, nil
}

func TestOMDBFindMovieByIMDbID(t *testing.T) {
	// Load API keys from config
	_, _, _, omdbKey, _ := loadTestConfig(t)

	// Use official NewOmdbClient constructor to initialize the global provider
	apiexternal.NewOmdbClient(omdbKey, 1, 1, false, 30)

	// Get the provider from global registry
	provider := providers.GetOMDB()
	if provider == nil {
		t.Fatal("OMDB provider not initialized by NewOmdbClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with both Nosferatu (the failing case) and Shawshank (baseline)
	t.Run("Nosferatu-2024", func(t *testing.T) {
		t.Logf("Testing OMDB GetDetailsByIMDb for IMDb ID: %s (Nosferatu 2024)", testIMDbIDNosferatu)
		testOMDBMovieLookup(t, provider, ctx, testIMDbIDNosferatu)
	})

	t.Run("Shawshank-Redemption", func(t *testing.T) {
		t.Logf("Testing OMDB GetDetailsByIMDb for IMDb ID: %s (Shawshank Redemption)", testIMDbIDShawshank)
		testOMDBMovieLookup(t, provider, ctx, testIMDbIDShawshank)
	})
}

func testOMDBMovieLookup(t *testing.T, provider *omdb.Provider, ctx context.Context, imdbID string) {
	result, err := provider.GetDetailsByIMDb(ctx, imdbID)
	if err != nil {
		t.Fatalf("OMDB GetDetailsByIMDb failed: %v", err)
	}

	if result == nil {
		t.Fatal("OMDB returned nil result")
	}

	t.Logf("OMDB Movie found:")
	t.Logf("  - ID: %d", result.ID)
	t.Logf("  - Title: %s", result.Title)
	t.Logf("  - Year: %d", result.Year)
	t.Logf("  - IMDb ID: %s", result.IMDbID)
	t.Logf("  - Release Date: %s", result.ReleaseDate)
	t.Logf("  - Runtime: %d minutes", result.Runtime)
	t.Logf("  - Overview: %s", result.Overview)
	t.Logf("  - Vote Average: %.2f", result.VoteAverage)
	t.Logf("  - Provider: %s", result.ProviderName)

	if result.Title == "" {
		t.Error("OMDB movie title is empty")
	}

	if result.IMDbID != imdbID {
		t.Errorf("OMDB IMDb ID mismatch: got %s, want %s", result.IMDbID, imdbID)
	}

	if result.ProviderName != "omdb" {
		t.Errorf("OMDB provider name incorrect: got %s, want omdb", result.ProviderName)
	}
}

func TestTVDBFindSeriesByIMDbID(t *testing.T) {
	// Use official NewTvdbClient constructor to initialize the global provider
	// Note: TVDB authentication is handled internally, no API key needed in config
	apiexternal.NewTvdbClient(1, 1, false, 30)

	// Get the provider from global registry
	provider := providers.GetTVDB()
	if provider == nil {
		t.Fatal("TVDB provider not initialized by NewTvdbClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with TV shows (TVDB is primarily for TV series)
	t.Run("BreakingBad", func(t *testing.T) {
		t.Logf("Testing TVDB FindSeriesByIMDbID for IMDb ID: %s (Breaking Bad)", testIMDbIDBreakingBad)
		testTVDBSeriesLookup(t, provider, ctx, testIMDbIDBreakingBad)
	})

	t.Run("GameOfThrones", func(t *testing.T) {
		t.Logf("Testing TVDB FindSeriesByIMDbID for IMDb ID: %s (Game of Thrones)", testIMDbIDGameOfThrones)
		testTVDBSeriesLookup(t, provider, ctx, testIMDbIDGameOfThrones)
	})
}

func testTVDBSeriesLookup(t *testing.T, provider *tvdb.Provider, ctx context.Context, imdbID string) {
	result, err := provider.FindSeriesByIMDbID(ctx, imdbID)
	if err != nil {
		// TVDB requires authentication - skip test if we get 401 or rate limit error
		errStr := err.Error()
		if strings.Contains(errStr, "HTTP 401") || strings.Contains(errStr, "rate limit") {
			t.Skipf("TVDB authentication required (no credentials configured): %v", err)
		}
		t.Fatalf("TVDB FindSeriesByIMDbID failed: %v", err)
	}

	if result == nil {
		t.Fatal("TVDB returned nil result")
	}

	if len(result.TVResults) == 0 {
		t.Fatal("TVDB returned no TV series results")
	}

	series := result.TVResults[0]
	t.Logf("TVDB Series found:")
	t.Logf("  - ID: %d", series.ID)
	t.Logf("  - Name: %s", series.Name)
	t.Logf("  - First Air Date: %s", series.FirstAirDate)
	t.Logf("  - Overview: %s", series.Overview)
	t.Logf("  - Provider: %s", series.ProviderName)

	if series.Name == "" {
		t.Error("TVDB series name is empty")
	}

	if series.ProviderName != "tvdb" {
		t.Errorf("TVDB provider name incorrect: got %s, want tvdb", series.ProviderName)
	}
}

func TestTVDBGetSeriesByID(t *testing.T) {
	// Use official NewTvdbClient constructor to initialize the global provider
	apiexternal.NewTvdbClient(1, 1, false, 30)

	// Get the provider from global registry
	provider := providers.GetTVDB()
	if provider == nil {
		t.Fatal("TVDB provider not initialized by NewTvdbClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test direct series lookup by TVDB ID
	t.Logf("Testing TVDB GetSeriesByID for TVDB ID: %d (Breaking Bad)", testTVDBIDBreakingBad)

	details, err := provider.GetSeriesByID(ctx, testTVDBIDBreakingBad)
	if err != nil {
		// TVDB requires authentication - skip test if we get 401 or rate limit error
		errStr := err.Error()
		if strings.Contains(errStr, "HTTP 401") || strings.Contains(errStr, "rate limit") {
			t.Skipf("TVDB authentication required (no credentials configured): %v", err)
		}
		t.Fatalf("TVDB GetSeriesByID failed: %v", err)
	}

	if details == nil {
		t.Fatal("TVDB returned nil details")
	}

	t.Logf("TVDB Series details:")
	t.Logf("  - ID: %d", details.ID)
	t.Logf("  - Name: %s", details.Name)
	t.Logf("  - First Air Date: %s", details.FirstAirDate)
	t.Logf("  - Overview: %s", details.Overview)
	t.Logf("  - Status: %s", details.Status)
	t.Logf("  - Number of Seasons: %d", details.NumberOfSeasons)
	t.Logf("  - Number of Episodes: %d", details.NumberOfEpisodes)
	if len(details.Networks) > 0 {
		t.Logf("  - Network: %s", details.Networks[0].Name)
	}

	if details.Name == "" {
		t.Error("TVDB series name is empty")
	}

	if details.ID != testTVDBIDBreakingBad {
		t.Errorf("TVDB series ID mismatch: got %d, want %d", details.ID, testTVDBIDBreakingBad)
	}
}

func TestAllProviders(t *testing.T) {
	t.Run("TMDB", func(t *testing.T) {
		TestTMDBFindMovieByIMDbID(t)
	})

	t.Run("Trakt", func(t *testing.T) {
		TestTraktFindMovieByIMDbID(t)
	})

	t.Run("OMDB", func(t *testing.T) {
		TestOMDBFindMovieByIMDbID(t)
	})

	t.Run("TVDB", func(t *testing.T) {
		TestTVDBFindSeriesByIMDbID(t)
		TestTVDBGetSeriesByID(t)
	})
}

// TestConfigTokenPath verifies that the config.Configfile variable is accessible
func TestConfigTokenPath(t *testing.T) {
	if config.Configfile == "" {
		t.Skip("config.Configfile is not set - skipping token path test")
	}

	configDir := filepath.Dir(config.Configfile)
	tokenPath := filepath.Join(configDir, "trakt_token.json")

	t.Logf("Config file: %s", config.Configfile)
	t.Logf("Config directory: %s", configDir)
	t.Logf("Expected token path: %s", tokenPath)

	// Check if token file exists at this path
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		t.Logf("Token file not found at config directory path: %s", tokenPath)
		// Load config to get the actual token path
		_, _, _, _, actualTokenPath := loadTestConfig(t)
		t.Logf("Fallback token path from config: %s", actualTokenPath)
	} else {
		t.Logf("Token file found at config directory path: %s", tokenPath)
	}
}
