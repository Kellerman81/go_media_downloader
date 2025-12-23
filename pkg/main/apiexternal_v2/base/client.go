package base

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

//
// BaseClient - Comprehensive client infrastructure with all features
// Provides rate limiting, circuit breaker, auth, statistics, and more
//

// AuthType defines the authentication method.
type AuthType string

const (
	AuthNone         AuthType = "none"
	AuthAPIKeyHeader AuthType = "api_key_header"
	AuthAPIKeyURL    AuthType = "api_key_url"
	AuthOAuth        AuthType = "oauth"
	AuthBasic        AuthType = "basic"
)

// ClientConfig holds configuration for a base client.
type ClientConfig struct {
	// Basic settings
	Name    string
	BaseURL string
	Timeout time.Duration

	// Authentication
	AuthType     AuthType
	APIKey       string
	APIKeyHeader string // e.g., "X-API-Key"
	APIKeyParam  string // e.g., "api_key" for URL parameter
	Username     string // For basic auth
	Password     string // For basic auth

	// OAuth settings
	OAuthClientID     string
	OAuthClientSecret string
	OAuthTokenURL     string
	OAuthScopes       []string

	// Rate limiting - Primary limiter with configurable window
	RateLimitCalls   int // Number of calls allowed
	RateLimitSeconds int // Time window in seconds (e.g., 10 calls per 60 seconds)
	// Legacy/Additional limiters
	RateLimitPer24h   int // Daily limit (optional)
	RateLimitPerTotal int // Total limit, 0 = unlimited (optional)

	// Circuit breaker
	CircuitBreakerThreshold   int           // Number of failures to open
	CircuitBreakerTimeout     time.Duration // How long to stay open
	CircuitBreakerHalfOpenMax int           // Max requests in half-open state

	// Statistics
	EnableStats  bool
	StatsDBTable string // Database table for stats

	// Advanced
	MaxRetries        int
	RetryBackoff      time.Duration
	EnableCompression bool
	UserAgent         string
	DisableTLSVerify  bool // Disable TLS certificate verification (insecure)
}

// BaseClient provides all infrastructure features for API clients.
type BaseClient struct {
	config ClientConfig

	// HTTP client (pooled)
	httpClient *http.Client

	// Rate limiting (using slidingwindow)
	rateLimiterHour  *slidingwindow.Limiter
	rateLimiter24h   *slidingwindow.Limiter
	rateLimiterTotal int64 // Use atomic counter for total requests

	// Server-side rate limiting (HTTP 429 responses)
	rateLimitedUntil atomic.Value // stores time.Time - when server rate limit expires

	// Circuit breaker
	circuitBreaker *CircuitBreaker

	// Authentication - OAuth2
	oauthConfig    *oauth2.Config
	oauthToken     *oauth2.Token
	oauthTokenLock sync.RWMutex

	// Statistics
	stats *ClientStats

	// Config change callback unsubscribe function
	configUnsubscribe func()
}

// ClientStats tracks request statistics.
type ClientStats struct {
	mu sync.RWMutex

	Requests1h          int64
	Requests24h         int64
	RequestsTotal       int64
	AvgResponseTimeMs   int64
	LastRequestAt       time.Time
	LastErrorAt         time.Time
	LastErrorMessage    string
	NextAvailableAt     time.Time
	SuccessCount        int64
	FailureCount        int64
	CircuitBreakerState string

	// For calculating averages
	totalResponseTimeMs int64
	responseCount       int64

	// Sliding window tracking for time-based counters
	requestTimestamps []time.Time
}

// OAuthToken holds OAuth access token information
// Deprecated: Use oauth2.Token from golang.org/x/oauth2 instead.
type OAuthToken struct {
	AccessToken  string
	TokenType    string
	ExpiresAt    time.Time
	RefreshToken string
}

// OAuth2TokenStorage defines the interface for OAuth2 token storage
//
// Implementations can store tokens in various backends:
// - Database (SQLite, PostgreSQL)
// - Key-value store (Pudge, BoltDB)
// - Encrypted file storage
// - Memory (for testing only).
type OAuth2TokenStorage interface {
	// SaveToken persists the OAuth2 token for a given client
	SaveToken(clientName string, token *oauth2.Token) error

	// LoadToken retrieves the OAuth2 token for a given client
	LoadToken(clientName string) (*oauth2.Token, error)

	// DeleteToken removes the OAuth2 token for a given client
	DeleteToken(clientName string) error
}

// OAuth2ProviderHooks defines optional provider-specific OAuth2 customizations
//
// Providers can implement these hooks to customize OAuth2 behavior:
// - Custom token endpoint parameters (e.g., Plex pin-based auth)
// - Token validation and refresh logic
// - Additional authentication steps.
type OAuth2ProviderHooks interface {
	// BeforeTokenRefresh is called before refreshing the OAuth2 token
	// Allows providers to modify the token request or handle custom flows
	BeforeTokenRefresh(ctx context.Context, config *oauth2.Config, token *oauth2.Token) error

	// AfterTokenRefresh is called after successfully refreshing the OAuth2 token
	// Allows providers to validate or process the new token
	AfterTokenRefresh(ctx context.Context, newToken *oauth2.Token) error

	// ValidateToken checks if a token is valid and not expired
	// Providers can implement custom validation logic
	ValidateToken(ctx context.Context, token *oauth2.Token) bool
}

// defaultOAuth2Storage is a simple in-memory token storage for testing.
type defaultOAuth2Storage struct {
	mu     sync.RWMutex
	tokens map[string]*oauth2.Token
}

func newDefaultOAuth2Storage() *defaultOAuth2Storage {
	return &defaultOAuth2Storage{
		tokens: make(map[string]*oauth2.Token),
	}
}

func (s *defaultOAuth2Storage) SaveToken(clientName string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[clientName] = token

	return nil
}

func (s *defaultOAuth2Storage) LoadToken(clientName string) (*oauth2.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.tokens[clientName]
	if !exists {
		return nil, fmt.Errorf("token not found for client %s", clientName)
	}

	return token, nil
}

func (s *defaultOAuth2Storage) DeleteToken(clientName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, clientName)

	return nil
}

var (
	// Global default OAuth2 token storage (can be replaced with custom implementation).
	defaultTokenStorage OAuth2TokenStorage = newDefaultOAuth2Storage()

	// Global OAuth2 provider hooks registry.
	providerHooks = make(map[string]OAuth2ProviderHooks)
	hooksMu       sync.RWMutex
)

// SetDefaultOAuth2Storage sets the global OAuth2 token storage implementation
//
// This should be called during application initialization to configure
// a persistent storage backend (e.g., database or encrypted file storage).
// func setDefaultOAuth2Storage(storage OAuth2TokenStorage) {
// 	defaultTokenStorage = storage
// }

// RegisterOAuth2ProviderHooks registers provider-specific OAuth2 hooks
//
// Example: RegisterOAuth2ProviderHooks("trakt", &TraktOAuth2Hooks{}).
// func registerOAuth2ProviderHooks(providerName string, hooks OAuth2ProviderHooks) {
// 	hooksMu.Lock()
// 	defer hooksMu.Unlock()

// 	providerHooks[providerName] = hooks
// }

// GetOAuth2ProviderHooks retrieves provider-specific OAuth2 hooks.
func getOAuth2ProviderHooks(providerName string) (OAuth2ProviderHooks, bool) {
	hooksMu.RLock()
	defer hooksMu.RUnlock()

	hooks, exists := providerHooks[providerName]

	return hooks, exists
}

