package api

import (
	"fmt"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderMovieCard renders the movie summary card.
func renderMovieCard(stats MovieStatistics) gomponents.Node {
	return html.Div(
		html.Class("card-body text-center"),
		html.H1(html.Class("display-4 text-primary"), gomponents.Textf("%d", stats.TotalMovies)),
		html.P(
			html.Class("text-muted mb-2"),
			html.I(html.Class("fa-solid fa-film mr-2")),
			gomponents.Text("Total Movies"),
		),
		html.P(html.Class("mb-0"),
			html.Span(html.Class("text-success mr-3"),
				gomponents.Textf("%d Available", stats.AvailableMovies)),
			gomponents.Text(" | "),
			html.Span(html.Class("text-danger"),
				gomponents.Textf("%d Missing", stats.MissingMovies)),
		),
	)
}

// renderSeriesCard renders the series summary card.
func renderSeriesCard(stats SeriesStatistics) gomponents.Node {
	return html.Div(
		html.Class("card-body text-center"),
		html.H1(html.Class("display-4 text-success"), gomponents.Textf("%d", stats.TotalSeries)),
		html.P(
			html.Class("text-muted mb-2"),
			html.I(html.Class("fa-solid fa-tv mr-2")),
			gomponents.Text("Total Series"),
		),
		html.P(
			html.Class("mb-0"),
			gomponents.Textf(
				"%d Episodes (%d Available)",
				stats.TotalEpisodes,
				stats.AvailableEpisodes,
			),
		),
	)
}

// renderStorageCard renders the storage summary card.
func renderStorageCard(stats StorageStatistics) gomponents.Node {
	return html.Div(
		html.Class("card-body text-center"),
		html.H1(html.Class("display-4 text-info"), gomponents.Textf("%d", stats.TotalFileCount)),
		html.P(
			html.Class("text-muted mb-2"),
			html.I(html.Class("fa-solid fa-hdd mr-2")),
			gomponents.Text("Files"),
		),
		html.P(html.Class("mb-0"),
			gomponents.Textf("%d Media Types", stats.TotalPaths),
		),
	)
}

// renderSystemCard renders the system summary card.
func renderSystemCard(stats SystemStatistics) gomponents.Node {
	uptime := formatDuration(stats.UptimeSeconds)

	return html.Div(
		html.Class("card-body text-center"),
		html.H1(html.Class("display-4 text-warning"), gomponents.Text(uptime)),
		html.P(
			html.Class("text-muted mb-2"),
			html.I(html.Class("fa-solid fa-server mr-2")),
			gomponents.Text("Uptime"),
		),
		html.P(html.Class("mb-0"),
			gomponents.Textf("Go Routines: %d", stats.GoRoutines),
		),
	)
}

// renderMovieDetails renders the movie details section.
func renderMovieDetails(stats MovieStatistics) gomponents.Node {
	qualityRows := []gomponents.Node{}
	if len(stats.ByQuality) == 0 {
		qualityRows = append(qualityRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "2"), html.Class("text-center text-muted"),
				gomponents.Text("No data")),
		))
	} else {
		for quality, count := range stats.ByQuality {
			qualityRows = append(qualityRows, html.Tr(
				html.Td(gomponents.Text(quality)),
				html.Td(html.Class("text-right"),
					html.Span(html.Class("badge badge-primary text-dark"), gomponents.Textf("%d", count))),
			))
		}
	}

	listRows := []gomponents.Node{}
	if len(stats.ByList) == 0 {
		listRows = append(listRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "2"), html.Class("text-center text-muted"),
				gomponents.Text("No data")),
		))
	} else {
		for list, count := range stats.ByList {
			listRows = append(listRows, html.Tr(
				html.Td(gomponents.Text(list)),
				html.Td(html.Class("text-right"),
					html.Span(html.Class("badge badge-info text-dark"), gomponents.Textf("%d", count))),
			))
		}
	}

	return html.Div(
		html.Div(
			html.Class("card-header bg-primary text-white"),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class("fa-solid fa-film mr-2")),
				gomponents.Text("Movie Details"),
			),
		),
		html.Div(html.Class("card-body"),
			html.H6(html.Class("text-muted"), gomponents.Text("By Quality Profile")),
			html.Table(html.Class("table table-sm table-hover"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Text("Quality")),
						html.Th(html.Class("text-right"), gomponents.Text("Count")),
					),
				),
				html.TBody(qualityRows...),
			),
			html.H6(html.Class("text-muted mt-3"), gomponents.Text("By List")),
			html.Table(html.Class("table table-sm table-hover"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Text("List")),
						html.Th(html.Class("text-right"), gomponents.Text("Count")),
					),
				),
				html.TBody(listRows...),
			),
		),
	)
}

