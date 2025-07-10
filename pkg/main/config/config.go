package config

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/pelletier/go-toml/v2"
	"github.com/recoilme/pudge"
	"golang.org/x/oauth2"
)

// Series Config

type MainSerieConfig struct {
	// Serie is a slice of SerieConfig structs that defines the series configurations
	Serie []SerieConfig `toml:"series"`
}

// SerieConfig defines the configuration for a TV series.
type SerieConfig struct {
	// Name is the primary name for the series
	Name string `toml:"name" comment:"the primary name for the serie"`

	// TvdbID is the tvdbid for the series for better searches
	TvdbID int `toml:"tvdb_id" comment:"the tvdbid for the serie for better searches"`

	// AlternateName specifies alternate names which the series is known for
	// Alternates from tvdb and trakt are added
	AlternateName []string `toml:"alternatename" comment:"specify some alternate names which the serie is known for: Alternates from tvdb and trakt are added"`

	// DisallowedName specifies names which the series is not allowed to have
	// These are removed from Alternates from tvdb and trakt
	DisallowedName []string `toml:"disallowedname" comment:"specify some names which the serie is not allowed to have: Removed from Alternates from tvdb and trakt"`

	// Identifiedby specifies how the media is structured, e.g. ep=S01E01, date=yy-mm-dd
	Identifiedby string `toml:"identifiedby" comment:"how is the media structured: ep=S01E01, date=yy-mm-dd"`

	// DontUpgrade indicates whether to skip searches for better versions of media
	DontUpgrade bool `toml:"dont_upgrade" comment:"do you want to skip the search for better versions of your media?"`

	// DontSearch indicates whether the series should not be searched
	DontSearch bool `toml:"dont_search" comment:"should the serie not be searched?"`

	// SearchSpecials indicates whether to also search Season 0 (specials)
	SearchSpecials bool `toml:"search_specials" comment:"do you want to also search Season 0 aka Specials?"`

	// IgnoreRuntime indicates whether to ignore episode runtime checks
	IgnoreRuntime bool `toml:"ignore_runtime" comment:"should the runtime of an episode not be looked at?"`

	// Source specifies the metadata source, e.g. none or tvdb
	Source string `toml:"source" comment:"use none or tvdb"`

	// Target defines a specific path to use for the media
	// This path must also be in the media data section
	Target string `toml:"target" comment:"you can define a specific path to be used for your media - the path needs also be included in the media data section or it will not be found after the download"`
}

// Main Config
// mainConfig struct defines the overall configuration
// It contains fields for each configuration section.
type mainConfig struct {
	// GeneralConfig contains general configuration settings
	General GeneralConfig `toml:"general" comment:"the general config"`

	// ImdbConfig contains IMDB specific configuration
	Imdbindexer ImdbConfig `toml:"imdbindexer" comment:"the imdb config"`

	// mediaConfig contains media related configuration
	Media MediaConfig `toml:"media" comment:"the media definitions"`

	// DownloaderConfig defines downloader specific configuration
	Downloader []DownloaderConfig `toml:"downloader" comment:"the downloader definitions"`

	// ListsConfig contains configuration for lists
	Lists []ListsConfig `toml:"lists" comment:"the list definitions"`

	// IndexersConfig defines configuration for indexers
	Indexers []IndexersConfig `toml:"indexers" comment:"the indexer definitions"`

	// PathsConfig contains configuration for paths
	Paths []PathsConfig `toml:"paths" comment:"the path definitions"`

	// NotificationConfig contains configuration for notifications
	Notification []NotificationConfig `toml:"notification" comment:"the notification definitions"`

	// RegexConfig contains configuration for regex
	Regex []RegexConfig `toml:"regex" comment:"the regex definitions"`

	// QualityConfig contains configuration for quality
	Quality []QualityConfig `toml:"quality" comment:"the quality definitions"`

	// SchedulerConfig contains configuration for scheduler
	Scheduler []SchedulerConfig `toml:"scheduler" comment:"the scheduler definitions"`
}

type GeneralConfig struct {
	// TimeFormat defines the time format to use, options are rfc3339, iso8601, rfc1123, rfc822, rfc850 - default: rfc3339
	TimeFormat string `toml:"time_format"      comment:"use one the these: rfc3339,iso8601,rfc1123,rfc822,rfc850 - default: rfc3339"`
	// TimeZone defines the timezone to use, options are local, utc or one from IANA Time Zone database
	TimeZone string `toml:"time_zone"        comment:"use local,utc or one from IANA Time Zone database"`
	// LogLevel defines the log level to use, options are info or debug - default: info
	LogLevel string `toml:"log_level"        comment:"use info or debug - default: info"`
	// DBLogLevel defines the database log level to use, options are info or debug (not recommended) - default: info
	DBLogLevel string `toml:"db_log_level"     comment:"use info or debug (not recommended) - default: info"`
	// LogFileSize defines the size in MB for the log files - default: 5
	LogFileSize int `toml:"log_file_size"    comment:"the size in MB for the logfiles - default: 5"`
	// LogFileCount defines how many log files to keep - default: 1
	LogFileCount uint8 `toml:"log_file_count"   comment:"how many logs do you want to keep? - default: 1"`
	// LogCompress defines whether to compress old log files - default: false
	LogCompress bool `toml:"log_compress"     comment:"do you want to compress old logfiles? - default: false"`
	// LogToFileOnly defines whether to only log to file and not console - default: false
	LogToFileOnly bool `toml:"log_to_file_only" comment:"do you want to only log to file and not to the console? - default: false"`
	// LogColorize defines whether to use colors in console output - default: false
	LogColorize bool `toml:"log_colorize"     comment:"do you want to use colors in the console output? - default: false"`
	// LogZeroValues determines whether to log variables without a value.
	LogZeroValues bool `toml:"log_zero_values"  comment:"do you want to log variables without a value? - default: false"`
	// WorkerMetadata defines how many parallel jobs of list retrievals to run - default: 1
	WorkerMetadata int `toml:"worker_metadata"  comment:"how many parallel jobs of list retrievals do you want to run? too many might decrease performance - default: 1"`
	// WorkerFiles defines how many parallel jobs of file scanning to run - default: 1
	WorkerFiles int `toml:"worker_files"     comment:"how many parallel jobs of file scanning do you want to run? i suggest one - default: 1"`

	// WorkerParse defines how many parallel parsings to run for list retrievals - default: 1
	WorkerParse int `toml:"worker_parse"           comment:"for list retrievals - how many parsings do you want to do at a time? - default: 1"`
	// WorkerSearch defines how many parallel search jobs to run - default: 1
	WorkerSearch int `toml:"worker_search"          comment:"how many parallel jobs of search scans do you want to run? too many might decrease performance - default: 1"`
	// WorkerRSS defines how many parallel rss jobs to run - default: 1
	WorkerRSS int `toml:"worker_rss"          comment:"how many parallel jobs of rss scans do you want to run? too many might decrease performance - default: 1"`
	// WorkerIndexer defines how many indexers to query in parallel for each scan job - default: 1
	WorkerIndexer int `toml:"worker_indexer"         comment:"for indexer scans - how many indexers do you want to query at a time? - default: 1"`
	// OmdbAPIKey is the API key for OMDB - get one at https://www.omdbapi.com/apikey.aspx
	OmdbAPIKey string `toml:"omdb_apikey"            comment:"apikey for omdb - get one here: https://www.omdbapi.com/apikey.aspx"`
	// UseMediaCache defines whether to cache movies and series in RAM for better performance - default: false
	UseMediaCache bool `toml:"use_media_cache"        comment:"do you want to keep your movies and series in RAM for better performance? - default: false"`
	// UseFileCache defines whether to cache all files in RAM - default: false
	UseFileCache bool `toml:"use_file_cache"         comment:"do you want to keep a list of all your files in RAM? - default: false"`
	// UseHistoryCache defines whether to cache downloaded entry history in RAM - default: false
	UseHistoryCache bool `toml:"use_history_cache"      comment:"do you want to keep the list of downloaded entries in RAM? - default: false"`
	// CacheDuration defines hours after which cached data will be refreshed - default: 12
	CacheDuration  int `toml:"cache_duration"         comment:"after how many hours do you want to refresh the cached data - default 12"`
	CacheDuration2 int `toml:"-"`
	// CacheAutoExtend defines whether cache expiration will be reset on access - default: false
	CacheAutoExtend bool `toml:"cache_auto_extend"      comment:"should the expiration be reset when the cache is accessed? - default: false"`
	// SearcherSize defines initial size of found entries slice - default: 5000
	SearcherSize int `toml:"searcher_size"          comment:"the initial size of the found entries slice - indexercount multiplied by maxentries multiplied by a number of alternate titles - default 5000"`
	// MovieMetaSourceImdb defines whether to scan IMDB for movie metadata - default: false
	MovieMetaSourceImdb bool `toml:"movie_meta_source_imdb" comment:"should imdb be scanned for movie metadata? - default: false"`

	// MovieMetaSourceTmdb defines whether to scan TMDb for movie metadata - default: false
	MovieMetaSourceTmdb bool `toml:"movie_meta_source_tmdb"                  comment:"should tmdb be scanned for movie metadata? - default: false"`
	// MovieMetaSourceOmdb defines whether to scan OMDB for movie metadata - default: false
	MovieMetaSourceOmdb bool `toml:"movie_meta_source_omdb"                  comment:"should omdb be scanned for movie metadata? - default: false"`
	// MovieMetaSourceTrakt defines whether to scan Trakt for movie metadata - default: false
	MovieMetaSourceTrakt bool `toml:"movie_meta_source_trakt"                 comment:"should trakt be scanned for movie metadata? - default: false"`
	// MovieAlternateTitleMetaSourceImdb defines whether to scan IMDB for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceImdb bool `toml:"movie_alternate_title_meta_source_imdb"  comment:"should imdb be scanned for alternate movie titles? - default: false"`
	// MovieAlternateTitleMetaSourceTmdb defines whether to scan TMDb for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceTmdb bool `toml:"movie_alternate_title_meta_source_tmdb"  comment:"should tmdb be scanned for alternate movie titles? - default: false"`
	// MovieAlternateTitleMetaSourceOmdb defines whether to scan OMDB for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceOmdb bool `toml:"movie_alternate_title_meta_source_omdb"  comment:"should omdb be scanned for alternate movie titles? - default: false"`
	// MovieAlternateTitleMetaSourceTrakt defines whether to scan Trakt for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceTrakt bool `toml:"movie_alternate_title_meta_source_trakt" comment:"should trakt be scanned for alternate movie titles? - default: false"`
	// SerieAlternateTitleMetaSourceImdb defines whether to scan IMDB for alternate series titles - default: false
	SerieAlternateTitleMetaSourceImdb bool `toml:"serie_alternate_title_meta_source_imdb"  comment:"should imdb be scanned for alternate serie titles? - default: false"`
	// SerieAlternateTitleMetaSourceTrakt defines whether to scan Trakt for alternate series titles - default: false
	SerieAlternateTitleMetaSourceTrakt bool `toml:"serie_alternate_title_meta_source_trakt" comment:"should trakt be scanned for alternate serie titles? - default: false"`
	// MovieMetaSourcePriority defines priority order to scan metadata providers for movies - overrides individual settings
	MovieMetaSourcePriority []string `toml:"movie_meta_source_priority"              comment:"order in which the metadata providers should be scanned - overrides movie_meta_source_*"                 multiline:"true"`
	// MovieRSSMetaSourcePriority defines priority order to scan metadata providers for movie RSS - overrides individual settings
	MovieRSSMetaSourcePriority []string `toml:"movie_rss_meta_source_priority"          comment:"order in which the metadata providers should be scanned for RSS imports - overrides movie_meta_source_*" multiline:"true"`

	// MovieParseMetaSourcePriority defines priority order to scan metadata providers for movie file parsing - overrides individual settings
	MovieParseMetaSourcePriority []string `toml:"movie_parse_meta_source_priority" multiline:"true" comment:"order in which the metadata providers should be scanned for file parsings - overrides movie_meta_source_*"`
	// SerieMetaSourceTmdb defines whether to scan TMDb for series metadata - default: false
	SerieMetaSourceTmdb bool `toml:"serie_meta_source_tmdb"                            comment:"should tmdb be scanned for serie metadata? - default: false"`
	// SerieMetaSourceTrakt defines whether to scan Trakt for series metadata - default: false
	SerieMetaSourceTrakt bool `toml:"serie_meta_source_trakt"                           comment:"should trakt be scanned for serie metadata? - default: false"`
	// MoveBufferSizeKB defines buffer size in KB to use if file buffer copy enabled - default: 1024
	MoveBufferSizeKB int `toml:"move_buffer_size_kb"                               comment:"buffer size in kb to use if use_file_buffer_copy is set to true - default: 1024"`
	// WebPort defines port for web interface and API - default: 9090
	WebPort string `toml:"webport"                                           comment:"port to use for webinterface and API - default: 9090"`
	// WebAPIKey defines API key for API calls - default: mysecure
	WebAPIKey string `toml:"webapikey"                                         comment:"apikey to use for API calls - default: mysecure"`
	// WebPortalEnabled enables/disables web portal - default: false
	WebPortalEnabled bool `toml:"web_portal_enabled"                                comment:"Should the webportal be enabled? - default: false"`
	// TheMovieDBApiKey defines API key for TMDb - get from: https://www.themoviedb.org/settings/api
	TheMovieDBApiKey string `toml:"themoviedb_apikey"                                 comment:"apikey for tmdb - get one here: https://www.themoviedb.org/settings/api"`
	// TraktClientID defines client ID for Trakt - get from: https://trakt.tv/oauth/applications/new
	TraktClientID string `toml:"trakt_client_id"                                   comment:"your id for trakt - get one here: https://trakt.tv/oauth/applications/new"`
	// TraktClientSecret defines client secret for Trakt application
	TraktClientSecret string `toml:"trakt_client_secret"                               comment:"the secret for you trakt application"`
	// SchedulerDisabled enables/disables scheduler - default false
	SchedulerDisabled bool `toml:"scheduler_disabled"                                comment:"do you want the scheduler to be disabled? - default false"`

	// DisableParserStringMatch defines whether to disable string matching in parsers - default: false
	DisableParserStringMatch bool `toml:"disable_parser_string_match"  comment:"do you want to disable the matching of strings and only use regex for field matching? might decrease performance but increase accuracy - default: false"`
	// UseCronInsteadOfInterval defines whether to convert intervals to cron strings - default: false
	UseCronInsteadOfInterval bool `toml:"use_cron_instead_of_interval" comment:"do you want to convert the scheduler intervals to cron strings? has better performance if you do - default: false"`
	// UseFileBufferCopy defines whether to use buffered file copy - default: false
	UseFileBufferCopy bool `toml:"use_file_buffer_copy"         comment:"do you want to use a buffered file copy? i suggest no - default: false"`
	// DisableSwagger defines whether to disable Swagger API docs - default: false
	DisableSwagger bool `toml:"disable_swagger"              comment:"do you want to disable the swagger api documentation generation? - default: false"`
	// TraktLimiterSeconds defines seconds limit for Trakt API calls - default: 1
	TraktLimiterSeconds uint8 `toml:"trakt_limiter_seconds"        comment:"how many calls to trakt are allowed in x seconds - default: 1"`
	// TraktLimiterCalls defines calls limit for Trakt API in defined seconds - default: 1
	TraktLimiterCalls int `toml:"trakt_limiter_calls"          comment:"how many calls to trakt are allowed in the defined number of seconds - default: 1"`
	// TvdbLimiterSeconds defines seconds limit for TVDB API calls - default: 1
	TvdbLimiterSeconds uint8 `toml:"tvdb_limiter_seconds"         comment:"how many calls to tvdb are allowed in x seconds - default: 1"`
	// TvdbLimiterCalls defines calls limit for TVDB API in defined seconds - default: 1
	TvdbLimiterCalls int `toml:"tvdb_limiter_calls"           comment:"how many calls to tvdb are allowed in the defined number of seconds - default: 1"`
	// TmdbLimiterSeconds defines seconds limit for TMDb API calls - default: 1
	TmdbLimiterSeconds uint8 `toml:"tmdb_limiter_seconds"         comment:"how many calls to tmdb are allowed in x seconds - default: 1"`
	// TmdbLimiterCalls defines calls limit for TMDb API in defined seconds - default: 1
	TmdbLimiterCalls int `toml:"tmdb_limiter_calls"           comment:"how many calls to tmdb are allowed in the defined number of seconds - default: 1"`
	// OmdbLimiterSeconds defines seconds limit for OMDb API calls - default: 1
	OmdbLimiterSeconds uint8 `toml:"omdb_limiter_seconds"         comment:"how many calls to omdb are allowed in x seconds - default: 1"`
	// OmdbLimiterCalls defines calls limit for OMDb API in defined seconds - default: 1
	OmdbLimiterCalls int `toml:"omdb_limiter_calls"           comment:"how many calls to omdb are allowed in the defined number of seconds - default: 1"`

	// TheMovieDBDisableTLSVerify disables TLS certificate verification for TheMovieDB API requests
	// Setting this to true may increase performance but reduces security
	TheMovieDBDisableTLSVerify bool `toml:"tmdb_disable_tls_verify" comment:"do you want to disable the verification of SSL certificates for tmdb? might increase performance - default: false"`

	// TraktDisableTLSVerify disables TLS certificate verification for Trakt API requests
	// Setting this to true may increase performance but reduces security
	TraktDisableTLSVerify bool `toml:"trakt_disable_tls_verify" comment:"do you want to disable the verification of SSL certificates for trakt? might increase performance - default: false"`

	// OmdbDisableTLSVerify disables TLS certificate verification for OMDb API requests
	// Setting this to true may increase performance but reduces security
	OmdbDisableTLSVerify bool `toml:"omdb_disable_tls_verify" comment:"do you want to disable the verification of SSL certificates for omdb? might increase performance - default: false"`

	// TvdbDisableTLSVerify disables TLS certificate verification for TVDB API requests
	// Setting this to true may increase performance but reduces security
	TvdbDisableTLSVerify bool `toml:"tvdb_disable_tls_verify" comment:"do you want to disable the verification of SSL certificates for tvdb? might increase performance - default: false"`

	// FfprobePath specifies the path to the ffprobe executable
	// Used for media analysis
	FfprobePath string `toml:"ffprobe_path" comment:"path to your ffprobe file - please add path for performance reasons - default: ./ffprobe"`

	// MediainfoPath specifies the path to the mediainfo executable
	// Used as an alternative to ffprobe for media analysis
	MediainfoPath string `toml:"mediainfo_path" comment:"path to your mediainfo file - please add path for performance reasons - default: ./mediainfo"`

	// UseMediainfo specifies whether to use mediainfo instead of ffprobe for media analysis
	UseMediainfo bool `toml:"use_mediainfo" comment:"do you want to use mediainfo instead of ffprobe? - default: false"`

	// UseMediaFallback specifies whether to use mediainfo as a fallback if ffprobe fails
	UseMediaFallback bool `toml:"use_media_fallback" comment:"do you want to use mediainfo if ffprobe fails? - default: false"`

	// FailedIndexerBlockTime specifies how long in minutes an indexer should be blocked after failures
	FailedIndexerBlockTime int `toml:"failed_indexer_block_time" comment:"how long (minitues) should an indexer be blocked after fails - default: 5"`

	// MaxDatabaseBackups defines the maximum number of database backups to retain
	MaxDatabaseBackups int `toml:"max_database_backups" comment:"how many backups of the database do you want to keep - default: 0 (disable backups)"`

	// DatabaseBackupStopTasks specifies whether to stop background tasks during database backups
	DatabaseBackupStopTasks bool `toml:"database_backup_stop_tasks" comment:"should we stop the task worker and scheduler during backup?"`

	// DisableVariableCleanup specifies whether to disable cleanup of variables after use
	// This may reduce RAM usage but variables will persist
	// Default is false
	DisableVariableCleanup bool `toml:"disable_variable_cleanup" comment:"should variables not be cleaned after use? - might reduce RAM usage - default: false"`
	// OmdbTimeoutSeconds defines the HTTP timeout in seconds for OMDb API calls
	// Default is 10 seconds
	OmdbTimeoutSeconds uint16 `toml:"omdb_timeout_seconds"     comment:"how long should the http timeout be for omdb calls (seconds)? - default: 10"`
	// TmdbTimeoutSeconds defines the HTTP timeout in seconds for TMDb API calls
	// Default is 10 seconds
	TmdbTimeoutSeconds uint16 `toml:"tmdb_timeout_seconds"     comment:"how long should the http timeout be for tmdb calls (seconds)? - default: 10"`
	// TvdbTimeoutSeconds defines the HTTP timeout in seconds for TVDB API calls
	// Default is 10 seconds
	TvdbTimeoutSeconds uint16 `toml:"tvdb_timeout_seconds"     comment:"how long should the http timeout be for tvdb calls (seconds)? - default: 10"`
	// TraktTimeoutSeconds defines the HTTP timeout in seconds for Trakt API calls
	// Default is 10 seconds
	TraktTimeoutSeconds uint16 `toml:"trakt_timeout_seconds"    comment:"how long should the http timeout be for trakt calls (seconds)? - default: 10"`

	// Jobs To Run
	Jobs map[string]func(uint32) `toml:"-" json:"-"`
	// UseGoDir                           bool     `toml:"use_godir"`
	// ConcurrentScheduler                int      `toml:"concurrent_scheduler"`
	// EnableFileWatcher                  bool     `toml:"enable_file_watcher"`
}

