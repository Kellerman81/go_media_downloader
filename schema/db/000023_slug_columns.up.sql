-- Add slug columns for normalized name searching

-- dbauthors (books/audiobooks)
ALTER TABLE `dbauthors` ADD COLUMN `slug` text DEFAULT "";
CREATE INDEX IF NOT EXISTS `idx_dbauthors_slug` ON `dbauthors` (`slug`);

-- dbbook_series
ALTER TABLE `dbbook_series` ADD COLUMN `slug` text DEFAULT "";
CREATE INDEX IF NOT EXISTS `idx_dbbook_series_slug` ON `dbbook_series` (`slug`);

-- dbartists (music)
ALTER TABLE `dbartists` ADD COLUMN `slug` text DEFAULT "";
CREATE INDEX IF NOT EXISTS `idx_dbartists_slug` ON `dbartists` (`slug`);

-- dbartist_aliases
ALTER TABLE `dbartist_aliases` ADD COLUMN `slug` text DEFAULT "";
CREATE INDEX IF NOT EXISTS `idx_dbartist_aliases_slug` ON `dbartist_aliases` (`slug`);
