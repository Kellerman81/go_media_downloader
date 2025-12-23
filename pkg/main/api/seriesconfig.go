package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderSeriesConfigPage renders the series configuration file editor page
func renderSeriesConfigPage(ctx *gin.Context) {
	// Get selected file from query param
	selectedFile := ctx.Query("file")

	// Get list of series config files
	files, err := getSeriesConfigFiles()
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error listing config files: "+err.Error())
		return
	}

	// Load the selected file's config if specified
	var seriesConfig *config.MainSerieConfig
	if selectedFile != "" {
		seriesConfig, err = loadSeriesConfigFile(selectedFile)
		if err != nil {
			sendJSONError(ctx, http.StatusInternalServerError, "Error loading config file: "+err.Error())
			return
		}
	}

	csrfToken := getCSRFToken(ctx)

	pageNode := page(
		"Series Configuration Files",
		true,
		false,
		false,
		renderSeriesConfigContent(files, selectedFile, seriesConfig, csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// getSeriesConfigFiles returns a list of series config .toml files
func getSeriesConfigFiles() ([]string, error) {
	configDir := config.GetConfigDir()

	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Look for series config files - files ending in .toml but exclude the main config
		// Common patterns: series.toml, series_*.toml, etc.
		if strings.HasSuffix(name, ".toml") && name != "config.toml" {
			// Try to validate it's actually a series config by attempting to load it
			_, err := loadSeriesConfigFile(name)
			if err == nil {
				files = append(files, name)
			}
		}
	}

	return files, nil
}

// loadSeriesConfigFile loads a MainSerieConfig from the specified file
func loadSeriesConfigFile(filename string) (*config.MainSerieConfig, error) {
	configDir := config.GetConfigDir()
	filePath := filepath.Join(configDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg config.MainSerieConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// saveSeriesConfigFile saves a MainSerieConfig to the specified file
func saveSeriesConfigFile(filename string, cfg *config.MainSerieConfig) error {
	configDir := config.GetConfigDir()
	filePath := filepath.Join(configDir, filename)

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0o644)
}

// renderSeriesConfigContent renders the main content for series config page
func renderSeriesConfigContent(files []string, selectedFile string, seriesConfig *config.MainSerieConfig, csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("container-fluid p-4"),

		// Header
		html.Div(
			html.Class("row mb-4"),
			html.Div(
				html.Class("col-12"),
				html.H1(html.Class("h3 mb-3"), gomponents.Text("Series Configuration Files")),
				html.P(html.Class("text-muted"),
					gomponents.Text("Select a series configuration file to view and edit TV series settings."),
				),
			),
		),

		// File selector
		html.Div(
			html.Class("row mb-4"),
			html.Div(
				html.Class("col-md-6"),
				html.Div(
					html.Class("card"),
					html.Div(
						html.Class("card-body"),
						html.H5(html.Class("card-title"), gomponents.Text("Select Configuration File")),
						html.Form(
							html.Method("GET"),
							html.Action("/api/admin/seriesconfig"),
							html.Div(
								html.Class("mb-3"),
								html.Label(html.Class("form-label"), html.For("file-select"), gomponents.Text("Configuration File")),
								html.Select(
									html.Class("form-select"),
									html.ID("file-select"),
									html.Name("file"),
									gomponents.Attr("onchange", "this.form.submit()"),
									html.Option(html.Value(""), gomponents.Text("-- Select a file --")),
									renderFileOptions(files, selectedFile),
								),
							),
						),
					),
				),
			),
		),

		// Series config editor (shown if file is selected)
		gomponents.If(selectedFile != "" && seriesConfig != nil,
			renderSeriesConfigEditor(selectedFile, seriesConfig, csrfToken),
		),
	)
}

// renderFileOptions renders option elements for file select dropdown
func renderFileOptions(files []string, selectedFile string) gomponents.Node {
	var options []gomponents.Node
	for _, file := range files {
		options = append(options, html.Option(
			html.Value(file),
			gomponents.If(file == selectedFile, html.Selected()),
			gomponents.Text(file),
		))
	}
	return gomponents.Group(options)
}

// renderSeriesConfigEditor renders the editor for series entries
func renderSeriesConfigEditor(filename string, cfg *config.MainSerieConfig, csrfToken string) gomponents.Node {
	// Safety check for nil config
	if cfg == nil {
		return html.Div(
			html.Class("alert alert-warning"),
			gomponents.Text("Error: Could not load configuration file."),
		)
	}

	return html.Div(
		html.Class("row"),
		html.Div(
			html.Class("col-12"),
			html.Div(
				html.Class("card"),
				html.Div(
					html.Class("card-header"),
					html.H5(html.Class("card-title mb-0"),
						html.I(html.Class("fa-solid fa-tv me-2")),
						gomponents.Textf("Editing: %s", filename),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(html.Class("text-muted mb-3"),
						gomponents.Textf("This file contains %d TV series configuration(s).", len(cfg.Serie)),
					),

					// Add new series button
					html.Div(
						html.Class("mb-3"),
						html.Button(
							html.Class("btn btn-primary"),
							html.Type("button"),
							gomponents.Attr("onclick", fmt.Sprintf("window.location.href='/api/admin/seriesconfig/add?file=%s'", filename)),
							html.I(html.Class("fa-solid fa-plus me-2")),
							gomponents.Text("Add New Series"),
						),
					),

					// Series list
					gomponents.If(len(cfg.Serie) > 0,
						renderSeriesList(filename, cfg.Serie),
					),
				),
			),
		),
	)
}

// renderSeriesList renders the list of series configurations
func renderSeriesList(filename string, series []config.SerieConfig) gomponents.Node {
	return html.Div(
		html.Class("table-responsive"),
		html.Table(
			html.Class("table table-hover"),
			html.THead(
				html.Tr(
					html.Th(gomponents.Text("Name")),
					html.Th(gomponents.Text("TVDB ID")),
					html.Th(gomponents.Text("Source")),
					html.Th(gomponents.Text("Identified By")),
					html.Th(gomponents.Text("Search")),
					html.Th(gomponents.Text("Actions")),
				),
			),
			html.TBody(
				renderSeriesRows(filename, series),
			),
		),
	)
}

// renderSeriesRows renders table rows for each series
func renderSeriesRows(filename string, series []config.SerieConfig) gomponents.Node {
	var rows []gomponents.Node
	for i, s := range series {
		searchStatus := "Enabled"
		if s.DontSearch {
			searchStatus = "Disabled"
		}

		rows = append(rows, html.Tr(
			html.Td(gomponents.Text(s.Name)),
			html.Td(gomponents.Textf("%d", s.TvdbID)),
			html.Td(gomponents.Text(s.Source)),
			html.Td(gomponents.Text(s.Identifiedby)),
			html.Td(gomponents.Text(searchStatus)),
			html.Td(
				html.A(
					html.Class("btn btn-sm btn-primary me-2"),
					html.Href(fmt.Sprintf("/api/admin/seriesconfig/edit?file=%s&index=%d", filename, i)),
					html.I(html.Class("fa-solid fa-edit")),
				),
				html.A(
					html.Class("btn btn-sm btn-danger"),
					html.Href(fmt.Sprintf("/api/admin/seriesconfig/delete?file=%s&index=%d", filename, i)),
					gomponents.Attr("onclick", "return confirm('Are you sure you want to delete this series configuration?')"),
					html.I(html.Class("fa-solid fa-trash")),
				),
			),
		))
	}
	return gomponents.Group(rows)
}

// handleSeriesConfigDelete handles deletion of a series entry
func handleSeriesConfigDelete(ctx *gin.Context) {
	filename := ctx.Query("file")
	index := ctx.Query("index")

	if filename == "" || index == "" {
		sendBadRequest(ctx, "Missing file or index parameter")
		return
	}

	// Parse index
	var idx int
	if _, err := fmt.Sscanf(index, "%d", &idx); err != nil {
		sendBadRequest(ctx, "Invalid index: "+err.Error())
		return
	}

	// Load config
	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	// Validate index
	if idx < 0 || idx >= len(cfg.Serie) {
		sendBadRequest(ctx, "Index out of range")
		return
	}

	// Remove entry
	cfg.Serie = append(cfg.Serie[:idx], cfg.Serie[idx+1:]...)

	// Save
	if err := saveSeriesConfigFile(filename, cfg); err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error saving config: "+err.Error())
		return
	}

	// Redirect back
	ctx.Redirect(http.StatusFound, "/api/admin/seriesconfig?file="+filename)
}

// handleSeriesConfigEdit handles the edit form page
func handleSeriesConfigEdit(ctx *gin.Context) {
	filename := ctx.Query("file")
	index := ctx.Query("index")

	if filename == "" || index == "" {
		sendBadRequest(ctx, "Missing file or index parameter")
		return
	}

	// Parse index
	var idx int
	if _, err := fmt.Sscanf(index, "%d", &idx); err != nil {
		sendBadRequest(ctx, "Invalid index: "+err.Error())
		return
	}

	// Load config
	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	// Validate index
	if idx < 0 || idx >= len(cfg.Serie) {
		sendBadRequest(ctx, "Index out of range")
		return
	}

	csrfToken := getCSRFToken(ctx)

	pageNode := page(
		"Edit Series Configuration",
		true,
		false,
		false,
		renderSeriesConfigForm(filename, &cfg.Serie[idx], idx, csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// handleSeriesConfigAdd handles the add form page
func handleSeriesConfigAdd(ctx *gin.Context) {
	filename := ctx.Query("file")

	if filename == "" {
		sendBadRequest(ctx, "Missing file parameter")
		return
	}

	csrfToken := getCSRFToken(ctx)

	// Create empty serie config
	emptySerie := &config.SerieConfig{
		Identifiedby: "ep",
		Source:       "tvdb",
	}

	pageNode := page(
		"Add Series Configuration",
		true,
		false,
		false,
		renderSeriesConfigForm(filename, emptySerie, -1, csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// renderSeriesConfigForm renders the add/edit form for a series configuration
func renderSeriesConfigForm(filename string, serie *config.SerieConfig, index int, csrfToken string) gomponents.Node {
	isEdit := index >= 0
	title := "Add New Series"
	if isEdit {
		title = "Edit Series: " + serie.Name
	}

	// Get field metadata
	comments, displayNames := getSerieConfigMetadata()
	group := "serie"

	return html.Div(
		html.Class("container-fluid p-4"),
		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.H1(html.Class("h3 mb-3"), gomponents.Text(title)),
				html.Div(
					html.Class("card"),
					html.Div(
						html.Class("card-body"),
						html.Form(
							html.Method("POST"),
							html.Action("/api/admin/seriesconfig/update"),
							html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
							html.Input(html.Type("hidden"), html.Name("file"), html.Value(filename)),
							gomponents.If(isEdit,
								html.Input(html.Type("hidden"), html.Name("index"), html.Value(fmt.Sprintf("%d", index))),
							),

							// Render form sections
							renderSeriesConfigSections(serie, group, comments, displayNames),

							// Submit buttons
							html.Div(
								html.Class("mt-4"),
								html.Button(
									html.Class("btn btn-primary me-2"),
									html.Type("submit"),
									html.I(html.Class("fa-solid fa-save me-2")),
									gomponents.Text("Save"),
								),
								html.A(
									html.Class("btn btn-info me-2"),
									html.Href(fmt.Sprintf("/api/admin/seriesconfig/helper?file=%s&index=%d", filename, index)),
									html.I(html.Class("fa-solid fa-question-circle me-2")),
									gomponents.Text("Scraper Helper"),
								),
								html.A(
									html.Class("btn btn-secondary"),
									html.Href("/api/admin/seriesconfig?file="+filename),
									gomponents.Text("Cancel"),
								),
							),
						),
					),
				),
			),
		),
	)
}

// getSerieConfigMetadata extracts field comments and display names from SerieConfig struct tags
func getSerieConfigMetadata() (map[string]string, map[string]string) {
	comments := logger.GetFieldComments(config.SerieConfig{})
	displayNames := logger.GetFieldDisplayNames(config.SerieConfig{})
	return comments, displayNames
}

// renderSeriesConfigSections organizes series config fields into logical groups
func renderSeriesConfigSections(
	serie *config.SerieConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(serie.Name, " ", "-"), "_", "-")
	if sanitizedName == "" {
		sanitizedName = "new-series"
	}
	accordionId := "serieConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-serie-"+sanitizedName, true,
			[]FormFieldDefinition{
				{Name: "Name", Type: "text", Value: serie.Name, Options: nil},
				{Name: "TvdbID", Type: "number", Value: serie.TvdbID, Options: nil},
				{Name: "AlternateName", Type: "array", Value: serie.AlternateName, Options: nil},
				{Name: "DisallowedName", Type: "array", Value: serie.DisallowedName, Options: nil},
				{Name: "Identifiedby", Type: "select", Value: serie.Identifiedby, Options: convertMapToSelectOptions(map[string][]string{
					"options": {"ep", "date"},
				})},
				{Name: "Source", Type: "select", Value: serie.Source, Options: convertMapToSelectOptions(map[string][]string{
					"options": {"tvdb", "scraper", "none"},
				})},
				{Name: "Target", Type: "text", Value: serie.Target, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Search & Upgrade Settings
		renderConfigGroupWithParent("Search & Upgrade Settings", "search-serie-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "DontUpgrade", Type: "checkbox", Value: serie.DontUpgrade, Options: nil},
				{Name: "DontSearch", Type: "checkbox", Value: serie.DontSearch, Options: nil},
				{Name: "SearchSpecials", Type: "checkbox", Value: serie.SearchSpecials, Options: nil},
				{Name: "IgnoreRuntime", Type: "checkbox", Value: serie.IgnoreRuntime, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Scraper Settings (when source = "scraper")
		renderConfigGroupWithParent("Scraper - Basic Configuration", "scraper-basic-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "ScraperType", Type: "select", Value: serie.ScraperType, Options: convertMapToSelectOptions(map[string][]string{
					"options": {"project1service", "algolia", "htmlxpath", "csrfapi"},
				})},
				{Name: "StartURL", Type: "text", Value: serie.StartURL, Options: nil},
				{Name: "SiteURL", Type: "text", Value: serie.SiteURL, Options: nil},
				{Name: "SiteID", Type: "number", Value: serie.SiteID, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Project1Service / Algolia Settings
		renderConfigGroupWithParent("Scraper - Filter Settings", "scraper-filters-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "FilterCollectionID", Type: "number", Value: serie.FilterCollectionID, Options: nil},
				{Name: "SiteFilterName", Type: "text", Value: serie.SiteFilterName, Options: nil},
				{Name: "SerieFilterName", Type: "text", Value: serie.SerieFilterName, Options: nil},
				{Name: "NetworkFilterName", Type: "text", Value: serie.NetworkFilterName, Options: nil},
				{Name: "NetworkSiteFilterName", Type: "text", Value: serie.NetworkSiteFilterName, Options: nil},
			}, group, comments, displayNames, accordionId),

		// HTML/XPath Scraper Settings
		renderConfigGroupWithParent("Scraper - HTML/XPath Settings", "scraper-xpath-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "SceneNodeXPath", Type: "text", Value: serie.SceneNodeXPath, Options: nil},
				{Name: "TitleXPath", Type: "text", Value: serie.TitleXPath, Options: nil},
				{Name: "URLXPath", Type: "text", Value: serie.URLXPath, Options: nil},
				{Name: "DateXPath", Type: "text", Value: serie.DateXPath, Options: nil},
				{Name: "ActorsXPath", Type: "text", Value: serie.ActorsXPath, Options: nil},
				{Name: "TitleAttribute", Type: "text", Value: serie.TitleAttribute, Options: nil},
				{Name: "URLAttribute", Type: "text", Value: serie.URLAttribute, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Pagination Settings
		renderConfigGroupWithParent("Scraper - Pagination Settings", "scraper-pagination-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "PaginationType", Type: "select", Value: serie.PaginationType, Options: convertMapToSelectOptions(map[string][]string{
					"options": {"sequential", "offset"},
				})},
				{Name: "PageIncrement", Type: "number", Value: serie.PageIncrement, Options: nil},
				{Name: "PageURLPattern", Type: "text", Value: serie.PageURLPattern, Options: nil},
			}, group, comments, displayNames, accordionId),

		// CSRF API Scraper Settings
		renderConfigGroupWithParent("Scraper - CSRF API Settings", "scraper-csrfapi-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "CSRFCookieName", Type: "text", Value: serie.CSRFCookieName, Options: nil},
				{Name: "CSRFHeaderName", Type: "text", Value: serie.CSRFHeaderName, Options: nil},
				{Name: "APIURLPattern", Type: "text", Value: serie.APIURLPattern, Options: nil},
				{Name: "PageStartIndex", Type: "number", Value: serie.PageStartIndex, Options: nil},
				{Name: "ResultsArrayPath", Type: "text", Value: serie.ResultsArrayPath, Options: nil},
				{Name: "TitleField", Type: "text", Value: serie.TitleField, Options: nil},
				{Name: "DateField", Type: "text", Value: serie.DateField, Options: nil},
				{Name: "URLField", Type: "text", Value: serie.URLField, Options: nil},
				{Name: "ActorsField", Type: "text", Value: serie.ActorsField, Options: nil},
				{Name: "ActorNameField", Type: "text", Value: serie.ActorNameField, Options: nil},
				{Name: "RuntimeField", Type: "text", Value: serie.RuntimeField, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Common Scraper Settings
		renderConfigGroupWithParent("Scraper - Common Settings", "scraper-common-"+sanitizedName, false,
			[]FormFieldDefinition{
				{Name: "DateFormat", Type: "text", Value: serie.DateFormat, Options: nil},
				{Name: "WaitSeconds", Type: "number", Value: serie.WaitSeconds, Options: nil},
			}, group, comments, displayNames, accordionId),
	)
}

// handleSeriesConfigUpdate handles form submission for add/edit
func handleSeriesConfigUpdate(ctx *gin.Context) {
	filename := ctx.PostForm("file")
	indexStr := ctx.PostForm("index")

	if filename == "" {
		sendBadRequest(ctx, "Missing file parameter")
		return
	}

	// Load existing config
	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	// Parse form data into SerieConfig
	serie := parseSerieConfigForm(ctx)

	// Determine if this is an add or edit
	if indexStr != "" {
		// Edit existing entry
		var idx int
		if _, err := fmt.Sscanf(indexStr, "%d", &idx); err != nil {
			sendBadRequest(ctx, "Invalid index: "+err.Error())
			return
		}

		if idx < 0 || idx >= len(cfg.Serie) {
			sendBadRequest(ctx, "Index out of range")
			return
		}

		cfg.Serie[idx] = serie
	} else {
		// Add new entry
		cfg.Serie = append(cfg.Serie, serie)
	}

	// Save config
	if err := saveSeriesConfigFile(filename, cfg); err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error saving config: "+err.Error())
		return
	}

	// Redirect back to main page
	ctx.Redirect(http.StatusFound, "/api/admin/seriesconfig?file="+filename)
}

// parseSerieConfigForm parses form data into a SerieConfig struct
func parseSerieConfigForm(ctx *gin.Context) config.SerieConfig {
	serie := config.SerieConfig{}

	// Basic fields
	serie.Name = ctx.PostForm("serie_Name")

	if tvdbID := ctx.PostForm("serie_TvdbID"); tvdbID != "" {
		fmt.Sscanf(tvdbID, "%d", &serie.TvdbID)
	}

	// Parse arrays (comma-separated values)
	if altNames := ctx.PostForm("serie_AlternateName"); altNames != "" {
		serie.AlternateName = parseArrayField(altNames)
	}
	if disallowed := ctx.PostForm("serie_DisallowedName"); disallowed != "" {
		serie.DisallowedName = parseArrayField(disallowed)
	}

	serie.Identifiedby = ctx.PostForm("serie_Identifiedby")
	serie.Source = ctx.PostForm("serie_Source")
	serie.Target = ctx.PostForm("serie_Target")

	// Checkboxes (only present if checked)
	serie.DontUpgrade = ctx.PostForm("serie_DontUpgrade") == "on"
	serie.DontSearch = ctx.PostForm("serie_DontSearch") == "on"
	serie.SearchSpecials = ctx.PostForm("serie_SearchSpecials") == "on"
	serie.IgnoreRuntime = ctx.PostForm("serie_IgnoreRuntime") == "on"

	// Scraper settings
	serie.ScraperType = ctx.PostForm("serie_ScraperType")
	serie.StartURL = ctx.PostForm("serie_StartURL")
	serie.SiteURL = ctx.PostForm("serie_SiteURL")

	if siteID := ctx.PostForm("serie_SiteID"); siteID != "" {
		var id uint
		fmt.Sscanf(siteID, "%d", &id)
		serie.SiteID = id
	}

	if filterID := ctx.PostForm("serie_FilterCollectionID"); filterID != "" {
		fmt.Sscanf(filterID, "%d", &serie.FilterCollectionID)
	}

	serie.SiteFilterName = ctx.PostForm("serie_SiteFilterName")
	serie.SerieFilterName = ctx.PostForm("serie_SerieFilterName")
	serie.NetworkFilterName = ctx.PostForm("serie_NetworkFilterName")
	serie.NetworkSiteFilterName = ctx.PostForm("serie_NetworkSiteFilterName")

	// XPath fields
	serie.SceneNodeXPath = ctx.PostForm("serie_SceneNodeXPath")
	serie.TitleXPath = ctx.PostForm("serie_TitleXPath")
	serie.URLXPath = ctx.PostForm("serie_URLXPath")
	serie.DateXPath = ctx.PostForm("serie_DateXPath")
	serie.ActorsXPath = ctx.PostForm("serie_ActorsXPath")
	serie.TitleAttribute = ctx.PostForm("serie_TitleAttribute")
	serie.URLAttribute = ctx.PostForm("serie_URLAttribute")

	// Pagination
	serie.PaginationType = ctx.PostForm("serie_PaginationType")
	if pageInc := ctx.PostForm("serie_PageIncrement"); pageInc != "" {
		fmt.Sscanf(pageInc, "%d", &serie.PageIncrement)
	}
	serie.PageURLPattern = ctx.PostForm("serie_PageURLPattern")

	// CSRF API fields
	serie.CSRFCookieName = ctx.PostForm("serie_CSRFCookieName")
	serie.CSRFHeaderName = ctx.PostForm("serie_CSRFHeaderName")
	serie.APIURLPattern = ctx.PostForm("serie_APIURLPattern")

	if startIdx := ctx.PostForm("serie_PageStartIndex"); startIdx != "" {
		fmt.Sscanf(startIdx, "%d", &serie.PageStartIndex)
	}

	serie.ResultsArrayPath = ctx.PostForm("serie_ResultsArrayPath")
	serie.TitleField = ctx.PostForm("serie_TitleField")
	serie.DateField = ctx.PostForm("serie_DateField")
	serie.URLField = ctx.PostForm("serie_URLField")
	serie.ActorsField = ctx.PostForm("serie_ActorsField")
	serie.ActorNameField = ctx.PostForm("serie_ActorNameField")
	serie.RuntimeField = ctx.PostForm("serie_RuntimeField")

	// Common settings
	serie.DateFormat = ctx.PostForm("serie_DateFormat")
	if waitSec := ctx.PostForm("serie_WaitSeconds"); waitSec != "" {
		fmt.Sscanf(waitSec, "%d", &serie.WaitSeconds)
	}

	return serie
}

// parseArrayField parses a comma-separated string into a string slice
func parseArrayField(value string) []string {
	if value == "" {
		return nil
	}

	// Split by comma and trim whitespace
	parts := strings.Split(value, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// handleScraperHelper handles the scraper helper/discovery page
func handleScraperHelper(ctx *gin.Context) {
	filename := ctx.Query("file")
	index := ctx.Query("index")
	scraperType := ctx.Query("type")
	startURL := ctx.Query("url")

	if filename == "" || index == "" {
		sendBadRequest(ctx, "Missing file or index parameter")
		return
	}

	// Auto-detect scraper type if URL is provided but type is not
	if startURL != "" && scraperType == "" {
		scraperType = detectScraperType(startURL)
	}

	// Fetch data if URL is provided
	var discoveredData *ScraperDiscoveryData
	if startURL != "" && scraperType != "" {
		discoveredData = discoverScraperConfig(scraperType, startURL)
	}

	csrfToken := getCSRFToken(ctx)

	pageNode := page(
		"Scraper Helper",
		true,
		false,
		false,
		renderScraperHelperPage(filename, index, scraperType, startURL, discoveredData, csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// ScraperDiscoveryData holds discovered configuration values
type ScraperDiscoveryData struct {
	Error              string
	AlgoliaAppID       string
	AlgoliaAPIKey      string
	AlgoliaIndexName   string
	AvailableSites     []FacetValue
	AvailableSeries    []FacetValue
	AvailableNetworks0 []FacetValue
	AvailableNetworks1 []FacetValue
	AlgoliaFacets      map[string][]FacetValue
	AlgoliaIndexFields []string
	Collections        []CollectionInfo
	CSRFCookie         string
	CSRFHeader         string
	SampleResponse     string
}

// FacetValue represents a facet value with count
type FacetValue struct {
	Value string
	Count int
}

// CollectionInfo holds collection information
type CollectionInfo struct {
	ID   int
	Name string
}

// detectScraperType automatically detects the scraper type by analyzing the URL and page content
func detectScraperType(startURL string) string {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // Allow redirects
		},
	}

	req, err := http.NewRequest(http.MethodGet, startURL, nil)
	if err != nil {
		return "" // Can't detect, let user choose
	}

	resp, err := client.Do(req)
	if err != nil {
		return "" // Can't detect, let user choose
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "" // Can't detect, let user choose
	}

	content := string(body)

	// Parse URL for domain-based detection
	parsedURL, _ := url.Parse(startURL)
	domain := ""
	if parsedURL != nil {
		domain = strings.ToLower(parsedURL.Host)
	}

	// Priority 1: Check for Algolia credentials in page
	// Algolia is used by Gamma Entertainment/Adult Time network
	algoliaPattern1 := regexp.MustCompile(`"applicationID":"([^"]+)","apiKey":"([^"]+)"`)
	algoliaPattern2 := regexp.MustCompile(`algoliaApplicationId:"([^"]+)",algoliaApiKey:"([^"]+)"`)
	if algoliaPattern1.MatchString(content) || algoliaPattern2.MatchString(content) {
		return "algolia"
	}

	// Priority 2: Check for Project1Service (Aylo/MindGeek network)
	// Look for instance_token cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "instance_token" {
			return "project1service"
		}
	}

	// Priority 3: Check for CSRF API patterns
	// Look for CSRF cookies and check if page makes API calls
	hasCSRFCookie := false
	for _, cookie := range resp.Cookies() {
		cookieName := strings.ToLower(cookie.Name)
		if strings.Contains(cookieName, "csrf") || strings.Contains(cookieName, "xsrf") {
			hasCSRFCookie = true
			break
		}
	}

	// If has CSRF cookie and contains API endpoints in JavaScript, likely CSRF API
	if hasCSRFCookie {
		apiPatterns := []string{"/api/", "fetch(", "axios", "XMLHttpRequest"}
		for _, pattern := range apiPatterns {
			if strings.Contains(content, pattern) {
				return "csrfapi"
			}
		}
	}

	// Priority 4: Check for known domain patterns
	knownDomains := map[string]string{
		// Algolia-based sites (Gamma/Adult Time network)
		"girlsway.com":       "algolia",
		"puretaboo.com":      "algolia",
		"burningangel.com":   "algolia",
		"devilsfilm.com":     "algolia",
		"fantasymassage.com": "algolia",
		"allgirlmassage.com": "algolia",
		"nurumassage.com":    "algolia",
		"transsensual.com":   "algolia",
		"adulttime.com":      "algolia",

		// Project1Service sites (Aylo/MindGeek)
		"brazzers.com":          "project1service",
		"realitykings.com":      "project1service",
		"mofos.com":             "project1service",
		"twistys.com":           "project1service",
		"babes.com":             "project1service",
		"digitalplayground.com": "project1service",
		"fakehub.com":           "project1service",
	}

	for knownDomain, scraperType := range knownDomains {
		if strings.Contains(domain, knownDomain) {
			return scraperType
		}
	}

	// Priority 5: If none of above, default to htmlxpath as it's most universal
	return "htmlxpath"
}

// discoverScraperConfig discovers configuration values by analyzing the target URL
func discoverScraperConfig(scraperType, startURL string) *ScraperDiscoveryData {
	data := &ScraperDiscoveryData{}

	switch scraperType {
	case "algolia":
		discoverAlgoliaConfig(startURL, data)
	case "project1service":
		discoverProject1ServiceConfig(startURL, data)
	case "csrfapi":
		discoverCSRFAPIConfig(startURL, data)
	case "htmlxpath":
		// For htmlxpath, we don't auto-discover, just return empty data
		// User needs to manually inspect the page
		data.Error = "" // No error, just show the help guide
	default:
		data.Error = "Scraper type not supported for automatic discovery"
	}

	return data
}

func getAlgoliaCredentials(resp *http.Response) (string, string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)

	// Try pattern 1: "applicationID":"...","apiKey":"..."
	re1 := regexp.MustCompile(
		`"applicationID":"([a-zA-Z0-9]{1,})","apiKey":"([a-zA-Z0-9=,\.]{1,})"`,
	)
	matches := re1.FindStringSubmatch(content)

	if len(matches) == 3 {
		// s.applicationID = matches[1]
		// s.apiKey = matches[2]
		return matches[1], matches[2], nil
	}

	// Try pattern 2: "apiKey":"...","applicationID":"..."
	re2 := regexp.MustCompile(
		`"apiKey":"([a-zA-Z0-9=,\.]{1,})","applicationID":"([a-zA-Z0-9]{1,})"`,
	)

	matches = re2.FindStringSubmatch(content)

	if len(matches) == 3 {
		// s.apiKey = matches[1]
		// s.applicationID = matches[2]
		return matches[2], matches[1], nil
	}
	return "", "", fmt.Errorf("algolia API credentials not found")
}

// discoverAlgoliaConfig discovers Algolia configuration
func discoverAlgoliaConfig(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, startURL, nil)
	if err != nil {
		data.Error = "Failed to create request: " + err.Error()
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch URL: " + err.Error()
		return
	}
	defer resp.Body.Close()

	data.AlgoliaAppID, data.AlgoliaAPIKey, err = getAlgoliaCredentials(resp)
	if err != nil {
		data.Error = "Failed to read response: " + err.Error()
		return
	}

	if data.AlgoliaAppID == "" || data.AlgoliaAPIKey == "" {
		data.Error = "Could not extract Algolia credentials from page HTML"
		return
	}

	// Decode base64 API key if needed
	if strings.Contains(data.AlgoliaAPIKey, "=") {
		//decoded, err := base64.StdEncoding.DecodeString(data.AlgoliaAPIKey)
		//if err == nil {
		//data.AlgoliaAPIKey = string(decoded)
		//}
	}

	// Now fetch a sample to discover available filter values
	fetchAlgoliaSample(startURL, data)
}

// fetchAlgoliaSample fetches comprehensive data from Algolia using their advanced API
func fetchAlgoliaSample(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Try common index names used by different sites
	// These match the hardcoded index in scrapers/algolia/scraper.go
	possibleIndexNames := []string{
		"all_scenes_latest_desc", // Gamma/Adult Time common index (PRIMARY)
		"all_scenes",             // Alternative Algolia index
		"scenes_latest_desc",     // Alternative with sorting
		"scenes",                 // Generic scenes index
		"all_videos",             // Video-based sites
		"videos",                 // Alternative video index
		"content",                // Generic content index
	}

	// Use the search API with facets to get comprehensive filter data
	// Algolia DSN is consistent across their network
	apiURL := fmt.Sprintf(
		"https://tsmkfa364q-dsn.algolia.net/1/indexes/*/queries?x-algolia-application-id=%s&x-algolia-api-key=%s",
		data.AlgoliaAppID,
		data.AlgoliaAPIKey,
	)

	// Helper function for min
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	// Try each possible index name
	var successfulIndex string
	var result struct {
		Facets map[string]map[string]int `json:"facets"`
		Hits   []map[string]interface{}  `json:"hits"`
	}

	var lastError string
	var debugInfo string // Store debug information
	for _, indexName := range possibleIndexNames {
		// Request with faceting enabled - get ALL facets with counts
		// This matches what the actual scraper does
		requestBody := fmt.Sprintf(
			`{"requests":[{"indexName":"%s","params":"query=&hitsPerPage=10&page=0&facets=*&maxValuesPerFacet=2000"}]}`,
			indexName,
		)

		req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(requestBody))
		if err != nil {
			lastError = fmt.Sprintf("Failed to create request for index '%s': %v", indexName, err)
			continue // Try next index
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Referer", startURL)
		req.Header.Set("x-algolia-api-key", data.AlgoliaAPIKey)
		req.Header.Set("x-algolia-application-id", data.AlgoliaAppID)

		resp, err := client.Do(req)
		if err != nil {
			lastError = fmt.Sprintf("Failed to fetch index '%s': %v", indexName, err)
			continue // Try next index
		}
		defer resp.Body.Close()

		// Check HTTP status
		if resp.StatusCode != http.StatusOK {
			// Build debug info for 403 errors
			debugInfo = fmt.Sprintf("\n\nDEBUG INFO:\nAPI URL: %s\nApp ID: %s\nAPI Key: %s\nRequest Body: %s\nReferer: %s\nHTTP Status: %d",
				apiURL, data.AlgoliaAppID, data.AlgoliaAPIKey, requestBody, startURL, resp.StatusCode)
			lastError = fmt.Sprintf("Index '%s' returned HTTP %d", indexName, resp.StatusCode)
			continue // Try next index
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastError = fmt.Sprintf("Failed to read response for index '%s': %v", indexName, err)
			continue // Try next index
		}

		// Parse response with facets
		var algoliaResp struct {
			Results []struct {
				Facets map[string]map[string]int `json:"facets"`
				Hits   []map[string]interface{}  `json:"hits"`
			} `json:"results"`
			Message string `json:"message"` // Algolia error message
		}

		if err := json.Unmarshal(body, &algoliaResp); err != nil {
			lastError = fmt.Sprintf("Failed to parse JSON for index '%s': %v. Response: %s", indexName, err, string(body[:min(200, len(body))]))
			continue // Try next index
		}

		// Check for Algolia error message
		if algoliaResp.Message != "" {
			lastError = fmt.Sprintf("Algolia error for index '%s': %s", indexName, algoliaResp.Message)
			continue // Try next index
		}

		// Check if we got a valid response with data
		if len(algoliaResp.Results) == 0 {
			lastError = fmt.Sprintf("Index '%s' returned empty results array", indexName)
			continue // Try next index - no results
		}

		// Success if we have at least some data (hits or facets)
		hasData := len(algoliaResp.Results[0].Hits) > 0 || len(algoliaResp.Results[0].Facets) > 0
		if !hasData {
			lastError = fmt.Sprintf("Index '%s' returned no hits or facets (hits: %d, facets: %d)",
				indexName, len(algoliaResp.Results[0].Hits), len(algoliaResp.Results[0].Facets))
			continue // Try next index - no data
		}

		// Success! We found a working index
		successfulIndex = indexName
		result = algoliaResp.Results[0]

		// Store sample response (truncate if too large)
		if len(body) > 5000 {
			data.SampleResponse = string(body[:5000]) + "... (truncated)"
		} else {
			data.SampleResponse = string(body)
		}
		break // Exit loop, we found data
	}

	// Check if we found any working index
	if successfulIndex == "" {
		errorMsg := "Could not find valid Algolia index. Tried: " + strings.Join(possibleIndexNames, ", ")
		if lastError != "" {
			errorMsg += ". Last error: " + lastError
		}
		if debugInfo != "" {
			errorMsg += debugInfo
		}
		data.Error = errorMsg
		return
	}

	data.AlgoliaIndexName = successfulIndex

	// Initialize facets map
	data.AlgoliaFacets = make(map[string][]FacetValue)

	// Extract all facets with their counts
	for facetName, facetValues := range result.Facets {
		var values []FacetValue
		for value, count := range facetValues {
			values = append(values, FacetValue{Value: value, Count: count})
		}
		data.AlgoliaFacets[facetName] = values

		// Populate specific known facets for easy access
		switch facetName {
		case "sitename":
			data.AvailableSites = values
		case "serie_name":
			data.AvailableSeries = values
		case "network.lvl0":
			data.AvailableNetworks0 = values
		case "network.lvl1":
			data.AvailableNetworks1 = values
		}
	}

	// Extract field names from a sample hit
	if len(result.Hits) > 0 {
		for fieldName := range result.Hits[0] {
			data.AlgoliaIndexFields = append(data.AlgoliaIndexFields, fieldName)
		}
	}
}

// discoverProject1ServiceConfig discovers Project1Service configuration
func discoverProject1ServiceConfig(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Fetch the start URL to get instance token
	req, err := http.NewRequest(http.MethodGet, startURL, nil)
	if err != nil {
		data.Error = "Failed to create request: " + err.Error()
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch URL: " + err.Error()
		return
	}
	defer resp.Body.Close()

	// Extract instance_token cookie
	var instanceToken string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "instance_token" {
			instanceToken = cookie.Value
			break
		}
	}

	if instanceToken == "" {
		data.Error = "Could not find instance_token cookie - make sure the URL is correct"
		return
	}

	// Parse URL to get site domain and path
	//parsedURL, err := url.Parse(startURL)
	//if err != nil {
	//	data.Error = "Invalid URL: " + err.Error()
	//	return
	//}

	// Fetch collections from API
	apiURL := "https://site-api.project1service.com/v1/collections?limit=100"
	req, err = http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		data.Error = "Failed to create API request: " + err.Error()
		return
	}

	req.AddCookie(&http.Cookie{Name: "instance_token", Value: instanceToken})
	req.Header.Set("Accept", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch collections: " + err.Error()
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		data.Error = "Failed to read collections response: " + err.Error()
		return
	}

	// Store sample response
	data.SampleResponse = string(body)

	// Parse collections
	var collections []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(body, &collections); err == nil {
		for _, col := range collections {
			data.Collections = append(data.Collections, CollectionInfo{
				ID:   col.ID,
				Name: col.Name,
			})
		}
	}
}

// discoverCSRFAPIConfig discovers CSRF API configuration
func discoverCSRFAPIConfig(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, startURL, nil)
	if err != nil {
		data.Error = "Failed to create request: " + err.Error()
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch URL: " + err.Error()
		return
	}
	defer resp.Body.Close()

	// Look for CSRF cookies
	for _, cookie := range resp.Cookies() {
		cookieName := strings.ToLower(cookie.Name)
		if strings.Contains(cookieName, "csrf") || strings.Contains(cookieName, "xsrf") {
			data.CSRFCookie = cookie.Name
			break
		}
	}

	if data.CSRFCookie == "" {
		data.Error = "No CSRF cookie found - try checking browser DevTools manually"
	}

	// Suggest common CSRF header names
	data.CSRFHeader = "csrf-token (check Network tab in DevTools for exact header name)"
}

