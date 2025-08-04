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
	Serie []SerieConfig `toml:"series" displayname:"TV Series Configurations" comment:"Array of TV series configurations for your media library.\nEach entry in this array defines a complete series configuration with:\n- Primary series name and TVDB ID for identification\n- Alternate names for improved release matching\n- Search and upgrade behavior settings\n- Episode identification format and runtime checking\n- Metadata sources and custom storage paths\nAdd one entry per TV series you want to monitor and download.\nExample: Add 'Breaking Bad', 'Game of Thrones', etc. as separate entries"`
}

// SerieConfig defines the configuration for a TV series.
type SerieConfig struct {
	// Name is the primary name for the series
	Name string `toml:"name" displayname:"Primary Series Name" comment:"Enter the primary name for this TV series.\nThis should be the most commonly used title for the show.\nExample: 'Breaking Bad' or 'Game of Thrones'"`

	// TvdbID is the tvdbid for the series for better searches
	TvdbID int `toml:"tvdb_id" displayname:"TVDB Database ID" comment:"Enter the numeric TVDB (TheTVDB.com) ID for this series.\nThis improves search accuracy and metadata retrieval.\nFind the ID by searching for your show on thetvdb.com and copying the number from the URL.\nExample: For 'Breaking Bad' use 81189"`

	// AlternateName specifies alternate names which the series is known for
	// Alternates from tvdb and trakt are added
	AlternateName []string `toml:"alternatename" displayname:"Alternate Series Names" comment:"Enter alternate titles that this series is known by.\nInclude foreign language titles, abbreviations, or common variations.\nThis helps find releases with different naming conventions.\nSeparate multiple names with commas in the array format.\nExample: ['BB', 'Breaking Bad US', 'Во все тяжкие']\nNote: Alternates from TVDB and Trakt are automatically added."`

	// DisallowedName specifies names which the series is not allowed to have
	// These are removed from Alternates from tvdb and trakt
	DisallowedName []string `toml:"disallowedname" displayname:"Disallowed Series Names" comment:"Enter titles that should NOT be associated with this series.\nThis prevents incorrect matches from releases with similar names.\nUseful when TVDB/Trakt automatically adds confusing alternate names.\nSeparate multiple names with commas in the array format.\nExample: ['Different Show', 'Breaking Bad Documentary']\nThese names will be excluded from automatic alternates."`

	// Identifiedby specifies how the media is structured, e.g. ep=S01E01, date=yy-mm-dd
	Identifiedby string `toml:"identifiedby" displayname:"Episode Identification Format" comment:"Specify how episodes are identified and organized.\nChoose the format that matches how your files are named:\n- 'ep' for standard episode numbering (S01E01, S02E03, etc.)\n- 'date' for date-based shows (YYYY-MM-DD format)\nMost TV series use 'ep', while daily shows use 'date'.\nExample: 'ep' for most shows, 'date' for news/talk shows"`

	// DontUpgrade indicates whether to skip searches for better versions of media
	DontUpgrade bool `toml:"dont_upgrade" displayname:"Disable Quality Upgrades" comment:"Set to true to disable quality upgrade searches for this series.\nWhen false, the system will automatically search for better quality versions\nof episodes you already have (e.g., upgrading from 720p to 1080p).\nSet to true if you're satisfied with current quality and want to save resources.\nDefault: false (upgrades enabled)"`

	// DontSearch indicates whether the series should not be searched
	DontSearch bool `toml:"dont_search" displayname:"Disable All Searches" comment:"Set to true to completely disable all searches for this series.\nThis stops both missing episode searches and quality upgrades.\nUseful for series that are discontinued, completed, or manually managed.\nWhen true, the series will remain in your library but won't be searched.\nDefault: false (searches enabled)"`

	// SearchSpecials indicates whether to also search Season 0 (specials)
	SearchSpecials bool `toml:"search_specials" displayname:"Search Season Zero Specials" comment:"Set to true to include Season 0 (specials) in searches.\nSeason 0 typically contains extras like behind-the-scenes content,\ndeleted scenes, webisodes, or special episodes that don't fit regular seasons.\nNote: Specials may have inconsistent naming and lower availability.\nDefault: false (specials not searched)"`

	// IgnoreRuntime indicates whether to ignore episode runtime checks
	IgnoreRuntime bool `toml:"ignore_runtime" displayname:"Skip Runtime Validation" comment:"Set to true to skip runtime validation for this series.\nWhen false, downloaded episodes are checked against expected runtime\nto ensure they're complete and not fake/incomplete files.\nSet to true for shows with highly variable episode lengths\nor when runtime checking causes issues with legitimate files.\nDefault: false (runtime checking enabled)"`

	// Source specifies the metadata source, e.g. none or tvdb
	Source string `toml:"source" displayname:"Metadata Source Provider" comment:"Specify the metadata source for this series information.\nAvailable options:\n- 'tvdb' to use TheTVDB.com for episode information and metadata\n- 'none' to disable automatic metadata fetching\nUsing 'tvdb' provides episode titles, air dates, and other metadata.\nUse 'none' for series with problematic or missing TVDB data.\nDefault: 'tvdb' (recommended for most series)"`

	// Target defines a specific path to use for the media
	// This path must also be in the media data section
	Target string `toml:"target" displayname:"Custom Storage Path" comment:"Optionally specify a custom path where this series should be stored.\nThis overrides the default path settings for this specific series.\nThe path must be an absolute path and must also be configured\nin the paths section of your configuration.\nLeave empty to use default path settings.\nExample: '/media/tv/special-shows/SeriesName'\nNote: The path must exist and be accessible to the application."`
}

// Main Config
// mainConfig struct defines the overall configuration
// It contains fields for each configuration section.
type MainConfig struct {
	// GeneralConfig contains general configuration settings
	General GeneralConfig `toml:"general" displayname:"General Application Settings" comment:"General application settings including logging, workers, and caching.\nThis section controls core behavior like log levels, worker threads,\nAPI keys, and performance-related cache settings.\nRequired section - must be configured for proper operation."`

	// ImdbConfig contains IMDB specific configuration
	Imdbindexer ImdbConfig `toml:"imdbindexer" displayname:"IMDB Database Configuration" comment:"IMDB database configuration for movie metadata and indexing.\nControls how IMDB data is downloaded, processed, and stored locally.\nIncludes settings for database paths, update intervals, and data filtering.\nOptional section - only needed if using IMDB metadata features."`

	// mediaConfig contains media related configuration
	Media MediaConfig `toml:"media" displayname:"Media Type Configuration" comment:"Media type definitions for movies and TV series.\nDefines how different media types are handled, including\ndata sources, quality profiles, notification settings, and search behavior.\nRequired section - must define at least one media type."`

	// DownloaderConfig defines downloader specific configuration
	Downloader []DownloaderConfig `toml:"downloader" displayname:"Download Client Configurations" comment:"Download client configurations for handling media downloads.\nDefine connections to SABnzbd, NZBGet, qBittorrent, Transmission, etc.\nEach entry specifies connection details, categories, and authentication.\nRequired section - must have at least one configured downloader."`

	// ListsConfig contains configuration for lists
	Lists []ListsConfig `toml:"lists" displayname:"External List Configurations" comment:"External list configurations for automatic media discovery.\nConnect to IMDB lists, Trakt lists, RSS feeds, and other sources\nto automatically add new media to your wanted lists.\nOptional section - only needed if using automatic list imports."`

	// IndexersConfig defines configuration for indexers
	Indexers []IndexersConfig `toml:"indexers" displayname:"Search Indexer Configurations" comment:"Search indexer configurations for finding media releases.\nDefine connections to Usenet indexers and torrent trackers.\nEach entry includes API keys, search limits, and connection settings.\nRequired section - must have at least one configured indexer."`

	// PathsConfig contains configuration for paths
	Paths []PathsConfig `toml:"paths" displayname:"File System Path Configurations" comment:"File system path configurations for media storage and organization.\nDefine where media files are stored, file extensions, size limits,\nand file management behaviors like upgrades and cleanup.\nRequired section - must define at least one path configuration."`

	// NotificationConfig contains configuration for notifications
	Notification []NotificationConfig `toml:"notification" displayname:"Notification Service Configurations" comment:"Notification service configurations for download alerts.\nSet up Pushover, email, webhooks, or file-based notifications\nto get alerts when media is downloaded or other events occur.\nOptional section - only needed if you want notifications."`

	// RegexConfig contains configuration for regex
	Regex []RegexConfig `toml:"regex" displayname:"Regular Expression Configurations" comment:"Regular expression configurations for filtering search results.\nDefine patterns to require or reject specific release characteristics\nlike group names, file naming conventions, or quality indicators.\nOptional section - only needed for advanced filtering requirements."`

	// QualityConfig contains configuration for quality
	Quality []QualityConfig `toml:"quality" displayname:"Quality Profile Configurations" comment:"Quality profile configurations defining preferred media characteristics.\nSet desired video resolutions, audio quality, codecs, and search behavior.\nEach profile can have different indexer and quality preferences.\nRequired section - must define at least one quality profile."`

	// SchedulerConfig contains configuration for scheduler
	Scheduler []SchedulerConfig `toml:"scheduler" displayname:"Task Scheduler Configurations" comment:"Task scheduler configurations for automated operations.\nDefine intervals or cron schedules for searches, scans, backups,\nand other maintenance tasks. Controls when and how often tasks run.\nRequired section - must define at least one scheduler configuration."`
}

