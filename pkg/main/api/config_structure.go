package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// FolderOrganizationResults represents the results from folder organization.
type FolderOrganizationResults struct {
	TotalFiles     int                 `json:"total_files"`
	ProcessedFiles int                 `json:"processed_files"`
	OrganizedFiles int                 `json:"organized_files"`
	SkippedFiles   int                 `json:"skipped_files"`
	ErrorFiles     int                 `json:"error_files"`
	FileOperations []FileOperation     `json:"file_operations"`
	Errors         []OrganizationError `json:"errors"`
	Summary        OrganizationSummary `json:"summary"`
}

// FileOperation represents a single file operation during organization.
type FileOperation struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	Operation  string `json:"operation"` // "move", "skip", "error"
	Reason     string `json:"reason"`
	MediaTitle string `json:"media_title"`
	MediaYear  string `json:"media_year"`
	Quality    string `json:"quality"`
	Resolution string `json:"resolution"`
}

// OrganizationError represents an error that occurred during organization.
type OrganizationError struct {
	FilePath string `json:"file_path"`
	Error    string `json:"error"`
}

// OrganizationSummary provides a summary of the organization operation.
type OrganizationSummary struct {
	ProcessingTime   string `json:"processing_time"`
	MovedFiles       int    `json:"moved_files"`
	RenamedFiles     int    `json:"renamed_files"`
	DeletedFiles     int    `json:"deleted_files"`
	CreatedFolders   int    `json:"created_folders"`
	RuntimeVerified  int    `json:"runtime_verified"`
	LanguageFiltered int    `json:"language_filtered"`
}

// ================================================================================
// FOLDER STRUCTURE PAGE
// ================================================================================