// getScraperTypeDisplayName returns a user-friendly display name for a scraper type
func getScraperTypeDisplayName(scraperType string) string {
	switch scraperType {
	case "htmlxpath":
		return "HTML/XPath Scraper"
	case "csrfapi":
		return "CSRF API Scraper"
	case "algolia":
		return "Algolia Search"
	case "project1service":
		return "Project1Service (Aylo/MindGeek)"
	default:
		return scraperType
	}
}

// renderScraperHelperPage renders the scraper helper interface
func renderScraperHelperPage(filename, index, scraperType, startURL string, discoveredData *ScraperDiscoveryData, csrfToken string) gomponents.Node {
	// Check if scraper type was auto-detected
	autoDetected := startURL != "" && scraperType != ""

	return html.Div(
		html.Class("container-fluid p-4"),
		html.H1(html.Class("h3 mb-3"), gomponents.Text("Scraper Configuration Helper")),

		html.Div(
			html.Class("card mb-4"),
			html.Div(
				html.Class("card-header"),
				html.H5(html.Class("mb-0"), gomponents.Text("Discover Scraper Values")),
			),
			html.Div(
				html.Class("card-body"),
				html.P(gomponents.Text("Enter a URL below and the helper will automatically detect the scraper type and extract available configuration values.")),

				// Show auto-detection notice
				gomponents.If(autoDetected,
					html.Div(
						html.Class("alert alert-info mb-3"),
						html.I(html.Class("fa-solid fa-magic me-2")),
						html.Strong(gomponents.Text("Auto-detected scraper type: ")),
						gomponents.Text(getScraperTypeDisplayName(scraperType)),
					),
				),

				html.Form(
					html.Method("GET"),
					html.Action("/api/admin/seriesconfig/helper"),
					html.Input(html.Type("hidden"), html.Name("file"), html.Value(filename)),
					html.Input(html.Type("hidden"), html.Name("index"), html.Value(index)),

					html.Div(
						html.Class("mb-3"),
						html.Label(html.Class("form-label"), gomponents.Text("Start URL")),
						html.Input(
							html.Type("text"),
							html.Class("form-control"),
							html.Name("url"),
							html.ID("start-url"),
							html.Placeholder("https://example.com/videos"),
							html.Value(startURL),
							html.Required(),
						),
						html.Div(html.Class("form-text"), gomponents.Text("The URL will be analyzed to auto-detect the scraper type")),
					),

					html.Div(
						html.Class("mb-3"),
						html.Label(html.Class("form-label"), gomponents.Text("Scraper Type (optional - auto-detected if blank)")),
						html.Select(
							html.Class("form-select"),
							html.Name("type"),
							html.ID("scraper-type"),
							html.Option(html.Value(""), gomponents.Text("Auto-detect"), gomponents.If(scraperType == "", html.Selected())),
							html.Option(html.Value("htmlxpath"), gomponents.Text("HTML/XPath"), gomponents.If(scraperType == "htmlxpath", html.Selected())),
							html.Option(html.Value("csrfapi"), gomponents.Text("CSRF API"), gomponents.If(scraperType == "csrfapi", html.Selected())),
							html.Option(html.Value("algolia"), gomponents.Text("Algolia"), gomponents.If(scraperType == "algolia", html.Selected())),
							html.Option(html.Value("project1service"), gomponents.Text("Project1Service"), gomponents.If(scraperType == "project1service", html.Selected())),
						),
						html.Div(html.Class("form-text"), gomponents.Text("Leave as 'Auto-detect' to automatically determine the scraper type")),
					),

					html.Button(
						html.Class("btn btn-primary"),
						html.Type("submit"),
						html.I(html.Class("fa-solid fa-magic me-2")),
						gomponents.Text("Analyze URL"),
					),

					html.A(
						html.Class("btn btn-secondary ms-2"),
						html.Href(fmt.Sprintf("/api/admin/seriesconfig/edit?file=%s&index=%s", filename, index)),
						gomponents.Text("Back to Edit"),
					),
				),
			),
		),

		gomponents.If(discoveredData != nil, renderScraperHelperResults(scraperType, startURL, discoveredData)),
	)
}

