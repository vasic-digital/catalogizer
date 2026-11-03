-- Catalogizer v3.0 - Multi-User Database Schema
-- Enhanced schema supporting multi-user, analytics, favorites, and advanced features

-- Enable foreign key constraints
PRAGMA foreign_keys = ON;

-- Core User Management Tables
-- ===========================

-- Roles and Permissions
CREATE TABLE IF NOT EXISTS roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    description TEXT,
    permissions TEXT NOT NULL, -- JSON array of permissions
    is_system_role BOOLEAN DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default system roles
INSERT OR IGNORE INTO roles (name, display_name, description, permissions, is_system_role) VALUES
('super_admin', 'Super Administrator', 'Full system access with all permissions',
 '["*"]', 1),
('admin', 'Administrator', 'Administrative access to most features',
 '["user.manage", "media.manage", "share.manage", "analytics.view", "system.configure"]', 1),
('manager', 'Manager', 'Manage users and content within organization',
 '["user.view", "media.manage", "share.create", "analytics.view"]', 1),
('user', 'Standard User', 'Standard user with basic media access',
 '["media.view", "media.favorite", "share.view", "profile.manage"]', 1),
('guest', 'Guest User', 'Limited read-only access',
 '["media.view"]', 1);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(255) NOT NULL,
    role_id INTEGER NOT NULL,
    display_name VARCHAR(100),
    avatar_url VARCHAR(500),
    preferences TEXT, -- JSON preferences object
    settings TEXT, -- JSON settings object
    location_tracking_enabled BOOLEAN DEFAULT 1,
    analytics_enabled BOOLEAN DEFAULT 1,
    is_active BOOLEAN DEFAULT 1,
    email_verified BOOLEAN DEFAULT 0,
    last_login_at TIMESTAMP,
    last_login_ip VARCHAR(45),
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id)
);

-- No default user is created here. An earlier revision seeded an 'admin' row
-- carrying a published password hash; applying this file by hand therefore
-- created a login anyone could use. The first admin is created by catalog-api
-- itself at startup from ADMIN_USERNAME / ADMIN_PASSWORD (placeholder and
-- published default passwords are refused).

-- User Sessions
CREATE TABLE IF NOT EXISTS user_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    session_token VARCHAR(255) NOT NULL UNIQUE,
    refresh_token VARCHAR(255),
    device_info TEXT, -- JSON device information
    ip_address VARCHAR(45),
    user_agent TEXT,
    is_active BOOLEAN DEFAULT 1,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Enhanced Media Tables
-- =====================

-- Update existing files table to support multi-user
-- Note: This assumes the existing table will be altered
-- ALTER TABLE files ADD COLUMN owner_id INTEGER REFERENCES users(id);
-- ALTER TABLE files ADD COLUMN visibility VARCHAR(20) DEFAULT 'private'; -- private, shared, public
-- ALTER TABLE files ADD COLUMN created_by INTEGER REFERENCES users(id);
-- ALTER TABLE files ADD COLUMN updated_by INTEGER REFERENCES users(id);

-- Media Access Logs
CREATE TABLE IF NOT EXISTS media_access_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    media_id INTEGER NOT NULL,
    action VARCHAR(50) NOT NULL, -- view, play, pause, stop, download, share
    location_latitude REAL,
    location_longitude REAL,
    location_accuracy REAL,
    location_address TEXT,
    device_info TEXT, -- JSON device information
    session_id VARCHAR(255),
    duration_seconds INTEGER, -- for playback actions
    position_seconds INTEGER, -- playback position
    quality_level VARCHAR(20), -- playback quality
    metadata TEXT, -- JSON additional metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES files(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_media_access_logs_user_id ON media_access_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_media_access_logs_media_id ON media_access_logs(media_id);
CREATE INDEX IF NOT EXISTS idx_media_access_logs_created_at ON media_access_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_media_access_logs_action ON media_access_logs(action);

-- Favorites System
-- ================

-- Generic favorites table supporting any entity type
CREATE TABLE IF NOT EXISTS favorites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    entity_type VARCHAR(50) NOT NULL, -- media, share, playlist, user, etc.
    entity_id INTEGER NOT NULL,
    category_id INTEGER,
    notes TEXT,
    sort_order INTEGER DEFAULT 0,
    is_pinned BOOLEAN DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES favorite_categories(id) ON DELETE SET NULL,
    UNIQUE(user_id, entity_type, entity_id)
);

