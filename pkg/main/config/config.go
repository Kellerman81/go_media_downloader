// koanf_api
package config

import (
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/pelletier/go-toml/v2"
	"github.com/recoilme/pudge"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

//Series Config

type MainSerieConfig struct {
	Global GlobalSerieConfig `koanf:"global" toml:"global"`
	Serie  []SerieConfig     `koanf:"series" toml:"series"`
}

func (s *MainSerieConfig) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		s.Serie = nil
		logger.ClearVar(&s)
	}
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

type MainConfigMap struct {
	Keys         map[string]bool
	General      GeneralConfig
	Imdbindexer  ImdbConfig
	Media        map[string]MediaTypeConfig
	Movies       map[string]MediaTypeConfig
	Series       map[string]MediaTypeConfig
	Downloader   map[string]DownloaderConfig
	Lists        map[string]ListsConfig
	Indexers     map[string]IndexersConfig
	Paths        map[string]PathsConfig
	Notification map[string]NotificationConfig
	Regex        map[string]RegexConfig
	Quality      map[string]QualityConfig
	Scheduler    map[string]SchedulerConfig
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
	WorkerDefault                      int      `koanf:"worker_default" toml:"worker_default"`
	WorkerMetadata                     int      `koanf:"worker_metadata" toml:"worker_metadata"`
	WorkerFiles                        int      `koanf:"worker_files" toml:"worker_files"`
	WorkerParse                        int      `koanf:"worker_parse" toml:"worker_parse"`
	WorkerSearch                       int      `koanf:"worker_search" toml:"worker_search"`
	WorkerIndexer                      int      `koanf:"worker_indexer" toml:"worker_indexer"`
	OmdbApiKey                         string   `koanf:"omdb_apikey" toml:"omdb_apikey"`
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
	WebApiKey                          string   `koanf:"webapikey" toml:"webapikey"`
	ConcurrentScheduler                int      `koanf:"concurrent_scheduler" toml:"concurrent_scheduler"`
	TheMovieDBApiKey                   string   `koanf:"themoviedb_apikey" toml:"themoviedb_apikey"`
	TraktClientId                      string   `koanf:"trakt_client_id" toml:"trakt_client_id"`
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
	Name                      string                      `koanf:"name" toml:"name"`
	NamePrefix                string                      `koanf:"name" toml:"nameprefix"`
	DefaultQuality            string                      `koanf:"default_quality" toml:"default_quality"`
	DefaultResolution         string                      `koanf:"default_resolution" toml:"default_resolution"`
	Naming                    string                      `koanf:"naming" toml:"naming"`
	NamingIdentifier          string                      `koanf:"naming_identifier" toml:"naming_identifier"`
	Template_quality          string                      `koanf:"template_quality" toml:"template_quality"`
	Template_scheduler        string                      `koanf:"template_scheduler" toml:"template_scheduler"`
	Metadata_language         string                      `koanf:"metadata_language" toml:"metadata_language"`
	Metadata_title_languages  []string                    `koanf:"metadata_title_languages" toml:"metadata_title_languages"`
	Metadata_source           string                      `koanf:"metadata_source" toml:"metadata_source"`
	Structure                 bool                        `koanf:"structure" toml:"structure"`
	Searchmissing_incremental int                         `koanf:"search_missing_incremental" toml:"search_missing_incremental"`
	Searchupgrade_incremental int                         `koanf:"search_upgrade_incremental" toml:"search_upgrade_incremental"`
	Data                      []MediaDataConfig           `koanf:"data" toml:"data"`
	DataImport                []MediaDataImportConfig     `koanf:"data_import" toml:"data_import"`
	Lists                     []MediaListsConfig          `koanf:"lists" toml:"lists"`
	Notification              []MediaNotificationConfig   `koanf:"notification" toml:"notification"`
	ListsInterface            []interface{}               `koanf:"-" toml:"-"`
	QualatiesInterface        []interface{}               `koanf:"-" toml:"-"`
	ListsMap                  map[string]MediaListsConfig `koanf:"-" toml:"-"`
}

type MediaDataConfig struct {
	Template_path string `koanf:"template_path" toml:"template_path"`
	AddFound      bool   `koanf:"add_found" toml:"add_found"`
	AddFoundList  string `koanf:"add_found_list" toml:"add_found_list"`
}

type MediaDataImportConfig struct {
	Template_path string `koanf:"template_path" toml:"template_path"`
}

type MediaListsConfig struct {
	Name               string   `koanf:"name" toml:"name"`
	Template_list      string   `koanf:"template_list" toml:"template_list"`
	Template_quality   string   `koanf:"template_quality" toml:"template_quality"`
	Template_scheduler string   `koanf:"template_scheduler" toml:"template_scheduler"`
	Ignore_map_lists   []string `koanf:"ignore_template_lists" toml:"ignore_template_lists"`
	Replace_map_lists  []string `koanf:"replace_template_lists" toml:"replace_template_lists"`
	Enabled            bool     `koanf:"enabled" toml:"enabled"`
	Addfound           bool     `koanf:"add_found" toml:"add_found"`
}

type MediaNotificationConfig struct {
	Map_notification string `koanf:"template_notification" toml:"template_notification"`
	Event            string `koanf:"event" toml:"event"`
	Title            string `koanf:"title" toml:"title"`
	Message          string `koanf:"message" toml:"message"`
	ReplacedPrefix   string `koanf:"replaced_prefix" toml:"replaced_prefix"`
}

