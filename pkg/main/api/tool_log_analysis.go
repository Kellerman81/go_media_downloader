package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

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
						"MaxLines": "Maximum number of log entries to analyze (the newest are kept)",
					}, map[string]string{
						"MaxLines": "Max Lines",
					}, "MaxLines", "number", "10000", nil),

					renderFormGroup("analysis", map[string]string{
						"LogFile": "downloader: all levels, rotates every few hours; errors: errors only, kept for days",
					}, map[string]string{
						"LogFile": "Log File",
					}, "LogFile", "select", "downloader", map[string][]string{
						"options": {"downloader", "errors"},
					}),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Analysis Types")),

					renderFormGroup("analysis",
						map[string]string{
							"ErrorPattern": "Most frequent warnings and errors, grouped by message",
						},
						map[string]string{
							"ErrorPattern": "Error Pattern Analysis",
						},
						"ErrorPattern", "checkbox", true, nil),

					renderFormGroup("analysis",
						map[string]string{
							"PerformanceMetrics": "Web request response times and slowest endpoints (from weblogs.log)",
						},
						map[string]string{
							"PerformanceMetrics": "Performance Metrics",
						},
						"PerformanceMetrics", "checkbox", true, nil),

					renderFormGroup("analysis",
						map[string]string{
							"AccessPattern": "Most requested endpoints and HTTP error responses (from weblogs.log)",
						},
						map[string]string{
							"AccessPattern": "Access Patterns",
						},
						"AccessPattern", "checkbox", false, nil),

					renderFormGroup("analysis",
						map[string]string{
							"SystemHealth": "Current queued/running jobs and open database connections",
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

	logName := appLogName
	if c.PostForm("analysis_LogFile") == errorLogName {
		logName = errorLogName
	}

	maxLines, err := strconv.ParseInt(maxLinesStr, 10, 64)
	if err != nil || maxLines <= 0 {
		maxLines = 10000
	}

	startTime := time.Now()

	logDir, err := getLogDir()
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to locate log files: "+err.Error(), "danger"))
		return
	}

	analysisResults, err := performLogAnalysis(logDir, logName, timeRange, logLevel, maxLines,
		errorPattern, performanceMetrics, accessPattern, startTime)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to analyze logs: "+err.Error(), "danger"))
		return
	}

	errorCount := analysisResults.ErrorCount
	analysisDuration := time.Since(startTime)

	row := func(label, value string) gomponents.Node {
		return html.Tr(html.Td(gomponents.Text(label)), html.Td(gomponents.Text(value)))
	}
	heading := func(label string) gomponents.Node {
		return html.Tr(html.Td(html.Strong(gomponents.Text(label))), html.Td(gomponents.Text("")))
	}
	separator := html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr()))
	itemList := func(items []CountedItem, format func(*CountedItem) string) gomponents.Node {
		lis := make([]gomponents.Node, 0, len(items))
		for i := range items {
			lis = append(lis, html.Li(gomponents.Text(format(&items[i]))))
		}

		return html.Ol(html.Class("mb-0 ps-3 small"), gomponents.Group(lis))
	}
	fileNames := func(files []string) string {
		names := make([]string, 0, len(files))
		for _, f := range files {
			names = append(names, filepath.Base(f))
		}

		return strings.Join(names, ", ")
	}
	timeLabel := func(t time.Time) string {
		if t.IsZero() {
			return "-"
		}

		return t.Local().Format("2006-01-02 15:04:05")
	}

	covered := timeLabel(analysisResults.First) + " to " + timeLabel(analysisResults.Last)
	if analysisResults.Truncated {
		covered += fmt.Sprintf(" (newest %d entries; raise Max Lines to look further back)", maxLines)
	}

	results := []gomponents.Node{
		heading("Analysis Configuration:"),
		row("Log File:", logName+".log"),
		row("Time Range:", timeRange),
		row("Minimum Log Level:", logLevel),
		row("Max Lines:", strconv.FormatInt(maxLines, 10)),
		row("Files Read:", fileNames(analysisResults.Files)),
		row("Analysis Duration:", analysisDuration.Round(time.Millisecond).String()),
		separator,
		heading("Analysis Results:"),
		row("Entries Analyzed:", strconv.FormatInt(analysisResults.TotalEntries, 10)),
		row("Period Covered:", covered),
		row("Errors:", strconv.FormatInt(analysisResults.ErrorCount, 10)),
		row("Warnings:", strconv.FormatInt(analysisResults.WarningCount, 10)),
		row("Info:", strconv.FormatInt(analysisResults.InfoCount, 10)),
		row("Debug:", strconv.FormatInt(analysisResults.DebugCount, 10)),
	}

	if analysisResults.Unparsed > 0 {
		results = append(results, row("Unreadable Lines:", strconv.FormatInt(analysisResults.Unparsed, 10)))
	}

	if errorPattern {
		var top gomponents.Node = gomponents.Text("No warnings or errors")
		if len(analysisResults.TopErrors) > 0 {
			top = itemList(analysisResults.TopErrors, func(it *CountedItem) string {
				return fmt.Sprintf("%d × %s", it.Count, it.Name)
			})
		}

		results = append(results,
			separator,
			row("Distinct Warnings/Errors:", strconv.Itoa(analysisResults.DistinctErrors)),
			html.Tr(html.Td(gomponents.Text("Most Frequent:")), html.Td(top)),
		)
	}

	if (performanceMetrics || accessPattern) && analysisResults.Requests == 0 {
		results = append(results, separator, row("Web Requests:", "No requests in weblogs.log for this time range"))
	} else if performanceMetrics || accessPattern {
		results = append(results,
			separator,
			row("Web Log Files:", fileNames(analysisResults.WebFiles)),
			row("Web Requests:", strconv.FormatInt(analysisResults.Requests, 10)),
		)
	}

	if performanceMetrics && analysisResults.Requests > 0 {
		results = append(results,
			row("Avg Response Time:", fmt.Sprintf("%.1f ms", analysisResults.AvgResponseMs)),
			row("Max Response Time:", fmt.Sprintf("%.1f ms (%s)", analysisResults.MaxResponseMs, analysisResults.SlowestRequest)),
			row(fmt.Sprintf("Requests ≥ %d ms:", slowRequestMs), strconv.FormatInt(analysisResults.SlowRequests, 10)),
			html.Tr(html.Td(gomponents.Text("Slowest Endpoints (avg):")), html.Td(itemList(analysisResults.SlowestEndpoints,
				func(it *CountedItem) string {
					return fmt.Sprintf("%s: %.0f ms avg, %.0f ms max, %d requests", it.Name, it.AvgMs, it.MaxMs, it.Count)
				}))),
		)
	}

	if accessPattern && analysisResults.Requests > 0 {
		results = append(results,
			row("Client Errors (4xx):", strconv.FormatInt(analysisResults.ClientErrors, 10)),
			row("Server Errors (5xx):", strconv.FormatInt(analysisResults.ServerErrors, 10)),
			html.Tr(html.Td(gomponents.Text("Most Requested:")), html.Td(itemList(analysisResults.TopEndpoints,
				func(it *CountedItem) string { return fmt.Sprintf("%d × %s", it.Count, it.Name) }))),
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
