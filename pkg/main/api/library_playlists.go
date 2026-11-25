package api

import (
	"cmp"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Playlists of the Library: an account collects movies, series, albums,
// audiobooks, single episodes and tracks in its own lists, plays them and
// may share them (read only) with the other accounts.

// playlistMaxItems caps what one call adds (a multi-select of a big page).
const playlistMaxItems = 1000

// libPlaylistEntry is a list entry a playlist names, with what the
// playlist view shows of it.
type libPlaylistEntry struct {
	ID       uint   `db:"id"`
	DbID     uint   `db:"db_id"`
	Title    string `db:"title"`
	Listname string `db:"listname"`
}

// libPlaylistEntries loads the list entries of kind with the ids the
// account may see, by id.
func libPlaylistEntries(acc *userAccess, k *libKind, ids []uint) map[uint]libPlaylistEntry {
	out := make(map[uint]libPlaylistEntry, len(ids))
	if len(ids) == 0 {
		return out
	}

	title := "title"
	if k.Type == "series" {
		title = "seriename"
	}

	args := make([]any, 0, len(ids)+8)
	for _, id := range ids {
		args = append(args, id)
	}

	where := "e.id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"

	if !acc.Admin() {
		in, largs := acc.listsIn()
		where += " AND e.listname" + in
		args = append(args, largs...)
	}

	for _, r := range database.StructscanT[libPlaylistEntry](false, uint(len(ids)), //nolint:gosec // a length
		"SELECT e.id, d.id AS db_id, COALESCE(d."+title+", '') AS title, e.listname FROM "+k.entryTable()+" e "+
			"INNER JOIN "+k.DbTable+" d ON d.id = e."+k.dbColumn()+" WHERE "+where, args...) {
		out[r.ID] = r
	}

	return out
}

// libPlaylistLoad loads the entries of the items, by entry kind and id.
func libPlaylistLoad(acc *userAccess, items []database.PlaylistItem) map[string]map[uint]libPlaylistEntry {
	ids := map[string][]uint{}
	for i := range items {
		ek := database.PlaylistEntryKinds[items[i].Kind]
		ids[ek] = append(ids[ek], items[i].EntryID)
	}

	out := make(map[string]map[uint]libPlaylistEntry, len(ids))
	for ek, list := range ids {
		if k := libKindByType(ek); k != nil {
			out[ek] = libPlaylistEntries(acc, k, list)
		}
	}

	return out
}

// playlistFromForm is the playlist of the form's (or query's) id the
// account may see.
func playlistFromForm(c *gin.Context) (*database.Playlist, bool) {
	id64, err := strconv.ParseUint(cmp.Or(c.PostForm("id"), c.Query("id")), 10, 64)
	if err != nil {
		return nil, false
	}

	id := uint(id64)

	p, err := database.GetPlaylist(accName(accessOf(c)), &id)
	if err != nil || p.ID == 0 {
		return nil, false
	}

	return p, true
}

// playlistError answers a failed change: the playlist isn't the account's
// (404) or the input is wrong (400).
func playlistError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, database.ErrPlaylistMissing) {
		status = http.StatusNotFound
	}

	c.JSON(status, gin.H{"error": err.Error()})
}

// HandlePlaylists lists the account's and the shared playlists
// (/admin/library/playlists).
func HandlePlaylists(c *gin.Context) {
	name := accName(accessOf(c))
	lists := database.GetPlaylists(name)

	type row struct {
		database.Playlist
		Mine bool `json:"mine"`
	}

	out := make([]row, 0, len(lists))
	for _, p := range lists {
		out = append(out, row{Playlist: p, Mine: strings.EqualFold(p.Username, name)})
	}

	c.JSON(http.StatusOK, gin.H{"lists": out})
}

