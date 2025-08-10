package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	gin "github.com/gin-gonic/gin"
)

// Generic config parsing pattern to reduce duplication
type ConfigParser[T any] struct {
	Prefix       string
	CreateConfig func(string, *gin.Context) T
	Validate     func([]T) error
	Save         func([]T) error
}

// ParseAndSave performs the complete config parsing, validation, and saving workflow
func (cp *ConfigParser[T]) ParseAndSave(c *gin.Context) error {
	if err := c.Request.ParseForm(); err != nil {
		return fmt.Errorf("failed to parse form data: %v", err)
	}

	// Extract form keys
	formKeys := extractFormKeys(c, cp.Prefix+"_", "_Name")
	configs := make([]T, 0, len(formKeys))

	// Create configs from form data
	for index := range formKeys {
		config := cp.CreateConfig(index, c)
		configs = append(configs, config)
	}

	// Validate all configs
	if cp.Validate != nil {
		if err := cp.Validate(configs); err != nil {
			return err
		}
	}

	// Save configs
	if cp.Save != nil {
		return cp.Save(configs)
	}

	return nil
}

// Pre-configured parsers for common config types
func createDownloaderParser() *ConfigParser[config.DownloaderConfig] {
	return &ConfigParser[config.DownloaderConfig]{
		Prefix: "downloader",
		CreateConfig: func(index string, c *gin.Context) config.DownloaderConfig {
			builder := NewOptimizedConfigBuilder(c, "downloader", index)
			return config.DownloaderConfig{
				Name:            builder.getString("Name"),
				DlType:          builder.getString("DLType"),
				Hostname:        builder.getString("Hostname"),
				Port:            builder.getInt("Port", 0),
				Username:        builder.getString("Username"),
				Password:        builder.getString("Password"),
				DelugeDlTo:      builder.getString("DelugeDlTo"),
				DelugeMoveTo:    builder.getString("DelugeMoveTo"),
				Priority:        builder.getInt("Priority", 0),
				AddPaused:       builder.getBool("AddPaused"),
				DelugeMoveAfter: builder.getBool("DelugeMoveAfter"),
				Enabled:         builder.getBool("Enabled"),
			}
		},
		Validate: func(configs []config.DownloaderConfig) error {
			return validateBatch(downloaderValidator, configs)
		},
		Save: func(configs []config.DownloaderConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create indexer parser using the optimized pattern
func createIndexerParser() *ConfigParser[config.IndexersConfig] {
	return &ConfigParser[config.IndexersConfig]{
		Prefix: "indexers",
		CreateConfig: func(index string, c *gin.Context) config.IndexersConfig {
			builder := NewOptimizedConfigBuilder(c, "indexers", index)
			return config.IndexersConfig{
				Name:                   builder.getString("Name"),
				IndexerType:            builder.getString("IndexerType"),
				URL:                    builder.getString("URL"),
				Apikey:                 builder.getString("Apikey"),
				Userid:                 builder.getString("Userid"),
				Enabled:                builder.getBool("Enabled"),
				Rssenabled:             builder.getBool("Rssenabled"),
				Addquotesfortitlequery: builder.getBool("Addquotesfortitlequery"),
				MaxEntries:             uint16(builder.getInt("MaxEntries", 0)),
				MaxEntriesStr:          builder.getString("MaxEntriesStr"),
				RssEntriesloop:         uint8(builder.getInt("RssEntriesloop", 0)),
				OutputAsJSON:           builder.getBool("OutputAsJSON"),
				Customapi:              builder.getString("Customapi"),
				Customurl:              builder.getString("Customurl"),
				Customrssurl:           builder.getString("Customrssurl"),
				Customrsscategory:      builder.getString("Customrsscategory"),
				Limitercalls:           builder.getInt("Limitercalls", 0),
				Limiterseconds:         uint8(builder.getInt("Limiterseconds", 0)),
				LimitercallsDaily:      builder.getInt("LimitercallsDaily", 0),
				MaxAge:                 uint16(builder.getInt("MaxAge", 0)),
				DisableTLSVerify:       builder.getBool("DisableTLSVerify"),
				DisableCompression:     builder.getBool("DisableCompression"),
				TimeoutSeconds:         uint16(builder.getInt("TimeoutSeconds", 0)),
				TrustWithIMDBIDs:       builder.getBool("TrustWithIMDBIDs"),
				TrustWithTVDBIDs:       builder.getBool("TrustWithTVDBIDs"),
				CheckTitleOnIDSearch:   builder.getBool("CheckTitleOnIDSearch"),
			}
		},
		Validate: func(configs []config.IndexersConfig) error {
			return validateBatch(indexerValidator, configs)
		},
		Save: func(configs []config.IndexersConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create paths parser using the optimized pattern
func createPathsParser() *ConfigParser[config.PathsConfig] {
	return &ConfigParser[config.PathsConfig]{
		Prefix: "paths",
		CreateConfig: func(index string, c *gin.Context) config.PathsConfig {
			builder := NewOptimizedConfigBuilder(c, "paths", index)
			return config.PathsConfig{
				Name:                           builder.getString("Name"),
				Path:                           builder.getString("Path"),
				AllowedVideoExtensions:         builder.getStringArray("AllowedVideoExtensions"),
				AllowedOtherExtensions:         builder.getStringArray("AllowedOtherExtensions"),
				AllowedVideoExtensionsNoRename: builder.getStringArray("AllowedVideoExtensionsNoRename"),
				AllowedOtherExtensionsNoRename: builder.getStringArray("AllowedOtherExtensionsNoRename"),
				Blocked:                        builder.getStringArray("Blocked"),
				Disallowed:                     builder.getStringArray("Disallowed"),
				AllowedLanguages:               builder.getStringArray("AllowedLanguages"),
				MaxSize:                        builder.getInt("MaxSize", 0),
				MinSize:                        builder.getInt("MinSize", 0),
				MinVideoSize:                   builder.getInt("MinVideoSize", 0),
				CleanupsizeMB:                  builder.getInt("CleanupsizeMB", 0),
				UpgradeScanInterval:            builder.getInt("UpgradeScanInterval", 0),
				MissingScanInterval:            builder.getInt("MissingScanInterval", 0),
				MissingScanReleaseDatePre:      builder.getInt("MissingScanReleaseDatePre", 0),
				MaxRuntimeDifference:           builder.getInt("MaxRuntimeDifference", 0),
				PresortFolderPath:              builder.getString("PresortFolderPath"),
				MoveReplacedTargetPath:         builder.getString("MoveReplacedTargetPath"),
				SetChmod:                       builder.getString("SetChmod"),
				SetChmodFolder:                 builder.getString("SetChmodFolder"),
				Upgrade:                        builder.getBool("Upgrade"),
				Replacelower:                   builder.getBool("Replacelower"),
				Usepresort:                     builder.getBool("Usepresort"),
				DeleteWrongLanguage:            builder.getBool("DeleteWrongLanguage"),
				DeleteDisallowed:               builder.getBool("DeleteDisallowed"),
				CheckRuntime:                   builder.getBool("CheckRuntime"),
				DeleteWrongRuntime:             builder.getBool("DeleteWrongRuntime"),
				MoveReplaced:                   builder.getBool("MoveReplaced"),
			}
		},
		Validate: func(configs []config.PathsConfig) error {
			return validatePathsConfig(configs)
		},
		Save: func(configs []config.PathsConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create lists parser using the optimized pattern
func createListsParser() *ConfigParser[config.ListsConfig] {
	return &ConfigParser[config.ListsConfig]{
		Prefix: "lists",
		CreateConfig: func(index string, c *gin.Context) config.ListsConfig {
			builder := NewOptimizedConfigBuilder(c, "lists", index)
			return config.ListsConfig{
				Name:             builder.getString("Name"),
				ListType:         builder.getString("ListType"),
				URL:              builder.getString("URL"),
				IMDBCSVFile:      builder.getString("IMDBCSVFile"),
				SeriesConfigFile: builder.getString("SeriesConfigFile"),
				TraktUsername:    builder.getString("TraktUsername"),
				TraktListName:    builder.getString("TraktListName"),
				TraktListType:    builder.getString("TraktListType"),
				Excludegenre:     builder.getStringArray("ExcludeGenre"),
				Includegenre:     builder.getStringArray("IncludeGenre"),
				TmdbDiscover:     builder.getStringArray("TmdbDiscover"),
				TmdbList:         builder.getIntArray("TmdbList"),
				Limit:            builder.getString("Limit"),
				MinVotes:         builder.getInt("MinVotes", 0),
				MinRating:        builder.getFloat32("MinRating", 0),
				RemoveFromList:   builder.getBool("RemoveFromList"),
				Enabled:          builder.getBool("Enabled"),
			}
		},
		Validate: func(configs []config.ListsConfig) error {
			return validateListsConfig(configs)
		},
		Save: func(configs []config.ListsConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create notification parser using the optimized pattern
func createNotificationParser() *ConfigParser[config.NotificationConfig] {
	return &ConfigParser[config.NotificationConfig]{
		Prefix: "notifications",
		CreateConfig: func(index string, c *gin.Context) config.NotificationConfig {
			builder := NewOptimizedConfigBuilder(c, "notifications", index)
			return config.NotificationConfig{
				Name:             builder.getString("Name"),
				NotificationType: builder.getString("NotificationType"),
				Apikey:           builder.getString("Apikey"),
				Recipient:        builder.getString("Recipient"),
				Outputto:         builder.getString("Outputto"),
			}
		},
		Validate: func(configs []config.NotificationConfig) error {
			return validateNotificationConfig(configs)
		},
		Save: func(configs []config.NotificationConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create regex parser using the optimized pattern
func createRegexParser() *ConfigParser[config.RegexConfig] {
	return &ConfigParser[config.RegexConfig]{
		Prefix: "regex",
		CreateConfig: func(index string, c *gin.Context) config.RegexConfig {
			builder := NewOptimizedConfigBuilder(c, "regex", index)
			return config.RegexConfig{
				Name:     builder.getString("Name"),
				Required: builder.getStringArray("Required"),
				Rejected: builder.getStringArray("Rejected"),
			}
		},
		Validate: func(configs []config.RegexConfig) error {
			return validateRegexConfig(configs)
		},
		Save: func(configs []config.RegexConfig) error {
			return saveConfig(configs)
		},
	}
}

// parseDownloaderConfigs parses form data into DownloaderConfig slice
func parseDownloaderConfigs(c *gin.Context) ([]config.DownloaderConfig, error) {
	configs := parseConfigFromForm(c, "downloader", createDownloaderConfig)
	if err := validateDownloaderConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// parseListsConfigs parses form data into ListsConfig slice
func parseListsConfigs(c *gin.Context) ([]config.ListsConfig, error) {
	configs := parseConfigFromForm(c, "lists", createListsConfig)
	if err := validateListsConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// parseIndexersConfigs parses form data into IndexersConfig slice
func parseIndexersConfigs(c *gin.Context) ([]config.IndexersConfig, error) {
	configs := parseConfigFromForm(c, "indexers", createIndexersConfig)
	if err := validateIndexersConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// parsePathsConfigs parses form data into PathsConfig slice
func parsePathsConfigs(c *gin.Context) ([]config.PathsConfig, error) {
	configs := parseConfigFromForm(c, "paths", createPathsConfig)
	if err := validatePathsConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// parseNotificationConfigs parses form data into NotificationConfig slice
func parseNotificationConfigs(c *gin.Context) ([]config.NotificationConfig, error) {
	configs := parseConfigFromForm(c, "notifications", createNotificationConfig)
	if err := validateNotificationConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// parseRegexConfigs parses form data into RegexConfig slice
func parseRegexConfigs(c *gin.Context) ([]config.RegexConfig, error) {
	formKeys := extractFormKeys(c, "regex_", "_Name")
	configs := make([]config.RegexConfig, 0, len(formKeys))

	for index := range formKeys {
		if config := createRegexConfig(index, c); config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, validateRegexConfig(configs)
}

// parseQualityConfigs parses form data into QualityConfig slice
func parseQualityConfigs(c *gin.Context) ([]config.QualityConfig, error) {
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, "quality_main_") {
			continue
		}
		formKeys[strings.Split(key, "_")[2]] = true
	}

	configs := make([]config.QualityConfig, 0, len(formKeys))
	for index := range formKeys {
		if config := createQualityConfig(index, c); config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, validateQualityConfig(configs)
}

// parseSchedulerConfigs parses form data into SchedulerConfig slice
func parseSchedulerConfigs(c *gin.Context) ([]config.SchedulerConfig, error) {
	formKeys := extractFormKeys(c, "scheduler_", "_Name")
	configs := make([]config.SchedulerConfig, 0, len(formKeys))

	for index := range formKeys {
		if config := createSchedulerConfig(index, c); config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, validateSchedulerConfig(configs)
}

func parseMediaTypeConfig[T any](c *gin.Context, mediaType, index string, builder *ConfigBuilder) config.MediaTypeConfig {
	var cfg config.MediaTypeConfig
	group := fmt.Sprintf("media_main_%s_%s", mediaType, index)

	builder.context = c
	builder.prefix = group
	builder.index = "" // Set to empty since prefix already includes the index

	builder.SetString(&cfg.Name, "Name").
		SetString(&cfg.DefaultQuality, "DefaultQuality").
		SetString(&cfg.DefaultResolution, "DefaultResolution").
		SetString(&cfg.Naming, "Naming").
		SetString(&cfg.TemplateQuality, "TemplateQuality").
		SetString(&cfg.TemplateScheduler, "TemplateScheduler").
		SetString(&cfg.MetadataLanguage, "MetadataLanguage").
		SetStringArray(&cfg.MetadataTitleLanguages, "MetadataTitleLanguages").
		SetBool(&cfg.Structure, "Structure").
		SetUint16(&cfg.SearchmissingIncremental, "SearchmissingIncremental").
		SetUint16(&cfg.SearchupgradeIncremental, "SearchupgradeIncremental")

	return cfg
}

func parseMediaDataConfigs(c *gin.Context, mediaType, index string) []config.MediaDataConfig {
	prefix := fmt.Sprintf("media_%s_%s_data", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_TemplatePath") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaDataConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		cfg := config.MediaDataConfig{TemplatePath: name}

		if val := c.PostForm(fmt.Sprintf("%s_%s_AddFound", prefix, subIndex)); val != "" {
			cfg.AddFound, _ = strconv.ParseBool(val)
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_AddFoundList", prefix, subIndex)); val != "" {
			cfg.AddFoundList = val
		}

		configs = append(configs, cfg)
	}
	return configs
}

func parseMediaDataImportConfigs(c *gin.Context, mediaType, index string) []config.MediaDataImportConfig {
	prefix := fmt.Sprintf("media_%s_%s_dataimport", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_TemplatePath") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaDataImportConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		configs = append(configs, config.MediaDataImportConfig{TemplatePath: name})
	}
	return configs
}

func parseMediaListsConfigs(c *gin.Context, mediaType, index string) []config.MediaListsConfig {
	prefix := fmt.Sprintf("media_%s_%s_lists", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaListsConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_Name", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		cfg := config.MediaListsConfig{Name: name}

		if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateList", prefix, subIndex)); val != "" {
			cfg.TemplateList = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateQuality", prefix, subIndex)); val != "" {
			cfg.TemplateQuality = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateScheduler", prefix, subIndex)); val != "" {
			cfg.TemplateScheduler = val
		}
		if val := c.PostFormArray(fmt.Sprintf("%s_%s_IgnoreMapLists", prefix, subIndex)); len(val) != 0 {
			cfg.IgnoreMapLists = val
		}
		if val := c.PostFormArray(fmt.Sprintf("%s_%s_ReplaceMapLists", prefix, subIndex)); len(val) != 0 {
			cfg.ReplaceMapLists = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Enabled", prefix, subIndex)); val != "" {
			cfg.Enabled, _ = strconv.ParseBool(val)
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Addfound", prefix, subIndex)); val != "" {
			cfg.Addfound, _ = strconv.ParseBool(val)
		}

		configs = append(configs, cfg)
	}
	return configs
}

func parseMediaNotificationConfigs(c *gin.Context, mediaType, index string) []config.MediaNotificationConfig {
	prefix := fmt.Sprintf("media_%s_%s_notification", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_MapNotification") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaNotificationConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_MapNotification", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		cfg := config.MediaNotificationConfig{MapNotification: name}

		if val := c.PostForm(fmt.Sprintf("%s_%s_Event", prefix, subIndex)); val != "" {
			cfg.Event = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Title", prefix, subIndex)); val != "" {
			cfg.Title = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Message", prefix, subIndex)); val != "" {
			cfg.Message = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_ReplacedPrefix", prefix, subIndex)); val != "" {
			cfg.ReplacedPrefix = val
		}

		configs = append(configs, cfg)
	}
	return configs
}

func parseMediaConfigs(c *gin.Context, mediaType string) []config.MediaTypeConfig {
	formKeys := make(map[string]bool)
	searchKey := fmt.Sprintf("media_main_%s_", mediaType)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, searchKey) {
			continue
		}
		// Extract the index part (e.g., "new0" from "media_main_series_new0_Name")
		parts := strings.Split(key, "_")
		if len(parts) >= 4 {
			formKeys[parts[3]] = true
		}
	}

	var configs []config.MediaTypeConfig
	var builder ConfigBuilder

	for index := range formKeys {
		cfg := parseMediaTypeConfig[config.MediaTypeConfig](c, mediaType, index, &builder)
		if cfg.Name == "" {
			continue
		}

		cfg.Data = parseMediaDataConfigs(c, mediaType, index)
		cfg.DataImport = parseMediaDataImportConfigs(c, mediaType, index)
		cfg.Lists = parseMediaListsConfigs(c, mediaType, index)
		cfg.Notification = parseMediaNotificationConfigs(c, mediaType, index)

		configs = append(configs, cfg)
	}
	return configs
}

func parseGeneralConfig(c *gin.Context) config.GeneralConfig {
	updatedConfig := config.GetToml().General

	builder := &ConfigBuilder{context: c, prefix: "general"}
	builder.SetString(&updatedConfig.TimeFormat, "TimeFormat").
		SetString(&updatedConfig.TimeZone, "TimeZone").
		SetString(&updatedConfig.LogLevel, "LogLevel").
		SetString(&updatedConfig.DBLogLevel, "DBLogLevel").
		SetInt(&updatedConfig.LogFileSize, "LogFileSize").
		SetUint8(&updatedConfig.LogFileCount, "LogFileCount").
		SetInt(&updatedConfig.WorkerMetadata, "WorkerMetadata").
		SetInt(&updatedConfig.WorkerFiles, "WorkerFiles").
		SetString(&updatedConfig.WebPort, "WebPort").
		SetBool(&updatedConfig.DisableVariableCleanup, "DisableVariableCleanup").
		SetBool(&updatedConfig.DisableParserStringMatch, "DisableParserStringMatch").
		SetBool(&updatedConfig.UseMediaCache, "UseMediaCache").
		SetInt(&updatedConfig.CacheDuration, "CacheDuration").
		SetBool(&updatedConfig.DisableSwagger, "DisableSwagger").
		SetString(&updatedConfig.WebAPIKey, "WebAPIKey").
		SetBool(&updatedConfig.LogCompress, "LogCompress").
		SetBool(&updatedConfig.LogToFileOnly, "LogToFileOnly").
		SetBool(&updatedConfig.LogColorize, "LogColorize").
		SetBool(&updatedConfig.LogZeroValues, "LogZeroValues").
		SetInt(&updatedConfig.WorkerParse, "WorkerParse").
		SetInt(&updatedConfig.WorkerSearch, "WorkerSearch").
		SetInt(&updatedConfig.WorkerIndexer, "WorkerIndexer").
		SetBool(&updatedConfig.WebPortalEnabled, "WebPortalEnabled").
		SetString(&updatedConfig.TheMovieDBApiKey, "TheMovieDBApiKey").
		SetString(&updatedConfig.TraktClientID, "TraktClientID").
		SetString(&updatedConfig.TraktClientSecret, "TraktClientSecret").
		SetString(&updatedConfig.TraktRedirectUrl, "TraktRedirectUrl").
		SetBool(&updatedConfig.EnableFileWatcher, "EnableFileWatcher").
		SetUint16(&updatedConfig.TraktTimeoutSeconds, "TraktTimeoutSeconds").
		SetUint16(&updatedConfig.TvdbTimeoutSeconds, "TvdbTimeoutSeconds").
		SetUint16(&updatedConfig.TmdbTimeoutSeconds, "TmdbTimeoutSeconds").
		SetUint16(&updatedConfig.OmdbTimeoutSeconds, "OmdbTimeoutSeconds").
		SetBool(&updatedConfig.DatabaseBackupStopTasks, "DatabaseBackupStopTasks").
		SetInt(&updatedConfig.MaxDatabaseBackups, "MaxDatabaseBackups").
		SetInt(&updatedConfig.FailedIndexerBlockTime, "FailedIndexerBlockTime").
		SetBool(&updatedConfig.UseMediaFallback, "UseMediaFallback").
		SetBool(&updatedConfig.UseMediainfo, "UseMediainfo").
		SetString(&updatedConfig.MediainfoPath, "MediainfoPath").
		SetString(&updatedConfig.FfprobePath, "FfprobePath").
		SetBool(&updatedConfig.TvdbDisableTLSVerify, "TvdbDisableTLSVerify").
		SetBool(&updatedConfig.OmdbDisableTLSVerify, "OmdbDisableTLSVerify").
		SetBool(&updatedConfig.TraktDisableTLSVerify, "TraktDisableTLSVerify").
		SetBool(&updatedConfig.TheMovieDBDisableTLSVerify, "TheMovieDBDisableTLSVerify").
		SetInt(&updatedConfig.OmdbLimiterCalls, "OmdbLimiterCalls").
		SetUint8(&updatedConfig.OmdbLimiterSeconds, "OmdbLimiterSeconds").
		SetInt(&updatedConfig.TmdbLimiterCalls, "TmdbLimiterCalls").
		SetUint8(&updatedConfig.TmdbLimiterSeconds, "TmdbLimiterSeconds").
		SetInt(&updatedConfig.TvdbLimiterCalls, "TvdbLimiterCalls").
		SetUint8(&updatedConfig.TvdbLimiterSeconds, "TvdbLimiterSeconds").
		SetInt(&updatedConfig.TraktLimiterCalls, "TraktLimiterCalls").
		SetUint8(&updatedConfig.TraktLimiterSeconds, "TraktLimiterSeconds").
		SetBool(&updatedConfig.UseFileBufferCopy, "UseFileBufferCopy").
		SetBool(&updatedConfig.UseCronInsteadOfInterval, "UseCronInsteadOfInterval").
		SetBool(&updatedConfig.SchedulerDisabled, "SchedulerDisabled").
		SetInt(&updatedConfig.MoveBufferSizeKB, "MoveBufferSizeKB").
		SetBool(&updatedConfig.SerieMetaSourceTrakt, "SerieMetaSourceTrakt").
		SetBool(&updatedConfig.SerieMetaSourceTmdb, "SerieMetaSourceTmdb").
		SetStringArray(&updatedConfig.MovieParseMetaSourcePriority, "MovieParseMetaSourcePriority").
		SetStringArray(&updatedConfig.MovieRSSMetaSourcePriority, "MovieRSSMetaSourcePriority").
		SetStringArray(&updatedConfig.MovieMetaSourcePriority, "MovieMetaSourcePriority").
		SetBool(&updatedConfig.SerieAlternateTitleMetaSourceTrakt, "SerieAlternateTitleMetaSourceTrakt").
		SetBool(&updatedConfig.SerieAlternateTitleMetaSourceImdb, "SerieAlternateTitleMetaSourceImdb").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceTrakt, "MovieAlternateTitleMetaSourceTrakt").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceOmdb, "MovieAlternateTitleMetaSourceOmdb").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceTmdb, "MovieAlternateTitleMetaSourceTmdb").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceImdb, "MovieAlternateTitleMetaSourceImdb").
		SetBool(&updatedConfig.MovieMetaSourceTrakt, "MovieMetaSourceTrakt").
		SetBool(&updatedConfig.MovieMetaSourceOmdb, "MovieMetaSourceOmdb").
		SetBool(&updatedConfig.MovieMetaSourceTmdb, "MovieMetaSourceTmdb").
		SetBool(&updatedConfig.MovieMetaSourceImdb, "MovieMetaSourceImdb").
		SetInt(&updatedConfig.SearcherSize, "SearcherSize").
		SetBool(&updatedConfig.CacheAutoExtend, "CacheAutoExtend").
		SetBool(&updatedConfig.UseHistoryCache, "UseHistoryCache").
		SetBool(&updatedConfig.UseFileCache, "UseFileCache").
		SetString(&updatedConfig.OmdbAPIKey, "OmdbAPIKey").
		SetInt(&updatedConfig.WorkerRSS, "WorkerRSS")

	return updatedConfig
}

// parseConfigFromForm is a generic helper to parse form data into config structs
func parseConfigFromForm[T any](c *gin.Context, prefix string, createConfig func(string, *gin.Context) T) []T {
	if err := c.Request.ParseForm(); err != nil {
		return nil
	}
	logger.LogDynamicany("info", "extractFormKeys debug",
		"prefix", prefix, "total form -  fields", len(c.Request.PostForm), "all", c.Request.PostForm, "Form", c.Request.Form)
	formKeys := extractFormKeys(c, prefix, "_Name")
	configs := make([]T, 0, len(formKeys))

	for i := range formKeys {
		nameField := fmt.Sprintf("%s_%s_Name", prefix, i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		config := createConfig(i, c)
		configs = append(configs, config)
	}

	return configs
}