// ImdbConfig defines the configuration for the IMDb indexer.
type ImdbConfig struct {
	// Indexedtypes is an array of strings specifying the types of IMDb media to import
	// Valid values are 'movie', 'tvMovie', 'tvmovie', 'tvSeries', 'tvseries', 'video'
	// Default is empty array which imports nothing
	Indexedtypes []string `toml:"indexed_types" multiline:"true" comment:"types of imdb media to import - use 'movie', 'tvMovie', 'tvmovie', 'tvSeries', 'tvseries', 'video' - default: <empty> = none"`

	// Indexedlanguages is an array of strings specifying the languages to use for titles
	// Examples: "DE", "UK", "US"
	// Include '' or '\N' for global titles
	// Default is empty array which imports all languages
	Indexedlanguages []string `toml:"indexed_languages" multiline:"true" comment:"array of languages to use for titles - ex. DE, UK, US - include '' for global titles - default: <empty> = all"`

	// Indexfull is a boolean specifying whether to index all available IMDb data
	// or only the bare minimum
	// Default is false
	Indexfull bool `toml:"index_full" comment:"do you want to index all available imdb data or only the bare minimum - default: false"`

	// ImdbIDSize is an integer specifying the number of expected entries in the IMDb database
	// Default is 12000000
	ImdbIDSize int `toml:"imdbid_size" comment:"how many entries do you think will be in the imdb database - default: 12000000"`

	// LoopSize is an integer specifying the number of entries to keep in RAM for cached queries
	// Default is 400000
	LoopSize int `toml:"loop_size" comment:"how many entries should be kept in RAM for cached queries - default: 400000"`

	// UseMemory is a boolean specifying whether to store the IMDb DB in RAM during generation
	// At least 2GB RAM required. Highly recommended.
	// Default is false
	UseMemory bool `toml:"use_memory" comment:"store imdb db during generation in RAM. mind. 2GB required - highly recommended - default: false"`

	// UseCache is a boolean specifying whether to use caching for SQL queries
	// Might reduce execution time
	// Default is false
	UseCache bool `toml:"use_cache" comment:"use cache for sql queries - might reduce execution time - default: false"`
}

// MediaConfig defines the configuration for media types like series and movies.
type MediaConfig struct {
	// Series defines the configuration for all series media types
	Series []MediaTypeConfig `toml:"series" comment:"the definitions of all your series"`
	// Movies defines the configuration for all movies media types
	Movies []MediaTypeConfig `toml:"movies" comment:"the definitions of all your movies"`
}

// MediaTypeConfig defines the configuration for a media type like movies or series.
type MediaTypeConfig struct {
	// Name is the name of the media group - keep it unique
	Name string `toml:"name" comment:"the name of the media group - keep it unique"`

	// NamePrefix is not set in the TOML config
	NamePrefix string `toml:"-"`

	// Useseries is set automatically to true if the configuration is for a series
	Useseries bool `toml:"-"`

	// DefaultQuality is the default quality to assume if none was found - keep it low
	DefaultQuality string `toml:"default_quality" comment:"if no quality was found - what should we assume it is? keep it low"`

	// DefaultResolution is the default resolution to assume if none was found - keep it low
	DefaultResolution string `toml:"default_resolution" comment:"if no resolution was found - what should we assume it is? keep it low"`

	// Naming is the naming scheme for files - see wiki for details
	Naming string `toml:"naming" comment:"the naming for your files - look at https://github.com/Kellerman81/go_media_downloader/wiki/Groups"`

	// TemplateQuality is the name of the quality template to use
	TemplateQuality string `toml:"template_quality" comment:"the name of the quality template to use"`

	// CfgQuality is the parsed quality config (not set in TOML)
	CfgQuality *QualityConfig `toml:"-"`

	// TemplateScheduler is the name of the scheduler template to use
	TemplateScheduler string `toml:"template_scheduler" comment:"the name of the scheduler template to use"`

	// CfgScheduler is the parsed scheduler config (not set in TOML)
	CfgScheduler *SchedulerConfig `toml:"-"`

	// MetadataLanguage is the default language for metadata
	MetadataLanguage string `toml:"metadata_language" comment:"the default language for your metadata - ex. en"`

	// MetadataTitleLanguages are the languages to import titles in
	MetadataTitleLanguages []string `toml:"metadata_title_languages" multiline:"true" comment:"what languages should be imported for the titles - ex. de, us, uk, en"`

	// MetadataTitleLanguagesLen is the number of title languages (not set in TOML)
	MetadataTitleLanguagesLen int `toml:"-"`

	// Structure indicates whether to structure media after download
	Structure bool `toml:"structure" comment:"do you want to structure your media after download? - default: false"`

	// SearchmissingIncremental is the number of entries to process in incremental missing scans
	SearchmissingIncremental uint16 `toml:"search_missing_incremental" comment:"how many entries should be processed on an incremental scan - default: 20"`

	// SearchupgradeIncremental is the number of entries to process in incremental upgrade scans
	SearchupgradeIncremental uint16 `toml:"search_upgrade_incremental" comment:"how many entries should be processed on an incremental upgrade scan - default: 20"`

	// Data contains the media data configs
	Data    []MediaDataConfig        `toml:"data"`
	DataMap map[int]*MediaDataConfig `toml:"-"`

	// DataLen is the number of data configs (not set in TOML)
	DataLen int `toml:"-"`

	// DataImport contains media data import configs
	DataImport    []MediaDataImportConfig        `toml:"data_import"`
	DataImportMap map[int]*MediaDataImportConfig `toml:"-"`

	// Lists contains media lists configs
	Lists []MediaListsConfig `toml:"lists"`

	// ListsMap is a map of the lists configs (not set in TOML)
	ListsMap    map[string]*MediaListsConfig `toml:"-"`
	ListsMapIdx map[string]int               `toml:"-"`

	// ListsQu is the quality from the lists config (not set in TOML)
	ListsQu string `toml:"-"`

	// ListsLen is the number of lists configs (not set in TOML)
	ListsLen int `toml:"-"`

	// ListsQualities are the quality strings from lists (not set in TOML)
	ListsQualities []string `toml:"-"`

	// Notification contains notification configs
	Notification []mediaNotificationConfig `toml:"notification"`

	// Jobs To Run
	Jobs map[string]func(uint32) `toml:"-" json:"-"`
}

