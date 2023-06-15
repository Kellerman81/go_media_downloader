// koanf_api
package config

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/pelletier/go-toml/v2"
	"github.com/recoilme/pudge"
	"golang.org/x/oauth2"
)

//Series Config

type MainSerieConfig struct {
	Global GlobalSerieConfig `koanf:"global" toml:"global"`
	Serie  []SerieConfig     `koanf:"series" toml:"series"`
}

type GlobalSerieConfig struct {
	Identifiedby   string `koanf:"identifiedby" toml:"identifiedby"`
	Upgrade        bool   `koanf:"upgrade" toml:"upgrade"`
	Search         bool   `koanf:"search" toml:"search"`
	SearchProvider string `koanf:"search_provider" toml:"search_provider"`
}
type SerieConfig struct {
	Name           string   `koanf:"name" toml:"name"`
	TvdbID         int      `koanf:"tvdb_id" toml:"tvdb_id"`
	AlternateName  []string `koanf:"alternatename" toml:"alternatename"`
	Identifiedby   string   `koanf:"identifiedby" toml:"identifiedby"`
	DontUpgrade    bool     `koanf:"dont_upgrade" toml:"dont_upgrade"`
	DontSearch     bool     `koanf:"dont_search" toml:"dont_search"`
	SearchSpecials bool     `koanf:"search_specials" toml:"search_specials"`
	IgnoreRuntime  bool     `koanf:"ignore_runtime" toml:"ignore_runtime"`
	Source         string   `koanf:"source" toml:"source"`
	Target         string   `koanf:"target" toml:"target"`
}

// Main Config
type MainConfig struct {
	General      GeneralConfig        `koanf:"general" toml:"general"`
	Imdbindexer  ImdbConfig           `koanf:"imdbindexer" toml:"imdbindexer"`
	Media        MediaConfig          `koanf:"media" toml:"media"`
	Downloader   []DownloaderConfig   `koanf:"downloader" toml:"downloader"`
	Lists        []ListsConfig        `koanf:"lists" toml:"lists"`
	Indexers     []IndexersConfig     `koanf:"indexers" toml:"indexers"`
	Paths        []PathsConfig        `koanf:"paths" toml:"paths"`
	Notification []NotificationConfig `koanf:"notification" toml:"notification"`
	Regex        []RegexConfig        `koanf:"regex" toml:"regex"`
	Quality      []QualityConfig      `koanf:"quality" toml:"quality"`
	Scheduler    []SchedulerConfig    `koanf:"scheduler" toml:"scheduler"`
}

type MainConfigOut struct {
	General      GeneralConfig        `koanf:"general" toml:"general"`
	Imdbindexer  ImdbConfig           `koanf:"imdbindexer" toml:"imdbindexer"`
	Media        MediaConfig          `koanf:"media" toml:"media"`
	Downloader   []DownloaderConfig   `koanf:"downloader" toml:"downloader"`
	Lists        []ListsConfig        `koanf:"lists" toml:"lists"`
	Indexers     []IndexersConfig     `koanf:"indexers" toml:"indexers"`
	Paths        []PathsConfig        `koanf:"paths" toml:"paths"`
	Notification []NotificationConfig `koanf:"notification" toml:"notification"`
	Regex        []RegexConfigIn      `koanf:"regex" toml:"regex"`
	Quality      []QualityConfig      `koanf:"quality" toml:"quality"`
	Scheduler    []SchedulerConfig    `koanf:"scheduler" toml:"scheduler"`
}

