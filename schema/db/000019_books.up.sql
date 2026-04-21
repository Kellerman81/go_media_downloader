-- Book Tables Schema

-- Authors metadata table
CREATE TABLE `dbauthors` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `name` text DEFAULT "",
    `aliases` text DEFAULT "",
    `bio` text DEFAULT "",
    `birth_date` text DEFAULT "",
    `death_date` text DEFAULT "",
    `goodreads_id` text DEFAULT "",
    `openlibrary_id` text DEFAULT "",
    `website` text DEFAULT "",
    `image_url` text DEFAULT ""
);

-- Book series metadata table
CREATE TABLE `dbbook_series` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `name` text DEFAULT "",
    `description` text DEFAULT "",
    `goodreads_id` text DEFAULT "",
    `openlibrary_id` text DEFAULT ""
);

-- Books metadata table
CREATE TABLE `dbbooks` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `title` text DEFAULT "",
    `original_title` text DEFAULT "",
    `isbn_13` text DEFAULT "",
    `isbn_10` text DEFAULT "",
    `asin` text DEFAULT "",
    `openlibrary_id` text DEFAULT "",
    `goodreads_id` text DEFAULT "",
    `description` text DEFAULT "",
    `publisher` text DEFAULT "",
    `publish_date` datetime,
    `page_count` integer DEFAULT 0,
    `language` text DEFAULT "",
    `genres` text DEFAULT "",
    `cover_url` text DEFAULT "",
    `dbauthor_id` integer DEFAULT 0,
    `dbbook_series_id` integer DEFAULT 0,
    `series_position` text DEFAULT "",
    `average_rating` real DEFAULT 0,
    `ratings_count` integer DEFAULT 0,
    `year` integer DEFAULT 0,
    `slug` text DEFAULT "",
    CONSTRAINT `fk_dbbooks_dbauthor` FOREIGN KEY (`dbauthor_id`) REFERENCES `dbauthors`(`id`) ON DELETE SET NULL,
    CONSTRAINT `fk_dbbooks_dbbook_series` FOREIGN KEY (`dbbook_series_id`) REFERENCES `dbbook_series`(`id`) ON DELETE SET NULL
);

-- Book alternate titles
CREATE TABLE `dbbook_titles` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `title` text DEFAULT "",
    `slug` text DEFAULT "",
    `region` text DEFAULT "",
    `dbbook_id` integer DEFAULT 0,
    CONSTRAINT `fk_dbbook_titles_dbbook` FOREIGN KEY (`dbbook_id`) REFERENCES `dbbooks`(`id`) ON DELETE CASCADE
);

-- Book-Author many-to-many relationship
CREATE TABLE `dbbook_authors` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `dbbook_id` integer DEFAULT 0,
    `dbauthor_id` integer DEFAULT 0,
    `role` text DEFAULT "",
    `position` integer DEFAULT 0,
    CONSTRAINT `fk_dbbook_authors_dbbook` FOREIGN KEY (`dbbook_id`) REFERENCES `dbbooks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_dbbook_authors_dbauthor` FOREIGN KEY (`dbauthor_id`) REFERENCES `dbauthors`(`id`) ON DELETE CASCADE
);

-- Tracked authors (user library)
CREATE TABLE `authors` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `listname` text DEFAULT "",
    `dbauthor_id` integer DEFAULT 0,
    `track_mode` text DEFAULT "",
    `dont_search` numeric DEFAULT 0,
    CONSTRAINT `fk_authors_dbauthor` FOREIGN KEY (`dbauthor_id`) REFERENCES `dbauthors`(`id`) ON DELETE CASCADE
);

-- Tracked book series (user library)
CREATE TABLE `book_series` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `listname` text DEFAULT "",
    `dbbook_series_id` integer DEFAULT 0,
    `author_id` integer DEFAULT 0,
    `dont_search` numeric DEFAULT 0,
    CONSTRAINT `fk_book_series_dbbook_series` FOREIGN KEY (`dbbook_series_id`) REFERENCES `dbbook_series`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_book_series_author` FOREIGN KEY (`author_id`) REFERENCES `authors`(`id`) ON DELETE SET NULL
);

-- Tracked books (user library)
CREATE TABLE `books` (
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
    `dbbook_id` integer DEFAULT 0,
    `book_series_id` integer DEFAULT 0,
    `author_id` integer DEFAULT 0,
    CONSTRAINT `fk_books_dbbook` FOREIGN KEY (`dbbook_id`) REFERENCES `dbbooks`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_books_book_series` FOREIGN KEY (`book_series_id`) REFERENCES `book_series`(`id`) ON DELETE SET NULL,
    CONSTRAINT `fk_books_author` FOREIGN KEY (`author_id`) REFERENCES `authors`(`id`) ON DELETE SET NULL
);

