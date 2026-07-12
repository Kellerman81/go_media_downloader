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
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// templateConfigInfo maps template field names to their configuration page URLs and types.
type templateConfigInfo struct {
	URL  string // The config page URL
	Type string // The config type for the preview endpoint
}

// getTemplateConfigInfo returns the config URL and type for a template field name.
// Returns empty struct if the field is not a template reference field.
func getTemplateConfigInfo(fieldName string) templateConfigInfo {
	mapping := map[string]templateConfigInfo{
		"TemplatePath":       {URL: "/api/admin/config/paths", Type: "path"},
		"TemplatePathNzb":    {URL: "/api/admin/config/paths", Type: "path"},
		"TemplateList":       {URL: "/api/admin/config/lists", Type: "list"},
		"TemplateQuality":    {URL: "/api/admin/config/quality", Type: "quality"},
		"TemplateScheduler":  {URL: "/api/admin/config/scheduler", Type: "scheduler"},
		"TemplateIndexer":    {URL: "/api/admin/config/indexers", Type: "indexer"},
		"TemplateDownloader": {URL: "/api/admin/config/downloader", Type: "downloader"},
		"TemplateRegex":      {URL: "/api/admin/config/regex", Type: "regex"},
		"MapNotification":    {URL: "/api/admin/config/notifications", Type: "notification"},
	}

	return mapping[fieldName]
}

// isTemplateField returns true if the field name is a template reference field.
// func isTemplateField(fieldName string) bool {
// 	info := getTemplateConfigInfo(fieldName)
// 	return info.URL != ""
// }

// renderConfigPreviewReadonly wraps a config form in a readonly preview container.
// This is used for the modal preview of template configurations.
// Note: The JavaScript to expand collapsed sections is in web_pages.go (expandAllCollapsedSections)
// and runs after the fetch completes, since inline scripts don't execute when inserted via innerHTML.
// CSS disables form inputs while keeping collapse buttons and scrolling interactive.
func renderConfigPreviewReadonly(title string, content gomponents.Node) gomponents.Node {
	return html.Div(
		html.Class("config-preview-readonly"),
		html.Div(
			html.Class("preview-header mb-3 pb-2 border-bottom"),
			html.H5(
				html.Class("text-primary mb-0"),
				html.I(html.Class("fas fa-eye me-2")),
				gomponents.Text(title),
			),
			html.Small(
				html.Class("text-muted"),
				gomponents.Text("Read-only preview - click headers to expand/collapse"),
			),
		),
		html.Div(
			html.Class("preview-content"),
			html.Style("max-height: 60vh; overflow-y: auto;"),
			content,
		),
		// CSS to disable form inputs but keep collapse buttons and scrolling working
		html.StyleEl(
			gomponents.Raw(`
				.config-preview-readonly input,
				.config-preview-readonly select,
				.config-preview-readonly textarea,
				.config-preview-readonly .btn-danger,
				.config-preview-readonly .btn-primary,
				.config-preview-readonly .btn-secondary:not([data-bs-toggle="collapse"]),
				.config-preview-readonly .selectwithpreview button {
					pointer-events: none;
					opacity: 0.7;
				}
				.config-preview-readonly .accordion-button,
				.config-preview-readonly [data-bs-toggle="collapse"] {
					pointer-events: auto;
					cursor: pointer;
				}
			`),
		),
	)
}

// renderEnhancedPageHeader creates a standardized enhanced page header with gradient background.
func renderEnhancedPageHeader(iconClass, title, subtitle string) gomponents.Node {
	return html.Div(
		html.Class("page-header-enhanced"),
		html.Div(
			html.Class("header-content"),
			html.Div(
				html.Class("header-icon-wrapper"),
				html.I(html.Class(iconClass+" header-icon")),
			),
			html.Div(
				html.Class("header-text"),
				html.H2(html.Class("header-title"), gomponents.Text(title)),
				html.P(html.Class("header-subtitle"), gomponents.Text(subtitle)),
			),
		),
	)
}

// renderHTMXSubmitButton creates a standardized HTMX form submit button.
func renderHTMXSubmitButton(
	buttonText, targetID, endpoint, formID, csrfToken string,
) gomponents.Node {
	return html.Div(
		html.Class("form-group submit-group"),
		html.Button(
			html.Class("btn btn-primary"),
			gomponents.Text(buttonText),
			html.Type("button"),
			hx.Target("#"+targetID),
			hx.Swap("innerHTML"),
			hx.Post(endpoint),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			hx.Include("#"+formID),
		),
	)
}

// renderGenericConfigSection creates a standardized config section.
func renderGenericConfigSection[T any](
	configs []T,
	csrfToken string,
	options RenderConfigOptions,
	renderItemFunc func(T, string) gomponents.Node,
) gomponents.Node {
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
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-"+options.Icon+" header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text(options.Title)),
					html.P(html.Class("header-subtitle"), gomponents.Text(options.Subtitle)),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.Div(
				append([]gomponents.Node{html.ID(options.FormContainer)}, containerContent...)...),
			createAddButton(
				options.AddButtonText,
				"#"+options.FormContainer,
				options.AddEndpoint,
				csrfToken,
			),

			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				options.SubmitEndpoint,
				csrfToken,
			),
			html.Div(html.ID("addalert")),
		),
	)
}