type GeneralConfig struct {
	TimeFormat                         string   `koanf:"time_format" toml:"time_format"`
	TimeZone                           string   `koanf:"time_zone" toml:"time_zone"`
	LogLevel                           string   `koanf:"log_level" toml:"log_level"`
	DBLogLevel                         string   `koanf:"db_log_level" toml:"db_log_level"`
	LogFileSize                        int      `koanf:"log_file_size" toml:"log_file_size"`
	LogFileCount                       int      `koanf:"log_file_count" toml:"log_file_count"`
	LogCompress                        bool     `koanf:"log_compress" toml:"log_compress"`
	WorkerMetadata                     int      `koanf:"worker_metadata" toml:"worker_metadata"`
	WorkerFiles                        int      `koanf:"worker_files" toml:"worker_files"`
	WorkerParse                        int      `koanf:"worker_parse" toml:"worker_parse"`
	WorkerSearch                       int      `koanf:"worker_search" toml:"worker_search"`
	WorkerIndexer                      int      `koanf:"worker_indexer" toml:"worker_indexer"`
	OmdbAPIKey                         string   `koanf:"omdb_apikey" toml:"omdb_apikey"`
	UseMediaCache                      bool     `koanf:"use_media_cache" toml:"use_media_cache"`
	MovieMetaSourceImdb                bool     `koanf:"movie_meta_source_imdb" toml:"movie_meta_source_imdb"`
	MovieMetaSourceTmdb                bool     `koanf:"movie_meta_source_tmdb" toml:"movie_meta_source_tmdb"`
	MovieMetaSourceOmdb                bool     `koanf:"movie_meta_source_omdb" toml:"movie_meta_source_omdb"`
	MovieMetaSourceTrakt               bool     `koanf:"movie_meta_source_trakt" toml:"movie_meta_source_trakt"`
	MovieAlternateTitleMetaSourceImdb  bool     `koanf:"movie_alternate_title_meta_source_imdb" toml:"movie_alternate_title_meta_source_imdb"`
	MovieAlternateTitleMetaSourceTmdb  bool     `koanf:"movie_alternate_title_meta_source_tmdb" toml:"movie_alternate_title_meta_source_tmdb"`
	MovieAlternateTitleMetaSourceOmdb  bool     `koanf:"movie_alternate_title_meta_source_omdb" toml:"movie_alternate_title_meta_source_omdb"`
	MovieAlternateTitleMetaSourceTrakt bool     `koanf:"movie_alternate_title_meta_source_trakt" toml:"movie_alternate_title_meta_source_trakt"`
	SerieAlternateTitleMetaSourceImdb  bool     `koanf:"serie_alternate_title_meta_source_imdb" toml:"serie_alternate_title_meta_source_imdb"`
	SerieAlternateTitleMetaSourceTrakt bool     `koanf:"serie_alternate_title_meta_source_trakt" toml:"serie_alternate_title_meta_source_trakt"`
	MovieMetaSourcePriority            []string `koanf:"movie_meta_source_priority" toml:"movie_meta_source_priority"`
	MovieRSSMetaSourcePriority         []string `koanf:"movie_rss_meta_source_priority" toml:"movie_rss_meta_source_priority"`
	MovieParseMetaSourcePriority       []string `koanf:"movie_parse_meta_source_priority" toml:"movie_parse_meta_source_priority"`
	SerieMetaSourceTmdb                bool     `koanf:"serie_meta_source_tmdb" toml:"serie_meta_source_tmdb"`
	SerieMetaSourceTrakt               bool     `koanf:"serie_meta_source_trakt" toml:"serie_meta_source_trakt"`
	UseGoDir                           bool     `koanf:"use_godir" toml:"use_godir"`
	MoveBufferSizeKB                   int      `koanf:"move_buffer_size_kb" toml:"move_buffer_size_kb"`
	WebPort                            string   `koanf:"webport" toml:"webport"`
	WebAPIKey                          string   `koanf:"webapikey" toml:"webapikey"`
	ConcurrentScheduler                int      `koanf:"concurrent_scheduler" toml:"concurrent_scheduler"`
	TheMovieDBApiKey                   string   `koanf:"themoviedb_apikey" toml:"themoviedb_apikey"`
	TraktClientID                      string   `koanf:"trakt_client_id" toml:"trakt_client_id"`
	TraktClientSecret                  string   `koanf:"trakt_client_secret" toml:"trakt_client_secret"`
	SchedulerDisabled                  bool     `koanf:"scheduler_disabled" toml:"scheduler_disabled"`
	DisableParserStringMatch           bool     `koanf:"disable_parser_string_match" toml:"disable_parser_string_match"`
	UseCronInsteadOfInterval           bool     `koanf:"use_cron_instead_of_interval" toml:"use_cron_instead_of_interval"`
	EnableFileWatcher                  bool     `koanf:"enable_file_watcher" toml:"enable_file_watcher"`
	UseFileBufferCopy                  bool     `koanf:"use_file_buffer_copy" toml:"use_file_buffer_copy"`
	DisableSwagger                     bool     `koanf:"disable_swagger" toml:"disable_swagger"`
	Traktlimiterseconds                int      `koanf:"trakt_limiter_seconds" toml:"trakt_limiter_seconds"`
	Traktlimitercalls                  int      `koanf:"trakt_limiter_calls" toml:"trakt_limiter_calls"`
	Tvdblimiterseconds                 int      `koanf:"tvdb_limiter_seconds" toml:"tvdb_limiter_seconds"`
	Tvdblimitercalls                   int      `koanf:"tvdb_limiter_calls" toml:"tvdb_limiter_calls"`
	Tmdblimiterseconds                 int      `koanf:"tmdb_limiter_seconds" toml:"tmdb_limiter_seconds"`
	Tmdblimitercalls                   int      `koanf:"tmdb_limiter_calls" toml:"tmdb_limiter_calls"`
	Omdblimiterseconds                 int      `koanf:"omdb_limiter_seconds" toml:"omdb_limiter_seconds"`
	Omdblimitercalls                   int      `koanf:"omdb_limiter_calls" toml:"omdb_limiter_calls"`
	TheMovieDBDisableTLSVerify         bool     `koanf:"tmdb_disable_tls_verify" toml:"tmdb_disable_tls_verify"`
	TraktDisableTLSVerify              bool     `koanf:"trakt_disable_tls_verify" toml:"trakt_disable_tls_verify"`
	OmdbDisableTLSVerify               bool     `koanf:"omdb_disable_tls_verify" toml:"omdb_disable_tls_verify"`
	TvdbDisableTLSVerify               bool     `koanf:"tvdb_disable_tls_verify" toml:"tvdb_disable_tls_verify"`
	FfprobePath                        string   `koanf:"ffprobe_path" toml:"ffprobe_path"`
	MediainfoPath                      string   `koanf:"mediainfo_path" toml:"mediainfo_path"`
	UseMediainfo                       bool     `koanf:"use_mediainfo" toml:"use_mediainfo"`
	UseMediaFallback                   bool     `koanf:"use_media_fallback" toml:"use_media_fallback"`
	FailedIndexerBlockTime             int      `koanf:"failed_indexer_block_time" toml:"failed_indexer_block_time"`
	MaxDatabaseBackups                 int      `koanf:"max_database_backups" toml:"max_database_backups"`
	DisableVariableCleanup             bool     `koanf:"disable_variable_cleanup" toml:"disable_variable_cleanup"`
	OmdbTimeoutSeconds                 int      `koanf:"omdb_timeout_seconds" toml:"omdb_timeout_seconds"`
	TmdbTimeoutSeconds                 int      `koanf:"tmdb_timeout_seconds" toml:"tmdb_timeout_seconds"`
	TvdbTimeoutSeconds                 int      `koanf:"tvdb_timeout_seconds" toml:"tvdb_timeout_seconds"`
	TraktTimeoutSeconds                int      `koanf:"trakt_timeout_seconds" toml:"trakt_timeout_seconds"`
}

type ImdbConfig struct {
	Indexedtypes     []string `koanf:"indexed_types" toml:"indexed_types"`
	Indexedlanguages []string `koanf:"indexed_languages" toml:"indexed_languages"`
	Indexfull        bool     `koanf:"index_full" toml:"index_full"`
}

type MediaConfig struct {
	Series []MediaTypeConfig `koanf:"series" toml:"series"`
	Movies []MediaTypeConfig `koanf:"movies" toml:"movies"`
}

type MediaTypeConfig struct {
	Name                     string                    `koanf:"name" toml:"name"`
	NamePrefix               string                    `koanf:"name" toml:"nameprefix"`
	DefaultQuality           string                    `koanf:"default_quality" toml:"default_quality"`
	DefaultResolution        string                    `koanf:"default_resolution" toml:"default_resolution"`
	Naming                   string                    `koanf:"naming" toml:"naming"`
	NamingIdentifier         string                    `koanf:"naming_identifier" toml:"naming_identifier"`
	TemplateQuality          string                    `koanf:"template_quality" toml:"template_quality"`
	TemplateScheduler        string                    `koanf:"template_scheduler" toml:"template_scheduler"`
	MetadataLanguage         string                    `koanf:"metadata_language" toml:"metadata_language"`
	MetadataTitleLanguages   []string                  `koanf:"metadata_title_languages" toml:"metadata_title_languages"`
	MetadataSource           string                    `koanf:"metadata_source" toml:"metadata_source"`
	Structure                bool                      `koanf:"structure" toml:"structure"`
	SearchmissingIncremental int                       `koanf:"search_missing_incremental" toml:"search_missing_incremental"`
	SearchupgradeIncremental int                       `koanf:"search_upgrade_incremental" toml:"search_upgrade_incremental"`
	Data                     []MediaDataConfig         `koanf:"data" toml:"data"`
	DataImport               []MediaDataImportConfig   `koanf:"data_import" toml:"data_import"`
	Lists                    []MediaListsConfig        `koanf:"lists" toml:"lists"`
	ListsInt                 []interface{}             `koanf:"-" toml:"-"`
	Notification             []MediaNotificationConfig `koanf:"notification" toml:"notification"`
}

type MediaDataConfig struct {
	TemplatePath string `koanf:"template_path" toml:"template_path"`
	AddFound     bool   `koanf:"add_found" toml:"add_found"`
	AddFoundList string `koanf:"add_found_list" toml:"add_found_list"`
}

type MediaDataImportConfig struct {
	TemplatePath string `koanf:"template_path" toml:"template_path"`
}

type MediaListsConfig struct {
	Name              string        `koanf:"name" toml:"name"`
	TemplateList      string        `koanf:"template_list" toml:"template_list"`
	TemplateQuality   string        `koanf:"template_quality" toml:"template_quality"`
	TemplateScheduler string        `koanf:"template_scheduler" toml:"template_scheduler"`
	IgnoreMapLists    []string      `koanf:"ignore_template_lists" toml:"ignore_template_lists"`
	IgnoreMapListsInt []interface{} `koanf:"-" toml:"-"`
	ReplaceMapLists   []string      `koanf:"replace_template_lists" toml:"replace_template_lists"`
	Enabled           bool          `koanf:"enabled" toml:"enabled"`
	Addfound          bool          `koanf:"add_found" toml:"add_found"`
}

