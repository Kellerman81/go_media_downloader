package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

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

// renderQualityForm renders one row; idx is the row index used in the form field names.
func renderQualityForm(idx int, configv *config.QualityConfig, csrfToken string) gomponents.Node {
	group := "quality_main_" + strconv.Itoa(idx)
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
				renderQualityConfigSections(idx, configv, group, comments, displayNames, csrfToken),
			),
		),
	)
}

// renderQualityConfigSections organizes quality fields into logical groups.
func renderQualityConfigSections(
	idx int,
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
			renderQualityReorderForm(i, strconv.Itoa(idx), &qualityReorder),
		)
	}

	QualityIndexer := make([]gomponents.Node, 0, len(configv.Indexer))
	for i, qualityIndexer := range configv.Indexer {
		QualityIndexer = append(
			QualityIndexer,
			renderQualityIndexerForm(i, strconv.Itoa(idx), &qualityIndexer),
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

		// Reorder Rules, with the priority editor that writes them.
		html.Div(html.Class("d-flex justify-content-end mb-1"),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-primary"),
				html.Title("Drag resolutions, sources, codecs and audio into the wanted order; the editor writes the reorder rules"),
				gomponents.Attr("onclick", "openPriorityEditor("+jsString(configv.Name)+", function () { refreshReorderRules("+jsString(configv.Name)+"); })"),
				html.I(html.Class("fa-solid fa-arrow-down-wide-short me-1")), gomponents.Text("Priority editor"))),
		renderMediaArraySection(
			"Reorder Rules",
			"reorder-quality-"+configv.Name,
			QualityReorder,
			"Add Reorder Rule",
			"/api/manage/qualityreorder/form/"+strconv.Itoa(idx),
			csrfToken,
			accordionId,
		),

		// Indexer Settings
		renderMediaArraySection(
			"Indexer Settings",
			"indexer-quality-"+configv.Name,
			QualityIndexer,
			"Add Indexer Setting",
			"/api/manage/qualityindexer/form/"+strconv.Itoa(idx),
			csrfToken,
			accordionId,
		),
	)
}

// renderQualityConfig renders the quality configuration section.
func renderQualityConfig(configv []config.QualityConfig, csrfToken string) gomponents.Node {
	elements := make([]gomponents.Node, 0, len(configv))
	for i, quality := range configv {
		elements = append(elements, renderQualityForm(i, &quality, csrfToken))
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
