// koanf_api
package config

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/recoilme/pudge"
	"golang.org/x/oauth2"
)

//Series Config

type MainSerieConfig struct {
	Global GlobalSerieConfig `koanf:"global"`
	Serie  []SerieConfig     `koanf:"series"`
}
type GlobalSerieConfig struct {
	Identifiedby   string `koanf:"identifiedby"`
	Upgrade        bool   `koanf:"upgrade"`
	Search         bool   `koanf:"search"`
	SearchProvider string `koanf:"search_provider"`
}
type SerieConfig struct {
	Name           string   `koanf:"name"`
	TvdbID         int      `koanf:"tvdb_id"`
	AlternateName  []string `koanf:"alternatename"`
	Identifiedby   string   `koanf:"identifiedby"`
	DontUpgrade    bool     `koanf:"dont_upgrade"`
	DontSearch     bool     `koanf:"dont_search"`
	SearchSpecials bool     `koanf:"search_specials"`
	IgnoreRuntime  bool     `koanf:"ignore_runtime"`
	Source         string   `koanf:"source"`
	Target         string   `koanf:"target"`
}

//Main Config
type MainConfig struct {
	General      GeneralConfig        `koanf:"general"`
	Imdbindexer  ImdbConfig           `koanf:"imdbindexer"`
	Media        MediaConfig          `koanf:"media"`
	Downloader   []DownloaderConfig   `koanf:"downloader"`
	Lists        []ListsConfig        `koanf:"lists"`
	Indexers     []IndexersConfig     `koanf:"indexers"`
	Paths        []PathsConfig        `koanf:"paths"`
	Notification []NotificationConfig `koanf:"notification"`
	Regex        []RegexConfig        `koanf:"regex"`
	Quality      []QualityConfig      `koanf:"quality"`
	Scheduler    []SchedulerConfig    `koanf:"scheduler"`
}
type MainConfigOut struct {
	General      GeneralConfig        `koanf:"general"`
	Imdbindexer  ImdbConfig           `koanf:"imdbindexer"`
	Media        MediaConfig          `koanf:"media"`
	Downloader   []DownloaderConfig   `koanf:"downloader"`
	Lists        []ListsConfig        `koanf:"lists"`
	Indexers     []IndexersConfig     `koanf:"indexers"`
	Paths        []PathsConfig        `koanf:"paths"`
	Notification []NotificationConfig `koanf:"notification"`
	Regex        []RegexConfigIn      `koanf:"regex"`
	Quality      []QualityConfig      `koanf:"quality"`
	Scheduler    []SchedulerConfig    `koanf:"scheduler"`
}
type GeneralConfig struct {
	LogLevel                           string   `koanf:"log_level"`
	DBLogLevel                         string   `koanf:"db_log_level"`
	LogFileSize                        int      `koanf:"log_file_size"`
	LogFileCount                       int      `koanf:"log_file_count"`
	LogCompress                        bool     `koanf:"log_compress"`
	WorkerDefault                      int      `koanf:"worker_default"`
	WorkerMetadata                     int      `koanf:"worker_metadata"`
	WorkerFiles                        int      `koanf:"worker_files"`
	WorkerParse                        int      `koanf:"worker_parse"`
	WorkerSearch                       int      `koanf:"worker_search"`
	WorkerIndexer                      int      `koanf:"worker_indexer"`
	OmdbApiKey                         string   `koanf:"omdb_apikey"`
	MovieMetaSourceImdb                bool     `koanf:"movie_meta_source_imdb"`
	MovieMetaSourceTmdb                bool     `koanf:"movie_meta_source_tmdb"`
	MovieMetaSourceOmdb                bool     `koanf:"movie_meta_source_omdb"`
	MovieMetaSourceTrakt               bool     `koanf:"movie_meta_source_trakt"`
	MovieAlternateTitleMetaSourceImdb  bool     `koanf:"movie_alternate_title_meta_source_imdb"`
	MovieAlternateTitleMetaSourceTmdb  bool     `koanf:"movie_alternate_title_meta_source_tmdb"`
	MovieAlternateTitleMetaSourceOmdb  bool     `koanf:"movie_alternate_title_meta_source_omdb"`
	MovieAlternateTitleMetaSourceTrakt bool     `koanf:"movie_alternate_title_meta_source_trakt"`
	SerieAlternateTitleMetaSourceImdb  bool     `koanf:"serie_alternate_title_meta_source_imdb"`
	SerieAlternateTitleMetaSourceTrakt bool     `koanf:"serie_alternate_title_meta_source_trakt"`
	MovieMetaSourcePriority            []string `koanf:"movie_meta_source_priority"`
	MovieRSSMetaSourcePriority         []string `koanf:"movie_rss_meta_source_priority"`
	MovieParseMetaSourcePriority       []string `koanf:"movie_parse_meta_source_priority"`
	SerieMetaSourceTmdb                bool     `koanf:"serie_meta_source_tmdb"`
	SerieMetaSourceTrakt               bool     `koanf:"serie_meta_source_trakt"`
	UseGoDir                           bool     `koanf:"use_godir"`
	MoveBufferSizeKB                   int      `koanf:"move_buffer_size_kb"`
	WebPort                            string   `koanf:"webport"`
	WebApiKey                          string   `koanf:"webapikey"`
	ConcurrentScheduler                int      `koanf:"concurrent_scheduler"`
	TheMovieDBApiKey                   string   `koanf:"themoviedb_apikey"`
	TraktClientId                      string   `koanf:"trakt_client_id"`
	TraktClientSecret                  string   `koanf:"trakt_client_secret"`
	SchedulerDisabled                  bool     `koanf:"scheduler_disabled"`
	DisableParserStringMatch           bool     `koanf:"disable_parser_string_match"`
	UseCronInsteadOfInterval           bool     `koanf:"use_cron_instead_of_interval"`
	EnableFileWatcher                  bool     `koanf:"enable_file_watcher"`
	UseFileBufferCopy                  bool     `koanf:"use_file_buffer_copy"`
	DisableSwagger                     bool     `koanf:"disable_swagger"`
	Traktlimiterseconds                int      `koanf:"trakt_limiter_seconds"`
	Traktlimitercalls                  int      `koanf:"trakt_limiter_calls"`
	Tvdblimiterseconds                 int      `koanf:"tvdb_limiter_seconds"`
	Tvdblimitercalls                   int      `koanf:"tvdb_limiter_calls"`
	Tmdblimiterseconds                 int      `koanf:"tmdb_limiter_seconds"`
	Tmdblimitercalls                   int      `koanf:"tmdb_limiter_calls"`
	Omdblimiterseconds                 int      `koanf:"omdb_limiter_seconds"`
	Omdblimitercalls                   int      `koanf:"omdb_limiter_calls"`
	TheMovieDBDisableTLSVerify         bool     `koanf:"tmdb_disable_tls_verify"`
	TraktDisableTLSVerify              bool     `koanf:"trakt_disable_tls_verify"`
	OmdbDisableTLSVerify               bool     `koanf:"omdb_disable_tls_verify"`
	TvdbDisableTLSVerify               bool     `koanf:"tvdb_disable_tls_verify"`
	FfprobePath                        string   `koanf:"ffprobe_path"`
	FailedIndexerBlockTime             int      `koanf:"failed_indexer_block_time"`
	MaxDatabaseBackups                 int      `koanf:"max_database_backups"`
	DisableVariableCleanup             bool     `koanf:"disable_variable_cleanup"`
}

