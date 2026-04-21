-- Music Tables Schema

-- Artists metadata table
CREATE TABLE `dbartists` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `name` text DEFAULT "",
    `sort_name` text DEFAULT "",
    `musicbrainz_id` text DEFAULT "",
    `discogs_id` text DEFAULT "",
    `spotify_id` text DEFAULT "",
    `artist_type` text DEFAULT "",
    `country` text DEFAULT "",
    `disambiguation` text DEFAULT "",
    `bio` text DEFAULT "",
    `image_url` text DEFAULT "",
    `genres` text DEFAULT "",
    `begin_date` text DEFAULT "",
    `end_date` text DEFAULT ""
);

-- Artist aliases table
CREATE TABLE `dbartist_aliases` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `alias` text DEFAULT "",
    `locale` text DEFAULT "",
    `alias_type` text DEFAULT "",
    `is_primary` numeric DEFAULT 0,
    `dbartist_id` integer DEFAULT 0,
    CONSTRAINT `fk_dbartist_aliases_dbartist` FOREIGN KEY (`dbartist_id`) REFERENCES `dbartists`(`id`) ON DELETE CASCADE
);

-- Albums metadata table
CREATE TABLE `dbalbums` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `title` text DEFAULT "",
    `musicbrainz_release_group_id` text DEFAULT "",
    `musicbrainz_release_id` text DEFAULT "",
    `discogs_master_id` text DEFAULT "",
    `discogs_release_id` text DEFAULT "",
    `spotify_id` text DEFAULT "",
    `upc` text DEFAULT "",
    `release_date` datetime,
    `release_type` text DEFAULT "",
    `format` text DEFAULT "",
    `label` text DEFAULT "",
    `country` text DEFAULT "",
    `total_tracks` integer DEFAULT 0,
    `total_runtime_ms` integer DEFAULT 0,
    `genres` text DEFAULT "",
    `styles` text DEFAULT "",
    `cover_url` text DEFAULT "",
    `year` integer DEFAULT 0,
    `slug` text DEFAULT ""
);

-- Album alternate titles
CREATE TABLE `dbalbum_titles` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `title` text DEFAULT "",
    `slug` text DEFAULT "",
    `region` text DEFAULT "",
    `dbalbum_id` integer DEFAULT 0,
    CONSTRAINT `fk_dbalbum_titles_dbalbum` FOREIGN KEY (`dbalbum_id`) REFERENCES `dbalbums`(`id`) ON DELETE CASCADE
);