// MediaDataConfig is a struct that defines configuration for media data.
type MediaDataConfig struct {
	// TemplatePath is the template to use for the path
	TemplatePath string `toml:"template_path"  comment:"the template to use for the path"`
	// CfgPath is a pointer to PathsConfig
	CfgPath *PathsConfig `toml:"-"`
	// AddFound indicates if entries not in watched media should be added if found
	// Default is false
	AddFound bool `toml:"add_found"      comment:"do you want to add entries not yet in your list of watched media if found? - default: false"`
	// AddFoundList is the list name that found entries should be added to
	AddFoundList string `toml:"add_found_list" comment:"under what list name should the found entries be added?"`
	// AddFoundListCfg is a pointer to ListsConfig
	AddFoundListCfg *ListsConfig `toml:"-"`
}

// MediaDataImportConfig defines the configuration for importing media data.
type MediaDataImportConfig struct {
	// TemplatePath is the template to use for the path
	TemplatePath string `toml:"template_path" comment:"the template to use for the path"`
	// CfgPath is the PathsConfig reference
	CfgPath *PathsConfig `toml:"-"`
}

// MediaListsConfig defines a media list configuration.
type MediaListsConfig struct {
	// Name is the name of the list - use this name in ignore or replace lists
	Name string `toml:"name"                   comment:"the name of the list - use this name in ignore or replace lists"`
	// TemplateList is the template to use for the list
	TemplateList string `toml:"template_list"          comment:"the template to use for the list"`
	// CfgList is the pointer to the ListsConfig
	CfgList *ListsConfig `toml:"-"`
	// TemplateQuality is the template to use for the quality
	TemplateQuality string `toml:"template_quality"       comment:"the template to use for the quality"`
	// CfgQuality is the pointer to the QualityConfig
	CfgQuality *QualityConfig `toml:"-"`
	// TemplateScheduler is the template to use for the scheduler - overrides default of media
	TemplateScheduler string `toml:"template_scheduler"     comment:"the template to use for the scheduler - overrides default of media"`
	// CfgScheduler is the pointer to the SchedulerConfig
	CfgScheduler *SchedulerConfig `toml:"-"`
	// IgnoreMapLists are the lists to check for ignoring entries
	IgnoreMapLists []string `toml:"ignore_template_lists"  comment:"if the entry exists in one of these lists it will be skipped"                                multiline:"true"`
	// IgnoreMapListsQu is the quality string
	IgnoreMapListsQu string `toml:"-"`
	// IgnoreMapListsLen is the length of IgnoreMapLists
	IgnoreMapListsLen int `toml:"-"`
	// ReplaceMapLists are the lists to check for replacing entries
	ReplaceMapLists []string `toml:"replace_template_lists" comment:"if the entry exists in one of these lists it will be replaced"                               multiline:"true"`
	// ReplaceMapListsLen is the length of ReplaceMapLists
	ReplaceMapListsLen int `toml:"-"`
	// Enabled indicates if this configuration is active
	Enabled bool `toml:"enabled"                comment:"is this configuration active? - default: false"`
	// Addfound indicates if entries not already watched should be added when found
	Addfound bool `toml:"add_found"              comment:"do you want to add entries not yet in your list of watched media if found? - default: false"`
}

// mediaNotificationConfig defines the configuration for notifications about media events.
type mediaNotificationConfig struct {
	// MapNotification is the template to use for the notification
	MapNotification string `toml:"template_notification" comment:"the template to use for the notification"`
	// CfgNotification is the NotificationConfig reference
	CfgNotification *NotificationConfig `toml:"-"`
	// Event is the type of event this is for - use added_download or added_data
	Event string `toml:"event"                 comment:"type of event this is for - use added_download or added_data"`
	// Title is the title of your message (for pushover)
	Title string `toml:"title"                 comment:"the title of your message (for pushover)"`
	// Message is the message body - look at https://github.com/Kellerman81/go_media_downloader/wiki/Groups for format info
	Message string `toml:"message"               comment:"the message - look at https://github.com/Kellerman81/go_media_downloader/wiki/Groups for format info"`
	// ReplacedPrefix is text to write in front of the old path if media was replaced
	ReplacedPrefix string `toml:"replaced_prefix"       comment:"if the media was replaced what do you want to write in front of the old path?"`
}

// DownloaderConfig is a struct that defines the configuration for a downloader client.
type DownloaderConfig struct {
	// Name is the name of the downloader template
	Name string `toml:"name"              comment:"the name of the template"`
	// DlType is the type of downloader, e.g. drone, nzbget, etc.
	DlType string `toml:"type"              comment:"type of the downloader - use: drone,nzbget,sabnzbd,transmission,rtorrent,qbittorrent,deluge"`
	// Hostname is the hostname to use if needed
	Hostname string `toml:"hostname"          comment:"hostname to use if needed"`
	// Port is the port to use if needed
	Port int `toml:"port"              comment:"port to use if needed"`
	// Username is the username to use if needed
	Username string `toml:"username"          comment:"username to use if needed"`
	// Password is the password to use if needed
	Password string `toml:"password"          comment:"password to use if needed"`
	// AddPaused specifies whether to add entries in paused state
	AddPaused bool `toml:"add_paused"        comment:"add entries in paused state"`
	// DelugeDlTo is the Deluge target for downloads
	DelugeDlTo string `toml:"deluge_dl_to"      comment:"deluge target for downloads"`
	// DelugeMoveAfter specifies if downloads should be moved after completion in Deluge
	DelugeMoveAfter bool `toml:"deluge_move_after" comment:"deluge - should the downloads be moved after completion - default: false"`
	// DelugeMoveTo is the Deluge target for downloads after completion
	DelugeMoveTo string `toml:"deluge_move_to"    comment:"deluge target for downloads after completion"`
	// Priority is the priority to set if needed
	Priority int `toml:"priority"          comment:"priority to set if needed"`
	// Enabled specifies if this template is active
	Enabled bool `toml:"enabled"           comment:"is this template active?"`
}

// ListsConfig defines the configuration for lists.
type ListsConfig struct {
	// Name is the name of the template
	Name string `toml:"name"               comment:"the name of the template"`
	// ListType is the type of the list
	ListType string `toml:"type"               comment:"type of the list - use one of: seriesconfig,traktpublicshowlist,imdbcsv,imdbfile,traktpublicmovielist,traktmoviepopular,traktmovieanticipated,traktmovietrending,traktseriepopular,traktserieanticipated,traktserietrending,newznabrss"`
	// URL is the url of the list
	URL string `toml:"url"                comment:"the url of the list"`
	// Enabled indicates if this template is active
	Enabled     bool   `toml:"enabled"            comment:"is this template active?"`
	IMDBCSVFile string `toml:"imdb_csv_file"      comment:"the path of the imdb csv file - ex. ./config/movies.csv"`
	// SeriesConfigFile is the path of the toml file
	SeriesConfigFile string `toml:"series_config_file" comment:"the path of the toml file - ex. ./config/series.toml"`
	// TraktUsername is the username who owns the trakt list
	TraktUsername string `toml:"trakt_username"     comment:"the username who owns the trakt list"`
	// TraktListName is the listname of the trakt list
	TraktListName string `toml:"trakt_listname"     comment:"the listname of the trakt list"`
	// TraktListType is the listtype of the trakt list
	TraktListType string `toml:"trakt_listtype"     comment:"the listtype of the trakt list - use one of: movie,show"`
	// Limit is how many entries should only be processed
	Limit string `toml:"limit"              comment:"how many entries should only be processed - default: 0 = all"`
	// MinVotes only import if that number of imdb votes have been reached
	MinVotes int `toml:"min_votes"          comment:"only import if that number of imdb votes have been reached"`
	// MinRating only import if that imdb rating has been reached
	MinRating float32 `toml:"min_rating"         comment:"only import if that imdb rating has been reached - ex. 5.5"`
	// Excludegenre don't import if it's one of the configured genres
	Excludegenre []string `toml:"exclude_genre"      comment:"don't import if it's one of the configured genres"                                                                                                                                                                                      multiline:"true"`
	// Includegenre only import if it's one of the configured genres
	Includegenre []string `toml:"include_genre"      comment:"only import if it's one of the configured genres"                                                                                                                                                                                       multiline:"true"`
	// ExcludegenreLen is the length of Excludegenre
	ExcludegenreLen int `toml:"-"`
	// IncludegenreLen is the length of Includegenre
	IncludegenreLen int `toml:"-"`
	// URLExtensions for discover
	TmdbDiscover []string `toml:"tmdb_discover"      comment:"tmdb discover url extension - https://developer.themoviedb.org/reference/discover-movie - or - https://developer.themoviedb.org/reference/discover-tv"`
	// List IDs of TMDB Lists
	TmdbList       []int `toml:"tmdb_list"`
	RemoveFromList bool  `toml:"remove_from_list"   comment:"remove the list after processing"`
}

// IndexersConfig defines the configuration for indexers.
type IndexersConfig struct {
	// Name is the name of the template
	Name string `toml:"name" comment:"the name of the template"`

	// IndexerType is the type of the indexer, currently has to be newznab
	IndexerType string `toml:"type" comment:"currently has to be newznab"`

	// URL is the main url of the indexer
	URL string `toml:"url" comment:"the main url of the indexer"`

	// Apikey is the apikey for the indexer
	Apikey string `toml:"apikey" comment:"the apikey for the indexer"`

	// Userid is the userid for rss queries to the indexer if needed
	Userid string `toml:"userid" comment:"the userid for rss queries to the indexer if needed"`

	// Enabled indicates if this template is active
	Enabled bool `toml:"enabled" comment:"is this template active?"`

	// Rssenabled indicates if this template is active for rss queries
	Rssenabled bool `toml:"rss_enabled" comment:"is this template active for rss queries?"`

	// Addquotesfortitlequery indicates if quotes should be added to a title query
	Addquotesfortitlequery bool `toml:"add_quotes_for_title_query" comment:"should quotes be added to a title query?"`

	// MaxEntries is the maximum number of entries to process, default is 100
	MaxEntries    uint16 `toml:"max_entries"      comment:"maximum number of entries to process - default: 100"`
	MaxEntriesStr string `toml:"-"`
	// RssEntriesloop is the number of rss calls to make to find last processed release, default is 2
	RssEntriesloop uint8 `toml:"rss_entries_loop" comment:"how many rss calls to make to find last processed release - default: 2"`

	// OutputAsJSON indicates if the indexer should return json instead of xml
	// Not recommended since the conversion is sometimes different
	OutputAsJSON bool `toml:"output_as_json" comment:"should the indexer return json instead of xml? - not recommended since the conversion is sometimes different"`

	// Customapi is used if the indexer needs a different value then 'apikey' for the key
	Customapi string `toml:"custom_api" comment:"does the indexer need a different value then 'apikey' for the key?"`

	// Customurl is used if the indexer needs a different url then url/api/ or url/rss/
	Customurl string `toml:"custom_url" comment:"does the indexer need a different url then url/api/ or url/rss/?"`

	// Customrssurl is used if the indexer uses a custom rss url (not url/rss/)
	Customrssurl string `toml:"custom_rss_url" comment:"does the indexer use a custom rss url (not url/rss/)?"`

	// Customrsscategory is used if the indexer uses something other than &t= for rss categories
	Customrsscategory string `toml:"custom_rss_category" comment:"does the indexer use something other than &t= for rss categories?"`

	// Limitercalls is the number of calls allowed in Limiterseconds
	Limitercalls int `toml:"limiter_calls" comment:"how many calls are we allowed to make in limiter_seconds? - default: 1"`

	// Limiterseconds is the number of seconds for Limitercalls calls
	Limiterseconds uint8 `toml:"limiter_seconds" comment:"how many calls (limiter_calls) are we allowed to make in x seconds? - default: 1"`

	// LimitercallsDaily is the number of calls allowed daily, 0 is unlimited
	LimitercallsDaily int `toml:"limiter_calls_daily" comment:"how many calls are we allowed to make daily - default: 0 = unlimited"`

	// MaxAge is the maximum age of releases in days
	MaxAge uint16 `toml:"max_age" comment:"maximum age of releases in days"`

	// DisableTLSVerify disables SSL Certificate Checks
	DisableTLSVerify bool `toml:"disable_tls_verify" comment:"disable SSL Certificate Checks"`

	// DisableCompression disables compression of data
	DisableCompression bool `toml:"disable_compression" comment:"disable compression of data"`

	// TimeoutSeconds is the timeout in seconds for queries
	TimeoutSeconds uint16 `toml:"timeout_seconds" comment:"timeout in seconds for queries"`

	TrustWithIMDBIDs bool `toml:"trust_with_imdb_ids" comment:"trust indexer imdb ids - can be problematic for RSS scans - some indexers tag wrong"`
	TrustWithTVDBIDs bool `toml:"trust_with_tvdb_ids" comment:"trust indexer tvdb ids - can be problematic for RSS scans - some indexers tag wrong"`

	// CheckTitleOnIDSearch is a bool indicating if the title of the release should be checked during an id based search? - default: false
	CheckTitleOnIDSearch bool `toml:"check_title_on_id_search" comment:"should the title of the release be checked during an id based search? - default: false"`
}