type MediaNotificationConfig struct {
	MapNotification string `koanf:"template_notification" toml:"template_notification"`
	Event           string `koanf:"event" toml:"event"`
	Title           string `koanf:"title" toml:"title"`
	Message         string `koanf:"message" toml:"message"`
	ReplacedPrefix  string `koanf:"replaced_prefix" toml:"replaced_prefix"`
}

type DownloaderConfig struct {
	Name            string `koanf:"name" toml:"name"`
	DlType          string `koanf:"type" toml:"type"`
	Hostname        string `koanf:"hostname" toml:"hostname"`
	Port            int    `koanf:"port" toml:"port"`
	Username        string `koanf:"username" toml:"username"`
	Password        string `koanf:"password" toml:"password"`
	AddPaused       bool   `koanf:"add_paused" toml:"add_paused"`
	DelugeDlTo      string `koanf:"deluge_dl_to" toml:"deluge_dl_to"`
	DelugeMoveAfter bool   `koanf:"deluge_move_after" toml:"deluge_move_after"`
	DelugeMoveTo    string `koanf:"deluge_move_to" toml:"deluge_move_to"`
	Priority        int    `koanf:"priority" toml:"priority"`
	Enabled         bool   `koanf:"enabled" toml:"enabled"`
}

type ListsConfig struct {
	Name             string   `koanf:"name" toml:"name"`
	ListType         string   `koanf:"type" toml:"type"`
	URL              string   `koanf:"url" toml:"url"`
	Enabled          bool     `koanf:"enabled" toml:"enabled"`
	SeriesConfigFile string   `koanf:"series_config_file" toml:"series_config_file"`
	TraktUsername    string   `koanf:"trakt_username" toml:"trakt_username"`
	TraktListName    string   `koanf:"trakt_listname" toml:"trakt_listname"`
	TraktListType    string   `koanf:"trakt_listtype" toml:"trakt_listtype"`
	Limit            int      `koanf:"limit" toml:"limit"`
	MinVotes         int      `koanf:"min_votes" toml:"min_votes"`
	MinRating        float32  `koanf:"min_rating" toml:"min_rating"`
	Excludegenre     []string `koanf:"exclude_genre" toml:"exclude_genre"`
	Includegenre     []string `koanf:"include_genre" toml:"include_genre"`
}

type IndexersConfig struct {
	Name                   string `koanf:"name" toml:"name"`
	IndexerType            string `koanf:"type" toml:"type"`
	URL                    string `koanf:"url" toml:"url"`
	Apikey                 string `koanf:"apikey" toml:"apikey"`
	Userid                 string `koanf:"userid" toml:"userid"`
	Enabled                bool   `koanf:"enabled" toml:"enabled"`
	Rssenabled             bool   `koanf:"rss_enabled" toml:"rss_enabled"`
	Addquotesfortitlequery bool   `koanf:"add_quotes_for_title_query" toml:"add_quotes_for_title_query"`
	MaxRssEntries          int    `koanf:"max_rss_entries" toml:"max_rss_entries"`
	RssEntriesloop         int    `koanf:"rss_entries_loop" toml:"rss_entries_loop"`
	OutputAsJSON           bool   `koanf:"output_as_json" toml:"output_as_json"`
	Customapi              string `koanf:"custom_api" toml:"custom_api"`
	Customurl              string `koanf:"custom_url" toml:"custom_url"`
	Customrssurl           string `koanf:"custom_rss_url" toml:"custom_rss_url"`
	Customrsscategory      string `koanf:"custom_rss_category" toml:"custom_rss_category"`
	Limitercalls           int    `koanf:"limiter_calls" toml:"limiter_calls"`
	Limiterseconds         int    `koanf:"limiter_seconds" toml:"limiter_seconds"`
	LimitercallsDaily      int    `koanf:"limiter_calls_daily" toml:"limiter_calls_daily"`
	MaxAge                 int    `koanf:"max_age" toml:"max_age"`
	DisableTLSVerify       bool   `koanf:"disable_tls_verify" toml:"disable_tls_verify"`
	DisableCompression     bool   `koanf:"disable_compression" toml:"disable_compression"`
	TimeoutSeconds         int    `koanf:"timeout_seconds" toml:"timeout_seconds"`
}

type PathsConfig struct {
	Name                           string   `koanf:"name" toml:"name"`
	Path                           string   `koanf:"path" toml:"path"`
	AllowedVideoExtensions         []string `koanf:"allowed_video_extensions" toml:"allowed_video_extensions"`
	AllowedOtherExtensions         []string `koanf:"allowed_other_extensions" toml:"allowed_other_extensions"`
	AllowedVideoExtensionsNoRename []string `koanf:"allowed_video_extensions_no_rename" toml:"allowed_video_extensions_no_rename"`
	AllowedOtherExtensionsNoRename []string `koanf:"allowed_other_extensions_no_rename" toml:"allowed_other_extensions_no_rename"`
	Blocked                        []string `koanf:"blocked" toml:"blocked"`
	Upgrade                        bool     `koanf:"upgrade" toml:"upgrade"`
	MinSize                        int      `koanf:"min_size" toml:"min_size"`
	MaxSize                        int      `koanf:"max_size" toml:"max_size"`
	MinSizeByte                    int64    `koanf:"-" toml:"-"`
	MaxSizeByte                    int64    `koanf:"-" toml:"-"`
	MinVideoSize                   int      `koanf:"min_video_size" toml:"min_video_size"`
	MinVideoSizeByte               int64    `koanf:"-" toml:"-"`
	CleanupsizeMB                  int      `koanf:"cleanup_size_mb" toml:"cleanup_size_mb"`
	AllowedLanguages               []string `koanf:"allowed_languages" toml:"allowed_languages"`
	Replacelower                   bool     `koanf:"replace_lower" toml:"replace_lower"`
	Usepresort                     bool     `koanf:"use_presort" toml:"use_presort"`
	PresortFolderPath              string   `koanf:"presort_folder_path" toml:"presort_folder_path"`
	UpgradeScanInterval            int      `koanf:"upgrade_scan_interval" toml:"upgrade_scan_interval"`
	MissingScanInterval            int      `koanf:"missing_scan_interval" toml:"missing_scan_interval"`
	MissingScanReleaseDatePre      int      `koanf:"missing_scan_release_date_pre" toml:"missing_scan_release_date_pre"`
	Disallowed                     []string `koanf:"disallowed" toml:"disallowed"`
	DeleteWrongLanguage            bool     `koanf:"delete_wrong_language" toml:"delete_wrong_language"`
	DeleteDisallowed               bool     `koanf:"delete_disallowed" toml:"delete_disallowed"`
	CheckRuntime                   bool     `koanf:"check_runtime" toml:"check_runtime"`
	MaxRuntimeDifference           int      `koanf:"max_runtime_difference" toml:"max_runtime_difference"`
	DeleteWrongRuntime             bool     `koanf:"delete_wrong_runtime" toml:"delete_wrong_runtime"`
	MoveReplaced                   bool     `koanf:"move_replaced" toml:"move_replaced"`
	MoveReplacedTargetPath         string   `koanf:"move_replaced_target_path" toml:"move_replaced_target_path"`
	SetChmod                       string   `koanf:"set_chmod" toml:"set_chmod"`
}

