package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderQueuePage renders the queue monitoring page
func renderQueuePage(ctx *gin.Context) {
	pageNode := page("Queue Monitor", false, false, true, renderQueueGrid())
	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderQueueGrid creates a grid showing active queue items
func renderQueueGrid() gomponents.Node {
	var queueData []map[string]any

	for i, value := range worker.GetQueues() {
		queueData = append(queueData, map[string]any{
			"id":      i,
			"queue":   value.Queue,
			"job":     value.Name,
			"added":   value.Added.Format("2006-01-02 15:04:05"),
			"started": value.Started.Format("2006-01-02 15:04:05"),
		})
	}
	var rows []gomponents.Node
	for _, item := range queueData {
		rows = append(rows, html.Tr(
			html.Style("border-left: 4px solid #007bff; transition: all 0.2s;"),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("badge bg-primary"),
					html.Style("font-size: 0.875rem; border-radius: 15px;"),
					gomponents.Text(fmt.Sprintf("#%v", item["id"])),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("fw-semibold"),
					html.Style("color: #495057;"),
					gomponents.Text(fmt.Sprintf("%v", item["queue"])),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Div(
					html.I(html.Class("fas fa-cog me-2"), html.Style("color: #6c757d;")),
					html.Span(gomponents.Text(fmt.Sprintf("%v", item["job"]))),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Small(
					html.Class("text-muted"),
					html.I(html.Class("fas fa-plus me-1")),
					gomponents.Text(fmt.Sprintf("%v", item["added"])),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Small(
					html.Class("text-success fw-semibold"),
					html.I(html.Class("fas fa-play me-1")),
					gomponents.Text(fmt.Sprintf("%v", item["started"])),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle; text-align: center;"),
				html.Button(
					html.Class("btn btn-outline-danger btn-sm cancel-queue-btn"),
					html.Style("border-radius: 20px; padding: 0.4rem 1rem;"),
					html.Type("button"),
					html.Data("queue-id", fmt.Sprintf("%v", item["id"])),
					html.I(html.Class("fas fa-times me-1")),
					gomponents.Text("Cancel"),
				),
			),
		))
	}
	if len(rows) == 0 {
		rows = append(rows, html.Tr(
			html.Td(
				html.ColSpan("6"),
				html.Class("text-center text-muted p-5"),
				html.I(html.Class("fas fa-inbox mb-3"), html.Style("font-size: 4rem; color: #dee2e6;")),
				html.Div(
					html.H5(html.Class("text-muted mb-2"), gomponents.Text("No Active Queue Items")),
					html.P(html.Class("text-muted mb-0"), gomponents.Text("All background tasks have completed successfully")),
				),
			),
		))
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
					html.I(html.Class("fas fa-tasks header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Queue Monitor")),
					html.P(html.Class("header-subtitle"), gomponents.Text("Real-time monitoring of active job queues and background tasks")),
				),
			),
		),

		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;"),
						html.Div(
							html.Class("d-flex align-items-center justify-content-between"),
							html.Div(
								html.Class("d-flex align-items-center"),
								html.I(html.Class("fas fa-list-alt me-2"), html.Style("color: #6c757d; font-size: 1.2rem;")),
								html.H5(html.Class("card-title mb-0"), html.Style("color: #495057; font-weight: 600;"), gomponents.Text("Active Queue Items")),
							),
							html.Div(
								html.Class("badge badge-primary px-3 py-2"),
								html.Style("background: linear-gradient(45deg, #007bff, #0056b3); border-radius: 20px;"),
								html.I(html.Class("fas fa-clock me-1")),
								gomponents.Text(fmt.Sprintf("%d Active", len(queueData))),
							),
						),
					),
					html.Div(
						html.Class("card-body p-0"),
						func() gomponents.Node {
							if len(rows) == 0 || (len(rows) == 1 && len(queueData) == 0) {
								return html.Div(
									html.Class("text-center p-5"),
									html.I(html.Class("fas fa-inbox mb-3"), html.Style("font-size: 4rem; color: #dee2e6;")),
									html.H5(html.Class("text-muted mb-2"), gomponents.Text("No Active Queue Items")),
									html.P(html.Class("text-muted mb-0"), gomponents.Text("All background tasks have completed successfully")),
								)
							}
							return html.Div(
								html.Class("table-responsive"),
								html.Table(
									html.Class("table table-hover mb-0"),
									html.Style("background: transparent;"),
									html.THead(
										html.Class("table-light"),
										html.Tr(
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("ID")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Queue")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Job")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Added")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Started")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem; text-align: center;"), gomponents.Text("Actions")),
										),
									),
									html.TBody(
										rows...,
									),
								),
							)
						}(),
					),
				),
			),
		),
		html.Script(
			gomponents.Raw(`
				// Auto-refresh every 10 seconds
				setInterval(function() {
					window.location.reload();
				}, 10000);
				
				// Handle cancel button clicks
				document.addEventListener('click', function(e) {
					if (e.target.classList.contains('cancel-queue-btn')) {
						const queueId = e.target.getAttribute('data-queue-id');
						if (confirm('Are you sure you want to cancel this job?')) {
							fetch('/api/queue/cancel/' + queueId + '?apikey=' + encodeURIComponent('`+config.GetSettingsGeneral().WebAPIKey+`'), {
								method: 'DELETE',
								headers: {
									'Content-Type': 'application/json',
									'X-CSRF-Token': document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || ''
								}
							})
							.then(response => response.json())
							.then(data => {
								if (data.success) {
									// Refresh the page to show updated queue
									window.location.reload();
								} else {
									alert('Failed to cancel job: ' + (data.error || 'Unknown error'));
								}
							})
							.catch(error => {
								console.error('Error:', error);
								alert('Error canceling job: ' + error.message);
							});
						}
					}
				});
			`),
		),
	)
}

