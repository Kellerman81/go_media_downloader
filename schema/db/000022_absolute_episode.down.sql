-- Remove absolute_episode column from dbserie_episodes table
DROP INDEX IF EXISTS idx_dbserie_episodes_absolute;

-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
CREATE TABLE dbserie_episodes_backup AS SELECT id, created_at, updated_at, episode, season, identifier, title, first_aired, overview, poster, dbserie_id FROM dbserie_episodes;

DROP TABLE dbserie_episodes;

CREATE TABLE `dbserie_episodes` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`episode` text DEFAULT "",`season` text DEFAULT "",`identifier` text DEFAULT "",`title` text DEFAULT "",`first_aired` text DEFAULT "",`overview` text DEFAULT "",`poster` text DEFAULT "",`dbserie_id` integer default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_dbserie_episodes_dbserie` FOREIGN KEY (`dbserie_id`) REFERENCES `dbseries`(`id`) ON DELETE CASCADE);

INSERT INTO dbserie_episodes SELECT * FROM dbserie_episodes_backup;

DROP TABLE dbserie_episodes_backup;
