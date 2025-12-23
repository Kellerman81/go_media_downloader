package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// ================================================================================
// JOB MANAGEMENT PAGE
// ================================================================================

// renderJobManagementPage renders a page for starting jobs.
func renderJobManagementPage(csrfToken string) gomponents.Node {
	// Get available media configurations
	media := config.GetSettingsMediaAll()

	var mediaConfigs []string
	for i := range media.Movies {
		mediaConfigs = append(mediaConfigs, media.Movies[i].NamePrefix)
	}

	for i := range media.Series {
		mediaConfigs = append(mediaConfigs, media.Series[i].NamePrefix)
	}

	jobTypes := []string{
		"data", "datafull", "feeds", "rss", "rssseasons", "rssseasonsall",
		"checkmissing", "checkmissingflag", "checkupgradeflag", "checkreachedflag",
		"clearhistory", "searchmissinginc", "searchmissingfull", "searchmissinginctitle",
		"searchmissingfulltitle", "searchupgradeinc", "searchupgradefull",
		"searchupgradeinctitle", "searchupgradefulltitle",
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
					html.I(html.Class("fa-solid fa-tasks header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Job Management")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Start single jobs for media processing, data refreshing, and searching. Jobs will be queued and executed based on your configuration settings.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("jobForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Job Configuration")),

					renderFormGroup("job", map[string]string{
						"JobType": "Select the type of job to run",
					}, map[string]string{
						"JobType": "Job Type",
					}, "JobType", "select", "data", map[string][]string{
						"options": jobTypes,
					}),

					renderFormGroup("job", map[string]string{
						"MediaConfig": "Media configuration to use for the job",
					}, map[string]string{
						"MediaConfig": "Media Configuration",
					}, "MediaConfig", "select", "", map[string][]string{
						"options": mediaConfigs,
					}),
				),

				html.Div(
					html.Class("col-md-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Job Options")),

					renderFormGroup("job", map[string]string{
						"ListName": "Optional: Specific list name to process",
					}, map[string]string{
						"ListName": "List Name",
					}, "ListName", "text", "", nil),

					renderFormGroup("job", map[string]string{
						"Force": "Force execution even if scheduler is disabled",
					}, map[string]string{
						"Force": "Force Execution",
					}, "Force", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Job Descriptions")),
					html.Details(
						html.Summary(gomponents.Text("Job Type Descriptions")),
						html.Ul(
							html.Li(
								html.Strong(gomponents.Text("data: ")),
								gomponents.Text("Scan and import new media files"),
							),
							html.Li(
								html.Strong(gomponents.Text("datafull: ")),
								gomponents.Text("Full rescan of all media files"),
							),
							html.Li(
								html.Strong(gomponents.Text("feeds: ")),
								gomponents.Text("Refresh RSS feeds and lists"),
							),
							html.Li(
								html.Strong(gomponents.Text("rss: ")),
								gomponents.Text("Process RSS downloads"),
							),
							html.Li(
								html.Strong(gomponents.Text("rssseasons/rssseasonsall: ")),
								gomponents.Text("Season-specific RSS processing"),
							),
							html.Li(
								html.Strong(gomponents.Text("checkmissing: ")),
								gomponents.Text("Check for missing episodes/movies"),
							),
							html.Li(
								html.Strong(gomponents.Text("search*: ")),
								gomponents.Text(
									"Various search operations for missing/upgrade content",
								),
							),
							html.Li(
								html.Strong(gomponents.Text("clearhistory: ")),
								gomponents.Text("Clear download history"),
							),
						),
					),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Start Job"),
					html.Type("button"),
					hx.Target("#jobResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/jobmanagement"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#jobForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('jobForm').reset(); document.getElementById('jobResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("jobResults"),
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
						html.I(html.Class("fas fa-info-circle me-1")),
						gomponents.Text("Usage"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Usage Instructions"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Follow these steps to execute management jobs"),
				),
				html.Ol(
					html.Class("mb-0 list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("1. Select the job type you want to execute"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"2. Choose the media configuration (required for most jobs)",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"3. Optionally specify a list name for targeted processing",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"4. Check 'Force Execution' to run even if scheduler is disabled",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("5. Click 'Start Job' to queue the job for execution"),
					),
				),
			),
		),

		html.Div(
			html.Class("card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-lightbulb me-1")),
						gomponents.Text("Note"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Important Information"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-0"),
					gomponents.Text(
						"Jobs are queued and executed asynchronously. Check the scheduler page for job status and history.",
					),
				),
			),
		),
	)
}

// HandleJobManagement handles job start requests.
func HandleJobManagement(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	jobType := c.PostForm("job_JobType")
	mediaConfig := c.PostForm("job_MediaConfig")
	listName := c.PostForm("job_ListName")
	force, _ := strconv.ParseBool(c.PostForm("job_Force"))

	if jobType == "" {
		c.String(http.StatusOK, renderAlert("Please select a job type", "warning"))
		return
	}

	if mediaConfig == "" {
		c.String(http.StatusOK, renderAlert("Please select a media configuration", "warning"))
		return
	}

	// Start the job using worker.Dispatch
	err := worker.Dispatch(jobType+"_"+mediaConfig, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, jobType, mediaConfig, listName, force, key)
	}, "Data")
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to start job: "+err.Error(), "danger"))
		return
	}

	result := html.Div(
		html.Class("card border-0 shadow-sm border-success mb-4"),

		html.Div(
			html.Class("card-header border-0"),
			html.Style(
				"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
			),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(
					html.Class("badge bg-success me-3"),
					html.I(html.Class("fas fa-play-circle me-1")),
					gomponents.Text("Started"),
				),
				html.H5(
					html.Class("card-title mb-0 text-success fw-bold"),
					gomponents.Text("Job Started Successfully"),
				),
			),
		),

		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3"),
					html.P(
						html.Class("mb-3"),
						html.Style("color: #495057;"),
						gomponents.Text(
							fmt.Sprintf(
								"Job '%s' has been queued for execution with the following parameters:",
								jobType,
							),
						),
					),

					html.Div(
						html.Class("row g-2 mb-3"),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-cog me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Job Type"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(jobType),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-film me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Media Config"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(mediaConfig),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-list me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("List Name"),
											),
											html.Div(
												html.Class("fw-semibold"),
												func() gomponents.Node {
													if listName != "" {
														return gomponents.Text(listName)
													}
													return gomponents.Text("(all lists)")
												}(),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-bolt me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Force Execution"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(fmt.Sprintf("%t", force)),
											),
										),
									),
								),
							),
						),
					),
				),
			),

			html.Div(
				html.Class("card border-0 mb-0"),
				html.Style("background-color: rgba(40, 167, 69, 0.1); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3"),
					html.Div(
						html.Class("d-flex align-items-start"),
						html.I(
							html.Class("fas fa-info-circle me-2 mt-1"),
							html.Style("color: #28a745; font-size: 0.9rem;"),
						),
						html.Small(
							html.Style("color: #495057; line-height: 1.4;"),
							html.Strong(gomponents.Text("Status: ")),
							gomponents.Text(
								"The job is now running in the background. You can monitor its progress in the scheduler interface.",
							),
						),
					),
				),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// ================================================================================
// DEBUG STATS PAGE
// ================================================================================

// renderDebugStatsPage renders a page for viewing debug statistics.
func renderDebugStatsPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-bug header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Debug Statistics")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"View runtime statistics, memory usage, garbage collection info, and worker statistics. This information is useful for monitoring application performance and troubleshooting issues.",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("form-group"),
			html.Button(
				html.Class("btn btn-primary"),
				gomponents.Text("Refresh Debug Stats"),
				html.Type("button"),
				hx.Target("#debugResults"),
				hx.Swap("innerHTML"),
				hx.Post("/api/admin/debugstats"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			),
			html.Button(
				html.Class("btn btn-warning ml-2"),
				gomponents.Text("Force Garbage Collection"),
				html.Type("button"),
				hx.Target("#debugResults"),
				hx.Swap("innerHTML"),
				hx.Post("/api/admin/debugstats"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				hx.Vals("{\"action\": \"gc\"}"),
			),
		),

		html.Div(
			html.ID("debugResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-info"),
			html.Div(
				html.Class("card-body"),
				html.H5(
					html.Class("card-title fw-bold mb-3"),
					gomponents.Text("Debug Information"),
				),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Available debug operations and system information"),
				),
				html.Ul(html.Class("list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-primary me-2"),
							html.I(html.Class("fas fa-server me-1")),
							gomponents.Text("Runtime"),
						),
						gomponents.Text(
							"Go runtime information including OS, CPU count, and goroutine count",
						),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-info me-2"),
							html.I(html.Class("fas fa-memory me-1")),
							gomponents.Text("Memory"),
						),
						gomponents.Text(
							"Detailed memory usage including heap, stack, and GC statistics",
						),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-success me-2"),
							html.I(html.Class("fas fa-recycle me-1")),
							gomponents.Text("GC Stats"),
						),
						gomponents.Text("Garbage collection performance metrics and timing"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-warning me-2"),
							html.I(html.Class("fas fa-tasks me-1")),
							gomponents.Text("Workers"),
						),
						gomponents.Text("Background job queue and worker pool statistics"),
					),
					html.Li(
						html.Class("mb-0"),
						html.Span(
							html.Class("badge bg-danger me-2"),
							html.I(html.Class("fas fa-download me-1")),
							gomponents.Text("Heap Dump"),
						),
						gomponents.Text("Memory heap dump is saved to temp/heapdump for analysis"),
					),
				),
			),
		),
	)
}

