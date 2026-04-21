-- Add provider-specific ID columns to dbalbums for Deezer, TheAudioDB, and iTunes fallback lookup.

ALTER TABLE `dbalbums` ADD COLUMN `deezer_id` text NOT NULL DEFAULT '';
ALTER TABLE `dbalbums` ADD COLUMN `theaudiodb_id` text NOT NULL DEFAULT '';
ALTER TABLE `dbalbums` ADD COLUMN `itunes_id` text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS `idx_dbalbums_deezer_id` ON `dbalbums`(`deezer_id`);
CREATE INDEX IF NOT EXISTS `idx_dbalbums_theaudiodb_id` ON `dbalbums`(`theaudiodb_id`);
CREATE INDEX IF NOT EXISTS `idx_dbalbums_itunes_id` ON `dbalbums`(`itunes_id`);