// HandlePlaylistSave creates (no id) or renames and shares one of the
// account's playlists (/admin/library/playlists/save: id, name, shared).
func HandlePlaylistSave(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	p := database.Playlist{ID: uint(id64), Username: accName(accessOf(c)), Name: c.PostForm("name"), Shared: c.PostForm("shared") == "1"}

	if err := database.SavePlaylist(&p); err != nil {
		playlistError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": p.ID, "name": p.Name, "shared": p.Shared})
}

// HandlePlaylistDelete removes one of the account's playlists
// (/admin/library/playlists/delete: id).
func HandlePlaylistDelete(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	id := uint(id64)

	if err := database.DeletePlaylist(accName(accessOf(c)), &id); err != nil {
		playlistError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// HandlePlaylistAdd appends entries, episodes or tracks to one of the
// account's playlists (/admin/library/playlists/add: id, items = JSON
// [{kind, entry, item}]). Titles are taken from the database; what the
// account may not see or that doesn't exist is skipped.
func HandlePlaylistAdd(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	id := uint(id64)

	var in []database.PlaylistItem
	if err := json.Unmarshal([]byte(c.PostForm("items")), &in); err != nil || len(in) == 0 || len(in) > playlistMaxItems {
		c.JSON(http.StatusBadRequest, gin.H{"error": tf("items must be a JSON list of 1 to %d items", playlistMaxItems)})
		return
	}

	acc := accessOf(c)
	items := playlistItemsFor(acc, in)

	added, err := database.AddPlaylistItems(acc.Name, &id, items)
	if err != nil {
		playlistError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"added": added, "skipped": len(in) - added})
}

// playlistItemsFor checks items to add against what the account may see
// and fills in their titles; the others are left out.
func playlistItemsFor(acc *userAccess, in []database.PlaylistItem) []database.PlaylistItem {
	entries := libPlaylistLoad(acc, in)
	items := make([]database.PlaylistItem, 0, len(in))

	for _, it := range in {
		e, ok := entries[database.PlaylistEntryKinds[it.Kind]][it.EntryID]
		if !ok {
			continue
		}

		it.Title, it.RefID = e.Title, 0

		switch it.Kind {
		case "episode":
			ep := database.StructscanT[struct {
				Ident string `db:"identifier"`
				Title string `db:"title"`
			}](false, 1, `SELECT COALESCE(dse.identifier, '') AS identifier, COALESCE(dse.title, '') AS title FROM serie_episodes se
				INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id WHERE se.id = ? AND se.serie_id = ?`, &it.ItemID, &it.EntryID)
			if len(ep) == 0 {
				continue
			}

			it.Title = strings.TrimSpace(e.Title + " " + ep[0].Ident + " " + ep[0].Title)
		case "track":
			tr := database.StructscanT[struct {
				Location string `db:"location"`
				Track    uint   `db:"dbtrack_id"`
			}](false, 1, "SELECT location, COALESCE(dbtrack_id, 0) AS dbtrack_id FROM album_files WHERE id = ? AND album_id = ?",
				&it.ItemID, &it.EntryID)
			if len(tr) == 0 {
				continue
			}

			it.Title, it.RefID = e.Title+" – "+strings.TrimSuffix(path.Base(tr[0].Location), path.Ext(tr[0].Location)), tr[0].Track
		default:
			it.ItemID = 0
		}

		items = append(items, it)
	}

	return items
}

// HandlePlaylistRemove takes an item out of one of the account's
// playlists (/admin/library/playlists/remove: id, item).
func HandlePlaylistRemove(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	item64, _ := strconv.ParseUint(c.PostForm("item"), 10, 64)
	id, item := uint(id64), uint(item64)

	if err := database.RemovePlaylistItem(accName(accessOf(c)), &id, &item); err != nil {
		playlistError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"removed": item})
}

// HandlePlaylistMove moves an item of one of the account's playlists one
// place up or down (/admin/library/playlists/move: id, item, by = -1|1).
func HandlePlaylistMove(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	item64, _ := strconv.ParseUint(c.PostForm("item"), 10, 64)
	id, item := uint(id64), uint(item64)
	by, _ := strconv.Atoi(c.PostForm("by"))

	if err := database.MovePlaylistItem(accName(accessOf(c)), &id, &item, by); err != nil {
		playlistError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"moved": item})
}

// HandlePlaylistPlay returns the files of a playlist to play, in its order
// (/admin/library/playlists/play?id=[&shuffle=1]): a movie's, series',
// album's or audiobook's files in their order, an episode's file, a track.
// What the account may not see is left out.
func HandlePlaylistPlay(c *gin.Context) {
	p, ok := playlistFromForm(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": t("no such playlist")})
		return
	}

	list := playlistFiles(accessOf(c), p.ID, libPlayAllMax)

	if c.Query("shuffle") == "1" {
		rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	}

	c.JSON(http.StatusOK, gin.H{"list": list, "entry": p.Name, "capped": len(list) == libPlayAllMax})
}

