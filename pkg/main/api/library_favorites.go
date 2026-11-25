package api

import (
	"net/http"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Favorites are the account's playlist "Favorites", made with the first
// favorite: they play and show like any playlist, in the Library and in
// the apps (Jellyfin's favorites, Subsonic's starred). Items are playlist
// items: movie, series, album, audiobook by list entry, episode
// (serie_episodes ID) and track (album_files ID) by item.

const favoritesName = "Favorites"

// favoritesID is the account's favorites playlist (0 without one; create
// makes it).
func favoritesID(user string, create bool) uint {
	id := database.Getdatarow[uint](false, "SELECT id FROM playlists WHERE username = ? AND name = ?", user, favoritesName)
	if id != 0 || !create {
		return id
	}

	p := database.Playlist{Username: user, Name: favoritesName}
	if err := database.SavePlaylist(&p); err != nil {
		return 0
	}

	return p.ID
}

// favoriteColumn is the playlist_items column naming an item of a kind.
func favoriteColumn(kind string) string {
	if kind == "episode" || kind == "track" {
		return "item_id"
	}

	return "entry_id"
}

// favoriteCond is the SQL condition "idExpr is a favorite of the account"
// for items of a playlist kind, with its arguments.
func favoriteCond(user, kind, idExpr string) (string, []any) {
	return "EXISTS (SELECT 1 FROM playlist_items fi INNER JOIN playlists fp ON fp.id = fi.playlist_id WHERE fp.username = ? " +
		"AND fp.name = ? AND fi.kind = ? AND fi." + favoriteColumn(kind) + " = " + idExpr + ")", []any{user, favoritesName, kind}
}

// favoriteSet returns the account's favorites by playlist kind and ID.
func favoriteSet(user string) map[string]map[uint]bool {
	out := map[string]map[uint]bool{}

	id := favoritesID(user, false)
	if id == 0 {
		return out
	}

	for _, it := range database.GetPlaylistItems(&id) {
		n := it.EntryID
		if favoriteColumn(it.Kind) == "item_id" {
			n = it.ItemID
		}

		if out[it.Kind] == nil {
			out[it.Kind] = map[uint]bool{}
		}

		out[it.Kind][n] = true
	}

	return out
}

// setFavorite adds an item to the account's favorites or takes it out.
// entry is the list entry (series of an episode, album of a track), item
// the episode or track ID.
func setFavorite(acc *userAccess, kind string, entry, item uint, on bool) error {
	user := accName(acc)

	if !on {
		id := favoritesID(user, false)
		if id == 0 {
			return nil
		}

		n := entry
		if favoriteColumn(kind) == "item_id" {
			n = item
		}

		return database.ExecNErr("DELETE FROM playlist_items WHERE playlist_id = ? AND kind = ? AND "+favoriteColumn(kind)+" = ?",
			&id, kind, &n)
	}

	items := playlistItemsFor(acc, []database.PlaylistItem{{Kind: kind, EntryID: entry, ItemID: item}})
	if len(items) == 0 {
		return database.ErrPlaylistMissing
	}

	id := favoritesID(user, true)
	if id == 0 {
		return database.ErrPlaylistMissing
	}

	_, err := database.AddPlaylistItems(user, &id, items)

	return err
}

// HandleLibraryFavorite sets or clears a favorite of the Library
// (/admin/library/playlists/favorite: kind, entry, item, on = 1|0).
func HandleLibraryFavorite(c *gin.Context) {
	entry, _ := strconv.ParseUint(c.PostForm("entry"), 10, 64)
	item, _ := strconv.ParseUint(c.PostForm("item"), 10, 64)
	kind, on := c.PostForm("kind"), c.PostForm("on") == "1"

	if database.PlaylistEntryKinds[kind] == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kind"})
		return
	}

	if err := setFavorite(accessOf(c), kind, uint(entry), uint(item), on); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": t("entry not found")})
		return
	}

	c.JSON(http.StatusOK, gin.H{"favorite": on})
}

// isFavorite reports whether an item is one of the account's favorites.
func isFavorite(user, kind string, entry, item uint) bool {
	cond, args := favoriteCond(user, kind, "?")

	n := entry
	if favoriteColumn(kind) == "item_id" {
		n = item
	}

	return database.Getdatarow[bool](false, "SELECT "+cond, append(args, n)...)
}

// libFavoriteButton is the heart that makes an item a favorite or not
// (libFavorite); small for the rows of episodes and tracks.
func libFavoriteButton(kind string, entry, item uint, on, small bool) gomponents.Node {
	class, icon, title := "btn btn-sm btn-outline-danger", "fa-regular fa-heart", t("Add to favorites")
	if on {
		class, icon, title = "btn btn-sm btn-danger", "fa-solid fa-heart", t("Remove from favorites")
	}

	if small {
		class += " py-0 me-1"
	}

	return html.Button(html.Type("button"), html.Class(class+" lib-fav"), html.Title(title), gomponents.Attr("aria-label", title),
		html.Data("kind", kind), html.Data("entry", strconv.FormatUint(uint64(entry), 10)),
		html.Data("item", strconv.FormatUint(uint64(item), 10)), html.Data("on", map[bool]string{true: "1", false: "0"}[on]),
		gomponents.Attr("onclick", "libFavorite(this)"), html.I(html.Class(icon)))
}

// favoriteJS toggles a favorite button (libFavoriteButton).
const favoriteJS = `
function libFavorite(btn) {
	var on = btn.dataset.on !== '1';
	libPlaylistPost('/api/admin/library/playlists/favorite', { kind: btn.dataset.kind, entry: btn.dataset.entry, item: btn.dataset.item, on: on ? '1' : '0' })
		.then(function () {
			btn.dataset.on = on ? '1' : '0';
			btn.classList.toggle('btn-danger', on);
			btn.classList.toggle('btn-outline-danger', !on);
			btn.querySelector('i').className = on ? 'fa-solid fa-heart' : 'fa-regular fa-heart';
			btn.title = on ? _t('Remove from favorites') : _t('Add to favorites');
			libPlaylistCache = null;
		})
		.catch(function (e) { libNotify('error', e.message); });
}
`
