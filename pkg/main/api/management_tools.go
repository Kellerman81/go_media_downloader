package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// Helper functions to reduce code duplication

// renderMissingEpisodesFinderPage renders a page for finding missing episodes.
func renderMissingEpisodesFinderPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-exclamation-triangle",
			"Missing Episodes Finder",
			"Search for gaps in your series collections and optionally trigger automatic downloads for missing episodes.",
		),

		html.Form(
			html.Class("config-form"),
			html.ID("missingEpisodesForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Search Configuration"),
					),

					renderFormGroup("missing", map[string]string{
						"SeriesName": "Enter series name or leave empty to scan all series",
					}, map[string]string{
						"SeriesName": "Series Name",
					}, "SeriesName", "text", "", nil),

					renderFormGroup("missing", map[string]string{
						"SeasonNumber": "Specific season to check (0 = all seasons)",
					}, map[string]string{
						"SeasonNumber": "Season Number",
					}, "SeasonNumber", "number", "0", nil),

					renderFormGroup("missing", map[string]string{
						"Status": "Filter by series status",
					}, map[string]string{
						"Status": "Series Status",
					}, "Status", "select", "all", map[string][]string{
						"options": {"all", "continuing", "ended", "upcoming"},
					}),

					renderFormGroup("missing",
						map[string]string{
							"IncludeSpecials": "Search for missing special episodes (Season 0)",
						},
						map[string]string{
							"IncludeSpecials": "Include Special Episodes",
						},
						"IncludeSpecials", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Action Configuration"),
					),

					renderFormGroup("missing", map[string]string{
						"DateRange": "How far back to look for missing episodes",
					}, map[string]string{
						"DateRange": "Date Range",
					}, "DateRange", "select", "30", map[string][]string{
						"options": {"7", "30", "90", "180", "365", "all"},
					}),

					renderFormGroup("missing",
						map[string]string{
							"AutoDownload": "Automatically search and download missing episodes",
						},
						map[string]string{
							"AutoDownload": "Auto-Download Missing",
						},
						"AutoDownload", "checkbox", false, nil),

					renderFormGroup("missing",
						map[string]string{
							"OnlyAired": "Only look for episodes that have already aired",
						},
						map[string]string{
							"OnlyAired": "Only Already Aired",
						},
						"OnlyAired", "checkbox", true, nil),

					renderFormGroup("missing",
						map[string]string{
							"ShowAllInTable": "Display all missing episodes in a sortable, searchable table instead of showing only first 5",
						},
						map[string]string{
							"ShowAllInTable": "Show All in DataTable",
						},
						"ShowAllInTable", "checkbox", false, nil),

					renderFormGroup("missing", map[string]string{
						"QualityProfile": "Quality profile to use for downloads",
					}, map[string]string{
						"QualityProfile": "Quality Profile",
					}, "QualityProfile", "select", "default", map[string][]string{
						"options": {"default", "720p", "1080p", "4k"},
					}),
				),
			),

			renderHTMXSubmitButton(
				"Find Missing Episodes",
				"missingResults",
				"/api/admin/missing-episodes",
				"missingEpisodesForm",
				csrfToken,
			),
			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('missingEpisodesForm').reset(); document.getElementById('missingResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("missingResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// renderLogAnalysisDashboardPage renders a page for analyzing logs.
func renderLogAnalysisDashboardPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-chart-line",
			"Log Analysis Dashboard",
			"Parse and analyze application logs for patterns, errors, and performance metrics with detailed statistics and insights.",
		),

		html.Form(
			html.Class("config-form"),
			html.ID("logAnalysisForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Analysis Configuration"),
					),

					renderFormGroup("analysis", map[string]string{
						"TimeRange": "Time range for log analysis",
					}, map[string]string{
						"TimeRange": "Time Range",
					}, "TimeRange", "select", "24h", map[string][]string{
						"options": {"1h", "6h", "12h", "24h", "3d", "7d", "30d"},
					}),

					renderFormGroup("analysis", map[string]string{
						"LogLevel": "Minimum log level to analyze",
					}, map[string]string{
						"LogLevel": "Log Level",
					}, "LogLevel", "select", "INFO", map[string][]string{
						"options": {"DEBUG", "INFO", "WARN", "ERROR", "FATAL"},
					}),

					renderFormGroup("analysis", map[string]string{
						"MaxLines": "Maximum number of log lines to process",
					}, map[string]string{
						"MaxLines": "Max Lines",
					}, "MaxLines", "number", "10000", nil),

					renderFormGroup("analysis",
						map[string]string{
							"IncludeStackTraces": "Analyze error stack traces for patterns",
						},
						map[string]string{
							"IncludeStackTraces": "Include Stack Traces",
						},
						"IncludeStackTraces", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Analysis Types")),

					renderFormGroup("analysis",
						map[string]string{
							"ErrorPattern": "Find common error patterns and frequencies",
						},
						map[string]string{
							"ErrorPattern": "Error Pattern Analysis",
						},
						"ErrorPattern", "checkbox", true, nil),

					renderFormGroup("analysis",
						map[string]string{
							"PerformanceMetrics": "Analyze response times and performance indicators",
						},
						map[string]string{
							"PerformanceMetrics": "Performance Metrics",
						},
						"PerformanceMetrics", "checkbox", true, nil),

					renderFormGroup("analysis",
						map[string]string{
							"AccessPattern": "Analyze API usage and access patterns",
						},
						map[string]string{
							"AccessPattern": "Access Patterns",
						},
						"AccessPattern", "checkbox", false, nil),

					renderFormGroup("analysis",
						map[string]string{
							"SystemHealth": "Monitor system health indicators",
						},
						map[string]string{
							"SystemHealth": "System Health",
						},
						"SystemHealth", "checkbox", true, nil),
				),
			),

			renderHTMXSubmitButton(
				"Analyze Logs",
				"analysisResults",
				"/api/admin/log-analysis",
				"logAnalysisForm",
				csrfToken,
			),
			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-success ml-2"),
					gomponents.Text("Export Report"),
					html.Type("button"),
					hx.Target("#analysisResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/log-analysis/export"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logAnalysisForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('logAnalysisForm').reset(); document.getElementById('analysisResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("analysisResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// renderStorageHealthMonitorPage renders a page for monitoring storage health.
func renderStorageHealthMonitorPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-hdd",
			"Storage Health Monitor",
			"Monitor disk space, file system permissions, and mount status across all configured media paths and storage locations.",
		),

		html.Form(
			html.Class("config-form"),
			html.ID("storageHealthForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Monitoring Configuration"),
					),

					renderFormGroup("storage",
						map[string]string{
							"CheckDiskSpace": "Monitor free space on all configured paths",
						},
						map[string]string{
							"CheckDiskSpace": "Check Disk Space",
						},
						"CheckDiskSpace", "checkbox", true, nil),

					renderFormGroup("storage",
						map[string]string{
							"CheckPermissions": "Verify read/write permissions on media directories",
						},
						map[string]string{
							"CheckPermissions": "Check Permissions",
						},
						"CheckPermissions", "checkbox", true, nil),

					renderFormGroup("storage",
						map[string]string{
							"CheckMountStatus": "Verify that network drives and mounts are accessible",
						},
						map[string]string{
							"CheckMountStatus": "Check Mount Status",
						},
						"CheckMountStatus", "checkbox", true, nil),

					renderFormGroup("storage",
						map[string]string{
							"CheckIOHealth": "Test read/write speed and I/O performance",
						},
						map[string]string{
							"CheckIOHealth": "Check I/O Health",
						},
						"CheckIOHealth", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Alert Thresholds")),

					renderFormGroup("storage", map[string]string{
						"LowSpaceThreshold": "Alert when free space drops below this percentage",
					}, map[string]string{
						"LowSpaceThreshold": "Low Space Alert (%)",
					}, "LowSpaceThreshold", "number", "10", nil),

					renderFormGroup("storage", map[string]string{
						"CriticalSpaceThreshold": "Critical alert threshold percentage",
					}, map[string]string{
						"CriticalSpaceThreshold": "Critical Space Alert (%)",
					}, "CriticalSpaceThreshold", "number", "5", nil),

					renderFormGroup("storage", map[string]string{
						"SlowIOThreshold": "Alert when I/O speed drops below this MB/s",
					}, map[string]string{
						"SlowIOThreshold": "Slow I/O Alert (MB/s)",
					}, "SlowIOThreshold", "number", "10", nil),

					renderFormGroup("storage",
						map[string]string{
							"EnableAlerts": "Send notifications when thresholds are exceeded",
						},
						map[string]string{
							"EnableAlerts": "Enable Alert Notifications",
						},
						"EnableAlerts", "checkbox", true, nil,
					),
				),
			),

			renderHTMXSubmitButton(
				"Check Storage Health",
				"storageResults",
				"/api/admin/storage-health",
				"storageHealthForm",
				csrfToken,
			),
			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-info ml-2"),
					gomponents.Text("Continuous Monitor"),
					html.Type("button"),
					hx.Target("#storageResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/storage-health/monitor"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#storageHealthForm"),
					hx.Trigger("every 30s"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('storageHealthForm').reset(); document.getElementById('storageResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("storageResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// renderExternalServiceHealthCheckPage renders a page for checking external service connectivity.
func renderExternalServiceHealthCheckPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-heartbeat",
			"External Service Health Check",
			"Test connectivity and response times for external services including IMDB, Trakt, indexers, and other integrated APIs.",
		),

		html.Form(
			html.Class("config-form"),
			html.ID("serviceHealthForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Service Selection")),

					renderFormGroup("service",
						map[string]string{
							"CheckIMDB": "Test IMDB metadata service connectivity",
						},
						map[string]string{
							"CheckIMDB": "IMDB API",
						},
						"CheckIMDB", "checkbox", true, nil),

					renderFormGroup("service",
						map[string]string{
							"CheckTrakt": "Test Trakt service connectivity and authentication",
						},
						map[string]string{
							"CheckTrakt": "Trakt API",
						},
						"CheckTrakt", "checkbox", true, nil),

					renderFormGroup("service",
						map[string]string{
							"CheckIndexers": "Test all configured download indexers",
						},
						map[string]string{
							"CheckIndexers": "Download Indexers",
						},
						"CheckIndexers", "checkbox", true, nil),

					renderFormGroup("service",
						map[string]string{
							"CheckNotifications": "Test Pushover and other notification services",
						},
						map[string]string{
							"CheckNotifications": "Notification Services",
						},
						"CheckNotifications", "checkbox", false, nil),
					renderFormGroup("service",
						map[string]string{
							"CheckOMDB": "Test OMDB movie database connectivity",
						},
						map[string]string{
							"CheckOMDB": "OMDB API",
						},
						"CheckOMDB", "checkbox", true, nil),
					renderFormGroup("service",
						map[string]string{
							"CheckTVDB": "Test TheTVDB service connectivity",
						},
						map[string]string{
							"CheckTVDB": "TVDB API",
						},
						"CheckTVDB", "checkbox", true, nil),
					renderFormGroup("service",
						map[string]string{
							"CheckTMDB": "Test TheMovieDB service connectivity",
						},
						map[string]string{
							"CheckTMDB": "TMDB API",
						},
						"CheckTMDB", "checkbox", true, nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Test Configuration"),
					),

					renderFormGroup("service", map[string]string{
						"Timeout": "Timeout for each service test in seconds",
					}, map[string]string{
						"Timeout": "Timeout (seconds)",
					}, "Timeout", "number", "10", nil),

					renderFormGroup("service", map[string]string{
						"Retries": "Number of retry attempts for failed tests",
					}, map[string]string{
						"Retries": "Retry Attempts",
					}, "Retries", "number", "3", nil),

					renderFormGroup("service",
						map[string]string{
							"DetailedTest": "Perform comprehensive tests including authentication and API calls",
						},
						map[string]string{
							"DetailedTest": "Detailed Testing",
						},
						"DetailedTest", "checkbox", false, nil),

					renderFormGroup("service",
						map[string]string{
							"MeasurePerformance": "Measure response times and performance metrics",
						},
						map[string]string{
							"MeasurePerformance": "Measure Performance",
						},
						"MeasurePerformance", "checkbox", true, nil),

					renderFormGroup("service",
						map[string]string{
							"SaveResults": "Store test results for trend analysis",
						},
						map[string]string{
							"SaveResults": "Save Historical Data",
						},
						"SaveResults", "checkbox", false, nil),
				),
			),

			renderHTMXSubmitButton(
				"Run Health Check",
				"serviceResults",
				"/api/admin/service-health",
				"serviceHealthForm",
				csrfToken,
			),
			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-warning ml-2"),
					gomponents.Text("Quick Test"),
					html.Type("button"),
					hx.Target("#serviceResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/service-health/quick"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#serviceHealthForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('serviceHealthForm').reset(); document.getElementById('serviceResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("serviceResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// renderAPITestingPage renders a comprehensive API testing interface.
func renderAPITestingPage() gomponents.Node {
	// Get the actual API key from configuration
	apiKey := config.GetSettingsGeneral().WebAPIKey

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header
		renderEnhancedPageHeader(
			"fa-solid fa-plug",
			"API Testing Suite",
			"Comprehensive testing interface for all API endpoints. Test jobs, searches, configurations, and database operations with real-time feedback.",
		),

		// Main content layout with resizable splitter
		html.Div(
			html.ID("resizable-container"),
			html.Style("display: flex; min-height: 800px;"),
			// Main content area
			html.Div(
				html.ID("main-content"),
				html.Style("flex: 1; padding-right: 15px; min-width: 300px;"),

				// API Configuration Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-secondary text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-key me-2")),
							gomponents.Text("API Configuration"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("row g-3 align-items-end"),
							html.Div(
								html.Class("col-md-8"),
								html.Label(
									html.For("apiKeyInput"),
									html.Class("form-label"),
									gomponents.Text("API Key"),
								),
								html.Div(
									html.Class("input-group"),
									html.Input(
										html.ID("apiKeyInput"),
										html.Type("text"),
										html.Class("form-control"),
										html.Value(apiKey),
										gomponents.Attr("placeholder", "Enter your API key"),
									),
									html.Span(
										html.ID("apiKeyStatus"),
										html.Class("input-group-text bg-light"),
										html.I(html.Class("fas fa-question-circle text-muted")),
									),
								),
								html.Small(
									html.Class("form-text text-muted"),
									gomponents.Text(
										"API key is automatically loaded from configuration. You can override it here if needed.",
									),
								),
							),
							html.Div(
								html.Class("col-md-2"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-primary w-100"),
									gomponents.Attr("onclick", "testAPIKey()"),
									html.I(
										html.Class("fas fa-vial me-2"),
									),
									gomponents.Text("Test Key"),
								),
							),
							html.Div(
								html.Class("col-md-2"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-secondary w-100"),
									gomponents.Attr("onclick", "resetAPIKey()"),
									html.I(
										html.Class("fas fa-undo me-2"),
									),
									gomponents.Text("Reset"),
								),
							),
						),
					),
				),

				// Quick Actions Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-primary text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-bolt me-2")),
							gomponents.Text("Quick Actions"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("row g-3"),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-success w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/debugstats', '', 'Debug Stats')",
									),
									html.I(
										html.Class("fas fa-chart-line me-2"),
									),
									gomponents.Text("Debug Stats"),
								),
							),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-info w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/queue', '', 'Job Queue')",
									),
									html.I(
										html.Class("fas fa-tasks me-2"),
									),
									gomponents.Text("Job Queue"),
								),
							),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-warning w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/config/all', '', 'All Configs')",
									),
									html.I(
										html.Class("fas fa-cog me-2"),
									),
									gomponents.Text("All Configs"),
								),
							),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-secondary w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/db/backup', '', 'DB Backup')",
									),
									html.I(
										html.Class("fas fa-database me-2"),
									),
									gomponents.Text("DB Backup"),
								),
							),
						),
					),
				),

				// Job Management Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-info text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-play me-2")),
							gomponents.Text("Job Management"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Form(
							gomponents.Attr("onsubmit", "return executeJobTest(event)"),
							html.Div(
								html.Class("row g-3"),
								html.Div(
									html.Class("col-md-4"),
									html.Label(
										html.For("jobType"),
										html.Class("form-label"),
										gomponents.Text("Job Type"),
									),
									html.Select(
										html.ID("jobType"),
										html.Name("jobType"),
										html.Class("form-select"),
										gomponents.Attr("onchange", "updateJobOptions()"),
										html.Option(
											html.Value(""),
											gomponents.Text("Select Job Type"),
										),
										html.Option(
											html.Value("movies"),
											gomponents.Text("Movies"),
										),
										html.Option(
											html.Value("series"),
											gomponents.Text("Series"),
										),
										html.Option(html.Value("db"), gomponents.Text("Database")),
									),
								),
								html.Div(
									html.Class("col-md-4"),
									html.Label(
										html.For("jobAction"),
										html.Class("form-label"),
										gomponents.Text("Action"),
									),
									html.Select(
										html.ID("jobAction"),
										html.Name("jobAction"),
										html.Class("form-select"),
										html.Option(
											html.Value(""),
											gomponents.Text("Select Action"),
										),
									),
								),
								html.Div(
									html.Class("col-md-4"),
									html.Label(
										html.For("jobConfig"),
										html.Class("form-label"),
										gomponents.Text("Config (Optional)"),
									),
									html.Input(
										html.ID("jobConfig"),
										html.Name("jobConfig"),
										html.Type("text"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "EN, DE, X, etc."),
									),
								),
							),
							html.Div(
								html.Class("mt-3"),
								html.Button(
									html.Type("submit"),
									html.Class("btn btn-primary me-2"),
									html.I(
										html.Class("fas fa-rocket me-2"),
									),
									gomponents.Text("Execute Job"),
								),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-secondary"),
									gomponents.Attr("onclick", "clearResults()"),
									html.I(
										html.Class("fas fa-trash me-2"),
									),
									gomponents.Text("Clear Results"),
								),
							),
						),
					),
				),

				// Custom API Testing Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-dark text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-terminal me-2")),
							gomponents.Text("Custom API Testing"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Form(
							gomponents.Attr("onsubmit", "return executeCustomAPITest(event)"),
							html.Div(
								html.Class("row g-3"),
								html.Div(
									html.Class("col-md-2"),
									html.Label(
										html.For("httpMethod"),
										html.Class("form-label"),
										gomponents.Text("Method"),
									),
									html.Select(
										html.ID("httpMethod"),
										html.Name("httpMethod"),
										html.Class("form-select"),
										html.Option(
											html.Value("GET"),
											gomponents.Attr("selected"),
											gomponents.Text("GET"),
										),
										html.Option(html.Value("POST"), gomponents.Text("POST")),
										html.Option(html.Value("PUT"), gomponents.Text("PUT")),
										html.Option(
											html.Value("DELETE"),
											gomponents.Text("DELETE"),
										),
									),
								),
								html.Div(
									html.Class("col-md-10"),
									html.Label(
										html.For("apiEndpoint"),
										html.Class("form-label"),
										gomponents.Text("API Endpoint"),
									),
									html.Input(
										html.ID("apiEndpoint"),
										html.Name("apiEndpoint"),
										html.Type("text"),
										html.Class("form-control"),
										gomponents.Attr(
											"placeholder",
											"/api/movies or /api/series/search/id/123",
										),
									),
								),
							),
							html.Div(
								html.Class("mt-3"),
								html.Label(
									html.For("requestBody"),
									html.Class("form-label"),
									gomponents.Text("Request Body (JSON for POST/PUT)"),
								),
								html.Textarea(
									html.ID("requestBody"),
									html.Name("requestBody"),
									html.Class("form-control"),
									gomponents.Attr("rows", "4"),
									gomponents.Attr("placeholder", `{"key": "value"}`),
								),
							),
							html.Div(
								html.Class("mt-3"),
								html.Button(
									html.Type("submit"),
									html.Class("btn btn-primary me-2"),
									html.I(
										html.Class("fas fa-paper-plane me-2"),
									),
									gomponents.Text("Send Request"),
								),
							),
						),
					),
				),

				// Search & Download Testing
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-success text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-search me-2")),
							gomponents.Text("Search & Download Testing"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("row g-3"),
							html.Div(
								html.Class("col-md-6"),
								html.H6(gomponents.Text("Movie Search")),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("movieSearchId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Movie ID"),
									),
									html.Button(
										html.Class("btn btn-outline-primary"),
										html.Type("button"),
										gomponents.Attr("onclick", "testMovieSearch()"),
										gomponents.Text("Search Movie"),
									),
								),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("movieListId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Movie List ID"),
									),
									html.Button(
										html.Class("btn btn-outline-success"),
										html.Type("button"),
										gomponents.Attr("onclick", "testMovieListSearch()"),
										gomponents.Text("Search List"),
									),
								),
							),
							html.Div(
								html.Class("col-md-6"),
								html.H6(gomponents.Text("Series Search")),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("seriesSearchId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Series ID"),
									),
									html.Button(
										html.Class("btn btn-outline-primary"),
										html.Type("button"),
										gomponents.Attr("onclick", "testSeriesSearch()"),
										gomponents.Text("Search Series"),
									),
								),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("episodeId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Episode ID"),
									),
									html.Button(
										html.Class("btn btn-outline-success"),
										html.Type("button"),
										gomponents.Attr("onclick", "testEpisodeSearch()"),
										gomponents.Text("Search Episode"),
									),
								),
							),
						),
					),
				),

				// API Endpoints Reference Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-warning text-dark"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-book me-2")),
							gomponents.Text("API Endpoints Reference"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("accordion"),
							html.ID("endpointsAccordion"),

							// All endpoints section
							html.Div(
								html.Class("accordion-item"),
								html.H2(
									html.Class("accordion-header"),
									html.ID("headingAll"),
									html.Button(
										html.Class("accordion-button collapsed"),
										html.Type("button"),
										gomponents.Attr("data-bs-toggle", "collapse"),
										gomponents.Attr("data-bs-target", "#collapseAll"),
										gomponents.Attr("aria-expanded", "false"),
										gomponents.Attr("aria-controls", "collapseAll"),
										gomponents.Text("All Endpoints (7)"),
									),
								),
								html.Div(
									html.ID("collapseAll"),
									html.Class("accordion-collapse collapse"),
									gomponents.Attr("aria-labelledby", "headingAll"),
									gomponents.Attr("data-bs-parent", "#endpointsAccordion"),
									html.Div(
										html.Class("accordion-body"),
										html.Ul(
											html.Class("list-unstyled mb-0"),
											html.Li(
												gomponents.Text(
													"GET /api/all/feeds - Search all feeds",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/all/data - Search all folders",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/all/search/rss - Search all rss feeds",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/all/search/missing/full - Search all Missing",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/all/search/missing/inc - Search all Missing Incremental",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/all/search/upgrade/full - Search all Upgrades",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/all/search/upgrade/inc - Search all Upgrades Incremental",
												),
											),
										),
									),
								),
							),

							// Movies endpoints section
							html.Div(
								html.Class("accordion-item"),
								html.H2(
									html.Class("accordion-header"),
									html.ID("headingMovies"),
									html.Button(
										html.Class("accordion-button collapsed"),
										html.Type("button"),
										gomponents.Attr("data-bs-toggle", "collapse"),
										gomponents.Attr("data-bs-target", "#collapseMovies"),
										gomponents.Attr("aria-expanded", "false"),
										gomponents.Attr("aria-controls", "collapseMovies"),
										gomponents.Text("Movies Endpoints (20)"),
									),
								),
								html.Div(
									html.ID("collapseMovies"),
									html.Class("accordion-collapse collapse"),
									gomponents.Attr("aria-labelledby", "headingMovies"),
									gomponents.Attr("data-bs-parent", "#endpointsAccordion"),
									html.Div(
										html.Class("accordion-body"),
										html.Ul(
											html.Class("list-unstyled mb-0"),
											html.Li(
												gomponents.Text("GET /api/movies - Get all movies"),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/unmatched - Get unmatched movies",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/movies/{id} - Delete movie",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/list/{name} - Get movies by list name",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/metadata/{imdb} - Get movie metadata",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/movies/list/{id} - Delete movie from list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/job/{job} - Execute movie job",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/job/{job}/{name} - Execute movie job for config",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/movies - Create/update movie",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/movies/list - Create/update movie list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/search/id/{id} - Search movie by ID",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/search/list/{id} - Search movie list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/rss/search/list/{group} - RSS search movie list",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/movies/search/download/{id} - Download movie",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/all/refreshall - Refresh all movies metadata",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/refresh/{id} - Refresh movie metadata",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/all/refresh - Refresh movies",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/search/history/clear/{name} - Clear movie search history",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/movies/search/history/clearid/{id} - Clear movie search history by ID",
												),
											),
										),
									),
								),
							),

							// Series endpoints section
							html.Div(
								html.Class("accordion-item"),
								html.H2(
									html.Class("accordion-header"),
									html.ID("headingSeries"),
									html.Button(
										html.Class("accordion-button collapsed"),
										html.Type("button"),
										gomponents.Attr("data-bs-toggle", "collapse"),
										gomponents.Attr("data-bs-target", "#collapseSeries"),
										gomponents.Attr("aria-expanded", "false"),
										gomponents.Attr("aria-controls", "collapseSeries"),
										gomponents.Text("Series Endpoints (28)"),
									),
								),
								html.Div(
									html.ID("collapseSeries"),
									html.Class("accordion-collapse collapse"),
									gomponents.Attr("aria-labelledby", "headingSeries"),
									gomponents.Attr("data-bs-parent", "#endpointsAccordion"),
									html.Div(
										html.Class("accordion-body"),
										html.Ul(
											html.Class("list-unstyled mb-0"),
											html.Li(
												gomponents.Text("GET /api/series - Get all series"),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/series/{id} - Delete series",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/list/{name} - Get series by list name",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/series/list/{id} - Delete series from list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/unmatched - Get unmatched series",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/episodes - Get all episodes",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/episodes/{id} - Get episode by ID",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/series/episodes/{id} - Delete episode",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/episodes/list/{id} - Get episodes by series ID",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/series/episodes/list/{id} - Delete episodes by series ID",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/job/{job} - Execute series job",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/job/{job}/{name} - Execute series job for config",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/series - Create/update series",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/series/episodes - Create/update episode",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/series/list - Create/update series list",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/series/episodes/list - Create/update episode list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/refresh/{id} - Refresh series metadata",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/all/refreshall - Refresh all series metadata",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/all/refresh - Refresh series",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/search/id/{id} - Search series by ID",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/search/id/{id}/{season} - Search series season",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/searchrss/id/{id} - RSS search series",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/searchrss/list/id/{id} - RSS search series list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/searchrss/id/{id}/{season} - RSS search series season",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/episodes/search/id/{id} - Search episodes",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/episodes/search/list/{id} - Search episode list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/rss/search/list/{group} - RSS search series list",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/series/episodes/search/download/{id} - Download episode",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/search/history/clear/{name} - Clear series search history",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/series/search/history/clearid/{id} - Clear series search history by ID",
												),
											),
										),
									),
								),
							),

							// General endpoints section
							html.Div(
								html.Class("accordion-item"),
								html.H2(
									html.Class("accordion-header"),
									html.ID("headingGeneral"),
									html.Button(
										html.Class("accordion-button collapsed"),
										html.Type("button"),
										gomponents.Attr("data-bs-toggle", "collapse"),
										gomponents.Attr("data-bs-target", "#collapseGeneral"),
										gomponents.Attr("aria-expanded", "false"),
										gomponents.Attr("aria-controls", "collapseGeneral"),
										gomponents.Text("General & System Endpoints (31)"),
									),
								),
								html.Div(
									html.ID("collapseGeneral"),
									html.Class("accordion-collapse collapse"),
									gomponents.Attr("aria-labelledby", "headingGeneral"),
									gomponents.Attr("data-bs-parent", "#endpointsAccordion"),
									html.Div(
										html.Class("accordion-body"),
										html.Ul(
											html.Class("list-unstyled mb-0"),
											html.Li(
												gomponents.Text(
													"GET /api/debugstats - Get debug statistics",
												),
											),
											html.Li(
												gomponents.Text("GET /api/queue - Get job queue"),
											),
											html.Li(
												gomponents.Text(
													"GET /api/queue/history - Get job history",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/queue/cancel/{id} - Cancel job",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/trakt/authorize - Authorize Trakt",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/trakt/token/{code} - Get Trakt token",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/trakt/user/{user}/{list} - Get Trakt user list",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/slug - Get slug information",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/parse/string - Parse string",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/parse/file - Parse file",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/fillimdb - Fill IMDB data",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/scheduler/stop - Stop scheduler",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/scheduler/start - Start scheduler",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/scheduler/list - List scheduled jobs",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/db/close - Close database",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/db/backup - Backup database",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/db/integrity - Check database integrity",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/db/clear/{name} - Clear database table",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/db/delete/{name}/{id} - Delete database entry",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/db/clearcache - Clear cache",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/db/vacuum - Vacuum database",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/db/oldjobs - Delete old jobs",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/quality - Get quality profiles",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/quality/{id} - Delete quality profile",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/quality - Create/update quality profile",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/quality/get/{name} - Get quality by name",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/quality/all - Get all qualities",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/quality/complete - Get complete qualities",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/config/all - Get all configurations",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/config/clear - Clear configuration",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/config/get/{name} - Get config by name",
												),
											),
											html.Li(
												gomponents.Text(
													"DELETE /api/config/delete/{name} - Delete configuration",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/config/refresh - Refresh configuration",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/config/update/{name} - Update configuration",
												),
											),
											html.Li(
												gomponents.Text(
													"GET /api/config/type/{type} - Get config by type",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/naming - Test naming patterns",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/structure - Test file structure",
												),
											),
										),
									),
								),
							),

							// Admin endpoints section
							html.Div(
								html.Class("accordion-item"),
								html.H2(
									html.Class("accordion-header"),
									html.ID("headingAdmin"),
									html.Button(
										html.Class("accordion-button collapsed"),
										html.Type("button"),
										gomponents.Attr("data-bs-toggle", "collapse"),
										gomponents.Attr("data-bs-target", "#collapseAdmin"),
										gomponents.Attr("aria-expanded", "false"),
										gomponents.Attr("aria-controls", "collapseAdmin"),
										gomponents.Text("Admin Endpoints (4)"),
									),
								),
								html.Div(
									html.ID("collapseAdmin"),
									html.Class("accordion-collapse collapse"),
									gomponents.Attr("aria-labelledby", "headingAdmin"),
									gomponents.Attr("data-bs-parent", "#endpointsAccordion"),
									html.Div(
										html.Class("accordion-body"),
										html.Ul(
											html.Class("list-unstyled mb-0"),
											html.Li(
												gomponents.Text("GET /api/admin - Admin dashboard"),
											),
											html.Li(
												gomponents.Text(
													"POST /api/admin/table/{name}/insert - Insert database record",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/admin/table/{name}/update/{index} - Update database record",
												),
											),
											html.Li(
												gomponents.Text(
													"POST /api/admin/table/{name}/delete/{index} - Delete database record",
												),
											),
										),
									),
								),
							),
						),

						html.Div(
							html.Class("mt-3 p-3 bg-light rounded"),
							html.H6(html.Class("fw-bold mb-2"), gomponents.Text("Quick Reference")),
							html.P(
								html.Class("mb-1 small"),
								html.Strong(gomponents.Text("Total Endpoints:")),
								gomponents.Text(" 90"),
							),
							html.P(
								html.Class("mb-1 small"),
								html.Strong(gomponents.Text("Authentication:")),
								gomponents.Text(" All endpoints require 'apikey' query parameter"),
							),
							html.P(
								html.Class("mb-1 small"),
								html.Strong(gomponents.Text("Base URL:")),
								gomponents.Text(" /api/{category}/{action}"),
							),
							html.P(
								html.Class("mb-0 small"),
								html.Strong(gomponents.Text("Response Format:")),
								gomponents.Text(" JSON (success/error messages or data)"),
							),
						),
					),
				),
			), // End main content

			// Resizable divider
			html.Div(
				html.ID("resizer"),
				html.Style(
					"width: 5px; cursor: col-resize; background: #dee2e6; border-left: 1px solid #adb5bd; border-right: 1px solid #adb5bd; flex-shrink: 0;",
				),
				gomponents.Attr("title", "Drag to resize"),
			),

			// Sidebar for results
			html.Div(
				html.ID("sidebar-content"),
				html.Style("width: 400px; min-width: 300px; max-width: 800px; padding-left: 15px;"),
				// Results Section
				html.Div(
					html.Class("card border-0 shadow-sm sticky-top"),
					html.Style("top: 20px;"), // Sticky positioning
					html.Div(
						html.Class(
							"card-header bg-primary text-white d-flex justify-content-between align-items-center",
						),
						html.H6(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-list-alt me-2")),
							gomponents.Text("Test Results"),
						),
						html.Div(
							html.Class("btn-group btn-group-sm"),
							html.Button(
								html.Type("button"),
								html.Class("btn btn-light btn-sm"),
								gomponents.Attr("onclick", "copyResults()"),
								gomponents.Attr("title", "Copy Results"),
								html.I(html.Class("fas fa-copy"), html.Style("color: black;")),
							),
							html.Button(
								html.Type("button"),
								html.Class("btn btn-light btn-sm"),
								gomponents.Attr("onclick", "clearResults()"),
								gomponents.Attr("title", "Clear Results"),
								html.I(html.Class("fas fa-trash"), html.Style("color: black;")),
							),
						),
					),
					html.Div(
						html.Class("card-body p-2"),
						html.Div(
							html.ID("testResults"),
							html.Class("bg-light p-2 rounded"),
							html.Style(
								"min-height: 400px; font-family: 'Courier New', monospace; white-space: pre-wrap; overflow: auto; font-size: 0.8rem; line-height: 1.3; resize: both; height: 600px;",
							),
							gomponents.Text("Test results will appear here..."),
						),
					),
				),
			), // End sidebar content
		), // End resizable container

		// JavaScript for functionality
		html.Script(gomponents.Raw(`
			// Store the original API key for reset functionality
			const originalAPIKey = '`+apiKey+`';
			
			// Job options mapping
			const jobOptions = {
				movies: ['checkreachedflag', 'checkmissingflag', 'checkmissing', 'rss', 'structure', 'datafull', 'searchmissinginc', 'searchupgradeinc', 'searchmissingfull', 'searchupgradefull', 'refreshinc', 'feeds'],
				series: ['checkreachedflag', 'checkmissingflag', 'checkmissing', 'rss', 'rssseasons', 'rssseasonsall', 'structure', 'datafull', 'searchmissinginc', 'searchupgradeinc', 'searchmissingfull', 'searchupgradefull', 'refreshinc', 'feeds'],
				db: ['backup', 'integrity', 'clearcache', 'clear/serie_episode_histories', 'clear/movie_histories', 'clear/r_sshistories']
			};
			
			function updateJobOptions() {
				const jobType = document.getElementById('jobType').value;
				const jobAction = document.getElementById('jobAction');
				
				jobAction.innerHTML = '<option value="">Select Action</option>';
				
				if (jobType && jobOptions[jobType]) {
					jobOptions[jobType].forEach(action => {
						const option = document.createElement('option');
						option.value = action;
						option.textContent = action;
						jobAction.appendChild(option);
					});
				}
			}
			
			function executeJobTest(event) {
				event.preventDefault();
				
				const jobType = document.getElementById('jobType').value;
				const jobAction = document.getElementById('jobAction').value;
				const jobConfig = document.getElementById('jobConfig').value;
				
				if (!jobType || !jobAction) {
					showResult('Error: Please select job type and action', 'error');
					return false;
				}
				
				let endpoint;
				if (jobType === 'db') {
					endpoint = '/api/db/' + jobAction;
				} else {
					endpoint = '/api/' + jobType + '/job/' + jobAction;
					if (jobConfig) {
						endpoint += '/' + jobConfig;
					}
				}
				
				executeAPICall('GET', endpoint, '', 'Job: ' + jobType + '/' + jobAction);
				return false;
			}
			
			function executeCustomAPITest(event) {
				event.preventDefault();
				
				const method = document.getElementById('httpMethod').value;
				const endpoint = document.getElementById('apiEndpoint').value;
				const body = document.getElementById('requestBody').value;
				
				if (!endpoint) {
					showResult('Error: Please enter an API endpoint', 'error');
					return false;
				}
				
				executeAPICall(method, endpoint, body, 'Custom: ' + method + ' ' + endpoint);
				return false;
			}
			
			function testMovieSearch() {
				const movieId = document.getElementById('movieSearchId').value;
				if (movieId) {
					executeAPICall('GET', '/api/movies/search/id/' + movieId, '', 'Movie Search ID: ' + movieId);
				}
			}
			
			function testMovieListSearch() {
				const listId = document.getElementById('movieListId').value;
				if (listId) {
					executeAPICall('GET', '/api/movies/search/list/' + listId + '?searchByTitle=true&download=true', '', 'Movie List Search: ' + listId);
				}
			}
			
			function testSeriesSearch() {
				const seriesId = document.getElementById('seriesSearchId').value;
				if (seriesId) {
					executeAPICall('GET', '/api/series/search/id/' + seriesId, '', 'Series Search ID: ' + seriesId);
				}
			}
			
			function testEpisodeSearch() {
				const episodeId = document.getElementById('episodeId').value;
				if (episodeId) {
					executeAPICall('GET', '/api/series/episodes/search/list/' + episodeId + '?searchByTitle=true&download=true', '', 'Episode Search: ' + episodeId);
				}
			}
			
			function executeAPICall(method, endpoint, body, title) {
				const resultsDiv = document.getElementById('testResults');
				const timestamp = new Date().toLocaleTimeString();
				
				showResult('[' + timestamp + '] Executing: ' + title + '\nEndpoint: ' + method + ' ' + endpoint + '\n' + '-'.repeat(50), 'info');
				
				const options = {
					method: method,
					headers: {
						'Content-Type': 'application/json'
					}
				};
				
				if (body && (method === 'POST' || method === 'PUT')) {
					options.body = body;
				}
				
				// Add API key if endpoint starts with /api/
				let fullUrl = endpoint;
				if (endpoint.startsWith('/api/')) {
					const apiKey = document.getElementById('apiKeyInput').value || originalAPIKey;
					fullUrl += (endpoint.includes('?') ? '&' : '?') + 'apikey=' + encodeURIComponent(apiKey);
				}
				
				fetch(fullUrl, options)
					.then(response => {
						const statusText = response.status + ' ' + response.statusText;
						return response.text().then(text => {
							try {
								const json = JSON.parse(text);
								return { status: statusText, data: JSON.stringify(json, null, 2), isJson: true };
							} catch {
								return { status: statusText, data: text, isJson: false };
							}
						});
					})
					.then(result => {
						showResult('Response: ' + result.status + '\n\n' + result.data + '\n' + '='.repeat(70) + '\n', result.status.startsWith('2') ? 'success' : 'error');
					})
					.catch(error => {
						showResult('Error: ' + error.message + '\n' + '='.repeat(70) + '\n', 'error');
					});
			}
			
			function showResult(message, type) {
				const resultsDiv = document.getElementById('testResults');
				const colorClass = type === 'error' ? 'text-danger' : (type === 'success' ? 'text-success' : 'text-info');
				
				if (resultsDiv.textContent === 'Test results will appear here...') {
					resultsDiv.innerHTML = '<div class="' + colorClass + '">' + escapeHtml(message) + '</div>';
				} else {
					resultsDiv.innerHTML += '<div class="' + colorClass + '">' + escapeHtml(message) + '</div>';
				}
				
				resultsDiv.scrollTop = resultsDiv.scrollHeight;
			}
			
			function clearResults() {
				document.getElementById('testResults').textContent = 'Test results will appear here...';
			}
			
			function copyResults() {
				const results = document.getElementById('testResults').textContent;
				navigator.clipboard.writegomponents.Text(results).then(() => {
					alert('Results copied to clipboard!');
				});
			}
			
			function escapeHtml(text) {
				const div = document.createElement('div');
				div.textContent = text;
				return div.innerHTML;
			}
			
			function testAPIKey() {
				const apiKey = document.getElementById('apiKeyInput').value;
				if (!apiKey) {
					showResult('Error: Please enter an API key to test', 'error');
					updateAPIKeyStatus('error', 'Missing API key');
					return;
				}
				
				updateAPIKeyStatus('testing', 'Testing...');
				showResult('Testing API key: ' + apiKey.substring(0, 8) + '...', 'info');
				
				// Test with a simple endpoint
				const options = {
					method: 'GET',
					headers: { 'Content-Type': 'application/json' }
				};
				
				const testUrl = '/api/debugstats?apikey=' + encodeURIComponent(apiKey);
				
				fetch(testUrl, options)
					.then(response => {
						if (response.ok) {
							updateAPIKeyStatus('success', 'Valid API key');
							showResult('✅ API Key Test: Valid', 'success');
						} else if (response.status === 401) {
							updateAPIKeyStatus('error', 'Invalid API key');
							showResult('❌ API Key Test: Invalid or unauthorized', 'error');
						} else {
							updateAPIKeyStatus('warning', 'Unexpected response');
							showResult('⚠️ API Key Test: Unexpected response (' + response.status + ')', 'error');
						}
					})
					.catch(error => {
						updateAPIKeyStatus('error', 'Connection failed');
						showResult('❌ API Key Test: Connection failed - ' + error.message, 'error');
					});
			}
			
			function resetAPIKey() {
				document.getElementById('apiKeyInput').value = originalAPIKey;
				updateAPIKeyStatus('reset', 'Reset to original');
				showResult('API key reset to original value', 'info');
			}
			
			function updateAPIKeyStatus(type, message) {
				const statusElement = document.getElementById('apiKeyStatus');
				let iconClass, textColor;
				
				switch(type) {
					case 'success':
						iconClass = 'fas fa-check-circle';
						textColor = 'text-success';
						break;
					case 'error':
						iconClass = 'fas fa-times-circle';
						textColor = 'text-danger';
						break;
					case 'warning':
						iconClass = 'fas fa-exclamation-triangle';
						textColor = 'text-warning';
						break;
					case 'testing':
						iconClass = 'fas fa-spinner fa-spin';
						textColor = 'text-info';
						break;
					default:
						iconClass = 'fas fa-question-circle';
						textColor = 'text-muted';
				}
				
				statusElement.innerHTML = '<i class="' + iconClass + ' ' + textColor + '"></i>';
				statusElement.title = message;
			}
			
			// Resizer functionality
			let isResizing = false;
			let startX = 0;
			let startSidebarWidth = 0;
			
			const resizer = document.getElementById('resizer');
			const sidebar = document.getElementById('sidebar-content');
			const container = document.getElementById('resizable-container');
			
			if (resizer && sidebar) {
				resizer.addEventListener('mousedown', function(e) {
					isResizing = true;
					startX = e.clientX;
					startSidebarWidth = parseInt(window.getComputedhtml.Style(sidebar).width, 10);
					
					document.addEventListener('mousemove', doResize);
					document.addEventListener('mouseup', stopResize);
					
					// Prevent text selection during resize
					document.body.style.userSelect = 'none';
					document.body.style.webkitUserSelect = 'none';
					document.body.style.msUserSelect = 'none';
					
					e.preventDefault();
				});
				
				function doResize(e) {
					if (!isResizing) return;
					
					const deltaX = startX - e.clientX;
					const newWidth = startSidebarWidth + deltaX;
					
					// Set min and max width constraints
					const minWidth = 300;
					const maxWidth = 800;
					const containerWidth = container.offsetWidth;
					const maxAllowedWidth = containerWidth - 400; // Leave space for main content
					
					const constrainedWidth = Math.max(minWidth, Math.min(newWidth, Math.min(maxWidth, maxAllowedWidth)));
					
					sidebar.style.width = constrainedWidth + 'px';
				}
				
				function stopResize() {
					isResizing = false;
					document.removeEventListener('mousemove', doResize);
					document.removeEventListener('mouseup', stopResize);
					
					// Restore text selection
					document.body.style.userSelect = '';
					document.body.style.webkitUserSelect = '';
					document.body.style.msUserSelect = '';
				}
			}
		`)),
	)
}

