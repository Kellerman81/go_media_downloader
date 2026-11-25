package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// Playlists and favorites in the Jellyfin API: the Library's playlists
// (the account's and the shared ones) are a "Playlists" library; the
// favorites are the account's favorites playlist (library_favorites.go).

// jfPlaylistKinds are the playlist item kinds of the item kinds.
var jfPlaylistKinds = map[byte]string{
	jfMovie: "movie", jfSeries: "series", jfEpisode: "episode", jfAlbum: "album", jfTrack: "track", jfBook: "audiobook",
}

// jfPlaylistItem is the playlist item of an item ID.
func jfPlaylistItem(id string) (database.PlaylistItem, bool) {
	code, n, ok := parseJFID(id)
	kind := jfPlaylistKinds[code]

	if !ok || kind == "" {
		return database.PlaylistItem{}, false
	}

	it := database.PlaylistItem{Kind: kind, EntryID: uint(n)}

	switch code {
	case jfEpisode:
		it.EntryID, it.ItemID = database.Getdatarow[uint](false, "SELECT serie_id FROM serie_episodes WHERE id = ?", &n), uint(n)
	case jfTrack:
		it.EntryID, it.ItemID = database.Getdatarow[uint](false, "SELECT album_id FROM album_files WHERE id = ?", &n), uint(n)
	}

	return it, it.EntryID != 0
}

// jfFavoriteOf reports whether a row of a kind is in the favorites.
func jfFavoriteOf(favs map[string]map[uint]bool, code byte, id uint64) bool {
	kind := jfPlaylistKinds[code]
	return kind != "" && favs[kind][uint(id)]
}

// jfSetFavorite adds an item to the account's favorites or takes it out.
func jfSetFavorite(c *gin.Context, id string, on bool) {
	if _, ok := jfLoad(c, jfGroups(c), id); !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if it, ok := jfPlaylistItem(id); ok {
		_ = setFavorite(accessOf(c), it.Kind, it.EntryID, it.ItemID, on)
	}

	jfUserDataReply(c, id)
}

// --- playlists --------------------------------------------------------------

// jfPlaylistsViewID is the ID of the playlists library.
var jfPlaylistsViewID = jfID(jfPlaylists, 1)

func jfPlaylistsView(lang string) jfItem {
	return jfItem{
		Name: trText(lang, t("Playlists")), ServerID: jfServerID(), ID: jfPlaylistsViewID, Type: "CollectionFolder", IsFolder: true,
		CollectionType: "playlists", LocationType: "FileSystem", ImageTags: map[string]string{}, BackdropImageTags: []string{},
		PlayAccess: "Full", SortName: "￿",
	}
}

// jfPlaylistItemOf describes a playlist.
func jfPlaylistItemOf(p *database.Playlist) jfItem {
	id := jfID(jfPlaylist, uint64(p.ID))

	return jfItem{
		Name: p.Name, ServerID: jfServerID(), ID: id, Type: "Playlist", IsFolder: true, LocationType: "FileSystem",
		ParentID: jfPlaylistsViewID, ChildCount: p.Items, RecursiveItemCount: p.Items, MediaType: "Unknown",
		ImageTags: map[string]string{}, BackdropImageTags: []string{}, PlayAccess: "Full", SortName: strings.ToLower(p.Name),
		DateCreated: jfCreated(p.Updated + ":00"), CanDelete: true,
	}
}

// jfPlaylistList lists the playlists the account sees, then its saved
// views as smart playlists.
func jfPlaylistList(c *gin.Context) []jfItem {
	var out []jfItem

	user := accName(accessOf(c))

	for _, p := range database.GetPlaylists(user) {
		out = append(out, jfPlaylistItemOf(&p))
	}

	for _, v := range savedViews(user) {
		out = append(out, jfSmartItem(&v))
	}

	return out
}

