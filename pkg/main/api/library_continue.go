package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

	done, mark := t("Watched - click to mark as not watched"), t("Mark as watched")
	if kind == "book" {
		done, mark = t("Read - click to mark as not read"), t("Mark as read")
	}

	if c >= database.Watched {
		return html.Button(html.Type("button"), html.Class("btn btn-sm btn-success py-0 me-1"),
			html.Title(done),
			gomponents.Attr("onclick", "libWatched("+jsString(kind)+", "+id+", false, this)"), html.I(html.Class("fa-solid fa-check")))
	}

	return html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary py-0 me-1"), html.Title(mark),
		gomponents.Attr("onclick", "libWatched("+jsString(kind)+", "+id+", true, this)"),
		gomponents.If(c > 0.02, html.Span(html.Class("small"), gomponents.Text(strconv.Itoa(int(c*100))+"%"))),
		gomponents.If(c <= 0.02, html.I(html.Class("fa-regular fa-eye"))))
}

// HandleLibraryWatched marks files watched or not for the account
// (/admin/library/watched: kind, id (repeated: the shown rows of a list),
// watched).
func HandleLibraryWatched(c *gin.Context) {
	kind := c.PostForm("kind")
	k := libKindByType(playLibKinds[kind])
	ids := c.PostFormArray("id")

	if k == nil || len(ids) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": t("file not found")})
		return
	}

	files := make([]uint, 0, len(ids))

	for _, s := range ids {
		id, err := strconv.ParseUint(s, 10, 64)
		if err != nil || !accessOf(c).allowsFile(k, &id) {
			c.JSON(http.StatusNotFound, gin.H{"error": t("file not found")})
			return
		}

		files = append(files, uint(id))
	}

	for i := range files {
		if err := database.SetWatched(accName(accessOf(c)), kind, &files[i], c.PostForm("watched") == "1"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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

	kind := watchKinds[k.Type]
	if kind == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": tf("%s have no watched state", k.Label)})
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
		return t("almost done")
	case m < 60:
		return tf("%d min left", m)
	}

	return tf("%d h %d min left", m/60, m%60)
}

// watchKinds are the player kinds whose files make a list entry watched
// (heard): albums when every track was played.
var watchKinds = map[string]string{"movie": "movie", "series": "episode", "audiobook": "audiobook", "album": "track", "book": "book"}

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
			sub = tf("Part %d of %d", it.Part, it.Parts)
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
				libContinueDrop("continue", g.Kind.Type, uint64(it.EntryID), t("Don’t continue")),
				html.Div(html.Class("lib-continue-bar"), html.Div(html.Style("width:"+strconv.Itoa(int(it.Progress*100))+"%")))),
			html.Div(html.Class("lib-title text-truncate"), gomponents.Text(it.Title)),
			html.Div(html.Class("lib-sub text-truncate"), gomponents.Text(info))))
	}

	return html.Div(html.Class("mb-4"),
		html.H6(html.Class("text-muted mb-2"), html.I(html.Class("fa-solid fa-clock-rotate-left me-1")), gomponents.Text(t("Continue"))),
		html.Div(html.Class("lib-continue"), gomponents.Group(cards)),
		html.StyleEl(gomponents.Raw(libContinueCSS)))
}

// libContinueCSS styles the Continue and Next up rows.
const libContinueCSS = `.lib-continue{display:flex;gap:1rem;overflow-x:auto;padding-bottom:.5rem}
.lib-continue-card{flex:0 0 150px;width:150px;min-width:0;cursor:pointer}
.lib-continue-card .lib-cover{width:150px}
.lib-continue-bar{position:absolute;left:0;right:0;bottom:0;height:5px;background:rgba(0,0,0,.45)}
.lib-continue-bar>div{height:100%;background:var(--bs-danger,#dc3545)}
.lib-continue-drop{position:absolute;top:.3rem;right:.3rem;width:1.75rem;height:1.75rem;padding:0;border:0;border-radius:50%;background:rgba(0,0,0,.6);color:#fff;line-height:1;opacity:.75}
.lib-continue-drop:hover,.lib-continue-drop:focus-visible{opacity:1;background:var(--bs-danger,#dc3545)}`

// libContinueDrop is the button on a Continue or Next up card that removes
// it from the row.
func libContinueDrop(row, typ string, entryID uint64, title string) gomponents.Node {
	return html.Button(html.Type("button"), html.Class("lib-continue-drop"), html.Title(title), html.Aria("label", title),
		gomponents.Attr("onclick", "event.stopPropagation(); libContinueDrop(this, "+jsString(row)+", "+jsString(typ)+", "+
			strconv.FormatUint(entryID, 10)+")"),
		html.I(html.Class("fa-solid fa-xmark")))
}

