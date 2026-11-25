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

// statSectionInfo is a section of the statistics page and its entry in the
// jump menu.
type statSectionInfo struct {
	id, label, icon string
}

// statCollapsedKey is the localStorage key prefix that remembers a collapsed
// section, per browser.
const statCollapsedKey = "statCollapsed-"

// statJumpMenu links to the sections; it stays at the top while scrolling.
// A link opens its section when it is collapsed; the last button collapses or
// expands all sections.
func statJumpMenu(sections []statSectionInfo) gomponents.Node {
	links := make([]gomponents.Node, 0, len(sections)+1)
	for _, sec := range sections {
		links = append(links, html.A(html.Class("btn btn-sm btn-outline-secondary"), html.Href("#stat-"+sec.id),
			gomponents.Attr("onclick", "var d=document.getElementById('stat-"+sec.id+"');if(d)d.open=true"),
			html.I(html.Class("fa-solid "+sec.icon+" me-1")), gomponents.Text(sec.label)))
	}
	links = append(links, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary ms-auto"),
		gomponents.Attr("onclick", "var s=document.querySelectorAll('details.stat-part'),o=!Array.prototype.some.call(s,function(d){return d.open});s.forEach(function(d){d.open=o})"),
		gomponents.Attr("aria-label", "Collapse or expand all sections"), html.TitleAttr("Collapse or expand all sections"),
		html.I(html.Class("fa-solid fa-up-down"), gomponents.Attr("aria-hidden", "true"))))

	// The template's .main hides its overflow, which turns sticky off; clip
	// cuts the same way without that.
	return gomponents.Group([]gomponents.Node{
		html.StyleEl(gomponents.Raw(".main:has(#statJump){overflow:clip}" +
			"details.stat-part>summary{list-style:none;cursor:pointer}" +
			"details.stat-part>summary::-webkit-details-marker{display:none}" +
			"details.stat-part .stat-chevron{transition:transform .15s}" +
			"details.stat-part:not([open]) .stat-chevron{transform:rotate(-90deg)}")),
		html.Nav(html.ID("statJump"), html.Class("d-flex flex-wrap gap-2 py-2 mb-3 bg-body border-bottom"), gomponents.Attr("aria-label", "Sections"),
			html.Style("position:sticky;top:0;z-index:1020"), gomponents.Group(links)),
	})
}

// statPart is a collapsible section of the statistics page under its heading.
// Its content still loads while collapsed; the browser remembers the state.
func statPart(sec statSectionInfo, children ...gomponents.Node) gomponents.Node {
	return html.Details(html.ID("stat-"+sec.id), html.Class("stat-part mb-3"), html.Style("scroll-margin-top:4rem"),
		gomponents.Attr("open", ""),
		gomponents.Attr("ontoggle", "try{localStorage.setItem('"+statCollapsedKey+"'+this.id,this.open?'0':'1')}catch(e){}"),
		html.Summary(html.Class("mb-3"),
			html.H5(html.Class("d-inline text-muted mb-0"),
				html.I(html.Class("fa-solid fa-chevron-down stat-chevron me-2 small"), gomponents.Attr("aria-hidden", "true")),
				html.I(html.Class("fa-solid "+sec.icon+" me-2")), gomponents.Text(sec.label))),
		gomponents.Group(children))
}

// statRestoreCollapsed closes the sections this browser collapsed last time.
func statRestoreCollapsed() gomponents.Node {
	return html.Script(gomponents.Raw("(function(){try{document.querySelectorAll('details.stat-part').forEach(function(d){" +
		"if(localStorage.getItem('" + statCollapsedKey + "'+d.id)==='1')d.open=false})}catch(e){}})();"))
}

// @Summary      [Web only] Statistics page (HTML)
// @Description  Returns the statistics dashboard of the web interface as HTML. Web interface only: needs a login session cookie from /api/login and does not accept the apikey, so it cannot be called from Swagger or with ?apikey=.
// @Tags         web-only
// @Produce      html
// @Success      200 {string} string "Statistics dashboard HTML"
// @Failure      500 {string} string "Internal server error"
// @Router       /api/admin/statistics [get].
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

	sections := []statSectionInfo{
		{"overview", "Overview", "fa-gauge"},
		{"libraries", "Libraries", "fa-photo-film"},
		{"downloads", "Downloads & Wanted", "fa-download"},
		{"quality", "Quality", "fa-star"},
		{"playback", "Playback", "fa-play"},
		{"storage", "Storage", "fa-hdd"},
		{"jobs", "Jobs & Workers", "fa-gears"},
		{"providers", "Providers", "fa-plug"},
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

		statJumpMenu(sections),
		html.Div(
			html.Class("container-fluid"),
			statPart(sections[0], html.Div(append([]gomponents.Node{html.Class("row")}, summaryCards...)...)),
			statPart(sections[1], html.Div(append([]gomponents.Node{html.Class("row")}, detailCards...)...)),
			statPart(sections[2],
				statSectionCard("text-primary", "/api/admin/statistics/activity", "Loading download activity..."),
				statSectionCard("text-warning", "/api/admin/statistics/backlog", "Loading wanted backlog...")),
			statPart(sections[3],
				statSectionCard("text-success", "/api/admin/statistics/quality", "Loading library quality...")),
			statPart(sections[4],
				statSectionCard("text-success", "/api/admin/statistics/plays", "Loading play history...")),
			// Storage: shows the last scan; a scan only runs on demand.
			statPart(sections[5], html.Div(html.Class("row"), html.Div(html.Class("col-12 mb-4"),
				html.Div(html.Class("card"), html.ID("statStorage"),
					gomponents.Attr("hx-get", storageStatsURL+"?type=detail"), gomponents.Attr("hx-trigger", "load, storageScan from:body"),
					gomponents.Attr("hx-swap", "innerHTML"),
					html.Div(html.Class("card-body text-center"), html.Div(html.Class("spinner-border text-info"))))))),
			statPart(sections[6],
				statSectionCard("text-info", "/api/admin/statistics/jobs", "Loading job runtimes..."),
				statSectionCard("text-dark", "/api/admin/statistics/workers", "Loading worker statistics...")),
			statPart(sections[7],
				statSectionCard("text-secondary", "/api/admin/statistics/http", "Loading HTTP client statistics..."),
				statSectionCard("text-primary", "/api/admin/statistics/metadata", "Loading metadata provider statistics..."),
				statSectionCard("text-info", "/api/admin/statistics/notification", "Loading notification provider statistics..."),
				statSectionCard("text-success", "/api/admin/statistics/downloader", "Loading downloader provider statistics...")),
		),
		statRestoreCollapsed(),
	))
}
