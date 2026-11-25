package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

func renderStatsPage(ctx *gin.Context) {
	pageNode := page("Media Statistics", false, false, true, renderStatsGrid())

	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

func renderStatsGrid() gomponents.Node {
	id := 0

	movieLists := database.GetrowsN[string](
		false,
		5,
		"select distinct listname from movies where listname is not null and listname != ''",
	)
	seriesLists := database.GetrowsN[string](
		false,
		5,
		"select distinct listname from series where listname is not null and listname != ''",
	)

	statsData := make([]map[string]any, 0, len(movieLists)+len(seriesLists))

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