type NotificationConfig struct {
	Name             string `koanf:"name" toml:"name"`
	NotificationType string `koanf:"type" toml:"type"`
	Apikey           string `koanf:"apikey" toml:"apikey"`
	Recipient        string `koanf:"recipient" toml:"recipient"`
	Outputto         string `koanf:"output_to" toml:"output_to"`
}

type RegexConfigIn struct {
	Name     string   `koanf:"name" toml:"name"`
	Required []string `koanf:"required" toml:"required"`
	Rejected []string `koanf:"rejected" toml:"rejected"`
}

type RegexConfig struct {
	RegexConfigIn
}

type QualityConfig struct {
	Name                           string                 `koanf:"name" toml:"name"`
	WantedResolution               []string               `koanf:"wanted_resolution" toml:"wanted_resolution"`
	WantedQuality                  []string               `koanf:"wanted_quality" toml:"wanted_quality"`
	WantedAudio                    []string               `koanf:"wanted_audio" toml:"wanted_audio"`
	WantedCodec                    []string               `koanf:"wanted_codec" toml:"wanted_codec"`
	CutoffResolution               string                 `koanf:"cutoff_resolution" toml:"cutoff_resolution"`
	CutoffQuality                  string                 `koanf:"cutoff_quality" toml:"cutoff_quality"`
	SearchForTitleIfEmpty          bool                   `koanf:"search_for_title_if_empty" toml:"search_for_title_if_empty"`
	BackupSearchForTitle           bool                   `koanf:"backup_search_for_title" toml:"backup_search_for_title"`
	SearchForAlternateTitleIfEmpty bool                   `koanf:"search_for_alternate_title_if_empty" toml:"search_for_alternate_title_if_empty"`
	BackupSearchForAlternateTitle  bool                   `koanf:"backup_search_for_alternate_title" toml:"backup_search_for_alternate_title"`
	ExcludeYearFromTitleSearch     bool                   `koanf:"exclude_year_from_title_search" toml:"exclude_year_from_title_search"`
	CheckUntilFirstFound           bool                   `koanf:"check_until_first_found" toml:"check_until_first_found"`
	CheckTitle                     bool                   `koanf:"check_title" toml:"check_title"`
	CheckYear                      bool                   `koanf:"check_year" toml:"check_year"`
	CheckYear1                     bool                   `koanf:"check_year1" toml:"check_year1"`
	TitleStripSuffixForSearch      []string               `koanf:"title_strip_suffix_for_search" toml:"title_strip_suffix_for_search"`
	TitleStripPrefixForSearch      []string               `koanf:"title_strip_prefix_for_search" toml:"title_strip_prefix_for_search"`
	QualityReorder                 []QualityReorderConfig `koanf:"reorder" toml:"reorder"`
	Indexer                        []QualityIndexerConfig `koanf:"indexers" toml:"indexers"`
	UseForPriorityResolution       bool                   `koanf:"use_for_priority_resolution" toml:"use_for_priority_resolution"`
	UseForPriorityQuality          bool                   `koanf:"use_for_priority_quality" toml:"use_for_priority_quality"`
	UseForPriorityAudio            bool                   `koanf:"use_for_priority_audio" toml:"use_for_priority_audio"`
	UseForPriorityCodec            bool                   `koanf:"use_for_priority_codec" toml:"use_for_priority_codec"`
	UseForPriorityOther            bool                   `koanf:"use_for_priority_other" toml:"use_for_priority_other"`
	UseForPriorityMinDifference    int                    `koanf:"use_for_priority_min_difference" toml:"use_for_priority_min_difference"`
}

type QualityReorderConfig struct {
	Name        string `koanf:"name" toml:"name"`
	ReorderType string `koanf:"type" toml:"type"`
	Newpriority int    `koanf:"new_priority" toml:"new_priority"`
}
type QualityReorderConfigGroup struct {
	Arr []QualityReorderConfig
}
type QualityIndexerConfig struct {
	TemplateIndexer       string `koanf:"template_indexer" toml:"template_indexer"`
	TemplateDownloader    string `koanf:"template_downloader" toml:"template_downloader"`
	TemplateRegex         string `koanf:"template_regex" toml:"template_regex"`
	TemplatePathNzb       string `koanf:"template_path_nzb" toml:"template_path_nzb"`
	CategoryDowloader     string `koanf:"category_dowloader" toml:"category_dowloader"`
	AdditionalQueryParams string `koanf:"additional_query_params" toml:"additional_query_params"`
	CustomQueryString     string `koanf:"custom_query_string" toml:"custom_query_string"`
	SkipEmptySize         bool   `koanf:"skip_empty_size" toml:"skip_empty_size"`
	HistoryCheckTitle     bool   `koanf:"history_check_title" toml:"history_check_title"`
	CategoriesIndexer     string `koanf:"categories_indexer" toml:"categories_indexer"`
}

