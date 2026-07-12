package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderModernAdminIntro creates the admin landing page: a live dashboard with
// library counts, queue/scheduler activity and quick actions.
func renderModernAdminIntro() gomponents.Node {
	return html.Div(
		html.Class("container-fluid p-4"),
		dashboardHeader(),

		// Live-updating stat region (refreshed via HTMX without a full reload).
		html.Div(
			html.ID("dashboard-cards"),
			hx.Get("/api/admin/dashboard/cards"),
			hx.Trigger("every 30s"),
			hx.Swap("innerHTML"),
			renderDashboardCards(),
		),

		dashboardQuickActions(),
	)
}

// dashboardHeader renders the page title row.
func dashboardHeader() gomponents.Node {
	return html.Div(
		html.Class("d-flex align-items-center justify-content-between flex-wrap mb-4"),
		html.Div(
			html.H1(
				html.Class("h3 mb-1"),
				html.I(
					html.Class("fas fa-gauge-high me-2 text-primary"),
					gomponents.Attr("aria-hidden", "true"),
				),
				gomponents.Text("Dashboard"),
			),
			html.P(
				html.Class("text-muted mb-0"),
				gomponents.Text("Library overview and current activity"),
			),
		),
		html.Button(
			html.Type("button"),
			html.Class("btn btn-outline-secondary btn-sm"),
			gomponents.Attr("aria-label", "Refresh dashboard now"),
			hx.Get("/api/admin/dashboard/cards"),
			hx.Target("#dashboard-cards"),
			hx.Swap("innerHTML"),
			html.I(html.Class("fas fa-sync-alt me-1"), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text("Refresh"),
		),
	)
}

// countRow returns the row count of a table, or 0 on any error.
func countRow(query string) int {
	return int(database.Getdatarow[uint](false, query))
}

// renderDashboardCards builds the live statistic cards. It is also served as an
// HTMX fragment for periodic refresh.
func renderDashboardCards() gomponents.Node {
	ctx := context.Background()
	movieStats, _ := getMovieStatistics(ctx)
	seriesStats, _ := getSeriesStatistics(ctx)

	albums := countRow("SELECT COUNT(*) FROM albums")
	albumsMissing := countRow("SELECT COUNT(*) FROM albums WHERE missing = 1")
	books := countRow("SELECT COUNT(*) FROM books")
	booksMissing := countRow("SELECT COUNT(*) FROM books WHERE missing = 1")
	audiobooks := countRow("SELECT COUNT(*) FROM audiobooks")
	audiobooksMissing := countRow("SELECT COUNT(*) FROM audiobooks WHERE missing = 1")

	queues := worker.GetQueues()
	queueActive := len(queues)

	schedules := worker.GetSchedules()

	schedulesRunning := 0
	for _, s := range schedules {
		if s.IsRunning {
			schedulesRunning++
		}
	}

	sys := getSystemStatistics()

	return html.Div(
		html.Class("row g-3"),

		statCard(
			"fas fa-film",
			"#0d6efd",
			"Movies",
			fmt.Sprintf("%d", movieStats.TotalMovies),
			fmt.Sprintf(
				"%d missing · %d available",
				movieStats.MissingMovies,
				movieStats.AvailableMovies,
			),
			"/api/admin/database/movies",
			movieStats.MissingMovies > 0,
		),

		statCard(
			"fas fa-tv",
			"#6610f2",
			"Series",
			fmt.Sprintf("%d", seriesStats.TotalSeries),
			fmt.Sprintf(
				"%d episodes · %d missing",
				seriesStats.TotalEpisodes,
				seriesStats.MissingEpisodes,
			),
			"/api/admin/database/series",
			seriesStats.MissingEpisodes > 0,
		),

		statCard("fas fa-compact-disc", "#d63384", "Albums",
			fmt.Sprintf("%d", albums),
			fmt.Sprintf("%d missing", albumsMissing),
			"/api/admin/database/albums", albumsMissing > 0),

		statCard("fas fa-book", "#fd7e14", "Books",
			fmt.Sprintf("%d", books),
			fmt.Sprintf("%d missing", booksMissing),
			"/api/admin/database/books", booksMissing > 0),

		statCard("fas fa-headphones", "#20c997", "Audiobooks",
			fmt.Sprintf("%d", audiobooks),
			fmt.Sprintf("%d missing", audiobooksMissing),
			"/api/admin/database/audiobooks", audiobooksMissing > 0),

		statCard("fas fa-list-ol", "#0dcaf0", "Queue",
			fmt.Sprintf("%d", queueActive),
			fmt.Sprintf("%d/%d schedulers running", schedulesRunning, len(schedules)),
			"/api/admin/grid/queue", false),

		// System mini-card spanning full width on small screens.
		html.Div(
			html.Class("col-12"),
			html.Div(
				html.Class("card border-0 shadow-sm"),
				html.Style("border-radius: 12px;"),
				html.Div(
					html.Class("card-body d-flex flex-wrap gap-4 align-items-center"),
					html.Div(
						html.I(
							html.Class("fas fa-server me-2 text-secondary"),
							gomponents.Attr("aria-hidden", "true"),
						),
						html.Span(html.Class("fw-semibold"), gomponents.Text("System")),
					),
					sysStat("Uptime", formatUptime(sys.UptimeSeconds)),
					sysStat("Goroutines", fmt.Sprintf("%d", sys.NumGoroutine)),
					sysStat("Memory", fmt.Sprintf("%.0f MB", sys.MemoryAllocMB)),
					sysStat("CPUs", fmt.Sprintf("%d", sys.NumCPU)),
					sysStat("Platform", sys.GOOS+"/"+sys.GOARCH),
					html.Small(
						html.Class("text-muted ms-auto"),
						gomponents.Text("Updated "+time.Now().Format("15:04:05")),
					),
				),
			),
		),
	)
}

// sysStat renders a small label/value pair for the system card.
func sysStat(label, value string) gomponents.Node {
	return html.Div(
		html.Small(html.Class("text-muted d-block"), gomponents.Text(label)),
		html.Span(html.Class("fw-semibold"), gomponents.Text(value)),
	)
}

// statCard renders a single library statistic card.
func statCard(icon, color, title, big, sub, href string, warn bool) gomponents.Node {
	subClass := "text-muted mb-0 small"
	if warn {
		subClass = "text-danger mb-0 small fw-semibold"
	}

	return html.Div(
		html.Class("col-xl-2 col-md-4 col-sm-6"),
		html.A(
			html.Href(href),
			html.Class("text-decoration-none"),
			html.Div(
				html.Class("card border-0 shadow-sm h-100 dashboard-stat-card"),
				html.Style(
					"border-radius: 12px; transition: transform 0.15s ease, box-shadow 0.15s ease;",
				),
				html.Div(
					html.Class("card-body"),
					html.Div(
						html.Class("d-flex align-items-center justify-content-between mb-2"),
						html.Span(
							html.Class("text-muted text-uppercase small fw-semibold"),
							gomponents.Text(title),
						),
						html.I(
							html.Class(icon),
							html.Style("font-size: 1.4rem; color: "+color+";"),
							gomponents.Attr("aria-hidden", "true"),
						),
					),
					html.Div(
						html.Class("h3 mb-1"),
						html.Style("color: "+color+";"),
						gomponents.Text(big),
					),
					html.P(html.Class(subClass), gomponents.Text(sub)),
				),
			),
		),
	)
}

// dashboardQuickActions renders a row of common shortcuts.
func dashboardQuickActions() gomponents.Node {
	type action struct {
		icon, label, href string
	}

	actions := []action{
		{"fas fa-magnifying-glass-arrow-right", "Search & Download", "/api/admin/searchdownload"},
		{"fas fa-film", "Add Movies", "/api/admin/metadata-search/movies"},
		{"fas fa-tv", "Add Series", "/api/admin/metadata-search/series"},
		{"fas fa-bullseye", "Wanted", "/api/admin/wanted"},
		{"fas fa-calendar-alt", "Calendar", "/api/admin/calendar"},
		{"fas fa-chart-line", "Statistics", "/api/admin/statistics"},
		{"fas fa-gear", "Configuration", "/api/admin/config/general"},
		{"fas fa-file-text", "Logs", "/api/admin/logviewer"},
	}

	buttons := make([]gomponents.Node, 0, len(actions))
	for _, a := range actions {
		buttons = append(buttons, html.A(
			html.Href(a.href),
			html.Class("btn btn-outline-secondary d-flex align-items-center gap-2"),
			html.I(html.Class(a.icon), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text(a.label),
		))
	}

	return html.Div(
		html.Class("mt-4"),
		html.H2(html.Class("h5 mb-3"), gomponents.Text("Quick Actions")),
		html.Div(
			append([]gomponents.Node{html.Class("d-flex flex-wrap gap-2")}, buttons...)...,
		),
	)
}

// formatUptime renders a seconds count as a compact human string.
func formatUptime(seconds int64) string {
	if seconds <= 0 {
		return "just started"
	}

	d := seconds / 86400
	h := (seconds % 86400) / 3600
	m := (seconds % 3600) / 60

	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// dashboardCardsPartial serves the live dashboard cards as an HTMX fragment.
func dashboardCardsPartial(ctx *gin.Context) {
	var buf strings.Builder
	renderDashboardCards().Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}
