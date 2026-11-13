package services

import (
	"database/sql"
	"testing"

	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
)

// setupTestDB creates an in-memory SQLite database with necessary tables for service tests
func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT,
			salt TEXT,
			role_id INTEGER NOT NULL DEFAULT 1,
			first_name TEXT,
			last_name TEXT,
			display_name TEXT,
			avatar_url TEXT,
			time_zone TEXT,
			language TEXT,
			settings TEXT DEFAULT '{}',
			is_active BOOLEAN DEFAULT 1,
			is_locked BOOLEAN DEFAULT 0,
			locked_until DATETIME,
			failed_login_attempts INTEGER DEFAULT 0,
			last_login_at DATETIME,
			last_login_ip TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS login_throttle (subject TEXT NOT NULL, scope TEXT NOT NULL, failures INTEGER NOT NULL DEFAULT 0, blocked_until_ms BIGINT NOT NULL DEFAULT 0, last_failure_ms BIGINT NOT NULL DEFAULT 0, last_success_ms BIGINT NOT NULL DEFAULT 0, version BIGINT NOT NULL DEFAULT 0, PRIMARY KEY (subject, scope))`,
		`CREATE TABLE IF NOT EXISTS favorites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id INTEGER NOT NULL,
			category TEXT,
			notes TEXT,
			tags TEXT,
			is_public BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS favorite_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			color TEXT,
			icon TEXT,
			is_public BOOLEAN DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS favorite_shares (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			favorite_id INTEGER NOT NULL,
			shared_with_user_id INTEGER NOT NULL,
			permission TEXT DEFAULT 'view',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (favorite_id) REFERENCES favorites(id) ON DELETE CASCADE,
			FOREIGN KEY (shared_with_user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS error_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			error_code TEXT,
			component TEXT,
			stack_trace TEXT,
			context TEXT,
			system_info TEXT,
			user_agent TEXT,
			url TEXT,
			fingerprint TEXT,
			status TEXT DEFAULT 'new',
			reported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS crash_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			signal TEXT NOT NULL,
			message TEXT NOT NULL,
			stack_trace TEXT,
			context TEXT,
			system_info TEXT,
			fingerprint TEXT,
			status TEXT DEFAULT 'new',
			reported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS media_access_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			media_id INTEGER NOT NULL,
			action TEXT NOT NULL,
			device_info TEXT,
			location TEXT,
			ip_address TEXT,
			user_agent TEXT,
			playback_duration INTEGER,
			access_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS analytics_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			event_category TEXT,
			data TEXT,
			device_info TEXT,
			location TEXT,
			ip_address TEXT,
			user_agent TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			url TEXT NOT NULL,
			username TEXT,
			password TEXT,
			sync_direction TEXT DEFAULT 'bidirectional',
			local_path TEXT,
			remote_path TEXT,
			sync_settings TEXT,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_sync_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			status TEXT DEFAULT 'running',
			sync_type TEXT,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			duration INTEGER,
			total_files INTEGER DEFAULT 0,
			synced_files INTEGER DEFAULT 0,
			failed_files INTEGER DEFAULT 0,
			skipped_files INTEGER DEFAULT 0,
			error_message TEXT,
			FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			frequency TEXT NOT NULL,
			last_run DATETIME,
			next_run DATETIME,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS conversion_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			source_path TEXT NOT NULL,
			target_path TEXT NOT NULL,
			source_format TEXT NOT NULL,
			target_format TEXT NOT NULL,
			conversion_type TEXT NOT NULL,
			quality TEXT DEFAULT 'medium',
			settings TEXT,
			priority INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			scheduled_for DATETIME,
			duration INTEGER,
			error_message TEXT,
			warning TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS log_collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			components TEXT,
			log_level TEXT DEFAULT 'info',
			start_time DATETIME,
			end_time DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			status TEXT DEFAULT 'pending',
			entry_count INTEGER DEFAULT 0,
			filters TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS log_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collection_id INTEGER NOT NULL,
			timestamp DATETIME NOT NULL,
			level TEXT NOT NULL,
			component TEXT,
			message TEXT NOT NULL,
			context TEXT,
			FOREIGN KEY (collection_id) REFERENCES log_collections(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS log_shares (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collection_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			share_token TEXT NOT NULL UNIQUE,
			share_type TEXT DEFAULT 'private',
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			accessed_at DATETIME,
			is_active BOOLEAN DEFAULT 1,
			permissions TEXT,
			recipients TEXT,
			FOREIGN KEY (collection_id) REFERENCES log_collections(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS configurations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version TEXT NOT NULL,
			config_data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS configuration_backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			config_data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS configuration_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			category TEXT,
			config_data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS wizard_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL,
			current_step INTEGER DEFAULT 0,
			total_steps INTEGER DEFAULT 0,
			step_data TEXT DEFAULT '{}',
			configuration TEXT DEFAULT '{}',
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_activity DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_completed BOOLEAN DEFAULT 0,
			config_type TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			permissions TEXT DEFAULT '{}',
			is_system BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT OR IGNORE INTO roles (id, name, description, permissions, is_system) VALUES (1, 'user', 'Regular user', '["read","write"]', 1)`,
		`INSERT OR IGNORE INTO roles (id, name, description, permissions, is_system) VALUES (2, 'admin', 'Administrator', '["read","write","admin","manage_users","view_shares","edit_shares","delete_shares"]', 1)`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			session_token TEXT NOT NULL UNIQUE,
			refresh_token TEXT,
			device_info TEXT,
			ip_address TEXT,
			user_agent TEXT,
			is_active BOOLEAN DEFAULT 1,
			duration INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			last_activity_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS system_configuration (
			id INTEGER PRIMARY KEY,
			version TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS system_configuration_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL,
			old_value TEXT,
			new_value TEXT NOT NULL,
			changed_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (changed_by) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS wizard_progress (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			current_step TEXT,
			step_data TEXT DEFAULT '{}',
			all_data TEXT DEFAULT '{}',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS wizard_completion (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			completed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS configuration_profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT,
			user_id INTEGER,
			configuration TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_active BOOLEAN DEFAULT 0,
			tags TEXT DEFAULT '[]',
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`INSERT OR IGNORE INTO users (id, username, email, password_hash, salt, is_active) VALUES (1, 'testuser', 'test@example.com', '', '', 1)`,
		`INSERT OR IGNORE INTO users (id, username, email, password_hash, salt, is_active) VALUES (2, 'testuser2', 'test2@example.com', '', '', 1)`,
	}

	for _, migration := range migrations {
		if _, err := sqlDB.Exec(migration); err != nil {
			t.Fatalf("Failed to run migration: %v\nSQL: %s", err, migration)
		}
	}

	return database.WrapDB(sqlDB, database.DialectSQLite)
}
