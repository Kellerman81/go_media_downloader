package api

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	. "maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	. "maragu.dev/gomponents/html"
)

// renderEnhancedPageHeader creates a standardized enhanced page header with gradient background
func renderEnhancedPageHeader(iconClass, title, subtitle string) Node {
	return Div(
		Class("page-header-enhanced"),
		Div(
			Class("header-content"),
			Div(
				Class("header-icon-wrapper"),
				I(Class(iconClass+" header-icon")),
			),
			Div(
				Class("header-text"),
				H2(Class("header-title"), Text(title)),
				P(Class("header-subtitle"), Text(subtitle)),
			),
		),
	)
}

// renderHTMXSubmitButton creates a standardized HTMX form submit button
func renderHTMXSubmitButton(buttonText, targetID, endpoint, formID, csrfToken string) Node {
	return Div(
		Class("form-group submit-group"),
		Button(
			Class("btn btn-primary"),
			Text(buttonText),
			Type("button"),
			hx.Target("#"+targetID),
			hx.Swap("innerHTML"),
			hx.Post(endpoint),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			hx.Include("#"+formID),
		),
	)
}

// renderGenericConfigSection creates a standardized config section
func renderGenericConfigSection[T any](
	configs []T,
	csrfToken string,
	options RenderConfigOptions,
	renderItemFunc func(T, string) Node,
) Node {
	items := getNodeSlice()
	defer putNodeSlice(items)

	for _, config := range configs {
		items = append(items, renderItemFunc(config, csrfToken))
	}

	containerContent := getNodeSlice()
	defer putNodeSlice(containerContent)

	// Add existing items
	containerContent = append(containerContent, items...)

	// Create the complete container
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fas fa-"+options.Icon+" header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text(options.Title)),
					P(Class("header-subtitle"), Text(options.Subtitle)),
				),
			),
		),

		Form(
			Class("config-form"),
			Div(append([]Node{ID(options.FormContainer)}, containerContent...)...),
			createAddButton(options.AddButtonText, "#"+options.FormContainer, options.AddEndpoint, csrfToken),

			createFormSubmitGroup("Save Configuration", "#addalert", options.SubmitEndpoint, csrfToken),
			Div(ID("addalert")),
		),
	)
}

// Common render patterns using the optimized builder
func renderStandardArrayForm[T any](
	prefix string,
	i int,
	title string,
	config T,
	buildFields func(*OptimizedFieldBuilder, T) *OptimizedFieldBuilder,
) Node {
	builder := NewOptimizedFieldBuilder(10) // Pre-allocate for common case
	fields := buildFields(builder, config).Build()
	return renderArrayItemFormWithIndex(prefix, i, title, config, fields)
}

// renderConfigSection creates a generic config section with form elements
func renderConfigSection[T any](configList []T, csrfToken string, options ConfigSectionOptions, renderForm func(*T) Node) Node {
	var elements []Node
	for _, config := range configList {
		elements = append(elements, renderForm(&config))
	}

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fas fa-"+options.SectionIcon+" header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text(options.SectionTitle)),
					P(Class("header-subtitle"), Text(options.SectionSubtitle)),
				),
			),
		),

		Form(
			Class("config-form"),
			Div(
				ID(options.ContainerID),
				Group(elements),
			),
			createAddButton(options.AddButtonText, "#"+options.ContainerID, options.AddFormPath, csrfToken),
			Group(createConfigFormButtons("Save Configuration", "#addalert", options.UpdatePath, csrfToken)),
			Script(
				Raw(`
					document.addEventListener('DOMContentLoaded', function() {
						// Update collapse indicators when cards are toggled
						document.querySelectorAll('[data-bs-toggle="collapse"]').forEach(function(trigger) {
							var targetId = trigger.getAttribute('data-bs-target');
							var indicator = trigger.querySelector('.badge');
							var targetEl = document.querySelector(targetId);
							
							if (targetEl && indicator) {
								targetEl.addEventListener('show.bs.collapse', function() {
									indicator.textContent = '▼';
								});
								targetEl.addEventListener('hide.bs.collapse', function() {
									indicator.textContent = '▶';
								});
							}
						});
					});
				`),
			),
		),
	)
}

func renderArrayItemFormWithIndex(prefix string, i int, headerText string, config any, fields []FormFieldDefinition) Node {
	group := prefix + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text(headerText),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				// createRemoveButton(true),
				Group(formGroups),
			),
		),
	)
}

func renderArrayItemFormWithNameAndIndex(namePrefix string, mainname string, i int, headerText string, config any, fields []FormFieldDefinition) Node {
	group := namePrefix + "_" + mainname + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text(headerText+" "+mainname),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				// createRemoveButton(true),
				Group(formGroups),
			),
		),
	)
}

// renderGeneralConfig renders the general configuration section
func renderGeneralConfig(configv *config.GeneralConfig, csrfToken string) Node {
	group := "general"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-sliders",
			"General Configuration",
			"Configure general application settings including logging, timeouts, paths, and basic operational parameters.",
		),
		Form(
			Class("config-form"),

			// Configuration sections organized by category
			renderGeneralConfigSections(configv, group, comments, displayNames),

			createFormSubmitGroup("Save Configuration", "#addalert", "/api/admin/general/update", csrfToken),
			Div(ID("addalert")),
		))
}

// renderGeneralConfigSections organizes general config fields into logical collapsible groups
func renderGeneralConfigSections(configv *config.GeneralConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	return Div(
		Class("accordion"),
		ID("generalConfigAccordion"),

		// Basic Settings Section
		renderConfigGroup("Basic Settings", "basic", true,
			[]FormFieldDefinition{
				{Name: "TimeFormat", Type: "select", Value: configv.TimeFormat, Options: map[string][]string{"options": {"rfc3339", "iso8601", "rfc1123", "rfc822", "rfc850"}}},
				{Name: "TimeZone", Type: "text", Value: configv.TimeZone},
				{Name: "WebPort", Type: "text", Value: configv.WebPort},
				{Name: "WebAPIKey", Type: "text", Value: configv.WebAPIKey},
				{Name: "WebPortalEnabled", Type: "checkbox", Value: configv.WebPortalEnabled},
			}, group, comments, displayNames),

		// Logging Settings Section
		renderConfigGroup("Logging Settings", "logging", false,
			[]FormFieldDefinition{
				{Name: "LogLevel", Type: "select", Value: configv.LogLevel, Options: map[string][]string{"options": {"info", "debug"}}},
				{Name: "DBLogLevel", Type: "select", Value: configv.DBLogLevel, Options: map[string][]string{"options": {"info", "debug"}}},
				{Name: "LogFileSize", Type: "number", Value: configv.LogFileSize},
				{Name: "LogFileCount", Type: "number", Value: configv.LogFileCount},
				{Name: "LogCompress", Type: "checkbox", Value: configv.LogCompress},
				{Name: "LogToFileOnly", Type: "checkbox", Value: configv.LogToFileOnly},
				{Name: "LogColorize", Type: "checkbox", Value: configv.LogColorize},
				{Name: "LogZeroValues", Type: "checkbox", Value: configv.LogZeroValues},
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
				{Name: "SearcherSize", Type: "number", Value: configv.SearcherSize},
			}, group, comments, displayNames),

		// API Integration Section
		renderConfigGroup("API Integration", "apis", false,
			[]FormFieldDefinition{
				{Name: "OmdbAPIKey", Type: "text", Value: configv.OmdbAPIKey},
				{Name: "TheMovieDBApiKey", Type: "text", Value: configv.TheMovieDBApiKey},
				{Name: "TraktClientID", Type: "text", Value: configv.TraktClientID},
				{Name: "TraktClientSecret", Type: "text", Value: configv.TraktClientSecret},
				{Name: "TraktRedirectUrl", Type: "text", Value: configv.TraktRedirectUrl},
			}, group, comments, displayNames),

		// Metadata Sources Section
		renderConfigGroup("Metadata Sources", "metadata", false,
			[]FormFieldDefinition{
				{Name: "MovieMetaSourceImdb", Type: "checkbox", Value: configv.MovieMetaSourceImdb},
				{Name: "MovieMetaSourceTmdb", Type: "checkbox", Value: configv.MovieMetaSourceTmdb},
				{Name: "MovieMetaSourceOmdb", Type: "checkbox", Value: configv.MovieMetaSourceOmdb},
				{Name: "MovieMetaSourceTrakt", Type: "checkbox", Value: configv.MovieMetaSourceTrakt},
				{Name: "MovieAlternateTitleMetaSourceImdb", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceImdb},
				{Name: "MovieAlternateTitleMetaSourceTmdb", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceTmdb},
				{Name: "MovieAlternateTitleMetaSourceOmdb", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceOmdb},
				{Name: "MovieAlternateTitleMetaSourceTrakt", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceTrakt},
				{Name: "SerieAlternateTitleMetaSourceImdb", Type: "checkbox", Value: configv.SerieAlternateTitleMetaSourceImdb},
				{Name: "SerieAlternateTitleMetaSourceTrakt", Type: "checkbox", Value: configv.SerieAlternateTitleMetaSourceTrakt},
				{Name: "SerieMetaSourceTmdb", Type: "checkbox", Value: configv.SerieMetaSourceTmdb},
				{Name: "SerieMetaSourceTrakt", Type: "checkbox", Value: configv.SerieMetaSourceTrakt},
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
			}, group, comments, displayNames),

		// Advanced Settings Section
		renderConfigGroup("Advanced Settings", "advanced", false,
			[]FormFieldDefinition{
				{Name: "MovieMetaSourcePriority", Type: "array", Value: configv.MovieMetaSourcePriority},
				{Name: "MovieRSSMetaSourcePriority", Type: "array", Value: configv.MovieRSSMetaSourcePriority},
				{Name: "MovieParseMetaSourcePriority", Type: "array", Value: configv.MovieParseMetaSourcePriority},
				{Name: "MoveBufferSizeKB", Type: "number", Value: configv.MoveBufferSizeKB},
				{Name: "SchedulerDisabled", Type: "checkbox", Value: configv.SchedulerDisabled},
				{Name: "DisableParserStringMatch", Type: "checkbox", Value: configv.DisableParserStringMatch},
				{Name: "UseCronInsteadOfInterval", Type: "checkbox", Value: configv.UseCronInsteadOfInterval},
				{Name: "UseFileBufferCopy", Type: "checkbox", Value: configv.UseFileBufferCopy},
				{Name: "DisableSwagger", Type: "checkbox", Value: configv.DisableSwagger},
				{Name: "TheMovieDBDisableTLSVerify", Type: "checkbox", Value: configv.TheMovieDBDisableTLSVerify},
				{Name: "TraktDisableTLSVerify", Type: "checkbox", Value: configv.TraktDisableTLSVerify},
				{Name: "OmdbDisableTLSVerify", Type: "checkbox", Value: configv.OmdbDisableTLSVerify},
				{Name: "TvdbDisableTLSVerify", Type: "checkbox", Value: configv.TvdbDisableTLSVerify},
				{Name: "FfprobePath", Type: "text", Value: configv.FfprobePath},
				{Name: "MediainfoPath", Type: "text", Value: configv.MediainfoPath},
				{Name: "UseMediainfo", Type: "checkbox", Value: configv.UseMediainfo},
				{Name: "UseMediaFallback", Type: "checkbox", Value: configv.UseMediaFallback},
				{Name: "FailedIndexerBlockTime", Type: "number", Value: configv.FailedIndexerBlockTime},
				{Name: "MaxDatabaseBackups", Type: "number", Value: configv.MaxDatabaseBackups},
				{Name: "DatabaseBackupStopTasks", Type: "checkbox", Value: configv.DatabaseBackupStopTasks},
				{Name: "DisableVariableCleanup", Type: "checkbox", Value: configv.DisableVariableCleanup},
				{Name: "OmdbTimeoutSeconds", Type: "number", Value: configv.OmdbTimeoutSeconds},
				{Name: "TmdbTimeoutSeconds", Type: "number", Value: configv.TmdbTimeoutSeconds},
				{Name: "TvdbTimeoutSeconds", Type: "number", Value: configv.TvdbTimeoutSeconds},
				{Name: "TraktTimeoutSeconds", Type: "number", Value: configv.TraktTimeoutSeconds},
				{Name: "EnableFileWatcher", Type: "checkbox", Value: configv.EnableFileWatcher},
			}, group, comments, displayNames),
	)
}