// NewBaseClient creates a new base client with full infrastructure.
func NewBaseClient(cfg ClientConfig) *BaseClient {
	// Create HTTP client with connection pooling and TLS configuration
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Configure TLS if needed
	if cfg.DisableTLSVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	httpClient := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}

	client := &BaseClient{
		config:     cfg,
		httpClient: httpClient,
		stats:      &ClientStats{},
	}

	// Initialize rate limiters with configurable time window
	if cfg.RateLimitCalls > 0 && cfg.RateLimitSeconds > 0 {
		window := time.Duration(cfg.RateLimitSeconds) * time.Second

		client.rateLimiterHour = slidingwindow.NewLimiter(window, int64(cfg.RateLimitCalls))
	}

	if cfg.RateLimitPer24h > 0 {
		client.rateLimiter24h = slidingwindow.NewLimiter(24*time.Hour, int64(cfg.RateLimitPer24h))
	}
	// rateLimiterTotal is an int64 counter, initialized to 0

	// Initialize circuit breaker
	client.circuitBreaker = newCircuitBreaker(CircuitBreakerConfig{
		Threshold:   cfg.CircuitBreakerThreshold,
		Timeout:     cfg.CircuitBreakerTimeout,
		HalfOpenMax: cfg.CircuitBreakerHalfOpenMax,
	}, cfg.Name)

	// Initialize OAuth2 if needed
	if cfg.AuthType == AuthOAuth {
		client.initializeOAuth2()
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("client", cfg.Name).
		Msg("BaseClient initialized with full infrastructure")

	return client
}

// NewBaseClientWithConfigProvider creates a client with configuration hot-reload support.
// func newBaseClientWithConfigProvider(
// 	cfg ClientConfig,
// 	provider config.ConfigProvider,
// 	configName string,
// ) *BaseClient {
// 	client := NewBaseClient(cfg)

// 	client.configProvider = provider
// 	client.configName = configName

// 	// Subscribe to configuration changes if ConfigManager interface is available
// 	if manager, ok := provider.(config.ConfigManager); ok {
// 		client.configUnsubscribe = manager.OnConfigChange(func(newProvider config.ConfigProvider) {
// 			// Configuration changed - trigger reload
// 			if err := client.RefreshConfig(); err != nil {
// 				logger.Logtype(logger.StatusError, 0).
// 					Err(err).
// 					Str("client", cfg.Name).
// 					Msg("Failed to refresh configuration after change notification")
// 			}
// 		})
// 	}

// 	return client
// }

// initializeOAuth2 sets up OAuth2 configuration and loads existing token.
func (bc *BaseClient) initializeOAuth2() {
	// Create OAuth2 config
	bc.oauthConfig = &oauth2.Config{
		ClientID:     bc.config.OAuthClientID,
		ClientSecret: bc.config.OAuthClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: bc.config.OAuthTokenURL,
		},
		Scopes: bc.config.OAuthScopes,
	}

	// Try to load existing token from storage
	if token, err := defaultTokenStorage.LoadToken(bc.config.Name); err == nil {
		bc.oauthTokenLock.Lock()

		bc.oauthToken = token
		bc.oauthTokenLock.Unlock()

		logger.Logtype(logger.StatusInfo, 0).
			Str("client", bc.config.Name).
			Time("expires", token.Expiry).
			Msg("Loaded existing OAuth2 token from storage")
	} else {
		logger.Logtype(logger.StatusDebug, 1).
			Str("client", bc.config.Name).
			Err(err).
			Msg("No existing OAuth2 token found in storage")
	}
}

func (bc *BaseClient) CheckFree() error {
	// Check circuit breaker
	if !bc.circuitBreaker.CanMakeRequest() {
		cbState := bc.circuitBreaker.GetState()
		failureCount := bc.circuitBreaker.GetFailureCount()

		logger.Logtype(logger.StatusDebug, 0).
			Str("client", bc.config.Name).
			Str("circuit_state", cbState).
			Int("failure_count", failureCount).
			Msg("Circuit breaker blocked request")

		return fmt.Errorf("circuit breaker is open for %s", bc.config.Name)
	}

	// Check server-side rate limiting (HTTP 429)
	if err := bc.checkServerRateLimit(); err != nil {
		return err
	}

	// Check rate limits
	if err := bc.checkRateLimits(); err != nil {
		return err
	}

	return nil
}

// CheckFreeForDownload performs rate limit checks with extended 2-minute grace period.
// This is specifically for download operations which are rare and critical.
// Downloads will wait up to 2 minutes for a rate limit slot to become available
// and are executed with priority (next in queue).
func (bc *BaseClient) CheckFreeForDownload() error {
	// Check circuit breaker
	if !bc.circuitBreaker.CanMakeRequest() {
		cbState := bc.circuitBreaker.GetState()
		failureCount := bc.circuitBreaker.GetFailureCount()

		logger.Logtype(logger.StatusDebug, 0).
			Str("client", bc.config.Name).
			Str("circuit_state", cbState).
			Int("failure_count", failureCount).
			Msg("Circuit breaker blocked download request")

		return fmt.Errorf("circuit breaker is open for %s", bc.config.Name)
	}

	// Check server-side rate limiting (HTTP 429)
	if err := bc.checkServerRateLimit(); err != nil {
		return err
	}

	// Check rate limits with extended grace period for downloads
	if err := bc.checkRateLimitsWithExtendedGrace(); err != nil {
		return err
	}

	return nil
}

// CheckFreeNonBlocking performs a non-blocking check of rate limits without retrying.
// Returns nil if a request slot is available immediately, otherwise returns an error.
// This is useful for pre-flight checks before queuing work.
func (bc *BaseClient) CheckFreeNonBlocking() error {
	// Check circuit breaker
	if !bc.circuitBreaker.CanMakeRequest() {
		return fmt.Errorf("circuit breaker is open for %s", bc.config.Name)
	}

	// Check server-side rate limiting (HTTP 429)
	if err := bc.checkServerRateLimit(); err != nil {
		return err
	}

	// Check rate limits non-blocking (no grace period retry)
	if err := bc.checkRateLimitsNonBlocking(); err != nil {
		return err
	}

	return nil
}

// MakeRequest - Global request handler with all infrastructure
//
// This is the core method that handles:
// - Rate limiting
// - Circuit breaker
// - Authentication
// - Statistics
// - Retries
// - Type-safe response parsing.
func (bc *BaseClient) MakeRequest(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
	target any,
	targetfunc func(*http.Response) error,
) error {
	return bc.MakeRequestWithHeaders(ctx, method, endpoint, body, target, targetfunc, nil)
}

// MakeRequestForDownload - Request handler specifically for download operations
//
// This is identical to MakeRequest but uses extended 2-minute grace period for rate limiting.
// Downloads are rare and critical, so they get priority treatment.
func (bc *BaseClient) MakeRequestForDownload(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
	target any,
	targetfunc func(*http.Response) error,
) error {
	return bc.MakeRequestWithGracePeriod(ctx, method, endpoint, body, target, targetfunc, 120*time.Second)
}

