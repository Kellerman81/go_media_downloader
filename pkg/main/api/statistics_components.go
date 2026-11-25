package api

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
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

// storageStatsURL is the storage part of the statistics page.
const storageStatsURL = "/api/admin/statistics/storage"

// storageRefresh makes an element reload from url when a scan starts
// (storageScan) and, while one runs, every few seconds.
func storageRefresh(url, swap string, stats StorageStatistics) gomponents.Node {
	trigger := "storageScan from:body"
	if stats.Scanning {
		trigger += ", every 3s"
	}

	return gomponents.Group([]gomponents.Node{gomponents.Attr("hx-get", url), gomponents.Attr("hx-trigger", trigger), gomponents.Attr("hx-swap", swap)})
}

// storageScanButton starts a scan and shows the details again.
func storageScanButton(stats StorageStatistics, class string) gomponents.Node {
	label := "Scan now"
	if !stats.ScannedAt.IsZero() {
		label = "Scan again"
	}

	return html.Button(html.Type("button"), html.Class("btn btn-sm "+class), gomponents.If(stats.Scanning, html.Disabled()),
		gomponents.Attr("hx-get", storageStatsURL+"?type=detail&scan=1"), gomponents.Attr("hx-target", "#statStorage"), gomponents.Attr("hx-swap", "innerHTML"),
		html.I(html.Class("fa-solid fa-magnifying-glass-chart me-1")), gomponents.Text(label))
}

// storageScanState is one line on the scan: running, never run, or when
// the shown one ran.
func storageScanState(stats StorageStatistics) string {
	switch {
	case stats.Scanning:
		return fmt.Sprintf("Scanning… %d media files so far", stats.ScannedFiles)
	case stats.ScannedAt.IsZero():
		return "Not scanned yet"
	default:
		return fmt.Sprintf("Scanned %s ago (took %s)", formatDuration(int64(time.Since(stats.ScannedAt).Seconds())), stats.Took)
	}
}