-- Favorite Categories
CREATE TABLE IF NOT EXISTS favorite_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    color VARCHAR(7), -- hex color code
    icon VARCHAR(50),
    parent_id INTEGER,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES favorite_categories(id) ON DELETE CASCADE,
    UNIQUE(user_id, name)
);

-- Analytics and Reporting
-- =======================

-- Analytics Events (for detailed tracking)
CREATE TABLE IF NOT EXISTS analytics_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    session_id VARCHAR(255),
    event_type VARCHAR(100) NOT NULL,
    event_category VARCHAR(50),
    event_action VARCHAR(50),
    event_label VARCHAR(100),
    event_value REAL,
    entity_type VARCHAR(50),
    entity_id INTEGER,
    properties TEXT, -- JSON properties
    location_latitude REAL,
    location_longitude REAL,
    device_info TEXT, -- JSON device information
    user_agent TEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Create indexes for analytics performance
CREATE INDEX IF NOT EXISTS idx_analytics_events_user_id ON analytics_events(user_id);
CREATE INDEX IF NOT EXISTS idx_analytics_events_event_type ON analytics_events(event_type);
CREATE INDEX IF NOT EXISTS idx_analytics_events_created_at ON analytics_events(created_at);
CREATE INDEX IF NOT EXISTS idx_analytics_events_entity ON analytics_events(entity_type, entity_id);

-- Analytics Reports
CREATE TABLE IF NOT EXISTS analytics_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    report_type VARCHAR(50) NOT NULL, -- usage, performance, user_behavior, content
    parameters TEXT, -- JSON report parameters
    schedule_expression VARCHAR(100), -- cron expression for scheduled reports
    output_format VARCHAR(20) DEFAULT 'html', -- html, pdf, markdown, json
    created_by INTEGER NOT NULL,
    is_active BOOLEAN DEFAULT 1,
    last_generated_at TIMESTAMP,
    next_generation_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Generated Reports
CREATE TABLE IF NOT EXISTS generated_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size INTEGER,
    output_format VARCHAR(20),
    generation_time_seconds REAL,
    parameters_used TEXT, -- JSON snapshot of parameters
    row_count INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    FOREIGN KEY (report_id) REFERENCES analytics_reports(id) ON DELETE CASCADE
);

-- Format Conversion System
-- ========================

-- Conversion Jobs
CREATE TABLE IF NOT EXISTS conversion_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    source_file_id INTEGER NOT NULL,
    source_path VARCHAR(1000) NOT NULL,
    target_format VARCHAR(50) NOT NULL,
    target_path VARCHAR(1000),
    conversion_profile TEXT, -- JSON conversion parameters
    status VARCHAR(20) DEFAULT 'pending', -- pending, processing, completed, failed, cancelled
    progress_percentage INTEGER DEFAULT 0,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,
    output_info TEXT, -- JSON output file information
    priority INTEGER DEFAULT 5, -- 1-10, higher is more priority
    queue_position INTEGER,
    estimated_duration_seconds INTEGER,
    actual_duration_seconds INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (source_file_id) REFERENCES files(id) ON DELETE CASCADE
);

-- Conversion Profiles (presets)
CREATE TABLE IF NOT EXISTS conversion_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    input_formats TEXT, -- JSON array of supported input formats
    output_format VARCHAR(50) NOT NULL,
    parameters TEXT NOT NULL, -- JSON conversion parameters
    is_system_profile BOOLEAN DEFAULT 0,
    created_by INTEGER,
    is_active BOOLEAN DEFAULT 1,
    usage_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Sync and Backup System
-- ======================

