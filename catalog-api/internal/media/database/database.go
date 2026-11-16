package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mutecomm/go-sqlcipher"
	"go.uber.org/zap"
)

// MediaDatabase handles SQLite database with SQLCipher encryption
type MediaDatabase struct {
	db       *sql.DB
	dbPath   string
	password string
	logger   *zap.Logger
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Path     string `json:"path"`
	Password string `json:"password"`
}

// NewMediaDatabase creates a new encrypted media database
func NewMediaDatabase(config DatabaseConfig, logger *zap.Logger) (*MediaDatabase, error) {
	if config.Path == "" {
		config.Path = "media_catalog.db"
	}

	if config.Password == "" {
		return nil, fmt.Errorf("database password is required for encryption")
	}

	mdb := &MediaDatabase{
		dbPath:   config.Path,
		password: config.Password,
		logger:   logger,
	}

	if err := mdb.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := mdb.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return mdb, nil
}

// connect establishes connection to the encrypted database
func (mdb *MediaDatabase) connect() error {
	// Connection string for SQLCipher
	dsn := fmt.Sprintf("file:%s?_pragma_key=%s&_pragma_cipher_page_size=4096", mdb.dbPath, mdb.password)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection and encryption
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Verify encryption is working
	var result string
	err = db.QueryRow("PRAGMA cipher_version").Scan(&result)
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to verify encryption: %w", err)
	}

	mdb.db = db
	mdb.logger.Info("Connected to encrypted media database",
		zap.String("path", mdb.dbPath),
		zap.String("cipher_version", result))

	return nil
}

// initialize creates database schema
func (mdb *MediaDatabase) initialize() error {
	// Read schema from file
	schemaPath := filepath.Join(filepath.Dir(mdb.dbPath), "schema.sql")
	schemaContent, err := os.ReadFile(schemaPath)
	if err != nil {
		// If schema file doesn't exist, use embedded schema
		return mdb.createSchemaFromString(getEmbeddedSchema())
	}

	return mdb.createSchemaFromString(string(schemaContent))
}

// createSchemaFromString executes schema SQL
func (mdb *MediaDatabase) createSchemaFromString(schema string) error {
	// Execute schema in a transaction
	tx, err := mdb.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit schema: %w", err)
	}

	mdb.logger.Info("Database schema initialized successfully")
	return nil
}

// GetDB returns the database connection
func (mdb *MediaDatabase) GetDB() *sql.DB {
	return mdb.db
}

// Close closes the database connection
func (mdb *MediaDatabase) Close() error {
	if mdb.db != nil {
		return mdb.db.Close()
	}
	return nil
}

// Backup creates an encrypted backup of the database
func (mdb *MediaDatabase) Backup(backupPath string) error {
	// Sanitize inputs for ATTACH DATABASE which cannot use parameterized queries.
	// Reject single quotes to prevent SQL injection via path or password.
	if strings.ContainsRune(backupPath, '\'') {
		return fmt.Errorf("backup path contains invalid character")
	}
	sanitizedPassword := strings.ReplaceAll(mdb.password, "'", "''")
	_, err := mdb.db.Exec(fmt.Sprintf(`
		ATTACH DATABASE '%s' AS backup KEY '%s';
		INSERT INTO backup.sqlite_master SELECT * FROM main.sqlite_master WHERE type='table';
	`, backupPath, sanitizedPassword))

	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Copy all tables
	tables, err := mdb.getTables()
	if err != nil {
		return fmt.Errorf("failed to get tables: %w", err)
	}

	tx, err := mdb.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin backup transaction: %w", err)
	}
	defer tx.Rollback()

	// Attach backup database
	if _, err := tx.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS backup KEY '%s'", backupPath, mdb.password)); err != nil {
		return fmt.Errorf("failed to attach backup database: %w", err)
	}

	// Copy each table
	for _, table := range tables {
		// Safe: table names come from getTables() which reads from sqlite_master (database-internal metadata, not user input)
		copyQuery := fmt.Sprintf("CREATE TABLE backup.%s AS SELECT * FROM main.%s", table, table) // #nosec G201
		if _, err := tx.Exec(copyQuery); err != nil {
			return fmt.Errorf("failed to copy table %s: %w", table, err)
		}
	}

	// Detach backup database
	if _, err := tx.Exec("DETACH DATABASE backup"); err != nil {
		return fmt.Errorf("failed to detach backup database: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit backup: %w", err)
	}

	mdb.logger.Info("Database backup created", zap.String("backup_path", backupPath))
	return nil
}

// HealthCheck verifies database health
func (mdb *MediaDatabase) HealthCheck() error {
	// Check database integrity
	var result string
	if err := mdb.db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}

	if result != "ok" {
		return fmt.Errorf("database integrity check failed: %s", result)
	}

	// Check if we can read from a table
	var count int
	if err := mdb.db.QueryRow("SELECT COUNT(*) FROM media_types").Scan(&count); err != nil {
		return fmt.Errorf("failed to query media_types: %w", err)
	}

	return nil
}

