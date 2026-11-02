-- Drop statistics tables nothing has written to since 2025 (their collectors
-- were removed) plus the empty placeholder table "stat". Their indexes go
-- with them.
DROP TABLE IF EXISTS historical_statistics;
DROP TABLE IF EXISTS http_request_statistics;
DROP TABLE IF EXISTS indexer_performance_statistics;
DROP TABLE IF EXISTS storage_usage_history;
DROP TABLE IF EXISTS stat;

-- The statistics page aggregates job runs and downloads by date.
CREATE INDEX IF NOT EXISTS idx_job_histories_created_at ON job_histories(created_at);
CREATE INDEX IF NOT EXISTS idx_album_histories_downloaded_at ON album_histories(downloaded_at);
CREATE INDEX IF NOT EXISTS idx_audiobook_histories_downloaded_at ON audiobook_histories(downloaded_at);
