-- settings: the account's web UI settings (player volume, languages,
-- resume positions, library views) as a JSON object.
-- features: what the account may do besides viewing and playing, comma
-- separated: search, request, download, requestdelete. The role pre-sets
-- them; admins may do everything regardless.
-- The built-in admin login keeps its settings in a row named 'admin' that
-- can't log in (the password hash '!' matches no password) and isn't listed.
ALTER TABLE users ADD COLUMN settings TEXT NOT NULL DEFAULT '{}';
ALTER TABLE users ADD COLUMN features TEXT NOT NULL DEFAULT '';
UPDATE users SET features = 'search' WHERE role = 'searchinguser';
UPDATE users SET features = 'request' WHERE role = 'requestinguser';

-- What was played: one row per account and file, updated while it plays.
-- kind: movie, episode, track, audiobook (the player's file kinds);
-- file_id: its files row. position and duration in seconds, completion
-- 0..1 (the furthest position reached).
CREATE TABLE IF NOT EXISTS play_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL COLLATE NOCASE,
	kind TEXT NOT NULL,
	file_id INTEGER NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	position REAL NOT NULL DEFAULT 0,
	duration REAL NOT NULL DEFAULT 0,
	completion REAL NOT NULL DEFAULT 0,
	plays INTEGER NOT NULL DEFAULT 1,
	started_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	updated_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	UNIQUE (username, kind, file_id)
);
CREATE INDEX IF NOT EXISTS idx_play_history_updated ON play_history (updated_at);

-- Files an account reported as wrong (requestdelete): an admin deletes the
-- file (and searches again) or dismisses the report.
-- kind: movie, episode, track, audiobook, book; file_id: its files row.
-- reason: quality, language, content, other.
CREATE TABLE IF NOT EXISTS media_reports (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL,
	kind TEXT NOT NULL,
	file_id INTEGER NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT 'other',
	note TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);
