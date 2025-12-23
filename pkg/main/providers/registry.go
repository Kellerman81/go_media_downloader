package providers

import (
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deluge"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/nzbget"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/omdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/qbittorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/rtorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sabnzbd"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/transmission"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvmaze"
)

//
// Registry - Direct typed provider storage
//
// This package sits above the apiexternal_v2 providers, so it can import them
// without creating import cycles. It provides type-safe global provider access.
//
// Example usage:
//
//	import "github.com/Kellerman81/go_media_downloader/pkg/main/providers"
//
//	// Get typed provider directly - no type assertion!
//	if omdbProvider := providers.GetOMDB(); omdbProvider != nil {
//	    details, err := omdbProvider.GetDetailsByIMDb(ctx, imdbID)
//	}
//

var (
	registryMutex sync.RWMutex

	// Metadata providers - stored as concrete types.
	omdbProvider   *omdb.Provider
	tmdbProvider   *tmdb.Provider
	traktProvider  *trakt.Provider
	tvdbProvider   *tvdb.Provider
	tvmazeProvider *tvmaze.Provider

	// Indexer providers - map by name for configuration-driven lookup.
	indexerProviders = make(map[string]*newznab.Provider)

	// Download providers - map by name
	// Stored as concrete types for full type safety.
	qbittorrentProviders  = make(map[string]*qbittorrent.Provider)
	delugeProviders       = make(map[string]*deluge.Provider)
	transmissionProviders = make(map[string]*transmission.Provider)
	rtorrentProviders     = make(map[string]*rtorrent.Provider)
	sabnzbdProviders      = make(map[string]*sabnzbd.Provider)
	nzbgetProviders       = make(map[string]*nzbget.Provider)
)

//
// Metadata Providers
//

// SetOMDB sets the global OMDB provider instance.
func SetOMDB(provider *omdb.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	omdbProvider = provider
}

// GetOMDB returns the global OMDB provider instance.
// Returns nil if not initialized.
func GetOMDB() *omdb.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return omdbProvider
}

// SetTMDB sets the global TMDB provider instance.
func SetTMDB(provider *tmdb.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	tmdbProvider = provider
}

// GetTMDB returns the global TMDB provider instance.
// Returns nil if not initialized.
func GetTMDB() *tmdb.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return tmdbProvider
}

// SetTrakt sets the global Trakt provider instance.
func SetTrakt(provider *trakt.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	traktProvider = provider
}

// GetTrakt returns the global Trakt provider instance.
// Returns nil if not initialized.
func GetTrakt() *trakt.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return traktProvider
}

// SetTVDB sets the global TVDB provider instance.
func SetTVDB(provider *tvdb.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	tvdbProvider = provider
}

// GetTVDB returns the global TVDB provider instance.
// Returns nil if not initialized.
func GetTVDB() *tvdb.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return tvdbProvider
}

// SetTVMaze sets the global TVMaze provider instance.
func SetTVMaze(provider *tvmaze.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	tvmazeProvider = provider
}

// GetTVMaze returns the global TVMaze provider instance.
// Returns nil if not initialized.
func GetTVMaze() *tvmaze.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return tvmazeProvider
}

//
// Indexer Providers
//

// SetIndexer registers an indexer provider by name.
func SetIndexer(name string, provider *newznab.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	indexerProviders[name] = provider
}

// GetIndexer returns an indexer provider by name.
// Returns nil if not found.
func GetIndexer(name string) *newznab.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return indexerProviders[name]
}

// GetAllIndexers returns all registered indexer providers.
func GetAllIndexers() map[string]*newznab.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	providers := make(map[string]*newznab.Provider, len(indexerProviders))
	for name, provider := range indexerProviders {
		providers[name] = provider
	}

	return providers
}

// GetAllIndexerDownloadClients returns the download clients from all indexer providers.
// These track download statistics separately from search statistics.
func GetAllIndexerDownloadClients() map[string]interface{} {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	clients := make(map[string]interface{})
	for name, provider := range indexerProviders {
		if provider != nil && provider.DownloadClient != nil {
			// Use the name from the DownloadClient which includes "_download" suffix
			clients[name+"_download"] = provider.DownloadClient
		}
	}

	return clients
}

//
// Download Providers - Type-specific registration
//

// SetQBittorrent registers a qBittorrent provider by name.
func SetQBittorrent(name string, provider *qbittorrent.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	qbittorrentProviders[name] = provider
}

// GetQBittorrent returns a qBittorrent provider by name.
func GetQBittorrent(name string) *qbittorrent.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return qbittorrentProviders[name]
}

// SetDeluge registers a Deluge provider by name.
func SetDeluge(name string, provider *deluge.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	delugeProviders[name] = provider
}

// GetDeluge returns a Deluge provider by name.
func GetDeluge(name string) *deluge.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return delugeProviders[name]
}

// SetTransmission registers a Transmission provider by name.
func SetTransmission(name string, provider *transmission.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	transmissionProviders[name] = provider
}

// GetTransmission returns a Transmission provider by name.
func GetTransmission(name string) *transmission.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return transmissionProviders[name]
}

// SetRTorrent registers an rTorrent provider by name.
func SetRTorrent(name string, provider *rtorrent.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	rtorrentProviders[name] = provider
}

// GetRTorrent returns an rTorrent provider by name.
func GetRTorrent(name string) *rtorrent.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return rtorrentProviders[name]
}

// SetSABnzbd registers a SABnzbd provider by name.
func SetSABnzbd(name string, provider *sabnzbd.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	sabnzbdProviders[name] = provider
}

// GetSABnzbd returns a SABnzbd provider by name.
func GetSABnzbd(name string) *sabnzbd.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return sabnzbdProviders[name]
}

// SetNZBGet registers an NZBGet provider by name.
func SetNZBGet(name string, provider *nzbget.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	nzbgetProviders[name] = provider
}

// GetNZBGet returns an NZBGet provider by name.
func GetNZBGet(name string) *nzbget.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return nzbgetProviders[name]
}

//
// GetAll Methods - Return all providers of each type
//

// GetAllMetadataProviders returns all registered metadata providers.
func GetAllMetadataProviders() map[string]interface{} {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	providers := make(map[string]interface{})
	if omdbProvider != nil {
		providers["omdb"] = omdbProvider
	}

	if tmdbProvider != nil {
		providers["tmdb"] = tmdbProvider
	}

	if traktProvider != nil {
		providers["trakt"] = traktProvider
	}

	if tvdbProvider != nil {
		providers["tvdb"] = tvdbProvider
	}

	if tvmazeProvider != nil {
		providers["tvmaze"] = tvmazeProvider
	}

	return providers
}

// GetAllDownloadProviders returns all registered download providers.
// This includes traditional download clients (qBittorrent, etc.) and
// indexer download clients (which track NZB file downloads separately from searches).
func GetAllDownloadProviders() map[string]interface{} {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	providers := make(map[string]interface{})
	for name, provider := range qbittorrentProviders {
		providers[name] = provider
	}

	for name, provider := range delugeProviders {
		providers[name] = provider
	}

	for name, provider := range transmissionProviders {
		providers[name] = provider
	}

	for name, provider := range rtorrentProviders {
		providers[name] = provider
	}

	for name, provider := range sabnzbdProviders {
		providers[name] = provider
	}

	for name, provider := range nzbgetProviders {
		providers[name] = provider
	}
	// Add indexer download clients (track NZB downloads separately from searches)
	for name, provider := range indexerProviders {
		if provider != nil && provider.DownloadClient != nil {
			providers[name+"_download"] = provider.DownloadClient
		}
	}

	return providers
}
