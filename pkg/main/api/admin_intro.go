package api

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderModernAdminIntro creates the admin landing page: setup checklist for
// new installs, live library/activity overview and a guide to the other pages.
func renderModernAdminIntro() gomponents.Node {
	return html.Div(
		html.Class("container-fluid p-4 dashboard"),
		dashboardStyle(),
		dashboardHeader(),
		dashboardSetupChecklist(),

		// Live-updating region (refreshed via HTMX without a full reload).
		html.Div(
			html.ID("dashboard-cards"),
			hx.Get("/api/admin/dashboard/cards"),
			hx.Trigger("every 30s"),
			hx.Swap("innerHTML"),
			renderDashboardCards(),
		),

		dashboardGuide(),
	)
}

func dashboardStyle() gomponents.Node {
	return html.StyleEl(gomponents.Raw(`
		.dashboard .dash-card { border: 0; border-radius: 12px; box-shadow: 0 1px 3px rgba(0,0,0,.08); }
		.dashboard a.dash-link-card { text-decoration: none; color: inherit; display: block; height: 100%; }
		.dashboard a.dash-link-card .dash-card { transition: transform .15s ease, box-shadow .15s ease; height: 100%; }
		.dashboard a.dash-link-card:hover .dash-card { transform: translateY(-2px); box-shadow: 0 6px 16px rgba(0,0,0,.12); }
		.dashboard .dash-icon { width: 40px; height: 40px; border-radius: 10px; display: inline-flex; align-items: center; justify-content: center;
			color: var(--dash-color); background: color-mix(in srgb, var(--dash-color) 12%, transparent); flex-shrink: 0; }
		.dashboard .dash-step { display: flex; gap: .75rem; align-items: flex-start; padding: .6rem 0; border-bottom: 1px solid #f1f3f5; }
		.dashboard .dash-step:last-child { border-bottom: 0; }
		.dashboard .dash-guide-item { display: flex; gap: .6rem; padding: .45rem .5rem; border-radius: 8px; text-decoration: none; color: inherit; }
		.dashboard .dash-guide-item:hover { background: #f8f9fa; }
		.dashboard .dash-list li { padding: .4rem 0; border-bottom: 1px solid #f1f3f5; }
		.dashboard .dash-list li:last-child { border-bottom: 0; }
	`))
}