type SchedulerConfig struct {
	Name                            string `koanf:"name" toml:"name"`
	IntervalImdb                    string `koanf:"interval_imdb" toml:"interval_imdb"`
	IntervalFeeds                   string `koanf:"interval_feeds" toml:"interval_feeds"`
	IntervalFeedsRefreshSeries      string `koanf:"interval_feeds_refresh_series" toml:"interval_feeds_refresh_series"`
	IntervalFeedsRefreshMovies      string `koanf:"interval_feeds_refresh_movies" toml:"interval_feeds_refresh_movies"`
	IntervalFeedsRefreshSeriesFull  string `koanf:"interval_feeds_refresh_series_full" toml:"interval_feeds_refresh_series_full"`
	IntervalFeedsRefreshMoviesFull  string `koanf:"interval_feeds_refresh_movies_full" toml:"interval_feeds_refresh_movies_full"`
	IntervalIndexerMissing          string `koanf:"interval_indexer_missing" toml:"interval_indexer_missing"`
	IntervalIndexerUpgrade          string `koanf:"interval_indexer_upgrade" toml:"interval_indexer_upgrade"`
	IntervalIndexerMissingFull      string `koanf:"interval_indexer_missing_full" toml:"interval_indexer_missing_full"`
	IntervalIndexerUpgradeFull      string `koanf:"interval_indexer_upgrade_full" toml:"interval_indexer_upgrade_full"`
	IntervalIndexerMissingTitle     string `koanf:"interval_indexer_missing_title" toml:"interval_indexer_missing_title"`
	IntervalIndexerUpgradeTitle     string `koanf:"interval_indexer_upgrade_title" toml:"interval_indexer_upgrade_title"`
	IntervalIndexerMissingFullTitle string `koanf:"interval_indexer_missing_full_title" toml:"interval_indexer_missing_full_title"`
	IntervalIndexerUpgradeFullTitle string `koanf:"interval_indexer_upgrade_full_title" toml:"interval_indexer_upgrade_full_title"`
	IntervalIndexerRss              string `koanf:"interval_indexer_rss" toml:"interval_indexer_rss"`
	IntervalScanData                string `koanf:"interval_scan_data" toml:"interval_scan_data"`
	IntervalScanDataMissing         string `koanf:"interval_scan_data_missing" toml:"interval_scan_data_missing"`
	IntervalScanDataFlags           string `koanf:"interval_scan_data_flags" toml:"interval_scan_data_flags"`
	IntervalScanDataimport          string `koanf:"interval_scan_data_import" toml:"interval_scan_data_import"`
	IntervalDatabaseBackup          string `koanf:"interval_database_backup" toml:"interval_database_backup"`
	IntervalDatabaseCheck           string `koanf:"interval_database_check" toml:"interval_database_check"`
	IntervalIndexerRssSeasons       string `koanf:"interval_indexer_rss_seasons" toml:"interval_indexer_rss_seasons"`
	IntervalIndexerRssSeasonsAll    string `koanf:"interval_indexer_rss_seasons_all" toml:"interval_indexer_rss_seasons_all"`
	CronIndexerRssSeasonsAll        string `koanf:"cron_indexer_rss_seasons_all" toml:"cron_indexer_rss_seasons_all"`
	CronIndexerRssSeasons           string `koanf:"cron_indexer_rss_seasons" toml:"cron_indexer_rss_seasons"`
	CronImdb                        string `koanf:"cron_imdb" toml:"cron_imdb"`
	CronFeeds                       string `koanf:"cron_feeds" toml:"cron_feeds"`
	CronFeedsRefreshSeries          string `koanf:"cron_feeds_refresh_series" toml:"cron_feeds_refresh_series"`
	CronFeedsRefreshMovies          string `koanf:"cron_feeds_refresh_movies" toml:"cron_feeds_refresh_movies"`
	CronFeedsRefreshSeriesFull      string `koanf:"cron_feeds_refresh_series_full" toml:"cron_feeds_refresh_series_full"`
	CronFeedsRefreshMoviesFull      string `koanf:"cron_feeds_refresh_movies_full" toml:"cron_feeds_refresh_movies_full"`
	CronIndexerMissing              string `koanf:"cron_indexer_missing" toml:"cron_indexer_missing"`
	CronIndexerUpgrade              string `koanf:"cron_indexer_upgrade" toml:"cron_indexer_upgrade"`
	CronIndexerMissingFull          string `koanf:"cron_indexer_missing_full" toml:"cron_indexer_missing_full"`
	CronIndexerUpgradeFull          string `koanf:"cron_indexer_upgrade_full" toml:"cron_indexer_upgrade_full"`
	CronIndexerMissingTitle         string `koanf:"cron_indexer_missing_title" toml:"cron_indexer_missing_title"`
	CronIndexerUpgradeTitle         string `koanf:"cron_indexer_upgrade_title" toml:"cron_indexer_upgrade_title"`
	CronIndexerMissingFullTitle     string `koanf:"cron_indexer_missing_full_title" toml:"cron_indexer_missing_full_title"`
	CronIndexerUpgradeFullTitle     string `koanf:"cron_indexer_upgrade_full_title" toml:"cron_indexer_upgrade_full_title"`
	CronIndexerRss                  string `koanf:"cron_indexer_rss" toml:"cron_indexer_rss"`
	CronScanData                    string `koanf:"cron_scan_data" toml:"cron_scan_data"`
	CronScanDataMissing             string `koanf:"cron_scan_data_missing" toml:"cron_scan_data_missing"`
	CronScanDataFlags               string `koanf:"cron_scan_data_flags" toml:"cron_scan_data_flags"`
	CronScanDataimport              string `koanf:"cron_scan_data_import" toml:"cron_scan_data_import"`
	CronDatabaseBackup              string `koanf:"cron_database_backup" toml:"cron_database_backup"`
	CronDatabaseCheck               string `koanf:"cron_database_check" toml:"cron_database_check"`
}

const configfile = "./config/config.toml"

var (
	SettingsGeneral      GeneralConfig
	SettingsImdb         ImdbConfig
	SettingsPath         map[string]PathsConfig
	SettingsQuality      map[string]QualityConfig
	SettingsList         map[string]ListsConfig
	SettingsIndexer      map[string]IndexersConfig
	SettingsRegex        map[string]RegexConfig
	SettingsMedia        map[string]MediaTypeConfig
	SettingsNotification map[string]NotificationConfig
	SettingsDownloader   map[string]DownloaderConfig
	SettingsScheduler    map[string]SchedulerConfig
)

//var Cfg MainConfigMap

func (s *QualityReorderConfigGroup) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.Clear(&s.Arr)
	logger.ClearVar(s)
}
func (s *MainConfig) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.Downloader = nil
	s.Indexers = nil
	s.Lists = nil
	s.Media.Movies = nil
	s.Media.Series = nil
	s.Notification = nil
	s.Paths = nil
	s.Quality = nil
	s.Regex = nil
	s.Scheduler = nil
	logger.ClearVar(s)
}
func (s *MainSerieConfig) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.Clear(&s.Serie)
	logger.ClearVar(s)
}
func (q *SerieConfig) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if q == nil {
		return
	}
	logger.Clear(&q.AlternateName)
	logger.ClearVar(q)
}
func GetTemplateList(cfgpstr string, str string) (string, bool) {
	for idx := range SettingsMedia[cfgpstr].Lists {
		if strings.EqualFold(SettingsMedia[cfgpstr].Lists[idx].Name, str) {
			return SettingsMedia[cfgpstr].Lists[idx].TemplateList, SettingsMedia[cfgpstr].Lists[idx].Enabled
		}
	}
	// intid := logger.IndexFuncS(SettingsMedia[cfgpstr].Lists, func(e MediaListsConfig) bool { return strings.EqualFold(e.Name, str) })
	// if intid != -1 {
	// 	return SettingsMedia[cfgpstr].Lists[intid].TemplateList, SettingsMedia[cfgpstr].Lists[intid].Enabled
	// }
	return "", false
}

