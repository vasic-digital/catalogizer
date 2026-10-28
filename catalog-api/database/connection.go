package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"catalogizer/config"

	_ "github.com/lib/pq"
	_ "github.com/mutecomm/go-sqlcipher"
)

// DB represents the database connection with dialect awareness.
type DB struct {
	*sql.DB
	config  *config.DatabaseConfig
	dialect Dialect
}

// NewConnection creates a new database connection.
// It detects the dialect from config.Type ("postgres" or "sqlite")
// and opens the appropriate driver.
func NewConnection(cfg *config.DatabaseConfig) (*DB, error) {
	dbType := cfg.Type
	if dbType == "" {
		dbType = "sqlite"
	}

	var dialect Dialect
	var sqlDB *sql.DB
	var err error

	switch dbType {
	case "postgres":
		dialect = Dialect{Type: DialectPostgres}
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.SSLMode)
		sqlDB, err = sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres database: %w", err)
		}

	default: // "sqlite" or empty
		dialect = Dialect{Type: DialectSQLite}
		// NOTE: go-sqlcipher honours only _busy_timeout and _foreign_keys from
		// this list; WAL is enabled explicitly below. The remaining parameters
		// are kept for parity with the previous DSN.
		params := "_busy_timeout=30000&_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=1"
		if cfg.EnableWAL {
			params += "&_wal_autocheckpoint=1000"
		}
		if cfg.CacheSize != 0 {
			params += fmt.Sprintf("&_cache_size=%d", cfg.CacheSize)
		}
		var connStr string
		if isMemoryPath(cfg.Path) {
			// The one explicit opt-out: in-memory databases are never persisted.
			connStr = cfg.Path + "?" + params
		} else {
			// CAT-001: file databases are ALWAYS encrypted; any problem with
			// the key fails closed (no plaintext fallback). The key is applied
			// by the driver on every pooled connection through the DSN.
			keyHex, kerr := prepareSQLiteKey(cfg)
			if kerr != nil {
				return nil, fmt.Errorf("database encryption: %w", kerr)
			}
			connStr = keyedDSN(cfg.Path, keyHex, params)
		}
		sqlDB, err = sql.Open("sqlite3", connStr)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite database: %w", scrubDSN(err))
		}
	}

	// Configure connection pool with sensible defaults.
	// MaxOpenConns acts as the query concurrency semaphore — the sql.DB pool
	// blocks callers when all connections are in use, so an additional
	// application-level semaphore is unnecessary.
	maxOpen := cfg.MaxOpenConnections
	if maxOpen <= 0 {
		maxOpen = 25
	}
	sqlDB.SetMaxOpenConns(maxOpen)

	maxIdle := cfg.MaxIdleConnections
	if maxIdle <= 0 {
		maxIdle = 10
	}
	sqlDB.SetMaxIdleConns(maxIdle)

	connMaxLifetime := time.Duration(cfg.ConnMaxLifetime) * time.Second
	if cfg.ConnMaxLifetime <= 0 {
		connMaxLifetime = 5 * time.Minute
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	connMaxIdleTime := time.Duration(cfg.ConnMaxIdleTime) * time.Second
	if cfg.ConnMaxIdleTime <= 0 {
		connMaxIdleTime = 3 * time.Minute
	}
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", scrubDSN(err))
	}

	// Ensure WAL mode is active for SQLite (connection string
	// pragma may not be applied by go-sqlcipher).
	if dbType != "postgres" {
		if !isMemoryPath(cfg.Path) {
			if err := verifyKeyed(sqlDB); err != nil {
				sqlDB.Close()
				return nil, fmt.Errorf("database encryption: %w", err)
			}
		}
		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
		}
	}

	db := &DB{
		DB:      sqlDB,
		config:  cfg,
		dialect: dialect,
	}

	return db, nil
}

// Dialect returns the database dialect.
func (db *DB) Dialect() *Dialect {
	return &db.dialect
}

// --- Shadowed methods that auto-rewrite queries for dialect ---

// rewriteQuery applies all dialect-specific query transformations:
// placeholder rewriting (? → $1), INSERT OR IGNORE, INSERT OR REPLACE.
func (db *DB) rewriteQuery(query string) string {
	query = db.dialect.RewritePlaceholders(query)
	if db.dialect.IsPostgres() {
		query = db.dialect.RewriteInsertOrIgnore(query)
		query = db.dialect.RewriteInsertOrReplace(query)
		query = db.dialect.RewriteBooleanLiterals(query)
	}
	return query
}

