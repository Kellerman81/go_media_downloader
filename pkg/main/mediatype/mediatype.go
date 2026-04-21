// Package mediatype provides a registration-based system for handling different media types.
// Each media type (movies, series, etc.) registers its own handler, allowing type-specific
// logic to be called without switch statements throughout the codebase.
package mediatype

import (
	"context"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// NamingData contains data needed for generating file/folder naming templates.
// This is populated by FillNamingData and used by structure package for template parsing.
type NamingData struct {
	Dbmovie            database.Dbmovie
	Dbserie            database.Dbserie
	Serie              database.Serie
	DbserieEpisode     database.DbserieEpisode
	Dbaudiobook        database.Dbaudiobook
	DbaudiobookChapter database.DbaudiobookChapter
	Dbbook             database.Dbbook
	Dbalbum            database.Dbalbum
	Dbtrack            database.Dbtrack
	Author             database.Dbauthor
	BookSeries         database.DbbookSeries
	Artist             database.Dbartist
	AlbumArtist        database.Dbartist
	TitleSource        string
	EpisodeTitleSource string
	Title              string // Track/chapter title
	Track              int    // Track/chapter number
	Episodes           []int
}

// Function type definitions for registerable functions.
// These allow packages like utils and structure to register their implementations
// without creating circular dependencies.
type (
	// OrganizeFunc is the signature for media organization functions
	// OrganizeFunc func(org any, data any, info *database.ParseInfo, qualcfg *config.QualityConfig, deleteWrongLang, checkRuntime bool) error.

	// ImportParseFunc is the signature for file import/parse functions
	// ImportParseFunc func(info *database.ParseInfo, fpath string, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, addFound bool) error.

	// RefreshFunc is the signature for data refresh functions.
	RefreshFunc func(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error

	// ImportNewFunc is the signature for new media import functions
	// ImportNewFunc func(ctx context.Context, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int) error.

	// InitialFillFunc is the signature for initial database fill functions
	// InitialFillFunc func().

	// DataFullFunc is the signature for full data refresh functions.
	DataFullFunc func()
)

// Handler defines the interface that each media type must implement.
// This allows type-specific behavior to be encapsulated in separate packages
// (e.g., movies, series) while maintaining a consistent API.
type Handler interface {
	// GetType returns the media type constant (e.g., config.MediaTypeMovie)
	GetType() uint

	// GetCategoryName returns the category name for job history ("movie" or "series")
	GetCategoryName() string

	// GetTableName returns the database table name ("movies" or "series")
	GetTableName() string

	// GetDBIDs retrieves database IDs for the parsed media info (simple lookup)
	GetDBIDs(info *database.ParseInfo) error

	// GetDBIDsFull retrieves database IDs with full search capabilities
	// including title-based search and list finding.
	// addFound controls whether unknown media should be added to the DB during search.
	GetDBIDsFull(
		info *database.ParseInfo,
		cfgp *config.MediaTypeConfig,
		allowSearchTitle bool,
		addFound bool,
	) error

	// ValidateIDs checks if all required IDs are set for this media type
	ValidateIDs(info *database.ParseInfo) bool

	// SetTempID sets the temporary ID from the appropriate source field
	SetTempID(info *database.ParseInfo)

	// SetDBID sets the database ID field for this media type
	SetDBID(info *database.ParseInfo, dbid uint)

	// GetDBID returns the database ID field for this media type (DbmovieID or DbserieID)
	GetDBID(info *database.ParseInfo) uint

	// GetMediaID returns the media-specific ID (MovieID or SerieID)
	GetMediaID(info *database.ParseInfo) uint

	// SetMediaID sets the media-specific ID (MovieID or SerieID)
	SetMediaID(info *database.ParseInfo, id uint)

	// GetListID retrieves the list ID for the media item
	GetListID(cfgp *config.MediaTypeConfig, info *database.ParseInfo) int

	// ClearUntrustedID clears the external ID (IMDB/TVDB) if indexer is not trusted
	ClearUntrustedID(entry *apiexternal_v2.Nzbwithprio)

	// SetNzbID sets the NZB ID field (NzbmovieID or NzbepisodeID)
	SetNzbID(entry *apiexternal_v2.Nzbwithprio, mediaid uint)

	// SetEntryTempID sets the temp ID on an entry from NZB data
	SetEntryTempID(entry *apiexternal_v2.Nzbwithprio)

	// PerformIDSearch executes a search by external ID (IMDB or TVDB)
	PerformIDSearch(
		indcfg *config.IndexersConfig,
		quality *config.QualityConfig,
		entry *apiexternal_v2.Nzbwithprio,
		cats int,
		raw *apiexternal.NzbSlice,
	) error

	// ClearUnmatchedCache removes the file from the unmatched cache
	ClearUnmatchedCache(fpath string)

	// ShortenYearPattern returns true if year pattern should be shortened during parsing
	// Movies shorten all patterns including year, series does not shorten year
	ShortenYearPattern() bool

	// GenerateIdentifier generates the episode identifier for series (e.g., "S01E05")
	// Movies do nothing, series generate from season/episode or date
	GenerateIdentifier(info *database.ParseInfo, onlyIfEmpty bool)

	// GetSchedulerRssSeasons returns the interval and cron strings for RSS seasons jobs
	// Movies return empty strings, series return the configured values
	GetSchedulerRssSeasons(
		scheduler *config.SchedulerConfig,
		jobType string,
	) (interval, cron string)

	// GetSchedulerRssArtistsAuthors returns the interval and cron strings for RSS artists/authors jobs
	// Music returns artist config values, audiobooks/books return author config values
	// Movies and series return empty strings
	GetSchedulerRssArtistsAuthors(
		scheduler *config.SchedulerConfig,
		jobType string,
	) (interval, cron string)

	// Organize organizes a media file into the proper folder structure
	// Organize(org any, data any, info *database.ParseInfo, qualcfg *config.QualityConfig, deleteWrongLang, checkRuntime bool) error

	// ImportParse imports and parses a media file
	// ImportParse(info *database.ParseInfo, fpath string, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, addFound bool) error

	// Refresh refreshes media data (incremental or full based on data parameter)
	Refresh(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error

	// ImportNew imports new media from feeds
	// ImportNew(ctx context.Context, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int) error

	// InitialFill performs initial database fill for this media type
	// InitialFill()

	// DataFull performs full data refresh for this media type
	DataFull()

	// SearchConfigByName searches for a media config entry by name in a list's config file.
	// For series: searches the series config file for matching name or alternate names.
	// For movies: returns nil, false (not supported).
	SearchConfigByName(
		searchName string,
		listCfg *config.MediaListsConfig,
	) (*config.ManualConfig, bool)

	// RecordDownloadHistory records a download in the appropriate history table.
	// For movies: inserts into movie_histories
	// For series: inserts into serie_episode_histories
	RecordDownloadHistory(nzb *apiexternal_v2.Nzbwithprio, targetPath string) error

	// GetDownloadTargetFolder returns the target folder name for a download.
	// For movies: returns title with IMDB ID (e.g., "Movie Title (tt1234567)")
	// For series: returns title with TVDB ID (e.g., "Series Title (tvdb12345)")
	// Returns empty string if no specific folder name can be generated.
	GetDownloadTargetFolder(nzb *apiexternal_v2.Nzbwithprio, dbExternalID string) string

	// FillSearchVar fills search variables from the database for the given media ID.
	// Sets the NZB ID field, loads data from DB, validates required fields.
	// Returns error if validation fails (ErrNotFound*, ErrDisabled, etc.)
	FillSearchVar(entry *apiexternal_v2.Nzbwithprio, mediaid uint) error

	// GetNzbID returns the NZB ID field (NzbmovieID or NzbepisodeID)
	GetNzbID(entry *apiexternal_v2.Nzbwithprio) uint

	// GetNzbID? returns the NZB ID field (NzbmovieID or NzbepisodeID) as a pointer
	GetNzbIDP(entry *apiexternal_v2.Nzbwithprio) *uint

	// CheckMediaMatch checks if the entry's media ID matches the source's NZB ID.
	// Returns true if they match, false if they don't.
	// For movies: compares entry.Info.MovieID with source.NzbmovieID
	// For series: compares entry.Info.SerieEpisodeID with source.NzbepisodeID
	CheckMediaMatch(source, entry *apiexternal_v2.Nzbwithprio) bool

	// GetUnwantedReason returns the reason string for unwanted media.
	// For movies: "unwanted Movie"
	// For series: "unwanted Episode"
	GetUnwantedReason() string

	// GetFoundID returns the ID that was found in the entry for logging.
	// For movies: entry.Info.MovieID
	// For series: entry.Info.SerieEpisodeID
	GetFoundID(entry *apiexternal_v2.Nzbwithprio) uint

	// ValidateRSSIDs validates the required IDs for RSS processing.
	// Returns error reason string if invalid, empty string if valid.
	// For movies: checks DbmovieID and MovieID
	// For series: checks SerieID, DbserieID, DbserieEpisodeID, SerieEpisodeID
	ValidateRSSIDs(entry *apiexternal_v2.Nzbwithprio) string

	// SetRSSIDs sets the Dbid and NzbID fields from entry.Info for RSS processing.
	// For movies: sets entry.Dbid = DbmovieID, entry.NzbmovieID = MovieID
	// For series: sets entry.Dbid = DbserieID, entry.NzbepisodeID = SerieEpisodeID
	SetRSSIDs(entry *apiexternal_v2.Nzbwithprio)

	// GetRSSMediaID returns the media ID to use for getrssdata.
	// For movies: returns entry.Info.MovieID
	// For series: returns entry.Info.SerieEpisodeID
	GetRSSMediaID(entry *apiexternal_v2.Nzbwithprio) uint

	// CheckCorrectID validates that the entry's external ID matches the source's.
	// For movies: compares IMDB IDs with prefix/zero trimming
	// For series: compares TVDB IDs
	// Returns true if IDs don't match (should skip), false if they match or can't compare.
	// Sets entry.Reason if there's a mismatch.
	// Also returns found and wanted ID strings for logging purposes.
	CheckCorrectID(
		sourceentry, entry *apiexternal_v2.Nzbwithprio,
	) (skip bool, foundID, wantedID string)

	// GetRuntimeBonus returns extra runtime tolerance for extended editions.
	// For movies: returns 10 if info.Extended is true, 0 otherwise
	// For series: always returns 0
	GetRuntimeBonus(info *database.ParseInfo) int

	// SkipMultipleFiles returns true if this media type should skip folders with multiple video files.
	// For movies: returns true (movies should be single files)
	// For series: returns false (series can have multiple episodes)
	SkipMultipleFiles() bool

	// FillNotifyData fills notification data from the database for the given ID.
	// Returns title, year, imdb/tvdb, series info, season, episode, identifier.
	// For movies: fills title, year, imdb from dbmovies
	// For series: fills title, year, tvdb, series, season, episode, identifier from dbseries/dbserie_episodes
	// Returns false if DB lookup fails.
	FillNotifyData(
		id *uint,
	) (title, year, externalID, series, season, episode, identifier string, ok bool)

	// FillNamingData fills NamingData with naming template data from the database.
	// For movies: fills Dbmovie, TitleSource, and handles IMDB prefix
	// For series: fills Dbserie, DbserieEpisode, TitleSource, EpisodeTitleSource, Episodes, TVDB
	// Returns clearFolder=true if folder should be cleared, ok=false if DB lookup fails.
	FillNamingData(
		dbid *uint,
		videofile string,
		m *database.ParseInfo,
		data *NamingData,
	) (clearFolder bool, ok bool)

	// GetRefreshIncData returns the data needed for incremental refresh.
	// For movies: returns []string of IMDB IDs (limit 100, ordered by updated_at desc)
	// For series: returns []DbstaticTwoStringOneRInt of continuing series (limit 20, ordered by updated_at asc)
	GetRefreshIncData() any

	// GetRefreshFullData returns the data needed for full refresh.
	// For movies: returns []string of all IMDB IDs
	// For series: returns []DbstaticTwoStringOneRInt of all series
	GetRefreshFullData() any

	// GetSchedulerJobNames returns a slice of job name pairs for scheduler configuration.
	// Each pair contains [schedulerJobName, singleJobName] where:
	// - schedulerJobName is the name used in the scheduler config
	// - singleJobName is the name passed to SingleJobs (may differ for refresh jobs)
	// For movies: common jobs + refreshmoviesfull/refreshmoviesinc
	// For series: common jobs + RssSeasons/RssSeasonsAll + refreshseriesfull/refreshseriesinc
	GetSchedulerJobNames() [][2]string

	// CleanupAfterRemove handles cleanup after a video file is removed.
	// For movies: calls walkcleanup on the rootpath (uses walkCleanupFn callback)
	// For series: removes other extension files (uses removeOtherFilesFn callback)
	// Returns an error if cleanup validation fails (e.g., missing path template).
	CleanupAfterRemove(
		folder, rootpath string,
		pathCfgName string,
		walkCleanupFn func(string),
		removeOtherFilesFn func(),
	) error

	// MoveOtherFilesAfterOrganize handles moving additional files after main video is organized.
	// For movies: calls walkcleanup to move related files
	// For series: moves files with other allowed extensions
	// Both types call notifyFn and cleanupFolderFn after processing.
	MoveOtherFilesAfterOrganize(params *MoveOtherFilesParams) error

	// CheckExtensions validates if a file extension is allowed for this media type.
	// For movies/series: checks video extensions
	// For books: checks book extensions
	// For audiobooks/music: checks audio extensions
	// Returns (allowed, skipRename) where:
	//   - allowed: true if extension is permitted for processing
	//   - skipRename: true if renaming should be skipped for this extension
	CheckExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool)

	// SupportsIDSearch returns true if this media type supports ID-based search
	// (IMDB for movies, TVDB for series). Returns false for books, audiobooks,
	// and music which only support query-based search.
	SupportsIDSearch() bool

	// SupportsSeasonSearch returns true if this media type has season/episode
	// structure (only series). Returns false for movies, books, audiobooks, music.
	SupportsSeasonSearch() bool

	// RequiresYearCheck returns true if year validation should be performed
	// during search result filtering. Movies require strict year matching,
	// while series and other media types have different release date semantics.
	RequiresYearCheck() bool

	// HasSearchID returns true if the entry has a valid ID for ID-based search.
	// For movies: checks if IMDB ID is present
	// For series: checks if TVDB ID is present
	// For books/audiobooks/music: always returns false (query-only)
	HasSearchID(entry *apiexternal_v2.Nzbwithprio) bool

	// SupportsAbsoluteEpisode returns true if this media type supports
	// absolute episode numbering (e.g., anime). Only series supports this.
	SupportsAbsoluteEpisode() bool

	// HandleRSSListID handles setting the list ID for RSS entries.
	// For movies: sets ListID from addinlistid parameter
	// For other types: no-op
	HandleRSSListID(entry *apiexternal_v2.Nzbwithprio, addinlistid int)

	// CheckEpisodeMatch validates if the entry matches the expected episode.
	// For series: validates season/episode identifiers
	// For other types: returns false (no validation needed)
	// Returns true if entry should be skipped (doesn't match).
	CheckEpisodeMatch(
		sourceentry, entry *apiexternal_v2.Nzbwithprio,
		searchActionType string,
		logdenied func(string, *apiexternal_v2.Nzbwithprio),
	) bool

	// SupportsVideoFile returns true if this media type uses video files.
	// For movies/series: returns true (uses video extensions)
	// For books/audiobooks/music: returns false (uses other file types)
	SupportsVideoFile() bool

	// GetRuntimeMultiplier returns the multiplier for runtime calculation.
	// For series: returns the number of episodes in the file (len(m.Episodes))
	// For other types: returns 1
	GetRuntimeMultiplier(m *database.ParseInfo) int

	// ShouldCheckOldFilePriority returns true if old file priority should be checked
	// before organizing. For movies: returns true (checks existing file quality).
	// For series and other types: returns false (handled differently).
	ShouldCheckOldFilePriority() bool

	// HasConfiguredExtensions returns true if primary extensions are configured for this type.
	// For movies/series: checks AllowedVideoExtensionsLen
	// For audiobooks/music: checks AllowedAudioExtensionsLen
	// For books: checks AllowedBookExtensionsLen
	HasConfiguredExtensions(pathcfg *config.PathsConfig) bool

	// IsExternalIDImdb returns true if the external ID is IMDB format.
	// For movies: returns true (uses IMDB)
	// For series and others: returns false (uses TVDB or other)
	IsExternalIDImdb() bool

	// UsesGroupedFileProcessing returns true if this media type should process files
	// grouped by folder (audiobooks, music) rather than individually.
	// For audiobooks/music: returns true (group audio files in folders)
	// For movies/series/books: returns false (process files individually)
	UsesGroupedFileProcessing() bool

	// GetCacheUnmatchedKey returns the cache key for unmatched items of this type.
	// For movies: returns CacheUnmatchedMovie
	// For series: returns CacheUnmatchedSeries
	// etc.
	GetCacheUnmatchedKey() string

	// GetCacheFilesKey returns the cache key for files of this type.
	// For movies: returns CacheFilesMovie
	// For series: returns CacheFilesSeries
	// etc.
	GetCacheFilesKey() string

	// UsesListNameAsQualityProfile returns true if this media type uses the list name
	// as the quality profile name instead of the quality config name.
	// For series: returns true (uses list name for episode quality tracking)
	// For other types: returns false (uses quality config name)
	UsesListNameAsQualityProfile() bool
}

// MoveOtherFilesParams contains parameters for MoveOtherFilesAfterOrganize.
type MoveOtherFilesParams struct {
	Folder                 string
	Rootpath               string
	MediaFile              string // Primary media file (video for movies/series, audio for music/audiobooks, ebook for books)
	TargetPath             string // Target directory path
	Filename               string
	PathCfgName            string
	AllowedOtherExtensions []string
	WalkCleanupFn          func(rootpath, targetpath, filename string)
	MoveFileFn             func(source, target, filename string) error
	NotifyFn               func()
	CleanupFolderFn        func()
}

// registry holds all registered media type handlers.
var (
	registry   = make(map[uint]Handler)
	registryMu sync.RWMutex
)

// Register adds a handler for the specified media type.
// This should be called from init() in each media type package.
func Register(handler Handler) {
	registryMu.Lock()
	defer registryMu.Unlock()

	registry[handler.GetType()] = handler
}

// Get returns the handler for the specified media type.
// Returns nil if no handler is registered for the type.
func Get(mediaType uint) Handler {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[mediaType]
}

// MustGet returns the handler for the specified media type.
// Panics if no handler is registered (useful for catching configuration errors early).
func MustGet(mediaType uint) Handler {
	h := Get(mediaType)
	if h == nil {
		panic("no handler registered for media type")
	}

	return h
}

// GetCategoryName returns the category name for the media type.
// Returns empty string if type is not registered.
func GetCategoryName(mediaType uint) string {
	if h := Get(mediaType); h != nil {
		return h.GetCategoryName()
	}

	return ""
}

// GetTableName returns the table name for the media type.
// Returns empty string if type is not registered.
func GetTableName(mediaType uint) string {
	if h := Get(mediaType); h != nil {
		return h.GetTableName()
	}

	return ""
}

// ForEach iterates over all registered handlers and calls the provided function.
// Useful for operations that need to run for all media types.
func ForEach(fn func(Handler)) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	for _, h := range registry {
		fn(h)
	}
}

// ForType executes a function only if a handler exists for the type.
// Returns false if no handler is registered.
func ForType(mediaType uint, fn func(Handler)) bool {
	if h := Get(mediaType); h != nil {
		fn(h)
		return true
	}

	return false
}

// GetDBID returns the database ID for the media type.
// Returns 0 if type is not registered.
func GetDBID(mediaType uint, info *database.ParseInfo) uint {
	if h := Get(mediaType); h != nil {
		return h.GetDBID(info)
	}

	return 0
}

// GetMediaID returns the media-specific ID for the media type.
// Returns 0 if type is not registered.
func GetMediaID(mediaType uint, info *database.ParseInfo) uint {
	if h := Get(mediaType); h != nil {
		return h.GetMediaID(info)
	}

	return 0
}

// CheckExtensions validates if a file extension is allowed for the specified media type.
// If checkother is true, checks "other" extensions (subtitles, NFOs, etc.) instead of primary.
// Returns (false, false) if no handler is registered for the type.
func CheckExtensions(
	mediaType uint,
	checkother bool,
	pathcfg *config.PathsConfig,
	ext string,
) (bool, bool) {
	if checkother {
		return CheckOtherExtensions(pathcfg, ext)
	}

	if h := Get(mediaType); h != nil {
		return h.CheckExtensions(pathcfg, ext)
	}

	return false, false
}

// CheckOtherExtensions validates if a file extension is allowed for supplementary files
// (subtitles, NFO files, images, etc.).
// Returns (true, true) if no extensions are configured (permissive mode).
// Returns (true, false) if extension is in the allowed list.
// Returns (true, true) if extension is in the no-rename list.
// Returns (false, false) if extension is not allowed.
func CheckOtherExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	if pathcfg.AllowedOtherExtensionsLen == 0 {
		return true, true
	}

	if logger.SlicesContainsI(pathcfg.AllowedOtherExtensions, ext) {
		return true, false
	}

	if pathcfg.AllowedOtherExtensionsNoRenameLen > 0 &&
		logger.SlicesContainsI(pathcfg.AllowedOtherExtensionsNoRename, ext) {
		return true, true
	}

	return false, false
}

