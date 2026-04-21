-- Add series information column to dbalbums
ALTER TABLE `dbalbums` ADD COLUMN `series_name` text DEFAULT "";
