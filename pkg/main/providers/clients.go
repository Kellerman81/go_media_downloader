package providers

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deluge"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/nzbget"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/qbittorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/rtorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sabnzbd"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/transmission"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// errNoClient marks a downloader type without an API client (e.g. "drone").
var errNoClient = errors.New("no client")

var (
	// downloadRegMu serializes RegisterDownloadClients so overlapping
	// reloads can't publish an older config's set last.
	downloadRegMu sync.Mutex

	// indexerMu guards indexerConfigs, the settings each registered indexer
	// provider was built from.
	indexerMu      sync.Mutex
	indexerConfigs = make(map[string]newznab.ProviderConfig)
)

// ReloadClientProviders re-creates the download-client and indexer providers
// from the current config. Call it after every config load or save that can
// touch downloader_* or indexer_* entries.
func ReloadClientProviders() {
	RegisterDownloadClients()
	SyncIndexerProviders()
}

// NewDownloadClient builds the provider for one downloader config. The
// hostname may be bare or a full URL; a URL sub-path is kept as the client's
// reverse-proxy base path. The result is one of the *Provider types of the
// apiexternal_v2/providers download clients.
func NewDownloadClient(dlCfg *config.DownloaderConfig) (any, error) {
	if dlCfg.Hostname == "" {
		return nil, fmt.Errorf("downloader %q: no hostname configured", dlCfg.Name)
	}

	switch dlCfg.DlType {
	case "qbittorrent":
		host, port, useSSL, urlBase := base.SplitHostURL(dlCfg.Hostname, dlCfg.Port, 8080)
		return qbittorrent.NewProvider(host, port, dlCfg.Username, dlCfg.Password, useSSL, urlBase)

	case "deluge":
		return deluge.NewProvider(
			buildBaseURL(dlCfg.Hostname, dlCfg.Port, 8112),
			dlCfg.Username,
			dlCfg.Password,
		), nil

	case "transmission":
		return transmission.NewProvider(
			buildBaseURL(dlCfg.Hostname, dlCfg.Port, 9091),
			dlCfg.Username,
			dlCfg.Password,
		), nil

	case "rtorrent":
		host, port, useSSL, urlBase := base.SplitHostURL(dlCfg.Hostname, dlCfg.Port, 0)
		return rtorrent.NewProvider(host, port, dlCfg.Username, dlCfg.Password, useSSL, urlBase)

	case "sabnzbd":
		// The Password field holds the SABnzbd API key.
		host, port, useSSL, urlBase := base.SplitHostURL(dlCfg.Hostname, dlCfg.Port, 8080)
		return sabnzbd.NewProvider(host, port, dlCfg.Password, useSSL, urlBase)

	case "nzbget":
		host, port, useSSL, urlBase := base.SplitHostURL(dlCfg.Hostname, dlCfg.Port, 6789)
		return nzbget.NewProvider(host, port, dlCfg.Username, dlCfg.Password, useSSL, urlBase)
	}

	// e.g. "drone" (blackhole folder) has no API client.
	return nil, fmt.Errorf("downloader %q: type %q: %w", dlCfg.Name, dlCfg.DlType, errNoClient)
}

// buildBaseURL constructs a base URL from a configured hostname (bare or a
// full URL, see base.SplitHostURL) and port, keeping any sub-path.
// Example: ("localhost", 8080, 0) -> "http://localhost:8080".
func buildBaseURL(hostname string, cfgPort, defaultPort int) string {
	host, port, useSSL, path := base.SplitHostURL(hostname, cfgPort, defaultPort)

	protocol := "http"
	if useSSL {
		protocol = "https"
	}

	return protocol + "://" + host + ":" + strconv.Itoa(port) + path
}