// renderScraperHelperResults renders the analysis results
func renderScraperHelperResults(scraperType, startURL string, data *ScraperDiscoveryData) gomponents.Node {
	// Safety check for nil data
	if data == nil {
		return html.Div(
			html.Class("card"),
			html.Div(
				html.Class("card-header"),
				html.H5(html.Class("mb-0"), gomponents.Text("Analysis Results")),
			),
			html.Div(
				html.Class("card-body"),
				html.Div(
					html.Class("alert alert-warning"),
					gomponents.Text("No data available. Please enter a URL and click Analyze."),
				),
			),
		)
	}

	return html.Div(
		html.Class("card"),
		html.Div(
			html.Class("card-header"),
			html.H5(html.Class("mb-0"), gomponents.Text("Analysis Results")),
		),
		html.Div(
			html.Class("card-body"),
			// Show error if any
			gomponents.If(data.Error != "",
				html.Div(
					html.Class("alert alert-danger"),
					html.I(html.Class("fa-solid fa-exclamation-circle me-2")),
					gomponents.Text(data.Error),
				),
			),

			// Show discovered data based on scraper type
			gomponents.If(scraperType == "algolia" && data.Error == "", renderAlgoliaDiscoveredData(data)),
			gomponents.If(scraperType == "project1service" && data.Error == "", renderProject1ServiceDiscoveredData(data)),
			gomponents.If(scraperType == "csrfapi" && data.Error == "", renderCSRFAPIDiscoveredData(data)),

			// Always show the help guide
			gomponents.If(scraperType == "htmlxpath", renderXPathHelp()),
			gomponents.If(scraperType == "csrfapi" && data.CSRFCookie == "", renderCSRFAPIHelp()),
			gomponents.If(scraperType == "algolia" && data.AlgoliaAppID == "", renderAlgoliaHelp()),
			gomponents.If(scraperType == "project1service" && len(data.Collections) == 0, renderProject1ServiceHelp()),
		),
	)
}

