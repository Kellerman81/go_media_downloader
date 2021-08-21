// koanf_api
package config

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
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
	SearchProvider string `koanf:"SearchProvider"`
}
type SerieConfig struct {
	Name          string   `koanf:"name"`
	TvdbID        int      `koanf:"tvdb_id"`
	AlternateName []string `koanf:"alternatename"`
	Identifiedby  string   `koanf:"identifiedby"`
	Upgrade       bool     `koanf:"upgrade"`
	Search        bool     `koanf:"search"`
	Source        string   `koanf:"source"`
	Target        string   `koanf:"target"`
}

//Main Config
type MainConfig struct {
	General      GeneralConfig        `koanf:"General"`
	Imdbindexer  ImdbConfig           `koanf:"imdbindexer"`
	Media        MediaConfig          `koanf:"Media"`
	Downloader   []DownloaderConfig   `koanf:"downloader"`
	Lists        []ListsConfig        `koanf:"lists"`
	Indexers     []IndexersConfig     `koanf:"indexers"`
	Paths        []PathsConfig        `koanf:"paths"`
	Notification []NotificationConfig `koanf:"notification"`
	Regex        []RegexConfig        `koanf:"regex"`
	Quality      []QualityConfig      `koanf:"quality"`
	Scheduler    []SchedulerConfig    `koanf:"scheduler"`
}
type ImdbConfig struct {
	Indexedtypes     []string `koanf:"indexedtypes"`
	Indexedlanguages []string `koanf:"indexedlanguages"`
	Indexfull        bool     `koanf:"indexfull"`
}
type MediaConfig struct {
	Series []MediaTypeConfig `koanf:"series"`
	Movies []MediaTypeConfig `koanf:"movies"`
}

type MediaTypeConfig struct {
	Name                      string   `koanf:"Name"`
	DefaultQuality            string   `koanf:"DefaultQuality"`
	DefaultResolution         string   `koanf:"DefaultResolution"`
	Naming                    string   `koanf:"Naming"`
	NamingIdentifier          string   `koanf:"NamingIdentifier"`
	Template_quality          string   `koanf:"template_quality"`
	Template_scheduler        string   `koanf:"template_scheduler"`
	Metadata_language         string   `koanf:"metadata_language"`
	Metadata_title_languages  []string `koanf:"metadata_title_languages"`
	Metadata_source           string   `koanf:"metadata_source"`
	Structure                 bool     `koanf:"structure"`
	Searchmissing_incremental int      `koanf:"searchmissing_incremental"`
	Searchupgrade_incremental int      `koanf:"searchupgrade_incremental"`

	Data         []MediaDataConfig         `koanf:"data"`
	DataImport   []MediaDataImportConfig   `koanf:"dataimport"`
	Lists        []MediaListsConfig        `koanf:"lists"`
	Notification []MediaNotificationConfig `koanf:"notification"`
}

type MediaDataConfig struct {
	Template_path string `koanf:"template_path"`
	Replacelower  bool   `koanf:"replacelower"`
}

type MediaDataImportConfig struct {
	Template_path     string   `koanf:"template_path"`
	CleanupsizeMB     int      `koanf:"cleanupsizeMB"`
	Allowed_languages []string `koanf:"allowed_languages"`
}

type MediaListsConfig struct {
	Name               string   `koanf:"name"`
	Template_list      string   `koanf:"template_list"`
	Template_quality   string   `koanf:"template_quality"`
	Template_scheduler string   `koanf:"template_scheduler"`
	Ignore_map_lists   []string `koanf:"ignore_template_lists"`
	Replace_map_lists  []string `koanf:"replace_template_lists"`
	// Indexer                       []MediaListsIndexerConfig `koanf:"indexers"`
	Enabled  bool `koanf:"enabled"`
	Addfound bool `koanf:"addfound"`
}

// type MediaListsIndexerConfig struct {
// 	Template_indexer        string `koanf:"template_indexer"`
// 	Template_downloader     string `koanf:"template_downloader"`
// 	Template_regex          string `koanf:"template_regex"`
// 	Template_path_nzb       string `koanf:"template_path_nzb"`
// 	Category_dowloader      string `koanf:"category_dowloader"`
// 	Additional_query_params string `koanf:"additional_query_params"`
// 	CustomQueryString       string `koanf:"custom_query_string"`
// 	Skip_empty_size         bool   `koanf:"skip_empty_size"`
// 	History_check_title     bool   `koanf:"history_check_title"`
// 	Categories_indexer      string `koanf:"categories_indexer"`
// }

type MediaNotificationConfig struct {
	Map_notification string `koanf:"template_notification"`
	Event            string `koanf:"event"`
	Title            string `koanf:"title"`
	Message          string `koanf:"message"`
	ReplacedPrefix   string `koanf:"ReplacedPrefix"`
}

