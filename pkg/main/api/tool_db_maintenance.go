package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

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
					html.Button(
						html.Class("btn btn-info mb-2"),
						gomponents.Text("Preview Orphaned Rows"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"cleanuporphans\"}"),
					),
					html.Button(
						html.Class("btn btn-danger mb-2"),
						gomponents.Text("Delete Orphaned Rows"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"cleanuporphans_delete\"}"),
						hx.Confirm("Delete all database rows whose parent row no longer exists?"),
					),
					html.Button(
						html.Class("btn btn-info mb-2"),
						gomponents.Text("Preview Rootpath Repair (Music/Audiobooks/Books)"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"repairaudiorootpaths\"}"),
					),
					html.Button(
						html.Class("btn btn-danger mb-2"),
						gomponents.Text("Apply Rootpath Repair"),
						html.Type("button"),
						hx.Target("#maintenanceResults"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/dbmaintenance"),
						hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
						hx.Vals("{\"action\": \"repairaudiorootpaths_apply\"}"),
						hx.Confirm("Set each album/audiobook/book rootpath to the folder its files are in? Back up the database first."),
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
									options := make([]gomponents.Node, 0, len(tableNames))
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
							hx.Confirm(
								"This will delete ALL data from the selected table. This cannot be undone.",
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
									options := make([]gomponents.Node, 0, len(tableNames))
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
							hx.Confirm(
								"This will permanently delete the selected record. This cannot be undone.",
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
	case "cleanuporphans":
		message, alertClass = performCleanupOrphans(true)
	case "cleanuporphans_delete":
		message, alertClass = performCleanupOrphans(false)
	case "repairaudiorootpaths":
		message, alertClass = performRepairAudioRootpaths(false)
	case "repairaudiorootpaths_apply":
		message, alertClass = performRepairAudioRootpaths(true)
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

// performCleanupOrphans counts (dryRun) or removes child rows whose parent
// rows no longer exist (the schema's ON DELETE CASCADE relationships, applied
// manually since FK enforcement is disabled).
func performCleanupOrphans(dryRun bool) (string, string) {
	found, err := database.CleanupOrphans(dryRun)

	var total int64
	for _, n := range found {
		total += n
	}

	tables := make([]string, 0, len(found))
	for t, n := range found {
		tables = append(tables, fmt.Sprintf("%s: %d", t, n))
	}

	sort.Strings(tables)

	detail := ""
	if len(tables) > 0 {
		detail = " (" + strings.Join(tables, ", ") + ")"
	}

	if err != nil {
		verb := "removed"
		if dryRun {
			verb = "counted"
		}

		return fmt.Sprintf("❌ Orphan cleanup failed: %v. %d orphaned rows %s before the failure%s.",
			err, total, verb, detail), "danger"
	}

	if dryRun {
		if total == 0 {
			return "✅ Preview: no orphaned rows found.", "success"
		}

		return fmt.Sprintf(
			"⚠️ Preview: %d orphaned rows in %d tables%s. Nothing was deleted - use \"Delete Orphaned Rows\" to remove them (deleting can orphan further child rows, which the delete also removes).",
			total, len(found), detail,
		), "warning"
	}

	if total == 0 {
		return "✅ Orphan cleanup completed. No orphaned rows found.", "success"
	}

	return fmt.Sprintf(
		"✅ Orphan cleanup completed. Removed %d orphaned rows across %d tables%s.",
		total, len(found), detail,
	), "success"
}

// performRepairAudioRootpaths previews (apply false) or writes the album /
// audiobook / book rootpaths recomputed from their files' folders.
func performRepairAudioRootpaths(apply bool) (string, string) {
	res := structure.RepairAudioRootpaths(apply)

	perTable := map[string]int{}
	for i := range res.Changes {
		perTable[res.Changes[i].Table]++
	}

	summary := fmt.Sprintf(
		"albums: %d to change (%d with files in several folders, skipped), audiobooks: %d to change (%d skipped), books: %d to change (%d skipped)",
		perTable["albums"], res.Ambiguous["albums"], perTable["audiobooks"], res.Ambiguous["audiobooks"],
		perTable["books"], res.Ambiguous["books"],
	)

	if len(res.Changes) == 0 {
		return "✅ All album/audiobook/book rootpaths match their files' folders - " + summary + ".", "success"
	}

	const maxSamples = 10

	samples := make([]string, 0, maxSamples)
	for i := range res.Changes {
		if i == maxSamples {
			samples = append(samples, "...")
			break
		}

		c := &res.Changes[i]
		samples = append(samples, fmt.Sprintf("%s %d: %q -> %q", c.Table, c.ID, c.Old, c.New))
	}

	if !apply {
		return fmt.Sprintf(
			"⚠️ Preview: %s. Nothing was changed - use \"Apply Rootpath Repair\" to write them. %s",
			summary, strings.Join(samples, "; "),
		), "warning"
	}

	return fmt.Sprintf(
		"✅ Rootpath repair applied - %s. Clear the cache so library scans see the new rootpaths. %s",
		summary, strings.Join(samples, "; "),
	), "success"
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
		// Pause, not Close: Close also stops the interval schedules for good.
		worker.StopCronWorker()
		worker.PauseWorkerPools(context.Background())

		defer func() {
			worker.InitWorkerPools(
				config.GetSettingsGeneral().WorkerSearch,
				config.GetSettingsGeneral().WorkerFiles,
				config.GetSettingsGeneral().WorkerMetadata,
				config.GetSettingsGeneral().WorkerRSS,
				config.GetSettingsGeneral().WorkerIndexer,
				config.GetSettingsGeneral().WorkerParse,
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
	// Jobs["RefreshImdb"] runs FillImdb() synchronously with no goroutine of
	// its own (utils/jobs.go) - calling it directly here blocked the request
	// for the entire fill duration while still claiming "started in
	// background". Dispatch it like every other job in this file
	// (HandleJobManagement) so the response returns immediately.
	fn := config.GetSettingsGeneral().Jobs["RefreshImdb"]

	err := worker.Dispatch("RefreshImdb", func(key uint32, ctx context.Context) error {
		return fn(key, ctx)
	}, "Data")
	if err != nil {
		return "❌ Failed to start IMDB fill: " + err.Error(), "danger"
	}

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

	res, err := database.DeleteRow("job_histories", "created_at < ?", &scantime)
	if err != nil {
		return fmt.Sprintf("❌ Failed to remove old jobs: %s", err.Error()), "danger"
	}

	var removed int64
	if res != nil {
		removed, _ = res.RowsAffected()
	}

	return fmt.Sprintf(
		"✅ Removed %d job entries older than %d days.",
		removed,
		daysInt,
	), "success"
}

func performClearTable(c *gin.Context) (string, string) {
	tableName := c.PostForm("tableName")
	if tableName == "" {
		return "❌ No table name specified.", "danger"
	}

	// tableName is concatenated directly into the DELETE below (a table name
	// can't be bound as a `?` SQL parameter) - the UI only offers a fixed
	// <select> list, but the server must re-validate it: anything reaching
	// this handler with an arbitrary tableName could otherwise clear any
	// table in the schema, not just the intended content tables. Same
	// allowlist check as apiDBClear (rest_general.go).
	if database.GetTableDefaults(tableName).Table == "" {
		return fmt.Sprintf("❌ Unknown table '%s'.", tableName), "danger"
	}

	// Call the actual clear table function (same as apiDBClear)
	err := database.ExecNErr("DELETE FROM " + tableName)
	if err != nil {
		return fmt.Sprintf("❌ Failed to clear table '%s': %s", tableName, err.Error()), "danger"
	}

	// Run vacuum after clearing
	if err := database.ExecNErr("VACUUM"); err != nil {
		return fmt.Sprintf(
			"⚠️ Table '%s' cleared, but VACUUM failed: %s",
			tableName,
			err.Error(),
		), "warning"
	}

	return fmt.Sprintf("✅ Table '%s' cleared successfully.", tableName), "success"
}

func performDeleteRecord(c *gin.Context) (string, string) {
	tableName := c.PostForm("tableName")
	recordID := c.PostForm("recordID")

	if tableName == "" || recordID == "" {
		return "❌ Table name and record ID are required.", "danger"
	}

	// See performClearTable above for why tableName must be checked
	// against the allowlist before being concatenated into the query.
	if database.GetTableDefaults(tableName).Table == "" {
		return fmt.Sprintf("❌ Unknown table '%s'.", tableName), "danger"
	}

	// Call the actual delete record function (same as apiDBDelete)
	err := database.ExecNErr("DELETE FROM "+tableName+" WHERE id = ?", &recordID)
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