// RegisterDownloadClients replaces the registered download clients with the
// enabled downloaders of the current config, so added, changed and removed
// downloaders take effect without a restart. A download already running
// keeps the provider instance it fetched.
func RegisterDownloadClients() {
	downloadRegMu.Lock()
	defer downloadRegMu.Unlock()

	set := DownloadProviders{
		QBittorrent:  make(map[string]*qbittorrent.Provider),
		Deluge:       make(map[string]*deluge.Provider),
		Transmission: make(map[string]*transmission.Provider),
		RTorrent:     make(map[string]*rtorrent.Provider),
		SABnzbd:      make(map[string]*sabnzbd.Provider),
		NZBGet:       make(map[string]*nzbget.Provider),
	}

	config.RangeSettingsDownloader(func(name string, dlCfg *config.DownloaderConfig) {
		if !dlCfg.Enabled || dlCfg.Hostname == "" {
			return
		}

		if dlCfg.DlType == "sabnzbd" && dlCfg.Password == "" {
			return
		}

		client, err := NewDownloadClient(dlCfg)
		if errors.Is(err, errNoClient) {
			return
		}

		if err != nil {
			logger.Logtype(logger.StatusWarning, 0).
				Err(err).
				Str("downloader", name).
				Msg("Download client not registered")

			return
		}

		switch p := client.(type) {
		case *qbittorrent.Provider:
			set.QBittorrent[name] = p
		case *deluge.Provider:
			set.Deluge[name] = p
		case *transmission.Provider:
			set.Transmission[name] = p
		case *rtorrent.Provider:
			set.RTorrent[name] = p
		case *sabnzbd.Provider:
			set.SABnzbd[name] = p
		case *nzbget.Provider:
			set.NZBGet[name] = p
		}

		logger.Logtype(logger.StatusDebug, 0).
			Str("downloader", name).
			Str("type", dlCfg.DlType).
			Msg("Registered download provider")
	})

	ReplaceDownloadProviders(set)
}

// newznabProviderConfig maps an indexer config to the newznab provider
// settings, applying the default rate limits.
func newznabProviderConfig(idxCfg *config.IndexersConfig) newznab.ProviderConfig {
	limiterCalls := idxCfg.Limitercalls
	if limiterCalls == 0 {
		limiterCalls = 5
	}

	limiterSeconds := idxCfg.Limiterseconds
	if limiterSeconds == 0 {
		limiterSeconds = 20
	}

	return newznab.ProviderConfig{
		IndexerName:       idxCfg.Name,
		BaseURL:           idxCfg.URL,
		APIKey:            idxCfg.Apikey,
		OutputAsJSON:      idxCfg.OutputAsJSON,
		TimeoutSeconds:    idxCfg.TimeoutSeconds,
		CustomAPI:         idxCfg.Customapi,
		CustomURL:         idxCfg.Customurl,
		LimiterCalls:      limiterCalls,
		LimiterSeconds:    limiterSeconds,
		LimiterCallsDaily: idxCfg.LimitercallsDaily,
		Enabled:           idxCfg.Enabled,
	}
}

// SyncIndexerProviders brings the registered indexer providers in line with
// the current config: indexers whose settings changed are re-created,
// removed ones are unregistered. Unchanged indexers keep their provider, and
// with it their rate-limit state and statistics.
func SyncIndexerProviders() {
	configured := make(map[string]struct{})

	config.RangeSettingsIndexer(func(_ string, idxCfg *config.IndexersConfig) {
		configured[idxCfg.Name] = struct{}{}

		indexerMu.Lock()
		built, ok := indexerConfigs[idxCfg.Name]
		indexerMu.Unlock()

		if ok && built == newznabProviderConfig(idxCfg) && GetIndexer(idxCfg.Name) != nil {
			return
		}

		registerIndexer(idxCfg)
	})

	indexerMu.Lock()
	defer indexerMu.Unlock()

	for name := range GetAllIndexers() {
		if _, ok := configured[name]; !ok {
			RemoveIndexer(name)
			delete(indexerConfigs, name)
		}
	}
}

// registerIndexer (re)creates the provider for one indexer config.
func registerIndexer(idxCfg *config.IndexersConfig) *newznab.Provider {
	pcfg := newznabProviderConfig(idxCfg)
	provider := newznab.NewProvider(pcfg)

	indexerMu.Lock()
	defer indexerMu.Unlock()

	SetIndexer(idxCfg.Name, provider)
	indexerConfigs[idxCfg.Name] = pcfg

	return provider
}

// EnsureIndexer returns the registered provider for an indexer config,
// creating it on first use.
func EnsureIndexer(idxCfg *config.IndexersConfig) *newznab.Provider {
	if provider := GetIndexer(idxCfg.Name); provider != nil {
		return provider
	}

	return registerIndexer(idxCfg)
}
