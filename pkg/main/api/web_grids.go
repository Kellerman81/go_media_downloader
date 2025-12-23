package api

import (
	"database/sql"
	"fmt"
	stdhtml "html"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	htmx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// Constants from the main API package.
const (
	// Common Form Field Types.
	fieldTypeText        = "text"
	fieldTypeNumber      = "number"
	fieldTypeCheckbox    = "checkbox"
	fieldTypeSelect      = "select"
	fieldTypeSelectArray = "selectarray"

	// Common HTML Attributes and Elements.
	attrTypeButton = "button"
	attrTitle      = "title"
	keyOptions     = "options"

	// Text limits and thresholds.
	commentLengthLimit   = 80
	commentTruncateLimit = 77

	// Separators and delimiters.
	separatorUnderscore = "_"

	// Array index position in field name parsing.
	fieldNameIndexPosition = 3

	// Number parsing base.
	decimalBase = 10
	trueValue   = "true"

	// HTML attributes.
	AttrHxTarget         = "hx-target"
	AttrHxPost           = "hx-post"
	AttrHxHeaders        = "hx-headers"
	AttrHxGet            = "hx-get"
	AttrHxTrigger        = "hx-trigger"
	AttrHxVals           = "hx-vals"
	AttrHxIndicator      = "hx-indicator"
	AttrHxDelete         = "hx-delete"
	AttrHxConfirm        = "hx-confirm"
	AttrHxOnAfterRequest = "hx-on--after-request"
	AttrHxPatch          = "hx-patch"
	AttrHxPut            = "hx-put"

	// Action icons.
	IconTimes = "fas fa-times"

	// System icons.
	IconCog = "fas fa-cog"

	// Data icons.
	IconList         = "fas fa-list"
	IconCalendarDays = "fas fa-calendar-days"

	// Additional FontAwesome icons.
	IconFont       = "fas fa-font"
	IconHashtag    = "fas fa-hashtag"
	IconToggleOn   = "fas fa-toggle-on"
	IconAlignLeft  = "fas fa-align-left"
	IconLayerGroup = "fas fa-layer-group"
)

// renderQueuePage renders the queue monitoring page.
func renderQueuePage(ctx *gin.Context) {
	pageNode := page("Queue Monitor", false, false, true, renderQueueGrid())

	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderQueueGrid creates a grid showing active queue items.
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
				html.I(
					html.Class("fas fa-inbox mb-3"),
					html.Style("font-size: 4rem; color: #dee2e6;"),
				),
				html.Div(
					html.H5(
						html.Class("text-muted mb-2"),
						gomponents.Text("No Active Queue Items"),
					),
					html.P(
						html.Class("text-muted mb-0"),
						gomponents.Text("All background tasks have completed successfully"),
					),
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
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Real-time monitoring of active job queues and background tasks",
						),
					),
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
						html.Style(
							"background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;",
						),
						html.Div(
							html.Class("d-flex align-items-center justify-content-between"),
							html.Div(
								html.Class("d-flex align-items-center"),
								html.I(
									html.Class("fas fa-list-alt me-2"),
									html.Style("color: #6c757d; font-size: 1.2rem;"),
								),
								html.H5(
									html.Class("card-title mb-0"),
									html.Style("color: #495057; font-weight: 600;"),
									gomponents.Text("Active Queue Items"),
								),
							),
							html.Div(
								html.Class("badge badge-primary px-3 py-2"),
								html.Style(
									"background: linear-gradient(45deg, #007bff, #0056b3); border-radius: 20px;",
								),
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
									html.I(
										html.Class("fas fa-inbox mb-3"),
										html.Style("font-size: 4rem; color: #dee2e6;"),
									),
									html.H5(
										html.Class("text-muted mb-2"),
										gomponents.Text("No Active Queue Items"),
									),
									html.P(
										html.Class("text-muted mb-0"),
										gomponents.Text(
											"All background tasks have completed successfully",
										),
									),
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
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("ID"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Queue"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Job"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Added"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Started"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem; text-align: center;",
												),
												gomponents.Text("Actions"),
											),
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

// renderSchedulerPage renders the scheduler monitoring page.
func renderSchedulerPage(ctx *gin.Context) {
	pageNode := page("Scheduler Monitor", false, false, true, renderSchedulerGrid())

	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderSchedulerGrid creates a grid showing scheduler status.
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
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Overview of scheduled jobs, their status and execution times",
						),
					),
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
						html.Style(
							"background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;",
						),
						html.Div(
							html.Class("d-flex align-items-center justify-content-between"),
							html.Div(
								html.Class("d-flex align-items-center"),
								html.I(
									html.Class("fas fa-tasks me-2"),
									html.Style("color: #6c757d; font-size: 1.2rem;"),
								),
								html.H5(
									html.Class("card-title mb-0"),
									html.Style("color: #495057; font-weight: 600;"),
									gomponents.Text("Scheduled Jobs"),
								),
							),
							html.Div(
								html.Class("badge badge-success px-3 py-2"),
								html.Style(
									"background: linear-gradient(45deg, #28a745, #20c997); border-radius: 20px;",
								),
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
									html.I(
										html.Class("fas fa-calendar-times mb-3"),
										html.Style("font-size: 4rem; color: #dee2e6;"),
									),
									html.H5(
										html.Class("text-muted mb-2"),
										gomponents.Text("No Scheduled Jobs"),
									),
									html.P(
										html.Class("text-muted mb-0"),
										gomponents.Text(
											"No scheduled jobs are currently configured",
										),
									),
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
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("ID"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Job"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Last Run"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Next Run"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem; text-align: center;",
												),
												gomponents.Text("Status"),
											),
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
				html.I(
					html.Class("fas fa-chart-bar mb-3"),
					html.Style("font-size: 4rem; color: #dee2e6;"),
				),
				html.Div(
					html.H5(
						html.Class("text-muted mb-2"),
						gomponents.Text("No Statistics Available"),
					),
					html.P(
						html.Class("text-muted mb-0"),
						gomponents.Text(
							"No media statistics found. Add media configurations to see statistics.",
						),
					),
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
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Overview of media library status including totals, missing items, and quality metrics",
						),
					),
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
						html.Style(
							"background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;",
						),
						html.Div(
							html.Class("d-flex align-items-center justify-content-between"),
							html.Div(
								html.Class("d-flex align-items-center"),
								html.I(
									html.Class("fas fa-list-ul me-2"),
									html.Style("color: #6c757d; font-size: 1.2rem;"),
								),
								html.H5(
									html.Class("card-title mb-0"),
									html.Style("color: #495057; font-weight: 600;"),
									gomponents.Text("Library Statistics"),
								),
							),
							html.Div(
								html.Class("badge badge-info px-3 py-2"),
								html.Style(
									"background: linear-gradient(45deg, #17a2b8, #20c997); border-radius: 20px;",
								),
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
									html.I(
										html.Class("fas fa-chart-bar mb-3"),
										html.Style("font-size: 4rem; color: #dee2e6;"),
									),
									html.H5(
										html.Class("text-muted mb-2"),
										gomponents.Text("No Statistics Available"),
									),
									html.P(
										html.Class("text-muted mb-0"),
										gomponents.Text(
											"No media statistics found. Add media configurations to see statistics.",
										),
									),
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
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("ID"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Type"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("List"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Total"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Missing"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Finished"),
											),
											html.Th(
												html.Style(
													"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
												),
												gomponents.Text("Upgradable"),
											),
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

func renderTableEditForm(
	table string,
	data map[string]any,
	id string,
	csrfToken string,
) gomponents.Node {
	formNodes := []gomponents.Node{
		html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
	}

	// logger.Logtype("info", 1).Any("csrf", csrfToken).Msg("testtable")

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

	// Helper function to get display name
	getColumnDisplayName := func(fieldName string) string {
		if displayName, exists := columnMap[fieldName]; exists {
			return displayName
		}
		// Fallback to formatted field name
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

		fieldName := "field-" + col
		displayName := getColumnDisplayName(col)

		// Generate field based on type and special cases
		formNodes = append(formNodes, generateFormField(col, fieldName, displayName, fieldData))
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
	var (
		formTitle  string
		formAction string
	)

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
			html.P(
				html.Class("edit-form-subtitle"),
				gomponents.Text("Complete the form fields below and save your changes"),
			),
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

// renderCustomFilters creates table-specific filter fields for enhanced searching.
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
					html.Select(
						html.Class("form-control custom-filter"),
						html.ID("filter-quality_reached"),
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
					html.Select(
						html.Class("form-control custom-filter"),
						html.ID("filter-dont_upgrade"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Don't Search")),
					html.Select(
						html.Class("form-control custom-filter"),
						html.ID("filter-dont_search"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Search Specials")),
					html.Select(
						html.Class("form-control custom-filter"),
						html.ID("filter-search_specials"),
						createOption("", "All", false),
						createOption("1", "Yes", false),
						createOption("0", "No", false),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Ignore Runtime")),
					html.Select(
						html.Class("form-control custom-filter"),
						html.ID("filter-ignore_runtime"),
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
					html.Input(
						html.Class("form-control custom-filter"),
						html.Type("text"),
						html.ID(
							"filter-series_name",
						),
						html.Placeholder("Filter by series name..."),
					),
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
					html.Input(
						html.Class("form-control custom-filter"),
						html.Type("text"),
						html.ID(
							"filter-series_name",
						),
						html.Placeholder("Filter by series name..."),
					),
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
					html.Label(html.Class("form-label"), gomponents.Text("TVDB ID")),
					html.Input(
						html.Class("form-control custom-filter"),
						html.Type("text"),
						html.ID("filter-tvdb_id"),
						html.Placeholder("Filter by TVDB ID..."),
					),
				),
				html.Div(
					html.Label(html.Class("form-label"), gomponents.Text("Episode Title")),
					html.Input(
						html.Class("form-control custom-filter"),
						html.Type("text"),
						html.ID(
							"filter-episode_title",
						),
						html.Placeholder("Filter by episode title..."),
					),
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
					html.Select(
						html.Class("form-control custom-filter"),
						html.ID("filter-use_regex"),
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
			// var listoptions []gomponents.Node
			// listoptions = append(listoptions, html.Class("form-control custom-filter"))
			// listoptions = append(listoptions, html.ID("filter-listname"))
			// listoptions = append(listoptions, createOption("", "All Lists", false))
			// for _, lc := range config.GetSettingsMediaListAll() {
			// 	listoptions = append(listoptions, createOption(lc, lc, false))
			// }
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
					html.Input(
						html.Class("form-control custom-filter"),
						html.Type("text"),
						html.ID(
							"filter-episode_title",
						),
						html.Placeholder("Filter by episode title..."),
					),
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
			html.Style(
				"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;",
			),
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
		var (
			addnode gomponents.Node
			addsort gomponents.Node
		)

		// Extract the alias from "table.column as alias" format for display
		var columnAlias string = col.Name
		if strings.Contains(strings.ToLower(col.Name), " as ") {
			parts := strings.Split(col.Name, " as ")
			if len(parts) == 2 {
				columnAlias = strings.TrimSpace(parts[1])
			}
		}

		// Check the alias (not the full col.Name) for priority assignment
		switch columnAlias {
		case "id":
			addnode = html.Data("priority", "1")
		case "title", "seriename", "name", "identifier", "listname", "filename", "year":
			addnode = html.Data("priority", "1")

		case "created_at", "updated_at", "release_date", "first_aired", "overview":
			addnode = html.Data("priority", "100000")
		}

		// Use displayname from column info
		header = append(
			header,
			html.Th(
				html.Class("sorting"),
				html.Role("columnheader"),
				html.Data("column-name", columnAlias),
				addnode,
				addsort,
				gomponents.Text(col.DisplayName),
			),
		)
		// Keep original col.Name in Mdata for server-side processing
		o = append(o, Mdata{Mdata: col.Name})
		// footer = append(footer, html.Th(html.Role("columnfooter"), html.Input(html.Type("text"), html.Name("search_"+col.Name), html.Value("Search "+col.Name), html.Class("search_init"))))
	}
	// Add Actions column header
	header = append(
		header,
		html.Th(
			html.Role("columnheader"),
			html.Data("priority", "2"),
			html.Data("sortable", "false"),
			html.Data("orderable", "false"),
			gomponents.Text("Actions"),
		),
	)
	o = append(o, Mdata{Mdata: "actions"})

	// For DataTables 1.13 with server-side processing, we don't need aoColumns
	// The server already knows the column structure and returns data correctly
	var columnsStr string = "" // Empty - no aoColumns configuration needed

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
					// html.TFoot(html.Tr(
					//	footer...,
					// )),
				),
				html.Script(gomponents.Rawf(`
					var oTable;
					// Debug logging - CODE UPDATED v2
					console.log('Initializing DataTable for server-side processing...');
					console.log('window.initDataTable available:', typeof window.initDataTable);
					console.log('jQuery DataTable available:', typeof $.fn.DataTable);

					if (window.initDataTable) {
						console.log('Calling window.initDataTable...');
						oTable = window.initDataTable('.datatable', {
							bServerSide: true,
							bProcessing: true,
							sAjaxSource: "/api/admin/tablejson/%s",
							%s
							"fnServerData": function (sSource, aoData, fnCallback) {
								// Add all filter-* parameters from URL
								var urlParams = new URLSearchParams(window.location.search);
								urlParams.forEach(function(value, key) {
									if (key.startsWith('filter-')) {
										aoData.push({ "name": key, "value": value });
									}
								});

								// Legacy support: Add 'id' parameter as 'filter-id' if present
								var idValue = urlParams.get('id');
								if (idValue && !urlParams.has('filter-id')) {
									aoData.push({ "name": "filter-id", "value": idValue });
								}

								// Add custom filter parameters from HTML elements
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
										var tableName = '%s';
										var buttons = '<div class="d-flex gap-1 justify-content-center">' +
											'<button class="btn-action-edit" data-id="' + id + '" data-bs-toggle="modal" data-bs-target="#editFormModal" title="Edit"><i class="fa fa-edit"></i></button>' +
											'<button class="btn-action-delete" data-id="' + id + '" title="Delete"><i class="fa fa-trash"></i></button>';

										// Add table-specific buttons
										if (tableName === 'movies') {
											buttons += '<button class="btn-action-files" data-id="' + id + '" title="View Files"><i class="fa fa-file"></i></button>' +
												'<button class="btn-action-search" data-id="' + id + '" data-search-type="imdb" title="Search by IMDB"><i class="fa fa-search"></i></button>' +
												'<button class="btn-action-search-title" data-id="' + id + '" data-search-type="title" title="Search by Title"><i class="fa fa-search-plus"></i></button>';
										} else if (tableName === 'serie_episodes') {
											buttons += '<button class="btn-action-files" data-id="' + id + '" title="View Files"><i class="fa fa-file"></i></button>' +
												'<button class="btn-action-search" data-id="' + id + '" data-search-type="tvdb" title="Search by TVDB"><i class="fa fa-search"></i></button>' +
												'<button class="btn-action-search-title" data-id="' + id + '" data-search-type="title" title="Search by Title"><i class="fa fa-search-plus"></i></button>';
										} else if (tableName === 'dbmovies') {
											buttons += '<button class="btn-action-metadata-refresh" data-id="' + id + '" data-type="movie" title="Refresh Metadata"><i class="fa fa-sync"></i></button>';
										} else if (tableName === 'dbseries') {
											buttons += '<button class="btn-action-metadata-refresh" data-id="' + id + '" data-type="serie" title="Refresh Metadata"><i class="fa fa-sync"></i></button>';
										}

										buttons += '</div>';
										return buttons;
									}
								},
								{
									"targets": "_all",
									"render": function (data, type, row, meta) {
										// Skip if this is the actions column (last column)
										if (meta.col === row.length - 1) {
											return data;
										}

										// Get column name from table header data attribute
										var $th = $('#table-data thead th').eq(meta.col);
										var colName = $th.attr('data-column-name');

										// Map of ID column names to their corresponding table names
										var idColumnMap = {
											'dbmovie_id': 'dbmovies',
											'movie_id': 'movies',
											'dbserie_id': 'dbseries',
											'serie_id': 'series',
											'serie_episode_id': 'serie_episodes',
											'dbserie_episode_id': 'dbserie_episodes'
										};

										// Check if this column is an ID column that should be a link
										if (idColumnMap[colName] && data && data != '') {
											var targetTable = idColumnMap[colName];
											return '<a href="/api/admin/database/' + targetTable + '?id=' + data + '" class="id-link" title="View in ' + targetTable + ' table">' + data + '</a>';
										}

										return data;
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
							%s
							"fnServerData": function (sSource, aoData, fnCallback) {
								console.log('Using fnServerData approach, data:', aoData);
								// Add ID filter from URL parameter if present
								var urlParams = new URLSearchParams(window.location.search);
								var idValue = urlParams.get('id');
								if (idValue) {
									aoData.push({ "name": "filter-id", "value": idValue });
								}

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
										var tableName = '%s';
										var buttons = '<div class="d-flex gap-1 justify-content-center">' +
											   '<button class="btn-action-edit" data-id="' + id + '" data-bs-toggle="modal" data-bs-target="#editFormModal" title="Edit"><i class="fa fa-edit"></i></button>' +
											   '<button class="btn-action-delete" data-id="' + id + '" title="Delete"><i class="fa fa-trash"></i></button>';

										// Add table-specific buttons
										if (tableName === 'movies') {
											buttons += '<button class="btn-action-files" data-id="' + id + '" title="View Files"><i class="fa fa-file"></i></button>' +
												'<button class="btn-action-search" data-id="' + id + '" data-search-type="imdb" title="Search by IMDB"><i class="fa fa-search"></i></button>' +
												'<button class="btn-action-search-title" data-id="' + id + '" data-search-type="title" title="Search by Title"><i class="fa fa-search-plus"></i></button>';
										} else if (tableName === 'serie_episodes') {
											buttons += '<button class="btn-action-files" data-id="' + id + '" title="View Files"><i class="fa fa-file"></i></button>' +
												'<button class="btn-action-search" data-id="' + id + '" data-search-type="tvdb" title="Search by TVDB"><i class="fa fa-search"></i></button>' +
												'<button class="btn-action-search-title" data-id="' + id + '" data-search-type="title" title="Search by Title"><i class="fa fa-search-plus"></i></button>';
										} else if (tableName === 'dbmovies') {
											buttons += '<button class="btn-action-metadata-refresh" data-id="' + id + '" data-type="movie" title="Refresh Metadata"><i class="fa fa-sync"></i></button>';
										} else if (tableName === 'dbseries') {
											buttons += '<button class="btn-action-metadata-refresh" data-id="' + id + '" data-type="serie" title="Refresh Metadata"><i class="fa fa-sync"></i></button>';
										}

										buttons += '</div>';
										return buttons;
									}
								},
								{
									"targets": "_all",
									"render": function (data, type, row, meta) {
										// Skip if this is the actions column (last column)
										if (meta.col === row.length - 1) {
											return data;
										}

										// Get column name from table header data attribute
										var $th = $('#table-data thead th').eq(meta.col);
										var colName = $th.attr('data-column-name');

										// Map of ID column names to their corresponding table names
										var idColumnMap = {
											'dbmovie_id': 'dbmovies',
											'movie_id': 'movies',
											'dbserie_id': 'dbseries',
											'serie_id': 'series',
											'serie_episode_id': 'serie_episodes',
											'dbserie_episode_id': 'dbserie_episodes'
										};

										// Check if this column is an ID column that should be a link
										if (idColumnMap[colName] && data && data != '') {
											var targetTable = idColumnMap[colName];
											return '<a href="/api/admin/database/' + targetTable + '?id=' + data + '" class="id-link" title="View in ' + targetTable + ' table">' + data + '</a>';
										}

										return data;
									}
								}
							]
						});
					}

					// Function to convert ID values to links in child rows
					function convertChildRowIdsToLinks() {
						var idColumnMap = {
							'dbmovie_id': 'dbmovies',
							'movie_id': 'movies',
							'dbserie_id': 'dbseries',
							'serie_id': 'series',
							'serie_episode_id': 'serie_episodes',
							'dbserie_episode_id': 'dbserie_episodes'
						};

						// Label to column name mapping (handles display names like "Database Movie ID")
						var labelToColumnMap = {
							'database_movie_id': 'dbmovie_id',
							'database_serie_id': 'dbserie_id',
							'database_serie_episode_id': 'dbserie_episode_id',
							'db_movie_id': 'dbmovie_id',
							'db_serie_id': 'dbserie_id',
							'db_serie_episode_id': 'dbserie_episode_id'
						};

						// Process all child rows
						$('.dtr-details li').each(function() {
							var $li = $(this);
							var labelText = $li.find('.dtr-title').text().trim();
							var $dataSpan = $li.find('.dtr-data');
							var dataText = $dataSpan.text().trim();

							// Check if this label matches an ID column
							var columnName = null;
							var labelLower = labelText.toLowerCase().replace(/\s+/g, '_');

							// First, check if there's a specific mapping for this label
							if (labelToColumnMap[labelLower]) {
								columnName = labelToColumnMap[labelLower];
							} else {
								// Otherwise, try direct match with ID column names
								for (var key in idColumnMap) {
									if (labelLower === key || labelLower === key.replace(/_/g, ' ')) {
										columnName = key;
										break;
									}
								}
							}

							// If this is an ID column and has a value, convert to link
							if (columnName && dataText && dataText !== '' && !isNaN(dataText)) {
								var targetTable = idColumnMap[columnName];
								var link = '<a href="/api/admin/database/' + targetTable + '?id=' + dataText + '" class="id-link" title="View in ' + targetTable + ' table">' + dataText + '</a>';
								$dataSpan.html(link);
							}
						});
					}

					// Handle responsive child row display
					oTable.on('responsive-display', function(e, datatable, row, showHide, update) {
						if (showHide) {
							// Child row was opened, convert IDs to links
							convertChildRowIdsToLinks();
						}
					});

					// Also run on initial load and after AJAX reload
					oTable.on('draw', function() {
						convertChildRowIdsToLinks();
					});

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

					// Handle Files button clicks (movies and serie_episodes tables)
					$(document).on('click', '.btn-action-files', function() {
						var id = $(this).data('id');
						var tableName = '%s';
						if (tableName === 'movies') {
							// Navigate to movie_files table with filter
							window.location.href = '/api/admin/database/movie_files?filter-movie_id=' + id;
						} else if (tableName === 'serie_episodes') {
							// Navigate to serie_episode_files table with filter
							window.location.href = '/api/admin/database/serie_episode_files?filter-serie_episode_id=' + id;
						}
					});

					// Handle Search button clicks (movies and serie_episodes tables - search by IMDB/TVDB)
					$(document).on('click', '.btn-action-search', function() {
						var id = $(this).data('id');
						var tableName = '%s';
						if (tableName === 'movies') {
							if (confirm('Start search for this movie by IMDB ID?')) {
								$.ajax({
									url: '/api/movies/search/list/' + id + '?apikey=%s&searchByTitle=false&download=true',
									type: 'GET',
									success: function(data) {
										var msg = 'Search completed!\n';
										msg += 'Accepted: ' + (data.accepted ? data.accepted.length : 0) + '\n';
										msg += 'Denied: ' + (data.denied ? data.denied.length : 0);
										alert(msg);
										oTable.ajax.reload();
									},
									error: function(xhr) {
										alert('Error starting search: ' + (xhr.responseText || 'Unknown error'));
									}
								});
							}
						} else if (tableName === 'serie_episodes') {
							if (confirm('Start search for this episode by TVDB ID?')) {
								$.ajax({
									url: '/api/series/episodes/search/list/' + id + '?apikey=%s&searchByTitle=false&download=true',
									type: 'GET',
									success: function(data) {
										var msg = 'Search completed!\n';
										msg += 'Accepted: ' + (data.accepted ? data.accepted.length : 0) + '\n';
										msg += 'Denied: ' + (data.denied ? data.denied.length : 0);
										alert(msg);
										oTable.ajax.reload();
									},
									error: function(xhr) {
										alert('Error starting search: ' + (xhr.responseText || 'Unknown error'));
									}
								});
							}
						}
					});

					// Handle Search by Title button clicks (movies and serie_episodes tables)
					$(document).on('click', '.btn-action-search-title', function() {
						var id = $(this).data('id');
						var tableName = '%s';
						if (tableName === 'movies') {
							if (confirm('Start search for this movie by Title?')) {
								$.ajax({
									url: '/api/movies/search/list/' + id + '?apikey=%s&searchByTitle=true&download=true',
									type: 'GET',
									success: function(data) {
										var msg = 'Search completed!\n';
										msg += 'Accepted: ' + (data.accepted ? data.accepted.length : 0) + '\n';
										msg += 'Denied: ' + (data.denied ? data.denied.length : 0);
										alert(msg);
										oTable.ajax.reload();
									},
									error: function(xhr) {
										alert('Error starting search: ' + (xhr.responseText || 'Unknown error'));
									}
								});
							}
						} else if (tableName === 'serie_episodes') {
							if (confirm('Start search for this episode by Title?')) {
								$.ajax({
									url: '/api/series/episodes/search/list/' + id + '?apikey=%s&searchByTitle=true&download=true',
									type: 'GET',
									success: function(data) {
										var msg = 'Search completed!\n';
										msg += 'Accepted: ' + (data.accepted ? data.accepted.length : 0) + '\n';
										msg += 'Denied: ' + (data.denied ? data.denied.length : 0);
										alert(msg);
										oTable.ajax.reload();
									},
									error: function(xhr) {
										alert('Error starting search: ' + (xhr.responseText || 'Unknown error'));
									}
								});
							}
						}
					});

					// Handle Metadata Refresh button clicks (dbmovies and dbseries tables)
					$(document).on('click', '.btn-action-metadata-refresh', function() {
						var dbId = $(this).data('id');
						var type = $(this).data('type');
						var apiUrl = type === 'movie' ? '/api/movies/refresh/' : '/api/series/refresh/';

						if (confirm('Refresh metadata for this ' + type + '?')) {
							// First, find the related movie/serie record using dbmovie_id/dbserie_id
							$.ajax({
								url: '/api/admin/tablejson/' + (type === 'movie' ? 'movies' : 'series') + '?apikey=%s',
								type: 'POST',
								data: {
									sSearch: dbId,
									iDisplayStart: 0,
									iDisplayLength: 1
								},
								success: function(response) {
									if (response.aaData && response.aaData.length > 0) {
										var recordId = response.aaData[0][0]; // First column is ID
										// Now call the metadata refresh API
										$.ajax({
											url: apiUrl + recordId + '?apikey=%s',
											type: 'GET',
											success: function(data) {
												alert('Metadata refresh started successfully!');
												oTable.ajax.reload();
											},
											error: function(xhr) {
												alert('Error starting metadata refresh: ' + (xhr.responseText || 'Unknown error'));
											}
										});
									} else {
										alert('No related ' + type + ' record found for this db' + type + ' ID');
									}
								},
								error: function(xhr) {
									alert('Error finding related record: ' + (xhr.responseText || 'Unknown error'));
								}
							});
						}
					});
					`, tableInfo.Name, columnsStr, csrfToken, tableInfo.Name, "", tableInfo.Name, columnsStr, csrfToken, tableInfo.Name, tableInfo.Name, config.GetSettingsGeneral().WebAPIKey, tableInfo.Name, config.GetSettingsGeneral().WebAPIKey, tableInfo.Name, tableInfo.Name, config.GetSettingsGeneral().WebAPIKey, config.GetSettingsGeneral().WebAPIKey, tableInfo.Name, config.GetSettingsGeneral().WebAPIKey, config.GetSettingsGeneral().WebAPIKey, config.GetSettingsGeneral().WebAPIKey, config.GetSettingsGeneral().WebAPIKey)),
			),
		})
}

// Helper function to generate form fields based on data type and column name.
func generateFormField(col, fieldName, displayName string, fieldData any) gomponents.Node {
	// Handle special config dropdown fields
	if col == "quality_profile" || col == "listname" || (col == "quality_type") {
		return generateConfigSelectField(col, fieldName, displayName, fieldData)
	}

	// Handle foreign key fields with AJAX
	if strings.HasSuffix(col, "_id") && col != "id" {
		refTable := getReferenceTable(col)
		if refTable != "" {
			return generateAjaxSelectField(col, fieldName, displayName, fieldData, refTable)
		}
	}

	// Handle different data types
	switch val := fieldData.(type) {
	case bool:
		return RenderFormGroup("",
			"",
			displayName, fieldName, "checkbox", val, nil)

	case string:
		return RenderFormGroup("",
			"",
			displayName, fieldName, "text", val, nil)

	case int:
		// Check if this should be a checkbox (boolean field stored as int)
		if isCheckboxFieldRefactored(col) {
			checked := parseCheckboxValue(fmt.Sprintf("%v", fieldData))
			return RenderFormGroup("",
				"",
				displayName, fieldName, "checkbox", checked, nil)
		}

		return RenderFormGroup("",
			"",
			displayName, fieldName, "number", fmt.Sprintf("%v", fieldData), nil)

	case time.Time:
		valformat := val.Format("2006-01-02")
		return RenderFormGroup("", "", displayName, fieldName, "date", valformat, map[string][]string{
			"class": {"form-control datepicker"},
		})

	case sql.NullTime:
		valformat := val.Time.Format("2006-01-02")
		return RenderFormGroup("", "", displayName, fieldName, "date", valformat, map[string][]string{
			"class": {"form-control datepicker"},
		})

	default:
		return RenderFormGroup("",
			"",
			displayName, fieldName, "text", fieldData, nil)
	}
}

// Generate config select field using SelectField component.
func generateConfigSelectField(col, fieldName, displayName string, fieldData any) gomponents.Node {
	currentValue := ""
	if fieldData != nil {
		currentValue = fmt.Sprintf("%v", fieldData)
	}

	var options []string

	options = append(options, "")

	// Add config options based on field type
	switch col {
	case "quality_profile":
		qualityConfigs := config.GetSettingsQualityAll()
		for _, qc := range qualityConfigs {
			options = append(options, qc.Name)
		}

	case "listname":
		options = append(options, config.GetSettingsMediaListAll()...)
	case "quality_type":
		qualityTypes := map[string]string{
			"1": "Resolution",
			"2": "Quality",
			"3": "Codec",
			"4": "Audio",
		}
		for value := range qualityTypes {
			options = append(options, value)
		}
	}

	return RenderFormGroup("",
		"",
		displayName, fieldName, "select", currentValue, map[string][]string{
			"options": options,
		})
}

// AjaxSelectOptions for AJAX-powered select fields with preselection support.
type AjaxSelectOptions struct {
	ID              string         // HTML ID attribute
	Name            string         // HTML name attribute for form submission
	Label           string         // Label text
	Icon            string         // Icon class for label (e.g., "fas fa-film")
	AjaxURL         string         // URL for AJAX data loading
	SelectedValue   string         // Current selected value ID
	SelectedText    string         // Current selected value display text (if known)
	Placeholder     string         // Placeholder text
	HelpText        string         // Help text below the field
	WrapperClass    string         // CSS class for wrapper div
	StaticOptions   []SelectOption // Static options (e.g., for quality profiles)
	SearchThreshold int            // Minimum characters before search triggers
	AllowClear      bool           // Allow clearing selection
	AllowCustom     bool           // Allow custom values to be typed
	Required        bool           // HTML required attribute
	Disabled        bool           // HTML disabled attribute
}

// Generate AJAX select field using AjaxSelectField component.
func generateAjaxSelectField(
	col, fieldName, displayName string,
	fieldData any,
	refTable string,
) gomponents.Node {
	currentValue := ""
	if fieldData != nil {
		switch v := fieldData.(type) {
		case int, int64, uint, uint64:
			currentValue = fmt.Sprintf("%d", v)
		case float64:
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

	return AjaxSelectField(AjaxSelectOptions{
		ID:              fieldName,
		Name:            fieldName,
		Label:           displayName,
		AjaxURL:         "/api/admin/dropdown/" + refTable + "/" + col,
		SelectedValue:   currentValue,
		Placeholder:     "Select " + displayName + "...",
		AllowClear:      true,
		SearchThreshold: 0,
	})
}

// AjaxSelectField creates a select field with AJAX loading and preselection support
// Following the same pattern as renderFormGroup for consistency.
func AjaxSelectField(options AjaxSelectOptions) gomponents.Node {
	// Determine icon based on type or use provided icon
	iconClass := IconList
	if options.Icon != "" {
		iconClass = options.Icon
	}

	// Create select attributes
	selectAttrs := []gomponents.Node{
		html.ID(options.ID),
		html.Name(options.Name),
		html.Class("form-select choices-ajax"),
	}

	// Add AJAX URL if provided
	if options.AjaxURL != "" {
		selectAttrs = append(selectAttrs, html.Data("ajax-url", options.AjaxURL))
	}

	// Add optional attributes
	if options.Placeholder != "" {
		selectAttrs = append(selectAttrs, html.Data("placeholder", options.Placeholder))
	}

	if options.AllowClear {
		selectAttrs = append(selectAttrs, html.Data("allow-clear", trueValue))
	}

	if options.AllowCustom {
		selectAttrs = append(selectAttrs, html.Data("allow-custom", trueValue))
	}

	if options.SearchThreshold > 0 {
		selectAttrs = append(
			selectAttrs,
			html.Data("search-threshold", fmt.Sprintf("%d", options.SearchThreshold)),
		)
	}

	if options.SelectedValue != "" {
		selectAttrs = append(selectAttrs, html.Data("selected-value", options.SelectedValue))
	}

	if options.Required {
		selectAttrs = append(selectAttrs, html.Required())
	}

	if options.Disabled {
		selectAttrs = append(selectAttrs, html.Disabled())
	}

	// Create select options
	var selectOptions []gomponents.Node

	// Add default empty option
	emptyText := "-- Select --"
	if options.Placeholder != "" {
		emptyText = options.Placeholder
	}

	selectOptions = append(selectOptions, html.Option(
		html.Value(""),
		gomponents.Text(emptyText),
	))

	// Add preselected option if there's a current value
	if options.SelectedValue != "" && options.SelectedText != "" {
		selectOptions = append(selectOptions, html.Option(
			html.Value(options.SelectedValue),
			html.Selected(),
			gomponents.Text(options.SelectedText),
		))
	} else if options.SelectedValue != "" {
		// If we have a value but no text, show "Loading..." until AJAX loads
		selectOptions = append(selectOptions, html.Option(
			html.Value(options.SelectedValue),
			html.Selected(),
			gomponents.Text("Loading..."),
		))
	}

	// Add any static options
	for _, opt := range options.StaticOptions {
		var valueStr string
		if opt.Value != nil {
			valueStr = fmt.Sprintf("%v", opt.Value)
		}

		optAttrs := []gomponents.Node{
			html.Value(valueStr),
			gomponents.Text(opt.Label),
		}
		// Note: Selected field is commented out in shared.SelectOption
		selectOptions = append(selectOptions, html.Option(optAttrs...))
	}

	selectAttrs = append(selectAttrs, selectOptions...)

	input := html.Select(selectAttrs...)

	// Create comment node if help text is provided
	var commentNode gomponents.Node
	if options.HelpText != "" {
		if len(options.HelpText) > commentLengthLimit || strings.Contains(options.HelpText, "\n") {
			// Long help text with collapsible section
			shortComment := options.HelpText
			if len(options.HelpText) > commentLengthLimit {
				lines := strings.Split(options.HelpText, "\n")

				shortComment = strings.TrimSpace(lines[0])
				if len(shortComment) > commentLengthLimit {
					shortComment = shortComment[:commentTruncateLimit] + "..."
				}
			}

			commentNode = html.Div(
				html.Div(
					html.Class("d-flex align-items-center mt-1"),
					html.Small(
						html.Class("text-muted me-2"),
						html.Style("font-size: 0.75em;"),
						gomponents.Text(shortComment),
					),
					html.Button(
						html.Type("button"),
						html.Class("btn btn-outline-info btn-sm"),
						html.Style(
							"font-size: 0.85em; padding: 0.25rem 0.5rem; border-radius: 0.375rem;",
						),
						gomponents.Attr("data-bs-toggle", "collapse"),
						gomponents.Attr("data-bs-target", "#help-"+options.ID),
						gomponents.Attr("aria-expanded", "false"),
						gomponents.Attr("title", "Show detailed help"),
						html.I(html.Class("fas fa-info-circle me-1")),
						gomponents.Text("Help"),
					),
				),
				html.Div(
					html.Class("collapse mt-2"),
					html.ID("help-"+options.ID),
					html.Div(
						html.Class("card card-body bg-light border-0"),
						html.Style("font-size: 0.875em; padding: 0.75rem;"),
						html.Pre(
							html.Class("mb-0 text-wrap"),
							html.Style("white-space: pre-wrap; font-family: inherit;"),
							gomponents.Text(options.HelpText),
						),
					),
				),
			)
		} else {
			// Short single line comment
			commentNode = html.Small(
				html.Class("form-text text-muted"),
				html.Style("font-size: 0.75em; margin-top: 0.25rem;"),
				gomponents.Text(options.HelpText),
			)
		}
	}

	// Get display name, fallback to fieldNameToUserFriendly if not provided
	displayName := options.Label
	if displayName == "" {
		displayName = fieldNameToUserFriendly(options.Name)
	}

	// Return consistent form group structure like renderFormGroup
	// Include JavaScript initialization for AJAX functionality
	return html.Div(
		createFormFieldWrapper(options.ID, iconClass, displayName, input, commentNode),
		// Add CSS to fix remove button icon styling
		html.StyleEl(gomponents.Raw(`
			.choices__button {
				width: 20px !important;
				height: 20px !important;
				padding: 2px !important;
				display: flex !important;
				align-items: center !important;
				justify-content: center !important;
				border: 1px solid #ccc !important;
				border-radius: 2px !important;
				background-size: 12px 12px !important;
				background-position: center !important;
				background-repeat: no-repeat !important;
			}
			.choices__button:hover {
				background-color: #f8f9fa !important;
				border-color: #dc3545 !important;
			}
			.choices__button:focus {
				outline: 2px solid #007bff !important;
				outline-offset: 1px !important;
			}
		`)),
		html.Script(gomponents.Raw(`
			document.addEventListener('DOMContentLoaded', function() {
				// Initialize Choices.js for enhanced selects
				if (window.initChoicesGlobal) {
					window.initChoicesGlobal();
				}
			});
		`)),
	)
}

// createFormFieldWrapper creates a compact form field wrapper with icon and label.
func createFormFieldWrapper(
	id, iconClass, displayName string,
	inputElement, commentNode gomponents.Node,
) gomponents.Node {
	return html.Div(
		html.Class("form-group mb-2"),
		html.Div(
			html.Class("form-field-compact p-2 border rounded"),
			html.Style(
				"background: #ffffff; border: 1px solid #e3e6ea !important; transition: border-color 0.15s ease-in-out;",
			),
			html.Div(
				html.Class("d-flex align-items-center mb-1"),
				html.I(
					html.Class(iconClass+" text-primary me-2"),
					html.Style("font-size: 0.85em;"),
				),
				createFormLabel(id, displayName, false),
			),
			inputElement,
			commentNode,
		),
	)
}

// Helper function to determine if an int field should be rendered as checkbox.
func isCheckboxFieldRefactored(col string) bool {
	checkboxFields := []string{
		"missing", "blacklisted", "dont_search", "dont_upgrade", "use_regex",
		"proper", "extended", "repack", "ignore_runtime", "adult",
		"search_specials", "quality_reached",
	}
	for _, field := range checkboxFields {
		if col == field {
			return true
		}
	}

	return false
}

// RenderFormGroup creates form group with proper input type handling.
func RenderFormGroup(
	group, comment, displayName, name, inputType string,
	value any,
	options map[string][]string,
) gomponents.Node {
	classelement := ClassFormControl
	if inputType == fieldTypeSelect || inputType == fieldTypeSelectArray {
		classelement = "form-select"
		if inputType == fieldTypeSelectArray {
			classelement += " selectpicker"
		}
	}

	classelement, addDetails := parseFormOptions(options, classelement)

	id := name
	if group != "" {
		id = group + "_" + name
	}

	iconClass := getInputIcon(inputType)
	input := renderInput(inputType, group, id, classelement, value, options, addDetails)
	commentNode := createCommentNode(comment, group, name)

	if displayName == "" {
		displayName = fieldNameToUserFriendly(name)
	}

	return createFormLayout(inputType, id, displayName, iconClass, input, commentNode)
}

// parseCheckboxValue safely parses checkbox values, handling corrupted "on" strings.
func parseCheckboxValue(value string) bool {
	switch value {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no", "":
		return false
	default:
		// For any other value, try standard ParseBool
		result, _ := strconv.ParseBool(value)
		return result
	}
}

// parseFormOptions processes the options map and returns additional HTML attributes.
func parseFormOptions(options map[string][]string, baseClass string) (string, []gomponents.Node) {
	classelement := baseClass

	var addDetails []gomponents.Node

	if len(options) >= 1 {
		for i, val := range options {
			switch i {
			case "class":
				classelement = val[0]
			case "style":
				addDetails = append(addDetails, html.Style(val[0]))
			case "rows":
				addDetails = append(addDetails, html.Rows(val[0]))
			case "required":
				if strings.EqualFold(val[0], "True") {
					addDetails = append(addDetails, html.Required())
				}

			case "readonly":
				if strings.EqualFold(val[0], "True") {
					addDetails = append(addDetails, html.ReadOnly())
				}

			case "min":
				addDetails = append(addDetails, html.Min(val[0]))
			case "max":
				addDetails = append(addDetails, html.Max(val[0]))
			case "pattern":
				addDetails = append(addDetails, html.Pattern(val[0]))
			case "data-toggle":
				addDetails = append(addDetails, html.Data("toggle", val[0]))
			case "title":
				addDetails = append(addDetails, html.Title(val[0]))
			case AttrHxGet:
				addDetails = append(addDetails, htmx.Get(val[0]))
			case AttrHxTarget:
				addDetails = append(addDetails, htmx.Target(val[0]))
			case AttrHxTrigger:
				addDetails = append(addDetails, htmx.Trigger(val[0]))
			case AttrHxVals:
				addDetails = append(addDetails, htmx.Vals(val[0]))
			case AttrHxHeaders:
				addDetails = append(addDetails, htmx.Headers(val[0]))
			case AttrHxIndicator:
				addDetails = append(addDetails, htmx.Indicator(val[0]))
			case AttrHxDelete:
				addDetails = append(addDetails, htmx.Delete(val[0]))
			case AttrHxConfirm:
				addDetails = append(addDetails, htmx.Confirm(val[0]))
			case AttrHxOnAfterRequest:
				addDetails = append(addDetails, htmx.On("after-request", val[0]))
			case AttrHxPatch:
				addDetails = append(addDetails, htmx.Patch(val[0]))
			case AttrHxPost:
				addDetails = append(addDetails, htmx.Post(val[0]))
			case AttrHxPut:
				addDetails = append(addDetails, htmx.Put(val[0]))
			case "onchange":
				addDetails = append(addDetails, htmx.On("change", val[0]))
			case "placeholder":
				addDetails = append(addDetails, html.Placeholder(val[0]))
			}
		}
	}

	return classelement, addDetails
}

// getInputIcon returns the appropriate FontAwesome icon for the input type.
func getInputIcon(inputType string) string {
	switch inputType {
	case fieldTypeText:
		return IconFont
	case "date":
		return IconCalendarDays
	case "number":
		return IconHashtag
	case "select", "selectarray":
		return IconList
	case "checkbox":
		return IconToggleOn
	case "textarea":
		return IconAlignLeft
	case "array", "arrayselect", "arrayselectarray", "arrayint":
		return IconLayerGroup
	default:
		return IconCog
	}
}

// renderInput creates the appropriate input element based on type.
func renderInput(
	inputType, group, id, classelement string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	switch inputType {
	case "removebutton":
		return createRemoveButton(false)
	case "selectarray":
		return createSelectArrayInput(id, classelement, value, options)
	case "select":
		return createSelectInput(id, classelement, value, options, addDetails)
	case fieldTypeCheckbox:
		return createCheckboxInput(id, value)
	case "textarea":
		return createTextareaInput(id, classelement, value, options, addDetails)
	case "date":
		return createDateInput(id, classelement, value, addDetails)
	case fieldTypeText:
		return createTextInput(id, classelement, value, addDetails)
	case "number":
		return createNumberInput(id, value, addDetails)
	case "array":
		return createArrayInput(group, id, classelement, value, options, addDetails)
	case "arrayselectarray":
		return createArraySelectArrayInput(id, value, options, addDetails)
	case "arrayselect":
		return createArraySelectInput(id, value, options, addDetails)
	case "arrayint":
		return createArrayIntInput(group, id, classelement, value, addDetails)
	default:
		return createFormField(inputType, id, "", "", addDetails)
	}
}

// createSelectArrayInput creates a multi-select dropdown.
func createSelectArrayInput(
	id, classelement string,
	value any,
	options map[string][]string,
) gomponents.Node {
	var optionElements []gomponents.Node
	if opts, ok := options[keyOptions]; ok {
		values, ok := value.([]string)
		if !ok {
			values = nil
		}

		labels, hasLabels := options["labels"]

		if hasLabels && len(labels) == len(opts) {
			for i, opt := range opts {
				selected := slices.Contains(values, opt)
				displayText := labels[i]

				optionElements = append(optionElements, createOption(opt, displayText, selected))
			}
		} else {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for i, opt := range opts2 {
				selected := slices.Contains(values, opt)

				displayText := opt
				if hasLabels && i < len(labels) {
					displayText = labels[i]
				}

				optionElements = append(optionElements, createOption(opt, displayText, selected))
			}
		}
	}

	return html.Select(
		html.Class(classelement+" choices-multiple"),
		html.Multiple(),
		html.Data("live-search", "true"),
		html.Data("choices", "multiple"),
		html.Data("choices-multiple", "true"),
		html.Data("choices-remove-item-button", "true"),
		html.Data("choices-search-enabled", "true"),
		html.Data("choices-search-placeholder", "Search options..."),
		html.Data("choices-no-results-text", "No results found"),
		html.Data("choices-no-choices-text", "No choices available"),
		html.Data("choices-placeholder", "Select options..."),
		html.ID(id),
		html.Name(id),
		gomponents.Group(optionElements),
	)
}

// createSelectInput creates a single select dropdown.
func createSelectInput(
	id, classelement string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	var optionElements []gomponents.Node
	if opts, ok := options[keyOptions]; ok {
		labels, hasLabels := options["labels"]

		if hasLabels && len(labels) == len(opts) {
			for i, opt := range opts {
				selected := opt == fmt.Sprintf("%v", value)
				displayText := labels[i]

				optionElements = append(optionElements, createOption(opt, displayText, selected))
			}
		} else {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for i, opt := range opts2 {
				selected := opt == fmt.Sprintf("%v", value)

				displayText := opt
				if hasLabels && i < len(labels) {
					displayText = labels[i]
				}

				optionElements = append(optionElements, createOption(opt, displayText, selected))
			}
		}
	}

	selectattr := append([]gomponents.Node{
		html.Class(classelement),
		html.ID(id),
		html.Name(id),
	}, addDetails...)

	selectattr = append(selectattr, gomponents.Group(optionElements))

	return html.Select(selectattr...)
}

// createCheckboxInput creates a checkbox input.
func createCheckboxInput(id string, value any) gomponents.Node {
	baseAttrs := []gomponents.Node{
		html.Class("form-check-input-modern"),
		html.Type(fieldTypeCheckbox),
		html.Role("switch"),
		html.ID(id),
		html.Name(id),
	}

	if val, ok := value.(bool); ok && val {
		baseAttrs = append(baseAttrs, html.Checked())
	}

	return html.Input(baseAttrs...)
}

// createTextareaInput creates a textarea input.
func createTextareaInput(
	id, classelement string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	baseAttrs := []gomponents.Node{
		html.Class(classelement),
		html.ID(id),
		html.Name(id),
		html.Value(fmt.Sprintf("%v", value)),
	}

	if !hasRowsOption(options) {
		baseAttrs = append(baseAttrs, html.Rows("3"))
	}

	return html.Textarea(append(baseAttrs, addDetails...)...)
}

// hasRowsOption checks if rows option is specified.
func hasRowsOption(options map[string][]string) bool {
	for _, option := range options {
		if len(option) == 0 {
			continue
		}

		for key := range options {
			if key == "rows" {
				return true
			}
		}
	}

	return false
}

// createDateInput creates a date input.
func createDateInput(
	id, classelement string,
	value any,
	addDetails []gomponents.Node,
) gomponents.Node {
	return html.Input(
		append([]gomponents.Node{
			html.Class(classelement),
			html.ID(id),
			html.Name(id),
			html.Type("date"),
			html.Value(fmt.Sprintf("%v", value)),
		}, addDetails...)...,
	)
}

// createTextInput creates a text input.
func createTextInput(
	id, classelement string,
	value any,
	addDetails []gomponents.Node,
) gomponents.Node {
	return html.Input(
		append([]gomponents.Node{
			html.Class(classelement),
			html.Type("text"),
			html.ID(id),
			html.Name(id),
			html.Value(fmt.Sprintf("%v", value)),
		}, addDetails...)...,
	)
}

// createNumberInput creates a number input with proper type conversion.
func createNumberInput(id string, value any, addDetails []gomponents.Node) gomponents.Node {
	var setvalue string
	switch val := value.(type) {
	case int, int64, int32, int16, int8:
		setvalue = fmt.Sprintf("%d", val)
	case uint, uint64, uint32, uint16, uint8:
		setvalue = fmt.Sprintf("%d", val)
	case float64, float32:
		setvalue = fmt.Sprintf("%f", val)
	}

	return createFormField(fieldTypeNumber, id, setvalue, "", addDetails)
}

// createArrayInput creates array text inputs with add/remove functionality.
func createArrayInput(
	group, id, classelement string,
	value any,
	_ map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	values, ok := value.([]string)
	if !ok {
		values = nil
	}

	var nodes []gomponents.Node

	for _, v := range values {
		nodes = append(nodes, createArrayRow(id, classelement, fieldTypeText, v, addDetails))
	}

	nodes = append(nodes, createArrayAddButton(group, id, "addArrayItem", fieldTypeText))

	return html.Div(
		html.ID(id+"-container"),
		gomponents.Group(nodes),
	)
}

// createArrayIntInput creates array number inputs.
func createArrayIntInput(
	group, id, classelement string,
	value any,
	addDetails []gomponents.Node,
) gomponents.Node {
	values, ok := value.([]int)
	if !ok {
		values = nil
	}

	var nodes []gomponents.Node

	for _, v := range values {
		nodes = append(
			nodes,
			createArrayRow(id, classelement, fieldTypeNumber, strconv.Itoa(v), addDetails),
		)
	}

	nodes = append(nodes, createArrayAddButton(group, id, "addArrayIntItem", fieldTypeNumber))

	return html.Div(
		html.ID(id+"-container"),
		gomponents.Group(nodes),
	)
}

// createArrayRow creates a single row in an array input with Bootstrap input group.
func createArrayRow(
	id, classelement, inputType, value string,
	addDetails []gomponents.Node,
) gomponents.Node {
	return html.Div(
		html.Class("input-group mb-2"),
		html.Input(
			append([]gomponents.Node{
				html.Class(classelement),
				html.Type(inputType),
				html.Name(id),
				html.Value(value),
			}, addDetails...)...,
		),
		html.Button(
			html.Class("btn btn-outline-danger"),
			html.Type("button"),
			gomponents.Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			gomponents.Attr("title", "Remove item"),
			html.I(html.Class(IconTimes)),
		),
	)
}

// createArrayAddButton creates add button for array inputs.
func createArrayAddButton(group, id, funcName, inputType string) gomponents.Node {
	parts := strings.Split(id, "_")
	name := parts[len(parts)-1]
	// group := ""
	// if len(parts) > 1 {
	// 	group = parts[0]
	// }

	return gomponents.Group([]gomponents.Node{
		html.Button(
			html.Class(ClassBtnPrimary),
			html.Type("button"),
			gomponents.Attr(
				"onclick",
				fmt.Sprintf("%s%s('%s', '%s')", funcName, name, group, name),
			),
			gomponents.Text("Add Item"),
		),
		html.Script(gomponents.Rawf(`
			function %s%s(group, name) {
				const container = document.getElementById(group + '_' + name + '-container');
				const newRow = document.createElement('div');
				newRow.className = 'input-group mb-2';
				newRow.innerHTML = '<input class="form-control" type="%s" name="%s"><button class="btn btn-outline-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()" title="Remove item"><i class=IconTimes></i></button>';
				container.insertBefore(newRow, container.lastElementChild);
			}
		`, funcName, name, inputType, id)),
	})
}

// createArraySelectArrayInput creates array of select inputs.
func createArraySelectArrayInput(
	id string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	optionString := buildOptionString(options)

	values, ok := value.([]string)
	if !ok {
		values = nil
	}

	var nodes []gomponents.Node

	for _, v := range values {
		nodes = append(nodes, createArraySelectRow(id, v, options, addDetails))
	}

	nodes = append(nodes, createArraySelectAddButton(id, "addArraySelectArrayItem", optionString))

	return html.Div(
		html.ID(id+"-container"),
		gomponents.Group(nodes),
	)
}

// createArraySelectInput creates single array select input.
func createArraySelectInput(
	id string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	var optionElements []gomponents.Node
	if opts, ok := options[keyOptions]; ok {
		var values []string
		if val, ok := value.([]string); ok {
			values = val
		}

		opts2 := sort.StringSlice(opts)
		opts2.Sort()

		for _, opt := range opts2 {
			var selected bool
			switch val := value.(type) {
			case []string:
				selected = slices.Contains(values, opt)
			case string:
				selected = opt == val
			}

			optionElements = append(optionElements, createOption(opt, opt, selected))
		}
	}

	return html.Div(
		html.ID(id+"-container"),
		html.Div(
			html.Class(ClassDFlex),
			html.Select(
				append([]gomponents.Node{
					html.Class(ClassFormSelect),
					html.Name(id),
					gomponents.Group(optionElements),
				}, addDetails...)...,
			),
		),
	)
}

// buildOptionString builds HTML option string for JavaScript.
func buildOptionString(options map[string][]string) string {
	var optionString string
	if opts, ok := options[keyOptions]; ok {
		opts2 := sort.StringSlice(opts)
		opts2.Sort()

		for _, opt := range opts2 {
			optionString += "<option value=\"" + opt + "\">" + opt + "</option>"
		}
	}

	return optionString
}

// createArraySelectRow creates a row with select dropdown.
func createArraySelectRow(
	id, value string,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	var optionElements []gomponents.Node
	if opts, ok := options[keyOptions]; ok {
		opts2 := sort.StringSlice(opts)
		opts2.Sort()

		for _, opt := range opts2 {
			selected := opt == value

			optionElements = append(optionElements, createOption(opt, opt, selected))
		}
	}

	return html.Div(
		html.Class("input-group mb-2"),
		html.Select(
			append([]gomponents.Node{
				html.Class(ClassFormSelect),
				html.Name(id),
				gomponents.Group(optionElements),
			}, addDetails...)...,
		),
		html.Button(
			html.Class("btn btn-outline-danger"),
			html.Type("button"),
			gomponents.Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			gomponents.Attr("title", "Remove item"),
			html.I(html.Class(IconTimes)),
		),
	)
}

// createArraySelectAddButton creates add button for array select inputs.
func createArraySelectAddButton(id, funcName, optionString string) gomponents.Node {
	parts := strings.Split(id, "_")
	name := parts[len(parts)-1]

	group := ""
	if len(parts) > 1 {
		group = parts[0]
	}

	return gomponents.Group([]gomponents.Node{
		html.Button(
			html.Class(ClassBtnPrimary),
			html.Type("button"),
			gomponents.Attr(
				"onclick",
				fmt.Sprintf("%s%s('%s', '%s')", funcName, name, group, name),
			),
			gomponents.Text("Add Item"),
		),
		html.Script(gomponents.Rawf(`
			function %s%s(group, name) {
				const container = document.getElementById(group + '_' + name + '-container');
				const newRow = document.createElement('div');
				newRow.className = 'input-group mb-2';
				newRow.innerHTML = '<select class="form-select" name="%s">%s</select><button class="btn btn-outline-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()" title="Remove item"><i class=IconTimes></i></button>';
				container.insertBefore(newRow, container.lastElementChild);
			}
		`, funcName, name, id, optionString)),
	})
}

// createCommentNode creates comment display node.
func createCommentNode(comment, group, name string) gomponents.Node {
	if comment == "" {
		return nil
	}

	shortComment := comment
	if len(comment) > commentLengthLimit {
		lines := strings.Split(comment, "\n")

		shortComment = strings.TrimSpace(lines[0])
		if len(shortComment) > commentLengthLimit {
			shortComment = shortComment[:commentTruncateLimit] + "..."
		}
	}

	if len(comment) > commentLengthLimit || strings.Contains(comment, "\n") {
		return createCollapsibleComment(shortComment, comment, group, name)
	}

	return html.Small(
		html.Class("form-text text-muted"),
		html.Style(
			"font-size: 0.75em; margin-top: 0.25rem; word-wrap: break-word; overflow-wrap: anywhere;",
		),
		gomponents.Raw(formatCommentText(comment)),
	)
}

// createCollapsibleComment creates collapsible comment with help button.
func createCollapsibleComment(shortComment, comment, group, name string) gomponents.Node {
	return html.Div(
		html.Div(
			html.Class("d-flex align-items-center mt-1"),
			html.Small(
				html.Class("text-muted me-2"),
				html.Style("font-size: 0.75em; word-wrap: break-word; overflow-wrap: anywhere;"),
				gomponents.Raw(formatCommentText(shortComment)),
			),
			html.Button(
				html.Type(attrTypeButton),
				html.Class("btn btn-outline-info btn-sm"),
				html.Style("font-size: 0.85em; padding: 0.25rem 0.5rem; border-radius: 0.375rem;"),
				gomponents.Attr("data-bs-toggle", "collapse"),
				gomponents.Attr("data-bs-target", "#help-"+group+"-"+name),
				gomponents.Attr("aria-expanded", "false"),
				gomponents.Attr(attrTitle, "Show detailed help"),
				html.I(html.Class("fas fa-info-circle")),
			),
		),
		html.Div(
			html.ID("help-"+group+"-"+name),
			html.Class("collapse mt-2"),
			html.Div(
				html.Class("alert alert-info alert-outline"),
				html.Role("alert"),
				html.Div(
					html.Class("alert-icon"),
					html.I(html.Class("far fa-fw fa-bell")),
				),
				html.Div(
					html.Class("alert-message"),
					html.Style(
						"font-size: 0.85em; padding: 0.75rem; margin: 0; border-radius: 0.375rem; word-wrap: break-word; overflow-wrap: anywhere;",
					),
					gomponents.Raw(formatCommentText(comment)),
				),
			),
		),
	)
}

// formatCommentText formats comment text by escaping HTML and converting line breaks to <br> tags.
func formatCommentText(text string) string {
	// First escape HTML to prevent XSS
	escaped := stdhtml.EscapeString(text)

	// Convert \n to <br> tags for proper line breaks in HTML
	// Handle different line break formats
	formatted := strings.ReplaceAll(escaped, "\r\n", "<br>") // Windows CRLF

	formatted = strings.ReplaceAll(formatted, "\r", "<br>") // Mac CR
	formatted = strings.ReplaceAll(formatted, "\n", "<br>") // Unix LF

	// Also handle literal \n strings (in case they're stored as literal text)
	formatted = strings.ReplaceAll(formatted, "\\n", "<br>")

	// Handle multiple consecutive line breaks
	formatted = strings.ReplaceAll(
		formatted,
		"<br><br><br>",
		"<br><br>",
	) // Reduce triple breaks to double

	return formatted
}

// createFormLayout creates the final form layout.
func createFormLayout(
	inputType, id, displayName, iconClass string,
	input, commentNode gomponents.Node,
) gomponents.Node {
	if inputType == "checkbox" {
		return html.Div(
			html.Class("form-group mb-4"),
			html.Div(
				html.Class("form-check-wrapper p-3 border rounded-3 bg-light"),
				html.Style(
					"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: 1px solid #dee2e6 !important; transition: all 0.2s ease;",
				),
				html.Div(
					html.Class("form-check form-switch"),
					html.I(html.Class(iconClass+" text-primary me-2")),
					input,
					createFormLabel(id, displayName, true),
				),
				commentNode,
			),
		)
	}

	return html.Div(
		html.Class("form-group mb-2"),
		html.Div(
			html.Class("form-field-compact p-2 border rounded"),
			html.Style(
				"background: #ffffff; border: 1px solid #e3e6ea !important; transition: border-color 0.15s ease-in-out;",
			),
			html.Div(
				html.Class("d-flex align-items-center mb-1"),
				html.I(
					html.Class(iconClass+" text-primary me-2"),
					html.Style("font-size: 0.85em;"),
				),
				createFormLabel(id, displayName, false),
			),
			input,
			commentNode,
		),
	)
}

// parseIntOrDefault parses int with default value.
func ParseIntOrDefault(s string, defaultVal int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return defaultVal
}

func ExtractFormKeys(c *gin.Context, prefix, fieldSuffix string) map[string]bool {
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, fieldSuffix) || !strings.Contains(key, prefix) {
			continue
		}

		parts := strings.Split(key, separatorUnderscore)
		if len(parts) <= 1 {
			continue
		}

		keyIndex := ""
		// Find the correct index position based on prefix structure
		if strings.Contains(prefix, "media_main_") {
			if len(parts) > fieldNameIndexPosition {
				keyIndex = parts[fieldNameIndexPosition] // media_main_movies_X_Name -> X
			}
		} else {
			keyIndex = parts[1] // downloader_X_Name -> X
		}

		if keyIndex != "" {
			formKeys[keyIndex] = true
		}
	}

	return formKeys
}

// CreateAddButton creates an add button for forms.
func CreateAddButton(
	addNewLabel, hxTarget, hxPost, hxHeaders string,
) gomponents.Node {
	attrs := []gomponents.Node{
		html.Type(attrTypeButton),
		html.Class("btn btn-success btn-sm mb-3"),
		html.Data("add-new", "true"),
		html.I(html.Class("fas fa-plus me-1")),
		gomponents.Text(addNewLabel),
	}

	if hxPost != "" {
		attrs = append(attrs, htmx.Post(hxPost))
	}

	if hxTarget != "" {
		attrs = append(attrs, htmx.Target(hxTarget))
		// Use beforeend to append new items inside the container instead of replacing existing content
		attrs = append(attrs, htmx.Swap("beforeend"))
	}

	if hxHeaders != "" {
		// Use the existing CreateHTMXHeaders function for proper CSRF token formatting
		attrs = append(attrs, htmx.Headers(CreateHTMXHeaders(hxHeaders)))
	}

	return html.Button(attrs...)
}

// createHTMXHeaders creates standardized HTMX headers with CSRF token.
func CreateHTMXHeaders(csrfToken string) string {
	return "{\"X-CSRF-Token\": \"" + csrfToken + "\"}"
}

// CreateFormSubmitGroup creates a submit button group for forms.
func CreateFormSubmitGroup(
	submitText, submitIcon, hxPost, hxHeaders string,
) gomponents.Node {
	btnAttrs := []gomponents.Node{
		html.Type("submit"),
		html.Class("btn btn-primary"),
	}

	if submitIcon != "" {
		btnAttrs = append(btnAttrs, html.I(html.Class(submitIcon+" me-1")))
	}

	if submitText != "" {
		btnAttrs = append(btnAttrs, gomponents.Text(submitText))
	} else {
		btnAttrs = append(btnAttrs, gomponents.Text("Save Configuration"))
	}

	if hxPost != "" {
		btnAttrs = append(btnAttrs, htmx.Swap("innerHTML"))
		btnAttrs = append(btnAttrs, htmx.Post(hxPost))
	}

	if hxHeaders != "" {
		// Use the existing CreateHTMXHeaders function for proper CSRF token formatting
		btnAttrs = append(btnAttrs, htmx.Headers(CreateHTMXHeaders(hxHeaders)))
	}

	return html.Div(
		html.Class("form-group mt-3"),
		html.Button(btnAttrs...),
	)
}

type SelectOption struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

// CreateImdbConfigFields creates IMDB configuration fields.
func CreateImdbConfigFields(configv *config.ImdbConfig) []FormFieldDefinition {
	return []FormFieldDefinition{
		{
			Name:         "Indexedtypes",
			Type:         "selectarray",
			DefaultValue: configv.Indexedtypes,
			Options: []SelectOption{
				{Value: "movie", Label: "movie"},
				{Value: "tvMovie", Label: "tvMovie"},
				{Value: "tvmovie", Label: "tvmovie"},
				{Value: "tvSeries", Label: "tvSeries"},
				{Value: "tvseries", Label: "tvseries"},
				{Value: "video", Label: "video"},
			},
		},
		{Name: "Indexedlanguages", Type: "array", DefaultValue: configv.Indexedlanguages},
		{Name: "Indexfull", Type: "checkbox", DefaultValue: configv.Indexfull},
		{Name: "ImdbIDSize", Type: "number", DefaultValue: configv.ImdbIDSize},
		{Name: "LoopSize", Type: "number", DefaultValue: configv.LoopSize},
		{Name: "UseMemory", Type: "checkbox", DefaultValue: configv.UseMemory},
		{Name: "UseCache", Type: "checkbox", DefaultValue: configv.UseCache},
	}
}

// renderFormFields renders a list of form fields.
func RenderFormFields(
	group string,
	comments, displayNames map[string]string,
	fields []FormFieldDefinition,
) []gomponents.Node {
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		// Convert SelectOption slice to map[string][]string for compatibility
		optionsMap := make(map[string][]string)
		if len(field.Options) > 0 {
			options := make([]string, len(field.Options))
			for i, opt := range field.Options {
				if strVal, ok := opt.Value.(string); ok {
					options[i] = strVal
				}
			}

			optionsMap["options"] = options
		}

		formGroups = append(
			formGroups,
			RenderFormGroup(
				group,
				comments[field.Name],
				displayNames[field.Name],
				field.Name,
				field.Type,
				field.DefaultValue,
				optionsMap,
			),
		)
	}

	return formGroups
}

// Optimize string operations with a string builder pool.
var StringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// getStringBuilder gets a string builder from a pool (simplified implementation).
func GetStringBuilder() *strings.Builder {
	sb, ok := StringBuilderPool.Get().(*strings.Builder)
	if !ok {
		return &strings.Builder{}
	}

	sb.Reset()

	return sb
}

// putStringBuilder returns a string builder to the pool (simplified implementation).
func PutStringBuilder(builder *strings.Builder) {
	StringBuilderPool.Put(builder)
}

// parseUintOrDefault parses uint with default value.
func ParseUintOrDefault(s string, defaultVal uint) uint {
	if s == "" {
		return defaultVal
	}

	if parsed, err := strconv.ParseUint(s, decimalBase, 32); err == nil {
		return uint(parsed)
	}

	return defaultVal
}

// getCSRFToken extracts CSRF token from gin context if available.
func GetCSRFToken(c *gin.Context) string {
	if token, exists := c.Get("csrf_token"); exists {
		if str, ok := token.(string); ok {
			return str
		}
	}

	return ""
}

func BindJSONWithValidation(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{logger.StatusError: err.Error()})
		return false
	}

	return true
}
