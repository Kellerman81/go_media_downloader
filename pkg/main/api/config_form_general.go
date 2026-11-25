package api

import (
	"fmt"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderGeneralConfig renders the general configuration section.
func renderGeneralConfig(configv *config.GeneralConfig, csrfToken string) gomponents.Node {
	group := "general"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-sliders",
			"General Configuration",
			"Configure general application settings including logging, timeouts, paths, and basic operational parameters.",
		),
		html.Form(
			html.Class("config-form"),

			// Configuration sections organized by category
			renderGeneralConfigSections(configv, group, comments, displayNames),

			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				"/api/admin/config/general/update",
				csrfToken,
			),
			html.Div(html.ID("addalert")),
		))
}

// convertMapToSelectOptions converts a map[string][]string to []shared.SelectOption.
func convertMapToSelectOptions(optionsMap map[string][]string) []SelectOption {
	var selectOptions []SelectOption
	if options, ok := optionsMap["options"]; ok {
		selectOptions = make([]SelectOption, len(options))
		for i, opt := range options {
			selectOptions[i] = SelectOption{Value: opt, Label: opt}
		}
	}

	return selectOptions
}

// convertSelectOptionsToMap turns field options into the option map of
// renderFormGroup. "options" holds the labels (single selects post the label;
// see feedTypeValue); "values" holds the values, used by the multi-choice chips.
func convertSelectOptionsToMap(optionsMap []SelectOption) map[string][]string {
	selectOptions := make([]string, 0, len(optionsMap))
	values := make([]string, 0, len(optionsMap))

	for _, row := range optionsMap {
		selectOptions = append(selectOptions, row.Label)
		values = append(values, fmt.Sprint(row.Value))
	}

	return map[string][]string{"options": selectOptions, "values": values}
}

