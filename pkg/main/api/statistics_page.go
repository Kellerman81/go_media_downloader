package api

import (
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// @Summary      Statistics Dashboard Page
// @Description  Renders the main statistics dashboard with system performance and media library analytics
// @Tags         web
// @Produce      html
// @Success      200 {string} string "Statistics dashboard HTML"
// @Failure      500 {string} string "Internal server error"
// @Router       /web/admin/statistics [get].
func webStatisticsPage(ctx *gin.Context) {
	AdminPageAny(ctx, "Statistics - Go Media Downloader", html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
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
					html.H2(html.Class("header-title"), gomponents.Text("Statistics & Analytics")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text("System performance metrics and media library analytics"),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Summary Cards Row
			html.Div(
				html.Class("row mb-4"),

				// Movies Card
				html.Div(
					html.Class("col-md-3 mb-3"),
					html.Div(
						html.Class("card border-primary"),
						html.Div(
							html.Class("card-body text-center"),
							gomponents.Attr("hx-get", "/api/admin/statistics/movies?type=summary"),
							gomponents.Attr("hx-trigger", "load"),
							gomponents.Attr("hx-swap", "outerHTML"),
							html.Div(html.Class("spinner-border spinner-border-sm text-primary")),
							gomponents.Text(" Loading..."),
						),
					),
				),

				// Series Card
				html.Div(
					html.Class("col-md-3 mb-3"),
					html.Div(
						html.Class("card border-success"),
						html.Div(
							html.Class("card-body text-center"),
							gomponents.Attr("hx-get", "/api/admin/statistics/series?type=summary"),
							gomponents.Attr("hx-trigger", "load"),
							gomponents.Attr("hx-swap", "outerHTML"),
							html.Div(html.Class("spinner-border spinner-border-sm text-success")),
							gomponents.Text(" Loading..."),
						),
					),
				),

				// Storage Card
				html.Div(
					html.Class("col-md-3 mb-3"),
					html.Div(
						html.Class("card border-info"),
						html.Div(
							html.Class("card-body text-center"),
							gomponents.Attr("hx-get", "/api/admin/statistics/storage?type=summary"),
							gomponents.Attr("hx-trigger", "load"),
							gomponents.Attr("hx-swap", "outerHTML"),
							html.Div(html.Class("spinner-border spinner-border-sm text-info")),
							gomponents.Text(" Loading..."),
						),
					),
				),

				// System Card
				html.Div(
					html.Class("col-md-3 mb-3"),
					html.Div(
						html.Class("card border-warning"),
						html.Div(
							html.Class("card-body text-center"),
							gomponents.Attr("hx-get", "/api/admin/statistics/system?type=summary"),
							gomponents.Attr("hx-trigger", "load"),
							gomponents.Attr("hx-swap", "outerHTML"),
							html.Div(html.Class("spinner-border spinner-border-sm text-warning")),
							gomponents.Text(" Loading..."),
						),
					),
				),
			),

			// Movie and Series Details Row
			html.Div(
				html.Class("row"),

				// Movie Details
				html.Div(
					html.Class("col-md-6 mb-4"),
					html.Div(
						html.Class("card"),
						html.Div(
							html.Class("card-body text-center"),
							gomponents.Attr("hx-get", "/api/admin/statistics/movies?type=detail"),
							gomponents.Attr("hx-trigger", "load"),
							gomponents.Attr("hx-swap", "outerHTML"),
							html.Div(html.Class("spinner-border text-primary")),
							html.P(html.Class("mt-2"), gomponents.Text("Loading movie details...")),
						),
					),
				),

				// Series Details
				html.Div(
					html.Class("col-md-6 mb-4"),
					html.Div(
						html.Class("card"),
						html.Div(
							html.Class("card-body text-center"),
							gomponents.Attr("hx-get", "/api/admin/statistics/series?type=detail"),
							gomponents.Attr("hx-trigger", "load"),
							gomponents.Attr("hx-swap", "outerHTML"),
							html.Div(html.Class("spinner-border text-success")),
							html.P(
								html.Class("mt-2"),
								gomponents.Text("Loading series details..."),
							),
						),
					),
				),
			),

			// Storage Details Row
			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-12 mb-4"),
					html.Div(
						html.Class("card"),
						html.Div(
							html.Class("card-body text-center"),
							gomponents.Attr("hx-get", "/api/admin/statistics/storage?type=detail"),
							gomponents.Attr("hx-trigger", "load"),
							gomponents.Attr("hx-swap", "outerHTML"),
							html.Div(html.Class("spinner-border text-info")),
							html.P(
								html.Class("mt-2"),
								gomponents.Text("Loading storage details..."),
							),
						),
					),
				),
			),

			// Worker Pool Statistics Row
			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-12 mb-4"),
					html.Div(
						html.Class("card"),
						gomponents.Attr("hx-get", "/api/admin/statistics/workers"),
						gomponents.Attr("hx-trigger", "load"),
						gomponents.Attr("hx-swap", "innerHTML"),
						html.Div(
							html.Class("card-body text-center"),
							html.Div(html.Class("spinner-border text-dark")),
							html.P(
								html.Class("mt-2"),
								gomponents.Text("Loading worker statistics..."),
							),
						),
					),
				),
			),

			// HTTP Client Statistics Row
			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-12 mb-4"),
					html.Div(
						html.Class("card"),
						gomponents.Attr("hx-get", "/api/admin/statistics/http"),
						gomponents.Attr("hx-trigger", "load"),
						gomponents.Attr("hx-swap", "innerHTML"),
						html.Div(
							html.Class("card-body text-center"),
							html.Div(html.Class("spinner-border text-secondary")),
							html.P(
								html.Class("mt-2"),
								gomponents.Text("Loading HTTP client statistics..."),
							),
						),
					),
				),
			),

			// Metadata Provider Statistics Row
			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-12 mb-4"),
					html.Div(
						html.Class("card"),
						gomponents.Attr("hx-get", "/api/admin/statistics/metadata"),
						gomponents.Attr("hx-trigger", "load"),
						gomponents.Attr("hx-swap", "innerHTML"),
						html.Div(
							html.Class("card-body text-center"),
							html.Div(html.Class("spinner-border text-primary")),
							html.P(
								html.Class("mt-2"),
								gomponents.Text("Loading metadata provider statistics..."),
							),
						),
					),
				),
			),

			// Notification Provider Statistics Row
			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-12 mb-4"),
					html.Div(
						html.Class("card"),
						gomponents.Attr("hx-get", "/api/admin/statistics/notification"),
						gomponents.Attr("hx-trigger", "load"),
						gomponents.Attr("hx-swap", "innerHTML"),
						html.Div(
							html.Class("card-body text-center"),
							html.Div(html.Class("spinner-border text-info")),
							html.P(
								html.Class("mt-2"),
								gomponents.Text("Loading notification provider statistics..."),
							),
						),
					),
				),
			),

			// Downloader Provider Statistics Row
			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-12 mb-4"),
					html.Div(
						html.Class("card"),
						gomponents.Attr("hx-get", "/api/admin/statistics/downloader"),
						gomponents.Attr("hx-trigger", "load"),
						gomponents.Attr("hx-swap", "innerHTML"),
						html.Div(
							html.Class("card-body text-center"),
							html.Div(html.Class("spinner-border text-success")),
							html.P(
								html.Class("mt-2"),
								gomponents.Text("Loading downloader provider statistics..."),
							),
						),
					),
				),
			),
		),
	))
}