-- Sync Endpoints
CREATE TABLE IF NOT EXISTS sync_endpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name VARCHAR(200) NOT NULL,
    endpoint_type VARCHAR(50) NOT NULL, -- webdav, ftp, cloud_storage
    url VARCHAR(1000) NOT NULL,
    credentials TEXT, -- JSON encrypted credentials
    settings TEXT, -- JSON sync settings
    is_active BOOLEAN DEFAULT 1,
    sync_direction VARCHAR(20) DEFAULT 'both', -- upload, download, both
    last_sync_at TIMESTAMP,
    last_successful_sync_at TIMESTAMP,
    sync_status VARCHAR(50) DEFAULT 'idle', -- idle, syncing, error
    error_message TEXT,
    files_synced INTEGER DEFAULT 0,
    bytes_synced INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Sync History
CREATE TABLE IF NOT EXISTS sync_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint_id INTEGER NOT NULL,
    sync_type VARCHAR(20) NOT NULL, -- manual, scheduled, auto
    direction VARCHAR(20) NOT NULL, -- upload, download, both
    status VARCHAR(20) NOT NULL, -- success, failed, partial
    files_processed INTEGER DEFAULT 0,
    files_successful INTEGER DEFAULT 0,
    files_failed INTEGER DEFAULT 0,
    bytes_transferred INTEGER DEFAULT 0,
    duration_seconds REAL,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    error_summary TEXT,
    details TEXT, -- JSON detailed sync information
    FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id) ON DELETE CASCADE
);

-- Sync Conflicts
CREATE TABLE IF NOT EXISTS sync_conflicts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint_id INTEGER NOT NULL,
    local_file_path VARCHAR(1000) NOT NULL,
    remote_file_path VARCHAR(1000) NOT NULL,
    conflict_type VARCHAR(50) NOT NULL, -- modified_both, deleted_local, deleted_remote
    local_file_info TEXT, -- JSON file metadata
    remote_file_info TEXT, -- JSON file metadata
    resolution VARCHAR(50), -- local_wins, remote_wins, manual, skip
    resolved_by INTEGER,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id) ON DELETE CASCADE,
    FOREIGN KEY (resolved_by) REFERENCES users(id)
);

-- Error and Crash Reporting
-- =========================

-- Error Reports
CREATE TABLE IF NOT EXISTS error_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    session_id VARCHAR(255),
    error_type VARCHAR(100) NOT NULL,
    error_level VARCHAR(20) DEFAULT 'error', -- debug, info, warning, error, critical
    error_message TEXT NOT NULL,
    error_code VARCHAR(50),
    stack_trace TEXT,
    context TEXT, -- JSON error context
    device_info TEXT, -- JSON device information
    app_version VARCHAR(50),
    os_version VARCHAR(50),
    user_agent TEXT,
    url VARCHAR(1000),
    user_feedback TEXT,
    is_crash BOOLEAN DEFAULT 0,
    is_resolved BOOLEAN DEFAULT 0,
    resolved_by INTEGER,
    resolved_at TIMESTAMP,
    resolution_notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (resolved_by) REFERENCES users(id)
);

-- Log Management
-- ==============

-- System Logs
CREATE TABLE IF NOT EXISTS system_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    log_level VARCHAR(20) NOT NULL, -- debug, info, warning, error, critical
    component VARCHAR(100) NOT NULL, -- api, android, sync, conversion, etc.
    message TEXT NOT NULL,
    context TEXT, -- JSON additional context
    user_id INTEGER,
    session_id VARCHAR(255),
    file_name VARCHAR(255),
    line_number INTEGER,
    function_name VARCHAR(255),
    request_id VARCHAR(255),
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Create indexes for log performance
CREATE INDEX IF NOT EXISTS idx_system_logs_level ON system_logs(log_level);
CREATE INDEX IF NOT EXISTS idx_system_logs_component ON system_logs(component);
CREATE INDEX IF NOT EXISTS idx_system_logs_created_at ON system_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_system_logs_user_id ON system_logs(user_id);

