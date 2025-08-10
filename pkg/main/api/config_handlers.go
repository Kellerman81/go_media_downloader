package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	gin "github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// handleConfigUpdate provides a generic pattern for handling config updates
func handleConfigUpdate[T any](c *gin.Context, configType string, parseFunc func(*gin.Context) ([]T, error), saveFunc func([]T) error) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data "+err.Error(), "danger"))
		return
	}

	configs, err := parseFunc(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Configuration validation failed: "+err.Error(), "danger"))
		return
	}

	if err := saveFunc(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save "+configType+" configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert(strings.ToTitle(configType)+" configuration updated successfully!", "success"))
}

// Gin handler to process the form submission
func HandleGeneralConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	updatedConfig := parseGeneralConfig(c)

	// Validate the configuration
	if err := validateGeneralConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update %s", err.Error()), "danger"))
		return
	}

	// Save the configuration
	if err := saveConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Update successful", "success"))
}

// HandleImdbConfigUpdate handles IMDB configuration updates
func HandleImdbConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	updatedConfig := config.GetToml().Imdbindexer

	builder := &ConfigBuilder{context: c, prefix: "imdb"}
	builder.SetStringMultiSelectArray(&updatedConfig.Indexedtypes, "Indexedtypes").
		SetStringArray(&updatedConfig.Indexedlanguages, "Indexedlanguages").
		SetBool(&updatedConfig.Indexfull, "Indexfull").
		SetInt(&updatedConfig.ImdbIDSize, "ImdbIDSize").
		SetInt(&updatedConfig.LoopSize, "LoopSize").
		SetBool(&updatedConfig.UseMemory, "UseMemory").
		SetBool(&updatedConfig.UseCache, "UseCache")

	// Validate the configuration
	if err := validateImdbConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	// Save the configuration
	if err := saveConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("IMDB configuration updated successfully", "success"))
}

// HandleMediaConfigUpdate handles media configuration updates
func HandleMediaConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data "+err.Error(), "danger"))
		return
	}

	logger.LogDynamicany1Any("info", "log post", "form data", c.Request.Form)
	logger.LogDynamicany1Any("info", "log post", "post data", c.Request.PostForm)

	var newConfig config.MediaConfig
	newConfig.Movies = parseMediaConfigs(c, "movies")
	newConfig.Series = parseMediaConfigs(c, "series")

	if err := saveConfig(&newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Media configuration updated successfully", "success"))
}

// HandleDownloaderConfigUpdate handles downloader configuration updates
func HandleDownloaderConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseDownloaderConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse downloader configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveDownloaderConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save downloader configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Downloader configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleDownloaderConfigUpdateGeneric(c *gin.Context) {
	parser := createDownloaderParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save downloader configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Downloader configuration updated successfully! (Generic Parser)", "success"))
}

// Alternative handler using the alternative validation approach
func HandleDownloaderConfigUpdateAlternative(c *gin.Context) {
	// Parse configs using the standard approach
	configs, err := parseDownloaderConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse downloader configuration: "+err.Error(), "danger"))
		return
	}

	// Use alternative validation approach
	if err := validateDownloaderConfigsAlternative(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Alternative validation failed: "+err.Error(), "danger"))
		return
	}

	// Save configs
	if err := saveDownloaderConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save downloader configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Downloader configuration updated successfully! (Alternative Validation)", "success"))
}

// HandleListsConfigUpdate handles lists configuration updates
func HandleListsConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseListsConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse lists configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveListsConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save lists configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Lists configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleListsConfigUpdateGeneric(c *gin.Context) {
	parser := createListsParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save lists configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Lists configuration updated successfully! (Generic Parser)", "success"))
}

// HandleIndexersConfigUpdate handles indexers configuration updates
func HandleIndexersConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseIndexersConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse indexer configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveIndexersConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save indexer configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Indexer configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleIndexersConfigUpdateGeneric(c *gin.Context) {
	parser := createIndexerParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save indexer configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Indexer configuration updated successfully! (Generic Parser)", "success"))
}

// HandlePathsConfigUpdate handles paths configuration updates
func HandlePathsConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parsePathsConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse paths configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := savePathsConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save paths configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Paths configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandlePathsConfigUpdateGeneric(c *gin.Context) {
	parser := createPathsParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save paths configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Paths configuration updated successfully! (Generic Parser)", "success"))
}

// HandleNotificationConfigUpdate handles notification configuration updates
func HandleNotificationConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseNotificationConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse notification configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveNotificationConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save notification configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Notification configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleNotificationConfigUpdateGeneric(c *gin.Context) {
	parser := createNotificationParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save notification configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Notification configuration updated successfully! (Generic Parser)", "success"))
}