// renderConfigSection creates a generic config section with form elements.
func renderConfigSection[T any](
	configList []T,
	csrfToken string,
	options ConfigSectionOptions,
	renderForm func(*T) gomponents.Node,
) gomponents.Node {
	elements := make([]gomponents.Node, 0, len(configList))
	for _, config := range configList {
		elements = append(elements, renderForm(&config))
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-"+options.SectionIcon+" header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text(options.SectionTitle)),
					html.P(html.Class("header-subtitle"), gomponents.Text(options.SectionSubtitle)),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.Div(
				html.ID(options.ContainerID),
				gomponents.Group(elements),
			),
			createAddButton(
				options.AddButtonText,
				"#"+options.ContainerID,
				options.AddFormPath,
				csrfToken,
			),
			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				options.UpdatePath,
				csrfToken,
			),
			html.Div(html.ID("addalert")),
			html.Script(
				gomponents.Raw(`
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

func renderArrayItemFormWithIndex(
	prefix string,
	i int,
	headerText string,
	config any,
	fields []FormFieldDefinition,
) gomponents.Node {
	group := prefix + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		formGroups = append(
			formGroups,
			renderFormGroup(
				group,
				comments,
				displayNames,
				field.Name,
				field.Type,
				field.Value,
				convertSelectOptionsToMap(field.Options),
			),
		)
	}

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px; padding: 10px;"),
		html.Div(
			html.Class(ClassCardHeader),
			gomponents.Attr(
				"style",
				"cursor: pointer; display: flex; justify-content: space-between; align-items: center;",
			),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			gomponents.Text(headerText),
			html.Span(
				html.Class(ClassBadge),
				gomponents.Text("▼"),
			),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// createRemoveButton(true),
				gomponents.Group(formGroups),
			),
		),
	)
}

func renderArrayItemFormWithNameAndIndex(
	namePrefix string,
	mainname string,
	i int,
	headerText string,
	config any,
	fields []FormFieldDefinition,
) gomponents.Node {
	group := namePrefix + "_" + mainname + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		formGroups = append(
			formGroups,
			renderFormGroup(
				group,
				comments,
				displayNames,
				field.Name,
				field.Type,
				field.Value,
				convertSelectOptionsToMap(field.Options),
			),
		)
	}

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px; padding: 10px;"),
		html.Div(
			html.Class(ClassCardHeader),
			gomponents.Attr(
				"style",
				"cursor: pointer; display: flex; justify-content: space-between; align-items: center;",
			),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			gomponents.Text(headerText+" "+mainname),
			html.Span(
				html.Class(ClassBadge),
				gomponents.Text("▼"),
			),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// createRemoveButton(true),
				gomponents.Group(formGroups),
			),
		),
	)
}

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

func convertSelectOptionsToMap(optionsMap []SelectOption) map[string][]string {
	selectOptions := make([]string, 0, len(optionsMap))
	for _, row := range optionsMap {
		selectOptions = append(selectOptions, row.Label)
	}

	return map[string][]string{"options": selectOptions}
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
				{Name: "WebAPIKey", Type: "text", Value: configv.WebAPIKey},
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
						map[string][]string{"options": {"info", "debug"}},
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
				{Name: "OmdbAPIKey", Type: "text", Value: configv.OmdbAPIKey},
				{Name: "TheMovieDBApiKey", Type: "text", Value: configv.TheMovieDBApiKey},
				{Name: "TraktClientID", Type: "text", Value: configv.TraktClientID},
				{Name: "TraktClientSecret", Type: "text", Value: configv.TraktClientSecret},
				{Name: "TraktRedirectUrl", Type: "text", Value: configv.TraktRedirectUrl},
				{Name: "SpotifyClientID", Type: "text", Value: configv.SpotifyClientID},
				{Name: "SpotifyClientSecret", Type: "text", Value: configv.SpotifyClientSecret},
				{Name: "SpotifyRegion", Type: "text", Value: configv.SpotifyRegion},
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
				{Name: "UnzipPath", Type: "text", Value: configv.UnzipPath},
				{Name: "TarPath", Type: "text", Value: configv.TarPath},
			}, group, comments, displayNames),

		// Book/Audiobook/Music Provider API Keys
		renderConfigGroup("Book/Music Provider API Keys", "provider_keys", false,
			[]FormFieldDefinition{
				{Name: "GoodreadsAPIKey", Type: "text", Value: configv.GoodreadsAPIKey},
				{Name: "DiscogsToken", Type: "text", Value: configv.DiscogsToken},
				{Name: "AcoustIDAPIKey", Type: "text", Value: configv.AcoustIDAPIKey},
				{Name: "LastFMAPIKey", Type: "text", Value: configv.LastFMAPIKey},
				{Name: "TheAudioDBAPIKey", Type: "text", Value: configv.TheAudioDBAPIKey},
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

// renderConfigGroup creates a collapsible group of configuration fields.
func renderConfigGroup(
	title, id string,
	expanded bool,
	fields []FormFieldDefinition,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	return renderConfigGroupWithParent(
		title,
		id,
		expanded,
		fields,
		group,
		comments,
		displayNames,
		"generalConfigAccordion",
	)
}

// renderConfigGroupWithParent creates a collapsible group with specified parent accordion.
func renderConfigGroupWithParent(
	title, id string,
	expanded bool,
	fields []FormFieldDefinition,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	parentAccordion string,
) gomponents.Node {
	collapseClass := "accordion-collapse collapse"
	if expanded {
		collapseClass += " show"
	}

	return html.Div(
		html.Class("accordion-item"),
		html.Style("border: 1px solid #dee2e6; border-radius: 8px; margin-bottom: 0.5rem;"),
		html.H2(
			html.Class("accordion-header"),
			html.ID("heading"+id),
			html.Button(
				html.Class("accordion-button"),
				gomponents.If(!expanded, gomponents.Attr("class", "accordion-button collapsed")),
				html.Style(
					"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: none; padding: 0.75rem 1rem; font-weight: 600;",
				),
				html.Type("button"),
				gomponents.Attr("data-bs-toggle", "collapse"),
				gomponents.Attr("data-bs-target", "#collapse"+id),
				gomponents.Attr("aria-expanded", fmt.Sprintf("%t", expanded)),
				gomponents.Attr("aria-controls", "collapse"+id),
				html.I(html.Class("fas fa-cog me-2 text-primary")),
				gomponents.Text(title),
				html.Span(
					html.Class("badge bg-primary ms-2"),
					gomponents.Text(fmt.Sprintf("%d", len(fields))),
				),
			),
		),
		html.Div(
			html.ID("collapse"+id),
			html.Class(collapseClass),
			gomponents.Attr("aria-labelledby", "heading"+id),
			gomponents.Attr("data-bs-parent", "#"+parentAccordion),
			html.Div(
				html.Class("accordion-body p-3"),
				html.Style("background-color: #fdfdfe;"),
				// Use compact grid layout for fields
				renderCompactFormFields(group, comments, displayNames, fields),
			),
		),
	)
}

// renderCompactFormFields creates a more compact grid layout for configuration fields.
func renderCompactFormFields(
	group string,
	comments map[string]string,
	displayNames map[string]string,
	fields []FormFieldDefinition,
) gomponents.Node {
	var formRows []gomponents.Node

	// Group fields into rows of 2 for better space utilization
	for i := 0; i < len(fields); i += 2 {
		var rowFields []gomponents.Node

		// First field in row
		if i < len(fields) {
			field := fields[i]

			rowFields = append(rowFields, html.Div(
				html.Class("col-md-6 mb-3"),
				renderFormGroup(
					group,
					comments,
					displayNames,
					field.Name,
					field.Type,
					field.Value,
					convertSelectOptionsToMap(field.Options),
				),
			))
		}

		// Second field in row (if exists)
		if i+1 < len(fields) {
			field := fields[i+1]

			rowFields = append(rowFields, html.Div(
				html.Class("col-md-6 mb-3"),
				renderFormGroup(
					group,
					comments,
					displayNames,
					field.Name,
					field.Type,
					field.Value,
					convertSelectOptionsToMap(field.Options),
				),
			))
		}

		// Create row
		formRows = append(formRows, html.Div(
			html.Class("row"),
			gomponents.Group(rowFields),
		))
	}

	return gomponents.Group(formRows)
}

// renderAlert creates a dismissible alert with icons using optimized createAlert.
func renderAlert(message string, typev string) string {
	return renderComponentToString(createAlert(message, typev))
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

func renderMediaDataForm(prefix string, i int, configv *config.MediaDataConfig) gomponents.Node {
	fields := []FormFieldDefinition{
		{Name: "", Type: "removebutton", Value: "", Options: nil},
		{
			Name:    "TemplatePath",
			Type:    "selectwithpreview",
			Value:   configv.TemplatePath,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("path")),
		},
		{Name: "AddFound", Type: "checkbox", Value: configv.AddFound, Options: nil},
		{Name: "AddFoundList", Type: "text", Value: configv.AddFoundList, Options: nil},
		{Name: "EnableUnpacking", Type: "checkbox", Value: configv.EnableUnpacking, Options: nil},
		{Name: "WriteRenameLog", Type: "checkbox", Value: configv.WriteRenameLog, Options: nil},
		{Name: "EmbedArt", Type: "checkbox", Value: configv.EmbedArt, Options: nil},
		{Name: "EmbedLyrics", Type: "checkbox", Value: configv.EmbedLyrics, Options: nil},
		{
			Name:    "SkipSeriesTrackMatch",
			Type:    "checkbox",
			Value:   configv.SkipSeriesTrackMatch,
			Options: nil,
		},
		{
			Name:    "AllowAlternativeReleases",
			Type:    "checkbox",
			Value:   configv.AllowAlternativeReleases,
			Options: nil,
		},
		{
			Name:    "AllowedReleaseTypes",
			Type:    "array",
			Value:   configv.AllowedReleaseTypes,
			Options: nil,
		},
		{Name: "MBMediaFormats", Type: "array", Value: configv.MBMediaFormats, Options: nil},
		{
			Name:    "AllowAllFormatsWhenStructuring",
			Type:    "checkbox",
			Value:   configv.AllowAllFormatsWhenStructuring,
			Options: nil,
		},
		{
			Name:    "PerTrackToleranceSeconds",
			Type:    "number",
			Value:   configv.PerTrackToleranceSeconds,
			Options: nil,
		},
		{
			Name:    "PerTrackToleranceSecondsMax",
			Type:    "number",
			Value:   configv.PerTrackToleranceSecondsMax,
			Options: nil,
		},
		{
			Name:    "MaxTotalDifferenceSeconds",
			Type:    "number",
			Value:   configv.MaxTotalDifferenceSeconds,
			Options: nil,
		},
		{
			Name:    "AllowMissingTracks",
			Type:    "checkbox",
			Value:   configv.AllowMissingTracks,
			Options: nil,
		},
		{
			Name:    "ExceedToleranceIfTotalMatch",
			Type:    "checkbox",
			Value:   configv.ExceedToleranceIfTotalMatch,
			Options: nil,
		},
		{Name: "TrackTitleWeight", Type: "number", Value: configv.TrackTitleWeight, Options: nil},
		{Name: "TrackIndexWeight", Type: "number", Value: configv.TrackIndexWeight, Options: nil},
		{Name: "TrackLengthWeight", Type: "number", Value: configv.TrackLengthWeight, Options: nil},
		{Name: "TrackArtistWeight", Type: "number", Value: configv.TrackArtistWeight, Options: nil},
		{Name: "TrackIdWeight", Type: "number", Value: configv.TrackIdWeight, Options: nil},
		{
			Name:    "DiscoverSeriesAlbums",
			Type:    "checkbox",
			Value:   configv.DiscoverSeriesAlbums,
			Options: nil,
		},
	}

	return renderArrayItemFormWithIndex(prefix, i, "Data", configv, fields)
}

func renderMediaDataImportForm(
	prefix string,
	i int,
	configv *config.MediaDataImportConfig,
) gomponents.Node {
	fields := []FormFieldDefinition{
		{Name: "", Type: "removebutton", Value: "", Options: nil},
		{
			Name:    "TemplatePath",
			Type:    "selectwithpreview",
			Value:   configv.TemplatePath,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("path")),
		},
		{Name: "EnableUnpacking", Type: "checkbox", Value: configv.EnableUnpacking, Options: nil},
		{Name: "AddFound", Type: "checkbox", Value: configv.AddFound, Options: nil},
		{Name: "AddFoundList", Type: "text", Value: configv.AddFoundList, Options: nil},
		{
			Name:    "AllowAlternativeReleases",
			Type:    "checkbox",
			Value:   configv.AllowAlternativeReleases,
			Options: nil,
		},
		{Name: "MoveUnprocessed", Type: "text", Value: configv.MoveUnprocessed, Options: nil},
		{Name: "EmbedArt", Type: "checkbox", Value: configv.EmbedArt, Options: nil},
		{
			Name:    "AllowAllFormatsWhenStructuring",
			Type:    "checkbox",
			Value:   configv.AllowAllFormatsWhenStructuring,
			Options: nil,
		},
		{
			Name:    "AllowedReleaseTypes",
			Type:    "array",
			Value:   configv.AllowedReleaseTypes,
			Options: nil,
		},
		{Name: "MBMediaFormats", Type: "array", Value: configv.MBMediaFormats, Options: nil},
		{
			Name:    "PerTrackToleranceSeconds",
			Type:    "number",
			Value:   configv.PerTrackToleranceSeconds,
			Options: nil,
		},
		{
			Name:    "PerTrackToleranceSecondsMax",
			Type:    "number",
			Value:   configv.PerTrackToleranceSecondsMax,
			Options: nil,
		},
		{
			Name:    "MaxTotalDifferenceSeconds",
			Type:    "number",
			Value:   configv.MaxTotalDifferenceSeconds,
			Options: nil,
		},
		{
			Name:    "AllowMissingTracks",
			Type:    "checkbox",
			Value:   configv.AllowMissingTracks,
			Options: nil,
		},
		{
			Name:    "ExceedToleranceIfTotalMatch",
			Type:    "checkbox",
			Value:   configv.ExceedToleranceIfTotalMatch,
			Options: nil,
		},
		{Name: "StrongRecThresh", Type: "number", Value: configv.StrongRecThresh, Options: nil},
		{Name: "MediumRecThresh", Type: "number", Value: configv.MediumRecThresh, Options: nil},
		{Name: "TrackTitleWeight", Type: "number", Value: configv.TrackTitleWeight, Options: nil},
		{Name: "TrackIndexWeight", Type: "number", Value: configv.TrackIndexWeight, Options: nil},
		{Name: "TrackLengthWeight", Type: "number", Value: configv.TrackLengthWeight, Options: nil},
		{Name: "TrackArtistWeight", Type: "number", Value: configv.TrackArtistWeight, Options: nil},
		{Name: "TrackIdWeight", Type: "number", Value: configv.TrackIdWeight, Options: nil},
		{
			Name:    "DiscoverSeriesAlbums",
			Type:    "checkbox",
			Value:   configv.DiscoverSeriesAlbums,
			Options: nil,
		},
	}

	return renderArrayItemFormWithIndex(prefix, i, "Data Import", configv, fields)
}

func renderMediaListsForm(prefix string, i int, configv *config.MediaListsConfig) gomponents.Node {
	fields := []FormFieldDefinition{
		{Name: "", Type: "removebutton", Value: "", Options: nil},
		{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
		{
			Name:    "TemplateList",
			Type:    "selectwithpreview",
			Value:   configv.TemplateList,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("list")),
		},
		{
			Name:    "TemplateQuality",
			Type:    "selectwithpreview",
			Value:   configv.TemplateQuality,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("quality")),
		},
		{
			Name:    "TemplateScheduler",
			Type:    "selectwithpreview",
			Value:   configv.TemplateScheduler,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("scheduler")),
		},
		{Name: "IgnoreMapLists", Type: "array", Value: configv.IgnoreMapLists, Options: nil},
		{Name: "ReplaceMapLists", Type: "array", Value: configv.ReplaceMapLists, Options: nil},
		{Name: "Enabled", Type: "checkbox", Value: configv.Enabled, Options: nil},
		{Name: "AddFound", Type: "checkbox", Value: configv.Addfound, Options: nil},
	}

	return renderArrayItemFormWithIndex(prefix, i, "List", configv, fields)
}

func renderMediaNotificationForm(
	prefix string,
	i int,
	configv *config.MediaNotificationConfig,
) gomponents.Node {
	fields := []FormFieldDefinition{
		{Name: "", Type: "removebutton", Value: "", Options: nil},
		{
			Name:    "MapNotification",
			Type:    "selectwithpreview",
			Value:   configv.MapNotification,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("notification")),
		},
		{
			Name:  "Event",
			Type:  "select",
			Value: configv.Event,
			Options: convertMapToSelectOptions(
				map[string][]string{"options": {"added_download", "added_data", "upgraded_data"}},
			),
		},
		{Name: "Title", Type: "text", Value: configv.Title, Options: nil},
		{Name: "Message", Type: "text", Value: configv.Message, Options: nil},
		{Name: "ReplacedPrefix", Type: "text", Value: configv.ReplacedPrefix, Options: nil},
	}

	return renderArrayItemFormWithIndex(prefix, i, "Notification", configv, fields)
}

func renderMediaForm(
	typev string,
	configv *config.MediaTypeConfig,
	csrfToken string,
) gomponents.Node {
	group := "media_" + typev + "_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	collapseId := group + "_main_collapse"

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px;"),
		html.Button(
			html.Class("card-header accordion-button w-100 border-0"),
			html.Style(
				"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;",
			),
			html.Type("button"),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			html.I(html.Class("fas fa-film me-2 text-primary")),
			gomponents.Text("Media "+configv.Name),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// Organized media sections
				renderMediaConfigSections(configv, typev, group, comments, displayNames, csrfToken),
			),
		),
	)
}

// renderMediaConfigSections organizes media fields into logical groups.
func renderMediaConfigSections(
	configv *config.MediaTypeConfig,
	typev, group string,
	comments map[string]string,
	displayNames map[string]string,
	csrfToken string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "mediaConfigAccordion-" + typev + "-" + sanitizedName

	// Prepare sub-arrays
	datav := make([]gomponents.Node, 0, len(configv.Data))
	for i, mediaType := range configv.Data {
		datav = append(datav, renderMediaDataForm(group+"_data", i, &mediaType))
	}

	DataImport := make([]gomponents.Node, 0, len(configv.DataImport))
	for i, mediaType := range configv.DataImport {
		DataImport = append(
			DataImport,
			renderMediaDataImportForm(group+"_dataimport", i, &mediaType),
		)
	}

	Lists := make([]gomponents.Node, 0, len(configv.Lists))
	for i, mediaType := range configv.Lists {
		Lists = append(Lists, renderMediaListsForm(group+"_lists", i, &mediaType))
	}

	Notification := make([]gomponents.Node, 0, len(configv.Notification))
	for i, mediaType := range configv.Notification {
		Notification = append(
			Notification,
			renderMediaNotificationForm(group+"_notification", i, &mediaType),
		)
	}

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-"+typev+"-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{Name: "Naming", Type: "text", Value: configv.Naming, Options: nil},
				{
					Name:    "MetadataLanguage",
					Type:    "text",
					Value:   configv.MetadataLanguage,
					Options: nil,
				},
				{Name: "Structure", Type: "checkbox", Value: configv.Structure, Options: nil},
				{
					Name:  "AudibleRegion",
					Type:  "select",
					Value: configv.AudibleRegion,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"us", "uk", "ca", "au", "de", "fr", "it", "es", "in", "jp"},
					}),
				},
			}, "media_main_"+typev+"_"+configv.Name, comments, displayNames, accordionId),

		// Quality & Templates
		renderConfigGroupWithParent(
			"Quality & Templates",
			"quality-"+typev+"-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "DefaultQuality",
					Type:    "select",
					Value:   configv.DefaultQuality,
					Options: convertMapToSelectOptions(database.GetSettingTemplatesFor("quality")),
				},
				{
					Name:  "DefaultResolution",
					Type:  "select",
					Value: configv.DefaultResolution,
					Options: convertMapToSelectOptions(
						database.GetSettingTemplatesFor("resolution"),
					),
				},
				{
					Name:    "TemplateQuality",
					Type:    "selectwithpreview",
					Value:   configv.TemplateQuality,
					Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("quality")),
				},
				{
					Name:    "TemplateScheduler",
					Type:    "selectwithpreview",
					Value:   configv.TemplateScheduler,
					Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("scheduler")),
				},
			},
			"media_main_"+typev+"_"+configv.Name,
			comments,
			displayNames,
			accordionId,
		),

		// Metadata & Search Settings
		renderConfigGroupWithParent(
			"Metadata & Search",
			"metadata-"+typev+"-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "MetadataTitleLanguages",
					Type:    "array",
					Value:   configv.MetadataTitleLanguages,
					Options: nil,
				},
				{
					Name:    "SearchmissingIncremental",
					Type:    "number",
					Value:   configv.SearchmissingIncremental,
					Options: nil,
				},
				{
					Name:    "SearchupgradeIncremental",
					Type:    "number",
					Value:   configv.SearchupgradeIncremental,
					Options: nil,
				},
			},
			"media_main_"+typev+"_"+configv.Name,
			comments,
			displayNames,
			accordionId,
		),

		// Data Sources
		renderMediaArraySection(
			"Data Sources",
			"data-"+typev+"-"+configv.Name,
			datav,
			"Add Data Source",
			"/api/manage/mediadata/form/"+typev+"/"+configv.Name,
			csrfToken,
			accordionId,
		),

		// Data Import
		renderMediaArraySection(
			"Data Import",
			"import-"+typev+"-"+configv.Name,
			DataImport,
			"Add Data Import",
			"/api/manage/mediaimport/form/"+typev+"/"+configv.Name,
			csrfToken,
			accordionId,
		),

		// Lists
		renderMediaArraySection(
			"Lists",
			"lists-"+typev+"-"+configv.Name,
			Lists,
			"Add List",
			"/api/manage/medialists/form/"+typev+"/"+configv.Name,
			csrfToken,
			accordionId,
		),

		// Notifications
		renderMediaArraySection(
			"Notifications",
			"notifications-"+typev+"-"+configv.Name,
			Notification,
			"Add Notification",
			"/api/manage/medianotification/form/"+typev+"/"+configv.Name,
			csrfToken,
			accordionId,
		),
	)
}

// renderMediaArraySection creates a collapsible section for array-based configurations.
func renderMediaArraySection(
	title, id string,
	items []gomponents.Node,
	addButtonText, addEndpoint, csrfToken, parentAccordion string,
) gomponents.Node {
	collapseClass := "accordion-collapse collapse"

	expanded := len(items) > 0 // Expand if has items
	if expanded {
		collapseClass += " show"
	}

	buttonClass := "accordion-button"
	if !expanded {
		buttonClass += " collapsed"
	}

	return html.Div(
		html.Class("accordion-item"),
		html.Style("border: 1px solid #dee2e6; border-radius: 8px; margin-bottom: 0.5rem;"),
		html.H2(
			html.Class("accordion-header"),
			html.ID("heading"+id),
			html.Button(
				html.Class(buttonClass),
				html.Style(
					"background-color: #fff; border: none; padding: 0.75rem 1rem; font-weight: 600;",
				),
				html.Type("button"),
				gomponents.Attr("data-bs-toggle", "collapse"),
				gomponents.Attr("data-bs-target", "#collapse"+id),
				gomponents.Attr("aria-expanded", fmt.Sprintf("%t", expanded)),
				gomponents.Attr("aria-controls", "collapse"+id),
				html.I(html.Class("fas fa-list me-2 text-muted")),
				gomponents.Text(title),
				html.Span(
					html.Class("badge bg-secondary ms-2"),
					gomponents.Text(fmt.Sprintf("%d", len(items))),
				),
			),
		),
		html.Div(
			html.ID("collapse"+id),
			html.Class(collapseClass),
			gomponents.Attr("aria-labelledby", "heading"+id),
			gomponents.Attr("data-bs-parent", "#"+parentAccordion),
			html.Div(
				html.Class("accordion-body p-3"),
				html.Style("background-color: #fff;"),
				gomponents.If(len(items) > 0,
					html.Div(html.Class("mb-3"), gomponents.Group(items)),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-outline-secondary btn-sm"),
					hx.Post(addEndpoint),
					hx.Target("#collapse"+id+" .accordion-body"),
					hx.Swap("beforeend"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					html.I(html.Class("fas fa-plus me-2")),
					gomponents.Text(addButtonText),
				),
			),
		),
	)
}

// renderMediaConfig renders the media configuration section.
func renderMediaConfig(configv *config.MediaConfig, csrfToken string) gomponents.Node {
	Movies := make([]gomponents.Node, 0, len(configv.Movies))
	for _, mediaType := range configv.Movies {
		Movies = append(Movies, renderMediaForm("movies", &mediaType, csrfToken))
	}

	Series := make([]gomponents.Node, 0, len(configv.Series))
	for _, mediaType := range configv.Series {
		Series = append(Series, renderMediaForm("series", &mediaType, csrfToken))
	}

	Books := make([]gomponents.Node, 0, len(configv.Books))
	for _, mediaType := range configv.Books {
		Books = append(Books, renderMediaForm("books", &mediaType, csrfToken))
	}

	AudioBooks := make([]gomponents.Node, 0, len(configv.AudioBooks))
	for _, mediaType := range configv.AudioBooks {
		AudioBooks = append(AudioBooks, renderMediaForm("audiobooks", &mediaType, csrfToken))
	}

	Music := make([]gomponents.Node, 0, len(configv.Music))
	for _, mediaType := range configv.Music {
		Music = append(Music, renderMediaForm("music", &mediaType, csrfToken))
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-video",
			"Media Configuration",
			"Configure media types, lists, qualities, indexers, and organizational settings for movies, TV series, books, audiobooks, and music.",
		),

		html.Form(
			html.Class("config-form"),

			// Series section
			html.Div(
				html.Class("mb-4"),
				html.H4(gomponents.Text("Series")),
				html.Div(
					html.ID("seriesContainer"),
					gomponents.Group(Series),
					// Series items will be added here dynamically
				),
				createAddButton(
					"Add Series",
					"#seriesContainer",
					"/api/manage/media/form/series",
					csrfToken,
				),
			),

			// Movies section
			html.Div(
				html.Class("mb-4"),
				html.H4(gomponents.Text("Movies")),
				html.Div(
					html.ID("moviesContainer"),
					gomponents.Group(Movies),
					// Movie items will be added here dynamically
				),
				createAddButton(
					"Add Movie",
					"#moviesContainer",
					"/api/manage/media/form/movies",
					csrfToken,
				),
			),

			// Books section
			html.Div(
				html.Class("mb-4"),
				html.H4(gomponents.Text("Books")),
				html.Div(
					html.ID("booksContainer"),
					gomponents.Group(Books),
				),
				createAddButton(
					"Add Book Config",
					"#booksContainer",
					"/api/manage/media/form/books",
					csrfToken,
				),
			),

			// AudioBooks section
			html.Div(
				html.Class("mb-4"),
				html.H4(gomponents.Text("AudioBooks")),
				html.Div(
					html.ID("audiobooksContainer"),
					gomponents.Group(AudioBooks),
				),
				createAddButton(
					"Add AudioBook Config",
					"#audiobooksContainer",
					"/api/manage/media/form/audiobooks",
					csrfToken,
				),
			),

			// Music section
			html.Div(
				html.Class("mb-4"),
				html.H4(gomponents.Text("Music")),
				html.Div(
					html.ID("musicContainer"),
					gomponents.Group(Music),
				),
				createAddButton(
					"Add Music Config",
					"#musicContainer",
					"/api/manage/media/form/music",
					csrfToken,
				),
			),

			// Submit button
			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				"/api/admin/config/media/update",
				csrfToken,
			),
			html.Div(html.ID("addalert")),
		),
	)
}

func renderDownloaderForm(configv *config.DownloaderConfig) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "downloader_" + configv.Name

	return renderOptimizedArrayItemForm("downloader", configv.Name, "Downloader", configv,
		renderDownloaderConfigSections(configv, group, comments, displayNames))
}

// renderDownloaderConfigSections organizes downloader fields into logical groups.
func renderDownloaderConfigSections(
	configv *config.DownloaderConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "downloaderConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-downloader-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "DlType",
					Type:  "select",
					Value: configv.DlType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {
							"drone",
							"nzbget",
							"sabnzbd",
							"transmission",
							"rtorrent",
							"qbittorrent",
							"deluge",
						},
					}),
				},
				{Name: "Enabled", Type: "checkbox", Value: configv.Enabled, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Connection Settings
		renderConfigGroupWithParent(
			"Connection Settings",
			"connection-downloader-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Hostname", Type: "text", Value: configv.Hostname, Options: nil},
				{Name: "Port", Type: "number", Value: configv.Port, Options: nil},
				{Name: "Username", Type: "text", Value: configv.Username, Options: nil},
				{Name: "Password", Type: "password", Value: configv.Password, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Download Settings
		renderConfigGroupWithParent(
			"Download Settings",
			"download-downloader-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "AddPaused", Type: "checkbox", Value: configv.AddPaused, Options: nil},
				{Name: "Priority", Type: "number", Value: configv.Priority, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Deluge-Specific Settings
		renderConfigGroupWithParent(
			"Deluge Settings",
			"deluge-downloader-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "DelugeDlTo", Type: "text", Value: configv.DelugeDlTo, Options: nil},
				{
					Name:    "DelugeMoveAfter",
					Type:    "checkbox",
					Value:   configv.DelugeMoveAfter,
					Options: nil,
				},
				{Name: "DelugeMoveTo", Type: "text", Value: configv.DelugeMoveTo, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// renderOptimizedArrayItemForm creates an optimized array item form with organized sections.
func renderOptimizedArrayItemForm(
	itemType, name, displayName string,
	_ any,
	sectionsContent gomponents.Node,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(name, " ", "-"), "_", "-")
	collapseId := itemType + "_" + sanitizedName + "_collapse"

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px;"),
		html.Button(
			html.Class("card-header accordion-button w-100 border-0"),
			html.Style(
				"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;",
			),
			html.Type("button"),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			html.I(html.Class("fas fa-folder me-2 text-primary")),
			gomponents.Text(displayName+" "+name),
		),
		html.Div(
			html.ID(collapseId),
			html.Class("collapse"),
			html.Div(
				html.Class(ClassCardBody),
				sectionsContent,
			),
		),
	)
}

// renderDownloaderConfig renders the downloader configuration section.
func renderDownloaderConfig(configv []config.DownloaderConfig, csrfToken string) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Downloader Configuration",
		Subtitle:       "Configure download clients and connection settings for automated media acquisition from various sources and protocols.",
		Icon:           "download",
		AddButtonText:  "Add Downloader",
		AddEndpoint:    "/api/manage/downloader/form",
		FormContainer:  "downloaderContainer",
		SubmitEndpoint: "/api/admin/config/downloader/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(config config.DownloaderConfig, _ string) gomponents.Node {
			return renderDownloaderForm(&config)
		},
	)
}

func renderListsForm(configv *config.ListsConfig) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "lists_" + configv.Name

	return renderOptimizedArrayItemForm("lists", configv.Name, "List", configv,
		renderListsConfigSections(configv, group, comments, displayNames))
}

// renderListsConfigSections organizes lists fields into logical groups.
func renderListsConfigSections(
	configv *config.ListsConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "listsConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-lists-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "ListType",
					Type:  "select",
					Value: configv.ListType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {
							"seriesconfig",
							"traktpublicshowlist",
							"imdbcsv",
							"imdbfile",
							"traktpublicmovielist",
							"traktmoviepopular",
							"traktmovieanticipated",
							"tmdbmovieupcoming",
							"traktmovietrending",
							"traktseriepopular",
							"traktserieanticipated",
							"traktserietrending",
							"tmdbmoviepopular",
							"tmdbmovietrending",
							"tmdbmoviediscover",
							"tmdblist",
							"tmdbseriepopular",
							"tmdbserietrending",
							"tmdbshowdiscover",
							"tmdbshowlist",
							"newznabrss",
							"plexwatchlist",
							"jellyfinwatchlist",
							"moviescraper",
							"audiobookconfig",
							"bookconfig",
							"musicconfig",
							"musiccharts",
							"bookbestsellers",
							"lastfmtopartists",
							"irc",
						},
					}),
				},
				{Name: "Enabled", Type: "checkbox", Value: configv.Enabled, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Source Configuration
		renderConfigGroupWithParent(
			"Source Configuration",
			"source-lists-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "URL", Type: "text", Value: configv.URL, Options: nil},
				{Name: "IMDBCSVFile", Type: "text", Value: configv.IMDBCSVFile, Options: nil},
				{
					Name:    "ManualConfigFile",
					Type:    "text",
					Value:   configv.ManualConfigFile,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Trakt Settings
		renderConfigGroupWithParent(
			"Trakt Settings",
			"trakt-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "TraktUsername", Type: "text", Value: configv.TraktUsername, Options: nil},
				{Name: "TraktListName", Type: "text", Value: configv.TraktListName, Options: nil},
				{
					Name:  "TraktListType",
					Type:  "select",
					Value: configv.TraktListType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"movie", "show"},
					}),
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Filter Settings
		renderConfigGroupWithParent(
			"Filter Settings",
			"filter-lists-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Limit", Type: "text", Value: configv.Limit, Options: nil},
				{Name: "MinVotes", Type: "number", Value: configv.MinVotes, Options: nil},
				{Name: "MinRating", Type: "number", Value: configv.MinRating, Options: nil},
				{Name: "Excludegenre", Type: "array", Value: configv.Excludegenre, Options: nil},
				{Name: "Includegenre", Type: "array", Value: configv.Includegenre, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// TMDB Settings
		renderConfigGroupWithParent(
			"TMDB Settings",
			"tmdb-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "TmdbDiscover", Type: "array", Value: configv.TmdbDiscover, Options: nil},
				{Name: "TmdbList", Type: "arrayint", Value: configv.TmdbList, Options: nil},
				{
					Name:    "RemoveFromList",
					Type:    "checkbox",
					Value:   configv.RemoveFromList,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Media Server Settings
		renderConfigGroupWithParent(
			"Media Server Settings",
			"mediaserver-lists-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "PlexServerURL", Type: "text", Value: configv.PlexServerURL, Options: nil},
				{Name: "PlexToken", Type: "text", Value: configv.PlexToken, Options: nil},
				{Name: "PlexUsername", Type: "text", Value: configv.PlexUsername, Options: nil},
				{
					Name:    "JellyfinServerURL",
					Type:    "text",
					Value:   configv.JellyfinServerURL,
					Options: nil,
				},
				{Name: "JellyfinToken", Type: "text", Value: configv.JellyfinToken, Options: nil},
				{
					Name:    "JellyfinUsername",
					Type:    "text",
					Value:   configv.JellyfinUsername,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - Basic Configuration
		renderConfigGroupWithParent(
			"Movie Scraper - Basic Configuration",
			"moviescraper-basic-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:  "MovieScraperType",
					Type:  "select",
					Value: configv.MovieScraperType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"htmlxpath", "csrfapi", "project1service", "algolia"},
					}),
				},
				{
					Name:    "MovieScraperStartURL",
					Type:    "text",
					Value:   configv.MovieScraperStartURL,
					Options: nil,
				},
				{
					Name:    "MovieScraperSiteURL",
					Type:    "text",
					Value:   configv.MovieScraperSiteURL,
					Options: nil,
				},
				{
					Name:    "MovieScraperSiteID",
					Type:    "number",
					Value:   configv.MovieScraperSiteID,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - HTML/XPath Settings
		renderConfigGroupWithParent(
			"Movie Scraper - HTML/XPath Settings",
			"moviescraper-xpath-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "MovieSceneNodeXPath",
					Type:    "text",
					Value:   configv.MovieSceneNodeXPath,
					Options: nil,
				},
				{
					Name:    "MovieTitleXPath",
					Type:    "text",
					Value:   configv.MovieTitleXPath,
					Options: nil,
				},
				{Name: "MovieYearXPath", Type: "text", Value: configv.MovieYearXPath, Options: nil},
				{
					Name:    "MovieImdbIDXPath",
					Type:    "text",
					Value:   configv.MovieImdbIDXPath,
					Options: nil,
				},
				{Name: "MovieURLXPath", Type: "text", Value: configv.MovieURLXPath, Options: nil},
				{
					Name:    "MovieReleaseDateXPath",
					Type:    "text",
					Value:   configv.MovieReleaseDateXPath,
					Options: nil,
				},
				{
					Name:    "MovieGenreXPath",
					Type:    "text",
					Value:   configv.MovieGenreXPath,
					Options: nil,
				},
				{
					Name:    "MovieRatingXPath",
					Type:    "text",
					Value:   configv.MovieRatingXPath,
					Options: nil,
				},
				{
					Name:    "MovieTitleAttribute",
					Type:    "text",
					Value:   configv.MovieTitleAttribute,
					Options: nil,
				},
				{
					Name:    "MovieURLAttribute",
					Type:    "text",
					Value:   configv.MovieURLAttribute,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - Pagination Settings
		renderConfigGroupWithParent(
			"Movie Scraper - Pagination Settings",
			"moviescraper-pagination-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:  "MoviePaginationType",
					Type:  "select",
					Value: configv.MoviePaginationType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"increment", "urlpattern"},
					}),
				},
				{
					Name:    "MoviePageIncrement",
					Type:    "number",
					Value:   configv.MoviePageIncrement,
					Options: nil,
				},
				{
					Name:    "MoviePageURLPattern",
					Type:    "text",
					Value:   configv.MoviePageURLPattern,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - CSRF API Settings
		renderConfigGroupWithParent(
			"Movie Scraper - CSRF API Settings",
			"moviescraper-csrfapi-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "MovieCSRFCookieName",
					Type:    "text",
					Value:   configv.MovieCSRFCookieName,
					Options: nil,
				},
				{
					Name:    "MovieCSRFHeaderName",
					Type:    "text",
					Value:   configv.MovieCSRFHeaderName,
					Options: nil,
				},
				{
					Name:    "MovieAPIURLPattern",
					Type:    "text",
					Value:   configv.MovieAPIURLPattern,
					Options: nil,
				},
				{
					Name:    "MoviePageStartIndex",
					Type:    "number",
					Value:   configv.MoviePageStartIndex,
					Options: nil,
				},
				{
					Name:    "MovieResultsArrayPath",
					Type:    "text",
					Value:   configv.MovieResultsArrayPath,
					Options: nil,
				},
				{
					Name:    "MovieTitleField",
					Type:    "text",
					Value:   configv.MovieTitleField,
					Options: nil,
				},
				{Name: "MovieYearField", Type: "text", Value: configv.MovieYearField, Options: nil},
				{
					Name:    "MovieImdbIDField",
					Type:    "text",
					Value:   configv.MovieImdbIDField,
					Options: nil,
				},
				{Name: "MovieURLField", Type: "text", Value: configv.MovieURLField, Options: nil},
				{
					Name:    "MovieRatingField",
					Type:    "text",
					Value:   configv.MovieRatingField,
					Options: nil,
				},
				{
					Name:    "MovieGenreField",
					Type:    "text",
					Value:   configv.MovieGenreField,
					Options: nil,
				},
				{
					Name:    "MovieReleaseDateField",
					Type:    "text",
					Value:   configv.MovieReleaseDateField,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - Common Settings
		renderConfigGroupWithParent(
			"Movie Scraper - Common Settings",
			"moviescraper-common-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "MovieDateFormat",
					Type:    "text",
					Value:   configv.MovieDateFormat,
					Options: nil,
				},
				{
					Name:    "MovieWaitSeconds",
					Type:    "number",
					Value:   configv.MovieWaitSeconds,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Chart / Bestseller Scraper Settings
		renderConfigGroupWithParent(
			"Chart / Bestseller Scraper Settings",
			"chartscraper-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "ChartEntryNodeXPath",
					Type:    "text",
					Value:   configv.ChartEntryNodeXPath,
					Options: nil,
				},
				{
					Name:    "ChartTitleXPath",
					Type:    "text",
					Value:   configv.ChartTitleXPath,
					Options: nil,
				},
				{
					Name:    "ChartArtistXPath",
					Type:    "text",
					Value:   configv.ChartArtistXPath,
					Options: nil,
				},
				{
					Name:    "ChartTitleAttribute",
					Type:    "text",
					Value:   configv.ChartTitleAttribute,
					Options: nil,
				},
				{
					Name:    "ChartArtistAttribute",
					Type:    "text",
					Value:   configv.ChartArtistAttribute,
					Options: nil,
				},
				{
					Name:    "ChartDefaultArtist",
					Type:    "text",
					Value:   configv.ChartDefaultArtist,
					Options: nil,
				},
				{
					Name:    "ChartDateURLPattern",
					Type:    "text",
					Value:   configv.ChartDateURLPattern,
					Options: nil,
				},
				{
					Name:    "ChartDateFormat",
					Type:    "text",
					Value:   configv.ChartDateFormat,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// IRC Announce Settings
		renderConfigGroupWithParent(
			"IRC Announce Settings",
			"irc-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "IRCServer", Type: "text", Value: configv.IRCServer, Options: nil},
				{Name: "IRCUseTLS", Type: "checkbox", Value: configv.IRCUseTLS, Options: nil},
				{Name: "IRCNick", Type: "text", Value: configv.IRCNick, Options: nil},
				{
					Name:    "IRCNickServPassword",
					Type:    "text",
					Value:   configv.IRCNickServPassword,
					Options: nil,
				},
				{
					Name:    "IRCInviteCommand",
					Type:    "text",
					Value:   configv.IRCInviteCommand,
					Options: nil,
				},
				{Name: "IRCChannels", Type: "array", Value: configv.IRCChannels, Options: nil},
				{
					Name:    "IRCAnnounceNicks",
					Type:    "array",
					Value:   configv.IRCAnnounceNicks,
					Options: nil,
				},
				{
					Name:    "IRCAnnounceRegex",
					Type:    "text",
					Value:   configv.IRCAnnounceRegex,
					Options: nil,
				},
				{
					Name:    "IRCReadSeconds",
					Type:    "number",
					Value:   configv.IRCReadSeconds,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// renderListsConfig renders the lists configuration section.
func renderListsConfig(configv []config.ListsConfig, csrfToken string) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Lists Configuration",
		Subtitle:       "Manage media lists and collections for organizing movies and TV series into custom groups with specific rules and criteria.",
		Icon:           "list",
		AddButtonText:  "Add List",
		AddEndpoint:    "/api/manage/lists/form",
		FormContainer:  "listsContainer",
		SubmitEndpoint: "/api/admin/config/list/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(config config.ListsConfig, _ string) gomponents.Node {
			return renderListsForm(&config)
		},
	)
}

func renderIndexersForm(configv *config.IndexersConfig) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "indexers_" + configv.Name

	return renderOptimizedArrayItemForm("indexers", configv.Name, "Indexer", configv,
		renderIndexersConfigSections(configv, group, comments, displayNames))
}

// renderIndexersConfigSections organizes indexer fields into logical groups.
func renderIndexersConfigSections(
	configv *config.IndexersConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "indexersConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-indexers-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "IndexerType",
					Type:  "select",
					Value: configv.IndexerType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"torznab", "newznab", "torrent", "torrentrss"},
					}),
				},
				{Name: "Enabled", Type: "checkbox", Value: configv.Enabled, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Connection Settings
		renderConfigGroupWithParent(
			"Connection Settings",
			"connection-indexers-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "URL", Type: "text", Value: configv.URL, Options: nil},
				{Name: "Apikey", Type: "password", Value: configv.Apikey, Options: nil},
				{Name: "Userid", Type: "text", Value: configv.Userid, Options: nil},
				{
					Name:    "DisableTLSVerify",
					Type:    "checkbox",
					Value:   configv.DisableTLSVerify,
					Options: nil,
				},
				{
					Name:    "DisableCompression",
					Type:    "checkbox",
					Value:   configv.DisableCompression,
					Options: nil,
				},
				{
					Name:    "TimeoutSeconds",
					Type:    "number",
					Value:   configv.TimeoutSeconds,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// RSS Settings
		renderConfigGroupWithParent(
			"RSS Settings",
			"rss-indexers-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Rssenabled", Type: "checkbox", Value: configv.Rssenabled, Options: nil},
				{
					Name:    "RssEntriesloop",
					Type:    "number",
					Value:   configv.RssEntriesloop,
					Options: nil,
				},
				{Name: "Customrssurl", Type: "text", Value: configv.Customrssurl, Options: nil},
				{
					Name:    "Customrsscategory",
					Type:    "text",
					Value:   configv.Customrsscategory,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Search Settings
		renderConfigGroupWithParent(
			"Search Settings",
			"search-indexers-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "Addquotesfortitlequery",
					Type:    "checkbox",
					Value:   configv.Addquotesfortitlequery,
					Options: nil,
				},
				{Name: "MaxEntries", Type: "number", Value: configv.MaxEntries, Options: nil},
				{Name: "MaxAge", Type: "number", Value: configv.MaxAge, Options: nil},
				{
					Name:    "TrustWithIMDBIDs",
					Type:    "checkbox",
					Value:   configv.TrustWithIMDBIDs,
					Options: nil,
				},
				{
					Name:    "TrustWithTVDBIDs",
					Type:    "checkbox",
					Value:   configv.TrustWithTVDBIDs,
					Options: nil,
				},
				{
					Name:    "CheckTitleOnIDSearch",
					Type:    "checkbox",
					Value:   configv.CheckTitleOnIDSearch,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Rate Limiting
		renderConfigGroupWithParent(
			"Rate Limiting",
			"limits-indexers-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Limitercalls", Type: "number", Value: configv.Limitercalls, Options: nil},
				{
					Name:    "Limiterseconds",
					Type:    "number",
					Value:   configv.Limiterseconds,
					Options: nil,
				},
				{
					Name:    "LimitercallsDaily",
					Type:    "number",
					Value:   configv.LimitercallsDaily,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Custom Settings
		renderConfigGroupWithParent(
			"Custom Settings",
			"custom-indexers-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Customapi", Type: "text", Value: configv.Customapi, Options: nil},
				{Name: "Customurl", Type: "text", Value: configv.Customurl, Options: nil},
				{Name: "OutputAsJSON", Type: "checkbox", Value: configv.OutputAsJSON, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// renderIndexersConfig renders the indexers configuration section.
func renderIndexersConfig(configv []config.IndexersConfig, csrfToken string) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Indexers Configuration",
		Subtitle:       "Configure search indexers and sources for discovering and retrieving media content from various providers and trackers.",
		Icon:           "search",
		AddButtonText:  "Add Indexer",
		AddEndpoint:    "/api/manage/indexers/form",
		FormContainer:  "indexersContainer",
		SubmitEndpoint: "/api/admin/config/indexer/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(config config.IndexersConfig, _ string) gomponents.Node {
			return renderIndexersForm(&config)
		},
	)
}

func renderPathsForm(configv *config.PathsConfig) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "paths_" + configv.Name

	return renderOptimizedArrayItemForm("paths", configv.Name, "Path", configv,
		renderPathsConfigSections(configv, group, comments, displayNames))
}

// renderPathsConfigSections organizes path fields into logical groups.
func renderPathsConfigSections(
	configv *config.PathsConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "pathsConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-paths-"+sanitizedName, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{Name: "Path", Type: "text", Value: configv.Path, Options: nil},
				{Name: "Upgrade", Type: "checkbox", Value: configv.Upgrade, Options: nil},
			}, group, comments, displayNames, accordionId),

		// File Extensions
		renderConfigGroupWithParent("File Extensions", "extensions-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{
					Name:    "AllowedVideoExtensions",
					Type:    "array",
					Value:   configv.AllowedVideoExtensions,
					Options: nil,
				},
				{
					Name:    "AllowedOtherExtensions",
					Type:    "array",
					Value:   configv.AllowedOtherExtensions,
					Options: nil,
				},
				{
					Name:    "AllowedVideoExtensionsNoRename",
					Type:    "array",
					Value:   configv.AllowedVideoExtensionsNoRename,
					Options: nil,
				},
				{
					Name:    "AllowedOtherExtensionsNoRename",
					Type:    "array",
					Value:   configv.AllowedOtherExtensionsNoRename,
					Options: nil,
				},
				{
					Name:    "AllowedAudioExtensions",
					Type:    "array",
					Value:   configv.AllowedAudioExtensions,
					Options: nil,
				},
				{
					Name:    "AllowedAudioExtensionsNoRename",
					Type:    "array",
					Value:   configv.AllowedAudioExtensionsNoRename,
					Options: nil,
				},
				{
					Name:    "AllowedBookExtensions",
					Type:    "array",
					Value:   configv.AllowedBookExtensions,
					Options: nil,
				},
				{
					Name:    "AllowedBookExtensionsNoRename",
					Type:    "array",
					Value:   configv.AllowedBookExtensionsNoRename,
					Options: nil,
				},
				{Name: "Blocked", Type: "array", Value: configv.Blocked, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Size & Language Filtering
		renderConfigGroupWithParent(
			"Size & Language Filtering",
			"filtering-paths-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{Name: "MinSize", Type: "number", Value: configv.MinSize, Options: nil},
				{Name: "MaxSize", Type: "number", Value: configv.MaxSize, Options: nil},
				{Name: "MinVideoSize", Type: "number", Value: configv.MinVideoSize, Options: nil},
				{Name: "CleanupsizeMB", Type: "number", Value: configv.CleanupsizeMB, Options: nil},
				{
					Name:    "AllowedLanguages",
					Type:    "array",
					Value:   configv.AllowedLanguages,
					Options: nil,
				},
				{Name: "Disallowed", Type: "array", Value: configv.Disallowed, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Scanning Settings
		renderConfigGroupWithParent("Scanning Settings", "scanning-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{
					Name:    "UpgradeScanInterval",
					Type:    "number",
					Value:   configv.UpgradeScanInterval,
					Options: nil,
				},
				{
					Name:    "MissingScanInterval",
					Type:    "number",
					Value:   configv.MissingScanInterval,
					Options: nil,
				},
				{
					Name:    "MissingScanReleaseDatePre",
					Type:    "number",
					Value:   configv.MissingScanReleaseDatePre,
					Options: nil,
				},
			}, group, comments, displayNames, accordionId),

		// Quality Control
		renderConfigGroupWithParent("Quality Control", "quality-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "CheckRuntime", Type: "checkbox", Value: configv.CheckRuntime, Options: nil},
				{
					Name:    "MaxRuntimeDifference",
					Type:    "number",
					Value:   configv.MaxRuntimeDifference,
					Options: nil,
				},
				{
					Name:    "DeleteWrongRuntime",
					Type:    "checkbox",
					Value:   configv.DeleteWrongRuntime,
					Options: nil,
				},
				{
					Name:    "DeleteWrongLanguage",
					Type:    "checkbox",
					Value:   configv.DeleteWrongLanguage,
					Options: nil,
				},
				{
					Name:    "DeleteDisallowed",
					Type:    "checkbox",
					Value:   configv.DeleteDisallowed,
					Options: nil,
				},
			}, group, comments, displayNames, accordionId),

		// File Organization
		renderConfigGroupWithParent("File Organization", "organization-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "Replacelower", Type: "checkbox", Value: configv.Replacelower, Options: nil},
				{Name: "Usepresort", Type: "checkbox", Value: configv.Usepresort, Options: nil},
				{
					Name:    "PresortFolderPath",
					Type:    "text",
					Value:   configv.PresortFolderPath,
					Options: nil,
				},
				{Name: "MoveReplaced", Type: "checkbox", Value: configv.MoveReplaced, Options: nil},
				{
					Name:    "MoveReplacedTargetPath",
					Type:    "text",
					Value:   configv.MoveReplacedTargetPath,
					Options: nil,
				},
			}, group, comments, displayNames, accordionId),

		// File Permissions
		renderConfigGroupWithParent("File Permissions", "permissions-paths-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "SetChmod", Type: "text", Value: configv.SetChmod, Options: nil},
				{Name: "SetChmodFolder", Type: "text", Value: configv.SetChmodFolder, Options: nil},
			}, group, comments, displayNames, accordionId),
	)
}

// renderPathsConfig renders the paths configuration section
// renderPathsConfig renders the paths configuration section.
func renderPathsConfig(configv []config.PathsConfig, csrfToken string) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Paths Configuration",
		Subtitle:       "Define file system paths, directory structures, and naming conventions for organizing downloaded media content.",
		Icon:           "folder",
		AddButtonText:  "Add Path",
		AddEndpoint:    "/api/manage/paths/form",
		FormContainer:  "pathsContainer",
		SubmitEndpoint: "/api/admin/config/path/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(config config.PathsConfig, _ string) gomponents.Node {
			return renderPathsForm(&config)
		},
	)
}

func renderNotificationForm(configv *config.NotificationConfig) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "notifications_" + configv.Name

	return renderOptimizedArrayItemForm("notifications", configv.Name, "Notification", configv,
		renderNotificationConfigSections(configv, group, comments, displayNames))
}

// renderNotificationConfigSections organizes notification fields into logical groups.
func renderNotificationConfigSections(
	configv *config.NotificationConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "notificationConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-notification-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "NotificationType",
					Type:  "select",
					Value: configv.NotificationType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"csv", "pushover", "gotify", "pushbullet", "apprise"},
					}),
				},
			}, group, comments, displayNames, accordionId),

		// Configuration
		renderConfigGroupWithParent(
			"Configuration",
			"config-notification-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Apikey", Type: "text", Value: configv.Apikey, Options: nil},
				{Name: "Recipient", Type: "text", Value: configv.Recipient, Options: nil},
				{Name: "Outputto", Type: "text", Value: configv.Outputto, Options: nil},
				{Name: "ServerURL", Type: "text", Value: configv.ServerURL, Options: nil},
				{Name: "AppriseURLs", Type: "text", Value: configv.AppriseURLs, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// renderNotificationConfig renders the notification configuration section.
func renderNotificationConfig(
	configv []config.NotificationConfig,
	csrfToken string,
) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Notification Configuration",
		Subtitle:       "Set up notification systems and alert mechanisms to stay informed about download status, errors, and system events.",
		Icon:           "bell",
		FormContainer:  "notificationContainer",
		AddButtonText:  "Add Notification",
		AddEndpoint:    "/api/manage/notification/form",
		SubmitEndpoint: "/api/admin/config/notification/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(config config.NotificationConfig, _ string) gomponents.Node {
			return renderNotificationForm(&config)
		},
	)
}

func renderRegexForm(configv *config.RegexConfig) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "regex_" + configv.Name

	return renderOptimizedArrayItemForm("regex", configv.Name, "Regex", configv,
		renderRegexConfigSections(configv, group, comments, displayNames))
}

// renderRegexConfigSections organizes regex fields into logical groups.
func renderRegexConfigSections(
	configv *config.RegexConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "regexConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-regex-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Pattern Rules
		renderConfigGroupWithParent(
			"Pattern Rules",
			"patterns-regex-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Required", Type: "array", Value: configv.Required, Options: nil},
				{Name: "Rejected", Type: "array", Value: configv.Rejected, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// renderRegexConfig renders the regex configuration section.
func renderRegexConfig(configv []config.RegexConfig, csrfToken string) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Regex Configuration",
		Subtitle:       "Create and manage regular expression patterns for parsing file names, extracting metadata, and content filtering.",
		Icon:           "code",
		FormContainer:  "regexContainer",
		AddButtonText:  "Add Regex",
		AddEndpoint:    "/api/manage/regex/form",
		SubmitEndpoint: "/api/admin/config/regex/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(config config.RegexConfig, _ string) gomponents.Node {
			return renderRegexForm(&config)
		},
	)
}

func renderQualityReorderForm(
	i int,
	mainname string,
	configv *config.QualityReorderConfig,
) gomponents.Node {
	fields := []FormFieldDefinition{
		{Name: "", Type: "removebutton", Value: "", Options: nil},
		{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
		{
			Name:  "ReorderType",
			Type:  "select",
			Value: configv.ReorderType,
			Options: convertMapToSelectOptions(map[string][]string{
				"options": {
					"resolution",
					"quality",
					"codec",
					"audio",
					"audio_format",
					"position",
					"combined_res_qual",
				},
			}),
		},
		{Name: "Newpriority", Type: "number", Value: configv.Newpriority, Options: nil},
	}

	return renderArrayItemFormWithNameAndIndex(
		"quality",
		mainname+"_reorder",
		i,
		"Reorder",
		configv,
		fields,
	)
}

func renderQualityIndexerForm(
	i int,
	mainname string,
	configv *config.QualityIndexerConfig,
) gomponents.Node {
	fields := []FormFieldDefinition{
		{Name: "", Type: "removebutton", Value: "", Options: nil},
		{
			Name:    "TemplateIndexer",
			Type:    "selectwithpreview",
			Value:   configv.TemplateIndexer,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("indexer")),
		},
		{
			Name:    "TemplateDownloader",
			Type:    "selectwithpreview",
			Value:   configv.TemplateDownloader,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("downloader")),
		},
		{
			Name:    "TemplateRegex",
			Type:    "selectwithpreview",
			Value:   configv.TemplateRegex,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("regex")),
		},
		{
			Name:    "TemplatePathNzb",
			Type:    "selectwithpreview",
			Value:   configv.TemplatePathNzb,
			Options: convertMapToSelectOptions(config.GetSettingTemplatesFor("path")),
		},
		{Name: "CategoryDownloader", Type: "text", Value: configv.CategoryDownloader, Options: nil},
		{
			Name:    "AdditionalQueryParams",
			Type:    "text",
			Value:   configv.AdditionalQueryParams,
			Options: nil,
		},
		{Name: "SkipEmptySize", Type: "checkbox", Value: configv.SkipEmptySize, Options: nil},
		{
			Name:    "HistoryCheckTitle",
			Type:    "checkbox",
			Value:   configv.HistoryCheckTitle,
			Options: nil,
		},
		{Name: "CategoriesIndexer", Type: "text", Value: configv.CategoriesIndexer, Options: nil},
	}

	return renderArrayItemFormWithNameAndIndex(
		"quality",
		mainname+"_indexer",
		i,
		"Indexer",
		configv,
		fields,
	)
}

func renderQualityForm(configv *config.QualityConfig, csrfToken string) gomponents.Node {
	group := "quality_main_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	collapseId := group + "_main_collapse"

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px;"),
		html.Button(
			html.Class("card-header accordion-button w-100 border-0"),
			html.Style(
				"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;",
			),
			html.Type("button"),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			html.I(html.Class("fas fa-star me-2 text-primary")),
			gomponents.Text("Quality "+configv.Name),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// Organized quality sections
				renderQualityConfigSections(configv, group, comments, displayNames, csrfToken),
			),
		),
	)
}

// renderQualityConfigSections organizes quality fields into logical groups.
func renderQualityConfigSections(
	configv *config.QualityConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	csrfToken string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "qualityConfigAccordion-" + sanitizedName

	// Prepare sub-arrays
	QualityReorder := make([]gomponents.Node, 0, len(configv.QualityReorder))
	for i, qualityReorder := range configv.QualityReorder {
		QualityReorder = append(
			QualityReorder,
			renderQualityReorderForm(i, configv.Name, &qualityReorder),
		)
	}

	QualityIndexer := make([]gomponents.Node, 0, len(configv.Indexer))
	for i, qualityIndexer := range configv.Indexer {
		QualityIndexer = append(
			QualityIndexer,
			renderQualityIndexerForm(i, configv.Name, &qualityIndexer),
		)
	}

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-quality-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Quality Preferences
		renderConfigGroupWithParent(
			"Quality Preferences",
			"preferences-quality-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:  "WantedResolution",
					Type:  "arrayselectarray",
					Value: configv.WantedResolution,
					Options: convertMapToSelectOptions(
						database.GetSettingTemplatesFor("resolution"),
					),
				},
				{
					Name:    "WantedQuality",
					Type:    "arrayselectarray",
					Value:   configv.WantedQuality,
					Options: convertMapToSelectOptions(database.GetSettingTemplatesFor("quality")),
				},
				{
					Name:    "WantedAudio",
					Type:    "arrayselectarray",
					Value:   configv.WantedAudio,
					Options: convertMapToSelectOptions(database.GetSettingTemplatesFor("audio")),
				},
				{
					Name:    "WantedCodec",
					Type:    "arrayselectarray",
					Value:   configv.WantedCodec,
					Options: convertMapToSelectOptions(database.GetSettingTemplatesFor("codec")),
				},
				{
					Name:    "WantedAudioFormats",
					Type:    "array",
					Value:   configv.WantedAudioFormats,
					Options: nil,
				},
				{
					Name:    "MinAudioBitrate",
					Type:    "number",
					Value:   configv.MinAudioBitrate,
					Options: nil,
				},
				{
					Name:    "PreferLossless",
					Type:    "checkbox",
					Value:   configv.PreferLossless,
					Options: nil,
				},
				{
					Name:  "CutoffResolution",
					Type:  "arrayselect",
					Value: configv.CutoffResolution,
					Options: convertMapToSelectOptions(
						database.GetSettingTemplatesFor("resolution"),
					),
				},
				{
					Name:    "CutoffQuality",
					Type:    "arrayselect",
					Value:   configv.CutoffQuality,
					Options: convertMapToSelectOptions(database.GetSettingTemplatesFor("quality")),
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Search Settings
		renderConfigGroupWithParent(
			"Search Settings",
			"search-quality-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "SearchForTitleIfEmpty",
					Type:    "checkbox",
					Value:   configv.SearchForTitleIfEmpty,
					Options: nil,
				},
				{
					Name:    "BackupSearchForTitle",
					Type:    "checkbox",
					Value:   configv.BackupSearchForTitle,
					Options: nil,
				},
				{
					Name:    "SearchForAlternateTitleIfEmpty",
					Type:    "checkbox",
					Value:   configv.SearchForAlternateTitleIfEmpty,
					Options: nil,
				},
				{
					Name:    "BackupSearchForAlternateTitle",
					Type:    "checkbox",
					Value:   configv.BackupSearchForAlternateTitle,
					Options: nil,
				},
				{
					Name:    "ExcludeYearFromTitleSearch",
					Type:    "checkbox",
					Value:   configv.ExcludeYearFromTitleSearch,
					Options: nil,
				},
				{
					Name:    "TitleStripSuffixForSearch",
					Type:    "array",
					Value:   configv.TitleStripSuffixForSearch,
					Options: nil,
				},
				{
					Name:    "TitleStripPrefixForSearch",
					Type:    "array",
					Value:   configv.TitleStripPrefixForSearch,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Validation Settings
		renderConfigGroupWithParent(
			"Validation Settings",
			"validation-quality-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "CheckUntilFirstFound",
					Type:    "checkbox",
					Value:   configv.CheckUntilFirstFound,
					Options: nil,
				},
				{Name: "CheckTitle", Type: "checkbox", Value: configv.CheckTitle, Options: nil},
				{
					Name:    "CheckTitleOnIDSearch",
					Type:    "checkbox",
					Value:   configv.CheckTitleOnIDSearch,
					Options: nil,
				},
				{Name: "CheckYear", Type: "checkbox", Value: configv.CheckYear, Options: nil},
				{Name: "CheckYear1", Type: "checkbox", Value: configv.CheckYear1, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Priority Settings
		renderConfigGroupWithParent(
			"Priority Settings",
			"priority-quality-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "UseForPriorityResolution",
					Type:    "checkbox",
					Value:   configv.UseForPriorityResolution,
					Options: nil,
				},
				{
					Name:    "UseForPriorityQuality",
					Type:    "checkbox",
					Value:   configv.UseForPriorityQuality,
					Options: nil,
				},
				{
					Name:    "UseForPriorityAudio",
					Type:    "checkbox",
					Value:   configv.UseForPriorityAudio,
					Options: nil,
				},
				{
					Name:    "UseForPriorityAudioFormat",
					Type:    "checkbox",
					Value:   configv.UseForPriorityAudioFormat,
					Options: nil,
				},
				{
					Name:    "UseForPriorityAudioBitrate",
					Type:    "checkbox",
					Value:   configv.UseForPriorityAudioBitrate,
					Options: nil,
				},
				{
					Name:    "UseForPriorityCodec",
					Type:    "checkbox",
					Value:   configv.UseForPriorityCodec,
					Options: nil,
				},
				{
					Name:    "UseForPriorityOther",
					Type:    "checkbox",
					Value:   configv.UseForPriorityOther,
					Options: nil,
				},
				{
					Name:    "UseForPriorityMinDifference",
					Type:    "number",
					Value:   configv.UseForPriorityMinDifference,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Reorder Rules
		renderMediaArraySection(
			"Reorder Rules",
			"reorder-quality-"+configv.Name,
			QualityReorder,
			"Add Reorder Rule",
			"/api/manage/qualityreorder/form/"+configv.Name,
			csrfToken,
			accordionId,
		),

		// Indexer Settings
		renderMediaArraySection(
			"Indexer Settings",
			"indexer-quality-"+configv.Name,
			QualityIndexer,
			"Add Indexer Setting",
			"/api/manage/qualityindexer/form/"+configv.Name,
			csrfToken,
			accordionId,
		),
	)
}

// renderQualityConfig renders the quality configuration section.
func renderQualityConfig(configv []config.QualityConfig, csrfToken string) gomponents.Node {
	elements := make([]gomponents.Node, 0, len(configv))
	for _, quality := range configv {
		elements = append(elements, renderQualityForm(&quality, csrfToken))
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fas fa-star",
			"Quality Configuration",
			"Configure quality profiles, resolution settings, codec preferences, and indexer priorities for media downloads.",
		),

		html.Form(
			html.Class("config-form"),

			html.Div(
				html.ID("qualityContainer"),
				gomponents.Group(elements),
				// Quality items will be added here dynamically
			),
			createAddButton(
				"Add Quality",
				"#qualityContainer",
				"/api/manage/quality/form",
				csrfToken,
			),
			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				"/api/admin/config/quality/update",
				csrfToken,
			),
			html.Div(html.ID("addalert")),
		),
	)
}

func renderSchedulerForm(configv *config.SchedulerConfig) gomponents.Node {
	group := "scheduler_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	collapseId := group + "_main_collapse"

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px;"),
		html.Button(
			html.Class("card-header accordion-button w-100 border-0"),
			html.Style(
				"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;",
			),
			html.Type("button"),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			html.I(html.Class("fas fa-clock me-2 text-primary")),
			gomponents.Text("Scheduler "+configv.Name),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// Organized scheduler sections
				renderSchedulerConfigSections(configv, group, comments, displayNames),
			),
		),
	)
}

// renderSchedulerConfigSections organizes scheduler fields into logical groups.
func renderSchedulerConfigSections(
	configv *config.SchedulerConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	return html.Div(
		html.Class("accordion"),
		html.ID(
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Basic Settings
		renderConfigGroupWithParent(
			"Basic Settings",
			"basic-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Feed Intervals
		renderConfigGroupWithParent(
			"Feed Scheduling",
			"feeds-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "IntervalFeeds", Type: "text", Value: configv.IntervalFeeds},
				{Name: "CronFeeds", Type: "text", Value: configv.CronFeeds},
				{
					Name:  "IntervalFeedsRefreshSeries",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshSeries,
				},
				{
					Name:  "CronFeedsRefreshSeries",
					Type:  "text",
					Value: configv.CronFeedsRefreshSeries,
				},
				{
					Name:  "IntervalFeedsRefreshMovies",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshMovies,
				},
				{
					Name:  "CronFeedsRefreshMovies",
					Type:  "text",
					Value: configv.CronFeedsRefreshMovies,
				},
				{
					Name:  "IntervalFeedsRefreshSeriesFull",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshSeriesFull,
				},
				{
					Name:  "CronFeedsRefreshSeriesFull",
					Type:  "text",
					Value: configv.CronFeedsRefreshSeriesFull,
				},
				{
					Name:  "IntervalFeedsRefreshMoviesFull",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshMoviesFull,
				},
				{
					Name:  "CronFeedsRefreshMoviesFull",
					Type:  "text",
					Value: configv.CronFeedsRefreshMoviesFull,
				},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Indexer Intervals
		renderConfigGroupWithParent(
			"Indexer Scheduling",
			"indexer-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{
					Name:  "IntervalIndexerMissing",
					Type:  "text",
					Value: configv.IntervalIndexerMissing,
				},
				{Name: "CronIndexerMissing", Type: "text", Value: configv.CronIndexerMissing},
				{
					Name:  "IntervalIndexerUpgrade",
					Type:  "text",
					Value: configv.IntervalIndexerUpgrade,
				},
				{Name: "CronIndexerUpgrade", Type: "text", Value: configv.CronIndexerUpgrade},
				{
					Name:  "IntervalIndexerMissingFull",
					Type:  "text",
					Value: configv.IntervalIndexerMissingFull,
				},
				{
					Name:  "CronIndexerMissingFull",
					Type:  "text",
					Value: configv.CronIndexerMissingFull,
				},
				{
					Name:  "IntervalIndexerUpgradeFull",
					Type:  "text",
					Value: configv.IntervalIndexerUpgradeFull,
				},
				{
					Name:  "CronIndexerUpgradeFull",
					Type:  "text",
					Value: configv.CronIndexerUpgradeFull,
				},
				{Name: "IntervalIndexerRss", Type: "text", Value: configv.IntervalIndexerRss},
				{Name: "CronIndexerRss", Type: "text", Value: configv.CronIndexerRss},
				{
					Name:  "IntervalIndexerRssSeasons",
					Type:  "text",
					Value: configv.IntervalIndexerRssSeasons,
				},
				{Name: "CronIndexerRssSeasons", Type: "text", Value: configv.CronIndexerRssSeasons},
				{
					Name:  "IntervalIndexerRssSeasonsAll",
					Type:  "text",
					Value: configv.IntervalIndexerRssSeasonsAll,
				},
				{
					Name:  "CronIndexerRssSeasonsAll",
					Type:  "text",
					Value: configv.CronIndexerRssSeasonsAll,
				},
				{
					Name:  "IntervalIndexerRssArtists",
					Type:  "text",
					Value: configv.IntervalIndexerRssArtists,
				},
				{
					Name:  "CronIndexerRssArtists",
					Type:  "text",
					Value: configv.CronIndexerRssArtists,
				},
				{
					Name:  "IntervalIndexerRssArtistsUpgrade",
					Type:  "text",
					Value: configv.IntervalIndexerRssArtistsUpgrade,
				},
				{
					Name:  "CronIndexerRssArtistsUpgrade",
					Type:  "text",
					Value: configv.CronIndexerRssArtistsUpgrade,
				},
				{
					Name:  "IntervalIndexerRssAuthors",
					Type:  "text",
					Value: configv.IntervalIndexerRssAuthors,
				},
				{
					Name:  "CronIndexerRssAuthors",
					Type:  "text",
					Value: configv.CronIndexerRssAuthors,
				},
				{
					Name:  "IntervalIndexerRssAuthorsUpgrade",
					Type:  "text",
					Value: configv.IntervalIndexerRssAuthorsUpgrade,
				},
				{
					Name:  "CronIndexerRssAuthorsUpgrade",
					Type:  "text",
					Value: configv.CronIndexerRssAuthorsUpgrade,
				},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Title Search Scheduling
		renderConfigGroupWithParent(
			"Title Search Scheduling",
			"titles-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{
					Name:  "IntervalIndexerMissingTitle",
					Type:  "text",
					Value: configv.IntervalIndexerMissingTitle,
				},
				{
					Name:  "CronIndexerMissingTitle",
					Type:  "text",
					Value: configv.CronIndexerMissingTitle,
				},
				{
					Name:  "IntervalIndexerUpgradeTitle",
					Type:  "text",
					Value: configv.IntervalIndexerUpgradeTitle,
				},
				{
					Name:  "CronIndexerUpgradeTitle",
					Type:  "text",
					Value: configv.CronIndexerUpgradeTitle,
				},
				{
					Name:  "IntervalIndexerMissingFullTitle",
					Type:  "text",
					Value: configv.IntervalIndexerMissingFullTitle,
				},
				{
					Name:  "CronIndexerMissingFullTitle",
					Type:  "text",
					Value: configv.CronIndexerMissingFullTitle,
				},
				{
					Name:  "IntervalIndexerUpgradeFullTitle",
					Type:  "text",
					Value: configv.IntervalIndexerUpgradeFullTitle,
				},
				{
					Name:  "CronIndexerUpgradeFullTitle",
					Type:  "text",
					Value: configv.CronIndexerUpgradeFullTitle,
				},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Data Scanning & Maintenance
		renderConfigGroupWithParent(
			"Data Scanning & Maintenance",
			"maintenance-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "IntervalImdb", Type: "text", Value: configv.IntervalImdb},
				{Name: "CronImdb", Type: "text", Value: configv.CronImdb},
				{Name: "IntervalScanData", Type: "text", Value: configv.IntervalScanData},
				{Name: "CronScanData", Type: "text", Value: configv.CronScanData},
				{
					Name:  "IntervalScanDataMissing",
					Type:  "text",
					Value: configv.IntervalScanDataMissing,
				},
				{Name: "CronScanDataMissing", Type: "text", Value: configv.CronScanDataMissing},
				{Name: "IntervalScanDataFlags", Type: "text", Value: configv.IntervalScanDataFlags},
				{Name: "CronScanDataFlags", Type: "text", Value: configv.CronScanDataFlags},
				{
					Name:  "IntervalScanDataimport",
					Type:  "text",
					Value: configv.IntervalScanDataimport,
				},
				{Name: "CronScanDataimport", Type: "text", Value: configv.CronScanDataimport},
				{
					Name:  "IntervalDatabaseBackup",
					Type:  "text",
					Value: configv.IntervalDatabaseBackup,
				},
				{Name: "CronDatabaseBackup", Type: "text", Value: configv.CronDatabaseBackup},
				{Name: "IntervalDatabaseCheck", Type: "text", Value: configv.IntervalDatabaseCheck},
				{Name: "CronDatabaseCheck", Type: "text", Value: configv.CronDatabaseCheck},
				{Name: "IntervalCacheRefresh", Type: "text", Value: configv.IntervalCacheRefresh},
				{Name: "CronCacheRefresh", Type: "text", Value: configv.CronCacheRefresh},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),
	)
}

// renderSchedulerConfig renders the scheduler configuration section
// renderSchedulerConfig renders the scheduler configuration section.
func renderSchedulerConfig(configv []config.SchedulerConfig, csrfToken string) gomponents.Node {
	options := ConfigSectionOptions{
		SectionTitle:    "Scheduler Configuration",
		SectionSubtitle: "Configure automated scheduling rules and timing for various system tasks, jobs, and maintenance operations.",
		SectionIcon:     "clock",
		ContainerID:     "schedulerContainer",
		AddButtonText:   "Add Scheduler",
		AddFormPath:     "/api/manage/scheduler/form",
		UpdatePath:      "/api/admin/config/scheduler/update",
	}

	return renderConfigSection(configv, csrfToken, options, renderSchedulerForm)
}

// renderFormGroup renders a form group with label and input.
func renderFormGroup(
	group string,
	comments map[string]string,
	displayNames map[string]string,
	name, inputType string,
	value any,
	options map[string][]string,
) gomponents.Node {
	var (
		input     gomponents.Node
		iconClass string
	)

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
	case "floatmap":
		iconClass = "fa-solid fa-sliders"
	default:
		iconClass = "fa-solid fa-cog"
	}

	switch inputType {
	case "removebutton":
		return createRemoveButton(false)
	case "selectarray":
		var optionElements []gomponents.Node
		if opts, ok := options["options"]; ok {
			values := value.([]string)
			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for _, opt := range opts2 {
				selected := slices.Contains(values, opt)
				if selected {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))
				}
			}
		}

		input = html.Select(
			html.Class("form-select selectpicker"),
			html.Multiple(),
			html.Data("live-search", "true"),
			html.ID(group+"_"+name),
			html.Name(group+"_"+name),
			gomponents.Group(optionElements),
		)

	case "select":
		var optionElements []gomponents.Node
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for _, opt := range opts2 {
				selected := opt == value.(string)
				if selected {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))
				}
			}
		}

		input = html.Select(
			html.Class("form-select"),
			html.ID(group+"_"+name),
			html.Name(group+"_"+name),
			gomponents.Group(optionElements),
		)

	case "selectwithpreview":
		// Select with preview link and modal - used for template reference fields
		var optionElements []gomponents.Node
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for _, opt := range opts2 {
				selected := opt == value.(string)
				if selected {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))
				}
			}
		}

		// Get template config info for the link and preview
		templateInfo := getTemplateConfigInfo(name)
		selectID := group + "_" + name
		previewBtnID := selectID + "_preview_btn"

		// Create the select with preview buttons
		input = html.Div(
			html.Class("input-group"),
			html.Select(
				html.Class("form-select"),
				html.ID(selectID),
				html.Name(selectID),
				gomponents.Group(optionElements),
			),
			// Preview button - opens modal with readonly preview
			html.Button(
				html.Type("button"),
				html.Class("btn btn-outline-info"),
				html.ID(previewBtnID),
				html.Title("Preview configuration"),
				gomponents.Attr(
					"onclick",
					fmt.Sprintf(
						"showConfigPreview('%s', document.getElementById('%s').value, '%s')",
						templateInfo.Type,
						selectID,
						name,
					),
				),
				html.I(html.Class("fas fa-eye")),
			),
			// Link button - opens config page in new tab with hash anchor to auto-expand
			html.Button(
				html.Type("button"),
				html.Class("btn btn-outline-secondary"),
				html.Title("Open configuration page"),
				gomponents.Attr(
					"onclick",
					fmt.Sprintf(
						"openConfigPage('%s', document.getElementById('%s').value)",
						templateInfo.URL,
						selectID,
					),
				),
				html.I(html.Class("fas fa-external-link-alt")),
			),
		)

	case "checkbox":
		var addnode gomponents.Node
		switch val := value.(type) {
		case bool:
			if val {
				addnode = html.Checked()
			}
		}

		// Use createFormField for checkbox with custom checkbox-specific styling
		if addnode != nil {
			input = html.Input(
				html.Class("form-check-input-modern"),
				html.Type("checkbox"),
				html.Role("switch"),
				html.ID(group+"_"+name),
				html.Name(group+"_"+name), addnode,
			)
		} else {
			input = html.Input(
				html.Class("form-check-input-modern"),
				html.Type("checkbox"),
				html.Role("switch"),
				html.ID(group+"_"+name),
				html.Name(group+"_"+name),
			)
		}

	case "textarea":
		input = html.Textarea(
			html.Class(ClassFormControl),
			html.ID(group+"_"+name),
			html.Name(group+"_"+name),
			html.Rows("3"),
			html.Value(value.(string)),
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
		input = html.Div(
			html.ID(group+"_"+name+"-container"),
			gomponents.Group(
				func() []gomponents.Node {
					var nodes []gomponents.Node
					for _, v := range value.([]string) {
						nodes = append(nodes, html.Div(
							html.Class(ClassDFlex),
							html.Input(
								html.Class(ClassFormControl+" me-2"),
								html.Type("text"),
								html.Name(group+"_"+name),
								html.Value(v),
							),
							html.Button(
								html.Class(ClassBtnDanger),
								html.Type("button"),
								gomponents.Attr(
									"onclick",
									"if(this.parentElement) this.parentElement.remove()",
								),
								html.I(html.Class("fa fa-trash")),
							),
						))
					}

					return append(nodes,
						html.Button(
							html.Class(ClassBtnPrimary),
							html.Type("button"),
							gomponents.Attr(
								"onclick",
								fmt.Sprintf("addArray%sItem('%s', '%s')", name, group, name),
							),
							gomponents.Text("Add Item"),
						),
						html.Script(gomponents.Rawf(`
							function addArray%sItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<input class="form-control me-2" type="text" name="%s"><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()"><i class="fa fa-trash"></i></button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name)),
					)
				}(),
			),
		)

	case "arrayselectarray":
		var optionString strings.Builder
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for _, opt := range opts2 {
				optionString.WriteString("<option value=\"" + opt + "\">" + opt + "</option>")
			}
		}

		input = html.Div(
			html.ID(group+"_"+name+"-container"),
			gomponents.Group(
				func() []gomponents.Node {
					var nodes []gomponents.Node
					for _, v := range value.([]string) {
						var optionElements []gomponents.Node
						if opts, ok := options["options"]; ok {
							opts2 := sort.StringSlice(opts)
							opts2.Sort()

							for _, opt := range opts2 {
								if opt == v {
									optionElements = append(optionElements, html.Option(
										html.Value(opt),
										gomponents.Text(opt),
										html.Selected(),
									))
								} else {
									optionElements = append(optionElements, html.Option(
										html.Value(opt),
										gomponents.Text(opt),
									))
								}
							}
						}

						nodes = append(nodes, html.Div(
							html.Class(ClassDFlex),
							html.Select(
								html.Class(ClassFormSelect),
								html.Name(group+"_"+name),
								gomponents.Group(optionElements),
							),
							html.Button(
								html.Class(ClassBtnDanger),
								html.Type("button"),
								gomponents.Attr(
									"onclick",
									"if(this.parentElement) this.parentElement.remove()",
								),
								html.I(html.Class("fa fa-trash")),
							),
						))
					}

					return append(nodes,
						html.Button(
							html.Class(ClassBtnPrimary),
							html.Type("button"),
							gomponents.Attr(
								"onclick",
								fmt.Sprintf("addArray%sItem('%s', '%s')", name, group, name),
							),
							gomponents.Text("Add Item"),
						),
						html.Script(gomponents.Rawf(`
							function addArray%sItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<select class="form-select me-2" name="%s">%s</select><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()"><i class="fa fa-trash"></i></button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name, optionString.String())),
					)
				}(),
			),
		)

	case "arrayselect":
		var (
			optionElements []gomponents.Node
			optionString   string
		)

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
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))

					optionString += "<option value=\"" + opt + "\" selected=\"\">" + opt + "</option>"
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))

					optionString += "<option value=\"" + opt + "\">" + opt + "</option>"
				}
			}
		}

		input = html.Div(
			html.ID(group+"_"+name+"-container"),
			html.Div(
				html.Class(ClassDFlex),
				html.Select(
					html.Class(ClassFormSelect),
					html.Name(group+"_"+name),
					gomponents.Group(optionElements),
				),
			),
		)

	case "arrayint":
		input = html.Div(
			html.ID(group+"_"+name+"-container"),
			gomponents.Group(
				func() []gomponents.Node {
					var nodes []gomponents.Node
					for _, v := range value.([]int) {
						nodes = append(nodes, html.Div(
							html.Class(ClassDFlex),
							html.Input(
								html.Class(ClassFormControl+" me-2"),
								html.Type("number"),
								html.Name(group+"_"+name),
								html.Value(strconv.Itoa(v)),
							),
							html.Button(
								html.Class(ClassBtnDanger),
								html.Type("button"),
								gomponents.Attr(
									"onclick",
									"if(this.parentElement) this.parentElement.remove()",
								),
								html.I(html.Class("fa fa-trash")),
							),
						))
					}

					return append(nodes,
						html.Button(
							html.Class(ClassBtnPrimary),
							html.Type("button"),
							gomponents.Attr(
								"onclick",
								fmt.Sprintf("addArray%sIntItem('%s', '%s')", name, group, name),
							),
							gomponents.Text("Add Item"),
						),
						html.Script(gomponents.Rawf(`
							function addArray%sIntItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<input class="form-control me-2" type="number" name="%s"><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()"><i class="fa fa-trash"></i></button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name)),
					)
				}(),
			),
		)

	case "floatmap":
		m, _ := value.(map[string]float64)

		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		input = html.Div(
			html.ID(group+"_"+name+"-container"),
			gomponents.Group(
				func() []gomponents.Node {
					var nodes []gomponents.Node
					for _, k := range keys {
						nodes = append(nodes, html.Div(
							html.Class(ClassDFlex+" mb-2"),
							html.Input(
								html.Class(ClassFormControl+" me-2"),
								html.Type("text"),
								html.Placeholder("source"),
								html.Name(group+"_"+name+"_key"),
								html.Value(k),
							),
							html.Input(
								html.Class(ClassFormControl+" me-2"),
								html.Type("number"),
								gomponents.Attr("step", "any"),
								html.Placeholder("penalty"),
								html.Name(group+"_"+name+"_value"),
								html.Value(strconv.FormatFloat(m[k], 'f', -1, 64)),
							),
							html.Button(
								html.Class(ClassBtnDanger),
								html.Type("button"),
								gomponents.Attr(
									"onclick",
									"if(this.parentElement) this.parentElement.remove()",
								),
								html.I(html.Class("fa fa-trash")),
							),
						))
					}

					return append(nodes,
						html.Button(
							html.Class(ClassBtnPrimary),
							html.Type("button"),
							gomponents.Attr(
								"onclick",
								fmt.Sprintf("addMap%sItem('%s', '%s')", name, group, name),
							),
							gomponents.Text("Add Item"),
						),
						html.Script(gomponents.Rawf(`
							function addMap%sItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<input class="form-control me-2" type="text" placeholder="source" name="%s_key"><input class="form-control me-2" type="number" step="any" placeholder="penalty" name="%s_value"><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()"><i class="fa fa-trash"></i></button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name, group+"_"+name)),
					)
				}(),
			),
		)

	default:
		input = createFormField(inputType, group+"_"+name, "", "", nil)
	}

	// Enhanced form group with compact comment display using tooltips
	var commentNode gomponents.Node
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
			commentNode = html.Div(
				html.Div(
					html.Class("d-flex align-items-center mt-1"),
					html.Small(
						html.Class("text-muted me-2"),
						html.Style("font-size: 0.75em;"),
						gomponents.Text(shortComment),
					),
					html.Button(
						html.Type("button"),
						html.Class("btn btn-outline-info btn-sm"),
						html.Style(
							"font-size: 0.85em; padding: 0.25rem 0.5rem; border-radius: 0.375rem;",
						),
						gomponents.Attr("data-bs-toggle", "collapse"),
						gomponents.Attr("data-bs-target", "#help-"+group+"-"+name),
						gomponents.Attr("aria-expanded", "false"),
						gomponents.Attr("title", "Show detailed help"),
						html.I(html.Class("fas fa-info-circle")),
					),
				),
				html.Div(
					html.ID("help-"+group+"-"+name),
					html.Class("collapse mt-2"),
					html.Div(
						html.Class("alert alert-info"),
						html.Style(
							"font-size: 0.85em; padding: 0.75rem; margin: 0; border-radius: 0.375rem;",
						),
						// html.Strong(gomponents.Text("Help: ")),
						html.Div(html.Class("mt-1"), gomponents.Group(createCommentLines(comment))),
					),
				),
			)
		} else {
			// Short single line comment - display compactly
			commentNode = html.Small(
				html.Class("form-text text-muted"),
				html.Style("font-size: 0.75em; margin-top: 0.25rem;"),
				gomponents.Text(comment),
			)
		}
	}

	// Get the display name from the displayNames map, fallback to fieldNameToUserFriendly
	displayName := displayNames[name]
	if displayName == "" {
		displayName = fieldNameToUserFriendly(name)
	}

	if inputType == "checkbox" {
		return html.Div(
			html.Class("form-group-enhanced mb-4"),
			html.Div(
				html.Class("form-check-wrapper p-3 border rounded-3 bg-light"),
				html.Style(
					"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: 1px solid #dee2e6 !important; transition: all 0.2s ease;",
				),
				html.Div(
					html.Class("form-check form-switch"),
					html.I(html.Class(iconClass+" text-primary me-2")),
					input,
					createFormLabel(group+"_"+name, displayName, true),
				),
				commentNode,
			),
		)
	}

	return html.Div(
		html.Class("form-group mb-2"),
		html.Div(
			html.Class("form-field-compact p-2 border rounded"),
			html.Style(
				"background: #ffffff; border: 1px solid #e3e6ea !important; transition: border-color 0.15s ease-in-out;",
			),
			html.Div(
				html.Class("d-flex align-items-center mb-1"),
				html.I(
					html.Class(iconClass+" text-primary me-2"),
					html.Style("font-size: 0.85em;"),
				),
				createFormLabel(group+"_"+name, displayName, false),
			),
			input,
			commentNode,
		),
	)
}

// renderFormFields renders multiple form fields using FormFieldDefinition array.
func renderFormFields(
	group string,
	comments map[string]string,
	displayNames map[string]string,
	fields []FormFieldDefinition,
) []gomponents.Node {
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		formGroups = append(
			formGroups,
			renderFormGroup(
				group,
				comments,
				displayNames,
				field.Name,
				field.Type,
				field.Value,
				convertSelectOptionsToMap(field.Options),
			),
		)
	}

	return formGroups
}

// renderTestParsePage renders a page for testing string parsing.
func renderTestParsePage(csrfToken string) gomponents.Node {
	media := config.GetSettingsMediaAll()

	lists := make(
		[]string,
		0,
		len(media.Movies)+len(media.Series)+len(media.Music)+len(media.Books)+len(media.AudioBooks),
	)
	for i := range media.Movies {
		lists = append(lists, media.Movies[i].NamePrefix)
	}

	for i := range media.Series {
		lists = append(lists, media.Series[i].NamePrefix)
	}

	for i := range media.Music {
		lists = append(lists, media.Music[i].NamePrefix)
	}

	for i := range media.Books {
		lists = append(lists, media.Books[i].NamePrefix)
	}

	for i := range media.AudioBooks {
		lists = append(lists, media.AudioBooks[i].NamePrefix)
	}

	qualitycfg := config.GetSettingsQualityAll()

	qualities := make([]string, 0, len(qualitycfg))
	for i := range qualitycfg {
		qualities = append(qualities, qualitycfg[i].Name)
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-flask",
			"String Parse Test",
			"Test the file parser with different filename patterns. This tool helps you understand how the parser extracts metadata from filenames.",
		),

		html.Form(
			html.Class("config-form"),
			html.ID("parseTestForm"),

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

			renderHTMXSubmitButton(
				"Parse",
				"parseResults",
				"/api/admin/testparse",
				"parseTestForm",
				csrfToken,
			),
			html.Div(
				html.Class("form-group submit-group"),
				createButton(
					"Reset",
					"button",
					"btn btn-secondary ml-2",
					gomponents.Attr(
						"onclick",
						"document.getElementById('parseTestForm').reset(); document.getElementById('parseResults').innerHTML = '';",
					),
				),
			),
		),

		html.Div(
			html.ID("parseResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// renderParseResults renders the parsing results in a formatted table.
func renderParseResults(
	m *database.ParseInfo,
	originalFilename, configKey, qualityKey string,
) string {
	resultRows := []gomponents.Node{
		// Header information
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Original Filename:"))),
			html.Td(gomponents.Text(originalFilename)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Config Used:"))),
			html.Td(gomponents.Text(configKey)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Quality Used:"))),
			html.Td(gomponents.Text(qualityKey)),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),

		// Basic media information
		html.Tr(html.Td(html.Strong(gomponents.Text("Title:"))), html.Td(gomponents.Text(m.Title))),
		html.Tr(html.Td(html.Strong(gomponents.Text("File:"))), html.Td(gomponents.Text(m.File))),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Year:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Year))),
		),
		html.Tr(html.Td(html.Strong(gomponents.Text("Date:"))), html.Td(gomponents.Text(m.Date))),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Identifier:"))),
			html.Td(gomponents.Text(m.Identifier)),
		),

		// Episode/Season information
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Season:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Season))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Season String:"))),
			html.Td(gomponents.Text(m.SeasonStr)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Episode:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Episode))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Episode String:"))),
			html.Td(gomponents.Text(m.EpisodeStr)),
		),

		// Quality and technical information
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Quality:"))),
			html.Td(gomponents.Text(m.Quality)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Resolution:"))),
			html.Td(gomponents.Text(m.Resolution)),
		),
		html.Tr(html.Td(html.Strong(gomponents.Text("Codec:"))), html.Td(gomponents.Text(m.Codec))),
		html.Tr(html.Td(html.Strong(gomponents.Text("Audio:"))), html.Td(gomponents.Text(m.Audio))),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Width x Height:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d x %d", m.Width, m.Height))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Runtime:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d min", m.Runtime))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Languages:"))),
			html.Td(gomponents.Text(strings.Join(m.Languages, ", "))),
		),

		// Flags and properties
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Extended:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%t", m.Extended))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Proper:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%t", m.Proper))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Repack:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%t", m.Repack))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Priority:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Priority))),
		),

		// External IDs
		html.Tr(
			html.Td(html.Strong(gomponents.Text("IMDB ID:"))),
			html.Td(gomponents.Text(m.Imdb)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("TVDB ID:"))),
			html.Td(gomponents.Text(m.Tvdb)),
		),

		// Internal information
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("List ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.ListID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("First IDX:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.FirstIDX))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("First Year IDX:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.FirstYearIDX))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Temp ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.TempID))),
		),

		// Database IDs
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Quality ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.QualityID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Resolution ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.ResolutionID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Codec ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.CodecID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Audio ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.AudioID))),
		),

		// Movie IDs
		html.Tr(
			html.Td(html.Strong(gomponents.Text("DB Movie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.DbmovieID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Movie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.MovieID))),
		),

		// Series IDs
		html.Tr(
			html.Td(html.Strong(gomponents.Text("DB Serie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.DbserieID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("DB Serie Episode ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.DbserieEpisodeID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Serie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.SerieID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Serie Episode ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.SerieEpisodeID))),
		),

		// Internal strings (if not empty)
		func() gomponents.Node {
			if m.Str != "" {
				return html.Tr(
					html.Td(html.Strong(gomponents.Text("Internal String:"))),
					html.Td(gomponents.Text(m.Str)),
				)
			}

			return gomponents.Text("")
		}(),
		func() gomponents.Node {
			if m.RuntimeStr != "" {
				return html.Tr(
					html.Td(html.Strong(gomponents.Text("Runtime String:"))),
					html.Td(gomponents.Text(m.RuntimeStr)),
				)
			}

			return gomponents.Text("")
		}(),
		func() gomponents.Node {
			if m.TempTitle != "" {
				return html.Tr(
					html.Td(html.Strong(gomponents.Text("Temp Title:"))),
					html.Td(gomponents.Text(m.TempTitle)),
				)
			}

			return gomponents.Text("")
		}(),
	}

	results := html.Div(
		html.Class("card border-0 shadow-sm border-success mb-4"),
		html.Div(
			html.Class("card-header border-0"),
			html.Style(
				"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
			),
			html.Div(
				html.Class("d-flex align-items-center justify-content-between"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-success me-3"),
						html.I(html.Class("fas fa-code me-1")),
						gomponents.Text("Parse"),
					),
					html.H5(
						html.Class("card-title mb-0 text-success fw-bold"),
						gomponents.Text("Parse Results"),
					),
				),
				html.Span(
					html.Class("badge bg-success"),
					gomponents.Text(fmt.Sprintf("%d", len(resultRows))),
				),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			func() gomponents.Node {
				if len(resultRows) == 0 {
					return html.Div(
						html.Class("text-center p-5"),
						html.I(
							html.Class("fas fa-code mb-3"),
							html.Style("font-size: 3rem; color: #28a745; opacity: 0.3;"),
						),
						html.H5(html.Class("text-muted mb-2"), gomponents.Text("No Parse Results")),
						html.P(
							html.Class("text-muted small mb-0"),
							gomponents.Text("No parsing operations have been performed yet."),
						),
					)
				}

				return html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(resultRows)),
				)
			}(),
		),
	)

	return renderComponentToString(results)
}

// renderComponentToString renders a gomponents Node to a string using pooled string builder.
func renderComponentToString(node gomponents.Node) string {
	sb := getStringBuilder()
	defer putStringBuilder(sb)

	node.Render(sb)

	return sb.String()
}

// renderTraktAuthPage renders a page for Trakt authentication.
func renderTraktAuthPage(csrfToken string) gomponents.Node {
	// Check current token status
	token := apiexternal.GetTraktToken()
	hasValidToken := token != nil && token.AccessToken != ""

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-key",
			"Trakt Authentication",
			"Authenticate with Trakt.tv to enable synchronized watchlists, ratings, and recommendations. This uses OAuth2 for secure authentication.",
		),

		// Current status display
		html.Div(
			func() gomponents.Node {
				if hasValidToken {
					return html.Class("card border-0 shadow-sm")
				}

				return html.Class("card border-0 shadow-sm border-warning")
			}(),
			html.Div(
				html.Class("card-body"),
				html.H5(
					html.Class("card-title fw-bold mb-3"),
					gomponents.Text("Current Authentication Status"),
				),
				html.Div(html.Class("d-flex align-items-center mb-3"), func() gomponents.Node {
					if hasValidToken {
						return html.Div(
							html.Span(
								html.Class("badge bg-success me-2"),
								html.I(html.Class("fas fa-check me-1")),
								gomponents.Text("Authenticated"),
							),
							html.Span(
								html.Class("text-success"),
								gomponents.Text("Trakt API access is enabled"),
							),
						)
					}

					return html.Div(
						html.Span(
							html.Class("badge bg-danger me-2"),
							html.I(html.Class("fas fa-times me-1")),
							gomponents.Text("Not Authenticated"),
						),
						html.Span(
							html.Class("text-danger"),
							gomponents.Text("Trakt API access is disabled"),
						),
					)
				}()),
				func() gomponents.Node {
					if hasValidToken {
						return html.Div(
							html.Class("mt-3"),
							html.H6(html.Class("fw-bold mb-2"), gomponents.Text("Token Details")),
							html.Ul(html.Class("list-unstyled"),
								html.Li(
									html.Class("mb-1"),
									html.Span(
										html.Class("badge bg-secondary me-2"),
										html.I(html.Class("fas fa-key me-1")),
										gomponents.Text("Access Token"),
									),
									html.Code(
										html.Class("text-muted"),
										gomponents.Text(token.AccessToken[:20]+"..."),
									),
								),
								html.Li(
									html.Class("mb-1"),
									html.Span(
										html.Class("badge bg-info me-2"),
										html.I(html.Class("fas fa-tag me-1")),
										gomponents.Text("Type"),
									),
									gomponents.Text(token.TokenType),
								),
								html.Li(
									html.Class("mb-0"),
									html.Span(
										html.Class("badge bg-warning me-2"),
										html.I(html.Class("fas fa-clock me-1")),
										gomponents.Text("Expiry"),
									),
									gomponents.Text(func() string {
										if token.Expiry.IsZero() {
											return "Never expires"
										}

										return token.Expiry.Format("2006-01-02 15:04:05")
									}()),
								),
							),
						)
					}

					return gomponents.Text("")
				}(),
			),
		),

		// Authentication form
		html.Form(
			html.Class("config-form"),
			html.ID("traktAuthForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Step 1: Get Authorization URL"),
					),
					html.P(
						gomponents.Text(
							"Click the button below to generate an authorization URL. This will open Trakt.tv in a new tab where you can authorize this application.",
						),
					),

					html.Button(
						html.Class("btn btn-info"),
						gomponents.Text("Get Trakt Authorization URL"),
						html.Type("button"),
						hx.Target("#authUrlResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"get_url\"}"),
					),
					html.Div(
						html.ID("authUrlResult"),
						html.Class("mt-3"),
						html.Style("min-height: 30px;"),
					),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Step 2: Enter Authorization Code"),
					),
					html.P(
						gomponents.Text(
							"After authorizing on Trakt.tv, you'll receive a code. Enter it below to complete the authentication process.",
						),
					),

					renderFormGroup("trakt", map[string]string{
						"AuthCode": "Enter the authorization code from Trakt.tv",
					}, map[string]string{
						"AuthCode": "Authorization Code",
					}, "AuthCode", "text", "", nil),

					html.Button(
						html.Class(ClassBtnSuccess),
						gomponents.Text("Store Trakt Token"),
						html.Type("button"),
						hx.Target("#tokenResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Include("#traktAuthForm"),
						hx.Vals("{\"action\": \"store_token\"}"),
					),
					html.Div(
						html.ID("tokenResult"),
						html.Class("mt-3"),
						html.Style("min-height: 30px;"),
					),
				),
			),
		),

		// Token management section
		func() gomponents.Node {
			if hasValidToken {
				return html.Div(
					html.Class("mt-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Token Management")),
					html.Div(
						html.Class("btn-group"),
						html.Button(
							html.Class("btn btn-warning"),
							gomponents.Text("Refresh Token"),
							html.Type("button"),
							hx.Target("#refreshResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"refresh_token\"}"),
						),
						html.Button(
							html.Class("btn btn-danger ml-2"),
							gomponents.Text("Revoke Token"),
							html.Type("button"),
							hx.Target("#revokeResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"revoke_token\"}"),
							hx.Confirm(
								"Revoke the Trakt authentication? This will disable Trakt API access.",
							),
						),
					),
					html.Div(
						html.ID("refreshResult"),
						html.Class("mt-3"),
					),
					html.Div(
						html.ID("revokeResult"),
						html.Class("mt-3"),
					),
				)
			}

			return gomponents.Text("")
		}(),

		// API Test section
		func() gomponents.Node {
			if hasValidToken {
				return html.Div(
					html.Class("mt-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("API Test")),
					html.P(
						gomponents.Text(
							"Test the Trakt API connection by fetching popular movies:",
						),
					),
					html.Button(
						html.Class(ClassBtnSecondary),
						gomponents.Text("Test Trakt API"),
						html.Type("button"),
						hx.Target("#apiTestResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"test_api\"}"),
					),
					html.Div(
						html.ID("apiTestResult"),
						html.Class("mt-3"),
						html.Style("min-height: 30px;"),
					),
				)
			}

			return gomponents.Text("")
		}(),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-primary mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #cfe2ff 0%, #b6d7ff 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-primary me-3"),
						html.I(html.Class("fas fa-cog me-1")),
						gomponents.Text("Setup"),
					),
					html.H5(
						html.Class("card-title mb-0 text-primary fw-bold"),
						gomponents.Text("Setup Instructions"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Follow these steps to set up Trakt authentication"),
				),
				html.Ol(
					html.Class("mb-0 list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"1. Make sure your Trakt Client ID and Client Secret are configured in the General settings",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"2. Click 'Get Trakt Authorization URL' to generate the authorization link",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"3. Visit the generated URL and authorize this application on Trakt.tv",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("4. Copy the authorization code from the redirect URL"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"5. Paste the code in the form above and click 'Store Trakt Token'",
						),
					),
					html.Li(
						html.Class("mb-0"),
						gomponents.Text(
							"6. Your Trakt authentication is now complete and will be stored securely",
						),
					),
				),
			),
		),
	)
}