type GeneralConfig struct {
	// TimeFormat defines the time format to use, options are rfc3339, iso8601, rfc1123, rfc822, rfc850 - default: rfc3339
	TimeFormat string `toml:"time_format" displayname:"Log Timestamp Format" comment:"Specify the timestamp format used in logs and API responses.\nAvailable options:\n- 'rfc3339': 2023-01-15T14:30:45Z (recommended, ISO 8601 compatible)\n- 'iso8601': 2023-01-15T14:30:45+00:00\n- 'rfc1123': Sun, 15 Jan 2023 14:30:45 GMT\n- 'rfc822': 15 Jan 23 14:30 GMT\n- 'rfc850': Sunday, 15-Jan-23 14:30:45 GMT\nDefault: 'rfc3339'"`
	// TimeZone defines the timezone to use, options are local, utc or one from IANA Time Zone database
	TimeZone string `toml:"time_zone" displayname:"Application Timezone" comment:"Set the timezone for timestamp display and scheduling.\nOptions:\n- 'local': Use system's local timezone\n- 'utc': Use Coordinated Universal Time\n- IANA timezone: Use specific timezone (e.g., 'America/New_York', 'Europe/London')\nFind IANA timezones at: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones\nExample: 'America/Los_Angeles' or 'Europe/Berlin'"`
	// LogLevel defines the log level to use, options are info or debug - default: info
	LogLevel string `toml:"log_level" displayname:"Application Log Level" comment:"Set the application logging verbosity level.\nOptions:\n- 'info': Standard logging with important events and errors\n- 'debug': Detailed logging including debug information (verbose)\nUse 'info' for normal operation, 'debug' for troubleshooting.\nWarning: Debug level generates large log files.\nDefault: 'info'"`
	// DBLogLevel defines the database log level to use, options are info or debug (not recommended) - default: info
	DBLogLevel string `toml:"db_log_level" displayname:"Database Log Level" comment:"Set the database operation logging level.\nOptions:\n- 'info': Log important database operations only\n- 'debug': Log all SQL queries and database operations (very verbose)\nWarning: Debug level creates extremely large logs and impacts performance.\nOnly use 'debug' for database troubleshooting.\nDefault: 'info'"`
	// LogFileSize defines the size in MB for the log files - default: 5
	LogFileSize int `toml:"log_file_size" displayname:"Log File Size MB" comment:"Maximum size in megabytes for each log file before rotation.\nWhen a log file reaches this size, it will be rotated and a new file started.\nLarger files mean less frequent rotation but harder to manage.\nSmaller files rotate more often but are easier to read.\nRecommended range: 5-50 MB\nDefault: 5"`
	// LogFileCount defines how many log files to keep - default: 1
	LogFileCount uint8 `toml:"log_file_count" displayname:"Rotated Log File Count" comment:"Number of rotated log files to retain before deletion.\nWhen log rotation occurs, this many old files will be kept.\nHigher values preserve more history but use more disk space.\nSet to 0 to keep only the current log file.\nRecommended range: 1-10\nDefault: 1"`
	// LogCompress defines whether to compress old log files - default: false
	LogCompress bool `toml:"log_compress" displayname:"Compress Old Log Files" comment:"Enable compression of rotated log files to save disk space.\nWhen true, old log files are compressed using gzip compression.\nThis significantly reduces disk usage but makes logs harder to read directly.\nUse true if disk space is limited and you rarely need to read old logs.\nDefault: false"`
	// LogToFileOnly defines whether to only log to file and not console - default: false
	LogToFileOnly bool `toml:"log_to_file_only" displayname:"Log Only To Files" comment:"Disable console output and log only to files.\nWhen true, all log messages go only to log files, not the console.\nUseful for background services or when console output causes issues.\nWhen false, logs appear both in files and console output.\nDefault: false (logs to both console and file)"`
	// LogColorize defines whether to use colors in console output - default: false
	LogColorize bool `toml:"log_colorize" displayname:"Enable Colored Console Output" comment:"Enable colored console output for better log readability.\nWhen true, different log levels are displayed in different colors\n(errors in red, warnings in yellow, etc.).\nMay not work properly in all terminal environments.\nDisable if colors cause display issues or when redirecting output.\nDefault: false"`
	// LogZeroValues determines whether to log variables without a value.
	LogZeroValues bool `toml:"log_zero_values" displayname:"Log Empty Values" comment:"Include empty/zero values in log output for debugging.\nWhen true, variables with empty strings, zero numbers, etc. are logged.\nUseful for debugging configuration issues but creates more verbose logs.\nWhen false, only variables with actual values are logged.\nDefault: false"`
	// WorkerMetadata defines how many parallel jobs of list retrievals to run - default: 1
	WorkerMetadata int `toml:"worker_metadata" displayname:"Metadata Worker Threads" comment:"Number of parallel workers for metadata and list retrieval tasks.\nHigher values speed up processing of IMDB lists, Trakt lists, etc.\nToo many workers may overwhelm external APIs or cause rate limiting.\nRecommended range: 1-5 depending on your system and API limits.\nDefault: 1"`
	// WorkerFiles defines how many parallel jobs of file scanning to run - default: 1
	WorkerFiles int `toml:"worker_files" displayname:"File Scanner Worker Threads" comment:"Number of parallel workers for file system scanning operations.\nHigher values can speed up large library scans but increase I/O load.\nMore workers may cause issues with network storage or slow drives.\nRecommended: Keep at 1 unless you have very fast local storage.\nDefault: 1"`

	// WorkerParse defines how many parallel parsings to run for list retrievals - default: 1
	WorkerParse int `toml:"worker_parse" displayname:"List Parser Worker Threads" comment:"Number of parallel workers for parsing list data (RSS, CSV, etc.).\nHigher values speed up processing of large lists and feeds.\nMore workers increase CPU usage but reduce processing time.\nUseful when importing large IMDB lists or processing many RSS feeds.\nRecommended range: 1-4\nDefault: 1"`
	// WorkerSearch defines how many parallel search jobs to run - default: 1
	WorkerSearch int `toml:"worker_search" displayname:"Search Worker Threads" comment:"Number of parallel workers for search operations (missing/upgrade scans).\nHigher values speed up searches but increase resource usage.\nToo many workers may overwhelm indexers or cause rate limiting.\nBalance between speed and indexer limits/system resources.\nRecommended range: 1-3\nDefault: 1"`
	// WorkerRSS defines how many parallel rss jobs to run - default: 1
	WorkerRSS int `toml:"worker_rss" displayname:"RSS Worker Threads" comment:"Number of parallel workers for RSS feed processing.\nHigher values speed up RSS feed checks and parsing.\nToo many workers may cause issues with feed servers or rate limits.\nIncrease only if you have many RSS feeds and fast internet.\nRecommended range: 1-3\nDefault: 1"`
	// WorkerIndexer defines how many indexers to query in parallel for each scan job - default: 1
	WorkerIndexer int `toml:"worker_indexer" displayname:"Parallel Indexer Workers" comment:"Number of indexers to query simultaneously during searches.\nHigher values speed up searches by querying multiple indexers at once.\nToo many may trigger rate limits or overwhelm your connection.\nBalance between search speed and indexer API limits.\nRecommended range: 1-5 depending on indexer count and limits\nDefault: 1"`
	// OmdbAPIKey is the API key for OMDB - get one at https://www.omdbapi.com/apikey.aspx
	OmdbAPIKey string `toml:"omdb_apikey" displayname:"OMDb API Key" comment:"API key for OMDb (Open Movie Database) service.\nRequired for enhanced movie metadata, ratings, and poster information.\nGet a free API key at: https://www.omdbapi.com/apikey.aspx\nLeave empty if you don't want OMDb integration.\nExample: 'a1b2c3d4'"`
	// UseMediaCache defines whether to cache movies and series in RAM for better performance - default: false
	UseMediaCache bool `toml:"use_media_cache" displayname:"Cache Media In RAM" comment:"Cache movie and TV series information in RAM for faster access.\nWhen enabled, media metadata is kept in memory to speed up searches and UI.\nUses more RAM but significantly improves performance for large libraries.\nRecommended for libraries with 1000+ items and sufficient RAM.\nDefault: false"`
	// UseFileCache defines whether to cache all files in RAM - default: false
	UseFileCache bool `toml:"use_file_cache" displayname:"Cache Files In RAM" comment:"Cache complete file listings in RAM for faster file operations.\nWhen enabled, all file paths and metadata are kept in memory.\nDramatically speeds up file scans but uses significant RAM.\nRecommended only for large libraries with fast systems and ample RAM.\nDefault: false"`
	// UseHistoryCache defines whether to cache downloaded entry history in RAM - default: false
	UseHistoryCache bool `toml:"use_history_cache" displayname:"Cache History In RAM" comment:"Cache download history in RAM to prevent duplicate downloads.\nWhen enabled, download history is kept in memory for faster duplicate checking.\nImproves performance when processing many search results.\nUses moderate RAM but significantly speeds up duplicate detection.\nDefault: false"`
	// CacheDuration defines hours after which cached data will be refreshed - default: 12
	CacheDuration  int `toml:"cache_duration" displayname:"Cache Duration Hours" comment:"Number of hours before cached data expires and gets refreshed.\nAfter this time, cached data is considered stale and will be reloaded.\nLower values keep data fresher but increase database load.\nHigher values reduce load but may show outdated information.\nRecommended range: 6-24 hours\nDefault: 12"`
	CacheDuration2 int `toml:"-"`
	// CacheAutoExtend defines whether cache expiration will be reset on access - default: false
	CacheAutoExtend bool `toml:"cache_auto_extend" displayname:"Extend Cache On Access" comment:"Reset cache expiration timer when data is accessed.\nWhen true, frequently accessed data stays cached longer.\nPrevents cache expiration for actively used data.\nWhen false, all cached data expires after the set duration regardless of usage.\nDefault: false"`
	// SearcherSize defines initial size of found entries slice - default: 5000
	SearcherSize int `toml:"searcher_size" displayname:"Search Result Buffer Size" comment:"Initial memory allocation size for search result storage.\nHigher values reduce memory reallocations during large searches.\nCalculate as: (number of indexers) × (max entries per search) × (alternate titles).\nToo low causes frequent reallocations, too high wastes memory.\nRecommended range: 1000-10000\nDefault: 5000"`
	// MovieMetaSourceImdb defines whether to scan IMDB for movie metadata - default: false
	MovieMetaSourceImdb bool `toml:"movie_meta_source_imdb" displayname:"Movie IMDB Metadata" comment:"Enable IMDB as a metadata source for movies.\nWhen true, movie information like ratings, cast, plot, etc. will be fetched from IMDB.\nRequires local IMDB database setup (see imdbindexer section).\nProvides comprehensive movie data but requires significant setup.\nDefault: false"`

	// MovieMetaSourceTmdb defines whether to scan TMDb for movie metadata - default: false
	MovieMetaSourceTmdb bool `toml:"movie_meta_source_tmdb" displayname:"Movie TMDb Metadata" comment:"Enable The Movie Database (TMDb) as a metadata source for movies.\nWhen true, movie information will be fetched from TMDb API.\nRequires themoviedb_apikey to be configured.\nProvides high-quality movie metadata with posters and backdrops.\nDefault: false"`
	// MovieMetaSourceOmdb defines whether to scan OMDB for movie metadata - default: false
	MovieMetaSourceOmdb bool `toml:"movie_meta_source_omdb" displayname:"Movie OMDb Metadata" comment:"Enable Open Movie Database (OMDb) as a metadata source for movies.\nWhen true, movie information will be fetched from OMDb API.\nRequires omdb_apikey to be configured.\nProvides movie ratings, plot summaries, and basic metadata.\nDefault: false"`
	// MovieMetaSourceTrakt defines whether to scan Trakt for movie metadata - default: false
	MovieMetaSourceTrakt bool `toml:"movie_meta_source_trakt" displayname:"Movie Trakt Metadata" comment:"Enable Trakt as a metadata source for movies.\nWhen true, movie information will be fetched from Trakt API.\nRequires Trakt authentication (client ID/secret) to be configured.\nProvides user ratings, watch statistics, and social features.\nDefault: false"`
	// MovieAlternateTitleMetaSourceImdb defines whether to scan IMDB for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceImdb bool `toml:"movie_alternate_title_meta_source_imdb" displayname:"Movie IMDB Alternate Titles" comment:"Fetch alternate movie titles from IMDB to improve search results.\nWhen true, foreign language titles and alternate names are retrieved from IMDB.\nHelps find releases with different naming conventions or translations.\nRequires movie_meta_source_imdb to be enabled.\nDefault: false"`
	// MovieAlternateTitleMetaSourceTmdb defines whether to scan TMDb for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceTmdb bool `toml:"movie_alternate_title_meta_source_tmdb" displayname:"Movie TMDb Alternate Titles" comment:"Fetch alternate movie titles from TMDb to improve search results.\nWhen true, international titles and alternate names are retrieved from TMDb.\nImproves matching of releases with regional or translated titles.\nRequires movie_meta_source_tmdb to be enabled.\nDefault: false"`
	// MovieAlternateTitleMetaSourceOmdb defines whether to scan OMDB for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceOmdb bool `toml:"movie_alternate_title_meta_source_omdb" displayname:"Movie OMDb Alternate Titles" comment:"Fetch alternate movie titles from OMDb to improve search results.\nWhen true, alternative titles are retrieved from OMDb API.\nProvides additional title variations for better release matching.\nRequires movie_meta_source_omdb to be enabled.\nDefault: false"`
	// MovieAlternateTitleMetaSourceTrakt defines whether to scan Trakt for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceTrakt bool `toml:"movie_alternate_title_meta_source_trakt" displayname:"Movie Trakt Alternate Titles" comment:"Fetch alternate movie titles from Trakt to improve search results.\nWhen true, alternative titles and aliases are retrieved from Trakt API.\nProvides community-contributed title variations for better matching.\nRequires movie_meta_source_trakt to be enabled.\nDefault: false"`
	// SerieAlternateTitleMetaSourceImdb defines whether to scan IMDB for alternate series titles - default: false
	SerieAlternateTitleMetaSourceImdb bool `toml:"serie_alternate_title_meta_source_imdb" displayname:"Series IMDB Alternate Titles" comment:"Fetch alternate TV series titles from IMDB to improve search results.\nWhen true, foreign language titles and alternate names are retrieved from IMDB.\nHelps find releases with different naming conventions or translations.\nRequires IMDB database setup and appropriate series metadata source.\nDefault: false"`
	// SerieAlternateTitleMetaSourceTrakt defines whether to scan Trakt for alternate series titles - default: false
	SerieAlternateTitleMetaSourceTrakt bool `toml:"serie_alternate_title_meta_source_trakt" displayname:"Series Trakt Alternate Titles" comment:"Fetch alternate TV series titles from Trakt to improve search results.\nWhen true, alternative titles and aliases are retrieved from Trakt API.\nProvides community-contributed title variations for better matching.\nRequires serie_meta_source_trakt to be enabled.\nDefault: false"`
	// MovieMetaSourcePriority defines priority order to scan metadata providers for movies - overrides individual settings
	MovieMetaSourcePriority []string `toml:"movie_meta_source_priority" displayname:"Movie Metadata Source Priority" comment:"Priority order for movie metadata providers.\nWhen specified, this overrides individual movie_meta_source_* settings.\nList providers in order of preference: first is tried first.\nAvailable options: 'imdb', 'tmdb', 'omdb', 'trakt'\nExample: ['tmdb', 'imdb', 'omdb']\nLeave empty to use individual settings."                 multiline:"true"`
	// MovieRSSMetaSourcePriority defines priority order to scan metadata providers for movie RSS - overrides individual settings
	MovieRSSMetaSourcePriority []string `toml:"movie_rss_meta_source_priority" displayname:"Movie RSS Metadata Priority" comment:"Priority order for movie metadata when processing RSS feeds.\nWhen specified, this overrides individual movie_meta_source_* settings for RSS imports.\nList providers in order of preference for RSS-discovered movies.\nAvailable options: 'imdb', 'tmdb', 'omdb', 'trakt'\nExample: ['tmdb', 'imdb']\nLeave empty to use individual settings." multiline:"true"`

	// MovieParseMetaSourcePriority defines priority order to scan metadata providers for movie file parsing - overrides individual settings
	MovieParseMetaSourcePriority []string `toml:"movie_parse_meta_source_priority" displayname:"Movie Parse Metadata Priority" multiline:"true" comment:"Priority order for movie metadata when parsing files.\nWhen specified, this overrides individual movie_meta_source_* settings for file parsing.\nList providers in order of preference for identifying movies from filenames.\nAvailable options: 'imdb', 'tmdb', 'omdb', 'trakt'\nExample: ['imdb', 'tmdb']\nLeave empty to use individual settings."`
	// SerieMetaSourceTmdb defines whether to scan TMDb for series metadata - default: false
	SerieMetaSourceTmdb bool `toml:"serie_meta_source_tmdb" displayname:"Series TMDb Metadata" comment:"Enable The Movie Database (TMDb) as a metadata source for TV series.\nWhen true, series information will be fetched from TMDb API.\nRequires themoviedb_apikey to be configured.\nProvides high-quality series metadata with posters and episode information.\nDefault: false"`
	// SerieMetaSourceTrakt defines whether to scan Trakt for series metadata - default: false
	SerieMetaSourceTrakt bool `toml:"serie_meta_source_trakt" displayname:"Series Trakt Metadata" comment:"Enable Trakt as a metadata source for TV series.\nWhen true, series information will be fetched from Trakt API.\nRequires Trakt authentication (client ID/secret) to be configured.\nProvides user ratings, watch statistics, and social features for series.\nDefault: false"`
	// MoveBufferSizeKB defines buffer size in KB to use if file buffer copy enabled - default: 1024
	MoveBufferSizeKB int `toml:"move_buffer_size_kb" displayname:"File Buffer Size KB" comment:"File buffer size in kilobytes for file operations.\nLarger buffers can improve file copy/move performance but use more RAM.\nUseful when moving large files or working with network storage.\nRecommended range: 64-4096 KB depending on system and storage type.\nDefault: 1024"`
	// WebPort defines port for web interface and API - default: 9090
	WebPort string `toml:"webport" displayname:"Web Interface Port" comment:"TCP port number for the web interface and API server.\nThe application will listen on this port for HTTP connections.\nMake sure the port is not used by other applications.\nCommon alternatives: 8080, 8090, 9091\nExample: '9090' or '8080'\nDefault: '9090'"`
	// WebAPIKey defines API key for API calls - default: mysecure
	WebAPIKey string `toml:"webapikey" displayname:"Web API Key" comment:"API key required for authentication with the REST API.\nUsed by third-party applications and scripts to access the API.\nAlso serves as the default admin password for the web interface.\nUse a strong, unique key for security.\nExample: 'mySecureApiKey123'\nDefault: 'mysecure' (change this!)"`
	// WebPortalEnabled enables/disables web portal - default: false
	WebPortalEnabled bool `toml:"web_portal_enabled" displayname:"Enable Web Interface" comment:"Enable the web-based administration interface.\nWhen true, you can access the application through a web browser.\nProvides a user-friendly interface for configuration and monitoring.\nWhen false, only API access is available.\nDefault: false"`
	// TheMovieDBApiKey defines API key for TMDb - get from: https://www.themoviedb.org/settings/api
	TheMovieDBApiKey string `toml:"themoviedb_apikey" displayname:"TMDb API Key" comment:"API key for The Movie Database (TMDb) service.\nRequired for TMDb metadata integration (posters, plot, cast, etc.).\nGet a free API key by registering at: https://www.themoviedb.org/settings/api\nLeave empty if you don't want TMDb integration.\nExample: 'a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6'"`
	// TraktClientID defines client ID for Trakt - get from: https://trakt.tv/oauth/applications/new
	TraktClientID string `toml:"trakt_client_id" displayname:"Trakt Client ID" comment:"Client ID for Trakt API integration.\nRequired for Trakt features like list syncing and user ratings.\nCreate an application at: https://trakt.tv/oauth/applications/new\nUse 'http://localhost:9090' as the redirect URI.\nLeave empty if you don't want Trakt integration.\nExample: 'a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0'"`
	// TraktClientSecret defines client secret for Trakt application
	TraktClientSecret string `toml:"trakt_client_secret" displayname:"Trakt Client Secret" comment:"Client secret for your Trakt application.\nThis is the secret key paired with your Trakt client ID.\nFound in your application settings on Trakt after creating the application.\nKeep this secret and do not share it publicly.\nRequired if using Trakt integration.\nExample: 'z9y8x7w6v5u4t3s2r1q0p9o8n7m6l5k4j3i2h1g0'"`

	TraktRedirectUrl string `toml:"trakt_redirect_url" displayname:"Trakt Client Secret" comment:"Redirect Url for your Trakt application.\nThis is the Redirect Url paired with your Trakt client ID.\nFound in your application settings on Trakt after creating the application.\nRequired if using Trakt integration.\nExample: 'http://localhost:9090/'"`
	// SchedulerDisabled enables/disables scheduler - default false
	SchedulerDisabled bool `toml:"scheduler_disabled" displayname:"Disable All Schedulers" comment:"Disable all automated scheduled tasks.\nWhen true, automatic searches, scans, and maintenance tasks are disabled.\nOnly manual operations will be performed.\nUseful for troubleshooting or when running tasks manually.\nDefault: false (scheduler enabled)"`

	// DisableParserStringMatch defines whether to disable string matching in parsers - default: false
	DisableParserStringMatch bool `toml:"disable_parser_string_match" displayname:"Disable String Matching Parser" comment:"Disable string-based parsing and use only regex for field matching.\nWhen true, only regex patterns are used to identify release information.\nThis may decrease performance but can increase parsing accuracy.\nUseful when string matching produces too many false positives.\nWhen false, both string matching and regex are used (faster).\nDefault: false (use both string matching and regex)"`
	// UseCronInsteadOfInterval defines whether to convert intervals to cron strings - default: false
	UseCronInsteadOfInterval bool `toml:"use_cron_instead_of_interval" displayname:"Use Cron For Intervals" comment:"Convert scheduler intervals to cron expressions for better performance.\nWhen true, simple intervals are internally converted to cron formats.\nThis improves scheduler performance and provides more precise timing.\nWhen false, intervals are used as-is (simpler but less efficient).\nRecommended for systems with many scheduled tasks.\nDefault: false"`
	// UseFileBufferCopy defines whether to use buffered file copy - default: false
	UseFileBufferCopy bool `toml:"use_file_buffer_copy" displayname:"Use Buffered File Copy" comment:"Enable buffered file copying for potentially improved performance.\nWhen true, files are copied using a buffer of configured size.\nMay improve performance on some systems but not recommended generally.\nCan cause issues with network storage or slow drives.\nBuffer size is controlled by move_buffer_size_kb setting.\nDefault: false (not recommended)"`
	// DisableSwagger defines whether to disable Swagger API docs - default: false
	DisableSwagger bool `toml:"disable_swagger" displayname:"Disable API Documentation" comment:"Disable automatic Swagger API documentation generation.\nWhen true, the Swagger UI and API docs are not generated or served.\nThis can slightly improve startup time and reduce memory usage.\nUseful in production environments where API docs are not needed.\nWhen false, API documentation is available at /swagger endpoint.\nDefault: false (Swagger enabled)"`
	// TraktLimiterSeconds defines seconds limit for Trakt API calls - default: 1
	TraktLimiterSeconds uint8 `toml:"trakt_limiter_seconds" displayname:"Trakt Rate Limit Seconds" comment:"Time window in seconds for Trakt API rate limiting.\nDefines the time period over which API call limits are applied.\nWorks together with trakt_limiter_calls to prevent API rate limit violations.\nTrakt's API limits change, so adjust based on current Trakt documentation.\nLower values provide more granular rate limiting control.\nDefault: 1 (one second time window)"`
	// TraktLimiterCalls defines calls limit for Trakt API in defined seconds - default: 1
	TraktLimiterCalls int `toml:"trakt_limiter_calls" displayname:"Trakt Calls Per Window" comment:"Maximum number of API calls allowed to Trakt within the defined time window.\nWorks with trakt_limiter_seconds to enforce rate limiting.\nIf you exceed this limit, requests will be delayed to comply with limits.\nCheck Trakt's current API documentation for their actual rate limits.\nConservative values prevent API key suspension due to rate limit violations.\nDefault: 1 (one call per time window)"`
	// TvdbLimiterSeconds defines seconds limit for TVDB API calls - default: 1
	TvdbLimiterSeconds uint8 `toml:"tvdb_limiter_seconds" displayname:"TVDB Rate Limit Seconds" comment:"Time window in seconds for TVDB API rate limiting.\nDefines the time period over which TVDB API call limits are applied.\nWorks together with tvdb_limiter_calls to prevent API rate limit violations.\nTVDB's API has specific rate limits that change over time.\nAdjust based on current TVDB API documentation and your subscription level.\nDefault: 1 (one second time window)"`
	// TvdbLimiterCalls defines calls limit for TVDB API in defined seconds - default: 1
	TvdbLimiterCalls int `toml:"tvdb_limiter_calls" displayname:"TVDB Calls Per Window" comment:"Maximum number of API calls allowed to TVDB within the defined time window.\nWorks with tvdb_limiter_seconds to enforce rate limiting.\nTVDB has different rate limits for free vs paid subscriptions.\nExceeding limits may result in temporary API access suspension.\nCheck your TVDB subscription level and current API documentation.\nDefault: 1 (one call per time window)"`
	// TmdbLimiterSeconds defines seconds limit for TMDb API calls - default: 1
	TmdbLimiterSeconds uint8 `toml:"tmdb_limiter_seconds" displayname:"TMDb Rate Limit Seconds" comment:"Time window in seconds for TMDb API rate limiting.\nDefines the time period over which TMDb API call limits are applied.\nWorks together with tmdb_limiter_calls to prevent API rate limit violations.\nTMDb has generous rate limits but they can change based on usage patterns.\nAdjust based on current TMDb API documentation and your usage needs.\nDefault: 1 (one second time window)"`
	// TmdbLimiterCalls defines calls limit for TMDb API in defined seconds - default: 1
	TmdbLimiterCalls int `toml:"tmdb_limiter_calls" displayname:"TMDb Calls Per Window" comment:"Maximum number of API calls allowed to TMDb within the defined time window.\nWorks with tmdb_limiter_seconds to enforce rate limiting.\nTMDb typically allows 40 requests per 10 seconds for free accounts.\nExceeding limits may result in temporary API throttling or blocks.\nAdjust based on your TMDb API key limits and usage requirements.\nDefault: 1 (one call per time window)"`
	// OmdbLimiterSeconds defines seconds limit for OMDb API calls - default: 1
	OmdbLimiterSeconds uint8 `toml:"omdb_limiter_seconds" displayname:"OMDb Rate Limit Seconds" comment:"Time window in seconds for OMDb API rate limiting.\nDefines the time period over which OMDb API call limits are applied.\nWorks together with omdb_limiter_calls to prevent API rate limit violations.\nOMDb has strict rate limits that vary by subscription tier.\nFree tier typically allows 1000 calls per day with rate restrictions.\nDefault: 1 (one second time window)"`
	// OmdbLimiterCalls defines calls limit for OMDb API in defined seconds - default: 1
	OmdbLimiterCalls int `toml:"omdb_limiter_calls" displayname:"OMDb Calls Per Window" comment:"Maximum number of API calls allowed to OMDb within the defined time window.\nWorks with omdb_limiter_seconds to enforce rate limiting.\nOMDb free tier allows 1000 calls/day, paid tiers have higher limits.\nExceeding daily limits results in API key suspension until reset.\nConservative rate limiting prevents accidental limit violations.\nDefault: 1 (one call per time window)"`

	// TheMovieDBDisableTLSVerify disables TLS certificate verification for TheMovieDB API requests
	// Setting this to true may increase performance but reduces security
	TheMovieDBDisableTLSVerify bool `toml:"tmdb_disable_tls_verify" displayname:"TMDb Disable SSL Verification" comment:"Disable SSL/TLS certificate verification for TMDb API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nLeaves connections vulnerable to man-in-the-middle attacks.\nDefault: false (secure connections with certificate verification)"`

	// TraktDisableTLSVerify disables TLS certificate verification for Trakt API requests
	// Setting this to true may increase performance but reduces security
	TraktDisableTLSVerify bool `toml:"trakt_disable_tls_verify" displayname:"Trakt Disable SSL Verification" comment:"Disable SSL/TLS certificate verification for Trakt API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nLeaves OAuth token exchange vulnerable to interception.\nDefault: false (secure connections with certificate verification)"`

	// OmdbDisableTLSVerify disables TLS certificate verification for OMDb API requests
	// Setting this to true may increase performance but reduces security
	OmdbDisableTLSVerify bool `toml:"omdb_disable_tls_verify" displayname:"OMDb Disable SSL Verification" comment:"Disable SSL/TLS certificate verification for OMDb API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nAPI keys could be intercepted through compromised connections.\nDefault: false (secure connections with certificate verification)"`

	// TvdbDisableTLSVerify disables TLS certificate verification for TVDB API requests
	// Setting this to true may increase performance but reduces security
	TvdbDisableTLSVerify bool `toml:"tvdb_disable_tls_verify" displayname:"TVDB Disable SSL Verification" comment:"Disable SSL/TLS certificate verification for TVDB API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nAuthentication tokens could be compromised through insecure connections.\nDefault: false (secure connections with certificate verification)"`

	// FfprobePath specifies the path to the ffprobe executable
	// Used for media analysis
	FfprobePath string `toml:"ffprobe_path" displayname:"FFprobe Executable Path" comment:"Absolute path to the ffprobe executable for media file analysis.\nSpecifying the full path improves performance by avoiding PATH searches.\nFfprobe is part of FFmpeg and is used to extract media file information.\nRequired for video duration, codec, and quality detection.\nDownload FFmpeg from: https://ffmpeg.org/download.html\nExample: '/usr/bin/ffprobe' or 'C:\\\\FFmpeg\\\\bin\\\\ffprobe.exe'\nDefault: './ffprobe' (current directory)"`

	// MediainfoPath specifies the path to the mediainfo executable
	// Used as an alternative to ffprobe for media analysis
	MediainfoPath string `toml:"mediainfo_path" displayname:"MediaInfo Executable Path" comment:"Absolute path to the MediaInfo executable for media file analysis.\nSpecifying the full path improves performance by avoiding PATH searches.\nMediaInfo is an alternative to ffprobe for extracting media information.\nSome users prefer it for certain file formats or analysis accuracy.\nDownload from: https://mediaarea.net/en/MediaInfo/Download\nExample: '/usr/bin/mediainfo' or 'C:\\\\MediaInfo\\\\mediainfo.exe'\nDefault: './mediainfo' (current directory)"`

	// UseMediainfo specifies whether to use mediainfo instead of ffprobe for media analysis
	UseMediainfo bool `toml:"use_mediainfo" displayname:"Use MediaInfo Over FFprobe" comment:"Use MediaInfo instead of ffprobe as the primary media analysis tool.\nWhen true, MediaInfo is used for all media file analysis tasks.\nWhen false, ffprobe (FFmpeg) is used for media analysis.\nMediaInfo may provide different information or work better with certain formats.\nRequires mediainfo_path to be properly configured.\nDefault: false (use ffprobe)"`

	// UseMediaFallback specifies whether to use mediainfo as a fallback if ffprobe fails
	UseMediaFallback bool `toml:"use_media_fallback" displayname:"Use MediaInfo As Fallback" comment:"Use MediaInfo as a backup when ffprobe fails to analyze media files.\nWhen true, if ffprobe fails, MediaInfo will be tried automatically.\nProvides redundancy for media analysis in case one tool fails.\nUseful when dealing with problematic or unusual file formats.\nRequires both ffprobe_path and mediainfo_path to be configured.\nDefault: false (no fallback)"`

	// FailedIndexerBlockTime specifies how long in minutes an indexer should be blocked after failures
	FailedIndexerBlockTime int `toml:"failed_indexer_block_time" displayname:"Failed Indexer Block Minutes" comment:"Duration in minutes to temporarily block an indexer after consecutive failures.\nWhen an indexer fails repeatedly, it's blocked for this time period.\nPrevents wasting resources on consistently failing indexers.\nAfter the block period, the indexer is retried automatically.\nLonger times reduce load on failing indexers, shorter times retry sooner.\nTypical range: 1-60 minutes\nDefault: 5"`

	// MaxDatabaseBackups defines the maximum number of database backups to retain
	MaxDatabaseBackups int `toml:"max_database_backups" displayname:"Maximum Database Backups" comment:"Maximum number of database backup files to keep before deleting old ones.\nAutomatic backups are created during maintenance and configuration changes.\nOlder backups beyond this limit are automatically deleted.\nSet to 0 to completely disable database backups (not recommended).\nHigher values preserve more backup history but use more disk space.\nRecommended range: 3-10 backups\nDefault: 0 (backups disabled)"`

	// DatabaseBackupStopTasks specifies whether to stop background tasks during database backups
	DatabaseBackupStopTasks bool `toml:"database_backup_stop_tasks" displayname:"Stop Tasks During Backup" comment:"Pause all background tasks and schedulers during database backup operations.\nWhen true, searches, scans, and scheduled tasks are suspended during backups.\nThis ensures database consistency but temporarily halts all operations.\nWhen false, tasks continue running during backups (may cause inconsistencies).\nRecommended: true for data integrity, false for minimal downtime.\nDefault: false"`

	// DisableVariableCleanup specifies whether to disable cleanup of variables after use
	// This may reduce RAM usage but variables will persist
	// Default is false
	DisableVariableCleanup bool `toml:"disable_variable_cleanup" displayname:"Disable Variable Cleanup" comment:"Disable automatic cleanup of variables after use to potentially reduce RAM usage.\nWhen true, variables remain in memory longer, possibly reducing allocations.\nMay actually increase memory usage if variables accumulate over time.\nWhen false, variables are cleaned up promptly after use (recommended).\nThis is an experimental optimization that may or may not help performance.\nDefault: false (enable cleanup)"`
	// OmdbTimeoutSeconds defines the HTTP timeout in seconds for OMDb API calls
	// Default is 10 seconds
	OmdbTimeoutSeconds uint16 `toml:"omdb_timeout_seconds" displayname:"OMDb Request Timeout Seconds" comment:"HTTP request timeout in seconds for OMDb API calls.\nMaximum time to wait for OMDb API responses before timing out.\nOMDb responses are typically fast but may vary based on server load.\nLonger timeouts accommodate network latency and server delays.\nShorter timeouts provide faster error detection but may cause false failures.\nTypical range: 5-30 seconds\nDefault: 10"`
	// TmdbTimeoutSeconds defines the HTTP timeout in seconds for TMDb API calls
	// Default is 10 seconds
	TmdbTimeoutSeconds uint16 `toml:"tmdb_timeout_seconds" displayname:"TMDb Request Timeout Seconds" comment:"HTTP request timeout in seconds for TMDb API calls.\nMaximum time to wait for TMDb API responses before timing out.\nTMDb generally has fast response times but may vary during peak usage.\nImage and artwork requests may take longer than metadata requests.\nBalance between patience for slow responses and quick error detection.\nTypical range: 5-30 seconds\nDefault: 10"`
	// TvdbTimeoutSeconds defines the HTTP timeout in seconds for TVDB API calls
	// Default is 10 seconds
	TvdbTimeoutSeconds uint16 `toml:"tvdb_timeout_seconds" displayname:"TVDB Request Timeout Seconds" comment:"HTTP request timeout in seconds for TVDB API calls.\nMaximum time to wait for TVDB API responses before timing out.\nLonger timeouts are more forgiving of network issues but slower to fail.\nShorter timeouts fail faster but may miss responses on slow connections.\nBalance between responsiveness and reliability based on your connection.\nTypical range: 5-30 seconds\nDefault: 10"`
	// TraktTimeoutSeconds defines the HTTP timeout in seconds for Trakt API calls
	// Default is 10 seconds
	TraktTimeoutSeconds uint16 `toml:"trakt_timeout_seconds" displayname:"Trakt Request Timeout Seconds" comment:"HTTP request timeout in seconds for Trakt API calls.\nMaximum time to wait for Trakt API responses before timing out.\nTrakt OAuth operations may take longer than simple API calls.\nLonger timeouts accommodate OAuth flows and complex requests.\nShorter timeouts provide faster failure detection.\nTypical range: 5-30 seconds\nDefault: 10"`

	// Jobs To Run
	Jobs map[string]func(uint32) error `toml:"-" json:"-"`
	// UseGoDir                           bool     `toml:"use_godir"`
	// ConcurrentScheduler                int      `toml:"concurrent_scheduler"`
	// EnableFileWatcher specifies whether the file watcher functionality is enabled
	// When set to true, the application will monitor specified directories for file changes
	// Default is false
	EnableFileWatcher bool `toml:"enable_file_watcher" displayname:"Enable Configuration File Watcher" comment:"Enable automatic monitoring of configuration file changes.\nWhen true, the application will watch the config file and reload changes automatically.\nAllows configuration updates without restarting the application.\nUseful for live configuration adjustments during operation.\nMay consume additional system resources for file monitoring.\nDefault: false (manual restart required for config changes)"`
}

// ImdbConfig defines the configuration for the IMDb indexer.
type ImdbConfig struct {
	// Indexedtypes is an array of strings specifying the types of IMDb media to import
	// Valid values are 'movie', 'tvMovie', 'tvmovie', 'tvSeries', 'tvseries', 'video'
	// Default is empty array which imports nothing
	Indexedtypes []string `toml:"indexed_types" displayname:"Media Types To Index" multiline:"true" comment:"Specify which types of media to import from the IMDB database.\nThis controls what content types are downloaded and indexed locally.\nValid options:\n- 'movie': Feature films and theatrical releases\n- 'tvMovie': Made-for-TV movies and TV films\n- 'tvSeries': TV shows, series, and episodic content\n- 'video': Direct-to-video releases, web series, shorts\nExample: ['movie', 'tvSeries'] to import only movies and TV shows\nLeave empty to disable IMDB indexing entirely\nWarning: Importing all types requires significant disk space (10+ GB)"`

	// Indexedlanguages is an array of strings specifying the languages to use for titles
	// Examples: "DE", "UK", "US"
	// Include '' or '\N' for global titles
	// Default is empty array which imports all languages
	Indexedlanguages []string `toml:"indexed_languages" displayname:"Languages To Index" multiline:"true" comment:"Filter IMDB titles by language/region to reduce database size.\nSpecify which languages and regions you want to include in your local IMDB index.\nUse standard region codes for language variants:\n- 'US': English (United States)\n- 'GB' or 'UK': English (United Kingdom)\n- 'DE': German titles\n- 'FR': French titles\n- 'ES': Spanish titles\n- '': Include titles without specific language designation (original/international)\nExample: ['US', 'GB', ''] for English titles plus international\nLeave empty to import all languages (requires more storage)"`

	// Indexfull is a boolean specifying whether to index all available IMDb data
	// or only the bare minimum
	// Default is false
	Indexfull bool `toml:"index_full" displayname:"Import Complete Dataset" comment:"Import complete IMDB dataset versus minimal data for basic functionality.\nWhen true, imports comprehensive data including cast, crew, ratings, plots, etc.\nWhen false, imports only essential data needed for media matching and identification.\nFull indexing provides richer metadata but requires significantly more:\n- Disk space: 50+ GB vs 2-5 GB for minimal\n- RAM usage: 4+ GB vs 1-2 GB during import\n- Import time: Hours vs minutes\nRecommended: false unless you need comprehensive IMDB metadata\nDefault: false (minimal data only)"`

	// ImdbIDSize is an integer specifying the number of expected entries in the IMDb database
	// Default is 12000000
	ImdbIDSize int `toml:"imdbid_size" displayname:"Expected Database Entry Count" comment:"Estimated total number of entries in the IMDB database for memory pre-allocation.\nThis helps optimize memory allocation during the import process.\nShould be set higher than the actual expected number of entries.\nIMDB contains approximately:\n- 10+ million titles (all types)\n- 12+ million entries when including episodes\nIncreasing this value uses more memory but prevents reallocations.\nDecreasing saves memory but may cause performance issues if too low.\nRecommended range: 10,000,000 - 15,000,000\nDefault: 12,000,000"`

	// LoopSize is an integer specifying the number of entries to keep in RAM for cached queries
	// Default is 400000
	LoopSize int `toml:"loop_size" displayname:"RAM Cache Entry Count" comment:"Number of IMDB entries to keep in RAM cache for fast query performance.\nHigher values improve lookup speed but use more memory.\nThis cache stores frequently accessed IMDB data in memory.\nMemory usage scales roughly: (loop_size × 1KB) per entry.\nRecommended values based on system RAM:\n- 2GB RAM: 200,000 - 400,000 entries\n- 4GB RAM: 400,000 - 800,000 entries\n- 8GB+ RAM: 800,000+ entries\nBalance between query performance and available memory\nDefault: 400,000"`

	// UseMemory is a boolean specifying whether to store the IMDb DB in RAM during generation
	// At least 2GB RAM required. Highly recommended.
	// Default is false
	UseMemory bool `toml:"use_memory" displayname:"Store Database In RAM" comment:"Store the entire IMDB database in RAM during import for dramatically faster processing.\nWhen true, the complete dataset is loaded into memory during import.\nThis provides significant performance improvements but requires substantial RAM.\nMemory requirements:\n- Minimal import: 0,5-2 GB RAM\n- Full import: 1-4 GB RAM\nBenefits: 10-50x faster import times, reduced disk I/O\nDrawbacks: High memory usage, system may become unresponsive if insufficient RAM\nHighly recommended if you have sufficient available memory\nDefault: false (use disk-based processing)"`

	// UseCache is a boolean specifying whether to use caching for SQL queries
	// Might reduce execution time
	// Default is false
	UseCache bool `toml:"use_cache" displayname:"Enable SQL Query Caching" comment:"Enable SQL query result caching to improve IMDB database query performance.\nWhen true, frequently executed queries are cached in memory.\nReduces database load and improves response times for repeated queries.\nMost beneficial when the same IMDB searches are performed repeatedly.\nCache memory usage grows over time with unique queries.\nRecommended for systems with ample RAM and heavy IMDB usage.\nDefault: false (no query caching)"`
}