type DownloaderConfig struct {
	Name                  string `koanf:"name" toml:"name"`
	DlType                string `koanf:"type" toml:"type"`
	Hostname              string `koanf:"hostname" toml:"hostname"`
	Port                  int    `koanf:"port" toml:"port"`
	Username              string `koanf:"username" toml:"username"`
	Password              string `koanf:"password" toml:"password"`
	AddPaused             bool   `koanf:"add_paused" toml:"add_paused"`
	DelugeDlTo            string `koanf:"deluge_dl_to" toml:"deluge_dl_to"`
	DelugeMoveAfter       bool   `koanf:"deluge_move_after" toml:"deluge_move_after"`
	DelugeMoveTo          string `koanf:"deluge_move_to" toml:"deluge_move_to"`
	Priority              int    `koanf:"priority" toml:"priority"`
	Enabled               bool   `koanf:"enabled" toml:"enabled"`
	Autoredownloadfailed  bool   `koanf:"auto_redownload_failed" toml:"auto_redownload_failed"`
	Removefaileddownloads bool   `koanf:"remove_failed_downloads" toml:"remove_failed_downloads"`
}

type ListsConfig struct {
	Name               string   `koanf:"name" toml:"name"`
	ListType           string   `koanf:"type" toml:"type"`
	Url                string   `koanf:"url" toml:"url"`
	Enabled            bool     `koanf:"enabled" toml:"enabled"`
	Series_config_file string   `koanf:"series_config_file" toml:"series_config_file"`
	TraktUsername      string   `koanf:"trakt_username" toml:"trakt_username"`
	TraktListName      string   `koanf:"trakt_listname" toml:"trakt_listname"`
	TraktListType      string   `koanf:"trakt_listtype" toml:"trakt_listtype"`
	Limit              int      `koanf:"limit" toml:"limit"`
	MinVotes           int      `koanf:"min_votes" toml:"min_votes"`
	MinRating          float32  `koanf:"min_rating" toml:"min_rating"`
	Excludegenre       []string `koanf:"exclude_genre" toml:"exclude_genre"`
	Includegenre       []string `koanf:"include_genre" toml:"include_genre"`
}

type IndexersConfig struct {
	Name                   string `koanf:"name" toml:"name"`
	IndexerType            string `koanf:"type" toml:"type"`
	Url                    string `koanf:"url" toml:"url"`
	Apikey                 string `koanf:"apikey" toml:"apikey"`
	Userid                 string `koanf:"userid" toml:"userid"`
	Enabled                bool   `koanf:"enabled" toml:"enabled"`
	Rssenabled             bool   `koanf:"rss_enabled" toml:"rss_enabled"`
	Addquotesfortitlequery bool   `koanf:"add_quotes_for_title_query" toml:"add_quotes_for_title_query"`
	MaxRssEntries          int    `koanf:"max_rss_entries" toml:"max_rss_entries"`
	RssEntriesloop         int    `koanf:"rss_entries_loop" toml:"rss_entries_loop"`
	RssDownloadAll         bool   `koanf:"rss_downlood_all" toml:"rss_downlood_all"`
	OutputAsJson           bool   `koanf:"output_as_json" toml:"output_as_json"`
	Customapi              string `koanf:"custom_api" toml:"custom_api"`
	Customurl              string `koanf:"custom_url" toml:"custom_url"`
	Customrssurl           string `koanf:"custom_rss_url" toml:"custom_rss_url"`
	Customrsscategory      string `koanf:"custom_rss_category" toml:"custom_rss_category"`
	Limitercalls           int    `koanf:"limiter_calls" toml:"limiter_calls"`
	Limiterseconds         int    `koanf:"limiter_seconds" toml:"limiter_seconds"`
	LimitercallsDaily      int    `koanf:"limiter_calls_daily" toml:"limiter_calls_daily"`
	MaxAge                 int    `koanf:"max_age" toml:"max_age"`
	DisableTLSVerify       bool   `koanf:"disable_tls_verify" toml:"disable_tls_verify"`
	TimeoutSeconds         int    `koanf:"timeout_seconds" toml:"timeout_seconds"`
}

