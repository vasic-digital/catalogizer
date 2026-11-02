package database

import (
	"context"
	"fmt"
)

// createRuntimeTables (v22) creates the tables that handlers and repositories
// wired in main.go query but that no earlier migration created (QA defect D5):
//
//   - system_configuration, system_configuration_history, configuration_backups,
//     configuration_templates, configuration_profiles, wizard_sessions,
//     wizard_completion  (repository/configuration_repository.go)
//   - log_entries                (repository/log_management_repository.go)
//   - favorite_shares            (repository/favorites_repository.go)
//   - login_attempts, system_alerts (services/reporting_service.go)
//   - thumbnail_cache, cache_activity (internal/services/cache_service.go)
//
// It also adds the unique indexes the repositories' upserts need
// (wizard_progress.user_id), de-duplicating existing rows first. The rewrite
// of "INSERT OR REPLACE" to a plain INSERT on PostgreSQL (dialect.go) cannot
// express an upsert, so the repositories use ON CONFLICT (see the code).
//
// Every statement is idempotent (IF NOT EXISTS), so a database where some of
// these tables were created by hand or by tests is left untouched.
func (db *DB) createRuntimeTables(ctx context.Context) error {
	var stmts []string
	if db.dialect.IsPostgres() {
		stmts = runtimeTablesPostgres
	} else {
		stmts = runtimeTablesSQLite
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("create runtime tables (v22): %w (statement: %.80s)", err, s)
		}
	}
	return nil
}

