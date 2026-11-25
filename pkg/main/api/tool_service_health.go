package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

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
					renderFormGroup("service",
						map[string]string{
							"CheckTVMaze": "Test TVmaze service connectivity",
						},
						map[string]string{
							"CheckTVMaze": "TVmaze API",
						},
						"CheckTVMaze", "checkbox", true, nil),
					renderFormGroup("service",
						map[string]string{
							"CheckMediaProviders": "Test configured book, audiobook and music providers (MusicBrainz, Last.fm, Discogs, Deezer, TheAudioDB, iTunes, AcoustID, OpenLibrary, Goodreads, Audnex, Audible)",
						},
						map[string]string{
							"CheckMediaProviders": "Book / Audiobook / Music APIs",
						},
						"CheckMediaProviders", "checkbox", true, nil),
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
	checkTVMaze := c.PostForm("service_CheckTVMaze") == "on" ||
		c.PostForm("service_CheckTVMaze") == "true"
	checkMediaProviders := c.PostForm("service_CheckMediaProviders") == "on" ||
		c.PostForm("service_CheckMediaProviders") == "true"
	timeoutStr := c.PostForm("service_Timeout")
	retriesStr := c.PostForm("service_Retries")
	detailedTest := c.PostForm("service_DetailedTest") == "on" ||
		c.PostForm("service_DetailedTest") == "true"
	measurePerformance := c.PostForm("service_MeasurePerformance") == "on" ||
		c.PostForm("service_MeasurePerformance") == "true"

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
		checkTVMaze,
		checkMediaProviders,
		timeout,
		retries,
		detailedTest,
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

		var successRate float64
		if healthResults.TotalServices > 0 {
			successRate = float64(healthResults.OnlineServices) /
				float64(healthResults.TotalServices) * 100
		}

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