// ExecContext executes a query with dialect rewriting.
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.DB.ExecContext(ctx, db.rewriteQuery(query), args...)
}

// QueryContext executes a query with dialect rewriting.
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, db.rewriteQuery(query), args...)
}

// QueryRowContext executes a query returning a single row with dialect rewriting.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRowContext(ctx, db.rewriteQuery(query), args...)
}

// defaultQueryTimeout is the safety timeout for context-free database operations.
// Prevents indefinite hangs from lock contention or network issues.
const defaultQueryTimeout = 30 * time.Second

// Exec executes a query with dialect rewriting and a default timeout.
// Unlike Query/QueryRow, the operation completes before this method returns,
// so defer cancel() is safe here.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)
	defer cancel()
	return db.DB.ExecContext(ctx, db.rewriteQuery(query), args...)
}

// Query executes a query with dialect rewriting.
//
// The returned *sql.Rows is consumed by the caller after this method returns
// (via rows.Next/Scan).  We cannot use context.WithTimeout + defer cancel()
// here because that would cancel the context before the caller iterates the
// result set, producing spurious "context canceled" errors.
//
// Timeout safety is provided by the database driver itself: SQLite uses
// _busy_timeout (30 s), PostgreSQL uses statement_timeout.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.QueryContext(context.Background(), db.rewriteQuery(query), args...)
}

// QueryRow executes a query returning a single row with dialect rewriting.
//
// *sql.Row buffers its result lazily — the actual read happens inside Scan(),
// which the caller invokes after this method returns.  Using
// context.WithTimeout + defer cancel() here races with Scan: the context may
// be canceled before the driver finishes reading the row, causing spurious
// "context canceled" errors.
//
// Timeout safety is provided by the database driver itself: SQLite uses
// _busy_timeout (30 s), PostgreSQL uses statement_timeout.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRowContext(context.Background(), db.rewriteQuery(query), args...)
}

// --- Dialect-aware helpers ---

// InsertReturningID executes an INSERT and returns the new row's ID.
// For PostgreSQL, it appends "RETURNING id" and uses QueryRow.
// For SQLite, it uses Exec + LastInsertId.
func (db *DB) InsertReturningID(ctx context.Context, query string, args ...interface{}) (int64, error) {
	query = db.rewriteQuery(query)

	if db.dialect.IsPostgres() {
		query += " RETURNING id"
		var id int64
		err := db.DB.QueryRowContext(ctx, query, args...).Scan(&id)
		if err != nil {
			return 0, err
		}
		return id, nil
	}

	// SQLite path
	result, err := db.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// TableExists checks if a table exists in the database.
func (db *DB) TableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	if db.dialect.IsPostgres() {
		err := db.DB.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)",
			tableName).Scan(&exists)
		return exists, err
	}
	// SQLite
	var count int
	err := db.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		tableName).Scan(&count)
	return count > 0, err
}

// HealthCheck performs a database health check.
func (db *DB) HealthCheck() error {
	ctx, cancel := db.createContext()
	defer cancel()
	return db.PingContext(ctx)
}

// GetStats returns database connection statistics.
func (db *DB) GetStats() sql.DBStats {
	return db.Stats()
}

// DatabaseType returns the configured database type string.
func (db *DB) DatabaseType() string {
	if db.dialect.IsPostgres() {
		return "postgres"
	}
	return "sqlite"
}

// DBPath returns the configured SQLite database file path.
// Returns an empty string for PostgreSQL connections or when no path is configured.
func (db *DB) DBPath() string {
	if db.config == nil {
		return ""
	}
	return db.config.Path
}

// WrapDB wraps a raw *sql.DB in a *DB with the given dialect.
// This is primarily used in tests where sqlmock or other test
// databases provide a raw *sql.DB.
func WrapDB(sqlDB *sql.DB, dialectType DialectType) *DB {
	if sqlDB == nil {
		return nil
	}
	return &DB{
		DB:      sqlDB,
		config:  &config.DatabaseConfig{},
		dialect: Dialect{Type: dialectType},
	}
}

// createContext creates a context with timeout.
func (db *DB) createContext() (context.Context, context.CancelFunc) {
	timeout := time.Duration(db.config.BusyTimeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}