// dashboardHeader renders the page title row.
func dashboardHeader() gomponents.Node {
	return html.Div(
		html.Class("d-flex align-items-center justify-content-between flex-wrap gap-2 mb-4"),
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
				gomponents.Text("Your library at a glance, what's happening right now, and where to find everything else"),
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

// setupStep is one item of the getting-started checklist.
type setupStep struct {
	title, help, href, linkText string
	done, optional              bool
}

// dashboardSetupSteps checks the configuration for the pieces a working
// install needs. The placeholders a new config starts with (ClearCfg: an
// "initial" drone downloader, an example.com indexer, an "initial" CSV
// notification, paths without a folder) don't count.
func dashboardSetupSteps() []setupStep {
	hasDownloader := slices.ContainsFunc(config.GetSettingsDownloaderAll(), func(d config.DownloaderConfig) bool {
		return d.Hostname != "" || (strings.EqualFold(d.DlType, "drone") && d.Name != "initial")
	})
	hasIndexer := slices.ContainsFunc(config.GetSettingsIndexerAll(), func(i config.IndexersConfig) bool {
		return i.URL != "" && !strings.Contains(i.URL, "example.com")
	})
	hasPath := slices.ContainsFunc(config.GetSettingsPathAll(), func(p config.PathsConfig) bool {
		return p.Path != ""
	})
	hasList := slices.ContainsFunc(config.GetSettingsListAll(), func(l config.ListsConfig) bool {
		return l.Enabled
	})
	hasNotification := slices.ContainsFunc(config.GetSettingsNotificationAll(), func(n config.NotificationConfig) bool {
		return n.NotificationType != "" && n.Name != "initial"
	})

	hasItems := countRow("SELECT (SELECT COUNT(*) FROM movies) + (SELECT COUNT(*) FROM series) + "+
		"(SELECT COUNT(*) FROM albums) + (SELECT COUNT(*) FROM books) + (SELECT COUNT(*) FROM audiobooks)") > 0

	return []setupStep{
		{
			title: "Connect a download client", href: "/api/admin/config/downloader", linkText: "Download clients",
			help: "SABnzbd, NZBGet, qBittorrent, Transmission, Deluge or rTorrent - this is what actually downloads the releases.",
			done: hasDownloader,
		},
		{
			title: "Add an indexer", href: "/api/admin/config/indexers", linkText: "Indexers",
			help: "A Newznab/Torznab indexer is where releases are searched. Without one nothing can be found.",
			done: hasIndexer,
		},
		{
			title: "Set your library folders", href: "/api/admin/config/paths", linkText: "Paths",
			help: "Where finished downloads are moved to and renamed, and which folders are scanned for existing files.",
			done: hasPath,
		},
		{
			title: "Add something to your library", href: "/api/admin/metadata-search/movies", linkText: "Add media",
			help: "Search a movie, series, album or book by name and add it - or let a list fill the library for you.",
			done: hasItems,
		},
		{
			title: "Import a list", href: "/api/admin/config/lists", linkText: "Lists",
			help: "Trakt, IMDB, TMDB, Plex/Jellyfin watchlists or RSS feeds keep adding wanted items automatically.",
			done: hasList, optional: true,
		},
		{
			title: "Get notified", href: "/api/admin/config/notifications", linkText: "Notifications",
			help: "Pushover, Gotify, Pushbullet, Apprise or e-mail when something was downloaded.",
			done: hasNotification, optional: true,
		},
	}
}

// dashboardSetupChecklist shows the getting-started steps while a required
// one is still open; afterwards it shrinks to a one-line confirmation.
func dashboardSetupChecklist() gomponents.Node {
	steps := dashboardSetupSteps()

	required, requiredDone := 0, 0
	for _, s := range steps {
		if !s.optional {
			required++
			if s.done {
				requiredDone++
			}
		}
	}

	if requiredDone == required {
		return html.Div(
			html.Class("alert alert-success d-flex align-items-center gap-2 py-2 mb-4"),
			html.I(html.Class("fas fa-circle-check"), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text("Setup complete - downloads run on the schedules in the "),
			html.A(html.Href("/api/admin/grid/scheduler"), gomponents.Text("Scheduler")),
			gomponents.Text(". New here? The guide at the bottom explains every area."),
		)
	}

	items := make([]gomponents.Node, 0, len(steps))
	for i, s := range steps {
		icon, color, titleClass, btnClass := "fa-regular fa-circle", "#adb5bd", "fw-semibold", "btn-primary"
		if s.done {
			icon, color = "fas fa-circle-check", "#198754"
			titleClass, btnClass = "fw-semibold text-muted text-decoration-line-through", "btn-outline-secondary"
		}

		label := fmt.Sprintf("%d. %s", i+1, s.title)
		if s.optional {
			label += " (optional)"
		}

		items = append(items, html.Div(
			html.Class("dash-step"),
			html.I(html.Class(icon+" mt-1"), html.Style("color:"+color+";font-size:1.2rem"), gomponents.Attr("aria-hidden", "true")),
			html.Div(
				html.Class("flex-grow-1"),
				html.Div(html.Class(titleClass), gomponents.Text(label)),
				html.Small(html.Class("text-muted"), gomponents.Text(s.help)),
			),
			html.A(
				html.Href(s.href),
				html.Class("btn btn-sm "+btnClass),
				gomponents.Text(s.linkText),
			),
		))
	}

	return html.Div(
		html.Class("card dash-card mb-4"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex align-items-center justify-content-between flex-wrap gap-2 mb-2"),
				html.Div(
					html.H2(html.Class("h5 mb-0"), gomponents.Text("Getting started")),
					html.Small(
						html.Class("text-muted"),
						gomponents.Textf("%d of %d required steps done", requiredDone, required),
					),
				),
				html.A(
					html.Href("/api/admin/wizard"),
					html.Class("btn btn-success btn-sm"),
					html.I(html.Class("fas fa-wand-magic-sparkles me-1"), gomponents.Attr("aria-hidden", "true")),
					gomponents.Text("Run the Setup Wizard"),
				),
			),
			html.Div(
				html.Class("progress mb-2"),
				html.Style("height: 6px;"),
				html.Div(
					html.Class("progress-bar bg-success"),
					html.Style(fmt.Sprintf("width: %d%%", requiredDone*100/required)),
				),
			),
			gomponents.Group(items),
		),
	)
}

// countRow returns the row count of a query, or 0 on any error.
func countRow(query string) int {
	return int(database.Getdatarow[uint](false, query))
}

// renderDashboardCards builds the live library cards and the activity panels.
// It is also served as an HTMX fragment for periodic refresh.
func renderDashboardCards() gomponents.Node {
	ctx := context.Background()
	media := config.GetSettingsMediaAll()

	var cards []gomponents.Node

	if media == nil || len(media.Movies) > 0 {
		s, _ := getMovieStatistics(ctx)
		cards = append(cards, statCard("fas fa-film", "#0d6efd", "Movies",
			s.TotalMovies, fmt.Sprintf("%d missing · %d have files", s.MissingMovies, s.AvailableMovies),
			"/api/admin/database/movies", s.MissingMovies > 0))
	}

	if media == nil || len(media.Series) > 0 {
		s, _ := getSeriesStatistics(ctx)
		cards = append(cards, statCard("fas fa-tv", "#6610f2", "Series",
			s.TotalSeries, fmt.Sprintf("%d episodes · %d missing", s.TotalEpisodes, s.MissingEpisodes),
			"/api/admin/database/series", s.MissingEpisodes > 0))
	}

	type simpleType struct {
		configured        bool
		icon, color, name string
		table             string
	}

	for _, t := range []simpleType{
		{media != nil && len(media.Music) > 0, "fas fa-compact-disc", "#d63384", "Albums", "albums"},
		{media != nil && len(media.Books) > 0, "fas fa-book", "#fd7e14", "Books", "books"},
		{media != nil && len(media.AudioBooks) > 0, "fas fa-headphones", "#20c997", "Audiobooks", "audiobooks"},
	} {
		if !t.configured {
			continue
		}

		missing := countRow("SELECT COUNT(*) FROM " + t.table + " WHERE missing = 1")
		cards = append(cards, statCard(t.icon, t.color, t.name,
			countRow("SELECT COUNT(*) FROM "+t.table), fmt.Sprintf("%d missing", missing),
			"/api/admin/database/"+t.table, missing > 0))
	}

	schedules := worker.GetSchedules()

	running, stopped := 0, 0
	for _, s := range schedules {
		if s.IsRunning {
			running++
		}

		if s.Paused {
			stopped++
		}
	}

	cards = append(cards, statCard("fas fa-list-ol", "#0dcaf0", "Queue",
		len(worker.GetQueues()), fmt.Sprintf("%d of %d scheduled jobs running", running, len(schedules)),
		"/api/admin/grid/queue", false))

	return gomponents.Group([]gomponents.Node{
		html.H2(html.Class("h5 mb-3"), gomponents.Text("Your library")),
		html.Div(append([]gomponents.Node{html.Class("row g-3 mb-4")}, cards...)...),
		html.Div(
			html.Class("row g-3 mb-4"),
			html.Div(html.Class("col-lg-7"), dashboardRecentDownloads()),
			html.Div(html.Class("col-lg-5"), dashboardUpcoming(schedules, stopped)),
		),
		dashboardSystemLine(),
	})
}

// statCard renders a single library statistic card linking to its table.
func statCard(icon, color, title string, count int, sub, href string, warn bool) gomponents.Node {
	subClass := "text-muted mb-0 small"
	if warn {
		subClass = "text-danger mb-0 small fw-semibold"
	}

	return html.Div(
		html.Class("col-xl col-md-4 col-sm-6"),
		html.A(
			html.Href(href),
			html.Class("dash-link-card"),
			html.Div(
				html.Class("card dash-card"),
				html.Div(
					html.Class("card-body"),
					html.Div(
						html.Class("d-flex align-items-center justify-content-between mb-2"),
						html.Span(html.Class("text-muted text-uppercase small fw-semibold"), gomponents.Text(title)),
						html.Span(
							html.Class("dash-icon"),
							html.Style("--dash-color:"+color),
							html.I(html.Class(icon), gomponents.Attr("aria-hidden", "true")),
						),
					),
					html.Div(html.Class("h3 mb-1"), gomponents.Textf("%d", count)),
					html.P(html.Class(subClass), gomponents.Text(sub)),
				),
			),
		),
	)
}

// recentDownload is one row of the recent downloads panel.
type recentDownload struct {
	Kind    string `db:"kind"`
	Title   string `db:"title"`
	Indexer string `db:"indexer"`
	At      string `db:"at"`
}

// dashboardRecentDownloads lists the latest grabs over all media types.
func dashboardRecentDownloads() gomponents.Node {
	const perType = "SELECT * FROM (SELECT '%s' AS kind, title, indexer, strftime('%%Y-%%m-%%d %%H:%%M', downloaded_at) AS at FROM %s ORDER BY downloaded_at DESC LIMIT 8)"

	parts := []string{
		fmt.Sprintf(perType, "Movie", "movie_histories"),
		fmt.Sprintf(perType, "Episode", "serie_episode_histories"),
		fmt.Sprintf(perType, "Album", "album_histories"),
		fmt.Sprintf(perType, "Book", "book_histories"),
		fmt.Sprintf(perType, "Audiobook", "audiobook_histories"),
	}

	rows := database.StructscanT[recentDownload](false, 8,
		strings.Join(parts, " UNION ALL ")+" ORDER BY at DESC LIMIT 8")

	var body gomponents.Node
	if len(rows) == 0 {
		body = html.P(html.Class("text-muted mb-0"),
			gomponents.Text("Nothing downloaded yet. Once searches find releases, they show up here."))
	} else {
		items := make([]gomponents.Node, 0, len(rows))
		for _, r := range rows {
			items = append(items, html.Li(
				html.Class("d-flex justify-content-between gap-3"),
				html.Div(
					html.Class("text-truncate"),
					html.Span(html.Class("badge bg-light text-dark me-2"), gomponents.Text(r.Kind)),
					html.Span(html.Title(r.Title), gomponents.Text(r.Title)),
				),
				html.Small(html.Class("text-muted text-nowrap"), gomponents.Text(r.At+" · "+r.Indexer)),
			))
		}

		body = html.Ul(append([]gomponents.Node{html.Class("list-unstyled mb-0 dash-list")}, items...)...)
	}

	return html.Div(
		html.Class("card dash-card h-100"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-center mb-2"),
				html.H2(html.Class("h6 mb-0"), gomponents.Text("Recent downloads")),
				html.A(html.Href("/api/admin/statistics"), html.Class("small"), gomponents.Text("Statistics →")),
			),
			body,
		),
	)
}

// dashboardUpcoming lists the next scheduled jobs and anything that needs a look.
func dashboardUpcoming(schedules map[uint32]syncops.JobSchedule, stopped int) gomponents.Node {
	next := make([]syncops.JobSchedule, 0, len(schedules))
	for _, s := range schedules {
		if !s.Paused && !s.NextRun.IsZero() {
			next = append(next, s)
		}
	}

	slices.SortFunc(next, func(a, b syncops.JobSchedule) int { return a.NextRun.Compare(b.NextRun) })

	items := make([]gomponents.Node, 0, 6)
	for _, s := range next[:min(len(next), 6)] {
		items = append(items, html.Li(
			html.Class("d-flex justify-content-between gap-3"),
			html.Span(html.Class("text-truncate"), html.Title(s.JobName), gomponents.Text(s.JobName)),
			html.Small(html.Class("text-muted text-nowrap"), gomponents.Text(untilText(s.NextRun))),
		))
	}

	var attention []gomponents.Node
	if stopped > 0 {
		attention = append(attention, attentionLine(
			fmt.Sprintf("%d scheduled job(s) are stopped", stopped), "/api/admin/grid/scheduler", "Scheduler"))
	}

	if blocked := blockedIndexers(); blocked > 0 {
		attention = append(attention, attentionLine(
			fmt.Sprintf("%d indexer(s) paused after errors", blocked), "/api/admin/service-health", "Service health"))
	}

	body := []gomponents.Node{html.Class("list-unstyled mb-0 dash-list")}
	if len(items) == 0 {
		body = append(body, html.Li(html.Class("text-muted"), gomponents.Text("No scheduled jobs. Add them under Configuration → Scheduler.")))
	}

	return html.Div(
		html.Class("card dash-card h-100"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-center mb-2"),
				html.H2(html.Class("h6 mb-0"), gomponents.Text("Coming up")),
				html.A(html.Href("/api/admin/grid/scheduler"), html.Class("small"), gomponents.Text("All schedules →")),
			),
			gomponents.Group(attention),
			html.Ul(append(body, items...)...),
		),
	)
}