// MakeRequestWithGracePeriod - Request handler with configurable grace period for rate limiting
//
// Parameters:
//   - gracePeriod: Maximum time to wait for rate limit slots (e.g., 30s, 120s, 5m)
//
// This allows flexible rate limit handling for different operation priorities.
// Higher gracePeriod = higher priority (more willing to wait for execution).
func (bc *BaseClient) MakeRequestWithGracePeriod(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
	target any,
	targetfunc func(*http.Response) error,
	gracePeriod time.Duration,
) error {
	// Check circuit breaker
	if !bc.circuitBreaker.CanMakeRequest() {
		cbState := bc.circuitBreaker.GetState()
		failureCount := bc.circuitBreaker.GetFailureCount()

		logger.Logtype(logger.StatusDebug, 0).
			Str("client", bc.config.Name).
			Str("circuit_state", cbState).
			Int("failure_count", failureCount).
			Msg("Circuit breaker blocked request")

		return fmt.Errorf("circuit breaker is open for %s", bc.config.Name)
	}

	// Check server-side rate limiting (HTTP 429)
	if err := bc.checkServerRateLimit(); err != nil {
		return err
	}

	// Check rate limits with configurable grace period
	if err := bc.checkRateLimitsWithGracePeriod(gracePeriod); err != nil {
		return err
	}

	// Consume rate limit tokens right before making the request (AllowForce bypasses checks)
	if bc.rateLimiterHour != nil {
		bc.rateLimiterHour.AllowForce()
	}

	if bc.rateLimiter24h != nil {
		bc.rateLimiter24h.AllowForce()
	}

	// Prepare request
	// If endpoint is already a full URL (starts with http:// or https://), use it directly
	// Otherwise, concatenate with BaseURL
	url := endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		url = bc.config.BaseURL + strings.ReplaceAll(endpoint, bc.config.BaseURL, "")
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("[%s] failed to create request: %w", bc.config.Name, err)
	}

	// Apply authentication
	if err := bc.applyAuth(req); err != nil {
		return fmt.Errorf("[%s] authentication failed: %w", bc.config.Name, err)
	}

	// Set default headers
	req.Header.Set("User-Agent", bc.getUserAgent())
	// For downloads, accept any content type (*/*) instead of just application/json
	// This allows downloading NZB files (application/x-nzb+xml) and other binary content
	req.Header.Set("Accept", "*/*")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if bc.config.EnableCompression {
		req.Header.Set("Accept-Encoding", "gzip, deflate")
	}

	// Execute request with retries
	var (
		resp   *http.Response
		reqErr error
	)

	startTime := time.Now()

	for attempt := 0; attempt <= bc.config.MaxRetries; attempt++ {
		resp, reqErr = bc.httpClient.Do(req)
		if reqErr == nil && resp.StatusCode < 500 {
			break // Success or client error (4xx) - don't retry
		}

		if attempt < bc.config.MaxRetries {
			backoff := bc.config.RetryBackoff * time.Duration(attempt+1)
			logger.Logtype(logger.StatusDebug, 2).
				Err(reqErr).
				Str("url", req.RequestURI).
				Str("client", bc.config.Name).
				Int("attempt", attempt+1).
				Dur("backoff", backoff).
				Msg("Retrying request")
			time.Sleep(backoff)
		}
	}

	duration := time.Since(startTime)

	if reqErr != nil {
		bc.circuitBreaker.RecordFailure()
		cbState := bc.circuitBreaker.GetState()

		logger.Logtype(logger.StatusDebug, 0).
			Str("client", bc.config.Name).
			Err(reqErr).
			Int("threshold", bc.config.CircuitBreakerThreshold).
			Str("circuit_state", cbState).
			Str("url", req.RequestURI).
			Msg("Circuit breaker: recorded failure with request")

		bc.recordStats(false, duration.Milliseconds(), reqErr.Error())

		return fmt.Errorf(
			"[%s] request failed after %d attempts: %w",
			bc.config.Name,
			bc.config.MaxRetries+1,
			reqErr,
		)
	}

	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))

		// Handle rate limiting responses (429, 400, 401, 403)
		if resp.StatusCode == 429 || resp.StatusCode == 400 || resp.StatusCode == 401 ||
			resp.StatusCode == 403 {
			bc.handleRateLimitResponse(resp, bodyBytes)
			bc.circuitBreaker.RecordSuccess() // Don't penalize circuit breaker for rate limits
			bc.recordRequestOnly(duration.Milliseconds())

			return fmt.Errorf(
				"[%s] server rate limit (HTTP %d), request paused",
				bc.config.Name,
				resp.StatusCode,
			)
		}

		// Classify errors for circuit breaker
		shouldCountAsFailure := resp.StatusCode >= 500 ||
			resp.StatusCode == 401 ||
			resp.StatusCode == 403 ||
			resp.StatusCode == 408 ||
			resp.StatusCode == 425

		if shouldCountAsFailure {
			bc.circuitBreaker.RecordFailure()
			failureCount := bc.circuitBreaker.GetFailureCount()
			cbState := bc.circuitBreaker.GetState()

			logger.Logtype(logger.StatusDebug, 0).
				Str("client", bc.config.Name).
				Int("status_code", resp.StatusCode).
				Int("failure_count", failureCount).
				Int("threshold", bc.config.CircuitBreakerThreshold).
				Str("circuit_state", cbState).
				Msg("Circuit breaker: recorded failure on request")
		} else {
			bc.circuitBreaker.RecordSuccess()
		}

		bc.recordStats(resp.StatusCode < 500, duration.Milliseconds(), errMsg)

		return fmt.Errorf("[%s] %s", bc.config.Name, errMsg)
	}

	// Parse response into target
	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			bc.circuitBreaker.RecordSuccess()
			bc.recordStats(true, duration.Milliseconds(), "json decode error")
			return fmt.Errorf("[%s] failed to decode response: %w", bc.config.Name, err)
		}
	}

	if targetfunc != nil {
		if err := targetfunc(resp); err != nil {
			bc.circuitBreaker.RecordSuccess()
			bc.recordStats(true, duration.Milliseconds(), "response processing error")
			return err
		}
	}

	// Record success
	bc.circuitBreaker.RecordSuccess()
	bc.recordStats(true, duration.Milliseconds(), "")

	return nil
}

