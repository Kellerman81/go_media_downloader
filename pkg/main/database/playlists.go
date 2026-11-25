package database

import (
	"errors"
	"strings"
)

// Playlist is a list of what to play in the Library (playlists).
type Playlist struct {
	ID       uint   `db:"id"        json:"id"`
	Username string `db:"username"  json:"owner"`
	Name     string `db:"name"      json:"name"`
	Shared   bool   `db:"shared"    json:"shared"`
	Items    int    `db:"items"     json:"items"`
	Updated  string `db:"updated_at" json:"updated"`
}

// PlaylistItem is one entry, episode or track of a playlist.
type PlaylistItem struct {
	ID       uint   `db:"id"       json:"id"`
	Kind     string `db:"kind"     json:"kind"` // movie, series, album, audiobook, episode, track
	EntryID  uint   `db:"entry_id" json:"entry"`
	ItemID   uint   `db:"item_id"  json:"item"`
	RefID    uint   `db:"ref_id"   json:"-"`
	Title    string `db:"title"    json:"title"`
	Position int    `db:"position" json:"position"`
}

// PlaylistEntryKinds are the item kinds that are a whole list entry; the
// others (episode, track) are one part of it, of the entry kind given.
var PlaylistEntryKinds = map[string]string{
	"movie": "movie", "series": "series", "album": "album", "audiobook": "audiobook",
	"episode": "series", "track": "album",
}

var (
	ErrPlaylistName    = errors.New("the playlist needs a name")
	ErrPlaylistExists  = errors.New("you already have a playlist with this name")
	ErrPlaylistMissing = errors.New("no such playlist of yours")
)

const playlistColumns = "p.id, p.username, p.name, p.shared, strftime('%Y-%m-%d %H:%M', p.updated_at) AS updated_at, " +
	"(SELECT count() FROM playlist_items i WHERE i.playlist_id = p.id) AS items"

// GetPlaylists returns the account's playlists and those others share, by
// name (own first).
func GetPlaylists(username string) []Playlist {
	return StructscanT[Playlist](false, 16, "SELECT "+playlistColumns+" FROM playlists p "+
		"WHERE p.username = ? OR p.shared = 1 ORDER BY p.username <> ? COLLATE NOCASE, p.name COLLATE NOCASE", username, username)
}

// GetPlaylist returns a playlist the account may see (its own or shared).
func GetPlaylist(username string, id *uint) (*Playlist, error) {
	return Structscan[Playlist]("SELECT "+playlistColumns+" FROM playlists p WHERE p.id = ? AND (p.username = ? OR p.shared = 1)",
		false, id, username)
}

// SavePlaylist creates (p.ID 0) or renames and shares/unshares one of the
// account's playlists.
func SavePlaylist(p *Playlist) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 100 {
		return ErrPlaylistName
	}

	if Getdatarow[bool](false, "SELECT EXISTS(SELECT 1 FROM playlists WHERE username = ? AND name = ? AND id <> ?)",
		p.Username, p.Name, p.ID) {
		return ErrPlaylistExists
	}

	if p.ID == 0 {
		id, err := ExecNid("INSERT INTO playlists (username, name, shared) VALUES (?, ?, ?)", p.Username, p.Name, p.Shared)
		if err == nil {
			p.ID = uint(id) //nolint:gosec // row ids are positive
		}

		return err
	}

	if !ownsPlaylist(p.Username, &p.ID) {
		return ErrPlaylistMissing
	}

	return ExecNErr("UPDATE playlists SET name = ?, shared = ?, updated_at = datetime('now','localtime') WHERE id = ?",
		p.Name, p.Shared, &p.ID)
}

// DeletePlaylist removes one of the account's playlists with its items.
func DeletePlaylist(username string, id *uint) error {
	if !ownsPlaylist(username, id) {
		return ErrPlaylistMissing
	}

	// The database runs without foreign keys: the items go explicitly.
	if err := ExecNErr("DELETE FROM playlist_items WHERE playlist_id = ?", id); err != nil {
		return err
	}

	return ExecNErr("DELETE FROM playlists WHERE id = ?", id)
}

func ownsPlaylist(username string, id *uint) bool {
	return Getdatarow[bool](false, "SELECT EXISTS(SELECT 1 FROM playlists WHERE id = ? AND username = ?)", id, username)
}

// AddPlaylistItems appends items to one of the account's playlists, in
// order; items it holds already are skipped. It returns how many were
// added.
func AddPlaylistItems(username string, id *uint, items []PlaylistItem) (int, error) {
	if !ownsPlaylist(username, id) {
		return 0, ErrPlaylistMissing
	}

	count := func() int {
		return Getdatarow[int](false, "SELECT count() FROM playlist_items WHERE playlist_id = ?", id)
	}
	before := count()

	for i := range items {
		it := &items[i]
		if PlaylistEntryKinds[it.Kind] == "" {
			continue
		}

		if err := ExecNErr("INSERT OR IGNORE INTO playlist_items (playlist_id, kind, entry_id, item_id, ref_id, title, position) "+
			"VALUES (?, ?, ?, ?, ?, ?, (SELECT COALESCE(max(position), 0) + 1 FROM playlist_items WHERE playlist_id = ?))",
			id, it.Kind, it.EntryID, it.ItemID, it.RefID, it.Title, id); err != nil {
			return count() - before, err
		}
	}

	added := count() - before

	if added > 0 {
		ExecN("UPDATE playlists SET updated_at = datetime('now','localtime') WHERE id = ?", id)
	}

	return added, nil
}

// GetPlaylistItems returns the items of a playlist in order.
func GetPlaylistItems(id *uint) []PlaylistItem {
	return StructscanT[PlaylistItem](false, 32,
		"SELECT id, kind, entry_id, item_id, ref_id, title, position FROM playlist_items WHERE playlist_id = ? ORDER BY position, id", id)
}

// RemovePlaylistItem takes an item out of one of the account's playlists.
func RemovePlaylistItem(username string, id, itemID *uint) error {
	if !ownsPlaylist(username, id) {
		return ErrPlaylistMissing
	}

	return ExecNErr("DELETE FROM playlist_items WHERE id = ? AND playlist_id = ?", itemID, id)
}

// MovePlaylistItem moves an item of one of the account's playlists one
// place up (by < 0) or down.
func MovePlaylistItem(username string, id, itemID *uint, by int) error {
	if !ownsPlaylist(username, id) {
		return ErrPlaylistMissing
	}

	items := GetPlaylistItems(id)

	from := -1
	for i := range items {
		if items[i].ID == *itemID {
			from = i
		}
	}

	to := from + 1
	if by < 0 {
		to = from - 1
	}

	if from < 0 || to < 0 || to >= len(items) {
		return nil
	}

	items[from], items[to] = items[to], items[from]

	// Positions are renumbered 1..n, which also repairs gaps and ties.
	for i := range items {
		if items[i].Position != i+1 {
			if err := ExecNErr("UPDATE playlist_items SET position = ? WHERE id = ?", i+1, &items[i].ID); err != nil {
				return err
			}
		}
	}

	return nil
}
