package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	hx "maragu.dev/gomponents-htmx"
)

// fieldNameToUserFriendly converts field names like "LogFileSize" to "Log File Size"
func fieldNameToUserFriendly(fieldName string) string {
	// Add space before capital letters (except the first one)
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	result := re.ReplaceAllString(fieldName, `${1} ${2}`)

	// Handle common abbreviations and acronyms
	replacements := map[string]string{
		"Url":     "URL",
		"Id":      "ID",
		"Db":      "Database",
		"Api":     "API",
		"Http":    "HTTP",
		"Ssl":     "SSL",
		"Tls":     "TLS",
		"Csv":     "CSV",
		"Json":    "JSON",
		"Xml":     "XML",
		"Html":    "HTML",
		"Imdb":    "IMDB",
		"Tvdb":    "TVDB",
		"Tmdb":    "TMDB",
		"Newznab": "Newznab",
		"Nzb":     "NZB",
		"Rss":     "RSS",
		"Mb":      "MB",
		"Gb":      "GB",
		"Kb":      "KB",
	}

	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

// renderGeneralConfig renders the general configuration section
func renderGeneralConfig(configv *config.GeneralConfig, csrfToken string) Node {
	group := "general"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("config-section"),
		H3(Text("General Configuration")),

		Form(
			Class("config-form"),

			renderFormGroup(group, comments, displayNames, "TimeFormat", "select", configv.TimeFormat, map[string][]string{
				"options": {"rfc3339", "iso8601", "rfc1123", "rfc822", "rfc850"},
			}),
			renderFormGroup(group, comments, displayNames, "TimeZone", "text", configv.TimeZone, nil),
			renderFormGroup(group, comments, displayNames, "LogLevel", "select", configv.LogLevel, map[string][]string{
				"options": {"info", "debug"},
			}),
			renderFormGroup(group, comments, displayNames, "DBLogLevel", "select", configv.DBLogLevel, map[string][]string{
				"options": {"info", "debug"},
			}),
			renderFormGroup(group, comments, displayNames, "LogFileSize", "number", configv.LogFileSize, nil),
			renderFormGroup(group, comments, displayNames, "LogFileCount", "number", configv.LogFileCount, nil),

			renderFormGroup(group, comments, displayNames, "LogCompress", "checkbox", configv.LogCompress, nil),
			renderFormGroup(group, comments, displayNames, "LogToFileOnly", "checkbox", configv.LogToFileOnly, nil),
			renderFormGroup(group, comments, displayNames, "LogColorize", "checkbox", configv.LogColorize, nil),
			renderFormGroup(group, comments, displayNames, "LogZeroValues", "checkbox", configv.LogZeroValues, nil),
			renderFormGroup(group, comments, displayNames, "WorkerMetadata", "number", configv.WorkerMetadata, nil),
			renderFormGroup(group, comments, displayNames, "WorkerFiles", "number", configv.WorkerFiles, nil),
			renderFormGroup(group, comments, displayNames, "WorkerParse", "number", configv.WorkerParse, nil),
			renderFormGroup(group, comments, displayNames, "WorkerSearch", "number", configv.WorkerSearch, nil),
			renderFormGroup(group, comments, displayNames, "WorkerRSS", "number", configv.WorkerRSS, nil),
			renderFormGroup(group, comments, displayNames, "WorkerIndexer", "number", configv.WorkerIndexer, nil),
			renderFormGroup(group, comments, displayNames, "OmdbAPIKey", "text", configv.OmdbAPIKey, nil),
			renderFormGroup(group, comments, displayNames, "UseMediaCache", "checkbox", configv.UseMediaCache, nil),
			renderFormGroup(group, comments, displayNames, "UseFileCache", "checkbox", configv.UseFileCache, nil),
			renderFormGroup(group, comments, displayNames, "UseHistoryCache", "checkbox", configv.UseHistoryCache, nil),
			renderFormGroup(group, comments, displayNames, "CacheDuration", "number", configv.CacheDuration, nil),
			renderFormGroup(group, comments, displayNames, "CacheAutoExtend", "checkbox", configv.CacheAutoExtend, nil),
			renderFormGroup(group, comments, displayNames, "SearcherSize", "number", configv.SearcherSize, nil),
			renderFormGroup(group, comments, displayNames, "MovieMetaSourceImdb", "checkbox", configv.MovieMetaSourceImdb, nil),
			renderFormGroup(group, comments, displayNames, "MovieMetaSourceTmdb", "checkbox", configv.MovieMetaSourceTmdb, nil),
			renderFormGroup(group, comments, displayNames, "MovieMetaSourceOmdb", "checkbox", configv.MovieMetaSourceOmdb, nil),
			renderFormGroup(group, comments, displayNames, "MovieMetaSourceTrakt", "checkbox", configv.MovieMetaSourceTrakt, nil),
			renderFormGroup(group, comments, displayNames, "MovieAlternateTitleMetaSourceImdb", "checkbox", configv.MovieAlternateTitleMetaSourceImdb, nil),
			renderFormGroup(group, comments, displayNames, "MovieAlternateTitleMetaSourceTmdb", "checkbox", configv.MovieAlternateTitleMetaSourceTmdb, nil),
			renderFormGroup(group, comments, displayNames, "MovieAlternateTitleMetaSourceOmdb", "checkbox", configv.MovieAlternateTitleMetaSourceOmdb, nil),
			renderFormGroup(group, comments, displayNames, "MovieAlternateTitleMetaSourceTrakt", "checkbox", configv.MovieAlternateTitleMetaSourceTrakt, nil),
			renderFormGroup(group, comments, displayNames, "SerieAlternateTitleMetaSourceImdb", "checkbox", configv.SerieAlternateTitleMetaSourceImdb, nil),
			renderFormGroup(group, comments, displayNames, "SerieAlternateTitleMetaSourceTrakt", "checkbox", configv.SerieAlternateTitleMetaSourceTrakt, nil),
			renderFormGroup(group, comments, displayNames, "MovieMetaSourcePriority", "array", configv.MovieMetaSourcePriority, nil),
			renderFormGroup(group, comments, displayNames, "MovieRSSMetaSourcePriority", "array", configv.MovieRSSMetaSourcePriority, nil),
			renderFormGroup(group, comments, displayNames, "MovieParseMetaSourcePriority", "array", configv.MovieParseMetaSourcePriority, nil),
			renderFormGroup(group, comments, displayNames, "SerieMetaSourceTmdb", "checkbox", configv.SerieMetaSourceTmdb, nil),
			renderFormGroup(group, comments, displayNames, "SerieMetaSourceTrakt", "checkbox", configv.SerieMetaSourceTrakt, nil),
			renderFormGroup(group, comments, displayNames, "MoveBufferSizeKB", "number", configv.MoveBufferSizeKB, nil),
			renderFormGroup(group, comments, displayNames, "WebPort", "text", configv.WebPort, nil),
			renderFormGroup(group, comments, displayNames, "WebAPIKey", "text", configv.WebAPIKey, nil),
			renderFormGroup(group, comments, displayNames, "WebPortalEnabled", "checkbox", configv.WebPortalEnabled, nil),
			renderFormGroup(group, comments, displayNames, "TheMovieDBApiKey", "text", configv.TheMovieDBApiKey, nil),
			renderFormGroup(group, comments, displayNames, "TraktClientID", "text", configv.TraktClientID, nil),
			renderFormGroup(group, comments, displayNames, "TraktClientSecret", "text", configv.TraktClientSecret, nil),
			renderFormGroup(group, comments, displayNames, "SchedulerDisabled", "checkbox", configv.SchedulerDisabled, nil),
			renderFormGroup(group, comments, displayNames, "DisableParserStringMatch", "checkbox", configv.DisableParserStringMatch, nil),
			renderFormGroup(group, comments, displayNames, "UseCronInsteadOfInterval", "checkbox", configv.UseCronInsteadOfInterval, nil),
			renderFormGroup(group, comments, displayNames, "UseFileBufferCopy", "checkbox", configv.UseFileBufferCopy, nil),
			renderFormGroup(group, comments, displayNames, "DisableSwagger", "checkbox", configv.DisableSwagger, nil),
			renderFormGroup(group, comments, displayNames, "TraktLimiterSeconds", "number", configv.TraktLimiterSeconds, nil),
			renderFormGroup(group, comments, displayNames, "TraktLimiterCalls", "number", configv.TraktLimiterCalls, nil),
			renderFormGroup(group, comments, displayNames, "TvdbLimiterSeconds", "number", configv.TvdbLimiterSeconds, nil),
			renderFormGroup(group, comments, displayNames, "TvdbLimiterCalls", "number", configv.TvdbLimiterCalls, nil),
			renderFormGroup(group, comments, displayNames, "TmdbLimiterSeconds", "number", configv.TmdbLimiterSeconds, nil),
			renderFormGroup(group, comments, displayNames, "TmdbLimiterCalls", "number", configv.TmdbLimiterCalls, nil),
			renderFormGroup(group, comments, displayNames, "OmdbLimiterSeconds", "number", configv.OmdbLimiterSeconds, nil),
			renderFormGroup(group, comments, displayNames, "OmdbLimiterCalls", "number", configv.OmdbLimiterCalls, nil),
			renderFormGroup(group, comments, displayNames, "TheMovieDBDisableTLSVerify", "checkbox", configv.TheMovieDBDisableTLSVerify, nil),
			renderFormGroup(group, comments, displayNames, "TraktDisableTLSVerify", "checkbox", configv.TraktDisableTLSVerify, nil),
			renderFormGroup(group, comments, displayNames, "OmdbDisableTLSVerify", "checkbox", configv.OmdbDisableTLSVerify, nil),
			renderFormGroup(group, comments, displayNames, "TvdbDisableTLSVerify", "checkbox", configv.TvdbDisableTLSVerify, nil),
			renderFormGroup(group, comments, displayNames, "FfprobePath", "text", configv.FfprobePath, nil),
			renderFormGroup(group, comments, displayNames, "MediainfoPath", "text", configv.MediainfoPath, nil),
			renderFormGroup(group, comments, displayNames, "UseMediainfo", "checkbox", configv.UseMediainfo, nil),
			renderFormGroup(group, comments, displayNames, "UseMediaFallback", "checkbox", configv.UseMediaFallback, nil),
			renderFormGroup(group, comments, displayNames, "FailedIndexerBlockTime", "number", configv.FailedIndexerBlockTime, nil),
			renderFormGroup(group, comments, displayNames, "MaxDatabaseBackups", "number", configv.MaxDatabaseBackups, nil),
			renderFormGroup(group, comments, displayNames, "DatabaseBackupStopTasks", "checkbox", configv.DatabaseBackupStopTasks, nil),
			renderFormGroup(group, comments, displayNames, "DisableVariableCleanup", "checkbox", configv.DisableVariableCleanup, nil),
			renderFormGroup(group, comments, displayNames, "OmdbTimeoutSeconds", "number", configv.OmdbTimeoutSeconds, nil),
			renderFormGroup(group, comments, displayNames, "TmdbTimeoutSeconds", "number", configv.TmdbTimeoutSeconds, nil),
			renderFormGroup(group, comments, displayNames, "TvdbTimeoutSeconds", "number", configv.TvdbTimeoutSeconds, nil),
			renderFormGroup(group, comments, displayNames, "TraktTimeoutSeconds", "number", configv.TraktTimeoutSeconds, nil),
			renderFormGroup(group, comments, displayNames, "EnableFileWatcher", "checkbox", configv.EnableFileWatcher, nil),

			// Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/general/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		))
}

// Gin handler to process the form submission
func HandleGeneralConfigUpdate(c *gin.Context) {
	// Parse form data
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	// Create a new GeneralConfig struct to populate
	updatedConfig := config.GetToml().General

	// Populate the struct from form values
	updatedConfig.TimeFormat = c.PostForm("general_TimeFormat")
	updatedConfig.TimeZone = c.PostForm("general_TimeZone")
	updatedConfig.LogLevel = c.PostForm("general_LogLevel")
	updatedConfig.DBLogLevel = c.PostForm("general_DBLogLevel")
	updatedConfig.LogFileSize = parseInt(c.PostForm("general_LogFileSize"), 5)
	updatedConfig.LogFileCount = parseUint8(c.PostForm("general_LogFileCount"), 1)
	updatedConfig.WorkerMetadata = parseInt(c.PostForm("general_WorkerMetadata"), 1)
	updatedConfig.WorkerFiles = parseInt(c.PostForm("general_WorkerFiles"), 1)
	updatedConfig.WebPort = c.PostForm("general_WebPort")
	updatedConfig.DisableVariableCleanup = parseBool(c.PostForm("general_DisableVariableCleanup"))
	updatedConfig.DisableParserStringMatch = parseBool(c.PostForm("general_DisableParserStringMatch"))
	updatedConfig.UseMediaCache = parseBool(c.PostForm("general_UseMediaCache"))
	updatedConfig.CacheDuration = parseInt(c.PostForm("general_CacheDuration"), 12)
	updatedConfig.DisableSwagger = parseBool(c.PostForm("general_DisableSwagger"))
	updatedConfig.WebAPIKey = c.PostForm("general_WebAPIKey")
	updatedConfig.LogCompress = parseBool(c.PostForm("general_LogCompress"))
	updatedConfig.LogToFileOnly = parseBool(c.PostForm("general_LogToFileOnly"))
	updatedConfig.LogColorize = parseBool(c.PostForm("general_LogColorize"))
	updatedConfig.LogZeroValues = parseBool(c.PostForm("general_LogZeroValues"))
	updatedConfig.WorkerParse = parseInt(c.PostForm("general_WorkerParse"), 1)
	updatedConfig.WorkerSearch = parseInt(c.PostForm("general_WorkerSearch"), 1)
	updatedConfig.WorkerIndexer = parseInt(c.PostForm("general_WorkerIndexer"), 1)
	updatedConfig.WebPortalEnabled = parseBool(c.PostForm("general_WebPortalEnabled"))
	updatedConfig.TheMovieDBApiKey = c.PostForm("general_TheMovieDBApiKey")
	updatedConfig.TraktClientID = c.PostForm("general_TraktClientID")
	updatedConfig.TraktClientSecret = c.PostForm("general_TraktClientSecret")
	updatedConfig.EnableFileWatcher = parseBool(c.PostForm("general_EnableFileWatcher"))
	updatedConfig.TraktTimeoutSeconds = parseUint16(c.PostForm("general_TraktTimeoutSeconds"), 10)
	updatedConfig.TvdbTimeoutSeconds = parseUint16(c.PostForm("general_TvdbTimeoutSeconds"), 10)
	updatedConfig.TmdbTimeoutSeconds = parseUint16(c.PostForm("general_TmdbTimeoutSeconds"), 10)
	updatedConfig.OmdbTimeoutSeconds = parseUint16(c.PostForm("general_OmdbTimeoutSeconds"), 10)
	updatedConfig.DatabaseBackupStopTasks = parseBool(c.PostForm("general_DatabaseBackupStopTasks"))
	updatedConfig.MaxDatabaseBackups = parseInt(c.PostForm("general_MaxDatabaseBackups"), 0)
	updatedConfig.FailedIndexerBlockTime = parseInt(c.PostForm("general_FailedIndexerBlockTime"), 5)
	updatedConfig.UseMediaFallback = parseBool(c.PostForm("general_UseMediaFallback"))
	updatedConfig.UseMediainfo = parseBool(c.PostForm("general_UseMediainfo"))
	updatedConfig.MediainfoPath = c.PostForm("general_MediainfoPath")
	updatedConfig.FfprobePath = c.PostForm("general_FfprobePath")
	updatedConfig.TvdbDisableTLSVerify = parseBool(c.PostForm("general_TvdbDisableTLSVerify"))
	updatedConfig.OmdbDisableTLSVerify = parseBool(c.PostForm("general_OmdbDisableTLSVerify"))
	updatedConfig.TraktDisableTLSVerify = parseBool(c.PostForm("general_TraktDisableTLSVerify"))
	updatedConfig.TheMovieDBDisableTLSVerify = parseBool(c.PostForm("general_TheMovieDBDisableTLSVerify"))
	updatedConfig.OmdbLimiterCalls = parseInt(c.PostForm("general_OmdbLimiterCalls"), 1)
	updatedConfig.OmdbLimiterSeconds = parseUint8(c.PostForm("general_OmdbLimiterSeconds"), 1)
	updatedConfig.TmdbLimiterCalls = parseInt(c.PostForm("general_TmdbLimiterCalls"), 1)
	updatedConfig.TmdbLimiterSeconds = parseUint8(c.PostForm("general_TmdbLimiterSeconds"), 1)
	updatedConfig.TvdbLimiterCalls = parseInt(c.PostForm("general_TvdbLimiterCalls"), 1)
	updatedConfig.TvdbLimiterSeconds = parseUint8(c.PostForm("general_TvdbLimiterSeconds"), 1)
	updatedConfig.TraktLimiterCalls = parseInt(c.PostForm("general_TraktLimiterCalls"), 1)
	updatedConfig.TraktLimiterSeconds = parseUint8(c.PostForm("general_TraktLimiterSeconds"), 1)
	updatedConfig.UseFileBufferCopy = parseBool(c.PostForm("general_UseFileBufferCopy"))
	updatedConfig.UseCronInsteadOfInterval = parseBool(c.PostForm("general_UseCronInsteadOfInterval"))
	updatedConfig.SchedulerDisabled = parseBool(c.PostForm("general_SchedulerDisabled"))
	updatedConfig.MoveBufferSizeKB = parseInt(c.PostForm("general_MoveBufferSizeKB"), 1024)
	updatedConfig.SerieMetaSourceTrakt = parseBool(c.PostForm("general_SerieMetaSourceTrakt"))
	updatedConfig.SerieMetaSourceTmdb = parseBool(c.PostForm("general_SerieMetaSourceTmdb"))
	updatedConfig.MovieParseMetaSourcePriority = parseStringArray(c.PostFormArray("general_MovieParseMetaSourcePriority"))
	updatedConfig.MovieRSSMetaSourcePriority = parseStringArray(c.PostFormArray("general_MovieRSSMetaSourcePriority"))
	updatedConfig.MovieMetaSourcePriority = parseStringArray(c.PostFormArray("general_MovieMetaSourcePriority"))
	updatedConfig.SerieAlternateTitleMetaSourceTrakt = parseBool(c.PostForm("general_SerieAlternateTitleMetaSourceTrakt"))
	updatedConfig.SerieAlternateTitleMetaSourceImdb = parseBool(c.PostForm("general_SerieAlternateTitleMetaSourceImdb"))
	updatedConfig.MovieAlternateTitleMetaSourceTrakt = parseBool(c.PostForm("general_MovieAlternateTitleMetaSourceTrakt"))
	updatedConfig.MovieAlternateTitleMetaSourceOmdb = parseBool(c.PostForm("general_MovieAlternateTitleMetaSourceOmdb"))
	updatedConfig.MovieAlternateTitleMetaSourceTmdb = parseBool(c.PostForm("general_MovieAlternateTitleMetaSourceTmdb"))
	updatedConfig.MovieAlternateTitleMetaSourceImdb = parseBool(c.PostForm("general_MovieAlternateTitleMetaSourceImdb"))
	updatedConfig.MovieMetaSourceTrakt = parseBool(c.PostForm("general_MovieMetaSourceTrakt"))
	updatedConfig.MovieMetaSourceOmdb = parseBool(c.PostForm("general_MovieMetaSourceOmdb"))
	updatedConfig.MovieMetaSourceTmdb = parseBool(c.PostForm("general_MovieMetaSourceTmdb"))
	updatedConfig.MovieMetaSourceImdb = parseBool(c.PostForm("general_MovieMetaSourceImdb"))
	updatedConfig.SearcherSize = parseInt(c.PostForm("general_SearcherSize"), 1)
	updatedConfig.CacheAutoExtend = parseBool(c.PostForm("general_CacheAutoExtend"))
	updatedConfig.UseHistoryCache = parseBool(c.PostForm("general_UseHistoryCache"))
	updatedConfig.UseFileCache = parseBool(c.PostForm("general_UseFileCache"))
	updatedConfig.OmdbAPIKey = c.PostForm("general_OmdbAPIKey")
	updatedConfig.WorkerRSS = parseInt(c.PostForm("general_WorkerRSS"), 1)

	// Validate the configuration (you may want to add custom validation)
	if err := validateGeneralConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update %s", err.Error()), "danger"))
		return
	}

	// Save the configuration (implement your save logic here)
	if err := saveConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update %s", err.Error()), "danger"))
		return
	}

	// Respond with success
	c.String(http.StatusOK, renderAlert("Update successful", "success"))
}

