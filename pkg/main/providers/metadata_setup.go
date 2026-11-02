package providers

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/omdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvmaze"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// RegisterMetadataProviders creates the OMDB, TMDB, TVDB, TVmaze and Trakt
// providers from the general settings.
func RegisterMetadataProviders() {
	general := config.GetSettingsGeneral()

	RegisterOMDB(
		general.OmdbAPIKey,
		general.OmdbLimiterSeconds,
		general.OmdbLimiterCalls,
		general.OmdbDisableTLSVerify,
		general.OmdbTimeoutSeconds,
	)
	RegisterTMDB(
		general.TheMovieDBApiKey,
		general.TmdbLimiterSeconds,
		general.TmdbLimiterCalls,
		general.TheMovieDBDisableTLSVerify,
		general.TmdbTimeoutSeconds,
	)
	RegisterTVDB(
		general.TvdbLimiterSeconds,
		general.TvdbLimiterCalls,
		general.TvdbDisableTLSVerify,
		general.TvdbTimeoutSeconds,
	)
	RegisterTVMaze(
		general.TvmazeLimiterSeconds,
		general.TvmazeLimiterCalls,
		general.TvmazeDisableTLSVerify,
		general.TvmazeTimeoutSeconds,
	)
	RegisterTrakt(
		general.TraktClientID,
		general.TraktClientSecret,
		general.TraktLimiterSeconds,
		general.TraktLimiterCalls,
		general.TraktDisableTLSVerify,
		general.TraktTimeoutSeconds,
		general.TraktRedirectUrl,
	)
}

// RegisterOMDB creates and registers the OMDb provider. Unset rate limit and
// timeout settings get their documented defaults.
func RegisterOMDB(
	apikey string,
	seconds uint8,
	calls int,
	disabletls bool,
	timeoutseconds uint16,
) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	// 0 would mean no HTTP timeout at all; the config documents 10s as default.
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}

	// Create v2 provider
	omdbConfig := base.ClientConfig{
		Name:                      "omdb",
		BaseURL:                   "http://www.omdbapi.com",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthAPIKeyURL,
		APIKey:                    apikey,
		APIKeyParam:               "apikey",
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disabletls,
	}
	if provider := omdb.NewProviderWithConfig(omdbConfig); provider != nil {
		SetOMDB(provider)
	}
}

// RegisterTMDB creates and registers the TMDb provider.
func RegisterTMDB(
	apikey string,
	seconds uint8,
	calls int,
	disabletls bool,
	timeoutseconds uint16,
) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	// 0 would mean no HTTP timeout at all; the config documents 10s as default.
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}

	// Create v2 provider
	tmdbConfig := base.ClientConfig{
		Name:                      "tmdb",
		BaseURL:                   "https://api.themoviedb.org/3",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthNone, // TMDB handles Bearer token auth internally
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		RateLimitPer24h:           40000,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disabletls,
	}
	if provider := tmdb.NewProviderWithConfig(tmdbConfig, apikey); provider != nil {
		SetTMDB(provider)
		logger.Logtype(logger.StatusDebug, 0).
			Msg("Registered TMDB metadata provider with rate limiting")
	}
}

// RegisterTVDB creates and registers the TheTVDB provider.
func RegisterTVDB(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	// 0 would mean no HTTP timeout at all; the config documents 10s as default.
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}

	// Create v2 provider with empty credentials (will need to be set separately)
	tvdbConfig := base.ClientConfig{
		Name:                      "tvdb",
		BaseURL:                   "https://api.thetvdb.com",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthNone, // TVDB uses JWT, handled by provider
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disabletls,
	}
	// Note: apiKey, userKey, username would need to be provided from config
	if provider := tvdb.NewProviderWithConfig(tvdbConfig, "", "", ""); provider != nil {
		SetTVDB(provider)
	}
}

// RegisterTVMaze creates and registers the TVmaze provider (no API key).
func RegisterTVMaze(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	// 0 would mean no rate limiter at all; the config documents 1 call per
	// second and a 10s timeout as defaults, like the other metadata clients.
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	if timeoutseconds == 0 {
		timeoutseconds = 10
	}

	tvmazeConfig := base.ClientConfig{
		Name:                      "tvmaze",
		BaseURL:                   "https://api.tvmaze.com",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthNone,
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		RateLimitPer24h:           20000, // TVMaze has generous limits
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disabletls,
	}
	if provider := tvmaze.NewProviderWithConfig(tvmazeConfig); provider != nil {
		// Store in direct providers registry
		SetTVMaze(provider)

		logger.Logtype(logger.StatusDebug, 0).
			Msg("Registered TVMaze metadata provider with rate limiting")
	}
}

// RegisterTrakt creates and registers the Trakt provider with its OAuth2
// credentials.
func RegisterTrakt(
	clientid, clientsecret string,
	seconds uint8,
	calls int,
	disabletls bool,
	timeoutseconds uint16,
	redirecturl string,
) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	// 0 would mean no HTTP timeout at all; the config documents 10s as default.
	if timeoutseconds == 0 {
		timeoutseconds = 10
	}

	// Create v2 provider
	traktConfig := base.ClientConfig{
		Name:                      "trakt",
		BaseURL:                   "https://api.trakt.tv",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthNone, // Trakt provider handles OAuth internally via EnsureValidToken
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disabletls,
	}
	if provider := trakt.NewProviderWithConfig(
		traktConfig,
		clientid,
		clientsecret,
		redirecturl,
	); provider != nil {
		SetTrakt(provider)
		logger.Logtype(logger.StatusDebug, 0).Msg("Registered Trakt metadata provider with OAuth2")
	}
}