-- Log Exports
CREATE TABLE IF NOT EXISTS log_exports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    requested_by INTEGER NOT NULL,
    export_type VARCHAR(50) NOT NULL, -- email, download, external_app
    filters TEXT, -- JSON export filters
    file_path VARCHAR(500),
    file_size INTEGER,
    status VARCHAR(20) DEFAULT 'pending', -- pending, processing, completed, failed
    privacy_level VARCHAR(20) DEFAULT 'sanitized', -- full, sanitized, minimal
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (requested_by) REFERENCES users(id) ON DELETE CASCADE
);

-- Configuration and Settings
-- ==========================

-- System Configuration
CREATE TABLE IF NOT EXISTS system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key VARCHAR(200) NOT NULL UNIQUE,
    config_value TEXT,
    data_type VARCHAR(20) DEFAULT 'string', -- string, integer, boolean, json
    description TEXT,
    category VARCHAR(100),
    is_system_config BOOLEAN DEFAULT 0,
    requires_restart BOOLEAN DEFAULT 0,
    is_secret BOOLEAN DEFAULT 0, -- for sensitive configuration
    updated_by INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (updated_by) REFERENCES users(id)
);

-- Insert default system configuration
INSERT OR IGNORE INTO system_config (config_key, config_value, data_type, description, category, is_system_config) VALUES
('app.version', '3.0.0', 'string', 'Application version', 'system', 1),
('analytics.enabled', 'true', 'boolean', 'Enable analytics collection', 'analytics', 1),
('analytics.retention_days', '365', 'integer', 'Days to retain analytics data', 'analytics', 1),
('conversion.max_concurrent_jobs', '3', 'integer', 'Maximum concurrent conversion jobs', 'conversion', 1),
('sync.auto_sync_interval', '3600', 'integer', 'Auto sync interval in seconds', 'sync', 1),
('logging.retention_days', '30', 'integer', 'Days to retain log data', 'logging', 1),
('security.session_timeout', '86400', 'integer', 'Session timeout in seconds', 'security', 1),
('security.max_failed_login_attempts', '5', 'integer', 'Maximum failed login attempts', 'security', 1);

-- User Preferences (per-user configuration)
CREATE TABLE IF NOT EXISTS user_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    preference_key VARCHAR(200) NOT NULL,
    preference_value TEXT,
    data_type VARCHAR(20) DEFAULT 'string',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, preference_key)
);

-- Performance and Monitoring
-- ==========================

-- Performance Metrics
CREATE TABLE IF NOT EXISTS performance_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_type VARCHAR(100) NOT NULL, -- api_response_time, db_query_time, conversion_time
    metric_name VARCHAR(200) NOT NULL,
    metric_value REAL NOT NULL,
    unit VARCHAR(20), -- ms, seconds, bytes, percentage
    context TEXT, -- JSON additional context
    user_id INTEGER,
    session_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Create indexes for performance monitoring
CREATE INDEX IF NOT EXISTS idx_performance_metrics_type ON performance_metrics(metric_type);
CREATE INDEX IF NOT EXISTS idx_performance_metrics_name ON performance_metrics(metric_name);
CREATE INDEX IF NOT EXISTS idx_performance_metrics_created_at ON performance_metrics(created_at);

-- Health Checks
CREATE TABLE IF NOT EXISTS health_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    check_name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL, -- healthy, warning, critical
    response_time_ms REAL,
    details TEXT, -- JSON health check details
    checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Database Views for Common Queries
-- =================================

-- User summary view
CREATE VIEW IF NOT EXISTS user_summary AS
SELECT
    u.id,
    u.username,
    u.email,
    u.display_name,
    r.name as role_name,
    r.display_name as role_display_name,
    u.is_active,
    u.last_login_at,
    COUNT(DISTINCT mal.id) as total_media_accesses,
    COUNT(DISTINCT f.id) as total_favorites,
    u.created_at
FROM users u
LEFT JOIN roles r ON u.role_id = r.id
LEFT JOIN media_access_logs mal ON u.id = mal.user_id
LEFT JOIN favorites f ON u.id = f.user_id
GROUP BY u.id, u.username, u.email, u.display_name, r.name, r.display_name, u.is_active, u.last_login_at, u.created_at;