// Validation function
func validateGeneralConfig(config *config.GeneralConfig) error {
	// Add your validation logic here
	if config.TimeFormat == "" {
		return errors.New("time format cannot be empty")
	}

	validTimeFormats := []string{"rfc3339", "iso8601", "rfc1123", "rfc822", "rfc850"}
	isValidTimeFormat := false
	for _, format := range validTimeFormats {
		if config.TimeFormat == format {
			isValidTimeFormat = true
			break
		}
	}
	if !isValidTimeFormat {
		return errors.New("invalid time format")
	}

	if config.LogLevel != "info" && config.LogLevel != "debug" {
		return errors.New("log level must be 'info' or 'debug'")
	}

	if config.LogFileSize <= 0 {
		return errors.New("log file size must be greater than 0")
	}

	if config.WebPort == "" {
		return errors.New("web port cannot be empty")
	}

	return nil
}

func renderAlert(message string, typev string) string {
	nodev := Div(
		Class("alert alert-"+typev+" alert-outline-coloured alert-dismissible"),
		Role("alert"),
		Button(
			Type("button"),
			Class("btn-close"),
			Data("bs-dismiss", "alert"),
			Aria("label", "Close"),
		),
		Div(
			Class("alert-icon"),
			I(
				Class("far fa-fw fa-bell"),
			),
		),
		Div(
			Class("alert-message"),
			Strong(
				Text(message),
			),
		),
	)
	var buf strings.Builder
	nodev.Render(&buf)
	return buf.String()
}

// HandleImdbConfigUpdate handles IMDB configuration updates
func HandleImdbConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	updatedConfig := config.GetToml().Imdbindexer

	// Populate the struct from form values
	updatedConfig.Indexedtypes = parseStringArray(c.PostFormArray("imdb_Indexedtypes"))
	updatedConfig.Indexedlanguages = parseStringArray(c.PostFormArray("imdb_Indexedlanguages"))
	updatedConfig.Indexfull = parseBool(c.PostForm("imdb_Indexfull"))
	updatedConfig.ImdbIDSize = parseInt(c.PostForm("imdb_ImdbIDSize"), 100000)
	updatedConfig.LoopSize = parseInt(c.PostForm("imdb_LoopSize"), 1000)
	updatedConfig.UseMemory = parseBool(c.PostForm("imdb_UseMemory"))
	updatedConfig.UseCache = parseBool(c.PostForm("imdb_UseCache"))

	// Validate the configuration
	if err := validateImdbConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	// Save the configuration
	if err := saveConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("IMDB configuration updated successfully", "success"))
}