// renderSchedulerPage renders the scheduler monitoring page
func renderSchedulerPage(ctx *gin.Context) {
	pageNode := page("Scheduler Monitor", false, false, true, renderSchedulerGrid())
	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderSchedulerGrid creates a grid showing scheduler status
func renderSchedulerGrid() gomponents.Node {
	var schedulerData []map[string]any

	for i, value := range worker.GetSchedules() {
		schedulerData = append(schedulerData, map[string]any{
			"id":        i,
			"job":       value.JobName,
			"lastrun":   value.LastRun.Format("2006-01-02 15:04:05"),
			"nextrun":   value.NextRun.Format("2006-01-02 15:04:05"),
			"isrunning": value.IsRunning,
		})
	}

	var rows []gomponents.Node
	for _, item := range schedulerData {
		isRunning := item["isrunning"].(bool)

		rows = append(rows, html.Tr(
			html.Style("border-left: 4px solid "+func() string {
				if isRunning {
					return "#28a745"
				}
				return "#6c757d"
			}()+"; transition: all 0.2s;"),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("badge bg-secondary"),
					html.Style("font-size: 0.875rem; border-radius: 15px;"),
					gomponents.Text(fmt.Sprintf("#%v", item["id"])),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Div(
					html.I(html.Class("fas fa-clock me-2"), html.Style("color: #6c757d;")),
					html.Span(
						html.Class("fw-semibold"),
						html.Style("color: #495057;"),
						gomponents.Text(fmt.Sprintf("%v", item["job"])),
					),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Small(
					html.Class("text-muted"),
					html.I(html.Class("fas fa-history me-1")),
					gomponents.Text(fmt.Sprintf("%v", item["lastrun"])),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Small(
					html.Class("text-info fw-semibold"),
					html.I(html.Class("fas fa-forward me-1")),
					gomponents.Text(fmt.Sprintf("%v", item["nextrun"])),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle; text-align: center;"),
				func() gomponents.Node {
					if isRunning {
						return html.Span(
							html.Class("badge bg-success px-3 py-2"),
							html.Style("border-radius: 15px;"),
							html.I(html.Class("fas fa-play me-1")),
							gomponents.Text("Running"),
						)
					}
					return html.Span(
						html.Class("badge bg-secondary px-3 py-2"),
						html.Style("border-radius: 15px;"),
						html.I(html.Class("fas fa-pause me-1")),
						gomponents.Text("Idle"),
					)
				}(),
			),
		))
	}
	if len(rows) == 0 {
		rows = append(rows, html.Tr(
			html.Td(
				html.ColSpan("5"),
				html.Class("text-center text-muted"),
				gomponents.Text("No scheduled jobs found"),
			),
		))
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
					html.I(html.Class("fas fa-calendar-alt header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Scheduler Monitor")),
					html.P(html.Class("header-subtitle"), gomponents.Text("Overview of scheduled jobs, their status and execution times")),
				),
			),
		),

		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;"),
						html.Div(
							html.Class("d-flex align-items-center justify-content-between"),
							html.Div(
								html.Class("d-flex align-items-center"),
								html.I(html.Class("fas fa-tasks me-2"), html.Style("color: #6c757d; font-size: 1.2rem;")),
								html.H5(html.Class("card-title mb-0"), html.Style("color: #495057; font-weight: 600;"), gomponents.Text("Scheduled Jobs")),
							),
							html.Div(
								html.Class("badge badge-success px-3 py-2"),
								html.Style("background: linear-gradient(45deg, #28a745, #20c997); border-radius: 20px;"),
								html.I(html.Class("fas fa-sync-alt me-1")),
								gomponents.Text(fmt.Sprintf("%d Jobs", len(schedulerData))),
							),
						),
					),
					html.Div(
						html.Class("card-body p-0"),
						func() gomponents.Node {
							if len(rows) == 0 || (len(rows) == 1 && len(schedulerData) == 0) {
								return html.Div(
									html.Class("text-center p-5"),
									html.I(html.Class("fas fa-calendar-times mb-3"), html.Style("font-size: 4rem; color: #dee2e6;")),
									html.H5(html.Class("text-muted mb-2"), gomponents.Text("No Scheduled Jobs")),
									html.P(html.Class("text-muted mb-0"), gomponents.Text("No scheduled jobs are currently configured")),
								)
							}
							return html.Div(
								html.Class("table-responsive"),
								html.Table(
									html.Class("table table-hover mb-0"),
									html.Style("background: transparent;"),
									html.THead(
										html.Class("table-light"),
										html.Tr(
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("ID")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Job")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Last Run")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Next Run")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem; text-align: center;"), gomponents.Text("Status")),
										),
									),
									html.TBody(
										rows...,
									),
								),
							)
						}(),
					),
				),
			),
		),
		html.Script(
			gomponents.Raw(`
				// Auto-refresh every 60 seconds
				setInterval(function() {
					window.location.reload();
				}, 60000);
			`),
		),
	)
}