func renderStorageCard(stats StorageStatistics) gomponents.Node {
	files := "–"
	if !stats.ScannedAt.IsZero() {
		files = strconv.FormatInt(stats.TotalFileCount, 10)
	}

	return html.Div(
		html.Class("card-body text-center"),
		storageRefresh(storageStatsURL+"?type=summary", "outerHTML", stats),
		html.H1(html.Class("display-4 text-info"), gomponents.Text(files)),
		html.P(
			html.Class("text-muted mb-2"),
			html.I(html.Class("fa-solid fa-hdd mr-2")),
			gomponents.Text("Files on disk"),
		),
		html.P(html.Class("mb-0 small"), gomponents.Text(storageScanState(stats))),
		gomponents.If(stats.Partial, html.P(html.Class("mb-0 text-warning small"),
			gomponents.Text("Partial: the scan timed out, counts are incomplete"),
		)),
		gomponents.If(stats.ScannedAt.IsZero() && !stats.Scanning, html.A(html.Href("#stat-storage"), html.Class("small"), gomponents.Text("Scan in Storage"))),
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

// renderLibraryCard renders a generic media-library summary card (music/books/audiobooks).
func renderLibraryCard(stats MediaLibraryStats, label, icon, colorClass string) gomponents.Node {
	return html.Div(
		html.Class("card-body text-center"),
		html.H1(html.Class("display-4 "+colorClass), gomponents.Textf("%d", stats.Total)),
		html.P(
			html.Class("text-muted mb-2"),
			html.I(html.Class(icon+" mr-2")),
			gomponents.Text(label),
		),
		html.P(html.Class("mb-0"),
			html.Span(html.Class("text-success mr-3"),
				gomponents.Textf("%d Available", stats.Available)),
			gomponents.Text(" | "),
			html.Span(html.Class("text-danger"),
				gomponents.Textf("%d Missing", stats.Missing)),
		),
	)
}

// renderLibraryDetails renders a generic media-library detail card (music/books/audiobooks).
func renderLibraryDetails(
	stats MediaLibraryStats,
	headerClass, qualityBadge, icon, title string,
) gomponents.Node {
	return renderMediaDetailsCard(
		stats.ByQuality,
		stats.ByList,
		headerClass,
		qualityBadge,
		icon,
		title,
	)
}

// renderMediaDetailsCard renders a media "details" card with By-Quality and By-List
// tables. headerClass is the card-header colour class, qualityBadge the quality count
// badge class, icon the header icon class, and title the card title. Shared by the
// movie and series detail sections.
func renderMediaDetailsCard(
	byQuality, byList map[string]int,
	headerClass, qualityBadge, icon, title string,
) gomponents.Node {
	qualityRows := []gomponents.Node{}
	if len(byQuality) == 0 {
		qualityRows = append(qualityRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "2"), html.Class("text-center text-muted"),
				gomponents.Text("No data")),
		))
	} else {
		for _, quality := range slices.Sorted(maps.Keys(byQuality)) {
			count := byQuality[quality]
			qualityRows = append(qualityRows, html.Tr(
				html.Td(gomponents.Text(quality)),
				html.Td(
					html.Class("text-right"),
					html.Span(
						html.Class(qualityBadge),
						gomponents.Textf("%d", count),
					),
				),
			))
		}
	}

	listRows := []gomponents.Node{}
	if len(byList) == 0 {
		listRows = append(listRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "2"), html.Class("text-center text-muted"),
				gomponents.Text("No data")),
		))
	} else {
		for _, list := range slices.Sorted(maps.Keys(byList)) {
			count := byList[list]
			listRows = append(listRows, html.Tr(
				html.Td(gomponents.Text(list)),
				html.Td(
					html.Class("text-right"),
					html.Span(
						html.Class("badge badge-info text-dark"),
						gomponents.Textf("%d", count),
					),
				),
			))
		}
	}

	return html.Div(
		html.Div(
			html.Class(headerClass),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class(icon)),
				gomponents.Text(title),
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

// renderMovieDetails renders the movie details section.
func renderMovieDetails(stats MovieStatistics) gomponents.Node {
	return renderMediaDetailsCard(
		stats.ByQuality, stats.ByList,
		"card-header bg-primary text-white",
		"badge badge-primary text-dark",
		"fa-solid fa-film mr-2",
		"Movie Details",
	)
}

// renderSeriesDetails renders the series details section.
func renderSeriesDetails(stats SeriesStatistics) gomponents.Node {
	return renderMediaDetailsCard(
		stats.ByQuality, stats.ByList,
		"card-header bg-success text-white",
		"badge badge-success text-dark",
		"fa-solid fa-tv mr-2",
		"Series Details",
	)
}

// renderStorageDetails renders the storage details section.
func renderStorageDetails(stats StorageStatistics) gomponents.Node {
	header := html.Div(
		html.Class("card-header bg-info text-white d-flex flex-wrap align-items-center gap-2"),
		html.H5(html.Class("mb-0 me-auto"), html.I(html.Class("fa-solid fa-hdd mr-2")), gomponents.Text("Storage Details")),
		html.Span(html.Class("small text-body-secondary"), gomponents.Text(storageScanState(stats))),
		storageScanButton(stats, "btn-outline-primary"),
	)

	// While a scan runs the details reload until it's done.
	var poll gomponents.Node
	if stats.Scanning {
		poll = html.Div(gomponents.Attr("hx-get", storageStatsURL+"?type=detail"), gomponents.Attr("hx-trigger", "load delay:3s"),
			gomponents.Attr("hx-target", "#statStorage"), gomponents.Attr("hx-swap", "innerHTML"))
	}

	if stats.ScannedAt.IsZero() {
		text := "Counts the media files and their size in every configured media and import folder. " +
			"It walks every folder, which can take minutes on a large library, so it only runs when you start it."
		if stats.Scanning {
			text = "The scan runs in the background; you can leave the page and come back."
		}

		return gomponents.Group([]gomponents.Node{header, html.Div(html.Class("card-body"),
			html.P(html.Class("text-muted mb-0"), gomponents.Text(text)), poll)})
	}

	var totalSize int64
	for _, path := range stats.ByPath {
		totalSize += path.TotalSize
	}

	pathRows := []gomponents.Node{}
	if len(stats.ByPath) == 0 {
		pathRows = append(pathRows, html.Tr(
			html.Td(gomponents.Attr("colspan", "5"), html.Class("text-center text-muted"),
				gomponents.Text("No media folders configured")),
		))
	}

	for _, mediaType := range slices.Sorted(maps.Keys(stats.ByPath)) {
		info := stats.ByPath[mediaType]

		var note gomponents.Node

		switch {
		case info.Error != "":
			note = html.Div(html.Class("small text-danger"), gomponents.Text(info.Error))
		case info.Partial:
			note = html.Div(html.Class("small text-warning"), gomponents.Text("partial - the scan timed out"))
		}

		pathRows = append(pathRows, html.Tr(
			html.Td(html.Strong(gomponents.Text(mediaType))),
			html.Td(html.Small(html.Class("text-muted"), gomponents.Text(info.Path)), note,
				gomponents.If(info.Unreadable > 0, html.Div(html.Class("small text-warning"),
					gomponents.Textf("%d entries could not be read", info.Unreadable)))),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", info.FileCount)),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", info.FolderCount)),
			html.Td(html.Class("text-end text-nowrap"), gomponents.Text(formatBytes(uint64(info.TotalSize)))),
		))
	}

	return gomponents.Group([]gomponents.Node{
		header,
		html.Div(html.Class("card-body"),
			poll,
			gomponents.If(stats.Partial, html.Div(html.Class("alert alert-warning py-2"),
				gomponents.Textf("The scan stopped after %s: the counts of the marked folders are incomplete.", storageScanTimeout))),
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
			html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm table-hover"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Text("Media Type")),
						html.Th(gomponents.Text("Path")),
						html.Th(html.Class("text-end"), gomponents.Text("Files")),
						html.Th(html.Class("text-end"), gomponents.Text("Folders")),
						html.Th(html.Class("text-end"), gomponents.Text("Size")),
					),
				),
				html.TBody(pathRows...),
			)),
		),
	})
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

	rows := make([]gomponents.Node, 0, len(pools))
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

	for _, name := range slices.Sorted(maps.Keys(stats.ClientStats)) {
		client := stats.ClientStats[name]
		hasClients = true

		totalReqs := client.RequestsTotal
		rateClass, rateText := successRateBadge(client.SuccessCount, client.FailureCount)

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
					gomponents.Text(rateText),
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

	for _, name := range slices.Sorted(maps.Keys(stats.ProviderStats)) {
		provider := stats.ProviderStats[name]
		hasProviders = true

		totalReqs := provider.RequestsTotal
		rateClass, rateText := successRateBadge(provider.SuccessCount, provider.FailureCount)

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
					gomponents.Text(rateText),
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
	} else if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	return t.Format("2006-01-02 15:04:05")
}

func truncateString(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}

	return string(r[:maxLen-3]) + "..."
}

// successRateBadge returns the badge colour and text for a success rate.
// RequestsTotal also counts rate-limited (429) responses, which the base client
// deliberately records as neither success nor failure, so the rate is taken
// over completed requests only.
func successRateBadge(success, failure int64) (string, string) {
	completed := success + failure
	if completed == 0 {
		return "secondary", "-"
	}

	rate := float64(success) / float64(completed) * 100

	switch {
	case rate >= 95:
		return "success", fmt.Sprintf("%.1f%%", rate)
	case rate >= 80:
		return "warning", fmt.Sprintf("%.1f%%", rate)
	default:
		return "danger", fmt.Sprintf("%.1f%%", rate)
	}
}
