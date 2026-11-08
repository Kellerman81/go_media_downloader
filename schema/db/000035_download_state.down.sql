DROP INDEX IF EXISTS idx_movie_histories_download_state;
ALTER TABLE movie_histories DROP COLUMN download_client;
ALTER TABLE movie_histories DROP COLUMN download_id;
ALTER TABLE movie_histories DROP COLUMN download_state;
ALTER TABLE movie_histories DROP COLUMN download_progress;
ALTER TABLE movie_histories DROP COLUMN download_message;
ALTER TABLE movie_histories DROP COLUMN state_changed_at;

DROP INDEX IF EXISTS idx_serie_episode_histories_download_state;
ALTER TABLE serie_episode_histories DROP COLUMN download_client;
ALTER TABLE serie_episode_histories DROP COLUMN download_id;
ALTER TABLE serie_episode_histories DROP COLUMN download_state;
ALTER TABLE serie_episode_histories DROP COLUMN download_progress;
ALTER TABLE serie_episode_histories DROP COLUMN download_message;
ALTER TABLE serie_episode_histories DROP COLUMN state_changed_at;

DROP INDEX IF EXISTS idx_book_histories_download_state;
ALTER TABLE book_histories DROP COLUMN download_client;
ALTER TABLE book_histories DROP COLUMN download_id;
ALTER TABLE book_histories DROP COLUMN download_state;
ALTER TABLE book_histories DROP COLUMN download_progress;
ALTER TABLE book_histories DROP COLUMN download_message;
ALTER TABLE book_histories DROP COLUMN state_changed_at;

DROP INDEX IF EXISTS idx_audiobook_histories_download_state;
ALTER TABLE audiobook_histories DROP COLUMN download_client;
ALTER TABLE audiobook_histories DROP COLUMN download_id;
ALTER TABLE audiobook_histories DROP COLUMN download_state;
ALTER TABLE audiobook_histories DROP COLUMN download_progress;
ALTER TABLE audiobook_histories DROP COLUMN download_message;
ALTER TABLE audiobook_histories DROP COLUMN state_changed_at;

DROP INDEX IF EXISTS idx_album_histories_download_state;
ALTER TABLE album_histories DROP COLUMN download_client;
ALTER TABLE album_histories DROP COLUMN download_id;
ALTER TABLE album_histories DROP COLUMN download_state;
ALTER TABLE album_histories DROP COLUMN download_progress;
ALTER TABLE album_histories DROP COLUMN download_message;
ALTER TABLE album_histories DROP COLUMN state_changed_at;