func Slepping(random bool, seconds int) {
	if random {
		rand.New(rand.NewSource(time.Now().UnixNano()))
		n := rand.Intn(seconds) + 1 // n will be between 0 and 10
		logger.Log.Debug().Int("seconds", n).Msg("Sleeping for")
		time.Sleep(time.Duration(n) * time.Second)
	} else {
		logger.Log.Debug().Int("seconds", seconds).Msg("Sleeping for")
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

func GetCfgFile() string {
	return configfile
}

func LoadCfgDB() {
	content, err := os.Open(configfile)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
		return
	}
	defer content.Close()
	var results MainConfig
	//err = toml.Unmarshal(content, &results)
	err = toml.NewDecoder(content).Decode(&results)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
		return
	}
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	pudge.BackupAll("")
	// Cfg.Keys = make(map[string]bool)
	SettingsDownloader = make(map[string]DownloaderConfig, len(results.Downloader))
	SettingsIndexer = make(map[string]IndexersConfig, len(results.Indexers))
	SettingsList = make(map[string]ListsConfig, len(results.Lists))
	SettingsMedia = make(map[string]MediaTypeConfig)
	// Cfg.Movies = make(map[string]MediaTypeConfig, len(results.Media.Movies))
	// Cfg.Series = make(map[string]MediaTypeConfig, len(results.Media.Series))
	SettingsNotification = make(map[string]NotificationConfig, len(results.Notification))
	SettingsPath = make(map[string]PathsConfig, len(results.Paths))
	SettingsQuality = make(map[string]QualityConfig, len(results.Quality))
	SettingsRegex = make(map[string]RegexConfig, len(results.Regex))
	SettingsScheduler = make(map[string]SchedulerConfig, len(results.Scheduler))

	//logger.Settings.Set("general", results.General, 0, false)
	SettingsGeneral = results.General
	//Cfg.General = results.General
	// if Cfg.General.WebAPIKey != "" {
	// 	Cfg.Keys["general"] = true
	// }
	//Cfg.Imdbindexer = results.Imdbindexer
	//	logger.Settings.Set("imdb", results.Imdbindexer, 0, false)
	SettingsImdb = results.Imdbindexer
	// if len(Cfg.Imdbindexer.Indexedtypes) >= 1 {
	// 	Cfg.Keys[logger.StrImdb] = true
	// }

	for idx := range results.Downloader {
		//logger.Settings.Set("downloader_"+results.Downloader[idx].Name, results.Downloader[idx], 0, false)
		SettingsDownloader["downloader_"+results.Downloader[idx].Name] = results.Downloader[idx]
		//Cfg.Keys["downloader_"+results.Downloader[idx].Name] = true
	}
	for idx := range results.Indexers {
		//logger.Settings.Set("indexer_"+results.Indexers[idx].Name, results.Indexers[idx], 0, false)
		SettingsIndexer["indexer_"+results.Indexers[idx].Name] = results.Indexers[idx]
		//Cfg.Indexers[results.Indexers[idx].Name] = results.Indexers[idx]
		//Cfg.Keys["indexer_"+results.Indexers[idx].Name] = true
	}
	for idx := range results.Lists {
		//logger.Settings.Set("list_"+results.Lists[idx].Name, results.Lists[idx], 0, false)
		SettingsList["list_"+results.Lists[idx].Name] = results.Lists[idx]
		//Cfg.Keys["list_"+results.Lists[idx].Name] = true
	}

	for idx := range results.Notification {
		//logger.Settings.Set("notification_"+results.Notification[idx].Name, results.Notification[idx], 0, false)
		SettingsNotification["notification_"+results.Notification[idx].Name] = results.Notification[idx]
		//Cfg.Keys["notification_"+results.Notification[idx].Name] = true
	}
	for idx := range results.Paths {
		results.Paths[idx].MaxSizeByte = int64(results.Paths[idx].MaxSize) * 1024 * 1024
		results.Paths[idx].MinSizeByte = int64(results.Paths[idx].MinSize) * 1024 * 1024
		results.Paths[idx].MinVideoSizeByte = int64(results.Paths[idx].MinVideoSize) * 1024 * 1024
		//logger.Settings.Set("path_"+results.Paths[idx].Name, results.Paths[idx], 0, false)
		SettingsPath["path_"+results.Paths[idx].Name] = results.Paths[idx]
		//Cfg.Keys["path_"+results.Paths[idx].Name] = true
	}
	//for key, val := range Cfg.Paths {
	//Cfg.PathsP[key] = &val
	//}
	for idx := range results.Quality {
		//logger.Settings.Set("quality_"+results.Quality[idx].Name, results.Quality[idx], 0, false)
		SettingsQuality["quality_"+results.Quality[idx].Name] = results.Quality[idx]
		//Cfg.Keys["quality_"+results.Quality[idx].Name] = true
	}
	for idx := range results.Regex {
		//logger.Settings.Set("regex_"+results.Regex[idx].Name, results.Regex[idx], 0, false)
		SettingsRegex["regex_"+results.Regex[idx].Name] = results.Regex[idx]
		//Cfg.Keys["regex_"+results.Regex[idx].Name] = true
	}
	for idx := range results.Scheduler {
		//logger.Settings.Set("scheduler_"+results.Scheduler[idx].Name, results.Scheduler[idx], 0, false)
		SettingsScheduler["scheduler_"+results.Scheduler[idx].Name] = results.Scheduler[idx]
		//Cfg.Keys["scheduler_"+results.Scheduler[idx].Name] = true
	}
	for idx := range results.Media.Movies {
		results.Media.Movies[idx].NamePrefix = "movie_" + results.Media.Movies[idx].Name
		results.Media.Movies[idx].ListsInt = make([]interface{}, len(results.Media.Movies[idx].Lists))
		for idxsub := range results.Media.Movies[idx].Lists {
			results.Media.Movies[idx].Lists[idxsub].IgnoreMapListsInt = make([]interface{}, len(results.Media.Movies[idx].Lists[idxsub].IgnoreMapLists))
			for idxign := range results.Media.Movies[idx].Lists[idxsub].IgnoreMapLists {
				results.Media.Movies[idx].Lists[idxsub].IgnoreMapListsInt[idxign] = results.Media.Movies[idx].Lists[idxsub].IgnoreMapLists[idxign]
			}
			results.Media.Movies[idx].ListsInt[idxsub] = results.Media.Movies[idx].Lists[idxsub].Name
		}

		//logger.Settings.Set("movie_"+results.Media.Movies[idx].Name, results.Media.Movies[idx], 0, false)
		//logger.Settings.Set("media_"+results.Media.Movies[idx].Name, results.Media.Movies[idx], 0, false)
		SettingsMedia["movie_"+results.Media.Movies[idx].Name] = results.Media.Movies[idx]
		//Cfg.Movies[results.Media.Movies[idx].Name] = results.Media.Movies[idx]
		//Cfg.Media["movie_"+results.Media.Movies[idx].Name] = results.Media.Movies[idx]
		//Cfg.Keys["movie_"+results.Media.Movies[idx].Name] = true
	}
	for idx := range results.Media.Series {
		results.Media.Series[idx].NamePrefix = "serie_" + results.Media.Series[idx].Name
		results.Media.Series[idx].ListsInt = make([]interface{}, len(results.Media.Series[idx].Lists))
		for idxsub := range results.Media.Series[idx].Lists {
			results.Media.Series[idx].Lists[idxsub].IgnoreMapListsInt = make([]interface{}, len(results.Media.Series[idx].Lists[idxsub].IgnoreMapLists))
			for idxign := range results.Media.Series[idx].Lists[idxsub].IgnoreMapLists {
				results.Media.Series[idx].Lists[idxsub].IgnoreMapListsInt[idxign] = results.Media.Series[idx].Lists[idxsub].IgnoreMapLists[idxign]
			}
			results.Media.Series[idx].ListsInt[idxsub] = results.Media.Series[idx].Lists[idxsub].Name
		}
		//logger.Settings.Set("serie_"+results.Media.Series[idx].Name, results.Media.Series[idx], 0, false)
		//logger.Settings.Set("media_"+results.Media.Series[idx].Name, results.Media.Series[idx], 0, false)
		SettingsMedia["serie_"+results.Media.Series[idx].Name] = results.Media.Series[idx]
		//Cfg.Series[results.Media.Series[idx].Name] = results.Media.Series[idx]
		//Cfg.Media["serie_"+results.Media.Series[idx].Name] = results.Media.Series[idx]
		//Cfg.Keys["serie_"+results.Media.Series[idx].Name] = true
	}
	results.Close()
	//Get from DB and not config
	hastoken, _ := configDB.Has("trakt_token")
	if hastoken {
		var token oauth2.Token
		if configDB.Get("trakt_token", &token) == nil {
			str := "trakt_token"
			logger.GlobalCache.Set(str, token, 0, false)
		}
	}
	configDB.Close()
}

