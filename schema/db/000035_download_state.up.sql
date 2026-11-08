-- Download tracking: which client got the release, its id there, and the
-- state the client and the structure job report (added, downloading,
-- completed, failed, removed, processed, rejected). Rows from before this
-- migration keep an empty state and are not tracked.

ALTER TABLE movie_histories ADD COLUMN download_client text DEFAULT '';
ALTER TABLE movie_histories ADD COLUMN download_id text DEFAULT '';
ALTER TABLE movie_histories ADD COLUMN download_state text DEFAULT '';
ALTER TABLE movie_histories ADD COLUMN download_progress real DEFAULT 0;
ALTER TABLE movie_histories ADD COLUMN download_message text DEFAULT '';
ALTER TABLE movie_histories ADD COLUMN state_changed_at datetime;
CREATE INDEX IF NOT EXISTS idx_movie_histories_download_state ON movie_histories(download_state) WHERE download_state IN ('added', 'downloading', 'completed');

ALTER TABLE serie_episode_histories ADD COLUMN download_client text DEFAULT '';
ALTER TABLE serie_episode_histories ADD COLUMN download_id text DEFAULT '';
ALTER TABLE serie_episode_histories ADD COLUMN download_state text DEFAULT '';
ALTER TABLE serie_episode_histories ADD COLUMN download_progress real DEFAULT 0;
ALTER TABLE serie_episode_histories ADD COLUMN download_message text DEFAULT '';
ALTER TABLE serie_episode_histories ADD COLUMN state_changed_at datetime;
CREATE INDEX IF NOT EXISTS idx_serie_episode_histories_download_state ON serie_episode_histories(download_state) WHERE download_state IN ('added', 'downloading', 'completed');

ALTER TABLE book_histories ADD COLUMN download_client text DEFAULT '';
ALTER TABLE book_histories ADD COLUMN download_id text DEFAULT '';
ALTER TABLE book_histories ADD COLUMN download_state text DEFAULT '';
ALTER TABLE book_histories ADD COLUMN download_progress real DEFAULT 0;
ALTER TABLE book_histories ADD COLUMN download_message text DEFAULT '';
ALTER TABLE book_histories ADD COLUMN state_changed_at datetime;
CREATE INDEX IF NOT EXISTS idx_book_histories_download_state ON book_histories(download_state) WHERE download_state IN ('added', 'downloading', 'completed');

ALTER TABLE audiobook_histories ADD COLUMN download_client text DEFAULT '';
ALTER TABLE audiobook_histories ADD COLUMN download_id text DEFAULT '';
ALTER TABLE audiobook_histories ADD COLUMN download_state text DEFAULT '';
ALTER TABLE audiobook_histories ADD COLUMN download_progress real DEFAULT 0;
ALTER TABLE audiobook_histories ADD COLUMN download_message text DEFAULT '';
ALTER TABLE audiobook_histories ADD COLUMN state_changed_at datetime;
CREATE INDEX IF NOT EXISTS idx_audiobook_histories_download_state ON audiobook_histories(download_state) WHERE download_state IN ('added', 'downloading', 'completed');

ALTER TABLE album_histories ADD COLUMN download_client text DEFAULT '';
ALTER TABLE album_histories ADD COLUMN download_id text DEFAULT '';
ALTER TABLE album_histories ADD COLUMN download_state text DEFAULT '';
ALTER TABLE album_histories ADD COLUMN download_progress real DEFAULT 0;
ALTER TABLE album_histories ADD COLUMN download_message text DEFAULT '';
ALTER TABLE album_histories ADD COLUMN state_changed_at datetime;
CREATE INDEX IF NOT EXISTS idx_album_histories_download_state ON album_histories(download_state) WHERE download_state IN ('added', 'downloading', 'completed');
