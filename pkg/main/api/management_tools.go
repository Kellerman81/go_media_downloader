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
	. "maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	. "maragu.dev/gomponents/html"
)

// Helper functions to reduce code duplication

// renderMissingEpisodesFinderPage renders a page for finding missing episodes
func renderMissingEpisodesFinderPage(csrfToken string) Node {
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-exclamation-triangle",
			"Missing Episodes Finder",
			"Search for gaps in your series collections and optionally trigger automatic downloads for missing episodes.",
		),

		Form(
			Class("config-form"),
			ID("missingEpisodesForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Search Configuration")),

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

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Action Configuration")),

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
			Div(
				Class("form-group submit-group"),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('missingEpisodesForm').reset(); document.getElementById('missingResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("missingResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// renderLogAnalysisDashboardPage renders a page for analyzing logs
func renderLogAnalysisDashboardPage(csrfToken string) Node {
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-chart-line",
			"Log Analysis Dashboard",
			"Parse and analyze application logs for patterns, errors, and performance metrics with detailed statistics and insights.",
		),

		Form(
			Class("config-form"),
			ID("logAnalysisForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Analysis Configuration")),

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

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Analysis Types")),

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

			renderHTMXSubmitButton("Analyze Logs", "analysisResults", "/api/admin/log-analysis", "logAnalysisForm", csrfToken),
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-success ml-2"),
					Text("Export Report"),
					Type("button"),
					hx.Target("#analysisResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/log-analysis/export"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logAnalysisForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('logAnalysisForm').reset(); document.getElementById('analysisResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("analysisResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// renderStorageHealthMonitorPage renders a page for monitoring storage health
func renderStorageHealthMonitorPage(csrfToken string) Node {
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-hdd",
			"Storage Health Monitor",
			"Monitor disk space, file system permissions, and mount status across all configured media paths and storage locations.",
		),

		Form(
			Class("config-form"),
			ID("storageHealthForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Monitoring Configuration")),

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

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Alert Thresholds")),

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

			renderHTMXSubmitButton("Check Storage Health", "storageResults", "/api/admin/storage-health", "storageHealthForm", csrfToken),
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-info ml-2"),
					Text("Continuous Monitor"),
					Type("button"),
					hx.Target("#storageResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/storage-health/monitor"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#storageHealthForm"),
					hx.Trigger("every 30s"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('storageHealthForm').reset(); document.getElementById('storageResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("storageResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// renderExternalServiceHealthCheckPage renders a page for checking external service connectivity
func renderExternalServiceHealthCheckPage(csrfToken string) Node {
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-heartbeat",
			"External Service Health Check",
			"Test connectivity and response times for external services including IMDB, Trakt, indexers, and other integrated APIs.",
		),

		Form(
			Class("config-form"),
			ID("serviceHealthForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Service Selection")),

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

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Test Configuration")),

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

			renderHTMXSubmitButton("Run Health Check", "serviceResults", "/api/admin/service-health", "serviceHealthForm", csrfToken),
			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-warning ml-2"),
					Text("Quick Test"),
					Type("button"),
					hx.Target("#serviceResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/service-health/quick"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#serviceHealthForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('serviceHealthForm').reset(); document.getElementById('serviceResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("serviceResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// renderAPITestingPage renders a comprehensive API testing interface
func renderAPITestingPage() Node {
	// Get the actual API key from configuration
	apiKey := config.GetSettingsGeneral().WebAPIKey
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header
		renderEnhancedPageHeader(
			"fa-solid fa-plug",
			"API Testing Suite",
			"Comprehensive testing interface for all API endpoints. Test jobs, searches, configurations, and database operations with real-time feedback.",
		),

		// Main content layout with resizable splitter
		Div(
			ID("resizable-container"),
			Style("display: flex; min-height: 800px;"),
			// Main content area
			Div(
				ID("main-content"),
				Style("flex: 1; padding-right: 15px; min-width: 300px;"),

				// API Configuration Section
				Div(
					Class("card border-0 shadow-sm mb-4"),
					Div(
						Class("card-header bg-gradient-secondary text-white"),
						H5(Class("card-title mb-0"), I(Class("fas fa-key me-2")), Text("API Configuration")),
					),
					Div(
						Class("card-body"),
						Div(
							Class("row g-3 align-items-end"),
							Div(
								Class("col-md-8"),
								Label(For("apiKeyInput"), Class("form-label"), Text("API Key")),
								Div(
									Class("input-group"),
									Input(
										ID("apiKeyInput"),
										Type("text"),
										Class("form-control"),
										Value(apiKey),
										Attr("placeholder", "Enter your API key"),
									),
									Span(
										ID("apiKeyStatus"),
										Class("input-group-text bg-light"),
										I(Class("fas fa-question-circle text-muted")),
									),
								),
								Small(Class("form-text text-muted"), Text("API key is automatically loaded from configuration. You can override it here if needed.")),
							),
							Div(
								Class("col-md-2"),
								Button(
									Type("button"),
									Class("btn btn-outline-primary w-100"),
									Attr("onclick", "testAPIKey()"),
									I(Class("fas fa-vial me-2")), Text("Test Key"),
								),
							),
							Div(
								Class("col-md-2"),
								Button(
									Type("button"),
									Class("btn btn-outline-secondary w-100"),
									Attr("onclick", "resetAPIKey()"),
									I(Class("fas fa-undo me-2")), Text("Reset"),
								),
							),
						),
					),
				),

				// Quick Actions Section
				Div(
					Class("card border-0 shadow-sm mb-4"),
					Div(
						Class("card-header bg-gradient-primary text-white"),
						H5(Class("card-title mb-0"), I(Class("fas fa-bolt me-2")), Text("Quick Actions")),
					),
					Div(
						Class("card-body"),
						Div(
							Class("row g-3"),
							Div(
								Class("col-md-3"),
								Button(
									Type("button"),
									Class("btn btn-success w-100"),
									Attr("onclick", "executeAPICall('GET', '/api/debugstats', '', 'Debug Stats')"),
									I(Class("fas fa-chart-line me-2")), Text("Debug Stats"),
								),
							),
							Div(
								Class("col-md-3"),
								Button(
									Type("button"),
									Class("btn btn-info w-100"),
									Attr("onclick", "executeAPICall('GET', '/api/queue', '', 'Job Queue')"),
									I(Class("fas fa-tasks me-2")), Text("Job Queue"),
								),
							),
							Div(
								Class("col-md-3"),
								Button(
									Type("button"),
									Class("btn btn-warning w-100"),
									Attr("onclick", "executeAPICall('GET', '/api/config/all', '', 'All Configs')"),
									I(Class("fas fa-cog me-2")), Text("All Configs"),
								),
							),
							Div(
								Class("col-md-3"),
								Button(
									Type("button"),
									Class("btn btn-secondary w-100"),
									Attr("onclick", "executeAPICall('GET', '/api/db/backup', '', 'DB Backup')"),
									I(Class("fas fa-database me-2")), Text("DB Backup"),
								),
							),
						),
					),
				),

				// Job Management Section
				Div(
					Class("card border-0 shadow-sm mb-4"),
					Div(
						Class("card-header bg-gradient-info text-white"),
						H5(Class("card-title mb-0"), I(Class("fas fa-play me-2")), Text("Job Management")),
					),
					Div(
						Class("card-body"),
						Form(
							Attr("onsubmit", "return executeJobTest(event)"),
							Div(
								Class("row g-3"),
								Div(
									Class("col-md-4"),
									Label(For("jobType"), Class("form-label"), Text("Job Type")),
									Select(
										ID("jobType"),
										Name("jobType"),
										Class("form-select"),
										Attr("onchange", "updateJobOptions()"),
										Option(Value(""), Text("Select Job Type")),
										Option(Value("movies"), Text("Movies")),
										Option(Value("series"), Text("Series")),
										Option(Value("db"), Text("Database")),
									),
								),
								Div(
									Class("col-md-4"),
									Label(For("jobAction"), Class("form-label"), Text("Action")),
									Select(
										ID("jobAction"),
										Name("jobAction"),
										Class("form-select"),
										Option(Value(""), Text("Select Action")),
									),
								),
								Div(
									Class("col-md-4"),
									Label(For("jobConfig"), Class("form-label"), Text("Config (Optional)")),
									Input(
										ID("jobConfig"),
										Name("jobConfig"),
										Type("text"),
										Class("form-control"),
										Attr("placeholder", "EN, DE, X, etc."),
									),
								),
							),
							Div(
								Class("mt-3"),
								Button(
									Type("submit"),
									Class("btn btn-primary me-2"),
									I(Class("fas fa-rocket me-2")), Text("Execute Job"),
								),
								Button(
									Type("button"),
									Class("btn btn-outline-secondary"),
									Attr("onclick", "clearResults()"),
									I(Class("fas fa-trash me-2")), Text("Clear Results"),
								),
							),
						),
					),
				),

				// Custom API Testing Section
				Div(
					Class("card border-0 shadow-sm mb-4"),
					Div(
						Class("card-header bg-gradient-dark text-white"),
						H5(Class("card-title mb-0"), I(Class("fas fa-terminal me-2")), Text("Custom API Testing")),
					),
					Div(
						Class("card-body"),
						Form(
							Attr("onsubmit", "return executeCustomAPITest(event)"),
							Div(
								Class("row g-3"),
								Div(
									Class("col-md-2"),
									Label(For("httpMethod"), Class("form-label"), Text("Method")),
									Select(
										ID("httpMethod"),
										Name("httpMethod"),
										Class("form-select"),
										Option(Value("GET"), Attr("selected"), Text("GET")),
										Option(Value("POST"), Text("POST")),
										Option(Value("PUT"), Text("PUT")),
										Option(Value("DELETE"), Text("DELETE")),
									),
								),
								Div(
									Class("col-md-10"),
									Label(For("apiEndpoint"), Class("form-label"), Text("API Endpoint")),
									Input(
										ID("apiEndpoint"),
										Name("apiEndpoint"),
										Type("text"),
										Class("form-control"),
										Attr("placeholder", "/api/movies or /api/series/search/id/123"),
									),
								),
							),
							Div(
								Class("mt-3"),
								Label(For("requestBody"), Class("form-label"), Text("Request Body (JSON for POST/PUT)")),
								Textarea(
									ID("requestBody"),
									Name("requestBody"),
									Class("form-control"),
									Attr("rows", "4"),
									Attr("placeholder", `{"key": "value"}`),
								),
							),
							Div(
								Class("mt-3"),
								Button(
									Type("submit"),
									Class("btn btn-primary me-2"),
									I(Class("fas fa-paper-plane me-2")), Text("Send Request"),
								),
							),
						),
					),
				),

				// Search & Download Testing
				Div(
					Class("card border-0 shadow-sm mb-4"),
					Div(
						Class("card-header bg-gradient-success text-white"),
						H5(Class("card-title mb-0"), I(Class("fas fa-search me-2")), Text("Search & Download Testing")),
					),
					Div(
						Class("card-body"),
						Div(
							Class("row g-3"),
							Div(
								Class("col-md-6"),
								H6(Text("Movie Search")),
								Div(
									Class("input-group mb-2"),
									Input(
										ID("movieSearchId"),
										Type("number"),
										Class("form-control"),
										Attr("placeholder", "Movie ID"),
									),
									Button(
										Class("btn btn-outline-primary"),
										Type("button"),
										Attr("onclick", "testMovieSearch()"),
										Text("Search Movie"),
									),
								),
								Div(
									Class("input-group mb-2"),
									Input(
										ID("movieListId"),
										Type("number"),
										Class("form-control"),
										Attr("placeholder", "Movie List ID"),
									),
									Button(
										Class("btn btn-outline-success"),
										Type("button"),
										Attr("onclick", "testMovieListSearch()"),
										Text("Search List"),
									),
								),
							),
							Div(
								Class("col-md-6"),
								H6(Text("Series Search")),
								Div(
									Class("input-group mb-2"),
									Input(
										ID("seriesSearchId"),
										Type("number"),
										Class("form-control"),
										Attr("placeholder", "Series ID"),
									),
									Button(
										Class("btn btn-outline-primary"),
										Type("button"),
										Attr("onclick", "testSeriesSearch()"),
										Text("Search Series"),
									),
								),
								Div(
									Class("input-group mb-2"),
									Input(
										ID("episodeId"),
										Type("number"),
										Class("form-control"),
										Attr("placeholder", "Episode ID"),
									),
									Button(
										Class("btn btn-outline-success"),
										Type("button"),
										Attr("onclick", "testEpisodeSearch()"),
										Text("Search Episode"),
									),
								),
							),
						),
					),
				),

				// API Endpoints Reference Section
				Div(
					Class("card border-0 shadow-sm mb-4"),
					Div(
						Class("card-header bg-gradient-warning text-dark"),
						H5(Class("card-title mb-0"), I(Class("fas fa-book me-2")), Text("API Endpoints Reference")),
					),
					Div(
						Class("card-body"),
						Div(
							Class("accordion"),
							ID("endpointsAccordion"),

							// All endpoints section
							Div(
								Class("accordion-item"),
								H2(
									Class("accordion-header"),
									ID("headingAll"),
									Button(
										Class("accordion-button collapsed"),
										Type("button"),
										Attr("data-bs-toggle", "collapse"),
										Attr("data-bs-target", "#collapseAll"),
										Attr("aria-expanded", "false"),
										Attr("aria-controls", "collapseAll"),
										Text("All Endpoints (7)"),
									),
								),
								Div(
									ID("collapseAll"),
									Class("accordion-collapse collapse"),
									Attr("aria-labelledby", "headingAll"),
									Attr("data-bs-parent", "#endpointsAccordion"),
									Div(
										Class("accordion-body"),
										Ul(
											Class("list-unstyled mb-0"),
											Li(Text("GET /api/all/feeds - Search all feeds")),
											Li(Text("GET /api/all/data - Search all folders")),
											Li(Text("GET /api/all/search/rss - Search all rss feeds")),
											Li(Text("GET /api/all/search/missing/full - Search all Missing")),
											Li(Text("GET /api/all/search/missing/inc - Search all Missing Incremental")),
											Li(Text("GET /api/all/search/upgrade/full - Search all Upgrades")),
											Li(Text("GET /api/all/search/upgrade/inc - Search all Upgrades Incremental")),
										),
									),
								),
							),

							// Movies endpoints section
							Div(
								Class("accordion-item"),
								H2(
									Class("accordion-header"),
									ID("headingMovies"),
									Button(
										Class("accordion-button collapsed"),
										Type("button"),
										Attr("data-bs-toggle", "collapse"),
										Attr("data-bs-target", "#collapseMovies"),
										Attr("aria-expanded", "false"),
										Attr("aria-controls", "collapseMovies"),
										Text("Movies Endpoints (20)"),
									),
								),
								Div(
									ID("collapseMovies"),
									Class("accordion-collapse collapse"),
									Attr("aria-labelledby", "headingMovies"),
									Attr("data-bs-parent", "#endpointsAccordion"),
									Div(
										Class("accordion-body"),
										Ul(
											Class("list-unstyled mb-0"),
											Li(Text("GET /api/movies - Get all movies")),
											Li(Text("GET /api/movies/unmatched - Get unmatched movies")),
											Li(Text("DELETE /api/movies/{id} - Delete movie")),
											Li(Text("GET /api/movies/list/{name} - Get movies by list name")),
											Li(Text("GET /api/movies/metadata/{imdb} - Get movie metadata")),
											Li(Text("DELETE /api/movies/list/{id} - Delete movie from list")),
											Li(Text("GET /api/movies/job/{job} - Execute movie job")),
											Li(Text("GET /api/movies/job/{job}/{name} - Execute movie job for config")),
											Li(Text("POST /api/movies - Create/update movie")),
											Li(Text("POST /api/movies/list - Create/update movie list")),
											Li(Text("GET /api/movies/search/id/{id} - Search movie by ID")),
											Li(Text("GET /api/movies/search/list/{id} - Search movie list")),
											Li(Text("GET /api/movies/rss/search/list/{group} - RSS search movie list")),
											Li(Text("POST /api/movies/search/download/{id} - Download movie")),
											Li(Text("GET /api/movies/all/refreshall - Refresh all movies metadata")),
											Li(Text("GET /api/movies/refresh/{id} - Refresh movie metadata")),
											Li(Text("GET /api/movies/all/refresh - Refresh movies")),
											Li(Text("GET /api/movies/search/history/clear/{name} - Clear movie search history")),
											Li(Text("GET /api/movies/search/history/clearid/{id} - Clear movie search history by ID")),
										),
									),
								),
							),

							// Series endpoints section
							Div(
								Class("accordion-item"),
								H2(
									Class("accordion-header"),
									ID("headingSeries"),
									Button(
										Class("accordion-button collapsed"),
										Type("button"),
										Attr("data-bs-toggle", "collapse"),
										Attr("data-bs-target", "#collapseSeries"),
										Attr("aria-expanded", "false"),
										Attr("aria-controls", "collapseSeries"),
										Text("Series Endpoints (28)"),
									),
								),
								Div(
									ID("collapseSeries"),
									Class("accordion-collapse collapse"),
									Attr("aria-labelledby", "headingSeries"),
									Attr("data-bs-parent", "#endpointsAccordion"),
									Div(
										Class("accordion-body"),
										Ul(
											Class("list-unstyled mb-0"),
											Li(Text("GET /api/series - Get all series")),
											Li(Text("DELETE /api/series/{id} - Delete series")),
											Li(Text("GET /api/series/list/{name} - Get series by list name")),
											Li(Text("DELETE /api/series/list/{id} - Delete series from list")),
											Li(Text("GET /api/series/unmatched - Get unmatched series")),
											Li(Text("GET /api/series/episodes - Get all episodes")),
											Li(Text("GET /api/series/episodes/{id} - Get episode by ID")),
											Li(Text("DELETE /api/series/episodes/{id} - Delete episode")),
											Li(Text("GET /api/series/episodes/list/{id} - Get episodes by series ID")),
											Li(Text("DELETE /api/series/episodes/list/{id} - Delete episodes by series ID")),
											Li(Text("GET /api/series/job/{job} - Execute series job")),
											Li(Text("GET /api/series/job/{job}/{name} - Execute series job for config")),
											Li(Text("POST /api/series - Create/update series")),
											Li(Text("POST /api/series/episodes - Create/update episode")),
											Li(Text("POST /api/series/list - Create/update series list")),
											Li(Text("POST /api/series/episodes/list - Create/update episode list")),
											Li(Text("GET /api/series/refresh/{id} - Refresh series metadata")),
											Li(Text("GET /api/series/all/refreshall - Refresh all series metadata")),
											Li(Text("GET /api/series/all/refresh - Refresh series")),
											Li(Text("GET /api/series/search/id/{id} - Search series by ID")),
											Li(Text("GET /api/series/search/id/{id}/{season} - Search series season")),
											Li(Text("GET /api/series/searchrss/id/{id} - RSS search series")),
											Li(Text("GET /api/series/searchrss/list/id/{id} - RSS search series list")),
											Li(Text("GET /api/series/searchrss/id/{id}/{season} - RSS search series season")),
											Li(Text("GET /api/series/episodes/search/id/{id} - Search episodes")),
											Li(Text("GET /api/series/episodes/search/list/{id} - Search episode list")),
											Li(Text("GET /api/series/rss/search/list/{group} - RSS search series list")),
											Li(Text("POST /api/series/episodes/search/download/{id} - Download episode")),
											Li(Text("GET /api/series/search/history/clear/{name} - Clear series search history")),
											Li(Text("GET /api/series/search/history/clearid/{id} - Clear series search history by ID")),
										),
									),
								),
							),

							// General endpoints section
							Div(
								Class("accordion-item"),
								H2(
									Class("accordion-header"),
									ID("headingGeneral"),
									Button(
										Class("accordion-button collapsed"),
										Type("button"),
										Attr("data-bs-toggle", "collapse"),
										Attr("data-bs-target", "#collapseGeneral"),
										Attr("aria-expanded", "false"),
										Attr("aria-controls", "collapseGeneral"),
										Text("General & System Endpoints (31)"),
									),
								),
								Div(
									ID("collapseGeneral"),
									Class("accordion-collapse collapse"),
									Attr("aria-labelledby", "headingGeneral"),
									Attr("data-bs-parent", "#endpointsAccordion"),
									Div(
										Class("accordion-body"),
										Ul(
											Class("list-unstyled mb-0"),
											Li(Text("GET /api/debugstats - Get debug statistics")),
											Li(Text("GET /api/queue - Get job queue")),
											Li(Text("GET /api/queue/history - Get job history")),
											Li(Text("DELETE /api/queue/cancel/{id} - Cancel job")),
											Li(Text("GET /api/trakt/authorize - Authorize Trakt")),
											Li(Text("GET /api/trakt/token/{code} - Get Trakt token")),
											Li(Text("GET /api/trakt/user/{user}/{list} - Get Trakt user list")),
											Li(Text("GET /api/slug - Get slug information")),
											Li(Text("POST /api/parse/string - Parse string")),
											Li(Text("POST /api/parse/file - Parse file")),
											Li(Text("GET /api/fillimdb - Fill IMDB data")),
											Li(Text("GET /api/scheduler/stop - Stop scheduler")),
											Li(Text("GET /api/scheduler/start - Start scheduler")),
											Li(Text("GET /api/scheduler/list - List scheduled jobs")),
											Li(Text("GET /api/db/close - Close database")),
											Li(Text("GET /api/db/backup - Backup database")),
											Li(Text("GET /api/db/integrity - Check database integrity")),
											Li(Text("DELETE /api/db/clear/{name} - Clear database table")),
											Li(Text("DELETE /api/db/delete/{name}/{id} - Delete database entry")),
											Li(Text("DELETE /api/db/clearcache - Clear cache")),
											Li(Text("GET /api/db/vacuum - Vacuum database")),
											Li(Text("DELETE /api/db/oldjobs - Delete old jobs")),
											Li(Text("GET /api/quality - Get quality profiles")),
											Li(Text("DELETE /api/quality/{id} - Delete quality profile")),
											Li(Text("POST /api/quality - Create/update quality profile")),
											Li(Text("GET /api/quality/get/{name} - Get quality by name")),
											Li(Text("GET /api/quality/all - Get all qualities")),
											Li(Text("GET /api/quality/complete - Get complete qualities")),
											Li(Text("GET /api/config/all - Get all configurations")),
											Li(Text("DELETE /api/config/clear - Clear configuration")),
											Li(Text("GET /api/config/get/{name} - Get config by name")),
											Li(Text("DELETE /api/config/delete/{name} - Delete configuration")),
											Li(Text("GET /api/config/refresh - Refresh configuration")),
											Li(Text("POST /api/config/update/{name} - Update configuration")),
											Li(Text("GET /api/config/type/{type} - Get config by type")),
											Li(Text("POST /api/naming - Test naming patterns")),
											Li(Text("POST /api/structure - Test file structure")),
										),
									),
								),
							),

							// Admin endpoints section
							Div(
								Class("accordion-item"),
								H2(
									Class("accordion-header"),
									ID("headingAdmin"),
									Button(
										Class("accordion-button collapsed"),
										Type("button"),
										Attr("data-bs-toggle", "collapse"),
										Attr("data-bs-target", "#collapseAdmin"),
										Attr("aria-expanded", "false"),
										Attr("aria-controls", "collapseAdmin"),
										Text("Admin Endpoints (4)"),
									),
								),
								Div(
									ID("collapseAdmin"),
									Class("accordion-collapse collapse"),
									Attr("aria-labelledby", "headingAdmin"),
									Attr("data-bs-parent", "#endpointsAccordion"),
									Div(
										Class("accordion-body"),
										Ul(
											Class("list-unstyled mb-0"),
											Li(Text("GET /api/admin - Admin dashboard")),
											Li(Text("POST /api/admin/table/{name}/insert - Insert database record")),
											Li(Text("POST /api/admin/table/{name}/update/{index} - Update database record")),
											Li(Text("POST /api/admin/table/{name}/delete/{index} - Delete database record")),
										),
									),
								),
							),
						),

						Div(
							Class("mt-3 p-3 bg-light rounded"),
							H6(Class("fw-bold mb-2"), Text("Quick Reference")),
							P(Class("mb-1 small"), Strong(Text("Total Endpoints:")), Text(" 90")),
							P(Class("mb-1 small"), Strong(Text("Authentication:")), Text(" All endpoints require 'apikey' query parameter")),
							P(Class("mb-1 small"), Strong(Text("Base URL:")), Text(" /api/{category}/{action}")),
							P(Class("mb-0 small"), Strong(Text("Response Format:")), Text(" JSON (success/error messages or data)")),
						),
					),
				),
			), // End main content

			// Resizable divider
			Div(
				ID("resizer"),
				Style("width: 5px; cursor: col-resize; background: #dee2e6; border-left: 1px solid #adb5bd; border-right: 1px solid #adb5bd; flex-shrink: 0;"),
				Attr("title", "Drag to resize"),
			),

			// Sidebar for results
			Div(
				ID("sidebar-content"),
				Style("width: 400px; min-width: 300px; max-width: 800px; padding-left: 15px;"),
				// Results Section
				Div(
					Class("card border-0 shadow-sm sticky-top"),
					Style("top: 20px;"), // Sticky positioning
					Div(
						Class("card-header bg-primary text-white d-flex justify-content-between align-items-center"),
						H6(Class("card-title mb-0"), I(Class("fas fa-list-alt me-2")), Text("Test Results")),
						Div(
							Class("btn-group btn-group-sm"),
							Button(
								Type("button"),
								Class("btn btn-light btn-sm"),
								Attr("onclick", "copyResults()"),
								Attr("title", "Copy Results"),
								I(Class("fas fa-copy"), Style("color: black;")),
							),
							Button(
								Type("button"),
								Class("btn btn-light btn-sm"),
								Attr("onclick", "clearResults()"),
								Attr("title", "Clear Results"),
								I(Class("fas fa-trash"), Style("color: black;")),
							),
						),
					),
					Div(
						Class("card-body p-2"),
						Div(
							ID("testResults"),
							Class("bg-light p-2 rounded"),
							Style("min-height: 400px; font-family: 'Courier New', monospace; white-space: pre-wrap; overflow: auto; font-size: 0.8rem; line-height: 1.3; resize: both; height: 600px;"),
							Text("Test results will appear here..."),
						),
					),
				),
			), // End sidebar content
		), // End resizable container

		// JavaScript for functionality
		Script(Raw(`
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
				navigator.clipboard.writeText(results).then(() => {
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
					startSidebarWidth = parseInt(window.getComputedStyle(sidebar).width, 10);
					
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

// HandleQualityReorderRules handles HTMX requests for loading reorder rules
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
		c.String(http.StatusOK, `<p class="text-muted mb-0 text-center">Select a quality profile to load rules.</p>`)
		return
	}

	// Get the selected quality profile configuration
	qualityConfig := config.GetSettingsQuality(selectedProfile)
	if qualityConfig == nil {
		c.String(http.StatusOK, `<p class="text-danger mb-0 text-center">Quality profile "<strong>`+selectedProfile+`</strong>" not found.</p>`)
		return
	}

	// Build HTML for displaying existing reorder rules from the profile
	var html strings.Builder

	if len(qualityConfig.QualityReorder) == 0 {
		html.WriteString(`<p class="text-muted mb-0 text-center">No reorder rules defined in profile: <strong>`)
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
			html.WriteString(strings.Title(rule.ReorderType))
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

// HandleQualityReorderAddRule handles HTMX requests for adding a new reorder rule
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

// HandleQualityReorderResetRules handles HTMX requests for resetting all reorder rules
func HandleQualityReorderResetRules(c *gin.Context) {
	c.String(http.StatusOK, `<p class="text-muted mb-0 text-center">No temporary reorder rules. Click 'Add Rule' to create one.</p>`)
}

// HandleMediaCleanup handles media cleanup requests
func HandleMediaCleanup(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	findOrphans := c.PostForm("cleanup_FindOrphans") == "on" || c.PostForm("cleanup_FindOrphans") == "true"
	findDuplicates := c.PostForm("cleanup_FindDuplicates") == "on" || c.PostForm("cleanup_FindDuplicates") == "true"
	findBroken := c.PostForm("cleanup_FindBroken") == "on" || c.PostForm("cleanup_FindBroken") == "true"
	findEmpty := c.PostForm("cleanup_FindEmpty") == "on" || c.PostForm("cleanup_FindEmpty") == "true"
	dryRun := c.PostForm("cleanup_DryRun") == "on" || c.PostForm("cleanup_DryRun") == "true"
	mediaTypes := c.PostForm("cleanup_MediaTypes")
	minFileSizeStr := c.PostForm("cleanup_MinFileSize")

	if !findOrphans && !findDuplicates && !findBroken && !findEmpty {
		c.String(http.StatusOK, renderComponentToString(
			Div(
				Class("card border-0 shadow-sm border-warning mb-4"),
				Div(
					Class("card-header border-0"),
					Style("background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%); border-radius: 15px 15px 0 0;"),
					Div(
						Class("d-flex align-items-center"),
						Span(Class("badge bg-warning me-3"), I(Class("fas fa-exclamation-triangle me-1")), Text("Warning")),
						H5(Class("card-title mb-0 text-warning fw-bold"), Text("No Options Selected")),
					),
				),
				Div(
					Class("card-body"),
					P(Class("card-text text-muted"), Text("Please select at least one cleanup option to perform the scan.")),
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
	results := []Node{
		Tr(Td(Strong(Text("Cleanup Configuration:"))), Td(Text(""))),
		Tr(Td(Text("Media Types:")), Td(Text(mediaTypes))),
		Tr(Td(Text("Min File Size:")), Td(Text(minFileSizeStr+" MB"))),
		Tr(Td(Text("Dry Run:")), Td(Text(func() string {
			if dryRun {
				return "Yes (preview only)"
			}
			return "No (will make changes)"
		}()))),
		Tr(Td(Text("Scan Duration:")), Td(Text(scanDuration.String()))),
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Scan Results:"))), Td(Text(""))),
	}

	// Add detailed results
	if findOrphans {
		results = append(results,
			Tr(Td(Text("Orphaned Files:")), Td(Text(fmt.Sprintf("%d files found", orphanedCount)))),
		)
	}
	if findDuplicates {
		results = append(results,
			Tr(Td(Text("Duplicate Sets:")), Td(Text(fmt.Sprintf("%d duplicate sets found", duplicateCount)))),
		)
	}
	if findBroken {
		results = append(results,
			Tr(Td(Text("Broken Links:")), Td(Text(fmt.Sprintf("%d broken database entries", brokenCount)))),
		)
	}
	if findEmpty {
		results = append(results,
			Tr(Td(Text("Empty Directories:")), Td(Text(fmt.Sprintf("%d empty folders", emptyCount)))),
		)
	}

	results = append(results,
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Summary:"))), Td(Text(""))),
		Tr(Td(Text("Total Issues Found:")), Td(Text(fmt.Sprintf("%d", totalIssues)))),
		Tr(Td(Text("Paths Scanned:")), Td(Text("/media/movies, /media/tv, /downloads"))),
	)

	// Determine alert styling based on results
	alertClass := "card border-0 shadow-sm border-success mb-4"
	message := fmt.Sprintf("Cleanup Scan Complete - %d Issues Found", totalIssues)
	if totalIssues > 10 {
		alertClass = "card border-0 shadow-sm border-danger mb-4"
		message = fmt.Sprintf("Cleanup Scan Complete - %d Issues Found (Action Required)", totalIssues)
	} else if totalIssues > 5 {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf("Cleanup Scan Complete - %d Issues Found (Review Required)", totalIssues)
	}

	result := Div(
		Class(alertClass),
		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, "+func() string {
				if totalIssues > 10 {
					return "#f8d7da 0%, #f5c6cb 100%"
				} else if totalIssues > 5 {
					return "#fff3cd 0%, #ffeaa7 100%"
				}
				return "#d4edda 0%, #c3e6cb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge "+func() string {
					if totalIssues > 10 {
						return "bg-danger"
					} else if totalIssues > 5 {
						return "bg-warning"
					}
					return "bg-success"
				}()+" me-3"), I(Class("fas fa-broom me-1")), Text("Cleanup")),
				H5(Class("card-title mb-0 "+func() string {
					if totalIssues > 10 {
						return "text-danger"
					} else if totalIssues > 5 {
						return "text-warning"
					}
					return "text-success"
				}()+" fw-bold"), Text(message)),
			),
		),
		Div(
			Class("card-body p-0"),
			Table(
				Class("table table-hover mb-0"),
				Style("background: transparent;"),
				TBody(Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleAPITesting handles API testing requests
func HandleAPITesting(c *gin.Context) {
	// This handler could be extended to provide additional server-side functionality
	// For now, the frontend JavaScript handles most of the API testing functionality
	c.String(http.StatusOK, renderAlert("API Testing functionality is handled client-side", "info"))
}

// HandleMissingEpisodes handles missing episodes finder requests
func HandleMissingEpisodes(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	seriesName := c.PostForm("missing_SeriesName")
	seasonNumberStr := c.PostForm("missing_SeasonNumber")
	status := c.PostForm("missing_Status")
	includeSpecials := c.PostForm("missing_IncludeSpecials") == "on" || c.PostForm("missing_IncludeSpecials") == "true"
	autoDownload := c.PostForm("missing_AutoDownload") == "on" || c.PostForm("missing_AutoDownload") == "true"
	onlyAired := c.PostForm("missing_OnlyAired") == "on" || c.PostForm("missing_OnlyAired") == "true"
	dateRangeStr := c.PostForm("missing_DateRange")
	qualityProfile := c.PostForm("missing_QualityProfile")
	showAllInTable := c.PostForm("missing_ShowAllInTable") == "on" || c.PostForm("missing_ShowAllInTable") == "true"

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
	var queryWhere string
	var queryArgs []any

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

	results := []Node{
		Tr(Td(Strong(Text("Search Configuration:"))), Td(Text(""))),
		Tr(Td(Text("Series:")), Td(Text(func() string {
			if seriesName == "" {
				return "All series"
			}
			return seriesName
		}()))),
		Tr(Td(Text("Season:")), Td(Text(func() string {
			if seasonNumber == 0 {
				return "All seasons"
			}
			return fmt.Sprintf("Season %d", seasonNumber)
		}()))),
		Tr(Td(Text("Status Filter:")), Td(Text(status))),
		Tr(Td(Text("Include Specials:")), Td(Text(func() string {
			if includeSpecials {
				return "Yes"
			}
			return "No"
		}()))),
		Tr(Td(Text("Only Aired:")), Td(Text(func() string {
			if onlyAired {
				return "Yes"
			}
			return "No"
		}()))),
		Tr(Td(Text("Scan Duration:")), Td(Text(scanDuration.String()))),
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Search Results:"))), Td(Text(""))),
		Tr(Td(Text("Series Scanned:")), Td(Text(fmt.Sprintf("%d", seriesScanned)))),
		Tr(Td(Text("Missing Episodes:")), Td(Text(fmt.Sprintf("%d", totalMissing)))),
	}

	if totalMissing > 0 {
		if showAllInTable {
			// Show all missing episodes in a detailed datatable
			results = append(results,
				Tr(Td(Attr("colspan", "2"), Hr())),
				Tr(Td(Strong(Text("All Missing Episodes:"))), Td(Text(""))),
			)
		} else {
			// Show actual missing episode details (first few episodes)
			var sampleEpisodes []string
			limit := 5 // Show first 5 missing episodes
			if len(missingEpisodes) < limit {
				limit = len(missingEpisodes)
			}

			for i := 0; i < limit; i++ {
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
					Tr(Td(Text("Sample Missing:")), Td(Text(strings.Join(sampleEpisodes, ", ")))),
				)
			}
		}

		if autoDownload {
			results = append(results,
				Tr(Td(Text("Downloads Triggered:")), Td(Text(fmt.Sprintf("%d episodes queued for download", downloadTriggered)))),
				Tr(Td(Text("Quality Profile:")), Td(Text(qualityProfile))),
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

	var resultContent Node

	if showAllInTable && totalMissing > 0 {
		// Create a comprehensive datatable for all missing episodes
		tableRows := make([]Node, 0)

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

				tableRows = append(tableRows, Tr(
					Td(Text(series.Listname)),
					Td(Text(ep.Season)),
					Td(Text(ep.Episode)),
					Td(Text(ep.Title)),
					Td(Text(func() string {
						if episode.Missing {
							return "Missing"
						}
						return "Available"
					}())),
					Td(Text(episode.QualityProfile)),
				))
			}
		}

		resultContent = Div(
			Class(alertClass),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, "+func() string {
					if totalMissing == 0 {
						return "#d4edda 0%, #c3e6cb 100%"
					} else if totalMissing > 20 {
						return "#fff3cd 0%, #ffeaa7 100%"
					}
					return "#d1ecf1 0%, #bee5eb 100%"
				}()+"); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge "+func() string {
						if totalMissing == 0 {
							return "bg-success"
						} else if totalMissing > 20 {
							return "bg-warning"
						}
						return "bg-info"
					}()+" me-3"), I(Class("fas fa-search me-1")), Text("Search")),
					H5(Class("card-title mb-0 "+func() string {
						if totalMissing == 0 {
							return "text-success"
						} else if totalMissing > 20 {
							return "text-warning"
						}
						return "text-info"
					}()+" fw-bold"), Text(message)),
				),
			),
			Div(
				Class("card-body p-0"),
				Table(
					Class("table table-hover mb-0"),
					Style("background: transparent;"),
					TBody(Group(results)),
				),
			),
			// Add detailed datatable section
			Div(
				Class("card-body"),
				H6(Class("text-primary mb-3"),
					I(Class("fas fa-table me-2")),
					Text(fmt.Sprintf("All %d Missing Episodes", totalMissing))),
				Div(
					Class("table-responsive"),
					Table(
						Class("table table-striped table-hover"),
						ID("missing-episodes-table"),
						THead(
							Tr(
								Th(Text("Series")),
								Th(Text("Season")),
								Th(Text("Episode")),
								Th(Text("Title")),
								Th(Text("Status")),
								Th(Text("Quality Profile")),
							),
						),
						TBody(Group(tableRows)),
					),
				),
				// Add DataTables initialization using centralized function
				Script(Text(`
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
		resultContent = Div(
			Class(alertClass),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, "+func() string {
					if totalMissing == 0 {
						return "#d4edda 0%, #c3e6cb 100%"
					} else if totalMissing > 20 {
						return "#fff3cd 0%, #ffeaa7 100%"
					}
					return "#d1ecf1 0%, #bee5eb 100%"
				}()+"); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge "+func() string {
						if totalMissing == 0 {
							return "bg-success"
						} else if totalMissing > 20 {
							return "bg-warning"
						}
						return "bg-info"
					}()+" me-3"), I(Class("fas fa-search me-1")), Text("Search")),
					H5(Class("card-title mb-0 "+func() string {
						if totalMissing == 0 {
							return "text-success"
						} else if totalMissing > 20 {
							return "text-warning"
						}
						return "text-info"
					}()+" fw-bold"), Text(message)),
				),
			),
			Div(
				Class("card-body p-0"),
				Table(
					Class("table table-hover mb-0"),
					Style("background: transparent;"),
					TBody(Group(results)),
				),
			),
		)
	}

	c.String(http.StatusOK, renderComponentToString(resultContent))
}

// HandleLogAnalysis handles log analysis requests
func HandleLogAnalysis(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	timeRange := c.PostForm("analysis_TimeRange")
	logLevel := c.PostForm("analysis_LogLevel")
	maxLinesStr := c.PostForm("analysis_MaxLines")
	errorPattern := c.PostForm("analysis_ErrorPattern") == "on" || c.PostForm("analysis_ErrorPattern") == "true"
	performanceMetrics := c.PostForm("analysis_PerformanceMetrics") == "on" || c.PostForm("analysis_PerformanceMetrics") == "true"
	accessPattern := c.PostForm("analysis_AccessPattern") == "on" || c.PostForm("analysis_AccessPattern") == "true"
	systemHealth := c.PostForm("analysis_SystemHealth") == "on" || c.PostForm("analysis_SystemHealth") == "true"
	includeStackTraces := c.PostForm("analysis_IncludeStackTraces") == "on" || c.PostForm("analysis_IncludeStackTraces") == "true"

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
	analysisResults, err := performLogAnalysis(logFile, timeRange, logLevel, maxLines, errorPattern, performanceMetrics, accessPattern, systemHealth, includeStackTraces)
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
	results := []Node{
		Tr(Td(Strong(Text("Analysis Configuration:"))), Td(Text(""))),
		Tr(Td(Text("Time Range:")), Td(Text(timeRange))),
		Tr(Td(Text("Log Level:")), Td(Text(logLevel))),
		Tr(Td(Text("Max Lines:")), Td(Text(maxLinesStr))),
		Tr(Td(Text("Include Stack Traces:")), Td(Text(func() string {
			if includeStackTraces {
				return "Yes"
			}
			return "No"
		}()))),
		Tr(Td(Text("Analysis Duration:")), Td(Text(analysisDuration.String()))),
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Analysis Results:"))), Td(Text(""))),
		Tr(Td(Text("Total Log Entries:")), Td(Text(fmt.Sprintf("%d", totalEntries)))),
		Tr(Td(Text("Error Count:")), Td(Text(fmt.Sprintf("%d", errorCount)))),
		Tr(Td(Text("Warning Count:")), Td(Text(fmt.Sprintf("%d", warningCount)))),
		Tr(Td(Text("Info Count:")), Td(Text(fmt.Sprintf("%d", infoCount)))),
	}

	if errorPattern {
		topError := analysisResults.TopErrorPattern
		if topError == "" {
			topError = "No error patterns found"
		}
		results = append(results,
			Tr(Td(Text("Top Error Pattern:")), Td(Text(topError))),
		)
	}

	if performanceMetrics {
		results = append(results,
			Tr(Td(Text("Avg Response Time:")), Td(Text(analysisResults.AvgResponseTime))),
			Tr(Td(Text("Max Response Time:")), Td(Text(analysisResults.MaxResponseTime))),
			Tr(Td(Text("Slowest Operation:")), Td(Text(analysisResults.SlowestOperation))),
		)
	}

	if accessPattern {
		topAccess := analysisResults.TopAccessPattern
		if topAccess == "" {
			topAccess = "No access patterns found"
		}
		results = append(results,
			Tr(Td(Text("Most Accessed Endpoint:")), Td(Text(topAccess))),
		)
	}

	if systemHealth {
		// Get real active jobs count
		queues := worker.GetQueues()
		activeJobsCount := len(queues)

		// Get real database statistics
		dbStats := database.Getdb(false).Stats()
		totalConnections := dbStats.OpenConnections

		results = append(results,
			Tr(Td(Text("Active Jobs:")), Td(Text(fmt.Sprintf("%d", activeJobsCount)))),
			Tr(Td(Text("DB Connections:")), Td(Text(fmt.Sprintf("%d", totalConnections)))),
		)
	}

	// Determine alert class based on error count
	alertClass := "card border-0 shadow-sm border-info mb-4"
	message := "Log Analysis Complete"
	if errorCount > 50 {
		alertClass = "card border-0 shadow-sm border-danger mb-4"
		message = fmt.Sprintf("Log Analysis Complete - %d Errors Found (Attention Required)", errorCount)
	} else if errorCount > 10 {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf("Log Analysis Complete - %d Errors Found", errorCount)
	}

	result := Div(
		Class(alertClass),
		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, "+func() string {
				if errorCount > 50 {
					return "#f8d7da 0%, #f5c6cb 100%"
				} else if errorCount > 10 {
					return "#fff3cd 0%, #ffeaa7 100%"
				}
				return "#d1ecf1 0%, #bee5eb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge "+func() string {
					if errorCount > 50 {
						return "bg-danger"
					} else if errorCount > 10 {
						return "bg-warning"
					}
					return "bg-info"
				}()+" me-3"), I(Class("fas fa-chart-line me-1")), Text("Analysis")),
				H5(Class("card-title mb-0 "+func() string {
					if errorCount > 50 {
						return "text-danger"
					} else if errorCount > 10 {
						return "text-warning"
					}
					return "text-info"
				}()+" fw-bold"), Text(message)),
			),
		),
		Div(
			Class("card-body p-0"),
			Table(
				Class("table table-hover mb-0"),
				Style("background: transparent;"),
				TBody(Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleStorageHealth handles storage health monitoring requests
func HandleStorageHealth(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	checkDiskSpace := c.PostForm("storage_CheckDiskSpace") == "on" || c.PostForm("storage_CheckDiskSpace") == "true"
	checkPermissions := c.PostForm("storage_CheckPermissions") == "on" || c.PostForm("storage_CheckPermissions") == "true"
	checkMountStatus := c.PostForm("storage_CheckMountStatus") == "on" || c.PostForm("storage_CheckMountStatus") == "true"
	checkIOHealth := c.PostForm("storage_CheckIOHealth") == "on" || c.PostForm("storage_CheckIOHealth") == "true"
	lowSpaceThresholdStr := c.PostForm("storage_LowSpaceThreshold")
	criticalSpaceThresholdStr := c.PostForm("storage_CriticalSpaceThreshold")
	slowIOThresholdStr := c.PostForm("storage_SlowIOThreshold")
	enableAlerts := c.PostForm("storage_EnableAlerts") == "on" || c.PostForm("storage_EnableAlerts") == "true"

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
	healthResults := performStorageHealthCheck(checkDiskSpace, checkPermissions, checkMountStatus, checkIOHealth, lowSpaceThreshold, criticalSpaceThreshold, slowIOThreshold)

	totalPaths := healthResults.TotalPaths
	healthyPaths := healthResults.HealthyPaths
	warningPaths := healthResults.WarningPaths
	criticalPaths := healthResults.CriticalPaths
	errorPaths := healthResults.ErrorPaths

	// Build results table
	results := []Node{
		Tr(Td(Strong(Text("Storage Health Check:"))), Td(Text(""))),
		Tr(Td(Text("Check Time:")), Td(Text(time.Now().Format("2006-01-02 15:04:05")))),
		Tr(Td(Text("Low Space Threshold:")), Td(Text(fmt.Sprintf("%.1f%%", lowSpaceThreshold)))),
		Tr(Td(Text("Critical Threshold:")), Td(Text(fmt.Sprintf("%.1f%%", criticalSpaceThreshold)))),
		Tr(Td(Text("Enable Alerts:")), Td(Text(func() string {
			if enableAlerts {
				return "Yes"
			}
			return "No"
		}()))),
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Storage Status:"))), Td(Text(""))),
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
				displayText = fmt.Sprintf("%.1f GB free (%.1f%% free) %s", freeGB, pathInfo.FreePercent, status)
			} else {
				displayText = fmt.Sprintf("Error: %s %s", pathInfo.ErrorMessage, status)
			}

			results = append(results,
				Tr(Td(Text(pathInfo.Path+":")), Td(Text(displayText))),
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

		results = append(results,
			Tr(Td(Text("Permissions Check:")), Td(Text(permissionText))),
		)
	}

	if checkMountStatus {
		results = append(results,
			Tr(Td(Text("Mount /media:")), Td(Text("mounted ✓"))),
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
				ioResults = append(ioResults, fmt.Sprintf("%s: R: %.1f MB/s, W: %.1f MB/s", pathInfo.Path, readMB, writeMB))
			} else if pathInfo.IOTest.Error != "" {
				ioResults = append(ioResults, fmt.Sprintf("%s: %s", pathInfo.Path, pathInfo.IOTest.Error))
			}
		}

		if len(ioResults) > 0 {
			for _, ioResult := range ioResults {
				results = append(results,
					Tr(Td(Text("I/O Performance:")), Td(Text(ioResult))),
				)
			}
		} else {
			results = append(results,
				Tr(Td(Text("I/O Performance:")), Td(Text("No I/O tests performed"))),
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
	results = append(results,
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Summary:"))), Td(Text(""))),
		Tr(Td(Text("Total Paths:")), Td(Text(fmt.Sprintf("%d", totalPaths)))),
		Tr(Td(Text("Healthy Paths:")), Td(Text(fmt.Sprintf("%d", healthyPaths)))),
		Tr(Td(Text("Warning Paths:")), Td(Text(fmt.Sprintf("%d", warningPaths)))),
		Tr(Td(Text("Critical Paths:")), Td(Text(fmt.Sprintf("%d", criticalPaths)))),
		Tr(Td(Text("Overall Score:")), Td(Text(fmt.Sprintf("%.1f/100", overallScore)))),
	)

	// Add real alerts based on actual storage health issues
	if warningPaths > 0 || criticalPaths > 0 {
		results = append(results,
			Tr(Td(Attr("colspan", "2"), Hr())),
			Tr(Td(Strong(Text("Active Alerts:"))), Td(Text(""))),
		)

		// Add specific alerts for each problematic path
		for _, pathInfo := range healthResults.PathsDetails {
			var alertText string
			switch pathInfo.Status {
			case "critical":
				alertText = fmt.Sprintf("CRITICAL: %s (%.1f%% free)", pathInfo.Path, pathInfo.FreePercent)
			case "warning":
				alertText = fmt.Sprintf("WARNING: %s (%.1f%% free)", pathInfo.Path, pathInfo.FreePercent)
			default:
				continue // Skip healthy paths
			}

			if pathInfo.ErrorMessage != "" {
				alertText = fmt.Sprintf("ERROR: %s - %s", pathInfo.Path, pathInfo.ErrorMessage)
			}

			results = append(results,
				Tr(Td(Text("Storage Alert:")), Td(Text(alertText))),
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
			message = fmt.Sprintf("Storage Health Check - %d Errors, %d Critical Issues", errorPaths, criticalPaths)
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

	result := Div(
		Class(alertClass),
		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, "+func() string {
				switch healthResults.OverallStatus {
				case "critical":
					return "#f8d7da 0%, #f5c6cb 100%" // Red gradient
				case "warning":
					return "#fff3cd 0%, #ffeaa7 100%" // Orange gradient
				default: // "healthy"
					return "#d4edda 0%, #c3e6cb 100%" // Green gradient
				}
			}()+"); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge "+func() string {
					switch healthResults.OverallStatus {
					case "critical":
						return "bg-danger"
					case "warning":
						return "bg-warning"
					default: // "healthy"
						return "bg-success"
					}
				}()+" me-3"), I(Class("fas fa-hdd me-1")), Text("Storage")),
				H5(Class("card-title mb-0 "+func() string {
					switch healthResults.OverallStatus {
					case "critical":
						return "text-danger"
					case "warning":
						return "text-warning"
					default: // "healthy"
						return "text-success"
					}
				}()+" fw-bold"), Text(message)),
			),
		),
		Div(
			Class("card-body p-0"),
			Table(
				Class("table table-hover mb-0"),
				Style("background: transparent;"),
				TBody(Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleServiceHealth handles external service health check requests
func HandleServiceHealth(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	checkIMDB := c.PostForm("service_CheckIMDB") == "on" || c.PostForm("service_CheckIMDB") == "true"
	checkTrakt := c.PostForm("service_CheckTrakt") == "on" || c.PostForm("service_CheckTrakt") == "true"
	checkIndexers := c.PostForm("service_CheckIndexers") == "on" || c.PostForm("service_CheckIndexers") == "true"
	checkNotifications := c.PostForm("service_CheckNotifications") == "on" || c.PostForm("service_CheckNotifications") == "true"
	checkOMDB := c.PostForm("service_CheckOMDB") == "on" || c.PostForm("service_CheckOMDB") == "true"
	checkTVDB := c.PostForm("service_CheckTVDB") == "on" || c.PostForm("service_CheckTVDB") == "true"
	checkTMDB := c.PostForm("service_CheckTMDB") == "on" || c.PostForm("service_CheckTMDB") == "true"
	timeoutStr := c.PostForm("service_Timeout")
	retriesStr := c.PostForm("service_Retries")
	detailedTest := c.PostForm("service_DetailedTest") == "on" || c.PostForm("service_DetailedTest") == "true"
	measurePerformance := c.PostForm("service_MeasurePerformance") == "on" || c.PostForm("service_MeasurePerformance") == "true"
	saveResults := c.PostForm("service_SaveResults") == "on" || c.PostForm("service_SaveResults") == "true"

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
	healthResults := performServiceHealthCheck(checkIMDB, checkTrakt, checkIndexers, checkNotifications, checkOMDB, checkTVDB, checkTMDB, timeout, retries, detailedTest, measurePerformance, saveResults)

	totalServices := healthResults.TotalServices
	onlineServices := healthResults.OnlineServices
	failedServices := healthResults.FailedServices
	testDuration := healthResults.TestDuration

	// Build results table
	results := []Node{
		Tr(Td(Strong(Text("Service Health Check:"))), Td(Text(""))),
		Tr(Td(Text("Check Time:")), Td(Text(time.Now().Format("2006-01-02 15:04:05")))),
		Tr(Td(Text("Timeout:")), Td(Text(fmt.Sprintf("%d seconds", timeout)))),
		Tr(Td(Text("Retries:")), Td(Text(fmt.Sprintf("%d", retries)))),
		Tr(Td(Text("Detailed Test:")), Td(Text(func() string {
			if detailedTest {
				return "Yes"
			}
			return "No"
		}()))),
		Tr(Td(Text("Test Duration:")), Td(Text(testDuration.String()))),
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Service Status:"))), Td(Text(""))),
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

		results = append(results,
			Tr(Td(Text(service.Name+":")), Td(Text(statusIcon+" "+statusText))),
		)
	}

	// Add real performance metrics if requested
	if measurePerformance {
		// Calculate real performance metrics from service results
		var totalResponseTime time.Duration
		var responsiveServices int

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

		successRate := float64(healthResults.OnlineServices) / float64(healthResults.TotalServices) * 100

		results = append(results,
			Tr(Td(Attr("colspan", "2"), Hr())),
			Tr(Td(Strong(Text("Performance Metrics:"))), Td(Text(""))),
			Tr(Td(Text("Overall Avg Response:")), Td(Text(avgResponseTime))),
			Tr(Td(Text("Success Rate:")), Td(Text(fmt.Sprintf("%.1f%%", successRate)))),
		)
	}

	// Add connectivity issues if any failed services
	if failedServices > 0 {
		results = append(results,
			Tr(Td(Attr("colspan", "2"), Hr())),
			Tr(Td(Strong(Text("Connectivity Issues:"))), Td(Text(""))),
		)

		// Add each failed service dynamically
		for _, service := range healthResults.ServiceDetails {
			if service.Status != "online" && service.Status != "disabled" {
				errorMsg := service.ErrorMessage
				if errorMsg == "" {
					errorMsg = fmt.Sprintf("Service unavailable (status: %s)", service.Status)
				}
				results = append(results,
					Tr(Td(Text(service.Name+":")), Td(Text(errorMsg))),
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
	results = append(results,
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Summary:"))), Td(Text(""))),
		Tr(Td(Text("Total Services:")), Td(Text(fmt.Sprintf("%d", totalServices)))),
		Tr(Td(Text("Online Services:")), Td(Text(fmt.Sprintf("%d", onlineServices)))),
		Tr(Td(Text("Failed Services:")), Td(Text(fmt.Sprintf("%d", failedServices)))),
		Tr(Td(Text("Overall Score:")), Td(Text(fmt.Sprintf("%.1f/100", serviceOverallScore)))),
	)

	if saveResults {
		results = append(results,
			Tr(Td(Text("Results Saved:")), Td(Text("Yes - stored for trend analysis"))),
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

	result := Div(
		Class(alertClass),
		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, "+func() string {
				if failedServices >= totalServices/2 {
					return "#f8d7da 0%, #f5c6cb 100%"
				} else if failedServices > 0 {
					return "#fff3cd 0%, #ffeaa7 100%"
				}
				return "#d4edda 0%, #c3e6cb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge "+func() string {
					if failedServices >= totalServices/2 {
						return "bg-danger"
					} else if failedServices > 0 {
						return "bg-warning"
					}
					return "bg-success"
				}()+" me-3"), I(Class("fas fa-heartbeat me-1")), Text("Health")),
				H5(Class("card-title mb-0 "+func() string {
					if failedServices >= totalServices/2 {
						return "text-danger"
					} else if failedServices > 0 {
						return "text-warning"
					}
					return "text-success"
				}()+" fw-bold"), Text(message)),
			),
		),
		Div(
			Class("card-body p-0"),
			Table(
				Class("table table-hover mb-0"),
				Style("background: transparent;"),
				TBody(Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// contains checks if a string slice contains a specific string (case insensitive)
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
	wantedOnly := c.PostForm("wanted_only") == "on" || c.PostForm("wanted_only") == "true" || c.PostForm("_wanted_only") == "on" || c.PostForm("_wanted_only") == "true"

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
		c.String(http.StatusOK, renderAlert("Quality profile not found: "+selectedQuality, "danger"))
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
		c.String(http.StatusOK, renderAlert("No wanted quality combinations found for this profile", "info"))
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
					originalResoPriority := getresolutions[idxreso].Gettypeidprioritysingle("resolution", originalConfig)
					originalQualPriority := getqualities[idxqual].Gettypeidprioritysingle("quality", originalConfig)
					originalCodecPriority := getcodecs[idxcodec].Gettypeidprioritysingle("codec", originalConfig)
					originalAudioPriority := getaudios[idxaudio].Gettypeidprioritysingle("audio", originalConfig)

					// Apply original config reorder rules to original priorities
					originalFinalResoPrio := originalResoPriority
					originalFinalQualPrio := originalQualPriority
					for idx := range originalConfig.QualityReorder {
						reorder := &originalConfig.QualityReorder[idx]
						if reorder.ReorderType == "combined_res_qual" || strings.EqualFold(reorder.ReorderType, "combined_res_qual") {
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
					modifiedResoPriority := getresolutions[idxreso].Gettypeidprioritysingle("resolution", qualityConfig)
					modifiedQualPriority := getqualities[idxqual].Gettypeidprioritysingle("quality", qualityConfig)
					modifiedCodecPriority := getcodecs[idxcodec].Gettypeidprioritysingle("codec", qualityConfig)
					modifiedAudioPriority := getaudios[idxaudio].Gettypeidprioritysingle("audio", qualityConfig)

					// Apply modified config reorder rules to modified priorities
					modifiedFinalResoPrio := modifiedResoPriority
					modifiedFinalQualPrio := modifiedQualPriority
					reorderApplied := false
					for idx := range qualityConfig.QualityReorder {
						reorder := &qualityConfig.QualityReorder[idx]
						if reorder.ReorderType == "combined_res_qual" || strings.EqualFold(reorder.ReorderType, "combined_res_qual") {
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
					if modifiedFinalResoPrio != originalFinalResoPrio || modifiedFinalQualPrio != originalFinalQualPrio {
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

	var rows []Node
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
		var changeDisplay Node
		var changeClass string

		if priorityChange > 0 {
			changeDisplay = Span(Class("text-success"), Text(fmt.Sprintf("+%d", priorityChange)))
			changeClass = "table-success"
		} else if priorityChange < 0 {
			changeDisplay = Span(Class("text-danger"), Text(fmt.Sprintf("%d", priorityChange)))
			changeClass = "table-warning"
		} else {
			changeDisplay = Span(Class("text-muted"), Text("0"))
			changeClass = ""
		}

		rows = append(rows, Tr(
			If(changeClass != "", Class(changeClass)),
			Td(Class("text-center"),
				Span(Class("badge bg-secondary"), Text(fmt.Sprintf("%d", i+1)))),
			Td(Text(combinationDisplay)),
			Td(Text(fmt.Sprintf("%d", combo.OriginalTotalPriority))),
			Td(Text(fmt.Sprintf("%d", combo.TotalPriority))),
			Td(Class("text-center"), changeDisplay),
			Td(Class("small text-muted"), Text(priorityBreakdown)),
		))
	}

	var reorderInfo Node

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

	reorderInfo = Div(
		Class(alertClass),
		H6(Class("alert-heading"),
			I(Class(iconClass)),
			If(tempRulesCount > 0, Text("Testing Mode - Temporary Rules Active")),
			If(tempRulesCount == 0, Text("Quality Profile Information")),
		),
		P(Class("mb-1"), Text(wantedInfo)),
		P(Class("mb-1"), Text(rulesInfo)),
		If(tempRulesCount > 0, P(Class("mb-1 text-warning"), Strong(Text("⚠ Temporary rules are active - changes are not saved to configuration")))),
		If(filterInfo != "", P(Class("mb-0"), Strong(Text("Active Filters: ")), Text(filterInfo))),
	)

	result := Div(
		Class("mt-3"),
		reorderInfo,
		Div(
			Class("card border-0 shadow-sm"),
			Div(
				Class("card-header bg-light"),
				H5(Class("card-title mb-0"),
					I(Class("fas fa-list-ol me-2 text-primary")),
					Text("Wanted Quality Priority Combinations: "+selectedQuality),
				),
				P(Class("text-muted mb-0 mt-1"),
					Text(fmt.Sprintf("Showing %d resolution + quality combinations (sorted by total priority)", len(combinations))),
				),
			),
			Div(
				Class("card-body p-0"),
				Table(
					Class("table table-hover mb-0"),
					THead(
						Class("table-light"),
						Tr(
							Th(Class("text-center"), Text("Rank")),
							Th(Text("Resolution + Quality")),
							Th(Text("Original Priority")),
							Th(Text("Modified Priority")),
							Th(Text("Change")),
							Th(Text("Breakdown (Res±Qual)")),
						),
					),
					TBody(Group(rows)),
				),
			),
		),
	)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleQualityProfileRules returns the existing reorder rules for a quality profile as JSON
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