func renderStatsPage(ctx *gin.Context) {
	pageNode := page("Media Statistics", false, false, true, renderStatsGrid())
	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

func renderStatsGrid() gomponents.Node {
	var statsData []map[string]any
	id := 0

	movieLists := database.GetrowsN[string](
		false,
		5,
		"select distinct listname from movies where listname is not null and listname != ''",
	)
	for idx := range movieLists {
		all := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE",
			&movieLists[idx],
		)
		missing := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE and missing=1",
			&movieLists[idx],
		)
		reached := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE and quality_reached=1",
			&movieLists[idx],
		)
		upgrade := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE and quality_reached=0 and missing=0",
			&movieLists[idx],
		)

		statsData = append(statsData, map[string]any{
			"id":       id,
			"typ":      "movies",
			"list":     movieLists[idx],
			"total":    all,
			"missing":  missing,
			"finished": reached,
			"upgrade":  upgrade,
		})
		id++
	}

	seriesLists := database.GetrowsN[string](
		false,
		5,
		"select distinct listname from series where listname is not null and listname != ''",
	)
	for idx := range seriesLists {
		all := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
			&seriesLists[idx],
		)
		missing := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and missing=1",
			&seriesLists[idx],
		)
		reached := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and quality_reached=1",
			&seriesLists[idx],
		)
		upgrade := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and quality_reached=0 and missing=0",
			&seriesLists[idx],
		)

		statsData = append(statsData, map[string]any{
			"id":       id,
			"typ":      "episodes",
			"list":     seriesLists[idx],
			"total":    all,
			"missing":  missing,
			"finished": reached,
			"upgrade":  upgrade,
		})
		id++
	}

	var rows []gomponents.Node
	for _, stat := range statsData {
		typ := stat["typ"].(string)

		rows = append(rows, html.Tr(
			html.Style("border-left: 4px solid "+func() string {
				if typ == "movies" {
					return "#007bff"
				}
				return "#28a745"
			}()+"; transition: all 0.2s;"),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("badge bg-secondary"),
					html.Style("font-size: 0.875rem; border-radius: 15px;"),
					gomponents.Text(fmt.Sprintf("#%d", stat["id"].(int))),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				func() gomponents.Node {
					if typ == "movies" {
						return html.Span(
							html.Class("badge bg-primary px-3 py-2"),
							html.Style("border-radius: 15px;"),
							html.I(html.Class("fas fa-film me-1")),
							gomponents.Text("Movies"),
						)
					}
					return html.Span(
						html.Class("badge bg-success px-3 py-2"),
						html.Style("border-radius: 15px;"),
						html.I(html.Class("fas fa-tv me-1")),
						gomponents.Text("Episodes"),
					)
				}(),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Div(
					html.I(html.Class("fas fa-list me-2"), html.Style("color: #6c757d;")),
					html.Span(
						html.Class("fw-semibold"),
						html.Style("color: #495057;"),
						gomponents.Text(stat["list"].(string)),
					),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("badge bg-secondary px-3 py-2"),
					html.Style("border-radius: 15px;"),
					html.I(html.Class("fas fa-database me-1")),
					gomponents.Text(fmt.Sprintf("%d", stat["total"].(uint))),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("badge bg-warning px-3 py-2"),
					html.Style("border-radius: 15px;"),
					html.I(html.Class("fas fa-exclamation-triangle me-1")),
					gomponents.Text(fmt.Sprintf("%d", stat["missing"].(uint))),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("badge bg-success px-3 py-2"),
					html.Style("border-radius: 15px;"),
					html.I(html.Class("fas fa-check-circle me-1")),
					gomponents.Text(fmt.Sprintf("%d", stat["finished"].(uint))),
				),
			),
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle;"),
				html.Span(
					html.Class("badge bg-info px-3 py-2"),
					html.Style("border-radius: 15px;"),
					html.I(html.Class("fas fa-arrow-up me-1")),
					gomponents.Text(fmt.Sprintf("%d", stat["upgrade"].(uint))),
				),
			),
		))
	}

	if len(rows) == 0 {
		rows = append(rows, html.Tr(
			html.Td(
				html.ColSpan("7"),
				html.Class("text-center text-muted p-5"),
				html.I(html.Class("fas fa-chart-bar mb-3"), html.Style("font-size: 4rem; color: #dee2e6;")),
				html.Div(
					html.H5(html.Class("text-muted mb-2"), gomponents.Text("No Statistics Available")),
					html.P(html.Class("text-muted mb-0"), gomponents.Text("No media statistics found. Add media configurations to see statistics.")),
				),
			),
		))
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
					html.I(html.Class("fas fa-chart-bar header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Media Statistics")),
					html.P(html.Class("header-subtitle"), gomponents.Text("Overview of media library status including totals, missing items, and quality metrics")),
				),
			),
		),

		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;"),
						html.Div(
							html.Class("d-flex align-items-center justify-content-between"),
							html.Div(
								html.Class("d-flex align-items-center"),
								html.I(html.Class("fas fa-list-ul me-2"), html.Style("color: #6c757d; font-size: 1.2rem;")),
								html.H5(html.Class("card-title mb-0"), html.Style("color: #495057; font-weight: 600;"), gomponents.Text("Library Statistics")),
							),
							html.Div(
								html.Class("badge badge-info px-3 py-2"),
								html.Style("background: linear-gradient(45deg, #17a2b8, #20c997); border-radius: 20px;"),
								html.I(html.Class("fas fa-database me-1")),
								gomponents.Text(fmt.Sprintf("%d Lists", len(statsData))),
							),
						),
					),
					html.Div(
						html.Class("card-body p-0"),
						func() gomponents.Node {
							if len(rows) == 0 || (len(rows) == 1 && len(statsData) == 0) {
								return html.Div(
									html.Class("text-center p-5"),
									html.I(html.Class("fas fa-chart-bar mb-3"), html.Style("font-size: 4rem; color: #dee2e6;")),
									html.H5(html.Class("text-muted mb-2"), gomponents.Text("No Statistics Available")),
									html.P(html.Class("text-muted mb-0"), gomponents.Text("No media statistics found. Add media configurations to see statistics.")),
								)
							}
							return html.Div(
								html.Class("table-responsive"),
								html.Table(
									html.Class("table table-hover mb-0"),
									html.Style("background: transparent;"),
									html.THead(
										html.Class("table-light"),
										html.Tr(
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("ID")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Type")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("List")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Total")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Missing")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Finished")),
											html.Th(html.Style("border-top: none; color: #495057; font-weight: 600; padding: 1rem;"), gomponents.Text("Upgradable")),
										),
									),
									html.TBody(
										rows...,
									),
								),
							)
						}(),
					),
				),
			),
		),
	)
}