type PathsConfig struct {
	// Name is the name of the media template
	Name string `toml:"name"                               comment:"the name of the template"`
	// Path is the path where the media will be stored
	Path string `toml:"path"                               comment:"the path of the media"`
	// AllowedVideoExtensions lists the allowed video file extensions
	AllowedVideoExtensions []string `toml:"allowed_video_extensions"           comment:"what extensions are allowed for videos - enter extensions with a dot in front"                                 multiline:"true"`
	// AllowedVideoExtensionsLen is the number of allowed video extensions
	AllowedVideoExtensionsLen int `toml:"-"`
	// AllowedOtherExtensions lists other allowed file extensions
	AllowedOtherExtensions []string `toml:"allowed_other_extensions"           comment:"what extensions are allowed for other files we need to copy - enter extensions with a dot in front"            multiline:"true"`
	// AllowedOtherExtensionsLen is the number of other allowed extensions
	AllowedOtherExtensionsLen int `toml:"-"`
	// AllowedVideoExtensionsNoRename lists video extensions that should not be renamed
	AllowedVideoExtensionsNoRename []string `toml:"allowed_video_extensions_no_rename" comment:"what extensions are allowed for videos but should not be renamed - enter extensions with a dot in front"       multiline:"true"`
	// AllowedVideoExtensionsNoRenameLen is the number of video extensions not to rename
	AllowedVideoExtensionsNoRenameLen int `toml:"-"`
	// AllowedOtherExtensionsNoRename lists other extensions not to rename
	AllowedOtherExtensionsNoRename []string `toml:"allowed_other_extensions_no_rename" comment:"what extensions are allowed for other files but should not be renamed - enter extensions with a dot in front"  multiline:"true"`
	// AllowedOtherExtensionsNoRenameLen is the number of other extensions not to rename
	AllowedOtherExtensionsNoRenameLen int `toml:"-"`
	// Blocked lists strings that will block processing of files
	Blocked []string `toml:"blocked"                            comment:"if one of these strings are found the file will not be processed"                                              multiline:"true"`
	// BlockedLen is the number of blocked strings
	BlockedLen int `toml:"-"`
	// Upgrade indicates if media should be upgraded
	Upgrade bool `toml:"upgrade"                            comment:"should media be upgraded"`
	// MinSize is the minimum media size in MB for searches
	MinSize int `toml:"min_size"                           comment:"minimum size of media files in MB - used for searches - 0 = no limit"`
	// MaxSize is the maximum media size in MB for searches
	MaxSize int `toml:"max_size"                           comment:"maximum size of media files in MB - used for searches - 0 = no limit"`
	// MinSizeByte is the minimum size in bytes
	MinSizeByte int64 `toml:"-"`
	// MaxSizeByte is the maximum size in bytes
	MaxSizeByte int64 `toml:"-"`
	// MinVideoSize is the minimum video size in MB for structure
	MinVideoSize int `toml:"min_video_size"                     comment:"minimum size of media files in MB - used for structure - 0 = no limit"`
	// MinVideoSizeByte is the minimum video size in bytes
	MinVideoSizeByte int64 `toml:"-"`
	// CleanupsizeMB is the minimum size in MB to keep a folder, 0 removes all
	CleanupsizeMB int `toml:"cleanup_size_mb"                    comment:"if only x MB are left the folder should be removed - 0 = removeall"`
	// AllowedLanguages lists allowed languages for audio streams in videos
	AllowedLanguages []string `toml:"allowed_languages"                  comment:"allowed languages for audio streams in videos"                                                                 multiline:"true"`
	// AllowedLanguagesLen is the number of allowed languages
	AllowedLanguagesLen int `toml:"-"`
	// Replacelower indicates if lower quality video files should be replaced, default false
	Replacelower bool `toml:"replace_lower"                      comment:"should we replace lower quality video files? - default: false"`
	// Usepresort indicates if a presort folder should be used before media is moved, default false
	Usepresort bool `toml:"use_presort"                        comment:"is a presort folder be used and the media then moved manually later on? - default: false"`
	// PresortFolderPath is the path to the presort folder
	PresortFolderPath string `toml:"presort_folder_path"                comment:"the path of the presort folder"`
	// UpgradeScanInterval is the number of days to wait after last search before looking for upgrades, 0 means don't wait
	UpgradeScanInterval int `toml:"upgrade_scan_interval"              comment:"number of days to wait after the last media search for upgrades - 0 = don't wait"`
	// MissingScanInterval is the number of days to wait after last search before looking for missing media, 0 means don't wait
	MissingScanInterval int `toml:"missing_scan_interval"              comment:"number of days to wait after the last media search for missing media - 0 = don't wait"`
	// MissingScanReleaseDatePre is the minimum number of days to wait after media release before scanning, 0 means don't check
	MissingScanReleaseDatePre int `toml:"missing_scan_release_date_pre"      comment:"minimum wait time before a media is released to start scanning - in days - 0 = don't check"`
	// Disallowed lists strings that will block processing if found
	Disallowed []string `toml:"disallowed"                         comment:"if one of these strings are found the release will not be structured"                                          multiline:"true"`
	// DisallowedLen is the number of disallowed strings
	DisallowedLen int `toml:"-"`
	// DeleteWrongLanguage indicates if media with wrong language should be deleted, default false
	DeleteWrongLanguage bool `toml:"delete_wrong_language"              comment:"should releases with a wrong language be deleted? - default: false"`
	// DeleteDisallowed indicates if media with disallowed strings should be deleted, default false
	DeleteDisallowed bool `toml:"delete_disallowed"                  comment:"should releases with a disallowed string in the path be deleted? - default: false"`
	// CheckRuntime indicates if runtime should be checked before import, default false
	CheckRuntime bool `toml:"check_runtime"                      comment:"should the runtime of releases be checked before import? - default: false"`
	// MaxRuntimeDifference is the max minutes of difference allowed in runtime checks, 0 means no check
	MaxRuntimeDifference int `toml:"max_runtime_difference"             comment:"if the runtime of releases is checked how many minutes are we allowed to differ? - if 0 no check will be done"`
	// DeleteWrongRuntime indicates if media with wrong runtime should be deleted, default false
	DeleteWrongRuntime bool `toml:"delete_wrong_runtime"               comment:"if the runtime of a releases is wrong - should we remove it? - default: false"`
	// MoveReplaced indicates if replaced media should be moved to old folder, default false
	MoveReplaced bool `toml:"move_replaced"                      comment:"should replaced media be moved to an old data folder? - default: false"`
	// MoveReplacedTargetPath is the path to the folder for replaced media
	MoveReplacedTargetPath string `toml:"move_replaced_target_path"          comment:"the path to the folder for the old replaced media files"`
	// SetChmod is the chmod for files in octal format, default 0777
	SetChmod       string `toml:"set_chmod"                          comment:"the chmod for files - default 0777 - use octal format"`
	SetChmodFolder string `toml:"set_chmod_folder"                   comment:"the chmod for folders - default 0777 - use octal format"`
}

// NotificationConfig defines the configuration for notifications.
type NotificationConfig struct {
	// Name is the name of the notification template
	Name string `toml:"name"      comment:"the name of the template"`
	// NotificationType is the type of notification - use csv or pushover
	NotificationType string `toml:"type"      comment:"the type - use csv or pushover"`
	// Apikey is the pushover apikey - create here: https://pushover.net/apps/build
	Apikey string `toml:"apikey"    comment:"the pushover apikey - create here: https://pushover.net/apps/build"`
	// Recipient is the pushover recipient
	Recipient string `toml:"recipient" comment:"the pushover recipient"`
	// Outputto is the path to output csv notifications
	Outputto string `toml:"output_to" comment:"the csv path"`
}

// RegexConfig is a struct that defines a regex template
// It contains fields for the template name, required regexes,
// rejected regexes, and lengths of the regex slices.
type RegexConfig struct {
	// Name is the name of the regex template
	Name string `toml:"name"     comment:"the name of the template"`
	// Required is a slice of regex strings that are required (one must match)
	Required []string `toml:"required" comment:"regexes which are required (one of them)" multiline:"true"`
	// Rejected is a slice of regex strings that cause rejection if matched
	Rejected []string `toml:"rejected" comment:"regexes which are rejected (any)"         multiline:"true"`
	// RequiredLen is the length of the Required slice
	RequiredLen int `toml:"-"`
	// RejectedLen is the length of the Rejected slice
	RejectedLen int `toml:"-"`
}

type QualityConfig struct {
	// Name is the name of the template
	Name string `toml:"name"              comment:"the name of the template"`
	// WantedResolution is resolutions which are wanted - others are skipped - empty = allow all
	WantedResolution []string `toml:"wanted_resolution" comment:"resolutions which are wanted - others are skipped - empty = allow all"  multiline:"true"`
	// WantedQuality is qualities which are wanted - others are skipped - empty = allow all
	WantedQuality []string `toml:"wanted_quality"    comment:"qualities which are wanted - others are skipped - empty = allow all"    multiline:"true"`
	// WantedAudio is audio codecs which are wanted - others are skipped - empty = allow all
	WantedAudio []string `toml:"wanted_audio"      comment:"audio codecs which are wanted - others are skipped - empty = allow all" multiline:"true"`
	// WantedCodec is video codecs which are wanted - others are skipped - empty = allow all
	WantedCodec []string `toml:"wanted_codec"      comment:"video codecs which are wanted - others are skipped - empty = allow all" multiline:"true"`
	// WantedResolutionLen is the length of the WantedResolution slice
	WantedResolutionLen int `toml:"-"`
	// WantedQualityLen is the length of the WantedQuality slice
	WantedQualityLen int `toml:"-"`
	// WantedAudioLen is the length of the WantedAudio slice
	WantedAudioLen int `toml:"-"`
	// WantedCodecLen is the length of the WantedCodec slice
	WantedCodecLen int `toml:"-"`
	// CutoffResolution is after which resolution should we stop searching for upgrades
	CutoffResolution string `toml:"cutoff_resolution" comment:"after which resolution should we stop searching for upgrades"`
	// CutoffQuality is after which quality should we stop searching for upgrades
	CutoffQuality string `toml:"cutoff_quality"    comment:"after which quality should we stop searching for upgrades"`
	// CutoffPriority is the priority cutoff
	CutoffPriority int `toml:"-"`

	// SearchForTitleIfEmpty is a bool indicating if we should do a title search if the id search didn't return an accepted release
	// - backup_search_for_title needs to be true? - default: false
	SearchForTitleIfEmpty bool `toml:"search_for_title_if_empty" comment:"should we do a title search if the id search didn't return an accepted release - backup_search_for_title needs to be true? - default: false"`

	// BackupSearchForTitle is a bool indicating if we want to search for titles and not only id's - default: false
	BackupSearchForTitle bool `toml:"backup_search_for_title" comment:"do we want to search for titles and not only id's - default: false"`

	// SearchForAlternateTitleIfEmpty is a bool indicating if we should do a alternate title search if the id search didn't return an accepted release
	// - backup_search_for_alternate_title needs to be true? - default: false
	SearchForAlternateTitleIfEmpty bool `toml:"search_for_alternate_title_if_empty" comment:"should we do a alternate title search if the id search didn't return an accepted release - backup_search_for_alternate_title needs to be true? - default: false"`

	// BackupSearchForAlternateTitle is a bool indicating if we want to search for alternate titles and not only id's - default: false
	BackupSearchForAlternateTitle bool `toml:"backup_search_for_alternate_title" comment:"do we want to search for alternate titles and not only id's - default: false"`

	// ExcludeYearFromTitleSearch is a bool indicating if the year should not be included in the title search? - default: false
	ExcludeYearFromTitleSearch bool `toml:"exclude_year_from_title_search" comment:"should the year not be included in the title search? - default: false"`

	// CheckUntilFirstFound is a bool indicating if we should stop searching if we found a release? - default: false
	CheckUntilFirstFound bool `toml:"check_until_first_found" comment:"should we stop searching if we found a release? - default: false"`

	// CheckTitle is a bool indicating if the title of the release should be checked? - default: false
	CheckTitle bool `toml:"check_title" comment:"should the title of the release be checked? - default: false"`

	// CheckTitleOnIDSearch is a bool indicating if the title of the release should be checked during an id based search? - default: false
	CheckTitleOnIDSearch bool `toml:"check_title_on_id_search" comment:"should the title of the release be checked during an id based search? - default: false"`

	// CheckYear is a bool indicating if the year of the release should be checked? - default: false
	CheckYear bool `toml:"check_year" comment:"should the year of the release be checked? - default: false"`

	// CheckYear1 is a bool indicating if the year of the release should be checked and is +-1 year allowed? - default: false
	CheckYear1 bool `toml:"check_year1" comment:"should the year of the release be checked and is +-1 year allowed? - default: false"`

	// TitleStripSuffixForSearch is a []string indicating what suffixes should be removed from the title
	TitleStripSuffixForSearch []string `toml:"title_strip_suffix_for_search" multiline:"true" comment:"what suffixes should be removed from the title"`

	// TitleStripPrefixForSearch is a []string indicating what prefixes should be removed from the title
	TitleStripPrefixForSearch []string `toml:"title_strip_prefix_for_search" multiline:"true" comment:"what prefixes should be removed from the title"`

	// QualityReorder is a []QualityReorderConfig for configs if a quality reordering is needed - for example if 720p releases should be preferred over 1080p
	QualityReorder []QualityReorderConfig `toml:"reorder" comment:"configs if a quality reordering is needed - for example if 720p releases should be preferred over 1080p"`

	// Indexer is a []QualityIndexerConfig for configs of the indexers to be used for this quality
	Indexer    []QualityIndexerConfig `toml:"indexers" comment:"configs of the indexers to be used for this quality"`
	IndexerCfg []*IndexersConfig      `toml:"-"`

	// TitleStripSuffixForSearchLen is a int for the length of the TitleStripSuffixForSearch slice
	TitleStripSuffixForSearchLen int `toml:"-"`

	// TitleStripPrefixForSearchLen is the length of the TitleStripPrefixForSearch slice
	TitleStripPrefixForSearchLen int `toml:"-"`
	// QualityReorderLen is the length of the QualityReorder slice
	QualityReorderLen int `toml:"-"`
	// IndexerLen is the length of the Indexer slice
	IndexerLen int `toml:"-"`
	// UseForPriorityResolution indicates if resolution should be used for priority
	UseForPriorityResolution bool `toml:"use_for_priority_resolution"     comment:"should we use the resolution for the priority determination in searches? - default: false - recommended: true"`
	// UseForPriorityQuality indicates if quality should be used for priority
	UseForPriorityQuality bool `toml:"use_for_priority_quality"        comment:"should we use the quality for the priority determination in searches? - default: false - recommended: true"`
	// UseForPriorityAudio indicates if audio codecs should be used for priority
	UseForPriorityAudio bool `toml:"use_for_priority_audio"          comment:"should we use the audio codecs for the priority determination in searches? - default: false - recommended: false"`
	// UseForPriorityCodec indicates if video codecs should be used for priority
	UseForPriorityCodec bool `toml:"use_for_priority_codec"          comment:"should we use the video codecs for the priority determination in searches? - default: false - recommended: false"`
	// UseForPriorityOther indicates if other data should be used for priority
	UseForPriorityOther bool `toml:"use_for_priority_other"          comment:"should we use the other data like repack, extended, ... for the priority determination in searches? - default: false - recommended: false"`
	// UseForPriorityMinDifference is the min difference to use a release for upgrade
	UseForPriorityMinDifference int `toml:"use_for_priority_min_difference" comment:"what has to be the minimum difference to use the release for an upgrade - default: 0 = must be lower or equal"`
}