type ImdbConfig struct {
	Indexedtypes     []string `koanf:"indexed_types"`
	Indexedlanguages []string `koanf:"indexed_languages"`
	Indexfull        bool     `koanf:"index_full"`
}
type MediaConfig struct {
	Series []MediaTypeConfig `koanf:"series"`
	Movies []MediaTypeConfig `koanf:"movies"`
}

type MediaTypeConfig struct {
	Name                      string                    `koanf:"name"`
	DefaultQuality            string                    `koanf:"default_quality"`
	DefaultResolution         string                    `koanf:"default_resolution"`
	Naming                    string                    `koanf:"naming"`
	NamingIdentifier          string                    `koanf:"naming_identifier"`
	Template_quality          string                    `koanf:"template_quality"`
	Template_scheduler        string                    `koanf:"template_scheduler"`
	Metadata_language         string                    `koanf:"metadata_language"`
	Metadata_title_languages  []string                  `koanf:"metadata_title_languages"`
	Metadata_source           string                    `koanf:"metadata_source"`
	Structure                 bool                      `koanf:"structure"`
	Searchmissing_incremental int                       `koanf:"search_missing_incremental"`
	Searchupgrade_incremental int                       `koanf:"search_upgrade_incremental"`
	Data                      []MediaDataConfig         `koanf:"data"`
	DataImport                []MediaDataImportConfig   `koanf:"data_import"`
	Lists                     []MediaListsConfig        `koanf:"lists"`
	Notification              []MediaNotificationConfig `koanf:"notification"`
}

type MediaDataConfig struct {
	Template_path string `koanf:"template_path"`
	AddFound      bool   `koanf:"add_found"`
	AddFoundList  string `koanf:"add_found_list"`
}

type MediaDataImportConfig struct {
	Template_path string `koanf:"template_path"`
}

type MediaListsConfig struct {
	Name               string   `koanf:"name"`
	Template_list      string   `koanf:"template_list"`
	Template_quality   string   `koanf:"template_quality"`
	Template_scheduler string   `koanf:"template_scheduler"`
	Ignore_map_lists   []string `koanf:"ignore_template_lists"`
	Replace_map_lists  []string `koanf:"replace_template_lists"`
	Enabled            bool     `koanf:"enabled"`
	Addfound           bool     `koanf:"add_found"`
}

type MediaNotificationConfig struct {
	Map_notification string `koanf:"template_notification"`
	Event            string `koanf:"event"`
	Title            string `koanf:"title"`
	Message          string `koanf:"message"`
	ReplacedPrefix   string `koanf:"replaced_prefix"`
}