// HandleDebugStats handles debug statistics requests.
func HandleDebugStats(c *gin.Context) {
	action := c.PostForm("action")

	// Collect debug information
	var gc debug.GCStats
	debug.ReadGCStats(&gc)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	if action == "gc" {
		debug.FreeOSMemory()
		runtime.GC()
	}

	// Create heap dump
	err := os.MkdirAll("./temp", 0o755)
	if err == nil {
		if f, err := os.Create("./temp/heapdump"); err == nil {
			debug.WriteHeapDump(f.Fd())
			f.Close()
		}
	}

	workerStats := worker.GetStats()

	result := html.Div(
		html.Class("card border-0 shadow-sm border-info mb-4"),
		html.Div(
			html.Class("card-header border-0"),
			html.Style(
				"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
			),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(
					html.Class("badge bg-info me-3"),
					html.I(html.Class("fas fa-chart-line me-1")),
					gomponents.Text("Statistics"),
				),
				html.H5(
					html.Class("card-title mb-0 text-info fw-bold"),
					gomponents.Text("Debug Statistics"),
				),
			),
		),
		html.Div(
			html.Class("card-body"),
			html.P(
				html.Class("card-text text-muted mb-3"),
				gomponents.Text(
					fmt.Sprintf("Generated at: %s", time.Now().Format("2006-01-02 15:04:05")),
				),
			),

			// Runtime Information
			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-header bg-transparent border-0 pb-0"),
					html.Style("background: transparent !important;"),
					html.H6(
						html.Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"),
						gomponents.Text("Runtime Information"),
					),
				),
				html.Div(
					html.Class("card-body pt-2"),
					html.Table(
						html.Class("table table-hover table-sm mb-0"),
						html.Style("background: transparent;"),
						html.TBody(
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Operating System:"))),
								html.Td(gomponents.Text(runtime.GOOS)),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Architecture:"))),
								html.Td(gomponents.Text(runtime.GOARCH)),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("CPU Count:"))),
								html.Td(gomponents.Text(fmt.Sprintf("%d", runtime.NumCPU()))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Goroutines:"))),
								html.Td(gomponents.Text(fmt.Sprintf("%d", runtime.NumGoroutine()))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Go Version:"))),
								html.Td(gomponents.Text(runtime.Version())),
							),
						),
					),
				),
			),

			// Memory Statistics
			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-header bg-transparent border-0 pb-0"),
					html.Style("background: transparent !important;"),
					html.H6(
						html.Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"),
						gomponents.Text("Memory Statistics"),
					),
				),
				html.Div(
					html.Class("card-body pt-2"),
					html.Table(
						html.Class("table table-hover table-sm mb-0"),
						html.Style("background: transparent;"),
						html.TBody(
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Allocated Memory:"))),
								html.Td(gomponents.Text(formatBytes(mem.Alloc))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Total Allocated:"))),
								html.Td(gomponents.Text(formatBytes(mem.TotalAlloc))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("System Memory:"))),
								html.Td(gomponents.Text(formatBytes(mem.Sys))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Lookups:"))),
								html.Td(gomponents.Text(fmt.Sprintf("%d", mem.Lookups))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Malloc Count:"))),
								html.Td(gomponents.Text(fmt.Sprintf("%d", mem.Mallocs))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Free Count:"))),
								html.Td(gomponents.Text(fmt.Sprintf("%d", mem.Frees))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Heap Allocated:"))),
								html.Td(gomponents.Text(formatBytes(mem.HeapAlloc))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Heap System:"))),
								html.Td(gomponents.Text(formatBytes(mem.HeapSys))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Heap Objects:"))),
								html.Td(gomponents.Text(fmt.Sprintf("%d", mem.HeapObjects))),
							),
						),
					),
				),
			),

			// Garbage Collection Statistics
			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-header bg-transparent border-0 pb-0"),
					html.Style("background: transparent !important;"),
					html.H6(
						html.Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"),
						gomponents.Text("Garbage Collection Statistics"),
					),
				),
				html.Div(
					html.Class("card-body pt-2"),
					html.Table(
						html.Class("table table-hover table-sm mb-0"),
						html.Style("background: transparent;"),
						html.TBody(
							html.Tr(
								html.Td(html.Strong(gomponents.Text("GC Cycles:"))),
								html.Td(gomponents.Text(fmt.Sprintf("%d", gc.NumGC))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Last GC:"))),
								html.Td(gomponents.Text(gc.LastGC.Format("2006-01-02 15:04:05"))),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Total Pause:"))),
								html.Td(gomponents.Text(gc.PauseTotal.String())),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Average Pause:"))),
								html.Td(gomponents.Text(func() string {
									if gc.NumGC > 0 {
										return (gc.PauseTotal / time.Duration(gc.NumGC)).String()
									}
									return "0"
								}())),
							),
						),
					),
				),
			),

			// Worker Statistics
			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-header bg-transparent border-0 pb-0"),
					html.Style("background: transparent !important;"),
					html.H6(
						html.Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"),
						gomponents.Text("Worker Statistics"),
					),
				),
				html.Div(
					html.Class("card-body pt-2"),
					html.Table(
						html.Class("table table-hover table-sm mb-0"),
						html.Style("background: transparent;"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Worker Pool")),
								html.Th(gomponents.Text("Submitted")),
								html.Th(gomponents.Text("Completed")),
								html.Th(gomponents.Text("Successful")),
								html.Th(gomponents.Text("Failed")),
								html.Th(gomponents.Text("Dropped")),
								html.Th(gomponents.Text("Waiting")),
								html.Th(gomponents.Text("Running")),
							),
						),
						html.TBody(
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Parse"))),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerParse.SubmittedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerParse.CompletedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerParse.SuccessfulTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerParse.FailedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerParse.DroppedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerParse.WaitingTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerParse.RunningWorkers),
									),
								),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Search"))),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerSearch.SubmittedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerSearch.CompletedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerSearch.SuccessfulTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerSearch.FailedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerSearch.DroppedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerSearch.WaitingTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerSearch.RunningWorkers),
									),
								),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("RSS"))),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerRSS.SubmittedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerRSS.CompletedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerRSS.SuccessfulTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerRSS.FailedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerRSS.DroppedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerRSS.WaitingTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerRSS.RunningWorkers),
									),
								),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Files"))),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerFiles.SubmittedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerFiles.CompletedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerFiles.SuccessfulTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerFiles.FailedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerFiles.DroppedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerFiles.WaitingTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerFiles.RunningWorkers),
									),
								),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Meta"))),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerMeta.SubmittedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerMeta.CompletedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerMeta.SuccessfulTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerMeta.FailedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerMeta.DroppedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerMeta.WaitingTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerMeta.RunningWorkers),
									),
								),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Index"))),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndex.SubmittedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndex.CompletedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndex.SuccessfulTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndex.FailedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndex.DroppedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndex.WaitingTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndex.RunningWorkers),
									),
								),
							),
							html.Tr(
								html.Td(html.Strong(gomponents.Text("Index RSS"))),
								html.Td(
									gomponents.Text(
										fmt.Sprintf(
											"%d",
											workerStats.WorkerIndexRSS.SubmittedTasks,
										),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf(
											"%d",
											workerStats.WorkerIndexRSS.CompletedTasks,
										),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf(
											"%d",
											workerStats.WorkerIndexRSS.SuccessfulTasks,
										),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndexRSS.FailedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndexRSS.DroppedTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf("%d", workerStats.WorkerIndexRSS.WaitingTasks),
									),
								),
								html.Td(
									gomponents.Text(
										fmt.Sprintf(
											"%d",
											workerStats.WorkerIndexRSS.RunningWorkers,
										),
									),
								),
							),
						),
					),
				),
			),

			func() gomponents.Node {
				if action == "gc" {
					return html.Div(
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
									gomponents.Text("Success"),
								),
								html.H5(
									html.Class("card-title mb-0 text-success fw-bold"),
									gomponents.Text("Garbage Collection Complete"),
								),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.P(
								html.Class("card-text text-muted mb-0"),
								gomponents.Text("Garbage collection completed and memory freed."),
							),
						),
					)
				}

				return gomponents.Text("")
			}(),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// formatBytes formats byte count as human readable string.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ================================================================================
// DATABASE MAINTENANCE PAGE
// ================================================================================