func GetMediaListsEntryIndex(cfgpstr string, listname string) int {
	if cfgpstr == "" {
		logger.LogerrorStr(nil, "config empty", cfgpstr, "the config couldnt be found5")
		return -1
	}
	for i := range SettingsMedia[cfgpstr].Lists {
		if strings.EqualFold(SettingsMedia[cfgpstr].Lists[i].Name, listname) {
			return i
		}
	}
	return -1
}

func UpdateCfg(configIn []Conf) {
	for _, val := range configIn {
		key := val.Name
		//Cfg.Keys[key] = true
		if strings.HasPrefix(key, "general") {
			SettingsGeneral = val.Data.(GeneralConfig)
		}
		if strings.HasPrefix(key, "downloader_") {
			SettingsDownloader[val.Data.(DownloaderConfig).Name] = val.Data.(DownloaderConfig)
		}
		if strings.HasPrefix(key, logger.StrImdb) {
			SettingsImdb = val.Data.(ImdbConfig)
		}
		if strings.HasPrefix(key, "indexer") {
			SettingsIndexer[val.Data.(IndexersConfig).Name] = val.Data.(IndexersConfig)
		}
		if strings.HasPrefix(key, "list") {
			SettingsList[val.Data.(ListsConfig).Name] = val.Data.(ListsConfig)
		}
		if strings.HasPrefix(key, logger.StrSerie) {
			SettingsMedia["serie_"+val.Data.(MediaTypeConfig).Name] = val.Data.(MediaTypeConfig)
		}
		if strings.HasPrefix(key, logger.StrMovie) {
			SettingsMedia["movie_"+val.Data.(MediaTypeConfig).Name] = val.Data.(MediaTypeConfig)
		}
		if strings.HasPrefix(key, "notification") {
			SettingsNotification[val.Data.(NotificationConfig).Name] = val.Data.(NotificationConfig)
		}
		if strings.HasPrefix(key, "path") {
			SettingsPath[val.Data.(PathsConfig).Name] = val.Data.(PathsConfig)
		}
		if strings.HasPrefix(key, "quality") {
			SettingsQuality[val.Data.(QualityConfig).Name] = val.Data.(QualityConfig)
		}
		if strings.HasPrefix(key, "regex") {
			SettingsRegex[val.Data.(RegexConfigIn).Name] = RegexConfig{RegexConfigIn: val.Data.(RegexConfigIn)}
		}
		if strings.HasPrefix(key, "scheduler") {
			SettingsScheduler[val.Data.(SchedulerConfig).Name] = val.Data.(SchedulerConfig)
		}
	}
}

func GetCfgAll() map[string]interface{} {
	q := make(map[string]interface{})
	q["general"] = SettingsGeneral
	q["imdb"] = SettingsImdb
	for key := range SettingsMedia {
		q[key] = SettingsMedia[key]
	}
	for key := range SettingsDownloader {
		q[key] = SettingsDownloader[key]
	}
	for key := range SettingsIndexer {
		q[key] = SettingsIndexer[key]
	}
	for key := range SettingsList {
		q[key] = SettingsList[key]
	}
	for key := range SettingsNotification {
		q[key] = SettingsNotification[key]
	}
	for key := range SettingsPath {
		q[key] = SettingsPath[key]
	}
	for key := range SettingsQuality {
		q[key] = SettingsQuality[key]
	}
	for key := range SettingsRegex {
		q[key] = SettingsRegex[key]
	}
	for key := range SettingsScheduler {
		q[key] = SettingsScheduler[key]
	}
	return q
}
func UpdateCfgEntry(configIn Conf) {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})

	key := configIn.Name
	//Cfg.Keys[key] = true

	if strings.HasPrefix(key, "general") {
		SettingsGeneral = configIn.Data.(GeneralConfig)
	}
	if strings.HasPrefix(key, "downloader_") {
		SettingsDownloader[configIn.Data.(DownloaderConfig).Name] = configIn.Data.(DownloaderConfig)
	}
	if strings.HasPrefix(key, logger.StrImdb) {
		SettingsImdb = configIn.Data.(ImdbConfig)
	}
	if strings.HasPrefix(key, "indexer") {
		SettingsIndexer[configIn.Data.(IndexersConfig).Name] = configIn.Data.(IndexersConfig)
	}
	if strings.HasPrefix(key, "list") {
		SettingsList[configIn.Data.(ListsConfig).Name] = configIn.Data.(ListsConfig)
	}
	if strings.HasPrefix(key, logger.StrSerie) {
		SettingsMedia["serie_"+configIn.Data.(MediaTypeConfig).Name] = configIn.Data.(MediaTypeConfig)
	}
	if strings.HasPrefix(key, logger.StrMovie) {
		SettingsMedia["movie_"+configIn.Data.(MediaTypeConfig).Name] = configIn.Data.(MediaTypeConfig)
	}
	if strings.HasPrefix(key, "notification") {
		SettingsNotification[configIn.Data.(NotificationConfig).Name] = configIn.Data.(NotificationConfig)
	}
	if strings.HasPrefix(key, "path") {
		SettingsPath[configIn.Data.(PathsConfig).Name] = configIn.Data.(PathsConfig)
	}
	if strings.HasPrefix(key, "quality") {
		SettingsQuality[configIn.Data.(QualityConfig).Name] = configIn.Data.(QualityConfig)
	}
	if strings.HasPrefix(key, "regex") {
		SettingsRegex[configIn.Data.(RegexConfigIn).Name] = RegexConfig{RegexConfigIn: configIn.Data.(RegexConfigIn)}
	}
	if strings.HasPrefix(key, "scheduler") {
		SettingsScheduler[configIn.Data.(SchedulerConfig).Name] = configIn.Data.(SchedulerConfig)
	}
	if strings.HasPrefix(key, "trakt_token") {
		logger.GlobalCache.Set(key, configIn.Data.(oauth2.Token), 0, false)
		configDB.Set(key, configIn.Data)
	}
	configDB.Close()
}
func DeleteCfgEntry(name string) {
	logger.GlobalCache.Delete(name)
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})

	//delete(Cfg.Keys, name)

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

	configDB.Delete(name)
	configDB.Close()
}