// playlistFile is a file a playlist plays, with its playlist item.
type playlistFile struct {
	libPlaylistItem
	Item uint `json:"-"` // playlist_items.id
}

// playlistFiles returns the files a playlist plays, in its order (at most
// limit): a movie's, series', album's or audiobook's files in their order,
// an episode's file, a track. What the account may not see is left out.
func playlistFiles(acc *userAccess, id uint, limit int) []playlistFile {
	items := database.GetPlaylistItems(&id)
	entries := libPlaylistLoad(acc, items)

	type file struct {
		ID       uint   `db:"id"`
		Location string `db:"location"`
		Label    string `db:"label"`
	}

	list := make([]playlistFile, 0, len(items))

	for i := range items {
		it := &items[i]

		e, ok := entries[database.PlaylistEntryKinds[it.Kind]][it.EntryID]
		if !ok {
			continue
		}

		var (
			kind  string
			files []file
		)

		switch it.Kind {
		case "movie":
			kind = "movie"
			files = database.StructscanT[file](false, 4, "SELECT id, location, ? AS label FROM movie_files WHERE movie_id = ? "+
				"AND location != '' ORDER BY "+libFileOrder["movie"], e.Title, &it.EntryID)
		case "series":
			kind = "episode"
			files = database.StructscanT[file](false, 64, `SELECT f.id, f.location,
				TRIM(COALESCE(dse.identifier, '') || ' ' || COALESCE(dse.title, '')) AS label
				FROM serie_episode_files f INNER JOIN serie_episodes se ON se.id = f.serie_episode_id
				INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id
				WHERE se.serie_id = ? AND f.location != ''
				ORDER BY CAST(dse.season AS INTEGER), CAST(dse.episode AS INTEGER), dse.identifier, f.id`, &it.EntryID)
		case "episode":
			kind = "episode"
			files = database.StructscanT[file](false, 1, `SELECT f.id, f.location,
				TRIM(COALESCE(dse.identifier, '') || ' ' || COALESCE(dse.title, '')) AS label
				FROM serie_episode_files f INNER JOIN serie_episodes se ON se.id = f.serie_episode_id
				INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id
				WHERE se.id = ? AND se.serie_id = ? AND f.location != '' ORDER BY f.id LIMIT 1`, &it.ItemID, &it.EntryID)
		case "album":
			kind = "track"
			files = database.StructscanT[file](false, 32, "SELECT id, location, '' AS label FROM album_files WHERE album_id = ? "+
				"AND location != '' ORDER BY "+libFileOrder["album"], &it.EntryID)
		case "track":
			// A replaced file is found again by its track.
			kind = "track"
			files = database.StructscanT[file](false, 1, "SELECT id, location, '' AS label FROM album_files WHERE album_id = ? "+
				"AND location != '' AND (id = ? OR (? > 0 AND dbtrack_id = ?)) ORDER BY id = ? DESC LIMIT 1",
				&it.EntryID, &it.ItemID, &it.RefID, &it.RefID, &it.ItemID)
		case "audiobook":
			kind = "audiobook"
			files = database.StructscanT[file](false, 32, "SELECT id, location, '' AS label FROM audiobook_files WHERE audiobook_id = ? "+
				"AND location != '' ORDER BY "+libFileOrder["audiobook"], &it.EntryID)
		}

		for _, f := range files {
			label := f.Label
			if label == "" {
				label = path.Base(f.Location)
			}

			if it.Kind == "series" || it.Kind == "episode" {
				label = e.Title + " " + label
			}

			list = append(list, playlistFile{libPlaylistItem{Kind: kind, ID: f.ID, Label: label}, it.ID})
			if len(list) == limit {
				break
			}
		}

		if len(list) == limit {
			break
		}
	}

	return list
}

var playlistKindLabels = map[string]string{
	"movie": t("Movie"), "series": t("Series"), "album": t("Album"), "audiobook": t("Audiobook"), "episode": t("Episode"), "track": t("Track"),
}