type GeneralConfig struct {
	LogLevel                     string   `koanf:"LogLevel"`
	DBLogLevel                   string   `koanf:"DBLogLevel"`
	LogFileSize                  int      `koanf:"LogFileSize"`
	LogFileCount                 int      `koanf:"LogFileCount"`
	WorkerDefault                int      `koanf:"WorkerDefault"`
	WorkerMetadata               int      `koanf:"WorkerMetadata"`
	WorkerFiles                  int      `koanf:"WorkerFiles"`
	WorkerParse                  int      `koanf:"WorkerParse"`
	WorkerSearch                 int      `koanf:"WorkerSearch"`
	OmdbApiKey                   string   `koanf:"OmdbApiKey"`
	MovieMetaSourceImdb          bool     `koanf:"MovieMetaSourceImdb"`
	MovieMetaSourceTmdb          bool     `koanf:"MovieMetaSourceTmdb"`
	MovieMetaSourceOmdb          bool     `koanf:"MovieMetaSourceOmdb"`
	MovieMetaSourceTrakt         bool     `koanf:"MovieMetaSourceTrakt"`
	MovieMetaSourcePriority      []string `koanf:"MovieMetaSourcePriority"`
	MovieRSSMetaSourcePriority   []string `koanf:"MovieRSSMetaSourcePriority"`
	MovieParseMetaSourcePriority []string `koanf:"MovieParseMetaSourcePriority"`
	SerieMetaSourceTmdb          bool     `koanf:"SerieMetaSourceTmdb"`
	SerieMetaSourceTrakt         bool     `koanf:"SerieMetaSourceTrakt"`
	WebPort                      string   `koanf:"webport"`
	WebApiKey                    string   `koanf:"webapikey"`
	ConcurrentScheduler          int      `koanf:"ConcurrentScheduler"`
	TheMovieDBApiKey             string   `koanf:"TheMovieDBApiKey"`
	TraktClientId                string   `koanf:"TraktClientId"`
	SchedulerDisabled            bool     `koanf:"SchedulerDisabled"`
	Traktlimiterseconds          int      `koanf:"traktlimiterseconds"`
	Traktlimitercalls            int      `koanf:"traktlimitercalls"`
	Tvdblimiterseconds           int      `koanf:"tvdblimiterseconds"`
	Tvdblimitercalls             int      `koanf:"tvdblimitercalls"`
	Tmdblimiterseconds           int      `koanf:"tmdblimiterseconds"`
	Tmdblimitercalls             int      `koanf:"tmdblimitercalls"`
	Omdblimiterseconds           int      `koanf:"omdblimiterseconds"`
	Omdblimitercalls             int      `koanf:"omdblimitercalls"`
}

type DownloaderConfig struct {
	Name                  string `koanf:"name"`
	Type                  string `koanf:"type"`
	Hostname              string `koanf:"hostname"`
	Port                  int    `koanf:"port"`
	Username              string `koanf:"username"`
	Password              string `koanf:"password"`
	AddPaused             bool   `koanf:"AddPaused"`
	DelugeDlTo            string `koanf:"DelugeDlTo"`
	DelugeMoveAfter       bool   `koanf:"DelugeMoveAfter"`
	DelugeMoveTo          string `koanf:"DelugeMoveTo"`
	Priority              int    `koanf:"Priority"`
	Enabled               bool   `koanf:"enabled"`
	Autoredownloadfailed  bool   `koanf:"autoredownloadfailed"`
	Removefaileddownloads bool   `koanf:"removefaileddownloads"`
}

type ListsConfig struct {
	Name               string   `koanf:"name"`
	Type               string   `koanf:"type"`
	Url                string   `koanf:"url"`
	Enabled            bool     `koanf:"enabled"`
	Series_config_file string   `koanf:"series_config_file"`
	Limit              int      `koanf:"limit"`
	Excludegenre       []string `koanf:"excludegenre"`
	Includegenre       []string `koanf:"includegenre"`
}
type IndexersConfig struct {
	Name                   string `koanf:"name"`
	Type                   string `koanf:"type"`
	Url                    string `koanf:"url"`
	Apikey                 string `koanf:"apikey"`
	Userid                 string `koanf:"userid"`
	Enabled                bool   `koanf:"enabled"`
	Rssenabled             bool   `koanf:"rssenabled"`
	Addquotesfortitlequery bool   `koanf:"addquotesfortitlequery"`
	MaxRssEntries          int    `koanf:"MaxRssEntries"`
	RssEntriesloop         int    `koanf:"RssEntriesloop"`
	Customapi              string `koanf:"customapi"`
	Customurl              string `koanf:"customurl"`
	Customrssurl           string `koanf:"customrssurl"`
	Limitercalls           int    `koanf:"limitercalls"`
	Limiterseconds         int    `koanf:"limiterseconds"`
}