type PathsConfig struct {
	Name                                string   `koanf:"name" toml:"name"`
	Path                                string   `koanf:"path" toml:"path"`
	AllowedVideoExtensions              []string `koanf:"allowed_video_extensions" toml:"allowed_video_extensions"`
	AllowedOtherExtensions              []string `koanf:"allowed_other_extensions" toml:"allowed_other_extensions"`
	AllowedVideoExtensionsNoRename      []string `koanf:"allowed_video_extensions_no_rename" toml:"allowed_video_extensions_no_rename"`
	AllowedOtherExtensionsNoRename      []string `koanf:"allowed_other_extensions_no_rename" toml:"allowed_other_extensions_no_rename"`
	AllowedVideoExtensionsLower         []string `koanf:"-" toml:"-"`
	AllowedVideoExtensionsNoRenameLower []string `koanf:"-" toml:"-"`
	Blocked                             []string `koanf:"blocked" toml:"blocked"`
	BlockedLower                        []string `koanf:"-" toml:"-"`
	Upgrade                             bool     `koanf:"upgrade" toml:"upgrade"`
	MinSize                             int      `koanf:"min_size" toml:"min_size"`
	MaxSize                             int      `koanf:"max_size" toml:"max_size"`
	MinVideoSize                        int      `koanf:"min_video_size" toml:"min_video_size"`
	CleanupsizeMB                       int      `koanf:"cleanup_size_mb" toml:"cleanup_size_mb"`
	Allowed_languages                   []string `koanf:"allowed_languages" toml:"allowed_languages"`
	Replacelower                        bool     `koanf:"replace_lower" toml:"replace_lower"`
	Usepresort                          bool     `koanf:"use_presort" toml:"use_presort"`
	PresortFolderPath                   string   `koanf:"presort_folder_path" toml:"presort_folder_path"`
	UpgradeScanInterval                 int      `koanf:"upgrade_scan_interval" toml:"upgrade_scan_interval"`
	MissingScanInterval                 int      `koanf:"missing_scan_interval" toml:"missing_scan_interval"`
	MissingScanReleaseDatePre           int      `koanf:"missing_scan_release_date_pre" toml:"missing_scan_release_date_pre"`
	Disallowed                          []string `koanf:"disallowed" toml:"disallowed"`
	DisallowedLower                     []string `koanf:"-" toml:"-"`
	DeleteWrongLanguage                 bool     `koanf:"delete_wrong_language" toml:"delete_wrong_language"`
	DeleteDisallowed                    bool     `koanf:"delete_disallowed" toml:"delete_disallowed"`
	CheckRuntime                        bool     `koanf:"check_runtime" toml:"check_runtime"`
	MaxRuntimeDifference                int      `koanf:"max_runtime_difference" toml:"max_runtime_difference"`
	DeleteWrongRuntime                  bool     `koanf:"delete_wrong_runtime" toml:"delete_wrong_runtime"`
	MoveReplaced                        bool     `koanf:"move_replaced" toml:"move_replaced"`
	MoveReplacedTargetPath              string   `koanf:"move_replaced_target_path" toml:"move_replaced_target_path"`
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

type RegexGroup struct {
	Name string
	Re   regexp.Regexp
}
type RegexConfig struct {
	RegexConfigIn
}

type QualityConfig struct {
	Name                          string                 `koanf:"name" toml:"name"`
	Wanted_resolution             []string               `koanf:"wanted_resolution" toml:"wanted_resolution"`
	Wanted_quality                []string               `koanf:"wanted_quality" toml:"wanted_quality"`
	Wanted_audio                  []string               `koanf:"wanted_audio" toml:"wanted_audio"`
	Wanted_codec                  []string               `koanf:"wanted_codec" toml:"wanted_codec"`
	Cutoff_resolution             string                 `koanf:"cutoff_resolution" toml:"cutoff_resolution"`
	Cutoff_quality                string                 `koanf:"cutoff_quality" toml:"cutoff_quality"`
	BackupSearchForTitle          bool                   `koanf:"backup_search_for_title" toml:"backup_search_for_title"`
	BackupSearchForAlternateTitle bool                   `koanf:"backup_search_for_alternate_title" toml:"backup_search_for_alternate_title"`
	ExcludeYearFromTitleSearch    bool                   `koanf:"exclude_year_from_title_search" toml:"exclude_year_from_title_search"`
	CheckUntilFirstFound          bool                   `koanf:"check_until_first_found" toml:"check_until_first_found"`
	CheckTitle                    bool                   `koanf:"check_title" toml:"check_title"`
	CheckYear                     bool                   `koanf:"check_year" toml:"check_year"`
	CheckYear1                    bool                   `koanf:"check_year1" toml:"check_year1"`
	TitleStripSuffixForSearch     []string               `koanf:"title_strip_suffix_for_search" toml:"title_strip_suffix_for_search"`
	TitleStripPrefixForSearch     []string               `koanf:"title_strip_prefix_for_search" toml:"title_strip_prefix_for_search"`
	QualityReorder                []QualityReorderConfig `koanf:"reorder" toml:"reorder"`
	Indexer                       []QualityIndexerConfig `koanf:"indexers" toml:"indexers"`
	Cutoff_priority               int
	UseForPriorityResolution      bool `koanf:"use_for_priority_resolution" toml:"use_for_priority_resolution"`
	UseForPriorityQuality         bool `koanf:"use_for_priority_quality" toml:"use_for_priority_quality"`
	UseForPriorityAudio           bool `koanf:"use_for_priority_audio" toml:"use_for_priority_audio"`
	UseForPriorityCodec           bool `koanf:"use_for_priority_codec" toml:"use_for_priority_codec"`
	UseForPriorityOther           bool `koanf:"use_for_priority_other" toml:"use_for_priority_other"`
	UseForPriorityMinDifference   int  `koanf:"use_for_priority_min_difference" toml:"use_for_priority_min_difference"`
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
	Template_indexer        string `koanf:"template_indexer" toml:"template_indexer"`
	Template_downloader     string `koanf:"template_downloader" toml:"template_downloader"`
	Template_regex          string `koanf:"template_regex" toml:"template_regex"`
	Template_path_nzb       string `koanf:"template_path_nzb" toml:"template_path_nzb"`
	Category_dowloader      string `koanf:"category_dowloader" toml:"category_dowloader"`
	Additional_query_params string `koanf:"additional_query_params" toml:"additional_query_params"`
	CustomQueryString       string `koanf:"custom_query_string" toml:"custom_query_string"`
	Skip_empty_size         bool   `koanf:"skip_empty_size" toml:"skip_empty_size"`
	History_check_title     bool   `koanf:"history_check_title" toml:"history_check_title"`
	Categories_indexer      string `koanf:"categories_indexer" toml:"categories_indexer"`
}

func (s *QualityIndexerConfig) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		logger.ClearVar(&s)
	}
}