// jfSmartItem describes a saved view as a playlist.
func jfSmartItem(v *savedView) jfItem {
	return jfItem{
		Name: v.Name, ServerID: jfServerID(), ID: jfID(jfSmart, smartID(v)), Type: "Playlist", IsFolder: true, LocationType: "FileSystem",
		ParentID: jfPlaylistsViewID, MediaType: "Unknown", ImageTags: map[string]string{}, BackdropImageTags: []string{},
		PlayAccess: "Full", SortName: strings.ToLower(v.Name),
	}
}

// jfSmartEntries answers what a saved view finds (a page of it).
func jfSmartEntries(c *gin.Context, id string, start, limit int) bool {
	code, n, ok := parseJFID(id)
	if !ok || code != jfSmart {
		return false
	}

	acc := accessOf(c)

	v, ok := smartView(accName(acc), n)
	if !ok {
		return false
	}

	kind, rows := smartRows(acc, requestLanguage(c), v)

	var items []jfItem
	if kind != 0 {
		items = jfBuild(c, jfKinds[kind], rows)
	}

	total := len(items)
	items = items[min(max(start, 0), total):]

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	jfResult(c, items, total, start)

	return true
}

// jfPlaylistOf is a playlist the account may see.
func jfPlaylistOf(c *gin.Context, id string) (*database.Playlist, bool) {
	code, n, ok := parseJFID(id)
	if !ok || code != jfPlaylist {
		return nil, false
	}

	pid := uint(n)

	p, err := database.GetPlaylist(accName(accessOf(c)), &pid)

	return p, err == nil && p.ID != 0
}

// playlistEntry is an item a playlist plays.
type playlistEntry struct {
	Code byte
	Row  jfRow
	Item jfItem
}

// playlistEntries returns the items a playlist plays, in its order: the
// movies, episodes, tracks and audiobook parts of its files, each with the
// playlist item it comes from.
func playlistEntries(acc *userAccess, groups []libGroup, id uint, lang string) []playlistEntry {
	files := playlistFiles(acc, id, libPlayAllMax)

	type key struct {
		code byte
		id   uint64
	}

	codes := map[string]byte{"movie": jfMovie, "episode": jfEpisode, "track": jfTrack, "audiobook": jfBookPart}
	order := make([]key, 0, len(files))
	from := map[key]uint{}
	ids := map[byte][]any{}

	for _, f := range files {
		code := codes[f.Kind]
		n := uint64(f.ID)

		switch code {
		case jfMovie:
			n = database.Getdatarow[uint64](false, "SELECT movie_id FROM movie_files WHERE id = ?", &f.ID)
		case jfEpisode:
			n = database.Getdatarow[uint64](false, "SELECT serie_episode_id FROM serie_episode_files WHERE id = ?", &f.ID)
		}

		k := key{code, n}
		if _, seen := from[k]; seen || code == 0 {
			continue
		}

		from[k] = f.Item
		order = append(order, k)
		ids[code] = append(ids[code], n)
	}

	found := map[key]playlistEntry{}

	for code, list := range ids {
		k := jfKinds[code]
		rows, _ := jfRows(k, jfQuery{Groups: groups, Cond: " AND x.id IN (?" + strings.Repeat(", ?", len(list)-1) + ")", Args: list, NoTotal: true})
		items := jfItems(k, rows, jfServerID(), lang)

		for i := range rows {
			found[key{code, rows[i].ID}] = playlistEntry{Code: code, Row: rows[i], Item: items[i]}
		}
	}

	out := make([]playlistEntry, 0, len(order))

	for _, k := range order {
		if e, ok := found[k]; ok {
			e.Item.PlaylistItemID = strconv.FormatUint(uint64(from[k]), 10)
			out = append(out, e)
		}
	}

	return out
}

// jfPlaylistEntries answers the items of a playlist (a page of them).
func jfPlaylistEntries(c *gin.Context, p *database.Playlist, start, limit int) {
	acc := accessOf(c)
	user := accName(acc)
	favs := favoriteSet(user)

	entries := playlistEntries(acc, jfGroups(c), p.ID, requestLanguage(c))
	items := make([]jfItem, 0, len(entries))

	for i := range entries {
		e := &entries[i]
		e.Item.UserData = jfUserDataOf(user, jfKinds[e.Code], &e.Row, e.Item.ID)
		e.Item.UserData.IsFavorite = jfFavoriteOf(favs, e.Code, e.Row.ID)
		items = append(items, e.Item)
	}

	total := len(items)
	items = items[min(max(start, 0), total):]

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	jfResult(c, items, total, start)
}