// CheckVideoExtensions validates if a file extension is allowed for video files.
// Returns (true, true) if no extensions are configured (permissive mode).
// Returns (true, false) if extension is in the allowed list.
// Returns (true, true) if extension is in the no-rename list.
// Returns (false, false) if extension is not allowed.
func CheckVideoExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	if pathcfg.AllowedVideoExtensionsLen == 0 {
		return true, true
	}

	if logger.SlicesContainsI(pathcfg.AllowedVideoExtensions, ext) {
		return true, false
	}

	if pathcfg.AllowedVideoExtensionsNoRenameLen > 0 &&
		logger.SlicesContainsI(pathcfg.AllowedVideoExtensionsNoRename, ext) {
		return true, true
	}

	return false, false
}

// CheckAudioExtensions validates if a file extension is allowed for audio files.
// Returns (true, true) if no extensions are configured (permissive mode).
// Returns (true, false) if extension is in the allowed list.
// Returns (true, true) if extension is in the no-rename list.
// Returns (false, false) if extension is not allowed.
func CheckAudioExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	if pathcfg.AllowedAudioExtensionsLen == 0 {
		return true, false
	}

	if logger.SlicesContainsI(pathcfg.AllowedAudioExtensions, ext) {
		return true, false
	}

	if pathcfg.AllowedAudioExtensionsNoRenameLen > 0 &&
		logger.SlicesContainsI(pathcfg.AllowedAudioExtensionsNoRename, ext) {
		return true, true
	}

	return false, false
}