type PathsConfig struct {
	Name                           string   `koanf:"name"`
	Path                           string   `koanf:"path"`
	AllowedVideoExtensions         []string `koanf:"AllowedVideoExtensions"`
	AllowedOtherExtensions         []string `koanf:"AllowedOtherExtensions"`
	AllowedVideoExtensionsNoRename []string `koanf:"AllowedVideoExtensionsNoRename"`
	AllowedOtherExtensionsNoRename []string `koanf:"AllowedOtherExtensionsNoRename"`
	Blocked                        []string `koanf:"Blocked"`
	Upgrade                        bool     `koanf:"Upgrade"`
	MinSize                        int      `koanf:"MinSize"`
	MaxSize                        int      `koanf:"MaxSize"`
	MinVideoSize                   int      `koanf:"MinVideoSize"`
	CleanupsizeMB                  int      `koanf:"cleanupsizeMB"`
	Allowed_languages              []string `koanf:"allowed_languages"`
	Replacelower                   bool     `koanf:"replacelower"`
	Usepresort                     bool     `koanf:"Usepresort"`
	UpgradeScanInterval            int      `koanf:"UpgradeScanInterval"`
	MissingScanInterval            int      `koanf:"MissingScanInterval"`
	Disallowed                     []string `koanf:"Disallowed"`
}

type NotificationConfig struct {
	Name      string `koanf:"name"`
	Type      string `koanf:"type"`
	Apikey    string `koanf:"apikey"`
	Recipient string `koanf:"recipient"`
	Outputto  string `koanf:"outputto"`
}

type RegexConfig struct {
	Name          string   `koanf:"name"`
	Required      []string `koanf:"Required"`
	Rejected      []string `koanf:"Rejected"`
	RequiredRegex map[string]*regexp.Regexp
	RejectedRegex map[string]*regexp.Regexp
}

type QualityConfig struct {
	Name                          string                 `koanf:"name"`
	Wanted_resolution             []string               `koanf:"wanted_resolution"`
	Wanted_quality                []string               `koanf:"wanted_quality"`
	Cutoff_resolution             string                 `koanf:"cutoff_resolution"`
	Cutoff_quality                string                 `koanf:"cutoff_quality"`
	BackupSearchForTitle          bool                   `koanf:"BackupSearchForTitle"`
	BackupSearchForAlternateTitle bool                   `koanf:"BackupSearchForAlternateTitle"`
	ExcludeYearFromTitleSearch    bool                   `koanf:"excludeYearFromTitleSearch"`
	CheckUntilFirstFound          bool                   `koanf:"CheckUntilFirstFound"`
	CheckTitle                    bool                   `koanf:"checktitle"`
	CheckYear                     bool                   `koanf:"checkyear"`
	CheckYear1                    bool                   `koanf:"checkyear1"`
	TitleStripSuffixForSearch     []string               `koanf:"TitleStripSuffixForSearch"`
	QualityReorder                []QualityReorderConfig `koanf:"reorder"`
	Indexer                       []QualityIndexerConfig `koanf:"indexers"`
	Cutoff_priority               int
}

type QualityReorderConfig struct {
	Name        string `koanf:"name"`
	Type        string `koanf:"type"`
	Newpriority int    `koanf:"newpriority"`
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
	Interval_scan_dataimport            string `koanf:"interval_scan_dataimport"`
}

//sorter
// type ByTaskPrio []Task

// func (a ByTaskPrio) Len() int           { return len(a) }
// func (a ByTaskPrio) Less(i, j int) bool { return a[i].Priority < a[j].Priority }
// func (a ByTaskPrio) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// type ByQualityTypePrio []QualityType

// func (a ByQualityTypePrio) Len() int           { return len(a) }
// func (a ByQualityTypePrio) Less(i, j int) bool { return a[i].Priority < a[j].Priority }
// func (a ByQualityTypePrio) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// type ByQualityProfileSettingPrio []QualityProfileSetting

// func (a ByQualityProfileSettingPrio) Len() int           { return len(a) }
// func (a ByQualityProfileSettingPrio) Less(i, j int) bool { return a[i].Priority < a[j].Priority }
// func (a ByQualityProfileSettingPrio) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

//sorter end