-- Album-Artist many-to-many relationship
CREATE TABLE `dbalbum_artists` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `dbalbum_id` integer DEFAULT 0,
    `dbartist_id` integer DEFAULT 0,
    `join_phrase` text DEFAULT "",
    `position` integer DEFAULT 0,
    CONSTRAINT `fk_dbalbum_artists_dbalbum` FOREIGN KEY (`dbalbum_id`) REFERENCES `dbalbums`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_dbalbum_artists_dbartist` FOREIGN KEY (`dbartist_id`) REFERENCES `dbartists`(`id`) ON DELETE CASCADE
);

-- Tracks metadata table
CREATE TABLE `dbtracks` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `dbalbum_id` integer DEFAULT 0,
    `musicbrainz_recording_id` text DEFAULT "",
    `isrc` text DEFAULT "",
    `acoustid` text DEFAULT "",
    `title` text DEFAULT "",
    `disc_number` integer DEFAULT 0,
    `track_number` integer DEFAULT 0,
    `runtime_ms` integer DEFAULT 0,
    `explicit` numeric DEFAULT 0,
    CONSTRAINT `fk_dbtracks_dbalbum` FOREIGN KEY (`dbalbum_id`) REFERENCES `dbalbums`(`id`) ON DELETE CASCADE
);

-- Track-Artist many-to-many relationship (for featured artists)
CREATE TABLE `dbtrack_artists` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `dbtrack_id` integer DEFAULT 0,
    `dbartist_id` integer DEFAULT 0,
    `role` text DEFAULT "",
    `position` integer DEFAULT 0,
    CONSTRAINT `fk_dbtrack_artists_dbtrack` FOREIGN KEY (`dbtrack_id`) REFERENCES `dbtracks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_dbtrack_artists_dbartist` FOREIGN KEY (`dbartist_id`) REFERENCES `dbartists`(`id`) ON DELETE CASCADE
);

-- Tracked artists (user library)
CREATE TABLE `artists` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `listname` text DEFAULT "",
    `dbartist_id` integer DEFAULT 0,
    `track_mode` text DEFAULT "",
    `dont_search` numeric DEFAULT 0,
    CONSTRAINT `fk_artists_dbartist` FOREIGN KEY (`dbartist_id`) REFERENCES `dbartists`(`id`) ON DELETE CASCADE
);

-- Tracked albums (user library)
CREATE TABLE `albums` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `lastscan` datetime,
    `blacklisted` numeric DEFAULT 0,
    `quality_reached` numeric DEFAULT 0,
    `quality_profile` text DEFAULT "",
    `missing` numeric DEFAULT 0,
    `dont_upgrade` numeric DEFAULT 0,
    `dont_search` numeric DEFAULT 0,
    `listname` text DEFAULT "",
    `rootpath` text DEFAULT "",
    `dbalbum_id` integer DEFAULT 0,
    `artist_id` integer DEFAULT 0,
    CONSTRAINT `fk_albums_dbalbum` FOREIGN KEY (`dbalbum_id`) REFERENCES `dbalbums`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_albums_artist` FOREIGN KEY (`artist_id`) REFERENCES `artists`(`id`) ON DELETE SET NULL
);

-- Album files
CREATE TABLE `album_files` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `location` text DEFAULT "",
    `filename` text DEFAULT "",
    `extension` text DEFAULT "",
    `format` text DEFAULT "",
    `quality_profile` text DEFAULT "",
    `file_size` integer DEFAULT 0,
    `bitrate` integer DEFAULT 0,
    `sample_rate` integer DEFAULT 0,
    `bit_depth` integer DEFAULT 0,
    `runtime_ms` integer DEFAULT 0,
    `disc_number` integer DEFAULT 0,
    `track_number` integer DEFAULT 0,
    `acoustid` text DEFAULT "",
    `album_id` integer DEFAULT 0,
    `dbalbum_id` integer DEFAULT 0,
    `dbtrack_id` integer DEFAULT 0,
    CONSTRAINT `fk_album_files_album` FOREIGN KEY (`album_id`) REFERENCES `albums`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_album_files_dbalbum` FOREIGN KEY (`dbalbum_id`) REFERENCES `dbalbums`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_album_files_dbtrack` FOREIGN KEY (`dbtrack_id`) REFERENCES `dbtracks`(`id`) ON DELETE SET NULL
);

