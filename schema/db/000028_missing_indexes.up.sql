-- Add missing indexes for music and audiobook tables

-- dbalbums: release ID columns searched on every import lookup
CREATE INDEX IF NOT EXISTS `idx_dbalbums_musicbrainz_release_id` ON `dbalbums`(`musicbrainz_release_id`);
CREATE INDEX IF NOT EXISTS `idx_dbalbums_discogs_release_id` ON `dbalbums`(`discogs_release_id`);
CREATE INDEX IF NOT EXISTS `idx_dbalbums_upc` ON `dbalbums`(`upc`);

-- dbalbum_artists: both FK columns used in JOIN / WHERE
CREATE INDEX IF NOT EXISTS `idx_dbalbum_artists_dbalbum_id` ON `dbalbum_artists`(`dbalbum_id`);
CREATE INDEX IF NOT EXISTS `idx_dbalbum_artists_dbartist_id` ON `dbalbum_artists`(`dbartist_id`);

-- dbtrack_artists: both FK columns used in JOIN / WHERE
CREATE INDEX IF NOT EXISTS `idx_dbtrack_artists_dbtrack_id` ON `dbtrack_artists`(`dbtrack_id`);
CREATE INDEX IF NOT EXISTS `idx_dbtrack_artists_dbartist_id` ON `dbtrack_artists`(`dbartist_id`);

-- dbtracks: recording ID searched for AcoustID / ISRC matching
CREATE INDEX IF NOT EXISTS `idx_dbtracks_musicbrainz_recording_id` ON `dbtracks`(`musicbrainz_recording_id`);

-- album_files: dbalbum_id and dbtrack_id used in tag and scan operations
CREATE INDEX IF NOT EXISTS `idx_album_files_dbalbum_id` ON `album_files`(`dbalbum_id`);
CREATE INDEX IF NOT EXISTS `idx_album_files_dbtrack_id` ON `album_files`(`dbtrack_id`);

-- albums: artist_id FK used in JOIN for artist-scoped album lookups
CREATE INDEX IF NOT EXISTS `idx_albums_artist_id` ON `albums`(`artist_id`);

-- album_histories: dbalbum_id FK has no index
CREATE INDEX IF NOT EXISTS `idx_album_histories_dbalbum_id` ON `album_histories`(`dbalbum_id`);

-- dbaudiobook_narrators: both FK columns used in JOIN / WHERE
CREATE INDEX IF NOT EXISTS `idx_dbaudiobook_narrators_dbaudiobook_id` ON `dbaudiobook_narrators`(`dbaudiobook_id`);
CREATE INDEX IF NOT EXISTS `idx_dbaudiobook_narrators_dbnarrator_id` ON `dbaudiobook_narrators`(`dbnarrator_id`);

-- dbaudiobook_authors: both FK columns used in JOIN / WHERE
CREATE INDEX IF NOT EXISTS `idx_dbaudiobook_authors_dbaudiobook_id` ON `dbaudiobook_authors`(`dbaudiobook_id`);
CREATE INDEX IF NOT EXISTS `idx_dbaudiobook_authors_dbauthor_id` ON `dbaudiobook_authors`(`dbauthor_id`);

-- dbaudiobook_chapters: dbaudiobook_id used in chapter lookups per audiobook
CREATE INDEX IF NOT EXISTS `idx_dbaudiobook_chapters_dbaudiobook_id` ON `dbaudiobook_chapters`(`dbaudiobook_id`);

-- audiobook_files: dbaudiobook_id FK used in tag and scan operations
CREATE INDEX IF NOT EXISTS `idx_audiobook_files_dbaudiobook_id` ON `audiobook_files`(`dbaudiobook_id`);

-- audiobooks: FK columns used in queries
CREATE INDEX IF NOT EXISTS `idx_audiobooks_author_id` ON `audiobooks`(`author_id`);
CREATE INDEX IF NOT EXISTS `idx_audiobooks_book_series_id` ON `audiobooks`(`book_series_id`);

-- audiobook_histories: dbaudiobook_id FK has no index
CREATE INDEX IF NOT EXISTS `idx_audiobook_histories_dbaudiobook_id` ON `audiobook_histories`(`dbaudiobook_id`);
