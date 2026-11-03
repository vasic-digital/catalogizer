package database

import (
	"context"
	"fmt"
	"strings"

	"catalogizer/internal/logging"
)

// createServiceTables creates tables for analytics, favorites, error reporting,
// crash reporting, log management, and cache services.
// These tables were previously created lazily by services but are now part of
// the migration to ensure they exist on fresh databases.
func (db *DB) createServiceTables(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createServiceTablesPostgres(ctx)
	}
	return db.createServiceTablesSQLite(ctx)
}

func (db *DB) createServiceTablesSQLite(ctx context.Context) error {
	schema := `
	-- Favorites (matches repository/favorites_repository.go columns)
	CREATE TABLE IF NOT EXISTS favorites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id INTEGER NOT NULL,
		category TEXT DEFAULT '',
		notes TEXT DEFAULT '',
		tags TEXT,
		is_public INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, entity_type, entity_id)
	);

	CREATE INDEX IF NOT EXISTS idx_favorites_user ON favorites(user_id);
	CREATE INDEX IF NOT EXISTS idx_favorites_entity ON favorites(entity_type, entity_id);

	-- Favorite categories
	CREATE TABLE IF NOT EXISTS favorite_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		color TEXT DEFAULT '',
		icon TEXT DEFAULT '',
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, name)
	);

	-- Analytics events
	CREATE TABLE IF NOT EXISTS analytics_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		event_type TEXT NOT NULL,
		event_data TEXT DEFAULT '{}',
		media_id INTEGER,
		session_id TEXT,
		device_info TEXT DEFAULT '{}',
		ip_address TEXT,
		user_agent TEXT,
		device_type TEXT,
		location TEXT,
		event_category TEXT DEFAULT '',
		access_count INTEGER DEFAULT 0,
		file_type TEXT DEFAULT '',
		data TEXT DEFAULT '{}',
		duration_seconds INTEGER DEFAULT 0,
		country TEXT DEFAULT '',
		city TEXT DEFAULT '',
		latitude REAL DEFAULT 0,
		longitude REAL DEFAULT 0,
		session_start DATETIME,
		session_end DATETIME,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_analytics_events_user ON analytics_events(user_id);
	CREATE INDEX IF NOT EXISTS idx_analytics_events_type ON analytics_events(event_type);
	CREATE INDEX IF NOT EXISTS idx_analytics_events_date ON analytics_events(created_at);

	-- Media access logs
	CREATE TABLE IF NOT EXISTS media_access_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		media_id INTEGER NOT NULL,
		action TEXT NOT NULL DEFAULT 'view',
		access_type TEXT NOT NULL DEFAULT 'view',
		duration INTEGER DEFAULT 0,
		playback_duration INTEGER DEFAULT 0,
		ip_address TEXT,
		user_agent TEXT,
		device_type TEXT,
		device_info TEXT DEFAULT '{}',
		location TEXT,
		access_time DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_media_access_user ON media_access_logs(user_id);
	CREATE INDEX IF NOT EXISTS idx_media_access_media ON media_access_logs(media_id);

	-- Error reports (matches repository/error_reporting_repository.go columns)
	CREATE TABLE IF NOT EXISTS error_reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		level TEXT NOT NULL DEFAULT 'error',
		message TEXT,
		error_code TEXT DEFAULT '',
		component TEXT DEFAULT '',
		stack_trace TEXT,
		context TEXT DEFAULT '{}',
		system_info TEXT DEFAULT '{}',
		user_agent TEXT DEFAULT '',
		url TEXT DEFAULT '',
		fingerprint TEXT DEFAULT '',
		status TEXT DEFAULT 'new',
		reported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		resolved_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_error_reports_user ON error_reports(user_id);
	CREATE INDEX IF NOT EXISTS idx_error_reports_status ON error_reports(status);
	CREATE INDEX IF NOT EXISTS idx_error_reports_level ON error_reports(level);

	-- Crash reports (matches repository/crash_reporting_repository.go columns)
	CREATE TABLE IF NOT EXISTS crash_reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		signal TEXT NOT NULL DEFAULT 'crash',
		crash_type TEXT NOT NULL DEFAULT 'crash',
		message TEXT,
		stack_trace TEXT,
		context TEXT DEFAULT '{}',
		system_info TEXT DEFAULT '{}',
		device_info TEXT DEFAULT '{}',
		app_version TEXT DEFAULT '',
		os_version TEXT DEFAULT '',
		fingerprint TEXT DEFAULT '',
		status TEXT DEFAULT 'new',
		reported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		resolved_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_crash_reports_user ON crash_reports(user_id);

	-- Log collections (matches repository/log_management_repository.go columns)
	CREATE TABLE IF NOT EXISTS log_collections (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		components TEXT DEFAULT '[]',
		log_level TEXT DEFAULT 'info',
		start_time DATETIME,
		end_time DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		status TEXT DEFAULT 'active',
		entry_count INTEGER DEFAULT 0,
		filters TEXT DEFAULT '{}'
	);

	CREATE INDEX IF NOT EXISTS idx_log_collections_user ON log_collections(user_id);

	-- Log shares
	CREATE TABLE IF NOT EXISTS log_shares (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		collection_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL DEFAULT 0,
		share_token TEXT NOT NULL UNIQUE,
		share_type TEXT DEFAULT 'link',
		created_by INTEGER NOT NULL DEFAULT 0,
		can_read INTEGER DEFAULT 1,
		can_write INTEGER DEFAULT 0,
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		accessed_at DATETIME,
		is_active INTEGER DEFAULT 1,
		permissions TEXT DEFAULT '{}',
		recipients TEXT DEFAULT '[]',
		FOREIGN KEY (collection_id) REFERENCES log_collections(id) ON DELETE CASCADE
	);

	-- Cache entries (for CacheService)
	CREATE TABLE IF NOT EXISTS cache_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cache_key TEXT NOT NULL UNIQUE,
		cache_value TEXT,
		cache_type TEXT DEFAULT 'general',
		provider TEXT DEFAULT '',
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_cache_entries_key ON cache_entries(cache_key);
	CREATE INDEX IF NOT EXISTS idx_cache_entries_expires ON cache_entries(expires_at);

	-- API cache
	CREATE TABLE IF NOT EXISTS api_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cache_key TEXT NOT NULL UNIQUE,
		cache_value TEXT,
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Media metadata cache
	CREATE TABLE IF NOT EXISTS media_metadata_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cache_key TEXT NOT NULL UNIQUE,
		cache_value TEXT,
		provider TEXT DEFAULT '',
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Analytics reports
	CREATE TABLE IF NOT EXISTS analytics_reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		report_type TEXT NOT NULL,
		title TEXT,
		report_data TEXT DEFAULT '{}',
		format TEXT DEFAULT 'json',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Configuration wizard progress
	CREATE TABLE IF NOT EXISTS wizard_progress (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		current_step TEXT NOT NULL DEFAULT '',
		step_id TEXT NOT NULL DEFAULT '',
		step_data TEXT DEFAULT '{}',
		all_data TEXT DEFAULT '{}',
		completed INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to create service tables: %w", err)
	}

	return nil
}

// fixServiceTableColumns adds missing columns to existing service tables.
// This runs as v12 to fix tables created before v11 had correct column definitions.
func (db *DB) fixServiceTableColumns(ctx context.Context) error {
	alterStmts := []string{
		"ALTER TABLE error_reports ADD COLUMN level TEXT DEFAULT 'error'",
		"ALTER TABLE error_reports ADD COLUMN error_code TEXT DEFAULT ''",
		"ALTER TABLE error_reports ADD COLUMN component TEXT DEFAULT ''",
		"ALTER TABLE error_reports ADD COLUMN context TEXT DEFAULT '{}'",
		"ALTER TABLE error_reports ADD COLUMN user_agent TEXT DEFAULT ''",
		"ALTER TABLE error_reports ADD COLUMN url TEXT DEFAULT ''",
		"ALTER TABLE error_reports ADD COLUMN reported_at DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE error_reports ADD COLUMN resolved_at DATETIME",
		"ALTER TABLE crash_reports ADD COLUMN signal TEXT DEFAULT 'crash'",
		"ALTER TABLE crash_reports ADD COLUMN crash_type TEXT DEFAULT 'crash'",
		"ALTER TABLE crash_reports ADD COLUMN context TEXT DEFAULT '{}'",
		"ALTER TABLE crash_reports ADD COLUMN device_info TEXT DEFAULT '{}'",
		"ALTER TABLE crash_reports ADD COLUMN app_version TEXT DEFAULT ''",
		"ALTER TABLE crash_reports ADD COLUMN os_version TEXT DEFAULT ''",
		"ALTER TABLE crash_reports ADD COLUMN reported_at DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE crash_reports ADD COLUMN resolved_at DATETIME",
		"ALTER TABLE log_shares ADD COLUMN user_id INTEGER DEFAULT 0",
		"ALTER TABLE log_shares ADD COLUMN share_type TEXT DEFAULT 'link'",
		"ALTER TABLE log_shares ADD COLUMN accessed_at DATETIME",
		"ALTER TABLE log_shares ADD COLUMN is_active INTEGER DEFAULT 1",
		"ALTER TABLE log_shares ADD COLUMN permissions TEXT DEFAULT '{}'",
		"ALTER TABLE log_shares ADD COLUMN recipients TEXT DEFAULT '[]'",
		"ALTER TABLE wizard_progress ADD COLUMN current_step TEXT DEFAULT ''",
		"ALTER TABLE wizard_progress ADD COLUMN all_data TEXT DEFAULT '{}'",
		"ALTER TABLE media_access_logs ADD COLUMN action TEXT DEFAULT 'view'",
		"ALTER TABLE media_access_logs ADD COLUMN playback_duration INTEGER DEFAULT 0",
		"ALTER TABLE media_access_logs ADD COLUMN access_time DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE media_access_logs ADD COLUMN device_info TEXT DEFAULT '{}'",
		"ALTER TABLE analytics_events ADD COLUMN timestamp DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE analytics_events ADD COLUMN device_info TEXT DEFAULT '{}'",
		"ALTER TABLE analytics_events ADD COLUMN event_category TEXT DEFAULT ''",
		"ALTER TABLE analytics_events ADD COLUMN access_count INTEGER DEFAULT 0",
		"ALTER TABLE analytics_events ADD COLUMN file_type TEXT DEFAULT ''",
		"ALTER TABLE analytics_events ADD COLUMN session_start DATETIME",
		"ALTER TABLE analytics_events ADD COLUMN session_end DATETIME",
		"ALTER TABLE analytics_events ADD COLUMN data TEXT DEFAULT '{}'",
		"ALTER TABLE analytics_events ADD COLUMN duration_seconds INTEGER DEFAULT 0",
		"ALTER TABLE analytics_events ADD COLUMN country TEXT DEFAULT ''",
		"ALTER TABLE analytics_events ADD COLUMN city TEXT DEFAULT ''",
		"ALTER TABLE analytics_events ADD COLUMN latitude REAL DEFAULT 0",
		"ALTER TABLE analytics_events ADD COLUMN longitude REAL DEFAULT 0",
		"ALTER TABLE log_collections ADD COLUMN description TEXT DEFAULT ''",
		"ALTER TABLE log_collections ADD COLUMN components TEXT DEFAULT '[]'",
		"ALTER TABLE log_collections ADD COLUMN log_level TEXT DEFAULT 'info'",
		"ALTER TABLE log_collections ADD COLUMN start_time DATETIME",
		"ALTER TABLE log_collections ADD COLUMN end_time DATETIME",
		"ALTER TABLE log_collections ADD COLUMN completed_at DATETIME",
		"ALTER TABLE log_collections ADD COLUMN entry_count INTEGER DEFAULT 0",
		"ALTER TABLE log_collections ADD COLUMN filters TEXT DEFAULT '{}'",
		"ALTER TABLE favorites ADD COLUMN tags TEXT",
		"ALTER TABLE favorites ADD COLUMN is_public INTEGER DEFAULT 0",
		"ALTER TABLE favorites ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP",
	}
	for _, stmt := range alterStmts {
		if db.dialect.IsPostgres() {
			stmt = postgresAlterAddColumn(stmt)
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			errMsg := strings.ToLower(err.Error())
			// Ignore "duplicate column" errors — column may already exist
			if strings.Contains(errMsg, "duplicate column") || strings.Contains(errMsg, "already exists") {
				continue
			}
			logging.With(
				logging.String("statement", stmt),
				logging.ErrorField(err),
			).Warn("Migration v12 ALTER failed")
		}
	}

	return nil
}

// postgresAlterAddColumn rewrites a SQLite-flavoured "ALTER TABLE t ADD COLUMN
// c TYPE ..." into its idempotent PostgreSQL form: ADD COLUMN IF NOT EXISTS
// (so re-applying over columns the v11 create already made raises no server
// error, instead of relying on swallowing "already exists") and DATETIME ->
// TIMESTAMP (PostgreSQL has no DATETIME type).
func postgresAlterAddColumn(stmt string) string {
	stmt = strings.Replace(stmt, " ADD COLUMN ", " ADD COLUMN IF NOT EXISTS ", 1)
	return strings.Replace(stmt, " DATETIME", " TIMESTAMP", 1)
}

func (db *DB) createServiceTablesPostgres(ctx context.Context) error {
	// PostgreSQL version with same schema, adjusted for PG syntax
	schema := `
	CREATE TABLE IF NOT EXISTS favorites (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id INTEGER NOT NULL,
		category TEXT DEFAULT '',
		notes TEXT DEFAULT '',
		tags TEXT,
		is_public BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, entity_type, entity_id)
	);

	CREATE TABLE IF NOT EXISTS favorite_categories (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		color TEXT DEFAULT '',
		icon TEXT DEFAULT '',
		sort_order INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, name)
	);

	CREATE TABLE IF NOT EXISTS analytics_events (
		id SERIAL PRIMARY KEY,
		user_id INTEGER,
		event_type TEXT NOT NULL,
		event_data JSONB DEFAULT '{}',
		media_id INTEGER,
		session_id TEXT,
		device_info JSONB DEFAULT '{}',
		ip_address TEXT,
		user_agent TEXT,
		device_type TEXT,
		location TEXT,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS media_access_logs (
		id SERIAL PRIMARY KEY,
		user_id INTEGER,
		media_id INTEGER NOT NULL,
		action TEXT NOT NULL DEFAULT 'view',
		access_type TEXT NOT NULL DEFAULT 'view',
		duration INTEGER DEFAULT 0,
		playback_duration INTEGER DEFAULT 0,
		ip_address TEXT,
		user_agent TEXT,
		device_type TEXT,
		location TEXT,
		access_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS error_reports (
		id SERIAL PRIMARY KEY,
		user_id INTEGER,
		level TEXT NOT NULL DEFAULT 'error',
		message TEXT,
		error_code TEXT DEFAULT '',
		component TEXT DEFAULT '',
		stack_trace TEXT,
		context TEXT DEFAULT '{}',
		system_info TEXT DEFAULT '{}',
		user_agent TEXT DEFAULT '',
		url TEXT DEFAULT '',
		fingerprint TEXT DEFAULT '',
		status TEXT DEFAULT 'new',
		reported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		resolved_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS crash_reports (
		id SERIAL PRIMARY KEY,
		user_id INTEGER,
		signal TEXT NOT NULL DEFAULT 'crash',
		crash_type TEXT NOT NULL DEFAULT 'crash',
		message TEXT,
		stack_trace TEXT,
		context TEXT DEFAULT '{}',
		system_info TEXT DEFAULT '{}',
		device_info TEXT DEFAULT '{}',
		app_version TEXT DEFAULT '',
		os_version TEXT DEFAULT '',
		fingerprint TEXT DEFAULT '',
		status TEXT DEFAULT 'new',
		reported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		resolved_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS log_collections (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		components TEXT DEFAULT '[]',
		log_level TEXT DEFAULT 'info',
		start_time TIMESTAMP,
		end_time TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		status TEXT DEFAULT 'active',
		entry_count INTEGER DEFAULT 0,
		filters JSONB DEFAULT '{}'
	);

	CREATE TABLE IF NOT EXISTS log_shares (
		id SERIAL PRIMARY KEY,
		collection_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL DEFAULT 0,
		share_token TEXT NOT NULL UNIQUE,
		share_type TEXT DEFAULT 'link',
		created_by INTEGER NOT NULL DEFAULT 0,
		can_read BOOLEAN DEFAULT TRUE,
		can_write BOOLEAN DEFAULT FALSE,
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accessed_at TIMESTAMP,
		is_active BOOLEAN DEFAULT TRUE,
		permissions TEXT DEFAULT '{}',
		recipients TEXT DEFAULT '[]',
		FOREIGN KEY (collection_id) REFERENCES log_collections(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS cache_entries (
		id SERIAL PRIMARY KEY,
		cache_key TEXT NOT NULL UNIQUE,
		cache_value TEXT,
		cache_type TEXT DEFAULT 'general',
		provider TEXT DEFAULT '',
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS api_cache (
		id SERIAL PRIMARY KEY,
		cache_key TEXT NOT NULL UNIQUE,
		cache_value TEXT,
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS media_metadata_cache (
		id SERIAL PRIMARY KEY,
		cache_key TEXT NOT NULL UNIQUE,
		cache_value TEXT,
		provider TEXT DEFAULT '',
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS analytics_reports (
		id SERIAL PRIMARY KEY,
		user_id INTEGER,
		report_type TEXT NOT NULL,
		title TEXT,
		report_data JSONB DEFAULT '{}',
		format TEXT DEFAULT 'json',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS wizard_progress (
		id SERIAL PRIMARY KEY,
		user_id INTEGER,
		current_step TEXT NOT NULL DEFAULT '',
		step_id TEXT NOT NULL DEFAULT '',
		step_data JSONB DEFAULT '{}',
		all_data JSONB DEFAULT '{}',
		completed BOOLEAN DEFAULT FALSE,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to create service tables: %w", err)
	}

	return nil
}