// MediaConfig defines the configuration for media types like series and movies.
type MediaConfig struct {
	// Series defines the configuration for all series media types
	Series []MediaTypeConfig `toml:"series" displayname:"TV Series Media Groups" comment:"Configuration definitions for all your TV series and episodic content.\nEach entry defines a separate media group with its own settings for:\n- Quality profiles and search preferences\n- Storage paths and file organization\n- Indexers and download clients to use\n- Notification settings for new episodes\n- List integrations and metadata sources\nYou can create multiple series groups for different types:\n- Regular TV shows, anime, documentaries, etc.\n- Different quality requirements (4K vs HD)\n- Separate storage locations or indexers\nExample: Create 'tv-hd' and 'tv-4k' groups with different quality profiles"`
	// Movies defines the configuration for all movies media types
	Movies []MediaTypeConfig `toml:"movies" displayname:"Movie Media Groups" comment:"Configuration definitions for all your movie collections and film content.\nEach entry defines a separate media group with its own settings for:\n- Quality profiles and search preferences\n- Storage paths and file organization\n- Indexers and download clients to use\n- Notification settings for new releases\n- List integrations and metadata sources\nYou can create multiple movie groups for different purposes:\n- Different genres (action, documentaries, foreign films)\n- Quality tiers (4K, HD, standard definition)\n- Separate storage locations or collection types\nExample: Create 'movies-4k' and 'movies-hd' groups with different storage paths"`
}

// MediaTypeConfig defines the configuration for a media type like movies or series.
type MediaTypeConfig struct {
	// Name is the name of the media group - keep it unique
	Name string `toml:"name" displayname:"Media Group Name" comment:"Unique identifier name for this media group configuration.\nThis name is used throughout the application to reference this specific media setup.\nMust be unique across all media groups (both series and movies).\nChoose descriptive names that indicate the purpose or characteristics:\n- Content type: 'anime', 'documentaries', 'foreign-films'\n- Quality tier: 'movies-4k', 'tv-hd', 'series-sd'\n- Storage location: 'nas-movies', 'local-tv'\nUsed in logs, web interface, and when configuring other sections.\nExample: 'movies-uhd' for a 4K movie collection"`

	// NamePrefix is not set in the TOML config
	NamePrefix string `toml:"-"`

	// Useseries is set automatically to true if the configuration is for a series
	Useseries bool `toml:"-"`

	// DefaultQuality is the default quality to assume if none was found - keep it low
	DefaultQuality string `toml:"default_quality" displayname:"Fallback Quality Level" comment:"Default quality level to assign when release quality cannot be determined.\nUsed as a fallback when parsing fails to detect the actual quality.\nShould be set to a conservative (lower) quality to avoid false upgrades.\nMust match one of the qualities defined in your quality profile.\nCommon conservative defaults:\n- For movies: 'HDTV' or 'DVD'\n- For TV series: 'HDTV' or 'WEBDL'\nAvoid using high-end qualities like 'BluRay' or '4K' as defaults.\nExample: 'HDTV' (safe, commonly available quality)"`

	// DefaultResolution is the default resolution to assume if none was found - keep it low
	DefaultResolution string `toml:"default_resolution" displayname:"Fallback Video Resolution" comment:"Default video resolution to assign when release resolution cannot be determined.\nUsed as a fallback when parsing fails to detect the actual resolution.\nShould be set to a conservative (lower) resolution to avoid false upgrades.\nMust match one of the resolutions defined in your quality profile.\nCommon conservative defaults:\n- For older content: '480p' or '576p'\n- For modern content: '720p'\nAvoid using high resolutions like '1080p' or '2160p' as defaults.\nExample: '720p' (safe, widely available resolution)"`

	// Naming is the naming scheme for files - see wiki for details
	Naming string `toml:"naming" displayname:"File Naming Template" comment:"File and folder naming template for organized media files.\nDefines how downloaded files will be renamed and organized in your library.\nUses template variables that get replaced with actual media information.\nCommon variables include title, year, quality, resolution, codec, etc.\nDifferent templates for movies vs TV series are typical.\nExample movie template: '{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}{{if eq .Source.Extended true}} extended{{end}}] ({{.Source.Imdb}})'\nExample series template: '{{.Dbserie.Seriename}}/Season {{.DbserieEpisode.Season}}/{{.Dbserie.Seriename}} - S{{printf \"%02s\" .DbserieEpisode.Season}}{{range .Episodes}}E{{printf \"%02d\" . }}{{end}} - {{.DbserieEpisode.Title}} [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}] ({{.Source.Tvdb}})'\nSee documentation for complete variable list and examples:\nhttps://github.com/Kellerman81/go_media_downloader/wiki/Groups"`

	// TemplateQuality is the name of the quality template to use
	TemplateQuality string `toml:"template_quality" displayname:"Quality Profile Reference" comment:"Name of the quality profile template to use for this media group.\nReferences a quality configuration defined in the quality section.\nThe quality profile controls:\n- Preferred video resolutions (720p, 1080p, 4K, etc.)\n- Desired source quality (BluRay, WEB-DL, HDTV, etc.)\n- Audio and video codec preferences\n- Search behavior and upgrade criteria\n- Indexer-specific settings\nMust exactly match the 'name' field of a quality configuration.\nExample: 'uhd-quality' to use a 4K-focused quality profile"`

	// CfgQuality is the parsed quality config (not set in TOML)
	CfgQuality *QualityConfig `toml:"-"`

	// TemplateScheduler is the name of the scheduler template to use
	TemplateScheduler string `toml:"template_scheduler" displayname:"Scheduler Template Reference" comment:"Name of the scheduler template to use for this media group's automated tasks.\nReferences a scheduler configuration defined in the scheduler section.\nThe scheduler controls:\n- How often to search for missing episodes/movies\n- When to perform quality upgrade scans\n- RSS feed check intervals\n- Maintenance task timing\nMust exactly match the 'name' field of a scheduler configuration.\nLeave empty to use default scheduler behavior.\nExample: 'aggressive-schedule' for frequent searches"`

	// CfgScheduler is the parsed scheduler config (not set in TOML)
	CfgScheduler *SchedulerConfig `toml:"-"`

	// MetadataLanguage is the default language for metadata
	MetadataLanguage string `toml:"metadata_language" displayname:"Primary Metadata Language" comment:"Primary language code for metadata retrieval and display.\nControls the language used when fetching information from TMDB, TVDB, etc.\nAffects plot summaries, descriptions, and other text metadata.\nUse standard ISO 639-1 two-letter language codes:\n- 'en': English\n- 'de': German\n- 'fr': French\n- 'es': Spanish\n- 'ja': Japanese\nMetadata sources may not have all languages available.\nExample: 'en' for English metadata"`

	// MetadataTitleLanguages are the languages to import titles in
	MetadataTitleLanguages []string `toml:"metadata_title_languages" displayname:"Alternate Title Languages" multiline:"true" comment:"List of language/region codes for importing alternate titles.\nCollects titles in multiple languages to improve release matching.\nUseful for finding releases with foreign or alternate names.\nUse ISO country/language codes:\n- 'en': English (generic)\n- 'us': English (United States)\n- 'gb' or 'uk': English (United Kingdom)\n- 'de': German titles\n- 'jp': Japanese titles\nMore languages = better release matching but increased processing time.\nExample: ['en', 'us', 'de'] for English and German titles"`

	// MetadataTitleLanguagesLen is the number of title languages (not set in TOML)
	MetadataTitleLanguagesLen int `toml:"-"`

	// Structure indicates whether to structure media after download
	Structure bool `toml:"structure" displayname:"Auto File Organization" comment:"Enable automatic file organization and renaming after download completion.\nWhen true, downloaded files are automatically:\n- Moved to proper library locations\n- Renamed according to the naming template\n- Organized into appropriate folder structures\n- Have metadata and artwork added\nWhen false, files remain in download location with original names.\nRequires proper path configuration in the data section.\nRecommended: true for organized media libraries\nDefault: false"`

	// SearchmissingIncremental is the number of entries to process in incremental missing scans
	SearchmissingIncremental uint16 `toml:"search_missing_incremental" displayname:"Missing Items Per Scan" comment:"Number of missing items to search for in each incremental scan cycle.\nIncremental scans process a limited number of items per run to avoid overwhelming indexers.\nLower values are gentler on indexers but take longer to process large backlogs.\nHigher values process backlogs faster but may trigger rate limits.\nBalance based on your indexer limits and how quickly you want missing content found.\nTypical range: 10-50 items per scan\nExample: 20 (process 20 missing items per scheduled scan)\nDefault: 20"`

	// SearchupgradeIncremental is the number of entries to process in incremental upgrade scans
	SearchupgradeIncremental uint16 `toml:"search_upgrade_incremental" displayname:"Upgrade Items Per Scan" comment:"Number of existing items to check for quality upgrades in each scan cycle.\nIncremental upgrade scans look for better quality versions of existing media.\nLower values reduce indexer load but slower upgrade discovery.\nHigher values find upgrades faster but consume more indexer API calls.\nUpgrade scans are typically less urgent than missing content searches.\nConsider setting lower than search_missing_incremental.\nTypical range: 5-30 items per scan\nExample: 15 (check 15 items for upgrades per scheduled scan)\nDefault: 20"`

	// Data contains the media data configs
	Data    []MediaDataConfig        `toml:"data" displayname:"Storage Path Configurations" comment:"Storage path configurations for this media group.\nDefines where media files will be stored and how they're organized.\nEach entry specifies:\n- Path template reference (from paths section)\n- Minimum and maximum file sizes\n- Upgrade behavior and file management rules\nMultiple data entries allow different storage tiers or locations.\nExample: Separate entries for different quality levels or storage devices.\nRequired: At least one data configuration must be defined."`
	DataMap map[int]*MediaDataConfig `toml:"-"`

	// DataLen is the number of data configs (not set in TOML)
	DataLen int `toml:"-"`

	// DataImport contains media data import configs
	DataImport    []MediaDataImportConfig        `toml:"data_import" displayname:"Import Path Configurations" comment:"Configuration for importing existing media files into the library.\nDefines how to scan and process media files already on your system.\nEach entry specifies:\n- Paths to scan for existing media\n- Import behavior and file handling rules\n- Whether to move, copy, or link existing files\nUseful for migrating from other media managers or adding existing collections.\nOptional: Only needed if importing existing media files."`
	DataImportMap map[int]*MediaDataImportConfig `toml:"-"`

	// Lists contains media lists configs
	Lists []MediaListsConfig `toml:"lists" displayname:"External List Integrations" comment:"External list integrations for automatic media discovery.\nConnects to various sources to automatically add new media to your wanted list.\nEach entry can reference:\n- List template configurations (from lists section)\n- IMDB lists, Trakt lists, RSS feeds\n- Custom lists and watchlists\nAllows automatic discovery of new releases based on your preferences.\nOptional: Only needed if using automatic list-based media discovery."`

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
	Notification []MediaNotificationConfig `toml:"notification" displayname:"Notification Configurations" comment:"Notification settings for this specific media group.\nDefines how and when you'll be alerted about media events.\nEach entry can reference:\n- Notification template configurations (from [notification] section)\n- Events to notify about (downloads, upgrades, failures)\n- Specific notification channels (Pushover, email, webhooks)\nAllows different notification preferences per media type.\nOptional: Only needed if you want notifications for this media group."`

	// Jobs To Run
	Jobs map[string]func(uint32) error `toml:"-" json:"-"`
}

// MediaDataConfig is a struct that defines configuration for media data.
type MediaDataConfig struct {
	// TemplatePath is the template to use for the path
	TemplatePath string `toml:"template_path" displayname:"Storage Path Template" comment:"Name of the path configuration template to use for media storage.\nReferences a path configuration defined in the paths section.\nThe path template controls:\n- Root storage directory for this media\n- File extensions and size limits\n- File organization and naming rules\n- Upgrade and cleanup behavior\nMust exactly match the 'name' field of a paths configuration.\nDifferent data entries can use different path templates for storage tiers.\nExample: 'movies-4k-storage' for a high-capacity 4K movie path"`
	// CfgPath is a pointer to PathsConfig
	CfgPath *PathsConfig `toml:"-"`
	// AddFound indicates if entries not in watched media should be added if found
	// Default is false
	AddFound bool `toml:"add_found" displayname:"Auto Add Discovered Media" comment:"Automatically add discovered media to your wanted list when found during scans.\nWhen true, media files found in this storage path are automatically added to tracking.\nUseful for discovering media that was added outside the application.\nWhen false, only manually added or list-imported media is tracked.\nHelps build your library from existing collections or shared storage.\nRequires add_found_list to specify which list to add discoveries to.\nDefault: false (manual management only)"`
	// AddFoundList is the list name that found entries should be added to
	AddFoundList string `toml:"add_found_list" displayname:"Discovery Target List" comment:"Name of the list where automatically discovered media should be added.\nSpecifies which list configuration to use when add_found is enabled.\nMust reference a list defined in the lists section of this media group.\nThe list determines:\n- Metadata sources and update behavior\n- Search and upgrade preferences\n- Quality requirements for discovered media\nRequired when add_found is true, ignored when false.\nExample: 'discovered-movies' for a list dedicated to found media"`
	// AddFoundListCfg is a pointer to ListsConfig
	AddFoundListCfg *ListsConfig `toml:"-"`
}

// MediaDataImportConfig defines the configuration for importing media data.
type MediaDataImportConfig struct {
	// TemplatePath is the template to use for the path
	TemplatePath string `toml:"template_path" displayname:"Import Path Template" comment:"Name of the path configuration template to use for importing existing media files.\nReferences a path configuration defined in the paths section.\nThe import path template controls:\n- Source directories to scan for existing media files\n- File types and extensions to import\n- Size limits and quality filters for import\n- Whether to move, copy, or hardlink imported files\n- File organization and renaming during import\nMust exactly match the 'name' field of a paths configuration.\nTypically uses different settings than regular download paths.\nExample: 'import-movies' for a path optimized for importing existing collections"`
	// CfgPath is the PathsConfig reference
	CfgPath *PathsConfig `toml:"-"`
}

// MediaListsConfig defines a media list configuration.
type MediaListsConfig struct {
	// Name is the name of the list - use this name in ignore or replace lists
	Name string `toml:"name" displayname:"List Configuration Name" comment:"Unique identifier name for this media list configuration within the media group.\nThis name is used to reference this list in other configurations and operations.\nMust be unique within this media group but can be reused across different media groups.\nUsed when referencing this list in:\n- ignore_template_lists and replace_template_lists of other lists\n- add_found_list references in data configurations\n- Log messages and web interface displays\nChoose descriptive names that indicate the list's purpose or source.\nExample: 'imdb-watchlist', 'trakt-collection', 'manual-movies'"`
	// TemplateList is the template to use for the list
	TemplateList string `toml:"template_list" displayname:"External List Template" comment:"Name of the list configuration template to use for external list integration.\nReferences a list configuration defined in the lists section.\nThe list template controls:\n- External list source (IMDB lists, Trakt lists, RSS feeds, etc.)\n- Update frequency and synchronization behavior\n- Filtering and processing rules for list entries\n- Authentication credentials for accessing external lists\nMust exactly match the 'name' field of a lists configuration.\nLeave empty if this is a manual list without external synchronization.\nExample: 'imdb-top250' for an IMDB top movies list template"`
	// CfgList is the pointer to the ListsConfig
	CfgList *ListsConfig `toml:"-"`
	// TemplateQuality is the template to use for the quality
	TemplateQuality string `toml:"template_quality" displayname:"Quality Profile Override" comment:"Name of the quality profile template to use for media from this list.\nReferences a quality configuration defined in the quality section.\nOverrides the media group's default quality settings for items from this specific list.\nUseful when different lists require different quality standards:\n- High-quality lists might use 4K/BluRay profiles\n- Bulk import lists might use standard HD profiles\nMust exactly match the 'name' field of a quality configuration.\nLeave empty to use the media group's default quality template.\nExample: 'uhd-quality' for a list requiring 4K content"`
	// CfgQuality is the pointer to the QualityConfig
	CfgQuality *QualityConfig `toml:"-"`
	// TemplateScheduler is the template to use for the scheduler - overrides default of media
	TemplateScheduler string `toml:"template_scheduler" displayname:"Scheduler Template Override" comment:"Name of the scheduler template to use for automated tasks for this list.\nReferences a scheduler configuration defined in the scheduler section.\nOverrides the media group's default scheduler settings for items from this specific list.\nUseful when different lists need different automation behavior:\n- Priority lists might check more frequently\n- Archive lists might check less often\n- New release lists might need immediate processing\nMust exactly match the 'name' field of a scheduler configuration.\nLeave empty to use the media group's default scheduler template.\nExample: 'priority-schedule' for high-priority list items"`
	// CfgScheduler is the pointer to the SchedulerConfig
	CfgScheduler *SchedulerConfig `toml:"-"`
	// IgnoreMapLists are the lists to check for ignoring entries
	IgnoreMapLists []string `toml:"ignore_template_lists" displayname:"Lists To Ignore" comment:"List of other list names whose entries should be ignored/skipped.\nWhen processing this list, any entries that exist in the specified ignore lists will be skipped.\nUseful for filtering out unwanted content or avoiding duplicates:\n- Skip entries that are in a 'blocked-movies' list\n- Ignore items already in a 'completed-series' list\n- Filter out content from a 'low-priority' list\nReferences other MediaListsConfig names within the same media group.\nProcessed before replace_template_lists during list processing.\nExample: ['completed-movies', 'blocked-content'] to skip those entries"                                multiline:"true"`
	// IgnoreMapListsQu is the quality string
	IgnoreMapListsQu string `toml:"-"`
	// IgnoreMapListsLen is the length of IgnoreMapLists
	IgnoreMapListsLen int `toml:"-"`
	// ReplaceMapLists are the lists to check for replacing entries
	ReplaceMapLists []string `toml:"replace_template_lists" displayname:"Lists To Override" comment:"List of other list names whose entries should override/replace entries in this list.\nWhen processing, entries from replace lists take precedence over this list's entries.\nUseful for priority management and quality upgrades:\n- Let 'high-priority' list override 'standard' list settings\n- Allow 'manual-additions' to override automated list imports\n- Use 'quality-upgrades' list to override original quality requirements\nReferences other MediaListsConfig names within the same media group.\nProcessed after ignore_template_lists during list processing.\nExample: ['manual-overrides', 'priority-queue'] to prioritize those entries"                               multiline:"true"`
	// ReplaceMapListsLen is the length of ReplaceMapLists
	ReplaceMapListsLen int `toml:"-"`
	// Enabled indicates if this configuration is active
	Enabled bool `toml:"enabled" displayname:"Enable List Processing" comment:"Enable or disable this list configuration.\nWhen true, this list is actively processed and its entries are managed.\nWhen false, this list is ignored during all operations:\n- No synchronization with external sources\n- No processing of list entries\n- No searches or downloads triggered by this list\nUseful for temporarily disabling lists without deleting the configuration.\nAlso useful during testing or when lists are under maintenance.\nDefault: false (list disabled)"`
	// Addfound indicates if entries not already watched should be added when found
	Addfound bool `toml:"add_found" displayname:"Auto Add Found Media" comment:"Automatically add discovered media to this list when found during file scans.\nWhen true, media files found during library scans are automatically added to this list.\nUseful for building lists from existing media collections:\n- Scan existing movie folders to populate a 'discovered-movies' list\n- Find TV series already on disk and add to 'existing-shows' list\n- Import media from shared storage or external drives\nWhen false, only manually added or externally synchronized entries are in the list.\nWorks in conjunction with the media group's data configuration settings.\nDefault: false (manual/external list management only)"`
}

// mediaNotificationConfig defines the configuration for notifications about media events.
type MediaNotificationConfig struct {
	// MapNotification is the template to use for the notification
	MapNotification string `toml:"template_notification" displayname:"Notification Template Reference" comment:"Name of the notification configuration template to use for this media event.\nReferences a notification configuration defined in the notification section.\nThe notification template controls:\n- Delivery method (Pushover, email, webhook, CSV file)\n- Authentication credentials and connection settings\n- Message formatting and delivery options\n- Rate limiting and retry behavior\nMust exactly match the 'name' field of a notification configuration.\nDifferent events can use different notification templates for varied delivery.\nExample: 'pushover-downloads' for mobile notifications"`
	// CfgNotification is the NotificationConfig reference
	CfgNotification *NotificationConfig `toml:"-"`
	// Event is the type of event this is for - use added_download or added_data
	Event string `toml:"event" displayname:"Notification Event Type" comment:"Type of media event that triggers this notification.\nSpecifies when this notification configuration should be used.\nSupported event types:\n- 'added_download': When media is successfully downloaded and added to library\n- 'added_data': When media files are manually added or imported\nEach event type can have different notification settings and messages.\nExample: 'added_download' for successful download notifications"`
	// Title is the title of your message (for pushover)
	Title string `toml:"title" displayname:"Notification Title Template" comment:"Notification title/subject line for the message.\nUsed as the title for Pushover notifications, email subject lines, etc.\nSupports template variables that are replaced with actual media information.\nKeep concise as some notification services limit title length.\nExample: 'New Movie added in {{.Configuration}}'"`
	// Message is the message body - look at https://github.com/Kellerman81/go_media_downloader/wiki/Groups for format info
	Message string `toml:"message" displayname:"Notification Message Template" comment:"Main notification message body content.\nSupports template variables that are replaced with actual media information.\nCan include multiple lines and detailed information.\nExample: '{{.Title}} - moved from {{.SourcePath}} to {{.Targetpath}}{{if .Replaced }} Replaced: {{ range .Replaced }}{{.}},{{end}}{{end}}'\nExample: '{{.Time}};{{.Title}};{{.Season}};{{.Episode}};{{.Tvdb}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}'\nExample: '{{.Time}};{{.Title}};{{.Year}};{{.Imdb}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}'\nSee wiki for complete variable list: https://github.com/Kellerman81/go_media_downloader/wiki/Groups"`
	// ReplacedPrefix is text to write in front of the old path if media was replaced
	ReplacedPrefix string `toml:"replaced_prefix" displayname:"File Replacement Prefix" comment:"Text prefix added to notifications when media files are replaced/upgraded.\nWhen existing media is replaced with better quality, this text appears before the old file path.\nHelps distinguish upgrade notifications from new download notifications.\nUseful for indicating what action was taken with the previous file.\nCommon prefixes:\n- 'Replaced: ' to indicate file replacement\n- 'Upgraded from: ' to show what was upgraded\n- 'Previous: ' to reference the old version\nExample: 'Upgraded from: ' results in 'Upgraded from: /path/to/old/file.mkv'"`
}

// DownloaderConfig is a struct that defines the configuration for a downloader client.
type DownloaderConfig struct {
	// Name is the name of the downloader template
	Name string `toml:"name" displayname:"Downloader Configuration Name" comment:"Unique name for this downloader configuration.\nUsed to identify this downloader in quality profiles and logs.\nChoose a descriptive name that identifies the client and purpose.\nExample: 'sabnzbd-main' or 'qbittorrent-movies'"`
	// DlType is the type of downloader, e.g. drone, nzbget, etc.
	DlType string `toml:"type" displayname:"Download Client Type" comment:"Type of download client software.\nSupported options:\n- 'sabnzbd': SABnzbd Usenet client\n- 'nzbget': NZBGet Usenet client\n- 'qbittorrent': qBittorrent torrent client\n- 'transmission': Transmission torrent client\n- 'rtorrent': rTorrent/ruTorrent client\n- 'deluge': Deluge torrent client\n- 'drone': Drone (Download to filesystem)\nExample: 'sabnzbd' or 'qbittorrent'"`
	// Hostname is the hostname to use if needed
	Hostname string `toml:"hostname" displayname:"Client Hostname Address" comment:"IP address or hostname of the download client.\nCan be a local IP (192.168.1.100), hostname (nas.local), or FQDN.\nUse 'localhost' or '127.0.0.1' for local installations.\nDo not include protocol (http://) or port number here.\nExample: '192.168.1.100' or 'localhost'"`
	// Port is the port to use if needed
	Port int `toml:"port" displayname:"Client Port Number" comment:"TCP port number where the download client is listening.\nCommon default ports:\n- SABnzbd: 8080\n- NZBGet: 6789\n- qBittorrent: 8080\n- Transmission: 9091\n- Deluge: 8112\nCheck your client's settings for the correct port.\nExample: 8080 or 6789"`
	// Username is the username to use if needed
	Username string `toml:"username" displayname:"Authentication Username" comment:"Username for authentication with the download client.\nRequired if your download client has authentication enabled.\nLeave empty if the client doesn't require authentication.\nSome clients allow guest access or have auth disabled.\nExample: 'admin' or 'myuser'"`
	// Password is the password to use if needed
	Password string `toml:"password" displayname:"Authentication Password" comment:"Password for authentication with the download client.\nRequired if your download client has authentication enabled.\nLeave empty if the client doesn't require authentication.\nFor API key-based clients, this may be the API key instead.\nExample: 'mypassword' or 'api-key-here'"`
	// AddPaused specifies whether to add entries in paused state
	AddPaused bool `toml:"add_paused" displayname:"Add Downloads Paused" comment:"Add downloads in paused state instead of starting immediately.\nWhen true, downloads are queued but not started automatically.\nUseful for manual review before starting downloads.\nWhen false, downloads start immediately after being added.\nDefault: false (start immediately)"`
	// DelugeDlTo is the Deluge target for downloads
	DelugeDlTo string `toml:"deluge_dl_to" displayname:"Deluge Download Directory" comment:"Initial download directory for Deluge client (Deluge-specific setting).\nThis is where files are downloaded before processing.\nMust be a path accessible to the Deluge daemon.\nOnly used when type is 'deluge'.\nExample: '/downloads/incomplete'"`
	// DelugeMoveAfter specifies if downloads should be moved after completion in Deluge
	DelugeMoveAfter bool `toml:"deluge_move_after" displayname:"Deluge Auto Move Files" comment:"Enable automatic file moving after download completion in Deluge.\nWhen true, completed downloads are moved to the path specified in deluge_move_to.\nWhen false, files remain in the download directory.\nOnly used when type is 'deluge'.\nDefault: false"`
	// DelugeMoveTo is the Deluge target for downloads after completion
	DelugeMoveTo string `toml:"deluge_move_to" displayname:"Deluge Completion Directory" comment:"Destination directory for completed downloads in Deluge.\nUsed when deluge_move_after is enabled.\nFiles are moved here after successful download completion.\nMust be a path accessible to the Deluge daemon.\nOnly used when type is 'deluge' and deluge_move_after is true.\nExample: '/downloads/complete'"`
	// Priority is the priority to set if needed
	Priority int `toml:"priority" displayname:"Default Download Priority" comment:"Default priority level for downloads added to this client.\nHigher numbers typically mean higher priority (client-dependent).\nCommon values: -2 (very low), -1 (low), 0 (normal), 1 (high), 2 (very high)\nCheck your download client's documentation for valid ranges.\nExample: 0 for normal priority"`
	// Enabled specifies if this template is active
	Enabled bool `toml:"enabled" displayname:"Enable Downloader Configuration" comment:"Enable or disable this downloader configuration.\nWhen true, this downloader can be used by quality profiles.\nWhen false, this downloader is ignored and won't receive downloads.\nUseful for temporarily disabling a downloader without deleting the config.\nDefault: true"`
}

