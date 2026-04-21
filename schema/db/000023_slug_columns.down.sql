-- Remove slug columns

DROP INDEX IF EXISTS `idx_dbauthors_slug`;
DROP INDEX IF EXISTS `idx_dbbook_series_slug`;
DROP INDEX IF EXISTS `idx_dbartists_slug`;
DROP INDEX IF EXISTS `idx_dbartist_aliases_slug`;

-- Note: SQLite doesn't support DROP COLUMN directly in older versions
-- These will work in SQLite 3.35.0+ (2021-03-12)
ALTER TABLE `dbauthors` DROP COLUMN `slug`;
ALTER TABLE `dbbook_series` DROP COLUMN `slug`;
ALTER TABLE `dbartists` DROP COLUMN `slug`;
ALTER TABLE `dbartist_aliases` DROP COLUMN `slug`;
