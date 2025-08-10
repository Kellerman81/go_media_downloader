package api

import (
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// renderModernAdminIntro creates a modern admin dashboard intro page
func renderModernAdminIntro() Node {
	return Div(
		Class("container-fluid"),
		Style("background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-height: 100vh; padding: 0;"),

		// Hero Section
		Div(
			Class("row"),
			Div(
				Class("col-12"),
				Div(
					Style("background: linear-gradient(135deg, rgba(102,126,234,0.9) 0%, rgba(118,75,162,0.9) 100%); color: white; padding: 4rem 0; text-align: center;"),
					Div(
						Class("container"),
						Div(
							Class("row justify-content-center"),
							Div(
								Class("col-lg-8"),
								I(Class("fas fa-download mb-4"), Style("font-size: 4rem; opacity: 0.9;")),
								H1(
									Class("display-4 mb-3"),
									Style("font-weight: 700; text-shadow: 0 2px 4px rgba(0,0,0,0.3);"),
									Text("Go Media Downloader"),
								),
								P(
									Class("lead mb-4"),
									Style("font-size: 1.25rem; opacity: 0.9; text-shadow: 0 1px 2px rgba(0,0,0,0.2);"),
									Text("Advanced media download automation and management platform"),
								),
								Div(
									Class("d-flex justify-content-center gap-3 flex-wrap"),
									Span(
										Class("badge bg-success px-3 py-2"),
										Style("font-size: 1rem; border-radius: 20px;"),
										I(Class("fas fa-check me-1")),
										Text("Movies & TV Series"),
									),
									Span(
										Class("badge bg-info px-3 py-2"),
										Style("font-size: 1rem; border-radius: 20px;"),
										I(Class("fas fa-rss me-1")),
										Text("RSS Automation"),
									),
									Span(
										Class("badge bg-warning px-3 py-2"),
										Style("font-size: 1rem; border-radius: 20px;"),
										I(Class("fas fa-search me-1")),
										Text("Smart Indexing"),
									),
								),
							),
						),
					),
				),
			),
		),

		// Quick Stats Cards
		Div(
			Class("container my-5"),
			Div(
				Class("row g-4"),

				// Configuration Card
				Div(
					Class("col-lg-3 col-md-6"),
					Div(
						Class("card border-0 shadow-lg h-100"),
						Style("border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;"),
						Div(
							Class("card-body text-center p-4"),
							Div(
								Class("mb-3"),
								I(Class("fas fa-cog"), Style("font-size: 3rem; color: #007bff;")),
							),
							H5(Class("card-title fw-bold"), Text("Configuration")),
							P(Class("card-text text-muted"), Text("Manage system settings, media configurations, and indexer connections")),
							A(
								Class("btn btn-primary btn-sm"),
								Style("border-radius: 15px;"),
								Href("/api/admin/config/general"),
								I(Class("fas fa-arrow-right me-1")),
								Text("Configure"),
							),
						),
					),
				),

				// Management Card
				Div(
					Class("col-lg-3 col-md-6"),
					Div(
						Class("card border-0 shadow-lg h-100"),
						Style("border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;"),
						Div(
							Class("card-body text-center p-4"),
							Div(
								Class("mb-3"),
								I(Class("fas fa-tasks"), Style("font-size: 3rem; color: #28a745;")),
							),
							H5(Class("card-title fw-bold"), Text("Management")),
							P(Class("card-text text-muted"), Text("Monitor queues, schedulers, and system performance in real-time")),
							A(
								Class("btn btn-success btn-sm"),
								Style("border-radius: 15px;"),
								Href("/api/admin/grid/queue"),
								I(Class("fas fa-arrow-right me-1")),
								Text("Monitor"),
							),
						),
					),
				),

				// Database Card
				Div(
					Class("col-lg-3 col-md-6"),
					Div(
						Class("card border-0 shadow-lg h-100"),
						Style("border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;"),
						Div(
							Class("card-body text-center p-4"),
							Div(
								Class("mb-3"),
								I(Class("fas fa-database"), Style("font-size: 3rem; color: #ffc107;")),
							),
							H5(Class("card-title fw-bold"), Text("Database")),
							P(Class("card-text text-muted"), Text("Browse and manage media database tables and records")),
							A(
								Class("btn btn-warning btn-sm"),
								Style("border-radius: 15px;"),
								Href("/api/admin/database/movies"),
								I(Class("fas fa-arrow-right me-1")),
								Text("Browse"),
							),
						),
					),
				),

				// Tools Card
				Div(
					Class("col-lg-3 col-md-6"),
					Div(
						Class("card border-0 shadow-lg h-100"),
						Style("border-radius: 20px; transition: transform 0.3s ease, box-shadow 0.3s ease;"),
						Div(
							Class("card-body text-center p-4"),
							Div(
								Class("mb-3"),
								I(Class("fas fa-tools"), Style("font-size: 3rem; color: #dc3545;")),
							),
							H5(Class("card-title fw-bold"), Text("Tools")),
							P(Class("card-text text-muted"), Text("Access search tools, log viewer, and system utilities")),
							A(
								Class("btn btn-danger btn-sm"),
								Style("border-radius: 15px;"),
								Href("/api/admin/searchdownload"),
								I(Class("fas fa-arrow-right me-1")),
								Text("Search"),
							),
						),
					),
				),
			),
		),

		// Feature Highlights
		Div(
			Class("bg-light py-5"),
			Div(
				Class("container"),
				Div(
					Class("row"),
					Div(
						Class("col-12 text-center mb-5"),
						H2(Class("display-6 fw-bold mb-3"), Text("Powerful Features")),
						P(Class("lead text-muted"), Text("Everything you need for automated media management")),
					),
				),
				Div(
					Class("row g-4"),

					// Smart Automation
					Div(
						Class("col-md-4"),
						Div(
							Class("text-center"),
							Div(
								Class("mb-3"),
								I(Class("fas fa-robot"), Style("font-size: 2.5rem; color: #17a2b8;")),
							),
							H5(Class("fw-bold"), Text("Smart Automation")),
							P(Class("text-muted"), Text("Automated RSS monitoring, quality filtering, and intelligent download scheduling")),
						),
					),

					// Multi-Platform Support
					Div(
						Class("col-md-4"),
						Div(
							Class("text-center"),
							Div(
								Class("mb-3"),
								I(Class("fas fa-server"), Style("font-size: 2.5rem; color: #6610f2;")),
							),
							H5(Class("fw-bold"), Text("Multi-Platform")),
							P(Class("text-muted"), Text("Cross-platform support with Docker deployment and extensive indexer compatibility")),
						),
					),

					// Real-time Monitoring
					Div(
						Class("col-md-4"),
						Div(
							Class("text-center"),
							Div(
								Class("mb-3"),
								I(Class("fas fa-chart-line"), Style("font-size: 2.5rem; color: #fd7e14;")),
							),
							H5(Class("fw-bold"), Text("Real-time Monitoring")),
							P(Class("text-muted"), Text("Live statistics, queue monitoring, and comprehensive logging for complete visibility")),
						),
					),
				),
			),
		),
	)
}
