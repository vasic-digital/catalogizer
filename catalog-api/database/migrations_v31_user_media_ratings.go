package database

import (
	"context"
	"fmt"
)

// createUserMediaRatingsTable (migration v31, CAT-RATING-1) creates user_media_ratings, the per-user store behind
// PUT|GET|DELETE /api/v1/media/:id/rating.
//
// DATA MODEL DECISION (recorded): a rating is an INTEGER 0..10 enforced by a CHECK constraint as well as by the handler.
// Half steps are not stored: no existing column, client or document needs them (the Android RatingRequest carries a
// number but its UI is not built; media_items.rating / external ratings are a different, REAL-valued community rating
// that this table never touches). 0 is a real rating, distinct from "not rated" (no row).
//
// A rating belongs to one user: the (user_id, media_id) pair is the primary key and every handler query is scoped by the
// authenticated user id. Both foreign keys cascade, so deleting a user or a media item removes its ratings. The DDL is
// portable (INTEGER/TIMESTAMP, IF NOT EXISTS) and identical for SQLite and PostgreSQL, and idempotent.
func (db *DB) createUserMediaRatingsTable(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS user_media_ratings (
			user_id INTEGER NOT NULL,
			media_id INTEGER NOT NULL,
			rating INTEGER NOT NULL CHECK (rating >= 0 AND rating <= 10),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, media_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_media_ratings_media ON user_media_ratings (media_id)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("create user_media_ratings: %w", err)
		}
	}
	return nil
}