// CheckBookExtensions validates if a file extension is allowed for ebook files.
// Returns (true, true) if no extensions are configured (permissive mode).
// Returns (true, false) if extension is in the allowed list.
// Returns (true, true) if extension is in the no-rename list.
// Returns (false, false) if extension is not allowed.
func CheckBookExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	if pathcfg.AllowedBookExtensionsLen == 0 {
		return true, true
	}

	if logger.SlicesContainsI(pathcfg.AllowedBookExtensions, ext) {
		return true, false
	}

	if pathcfg.AllowedBookExtensionsNoRenameLen > 0 &&
		logger.SlicesContainsI(pathcfg.AllowedBookExtensionsNoRename, ext) {
		return true, true
	}

	return false, false
}

// SupportsIDSearch returns true if the media type supports ID-based search.
// Returns false if no handler is registered for the type.
func SupportsIDSearch(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.SupportsIDSearch()
	}

	return false
}

// SupportsSeasonSearch returns true if the media type supports season/episode search.
// Returns false if no handler is registered for the type.
func SupportsSeasonSearch(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.SupportsSeasonSearch()
	}

	return false
}

// RequiresYearCheck returns true if year validation is required for the media type.
// Returns false if no handler is registered for the type.
func RequiresYearCheck(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.RequiresYearCheck()
	}

	return false
}