// ListsConfig defines the configuration for lists.
type ListsConfig struct {
	// Name is the name of the template
	Name string `toml:"name" displayname:"List Configuration Name" comment:"Unique name for this list configuration.\nUsed to identify this list in logs and management interfaces.\nChoose a descriptive name that indicates the list source and purpose.\nExample: 'imdb-top250', 'trakt-watchlist', 'popular-movies'"`
	// ListType is the type of the list
	ListType string `toml:"type" displayname:"List Source Type" comment:"Type of list source to import from.\nAvailable options:\n- 'imdbcsv': IMDB CSV export file\n- 'imdbfile': IMDB watchlist file\n- 'seriesconfig': Local TOML series configuration\n- 'traktpublicmovielist': Public Trakt movie list\n- 'traktpublicshowlist': Public Trakt TV show list\n- 'traktmoviepopular': Trakt popular movies\n- 'traktmovieanticipated': Trakt anticipated movies\n- 'traktmovietrending': Trakt trending movies\n- 'traktseriepopular': Trakt popular TV series\n- 'traktserieanticipated': Trakt anticipated TV series\n- 'traktserietrending': Trakt trending TV series\n- 'newznabrss': Newznab RSS feed\nExample: 'imdbcsv' or 'traktmoviepopular'"`
	// URL is the url of the list
	URL string `toml:"url" displayname:"External List URL" comment:"URL for the list source (when applicable).\nRequired for:\n- Trakt public lists: Full Trakt list URL\n- RSS feeds: RSS feed URL\n- IMDB watchlists: IMDB watchlist URL\nNot needed for popular/trending lists or local files.\nExample: 'https://trakt.tv/users/username/lists/listname'\nExample: 'https://rss.example.com/movies.xml'"`
	// Enabled indicates if this template is active
	Enabled     bool   `toml:"enabled" displayname:"Enable List Processing" comment:"Enable or disable this list configuration.\nWhen true, this list will be processed during scheduled imports.\nWhen false, this list is ignored and won't be imported.\nUseful for temporarily disabling a list without deleting the config.\nDefault: true"`
	IMDBCSVFile string `toml:"imdb_csv_file" displayname:"IMDB CSV File Path" comment:"Path to IMDB CSV export file (for type 'imdbcsv').\nThis should be a CSV file exported from IMDB containing movie/show data.\nPath can be absolute or relative to the application directory.\nRequired when type is 'imdbcsv', ignored for other types.\nExample: './config/movies.csv' or '/path/to/imdb-export.csv'"`
	// SeriesConfigFile is the path of the toml file
	SeriesConfigFile string `toml:"series_config_file" displayname:"Series Config File Path" comment:"Path to TOML series configuration file (for type 'seriesconfig').\nThis should be a TOML file containing series definitions and settings.\nPath can be absolute or relative to the application directory.\nRequired when type is 'seriesconfig', ignored for other types.\nExample: './config/series.toml' or '/path/to/my-series.toml'"`
	// TraktUsername is the username who owns the trakt list
	TraktUsername string `toml:"trakt_username" displayname:"Trakt List Owner Username" comment:"Trakt username for public list access (for Trakt list types).\nThis is the username of the person who created/owns the Trakt list.\nRequired for public Trakt lists (traktpublicmovielist, traktpublicshowlist).\nNot needed for popular/trending lists as they don't belong to specific users.\nExample: 'moviefan123' or 'tvshowlover'"`
	// TraktListName is the listname of the trakt list
	TraktListName string `toml:"trakt_listname" displayname:"Trakt List Name" comment:"Name of the Trakt list to import (for public Trakt lists).\nThis is the list name as it appears in the Trakt URL.\nRequired for public Trakt lists (traktpublicmovielist, traktpublicshowlist).\nNot needed for popular/trending lists.\nExample: 'my-favorites' or 'must-watch-movies'"`
	// TraktListType is the listtype of the trakt list
	TraktListType string `toml:"trakt_listtype" displayname:"Trakt Content Type" comment:"Content type for Trakt lists (for Trakt list types).\nSpecifies whether the list contains movies or TV shows.\nAvailable options:\n- 'movie': List contains movies\n- 'show': List contains TV shows/series\nRequired for public Trakt lists to determine content type.\nExample: 'movie' for movie lists, 'show' for TV series lists"`
	// Limit is how many entries should only be processed
	Limit string `toml:"limit" displayname:"Maximum Items To Import" comment:"Maximum number of items to import from this list.\nSet to limit large lists to prevent overwhelming the system.\nUse '0' or leave empty to import all items from the list.\nHigher numbers import more items but take longer to process.\nUseful for testing or limiting popular lists to top items.\nExample: '50' for top 50 items, '0' for all items\nDefault: '0' (all items)"`
	// MinVotes only import if that number of imdb votes have been reached
	MinVotes int `toml:"min_votes" displayname:"Minimum IMDB Vote Count" comment:"Minimum IMDB vote count required for import (filtering criterion).\nOnly items with at least this many IMDB votes will be imported.\nHelps filter out obscure or low-quality content.\nSet to 0 to disable vote filtering.\nHigher values result in more popular/mainstream content.\nExample: 1000 for moderately popular, 10000 for very popular\nDefault: 0 (no filtering)"`
	// MinRating only import if that imdb rating has been reached
	MinRating float32 `toml:"min_rating" displayname:"Minimum IMDB Rating" comment:"Minimum IMDB rating required for import (filtering criterion).\nOnly items with at least this IMDB rating will be imported.\nHelps filter out low-quality or poorly rated content.\nRating scale is 0.0 to 10.0 (IMDB standard).\nSet to 0.0 to disable rating filtering.\nExample: 6.5 for decent quality, 7.5 for high quality\nDefault: 0.0 (no filtering)"`
	// Excludegenre don't import if it's one of the configured genres
	Excludegenre []string `toml:"exclude_genre" displayname:"Genres To Exclude" comment:"List of genres to exclude from import (filtering criterion).\nItems matching any of these genres will be skipped during import.\nUse exact genre names as they appear in IMDB/TMDB/Trakt.\nCommon genres: Action, Comedy, Drama, Horror, Romance, Sci-Fi, Thriller\nLeave empty to disable genre exclusion filtering.\nExample: ['Horror', 'Documentary', 'Reality-TV']"                                                                                                                                                                                      multiline:"true"`
	// Includegenre only import if it's one of the configured genres
	Includegenre []string `toml:"include_genre" displayname:"Genres To Include" comment:"List of genres to include in import (filtering criterion).\nOnly items matching at least one of these genres will be imported.\nUse exact genre names as they appear in IMDB/TMDB/Trakt.\nCommon genres: Action, Comedy, Drama, Horror, Romance, Sci-Fi, Thriller\nLeave empty to disable genre inclusion filtering (import all genres).\nExample: ['Action', 'Sci-Fi', 'Thriller']"                                                                                                                                                                                       multiline:"true"`
	// ExcludegenreLen is the length of Excludegenre
	ExcludegenreLen int `toml:"-"`
	// IncludegenreLen is the length of Includegenre
	IncludegenreLen int `toml:"-"`
	// URLExtensions for discover
	TmdbDiscover []string `toml:"tmdb_discover" displayname:"TMDB Discover Parameters" comment:"TMDB Discover API URL parameters for dynamic content discovery.\nUse TMDB Discover API parameters to find movies/shows matching specific criteria.\nParameters should be in URL query format without the base URL.\nSee TMDB API docs: https://developer.themoviedb.org/reference/discover-movie\nSee TMDB API docs: https://developer.themoviedb.org/reference/discover-tv\nExample: ['sort_by=popularity.desc&vote_average.gte=7']\nExample: ['with_genres=28,12&release_date.gte=2020-01-01']"`
	// List IDs of TMDB Lists
	TmdbList       []int `toml:"tmdb_list" displayname:"TMDB List IDs" comment:"List of TMDB list IDs to import from.\nThese are numeric IDs of public lists on The Movie Database.\nFind list IDs in TMDB list URLs: themoviedb.org/list/{ID}\nMultiple list IDs can be specified to import from several lists.\nRequires themoviedb_apikey to be configured in general settings.\nExample: [1, 28, 1000] for list IDs 1, 28, and 1000"`
	RemoveFromList bool  `toml:"remove_from_list" displayname:"Remove After Processing" comment:"Remove items from the list after successful processing.\nWhen true, items are removed from the source list after being imported.\nUseful for one-time imports or clearing processed items from lists.\nWhen false, items remain in the list for future processing.\nCaution: This permanently modifies the source list.\nDefault: false (keep items in list)"`
}

// IndexersConfig defines the configuration for indexers.
type IndexersConfig struct {
	// Name is the name of the template
	Name string `toml:"name" displayname:"Indexer Configuration Name" comment:"Unique name for this indexer configuration.\nUsed to identify this indexer in quality profiles and logs.\nChoose a descriptive name that identifies the indexer site.\nExample: 'nzbgeek', 'drunkenslug', 'nzbfinder'"`

	// IndexerType is the type of the indexer, currently has to be newznab
	IndexerType string `toml:"type" displayname:"Indexer Protocol Type" comment:"Protocol type used by this indexer.\nCurrently only 'newznab' is supported.\nNewznab is the standard API used by most Usenet indexers.\nTorrent indexers using Newznab-compatible APIs also use 'newznab'.\nExample: 'newznab'"`

	// URL is the main url of the indexer
	URL string `toml:"url" displayname:"Indexer Base URL" comment:"Base URL of the indexer website.\nThis should be the main domain without any API paths.\nDo not include '/api' or other paths - they're added automatically.\nMust include protocol (http:// or https://).\nExample: 'https://api.nzbgeek.info' or 'https://drunkenslug.com'"`

	// Apikey is the apikey for the indexer
	Apikey string `toml:"apikey" displayname:"Indexer API Key" comment:"API key for accessing this indexer.\nObtained from your indexer account settings or profile page.\nRequired for authentication and to track your usage limits.\nKeep this key secret and don't share it publicly.\nSome indexers call this 'API Token' or 'RSS Key'.\nExample: 'a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0'"`

	// Userid is the userid for rss queries to the indexer if needed
	Userid string `toml:"userid" displayname:"Indexer User ID" comment:"User ID for RSS feed access (if required by the indexer).\nSome indexers require both API key and user ID for RSS feeds.\nUsually found in your indexer account settings alongside the API key.\nLeave empty if the indexer doesn't require a user ID.\nExample: '12345' or 'username123'"`

	// Enabled indicates if this template is active
	Enabled bool `toml:"enabled" displayname:"Enable Indexer Configuration" comment:"Enable or disable this indexer configuration.\nWhen true, this indexer will be used for searches.\nWhen false, this indexer is ignored and won't be queried.\nUseful for temporarily disabling problematic indexers.\nDefault: true"`

	// Rssenabled indicates if this template is active for rss queries
	Rssenabled bool `toml:"rss_enabled" displayname:"Enable RSS Monitoring" comment:"Enable RSS feed monitoring for this indexer.\nWhen true, RSS feeds are checked for new releases automatically.\nWhen false, only manual searches are performed on this indexer.\nRSS monitoring helps catch new releases quickly.\nDefault: true"`

	// Addquotesfortitlequery indicates if quotes should be added to a title query
	Addquotesfortitlequery bool `toml:"add_quotes_for_title_query" displayname:"Add Quotes Title Search" comment:"Add quotes around title searches for exact matching.\nWhen true, searches like 'Movie Title' become '\"Movie Title\"'.\nImproves search accuracy but may reduce results.\nSome indexers work better with quotes, others without.\nTest with your indexer to see which works better.\nDefault: false"`

	// MaxEntries is the maximum number of entries to process, default is 100
	MaxEntries    uint16 `toml:"max_entries" displayname:"Maximum Search Results" comment:"Maximum number of search results to retrieve per query.\nHigher values get more results but increase processing time and API usage.\nSome indexers have limits on how many results they return.\nTypical range: 50-500 depending on indexer capabilities.\nDefault: 100"`
	MaxEntriesStr string `toml:"-"`
	// RssEntriesloop is the number of rss calls to make to find last processed release, default is 2
	RssEntriesloop uint8 `toml:"rss_entries_loop" displayname:"RSS Feed Pages" comment:"Number of RSS feed pages to check for finding the last processed release.\nHigher values ensure no releases are missed but increase API usage.\nUsed to determine where to resume RSS processing after downtime.\nIncrease if you frequently miss releases during outages.\nRange: 1-10 depending on indexer activity and downtime frequency.\nDefault: 2"`

	// OutputAsJSON indicates if the indexer should return json instead of xml
	// Not recommended since the conversion is sometimes different
	OutputAsJSON bool `toml:"output_as_json" displayname:"Request JSON Format" comment:"Request JSON format instead of XML from the indexer.\nSome indexers support JSON responses which can be faster to parse.\nNot recommended as JSON conversion may lose data or format differently.\nOnly enable if you experience XML parsing issues with this indexer.\nDefault: false (use XML format)"`

	// Customapi is used if the indexer needs a different value then 'apikey' for the key
	Customapi string `toml:"custom_api" displayname:"Custom API Parameter Name" comment:"Custom API parameter name if the indexer doesn't use 'apikey'.\nMost indexers use 'apikey' but some use different parameter names.\nCommon alternatives: 'api_key', 'token', 'key', 'apitoken'\nLeave empty if the indexer uses the standard 'apikey' parameter.\nCheck your indexer's API documentation for the correct parameter name.\nExample: 'api_key' or 'token'"`

	// Customurl is used if the indexer needs a different url then url/api/ or url/rss/
	Customurl string `toml:"custom_url" displayname:"Custom API Path" comment:"Custom API URL path if the indexer doesn't use standard paths.\nMost indexers use '/api' for API calls and '/rss' for RSS feeds.\nSome indexers use different paths like '/newznab/api' or '/api/v1'.\nLeave empty if the indexer uses standard '/api' and '/rss' paths.\nInclude the leading slash in your custom path.\nExample: '/newznab/api' or '/api/v2'"`

	// Customrssurl is used if the indexer uses a custom rss url (not url/rss/)
	Customrssurl string `toml:"custom_rss_url" displayname:"Custom RSS Path" comment:"Custom RSS URL path if different from the standard '/rss'.\nSome indexers use non-standard RSS paths or completely different URLs.\nCan be a relative path (e.g., '/feed') or absolute URL.\nLeave empty if the indexer uses the standard '/rss' path.\nCheck your indexer's RSS feed URL in their documentation.\nExample: '/feed' or 'https://indexer.com/rss.php'"`

	// Customrsscategory is used if the indexer uses something other than &t= for rss categories
	Customrsscategory string `toml:"custom_rss_category" displayname:"Custom RSS Category Parameter" comment:"Custom RSS category parameter if different from the standard '&t='.\nMost Newznab indexers use '&t=' for category filtering in RSS feeds.\nSome indexers use different parameters like '&cat=' or '&category='.\nLeave empty if the indexer uses the standard '&t=' parameter.\nCheck your indexer's RSS URL format for the correct parameter.\nExample: '&cat=' or '&category='"`

	// Limitercalls is the number of calls allowed in Limiterseconds
	Limitercalls int `toml:"limiter_calls" displayname:"API Calls Per Window" comment:"Number of API calls allowed within the limiter_seconds timeframe.\nUsed to respect the indexer's rate limiting to avoid being banned.\nCheck your indexer's API documentation for their rate limits.\nCommon limits: 1-10 calls per second depending on the indexer.\nSet conservatively to avoid hitting limits during busy periods.\nDefault: 1"`

	// Limiterseconds is the number of seconds for Limitercalls calls
	Limiterseconds uint8 `toml:"limiter_seconds" displayname:"Rate Limit Window Seconds" comment:"Time window in seconds for the limiter_calls limit.\nDefines the period over which the call limit applies.\nTogether with limiter_calls, controls the rate limiting.\nExample: limiter_calls=5, limiter_seconds=10 = 5 calls per 10 seconds.\nMost indexers use per-second limits, so typically set to 1.\nDefault: 1"`

	// LimitercallsDaily is the number of calls allowed daily, 0 is unlimited
	LimitercallsDaily int `toml:"limiter_calls_daily" displayname:"Daily API Call Limit" comment:"Maximum number of API calls allowed per day (24-hour period).\nHelps stay within daily API limits imposed by some indexers.\nSet to 0 for unlimited daily calls (only rate limiting applies).\nCheck your indexer account for daily API call limits.\nUseful for free accounts with daily restrictions.\nExample: 100 for limited accounts, 0 for unlimited\nDefault: 0 (unlimited)"`

	// MaxAge is the maximum age of releases in days
	MaxAge uint16 `toml:"max_age" displayname:"Maximum Release Age Days" comment:"Maximum age of releases to consider during searches (in days).\nReleases older than this age will be ignored.\nHelps focus on recent releases and reduces processing time.\nSet to 0 to disable age filtering (search all releases).\nTypical values: 30-365 days depending on content preferences.\nExample: 90 for 3 months, 365 for 1 year\nDefault: 0 (no age limit)"`

	// DisableTLSVerify disables SSL Certificate Checks
	DisableTLSVerify bool `toml:"disable_tls_verify" displayname:"Disable SSL Certificate Verification" comment:"Disable SSL certificate verification for this indexer.\nOnly enable if the indexer has SSL certificate issues.\nThis reduces security by allowing invalid/expired certificates.\nUse only as a last resort for indexers with certificate problems.\nMost indexers should work with SSL verification enabled.\nDefault: false (SSL verification enabled)"`

	// DisableCompression disables compression of data
	DisableCompression bool `toml:"disable_compression" displayname:"Disable HTTP Compression" comment:"Disable HTTP compression for requests to this indexer.\nCompression reduces bandwidth usage and improves performance.\nOnly disable if the indexer has issues with compressed responses.\nMost indexers support compression and it should remain enabled.\nMay be needed for older or misconfigured indexers.\nDefault: false (compression enabled)"`

	// TimeoutSeconds is the timeout in seconds for queries
	TimeoutSeconds uint16 `toml:"timeout_seconds" displayname:"Request Timeout Seconds" comment:"Maximum time to wait for indexer responses (in seconds).\nRequests taking longer than this will be cancelled.\nSet higher for slow indexers, lower for fast ones.\nToo low causes timeouts, too high delays error detection.\nTypical range: 30-120 seconds depending on indexer performance.\nExample: 60 for average indexers, 120 for slow ones\nDefault: 60"`

	TrustWithIMDBIDs bool `toml:"trust_with_imdb_ids" displayname:"Trust Indexer IMDB IDs" comment:"trust indexer imdb ids - can be problematic for RSS scans - some indexers tag wrong"`
	TrustWithTVDBIDs bool `toml:"trust_with_tvdb_ids" displayname:"Trust Indexer TVDB IDs" comment:"Trust TVDB IDs provided by this indexer for TV show identification.\nWhen true, indexer-provided TVDB IDs are used for matching.\nWhen false, titles are used for matching instead of IDs.\nSome indexers provide incorrect TVDB IDs, especially in RSS feeds.\nDisable if you notice incorrect TV show matches from this indexer.\nDefault: false (don't trust indexer TVDB IDs)"`

	// CheckTitleOnIDSearch is a bool indicating if the title of the release should be checked during an id based search? - default: false
	CheckTitleOnIDSearch bool `toml:"check_title_on_id_search" displayname:"Verify Title On ID Search" comment:"Verify release titles even when searching by IMDB/TVDB ID.\nWhen true, both ID and title must match for a release to be accepted.\nWhen false, only the ID needs to match (faster but less accurate).\nUseful when trust_with_imdb_ids or trust_with_tvdb_ids is enabled.\nHelps prevent incorrect matches from indexers with unreliable IDs.\nDefault: false (ID matching only)"`
}