// HandleMediaConfigUpdate handles media configuration updates
func HandleMediaConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data "+err.Error(), "danger"))
		return
	}

	// Handle media configuration updates - this is complex due to nested structures
	// For movies
	var newConfig config.MediaConfig

	// Parse form to find all regex entries
	formKeys := make(map[string]bool)
	logger.LogDynamicany1Any("info", "log post", "form data", c.Request.Form)
	logger.LogDynamicany1Any("info", "log post", "post data", c.Request.PostForm)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "media_main_movies_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[3]] = true
	}

	newConfig.Movies = make([]config.MediaTypeConfig, 0, len(formKeys))

	for i := range formKeys {
		nameField := fmt.Sprintf("media_main_movies_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		updatedConfig := config.MediaTypeConfig{
			Name: name,
		}

		defaultQualityField := fmt.Sprintf("media_main_movies_%s_DefaultQuality", i)
		if defaultQuality := c.PostForm(defaultQualityField); defaultQuality != "" {
			updatedConfig.DefaultQuality = defaultQuality
		}

		defaultResolutionField := fmt.Sprintf("media_main_movies_%s_DefaultResolution", i)
		if defaultResolution := c.PostForm(defaultResolutionField); defaultResolution != "" {
			updatedConfig.DefaultResolution = defaultResolution
		}

		structureField := fmt.Sprintf("media_main_movies_%s_Structure", i)
		updatedConfig.Structure = parseBool(c.PostForm(structureField))

		searchMissingField := fmt.Sprintf("media_main_movies_%s_SearchmissingIncremental", i)
		if searchMissing := c.PostForm(searchMissingField); searchMissing != "" {
			updatedConfig.SearchmissingIncremental = parseUint16(searchMissing, 0)
		}

		searchUpgradeField := fmt.Sprintf("media_main_movies_%s_SearchupgradeIncremental", i)
		if searchUpgrade := c.PostForm(searchUpgradeField); searchUpgrade != "" {
			updatedConfig.SearchupgradeIncremental = parseUint16(searchUpgrade, 0)
		}

		namingField := fmt.Sprintf("media_main_movies_%s_Naming", i)
		if naming := c.PostForm(namingField); naming != "" {
			updatedConfig.Naming = naming
		}

		templateQualityField := fmt.Sprintf("media_main_movies_%s_TemplateQuality", i)
		if templateQuality := c.PostForm(templateQualityField); templateQuality != "" {
			updatedConfig.TemplateQuality = templateQuality
		}

		templateSchedulerField := fmt.Sprintf("media_main_movies_%s_TemplateScheduler", i)
		if templateScheduler := c.PostForm(templateSchedulerField); templateScheduler != "" {
			updatedConfig.TemplateScheduler = templateScheduler
		}

		metadataLanguageField := fmt.Sprintf("media_main_movies_%s_MetadataLanguage", i)
		if metadataLanguage := c.PostForm(metadataLanguageField); metadataLanguage != "" {
			updatedConfig.MetadataLanguage = metadataLanguage
		}

		metadataTitleLanguagesField := fmt.Sprintf("media_main_movies_%s_MetadataTitleLanguages", i)
		if metadataTitleLanguages := c.PostFormArray(metadataTitleLanguagesField); len(metadataTitleLanguages) > 0 {
			var validLanguages []string
			for _, lang := range metadataTitleLanguages {
				if strings.TrimSpace(lang) != "" {
					validLanguages = append(validLanguages, strings.TrimSpace(lang))
				}
			}
			updatedConfig.MetadataTitleLanguages = validLanguages
		}

		// Parse QualityReorder slice
		subformKeys := make(map[string]bool)
		prefix := "media_movies_" + i + "_data"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_TemplatePath")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var dataConfigs []config.MediaDataConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaDataConfig{
				TemplatePath: name,
			}

			if val := c.PostForm(fmt.Sprintf("%s_%s_AddFound", prefix, subIndex)); val != "" {
				addConfig.AddFound, _ = strconv.ParseBool(val)
			}

			if val := c.PostForm(fmt.Sprintf("%s_%s_AddFoundList", prefix, subIndex)); val != "" {
				addConfig.AddFoundList = val
			}

			dataConfigs = append(dataConfigs, addConfig)
		}
		updatedConfig.Data = dataConfigs

		// Parse QualityReorder slice
		subformKeys = make(map[string]bool)
		prefix = "media_movies_" + i + "_dataimport"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_TemplatePath")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var dataImportConfigs []config.MediaDataImportConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaDataImportConfig{
				TemplatePath: name,
			}

			dataImportConfigs = append(dataImportConfigs, addConfig)
		}
		updatedConfig.DataImport = dataImportConfigs

		// Parse QualityReorder slice
		subformKeys = make(map[string]bool)
		prefix = "media_movies_" + i + "_lists"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var listConfigs []config.MediaListsConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_Name", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaListsConfig{
				Name: name,
			}

			if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateList", prefix, subIndex)); val != "" {
				addConfig.TemplateList = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateQuality", prefix, subIndex)); val != "" {
				addConfig.TemplateQuality = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateScheduler", prefix, subIndex)); val != "" {
				addConfig.TemplateScheduler = val
			}
			if val := c.PostFormArray(fmt.Sprintf("%s_%s_IgnoreMapLists", prefix, subIndex)); len(val) != 0 {
				addConfig.IgnoreMapLists = val
			}
			if val := c.PostFormArray(fmt.Sprintf("%s_%s_ReplaceMapLists", prefix, subIndex)); len(val) != 0 {
				addConfig.ReplaceMapLists = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Enabled", prefix, subIndex)); val != "" {
				addConfig.Enabled, _ = strconv.ParseBool(val)
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Addfound", prefix, subIndex)); val != "" {
				addConfig.Addfound, _ = strconv.ParseBool(val)
			}

			listConfigs = append(listConfigs, addConfig)
		}
		updatedConfig.Lists = listConfigs

		// Parse QualityReorder slice
		subformKeys = make(map[string]bool)
		prefix = "media_movies_" + i + "_notification"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_MapNotification")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var notificationConfigs []config.MediaNotificationConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_MapNotification", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaNotificationConfig{
				MapNotification: name,
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Event", prefix, subIndex)); val != "" {
				addConfig.Event = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Title", prefix, subIndex)); val != "" {
				addConfig.Title = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Message", prefix, subIndex)); val != "" {
				addConfig.Message = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_ReplacedPrefix", prefix, subIndex)); val != "" {
				addConfig.ReplacedPrefix = val
			}

			notificationConfigs = append(notificationConfigs, addConfig)
		}
		updatedConfig.Notification = notificationConfigs

		newConfig.Movies = append(newConfig.Movies, updatedConfig)
	}

	formKeys = make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "media_main_series_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[3]] = true
	}

	newConfig.Series = make([]config.MediaTypeConfig, 0, len(formKeys))

	for i := range formKeys {
		nameField := fmt.Sprintf("media_main_series_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		updatedConfig := config.MediaTypeConfig{
			Name: name,
		}

		defaultQualityField := fmt.Sprintf("media_main_series_%s_DefaultQuality", i)
		if defaultQuality := c.PostForm(defaultQualityField); defaultQuality != "" {
			updatedConfig.DefaultQuality = defaultQuality
		}

		defaultResolutionField := fmt.Sprintf("media_main_series_%s_DefaultResolution", i)
		if defaultResolution := c.PostForm(defaultResolutionField); defaultResolution != "" {
			updatedConfig.DefaultResolution = defaultResolution
		}

		structureField := fmt.Sprintf("media_main_series_%s_Structure", i)
		updatedConfig.Structure = parseBool(c.PostForm(structureField))

		searchMissingField := fmt.Sprintf("media_main_series_%s_SearchmissingIncremental", i)
		if searchMissing := c.PostForm(searchMissingField); searchMissing != "" {
			updatedConfig.SearchmissingIncremental = parseUint16(searchMissing, 0)
		}

		searchUpgradeField := fmt.Sprintf("media_main_series_%s_SearchupgradeIncremental", i)
		if searchUpgrade := c.PostForm(searchUpgradeField); searchUpgrade != "" {
			updatedConfig.SearchupgradeIncremental = parseUint16(searchUpgrade, 0)
		}

		namingField := fmt.Sprintf("media_main_series_%s_Naming", i)
		if naming := c.PostForm(namingField); naming != "" {
			updatedConfig.Naming = naming
		}

		templateQualityField := fmt.Sprintf("media_main_series_%s_TemplateQuality", i)
		if templateQuality := c.PostForm(templateQualityField); templateQuality != "" {
			updatedConfig.TemplateQuality = templateQuality
		}

		templateSchedulerField := fmt.Sprintf("media_main_series_%s_TemplateScheduler", i)
		if templateScheduler := c.PostForm(templateSchedulerField); templateScheduler != "" {
			updatedConfig.TemplateScheduler = templateScheduler
		}

		metadataLanguageField := fmt.Sprintf("media_main_series_%s_MetadataLanguage", i)
		if metadataLanguage := c.PostForm(metadataLanguageField); metadataLanguage != "" {
			updatedConfig.MetadataLanguage = metadataLanguage
		}

		metadataTitleLanguagesField := fmt.Sprintf("media_main_series_%s_MetadataTitleLanguages", i)
		if metadataTitleLanguages := c.PostFormArray(metadataTitleLanguagesField); len(metadataTitleLanguages) > 0 {
			var validLanguages []string
			for _, lang := range metadataTitleLanguages {
				if strings.TrimSpace(lang) != "" {
					validLanguages = append(validLanguages, strings.TrimSpace(lang))
				}
			}
			updatedConfig.MetadataTitleLanguages = validLanguages
		}

		// Parse QualityReorder slice
		subformKeys := make(map[string]bool)
		prefix := "media_movies_" + i + "_data"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_TemplatePath")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var dataConfigs []config.MediaDataConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaDataConfig{
				TemplatePath: name,
			}

			if val := c.PostForm(fmt.Sprintf("%s_%s_AddFound", prefix, subIndex)); val != "" {
				addConfig.AddFound, _ = strconv.ParseBool(val)
			}

			if val := c.PostForm(fmt.Sprintf("%s_%s_AddFoundList", prefix, subIndex)); val != "" {
				addConfig.AddFoundList = val
			}

			dataConfigs = append(dataConfigs, addConfig)
		}
		updatedConfig.Data = dataConfigs

		// Parse QualityReorder slice
		subformKeys = make(map[string]bool)
		prefix = "media_movies_" + i + "_dataimport"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_TemplatePath")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var dataImportConfigs []config.MediaDataImportConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaDataImportConfig{
				TemplatePath: name,
			}

			dataImportConfigs = append(dataImportConfigs, addConfig)
		}
		updatedConfig.DataImport = dataImportConfigs

		// Parse QualityReorder slice
		subformKeys = make(map[string]bool)
		prefix = "media_movies_" + i + "_lists"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var listConfigs []config.MediaListsConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_Name", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaListsConfig{
				Name: name,
			}

			if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateList", prefix, subIndex)); val != "" {
				addConfig.TemplateList = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateQuality", prefix, subIndex)); val != "" {
				addConfig.TemplateQuality = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateScheduler", prefix, subIndex)); val != "" {
				addConfig.TemplateScheduler = val
			}
			if val := c.PostFormArray(fmt.Sprintf("%s_%s_IgnoreMapLists", prefix, subIndex)); len(val) != 0 {
				addConfig.IgnoreMapLists = val
			}
			if val := c.PostFormArray(fmt.Sprintf("%s_%s_ReplaceMapLists", prefix, subIndex)); len(val) != 0 {
				addConfig.ReplaceMapLists = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Enabled", prefix, subIndex)); val != "" {
				addConfig.Enabled, _ = strconv.ParseBool(val)
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Addfound", prefix, subIndex)); val != "" {
				addConfig.Addfound, _ = strconv.ParseBool(val)
			}

			listConfigs = append(listConfigs, addConfig)
		}
		updatedConfig.Lists = listConfigs

		// Parse QualityReorder slice
		subformKeys = make(map[string]bool)
		prefix = "media_movies_" + i + "_notification"
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_MapNotification")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[4]] = true
		}

		var notificationConfigs []config.MediaNotificationConfig
		// Process each form entry
		for subIndex := range subformKeys {
			nameField := fmt.Sprintf("%s_%s_MapNotification", prefix, subIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.MediaNotificationConfig{
				MapNotification: name,
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Event", prefix, subIndex)); val != "" {
				addConfig.Event = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Title", prefix, subIndex)); val != "" {
				addConfig.Title = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_Message", prefix, subIndex)); val != "" {
				addConfig.Message = val
			}
			if val := c.PostForm(fmt.Sprintf("%s_%s_ReplacedPrefix", prefix, subIndex)); val != "" {
				addConfig.ReplacedPrefix = val
			}

			notificationConfigs = append(notificationConfigs, addConfig)
		}
		updatedConfig.Notification = notificationConfigs

		newConfig.Series = append(newConfig.Series, updatedConfig)
	}

	if err := validateMediaConfig(&newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(&newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Media configuration updated successfully", "success"))
}

// validateMediaConfig validates media configuration
func validateMediaConfig(config *config.MediaConfig) error {
	// Validate movies
	for i, movie := range config.Movies {
		if movie.Name == "" {
			return fmt.Errorf("movie configuration %d: name cannot be empty", i)
		}
	}

	// Validate series
	for i, series := range config.Series {
		if series.Name == "" {
			return fmt.Errorf("series configuration %d: name cannot be empty", i)
		}
	}

	return nil
}

// validateImdbConfig validates IMDB configuration
func validateImdbConfig(config *config.ImdbConfig) error {
	if config.ImdbIDSize <= 0 {
		return errors.New("IMDB ID size must be greater than 0")
	}

	if config.LoopSize <= 0 {
		return errors.New("loop size must be greater than 0")
	}

	validTypes := []string{"movie", "tvMovie", "tvmovie", "tvSeries", "tvseries", "video"}
	for _, indexedType := range config.Indexedtypes {
		isValid := false
		for _, validType := range validTypes {
			if indexedType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid indexed type: %s", indexedType)
		}
	}

	return nil
}

// renderImdbConfig renders the IMDB configuration section
func renderImdbConfig(configv *config.ImdbConfig, csrfToken string) Node {
	group := "imdb"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("config-section"),
		H3(Text("IMDB Configuration")),

		Form(
			Class("config-form"),

			renderFormGroup(group, comments, displayNames, "Indexedtypes", "selectarray", configv.Indexedtypes, map[string][]string{
				"options": {"movie", "tvMovie", "tvmovie", "tvSeries", "tvseries", "video"},
			}),
			renderFormGroup(group, comments, displayNames, "Indexedlanguages", "array", configv.Indexedlanguages, nil),
			renderFormGroup(group, comments, displayNames, "Indexfull", "checkbox", configv.Indexfull, nil),
			renderFormGroup(group, comments, displayNames, "ImdbIDSize", "number", configv.ImdbIDSize, nil),
			renderFormGroup(group, comments, displayNames, "LoopSize", "number", configv.LoopSize, nil),
			renderFormGroup(group, comments, displayNames, "UseMemory", "checkbox", configv.UseMemory, nil),
			renderFormGroup(group, comments, displayNames, "UseCache", "checkbox", configv.UseCache, nil),

			// Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/imdb/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

// Save function (implement according to your storage method)
func saveConfig(configv any) error {
	d, err := json.Marshal(configv)
	if err != nil {
		logger.LogDynamicanyErr("info", "log struct failed", err)
		return err
	}
	logger.LogDynamicany1String("info", "log struct", "data", string(d))
	return config.UpdateCfgEntryAny(configv)
}

func renderMediaDataForm(prefix string, i int, configv *config.MediaDataConfig) Node {
	group := prefix + "_" + strconv.Itoa(int(i))
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Data"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "TemplatePath", "select", configv.TemplatePath, config.GetSettingTemplatesFor("path")),
		renderFormGroup(group, comments, displayNames, "AddFound", "checkbox", configv.AddFound, nil),
		renderFormGroup(group, comments, displayNames, "AddFoundList", "text", configv.AddFoundList, nil),
	)
}

func renderMediaDataImportForm(prefix string, i int, configv *config.MediaDataImportConfig) Node {
	group := prefix + "_" + strconv.Itoa(int(i))
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Data Import"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "TemplatePath", "select", configv.TemplatePath, config.GetSettingTemplatesFor("path")),
	)
}

func renderMediaListsForm(prefix string, i int, configv *config.MediaListsConfig) Node {
	group := prefix + "_" + strconv.Itoa(int(i))
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("List"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "TemplateList", "select", configv.TemplateList, config.GetSettingTemplatesFor("list")),
		renderFormGroup(group, comments, displayNames, "TemplateQuality", "select", configv.TemplateQuality, config.GetSettingTemplatesFor("quality")),
		renderFormGroup(group, comments, displayNames, "TemplateScheduler", "select", configv.TemplateScheduler, config.GetSettingTemplatesFor("scheduler")),
		renderFormGroup(group, comments, displayNames, "IgnoreMapLists", "array", configv.IgnoreMapLists, nil),
		renderFormGroup(group, comments, displayNames, "ReplaceMapLists", "array", configv.ReplaceMapLists, nil),
		renderFormGroup(group, comments, displayNames, "Enabled", "checkbox", configv.Enabled, nil),
		renderFormGroup(group, comments, displayNames, "AddFound", "checkbox", configv.Addfound, nil),
	)
}

func renderMediaNotificationForm(prefix string, i int, configv *config.MediaNotificationConfig) Node {
	group := prefix + "_" + strconv.Itoa(int(i))
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Notification"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "MapNotification", "select", configv.MapNotification, config.GetSettingTemplatesFor("notification")),
		renderFormGroup(group, comments, displayNames, "Event", "select", configv.Event, map[string][]string{
			"options": {"added_download", "added_data"},
		}),
		renderFormGroup(group, comments, displayNames, "Title", "text", configv.Title, nil),
		renderFormGroup(group, comments, displayNames, "Message", "text", configv.Message, nil),
		renderFormGroup(group, comments, displayNames, "ReplacedPrefix", "text", configv.ReplacedPrefix, nil),
	)
}

func renderMediaForm(typev string, configv *config.MediaTypeConfig, csrfToken string) Node {
	group := "media_" + typev + "_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	var datav []gomponents.Node
	for i, mediaType := range configv.Data {
		datav = append(datav, renderMediaDataForm(group+"_data", i, &mediaType))
	}
	var DataImport []Node
	for i, mediaType := range configv.DataImport {
		DataImport = append(DataImport, renderMediaDataImportForm(group+"_dataimport", i, &mediaType))
	}
	var Lists []Node
	for i, mediaType := range configv.Lists {
		Lists = append(Lists, renderMediaListsForm(group+"_lists", i, &mediaType))
	}
	var Notification []Node
	for i, mediaType := range configv.Notification {
		Notification = append(Notification, renderMediaNotificationForm(group+"_notification", i, &mediaType))
	}
	group = "media_main_" + typev + "_" + configv.Name
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Media"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "DefaultQuality", "select", configv.DefaultQuality, database.GetSettingTemplatesFor("quality")),
		renderFormGroup(group, comments, displayNames, "DefaultResolution", "select", configv.DefaultResolution, database.GetSettingTemplatesFor("resolution")),
		renderFormGroup(group, comments, displayNames, "Naming", "text", configv.Naming, nil),
		renderFormGroup(group, comments, displayNames, "TemplateQuality", "select", configv.TemplateQuality, config.GetSettingTemplatesFor("quality")),
		renderFormGroup(group, comments, displayNames, "TemplateScheduler", "select", configv.TemplateScheduler, config.GetSettingTemplatesFor("scheduler")),
		renderFormGroup(group, comments, displayNames, "MetadataLanguage", "text", configv.MetadataLanguage, nil),
		renderFormGroup(group, comments, displayNames, "MetadataTitleLanguages", "array", configv.MetadataTitleLanguages, nil),
		renderFormGroup(group, comments, displayNames, "Structure", "checkbox", configv.Structure, nil),
		renderFormGroup(group, comments, displayNames, "SearchmissingIncremental", "number", configv.SearchmissingIncremental, nil),
		renderFormGroup(group, comments, displayNames, "SearchupgradeIncremental", "number", configv.SearchupgradeIncremental, nil),
		Div(append([]Node{ID("mediadata" + configv.Name), Class("card border-secondary"), Style("margin: 10px; padding: 10px;"), H5(Text("Data"))}, datav...)...),
		Button(
			Class("btn btn-success"),
			Type("button"),
			hx.Target("#mediadata"+configv.Name),
			hx.Swap("beforeend"),
			hx.Post("/api/manage/mediadata/form/"+typev+"/"+configv.Name),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			Text("Add Data"),
		),
		Div(append([]Node{ID("mediadataimport" + configv.Name), Class("card border-secondary"), Style("margin: 10px; padding: 10px;"), H5(Text("Data Import"))}, DataImport...)...),
		Button(
			Class("btn btn-success"),
			Type("button"),
			hx.Target("#mediadataimport"+configv.Name),
			hx.Swap("beforeend"),
			hx.Post("/api/manage/mediaimport/form/"+typev+"/"+configv.Name),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			Text("Add Data Import"),
		),
		Div(append([]Node{ID("medialists" + configv.Name), Class("card border-secondary"), Style("margin: 10px; padding: 10px;"), H5(Text("Lists"))}, Lists...)...),
		Button(
			Class("btn btn-success"),
			Type("button"),
			hx.Target("#medialists"+configv.Name),
			hx.Swap("beforeend"),
			hx.Post("/api/manage/medialists/form/"+typev+"/"+configv.Name),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			Text("Add List"),
		),
		Div(append([]Node{ID("medianotification" + configv.Name), Class("card border-secondary"), Style("margin: 10px; padding: 10px;"), H5(Text("Notifications"))}, Notification...)...),
		Button(
			Class("btn btn-success"),
			Type("button"),
			hx.Target("#medianotification"+configv.Name),
			hx.Swap("beforeend"),
			hx.Post("/api/manage/medianotification/form/"+typev+"/"+configv.Name),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			Text("Add Notification"),
		),
	)
}

