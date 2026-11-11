package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
)

// parseDownloaderConfigs parses form data into DownloaderConfig slice.
func parseDownloaderConfigs(c *gin.Context) ([]config.DownloaderConfig, error) {
	configs, err := parseConfigFromForm(c, "downloader", createDownloaderConfig)
	if err != nil {
		return nil, err
	}

	if err := validateDownloaderConfig(configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// parseListsConfigs parses form data into ListsConfig slice.
func parseListsConfigs(c *gin.Context) ([]config.ListsConfig, error) {
	configs, err := parseConfigFromForm(c, "lists", createListsConfig)
	if err != nil {
		return nil, err
	}

	if err := validateListsConfig(configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// parseIndexersConfigs parses form data into IndexersConfig slice.
func parseIndexersConfigs(c *gin.Context) ([]config.IndexersConfig, error) {
	configs, err := parseConfigFromForm(c, "indexers", createIndexersConfig)
	if err != nil {
		return nil, err
	}

	if err := validateIndexersConfig(configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// parsePathsConfigs parses form data into PathsConfig slice.
func parsePathsConfigs(c *gin.Context) ([]config.PathsConfig, error) {
	configs, err := parseConfigFromForm(c, "paths", createPathsConfig)
	if err != nil {
		return nil, err
	}

	if err := validatePathsConfig(configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// parseNotificationConfigs parses form data into NotificationConfig slice.
func parseNotificationConfigs(c *gin.Context) ([]config.NotificationConfig, error) {
	configs, err := parseConfigFromForm(c, "notifications", createNotificationConfig)
	if err != nil {
		return nil, err
	}

	if err := validateNotificationConfig(configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// parseRegexConfigs parses form data into RegexConfig slice.
func parseRegexConfigs(c *gin.Context) ([]config.RegexConfig, error) {
	configs, err := parseConfigFromForm(c, "regex", createRegexConfig)
	if err != nil {
		return nil, err
	}

	return configs, validateRegexConfig(configs)
}

// parseQualityConfigs parses form data into QualityConfig slice.
func parseQualityConfigs(c *gin.Context) ([]config.QualityConfig, error) {
	configs, err := parseConfigFromForm(c, "quality_main", createQualityConfig)
	if err != nil {
		return nil, err
	}

	return configs, validateQualityConfig(configs)
}

// parseSchedulerConfigs parses form data into SchedulerConfig slice.
func parseSchedulerConfigs(c *gin.Context) ([]config.SchedulerConfig, error) {
	configs, err := parseConfigFromForm(
		c,
		"scheduler",
		func(index string, c *gin.Context) (config.SchedulerConfig, error) {
			return createSchedulerConfig(index, c), nil
		},
	)
	if err != nil {
		return nil, err
	}

	return configs, validateSchedulerConfig(configs)
}

func parseMediaTypeConfig(
	c *gin.Context,
	mediaType, index string,
) (config.MediaTypeConfig, error) {
	var cfg config.MediaTypeConfig

	builder := NewConfigBuilder(c, fmt.Sprintf("media_main_%s_%s", mediaType, index), "")

	builder.SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.DefaultQuality, "DefaultQuality").
		SetString(&cfg.DefaultResolution, "DefaultResolution").
		SetString(&cfg.Naming, "Naming").
		SetString(&cfg.TemplateQuality, "TemplateQuality").
		SetString(&cfg.TemplateScheduler, "TemplateScheduler").
		SetString(&cfg.MetadataLanguage, "MetadataLanguage").
		SetStringArrayFromForm(&cfg.MetadataTitleLanguages, "MetadataTitleLanguages").
		SetBool(&cfg.Structure, "Structure").
		SetUint16(&cfg.SearchmissingIncremental, "SearchmissingIncremental").
		SetUint16(&cfg.SearchupgradeIncremental, "SearchupgradeIncremental").
		SetString(&cfg.AudibleRegion, "AudibleRegion")

	return cfg, builder.Err()
}

func parseMediaDataConfigs(
	c *gin.Context,
	mediaType, index string,
) ([]config.MediaDataConfig, error) {
	prefix := fmt.Sprintf("media_%s_%s_data", mediaType, index)

	var (
		configs []config.MediaDataConfig
		errs    []error
	)

	for _, subIndex := range formRowIndexes(c.Request.PostForm, prefix+"_", "_TemplatePath") {
		builder := NewConfigBuilder(c, prefix+"_"+subIndex, "")

		var cfg config.MediaDataConfig

		builder.SetStringRequired(&cfg.TemplatePath, "TemplatePath")

		if cfg.TemplatePath == "" {
			continue
		}

		builder.SetBool(&cfg.AddFound, "AddFound").
			SetString(&cfg.AddFoundList, "AddFoundList").
			SetBool(&cfg.EnableUnpacking, "EnableUnpacking").
			SetBool(&cfg.WriteRenameLog, "WriteRenameLog").
			SetBool(&cfg.EmbedArt, "EmbedArt").
			SetBool(&cfg.EmbedLyrics, "EmbedLyrics").
			SetBool(&cfg.SkipSeriesTrackMatch, "SkipSeriesTrackMatch").
			SetBool(&cfg.AllowAlternativeReleases, "AllowAlternativeReleases").
			SetStringArrayFromForm(&cfg.AllowedReleaseTypes, "AllowedReleaseTypes").
			SetStringArrayFromForm(&cfg.MBMediaFormats, "MBMediaFormats").
			SetBool(&cfg.AllowAllFormatsWhenStructuring, "AllowAllFormatsWhenStructuring").
			SetInt(&cfg.PerTrackToleranceSeconds, "PerTrackToleranceSeconds").
			SetInt(&cfg.PerTrackToleranceSecondsMax, "PerTrackToleranceSecondsMax").
			SetInt(&cfg.MaxTotalDifferenceSeconds, "MaxTotalDifferenceSeconds").
			SetBool(&cfg.AllowMissingTracks, "AllowMissingTracks").
			SetBool(&cfg.AllowExtraTracks, "AllowExtraTracks").
			SetBool(&cfg.AllowCountMismatchIfTotalMatch, "AllowCountMismatchIfTotalMatch").
			SetBool(&cfg.ReplaceEmbeddedChapters, "ReplaceEmbeddedChapters").
			SetBool(&cfg.ExceedToleranceIfTotalMatch, "ExceedToleranceIfTotalMatch").
			SetFloat64(&cfg.StrongRecThresh, "StrongRecThresh").
			SetFloat64(&cfg.MediumRecThresh, "MediumRecThresh").
			SetFloat64(&cfg.TrackTitleWeight, "TrackTitleWeight").
			SetFloat64(&cfg.TrackIndexWeight, "TrackIndexWeight").
			SetFloat64(&cfg.TrackLengthWeight, "TrackLengthWeight").
			SetFloat64(&cfg.TrackArtistWeight, "TrackArtistWeight").
			SetFloat64(&cfg.TrackIdWeight, "TrackIdWeight").
			SetBool(&cfg.DiscoverSeriesAlbums, "DiscoverSeriesAlbums")

		if err := builder.Err(); err != nil {
			errs = append(errs, err)
		}

		configs = append(configs, cfg)
	}

	return configs, errors.Join(errs...)
}

func parseMediaDataImportConfigs(
	c *gin.Context,
	mediaType, index string,
) ([]config.MediaDataImportConfig, error) {
	prefix := fmt.Sprintf("media_%s_%s_dataimport", mediaType, index)

	var (
		configs []config.MediaDataImportConfig
		errs    []error
	)

	for _, subIndex := range formRowIndexes(c.Request.PostForm, prefix+"_", "_TemplatePath") {
		builder := NewConfigBuilder(c, prefix+"_"+subIndex, "")

		var cfg config.MediaDataImportConfig

		builder.SetStringRequired(&cfg.TemplatePath, "TemplatePath")

		if cfg.TemplatePath == "" {
			continue
		}

		builder.SetBool(&cfg.EnableUnpacking, "EnableUnpacking").
			SetBool(&cfg.AddFound, "AddFound").
			SetString(&cfg.AddFoundList, "AddFoundList").
			SetBool(&cfg.AllowAlternativeReleases, "AllowAlternativeReleases").
			SetStringRequired(&cfg.MoveUnprocessed, "MoveUnprocessed").
			SetBool(&cfg.EmbedArt, "EmbedArt").
			SetBool(&cfg.AllowAllFormatsWhenStructuring, "AllowAllFormatsWhenStructuring").
			SetStringArrayFromForm(&cfg.AllowedReleaseTypes, "AllowedReleaseTypes").
			SetStringArrayFromForm(&cfg.MBMediaFormats, "MBMediaFormats").
			SetInt(&cfg.PerTrackToleranceSeconds, "PerTrackToleranceSeconds").
			SetInt(&cfg.PerTrackToleranceSecondsMax, "PerTrackToleranceSecondsMax").
			SetInt(&cfg.MaxTotalDifferenceSeconds, "MaxTotalDifferenceSeconds").
			SetBool(&cfg.AllowMissingTracks, "AllowMissingTracks").
			SetBool(&cfg.AllowExtraTracks, "AllowExtraTracks").
			SetBool(&cfg.AllowCountMismatchIfTotalMatch, "AllowCountMismatchIfTotalMatch").
			SetBool(&cfg.ReplaceEmbeddedChapters, "ReplaceEmbeddedChapters").
			SetBool(&cfg.ExceedToleranceIfTotalMatch, "ExceedToleranceIfTotalMatch").
			SetFloat64(&cfg.StrongRecThresh, "StrongRecThresh").
			SetFloat64(&cfg.MediumRecThresh, "MediumRecThresh").
			SetFloat64(&cfg.TrackTitleWeight, "TrackTitleWeight").
			SetFloat64(&cfg.TrackIndexWeight, "TrackIndexWeight").
			SetFloat64(&cfg.TrackLengthWeight, "TrackLengthWeight").
			SetFloat64(&cfg.TrackArtistWeight, "TrackArtistWeight").
			SetFloat64(&cfg.TrackIdWeight, "TrackIdWeight").
			SetBool(&cfg.DiscoverSeriesAlbums, "DiscoverSeriesAlbums")

		if err := builder.Err(); err != nil {
			errs = append(errs, err)
		}

		configs = append(configs, cfg)
	}

	return configs, errors.Join(errs...)
}

func parseMediaListsConfigs(
	c *gin.Context,
	mediaType, index string,
) []config.MediaListsConfig {
	prefix := fmt.Sprintf("media_%s_%s_lists", mediaType, index)

	var configs []config.MediaListsConfig

	for _, subIndex := range formRowIndexes(c.Request.PostForm, prefix+"_", "_Name") {
		builder := NewConfigBuilder(c, prefix+"_"+subIndex, "")

		var cfg config.MediaListsConfig

		builder.SetStringRequired(&cfg.Name, "Name")

		// Sub-rows without a name are dropped as before: the default config
		// ships an unnamed placeholder list row.
		if cfg.Name == "" {
			continue
		}

		builder.SetString(&cfg.TemplateList, "TemplateList").
			SetString(&cfg.TemplateQuality, "TemplateQuality").
			SetString(&cfg.TemplateScheduler, "TemplateScheduler").
			SetStringArrayFromForm(&cfg.IgnoreMapLists, "IgnoreMapLists").
			SetStringArrayFromForm(&cfg.ReplaceMapLists, "ReplaceMapLists").
			SetBool(&cfg.Enabled, "Enabled").
			SetBool(&cfg.Addfound, "Addfound")

		configs = append(configs, cfg)
	}

	return configs
}

func parseMediaNotificationConfigs(
	c *gin.Context,
	mediaType, index string,
) []config.MediaNotificationConfig {
	prefix := fmt.Sprintf("media_%s_%s_notification", mediaType, index)

	var configs []config.MediaNotificationConfig

	for _, subIndex := range formRowIndexes(c.Request.PostForm, prefix+"_", "_MapNotification") {
		builder := NewConfigBuilder(c, prefix+"_"+subIndex, "")

		var cfg config.MediaNotificationConfig

		builder.SetStringRequired(&cfg.MapNotification, "MapNotification")

		if cfg.MapNotification == "" {
			continue
		}

		builder.SetString(&cfg.Event, "Event").
			SetString(&cfg.Title, "Title").
			SetString(&cfg.Message, "Message").
			SetString(&cfg.ReplacedPrefix, "ReplacedPrefix")

		configs = append(configs, cfg)
	}

	return configs
}

func parseMediaConfigs(c *gin.Context, mediaType string) ([]config.MediaTypeConfig, error) {
	var (
		configs []config.MediaTypeConfig
		errs    []error
	)

	for _, index := range formRowIndexes(c.Request.PostForm, "media_main_"+mediaType+"_", "_Name") {
		cfg, err := parseMediaTypeConfig(c, mediaType, index)
		if err != nil {
			errs = append(errs, err)
		}

		if cfg.Name == "" {
			errs = append(errs, fmt.Errorf("%s entry %s: name must not be empty", mediaType, index))
			continue
		}

		if cfg.Data, err = parseMediaDataConfigs(c, mediaType, index); err != nil {
			errs = append(errs, err)
		}

		if cfg.DataImport, err = parseMediaDataImportConfigs(c, mediaType, index); err != nil {
			errs = append(errs, err)
		}

		cfg.Lists = parseMediaListsConfigs(c, mediaType, index)

		cfg.Notification = parseMediaNotificationConfigs(c, mediaType, index)

		configs = append(configs, cfg)
	}

	return configs, errors.Join(errs...)
}

func parseGeneralConfig(c *gin.Context) (config.GeneralConfig, error) {
	updatedConfig := config.GetToml().General

	builder := &ConfigBuilder{context: c, prefix: "general"}
	builder.SetString(&updatedConfig.TimeFormat, "TimeFormat").
		SetString(&updatedConfig.TimeZone, "TimeZone").
		SetString(&updatedConfig.UserAgent, "UserAgent").
		SetString(&updatedConfig.LogLevel, "LogLevel").
		SetString(&updatedConfig.DBLogLevel, "DBLogLevel").
		SetInt(&updatedConfig.LogFileSize, "LogFileSize").
		SetUint8(&updatedConfig.LogFileCount, "LogFileCount").
		SetInt(&updatedConfig.WorkerMetadata, "WorkerMetadata").
		SetInt(&updatedConfig.WorkerFiles, "WorkerFiles").
		SetString(&updatedConfig.WebPort, "WebPort").
		// SetBool(&updatedConfig.DisableVariableCleanup, "DisableVariableCleanup").
		SetBool(&updatedConfig.DisableParserStringMatch, "DisableParserStringMatch").
		SetBool(&updatedConfig.UseMediaCache, "UseMediaCache").
		SetInt(&updatedConfig.CacheDuration, "CacheDuration").
		// SetBool(&updatedConfig.DisableSwagger, "DisableSwagger").
		SetString(&updatedConfig.WebAPIKey, "WebAPIKey").
		SetBool(&updatedConfig.LogCompress, "LogCompress").
		SetBool(&updatedConfig.LogToFileOnly, "LogToFileOnly").
		SetBool(&updatedConfig.LogColorize, "LogColorize").
		// SetBool(&updatedConfig.LogZeroValues, "LogZeroValues").
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
		SetString(&updatedConfig.MetaflacPath, "MetaflacPath").
		SetString(&updatedConfig.UnrarPath, "UnrarPath").
		SetString(&updatedConfig.SevenZipPath, "SevenZipPath").
		SetString(&updatedConfig.WhisperURL, "WhisperURL").
		SetString(&updatedConfig.HeadlessChromeURL, "HeadlessChromeURL").
		SetString(&updatedConfig.UnzipPath, "UnzipPath").
		SetString(&updatedConfig.TarPath, "TarPath").
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
		SetUint8(&updatedConfig.PlexLimiterSeconds, "PlexLimiterSeconds").
		SetInt(&updatedConfig.PlexLimiterCalls, "PlexLimiterCalls").
		SetUint16(&updatedConfig.PlexTimeoutSeconds, "PlexTimeoutSeconds").
		SetBool(&updatedConfig.PlexDisableTLSVerify, "PlexDisableTLSVerify").
		SetUint8(&updatedConfig.JellyfinLimiterSeconds, "JellyfinLimiterSeconds").
		SetInt(&updatedConfig.JellyfinLimiterCalls, "JellyfinLimiterCalls").
		SetUint16(&updatedConfig.JellyfinTimeoutSeconds, "JellyfinTimeoutSeconds").
		SetBool(&updatedConfig.JellyfinDisableTLSVerify, "JellyfinDisableTLSVerify").
		SetBool(&updatedConfig.UseFileBufferCopy, "UseFileBufferCopy").
		SetBool(&updatedConfig.UseCronInsteadOfInterval, "UseCronInsteadOfInterval").
		SetBool(&updatedConfig.SchedulerDisabled, "SchedulerDisabled").
		SetInt(&updatedConfig.MoveBufferSizeKB, "MoveBufferSizeKB").
		SetBool(&updatedConfig.SerieMetaSourceTrakt, "SerieMetaSourceTrakt").
		SetBool(&updatedConfig.SerieMetaSourceTmdb, "SerieMetaSourceTmdb").
		SetStringArrayFromForm(&updatedConfig.MovieParseMetaSourcePriority, "MovieParseMetaSourcePriority").
		SetStringArrayFromForm(&updatedConfig.MovieRSSMetaSourcePriority, "MovieRSSMetaSourcePriority").
		SetStringArrayFromForm(&updatedConfig.MovieMetaSourcePriority, "MovieMetaSourcePriority").
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
		SetInt(&updatedConfig.WorkerRSS, "WorkerRSS").
		// Book/Audiobook/Music Provider Settings
		SetString(&updatedConfig.FpcalcPath, "FpcalcPath").
		SetString(&updatedConfig.GoodreadsAPIKey, "GoodreadsAPIKey").
		SetString(&updatedConfig.DiscogsToken, "DiscogsToken").
		SetString(&updatedConfig.AcoustIDAPIKey, "AcoustIDAPIKey").
		// Book/Audiobook/Music Provider Rate Limits
		SetUint8(&updatedConfig.OpenLibraryLimiterSeconds, "OpenLibraryLimiterSeconds").
		SetInt(&updatedConfig.OpenLibraryLimiterCalls, "OpenLibraryLimiterCalls").
		SetUint8(&updatedConfig.GoodreadsLimiterSeconds, "GoodreadsLimiterSeconds").
		SetInt(&updatedConfig.GoodreadsLimiterCalls, "GoodreadsLimiterCalls").
		SetUint8(&updatedConfig.AudibleLimiterSeconds, "AudibleLimiterSeconds").
		SetInt(&updatedConfig.AudibleLimiterCalls, "AudibleLimiterCalls").
		SetUint8(&updatedConfig.AudnexLimiterSeconds, "AudnexLimiterSeconds").
		SetInt(&updatedConfig.AudnexLimiterCalls, "AudnexLimiterCalls").
		SetUint8(&updatedConfig.MusicBrainzLimiterSeconds, "MusicBrainzLimiterSeconds").
		SetInt(&updatedConfig.MusicBrainzLimiterCalls, "MusicBrainzLimiterCalls").
		SetUint8(&updatedConfig.DiscogsLimiterSeconds, "DiscogsLimiterSeconds").
		SetInt(&updatedConfig.DiscogsLimiterCalls, "DiscogsLimiterCalls").
		SetUint8(&updatedConfig.AcoustIDLimiterSeconds, "AcoustIDLimiterSeconds").
		SetInt(&updatedConfig.AcoustIDLimiterCalls, "AcoustIDLimiterCalls").
		SetUint8(&updatedConfig.LastFMLimiterSeconds, "LastFMLimiterSeconds").
		SetInt(&updatedConfig.LastFMLimiterCalls, "LastFMLimiterCalls").
		SetUint8(&updatedConfig.DeezerLimiterSeconds, "DeezerLimiterSeconds").
		SetInt(&updatedConfig.DeezerLimiterCalls, "DeezerLimiterCalls").
		SetUint8(&updatedConfig.ITunesLimiterSeconds, "ITunesLimiterSeconds").
		SetInt(&updatedConfig.ITunesLimiterCalls, "ITunesLimiterCalls").
		SetUint8(&updatedConfig.TheAudioDBLimiterSeconds, "TheAudioDBLimiterSeconds").
		SetInt(&updatedConfig.TheAudioDBLimiterCalls, "TheAudioDBLimiterCalls").
		// Cache
		SetBool(&updatedConfig.UseIndexedCache, "UseIndexedCache").
		// TVmaze
		SetUint8(&updatedConfig.TvmazeLimiterSeconds, "TvmazeLimiterSeconds").
		SetInt(&updatedConfig.TvmazeLimiterCalls, "TvmazeLimiterCalls").
		SetBool(&updatedConfig.TvmazeDisableTLSVerify, "TvmazeDisableTLSVerify").
		SetUint16(&updatedConfig.TvmazeTimeoutSeconds, "TvmazeTimeoutSeconds").
		// Spotify
		SetString(&updatedConfig.SpotifyClientID, "SpotifyClientID").
		SetString(&updatedConfig.SpotifyClientSecret, "SpotifyClientSecret").
		SetString(&updatedConfig.SpotifyRegion, "SpotifyRegion").
		// Last.fm
		SetString(&updatedConfig.LastFMAPIKey, "LastFMAPIKey").
		// TheAudioDB
		SetString(&updatedConfig.TheAudioDBAPIKey, "TheAudioDBAPIKey").
		// Music metadata source priority
		SetStringArrayFromForm(&updatedConfig.MusicMetaSourcePriority, "MusicMetaSourcePriority").
		// Music metadata source penalties (provider -> penalty weight)
		SetStringFloatMap(&updatedConfig.MusicMetaSourcePenalties, "MusicMetaSourcePenalties")

	return updatedConfig, builder.Err()
}

// parseConfigFromForm is a generic helper to parse form data into config structs.
// Rows are identified by the numeric index in "<prefix>_<index>_Name" keys and
// returned in index order, which is the order they appear in the form.
func parseConfigFromForm[T any](
	c *gin.Context,
	prefix string,
	createConfig func(string, *gin.Context) (T, error),
) ([]T, error) {
	if err := c.Request.ParseForm(); err != nil {
		return nil, fmt.Errorf("failed to parse form data: %w", err)
	}

	indexes := formRowIndexes(c.Request.PostForm, prefix+"_", "_Name")
	configs := make([]T, 0, len(indexes))

	var errs []error

	for _, i := range indexes {
		if strings.TrimSpace(c.PostForm(fmt.Sprintf("%s_%s_Name", prefix, i))) == "" {
			errs = append(errs, fmt.Errorf("%s entry %s: name must not be empty", prefix, i))
			continue
		}

		cfg, err := createConfig(i, c)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		configs = append(configs, cfg)
	}

	return configs, errors.Join(errs...)
}