// QualityReorderConfig is a struct for configuring reordering of qualities
// It contains a Name string field for the name of the quality
// A ReorderType string field for the type of reordering
// And a Newpriority int field for the new priority.
type QualityReorderConfig struct {
	// Name is the name of the quality to reorder
	Name string `toml:"name"         comment:"the name of the quality to reorder - ex. 1080p - use a comma to separate multiple"`
	// ReorderType is the type of reordering to use
	ReorderType string `toml:"type"         comment:"the type of the reorder: use one of resolution,quality,codec,audio,position,combined_res_qual"`
	// Newpriority is the new priority to set for the quality
	Newpriority int `toml:"new_priority" comment:"the new priority for the entry - if position is used it is muliplied by the number - others are set to the value - for combined_res_qual the resolution is used for the priority and the quality is set to 0"`
}

// QualityIndexerConfig defines the configuration for an indexer used for a specific quality.
type QualityIndexerConfig struct {
	// TemplateIndexer is the template to use for the indexer
	TemplateIndexer string `toml:"template_indexer"        comment:"the template to use for the indexer"`
	// CfgIndexer is a pointer to the IndexersConfig for this indexer
	CfgIndexer *IndexersConfig `toml:"-"`
	// TemplateDownloader is the template to use for the downloader
	TemplateDownloader string `toml:"template_downloader"     comment:"the template to use for the downloader"`
	// CfgDownloader is a pointer to the DownloaderConfig for this downloader
	CfgDownloader *DownloaderConfig `toml:"-"`
	// TemplateRegex is the template to use for the regex
	TemplateRegex string `toml:"template_regex"          comment:"the template to use for the regex"`
	// CfgRegex is a pointer to the RegexConfig for this regex
	CfgRegex *RegexConfig `toml:"-"`
	// TemplatePathNzb is the template to use for the nzb path
	TemplatePathNzb string `toml:"template_path_nzb"       comment:"the template to use for the nzb path"`
	// CfgPath is a pointer to the PathsConfig for this path
	CfgPath *PathsConfig `toml:"-"`
	// CategoryDownloader is the category to use for the downloader
	CategoryDownloader string `toml:"category_downloader"     comment:"the category to use for the downloader"`
	// AdditionalQueryParams are additional params to add to the indexer query string
	AdditionalQueryParams string `toml:"additional_query_params" comment:"additional params to add to the indexer query string like &extended=1&maxsize=1572864000"`
	// SkipEmptySize indicates if releases with an empty size are allowed
	SkipEmptySize bool `toml:"skip_empty_size"         comment:"are releases with an empty size allowed? - default: false"`
	// HistoryCheckTitle indicates if the download history should check the title in addition to the url
	HistoryCheckTitle bool `toml:"history_check_title"     comment:"should the download history not only be checked for the url but also the title? - default: false"`
	// CategoriesIndexer are the categories to use for the indexer
	CategoriesIndexer string `toml:"categories_indexer"      comment:"categories to use for the indexer - separate with a comma use no spaces"`
}

type SchedulerConfig struct {
	// Name is the name of the template - see https://github.com/Kellerman81/go_media_downloader/wiki/Scheduler for details
	Name string `toml:"name" comment:"the name of the template - look at https://github.com/Kellerman81/go_media_downloader/wiki/Scheduler"`

	// IntervalImdb is the interval for imdb scans
	IntervalImdb string `toml:"interval_imdb"`

	// IntervalFeeds is the interval for rss feed scans
	IntervalFeeds string `toml:"interval_feeds"`

	// IntervalFeedsRefreshSeries is the interval for rss feed refreshes for series
	IntervalFeedsRefreshSeries string `toml:"interval_feeds_refresh_series"`

	// IntervalFeedsRefreshMovies is the interval for rss feed refreshes for movies
	IntervalFeedsRefreshMovies string `toml:"interval_feeds_refresh_movies"`

	// IntervalFeedsRefreshSeriesFull is the interval for full rss feed refreshes for series
	IntervalFeedsRefreshSeriesFull string `toml:"interval_feeds_refresh_series_full"`

	// IntervalFeedsRefreshMoviesFull is the interval for full rss feed refreshes for movies
	IntervalFeedsRefreshMoviesFull string `toml:"interval_feeds_refresh_movies_full"`

	// IntervalIndexerMissing is the interval for missing media scans
	IntervalIndexerMissing string `toml:"interval_indexer_missing"`

	// IntervalIndexerUpgrade is the interval for upgrade media scans
	IntervalIndexerUpgrade string `toml:"interval_indexer_upgrade"`

	// IntervalIndexerMissingFull is the interval for full missing media scans
	IntervalIndexerMissingFull string `toml:"interval_indexer_missing_full"`

	// IntervalIndexerUpgradeFull is the interval for full upgrade media scans
	IntervalIndexerUpgradeFull string `toml:"interval_indexer_upgrade_full"`

	// IntervalIndexerMissingTitle is the interval for missing media scans by title
	IntervalIndexerMissingTitle string `toml:"interval_indexer_missing_title"`

	// IntervalIndexerUpgradeTitle is the interval for upgrade media scans by title
	IntervalIndexerUpgradeTitle string `toml:"interval_indexer_upgrade_title"`

	// IntervalIndexerMissingFullTitle is the interval for full missing media scans
	IntervalIndexerMissingFullTitle string `toml:"interval_indexer_missing_full_title"`
	// IntervalIndexerUpgradeFullTitle is the interval for full upgrade media scans
	IntervalIndexerUpgradeFullTitle string `toml:"interval_indexer_upgrade_full_title"`
	// IntervalIndexerRss is the interval for rss feed scans
	IntervalIndexerRss string `toml:"interval_indexer_rss"`
	// IntervalScanData is the interval for data scans
	IntervalScanData string `toml:"interval_scan_data"`
	// IntervalScanDataMissing is the interval for missing data scans
	IntervalScanDataMissing string `toml:"interval_scan_data_missing"`
	// IntervalScanDataFlags is the interval for flagged data scans
	IntervalScanDataFlags string `toml:"interval_scan_data_flags"`
	// IntervalScanDataimport is the interval for data import scans
	IntervalScanDataimport string `toml:"interval_scan_data_import"`
	// IntervalDatabaseBackup is the interval for database backups
	IntervalDatabaseBackup string `toml:"interval_database_backup"`
	// IntervalDatabaseCheck is the interval for database checks
	IntervalDatabaseCheck string `toml:"interval_database_check"`
	// IntervalIndexerRssSeasons is the interval for rss feed season scans
	IntervalIndexerRssSeasons string `toml:"interval_indexer_rss_seasons"`
	// IntervalIndexerRssSeasonsAll is the interval for rss feed all season scans
	IntervalIndexerRssSeasonsAll string `toml:"interval_indexer_rss_seasons_all"`
	// CronIndexerRssSeasonsAll is the cron schedule for rss feed all season scans
	CronIndexerRssSeasonsAll string `toml:"cron_indexer_rss_seasons_all"`
	// CronIndexerRssSeasons is the cron schedule for rss feed season scans
	CronIndexerRssSeasons string `toml:"cron_indexer_rss_seasons"`
	// CronImdb is the cron schedule for imdb scans
	CronImdb string `toml:"cron_imdb"`
	// CronFeeds is the cron schedule for rss feed scans
	CronFeeds string `toml:"cron_feeds"`

	// CronFeedsRefreshSeries is the cron schedule for refreshing series RSS feeds
	CronFeedsRefreshSeries string `toml:"cron_feeds_refresh_series"`
	// CronFeedsRefreshMovies is the cron schedule for refreshing movie RSS feeds
	CronFeedsRefreshMovies string `toml:"cron_feeds_refresh_movies"`
	// CronFeedsRefreshSeriesFull is the cron schedule for full refreshing of series RSS feeds
	CronFeedsRefreshSeriesFull string `toml:"cron_feeds_refresh_series_full"`
	// CronFeedsRefreshMoviesFull is the cron schedule for full refreshing of movie RSS feeds
	CronFeedsRefreshMoviesFull string `toml:"cron_feeds_refresh_movies_full"`
	// CronIndexerMissing is the cron schedule for missing media scans
	CronIndexerMissing string `toml:"cron_indexer_missing"`
	// CronIndexerUpgrade is the cron schedule for upgrade media scans
	CronIndexerUpgrade string `toml:"cron_indexer_upgrade"`
	// CronIndexerMissingFull is the cron schedule for full missing media scans
	CronIndexerMissingFull string `toml:"cron_indexer_missing_full"`
	// CronIndexerUpgradeFull is the cron schedule for full upgrade media scans
	CronIndexerUpgradeFull string `toml:"cron_indexer_upgrade_full"`
	// CronIndexerMissingTitle is the cron schedule for missing media scans by title
	CronIndexerMissingTitle string `toml:"cron_indexer_missing_title"`
	// CronIndexerUpgradeTitle is the cron schedule for upgrade media scans by title
	CronIndexerUpgradeTitle string `toml:"cron_indexer_upgrade_title"`

	// CronIndexerMissingFullTitle is the cron schedule for full missing media scans
	CronIndexerMissingFullTitle string `toml:"cron_indexer_missing_full_title"`
	// CronIndexerUpgradeFullTitle is the cron schedule for full upgrade media scans
	CronIndexerUpgradeFullTitle string `toml:"cron_indexer_upgrade_full_title"`
	// CronIndexerRss is the cron schedule for rss feed scans
	CronIndexerRss string `toml:"cron_indexer_rss"`
	// CronScanData is the cron schedule for data scans
	CronScanData string `toml:"cron_scan_data"`
	// CronScanDataMissing is the cron schedule for missing data scans
	CronScanDataMissing string `toml:"cron_scan_data_missing"`
	// CronScanDataFlags is the cron schedule for flagged data scans
	CronScanDataFlags string `toml:"cron_scan_data_flags"`
	// CronScanDataimport is the cron schedule for data import scans
	CronScanDataimport string `toml:"cron_scan_data_import"`
	// CronDatabaseBackup is the cron schedule for database backups
	CronDatabaseBackup string `toml:"cron_database_backup"`
	// CronDatabaseCheck is the cron schedule for database checks
	CronDatabaseCheck string `toml:"cron_database_check"`
}

const Configfile = "./config/config.toml"

var (
	// SettingsGeneral contains the general configuration settings.
	SettingsGeneral GeneralConfig

	// SettingsImdb contains the IMDB specific configuration.
	SettingsImdb ImdbConfig

	// SettingsPath contains the path configuration settings.
	SettingsPath map[string]*PathsConfig

	// SettingsQuality contains the quality configuration settings.
	SettingsQuality map[string]*QualityConfig

	// SettingsList contains the list configuration settings.
	SettingsList map[string]*ListsConfig

	// SettingsIndexer contains the indexer configuration settings.
	SettingsIndexer map[string]*IndexersConfig

	// SettingsRegex contains the regex configuration settings.
	SettingsRegex map[string]*RegexConfig

	// SettingsMedia contains the media configuration settings.
	SettingsMedia map[string]*MediaTypeConfig

	// SettingsNotification contains the notification configuration settings.
	SettingsNotification map[string]*NotificationConfig

	// SettingsDownloader contains the downloader configuration settings.
	SettingsDownloader map[string]*DownloaderConfig

	// SettingsScheduler contains the scheduler configuration settings.
	SettingsScheduler map[string]*SchedulerConfig

	// traktToken contains the trakt OAuth token.
	traktToken *oauth2.Token

	// cachetoml contains the cached TOML configuration.
	cachetoml mainConfig

	mu = sync.RWMutex{}
)