// renderAlgoliaDiscoveredData renders discovered Algolia configuration
func renderAlgoliaDiscoveredData(data *ScraperDiscoveryData) gomponents.Node {
	return html.Div(
		html.Div(
			html.Class("alert alert-success"),
			html.I(html.Class("fa-solid fa-check-circle me-2")),
			gomponents.Text("Successfully discovered Algolia configuration!"),
		),

		html.H6(gomponents.Text("Algolia Credentials")),
		html.Div(
			html.Class("mb-3"),
			html.Strong(gomponents.Text("Application ID: ")),
			html.Code(gomponents.Text(data.AlgoliaAppID)),
		),
		html.Div(
			html.Class("mb-3"),
			html.Strong(gomponents.Text("API Key: ")),
			html.Code(gomponents.Text(data.AlgoliaAPIKey)),
		),
		html.Div(
			html.Class("mb-3"),
			html.Strong(gomponents.Text("Index Name: ")),
			html.Code(gomponents.Text(data.AlgoliaIndexName)),
		),

		// Display available index fields
		gomponents.If(len(data.AlgoliaIndexFields) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Available Index Fields")),
				html.Div(
					html.Class("alert alert-secondary"),
					html.Small(gomponents.Text("These are the fields available in the Algolia index that you can use for filtering and extraction:")),
					html.Div(
						html.Class("mt-2"),
						renderFieldList(data.AlgoliaIndexFields),
					),
				),
			),
		),

		// Display main facets with counts
		gomponents.If(len(data.AvailableSites) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Available Sites ("+fmt.Sprintf("%d", len(data.AvailableSites))+")")),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("SiteFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact site name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Site Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableSites),
						),
					),
				),
			),
		),

		gomponents.If(len(data.AvailableSeries) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Available Series ("+fmt.Sprintf("%d", len(data.AvailableSeries))+")")),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("SerieFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact series name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Series Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableSeries),
						),
					),
				),
			),
		),

		gomponents.If(len(data.AvailableNetworks0) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Available Networks Lvl0 ("+fmt.Sprintf("%d", len(data.AvailableNetworks0))+")")),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("NetworkFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact network name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Network Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableNetworks0),
						),
					),
				),
			),
		),

		gomponents.If(len(data.AvailableNetworks1) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Available Networks Lvl1 ("+fmt.Sprintf("%d", len(data.AvailableNetworks1))+")")),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("NetworkFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact network name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Network Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableNetworks1),
						),
					),
				),
			),
		),
		// Display all other facets discovered
		gomponents.If(len(data.AlgoliaFacets) > 3,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Other Available Facets")),
				renderOtherFacets(data.AlgoliaFacets),
			),
		),
	)
}