// MakeRequestWithHeaders executes an HTTP request with custom headers and comprehensive error handling.
// This method provides full infrastructure support including rate limiting, circuit breaker,
// retries, and statistics tracking while allowing custom headers.
func (bc *BaseClient) MakeRequestWithHeaders(
	ctx context.Context,
	method, endpoint string,
	body io.Reader,
	target any,
	targetfunc func(*http.Response) error,
	customHeaders map[string]string,
) error {
	// Check circuit breaker
	if !bc.circuitBreaker.CanMakeRequest() {
		cbState := bc.circuitBreaker.GetState()
		failureCount := bc.circuitBreaker.GetFailureCount()

		logger.Logtype(logger.StatusDebug, 0).
			Str("client", bc.config.Name).
			Str("circuit_state", cbState).
			Int("failure_count", failureCount).
			Msg("Circuit breaker blocked request")

		return fmt.Errorf("circuit breaker is open for %s", bc.config.Name)
	}

	// Check server-side rate limiting (HTTP 429)
	if err := bc.checkServerRateLimit(); err != nil {
		return err
	}

	// Check rate limits
	if err := bc.checkRateLimits(); err != nil {
		return err
	}

	// Consume rate limit tokens right before making the request (AllowForce bypasses checks)
	if bc.rateLimiterHour != nil {
		bc.rateLimiterHour.AllowForce()
	}

	if bc.rateLimiter24h != nil {
		bc.rateLimiter24h.AllowForce()
	}

	// Prepare request
	// If endpoint is already a full URL (starts with http:// or https://), use it directly
	// Otherwise, concatenate with BaseURL
	url := endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		url = bc.config.BaseURL + strings.ReplaceAll(endpoint, bc.config.BaseURL, "")
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("[%s] failed to create request: %w", bc.config.Name, err)
	}

	// Apply authentication
	if err := bc.applyAuth(req); err != nil {
		return fmt.Errorf("[%s] authentication failed: %w", bc.config.Name, err)
	}

	// Set default headers
	req.Header.Set("User-Agent", bc.getUserAgent())
	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if bc.config.EnableCompression {
		req.Header.Set("Accept-Encoding", "gzip, deflate")
	}

	// Apply custom headers (can override defaults)
	for key, value := range customHeaders {
		req.Header.Set(key, value)
	}

	// Execute request with retries
	var (
		resp   *http.Response
		reqErr error
	)

	startTime := time.Now()

	for attempt := 0; attempt <= bc.config.MaxRetries; attempt++ {
		resp, reqErr = bc.httpClient.Do(req)
		if reqErr == nil && resp.StatusCode < 500 {
			break // Success or client error (4xx) - don't retry
		}

		if attempt < bc.config.MaxRetries {
			backoff := bc.config.RetryBackoff * time.Duration(attempt+1)
			logger.Logtype(logger.StatusDebug, 2).
				Err(reqErr).
				Str("url", req.RequestURI).
				Str("client", bc.config.Name).
				Int("attempt", attempt+1).
				Dur("backoff", backoff).
				Msg("Retrying request")
			time.Sleep(backoff)
		}
	}

	duration := time.Since(startTime)

	if reqErr != nil {
		bc.circuitBreaker.RecordFailure()

		cbState := bc.circuitBreaker.GetState()

		logger.Logtype(logger.StatusDebug, 0).
			Str("client", bc.config.Name).
			Int("status_code", resp.StatusCode).
			Err(reqErr).
			Int("threshold", bc.config.CircuitBreakerThreshold).
			Str("circuit_state", cbState).
			Str("url", req.RequestURI).
			Msg("Circuit breaker: recorded failure with request")

		bc.recordStats(false, duration.Milliseconds(), reqErr.Error())

		return fmt.Errorf(
			"[%s] request failed after %d attempts: %w",
			bc.config.Name,
			bc.config.MaxRetries+1,
			reqErr,
		)
	}

	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))

		// Handle rate limiting responses (429, 400, 401, 403)
		// These status codes can indicate rate limits with Retry-After headers or body content
		if resp.StatusCode == 429 || resp.StatusCode == 400 || resp.StatusCode == 401 ||
			resp.StatusCode == 403 {
			bc.handleRateLimitResponse(resp, bodyBytes)
			bc.circuitBreaker.RecordSuccess() // Don't penalize circuit breaker for rate limits
			// Record that we made a request without counting it as success or failure
			bc.recordRequestOnly(duration.Milliseconds())

			return fmt.Errorf(
				"[%s] server rate limit (HTTP %d), requests paused",
				bc.config.Name,
				resp.StatusCode,
			)
		}

		// Classify errors for circuit breaker:
		// - 5xx errors (server errors) → always count as failure
		// - 401, 403 (auth errors) → count as failure (persistent configuration issues)
		// - 408, 425, 502, 503, 504 → count as failure (transient but should trigger breaker)
		// - 404, 400, etc. → count as success (client error, not server's fault)
		shouldCountAsFailure := resp.StatusCode >= 500 ||
			resp.StatusCode == 401 ||
			resp.StatusCode == 403 ||
			resp.StatusCode == 408 ||
			resp.StatusCode == 425

		if shouldCountAsFailure {
			bc.circuitBreaker.RecordFailure()

			failureCount := bc.circuitBreaker.GetFailureCount()
			cbState := bc.circuitBreaker.GetState()

			logger.Logtype(logger.StatusDebug, 0).
				Str("client", bc.config.Name).
				Int("status_code", resp.StatusCode).
				Int("failure_count", failureCount).
				Int("threshold", bc.config.CircuitBreakerThreshold).
				Str("circuit_state", cbState).
				Msg("Circuit breaker: recorded failure")

			// Log when circuit opens
			if cbState == string(StateOpen) && failureCount == bc.config.CircuitBreakerThreshold {
				logger.Logtype(logger.StatusWarning, 0).
					Str("client", bc.config.Name).
					Int("failure_count", failureCount).
					Int("threshold", bc.config.CircuitBreakerThreshold).
					Float64("timeout_seconds", bc.config.CircuitBreakerTimeout.Seconds()).
					Msg("Circuit breaker OPENED - requests will be blocked")
			}
		} else {
			bc.circuitBreaker.RecordSuccess()
		}

		bc.recordStats(resp.StatusCode < 500, duration.Milliseconds(), errMsg)

		return fmt.Errorf("[%s] %s", bc.config.Name, errMsg)
	}

	// Parse response into target
	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			// JSON decode errors are application-level parsing issues, not infrastructure failures
			// The HTTP request succeeded (200 OK), so don't penalize the circuit breaker
			bc.circuitBreaker.RecordSuccess()

			cbState := bc.circuitBreaker.GetState()

			logger.Logtype(logger.StatusDebug, 0).
				Str("client", bc.config.Name).
				Int("status_code", resp.StatusCode).
				Err(err).
				Int("threshold", bc.config.CircuitBreakerThreshold).
				Str("circuit_state", cbState).
				Str("url", req.RequestURI).
				Msg("JSON decode error (not a circuit breaker failure)")

			bc.recordStats(true, duration.Milliseconds(), "json decode error")

			return fmt.Errorf("[%s] failed to decode response: %w", bc.config.Name, err)
		}
	}

	if targetfunc != nil {
		if err := targetfunc(resp); err != nil {
			// targetfunc errors are application-level (parsing, validation), not infrastructure failures
			// The HTTP request succeeded (200 OK), so don't penalize the circuit breaker
			bc.circuitBreaker.RecordSuccess()

			cbState := bc.circuitBreaker.GetState()

			logger.Logtype(logger.StatusDebug, 0).
				Str("client", bc.config.Name).
				Int("status_code", resp.StatusCode).
				Err(err).
				Int("threshold", bc.config.CircuitBreakerThreshold).
				Str("url", req.RequestURI).
				Str("circuit_state", cbState).
				Msg("Response processing returned error (not a circuit breaker failure)")

			bc.recordStats(true, duration.Milliseconds(), "response processing error")

			return err
		}
	}

	// Record success
	bc.circuitBreaker.RecordSuccess()
	// cbState := bc.circuitBreaker.GetState()
	// logger.Logtype(logger.StatusDebug, 0).
	// 	Str("client", bc.config.Name).
	// 	Int("status_code", resp.StatusCode).
	// 	Int("threshold", bc.config.CircuitBreakerThreshold).
	// 	Str("circuit_state", cbState).
	// 	Msg("Circuit breaker: recorded success")

	bc.recordStats(true, duration.Milliseconds(), "")

	return nil
}

// checkServerRateLimit checks if we're currently blocked by a server-side rate limit (HTTP 429).
func (bc *BaseClient) checkServerRateLimit() error {
	// Check if we have a rate limit timestamp stored
	if limitTime := bc.rateLimitedUntil.Load(); limitTime != nil {
		until := limitTime.(time.Time)
		now := time.Now()

		if now.Before(until) {
			waitTime := until.Sub(now)
			// logger.Logtype(logger.StatusWarning, 1).
			// 	Str("client", bc.config.Name).
			// 	Dur("wait_time", waitTime).
			// 	Time("rate_limited_until", until).
			// 	Msg("Skipping request due to server rate limit (HTTP 429)")

			return fmt.Errorf(
				"[%s] server rate limit active, retry after %v",
				bc.config.Name,
				waitTime,
			)
		}

		// Rate limit has expired, clear it
		bc.rateLimitedUntil.Store(time.Time{})
	}

	return nil
}