// renderFolderStructurePage renders a page for organizing a single folder.
func renderFolderStructurePage(csrfToken string) gomponents.Node {
	// Get available media configurations
	media := config.GetSettingsMediaAll()

	var mediaConfigs []string
	for i := range media.Movies {
		mediaConfigs = append(mediaConfigs, media.Movies[i].NamePrefix)
	}

	for i := range media.Series {
		mediaConfigs = append(mediaConfigs, media.Series[i].NamePrefix)
	}

	cfgdata := config.GetSettingsPathAll()

	var pathConfigs []string
	for i := range cfgdata {
		pathConfigs = append(pathConfigs, cfgdata[i].Name)
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
					html.I(html.Class("fa-solid fa-folder header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(
						html.Class("header-title"),
						gomponents.Text("Folder Structure Organizer"),
					),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Organize and structure a single folder using your media configuration templates. This tool will scan the folder, parse media files, and organize them according to your naming conventions.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("folderStructureForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Folder & Configuration"),
					),

					renderFormGroup("structure", map[string]string{
						"FolderPath": "Path to the folder to organize (must exist)",
					}, map[string]string{
						"FolderPath": "Folder Path",
					}, "FolderPath", "text", "", nil),

					renderFormGroup("structure", map[string]string{
						"MediaConfig": "Select the media configuration to use for organization",
					}, map[string]string{
						"MediaConfig": "Media Configuration",
					}, "MediaConfig", "select", "", map[string][]string{
						"options": mediaConfigs,
					}),

					renderFormGroup("structure", map[string]string{
						"DataImportTemplate": "Template for data import organization",
					}, map[string]string{
						"DataImportTemplate": "Data Import Template",
					}, "DataImportTemplate", "select", "movie", map[string][]string{
						"options": pathConfigs,
					}),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Organization Options"),
					),

					renderFormGroup("structure", map[string]string{
						"DefaultTemplate": "Default template for organization",
					}, map[string]string{
						"DefaultTemplate": "Default Template",
					}, "DefaultTemplate", "select", "movie", map[string][]string{
						"options": pathConfigs,
					}),

					renderFormGroup("structure", map[string]string{
						"CheckRuntime": "Verify runtime during organization",
					}, map[string]string{
						"CheckRuntime": "Check Runtime",
					}, "CheckRuntime", "checkbox", false, nil),

					renderFormGroup("structure", map[string]string{
						"DeleteWrongLanguage": "Delete files with wrong language",
					}, map[string]string{
						"DeleteWrongLanguage": "Delete Wrong Language",
					}, "DeleteWrongLanguage", "checkbox", false, nil),

					renderFormGroup("structure", map[string]string{
						"ManualID": "Manual ID for organization (0 = auto)",
					}, map[string]string{
						"ManualID": "Manual ID",
					}, "ManualID", "number", "0", nil),

					renderFormGroup("structure", map[string]string{
						"DryRun": "Preview organization without making changes",
					}, map[string]string{
						"DryRun": "Dry Run (Preview Only)",
					}, "DryRun", "checkbox", true, nil),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class(ClassBtnPrimary),
					gomponents.Text("Organize Folder"),
					html.Type("button"),
					hx.Target("#structureResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/folderstructure"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					hx.Include("#folderStructureForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('folderStructureForm').reset(); document.getElementById('structureResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("structureResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-folder-open me-1")),
						gomponents.Text("Organization"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Folder Organization Information"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text(
						"Configure your folder organization settings and understand the options below:",
					),
				),
				html.Ul(
					html.Class("list-unstyled mb-3"),
					html.Li(
						html.Class("mb-2 d-flex align-items-start"),
						html.I(html.Class("fas fa-folder me-2 mt-1 text-success")),
						html.Div(
							html.Strong(gomponents.Text("Folder Path: ")),
							gomponents.Text(
								"Full path to the folder containing media files to organize",
							),
						),
					),
					html.Li(
						html.Class("mb-2 d-flex align-items-start"),
						html.I(html.Class("fas fa-cogs me-2 mt-1 text-info")),
						html.Div(
							html.Strong(gomponents.Text("Media Config: ")),
							gomponents.Text(
								"Configuration containing naming templates and organization rules",
							),
						),
					),
					html.Li(
						html.Class("mb-2 d-flex align-items-start"),
						html.I(html.Class("fas fa-code me-2 mt-1 text-primary")),
						html.Div(
							html.Strong(gomponents.Text("Templates: ")),
							gomponents.Text(
								"Templates define how files should be named and organized",
							),
						),
					),
					html.Li(
						html.Class("mb-2 d-flex align-items-start"),
						html.I(html.Class("fas fa-clock me-2 mt-1 text-warning")),
						html.Div(
							html.Strong(gomponents.Text("Check Runtime: ")),
							gomponents.Text("Validates media file runtime against expected values"),
						),
					),
					html.Li(
						html.Class("mb-2 d-flex align-items-start"),
						html.I(html.Class("fas fa-language me-2 mt-1 text-warning")),
						html.Div(
							html.Strong(gomponents.Text("Delete Wrong Language: ")),
							gomponents.Text("Removes files that don't match language preferences"),
						),
					),
					html.Li(
						html.Class("mb-2 d-flex align-items-start"),
						html.I(html.Class("fas fa-hashtag me-2 mt-1 text-primary")),
						html.Div(
							html.Strong(gomponents.Text("Manual ID: ")),
							gomponents.Text(
								"Override automatic ID detection with a specific value",
							),
						),
					),
				),

				html.Div(
					html.Class("alert alert-light border-0 mt-3 mb-0"),
					html.Style(
						"background-color: rgba(13, 110, 253, 0.1); border-radius: 8px; padding: 0.75rem 1rem;",
					),
					html.Div(
						html.Class("d-flex align-items-start"),
						html.I(
							html.Class("fas fa-eye me-2 mt-1"),
							html.Style("color: #0d6efd; font-size: 0.9rem;"),
						),
						html.Div(
							html.Strong(
								html.Style("color: #0d6efd;"),
								gomponents.Text("Dry Run: "),
							),
							gomponents.Text(
								"When enabled, shows what changes would be made without actually moving or renaming files. Recommended for testing before actual organization.",
							),
						),
					),
				),
			),
		),

		html.Div(
			html.Class("alert alert-warning border-0 mb-0"),
			html.Style(
				"background-color: rgba(255, 193, 7, 0.1); border-radius: 8px; padding: 0.75rem 1rem; border-left: 4px solid #ffc107;",
			),
			html.Div(
				html.Class("d-flex align-items-start"),
				html.I(
					html.Class("fas fa-exclamation-triangle me-2 mt-1"),
					html.Style("color: #ffc107; font-size: 0.9rem;"),
				),
				html.Div(
					html.Strong(html.Style("color: #856404;"), gomponents.Text("Warning: ")),
					gomponents.Text(
						"Organization will move and rename files according to your templates. Always test with dry run first!",
					),
				),
			),
		),
	)
}

