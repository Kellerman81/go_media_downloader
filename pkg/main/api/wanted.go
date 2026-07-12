package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// wantedPageSize is how many missing items are shown per page within a tab.
const wantedPageSize = 50

// wantedSection describes one media category shown on the Wanted page. The query
// is assembled from parts so it can be filtered (search) and paginated.
type wantedSection struct {
	Key        string // tab/media key used by the client-side search handler
	Title      string
	Icon       string
	SelectExpr string // label expression + id, scanned positionally
	From       string // FROM ... JOIN ...
	Where      string // base filter (missing = 1)
	SearchExpr string // expression matched with LIKE when filtering
	OrderBy    string // ORDER BY ...
}

// wantedSections defines the queries for each media type's missing items.
var wantedSections = []wantedSection{
	{
		Key:        "movie",
		Title:      "Movies",
		Icon:       "fas fa-film",
		SelectExpr: "dm.title, m.id",
		From:       "FROM movies m JOIN dbmovies dm ON dm.id = m.dbmovie_id",
		Where:      "m.missing = 1",
		SearchExpr: "dm.title",
		OrderBy:    "ORDER BY dm.title",
	},
	{
		Key:        "episode",
		Title:      "Episodes",
		Icon:       "fas fa-tv",
		SelectExpr: "ds.seriename || ' - ' || dse.identifier, se.id",
		From: "FROM serie_episodes se " +
			"JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id " +
			"JOIN series s ON s.id = se.serie_id " +
			"JOIN dbseries ds ON ds.id = s.dbserie_id",
		Where:      "se.missing = 1",
		SearchExpr: "(ds.seriename || ' ' || dse.identifier)",
		OrderBy:    "ORDER BY ds.seriename, dse.identifier",
	},
	{
		// dbalbums has no artist column; the (primary) artist comes from the
		// dbalbum_artists -> dbartists join, fetched per-row as a subquery.
		Key:   "album",
		Title: "Albums",
		Icon:  "fas fa-compact-disc",
		SelectExpr: "COALESCE((SELECT ar.name FROM dbalbum_artists aa " +
			"JOIN dbartists ar ON ar.id = aa.dbartist_id " +
			"WHERE aa.dbalbum_id = da.id LIMIT 1) || ' - ', '') || da.title, a.id",
		From:       "FROM albums a JOIN dbalbums da ON da.id = a.dbalbum_id",
		Where:      "a.missing = 1",
		SearchExpr: "da.title",
		// Push records with a blank title to the end so page 1 shows real titles.
		OrderBy: "ORDER BY CASE WHEN da.title IS NULL OR da.title = '' THEN 1 ELSE 0 END, da.title",
	},
	{
		// dbbooks stores the author via dbauthor_id -> dbauthors, not a column.
		Key:   "book",
		Title: "Books",
		Icon:  "fas fa-book",
		SelectExpr: "COALESCE((SELECT au.name FROM dbauthors au " +
			"WHERE au.id = db.dbauthor_id) || ' - ', '') || db.title, b.id",
		From:       "FROM books b JOIN dbbooks db ON db.id = b.dbbook_id",
		Where:      "b.missing = 1",
		SearchExpr: "db.title",
		OrderBy:    "ORDER BY CASE WHEN db.title IS NULL OR db.title = '' THEN 1 ELSE 0 END, db.title",
	},
	{
		// Author comes from dbaudiobook_authors -> dbauthors. The title can be
		// blank on shell records, so fall back to the linked print book's title.
		Key:   "audiobook",
		Title: "Audiobooks",
		Icon:  "fas fa-headphones",
		SelectExpr: "COALESCE((SELECT au.name FROM dbaudiobook_authors aba " +
			"JOIN dbauthors au ON au.id = aba.dbauthor_id " +
			"WHERE aba.dbaudiobook_id = da.id LIMIT 1) || ' - ', '') || " +
			"COALESCE(NULLIF(da.title, ''), " +
			"(SELECT bk.title FROM dbbooks bk WHERE bk.id = da.dbbook_id), ''), ab.id",
		From:       "FROM audiobooks ab JOIN dbaudiobooks da ON da.id = ab.dbaudiobook_id",
		Where:      "ab.missing = 1",
		SearchExpr: "da.title",
		OrderBy:    "ORDER BY CASE WHEN da.title IS NULL OR da.title = '' THEN 1 ELSE 0 END, da.title",
	},
}