type PathsConfig struct {
	// Name is the name of the media template
	Name string `toml:"name" displayname:"Path Configuration Name" comment:"Unique name for this path configuration.\nUsed to identify this path template in media configurations.\nChoose a descriptive name that indicates the media type or purpose.\nExample: 'movies-4k', 'tv-shows', 'anime', 'documentaries'"`
	// Path is the path where the media will be stored
	Path string `toml:"path" displayname:"Media Storage Directory" comment:"Absolute path where media files will be organized and stored.\nThis is the root directory for your media library.\nMust be an absolute path accessible to the application.\nEnsure the directory exists and has proper read/write permissions.\nExample: '/media/movies', '/mnt/storage/tv-shows', 'D:\\Movies'"`
	// AllowedVideoExtensions lists the allowed video file extensions
	AllowedVideoExtensions []string `toml:"allowed_video_extensions" displayname:"Video File Extensions" comment:"List of video file extensions that will be processed and renamed.\nThese files are considered main media files and will be renamed according to your naming scheme.\nInclude the dot (.) prefix for each extension.\nCommon video formats: .mkv, .mp4, .avi, .m4v, .wmv, .mov\nExample: ['.mkv', '.mp4', '.avi', '.m4v']"                                 multiline:"true"`
	// AllowedVideoExtensionsLen is the number of allowed video extensions
	AllowedVideoExtensionsLen int `toml:"-"`
	// AllowedOtherExtensions lists other allowed file extensions
	AllowedOtherExtensions []string `toml:"allowed_other_extensions" displayname:"Other File Extensions" comment:"List of non-video file extensions that will be copied alongside media files.\nThese are supplementary files like subtitles, NFOs, artwork, etc.\nThey will be renamed to match the main media file.\nInclude the dot (.) prefix for each extension.\nCommon formats: .srt, .nfo, .jpg, .png, .txt, .xml\nExample: ['.srt', '.nfo', '.jpg', '.png']"            multiline:"true"`
	// AllowedOtherExtensionsLen is the number of other allowed extensions
	AllowedOtherExtensionsLen int `toml:"-"`
	// AllowedVideoExtensionsNoRename lists video extensions that should not be renamed
	AllowedVideoExtensionsNoRename []string `toml:"allowed_video_extensions_no_rename" displayname:"Video Extensions No Rename" comment:"List of video file extensions that will be copied but NOT renamed.\nThese files are preserved with their original names.\nUseful for samples, trailers, or files you want to keep as-is.\nInclude the dot (.) prefix for each extension.\nExample: ['.sample.mkv', '.trailer.mp4'] for samples and trailers"       multiline:"true"`
	// AllowedVideoExtensionsNoRenameLen is the number of video extensions not to rename
	AllowedVideoExtensionsNoRenameLen int `toml:"-"`
	// AllowedOtherExtensionsNoRename lists other extensions not to rename
	AllowedOtherExtensionsNoRename []string `toml:"allowed_other_extensions_no_rename" displayname:"Other Extensions No Rename" comment:"List of non-video file extensions that will be copied but NOT renamed.\nThese files preserve their original names and are not matched to media.\nUseful for readme files, original NFOs, or reference materials.\nInclude the dot (.) prefix for each extension.\nExample: ['.txt', '.readme', '.original.nfo']"  multiline:"true"`
	// AllowedOtherExtensionsNoRenameLen is the number of other extensions not to rename
	AllowedOtherExtensionsNoRenameLen int `toml:"-"`
	// Blocked lists strings that will block processing of files
	Blocked []string `toml:"blocked" displayname:"Blocked File Patterns" comment:"List of strings that prevent file processing when found in filenames or paths.\nFiles containing any of these strings will be completely ignored.\nUse for blocking unwanted content, file types, or release groups.\nStrings are case-insensitive and can be partial matches.\nExample: ['sample', 'trailer', 'RARBG', 'password']"                                              multiline:"true"`
	// BlockedLen is the number of blocked strings
	BlockedLen int `toml:"-"`
	// Upgrade indicates if media should be upgraded
	Upgrade bool `toml:"upgrade" displayname:"Enable Quality Upgrades" comment:"Enable automatic quality upgrades for media in this path.\nWhen true, the system will search for better quality versions of existing files.\nUpgrades happen based on quality profiles (resolution, codec, etc.).\nWhen false, no upgrade searches are performed for this path.\nDefault: false"`
	// MinSize is the minimum media size in MB for searches
	MinSize int `toml:"min_size" displayname:"Minimum File Size MB" comment:"Minimum file size in megabytes for search filtering.\nReleases smaller than this size will be rejected during searches.\nHelps filter out low-quality releases, samples, and fake files.\nSet to 0 to disable minimum size filtering.\nTypical values: 100MB for TV episodes, 1000MB for movies\nExample: 500 for 500MB minimum"`
	// MaxSize is the maximum media size in MB for searches
	MaxSize int `toml:"max_size" displayname:"Maximum File Size MB" comment:"Maximum file size in megabytes for search filtering.\nReleases larger than this size will be rejected during searches.\nHelps avoid extremely large files that may be unwanted or problematic.\nSet to 0 to disable maximum size filtering.\nTypical values: 2000MB for TV episodes, 50000MB for 4K movies\nExample: 10000 for 10GB maximum"`
	// MinSizeByte is the minimum size in bytes
	MinSizeByte int64 `toml:"-"`
	// MaxSizeByte is the maximum size in bytes
	MaxSizeByte int64 `toml:"-"`
	// MinVideoSize is the minimum video size in MB for structure
	MinVideoSize int `toml:"min_video_size" displayname:"Minimum Video Size MB" comment:"Minimum video file size in megabytes for file organization.\nVideo files smaller than this size will not be organized/renamed.\nHelps exclude samples, trailers, and low-quality files from organization.\nSet to 0 to organize all video files regardless of size.\nTypical values: 50MB for TV episodes, 200MB for movies\nExample: 100 for 100MB minimum for organization"`
	// MinVideoSizeByte is the minimum video size in bytes
	MinVideoSizeByte int64 `toml:"-"`
	// CleanupsizeMB is the minimum size in MB to keep a folder, 0 removes all
	CleanupsizeMB int `toml:"cleanup_size_mb" displayname:"Folder Cleanup Size MB" comment:"Minimum total size in megabytes to keep a folder during cleanup.\nFolders with total content smaller than this size will be deleted.\nHelps remove leftover folders with only samples, subtitles, or small files.\nSet to 0 to remove all folders regardless of size (aggressive cleanup).\nTypical values: 50-200MB depending on your minimum file requirements\nExample: 100 to keep folders with at least 100MB of content"`
	// AllowedLanguages lists allowed languages for audio streams in videos
	AllowedLanguages []string `toml:"allowed_languages" displayname:"Allowed Audio Languages" comment:"List of allowed audio languages for video files.\nFiles without audio tracks in these languages may be rejected or flagged.\nUse ISO 639-1 two-letter language codes (en, de, fr, es, etc.).\nSometimes the audio streams can also have 3 char names or full names - so enter all possiblities.\nLeave empty to allow all languages without filtering.\nExample: ['en', 'de'] for English and German audio only"                                                                 multiline:"true"`
	// AllowedLanguagesLen is the number of allowed languages
	AllowedLanguagesLen int `toml:"-"`
	// Replacelower indicates if lower quality video files should be replaced, default false
	Replacelower bool `toml:"replace_lower" displayname:"Replace Lower Quality" comment:"Automatically replace existing files with higher quality versions.\nWhen true, better quality releases will replace lower quality existing files.\nReplacement is based on quality profiles (resolution, codec, bitrate, etc.).\nWhen false, duplicate files are kept or rejected based on other settings.\nDefault: false"`
	// Usepresort indicates if a presort folder should be used before media is moved, default false
	Usepresort bool `toml:"use_presort" displayname:"Enable Presort Directory" comment:"Use a temporary presort directory before final organization.\nWhen true, files are placed in presort_folder_path for manual review.\nAllows manual verification before files are moved to final locations.\nWhen false, files are moved directly to their final organized locations.\nUseful for quality control or manual intervention workflows.\nDefault: false"`
	// PresortFolderPath is the path to the presort folder
	PresortFolderPath string `toml:"presort_folder_path" displayname:"Presort Directory Path" comment:"Absolute path to the presort directory (when use_presort is enabled).\nFiles are temporarily placed here before manual organization.\nMust be an absolute path accessible to the application.\nShould be on the same filesystem as final media path for efficient moves.\nRequired when use_presort is true, ignored otherwise.\nExample: '/tmp/presort', '/downloads/presort'"`
	// UpgradeScanInterval is the number of days to wait after last search before looking for upgrades, 0 means don't wait
	UpgradeScanInterval int `toml:"upgrade_scan_interval" displayname:"Upgrade Search Wait Days" comment:"Minimum days to wait between upgrade searches for the same media.\nPrevents excessive searching by spacing out upgrade attempts.\nSet to 0 to disable waiting (search for upgrades every scan cycle).\nHigher values reduce indexer load but may delay finding upgrades.\nTypical values: 7-30 days depending on how frequently you want upgrades.\nExample: 14 for bi-weekly upgrade searches"`
	// MissingScanInterval is the number of days to wait after last search before looking for missing media, 0 means don't wait
	MissingScanInterval int `toml:"missing_scan_interval" displayname:"Missing Search Wait Days" comment:"Minimum days to wait between missing media searches for the same item.\nPrevents excessive searching by spacing out search attempts.\nSet to 0 to disable waiting (search for missing media every scan cycle).\nHigher values reduce indexer load but may delay finding new releases.\nTypical values: 1-7 days depending on how actively you want to search.\nExample: 3 for searches every 3 days"`
	// MissingScanReleaseDatePre is the minimum number of days to wait after media release before scanning, 0 means don't check
	MissingScanReleaseDatePre int `toml:"missing_scan_release_date_pre" displayname:"Pre Release Search Days" comment:"Days to wait before the official release date before starting searches.\nAllows searching for media before its official release date (for pre-releases).\nPositive values search X days before release, negative values wait X days after.\nSet to 0 to disable release date checking (search immediately when added).\nExample: -7 to wait 7 days after release, 3 to search 3 days before release"`
	// Disallowed lists strings that will block processing if found
	Disallowed []string `toml:"disallowed" displayname:"Disallowed File Patterns" comment:"List of strings that prevent file organization when found in release names.\nFiles are downloaded but not organized/renamed if they contain these strings.\nUseful for blocking specific release groups, qualities, or naming patterns.\nStrings are case-insensitive and can be partial matches.\nExample: ['CAM', 'TS', 'HDCAM', 'BadGroup'] to block low-quality releases"                                          multiline:"true"`
	// DisallowedLen is the number of disallowed strings
	DisallowedLen int `toml:"-"`
	// DeleteWrongLanguage indicates if media with wrong language should be deleted, default false
	DeleteWrongLanguage bool `toml:"delete_wrong_language" displayname:"Delete Wrong Language Files" comment:"Automatically delete files with audio languages not in allowed_languages.\nWhen true, files without allowed audio languages are deleted after download.\nWhen false, files are kept but may not be organized properly.\nOnly works when allowed_languages is configured.\nCaution: This permanently deletes files - use with care.\nDefault: false"`
	// DeleteDisallowed indicates if media with disallowed strings should be deleted, default false
	DeleteDisallowed bool `toml:"delete_disallowed" displayname:"Delete Disallowed Files" comment:"Automatically delete files containing disallowed strings.\nWhen true, files matching disallowed patterns are deleted after download.\nWhen false, files are kept but not organized (safer option).\nOnly affects files matching strings in the disallowed list.\nCaution: This permanently deletes files - use with care.\nDefault: false"`
	// CheckRuntime indicates if runtime should be checked before import, default false
	CheckRuntime bool `toml:"check_runtime" displayname:"Enable Runtime Verification" comment:"Verify video runtime against expected duration before organization.\nWhen true, video files are checked against database runtime information.\nHelps detect incomplete, fake, or incorrectly matched files.\nWhen false, runtime verification is skipped (faster processing).\nRequires metadata sources with runtime information to be effective.\nDefault: false"`
	// MaxRuntimeDifference is the max minutes of difference allowed in runtime checks, 0 means no check
	MaxRuntimeDifference int `toml:"max_runtime_difference" displayname:"Max Runtime Difference Minutes" comment:"Maximum allowed runtime difference in minutes for runtime verification.\nFiles with runtime differing more than this amount are flagged or rejected.\nAccounts for encoding differences, credits, and metadata inaccuracies.\nSet to 0 to disable runtime checking entirely.\nTypical values: 5-15 minutes depending on content type and tolerance.\nExample: 10 to allow up to 10 minutes difference"`
	// DeleteWrongRuntime indicates if media with wrong runtime should be deleted, default false
	DeleteWrongRuntime bool `toml:"delete_wrong_runtime" displayname:"Delete Wrong Runtime Files" comment:"Automatically delete files that fail runtime verification.\nWhen true, files with runtime outside max_runtime_difference are deleted.\nWhen false, files are kept but may not be organized (safer option).\nOnly works when check_runtime is enabled and max_runtime_difference is set.\nCaution: This permanently deletes files - use with care.\nDefault: false"`
	// MoveReplaced indicates if replaced media should be moved to old folder, default false
	MoveReplaced bool `toml:"move_replaced" displayname:"Move Replaced Files" comment:"Move replaced files to a backup directory instead of deleting them.\nWhen true, old files are moved to move_replaced_target_path when upgraded.\nWhen false, old files are deleted during replacement (saves space).\nProvides a safety net for undoing upgrades if needed.\nRequires move_replaced_target_path to be configured.\nDefault: false"`
	// MoveReplacedTargetPath is the path to the folder for replaced media
	MoveReplacedTargetPath string `toml:"move_replaced_target_path" displayname:"Replaced Files Directory" comment:"Absolute path where replaced/upgraded files are moved for backup.\nUsed when move_replaced is enabled to store old versions of files.\nMust be an absolute path accessible to the application.\nConsider storage space as this will accumulate replaced files over time.\nRequired when move_replaced is true, ignored otherwise.\nExample: '/backup/replaced-media', '/storage/old-files'"`
	// SetChmod is the chmod for files in octal format, default 0777
	SetChmod       string `toml:"set_chmod" displayname:"File Permissions Octal" comment:"File permissions to set on organized media files (Unix/Linux only).\nUse octal format (3-4 digits) to specify read/write/execute permissions.\nApplied to all organized files to ensure consistent access permissions.\nIgnored on Windows systems - only affects Unix-like systems.\nCommon values: '0644' (rw-r--r--), '0664' (rw-rw-r--), '0777' (rwxrwxrwx)\nDefault: '0777'"`
	SetChmodFolder string `toml:"set_chmod_folder" displayname:"Folder Permissions Octal" comment:"Directory permissions to set on organized media folders (Unix/Linux only).\nUse octal format (3-4 digits) to specify read/write/execute permissions.\nApplied to all created directories to ensure consistent access permissions.\nIgnored on Windows systems - only affects Unix-like systems.\nFolders typically need execute permission for access (x bit set).\nCommon values: '0755' (rwxr-xr-x), '0775' (rwxrwxr-x), '0777' (rwxrwxrwx)\nDefault: '0777'"`
}

// NotificationConfig defines the configuration for notifications.
type NotificationConfig struct {
	// Name is the name of the notification template
	Name string `toml:"name" displayname:"Notification Configuration Name" comment:"Unique name for this notification configuration.\nUsed to identify this notification method in media configurations.\nChoose a descriptive name that indicates the notification type and purpose.\nExample: 'pushover-main', 'csv-log', 'slack-alerts'"`
	// NotificationType is the type of notification - use csv or pushover
	NotificationType string `toml:"type" displayname:"Notification Service Type" comment:"Type of notification service to use.\nAvailable options:\n- 'csv': Write notifications to a CSV file for logging/tracking\n- 'pushover': Send push notifications via Pushover service\nCSV is useful for record keeping, Pushover for real-time alerts.\nExample: 'pushover' for mobile notifications, 'csv' for logs"`
	// Apikey is the pushover apikey - create here: https://pushover.net/apps/build
	Apikey string `toml:"apikey" displayname:"Pushover API Key" comment:"API key for Pushover service (required when type is 'pushover').\nObtain by creating an application at: https://pushover.net/apps/build\nThis identifies your application to the Pushover service.\nKeep this key secure and don't share it publicly.\nLeave empty if using CSV notifications.\nExample: 'azGDORePK8gMaC0QOYAMyEEuzJnyUi'"`
	// Recipient is the pushover recipient
	Recipient string `toml:"recipient" displayname:"Pushover User Key" comment:"Pushover user key or group key to receive notifications.\nThis is the target recipient for Pushover notifications.\nFind your user key in your Pushover account dashboard.\nCan be a user key (for individual) or group key (for groups).\nRequired when type is 'pushover', ignored for CSV notifications.\nExample: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG'"`
	// Outputto is the path to output csv notifications
	Outputto string `toml:"output_to" displayname:"CSV Output File Path" comment:"File path for CSV notification output (required when type is 'csv').\nNotifications will be appended to this CSV file with timestamps.\nPath can be absolute or relative to the application directory.\nFile will be created if it doesn't exist, appended to if it does.\nIgnored when using Pushover notifications.\nExample: './logs/notifications.csv' or '/var/log/media-notifications.csv'"`
}

// RegexConfig is a struct that defines a regex template
// It contains fields for the template name, required regexes,
// rejected regexes, and lengths of the regex slices.
type RegexConfig struct {
	// Name is the name of the regex template
	Name string `toml:"name" displayname:"Regex Filter Name" comment:"Unique name for this regex filter configuration.\nUsed to identify this regex set in quality profiles and logs.\nChoose a descriptive name that indicates the filtering purpose.\nExample: 'no-cams', 'preferred-groups', 'block-samples'"`
	// Required is a slice of regex strings that are required (one must match)
	Required []string `toml:"required" displayname:"Required Pattern Matches" comment:"List of regular expressions where at least ONE must match for acceptance.\nReleases must match at least one of these patterns to be considered.\nUse standard regex syntax - patterns are case-insensitive by default.\nUseful for requiring specific release groups, qualities, or naming patterns.\nLeave empty to disable required pattern filtering.\nExample: ['SPARKS', 'FGT', 'DIMENSION'] to require specific groups" multiline:"true"`
	// Rejected is a slice of regex strings that cause rejection if matched
	Rejected []string `toml:"rejected" displayname:"Rejected Pattern Matches" comment:"List of regular expressions that cause immediate rejection if ANY match.\nReleases matching any of these patterns will be rejected/blocked.\nUse standard regex syntax - patterns are case-insensitive by default.\nUseful for blocking unwanted release groups, qualities, or content types.\nProcessed after required patterns - rejection overrides acceptance.\nExample: ['CAM', 'TS', 'HDCAM', '.*YIFY.*'] to block low-quality releases"         multiline:"true"`
	// RequiredLen is the length of the Required slice
	RequiredLen int `toml:"-"`
	// RejectedLen is the length of the Rejected slice
	RejectedLen int `toml:"-"`
}

type QualityConfig struct {
	// Name is the name of the template
	Name string `toml:"name" displayname:"Quality Profile Name" comment:"Unique name for this quality profile configuration.\nUsed to identify this quality profile in media configurations.\nChoose a descriptive name that indicates the quality standards.\nExample: 'uhd-4k', 'hd-1080p', 'standard-720p', 'anime-preferred'"`
	// WantedResolution is resolutions which are wanted - others are skipped - empty = allow all
	WantedResolution []string `toml:"wanted_resolution" displayname:"Accepted Video Resolutions" comment:"List of acceptable video resolutions for this quality profile.\nReleases not matching these resolutions will be rejected.\nLeave empty to accept all resolutions without filtering.\nCommon values: '2160p', '1080p', '720p', '576p', '480p'\nExample: ['2160p', '1080p'] for UHD and Full HD only"  multiline:"true"`
	// WantedQuality is qualities which are wanted - others are skipped - empty = allow all
	WantedQuality []string `toml:"wanted_quality" displayname:"Accepted Source Quality Types" comment:"List of acceptable video quality levels for this profile.\nReleases not matching these quality standards will be rejected.\nLeave empty to accept all quality levels without filtering.\nCommon values: 'BluRay', 'WEB-DL', 'WEBRip', 'HDTV', 'DVD'\nExample: ['BluRay', 'WEB-DL'] for highest quality sources only"    multiline:"true"`
	// WantedAudio is audio codecs which are wanted - others are skipped - empty = allow all
	WantedAudio []string `toml:"wanted_audio" displayname:"Accepted Audio Codecs" comment:"List of acceptable audio codecs and formats for this profile.\nReleases not matching these audio specifications will be rejected.\nLeave empty to accept all audio formats without filtering.\nCommon values: 'DTS', 'AC3', 'AAC', 'FLAC', 'TrueHD', 'Atmos'\nExample: ['DTS', 'TrueHD', 'Atmos'] for high-quality audio only" multiline:"true"`
	// WantedCodec is video codecs which are wanted - others are skipped - empty = allow all
	WantedCodec []string `toml:"wanted_codec" displayname:"Accepted Video Codecs" comment:"List of acceptable video codecs for this profile.\nReleases not matching these video encoding standards will be rejected.\nLeave empty to accept all video codecs without filtering.\nCommon values: 'x264', 'x265', 'H.264', 'H.265', 'HEVC', 'AV1'\nExample: ['x265', 'HEVC'] for modern efficient encoding only" multiline:"true"`
	// WantedResolutionLen is the length of the WantedResolution slice
	WantedResolutionLen int `toml:"-"`
	// WantedQualityLen is the length of the WantedQuality slice
	WantedQualityLen int `toml:"-"`
	// WantedAudioLen is the length of the WantedAudio slice
	WantedAudioLen int `toml:"-"`
	// WantedCodecLen is the length of the WantedCodec slice
	WantedCodecLen int `toml:"-"`
	// CutoffResolution is after which resolution should we stop searching for upgrades
	CutoffResolution string `toml:"cutoff_resolution" displayname:"Upgrade Stop Resolution" comment:"Resolution at which upgrade searches stop (satisfaction point).\nOnce media reaches this resolution, no further upgrades are sought.\nMust be one of the resolutions listed in wanted_resolution.\nSet to the highest quality you want to prevent excessive upgrading.\nExample: '2160p' to stop upgrading once 4K is achieved"`
	// CutoffQuality is after which quality should we stop searching for upgrades
	CutoffQuality string `toml:"cutoff_quality" displayname:"Upgrade Stop Quality" comment:"Quality level at which upgrade searches stop (satisfaction point).\nOnce media reaches this quality, no further upgrades are sought.\nMust be one of the qualities listed in wanted_quality.\nSet to the highest quality you want to prevent excessive upgrading.\nExample: 'BluRay' to stop upgrading once Blu-ray quality is achieved"`
	// CutoffPriority is the priority cutoff
	CutoffPriority int `toml:"-"`

	// SearchForTitleIfEmpty is a bool indicating if we should do a title search if the id search didn't return an accepted release
	// - backup_search_for_title needs to be true? - default: false
	SearchForTitleIfEmpty bool `toml:"search_for_title_if_empty" displayname:"Fallback Title Search" comment:"Enable title-based searching when ID-based search yields no results.\nWhen true, if IMDB/TVDB ID searches fail, fallback to title searches.\nRequires backup_search_for_title to be enabled to function.\nUseful when indexers have limited ID coverage but good title matching.\nIncreases search coverage but may reduce accuracy.\nDefault: false"`

	// BackupSearchForTitle is a bool indicating if we want to search for titles and not only id's - default: false
	BackupSearchForTitle bool `toml:"backup_search_for_title" displayname:"Enable Title Search Backup" comment:"Enable title-based searches as a backup to ID-based searches.\nWhen true, searches can use media titles when ID searches are insufficient.\nProvides broader search coverage when indexers lack proper ID tagging.\nRequired for search_for_title_if_empty functionality.\nMay increase false positives due to title ambiguity.\nDefault: false"`

	// SearchForAlternateTitleIfEmpty is a bool indicating if we should do a alternate title search if the id search didn't return an accepted release
	// - backup_search_for_alternate_title needs to be true? - default: false
	SearchForAlternateTitleIfEmpty bool `toml:"search_for_alternate_title_if_empty" displayname:"Fallback Alternate Title Search" comment:"Enable alternate title searching when ID-based search yields no results.\nWhen true, if IMDB/TVDB ID searches fail, search using alternate/foreign titles.\nRequires backup_search_for_alternate_title to be enabled to function.\nUseful for finding releases with regional or translated titles.\nFurther increases search coverage but may reduce accuracy.\nDefault: false"`

	// BackupSearchForAlternateTitle is a bool indicating if we want to search for alternate titles and not only id's - default: false
	BackupSearchForAlternateTitle bool `toml:"backup_search_for_alternate_title" displayname:"Enable Alternate Title Backup" comment:"Enable alternate title searches as a backup to ID-based searches.\nWhen true, searches can use foreign language titles and aliases.\nHelps find releases using regional names, translations, or alternate titles.\nRequired for search_for_alternate_title_if_empty functionality.\nIncreases search coverage but may have accuracy trade-offs.\nDefault: false"`

	// ExcludeYearFromTitleSearch is a bool indicating if the year should not be included in the title search? - default: false
	ExcludeYearFromTitleSearch bool `toml:"exclude_year_from_title_search" displayname:"Exclude Year From Title Search" comment:"Exclude release year from title-based searches.\nWhen true, searches use only the title without the year.\nUseful when indexers have inconsistent or missing year information.\nMay increase matches but also increases chance of wrong matches.\nOnly affects title searches, not ID-based searches.\nDefault: false"`

	// CheckUntilFirstFound is a bool indicating if we should stop searching if we found a release? - default: false
	CheckUntilFirstFound bool `toml:"check_until_first_found" displayname:"Stop At First Match" comment:"Stop searching across indexers after finding the first acceptable release.\nWhen true, search stops at first match that passes quality filters.\nWhen false, all indexers are searched to find the best available release.\nEnabling speeds up searches but may miss better quality releases.\nDisabling finds best quality but increases search time and API usage.\nDefault: false"`

	// CheckTitle is a bool indicating if the title of the release should be checked? - default: false
	CheckTitle bool `toml:"check_title" displayname:"Verify Release Title" comment:"Verify that release titles match the expected media title.\nWhen true, release titles are compared against media titles for accuracy.\nHelps prevent downloading incorrectly named or mismatched releases.\nWhen false, title verification is skipped (faster but less accurate).\nRecommended for reducing false positives and wrong downloads.\nDefault: false"`

	// CheckTitleOnIDSearch is a bool indicating if the title of the release should be checked during an id based search? - default: false
	CheckTitleOnIDSearch bool `toml:"check_title_on_id_search" displayname:"Verify Title On ID Search" comment:"Verify release titles even when searching by IMDB/TVDB ID.\nWhen true, both ID and title must match for acceptance.\nWhen false, ID matching alone is sufficient (faster).\nUseful when indexers have unreliable ID tagging.\nProvides extra verification at the cost of some performance.\nDefault: false"`

	// CheckYear is a bool indicating if the year of the release should be checked? - default: false
	CheckYear bool `toml:"check_year" displayname:"Verify Release Year" comment:"Verify that release years match the expected media release year.\nWhen true, release years must exactly match the media's release year.\nHelps prevent downloading releases from wrong years (remakes, etc.).\nWhen false, year verification is skipped.\nUseful for ensuring correct version matching.\nDefault: false"`

	// CheckYear1 is a bool indicating if the year of the release should be checked and is +-1 year allowed? - default: false
	CheckYear1 bool `toml:"check_year1" displayname:"Verify Year Plus Minus One" comment:"Verify release years with ±1 year tolerance from expected year.\nWhen true, releases within 1 year of the expected year are accepted.\nMore flexible than check_year for handling release date variations.\nAccounts for different regional release dates or metadata discrepancies.\nWhen false, no year tolerance is applied.\nDefault: false"`

	// TitleStripSuffixForSearch is a []string indicating what suffixes should be removed from the title
	TitleStripSuffixForSearch []string `toml:"title_strip_suffix_for_search" displayname:"Strip Title Suffixes" multiline:"true" comment:"List of suffixes to remove from titles before searching.\nHelps normalize titles by removing common suffixes that vary between sources.\nProcessed before sending search queries to indexers.\nCommon suffixes include year ranges, edition markers, or format indicators.\nLeave empty to search with original titles."`

	// TitleStripPrefixForSearch is a []string indicating what prefixes should be removed from the title
	TitleStripPrefixForSearch []string `toml:"title_strip_prefix_for_search" displayname:"Strip Title Prefixes" multiline:"true" comment:"List of prefixes to remove from titles before searching.\nHelps normalize titles by removing common prefixes that vary between sources.\nProcessed before sending search queries to indexers.\nCommon prefixes include articles, franchise markers, or format indicators.\nLeave empty to search with original titles."`

	// QualityReorder is a []QualityReorderConfig for configs if a quality reordering is needed - for example if 720p releases should be preferred over 1080p
	QualityReorder []QualityReorderConfig `toml:"reorder" displayname:"Quality Priority Reorder Rules" comment:"Custom priority reordering rules for specific quality characteristics.\nAllows overriding default priority calculations for special cases.\nUseful when you prefer certain resolutions, codecs, or groups over others.\nEach rule specifies what to match and what new priority to assign.\nExample: Prefer 720p over 1080p for bandwidth-limited situations.\nLeave empty to use default priority calculations."`

	// Indexer is a []QualityIndexerConfig for configs of the indexers to be used for this quality
	Indexer    []QualityIndexerConfig `toml:"indexers" displayname:"Indexer Configurations" comment:"List of indexer configurations specific to this quality profile.\nDefines which indexers to use and their specific settings for this profile.\nEach entry maps to an indexer template and can override default settings.\nAllows different search strategies per quality profile.\nRequired - must specify at least one indexer for searches to work."`
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
	UseForPriorityResolution bool `toml:"use_for_priority_resolution" displayname:"Use Resolution For Priority" comment:"Include video resolution in priority calculations for release ranking.\nWhen true, higher resolutions get higher priority scores.\nHelps automatically prefer 4K over 1080p, 1080p over 720p, etc.\nRecommended for most users who want the highest available resolution.\nWhen false, resolution doesn't affect priority ranking.\nDefault: false, Recommended: true"`
	// UseForPriorityQuality indicates if quality should be used for priority
	UseForPriorityQuality bool `toml:"use_for_priority_quality" displayname:"Use Quality For Priority" comment:"Include source quality in priority calculations for release ranking.\nWhen true, higher quality sources get higher priority scores.\nHelps automatically prefer BluRay over WEB-DL, WEB-DL over HDTV, etc.\nRecommended for most users who want the highest available quality.\nWhen false, source quality doesn't affect priority ranking.\nDefault: false, Recommended: true"`
	// UseForPriorityAudio indicates if audio codecs should be used for priority
	UseForPriorityAudio bool `toml:"use_for_priority_audio" displayname:"Use Audio For Priority" comment:"Include audio codec quality in priority calculations for release ranking.\nWhen true, preferred audio codecs (DTS, TrueHD, Atmos) get higher priority.\nHelps distinguish between releases with different audio quality levels.\nMay not be necessary if you don't have strong audio preferences.\nWhen false, audio codec doesn't affect priority ranking.\nDefault: false, Recommended: false (unless audio quality is critical)"`
	// UseForPriorityCodec indicates if video codecs should be used for priority
	UseForPriorityCodec bool `toml:"use_for_priority_codec" displayname:"Use Codec For Priority" comment:"Include video codec efficiency in priority calculations for release ranking.\nWhen true, modern codecs (x265, HEVC, AV1) may get priority adjustments.\nHelps distinguish between older and newer encoding technologies.\nMay not be necessary unless you have specific codec preferences.\nWhen false, video codec doesn't affect priority ranking.\nDefault: false, Recommended: false (unless codec efficiency is important)"`
	// UseForPriorityOther indicates if other data should be used for priority
	UseForPriorityOther bool `toml:"use_for_priority_other" displayname:"Use Metadata For Priority" comment:"Include release metadata in priority calculations for ranking.\nWhen true, factors like REPACK, PROPER, EXTENDED, UNCUT affect priority.\nHelps prefer corrected releases and special editions over originals.\nUseful for getting the best available version of releases.\nWhen false, these metadata factors don't affect priority ranking.\nDefault: false, Recommended: false (unless you want enhanced versions)"`
	// UseForPriorityMinDifference is the min difference to use a release for upgrade
	UseForPriorityMinDifference int `toml:"use_for_priority_min_difference" displayname:"Minimum Priority Upgrade Difference" comment:"Minimum priority score difference required to trigger an upgrade.\nOnly releases with priority scores this much higher will replace existing files.\nHigher values make upgrades more selective, lower values more aggressive.\nSet to 0 to upgrade for any improvement, however small.\nHelps prevent excessive upgrading for marginal improvements.\nTypical values: 0-50, where 0 = any improvement, 20 = significant improvement only\nDefault: 0 (upgrade for any improvement)"`
}

