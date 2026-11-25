package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderPathsForm renders one row; idx is the row index used in the form field names.
func renderPathsForm(idx int, configv *config.PathsConfig, csrfToken string) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "paths_" + strconv.Itoa(idx)

	return renderOptimizedArrayItemForm("paths", configv.Name, "Path", configv,
		renderPathsConfigSections(configv, group, comments, displayNames, csrfToken))
}

// renderPathsConfigSections organizes path fields into logical groups.
func renderPathsConfigSections(
	configv *config.PathsConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	csrfToken string,
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
				{
					Name:    "",
					Type:    "custom",
					Value:   renderPathCheckControl(csrfToken, group+"_Path"),
					Options: nil,
				},
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
					Name:    "DetectAudioLanguage",
					Type:    "checkbox",
					Value:   configv.DetectAudioLanguage,
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
		func(i int, config config.PathsConfig, itemCsrfToken string) gomponents.Node {
			return renderPathsForm(i, &config, itemCsrfToken)
		},
	)
}