// renderDatabaseMaintenancePage renders a page for database maintenance operations.
func renderDatabaseMaintenancePage(csrfToken string) gomponents.Node {
	// Get available table names for clear operations
	tableNames := []string{
		"movies", "dbmovies", "dbmovie_titles",
		"series", "dbseries", "dbserie_episodes", "dbserie_alternates",
		"movie_files", "movie_histories", "movie_file_unmatcheds",
		"serie_episodes", "serie_episode_files", "serie_episode_histories", "serie_file_unmatcheds",
		"qualities", "job_histories", "r_sshistories", "indexer_fails",
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
					html.I(html.Class("fa-solid fa-tools header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Database Maintenance")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Perform various database maintenance operations including integrity checks, backups, cleanup, and optimization tasks.",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-md-6"),
				html.H5(
					html.Class("form-section-title"),
					gomponents.Text("Maintenance Operations"),
				),

				html.Div(
					html.Class("btn-group-vertical d-block"),
					html.Button(
						html.Class("btn btn-info mb-2"),
						gomponents.Text("Check Database Integrity"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"integrity\"}"),
					),
					html.Button(
						html.Class("btn btn-success mb-2"),
						gomponents.Text("Create Database Backup"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"backup\"}"),
					),
					html.Button(
						html.Class("btn btn-primary mb-2"),
						gomponents.Text("Vacuum Database"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"vacuum\"}"),
					),
					html.Button(
						html.Class("btn btn-info mb-2"),
						gomponents.Text("Fill IMDB Data"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"fillimdb\"}"),
					),
				),

				html.H6(gomponents.Text("Clear Cache & Jobs")),
				html.Div(
					html.Class("btn-group-vertical d-block"),
					html.Button(
						html.Class("btn btn-warning mb-2"),
						gomponents.Text("Clear Cache"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"clearcache\"}"),
					),
					html.Details(
						html.Summary(gomponents.Text("Remove Old Jobs (with days parameter)")),
						html.Form(
							html.Class("mt-2"),
							html.ID("oldJobsForm"),
							html.Div(
								html.Class("form-group"),
								html.Label(gomponents.Text("Days to keep:")),
								html.Input(
									html.Type("number"),
									html.Class("form-control"),
									html.Name("days"),
									html.Value("30"),
									html.Min("1"),
									html.Max("365"),
								),
							),
							html.Button(
								html.Class("btn btn-warning"),
								gomponents.Text("Remove Old Jobs"),
								html.Type("button"),
								hx.Target("#maintenanceResults"),
								hx.Swap("innerHTML"),
								hx.Post("/api/admin/dbmaintenance"),
								hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
								hx.Include("#oldJobsForm"),
								hx.Vals("{\"action\": \"removeoldjobs\"}"),
							),
						),
					),
				),
			),

			html.Div(
				html.Class("col-md-6"),
				html.H5(html.Class("form-section-title"), gomponents.Text("Table Operations")),

				html.Details(
					html.Summary(gomponents.Text("Clear Specific Table")),
					html.Form(
						html.Class("mt-2"),
						html.ID("clearTableForm"),
						html.Div(
							html.Class("form-group"),
							html.Label(gomponents.Text("Select table to clear:")),
							html.Select(
								html.Class("form-control"),
								html.Name("tableName"),
								gomponents.Group(func() []gomponents.Node {
									var options []gomponents.Node
									for _, table := range tableNames {
										options = append(
											options,
											html.Option(html.Value(table), gomponents.Text(table)),
										)
									}

									return options
								}()),
							),
						),
						html.Button(
							html.Class("btn btn-danger"),
							gomponents.Text("Clear Table"),
							html.Type("button"),
							hx.Target("#maintenanceResults"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/dbmaintenance"),
							hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
							hx.Include("#clearTableForm"),
							hx.Vals("{\"action\": \"clear\"}"),
							gomponents.Attr(
								"onclick",
								"return confirm('Are you sure? This will delete all data from the selected table!')",
							),
						),
					),
				),

				html.Details(
					html.Summary(gomponents.Text("Delete Specific Record")),
					html.Form(
						html.Class("mt-2"),
						html.ID("deleteRecordForm"),
						html.Div(
							html.Class("form-group"),
							html.Label(gomponents.Text("Table name:")),
							html.Select(
								html.Class("form-control"),
								html.Name("tableName"),
								gomponents.Group(func() []gomponents.Node {
									var options []gomponents.Node
									for _, table := range tableNames {
										options = append(
											options,
											html.Option(html.Value(table), gomponents.Text(table)),
										)
									}

									return options
								}()),
							),
						),
						html.Div(
							html.Class("form-group mt-2"),
							html.Label(gomponents.Text("Record ID:")),
							html.Input(
								html.Type("number"),
								html.Class("form-control"),
								html.Name("recordID"),
								html.Min("1"),
								html.Required(),
							),
						),
						html.Button(
							html.Class("btn btn-danger"),
							gomponents.Text("Delete Record"),
							html.Type("button"),
							hx.Target("#maintenanceResults"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/dbmaintenance"),
							hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
							hx.Include("#deleteRecordForm"),
							hx.Vals("{\"action\": \"delete\"}"),
							gomponents.Attr(
								"onclick",
								"return confirm('Are you sure? This will permanently delete the selected record!')",
							),
						),
					),
				),
			),
		),

		html.Div(
			html.ID("maintenanceResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-info"),
			html.Div(
				html.Class("card-body"),
				html.H5(
					html.Class("card-title fw-bold mb-3"),
					gomponents.Text("Operation Descriptions"),
				),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Database maintenance and management operations"),
				),
				html.Ul(html.Class("list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-success me-2"),
							html.I(html.Class("fas fa-check me-1")),
							gomponents.Text("Integrity"),
						),
						gomponents.Text("Verifies database consistency and reports any corruption"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-primary me-2"),
							html.I(html.Class("fas fa-save me-1")),
							gomponents.Text("Backup"),
						),
						gomponents.Text("Creates a backup of the current database"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-info me-2"),
							html.I(html.Class("fas fa-compress me-1")),
							gomponents.Text("Vacuum"),
						),
						gomponents.Text("Optimizes database by reclaiming space and defragmenting"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-secondary me-2"),
							html.I(html.Class("fas fa-film me-1")),
							gomponents.Text("Fill IMDB"),
						),
						gomponents.Text("Populates IMDB data for movies and series"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-warning me-2"),
							html.I(html.Class("fas fa-broom me-1")),
							gomponents.Text("Clear Cache"),
						),
						gomponents.Text("Removes cached data to free memory"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-dark me-2"),
							html.I(html.Class("fas fa-history me-1")),
							gomponents.Text("Remove Jobs"),
						),
						gomponents.Text("Cleans up job history older than specified days"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-danger me-2"),
							html.I(html.Class("fas fa-exclamation-triangle me-1")),
							gomponents.Text("Clear Table"),
						),
						gomponents.Text("⚠️ Removes all data from the selected table"),
					),
					html.Li(
						html.Class("mb-0"),
						html.Span(
							html.Class("badge bg-danger me-2"),
							html.I(html.Class("fas fa-trash me-1")),
							gomponents.Text("Delete Record"),
						),
						gomponents.Text("⚠️ Removes a specific record by ID"),
					),
				),
			),
		),
	)
}

