package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderDownloaderForm renders one row; idx is the row index used in the form field names.
func renderDownloaderForm(idx int, configv *config.DownloaderConfig, csrfToken string) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "downloader_" + strconv.Itoa(idx)

	return renderOptimizedArrayItemForm("downloader", configv.Name, "Downloader", configv,
		renderDownloaderConfigSections(configv, group, comments, displayNames, csrfToken))
}

// downloaderTestFields are the fields HandleTestDownloaderConnection reads,
// by name suffix within the current row.
var downloaderTestFields = []connTestField{
	{Key: "dltype", Suffix: "DlType"},
	{Key: "hostname", Suffix: "Hostname"},
	{Key: "port", Suffix: "Port"},
	{Key: "username", Suffix: "Username"},
	{Key: "password", Suffix: "Password"},
}

// renderDownloaderConfigSections organizes downloader fields into logical groups.
func renderDownloaderConfigSections(
	configv *config.DownloaderConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	csrfToken string,
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
				{
					Name:    "",
					Type:    "custom",
					Value:   renderTemplateSelect("Template", downloaderTemplates),
					Options: nil,
				},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "DlType",
					Type:  "select",
					Value: configv.DlType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": providers.DownloadClientTypeNames(),
					}),
				},
				{
					Name:  "",
					Type:  "custom",
					Value: renderConnectionTestControl(csrfToken, "/api/admin/downloader/test", downloaderTestFields),
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
				{Name: "RemoveFailed", Type: "checkbox", Value: configv.RemoveFailed, Options: nil},
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
		func(i int, config config.DownloaderConfig, _ string) gomponents.Node {
			return renderDownloaderForm(i, &config, csrfToken)
		},
	)
}
