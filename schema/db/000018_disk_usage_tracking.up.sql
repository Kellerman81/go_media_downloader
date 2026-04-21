-- Disk usage history tracking for accurate storage forecasting

-- Storage path usage history
CREATE TABLE IF NOT EXISTS storage_usage_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    
    -- Path identification
    path_name VARCHAR(100) NOT NULL,
    path_location VARCHAR(500) NOT NULL,
    
    -- Storage metrics (in bytes)
    total_space INTEGER NOT NULL,
    used_space INTEGER NOT NULL,
    free_space INTEGER NOT NULL,
    usage_percent REAL NOT NULL,
    
    -- File system metrics
    file_count INTEGER DEFAULT 0,
    folder_count INTEGER DEFAULT 0,
    
    -- Growth metrics (calculated)
    bytes_added_since_last INTEGER DEFAULT 0,
    files_added_since_last INTEGER DEFAULT 0,
    
    -- Status information
    accessible BOOLEAN DEFAULT TRUE,
    error_message TEXT,
    
    -- Additional metadata
    file_system_type VARCHAR(50),
    mount_point VARCHAR(500)
);

-- Create index for efficient querying
CREATE INDEX IF NOT EXISTS idx_storage_usage_history_timestamp ON storage_usage_history(timestamp);
CREATE INDEX IF NOT EXISTS idx_storage_usage_history_path ON storage_usage_history(path_name, path_location, timestamp);
CREATE INDEX IF NOT EXISTS idx_storage_usage_history_recent ON storage_usage_history(path_name, timestamp DESC);