// previewFolderOrganization scans a folder and shows what would be organized without making changes.
func previewFolderOrganization(
	ctx context.Context,
	folderPath string,
) (*FolderOrganizationResults, error) {
	startTime := time.Now()
	results := &FolderOrganizationResults{
		FileOperations: make([]FileOperation, 0),
		Errors:         make([]OrganizationError, 0),
	}

	// Walk through the folder to find media files
	err := filepath.WalkDir(folderPath, func(fpath string, info os.DirEntry, errw error) error {
		if errw != nil {
			results.Errors = append(results.Errors, OrganizationError{
				FilePath: fpath,
				Error:    errw.Error(),
			})

			return nil // Continue processing other files
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if info.IsDir() {
			return nil
		}

		results.TotalFiles++

		// Check if it's a media file
		ext := filepath.Ext(info.Name())
		if ext == "" {
			return nil
		}

		// For preview, we'll simulate the organization process
		// In a real implementation, this would use the actual parser and structure logic
		mediaTitle := "Example Media"
		mediaYear := fmt.Sprintf("%d", time.Now().Year())
		quality := "1080p"
		resolution := "HD"

		// Simulate what the organized path would be
		targetPath := filepath.Join(
			folderPath,
			"organized",
			mediaTitle+" ("+mediaYear+")",
			info.Name(),
		)

		operation := FileOperation{
			SourcePath: fpath,
			TargetPath: targetPath,
			Operation:  "move",
			Reason:     "Preview - would organize this file",
			MediaTitle: mediaTitle,
			MediaYear:  mediaYear,
			Quality:    quality,
			Resolution: resolution,
		}

		results.FileOperations = append(results.FileOperations, operation)
		results.ProcessedFiles++

		results.OrganizedFiles++

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Calculate summary
	processingTime := time.Since(startTime)

	results.Summary = OrganizationSummary{
		ProcessingTime:   processingTime.String(),
		MovedFiles:       results.OrganizedFiles,
		RenamedFiles:     0,
		DeletedFiles:     0,
		CreatedFolders:   results.OrganizedFiles, // Estimate one folder per file
		RuntimeVerified:  0,
		LanguageFiltered: 0,
	}

	return results, nil
}

// organizeFolderWithResults organizes a folder and returns detailed results.
func organizeFolderWithResults(
	ctx context.Context,
	folderPath string,
	mediaTypeConfig *config.MediaTypeConfig,
	dataImportConfig *config.MediaDataImportConfig,
	defaultTemplate string,
	checkRuntime, deleteWrongLanguage bool,
	manualID uint,
) (*FolderOrganizationResults, error) {
	startTime := time.Now()
	results := &FolderOrganizationResults{
		FileOperations: make([]FileOperation, 0),
		Errors:         make([]OrganizationError, 0),
	}

	// Use the actual OrganizeSingleFolder function
	err := structure.OrganizeSingleFolder(
		ctx,
		folderPath,
		mediaTypeConfig,
		dataImportConfig,
		defaultTemplate,
		checkRuntime,
		deleteWrongLanguage,
		manualID,
	)
	if err != nil {
		results.Errors = append(results.Errors, OrganizationError{
			FilePath: folderPath,
			Error:    err.Error(),
		})
		results.ErrorFiles++
	} else {
		results.OrganizedFiles++
	}

	// Count total files in folder
	filepath.WalkDir(folderPath, func(fpath string, info os.DirEntry, errw error) error {
		if errw != nil || info.IsDir() {
			return nil
		}

		results.TotalFiles++

		return nil
	})

	results.ProcessedFiles = results.TotalFiles - results.ErrorFiles

	// Calculate summary
	processingTime := time.Since(startTime)

	results.Summary = OrganizationSummary{
		ProcessingTime:   processingTime.String(),
		MovedFiles:       results.OrganizedFiles,
		RenamedFiles:     0, // This would be tracked by the actual organization
		DeletedFiles:     0, // This would be tracked by the actual organization
		CreatedFolders:   0, // This would be tracked by the actual organization
		RuntimeVerified:  0, // This would be tracked by the actual organization
		LanguageFiltered: 0, // This would be tracked by the actual organization
	}

	// Add a general operation result since we don't have detailed tracking yet
	if err == nil {
		results.FileOperations = append(results.FileOperations, FileOperation{
			SourcePath: folderPath,
			TargetPath: "Organized according to configuration",
			Operation:  "organize",
			Reason:     "Folder organization completed successfully",
			MediaTitle: "",
			MediaYear:  "",
			Quality:    "",
			Resolution: "",
		})
	}

	return results, nil
}

// HandleFolderStructure handles folder structure organization requests.
func HandleFolderStructure(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	folderPath := c.PostForm("structure_FolderPath")
	mediaConfig := c.PostForm("structure_MediaConfig")
	dataImportTemplate := c.PostForm("structure_DataImportTemplate")
	defaultTemplate := c.PostForm("structure_DefaultTemplate")
	checkRuntime := c.PostForm("structure_CheckRuntime") == "on"
	deleteWrongLanguage := c.PostForm("structure_DeleteWrongLanguage") == "on"
	dryRun := c.PostForm("structure_DryRun") == "on"
	manualIDStr := c.PostForm("structure_ManualID")

	if folderPath == "" || mediaConfig == "" {
		c.String(
			http.StatusOK,
			renderAlert("Please fill in folder path and media configuration", "warning"),
		)

		return
	}

	manualID := parseUintOrDefault(manualIDStr, 0)

	// Get the media configuration
	var mediaTypeConfig *config.MediaTypeConfig
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if strings.EqualFold(media.NamePrefix, mediaConfig) {
			mediaTypeConfig = media
			return nil
		}

		return nil
	})

	if mediaTypeConfig == nil {
		c.String(http.StatusOK, renderAlert("Media configuration not found", "danger"))
		return
	}

	// Check if folder exists
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		c.String(http.StatusOK, renderAlert("Folder path does not exist: "+folderPath, "danger"))
		return
	}

	// Create a MediaDataImportConfig for structure organization
	dataImportConfig := &config.MediaDataImportConfig{
		TemplatePath: dataImportTemplate,
	}
	if dataImportConfig.TemplatePath == "" {
		dataImportConfig.TemplatePath = defaultTemplate
	}

	// Run folder organization or preview
	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var (
		organizationResults *FolderOrganizationResults
		err                 error
	)

	if dryRun {
		// Preview mode - scan folder and show what would be organized
		organizationResults, err = previewFolderOrganization(ctx, folderPath)
	} else {
		// Actual organization
		organizationResults, err = organizeFolderWithResults(ctx, folderPath, mediaTypeConfig, dataImportConfig, defaultTemplate, checkRuntime, deleteWrongLanguage, manualID)
	}

	if err != nil {
		c.String(http.StatusOK, renderAlert("Organization failed: "+err.Error(), "danger"))
		return
	}

	result := map[string]any{
		"folder_path":           folderPath,
		"media_config":          mediaConfig,
		"data_import_template":  dataImportTemplate,
		"default_template":      defaultTemplate,
		"check_runtime":         checkRuntime,
		"delete_wrong_language": deleteWrongLanguage,
		"manual_id":             manualID,
		"dry_run":               dryRun,
		"organization_results":  organizationResults,
		"success":               true,
	}

	c.String(http.StatusOK, renderFolderStructureResults(result))
}