// HandleLibraryContinueDrop removes a list entry from the account's
// Continue row (where it stopped is cleared) or a series from its Next up
// row (until it plays the series again) (/admin/library/continue-drop:
// row, type, id). A series dropped from Continue leaves Next up too: else
// the episode it stopped in would show there next.
func HandleLibraryContinueDrop(c *gin.Context) {
	row := c.PostForm("row")
	k := libKindByType(c.PostForm("type"))
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)

	if err != nil || k == nil || (row != "continue" && row != "nextup") || !accessOf(c).allowsEntry(k, &id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown entry"})
		return
	}

	kind := continueKinds[k.Type]
	if kind == "" || (row == "nextup" && k.Type != "series") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no such row"})
		return
	}

	user := accName(accessOf(c))

	if row == "continue" {
		err = database.DropContinue(user, kind, k.Files, k.FilesFK, &id)
	}

	if err == nil && k.Type == "series" {
		err = database.HideNextUp(user, id)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// nextEpisodes returns the next episode of the series the account watches
// (the given ones; nil: those it played in the last days, without those it
// removed from Next up): the first one after the last
// one it finished, not finished itself, specials left out; with
// skipStarted not one it stopped in either (the Continue row has it). Rows
// of the Jellyfin episode query.
func nextEpisodes(groups []libGroup, user string, series []uint64, limit int, skipStarted bool) []jfRow {
	if series == nil {
		series = database.NextUpSeries(user, 50)
	}

	k := jfKinds[jfEpisode]
	w := strconv.FormatFloat(database.Watched, 'f', 2, 64)

	var out []jfRow

	for _, sid := range series {
		rows, _ := jfRows(k, jfQuery{
			Groups: groups,
			Cond: " AND x.parent_id = ? AND x.pidx > 0 AND (x.pidx * 10000 + x.idx) > COALESCE((SELECT max(CAST(de.season AS INTEGER) * 10000 + " +
				"CAST(de.episode AS INTEGER)) FROM play_history h INNER JOIN serie_episode_files f ON f.id = h.file_id " +
				"INNER JOIN dbserie_episodes de ON de.id = f.dbserie_episode_id WHERE h.username = ? AND h.kind = 'episode' " +
				"AND h.completion >= " + w + " AND f.serie_id = ? AND CAST(de.season AS INTEGER) > 0), 0)",
			Args:  []any{sid, user, sid},
			Order: "x.pidx, x.idx", Limit: 1, NoTotal: true,
		})

		if skipStarted && len(rows) > 0 && database.PlayResume(user, "episode", new(uint(rows[0].FileID))) > 0 {
			continue
		}

		out = append(out, rows...)
		if len(out) >= limit {
			break
		}
	}

	return out
}

// libNextUpRow lists the next episode of each series of the group the
// account watches; a click plays it.
func libNextUpRow(user, lang string, g *libGroup) gomponents.Node {
	if g.Kind.Type != "series" || user == "" {
		return nil
	}

	rows := nextEpisodes([]libGroup{*g}, user, nil, 12, true)
	if len(rows) == 0 {
		return nil
	}

	items := jfItems(jfKinds[jfEpisode], rows, "", lang)
	cards := make([]gomponents.Node, 0, len(rows))

	for i := range rows {
		r, it := &rows[i], &items[i]
		cover := "/api/admin/library/cover/series/" + strconv.FormatUint(r.Db2, 10)
		ep := fmt.Sprintf("S%02dE%02d", r.Pidx, r.Idx)
		label := it.SeriesName + " " + ep + " " + it.Name

		cards = append(cards, html.Div(html.Class("lib-continue-card"), html.Title(label),
			gomponents.Attr("onclick", "libContinue('episode', "+strconv.FormatUint(r.FileID, 10)+", "+
				strconv.FormatUint(r.ParentID, 10)+", "+jsString(label)+", "+jsString(it.SeriesName)+", "+jsString(cover)+")"),
			html.Div(html.Class("lib-cover"),
				html.Img(html.Src(cover), html.Alt(""), gomponents.Attr("loading", "lazy")),
				libContinueDrop("nextup", "series", r.ParentID, t("Remove from Next up"))),
			html.Div(html.Class("lib-title text-truncate"), gomponents.Text(it.SeriesName)),
			html.Div(html.Class("lib-sub text-truncate"), gomponents.Text(strings.TrimSpace(ep+" "+it.Name)))))
	}

	return html.Div(html.Class("mb-4"),
		html.H6(html.Class("text-muted mb-2"), html.I(html.Class("fa-solid fa-forward me-1")), gomponents.Text(t("Next up"))),
		html.Div(html.Class("lib-continue"), gomponents.Group(cards)),
		html.StyleEl(gomponents.Raw(libContinueCSS)))
}
