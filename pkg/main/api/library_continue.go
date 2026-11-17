package api

import (
	"net/http"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Watched marks and the Library's "Continue" row, from the account's play
// history.

// libWatchedMark shows whether the account watched a file (a toggle) or
// how far it got.
func libWatchedMark(kind string, fileID uint, watched map[uint]float64) gomponents.Node {
	c := watched[fileID]
	id := strconv.FormatUint(uint64(fileID), 10)

	if c >= database.Watched {
		return html.Button(html.Type("button"), html.Class("btn btn-sm btn-success py-0 me-1"),
			html.Title("Watched - click to mark as not watched"),
			gomponents.Attr("onclick", "libWatched("+jsString(kind)+", "+id+", false, this)"), html.I(html.Class("fa-solid fa-check")))
	}

	return html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary py-0 me-1"), html.Title("Mark as watched"),
		gomponents.Attr("onclick", "libWatched("+jsString(kind)+", "+id+", true, this)"),
		gomponents.If(c > 0.02, html.Span(html.Class("small"), gomponents.Text(strconv.Itoa(int(c*100))+"%"))),
		gomponents.If(c <= 0.02, html.I(html.Class("fa-regular fa-eye"))))
}

// HandleLibraryWatched marks a file watched or not for the account
// (/admin/library/watched: kind, id, watched).
func HandleLibraryWatched(c *gin.Context) {
	kind := c.PostForm("kind")
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)
	k := libKindByType(playLibKinds[kind])

	if err != nil || k == nil || !accessOf(c).allowsFile(k, &id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	fid := uint(id)
	if err := database.SetWatched(accName(accessOf(c)), kind, &fid, c.PostForm("watched") == "1"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleLibraryWatchedEntry marks all files of a list entry watched or not
// for the account (/admin/library/watched-entry: type, id, watched); the
// selection mode's Watched / Not watched buttons.
func HandleLibraryWatchedEntry(c *gin.Context) {
	k := libKindByType(c.PostForm("type"))
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)

	if k == nil || err != nil || !accessOf(c).allowsEntry(k, &id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown entry"})
		return
	}

	kind := continueKinds[k.Type]
	if kind == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": k.Label + " have no watched state"})
		return
	}

	if err := database.SetEntryWatched(accName(accessOf(c)), kind, k.Files, k.FilesFK, &id, c.PostForm("watched") == "1"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// minutesLeft is "1 h 23 min left", "12 min left" or "almost done".
func minutesLeft(secs float64) string {
	m := int(secs+30) / 60
	switch {
	case m < 1:
		return "almost done"
	case m < 60:
		return strconv.Itoa(m) + " min left"
	}

	return strconv.Itoa(m/60) + " h " + strconv.Itoa(m%60) + " min left"
}

// continueKinds are the player kinds a group's Continue row shows.
var continueKinds = map[string]string{"movie": "movie", "series": "episode", "audiobook": "audiobook"}

// libContinueRow lists what the account started in the group and didn't
// finish; a click plays on where it stopped.
func libContinueRow(user string, g *libGroup) gomponents.Node {
	kind := continueKinds[g.Kind.Type]
	if kind == "" || user == "" {
		return nil
	}

	items := database.ContinuePlaying(user, kind, g.Lists, 12)
	if len(items) == 0 {
		return nil
	}

	cards := make([]gomponents.Node, 0, len(items))

	for i := range items {
		it := &items[i]
		cover := "/api/admin/library/cover/" + g.Kind.Type + "/" + strconv.FormatUint(uint64(it.DbID), 10)

		sub := it.Sub
		if it.Parts > 1 {
			sub = "Part " + strconv.Itoa(it.Part) + " of " + strconv.Itoa(it.Parts)
		}

		label := it.Title
		if sub != "" {
			label += " · " + sub
		}

		info := minutesLeft(it.Left)
		if sub != "" {
			info = sub + " · " + info
		}

		cards = append(cards, html.Div(html.Class("lib-continue-card"), html.Title(label),
			gomponents.Attr("onclick", "libContinue("+jsString(kind)+", "+strconv.FormatUint(uint64(it.FileID), 10)+", "+
				strconv.FormatUint(uint64(it.EntryID), 10)+", "+jsString(label)+", "+jsString(it.Title)+", "+jsString(cover)+")"),
			html.Div(html.Class("lib-cover"+map[bool]string{true: " lib-continue-square"}[g.Kind.Square]),
				html.Img(html.Src(cover), html.Alt(""), gomponents.Attr("loading", "lazy")),
				html.Div(html.Class("lib-continue-bar"), html.Div(html.Style("width:"+strconv.Itoa(int(it.Progress*100))+"%")))),
			html.Div(html.Class("lib-title text-truncate"), gomponents.Text(it.Title)),
			html.Div(html.Class("lib-sub text-truncate"), gomponents.Text(info))))
	}

	return html.Div(html.Class("mb-4"),
		html.H6(html.Class("text-muted mb-2"), html.I(html.Class("fa-solid fa-clock-rotate-left me-1")), gomponents.Text("Continue")),
		html.Div(html.Class("lib-continue"), gomponents.Group(cards)),
		html.StyleEl(gomponents.Raw(`.lib-continue{display:flex;gap:1rem;overflow-x:auto;padding-bottom:.5rem}
.lib-continue-card{flex:0 0 150px;width:150px;min-width:0;cursor:pointer}
.lib-continue-card .lib-cover{width:150px}
.lib-continue-bar{position:absolute;left:0;right:0;bottom:0;height:5px;background:rgba(0,0,0,.45)}
.lib-continue-bar>div{height:100%;background:var(--bs-danger,#dc3545)}`)))
}