func attentionLine(text, href, linkText string) gomponents.Node {
	return html.Div(
		html.Class("alert alert-warning py-1 px-2 mb-2 small d-flex justify-content-between gap-2"),
		html.Span(
			html.I(html.Class("fas fa-triangle-exclamation me-1"), gomponents.Attr("aria-hidden", "true")),
			gomponents.Text(text),
		),
		html.A(html.Href(href), gomponents.Text(linkText)),
	)
}

// blockedIndexers counts indexers inside their failure block window.
func blockedIndexers() int {
	block := config.GetSettingsGeneral().FailedIndexerBlockTime
	if block == 0 {
		return 0
	}

	since := time.Now().Add(-time.Duration(block) * time.Minute)

	return int(database.Getdatarow[uint](false,
		"SELECT COUNT(DISTINCT indexer) FROM indexer_fails WHERE last_fail > ?", &since))
}

// untilText renders the time until t compactly ("in 5m", "in 2h 10m", "now").
func untilText(t time.Time) string {
	d := time.Until(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("in %dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return "on " + t.Format("Jan 2 15:04")
	}
}

// dashboardSystemLine renders a compact system status line.
func dashboardSystemLine() gomponents.Node {
	sys := getSystemStatistics()

	return html.Div(
		html.Class("d-flex flex-wrap gap-4 align-items-center small text-muted mb-4"),
		html.Span(html.I(html.Class("fas fa-server me-1"), gomponents.Attr("aria-hidden", "true")), gomponents.Text("System")),
		html.Span(gomponents.Text("Uptime "+formatUptime(sys.UptimeSeconds))),
		html.Span(gomponents.Textf("Memory %.0f MB", sys.MemoryAllocMB)),
		html.Span(gomponents.Textf("Goroutines %d", sys.NumGoroutine)),
		html.Span(gomponents.Textf("%d CPUs · %s/%s", sys.NumCPU, sys.GOOS, sys.GOARCH)),
		html.Span(html.Class("ms-auto"), gomponents.Text("Updated "+time.Now().Format("15:04:05"))),
	)
}

// guideLink is one page in the "where to find what" guide.
type guideLink struct {
	icon, label, desc, href string
}

// dashboardGuide explains the areas of the app and links to their pages.
func dashboardGuide() gomponents.Node {
	type section struct {
		icon, color, title, intro string
		links                     []guideLink
	}

	sections := []section{
		{
			"fas fa-plus", "#0d6efd", "Add media", "Put things on your wanted list.",
			addMediaGuideLinks(),
		},
		{
			"fas fa-magnifying-glass", "#fd7e14", "Find & download", "Search releases yourself or see what's still missing.",
			[]guideLink{
				{"fas fa-magnifying-glass-arrow-right", "Search & Download", "Search the indexers for one item and pick a release", "/api/admin/searchdownload"},
				{"fas fa-bullseye", "Wanted", "Everything that is still missing or can be upgraded", "/api/admin/wanted"},
				{"fas fa-calendar-alt", "Calendar", "Upcoming and recent episodes and releases", "/api/admin/calendar"},
				{"fas fa-list-check", "Missing episodes", "Series episodes without a file", "/api/admin/missing-episodes"},
			},
		},
		{
			"fas fa-wave-square", "#20c997", "Watch activity", "See what the app is doing.",
			[]guideLink{
				{"fas fa-list-ol", "Queue", "Jobs running or waiting right now", "/api/admin/grid/queue"},
				{"fas fa-clock", "Scheduler", "Recurring jobs - start, stop or run one now", "/api/admin/grid/scheduler"},
				{"fas fa-chart-line", "Statistics", "Downloads, backlog, job runtimes, library quality", "/api/admin/statistics"},
				{"fas fa-file-lines", "Logs", "Read the application log", "/api/admin/logviewer"},
			},
		},
		{
			"fas fa-gear", "#6c757d", "Configure", "Change how searching, downloading and sorting work.",
			[]guideLink{
				{"fas fa-sliders", "General", "Ports, API keys, logging, caching", "/api/admin/config/general"},
				{"fas fa-photo-film", "Media", "Media groups: lists, quality, paths per type", "/api/admin/config/media"},
				{"fas fa-star", "Quality", "Which resolutions/codecs are wanted and preferred", "/api/admin/config/quality"},
				{"fas fa-satellite-dish", "Indexers & downloaders", "Where to search and what downloads", "/api/admin/config/indexers"},
				{"fas fa-folder-tree", "Paths & naming", "Library folders and file naming", "/api/admin/config/paths"},
			},
		},
		{
			"fas fa-stethoscope", "#dc3545", "Troubleshoot", "When something doesn't get found or sorted.",
			[]guideLink{
				{"fas fa-heart-pulse", "Service health", "Are indexers, downloaders and APIs reachable?", "/api/admin/service-health"},
				{"fas fa-flask", "Test parse", "How a release name is understood", "/api/admin/testparse"},
				{"fas fa-signature", "Naming test", "Preview the folder and file name for an item", "/api/admin/namingtest"},
				{"fas fa-database", "Database", "Browse and edit every table", "/api/admin/database/movies"},
			},
		},
	}

	cols := make([]gomponents.Node, 0, len(sections))
	for _, s := range sections {
		links := make([]gomponents.Node, 0, len(s.links))
		for _, l := range s.links {
			links = append(links, html.A(
				html.Href(l.href),
				html.Class("dash-guide-item"),
				html.I(html.Class(l.icon+" mt-1 text-muted"), html.Style("width:1.1rem"), gomponents.Attr("aria-hidden", "true")),
				html.Div(
					html.Div(html.Class("fw-semibold small"), gomponents.Text(l.label)),
					html.Div(html.Class("text-muted small"), gomponents.Text(l.desc)),
				),
			))
		}

		cols = append(cols, html.Div(
			html.Class("col-xl col-md-6"),
			html.Div(
				html.Class("card dash-card h-100"),
				html.Div(
					html.Class("card-body"),
					html.Div(
						html.Class("d-flex align-items-center gap-2 mb-1"),
						html.Span(html.Class("dash-icon"), html.Style("--dash-color:"+s.color),
							html.I(html.Class(s.icon), gomponents.Attr("aria-hidden", "true"))),
						html.H3(html.Class("h6 mb-0"), gomponents.Text(s.title)),
					),
					html.P(html.Class("text-muted small mb-2"), gomponents.Text(s.intro)),
					gomponents.Group(links),
				),
			),
		))
	}

	return html.Div(
		html.H2(html.Class("h5 mb-1"), gomponents.Text("Where to find what")),
		html.P(html.Class("text-muted small mb-3"),
			gomponents.Text("Everything is also in the sidebar on the left. The most common pages:")),
		html.Div(append([]gomponents.Node{html.Class("row g-3")}, cols...)...),
	)
}

// addMediaGuideLinks returns the "add media" pages of the configured media types.
func addMediaGuideLinks() []guideLink {
	media := config.GetSettingsMediaAll()
	if media == nil {
		return nil
	}

	var links []guideLink
	if len(media.Movies) > 0 {
		links = append(links, guideLink{"fas fa-film", "Movies", "Search TMDB/IMDB and add a movie", "/api/admin/metadata-search/movies"})
	}

	if len(media.Series) > 0 {
		links = append(links, guideLink{"fas fa-tv", "Series", "Add a TV show with all its episodes", "/api/admin/metadata-search/series"})
	}

	if len(media.Music) > 0 {
		links = append(links, guideLink{"fas fa-compact-disc", "Music", "Add albums or whole artists", "/api/admin/metadata-search/music"})
	}

	if len(media.Books) > 0 {
		links = append(links, guideLink{"fas fa-book", "Books", "Add books by title or author", "/api/admin/metadata-search/books"})
	}

	if len(media.AudioBooks) > 0 {
		links = append(links, guideLink{"fas fa-headphones", "Audiobooks", "Add audiobooks by title, author or series", "/api/admin/metadata-search/audiobooks"})
	}

	if len(media.Music) == 0 || len(media.Books) == 0 || len(media.AudioBooks) == 0 {
		links = append(links, guideLink{"fas fa-photo-film", "More media types", "Enable music, books or audiobooks under Configuration → Media", "/api/admin/config/media"})
	}

	return links
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