// HasSearchID returns true if the entry has a valid search ID for the media type.
// Returns false if no handler is registered for the type.
func HasSearchID(mediaType uint, entry *apiexternal_v2.Nzbwithprio) bool {
	if h := Get(mediaType); h != nil {
		return h.HasSearchID(entry)
	}

	return false
}

// SupportsAbsoluteEpisode returns true if the media type supports absolute episode numbering.
// Returns false if no handler is registered for the type.
func SupportsAbsoluteEpisode(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.SupportsAbsoluteEpisode()
	}

	return false
}

// HandleRSSListID handles setting the list ID for RSS entries.
func HandleRSSListID(mediaType uint, entry *apiexternal_v2.Nzbwithprio, addinlistid int) {
	if h := Get(mediaType); h != nil {
		h.HandleRSSListID(entry, addinlistid)
	}
}

// CheckEpisodeMatch validates if the entry matches the expected episode for the media type.
// Returns true if entry should be skipped (doesn't match).
func CheckEpisodeMatch(
	mediaType uint,
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
	searchActionType string,
	logdenied func(string, *apiexternal_v2.Nzbwithprio),
) bool {
	if h := Get(mediaType); h != nil {
		return h.CheckEpisodeMatch(sourceentry, entry, searchActionType, logdenied)
	}

	return false
}