// renderGeneralConfigSections organizes general config fields into logical collapsible groups.
func renderGeneralConfigSections(
	configv *config.GeneralConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	return html.Div(
		html.Class("accordion"),
		html.ID("generalConfigAccordion"),

		// Basic Settings Section
		renderConfigGroup("Basic Settings", "basic", true,
			[]FormFieldDefinition{
				{
					Name:  "TimeFormat",
					Type:  "select",
					Value: configv.TimeFormat,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"rfc3339", "iso8601", "rfc1123", "rfc822", "rfc850"},
					}),
				},
				{Name: "TimeZone", Type: "text", Value: configv.TimeZone},
				{Name: "UserAgent", Type: "text", Value: configv.UserAgent},
				{Name: "WebPort", Type: "text", Value: configv.WebPort},
				{Name: "WebAPIKey", Type: "password", Value: configv.WebAPIKey},
				{
					Name:    "",
					Type:    "custom",
					Value:   renderGenerateAPIKeyControl(group + "_WebAPIKey"),
					Options: nil,
				},
				{Name: "WebPortalEnabled", Type: "checkbox", Value: configv.WebPortalEnabled},
			}, group, comments, displayNames),

		// Logging Settings Section
		renderConfigGroup("Logging Settings", "logging", false,
			[]FormFieldDefinition{
				{
					Name:  "LogLevel",
					Type:  "select",
					Value: configv.LogLevel,
					Options: convertMapToSelectOptions(
						map[string][]string{"options": {"info", "debug", "warning"}},
					),
				},
				{
					Name:  "DBLogLevel",
					Type:  "select",
					Value: configv.DBLogLevel,
					Options: convertMapToSelectOptions(
						map[string][]string{"options": {"info", "debug"}},
					),
				},
				{Name: "LogFileSize", Type: "number", Value: configv.LogFileSize},
				{Name: "LogFileCount", Type: "number", Value: configv.LogFileCount},
				{Name: "LogCompress", Type: "checkbox", Value: configv.LogCompress},
				{Name: "LogToFileOnly", Type: "checkbox", Value: configv.LogToFileOnly},
				{Name: "LogColorize", Type: "checkbox", Value: configv.LogColorize},
				// {Name: "LogZeroValues", Type: "checkbox", Value: configv.LogZeroValues},
			}, group, comments, displayNames),

		// Worker Settings Section
		renderConfigGroup("Worker Settings", "workers", false,
			[]FormFieldDefinition{
				{Name: "WorkerMetadata", Type: "number", Value: configv.WorkerMetadata},
				{Name: "WorkerFiles", Type: "number", Value: configv.WorkerFiles},
				{Name: "WorkerParse", Type: "number", Value: configv.WorkerParse},
				{Name: "WorkerSearch", Type: "number", Value: configv.WorkerSearch},
				{Name: "WorkerRSS", Type: "number", Value: configv.WorkerRSS},
				{Name: "WorkerIndexer", Type: "number", Value: configv.WorkerIndexer},
			}, group, comments, displayNames),

		// Cache Settings Section
		renderConfigGroup("Cache Settings", "cache", false,
			[]FormFieldDefinition{
				{Name: "UseMediaCache", Type: "checkbox", Value: configv.UseMediaCache},
				{Name: "UseFileCache", Type: "checkbox", Value: configv.UseFileCache},
				{Name: "UseHistoryCache", Type: "checkbox", Value: configv.UseHistoryCache},
				{Name: "CacheDuration", Type: "number", Value: configv.CacheDuration},
				{Name: "CacheAutoExtend", Type: "checkbox", Value: configv.CacheAutoExtend},
				{Name: "UseIndexedCache", Type: "checkbox", Value: configv.UseIndexedCache},
				{Name: "SearcherSize", Type: "number", Value: configv.SearcherSize},
			}, group, comments, displayNames),

		// API Integration Section
		renderConfigGroup("API Integration", "apis", false,
			[]FormFieldDefinition{
				{Name: "OmdbAPIKey", Type: "password", Value: configv.OmdbAPIKey},
				{Name: "TheMovieDBApiKey", Type: "password", Value: configv.TheMovieDBApiKey},
				{Name: "TraktClientID", Type: "text", Value: configv.TraktClientID},
				{Name: "TraktClientSecret", Type: "password", Value: configv.TraktClientSecret},
				{Name: "TraktRedirectUrl", Type: "text", Value: configv.TraktRedirectUrl},
			}, group, comments, displayNames),

		// Metadata Sources Section
		renderConfigGroup("Metadata Sources", "metadata", false,
			[]FormFieldDefinition{
				{Name: "MovieMetaSourceImdb", Type: "checkbox", Value: configv.MovieMetaSourceImdb},
				{Name: "MovieMetaSourceTmdb", Type: "checkbox", Value: configv.MovieMetaSourceTmdb},
				{Name: "MovieMetaSourceOmdb", Type: "checkbox", Value: configv.MovieMetaSourceOmdb},
				{
					Name:  "MovieMetaSourceTrakt",
					Type:  "checkbox",
					Value: configv.MovieMetaSourceTrakt,
				},
				{Name: "MovieMetaSourcePlex", Type: "checkbox", Value: configv.MovieMetaSourcePlex},
				{Name: "MovieMetaSourceJellyfin", Type: "checkbox", Value: configv.MovieMetaSourceJellyfin},
				{
					Name:  "MovieAlternateTitleMetaSourceImdb",
					Type:  "checkbox",
					Value: configv.MovieAlternateTitleMetaSourceImdb,
				},
				{
					Name:  "MovieAlternateTitleMetaSourceTmdb",
					Type:  "checkbox",
					Value: configv.MovieAlternateTitleMetaSourceTmdb,
				},
				{
					Name:  "MovieAlternateTitleMetaSourceOmdb",
					Type:  "checkbox",
					Value: configv.MovieAlternateTitleMetaSourceOmdb,
				},
				{
					Name:  "MovieAlternateTitleMetaSourceTrakt",
					Type:  "checkbox",
					Value: configv.MovieAlternateTitleMetaSourceTrakt,
				},
				{
					Name:  "SerieAlternateTitleMetaSourceImdb",
					Type:  "checkbox",
					Value: configv.SerieAlternateTitleMetaSourceImdb,
				},
				{
					Name:  "SerieAlternateTitleMetaSourceTrakt",
					Type:  "checkbox",
					Value: configv.SerieAlternateTitleMetaSourceTrakt,
				},
				{Name: "SerieMetaSourceTmdb", Type: "checkbox", Value: configv.SerieMetaSourceTmdb},
				{
					Name:  "SerieMetaSourceTrakt",
					Type:  "checkbox",
					Value: configv.SerieMetaSourceTrakt,
				},
				{Name: "SerieMetaSourcePlex", Type: "checkbox", Value: configv.SerieMetaSourcePlex},
				{Name: "SerieMetaSourceJellyfin", Type: "checkbox", Value: configv.SerieMetaSourceJellyfin},
				{Name: "PlexServerURL", Type: "text", Value: configv.PlexServerURL},
				{Name: "PlexToken", Type: "password", Value: configv.PlexToken},
				{Name: "JellyfinServerURL", Type: "text", Value: configv.JellyfinServerURL},
				{Name: "JellyfinToken", Type: "password", Value: configv.JellyfinToken},
				{Name: "CoverCacheDir", Type: "text", Value: configv.CoverCacheDir},
				{Name: "PlayerVideoProfiles", Type: "array", Value: configv.PlayerVideoProfiles},
				{Name: "PlayerAudioBitrates", Type: "arrayint", Value: configv.PlayerAudioBitrates},
				{Name: "PlayerMaxTranscodes", Type: "number", Value: configv.PlayerMaxTranscodes},
				{Name: "PlayerMaxBandwidth", Type: "number", Value: configv.PlayerMaxBandwidth},
				{Name: "PlayerSpeedTestMinutes", Type: "number", Value: configv.PlayerSpeedTestMinutes},
				{Name: "UserMailNotification", Type: "text", Value: configv.UserMailNotification},
				{Name: "UserPushNotification", Type: "text", Value: configv.UserPushNotification},
				{
					Name:  "PlayerHWAccel",
					Type:  "select",
					Value: configv.PlayerHWAccel,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"none", "vaapi", "qsv"},
					}),
				},
				{Name: "PlayerHWDevice", Type: "text", Value: configv.PlayerHWDevice},
			}, group, comments, displayNames),

		// Apps and devices: WebDAV, Jellyfin, Subsonic, OPDS, DLNA
		renderConfigGroup("Apps and Devices", "clients", false,
			[]FormFieldDefinition{
				{Name: "ServerName", Type: "text", Value: configv.ServerName},
				{Name: "ServeWebDAV", Type: "checkbox", Value: configv.ServeWebDAV},
				{Name: "ServeJellyfin", Type: "checkbox", Value: configv.ServeJellyfin},
				{Name: "ServeSubsonic", Type: "checkbox", Value: configv.ServeSubsonic},
				{Name: "ServeOPDS", Type: "checkbox", Value: configv.ServeOPDS},
				{Name: "ServeDLNA", Type: "checkbox", Value: configv.ServeDLNA},
				{Name: "DLNAUser", Type: "text", Value: configv.DLNAUser},
				{Name: "DLNAURL", Type: "text", Value: configv.DLNAURL},
				{Name: "WatchedSyncUser", Type: "text", Value: configv.WatchedSyncUser},
				{Name: "WatchedSyncPlex", Type: "checkbox", Value: configv.WatchedSyncPlex},
				{Name: "WatchedSyncJellyfinUser", Type: "text", Value: configv.WatchedSyncJellyfinUser},
			}, group, comments, displayNames),

		// Rate Limiting Section
		renderConfigGroup("Rate Limiting", "limits", false,
			[]FormFieldDefinition{
				{Name: "TraktLimiterSeconds", Type: "number", Value: configv.TraktLimiterSeconds},
				{Name: "TraktLimiterCalls", Type: "number", Value: configv.TraktLimiterCalls},
				{Name: "TvdbLimiterSeconds", Type: "number", Value: configv.TvdbLimiterSeconds},
				{Name: "TvdbLimiterCalls", Type: "number", Value: configv.TvdbLimiterCalls},
				{Name: "TmdbLimiterSeconds", Type: "number", Value: configv.TmdbLimiterSeconds},
				{Name: "TmdbLimiterCalls", Type: "number", Value: configv.TmdbLimiterCalls},
				{Name: "OmdbLimiterSeconds", Type: "number", Value: configv.OmdbLimiterSeconds},
				{Name: "OmdbLimiterCalls", Type: "number", Value: configv.OmdbLimiterCalls},
				{Name: "TvmazeLimiterSeconds", Type: "number", Value: configv.TvmazeLimiterSeconds},
				{Name: "TvmazeLimiterCalls", Type: "number", Value: configv.TvmazeLimiterCalls},
				{Name: "PlexLimiterSeconds", Type: "number", Value: configv.PlexLimiterSeconds},
				{Name: "PlexLimiterCalls", Type: "number", Value: configv.PlexLimiterCalls},
				{Name: "PlexTimeoutSeconds", Type: "number", Value: configv.PlexTimeoutSeconds},
				{
					Name:  "PlexDisableTLSVerify",
					Type:  "checkbox",
					Value: configv.PlexDisableTLSVerify,
				},
				{
					Name:  "JellyfinLimiterSeconds",
					Type:  "number",
					Value: configv.JellyfinLimiterSeconds,
				},
				{Name: "JellyfinLimiterCalls", Type: "number", Value: configv.JellyfinLimiterCalls},
				{
					Name:  "JellyfinTimeoutSeconds",
					Type:  "number",
					Value: configv.JellyfinTimeoutSeconds,
				},
				{
					Name:  "JellyfinDisableTLSVerify",
					Type:  "checkbox",
					Value: configv.JellyfinDisableTLSVerify,
				},
			}, group, comments, displayNames),

		// External Tools
		renderConfigGroup("External Tools", "external", false,
			[]FormFieldDefinition{
				{Name: "FfprobePath", Type: "text", Value: configv.FfprobePath},
				{Name: "MediainfoPath", Type: "text", Value: configv.MediainfoPath},
				{Name: "MetaflacPath", Type: "text", Value: configv.MetaflacPath},
				{Name: "FpcalcPath", Type: "text", Value: configv.FpcalcPath},
				{Name: "UseMediainfo", Type: "checkbox", Value: configv.UseMediainfo},
				{Name: "UseMediaFallback", Type: "checkbox", Value: configv.UseMediaFallback},
				{Name: "UnrarPath", Type: "text", Value: configv.UnrarPath},
				{Name: "SevenZipPath", Type: "text", Value: configv.SevenZipPath},
				{Name: "WhisperURL", Type: "text", Value: configv.WhisperURL},
				{Name: "HeadlessChromeURL", Type: "text", Value: configv.HeadlessChromeURL},
				{Name: "UnzipPath", Type: "text", Value: configv.UnzipPath},
				{Name: "TarPath", Type: "text", Value: configv.TarPath},
			}, group, comments, displayNames),

		// Book/Audiobook/Music Provider API Keys
		renderConfigGroup("Book/Music Provider API Keys", "provider_keys", false,
			[]FormFieldDefinition{
				{Name: "GoodreadsAPIKey", Type: "password", Value: configv.GoodreadsAPIKey},
				{Name: "DiscogsToken", Type: "password", Value: configv.DiscogsToken},
				{Name: "AcoustIDAPIKey", Type: "password", Value: configv.AcoustIDAPIKey},
				{Name: "LastFMAPIKey", Type: "password", Value: configv.LastFMAPIKey},
				{Name: "TheAudioDBAPIKey", Type: "password", Value: configv.TheAudioDBAPIKey},
			}, group, comments, displayNames),

		// Book/Audiobook/Music Provider Rate Limits
		renderConfigGroup("Book/Music Provider Rate Limits", "provider_limits", false,
			[]FormFieldDefinition{
				{
					Name:  "OpenLibraryLimiterSeconds",
					Type:  "number",
					Value: configv.OpenLibraryLimiterSeconds,
				},
				{
					Name:  "OpenLibraryLimiterCalls",
					Type:  "number",
					Value: configv.OpenLibraryLimiterCalls,
				},
				{
					Name:  "GoodreadsLimiterSeconds",
					Type:  "number",
					Value: configv.GoodreadsLimiterSeconds,
				},
				{
					Name:  "GoodreadsLimiterCalls",
					Type:  "number",
					Value: configv.GoodreadsLimiterCalls,
				},
				{
					Name:  "AudibleLimiterSeconds",
					Type:  "number",
					Value: configv.AudibleLimiterSeconds,
				},
				{Name: "AudibleLimiterCalls", Type: "number", Value: configv.AudibleLimiterCalls},
				{Name: "AudnexLimiterSeconds", Type: "number", Value: configv.AudnexLimiterSeconds},
				{Name: "AudnexLimiterCalls", Type: "number", Value: configv.AudnexLimiterCalls},
				{
					Name:  "MusicBrainzLimiterSeconds",
					Type:  "number",
					Value: configv.MusicBrainzLimiterSeconds,
				},
				{
					Name:  "MusicBrainzLimiterCalls",
					Type:  "number",
					Value: configv.MusicBrainzLimiterCalls,
				},
				{
					Name:  "DiscogsLimiterSeconds",
					Type:  "number",
					Value: configv.DiscogsLimiterSeconds,
				},
				{Name: "DiscogsLimiterCalls", Type: "number", Value: configv.DiscogsLimiterCalls},
				{
					Name:  "AcoustIDLimiterSeconds",
					Type:  "number",
					Value: configv.AcoustIDLimiterSeconds,
				},
				{Name: "AcoustIDLimiterCalls", Type: "number", Value: configv.AcoustIDLimiterCalls},
				{Name: "LastFMLimiterSeconds", Type: "number", Value: configv.LastFMLimiterSeconds},
				{Name: "LastFMLimiterCalls", Type: "number", Value: configv.LastFMLimiterCalls},
				{Name: "DeezerLimiterSeconds", Type: "number", Value: configv.DeezerLimiterSeconds},
				{Name: "DeezerLimiterCalls", Type: "number", Value: configv.DeezerLimiterCalls},
				{Name: "ITunesLimiterSeconds", Type: "number", Value: configv.ITunesLimiterSeconds},
				{Name: "ITunesLimiterCalls", Type: "number", Value: configv.ITunesLimiterCalls},
				{
					Name:  "TheAudioDBLimiterSeconds",
					Type:  "number",
					Value: configv.TheAudioDBLimiterSeconds,
				},
				{
					Name:  "TheAudioDBLimiterCalls",
					Type:  "number",
					Value: configv.TheAudioDBLimiterCalls,
				},
			}, group, comments, displayNames),

		// Music Metadata Sources
		renderConfigGroup("Music Metadata Sources", "music_meta_sources", false,
			[]FormFieldDefinition{
				{
					Name:  "MusicMetaSourcePriority",
					Type:  "array",
					Value: configv.MusicMetaSourcePriority,
				},
				{
					Name:  "MusicMetaSourcePenalties",
					Type:  "floatmap",
					Value: configv.MusicMetaSourcePenalties,
				},
			}, group, comments, displayNames),

		// Advanced Settings Section
		renderConfigGroup("Advanced Settings", "advanced", false,
			[]FormFieldDefinition{
				{
					Name:  "MovieMetaSourcePriority",
					Type:  "array",
					Value: configv.MovieMetaSourcePriority,
				},
				{
					Name:  "MovieRSSMetaSourcePriority",
					Type:  "array",
					Value: configv.MovieRSSMetaSourcePriority,
				},
				{
					Name:  "MovieParseMetaSourcePriority",
					Type:  "array",
					Value: configv.MovieParseMetaSourcePriority,
				},
				{
					Name:  "SerieMetaSourcePriority",
					Type:  "array",
					Value: configv.SerieMetaSourcePriority,
				},
				{Name: "MoveBufferSizeKB", Type: "number", Value: configv.MoveBufferSizeKB},
				{Name: "SchedulerDisabled", Type: "checkbox", Value: configv.SchedulerDisabled},
				{
					Name:  "DisableParserStringMatch",
					Type:  "checkbox",
					Value: configv.DisableParserStringMatch,
				},
				{
					Name:  "UseCronInsteadOfInterval",
					Type:  "checkbox",
					Value: configv.UseCronInsteadOfInterval,
				},
				{Name: "UseFileBufferCopy", Type: "checkbox", Value: configv.UseFileBufferCopy},
				// {Name: "DisableSwagger", Type: "checkbox", Value: configv.DisableSwagger},
				{
					Name:  "TheMovieDBDisableTLSVerify",
					Type:  "checkbox",
					Value: configv.TheMovieDBDisableTLSVerify,
				},
				{
					Name:  "TraktDisableTLSVerify",
					Type:  "checkbox",
					Value: configv.TraktDisableTLSVerify,
				},
				{
					Name:  "OmdbDisableTLSVerify",
					Type:  "checkbox",
					Value: configv.OmdbDisableTLSVerify,
				},
				{
					Name:  "TvdbDisableTLSVerify",
					Type:  "checkbox",
					Value: configv.TvdbDisableTLSVerify,
				},
				{
					Name:  "TvmazeDisableTLSVerify",
					Type:  "checkbox",
					Value: configv.TvmazeDisableTLSVerify,
				},
				{
					Name:  "FailedIndexerBlockTime",
					Type:  "number",
					Value: configv.FailedIndexerBlockTime,
				},
				{Name: "MaxDatabaseBackups", Type: "number", Value: configv.MaxDatabaseBackups},
				{
					Name:  "DatabaseBackupStopTasks",
					Type:  "checkbox",
					Value: configv.DatabaseBackupStopTasks,
				},
				// {
				// 	Name:  "DisableVariableCleanup",
				// 	Type:  "checkbox",
				// 	Value: configv.DisableVariableCleanup,
				// },
				{Name: "OmdbTimeoutSeconds", Type: "number", Value: configv.OmdbTimeoutSeconds},
				{Name: "TmdbTimeoutSeconds", Type: "number", Value: configv.TmdbTimeoutSeconds},
				{Name: "TvdbTimeoutSeconds", Type: "number", Value: configv.TvdbTimeoutSeconds},
				{Name: "TraktTimeoutSeconds", Type: "number", Value: configv.TraktTimeoutSeconds},
				{Name: "TvmazeTimeoutSeconds", Type: "number", Value: configv.TvmazeTimeoutSeconds},
				{Name: "EnableFileWatcher", Type: "checkbox", Value: configv.EnableFileWatcher},
			}, group, comments, displayNames),
	)
}

// renderImdbConfig renders the IMDB configuration section.
func renderImdbConfig(configv *config.ImdbConfig, csrfToken string) gomponents.Node {
	group := "imdb"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	fields := createImdbConfigFields(configv)
	formGroups := renderFormFields(group, comments, displayNames, fields)

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-film",
			"IMDB Configuration",
			"Configure IMDB database settings including titles, ratings, episodes, and cast information.",
		),

		html.Form(
			html.Class("config-form"),
			gomponents.Group(formGroups),

			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				"/api/admin/config/imdb/update",
				csrfToken,
			),
			html.Div(html.ID("addalert")),
		),
	)
}