// findWantedSection looks up a section by its key.
func findWantedSection(key string) (wantedSection, bool) {
	for _, s := range wantedSections {
		if s.Key == key {
			return s, true
		}
	}

	return wantedSection{}, false
}

// countQuery / rowsQuery assemble the COUNT and paginated SELECT for a section,
// optionally filtered by a LIKE search. Args are returned alongside.
func (s wantedSection) countQuery(search string) (string, []any) {
	where := s.Where

	var args []any
	if search != "" {
		where += " AND " + s.SearchExpr + " LIKE ?"

		args = append(args, "%"+search+"%")
	}

	return "SELECT COUNT(*) " + s.From + " WHERE " + where, args
}

func (s wantedSection) rowsQuery(search string, page int) (string, []any) {
	where := s.Where

	var args []any
	if search != "" {
		where += " AND " + s.SearchExpr + " LIKE ?"

		args = append(args, "%"+search+"%")
	}

	offset := (page - 1) * wantedPageSize

	args = append(args, wantedPageSize, offset)

	return "SELECT " + s.SelectExpr + " " + s.From + " WHERE " + where +
		" " + s.OrderBy + " LIMIT ? OFFSET ?", args
}

// renderWantedPage serves the full Wanted page.
func renderWantedPage(ctx *gin.Context) {
	pageNode := page("Wanted", false, false, true, renderWantedContent())

	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderWantedPartial serves just the tabbed content for HTMX refresh.
func renderWantedPartial(ctx *gin.Context) {
	var buf strings.Builder
	renderWantedContent().Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderWantedContent builds the tabbed shell. Each tab has a filter box and a
// results region that lazy-loads (and paginates) via HTMX.
func renderWantedContent() gomponents.Node {
	tabs := make([]gomponents.Node, 0, len(wantedSections))
	panes := make([]gomponents.Node, 0, len(wantedSections))

	for i, s := range wantedSections {
		active := i == 0
		tabID := "wanted-" + s.Key

		// Total (unfiltered) count for the tab badge.
		cq, _ := s.countQuery("")
		total := int(database.Getdatarow[uint](false, cq))

		linkClass := "nav-link"
		if active {
			linkClass = "nav-link active"
		}

		tabs = append(tabs, html.Li(
			html.Class("nav-item"),
			html.Role("presentation"),
			html.Button(
				html.Class(linkClass),
				html.ID(tabID+"-tab"),
				html.Type("button"),
				html.Role("tab"),
				html.Data("bs-toggle", "tab"),
				html.Data("bs-target", "#"+tabID),
				gomponents.Attr("aria-controls", tabID),
				html.I(html.Class(s.Icon+" me-1"), gomponents.Attr("aria-hidden", "true")),
				gomponents.Text(s.Title+" "),
				html.Span(html.Class("badge bg-secondary ms-1"), gomponents.Textf("%d", total)),
			),
		))

		paneClass := "tab-pane fade"
		if active {
			paneClass = "tab-pane fade show active"
		}

		resID := "wres-" + s.Key

		panes = append(panes, html.Div(
			html.Class(paneClass),
			html.ID(tabID),
			html.Role("tabpanel"),
			gomponents.Attr("aria-labelledby", tabID+"-tab"),
			html.Div(
				html.Class("pt-3"),

				// Filter box — typing reloads page 1 of this tab's results.
				html.Div(
					html.Class("input-group input-group-sm mb-2"),
					html.Span(
						html.Class("input-group-text"),
						html.I(
							html.Class("fas fa-magnifying-glass"),
							gomponents.Attr("aria-hidden", "true"),
						),
					),
					html.Input(
						html.Type("search"),
						html.Class("form-control"),
						html.Name("q"),
						html.Placeholder("Filter "+strings.ToLower(s.Title)+"..."),
						gomponents.Attr("aria-label", "Filter "+s.Title),
						hx.Get("/api/admin/wanted/tab?media="+s.Key),
						hx.Trigger("keyup changed delay:350ms, search"),
						hx.Target("#"+resID),
						hx.Swap("innerHTML"),
						hx.Indicator("#"+resID+"-ind"),
					),
					html.Span(
						html.ID(resID+"-ind"),
						html.Class("input-group-text htmx-indicator"),
						html.Div(html.Class("spinner-border spinner-border-sm")),
					),
				),

				// Results region — lazy-loads the first page.
				html.Div(
					html.ID(resID),
					hx.Get("/api/admin/wanted/tab?media="+s.Key+"&page=1"),
					hx.Trigger("load"),
					hx.Swap("innerHTML"),
					html.Div(
						html.Class("text-center text-muted p-4"),
						html.Div(html.Class("spinner-border")),
					),
				),
			),
		))
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Page header.
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(
						html.Class("fas fa-bullseye header-icon"),
						gomponents.Attr("aria-hidden", "true"),
					),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Wanted")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Missing items across all media types. Filter, page through and search individually.",
						),
					),
				),
				html.Div(
					html.Class("ms-auto"),
					html.Button(
						html.Type("button"),
						html.Class("btn btn-outline-secondary btn-sm"),
						gomponents.Attr("aria-label", "Refresh wanted list"),
						hx.Get("/api/admin/wanted/partial"),
						hx.Target("#wanted-content"),
						hx.Swap("outerHTML"),
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
			html.ID("wanted-content"),
			html.Ul(
				append(
					[]gomponents.Node{html.Class("nav nav-tabs"), html.Role("tablist")},
					tabs...)...,
			),
			html.Div(
				append([]gomponents.Node{html.Class("tab-content")}, panes...)...,
			),
		),

		wantedScript(),
	)
}