// renderConfigGroup creates a collapsible group of configuration fields
func renderConfigGroup(title, id string, expanded bool, fields []FormFieldDefinition, group string, comments map[string]string, displayNames map[string]string) Node {
	return renderConfigGroupWithParent(title, id, expanded, fields, group, comments, displayNames, "generalConfigAccordion")
}

// renderConfigGroupWithParent creates a collapsible group with specified parent accordion
func renderConfigGroupWithParent(title, id string, expanded bool, fields []FormFieldDefinition, group string, comments map[string]string, displayNames map[string]string, parentAccordion string) Node {
	collapseClass := "accordion-collapse collapse"
	if expanded {
		collapseClass += " show"
	}

	return Div(
		Class("accordion-item"),
		Style("border: 1px solid #dee2e6; border-radius: 8px; margin-bottom: 0.5rem;"),
		H2(
			Class("accordion-header"),
			ID("heading"+id),
			Button(
				Class("accordion-button"),
				If(!expanded, Attr("class", "accordion-button collapsed")),
				Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: none; padding: 0.75rem 1rem; font-weight: 600;"),
				Type("button"),
				Attr("data-bs-toggle", "collapse"),
				Attr("data-bs-target", "#collapse"+id),
				Attr("aria-expanded", fmt.Sprintf("%t", expanded)),
				Attr("aria-controls", "collapse"+id),
				I(Class("fas fa-cog me-2 text-primary")),
				Text(title),
				Span(Class("badge bg-primary ms-2"), Text(fmt.Sprintf("%d", len(fields)))),
			),
		),
		Div(
			ID("collapse"+id),
			Class(collapseClass),
			Attr("aria-labelledby", "heading"+id),
			Attr("data-bs-parent", "#"+parentAccordion),
			Div(
				Class("accordion-body p-3"),
				Style("background-color: #fdfdfe;"),
				// Use compact grid layout for fields
				renderCompactFormFields(group, comments, displayNames, fields),
			),
		),
	)
}

// renderCompactFormFields creates a more compact grid layout for configuration fields
func renderCompactFormFields(group string, comments map[string]string, displayNames map[string]string, fields []FormFieldDefinition) Node {
	var formRows []Node

	// Group fields into rows of 2 for better space utilization
	for i := 0; i < len(fields); i += 2 {
		var rowFields []Node

		// First field in row
		if i < len(fields) {
			field := fields[i]
			rowFields = append(rowFields, Div(
				Class("col-md-6 mb-3"),
				renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options),
			))
		}

		// Second field in row (if exists)
		if i+1 < len(fields) {
			field := fields[i+1]
			rowFields = append(rowFields, Div(
				Class("col-md-6 mb-3"),
				renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options),
			))
		}

		// Create row
		formRows = append(formRows, Div(
			Class("row"),
			Group(rowFields),
		))
	}

	return Group(formRows)
}

// renderAlert creates a dismissible alert with icons using optimized createAlert
func renderAlert(message string, typev string) string {
	return renderComponentToString(createAlert(message, typev))
}

// renderImdbConfig renders the IMDB configuration section
func renderImdbConfig(configv *config.ImdbConfig, csrfToken string) Node {
	group := "imdb"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	fields := createImdbConfigFields(configv)
	formGroups := renderFormFields(group, comments, displayNames, fields)

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-film",
			"IMDB Configuration",
			"Configure IMDB database settings including titles, ratings, episodes, and cast information.",
		),

		Form(
			Class("config-form"),
			Group(formGroups),

			createFormSubmitGroup("Save Configuration", "#addalert", "/api/admin/imdb/update", csrfToken),
			Div(ID("addalert")),
		),
	)
}

func renderMediaDataForm(prefix string, i int, configv *config.MediaDataConfig) Node {
	fields := []FormFieldDefinition{
		{"", "removebutton", "", nil},
		{"TemplatePath", "select", configv.TemplatePath, config.GetSettingTemplatesFor("path")},
		{"AddFound", "checkbox", configv.AddFound, nil},
		{"AddFoundList", "text", configv.AddFoundList, nil},
	}
	return renderArrayItemFormWithIndex(prefix, i, "Data", configv, fields)
}

func renderMediaDataImportForm(prefix string, i int, configv *config.MediaDataImportConfig) Node {
	fields := []FormFieldDefinition{
		{"", "removebutton", "", nil},
		{"TemplatePath", "select", configv.TemplatePath, config.GetSettingTemplatesFor("path")},
	}
	return renderArrayItemFormWithIndex(prefix, i, "Data Import", configv, fields)
}

func renderMediaListsForm(prefix string, i int, configv *config.MediaListsConfig) Node {
	return renderStandardArrayForm(prefix, i, "List", configv, func(b *OptimizedFieldBuilder, config *config.MediaListsConfig) *OptimizedFieldBuilder {
		return b.
			AddText("Name", config.Name).
			AddSelectCached("TemplateList", config.TemplateList, "list").
			AddSelectCached("TemplateQuality", config.TemplateQuality, "quality").
			AddSelectCached("TemplateScheduler", config.TemplateScheduler, "scheduler").
			AddArray("IgnoreMapLists", config.IgnoreMapLists).
			AddArray("ReplaceMapLists", config.ReplaceMapLists).
			AddCheckbox("Enabled", config.Enabled).
			AddCheckbox("AddFound", config.Addfound)
	})
}

func renderMediaNotificationForm(prefix string, i int, configv *config.MediaNotificationConfig) Node {
	return renderStandardArrayForm(prefix, i, "Notification", configv, func(b *OptimizedFieldBuilder, config *config.MediaNotificationConfig) *OptimizedFieldBuilder {
		return b.
			AddSelectCached("MapNotification", config.MapNotification, "notification").
			AddSelect("Event", config.Event, []string{"added_download", "added_data"}).
			AddText("Title", config.Title).
			AddText("Message", config.Message).
			AddText("ReplacedPrefix", config.ReplacedPrefix)
	})
}

func renderMediaForm(typev string, configv *config.MediaTypeConfig, csrfToken string) Node {
	group := "media_" + typev + "_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	collapseId := group + "_main_collapse"

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px;"),
		Button(
			Class("card-header accordion-button w-100 border-0"),
			Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;"),
			Type("button"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			I(Class("fas fa-film me-2 text-primary")),
			Text("Media "+configv.Name),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				// Organized media sections
				renderMediaConfigSections(configv, typev, group, comments, displayNames, csrfToken),
			),
		),
	)
}

