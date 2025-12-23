package apiexternal_v2

import "context"

//
// Metadata Provider Interface - Fully Typed, No any or any
//

// MetadataProvider defines the interface for metadata providers with complete type safety.
// type MetadataProvider interface {
// 	// Provider information
// 	GetProviderType() ProviderType
// 	GetProviderName() string

// 	// Movie search and lookup
// 	SearchMovies(ctx context.Context, query string, year int) ([]MovieSearchResult, error)
// 	GetMovieByID(ctx context.Context, id int) (*MovieDetails, error)
// 	FindMovieByIMDbID(ctx context.Context, imdbID string) (*FindByIMDbResult, error)
// 	GetMovieExternalIDs(ctx context.Context, id int) (*ExternalIDs, error)

// 	// TV series search and lookup
// 	SearchSeries(ctx context.Context, query string, year int) ([]SeriesSearchResult, error)
// 	GetSeriesByID(ctx context.Context, id int) (*SeriesDetails, error)
// 	FindSeriesByIMDbID(ctx context.Context, imdbID string) (*FindByIMDbResult, error)
// 	FindSeriesByTVDbID(ctx context.Context, tvdbID int) (*SeriesDetails, error)
// 	GetSeriesExternalIDs(ctx context.Context, id int) (*ExternalIDs, error)

// 	// Episode information
// 	GetSeasonDetails(ctx context.Context, seriesID int, seasonNumber int) (*Season, error)
// 	GetEpisodeDetails(
// 		ctx context.Context,
// 		seriesID int,
// 		seasonNumber int,
// 		episodeNumber int,
// 	) (*Episode, error)

// 	// Popular/Trending
// 	GetPopularMovies(ctx context.Context, page int) (*PopularMoviesResponse, error)
// 	GetPopularSeries(ctx context.Context, page int) (*PopularSeriesResponse, error)
// 	GetTrendingMovies(ctx context.Context, page int) (*PopularMoviesResponse, error)
// 	GetTrendingSeries(ctx context.Context, page int) (*PopularSeriesResponse, error)
// 	GetUpcomingMovies(ctx context.Context, page int) (*PopularMoviesResponse, error)

// 	// Alternative titles
// 	GetMovieAlternativeTitles(ctx context.Context, id int) ([]AlternativeTitle, error)
// 	GetSeriesAlternativeTitles(ctx context.Context, id int) ([]AlternativeTitle, error)

// 	// Credits
// 	GetMovieCredits(ctx context.Context, id int) (*Credits, error)
// 	GetSeriesCredits(ctx context.Context, id int) (*Credits, error)

// 	// Media resources
// 	GetMovieImages(ctx context.Context, id int) (*ImageCollection, error)
// 	GetSeriesImages(ctx context.Context, id int) (*ImageCollection, error)
// 	GetMovieVideos(ctx context.Context, id int) ([]Video, error)
// 	GetSeriesVideos(ctx context.Context, id int) ([]Video, error)

// 	// Recommendations and similar
// 	GetSimilarMovies(ctx context.Context, id int) ([]MovieSearchResult, error)
// 	GetSimilarSeries(ctx context.Context, id int) ([]SeriesSearchResult, error)
// 	GetMovieRecommendations(ctx context.Context, id int) ([]MovieSearchResult, error)
// 	GetSeriesRecommendations(ctx context.Context, id int) ([]SeriesSearchResult, error)
// }

// WatchlistProvider defines the interface for watchlist functionality (Plex, Jellyfin, Trakt).
type WatchlistProvider interface {
	GetProviderType() ProviderType
	GetProviderName() string

	// Watchlist operations
	GetWatchlist(ctx context.Context, username string) ([]WatchlistItem, error)
	AddToWatchlist(ctx context.Context, username string, itemType string, id int) error
	RemoveFromWatchlist(ctx context.Context, username string, itemType string, id int) error
}

// DownloadProvider defines the interface for download clients with full type safety.
// type DownloadProvider interface {
// 	GetProviderType() DownloadProviderType
// 	GetProviderName() string

// 	// Torrent operations - fully typed
// 	AddTorrent(ctx context.Context, request TorrentAddRequest) (*TorrentAddResponse, error)
// 	GetTorrentInfo(ctx context.Context, hash string) (*TorrentInfo, error)
// 	ListTorrents(ctx context.Context, filter string) (*TorrentListResponse, error)

// 	// NZB operations - for NZB download clients (SABnzbd, NZBGet)
// 	AddNZB(ctx context.Context, nzbURL, category string, priority int) error

// 	// Torrent control
// 	PauseTorrent(ctx context.Context, hash string) error
// 	ResumeTorrent(ctx context.Context, hash string) error
// 	RemoveTorrent(ctx context.Context, hash string, deleteFiles bool) error

// 	// Status and health
// 	GetStatus(ctx context.Context) (*DownloadClientStatus, error)
// 	TestConnection(ctx context.Context) error
// }

// NotificationProvider defines the interface for notification services with full type safety.
type NotificationProvider interface {
	GetProviderType() NotificationProviderType
	GetProviderName() string

	// Core notification methods
	SendNotification(
		ctx context.Context,
		request NotificationRequest,
	) (*NotificationResponse, error)

	// Test connectivity
	TestConnection(ctx context.Context) error
}

// IndexerProvider defines the interface for indexer services (Newznab/Torznab) with full type safety.
// type IndexerProvider interface {
// 	GetProviderType() IndexerProviderType
// 	GetProviderName() string

// 	// Search operations - fully typed, returns slice of Nzbwithprio directly
// 	Search(ctx context.Context, request IndexerSearchRequest) ([]Nzbwithprio, error)
// 	SearchMovies(ctx context.Context, query string, year int) ([]Nzbwithprio, error)
// 	SearchTV(ctx context.Context, query string, season, episode int) ([]Nzbwithprio, error)
// 	SearchByIMDB(ctx context.Context, imdbID string) ([]Nzbwithprio, error)
// 	SearchByTVDB(
// 		ctx context.Context,
// 		tvdbID int,
// 		season, episode int,
// 	) ([]Nzbwithprio, error)

// 	// Capabilities
// 	GetCapabilities(ctx context.Context) (*IndexerCapabilities, error)

// 	// RSS feed
// 	GetRSSFeed(ctx context.Context, categories []string, limit int) ([]Nzbwithprio, error)

// 	// Test connectivity
// 	TestConnection(ctx context.Context) error
// }
