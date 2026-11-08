package providers

import (
	"maps"
	"reflect"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/acoustid"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deezer"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/discogs"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/goodreads"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/itunes"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/lastfm"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/omdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/theaudiodb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
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

	// Book/Audiobook/Music providers - stored as concrete types.
	acoustidProvider    *acoustid.Provider
	audibleProviders    = make(map[string]*audible.Provider) // Keyed by region
	audnexProvider      *audnex.Provider
	openlibraryProvider *openlibrary.Provider
	goodreadsProvider   *goodreads.Provider
	musicbrainzProvider *musicbrainz.Provider
	// musicbrainzFallback serves callers that need MusicBrainz even when it is
	// not in the music metadata priority list. It is kept apart from
	// musicbrainzProvider because GetMusicBrainz() != nil is used as the
	// "MusicBrainz enabled" check.
	musicbrainzFallback *musicbrainz.Provider
	lastfmProvider      *lastfm.Provider
	discogsProvider     *discogs.Provider
	deezerProvider      *deezer.Provider
	theaudiodbProvider  *theaudiodb.Provider
	itunesProvider      *itunes.Provider

	// Indexer providers - map by name for configuration-driven lookup.
	indexerProviders = make(map[string]*newznab.Provider)

	// Download providers - map by name
	// Stored as concrete types for full type safety.
	downloadProviders = make(map[string]any)
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

// SetIndexer registers an indexer provider by name. A nil provider removes
// the entry instead of storing a nil that GetAll* would hand out as a non-nil
// interface value.
func SetIndexer(name string, provider *newznab.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	setOrDelete(indexerProviders, name, provider)
}

// RemoveIndexer unregisters the indexer provider with the given name.
func RemoveIndexer(name string) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	delete(indexerProviders, name)
}

// setOrDelete stores provider under name, or deletes name when provider is nil.
func setOrDelete[T any](m map[string]*T, name string, provider *T) {
	if provider == nil {
		delete(m, name)
		return
	}

	m[name] = provider
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
	maps.Copy(providers, indexerProviders)

	return providers
}

//
// Download Providers
//

// ReplaceDownloadProviders swaps the whole set of registered download
// clients, keyed by downloader config name, in one step, so a concurrent
// lookup never sees a half-registered set. Nil entries are dropped. Callers
// still holding a replaced client keep a working instance.
func ReplaceDownloadProviders(set map[string]any) {
	clients := make(map[string]any, len(set))
	for name, p := range set {
		if p != nil && !reflect.ValueOf(p).IsNil() {
			clients[name] = p
		}
	}

	registryMutex.Lock()
	defer registryMutex.Unlock()

	downloadProviders = clients
}

// GetDownloadClient returns the client of the downloader config name, or nil
// (also for drone, which has none).
func GetDownloadClient(name string) any {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	return downloadProviders[name]
}

//
// GetAll Methods - Return all providers of each type
//

// GetAllMetadataProviders returns all registered metadata providers.
func GetAllMetadataProviders() map[string]any {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	providers := make(map[string]any)
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

	if audnexProvider != nil {
		providers["audnex"] = audnexProvider
	}

	if openlibraryProvider != nil {
		providers["openlibrary"] = openlibraryProvider
	}

	if goodreadsProvider != nil {
		providers["goodreads"] = goodreadsProvider
	}

	if musicbrainzProvider != nil {
		providers["musicbrainz"] = musicbrainzProvider
	}

	for region, p := range audibleProviders {
		if p != nil {
			providers["audible_"+region] = p
		}
	}

	if acoustidProvider != nil {
		providers["acoustid"] = acoustidProvider
	}

	if lastfmProvider != nil {
		providers["lastfm"] = lastfmProvider
	}

	if discogsProvider != nil {
		providers["discogs"] = discogsProvider
	}

	if deezerProvider != nil {
		providers["deezer"] = deezerProvider
	}

	if theaudiodbProvider != nil {
		providers["theaudiodb"] = theaudiodbProvider
	}

	if itunesProvider != nil {
		providers["itunes"] = itunesProvider
	}

	return providers
}

// GetAllDownloadProviders returns all registered download providers.
// This includes traditional download clients (qBittorrent, etc.) and
// indexer download clients (which track NZB file downloads separately from searches).
// Keys are "<type>_<name>" (indexer download clients:
// "newznab_<name>_download"), since config names are only unique per type.
func GetAllDownloadProviders() map[string]any {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	providers := make(map[string]any)
	for name, provider := range downloadProviders {
		if n, ok := provider.(interface{ GetProviderName() string }); ok {
			providers[n.GetProviderName()+"_"+name] = provider
		}
	}

	// Add indexer download clients (track NZB downloads separately from searches)
	for name, provider := range indexerProviders {
		if provider != nil && provider.DownloadClient != nil {
			providers["newznab_"+name+"_download"] = provider.DownloadClient
		}
	}

	return providers
}

//
// Book/Audiobook/Music Providers
//

// SetAudible registers an Audible provider by region.
func SetAudible(region string, provider *audible.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	audibleProviders[region] = provider
}