// handleRateLimitResponse processes rate limit responses (429, 400, 401, 403) and extracts retry timing
//
// Retry timing can come from:
// - Retry-After or X-Retry-After header (integer seconds, RFC1123/RFC3339 date, or custom format)
// - Response body containing "Request limit reached" (for status 400 only)
// - Default fallback to 5 minutes
//
// This matches the legacy apiexternal/general.go addwait() behavior.
func (bc *BaseClient) handleRateLimitResponse(resp *http.Response, bodyBytes []byte) {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		retryAfter = resp.Header.Get("X-Retry-After")
	}

	now := time.Now()

	var (
		until               time.Time
		rateLimiterInterval time.Duration
	)

	// Get the rate limiter interval to subtract from wait times (legacy behavior)

	if bc.rateLimiterHour != nil {
		rateLimiterInterval = bc.rateLimiterHour.Interval()
	}

	// If Retry-After header exists, parse it
	if retryAfter != "" {
		// Try parsing custom format: "Request limit reached. Retry in X minutes/hours"
		if strings.Contains(retryAfter, "Request limit reached. Retry in ") {
			timeStr := strings.TrimSpace(
				strings.Replace(retryAfter, "Request limit reached. Retry in ", "", 1),
			)

			timeStr = strings.TrimSuffix(timeStr, ".")

			parts := strings.Fields(timeStr)
			if len(parts) == 2 {
				if value, err := strconv.Atoi(parts[0]); err == nil {
					switch parts[1] {
					case "minute", "minutes":
						until = now.Add(time.Duration(value) * time.Minute)
					case "hour", "hours":
						until = now.Add(time.Duration(value) * time.Hour)
					case "second", "seconds":
						until = now.Add(time.Duration(value) * time.Second)
					}

					if !until.IsZero() {
						logger.Logtype(logger.StatusWarning, 0).
							Str("client", bc.config.Name).
							Str("retry_message", retryAfter).
							Time("rate_limited_until", until).
							Msg("Server rate limit detected with custom retry message")

						bc.rateLimitedUntil.Store(until)
						bc.stats.mu.Lock()

						bc.stats.NextAvailableAt = until
						bc.stats.mu.Unlock()

						return
					}
				}
			}
			// Custom format parse failed, fall through to default
		} else if seconds, err := strconv.Atoi(retryAfter); err == nil {
			// Parse as integer seconds - subtract rate limiter interval (legacy behavior)
			until = now.Add(time.Duration(seconds)*time.Second - rateLimiterInterval)
			logger.Logtype(logger.StatusWarning, 0).
				Str("client", bc.config.Name).
				Dur("retry_after", time.Duration(seconds)*time.Second).
				Time("rate_limited_until", until).
				Msg("Server rate limit detected")

			bc.rateLimitedUntil.Store(until)
			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = until
			bc.stats.mu.Unlock()

			return
		} else if strings.ContainsRune(retryAfter, ' ') && strings.ContainsRune(retryAfter, ':') {
			// Try parsing as RFC1123 date
			if parsedTime, err := time.Parse(time.RFC1123, retryAfter); err == nil {
				until = parsedTime.Add(-rateLimiterInterval)
				logger.Logtype(logger.StatusWarning, 0).
					Str("client", bc.config.Name).
					Time("rate_limited_until", until).
					Msg("Server rate limit detected (RFC1123)")

				bc.rateLimitedUntil.Store(until)
				bc.stats.mu.Lock()

				bc.stats.NextAvailableAt = until
				bc.stats.mu.Unlock()

				return
			}
		} else if strings.ContainsRune(retryAfter, 'T') && strings.ContainsRune(retryAfter, ':') {
			// Try parsing as RFC3339 date
			if parsedTime, err := time.Parse(time.RFC3339, retryAfter); err == nil {
				until = parsedTime.Add(-rateLimiterInterval)
				logger.Logtype(logger.StatusWarning, 0).
					Str("client", bc.config.Name).
					Time("rate_limited_until", until).
					Msg("Server rate limit detected (RFC3339)")

				bc.rateLimitedUntil.Store(until)
				bc.stats.mu.Lock()

				bc.stats.NextAvailableAt = until
				bc.stats.mu.Unlock()

				return
			}
		}

		// Header exists but couldn't parse it - use default 5 minutes
		until = now.Add(5 * time.Minute)

		logger.Logtype(logger.StatusWarning, 0).
			Str("client", bc.config.Name).
			Str("retry_after_header", retryAfter).
			Msg("Server rate limit detected, unparseable Retry-After header, using 5min default")

		bc.rateLimitedUntil.Store(until)
		bc.stats.mu.Lock()

		bc.stats.NextAvailableAt = until
		bc.stats.mu.Unlock()

		return
	}

	// No Retry-After header - check body for "Request limit reached" (status 400 only)
	if resp.StatusCode == 400 && len(bodyBytes) > 0 {
		if bytes.Contains(bytes.ToLower(bodyBytes), []byte("request limit reached")) {
			// Daily limit hit - wait 3 hours (legacy behavior)
			until = now.Add(3 * time.Hour)

			logger.Logtype(logger.StatusWarning, 0).
				Str("client", bc.config.Name).
				Time("rate_limited_until", until).
				Msg("Daily rate limit reached (body contains 'Request limit reached'), waiting 3 hours")

			bc.rateLimitedUntil.Store(until)
			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = until
			bc.stats.mu.Unlock()

			return
		}
	}

	// No header, no body match - use default 5 minutes (not blocking, just return)
	// The circuit breaker will handle repeated failures
	logger.Logtype(logger.StatusDebug, 1).
		Str("client", bc.config.Name).
		Int("status_code", resp.StatusCode).
		Msg("Rate limit status code without retry information, relying on circuit breaker")
}

// checkRateLimits checks all rate limiters with automatic retry grace period
//
// This implements the 30-second grace period behavior from the legacy system:
// - If wait time is <= 30 seconds, retry every second until slot available
// - If wait time is > 30 seconds, return error immediately
// - This prevents excessive waiting while allowing short rate limit waits.
func (bc *BaseClient) checkRateLimits() error {
	const (
		maxGracePeriod = 30 * time.Second
		retryInterval  = 1 * time.Second
	)

	now := time.Now()

	// Check hourly limit with grace period retry
	if bc.rateLimiterHour != nil {
		allowed, waitTime := bc.rateLimiterHour.Check()
		if !allowed {
			// If wait time exceeds grace period, return error immediately
			if waitTime > maxGracePeriod {
				nextAvailable := now.Add(waitTime)

				bc.stats.mu.Lock()

				bc.stats.NextAvailableAt = nextAvailable
				bc.stats.mu.Unlock()

				return fmt.Errorf(
					"[%s] rate limit exceeded, next request available in %v (exceeds 30s grace period)",
					bc.config.Name,
					waitTime,
				)
			}

			// Wait time is within grace period - retry every second
			deadline := now.Add(waitTime)
			for time.Now().Before(deadline) {
				time.Sleep(retryInterval)

				allowed, newWaitTime := bc.rateLimiterHour.Check()
				if allowed {
					goto checkDailyLimit
				}

				waitTime = newWaitTime
			}

			// Grace period exhausted without getting slot
			nextAvailable := now.Add(waitTime)

			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = nextAvailable
			bc.stats.mu.Unlock()

			return fmt.Errorf(
				"[%s] rate limit exceeded, grace period exhausted, next request available in %v",
				bc.config.Name,
				waitTime,
			)
		}
	}

checkDailyLimit:
	// Check 24h limit with grace period retry (using Check() not Allow() to avoid consuming tokens during retry)
	if bc.rateLimiter24h != nil {
		allowed, waitTime := bc.rateLimiter24h.Check()
		if !allowed {
			// If wait time exceeds grace period, return error immediately
			if waitTime > maxGracePeriod {
				nextAvailable := now.Add(waitTime)

				bc.stats.mu.Lock()

				bc.stats.NextAvailableAt = nextAvailable
				bc.stats.mu.Unlock()

				return fmt.Errorf("[%s] daily rate limit exceeded, next request available in %v (exceeds 30s grace period)", bc.config.Name, waitTime)
			}

			// Wait time is within grace period - retry every second
			deadline := now.Add(waitTime)
			for time.Now().Before(deadline) {
				time.Sleep(retryInterval)

				allowed, newWaitTime := bc.rateLimiter24h.Check()
				if allowed {
					goto checkTotalLimit
				}

				waitTime = newWaitTime
			}

			// Grace period exhausted without getting slot
			nextAvailable := now.Add(waitTime)

			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = nextAvailable
			bc.stats.mu.Unlock()

			return fmt.Errorf("[%s] daily rate limit exceeded, grace period exhausted, next request available in %v", bc.config.Name, waitTime)
		}
	}

checkTotalLimit:
	// Check total limit (no retry for absolute limits)
	if bc.config.RateLimitPerTotal > 0 {
		currentTotal := atomic.LoadInt64(&bc.rateLimiterTotal)
		if currentTotal >= int64(bc.config.RateLimitPerTotal) {
			return fmt.Errorf("[%s] total rate limit exceeded (%d requests)", bc.config.Name, bc.config.RateLimitPerTotal)
		}

		atomic.AddInt64(&bc.rateLimiterTotal, 1)
	}

	return nil
}

