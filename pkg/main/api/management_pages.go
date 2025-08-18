package api

import (
	"context"
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
	. "maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	. "maragu.dev/gomponents/html"
)

// ================================================================================
// JOB MANAGEMENT PAGE
// ================================================================================

// renderJobManagementPage renders a page for starting jobs
func renderJobManagementPage(csrfToken string) Node {
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

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-tasks header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Job Management")),
					P(Class("header-subtitle"), Text("Start single jobs for media processing, data refreshing, and searching. Jobs will be queued and executed based on your configuration settings.")),
				),
			),
		),

		Form(
			Class("config-form"),
			ID("jobForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-4"),
					H5(Class("form-section-title"), Text("Job Configuration")),

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

				Div(
					Class("col-md-4"),
					H5(Class("form-section-title"), Text("Job Options")),

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

				Div(
					Class("col-md-4"),
					H5(Class("form-section-title"), Text("Job Descriptions")),
					Details(
						Summary(Text("Job Type Descriptions")),
						Ul(
							Li(Strong(Text("data: ")), Text("Scan and import new media files")),
							Li(Strong(Text("datafull: ")), Text("Full rescan of all media files")),
							Li(Strong(Text("feeds: ")), Text("Refresh RSS feeds and lists")),
							Li(Strong(Text("rss: ")), Text("Process RSS downloads")),
							Li(Strong(Text("rssseasons/rssseasonsall: ")), Text("Season-specific RSS processing")),
							Li(Strong(Text("checkmissing: ")), Text("Check for missing episodes/movies")),
							Li(Strong(Text("search*: ")), Text("Various search operations for missing/upgrade content")),
							Li(Strong(Text("clearhistory: ")), Text("Clear download history")),
						),
					),
				),
			),

			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Start Job"),
					Type("button"),
					hx.Target("#jobResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/jobmanagement"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#jobForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('jobForm').reset(); document.getElementById('jobResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("jobResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),

		// Instructions
		Div(
			Class("mt-4 card border-0 shadow-sm border-info mb-4"),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-info me-3"), I(Class("fas fa-info-circle me-1")), Text("Usage")),
					H5(Class("card-title mb-0 text-info fw-bold"), Text("Usage Instructions")),
				),
			),
			Div(
				Class("card-body"),
				P(Class("card-text text-muted mb-3"), Text("Follow these steps to execute management jobs")),
				Ol(
					Class("mb-0 list-unstyled"),
					Li(Class("mb-2"), Text("1. Select the job type you want to execute")),
					Li(Class("mb-2"), Text("2. Choose the media configuration (required for most jobs)")),
					Li(Class("mb-2"), Text("3. Optionally specify a list name for targeted processing")),
					Li(Class("mb-2"), Text("4. Check 'Force Execution' to run even if scheduler is disabled")),
					Li(Class("mb-2"), Text("5. Click 'Start Job' to queue the job for execution")),
				),
			),
		),

		Div(
			Class("card border-0 shadow-sm border-info mb-4"),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-info me-3"), I(Class("fas fa-lightbulb me-1")), Text("Note")),
					H5(Class("card-title mb-0 text-info fw-bold"), Text("Important Information")),
				),
			),
			Div(
				Class("card-body"),
				P(Class("card-text text-muted mb-0"), Text("Jobs are queued and executed asynchronously. Check the scheduler page for job status and history.")),
			),
		),
	)
}