// HandlePlaylistView renders a playlist for its dialog
// (/admin/library/playlists/view?id=).
func HandlePlaylistView(c *gin.Context) {
	p, ok := playlistFromForm(c)
	if !ok {
		c.String(http.StatusNotFound, t("No such playlist."))
		return
	}

	acc := accessOf(c)
	mine := strings.EqualFold(p.Username, accName(acc))
	items := database.GetPlaylistItems(&p.ID)
	entries := libPlaylistLoad(acc, items)

	rows := make([]gomponents.Node, 0, len(items))

	for i := range items {
		it := &items[i]
		ek := database.PlaylistEntryKinds[it.Kind]
		e, ok := entries[ek][it.EntryID]
		k := libKindByType(ek)

		cover := gomponents.Node(html.Div(html.Class("lib-pl-cover")))
		if ok && k != nil {
			cover = html.Img(html.Class("lib-pl-cover"), html.Alt(""), gomponents.Attr("loading", "lazy"),
				html.Src(libCoverURL(k, &libRow{DbID: e.DbID})))
		}

		title := gomponents.Node(html.Span(html.Class("text-muted"), gomponents.Text(it.Title)))
		if ok {
			title = html.A(html.Href("#"), gomponents.Attr("onclick", "libPlaylistOpenEntry("+jsString(ek)+", "+
				strconv.FormatUint(uint64(it.EntryID), 10)+"); return false;"), gomponents.Text(it.Title))
		}

		itemID := strconv.FormatUint(uint64(it.ID), 10)
		rows = append(rows, html.Tr(
			html.Td(cover),
			html.Td(title, gomponents.If(!ok, html.Span(html.Class("badge text-bg-secondary ms-2"),
				html.Title(t("Deleted from the Library, or in a media group you can't see")), gomponents.Text(t("not available"))))),
			html.Td(html.Class("small text-muted"), gomponents.Text(playlistKindLabels[it.Kind])),
			gomponents.If(mine, html.Td(html.Class("text-end text-nowrap"),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary py-0 me-1"), html.Title(t("Move up")),
					gomponents.If(i == 0, html.Disabled()), gomponents.Attr("onclick", "libPlaylistItem('move', "+itemID+", -1)"),
					html.I(html.Class("fa-solid fa-arrow-up"))),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary py-0 me-1"), html.Title(t("Move down")),
					gomponents.If(i == len(items)-1, html.Disabled()), gomponents.Attr("onclick", "libPlaylistItem('move', "+itemID+", 1)"),
					html.I(html.Class("fa-solid fa-arrow-down"))),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger py-0"), html.Title(t("Remove from the playlist")),
					gomponents.Attr("onclick", "libPlaylistItem('remove', "+itemID+", 0)"),
					html.I(html.Class("fa-solid fa-xmark"))))),
		))
	}

	id := strconv.FormatUint(uint64(p.ID), 10)
	owner := gomponents.Node(nil)

	if !mine {
		owner = html.Span(html.Class("badge text-bg-info ms-2"), gomponents.Text("shared by "+p.Username))
	}

	body := gomponents.Node(html.P(html.Class("text-muted mt-3 mb-0"),
		gomponents.Text(t("Empty. Add movies, series, albums, audiobooks, episodes or tracks with the playlist button in their details or in the multi-select."))))
	if len(rows) > 0 {
		body = html.Div(html.Class("table-responsive mt-3"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.TBody(rows...)))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = html.Div(html.ID("libPlaylistBody"), html.Data("id", id), html.Data("name", p.Name), html.Data("mine", strconv.FormatBool(mine)),
		html.Div(html.Class("d-flex flex-wrap align-items-center gap-2"),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-success"), gomponents.If(len(rows) == 0, html.Disabled()),
				gomponents.Attr("onclick", "libPlaylistPlay("+id+", false)"),
				html.I(html.Class("fa-solid fa-play me-1")), gomponents.Text(t("Play"))),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success"), gomponents.If(len(rows) == 0, html.Disabled()),
				gomponents.Attr("onclick", "libPlaylistPlay("+id+", true)"),
				html.I(html.Class("fa-solid fa-shuffle me-1")), gomponents.Text(t("Shuffle"))),
			html.Span(html.Class("small text-muted"), gomponents.Text(tf("%d items", len(items)))),
			owner,
			html.Span(html.Class("flex-grow-1")),
			gomponents.If(mine, html.Div(html.Class("form-check form-switch mb-0 me-2"),
				html.Input(html.Class("form-check-input"), html.Type("checkbox"), html.ID("libPlaylistShared"),
					gomponents.If(p.Shared, html.Checked()), gomponents.Attr("onchange", "libPlaylistSave("+id+", null, this.checked)")),
				html.Label(html.Class("form-check-label small"), html.For("libPlaylistShared"), gomponents.Text(t("Shared with the other accounts"))))),
			gomponents.If(mine, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary"),
				gomponents.Attr("onclick", "libPlaylistRename("+id+")"),
				html.I(html.Class("fa-solid fa-pen me-1")), gomponents.Text(t("Rename")))),
			gomponents.If(mine, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger"),
				gomponents.Attr("onclick", "libPlaylistDelete("+id+")"),
				html.I(html.Class("fa-solid fa-trash me-1")), gomponents.Text(t("Delete")))),
		),
		body,
	).Render(c.Writer)
}

