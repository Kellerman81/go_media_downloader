package api

import (
	"errors"
	"net/http"
	"strconv"

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
			"Titles deleted with \"Don't add again from lists\". The list imports and searches of their media group skip them; "+
				"adding one by hand on Add Media still works. Remove an exclusion to let the lists add the title again."),
		html.Div(html.ID("exclusionsContent"), renderExclusions(getCSRFToken(c), ""))))
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