// HandleRegexConfigUpdate handles regex configuration updates
func HandleRegexConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseRegexConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse regex configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveRegexConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save regex configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Regex configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleRegexConfigUpdateGeneric(c *gin.Context) {
	parser := createRegexParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save regex configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Regex configuration updated successfully! (Generic Parser)", "success"))
}

// HandleQualityConfigUpdate handles quality configuration updates
func HandleQualityConfigUpdate(c *gin.Context) {
	handleConfigUpdate(c, "quality", parseQualityConfigs, saveQualityConfigs)
}

// HandleSchedulerConfigUpdate handles scheduler configuration updates
func HandleSchedulerConfigUpdate(c *gin.Context) {
	handleConfigUpdate(c, "scheduler", parseSchedulerConfigs, saveSchedulerConfigs)
}

// validateSchedulerConfig validates scheduler configuration

// HandleConfigUpdate - consolidated handler for all config update routes
func HandleConfigUpdate(c *gin.Context) {
	configType := c.Param("configtype")

	switch configType {
	case "general":
		HandleGeneralConfigUpdate(c)
	case "imdb":
		HandleImdbConfigUpdate(c)
	case "quality":
		HandleQualityConfigUpdate(c)
	case "downloader":
		HandleDownloaderConfigUpdate(c)
	case "indexer":
		HandleIndexersConfigUpdate(c)
	case "list":
		HandleListsConfigUpdate(c)
	case "media":
		HandleMediaConfigUpdate(c)
	case "path":
		HandlePathsConfigUpdate(c)
	case "notification":
		HandleNotificationConfigUpdate(c)
	case "regex":
		HandleRegexConfigUpdate(c)
	case "scheduler":
		HandleSchedulerConfigUpdate(c)
	default:
		c.String(http.StatusNotFound, renderAlert("Configuration type not found", "danger"))
	}
}

// HandleTestParse handles test parsing requests
func HandleTestParse(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	filename := c.PostForm("testparse_Filename")
	configKey := c.PostForm("testparse_ConfigKey")
	qualityKey := c.PostForm("testparse_QualityKey")
	usePath, _ := strconv.ParseBool(c.PostForm("testparse_UsePath"))
	useFolder, _ := strconv.ParseBool(c.PostForm("testparse_UseFolder"))

	if filename == "" {
		c.String(http.StatusOK, renderAlert("Please enter a filename to parse", "warning"))
		return
	}

	// Get configuration objects
	cfgp := config.GetSettingsMedia(configKey)
	if cfgp == nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Config key %s not found", configKey), "danger"))
		return
	}

	quality := config.GetSettingsQuality(qualityKey)
	if quality == nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Quality key %s not found", qualityKey), "danger"))
		return
	}

	// Parse the file
	m := parser.ParseFile(filename, usePath, useFolder, cfgp, -1)
	if m == nil {
		c.String(http.StatusOK, renderAlert("ParseFile returned nil - parsing failed", "danger"))
		return
	}

	// Get database IDs and quality mapping
	parser.GetDBIDs(m, cfgp, true)
	parser.GetPriorityMapQual(m, cfgp, quality, false, true)

	// Render results
	c.String(http.StatusOK, renderParseResults(m, filename, configKey, qualityKey))
}

// HandleTraktAuth handles Trakt authentication requests
func HandleTraktAuth(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	action := c.PostForm("action")
	if action == "" {
		// Try to get from JSON body
		var reqData map[string]string
		if err := c.ShouldBindJSON(&reqData); err == nil {
			action = reqData["action"]
		}
	}

	switch action {
	case "get_url":
		handleGetTraktAuthURL(c)
	case "store_token":
		handleStoreTraktToken(c)
	case "refresh_token":
		handleRefreshTraktToken(c)
	case "revoke_token":
		handleRevokeTraktToken(c)
	case "test_api":
		handleTestTraktAPI(c)
	default:
		c.String(http.StatusOK, renderAlert("Invalid action specified", "danger"))
	}
}

