package database

import (
	"context"
	"fmt"
)

// RunMigrations runs database migrations.
func (db *DB) RunMigrations(ctx context.Context) error {
	if err := db.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	migrations := []Migration{
		{Version: 1, Name: "create_initial_tables", Up: db.createInitialTables},
		{Version: 2, Name: "migrate_smb_to_storage_roots", Up: db.migrateSMBToStorageRoots},
		{Version: 3, Name: "create_auth_tables", Up: db.createAuthTables},
		{Version: 4, Name: "create_conversion_jobs_table", Up: db.createConversionJobsTable},
		{Version: 5, Name: "create_subtitle_tables", Up: db.createSubtitleTables},
		{Version: 6, Name: "fix_subtitle_foreign_keys", Up: db.fixSubtitleForeignKeys},
		{Version: 7, Name: "create_assets_table", Up: db.createAssetsTable},
		{Version: 8, Name: "create_media_entity_tables", Up: db.createMediaEntityTables},
		{Version: 9, Name: "create_performance_indexes", Up: db.createPerformanceIndexes},
		{Version: 10, Name: "create_sync_tables", Up: db.createSyncTables},
		{Version: 11, Name: "create_service_tables", Up: db.createServiceTables},
		{Version: 12, Name: "fix_service_table_columns", Up: db.fixServiceTableColumns},
		{Version: 13, Name: "create_playlist_tables", Up: db.createPlaylistTables},
		{Version: 14, Name: "create_additional_indexes", Up: db.createAdditionalIndexes},
		{Version: 15, Name: "create_playback_session_tables", Up: db.createPlaybackSessionTables},
		{Version: 16, Name: "create_cover_art_tables", Up: db.createCoverArtTables},
		{Version: 17, Name: "create_image_quality_assessments", Up: db.createImageQualityAssessmentsTable},
		{Version: 18, Name: "add_media_items_favorite_column", Up: db.addMediaItemsFavoriteColumn},
		{Version: 19, Name: "create_share_identity_bindings", Up: db.createShareIdentityBindingsTable},
		{Version: 20, Name: "external_metadata_unique_index", Up: db.createExternalMetadataUniqueIndex},
		{Version: 21, Name: "storage_roots_allow_empty", Up: db.addStorageRootsAllowEmptyColumn},
	}

	for _, migration := range migrations {
		if err := db.runMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migration.Name, err)
		}
	}

	return nil
}

// Migration represents a database migration.
type Migration struct {
	Version int
	Name    string
	Up      func(context.Context) error
}

// createMigrationsTable creates the migrations tracking table.
func (db *DB) createMigrationsTable(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createMigrationsTablePostgres(ctx)
	}
	return db.createMigrationsTableSQLite(ctx)
}

// runMigration runs a single migration if it hasn't been applied.
func (db *DB) runMigration(ctx context.Context, migration Migration) error {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations WHERE version = ?", migration.Version).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	if err := migration.Up(ctx); err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "INSERT INTO migrations (version, name) VALUES (?, ?)", migration.Version, migration.Name)
	return err
}

// --- Dialect dispatch functions ---

func (db *DB) createInitialTables(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createInitialTablesPostgres(ctx)
	}
	return db.createInitialTablesSQLite(ctx)
}

func (db *DB) migrateSMBToStorageRoots(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.migrateSMBToStorageRootsPostgres(ctx)
	}
	return db.migrateSMBToStorageRootsSQLite(ctx)
}

func (db *DB) createAuthTables(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createAuthTablesPostgres(ctx)
	}
	return db.createAuthTablesSQLite(ctx)
}

func (db *DB) createConversionJobsTable(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createConversionJobsTablePostgres(ctx)
	}
	return db.createConversionJobsTableSQLite(ctx)
}

func (db *DB) createSubtitleTables(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createSubtitleTablesPostgres(ctx)
	}
	return db.createSubtitleTablesSQLite(ctx)
}

func (db *DB) fixSubtitleForeignKeys(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.fixSubtitleForeignKeysPostgres(ctx)
	}
	return db.fixSubtitleForeignKeysSQLite(ctx)
}

func (db *DB) createAssetsTable(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createAssetsTablePostgres(ctx)
	}
	return db.createAssetsTableSQLite(ctx)
}

func (db *DB) createMediaEntityTables(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createMediaEntityTablesPostgres(ctx)
	}
	return db.createMediaEntityTablesSQLite(ctx)
}

func (db *DB) createAdditionalIndexes(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.createV14AdditionalIndexesPostgres(ctx)
	}
	return db.createV14AdditionalIndexesSQLite(ctx)
}
