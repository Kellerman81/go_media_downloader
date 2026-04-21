-- Add series information columns to dbaudiobooks
ALTER TABLE `dbaudiobooks` ADD COLUMN `series_name` text DEFAULT "";
ALTER TABLE `dbaudiobooks` ADD COLUMN `series_position` text DEFAULT "";
