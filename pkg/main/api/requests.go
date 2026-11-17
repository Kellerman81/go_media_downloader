package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// A requestinguser's Add Media call creates the list entry as for an admin,
// but held back from searching (dont_search) and recorded in
// media_requests, until an admin approves it on the Requests page.

// requestListField is the form field naming the target list, per kind.
var requestListField = map[string][]string{
	"movie": {"movie_list", "manualMovie_list"}, "series": {"series_list", "manualSeries_list"},
	"book": {"book_list", "manualBook_list"}, "audiobook": {"audiobook_list", "manualAudiobook_list"},
	"album": {"music_list"},
}

// bufferedWriter holds a handler's response so it can be changed before it
// is sent.
type bufferedWriter struct {
	gin.ResponseWriter
	body   bytes.Buffer
	status int
}

func (w *bufferedWriter) WriteHeader(code int)              { w.status = code }
func (w *bufferedWriter) WriteHeaderNow()                   {}
func (w *bufferedWriter) Write(b []byte) (int, error)       { return w.body.Write(b) }
func (w *bufferedWriter) WriteString(s string) (int, error) { return w.body.WriteString(s) }
func (w *bufferedWriter) Status() int                       { return w.status }
func (w *bufferedWriter) Written() bool                     { return w.body.Len() > 0 }
func (w *bufferedWriter) Size() int                         { return w.body.Len() }

// requestable wraps an Add Media handler of one kind: for a non-admin
// (with the request feature) the entries it creates are held back as
// requests.
func requestable(kind string, h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		acc := accessOf(c)
		if acc.Admin() {
			h(c)
			return
		}

		var listname string

		for _, f := range requestListField[kind] {
			if v := strings.TrimSpace(c.PostForm(f)); v != "" {
				listname = v
			}
		}

		before := database.MaxEntryID(kind)

		orig := c.Writer
		buf := &bufferedWriter{ResponseWriter: orig, status: http.StatusOK}
		c.Writer = buf

		h(c)

		c.Writer = orig

		var held []string

		if buf.status == http.StatusOK && listname != "" {
			k := libKindByType(kind)

			for _, id := range database.NewEntryIDs(kind, before, &listname) {
				title := ""
				if it := loadLibraryItem(k, new(uint64(id))); it != nil {
					title = it.Title
				}

				r := database.MediaRequest{Username: acc.Name, Kind: kind, EntryID: id, Listname: listname, Title: title}
				if err := database.HoldMediaRequest(&r); err == nil {
					held = append(held, title)
					notifyAccountEvent(c, providers.EventRequested, "/api/admin/requests",
						&accountEvent{List: listname, User: acc.Name, Type: kind, Title: title})
				}
			}
		}

		body := buf.body.Bytes()

		if len(held) > 0 {
			body, _ = json.Marshal(gin.H{"success": "Requested: " + strings.Join(held, ", ") +
				". An admin has to approve it before it is searched."})
		}

		for k, v := range buf.Header() {
			orig.Header()[k] = v
		}

		orig.WriteHeader(buf.status)
		_, _ = orig.Write(body)
	}
}

// RequestsPage lists the open requests (/admin/requests, admins).
func RequestsPage(c *gin.Context) {
	renderPageIn(c, "Requests", true, html.Div(html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-inbox", "Requests",
			"Media requesting users added. It is in their list but not searched until you approve it; rejecting removes it again."),
		html.Div(html.ID("requestsContent"), renderRequests(getCSRFToken(c), ""))))
}

func renderRequests(csrf, alert string) gomponents.Node {
	reqs := database.GetMediaRequests()
	if len(reqs) == 0 {
		return html.Div(gomponents.Raw(alert), html.P(html.Class("text-muted"), gomponents.Text("No open requests.")))
	}

	button := func(action, class, icon, label string, id uint, confirm string) gomponents.Node {
		return html.Button(html.Type("button"), html.Class("btn btn-sm "+class+" me-1"),
			hx.Post("/api/admin/requests/"+action), hx.Vals(`{"id": "`+strconv.FormatUint(uint64(id), 10)+`"}`),
			hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#requestsContent"), hx.Swap("innerHTML"),
			gomponents.If(confirm != "", hx.Confirm(confirm)),
			html.I(html.Class("fa-solid "+icon+" me-1")), gomponents.Text(label))
	}

	qualities := getQualityProfileNames()

	rows := make([]gomponents.Node, 0, len(reqs))
	for i := range reqs {
		r := &reqs[i]
		k := libKindByType(r.Kind)
		rowID := "req" + strconv.FormatUint(uint64(r.ID), 10)

		title := gomponents.Node(gomponents.Text(r.Title))
		if k != nil {
			title = html.A(html.Href("/api/admin/library#"+r.Kind+"/"+strconv.FormatUint(uint64(r.EntryID), 10)), gomponents.Text(r.Title))
		}

		rows = append(rows, html.Tr(html.ID(rowID),
			html.Td(html.Class("small text-muted"), gomponents.Text(shortTime(r.CreatedAt))),
			html.Td(gomponents.Text(r.Username)),
			html.Td(gomponents.Text(r.Kind)),
			html.Td(title),
			html.Td(requestListSelect(r)),
			html.Td(requestQualitySelect(qualities, database.RequestEntryQuality(r))),
			html.Td(html.Class("text-end text-nowrap"),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-success me-1"),
					hx.Post("/api/admin/requests/approve"), hx.Vals(`{"id": "`+strconv.FormatUint(uint64(r.ID), 10)+`"}`),
					hx.Include("#"+rowID+" select"), hx.Headers(wizardCSRFHeader(csrf)), hx.Target("#requestsContent"), hx.Swap("innerHTML"),
					html.I(html.Class("fa-solid fa-check me-1")), gomponents.Text("Approve")),
				button("reject", "btn-outline-danger", "fa-xmark", "Reject", r.ID, "Remove "+r.Title+" from "+r.Listname+"?"))))
	}

	return html.Div(gomponents.Raw(alert),
		html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle"),
			html.THead(html.Tr(html.Th(gomponents.Text("Requested")), html.Th(gomponents.Text("By")), html.Th(gomponents.Text("Type")),
				html.Th(gomponents.Text("Title")), html.Th(gomponents.Text("List")), html.Th(gomponents.Text("Quality")), html.Th())),
			html.TBody(gomponents.Group(rows)))))
}