// SupportsVideoFile returns true if the media type uses video files.
// Returns false if no handler is registered for the type.
func SupportsVideoFile(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.SupportsVideoFile()
	}

	return false
}

// GetRuntimeMultiplier returns the runtime multiplier for the media type.
// Returns 1 if no handler is registered for the type.
func GetRuntimeMultiplier(mediaType uint, m *database.ParseInfo) int {
	if h := Get(mediaType); h != nil {
		return h.GetRuntimeMultiplier(m)
	}

	return 1
}

// ShouldCheckOldFilePriority returns true if old file priority should be checked.
// Returns false if no handler is registered for the type.
func ShouldCheckOldFilePriority(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.ShouldCheckOldFilePriority()
	}

	return false
}

// HasConfiguredExtensions returns true if primary extensions are configured for the type.
// Returns false if no handler is registered for the type.
func HasConfiguredExtensions(mediaType uint, pathcfg *config.PathsConfig) bool {
	if h := Get(mediaType); h != nil {
		return h.HasConfiguredExtensions(pathcfg)
	}

	return false
}

// IsExternalIDImdb returns true if the media type uses IMDB for external IDs.
// Returns false if no handler is registered for the type.
func IsExternalIDImdb(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.IsExternalIDImdb()
	}

	return false
}

// UsesGroupedFileProcessing returns true if the media type processes files grouped by folder.
// Returns false if no handler is registered for the type.
func UsesGroupedFileProcessing(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.UsesGroupedFileProcessing()
	}

	return false
}

// GetCacheUnmatchedKey returns the cache key for unmatched items.
// Returns empty string if no handler is registered for the type.
func GetCacheUnmatchedKey(mediaType uint) string {
	if h := Get(mediaType); h != nil {
		return h.GetCacheUnmatchedKey()
	}

	return ""
}

// GetCacheFilesKey returns the cache key for files.
// Returns empty string if no handler is registered for the type.
func GetCacheFilesKey(mediaType uint) string {
	if h := Get(mediaType); h != nil {
		return h.GetCacheFilesKey()
	}

	return ""
}

// UsesListNameAsQualityProfile returns true if the media type uses list name as quality profile.
// Returns false if no handler is registered for the type.
func UsesListNameAsQualityProfile(mediaType uint) bool {
	if h := Get(mediaType); h != nil {
		return h.UsesListNameAsQualityProfile()
	}

	return false
}