// checkRateLimitsWithExtendedGrace checks all rate limiters with extended 2-minute grace period
//
// This is specifically for download operations which are rare and critical.
// - If wait time is <= 2 minutes, retry every second until slot available
// - If wait time is > 2 minutes, return error immediately
// - Downloads get priority execution (next in queue) rather than end of queue.
func (bc *BaseClient) checkRateLimitsWithExtendedGrace() error {
	return bc.checkRateLimitsWithGracePeriod(120 * time.Second)
}

// checkRateLimitsWithGracePeriod checks all rate limiters with a configurable grace period
//
// Parameters:
//   - gracePeriod: Maximum time to wait/retry for a rate limit slot (e.g., 30s, 120s)
//
// Behavior:
// - If wait time is <= gracePeriod, retry every second until slot available
// - If wait time is > gracePeriod, return error immediately
// - This allows flexible rate limit handling for different operation types.
func (bc *BaseClient) checkRateLimitsWithGracePeriod(gracePeriod time.Duration) error {
	const retryInterval = 1 * time.Second
	maxGracePeriod := gracePeriod

	now := time.Now()

	// Check hourly limit with grace period retry
	if bc.rateLimiterHour != nil {
		allowed, waitTime := bc.rateLimiterHour.Check()
		if !allowed {
			// If wait time exceeds grace period, return error immediately
			if waitTime > maxGracePeriod {
				nextAvailable := now.Add(waitTime)

				bc.stats.mu.Lock()

				bc.stats.NextAvailableAt = nextAvailable
				bc.stats.mu.Unlock()

				return fmt.Errorf(
					"[%s] rate limit exceeded, next request available in %v (exceeds %v grace period)",
					bc.config.Name,
					waitTime,
					gracePeriod,
				)
			}

			// Wait time is within grace period - retry every second
			deadline := now.Add(waitTime)
			for time.Now().Before(deadline) {
				time.Sleep(retryInterval)

				allowed, newWaitTime := bc.rateLimiterHour.Check()
				if allowed {
					goto checkDailyLimit
				}

				waitTime = newWaitTime
			}

			// Grace period exhausted without getting slot
			nextAvailable := now.Add(waitTime)

			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = nextAvailable
			bc.stats.mu.Unlock()

			return fmt.Errorf(
				"[%s] rate limit exceeded, grace period (%v) exhausted, next request available in %v",
				bc.config.Name,
				gracePeriod,
				waitTime,
			)
		}
	}

checkDailyLimit:
	// Check 24h limit with grace period retry
	if bc.rateLimiter24h != nil {
		allowed, waitTime := bc.rateLimiter24h.Check()
		if !allowed {
			// If wait time exceeds grace period, return error immediately
			if waitTime > maxGracePeriod {
				nextAvailable := now.Add(waitTime)

				bc.stats.mu.Lock()

				bc.stats.NextAvailableAt = nextAvailable
				bc.stats.mu.Unlock()

				return fmt.Errorf("[%s] daily rate limit exceeded, next request available in %v (exceeds %v grace period)", bc.config.Name, waitTime, gracePeriod)
			}

			// Wait time is within grace period - retry every second
			deadline := now.Add(waitTime)
			for time.Now().Before(deadline) {
				time.Sleep(retryInterval)

				allowed, newWaitTime := bc.rateLimiter24h.Check()
				if allowed {
					goto checkTotalLimit
				}

				waitTime = newWaitTime
			}

			// Grace period exhausted without getting slot
			nextAvailable := now.Add(waitTime)

			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = nextAvailable
			bc.stats.mu.Unlock()

			return fmt.Errorf("[%s] daily rate limit exceeded, grace period (%v) exhausted, next request available in %v", bc.config.Name, gracePeriod, waitTime)
		}
	}

checkTotalLimit:
	// Check total limit (no retry for absolute limits)
	if bc.config.RateLimitPerTotal > 0 {
		currentTotal := atomic.LoadInt64(&bc.rateLimiterTotal)
		if currentTotal >= int64(bc.config.RateLimitPerTotal) {
			return fmt.Errorf("[%s] total rate limit exceeded (%d requests)", bc.config.Name, bc.config.RateLimitPerTotal)
		}

		atomic.AddInt64(&bc.rateLimiterTotal, 1)
	}

	return nil
}

// checkRateLimitsNonBlocking checks all rate limiters without grace period retry.
// Returns immediately if rate limit is hit, without sleeping or retrying.
// This is useful for pre-flight checks before queuing work.
// Only rejects if wait time exceeds grace period (30s) - allows queuing if request will succeed soon.
func (bc *BaseClient) checkRateLimitsNonBlocking() error {
	now := time.Now()

	const maxGracePeriod = 31 * time.Second

	// Check rate limit - only reject if exceeds grace period
	if bc.rateLimiterHour != nil {
		allowed, waitTime := bc.rateLimiterHour.Check()
		if !allowed && waitTime > maxGracePeriod {
			nextAvailable := now.Add(waitTime)

			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = nextAvailable
			bc.stats.mu.Unlock()

			return fmt.Errorf(
				"[%s] rate limit exceeded, next request available in %v (exceeds grace period)",
				bc.config.Name,
				waitTime,
			)
		}
	}

	// Check 24h limit - only reject if exceeds grace period
	if bc.rateLimiter24h != nil {
		allowed, waitTime := bc.rateLimiter24h.Check()
		if !allowed && waitTime > maxGracePeriod {
			nextAvailable := now.Add(waitTime)

			bc.stats.mu.Lock()

			bc.stats.NextAvailableAt = nextAvailable
			bc.stats.mu.Unlock()

			return fmt.Errorf(
				"[%s] daily rate limit exceeded, next request available in %v (exceeds grace period)",
				bc.config.Name,
				waitTime,
			)
		}
	}

	// Check total limit (no retry for absolute limits)
	// NOTE: Does NOT increment counter - this is a read-only pre-flight check
	if bc.config.RateLimitPerTotal > 0 {
		currentTotal := atomic.LoadInt64(&bc.rateLimiterTotal)
		if currentTotal >= int64(bc.config.RateLimitPerTotal) {
			return fmt.Errorf(
				"[%s] total rate limit exceeded (%d requests)",
				bc.config.Name,
				bc.config.RateLimitPerTotal,
			)
		}
	}

	return nil
}

// applyAuth applies authentication to the request.
func (bc *BaseClient) applyAuth(req *http.Request) error {
	switch bc.config.AuthType {
	case AuthNone:
		return nil

	case AuthAPIKeyHeader:
		req.Header.Set(bc.config.APIKeyHeader, bc.config.APIKey)
		return nil

	case AuthAPIKeyURL:
		q := req.URL.Query()
		q.Set(bc.config.APIKeyParam, bc.config.APIKey)

		req.URL.RawQuery = q.Encode()

		return nil

	case AuthBasic:
		req.SetBasicAuth(bc.config.Username, bc.config.Password)
		return nil

	case AuthOAuth:
		// Get current token (may trigger refresh)
		token, err := bc.getValidOAuthToken(req.Context())
		if err != nil {
			return err
		}

		if token != nil {
			// Apply token to request
			token.SetAuthHeader(req)
		}

		return nil

	default:
		return nil
	}
}

// getValidOAuthToken returns a valid OAuth2 token, refreshing if necessary.
func (bc *BaseClient) getValidOAuthToken(ctx context.Context) (*oauth2.Token, error) {
	bc.oauthTokenLock.RLock()

	token := bc.oauthToken
	bc.oauthTokenLock.RUnlock()

	// Check if token needs refresh (5 minute buffer before expiry)
	if token == nil || !token.Valid() || time.Until(token.Expiry) < 5*time.Minute {
		// Need to refresh token
		if err := bc.refreshOAuthToken(ctx); err != nil {
			return nil, err
		}

		// Get refreshed token
		bc.oauthTokenLock.RLock()

		token = bc.oauthToken
		bc.oauthTokenLock.RUnlock()
	}

	return token, nil
}