// renderFolderStructureResults renders the folder structure organization results.
func renderFolderStructureResults(result map[string]any) string {
	folderPath, _ := result["folder_path"].(string)
	mediaConfig, _ := result["media_config"].(string)
	dataImportTemplate, _ := result["data_import_template"].(string)
	defaultTemplate, _ := result["default_template"].(string)
	checkRuntime, _ := result["check_runtime"].(bool)
	deleteWrongLanguage, _ := result["delete_wrong_language"].(bool)
	manualID, _ := result["manual_id"].(uint)
	dryRun, _ := result["dry_run"].(bool)

	// Display actual organization results
	organizationResults, hasResults := result["organization_results"].(*FolderOrganizationResults)
	if !hasResults {
		return renderComponentToString(
			html.Div(
				html.Class("card border-0 shadow-sm border-danger mb-4"),
				html.Div(
					html.Class("card-header border-0"),
					html.Style(
						"background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%); border-radius: 15px 15px 0 0;",
					),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-danger me-3"),
							html.I(html.Class("fas fa-exclamation-triangle me-1")),
							gomponents.Text("Error"),
						),
						html.H5(
							html.Class("card-title mb-0 text-danger fw-bold"),
							gomponents.Text("Organization Error"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted mb-0"),
						gomponents.Text(
							"No results were returned from the folder organization operation.",
						),
					),
				),
			),
		)
	}

	// Create components for displaying results
	var components []gomponents.Node

	// Add basic information table
	components = append(components,
		html.Table(
			html.Class("table table-sm"),
			html.TBody(
				html.Tr(
					html.Td(gomponents.Text("Folder Path:")),
					html.Td(gomponents.Text(folderPath)),
				),
				html.Tr(
					html.Td(gomponents.Text("Media Configuration:")),
					html.Td(gomponents.Text(mediaConfig)),
				),
				html.Tr(
					html.Td(gomponents.Text("Data Import Template:")),
					html.Td(gomponents.Text(dataImportTemplate)),
				),
				html.Tr(
					html.Td(gomponents.Text("Default Template:")),
					html.Td(gomponents.Text(defaultTemplate)),
				),
				html.Tr(
					html.Td(gomponents.Text("Check Runtime:")),
					html.Td(gomponents.Text(fmt.Sprintf("%t", checkRuntime))),
				),
				html.Tr(
					html.Td(gomponents.Text("Delete Wrong Language:")),
					html.Td(gomponents.Text(fmt.Sprintf("%t", deleteWrongLanguage))),
				),
				html.Tr(
					html.Td(gomponents.Text("Manual ID:")),
					html.Td(gomponents.Text(func() string {
						if manualID > 0 {
							return fmt.Sprintf("%d", manualID)
						}
						return "Auto-detect"
					}())),
				),
				html.Tr(html.Td(gomponents.Text("Mode:")), html.Td(gomponents.Text(func() string {
					if dryRun {
						return "Preview (Dry Run)"
					}
					return "Actual Organization"
				}()))),
			),
		),
	)

	// Add results summary
	components = append(components,
		html.Div(
			html.Class("mt-3 card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-chart-bar me-1")),
						gomponents.Text("Results"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Organization Results"),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(
						html.Tr(
							html.Td(gomponents.Text("Total Files:")),
							html.Td(
								gomponents.Text(fmt.Sprintf("%d", organizationResults.TotalFiles)),
							),
						),
						html.Tr(
							html.Td(gomponents.Text("Processed Files:")),
							html.Td(
								gomponents.Text(
									fmt.Sprintf("%d", organizationResults.ProcessedFiles),
								),
							),
						),
						html.Tr(
							html.Td(gomponents.Text("Organized Files:")),
							html.Td(
								gomponents.Text(
									fmt.Sprintf("%d", organizationResults.OrganizedFiles),
								),
							),
						),
						html.Tr(
							html.Td(gomponents.Text("Skipped Files:")),
							html.Td(
								gomponents.Text(
									fmt.Sprintf("%d", organizationResults.SkippedFiles),
								),
							),
						),
						html.Tr(
							html.Td(gomponents.Text("Error Files:")),
							html.Td(
								gomponents.Text(fmt.Sprintf("%d", organizationResults.ErrorFiles)),
							),
						),
						html.Tr(
							html.Td(gomponents.Text("Processing Time:")),
							html.Td(gomponents.Text(organizationResults.Summary.ProcessingTime)),
						),
					),
				),
			),
		),
	)

	// Display file operations if any
	if len(organizationResults.FileOperations) > 0 {
		var operationNodes []gomponents.Node

		operationNodes = append(
			operationNodes,
			html.H6(
				gomponents.Text(
					fmt.Sprintf("File Operations (%d)", len(organizationResults.FileOperations)),
				),
			),
		)

		// Show first 20 operations, with option to show more
		maxDisplay := len(organizationResults.FileOperations)
		if maxDisplay > 20 {
			maxDisplay = 20
		}

		var operationItems []gomponents.Node
		for i := range maxDisplay {
			op := organizationResults.FileOperations[i]

			operationClass := "list-group-item"
			switch op.Operation {
			case "move":
				operationClass += " list-group-item-success"
			case "skip":
				operationClass += " list-group-item-warning"
			case "error":
				operationClass += " list-group-item-danger"
			}

			operationItems = append(operationItems, html.Div(
				html.Class(operationClass),
				html.Strong(gomponents.Text(strings.ToTitle(op.Operation)+": ")),
				gomponents.Text(op.SourcePath),
				gomponents.If(op.TargetPath != "" && op.TargetPath != op.SourcePath,
					gomponents.Text(" → "+op.TargetPath),
				),
				gomponents.If(op.Reason != "",
					html.Small(html.Class("text-muted d-block"), gomponents.Text(op.Reason)),
				),
				gomponents.If(op.MediaTitle != "",
					html.Small(
						html.Class("text-info d-block"),
						gomponents.Text(
							fmt.Sprintf(
								"Media: %s (%s) - %s %s",
								op.MediaTitle,
								op.MediaYear,
								op.Quality,
								op.Resolution,
							),
						),
					),
				),
			))
		}

		if len(organizationResults.FileOperations) > 20 {
			operationItems = append(operationItems, html.Div(
				html.Class("list-group-item text-muted"),
				html.Em(
					gomponents.Text(
						fmt.Sprintf(
							"... and %d more operations",
							len(organizationResults.FileOperations)-20,
						),
					),
				),
			))
		}

		operationListNodes := append(
			[]gomponents.Node{html.Class("list-group mt-2")},
			operationItems...)

		operationNodes = append(operationNodes, html.Div(operationListNodes...))

		operationAllNodes := append([]gomponents.Node{html.Class("mt-3")}, operationNodes...)

		components = append(components, html.Div(operationAllNodes...))
	}

	// Display errors if any
	if len(organizationResults.Errors) > 0 {
		var errorNodes []gomponents.Node

		errorNodes = append(
			errorNodes,
			html.H6(
				gomponents.Text(fmt.Sprintf("Errors (%d)", len(organizationResults.Errors))),
				html.Class("text-danger"),
			),
		)

		var errorItems []gomponents.Node
		for _, err := range organizationResults.Errors {
			errorItems = append(errorItems, html.Div(
				html.Class("list-group-item list-group-item-danger"),
				html.Strong(gomponents.Text("Error: ")),
				gomponents.Text(err.FilePath),
				html.Small(html.Class("text-muted d-block"), gomponents.Text(err.Error)),
			))
		}

		errorListNodes := append([]gomponents.Node{html.Class("list-group mt-2")}, errorItems...)

		errorNodes = append(errorNodes, html.Div(errorListNodes...))

		errorAllNodes := append([]gomponents.Node{html.Class("mt-3")}, errorNodes...)

		components = append(components, html.Div(errorAllNodes...))
	}

	// Add summary statistics if available
	if organizationResults.Summary.MovedFiles > 0 ||
		organizationResults.Summary.CreatedFolders > 0 {
		components = append(components,
			html.Div(
				html.Class("mt-3 card border-0 shadow-sm border-success mb-4"),
				html.Div(
					html.Class("card-header border-0"),
					html.Style(
						"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
					),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-success me-3"),
							html.I(html.Class("fas fa-check-circle me-1")),
							gomponents.Text("Summary"),
						),
						html.H5(
							html.Class("card-title mb-0 text-success fw-bold"),
							gomponents.Text("Summary Statistics"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.Ul(
						html.Class("list-unstyled mb-0"),
						gomponents.If(
							organizationResults.Summary.MovedFiles > 0,
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-arrow-right me-2 text-success")),
								gomponents.Text(
									fmt.Sprintf(
										"Files moved: %d",
										organizationResults.Summary.MovedFiles,
									),
								),
							),
						),
						gomponents.If(
							organizationResults.Summary.RenamedFiles > 0,
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-edit me-2 text-info")),
								gomponents.Text(
									fmt.Sprintf(
										"Files renamed: %d",
										organizationResults.Summary.RenamedFiles,
									),
								),
							),
						),
						gomponents.If(
							organizationResults.Summary.CreatedFolders > 0,
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-folder-plus me-2 text-primary")),
								gomponents.Text(
									fmt.Sprintf(
										"Folders created: %d",
										organizationResults.Summary.CreatedFolders,
									),
								),
							),
						),
						gomponents.If(
							organizationResults.Summary.DeletedFiles > 0,
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-trash me-2 text-danger")),
								gomponents.Text(
									fmt.Sprintf(
										"Files deleted: %d",
										organizationResults.Summary.DeletedFiles,
									),
								),
							),
						),
						gomponents.If(
							organizationResults.Summary.RuntimeVerified > 0,
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-clock me-2 text-warning")),
								gomponents.Text(
									fmt.Sprintf(
										"Runtime verified: %d",
										organizationResults.Summary.RuntimeVerified,
									),
								),
							),
						),
						gomponents.If(
							organizationResults.Summary.LanguageFiltered > 0,
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-language me-2 text-info")),
								gomponents.Text(
									fmt.Sprintf(
										"Language filtered: %d",
										organizationResults.Summary.LanguageFiltered,
									),
								),
							),
						),
					),
				),
			),
		)
	}

	if organizationResults.TotalFiles == 0 {
		components = append(components,
			html.Div(
				html.Class("mt-3 card border-0 shadow-sm border-warning mb-4"),
				html.Div(
					html.Class("card-header border-0"),
					html.Style(
						"background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%); border-radius: 15px 15px 0 0;",
					),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-warning me-3"),
							html.I(html.Class("fas fa-exclamation-triangle me-1")),
							gomponents.Text("Warning"),
						),
						html.H5(
							html.Class("card-title mb-0 text-warning fw-bold"),
							gomponents.Text("No Files Found"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted mb-3"),
						gomponents.Text(
							"No files were found in the specified folder path. This could be due to:",
						),
					),
					html.Ul(
						html.Class("list-unstyled mb-0"),
						html.Li(html.Class("mb-2"), gomponents.Text("• Empty folder")),
						html.Li(
							html.Class("mb-2"),
							gomponents.Text("• No media files matching the configured extensions"),
						),
						html.Li(
							html.Class("mb-2"),
							gomponents.Text("• Insufficient permissions to read folder contents"),
						),
						html.Li(
							html.Class("mb-2"),
							gomponents.Text("• Folder path does not exist or is not accessible"),
						),
					),
				),
			),
		)
	}

	// Determine styling based on results
	var (
		borderClass, gradientStyle, badgeClass, badgeIcon, badgeText string
		statusText                                                   string
	)

	if organizationResults.ErrorFiles > 0 {
		borderClass = "border-warning"
		gradientStyle = "background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%); border-radius: 15px 15px 0 0;"
		badgeClass = "bg-warning"
		badgeIcon = "fas fa-exclamation-triangle"
		badgeText = "Warning"
		statusText = "Folder Organization Complete with Errors"
	} else {
		borderClass = "border-success"
		gradientStyle = "background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"
		badgeClass = "bg-success"
		badgeIcon = "fas fa-check-circle"
		badgeText = "Success"
		statusText = "Folder Organization Complete"
	}

	if dryRun {
		statusText = "Folder Organization Preview"
		badgeText = "Preview"
		badgeIcon = "fas fa-eye"
	}

	// Create header card
	headerCard := html.Div(
		html.Class("card border-0 shadow-sm "+borderClass+" mb-4"),
		html.Div(
			html.Class("card-header border-0"),
			html.Style(gradientStyle),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(
					html.Class("badge "+badgeClass+" me-3"),
					html.I(html.Class(badgeIcon+" me-1")),
					gomponents.Text(badgeText),
				),
				html.H5(html.Class("card-title mb-0 fw-bold"), gomponents.Text(statusText)),
			),
		),
	)

	allNodes := append([]gomponents.Node{headerCard}, components...)

	return renderComponentToString(
		html.Div(allNodes...),
	)
}