func renderTableEditForm(table string, data map[string]any, id string, csrfToken string) gomponents.Node {
	formNodes := []gomponents.Node{
		html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
	}

	logger.LogDynamicany1Any("info", "testtable", "csrf", csrfToken)

	// Get table columns for displaynames - use form-specific columns to exclude joined columns
	tableColumns := getAdminFormColumns(table)
	columnMap := make(map[string]string)
	for _, col := range tableColumns {
		cleanName := col.Name
		if strings.Contains(col.Name, " as ") {
			cleanName = strings.Split(col.Name, " as ")[1]
		}
		columnMap[cleanName] = col.DisplayName
	}

	// Helper function to get display name from column map
	getColumnDisplayName := func(columnMap map[string]string, fieldName string) string {
		if displayName, exists := columnMap[fieldName]; exists {
			return displayName
		}
		// Fallback to formatted field name with proper capitalization
		parts := strings.Split(fieldName, "_")
		var capitalizedParts []string
		for _, part := range parts {
			if len(part) > 0 {
				capitalizedParts = append(capitalizedParts, strings.ToTitle(strings.ToLower(part)))
			}
		}
		return strings.Join(capitalizedParts, " ")
	}
	// Sort keys to ensure consistent field ordering
	var sortedKeys []string
	for col := range data {
		sortedKeys = append(sortedKeys, col)
	}
	sort.Strings(sortedKeys)

	for _, col := range sortedKeys {
		fieldData := data[col]

		// Skip readonly fields entirely - don't include them in forms
		if col == "id" || col == "created_at" || col == "updated_at" || col == "lastscan" {
			continue
		}

		// Check if this is a quality_profile, listname, or quality_type field that should be a config dropdown
		if col == "quality_profile" || col == "listname" || (col == "quality_type" && table == "qualities") {
			currentValue := ""
			if fieldData != nil {
				currentValue = fmt.Sprintf("%v", fieldData)
			}

			var options []gomponents.Node
			options = append(options, createOption("", "-- Select or type custom --", false))

			// Add config options based on field type
			switch col {
			case "quality_profile":
				qualityConfigs := config.GetSettingsQualityAll()
				for _, qc := range qualityConfigs {
					options = append(options, createOption(qc.Name, qc.Name, currentValue == qc.Name))
				}
			case "listname":
				for _, lc := range config.GetSettingsMediaListAll() {
					options = append(options, createOption(lc, lc, currentValue == lc))
				}
			case "quality_type":
				// Quality type options: 1 = Resolution, 2 = Quality, 3 = Codec, 4 = Audio
				qualityTypes := map[string]string{
					"1": "Resolution",
					"2": "Quality",
					"3": "Codec",
					"4": "Audio",
				}
				for value, label := range qualityTypes {
					options = append(options, createOption(value, label, currentValue == value))
				}
			}

			formNodes = append(formNodes, html.Div(
				html.Class("form-field-enhanced"),
				html.Label(html.Class("form-label-modern"), html.For("field-"+col),
					html.I(html.Class("fas fa-cog field-icon")),
					gomponents.Text(" "+getColumnDisplayName(columnMap, col)),
				),
				html.Select(
					html.Class("form-select choices-ajax"),
					html.ID("field-"+col),
					html.Name("field-"+col),
					html.Data("allow-custom", "true"),
					gomponents.Group(options),
				),
			))
			continue
		}

		// Check if this is a foreign key field that should be a dropdown
		if strings.HasSuffix(col, "_id") && col != "id" {
			refTable := getReferenceTable(col)
			if refTable != "" {
				// Remove static options loading for better performance
				// Convert current value to string for comparison
				currentValue := ""
				if fieldData != nil {
					switch v := fieldData.(type) {
					case int:
						currentValue = fmt.Sprintf("%d", v)
					case int64:
						currentValue = fmt.Sprintf("%d", v)
					case uint:
						currentValue = fmt.Sprintf("%d", v)
					case uint64:
						currentValue = fmt.Sprintf("%d", v)
					case float64:
						// Handle float64 (JSON unmarshaling default)
						if v == float64(int64(v)) {
							currentValue = fmt.Sprintf("%.0f", v)
						} else {
							currentValue = fmt.Sprintf("%v", v)
						}
					case string:
						currentValue = v
					default:
						currentValue = fmt.Sprintf("%v", v)
					}
				}

				// Create AJAX-powered dropdown with current value
				var optionNodes []gomponents.Node
				optionNodes = append(optionNodes, createOption("", "-- Select --", false))

				// If there's a current value, add it as a selected option (will be replaced by AJAX)
				if currentValue != "" {
					optionNodes = append(optionNodes, createOption(currentValue, "Loading...", true))
				}

				formNodes = append(formNodes, html.Div(
					html.Class("mb-3"),
					html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
					html.Select(
						html.Class("form-select choices-ajax"),
						html.ID("field-"+col),
						html.Name("field-"+col),
						html.Data("ajax-url", "/api/admin/dropdown/"+refTable+"/"+col),
						html.Data("selected-value", currentValue),
						html.Data("placeholder", "Search..."),
						gomponents.Group(optionNodes),
					),
				))
				continue
			}
		}

		switch val := (fieldData).(type) {
		case bool:
			formNodes = append(formNodes, html.Div(
				html.Class("form-check form-switch"),
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				formCheckboxInput("field-"+col, "field-"+col, val),
			))
		case string:
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				formTextInput("field-"+col, "field-"+col, val),
			))
		case int:
			if col == "missing" || col == "blacklisted" || col == "dont_search" || col == "dont_upgrade" || col == "use_regex" || col == "proper" || col == "extended" || col == "repack" || col == "ignore_runtime" || col == "adult" || col == "search_specials" || col == "quality_reached" {
				checked, _ := strconv.ParseBool(fmt.Sprintf("%v", fieldData))
				formNodes = append(formNodes, html.Div(
					html.Class("form-check form-switch"),
					html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
					formCheckboxInput("field-"+col, "field-"+col, checked, html.Value(fmt.Sprintf("%v", fieldData))),
				))
			} else {
				formNodes = append(formNodes, html.Div(
					html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
					html.Input(html.Class("form-control"), html.ID("field-"+col), html.Type("number"), html.Name("field-"+col), html.Value(fmt.Sprintf("%v", fieldData))),
				))
			}
		case time.Time:
			valformat := val.Format("2006-01-02")
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				html.Input(html.Class("form-control datepicker"), html.ID("field-"+col), html.Type("date"), html.Name("field-"+col), html.Value(valformat)),
			))
		case sql.NullTime:
			valformat := val.Time.Format("2006-01-02")
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				html.Input(html.Class("form-control datepicker"), html.ID("field-"+col), html.Type("date"), html.Name("field-"+col), html.Value(valformat)),
			))
		default:
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				formTextInput("field-"+col, "field-"+col, fmt.Sprintf("%v", fieldData)),
			))
		}
	}

	// Add save button to form
	formNodes = append(formNodes,
		html.Div(
			html.Class("mt-3"),
			html.Button(
				html.Type("submit"),
				html.Class("btn btn-primary me-2"),
				gomponents.Text("Save Changes"),
			),
			html.Button(
				html.Type("button"),
				html.Class("btn btn-secondary"),
				html.Data("bs-dismiss", "modal"),
				gomponents.Text("Cancel"),
			),
		),
	)

	// Determine form title and action based on whether we're adding or editing
	var formTitle string
	var formAction string
	if id == "new" {
		formTitle = "Add New Row"
		formAction = "/api/admin/table/" + table + "/insert"
	} else {
		formTitle = "Edit Row"
		formAction = "/api/admin/table/" + table + "/update/" + id
	}

	return html.Div(
		html.Class("edit-form-container"),
		// Enhanced form header
		html.Div(
			html.Class("edit-form-header"),
			html.H2(html.Class("edit-form-title"), gomponents.Text(formTitle)),
			html.P(html.Class("edit-form-subtitle"), gomponents.Text("Complete the form fields below and save your changes")),
		),
		// Enhanced form with modern styling
		html.Form(
			html.Method("post"),
			html.Action(formAction),
			html.ID("editForm"),
			html.Class("edit-form-modern"),

			// Form fields container
			html.Div(
				html.Class("edit-form-fields"),
				gomponents.Group(formNodes),
			),

			// Form actions - moved to modal footer
			addEditFormJavascript(),
		),
	)
}