func LoadSerie(filepath string) MainSerieConfig {
	var k = koanf.New(".")
	f := file.Provider(filepath)
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
		logger.Log.Debug("Sleeping ", n, " seconds...")
		time.Sleep(time.Duration(n) * time.Second)
	} else {
		logger.Log.Debug("Sleeping ", seconds, " seconds...")
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

type Cfg struct {
	Movie        map[string]MediaTypeConfig
	Serie        map[string]MediaTypeConfig
	Downloader   map[string]DownloaderConfig
	General      GeneralConfig
	Imdb         ImdbConfig
	Indexer      map[string]IndexersConfig
	List         map[string]ListsConfig
	Notification map[string]NotificationConfig
	Path         map[string]PathsConfig
	Quality      map[string]QualityConfig
	Regex        map[string]RegexConfig
	Scheduler    map[string]SchedulerConfig
}

const Configfile string = "config.toml"

func LoadCfg(what []string, configfile string) (Cfg, *file.File, error) {
	var k = koanf.New(".")

	f := file.Provider(configfile)
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
			return Cfg{}, nil, err
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
		return Cfg{}, nil, errors.New("error loading config")
	}
	cfg := LoadCfgData(f, configfile, what)
	return cfg, f, nil
}

func Watch(f *file.File, parser string, what []string) {
	f.Watch(func(event interface{}, err error) {
		if err != nil {
			log.Printf("watch error: %v", err)
			return
		}

		log.Println("cfg reloaded")
		time.Sleep(time.Duration(2) * time.Second)
		LoadCfgData(f, parser, what)
	})
}

func LoadCfgData(f *file.File, parser string, what []string) Cfg {
	var k = koanf.New(".")

	// if strings.Contains(parser, "json") {
	// 	err := k.Load(f, json.Parser())
	// 	if err != nil {
	// 		fmt.Println("Error loading config. ", err)
	// 		return Cfg{}
	// 	}
	// }
	if strings.Contains(parser, "toml") {
		err := k.Load(f, toml.Parser())
		if err != nil {
			fmt.Println("Error loading config. ", err)
			return Cfg{}
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
		return Cfg{}
	}
	cfg := Cfg{}
	for _, load := range what {
		switch load {
		case "Downloader":
			var out []DownloaderConfig
			err := k.Unmarshal("downloader", &out)
			if err == nil {
				structout := make(map[string]DownloaderConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Downloader = structout
			}
		case "General":
			var out GeneralConfig
			err := k.Unmarshal("general", &out)
			if err == nil {
				cfg.General = out
			}
		case "Imdb":
			var out ImdbConfig
			err := k.Unmarshal("imdbindexer", &out)
			if err == nil {
				cfg.Imdb = out
			}
		case "Indexer":
			var out []IndexersConfig
			err := k.Unmarshal("indexers", &out)
			if err == nil {
				structout := make(map[string]IndexersConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Indexer = structout
			}
		case "List":
			fmt.Println("list")
			var out []ListsConfig
			err := k.Unmarshal("lists", &out)
			if err == nil {
				structout := make(map[string]ListsConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.List = structout
			}
		case "Notification":
			var out []NotificationConfig
			err := k.Unmarshal("notification", &out)
			if err == nil {
				structout := make(map[string]NotificationConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Notification = structout
			}
		case "Path":
			var out []PathsConfig
			err := k.Unmarshal("paths", &out)
			if err == nil {
				structout := make(map[string]PathsConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Path = structout
			}
		case "Quality":
			var out []QualityConfig
			err := k.Unmarshal("quality", &out)
			if err == nil {
				structout := make(map[string]QualityConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Quality = structout
			}
		case "Regex":
			var out []RegexConfig
			err := k.Unmarshal("regex", &out)
			if err == nil {
				structout := make(map[string]RegexConfig, len(out))
				for idx := range out {
					out[idx].RejectedRegex = make(map[string]*regexp.Regexp, len(out[idx].Rejected))
					for _, entry := range out[idx].Rejected {
						out[idx].RejectedRegex[entry] = regexp.MustCompile(entry)
					}
					out[idx].RequiredRegex = make(map[string]*regexp.Regexp, len(out[idx].Required))
					for _, entry := range out[idx].Required {
						out[idx].RequiredRegex[entry] = regexp.MustCompile(entry)
					}
					structout[out[idx].Name] = out[idx]
				}
				cfg.Regex = structout
			}
		case "Scheduler":
			var out []SchedulerConfig
			err := k.Unmarshal("scheduler", &out)
			if err == nil {
				structout := make(map[string]SchedulerConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Scheduler = structout
			}
		case "Movie":
			var out []MediaTypeConfig
			err := k.Unmarshal("media.movies", &out)
			if err == nil {
				structout := make(map[string]MediaTypeConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Movie = structout
			}
		case "Serie":
			var out []MediaTypeConfig
			err := k.Unmarshal("media.series", &out)
			if err == nil {
				structout := make(map[string]MediaTypeConfig, len(out))
				for idx := range out {
					structout[out[idx].Name] = out[idx]
				}
				cfg.Serie = structout
			}
		}
	}
	return cfg
}
