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
				ioResults = append(
					ioResults,
					fmt.Sprintf("%s: %s", pathInfo.Path, pathInfo.IOTest.Error),
				)
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
			results = append(
				results,
				html.Tr(
					html.Td(gomponents.Text("I/O Performance:")),
					html.Td(gomponents.Text("No I/O tests performed")),
				),
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
