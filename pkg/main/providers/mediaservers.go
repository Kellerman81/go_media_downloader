package providers

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/jellyfin"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/plex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

var (
	plexProvider     *plex.Provider
	jellyfinProvider *jellyfin.Provider
)

// GetPlex returns the Plex provider, nil before RegisterMediaServerProviders.
func GetPlex() *plex.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	return plexProvider
}

// GetJellyfin returns the Jellyfin provider, nil before
// RegisterMediaServerProviders.
func GetJellyfin() *jellyfin.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	return jellyfinProvider
}

// mediaServerLimiterDefaults applies the documented defaults (10 calls per
// second) to unset limiter settings.
func mediaServerLimiterDefaults(seconds uint8, calls int) (uint8, int) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 10
	}

	return seconds, calls
}

func mediaServerClientConfig(seconds uint8, calls int, disableTLS bool, timeoutSeconds uint16) base.ClientConfig {
	seconds, calls = mediaServerLimiterDefaults(seconds, calls)
	if timeoutSeconds == 0 {
		timeoutSeconds = 30
	}

	return base.ClientConfig{
		Timeout:                   time.Duration(timeoutSeconds) * time.Second,
		AuthType:                  base.AuthNone, // token/api key travel in the URL
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disableTLS,
	}
}

// RegisterMediaServerProviders creates the Plex and Jellyfin providers from
// the general settings.
func RegisterMediaServerProviders() {
	general := config.GetSettingsGeneral()

	p := plex.NewProviderWithConfig(mediaServerClientConfig(
		general.PlexLimiterSeconds, general.PlexLimiterCalls,
		general.PlexDisableTLSVerify, general.PlexTimeoutSeconds,
	))
	j := jellyfin.NewProviderWithConfig(mediaServerClientConfig(
		general.JellyfinLimiterSeconds, general.JellyfinLimiterCalls,
		general.JellyfinDisableTLSVerify, general.JellyfinTimeoutSeconds,
	))

	registryMutex.Lock()
	defer registryMutex.Unlock()

	plexProvider = p
	jellyfinProvider = j
}