// renderProject1ServiceDiscoveredData renders discovered Project1Service configuration
func renderProject1ServiceDiscoveredData(data *ScraperDiscoveryData) gomponents.Node {
	return html.Div(
		html.Div(
			html.Class("alert alert-success"),
			html.I(html.Class("fa-solid fa-check-circle me-2")),
			gomponents.Text("Successfully discovered Project1Service collections!"),
		),

		gomponents.If(len(data.Collections) > 0,
			html.Div(
				html.H6(gomponents.Text("Available Collections ("+fmt.Sprintf("%d", len(data.Collections))+")")),
				html.Div(
					html.Class("table-responsive mt-3"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("ID")),
								html.Th(gomponents.Text("Collection Name")),
							),
						),
						html.TBody(
							renderCollectionRows(data.Collections),
						),
					),
				),
				html.Div(
					html.Class("alert alert-info mt-3"),
					html.Strong(gomponents.Text("Usage: ")),
					gomponents.Text("Copy the ID number from the table above and use it as FilterCollectionID in your configuration."),
				),
			),
		),
	)
}

// renderCSRFAPIDiscoveredData renders discovered CSRF API configuration
func renderCSRFAPIDiscoveredData(data *ScraperDiscoveryData) gomponents.Node {
	return html.Div(
		gomponents.If(data.CSRFCookie != "",
			html.Div(
				html.Div(
					html.Class("alert alert-success"),
					html.I(html.Class("fa-solid fa-check-circle me-2")),
					gomponents.Text("Found CSRF cookie!"),
				),
				html.Div(
					html.Class("mb-3"),
					html.Strong(gomponents.Text("CSRF Cookie Name: ")),
					html.Code(gomponents.Text(data.CSRFCookie)),
				),
				html.Div(
					html.Class("mb-3"),
					html.Strong(gomponents.Text("CSRF Header Name: ")),
					gomponents.Text(data.CSRFHeader),
				),
			),
		),
	)
}

