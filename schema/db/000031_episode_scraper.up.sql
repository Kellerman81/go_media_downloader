-- Add scraper provenance columns to dbserie_episodes so missing episode
-- metadata can later be re-scraped from its original source. scraper_id records
-- which scraper/source provided the episode, scraper_url the page it came from.
ALTER TABLE `dbserie_episodes` ADD COLUMN `scraper_id` text NOT NULL DEFAULT '';
ALTER TABLE `dbserie_episodes` ADD COLUMN `scraper_url` text NOT NULL DEFAULT '';