type SchedulerConfig struct {
	Name                                string `koanf:"name" toml:"name"`
	Interval_imdb                       string `koanf:"interval_imdb" toml:"interval_imdb"`
	Interval_feeds                      string `koanf:"interval_feeds" toml:"interval_feeds"`
	Interval_feeds_refresh_series       string `koanf:"interval_feeds_refresh_series" toml:"interval_feeds_refresh_series"`
	Interval_feeds_refresh_movies       string `koanf:"interval_feeds_refresh_movies" toml:"interval_feeds_refresh_movies"`
	Interval_feeds_refresh_series_full  string `koanf:"interval_feeds_refresh_series_full" toml:"interval_feeds_refresh_series_full"`
	Interval_feeds_refresh_movies_full  string `koanf:"interval_feeds_refresh_movies_full" toml:"interval_feeds_refresh_movies_full"`
	Interval_indexer_missing            string `koanf:"interval_indexer_missing" toml:"interval_indexer_missing"`
	Interval_indexer_upgrade            string `koanf:"interval_indexer_upgrade" toml:"interval_indexer_upgrade"`
	Interval_indexer_missing_full       string `koanf:"interval_indexer_missing_full" toml:"interval_indexer_missing_full"`
	Interval_indexer_upgrade_full       string `koanf:"interval_indexer_upgrade_full" toml:"interval_indexer_upgrade_full"`
	Interval_indexer_missing_title      string `koanf:"interval_indexer_missing_title" toml:"interval_indexer_missing_title"`
	Interval_indexer_upgrade_title      string `koanf:"interval_indexer_upgrade_title" toml:"interval_indexer_upgrade_title"`
	Interval_indexer_missing_full_title string `koanf:"interval_indexer_missing_full_title" toml:"interval_indexer_missing_full_title"`
	Interval_indexer_upgrade_full_title string `koanf:"interval_indexer_upgrade_full_title" toml:"interval_indexer_upgrade_full_title"`
	Interval_indexer_rss                string `koanf:"interval_indexer_rss" toml:"interval_indexer_rss"`
	Interval_scan_data                  string `koanf:"interval_scan_data" toml:"interval_scan_data"`
	Interval_scan_data_missing          string `koanf:"interval_scan_data_missing" toml:"interval_scan_data_missing"`
	Interval_scan_data_flags            string `koanf:"interval_scan_data_flags" toml:"interval_scan_data_flags"`
	Interval_scan_dataimport            string `koanf:"interval_scan_data_import" toml:"interval_scan_data_import"`
	Interval_database_backup            string `koanf:"interval_database_backup" toml:"interval_database_backup"`
	Interval_database_check             string `koanf:"interval_database_check" toml:"interval_database_check"`
	Interval_indexer_rss_seasons        string `koanf:"interval_indexer_rss_seasons" toml:"interval_indexer_rss_seasons"`
	Cron_indexer_rss_seasons            string `koanf:"cron_indexer_rss_seasons" toml:"cron_indexer_rss_seasons"`
	Cron_imdb                           string `koanf:"cron_imdb" toml:"cron_imdb"`
	Cron_feeds                          string `koanf:"cron_feeds" toml:"cron_feeds"`
	Cron_feeds_refresh_series           string `koanf:"cron_feeds_refresh_series" toml:"cron_feeds_refresh_series"`
	Cron_feeds_refresh_movies           string `koanf:"cron_feeds_refresh_movies" toml:"cron_feeds_refresh_movies"`
	Cron_feeds_refresh_series_full      string `koanf:"cron_feeds_refresh_series_full" toml:"cron_feeds_refresh_series_full"`
	Cron_feeds_refresh_movies_full      string `koanf:"cron_feeds_refresh_movies_full" toml:"cron_feeds_refresh_movies_full"`
	Cron_indexer_missing                string `koanf:"cron_indexer_missing" toml:"cron_indexer_missing"`
	Cron_indexer_upgrade                string `koanf:"cron_indexer_upgrade" toml:"cron_indexer_upgrade"`
	Cron_indexer_missing_full           string `koanf:"cron_indexer_missing_full" toml:"cron_indexer_missing_full"`
	Cron_indexer_upgrade_full           string `koanf:"cron_indexer_upgrade_full" toml:"cron_indexer_upgrade_full"`
	Cron_indexer_missing_title          string `koanf:"cron_indexer_missing_title" toml:"cron_indexer_missing_title"`
	Cron_indexer_upgrade_title          string `koanf:"cron_indexer_upgrade_title" toml:"cron_indexer_upgrade_title"`
	Cron_indexer_missing_full_title     string `koanf:"cron_indexer_missing_full_title" toml:"cron_indexer_missing_full_title"`
	Cron_indexer_upgrade_full_title     string `koanf:"cron_indexer_upgrade_full_title" toml:"cron_indexer_upgrade_full_title"`
	Cron_indexer_rss                    string `koanf:"cron_indexer_rss" toml:"cron_indexer_rss"`
	Cron_scan_data                      string `koanf:"cron_scan_data" toml:"cron_scan_data"`
	Cron_scan_data_missing              string `koanf:"cron_scan_data_missing" toml:"cron_scan_data_missing"`
	Cron_scan_data_flags                string `koanf:"cron_scan_data_flags" toml:"cron_scan_data_flags"`
	Cron_scan_dataimport                string `koanf:"cron_scan_data_import" toml:"cron_scan_data_import"`
	Cron_database_backup                string `koanf:"cron_database_backup" toml:"cron_database_backup"`
	Cron_database_check                 string `koanf:"cron_database_check" toml:"cron_database_check"`
}