func addEditFormJavascript() gomponents.Node {
	return html.Script(gomponents.Raw(`
				// Select2 initialization is now handled by the global initSelect2Global function
				
				// Dropdown initialization is now handled by initChoicesGlobal in web.go
				// Use class 'choices-ajax' with data-allow-custom="true" for custom value support
				
				var editForm = document.getElementById('editForm');
				if (editForm) {
					editForm.addEventListener('submit', function(e) {
					e.preventDefault();
					var formData = new FormData(this);
					
					// Convert form data to URL-encoded format
					var params = new URLSearchParams();
					for (var pair of formData.entries()) {
						params.append(pair[0], pair[1]);
					}
					
					// Get CSRF token from form
					var csrfToken = this.querySelector('input[name="csrf_token"]').value;
					
					fetch(this.action, {
						method: 'POST',
						headers: {
							'Content-Type': 'application/x-www-form-urlencoded',
							'X-CSRF-Token': csrfToken,
						},
						body: params.toString()
					})
					.then(response => {
						return response.json();
					})
					.then(data => {
						// Find which modal this form is in and close it
						var $modal = $(this).closest('.modal');
						if ($modal.length) {
							$modal.modal('hide');
						}
						
						// Reload the table
						if (typeof oTable !== 'undefined') {
							oTable.ajax.reload();
						}
						
						// Show toaster notification - handle both formats
						if (data.success === true || data.success === "true") {
							if (typeof showToaster === 'function') {
								showToaster('success', 'Record saved successfully');
							}
						} else {
							if (typeof showToaster === 'function') {
								showToaster('error', data.error || 'Error saving record');
							}
						}
					})
					.catch(error => {
						console.error('Fetch error:', error);
						if (typeof showToaster === 'function') {
							showToaster('error', 'Network error: ' + error.message);
						} else {
							console.error('showToaster function not available');
							alert('Network error: ' + error.message);
						}
					});
					});
				}
			`))
}

