-- Titles the list imports of a media group don't add again (deleted with
-- "don't add again"). kind: movie, series, album, audiobook, book; db_id
-- the dbmovies/dbseries/... row; ext_id its IMDb ID, TVDB ID, MusicBrainz
-- release group, ASIN or ISBN, so a re-created row still matches.
CREATE TABLE IF NOT EXISTS import_exclusions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL,
	media_group TEXT NOT NULL,
	db_id INTEGER NOT NULL,
	ext_id TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	UNIQUE (kind, media_group, db_id)
);

-- TMDB collections whose new parts are added to a movie list.
CREATE TABLE IF NOT EXISTS movie_collections (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	collection_id INTEGER NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	listname TEXT NOT NULL,
	quality_profile TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	last_check DATETIME,
	UNIQUE (collection_id, listname)
);

-- One row per play in the Library player, for the viewing statistics.
-- mode: direct, remux, transcode, hls; seconds: time actually played.
CREATE TABLE IF NOT EXISTS play_sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL COLLATE NOCASE,
	kind TEXT NOT NULL,
	file_id INTEGER NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	mode TEXT NOT NULL DEFAULT '',
	quality TEXT NOT NULL DEFAULT '',
	started_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	ended_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	seconds REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_play_sessions_started ON play_sessions (started_at);
