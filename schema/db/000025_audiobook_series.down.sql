-- Remove series information columns from dbaudiobooks
-- SQLite does not support DROP COLUMN before 3.35.0, so this is a best-effort rollback
ALTER TABLE `dbaudiobooks` DROP COLUMN `series_name`;
ALTER TABLE `dbaudiobooks` DROP COLUMN `series_position`;