// renderMediaConfig renders the media configuration section
func renderMediaConfig(configv *config.MediaConfig, csrfToken string) Node {
	var Movies []Node
	for _, mediaType := range configv.Movies {
		Movies = append(Movies, renderMediaForm("movies", &mediaType, csrfToken))
	}
	var Series []Node
	for _, mediaType := range configv.Series {
		Series = append(Series, renderMediaForm("series", &mediaType, csrfToken))
	}
	return Div(
		Class("config-section"),
		H3(Text("Media Configuration")),

		Form(
			Class("config-form"),

			// Series section
			Div(
				Class("mb-4"),
				H4(Text("Series")),
				Div(
					ID("seriesContainer"),
					Group(Series),
					// Series items will be added here dynamically
				),
				Button(
					Class("btn btn-success"),
					Type("button"),
					hx.Target("#seriesContainer"),
					hx.Swap("beforeend"),
					hx.Post("/api/manage/media/form/series"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					Text("Add Series"),
				),
			),

			// Movies section
			Div(
				Class("mb-4"),
				H4(Text("Movies")),
				Div(
					ID("moviesContainer"),
					Group(Movies),
					// Movie items will be added here dynamically
				),
				Button(
					Class("btn btn-success"),
					Type("button"),
					hx.Target("#moviesContainer"),
					hx.Swap("beforeend"),
					hx.Post("/api/manage/media/form/movies"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					Text("Add Movie"),
				),
			),

			// Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/media/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderDownloaderForm(configv *config.DownloaderConfig) Node {
	group := "downloader_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Downloader"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "DlType", "select", configv.DlType, map[string][]string{
			"options": {"drone", "nzbget", "sabnzbd", "transmission", "rtorrent", "qbittorrent", "deluge"},
		}),
		renderFormGroup(group, comments, displayNames, "Hostname", "text", configv.Hostname, nil),
		renderFormGroup(group, comments, displayNames, "Port", "number", configv.Port, nil),
		renderFormGroup(group, comments, displayNames, "Username", "text", configv.Username, nil),
		renderFormGroup(group, comments, displayNames, "Password", "password", configv.Password, nil),
		renderFormGroup(group, comments, displayNames, "AddPaused", "checkbox", configv.AddPaused, nil),
		renderFormGroup(group, comments, displayNames, "DelugeDlTo", "text", configv.DelugeDlTo, nil),
		renderFormGroup(group, comments, displayNames, "DelugeMoveAfter", "checkbox", configv.DelugeMoveAfter, nil),
		renderFormGroup(group, comments, displayNames, "DelugeMoveTo", "text", configv.DelugeMoveTo, nil),
		renderFormGroup(group, comments, displayNames, "Priority", "number", configv.Priority, nil),
		renderFormGroup(group, comments, displayNames, "Enabled", "checkbox", configv.Enabled, nil),
	)
}

// renderDownloaderConfig renders the downloader configuration section
func renderDownloaderConfig(configv []config.DownloaderConfig, csrfToken string) Node {
	var elements []Node
	for _, dl := range configv {
		elements = append(elements, renderDownloaderForm(&dl))
	}
	return Div(
		Class("config-section"),
		H3(Text("Downloader Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("downloaderContainer"),
				Group(elements),
				// Downloader items will be added here dynamically
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#downloaderContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/downloader/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add Downloader"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/downloader/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderListsForm(configv *config.ListsConfig) Node {
	group := "lists_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("List"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "ListType", "select", configv.ListType, map[string][]string{
			"options": {"seriesconfig", "traktpublicshowlist", "imdbcsv", "imdbfile", "traktpublicmovielist", "traktmoviepopular", "traktmovieanticipated", "traktmovietrending", "traktseriepopular", "traktserieanticipated", "traktserietrending", "newznabrss"},
		}),
		renderFormGroup(group, comments, displayNames, "URL", "text", configv.URL, nil),
		renderFormGroup(group, comments, displayNames, "Enabled", "checkbox", configv.Enabled, nil),
		renderFormGroup(group, comments, displayNames, "IMDBCSVFile", "text", configv.IMDBCSVFile, nil),
		renderFormGroup(group, comments, displayNames, "SeriesConfigFile", "text", configv.SeriesConfigFile, nil),
		renderFormGroup(group, comments, displayNames, "TraktUsername", "text", configv.TraktUsername, nil),
		renderFormGroup(group, comments, displayNames, "TraktListName", "text", configv.TraktListName, nil),
		renderFormGroup(group, comments, displayNames, "TraktListType", "select", configv.TraktListType, map[string][]string{
			"options": {"movie", "show"},
		}),
		renderFormGroup(group, comments, displayNames, "Limit", "text", configv.Limit, nil),
		renderFormGroup(group, comments, displayNames, "MinVotes", "number", configv.MinVotes, nil),
		renderFormGroup(group, comments, displayNames, "MinRating", "number", configv.MinRating, nil),
		renderFormGroup(group, comments, displayNames, "Excludegenre", "array", configv.Excludegenre, nil),
		renderFormGroup(group, comments, displayNames, "Includegenre", "array", configv.Includegenre, nil),
		renderFormGroup(group, comments, displayNames, "TmdbDiscover", "array", configv.TmdbDiscover, nil),
		renderFormGroup(group, comments, displayNames, "TmdbList", "arrayint", configv.TmdbList, nil),
		renderFormGroup(group, comments, displayNames, "RemoveFromList", "checkbox", configv.RemoveFromList, nil),
	)
}

// renderListsConfig renders the lists configuration section
func renderListsConfig(configv []config.ListsConfig, csrfToken string) Node {
	var elements []Node
	for _, list := range configv {
		elements = append(elements, renderListsForm(&list))
	}
	return Div(
		Class("config-section"),
		H3(Text("Lists Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("listsContainer"),
				Group(elements),
				// Lists items will be added here dynamically
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#listsContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/lists/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add List"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/list/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderIndexersForm(configv *config.IndexersConfig) Node {
	group := "indexers_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Indexer"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "IndexerType", "select", configv.IndexerType, map[string][]string{
			"options": {"torznab", "newznab", "torrent", "torrentrss"},
		}),
		renderFormGroup(group, comments, displayNames, "URL", "text", configv.URL, nil),
		renderFormGroup(group, comments, displayNames, "Apikey", "password", configv.Apikey, nil),
		renderFormGroup(group, comments, displayNames, "Userid", "text", configv.Userid, nil),
		renderFormGroup(group, comments, displayNames, "Enabled", "checkbox", configv.Enabled, nil),
		renderFormGroup(group, comments, displayNames, "Rssenabled", "checkbox", configv.Rssenabled, nil),
		renderFormGroup(group, comments, displayNames, "Addquotesfortitlequery", "checkbox", configv.Addquotesfortitlequery, nil),
		renderFormGroup(group, comments, displayNames, "MaxEntries", "number", configv.MaxEntries, nil),
		renderFormGroup(group, comments, displayNames, "RssEntriesloop", "number", configv.RssEntriesloop, nil),
		renderFormGroup(group, comments, displayNames, "OutputAsJSON", "checkbox", configv.OutputAsJSON, nil),
		renderFormGroup(group, comments, displayNames, "Customapi", "text", configv.Customapi, nil),
		renderFormGroup(group, comments, displayNames, "Customurl", "text", configv.Customurl, nil),
		renderFormGroup(group, comments, displayNames, "Customrssurl", "text", configv.Customrssurl, nil),
		renderFormGroup(group, comments, displayNames, "Customrsscategory", "text", configv.Customrsscategory, nil),
		renderFormGroup(group, comments, displayNames, "Limitercalls", "number", configv.Limitercalls, nil),
		renderFormGroup(group, comments, displayNames, "Limiterseconds", "number", configv.Limiterseconds, nil),
		renderFormGroup(group, comments, displayNames, "LimitercallsDaily", "number", configv.LimitercallsDaily, nil),
		renderFormGroup(group, comments, displayNames, "MaxAge", "number", configv.MaxAge, nil),
		renderFormGroup(group, comments, displayNames, "DisableTLSVerify", "checkbox", configv.DisableTLSVerify, nil),
		renderFormGroup(group, comments, displayNames, "DisableCompression", "checkbox", configv.DisableCompression, nil),
		renderFormGroup(group, comments, displayNames, "TimeoutSeconds", "number", configv.TimeoutSeconds, nil),
		renderFormGroup(group, comments, displayNames, "TrustWithIMDBIDs", "checkbox", configv.TrustWithIMDBIDs, nil),
		renderFormGroup(group, comments, displayNames, "TrustWithTVDBIDs", "checkbox", configv.TrustWithTVDBIDs, nil),
		renderFormGroup(group, comments, displayNames, "CheckTitleOnIDSearch ", "checkbox", configv.CheckTitleOnIDSearch, nil),
	)
}

// renderIndexersConfig renders the indexers configuration section
func renderIndexersConfig(configv []config.IndexersConfig, csrfToken string) Node {
	var elements []Node
	for _, idx := range configv {
		elements = append(elements, renderIndexersForm(&idx))
	}
	return Div(
		Class("config-section"),
		H3(Text("Indexers Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("indexersContainer"),
				Group(elements),
				// Indexer items will be added here dynamically
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#indexersContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/indexers/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add Indexer"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/indexer/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderPathsForm(configv *config.PathsConfig) Node {
	group := "paths_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Path"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "Path", "text", configv.Path, nil),
		renderFormGroup(group, comments, displayNames, "AllowedVideoExtensions", "array", configv.AllowedVideoExtensions, nil),
		renderFormGroup(group, comments, displayNames, "AllowedOtherExtensions", "array", configv.AllowedOtherExtensions, nil),
		renderFormGroup(group, comments, displayNames, "AllowedVideoExtensionsNoRename", "array", configv.AllowedVideoExtensionsNoRename, nil),
		renderFormGroup(group, comments, displayNames, "AllowedOtherExtensionsNoRename", "array", configv.AllowedOtherExtensionsNoRename, nil),
		renderFormGroup(group, comments, displayNames, "Blocked", "array", configv.Blocked, nil),
		renderFormGroup(group, comments, displayNames, "Upgrade", "checkbox", configv.Upgrade, nil),
		renderFormGroup(group, comments, displayNames, "MinSize", "number", configv.MinSize, nil),
		renderFormGroup(group, comments, displayNames, "MaxSize", "number", configv.MaxSize, nil),
		renderFormGroup(group, comments, displayNames, "MinVideoSize", "number", configv.MinVideoSize, nil),
		renderFormGroup(group, comments, displayNames, "CleanupsizeMB", "number", configv.CleanupsizeMB, nil),
		renderFormGroup(group, comments, displayNames, "AllowedLanguages", "array", configv.AllowedLanguages, nil),
		renderFormGroup(group, comments, displayNames, "Replacelower", "checkbox", configv.Replacelower, nil),
		renderFormGroup(group, comments, displayNames, "Usepresort", "checkbox", configv.Usepresort, nil),
		renderFormGroup(group, comments, displayNames, "PresortFolderPath", "text", configv.PresortFolderPath, nil),
		renderFormGroup(group, comments, displayNames, "UpgradeScanInterval", "number", configv.UpgradeScanInterval, nil),
		renderFormGroup(group, comments, displayNames, "MissingScanInterval", "number", configv.MissingScanInterval, nil),
		renderFormGroup(group, comments, displayNames, "MissingScanReleaseDatePre", "number", configv.MissingScanReleaseDatePre, nil),
		renderFormGroup(group, comments, displayNames, "Disallowed", "array", configv.Disallowed, nil),
		renderFormGroup(group, comments, displayNames, "DeleteWrongLanguage", "checkbox", configv.DeleteWrongLanguage, nil),
		renderFormGroup(group, comments, displayNames, "DeleteDisallowed", "checkbox", configv.DeleteDisallowed, nil),
		renderFormGroup(group, comments, displayNames, "CheckRuntime", "checkbox", configv.CheckRuntime, nil),
		renderFormGroup(group, comments, displayNames, "MaxRuntimeDifference", "number", configv.MaxRuntimeDifference, nil),
		renderFormGroup(group, comments, displayNames, "DeleteWrongRuntime", "checkbox", configv.DeleteWrongRuntime, nil),
		renderFormGroup(group, comments, displayNames, "MoveReplaced", "checkbox", configv.MoveReplaced, nil),
		renderFormGroup(group, comments, displayNames, "MoveReplacedTargetPath", "text", configv.MoveReplacedTargetPath, nil),
		renderFormGroup(group, comments, displayNames, "SetChmod", "text", configv.SetChmod, nil),
		renderFormGroup(group, comments, displayNames, "SetChmodFolder", "text", configv.SetChmodFolder, nil),
	)
}

// renderPathsConfig renders the paths configuration section
func renderPathsConfig(configv []config.PathsConfig, csrfToken string) Node {
	var elements []Node
	for _, path := range configv {
		elements = append(elements, renderPathsForm(&path))
	}
	return Div(
		Class("config-section"),
		H3(Text("Paths Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("pathsContainer"),
				Group(elements),
				// Path items will be added here dynamically
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#pathsContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/paths/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add Path"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/path/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderNotificationForm(configv *config.NotificationConfig) Node {
	group := "notifications_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Notification"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "NotificationType", "select", configv.NotificationType, map[string][]string{
			"options": {"csv", "pushover"},
		}),
		renderFormGroup(group, comments, displayNames, "Apikey", "text", configv.Apikey, nil),
		renderFormGroup(group, comments, displayNames, "Recipient", "text", configv.Recipient, nil),
		renderFormGroup(group, comments, displayNames, "Outputto", "text", configv.Outputto, nil),
	)
}

// renderNotificationConfig renders the notification configuration section
func renderNotificationConfig(configv []config.NotificationConfig, csrfToken string) Node {
	var elements []Node
	for _, idx := range configv {
		elements = append(elements, renderNotificationForm(&idx))
	}
	return Div(
		Class("config-section"),
		H3(Text("Notification Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("notificationContainer"),
				Group(elements),
				// Notification items will be added here dynamically
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#notificationContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/notification/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add Notification"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/notification/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderRegexForm(configv *config.RegexConfig) Node {
	group := "regex_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Regex"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "Required", "array", configv.Required, nil),
		renderFormGroup(group, comments, displayNames, "Rejected", "array", configv.Rejected, nil),
	)
}

// renderRegexConfig renders the regex configuration section
func renderRegexConfig(configv []config.RegexConfig, csrfToken string) Node {
	var elements []Node
	for _, regex := range configv {
		elements = append(elements, renderRegexForm(&regex))
	}
	return Div(
		Class("config-section"),
		H3(Text("Regex Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("regexContainer"),
				Group(elements),
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#regexContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/regex/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add Regex"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/regex/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderQualityReorderForm(i int, mainname string, configv *config.QualityReorderConfig) Node {
	group := "quality_" + mainname + "_reorder_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Reorder"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "ReorderType", "select", configv.ReorderType, map[string][]string{
			"options": {"resolution", "quality", "codec", "audio", "position", "combined_res_qual"},
		}),
		renderFormGroup(group, comments, displayNames, "Newpriority", "number", configv.Newpriority, nil),
	)
}

func renderQualityIndexerForm(i int, mainname string, configv *config.QualityIndexerConfig) Node {
	group := "quality_" + mainname + "_indexer_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Indexer"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "TemplateIndexer", "select", configv.TemplateIndexer, config.GetSettingTemplatesFor("indexer")),
		renderFormGroup(group, comments, displayNames, "TemplateDownloader", "select", configv.TemplateDownloader, config.GetSettingTemplatesFor("downloader")),
		renderFormGroup(group, comments, displayNames, "TemplateRegex", "select", configv.TemplateRegex, config.GetSettingTemplatesFor("regex")),
		renderFormGroup(group, comments, displayNames, "TemplatePathNzb", "select", configv.TemplatePathNzb, config.GetSettingTemplatesFor("path")),
		renderFormGroup(group, comments, displayNames, "CategoryDownloader", "text", configv.CategoryDownloader, nil),
		renderFormGroup(group, comments, displayNames, "AdditionalQueryParams", "text", configv.AdditionalQueryParams, nil),
		renderFormGroup(group, comments, displayNames, "SkipEmptySize", "checkbox", configv.SkipEmptySize, nil),
		renderFormGroup(group, comments, displayNames, "HistoryCheckTitle", "checkbox", configv.HistoryCheckTitle, nil),
		renderFormGroup(group, comments, displayNames, "CategoriesIndexer", "text", configv.CategoriesIndexer, nil),
	)
}

func renderQualityForm(configv *config.QualityConfig, csrfToken string) Node {
	group := "quality_main_" + configv.Name
	var QualityReorder []Node
	for i, qualityReorder := range configv.QualityReorder {
		QualityReorder = append(QualityReorder, renderQualityReorderForm(i, configv.Name, &qualityReorder))
	}
	var QualityIndexer []Node
	for i, qualityIndexer := range configv.Indexer {
		QualityIndexer = append(QualityIndexer, renderQualityIndexerForm(i, configv.Name, &qualityIndexer))
	}
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Quality"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),
			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "WantedResolution", "arrayselectarray", configv.WantedResolution, database.GetSettingTemplatesFor("resolution")),
		renderFormGroup(group, comments, displayNames, "WantedQuality", "arrayselectarray", configv.WantedQuality, database.GetSettingTemplatesFor("quality")),
		renderFormGroup(group, comments, displayNames, "WantedAudio", "arrayselectarray", configv.WantedAudio, database.GetSettingTemplatesFor("audio")),
		renderFormGroup(group, comments, displayNames, "WantedCodec", "arrayselectarray", configv.WantedCodec, database.GetSettingTemplatesFor("codec")),
		renderFormGroup(group, comments, displayNames, "CutoffResolution", "arrayselect", configv.CutoffResolution, database.GetSettingTemplatesFor("resolution")),
		renderFormGroup(group, comments, displayNames, "CutoffQuality", "arrayselect", configv.CutoffQuality, database.GetSettingTemplatesFor("quality")),
		renderFormGroup(group, comments, displayNames, "SearchForTitleIfEmpty", "checkbox", configv.SearchForTitleIfEmpty, nil),
		renderFormGroup(group, comments, displayNames, "BackupSearchForTitle", "checkbox", configv.BackupSearchForTitle, nil),
		renderFormGroup(group, comments, displayNames, "SearchForAlternateTitleIfEmpty", "checkbox", configv.SearchForAlternateTitleIfEmpty, nil),
		renderFormGroup(group, comments, displayNames, "BackupSearchForAlternateTitle", "checkbox", configv.BackupSearchForAlternateTitle, nil),
		renderFormGroup(group, comments, displayNames, "ExcludeYearFromTitleSearch", "checkbox", configv.ExcludeYearFromTitleSearch, nil),
		renderFormGroup(group, comments, displayNames, "CheckUntilFirstFound", "checkbox", configv.CheckUntilFirstFound, nil),
		renderFormGroup(group, comments, displayNames, "CheckTitle", "checkbox", configv.CheckTitle, nil),
		renderFormGroup(group, comments, displayNames, "CheckTitleOnIDSearch", "checkbox", configv.CheckTitleOnIDSearch, nil),
		renderFormGroup(group, comments, displayNames, "CheckYear", "checkbox", configv.CheckYear, nil),
		renderFormGroup(group, comments, displayNames, "CheckYear1", "checkbox", configv.CheckYear1, nil),
		renderFormGroup(group, comments, displayNames, "TitleStripSuffixForSearch", "array", configv.TitleStripSuffixForSearch, nil),
		renderFormGroup(group, comments, displayNames, "TitleStripPrefixForSearch", "array", configv.TitleStripPrefixForSearch, nil),
		renderFormGroup(group, comments, displayNames, "UseForPriorityResolution", "checkbox", configv.UseForPriorityResolution, nil),
		renderFormGroup(group, comments, displayNames, "UseForPriorityQuality", "checkbox", configv.UseForPriorityQuality, nil),
		renderFormGroup(group, comments, displayNames, "UseForPriorityAudio", "checkbox", configv.UseForPriorityAudio, nil),
		renderFormGroup(group, comments, displayNames, "UseForPriorityCodec", "checkbox", configv.UseForPriorityCodec, nil),
		renderFormGroup(group, comments, displayNames, "UseForPriorityOther", "checkbox", configv.UseForPriorityOther, nil),
		renderFormGroup(group, comments, displayNames, "UseForPriorityMinDifference", "number", configv.UseForPriorityMinDifference, nil),
		Div(append([]Node{ID("qualityreorder" + configv.Name), Class("card border-secondary"), Style("margin: 10px; padding: 10px;"), H5(Text("Reorder"))}, QualityReorder...)...),
		Button(
			Class("btn btn-success"),
			Type("button"),
			hx.Target("#qualityreorder"+configv.Name),
			hx.Swap("beforeend"),
			hx.Post("/api/manage/qualityreorder/form/"+configv.Name),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			Text("Add Reorder"),
		),
		Div(append([]Node{ID("qualityindexer" + configv.Name), Class("card border-secondary"), Style("margin: 10px; padding: 10px;"), H5(Text("Indexer"))}, QualityIndexer...)...),
		Button(
			Class("btn btn-success"),
			Type("button"),
			hx.Target("#qualityindexer"+configv.Name),
			hx.Swap("beforeend"),
			hx.Post("/api/manage/qualityindexer/form/"+configv.Name),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			Text("Add Indexer"),
		),
	)
}

// renderQualityConfig renders the quality configuration section
func renderQualityConfig(configv []config.QualityConfig, csrfToken string) Node {
	var elements []Node
	for _, quality := range configv {
		elements = append(elements, renderQualityForm(&quality, csrfToken))
	}
	return Div(
		Class("config-section"),
		H3(Text("Quality Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("qualityContainer"),
				Group(elements),
				// Quality items will be added here dynamically
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#qualityContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/quality/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add Quality"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/quality/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

func renderSchedulerForm(configv *config.SchedulerConfig) Node {
	group := "scheduler_" + configv.Name

	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	return Div(
		Class("array-item card border-secondary"),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class("card-header"),
			Text("Sccheduler"),
		),
		Button(
			Type("button"),
			Class("btn btn-danger btn-sm"),

			Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			Text("Remove"),
		),
		renderFormGroup(group, comments, displayNames, "Name", "text", configv.Name, nil),
		renderFormGroup(group, comments, displayNames, "IntervalImdb", "text", configv.IntervalImdb, nil),
		renderFormGroup(group, comments, displayNames, "IntervalFeeds", "text", configv.IntervalFeeds, nil),
		renderFormGroup(group, comments, displayNames, "IntervalFeedsRefreshSeries", "text", configv.IntervalFeedsRefreshSeries, nil),
		renderFormGroup(group, comments, displayNames, "IntervalFeedsRefreshMovies", "text", configv.IntervalFeedsRefreshMovies, nil),
		renderFormGroup(group, comments, displayNames, "IntervalFeedsRefreshSeriesFull", "text", configv.IntervalFeedsRefreshSeriesFull, nil),
		renderFormGroup(group, comments, displayNames, "IntervalFeedsRefreshMoviesFull", "text", configv.IntervalFeedsRefreshMoviesFull, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerMissing", "text", configv.IntervalIndexerMissing, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerUpgrade", "text", configv.IntervalIndexerUpgrade, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerMissingFull", "text", configv.IntervalIndexerMissingFull, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerUpgradeFull", "text", configv.IntervalIndexerUpgradeFull, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerMissingTitle", "text", configv.IntervalIndexerMissingTitle, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerUpgradeTitle", "text", configv.IntervalIndexerUpgradeTitle, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerMissingFullTitle", "text", configv.IntervalIndexerMissingFullTitle, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerUpgradeFullTitle", "text", configv.IntervalIndexerUpgradeFullTitle, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerRss", "text", configv.IntervalIndexerRss, nil),
		renderFormGroup(group, comments, displayNames, "IntervalScanData", "text", configv.IntervalScanData, nil),
		renderFormGroup(group, comments, displayNames, "IntervalScanDataMissing", "text", configv.IntervalScanDataMissing, nil),
		renderFormGroup(group, comments, displayNames, "IntervalScanDataFlags", "text", configv.IntervalScanDataFlags, nil),
		renderFormGroup(group, comments, displayNames, "IntervalScanDataimport", "text", configv.IntervalScanDataimport, nil),
		renderFormGroup(group, comments, displayNames, "IntervalDatabaseBackup", "text", configv.IntervalDatabaseBackup, nil),
		renderFormGroup(group, comments, displayNames, "IntervalDatabaseCheck", "text", configv.IntervalDatabaseCheck, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerRssSeasons", "text", configv.IntervalIndexerRssSeasons, nil),
		renderFormGroup(group, comments, displayNames, "IntervalIndexerRssSeasonsAll", "text", configv.IntervalIndexerRssSeasonsAll, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerRssSeasonsAll", "text", configv.CronIndexerRssSeasonsAll, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerRssSeasons", "text", configv.CronIndexerRssSeasons, nil),
		renderFormGroup(group, comments, displayNames, "CronImdb", "text", configv.CronImdb, nil),
		renderFormGroup(group, comments, displayNames, "CronFeeds", "text", configv.CronFeeds, nil),
		renderFormGroup(group, comments, displayNames, "CronFeedsRefreshSeries", "text", configv.CronFeedsRefreshSeries, nil),
		renderFormGroup(group, comments, displayNames, "CronFeedsRefreshMovies", "text", configv.CronFeedsRefreshMovies, nil),
		renderFormGroup(group, comments, displayNames, "CronFeedsRefreshSeriesFull", "text", configv.CronFeedsRefreshSeriesFull, nil),
		renderFormGroup(group, comments, displayNames, "CronFeedsRefreshMoviesFull", "text", configv.CronFeedsRefreshMoviesFull, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerMissing", "text", configv.CronIndexerMissing, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerUpgrade", "text", configv.CronIndexerUpgrade, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerMissingFull", "text", configv.CronIndexerMissingFull, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerUpgradeFull", "text", configv.CronIndexerUpgradeFull, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerMissingTitle", "text", configv.CronIndexerMissingTitle, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerUpgradeTitle", "text", configv.CronIndexerUpgradeTitle, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerMissingFullTitle", "text", configv.CronIndexerMissingFullTitle, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerUpgradeFullTitle", "text", configv.CronIndexerUpgradeFullTitle, nil),
		renderFormGroup(group, comments, displayNames, "CronIndexerRss", "text", configv.CronIndexerRss, nil),
		renderFormGroup(group, comments, displayNames, "CronScanData", "text", configv.CronScanData, nil),
		renderFormGroup(group, comments, displayNames, "CronScanDataMissing", "text", configv.CronScanDataMissing, nil),
		renderFormGroup(group, comments, displayNames, "CronScanDataFlags", "text", configv.CronScanDataFlags, nil),
		renderFormGroup(group, comments, displayNames, "CronScanDataimport", "text", configv.CronScanDataimport, nil),
		renderFormGroup(group, comments, displayNames, "CronDatabaseBackup", "text", configv.CronDatabaseBackup, nil),
		renderFormGroup(group, comments, displayNames, "CronDatabaseCheck", "text", configv.CronDatabaseCheck, nil),
	)
}

// renderSchedulerConfig renders the scheduler configuration section
func renderSchedulerConfig(configv []config.SchedulerConfig, csrfToken string) Node {
	var elements []Node
	for _, config := range configv {
		elements = append(elements, renderSchedulerForm(&config))
	}
	return Div(
		Class("config-section"),
		H3(Text("Scheduler Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("schedulerContainer"),
				Group(elements),
				// Scheduler items will be added here dynamically
			),
			Button(
				Class("btn btn-success"),
				Type("button"),
				hx.Target("#schedulerContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/scheduler/form"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				Text("Add Scheduler"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/scheduler/update"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "window.location.reload()"),
					Text("Reset"),
				),
			),

			Div(ID("addalert")),
		),
	)
}

// renderFormGroup renders a form group with label and input
func renderFormGroup(group string, comments map[string]string, displayNames map[string]string, name, inputType string, value any, options map[string][]string) Node {
	var input Node

	switch inputType {
	case "selectarray":
		var optionElements []Node
		if opts, ok := options["options"]; ok {
			values := value.([]string)
			opts2 := sort.StringSlice(opts)
			opts2.Sort()
			for _, opt := range opts2 {
				selected := slices.Contains(values, opt)
				if selected {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
						Selected(),
					))
				} else {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
					))
				}
			}
		}
		input = Select(
			Class("form-select selectpicker"),
			Multiple(),
			Data("live-search", "true"),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Group(optionElements),
		)
	case "select":
		var optionElements []Node
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()
			for _, opt := range opts2 {
				selected := opt == value.(string)
				if selected {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
						Selected(),
					))
				} else {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
					))
				}
			}
		}

		input = Select(
			Class("form-select"),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Group(optionElements),
		)

	case "checkbox":
		var addnode gomponents.Node
		switch val := value.(type) {
		case bool:
			if val {
				addnode = Checked()
			}
		}
		input = Input(
			Class("form-check-input"),
			Type("checkbox"),
			Role("switch"),
			ID(group+"_"+name),
			Name(group+"_"+name), addnode,
		)

	case "textarea":
		input = Textarea(
			Class("form-control"),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Rows("3"),
			Value(value.(string)),
		)
	case "text":
		input = Input(
			Class("form-control"),
			Type(inputType),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Value(value.(string)),
		)
	case "number":
		var setvalue string
		switch val := value.(type) {
		case int:
			setvalue = fmt.Sprintf("%d", val)
		case int64:
			setvalue = fmt.Sprintf("%d", val)
		case int32:
			setvalue = fmt.Sprintf("%d", val)
		case int16:
			setvalue = fmt.Sprintf("%d", val)
		case int8:
			setvalue = fmt.Sprintf("%d", val)
		case uint:
			setvalue = fmt.Sprintf("%d", val)
		case uint64:
			setvalue = fmt.Sprintf("%d", val)
		case uint32:
			setvalue = fmt.Sprintf("%d", val)
		case uint16:
			setvalue = fmt.Sprintf("%d", val)
		case uint8:
			setvalue = fmt.Sprintf("%d", val)
		case float64:
			setvalue = fmt.Sprintf("%f", val)
		case float32:
			setvalue = fmt.Sprintf("%f", val)
		}
		input = Input(
			Class("form-control"),
			Type(inputType),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Value(setvalue),
		)

	case "array":
		input = Div(
			ID(group+"_"+name+"-container"),
			Group(
				func() []Node {
					var nodes []Node
					for _, v := range value.([]string) {
						nodes = append(nodes, Div(
							Class("d-flex mb-2"),
							Input(
								Class("form-control me-2"),
								Type("text"),
								Name(group+"_"+name),
								Value(v),
							),
							Button(
								Class("btn btn-danger"),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class("btn btn-primary"),
							Type("button"),
							Attr("onclick", fmt.Sprintf("addArray%sItem('%s', '%s')", name, group, name)),
							Text("Add Item"),
						),
						Script(Rawf(`
							function addArray%sItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<input class="form-control me-2" type="text" name="%s"><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()">Remove</button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name)),
					)
				}(),
			),
		)
	case "arrayselectarray":
		var optionString string
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()
			for _, opt := range opts2 {
				optionString += "<option value=\"" + opt + "\">" + opt + "</option>"
			}
		}

		input = Div(
			ID(group+"_"+name+"-container"),
			Group(
				func() []Node {
					var nodes []Node
					for _, v := range value.([]string) {
						var optionElements []Node
						if opts, ok := options["options"]; ok {
							opts2 := sort.StringSlice(opts)
							opts2.Sort()
							for _, opt := range opts2 {
								if opt == v {
									optionElements = append(optionElements, Option(
										Value(opt),
										Text(opt),
										Selected(),
									))
								} else {
									optionElements = append(optionElements, Option(
										Value(opt),
										Text(opt),
									))
								}
							}
						}
						nodes = append(nodes, Div(
							Class("d-flex mb-2"),
							Select(
								Class("form-select me-2"),
								Name(group+"_"+name),
								Group(optionElements),
							),
							Button(
								Class("btn btn-danger"),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class("btn btn-primary"),
							Type("button"),
							Attr("onclick", fmt.Sprintf("addArray%sItem('%s', '%s')", name, group, name)),
							Text("Add Item"),
						),
						Script(Rawf(`
							function addArray%sItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<select class="form-select me-2" name="%s">%s</select><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()">Remove</button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name, optionString)),
					)
				}(),
			),
		)
	case "arrayselect":
		var optionElements []Node
		var optionString string
		if opts, ok := options["options"]; ok {
			var values []string
			switch val := value.(type) {
			case []string:
				values = val
			}
			opts2 := sort.StringSlice(opts)
			opts2.Sort()
			for _, opt := range opts2 {
				var selected bool
				switch val := value.(type) {
				case []string:
					selected = slices.Contains(values, opt)
				case string:
					selected = opt == val
				}
				if selected {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
						Selected(),
					))
					optionString += "<option value=\"" + opt + "\" selected=\"\">" + opt + "</option>"
				} else {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
					))
					optionString += "<option value=\"" + opt + "\">" + opt + "</option>"
				}
			}
		}

		input = Div(
			ID(group+"_"+name+"-container"),
			Div(
				Class("d-flex mb-2"),
				Select(
					Class("form-select me-2"),
					Name(group+"_"+name),
					Group(optionElements),
				),
			),
		)
	case "arrayint":
		input = Div(
			ID(group+"_"+name+"-container"),
			Group(
				func() []Node {
					var nodes []Node
					for _, v := range value.([]int) {
						nodes = append(nodes, Div(
							Class("d-flex mb-2"),
							Input(
								Class("form-control me-2"),
								Type("number"),
								Name(group+"_"+name),
								Value(strconv.Itoa(v)),
							),
							Button(
								Class("btn btn-danger"),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class("btn btn-primary"),
							Type("button"),
							Attr("onclick", fmt.Sprintf("addArray%sIntItem('%s', '%s')", name, group, name)),
							Text("Add Item"),
						),
						Script(Rawf(`
							function addArray%sIntItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<input class="form-control me-2" type="number" name="%s"><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()">Remove</button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name)),
					)
				}(),
			),
		)

	default:
		input = Input(
			Class("form-control"),
			Type(inputType),
			ID(group+"_"+name),
			Name(group+"_"+name),
		)
	}

	// Enhanced form group with better comment display
	var commentNode Node
	if comment, exists := comments[name]; exists && comment != "" {
		// Split multi-line comments and create proper help text
		lines := strings.Split(comment, "\n")
		if len(lines) > 1 {
			// Multi-line comment - create a collapsible help section
			var lineNodes []Node
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					lineNodes = append(lineNodes, Div(Text(strings.TrimSpace(line))))
				}
			}
			commentNode = Div(
				Class("form-text"),
				Div(
					Class("help-text-content mt-2 p-2"),
					Style("background-color: #f8f9fa; border-left: 3px solid #0d6efd; border-radius: 4px;"),
					Group(lineNodes),
				),
			)
		} else {
			// Single line comment - display normally with better styling
			commentNode = Div(
				Class("form-text text-muted"),
				Style("font-size: 0.875em; margin-top: 0.25rem;"),
				Text(comment),
			)
		}
	}

	// Get the display name from the displayNames map, fallback to fieldNameToUserFriendly
	displayName := displayNames[name]
	if displayName == "" {
		displayName = fieldNameToUserFriendly(name)
	}

	if inputType == "checkbox" {
		return Div(
			Class("form-check form-switch mb-3"),
			input,
			Label(
				Class("form-check-label fw-semibold"),
				For(group+"_"+name),
				Text(displayName),
			),
			commentNode,
		)
	}

	return Div(
		Class("mb-3"),
		Label(
			Class("form-label fw-semibold"),
			For(group+"_"+name),
			Text(displayName),
		),
		input,
		commentNode,
	)
}

// HandleDownloaderConfigUpdate handles downloader configuration updates
func HandleDownloaderConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	// Handle downloader array updates - this is simplified
	// In practice, you'd need to handle adding/removing/updating individual downloaders
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "downloader_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[1]] = true
	}

	newConfig := make([]config.DownloaderConfig, 0, len(formKeys))
	// Process each form entry
	for i := range formKeys {
		nameField := fmt.Sprintf("downloader_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		addConfig := config.DownloaderConfig{
			Name: name,
		}

		dlTypeField := fmt.Sprintf("downloader_%s_DLType", i)
		if dlType := c.PostForm(dlTypeField); dlType != "" {
			addConfig.DlType = dlType
		}

		hostnameField := fmt.Sprintf("downloader_%s_Hostname", i)
		if hostname := c.PostForm(hostnameField); hostname != "" {
			addConfig.Hostname = hostname
		}

		portField := fmt.Sprintf("downloader_%s_Port", i)
		if port := c.PostForm(portField); port != "" {
			if portNum, err := strconv.Atoi(port); err == nil {
				addConfig.Port = portNum
			}
		}

		usernameField := fmt.Sprintf("downloader_%s_Username", i)
		if username := c.PostForm(usernameField); username != "" {
			addConfig.Username = username
		}

		passwordField := fmt.Sprintf("downloader_%s_Password", i)
		if password := c.PostForm(passwordField); password != "" {
			addConfig.Password = password
		}

		addPausedField := fmt.Sprintf("downloader_%s_AddPaused", i)
		addConfig.AddPaused = parseBool(c.PostForm(addPausedField))

		delugeDlToField := fmt.Sprintf("downloader_%s_DelugeDlTo", i)
		if delugeDlTo := c.PostForm(delugeDlToField); delugeDlTo != "" {
			addConfig.DelugeDlTo = delugeDlTo
		}

		delugeMoveAfterField := fmt.Sprintf("downloader_%s_DelugeMoveAfter", i)
		addConfig.DelugeMoveAfter = parseBool(c.PostForm(delugeMoveAfterField))

		delugeMoveToField := fmt.Sprintf("downloader_%s_DelugeMoveTo", i)
		if delugeMoveTo := c.PostForm(delugeMoveToField); delugeMoveTo != "" {
			addConfig.DelugeMoveTo = delugeMoveTo
		}

		priorityField := fmt.Sprintf("downloader_%s_Priority", i)
		if priority := c.PostForm(priorityField); priority != "" {
			if priorityNum, err := strconv.Atoi(priority); err == nil {
				addConfig.Priority = priorityNum
			}
		}

		enabledField := fmt.Sprintf("downloader_%s_Enabled", i)
		addConfig.Enabled = parseBool(c.PostForm(enabledField))

		newConfig = append(newConfig, addConfig)
	}

	if err := validateDownloaderConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Downloader configuration updated successfully", "success"))
}

// validateDownloaderConfig validates downloader configuration
func validateDownloaderConfig(configs []config.DownloaderConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("downloader name cannot be empty")
		}
		if config.Port < 0 || config.Port > 65535 {
			return fmt.Errorf("invalid port number: %d", config.Port)
		}
		validTypes := []string{"drone", "nzbget", "sabnzbd", "transmission", "rtorrent", "qbittorrent", "deluge"}
		isValidType := false
		for _, validType := range validTypes {
			if config.DlType == validType {
				isValidType = true
				break
			}
		}
		if !isValidType {
			return fmt.Errorf("invalid downloader type: %s", config.DlType)
		}
	}
	return nil
}

func parseBool(s string) bool {
	return s == "on" || s == "true" || s == "1"
}

func parseInt(s string, defaultVal int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return defaultVal
}

func parseUint16(s string, defaultVal uint16) uint16 {
	if val, err := strconv.Atoi(s); err == nil && val >= 0 {
		return uint16(val)
	}
	return defaultVal
}

func parseUint8(s string, defaultVal uint8) uint8 {
	if val, err := strconv.Atoi(s); err == nil && val >= 0 {
		return uint8(val)
	}
	return defaultVal
}

func parseStringArray(values []string) []string {
	var result []string
	for _, v := range values {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

// HandleListsConfigUpdate handles lists configuration updates
func HandleListsConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	// Handle lists array updates - simplified
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "lists_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[1]] = true
	}

	newConfig := make([]config.ListsConfig, 0, len(formKeys))
	// Process each form entry
	for i := range formKeys {
		nameField := fmt.Sprintf("lists_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		addConfig := config.ListsConfig{
			Name: name,
		}

		listTypeField := fmt.Sprintf("lists_%s_ListType", i)
		if listType := c.PostForm(listTypeField); listType != "" {
			addConfig.ListType = listType
		}

		urlField := fmt.Sprintf("lists_%s_URL", i)
		if url := c.PostForm(urlField); url != "" {
			addConfig.URL = url
		}

		enabledField := fmt.Sprintf("lists_%s_Enabled", i)
		addConfig.Enabled = parseBool(c.PostForm(enabledField))

		imdbCSVFileField := fmt.Sprintf("lists_%s_IMDBCSVFile", i)
		if imdbCSVFile := c.PostForm(imdbCSVFileField); imdbCSVFile != "" {
			addConfig.IMDBCSVFile = imdbCSVFile
		}

		seriesConfigFileField := fmt.Sprintf("lists_%s_SeriesConfigFile", i)
		if seriesConfigFile := c.PostForm(seriesConfigFileField); seriesConfigFile != "" {
			addConfig.SeriesConfigFile = seriesConfigFile
		}

		traktUsernameField := fmt.Sprintf("lists_%s_TraktUsername", i)
		if traktUsername := c.PostForm(traktUsernameField); traktUsername != "" {
			addConfig.TraktUsername = traktUsername
		}

		traktListNameField := fmt.Sprintf("lists_%s_TraktListName", i)
		if traktListName := c.PostForm(traktListNameField); traktListName != "" {
			addConfig.TraktListName = traktListName
		}

		traktListTypeField := fmt.Sprintf("lists_%s_TraktListType", i)
		if traktListType := c.PostForm(traktListTypeField); traktListType != "" {
			addConfig.TraktListType = traktListType
		}

		limitField := fmt.Sprintf("lists_%s_Limit", i)
		if limit := c.PostForm(limitField); limit != "" {
			addConfig.Limit = limit
		}

		// TODO: Add Missing List Fields

		newConfig = append(newConfig, addConfig)
	}

	if err := validateListsConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Lists configuration updated successfully", "success"))
}

// validateListsConfig validates lists configuration
func validateListsConfig(configs []config.ListsConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("list name cannot be empty")
		}
		validTypes := []string{"seriesconfig", "traktpublicshowlist", "imdbcsv", "imdbfile", "traktpublicmovielist", "traktmoviepopular", "traktmovieanticipated", "traktmovietrending", "traktseriepopular", "traktserieanticipated", "traktserietrending", "newznabrss"}
		isValidType := false
		for _, validType := range validTypes {
			if config.ListType == validType {
				isValidType = true
				break
			}
		}
		if !isValidType {
			return fmt.Errorf("invalid list type: %s", config.ListType)
		}
		if config.MinVotes < 0 {
			return errors.New("minimum votes cannot be negative")
		}
		if config.MinRating < 0 {
			return errors.New("minimum rating cannot be negative")
		}
	}
	return nil
}

// HandleIndexersConfigUpdate handles indexers configuration updates
func HandleIndexersConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	// Handle indexers array updates - simplified
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "indexers_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[1]] = true
	}

	newConfig := make([]config.IndexersConfig, 0, len(formKeys))
	// Process each form entry
	for i := range formKeys {
		nameField := fmt.Sprintf("indexers_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		addConfig := config.IndexersConfig{
			Name: name,
		}

		indexerTypeField := fmt.Sprintf("indexers_%s_IndexerType", i)
		if indexerType := c.PostForm(indexerTypeField); indexerType != "" {
			addConfig.IndexerType = indexerType
		}

		urlField := fmt.Sprintf("indexers_%s_URL", i)
		if url := c.PostForm(urlField); url != "" {
			addConfig.URL = url
		}

		apikeyField := fmt.Sprintf("indexers_%s_APIKey", i)
		if apikey := c.PostForm(apikeyField); apikey != "" {
			addConfig.Apikey = apikey
		}

		useridField := fmt.Sprintf("indexers_%s_Userid", i)
		if userid := c.PostForm(useridField); userid != "" {
			addConfig.Userid = userid
		}

		enabledField := fmt.Sprintf("indexers_%s_Enabled", i)
		addConfig.Enabled = parseBool(c.PostForm(enabledField))

		rssenabledField := fmt.Sprintf("indexers_%s_Rssenabled", i)
		addConfig.Rssenabled = parseBool(c.PostForm(rssenabledField))

		addquotesfortitlequeryField := fmt.Sprintf("indexers_%s_Addquotesfortitlequery", i)
		addConfig.Addquotesfortitlequery = parseBool(c.PostForm(addquotesfortitlequeryField))

		maxEntriesField := fmt.Sprintf("indexers_%s_Max_entries", i)
		if maxEntries := c.PostForm(maxEntriesField); maxEntries != "" {
			addConfig.MaxEntries = parseUint16(maxEntries, 100)
		}

		rssEntriesloopField := fmt.Sprintf("indexers_%s_rss_entries_loop", i)
		if rssEntriesloop := c.PostForm(rssEntriesloopField); rssEntriesloop != "" {
			addConfig.RssEntriesloop = parseUint8(rssEntriesloop, 2)
		}

		outputAsJSONField := fmt.Sprintf("indexers_%s_output_as_json", i)
		addConfig.OutputAsJSON = parseBool(c.PostForm(outputAsJSONField))

		customapiField := fmt.Sprintf("indexers_%s_customapi", i)
		if customapi := c.PostForm(customapiField); customapi != "" {
			addConfig.Customapi = customapi
		}

		customurlField := fmt.Sprintf("indexers_%s_customurl", i)
		if customurl := c.PostForm(customurlField); customurl != "" {
			addConfig.Customurl = customurl
		}

		customrssurlField := fmt.Sprintf("indexers_%s_customrssurl", i)
		if customrssurl := c.PostForm(customrssurlField); customrssurl != "" {
			addConfig.Customrssurl = customrssurl
		}

		customrsscategoryField := fmt.Sprintf("indexers_%s_customrsscategory", i)
		if customrsscategory := c.PostForm(customrsscategoryField); customrsscategory != "" {
			addConfig.Customrsscategory = customrsscategory
		}

		limitercallsField := fmt.Sprintf("indexers_%s_limitercalls", i)
		if limitercalls := c.PostForm(limitercallsField); limitercalls != "" {
			addConfig.Limitercalls = parseInt(limitercalls, 1)
		}

		limitersecondsField := fmt.Sprintf("indexers_%s_limiterseconds", i)
		if limiterseconds := c.PostForm(limitersecondsField); limiterseconds != "" {
			addConfig.Limiterseconds = parseUint8(limiterseconds, 1)
		}

		limitercallsDailyField := fmt.Sprintf("indexers_%s_LimitercallsDaily", i)
		if limitercallsDaily := c.PostForm(limitercallsDailyField); limitercallsDaily != "" {
			addConfig.LimitercallsDaily = parseInt(limitercallsDaily, 0)
		}

		maxAgeField := fmt.Sprintf("indexers_%s_max_age", i)
		if maxAge := c.PostForm(maxAgeField); maxAge != "" {
			addConfig.MaxAge = parseUint16(maxAge, 0)
		}

		disableTLSVerifyField := fmt.Sprintf("indexers_%s_DisableTLSVerify", i)
		addConfig.DisableTLSVerify = parseBool(c.PostForm(disableTLSVerifyField))

		disableCompressionField := fmt.Sprintf("indexers_%s_DisableCompression", i)
		addConfig.DisableCompression = parseBool(c.PostForm(disableCompressionField))

		newConfig = append(newConfig, addConfig)
	}

	if err := validateIndexersConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Indexers configuration updated successfully", "success"))
}

// validateIndexersConfig validates indexers configuration
func validateIndexersConfig(configs []config.IndexersConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("indexer name cannot be empty")
		}
		if config.URL == "" {
			return errors.New("indexer URL cannot be empty")
		}
		validTypes := []string{"torznab", "newznab", "torrent", "torrentrss"}
		isValidType := false
		for _, validType := range validTypes {
			if config.IndexerType == validType {
				isValidType = true
				break
			}
		}
		if !isValidType {
			return fmt.Errorf("invalid indexer type: %s", config.IndexerType)
		}
	}
	return nil
}

// HandlePathsConfigUpdate handles paths configuration updates
func HandlePathsConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	// Handle paths array updates - simplified
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "paths_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[1]] = true
	}

	newConfig := make([]config.PathsConfig, 0, len(formKeys))
	// Process each form entry
	for i := range formKeys {
		nameField := fmt.Sprintf("paths_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		addConfig := config.PathsConfig{
			Name: name,
		}

		pathField := fmt.Sprintf("paths_%s_Path", i)
		if path := c.PostForm(pathField); path != "" {
			addConfig.Path = path
		}

		allowedVideoExtensionsField := fmt.Sprintf("paths_%s_AllowedVideoExtensions", i)
		if allowedVideoExtensions := c.PostFormArray(allowedVideoExtensionsField); len(allowedVideoExtensions) > 0 {
			addConfig.AllowedVideoExtensions = parseStringArray(allowedVideoExtensions)
		}

		allowedOtherExtensionsField := fmt.Sprintf("paths_%s_AllowedOtherExtensions", i)
		if allowedOtherExtensions := c.PostFormArray(allowedOtherExtensionsField); len(allowedOtherExtensions) > 0 {
			addConfig.AllowedOtherExtensions = parseStringArray(allowedOtherExtensions)
		}

		allowedVideoExtensionsNoRenameField := fmt.Sprintf("paths_%s_AllowedVideoExtensionsNoRename", i)
		if allowedVideoExtensionsNoRename := c.PostFormArray(allowedVideoExtensionsNoRenameField); len(allowedVideoExtensionsNoRename) > 0 {
			addConfig.AllowedVideoExtensionsNoRename = parseStringArray(allowedVideoExtensionsNoRename)
		}

		allowedOtherExtensionsNoRenameField := fmt.Sprintf("paths_%s_AllowedOtherExtensionsNoRename", i)
		if allowedOtherExtensionsNoRename := c.PostFormArray(allowedOtherExtensionsNoRenameField); len(allowedOtherExtensionsNoRename) > 0 {
			addConfig.AllowedOtherExtensionsNoRename = parseStringArray(allowedOtherExtensionsNoRename)
		}

		blockedField := fmt.Sprintf("paths_%s_Blocked", i)
		if blocked := c.PostFormArray(blockedField); len(blocked) > 0 {
			addConfig.Blocked = parseStringArray(blocked)
		}

		upgradeField := fmt.Sprintf("paths_%s_Upgrade", i)
		addConfig.Upgrade = parseBool(c.PostForm(upgradeField))

		minSizeField := fmt.Sprintf("paths_%s_MinSize", i)
		if minSize := c.PostForm(minSizeField); minSize != "" {
			addConfig.MinSize = parseInt(minSize, 0)
		}

		maxSizeField := fmt.Sprintf("paths_%s_MaxSize", i)
		if maxSize := c.PostForm(maxSizeField); maxSize != "" {
			addConfig.MaxSize = parseInt(maxSize, 0)
		}

		minVideoSizeField := fmt.Sprintf("paths_%s_MinVideoSize", i)
		if minVideoSize := c.PostForm(minVideoSizeField); minVideoSize != "" {
			addConfig.MinVideoSize = parseInt(minVideoSize, 0)
		}

		cleanupsizeMBField := fmt.Sprintf("paths_%s_CleanupsizeMB", i)
		if cleanupsizeMB := c.PostForm(cleanupsizeMBField); cleanupsizeMB != "" {
			addConfig.CleanupsizeMB = parseInt(cleanupsizeMB, 0)
		}

		allowedLanguagesField := fmt.Sprintf("paths_%s_AllowedLanguages", i)
		if allowedLanguages := c.PostFormArray(allowedLanguagesField); len(allowedLanguages) > 0 {
			addConfig.AllowedLanguages = parseStringArray(allowedLanguages)
		}

		replacelowerField := fmt.Sprintf("paths_%s_Replacelower", i)
		addConfig.Replacelower = parseBool(c.PostForm(replacelowerField))

		usepresortField := fmt.Sprintf("paths_%s_Usepresort", i)
		addConfig.Usepresort = parseBool(c.PostForm(usepresortField))

		presortFolderPathField := fmt.Sprintf("paths_%s_PresortFolderPath", i)
		if presortFolderPath := c.PostForm(presortFolderPathField); presortFolderPath != "" {
			addConfig.PresortFolderPath = presortFolderPath
		}

		upgradeScanIntervalField := fmt.Sprintf("paths_%s_UpgradeScanInterval", i)
		if upgradeScanInterval := c.PostForm(upgradeScanIntervalField); upgradeScanInterval != "" {
			addConfig.UpgradeScanInterval = parseInt(upgradeScanInterval, 0)
		}

		missingScanIntervalField := fmt.Sprintf("paths_%s_MissingScanInterval", i)
		if missingScanInterval := c.PostForm(missingScanIntervalField); missingScanInterval != "" {
			addConfig.MissingScanInterval = parseInt(missingScanInterval, 0)
		}

		missingScanReleaseDatePreField := fmt.Sprintf("paths_%s_MissingScanReleaseDatePre", i)
		if missingScanReleaseDatePre := c.PostForm(missingScanReleaseDatePreField); missingScanReleaseDatePre != "" {
			addConfig.MissingScanReleaseDatePre = parseInt(missingScanReleaseDatePre, 0)
		}

		disallowedField := fmt.Sprintf("paths_%s_Disallowed", i)
		if disallowed := c.PostFormArray(disallowedField); len(disallowed) > 0 {
			addConfig.Disallowed = parseStringArray(disallowed)
		}

		newConfig = append(newConfig, addConfig)
	}
	if err := validatePathsConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Paths configuration updated successfully", "success"))
}

// validatePathsConfig validates paths configuration
func validatePathsConfig(configs []config.PathsConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("path name cannot be empty")
		}
		if config.Path == "" {
			return errors.New("path cannot be empty")
		}
		if config.MinSize < 0 {
			return errors.New("minimum size cannot be negative")
		}
		if config.MaxSize < 0 {
			return errors.New("maximum size cannot be negative")
		}
		if config.MinSize > 0 && config.MaxSize > 0 && config.MinSize > config.MaxSize {
			return errors.New("minimum size cannot be greater than maximum size")
		}
	}
	return nil
}

// HandleNotificationConfigUpdate handles notification configuration updates
func HandleNotificationConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	// Handle notification array updates - simplified
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "notifications_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[1]] = true
	}

	newConfig := make([]config.NotificationConfig, 0, len(formKeys))
	// Process each form entry
	for i := range formKeys {
		nameField := fmt.Sprintf("notifications_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		addConfig := config.NotificationConfig{
			Name: name,
		}

		notificationTypeField := fmt.Sprintf("notifications_%s_NotificationType", i)
		if notificationType := c.PostForm(notificationTypeField); notificationType != "" {
			addConfig.NotificationType = notificationType
		}

		apikeyField := fmt.Sprintf("notifications_%s_Apikey ", i)
		if apikey := c.PostForm(apikeyField); apikey != "" {
			addConfig.Apikey = apikey
		}

		recipientField := fmt.Sprintf("notifications_%s_Recipient ", i)
		if recipient := c.PostForm(recipientField); recipient != "" {
			addConfig.Recipient = recipient
		}

		outputtoField := fmt.Sprintf("notifications_%s_Outputto", i)
		if outputto := c.PostForm(outputtoField); outputto != "" {
			addConfig.Outputto = outputto
		}

		newConfig = append(newConfig, addConfig)
	}

	if err := validateNotificationConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Notification configuration updated successfully", "success"))
}

// validateNotificationConfig validates notification configuration
func validateNotificationConfig(configs []config.NotificationConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("notification name cannot be empty")
		}
		validTypes := []string{"csv", "pushover"}
		isValidType := false
		for _, validType := range validTypes {
			if config.NotificationType == validType {
				isValidType = true
				break
			}
		}
		if !isValidType {
			return fmt.Errorf("invalid notification type: %s", config.NotificationType)
		}
	}
	return nil
}

// HandleRegexConfigUpdate handles regex configuration updates
func HandleRegexConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	jsondata, _ := json.Marshal(c.Request.PostForm)
	logger.LogDynamicany1String("info", "log form post", "data", string(jsondata))

	// Parse form to find all regex entries
	formKeys := make(map[string]bool)
	formKeysAll := make(map[string]bool)
	for key := range c.Request.PostForm {
		formKeysAll[key] = true
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "regex_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[1]] = true
	}

	jsondatakeys, _ := json.Marshal(formKeys)
	logger.LogDynamicany1String("info", "log form keys", "data", string(jsondatakeys))

	jsondataallkeys, _ := json.Marshal(formKeysAll)
	logger.LogDynamicany1String("info", "log all form keys", "data", string(jsondataallkeys))
	// Build new config from form data
	newConfig := make([]config.RegexConfig, 0, len(formKeys))

	// Process each form entry
	for index := range formKeys {
		nameField := fmt.Sprintf("regex_%s_Name", index)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		regexConfig := config.RegexConfig{
			Name: name,
		}
		requiredField := fmt.Sprintf("regex_%s_Required", index)
		rejectedField := fmt.Sprintf("regex_%s_Rejected", index)

		// Parse Required array
		if required := c.PostFormArray(requiredField); len(required) > 0 {
			var filteredRequired []string
			for _, r := range required {
				if r != "" {
					filteredRequired = append(filteredRequired, r)
				}
			}
			regexConfig.Required = filteredRequired
		}

		// Parse Rejected array
		if rejected := c.PostFormArray(rejectedField); len(rejected) > 0 {
			var filteredRejected []string
			for _, r := range rejected {
				if r != "" {
					filteredRejected = append(filteredRejected, r)
				}
			}
			regexConfig.Rejected = filteredRejected
		}

		newConfig = append(newConfig, regexConfig)
	}

	updatedConfig := newConfig

	if err := validateRegexConfig(updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Regex configuration updated successfully", "success"))
}

// validateRegexConfig validates regex configuration
func validateRegexConfig(configs []config.RegexConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("regex name cannot be empty")
		}
	}
	return nil
}

// HandleQualityConfigUpdate handles quality configuration updates
func HandleQualityConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	// Parse form to find all regex entries
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "quality_main_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[2]] = true
	}

	newConfig := make([]config.QualityConfig, 0, len(formKeys))

	// Process each form entry
	for index := range formKeys {
		nameField := fmt.Sprintf("quality_main_%s_Name", index)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		qualityConfig := config.QualityConfig{
			Name: name,
		}

		// Parse array fields
		if wantedResolution := c.PostFormArray(fmt.Sprintf("quality_main_%s_WantedResolution", index)); len(wantedResolution) > 0 {
			var filteredResolution []string
			for _, r := range wantedResolution {
				if r != "" {
					filteredResolution = append(filteredResolution, r)
				}
			}
			qualityConfig.WantedResolution = filteredResolution
		}

		if wantedQuality := c.PostFormArray(fmt.Sprintf("quality_main_%s_WantedQuality", index)); len(wantedQuality) > 0 {
			var filteredQuality []string
			for _, q := range wantedQuality {
				if q != "" {
					filteredQuality = append(filteredQuality, q)
				}
			}
			qualityConfig.WantedQuality = filteredQuality
		}

		if wantedAudio := c.PostFormArray(fmt.Sprintf("quality_main_%s_WantedAudio", index)); len(wantedAudio) > 0 {
			var filteredAudio []string
			for _, a := range wantedAudio {
				if a != "" {
					filteredAudio = append(filteredAudio, a)
				}
			}
			qualityConfig.WantedAudio = filteredAudio
		}

		if wantedCodec := c.PostFormArray(fmt.Sprintf("quality_main_%s_WantedCodec", index)); len(wantedCodec) > 0 {
			var filteredCodec []string
			for _, codec := range wantedCodec {
				if codec != "" {
					filteredCodec = append(filteredCodec, codec)
				}
			}
			qualityConfig.WantedCodec = filteredCodec
		}

		if titleStripSuffix := c.PostFormArray(fmt.Sprintf("quality_main_%s_TitleStripSuffixForSearch", index)); len(titleStripSuffix) > 0 {
			var filteredSuffix []string
			for _, s := range titleStripSuffix {
				if s != "" {
					filteredSuffix = append(filteredSuffix, s)
				}
			}
			qualityConfig.TitleStripSuffixForSearch = filteredSuffix
		}

		if titleStripPrefix := c.PostFormArray(fmt.Sprintf("quality_main_%s_TitleStripPrefixForSearch", index)); len(titleStripPrefix) > 0 {
			var filteredPrefix []string
			for _, p := range titleStripPrefix {
				if p != "" {
					filteredPrefix = append(filteredPrefix, p)
				}
			}
			qualityConfig.TitleStripPrefixForSearch = filteredPrefix
		}

		// Parse string fields
		if cutoffResolution := c.PostForm(fmt.Sprintf("quality_main_%s_CutoffResolution", index)); cutoffResolution != "" {
			qualityConfig.CutoffResolution = cutoffResolution
		}

		if cutoffQuality := c.PostForm(fmt.Sprintf("quality_main_%s_CutoffQuality", index)); cutoffQuality != "" {
			qualityConfig.CutoffQuality = cutoffQuality
		}

		// Parse boolean fields
		if searchForTitleIfEmpty := c.PostForm(fmt.Sprintf("quality_main_%s_SearchForTitleIfEmpty", index)); searchForTitleIfEmpty == "on" {
			qualityConfig.SearchForTitleIfEmpty = true
		}

		if backupSearchForTitle := c.PostForm(fmt.Sprintf("quality_main_%s_BackupSearchForTitle", index)); backupSearchForTitle == "on" {
			qualityConfig.BackupSearchForTitle = true
		}

		if searchForAlternateTitleIfEmpty := c.PostForm(fmt.Sprintf("quality_main_%s_SearchForAlternateTitleIfEmpty", index)); searchForAlternateTitleIfEmpty == "on" {
			qualityConfig.SearchForAlternateTitleIfEmpty = true
		}

		if backupSearchForAlternateTitle := c.PostForm(fmt.Sprintf("quality_main_%s_BackupSearchForAlternateTitle", index)); backupSearchForAlternateTitle == "on" {
			qualityConfig.BackupSearchForAlternateTitle = true
		}

		if excludeYearFromTitleSearch := c.PostForm(fmt.Sprintf("quality_main_%s_ExcludeYearFromTitleSearch", index)); excludeYearFromTitleSearch == "on" {
			qualityConfig.ExcludeYearFromTitleSearch = true
		}

		if checkUntilFirstFound := c.PostForm(fmt.Sprintf("quality_main_%s_CheckUntilFirstFound", index)); checkUntilFirstFound == "on" {
			qualityConfig.CheckUntilFirstFound = true
		}

		if checkTitle := c.PostForm(fmt.Sprintf("quality_main_%s_CheckTitle", index)); checkTitle == "on" {
			qualityConfig.CheckTitle = true
		}

		if checkTitleOnIDSearch := c.PostForm(fmt.Sprintf("quality_main_%s_CheckTitleOnIDSearch", index)); checkTitleOnIDSearch == "on" {
			qualityConfig.CheckTitleOnIDSearch = true
		}

		if checkYear := c.PostForm(fmt.Sprintf("quality_main_%s_CheckYear", index)); checkYear == "on" {
			qualityConfig.CheckYear = true
		}

		if checkYear1 := c.PostForm(fmt.Sprintf("quality_main_%s_CheckYear1", index)); checkYear1 == "on" {
			qualityConfig.CheckYear1 = true
		}

		if useForPriorityResolution := c.PostForm(fmt.Sprintf("quality_main_%s_UseForPriorityResolution", index)); useForPriorityResolution == "on" {
			qualityConfig.UseForPriorityResolution = true
		}

		if useForPriorityQuality := c.PostForm(fmt.Sprintf("quality_main_%s_UseForPriorityQuality", index)); useForPriorityQuality == "on" {
			qualityConfig.UseForPriorityQuality = true
		}

		if useForPriorityAudio := c.PostForm(fmt.Sprintf("quality_main_%s_UseForPriorityAudio", index)); useForPriorityAudio == "on" {
			qualityConfig.UseForPriorityAudio = true
		}

		if useForPriorityCodec := c.PostForm(fmt.Sprintf("quality_main_%s_UseForPriorityCodec", index)); useForPriorityCodec == "on" {
			qualityConfig.UseForPriorityCodec = true
		}

		if useForPriorityOther := c.PostForm(fmt.Sprintf("quality_main_%s_UseForPriorityOther", index)); useForPriorityOther == "on" {
			qualityConfig.UseForPriorityOther = true
		}

		// Parse integer field
		if useForPriorityMinDifference := c.PostForm(fmt.Sprintf("quality_main_%s_UseForPriorityMinDifference", index)); useForPriorityMinDifference != "" {
			if minDiff, err := strconv.Atoi(useForPriorityMinDifference); err == nil {
				qualityConfig.UseForPriorityMinDifference = minDiff
			}
		}

		// Parse QualityReorder slice
		subformKeys := make(map[string]bool)
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "quality_")) == false || (strings.Contains(key, "_reorder_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[3]] = true
		}

		var qualityReorderConfigs []config.QualityReorderConfig
		// Process each form entry
		for reorderIndex := range subformKeys {
			nameField := fmt.Sprintf("quality_%s_reorder_%s_Name", index, reorderIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			addConfig := config.QualityReorderConfig{
				Name: name,
			}

			if reorderType := c.PostForm(fmt.Sprintf("quality_%s_reorder_%s_ReorderType", index, reorderIndex)); reorderType != "" {
				addConfig.ReorderType = reorderType
			}

			if newPriority := c.PostForm(fmt.Sprintf("quality_%s_reorder_%s_Newpriority", index, reorderIndex)); newPriority != "" {
				if priority, err := strconv.Atoi(newPriority); err == nil {
					addConfig.Newpriority = priority
				}
			}

			qualityReorderConfigs = append(qualityReorderConfigs, addConfig)
		}
		qualityConfig.QualityReorder = qualityReorderConfigs

		// Parse Indexer slice
		subformKeys = make(map[string]bool)
		for key := range c.Request.PostForm {
			if (strings.Contains(key, "_TemplateIndexer")) == false || (strings.Contains(key, "quality_")) == false || (strings.Contains(key, "_indexer_")) == false {
				continue
			}
			subformKeys[strings.Split(key, "_")[3]] = true
		}
		var qualityIndexerConfigs []config.QualityIndexerConfig
		for indexerIndex := range subformKeys {
			nameField := fmt.Sprintf("quality_%s_indexer_%s_TemplateIndexer", index, indexerIndex)
			name := c.PostForm(nameField)
			if name == "" {
				continue // Skip entries without names
			}

			indexerConfig := config.QualityIndexerConfig{
				TemplateIndexer: name,
			}

			if templateDownloader := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplateDownloader", index, indexerIndex)); templateDownloader != "" {
				indexerConfig.TemplateDownloader = templateDownloader
			}

			if templateRegex := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplateRegex", index, indexerIndex)); templateRegex != "" {
				indexerConfig.TemplateRegex = templateRegex
			}

			if templatePathNzb := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplatePathNzb", index, indexerIndex)); templatePathNzb != "" {
				indexerConfig.TemplatePathNzb = templatePathNzb
			}

			if categoryDownloader := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_CategoryDownloader", index, indexerIndex)); categoryDownloader != "" {
				indexerConfig.CategoryDownloader = categoryDownloader
			}

			if additionalQueryParams := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_AdditionalQueryParams", index, indexerIndex)); additionalQueryParams != "" {
				indexerConfig.AdditionalQueryParams = additionalQueryParams
			}

			if skipEmptySize := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_SkipEmptySize", index, indexerIndex)); skipEmptySize == "on" {
				indexerConfig.SkipEmptySize = true
			}

			if historyCheckTitle := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_HistoryCheckTitle", index, indexerIndex)); historyCheckTitle == "on" {
				indexerConfig.HistoryCheckTitle = true
			}

			if categoriesIndexer := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_CategoriesIndexer", index, indexerIndex)); categoriesIndexer != "" {
				indexerConfig.CategoriesIndexer = categoriesIndexer
			}

			qualityIndexerConfigs = append(qualityIndexerConfigs, indexerConfig)
		}
		qualityConfig.Indexer = qualityIndexerConfigs

		newConfig = append(newConfig, qualityConfig)
	}

	if err := validateQualityConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Quality configuration updated successfully", "success"))
}

// validateQualityConfig validates quality configuration
func validateQualityConfig(configs []config.QualityConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("quality name cannot be empty")
		}
		if config.UseForPriorityMinDifference < 0 {
			return errors.New("priority minimum difference cannot be negative")
		}
	}
	return nil
}

// HandleSchedulerConfigUpdate handles scheduler configuration updates
func HandleSchedulerConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "scheduler_")) == false {
			continue
		}
		formKeys[strings.Split(key, "_")[1]] = true
	}

	newConfig := make([]config.SchedulerConfig, 0, len(formKeys))
	// Handle scheduler array updates - simplified
	for i := range formKeys {
		nameField := fmt.Sprintf("scheduler_%s_Name", i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		updatedConfig := config.SchedulerConfig{
			Name: name,
		}

		// Update interval fields
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalImdb", i)); val != "" {
			updatedConfig.IntervalImdb = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalFeeds", i)); val != "" {
			updatedConfig.IntervalFeeds = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalFeedsRefreshSeries", i)); val != "" {
			updatedConfig.IntervalFeedsRefreshSeries = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalFeedsRefreshMovies", i)); val != "" {
			updatedConfig.IntervalFeedsRefreshMovies = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalFeedsRefreshSeriesFull", i)); val != "" {
			updatedConfig.IntervalFeedsRefreshSeriesFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalFeedsRefreshMoviesFull", i)); val != "" {
			updatedConfig.IntervalFeedsRefreshMoviesFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerMissing", i)); val != "" {
			updatedConfig.IntervalIndexerMissing = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerUpgrade", i)); val != "" {
			updatedConfig.IntervalIndexerUpgrade = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerMissingFull", i)); val != "" {
			updatedConfig.IntervalIndexerMissingFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerUpgradeFull", i)); val != "" {
			updatedConfig.IntervalIndexerUpgradeFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerMissingTitle", i)); val != "" {
			updatedConfig.IntervalIndexerMissingTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerUpgradeTitle", i)); val != "" {
			updatedConfig.IntervalIndexerUpgradeTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerMissingFullTitle", i)); val != "" {
			updatedConfig.IntervalIndexerMissingFullTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerUpgradeFullTitle", i)); val != "" {
			updatedConfig.IntervalIndexerUpgradeFullTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerRss", i)); val != "" {
			updatedConfig.IntervalIndexerRss = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalScanData", i)); val != "" {
			updatedConfig.IntervalScanData = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalScanDataMissing", i)); val != "" {
			updatedConfig.IntervalScanDataMissing = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalScanDataFlags", i)); val != "" {
			updatedConfig.IntervalScanDataFlags = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalScanDataimport", i)); val != "" {
			updatedConfig.IntervalScanDataimport = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalDatabaseBackup", i)); val != "" {
			updatedConfig.IntervalDatabaseBackup = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalDatabaseCheck", i)); val != "" {
			updatedConfig.IntervalDatabaseCheck = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerRssSeasons", i)); val != "" {
			updatedConfig.IntervalIndexerRssSeasons = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_IntervalIndexerRssSeasonsAll", i)); val != "" {
			updatedConfig.IntervalIndexerRssSeasonsAll = val
		}

		// Update cron fields
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerRssSeasonsAll", i)); val != "" {
			updatedConfig.CronIndexerRssSeasonsAll = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerRssSeasons", i)); val != "" {
			updatedConfig.CronIndexerRssSeasons = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronImdb", i)); val != "" {
			updatedConfig.CronImdb = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronFeeds", i)); val != "" {
			updatedConfig.CronFeeds = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronFeedsRefreshSeries", i)); val != "" {
			updatedConfig.CronFeedsRefreshSeries = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronFeedsRefreshMovies", i)); val != "" {
			updatedConfig.CronFeedsRefreshMovies = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronFeedsRefreshSeriesFull", i)); val != "" {
			updatedConfig.CronFeedsRefreshSeriesFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronFeedsRefreshMoviesFull", i)); val != "" {
			updatedConfig.CronFeedsRefreshMoviesFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerMissing", i)); val != "" {
			updatedConfig.CronIndexerMissing = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerUpgrade", i)); val != "" {
			updatedConfig.CronIndexerUpgrade = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerMissingFull", i)); val != "" {
			updatedConfig.CronIndexerMissingFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerUpgradeFull", i)); val != "" {
			updatedConfig.CronIndexerUpgradeFull = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerMissingTitle", i)); val != "" {
			updatedConfig.CronIndexerMissingTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerUpgradeTitle", i)); val != "" {
			updatedConfig.CronIndexerUpgradeTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerMissingFullTitle", i)); val != "" {
			updatedConfig.CronIndexerMissingFullTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerUpgradeFullTitle", i)); val != "" {
			updatedConfig.CronIndexerUpgradeFullTitle = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronIndexerRss", i)); val != "" {
			updatedConfig.CronIndexerRss = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronScanData", i)); val != "" {
			updatedConfig.CronScanData = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronScanDataMissing", i)); val != "" {
			updatedConfig.CronScanDataMissing = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronScanDataFlags", i)); val != "" {
			updatedConfig.CronScanDataFlags = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronScanDataimport", i)); val != "" {
			updatedConfig.CronScanDataimport = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronDatabaseBackup", i)); val != "" {
			updatedConfig.CronDatabaseBackup = val
		}
		if val := c.PostForm(fmt.Sprintf("scheduler_%s_CronDatabaseCheck", i)); val != "" {
			updatedConfig.CronDatabaseCheck = val
		}
		newConfig = append(newConfig, updatedConfig)
	}

	if err := validateSchedulerConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	if err := saveConfig(newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Scheduler configuration updated successfully", "success"))
}

// validateSchedulerConfig validates scheduler configuration
func validateSchedulerConfig(configs []config.SchedulerConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("scheduler name cannot be empty")
		}
	}
	return nil
}

// HandleConfigUpdate - consolidated handler for all config update routes
func HandleConfigUpdate(c *gin.Context) {
	configType := c.Param("configtype")
	
	switch configType {
	case "general":
		HandleGeneralConfigUpdate(c)
	case "imdb":
		HandleImdbConfigUpdate(c)
	case "quality":
		HandleQualityConfigUpdate(c)
	case "downloader":
		HandleDownloaderConfigUpdate(c)
	case "indexer":
		HandleIndexersConfigUpdate(c)
	case "list":
		HandleListsConfigUpdate(c)
	case "media":
		HandleMediaConfigUpdate(c)
	case "path":
		HandlePathsConfigUpdate(c)
	case "notification":
		HandleNotificationConfigUpdate(c)
	case "regex":
		HandleRegexConfigUpdate(c)
	case "scheduler":
		HandleSchedulerConfigUpdate(c)
	default:
		c.String(http.StatusNotFound, renderAlert("Configuration type not found", "danger"))
	}
}
