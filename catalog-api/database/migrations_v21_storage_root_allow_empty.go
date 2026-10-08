package database

import (
	"context"
	"fmt"
)

// addStorageRootsAllowEmptyColumn adds storage_roots.allow_empty (WF22 R3, PA-03).
//
// The generic scanner treats an EMPTY root as a failure by default: an empty listing is also what an unreachable, mis-addressed or
// wrongly-authorised source looks like, and "completed with 0 files" for those is the PASS-bluff the scanner exists to prevent. A share that
// is legitimately empty (a fresh Synology share holding only @eaDir / #recycle, an archive volume not yet filled) needs a way to say so, per
// root, explicitly: allow_empty = true. The default is false, so every existing root keeps the strict behaviour.
func (db *DB) addStorageRootsAllowEmptyColumn(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		return db.addStorageRootsAllowEmptyColumnPostgres(ctx)
	}
	return db.addStorageRootsAllowEmptyColumnSQLite(ctx)
}

func (db *DB) addStorageRootsAllowEmptyColumnSQLite(ctx context.Context) error {
	has, err := db.columnExistsSQLite(ctx, "storage_roots", "allow_empty")
	if err != nil {
		return fmt.Errorf("sqlite: probe storage_roots.allow_empty: %w", err)
	}
	if !has {
		if _, err := db.ExecContext(ctx, `ALTER TABLE storage_roots ADD COLUMN allow_empty BOOLEAN NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("sqlite: add storage_roots.allow_empty: %w", err)
		}
	}
	return nil
}

func (db *DB) addStorageRootsAllowEmptyColumnPostgres(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE storage_roots ADD COLUMN IF NOT EXISTS allow_empty BOOLEAN NOT NULL DEFAULT FALSE`); err != nil {
		return fmt.Errorf("postgres: add storage_roots.allow_empty: %w", err)
	}
	return nil
}
