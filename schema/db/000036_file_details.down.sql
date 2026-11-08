ALTER TABLE movie_files DROP COLUMN file_size;
ALTER TABLE movie_files DROP COLUMN runtime_seconds;
ALTER TABLE movie_files DROP COLUMN bitrate;
ALTER TABLE movie_files DROP COLUMN languages;

ALTER TABLE serie_episode_files DROP COLUMN file_size;
ALTER TABLE serie_episode_files DROP COLUMN runtime_seconds;
ALTER TABLE serie_episode_files DROP COLUMN bitrate;
ALTER TABLE serie_episode_files DROP COLUMN languages;
