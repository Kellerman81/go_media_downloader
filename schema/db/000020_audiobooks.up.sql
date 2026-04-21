-- Audiobook Tables Schema

-- Narrators metadata table
CREATE TABLE `dbnarrators` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `name` text DEFAULT "",
    `audible_id` text DEFAULT "",
    `bio` text DEFAULT "",
    `image_url` text DEFAULT ""
);

-- Audiobooks metadata table
CREATE TABLE `dbaudiobooks` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `title` text DEFAULT "",
    `asin` text DEFAULT "",
    `audible_id` text DEFAULT "",
    `description` text DEFAULT "",
    `publisher` text DEFAULT "",
    `release_date` datetime,
    `runtime_minutes` integer DEFAULT 0,
    `chapter_count` integer DEFAULT 0,
    `language` text DEFAULT "",
    `abridged` numeric DEFAULT 0,
    `cover_url` text DEFAULT "",
    `sample_url` text DEFAULT "",
    `average_rating` real DEFAULT 0,
    `ratings_count` integer DEFAULT 0,
    `year` integer DEFAULT 0,
    `slug` text DEFAULT "",
    `dbbook_id` integer DEFAULT 0,
    CONSTRAINT `fk_dbaudiobooks_dbbook` FOREIGN KEY (`dbbook_id`) REFERENCES `dbbooks`(`id`) ON DELETE SET NULL
);

-- Audiobook alternate titles
CREATE TABLE `dbaudiobook_titles` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `title` text DEFAULT "",
    `slug` text DEFAULT "",
    `region` text DEFAULT "",
    `dbaudiobook_id` integer DEFAULT 0,
    CONSTRAINT `fk_dbaudiobook_titles_dbaudiobook` FOREIGN KEY (`dbaudiobook_id`) REFERENCES `dbaudiobooks`(`id`) ON DELETE CASCADE
);

-- Audiobook-Narrator many-to-many relationship
CREATE TABLE `dbaudiobook_narrators` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `dbaudiobook_id` integer DEFAULT 0,
    `dbnarrator_id` integer DEFAULT 0,
    `position` integer DEFAULT 0,
    CONSTRAINT `fk_dbaudiobook_narrators_dbaudiobook` FOREIGN KEY (`dbaudiobook_id`) REFERENCES `dbaudiobooks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_dbaudiobook_narrators_dbnarrator` FOREIGN KEY (`dbnarrator_id`) REFERENCES `dbnarrators`(`id`) ON DELETE CASCADE
);

-- Audiobook-Author many-to-many relationship
CREATE TABLE `dbaudiobook_authors` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `dbaudiobook_id` integer DEFAULT 0,
    `dbauthor_id` integer DEFAULT 0,
    `role` text DEFAULT "",
    `position` integer DEFAULT 0,
    CONSTRAINT `fk_dbaudiobook_authors_dbaudiobook` FOREIGN KEY (`dbaudiobook_id`) REFERENCES `dbaudiobooks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_dbaudiobook_authors_dbauthor` FOREIGN KEY (`dbauthor_id`) REFERENCES `dbauthors`(`id`) ON DELETE CASCADE
);

-- Audiobook chapters
CREATE TABLE `dbaudiobook_chapters` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `dbaudiobook_id` integer DEFAULT 0,
    `title` text DEFAULT "",
    `chapter_number` integer DEFAULT 0,
    `position` integer DEFAULT 0,
    `start_time_ms` integer DEFAULT 0,
    `end_time_ms` integer DEFAULT 0,
    `runtime_ms` integer DEFAULT 0,
    CONSTRAINT `fk_dbaudiobook_chapters_dbaudiobook` FOREIGN KEY (`dbaudiobook_id`) REFERENCES `dbaudiobooks`(`id`) ON DELETE CASCADE
);

-- Tracked audiobooks (user library)
CREATE TABLE `audiobooks` (
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
    `dbaudiobook_id` integer DEFAULT 0,
    `author_id` integer DEFAULT 0,
    `book_series_id` integer DEFAULT 0,
    CONSTRAINT `fk_audiobooks_dbaudiobook` FOREIGN KEY (`dbaudiobook_id`) REFERENCES `dbaudiobooks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_audiobooks_author` FOREIGN KEY (`author_id`) REFERENCES `authors`(`id`) ON DELETE SET NULL,
    CONSTRAINT `fk_audiobooks_book_series` FOREIGN KEY (`book_series_id`) REFERENCES `book_series`(`id`) ON DELETE SET NULL
);