// renderWantedTab serves one tab's filtered, paginated results fragment.
func renderWantedTab(ctx *gin.Context) {
	media := ctx.Query("media")

	sec, ok := findWantedSection(media)
	if !ok {
		ctx.String(
			http.StatusBadRequest,
			`<div class="alert alert-danger mb-0">Unknown media type.</div>`,
		)

		return
	}

	search := strings.TrimSpace(ctx.Query("q"))

	page := 1
	if p, err := strconv.Atoi(ctx.Query("page")); err == nil && p > 0 {
		page = p
	}

	cq, cargs := sec.countQuery(search)
	total := int(database.Getdatarow[uint](false, cq, cargs...))

	rq, rargs := sec.rowsQuery(search, page)
	rows := database.GetrowsN[database.DbstaticOneStringOneUInt](
		false,
		uint(wantedPageSize),
		rq,
		rargs...)

	var buf strings.Builder
	wantedResults(sec, search, page, total, rows).Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// wantedResults renders the count summary, table and pagination for a tab page.
func wantedResults(
	sec wantedSection,
	search string,
	page, total int,
	rows []database.DbstaticOneStringOneUInt,
) gomponents.Node {
	if total == 0 {
		msg := "All " + strings.ToLower(sec.Title) + " are accounted for."
		icon := "fas fa-check-circle"
		color := "#28a745"

		if search != "" {
			msg = "No " + strings.ToLower(sec.Title) + " match \"" + search + "\"."
			icon = "fas fa-magnifying-glass"
			color = "#6c757d"
		}

		return html.Div(
			html.Class("text-center text-muted p-5"),
			html.I(html.Class(icon+" mb-3"), html.Style("font-size: 3rem; color: "+color+";")),
			html.H5(html.Class("text-muted"), gomponents.Text("Nothing to show")),
			html.P(html.Class("text-muted mb-0"), gomponents.Text(msg)),
		)
	}

	totalPages := (total + wantedPageSize - 1) / wantedPageSize
	if page > totalPages {
		page = totalPages
	}

	start := (page-1)*wantedPageSize + 1
	end := start + len(rows) - 1

	trs := make([]gomponents.Node, 0, len(rows))
	for _, r := range rows {
		idStr := strconv.FormatUint(uint64(r.Num), 10)

		trs = append(trs, html.Tr(
			html.Td(gomponents.Text(r.Str)),
			html.Td(
				html.Class("text-end"),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-sm btn-outline-primary wanted-search-btn"),
					html.Data("media", sec.Key),
					html.Data("id", idStr),
					gomponents.Attr("aria-label", "Search for "+r.Str),
					html.I(
						html.Class("fas fa-magnifying-glass me-1"),
						gomponents.Attr("aria-hidden", "true"),
					),
					gomponents.Text("Search"),
				),
			),
		))
	}

	return html.Div(
		html.Div(
			html.Class("d-flex justify-content-between align-items-center mb-2 small text-muted"),
			gomponents.Textf("Showing %d–%d of %d", start, end, total),
			gomponents.Textf("Page %d of %d", page, totalPages),
		),
		html.Div(
			html.Class("table-responsive"),
			html.Table(
				html.Class("table table-sm table-hover align-middle"),
				html.THead(
					html.Tr(
						html.Th(gomponents.Attr("scope", "col"), gomponents.Text(sec.Title)),
						html.Th(
							gomponents.Attr("scope", "col"),
							html.Class("text-end"),
							html.Style("width: 8rem;"),
							gomponents.Text("Action"),
						),
					),
				),
				html.TBody(trs...),
			),
		),
		wantedPagination(sec.Key, search, page, totalPages),
	)
}

