package database

import (
	"context"
	"os"
	"testing"

	"catalogizer/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB creates a temporary database for migration testing
func newTestDB(t *testing.T) (*DB, func()) {
	tmpFile, err := os.CreateTemp("", "migrations_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	cfg := &config.DatabaseConfig{
		Type:               "sqlite",
		Path:               tmpFile.Name(),
		MaxOpenConnections: 1,
		MaxIdleConnections: 1,
		ConnMaxLifetime:    3600,
		ConnMaxIdleTime:    1800,
		EnableWAL:          true,
		CacheSize:          2000,
		BusyTimeout:        5000,
	}

	db, err := NewConnection(cfg)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

// ---------------------------------------------------------------------------
// RunMigrations
// ---------------------------------------------------------------------------

func TestRunMigrations_Success(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.RunMigrations(ctx)
	assert.NoError(t, err)
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Run migrations twice; second run should be a no-op
	err := db.RunMigrations(ctx)
	require.NoError(t, err)

	err = db.RunMigrations(ctx)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// createMigrationsTable
// ---------------------------------------------------------------------------

func TestCreateMigrationsTable(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.createMigrationsTable(ctx)
	require.NoError(t, err)

	// Verify table exists by querying it
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCreateMigrationsTable_Idempotent(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	err := db.createMigrationsTable(ctx)
	require.NoError(t, err)

	// Call again; should succeed due to IF NOT EXISTS
	err = db.createMigrationsTable(ctx)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// runMigration
// ---------------------------------------------------------------------------

func TestRunMigration_AppliesOnce(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create migrations table first
	err := db.createMigrationsTable(ctx)
	require.NoError(t, err)

	callCount := 0
	migration := Migration{
		Version: 100,
		Name:    "test_migration",
		Up: func(ctx context.Context) error {
			callCount++
			return nil
		},
	}

	// First run should apply
	err = db.runMigration(ctx, migration)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second run should skip (already applied)
	err = db.runMigration(ctx, migration)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRunMigration_RecordsMigration(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.createMigrationsTable(ctx)
	require.NoError(t, err)

	migration := Migration{
		Version: 200,
		Name:    "recorded_migration",
		Up: func(ctx context.Context) error {
			return nil
		},
	}

	err = db.runMigration(ctx, migration)
	require.NoError(t, err)

	// Verify migration was recorded
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations WHERE version = ?", 200).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	var name string
	err = db.QueryRowContext(ctx, "SELECT name FROM migrations WHERE version = ?", 200).Scan(&name)
	assert.NoError(t, err)
	assert.Equal(t, "recorded_migration", name)
}

// ---------------------------------------------------------------------------
// createInitialTables
// ---------------------------------------------------------------------------

func TestCreateInitialTables(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.createInitialTables(ctx)
	require.NoError(t, err)

	// Verify core tables exist
	tables := []string{"storage_roots", "files", "file_metadata", "duplicate_groups", "virtual_paths", "scan_history"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.NoError(t, err, "checking table %s", table)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}
}

func TestCreateInitialTables_Idempotent(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	err := db.createInitialTables(ctx)
	require.NoError(t, err)

	// Call again; should succeed due to IF NOT EXISTS
	err = db.createInitialTables(ctx)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// createConversionJobsTable
// ---------------------------------------------------------------------------

func TestCreateConversionJobsTable(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Need auth tables first for the foreign key reference (users table)
	ctx := context.Background()
	err := db.createAuthTables(ctx)
	require.NoError(t, err)

	err = db.createConversionJobsTable(ctx)
	require.NoError(t, err)

	// Verify table exists
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='conversion_jobs'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify indexes exist
	var indexCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name LIKE 'idx_conversion_jobs_%'").Scan(&indexCount)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, indexCount, 3)
}

// ---------------------------------------------------------------------------
// createAuthTables
// ---------------------------------------------------------------------------

func TestCreateAuthTables(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.createAuthTables(ctx)
	require.NoError(t, err)

	// Verify auth tables exist
	authTables := []string{"users", "roles", "user_sessions", "permissions", "user_permissions", "auth_audit_log"}
	for _, table := range authTables {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.NoError(t, err, "checking table %s", table)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}

	// Verify default roles were inserted
	var roleCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM roles").Scan(&roleCount)
	assert.NoError(t, err)
	assert.Equal(t, 2, roleCount)
}

// ---------------------------------------------------------------------------
// MigrationSequence
// ---------------------------------------------------------------------------

func TestMigrationSequence_AllVersionsApplied(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.RunMigrations(ctx)
	require.NoError(t, err)

	// Verify all 21 migrations were recorded
	// (v17 adds image_quality_assessments; v18 adds media_items.is_favorite + updated_at;
	//  v19 adds share_identity_bindings; v20 adds external_metadata unique index;
	//  v21 adds storage_roots.allow_empty).
	const latestVersion = 21
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, latestVersion, count)

	// Verify each version exists
	for v := 1; v <= latestVersion; v++ {
		var exists int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations WHERE version = ?", v).Scan(&exists)
		assert.NoError(t, err)
		assert.Equal(t, 1, exists, "migration version %d should be recorded", v)
	}
}

// ---------------------------------------------------------------------------
// createSubtitleTables
// ---------------------------------------------------------------------------

func TestCreateSubtitleTables(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	// Subtitle tables need media_items reference which doesn't exist standalone,
	// but the tables use IF NOT EXISTS and the FK is deferred in SQLite by default.
	// We can still verify the tables get created.
	ctx := context.Background()
	err := db.createSubtitleTables(ctx)
	require.NoError(t, err)

	// Verify subtitle tables exist
	subtitleTables := []string{"subtitle_tracks", "subtitle_sync_status", "subtitle_cache", "subtitle_downloads", "media_subtitles"}
	for _, table := range subtitleTables {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.NoError(t, err, "checking table %s", table)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}
}

// ---------------------------------------------------------------------------
// MigrationStruct
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// createMediaEntityTables (v8)
// ---------------------------------------------------------------------------

func TestCreateMediaEntityTables(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Need initial tables + auth tables first (files, users FKs)
	err := db.RunMigrations(ctx)
	require.NoError(t, err)

	// Verify media entity tables exist
	entityTables := []string{
		"media_types", "media_items", "media_files",
		"media_collections", "media_collection_items",
		"external_metadata", "user_metadata",
		"directory_analyses", "detection_rules",
	}
	for _, table := range entityTables {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.NoError(t, err, "checking table %s", table)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}

	// Verify media types were seeded
	var typeCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_types").Scan(&typeCount)
	assert.NoError(t, err)
	assert.Equal(t, 11, typeCount, "should have 11 seeded media types")

	// Verify specific media types
	var movieName string
	err = db.QueryRowContext(ctx, "SELECT name FROM media_types WHERE name = 'movie'").Scan(&movieName)
	assert.NoError(t, err)
	assert.Equal(t, "movie", movieName)

	// Verify indexes exist
	var indexCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name LIKE 'idx_media_%'").Scan(&indexCount)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, indexCount, 7)
}

func TestCreateMediaEntityTables_CanInsertAndQuery(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := db.RunMigrations(ctx)
	require.NoError(t, err)

	// Insert a media item
	var movieTypeID int64
	err = db.QueryRowContext(ctx, "SELECT id FROM media_types WHERE name = 'movie'").Scan(&movieTypeID)
	require.NoError(t, err)

	id, err := db.InsertReturningID(ctx,
		"INSERT INTO media_items (media_type_id, title, year, status) VALUES (?, ?, ?, ?)",
		movieTypeID, "The Matrix", 1999, "detected")
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	// Query it back
	var title string
	var year int
	err = db.QueryRowContext(ctx, "SELECT title, year FROM media_items WHERE id = ?", id).Scan(&title, &year)
	assert.NoError(t, err)
	assert.Equal(t, "The Matrix", title)
	assert.Equal(t, 1999, year)

	// Insert a child (tv_episode of a tv_show) to test parent_id
	var tvShowTypeID int64
	err = db.QueryRowContext(ctx, "SELECT id FROM media_types WHERE name = 'tv_show'").Scan(&tvShowTypeID)
	require.NoError(t, err)

	showID, err := db.InsertReturningID(ctx,
		"INSERT INTO media_items (media_type_id, title, status) VALUES (?, ?, ?)",
		tvShowTypeID, "Breaking Bad", "detected")
	require.NoError(t, err)

	var epTypeID int64
	err = db.QueryRowContext(ctx, "SELECT id FROM media_types WHERE name = 'tv_episode'").Scan(&epTypeID)
	require.NoError(t, err)

	epID, err := db.InsertReturningID(ctx,
		"INSERT INTO media_items (media_type_id, title, parent_id, season_number, episode_number, status) VALUES (?, ?, ?, ?, ?, ?)",
		epTypeID, "Pilot", showID, 1, 1, "detected")
	require.NoError(t, err)

	// Verify parent_id
	var parentID int64
	err = db.QueryRowContext(ctx, "SELECT parent_id FROM media_items WHERE id = ?", epID).Scan(&parentID)
	assert.NoError(t, err)
	assert.Equal(t, showID, parentID)
}

func TestMigrationStruct(t *testing.T) {
	m := Migration{
		Version: 42,
		Name:    "test_migration",
		Up: func(ctx context.Context) error {
			return nil
		},
	}

	assert.Equal(t, 42, m.Version)
	assert.Equal(t, "test_migration", m.Name)
	assert.NotNil(t, m.Up)

	// Verify the Up function executes without error
	err := m.Up(context.Background())
	assert.NoError(t, err)
}