// renderMediaConfigSections organizes media fields into logical groups
func renderMediaConfigSections(configv *config.MediaTypeConfig, typev, group string, comments map[string]string, displayNames map[string]string, csrfToken string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "mediaConfigAccordion-" + typev + "-" + sanitizedName

	// Prepare sub-arrays
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

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-"+typev+"-"+configv.Name, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
				{"Naming", "text", configv.Naming, nil},
				{"MetadataLanguage", "text", configv.MetadataLanguage, nil},
				{"Structure", "checkbox", configv.Structure, nil},
			}, "media_main_"+typev+"_"+configv.Name, comments, displayNames, accordionId),

		// Quality & Templates
		renderConfigGroupWithParent("Quality & Templates", "quality-"+typev+"-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"DefaultQuality", "select", configv.DefaultQuality, database.GetSettingTemplatesFor("quality")},
				{"DefaultResolution", "select", configv.DefaultResolution, database.GetSettingTemplatesFor("resolution")},
				{"TemplateQuality", "select", configv.TemplateQuality, config.GetSettingTemplatesFor("quality")},
				{"TemplateScheduler", "select", configv.TemplateScheduler, config.GetSettingTemplatesFor("scheduler")},
			}, "media_main_"+typev+"_"+configv.Name, comments, displayNames, accordionId),

		// Metadata & Search Settings
		renderConfigGroupWithParent("Metadata & Search", "metadata-"+typev+"-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"MetadataTitleLanguages", "array", configv.MetadataTitleLanguages, nil},
				{"SearchmissingIncremental", "number", configv.SearchmissingIncremental, nil},
				{"SearchupgradeIncremental", "number", configv.SearchupgradeIncremental, nil},
			}, "media_main_"+typev+"_"+configv.Name, comments, displayNames, accordionId),

		// Data Sources
		renderMediaArraySection("Data Sources", "data-"+typev+"-"+configv.Name, datav,
			"Add Data Source", "/api/manage/mediadata/form/"+typev+"/"+configv.Name, csrfToken, accordionId),

		// Data Import
		renderMediaArraySection("Data Import", "import-"+typev+"-"+configv.Name, DataImport,
			"Add Data Import", "/api/manage/mediaimport/form/"+typev+"/"+configv.Name, csrfToken, accordionId),

		// Lists
		renderMediaArraySection("Lists", "lists-"+typev+"-"+configv.Name, Lists,
			"Add List", "/api/manage/medialists/form/"+typev+"/"+configv.Name, csrfToken, accordionId),

		// Notifications
		renderMediaArraySection("Notifications", "notifications-"+typev+"-"+configv.Name, Notification,
			"Add Notification", "/api/manage/medianotification/form/"+typev+"/"+configv.Name, csrfToken, accordionId),
	)
}

// renderMediaArraySection creates a collapsible section for array-based configurations
func renderMediaArraySection(title, id string, items []Node, addButtonText, addEndpoint, csrfToken, parentAccordion string) Node {
	collapseClass := "accordion-collapse collapse"
	expanded := len(items) > 0 // Expand if has items
	if expanded {
		collapseClass += " show"
	}

	return Div(
		Class("accordion-item"),
		Style("border: 1px solid #dee2e6; border-radius: 8px; margin-bottom: 0.5rem;"),
		H2(
			Class("accordion-header"),
			ID("heading"+id),
			Button(
				Class("accordion-button"),
				If(!expanded, Attr("class", "accordion-button collapsed")),
				Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: none; padding: 0.75rem 1rem; font-weight: 600;"),
				Type("button"),
				Attr("data-bs-toggle", "collapse"),
				Attr("data-bs-target", "#collapse"+id),
				Attr("aria-expanded", fmt.Sprintf("%t", expanded)),
				Attr("aria-controls", "collapse"+id),
				I(Class("fas fa-list me-2 text-primary")),
				Text(title),
				Span(Class("badge bg-primary ms-2"), Text(fmt.Sprintf("%d", len(items)))),
			),
		),
		Div(
			ID("collapse"+id),
			Class(collapseClass),
			Attr("aria-labelledby", "heading"+id),
			Attr("data-bs-parent", "#"+parentAccordion),
			Div(
				Class("accordion-body p-3"),
				Style("background-color: #fdfdfe;"),
				If(len(items) > 0,
					Div(Class("mb-3"), Group(items)),
				),
				Button(
					Type("button"),
					Class("btn btn-outline-primary btn-sm"),
					hx.Post(addEndpoint),
					hx.Target("#collapse"+id+" .accordion-body"),
					hx.Swap("beforeend"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					I(Class("fas fa-plus me-2")),
					Text(addButtonText),
				),
			),
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
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-video",
			"Media Configuration",
			"Configure media types, lists, qualities, indexers, and organizational settings for movies and TV series.",
		),

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
				createAddButton("Add Series", "#seriesContainer", "/api/manage/media/form/series", csrfToken),
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
				createAddButton("Add Movie", "#moviesContainer", "/api/manage/media/form/movies", csrfToken),
			),

			// Submit button
			createFormSubmitGroup("Save Configuration", "#addalert", "/api/admin/media/update", csrfToken),
			Div(ID("addalert")),
		),
	)
}

func renderDownloaderForm(configv *config.DownloaderConfig) Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "downloader_" + configv.Name

	return renderOptimizedArrayItemForm("downloader", configv.Name, "Downloader", configv,
		renderDownloaderConfigSections(configv, group, comments, displayNames))
}

// renderDownloaderConfigSections organizes downloader fields into logical groups
func renderDownloaderConfigSections(configv *config.DownloaderConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "downloaderConfigAccordion-" + sanitizedName

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-downloader-"+configv.Name, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
				{"DlType", "select", configv.DlType, map[string][]string{
					"options": {"drone", "nzbget", "sabnzbd", "transmission", "rtorrent", "qbittorrent", "deluge"},
				}},
				{"Enabled", "checkbox", configv.Enabled, nil},
			}, group, comments, displayNames, accordionId),

		// Connection Settings
		renderConfigGroupWithParent("Connection Settings", "connection-downloader-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Hostname", "text", configv.Hostname, nil},
				{"Port", "number", configv.Port, nil},
				{"Username", "text", configv.Username, nil},
				{"Password", "password", configv.Password, nil},
			}, group, comments, displayNames, accordionId),

		// Download Settings
		renderConfigGroupWithParent("Download Settings", "download-downloader-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"AddPaused", "checkbox", configv.AddPaused, nil},
				{"Priority", "number", configv.Priority, nil},
			}, group, comments, displayNames, accordionId),

		// Deluge-Specific Settings
		renderConfigGroupWithParent("Deluge Settings", "deluge-downloader-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"DelugeDlTo", "text", configv.DelugeDlTo, nil},
				{"DelugeMoveAfter", "checkbox", configv.DelugeMoveAfter, nil},
				{"DelugeMoveTo", "text", configv.DelugeMoveTo, nil},
			}, group, comments, displayNames, accordionId),
	)
}

// renderOptimizedArrayItemForm creates an optimized array item form with organized sections
func renderOptimizedArrayItemForm(itemType, name, displayName string, configv interface{}, sectionsContent Node) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(name, " ", "-"), "_", "-")
	collapseId := itemType + "_" + sanitizedName + "_collapse"

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px;"),
		Button(
			Class("card-header accordion-button w-100 border-0"),
			Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;"),
			Type("button"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			I(Class("fas fa-folder me-2 text-primary")),
			Text(displayName+" "+name),
		),
		Div(
			ID(collapseId),
			Class("collapse"),
			Div(
				Class(ClassCardBody),
				sectionsContent,
			),
		),
	)
}

// renderDownloaderConfig renders the downloader configuration section
func renderDownloaderConfig(configv []config.DownloaderConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Downloader Configuration",
		Subtitle:       "Configure download clients and connection settings for automated media acquisition from various sources and protocols.",
		Icon:           "download",
		AddButtonText:  "Add Downloader",
		AddEndpoint:    "/api/manage/downloader/form",
		FormContainer:  "downloaderContainer",
		SubmitEndpoint: "/api/admin/downloader/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.DownloaderConfig, token string) Node {
		return renderDownloaderForm(&config)
	})
}

func renderListsForm(configv *config.ListsConfig) Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "lists_" + configv.Name

	return renderOptimizedArrayItemForm("lists", configv.Name, "List", configv,
		renderListsConfigSections(configv, group, comments, displayNames))
}

// renderListsConfigSections organizes lists fields into logical groups
func renderListsConfigSections(configv *config.ListsConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "listsConfigAccordion-" + sanitizedName

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-lists-"+configv.Name, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
				{"ListType", "select", configv.ListType, map[string][]string{
					"options": {"seriesconfig", "traktpublicshowlist", "imdbcsv", "imdbfile", "traktpublicmovielist", "traktmoviepopular", "traktmovieanticipated", "traktmovietrending", "traktseriepopular", "traktserieanticipated", "traktserietrending", "newznabrss"},
				}},
				{"Enabled", "checkbox", configv.Enabled, nil},
			}, group, comments, displayNames, accordionId),

		// Source Configuration
		renderConfigGroupWithParent("Source Configuration", "source-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"URL", "text", configv.URL, nil},
				{"IMDBCSVFile", "text", configv.IMDBCSVFile, nil},
				{"SeriesConfigFile", "text", configv.SeriesConfigFile, nil},
			}, group, comments, displayNames, accordionId),

		// Trakt Settings
		renderConfigGroupWithParent("Trakt Settings", "trakt-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"TraktUsername", "text", configv.TraktUsername, nil},
				{"TraktListName", "text", configv.TraktListName, nil},
				{"TraktListType", "select", configv.TraktListType, map[string][]string{
					"options": {"movie", "show"},
				}},
			}, group, comments, displayNames, accordionId),

		// Filter Settings
		renderConfigGroupWithParent("Filter Settings", "filter-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Limit", "text", configv.Limit, nil},
				{"MinVotes", "number", configv.MinVotes, nil},
				{"MinRating", "number", configv.MinRating, nil},
				{"Excludegenre", "array", configv.Excludegenre, nil},
				{"Includegenre", "array", configv.Includegenre, nil},
			}, group, comments, displayNames, accordionId),

		// TMDB Settings
		renderConfigGroupWithParent("TMDB Settings", "tmdb-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"TmdbDiscover", "array", configv.TmdbDiscover, nil},
				{"TmdbList", "arrayint", configv.TmdbList, nil},
				{"RemoveFromList", "checkbox", configv.RemoveFromList, nil},
			}, group, comments, displayNames, accordionId),
	)
}