// wantedPagination renders Prev/Next paging controls that swap the results region.
func wantedPagination(key, search string, page, totalPages int) gomponents.Node {
	if totalPages <= 1 {
		return gomponents.Text("")
	}

	base := "/api/admin/wanted/tab?media=" + key + "&q=" + url.QueryEscape(search) + "&page="
	resTarget := "#wres-" + key

	pageBtn := func(label string, target int, disabled bool) gomponents.Node {
		liClass := "page-item"
		if disabled {
			liClass = "page-item disabled"
		}

		btn := []gomponents.Node{
			html.Class("page-link"),
			html.Type("button"),
			gomponents.Text(label),
		}
		if !disabled {
			btn = append(btn,
				hx.Get(base+strconv.Itoa(target)),
				hx.Target(resTarget),
				hx.Swap("innerHTML"),
			)
		}

		return html.Li(html.Class(liClass), html.Button(btn...))
	}

	return html.Nav(
		gomponents.Attr("aria-label", "Wanted pagination"),
		html.Ul(
			html.Class("pagination pagination-sm mb-0 justify-content-center"),
			pageBtn("« First", 1, page <= 1),
			pageBtn("‹ Prev", page-1, page <= 1),
			html.Li(html.Class("page-item disabled"),
				html.Span(html.Class("page-link"), gomponents.Textf("%d / %d", page, totalPages))),
			pageBtn("Next ›", page+1, page >= totalPages),
			pageBtn("Last »", totalPages, page >= totalPages),
		),
	)
}

// wantedScript wires the per-row search buttons to the existing search endpoints.
func wantedScript() gomponents.Node {
	apikey := config.GetSettingsGeneral().WebAPIKey

	return html.Script(gomponents.Raw(`
		(function() {
			var apikey = '` + apikey + `';
			function endpointFor(media, id) {
				switch (media) {
					case 'movie':     return '/api/movies/search/list/' + id + '?apikey=' + encodeURIComponent(apikey) + '&searchByTitle=false&download=true';
					case 'episode':   return '/api/series/episodes/search/list/' + id + '?apikey=' + encodeURIComponent(apikey) + '&download=true';
					case 'album':     return '/api/music/search/list/' + id + '?apikey=' + encodeURIComponent(apikey);
					case 'book':      return '/api/books/search/list/' + id + '?apikey=' + encodeURIComponent(apikey);
					case 'audiobook': return '/api/audiobooks/search/list/' + id + '?apikey=' + encodeURIComponent(apikey);
				}
				return null;
			}
			document.addEventListener('click', function(e) {
				var btn = e.target.closest ? e.target.closest('.wanted-search-btn') : null;
				if (!btn) return;
				var url = endpointFor(btn.getAttribute('data-media'), btn.getAttribute('data-id'));
				if (!url) return;
				var original = btn.innerHTML;
				btn.disabled = true;
				btn.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Searching...';
				fetch(url, { method: 'GET', headers: { 'X-CSRF-Token': document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || '' } })
					.then(function(r){ return r.json().catch(function(){ return {}; }); })
					.then(function(data){
						var accepted = (data && data.accepted) ? data.accepted.length : 0;
						var denied = (data && data.denied) ? data.denied.length : 0;
						showToaster(accepted > 0 ? 'success' : 'info', 'Search done — accepted: ' + accepted + ', denied: ' + denied);
						btn.innerHTML = '<i class="fas fa-check me-1"></i>Searched';
						btn.classList.remove('btn-outline-primary');
						btn.classList.add('btn-outline-success');
					})
					.catch(function(err){
						showToaster('error', 'Search failed: ' + err.message);
						btn.innerHTML = original;
						btn.disabled = false;
					});
			});
		})();
	`))
}