func Slepping(random bool, seconds int) {
	if random {
		rand.Seed(time.Now().UnixNano())
		n := rand.Intn(seconds) // n will be between 0 and 10
		logger.Log.GlobalLogger.Debug("Sleeping for", zap.Int("seconds", n+1))
		time.Sleep(time.Duration(1+n) * time.Second)
	} else {
		logger.Log.GlobalLogger.Debug("Sleeping for", zap.Int("seconds", seconds))
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

const configfile string = "./config/config.toml"

var Cfg MainConfigMap

func GetCfgFile() string {
	return configfile
}

func LoadCfgDB(f string) error {
	LoadCfgDataDB(f)

	return nil
}

func LoadCfgDataDB(parser string) {
	content, err := os.ReadFile(configfile)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
	}
	var results MainConfig
	toml.Unmarshal(content, &results)
	content = nil
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()
	pudge.BackupAll("")
	Cfg.Keys = make(map[string]bool)
	Cfg.Downloader = make(map[string]DownloaderConfig)
	Cfg.Indexers = make(map[string]IndexersConfig)
	Cfg.Lists = make(map[string]ListsConfig)
	Cfg.Media = make(map[string]MediaTypeConfig)
	Cfg.Movies = make(map[string]MediaTypeConfig)
	Cfg.Series = make(map[string]MediaTypeConfig)
	Cfg.Notification = make(map[string]NotificationConfig)
	Cfg.Paths = make(map[string]PathsConfig)
	Cfg.Quality = make(map[string]QualityConfig)
	Cfg.Regex = make(map[string]RegexConfig)
	Cfg.Scheduler = make(map[string]SchedulerConfig)

	Cfg.General = results.General
	if Cfg.General.WebApiKey != "" {
		Cfg.Keys["general"] = true
	}
	Cfg.Imdbindexer = results.Imdbindexer
	if len(Cfg.Imdbindexer.Indexedtypes) >= 1 {
		Cfg.Keys["imdb"] = true
	}

	for idx := range results.Downloader {
		Cfg.Downloader[results.Downloader[idx].Name] = results.Downloader[idx]
		Cfg.Keys["downloader_"+results.Downloader[idx].Name] = true
	}
	for idx := range results.Indexers {
		Cfg.Indexers[results.Indexers[idx].Name] = results.Indexers[idx]
		Cfg.Keys["indexer_"+results.Indexers[idx].Name] = true
	}
	for idx := range results.Lists {
		Cfg.Lists[results.Lists[idx].Name] = results.Lists[idx]
		Cfg.Keys["list_"+results.Lists[idx].Name] = true
	}

	for idx := range results.Notification {
		Cfg.Notification[results.Notification[idx].Name] = results.Notification[idx]
		Cfg.Keys["notification_"+results.Notification[idx].Name] = true
	}
	for idx := range results.Paths {
		results.Paths[idx].BlockedLower = logger.StringArrayToLower(results.Paths[idx].Blocked)
		results.Paths[idx].AllowedVideoExtensionsLower = logger.StringArrayToLower(results.Paths[idx].AllowedVideoExtensions)
		results.Paths[idx].AllowedVideoExtensionsNoRenameLower = logger.StringArrayToLower(results.Paths[idx].AllowedVideoExtensionsNoRenameLower)
		results.Paths[idx].DisallowedLower = logger.StringArrayToLower(results.Paths[idx].Disallowed)
		Cfg.Paths[results.Paths[idx].Name] = results.Paths[idx]
		Cfg.Keys["path_"+results.Paths[idx].Name] = true
	}
	for idx := range results.Quality {
		Cfg.Quality[results.Quality[idx].Name] = results.Quality[idx]
		Cfg.Keys["quality_"+results.Quality[idx].Name] = true
	}
	for idx := range results.Regex {
		Cfg.Regex[results.Regex[idx].Name] = results.Regex[idx]
		Cfg.Keys["regex_"+results.Regex[idx].Name] = true
		var generalCache RegexConfig
		generalCache.Name = results.Regex[idx].Name
		generalCache.Rejected = results.Regex[idx].Rejected
		generalCache.Required = results.Regex[idx].Required
		for idxreg := range results.Regex[idx].Rejected {
			if !RegexCheck(results.Regex[idx].Rejected[idxreg]) {
				reg, err := regexp.Compile(results.Regex[idx].Rejected[idxreg])
				if err == nil {
					logger.GlobalRegexCache.SetRegexp(results.Regex[idx].Rejected[idxreg], reg, 0)
				}
			}
		}
		for idxreg := range results.Regex[idx].Required {
			if !RegexCheck(results.Regex[idx].Required[idxreg]) {
				reg, err := regexp.Compile(results.Regex[idx].Required[idxreg])
				if err == nil {
					logger.GlobalRegexCache.SetRegexp(results.Regex[idx].Required[idxreg], reg, 0)
				}
			}
		}
	}
	for idx := range results.Scheduler {
		Cfg.Scheduler[results.Scheduler[idx].Name] = results.Scheduler[idx]
		Cfg.Keys["scheduler_"+results.Scheduler[idx].Name] = true
	}
	for idx := range results.Media.Movies {
		results.Media.Movies[idx].NamePrefix = "movie_" + results.Media.Movies[idx].Name
		results.Media.Movies[idx].ListsMap = make(map[string]MediaListsConfig)
		results.Media.Movies[idx].ListsInterface = make([]interface{}, len(results.Media.Movies[idx].Lists))
		for idx2 := range results.Media.Movies[idx].Lists {
			results.Media.Movies[idx].ListsInterface[idx2] = results.Media.Movies[idx].Lists[idx2].Name
			results.Media.Movies[idx].ListsMap[results.Media.Movies[idx].Lists[idx2].Name] = results.Media.Movies[idx].Lists[idx2]
		}
		results.Media.Movies[idx].QualatiesInterface = make([]interface{}, len(results.Media.Movies[idx].Lists))
		for idx2 := range results.Media.Movies[idx].Lists {
			results.Media.Movies[idx].QualatiesInterface[idx2] = results.Media.Movies[idx].Lists[idx2].Template_quality
		}

		Cfg.Movies[results.Media.Movies[idx].Name] = results.Media.Movies[idx]
		Cfg.Media["movie_"+results.Media.Movies[idx].Name] = results.Media.Movies[idx]
		Cfg.Keys["movie_"+results.Media.Movies[idx].Name] = true
	}
	for idx := range results.Media.Series {
		results.Media.Series[idx].NamePrefix = "serie_" + results.Media.Series[idx].Name
		results.Media.Series[idx].ListsMap = make(map[string]MediaListsConfig)
		results.Media.Series[idx].ListsInterface = make([]interface{}, len(results.Media.Series[idx].Lists))
		for idx2 := range results.Media.Series[idx].Lists {
			results.Media.Series[idx].ListsInterface[idx2] = results.Media.Series[idx].Lists[idx2].Name
			results.Media.Series[idx].ListsMap[results.Media.Series[idx].Lists[idx2].Name] = results.Media.Series[idx].Lists[idx2]
		}
		results.Media.Series[idx].QualatiesInterface = make([]interface{}, len(results.Media.Series[idx].Lists))
		for idx2 := range results.Media.Series[idx].Lists {
			results.Media.Series[idx].QualatiesInterface[idx2] = results.Media.Series[idx].Lists[idx2].Template_quality
		}
		Cfg.Series[results.Media.Series[idx].Name] = results.Media.Series[idx]
		Cfg.Media["serie_"+results.Media.Series[idx].Name] = results.Media.Series[idx]
		Cfg.Keys["serie_"+results.Media.Series[idx].Name] = true
	}
	//Get from DB and not config
	hastoken, _ := configDB.Has("trakt_token")
	if hastoken {
		var token oauth2.Token
		err = configDB.Get("trakt_token", &token)
		if err == nil {
			logger.GlobalConfigCache.Set("trakt_token", token, 0)
		}
	}
	results = MainConfig{}
}

func UpdateCfg(configIn []Conf) {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	for _, val := range configIn {
		key := val.Name
		Cfg.Keys[key] = true
		if strings.HasPrefix(key, "general") {
			Cfg.General = val.Data.(GeneralConfig)
		}
		if strings.HasPrefix(key, "downloader_") {
			Cfg.Downloader[val.Data.(DownloaderConfig).Name] = val.Data.(DownloaderConfig)
		}
		if strings.HasPrefix(key, "imdb") {
			Cfg.Imdbindexer = val.Data.(ImdbConfig)
		}
		if strings.HasPrefix(key, "indexer") {
			Cfg.Indexers[val.Data.(IndexersConfig).Name] = val.Data.(IndexersConfig)
		}
		if strings.HasPrefix(key, "list") {
			Cfg.Lists[val.Data.(ListsConfig).Name] = val.Data.(ListsConfig)
		}
		if strings.HasPrefix(key, "serie") {
			Cfg.Series[val.Data.(MediaTypeConfig).Name] = val.Data.(MediaTypeConfig)
			Cfg.Media["serie_"+val.Data.(MediaTypeConfig).Name] = val.Data.(MediaTypeConfig)
		}
		if strings.HasPrefix(key, "movie") {
			Cfg.Movies[val.Data.(MediaTypeConfig).Name] = val.Data.(MediaTypeConfig)
			Cfg.Media["movie_"+val.Data.(MediaTypeConfig).Name] = val.Data.(MediaTypeConfig)
		}
		if strings.HasPrefix(key, "notification") {
			Cfg.Notification[val.Data.(NotificationConfig).Name] = val.Data.(NotificationConfig)
		}
		if strings.HasPrefix(key, "path") {
			Cfg.Paths[val.Data.(PathsConfig).Name] = val.Data.(PathsConfig)
		}
		if strings.HasPrefix(key, "quality") {
			Cfg.Quality[val.Data.(QualityConfig).Name] = val.Data.(QualityConfig)
		}
		if strings.HasPrefix(key, "regex") {
			var tmpin RegexConfigIn
			var tmpout RegexConfig
			tmpin = val.Data.(RegexConfigIn)
			tmpout.Name = tmpin.Name
			tmpout.Rejected = tmpin.Rejected
			tmpout.Required = tmpin.Required
			Cfg.Regex[tmpout.Name] = tmpout
		}
		if strings.HasPrefix(key, "scheduler") {
			Cfg.Scheduler[val.Data.(SchedulerConfig).Name] = val.Data.(SchedulerConfig)
		}
	}
}

func UpdateCfgEntry(configIn Conf) {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	key := configIn.Name
	Cfg.Keys[key] = true

	if strings.HasPrefix(key, "general") {
		Cfg.General = configIn.Data.(GeneralConfig)
	}
	if strings.HasPrefix(key, "downloader_") {
		Cfg.Downloader[configIn.Data.(DownloaderConfig).Name] = configIn.Data.(DownloaderConfig)
	}
	if strings.HasPrefix(key, "imdb") {
		Cfg.Imdbindexer = configIn.Data.(ImdbConfig)
	}
	if strings.HasPrefix(key, "indexer") {
		Cfg.Indexers[configIn.Data.(IndexersConfig).Name] = configIn.Data.(IndexersConfig)
	}
	if strings.HasPrefix(key, "list") {
		Cfg.Lists[configIn.Data.(ListsConfig).Name] = configIn.Data.(ListsConfig)
	}
	if strings.HasPrefix(key, "serie") {
		Cfg.Series[configIn.Data.(MediaTypeConfig).Name] = configIn.Data.(MediaTypeConfig)
		Cfg.Media["serie_"+configIn.Data.(MediaTypeConfig).Name] = configIn.Data.(MediaTypeConfig)
	}
	if strings.HasPrefix(key, "movie") {
		Cfg.Movies[configIn.Data.(MediaTypeConfig).Name] = configIn.Data.(MediaTypeConfig)
		Cfg.Media["movie_"+configIn.Data.(MediaTypeConfig).Name] = configIn.Data.(MediaTypeConfig)
	}
	if strings.HasPrefix(key, "notification") {
		Cfg.Notification[configIn.Data.(NotificationConfig).Name] = configIn.Data.(NotificationConfig)
	}
	if strings.HasPrefix(key, "path") {
		Cfg.Paths[configIn.Data.(PathsConfig).Name] = configIn.Data.(PathsConfig)
	}
	if strings.HasPrefix(key, "quality") {
		Cfg.Quality[configIn.Data.(QualityConfig).Name] = configIn.Data.(QualityConfig)
	}
	if strings.HasPrefix(key, "regex") {
		var tmpin RegexConfigIn
		var tmpout RegexConfig
		tmpin = configIn.Data.(RegexConfigIn)
		tmpout.Name = tmpin.Name
		tmpout.Rejected = tmpin.Rejected
		tmpout.Required = tmpin.Required
		Cfg.Regex[tmpout.Name] = tmpout
	}
	if strings.HasPrefix(key, "scheduler") {
		Cfg.Scheduler[configIn.Data.(SchedulerConfig).Name] = configIn.Data.(SchedulerConfig)
	}
	if strings.HasPrefix(key, "trakt_token") {
		logger.GlobalConfigCache.Set(key, configIn.Data.(oauth2.Token), 0)
		configDB.Set(key, configIn.Data)
	}
}
func DeleteCfgEntry(name string) {
	logger.GlobalConfigCache.Delete(name)
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	delete(Cfg.Keys, name)

	if strings.HasPrefix(name, "general") {
		Cfg.General = GeneralConfig{}
	}
	if strings.HasPrefix(name, "downloader_") {
		delete(Cfg.Downloader, name)
	}
	if strings.HasPrefix(name, "imdb") {
		Cfg.Imdbindexer = ImdbConfig{}
	}
	if strings.HasPrefix(name, "indexer") {
		delete(Cfg.Indexers, name)
	}
	if strings.HasPrefix(name, "list") {
		delete(Cfg.Lists, name)
	}
	if strings.HasPrefix(name, "serie") {
		delete(Cfg.Media, name)
		delete(Cfg.Series, strings.Replace(name, "serie_", "", 1))
	}
	if strings.HasPrefix(name, "movie") {
		delete(Cfg.Media, name)
		delete(Cfg.Movies, strings.Replace(name, "movie_", "", 1))
	}
	if strings.HasPrefix(name, "notification") {
		delete(Cfg.Notification, name)
	}
	if strings.HasPrefix(name, "path") {
		delete(Cfg.Paths, name)
	}
	if strings.HasPrefix(name, "quality") {
		delete(Cfg.Quality, name)
	}
	if strings.HasPrefix(name, "regex") {
		delete(Cfg.Regex, name)
	}
	if strings.HasPrefix(name, "scheduler") {
		delete(Cfg.Scheduler, name)
	}

	configDB.Delete(name)
}

func ClearCfg() {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	configDB.DeleteFile()

	var dataconfig []MediaDataConfig
	dataconfig = append(dataconfig, MediaDataConfig{Template_path: "initial"})
	var dataimportconfig []MediaDataImportConfig
	dataimportconfig = append(dataimportconfig, MediaDataImportConfig{Template_path: "initial"})
	var noticonfig []MediaNotificationConfig
	noticonfig = append(noticonfig, MediaNotificationConfig{Map_notification: "initial"})
	var listsconfig []MediaListsConfig
	listsconfig = append(listsconfig, MediaListsConfig{Template_list: "initial", Template_quality: "initial", Template_scheduler: "Default"})

	var quindconfig []QualityIndexerConfig
	quindconfig = append(quindconfig, QualityIndexerConfig{Template_indexer: "initial", Template_downloader: "initial", Template_regex: "initial", Template_path_nzb: "initial"})
	var qureoconfig []QualityReorderConfig
	qureoconfig = append(qureoconfig, QualityReorderConfig{})

	Cfg.Keys = map[string]bool{"general": true, "imdb": true, "scheduler_Default": true, "downloader_initial": true, "indexer_initial": true, "list_initial": true, "movie_initial": true, "serie_initial": true, "notification_initial": true, "path_initial": true, "quality_initial": true, "regex_initial": true}
	Cfg = MainConfigMap{
		General: GeneralConfig{
			LogLevel:            "Info",
			DBLogLevel:          "Info",
			LogFileCount:        5,
			LogFileSize:         5,
			LogCompress:         false,
			WebApiKey:           "mysecure",
			WebPort:             "9090",
			WorkerDefault:       1,
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
		},
		Scheduler: map[string]SchedulerConfig{"scheduler_Default": {
			Name:                          "Default",
			Interval_imdb:                 "3d",
			Interval_feeds:                "1d",
			Interval_feeds_refresh_series: "1d",
			Interval_feeds_refresh_movies: "1d",
			Interval_indexer_missing:      "40m",
			Interval_indexer_upgrade:      "60m",
			Interval_indexer_rss:          "15m",
			Interval_scan_data:            "1h",
			Interval_scan_data_missing:    "1d",
			Interval_scan_dataimport:      "60m",
		}},
		Downloader:  map[string]DownloaderConfig{"downloader_initial": {Name: "initial", DlType: "drone"}},
		Imdbindexer: ImdbConfig{Indexedtypes: []string{"movie"}, Indexedlanguages: []string{"US", "UK", "\\N"}},
		Indexers:    map[string]IndexersConfig{"indexer_initial": {Name: "initial", IndexerType: "newznab", Limitercalls: 1, Limiterseconds: 1, MaxRssEntries: 100, RssEntriesloop: 2}},
		Lists:       map[string]ListsConfig{"list_initial": {Name: "initial", ListType: "traktmovieanticipated", Limit: 20}},
		Movies:      map[string]MediaTypeConfig{"initial": {Name: "initial", Template_quality: "initial", Template_scheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig}},
		Series:      map[string]MediaTypeConfig{"initial": {Name: "initial", Template_quality: "initial", Template_scheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig}},
		Media: map[string]MediaTypeConfig{"movie_initial": {Name: "initial", Template_quality: "initial", Template_scheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig},
			"serie_initial": {Name: "initial", Template_quality: "initial", Template_scheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig}},
		Notification: map[string]NotificationConfig{"notification_initial": {Name: "initial", NotificationType: "csv"}},
		Paths:        map[string]PathsConfig{"path_initial": {Name: "initial", AllowedVideoExtensions: []string{".avi", ".mkv", ".mp4"}, AllowedOtherExtensions: []string{".idx", ".sub", ".srt"}}},
		Quality:      map[string]QualityConfig{"quality_initial": {Name: "initial", QualityReorder: qureoconfig, Indexer: quindconfig}},
		Regex:        map[string]RegexConfig{"regex_initial": {RegexConfigIn: RegexConfigIn{Name: "initial"}}},
	}

}
func WriteCfg() {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	var bla MainConfigOut
	bla.General = Cfg.General
	bla.Imdbindexer = Cfg.Imdbindexer
	for idx := range Cfg.Downloader {
		bla.Downloader = append(bla.Downloader, Cfg.Downloader[idx])
	}
	for idx := range Cfg.Indexers {
		bla.Indexers = append(bla.Indexers, Cfg.Indexers[idx])
	}
	for idx := range Cfg.Lists {
		bla.Lists = append(bla.Lists, Cfg.Lists[idx])
	}
	for idx := range Cfg.Series {
		bla.Media.Series = append(bla.Media.Series, Cfg.Series[idx])
	}
	for idx := range Cfg.Movies {
		bla.Media.Movies = append(bla.Media.Movies, Cfg.Movies[idx])
	}
	for idx := range Cfg.Notification {
		bla.Notification = append(bla.Notification, Cfg.Notification[idx])
	}
	for idx := range Cfg.Paths {
		bla.Paths = append(bla.Paths, Cfg.Paths[idx])
	}
	for idx := range Cfg.Quality {
		bla.Quality = append(bla.Quality, Cfg.Quality[idx])
	}
	for idx := range Cfg.Regex {
		bla.Regex = append(bla.Regex, RegexConfigIn{Name: Cfg.Regex[idx].Name, Required: Cfg.Regex[idx].Required, Rejected: Cfg.Regex[idx].Rejected})
	}
	for idx := range Cfg.Scheduler {
		bla.Scheduler = append(bla.Scheduler, Cfg.Scheduler[idx])
	}

	cnt, err := toml.Marshal(bla)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
	}
	os.WriteFile(configfile, cnt, 0777)
}