type DownloaderConfig struct {
	Name                  string `koanf:"name"`
	DlType                string `koanf:"type"`
	Hostname              string `koanf:"hostname"`
	Port                  int    `koanf:"port"`
	Username              string `koanf:"username"`
	Password              string `koanf:"password"`
	AddPaused             bool   `koanf:"add_paused"`
	DelugeDlTo            string `koanf:"deluge_dl_to"`
	DelugeMoveAfter       bool   `koanf:"deluge_move_after"`
	DelugeMoveTo          string `koanf:"deluge_move_to"`
	Priority              int    `koanf:"priority"`
	Enabled               bool   `koanf:"enabled"`
	Autoredownloadfailed  bool   `koanf:"auto_redownload_failed"`
	Removefaileddownloads bool   `koanf:"remove_failed_downloads"`
}

type ListsConfig struct {
	Name               string   `koanf:"name"`
	ListType           string   `koanf:"type"`
	Url                string   `koanf:"url"`
	Enabled            bool     `koanf:"enabled"`
	Series_config_file string   `koanf:"series_config_file"`
	TraktUsername      string   `koanf:"trakt_username"`
	TraktListName      string   `koanf:"trakt_listname"`
	TraktListType      string   `koanf:"trakt_listtype"`
	Limit              int      `koanf:"limit"`
	MinVotes           int      `koanf:"min_votes"`
	MinRating          float32  `koanf:"min_rating"`
	Excludegenre       []string `koanf:"exclude_genre"`
	Includegenre       []string `koanf:"include_genre"`
}
type IndexersConfig struct {
	Name                   string `koanf:"name"`
	IndexerType            string `koanf:"type"`
	Url                    string `koanf:"url"`
	Apikey                 string `koanf:"apikey"`
	Userid                 string `koanf:"userid"`
	Enabled                bool   `koanf:"enabled"`
	Rssenabled             bool   `koanf:"rss_enabled"`
	Addquotesfortitlequery bool   `koanf:"add_quotes_for_title_query"`
	MaxRssEntries          int    `koanf:"max_rss_entries"`
	RssEntriesloop         int    `koanf:"rss_entries_loop"`
	RssDownloadAll         bool   `koanf:"rss_downlood_all"`
	OutputAsJson           bool   `koanf:"output_as_json"`
	Customapi              string `koanf:"custom_api"`
	Customurl              string `koanf:"custom_url"`
	Customrssurl           string `koanf:"custom_rss_url"`
	Customrsscategory      string `koanf:"custom_rss_category"`
	Limitercalls           int    `koanf:"limiter_calls"`
	Limiterseconds         int    `koanf:"limiter_seconds"`
	MaxAge                 int    `koanf:"max_age"`
	DisableTLSVerify       bool   `koanf:"disable_tls_verify"`
}

type PathsConfig struct {
	Name                           string   `koanf:"name"`
	Path                           string   `koanf:"path"`
	AllowedVideoExtensions         []string `koanf:"allowed_video_extensions"`
	AllowedOtherExtensions         []string `koanf:"allowed_other_extensions"`
	AllowedVideoExtensionsNoRename []string `koanf:"allowed_video_extensions_no_rename"`
	AllowedOtherExtensionsNoRename []string `koanf:"allowed_other_extensions_no_rename"`
	Blocked                        []string `koanf:"blocked"`
	Upgrade                        bool     `koanf:"upgrade"`
	MinSize                        int      `koanf:"min_size"`
	MaxSize                        int      `koanf:"max_size"`
	MinVideoSize                   int      `koanf:"min_video_size"`
	CleanupsizeMB                  int      `koanf:"cleanup_size_mb"`
	Allowed_languages              []string `koanf:"allowed_languages"`
	Replacelower                   bool     `koanf:"replace_lower"`
	Usepresort                     bool     `koanf:"use_presort"`
	PresortFolderPath              string   `koanf:"presort_folder_path"`
	UpgradeScanInterval            int      `koanf:"upgrade_scan_interval"`
	MissingScanInterval            int      `koanf:"missing_scan_interval"`
	MissingScanReleaseDatePre      int      `koanf:"missing_scan_release_date_pre"`
	Disallowed                     []string `koanf:"disallowed"`
	DeleteWrongLanguage            bool     `koanf:"delete_wrong_language"`
	DeleteDisallowed               bool     `koanf:"delete_disallowed"`
	CheckRuntime                   bool     `koanf:"check_runtime"`
	MaxRuntimeDifference           int      `koanf:"max_runtime_difference"`
	DeleteWrongRuntime             bool     `koanf:"delete_wrong_runtime"`
	MoveReplaced                   bool     `koanf:"move_replaced"`
	MoveReplacedTargetPath         string   `koanf:"move_replaced_target_path"`
}

type NotificationConfig struct {
	Name             string `koanf:"name"`
	NotificationType string `koanf:"type"`
	Apikey           string `koanf:"apikey"`
	Recipient        string `koanf:"recipient"`
	Outputto         string `koanf:"output_to"`
}

type RegexConfigIn struct {
	Name     string   `koanf:"name"`
	Required []string `koanf:"required"`
	Rejected []string `koanf:"rejected"`
}