// HandleDatabaseMaintenance handles database maintenance requests.
func HandleDatabaseMaintenance(c *gin.Context) {
	action := c.PostForm("action")
	if action == "" {
		// Try to get from JSON body
		var reqData map[string]string
		if err := c.ShouldBindJSON(&reqData); err == nil {
			action = reqData["action"]
		}
	}

	var (
		result              gomponents.Node
		message, alertClass string
	)

	switch action {
	case "integrity":
		message, alertClass = performIntegrityCheck(c)
	case "backup":
		message, alertClass = performBackup(c)
	case "vacuum":
		message, alertClass = performVacuum(c)
	case "fillimdb":
		message, alertClass = performFillIMDB(c)
	case "clearcache":
		message, alertClass = performClearCache(c)
	case "removeoldjobs":
		message, alertClass = performRemoveOldJobs(c)
	case "clear":
		message, alertClass = performClearTable(c)
	case "delete":
		message, alertClass = performDeleteRecord(c)
	default:
		message = "Invalid action specified"
		alertClass = "danger"
	}

	result = html.Div(
		html.Class("card border-0 shadow-sm border-"+alertClass),
		html.Div(
			html.Class("card-body"),
			html.H5(
				html.Class("card-title fw-bold mb-3"),
				gomponents.Text("Maintenance Operation Result"),
			),
			html.Div(html.Class("mb-3"),
				html.P(html.Class("card-text"), gomponents.Text(message)),
			),
			html.Div(
				html.Class("d-flex align-items-center text-muted"),
				html.I(html.Class("fas fa-clock me-2")),
				html.Small(
					gomponents.Text(
						"Operation completed at: "+time.Now().Format("2006-01-02 15:04:05"),
					),
				),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// Database maintenance helper functions that call actual API functions.
func performIntegrityCheck(_ *gin.Context) (string, string) {
	// Call the actual integrity check function
	results := database.DBIntegrityCheck()
	if len(results) == 0 || results == "ok" {
		return "✅ Database integrity check completed. No issues found.", "success"
	}

	return fmt.Sprintf("⚠️ Database integrity issues found: %s", results), "warning"
}

func performBackup(_ *gin.Context) (string, string) {
	// Call the actual backup function (same logic as apiDBBackup)
	if config.GetSettingsGeneral().DatabaseBackupStopTasks {
		worker.StopCronWorker()
		worker.CloseWorkerPools()

		defer func() {
			worker.InitWorkerPools(
				config.GetSettingsGeneral().WorkerSearch,
				config.GetSettingsGeneral().WorkerFiles,
				config.GetSettingsGeneral().WorkerMetadata,
				config.GetSettingsGeneral().WorkerRSS,
				config.GetSettingsGeneral().WorkerIndexer,
			)
			worker.StartCronWorker()
		}()
	}

	backupto := "./backup/data.db." + database.GetVersion() + "." + time.Now().
		Format("20060102_150405")

	err := database.Backup(&backupto, config.GetSettingsGeneral().MaxDatabaseBackups)
	if err != nil {
		return fmt.Sprintf("❌ Database backup failed: %s", err.Error()), "danger"
	}

	return fmt.Sprintf("✅ Database backup created successfully at: %s", backupto), "success"
}

func performVacuum(_ *gin.Context) (string, string) {
	// Call the actual vacuum function
	err := database.ExecNErr("VACUUM")
	if err != nil {
		return fmt.Sprintf("❌ Database vacuum failed: %s", err.Error()), "danger"
	}

	return "✅ Database vacuum completed. Storage optimized.", "success"
}

func performFillIMDB(c *gin.Context) (string, string) {
	// Call the actual IMDB fill function
	config.GetSettingsGeneral().Jobs["RefreshImdb"](0, c)
	return "✅ IMDB data population started in background.", "info"
}

func performClearCache(_ *gin.Context) (string, string) {
	// Call the actual cache clear function
	database.ClearCaches()
	return "✅ Cache cleared successfully.", "success"
}

func performRemoveOldJobs(c *gin.Context) (string, string) {
	// Get the days parameter
	days := c.PostForm("days")
	if days == "" {
		days = "30" // default to 30 days
	}

	daysInt, err := strconv.Atoi(days)
	if err != nil || daysInt < 1 {
		return "❌ Invalid days parameter. Must be a positive number.", "danger"
	}

	// Call the actual remove old jobs function (same logic as apiDBRemoveOldJobs)
	scantime := time.Now().AddDate(0, 0, 0-daysInt)

	_, err = database.DeleteRow("job_histories", "created_at < ?", scantime)
	if err != nil {
		return fmt.Sprintf("❌ Failed to remove old jobs: %s", err.Error()), "danger"
	}

	return fmt.Sprintf(
		"✅ Old job entries older than %d days removed successfully.",
		daysInt,
	), "success"
}

func performClearTable(c *gin.Context) (string, string) {
	tableName := c.PostForm("tableName")
	if tableName == "" {
		return "❌ No table name specified.", "danger"
	}

	// Call the actual clear table function (same as apiDBClear)
	err := database.ExecNErr("DELETE FROM " + tableName)
	if err != nil {
		return fmt.Sprintf("❌ Failed to clear table '%s': %s", tableName, err.Error()), "danger"
	}

	// Run vacuum after clearing
	database.ExecN("VACUUM")

	return fmt.Sprintf("✅ Table '%s' cleared successfully.", tableName), "success"
}

func performDeleteRecord(c *gin.Context) (string, string) {
	tableName := c.PostForm("tableName")
	recordID := c.PostForm("recordID")

	if tableName == "" || recordID == "" {
		return "❌ Table name and record ID are required.", "danger"
	}

	// Call the actual delete record function (same as apiDBDelete)
	err := database.ExecNErr("DELETE FROM "+tableName+" WHERE id = ?", recordID)
	if err != nil {
		return fmt.Sprintf(
			"❌ Failed to delete record %s from table '%s': %s",
			recordID,
			tableName,
			err.Error(),
		), "danger"
	}

	return fmt.Sprintf(
		"✅ Record %s deleted from table '%s' successfully.",
		recordID,
		tableName,
	), "success"
}

// ================================================================================
// PUSHOVER TEST PAGE
// ================================================================================

// renderPushoverTestPage renders a page for testing Pushover notifications.
func renderPushoverTestPage(csrfToken string) gomponents.Node {
	// Get available notification configurations
	notifications := config.GetSettingsNotificationAll()

	var notificationConfigs []string
	for i := range notifications {
		if notifications[i].NotificationType == "pushover" {
			notificationConfigs = append(notificationConfigs, notifications[i].Name)
		}
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
					html.I(html.Class("fa-solid fa-paper-plane header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Pushover Test Message")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Send a test message through Pushover to verify your notification configuration is working correctly.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("pushoverForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Message Configuration"),
					),

					renderFormGroup("pushover", map[string]string{
						"NotificationConfig": "Select the Pushover notification configuration to use",
					}, map[string]string{
						"NotificationConfig": "Notification Config",
					}, "NotificationConfig", "select", "", map[string][]string{
						"options": notificationConfigs,
					}),

					renderFormGroup("pushover", map[string]string{
						"MessageTitle": "Title for the test message",
					}, map[string]string{
						"MessageTitle": "Message Title",
					}, "MessageTitle", "text", "Test Message from Media Downloader", nil),

					renderFormGroup("pushover", map[string]string{
						"MessageText": "Content of the test message",
					}, map[string]string{
						"MessageText": "Message Text",
					}, "MessageText", "textarea", "This is a test message to verify Pushover notification configuration is working correctly.", nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Pushover Information"),
					),
					html.P(
						gomponents.Text(
							"Pushover is a service that sends real-time notifications to your devices. To use this feature:",
						),
					),
					html.Ol(
						html.Li(gomponents.Text("Create a Pushover account at pushover.net")),
						html.Li(gomponents.Text("Get your User Key from your dashboard")),
						html.Li(gomponents.Text("Create an application to get an API Token")),
						html.Li(gomponents.Text("Configure these in your notification settings")),
					),
					html.P(
						html.Class("mt-3"),
						html.Strong(gomponents.Text("Note: ")),
						gomponents.Text(
							"Make sure you have at least one Pushover notification configuration set up before testing.",
						),
					),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Send Test Message"),
					html.Type("button"),
					hx.Target("#pushoverResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/pushovertest"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#pushoverForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('pushoverForm').reset(); document.getElementById('pushoverResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("pushoverResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// HandlePushoverTest handles Pushover test message requests.
func HandlePushoverTest(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	notificationConfig := c.PostForm("pushover_NotificationConfig")
	messageTitle := c.PostForm("pushover_MessageTitle")
	messageText := c.PostForm("pushover_MessageText")

	if notificationConfig == "" {
		c.String(
			http.StatusOK,
			renderAlert("Please select a notification configuration", "warning"),
		)

		return
	}

	if messageTitle == "" || messageText == "" {
		c.String(
			http.StatusOK,
			renderAlert("Please provide both message title and text", "warning"),
		)

		return
	}

	// Get the notification configuration
	notifCfg := config.GetSettingsNotification(notificationConfig)
	if notifCfg == nil {
		c.String(http.StatusOK, renderAlert("Notification configuration not found", "danger"))
		return
	}

	// Send the test message
	err := apiexternal.SendPushoverMessage(
		notifCfg.Name,
		notifCfg.Apikey,
		messageText,
		messageTitle,
		notifCfg.Recipient,
	)

	var result gomponents.Node
	if err != nil {
		result = html.Div(
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
						html.I(html.Class("fas fa-times-circle me-1")),
						gomponents.Text("Failed"),
					),
					html.H5(
						html.Class("card-title mb-0 text-danger fw-bold"),
						gomponents.Text("Message Send Failed"),
					),
				),
			),

			html.Div(
				html.Class("card border-0"),
				html.Style("background: rgba(220, 53, 69, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3 text-center"),
					html.I(
						html.Class("fas fa-exclamation-triangle mb-2"),
						html.Style("color: #dc3545; font-size: 2rem;"),
					),
					html.P(
						html.Class("mb-2"),
						html.Style("color: #495057;"),
						gomponents.Text("Failed to send Pushover message:"),
					),
					html.P(
						html.Class("mb-2"),
						html.Style(
							"color: #dc3545; font-family: monospace; background: rgba(220, 53, 69, 0.1); padding: 0.5rem; border-radius: 4px;",
						),
						gomponents.Text(err.Error()),
					),
					html.P(
						html.Class("mb-0"),
						html.Style("color: #495057;"),
						gomponents.Text(
							"Please check your notification configuration and try again.",
						),
					),
				),
			),
		)
	} else {
		result = html.Div(
			html.Class("card border-0 shadow-sm border-success mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(html.Class("badge bg-success me-3"), html.I(html.Class("fas fa-paper-plane me-1")), gomponents.Text("Success")),
					html.H5(html.Class("card-title mb-0 text-success fw-bold"), gomponents.Text("Message Sent Successfully")),
				),
			),

			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3"),
					html.P(html.Class("mb-3 text-center"), html.Style("color: #495057;"), gomponents.Text("Test message sent via Pushover with the following details:")),

					html.Div(
						html.Class("row g-2"),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(html.Class("d-flex align-items-center"),
										html.I(html.Class("fas fa-cogs me-2"), html.Style("color: #28a745;")),
										html.Div(
											html.Div(html.Class("small fw-bold text-muted"), gomponents.Text("Configuration")),
											html.Div(html.Class("fw-semibold"), gomponents.Text(notificationConfig)),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(html.Class("d-flex align-items-center"),
										html.I(html.Class("fas fa-user me-2"), html.Style("color: #28a745;")),
										html.Div(
											html.Div(html.Class("small fw-bold text-muted"), gomponents.Text("Recipient")),
											html.Div(html.Class("fw-semibold"), gomponents.Text(notifCfg.Recipient)),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-12"),
							html.Div(
								html.Class("card border-0"),
								html.Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(html.Class("d-flex align-items-start"),
										html.I(html.Class("fas fa-envelope me-2 mt-1"), html.Style("color: #28a745;")),
										html.Div(html.Class("flex-grow-1"),
											html.Div(html.Class("small fw-bold text-muted"), gomponents.Text("Message Details")),
											html.Div(html.Class("fw-semibold mb-1"), gomponents.Text("Title: "+messageTitle)),
											html.Div(html.Class("text-muted small"), gomponents.Text(messageText)),
										),
									),
								),
							),
						),
					),
				),
			),

			html.Div(
				html.Class("alert alert-light border-0 mb-0"),
				html.Style("background-color: rgba(40, 167, 69, 0.1); border-radius: 8px; padding: 0.75rem 1rem;"),
				html.Div(
					html.Class("d-flex align-items-start"),
					html.I(html.Class("fas fa-mobile-alt me-2 mt-1"), html.Style("color: #28a745; font-size: 0.9rem;")),
					html.Small(
						html.Style("color: #495057; line-height: 1.4;"),
						html.Strong(gomponents.Text("Next Step: ")),
						gomponents.Text("Check your device to confirm the message was received."),
					),
				),
			),
		)
	}

	c.String(http.StatusOK, renderComponentToString(result))
}

// ================================================================================
// LOG VIEWER PAGE
// ================================================================================

// renderLogViewerPage renders a page for viewing log files.
func renderLogViewerPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-file-text header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Log Viewer")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Real-time application logs and system events monitoring. Filter by log level and automatically refresh to stay up-to-date with system activity.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("logForm"),
			html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Log Configuration")),

					renderFormGroup("log", map[string]string{
						"LineCount": "Number of lines to display from the end of the log file",
					}, map[string]string{
						"LineCount": "Number of Lines",
					}, "LineCount", "number", "100", nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Filter Options")),

					renderFormGroup("log", map[string]string{
						"LogLevel": "Filter logs by level (leave empty for all)",
					}, map[string]string{
						"LogLevel": "Log Level Filter",
					}, "LogLevel", "select", "", map[string][]string{
						"options": {"", "error", "warn", "info", "debug"},
					}),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Load Log"),
					html.Type("button"),
					hx.Target("#logResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/logviewer"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logForm"),
					html.I(html.Class("fas fa-sync-alt me-1")),
				),
				html.Button(
					html.Class("btn btn-info ml-2"),
					gomponents.Text("Auto-Refresh"),
					html.Type("button"),
					hx.Target("#logResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/logviewer"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logForm"),
					hx.Trigger("every 5s"),
					html.ID("autoRefreshBtn"),
					html.I(html.Class("fas fa-play me-1")),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('logForm').reset(); document.getElementById('logResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("logResults"),
			html.Class("mt-4"),
			html.Style(
				"min-height: 400px; background: #1e1e1e; color: #f8f8f2; font-family: 'Courier New', monospace; padding: 1rem; overflow-y: auto; max-height: 600px; border-radius: 8px;",
			),
			html.P(
				html.Class("text-center text-muted"),
				gomponents.Text("Click 'Load Log' to view application logs..."),
			),
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
						html.I(html.Class("fas fa-info-circle me-1")),
						gomponents.Text("Usage"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Log Viewer Information"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Real-time log monitoring with filtering capabilities"),
				),
				html.Ul(
					html.Class("mb-3 list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("📂 Logs are read from the 'logs/downloader.log' file"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"🔄 Lines are displayed in reverse chronological order (newest first)",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"🎯 Use the log level filter to show only specific types of messages",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("⚡ Auto-refresh updates the display every 5 seconds"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("⏱️ Large log files may take a moment to load"),
					),
				),
			),
		),
	)
}

// HandleLogViewer handles log viewing requests.
func HandleLogViewer(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	lineCountStr := c.PostForm("log_LineCount")
	logLevel := c.PostForm("log_LogLevel")

	lineCount := 100
	if lineCountStr != "" {
		if parsed, err := strconv.Atoi(lineCountStr); err == nil && parsed > 0 {
			lineCount = parsed
		}
	}

	// Read log file
	logPath := filepath.Join("logs", "downloader.log")

	lines, err := readLastLines(logPath, lineCount, logLevel)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to read log file: "+err.Error(), "danger"))
		return
	}

	// Create log display
	var logNodes []gomponents.Node
	for _, line := range lines {
		// Determine log level for styling
		class := "text-muted"
		if strings.Contains(line, "error") || strings.Contains(line, "ERROR") {
			class = "text-danger"
		} else if strings.Contains(line, "warn") || strings.Contains(line, "WARN") {
			class = "text-warning"
		} else if strings.Contains(line, "info") || strings.Contains(line, "INFO") {
			class = "text-info"
		}

		logNodes = append(logNodes,
			html.Div(
				html.Class(class+" font-monospace small"),
				gomponents.Text(line),
			),
		)
	}

	result := html.Div(
		html.Class("card border-0 shadow-sm border-info mb-4"),
		html.Div(
			html.Class("card-header border-0"),
			html.Style(
				"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
			),
			html.Div(
				html.Class("d-flex align-items-center justify-content-between"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-file-text me-1")),
						gomponents.Text("Logs"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text(fmt.Sprintf("Log Entries (Last %d lines)", len(lines))),
					),
				),
				html.Span(
					html.Class("badge bg-info"),
					gomponents.Text(time.Now().Format("15:04:05")),
				),
			),
		),
		html.Div(
			html.Class("card-body"),
			html.P(
				html.Class("card-text text-muted mb-3"),
				gomponents.Text(
					fmt.Sprintf("Loaded at: %s", time.Now().Format("2006-01-02 15:04:05")),
				),
				func() gomponents.Node {
					if logLevel != "" {
						return gomponents.Text(" - Filtered by level: " + logLevel)
					}
					return gomponents.Text("")
				}(),
			),
			html.Div(
				html.Class("log-container"),
				html.Style(
					"max-height: 400px; overflow-y: auto; background: #f8f9fa; padding: 10px; border-radius: 4px;",
				),
				gomponents.Group(logNodes),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// readLastLines reads the last n lines from a file, optionally filtering by log level.
func readLastLines(filename string, lineCount int, logLevel string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileSize := stat.Size()
	buffer := make([]byte, fileSize)

	// Read file from end
	_, err = file.ReadAt(buffer, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	// Split into lines
	lines := strings.Split(string(buffer), "\n")

	// Filter out empty lines
	var filteredLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Apply log level filter if specified
			if logLevel == "" ||
				strings.Contains(strings.ToLower(line), strings.ToLower(logLevel)) {
				filteredLines = append(filteredLines, line)
			}
		}
	}

	// Get last n lines and reverse order (newest first)
	start := 0
	if len(filteredLines) > lineCount {
		start = len(filteredLines) - lineCount
	}

	result := filteredLines[start:]

	// Reverse the slice to show newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// renderMediaCleanupWizardPage renders a page for media cleanup operations.
func renderMediaCleanupWizardPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-broom header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Media Cleanup Wizard")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Find and remove orphaned files, duplicates, or broken links in your media library. Keep your collection clean and organized.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("cleanupForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Cleanup Options")),

					renderFormGroup("cleanup",
						map[string]string{
							"FindOrphans": "Files on disk that aren't tracked in the database",
						},
						map[string]string{
							"FindOrphans": "Find Orphaned Files",
						},
						"FindOrphans", "checkbox", true, nil),

					renderFormGroup("cleanup",
						map[string]string{
							"FindDuplicates": "Files with identical names or sizes",
						},
						map[string]string{
							"FindDuplicates": "Find Duplicate Files",
						},
						"FindDuplicates", "checkbox", false, nil),

					renderFormGroup("cleanup",
						map[string]string{
							"FindBroken": "Database entries pointing to missing files",
						},
						map[string]string{
							"FindBroken": "Find Broken Links",
						},
						"FindBroken", "checkbox", false, nil),

					renderFormGroup("cleanup",
						map[string]string{
							"FindEmpty": "Folders that contain no media files",
						},
						map[string]string{
							"FindEmpty": "Find Empty Directories",
						},
						"FindEmpty", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Scan Configuration"),
					),

					renderFormGroup("cleanup", map[string]string{
						"MediaTypes": "Select which media types to scan",
					}, map[string]string{
						"MediaTypes": "Media Types",
					}, "MediaTypes", "select", "all", map[string][]string{
						"options": {"all", "movies", "series"},
					}),

					renderFormGroup("cleanup", map[string]string{
						"MinFileSize": "Minimum file size in MB to consider (0 = no limit)",
					}, map[string]string{
						"MinFileSize": "Min File Size (MB)",
					}, "MinFileSize", "number", "10", nil),

					renderFormGroup("cleanup", map[string]string{
						"Paths": "Specific paths to scan (leave empty for all configured paths)",
					}, map[string]string{
						"Paths": "Scan Paths",
					}, "Paths", "textarea", "", nil),

					renderFormGroup("cleanup",
						map[string]string{
							"DryRun": "Scan and report issues without making changes",
						},
						map[string]string{
							"DryRun": "Dry Run (Preview Only)",
						},
						"DryRun", "checkbox", true, nil),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Start Cleanup Scan"),
					html.Type("button"),
					hx.Target("#cleanupResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/cleanup"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#cleanupForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('cleanupForm').reset(); document.getElementById('cleanupResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("cleanupResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// ================================================================================
// CRON EXPRESSION GENERATOR PAGE
// ================================================================================

// renderCronGeneratorPage renders the cron expression generator page.
func renderCronGeneratorPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("container-fluid"),

		// Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-clock header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(
						html.Class("header-title"),
						gomponents.Text("Cron Expression Generator"),
					),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Generate and validate cron expressions for scheduling tasks. Choose from presets or build custom expressions with validation and examples.",
						),
					),
				),
			),
		),

		// Generator Form
		html.Div(
			html.Class("row"),

			// Input Form Column
			html.Div(
				html.Class("col-lg-6"),
				html.Div(
					html.Class("card"),
					html.Div(
						html.Class("card-header"),
						html.H5(
							html.Class("card-title mb-0"),
							gomponents.Text("Generate Expression"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Form(
							html.ID("cronForm"),
							html.Input(
								html.Type("hidden"),
								html.Name("csrf_token"),
								html.Value(csrfToken),
							),

							// Preset Options
							html.Div(
								html.Class("form-group mb-3"),
								html.Label(
									html.Class("form-label fw-bold"),
									gomponents.Text("Quick Presets"),
								),
								html.Div(
									html.Class("btn-group d-flex flex-wrap gap-2"),
									html.Button(
										html.Type("button"),
										html.Class("btn btn-outline-primary btn-sm"),
										gomponents.Attr("data-cron", "*/30 * * * * *"),
										gomponents.Text("Every 30s"),
									),
									html.Button(
										html.Type("button"),
										html.Class("btn btn-outline-primary btn-sm"),
										gomponents.Attr("data-cron", "0 */5 * * * *"),
										gomponents.Text("Every 5min"),
									),
									html.Button(
										html.Type("button"),
										html.Class("btn btn-outline-primary btn-sm"),
										gomponents.Attr("data-cron", "0 0 * * * *"),
										gomponents.Text("Every Hour"),
									),
									html.Button(
										html.Type("button"),
										html.Class("btn btn-outline-primary btn-sm"),
										gomponents.Attr("data-cron", "0 0 0 * * *"),
										gomponents.Text("Daily at Midnight"),
									),
									html.Button(
										html.Type("button"),
										html.Class("btn btn-outline-primary btn-sm"),
										gomponents.Attr("data-cron", "0 0 12 * * *"),
										gomponents.Text("Daily at Noon"),
									),
									html.Button(
										html.Type("button"),
										html.Class("btn btn-outline-primary btn-sm"),
										gomponents.Attr("data-cron", "0 0 0 * * 0"),
										gomponents.Text("Weekly (Sunday)"),
									),
									html.Button(
										html.Type("button"),
										html.Class("btn btn-outline-primary btn-sm"),
										gomponents.Attr("data-cron", "0 0 0 1 * *"),
										gomponents.Text("Monthly"),
									),
								),
							),

							html.Hr(),

							// Manual Input Fields
							html.Div(
								html.Class("row"),
								html.Div(
									html.Class("col-md-2"),
									html.Div(
										html.Class("form-group"),
										html.Label(
											html.Class("form-label"),
											gomponents.Text("Second"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control cron-field"),
											html.ID("second"),
											html.Placeholder("0-59"),
											html.Value("0"),
											gomponents.Attr("data-toggle", "tooltip"),
											gomponents.Attr(
												"title",
												"0-59 or * or */5 for every 5 seconds",
											),
										),
										html.Small(
											html.Class("form-text text-muted"),
											gomponents.Text("0-59"),
										),
									),
								),
								html.Div(
									html.Class("col-md-2"),
									html.Div(
										html.Class("form-group"),
										html.Label(
											html.Class("form-label"),
											gomponents.Text("Minute"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control cron-field"),
											html.ID("minute"),
											html.Placeholder("0-59"),
											html.Value("0"),
											gomponents.Attr("data-toggle", "tooltip"),
											gomponents.Attr(
												"title",
												"0-59 or * or */5 for every 5 minutes",
											),
										),
										html.Small(
											html.Class("form-text text-muted"),
											gomponents.Text("0-59"),
										),
									),
								),
								html.Div(
									html.Class("col-md-2"),
									html.Div(
										html.Class("form-group"),
										html.Label(
											html.Class("form-label"),
											gomponents.Text("Hour"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control cron-field"),
											html.ID("hour"),
											html.Placeholder("0-23"),
											html.Value("*"),
											gomponents.Attr("data-toggle", "tooltip"),
											gomponents.Attr(
												"title",
												"0-23 or * or */2 for every 2 hours",
											),
										),
										html.Small(
											html.Class("form-text text-muted"),
											gomponents.Text("0-23"),
										),
									),
								),
								html.Div(
									html.Class("col-md-2"),
									html.Div(
										html.Class("form-group"),
										html.Label(
											html.Class("form-label"),
											gomponents.Text("Day"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control cron-field"),
											html.ID("day"),
											html.Placeholder("1-31"),
											html.Value("*"),
											gomponents.Attr("data-toggle", "tooltip"),
											gomponents.Attr("title", "1-31 or * or L for last day"),
										),
										html.Small(
											html.Class("form-text text-muted"),
											gomponents.Text("1-31"),
										),
									),
								),
								html.Div(
									html.Class("col-md-2"),
									html.Div(
										html.Class("form-group"),
										html.Label(
											html.Class("form-label"),
											gomponents.Text("Month"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control cron-field"),
											html.ID("month"),
											html.Placeholder("1-12"),
											html.Value("*"),
											gomponents.Attr("data-toggle", "tooltip"),
											gomponents.Attr("title", "1-12 or JAN-DEC or *"),
										),
										html.Small(
											html.Class("form-text text-muted"),
											gomponents.Text("1-12"),
										),
									),
								),
								html.Div(
									html.Class("col-md-2"),
									html.Div(
										html.Class("form-group"),
										html.Label(
											html.Class("form-label"),
											gomponents.Text("Weekday"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control cron-field"),
											html.ID("weekday"),
											html.Placeholder("0-6"),
											html.Value("*"),
											gomponents.Attr("data-toggle", "tooltip"),
											gomponents.Attr(
												"title",
												"0-6 (0=Sunday) or SUN-SAT or *",
											),
										),
										html.Small(
											html.Class("form-text text-muted"),
											gomponents.Text("0-6"),
										),
									),
								),
							),

							html.Hr(),

							// Manual Expression Input
							html.Div(
								html.Class("form-group"),
								html.Label(
									html.Class("form-label fw-bold"),
									gomponents.Text("Manual Expression"),
								),
								html.Div(
									html.Class("input-group"),
									html.Input(
										html.Type("text"),
										html.Class("form-control font-monospace"),
										html.ID("cronExpression"),
										html.Placeholder("0 12 * * 1"),
										gomponents.Attr("data-toggle", "tooltip"),
										gomponents.Attr(
											"title",
											"Enter a complete cron expression",
										),
										hx.Post("/api/admin/crongen/validate"),
										hx.Target("#cronResult"),
										hx.Swap("innerHTML"),
										hx.Trigger("input changed delay:500ms"),
										hx.Headers(createHTMXHeaders(csrfToken)),
									),
									html.Button(
										html.Class("btn btn-primary"),
										html.Type("button"),
										gomponents.Attr(
											"onclick",
											"htmx.trigger('#cronExpression', 'input')",
										),
										gomponents.Text("Validate"),
									),
								),
							),
						),
					),
				),
			),

			// Result Column
			html.Div(
				html.Class("col-lg-6"),
				html.Div(
					html.Class("card"),
					html.Div(
						html.Class("card-header"),
						html.H5(
							html.Class("card-title mb-0"),
							gomponents.Text("Expression Details"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.ID("cronResult"),
							html.Class("p-3 bg-light rounded"),
							html.P(
								html.Class("text-muted mb-0"),
								gomponents.Text("Enter a cron expression to see its details..."),
							),
						),
					),
				),

				// Help Card
				html.Div(
					html.Class("card mt-4"),
					html.Div(
						html.Class("card-header"),
						html.H5(html.Class("card-title mb-0"), gomponents.Text("Cron Format Help")),
					),
					html.Div(
						html.Class("card-body"),
						html.Table(
							html.Class("table table-sm"),
							html.THead(
								html.Tr(
									html.Th(gomponents.Text("Field")),
									html.Th(gomponents.Text("Values")),
									html.Th(gomponents.Text("Special Characters")),
								),
							),
							html.TBody(
								html.Tr(
									html.Td(gomponents.Text("Second")),
									html.Td(gomponents.Text("0-59")),
									html.Td(gomponents.Text("* , - /")),
								),
								html.Tr(
									html.Td(gomponents.Text("Minute")),
									html.Td(gomponents.Text("0-59")),
									html.Td(gomponents.Text("* , - /")),
								),
								html.Tr(
									html.Td(gomponents.Text("Hour")),
									html.Td(gomponents.Text("0-23")),
									html.Td(gomponents.Text("* , - /")),
								),
								html.Tr(
									html.Td(gomponents.Text("Day")),
									html.Td(gomponents.Text("1-31")),
									html.Td(gomponents.Text("* , - / L")),
								),
								html.Tr(
									html.Td(gomponents.Text("Month")),
									html.Td(gomponents.Text("1-12 or JAN-DEC")),
									html.Td(gomponents.Text("* , - /")),
								),
								html.Tr(
									html.Td(gomponents.Text("Weekday")),
									html.Td(gomponents.Text("0-7 or SUN-SAT")),
									html.Td(gomponents.Text("* , - / L #")),
								),
							),
						),
						html.Div(
							html.Class("mt-3"),
							html.H6(gomponents.Text("Examples:")),
							html.Ul(
								html.Class("list-unstyled small text-muted"),
								html.Li(
									html.Code(gomponents.Text("0 0 12 * * *")),
									gomponents.Text(" - Daily at 12:00 PM (6-field)"),
								),
								html.Li(
									html.Code(gomponents.Text("0 12 * * *")),
									gomponents.Text(" - Daily at 12:00 PM (5-field)"),
								),
								html.Li(
									html.Code(gomponents.Text("*/30 * * * * *")),
									gomponents.Text(" - Every 30 seconds"),
								),
								html.Li(
									html.Code(gomponents.Text("0 */15 * * * *")),
									gomponents.Text(" - Every 15 minutes"),
								),
								html.Li(
									html.Code(gomponents.Text("0 0 9-17 * * 1-5")),
									gomponents.Text(" - Every hour from 9-5, Mon-Fri"),
								),
							),
						),
					),
				),
			),
		),

		// Complete JavaScript for Cron Generator
		html.Script(gomponents.Raw(`
			$(document).ready(function() {
				// Initialize tooltips
				$('[data-toggle="tooltip"]').tooltip();

				// Expression builder function
				function updateExpression() {
					const second = $('#second').val() || '0';
					const minute = $('#minute').val() || '0';
					const hour = $('#hour').val() || '*';
					const day = $('#day').val() || '*';
					const month = $('#month').val() || '*';
					const weekday = $('#weekday').val() || '*';
					const expression = second + ' ' + minute + ' ' + hour + ' ' + day + ' ' + month + ' ' + weekday;
					$('#cronExpression').val(expression);
					validateExpression(expression);
				}

				// Field change handlers
				$('.cron-field').on('input', updateExpression);

				// Preset button handlers
				$('[data-cron]').click(function() {
					const cron = $(this).data('cron');
					const parts = cron.split(' ');
					if (parts.length === 5) {
						// 5-field format: minute hour day month weekday -> convert to 6-field
						$('#second').val('0');
						$('#minute').val(parts[0] || '*');
						$('#hour').val(parts[1] || '*');
						$('#day').val(parts[2] || '*');
						$('#month').val(parts[3] || '*');
						$('#weekday').val(parts[4] || '*');
						// Convert to 6-field expression
						const sixFieldExpr = '0 ' + cron;
						$('#cronExpression').val(sixFieldExpr);
						validateExpression(sixFieldExpr);
					} else if (parts.length === 6) {
						// 6-field format: second minute hour day month weekday
						$('#second').val(parts[0] || '0');
						$('#minute').val(parts[1] || '0');
						$('#hour').val(parts[2] || '*');
						$('#day').val(parts[3] || '*');
						$('#month').val(parts[4] || '*');
						$('#weekday').val(parts[5] || '*');
						$('#cronExpression').val(cron);
						validateExpression(cron);
					}
				});

				// Manual expression input handler
				$('#cronExpression').on('input', function() {
					const expression = $(this).val();
					const parts = expression.split(' ');
					if (parts.length === 5) {
						// 5-field format: minute hour day month weekday
						$('#second').val('0');
						$('#minute').val(parts[0]);
						$('#hour').val(parts[1]);
						$('#day').val(parts[2]);
						$('#month').val(parts[3]);
						$('#weekday').val(parts[4]);
					} else if (parts.length === 6) {
						// 6-field format: second minute hour day month weekday
						$('#second').val(parts[0]);
						$('#minute').val(parts[1]);
						$('#hour').val(parts[2]);
						$('#day').val(parts[3]);
						$('#month').val(parts[4]);
						$('#weekday').val(parts[5]);
					}
					validateExpression(expression);
				});

				// Validate button handler
				$('#validateBtn').click(function() {
					const expression = $('#cronExpression').val();
					validateExpression(expression);
				});

				// Validation function
				function validateExpression(expression) {
					if (!expression || expression.trim() === '') {
						$('#cronResult').html('<p class="text-muted mb-0">Enter a cron expression to see its details...</p>');
						return;
					}
					// Get CSRF token from form
					const csrfToken = document.querySelector('input[name="csrf_token"]').value;

					$.ajax({
						url: '/api/admin/crongen/validate',
						method: 'POST',
						headers: {
							'X-CSRF-Token': csrfToken,
							'Content-Type': 'application/x-www-form-urlencoded'
						},
						data: { expression: expression },
						success: function(response) {
							$('#cronResult').html(response);
						},
						error: function() {
							$('#cronResult').html('<div class="alert alert-danger">Error validating expression</div>');
						}
					});
				}

				// Initialize with default expression
				updateExpression();
			});
		`)),
	)
}

func renderQualityReorderPage(csrfToken string) gomponents.Node {
	// Get all available quality profiles
	qualityConfigs := config.GetSettingsQualityAll()

	var qualityOptions []gomponents.Node

	qualityOptions = append(
		qualityOptions,
		html.Option(html.Value(""), gomponents.Text("-- Select Quality Profile --")),
	)
	for _, qc := range qualityConfigs {
		qualityOptions = append(
			qualityOptions,
			html.Option(html.Value(qc.Name), gomponents.Text(qc.Name)),
		)
	}

	// Get all resolutions and qualities from database for filter options
	allResolutions := database.DBConnect.GetresolutionsIn
	allQualities := database.DBConnect.GetqualitiesIn

	var resolutionOptions []gomponents.Node

	resolutionOptions = append(
		resolutionOptions,
		html.Option(html.Value("all"), gomponents.Text("All Resolutions")),
	)
	for _, res := range allResolutions {
		if res.Name != "" {
			resolutionOptions = append(
				resolutionOptions,
				html.Option(html.Value(res.Name), gomponents.Text(res.Name)),
			)
		}
	}

	var qualityFilterOptions []gomponents.Node

	qualityFilterOptions = append(
		qualityFilterOptions,
		html.Option(html.Value("all"), gomponents.Text("All Qualities")),
	)
	for _, qual := range allQualities {
		if qual.Name != "" {
			qualityFilterOptions = append(
				qualityFilterOptions,
				html.Option(html.Value(qual.Name), gomponents.Text(qual.Name)),
			)
		}
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		// Enhanced page header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-sort-amount-down header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Quality Reorder Testing")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Experiment with quality settings and preview the resulting order and priority",
						),
					),
				),
			),
		),

		// Main form
		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style(
							"background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;",
						),
						html.H5(html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-cog me-2 text-primary")),
							gomponents.Text("Quality Profile Selection"),
						),
					),
					html.Div(
						html.Class("card-body p-4"),
						html.Form(
							html.ID("quality-reorder-form"),
							hx.Post("/api/admin/quality-reorder"),
							hx.Target("#quality-results"),
							hx.Swap("innerHTML"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							html.Input(
								html.Type("hidden"),
								html.Name("csrf_token"),
								html.Value(csrfToken),
							),

							html.Div(
								html.Class("mb-4"),
								html.Label(
									html.Class("form-label fw-semibold"),
									gomponents.Text("Quality Profile"),
								),
								html.Select(
									html.Class("form-select"),
									html.Name("selected_quality"),
									html.ID("selected_quality"),
									html.Required(),
									gomponents.Attr("hx-get", "/api/admin/quality-reorder/rules"),
									gomponents.Attr("hx-target", "#reorder-rules-container"),
									gomponents.Attr("hx-trigger", "change"),
									gomponents.Attr("hx-vals", "js:{profile: this.value}"),
									gomponents.Attr(
										"hx-headers",
										`{"X-CSRF-Token": "`+csrfToken+`"}`,
									),
									gomponents.Attr("hx-indicator", "#loading-indicator"),
									gomponents.Group(qualityOptions),
								),
								html.Div(
									html.Class("form-text"),
									gomponents.Text(
										"Select a quality profile to view its current ordering",
									),
								),
							),

							html.Div(
								html.Class("row mb-4"),
								html.Div(
									html.Class("col-md-6"),
									html.Label(
										html.Class("form-label fw-semibold"),
										gomponents.Text("Filter by Resolution"),
									),
									html.Select(
										html.Class("form-select"),
										html.Name("filter_resolution"),
										html.ID("filter_resolution"),
										gomponents.Group(resolutionOptions),
									),
									html.Div(
										html.Class("form-text"),
										gomponents.Text("Filter results to specific resolution"),
									),
								),
								html.Div(
									html.Class("col-md-6"),
									html.Label(
										html.Class("form-label fw-semibold"),
										gomponents.Text("Filter by Quality"),
									),
									html.Select(
										html.Class("form-select"),
										html.Name("filter_quality"),
										html.ID("filter_quality"),
										gomponents.Group(qualityFilterOptions),
									),
									html.Div(
										html.Class("form-text"),
										gomponents.Text(
											"Filter results to specific quality source",
										),
									),
								),
							),

							// Additional Filter Options Row
							html.Div(
								html.Class("row mb-4"),
								html.Div(
									html.Class("col-12"),
									html.Div(
										html.Class("card border-0 shadow-sm"),
										html.Style("border-radius: 10px;"),
										html.Div(
											html.Class("card-header border-0"),
											html.Style(
												"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 1rem;",
											),
											html.H6(html.Class("card-title mb-0"),
												html.I(html.Class("fas fa-filter me-2 text-info")),
												gomponents.Text("Filter Options"),
											),
										),
										html.Div(
											html.Class("card-body p-3"),
											renderFormGroup("",
												map[string]string{
													"wanted_only": "When enabled, only shows qualities that are actively wanted by the selected profile. When disabled, shows all available qualities regardless of wanted status.",
												},
												map[string]string{
													"wanted_only": "Show Wanted Qualities Only",
												},
												"wanted_only", "checkbox", false, nil),
										),
									),
								),
							),

							// Reorder Rules Management Section
							html.Div(
								html.Class("mb-4"),
								html.Div(
									html.Class(
										"d-flex justify-content-between align-items-center mb-3",
									),
									html.H6(html.Class("mb-0 fw-semibold"),
										html.I(html.Class("fas fa-flask me-2 text-warning")),
										gomponents.Text("Temporary Reorder Rules"),
									),
									html.Div(
										html.Class("btn-group"),
										html.Button(
											html.Type("button"),
											html.Class("btn btn-sm btn-outline-primary"),
											hx.Post("/api/admin/quality-reorder/add-rule"),
											hx.Target("#reorder-rules-container"),
											hx.Swap("beforeend"),
											hx.Headers(createHTMXHeaders(csrfToken)),
											html.I(html.Class("fas fa-plus me-1")),
											gomponents.Text("Add Rule"),
										),
										html.Button(
											html.Type("button"),
											html.Class("btn btn-sm btn-outline-secondary"),
											hx.Post("/api/admin/quality-reorder/reset-rules"),
											hx.Target("#reorder-rules-container"),
											hx.Swap("innerHTML"),
											hx.Headers(createHTMXHeaders(csrfToken)),
											hx.Confirm(
												"Are you sure you want to reset all temporary reorder rules?",
											),
											html.I(html.Class("fas fa-undo me-1")),
											gomponents.Text("Reset"),
										),
									),
								),
								// Rules headers
								html.Div(
									html.Class("row mb-2 px-3"),
									html.Div(
										html.Class("col-md-2"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("TYPE"),
										),
									),
									html.Div(
										html.Class("col-md-3"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("PATTERN"),
										),
									),
									html.Div(
										html.Class("col-md-2"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("PRIORITY (+/-)"),
										),
									),
									html.Div(
										html.Class("col-md-3"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("ENABLED"),
										),
									),
									html.Div(
										html.Class("col-md-2"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("ACTIONS"),
										),
									),
								),
								html.Div(
									html.ID("reorder-rules-container"),
									html.Class("border rounded p-3 bg-light"),
									html.P(
										html.Class("text-muted mb-0 text-center"),
										gomponents.Text(
											"No temporary reorder rules. Select a quality profile above to load rules.",
										),
									),
								),
								html.Div(
									html.ID("loading-indicator"),
									html.Class("htmx-indicator text-center p-2"),
									html.I(html.Class("fas fa-spinner fa-spin me-2")),
									gomponents.Text("Loading rules..."),
								),
								html.Input(
									html.Type("hidden"),
									html.Name("reorder_rules"),
									html.ID("reorder_rules_json"),
									html.Value("[]"),
								),
								html.Div(
									html.Class("form-text mt-2"),
									gomponents.Text(
										"Add temporary reorder rules to test priority changes without modifying the actual configuration. Changes are not saved permanently. Priorities can be negative to reduce ranking.",
									),
								),
							),

							html.Div(
								html.Class("d-flex gap-2"),
								html.Button(
									html.Type("submit"),
									html.Class("btn btn-primary px-4"),
									hx.Post("/api/admin/quality-reorder"),
									hx.Target("#quality-results"),
									hx.Swap("innerHTML"),
									hx.Headers(createHTMXHeaders(csrfToken)),
									hx.Include("#quality-reorder-form"),
									hx.Indicator("#loading-indicator"),
									html.I(html.Class("fas fa-eye me-2")),
									gomponents.Text("Preview Quality Order"),
								),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-secondary px-4"),
									html.ID("refresh-profiles"),
									html.I(html.Class("fas fa-sync-alt me-2")),
									gomponents.Text("Refresh Profiles"),
								),
							),
						),
					),
				),
			),
		),

		// Results container
		html.Div(
			html.ID("quality-results"),
			html.Class("mt-4"),
		),

		// Instructions card
		html.Div(
			html.Class("row mt-4"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm bg-light"),
					html.Div(
						html.Class("card-body p-4"),
						html.H6(html.Class("card-title text-muted mb-3"),
							html.I(html.Class("fas fa-info-circle me-2")),
							gomponents.Text("How to Use This Tool"),
						),
						html.Ul(
							html.Class("list-unstyled mb-0 text-muted small"),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"Select a quality profile from the dropdown to view its current ordering",
								),
							),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"Use filters to focus on specific resolutions or quality sources",
								),
							),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-flask text-warning me-2")),
								gomponents.Text(
									"Add temporary reorder rules to test priority changes (positive values increase ranking, negative decrease)",
								),
							),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"Enable/disable rules to see how they affect priority calculations",
								),
							),
							html.Li(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"The results table shows resolution + quality combinations ranked by total priority",
								),
							),
						),
					),
				),
			),
		),

		// Complete JavaScript for Quality Reorder
		html.Script(gomponents.Raw(`
			$(document).ready(function() {
				
				// Refresh button handler
				$('#refresh-profiles').on('click', function() {
					location.reload();
				});

				// Profile selection handler - removed jQuery handler as HTMX handles this directly

				// Function to collect temporary reorder rules and update the JSON field
				function updateReorderRulesJSON() {
					var rules = [];
					$('#reorder-rules-container [data-rule-id]').each(function() {
						var $rule = $(this);
						var type = $rule.find('select[name="rule_type"]').val();
						var pattern = $rule.find('input[name="rule_pattern"]').val();
						var priority = parseInt($rule.find('input[name="rule_priority"]').val()) || 0;
						var enabled = $rule.find('input[name="rule_enabled"]').is(':checked');
						
						if (type && pattern && enabled) {
							rules.push({
								name: pattern,
								type: type,
								new_priority: priority
							});
						}
					});
					
					$('#reorder_rules_json').val(JSON.stringify(rules));
				}
				
				// Update JSON before form submission
				$(document).on('htmx:configRequest', function(evt) {
					if (evt.target.id === 'quality-reorder-form' || $(evt.target).closest('#quality-reorder-form').length) {
						updateReorderRulesJSON();
					}
				});
				
				// Update JSON when rules change
				$(document).on('change', '#reorder-rules-container input, #reorder-rules-container select', function() {
					updateReorderRulesJSON();
				});
				
				// Filter change handlers - trigger HTMX form submission
				$('#filter_resolution, #filter_quality, #wanted_only, #_wanted_only').on('change', function() {
					if ($('#selected_quality').val()) {
						// Use HTMX to trigger form submission
						htmx.trigger('#quality-reorder-form', 'submit');
					}
				});

				document.addEventListener('htmx:responseError', function(evt) {
					console.error('HTMX Response Error:', evt.detail);
				});
			});
		`)),
	)
}
