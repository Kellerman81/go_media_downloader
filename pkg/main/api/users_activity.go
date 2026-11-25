package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The Activity popups of the user management page: what an account played,
// reported and requested.

var userActivityTitles = map[string]string{"plays": "Plays", "reports": "Reports", "requests": "Requests"}

var entryKindLabels = map[string]string{"movie": "Movie", "series": "Series", "album": "Album", "audiobook": "Audiobook", "book": "Book"}

// userActivityButtons open the popups; each shows its count.
func userActivityButtons(id, name string, a database.UserActivity) gomponents.Node {
	btn := func(what, icon string, n int) gomponents.Node {
		return html.Button(html.Type("button"), html.Class("btn btn-sm me-1 "+map[bool]string{true: "btn-outline-info", false: "btn-outline-secondary"}[n > 0]),
			html.Title(userActivityTitles[what]+" of "+name),
			gomponents.Attr("onclick", "usersActivity("+id+", "+jsString(what)+", "+jsString(userActivityTitles[what]+" of "+name)+")"),
			html.I(html.Class("fa-solid "+icon+" me-1")), gomponents.Text(strconv.Itoa(n)))
	}

	return gomponents.Group([]gomponents.Node{
		btn("plays", "fa-play", a.Plays),
		btn("reports", "fa-flag", a.Reports),
		btn("requests", "fa-inbox", a.Requests),
	})
}

// userActivityModal is the one dialog the popups load into.
func userActivityModal() gomponents.Node {
	return html.Div(html.Class("modal fade"), html.ID("usersActivityModal"), html.TabIndex("-1"), html.Role("dialog"),
		html.Aria("labelledby", "usersActivityTitle"),
		html.Div(html.Class("modal-dialog modal-xl modal-dialog-scrollable"),
			html.Div(html.Class("modal-content"),
				html.Div(html.Class("modal-header"),
					html.H5(html.Class("modal-title"), html.ID("usersActivityTitle")),
					html.Button(html.Type("button"), html.Class("btn-close"), html.Data("bs-dismiss", "modal"), html.Aria("label", "Close"))),
				html.Div(html.Class("modal-body")))))
}

// HandleUserActivity renders a popup's content
// (/admin/users/activity?id=&what=plays|reports|requests).
func HandleUserActivity(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid id")
		return
	}

	id := uint(id64)

	u, err := database.GetUserByID(&id)
	if err != nil || u.ID == 0 {
		c.String(http.StatusNotFound, "no such account")
		return
	}

	var node gomponents.Node

	switch c.Query("what") {
	case "plays":
		node = renderUserPlays(u.Username)
	case "reports":
		node = renderUserReports(u.Username)
	case "requests":
		node = renderUserRequests(u.Username)
	default:
		c.String(http.StatusBadRequest, "unknown activity")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = node.Render(c.Writer)
}

func activityTable(heads []string, rows []gomponents.Node) gomponents.Node {
	ths := make([]gomponents.Node, len(heads))
	for i, h := range heads {
		ths[i] = html.Th(gomponents.Text(h))
	}

	return html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm table-striped align-middle mb-0"),
		html.THead(html.Tr(ths...)), html.TBody(rows...)))
}

func activityEmpty(text string) gomponents.Node {
	return html.P(html.Class("text-muted mb-0"), gomponents.Text(text))
}

func renderUserPlays(username string) gomponents.Node {
	t := database.UserPlayTotals(username)
	if t.Titles == 0 {
		return activityEmpty("Nothing played yet.")
	}

	stat := func(label, value string) gomponents.Node {
		return html.Div(html.Class("col"), html.Div(html.Class("border rounded p-2 text-center"),
			html.Div(html.Class("fs-5 fw-semibold"), gomponents.Text(value)),
			html.Div(html.Class("small text-muted"), gomponents.Text(label))))
	}

	plays := database.RecentPlays(username, 100)
	rows := make([]gomponents.Node, 0, len(plays))

	for i := range plays {
		h := &plays[i]
		pct := 0
		if h.Completion > 0 {
			pct = int(h.Completion*100 + 0.5)
		}

		bar := "bg-primary"
		if h.Completion >= database.Watched {
			bar = "bg-success"
		}

		rows = append(rows, html.Tr(
			html.Td(html.Class("small text-nowrap text-muted"), gomponents.Text(shortTime(h.UpdatedAt))),
			html.Td(html.Class("small"), gomponents.Text(orDash(playKindLabels[h.Kind]))),
			html.Td(gomponents.Text(orDash(h.Title))),
			html.Td(html.Class("text-end"), gomponents.Text(strconv.Itoa(h.Plays))),
			html.Td(html.Style("min-width: 8rem"),
				html.Div(html.Class("progress"), html.Style("height: .9rem"), html.Title(strconv.Itoa(pct)+"%"),
					html.Div(html.Class("progress-bar "+bar), html.Style("width: "+strconv.Itoa(pct)+"%"),
						gomponents.Text(strconv.Itoa(pct)+"%")))),
		))
	}

	note := gomponents.Node(nil)
	if t.Titles > len(plays) {
		note = html.P(html.Class("small text-muted mt-2 mb-0"), gomponents.Textf("The last %d of %d files.", len(plays), t.Titles))
	}

	return gomponents.Group([]gomponents.Node{
		html.Div(html.Class("row row-cols-2 row-cols-md-5 g-2 mb-3"),
			stat("Files", strconv.Itoa(t.Titles)),
			stat("Plays", strconv.Itoa(t.Plays)),
			stat("Played", playedTime(t.Hours)),
			stat("Finished", strconv.Itoa(t.Finished)),
			stat("Last played", orDash(t.Last))),
		activityTable([]string{"Last played", "Kind", "Title", "Plays", "Progress"}, rows),
		note,
	})
}