// GetMediaListsEntryListID returns the index position of the list with the given
// name in the MediaTypeConfig. Returns -1 if no match is found.
func (cfgp *MediaTypeConfig) GetMediaListsEntryListID(listname string) int {
	if listname == "" {
		return -1
	}
	if cfgp == nil {
		logger.LogDynamicany0("error", "the config couldnt be found")
		return -1
	}
	k, ok := cfgp.ListsMapIdx[listname]
	if ok {
		return k
	}
	for k := range cfgp.Lists {
		if cfgp.Lists[k].Name == listname || strings.EqualFold(cfgp.Lists[k].Name, listname) {
			return k
		}
	}
	return -1
}

// GetMediaQualityConfig returns the QualityConfig from cfgp for the
// media with the given ID. It first checks if there is a quality profile
// set for that media in the database. If not, it returns the default
// QualityConfig from cfgp.
func (cfgp *MediaTypeConfig) GetMediaQualityConfigStr(str string) *QualityConfig {
	if str == "" {
		return SettingsQuality[cfgp.DefaultQuality]
	}
	q, ok := SettingsQuality[str]
	if ok {
		return q
	}
	return SettingsQuality[cfgp.DefaultQuality]
}

// getlistnamefilterignore returns a SQL WHERE clause to filter movies
// by list name ignore lists. If the list has ignore lists configured,
// it will generate a clause to exclude movies in those lists.
// Otherwise returns empty string.
func (list *MediaListsConfig) Getlistnamefilterignore() string {
	if list.IgnoreMapListsLen >= 1 {
		return ("listname in (?" + list.IgnoreMapListsQu + ") and ")
	}
	return ""
}

// qualityIndexerByQualityAndTemplate returns the CategoriesIndexer string for the indexer
// in the given QualityConfig that matches the given IndexersConfig by name.
// Returns empty string if no match is found.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplate(ind *IndexersConfig) int {
	if ind == nil {
		return -1
	}
	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return index
		}
	}
	return -1
}

// QualityIndexerByQualityAndTemplateCheckRegex returns the RegexConfig for the indexer
// in the given QualityConfig that matches the given IndexersConfig by name.
// Returns nil if no match is found.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplateCheckRegex(
	ind *IndexersConfig,
) *RegexConfig {
	if ind == nil {
		return nil
	}
	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return quality.Indexer[index].CfgRegex
		}
	}
	return nil
}

// QualityIndexerByQualityAndTemplateCheckTitle checks if the HistoryCheckTitle field of the
// IndexersConfig that matches the given IndexersConfig by name is true. If no match is found,
// it returns false.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplateCheckTitle(
	ind *IndexersConfig,
) bool {
	if ind == nil {
		return false
	}
	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return quality.Indexer[index].HistoryCheckTitle
		}
	}
	return false
}

// QualityIndexerByQualityAndTemplateSkipEmpty checks if the SkipEmptySize field of the
// IndexersConfig that matches the given IndexersConfig by name is true. If no match is found,
// it returns false.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplateSkipEmpty(
	ind *IndexersConfig,
) bool {
	if ind == nil {
		return false
	}
	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return quality.Indexer[index].SkipEmptySize
		}
	}
	return false
}

// getlistbyindexer returns the ListsConfig for the list matching the
// given IndexersConfig name. Returns nil if no match is found.
func (ind *IndexersConfig) Getlistbyindexer() *ListsConfig {
	for _, listcfg := range SettingsList {
		if listcfg.Name == ind.Name || strings.EqualFold(listcfg.Name, ind.Name) {
			return listcfg
		}
	}
	return nil
}

var RandomizerSource = rand.NewSource(time.Now().UnixNano())

// Slepping sleeps for a random or fixed number of seconds. If random is true,
// it will sleep for a random number of seconds between 1 and seconds. If random
// is false, it will sleep for the specified number of seconds. It uses the
// rand and time packages to generate the random sleep duration and sleep.
func Slepping(random bool, seconds int) {
	if random {
		n := rand.New(RandomizerSource).Intn(seconds) + 1 // n will be between 0 and 10
		time.Sleep(time.Duration(n) * time.Second)
	} else {
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

// LoadCfgDB loads the application configuration from a database file.
// It opens the configuration file, decodes the TOML data into a cache,
// and then populates various global settings maps with the configuration
// data. This allows the configuration system to be extended by adding
// new config types.
func LoadCfgDB() error {
	if _, err := os.Stat(Configfile); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Config file not found. Creating new config file.")
		ClearCfg()
		WriteCfg()
		fmt.Println("Config file created. Please edit it and run the application again.")
	} else {
		fmt.Println("Config file found. Loading config.")
	}
	err := Readconfigtoml()
	if err != nil {
		return err
	}
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{SyncInterval: 0})
	defer configDB.Close()
	pudge.BackupAll("")
	ClearSettings()
	Getconfigtoml()

	hastoken, _ := configDB.Has("trakt_token")
	if hastoken {
		var token oauth2.Token
		if configDB.Get("trakt_token", &token) == nil {
			traktToken = &token
		}
	}

	return nil
}

// Readconfigtoml reads and decodes the configuration file specified by Configfile into the global cachetoml struct.
// It opens the configuration file, uses a TOML decoder to parse its contents, and handles any potential errors.
// Returns an error if the file cannot be opened or decoded, otherwise returns nil.
func Readconfigtoml() error {
	content, err := os.Open(Configfile)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
		return err
	}
	defer content.Close()
	err = toml.NewDecoder(content).Decode(&cachetoml)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
		return err
	}
	return nil
}

// ClearSettings initializes and resets global configuration maps for various application components.
// It creates empty maps for downloaders, indexers, lists, media types, notifications,
// paths, quality settings, regex, and schedulers, preparing them to be populated with
// configuration data from the TOML configuration file.
func ClearSettings() {
	mu.Lock()
	defer mu.Unlock()
	SettingsDownloader = make(map[string]*DownloaderConfig, len(cachetoml.Downloader))
	SettingsIndexer = make(map[string]*IndexersConfig, len(cachetoml.Indexers))
	SettingsList = make(map[string]*ListsConfig, len(cachetoml.Lists))
	SettingsMedia = make(map[string]*MediaTypeConfig)
	SettingsNotification = make(map[string]*NotificationConfig, len(cachetoml.Notification))
	SettingsPath = make(map[string]*PathsConfig, len(cachetoml.Paths))
	SettingsQuality = make(map[string]*QualityConfig, len(cachetoml.Quality))
	SettingsRegex = make(map[string]*RegexConfig, len(cachetoml.Regex))
	SettingsScheduler = make(map[string]*SchedulerConfig, len(cachetoml.Scheduler))
}