// renderListsConfig renders the lists configuration section
func renderListsConfig(configv []config.ListsConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Lists Configuration",
		Subtitle:       "Manage media lists and collections for organizing movies and TV series into custom groups with specific rules and criteria.",
		Icon:           "list",
		AddButtonText:  "Add List",
		AddEndpoint:    "/api/manage/lists/form",
		FormContainer:  "listsContainer",
		SubmitEndpoint: "/api/admin/list/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.ListsConfig, token string) Node {
		return renderListsForm(&config)
	})
}

func renderIndexersForm(configv *config.IndexersConfig) Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "indexers_" + configv.Name

	return renderOptimizedArrayItemForm("indexers", configv.Name, "Indexer", configv,
		renderIndexersConfigSections(configv, group, comments, displayNames))
}

// renderIndexersConfigSections organizes indexer fields into logical groups
func renderIndexersConfigSections(configv *config.IndexersConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "indexersConfigAccordion-" + sanitizedName

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-indexers-"+configv.Name, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
				{"IndexerType", "select", configv.IndexerType, map[string][]string{
					"options": {"torznab", "newznab", "torrent", "torrentrss"},
				}},
				{"Enabled", "checkbox", configv.Enabled, nil},
			}, group, comments, displayNames, accordionId),

		// Connection Settings
		renderConfigGroupWithParent("Connection Settings", "connection-indexers-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"URL", "text", configv.URL, nil},
				{"Apikey", "password", configv.Apikey, nil},
				{"Userid", "text", configv.Userid, nil},
				{"DisableTLSVerify", "checkbox", configv.DisableTLSVerify, nil},
				{"DisableCompression", "checkbox", configv.DisableCompression, nil},
				{"TimeoutSeconds", "number", configv.TimeoutSeconds, nil},
			}, group, comments, displayNames, accordionId),

		// RSS Settings
		renderConfigGroupWithParent("RSS Settings", "rss-indexers-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Rssenabled", "checkbox", configv.Rssenabled, nil},
				{"RssEntriesloop", "number", configv.RssEntriesloop, nil},
				{"Customrssurl", "text", configv.Customrssurl, nil},
				{"Customrsscategory", "text", configv.Customrsscategory, nil},
			}, group, comments, displayNames, accordionId),

		// Search Settings
		renderConfigGroupWithParent("Search Settings", "search-indexers-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Addquotesfortitlequery", "checkbox", configv.Addquotesfortitlequery, nil},
				{"MaxEntries", "number", configv.MaxEntries, nil},
				{"MaxAge", "number", configv.MaxAge, nil},
				{"TrustWithIMDBIDs", "checkbox", configv.TrustWithIMDBIDs, nil},
				{"TrustWithTVDBIDs", "checkbox", configv.TrustWithTVDBIDs, nil},
				{"CheckTitleOnIDSearch", "checkbox", configv.CheckTitleOnIDSearch, nil},
			}, group, comments, displayNames, accordionId),

		// Rate Limiting
		renderConfigGroupWithParent("Rate Limiting", "limits-indexers-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Limitercalls", "number", configv.Limitercalls, nil},
				{"Limiterseconds", "number", configv.Limiterseconds, nil},
				{"LimitercallsDaily", "number", configv.LimitercallsDaily, nil},
			}, group, comments, displayNames, accordionId),

		// Custom Settings
		renderConfigGroupWithParent("Custom Settings", "custom-indexers-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Customapi", "text", configv.Customapi, nil},
				{"Customurl", "text", configv.Customurl, nil},
				{"OutputAsJSON", "checkbox", configv.OutputAsJSON, nil},
			}, group, comments, displayNames, accordionId),
	)
}

// renderIndexersConfig renders the indexers configuration section
func renderIndexersConfig(configv []config.IndexersConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Indexers Configuration",
		Subtitle:       "Configure search indexers and sources for discovering and retrieving media content from various providers and trackers.",
		Icon:           "search",
		AddButtonText:  "Add Indexer",
		AddEndpoint:    "/api/manage/indexers/form",
		FormContainer:  "indexersContainer",
		SubmitEndpoint: "/api/admin/indexer/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.IndexersConfig, token string) Node {
		return renderIndexersForm(&config)
	})
}

func renderPathsForm(configv *config.PathsConfig) Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "paths_" + configv.Name

	return renderOptimizedArrayItemForm("paths", configv.Name, "Path", configv,
		renderPathsConfigSections(configv, group, comments, displayNames))
}

// renderPathsConfigSections organizes path fields into logical groups
func renderPathsConfigSections(configv *config.PathsConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "pathsConfigAccordion-" + sanitizedName

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-paths-"+sanitizedName, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
				{"Path", "text", configv.Path, nil},
				{"Upgrade", "checkbox", configv.Upgrade, nil},
			}, group, comments, displayNames, accordionId),

		// File Extensions
		renderConfigGroupWithParent("File Extensions", "extensions-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{"AllowedVideoExtensions", "array", configv.AllowedVideoExtensions, nil},
				{"AllowedOtherExtensions", "array", configv.AllowedOtherExtensions, nil},
				{"AllowedVideoExtensionsNoRename", "array", configv.AllowedVideoExtensionsNoRename, nil},
				{"AllowedOtherExtensionsNoRename", "array", configv.AllowedOtherExtensionsNoRename, nil},
				{"Blocked", "array", configv.Blocked, nil},
			}, group, comments, displayNames, accordionId),

		// Size & Language Filtering
		renderConfigGroupWithParent("Size & Language Filtering", "filtering-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{"MinSize", "number", configv.MinSize, nil},
				{"MaxSize", "number", configv.MaxSize, nil},
				{"MinVideoSize", "number", configv.MinVideoSize, nil},
				{"CleanupsizeMB", "number", configv.CleanupsizeMB, nil},
				{"AllowedLanguages", "array", configv.AllowedLanguages, nil},
				{"Disallowed", "array", configv.Disallowed, nil},
			}, group, comments, displayNames, accordionId),

		// Scanning Settings
		renderConfigGroupWithParent("Scanning Settings", "scanning-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{"UpgradeScanInterval", "number", configv.UpgradeScanInterval, nil},
				{"MissingScanInterval", "number", configv.MissingScanInterval, nil},
				{"MissingScanReleaseDatePre", "number", configv.MissingScanReleaseDatePre, nil},
			}, group, comments, displayNames, accordionId),

		// Quality Control
		renderConfigGroupWithParent("Quality Control", "quality-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{"CheckRuntime", "checkbox", configv.CheckRuntime, nil},
				{"MaxRuntimeDifference", "number", configv.MaxRuntimeDifference, nil},
				{"DeleteWrongRuntime", "checkbox", configv.DeleteWrongRuntime, nil},
				{"DeleteWrongLanguage", "checkbox", configv.DeleteWrongLanguage, nil},
				{"DeleteDisallowed", "checkbox", configv.DeleteDisallowed, nil},
			}, group, comments, displayNames, accordionId),

		// File Organization
		renderConfigGroupWithParent("File Organization", "organization-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{"Replacelower", "checkbox", configv.Replacelower, nil},
				{"Usepresort", "checkbox", configv.Usepresort, nil},
				{"PresortFolderPath", "text", configv.PresortFolderPath, nil},
				{"MoveReplaced", "checkbox", configv.MoveReplaced, nil},
				{"MoveReplacedTargetPath", "text", configv.MoveReplacedTargetPath, nil},
			}, group, comments, displayNames, accordionId),

		// File Permissions
		renderConfigGroupWithParent("File Permissions", "permissions-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{"SetChmod", "text", configv.SetChmod, nil},
				{"SetChmodFolder", "text", configv.SetChmodFolder, nil},
			}, group, comments, displayNames, accordionId),
	)
}

// renderPathsConfig renders the paths configuration section
// renderPathsConfig renders the paths configuration section
func renderPathsConfig(configv []config.PathsConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Paths Configuration",
		Subtitle:       "Define file system paths, directory structures, and naming conventions for organizing downloaded media content.",
		Icon:           "folder",
		AddButtonText:  "Add Path",
		AddEndpoint:    "/api/manage/paths/form",
		FormContainer:  "pathsContainer",
		SubmitEndpoint: "/api/admin/path/update",
	}

	return renderGenericConfigSection(configv, csrfToken, options, func(config config.PathsConfig, token string) Node {
		return renderPathsForm(&config)
	})
}

func renderNotificationForm(configv *config.NotificationConfig) Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "notifications_" + configv.Name

	return renderOptimizedArrayItemForm("notifications", configv.Name, "Notification", configv,
		renderNotificationConfigSections(configv, group, comments, displayNames))
}

