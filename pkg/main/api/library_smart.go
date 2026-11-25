package api

import (
	"encoding/json"
	"hash/crc32"
	"net/url"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// Smart playlists are the account's saved views of the Library (a search
// and filters under a name, kept in its settings by the page): the apps
// show each as a playlist of what the view finds now.

// smartMax caps the entries of a smart playlist.
const smartMax = 500

// savedView is a saved view of the Library page.
type savedView struct {
	Name   string `json:"name"`
	Params string `json:"params"`
}

// savedViews returns the account's saved views.
func savedViews(user string) []savedView {
	var settings map[string]any
	if json.Unmarshal([]byte(database.GetUserSettings(user)), &settings) != nil {
		return nil
	}

	raw, _ := settings["lib-saved-views"].(string)

	var views []savedView
	if json.Unmarshal([]byte(raw), &views) != nil {
		return nil
	}

	return views
}

// smartID is a saved view's ID (a hash of its name).
func smartID(v *savedView) uint64 { return uint64(crc32.ChecksumIEEE([]byte(v.Name))) }

// smartView returns the account's saved view with the ID.
func smartView(user string, id uint64) (*savedView, bool) {
	for _, v := range savedViews(user) {
		if smartID(&v) == id {
			return &v, true
		}
	}

	return nil, false
}

// smartEntries returns what a saved view finds for the account, at most
// smartMax: the item kind (movies, series, albums, audiobooks; episodes for
// the episode view) and the IDs in the view's order. Book groups and the
// artist view give nothing.
func smartEntries(acc *userAccess, lang string, v *savedView) (byte, []any) {
	params, err := url.ParseQuery(v.Params)
	if err != nil {
		return 0, nil
	}

	q := libQueryOf(params, acc, lang)
	q.Page, q.Size = 1, smartMax

	g := libGroupByName(q.Group)
	if q.Group == "" || g == nil || q.View == "artists" {
		return 0, nil
	}

	if q.View == "episodes" && g.Kind.Type == "series" {
		rows, _ := libraryEpisodeRows(g, q)

		ids := make([]any, 0, len(rows))
		for i := range rows {
			ids = append(ids, uint64(rows[i].ID))
		}

		return jfEpisode, ids
	}

	code := map[string]byte{"movie": jfMovie, "series": jfSeries, "album": jfAlbum, "audiobook": jfBook}[g.Kind.Type]
	if code == 0 {
		return 0, nil
	}

	rows, _ := libraryRows(g, q)

	ids := make([]any, 0, len(rows))
	for i := range rows {
		ids = append(ids, uint64(rows[i].ID))
	}

	return code, ids
}

// smartRows loads the rows of a saved view's entries in its order (only
// those with files: the apps play them).
func smartRows(acc *userAccess, lang string, v *savedView) (byte, []jfRow) {
	code, ids := smartEntries(acc, lang, v)
	if code == 0 || len(ids) == 0 {
		return code, nil
	}

	rows, _ := jfRows(jfKinds[code], jfQuery{Groups: groupsOf(acc), Cond: " AND x.id IN (?" + strings.Repeat(", ?", len(ids)-1) + ")", Args: ids, NoTotal: true})

	byID := make(map[uint64]jfRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	out := make([]jfRow, 0, len(rows))

	for _, id := range ids {
		if r, ok := byID[id.(uint64)]; ok {
			out = append(out, r)
		}
	}

	return code, out
}
