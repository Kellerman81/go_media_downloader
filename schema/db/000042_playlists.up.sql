-- Playlists of the Library: an account's own lists of what to play, which
-- it may share (read only) with the other accounts.
CREATE TABLE IF NOT EXISTS playlists (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL COLLATE NOCASE,
	name TEXT NOT NULL COLLATE NOCASE,
	shared INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	updated_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	UNIQUE (username, name)
);

-- What a playlist holds, in order. kind: movie, series, album, audiobook
-- (a whole list entry: entry_id) or episode, track (one part of the list
-- entry entry_id: item_id is the serie_episodes row / the album_files row,
-- ref_id a track's dbtracks row, which finds the track again after its file
-- was replaced).
CREATE TABLE IF NOT EXISTS playlist_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	playlist_id INTEGER NOT NULL REFERENCES playlists (id) ON DELETE CASCADE,
	kind TEXT NOT NULL,
	entry_id INTEGER NOT NULL,
	item_id INTEGER NOT NULL DEFAULT 0,
	ref_id INTEGER NOT NULL DEFAULT 0,
	title TEXT NOT NULL DEFAULT '',
	position INTEGER NOT NULL DEFAULT 0,
	added_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	UNIQUE (playlist_id, kind, entry_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_playlist_items_playlist ON playlist_items (playlist_id, position);
