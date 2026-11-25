package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderRegexForm renders one row; idx is the row index used in the form field names.
func renderRegexForm(idx int, configv *config.RegexConfig) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "regex_" + strconv.Itoa(idx)

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
		func(i int, config config.RegexConfig, _ string) gomponents.Node {
			return renderRegexForm(i, &config)
		},
	)
}
