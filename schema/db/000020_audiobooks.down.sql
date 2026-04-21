-- Audiobook Tables Schema Rollback

-- Drop triggers
DROP TRIGGER IF EXISTS tg_audiobook_file_unmatcheds_updated_at;
DROP TRIGGER IF EXISTS tg_audiobook_histories_updated_at;
DROP TRIGGER IF EXISTS tg_audiobook_files_updated_at;
DROP TRIGGER IF EXISTS tg_audiobooks_updated_at;
DROP TRIGGER IF EXISTS tg_dbaudiobook_chapters_updated_at;
DROP TRIGGER IF EXISTS tg_dbaudiobook_authors_updated_at;
DROP TRIGGER IF EXISTS tg_dbaudiobook_narrators_updated_at;
DROP TRIGGER IF EXISTS tg_dbaudiobook_titles_updated_at;
DROP TRIGGER IF EXISTS tg_dbaudiobooks_updated_at;
DROP TRIGGER IF EXISTS tg_dbnarrators_updated_at;

-- Drop indexes
DROP INDEX IF EXISTS idx_audiobook_histories_audiobook_id;
DROP INDEX IF EXISTS idx_audiobook_histories_title;
DROP INDEX IF EXISTS idx_audiobook_files_audiobook_id;
DROP INDEX IF EXISTS idx_audiobook_files_location;
DROP INDEX IF EXISTS idx_audiobooks_dbaudiobook_id;
DROP INDEX IF EXISTS idx_audiobooks_listname;
DROP INDEX IF EXISTS idx_dbaudiobook_titles_dbaudiobook_id;
DROP INDEX IF EXISTS idx_dbaudiobooks_dbbook_id;
DROP INDEX IF EXISTS idx_dbaudiobooks_audible_id;
DROP INDEX IF EXISTS idx_dbaudiobooks_asin;
DROP INDEX IF EXISTS idx_dbaudiobooks_slug;
DROP INDEX IF EXISTS idx_dbaudiobooks_title;
DROP INDEX IF EXISTS idx_dbnarrators_audible_id;
DROP INDEX IF EXISTS idx_dbnarrators_name;

-- Drop tables (reverse order of creation due to foreign keys)
DROP TABLE IF EXISTS audiobook_file_unmatcheds;
DROP TABLE IF EXISTS audiobook_histories;
DROP TABLE IF EXISTS audiobook_files;
DROP TABLE IF EXISTS audiobooks;
DROP TABLE IF EXISTS dbaudiobook_chapters;
DROP TABLE IF EXISTS dbaudiobook_authors;
DROP TABLE IF EXISTS dbaudiobook_narrators;
DROP TABLE IF EXISTS dbaudiobook_titles;
DROP TABLE IF EXISTS dbaudiobooks;
DROP TABLE IF EXISTS dbnarrators;