// QualityReorderConfig is a struct for configuring reordering of qualities
// It contains a Name string field for the name of the quality
// A ReorderType string field for the type of reordering
// And a Newpriority int field for the new priority.
type QualityReorderConfig struct {
	// Name is the name of the quality to reorder
	Name string `toml:"name" displayname:"Quality Pattern To Reorder" comment:"Name or pattern of the quality characteristic to reorder.\nSpecifies which quality aspect should have its priority modified.\nSupports multiple values separated by commas (no spaces).\nExamples based on reorder type:\n- Resolution: '1080p', '2160p', '720p'\n- Quality: 'BluRay', 'WEB-DL', 'HDTV'\n- Codec: 'x265', 'HEVC', 'x264'\n- Audio: 'DTS', 'AC3', 'AAC'\n- Multiple values: '1080p,2160p' or 'BluRay,WEB-DL'\nMust match the actual values found in release names.\nExample: '720p,1080p' to reorder multiple resolutions"`
	// ReorderType is the type of reordering to use
	ReorderType string `toml:"type" displayname:"Reorder Type" comment:"Type of quality characteristic to reorder for priority calculation.\nSpecifies which aspect of releases should have custom priority scoring.\nSupported reorder types:\n- 'resolution': Video resolution (720p, 1080p, 2160p, etc.)\n- 'quality': Source quality (BluRay, WEB-DL, HDTV, etc.)\n- 'codec': Video codec (x264, x265, HEVC, AV1, etc.)\n- 'audio': Audio codec (DTS, AC3, AAC, FLAC, etc.)\n- 'position': Custom positional priority (manual ranking)\n- 'combined_res_qual': Combined resolution and quality scoring\nDifferent types affect how new_priority values are applied.\nExample: 'resolution' to customize resolution priority scoring"`
	// Newpriority is the new priority to set for the quality
	Newpriority int `toml:"new_priority" displayname:"New Priority Value" comment:"Custom priority value to assign to the specified quality characteristic.\nHow this value is applied depends on the reorder type:\n- 'resolution', 'quality', 'codec', 'audio': Direct priority assignment\n- 'position': Multiplied by position number for ranking\n- 'combined_res_qual': Resolution gets this value, quality set to 0\nHigher numbers = higher priority in search results.\nUseful for preferring specific characteristics:\n- Set 720p to priority 100 to prefer over 1080p (bandwidth saving)\n- Set x265 to priority 150 for codec preference\n- Set BluRay to priority 200 for quality preference\nTypical range: 0-1000, where higher values are preferred.\nExample: 150 to give moderate preference to specified items"`
}

// QualityIndexerConfig defines the configuration for an indexer used for a specific quality.
type QualityIndexerConfig struct {
	// TemplateIndexer is the template to use for the indexer
	TemplateIndexer string `toml:"template_indexer" displayname:"Indexer Template Name" comment:"Name of the indexer configuration template to use for this quality profile.\nReferences an indexer configuration defined in the indexers section.\nThe indexer template controls:\n- API connection details and authentication\n- Search capabilities and supported categories\n- Rate limiting and timeout settings\n- Site-specific search parameters\nMust exactly match the 'name' field of an indexers configuration.\nDifferent quality profiles can use different indexers for specialized content.\nExample: 'nzbgeek-hd' for a high-definition focused indexer setup"`
	// CfgIndexer is a pointer to the IndexersConfig for this indexer
	CfgIndexer *IndexersConfig `toml:"-"`
	// TemplateDownloader is the template to use for the downloader
	TemplateDownloader string `toml:"template_downloader" displayname:"Downloader Template Name" comment:"Name of the downloader configuration template to use for this indexer.\nReferences a downloader configuration defined in the downloader section.\nThe downloader template controls:\n- Download client connection (SABnzbd, NZBGet, qBittorrent, etc.)\n- Authentication credentials and API settings\n- Download categories and priority settings\n- Post-processing behavior\nMust exactly match the 'name' field of a downloader configuration.\nAllows different indexers to use different download clients or settings.\nExample: 'sabnzbd-movies' for movie-specific download handling"`
	// CfgDownloader is a pointer to the DownloaderConfig for this downloader
	CfgDownloader *DownloaderConfig `toml:"-"`
	// TemplateRegex is the template to use for the regex
	TemplateRegex string `toml:"template_regex" displayname:"Regex Template Name" comment:"Name of the regex configuration template to use for filtering releases from this indexer.\nReferences a regex configuration defined in the regex section.\nThe regex template controls:\n- Release name patterns to require or reject\n- Group name filtering rules\n- Quality and format validation patterns\n- Size and naming convention filters\nMust exactly match the 'name' field of a regex configuration.\nLeave empty to disable regex filtering for this indexer.\nUseful for indexer-specific filtering needs.\nExample: 'anime-regex' for anime-specific release filtering"`
	// CfgRegex is a pointer to the RegexConfig for this regex
	CfgRegex *RegexConfig `toml:"-"`
	// TemplatePathNzb is the template to use for the nzb path
	TemplatePathNzb string `toml:"template_path_nzb" displayname:"NZB Path Template Name" comment:"Name of the path configuration template for storing NZB/torrent files from this indexer.\nReferences a path configuration defined in the paths section.\nThe NZB path template controls:\n- Directory where .nzb or .torrent files are saved\n- File naming and organization for download files\n- Cleanup and retention policies for download files\n- Access permissions and file handling\nMust exactly match the 'name' field of a paths configuration.\nUseful for organizing download files by indexer or quality.\nExample: 'nzb-storage' for centralized NZB file storage"`
	// CfgPath is a pointer to the PathsConfig for this path
	CfgPath *PathsConfig `toml:"-"`
	// CategoryDownloader is the category to use for the downloader
	CategoryDownloader string `toml:"category_downloader" displayname:"Download Category" comment:"Download category to assign to releases from this indexer.\nSpecifies which category the download client should use for organization.\nCategories help organize downloads and can trigger different post-processing.\nCommon categories:\n- 'movies', 'tv', 'anime' for content type organization\n- 'hd', '4k', 'sd' for quality-based organization\n- 'priority', 'bulk' for processing priority\nMust match categories configured in your download client.\nLeave empty to use the download client's default category.\nExample: 'movies-4k' for 4K movie downloads"`
	// AdditionalQueryParams are additional params to add to the indexer query string
	AdditionalQueryParams string `toml:"additional_query_params" displayname:"Additional Query Parameters" comment:"Additional URL parameters to append to indexer search queries.\nAllows customization of indexer-specific search options not covered by standard settings.\nParameters are appended to the search URL as-is, so include proper formatting.\nCommon examples:\n- '&extended=1' to enable extended search features\n- '&maxsize=1572864000' to set maximum file size (1.5GB in bytes)\n- '&minage=0&maxage=365' to set age limits in days\n- '&season=complete' for season pack preferences\nFormat: '&param1=value1&param2=value2' (include leading &)\nExample: '&extended=1&maxsize=5368709120' for extended search with 5GB size limit"`
	// SkipEmptySize indicates if releases with an empty size are allowed
	SkipEmptySize bool `toml:"skip_empty_size" displayname:"Skip Empty Size Releases" comment:"Skip releases that don't report a file size from this indexer.\nWhen true, releases without size information are ignored.\nWhen false, releases with missing size information are processed normally.\nMissing size info can indicate:\n- Indexer limitations or API issues\n- Fake or problematic releases\n- Freeleech torrents (some trackers)\nRecommended: true for Usenet indexers, false for torrent trackers.\nHelps filter out potentially problematic releases.\nDefault: false"`
	// HistoryCheckTitle indicates if the download history should check the title in addition to the url
	HistoryCheckTitle bool `toml:"history_check_title" displayname:"Check Title In History" comment:"Enable title-based duplicate checking in addition to URL-based checking.\nWhen true, both release URLs and titles are checked against download history.\nWhen false, only URLs are checked for duplicates (faster).\nTitle checking helps prevent:\n- Re-downloading same content from different URLs\n- Downloading reposts or mirrors of already grabbed releases\n- Processing renamed versions of already downloaded content\nUseful when indexers frequently change URLs or have multiple mirrors.\nMay increase processing time but improves duplicate detection accuracy.\nDefault: false"`
	// CategoriesIndexer are the categories to use for the indexer
	CategoriesIndexer string `toml:"categories_indexer" displayname:"Indexer Categories" comment:"Comma-separated list of indexer categories to search (no spaces).\nSpecifies which content categories on the indexer should be searched.\nCategories vary by indexer but commonly include:\n- Movies: 2000, 2010, 2020, 2030, 2040, 2045, 2050, 2060\n- TV: 5000, 5020, 5030, 5040, 5045, 5050, 5060, 5070\n- Anime: 5070 (TV), 2070 (Movies)\nCheck your indexer's category list for specific numbers.\nMore categories = broader search but more API calls and results.\nExample: '2000,2010,2020' for SD/HD/UHD movies\nExample: '5000,5020,5030' for SD/HD/UHD TV shows"`
}

