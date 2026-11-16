package database

import (
	"context"
	"fmt"
)

// createUserFavoritesTable (migration v30, CAT-HAUTHZ-F2) creates user_favorites, the per-user store behind
// PUT /api/v1/media/:id/favorite and the is_favorite field of GET /api/v1/media/:id.
//
// Before v30 that route wrote media_items.is_favorite, ONE flag per item shared by every user, so one user's favorite was
// visible to, and overwritable by, every other user. Since v30 the caller's favorite state is the existence of a
// (user_id, media_id) row; the column media_items.is_favorite is no longer read or written by any handler.
//
// LEGACY DATA DECISION (recorded, not guessed): existing media_items.is_favorite = 1 flags are NOT migrated into
// user_favorites. The column never recorded WHO set the flag, so any attribution (to the first user, to every user, to an
// administrator) would invent a preference for an account that never expressed it, and "everybody" would re-create the
// cross-user leak this migration removes. The legacy flags stay in the column untouched (no data loss, no destructive
// DDL); an operator who knows the owner can insert the rows by hand. The cost is that a favorite set before the upgrade
// shows as not-favorite until the owner toggles it again.
//
// Both foreign keys cascade, so deleting a user or a media item removes its favorite rows. The DDL is portable
// (INTEGER/TIMESTAMP, IF NOT EXISTS), identical for SQLite and PostgreSQL, and idempotent.
func (db *DB) createUserFavoritesTable(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS user_favorites (
			user_id INTEGER NOT NULL,
			media_id INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, media_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_favorites_media ON user_favorites (media_id)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("create user_favorites: %w", err)
		}
	}
	return nil
}
