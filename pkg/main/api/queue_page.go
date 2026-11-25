package api

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	htmx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderQueuePage renders the queue monitoring page.
func renderQueuePage(ctx *gin.Context) {
	pageNode := page("Queue Monitor", false, false, true, renderQueueGrid())

	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderQueuePartial returns just the live queue card fragment for HTMX polling.
func renderQueuePartial(ctx *gin.Context) {
	var buf strings.Builder
	renderQueueCard().Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderQueueGrid creates the queue monitoring page: a static header plus a
// live region that is refreshed in place via HTMX (no full-page reload).
func renderQueueGrid() gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),
		queueHeader(),
		html.Div(
			html.ID("queue-refresh-region"),
			renderQueueCard(),
		),
		queueScript(),
	)
}

// queueHeader renders the static page header with manual refresh / pause controls.
func queueHeader() gomponents.Node {
	return html.Div(
		html.Class("page-header-enhanced"),
		html.Div(
			html.Class("header-content"),
			html.Div(
				html.Class("header-icon-wrapper"),
				html.I(
					html.Class("fas fa-tasks header-icon"),
					gomponents.Attr("aria-hidden", "true"),
				),
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
			html.Div(
				html.Class("ms-auto d-flex align-items-center gap-2"),
				html.Button(
					html.Type("button"),
					html.ID("queue-pause-btn"),
					html.Class("btn btn-outline-secondary btn-sm"),
					gomponents.Attr("aria-label", "Pause automatic queue refresh"),
					html.I(
						html.Class("fas fa-pause me-1"),
						gomponents.Attr("aria-hidden", "true"),
					),
					html.Span(html.ID("queue-pause-label"), gomponents.Text("Pause")),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-outline-secondary btn-sm"),
					gomponents.Attr("aria-label", "Refresh queue now"),
					htmx.Get("/api/admin/queue/partial"),
					htmx.Target("#queue-refresh-region"),
					htmx.Swap("innerHTML"),
					html.I(
						html.Class("fas fa-sync-alt me-1"),
						gomponents.Attr("aria-hidden", "true"),
					),
					gomponents.Text("Refresh"),
				),
			),
		),
	)
}

// renderQueueCard renders the live data card. It is the fragment returned by the
// HTMX partial endpoint and swapped into #queue-refresh-region on each poll.
func renderQueueCard() gomponents.Node {
	queues := worker.GetQueues()

	// Map order is random; sort so rows don't reshuffle on every poll.
	ids := make([]uint32, 0, len(queues))
	for id := range queues {
		ids = append(ids, id)
	}

	slices.SortFunc(ids, func(x, y uint32) int {
		if c := queues[x].Added.Compare(queues[y].Added); c != 0 {
			return c
		}

		return cmp.Compare(x, y)
	})

	queueData := make([]map[string]any, 0, len(queues))

	for _, i := range ids {
		value := queues[i]
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
					gomponents.Attr("aria-label", fmt.Sprintf("Cancel job #%v", item["id"])),
					html.I(
						html.Class("fas fa-times me-1"),
						gomponents.Attr("aria-hidden", "true"),
					),
					gomponents.Text("Cancel"),
				),
			),
		))
	}

	return html.Div(
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
						if len(rows) == 0 {
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
											gomponents.Attr("scope", "col"),
											html.Style(
												"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
											),
											gomponents.Text("ID"),
										),
										html.Th(
											gomponents.Attr("scope", "col"),
											html.Style(
												"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
											),
											gomponents.Text("Queue"),
										),
										html.Th(
											gomponents.Attr("scope", "col"),
											html.Style(
												"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
											),
											gomponents.Text("Job"),
										),
										html.Th(
											gomponents.Attr("scope", "col"),
											html.Style(
												"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
											),
											gomponents.Text("Added"),
										),
										html.Th(
											gomponents.Attr("scope", "col"),
											html.Style(
												"border-top: none; color: #495057; font-weight: 600; padding: 1rem;",
											),
											gomponents.Text("Started"),
										),
										html.Th(
											gomponents.Attr("scope", "col"),
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
	)
}

// queueScript wires up HTMX-based live refresh (with pause/resume) and the
// cancel-job action, replacing the previous full-page reload.
func queueScript() gomponents.Node {
	return html.Script(gomponents.Raw(`
		(function() {
			var REGION = '#queue-refresh-region';
			var INTERVAL = 10000;
			var paused = false;

			function poll() {
				if (paused) return;
				if (document.querySelector(REGION) && window.htmx) {
					htmx.ajax('GET', '/api/admin/queue/partial', {target: REGION, swap: 'innerHTML'});
				}
			}
			setInterval(poll, INTERVAL);

			var pauseBtn = document.getElementById('queue-pause-btn');
			if (pauseBtn) {
				pauseBtn.addEventListener('click', function() {
					paused = !paused;
					var label = document.getElementById('queue-pause-label');
					var icon = pauseBtn.querySelector('i');
					if (paused) {
						if (label) label.textContent = 'Resume';
						if (icon) icon.className = 'fas fa-play me-1';
						pauseBtn.setAttribute('aria-label', 'Resume automatic queue refresh');
					} else {
						if (label) label.textContent = 'Pause';
						if (icon) icon.className = 'fas fa-pause me-1';
						pauseBtn.setAttribute('aria-label', 'Pause automatic queue refresh');
						poll();
					}
				});
			}

			// Cancel job (event-delegated so it survives partial swaps).
			document.addEventListener('click', function(e) {
				var btn = e.target.closest ? e.target.closest('.cancel-queue-btn') : null;
				if (!btn) return;
				var queueId = btn.getAttribute('data-queue-id');
				confirmAction('Cancel this job?', 'This will remove the job from the active queue.', function() {
					gmdApiProxyFetch('/api/queue/cancel/' + queueId)
					.then(function(r){ return r.json(); })
					.then(function(data){
						if (data.success) {
							showToaster('success', 'Job cancelled');
							poll();
						} else {
							showToaster('error', 'Failed to cancel job: ' + (data.error || 'Unknown error'));
						}
					})
					.catch(function(err){
						showToaster('error', 'Error canceling job: ' + err.message);
					});
				});
			});
		})();
	`))
}