-- Media usage statistics view
CREATE VIEW IF NOT EXISTS media_usage_stats AS
SELECT
    mal.media_id,
    COUNT(*) as total_accesses,
    COUNT(DISTINCT mal.user_id) as unique_users,
    AVG(mal.duration_seconds) as avg_duration,
    MAX(mal.created_at) as last_accessed,
    COUNT(CASE WHEN mal.action = 'play' THEN 1 END) as play_count,
    COUNT(CASE WHEN mal.action = 'download' THEN 1 END) as download_count
FROM media_access_logs mal
GROUP BY mal.media_id;

-- Popular content view
CREATE VIEW IF NOT EXISTS popular_content AS
SELECT
    mus.media_id,
    mus.total_accesses,
    mus.unique_users,
    mus.play_count,
    mus.last_accessed,
    COUNT(f.id) as favorite_count
FROM media_usage_stats mus
LEFT JOIN favorites f ON f.entity_type = 'media' AND f.entity_id = mus.media_id
GROUP BY mus.media_id, mus.total_accesses, mus.unique_users, mus.play_count, mus.last_accessed
ORDER BY mus.total_accesses DESC, mus.unique_users DESC;

-- Database Triggers for Maintenance
-- =================================

-- Update user updated_at timestamp
CREATE TRIGGER IF NOT EXISTS update_users_timestamp
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Update sync endpoints timestamp
CREATE TRIGGER IF NOT EXISTS update_sync_endpoints_timestamp
AFTER UPDATE ON sync_endpoints
BEGIN
    UPDATE sync_endpoints SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Update system config timestamp
CREATE TRIGGER IF NOT EXISTS update_system_config_timestamp
AFTER UPDATE ON system_config
BEGIN
    UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Auto-delete old analytics events (optional, can be disabled)
-- CREATE TRIGGER IF NOT EXISTS cleanup_old_analytics_events
-- AFTER INSERT ON analytics_events
-- BEGIN
--     DELETE FROM analytics_events
--     WHERE created_at < datetime('now', '-1 year');
-- END;

-- Final schema version tracking
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO schema_migrations (version) VALUES ('3.0.0_multiuser_complete');

-- Comments and Documentation
-- =========================

/*
Catalogizer v3.0 Multi-User Schema Summary:

1. USER MANAGEMENT:
   - Complete role-based access control (RBAC)
   - Secure session management
   - User preferences and settings

2. ANALYTICS & TRACKING:
   - Detailed media access logging with location tracking
   - Comprehensive analytics events
   - Performance metrics collection

3. FAVORITES SYSTEM:
   - Generic favorites supporting any entity type
   - Hierarchical categories
   - User-specific organization

4. REPORTING SYSTEM:
   - Scheduled report generation
   - Multiple output formats (HTML, PDF, Markdown)
   - Report history and archiving

5. FORMAT CONVERSION:
   - Queue-based conversion jobs
   - Conversion profiles and presets
   - Progress tracking

6. SYNC & BACKUP:
   - Multiple endpoint support
   - Conflict resolution
   - Sync history and monitoring

7. ERROR HANDLING:
   - Comprehensive error reporting
   - Crash analytics
   - User feedback collection

8. LOGGING:
   - Structured system logging
   - Log export and sharing
   - Privacy controls

9. CONFIGURATION:
   - System-wide configuration
   - User-specific preferences
   - Feature toggles

10. PERFORMANCE:
    - Performance metrics collection
    - Health monitoring
    - System optimization

This schema supports:
- Thousands of concurrent users
- Millions of media access events
- Comprehensive analytics and reporting
- Real-time monitoring and alerting
- Complete audit trails
- GDPR compliance capabilities

Performance Considerations:
- Extensive indexing for fast queries
- Views for common query patterns
- Triggers for data consistency
- Configurable data retention
- Query optimization friendly structure

Security Features:
- Password hashing and salting
- Session token management
- Role-based permissions
- Audit logging
- Encrypted sensitive data storage
- Privacy controls
*/