-- Book files
CREATE TABLE `book_files` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `location` text DEFAULT "",
    `filename` text DEFAULT "",
    `extension` text DEFAULT "",
    `format` text DEFAULT "",
    `quality_profile` text DEFAULT "",
    `file_size` integer DEFAULT 0,
    `book_id` integer DEFAULT 0,
    `dbbook_id` integer DEFAULT 0,
    CONSTRAINT `fk_book_files_book` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_book_files_dbbook` FOREIGN KEY (`dbbook_id`) REFERENCES `dbbooks`(`id`) ON DELETE CASCADE
);

-- Book download history
CREATE TABLE `book_histories` (
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
    `book_id` integer DEFAULT 0,
    `dbbook_id` integer DEFAULT 0,
    CONSTRAINT `fk_book_histories_book` FOREIGN KEY (`book_id`) REFERENCES `books`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_book_histories_dbbook` FOREIGN KEY (`dbbook_id`) REFERENCES `dbbooks`(`id`) ON DELETE CASCADE
);

-- Unmatched book files
CREATE TABLE `book_file_unmatcheds` (
    `id` integer PRIMARY KEY,
    `created_at` datetime NOT NULL DEFAULT current_timestamp,
    `updated_at` datetime NOT NULL DEFAULT current_timestamp,
    `listname` text DEFAULT "",
    `filepath` text DEFAULT "",
    `last_checked` datetime,
    `parsed_data` text DEFAULT ""
);

-- Indexes
CREATE INDEX `idx_dbauthors_name` ON `dbauthors`(`name`);
CREATE INDEX `idx_dbauthors_goodreads_id` ON `dbauthors`(`goodreads_id`);
CREATE INDEX `idx_dbbooks_title` ON `dbbooks`(`title`);
CREATE INDEX `idx_dbbooks_slug` ON `dbbooks`(`slug`);
CREATE INDEX `idx_dbbooks_isbn_13` ON `dbbooks`(`isbn_13`);
CREATE INDEX `idx_dbbooks_isbn_10` ON `dbbooks`(`isbn_10`);
CREATE INDEX `idx_dbbooks_asin` ON `dbbooks`(`asin`);
CREATE INDEX `idx_dbbooks_dbauthor_id` ON `dbbooks`(`dbauthor_id`);
CREATE INDEX `idx_dbbook_titles_dbbook_id` ON `dbbook_titles`(`dbbook_id`);
CREATE INDEX `idx_books_listname` ON `books`(`listname`);
CREATE INDEX `idx_books_dbbook_id` ON `books`(`dbbook_id`);
CREATE INDEX `idx_book_files_location` ON `book_files`(`location`);
CREATE INDEX `idx_book_files_book_id` ON `book_files`(`book_id`);
CREATE INDEX `idx_book_histories_title` ON `book_histories`(`title`);
CREATE INDEX `idx_book_histories_book_id` ON `book_histories`(`book_id`);

-- Triggers for updated_at
CREATE TRIGGER tg_dbauthors_updated_at AFTER UPDATE ON dbauthors FOR EACH ROW BEGIN UPDATE dbauthors SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbbook_series_updated_at AFTER UPDATE ON dbbook_series FOR EACH ROW BEGIN UPDATE dbbook_series SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbbooks_updated_at AFTER UPDATE ON dbbooks FOR EACH ROW BEGIN UPDATE dbbooks SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbbook_titles_updated_at AFTER UPDATE ON dbbook_titles FOR EACH ROW BEGIN UPDATE dbbook_titles SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_dbbook_authors_updated_at AFTER UPDATE ON dbbook_authors FOR EACH ROW BEGIN UPDATE dbbook_authors SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_authors_updated_at AFTER UPDATE ON authors FOR EACH ROW BEGIN UPDATE authors SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_book_series_updated_at AFTER UPDATE ON book_series FOR EACH ROW BEGIN UPDATE book_series SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_books_updated_at AFTER UPDATE ON books FOR EACH ROW BEGIN UPDATE books SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_book_files_updated_at AFTER UPDATE ON book_files FOR EACH ROW BEGIN UPDATE book_files SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_book_histories_updated_at AFTER UPDATE ON book_histories FOR EACH ROW BEGIN UPDATE book_histories SET updated_at = current_timestamp WHERE id = old.id; END;
CREATE TRIGGER tg_book_file_unmatcheds_updated_at AFTER UPDATE ON book_file_unmatcheds FOR EACH ROW BEGIN UPDATE book_file_unmatcheds SET updated_at = current_timestamp WHERE id = old.id; END;