// renderSeriesDetails renders the series details section.
func renderSeriesDetails(stats SeriesStatistics) gomponents.Node {
	qualityRows := []gomponents.Node{}
	if len(stats.ByQuality) == 0 {
		qualityRows = append(qualityRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "2"), html.Class("text-center text-muted"),
				gomponents.Text("No data")),
		))
	} else {
		for quality, count := range stats.ByQuality {
			qualityRows = append(qualityRows, html.Tr(
				html.Td(gomponents.Text(quality)),
				html.Td(html.Class("text-right"),
					html.Span(html.Class("badge badge-success text-dark"), gomponents.Textf("%d", count))),
			))
		}
	}

	listRows := []gomponents.Node{}
	if len(stats.ByList) == 0 {
		listRows = append(listRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "2"), html.Class("text-center text-muted"),
				gomponents.Text("No data")),
		))
	} else {
		for list, count := range stats.ByList {
			listRows = append(listRows, html.Tr(
				html.Td(gomponents.Text(list)),
				html.Td(html.Class("text-right"),
					html.Span(html.Class("badge badge-info text-dark"), gomponents.Textf("%d", count))),
			))
		}
	}

	return html.Div(
		html.Div(
			html.Class("card-header bg-success text-white"),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class("fa-solid fa-tv mr-2")),
				gomponents.Text("Series Details"),
			),
		),
		html.Div(html.Class("card-body"),
			html.H6(html.Class("text-muted"), gomponents.Text("By Quality Profile")),
			html.Table(html.Class("table table-sm table-hover"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Text("Quality")),
						html.Th(html.Class("text-right"), gomponents.Text("Count")),
					),
				),
				html.TBody(qualityRows...),
			),
			html.H6(html.Class("text-muted mt-3"), gomponents.Text("By List")),
			html.Table(html.Class("table table-sm table-hover"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Text("List")),
						html.Th(html.Class("text-right"), gomponents.Text("Count")),
					),
				),
				html.TBody(listRows...),
			),
		),
	)
}

