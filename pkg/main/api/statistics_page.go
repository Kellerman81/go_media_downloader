package api

import (
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// statSummaryCard renders a lazy-loaded KPI summary card (swaps its own body).
func statSummaryCard(borderClass, spinnerClass, url string) gomponents.Node {
	return html.Div(
		html.Class("col-xl-3 col-md-4 col-sm-6 mb-3"),
		html.Div(
			html.Class("card "+borderClass),
			html.Div(
				html.Class("card-body text-center"),
				gomponents.Attr("hx-get", url),
				gomponents.Attr("hx-trigger", "load"),
				gomponents.Attr("hx-swap", "outerHTML"),
				html.Div(html.Class("spinner-border spinner-border-sm "+spinnerClass)),
				gomponents.Text(" Loading..."),
			),
		),
	)
}

// statDetailCard renders a lazy-loaded detail card occupying the given column class.
func statDetailCard(colClass, spinnerClass, url, loadingText string) gomponents.Node {
	return html.Div(
		html.Class(colClass+" mb-4"),
		html.Div(
			html.Class("card h-100"),
			html.Div(
				html.Class("card-body text-center"),
				gomponents.Attr("hx-get", url),
				gomponents.Attr("hx-trigger", "load"),
				gomponents.Attr("hx-swap", "outerHTML"),
				html.Div(html.Class("spinner-border "+spinnerClass)),
				html.P(html.Class("mt-2"), gomponents.Text(loadingText)),
			),
		),
	)
}

// statSectionCard renders a full-width lazy-loaded section (swaps card inner HTML).
func statSectionCard(spinnerClass, url, loadingText string) gomponents.Node {
	return html.Div(
		html.Class("row"),
		html.Div(
			html.Class("col-12 mb-4"),
			html.Div(
				html.Class("card"),
				gomponents.Attr("hx-get", url),
				gomponents.Attr("hx-trigger", "load"),
				gomponents.Attr("hx-swap", "innerHTML"),
				html.Div(
					html.Class("card-body text-center"),
					html.Div(html.Class("spinner-border "+spinnerClass)),
					html.P(html.Class("mt-2"), gomponents.Text(loadingText)),
				),
			),
		),
	)
}

// @Summary      Statistics Dashboard Page
// @Description  Renders the main statistics dashboard with system performance and media library analytics
// @Tags         web
// @Produce      html
// @Success      200 {string} string "Statistics dashboard HTML"
// @Failure      500 {string} string "Internal server error"
// @Router       /web/admin/statistics [get].
func webStatisticsPage(ctx *gin.Context) {
	hasMusic := mediaTypeConfigured("music")
	hasBooks := mediaTypeConfigured("book")
	hasAudiobooks := mediaTypeConfigured("audiobook")

	// KPI summary cards — always movies & series, library types only when configured,
	// then storage & system.
	summaryCards := []gomponents.Node{
		statSummaryCard(
			"border-primary",
			"text-primary",
			"/api/admin/statistics/movies?type=summary",
		),
		statSummaryCard(
			"border-success",
			"text-success",
			"/api/admin/statistics/series?type=summary",
		),
	}

	if hasMusic {
		summaryCards = append(
			summaryCards,
			statSummaryCard(
				"border-danger",
				"text-danger",
				"/api/admin/statistics/music?type=summary",
			),
		)
	}

	if hasBooks {
		summaryCards = append(
			summaryCards,
			statSummaryCard(
				"border-warning",
				"text-warning",
				"/api/admin/statistics/books?type=summary",
			),
		)
	}

	if hasAudiobooks {
		summaryCards = append(
			summaryCards,
			statSummaryCard(
				"border-info",
				"text-info",
				"/api/admin/statistics/audiobooks?type=summary",
			),
		)
	}

	summaryCards = append(
		summaryCards,
		statSummaryCard("border-info", "text-info", "/api/admin/statistics/storage?type=summary"),
		statSummaryCard(
			"border-warning",
			"text-warning",
			"/api/admin/statistics/system?type=summary",
		),
	)

	// Library detail cards (two per row).
	detailCards := []gomponents.Node{
		statDetailCard(
			"col-md-6",
			"text-primary",
			"/api/admin/statistics/movies?type=detail",
			"Loading movie details...",
		),
		statDetailCard(
			"col-md-6",
			"text-success",
			"/api/admin/statistics/series?type=detail",
			"Loading series details...",
		),
	}

	if hasMusic {
		detailCards = append(
			detailCards,
			statDetailCard(
				"col-md-6",
				"text-danger",
				"/api/admin/statistics/music?type=detail",
				"Loading music details...",
			),
		)
	}

	if hasBooks {
		detailCards = append(
			detailCards,
			statDetailCard(
				"col-md-6",
				"text-warning",
				"/api/admin/statistics/books?type=detail",
				"Loading book details...",
			),
		)
	}

	if hasAudiobooks {
		detailCards = append(
			detailCards,
			statDetailCard(
				"col-md-6",
				"text-info",
				"/api/admin/statistics/audiobooks?type=detail",
				"Loading audiobook details...",
			),
		)
	}

	AdminPageAny(ctx, "Statistics - Go Media Downloader", html.Div(
		html.Class("config-section-enhanced"),

		// Page Header with refresh control.
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(
						html.Class("fas fa-chart-bar header-icon"),
						gomponents.Attr("aria-hidden", "true"),
					),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Statistics & Analytics")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text("System performance metrics and media library analytics"),
					),
				),
				html.Div(
					html.Class("ms-auto"),
					html.Button(
						html.Type("button"),
						html.Class("btn btn-outline-secondary btn-sm"),
						gomponents.Attr("onclick", "location.reload()"),
						gomponents.Attr("aria-label", "Refresh statistics"),
						html.I(
							html.Class("fas fa-sync-alt me-1"),
							gomponents.Attr("aria-hidden", "true"),
						),
						gomponents.Text("Refresh"),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),
			// Summary KPI row
			html.Div(append([]gomponents.Node{html.Class("row mb-4")}, summaryCards...)...),
			// Library details
			html.H5(html.Class("mb-3 text-muted"), gomponents.Text("Library Breakdown")),
			html.Div(append([]gomponents.Node{html.Class("row")}, detailCards...)...),
			// Storage details
			statSectionCard(
				"text-info",
				"/api/admin/statistics/storage?type=detail",
				"Loading storage details...",
			),
			// Operational sections
			html.H5(html.Class("mb-3 mt-2 text-muted"), gomponents.Text("System & Providers")),
			statSectionCard(
				"text-dark",
				"/api/admin/statistics/workers",
				"Loading worker statistics...",
			),
			statSectionCard(
				"text-secondary",
				"/api/admin/statistics/http",
				"Loading HTTP client statistics...",
			),
			statSectionCard(
				"text-primary",
				"/api/admin/statistics/metadata",
				"Loading metadata provider statistics...",
			),
			statSectionCard(
				"text-info",
				"/api/admin/statistics/notification",
				"Loading notification provider statistics...",
			),
			statSectionCard(
				"text-success",
				"/api/admin/statistics/downloader",
				"Loading downloader provider statistics...",
			),
		),
	))
}