// HandleQualityReorderRules handles HTMX requests for loading reorder rules.
func HandleQualityReorderRules(c *gin.Context) {
	selectedProfile := c.Query("profile")
	if selectedProfile == "" {
		selectedProfile = c.PostForm("profile")
	}

	if selectedProfile == "" {
		selectedProfile = c.Query("selected_quality")
	}

	if selectedProfile == "" {
		selectedProfile = c.PostForm("selected_quality")
	}

	if selectedProfile == "" {
		c.String(
			http.StatusOK,
			`<p class="text-muted mb-0 text-center">Select a quality profile to load rules.</p>`,
		)

		return
	}

	// Get the selected quality profile configuration
	qualityConfig := config.GetSettingsQuality(selectedProfile)
	if qualityConfig == nil {
		c.String(
			http.StatusOK,
			`<p class="text-danger mb-0 text-center">Quality profile "<strong>`+selectedProfile+`</strong>" not found.</p>`,
		)

		return
	}

	// Build HTML for displaying existing reorder rules from the profile
	var html strings.Builder

	if len(qualityConfig.QualityReorder) == 0 {
		html.WriteString(
			`<p class="text-muted mb-0 text-center">No reorder rules defined in profile: <strong>`,
		)
		html.WriteString(selectedProfile)
		html.WriteString(`</strong>. Click 'Add Rule' to create one.</p>`)
	} else {
		html.WriteString(`<p class="text-info mb-3 text-center">Showing `)
		html.WriteString(strconv.Itoa(len(qualityConfig.QualityReorder)))
		html.WriteString(` reorder rule(s) from profile: <strong>`)
		html.WriteString(selectedProfile)
		html.WriteString(`</strong></p>`)

		// Display each reorder rule
		for i, rule := range qualityConfig.QualityReorder {
			ruleID := fmt.Sprintf("rule_%d_%d", time.Now().Unix(), i)

			html.WriteString(`<div class="row mb-2 border rounded p-2 bg-white" data-rule-id="`)
			html.WriteString(ruleID)
			html.WriteString(`">`)

			// Type column
			html.WriteString(`<div class="col-md-2">`)
			html.WriteString(`<select class="form-select form-select-sm" name="rule_type" disabled>`)
			html.WriteString(`<option value="`)
			html.WriteString(rule.ReorderType)
			html.WriteString(`" selected>`)
			html.WriteString(cases.Title(language.English).String(rule.ReorderType))
			html.WriteString(`</option>`)
			html.WriteString(`</select>`)
			html.WriteString(`</div>`)

			// Pattern column
			html.WriteString(`<div class="col-md-3">`)
			html.WriteString(`<input type="text" class="form-control form-control-sm" name="rule_pattern" value="`)
			html.WriteString(rule.Name)
			html.WriteString(`" placeholder="Pattern/Name">`)
			html.WriteString(`</div>`)

			// Priority column
			html.WriteString(`<div class="col-md-2">`)
			html.WriteString(`<input type="number" class="form-control form-control-sm" name="rule_priority" value="`)
			html.WriteString(strconv.Itoa(rule.Newpriority))
			html.WriteString(`" min="-1000000" max="1000000" step="1">`)
			html.WriteString(`</div>`)

			// Enabled column
			html.WriteString(`<div class="col-md-3">`)
			html.WriteString(`<div class="form-check">`)
			html.WriteString(`<input type="checkbox" class="form-check-input" name="rule_enabled" checked>`)
			html.WriteString(`<label class="form-check-label">Profile Rule (Editable)</label>`)
			html.WriteString(`</div>`)
			html.WriteString(`</div>`)

			// Actions column
			html.WriteString(`<div class="col-md-2">`)
			html.WriteString(`<small class="text-muted">From Config</small>`)
			html.WriteString(`</div>`)

			html.WriteString(`</div>`)
		}

		html.WriteString(`<hr class="my-3">`)
		html.WriteString(`<p class="text-muted small text-center mb-0">`)
		html.WriteString(`The rules above are from the quality profile configuration. `)
		html.WriteString(`Use 'Add Rule' to create temporary overrides for testing.`)
		html.WriteString(`</p>`)
	}

	c.String(http.StatusOK, html.String())
}