func QualityIndexerByQualityAndTemplate(quality string, indexerTemplate string) *QualityIndexerConfig {
	if _, test := Cfg.Quality[quality]; test {
		for idx := range Cfg.Quality[quality].Indexer {
			if Cfg.Quality[quality].Indexer[idx].Template_indexer == indexerTemplate {
				return &Cfg.Quality[quality].Indexer[idx]
			}
		}
	}
	return nil
}

func QualityIndexerByQualityAndTemplateGetFieldBool(quality string, indexerTemplate string, field string) bool {
	if _, test := Cfg.Quality[quality]; test {
		for idx := range Cfg.Quality[quality].Indexer {
			if Cfg.Quality[quality].Indexer[idx].Template_indexer == indexerTemplate {
				switch field {
				case "History_check_title":
					return Cfg.Quality[quality].Indexer[idx].History_check_title
				default:
					return reflect.ValueOf(Cfg.Quality[quality].Indexer[idx]).FieldByName(field).Bool()
				}
			}
		}
	}
	return false
}
func QualityIndexerByQualityAndTemplateGetFieldString(quality string, indexerTemplate string, field string) string {
	if _, test := Cfg.Quality[quality]; test {
		for idx := range Cfg.Quality[quality].Indexer {
			if Cfg.Quality[quality].Indexer[idx].Template_indexer == indexerTemplate {
				switch field {
				case "Template_regex":
					return Cfg.Quality[quality].Indexer[idx].Template_regex
				case "Additional_query_params":
					return Cfg.Quality[quality].Indexer[idx].Additional_query_params
				case "Categories_indexer":
					return Cfg.Quality[quality].Indexer[idx].Categories_indexer
				default:
					return reflect.ValueOf(Cfg.Quality[quality].Indexer[idx]).FieldByName(field).String()
				}
			}
		}
	}
	return ""
}

