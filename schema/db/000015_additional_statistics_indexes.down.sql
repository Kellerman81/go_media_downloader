-- Drop additional performance indexes for statistics

-- Drop composite indexes
DROP INDEX IF EXISTS idx_movies_dbmovie_year;
DROP INDEX IF EXISTS idx_series_dbserie_id;

-- Drop file indexes
DROP INDEX IF EXISTS idx_movie_files_created_at;
DROP INDEX IF EXISTS idx_serie_episode_files_created_at;
DROP INDEX IF EXISTS idx_movie_files_location;
DROP INDEX IF EXISTS idx_serie_episode_files_location;

-- Drop download history indexes
DROP INDEX IF EXISTS idx_movie_histories_downloaded_at;
DROP INDEX IF EXISTS idx_serie_episode_histories_downloaded_at;
DROP INDEX IF EXISTS idx_movie_histories_movie_id_date;
DROP INDEX IF EXISTS idx_serie_episode_histories_episode_id_date;

-- Drop metadata indexes
DROP INDEX IF EXISTS idx_dbmovies_poster;
DROP INDEX IF EXISTS idx_dbseries_poster;
DROP INDEX IF EXISTS idx_movie_files_quality;
DROP INDEX IF EXISTS idx_serie_episode_files_quality;

-- Drop outlier detection indexes
DROP INDEX IF EXISTS idx_dbmovies_year_outliers;
DROP INDEX IF EXISTS idx_dbseries_firstaired_outliers;

-- Drop duplicate detection indexes
DROP INDEX IF EXISTS idx_dbmovies_imdb_id;
DROP INDEX IF EXISTS idx_dbseries_thetvdb_id;

-- Drop temporal statistics indexes
DROP INDEX IF EXISTS idx_statistics_daily_range;
DROP INDEX IF EXISTS idx_statistics_weekly_range;
DROP INDEX IF EXISTS idx_error_stats_recent;

-- Drop specific statistics indexes
DROP INDEX IF EXISTS idx_maintenance_stats_success;
DROP INDEX IF EXISTS idx_config_stats_section_key;

-- Drop covering indexes
DROP INDEX IF EXISTS idx_movie_files_movie_resolution;
DROP INDEX IF EXISTS idx_serie_files_episode_resolution;