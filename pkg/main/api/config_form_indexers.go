package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderIndexersForm renders one row; idx is the row index used in the form field names.
func renderIndexersForm(idx int, configv *config.IndexersConfig, csrfToken string) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "indexers_" + strconv.Itoa(idx)

	return renderOptimizedArrayItemForm("indexers", configv.Name, "Indexer", configv,
		renderIndexersConfigSections(configv, group, comments, displayNames, csrfToken))
}

// indexerTestFields are the fields HandleTestIndexerConnection reads, by
// name suffix within the current row.
var indexerTestFields = []connTestField{
	{Key: "url", Suffix: "URL"},
	{Key: "apikey", Suffix: "Apikey"},
	{Key: "customapi", Suffix: "Customapi"},
	{Key: "customurl", Suffix: "Customurl"},
}

// renderIndexersConfigSections organizes indexer fields into logical groups.
func renderIndexersConfigSections(
	configv *config.IndexersConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	csrfToken string,
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
				{
					Name:    "",
					Type:    "custom",
					Value:   renderTemplateSelect("Template", indexerTemplates),
					Options: nil,
				},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "IndexerType",
					Type:  "select",
					Value: configv.IndexerType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"torznab", "newznab", "torrent", "torrentrss"},
					}),
				},
				{
					Name:  "",
					Type:  "custom",
					Value: renderConnectionTestControl(csrfToken, "/api/admin/indexers/test", indexerTestFields),
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
		func(i int, config config.IndexersConfig, _ string) gomponents.Node {
			return renderIndexersForm(i, &config, csrfToken)
		},
	)
}
