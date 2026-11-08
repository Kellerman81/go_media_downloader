package providers

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/aria2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deluge"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/downloadstation"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/flood"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/nzbget"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/nzbvortex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/qbittorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/rqbit"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/rtorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sabnzbd"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/transmission"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tribler"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/utorrent"
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

// DownloadClientType describes one downloader type.
type DownloadClientType struct {
	Name        string // the config's DlType
	Label       string
	DefaultPort int
	// Torrent clients get DelugeDlTo as their save path.
	Torrent bool
}

// downloadClientTypes lists every supported downloader type, in the order
// the config screens offer them.
var downloadClientTypes = []DownloadClientType{
	{Name: "drone", Label: "Drone (file only, no client)"},
	{Name: "nzbget", Label: "NZBGet", DefaultPort: 6789},
	{Name: "sabnzbd", Label: "SABnzbd", DefaultPort: 8080},
	{Name: "nzbvortex", Label: "NZBVortex", DefaultPort: 4321},
	{Name: "qbittorrent", Label: "qBittorrent", DefaultPort: 8080, Torrent: true},
	{Name: "deluge", Label: "Deluge", DefaultPort: 8112, Torrent: true},
	{Name: "transmission", Label: "Transmission", DefaultPort: 9091, Torrent: true},
	{Name: "vuze", Label: "Vuze (Transmission RPC)", DefaultPort: 9091, Torrent: true},
	{Name: "rtorrent", Label: "rTorrent", DefaultPort: 5000, Torrent: true},
	{Name: "aria2", Label: "aria2", DefaultPort: 6800, Torrent: true},
	{Name: "flood", Label: "Flood", DefaultPort: 3000, Torrent: true},
	{Name: "utorrent", Label: "uTorrent", DefaultPort: 8080, Torrent: true},
	{Name: "rqbit", Label: "rqbit", DefaultPort: 3030, Torrent: true},
	{Name: "tribler", Label: "Tribler", DefaultPort: 20100, Torrent: true},
	{Name: "downloadstation", Label: "Synology Download Station", DefaultPort: 5000, Torrent: true},
}

// DownloadClientTypes returns every supported downloader type.
func DownloadClientTypes() []DownloadClientType {
	return slices.Clone(downloadClientTypes)
}

// DownloadClientTypeNames returns the DlType of every supported downloader.
func DownloadClientTypeNames() []string {
	names := make([]string, len(downloadClientTypes))
	for i, t := range downloadClientTypes {
		names[i] = t.Name
	}

	return names
}

// IsTorrentClient reports whether dlType is a torrent client.
func IsTorrentClient(dlType string) bool {
	i := slices.IndexFunc(downloadClientTypes, func(t DownloadClientType) bool { return t.Name == dlType })

	return i >= 0 && downloadClientTypes[i].Torrent
}

var errNoAPIKey = errors.New("no API key configured (Password field)")

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

	case "transmission", "vuze":
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

	case "nzbvortex":
		if dlCfg.Password == "" {
			return nil, fmt.Errorf("downloader %q: %w", dlCfg.Name, errNoAPIKey)
		}

		host, port, _, urlBase := base.SplitHostURL(dlCfg.Hostname, dlCfg.Port, 4321)

		return nzbvortex.NewProvider(host, port, dlCfg.Password, urlBase), nil

	case "aria2":
		// The Password field holds the --rpc-secret.
		host, port, useSSL, rpcPath := base.SplitHostURL(dlCfg.Hostname, dlCfg.Port, 6800)
		return aria2.NewProvider(host, port, dlCfg.Password, useSSL, rpcPath), nil

	case "flood":
		return flood.NewProvider(buildBaseURL(dlCfg.Hostname, dlCfg.Port, 3000), dlCfg.Username, dlCfg.Password)

	case "utorrent":
		return utorrent.NewProvider(buildBaseURL(dlCfg.Hostname, dlCfg.Port, 8080), dlCfg.Username, dlCfg.Password)

	case "rqbit":
		return rqbit.NewProvider(buildBaseURL(dlCfg.Hostname, dlCfg.Port, 3030), dlCfg.Username, dlCfg.Password), nil

	case "tribler":
		if dlCfg.Password == "" {
			return nil, fmt.Errorf("downloader %q: %w", dlCfg.Name, errNoAPIKey)
		}

		return tribler.NewProvider(buildBaseURL(dlCfg.Hostname, dlCfg.Port, 20100), dlCfg.Password), nil

	case "downloadstation":
		return downloadstation.NewProvider(
			buildBaseURL(dlCfg.Hostname, dlCfg.Port, 5000),
			dlCfg.Username,
			dlCfg.Password,
		), nil
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

	set := make(map[string]any)

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

		set[name] = client

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