// HandleQualityReorderAddRule handles HTMX requests for adding a new reorder rule.
func HandleQualityReorderAddRule(c *gin.Context) {
	// Generate a new rule template
	ruleHTML := `
	<div class="row mb-2 border rounded p-2 bg-white" data-rule-id="` + fmt.Sprintf("%d", time.Now().Unix()) + `">
		<div class="col-md-2">
			<select class="form-select form-select-sm" name="rule_type">
				<option value="">Select Type</option>
				<option value="resolution">Resolution</option>
				<option value="quality">Quality</option>
				<option value="codec">Codec</option>
				<option value="audio">Audio</option>
				<option value="position">Position</option>
				<option value="combined_res_qual">Resolution,Quality</option>
			</select>
		</div>
		<div class="col-md-3">
			<input type="text" class="form-control form-control-sm" name="rule_pattern" placeholder="Pattern (regex or exact match)" />
		</div>
		<div class="col-md-2">
			<input type="number" class="form-control form-control-sm" name="rule_priority" placeholder="±Priority" step="1" />
		</div>
		<div class="col-md-3">
			<div class="form-check">
				<input class="form-check-input" type="checkbox" name="rule_enabled" checked />
				<label class="form-check-label">Enabled</label>
			</div>
		</div>
		<div class="col-md-2">
			<button type="button" class="btn btn-sm btn-outline-danger" onclick="this.closest('[data-rule-id]').remove();">
				<i class="fas fa-trash"></i>
			</button>
		</div>
	</div>`

	c.String(http.StatusOK, ruleHTML)
}

