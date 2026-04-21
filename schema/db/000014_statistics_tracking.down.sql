-- Drop statistics tracking tables

DROP INDEX IF EXISTS idx_indexer_performance_statistics_name;
DROP INDEX IF EXISTS idx_indexer_performance_statistics_timestamp;
DROP INDEX IF EXISTS idx_http_request_statistics_path;
DROP INDEX IF EXISTS idx_http_request_statistics_timestamp;
DROP INDEX IF EXISTS idx_configuration_statistics_section;
DROP INDEX IF EXISTS idx_configuration_statistics_timestamp;
DROP INDEX IF EXISTS idx_maintenance_statistics_type;
DROP INDEX IF EXISTS idx_maintenance_statistics_timestamp;
DROP INDEX IF EXISTS idx_error_statistics_type;
DROP INDEX IF EXISTS idx_error_statistics_timestamp;
DROP INDEX IF EXISTS idx_performance_statistics_timestamp;

DROP TABLE IF EXISTS indexer_performance_statistics;
DROP TABLE IF EXISTS http_request_statistics;
DROP TABLE IF EXISTS configuration_statistics;
DROP TABLE IF EXISTS maintenance_statistics;
DROP TABLE IF EXISTS error_statistics;
DROP TABLE IF EXISTS performance_statistics;