-- Web UI accounts besides the built-in admin login (admin + web API key).
-- role: admin, user (view, play), searchinguser (+ search),
-- requestinguser (+ request new media). media_groups: the media configs a
-- non-admin may see, comma separated (movie_DE,serie_EN); ignored for admins.
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user',
	media_groups TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	updated_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	last_login DATETIME
);

-- Media a requestinguser added: the list entry was created with dont_search
-- set and waits for an admin to approve (search) or reject (remove) it.
-- kind: movie, series, album, audiobook, book (the list entry table).
CREATE TABLE IF NOT EXISTS media_requests (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL,
	kind TEXT NOT NULL,
	entry_id INTEGER NOT NULL,
	listname TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);