// playedTime shows hours, short times in minutes.
func playedTime(hours float64) string {
	if hours >= 1 {
		return strconv.FormatFloat(hours, 'f', 1, 64) + " h"
	}

	return strconv.Itoa(int(hours*60+0.5)) + " min"
}

func renderUserReports(username string) gomponents.Node {
	reports := database.UserMediaReports(username)
	rows := make([]gomponents.Node, 0, len(reports))

	for i := range reports {
		r := &reports[i]
		reason := database.ReportReasons[r.Reason]
		if reason == "" {
			reason = r.Reason
		}

		rows = append(rows, html.Tr(
			html.Td(html.Class("small text-nowrap text-muted"), gomponents.Text(r.CreatedAt)),
			html.Td(html.Class("small"), gomponents.Text(orDash(entryKindLabels[r.Kind]))),
			html.Td(gomponents.Text(orDash(r.Title))),
			html.Td(html.Span(html.Class("badge text-bg-warning"), gomponents.Text(reason))),
			html.Td(html.Class("small"), gomponents.Text(orDash(r.Note))),
		))
	}

	open := activityEmpty("No open reports.")
	if len(rows) > 0 {
		open = activityTable([]string{"Reported", "Kind", "File", "Reason", "Note"}, rows)
	}

	return gomponents.Group([]gomponents.Node{
		html.H6(html.Class("mb-2"), gomponents.Text("Open")),
		open,
		renderActivityHandled(username, "Report "),
	})
}

func renderUserRequests(username string) gomponents.Node {
	requests := database.UserMediaRequests(username)
	rows := make([]gomponents.Node, 0, len(requests))

	for i := range requests {
		r := &requests[i]
		status := html.Span(html.Class("badge text-bg-secondary"), gomponents.Text("Waiting for approval"))
		if r.Status == "approved" {
			status = html.Span(html.Class("badge text-bg-info"), gomponents.Text("Approved, waiting for download"))
		}

		rows = append(rows, html.Tr(
			html.Td(html.Class("small text-nowrap text-muted"), gomponents.Text(r.CreatedAt)),
			html.Td(html.Class("small"), gomponents.Text(orDash(entryKindLabels[r.Kind]))),
			html.Td(html.A(html.Href("/api/admin/library#"+r.Kind+"/"+strconv.FormatUint(uint64(r.EntryID), 10)),
				gomponents.Text(orDash(r.Title)))),
			html.Td(html.Class("small"), gomponents.Text(orDash(r.Listname))),
			html.Td(status),
		))
	}

	open := activityEmpty("No open requests.")
	if len(rows) > 0 {
		open = activityTable([]string{"Requested", "Kind", "Title", "List", "Status"}, rows)
	}

	return gomponents.Group([]gomponents.Node{
		html.H6(html.Class("mb-2"), gomponents.Text("Open")),
		open,
		renderActivityHandled(username, "Request ", "Now available: "),
	})
}

// renderActivityHandled lists what the account was told about its handled
// requests or reports (handled ones are no longer stored themselves).
func renderActivityHandled(username string, prefixes ...string) gomponents.Node {
	msgs := database.UserNotificationsTitled(username, prefixes...)

	rows := make([]gomponents.Node, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		head, title, _ := strings.Cut(m.Title, ": ")
		rows = append(rows, html.Tr(
			html.Td(html.Class("small text-nowrap text-muted"), gomponents.Text(m.CreatedAt)),
			html.Td(html.Class("small text-nowrap"), gomponents.Text(head)),
			html.Td(gomponents.Text(orDash(title))),
			html.Td(html.Class("small text-muted"), gomponents.Text(m.Message)),
		))
	}

	list := activityEmpty("Nothing handled yet.")
	if len(rows) > 0 {
		list = activityTable([]string{"When", "Outcome", "Title", "Message"}, rows)
	}

	return gomponents.Group([]gomponents.Node{html.H6(html.Class("mt-4 mb-2"), gomponents.Text("Handled")), list})
}
