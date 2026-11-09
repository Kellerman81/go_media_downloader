package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// wantedPageSize is how many missing items are shown per page within a tab.
const wantedPageSize = 50

// wantedSection describes one media category shown on the Wanted page. The query
// is assembled from parts so it can be filtered and paginated. A title wanted
// by several media groups (EN and DE) or lists has one row per list entry.
type wantedSection struct {
	Key        string // tab/media key used by the client-side search handler
	Kind       string // media config kind for mediaEntriesForKind
	Title      string
	Icon       string
	LabelExpr  string // the row's display label
	IDExpr     string // the list entry id the search endpoints take
	ListExpr   string // the entry's list name
	From       string // FROM ... JOIN ...
	Where      string // base filter (missing = 1)
	SearchExpr string // expression matched with LIKE when filtering
	OrderBy    string // ORDER BY ...
}

// wantedSections defines the queries for each media type's missing items.
var wantedSections = []wantedSection{
	{
		Key:        "movie",
		Kind:       "movie",
		Title:      "Movies",
		Icon:       "fas fa-film",
		LabelExpr:  "dm.title || CASE WHEN dm.year > 0 THEN ' (' || dm.year || ')' ELSE '' END",
		IDExpr:     "m.id",
		ListExpr:   "m.listname",
		From:       "FROM movies m JOIN dbmovies dm ON dm.id = m.dbmovie_id",
		Where:      "m.missing = 1",
		SearchExpr: "dm.title",
		OrderBy:    "ORDER BY dm.title, m.listname",
	},
	{
		Key:       "episode",
		Kind:      "serie",
		Title:     "Episodes",
		Icon:      "fas fa-tv",
		LabelExpr: "ds.seriename || ' - ' || dse.identifier",
		IDExpr:    "se.id",
		ListExpr:  "s.listname",
		From: "FROM serie_episodes se " +
			"JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id " +
			"JOIN series s ON s.id = se.serie_id " +
			"JOIN dbseries ds ON ds.id = s.dbserie_id",
		Where:      "se.missing = 1",
		SearchExpr: "(ds.seriename || ' ' || dse.identifier)",
		OrderBy:    "ORDER BY ds.seriename, dse.identifier, s.listname",
	},
	{
		// dbalbums has no artist column; the (primary) artist comes from the
		// dbalbum_artists -> dbartists join, fetched per-row as a subquery.
		Key:   "album",
		Kind:  "music",
		Title: "Albums",
		Icon:  "fas fa-compact-disc",
		LabelExpr: "COALESCE((SELECT ar.name FROM dbalbum_artists aa " +
			"JOIN dbartists ar ON ar.id = aa.dbartist_id " +
			"WHERE aa.dbalbum_id = da.id LIMIT 1) || ' - ', '') || da.title",
		IDExpr:     "a.id",
		ListExpr:   "a.listname",
		From:       "FROM albums a JOIN dbalbums da ON da.id = a.dbalbum_id",
		Where:      "a.missing = 1",
		SearchExpr: "da.title",
		// Push records with a blank title to the end so page 1 shows real titles.
		OrderBy: "ORDER BY CASE WHEN da.title IS NULL OR da.title = '' THEN 1 ELSE 0 END, da.title, a.listname",
	},
	{
		// dbbooks stores the author via dbauthor_id -> dbauthors, not a column.
		Key:   "book",
		Kind:  "book",
		Title: "Books",
		Icon:  "fas fa-book",
		LabelExpr: "COALESCE((SELECT au.name FROM dbauthors au " +
			"WHERE au.id = db.dbauthor_id) || ' - ', '') || db.title",
		IDExpr:     "b.id",
		ListExpr:   "b.listname",
		From:       "FROM books b JOIN dbbooks db ON db.id = b.dbbook_id",
		Where:      "b.missing = 1",
		SearchExpr: "db.title",
		OrderBy:    "ORDER BY CASE WHEN db.title IS NULL OR db.title = '' THEN 1 ELSE 0 END, db.title, b.listname",
	},
	{
		// Author comes from dbaudiobook_authors -> dbauthors. The title can be
		// blank on shell records, so fall back to the linked print book's title.
		Key:   "audiobook",
		Kind:  "audiobook",
		Title: "Audiobooks",
		Icon:  "fas fa-headphones",
		LabelExpr: "COALESCE((SELECT au.name FROM dbaudiobook_authors aba " +
			"JOIN dbauthors au ON au.id = aba.dbauthor_id " +
			"WHERE aba.dbaudiobook_id = da.id LIMIT 1) || ' - ', '') || " +
			"COALESCE(NULLIF(da.title, ''), " +
			"(SELECT bk.title FROM dbbooks bk WHERE bk.id = da.dbbook_id), '')",
		IDExpr:     "ab.id",
		ListExpr:   "ab.listname",
		From:       "FROM audiobooks ab JOIN dbaudiobooks da ON da.id = ab.dbaudiobook_id",
		Where:      "ab.missing = 1",
		SearchExpr: "da.title",
		OrderBy:    "ORDER BY CASE WHEN da.title IS NULL OR da.title = '' THEN 1 ELSE 0 END, da.title, ab.listname",
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

// wantedList is one configured list of a media group.
type wantedList struct {
	Group   string // the media group's name, e.g. "EN"
	Name    string
	Enabled bool
}

// lists returns the section's configured lists, in config order.
func (s wantedSection) lists() []wantedList {
	var out []wantedList

	for _, m := range mediaEntriesForKind(s.Kind) {
		for _, l := range m.Lists {
			if l.Name != "" {
				out = append(out, wantedList{Group: m.Name, Name: l.Name, Enabled: l.Enabled})
			}
		}
	}

	return out
}

// listInfo finds the configured list named name (list names are matched
// case-insensitively, like the listname columns).
func listInfo(lists []wantedList, name string) (wantedList, bool) {
	for _, l := range lists {
		if strings.EqualFold(l.Name, name) {
			return l, true
		}
	}

	return wantedList{}, false
}

// wantedFilter is what a tab is narrowed to.
type wantedFilter struct {
	Search string
	Group  string // a media group name, "" for all
	List   string // a list name, "" for all
}

func wantedFilterFrom(ctx *gin.Context) wantedFilter {
	return wantedFilter{
		Search: strings.TrimSpace(ctx.Query("q")),
		Group:  ctx.Query("group"),
		List:   ctx.Query("list"),
	}
}

func (f wantedFilter) query() string {
	return "&q=" + url.QueryEscape(f.Search) + "&group=" + url.QueryEscape(f.Group) + "&list=" + url.QueryEscape(f.List)
}

// where returns the section's WHERE clause for f and its args. A group
// limits the rows to that group's lists.
func (s wantedSection) where(f wantedFilter) (string, []any) {
	where := s.Where

	var args []any

	if f.Search != "" {
		where += " AND " + s.SearchExpr + " LIKE ? ESCAPE '\\'"

		args = append(args, "%"+escapeLikePattern(f.Search)+"%")
	}

	if f.List != "" {
		where += " AND " + s.ListExpr + " = ? COLLATE NOCASE"

		args = append(args, f.List)
	}

	if f.Group != "" {
		var names []string

		for _, l := range s.lists() {
			if l.Group == f.Group {
				names = append(names, l.Name)
			}
		}

		if len(names) == 0 {
			return where + " AND 0", args
		}

		where += " AND " + s.ListExpr + " COLLATE NOCASE IN (?" + strings.Repeat(", ?", len(names)-1) + ")"

		for _, n := range names {
			args = append(args, n)
		}
	}

	return where, args
}

// countQuery / rowsQuery assemble the COUNT and paginated SELECT for a section.
func (s wantedSection) countQuery(f wantedFilter) (string, []any) {
	where, args := s.where(f)
	return "SELECT COUNT(*) " + s.From + " WHERE " + where, args
}

// escapeLikePattern escapes LIKE wildcards so a search for "100%" or "a_b"
// matches literally (used with ESCAPE '\').
func escapeLikePattern(v string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(v)
}

func (s wantedSection) rowsQuery(f wantedFilter, page int) (string, []any) {
	where, args := s.where(f)
	args = append(args, wantedPageSize, (page-1)*wantedPageSize)

	return "SELECT " + s.LabelExpr + " AS label, " + s.IDExpr + " AS id, " + s.ListExpr + " AS listname " +
		s.From + " WHERE " + where + " " + s.OrderBy + " LIMIT ? OFFSET ?", args
}

// wantedRow is one missing list entry.
type wantedRow struct {
	Label    string `db:"label"`
	ID       uint   `db:"id"`
	Listname string `db:"listname"`
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

// wantedFilterForm renders a tab's filters: text, media group and list. Any
// change reloads page 1 of the tab's results with all three.
func wantedFilterForm(s wantedSection, resID string) gomponents.Node {
	lists := s.lists()

	var groups []string

	for _, l := range lists {
		if len(groups) == 0 || groups[len(groups)-1] != l.Group {
			groups = append(groups, l.Group)
		}
	}

	groupOpts := []gomponents.Node{html.Option(html.Value(""), gomponents.Text("All media groups"))}
	for _, g := range groups {
		groupOpts = append(groupOpts, html.Option(html.Value(g), gomponents.Text(g)))
	}

	listOpts := []gomponents.Node{html.Option(html.Value(""), gomponents.Text("All lists"))}
	for _, g := range groups {
		opts := []gomponents.Node{gomponents.Attr("label", g)}

		for _, l := range lists {
			if l.Group != g {
				continue
			}

			label := l.Name
			if !l.Enabled {
				label += " (disabled)"
			}

			opts = append(opts, html.Option(html.Value(l.Name), gomponents.Text(label)))
		}

		listOpts = append(listOpts, html.OptGroup(opts...))
	}

	// Each control reloads the results with the whole form's values.
	reload := func(trigger string) gomponents.Node {
		return gomponents.Group([]gomponents.Node{
			hx.Get("/api/admin/wanted/tab?media=" + s.Key),
			hx.Trigger(trigger),
			hx.Include("closest form"),
			hx.Target("#" + resID),
			hx.Swap("innerHTML"),
			hx.Indicator("#" + resID + "-ind"),
		})
	}

	return html.Form(
		html.Class("row g-2 mb-2"),
		gomponents.Attr("onsubmit", "return false;"),
		html.Div(
			html.Class("col-md-6"),
			html.Div(
				html.Class("input-group input-group-sm"),
				html.Span(
					html.Class("input-group-text"),
					html.I(html.Class("fas fa-magnifying-glass"), gomponents.Attr("aria-hidden", "true")),
				),
				html.Input(
					html.Type("search"),
					html.Class("form-control"),
					html.Name("q"),
					html.Placeholder("Filter "+strings.ToLower(s.Title)+"..."),
					gomponents.Attr("aria-label", "Filter "+s.Title),
					reload("keyup changed delay:350ms, search"),
				),
				html.Span(
					html.ID(resID+"-ind"),
					html.Class("input-group-text htmx-indicator"),
					html.Div(html.Class("spinner-border spinner-border-sm")),
				),
			),
		),
		html.Div(
			html.Class("col-md-3"),
			html.Select(
				html.Class("form-select form-select-sm"),
				html.Name("group"),
				gomponents.Attr("aria-label", "Media group"),
				reload("change"),
				gomponents.Group(groupOpts),
			),
		),
		html.Div(
			html.Class("col-md-3"),
			html.Select(
				html.Class("form-select form-select-sm"),
				html.Name("list"),
				gomponents.Attr("aria-label", "List"),
				reload("change"),
				gomponents.Group(listOpts),
			),
		),
	)
}

// renderWantedContent builds the tabbed shell. Each tab has filters and a
// results region that lazy-loads (and paginates) via HTMX.
func renderWantedContent() gomponents.Node {
	tabs := make([]gomponents.Node, 0, len(wantedSections))
	panes := make([]gomponents.Node, 0, len(wantedSections))

	for i, s := range wantedSections {
		active := i == 0
		tabID := "wanted-" + s.Key

		// Total (unfiltered) count for the tab badge.
		cq, _ := s.countQuery(wantedFilter{})
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
				wantedFilterForm(s, resID),

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
							"Missing list entries across all media types. A title wanted by several media groups "+
								"or lists has a row per list; filter by group or list and search each entry.",
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

	f := wantedFilterFrom(ctx)

	page := 1
	if p, err := strconv.Atoi(ctx.Query("page")); err == nil && p > 0 {
		page = p
	}

	cq, cargs := sec.countQuery(f)
	total := int(database.Getdatarow[uint](false, cq, cargs...))

	// Clamp before querying so an out-of-range page shows the last page's
	// rows rather than an empty table under a "page N of M" header.
	if totalPages := (total + wantedPageSize - 1) / wantedPageSize; totalPages > 0 && page > totalPages {
		page = totalPages
	}

	rq, rargs := sec.rowsQuery(f, page)
	rows := database.StructscanT[wantedRow](false, uint(wantedPageSize), rq, rargs...)

	var buf strings.Builder
	wantedResults(sec, f, page, total, rows).Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// wantedListCells renders a row's media group and list, flagging lists that
// are disabled or in no media group of this type.
func wantedListCells(lists []wantedList, name string) []gomponents.Node {
	l, ok := listInfo(lists, name)
	if !ok {
		return []gomponents.Node{
			html.Td(html.Class("text-muted"), gomponents.Text("—")),
			html.Td(
				gomponents.Text(name+" "),
				html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text("not configured")),
			),
		}
	}

	list := []gomponents.Node{gomponents.Text(l.Name)}
	if !l.Enabled {
		list = append(list, gomponents.Text(" "), html.Span(html.Class("badge bg-secondary"), gomponents.Text("disabled")))
	}

	return []gomponents.Node{
		html.Td(html.Span(html.Class("badge bg-light text-dark border"), gomponents.Text(l.Group))),
		html.Td(list...),
	}
}

// wantedResults renders the count summary, table and pagination for a tab page.
func wantedResults(
	sec wantedSection,
	f wantedFilter,
	page, total int,
	rows []wantedRow,
) gomponents.Node {
	if total == 0 {
		msg := "All " + strings.ToLower(sec.Title) + " are accounted for."
		icon := "fas fa-check-circle"
		color := "#28a745"

		if f != (wantedFilter{}) {
			msg = "No missing " + strings.ToLower(sec.Title) + " match the filter."
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
	lists := sec.lists()

	trs := make([]gomponents.Node, 0, len(rows))
	for _, r := range rows {
		idStr := strconv.FormatUint(uint64(r.ID), 10)

		cells := []gomponents.Node{html.Td(gomponents.Text(r.Label))}
		cells = append(cells, wantedListCells(lists, r.Listname)...)
		cells = append(cells, html.Td(
			html.Class("text-end"),
			html.Button(
				html.Type("button"),
				html.Class("btn btn-sm btn-outline-primary wanted-search-btn"),
				html.Data("media", sec.Key),
				html.Data("id", idStr),
				gomponents.Attr("aria-label", "Search for "+r.Label+" in "+r.Listname),
				html.I(
					html.Class("fas fa-magnifying-glass me-1"),
					gomponents.Attr("aria-hidden", "true"),
				),
				gomponents.Text("Search"),
			),
		))

		trs = append(trs, html.Tr(cells...))
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
						html.Th(gomponents.Attr("scope", "col"), html.Style("width: 9rem;"), gomponents.Text("Media Group")),
						html.Th(gomponents.Attr("scope", "col"), html.Style("width: 16rem;"), gomponents.Text("List")),
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
		wantedPagination(sec.Key, f, page, totalPages),
	)
}

// wantedPagination renders paging controls that swap the results region and
// keep the tab's filters.
func wantedPagination(key string, f wantedFilter, page, totalPages int) gomponents.Node {
	if totalPages <= 1 {
		return gomponents.Text("")
	}

	base := "/api/admin/wanted/tab?media=" + key + f.query() + "&page="
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
// Requests go through the session-authenticated admin proxy (admin_proxy.go,
// window.gmdApiProxyFetch) rather than embedding the real WebAPIKey in this
// page - see admin_proxy.go's package doc for why.
func wantedScript() gomponents.Node {
	return html.Script(gomponents.Raw(`
		(function() {
			function endpointFor(media, id) {
				switch (media) {
					case 'movie':     return '/api/movies/search/list/' + id + '?searchByTitle=false&download=true';
					case 'episode':   return '/api/series/episodes/search/list/' + id + '?download=true';
					case 'album':     return '/api/music/search/list/' + id + '?download=true';
					case 'book':      return '/api/books/search/list/' + id + '?download=true';
					case 'audiobook': return '/api/audiobooks/search/list/' + id + '?download=true';
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
				gmdApiProxyFetch(url)
					.then(function(r){
						// 204 means no media group has this entry's list.
						if (r.status === 204) { throw new Error('the entry\'s list is in no media group'); }
						return r.json().catch(function(){ return {}; });
					})
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