// libPlaylistAddMenu is the "add to playlist" dropdown. items is the JSON
// list of what it adds ([{kind, entry, item}]); "selection" takes the
// multi-select's entries. The menu is filled with the playlists when it
// opens (libPlaylistFill).
func libPlaylistAddMenu(items, label, btnClass string) gomponents.Node {
	return html.Div(html.Class("btn-group lib-pl-add"),
		html.Button(html.Type("button"), html.Class("btn btn-sm dropdown-toggle "+btnClass),
			gomponents.Attr("data-bs-toggle", "dropdown"), gomponents.Attr("aria-expanded", "false"),
			// Fixed: the menu isn't cut off by the tables' scroll boxes.
			gomponents.Attr("data-bs-popper-config", `{"strategy":"fixed"}`),
			html.Title(t("Add to a playlist")), gomponents.Attr("aria-label", t("Add to a playlist")),
			html.I(html.Class("fa-solid fa-list-ul"+map[bool]string{true: " me-1"}[label != ""])), gomponents.Text(label)),
		html.Ul(html.Class("dropdown-menu lib-pl-menu"), html.Data("pl-items", items)))
}

// libPlaylistItemsJSON is the JSON of one playlist item for
// libPlaylistAddMenu.
func libPlaylistItemsJSON(kind string, entry, item uint) string {
	b, _ := json.Marshal([]database.PlaylistItem{{Kind: kind, EntryID: entry, ItemID: item}})
	return string(b)
}

// libPlaylistsButton opens the playlists from the Library's toolbar.
func libPlaylistsButton() gomponents.Node {
	return html.Div(html.Class("btn-group"),
		html.Button(html.Type("button"), html.Class("btn btn-outline-success dropdown-toggle"), html.ID("libPlaylistsBtn"),
			gomponents.Attr("data-bs-toggle", "dropdown"), gomponents.Attr("aria-expanded", "false"),
			html.Title(t("Your playlists and the shared ones")),
			html.I(html.Class("fa-solid fa-list-ul me-1")), gomponents.Text(t("Playlists"))),
		html.Ul(html.Class("dropdown-menu dropdown-menu-end lib-pl-menu"), html.Data("pl-open", "1")))
}

