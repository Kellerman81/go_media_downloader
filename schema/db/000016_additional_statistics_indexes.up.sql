-- Additional performance indexes for statistics optimization

-- Composite indexes for frequently used queries in statistics system
CREATE INDEX IF NOT EXISTS idx_movies_dbmovie_year ON movies(dbmovie_id) WHERE dbmovie_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_series_dbserie_id ON series(dbserie_id) WHERE dbserie_id IS NOT NULL;

-- Indexes for file queries used in statistics
CREATE INDEX IF NOT EXISTS idx_movie_files_created_at ON movie_files(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_serie_episode_files_created_at ON serie_episode_files(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_movie_files_location ON movie_files(location);
CREATE INDEX IF NOT EXISTS idx_serie_episode_files_location ON serie_episode_files(location);

-- Indexes for download history queries
CREATE INDEX IF NOT EXISTS idx_movie_histories_downloaded_at ON movie_histories(downloaded_at DESC) WHERE downloaded_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_serie_episode_histories_downloaded_at ON serie_episode_histories(downloaded_at DESC) WHERE downloaded_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_movie_histories_movie_id_date ON movie_histories(movie_id, downloaded_at DESC);
CREATE INDEX IF NOT EXISTS idx_serie_episode_histories_episode_id_date ON serie_episode_histories(serie_episode_id, downloaded_at DESC);

-- Indexes for metadata completeness queries
CREATE INDEX IF NOT EXISTS idx_dbmovies_poster ON dbmovies(poster) WHERE poster IS NULL OR poster = '';
CREATE INDEX IF NOT EXISTS idx_dbseries_poster ON dbseries(poster) WHERE poster IS NULL OR poster = '';
CREATE INDEX IF NOT EXISTS idx_movie_files_quality ON movie_files(resolution_id, quality_profile);
CREATE INDEX IF NOT EXISTS idx_serie_episode_files_quality ON serie_episode_files(resolution_id, quality_profile);

-- Indexes for outlier detection queries
CREATE INDEX IF NOT EXISTS idx_dbmovies_year_outliers ON dbmovies(year) WHERE year < 1920 OR year > 2027;
CREATE INDEX IF NOT EXISTS idx_dbseries_firstaired_outliers ON dbseries(firstaired) WHERE firstaired < '1920-01-01' OR firstaired > '2027-12-31';

-- Indexes for duplicate detection
CREATE INDEX IF NOT EXISTS idx_dbmovies_imdb_id ON dbmovies(imdb_id) WHERE imdb_id IS NOT NULL AND imdb_id != '';
CREATE INDEX IF NOT EXISTS idx_dbseries_thetvdb_id ON dbseries(thetvdb_id) WHERE thetvdb_id IS NOT NULL AND thetvdb_id != 0;

-- Performance indexes for temporal statistics queries
CREATE INDEX IF NOT EXISTS idx_statistics_daily_range ON performance_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_statistics_weekly_range ON performance_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_error_stats_recent ON error_statistics(timestamp, error_type);

-- Indexes for specific statistics table queries used in recent implementations
CREATE INDEX IF NOT EXISTS idx_maintenance_stats_success ON maintenance_statistics(operation_type, timestamp, success) WHERE success = 1;
CREATE INDEX IF NOT EXISTS idx_config_stats_section_key ON configuration_statistics(config_section, config_key, timestamp);

-- Covering indexes for frequently accessed combinations
CREATE INDEX IF NOT EXISTS idx_movie_files_movie_resolution ON movie_files(movie_id, resolution_id, created_at);
CREATE INDEX IF NOT EXISTS idx_serie_files_episode_resolution ON serie_episode_files(serie_episode_id, resolution_id, created_at);