// renderNotificationConfigSections organizes notification fields into logical groups
func renderNotificationConfigSections(configv *config.NotificationConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "notificationConfigAccordion-" + sanitizedName

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-notification-"+configv.Name, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
				{"NotificationType", "select", configv.NotificationType, map[string][]string{
					"options": {"csv", "pushover"},
				}},
			}, group, comments, displayNames, accordionId),

		// Configuration
		renderConfigGroupWithParent("Configuration", "config-notification-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Apikey", "text", configv.Apikey, nil},
				{"Recipient", "text", configv.Recipient, nil},
				{"Outputto", "text", configv.Outputto, nil},
			}, group, comments, displayNames, accordionId),
	)
}

// renderNotificationConfig renders the notification configuration section
func renderNotificationConfig(configv []config.NotificationConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Notification Configuration",
		Subtitle:       "Set up notification systems and alert mechanisms to stay informed about download status, errors, and system events.",
		Icon:           "bell",
		FormContainer:  "notificationContainer",
		AddButtonText:  "Add Notification",
		AddEndpoint:    "/api/manage/notification/form",
		SubmitEndpoint: "/api/admin/notification/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.NotificationConfig, token string) Node {
		return renderNotificationForm(&config)
	})
}

func renderRegexForm(configv *config.RegexConfig) Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "regex_" + configv.Name

	return renderOptimizedArrayItemForm("regex", configv.Name, "Regex", configv,
		renderRegexConfigSections(configv, group, comments, displayNames))
}

// renderRegexConfigSections organizes regex fields into logical groups
func renderRegexConfigSections(configv *config.RegexConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "regexConfigAccordion-" + sanitizedName

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-regex-"+configv.Name, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
			}, group, comments, displayNames, accordionId),

		// Pattern Rules
		renderConfigGroupWithParent("Pattern Rules", "patterns-regex-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"Required", "array", configv.Required, nil},
				{"Rejected", "array", configv.Rejected, nil},
			}, group, comments, displayNames, accordionId),
	)
}

// renderRegexConfig renders the regex configuration section
func renderRegexConfig(configv []config.RegexConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Regex Configuration",
		Subtitle:       "Create and manage regular expression patterns for parsing file names, extracting metadata, and content filtering.",
		Icon:           "code",
		FormContainer:  "regexContainer",
		AddButtonText:  "Add Regex",
		AddEndpoint:    "/api/manage/regex/form",
		SubmitEndpoint: "/api/admin/regex/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.RegexConfig, token string) Node {
		return renderRegexForm(&config)
	})
}

func renderQualityReorderForm(i int, mainname string, configv *config.QualityReorderConfig) Node {
	fields := []FormFieldDefinition{
		{"", "removebutton", "", nil},
		{"Name", "text", configv.Name, nil},
		{"ReorderType", "select", configv.ReorderType, map[string][]string{
			"options": {"resolution", "quality", "codec", "audio", "position", "combined_res_qual"},
		}},
		{"Newpriority", "number", configv.Newpriority, nil},
	}
	return renderArrayItemFormWithNameAndIndex("quality", mainname+"_reorder", i, "Reorder", configv, fields)
}

func renderQualityIndexerForm(i int, mainname string, configv *config.QualityIndexerConfig) Node {
	fields := []FormFieldDefinition{
		{"", "removebutton", "", nil},
		{"TemplateIndexer", "select", configv.TemplateIndexer, config.GetSettingTemplatesFor("indexer")},
		{"TemplateDownloader", "select", configv.TemplateDownloader, config.GetSettingTemplatesFor("downloader")},
		{"TemplateRegex", "select", configv.TemplateRegex, config.GetSettingTemplatesFor("regex")},
		{"TemplatePathNzb", "select", configv.TemplatePathNzb, config.GetSettingTemplatesFor("path")},
		{"CategoryDownloader", "text", configv.CategoryDownloader, nil},
		{"AdditionalQueryParams", "text", configv.AdditionalQueryParams, nil},
		{"SkipEmptySize", "checkbox", configv.SkipEmptySize, nil},
		{"HistoryCheckTitle", "checkbox", configv.HistoryCheckTitle, nil},
		{"CategoriesIndexer", "text", configv.CategoriesIndexer, nil},
	}
	return renderArrayItemFormWithNameAndIndex("quality", mainname+"_indexer", i, "Indexer", configv, fields)
}

func renderQualityForm(configv *config.QualityConfig, csrfToken string) Node {
	group := "quality_main_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	collapseId := group + "_main_collapse"

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px;"),
		Button(
			Class("card-header accordion-button w-100 border-0"),
			Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;"),
			Type("button"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			I(Class("fas fa-star me-2 text-primary")),
			Text("Quality "+configv.Name),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				// Organized quality sections
				renderQualityConfigSections(configv, group, comments, displayNames, csrfToken),
			),
		),
	)
}

// renderQualityConfigSections organizes quality fields into logical groups
func renderQualityConfigSections(configv *config.QualityConfig, group string, comments map[string]string, displayNames map[string]string, csrfToken string) Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "qualityConfigAccordion-" + sanitizedName

	// Prepare sub-arrays
	var QualityReorder []Node
	for i, qualityReorder := range configv.QualityReorder {
		QualityReorder = append(QualityReorder, renderQualityReorderForm(i, configv.Name, &qualityReorder))
	}
	var QualityIndexer []Node
	for i, qualityIndexer := range configv.Indexer {
		QualityIndexer = append(QualityIndexer, renderQualityIndexerForm(i, configv.Name, &qualityIndexer))
	}

	return Div(
		Class("accordion"),
		ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-quality-"+configv.Name, true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{"Name", "text", configv.Name, nil},
			}, group, comments, displayNames, accordionId),

		// Quality Preferences
		renderConfigGroupWithParent("Quality Preferences", "preferences-quality-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"WantedResolution", "arrayselectarray", configv.WantedResolution, database.GetSettingTemplatesFor("resolution")},
				{"WantedQuality", "arrayselectarray", configv.WantedQuality, database.GetSettingTemplatesFor("quality")},
				{"WantedAudio", "arrayselectarray", configv.WantedAudio, database.GetSettingTemplatesFor("audio")},
				{"WantedCodec", "arrayselectarray", configv.WantedCodec, database.GetSettingTemplatesFor("codec")},
				{"CutoffResolution", "arrayselect", configv.CutoffResolution, database.GetSettingTemplatesFor("resolution")},
				{"CutoffQuality", "arrayselect", configv.CutoffQuality, database.GetSettingTemplatesFor("quality")},
			}, group, comments, displayNames, accordionId),

		// Search Settings
		renderConfigGroupWithParent("Search Settings", "search-quality-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"SearchForTitleIfEmpty", "checkbox", configv.SearchForTitleIfEmpty, nil},
				{"BackupSearchForTitle", "checkbox", configv.BackupSearchForTitle, nil},
				{"SearchForAlternateTitleIfEmpty", "checkbox", configv.SearchForAlternateTitleIfEmpty, nil},
				{"BackupSearchForAlternateTitle", "checkbox", configv.BackupSearchForAlternateTitle, nil},
				{"ExcludeYearFromTitleSearch", "checkbox", configv.ExcludeYearFromTitleSearch, nil},
				{"TitleStripSuffixForSearch", "array", configv.TitleStripSuffixForSearch, nil},
				{"TitleStripPrefixForSearch", "array", configv.TitleStripPrefixForSearch, nil},
			}, group, comments, displayNames, accordionId),

		// Validation Settings
		renderConfigGroupWithParent("Validation Settings", "validation-quality-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"CheckUntilFirstFound", "checkbox", configv.CheckUntilFirstFound, nil},
				{"CheckTitle", "checkbox", configv.CheckTitle, nil},
				{"CheckTitleOnIDSearch", "checkbox", configv.CheckTitleOnIDSearch, nil},
				{"CheckYear", "checkbox", configv.CheckYear, nil},
				{"CheckYear1", "checkbox", configv.CheckYear1, nil},
			}, group, comments, displayNames, accordionId),

		// Priority Settings
		renderConfigGroupWithParent("Priority Settings", "priority-quality-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{"UseForPriorityResolution", "checkbox", configv.UseForPriorityResolution, nil},
				{"UseForPriorityQuality", "checkbox", configv.UseForPriorityQuality, nil},
				{"UseForPriorityAudio", "checkbox", configv.UseForPriorityAudio, nil},
				{"UseForPriorityCodec", "checkbox", configv.UseForPriorityCodec, nil},
				{"UseForPriorityOther", "checkbox", configv.UseForPriorityOther, nil},
				{"UseForPriorityMinDifference", "number", configv.UseForPriorityMinDifference, nil},
			}, group, comments, displayNames, accordionId),

		// Reorder Rules
		renderMediaArraySection("Reorder Rules", "reorder-quality-"+configv.Name, QualityReorder,
			"Add Reorder Rule", "/api/manage/qualityreorder/form/"+configv.Name, csrfToken, accordionId),

		// Indexer Settings
		renderMediaArraySection("Indexer Settings", "indexer-quality-"+configv.Name, QualityIndexer,
			"Add Indexer Setting", "/api/manage/qualityindexer/form/"+configv.Name, csrfToken, accordionId),
	)
}

// renderQualityConfig renders the quality configuration section
func renderQualityConfig(configv []config.QualityConfig, csrfToken string) Node {
	var elements []Node
	for _, quality := range configv {
		elements = append(elements, renderQualityForm(&quality, csrfToken))
	}
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fas fa-star",
			"Quality Configuration",
			"Configure quality profiles, resolution settings, codec preferences, and indexer priorities for media downloads.",
		),

		Form(
			Class("config-form"),

			Div(
				ID("qualityContainer"),
				Group(elements),
				// Quality items will be added here dynamically
			),
			createAddButton("Add Quality", "#qualityContainer", "/api/manage/quality/form", csrfToken),
			createFormSubmitGroup("Save Configuration", "#addalert", "/api/admin/quality/update", csrfToken),
			Div(ID("addalert")),
		),
	)
}