// refreshOAuthToken refreshes the OAuth2 access token
//
// This method implements the complete OAuth2 token refresh flow:
//  1. Checks for provider-specific hooks (BeforeTokenRefresh)
//  2. Uses the refresh token to obtain a new access token
//  3. Calls provider-specific hooks (AfterTokenRefresh)
//  4. Saves the new token to persistent storage
//
// Supported OAuth2 Flows:
//   - Client Credentials Grant (for application-level authentication)
//   - Refresh Token Flow (for user-level authentication)
//
// Provider-Specific Implementations:
//   - Trakt.tv: Standard OAuth2 with refresh tokens
//   - Plex: Custom pin-based authentication flow (requires custom hooks)
//   - TMDB v4: Standard OAuth2 for user-specific operations
//
// Error Handling:
//   - Returns error if OAuth2 config is not initialized
//   - Returns error if token refresh fails
//   - Preserves old token on refresh failure for graceful degradation
func (bc *BaseClient) refreshOAuthToken(ctx context.Context) error {
	if bc.oauthConfig == nil {
		return fmt.Errorf("OAuth2 not configured for %s", bc.config.Name)
	}

	logger.Logtype(logger.StatusDebug, 1).
		Str("client", bc.config.Name).
		Msg("Refreshing OAuth2 token")

	bc.oauthTokenLock.Lock()
	defer bc.oauthTokenLock.Unlock()

	// Get current token for refresh
	currentToken := bc.oauthToken

	// Check for provider-specific hooks
	if hooks, exists := getOAuth2ProviderHooks(bc.config.Name); exists {
		if err := hooks.BeforeTokenRefresh(ctx, bc.oauthConfig, currentToken); err != nil {
			logger.Logtype(logger.StatusError, 0).
				Err(err).
				Str("client", bc.config.Name).
				Msg("Provider hook BeforeTokenRefresh failed")

			return fmt.Errorf("[%s] provider hook failed: %w", bc.config.Name, err)
		}
	}

	var (
		newToken *oauth2.Token
		err      error
	)

	if currentToken != nil && currentToken.RefreshToken != "" {
		// Use refresh token flow
		tokenSource := bc.oauthConfig.TokenSource(ctx, currentToken)

		newToken, err = tokenSource.Token()
		if err != nil {
			logger.Logtype(logger.StatusError, 0).
				Err(err).
				Str("client", bc.config.Name).
				Msg("Failed to refresh OAuth2 token using refresh token")

			return fmt.Errorf("[%s] failed to refresh token: %w", bc.config.Name, err)
		}
	} else {
		// Use client credentials flow (if no refresh token available)
		newToken, err = bc.oauthConfig.PasswordCredentialsToken(ctx, bc.config.Username, bc.config.Password)
		if err != nil {
			// Try client credentials grant using clientcredentials package
			ccConfig := &clientcredentials.Config{
				ClientID:     bc.config.OAuthClientID,
				ClientSecret: bc.config.OAuthClientSecret,
				TokenURL:     bc.config.OAuthTokenURL,
				Scopes:       bc.config.OAuthScopes,
			}

			newToken, err = ccConfig.Token(ctx)
			if err != nil {
				logger.Logtype(logger.StatusError, 0).
					Err(err).
					Str("client", bc.config.Name).
					Msg("Failed to obtain OAuth2 token using client credentials")

				return fmt.Errorf("[%s] failed to obtain token: %w", bc.config.Name, err)
			}
		}
	}

	// Call provider-specific post-refresh hook
	if hooks, exists := getOAuth2ProviderHooks(bc.config.Name); exists {
		if err := hooks.AfterTokenRefresh(ctx, newToken); err != nil {
			logger.Logtype(logger.StatusError, 0).
				Err(err).
				Str("client", bc.config.Name).
				Msg("Provider hook AfterTokenRefresh failed")

			return fmt.Errorf("[%s] provider hook failed: %w", bc.config.Name, err)
		}
	}

	// Update token in memory
	bc.oauthToken = newToken

	// Save token to persistent storage
	if err := defaultTokenStorage.SaveToken(bc.config.Name, newToken); err != nil {
		logger.Logtype(logger.StatusWarning, 0).
			Err(err).
			Str("client", bc.config.Name).
			Msg("Failed to save OAuth2 token to storage (token refresh succeeded)")
		// Don't fail the refresh if storage fails - token is still valid in memory
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("client", bc.config.Name).
		Time("expires", newToken.Expiry).
		Msg("OAuth2 token refreshed successfully")

	return nil
}

// SetOAuthToken sets the OAuth2 token manually (for initial authorization)
//
// This method is used to set the initial OAuth2 token after user authorization.
// Example: After completing the OAuth2 authorization code flow in a web browser.
func (bc *BaseClient) SetOAuthToken(token *oauth2.Token) error {
	if bc.config.AuthType != AuthOAuth {
		return fmt.Errorf("client %s is not configured for OAuth authentication", bc.config.Name)
	}

	bc.oauthTokenLock.Lock()

	bc.oauthToken = token
	bc.oauthTokenLock.Unlock()

	// Save to persistent storage
	if err := defaultTokenStorage.SaveToken(bc.config.Name, token); err != nil {
		logger.Logtype(logger.StatusWarning, 0).
			Err(err).
			Str("client", bc.config.Name).
			Msg("Failed to save OAuth2 token to storage")

		return err
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("client", bc.config.Name).
		Time("expires", token.Expiry).
		Msg("OAuth2 token set successfully")

	return nil
}

// GetOAuthToken returns the current OAuth2 token.
func (bc *BaseClient) GetOAuthToken() *oauth2.Token {
	bc.oauthTokenLock.RLock()
	defer bc.oauthTokenLock.RUnlock()
	return bc.oauthToken
}

// GetOAuthAuthorizationURL generates the OAuth2 authorization URL for user authorization
//
// This is used for the Authorization Code flow where users need to authorize
// the application in a web browser.
//
// Parameters:
//   - redirectURL: The URL where the OAuth provider will redirect after authorization
//   - state: A random string to prevent CSRF attacks (verify this when handling callback)
//
// Returns the authorization URL to redirect the user to.
func (bc *BaseClient) GetOAuthAuthorizationURL(redirectURL, state string) (string, error) {
	if bc.oauthConfig == nil {
		return "", fmt.Errorf("OAuth2 not configured for %s", bc.config.Name)
	}

	bc.oauthConfig.RedirectURL = redirectURL

	authURL := bc.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	logger.Logtype(logger.StatusInfo, 0).
		Str("client", bc.config.Name).
		Str("redirect_url", redirectURL).
		Msg("Generated OAuth2 authorization URL")

	return authURL, nil
}

// ExchangeOAuthCode exchanges an authorization code for an access token
//
// This is used after the user authorizes the application and is redirected
// back with an authorization code.
func (bc *BaseClient) ExchangeOAuthCode(ctx context.Context, code string) (*oauth2.Token, error) {
	if bc.oauthConfig == nil {
		return nil, fmt.Errorf("OAuth2 not configured for %s", bc.config.Name)
	}

	token, err := bc.oauthConfig.Exchange(ctx, code)
	if err != nil {
		logger.Logtype(logger.StatusError, 0).
			Err(err).
			Str("client", bc.config.Name).
			Msg("Failed to exchange OAuth2 authorization code")

		return nil, fmt.Errorf("[%s] failed to exchange code: %w", bc.config.Name, err)
	}

	// Save the token
	if err := bc.SetOAuthToken(token); err != nil {
		return nil, err
	}

	return token, nil
}

// recordStats records request statistics.
func (bc *BaseClient) recordStats(success bool, durationMs int64, errorMsg string) {
	bc.stats.mu.Lock()
	defer bc.stats.mu.Unlock()

	now := time.Now()

	bc.stats.LastRequestAt = now

	if success {
		bc.stats.SuccessCount++
	} else {
		bc.stats.FailureCount++

		bc.stats.LastErrorAt = now
		bc.stats.LastErrorMessage = errorMsg
	}

	// Update response time average
	bc.stats.totalResponseTimeMs += durationMs

	bc.stats.responseCount++
	if bc.stats.responseCount > 0 {
		bc.stats.AvgResponseTimeMs = bc.stats.totalResponseTimeMs / bc.stats.responseCount
	}

	bc.stats.CircuitBreakerState = bc.circuitBreaker.GetState()

	// Add timestamp to sliding window for time-based tracking
	bc.stats.requestTimestamps = append(bc.stats.requestTimestamps, now)

	// Clean up old timestamps periodically (every 100 requests to avoid overhead)
	if len(bc.stats.requestTimestamps)%100 == 0 {
		bc.cleanupOldTimestamps()
	}

	// Calculate time-based request counts from sliding window
	bc.stats.Requests1h = bc.countRequestsSince(now.Add(-1 * time.Hour))
	bc.stats.Requests24h = bc.countRequestsSince(now.Add(-24 * time.Hour))
	bc.stats.RequestsTotal++

	// Save to database if enabled
	if bc.config.EnableStats && bc.config.StatsDBTable != "" {
		go bc.saveStatsToDatabase()
	}
}

// recordRequestOnly records that a request was made without marking it as success or failure.
// This is used for rate limit responses (429) where we made a request but don't want to count it.
func (bc *BaseClient) recordRequestOnly(durationMs int64) {
	bc.stats.mu.Lock()
	defer bc.stats.mu.Unlock()

	now := time.Now()

	bc.stats.LastRequestAt = now

	// Update response time average
	bc.stats.totalResponseTimeMs += durationMs

	bc.stats.responseCount++
	if bc.stats.responseCount > 0 {
		bc.stats.AvgResponseTimeMs = bc.stats.totalResponseTimeMs / bc.stats.responseCount
	}

	bc.stats.CircuitBreakerState = bc.circuitBreaker.GetState()

	// Add timestamp to sliding window for time-based tracking
	bc.stats.requestTimestamps = append(bc.stats.requestTimestamps, now)

	// Clean up old timestamps periodically (every 100 requests to avoid overhead)
	if len(bc.stats.requestTimestamps)%100 == 0 {
		bc.cleanupOldTimestamps()
	}

	// Calculate time-based request counts from sliding window
	bc.stats.Requests1h = bc.countRequestsSince(now.Add(-1 * time.Hour))
	bc.stats.Requests24h = bc.countRequestsSince(now.Add(-24 * time.Hour))
	bc.stats.RequestsTotal++

	// Save to database if enabled
	if bc.config.EnableStats && bc.config.StatsDBTable != "" {
		go bc.saveStatsToDatabase()
	}
}

// cleanupOldTimestamps removes timestamps older than 24 hours.
// Must be called with bc.stats.mu already locked.
func (bc *BaseClient) cleanupOldTimestamps() {
	cutoff := time.Now().Add(-24 * time.Hour)
	validIdx := 0

	// Find first timestamp within the 24h window
	for validIdx < len(bc.stats.requestTimestamps) && bc.stats.requestTimestamps[validIdx].Before(cutoff) {
		validIdx++
	}

	// Keep only timestamps within the 24h window
	if validIdx > 0 {
		bc.stats.requestTimestamps = bc.stats.requestTimestamps[validIdx:]
	}
}

// countRequestsSince counts requests since a given time.
// Must be called with bc.stats.mu already locked.
func (bc *BaseClient) countRequestsSince(since time.Time) int64 {
	count := int64(0)
	for i := len(bc.stats.requestTimestamps) - 1; i >= 0; i-- {
		// Count timestamps that are at or after the since time
		if bc.stats.requestTimestamps[i].Before(since) {
			// We've reached timestamps older than our window, stop counting
			break
		}

		count++
	}

	return count
}

// saveStatsToDatabase persists statistics to the database.
func (bc *BaseClient) saveStatsToDatabase() {
	// bc.stats.mu.RLock()
	// defer bc.stats.mu.RUnlock()

	// query := fmt.Sprintf(`
	// 	INSERT OR REPLACE INTO %s (
	// 		provider_name, requests_1h, requests_24h, requests_total,
	// 		avg_response_time_ms, last_request_at, last_error_at, last_error_message,
	// 		next_available_at, success_count, failure_count, circuit_breaker_state,
	// 		updated_at
	// 	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	// `, bc.config.StatsDBTable)

	// err := database.ExecNErr(query,
	// 	bc.config.Name,
	// 	bc.stats.Requests1h,
	// 	bc.stats.Requests24h,
	// 	bc.stats.RequestsTotal,
	// 	bc.stats.AvgResponseTimeMs,
	// 	bc.stats.LastRequestAt,
	// 	bc.stats.LastErrorAt,
	// 	bc.stats.LastErrorMessage,
	// 	bc.stats.NextAvailableAt,
	// 	bc.stats.SuccessCount,
	// 	bc.stats.FailureCount,
	// 	bc.stats.CircuitBreakerState,
	// 	time.Now(),
	// )
	// if err != nil {
	// 	logger.Logtype(logger.StatusError, 0).
	// 		Err(err).
	// 		Str("client", bc.config.Name).
	// 		Msg("Failed to save stats to database")
	// }
}

// GetStats returns current statistics.
// Returns a copy of the stats without the mutex to avoid copying locks.
func (bc *BaseClient) GetStats() ClientStats {
	bc.stats.mu.RLock()
	defer bc.stats.mu.RUnlock()

	// Recalculate time-based counts based on current time for accurate sliding window
	now := time.Now()
	requests1h := bc.countRequestsSince(now.Add(-1 * time.Hour))
	requests24h := bc.countRequestsSince(now.Add(-24 * time.Hour))

	// Create a copy without the mutex
	return ClientStats{
		Requests1h:          requests1h,
		Requests24h:         requests24h,
		RequestsTotal:       bc.stats.RequestsTotal,
		AvgResponseTimeMs:   bc.stats.AvgResponseTimeMs,
		LastRequestAt:       bc.stats.LastRequestAt,
		LastErrorAt:         bc.stats.LastErrorAt,
		LastErrorMessage:    bc.stats.LastErrorMessage,
		NextAvailableAt:     bc.stats.NextAvailableAt,
		SuccessCount:        bc.stats.SuccessCount,
		FailureCount:        bc.stats.FailureCount,
		CircuitBreakerState: bc.stats.CircuitBreakerState,
		totalResponseTimeMs: bc.stats.totalResponseTimeMs,
		responseCount:       bc.stats.responseCount,
	}
}

// GetHTTPClient returns the underlying HTTP client for direct use when needed
// Note: Using this bypasses rate limiting and circuit breaker - use with caution.
func (bc *BaseClient) GetHTTPClient() *http.Client {
	return bc.httpClient
}

// getUserAgent returns the user agent string.
func (bc *BaseClient) getUserAgent() string {
	if bc.config.UserAgent != "" {
		return bc.config.UserAgent
	}
	return fmt.Sprintf("GoMediaDownloader/%s", bc.config.Name)
}

// GetName returns the client name.
func (bc *BaseClient) GetName() string {
	return bc.config.Name
}

// Close cleans up resources.
func (bc *BaseClient) Close() error {
	// Unsubscribe from config changes
	if bc.configUnsubscribe != nil {
		bc.configUnsubscribe()

		bc.configUnsubscribe = nil
	}

	// Save final stats
	if bc.config.EnableStats {
		bc.saveStatsToDatabase()
	}

	logger.Logtype(logger.StatusInfo, 0).
		Str("client", bc.config.Name).
		Msg("BaseClient closed")

	return nil
}