// GetStats returns database statistics
func (mdb *MediaDatabase) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Database size
	var pageCount, pageSize int64
	if err := mdb.db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return nil, err
	}
	if err := mdb.db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	stats["size_bytes"] = pageCount * pageSize

	// Table counts
	tableCounts := map[string]int64{
		"media_types":        0,
		"media_items":        0,
		"external_metadata":  0,
		"directory_analysis": 0,
		"media_files":        0,
		"media_collections":  0,
		"user_metadata":      0,
	}

	for table := range tableCounts {
		var count int64
		// Safe: table names are hardcoded string keys in the tableCounts map above, not user input
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table) // #nosec G201
		if err := mdb.db.QueryRow(query).Scan(&count); err != nil {
			mdb.logger.Error("Failed to count table", zap.String("table", table), zap.Error(err))
			continue
		}
		tableCounts[table] = count
	}
	stats["table_counts"] = tableCounts

	// Recent activity
	var recentAnalysis int64
	cutoffTime := time.Now().Add(-24 * time.Hour)
	if err := mdb.db.QueryRow("SELECT COUNT(*) FROM directory_analysis WHERE last_analyzed > ?", cutoffTime).Scan(&recentAnalysis); err == nil {
		stats["recent_analysis_24h"] = recentAnalysis
	}

	return stats, nil
}

// getTables returns list of all tables
func (mdb *MediaDatabase) getTables() ([]string, error) {
	rows, err := mdb.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, nil
}

// getEmbeddedSchema returns the embedded schema as fallback
func getEmbeddedSchema() string {
	return `
-- Simplified embedded schema for fallback
CREATE TABLE IF NOT EXISTS media_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    detection_patterns TEXT,
    metadata_providers TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS media_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    original_title TEXT,
    year INTEGER,
    description TEXT,
    genre TEXT,
    director TEXT,
    cast_crew TEXT,
    rating REAL,
    runtime INTEGER,
    language TEXT,
    country TEXT,
    status TEXT DEFAULT 'active',
    first_detected DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_type_id) REFERENCES media_types(id)
);

CREATE TABLE IF NOT EXISTS external_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    data TEXT NOT NULL,
    rating REAL,
    review_url TEXT,
    cover_url TEXT,
    trailer_url TEXT,
    last_fetched DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id),
    UNIQUE(media_item_id, provider)
);

CREATE TABLE IF NOT EXISTS user_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL DEFAULT 0,
    user_rating REAL,
    watched_status TEXT,
    watched_date DATETIME,
    personal_notes TEXT,
    tags TEXT,
    favorite BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id)
);

CREATE TABLE IF NOT EXISTS directory_analysis (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    directory_path TEXT NOT NULL UNIQUE,
    smb_root TEXT NOT NULL,
    media_item_id INTEGER,
    confidence_score REAL NOT NULL,
    detection_method TEXT NOT NULL,
    analysis_data TEXT,
    last_analyzed DATETIME DEFAULT CURRENT_TIMESTAMP,
    files_count INTEGER DEFAULT 0,
    total_size INTEGER DEFAULT 0,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id)
);

CREATE TABLE IF NOT EXISTS media_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    smb_root TEXT NOT NULL,
    filename TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    file_extension TEXT,
    quality_info TEXT,
    language TEXT,
    subtitle_tracks TEXT,
    audio_tracks TEXT,
    duration INTEGER,
    checksum TEXT,
    virtual_smb_link TEXT,
    direct_smb_link TEXT,
    last_verified DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id)
);

-- Insert basic media types
INSERT OR IGNORE INTO media_types (name, description) VALUES
('movie', 'Feature films and movies'),
('tv_show', 'Television series and episodes'),
('music', 'Music albums and tracks'),
('game', 'Video games and software'),
('software', 'Applications and utilities'),
('comic', 'Comic books and graphic novels'),
('training', 'Educational and training content'),
('other', 'Unclassified content');
`
}

// Vacuum optimizes the database
func (mdb *MediaDatabase) Vacuum() error {
	mdb.logger.Info("Starting database vacuum")
	if _, err := mdb.db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("vacuum failed: %w", err)
	}
	mdb.logger.Info("Database vacuum completed")
	return nil
}

// ChangePassword changes the database encryption password
func (mdb *MediaDatabase) ChangePassword(newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password cannot be empty")
	}

	// Escape single quotes for PRAGMA which cannot use parameterized queries
	sanitized := strings.ReplaceAll(newPassword, "'", "''")
	pragma := fmt.Sprintf("PRAGMA rekey = '%s'", sanitized)
	if _, err := mdb.db.Exec(pragma); err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	mdb.password = newPassword
	mdb.logger.Info("Database password changed successfully")
	return nil
}

// ExecuteInTransaction executes multiple statements in a transaction
func (mdb *MediaDatabase) ExecuteInTransaction(fn func(*sql.Tx) error) error {
	tx, err := mdb.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}