func renderSchedulerForm(configv *config.SchedulerConfig) Node {
	group := "scheduler_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	collapseId := group + "_main_collapse"
	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px;"),
		Button(
			Class("card-header accordion-button w-100 border-0"),
			Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;"),
			Type("button"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			I(Class("fas fa-clock me-2 text-primary")),
			Text("Scheduler "+configv.Name),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				// Organized scheduler sections
				renderSchedulerConfigSections(configv, group, comments, displayNames),
			),
		),
	)
}

// renderSchedulerConfigSections organizes scheduler fields into logical groups
func renderSchedulerConfigSections(configv *config.SchedulerConfig, group string, comments map[string]string, displayNames map[string]string) Node {
	return Div(
		Class("accordion"),
		ID("schedulerConfigAccordion-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), true,
			[]FormFieldDefinition{
				{"", "removebutton", "", nil},
				{Name: "Name", Type: "text", Value: configv.Name},
			}, group, comments, displayNames, "schedulerConfigAccordion-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")),

		// Feed Intervals
		renderConfigGroupWithParent("Feed Scheduling", "feeds-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{Name: "IntervalFeeds", Type: "text", Value: configv.IntervalFeeds},
				{Name: "IntervalFeedsRefreshSeries", Type: "text", Value: configv.IntervalFeedsRefreshSeries},
				{Name: "IntervalFeedsRefreshMovies", Type: "text", Value: configv.IntervalFeedsRefreshMovies},
				{Name: "IntervalFeedsRefreshSeriesFull", Type: "text", Value: configv.IntervalFeedsRefreshSeriesFull},
				{Name: "IntervalFeedsRefreshMoviesFull", Type: "text", Value: configv.IntervalFeedsRefreshMoviesFull},
				{Name: "CronFeeds", Type: "text", Value: configv.CronFeeds},
				{Name: "CronFeedsRefreshSeries", Type: "text", Value: configv.CronFeedsRefreshSeries},
				{Name: "CronFeedsRefreshMovies", Type: "text", Value: configv.CronFeedsRefreshMovies},
				{Name: "CronFeedsRefreshSeriesFull", Type: "text", Value: configv.CronFeedsRefreshSeriesFull},
				{Name: "CronFeedsRefreshMoviesFull", Type: "text", Value: configv.CronFeedsRefreshMoviesFull},
			}, group, comments, displayNames, "schedulerConfigAccordion-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")),

		// Indexer Intervals
		renderConfigGroupWithParent("Indexer Scheduling", "indexer-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{Name: "IntervalIndexerMissing", Type: "text", Value: configv.IntervalIndexerMissing},
				{Name: "IntervalIndexerUpgrade", Type: "text", Value: configv.IntervalIndexerUpgrade},
				{Name: "IntervalIndexerMissingFull", Type: "text", Value: configv.IntervalIndexerMissingFull},
				{Name: "IntervalIndexerUpgradeFull", Type: "text", Value: configv.IntervalIndexerUpgradeFull},
				{Name: "IntervalIndexerRss", Type: "text", Value: configv.IntervalIndexerRss},
				{Name: "IntervalIndexerRssSeasons", Type: "text", Value: configv.IntervalIndexerRssSeasons},
				{Name: "IntervalIndexerRssSeasonsAll", Type: "text", Value: configv.IntervalIndexerRssSeasonsAll},
				{Name: "CronIndexerMissing", Type: "text", Value: configv.CronIndexerMissing},
				{Name: "CronIndexerUpgrade", Type: "text", Value: configv.CronIndexerUpgrade},
				{Name: "CronIndexerMissingFull", Type: "text", Value: configv.CronIndexerMissingFull},
				{Name: "CronIndexerUpgradeFull", Type: "text", Value: configv.CronIndexerUpgradeFull},
				{Name: "CronIndexerRssSeasons", Type: "text", Value: configv.CronIndexerRssSeasons},
				{Name: "CronIndexerRssSeasonsAll", Type: "text", Value: configv.CronIndexerRssSeasonsAll},
			}, group, comments, displayNames, "schedulerConfigAccordion-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")),

		// Title Search Scheduling
		renderConfigGroupWithParent("Title Search Scheduling", "titles-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{Name: "IntervalIndexerMissingTitle", Type: "text", Value: configv.IntervalIndexerMissingTitle},
				{Name: "IntervalIndexerUpgradeTitle", Type: "text", Value: configv.IntervalIndexerUpgradeTitle},
				{Name: "IntervalIndexerMissingFullTitle", Type: "text", Value: configv.IntervalIndexerMissingFullTitle},
				{Name: "IntervalIndexerUpgradeFullTitle", Type: "text", Value: configv.IntervalIndexerUpgradeFullTitle},
				{Name: "CronIndexerMissingTitle", Type: "text", Value: configv.CronIndexerMissingTitle},
				{Name: "CronIndexerUpgradeTitle", Type: "text", Value: configv.CronIndexerUpgradeTitle},
				{Name: "CronIndexerMissingFullTitle", Type: "text", Value: configv.CronIndexerMissingFullTitle},
				{Name: "CronIndexerUpgradeFullTitle", Type: "text", Value: configv.CronIndexerUpgradeFullTitle},
			}, group, comments, displayNames, "schedulerConfigAccordion-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")),

		// Data Scanning & Maintenance
		renderConfigGroupWithParent("Data Scanning & Maintenance", "maintenance-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"), false,
			[]FormFieldDefinition{
				{Name: "IntervalImdb", Type: "text", Value: configv.IntervalImdb},
				{Name: "IntervalScanData", Type: "text", Value: configv.IntervalScanData},
				{Name: "IntervalScanDataMissing", Type: "text", Value: configv.IntervalScanDataMissing},
				{Name: "IntervalScanDataFlags", Type: "text", Value: configv.IntervalScanDataFlags},
				{Name: "IntervalScanDataimport", Type: "text", Value: configv.IntervalScanDataimport},
				{Name: "IntervalDatabaseBackup", Type: "text", Value: configv.IntervalDatabaseBackup},
				{Name: "IntervalDatabaseCheck", Type: "text", Value: configv.IntervalDatabaseCheck},
				{Name: "CronImdb", Type: "text", Value: configv.CronImdb},
			}, group, comments, displayNames, "schedulerConfigAccordion-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")),
	)
}

// renderSchedulerConfig renders the scheduler configuration section
// renderSchedulerConfig renders the scheduler configuration section
func renderSchedulerConfig(configv []config.SchedulerConfig, csrfToken string) Node {
	options := ConfigSectionOptions{
		SectionTitle:    "Scheduler Configuration",
		SectionSubtitle: "Configure automated scheduling rules and timing for various system tasks, jobs, and maintenance operations.",
		SectionIcon:     "clock",
		ContainerID:     "schedulerContainer",
		AddButtonText:   "Add Scheduler",
		AddFormPath:     "/api/manage/scheduler/form",
		UpdatePath:      "/api/admin/scheduler/update",
	}
	return renderConfigSection(configv, csrfToken, options, renderSchedulerForm)
}