// HandleJobManagement handles job start requests
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

	result := Div(
		Class("card border-0 shadow-sm border-success mb-4"),

		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge bg-success me-3"), I(Class("fas fa-play-circle me-1")), Text("Started")),
				H5(Class("card-title mb-0 text-success fw-bold"), Text("Job Started Successfully")),
			),
		),

		Div(
			Class("card-body"),
			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				Div(
					Class("card-body p-3"),
					P(Class("mb-3"), Style("color: #495057;"), Text(fmt.Sprintf("Job '%s' has been queued for execution with the following parameters:", jobType))),

					Div(
						Class("row g-2 mb-3"),
						Div(
							Class("col-sm-6"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2"),
									Div(Class("d-flex align-items-center"),
										I(Class("fas fa-cog me-2"), Style("color: #28a745;")),
										Div(
											Div(Class("small fw-bold text-muted"), Text("Job Type")),
											Div(Class("fw-semibold"), Text(jobType)),
										),
									),
								),
							),
						),
						Div(
							Class("col-sm-6"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2"),
									Div(Class("d-flex align-items-center"),
										I(Class("fas fa-film me-2"), Style("color: #28a745;")),
										Div(
											Div(Class("small fw-bold text-muted"), Text("Media Config")),
											Div(Class("fw-semibold"), Text(mediaConfig)),
										),
									),
								),
							),
						),
						Div(
							Class("col-sm-6"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2"),
									Div(Class("d-flex align-items-center"),
										I(Class("fas fa-list me-2"), Style("color: #28a745;")),
										Div(
											Div(Class("small fw-bold text-muted"), Text("List Name")),
											Div(Class("fw-semibold"), func() Node {
												if listName != "" {
													return Text(listName)
												}
												return Text("(all lists)")
											}()),
										),
									),
								),
							),
						),
						Div(
							Class("col-sm-6"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2"),
									Div(Class("d-flex align-items-center"),
										I(Class("fas fa-bolt me-2"), Style("color: #28a745;")),
										Div(
											Div(Class("small fw-bold text-muted"), Text("Force Execution")),
											Div(Class("fw-semibold"), Text(fmt.Sprintf("%t", force))),
										),
									),
								),
							),
						),
					),
				),
			),

			Div(
				Class("card border-0 mb-0"),
				Style("background-color: rgba(40, 167, 69, 0.1); border-radius: 8px;"),
				Div(
					Class("card-body p-3"),
					Div(
						Class("d-flex align-items-start"),
						I(Class("fas fa-info-circle me-2 mt-1"), Style("color: #28a745; font-size: 0.9rem;")),
						Small(
							Style("color: #495057; line-height: 1.4;"),
							Strong(Text("Status: ")),
							Text("The job is now running in the background. You can monitor its progress in the scheduler interface."),
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

// renderDebugStatsPage renders a page for viewing debug statistics
func renderDebugStatsPage(csrfToken string) Node {
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-bug header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Debug Statistics")),
					P(Class("header-subtitle"), Text("View runtime statistics, memory usage, garbage collection info, and worker statistics. This information is useful for monitoring application performance and troubleshooting issues.")),
				),
			),
		),

		Div(
			Class("form-group"),
			Button(
				Class("btn btn-primary"),
				Text("Refresh Debug Stats"),
				Type("button"),
				hx.Target("#debugResults"),
				hx.Swap("innerHTML"),
				hx.Post("/api/admin/debugstats"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			),
			Button(
				Class("btn btn-warning ml-2"),
				Text("Force Garbage Collection"),
				Type("button"),
				hx.Target("#debugResults"),
				hx.Swap("innerHTML"),
				hx.Post("/api/admin/debugstats"),
				hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
				hx.Vals("{\"action\": \"gc\"}"),
			),
		),

		Div(
			ID("debugResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),

		// Instructions
		Div(
			Class("mt-4 card border-0 shadow-sm border-info"),
			Div(
				Class("card-body"),
				H5(Class("card-title fw-bold mb-3"), Text("Debug Information")),
				P(Class("card-text text-muted mb-3"), Text("Available debug operations and system information")),
				Ul(Class("list-unstyled"),
					Li(Class("mb-2"),
						Span(Class("badge bg-primary me-2"), I(Class("fas fa-server me-1")), Text("Runtime")),
						Text("Go runtime information including OS, CPU count, and goroutine count"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-info me-2"), I(Class("fas fa-memory me-1")), Text("Memory")),
						Text("Detailed memory usage including heap, stack, and GC statistics"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-success me-2"), I(Class("fas fa-recycle me-1")), Text("GC Stats")),
						Text("Garbage collection performance metrics and timing"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-warning me-2"), I(Class("fas fa-tasks me-1")), Text("Workers")),
						Text("Background job queue and worker pool statistics"),
					),
					Li(Class("mb-0"),
						Span(Class("badge bg-danger me-2"), I(Class("fas fa-download me-1")), Text("Heap Dump")),
						Text("Memory heap dump is saved to temp/heapdump for analysis"),
					),
				),
			),
		),
	)
}

// HandleDebugStats handles debug statistics requests
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

	result := Div(
		Class("card border-0 shadow-sm border-info mb-4"),
		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center"),
				Span(Class("badge bg-info me-3"), I(Class("fas fa-chart-line me-1")), Text("Statistics")),
				H5(Class("card-title mb-0 text-info fw-bold"), Text("Debug Statistics")),
			),
		),
		Div(
			Class("card-body"),
			P(Class("card-text text-muted mb-3"), Text(fmt.Sprintf("Generated at: %s", time.Now().Format("2006-01-02 15:04:05")))),

			// Runtime Information
			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				Div(
					Class("card-header bg-transparent border-0 pb-0"),
					Style("background: transparent !important;"),
					H6(Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"), Text("Runtime Information")),
				),
				Div(
					Class("card-body pt-2"),
					Table(
						Class("table table-hover table-sm mb-0"),
						Style("background: transparent;"),
						TBody(
							Tr(Td(Strong(Text("Operating System:"))), Td(Text(runtime.GOOS))),
							Tr(Td(Strong(Text("Architecture:"))), Td(Text(runtime.GOARCH))),
							Tr(Td(Strong(Text("CPU Count:"))), Td(Text(fmt.Sprintf("%d", runtime.NumCPU())))),
							Tr(Td(Strong(Text("Goroutines:"))), Td(Text(fmt.Sprintf("%d", runtime.NumGoroutine())))),
							Tr(Td(Strong(Text("Go Version:"))), Td(Text(runtime.Version()))),
						),
					),
				),
			),

			// Memory Statistics
			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				Div(
					Class("card-header bg-transparent border-0 pb-0"),
					Style("background: transparent !important;"),
					H6(Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"), Text("Memory Statistics")),
				),
				Div(
					Class("card-body pt-2"),
					Table(
						Class("table table-hover table-sm mb-0"),
						Style("background: transparent;"),
						TBody(
							Tr(Td(Strong(Text("Allocated Memory:"))), Td(Text(formatBytes(mem.Alloc)))),
							Tr(Td(Strong(Text("Total Allocated:"))), Td(Text(formatBytes(mem.TotalAlloc)))),
							Tr(Td(Strong(Text("System Memory:"))), Td(Text(formatBytes(mem.Sys)))),
							Tr(Td(Strong(Text("Lookups:"))), Td(Text(fmt.Sprintf("%d", mem.Lookups)))),
							Tr(Td(Strong(Text("Malloc Count:"))), Td(Text(fmt.Sprintf("%d", mem.Mallocs)))),
							Tr(Td(Strong(Text("Free Count:"))), Td(Text(fmt.Sprintf("%d", mem.Frees)))),
							Tr(Td(Strong(Text("Heap Allocated:"))), Td(Text(formatBytes(mem.HeapAlloc)))),
							Tr(Td(Strong(Text("Heap System:"))), Td(Text(formatBytes(mem.HeapSys)))),
							Tr(Td(Strong(Text("Heap Objects:"))), Td(Text(fmt.Sprintf("%d", mem.HeapObjects)))),
						),
					),
				),
			),

			// Garbage Collection Statistics
			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				Div(
					Class("card-header bg-transparent border-0 pb-0"),
					Style("background: transparent !important;"),
					H6(Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"), Text("Garbage Collection Statistics")),
				),
				Div(
					Class("card-body pt-2"),
					Table(
						Class("table table-hover table-sm mb-0"),
						Style("background: transparent;"),
						TBody(
							Tr(Td(Strong(Text("GC Cycles:"))), Td(Text(fmt.Sprintf("%d", gc.NumGC)))),
							Tr(Td(Strong(Text("Last GC:"))), Td(Text(gc.LastGC.Format("2006-01-02 15:04:05")))),
							Tr(Td(Strong(Text("Total Pause:"))), Td(Text(gc.PauseTotal.String()))),
							Tr(Td(Strong(Text("Average Pause:"))), Td(Text(func() string {
								if gc.NumGC > 0 {
									return (gc.PauseTotal / time.Duration(gc.NumGC)).String()
								}
								return "0"
							}()))),
						),
					),
				),
			),

			// Worker Statistics
			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(13, 110, 253, 0.05); border-radius: 8px;"),
				Div(
					Class("card-header bg-transparent border-0 pb-0"),
					Style("background: transparent !important;"),
					H6(Style("color: #0d6efd; font-weight: 600; margin-bottom: 0.5rem;"), Text("Worker Statistics")),
				),
				Div(
					Class("card-body pt-2"),
					Table(
						Class("table table-hover table-sm mb-0"),
						Style("background: transparent;"),
						THead(
							Tr(
								Th(Text("Worker Pool")),
								Th(Text("Submitted")),
								Th(Text("Completed")),
								Th(Text("Successful")),
								Th(Text("Failed")),
								Th(Text("Dropped")),
								Th(Text("Waiting")),
								Th(Text("Running")),
							),
						),
						TBody(
							Tr(
								Td(Strong(Text("Parse"))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerParse.SubmittedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerParse.CompletedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerParse.SuccessfulTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerParse.FailedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerParse.DroppedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerParse.WaitingTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerParse.RunningWorkers))),
							),
							Tr(
								Td(Strong(Text("Search"))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerSearch.SubmittedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerSearch.CompletedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerSearch.SuccessfulTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerSearch.FailedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerSearch.DroppedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerSearch.WaitingTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerSearch.RunningWorkers))),
							),
							Tr(
								Td(Strong(Text("RSS"))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerRSS.SubmittedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerRSS.CompletedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerRSS.SuccessfulTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerRSS.FailedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerRSS.DroppedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerRSS.WaitingTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerRSS.RunningWorkers))),
							),
							Tr(
								Td(Strong(Text("Files"))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerFiles.SubmittedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerFiles.CompletedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerFiles.SuccessfulTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerFiles.FailedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerFiles.DroppedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerFiles.WaitingTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerFiles.RunningWorkers))),
							),
							Tr(
								Td(Strong(Text("Meta"))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerMeta.SubmittedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerMeta.CompletedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerMeta.SuccessfulTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerMeta.FailedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerMeta.DroppedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerMeta.WaitingTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerMeta.RunningWorkers))),
							),
							Tr(
								Td(Strong(Text("Index"))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndex.SubmittedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndex.CompletedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndex.SuccessfulTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndex.FailedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndex.DroppedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndex.WaitingTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndex.RunningWorkers))),
							),
							Tr(
								Td(Strong(Text("Index RSS"))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndexRSS.SubmittedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndexRSS.CompletedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndexRSS.SuccessfulTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndexRSS.FailedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndexRSS.DroppedTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndexRSS.WaitingTasks))),
								Td(Text(fmt.Sprintf("%d", workerStats.WorkerIndexRSS.RunningWorkers))),
							),
						),
					),
				),
			),

			func() Node {
				if action == "gc" {
					return Div(
						Class("mt-3 card border-0 shadow-sm border-success mb-4"),
						Div(
							Class("card-header border-0"),
							Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
							Div(
								Class("d-flex align-items-center"),
								Span(Class("badge bg-success me-3"), I(Class("fas fa-check-circle me-1")), Text("Success")),
								H5(Class("card-title mb-0 text-success fw-bold"), Text("Garbage Collection Complete")),
							),
						),
						Div(
							Class("card-body"),
							P(Class("card-text text-muted mb-0"), Text("Garbage collection completed and memory freed.")),
						),
					)
				}
				return Text("")
			}(),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// formatBytes formats byte count as human readable string
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

