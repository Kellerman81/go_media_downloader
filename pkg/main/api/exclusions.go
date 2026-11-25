package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// Import exclusions: titles deleted with "don't add again", which the
// list imports and searches of their media group skip.

var errNoMediaGroup = errors.New("the entry's list belongs to no media group")

// excludeLibraryEntry keeps the media group of a list entry from adding
// its title again.
func excludeLibraryEntry(k *libKind, id *uint64) error {
	it := loadLibraryItem(k, id)
	if it == nil {
		return errNoMediaGroup
	}

	cfgp := mediaConfigOfList(it.Listname)
	if cfgp == nil {
		return errNoMediaGroup
	}

	return database.AddImportExclusion(k.Type, cfgp.NamePrefix, &it.DbID, it.Title)
}

// ExclusionsPage lists the exclusions (/admin/exclusions, admins).
func ExclusionsPage(c *gin.Context) {
	renderPageIn(c, "Import Exclusions", true, html.Div(html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-ban", "Import Exclusions",
			"Titles deleted with \"Don't add again from lists\", or added below. The list imports and searches of their media group skip them; "+
				"adding one by hand on Add Media still works. Remove an exclusion to let the lists add the title again."),
		renderExclusionAdd(getCSRFToken(c)),
		html.Div(html.ID("exclusionsContent"), renderExclusions(getCSRFToken(c), ""))))
}

// exclusionSearchLimit caps the titles one search shows.
const exclusionSearchLimit = 50

// renderExclusionAdd searches the metadata of a media group's type for
// titles to exclude.
func renderExclusionAdd(csrf string) gomponents.Node {
	groups := libraryGroups()
	if len(groups) == 0 {
		return nil
	}

	opts := make([]gomponents.Node, 0, len(groups))
	for _, g := range groups {
		opts = append(opts, html.Option(html.Value(g.Name), gomponents.Text(g.Name+" ("+g.Kind.Label+")")))
	}

	return html.Div(html.Class("card mb-4"), html.Div(html.Class("card-body"),
		html.H5(html.Class("card-title"), html.I(html.Class("fa-solid fa-plus me-2")), gomponents.Text("Add an exclusion")),
		html.P(html.Class("small text-muted"),
			gomponents.Text("Search the metadata in the database by title or by its ID (IMDb, TheTVDB, MusicBrainz, ASIN, ISBN) "+
				"and exclude it from a media group. A title that is in one of the group's lists stays there; delete it in the Library to remove it.")),
		html.Form(html.ID("exclusionSearchForm"), html.Class("row g-2 mb-3"),
			hx.Get("/api/admin/exclusions/search"), hx.Target("#exclusionSearchResults"), hx.Swap("innerHTML"),
			hx.Trigger("input changed delay:400ms from:#exclusionSearchQ, change from:#exclusionSearchGroup, submit"),
			html.Div(html.Class("col-md-4"),
				html.Select(html.Class("form-select"), html.ID("exclusionSearchGroup"), html.Name("group"), html.Aria("label", "Media group"),
					gomponents.Group(opts))),
			html.Div(html.Class("col-md-8"),
				html.Input(html.Type("search"), html.Class("form-control"), html.ID("exclusionSearchQ"), html.Name("q"),
					html.Placeholder("Title or ID, e.g. Matrix or tt0133093"), html.AutoComplete("off"), html.Aria("label", "Search")))),
		html.Div(html.ID("exclusionSearchResults"), html.Data("csrf", csrf))))
}