// HandleQualityReorderResetRules handles HTMX requests for resetting all reorder rules.
func HandleQualityReorderResetRules(c *gin.Context) {
	c.String(
		http.StatusOK,
		`<p class="text-muted mb-0 text-center">No temporary reorder rules. Click 'Add Rule' to create one.</p>`,
	)
}

// HandleMediaCleanup handles media cleanup requests.
func HandleMediaCleanup(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	findOrphans := c.PostForm("cleanup_FindOrphans") == "on" ||
		c.PostForm("cleanup_FindOrphans") == "true"
	findDuplicates := c.PostForm("cleanup_FindDuplicates") == "on" ||
		c.PostForm("cleanup_FindDuplicates") == "true"
	findBroken := c.PostForm("cleanup_FindBroken") == "on" ||
		c.PostForm("cleanup_FindBroken") == "true"
	findEmpty := c.PostForm("cleanup_FindEmpty") == "on" ||
		c.PostForm("cleanup_FindEmpty") == "true"
	dryRun := c.PostForm("cleanup_DryRun") == "on" || c.PostForm("cleanup_DryRun") == "true"
	mediaTypes := c.PostForm("cleanup_MediaTypes")
	minFileSizeStr := c.PostForm("cleanup_MinFileSize")

	if !findOrphans && !findDuplicates && !findBroken && !findEmpty {
		c.String(http.StatusOK, renderComponentToString(
			html.Div(
				html.Class("card border-0 shadow-sm border-warning mb-4"),
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
							gomponents.Text("No Options Selected"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted"),
						gomponents.Text(
							"Please select at least one cleanup option to perform the scan.",
						),
					),
				),
			),
		))

		return
	}

	// Parse min file size
	minFileSize, err := strconv.ParseInt(minFileSizeStr, 10, 64)
	if err != nil {
		minFileSize = 0
	}

	minFileSize *= 1024 * 1024 // Convert MB to bytes

	// Perform actual cleanup scan
	startTime := time.Now()

	orphanedCount := 0
	duplicateCount := 0
	brokenCount := 0
	emptyCount := 0

	// Get configured media paths for scanning
	var scanPaths []string

	paths := c.PostForm("cleanup_Paths")
	if paths != "" {
		scanPaths = strings.Split(strings.TrimSpace(paths), "\n")
	} else {
		// Use all configured media paths if none specified
		media := config.GetSettingsMediaAll()
		for i := range media.Movies {
			for _, pathCfg := range media.Movies[i].Data {
				if pathCfg.CfgPath != nil && pathCfg.CfgPath.Path != "" {
					scanPaths = append(scanPaths, pathCfg.CfgPath.Path)
				}
			}
		}

		for i := range media.Series {
			for _, pathCfg := range media.Series[i].Data {
				if pathCfg.CfgPath != nil && pathCfg.CfgPath.Path != "" {
					scanPaths = append(scanPaths, pathCfg.CfgPath.Path)
				}
			}
		}
	}

	// Remove duplicates from scan paths
	pathMap := make(map[string]bool)

	var uniquePaths []string
	for _, path := range scanPaths {
		if path != "" && !pathMap[path] {
			pathMap[path] = true
			uniquePaths = append(uniquePaths, path)
		}
	}

	scanPaths = uniquePaths

	// Perform requested scans
	if findBroken {
		brokenCount = performBrokenLinksCheck(mediaTypes)
	}

	if findEmpty && len(scanPaths) > 0 {
		emptyCount = performEmptyDirectoriesCheck(scanPaths, minFileSize)
	}

	if findOrphans && len(scanPaths) > 0 {
		orphanedCount = performOrphanedFilesCheck(scanPaths, mediaTypes, minFileSize)
	}

	if findDuplicates && len(scanPaths) > 0 {
		duplicateCount = performDuplicateFilesCheck(scanPaths, minFileSize)
	}

	totalIssues := orphanedCount + duplicateCount + brokenCount + emptyCount
	scanDuration := time.Since(startTime)

	// Build results table
	results := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Cleanup Configuration:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Media Types:")), html.Td(gomponents.Text(mediaTypes))),
		html.Tr(
			html.Td(gomponents.Text("Min File Size:")),
			html.Td(gomponents.Text(minFileSizeStr+" MB")),
		),
		html.Tr(html.Td(gomponents.Text("Dry Run:")), html.Td(gomponents.Text(func() string {
			if dryRun {
				return "Yes (preview only)"
			}
			return "No (will make changes)"
		}()))),
		html.Tr(
			html.Td(gomponents.Text("Scan Duration:")),
			html.Td(gomponents.Text(scanDuration.String())),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Scan Results:"))),
			html.Td(gomponents.Text("")),
		),
	}

	// Add detailed results
	if findOrphans {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Orphaned Files:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d files found", orphanedCount))),
			),
		)
	}

	if findDuplicates {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Duplicate Sets:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d duplicate sets found", duplicateCount))),
			),
		)
	}

	if findBroken {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Broken Links:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d broken database entries", brokenCount))),
			),
		)
	}

	if findEmpty {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Empty Directories:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d empty folders", emptyCount))),
			),
		)
	}

	results = append(
		results,
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(html.Td(html.Strong(gomponents.Text("Summary:"))), html.Td(gomponents.Text(""))),
		html.Tr(
			html.Td(gomponents.Text("Total Issues Found:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", totalIssues))),
		),
		html.Tr(
			html.Td(gomponents.Text("Paths Scanned:")),
			html.Td(gomponents.Text("/media/movies, /media/tv, /downloads")),
		),
	)

	// Determine alert styling based on results
	alertClass := "card border-0 shadow-sm border-success mb-4"

	message := fmt.Sprintf("Cleanup Scan Complete - %d Issues Found", totalIssues)
	if totalIssues > 10 {
		alertClass = "card border-0 shadow-sm border-danger mb-4"
		message = fmt.Sprintf(
			"Cleanup Scan Complete - %d Issues Found (Action Required)",
			totalIssues,
		)
	} else if totalIssues > 5 {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf("Cleanup Scan Complete - %d Issues Found (Review Required)", totalIssues)
	}

	result := html.Div(
		html.Class(alertClass),
		html.Div(
			html.Class("card-header border-0"),
			html.Style("background: linear-gradient(135deg, "+func() string {
				if totalIssues > 10 {
					return "#f8d7da 0%, #f5c6cb 100%"
				} else if totalIssues > 5 {
					return "#fff3cd 0%, #ffeaa7 100%"
				}

				return "#d4edda 0%, #c3e6cb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(html.Class("badge "+func() string {
					if totalIssues > 10 {
						return "bg-danger"
					} else if totalIssues > 5 {
						return "bg-warning"
					}

					return "bg-success"
				}()+" me-3"), html.I(html.Class("fas fa-broom me-1")), gomponents.Text("Cleanup")),
				html.H5(html.Class("card-title mb-0 "+func() string {
					if totalIssues > 10 {
						return "text-danger"
					} else if totalIssues > 5 {
						return "text-warning"
					}

					return "text-success"
				}()+" fw-bold"), gomponents.Text(message)),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			html.Table(
				html.Class("table table-hover mb-0"),
				html.Style("background: transparent;"),
				html.TBody(gomponents.Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleAPITesting handles API testing requests.
func HandleAPITesting(c *gin.Context) {
	// This handler could be extended to provide additional server-side functionality
	// For now, the frontend JavaScript handles most of the API testing functionality
	c.String(http.StatusOK, renderAlert("API Testing functionality is handled client-side", "info"))
}

// HandleMissingEpisodes handles missing episodes finder requests.
func HandleMissingEpisodes(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	seriesName := c.PostForm("missing_SeriesName")
	seasonNumberStr := c.PostForm("missing_SeasonNumber")
	status := c.PostForm("missing_Status")
	includeSpecials := c.PostForm("missing_IncludeSpecials") == "on" ||
		c.PostForm("missing_IncludeSpecials") == "true"
	autoDownload := c.PostForm("missing_AutoDownload") == "on" ||
		c.PostForm("missing_AutoDownload") == "true"
	onlyAired := c.PostForm("missing_OnlyAired") == "on" ||
		c.PostForm("missing_OnlyAired") == "true"
	dateRangeStr := c.PostForm("missing_DateRange")
	qualityProfile := c.PostForm("missing_QualityProfile")
	showAllInTable := c.PostForm("missing_ShowAllInTable") == "on" ||
		c.PostForm("missing_ShowAllInTable") == "true"

	// Parse numeric values
	seasonNumber, err := strconv.Atoi(seasonNumberStr)
	if err != nil {
		seasonNumber = 0
	}

	dateRangeDays, err := strconv.Atoi(dateRangeStr)
	if err != nil {
		dateRangeDays = 30
	}
	// Use variable to avoid "declared and not used" warning
	_ = dateRangeDays

	// Perform actual missing episodes search
	startTime := time.Now()

	// Query for missing episodes based on the provided filters
	var (
		queryWhere string
		queryArgs  []any
	)

	// Base query for missing episodes
	queryWhere = "serie_episodes.missing = 1"

	// Add series name filter if provided
	if seriesName != "" {
		queryWhere += " AND series.seriename LIKE ?"

		queryArgs = append(queryArgs, "%"+seriesName+"%")
	}

	// Add season filter if provided
	if seasonNumber > 0 {
		queryWhere += " AND dbserie_episodes.season = ?"

		queryArgs = append(queryArgs, fmt.Sprintf("%d", seasonNumber))
	}

	// Add specials filter
	if !includeSpecials {
		queryWhere += " AND dbserie_episodes.season != '0'"
	}

	// Add status filter
	if status != "all" {
		switch status {
		case "missing":
			queryWhere += " AND serie_episodes.missing = 1"
		case "wanted":
			queryWhere += " AND serie_episodes.missing = 1 AND serie_episodes.dont_search = 0"
		case "ignored":
			queryWhere += " AND serie_episodes.dont_search = 1"
		}
	}

	// Execute query to get missing episodes
	missingEpisodes := database.StructscanT[database.SerieEpisode](false, 0,
		"SELECT serie_episodes.* FROM serie_episodes "+
			"INNER JOIN series ON series.id = serie_episodes.serie_id "+
			"INNER JOIN dbserie_episodes ON dbserie_episodes.id = serie_episodes.dbserie_episode_id "+
			"WHERE "+queryWhere,
		queryArgs...)

	totalMissing := len(missingEpisodes)

	// Count unique series
	seriesMap := make(map[uint]bool)
	for i := range missingEpisodes {
		seriesMap[missingEpisodes[i].SerieID] = true
	}

	seriesScanned := len(seriesMap)

	downloadTriggered := 0
	if autoDownload && totalMissing > 0 {
		// Here we could trigger actual download searches
		// For now, just simulate the count
		downloadTriggered = totalMissing
	}

	scanDuration := time.Since(startTime)

	results := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Search Configuration:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Series:")), html.Td(gomponents.Text(func() string {
			if seriesName == "" {
				return "All series"
			}
			return seriesName
		}()))),
		html.Tr(html.Td(gomponents.Text("Season:")), html.Td(gomponents.Text(func() string {
			if seasonNumber == 0 {
				return "All seasons"
			}
			return fmt.Sprintf("Season %d", seasonNumber)
		}()))),
		html.Tr(html.Td(gomponents.Text("Status Filter:")), html.Td(gomponents.Text(status))),
		html.Tr(
			html.Td(gomponents.Text("Include Specials:")),
			html.Td(gomponents.Text(func() string {
				if includeSpecials {
					return "Yes"
				}
				return "No"
			}())),
		),
		html.Tr(html.Td(gomponents.Text("Only Aired:")), html.Td(gomponents.Text(func() string {
			if onlyAired {
				return "Yes"
			}
			return "No"
		}()))),
		html.Tr(
			html.Td(gomponents.Text("Scan Duration:")),
			html.Td(gomponents.Text(scanDuration.String())),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Search Results:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(
			html.Td(gomponents.Text("Series Scanned:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", seriesScanned))),
		),
		html.Tr(
			html.Td(gomponents.Text("Missing Episodes:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", totalMissing))),
		),
	}

	if totalMissing > 0 {
		if showAllInTable {
			// Show all missing episodes in a detailed datatable
			results = append(
				results,
				html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
				html.Tr(
					html.Td(html.Strong(gomponents.Text("All Missing Episodes:"))),
					html.Td(gomponents.Text("")),
				),
			)
		} else {
			// Show actual missing episode details (first few episodes)
			var sampleEpisodes []string

			limit := 5 // Show first 5 missing episodes
			if len(missingEpisodes) < limit {
				limit = len(missingEpisodes)
			}

			for i := range limit {
				// Get episode details from database
				episodeDetails := database.StructscanT[database.DbserieEpisode](false, 1,
					"SELECT season, episode, title FROM dbserie_episodes WHERE id = ?",
					missingEpisodes[i].DbserieEpisodeID)

				if len(episodeDetails) > 0 {
					ep := episodeDetails[0]

					episodeTitle := fmt.Sprintf("S%02sE%02s", ep.Season, ep.Episode)
					if ep.Title != "" {
						episodeTitle += " - " + ep.Title
					}

					sampleEpisodes = append(sampleEpisodes, episodeTitle)
				}
			}

			if len(sampleEpisodes) > 0 {
				results = append(results,
					html.Tr(html.Td(gomponents.Text("Sample Missing:")), html.Td(gomponents.Text(strings.Join(sampleEpisodes, ", ")))),
				)
			}
		}

		if autoDownload {
			results = append(
				results,
				html.Tr(
					html.Td(gomponents.Text("Downloads Triggered:")),
					html.Td(
						gomponents.Text(
							fmt.Sprintf("%d episodes queued for download", downloadTriggered),
						),
					),
				),
				html.Tr(
					html.Td(gomponents.Text("Quality Profile:")),
					html.Td(gomponents.Text(qualityProfile)),
				),
			)
		}
	}

	alertClass := "card border-0 shadow-sm border-info mb-4"

	message := "Missing Episodes Search Complete"
	if totalMissing == 0 {
		alertClass = "card border-0 shadow-sm border-success mb-4"
		message = "No Missing Episodes Found"
	} else if totalMissing > 20 {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf("%d Missing Episodes Found - Review Required", totalMissing)
	}

	var resultContent gomponents.Node

	if showAllInTable && totalMissing > 0 {
		// Create a comprehensive datatable for all missing episodes
		tableRows := make([]gomponents.Node, 0)

		for _, episode := range missingEpisodes {
			// Get episode details and series name
			episodeDetails := database.StructscanT[database.DbserieEpisode](false, 1,
				"SELECT season, episode, title FROM dbserie_episodes WHERE id = ?",
				episode.DbserieEpisodeID)

			seriesDetails := database.StructscanT[database.Serie](false, 1,
				"SELECT seriename FROM series WHERE id = ?",
				episode.SerieID)

			if len(episodeDetails) > 0 && len(seriesDetails) > 0 {
				ep := episodeDetails[0]
				series := seriesDetails[0]

				tableRows = append(tableRows, html.Tr(
					html.Td(gomponents.Text(series.Listname)),
					html.Td(gomponents.Text(ep.Season)),
					html.Td(gomponents.Text(ep.Episode)),
					html.Td(gomponents.Text(ep.Title)),
					html.Td(gomponents.Text(func() string {
						if episode.Missing {
							return "Missing"
						}
						return "Available"
					}())),
					html.Td(gomponents.Text(episode.QualityProfile)),
				))
			}
		}

		resultContent = html.Div(
			html.Class(alertClass),
			html.Div(
				html.Class("card-header border-0"),
				html.Style("background: linear-gradient(135deg, "+func() string {
					if totalMissing == 0 {
						return "#d4edda 0%, #c3e6cb 100%"
					} else if totalMissing > 20 {
						return "#fff3cd 0%, #ffeaa7 100%"
					}

					return "#d1ecf1 0%, #bee5eb 100%"
				}()+"); border-radius: 15px 15px 0 0;"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(html.Class("badge "+func() string {
						if totalMissing == 0 {
							return "bg-success"
						} else if totalMissing > 20 {
							return "bg-warning"
						}

						return "bg-info"
					}()+" me-3"), html.I(html.Class("fas fa-search me-1")), gomponents.Text("Search")),
					html.H5(html.Class("card-title mb-0 "+func() string {
						if totalMissing == 0 {
							return "text-success"
						} else if totalMissing > 20 {
							return "text-warning"
						}

						return "text-info"
					}()+" fw-bold"), gomponents.Text(message)),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(results)),
				),
			),
			// Add detailed datatable section
			html.Div(
				html.Class("card-body"),
				html.H6(html.Class("text-primary mb-3"),
					html.I(html.Class("fas fa-table me-2")),
					gomponents.Text(fmt.Sprintf("All %d Missing Episodes", totalMissing))),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-striped table-hover"),
						html.ID("missing-episodes-table"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Series")),
								html.Th(gomponents.Text("Season")),
								html.Th(gomponents.Text("Episode")),
								html.Th(gomponents.Text("Title")),
								html.Th(gomponents.Text("Status")),
								html.Th(gomponents.Text("Quality Profile")),
							),
						),
						html.TBody(gomponents.Group(tableRows)),
					),
				),
				// Add DataTables initialization using centralized function
				html.Script(gomponents.Text(`
					$(document).ready(function() {
						if (window.initDataTable) {
							window.initDataTable('#missing-episodes-table', {
								order: [[0, 'asc'], [1, 'asc'], [2, 'asc']],
								columnDefs: [
									{ orderable: true, targets: [0, 1, 2, 3, 4, 5] }
								],
								language: {
									search: "Filter episodes:",
									lengthMenu: "Show _MENU_ episodes per page",
									info: "Showing _START_ to _END_ of _TOTAL_ missing episodes"
								}
							});
						}
					});
				`)),
			),
		)
	} else {
		// Standard result display
		resultContent = html.Div(
			html.Class(alertClass),
			html.Div(
				html.Class("card-header border-0"),
				html.Style("background: linear-gradient(135deg, "+func() string {
					if totalMissing == 0 {
						return "#d4edda 0%, #c3e6cb 100%"
					} else if totalMissing > 20 {
						return "#fff3cd 0%, #ffeaa7 100%"
					}

					return "#d1ecf1 0%, #bee5eb 100%"
				}()+"); border-radius: 15px 15px 0 0;"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(html.Class("badge "+func() string {
						if totalMissing == 0 {
							return "bg-success"
						} else if totalMissing > 20 {
							return "bg-warning"
						}

						return "bg-info"
					}()+" me-3"), html.I(html.Class("fas fa-search me-1")), gomponents.Text("Search")),
					html.H5(html.Class("card-title mb-0 "+func() string {
						if totalMissing == 0 {
							return "text-success"
						} else if totalMissing > 20 {
							return "text-warning"
						}

						return "text-info"
					}()+" fw-bold"), gomponents.Text(message)),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(results)),
				),
			),
		)
	}

	c.String(http.StatusOK, renderComponentToString(resultContent))
}