// handleGetTraktAuthURL generates and returns the Trakt authorization URL
func handleGetTraktAuthURL(c *gin.Context) {
	// Check if Trakt is configured
	generalConfig := config.GetSettingsGeneral()
	if generalConfig.TraktClientID == "" || generalConfig.TraktClientSecret == "" {
		c.String(http.StatusOK, renderAlert("Trakt Client ID and Secret must be configured in General settings first", "danger"))
		return
	}

	// Get the authorization URL
	authURL := apiexternal.GetTraktAuthURL()
	if authURL == "" {
		c.String(http.StatusOK, renderAlert("Failed to generate authorization URL", "danger"))
		return
	}

	result := Div(
		Class("card border-0 shadow-sm border-info mb-4"),

		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge bg-info me-3"), I(Class("fas fa-link me-1")), Text("Generated")),
				H5(Class("card-title mb-0 text-info fw-bold"), Text("Authorization URL Generated")),
			),
		),
		Div(
			Class("card-body"),
			Div(
				Class("card border-0 shadow-sm border-primary mb-4"),
				Div(
					Class("card-header border-0"),
					Style("background: linear-gradient(135deg, #cfe2ff 0%, #b6d7ff 100%); border-radius: 15px 15px 0 0;"),
					Div(
						Class("d-flex align-items-center justify-content-between"),
						Div(
							Class("d-flex align-items-center"),
							Span(Class("badge bg-primary me-3"), I(Class("fas fa-link me-1")), Text("Generated")),
							H5(Class("card-title mb-0 text-primary fw-bold"), Text("Authorization URL Generated")),
						),
						Span(Class("badge bg-primary"), I(Class("fas fa-external-link-alt me-1")), Text("Ready")),
					),
				),
				Div(
					Class("card-body text-center"),
					P(Class("mb-3"), Style("color: #495057;"), Text("Click the button below to authorize this application on Trakt.tv:")),
					P(Class("mb-0"),
						A(
							Href(authURL),
							Target("_blank"),
							Class("btn btn-primary btn-lg shadow-sm"),
							Style("border-radius: 8px; padding: 0.75rem 2rem;"),
							I(Class("fas fa-external-link-alt me-2")),
							Text("Open Trakt Authorization Page"),
						),
					),
				),
			),

			Details(
				Class("mt-3"),
				Summary(
					Class("btn btn-outline-secondary btn-sm mb-2"),
					Style("border-radius: 6px;"),
					I(Class("fas fa-copy me-1")),
					Text("Or copy URL manually"),
				),
				Div(
					Class("card border-0 mt-2"),
					Style("background: #f8f9fa; border-radius: 6px;"),
					Div(
						Class("card-body p-3"),
						Label(Class("form-label small fw-bold text-muted mb-2"), Text("Authorization URL:")),
						Textarea(
							Class("form-control"),
							Style("background: white; border: 1px solid #dee2e6; border-radius: 6px; font-family: monospace; font-size: 0.875rem;"),
							Attr("readonly", "true"),
							Attr("rows", "3"),
							Text(authURL),
						),
					),
				),
			),

			Div(
				Class("card border-0 mt-3 mb-0"),
				Style("background-color: rgba(13, 110, 253, 0.1); border-radius: 8px;"),
				Div(
					Class("card-body p-3"),
					Div(
						Class("d-flex align-items-start"),
						I(Class("fas fa-info-circle me-2 mt-1"), Style("color: #0d6efd; font-size: 0.9rem;")),
						Small(
							Style("color: #495057; line-height: 1.4;"),
							Strong(Text("Next Step: ")),
							Text("After authorization, copy the code from the redirect URL and use it in Step 2 below."),
						),
					),
				),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// handleStoreTraktToken stores the Trakt token using the authorization code
func handleStoreTraktToken(c *gin.Context) {
	authCode := c.PostForm("trakt_AuthCode")
	if authCode == "" {
		c.String(http.StatusOK, renderAlert("Please enter the authorization code", "warning"))
		return
	}

	// Exchange code for token
	token := apiexternal.GetTraktAuthToken(authCode)
	if token == nil || token.AccessToken == "" {
		c.String(http.StatusOK, renderAlert("Failed to exchange authorization code for token. Please check the code and try again.", "danger"))
		return
	}

	// Store the token
	apiexternal.SetTraktToken(token)
	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: apiexternal.GetTraktToken()})

	result := Div(
		Class("card border-0 shadow-sm border-success mb-4"),

		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge bg-success me-3"), I(Class("fas fa-check-circle me-1")), Text("Success")),
				H5(Class("card-title mb-0 text-success fw-bold"), Text("Trakt Authentication Successful!")),
			),
		),

		Div(
			Class("card-body"),

			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				Div(
					Class("card-body p-3"),
					P(Class("mb-3"), Style("color: #495057;"), Text("Your Trakt token has been stored successfully. The application now has access to your Trakt account.")),

					Div(
						Class("row g-2"),
						Div(
							Class("col-sm-4"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2 text-center"),
									I(Class("fas fa-key mb-1"), Style("color: #28a745;")),
									Div(Class("small fw-bold text-muted"), Text("Access Token")),
									Div(Class("small"), Style("font-family: monospace;"), Text(token.AccessToken[:20]+"...")),
								),
							),
						),
						Div(
							Class("col-sm-4"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2 text-center"),
									I(Class("fas fa-tag mb-1"), Style("color: #28a745;")),
									Div(Class("small fw-bold text-muted"), Text("Token Type")),
									Div(Class("small"), Text(token.TokenType)),
								),
							),
						),
						Div(
							Class("col-sm-4"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2 text-center"),
									I(Class("fas fa-calendar-alt mb-1"), Style("color: #28a745;")),
									Div(Class("small fw-bold text-muted"), Text("Expiry")),
									Div(Class("small"), Text(func() string {
										if token.Expiry.IsZero() {
											return "Never"
										}
										return token.Expiry.Format("2006-01-02 15:04:05")
									}())),
								),
							),
						),
					),
				),
			),

			Div(
				Class("text-center"),
				Button(
					Class("btn btn-success shadow-sm"),
					Style("border-radius: 8px; padding: 0.5rem 1.5rem;"),
					I(Class("fas fa-sync-alt me-2")),
					Text("Reload Page"),
					Attr("onclick", "window.location.reload()"),
				),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// handleRefreshTraktToken refreshes the current Trakt token
func handleRefreshTraktToken(c *gin.Context) {
	currentToken := apiexternal.GetTraktToken()
	if currentToken == nil || currentToken.RefreshToken == "" {
		c.String(http.StatusOK, renderAlert("No refresh token available. Please re-authenticate.", "danger"))
		return
	}

	// Note: Trakt API token refresh would need to be implemented in the apiexternal package
	// For now, we'll just indicate that refresh is not yet implemented
	c.String(http.StatusOK, renderAlert("Token refresh functionality is not yet implemented. Please re-authenticate if needed.", "info"))
}

// handleRevokeTraktToken revokes the current Trakt token
func handleRevokeTraktToken(c *gin.Context) {
	// Clear the token
	apiexternal.SetTraktToken(&oauth2.Token{})
	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: apiexternal.GetTraktToken()})

	result := Div(
		Class("card border-0 shadow-sm border-success mb-4"),

		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge bg-success me-3"), I(Class("fas fa-user-times me-1")), Text("Revoked")),
				H5(Class("card-title mb-0 text-success fw-bold"), Text("Trakt Token Revoked")),
			),
		),

		Div(
			Class("card-body"),

			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				Div(
					Class("card-body p-3 text-center"),
					I(Class("fas fa-shield-alt mb-2"), Style("color: #28a745; font-size: 2rem;")),
					P(Class("mb-0"), Style("color: #495057;"), Text("Your Trakt authentication has been revoked successfully. The application no longer has access to your Trakt account.")),
				),
			),

			Div(
				Class("text-center"),
				Button(
					Class("btn btn-success shadow-sm"),
					Style("border-radius: 8px; padding: 0.5rem 1.5rem;"),
					I(Class("fas fa-sync-alt me-2")),
					Text("Reload Page"),
					Attr("onclick", "window.location.reload()"),
				),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// handleTestTraktAPI tests the Trakt API connection
func handleTestTraktAPI(c *gin.Context) {
	token := apiexternal.GetTraktToken()
	if token == nil || token.AccessToken == "" {
		c.String(http.StatusOK, renderAlert("No valid Trakt token available", "danger"))
		return
	}

	// Test the API by getting popular movies (limit to 5 for testing)
	limit := "5"

	_, movies, _ := apiexternal.TestTraktConnectivity(time.Duration(20*time.Second), &limit)

	if len(movies) == 0 {
		c.String(http.StatusOK, renderAlert("API test failed - no movies returned. Check your network connection and token validity.", "danger"))
		return
	}

	var movieRows []Node
	for i, movie := range movies {
		movieRows = append(movieRows,
			Tr(
				Td(Text(fmt.Sprintf("%d", i+1))),
				Td(Text(movie.Movie.Title)),
				Td(Text(fmt.Sprintf("%d", movie.Movie.Year))),
				Td(Text(fmt.Sprintf("tt%s", movie.Movie.IDs.Imdb))),
				Td(Text(fmt.Sprintf("%d", movie.Movie.IDs.Trakt))),
			),
		)
	}

	result := Div(
		Class("card border-0 shadow-sm border-success mb-4"),

		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge bg-success me-3"), I(Class("fas fa-check-circle me-1")), Text("API Test")),
				H5(Class("card-title mb-0 text-success fw-bold"), Text("Trakt API Test Successful!")),
			),
		),

		Div(
			Class("card-body"),

			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				Div(
					Class("card-body p-3"),
					P(Class("mb-3"), Style("color: #495057;"), Text(fmt.Sprintf("Successfully retrieved %d popular movies from Trakt:", len(movies)))),

					Div(
						Class("table-responsive"),
						Table(
							Class("table table-hover table-sm mb-0"),
							Style("background: transparent;"),
							THead(
								Class("table-success"),
								Tr(
									Th(Style("border-top: none; color: #28a745;"), Text("#")),
									Th(Style("border-top: none; color: #28a745;"), Text("Title")),
									Th(Style("border-top: none; color: #28a745;"), Text("Year")),
									Th(Style("border-top: none; color: #28a745;"), Text("IMDB ID")),
									Th(Style("border-top: none; color: #28a745;"), Text("Trakt ID")),
								),
							),
							TBody(Group(movieRows)),
						),
					),
				),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}
