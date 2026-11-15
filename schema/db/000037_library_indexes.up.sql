-- The Library page and the series jobs look up episodes and episode files by
-- series; without these, every lookup scans the whole table.
CREATE INDEX IF NOT EXISTS idx_serie_episodes_serie_id_missing ON serie_episodes(serie_id, missing);
CREATE INDEX IF NOT EXISTS idx_serie_episode_files_serie_id ON serie_episode_files(serie_id);