// renderFormGroup renders a form group with label and input
func renderFormGroup(group string, comments map[string]string, displayNames map[string]string, name, inputType string, value any, options map[string][]string) Node {
	var input Node
	var iconClass string

	// Add icons based on input type
	switch inputType {
	case "text":
		iconClass = "fa-solid fa-font"
	case "number":
		iconClass = "fa-solid fa-hashtag"
	case "select", "selectarray":
		iconClass = "fa-solid fa-list"
	case "checkbox":
		iconClass = "fa-solid fa-toggle-on"
	case "textarea":
		iconClass = "fa-solid fa-align-left"
	case "array", "arrayselect", "arrayselectarray", "arrayint":
		iconClass = "fa-solid fa-layer-group"
	default:
		iconClass = "fa-solid fa-cog"
	}

	switch inputType {
	case "removebutton":
		return createRemoveButton(false)
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
		// Use createFormField for checkbox with custom checkbox-specific styling
		if addnode != nil {
			input = Input(
				Class("form-check-input-modern"),
				Type("checkbox"),
				Role("switch"),
				ID(group+"_"+name),
				Name(group+"_"+name), addnode,
			)
		} else {
			input = Input(
				Class("form-check-input-modern"),
				Type("checkbox"),
				Role("switch"),
				ID(group+"_"+name),
				Name(group+"_"+name),
			)
		}

	case "textarea":
		input = Textarea(
			Class(ClassFormControl),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Rows("3"),
			Value(value.(string)),
		)
	case "text":
		input = createFormField("text", group+"_"+name, value.(string), "", nil)
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
		input = createFormField("number", group+"_"+name, setvalue, "", nil)

	case "array":
		input = Div(
			ID(group+"_"+name+"-container"),
			Group(
				func() []Node {
					var nodes []Node
					for _, v := range value.([]string) {
						nodes = append(nodes, Div(
							Class(ClassDFlex),
							Input(
								Class(ClassFormControl+" me-2"),
								Type("text"),
								Name(group+"_"+name),
								Value(v),
							),
							Button(
								Class(ClassBtnDanger),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class(ClassBtnPrimary),
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
							Class(ClassDFlex),
							Select(
								Class(ClassFormSelect),
								Name(group+"_"+name),
								Group(optionElements),
							),
							Button(
								Class(ClassBtnDanger),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class(ClassBtnPrimary),
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
				Class(ClassDFlex),
				Select(
					Class(ClassFormSelect),
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
							Class(ClassDFlex),
							Input(
								Class(ClassFormControl+" me-2"),
								Type("number"),
								Name(group+"_"+name),
								Value(strconv.Itoa(v)),
							),
							Button(
								Class(ClassBtnDanger),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class(ClassBtnPrimary),
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
		input = createFormField(inputType, group+"_"+name, "", "", nil)
	}

	// Enhanced form group with compact comment display using tooltips
	var commentNode Node
	if comment, exists := comments[name]; exists && comment != "" {
		// Create compact comment display with tooltip or collapsible help
		shortComment := comment
		if len(comment) > 80 {
			// Truncate long comments and show first line only
			lines := strings.Split(comment, "\n")
			shortComment = strings.TrimSpace(lines[0])
			if len(shortComment) > 80 {
				shortComment = shortComment[:77] + "..."
			}
		}

		// Create a compact help icon with tooltip for long comments
		if len(comment) > 80 || strings.Contains(comment, "\n") {
			commentNode = Div(
				Div(
					Class("d-flex align-items-center mt-1"),
					Small(
						Class("text-muted me-2"),
						Style("font-size: 0.75em;"),
						Text(shortComment),
					),
					Button(
						Type("button"),
						Class("btn btn-outline-info btn-sm"),
						Style("font-size: 0.85em; padding: 0.25rem 0.5rem; border-radius: 0.375rem;"),
						Attr("data-bs-toggle", "collapse"),
						Attr("data-bs-target", "#help-"+group+"-"+name),
						Attr("aria-expanded", "false"),
						Attr("title", "Show detailed help"),
						I(Class("fas fa-info-circle me-1")),
						Text("Help"),
					),
				),
				Div(
					ID("help-"+group+"-"+name),
					Class("collapse mt-2"),
					Div(
						Class("alert alert-info"),
						Style("font-size: 0.85em; padding: 0.75rem; margin: 0; border-radius: 0.375rem;"),
						Strong(Text("Help: ")),
						Div(Class("mt-1"), Group(createCommentLines(comment))),
					),
				),
			)
		} else {
			// Short single line comment - display compactly
			commentNode = Small(
				Class("form-text text-muted"),
				Style("font-size: 0.75em; margin-top: 0.25rem;"),
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
			Class("form-group-enhanced mb-4"),
			Div(
				Class("form-check-wrapper p-3 border rounded-3 bg-light"),
				Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: 1px solid #dee2e6 !important; transition: all 0.2s ease;"),
				Div(
					Class("form-check form-switch"),
					I(Class(iconClass+" text-primary me-2")),
					input,
					createFormLabel(group+"_"+name, displayName, true),
				),
				commentNode,
			),
		)
	}

	return Div(
		Class("form-group mb-2"),
		Div(
			Class("form-field-compact p-2 border rounded"),
			Style("background: #ffffff; border: 1px solid #e3e6ea !important; transition: border-color 0.15s ease-in-out;"),
			Div(
				Class("d-flex align-items-center mb-1"),
				I(Class(iconClass+" text-primary me-2"), Style("font-size: 0.85em;")),
				createFormLabel(group+"_"+name, displayName, false),
			),
			input,
			commentNode,
		),
	)
}

// renderFormFields renders multiple form fields using FormFieldDefinition array
func renderFormFields(group string, comments map[string]string, displayNames map[string]string, fields []FormFieldDefinition) []Node {
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}
	return formGroups
}

// renderTestParsePage renders a page for testing string parsing
func renderTestParsePage(csrfToken string) Node {
	media := config.GetSettingsMediaAll()
	lists := make([]string, 0, len(media.Movies)+len(media.Series))
	for i := range media.Movies {
		lists = append(lists, media.Movies[i].NamePrefix)
	}
	for i := range media.Series {
		lists = append(lists, media.Series[i].NamePrefix)
	}
	qualitycfg := config.GetSettingsQualityAll()
	qualities := make([]string, 0, len(qualitycfg))
	for i := range qualitycfg {
		qualities = append(qualities, qualitycfg[i].Name)
	}
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-flask",
			"String Parse Test",
			"Test the file parser with different filename patterns. This tool helps you understand how the parser extracts metadata from filenames.",
		),

		Form(
			Class("config-form"),
			ID("parseTestForm"),

			renderFormGroup("testparse", map[string]string{
				"Filename": "Enter a filename to parse (e.g., 'The.Matrix.1999.1080p.BluRay.x264-RARBG')",
			}, map[string]string{
				"Filename": "Filename",
			}, "Filename", "text", "", nil),

			renderFormGroup("testparse", map[string]string{
				"ConfigKey": "Select the media configuration to use for parsing",
			}, map[string]string{
				"ConfigKey": "Media Config",
			}, "ConfigKey", "select", "", map[string][]string{
				"options": lists,
			}),

			renderFormGroup("testparse", map[string]string{
				"QualityKey": "Select the quality configuration to use",
			}, map[string]string{
				"QualityKey": "Quality Config",
			}, "QualityKey", "select", "", map[string][]string{
				"options": qualities,
			}),

			renderFormGroup("testparse", map[string]string{
				"UsePath": "Parse as full path instead of just filename",
			}, map[string]string{
				"UsePath": "Use Path",
			}, "UsePath", "checkbox", false, nil),

			renderFormGroup("testparse", map[string]string{
				"UseFolder": "Use folder information in parsing",
			}, map[string]string{
				"UseFolder": "Use Folder",
			}, "UseFolder", "checkbox", false, nil),

			renderHTMXSubmitButton("Parse", "parseResults", "/api/admin/testparse", "parseTestForm", csrfToken),
			Div(
				Class("form-group submit-group"),
				createButton("Reset", "button", "btn btn-secondary ml-2",
					Attr("onclick", "document.getElementById('parseTestForm').reset(); document.getElementById('parseResults').innerHTML = '';")),
			),
		),

		Div(
			ID("parseResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// renderParseResults renders the parsing results in a formatted table
func renderParseResults(m *database.ParseInfo, originalFilename, configKey, qualityKey string) string {
	resultRows := []Node{
		// Header information
		Tr(Td(Strong(Text("Original Filename:"))), Td(Text(originalFilename))),
		Tr(Td(Strong(Text("Config Used:"))), Td(Text(configKey))),
		Tr(Td(Strong(Text("Quality Used:"))), Td(Text(qualityKey))),
		Tr(Td(Attr("colspan", "2"), Hr())),

		// Basic media information
		Tr(Td(Strong(Text("Title:"))), Td(Text(m.Title))),
		Tr(Td(Strong(Text("File:"))), Td(Text(m.File))),
		Tr(Td(Strong(Text("Year:"))), Td(Text(fmt.Sprintf("%d", m.Year)))),
		Tr(Td(Strong(Text("Date:"))), Td(Text(m.Date))),
		Tr(Td(Strong(Text("Identifier:"))), Td(Text(m.Identifier))),

		// Episode/Season information
		Tr(Td(Strong(Text("Season:"))), Td(Text(fmt.Sprintf("%d", m.Season)))),
		Tr(Td(Strong(Text("Season String:"))), Td(Text(m.SeasonStr))),
		Tr(Td(Strong(Text("Episode:"))), Td(Text(fmt.Sprintf("%d", m.Episode)))),
		Tr(Td(Strong(Text("Episode String:"))), Td(Text(m.EpisodeStr))),

		// Quality and technical information
		Tr(Td(Strong(Text("Quality:"))), Td(Text(m.Quality))),
		Tr(Td(Strong(Text("Resolution:"))), Td(Text(m.Resolution))),
		Tr(Td(Strong(Text("Codec:"))), Td(Text(m.Codec))),
		Tr(Td(Strong(Text("Audio:"))), Td(Text(m.Audio))),
		Tr(Td(Strong(Text("Width x Height:"))), Td(Text(fmt.Sprintf("%d x %d", m.Width, m.Height)))),
		Tr(Td(Strong(Text("Runtime:"))), Td(Text(fmt.Sprintf("%d min", m.Runtime)))),
		Tr(Td(Strong(Text("Languages:"))), Td(Text(strings.Join(m.Languages, ", ")))),

		// Flags and properties
		Tr(Td(Strong(Text("Extended:"))), Td(Text(fmt.Sprintf("%t", m.Extended)))),
		Tr(Td(Strong(Text("Proper:"))), Td(Text(fmt.Sprintf("%t", m.Proper)))),
		Tr(Td(Strong(Text("Repack:"))), Td(Text(fmt.Sprintf("%t", m.Repack)))),
		Tr(Td(Strong(Text("Priority:"))), Td(Text(fmt.Sprintf("%d", m.Priority)))),

		// External IDs
		Tr(Td(Strong(Text("IMDB ID:"))), Td(Text(m.Imdb))),
		Tr(Td(Strong(Text("TVDB ID:"))), Td(Text(m.Tvdb))),

		// Internal information
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("List ID:"))), Td(Text(fmt.Sprintf("%d", m.ListID)))),
		Tr(Td(Strong(Text("First IDX:"))), Td(Text(fmt.Sprintf("%d", m.FirstIDX)))),
		Tr(Td(Strong(Text("First Year IDX:"))), Td(Text(fmt.Sprintf("%d", m.FirstYearIDX)))),
		Tr(Td(Strong(Text("Temp ID:"))), Td(Text(fmt.Sprintf("%d", m.TempID)))),

		// Database IDs
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Quality ID:"))), Td(Text(fmt.Sprintf("%d", m.QualityID)))),
		Tr(Td(Strong(Text("Resolution ID:"))), Td(Text(fmt.Sprintf("%d", m.ResolutionID)))),
		Tr(Td(Strong(Text("Codec ID:"))), Td(Text(fmt.Sprintf("%d", m.CodecID)))),
		Tr(Td(Strong(Text("Audio ID:"))), Td(Text(fmt.Sprintf("%d", m.AudioID)))),

		// Movie IDs
		Tr(Td(Strong(Text("DB Movie ID:"))), Td(Text(fmt.Sprintf("%d", m.DbmovieID)))),
		Tr(Td(Strong(Text("Movie ID:"))), Td(Text(fmt.Sprintf("%d", m.MovieID)))),

		// Series IDs
		Tr(Td(Strong(Text("DB Serie ID:"))), Td(Text(fmt.Sprintf("%d", m.DbserieID)))),
		Tr(Td(Strong(Text("DB Serie Episode ID:"))), Td(Text(fmt.Sprintf("%d", m.DbserieEpisodeID)))),
		Tr(Td(Strong(Text("Serie ID:"))), Td(Text(fmt.Sprintf("%d", m.SerieID)))),
		Tr(Td(Strong(Text("Serie Episode ID:"))), Td(Text(fmt.Sprintf("%d", m.SerieEpisodeID)))),

		// Internal strings (if not empty)
		func() Node {
			if m.Str != "" {
				return Tr(Td(Strong(Text("Internal String:"))), Td(Text(m.Str)))
			}
			return Text("")
		}(),
		func() Node {
			if m.RuntimeStr != "" {
				return Tr(Td(Strong(Text("Runtime String:"))), Td(Text(m.RuntimeStr)))
			}
			return Text("")
		}(),
		func() Node {
			if m.TempTitle != "" {
				return Tr(Td(Strong(Text("Temp Title:"))), Td(Text(m.TempTitle)))
			}
			return Text("")
		}(),
	}

	results := Div(
		Class("card border-0 shadow-sm border-success mb-4"),
		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center justify-content-between"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-success me-3"), I(Class("fas fa-code me-1")), Text("Parse")),
					H5(Class("card-title mb-0 text-success fw-bold"), Text("Parse Results")),
				),
				Span(Class("badge bg-success"), Text(fmt.Sprintf("%d", len(resultRows)))),
			),
		),
		Div(
			Class("card-body p-0"),
			func() Node {
				if len(resultRows) == 0 {
					return Div(
						Class("text-center p-5"),
						I(Class("fas fa-code mb-3"), Style("font-size: 3rem; color: #28a745; opacity: 0.3;")),
						H5(Class("text-muted mb-2"), Text("No Parse Results")),
						P(Class("text-muted small mb-0"), Text("No parsing operations have been performed yet.")),
					)
				}
				return Table(
					Class("table table-hover mb-0"),
					Style("background: transparent;"),
					TBody(Group(resultRows)),
				)
			}(),
		),
	)

	return renderComponentToString(results)
}

// renderComponentToString renders a gomponents Node to a string using pooled string builder
func renderComponentToString(node Node) string {
	sb := getStringBuilder()
	defer putStringBuilder(sb)
	node.Render(sb)
	return sb.String()
}

// renderTraktAuthPage renders a page for Trakt authentication
func renderTraktAuthPage(csrfToken string) Node {
	// Check current token status
	token := apiexternal.GetTraktToken()
	hasValidToken := token != nil && token.AccessToken != ""

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-key",
			"Trakt Authentication",
			"Authenticate with Trakt.tv to enable synchronized watchlists, ratings, and recommendations. This uses OAuth2 for secure authentication.",
		),

		// Current status display
		Div(
			func() Node {
				if hasValidToken {
					return Class("card border-0 shadow-sm")
				}
				return Class("card border-0 shadow-sm border-warning")
			}(),
			Div(
				Class("card-body"),
				H5(Class("card-title fw-bold mb-3"), Text("Current Authentication Status")),
				Div(Class("d-flex align-items-center mb-3"), func() Node {
					if hasValidToken {
						return Div(
							Span(Class("badge bg-success me-2"), I(Class("fas fa-check me-1")), Text("Authenticated")),
							Span(Class("text-success"), Text("Trakt API access is enabled")),
						)
					}
					return Div(
						Span(Class("badge bg-danger me-2"), I(Class("fas fa-times me-1")), Text("Not Authenticated")),
						Span(Class("text-danger"), Text("Trakt API access is disabled")),
					)
				}()),
				func() Node {
					if hasValidToken {
						return Div(
							Class("mt-3"),
							H6(Class("fw-bold mb-2"), Text("Token Details")),
							Ul(Class("list-unstyled"),
								Li(Class("mb-1"),
									Span(Class("badge bg-secondary me-2"), I(Class("fas fa-key me-1")), Text("Access Token")),
									Code(Class("text-muted"), Text(token.AccessToken[:20]+"...")),
								),
								Li(Class("mb-1"),
									Span(Class("badge bg-info me-2"), I(Class("fas fa-tag me-1")), Text("Type")),
									Text(token.TokenType),
								),
								Li(Class("mb-0"),
									Span(Class("badge bg-warning me-2"), I(Class("fas fa-clock me-1")), Text("Expiry")),
									Text(func() string {
										if token.Expiry.IsZero() {
											return "Never expires"
										}
										return token.Expiry.Format("2006-01-02 15:04:05")
									}()),
								),
							),
						)
					}
					return Text("")
				}(),
			),
		),

		// Authentication form
		Form(
			Class("config-form"),
			ID("traktAuthForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Step 1: Get Authorization URL")),
					P(Text("Click the button below to generate an authorization URL. This will open Trakt.tv in a new tab where you can authorize this application.")),

					Button(
						Class("btn btn-info"),
						Text("Get Trakt Authorization URL"),
						Type("button"),
						hx.Target("#authUrlResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"get_url\"}"),
					),
					Div(
						ID("authUrlResult"),
						Class("mt-3"),
						Style("min-height: 30px;"),
					),
				),

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Step 2: Enter Authorization Code")),
					P(Text("After authorizing on Trakt.tv, you'll receive a code. Enter it below to complete the authentication process.")),

					renderFormGroup("trakt", map[string]string{
						"AuthCode": "Enter the authorization code from Trakt.tv",
					}, map[string]string{
						"AuthCode": "Authorization Code",
					}, "AuthCode", "text", "", nil),

					Button(
						Class(ClassBtnSuccess),
						Text("Store Trakt Token"),
						Type("button"),
						hx.Target("#tokenResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Include("#traktAuthForm"),
						hx.Vals("{\"action\": \"store_token\"}"),
					),
					Div(
						ID("tokenResult"),
						Class("mt-3"),
						Style("min-height: 30px;"),
					),
				),
			),
		),

		// Token management section
		func() Node {
			if hasValidToken {
				return Div(
					Class("mt-4"),
					H5(Class("form-section-title"), Text("Token Management")),
					Div(
						Class("btn-group"),
						Button(
							Class("btn btn-warning"),
							Text("Refresh Token"),
							Type("button"),
							hx.Target("#refreshResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"refresh_token\"}"),
						),
						Button(
							Class("btn btn-danger ml-2"),
							Text("Revoke Token"),
							Type("button"),
							hx.Target("#revokeResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"revoke_token\"}"),
							Attr("onclick", "return confirm('Are you sure you want to revoke the Trakt authentication? This will disable Trakt API access.')"),
						),
					),
					Div(
						ID("refreshResult"),
						Class("mt-3"),
					),
					Div(
						ID("revokeResult"),
						Class("mt-3"),
					),
				)
			}
			return Text("")
		}(),

		// API Test section
		func() Node {
			if hasValidToken {
				return Div(
					Class("mt-4"),
					H5(Class("form-section-title"), Text("API Test")),
					P(Text("Test the Trakt API connection by fetching popular movies:")),
					Button(
						Class(ClassBtnSecondary),
						Text("Test Trakt API"),
						Type("button"),
						hx.Target("#apiTestResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"test_api\"}"),
					),
					Div(
						ID("apiTestResult"),
						Class("mt-3"),
						Style("min-height: 30px;"),
					),
				)
			}
			return Text("")
		}(),

		// Instructions
		Div(
			Class("mt-4 card border-0 shadow-sm border-primary mb-4"),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, #cfe2ff 0%, #b6d7ff 100%); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-primary me-3"), I(Class("fas fa-cog me-1")), Text("Setup")),
					H5(Class("card-title mb-0 text-primary fw-bold"), Text("Setup Instructions")),
				),
			),
			Div(
				Class("card-body"),
				P(Class("card-text text-muted mb-3"), Text("Follow these steps to set up Trakt authentication")),
				Ol(
					Class("mb-0 list-unstyled"),
					Li(Class("mb-2"), Text("1. Make sure your Trakt Client ID and Client Secret are configured in the General settings")),
					Li(Class("mb-2"), Text("2. Click 'Get Trakt Authorization URL' to generate the authorization link")),
					Li(Class("mb-2"), Text("3. Visit the generated URL and authorize this application on Trakt.tv")),
					Li(Class("mb-2"), Text("4. Copy the authorization code from the redirect URL")),
					Li(Class("mb-2"), Text("5. Paste the code in the form above and click 'Store Trakt Token'")),
					Li(Class("mb-0"), Text("6. Your Trakt authentication is now complete and will be stored securely")),
				),
			),
		),
	)
}