// renderDatabaseMaintenancePage renders a page for database maintenance operations
func renderDatabaseMaintenancePage(csrfToken string) Node {
	// Get available table names for clear operations
	tableNames := []string{
		"movies", "dbmovies", "dbmovie_titles",
		"series", "dbseries", "dbserie_episodes", "dbserie_alternates",
		"movie_files", "movie_histories", "movie_file_unmatcheds",
		"serie_episodes", "serie_episode_files", "serie_episode_histories", "serie_file_unmatcheds",
		"qualities", "job_histories", "r_sshistories", "indexer_fails",
	}

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-tools header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Database Maintenance")),
					P(Class("header-subtitle"), Text("Perform various database maintenance operations including integrity checks, backups, cleanup, and optimization tasks.")),
				),
			),
		),

		Div(
			Class("row"),
			Div(
				Class("col-md-6"),
				H5(Class("form-section-title"), Text("Maintenance Operations")),

				Div(
					Class("btn-group-vertical d-block"),
					Button(
						Class("btn btn-info mb-2"),
						Text("Check Database Integrity"),
						Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"integrity\"}"),
					),
					Button(
						Class("btn btn-success mb-2"),
						Text("Create Database Backup"),
						Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"backup\"}"),
					),
					Button(
						Class("btn btn-primary mb-2"),
						Text("Vacuum Database"),
						Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"vacuum\"}"),
					),
					Button(
						Class("btn btn-info mb-2"),
						Text("Fill IMDB Data"),
						Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"fillimdb\"}"),
					),
				),

				H6(Text("Clear Cache & Jobs")),
				Div(
					Class("btn-group-vertical d-block"),
					Button(
						Class("btn btn-warning mb-2"),
						Text("Clear Cache"),
						Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"clearcache\"}"),
					),
					Details(
						Summary(Text("Remove Old Jobs (with days parameter)")),
						Form(
							Class("mt-2"),
							ID("oldJobsForm"),
							Div(
								Class("form-group"),
								Label(Text("Days to keep:")),
								Input(
									Type("number"),
									Class("form-control"),
									Name("days"),
									Value("30"),
									Min("1"),
									Max("365"),
								),
							),
							Button(
								Class("btn btn-warning"),
								Text("Remove Old Jobs"),
								Type("button"),
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

			Div(
				Class("col-md-6"),
				H5(Class("form-section-title"), Text("Table Operations")),

				Details(
					Summary(Text("Clear Specific Table")),
					Form(
						Class("mt-2"),
						ID("clearTableForm"),
						Div(
							Class("form-group"),
							Label(Text("Select table to clear:")),
							Select(
								Class("form-control"),
								Name("tableName"),
								Group(func() []Node {
									var options []Node
									for _, table := range tableNames {
										options = append(options, Option(Value(table), Text(table)))
									}
									return options
								}()),
							),
						),
						Button(
							Class("btn btn-danger"),
							Text("Clear Table"),
							Type("button"),
							hx.Target("#maintenanceResults"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/dbmaintenance"),
							hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
							hx.Include("#clearTableForm"),
							hx.Vals("{\"action\": \"clear\"}"),
							Attr("onclick", "return confirm('Are you sure? This will delete all data from the selected table!')"),
						),
					),
				),

				Details(
					Summary(Text("Delete Specific Record")),
					Form(
						Class("mt-2"),
						ID("deleteRecordForm"),
						Div(
							Class("form-group"),
							Label(Text("Table name:")),
							Select(
								Class("form-control"),
								Name("tableName"),
								Group(func() []Node {
									var options []Node
									for _, table := range tableNames {
										options = append(options, Option(Value(table), Text(table)))
									}
									return options
								}()),
							),
						),
						Div(
							Class("form-group mt-2"),
							Label(Text("Record ID:")),
							Input(
								Type("number"),
								Class("form-control"),
								Name("recordID"),
								Min("1"),
								Required(),
							),
						),
						Button(
							Class("btn btn-danger"),
							Text("Delete Record"),
							Type("button"),
							hx.Target("#maintenanceResults"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/dbmaintenance"),
							hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
							hx.Include("#deleteRecordForm"),
							hx.Vals("{\"action\": \"delete\"}"),
							Attr("onclick", "return confirm('Are you sure? This will permanently delete the selected record!')"),
						),
					),
				),
			),
		),

		Div(
			ID("maintenanceResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),

		// Instructions
		Div(
			Class("mt-4 card border-0 shadow-sm border-info"),
			Div(
				Class("card-body"),
				H5(Class("card-title fw-bold mb-3"), Text("Operation Descriptions")),
				P(Class("card-text text-muted mb-3"), Text("Database maintenance and management operations")),
				Ul(Class("list-unstyled"),
					Li(Class("mb-2"),
						Span(Class("badge bg-success me-2"), I(Class("fas fa-check me-1")), Text("Integrity")),
						Text("Verifies database consistency and reports any corruption"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-primary me-2"), I(Class("fas fa-save me-1")), Text("Backup")),
						Text("Creates a backup of the current database"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-info me-2"), I(Class("fas fa-compress me-1")), Text("Vacuum")),
						Text("Optimizes database by reclaiming space and defragmenting"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-secondary me-2"), I(Class("fas fa-film me-1")), Text("Fill IMDB")),
						Text("Populates IMDB data for movies and series"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-warning me-2"), I(Class("fas fa-broom me-1")), Text("Clear Cache")),
						Text("Removes cached data to free memory"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-dark me-2"), I(Class("fas fa-history me-1")), Text("Remove Jobs")),
						Text("Cleans up job history older than specified days"),
					),
					Li(Class("mb-2"),
						Span(Class("badge bg-danger me-2"), I(Class("fas fa-exclamation-triangle me-1")), Text("Clear Table")),
						Text("⚠️ Removes all data from the selected table"),
					),
					Li(Class("mb-0"),
						Span(Class("badge bg-danger me-2"), I(Class("fas fa-trash me-1")), Text("Delete Record")),
						Text("⚠️ Removes a specific record by ID"),
					),
				),
			),
		),
	)
}

// HandleDatabaseMaintenance handles database maintenance requests
func HandleDatabaseMaintenance(c *gin.Context) {
	action := c.PostForm("action")
	if action == "" {
		// Try to get from JSON body
		var reqData map[string]string
		if err := c.ShouldBindJSON(&reqData); err == nil {
			action = reqData["action"]
		}
	}

	var result Node
	var message, alertClass string

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

	result = Div(
		Class("card border-0 shadow-sm border-"+alertClass),
		Div(
			Class("card-body"),
			H5(Class("card-title fw-bold mb-3"), Text("Maintenance Operation Result")),
			Div(Class("mb-3"),
				P(Class("card-text"), Text(message)),
			),
			Div(Class("d-flex align-items-center text-muted"),
				I(Class("fas fa-clock me-2")),
				Small(Text("Operation completed at: "+time.Now().Format("2006-01-02 15:04:05"))),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// Database maintenance helper functions that call actual API functions
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
	return fmt.Sprintf("✅ Old job entries older than %d days removed successfully.", daysInt), "success"
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
		return fmt.Sprintf("❌ Failed to delete record %s from table '%s': %s", recordID, tableName, err.Error()), "danger"
	}
	return fmt.Sprintf("✅ Record %s deleted from table '%s' successfully.", recordID, tableName), "success"
}

// ================================================================================
// PUSHOVER TEST PAGE
// ================================================================================

// renderPushoverTestPage renders a page for testing Pushover notifications
func renderPushoverTestPage(csrfToken string) Node {
	// Get available notification configurations
	notifications := config.GetSettingsNotificationAll()
	var notificationConfigs []string
	for i := range notifications {
		if notifications[i].NotificationType == "pushover" {
			notificationConfigs = append(notificationConfigs, notifications[i].Name)
		}
	}

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-paper-plane header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Pushover Test Message")),
					P(Class("header-subtitle"), Text("Send a test message through Pushover to verify your notification configuration is working correctly.")),
				),
			),
		),

		Form(
			Class("config-form"),
			ID("pushoverForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Message Configuration")),

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

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Pushover Information")),
					P(Text("Pushover is a service that sends real-time notifications to your devices. To use this feature:")),
					Ol(
						Li(Text("Create a Pushover account at pushover.net")),
						Li(Text("Get your User Key from your dashboard")),
						Li(Text("Create an application to get an API Token")),
						Li(Text("Configure these in your notification settings")),
					),
					P(
						Class("mt-3"),
						Strong(Text("Note: ")),
						Text("Make sure you have at least one Pushover notification configuration set up before testing."),
					),
				),
			),

			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Send Test Message"),
					Type("button"),
					hx.Target("#pushoverResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/pushovertest"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#pushoverForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('pushoverForm').reset(); document.getElementById('pushoverResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("pushoverResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// HandlePushoverTest handles Pushover test message requests
func HandlePushoverTest(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	notificationConfig := c.PostForm("pushover_NotificationConfig")
	messageTitle := c.PostForm("pushover_MessageTitle")
	messageText := c.PostForm("pushover_MessageText")

	if notificationConfig == "" {
		c.String(http.StatusOK, renderAlert("Please select a notification configuration", "warning"))
		return
	}

	if messageTitle == "" || messageText == "" {
		c.String(http.StatusOK, renderAlert("Please provide both message title and text", "warning"))
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
		notifCfg.Apikey,
		messageText,
		messageTitle,
		notifCfg.Recipient,
	)

	var result Node
	if err != nil {
		result = Div(
			Class("card border-0 shadow-sm border-danger mb-4"),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-danger me-3"), I(Class("fas fa-times-circle me-1")), Text("Failed")),
					H5(Class("card-title mb-0 text-danger fw-bold"), Text("Message Send Failed")),
				),
			),

			Div(
				Class("card border-0"),
				Style("background: rgba(220, 53, 69, 0.05); border-radius: 8px;"),
				Div(
					Class("card-body p-3 text-center"),
					I(Class("fas fa-exclamation-triangle mb-2"), Style("color: #dc3545; font-size: 2rem;")),
					P(Class("mb-2"), Style("color: #495057;"), Text("Failed to send Pushover message:")),
					P(Class("mb-2"), Style("color: #dc3545; font-family: monospace; background: rgba(220, 53, 69, 0.1); padding: 0.5rem; border-radius: 4px;"), Text(err.Error())),
					P(Class("mb-0"), Style("color: #495057;"), Text("Please check your notification configuration and try again.")),
				),
			),
		)
	} else {
		result = Div(
			Class("card border-0 shadow-sm border-success mb-4"),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-success me-3"), I(Class("fas fa-paper-plane me-1")), Text("Success")),
					H5(Class("card-title mb-0 text-success fw-bold"), Text("Message Sent Successfully")),
				),
			),

			Div(
				Class("card border-0 mb-3"),
				Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				Div(
					Class("card-body p-3"),
					P(Class("mb-3 text-center"), Style("color: #495057;"), Text("Test message sent via Pushover with the following details:")),

					Div(
						Class("row g-2"),
						Div(
							Class("col-sm-6"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2"),
									Div(Class("d-flex align-items-center"),
										I(Class("fas fa-cogs me-2"), Style("color: #28a745;")),
										Div(
											Div(Class("small fw-bold text-muted"), Text("Configuration")),
											Div(Class("fw-semibold"), Text(notificationConfig)),
										),
									),
								),
							),
						),
						Div(
							Class("col-sm-6"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2"),
									Div(Class("d-flex align-items-center"),
										I(Class("fas fa-user me-2"), Style("color: #28a745;")),
										Div(
											Div(Class("small fw-bold text-muted"), Text("Recipient")),
											Div(Class("fw-semibold"), Text(notifCfg.Recipient)),
										),
									),
								),
							),
						),
						Div(
							Class("col-12"),
							Div(
								Class("card border-0"),
								Style("background: rgba(40, 167, 69, 0.1); border-radius: 6px;"),
								Div(
									Class("card-body p-2"),
									Div(Class("d-flex align-items-start"),
										I(Class("fas fa-envelope me-2 mt-1"), Style("color: #28a745;")),
										Div(Class("flex-grow-1"),
											Div(Class("small fw-bold text-muted"), Text("Message Details")),
											Div(Class("fw-semibold mb-1"), Text("Title: "+messageTitle)),
											Div(Class("text-muted small"), Text(messageText)),
										),
									),
								),
							),
						),
					),
				),
			),

			Div(
				Class("alert alert-light border-0 mb-0"),
				Style("background-color: rgba(40, 167, 69, 0.1); border-radius: 8px; padding: 0.75rem 1rem;"),
				Div(
					Class("d-flex align-items-start"),
					I(Class("fas fa-mobile-alt me-2 mt-1"), Style("color: #28a745; font-size: 0.9rem;")),
					Small(
						Style("color: #495057; line-height: 1.4;"),
						Strong(Text("Next Step: ")),
						Text("Check your device to confirm the message was received."),
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

// renderLogViewerPage renders a page for viewing log files
func renderLogViewerPage(csrfToken string) Node {
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-file-text header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Log Viewer")),
					P(Class("header-subtitle"), Text("Real-time application logs and system events monitoring. Filter by log level and automatically refresh to stay up-to-date with system activity.")),
				),
			),
		),

		Form(
			Class("config-form"),
			ID("logForm"),
			Input(Type("hidden"), Name("csrf_token"), Value(csrfToken)),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Log Configuration")),

					renderFormGroup("log", map[string]string{
						"LineCount": "Number of lines to display from the end of the log file",
					}, map[string]string{
						"LineCount": "Number of Lines",
					}, "LineCount", "number", "100", nil),
				),

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Filter Options")),

					renderFormGroup("log", map[string]string{
						"LogLevel": "Filter logs by level (leave empty for all)",
					}, map[string]string{
						"LogLevel": "Log Level Filter",
					}, "LogLevel", "select", "", map[string][]string{
						"options": {"", "error", "warn", "info", "debug"},
					}),
				),
			),

			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Load Log"),
					Type("button"),
					hx.Target("#logResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/logviewer"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logForm"),
					I(Class("fas fa-sync-alt me-1")),
				),
				Button(
					Class("btn btn-info ml-2"),
					Text("Auto-Refresh"),
					Type("button"),
					hx.Target("#logResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/logviewer"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logForm"),
					hx.Trigger("every 5s"),
					ID("autoRefreshBtn"),
					I(Class("fas fa-play me-1")),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('logForm').reset(); document.getElementById('logResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("logResults"),
			Class("mt-4"),
			Style("min-height: 400px; background: #1e1e1e; color: #f8f8f2; font-family: 'Courier New', monospace; padding: 1rem; overflow-y: auto; max-height: 600px; border-radius: 8px;"),
			P(Class("text-center text-muted"), Text("Click 'Load Log' to view application logs...")),
		),

		// Instructions
		Div(
			Class("mt-4 card border-0 shadow-sm border-info mb-4"),
			Div(
				Class("card-header border-0"),
				Style("background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-info me-3"), I(Class("fas fa-info-circle me-1")), Text("Usage")),
					H5(Class("card-title mb-0 text-info fw-bold"), Text("Log Viewer Information")),
				),
			),
			Div(
				Class("card-body"),
				P(Class("card-text text-muted mb-3"), Text("Real-time log monitoring with filtering capabilities")),
				Ul(
					Class("mb-3 list-unstyled"),
					Li(Class("mb-2"), Text("📂 Logs are read from the 'logs/downloader.log' file")),
					Li(Class("mb-2"), Text("🔄 Lines are displayed in reverse chronological order (newest first)")),
					Li(Class("mb-2"), Text("🎯 Use the log level filter to show only specific types of messages")),
					Li(Class("mb-2"), Text("⚡ Auto-refresh updates the display every 5 seconds")),
					Li(Class("mb-2"), Text("⏱️ Large log files may take a moment to load")),
				),
			),
		),
	)
}

// HandleLogViewer handles log viewing requests
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
	var logNodes []Node
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
			Div(
				Class(class+" font-monospace small"),
				Text(line),
			),
		)
	}

	result := Div(
		Class("card border-0 shadow-sm border-info mb-4"),
		Div(
			Class("card-header border-0"),
			Style("background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;"),
			Div(
				Class("d-flex align-items-center justify-content-between"),
				Div(
					Class("d-flex align-items-center"),
					Span(Class("badge bg-info me-3"), I(Class("fas fa-file-text me-1")), Text("Logs")),
					H5(Class("card-title mb-0 text-info fw-bold"), Text(fmt.Sprintf("Log Entries (Last %d lines)", len(lines)))),
				),
				Span(Class("badge bg-info"), Text(fmt.Sprintf("%s", time.Now().Format("15:04:05")))),
			),
		),
		Div(
			Class("card-body"),
			P(Class("card-text text-muted mb-3"),
				Text(fmt.Sprintf("Loaded at: %s", time.Now().Format("2006-01-02 15:04:05"))),
				func() Node {
					if logLevel != "" {
						return Text(" - Filtered by level: " + logLevel)
					}
					return Text("")
				}(),
			),
			Div(
				Class("log-container"),
				Style("max-height: 400px; overflow-y: auto; background: #f8f9fa; padding: 10px; border-radius: 4px;"),
				Group(logNodes),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// readLastLines reads the last n lines from a file, optionally filtering by log level
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
	if err != nil && err != io.EOF {
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
			if logLevel == "" || strings.Contains(strings.ToLower(line), strings.ToLower(logLevel)) {
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

// renderMediaCleanupWizardPage renders a page for media cleanup operations
func renderMediaCleanupWizardPage(csrfToken string) Node {
	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-broom header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Media Cleanup Wizard")),
					P(Class("header-subtitle"), Text("Find and remove orphaned files, duplicates, or broken links in your media library. Keep your collection clean and organized.")),
				),
			),
		),

		Form(
			Class("config-form"),
			ID("cleanupForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Cleanup Options")),

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

				Div(
					Class("col-md-6"),
					H5(Class("form-section-title"), Text("Scan Configuration")),

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

			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Start Cleanup Scan"),
					Type("button"),
					hx.Target("#cleanupResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/cleanup"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#cleanupForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('cleanupForm').reset(); document.getElementById('cleanupResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("cleanupResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// ================================================================================
// CRON EXPRESSION GENERATOR PAGE
// ================================================================================

// renderCronGeneratorPage renders the cron expression generator page
func renderCronGeneratorPage(csrfToken string) Node {
	return Div(
		Class("container-fluid"),

		// Header
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-clock header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Cron Expression Generator")),
					P(Class("header-subtitle"), Text("Generate and validate cron expressions for scheduling tasks. Choose from presets or build custom expressions with validation and examples.")),
				),
			),
		),

		// Generator Form
		Div(
			Class("row"),

			// Input Form Column
			Div(
				Class("col-lg-6"),
				Div(
					Class("card"),
					Div(
						Class("card-header"),
						H5(Class("card-title mb-0"), Text("Generate Expression")),
					),
					Div(
						Class("card-body"),
						Form(
							ID("cronForm"),
							Input(Type("hidden"), Name("csrf_token"), Value(csrfToken)),

							// Preset Options
							Div(
								Class("form-group mb-3"),
								Label(Class("form-label fw-bold"), Text("Quick Presets")),
								Div(
									Class("btn-group d-flex flex-wrap gap-2"),
									Button(Type("button"), Class("btn btn-outline-primary btn-sm"), Attr("data-cron", "*/30 * * * * *"), Text("Every 30s")),
									Button(Type("button"), Class("btn btn-outline-primary btn-sm"), Attr("data-cron", "0 */5 * * * *"), Text("Every 5min")),
									Button(Type("button"), Class("btn btn-outline-primary btn-sm"), Attr("data-cron", "0 0 * * * *"), Text("Every Hour")),
									Button(Type("button"), Class("btn btn-outline-primary btn-sm"), Attr("data-cron", "0 0 0 * * *"), Text("Daily at Midnight")),
									Button(Type("button"), Class("btn btn-outline-primary btn-sm"), Attr("data-cron", "0 0 12 * * *"), Text("Daily at Noon")),
									Button(Type("button"), Class("btn btn-outline-primary btn-sm"), Attr("data-cron", "0 0 0 * * 0"), Text("Weekly (Sunday)")),
									Button(Type("button"), Class("btn btn-outline-primary btn-sm"), Attr("data-cron", "0 0 0 1 * *"), Text("Monthly")),
								),
							),

							Hr(),

							// Manual Input Fields
							Div(
								Class("row"),
								Div(
									Class("col-md-2"),
									Div(
										Class("form-group"),
										Label(Class("form-label"), Text("Second")),
										Input(
											Type("text"),
											Class("form-control cron-field"),
											ID("second"),
											Placeholder("0-59"),
											Value("0"),
											Attr("data-toggle", "tooltip"),
											Attr("title", "0-59 or * or */5 for every 5 seconds"),
										),
										Small(Class("form-text text-muted"), Text("0-59")),
									),
								),
								Div(
									Class("col-md-2"),
									Div(
										Class("form-group"),
										Label(Class("form-label"), Text("Minute")),
										Input(
											Type("text"),
											Class("form-control cron-field"),
											ID("minute"),
											Placeholder("0-59"),
											Value("0"),
											Attr("data-toggle", "tooltip"),
											Attr("title", "0-59 or * or */5 for every 5 minutes"),
										),
										Small(Class("form-text text-muted"), Text("0-59")),
									),
								),
								Div(
									Class("col-md-2"),
									Div(
										Class("form-group"),
										Label(Class("form-label"), Text("Hour")),
										Input(
											Type("text"),
											Class("form-control cron-field"),
											ID("hour"),
											Placeholder("0-23"),
											Value("*"),
											Attr("data-toggle", "tooltip"),
											Attr("title", "0-23 or * or */2 for every 2 hours"),
										),
										Small(Class("form-text text-muted"), Text("0-23")),
									),
								),
								Div(
									Class("col-md-2"),
									Div(
										Class("form-group"),
										Label(Class("form-label"), Text("Day")),
										Input(
											Type("text"),
											Class("form-control cron-field"),
											ID("day"),
											Placeholder("1-31"),
											Value("*"),
											Attr("data-toggle", "tooltip"),
											Attr("title", "1-31 or * or L for last day"),
										),
										Small(Class("form-text text-muted"), Text("1-31")),
									),
								),
								Div(
									Class("col-md-2"),
									Div(
										Class("form-group"),
										Label(Class("form-label"), Text("Month")),
										Input(
											Type("text"),
											Class("form-control cron-field"),
											ID("month"),
											Placeholder("1-12"),
											Value("*"),
											Attr("data-toggle", "tooltip"),
											Attr("title", "1-12 or JAN-DEC or *"),
										),
										Small(Class("form-text text-muted"), Text("1-12")),
									),
								),
								Div(
									Class("col-md-2"),
									Div(
										Class("form-group"),
										Label(Class("form-label"), Text("Weekday")),
										Input(
											Type("text"),
											Class("form-control cron-field"),
											ID("weekday"),
											Placeholder("0-6"),
											Value("*"),
											Attr("data-toggle", "tooltip"),
											Attr("title", "0-6 (0=Sunday) or SUN-SAT or *"),
										),
										Small(Class("form-text text-muted"), Text("0-6")),
									),
								),
							),

							Hr(),

							// Manual Expression Input
							Div(
								Class("form-group"),
								Label(Class("form-label fw-bold"), Text("Manual Expression")),
								Div(
									Class("input-group"),
									Input(
										Type("text"),
										Class("form-control font-monospace"),
										ID("cronExpression"),
										Placeholder("0 12 * * 1"),
										Attr("data-toggle", "tooltip"),
										Attr("title", "Enter a complete cron expression"),
										hx.Post("/api/admin/crongen/validate"),
										hx.Target("#cronResult"),
										hx.Swap("innerHTML"),
										hx.Trigger("input changed delay:500ms"),
										hx.Headers(createHTMXHeaders(csrfToken)),
									),
									Button(
										Class("btn btn-primary"),
										Type("button"),
										Attr("onclick", "htmx.trigger('#cronExpression', 'input')"),
										Text("Validate"),
									),
								),
							),
						),
					),
				),
			),

			// Result Column
			Div(
				Class("col-lg-6"),
				Div(
					Class("card"),
					Div(
						Class("card-header"),
						H5(Class("card-title mb-0"), Text("Expression Details")),
					),
					Div(
						Class("card-body"),
						Div(
							ID("cronResult"),
							Class("p-3 bg-light rounded"),
							P(Class("text-muted mb-0"), Text("Enter a cron expression to see its details...")),
						),
					),
				),

				// Help Card
				Div(
					Class("card mt-4"),
					Div(
						Class("card-header"),
						H5(Class("card-title mb-0"), Text("Cron Format Help")),
					),
					Div(
						Class("card-body"),
						Table(
							Class("table table-sm"),
							THead(
								Tr(
									Th(Text("Field")),
									Th(Text("Values")),
									Th(Text("Special Characters")),
								),
							),
							TBody(
								Tr(Td(Text("Second")), Td(Text("0-59")), Td(Text("* , - /"))),
								Tr(Td(Text("Minute")), Td(Text("0-59")), Td(Text("* , - /"))),
								Tr(Td(Text("Hour")), Td(Text("0-23")), Td(Text("* , - /"))),
								Tr(Td(Text("Day")), Td(Text("1-31")), Td(Text("* , - / L"))),
								Tr(Td(Text("Month")), Td(Text("1-12 or JAN-DEC")), Td(Text("* , - /"))),
								Tr(Td(Text("Weekday")), Td(Text("0-7 or SUN-SAT")), Td(Text("* , - / L #"))),
							),
						),
						Div(
							Class("mt-3"),
							H6(Text("Examples:")),
							Ul(
								Class("list-unstyled small text-muted"),
								Li(Code(Text("0 0 12 * * *")), Text(" - Daily at 12:00 PM (6-field)")),
								Li(Code(Text("0 12 * * *")), Text(" - Daily at 12:00 PM (5-field)")),
								Li(Code(Text("*/30 * * * * *")), Text(" - Every 30 seconds")),
								Li(Code(Text("0 */15 * * * *")), Text(" - Every 15 minutes")),
								Li(Code(Text("0 0 9-17 * * 1-5")), Text(" - Every hour from 9-5, Mon-Fri")),
							),
						),
					),
				),
			),
		),

		// Complete JavaScript for Cron Generator
		Script(Raw(`
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

func renderQualityReorderPage(csrfToken string) Node {
	// Get all available quality profiles
	qualityConfigs := config.GetSettingsQualityAll()

	var qualityOptions []Node
	qualityOptions = append(qualityOptions, Option(Value(""), Text("-- Select Quality Profile --")))
	for _, qc := range qualityConfigs {
		qualityOptions = append(qualityOptions, Option(Value(qc.Name), Text(qc.Name)))
	}

	// Get all resolutions and qualities from database for filter options
	allResolutions := database.DBConnect.GetresolutionsIn
	allQualities := database.DBConnect.GetqualitiesIn

	var resolutionOptions []Node
	resolutionOptions = append(resolutionOptions, Option(Value("all"), Text("All Resolutions")))
	for _, res := range allResolutions {
		if res.Name != "" {
			resolutionOptions = append(resolutionOptions, Option(Value(res.Name), Text(res.Name)))
		}
	}

	var qualityFilterOptions []Node
	qualityFilterOptions = append(qualityFilterOptions, Option(Value("all"), Text("All Qualities")))
	for _, qual := range allQualities {
		if qual.Name != "" {
			qualityFilterOptions = append(qualityFilterOptions, Option(Value(qual.Name), Text(qual.Name)))
		}
	}

	return Div(
		Class("config-section-enhanced"),
		// Enhanced page header
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fas fa-sort-amount-down header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Quality Reorder Testing")),
					P(Class("header-subtitle"), Text("Experiment with quality settings and preview the resulting order and priority")),
				),
			),
		),

		// Main form
		Div(
			Class("row"),
			Div(
				Class("col-12"),
				Div(
					Class("card border-0 shadow-sm"),
					Style("border-radius: 15px; overflow: hidden;"),
					Div(
						Class("card-header border-0"),
						Style("background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;"),
						H5(Class("card-title mb-0"),
							I(Class("fas fa-cog me-2 text-primary")),
							Text("Quality Profile Selection"),
						),
					),
					Div(
						Class("card-body p-4"),
						Form(
							ID("quality-reorder-form"),
							hx.Post("/api/admin/quality-reorder"),
							hx.Target("#quality-results"),
							hx.Swap("innerHTML"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							Input(Type("hidden"), Name("csrf_token"), Value(csrfToken)),

							Div(
								Class("mb-4"),
								Label(Class("form-label fw-semibold"), Text("Quality Profile")),
								Select(
									Class("form-select"),
									Name("selected_quality"),
									ID("selected_quality"),
									Required(),
									Attr("hx-get", "/api/admin/quality-reorder/rules"),
									Attr("hx-target", "#reorder-rules-container"),
									Attr("hx-trigger", "change"),
									Attr("hx-vals", "js:{profile: this.value}"),
									Attr("hx-headers", `{"X-CSRF-Token": "`+csrfToken+`"}`),
									Attr("hx-indicator", "#loading-indicator"),
									Group(qualityOptions),
								),
								Div(Class("form-text"), Text("Select a quality profile to view its current ordering")),
							),

							Div(
								Class("row mb-4"),
								Div(
									Class("col-md-6"),
									Label(Class("form-label fw-semibold"), Text("Filter by Resolution")),
									Select(
										Class("form-select"),
										Name("filter_resolution"),
										ID("filter_resolution"),
										Group(resolutionOptions),
									),
									Div(Class("form-text"), Text("Filter results to specific resolution")),
								),
								Div(
									Class("col-md-6"),
									Label(Class("form-label fw-semibold"), Text("Filter by Quality")),
									Select(
										Class("form-select"),
										Name("filter_quality"),
										ID("filter_quality"),
										Group(qualityFilterOptions),
									),
									Div(Class("form-text"), Text("Filter results to specific quality source")),
								),
							),

							// Additional Filter Options Row
							Div(
								Class("row mb-4"),
								Div(
									Class("col-12"),
									Div(
										Class("card border-0 shadow-sm"),
										Style("border-radius: 10px;"),
										Div(
											Class("card-header border-0"),
											Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 1rem;"),
											H6(Class("card-title mb-0"),
												I(Class("fas fa-filter me-2 text-info")),
												Text("Filter Options"),
											),
										),
										Div(
											Class("card-body p-3"),
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
							Div(
								Class("mb-4"),
								Div(
									Class("d-flex justify-content-between align-items-center mb-3"),
									H6(Class("mb-0 fw-semibold"),
										I(Class("fas fa-flask me-2 text-warning")),
										Text("Temporary Reorder Rules"),
									),
									Div(
										Class("btn-group"),
										Button(
											Type("button"),
											Class("btn btn-sm btn-outline-primary"),
											hx.Post("/api/admin/quality-reorder/add-rule"),
											hx.Target("#reorder-rules-container"),
											hx.Swap("beforeend"),
											hx.Headers(createHTMXHeaders(csrfToken)),
											I(Class("fas fa-plus me-1")),
											Text("Add Rule"),
										),
										Button(
											Type("button"),
											Class("btn btn-sm btn-outline-secondary"),
											hx.Post("/api/admin/quality-reorder/reset-rules"),
											hx.Target("#reorder-rules-container"),
											hx.Swap("innerHTML"),
											hx.Headers(createHTMXHeaders(csrfToken)),
											hx.Confirm("Are you sure you want to reset all temporary reorder rules?"),
											I(Class("fas fa-undo me-1")),
											Text("Reset"),
										),
									),
								),
								// Rules headers
								Div(
									Class("row mb-2 px-3"),
									Div(Class("col-md-2"), Small(Class("text-muted fw-semibold"), Text("TYPE"))),
									Div(Class("col-md-3"), Small(Class("text-muted fw-semibold"), Text("PATTERN"))),
									Div(Class("col-md-2"), Small(Class("text-muted fw-semibold"), Text("PRIORITY (+/-)"))),
									Div(Class("col-md-3"), Small(Class("text-muted fw-semibold"), Text("ENABLED"))),
									Div(Class("col-md-2"), Small(Class("text-muted fw-semibold"), Text("ACTIONS"))),
								),
								Div(
									ID("reorder-rules-container"),
									Class("border rounded p-3 bg-light"),
									P(Class("text-muted mb-0 text-center"), Text("No temporary reorder rules. Select a quality profile above to load rules.")),
								),
								Div(
									ID("loading-indicator"),
									Class("htmx-indicator text-center p-2"),
									I(Class("fas fa-spinner fa-spin me-2")),
									Text("Loading rules..."),
								),
								Input(Type("hidden"), Name("reorder_rules"), ID("reorder_rules_json"), Value("[]")),
								Div(Class("form-text mt-2"),
									Text("Add temporary reorder rules to test priority changes without modifying the actual configuration. Changes are not saved permanently. Priorities can be negative to reduce ranking."),
								),
							),

							Div(
								Class("d-flex gap-2"),
								Button(
									Type("submit"),
									Class("btn btn-primary px-4"),
									hx.Post("/api/admin/quality-reorder"),
									hx.Target("#quality-results"),
									hx.Swap("innerHTML"),
									hx.Headers(createHTMXHeaders(csrfToken)),
									hx.Include("#quality-reorder-form"),
									hx.Indicator("#loading-indicator"),
									I(Class("fas fa-eye me-2")),
									Text("Preview Quality Order"),
								),
								Button(
									Type("button"),
									Class("btn btn-outline-secondary px-4"),
									ID("refresh-profiles"),
									I(Class("fas fa-sync-alt me-2")),
									Text("Refresh Profiles"),
								),
							),
						),
					),
				),
			),
		),

		// Results container
		Div(
			ID("quality-results"),
			Class("mt-4"),
		),

		// Instructions card
		Div(
			Class("row mt-4"),
			Div(
				Class("col-12"),
				Div(
					Class("card border-0 shadow-sm bg-light"),
					Div(
						Class("card-body p-4"),
						H6(Class("card-title text-muted mb-3"),
							I(Class("fas fa-info-circle me-2")),
							Text("How to Use This Tool"),
						),
						Ul(
							Class("list-unstyled mb-0 text-muted small"),
							Li(Class("mb-2"),
								I(Class("fas fa-check text-success me-2")),
								Text("Select a quality profile from the dropdown to view its current ordering"),
							),
							Li(Class("mb-2"),
								I(Class("fas fa-check text-success me-2")),
								Text("Use filters to focus on specific resolutions or quality sources"),
							),
							Li(Class("mb-2"),
								I(Class("fas fa-flask text-warning me-2")),
								Text("Add temporary reorder rules to test priority changes (positive values increase ranking, negative decrease)"),
							),
							Li(Class("mb-2"),
								I(Class("fas fa-check text-success me-2")),
								Text("Enable/disable rules to see how they affect priority calculations"),
							),
							Li(Class("mb-0"),
								I(Class("fas fa-check text-success me-2")),
								Text("The results table shows resolution + quality combinations ranked by total priority"),
							),
						),
					),
				),
			),
		),

		// Complete JavaScript for Quality Reorder
		Script(Raw(`
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