var runtimeTablesSQLite = []string{
	`CREATE TABLE IF NOT EXISTS system_configuration (
		id INTEGER PRIMARY KEY,
		version TEXT NOT NULL,
		configuration TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS system_configuration_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version TEXT NOT NULL,
		configuration TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS configuration_backups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		configuration TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS configuration_templates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		category TEXT NOT NULL DEFAULT '',
		configuration TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS configuration_profiles (
		profile_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		user_id INTEGER NOT NULL DEFAULT 0,
		configuration TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		is_active BOOLEAN NOT NULL DEFAULT 0,
		tags TEXT NOT NULL DEFAULT '[]'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_configuration_profiles_user ON configuration_profiles(user_id)`,
	`CREATE TABLE IF NOT EXISTS wizard_sessions (
		session_id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL DEFAULT 0,
		current_step INTEGER NOT NULL DEFAULT 0,
		total_steps INTEGER NOT NULL DEFAULT 0,
		step_data TEXT NOT NULL DEFAULT '{}',
		configuration TEXT NOT NULL DEFAULT '{}',
		started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_activity DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		is_completed BOOLEAN NOT NULL DEFAULT 0,
		config_type TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS wizard_completion (
		user_id INTEGER PRIMARY KEY,
		completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`DELETE FROM wizard_progress WHERE id NOT IN (SELECT MAX(id) FROM wizard_progress GROUP BY user_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_wizard_progress_user ON wizard_progress(user_id)`,
	`CREATE TABLE IF NOT EXISTS log_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		collection_id INTEGER NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		level TEXT NOT NULL DEFAULT 'info',
		component TEXT NOT NULL DEFAULT '',
		message TEXT NOT NULL DEFAULT '',
		context TEXT DEFAULT '{}'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_log_entries_collection ON log_entries(collection_id)`,
	`CREATE INDEX IF NOT EXISTS idx_log_entries_component_ts ON log_entries(component, timestamp)`,
	`CREATE TABLE IF NOT EXISTS favorite_shares (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		favorite_id INTEGER NOT NULL,
		shared_by_user INTEGER NOT NULL,
		shared_with TEXT NOT NULL DEFAULT '[]',
		permissions TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		is_active BOOLEAN NOT NULL DEFAULT 1
	)`,
	`CREATE INDEX IF NOT EXISTS idx_favorite_shares_favorite ON favorite_shares(favorite_id)`,
	`CREATE TABLE IF NOT EXISTS login_attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL DEFAULT '',
		ip_address TEXT NOT NULL DEFAULT '',
		success BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS system_alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		severity TEXT NOT NULL DEFAULT 'low',
		title TEXT NOT NULL DEFAULT '',
		message TEXT NOT NULL DEFAULT '',
		resolved BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		resolved_at DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS thumbnail_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		video_id INTEGER NOT NULL,
		position INTEGER NOT NULL,
		url TEXT NOT NULL,
		width INTEGER NOT NULL,
		height INTEGER NOT NULL,
		file_size INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_thumbnail_cache_key ON thumbnail_cache(video_id, position, width, height)`,
	`CREATE TABLE IF NOT EXISTS cache_activity (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		cache_key TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT '',
		hit BOOLEAN NOT NULL DEFAULT 0,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cache_activity_ts ON cache_activity(timestamp)`,
}

var runtimeTablesPostgres = []string{
	`CREATE TABLE IF NOT EXISTS system_configuration (
		id INTEGER PRIMARY KEY,
		version TEXT NOT NULL,
		configuration TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS system_configuration_history (
		id SERIAL PRIMARY KEY,
		version TEXT NOT NULL,
		configuration TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS configuration_backups (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		configuration TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS configuration_templates (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		category TEXT NOT NULL DEFAULT '',
		configuration TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS configuration_profiles (
		profile_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		user_id INTEGER NOT NULL DEFAULT 0,
		configuration TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		is_active BOOLEAN NOT NULL DEFAULT FALSE,
		tags TEXT NOT NULL DEFAULT '[]'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_configuration_profiles_user ON configuration_profiles(user_id)`,
	`CREATE TABLE IF NOT EXISTS wizard_sessions (
		session_id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL DEFAULT 0,
		current_step INTEGER NOT NULL DEFAULT 0,
		total_steps INTEGER NOT NULL DEFAULT 0,
		step_data TEXT NOT NULL DEFAULT '{}',
		configuration TEXT NOT NULL DEFAULT '{}',
		started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_activity TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		is_completed BOOLEAN NOT NULL DEFAULT FALSE,
		config_type TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS wizard_completion (
		user_id INTEGER PRIMARY KEY,
		completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`DELETE FROM wizard_progress WHERE id NOT IN (SELECT MAX(id) FROM wizard_progress GROUP BY user_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_wizard_progress_user ON wizard_progress(user_id)`,
	`CREATE TABLE IF NOT EXISTS log_entries (
		id SERIAL PRIMARY KEY,
		collection_id INTEGER NOT NULL,
		timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		level TEXT NOT NULL DEFAULT 'info',
		component TEXT NOT NULL DEFAULT '',
		message TEXT NOT NULL DEFAULT '',
		context TEXT DEFAULT '{}'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_log_entries_collection ON log_entries(collection_id)`,
	`CREATE INDEX IF NOT EXISTS idx_log_entries_component_ts ON log_entries(component, timestamp)`,
	`CREATE TABLE IF NOT EXISTS favorite_shares (
		id SERIAL PRIMARY KEY,
		favorite_id INTEGER NOT NULL,
		shared_by_user INTEGER NOT NULL,
		shared_with TEXT NOT NULL DEFAULT '[]',
		permissions TEXT NOT NULL DEFAULT '{}',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		is_active BOOLEAN NOT NULL DEFAULT TRUE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_favorite_shares_favorite ON favorite_shares(favorite_id)`,
	`CREATE TABLE IF NOT EXISTS login_attempts (
		id SERIAL PRIMARY KEY,
		username TEXT NOT NULL DEFAULT '',
		ip_address TEXT NOT NULL DEFAULT '',
		success BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS system_alerts (
		id SERIAL PRIMARY KEY,
		severity TEXT NOT NULL DEFAULT 'low',
		title TEXT NOT NULL DEFAULT '',
		message TEXT NOT NULL DEFAULT '',
		resolved BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		resolved_at TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS thumbnail_cache (
		id SERIAL PRIMARY KEY,
		video_id BIGINT NOT NULL,
		position BIGINT NOT NULL,
		url TEXT NOT NULL,
		width INTEGER NOT NULL,
		height INTEGER NOT NULL,
		file_size BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_thumbnail_cache_key ON thumbnail_cache(video_id, position, width, height)`,
	`CREATE TABLE IF NOT EXISTS cache_activity (
		id SERIAL PRIMARY KEY,
		type TEXT NOT NULL,
		cache_key TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT '',
		hit BOOLEAN NOT NULL DEFAULT FALSE,
		timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cache_activity_ts ON cache_activity(timestamp)`,
}