func requestFromForm(c *gin.Context) (*database.MediaRequest, bool) {
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)
	if err != nil {
		return nil, false
	}

	r, err := database.GetMediaRequest(new(uint(id)))

	return r, err == nil && r.ID != 0
}

func requestsReply(c *gin.Context, msg, level string) {
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, renderComponentToString(renderRequests(getCSRFToken(c), renderAlert(msg, level))))
}

// HandleRequestApprove lets a requested entry be searched (/admin/requests/approve).
func HandleRequestApprove(c *gin.Context) {
	r, ok := requestFromForm(c)
	if !ok {
		requestsReply(c, "The request no longer exists.", "warning")
		return
	}

	listname, quality := strings.TrimSpace(c.PostForm("list")), strings.TrimSpace(c.PostForm("quality"))

	if listname != "" && !slices.ContainsFunc(requestGroupLists(r), func(l string) bool { return strings.EqualFold(l, listname) }) {
		requestsReply(c, "Not approved: "+listname+" is not a list of the media group of "+r.Listname+".", "danger")
		return
	}

	if _, ok := config.GetSettingsQualityOk(quality); quality != "" && !ok {
		requestsReply(c, "Not approved: the quality profile "+quality+" is not configured.", "danger")
		return
	}

	if err := database.ApproveMediaRequest(r, listname, quality); err != nil {
		requestsReply(c, "Not approved: "+err.Error(), "danger")
		return
	}

	where := ""
	if listname != "" && !strings.EqualFold(listname, r.Listname) {
		where = " (moved to " + listname + ")"
	}

	requestsReply(c, r.Title+" approved"+where+": it is searched with the next scheduled search.", "success")
}

// HandleRequestReject removes a requested entry from its list (/admin/requests/reject).
func HandleRequestReject(c *gin.Context) {
	r, ok := requestFromForm(c)
	if !ok {
		requestsReply(c, "The request no longer exists.", "warning")
		return
	}

	if k := libKindByType(r.Kind); k != nil {
		if err := deleteLibraryEntry(k, new(uint64(r.EntryID))); err != nil {
			requestsReply(c, "Not removed: "+err.Error(), "danger")
			return
		}
	}

	_ = database.CloseMediaRequest(&r.ID)
	requestsReply(c, r.Title+" rejected and removed from "+r.Listname+".", "success")
}

// requestConfigPrefix is the media config name prefix of a request kind.
var requestConfigPrefix = map[string]string{"movie": "movie", "series": "serie", "album": "music", "audiobook": "audiobook", "book": "book"}

// requestGroupLists are the lists of the media group a request's list
// belongs to: a list of another group isn't searched by this one, so the
// request stays in its group. Just its own list when the group is gone.
func requestGroupLists(r *database.MediaRequest) []string {
	prefix := requestConfigPrefix[r.Kind]

	cfgp, listid := findCfgpAndListID(r.Listname, prefix+"_", mediaEntriesForKind(prefix))
	if cfgp == nil || listid < 0 {
		return []string{r.Listname}
	}

	lists := make([]string, len(cfgp.Lists))
	for i := range cfgp.Lists {
		lists[i] = cfgp.Lists[i].Name
	}

	return lists
}

// requestListSelect picks the list of the request's media group it goes
// to; its own list preselected.
func requestListSelect(r *database.MediaRequest) gomponents.Node {
	lists := requestGroupLists(r)

	opts := make([]gomponents.Node, 0, len(lists))
	for _, l := range lists {
		opts = append(opts, html.Option(html.Value(l), gomponents.If(strings.EqualFold(l, r.Listname), html.Selected()), gomponents.Text(l)))
	}

	return html.Select(html.Class("form-select form-select-sm"), html.Name("list"), gomponents.Attr("aria-label", "List"),
		gomponents.Group(opts))
}

// requestQualitySelect picks the quality profile; the entry's preselected.
func requestQualitySelect(names []string, current string) gomponents.Node {
	opts := make([]gomponents.Node, 0, len(names)+1)
	if current != "" && !slices.Contains(names, current) {
		opts = append(opts, html.Option(html.Value(current), html.Selected(), gomponents.Text(current)))
	}

	for _, n := range names {
		opts = append(opts, html.Option(html.Value(n), gomponents.If(n == current, html.Selected()), gomponents.Text(n)))
	}

	return html.Select(html.Class("form-select form-select-sm"), html.Name("quality"), gomponents.Attr("aria-label", "Quality profile"),
		gomponents.Group(opts))
}