type RegexGroup struct {
	Name string
	Re   regexp.Regexp
}
type RegexConfig struct {
	RegexConfigIn
}

type QualityConfig struct {
	Name                          string                 `koanf:"name"`
	Wanted_resolution             []string               `koanf:"wanted_resolution"`
	Wanted_quality                []string               `koanf:"wanted_quality"`
	Wanted_audio                  []string               `koanf:"wanted_audio"`
	Wanted_codec                  []string               `koanf:"wanted_codec"`
	Cutoff_resolution             string                 `koanf:"cutoff_resolution"`
	Cutoff_quality                string                 `koanf:"cutoff_quality"`
	BackupSearchForTitle          bool                   `koanf:"backup_search_for_title"`
	BackupSearchForAlternateTitle bool                   `koanf:"backup_search_for_alternate_title"`
	ExcludeYearFromTitleSearch    bool                   `koanf:"exclude_year_from_title_search"`
	CheckUntilFirstFound          bool                   `koanf:"check_until_first_found"`
	CheckTitle                    bool                   `koanf:"check_title"`
	CheckYear                     bool                   `koanf:"check_year"`
	CheckYear1                    bool                   `koanf:"check_year1"`
	TitleStripSuffixForSearch     []string               `koanf:"title_strip_suffix_for_search"`
	TitleStripPrefixForSearch     []string               `koanf:"title_strip_prefix_for_search"`
	QualityReorder                []QualityReorderConfig `koanf:"reorder"`
	Indexer                       []QualityIndexerConfig `koanf:"indexers"`
	Cutoff_priority               int
}

type QualityReorderConfig struct {
	Name        string `koanf:"name"`
	ReorderType string `koanf:"type"`
	Newpriority int    `koanf:"new_priority"`
}
type QualityIndexerConfig struct {
	Template_indexer        string `koanf:"template_indexer"`
	Template_downloader     string `koanf:"template_downloader"`
	Template_regex          string `koanf:"template_regex"`
	Template_path_nzb       string `koanf:"template_path_nzb"`
	Category_dowloader      string `koanf:"category_dowloader"`
	Additional_query_params string `koanf:"additional_query_params"`
	CustomQueryString       string `koanf:"custom_query_string"`
	Skip_empty_size         bool   `koanf:"skip_empty_size"`
	History_check_title     bool   `koanf:"history_check_title"`
	Categories_indexer      string `koanf:"categories_indexer"`
}

type SchedulerConfig struct {
	Name                                string `koanf:"name"`
	Interval_imdb                       string `koanf:"interval_imdb"`
	Interval_feeds                      string `koanf:"interval_feeds"`
	Interval_feeds_refresh_series       string `koanf:"interval_feeds_refresh_series"`
	Interval_feeds_refresh_movies       string `koanf:"interval_feeds_refresh_movies"`
	Interval_feeds_refresh_series_full  string `koanf:"interval_feeds_refresh_series_full"`
	Interval_feeds_refresh_movies_full  string `koanf:"interval_feeds_refresh_movies_full"`
	Interval_indexer_missing            string `koanf:"interval_indexer_missing"`
	Interval_indexer_upgrade            string `koanf:"interval_indexer_upgrade"`
	Interval_indexer_missing_full       string `koanf:"interval_indexer_missing_full"`
	Interval_indexer_upgrade_full       string `koanf:"interval_indexer_upgrade_full"`
	Interval_indexer_missing_title      string `koanf:"interval_indexer_missing_title"`
	Interval_indexer_upgrade_title      string `koanf:"interval_indexer_upgrade_title"`
	Interval_indexer_missing_full_title string `koanf:"interval_indexer_missing_full_title"`
	Interval_indexer_upgrade_full_title string `koanf:"interval_indexer_upgrade_full_title"`
	Interval_indexer_rss                string `koanf:"interval_indexer_rss"`
	Interval_scan_data                  string `koanf:"interval_scan_data"`
	Interval_scan_data_missing          string `koanf:"interval_scan_data_missing"`
	Interval_scan_data_flags            string `koanf:"interval_scan_data_flags"`
	Interval_scan_dataimport            string `koanf:"interval_scan_data_import"`
	Interval_database_backup            string `koanf:"interval_database_backup"`
	Interval_database_check             string `koanf:"interval_database_check"`
	Interval_indexer_rss_seasons        string `koanf:"interval_indexer_rss_seasons"`
	Cron_indexer_rss_seasons            string `koanf:"cron_indexer_rss_seasons"`
	Cron_imdb                           string `koanf:"cron_imdb"`
	Cron_feeds                          string `koanf:"cron_feeds"`
	Cron_feeds_refresh_series           string `koanf:"cron_feeds_refresh_series"`
	Cron_feeds_refresh_movies           string `koanf:"cron_feeds_refresh_movies"`
	Cron_feeds_refresh_series_full      string `koanf:"cron_feeds_refresh_series_full"`
	Cron_feeds_refresh_movies_full      string `koanf:"cron_feeds_refresh_movies_full"`
	Cron_indexer_missing                string `koanf:"cron_indexer_missing"`
	Cron_indexer_upgrade                string `koanf:"cron_indexer_upgrade"`
	Cron_indexer_missing_full           string `koanf:"cron_indexer_missing_full"`
	Cron_indexer_upgrade_full           string `koanf:"cron_indexer_upgrade_full"`
	Cron_indexer_missing_title          string `koanf:"cron_indexer_missing_title"`
	Cron_indexer_upgrade_title          string `koanf:"cron_indexer_upgrade_title"`
	Cron_indexer_missing_full_title     string `koanf:"cron_indexer_missing_full_title"`
	Cron_indexer_upgrade_full_title     string `koanf:"cron_indexer_upgrade_full_title"`
	Cron_indexer_rss                    string `koanf:"cron_indexer_rss"`
	Cron_scan_data                      string `koanf:"cron_scan_data"`
	Cron_scan_data_missing              string `koanf:"cron_scan_data_missing"`
	Cron_scan_data_flags                string `koanf:"cron_scan_data_flags"`
	Cron_scan_dataimport                string `koanf:"cron_scan_data_import"`
	Cron_database_backup                string `koanf:"cron_database_backup"`
	Cron_database_check                 string `koanf:"cron_database_check"`
}

