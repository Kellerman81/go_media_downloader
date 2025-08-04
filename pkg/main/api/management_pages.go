package api

import (
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
		Class("config-section"),
		H3(Text("Job Management")),
		P(Text("Start single jobs for media processing, data refreshing, and searching. Jobs will be queued and executed based on your configuration settings.")),

		Form(
			Class("config-form"),
			ID("jobForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-4"),
					H5(Text("Job Configuration")),

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
					H5(Text("Job Options")),

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
					H5(Text("Job Descriptions")),
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
			Class("mt-4 alert alert-info"),
			H5(Text("Usage Instructions:")),
			Ol(
				Li(Text("Select the job type you want to execute")),
				Li(Text("Choose the media configuration (required for most jobs)")),
				Li(Text("Optionally specify a list name for targeted processing")),
				Li(Text("Check 'Force Execution' to run even if scheduler is disabled")),
				Li(Text("Click 'Start Job' to queue the job for execution")),
			),
			P(
				Class("mt-2"),
				Strong(Text("Note: ")),
				Text("Jobs are queued and executed asynchronously. Check the scheduler page for job status and history."),
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
	err := worker.Dispatch(jobType+"_"+mediaConfig, func(key uint32) error {
		return utils.SingleJobs(jobType, mediaConfig, listName, force, key)
	}, "Data")
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to start job: "+err.Error(), "danger"))
		return
	}

	result := Div(
		Class("alert alert-success"),
		H5(Text("Job Started Successfully")),
		P(Text(fmt.Sprintf("Job '%s' has been queued for execution with the following parameters:", jobType))),
		Ul(
			Li(Text("Job Type: "+jobType)),
			Li(Text("Media Config: "+mediaConfig)),
			Li(func() Node {
				if listName != "" {
					return Text("List Name: " + listName)
				}
				return Text("List Name: (all lists)")
			}()),
			Li(Text(fmt.Sprintf("Force Execution: %t", force))),
		),
		P(
			Class("mt-2"),
			Text("The job is now running in the background. You can monitor its progress in the scheduler interface."),
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
		Class("config-section"),
		H3(Text("Debug Statistics")),
		P(Text("View runtime statistics, memory usage, garbage collection info, and worker statistics. This information is useful for monitoring application performance and troubleshooting issues.")),

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
			Class("mt-4 alert alert-info"),
			H5(Text("Debug Information:")),
			Ul(
				Li(Strong(Text("Runtime Stats: ")), Text("Go runtime information including OS, CPU count, and goroutine count")),
				Li(Strong(Text("Memory Stats: ")), Text("Detailed memory usage including heap, stack, and GC statistics")),
				Li(Strong(Text("GC Stats: ")), Text("Garbage collection performance metrics and timing")),
				Li(Strong(Text("Worker Stats: ")), Text("Background job queue and worker pool statistics")),
				Li(Strong(Text("Heap Dump: ")), Text("Memory heap dump is saved to temp/heapdump for analysis")),
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
		Class("alert-info"),
		H5(Text("Debug Statistics")),
		P(Text(fmt.Sprintf("Generated at: %s", time.Now().Format("2006-01-02 15:04:05")))),
		Br(),
		// Runtime Information
		H6(Text("Runtime Information")),
		Table(
			Class("table table-sm table-striped"),
			TBody(
				Tr(Td(Strong(Text("Operating System:"))), Td(Text(runtime.GOOS))),
				Tr(Td(Strong(Text("Architecture:"))), Td(Text(runtime.GOARCH))),
				Tr(Td(Strong(Text("CPU Count:"))), Td(Text(fmt.Sprintf("%d", runtime.NumCPU())))),
				Tr(Td(Strong(Text("Goroutines:"))), Td(Text(fmt.Sprintf("%d", runtime.NumGoroutine())))),
				Tr(Td(Strong(Text("Go Version:"))), Td(Text(runtime.Version()))),
			),
		),
		Br(),

		// Memory Statistics
		H6(Text("Memory Statistics")),
		Table(
			Class("table table-sm table-striped"),
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
		Br(),

		// Garbage Collection Statistics
		H6(Text("Garbage Collection Statistics")),
		Table(
			Class("table table-sm table-striped"),
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
		Br(),

		// Worker Statistics
		H6(Text("Worker Statistics")),
		Table(
			Class("table table-sm table-striped"),
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

		func() Node {
			if action == "gc" {
				return Div(
					Class("alert alert-success mt-3"),
					Text("Garbage collection completed and memory freed."),
				)
			}
			return Text("")
		}(),
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
		Class("config-section"),
		H3(Text("Database Maintenance")),
		P(Text("Perform various database maintenance operations including integrity checks, backups, cleanup, and optimization tasks.")),

		Div(
			Class("row"),
			Div(
				Class("col-md-6"),
				H5(Text("Maintenance Operations")),

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
				H5(Text("Table Operations")),

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
			Class("mt-4 alert alert-info"),
			H5(Text("Operation Descriptions:")),
			Ul(
				Li(Strong(Text("Integrity Check: ")), Text("Verifies database consistency and reports any corruption")),
				Li(Strong(Text("Backup: ")), Text("Creates a backup of the current database")),
				Li(Strong(Text("Vacuum: ")), Text("Optimizes database by reclaiming space and defragmenting")),
				Li(Strong(Text("Fill IMDB: ")), Text("Populates IMDB data for movies and series")),
				Li(Strong(Text("Clear Cache: ")), Text("Removes cached data to free memory")),
				Li(Strong(Text("Remove Old Jobs: ")), Text("Cleans up job history older than specified days")),
				Li(Strong(Text("Clear Table: ")), Text("⚠️ Removes all data from the selected table")),
				Li(Strong(Text("Delete Record: ")), Text("⚠️ Removes a specific record by ID")),
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
		Class("alert alert-"+alertClass),
		H5(Text("Maintenance Operation Result")),
		P(Text(message)),
		P(
			Class("text-muted"),
			Small(Text("Operation completed at: "+time.Now().Format("2006-01-02 15:04:05"))),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// Database maintenance helper functions that call actual API functions
func performIntegrityCheck(_ *gin.Context) (string, string) {
	// Call the actual integrity check function
	results := database.DBIntegrityCheck()
	if len(results) == 0 {
		return "✅ Database integrity check completed. No issues found.", "success"
	}

	var issues []string
	for _, result := range results {
		issues = append(issues, fmt.Sprintf("%v", result))
	}
	return fmt.Sprintf("⚠️ Database integrity issues found: %s", strings.Join(issues, ", ")), "warning"
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

func performFillIMDB(_ *gin.Context) (string, string) {
	// Call the actual IMDB fill function
	config.GetSettingsGeneral().Jobs["RefreshImdb"](0)
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
		Class("config-section"),
		H3(Text("Pushover Test Message")),
		P(Text("Send a test message through Pushover to verify your notification configuration is working correctly.")),

		Form(
			Class("config-form"),
			ID("pushoverForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Text("Message Configuration")),

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
					H5(Text("Pushover Information")),
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
			Class("alert alert-danger"),
			H5(Text("Message Send Failed")),
			P(Text("Failed to send Pushover message: "+err.Error())),
			P(Text("Please check your notification configuration and try again.")),
		)
	} else {
		result = Div(
			Class("alert alert-success"),
			H5(Text("Message Sent Successfully")),
			P(Text("Test message sent via Pushover with the following details:")),
			Ul(
				Li(Text("Configuration: "+notificationConfig)),
				Li(Text("Title: "+messageTitle)),
				Li(Text("Message: "+messageText)),
				Li(Text("Recipient: "+notifCfg.Recipient)),
			),
			P(Text("Check your device to confirm the message was received.")),
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
		Class("config-section"),
		H3(Text("Log Viewer")),
		P(Text("View the last entries from the downloader.log file to monitor application activity and troubleshoot issues.")),

		Form(
			Class("config-form"),
			ID("logForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					renderFormGroup("log", map[string]string{
						"LineCount": "Number of lines to display from the end of the log file",
					}, map[string]string{
						"LineCount": "Number of Lines",
					}, "LineCount", "number", "100", nil),
				),
				Div(
					Class("col-md-6"),
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
				),
			),
		),

		Div(
			ID("logResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),

		// Instructions
		Div(
			Class("mt-4 alert alert-info"),
			H5(Text("Log Viewer Information:")),
			Ul(
				Li(Text("Logs are read from the 'logs/downloader.log' file")),
				Li(Text("Lines are displayed in reverse chronological order (newest first)")),
				Li(Text("Use the log level filter to show only specific types of messages")),
				Li(Text("Auto-refresh updates the display every 5 seconds")),
				Li(Text("Large log files may take a moment to load")),
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
		Class("alert-secondary"),
		H5(Text(fmt.Sprintf("Log Entries (Last %d lines)", len(lines)))),
		P(Text(fmt.Sprintf("Loaded at: %s", time.Now().Format("2006-01-02 15:04:05")))),
		func() Node {
			if logLevel != "" {
				return P(Text("Filtered by level: " + logLevel))
			}
			return Text("")
		}(),
		Hr(),
		Div(
			Class("log-container"),
			Style("max-height: 400px; overflow-y: auto; background: #f8f9fa; padding: 10px; border-radius: 4px;"),
			Group(logNodes),
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