func ClearCfg() {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})

	configDB.DeleteFile()
	SettingsDownloader = make(map[string]DownloaderConfig)
	SettingsIndexer = make(map[string]IndexersConfig)
	SettingsList = make(map[string]ListsConfig)
	SettingsMedia = make(map[string]MediaTypeConfig)
	SettingsNotification = make(map[string]NotificationConfig)
	SettingsPath = make(map[string]PathsConfig)
	SettingsQuality = make(map[string]QualityConfig)
	SettingsRegex = make(map[string]RegexConfig)
	SettingsScheduler = make(map[string]SchedulerConfig)

	var dataconfig []MediaDataConfig
	dataconfig = append(dataconfig, MediaDataConfig{TemplatePath: "initial"})
	var dataimportconfig []MediaDataImportConfig
	dataimportconfig = append(dataimportconfig, MediaDataImportConfig{TemplatePath: "initial"})
	var noticonfig []MediaNotificationConfig
	noticonfig = append(noticonfig, MediaNotificationConfig{MapNotification: "initial"})
	var listsconfig []MediaListsConfig
	listsconfig = append(listsconfig, MediaListsConfig{TemplateList: "initial", TemplateQuality: "initial", TemplateScheduler: "Default"})

	var quindconfig []QualityIndexerConfig
	quindconfig = append(quindconfig, QualityIndexerConfig{TemplateIndexer: "initial", TemplateDownloader: "initial", TemplateRegex: "initial", TemplatePathNzb: "initial"})
	var qureoconfig []QualityReorderConfig
	qureoconfig = append(qureoconfig, QualityReorderConfig{})

	//Cfg.Keys = map[string]bool{"general": true, logger.StrImdb: true, "scheduler_Default": true, "downloader_initial": true, "indexer_initial": true, "list_initial": true, "movie_initial": true, "serie_initial": true, "notification_initial": true, "path_initial": true, "quality_initial": true, "regex_initial": true}

	SettingsGeneral = GeneralConfig{
		LogLevel:            "Info",
		DBLogLevel:          "Info",
		LogFileCount:        5,
		LogFileSize:         5,
		LogCompress:         false,
		WebAPIKey:           "mysecure",
		WebPort:             "9090",
		WorkerMetadata:      1,
		WorkerFiles:         1,
		WorkerParse:         1,
		WorkerSearch:        1,
		WorkerIndexer:       1,
		ConcurrentScheduler: 1,
		Omdblimiterseconds:  1,
		Omdblimitercalls:    1,
		Tmdblimiterseconds:  1,
		Tmdblimitercalls:    1,
		Traktlimiterseconds: 1,
		Traktlimitercalls:   1,
		Tvdblimiterseconds:  1,
		Tvdblimitercalls:    1,
	}
	SettingsScheduler["scheduler_Default"] = SchedulerConfig{
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
	}
	SettingsDownloader["downloader_initial"] = DownloaderConfig{Name: "initial", DlType: "drone"}

	SettingsImdb = ImdbConfig{Indexedtypes: []string{logger.StrMovie}, Indexedlanguages: []string{"US", "UK", "\\N"}}
	SettingsIndexer["indexer_initial"] = IndexersConfig{Name: "initial", IndexerType: "newznab", Limitercalls: 1, Limiterseconds: 1, MaxRssEntries: 100, RssEntriesloop: 2}
	SettingsList["list_initial"] = ListsConfig{Name: "initial", ListType: "traktmovieanticipated", Limit: 20}
	SettingsMedia["movie_initial"] = MediaTypeConfig{Name: "initial", TemplateQuality: "initial", TemplateScheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig}
	SettingsMedia["serie_initial"] = MediaTypeConfig{Name: "initial", TemplateQuality: "initial", TemplateScheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig}
	SettingsNotification["notification_initial"] = NotificationConfig{Name: "initial", NotificationType: "csv"}
	SettingsPath["path_initial"] = PathsConfig{Name: "initial", AllowedVideoExtensions: []string{".avi", ".mkv", ".mp4"}, AllowedOtherExtensions: []string{".idx", ".sub", ".srt"}}
	SettingsQuality["quality_initial"] = QualityConfig{Name: "initial", QualityReorder: qureoconfig, Indexer: quindconfig}
	SettingsRegex["regex_initial"] = RegexConfig{RegexConfigIn: RegexConfigIn{Name: "initial"}}

	configDB.Close()
}
func WriteCfg() {

	var bla MainConfigOut

	bla.General = SettingsGeneral
	bla.Imdbindexer = SettingsImdb
	for _, cfgdata := range SettingsDownloader {
		bla.Downloader = append(bla.Downloader, cfgdata)
	}
	for _, cfgdata := range SettingsIndexer {
		bla.Indexers = append(bla.Indexers, cfgdata)
	}
	for _, cfgdata := range SettingsList {
		bla.Lists = append(bla.Lists, cfgdata)
	}
	for idxp := range SettingsMedia {
		if !strings.HasPrefix(SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		bla.Media.Series = append(bla.Media.Series, SettingsMedia[idxp])
	}
	for idxp := range SettingsMedia {
		if !strings.HasPrefix(SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}
		bla.Media.Movies = append(bla.Media.Movies, SettingsMedia[idxp])
	}
	for _, cfgdata := range SettingsNotification {
		bla.Notification = append(bla.Notification, cfgdata)
	}
	for _, cfgdata := range SettingsPath {
		bla.Paths = append(bla.Paths, cfgdata)
	}
	for _, cfgdata := range SettingsQuality {
		bla.Quality = append(bla.Quality, cfgdata)
	}
	for _, cfgdata := range SettingsRegex {
		bla.Regex = append(bla.Regex, RegexConfigIn{Name: cfgdata.Name, Required: cfgdata.Required, Rejected: cfgdata.Rejected})
	}
	for _, cfgdata := range SettingsScheduler {
		bla.Scheduler = append(bla.Scheduler, cfgdata)
	}

	cnt, err := toml.Marshal(bla)
	if err != nil {
		fmt.Println("Error loading config. " + err.Error())
	}
	os.WriteFile(configfile, cnt, 0777)
}

func QualityIndexerByQualityAndTemplateFirTemplateAndSize(quality string, indexerTemplate string) (string, bool) {
	for idx := range SettingsQuality["quality_"+quality].Indexer {
		if strings.EqualFold(SettingsQuality["quality_"+quality].Indexer[idx].TemplateIndexer, indexerTemplate) {
			return SettingsQuality["quality_"+quality].Indexer[idx].TemplateRegex, SettingsQuality["quality_"+quality].Indexer[idx].SkipEmptySize
		}
	}
	for idx := range SettingsList {
		if strings.EqualFold(SettingsList[idx].Name, indexerTemplate) {
			return SettingsQuality["quality_"+quality].Indexer[0].TemplateRegex, SettingsQuality["quality_"+quality].Indexer[0].SkipEmptySize
		}
	}
	return "", false
}

func QualityIndexerByQualityAndTemplateGetFieldHistoryCheckTitle(quality string, indexerTemplate string) bool {
	for idx := range SettingsQuality["quality_"+quality].Indexer {
		if strings.EqualFold(SettingsQuality["quality_"+quality].Indexer[idx].TemplateIndexer, indexerTemplate) {
			return SettingsQuality["quality_"+quality].Indexer[idx].HistoryCheckTitle
		}
	}
	return false
}
func QualityIndexerByQualityAndTemplateGetFieldString(quality string, indexerTemplate string, field string) string {
	for idx := range SettingsQuality["quality_"+quality].Indexer {
		if strings.EqualFold(SettingsQuality["quality_"+quality].Indexer[idx].TemplateIndexer, indexerTemplate) {
			switch field {
			case "TemplateRegex":
				return SettingsQuality["quality_"+quality].Indexer[idx].TemplateRegex
			case "AdditionalQueryParams":
				return SettingsQuality["quality_"+quality].Indexer[idx].AdditionalQueryParams
			case "CategoriesIndexer":
				return SettingsQuality["quality_"+quality].Indexer[idx].CategoriesIndexer
				//default:
				//	return reflect.ValueOf(Cfg.Quality[quality].Indexer[idx]).FieldByName(field).String()
			}
		}
	}
	return ""
}