-- Audiobook files
CREATE TABLE `audiobook_files` (
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
    `runtime_ms` integer DEFAULT 0,
    `track_number` integer DEFAULT 0,
    `disc_number` integer DEFAULT 0,
    `audiobook_id` integer DEFAULT 0,
    `dbaudiobook_id` integer DEFAULT 0,
    CONSTRAINT `fk_audiobook_files_audiobook` FOREIGN KEY (`audiobook_id`) REFERENCES `audiobooks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_audiobook_files_dbaudiobook` FOREIGN KEY (`dbaudiobook_id`) REFERENCES `dbaudiobooks`(`id`) ON DELETE CASCADE
);

-- Audiobook download history
CREATE TABLE `audiobook_histories` (
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
    `audiobook_id` integer DEFAULT 0,
    `dbaudiobook_id` integer DEFAULT 0,
    CONSTRAINT `fk_audiobook_histories_audiobook` FOREIGN KEY (`audiobook_id`) REFERENCES `audiobooks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_audiobook_histories_dbaudiobook` FOREIGN KEY (`dbaudiobook_id`) REFERENCES `dbaudiobooks`(`id`) ON DELETE CASCADE
);

-- Unmatched audiobook files
CREATE TABLE `audiobook_file_unmatcheds` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `listname` text DEFAULT "",
    `filepath` text DEFAULT "",
    `last_checked` datetime,
    `parsed_data` text DEFAULT ""
);

-- Indexes
CREATE INDEX `idx_dbnarrators_name` ON `dbnarrators`(`name`);
CREATE INDEX `idx_dbnarrators_audible_id` ON `dbnarrators`(`audible_id`);
CREATE INDEX `idx_dbaudiobooks_title` ON `dbaudiobooks`(`title`);
CREATE INDEX `idx_dbaudiobooks_slug` ON `dbaudiobooks`(`slug`);
CREATE INDEX `idx_dbaudiobooks_asin` ON `dbaudiobooks`(`asin`);
CREATE INDEX `idx_dbaudiobooks_audible_id` ON `dbaudiobooks`(`audible_id`);
CREATE INDEX `idx_dbaudiobooks_dbbook_id` ON `dbaudiobooks`(`dbbook_id`);
CREATE INDEX `idx_dbaudiobook_titles_dbaudiobook_id` ON `dbaudiobook_titles`(`dbaudiobook_id`);
CREATE INDEX `idx_audiobooks_listname` ON `audiobooks`(`listname`);
CREATE INDEX `idx_audiobooks_dbaudiobook_id` ON `audiobooks`(`dbaudiobook_id`);
CREATE INDEX `idx_audiobook_files_location` ON `audiobook_files`(`location`);
CREATE INDEX `idx_audiobook_files_audiobook_id` ON `audiobook_files`(`audiobook_id`);
CREATE INDEX `idx_audiobook_histories_title` ON `audiobook_histories`(`title`);
CREATE INDEX `idx_audiobook_histories_audiobook_id` ON `audiobook_histories`(`audiobook_id`);

-- Triggers for updated_at
CREATE TRIGGER tg_dbnarrators_updated_at AFTER UPDATE ON dbnarrators FOR EACH ROW BEGIN UPDATE dbnarrators SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbaudiobooks_updated_at AFTER UPDATE ON dbaudiobooks FOR EACH ROW BEGIN UPDATE dbaudiobooks SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbaudiobook_titles_updated_at AFTER UPDATE ON dbaudiobook_titles FOR EACH ROW BEGIN UPDATE dbaudiobook_titles SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbaudiobook_narrators_updated_at AFTER UPDATE ON dbaudiobook_narrators FOR EACH ROW BEGIN UPDATE dbaudiobook_narrators SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbaudiobook_authors_updated_at AFTER UPDATE ON dbaudiobook_authors FOR EACH ROW BEGIN UPDATE dbaudiobook_authors SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbaudiobook_chapters_updated_at AFTER UPDATE ON dbaudiobook_chapters FOR EACH ROW BEGIN UPDATE dbaudiobook_chapters SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_audiobooks_updated_at AFTER UPDATE ON audiobooks FOR EACH ROW BEGIN UPDATE audiobooks SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_audiobook_files_updated_at AFTER UPDATE ON audiobook_files FOR EACH ROW BEGIN UPDATE audiobook_files SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_audiobook_histories_updated_at AFTER UPDATE ON audiobook_histories FOR EACH ROW BEGIN UPDATE audiobook_histories SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_audiobook_file_unmatcheds_updated_at AFTER UPDATE ON audiobook_file_unmatcheds FOR EACH ROW BEGIN UPDATE audiobook_file_unmatcheds SET updated_at = current_timestamp WHERE id = old.id; END;
