-- File details of organized and scanned video files: size, runtime, overall
-- bitrate and audio languages. Rows from before stay 0 / empty until the
-- file is scanned again.

ALTER TABLE movie_files ADD COLUMN file_size integer DEFAULT 0;
ALTER TABLE movie_files ADD COLUMN runtime_seconds integer DEFAULT 0;
ALTER TABLE movie_files ADD COLUMN bitrate integer DEFAULT 0;
ALTER TABLE movie_files ADD COLUMN languages text DEFAULT '';

ALTER TABLE serie_episode_files ADD COLUMN file_size integer DEFAULT 0;
ALTER TABLE serie_episode_files ADD COLUMN runtime_seconds integer DEFAULT 0;
ALTER TABLE serie_episode_files ADD COLUMN bitrate integer DEFAULT 0;
ALTER TABLE serie_episode_files ADD COLUMN languages text DEFAULT '';