-- Album download history
CREATE TABLE `album_histories` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `title` text DEFAULT "",
    `url` text DEFAULT "",
    `indexer` text DEFAULT "",
    `type` text DEFAULT "",
    `target` text DEFAULT "",
    `downloaded_at` datetime,
    `blacklisted` numeric DEFAULT 0,
    `quality_profile` text DEFAULT "",
    `album_id` integer DEFAULT 0,
    `dbalbum_id` integer DEFAULT 0,
    CONSTRAINT `fk_album_histories_album` FOREIGN KEY (`album_id`) REFERENCES `albums`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_album_histories_dbalbum` FOREIGN KEY (`dbalbum_id`) REFERENCES `dbalbums`(`id`) ON DELETE CASCADE
);

-- Unmatched album files
CREATE TABLE `album_file_unmatcheds` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `listname` text DEFAULT "",
    `filepath` text DEFAULT "",
    `last_checked` datetime,
    `parsed_data` text DEFAULT ""
);

-- Indexes
CREATE INDEX `idx_dbartists_name` ON `dbartists`(`name`);
CREATE INDEX `idx_dbartists_musicbrainz_id` ON `dbartists`(`musicbrainz_id`);
CREATE INDEX `idx_dbartists_discogs_id` ON `dbartists`(`discogs_id`);
CREATE INDEX `idx_dbartists_spotify_id` ON `dbartists`(`spotify_id`);
CREATE INDEX `idx_dbartist_aliases_dbartist_id` ON `dbartist_aliases`(`dbartist_id`);
CREATE INDEX `idx_dbalbums_title` ON `dbalbums`(`title`);
CREATE INDEX `idx_dbalbums_slug` ON `dbalbums`(`slug`);
CREATE INDEX `idx_dbalbums_musicbrainz_release_group_id` ON `dbalbums`(`musicbrainz_release_group_id`);
CREATE INDEX `idx_dbalbums_discogs_master_id` ON `dbalbums`(`discogs_master_id`);
CREATE INDEX `idx_dbalbums_spotify_id` ON `dbalbums`(`spotify_id`);
CREATE INDEX `idx_dbalbum_titles_dbalbum_id` ON `dbalbum_titles`(`dbalbum_id`);
CREATE INDEX `idx_dbtracks_dbalbum_id` ON `dbtracks`(`dbalbum_id`);
CREATE INDEX `idx_dbtracks_isrc` ON `dbtracks`(`isrc`);
CREATE INDEX `idx_artists_listname` ON `artists`(`listname`);
CREATE INDEX `idx_artists_dbartist_id` ON `artists`(`dbartist_id`);
CREATE INDEX `idx_albums_listname` ON `albums`(`listname`);
CREATE INDEX `idx_albums_dbalbum_id` ON `albums`(`dbalbum_id`);
CREATE INDEX `idx_album_files_location` ON `album_files`(`location`);
CREATE INDEX `idx_album_files_album_id` ON `album_files`(`album_id`);
CREATE INDEX `idx_album_histories_title` ON `album_histories`(`title`);
CREATE INDEX `idx_album_histories_album_id` ON `album_histories`(`album_id`);

-- Triggers for updated_at
CREATE TRIGGER tg_dbartists_updated_at AFTER UPDATE ON dbartists FOR EACH ROW BEGIN UPDATE dbartists SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbartist_aliases_updated_at AFTER UPDATE ON dbartist_aliases FOR EACH ROW BEGIN UPDATE dbartist_aliases SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbalbums_updated_at AFTER UPDATE ON dbalbums FOR EACH ROW BEGIN UPDATE dbalbums SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbalbum_titles_updated_at AFTER UPDATE ON dbalbum_titles FOR EACH ROW BEGIN UPDATE dbalbum_titles SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbalbum_artists_updated_at AFTER UPDATE ON dbalbum_artists FOR EACH ROW BEGIN UPDATE dbalbum_artists SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbtracks_updated_at AFTER UPDATE ON dbtracks FOR EACH ROW BEGIN UPDATE dbtracks SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbtrack_artists_updated_at AFTER UPDATE ON dbtrack_artists FOR EACH ROW BEGIN UPDATE dbtrack_artists SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_artists_updated_at AFTER UPDATE ON artists FOR EACH ROW BEGIN UPDATE artists SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_albums_updated_at AFTER UPDATE ON albums FOR EACH ROW BEGIN UPDATE albums SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_album_files_updated_at AFTER UPDATE ON album_files FOR EACH ROW BEGIN UPDATE album_files SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_album_histories_updated_at AFTER UPDATE ON album_histories FOR EACH ROW BEGIN UPDATE album_histories SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_album_file_unmatcheds_updated_at AFTER UPDATE ON album_file_unmatcheds FOR EACH ROW BEGIN UPDATE album_file_unmatcheds SET updated_at = current_timestamp WHERE id = old.id; END;