func LoadSerie(filepath string) MainSerieConfig {
	var k = koanf.New(".")
	defer logger.ClearVar(k)
	f := file.Provider(filepath)
	defer logger.ClearVar(f)
	// if strings.Contains(filepath, ".json") {
	// 	err := k.Load(f, json.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 	}
	// }
	if strings.Contains(filepath, ".toml") {
		err := k.Load(f, toml.Parser())
		if err != nil {
			fmt.Println("Error loading config. ", err)
		}
	}
	// if strings.Contains(filepath, ".yaml") {
	// 	err := k.Load(f, yaml.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 	}
	// }
	//k.Load(file.Provider("config.yaml"), yaml.Parser())
	var out MainSerieConfig
	k.Unmarshal("", &out)
	return out
}

func Slepping(random bool, seconds int) {
	if random {
		rand.Seed(time.Now().UnixNano())
		n := rand.Intn(seconds) // n will be between 0 and 10
		logger.Log.Debug("Sleeping ", n+1, " seconds...")
		time.Sleep(time.Duration(1+n) * time.Second)
	} else {
		logger.Log.Debug("Sleeping ", seconds, " seconds...")
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

const Configfile string = "./config/config.toml"

func LoadCfgDB(configfile string) error {
	var k = koanf.New(".")
	defer logger.ClearVar(k)

	f := file.Provider(configfile)
	defer logger.ClearVar(f)

	// if strings.Contains(configfile, "json") {
	// 	err := k.Load(f, json.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 		return Cfg{}, nil, err
	// 	}
	// }
	if strings.Contains(configfile, "toml") {
		err := k.Load(f, toml.Parser())
		if err != nil {
			fmt.Println("Error loading config. ", err)
			return err
		}
	}
	// if strings.Contains(configfile, "yaml") {
	// 	err := k.Load(f, yaml.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 		return Cfg{}, nil, err
	// 	}
	// }

	if k.Sprint() == "" {
		fmt.Println("Error loading config. Config Empty")
	}
	LoadCfgDataDB(*f, configfile)

	outgen := ConfigGet("general").Data.(GeneralConfig)
	if outgen.EnableFileWatcher {
		f.Watch(func(event interface{}, err error) {
			if err != nil {
				logger.Log.Printf("watch error: %v", err)
				return
			}

			logger.Log.Println("cfg reloaded")
			time.Sleep(time.Duration(2) * time.Second)
			LoadCfgDataDB(*f, Configfile)
		})
	}
	return nil
}

func LoadCfgDataDB(f file.File, parser string) {
	var k = koanf.New(".")
	defer logger.ClearVar(k)

	// if strings.Contains(parser, "json") {
	// 	err := k.Load(f, json.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 		return Cfg{}
	// 	}
	// }
	if strings.Contains(parser, "toml") {
		err := k.Load(&f, toml.Parser())
		if err != nil {
			fmt.Println("Error loading config. ", err)
		}
	}
	// if strings.Contains(parser, "yaml") {
	// 	err := k.Load(f, yaml.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 		return Cfg{}
	// 	}
	// }

	if k.Sprint() == "" {
		fmt.Println("Error loading config. Config Empty")
	}
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()
	//config.CacheConfig()
	//scanner.CleanUpFolder("./backup", 10)
	pudge.BackupAll("")
	ConfigEntries = make([]Conf, 0, 50)
	var outdl []DownloaderConfig
	errdl := k.Unmarshal("downloader", &outdl)
	if errdl == nil {

		for idx := range outdl {
			errdlset := configDB.Set("downloader_"+outdl[idx].Name, outdl[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "downloader_" + outdl[idx].Name, Data: outdl[idx]})

			if errdlset != nil {
				logger.Log.Errorln("Error downloader setting db:", errdlset)
			}
		}
	} else {
		fmt.Println("Error unmarschall config. ", errdl)
	}
	var outgen GeneralConfig
	errgen := k.Unmarshal("general", &outgen)
	if errgen == nil {
		errgenset := configDB.Set("general", outgen)

		ConfigEntries = append(ConfigEntries, Conf{Name: "general", Data: outgen})
		if errgenset != nil {
			logger.Log.Errorln("Error general setting db:", errgenset)
		}
	}
	var outim ImdbConfig
	errimdb := k.Unmarshal("imdbindexer", &outim)
	if errimdb == nil {
		errimdbset := configDB.Set("imdb", &outim)
		ConfigEntries = append(ConfigEntries, Conf{Name: "imdb", Data: outim})
		if errimdbset != nil {
			logger.Log.Errorln("Error imdb setting db:", errimdbset)
		}
	}
	var outind []IndexersConfig
	errind := k.Unmarshal("indexers", &outind)
	if errind == nil {

		for idx := range outind {
			errindset := configDB.Set("indexer_"+outind[idx].Name, &outind[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "indexer_" + outind[idx].Name, Data: outind[idx]})
			if errindset != nil {
				logger.Log.Errorln("Error indexer setting db:", errindset)
			}
		}
	}
	var outlst []ListsConfig
	errlst := k.Unmarshal("lists", &outlst)
	if errlst == nil {

		for idx := range outlst {
			errlstset := configDB.Set("list_"+outlst[idx].Name, &outlst[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "list_" + outlst[idx].Name, Data: outlst[idx]})
			if errlstset != nil {
				logger.Log.Errorln("Error list setting db:", errlstset)
			}
		}
	}
	var outntf []NotificationConfig
	errntf := k.Unmarshal("notification", &outntf)
	if errntf == nil {

		for idx := range outntf {
			errntfset := configDB.Set("notification_"+outntf[idx].Name, &outntf[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "notification_" + outntf[idx].Name, Data: outntf[idx]})
			if errntfset != nil {
				logger.Log.Errorln("Error notification setting db:", errntfset)
			}
		}
	}
	var outpth []PathsConfig
	errpth := k.Unmarshal("paths", &outpth)
	if errpth == nil {
		for idx := range outpth {
			errpthset := configDB.Set("path_"+outpth[idx].Name, &outpth[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "path_" + outpth[idx].Name, Data: outpth[idx]})
			if errpthset != nil {
				logger.Log.Errorln("Error path setting db:", errpthset)
			}
		}
	}
	var outql []QualityConfig
	errql := k.Unmarshal("quality", &outql)
	if errql == nil {
		for idx := range outql {
			errqlset := configDB.Set("quality_"+outql[idx].Name, &outql[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "quality_" + outql[idx].Name, Data: outql[idx]})
			if errqlset != nil {
				logger.Log.Errorln("Error quality setting db:", errqlset)
			}
		}
	}
	var outrgx []RegexConfigIn
	errrgx := k.Unmarshal("regex", &outrgx)
	if errrgx == nil {
		for idx := range outrgx {
			errrgxset := configDB.Set("regex_"+outrgx[idx].Name, outrgx[idx])

			var generalCache RegexConfig
			generalCache.Name = outrgx[idx].Name
			generalCache.Rejected = outrgx[idx].Rejected
			generalCache.Required = outrgx[idx].Required
			var reg *regexp.Regexp
			var errreg error
			for _, rowtitle := range outrgx[idx].Rejected {
				if !RegexCheck(rowtitle) {
					reg, errreg = regexp.Compile(rowtitle)
					if errreg == nil {
						RegexAdd(rowtitle, *reg)
					}
				}
			}
			for _, rowtitle := range outrgx[idx].Required {
				if !RegexCheck(rowtitle) {
					reg, errreg = regexp.Compile(rowtitle)
					if errreg == nil {
						RegexAdd(rowtitle, *reg)
					}
				}
			}
			reg = nil
			ConfigEntries = append(ConfigEntries, Conf{Name: "regex_" + outrgx[idx].Name, Data: generalCache})
			if errrgxset != nil {
				logger.Log.Errorln("Error regex setting db:", errrgxset)
			}
		}
	}
	var outsch []SchedulerConfig
	errsch := k.Unmarshal("scheduler", &outsch)
	if errsch == nil {
		for idx := range outsch {
			errschset := configDB.Set("scheduler_"+outsch[idx].Name, &outsch[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "scheduler_" + outsch[idx].Name, Data: outsch[idx]})
			if errschset != nil {
				logger.Log.Errorln("Error scheduler setting db:", errschset)
			}
		}
	}
	var outmov []MediaTypeConfig
	errmov := k.Unmarshal("media.movies", &outmov)
	if errmov == nil {
		for idx := range outmov {
			errmovset := configDB.Set("movie_"+outmov[idx].Name, &outmov[idx])
			ConfigEntries = append(ConfigEntries, Conf{Name: "movie_" + outmov[idx].Name, Data: outmov[idx]})
			if errmovset != nil {
				logger.Log.Errorln("Error movie setting db:", errmovset)
			}
		}
	}
	var out []MediaTypeConfig
	errser := k.Unmarshal("media.series", &out)
	if errser == nil {
		for idx := range out {

			errset := configDB.Set("serie_"+out[idx].Name, &out[idx])

			ConfigEntries = append(ConfigEntries, Conf{Name: "serie_" + out[idx].Name, Data: out[idx]})
			if errset != nil {
				logger.Log.Errorln("Error serie setting db:", errset)
			}
		}
	}
	//Get from DB and not config
	hastoken, _ := configDB.Has("trakt_token")
	if hastoken {
		var token oauth2.Token
		errtoken := configDB.Get("trakt_token", &token)
		if errtoken == nil {
			ConfigEntries = append(ConfigEntries, Conf{Name: "trakt_token", Data: token})
		}
	}

	saveConfigEntriesMap()
}

func saveConfigEntriesMap() {
	MapConfigEntries = nil
	MapConfigEntries = make(map[string]*Conf)
	for idx := range ConfigEntries {
		MapConfigEntries[ConfigEntries[idx].Name] = &ConfigEntries[idx]
	}
}
func UpdateCfg(configIn []Conf) {
	defer logger.ClearVar(&configIn)
	ConfigEntries = nil

	for idx := range configIn {
		ConfigEntries = append(ConfigEntries, configIn[idx])
	}
	saveConfigEntriesMap()
}

func UpdateCfgEntry(configIn Conf) {
	configfound := false

	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()
	for idx := range ConfigEntries {
		if ConfigEntries[idx].Name == configIn.Name {
			ConfigEntries[idx].Data = configIn.Data
			configfound = true
			break
		}
	}
	if !configfound {
		data := Conf{Name: configIn.Name, Data: configIn.Data}
		ConfigEntries = append(ConfigEntries, data)
	}
	configDB.Set(configIn.Name, configIn.Data)

	saveConfigEntriesMap()
}
func findAndDelete(s []Conf, item string) []Conf {
	new := s[:0]
	defer logger.ClearVar(&new)
	for idx := range s {
		if s[idx].Name != item {
			new = append(new, s[idx])
		}
	}
	return new
}
func DeleteCfgEntry(name string) {
	new := ConfigEntries[:0]
	for idx := range ConfigEntries {
		if ConfigEntries[idx].Name != name {
			new = append(new, ConfigEntries[idx])
		}
	}
	ConfigEntries = new
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	configDB.Delete(name)

	saveConfigEntriesMap()
}

func ClearCfg() {
	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	configDB.DeleteFile()
	ConfigEntries = []Conf{}
	ConfigEntries = append(ConfigEntries, Conf{Name: "general", Data: GeneralConfig{
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
	}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "scheduler_Default", Data: SchedulerConfig{
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
	}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "downloader_initial", Data: DownloaderConfig{Name: "initial", DlType: "drone"}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "imdb", Data: ImdbConfig{Indexedtypes: []string{"movie"}, Indexedlanguages: []string{"US", "UK", "\\N"}}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "indexer_initial", Data: IndexersConfig{Name: "initial", IndexerType: "newznab", Limitercalls: 1, Limiterseconds: 1, MaxRssEntries: 100, RssEntriesloop: 2}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "list_initial", Data: ListsConfig{Name: "initial", ListType: "traktmovieanticipated", Limit: 20}})
	var dataconfig []MediaDataConfig
	dataconfig = append(dataconfig, MediaDataConfig{Template_path: "initial"})
	var dataimportconfig []MediaDataImportConfig
	dataimportconfig = append(dataimportconfig, MediaDataImportConfig{Template_path: "initial"})
	var noticonfig []MediaNotificationConfig
	noticonfig = append(noticonfig, MediaNotificationConfig{Map_notification: "initial"})
	var listsconfig []MediaListsConfig
	listsconfig = append(listsconfig, MediaListsConfig{Template_list: "initial", Template_quality: "initial", Template_scheduler: "Default"})
	ConfigEntries = append(ConfigEntries, Conf{Name: "movie_initial", Data: MediaTypeConfig{Name: "initial", Template_quality: "initial", Template_scheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "serie_initial", Data: MediaTypeConfig{Name: "initial", Template_quality: "initial", Template_scheduler: "Default", Data: dataconfig, DataImport: dataimportconfig, Lists: listsconfig, Notification: noticonfig}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "notification_initial", Data: NotificationConfig{Name: "initial", NotificationType: "csv"}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "path_initial", Data: PathsConfig{Name: "initial", AllowedVideoExtensions: []string{".avi", ".mkv", ".mp4"}, AllowedOtherExtensions: []string{".idx", ".sub", ".srt"}}})
	var quindconfig []QualityIndexerConfig
	quindconfig = append(quindconfig, QualityIndexerConfig{Template_indexer: "initial", Template_downloader: "initial", Template_regex: "initial", Template_path_nzb: "initial"})
	var qureoconfig []QualityReorderConfig
	qureoconfig = append(qureoconfig, QualityReorderConfig{})
	ConfigEntries = append(ConfigEntries, Conf{Name: "quality_initial", Data: QualityConfig{Name: "initial", QualityReorder: qureoconfig, Indexer: quindconfig}})
	ConfigEntries = append(ConfigEntries, Conf{Name: "regex_initial", Data: RegexConfig{RegexConfigIn: RegexConfigIn{Name: "initial"}}})

	saveConfigEntriesMap()
}
func WriteCfg() {
	var k = koanf.New(".")
	defer logger.ClearVar(k)

	configDB, _ := pudge.Open("./databases/config.db", &pudge.Config{
		SyncInterval: 0})
	defer configDB.Close()

	var bla MainConfigOut

	for idx := range ConfigEntries {
		if strings.HasPrefix(ConfigEntries[idx].Name, "general") {
			bla.General = ConfigEntries[idx].Data.(GeneralConfig)
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "downloader_") {
			bla.Downloader = append(bla.Downloader, ConfigEntries[idx].Data.(DownloaderConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "imdb") {
			bla.Imdbindexer = ConfigEntries[idx].Data.(ImdbConfig)
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "indexer") {
			bla.Indexers = append(bla.Indexers, ConfigEntries[idx].Data.(IndexersConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "list") {
			bla.Lists = append(bla.Lists, ConfigEntries[idx].Data.(ListsConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "serie") {
			bla.Media.Series = append(bla.Media.Series, ConfigEntries[idx].Data.(MediaTypeConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "movie") {
			bla.Media.Movies = append(bla.Media.Movies, ConfigEntries[idx].Data.(MediaTypeConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "notification") {
			bla.Notification = append(bla.Notification, ConfigEntries[idx].Data.(NotificationConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "path") {
			bla.Paths = append(bla.Paths, ConfigEntries[idx].Data.(PathsConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "quality") {
			bla.Quality = append(bla.Quality, ConfigEntries[idx].Data.(QualityConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "regex") {
			var tmpout RegexConfigIn
			var tmp RegexConfig
			tmp = ConfigEntries[idx].Data.(RegexConfig)
			tmpout.Name = tmp.Name
			tmpout.Rejected = tmp.Rejected
			tmpout.Required = tmp.Required
			bla.Regex = append(bla.Regex, tmpout)
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
		if strings.HasPrefix(ConfigEntries[idx].Name, "scheduler") {
			bla.Scheduler = append(bla.Scheduler, ConfigEntries[idx].Data.(SchedulerConfig))
			configDB.Set(ConfigEntries[idx].Name, ConfigEntries[idx].Data)
		}
	}
	k.Load(structs.Provider(bla, "koanf"), nil)

	byteArray, _ := k.Marshal(toml.Parser())
	defer logger.ClearVar(&byteArray)
	ioutil.WriteFile("./config/config.toml", byteArray, 0777)
}

func MedialistConfigFilterByListName(list interface{}, listname string) []MediaListsConfig {
	var templists []MediaListsConfig
	defer logger.ClearVar(&templists)
	for _, val := range list.([]MediaListsConfig) {
		if val.Name == listname {
			templists = append(templists, val)
		}
	}
	return templists
}

func QualityIndexerByQualityAndTemplate(quality string, indexerTemplate string) *QualityIndexerConfig {
	if ConfigCheck("quality_" + quality) {
		for _, indexer := range ConfigGet("quality_" + quality).Data.(QualityConfig).Indexer {
			if indexer.Template_indexer == indexerTemplate {
				return &indexer
			}
		}
	}
	return nil
}

func MedialistConfigToInterfaceArray(list interface{}) []interface{} {
	lists := make([]interface{}, 0, len(list.([]MediaListsConfig)))
	defer logger.ClearVar(&lists)
	for _, val := range list.([]MediaListsConfig) {
		lists = append(lists, val.Name)
	}
	return lists
}

func StringArrayToInterfaceArray(list []string) []interface{} {
	defer logger.ClearVar(&list)
	if list == nil {
		return []interface{}{}
	}
	lists := make([]interface{}, 0, len(list))
	defer logger.ClearVar(&lists)
	for idx := range list {
		lists = append(lists, list[idx])
	}
	return lists
}

func (indexer *QualityIndexerConfig) Filter_size_nzbs(configTemplate string, title string, size int64) bool {
	configEntry := ConfigGet(configTemplate).Data.(MediaTypeConfig)
	for idx := range configEntry.DataImport {

		if indexer.Skip_empty_size && size == 0 {
			logger.Log.Debug("Skipped - Size missing: ", title)
			return true
		}
		if !ConfigCheck("path_" + configEntry.DataImport[idx].Template_path) {
			return false
		}
		cfg_path := ConfigGet("path_" + configEntry.DataImport[idx].Template_path).Data.(PathsConfig)

		if cfg_path.MinSize != 0 {
			if size < int64(cfg_path.MinSize*1024*1024) && size != 0 {
				logger.Log.Debug("Skipped - MinSize not matched: ", title)
				return true
			}
		}

		if cfg_path.MaxSize != 0 {
			if size > int64(cfg_path.MaxSize*1024*1024) {
				logger.Log.Debug("Skipped - MaxSize not matched: ", title)
				return true
			}
		}
	}
	return false
}
