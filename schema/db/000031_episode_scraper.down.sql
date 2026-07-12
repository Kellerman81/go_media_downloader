-- Remove scraper provenance columns from dbserie_episodes.
ALTER TABLE `dbserie_episodes` DROP COLUMN `scraper_url`;
ALTER TABLE `dbserie_episodes` DROP COLUMN `scraper_id`;