func (indexer *QualityIndexerConfig) Filter_size_nzbs(cfg string, title string, size int64) bool {
	for idx := range Cfg.Media[cfg].DataImport {

		if indexer.Skip_empty_size && size == 0 {
			logger.Log.GlobalLogger.Debug("Skipped - Size missing", zap.String("title", title))
			return true
		}
		if !ConfigCheck("path_" + Cfg.Media[cfg].DataImport[idx].Template_path) {
			return false
		}

		if Cfg.Paths[Cfg.Media[cfg].DataImport[idx].Template_path].MinSize != 0 {
			if size < (int64(Cfg.Paths[Cfg.Media[cfg].DataImport[idx].Template_path].MinSize)*1024*1024) && size != 0 {
				//logger.Log.GlobalLogger.Debug("Skipped - MinSize not matched", zap.String("title", title))
				return true
			}
		}

		if Cfg.Paths[Cfg.Media[cfg].DataImport[idx].Template_path].MaxSize != 0 {
			if size > (int64(Cfg.Paths[Cfg.Media[cfg].DataImport[idx].Template_path].MaxSize) * 1024 * 1024) {
				//logger.Log.GlobalLogger.Debug("Skipped - MaxSize not matched", zap.String("title", title))
				return true
			}
		}
	}
	return false
}