// renderFacetRows renders facet values with counts as table rows
func renderFacetRows(facets []FacetValue) gomponents.Node {
	var rows []gomponents.Node
	for _, facet := range facets {
		rows = append(rows, html.Tr(
			html.Td(html.Code(gomponents.Text(facet.Value))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", facet.Count))),
		))
	}
	return gomponents.Group(rows)
}

// renderFieldList renders a list of field names
func renderFieldList(fields []string) gomponents.Node {
	var nodes []gomponents.Node
	for _, field := range fields {
		nodes = append(nodes, html.Span(
			html.Class("badge bg-secondary me-1 mb-1"),
			gomponents.Text(field),
		))
	}
	return gomponents.Group(nodes)
}

// renderOtherFacets renders all facets that aren't the main three
func renderOtherFacets(allFacets map[string][]FacetValue) gomponents.Node {
	var nodes []gomponents.Node

	// Skip the main three facets we already displayed
	skipFacets := map[string]bool{
		"sitename":     true,
		"serie_name":   true,
		"network_name": true,
	}

	for facetName, values := range allFacets {
		if skipFacets[facetName] || len(values) == 0 {
			continue
		}

		// Limit display to first 20 values for other facets
		displayValues := values
		if len(displayValues) > 20 {
			displayValues = displayValues[:20]
		}

		nodes = append(nodes, html.Div(
			html.Class("mb-3"),
			html.H6(html.Class("text-muted"),
				gomponents.Text(facetName),
				html.Small(html.Class("ms-2"), gomponents.Text(fmt.Sprintf("(%d values)", len(values)))),
			),
			html.Div(
				html.Class("table-responsive"),
				html.Table(
					html.Class("table table-sm table-striped"),
					html.THead(
						html.Tr(
							html.Th(gomponents.Text("Value")),
							html.Th(gomponents.Text("Count")),
						),
					),
					html.TBody(
						renderFacetRows(displayValues),
					),
				),
			),
			gomponents.If(len(values) > 20,
				html.Small(html.Class("text-muted"),
					gomponents.Text(fmt.Sprintf("Showing first 20 of %d values", len(values))),
				),
			),
		))
	}

	if len(nodes) == 0 {
		return gomponents.Text("")
	}

	return gomponents.Group(nodes)
}

// renderCollectionRows renders collection table rows
func renderCollectionRows(collections []CollectionInfo) gomponents.Node {
	var rows []gomponents.Node
	for _, col := range collections {
		rows = append(rows, html.Tr(
			html.Td(html.Code(gomponents.Text(fmt.Sprintf("%d", col.ID)))),
			html.Td(gomponents.Text(col.Name)),
		))
	}
	return gomponents.Group(rows)
}

// renderXPathHelp renders XPath configuration help
func renderXPathHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("HTML/XPath Scraper Configuration Guide")),
		html.P(gomponents.Text("To configure an HTML/XPath scraper, you need to inspect the website's HTML structure using browser DevTools (F12):")),
		html.Ol(
			html.Li(gomponents.Text("Open the target URL in your browser")),
			html.Li(gomponents.Text("Press F12 to open DevTools, go to Elements/Inspector tab")),
			html.Li(gomponents.Text("Right-click on a video/scene container → Inspect")),
			html.Li(gomponents.Text("Find the common container element (e.g., <div class=\"video-thumb\">)")),
			html.Li(gomponents.Text("Right-click the element → Copy → Copy XPath")),
		),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("Common XPath Patterns:")),
			html.Ul(
				html.Li(html.Code(gomponents.Text("//div[@class=\"video-thumb\"]")), gomponents.Text(" - Scene containers")),
				html.Li(html.Code(gomponents.Text(".//a[@class=\"title\"]")), gomponents.Text(" - Title link (relative to container)")),
				html.Li(html.Code(gomponents.Text(".//span[@class=\"date\"]")), gomponents.Text(" - Date element")),
				html.Li(html.Code(gomponents.Text(".//div[@class=\"models\"]/a")), gomponents.Text(" - Actor links")),
			),
		),

		html.Div(
			html.Class("alert alert-warning mt-3"),
			html.Strong(gomponents.Text("Testing XPath: ")),
			gomponents.Text("In browser console, use "),
			html.Code(gomponents.Text("$x('//your/xpath')")),
			gomponents.Text(" to test if it selects the right elements."),
		),
	)
}

