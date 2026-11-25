package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderSchedulerPage renders the scheduler monitoring page.
func renderSchedulerPage(ctx *gin.Context) {
	pageNode := page("Scheduler Monitor", false, false, true, renderSchedulerGrid(getCSRFToken(ctx)))

	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// scheduleActionButtons renders "Add to queue" plus the Stop button of an
// active schedule or the Start button of a stopped one.
func scheduleActionButtons(id any, job string, paused bool) gomponents.Node {
	toggle := scheduleActionButton(id, job, "stop", "Stop", "fas fa-stop", "btn-outline-danger")
	if paused {
		toggle = scheduleActionButton(id, job, "start", "Start", "fas fa-play", "btn-outline-success")
	}

	return html.Div(
		html.Class("d-inline-flex gap-2"),
		scheduleActionButton(id, job, "queue", "Add to queue", "fas fa-plus", "btn-outline-primary"),
		toggle,
	)
}

func scheduleActionButton(id any, job, action, label, icon, class string) gomponents.Node {
	return html.Button(
		html.Class("btn btn-sm text-nowrap schedule-action-btn "+class),
		html.Style("border-radius: 20px; padding: 0.4rem 1rem;"),
		html.Type("button"),
		html.Data("schedule-id", fmt.Sprintf("%v", id)),
		html.Data("action", action),
		html.Data("job", job),
		gomponents.Attr("aria-label", label+": "+job),
		html.I(html.Class(icon+" me-1"), gomponents.Attr("aria-hidden", "true")),
		gomponents.Text(label),
	)
}

// renderSchedulerGrid creates a grid showing scheduler status.
func renderSchedulerGrid(csrfToken string) gomponents.Node {
	schedules := worker.GetSchedules()
	schedulerData := make([]map[string]any, 0, len(schedules))

	for i := range schedules {
		schedulerData = append(schedulerData, map[string]any{
			"id":        i,
			"job":       schedules[i].JobName,
			"lastrun":   schedules[i].LastRun.Format("2006-01-02 15:04:05"),
			"nextrun":   schedules[i].NextRun.Format("2006-01-02 15:04:05"),
			"isrunning": schedules[i].IsRunning,
			"paused":    schedules[i].Paused,
		})
	}

	// Sort by job name
	slices.SortFunc(schedulerData, func(a, b map[string]any) int {
		return strings.Compare(a["job"].(string), b["job"].(string))
	})

	var rows []gomponents.Node
	for _, item := range schedulerData {
		isRunning := item["isrunning"].(bool)
		paused := item["paused"].(bool)

		nextRun := fmt.Sprintf("%v", item["nextrun"])
		if paused {
			nextRun = "—"
		}

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
					gomponents.Text(nextRun),
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

					if paused {
						return html.Span(
							html.Class("badge bg-danger px-3 py-2"),
							html.Style("border-radius: 15px;"),
							html.I(html.Class("fas fa-stop me-1")),
							gomponents.Text("Stopped"),
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
			html.Td(
				html.Style("padding: 1rem; vertical-align: middle; text-align: center;"),
				scheduleActionButtons(item["id"], fmt.Sprintf("%v", item["job"]), paused),
			),
		))
	}

	if len(rows) == 0 {
		rows = append(rows, html.Tr(
			html.Td(
				html.ColSpan("6"),
				html.Class("text-center text-muted"),
				gomponents.Text("No scheduled jobs found"),
			),
		))
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		// gmdApiProxyFetch reads its CSRF token from here.
		html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
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
				// Auto-refresh every 60 seconds
				setInterval(function() {
					window.location.reload();
				}, 60000);

				document.addEventListener('click', function(e) {
					var btn = e.target.closest ? e.target.closest('.schedule-action-btn') : null;
					if (!btn) return;
					var action = btn.getAttribute('data-action');
					btn.disabled = true;
					gmdApiProxyFetch('/api/scheduler/' + action + '/' + btn.getAttribute('data-schedule-id'))
					.then(function(r) {
						return r.json().then(function(data) {
							if (!r.ok) throw new Error(data.error || ('HTTP ' + r.status));
						});
					})
					.then(function() {
						var done = {stop: ' stopped', start: ' started', queue: ' added to queue'};
						showToaster('success', btn.getAttribute('data-job') + done[action]);
						window.location.reload();
					})
					.catch(function(err) {
						btn.disabled = false;
						var what = {stop: 'stop schedule', start: 'start schedule', queue: 'add to queue'};
						showToaster('error', 'Failed to ' + what[action] + ': ' + err.message);
					});
				});
			`),
		),
	)
}