// renderStorageDetails renders the storage details section.
func renderStorageDetails(stats StorageStatistics) gomponents.Node {
	var totalSize int64
	for _, path := range stats.ByPath {
		totalSize += path.TotalSize
	}

	pathRows := []gomponents.Node{}
	if len(stats.ByPath) == 0 {
		pathRows = append(pathRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "5"), html.Class("text-center text-muted"),
				gomponents.Text("No storage paths configured")),
		))
	} else {
		for mediaType, info := range stats.ByPath {
			pathRows = append(pathRows, html.Tr(
				html.Td(html.Strong(gomponents.Text(mediaType))),
				html.Td(html.Small(html.Class("text-muted"), gomponents.Text(info.Path))),
				html.Td(html.Class("text-right text-dark"), gomponents.Textf("%d", info.FileCount)),
				html.Td(html.Class("text-right text-dark"), gomponents.Textf("%d", info.FolderCount)),
				html.Td(html.Class("text-right text-dark"), gomponents.Text(formatBytes(uint64(info.TotalSize)))),
			))
		}
	}

	return html.Div(
		html.Div(
			html.Class("card-header bg-info text-white"),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class("fa-solid fa-hdd mr-2")),
				gomponents.Text("Storage Details"),
			),
		),
		html.Div(html.Class("card-body"),
			html.Div(html.Class("row mb-3"),
				html.Div(html.Class("col-md-4"),
					html.H6(html.Class("text-muted"), gomponents.Text("Total Files")),
					html.H4(gomponents.Textf("%d", stats.TotalFileCount)),
				),
				html.Div(html.Class("col-md-4"),
					html.H6(html.Class("text-muted"), gomponents.Text("Total Size")),
					html.H4(gomponents.Text(formatBytes(uint64(totalSize)))),
				),
				html.Div(html.Class("col-md-4"),
					html.H6(html.Class("text-muted"), gomponents.Text("Storage Paths")),
					html.H4(gomponents.Textf("%d", stats.TotalPaths)),
				),
			),
			html.H6(html.Class("text-muted"), gomponents.Text("By Storage Path")),
			html.Table(html.Class("table table-sm table-hover"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Text("Media Type")),
						html.Th(gomponents.Text("Path")),
						html.Th(html.Class("text-right"), gomponents.Text("Files")),
						html.Th(html.Class("text-right"), gomponents.Text("Folders")),
						html.Th(html.Class("text-right"), gomponents.Text("Size")),
					),
				),
				html.TBody(pathRows...),
			),
		),
	)
}

// renderWorkerPoolStats renders the worker pool statistics.
func renderWorkerPoolStats(stats worker.Stats) gomponents.Node {
	pools := []struct {
		name  string
		stats worker.StatsDetail
	}{
		{"Parse", stats.WorkerParse},
		{"Search", stats.WorkerSearch},
		{"RSS", stats.WorkerRSS},
		{"Files", stats.WorkerFiles},
		{"Metadata", stats.WorkerMeta},
		{"Indexer", stats.WorkerIndex},
		{"Indexer RSS", stats.WorkerIndexRSS},
	}

	rows := []gomponents.Node{}
	for _, pool := range pools {
		s := pool.stats

		statusClass := ""
		if s.FailedTasks > 0 {
			statusClass = "table-warning"
		}

		failedCell := gomponents.Textf("%d", s.FailedTasks)
		if s.FailedTasks > 0 {
			failedCell = html.Span(
				html.Class("badge badge-danger text-dark"),
				gomponents.Textf("%d", s.FailedTasks),
			)
		}

		droppedCell := gomponents.Textf("%d", s.DroppedTasks)
		if s.DroppedTasks > 0 {
			droppedCell = html.Span(
				html.Class("badge badge-warning"),
				gomponents.Textf("%d", s.DroppedTasks),
			)
		}

		rows = append(rows, html.Tr(
			gomponents.If(statusClass != "", html.Class(statusClass)),
			html.Td(html.Strong(gomponents.Text(pool.name))),
			html.Td(
				html.Class("text-center"),
				html.Span(
					html.Class("badge badge-primary text-dark"),
					gomponents.Textf("%d", s.RunningWorkers),
				),
			),
			html.Td(html.Class("text-center text-dark"), gomponents.Textf("%d", s.WaitingTasks)),
			html.Td(html.Class("text-center text-dark"), gomponents.Textf("%d", s.CompletedTasks)),
			html.Td(
				html.Class("text-center"),
				html.Span(
					html.Class("badge badge-success text-dark"),
					gomponents.Textf("%d", s.SuccessfulTasks),
				),
			),
			html.Td(html.Class("text-center text-dark"), failedCell),
			html.Td(html.Class("text-center text-dark"), droppedCell),
		))
	}

	return html.Div(
		html.Div(
			html.Class("card-header bg-dark text-white"),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class("fa-solid fa-cogs mr-2")),
				gomponents.Text("Worker Pool Statistics"),
			),
		),
		html.Div(html.Class("card-body"),
			html.Table(html.Class("table table-sm table-hover"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Text("Pool Name")),
						html.Th(html.Class("text-center"), gomponents.Text("Running")),
						html.Th(html.Class("text-center"), gomponents.Text("Waiting")),
						html.Th(html.Class("text-center"), gomponents.Text("Completed")),
						html.Th(html.Class("text-center"), gomponents.Text("Successful")),
						html.Th(html.Class("text-center"), gomponents.Text("Failed")),
						html.Th(html.Class("text-center"), gomponents.Text("Dropped")),
					),
				),
				html.TBody(rows...),
			),
		),
	)
}