// renderCSRFAPIHelp renders CSRF API configuration help
func renderCSRFAPIHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("CSRF API Scraper Configuration Guide")),
		html.P(gomponents.Text("To configure a CSRF API scraper, you need to inspect the website's API calls using browser DevTools:")),
		html.Ol(
			html.Li(gomponents.Text("Open the target URL in your browser")),
			html.Li(gomponents.Text("Press F12 → Network tab → Filter to 'Fetch/XHR'")),
			html.Li(gomponents.Text("Reload the page or navigate to see API calls")),
			html.Li(gomponents.Text("Look for JSON API requests (usually contain 'api', 'page', 'gallery', etc.)")),
			html.Li(gomponents.Text("Click on the API call → Headers tab")),
		),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("What to find:")),
			html.Ul(
				html.Li(html.Strong(gomponents.Text("CSRF Cookie: ")), gomponents.Text("Application tab → Cookies → look for cookies like '_csrf', 'csrf_token', 'XSRF-TOKEN'")),
				html.Li(html.Strong(gomponents.Text("CSRF Header: ")), gomponents.Text("Request Headers → look for 'csrf-token', 'X-CSRF-Token', 'X-XSRF-TOKEN'")),
				html.Li(html.Strong(gomponents.Text("API URL: ")), gomponents.Text("Copy the Request URL, replace page number with {page}")),
			),
		),

		html.Div(
			html.Class("alert alert-info mt-3"),
			html.H6(gomponents.Text("JSON Response Fields:")),
			gomponents.Text("Click on the API call → Response/Preview tab to see the JSON structure:"),
			html.Ul(
				html.Li(html.Code(gomponents.Text("galleries")), gomponents.Text(", "), html.Code(gomponents.Text("scenes")), gomponents.Text(", "), html.Code(gomponents.Text("data.results")), gomponents.Text(" - common array paths")),
				html.Li(html.Code(gomponents.Text("name")), gomponents.Text(", "), html.Code(gomponents.Text("title")), gomponents.Text(" - title fields")),
				html.Li(html.Code(gomponents.Text("publishedAt")), gomponents.Text(", "), html.Code(gomponents.Text("release_date")), gomponents.Text(" - date fields")),
			),
		),
	)
}

