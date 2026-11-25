package api

import (
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// createNavbarFor is the sidebar of an account: the full one for admins
// (nil: an admin page), else only the pages of its role.
func createNavbarFor(acc *userAccess, activeDatabase bool) gomponents.Node {
	if acc != nil && acc.App {
		return appNavbar(acc)
	}

	if acc == nil || acc.Admin() {
		return createNavbar(activeDatabase)
	}

	link := func(href, icon, label string) gomponents.Node {
		return html.Li(html.Class("sidebar-item"),
			html.A(html.Class("sidebar-link"), html.Href(href),
				html.I(html.Class("align-middle fa-solid "+icon)),
				html.Span(html.Class("align-middle"), gomponents.Text(label))))
	}

	return html.Nav(html.ID("sidebar"), html.Class("sidebar js-sidebar"), gomponents.Attr("aria-label", "Main navigation"),
		html.Div(html.Class("sidebar-content js-simplebar"),
			html.A(html.Class("sidebar-brand"), html.Href("/api/admin/library"),
				html.Span(html.Class("sidebar-brand-text align-middle"), gomponents.Text("Go Media"))),
			html.Ul(html.Class("sidebar-nav"),
				html.Li(html.Class("sidebar-header"), gomponents.Text(t("Pages"))),
				link("/api/admin/library", "fa-photo-film", t("Library")),
				link("/api/admin/calendar", "fa-calendar-alt", t("Calendar")),
				gomponents.If(acc.CanRequest(), link("/api/admin/metadata-search", "fa-plus", t("Add Media"))),
				link("/api/admin/account", "fa-user", t("My Account")),
				link("/api/admin/help", "fa-circle-question", t("Help")),
				link("/api/logout", "fa-right-from-bracket", t("Log out")),
			)))
}

// requestsBadge is the number of open media requests, when there are any.
func requestsBadge() gomponents.Node {
	n := database.CountMediaRequests()
	if n == 0 {
		return nil
	}

	return html.Span(html.Class("badge bg-warning text-dark ms-2"), gomponents.Text(strconv.FormatUint(uint64(n), 10)))
}

// accountMenu is the top bar's account menu: the name, the own account page
// and logging out.
func accountMenu(acc *userAccess) gomponents.Node {
	name := DefaultUsername
	if acc != nil && acc.Name != "" {
		name = acc.Name
	}

	return html.Div(html.Class("navbar-collapse collapse"),
		html.Ul(html.Class("navbar-nav navbar-align ms-auto"),
			notificationBell(acc),
			html.Li(html.Class("nav-item dropdown"),
				html.A(html.Class("nav-link dropdown-toggle"), html.Href("#"), html.Data("bs-toggle", "dropdown"),
					html.I(html.Class("fa-solid fa-user me-1")), gomponents.Text(name)),
				html.Div(html.Class("dropdown-menu dropdown-menu-end"),
					html.A(html.Class("dropdown-item"), html.Href("/api/admin/account"),
						html.I(html.Class("fa-solid fa-key me-2")), gomponents.Text(t("My Account"))),
					themeMenuItems(),
					html.A(html.Class("dropdown-item"), html.Href("/api/logout"),
						html.I(html.Class("fa-solid fa-right-from-bracket me-2")), gomponents.Text(t("Log out")))))))
}

// navLink is an entry of the admin sidebar.
type navLink struct {
	href, icon, label string
	newTab            bool
	badge             func() gomponents.Node // a count after the label
}

// navSection is a collapsible group of the admin sidebar.
type navSection struct {
	id, icon, label string
	links           []navLink
}

// adminNav is the admin sidebar: the everyday pages on top, then the
// groups from daily work to rarely used tools.
var (
	adminNavPages = []navLink{
		{href: "/api/admin", icon: "fa-gauge", label: "Dashboard"},
		{href: "/api/admin/library", icon: "fa-photo-film", label: "Library"},
		{href: "/api/admin/calendar", icon: "fa-calendar-alt", label: "Calendar"},
		{href: "/api/admin/metadata-search", icon: "fa-plus", label: "Add Media"},
		{href: "/api/admin/wanted", icon: "fa-bullseye", label: "Wanted"},
		{href: "/api/admin/requests", icon: "fa-inbox", label: "Requests", badge: requestsBadge},
		{href: "/api/admin/help", icon: "fa-circle-question", label: "Help"},
	}
	adminNavSections = []navSection{
		{id: "Activity", icon: "fa-bolt", label: "Activity", links: []navLink{
			{href: "/api/admin/grid/queue", icon: "fa-list-ol", label: "Queue"},
			{href: "/api/admin/grid/scheduler", icon: "fa-calendar", label: "Scheduled Jobs"},
			{href: "/api/admin/jobmanagement", icon: "fa-tasks", label: "Job Management"},
			{href: "/api/admin/nowplaying", icon: "fa-circle-play", label: "Now Playing"},
			{href: "/api/admin/statistics", icon: "fa-chart-line", label: "Statistics Dashboard"},
			{href: "/api/admin/grid/stats", icon: "fa-chart-bar", label: "Media Statistics"},
			{href: "/api/admin/reports", icon: "fa-flag", label: "Reports"},
		}},
		{id: "LibraryTasks", icon: "fa-wand-sparkles", label: "Library Tasks", links: []navLink{
			{href: "/api/admin/searchdownload", icon: "fa-magnifying-glass-arrow-right", label: "Search & Download"},
			{href: "/api/admin/missing-episodes", icon: "fa-exclamation-triangle", label: "Missing Episodes"},
			{href: "/api/admin/media-cleanup", icon: "fa-broom", label: "Media Cleanup"},
			{href: "/api/admin/exclusions", icon: "fa-ban", label: "Import Exclusions"},
		}},
		{id: "Configuration", icon: "fa-gear", label: "Configuration", links: []navLink{
			{href: "/api/admin/wizard", icon: "fa-wand-magic-sparkles", label: "Setup Wizard"},
			{href: "/api/admin/config/general", icon: "fa-sliders", label: "General"},
			{href: "/api/admin/config/media", icon: "fa-video", label: "Media"},
			{href: "/api/admin/config/paths", icon: "fa-folder", label: "Paths"},
			{href: "/api/admin/config/quality", icon: "fa-star", label: "Quality"},
			{href: "/api/admin/config/regex", icon: "fa-code", label: "Regex"},
			{href: "/api/admin/config/lists", icon: "fa-list", label: "Lists"},
			{href: "/api/admin/config/indexers", icon: "fa-search", label: "Indexers"},
			{href: "/api/admin/config/downloader", icon: "fa-download", label: "Downloader"},
			{href: "/api/admin/config/notifications", icon: "fa-bell", label: "Notifications"},
			{href: "/api/admin/config/scheduler", icon: "fa-clock", label: "Scheduler"},
			{href: "/api/admin/config/imdb", icon: "fa-film", label: "Imdb"},
			{href: "/api/admin/seriesconfig", icon: "fa-tv", label: "Series Config"},
			{href: "/api/admin/users", icon: "fa-users", label: "Users"},
			{href: "/api/admin/traktauth", icon: "fa-key", label: "Trakt Authenticate"},
		}},
		{id: "Helpers", icon: "fa-toolbox", label: "Config Helpers", links: []navLink{
			{href: "/api/admin/quality-reorder", icon: "fa-sort-amount-down", label: "Quality Reorder"},
			{href: "/api/admin/quality-priorities", icon: "fa-arrow-down-wide-short", label: "Quality Priorities"},
			{href: "/api/admin/naming-generator", icon: "fa-code", label: "Naming Generator"},
			{href: "/api/admin/regex-tester", icon: "fa-search-plus", label: "Regex Tester"},
			{href: "/api/admin/crongen", icon: "fa-clock", label: "Cron Generator"},
		}},
		{id: "Testing", icon: "fa-vial", label: "Testing", links: []navLink{
			{href: "/api/admin/testparse", icon: "fa-file-signature", label: "Test Parsing"},
			{href: "/api/admin/matchtester", icon: "fa-flask", label: "Match Tester"},
			{href: "/api/admin/namingtest", icon: "fa-edit", label: "Test Naming"},
			{href: "/api/admin/moviemetadata", icon: "fa-info-circle", label: "Test Metadata"},
			{href: "/api/admin/feedparse", icon: "fa-rss", label: "Feed Parser"},
			{href: "/api/admin/pushovertest", icon: "fa-paper-plane", label: "Notification Test"},
			{href: "/api/admin/api-testing", icon: "fa-cogs", label: "API Testing"},
		}},
		{id: "System", icon: "fa-server", label: "System", links: []navLink{
			{href: "/api/admin/service-health", icon: "fa-heartbeat", label: "Service Health"},
			{href: "/api/admin/storage-health", icon: "fa-hdd", label: "Storage Health"},
			{href: "/api/admin/logviewer", icon: "fa-file-text", label: "Log Viewer"},
			{href: "/api/admin/log-analysis", icon: "fa-chart-line", label: "Log Analysis"},
			{href: "/api/admin/dbmaintenance", icon: "fa-tools", label: "Database Maintenance"},
			{href: "/api/admin/debugstats", icon: "fa-bug", label: "Debug Statistics"},
			{href: "/api/admin/changelog", icon: "fa-code-commit", label: "Changelog"},
			{href: "/swagger/", icon: "fa-book", label: "API Docs", newTab: true},
		}},
	}
)

func (l navLink) node() gomponents.Node {
	var badge gomponents.Node
	if l.badge != nil {
		badge = l.badge()
	}

	return html.Li(html.Class("sidebar-item"),
		html.A(html.Class("sidebar-link"), html.Href(l.href),
			gomponents.If(l.newTab, html.Target("_blank")), gomponents.If(l.newTab, html.Rel("noopener")),
			html.I(html.Class("align-middle fa-solid "+l.icon)),
			html.Span(html.Class("align-middle"), gomponents.Text(l.label)), badge))
}

// navCurrentScript marks the link of the current page and opens its group
// (the longest link the path starts with).
const navCurrentScript = `(function () {
	var path = location.pathname.replace(/\/+$/, ''), best = null, len = -1;
	document.querySelectorAll('#sidebar .sidebar-link[href]').forEach(function (a) {
		var h = a.getAttribute('href').replace(/\/+$/, '');
		// The dashboard only for itself, not for every admin page.
		if ((path === h || h !== '/api/admin' && path.indexOf(h + '/') === 0) && h.length > len) { best = a; len = h.length; }
	});
	if (!best) { return; }
	best.parentElement.classList.add('active');
	var group = best.closest('.sidebar-dropdown');
	if (group) {
		group.classList.add('show');
		var toggle = document.querySelector('#sidebar [data-bs-target="#' + group.id + '"]');
		if (toggle) { toggle.classList.remove('collapsed'); }
	}
})();`

// createNavbar is the admin sidebar; the group of the current page opens by
// itself, the database group on its table pages.
func createNavbar(activeDatabase bool) gomponents.Node {
	group := func(id, icon, label string, open bool, children ...gomponents.Node) gomponents.Node {
		return html.Li(html.Class("sidebar-item"),
			html.A(html.Data("bs-target", "#"+id), html.Data("bs-toggle", "collapse"),
				html.Class("sidebar-link"+map[bool]string{false: " collapsed"}[open]),
				html.I(html.Class("align-middle fa-solid "+icon)),
				html.Span(html.Class("align-middle"), gomponents.Text(label))),
			html.Ul(html.Class("sidebar-dropdown list-unstyled listunstyle collapse"+map[bool]string{true: " show"}[open]),
				html.ID(id), html.Data("bs-parent", "#sidebar"), gomponents.Group(children)))
	}

	items := []gomponents.Node{html.Li(html.Class("sidebar-header"), gomponents.Text(t("Pages")))}
	for _, l := range adminNavPages {
		items = append(items, l.node())
	}

	items = append(items, html.Li(html.Class("sidebar-header"), gomponents.Text("Admin")))

	for _, sec := range adminNavSections {
		links := make([]gomponents.Node, 0, len(sec.links))
		for _, l := range sec.links {
			links = append(links, l.node())
		}

		items = append(items, group(sec.id, sec.icon, sec.label, false, links...))
	}

	items = append(items, group("Database", "fa-database", "Database", activeDatabase,
		html.Li(html.Class("sidebar-item px-2 py-1"), buildDatabaseTableSelect())))

	return html.Nav(
		html.ID("sidebar"),
		html.Class("sidebar js-sidebar"),
		gomponents.Attr("aria-label", "Main navigation"),
		html.Div(
			html.Class("sidebar-content js-simplebar"),
			html.A(
				html.Class("sidebar-brand"),
				html.Href("/api/admin"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Img(
						html.Class("me-3"),
						html.Src("/static/img/icons/icon-48x48.png"),
						html.Alt("Go Media Downloader"),
						html.Style("width:32px;height:32px;border-radius:6px;"),
					),
					html.Div(
						html.Span(
							html.Class("sidebar-brand-text align-middle d-block"),
							gomponents.Text("Go Media"),
						),
						html.Small(
							html.Class("text-white-50 d-block"),
							html.Style("font-size: 0.7rem; margin-top: -2px;"),
							gomponents.Text("Downloader"),
						),
					),
				),
			),
			html.Ul(html.Class("sidebar-nav "), gomponents.Group(items)),
			html.Script(gomponents.Raw(navCurrentScript)),
			html.Div(
				html.Class("simplebar-track simplebar-horizontal"),
				html.Style("visibility: hidden;"),
				html.Div(
					html.Class("simplebar-scrollbar"),
					html.Style("width: 0px; display: none;"),
				),
			),
			html.Div(
				html.Class("simplebar-track simplebar-vertical"),
				html.Style("visibility: visible;"),
				html.Div(
					html.Class("simplebar-scrollbar"),
					html.Style(
						"height: 933px; transform: translate3d(0px, 0px, 0px); display: block;",
					),
				),
			),
		),
	)
}
