-- Recreate statistics tables if needed (rollback for removal)

-- Performance statistics tracking
CREATE TABLE IF NOT EXISTS performance_statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    
    -- System metrics
    cpu_usage REAL,
    memory_usage REAL,
    disk_usage REAL,
    network_io_bytes INTEGER,
    
    -- Database performance
    db_queries_per_second REAL,
    db_avg_query_time REAL,
    db_active_connections INTEGER,
    
    -- Application metrics
    active_workers INTEGER,
    queued_jobs INTEGER,
    processed_jobs INTEGER,
    failed_jobs INTEGER,
    
    -- HTTP/API metrics
    http_requests_per_second REAL,
    http_avg_response_time REAL,
    http_error_rate REAL,
    
    -- Indexer performance
    indexer_response_time REAL,
    indexer_success_rate REAL,
    indexer_timeout_count INTEGER,
    
    -- Media processing metrics
    media_scan_time REAL,
    media_match_rate REAL,
    download_speed REAL
);

-- Error statistics tracking
CREATE TABLE IF NOT EXISTS error_statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    
    -- Error categorization
    error_type VARCHAR(50) NOT NULL,
    error_category VARCHAR(50) NOT NULL,
    error_source VARCHAR(100) NOT NULL,
    error_message TEXT,
    error_count INTEGER DEFAULT 1,
    
    -- Context information
    media_type VARCHAR(20),
    indexer_name VARCHAR(100),
    component VARCHAR(50),
    severity VARCHAR(20),
    
    -- Resolution tracking
    resolved BOOLEAN DEFAULT FALSE,
    resolution_time TIMESTAMP,
    resolution_method VARCHAR(100)
);

-- Maintenance statistics tracking
CREATE TABLE IF NOT EXISTS maintenance_statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    
    -- Maintenance operations
    operation_type VARCHAR(50) NOT NULL,
    operation_category VARCHAR(50) NOT NULL,
    duration_seconds INTEGER,
    success BOOLEAN DEFAULT TRUE,
    
    -- Database maintenance
    database_vacuum_time INTEGER,
    database_size_before INTEGER,
    database_size_after INTEGER,
    
    -- File system maintenance
    files_cleaned INTEGER DEFAULT 0,
    disk_space_freed INTEGER DEFAULT 0,
    
    -- Cache maintenance
    cache_entries_cleared INTEGER DEFAULT 0,
    cache_hit_rate_before REAL,
    cache_hit_rate_after REAL,
    
    -- Log maintenance
    logs_rotated INTEGER DEFAULT 0,
    logs_compressed INTEGER DEFAULT 0,
    
    -- Metadata updates
    metadata_updates INTEGER DEFAULT 0,
    metadata_sources VARCHAR(200),
    
    -- Additional context
    notes TEXT
);

-- Configuration change tracking
CREATE TABLE IF NOT EXISTS configuration_statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    
    -- Change information
    config_section VARCHAR(100) NOT NULL,
    config_key VARCHAR(100) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    
    -- Change context
    change_type VARCHAR(50) NOT NULL, -- CREATE, UPDATE, DELETE
    change_source VARCHAR(50), -- WEB, API, CONFIG_FILE, SYSTEM
    user_agent VARCHAR(200),
    ip_address VARCHAR(45),
    
    -- Impact tracking
    restart_required BOOLEAN DEFAULT FALSE,
    validation_passed BOOLEAN DEFAULT TRUE,
    validation_errors TEXT,
    
    -- Quality profile usage tracking
    quality_profile VARCHAR(100),
    usage_count INTEGER DEFAULT 0,
    
    -- List configuration tracking
    list_name VARCHAR(100),
    media_type VARCHAR(20)
);

-- HTTP request statistics for detailed API/web tracking
CREATE TABLE IF NOT EXISTS http_request_statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    
    -- Request details
    method VARCHAR(10) NOT NULL,
    path VARCHAR(500) NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms INTEGER NOT NULL,
    
    -- Request size metrics
    request_size_bytes INTEGER,
    response_size_bytes INTEGER,
    
    -- Client information
    user_agent VARCHAR(500),
    ip_address VARCHAR(45),
    
    -- Additional context
    component VARCHAR(50), -- API, WEB, ADMIN
    endpoint_category VARCHAR(100),
    error_message TEXT
);

-- Indexer performance detailed tracking
CREATE TABLE IF NOT EXISTS indexer_performance_statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    
    -- Indexer identification
    indexer_name VARCHAR(100) NOT NULL,
    indexer_type VARCHAR(50), -- NEWZNAB, TORZNAB, RSS
    indexer_url VARCHAR(500),
    
    -- Performance metrics
    response_time_ms INTEGER,
    success BOOLEAN DEFAULT TRUE,
    error_type VARCHAR(100),
    error_message TEXT,
    
    -- Search metrics
    search_query VARCHAR(500),
    results_count INTEGER DEFAULT 0,
    results_filtered INTEGER DEFAULT 0,
    
    -- Rate limiting
    rate_limited BOOLEAN DEFAULT FALSE,
    retry_count INTEGER DEFAULT 0,
    
    -- Quality metrics
    connection_quality VARCHAR(20), -- EXCELLENT, GOOD, POOR, FAILED
    dns_resolution_time_ms INTEGER,
    ssl_handshake_time_ms INTEGER
);

-- Create indices for performance
CREATE INDEX IF NOT EXISTS idx_performance_statistics_timestamp ON performance_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_error_statistics_timestamp ON error_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_error_statistics_type ON error_statistics(error_type, timestamp);
CREATE INDEX IF NOT EXISTS idx_maintenance_statistics_timestamp ON maintenance_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_maintenance_statistics_type ON maintenance_statistics(operation_type, timestamp);
CREATE INDEX IF NOT EXISTS idx_configuration_statistics_timestamp ON configuration_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_configuration_statistics_section ON configuration_statistics(config_section, timestamp);
CREATE INDEX IF NOT EXISTS idx_http_request_statistics_timestamp ON http_request_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_http_request_statistics_path ON http_request_statistics(path, timestamp);
CREATE INDEX IF NOT EXISTS idx_indexer_performance_statistics_timestamp ON indexer_performance_statistics(timestamp);
CREATE INDEX IF NOT EXISTS idx_indexer_performance_statistics_name ON indexer_performance_statistics(indexer_name, timestamp);