// renderAlgoliaHelp renders Algolia configuration help
func renderAlgoliaHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("Algolia Scraper Configuration Guide")),
		html.P(gomponents.Text("Algolia scrapers are used for Gamma Entertainment/Adult Time network sites:")),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("How to find filter values:")),
			html.Ol(
				html.Li(gomponents.Text("Open the target URL in your browser")),
				html.Li(gomponents.Text("Press F12 → Network tab → Filter to 'Fetch/XHR'")),
				html.Li(gomponents.Text("Reload the page")),
				html.Li(gomponents.Text("Look for requests to 'algolia' or 'algolianet'")),
				html.Li(gomponents.Text("Click on the request → Payload tab")),
				html.Li(gomponents.Text("Look at the filters/facetFilters in the request body")),
			),
		),

		html.Div(
			html.Class("alert alert-info mt-3"),
			html.H6(gomponents.Text("Common Filter Values:")),
			html.Ul(
				html.Li(html.Strong(gomponents.Text("SiteFilterName: ")), html.Code(gomponents.Text("sitename:girlsway")), gomponents.Text(" → use 'girlsway'")),
				html.Li(html.Strong(gomponents.Text("SerieFilterName: ")), gomponents.Text("Usually the series/channel name")),
				html.Li(html.Strong(gomponents.Text("NetworkFilterName: ")), gomponents.Text("The network name if filtering by network")),
			),
		),
	)
}

// renderProject1ServiceHelp renders Project1Service configuration help
func renderProject1ServiceHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("Project1Service Scraper Configuration Guide")),
		html.P(gomponents.Text("Project1Service scrapers are used for Aylo/MindGeek network sites (Brazzers, RealityKings, etc.):")),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("How to find FilterCollectionID:")),
			html.Ol(
				html.Li(gomponents.Text("Navigate to the site (e.g., twistys.com)")),
				html.Li(gomponents.Text("Browse to the specific series/collection you want")),
				html.Li(gomponents.Text("Look at the URL in your browser")),
				html.Li(gomponents.Text("The collection ID is the numeric value in the URL")),
			),
		),

		html.Div(
			html.Class("alert alert-info mt-3"),
			html.H6(gomponents.Text("Example:")),
			html.P(
				gomponents.Text("If the URL is: "),
				html.Code(gomponents.Text("https://www.twistys.com/collection/227/when-girls-play")),
			),
			html.P(
				gomponents.Text("Then "),
				html.Strong(gomponents.Text("FilterCollectionID = 227")),
			),
		),

		html.Div(
			html.Class("alert alert-warning mt-3"),
			html.Strong(gomponents.Text("Note: ")),
			gomponents.Text("You may need to filter by site first to see collections. Set to 0 to disable filtering and get all scenes from the site."),
		),
	)
}