// renderHTTPClientStats renders the HTTP client statistics.
func renderHTTPClientStats(stats HTTPStatistics) gomponents.Node {
	rows := []gomponents.Node{}
	hasClients := false

	for name, client := range stats.ClientStats {
		hasClients = true

		totalReqs := client.RequestsTotal

		successRate := 0.0
		if totalReqs > 0 {
			successRate = (float64(client.SuccessCount) / float64(totalReqs)) * 100
		}

		rateClass := "danger"
		if successRate >= 95 {
			rateClass = "success"
		} else if successRate >= 80 {
			rateClass = "warning"
		}

		lastRequest := formatTimestamp(client.LastRequestAt)
		nextAvailable := formatTimestamp(client.NextAvailableAt)

		errorMsg := gomponents.Text("-")
		if client.LastErrorMessage != "" {
			errorMsg = html.Span(
				html.Class("text-danger"),
				gomponents.Attr("title", client.LastErrorMessage),
				gomponents.Text(truncateString(client.LastErrorMessage, 30)),
			)
		}

		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(name))),
			html.Td(html.Class("text-right text-dark"), gomponents.Textf("%d", client.Requests1h)),
			html.Td(html.Class("text-right text-dark"), gomponents.Textf("%d", client.Requests24h)),
			html.Td(html.Class("text-right text-dark"), gomponents.Textf("%d", totalReqs)),
			html.Td(
				html.Class("text-right text-dark"),
				gomponents.Textf("%d", client.SuccessCount),
			),
			html.Td(
				html.Class("text-right text-danger"),
				gomponents.Textf("%d", client.FailureCount),
			),
			html.Td(
				html.Class("text-right text-dark"),
				gomponents.Textf("%.0fms", client.AvgResponseTimeMs),
			),
			html.Td(
				html.Class("text-center"),
				html.Span(
					html.Class("badge badge-"+rateClass+" text-dark"),
					gomponents.Textf("%.1f%%", successRate),
				),
			),
			html.Td(html.Class("text-dark"), html.Small(gomponents.Text(lastRequest))),
			html.Td(html.Class("text-dark"), html.Small(gomponents.Text(nextAvailable))),
			html.Td(html.Small(errorMsg)),
		))
	}

	if !hasClients {
		rows = append(rows, html.Tr(
			html.Td(gomponents.Attr("colspan", "11"), html.Class("text-center text-muted"),
				gomponents.Text("No HTTP clients")),
		))
	}

	return html.Div(
		html.Div(
			html.Class("card-header bg-secondary text-white"),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class("fa-solid fa-globe mr-2")),
				gomponents.Text("HTTP Client Statistics"),
			),
		),
		html.Div(html.Class("card-body"),
			html.Div(html.Class("table-responsive"),
				html.Table(html.Class("table table-sm table-hover"),
					html.THead(
						html.Tr(
							html.Th(gomponents.Text("Client")),
							html.Th(html.Class("text-right"), gomponents.Text("Req 1h")),
							html.Th(html.Class("text-right"), gomponents.Text("Req 24h")),
							html.Th(html.Class("text-right"), gomponents.Text("Total")),
							html.Th(html.Class("text-right"), gomponents.Text("Success")),
							html.Th(html.Class("text-right"), gomponents.Text("Failed")),
							html.Th(html.Class("text-right"), gomponents.Text("Avg Time")),
							html.Th(html.Class("text-center"), gomponents.Text("Rate")),
							html.Th(gomponents.Text("Last Request")),
							html.Th(gomponents.Text("Next Available")),
							html.Th(gomponents.Text("Last Error")),
						),
					),
					html.TBody(rows...),
				),
			),
		),
	)
}