type SchedulerConfig struct {
	// Name is the name of the template - see https://github.com/Kellerman81/go_media_downloader/wiki/Scheduler for details
	Name string `toml:"name" displayname:"Scheduler Template Name" comment:"Unique name for this scheduler configuration template.\nUsed to identify this scheduler in media group and list configurations.\nThe scheduler controls automated task timing and frequency:\n- Missing media search intervals\n- Quality upgrade scan schedules\n- RSS feed refresh timing\n- Database maintenance intervals\n- File scanning and import schedules\nChoose descriptive names that indicate the scheduling strategy:\n- 'aggressive' for frequent checks and fast discovery\n- 'conservative' for light resource usage\n- 'priority' for high-value content\n- 'bulk' for large collection management\nSee wiki for detailed scheduling options: https://github.com/Kellerman81/go_media_downloader/wiki/Scheduler\nExample: 'standard-schedule' or 'high-priority'"`

	// IntervalImdb is the interval for imdb scans
	IntervalImdb string `toml:"interval_imdb" displayname:"IMDB Update Interval" comment:"Time interval between IMDB database updates and metadata refreshes.\nControls how often IMDB data is synchronized and movie/series information is updated.\nSupports Go duration format: '1h30m', '2h', '45m', '24h', '2d'\nAlso supports cron-like format: '@daily', '@weekly', '@monthly'\nLonger intervals reduce server load but delay metadata updates.\nShorter intervals keep data fresh but increase resource usage.\nRecommended: '24h' for daily updates, '168h' for weekly\nExample: '24h' for daily IMDB updates"`

	// IntervalFeeds is the interval for rss feed scans
	IntervalFeeds string `toml:"interval_feeds" displayname:"RSS Feed Check Interval" comment:"Time interval between RSS feed checks for new releases.\nControls how often external RSS feeds are polled for new content.\nSupports Go duration format: '5m', '15m', '1h', '30m', '2d'\nAlso supports cron format: '*/15 * * * *' for every 15 minutes\nShorter intervals catch new releases faster but increase server load.\nLonger intervals reduce load but may miss time-sensitive releases.\nBalance based on feed update frequency and urgency needs.\nRecommended: '15m' to '1h' depending on feed activity\nExample: '30m' for moderate RSS feed monitoring"`

	// IntervalFeedsRefreshSeries is the interval for rss feed refreshes for series
	IntervalFeedsRefreshSeries string `toml:"interval_feeds_refresh_series" displayname:"Series Metadata Refresh Interval" comment:"Time interval for refreshing TV series metadata from RSS feeds.\nControls how often series information is updated from RSS sources.\nThis includes episode lists, season information, and series metadata.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nTV series change less frequently than movies, so longer intervals are typical.\nBalance between keeping episode data current and resource usage.\nRecommended: '12h' to '24h' for active series\nExample: '12h' for twice-daily series metadata refresh"`

	// IntervalFeedsRefreshMovies is the interval for rss feed refreshes for movies
	IntervalFeedsRefreshMovies string `toml:"interval_feeds_refresh_movies" displayname:"Movie Metadata Refresh Interval" comment:"Time interval for refreshing movie metadata from RSS feeds.\nControls how often movie information is updated from RSS sources.\nThis includes release dates, ratings, and movie metadata updates.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nMovies change less frequently after release, so longer intervals work well.\nBalance between metadata freshness and resource consumption.\nRecommended: '24h' to '48h' for movie metadata\nExample: '24h' for daily movie metadata refresh"`

	// IntervalFeedsRefreshSeriesFull is the interval for full rss feed refreshes for series
	IntervalFeedsRefreshSeriesFull string `toml:"interval_feeds_refresh_series_full" displayname:"Full Series Metadata Rebuild Interval" comment:"Time interval for complete TV series metadata rebuilds from RSS feeds.\nControls how often ALL series data is fully refreshed and rebuilt.\nThis is more comprehensive than regular refresh and rebuilds entire series records.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull refreshes are resource-intensive but ensure data consistency.\nShould be much less frequent than regular refreshes.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly full series refresh"`

	// IntervalFeedsRefreshMoviesFull is the interval for full rss feed refreshes for movies
	IntervalFeedsRefreshMoviesFull string `toml:"interval_feeds_refresh_movies_full" displayname:"Full Movie Metadata Rebuild Interval" comment:"Time interval for complete movie metadata rebuilds from RSS feeds.\nControls how often ALL movie data is fully refreshed and rebuilt.\nThis is more comprehensive than regular refresh and rebuilds entire movie records.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull refreshes are resource-intensive but ensure data consistency.\nShould be much less frequent than regular refreshes.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '720h' for monthly full movie refresh"`

	// IntervalIndexerMissing is the interval for missing media scans
	IntervalIndexerMissing string `toml:"interval_indexer_missing" displayname:"Missing Media Search Interval" comment:"Time interval between incremental searches for missing media.\nControls how often indexers are searched for media not yet in your library.\nThis is incremental scanning that processes a limited number of items per run.\nSupports Go duration format: '30m', '1h', '2h', '6h', '2d'\nAlso supports cron format for specific timing\nShorter intervals find new content faster but increase indexer API usage.\nLonger intervals reduce load but delay content discovery.\nRecommended: '1h' to '6h' depending on urgency and indexer limits\nExample: '2h' for moderate missing content discovery"`

	// IntervalIndexerUpgrade is the interval for upgrade media scans
	IntervalIndexerUpgrade string `toml:"interval_indexer_upgrade" displayname:"Quality Upgrade Search Interval" comment:"Time interval between incremental searches for media quality upgrades.\nControls how often indexers are searched for better quality versions of existing media.\nThis is incremental scanning that processes a limited number of items per run.\nSupports Go duration format: '2h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nUpgrade scans are typically less urgent than missing content searches.\nBalance between finding upgrades and conserving indexer API calls.\nRecommended: '6h' to '24h' depending on upgrade priority\nExample: '12h' for twice-daily upgrade scanning"`

	// IntervalIndexerMissingFull is the interval for full missing media scans
	IntervalIndexerMissingFull string `toml:"interval_indexer_missing_full" displayname:"Full Missing Media Scan Interval" comment:"Time interval between comprehensive searches for ALL missing media.\nControls how often a complete scan for missing content is performed.\nThis processes the entire library, not just incremental batches.\nSupports Go duration format: '24h', '48h', '168h' (1 week), '8d'\nAlso supports cron format for scheduled timing\nFull scans are resource-intensive and consume many indexer API calls.\nShould be much less frequent than incremental scans.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly comprehensive missing content scan"`

	// IntervalIndexerUpgradeFull is the interval for full upgrade media scans
	IntervalIndexerUpgradeFull string `toml:"interval_indexer_upgrade_full" displayname:"Full Upgrade Scan Interval" comment:"Time interval between comprehensive searches for ALL media upgrades.\nControls how often a complete scan for quality upgrades is performed.\nThis processes the entire library, not just incremental batches.\nSupports Go duration format: '48h', '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull upgrade scans are very resource-intensive and use many API calls.\nShould be infrequent as upgrades are less urgent than missing content.\nRecommended: '720h' (monthly) to '2160h' (quarterly)\nExample: '720h' for monthly comprehensive upgrade scanning"`

	// IntervalIndexerMissingTitle is the interval for missing media scans by title
	IntervalIndexerMissingTitle string `toml:"interval_indexer_missing_title" displayname:"Title Missing Search Interval" comment:"Time interval between incremental title-based searches for missing media.\nControls how often indexers are searched using media titles instead of IDs.\nUseful when ID-based searches fail or indexers have poor ID coverage.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nTitle searches are less accurate but broader than ID searches.\nShould be less frequent than ID-based searches due to accuracy concerns.\nRecommended: '12h' to '48h' depending on indexer ID coverage\nExample: '24h' for daily title-based missing content search"`

	// IntervalIndexerUpgradeTitle is the interval for upgrade media scans by title
	IntervalIndexerUpgradeTitle string `toml:"interval_indexer_upgrade_title" displayname:"Title Upgrade Search Interval" comment:"Time interval between incremental title-based searches for media upgrades.\nControls how often indexers are searched for upgrades using media titles instead of IDs.\nUseful when ID-based upgrade searches miss releases with poor tagging.\nSupports Go duration format: '12h', '24h', '48h', '168h', '2d'\nAlso supports cron format for specific timing\nTitle-based upgrade searches can be less precise than ID-based searches.\nShould be infrequent due to potential false positives.\nRecommended: '48h' to '168h' depending on upgrade needs\nExample: '48h' for twice-weekly title-based upgrade search"`

	// IntervalIndexerMissingFullTitle is the interval for full missing media scans
	IntervalIndexerMissingFullTitle string `toml:"interval_indexer_missing_full_title" displayname:"Full Title Missing Scan Interval" comment:"Time interval between comprehensive title-based searches for ALL missing media.\nControls how often complete title-based missing content scans are performed.\nProcesses entire library using titles when ID searches are insufficient.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull title scans are very resource-intensive and less accurate than ID scans.\nUse sparingly due to potential false positives and high API usage.\nRecommended: '720h' (monthly) to '2160h' (quarterly)\nExample: '720h' for monthly full title-based missing scan"`
	// IntervalIndexerUpgradeFullTitle is the interval for full upgrade media scans
	IntervalIndexerUpgradeFullTitle string `toml:"interval_indexer_upgrade_full_title" displayname:"Full Title Upgrade Scan Interval" comment:"Time interval between comprehensive title-based searches for ALL media upgrades.\nControls how often complete title-based upgrade scans are performed.\nProcesses entire library using titles when ID-based upgrade searches are insufficient.\nSupports Go duration format: '720h' (1 month), '2160h' (3 months), '8d'\nAlso supports cron format for scheduled timing\nFull title upgrade scans have high false positive risk and massive API usage.\nShould be very infrequent due to accuracy and resource concerns.\nRecommended: '2160h' (quarterly) or disable entirely\nExample: '2160h' for quarterly full title-based upgrade scan"`
	// IntervalIndexerRss is the interval for rss feed scans
	IntervalIndexerRss string `toml:"interval_indexer_rss" displayname:"Indexer RSS Feed Interval" comment:"Time interval between indexer-specific RSS feed checks.\nControls how often each indexer's RSS feeds are polled for new releases.\nDifferent from general feed scanning, this is indexer-focused RSS monitoring.\nSupports Go duration format: '5m', '15m', '30m', '1h', '2d'\nAlso supports cron format for specific timing\nIndexer RSS feeds often update frequently with new releases.\nShorter intervals catch releases faster but increase API usage.\nRecommended: '15m' to '1h' depending on indexer feed activity\nExample: '30m' for half-hourly indexer RSS monitoring"`
	// IntervalScanData is the interval for data scans
	IntervalScanData string `toml:"interval_scan_data" displayname:"Filesystem Scan Interval" comment:"Time interval between filesystem scans for media file changes.\nControls how often storage paths are scanned for new, moved, or deleted files.\nThis maintains synchronization between filesystem and database records.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nFrequent scans keep database current but increase I/O load.\nBalance based on how often files are added/moved externally.\nRecommended: '6h' to '24h' depending on library activity\nExample: '12h' for twice-daily filesystem scanning"`
	// IntervalScanDataMissing is the interval for missing data scans
	IntervalScanDataMissing string `toml:"interval_scan_data_missing" displayname:"Missing File Scan Interval" comment:"Time interval between scans for media files that should exist but are missing.\nControls how often the system checks for files that are tracked but no longer on disk.\nHelps identify moved, deleted, or corrupted media files.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nMissing file detection helps maintain library integrity.\nFrequent scans catch issues faster but increase I/O overhead.\nRecommended: '24h' to '48h' for missing file detection\nExample: '24h' for daily missing file verification"`
	// IntervalScanDataFlags is the interval for flagged data scans
	IntervalScanDataFlags string `toml:"interval_scan_data_flags" displayname:"Flagged File Scan Interval" comment:"Time interval between scans for media files marked with processing flags.\nControls how often files flagged for reprocessing, upgrading, or fixing are handled.\nFlags indicate files needing attention (corrupt, misnamed, quality issues).\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nFlagged files often need prompt attention to resolve issues.\nShorter intervals resolve problems faster but increase processing load.\nRecommended: '6h' to '12h' for flagged file processing\nExample: '6h' for four-times-daily flagged file handling"`
	// IntervalScanDataimport is the interval for data import scans
	IntervalScanDataimport string `toml:"interval_scan_data_import" displayname:"Import Directory Scan Interval" comment:"Time interval between scans for new media files to import from configured import paths.\nControls how often import directories are scanned for existing media to add to library.\nUseful for gradually importing large existing collections.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nImport scanning processes external media for library integration.\nFrequency depends on how often new files are added to import paths.\nRecommended: '12h' to '24h' for import directory monitoring\nExample: '12h' for twice-daily import scanning"`
	// IntervalDatabaseBackup is the interval for database backups
	IntervalDatabaseBackup string `toml:"interval_database_backup" displayname:"Database Backup Interval" comment:"Time interval between automatic database backup operations.\nControls how often the application database is backed up for safety.\nBackups protect against data loss from corruption or system failures.\nSupports Go duration format: '24h', '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for specific timing (e.g., daily at 3 AM)\nDatabase backups temporarily lock the database during operation.\nBalance between data protection and system performance impact.\nRecommended: '24h' for daily backups, '168h' for weekly\nExample: '24h' for daily database backup at configured time"`
	// IntervalDatabaseCheck is the interval for database checks
	IntervalDatabaseCheck string `toml:"interval_database_check" displayname:"Database Check Interval" comment:"Time interval between database integrity and consistency checks.\nControls how often the database is examined for corruption or inconsistencies.\nChecks help identify and repair database issues before they cause problems.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for specific timing\nDatabase checks can be resource-intensive and temporarily slow operations.\nInfrequent checks balance integrity monitoring with performance impact.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly database integrity verification"`
	// IntervalIndexerRssSeasons is the interval for rss feed season scans
	IntervalIndexerRssSeasons string `toml:"interval_indexer_rss_seasons" displayname:"Season Pack RSS Interval" comment:"Time interval between RSS feed checks specifically for TV season packs and batches.\nControls how often indexer RSS feeds are scanned for complete season releases.\nSeason packs provide entire TV seasons in single downloads.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nSeason releases are less frequent but valuable for batch downloading.\nBalance between catching season packs and API usage.\nRecommended: '6h' to '24h' depending on season pack priority\nExample: '12h' for twice-daily season pack monitoring"`
	// IntervalIndexerRssSeasonsAll is the interval for rss feed all season scans
	IntervalIndexerRssSeasonsAll string `toml:"interval_indexer_rss_seasons_all" displayname:"Full Season RSS Scan Interval" comment:"Time interval between comprehensive RSS feed scans for ALL available season packs.\nControls how often complete season pack catalogs are refreshed from indexer feeds.\nThis processes all available seasons, not just new releases.\nSupports Go duration format: '24h', '48h', '168h' (1 week), '2d'\nAlso supports cron format for specific timing\nComprehensive season scanning is resource-intensive.\nShould be much less frequent than regular season monitoring.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly comprehensive season catalog refresh"`
	// CronIndexerRssSeasonsAll is the cron schedule for rss feed all season scans
	CronIndexerRssSeasonsAll string `toml:"cron_indexer_rss_seasons_all" displayname:"Full Season RSS Cron Schedule" comment:"Cron schedule for comprehensive RSS season pack scans (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 3 * * 1': Every Monday at 3 AM\n- '0 2 */7 * *': Every 7 days at 2 AM\n- '30 1 1 * *': First day of each month at 1:30 AM\nCron scheduling provides better control than intervals for resource management.\nUseful for scheduling intensive scans during low-usage periods.\nExample: '0 2 * * 0' for Sunday at 2 AM weekly scan"`
	// CronIndexerRssSeasons is the cron schedule for rss feed season scans
	CronIndexerRssSeasons string `toml:"cron_indexer_rss_seasons" displayname:"Season Pack RSS Cron Schedule" comment:"Cron schedule for RSS season pack monitoring (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 8,20 * * *': Daily at 8 AM and 8 PM\n- '15 */4 * * *': Every 4 hours at 15 minutes past\nAllows scheduling season checks at optimal times.\nUseful for avoiding peak indexer usage periods.\nExample: '0 */8 * * *' for every 8 hours season monitoring"`
	// CronImdb is the cron schedule for imdb scans
	CronImdb string `toml:"cron_imdb" displayname:"IMDB Update Cron Schedule" comment:"Cron schedule for IMDB database updates (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * *': Daily at 4 AM\n- '0 2 * * 1': Weekly on Monday at 2 AM\n- '0 3 1 * *': Monthly on first day at 3 AM\nIMDB updates are resource-intensive, best scheduled during low-usage periods.\nAllows coordination with IMDB's data release schedule.\nExample: '0 3 * * *' for daily IMDB updates at 3 AM"`
	// CronFeeds is the cron schedule for rss feed scans
	CronFeeds string `toml:"cron_feeds" displayname:"RSS Feed Cron Schedule" comment:"Cron schedule for RSS feed monitoring (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '*/15 * * * *': Every 15 minutes\n- '0,30 * * * *': Every 30 minutes (at 0 and 30)\n- '*/10 * * * *': Every 10 minutes\nRSS feeds update frequently, so shorter intervals are common.\nCron allows avoiding specific times when feeds might be busy.\nExample: '*/20 * * * *' for every 20 minutes RSS monitoring"`

	// CronFeedsRefreshSeries is the cron schedule for refreshing series RSS feeds
	CronFeedsRefreshSeries string `toml:"cron_feeds_refresh_series" displayname:"Series Metadata Cron Schedule" comment:"Cron schedule for TV series RSS feed metadata refreshes (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */12 * * *': Every 12 hours\n- '0 6,18 * * *': Daily at 6 AM and 6 PM\n- '0 8 * * *': Daily at 8 AM\nSeries metadata changes less frequently than new releases.\nSchedule during periods when metadata sources are most current.\nExample: '0 */12 * * *' for twice-daily series metadata refresh"`
	// CronFeedsRefreshMovies is the cron schedule for refreshing movie RSS feeds
	CronFeedsRefreshMovies string `toml:"cron_feeds_refresh_movies" displayname:"Movie Metadata Cron Schedule" comment:"Cron schedule for movie RSS feed metadata refreshes (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * *': Daily at 4 AM\n- '0 2 * * 1,4': Twice weekly on Monday and Thursday at 2 AM\n- '0 6 */2 * *': Every other day at 6 AM\nMovie metadata updates are less frequent than series.\nSchedule when movie databases typically update.\nExample: '0 4 * * *' for daily movie metadata refresh at 4 AM"`
	// CronFeedsRefreshSeriesFull is the cron schedule for full refreshing of series RSS feeds
	CronFeedsRefreshSeriesFull string `toml:"cron_feeds_refresh_series_full" displayname:"Full Series Rebuild Cron Schedule" comment:"Cron schedule for complete TV series metadata rebuilds (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 1 * * 0': Weekly on Sunday at 1 AM\n- '0 2 1 * *': Monthly on first day at 2 AM\n- '0 3 */14 * *': Every 14 days at 3 AM\nFull refreshes are resource-intensive and should be infrequent.\nSchedule during lowest usage periods for minimal impact.\nExample: '0 1 * * 0' for weekly full series refresh on Sunday"`
	// CronFeedsRefreshMoviesFull is the cron schedule for full refreshing of movie RSS feeds
	CronFeedsRefreshMoviesFull string `toml:"cron_feeds_refresh_movies_full" displayname:"Full Movie Rebuild Cron Schedule" comment:"Cron schedule for complete movie metadata rebuilds (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 2 * * 0': Weekly on Sunday at 2 AM\n- '0 3 1 * *': Monthly on first day at 3 AM\n- '0 1 1 */3 *': Quarterly on first day at 1 AM\nFull movie refreshes are very resource-intensive.\nSchedule during absolute lowest usage periods.\nExample: '0 2 1 * *' for monthly full movie refresh on first day"`
	// CronIndexerMissing is the cron schedule for missing media scans
	CronIndexerMissing string `toml:"cron_indexer_missing" displayname:"Missing Media Cron Schedule" comment:"Cron schedule for incremental missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */2 * * *': Every 2 hours\n- '0 8,14,20 * * *': Three times daily at 8 AM, 2 PM, 8 PM\n- '*/30 * * * *': Every 30 minutes\nAllows scheduling searches during indexer low-traffic periods.\nAvoid peak hours when indexers may be slow or limited.\nExample: '0 */3 * * *' for every 3 hours missing content search"`
	// CronIndexerUpgrade is the cron schedule for upgrade media scans
	CronIndexerUpgrade string `toml:"cron_indexer_upgrade" displayname:"Quality Upgrade Cron Schedule" comment:"Cron schedule for incremental media quality upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 10,22 * * *': Daily at 10 AM and 10 PM\n- '0 14 * * *': Daily at 2 PM\nUpgrade searches are less urgent than missing content searches.\nSchedule during moderate usage periods when indexers are responsive.\nExample: '0 */8 * * *' for every 8 hours upgrade search"`
	// CronIndexerMissingFull is the cron schedule for full missing media scans
	CronIndexerMissingFull string `toml:"cron_indexer_missing_full" displayname:"Full Missing Scan Cron Schedule" comment:"Cron schedule for comprehensive missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 2 * * 0': Weekly on Sunday at 2 AM\n- '0 1 1 * *': Monthly on first day at 1 AM\n- '0 3 */7 * *': Every 7 days at 3 AM\nFull missing scans consume massive indexer API calls.\nSchedule very infrequently during absolute lowest usage periods.\nExample: '0 2 * * 0' for weekly comprehensive missing scan on Sunday"`
	// CronIndexerUpgradeFull is the cron schedule for full upgrade media scans
	CronIndexerUpgradeFull string `toml:"cron_indexer_upgrade_full" displayname:"Full Upgrade Scan Cron Schedule" comment:"Cron schedule for comprehensive media upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 3 1 * *': Monthly on first day at 3 AM\n- '0 4 1 */3 *': Quarterly on first day at 4 AM\n- '0 2 15 * *': Monthly on 15th day at 2 AM\nFull upgrade scans are extremely resource-intensive.\nShould be very infrequent due to massive API usage.\nExample: '0 3 1 * *' for monthly comprehensive upgrade scan"`
	// CronIndexerMissingTitle is the cron schedule for missing media scans by title
	CronIndexerMissingTitle string `toml:"cron_indexer_missing_title" displayname:"Title Missing Cron Schedule" comment:"Cron schedule for incremental title-based missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 12 * * *': Daily at noon\n- '0 6 * * 1,4': Twice weekly on Monday and Thursday at 6 AM\n- '0 18 */2 * *': Every other day at 6 PM\nTitle searches are less accurate but broader than ID searches.\nSchedule less frequently than ID searches due to accuracy concerns.\nExample: '0 12 * * *' for daily title-based missing search at noon"`
	// CronIndexerUpgradeTitle is the cron schedule for upgrade media scans by title
	CronIndexerUpgradeTitle string `toml:"cron_indexer_upgrade_title" displayname:"Title Upgrade Cron Schedule" comment:"Cron schedule for incremental title-based upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 16 * * *': Daily at 4 PM\n- '0 9 * * 2,5': Twice weekly on Tuesday and Friday at 9 AM\n- '0 20 */3 * *': Every 3 days at 8 PM\nTitle-based upgrade searches can be less precise than ID-based searches.\nSchedule infrequently due to potential false positives.\nExample: '0 16 */2 * *' for every other day title-based upgrade search"`

	// CronIndexerMissingFullTitle is the cron schedule for full missing media scans
	CronIndexerMissingFullTitle string `toml:"cron_indexer_missing_full_title" displayname:"Full Title Missing Cron Schedule" comment:"Cron schedule for comprehensive title-based missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 1 * *': Monthly on first day at 4 AM\n- '0 5 1 */3 *': Quarterly on first day at 5 AM\n- '0 3 15 * *': Monthly on 15th day at 3 AM\nFull title scans are very resource-intensive and less accurate than ID scans.\nUse sparingly due to potential false positives and high API usage.\nExample: '0 4 1 * *' for monthly full title-based missing scan"`
	// CronIndexerUpgradeFullTitle is the cron schedule for full upgrade media scans
	CronIndexerUpgradeFullTitle string `toml:"cron_indexer_upgrade_full_title" displayname:"Full Title Upgrade Cron Schedule" comment:"Cron schedule for comprehensive title-based upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 5 1 */3 *': Quarterly on first day at 5 AM\n- '0 6 1 */6 *': Twice yearly on first day at 6 AM\n- 'disabled': Consider disabling due to high false positive risk\nFull title upgrade scans have high false positive risk and massive API usage.\nShould be very infrequent or disabled entirely.\nExample: '0 5 1 */6 *' for semi-annual full title upgrade scan (or disable)"`
	// CronIndexerRss is the cron schedule for rss feed scans
	CronIndexerRss string `toml:"cron_indexer_rss" displayname:"Indexer RSS Cron Schedule" comment:"Cron schedule for indexer-specific RSS feed monitoring (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '*/30 * * * *': Every 30 minutes\n- '*/15 * * * *': Every 15 minutes\n- '0,20,40 * * * *': Every 20 minutes (at 0, 20, 40)\nIndexer RSS feeds often update frequently with new releases.\nBalance between catching releases quickly and API rate limits.\nExample: '*/20 * * * *' for every 20 minutes indexer RSS monitoring"`
	// CronScanData is the cron schedule for data scans
	CronScanData string `toml:"cron_scan_data" displayname:"Filesystem Scan Cron Schedule" comment:"Cron schedule for filesystem media file scanning (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 8,20 * * *': Daily at 8 AM and 8 PM\n- '0 12 * * *': Daily at noon\nFilesystem scans maintain synchronization between disk and database.\nSchedule based on how frequently files are added/moved externally.\nExample: '0 */8 * * *' for every 8 hours filesystem scanning"`
	// CronScanDataMissing is the cron schedule for missing data scans
	CronScanDataMissing string `toml:"cron_scan_data_missing" displayname:"Missing File Cron Schedule" comment:"Cron schedule for missing media file detection scans (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 6 * * *': Daily at 6 AM\n- '0 4 * * 0': Weekly on Sunday at 4 AM\n- '0 5 */2 * *': Every other day at 5 AM\nMissing file detection helps maintain library integrity.\nSchedule during low-usage periods due to intensive I/O operations.\nExample: '0 6 * * *' for daily missing file verification at 6 AM"`
	// CronScanDataFlags is the cron schedule for flagged data scans
	CronScanDataFlags string `toml:"cron_scan_data_flags" displayname:"Flagged File Cron Schedule" comment:"Cron schedule for flagged media file processing (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */4 * * *': Every 4 hours\n- '0 9,15,21 * * *': Three times daily at 9 AM, 3 PM, 9 PM\n- '*/45 * * * *': Every 45 minutes\nFlagged files often need prompt attention to resolve issues.\nSchedule frequently enough to handle problems quickly.\nExample: '0 */6 * * *' for every 6 hours flagged file processing"`
	// CronScanDataimport is the cron schedule for data import scans
	CronScanDataimport string `toml:"cron_scan_data_import" displayname:"Import Directory Cron Schedule" comment:"Cron schedule for media import directory scanning (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */8 * * *': Every 8 hours\n- '0 10,22 * * *': Daily at 10 AM and 10 PM\n- '0 14 * * *': Daily at 2 PM\nImport scanning processes external media for library integration.\nFrequency depends on how often new files are added to import paths.\nExample: '0 */12 * * *' for every 12 hours import directory scanning"`
	// CronDatabaseBackup is the cron schedule for database backups
	CronDatabaseBackup string `toml:"cron_database_backup" displayname:"Database Backup Cron Schedule" comment:"Cron schedule for automatic database backup operations (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 3 * * *': Daily at 3 AM\n- '0 2 * * 0': Weekly on Sunday at 2 AM\n- '0 1 1 * *': Monthly on first day at 1 AM\nDatabase backups temporarily lock database during operation.\nSchedule during absolute lowest system usage periods.\nExample: '0 3 * * *' for daily database backup at 3 AM"`
	// CronDatabaseCheck is the cron schedule for database checks
	CronDatabaseCheck string `toml:"cron_database_check" displayname:"Database Check Cron Schedule" comment:"Cron schedule for database integrity and consistency checks (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * 0': Weekly on Sunday at 4 AM\n- '0 5 1 * *': Monthly on first day at 5 AM\n- '0 6 1 */3 *': Quarterly on first day at 6 AM\nDatabase checks are resource-intensive and can slow operations.\nSchedule during lowest usage periods for minimal impact.\nExample: '0 4 * * 0' for weekly database integrity check on Sunday"`
}

const Configfile = "./config/config.toml"

var (
	settings struct {
		// SettingsGeneral contains the general configuration settings.
		SettingsGeneral *GeneralConfig

		// SettingsImdb contains the IMDB specific configuration.
		SettingsImdb *ImdbConfig

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

		// settings.cachetoml contains the cached TOML configuration.
		cachetoml MainConfig
	}

	// traktToken contains the trakt OAuth token.
	traktToken *oauth2.Token

	mu = sync.Mutex{}
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
	mu.Lock()
	defer mu.Unlock()
	if str == "" {
		return settings.SettingsQuality[cfgp.DefaultQuality]
	}
	q, ok := settings.SettingsQuality[str]
	if ok {
		return q
	}
	return settings.SettingsQuality[cfgp.DefaultQuality]
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
	mu.Lock()
	defer mu.Unlock()
	for _, listcfg := range settings.SettingsList {
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
func LoadCfgDB(reload bool) error {
	if _, err := os.Stat(Configfile); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Config file not found. Creating new config file.")
		ClearCfg()
		WriteCfg()
		fmt.Println("Config file created. Please edit it and run the application again.")
	} else {
		fmt.Println("Config file found. Loading config.")
	}

	if err := Loadallsettings(reload); err != nil {
		return err
	}

	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{SyncInterval: 0})
	defer configDB.Close()
	pudge.BackupAll("")
	hastoken, _ := configDB.Has("trakt_token")
	if hastoken {
		var token oauth2.Token
		if configDB.Get("trakt_token", &token) == nil {
			traktToken = &token
		}
	}

	return nil
}

// Loadallsettings loads all configuration settings from the TOML configuration file.
// It performs a thread-safe reload of configuration by reading the TOML file,
// clearing existing settings, and repopulating them with fresh data.
// The reload parameter controls whether scheduler settings are preserved during reload.
func Loadallsettings(reload bool) error {
	mu.Lock()
	defer mu.Unlock()
	err := Readconfigtoml()
	if err != nil {
		return err
	}
	ClearSettings(reload)
	Getconfigtoml(reload)
	return nil
}

// Readconfigtoml reads and decodes the configuration file specified by Configfile into the global settings.cachetoml struct.
// It opens the configuration file, uses a TOML decoder to parse its contents, and handles any potential errors.
// Returns an error if the file cannot be opened or decoded, otherwise returns nil.
func Readconfigtoml() error {
	content, err := os.Open(Configfile)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
		return err
	}
	defer content.Close()
	err = toml.NewDecoder(content).Decode(&settings.cachetoml)
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
func ClearSettings(reload bool) {
	settings.SettingsDownloader = make(map[string]*DownloaderConfig, len(settings.cachetoml.Downloader))
	settings.SettingsIndexer = make(map[string]*IndexersConfig, len(settings.cachetoml.Indexers))
	settings.SettingsList = make(map[string]*ListsConfig, len(settings.cachetoml.Lists))
	settings.SettingsMedia = make(map[string]*MediaTypeConfig)
	settings.SettingsNotification = make(map[string]*NotificationConfig, len(settings.cachetoml.Notification))
	settings.SettingsPath = make(map[string]*PathsConfig, len(settings.cachetoml.Paths))
	settings.SettingsQuality = make(map[string]*QualityConfig, len(settings.cachetoml.Quality))
	settings.SettingsRegex = make(map[string]*RegexConfig, len(settings.cachetoml.Regex))
	if !reload {
		settings.SettingsScheduler = make(map[string]*SchedulerConfig, len(settings.cachetoml.Scheduler))
	}
}

// Getconfigtoml populates global configuration settings from the cached TOML configuration.
// It sets default values, initializes various configuration maps, and processes configuration
// for different components such as general settings, downloaders, indexers, lists, media types,
// notifications, paths, quality, regex, and schedulers. This function prepares the application's
// configuration by linking and transforming configuration data from the parsed TOML file.
func Getconfigtoml(reload bool) {
	settings.SettingsGeneral = &settings.cachetoml.General
	if settings.SettingsGeneral.CacheDuration == 0 {
		settings.SettingsGeneral.CacheDuration = 12
	}
	settings.SettingsGeneral.CacheDuration2 = 2 * settings.SettingsGeneral.CacheDuration

	if len(settings.SettingsGeneral.MovieMetaSourcePriority) == 0 {
		settings.SettingsGeneral.MovieMetaSourcePriority = []string{"imdb", "tmdb", "omdb", "trakt"}
	}

	settings.SettingsImdb = &settings.cachetoml.Imdbindexer

	setupSimpleConfigMaps()
	setupPathConfigs()
	setupRegexConfigs()
	setupSchedulerConfigs(reload)
	setupQualityConfigs()
	for idx := range settings.cachetoml.Media.Movies {
		setupMediaTypeConfig(&settings.cachetoml.Media.Movies[idx], "movie_", false)
		setupMediaConfigLists(&settings.cachetoml.Media.Movies[idx])
		settings.SettingsMedia["movie_"+settings.cachetoml.Media.Movies[idx].Name] = &settings.cachetoml.Media.Movies[idx]
	}
	for idx := range settings.cachetoml.Media.Series {
		setupMediaTypeConfig(&settings.cachetoml.Media.Series[idx], "serie_", true)
		setupMediaConfigLists(&settings.cachetoml.Media.Series[idx])
		settings.SettingsMedia["serie_"+settings.cachetoml.Media.Series[idx].Name] = &settings.cachetoml.Media.Series[idx]
	}
}

// handleConfigEntry processes a single config entry based on its name prefix
func handleConfigEntry(val Conf) {
	switch {
	case strings.HasPrefix(val.Name, "general"):
		data := val.Data.(GeneralConfig)
		settings.SettingsGeneral = &data
	case strings.HasPrefix(val.Name, "downloader_"):
		data := val.Data.(DownloaderConfig)
		settings.SettingsDownloader[val.Data.(DownloaderConfig).Name] = &data
	case strings.HasPrefix(val.Name, logger.StrImdb):
		data := val.Data.(ImdbConfig)
		settings.SettingsImdb = &data
	case strings.HasPrefix(val.Name, "indexer"):
		data := val.Data.(IndexersConfig)
		settings.SettingsIndexer[val.Data.(IndexersConfig).Name] = &data
	case strings.HasPrefix(val.Name, "list"):
		data := val.Data.(ListsConfig)
		settings.SettingsList[val.Data.(ListsConfig).Name] = &data
	case strings.HasPrefix(val.Name, logger.StrSerie):
		data := val.Data.(MediaTypeConfig)
		settings.SettingsMedia["serie_"+val.Data.(MediaTypeConfig).Name] = &data
	case strings.HasPrefix(val.Name, logger.StrMovie):
		data := val.Data.(MediaTypeConfig)
		settings.SettingsMedia["movie_"+val.Data.(MediaTypeConfig).Name] = &data
	case strings.HasPrefix(val.Name, "notification"):
		data := val.Data.(NotificationConfig)
		settings.SettingsNotification[val.Data.(NotificationConfig).Name] = &data
	case strings.HasPrefix(val.Name, "path"):
		data := val.Data.(PathsConfig)
		settings.SettingsPath[val.Data.(PathsConfig).Name] = &data
	case strings.HasPrefix(val.Name, "quality"):
		data := val.Data.(QualityConfig)
		settings.SettingsQuality[val.Data.(QualityConfig).Name] = &data
	case strings.HasPrefix(val.Name, "regex"):
		data := val.Data.(RegexConfig)
		settings.SettingsRegex[val.Data.(RegexConfig).Name] = &data
	case strings.HasPrefix(val.Name, "scheduler"):
		data := val.Data.(SchedulerConfig)
		settings.SettingsScheduler[val.Data.(SchedulerConfig).Name] = &data
	}
}

// handleConfigEntryWithDB processes a single config entry and handles trakt_token special case
func handleConfigEntryWithDB(configIn Conf, configDB *pudge.Db) {
	handleConfigEntry(configIn)

	// Handle special case for trakt_token
	if strings.HasPrefix(configIn.Name, "trakt_token") {
		traktToken = configIn.Data.(*oauth2.Token)
		configDB.Set("trakt_token", *configIn.Data.(*oauth2.Token))
	}
}

// handleConfigDeletion deletes a config entry based on its name prefix
func handleConfigDeletion(name string) {
	switch {
	case strings.HasPrefix(name, "general"):
		settings.SettingsGeneral = &GeneralConfig{}
	case strings.HasPrefix(name, "downloader_"):
		delete(settings.SettingsDownloader, name)
	case strings.HasPrefix(name, logger.StrImdb):
		settings.SettingsImdb = &ImdbConfig{}
	case strings.HasPrefix(name, "indexer"):
		delete(settings.SettingsIndexer, name)
	case strings.HasPrefix(name, "list"):
		delete(settings.SettingsList, name)
	case strings.HasPrefix(name, logger.StrSerie):
		delete(settings.SettingsMedia, name)
	case strings.HasPrefix(name, logger.StrMovie):
		delete(settings.SettingsMedia, name)
	case strings.HasPrefix(name, "notification"):
		delete(settings.SettingsNotification, name)
	case strings.HasPrefix(name, "path"):
		delete(settings.SettingsPath, name)
	case strings.HasPrefix(name, "quality"):
		delete(settings.SettingsQuality, name)
	case strings.HasPrefix(name, "regex"):
		delete(settings.SettingsRegex, name)
	case strings.HasPrefix(name, "scheduler"):
		delete(settings.SettingsScheduler, name)
	}
}

// handleConfigDeletionWithDB deletes a config entry and handles trakt_token special case
func handleConfigDeletionWithDB(name string, configDB *pudge.Db) {
	handleConfigDeletion(name)

	// Handle special case for trakt_token
	if strings.HasPrefix(name, "trakt_token") {
		configDB.Delete("trakt_token")
	}
}

