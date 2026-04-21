-- Remove unused statistics tables that have never been populated

-- Drop tables that are empty and not actively used
DROP TABLE IF EXISTS performance_statistics;
DROP TABLE IF EXISTS error_statistics;
DROP TABLE IF EXISTS maintenance_statistics;
DROP TABLE IF EXISTS configuration_statistics;

-- Drop related indexes
DROP INDEX IF EXISTS idx_performance_statistics_timestamp;
DROP INDEX IF EXISTS idx_error_statistics_timestamp;
DROP INDEX IF EXISTS idx_error_statistics_type;
DROP INDEX IF EXISTS idx_maintenance_statistics_timestamp;
DROP INDEX IF EXISTS idx_maintenance_statistics_type;
DROP INDEX IF EXISTS idx_configuration_statistics_timestamp;
DROP INDEX IF EXISTS idx_configuration_statistics_section;

-- Also remove related indexes from 000015
DROP INDEX IF EXISTS idx_statistics_daily_range;
DROP INDEX IF EXISTS idx_statistics_weekly_range;
DROP INDEX IF EXISTS idx_error_stats_recent;
DROP INDEX IF EXISTS idx_maintenance_stats_success;
DROP INDEX IF EXISTS idx_config_stats_section_key;