// GetAudible returns an Audible provider for the specified region.
// Returns nil if not found.
func GetAudible(region string) *audible.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return audibleProviders[region]
}

// SharedAudible returns the registered Audible provider for region, creating
// and registering one on first use, so every caller shares one rate limiter
// per region. An empty region means "us".
func SharedAudible(region string) *audible.Provider {
	if region == "" {
		region = string(audible.RegionUS)
	}

	if p := GetAudible(region); p != nil {
		return p
	}

	registryMutex.Lock()
	defer registryMutex.Unlock()

	if p := audibleProviders[region]; p != nil {
		return p
	}

	p := audible.NewProviderWithRegion(audible.Region(region))
	audibleProviders[region] = p

	return p
}

// GetAllAudible returns a copy of all registered Audible providers keyed by region.
func GetAllAudible() map[string]*audible.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	out := make(map[string]*audible.Provider, len(audibleProviders))
	maps.Copy(out, audibleProviders)

	return out
}

// SetAudnex sets the global Audnex provider instance.
func SetAudnex(provider *audnex.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	audnexProvider = provider
}

// GetAudnex returns the global Audnex provider instance.
// Returns nil if not initialized.
func GetAudnex() *audnex.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return audnexProvider
}

// SetOpenLibrary sets the global OpenLibrary provider instance.
func SetOpenLibrary(provider *openlibrary.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	openlibraryProvider = provider
}

// GetOpenLibrary returns the global OpenLibrary provider instance.
// Returns nil if not initialized.
func GetOpenLibrary() *openlibrary.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return openlibraryProvider
}

// SharedOpenLibrary returns the registered OpenLibrary provider, creating and
// registering one when none exists, so every caller shares one rate limiter.
func SharedOpenLibrary() *openlibrary.Provider {
	if p := GetOpenLibrary(); p != nil {
		return p
	}

	registryMutex.Lock()
	defer registryMutex.Unlock()

	if openlibraryProvider == nil {
		openlibraryProvider = openlibrary.NewProvider()
	}

	return openlibraryProvider
}

// SetGoodreads sets the global Goodreads provider instance.
func SetGoodreads(provider *goodreads.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	goodreadsProvider = provider
}

// GetGoodreads returns the global Goodreads provider instance.
// Returns nil if not initialized.
func GetGoodreads() *goodreads.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return goodreadsProvider
}

// SetLastFM sets the global Last.fm provider instance.
func SetLastFM(provider *lastfm.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	lastfmProvider = provider
}

// GetLastFM returns the global Last.fm provider instance.
// Returns nil if not initialized.
func GetLastFM() *lastfm.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return lastfmProvider
}

// SetDiscogs sets the global Discogs provider instance.
func SetDiscogs(provider *discogs.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	discogsProvider = provider
}

// GetDiscogs returns the global Discogs provider instance.
// Returns nil if not initialized.
func GetDiscogs() *discogs.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return discogsProvider
}

// SetITunes sets the global iTunes provider instance.
func SetITunes(provider *itunes.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	itunesProvider = provider
}

// GetITunes returns the global iTunes provider instance.
// Returns nil if not initialized.
func GetITunes() *itunes.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return itunesProvider
}

// SetTheAudioDB sets the global TheAudioDB provider instance.
func SetTheAudioDB(provider *theaudiodb.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	theaudiodbProvider = provider
}

// GetTheAudioDB returns the global TheAudioDB provider instance.
// Returns nil if not initialized.
func GetTheAudioDB() *theaudiodb.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return theaudiodbProvider
}

// SetDeezer sets the global Deezer provider instance.
func SetDeezer(provider *deezer.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	deezerProvider = provider
}

// GetDeezer returns the global Deezer provider instance.
// Returns nil if not initialized.
func GetDeezer() *deezer.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return deezerProvider
}

// SetMusicBrainz sets the global MusicBrainz provider instance.
func SetMusicBrainz(provider *musicbrainz.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	musicbrainzProvider = provider
}

// GetMusicBrainz returns the global MusicBrainz provider instance.
// Returns nil if not initialized.
func GetMusicBrainz() *musicbrainz.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return musicbrainzProvider
}

// SharedMusicBrainz returns the registered MusicBrainz provider or, when none
// is registered, one shared fallback instance. MusicBrainz allows 1 request/s,
// so independent instances with their own limiters would exceed it together.
func SharedMusicBrainz() *musicbrainz.Provider {
	registryMutex.RLock()
	p := musicbrainzProvider
	if p == nil {
		p = musicbrainzFallback
	}
	registryMutex.RUnlock()

	if p != nil {
		return p
	}

	registryMutex.Lock()
	defer registryMutex.Unlock()

	if musicbrainzProvider != nil {
		return musicbrainzProvider
	}

	if musicbrainzFallback == nil {
		musicbrainzFallback = musicbrainz.NewProvider()
	}

	return musicbrainzFallback
}

// SetAcoustID sets the global AcoustID provider instance.
func SetAcoustID(provider *acoustid.Provider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	acoustidProvider = provider
}

// GetAcoustID returns the global AcoustID provider instance.
// Returns nil if not initialized.
func GetAcoustID() *acoustid.Provider {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	return acoustidProvider
}