// setupMediaTypeConfig initializes common configuration for a media type config
func setupMediaTypeConfig(mediaConfig *MediaTypeConfig, prefix string, isSeriesType bool) {
	// Initialize maps
	mediaConfig.DataMap = make(map[int]*MediaDataConfig, len(mediaConfig.Data))
	mediaConfig.DataImportMap = make(map[int]*MediaDataImportConfig, len(mediaConfig.DataImport))

	// Setup Data configs
	for idx2 := range mediaConfig.Data {
		mediaConfig.Data[idx2].CfgPath = settings.SettingsPath[mediaConfig.Data[idx2].TemplatePath]
		if !isSeriesType && mediaConfig.Data[idx2].AddFoundList != "" {
			mediaConfig.Data[idx2].AddFoundListCfg = settings.SettingsList[mediaConfig.Data[idx2].AddFoundList]
		}
		mediaConfig.DataMap[idx2] = &mediaConfig.Data[idx2]
	}

	// Setup DataImport configs
	for idx2 := range mediaConfig.DataImport {
		mediaConfig.DataImport[idx2].CfgPath = settings.SettingsPath[mediaConfig.DataImport[idx2].TemplatePath]
		mediaConfig.DataImportMap[idx2] = &mediaConfig.DataImport[idx2]
	}

	// Setup Notification configs
	for idx2 := range mediaConfig.Notification {
		mediaConfig.Notification[idx2].CfgNotification = settings.SettingsNotification[mediaConfig.Notification[idx2].MapNotification]
	}

	// Setup main configs
	mediaConfig.CfgQuality = settings.SettingsQuality[mediaConfig.TemplateQuality]
	mediaConfig.CfgScheduler = settings.SettingsScheduler[mediaConfig.TemplateScheduler]
	mediaConfig.NamePrefix = prefix + mediaConfig.Name
	mediaConfig.Useseries = isSeriesType

	// Setup Lists maps and related fields
	mediaConfig.ListsMap = make(map[string]*MediaListsConfig, len(mediaConfig.Lists))
	mediaConfig.ListsMapIdx = make(map[string]int, len(mediaConfig.Lists))
	if len(mediaConfig.Lists) >= 1 {
		mediaConfig.ListsQu = strings.Repeat(",?", len(mediaConfig.Lists)-1)
	}
	mediaConfig.ListsLen = len(mediaConfig.Lists)
	mediaConfig.MetadataTitleLanguagesLen = len(mediaConfig.MetadataTitleLanguages)
	mediaConfig.DataLen = len(mediaConfig.Data)
	mediaConfig.ListsQualities = make([]string, 0, len(mediaConfig.Lists))
}

// setupMediaListConfig initializes configuration for a media list
func setupMediaListConfig(listConfig *MediaListsConfig, mediaConfig *MediaTypeConfig, idxsub int) {
	listConfig.CfgList = settings.SettingsList[listConfig.TemplateList]
	listConfig.CfgQuality = settings.SettingsQuality[listConfig.TemplateQuality]
	listConfig.CfgScheduler = settings.SettingsScheduler[listConfig.TemplateScheduler]

	if len(listConfig.IgnoreMapLists) >= 1 {
		listConfig.IgnoreMapListsQu = strings.Repeat(",?", len(listConfig.IgnoreMapLists)-1)
	}
	listConfig.IgnoreMapListsLen = len(listConfig.IgnoreMapLists)
	listConfig.ReplaceMapListsLen = len(listConfig.ReplaceMapLists)

	// Add quality to ListsQualities if not already present
	if !slices.Contains(mediaConfig.ListsQualities, listConfig.TemplateQuality) {
		mediaConfig.ListsQualities = append(mediaConfig.ListsQualities, listConfig.TemplateQuality)
	}

	// Setup maps
	mediaConfig.ListsMap[listConfig.Name] = listConfig
	mediaConfig.ListsMapIdx[listConfig.Name] = idxsub
}

// setupMediaConfigLists processes all lists for a media configuration
func setupMediaConfigLists(mediaConfig *MediaTypeConfig) {
	for idxsub := range mediaConfig.Lists {
		setupMediaListConfig(&mediaConfig.Lists[idxsub], mediaConfig, idxsub)
	}
}

// setupSimpleConfigMaps sets up basic configuration mappings
func setupSimpleConfigMaps() {
	// Setup Downloader configs
	for idx := range settings.cachetoml.Downloader {
		settings.SettingsDownloader[settings.cachetoml.Downloader[idx].Name] = &settings.cachetoml.Downloader[idx]
	}

	// Setup Indexer configs with additional string conversion
	for idx := range settings.cachetoml.Indexers {
		settings.cachetoml.Indexers[idx].MaxEntriesStr = logger.IntToString(settings.cachetoml.Indexers[idx].MaxEntries)
		settings.SettingsIndexer[settings.cachetoml.Indexers[idx].Name] = &settings.cachetoml.Indexers[idx]
	}

	// Setup Lists configs with length calculations
	for idx := range settings.cachetoml.Lists {
		settings.cachetoml.Lists[idx].ExcludegenreLen = len(settings.cachetoml.Lists[idx].Excludegenre)
		settings.cachetoml.Lists[idx].IncludegenreLen = len(settings.cachetoml.Lists[idx].Includegenre)
		settings.SettingsList[settings.cachetoml.Lists[idx].Name] = &settings.cachetoml.Lists[idx]
	}

	// Setup Notification configs
	for idx := range settings.cachetoml.Notification {
		settings.SettingsNotification[settings.cachetoml.Notification[idx].Name] = &settings.cachetoml.Notification[idx]
	}

	// Setup Regex configs with length calculations
	for idx := range settings.cachetoml.Regex {
		settings.cachetoml.Regex[idx].RejectedLen = len(settings.cachetoml.Regex[idx].Rejected)
		settings.cachetoml.Regex[idx].RequiredLen = len(settings.cachetoml.Regex[idx].Required)
		settings.SettingsRegex[settings.cachetoml.Regex[idx].Name] = &settings.cachetoml.Regex[idx]
	}
}

// setupPathConfigs sets up Path configurations with complex length calculations
func setupPathConfigs() {
	for idx := range settings.cachetoml.Paths {
		settings.cachetoml.Paths[idx].AllowedLanguagesLen = len(settings.cachetoml.Paths[idx].AllowedLanguages)
		settings.cachetoml.Paths[idx].AllowedOtherExtensionsLen = len(settings.cachetoml.Paths[idx].AllowedOtherExtensions)
		settings.cachetoml.Paths[idx].AllowedOtherExtensionsNoRenameLen = len(settings.cachetoml.Paths[idx].AllowedOtherExtensionsNoRename)
		settings.cachetoml.Paths[idx].AllowedVideoExtensionsLen = len(settings.cachetoml.Paths[idx].AllowedVideoExtensions)
		settings.cachetoml.Paths[idx].AllowedVideoExtensionsNoRenameLen = len(settings.cachetoml.Paths[idx].AllowedVideoExtensionsNoRename)
		settings.cachetoml.Paths[idx].BlockedLen = len(settings.cachetoml.Paths[idx].Blocked)
		settings.cachetoml.Paths[idx].DisallowedLen = len(settings.cachetoml.Paths[idx].Disallowed)
		settings.cachetoml.Paths[idx].MaxSizeByte = int64(settings.cachetoml.Paths[idx].MaxSize) * 1024 * 1024
		settings.cachetoml.Paths[idx].MinSizeByte = int64(settings.cachetoml.Paths[idx].MinSize) * 1024 * 1024
		settings.cachetoml.Paths[idx].MinVideoSizeByte = int64(settings.cachetoml.Paths[idx].MinVideoSize) * 1024 * 1024
		settings.SettingsPath[settings.cachetoml.Paths[idx].Name] = &settings.cachetoml.Paths[idx]
	}
}

// setupRegexConfigs sets up Regex configurations with length calculations
func setupRegexConfigs() {
	for idx := range settings.cachetoml.Regex {
		settings.cachetoml.Regex[idx].RejectedLen = len(settings.cachetoml.Regex[idx].Rejected)
		settings.cachetoml.Regex[idx].RequiredLen = len(settings.cachetoml.Regex[idx].Required)
		settings.SettingsRegex[settings.cachetoml.Regex[idx].Name] = &settings.cachetoml.Regex[idx]
	}
}

// setupSchedulerConfigs sets up scheduler configurations conditionally
func setupSchedulerConfigs(reload bool) {
	if !reload {
		for idx := range settings.cachetoml.Scheduler {
			settings.SettingsScheduler[settings.cachetoml.Scheduler[idx].Name] = &settings.cachetoml.Scheduler[idx]
		}
	}
}

// setupQualityConfigs sets up Quality configurations with nested indexer setup
func setupQualityConfigs() {
	for idx := range settings.cachetoml.Quality {
		settings.cachetoml.Quality[idx].IndexerCfg = make([]*IndexersConfig, len(settings.cachetoml.Quality[idx].Indexer))
		for idx2 := range settings.cachetoml.Quality[idx].Indexer {
			settings.cachetoml.Quality[idx].Indexer[idx2].CfgDownloader = settings.SettingsDownloader[settings.cachetoml.Quality[idx].Indexer[idx2].TemplateDownloader]
			settings.cachetoml.Quality[idx].Indexer[idx2].CfgIndexer = settings.SettingsIndexer[settings.cachetoml.Quality[idx].Indexer[idx2].TemplateIndexer]
			settings.cachetoml.Quality[idx].IndexerCfg[idx2] = settings.SettingsIndexer[settings.cachetoml.Quality[idx].Indexer[idx2].TemplateIndexer]
			settings.cachetoml.Quality[idx].Indexer[idx2].CfgPath = settings.SettingsPath[settings.cachetoml.Quality[idx].Indexer[idx2].TemplatePathNzb]
			settings.cachetoml.Quality[idx].Indexer[idx2].CfgRegex = settings.SettingsRegex[settings.cachetoml.Quality[idx].Indexer[idx2].TemplateRegex]
		}
		settings.cachetoml.Quality[idx].IndexerLen = len(settings.cachetoml.Quality[idx].Indexer)
		settings.cachetoml.Quality[idx].QualityReorderLen = len(settings.cachetoml.Quality[idx].QualityReorder)
		settings.cachetoml.Quality[idx].TitleStripPrefixForSearchLen = len(settings.cachetoml.Quality[idx].TitleStripPrefixForSearch)
		settings.cachetoml.Quality[idx].TitleStripSuffixForSearchLen = len(settings.cachetoml.Quality[idx].TitleStripSuffixForSearch)
		settings.cachetoml.Quality[idx].WantedAudioLen = len(settings.cachetoml.Quality[idx].WantedAudio)
		settings.cachetoml.Quality[idx].WantedCodecLen = len(settings.cachetoml.Quality[idx].WantedCodec)
		settings.cachetoml.Quality[idx].WantedQualityLen = len(settings.cachetoml.Quality[idx].WantedQuality)
		settings.cachetoml.Quality[idx].WantedResolutionLen = len(settings.cachetoml.Quality[idx].WantedResolution)
		settings.SettingsQuality[settings.cachetoml.Quality[idx].Name] = &settings.cachetoml.Quality[idx]
	}
}

// populateConfigsInMap adds config entries to a map with prefix for GetCfgAll
func populateConfigsInMap(configMap map[string]any) {
	// Media configs use NamePrefix
	for key := range settings.SettingsMedia {
		configMap[settings.SettingsMedia[key].NamePrefix] = *settings.SettingsMedia[key]
	}
	// All other configs use standard prefixes
	for key := range settings.SettingsDownloader {
		configMap["downloader_"+key] = *settings.SettingsDownloader[key]
	}
	for key := range settings.SettingsIndexer {
		configMap["indexer_"+key] = *settings.SettingsIndexer[key]
	}
	for key := range settings.SettingsList {
		configMap["list_"+key] = *settings.SettingsList[key]
	}
	for key := range settings.SettingsNotification {
		configMap["notification_"+key] = *settings.SettingsNotification[key]
	}
	for key := range settings.SettingsPath {
		configMap["path_"+key] = *settings.SettingsPath[key]
	}
	for key := range settings.SettingsQuality {
		configMap["quality_"+key] = *settings.SettingsQuality[key]
	}
	for key := range settings.SettingsRegex {
		configMap["regex_"+key] = *settings.SettingsRegex[key]
	}
	for key := range settings.SettingsScheduler {
		configMap["scheduler_"+key] = *settings.SettingsScheduler[key]
	}
}

// getTemplateOptionsForType returns template options for a specific config type
func getTemplateOptionsForType(configType string) []string {
	var options []string
	switch configType {
	case "downloader":
		options = make([]string, 0, len(settings.SettingsDownloader)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsDownloader {
			options = append(options, cfg.Name)
		}
	case "indexer":
		options = make([]string, 0, len(settings.SettingsIndexer)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsIndexer {
			options = append(options, cfg.Name)
		}
	case "list":
		options = make([]string, 0, len(settings.SettingsList)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsList {
			options = append(options, cfg.Name)
		}
	case "notification":
		options = make([]string, 0, len(settings.SettingsNotification)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsNotification {
			options = append(options, cfg.Name)
		}
	case "path":
		options = make([]string, 0, len(settings.SettingsPath)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsPath {
			options = append(options, cfg.Name)
		}
	case "quality":
		options = make([]string, 0, len(settings.SettingsQuality)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsQuality {
			options = append(options, cfg.Name)
		}
	case "regex":
		options = make([]string, 0, len(settings.SettingsRegex)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsRegex {
			options = append(options, cfg.Name)
		}
	case "scheduler":
		options = make([]string, 0, len(settings.SettingsScheduler)+1)
		options = append(options, "")
		for _, cfg := range settings.SettingsScheduler {
			options = append(options, cfg.Name)
		}
	default:
		return nil
	}
	return options
}

// populateConfigSlicesInMap adds config entries as slices for GetCfgAllJson
func populateConfigSlicesInMap(configMap map[string]any) {
	configMap["media"] = []MediaTypeConfig{}
	for key := range settings.SettingsMedia {
		configMap["media"] = append(configMap["media"].([]MediaTypeConfig), *settings.SettingsMedia[key])
	}
	configMap["downloader"] = []DownloaderConfig{}
	for key := range settings.SettingsDownloader {
		configMap["downloader"] = append(configMap["downloader"].([]DownloaderConfig), *settings.SettingsDownloader[key])
	}
	configMap["indexer"] = []IndexersConfig{}
	for key := range settings.SettingsIndexer {
		configMap["indexer"] = append(configMap["indexer"].([]IndexersConfig), *settings.SettingsIndexer[key])
	}
	configMap["list"] = []ListsConfig{}
	for key := range settings.SettingsList {
		configMap["list"] = append(configMap["list"].([]ListsConfig), *settings.SettingsList[key])
	}
	configMap["notification"] = []NotificationConfig{}
	for key := range settings.SettingsNotification {
		configMap["notification"] = append(configMap["notification"].([]NotificationConfig), *settings.SettingsNotification[key])
	}
	configMap["path"] = []PathsConfig{}
	for key := range settings.SettingsPath {
		configMap["path"] = append(configMap["path"].([]PathsConfig), *settings.SettingsPath[key])
	}
	configMap["quality"] = []QualityConfig{}
	for key := range settings.SettingsQuality {
		configMap["quality"] = append(configMap["quality"].([]QualityConfig), *settings.SettingsQuality[key])
	}
	configMap["regex"] = []RegexConfig{}
	for key := range settings.SettingsRegex {
		configMap["regex"] = append(configMap["regex"].([]RegexConfig), *settings.SettingsRegex[key])
	}
	configMap["scheduler"] = []SchedulerConfig{}
	for key := range settings.SettingsScheduler {
		configMap["scheduler"] = append(configMap["scheduler"].([]SchedulerConfig), *settings.SettingsScheduler[key])
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
		handleConfigEntry(val)
	}
}

// GetCfgAll returns a map containing all the application configuration settings.
// It collects the settings from the various global config variables and organizes
// them into a single map indexed by config section name prefixes.
func GetCfgAll() map[string]any {
	mu.Lock()
	defer mu.Unlock()
	q := make(map[string]any)
	q["general"] = settings.SettingsGeneral
	q["imdb"] = settings.SettingsImdb
	populateConfigsInMap(q)
	return q
}

// GetSettingTemplatesFor returns template options for configuration forms.
// It generates a map containing available configuration names for a given type
// (downloader, indexer, list, notification, path, quality, regex, scheduler).
// Used to populate dropdown menus and form options in the web interface.
func GetSettingTemplatesFor(key string) map[string][]string {
	options := getTemplateOptionsForType(key)
	if options == nil {
		return nil
	}
	return map[string][]string{"options": options}
}

func GetCfgAllJson() map[string]any {
	mu.Lock()
	defer mu.Unlock()
	q := make(map[string]any)
	q["general"] = *settings.SettingsGeneral
	q["imdb"] = *settings.SettingsImdb
	populateConfigSlicesInMap(q)
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
	handleConfigEntryWithDB(configIn, configDB)
}

func UpdateCfgEntryAny(configIn any) error {
	mu.Lock()
	defer mu.Unlock()
	switch data := configIn.(type) {
	case GeneralConfig:
		settings.cachetoml.General = data
	case *GeneralConfig:
		settings.cachetoml.General = *data
	case []DownloaderConfig:
		settings.cachetoml.Downloader = data
	case ImdbConfig:
		settings.cachetoml.Imdbindexer = data
	case *ImdbConfig:
		settings.cachetoml.Imdbindexer = *data
	case []IndexersConfig:
		settings.cachetoml.Indexers = data
	case []ListsConfig:
		settings.cachetoml.Lists = data
	case MediaConfig:
		settings.cachetoml.Media = data
	case *MediaConfig:
		settings.cachetoml.Media = *data
	case []NotificationConfig:
		settings.cachetoml.Notification = data
	case []PathsConfig:
		settings.cachetoml.Paths = data
	case []QualityConfig:
		settings.cachetoml.Quality = data
	case []RegexConfig:
		settings.cachetoml.Regex = data
	case []SchedulerConfig:
		settings.cachetoml.Scheduler = data
	}
	return WriteCfgToml()
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
	handleConfigDeletionWithDB(name, configDB)
}

// GetToml returns the cached main configuration settings as a mainConfig struct.
// This function provides read-only access to the current configuration state.
func GetToml() MainConfig {
	return settings.cachetoml
}

// ClearCfg clears all configuration settings by deleting the config database file,
// resetting all config maps to empty maps, and reinitializing default settings.
// It wipes the existing config and starts fresh with defaults.
func ClearCfg() {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{SyncInterval: 0})
	defer configDB.Close()
	configDB.DeleteFile()

	mu.Lock()
	defer mu.Unlock()
	ClearSettings(true)

	settings.cachetoml = MainConfig{
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
				Notification: []MediaNotificationConfig{{MapNotification: "initial"}},
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
				Notification: []MediaNotificationConfig{{MapNotification: "initial"}},
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

	Getconfigtoml(true)
}

// WriteCfg marshals the application configuration structs into a TOML
// configuration file. It gathers all the global configuration structs,
// assembles them into a MainConfig struct, marshals to TOML and writes
// to the Configfile location.
func WriteCfg() {
	mu.Lock()
	defer mu.Unlock()
	bla := GetMainConfig()

	cnt, err := toml.Marshal(bla)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
	}
	os.WriteFile(Configfile, cnt, 0o777)
	settings.cachetoml = bla
}

// WriteCfgToml writes the cached TOML configuration to the configuration file.
// It marshals the cached configuration and writes it to disk, then refreshes
// the configuration cache. Returns any error encountered during the process.
func WriteCfgToml() error {
	cnt, err := toml.Marshal(&settings.cachetoml)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
	} else {
		err = os.WriteFile(Configfile, cnt, 0o777)
	}
	Getconfigtoml(true)
	return err
}

// GetMainConfig assembles and returns a complete MainConfig structure.
// It collects all configuration settings from global maps and structures
// them into a single MainConfig that can be marshaled to TOML format.
func GetMainConfig() MainConfig {
	var bla MainConfig

	bla.General = *settings.SettingsGeneral
	bla.Imdbindexer = *settings.SettingsImdb
	for _, cfgdata := range settings.SettingsDownloader {
		bla.Downloader = append(bla.Downloader, *cfgdata)
	}
	for _, cfgdata := range settings.SettingsIndexer {
		bla.Indexers = append(bla.Indexers, *cfgdata)
	}
	for _, cfgdata := range settings.SettingsList {
		bla.Lists = append(bla.Lists, *cfgdata)
	}
	for _, cfgdata := range settings.cachetoml.Media.Series {
		if !strings.HasPrefix(cfgdata.NamePrefix, logger.StrSerie) {
			continue
		}
		bla.Media.Series = append(bla.Media.Series, cfgdata)
	}
	for _, cfgdata := range settings.cachetoml.Media.Movies {
		if !strings.HasPrefix(cfgdata.NamePrefix, logger.StrMovie) {
			continue
		}
		bla.Media.Movies = append(bla.Media.Movies, cfgdata)
	}
	for _, cfgdata := range settings.SettingsNotification {
		bla.Notification = append(bla.Notification, *cfgdata)
	}
	for _, cfgdata := range settings.SettingsPath {
		bla.Paths = append(bla.Paths, *cfgdata)
	}
	for _, cfgdata := range settings.SettingsQuality {
		bla.Quality = append(bla.Quality, *cfgdata)
	}
	for _, cfgdata := range settings.SettingsRegex {
		bla.Regex = append(
			bla.Regex,
			RegexConfig{Name: cfgdata.Name, Required: cfgdata.Required, Rejected: cfgdata.Rejected},
		)
	}
	for _, cfgdata := range settings.SettingsScheduler {
		bla.Scheduler = append(bla.Scheduler, *cfgdata)
	}
	return bla
}

func GetSettingsGeneral() *GeneralConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsGeneral
}

func GetSettingsImdb() *ImdbConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsImdb
}

func GetSettingsMedia(name string) *MediaTypeConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsMedia[name]
}

func GetSettingsMediaAll() *MediaConfig {
	mu.Lock()
	defer mu.Unlock()
	return &settings.cachetoml.Media
}

func GetSettingsPath(name string) *PathsConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsPath[name]
}

func GetSettingsPathAll() []PathsConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Paths
}

func GetSettingsDownloaderAll() []DownloaderConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Downloader
}

func GetSettingsRegexAll() []RegexConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Regex
}

func GetSettingsQuality(name string) *QualityConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsQuality[name]
}

func GetSettingsQualityAll() []QualityConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Quality
}

func GetSettingsQualityOk(name string) (*QualityConfig, bool) {
	mu.Lock()
	defer mu.Unlock()
	val, ok := settings.SettingsQuality[name]
	return val, ok
}

func GetSettingsQualityLen() int {
	mu.Lock()
	defer mu.Unlock()
	return len(settings.SettingsQuality)
}

func GetSettingsScheduler(name string) *SchedulerConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsScheduler[name]
}

func GetSettingsSchedulerAll() []SchedulerConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Scheduler
}

func GetSettingsList(name string) *ListsConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsList[name]
}

func GetSettingsListAll() []ListsConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Lists
}

func GetSettingsMediaListAll() []string {
	mu.Lock()
	defer mu.Unlock()
	retlists := make([]string, 0, 50)
	for _, cfg := range settings.SettingsMedia {
		for i := range cfg.Lists {
			if slices.Contains(retlists, cfg.Lists[i].Name) {
				continue
			}
			retlists = append(retlists, cfg.Lists[i].Name)
		}
	}
	return retlists
}

func TestSettingsList(name string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := settings.SettingsList[name]
	return ok
}

func GetSettingsIndexer(name string) *IndexersConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsIndexer[name]
}

func GetSettingsIndexerAll() []IndexersConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Indexers
}

func GetSettingsNotification(name string) *NotificationConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.SettingsNotification[name]
}

func GetSettingsNotificationAll() []NotificationConfig {
	mu.Lock()
	defer mu.Unlock()
	return settings.cachetoml.Notification
}

func RangeSettingsMedia(fn func(string, *MediaTypeConfig) error) {
	for key, cfg := range settings.SettingsMedia {
		fn(key, cfg)
	}
}

func RangeSettingsMediaLists(media string, fn func(*MediaListsConfig)) {
	for _, cfg := range settings.SettingsMedia[media].Lists {
		fn(&cfg)
	}
}

func RangeSettingsMediaBreak(fn func(string, *MediaTypeConfig) bool) {
	for key, cfg := range settings.SettingsMedia {
		if fn(key, cfg) {
			break
		}
	}
}

func RangeSettingsQuality(fn func(string, *QualityConfig)) {
	for key, cfg := range settings.SettingsQuality {
		fn(key, cfg)
	}
}

func RangeSettingsList(fn func(string, *ListsConfig)) {
	for key, cfg := range settings.SettingsList {
		fn(key, cfg)
	}
}

func RangeSettingsIndexer(fn func(string, *IndexersConfig)) {
	for key, cfg := range settings.SettingsIndexer {
		fn(key, cfg)
	}
}

func RangeSettingsScheduler(fn func(string, *SchedulerConfig)) {
	for key, cfg := range settings.SettingsScheduler {
		fn(key, cfg)
	}
}

func RangeSettingsNotification(fn func(string, *NotificationConfig)) {
	for key, cfg := range settings.SettingsNotification {
		fn(key, cfg)
	}
}

func RangeSettingsPath(fn func(string, *PathsConfig)) {
	for key, cfg := range settings.SettingsPath {
		fn(key, cfg)
	}
}

func RangeSettingsRegex(fn func(string, *RegexConfig)) {
	for key, cfg := range settings.SettingsRegex {
		fn(key, cfg)
	}
}

func RangeSettingsDownloader(fn func(string, *DownloaderConfig)) {
	for key, cfg := range settings.SettingsDownloader {
		fn(key, cfg)
	}
}