// renderProviderStats renders provider statistics (metadata, notification, downloader).
func renderProviderStats(
	stats ProviderStatistics,
	title string,
	colorClass string,
) gomponents.Node {
	rows := []gomponents.Node{}
	hasProviders := false

	for name, provider := range stats.ProviderStats {
		hasProviders = true

		totalReqs := provider.RequestsTotal

		successRate := 0.0
		if totalReqs > 0 {
			successRate = (float64(provider.SuccessCount) / float64(totalReqs)) * 100
		}

		rateClass := "danger"
		if successRate >= 95 {
			rateClass = "success"
		} else if successRate >= 80 {
			rateClass = "warning"
		}

		lastRequest := formatTimestamp(provider.LastRequestAt)
		nextAvailable := formatTimestamp(provider.NextAvailableAt)

		errorMsg := gomponents.Text("-")
		if provider.LastErrorMessage != "" {
			errorMsg = html.Span(
				html.Class("text-danger"),
				gomponents.Attr("title", provider.LastErrorMessage),
				gomponents.Text(truncateString(provider.LastErrorMessage, 30)),
			)
		}

		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(name))),
			html.Td(
				html.Class("text-right text-dark"),
				gomponents.Textf("%d", provider.Requests1h),
			),
			html.Td(
				html.Class("text-right text-dark"),
				gomponents.Textf("%d", provider.Requests24h),
			),
			html.Td(html.Class("text-right text-dark"), gomponents.Textf("%d", totalReqs)),
			html.Td(
				html.Class("text-right text-dark"),
				gomponents.Textf("%d", provider.SuccessCount),
			),
			html.Td(
				html.Class("text-right text-danger"),
				gomponents.Textf("%d", provider.FailureCount),
			),
			html.Td(
				html.Class("text-right text-dark"),
				gomponents.Textf("%.0fms", provider.AvgResponseTimeMs),
			),
			html.Td(
				html.Class("text-center"),
				html.Span(
					html.Class("badge badge-"+rateClass+" text-dark"),
					gomponents.Textf("%.1f%%", successRate),
				),
			),
			html.Td(html.Class("text-dark"), html.Small(gomponents.Text(lastRequest))),
			html.Td(html.Class("text-dark"), html.Small(gomponents.Text(nextAvailable))),
			html.Td(html.Small(errorMsg)),
		))
	}

	if !hasProviders {
		rows = append(rows, html.Tr(
			html.Td(gomponents.Attr("colspan", "11"), html.Class("text-center text-muted"),
				gomponents.Textf("No %s configured", title)),
		))
	}

	return html.Div(
		html.Div(
			html.Class("card-header bg-"+colorClass+" text-white"),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class("fa-solid fa-plug mr-2")),
				gomponents.Text(title),
			),
		),
		html.Div(html.Class("card-body"),
			html.Div(html.Class("table-responsive"),
				html.Table(html.Class("table table-sm table-hover"),
					html.THead(
						html.Tr(
							html.Th(gomponents.Text("Provider")),
							html.Th(html.Class("text-right"), gomponents.Text("Req 1h")),
							html.Th(html.Class("text-right"), gomponents.Text("Req 24h")),
							html.Th(html.Class("text-right"), gomponents.Text("Total")),
							html.Th(html.Class("text-right"), gomponents.Text("Success")),
							html.Th(html.Class("text-right"), gomponents.Text("Failed")),
							html.Th(html.Class("text-right"), gomponents.Text("Avg Time")),
							html.Th(html.Class("text-center"), gomponents.Text("Rate")),
							html.Th(gomponents.Text("Last Request")),
							html.Th(gomponents.Text("Next Available")),
							html.Th(gomponents.Text("Last Error")),
						),
					),
					html.TBody(rows...),
				),
			),
		),
	)
}

// Helper functions

func formatDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
