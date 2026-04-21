-- Book Tables Schema Rollback

-- Drop triggers
DROP TRIGGER IF EXISTS tg_book_file_unmatcheds_updated_at;
DROP TRIGGER IF EXISTS tg_book_histories_updated_at;
DROP TRIGGER IF EXISTS tg_book_files_updated_at;
DROP TRIGGER IF EXISTS tg_books_updated_at;
DROP TRIGGER IF EXISTS tg_book_series_updated_at;
DROP TRIGGER IF EXISTS tg_authors_updated_at;
DROP TRIGGER IF EXISTS tg_dbbook_authors_updated_at;
DROP TRIGGER IF EXISTS tg_dbbook_titles_updated_at;
DROP TRIGGER IF EXISTS tg_dbbooks_updated_at;
DROP TRIGGER IF EXISTS tg_dbbook_series_updated_at;
DROP TRIGGER IF EXISTS tg_dbauthors_updated_at;

-- Drop indexes
DROP INDEX IF EXISTS idx_book_histories_book_id;
DROP INDEX IF EXISTS idx_book_histories_title;
DROP INDEX IF EXISTS idx_book_files_book_id;
DROP INDEX IF EXISTS idx_book_files_location;
DROP INDEX IF EXISTS idx_books_dbbook_id;
DROP INDEX IF EXISTS idx_books_listname;
DROP INDEX IF EXISTS idx_dbbook_titles_dbbook_id;
DROP INDEX IF EXISTS idx_dbbooks_dbauthor_id;
DROP INDEX IF EXISTS idx_dbbooks_asin;
DROP INDEX IF EXISTS idx_dbbooks_isbn_10;
DROP INDEX IF EXISTS idx_dbbooks_isbn_13;
DROP INDEX IF EXISTS idx_dbbooks_slug;
DROP INDEX IF EXISTS idx_dbbooks_title;
DROP INDEX IF EXISTS idx_dbauthors_goodreads_id;
DROP INDEX IF EXISTS idx_dbauthors_name;

-- Drop tables (reverse order of creation due to foreign keys)
DROP TABLE IF EXISTS book_file_unmatcheds;
DROP TABLE IF EXISTS book_histories;
DROP TABLE IF EXISTS book_files;
DROP TABLE IF EXISTS books;
DROP TABLE IF EXISTS book_series;
DROP TABLE IF EXISTS authors;
DROP TABLE IF EXISTS dbbook_authors;
DROP TABLE IF EXISTS dbbook_titles;
DROP TABLE IF EXISTS dbbooks;
DROP TABLE IF EXISTS dbbook_series;
DROP TABLE IF EXISTS dbauthors;