func jfGetPlaylistItems(c *gin.Context, p map[string]string) {
	if jfSmartEntries(c, p["id"], jfInt(c, "startIndex"), jfInt(c, "limit")) {
		return
	}

	pl, ok := jfPlaylistOf(c, p["id"])
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	jfPlaylistEntries(c, pl, jfInt(c, "startIndex"), jfInt(c, "limit"))
}

// jfIDsParam reads item IDs from the query or the JSON body's field.
func jfIDsParam(c *gin.Context, query string, body []string) []string {
	ids := jfList(c, query)
	for _, s := range body {
		if s = strings.TrimSpace(s); s != "" {
			ids = append(ids, strings.ToLower(s))
		}
	}

	return ids
}

// jfAddToPlaylist appends items (IDs) to one of the account's playlists.
func jfAddToPlaylist(c *gin.Context, id uint, ids []string) error {
	var in []database.PlaylistItem

	for _, s := range ids {
		if it, ok := jfPlaylistItem(s); ok {
			in = append(in, it)
		}
	}

	if len(in) == 0 {
		return nil
	}

	acc := accessOf(c)
	_, err := database.AddPlaylistItems(accName(acc), &id, playlistItemsFor(acc, in))

	return err
}

// jfCreatePlaylist makes a playlist (POST /Playlists: Name, Ids).
func jfCreatePlaylist(c *gin.Context, _ map[string]string) {
	var body struct {
		Name string   `json:"Name"`
		Ids  []string `json:"Ids"`
	}

	_ = c.ShouldBindJSON(&body)

	if body.Name == "" {
		body.Name = jfParam(c, "name")
	}

	p := database.Playlist{Username: accName(accessOf(c)), Name: body.Name}
	if err := database.SavePlaylist(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := jfAddToPlaylist(c, p.ID, jfIDsParam(c, "ids", body.Ids)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"Id": jfID(jfPlaylist, uint64(p.ID))})
}

// jfOwnPlaylist is one of the account's own playlists.
func jfOwnPlaylist(c *gin.Context, id string) (*database.Playlist, bool) {
	p, ok := jfPlaylistOf(c, id)
	return p, ok && strings.EqualFold(p.Username, accName(accessOf(c)))
}

func jfPlaylistAdd(c *gin.Context, p map[string]string) {
	pl, ok := jfOwnPlaylist(c, p["id"])
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if err := jfAddToPlaylist(c, pl.ID, jfList(c, "ids")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// jfPlaylistRemove takes entries (their PlaylistItemId) out of a playlist.
func jfPlaylistRemove(c *gin.Context, p map[string]string) {
	pl, ok := jfOwnPlaylist(c, p["id"])
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	for _, s := range jfList(c, "entryIds") {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			item := uint(n)
			_ = database.RemovePlaylistItem(pl.Username, &pl.ID, &item)
		}
	}

	c.Status(http.StatusNoContent)
}

// jfDeleteItem deletes a playlist (the only items apps may delete).
func jfDeleteItem(c *gin.Context, p map[string]string) {
	pl, ok := jfOwnPlaylist(c, p["id"])
	if !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if err := database.DeletePlaylist(pl.Username, &pl.ID); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusNoContent)
}

func init() {
	jfRoute2(http.MethodPost, "playlists", false, jfCreatePlaylist)
	jfRoute2(http.MethodGet, "playlists/{id}/items", false, jfGetPlaylistItems)
	jfRoute2(http.MethodPost, "playlists/{id}/items", false, jfPlaylistAdd)
	jfRoute2(http.MethodDelete, "playlists/{id}/items", false, jfPlaylistRemove)
	jfRoute2(http.MethodDelete, "items/{id}", false, jfDeleteItem)
}