// HandleExclusionSearch lists the titles found to exclude
// (/admin/exclusions/search?group=&q=).
func HandleExclusionSearch(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	g := libGroupByName(c.Query("group"))
	q := strings.TrimSpace(c.Query("q"))

	if g == nil || q == "" {
		c.String(http.StatusOK, "")
		return
	}

	found := database.SearchExclusionCandidates(g.Kind.Type, g.Name, g.Lists, q, exclusionSearchLimit)
	if len(found) == 0 {
		c.String(http.StatusOK, renderComponentToString(html.P(html.Class("text-muted mb-0"),
			gomponents.Text("Nothing found in the "+g.Kind.Label+" metadata."))))

		return
	}

	csrf := getCSRFToken(c)
	rows := make([]gomponents.Node, 0, len(found))

	for i := range found {
		f := &found[i]
		year := ""
		if f.Year > 0 {
			year = strconv.Itoa(f.Year)
		}

		var action gomponents.Node
		if f.Excluded {
			action = html.Span(html.Class("badge text-bg-secondary"), gomponents.Text("Excluded"))
		} else {
			vals, _ := json.Marshal(map[string]string{"group": g.Name, "id": strconv.FormatUint(uint64(f.DBID), 10)})
			action = html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger"),
				hx.Post("/api/admin/exclusions/add"), hx.Vals(string(vals)), hx.Headers(wizardCSRFHeader(csrf)),
				hx.Target("#exclusionsContent"), hx.Swap("innerHTML"),
				// The row shows the new state; the list below is reloaded.
				gomponents.Attr("hx-on::after-request",
					"if (event.detail.successful) { this.outerHTML = '<span class=\"badge text-bg-secondary\">Excluded</span>'; }"),
				html.I(html.Class("fa-solid fa-ban me-1")), gomponents.Text("Exclude"))
		}

		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(orDash(f.Title))),
				gomponents.If(f.Listed, html.Span(html.Class("badge text-bg-info ms-2"), html.Title("In one of the group's lists"),
					gomponents.Text("in a list")))),
			html.Td(html.Class("small"), gomponents.Text(orDash(year))),
			html.Td(html.Class("small font-monospace"), gomponents.Text(orDash(f.ExtID))),
			html.Td(html.Class("text-end"), action)))
	}

	more := gomponents.Node(nil)
	if len(found) == exclusionSearchLimit {
		more = html.P(html.Class("small text-muted mt-2 mb-0"),
			gomponents.Textf("The first %d titles; search more exactly to find others.", exclusionSearchLimit))
	}

	c.String(http.StatusOK, renderComponentToString(gomponents.Group([]gomponents.Node{
		html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(html.Th(gomponents.Text("Title")), html.Th(gomponents.Text("Year")), html.Th(gomponents.Text("ID")), html.Th())),
			html.TBody(rows...))),
		more,
	})))
}

// HandleExclusionAdd excludes a metadata row from a media group
// (/admin/exclusions/add: group, id).
func HandleExclusionAdd(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	id := uint(id64)

	var alert string

	g := libGroupByName(c.PostForm("group"))

	var (
		title string
		found bool
	)

	if g != nil {
		title, found = database.ExclusionTitle(g.Kind.Type, &id)
	}

	switch {
	case g == nil:
		alert = renderAlert("Unknown media group.", "danger")
	case !found:
		alert = renderAlert("No such title in the database.", "danger")
	default:
		if err := database.AddImportExclusion(g.Kind.Type, g.Name, &id, title); err != nil {
			alert = renderAlert(err.Error(), "danger")
		} else {
			alert = renderAlert("Excluded from "+g.Name+": "+orDash(title)+". Its lists won't add it again.", "success")
		}
	}

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, renderComponentToString(renderExclusions(getCSRFToken(c), alert)))
}

func renderExclusions(csrf, alert string) gomponents.Node {
	ex := database.GetImportExclusions()
	if len(ex) == 0 {
		return html.Div(gomponents.Raw(alert), html.P(html.Class("text-muted"), gomponents.Text("No exclusions.")))
	}

	rows := make([]gomponents.Node, 0, len(ex))
	for i := range ex {
		e := &ex[i]
		rows = append(rows, html.Tr(
			html.Td(html.Class("small text-nowrap"), gomponents.Text(shortTime(e.CreatedAt))),
			html.Td(html.Strong(gomponents.Text(orDash(e.Title)))),
			html.Td(html.Class("small"), gomponents.Text(e.Kind)),
			html.Td(html.Class("small"), gomponents.Text(e.MediaGroup)),
			html.Td(html.Class("small font-monospace"), gomponents.Text(orDash(e.ExtID))),
			html.Td(html.Class("text-end"), html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary"),
				hx.Post("/api/admin/exclusions/delete"), hx.Vals(`{"id": "`+strconv.FormatUint(uint64(e.ID), 10)+`"}`),
				hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#exclusionsContent"), hx.Swap("innerHTML"),
				gomponents.Text("Remove")))))
	}

	return html.Div(gomponents.Raw(alert), html.Div(html.Class("card"), html.Div(html.Class("card-body"),
		html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(html.Th(gomponents.Text("Excluded")), html.Th(gomponents.Text("Title")), html.Th(gomponents.Text("Type")),
				html.Th(gomponents.Text("Media group")), html.Th(gomponents.Text("ID")), html.Th())),
			html.TBody(gomponents.Group(rows)))))))
}

// HandleExclusionDelete removes an exclusion (/admin/exclusions/delete: id).
func HandleExclusionDelete(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	id := uint(id64)

	alert := renderAlert("Removed: the lists may add the title again.", "success")
	if err := database.DeleteImportExclusion(&id); err != nil {
		alert = renderAlert(err.Error(), "danger")
	}

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, renderComponentToString(renderExclusions(getCSRFToken(c), alert)))
}
