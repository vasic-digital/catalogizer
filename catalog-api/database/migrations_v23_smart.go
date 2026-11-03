package database

import (
	"context"
	"fmt"
)

// addSmartColumns (migration v23) adds what smart collections and smart
// playlists need on top of the existing tables:
//
//	media_collections: is_smart, smart_rules (validated JSON rule list),
//	                   owner_user_id (NULL = legacy shared collection), is_public
//	playlists:         is_smart, smart_rules
//
// Existing rows keep working: they become manual (is_smart 0), private
// (is_public 0) collections without an owner.
func (db *DB) addSmartColumns(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		stmts := []string{
			`ALTER TABLE media_collections ADD COLUMN IF NOT EXISTS is_smart BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE media_collections ADD COLUMN IF NOT EXISTS smart_rules TEXT`,
			`ALTER TABLE media_collections ADD COLUMN IF NOT EXISTS owner_user_id INTEGER`,
			`ALTER TABLE media_collections ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE playlists ADD COLUMN IF NOT EXISTS is_smart BOOLEAN NOT NULL DEFAULT FALSE`,
			`ALTER TABLE playlists ADD COLUMN IF NOT EXISTS smart_rules TEXT`,
			`CREATE INDEX IF NOT EXISTS idx_media_collections_owner ON media_collections(owner_user_id)`,
		}
		for _, s := range stmts {
			if _, err := db.ExecContext(ctx, s); err != nil {
				return fmt.Errorf("postgres: smart columns: %w", err)
			}
		}
		return nil
	}
	cols := []struct{ table, column, ddl string }{
		{"media_collections", "is_smart", `ALTER TABLE media_collections ADD COLUMN is_smart INTEGER NOT NULL DEFAULT 0`},
		{"media_collections", "smart_rules", `ALTER TABLE media_collections ADD COLUMN smart_rules TEXT`},
		{"media_collections", "owner_user_id", `ALTER TABLE media_collections ADD COLUMN owner_user_id INTEGER`},
		{"media_collections", "is_public", `ALTER TABLE media_collections ADD COLUMN is_public INTEGER NOT NULL DEFAULT 0`},
		{"playlists", "is_smart", `ALTER TABLE playlists ADD COLUMN is_smart INTEGER NOT NULL DEFAULT 0`},
		{"playlists", "smart_rules", `ALTER TABLE playlists ADD COLUMN smart_rules TEXT`},
	}
	for _, c := range cols {
		has, err := db.columnExistsSQLite(ctx, c.table, c.column)
		if err != nil {
			return fmt.Errorf("sqlite: probe %s.%s: %w", c.table, c.column, err)
		}
		if has {
			continue
		}
		if _, err := db.ExecContext(ctx, c.ddl); err != nil {
			return fmt.Errorf("sqlite: add %s.%s: %w", c.table, c.column, err)
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_media_collections_owner ON media_collections(owner_user_id)`); err != nil {
		return fmt.Errorf("sqlite: index media_collections.owner_user_id: %w", err)
	}
	return nil
}