// renderCustomFilters creates table-specific filter fields for enhanced searching
func renderCustomFilters(tableName string) gomponents.Node {
	var filterFields []gomponents.Node

	// Get filterable fields from goadmin models
	filterFields = getFilterableFieldsForTable(tableName)

	// If no dynamic filters found, check for special hardcoded cases
	if len(filterFields) == 0 {
		switch tableName {
		case "dbmovie_titles":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-title"), html.Placeholder("Filter by title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Movie Name")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-movie_title"), html.Placeholder("Filter by movie name...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Region")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-region"), html.Placeholder("Region...")),
				),
			}
		}
	}

	// Enhanced cases with dynamic options (only if dynamic filters not available)
	if len(filterFields) == 0 {
		switch tableName {
		case "movies":
			var qualoptions []gomponents.Node
			qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
			qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
			qualoptions = append(qualoptions, createOption("", "All Profiles", false))
			qualityConfigs := config.GetSettingsQualityAll()
			for _, qc := range qualityConfigs {
				qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
			}
			var listoptions []gomponents.Node
			listoptions = append(listoptions, html.Class("form-control custom-filter"))
			listoptions = append(listoptions, html.ID("filter-listname"))
			listoptions = append(listoptions, createOption("", "All Lists", false))
			for _, lc := range config.GetSettingsMediaListAll() {
				listoptions = append(listoptions, createOption(lc, lc, false))
			}
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-title"), html.Placeholder("Filter by title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Year")),
					html.Input(html.Class("form-control custom-filter"), html.Type("number"),
						html.ID("filter-year"), html.Placeholder("Year...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("IMDB ID")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-imdb_id"), html.Placeholder("tt1234567...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Listname")),
					html.Select(listoptions...),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Quality Reached")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-quality_reached"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Missing")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-missing"),
						createOption("", "All", false),
						createOption("1", "Missing", false),
						createOption("0", "Available", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
					html.Select(qualoptions...),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Rootpath")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-rootpath"), html.Placeholder("Filter by rootpath...")),
				),
			}
		case "series":
			var listoptions []gomponents.Node
			listoptions = append(listoptions, html.Class("form-control custom-filter"))
			listoptions = append(listoptions, html.ID("filter-listname"))
			listoptions = append(listoptions, createOption("", "All Lists", false))
			for _, lc := range config.GetSettingsMediaListAll() {
				listoptions = append(listoptions, createOption(lc, lc, false))
			}
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Series Name")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-seriename"), html.Placeholder("Filter by series name...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Listname")),
					html.Select(listoptions...),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Rootpath")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-rootpath"), html.Placeholder("Filter by rootpath...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Don't Upgrade")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-dont_upgrade"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Don't Search")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-dont_search"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Search Specials")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-search_specials"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Ignore Runtime")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-ignore_runtime"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
			}
		case "movie_files", "serie_episode_files":
			var qualoptions []gomponents.Node
			qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
			qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
			qualoptions = append(qualoptions, createOption("", "All Profiles", false))
			qualityConfigs := config.GetSettingsQualityAll()
			for _, qc := range qualityConfigs {
				qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
			}
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Filename")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-filename"), html.Placeholder("Filter by filename...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Resolution")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-resolution"), html.Placeholder("1080p, 720p...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
					html.Select(qualoptions...),
				),
			}
		case "dbserie_alternates":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-title"), html.Placeholder("Filter by title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Series Name")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-series_name"), html.Placeholder("Filter by series name...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Region")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-region"), html.Placeholder("Region...")),
				),
			}
		case "dbserie_episodes":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Episode Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-title"), html.Placeholder("Filter by episode title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Series Name")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-series_name"), html.Placeholder("Filter by series name...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Season")),
					html.Input(html.Class("form-control custom-filter"), html.Type("number"),
						html.ID("filter-season"), html.Placeholder("Season...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Episode")),
					html.Input(html.Class("form-control custom-filter"), html.Type("number"),
						html.ID("filter-episode"), html.Placeholder("Episode...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Identifier")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-identifier"), html.Placeholder("Identifier...")),
				),
			}
		case "serie_episodes":
			var qualoptions []gomponents.Node
			qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
			qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
			qualoptions = append(qualoptions, createOption("", "All Profiles", false))
			qualityConfigs := config.GetSettingsQualityAll()
			for _, qc := range qualityConfigs {
				qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
			}
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Episode Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-episode_title"), html.Placeholder("Filter by episode title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
					html.Select(qualoptions...),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Missing")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-missing"),
						createOption("", "All", false),
						createOption("1", "Missing", false),
						createOption("0", "Available", false),
					),
				),
			}
		case "job_histories":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Job Type")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-job_type"), html.Placeholder("Filter by job type...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Job Category")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-job_category"), html.Placeholder("Filter by category...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Job Group")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-job_group"), html.Placeholder("Filter by group...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Status")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-ended"),
						createOption("", "All", false),
						createOption("1", "Completed", false),
						createOption("0", "Running", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Started Date")),
					html.Input(html.Class("form-control custom-filter"), html.Type("date"),
						html.ID("filter-started_date"), html.Placeholder("Started date...")),
				),
			}
		case "qualities":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Type")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-type"),
						createOption("", "All Types", false),
						createOption("0", "Movies", false),
						createOption("1", "Series", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Name")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-name"), html.Placeholder("Filter by name...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Use Regex")),
					html.Select(html.Class("form-control custom-filter"), html.ID("filter-use_regex"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Priority")),
					html.Input(html.Class("form-control custom-filter"), html.Type("number"),
						html.ID("filter-priority"), html.Placeholder("Priority...")),
				),
			}
		case "movie_histories":
			var qualoptions []gomponents.Node
			qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
			qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
			qualoptions = append(qualoptions, createOption("", "All Profiles", false))
			qualityConfigs := config.GetSettingsQualityAll()
			for _, qc := range qualityConfigs {
				qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
			}
			var listoptions []gomponents.Node
			listoptions = append(listoptions, html.Class("form-control custom-filter"))
			listoptions = append(listoptions, html.ID("filter-listname"))
			listoptions = append(listoptions, createOption("", "All Lists", false))
			for _, lc := range config.GetSettingsMediaListAll() {
				listoptions = append(listoptions, createOption(lc, lc, false))
			}
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-title"), html.Placeholder("Filter by title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Indexer")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-indexer"), html.Placeholder("Filter by indexer...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
					html.Select(qualoptions...),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Downloaded Date")),
					html.Input(html.Class("form-control custom-filter"), html.Type("date"),
						html.ID("filter-downloaded_date"), html.Placeholder("Downloaded date...")),
				),
			}
		case "movie_file_unmatcheds":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Filepath")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-filepath"), html.Placeholder("Filter by filepath...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Listname")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-listname"), html.Placeholder("Filter by listname...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-movie_quality_profile"), html.Placeholder("Quality...")),
				),
			}
		case "serie_file_unmatcheds":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Filepath")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-filepath"), html.Placeholder("Filter by filepath...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Listname")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-listname"), html.Placeholder("Filter by listname...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Root Path")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-series_rootpath"), html.Placeholder("Root path...")),
				),
			}
		case "serie_episode_histories":
			filterFields = []gomponents.Node{
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-title"), html.Placeholder("Filter by title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Episode Title")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-episode_title"), html.Placeholder("Filter by episode title...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Indexer")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-indexer"), html.Placeholder("Indexer...")),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Quality")),
					html.Input(html.Class("form-control custom-filter"), html.Type("text"),
						html.ID("filter-quality_profile"), html.Placeholder("Quality...")),
				),
			}
		default:
			// No special filters defined for this table
		}
	}

	if len(filterFields) == 0 {
		return html.Div()
	}

	// Add clear filters button
	filterFields = append(filterFields,
		html.Div(html.Class("d-flex align-items-end"),
			html.Button(html.Class("btn btn-secondary me-2"), html.ID("apply-filters"),
				gomponents.Text("Apply Filters")),
			html.Button(html.Class("btn btn-outline-secondary"), html.ID("clear-filters"),
				gomponents.Text("Clear")),
		),
	)

	return html.Div(html.Class("filters-card-enhanced"),
		html.Button(
			html.Class("filters-header accordion-button w-100 border-0"),
			html.Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;"),
			html.Type("button"),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#filters-content"),
			gomponents.Attr("aria-expanded", "false"),
			gomponents.Attr("aria-controls", "filters-content"),
			html.I(html.Class("fas fa-filter me-2 text-primary")),
			gomponents.Text("Advanced Filters"),
		),
		html.Div(
			html.Class("collapse"),
			html.ID("filters-content"),
			html.Div(
				html.Class("filters-body p-3"),
				html.Div(html.Class("filters-grid"),
					gomponents.Group(filterFields),
				),
			),
		),
		html.Script(gomponents.Raw(`
			$(document).ready(function() {
				// Restore filter state from localStorage using Bootstrap collapse
				if (localStorage.getItem('filtersCollapsed') === 'false') {
					$('#filters-content').addClass('show');
					$('[data-bs-target="#filters-content"]').attr('aria-expanded', 'true');
				}
				
				// Save state when collapse is toggled
				$('#filters-content').on('shown.bs.collapse', function () {
					localStorage.setItem('filtersCollapsed', 'false');
				});
				
				$('#filters-content').on('hidden.bs.collapse', function () {
					localStorage.setItem('filtersCollapsed', 'true');
				});
				
				// Enhanced refresh functionality
				$('#refresh-table').click(function() {
					const btn = $(this);
					const icon = btn.find('i');
					
					// Add loading state
					icon.addClass('fa-spin');
					btn.prop('disabled', true);
					
					if (typeof oTable !== 'undefined') {
						oTable.ajax.reload(function() {
							// Remove loading state after reload
							setTimeout(() => {
								icon.removeClass('fa-spin');
								btn.prop('disabled', false);
							}, 500);
						});
					}
				});
				
				// Apply filters button
				$('#apply-filters').click(function() {
					if (typeof oTable !== 'undefined') {
						oTable.ajax.reload();
					}
				});
				
				// Clear filters button
				$('#clear-filters').click(function() {
					$('.custom-filter').val('');
					if (typeof oTable !== 'undefined') {
						oTable.ajax.reload();
					}
				});
				
				// Apply filters on Enter key
				$('.custom-filter').keypress(function(e) {
					if (e.which === 13) {
						if (typeof oTable !== 'undefined') {
							oTable.ajax.reload();
						}
					}
				});
				
				// Enhanced table loading states
				if (typeof oTable !== 'undefined') {
					oTable.on('processing.dt', function(e, settings, processing) {
						if (processing) {
							$('#table-loading').show();
						} else {
							$('#table-loading').hide();
						}
					});
				}
			});
		`)),
	)
}

