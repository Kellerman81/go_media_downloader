package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

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
			Name:    "AllowExtraTracks",
			Type:    "checkbox",
			Value:   configv.AllowExtraTracks,
			Options: nil,
		},
		{
			Name:    "AllowCountMismatchIfTotalMatch",
			Type:    "checkbox",
			Value:   configv.AllowCountMismatchIfTotalMatch,
			Options: nil,
		},
		{
			Name:    "ReplaceEmbeddedChapters",
			Type:    "checkbox",
			Value:   configv.ReplaceEmbeddedChapters,
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
			Name:    "AllowExtraTracks",
			Type:    "checkbox",
			Value:   configv.AllowExtraTracks,
			Options: nil,
		},
		{
			Name:    "AllowCountMismatchIfTotalMatch",
			Type:    "checkbox",
			Value:   configv.AllowCountMismatchIfTotalMatch,
			Options: nil,
		},
		{
			Name:    "ReplaceEmbeddedChapters",
			Type:    "checkbox",
			Value:   configv.ReplaceEmbeddedChapters,
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
		{Name: "Addfound", Type: "checkbox", Value: configv.Addfound, Options: nil},
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
				map[string][]string{"options": providers.NotificationEvents},
			),
		},
		{Name: "Title", Type: "text", Value: configv.Title, Options: nil},
		{Name: "Message", Type: "text", Value: configv.Message, Options: nil},
		{Name: "ReplacedPrefix", Type: "text", Value: configv.ReplacedPrefix, Options: nil},
	}

	return renderArrayItemFormWithIndex(prefix, i, "Notification", configv, fields)
}

// renderMediaForm renders one media entry; idx is the row index used in the
// form field names.
func renderMediaForm(
	typev string,
	idx int,
	configv *config.MediaTypeConfig,
	csrfToken string,
) gomponents.Node {
	group := "media_" + typev + "_" + strconv.Itoa(idx)
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
				renderMediaConfigSections(
					configv,
					typev,
					strconv.Itoa(idx),
					group,
					comments,
					displayNames,
					csrfToken,
				),
			),
		),
	)
}

// renderMediaConfigSections organizes media fields into logical groups.
func renderMediaConfigSections(
	configv *config.MediaTypeConfig,
	typev, idx, group string,
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
					Type:    "language",
					Value:   configv.MetadataLanguage,
					Options: nil,
				},
				{Name: "Structure", Type: "checkbox", Value: configv.Structure, Options: nil},
				{
					Name:  "AudibleRegion",
					Type:  "select",
					Value: configv.AudibleRegion,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"", "us", "uk", "ca", "au", "de", "fr", "it", "es", "in", "jp"},
					}),
				},
			}, "media_main_"+typev+"_"+idx, comments, displayNames, accordionId),

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
			"media_main_"+typev+"_"+idx,
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
			"media_main_"+typev+"_"+idx,
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
			"/api/manage/mediadata/form/"+typev+"/"+idx,
			csrfToken,
			accordionId,
		),

		// Data Import
		renderMediaArraySection(
			"Data Import",
			"import-"+typev+"-"+configv.Name,
			DataImport,
			"Add Data Import",
			"/api/manage/mediaimport/form/"+typev+"/"+idx,
			csrfToken,
			accordionId,
		),

		// Lists
		renderMediaArraySection(
			"Lists",
			"lists-"+typev+"-"+configv.Name,
			Lists,
			"Add List",
			"/api/manage/medialists/form/"+typev+"/"+idx,
			csrfToken,
			accordionId,
		),

		// Notifications
		renderMediaArraySection(
			"Notifications",
			"notifications-"+typev+"-"+configv.Name,
			Notification,
			"Add Notification",
			"/api/manage/medianotification/form/"+typev+"/"+idx,
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
	for i, mediaType := range configv.Movies {
		Movies = append(Movies, renderMediaForm("movies", i, &mediaType, csrfToken))
	}

	Series := make([]gomponents.Node, 0, len(configv.Series))
	for i, mediaType := range configv.Series {
		Series = append(Series, renderMediaForm("series", i, &mediaType, csrfToken))
	}

	Books := make([]gomponents.Node, 0, len(configv.Books))
	for i, mediaType := range configv.Books {
		Books = append(Books, renderMediaForm("books", i, &mediaType, csrfToken))
	}

	AudioBooks := make([]gomponents.Node, 0, len(configv.AudioBooks))
	for i, mediaType := range configv.AudioBooks {
		AudioBooks = append(AudioBooks, renderMediaForm("audiobooks", i, &mediaType, csrfToken))
	}

	Music := make([]gomponents.Node, 0, len(configv.Music))
	for i, mediaType := range configv.Music {
		Music = append(Music, renderMediaForm("music", i, &mediaType, csrfToken))
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
