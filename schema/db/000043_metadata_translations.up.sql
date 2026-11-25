-- Translations of metadata texts: one row per entry, field and language
-- (media_type is the metadata table: dbmovie, dbserie_episode, ...).
-- manual rows were edited by hand and are kept by metadata refreshes.
CREATE TABLE metadata_translations (
	id integer PRIMARY KEY,
	created_at datetime NOT NULL DEFAULT current_timestamp,
	updated_at datetime NOT NULL DEFAULT current_timestamp,
	media_type text NOT NULL,
	media_id integer NOT NULL,
	field text NOT NULL,
	language text NOT NULL,
	translation text NOT NULL DEFAULT '',
	manual numeric NOT NULL DEFAULT 0,
	UNIQUE (media_type, media_id, field, language)
);

CREATE INDEX idx_metadata_translations_language ON metadata_translations (media_type, language, media_id);

CREATE TRIGGER trg_dbmovies_translations AFTER DELETE ON dbmovies
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbmovie' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbseries_translations AFTER DELETE ON dbseries
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbserie' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbserie_episodes_translations AFTER DELETE ON dbserie_episodes
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbserie_episode' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbbooks_translations AFTER DELETE ON dbbooks
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbbook' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbaudiobooks_translations AFTER DELETE ON dbaudiobooks
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbaudiobook' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbbook_series_translations AFTER DELETE ON dbbook_series
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbbook_series' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbartists_translations AFTER DELETE ON dbartists
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbartist' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbauthors_translations AFTER DELETE ON dbauthors
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbauthor' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbnarrators_translations AFTER DELETE ON dbnarrators
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbnarrator' AND media_id = OLD.id;
END;

CREATE TRIGGER trg_dbalbums_translations AFTER DELETE ON dbalbums
BEGIN
	DELETE FROM metadata_translations WHERE media_type = 'dbalbum' AND media_id = OLD.id;
END;