// renderPlaylistModal is the dialog a playlist opens in.
func renderPlaylistModal() gomponents.Node {
	return html.Div(html.Class("modal fade"), html.ID("libPlaylistModal"), html.TabIndex("-1"),
		gomponents.Attr("aria-labelledby", "libPlaylistTitle"), html.Data("bs-focus", "false"),
		html.Div(html.Class("modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable"),
			html.Div(html.Class("modal-content"),
				html.Div(html.Class("modal-header"),
					html.H5(html.Class("modal-title"), html.ID("libPlaylistTitle"),
						html.I(html.Class("fa-solid fa-list-ul me-2 text-success")), html.Span()),
					html.Button(html.Type("button"), html.Class("btn-close"), gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", t("Close")))),
				html.Div(html.Class("modal-body"), html.ID("libPlaylistModalBody")))))
}

const playlistCSS = `
.lib-pl-cover{width:2.5rem;height:3.5rem;object-fit:cover;border-radius:.25rem;background:#e9ecef;display:block}
[data-gmd-dark="1"] .lib-pl-cover{background:#2d3846}
.lib-pl-menu{max-height:60vh;overflow-y:auto}
`

// playlistJS drives the playlist menus and dialog.
const playlistJS = `
var libPlaylistCache = null;

function libPlaylistPost(url, values) {
	values.csrf_token = libCsrf;
	return fetch(url, { method: 'POST', credentials: 'same-origin', body: new URLSearchParams(values), headers: { 'X-CSRF-Token': libCsrf } })
		.then(function (r) { return r.json().catch(function () { return {}; }).then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); });
}

function libPlaylistLoad(force) {
	if (libPlaylistCache && !force) { return Promise.resolve(libPlaylistCache); }
	return fetch('/api/admin/library/playlists', { credentials: 'same-origin' })
		.then(function (r) { return r.json(); })
		.then(function (d) { libPlaylistCache = d.lists || []; return libPlaylistCache; });
}

// libPlaylistItemsOf is what an add menu adds: its JSON, or the
// multi-select's entries (books can't be played).
function libPlaylistItemsOf(menu) {
	var raw = menu.dataset.plItems;
	if (raw === 'selection') {
		return Array.from(libSel.items.values()).filter(function (it) { return it.type !== 'book'; })
			.map(function (it) { return { kind: it.type, entry: +it.id }; });
	}
	try { return JSON.parse(raw || '[]'); } catch (e) { return []; }
}

// libPlaylistFill lists the playlists in a menu when it opens: an add menu
// shows the account's own, the toolbar menu all it may open.
function libPlaylistFill(menu) {
	var open = menu.dataset.plOpen === '1';
	menu.innerHTML = '<li><span class="dropdown-item-text small text-muted">' + _t('Loading…') + '</span></li>';
	libPlaylistLoad(true).then(function (lists) {
		menu.innerHTML = '';
		var add = function (html, onclick, cls) {
			var li = document.createElement('li'), b = document.createElement('button');
			b.type = 'button'; b.className = 'dropdown-item ' + (cls || ''); b.innerHTML = html;
			b.addEventListener('click', onclick); li.appendChild(b); menu.appendChild(li);
		};
		var esc = function (t) { var d = document.createElement('div'); d.textContent = t; return d.innerHTML; };
		var shown = lists.filter(function (p) { return open || p.mine; });
		if (!open) {
			var h = document.createElement('li');
			h.innerHTML = '<h6 class="dropdown-header">' + _t('Add to playlist') + '</h6>';
			menu.appendChild(h);
		}
		shown.forEach(function (p) {
			add('<i class="fa-solid ' + (p.mine ? 'fa-list-ul' : 'fa-share-nodes') + ' me-2 text-muted"></i>' + esc(p.name) +
				' <span class="small text-muted">' + p.items + (p.mine ? '' : ' · ' + esc(p.owner)) + '</span>',
				function () { if (open) { libPlaylistOpen(p.id); } else { libPlaylistAdd(p.id, p.name, libPlaylistItemsOf(menu)); } });
		});
		if (!shown.length) {
			var e = document.createElement('li');
			e.innerHTML = '<span class="dropdown-item-text small text-muted">' + _t('No playlists yet') + '</span>';
			menu.appendChild(e);
		}
		var d = document.createElement('li'); d.innerHTML = '<hr class="dropdown-divider">'; menu.appendChild(d);
		add('<i class="fa-solid fa-plus me-2"></i>' + _t('New playlist…'), function () {
			var name = prompt(_t('Name of the new playlist:'));
			if (!name || !name.trim()) { return; }
			libPlaylistPost('/api/admin/library/playlists/save', { name: name.trim(), shared: '0' }).then(function (p) {
				if (open) { libPlaylistOpen(p.id); } else { libPlaylistAdd(p.id, p.name, libPlaylistItemsOf(menu)); }
			}).catch(function (e) { libNotify('error', e.message); });
		});
	}).catch(function (e) { menu.innerHTML = ''; libNotify('error', _t('Playlists: ') + e.message); });
}

document.addEventListener('show.bs.dropdown', function (e) {
	var menu = e.target.parentElement && e.target.parentElement.querySelector('.lib-pl-menu');
	if (menu) { libPlaylistFill(menu); }
});

function libPlaylistAdd(id, name, items) {
	if (!items.length) { libNotify('error', _t('Nothing to add')); return; }
	libPlaylistPost('/api/admin/library/playlists/add', { id: id, items: JSON.stringify(items) })
		.then(function (d) {
			var msg = d.added === 1 ? _tf('1 item added to "%s"', name) : _tf('%d items added to "%s"', d.added, name);
			if (d.skipped) { msg += _tf(' (%d already in it or not playable)', d.skipped); }
			libNotify(d.added ? 'success' : 'info', msg);
			libPlaylistRefresh(id);
		})
		.catch(function (e) { libNotify('error', _t('Not added: ') + e.message); });
}

function libPlaylistOpen(id) {
	var modal = document.getElementById('libPlaylistModal');
	modal.dataset.id = id;
	modal.querySelector('#libPlaylistTitle span').textContent = '';
	document.getElementById('libPlaylistModalBody').innerHTML = '<div class="text-center p-4"><div class="spinner-border text-success" role="status"></div></div>';
	bootstrap.Modal.getOrCreateInstance(modal).show();
	libPlaylistRefresh(id);
}

// libPlaylistRefresh reloads the open dialog when it shows that playlist.
function libPlaylistRefresh(id) {
	var modal = document.getElementById('libPlaylistModal');
	if (!modal || String(modal.dataset.id) !== String(id)) { return; }
	fetch('/api/admin/library/playlists/view?id=' + encodeURIComponent(id), { credentials: 'same-origin' })
		.then(function (r) { return r.text().then(function (t) { if (!r.ok) { throw new Error(t || ('HTTP ' + r.status)); } return t; }); })
		.then(function (html) {
			var body = document.getElementById('libPlaylistModalBody');
			body.innerHTML = html;
			var root = document.getElementById('libPlaylistBody');
			modal.querySelector('#libPlaylistTitle span').textContent = root ? root.dataset.name : '';
		})
		.catch(function (e) { document.getElementById('libPlaylistModalBody').textContent = e.message; });
}

function libPlaylistItem(action, item, by) {
	var id = document.getElementById('libPlaylistModal').dataset.id;
	libPlaylistPost('/api/admin/library/playlists/' + action, { id: id, item: item, by: by })
		.then(function () { libPlaylistRefresh(id); })
		.catch(function (e) { libNotify('error', e.message); });
}

function libPlaylistSave(id, name, shared) {
	var root = document.getElementById('libPlaylistBody');
	if (name === null) { name = root ? root.dataset.name : ''; }
	if (shared === null) { var s = document.getElementById('libPlaylistShared'); shared = !!(s && s.checked); }
	libPlaylistPost('/api/admin/library/playlists/save', { id: id, name: name, shared: shared ? '1' : '0' })
		.then(function () { libPlaylistCache = null; libPlaylistRefresh(id); })
		.catch(function (e) { libNotify('error', e.message); libPlaylistRefresh(id); });
}

function libPlaylistRename(id) {
	var root = document.getElementById('libPlaylistBody');
	var name = prompt(_t('New name of the playlist:'), root ? root.dataset.name : '');
	if (name && name.trim()) { libPlaylistSave(id, name.trim(), null); }
}

function libPlaylistDelete(id) {
	var root = document.getElementById('libPlaylistBody');
	confirmAction(_t('Please confirm'), _t('Delete the playlist "') + (root ? root.dataset.name : '') + '"? The media stay in the Library.', function () {
		libPlaylistPost('/api/admin/library/playlists/delete', { id: id }).then(function () {
			libPlaylistCache = null;
			bootstrap.Modal.getOrCreateInstance(document.getElementById('libPlaylistModal')).hide();
			libNotify('success', _t('Playlist deleted'));
		}).catch(function (e) { libNotify('error', e.message); });
	});
}

function libPlaylistPlay(id, shuffle) {
	fetch('/api/admin/library/playlists/play?id=' + encodeURIComponent(id) + (shuffle ? '&shuffle=1' : ''), { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) {
			if (!d.list || !d.list.length) { throw new Error(_t('nothing downloaded to play')); }
			if (d.capped) { libNotify('info', _tf('Playing the first %d files.', d.list.length)); }
			bootstrap.Modal.getOrCreateInstance(document.getElementById('libPlaylistModal')).hide();
			lpPlayList(d.list.map(function (x) { return { kind: x.kind, id: String(x.id), label: x.label }; }), 0, d.entry, '');
		})
		.catch(function (e) { libNotify('error', _t('Can\'t play: ') + e.message); });
}

function libPlaylistOpenEntry(type, id) {
	bootstrap.Modal.getOrCreateInstance(document.getElementById('libPlaylistModal')).hide();
	openLibraryItem(type, id);
}
`
