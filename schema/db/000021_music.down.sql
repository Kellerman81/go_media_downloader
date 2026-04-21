-- Music Tables Schema Rollback

-- Drop triggers
DROP TRIGGER IF EXISTS tg_album_file_unmatcheds_updated_at;
DROP TRIGGER IF EXISTS tg_album_histories_updated_at;
DROP TRIGGER IF EXISTS tg_album_files_updated_at;
DROP TRIGGER IF EXISTS tg_albums_updated_at;
DROP TRIGGER IF EXISTS tg_artists_updated_at;
DROP TRIGGER IF EXISTS tg_dbtrack_artists_updated_at;
DROP TRIGGER IF EXISTS tg_dbtracks_updated_at;
DROP TRIGGER IF EXISTS tg_dbalbum_artists_updated_at;
DROP TRIGGER IF EXISTS tg_dbalbum_titles_updated_at;
DROP TRIGGER IF EXISTS tg_dbalbums_updated_at;
DROP TRIGGER IF EXISTS tg_dbartist_aliases_updated_at;
DROP TRIGGER IF EXISTS tg_dbartists_updated_at;

-- Drop indexes
DROP INDEX IF EXISTS idx_album_histories_album_id;
DROP INDEX IF EXISTS idx_album_histories_title;
DROP INDEX IF EXISTS idx_album_files_album_id;
DROP INDEX IF EXISTS idx_album_files_location;
DROP INDEX IF EXISTS idx_albums_dbalbum_id;
DROP INDEX IF EXISTS idx_albums_listname;
DROP INDEX IF EXISTS idx_artists_dbartist_id;
DROP INDEX IF EXISTS idx_artists_listname;
DROP INDEX IF EXISTS idx_dbtracks_isrc;
DROP INDEX IF EXISTS idx_dbtracks_dbalbum_id;
DROP INDEX IF EXISTS idx_dbalbum_titles_dbalbum_id;
DROP INDEX IF EXISTS idx_dbalbums_spotify_id;
DROP INDEX IF EXISTS idx_dbalbums_discogs_master_id;
DROP INDEX IF EXISTS idx_dbalbums_musicbrainz_release_group_id;
DROP INDEX IF EXISTS idx_dbalbums_slug;
DROP INDEX IF EXISTS idx_dbalbums_title;
DROP INDEX IF EXISTS idx_dbartist_aliases_dbartist_id;
DROP INDEX IF EXISTS idx_dbartists_spotify_id;
DROP INDEX IF EXISTS idx_dbartists_discogs_id;
DROP INDEX IF EXISTS idx_dbartists_musicbrainz_id;
DROP INDEX IF EXISTS idx_dbartists_name;

-- Drop tables (reverse order of creation due to foreign keys)
DROP TABLE IF EXISTS album_file_unmatcheds;
DROP TABLE IF EXISTS album_histories;
DROP TABLE IF EXISTS album_files;
DROP TABLE IF EXISTS albums;
DROP TABLE IF EXISTS artists;
DROP TABLE IF EXISTS dbtrack_artists;
DROP TABLE IF EXISTS dbtracks;
DROP TABLE IF EXISTS dbalbum_artists;
DROP TABLE IF EXISTS dbalbum_titles;
DROP TABLE IF EXISTS dbalbums;
DROP TABLE IF EXISTS dbartist_aliases;
DROP TABLE IF EXISTS dbartists;
