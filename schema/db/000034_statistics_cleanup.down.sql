DROP INDEX IF EXISTS idx_job_histories_created_at;
DROP INDEX IF EXISTS idx_album_histories_downloaded_at;
DROP INDEX IF EXISTS idx_audiobook_histories_downloaded_at;

-- Recreate the dropped tables empty; their data is not restored.
CREATE TABLE IF NOT EXISTS historical_statistics (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	metric TEXT NOT NULL,
	value REAL NOT NULL,
	metadata TEXT DEFAULT '{}',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_metric_timestamp ON historical_statistics (metric, timestamp);
CREATE INDEX IF NOT EXISTS idx_timestamp ON historical_statistics (timestamp);

CREATE TABLE IF NOT EXISTS http_request_statistics (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
	timestamp TIMESTAMP NOT NULL,
	method VARCHAR(10) NOT NULL,
	path VARCHAR(500) NOT NULL,
	status_code INTEGER NOT NULL,
	response_time_ms INTEGER NOT NULL,
	request_size_bytes INTEGER,
	response_size_bytes INTEGER,
	user_agent VARCHAR(500),
	ip_address VARCHAR(45),
	component VARCHAR(50),
	endpoint_category VARCHAR(100),
	error_message TEXT
);
CREATE INDEX IF NOT EXISTS idx_http_request_statistics_timestamp ON http_request_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_http_request_statistics_path ON http_request_statistics(path, timestamp);

CREATE TABLE IF NOT EXISTS indexer_performance_statistics (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
	timestamp TIMESTAMP NOT NULL,
	indexer_name VARCHAR(100) NOT NULL,
	indexer_type VARCHAR(50),
	indexer_url VARCHAR(500),
	response_time_ms INTEGER,
	success BOOLEAN DEFAULT TRUE,
	error_type VARCHAR(100),
	error_message TEXT,
	search_query VARCHAR(500),
	results_count INTEGER DEFAULT 0,
	results_filtered INTEGER DEFAULT 0,
	rate_limited BOOLEAN DEFAULT FALSE,
	retry_count INTEGER DEFAULT 0,
	connection_quality VARCHAR(20),
	dns_resolution_time_ms INTEGER,
	ssl_handshake_time_ms INTEGER
);
CREATE INDEX IF NOT EXISTS idx_indexer_performance_statistics_timestamp ON indexer_performance_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_indexer_performance_statistics_name ON indexer_performance_statistics(indexer_name, timestamp);

CREATE TABLE IF NOT EXISTS storage_usage_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
	timestamp TIMESTAMP NOT NULL,
	path_name VARCHAR(100) NOT NULL,
	path_location VARCHAR(500) NOT NULL,
	total_space INTEGER NOT NULL,
	used_space INTEGER NOT NULL,
	free_space INTEGER NOT NULL,
	usage_percent REAL NOT NULL,
	file_count INTEGER DEFAULT 0,
	folder_count INTEGER DEFAULT 0,
	bytes_added_since_last INTEGER DEFAULT 0,
	files_added_since_last INTEGER DEFAULT 0,
	accessible BOOLEAN DEFAULT TRUE,
	error_message TEXT,
	file_system_type VARCHAR(50),
	mount_point VARCHAR(500)
);
CREATE INDEX IF NOT EXISTS idx_storage_usage_history_timestamp ON storage_usage_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_storage_usage_history_path ON storage_usage_history(path_name, path_location, timestamp);
CREATE INDEX IF NOT EXISTS idx_storage_usage_history_recent ON storage_usage_history(path_name, timestamp DESC);

CREATE TABLE IF NOT EXISTS "stat" (
	"id" INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT)
);