// Getconfigtoml populates global configuration settings from the cached TOML configuration.
// It sets default values, initializes various configuration maps, and processes configuration
// for different components such as general settings, downloaders, indexers, lists, media types,
// notifications, paths, quality, regex, and schedulers. This function prepares the application's
// configuration by linking and transforming configuration data from the parsed TOML file.
func Getconfigtoml() {
	mu.Lock()
	defer mu.Unlock()
	SettingsGeneral = cachetoml.General
	if SettingsGeneral.CacheDuration == 0 {
		SettingsGeneral.CacheDuration = 12
	}
	SettingsGeneral.CacheDuration2 = 2 * SettingsGeneral.CacheDuration

	if len(SettingsGeneral.MovieMetaSourcePriority) == 0 {
		SettingsGeneral.MovieMetaSourcePriority = []string{"imdb", "tmdb", "omdb", "trakt"}
	}

	SettingsImdb = cachetoml.Imdbindexer

	for idx := range cachetoml.Downloader {
		SettingsDownloader[cachetoml.Downloader[idx].Name] = &cachetoml.Downloader[idx]
	}
	for idx := range cachetoml.Indexers {
		cachetoml.Indexers[idx].MaxEntriesStr = logger.IntToString(
			cachetoml.Indexers[idx].MaxEntries,
		)
		SettingsIndexer[cachetoml.Indexers[idx].Name] = &cachetoml.Indexers[idx]
	}
	for idx := range cachetoml.Lists {
		cachetoml.Lists[idx].ExcludegenreLen = len(cachetoml.Lists[idx].Excludegenre)
		cachetoml.Lists[idx].IncludegenreLen = len(cachetoml.Lists[idx].Includegenre)
		SettingsList[cachetoml.Lists[idx].Name] = &cachetoml.Lists[idx]
	}

	for idx := range cachetoml.Notification {
		SettingsNotification[cachetoml.Notification[idx].Name] = &cachetoml.Notification[idx]
	}
	for idx := range cachetoml.Paths {
		cachetoml.Paths[idx].AllowedLanguagesLen = len(cachetoml.Paths[idx].AllowedLanguages)
		cachetoml.Paths[idx].AllowedOtherExtensionsLen = len(
			cachetoml.Paths[idx].AllowedOtherExtensions,
		)
		cachetoml.Paths[idx].AllowedOtherExtensionsNoRenameLen = len(
			cachetoml.Paths[idx].AllowedOtherExtensionsNoRename,
		)
		cachetoml.Paths[idx].AllowedVideoExtensionsLen = len(
			cachetoml.Paths[idx].AllowedVideoExtensions,
		)
		cachetoml.Paths[idx].AllowedVideoExtensionsNoRenameLen = len(
			cachetoml.Paths[idx].AllowedVideoExtensionsNoRename,
		)
		cachetoml.Paths[idx].BlockedLen = len(cachetoml.Paths[idx].Blocked)
		cachetoml.Paths[idx].DisallowedLen = len(cachetoml.Paths[idx].Disallowed)
		cachetoml.Paths[idx].MaxSizeByte = int64(cachetoml.Paths[idx].MaxSize) * 1024 * 1024
		cachetoml.Paths[idx].MinSizeByte = int64(cachetoml.Paths[idx].MinSize) * 1024 * 1024
		cachetoml.Paths[idx].MinVideoSizeByte = int64(
			cachetoml.Paths[idx].MinVideoSize,
		) * 1024 * 1024
		SettingsPath[cachetoml.Paths[idx].Name] = &cachetoml.Paths[idx]
	}
	for idx := range cachetoml.Regex {
		cachetoml.Regex[idx].RejectedLen = len(cachetoml.Regex[idx].Rejected)
		cachetoml.Regex[idx].RequiredLen = len(cachetoml.Regex[idx].Required)
		SettingsRegex[cachetoml.Regex[idx].Name] = &cachetoml.Regex[idx]
	}
	for idx := range cachetoml.Scheduler {
		SettingsScheduler[cachetoml.Scheduler[idx].Name] = &cachetoml.Scheduler[idx]
	}
	for idx := range cachetoml.Quality {
		cachetoml.Quality[idx].IndexerCfg = make(
			[]*IndexersConfig,
			len(cachetoml.Quality[idx].Indexer),
		)
		for idx2 := range cachetoml.Quality[idx].Indexer {
			cachetoml.Quality[idx].Indexer[idx2].CfgDownloader = SettingsDownloader[cachetoml.Quality[idx].Indexer[idx2].TemplateDownloader]
			cachetoml.Quality[idx].Indexer[idx2].CfgIndexer = SettingsIndexer[cachetoml.Quality[idx].Indexer[idx2].TemplateIndexer]
			cachetoml.Quality[idx].IndexerCfg[idx2] = SettingsIndexer[cachetoml.Quality[idx].Indexer[idx2].TemplateIndexer]
			cachetoml.Quality[idx].Indexer[idx2].CfgPath = SettingsPath[cachetoml.Quality[idx].Indexer[idx2].TemplatePathNzb]
			cachetoml.Quality[idx].Indexer[idx2].CfgRegex = SettingsRegex[cachetoml.Quality[idx].Indexer[idx2].TemplateRegex]
		}
		cachetoml.Quality[idx].IndexerLen = len(cachetoml.Quality[idx].Indexer)
		cachetoml.Quality[idx].QualityReorderLen = len(cachetoml.Quality[idx].QualityReorder)
		cachetoml.Quality[idx].TitleStripPrefixForSearchLen = len(
			cachetoml.Quality[idx].TitleStripPrefixForSearch,
		)
		cachetoml.Quality[idx].TitleStripSuffixForSearchLen = len(
			cachetoml.Quality[idx].TitleStripSuffixForSearch,
		)
		cachetoml.Quality[idx].WantedAudioLen = len(cachetoml.Quality[idx].WantedAudio)
		cachetoml.Quality[idx].WantedCodecLen = len(cachetoml.Quality[idx].WantedCodec)
		cachetoml.Quality[idx].WantedQualityLen = len(cachetoml.Quality[idx].WantedQuality)
		cachetoml.Quality[idx].WantedResolutionLen = len(cachetoml.Quality[idx].WantedResolution)
		SettingsQuality[cachetoml.Quality[idx].Name] = &cachetoml.Quality[idx]
	}
	for idx := range cachetoml.Media.Movies {
		cachetoml.Media.Movies[idx].DataMap = make(
			map[int]*MediaDataConfig,
			len(cachetoml.Media.Movies[idx].Data),
		)
		cachetoml.Media.Movies[idx].DataImportMap = make(
			map[int]*MediaDataImportConfig,
			len(cachetoml.Media.Movies[idx].DataImport),
		)
		for idx2 := range cachetoml.Media.Movies[idx].Data {
			cachetoml.Media.Movies[idx].Data[idx2].CfgPath = SettingsPath[cachetoml.Media.Movies[idx].Data[idx2].TemplatePath]
			if cachetoml.Media.Movies[idx].Data[idx2].AddFoundList != "" {
				cachetoml.Media.Movies[idx].Data[idx2].AddFoundListCfg = SettingsList[cachetoml.Media.Movies[idx].Data[idx2].AddFoundList]
			}
			cachetoml.Media.Movies[idx].DataMap[idx2] = &cachetoml.Media.Movies[idx].Data[idx2]
		}
		for idx2 := range cachetoml.Media.Movies[idx].DataImport {
			cachetoml.Media.Movies[idx].DataImport[idx2].CfgPath = SettingsPath[cachetoml.Media.Movies[idx].DataImport[idx2].TemplatePath]
			cachetoml.Media.Movies[idx].DataImportMap[idx2] = &cachetoml.Media.Movies[idx].DataImport[idx2]
		}
		for idx2 := range cachetoml.Media.Movies[idx].Notification {
			cachetoml.Media.Movies[idx].Notification[idx2].CfgNotification = SettingsNotification[cachetoml.Media.Movies[idx].Notification[idx2].MapNotification]
		}
		cachetoml.Media.Movies[idx].CfgQuality = SettingsQuality[cachetoml.Media.Movies[idx].TemplateQuality]
		cachetoml.Media.Movies[idx].CfgScheduler = SettingsScheduler[cachetoml.Media.Movies[idx].TemplateScheduler]
		cachetoml.Media.Movies[idx].NamePrefix = "movie_" + cachetoml.Media.Movies[idx].Name
		cachetoml.Media.Movies[idx].Useseries = false
		cachetoml.Media.Movies[idx].ListsMap = make(
			map[string]*MediaListsConfig,
			len(cachetoml.Media.Movies[idx].Lists),
		)
		cachetoml.Media.Movies[idx].ListsMapIdx = make(
			map[string]int,
			len(cachetoml.Media.Movies[idx].Lists),
		)
		if len(cachetoml.Media.Movies[idx].Lists) >= 1 {
			cachetoml.Media.Movies[idx].ListsQu = strings.Repeat(
				",?",
				len(cachetoml.Media.Movies[idx].Lists)-1,
			)
		}
		cachetoml.Media.Movies[idx].ListsLen = len(cachetoml.Media.Movies[idx].Lists)
		cachetoml.Media.Movies[idx].MetadataTitleLanguagesLen = len(
			cachetoml.Media.Movies[idx].MetadataTitleLanguages,
		)
		cachetoml.Media.Movies[idx].DataLen = len(cachetoml.Media.Movies[idx].Data)
		cachetoml.Media.Movies[idx].ListsQualities = make(
			[]string,
			0,
			len(cachetoml.Media.Movies[idx].Lists),
		)
		for idxsub := range cachetoml.Media.Movies[idx].Lists {
			cachetoml.Media.Movies[idx].Lists[idxsub].CfgList = SettingsList[cachetoml.Media.Movies[idx].Lists[idxsub].TemplateList]
			cachetoml.Media.Movies[idx].Lists[idxsub].CfgQuality = SettingsQuality[cachetoml.Media.Movies[idx].Lists[idxsub].TemplateQuality]
			cachetoml.Media.Movies[idx].Lists[idxsub].CfgScheduler = SettingsScheduler[cachetoml.Media.Movies[idx].Lists[idxsub].TemplateScheduler]
			if len(cachetoml.Media.Movies[idx].Lists[idxsub].IgnoreMapLists) >= 1 {
				cachetoml.Media.Movies[idx].Lists[idxsub].IgnoreMapListsQu = strings.Repeat(
					",?",
					len(cachetoml.Media.Movies[idx].Lists[idxsub].IgnoreMapLists)-1,
				)
			}
			cachetoml.Media.Movies[idx].Lists[idxsub].IgnoreMapListsLen = len(
				cachetoml.Media.Movies[idx].Lists[idxsub].IgnoreMapLists,
			)
			cachetoml.Media.Movies[idx].Lists[idxsub].ReplaceMapListsLen = len(
				cachetoml.Media.Movies[idx].Lists[idxsub].ReplaceMapLists,
			)
			if !slices.Contains(
				cachetoml.Media.Movies[idx].ListsQualities,
				cachetoml.Media.Movies[idx].Lists[idxsub].TemplateQuality,
			) {
				cachetoml.Media.Movies[idx].ListsQualities = append(
					cachetoml.Media.Movies[idx].ListsQualities,
					cachetoml.Media.Movies[idx].Lists[idxsub].TemplateQuality,
				)
			}
			cachetoml.Media.Movies[idx].ListsMap[cachetoml.Media.Movies[idx].Lists[idxsub].Name] = &cachetoml.Media.Movies[idx].Lists[idxsub]
			cachetoml.Media.Movies[idx].ListsMapIdx[cachetoml.Media.Movies[idx].Lists[idxsub].Name] = idxsub
		}

		SettingsMedia["movie_"+cachetoml.Media.Movies[idx].Name] = &cachetoml.Media.Movies[idx]
	}
	for idx := range cachetoml.Media.Series {
		cachetoml.Media.Series[idx].DataMap = make(
			map[int]*MediaDataConfig,
			len(cachetoml.Media.Series[idx].Data),
		)
		cachetoml.Media.Series[idx].DataImportMap = make(
			map[int]*MediaDataImportConfig,
			len(cachetoml.Media.Series[idx].DataImport),
		)
		for idx2 := range cachetoml.Media.Series[idx].Data {
			cachetoml.Media.Series[idx].Data[idx2].CfgPath = SettingsPath[cachetoml.Media.Series[idx].Data[idx2].TemplatePath]
			cachetoml.Media.Series[idx].DataMap[idx2] = &cachetoml.Media.Series[idx].Data[idx2]
		}
		for idx2 := range cachetoml.Media.Series[idx].DataImport {
			cachetoml.Media.Series[idx].DataImport[idx2].CfgPath = SettingsPath[cachetoml.Media.Series[idx].DataImport[idx2].TemplatePath]
			cachetoml.Media.Series[idx].DataImportMap[idx2] = &cachetoml.Media.Series[idx].DataImport[idx2]
		}
		for idx2 := range cachetoml.Media.Series[idx].Notification {
			cachetoml.Media.Series[idx].Notification[idx2].CfgNotification = SettingsNotification[cachetoml.Media.Series[idx].Notification[idx2].MapNotification]
		}
		cachetoml.Media.Series[idx].CfgQuality = SettingsQuality[cachetoml.Media.Series[idx].TemplateQuality]
		cachetoml.Media.Series[idx].CfgScheduler = SettingsScheduler[cachetoml.Media.Series[idx].TemplateScheduler]
		cachetoml.Media.Series[idx].NamePrefix = "serie_" + cachetoml.Media.Series[idx].Name
		cachetoml.Media.Series[idx].Useseries = true
		cachetoml.Media.Series[idx].ListsMap = make(
			map[string]*MediaListsConfig,
			len(cachetoml.Media.Series[idx].Lists),
		)
		cachetoml.Media.Series[idx].ListsMapIdx = make(
			map[string]int,
			len(cachetoml.Media.Series[idx].Lists),
		)
		if len(cachetoml.Media.Series[idx].Lists) >= 1 {
			cachetoml.Media.Series[idx].ListsQu = strings.Repeat(
				",?",
				len(cachetoml.Media.Series[idx].Lists)-1,
			)
		}
		cachetoml.Media.Series[idx].ListsLen = len(cachetoml.Media.Series[idx].Lists)
		cachetoml.Media.Series[idx].MetadataTitleLanguagesLen = len(
			cachetoml.Media.Series[idx].MetadataTitleLanguages,
		)
		cachetoml.Media.Series[idx].DataLen = len(cachetoml.Media.Series[idx].Data)
		cachetoml.Media.Series[idx].ListsQualities = make(
			[]string,
			0,
			len(cachetoml.Media.Series[idx].Lists),
		)
		for idxsub := range cachetoml.Media.Series[idx].Lists {
			cachetoml.Media.Series[idx].Lists[idxsub].CfgList = SettingsList[cachetoml.Media.Series[idx].Lists[idxsub].TemplateList]
			cachetoml.Media.Series[idx].Lists[idxsub].CfgQuality = SettingsQuality[cachetoml.Media.Series[idx].Lists[idxsub].TemplateQuality]
			cachetoml.Media.Series[idx].Lists[idxsub].CfgScheduler = SettingsScheduler[cachetoml.Media.Series[idx].Lists[idxsub].TemplateScheduler]
			if len(cachetoml.Media.Series[idx].Lists[idxsub].IgnoreMapLists) >= 1 {
				cachetoml.Media.Series[idx].Lists[idxsub].IgnoreMapListsQu = strings.Repeat(
					",?",
					len(cachetoml.Media.Series[idx].Lists[idxsub].IgnoreMapLists)-1,
				)
			}
			cachetoml.Media.Series[idx].Lists[idxsub].IgnoreMapListsLen = len(
				cachetoml.Media.Series[idx].Lists[idxsub].IgnoreMapLists,
			)
			cachetoml.Media.Series[idx].Lists[idxsub].ReplaceMapListsLen = len(
				cachetoml.Media.Series[idx].Lists[idxsub].ReplaceMapLists,
			)
			if !slices.Contains(
				cachetoml.Media.Series[idx].ListsQualities,
				cachetoml.Media.Series[idx].Lists[idxsub].TemplateQuality,
			) {
				cachetoml.Media.Series[idx].ListsQualities = append(
					cachetoml.Media.Series[idx].ListsQualities,
					cachetoml.Media.Series[idx].Lists[idxsub].TemplateQuality,
				)
			}
			cachetoml.Media.Series[idx].ListsMap[cachetoml.Media.Series[idx].Lists[idxsub].Name] = &cachetoml.Media.Series[idx].Lists[idxsub]
			cachetoml.Media.Series[idx].ListsMapIdx[cachetoml.Media.Series[idx].Lists[idxsub].Name] = idxsub
		}
		SettingsMedia["serie_"+cachetoml.Media.Series[idx].Name] = &cachetoml.Media.Series[idx]
	}
}

