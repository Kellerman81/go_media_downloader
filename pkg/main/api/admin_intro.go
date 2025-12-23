package api

import (
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderModernAdminIntro creates a modern admin dashboard intro page.
func renderModernAdminIntro() gomponents.Node {
	return html.Div(
		html.Class("container-fluid"),
		html.Style(
			"background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-height: 100vh; padding: 0;",
		),

		// Hero Section
		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Style(
						"background: linear-gradient(135deg, rgba(102,126,234,0.9) 0%, rgba(118,75,162,0.9) 100%); color: white; padding: 4rem 0; text-align: center;",
					),
					html.Div(
						html.Class("container"),
						html.Div(
							html.Class("row justify-content-center"),
							html.Div(
								html.Class("col-lg-8"),
								html.I(
									html.Class("fas fa-download mb-4"),
									html.Style("font-size: 4rem; opacity: 0.9;"),
								),
								html.H1(
									html.Class("display-4 mb-3"),
									html.Style(
										"font-weight: 700; text-shadow: 0 2px 4px rgba(0,0,0,0.3);",
									),
									gomponents.Text("Go Media Downloader"),
								),
								html.P(
									html.Class("lead mb-4"),
									html.Style(
										"font-size: 1.25rem; opacity: 0.9; text-shadow: 0 1px 2px rgba(0,0,0,0.2);",
									),
									gomponents.Text(
										"Advanced media download automation and management platform",
									),
								),
								html.Div(
									html.Class("d-flex justify-content-center gap-3 flex-wrap"),
									html.Span(
										html.Class("badge bg-success px-3 py-2"),
										html.Style("font-size: 1rem; border-radius: 20px;"),
										html.I(html.Class("fas fa-check me-1")),
										gomponents.Text("Movies & TV Series"),
									),
									html.Span(
										html.Class("badge bg-info px-3 py-2"),
										html.Style("font-size: 1rem; border-radius: 20px;"),
										html.I(html.Class("fas fa-rss me-1")),
										gomponents.Text("RSS Automation"),
									),
									html.Span(
										html.Class("badge bg-warning px-3 py-2"),
										html.Style("font-size: 1rem; border-radius: 20px;"),
										html.I(html.Class("fas fa-search me-1")),
										gomponents.Text("Smart Indexing"),
									),
								),
							),
						),
					),
				),
			),
		),

		// Quick Stats Cards
		html.Div(
			html.Class("container my-5"),
			html.Div(
				html.Class("row g-4"),

				// Configuration Card
				html.Div(
					html.Class("col-lg-3 col-md-6"),
					html.Div(
						html.Class("card border-0 shadow-lg h-100"),
						html.Style(
							"border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;",
						),
						html.Div(
							html.Class("card-body text-center p-4"),
							html.Div(
								html.Class("mb-3"),
								html.I(
									html.Class("fas fa-cog"),
									html.Style("font-size: 3rem; color: #007bff;"),
								),
							),
							html.H5(
								html.Class("card-title fw-bold"),
								gomponents.Text("Configuration"),
							),
							html.P(
								html.Class("card-text text-muted"),
								gomponents.Text(
									"Manage system settings, media configurations, and indexer connections",
								),
							),
							html.A(
								html.Class("btn btn-primary btn-sm"),
								html.Style("border-radius: 15px;"),
								html.Href("/api/admin/config/general"),
								html.I(html.Class("fas fa-arrow-right me-1")),
								gomponents.Text("Configure"),
							),
						),
					),
				),

				// Management Card
				html.Div(
					html.Class("col-lg-3 col-md-6"),
					html.Div(
						html.Class("card border-0 shadow-lg h-100"),
						html.Style(
							"border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;",
						),
						html.Div(
							html.Class("card-body text-center p-4"),
							html.Div(
								html.Class("mb-3"),
								html.I(
									html.Class("fas fa-tasks"),
									html.Style("font-size: 3rem; color: #28a745;"),
								),
							),
							html.H5(
								html.Class("card-title fw-bold"),
								gomponents.Text("Management"),
							),
							html.P(
								html.Class("card-text text-muted"),
								gomponents.Text(
									"Monitor queues, schedulers, and system performance in real-time",
								),
							),
							html.A(
								html.Class("btn btn-success btn-sm"),
								html.Style("border-radius: 15px;"),
								html.Href("/api/admin/grid/queue"),
								html.I(html.Class("fas fa-arrow-right me-1")),
								gomponents.Text("Monitor"),
							),
						),
					),
				),

				// Database Card
				html.Div(
					html.Class("col-lg-3 col-md-6"),
					html.Div(
						html.Class("card border-0 shadow-lg h-100"),
						html.Style(
							"border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;",
						),
						html.Div(
							html.Class("card-body text-center p-4"),
							html.Div(
								html.Class("mb-3"),
								html.I(
									html.Class("fas fa-database"),
									html.Style("font-size: 3rem; color: #ffc107;"),
								),
							),
							html.H5(html.Class("card-title fw-bold"), gomponents.Text("Database")),
							html.P(
								html.Class("card-text text-muted"),
								gomponents.Text(
									"Browse and manage media database tables and records",
								),
							),
							html.A(
								html.Class("btn btn-warning btn-sm"),
								html.Style("border-radius: 15px;"),
								html.Href("/api/admin/database/movies"),
								html.I(html.Class("fas fa-arrow-right me-1")),
								gomponents.Text("Browse"),
							),
						),
					),
				),

				// Tools Card
				html.Div(
					html.Class("col-lg-3 col-md-6"),
					html.Div(
						html.Class("card border-0 shadow-lg h-100"),
						html.Style(
							"border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;",
						),
						html.Div(
							html.Class("card-body text-center p-4"),
							html.Div(
								html.Class("mb-3"),
								html.I(
									html.Class("fas fa-tools"),
									html.Style("font-size: 3rem; color: #dc3545;"),
								),
							),
							html.H5(html.Class("card-title fw-bold"), gomponents.Text("Tools")),
							html.P(
								html.Class("card-text text-muted"),
								gomponents.Text(
									"Access search tools, log viewer, and system utilities",
								),
							),
							html.A(
								html.Class("btn btn-danger btn-sm"),
								html.Style("border-radius: 15px;"),
								html.Href("/api/admin/searchdownload"),
								html.I(html.Class("fas fa-arrow-right me-1")),
								gomponents.Text("Search"),
							),
						),
					),
				),
			),
		),

		// Feature Highlights
		html.Div(
			html.Class("bg-light py-5"),
			html.Div(
				html.Class("container"),
				html.Div(
					html.Class("row"),
					html.Div(
						html.Class("col-12 text-center mb-5"),
						html.H2(
							html.Class("display-6 fw-bold mb-3"),
							gomponents.Text("Powerful Features"),
						),
						html.P(
							html.Class("lead text-muted"),
							gomponents.Text("Everything you need for automated media management"),
						),
					),
				),
				html.Div(
					html.Class("row g-4"),

					// Smart Automation
					html.Div(
						html.Class("col-md-4"),
						html.Div(
							html.Class("text-center"),
							html.Div(
								html.Class("mb-3"),
								html.I(
									html.Class("fas fa-robot"),
									html.Style("font-size: 2.5rem; color: #17a2b8;"),
								),
							),
							html.H5(html.Class("fw-bold"), gomponents.Text("Smart Automation")),
							html.P(
								html.Class("text-muted"),
								gomponents.Text(
									"Automated RSS monitoring, quality filtering, and intelligent download scheduling",
								),
							),
						),
					),

					// Multi-Platform Support
					html.Div(
						html.Class("col-md-4"),
						html.Div(
							html.Class("text-center"),
							html.Div(
								html.Class("mb-3"),
								html.I(
									html.Class("fas fa-server"),
									html.Style("font-size: 2.5rem; color: #6610f2;"),
								),
							),
							html.H5(html.Class("fw-bold"), gomponents.Text("Multi-Platform")),
							html.P(
								html.Class("text-muted"),
								gomponents.Text(
									"Cross-platform support with Docker deployment and extensive indexer compatibility",
								),
							),
						),
					),

					// Real-time Monitoring
					html.Div(
						html.Class("col-md-4"),
						html.Div(
							html.Class("text-center"),
							html.Div(
								html.Class("mb-3"),
								html.I(
									html.Class("fas fa-chart-line"),
									html.Style("font-size: 2.5rem; color: #fd7e14;"),
								),
							),
							html.H5(html.Class("fw-bold"), gomponents.Text("Real-time Monitoring")),
							html.P(
								html.Class("text-muted"),
								gomponents.Text(
									"Live statistics, queue monitoring, and comprehensive logging for complete visibility",
								),
							),
						),
					),
				),
			),
		),
	)
}
