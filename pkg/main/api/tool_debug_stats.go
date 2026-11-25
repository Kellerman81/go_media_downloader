package api

import (
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

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
						html.Class("mb-0"),
						html.Span(
							html.Class("badge bg-warning me-2"),
							html.I(html.Class("fas fa-tasks me-1")),
							gomponents.Text("Workers"),
						),
						gomponents.Text("Background job queue and worker pool statistics"),
					),
				),
			),
		),
	)
}

// HandleDebugStats handles debug statistics requests.
func HandleDebugStats(c *gin.Context) {
	action := c.PostForm("action")

	// Run the requested GC first so the figures below show its effect.
	if action == "gc" {
		debug.FreeOSMemory()
	}

	var gc debug.GCStats
	debug.ReadGCStats(&gc)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

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
							workerStatsRow("Parse", workerStats.WorkerParse),
							workerStatsRow("Search", workerStats.WorkerSearch),
							workerStatsRow("RSS", workerStats.WorkerRSS),
							workerStatsRow("Files", workerStats.WorkerFiles),
							workerStatsRow("Meta", workerStats.WorkerMeta),
							workerStatsRow("Index", workerStats.WorkerIndex),
							workerStatsRow("Index RSS", workerStats.WorkerIndexRSS),
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

// workerStatsRow renders one table row of worker-pool statistics.
func workerStatsRow(label string, w worker.StatsDetail) gomponents.Node {
	cell := func(v uint64) gomponents.Node {
		return html.Td(gomponents.Text(fmt.Sprintf("%d", v)))
	}

	return html.Tr(
		html.Td(html.Strong(gomponents.Text(label))),
		cell(w.SubmittedTasks),
		cell(w.CompletedTasks),
		cell(w.SuccessfulTasks),
		cell(w.FailedTasks),
		cell(w.DroppedTasks),
		cell(w.WaitingTasks),
		cell(w.RunningWorkers),
	)
}