// UpdateCfg updates the application configuration settings based on the
// provided Conf struct. It iterates through the configIn slice, checks
// the prefix of the Name field to determine the config type, casts the
// Data field to the appropriate config struct, and saves the config
// values to the respective global settings maps. This allows the config
// system to be extended by adding new config types.
func UpdateCfg(configIn []Conf) {
	mu.Lock()
	defer mu.Unlock()
	for _, val := range configIn {
		if strings.HasPrefix(val.Name, "general") {
			SettingsGeneral = val.Data.(GeneralConfig)
		}
		if strings.HasPrefix(val.Name, "downloader_") {
			data := val.Data.(DownloaderConfig)
			SettingsDownloader[val.Data.(DownloaderConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, logger.StrImdb) {
			SettingsImdb = val.Data.(ImdbConfig)
		}
		if strings.HasPrefix(val.Name, "indexer") {
			data := val.Data.(IndexersConfig)
			SettingsIndexer[val.Data.(IndexersConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, "list") {
			data := val.Data.(ListsConfig)
			SettingsList[val.Data.(ListsConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, logger.StrSerie) {
			data := val.Data.(MediaTypeConfig)
			SettingsMedia["serie_"+val.Data.(MediaTypeConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, logger.StrMovie) {
			data := val.Data.(MediaTypeConfig)
			SettingsMedia["movie_"+val.Data.(MediaTypeConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, "notification") {
			data := val.Data.(NotificationConfig)
			SettingsNotification[val.Data.(NotificationConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, "path") {
			data := val.Data.(PathsConfig)
			SettingsPath[val.Data.(PathsConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, "quality") {
			data := val.Data.(QualityConfig)
			SettingsQuality[val.Data.(QualityConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, "regex") {
			data := val.Data.(RegexConfig)
			SettingsRegex[val.Data.(RegexConfig).Name] = &data
		}
		if strings.HasPrefix(val.Name, "scheduler") {
			data := val.Data.(SchedulerConfig)
			SettingsScheduler[val.Data.(SchedulerConfig).Name] = &data
		}
	}
}

// GetCfgAll returns a map containing all the application configuration settings.
// It collects the settings from the various global config variables and organizes
// them into a single map indexed by config section name prefixes.
func GetCfgAll() map[string]any {
	mu.RLock()
	defer mu.RUnlock()
	q := make(map[string]any)
	q["general"] = SettingsGeneral
	q["imdb"] = SettingsImdb
	for key := range SettingsMedia {
		q[SettingsMedia[key].NamePrefix] = *SettingsMedia[key]
	}
	for key := range SettingsDownloader {
		q["downloader_"+key] = *SettingsDownloader[key]
	}
	for key := range SettingsIndexer {
		q["indexer_"+key] = *SettingsIndexer[key]
	}
	for key := range SettingsList {
		q["list_"+key] = *SettingsList[key]
	}
	for key := range SettingsNotification {
		q["notification_"+key] = *SettingsNotification[key]
	}
	for key := range SettingsPath {
		q["path_"+key] = *SettingsPath[key]
	}
	for key := range SettingsQuality {
		q["quality_"+key] = *SettingsQuality[key]
	}
	for key := range SettingsRegex {
		q["regex_"+key] = *SettingsRegex[key]
	}
	for key := range SettingsScheduler {
		q["scheduler_"+key] = *SettingsScheduler[key]
	}
	return q
}

// UpdateCfgEntry updates the application configuration settings
// based on the provided Conf struct. It saves the config values to
// the config database.
func UpdateCfgEntry(configIn Conf) {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{SyncInterval: 0})

	defer configDB.Close()

	mu.Lock()
	defer mu.Unlock()
	if strings.HasPrefix(configIn.Name, "general") {
		SettingsGeneral = configIn.Data.(GeneralConfig)
	}
	if strings.HasPrefix(configIn.Name, "downloader_") {
		data := configIn.Data.(DownloaderConfig)
		SettingsDownloader[configIn.Data.(DownloaderConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, logger.StrImdb) {
		SettingsImdb = configIn.Data.(ImdbConfig)
	}
	if strings.HasPrefix(configIn.Name, "indexer") {
		data := configIn.Data.(IndexersConfig)
		SettingsIndexer[configIn.Data.(IndexersConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, "list") {
		data := configIn.Data.(ListsConfig)
		SettingsList[configIn.Data.(ListsConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, logger.StrSerie) {
		data := configIn.Data.(MediaTypeConfig)
		SettingsMedia["serie_"+configIn.Data.(MediaTypeConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, logger.StrMovie) {
		data := configIn.Data.(MediaTypeConfig)
		SettingsMedia["movie_"+configIn.Data.(MediaTypeConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, "notification") {
		data := configIn.Data.(NotificationConfig)
		SettingsNotification[configIn.Data.(NotificationConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, "path") {
		data := configIn.Data.(PathsConfig)
		SettingsPath[configIn.Data.(PathsConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, "quality") {
		data := configIn.Data.(QualityConfig)
		SettingsQuality[configIn.Data.(QualityConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, "regex") {
		data := configIn.Data.(RegexConfig)
		SettingsRegex[configIn.Data.(RegexConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, "scheduler") {
		data := configIn.Data.(SchedulerConfig)
		SettingsScheduler[configIn.Data.(SchedulerConfig).Name] = &data
	}
	if strings.HasPrefix(configIn.Name, "trakt_token") {
		traktToken = configIn.Data.(*oauth2.Token)
		configDB.Set("trakt_token", *configIn.Data.(*oauth2.Token))
	}
}

// DeleteCfgEntry deletes the configuration entry with the given name from
// the config database and clears the corresponding value in the in-memory
// config maps. It handles entries for all major configuration categories like
// general, downloader, indexer etc.
func DeleteCfgEntry(name string) {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{SyncInterval: 0})

	defer configDB.Close()

	mu.Lock()
	defer mu.Unlock()
	if strings.HasPrefix(name, "general") {
		SettingsGeneral = GeneralConfig{}
	}
	if strings.HasPrefix(name, "downloader_") {
		delete(SettingsDownloader, name)
	}
	if strings.HasPrefix(name, logger.StrImdb) {
		SettingsImdb = ImdbConfig{}
	}
	if strings.HasPrefix(name, "indexer") {
		delete(SettingsIndexer, name)
	}
	if strings.HasPrefix(name, "list") {
		delete(SettingsList, name)
	}
	if strings.HasPrefix(name, logger.StrSerie) {
		delete(SettingsMedia, name)
	}
	if strings.HasPrefix(name, logger.StrMovie) {
		delete(SettingsMedia, name)
	}
	if strings.HasPrefix(name, "notification") {
		delete(SettingsNotification, name)
	}
	if strings.HasPrefix(name, "path") {
		delete(SettingsPath, name)
	}
	if strings.HasPrefix(name, "quality") {
		delete(SettingsQuality, name)
	}
	if strings.HasPrefix(name, "regex") {
		delete(SettingsRegex, name)
	}
	if strings.HasPrefix(name, "scheduler") {
		delete(SettingsScheduler, name)
	}

	if strings.HasPrefix(name, "trakt_token") {
		configDB.Delete("trakt_token")
	}
}

// GetToml returns the cached main configuration settings as a mainConfig struct.
// This function provides read-only access to the current configuration state.
func GetToml() mainConfig {
	return cachetoml
}

// ClearCfg clears all configuration settings by deleting the config database file,
// resetting all config maps to empty maps, and reinitializing default settings.
// It wipes the existing config and starts fresh with defaults.
func ClearCfg() {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{SyncInterval: 0})
	defer configDB.Close()
	configDB.DeleteFile()

	ClearSettings()

	cachetoml = mainConfig{
		General: GeneralConfig{
			LogLevel:       "Info",
			DBLogLevel:     "Info",
			LogFileCount:   5,
			LogFileSize:    5,
			LogCompress:    false,
			WebAPIKey:      "mysecure",
			WebPort:        "9090",
			WorkerMetadata: 1,
			WorkerFiles:    1,
			WorkerParse:    1,
			WorkerSearch:   1,
			WorkerIndexer:  1,
			// ConcurrentScheduler: 1,
			OmdbLimiterSeconds:  1,
			OmdbLimiterCalls:    1,
			TmdbLimiterSeconds:  1,
			TmdbLimiterCalls:    1,
			TraktLimiterSeconds: 1,
			TraktLimiterCalls:   1,
			TvdbLimiterSeconds:  1,
			TvdbLimiterCalls:    1,
			SchedulerDisabled:   true,
		},
		Scheduler: []SchedulerConfig{{
			Name:                       "Default",
			IntervalImdb:               "3d",
			IntervalFeeds:              "1d",
			IntervalFeedsRefreshSeries: "1d",
			IntervalFeedsRefreshMovies: "1d",
			IntervalIndexerMissing:     "40m",
			IntervalIndexerUpgrade:     "60m",
			IntervalIndexerRss:         "15m",
			IntervalScanData:           "1h",
			IntervalScanDataMissing:    "1d",
			IntervalScanDataimport:     "60m",
		}},
		Downloader: []DownloaderConfig{{
			Name:   "initial",
			DlType: "drone",
		}},
		Imdbindexer: ImdbConfig{
			Indexedtypes:     []string{logger.StrMovie},
			Indexedlanguages: []string{"US", "UK", "\\N"},
		},
		Indexers: []IndexersConfig{{
			Name:           "initial",
			IndexerType:    "newznab",
			Limitercalls:   5,
			Limiterseconds: 20,
			MaxEntries:     100,
			RssEntriesloop: 2,
		}},
		Lists: []ListsConfig{{
			Name:     "initial",
			ListType: "traktmovieanticipated",
			Limit:    "20",
		}},
		Media: MediaConfig{
			Movies: []MediaTypeConfig{{
				Name:              "initial",
				TemplateQuality:   "initial",
				TemplateScheduler: "Default",
				Data:              []MediaDataConfig{{TemplatePath: "initial"}},
				DataImport:        []MediaDataImportConfig{{TemplatePath: "initial"}},
				Lists: []MediaListsConfig{{
					TemplateList:      "initial",
					TemplateQuality:   "initial",
					TemplateScheduler: "Default",
				}},
				Notification: []mediaNotificationConfig{{MapNotification: "initial"}},
			}},
			Series: []MediaTypeConfig{{
				Name:              "initial",
				TemplateQuality:   "initial",
				TemplateScheduler: "Default",
				Data:              []MediaDataConfig{{TemplatePath: "initial"}},
				DataImport:        []MediaDataImportConfig{{TemplatePath: "initial"}},
				Lists: []MediaListsConfig{{
					TemplateList:      "initial",
					TemplateQuality:   "initial",
					TemplateScheduler: "Default",
				}},
				Notification: []mediaNotificationConfig{{MapNotification: "initial"}},
			}},
		},
		Notification: []NotificationConfig{{
			Name:             "initial",
			NotificationType: "csv",
		}},
		Paths: []PathsConfig{{
			Name:                   "initial",
			AllowedVideoExtensions: []string{".avi", ".mkv", ".mp4"},
			AllowedOtherExtensions: []string{".idx", ".sub", ".srt"},
		}},
		Quality: []QualityConfig{{
			Name:           "initial",
			QualityReorder: []QualityReorderConfig{{}},
			Indexer: []QualityIndexerConfig{{
				TemplateIndexer:    "initial",
				TemplateDownloader: "initial",
				TemplateRegex:      "initial",
				TemplatePathNzb:    "initial",
			}},
		}},
		Regex: []RegexConfig{{
			Name: "initial",
		}},
	}

	Getconfigtoml()
}

// WriteCfg marshals the application configuration structs into a TOML
// configuration file. It gathers all the global configuration structs,
// assembles them into a MainConfig struct, marshals to TOML and writes
// to the Configfile location.
func WriteCfg() {
	var bla mainConfig

	mu.Lock()
	defer mu.Unlock()

	bla.General = SettingsGeneral
	bla.Imdbindexer = SettingsImdb
	for _, cfgdata := range SettingsDownloader {
		bla.Downloader = append(bla.Downloader, *cfgdata)
	}
	for _, cfgdata := range SettingsIndexer {
		bla.Indexers = append(bla.Indexers, *cfgdata)
	}
	for _, cfgdata := range SettingsList {
		bla.Lists = append(bla.Lists, *cfgdata)
	}
	for _, cfgdata := range cachetoml.Media.Series {
		if !strings.HasPrefix(cfgdata.NamePrefix, logger.StrSerie) {
			continue
		}
		bla.Media.Series = append(bla.Media.Series, cfgdata)
	}
	for _, cfgdata := range cachetoml.Media.Movies {
		if !strings.HasPrefix(cfgdata.NamePrefix, logger.StrMovie) {
			continue
		}
		bla.Media.Movies = append(bla.Media.Movies, cfgdata)
	}
	for _, cfgdata := range SettingsNotification {
		bla.Notification = append(bla.Notification, *cfgdata)
	}
	for _, cfgdata := range SettingsPath {
		bla.Paths = append(bla.Paths, *cfgdata)
	}
	for _, cfgdata := range SettingsQuality {
		bla.Quality = append(bla.Quality, *cfgdata)
	}
	for _, cfgdata := range SettingsRegex {
		bla.Regex = append(
			bla.Regex,
			RegexConfig{Name: cfgdata.Name, Required: cfgdata.Required, Rejected: cfgdata.Rejected},
		)
	}
	for _, cfgdata := range SettingsScheduler {
		bla.Scheduler = append(bla.Scheduler, *cfgdata)
	}

	cnt, err := toml.Marshal(bla)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
	}
	os.WriteFile(Configfile, cnt, 0o777)
	cachetoml = bla
}

func GetSettingsGeneral() *GeneralConfig {
	mu.RLock()
	defer mu.RUnlock()
	return &SettingsGeneral
}

func GetSettingsImdb() *ImdbConfig {
	mu.RLock()
	defer mu.RUnlock()
	return &SettingsImdb
}

func GetSettingsMedia(name string) *MediaTypeConfig {
	mu.RLock()
	defer mu.RUnlock()
	return SettingsMedia[name]
}

func GetSettingsPath(name string) *PathsConfig {
	mu.RLock()
	defer mu.RUnlock()
	return SettingsPath[name]
}

func GetSettingsQuality(name string) *QualityConfig {
	mu.RLock()
	defer mu.RUnlock()
	return SettingsQuality[name]
}

func GetSettingsQualityOk(name string) (*QualityConfig, bool) {
	mu.RLock()
	defer mu.RUnlock()
	val, ok := SettingsQuality[name]
	return val, ok
}

func GetSettingsScheduler(name string) *SchedulerConfig {
	mu.RLock()
	defer mu.RUnlock()
	return SettingsScheduler[name]
}

func GetSettingsList(name string) *ListsConfig {
	mu.RLock()
	defer mu.RUnlock()
	return SettingsList[name]
}

func TestSettingsList(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := SettingsList[name]
	return ok
}

func GetSettingsIndexer(name string) *IndexersConfig {
	mu.RLock()
	defer mu.RUnlock()
	return SettingsIndexer[name]
}

func GetSettingsNotification(name string) *NotificationConfig {
	mu.RLock()
	defer mu.RUnlock()
	return SettingsNotification[name]
}

func RangeSettingsMedia(fn func(string, *MediaTypeConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsMedia {
		fn(key, cfg)
	}
}

func RangeSettingsMediaLists(media string, fn func(*MediaListsConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for _, cfg := range SettingsMedia[media].Lists {
		fn(&cfg)
	}
}

func RangeSettingsMediaBreak(fn func(string, *MediaTypeConfig) bool) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsMedia {
		if fn(key, cfg) {
			break
		}
	}
}

func RangeSettingsQuality(fn func(string, *QualityConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsQuality {
		fn(key, cfg)
	}
}

func RangeSettingsList(fn func(string, *ListsConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsList {
		fn(key, cfg)
	}
}

func RangeSettingsIndexer(fn func(string, *IndexersConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsIndexer {
		fn(key, cfg)
	}
}

func RangeSettingsScheduler(fn func(string, *SchedulerConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsScheduler {
		fn(key, cfg)
	}
}

func RangeSettingsNotification(fn func(string, *NotificationConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsNotification {
		fn(key, cfg)
	}
}

func RangeSettingsPath(fn func(string, *PathsConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsPath {
		fn(key, cfg)
	}
}

func RangeSettingsRegex(fn func(string, *RegexConfig)) {
	mu.RLock()
	defer mu.RUnlock()
	for key, cfg := range SettingsRegex {
		fn(key, cfg)
	}
}

func RangeSettingsDownloader(fn func(string, *DownloaderConfig)) {
	mu.RLock()
	defer mu.RUnlock()

	for key, cfg := range SettingsDownloader {
		fn(key, cfg)
	}
}