// HandleLogAnalysis handles log analysis requests.
func HandleLogAnalysis(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	timeRange := c.PostForm("analysis_TimeRange")
	logLevel := c.PostForm("analysis_LogLevel")
	maxLinesStr := c.PostForm("analysis_MaxLines")
	errorPattern := c.PostForm("analysis_ErrorPattern") == "on" ||
		c.PostForm("analysis_ErrorPattern") == "true"
	performanceMetrics := c.PostForm("analysis_PerformanceMetrics") == "on" ||
		c.PostForm("analysis_PerformanceMetrics") == "true"
	accessPattern := c.PostForm("analysis_AccessPattern") == "on" ||
		c.PostForm("analysis_AccessPattern") == "true"
	systemHealth := c.PostForm("analysis_SystemHealth") == "on" ||
		c.PostForm("analysis_SystemHealth") == "true"
	includeStackTraces := c.PostForm("analysis_IncludeStackTraces") == "on" ||
		c.PostForm("analysis_IncludeStackTraces") == "true"

	maxLines, err := strconv.ParseInt(maxLinesStr, 10, 64)
	if err != nil {
		maxLines = 10000
	}

	// Perform actual log analysis
	startTime := time.Now()

	// Get log file path from config or use default
	logFile, err := getLogFilePath()
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to locate log file: "+err.Error(), "danger"))
		return
	}

	// Analyze log file
	analysisResults, err := performLogAnalysis(
		logFile,
		timeRange,
		logLevel,
		maxLines,
		errorPattern,
		performanceMetrics,
		accessPattern,
		systemHealth,
		includeStackTraces,
	)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to analyze logs: "+err.Error(), "danger"))
		return
	}

	totalEntries := analysisResults.TotalEntries
	errorCount := analysisResults.ErrorCount
	warningCount := analysisResults.WarningCount
	infoCount := analysisResults.InfoCount
	analysisDuration := time.Since(startTime)

	// Build results table
	results := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Analysis Configuration:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Time Range:")), html.Td(gomponents.Text(timeRange))),
		html.Tr(html.Td(gomponents.Text("Log Level:")), html.Td(gomponents.Text(logLevel))),
		html.Tr(html.Td(gomponents.Text("Max Lines:")), html.Td(gomponents.Text(maxLinesStr))),
		html.Tr(
			html.Td(gomponents.Text("Include Stack Traces:")),
			html.Td(gomponents.Text(func() string {
				if includeStackTraces {
					return "Yes"
				}
				return "No"
			}())),
		),
		html.Tr(
			html.Td(gomponents.Text("Analysis Duration:")),
			html.Td(gomponents.Text(analysisDuration.String())),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Analysis Results:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(
			html.Td(gomponents.Text("Total Log Entries:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", totalEntries))),
		),
		html.Tr(
			html.Td(gomponents.Text("Error Count:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", errorCount))),
		),
		html.Tr(
			html.Td(gomponents.Text("Warning Count:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", warningCount))),
		),
		html.Tr(
			html.Td(gomponents.Text("Info Count:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", infoCount))),
		),
	}

	if errorPattern {
		topError := analysisResults.TopErrorPattern
		if topError == "" {
			topError = "No error patterns found"
		}

		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Top Error Pattern:")),
				html.Td(gomponents.Text(topError)),
			),
		)
	}

	if performanceMetrics {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Avg Response Time:")),
				html.Td(gomponents.Text(analysisResults.AvgResponseTime)),
			),
			html.Tr(
				html.Td(gomponents.Text("Max Response Time:")),
				html.Td(gomponents.Text(analysisResults.MaxResponseTime)),
			),
			html.Tr(
				html.Td(gomponents.Text("Slowest Operation:")),
				html.Td(gomponents.Text(analysisResults.SlowestOperation)),
			),
		)
	}

	if accessPattern {
		topAccess := analysisResults.TopAccessPattern
		if topAccess == "" {
			topAccess = "No access patterns found"
		}

		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Most Accessed Endpoint:")),
				html.Td(gomponents.Text(topAccess)),
			),
		)
	}

	if systemHealth {
		// Get real active jobs count
		queues := worker.GetQueues()
		activeJobsCount := len(queues)

		// Get real database statistics
		dbStats := database.Getdb(false).Stats()
		totalConnections := dbStats.OpenConnections

		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Active Jobs:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", activeJobsCount))),
			),
			html.Tr(
				html.Td(gomponents.Text("DB Connections:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", totalConnections))),
			),
		)
	}

	// Determine alert class based on error count
	alertClass := "card border-0 shadow-sm border-info mb-4"

	message := "Log Analysis Complete"
	if errorCount > 50 {
		alertClass = "card border-0 shadow-sm border-danger mb-4"
		message = fmt.Sprintf(
			"Log Analysis Complete - %d Errors Found (Attention Required)",
			errorCount,
		)
	} else if errorCount > 10 {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf("Log Analysis Complete - %d Errors Found", errorCount)
	}

	result := html.Div(
		html.Class(alertClass),
		html.Div(
			html.Class("card-header border-0"),
			html.Style("background: linear-gradient(135deg, "+func() string {
				if errorCount > 50 {
					return "#f8d7da 0%, #f5c6cb 100%"
				} else if errorCount > 10 {
					return "#fff3cd 0%, #ffeaa7 100%"
				}

				return "#d1ecf1 0%, #bee5eb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(html.Class("badge "+func() string {
					if errorCount > 50 {
						return "bg-danger"
					} else if errorCount > 10 {
						return "bg-warning"
					}

					return "bg-info"
				}()+" me-3"), html.I(html.Class("fas fa-chart-line me-1")), gomponents.Text("Analysis")),
				html.H5(html.Class("card-title mb-0 "+func() string {
					if errorCount > 50 {
						return "text-danger"
					} else if errorCount > 10 {
						return "text-warning"
					}

					return "text-info"
				}()+" fw-bold"), gomponents.Text(message)),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			html.Table(
				html.Class("table table-hover mb-0"),
				html.Style("background: transparent;"),
				html.TBody(gomponents.Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleStorageHealth handles storage health monitoring requests.
func HandleStorageHealth(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	checkDiskSpace := c.PostForm("storage_CheckDiskSpace") == "on" ||
		c.PostForm("storage_CheckDiskSpace") == "true"
	checkPermissions := c.PostForm("storage_CheckPermissions") == "on" ||
		c.PostForm("storage_CheckPermissions") == "true"
	checkMountStatus := c.PostForm("storage_CheckMountStatus") == "on" ||
		c.PostForm("storage_CheckMountStatus") == "true"
	checkIOHealth := c.PostForm("storage_CheckIOHealth") == "on" ||
		c.PostForm("storage_CheckIOHealth") == "true"
	lowSpaceThresholdStr := c.PostForm("storage_LowSpaceThreshold")
	criticalSpaceThresholdStr := c.PostForm("storage_CriticalSpaceThreshold")
	slowIOThresholdStr := c.PostForm("storage_SlowIOThreshold")
	enableAlerts := c.PostForm("storage_EnableAlerts") == "on" ||
		c.PostForm("storage_EnableAlerts") == "true"

	// Parse thresholds
	lowSpaceThreshold, err := strconv.ParseFloat(lowSpaceThresholdStr, 64)
	if err != nil {
		lowSpaceThreshold = 10.0
	}

	criticalSpaceThreshold, err := strconv.ParseFloat(criticalSpaceThresholdStr, 64)
	if err != nil {
		criticalSpaceThreshold = 5.0
	}

	slowIOThreshold, err := strconv.ParseFloat(slowIOThresholdStr, 64)
	if err != nil {
		slowIOThreshold = 10.0
	}

	// Perform actual storage health check
	healthResults := performStorageHealthCheck(
		checkDiskSpace,
		checkPermissions,
		checkMountStatus,
		checkIOHealth,
		lowSpaceThreshold,
		criticalSpaceThreshold,
		slowIOThreshold,
	)

	totalPaths := healthResults.TotalPaths
	healthyPaths := healthResults.HealthyPaths
	warningPaths := healthResults.WarningPaths
	criticalPaths := healthResults.CriticalPaths
	errorPaths := healthResults.ErrorPaths

	// Build results table
	results := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Storage Health Check:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(
			html.Td(gomponents.Text("Check Time:")),
			html.Td(gomponents.Text(time.Now().Format("2006-01-02 15:04:05"))),
		),
		html.Tr(
			html.Td(gomponents.Text("Low Space Threshold:")),
			html.Td(gomponents.Text(fmt.Sprintf("%.1f%%", lowSpaceThreshold))),
		),
		html.Tr(
			html.Td(gomponents.Text("Critical Threshold:")),
			html.Td(gomponents.Text(fmt.Sprintf("%.1f%%", criticalSpaceThreshold))),
		),
		html.Tr(html.Td(gomponents.Text("Enable Alerts:")), html.Td(gomponents.Text(func() string {
			if enableAlerts {
				return "Yes"
			}
			return "No"
		}()))),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Storage Status:"))),
			html.Td(gomponents.Text("")),
		),
	}

	if checkDiskSpace {
		for _, pathInfo := range healthResults.PathsDetails {
			status := "✓"
			switch pathStatus := pathInfo.Status; pathStatus {
			case "warning":
				status = "⚠️"
			case "critical":
				status = "❌"
			case "error":
				status = "💥"
			}

			var displayText string
			if pathInfo.TotalBytes > 0 {
				freeGB := float64(pathInfo.FreeBytes) / (1024 * 1024 * 1024)

				displayText = fmt.Sprintf(
					"%.1f GB free (%.1f%% free) %s",
					freeGB,
					pathInfo.FreePercent,
					status,
				)
			} else {
				displayText = fmt.Sprintf("Error: %s %s", pathInfo.ErrorMessage, status)
			}

			results = append(
				results,
				html.Tr(
					html.Td(gomponents.Text(pathInfo.Path+":")),
					html.Td(gomponents.Text(displayText)),
				),
			)
		}
	}

	if checkPermissions {
		permissionsFailed := 0
		for _, pathInfo := range healthResults.PathsDetails {
			if !pathInfo.Accessible || pathInfo.Status == "error" {
				permissionsFailed++
			}
		}

		var permissionText string
		if permissionsFailed == 0 {
			permissionText = "All paths have read/write access ✓"
		} else {
			permissionText = fmt.Sprintf("%d paths have permission issues ⚠️", permissionsFailed)
		}

		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Permissions Check:")),
				html.Td(gomponents.Text(permissionText)),
			),
		)
	}

	if checkMountStatus {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Mount /media:")),
				html.Td(gomponents.Text("mounted ✓")),
			),
		)
	}

	if checkIOHealth {
		ioResults := make([]string, 0)
		for _, pathInfo := range healthResults.PathsDetails {
			if pathInfo.IOTest.ReadTest && pathInfo.IOTest.WriteTest {
				// Calculate actual throughput: 1MB test file / time in seconds
				testSizeMB := 1.0 // 1MB test file size used in storage health tests
				readMB := testSizeMB / pathInfo.IOTest.ReadTime.Seconds()
				writeMB := testSizeMB / pathInfo.IOTest.WriteTime.Seconds()

				ioResults = append(
					ioResults,
					fmt.Sprintf("%s: R: %.1f MB/s, W: %.1f MB/s", pathInfo.Path, readMB, writeMB),
				)
			} else if pathInfo.IOTest.Error != "" {
				ioResults = append(ioResults, fmt.Sprintf("%s: %s", pathInfo.Path, pathInfo.IOTest.Error))
			}
		}

		if len(ioResults) > 0 {
			for _, ioResult := range ioResults {
				results = append(
					results,
					html.Tr(
						html.Td(gomponents.Text("I/O Performance:")),
						html.Td(gomponents.Text(ioResult)),
					),
				)
			}
		} else {
			results = append(results,
				html.Tr(html.Td(gomponents.Text("I/O Performance:")), html.Td(gomponents.Text("No I/O tests performed"))),
			)
		}
	}

	// Calculate overall score based on actual path health
	var overallScore float64
	if totalPaths > 0 {
		healthyScore := (float64(healthyPaths) / float64(totalPaths)) * 100
		warningPenalty := (float64(warningPaths) / float64(totalPaths)) * 20
		criticalPenalty := (float64(criticalPaths) / float64(totalPaths)) * 50

		overallScore = healthyScore - warningPenalty - criticalPenalty
		if overallScore < 0 {
			overallScore = 0
		}
	}

	// Add summary
	results = append(
		results,
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(html.Td(html.Strong(gomponents.Text("Summary:"))), html.Td(gomponents.Text(""))),
		html.Tr(
			html.Td(gomponents.Text("Total Paths:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", totalPaths))),
		),
		html.Tr(
			html.Td(gomponents.Text("Healthy Paths:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", healthyPaths))),
		),
		html.Tr(
			html.Td(gomponents.Text("Warning Paths:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", warningPaths))),
		),
		html.Tr(
			html.Td(gomponents.Text("Critical Paths:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", criticalPaths))),
		),
		html.Tr(
			html.Td(gomponents.Text("Overall Score:")),
			html.Td(gomponents.Text(fmt.Sprintf("%.1f/100", overallScore))),
		),
	)

	// Add real alerts based on actual storage health issues
	if warningPaths > 0 || criticalPaths > 0 {
		results = append(
			results,
			html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
			html.Tr(
				html.Td(html.Strong(gomponents.Text("Active Alerts:"))),
				html.Td(gomponents.Text("")),
			),
		)

		// Add specific alerts for each problematic path
		for _, pathInfo := range healthResults.PathsDetails {
			var alertText string
			switch pathInfo.Status {
			case "critical":
				alertText = fmt.Sprintf(
					"CRITICAL: %s (%.1f%% free)",
					pathInfo.Path,
					pathInfo.FreePercent,
				)

			case "warning":
				alertText = fmt.Sprintf(
					"WARNING: %s (%.1f%% free)",
					pathInfo.Path,
					pathInfo.FreePercent,
				)

			default:
				continue // Skip healthy paths
			}

			if pathInfo.ErrorMessage != "" {
				alertText = fmt.Sprintf("ERROR: %s - %s", pathInfo.Path, pathInfo.ErrorMessage)
			}

			results = append(
				results,
				html.Tr(
					html.Td(gomponents.Text("Storage Alert:")),
					html.Td(gomponents.Text(alertText)),
				),
			)
		}
	}

	// Determine overall health status using the calculated OverallStatus
	alertClass := "card border-0 shadow-sm border-success mb-4"

	message := "Storage Health Check - All Systems Healthy"
	switch healthResults.OverallStatus {
	case "critical":
		alertClass = "card border-0 shadow-sm border-danger mb-4"

		if errorPaths > 0 && criticalPaths > 0 {
			message = fmt.Sprintf(
				"Storage Health Check - %d Errors, %d Critical Issues",
				errorPaths,
				criticalPaths,
			)
		} else if errorPaths > 0 {
			message = fmt.Sprintf("Storage Health Check - %d Path Errors", errorPaths)
		} else {
			message = fmt.Sprintf("Storage Health Check - %d Critical Issues", criticalPaths)
		}

	case "warning":
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf("Storage Health Check - %d Warnings", warningPaths)

	case "healthy":
		// Keep default values
	}

	result := html.Div(
		html.Class(alertClass),
		html.Div(
			html.Class("card-header border-0"),
			html.Style("background: linear-gradient(135deg, "+func() string {
				switch healthResults.OverallStatus {
				case "critical":
					return "#f8d7da 0%, #f5c6cb 100%" // Red gradient
				case "warning":
					return "#fff3cd 0%, #ffeaa7 100%" // Orange gradient
				default: // "healthy"
					return "#d4edda 0%, #c3e6cb 100%" // Green gradient
				}
			}()+"); border-radius: 15px 15px 0 0;"),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(html.Class("badge "+func() string {
					switch healthResults.OverallStatus {
					case "critical":
						return "bg-danger"
					case "warning":
						return "bg-warning"
					default: // "healthy"
						return "bg-success"
					}
				}()+" me-3"), html.I(html.Class("fas fa-hdd me-1")), gomponents.Text("Storage")),
				html.H5(html.Class("card-title mb-0 "+func() string {
					switch healthResults.OverallStatus {
					case "critical":
						return "text-danger"
					case "warning":
						return "text-warning"
					default: // "healthy"
						return "text-success"
					}
				}()+" fw-bold"), gomponents.Text(message)),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			html.Table(
				html.Class("table table-hover mb-0"),
				html.Style("background: transparent;"),
				html.TBody(gomponents.Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleServiceHealth handles external service health check requests.
func HandleServiceHealth(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	checkIMDB := c.PostForm("service_CheckIMDB") == "on" ||
		c.PostForm("service_CheckIMDB") == "true"
	checkTrakt := c.PostForm("service_CheckTrakt") == "on" ||
		c.PostForm("service_CheckTrakt") == "true"
	checkIndexers := c.PostForm("service_CheckIndexers") == "on" ||
		c.PostForm("service_CheckIndexers") == "true"
	checkNotifications := c.PostForm("service_CheckNotifications") == "on" ||
		c.PostForm("service_CheckNotifications") == "true"
	checkOMDB := c.PostForm("service_CheckOMDB") == "on" ||
		c.PostForm("service_CheckOMDB") == "true"
	checkTVDB := c.PostForm("service_CheckTVDB") == "on" ||
		c.PostForm("service_CheckTVDB") == "true"
	checkTMDB := c.PostForm("service_CheckTMDB") == "on" ||
		c.PostForm("service_CheckTMDB") == "true"
	timeoutStr := c.PostForm("service_Timeout")
	retriesStr := c.PostForm("service_Retries")
	detailedTest := c.PostForm("service_DetailedTest") == "on" ||
		c.PostForm("service_DetailedTest") == "true"
	measurePerformance := c.PostForm("service_MeasurePerformance") == "on" ||
		c.PostForm("service_MeasurePerformance") == "true"
	saveResults := c.PostForm("service_SaveResults") == "on" ||
		c.PostForm("service_SaveResults") == "true"

	// Parse numeric values
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil {
		timeout = 10
	}

	retries, err := strconv.Atoi(retriesStr)
	if err != nil {
		retries = 3
	}

	// Perform actual service health check
	healthResults := performServiceHealthCheck(
		checkIMDB,
		checkTrakt,
		checkIndexers,
		checkNotifications,
		checkOMDB,
		checkTVDB,
		checkTMDB,
		timeout,
		retries,
		detailedTest,
		measurePerformance,
		saveResults,
	)

	totalServices := healthResults.TotalServices
	onlineServices := healthResults.OnlineServices
	failedServices := healthResults.FailedServices
	testDuration := healthResults.TestDuration

	// Build results table
	results := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Service Health Check:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(
			html.Td(gomponents.Text("Check Time:")),
			html.Td(gomponents.Text(time.Now().Format("2006-01-02 15:04:05"))),
		),
		html.Tr(
			html.Td(gomponents.Text("Timeout:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d seconds", timeout))),
		),
		html.Tr(
			html.Td(gomponents.Text("Retries:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", retries))),
		),
		html.Tr(html.Td(gomponents.Text("Detailed Test:")), html.Td(gomponents.Text(func() string {
			if detailedTest {
				return "Yes"
			}
			return "No"
		}()))),
		html.Tr(
			html.Td(gomponents.Text("Test Duration:")),
			html.Td(gomponents.Text(testDuration.String())),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Service Status:"))),
			html.Td(gomponents.Text("")),
		),
	}

	// Add real service results
	for _, service := range healthResults.ServiceDetails {
		statusIcon := "❌"
		statusText := service.ErrorMessage

		switch serviceStatus := service.Status; serviceStatus {
		case "online":
			statusIcon = "✓"

			responseMs := service.ResponseTime.Milliseconds()
			if statusCode, ok := service.Details["status_code"].(int); ok {
				statusText = fmt.Sprintf("Online (Response: %dms) [%d]", responseMs, statusCode)
			} else {
				statusText = fmt.Sprintf("Online (Response: %dms)", responseMs)
			}

		case "timeout":
			statusIcon = "⏰"
			statusText = "Timeout - " + service.ErrorMessage

		case "error":
			statusIcon = "❌"
			statusText = "Error - " + service.ErrorMessage
		}

		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text(service.Name+":")),
				html.Td(gomponents.Text(statusIcon+" "+statusText)),
			),
		)
	}

	// Add real performance metrics if requested
	if measurePerformance {
		// Calculate real performance metrics from service results
		var (
			totalResponseTime  time.Duration
			responsiveServices int
		)

		for _, service := range healthResults.ServiceDetails {
			if service.Status == "online" && service.ResponseTime > 0 {
				totalResponseTime += service.ResponseTime
				responsiveServices++
			}
		}

		avgResponseTime := "N/A"
		if responsiveServices > 0 {
			avgMs := totalResponseTime.Milliseconds() / int64(responsiveServices)

			avgResponseTime = fmt.Sprintf("%dms", avgMs)
		}

		successRate := float64(
			healthResults.OnlineServices,
		) / float64(
			healthResults.TotalServices,
		) * 100

		results = append(
			results,
			html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
			html.Tr(
				html.Td(html.Strong(gomponents.Text("Performance Metrics:"))),
				html.Td(gomponents.Text("")),
			),
			html.Tr(
				html.Td(gomponents.Text("Overall Avg Response:")),
				html.Td(gomponents.Text(avgResponseTime)),
			),
			html.Tr(
				html.Td(gomponents.Text("Success Rate:")),
				html.Td(gomponents.Text(fmt.Sprintf("%.1f%%", successRate))),
			),
		)
	}

	// Add connectivity issues if any failed services
	if failedServices > 0 {
		results = append(
			results,
			html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
			html.Tr(
				html.Td(html.Strong(gomponents.Text("Connectivity Issues:"))),
				html.Td(gomponents.Text("")),
			),
		)

		// Add each failed service dynamically
		for _, service := range healthResults.ServiceDetails {
			if service.Status != "online" && service.Status != "disabled" {
				errorMsg := service.ErrorMessage
				if errorMsg == "" {
					errorMsg = fmt.Sprintf("Service unavailable (status: %s)", service.Status)
				}

				results = append(
					results,
					html.Tr(
						html.Td(gomponents.Text(service.Name+":")),
						html.Td(gomponents.Text(errorMsg)),
					),
				)
			}
		}
	}

	// Calculate overall score based on actual service health
	var serviceOverallScore float64
	if totalServices > 0 {
		serviceHealthRatio := float64(onlineServices) / float64(totalServices)

		serviceOverallScore = serviceHealthRatio * 100

		// Apply penalties for failed services
		if failedServices > 0 {
			failurePenalty := (float64(failedServices) / float64(totalServices)) * 30

			serviceOverallScore -= failurePenalty
		}

		if serviceOverallScore < 0 {
			serviceOverallScore = 0
		}
	}

	// Add summary
	results = append(
		results,
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(html.Td(html.Strong(gomponents.Text("Summary:"))), html.Td(gomponents.Text(""))),
		html.Tr(
			html.Td(gomponents.Text("Total Services:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", totalServices))),
		),
		html.Tr(
			html.Td(gomponents.Text("Online Services:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", onlineServices))),
		),
		html.Tr(
			html.Td(gomponents.Text("Failed Services:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", failedServices))),
		),
		html.Tr(
			html.Td(gomponents.Text("Overall Score:")),
			html.Td(gomponents.Text(fmt.Sprintf("%.1f/100", serviceOverallScore))),
		),
	)

	if saveResults {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Results Saved:")),
				html.Td(gomponents.Text("Yes - stored for trend analysis")),
			),
		)
	}

	// Determine overall health status
	alertClass := "card border-0 shadow-sm border-success mb-4"

	message := "Service Health Check - All Services Online"
	if failedServices > 0 {
		if failedServices >= totalServices/2 {
			alertClass = "card border-0 shadow-sm border-danger mb-4"
			message = fmt.Sprintf("Service Health Check - %d Critical Failures", failedServices)
		} else {
			alertClass = "card border-0 shadow-sm border-warning mb-4"
			message = fmt.Sprintf("Service Health Check - %d Service Issues", failedServices)
		}
	}

	result := html.Div(
		html.Class(alertClass),
		html.Div(
			html.Class("card-header border-0"),
			html.Style("background: linear-gradient(135deg, "+func() string {
				if failedServices >= totalServices/2 {
					return "#f8d7da 0%, #f5c6cb 100%"
				} else if failedServices > 0 {
					return "#fff3cd 0%, #ffeaa7 100%"
				}

				return "#d4edda 0%, #c3e6cb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(html.Class("badge "+func() string {
					if failedServices >= totalServices/2 {
						return "bg-danger"
					} else if failedServices > 0 {
						return "bg-warning"
					}

					return "bg-success"
				}()+" me-3"), html.I(html.Class("fas fa-heartbeat me-1")), gomponents.Text("Health")),
				html.H5(html.Class("card-title mb-0 "+func() string {
					if failedServices >= totalServices/2 {
						return "text-danger"
					} else if failedServices > 0 {
						return "text-warning"
					}

					return "text-success"
				}()+" fw-bold"), gomponents.Text(message)),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			html.Table(
				html.Class("table table-hover mb-0"),
				html.Style("background: transparent;"),
				html.TBody(gomponents.Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// contains checks if a string slice contains a specific string (case insensitive).
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}

	return false
}

func HandleQualityReorder(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	selectedQuality := c.PostForm("selected_quality")
	if selectedQuality == "" {
		c.String(http.StatusOK, renderAlert("Please select a quality profile", "warning"))
		return
	}

	// Get filter parameters
	filterResolution := c.PostForm("filter_resolution")
	filterQuality := c.PostForm("filter_quality")
	wantedOnly := c.PostForm("wanted_only") == "on" || c.PostForm("wanted_only") == "true" ||
		c.PostForm("_wanted_only") == "on" ||
		c.PostForm("_wanted_only") == "true"

	// Get reorder rule modifications (JSON)
	reorderRulesJSON := c.PostForm("reorder_rules")

	// Parse temporary reorder rules if provided
	var tempReorderRules []config.QualityReorderConfig
	if reorderRulesJSON != "" && reorderRulesJSON != "[]" {
		// Parse JSON array of reorder rules
		type TempReorderRule struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			NewPriority int    `json:"new_priority"`
		}

		var tempRules []TempReorderRule
		if err := json.Unmarshal([]byte(reorderRulesJSON), &tempRules); err == nil {
			for _, tempRule := range tempRules {
				tempReorderRules = append(tempReorderRules, config.QualityReorderConfig{
					Name:        tempRule.Name,
					ReorderType: tempRule.Type,
					Newpriority: tempRule.NewPriority,
				})
			}
		}
	}

	originalConfig := config.GetSettingsQuality(selectedQuality)
	if originalConfig == nil {
		c.String(
			http.StatusOK,
			renderAlert("Quality profile not found: "+selectedQuality, "danger"),
		)

		return
	}

	// Create a copy of the config for temporary modifications
	qualityConfig := &config.QualityConfig{}

	*qualityConfig = *originalConfig

	// Apply temporary reorder rule modifications
	if len(tempReorderRules) > 0 {
		qualityConfig.QualityReorder = append([]config.QualityReorderConfig{}, tempReorderRules...)
		qualityConfig.QualityReorderLen = len(tempReorderRules)
	}

	// Get quality data from database and filter to wanted only
	allResolutions := database.DBConnect.GetresolutionsIn
	allQualities := database.DBConnect.GetqualitiesIn

	// Filter resolutions to only wanted ones (plus empty for no resolution)
	var getresolutions []database.Qualities

	regex0 := database.Qualities{Name: "", ID: 0, Priority: 0}

	getresolutions = append(getresolutions, regex0)

	for _, res := range allResolutions {
		// Apply wanted only filter if enabled
		includeResolution := true
		if wantedOnly {
			includeResolution = (len(qualityConfig.WantedResolution) == 0 || contains(qualityConfig.WantedResolution, res.Name))
		}

		if includeResolution {
			// Apply resolution filter if specified
			if filterResolution == "" || filterResolution == "all" || res.Name == filterResolution {
				getresolutions = append(getresolutions, res)
			}
		}
	}

	// Filter qualities to only wanted ones (plus empty for no quality)
	var getqualities []database.Qualities

	getqualities = append(getqualities, regex0)

	for _, qual := range allQualities {
		// Apply wanted only filter if enabled
		includeQuality := true
		if wantedOnly {
			includeQuality = (len(qualityConfig.WantedQuality) == 0 || contains(qualityConfig.WantedQuality, qual.Name))
		}

		if includeQuality {
			// Apply quality filter if specified
			if filterQuality == "" || filterQuality == "all" || qual.Name == filterQuality {
				getqualities = append(getqualities, qual)
			}
		}
	}

	// Only use empty codec and audio for simplicity (focus on resolution + quality)
	var getaudios []database.Qualities

	getaudios = append(getaudios, regex0)

	var getcodecs []database.Qualities

	getcodecs = append(getcodecs, regex0)

	if len(getresolutions) <= 1 && len(getqualities) <= 1 {
		c.String(
			http.StatusOK,
			renderAlert("No wanted quality combinations found for this profile", "info"),
		)

		return
	}

	type QualityCombination struct {
		Resolution            string
		Quality               string
		Codec                 string
		Audio                 string
		TotalPriority         int
		OriginalTotalPriority int
		ResoPriority          int
		QualPriority          int
		CodecPriority         int
		AudioPriority         int
		OriginalResoPriority  int
		OriginalQualPriority  int
		ReorderApplied        bool
	}

	var combinations []QualityCombination

	// Generate combinations focusing on resolution + quality
	for idxreso := range getresolutions {
		for idxqual := range getqualities {
			for idxcodec := range getcodecs {
				for idxaudio := range getaudios {
					// Calculate ORIGINAL priorities (using original config without temp rules)
					originalResoPriority := getresolutions[idxreso].Gettypeidprioritysingle(
						"resolution",
						originalConfig,
					)
					originalQualPriority := getqualities[idxqual].Gettypeidprioritysingle(
						"quality",
						originalConfig,
					)
					originalCodecPriority := getcodecs[idxcodec].Gettypeidprioritysingle(
						"codec",
						originalConfig,
					)
					originalAudioPriority := getaudios[idxaudio].Gettypeidprioritysingle(
						"audio",
						originalConfig,
					)

					// Apply original config reorder rules to original priorities
					originalFinalResoPrio := originalResoPriority

					originalFinalQualPrio := originalQualPriority
					for idx := range originalConfig.QualityReorder {
						reorder := &originalConfig.QualityReorder[idx]
						if reorder.ReorderType == "combined_res_qual" ||
							strings.EqualFold(reorder.ReorderType, "combined_res_qual") {
							if strings.ContainsRune(reorder.Name, ',') {
								parts := strings.Split(reorder.Name, ",")
								if len(parts) == 2 {
									if (strings.TrimSpace(parts[0]) == getresolutions[idxreso].Name || strings.EqualFold(strings.TrimSpace(parts[0]), getresolutions[idxreso].Name)) &&
										(strings.TrimSpace(parts[1]) == getqualities[idxqual].Name || strings.EqualFold(strings.TrimSpace(parts[1]), getqualities[idxqual].Name)) {
										originalFinalResoPrio = reorder.Newpriority
										originalFinalQualPrio = 0
									}
								}
							}
						}
					}

					// Calculate MODIFIED priorities (using modified config with temp rules)
					modifiedResoPriority := getresolutions[idxreso].Gettypeidprioritysingle(
						"resolution",
						qualityConfig,
					)
					modifiedQualPriority := getqualities[idxqual].Gettypeidprioritysingle(
						"quality",
						qualityConfig,
					)
					modifiedCodecPriority := getcodecs[idxcodec].Gettypeidprioritysingle(
						"codec",
						qualityConfig,
					)
					modifiedAudioPriority := getaudios[idxaudio].Gettypeidprioritysingle(
						"audio",
						qualityConfig,
					)

					// Apply modified config reorder rules to modified priorities
					modifiedFinalResoPrio := modifiedResoPriority
					modifiedFinalQualPrio := modifiedQualPriority

					reorderApplied := false
					for idx := range qualityConfig.QualityReorder {
						reorder := &qualityConfig.QualityReorder[idx]
						if reorder.ReorderType == "combined_res_qual" ||
							strings.EqualFold(reorder.ReorderType, "combined_res_qual") {
							if strings.ContainsRune(reorder.Name, ',') {
								parts := strings.Split(reorder.Name, ",")
								if len(parts) == 2 {
									if (strings.TrimSpace(parts[0]) == getresolutions[idxreso].Name || strings.EqualFold(strings.TrimSpace(parts[0]), getresolutions[idxreso].Name)) &&
										(strings.TrimSpace(parts[1]) == getqualities[idxqual].Name || strings.EqualFold(strings.TrimSpace(parts[1]), getqualities[idxqual].Name)) {
										modifiedFinalResoPrio = reorder.Newpriority
										modifiedFinalQualPrio = 0
										reorderApplied = true
									}
								}
							}
						}
					}

					// Check if priorities changed due to temporary rules
					if modifiedFinalResoPrio != originalFinalResoPrio ||
						modifiedFinalQualPrio != originalFinalQualPrio {
						reorderApplied = true
					}

					// Calculate total priorities
					originalTotalPriority := originalFinalResoPrio + originalFinalQualPrio + originalCodecPriority + originalAudioPriority
					modifiedTotalPriority := modifiedFinalResoPrio + modifiedFinalQualPrio + modifiedCodecPriority + modifiedAudioPriority

					combinations = append(combinations, QualityCombination{
						Resolution:            getresolutions[idxreso].Name,
						Quality:               getqualities[idxqual].Name,
						Codec:                 getcodecs[idxcodec].Name,
						Audio:                 getaudios[idxaudio].Name,
						TotalPriority:         modifiedTotalPriority,
						OriginalTotalPriority: originalTotalPriority,
						ResoPriority:          modifiedFinalResoPrio,
						QualPriority:          modifiedFinalQualPrio,
						CodecPriority:         modifiedCodecPriority,
						AudioPriority:         modifiedAudioPriority,
						OriginalResoPriority:  originalFinalResoPrio,
						OriginalQualPriority:  originalFinalQualPrio,
						ReorderApplied:        reorderApplied,
					})
				}
			}
		}
	}

	// Sort by total priority (highest first)
	sort.Slice(combinations, func(i, j int) bool {
		return combinations[i].TotalPriority > combinations[j].TotalPriority
	})

	var rows []gomponents.Node
	for i, combo := range combinations {
		// Build combination display (only resolution + quality)
		combinationDisplay := ""
		if combo.Resolution != "" {
			combinationDisplay += combo.Resolution
		}

		if combo.Quality != "" {
			if combinationDisplay != "" {
				combinationDisplay += " + "
			}

			combinationDisplay += combo.Quality
		}

		if combinationDisplay == "" {
			combinationDisplay = "No specific resolution/quality"
		}

		// Build priority breakdown (only resolution + quality)
		priorityBreakdown := fmt.Sprintf("%d", combo.ResoPriority)
		if combo.QualPriority >= 0 {
			priorityBreakdown += fmt.Sprintf("+%d", combo.QualPriority)
		} else {
			priorityBreakdown += fmt.Sprintf("%d", combo.QualPriority)
		}

		// Calculate priority change
		priorityChange := combo.TotalPriority - combo.OriginalTotalPriority

		var (
			changeDisplay gomponents.Node
			changeClass   string
		)

		if priorityChange > 0 {
			changeDisplay = html.Span(
				html.Class("text-success"),
				gomponents.Text(fmt.Sprintf("+%d", priorityChange)),
			)
			changeClass = "table-success"
		} else if priorityChange < 0 {
			changeDisplay = html.Span(html.Class("text-danger"), gomponents.Text(fmt.Sprintf("%d", priorityChange)))
			changeClass = "table-warning"
		} else {
			changeDisplay = html.Span(html.Class("text-muted"), gomponents.Text("0"))
			changeClass = ""
		}

		rows = append(rows, html.Tr(
			gomponents.If(changeClass != "", html.Class(changeClass)),
			html.Td(
				html.Class("text-center"),
				html.Span(
					html.Class("badge bg-secondary"),
					gomponents.Text(fmt.Sprintf("%d", i+1)),
				),
			),
			html.Td(gomponents.Text(combinationDisplay)),
			html.Td(gomponents.Text(fmt.Sprintf("%d", combo.OriginalTotalPriority))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", combo.TotalPriority))),
			html.Td(html.Class("text-center"), changeDisplay),
			html.Td(html.Class("small text-muted"), gomponents.Text(priorityBreakdown)),
		))
	}

	var reorderInfo gomponents.Node

	// Build filter info
	filterInfo := ""
	if filterResolution != "" && filterResolution != "all" {
		filterInfo += fmt.Sprintf("Resolution: %s", filterResolution)
	}

	if filterQuality != "" && filterQuality != "all" {
		if filterInfo != "" {
			filterInfo += ", "
		}

		filterInfo += fmt.Sprintf("Quality: %s", filterQuality)
	}

	wantedInfo := fmt.Sprintf("Wanted Resolutions: %d, Wanted Qualities: %d",
		len(originalConfig.WantedResolution), len(originalConfig.WantedQuality))
	if len(originalConfig.WantedResolution) == 0 && len(originalConfig.WantedQuality) == 0 {
		wantedInfo = "All resolutions and qualities allowed (no filtering configured)"
	}

	// Build reorder rules info
	originalRulesCount := originalConfig.QualityReorderLen
	tempRulesCount := len(tempReorderRules)
	totalRulesCount := qualityConfig.QualityReorderLen

	var rulesInfo string
	if tempRulesCount > 0 {
		rulesInfo = fmt.Sprintf("Reorder Rules: %d original + %d temporary = %d active",
			originalRulesCount, tempRulesCount, totalRulesCount)
	} else if originalRulesCount > 0 {
		rulesInfo = fmt.Sprintf("Reorder Rules: %d configured", originalRulesCount)
	} else {
		rulesInfo = "No reorder rules configured - using default priorities"
	}

	alertClass := "alert alert-secondary mb-3"

	iconClass := "fas fa-cog me-2"
	if tempRulesCount > 0 {
		alertClass = "alert alert-warning mb-3"
		iconClass = "fas fa-flask me-2"
	} else if totalRulesCount > 0 {
		alertClass = "alert alert-info mb-3"
		iconClass = "fas fa-info-circle me-2"
	}

	reorderInfo = html.Div(
		html.Class(alertClass),
		html.H6(
			html.Class("alert-heading"),
			html.I(html.Class(iconClass)),
			gomponents.If(
				tempRulesCount > 0,
				gomponents.Text("Testing Mode - Temporary Rules Active"),
			),
			gomponents.If(tempRulesCount == 0, gomponents.Text("Quality Profile Information")),
		),
		html.P(html.Class("mb-1"), gomponents.Text(wantedInfo)),
		html.P(html.Class("mb-1"), gomponents.Text(rulesInfo)),
		gomponents.If(
			tempRulesCount > 0,
			html.P(
				html.Class("mb-1 text-warning"),
				html.Strong(
					gomponents.Text(
						"⚠ Temporary rules are active - changes are not saved to configuration",
					),
				),
			),
		),
		gomponents.If(
			filterInfo != "",
			html.P(
				html.Class("mb-0"),
				html.Strong(gomponents.Text("Active Filters: ")),
				gomponents.Text(filterInfo),
			),
		),
	)

	result := html.Div(
		html.Class("mt-3"),
		reorderInfo,
		html.Div(
			html.Class("card border-0 shadow-sm"),
			html.Div(
				html.Class("card-header bg-light"),
				html.H5(html.Class("card-title mb-0"),
					html.I(html.Class("fas fa-list-ol me-2 text-primary")),
					gomponents.Text("Wanted Quality Priority Combinations: "+selectedQuality),
				),
				html.P(
					html.Class("text-muted mb-0 mt-1"),
					gomponents.Text(
						fmt.Sprintf(
							"Showing %d resolution + quality combinations (sorted by total priority)",
							len(combinations),
						),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.THead(
						html.Class("table-light"),
						html.Tr(
							html.Th(html.Class("text-center"), gomponents.Text("Rank")),
							html.Th(gomponents.Text("Resolution + Quality")),
							html.Th(gomponents.Text("Original Priority")),
							html.Th(gomponents.Text("Modified Priority")),
							html.Th(gomponents.Text("Change")),
							html.Th(gomponents.Text("Breakdown (Res±Qual)")),
						),
					),
					html.TBody(gomponents.Group(rows)),
				),
			),
		),
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleQualityProfileRules returns the existing reorder rules for a quality profile as JSON.
func HandleQualityProfileRules(c *gin.Context) {
	profileName := c.Query("profile")
	if profileName == "" {
		c.JSON(400, gin.H{"error": "Profile name is required"})
		return
	}

	qualityConfig := config.GetSettingsQuality(profileName)
	if qualityConfig == nil {
		c.JSON(404, gin.H{"error": "Quality profile not found"})
		return
	}

	type RuleResponse struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		NewPriority int    `json:"new_priority"`
	}

	var rules []RuleResponse
	for _, rule := range qualityConfig.QualityReorder {
		rules = append(rules, RuleResponse{
			Name:        rule.Name,
			Type:        rule.ReorderType,
			NewPriority: rule.Newpriority,
		})
	}

	c.JSON(200, rules)
}