func renderTable(tableInfo *TableInfo, csrfToken string) gomponents.Node {
	var header []gomponents.Node
	// var footer []gomponents.Node
	var o []Mdata

	for _, col := range tableInfo.Columns {
		var addnode gomponents.Node
		var addsort gomponents.Node
		switch col.Name {
		case "id":
			addnode = html.Data("priority", "1")
		case "title", "seriename", "name", "identifier", "listname", "filename", "year":
			addnode = html.Data("priority", "1")

		case "created_at", "updated_at", "release_date", "first_aired", "overview":
			addnode = html.Data("priority", "100000")
		}
		var setname string = col.Name

		if logger.ContainsI(setname, " as ") {
			setname = strings.Split(setname, " as ")[1]
		}

		// Use displayname from column info
		header = append(header, html.Th(html.Class("sorting"), html.Role("columnheader"), addnode, addsort, gomponents.Text(col.DisplayName)))
		o = append(o, Mdata{Mdata: col.Name})
		// footer = append(footer, html.Th(html.Role("columnfooter"), html.Input(html.Type("text"), html.Name("search_"+col.Name), html.Value("Search "+col.Name), html.Class("search_init"))))
	}
	// Add Actions column header
	header = append(header, html.Th(html.Role("columnheader"), html.Data("priority", "2"), html.Data("sortable", "false"), html.Data("orderable", "false"), gomponents.Text("Actions")))
	o = append(o, Mdata{Mdata: "actions"})

	return gomponents.Group(
		[]gomponents.Node{
			html.Div(html.Class("datatables-reponsive_wrapper"),
				html.Table(
					html.ID("table-data"),
					html.Class("table table-striped datatable"),
					// html.Style("width: 100%"),
					html.THead(
						html.Tr(
							header...,
						),
					),
					//html.TFoot(html.Tr(
					//	footer...,
					//)),
				),
				html.Script(gomponents.Rawf(`
					var oTable;
					// Debug logging
					console.log('Initializing DataTable for server-side processing...');
					console.log('window.initDataTable available:', typeof window.initDataTable);
					console.log('jQuery DataTable available:', typeof $.fn.DataTable);
					
					if (window.initDataTable) {
						console.log('Calling window.initDataTable...');
						oTable = window.initDataTable('.datatable', {
							bServerSide: true,
							bProcessing: true,
							sAjaxSource: "/api/admin/tablejson/%s",
							"fnServerData": function (sSource, aoData, fnCallback) {
								// Add custom filter parameters
								$('.custom-filter').each(function() {
									var id = $(this).attr('id');
									var value = $(this).val();
									if (value) {				
										aoData.push({ "name": id, "value": value });
									}
								});
								
								$.ajax({
									"dataType": 'json',
									"type": "POST",
									"url": sSource,
									"data": aoData,
									"headers": {
										"X-CSRF-Token": "%s"
									},
									"success": fnCallback
								});
							},
							"columnDefs": [
								{
									"targets": -1,
									"data": null,
									"orderable": false,
									"searchable": false,
									"render": function (data, type, row, meta) {
										var id = row[0]; // Assuming ID is first column
										return '<div class="d-flex gap-1 justify-content-center">' +
											'<button class="btn-action-edit" data-id="' + id + '" data-bs-toggle="modal" data-bs-target="#editFormModal" title="Edit"><i class="fa fa-edit"></i></button>' +
											'<button class="btn-action-delete" data-id="' + id + '" title="Delete"><i class="fa fa-trash"></i></button>' +
											'</div>';
									}
								}
							]

							%s
						});
						console.log('DataTable initialized:', oTable);
					} else {
						console.warn('window.initDataTable not available, falling back to direct initialization');
						oTable = $('.datatable').DataTable({						
							"bDestroy": true,
							"bFilter": true,
							"bSort": true,
							"bPaginate": true,
							responsive: true,
							"aaSorting": [[ 0, "desc" ]],
							"bProcessing": true,
        					"bServerSide": true,
							"sAjaxSource": "/api/admin/tablejson/%s",
							"fnServerData": function (sSource, aoData, fnCallback) {
								console.log('Using fnServerData approach, data:', aoData);
								// Add custom filter parameters
								$('.custom-filter').each(function() {
									var id = $(this).attr('id');
									var value = $(this).val();
									if (value) {
										aoData.push({ "name": id, "value": value });
									}
								});
								
								$.ajax({
									"dataType": 'json',
									"type": "POST",
									"url": sSource,
									"data": aoData,
									"headers": {
										"X-CSRF-Token": "%s"
									},
									"success": function(data) {
										console.log('fnServerData AJAX Success:', data);
										fnCallback(data);
									},
									"error": function(xhr, error, code) {
										console.error('fnServerData AJAX Error:', error, code, xhr.responseText);
									}
								});
							},
							"columnDefs": [
								{
									"targets": -1,
									"data": null,
									"orderable": false,
									"searchable": false,
									"render": function (data, type, row, meta) {
										var id = row[0];
										return '<div class="d-flex gap-1 justify-content-center">' +
											   '<button class="btn-action-edit" data-id="' + id + '" data-bs-toggle="modal" data-bs-target="#editFormModal" title="Edit"><i class="fa fa-edit"></i></button>' +
											   '<button class="btn-action-delete" data-id="' + id + '" title="Delete"><i class="fa fa-trash"></i></button>' +
											   '</div>';
									}
								}
							]
						});
					}
					
					// Handle custom filter changes - trigger table refresh
					$(document).on('change keyup input', '.custom-filter', function() {
						var delay = 500; // Delay in milliseconds for text inputs
						var element = $(this);
						
						// Clear existing timer
						clearTimeout(element.data('timer'));
						
						// Set new timer
						element.data('timer', setTimeout(function() {
							oTable.ajax.reload();
						}, element.is('select') ? 0 : delay)); // No delay for select elements
					});
					
					// Handle Edit button clicks
					$(document).on('click', '.btn-action-edit', function() {
						var id = $(this).data('id');
						$('#editFormModal .modal-body').html('<div class="text-center"><div class="spinner-border" role="status"><span class="sr-only">Loading...</span></div><p class="mt-2">Loading edit form...</p></div>');
						var url = '/api/admin/tableedit/%s/' + id + '?apikey=%s';
						
						// Set a timeout to help debug hanging requests
						var requestTimeout = setTimeout(function() {
							console.warn('AJAX request taking longer than 10 seconds...');
							$('#editFormModal .modal-body').append('<div class="alert alert-warning mt-3">Request is taking longer than expected...</div>');
						}, 10000);
						
						$.get(url)
							.done(function(data) {
								clearTimeout(requestTimeout);
								$('#editFormModal .modal-body').html(data);
								// Initialize Select2 after form is loaded
								setTimeout(function() {
									if (window.initSelect2Global) {
										window.initSelect2Global();
									}
								}, 100);
							})
							.fail(function(xhr, status, error) {
								clearTimeout(requestTimeout);
								console.error('Failed to load edit form:', status, error);
								console.error('HTTP Status Code:', xhr.status);
								console.error('Response Text:', xhr.responseText);
								var errorMsg = 'Error loading form: ' + error;
								if (xhr.status) {
									errorMsg += ' (HTTP ' + xhr.status + ')';
								}
								$('#editFormModal .modal-body').html('<div class="alert alert-danger">' + errorMsg + '<br><small>Check console for details</small></div>');
							});
					});
					
					// Handle Delete button clicks
					$(document).on('click', '.btn-action-delete', function() {
						var id = $(this).data('id');
						if (confirm('⚠️ Are you sure you want to permanently delete this record?\n\nThis action cannot be undone.')) {
							$.ajax({
								url: '/api/admin/table/%s/delete/' + id + '?apikey=%s',
								type: 'POST',
								headers: {
									'X-CSRF-Token': $('input[name="csrf_token"]').val() || ''
								},
								success: function(data) {
									oTable.ajax.reload();
									alert('Record deleted successfully');
								},
								error: function() {
									alert('Error deleting record');
								}
							});
						}
					});
					`, tableInfo.Name, csrfToken, "", tableInfo.Name, csrfToken, tableInfo.Name, config.GetSettingsGeneral().WebAPIKey, tableInfo.Name, config.GetSettingsGeneral().WebAPIKey)),
			),
		})
}
