package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"
)

// ExternalMetadataRepository handles external_metadata table operations.
type ExternalMetadataRepository struct {
	db *database.DB
}

// NewExternalMetadataRepository creates a new external metadata repository.
func NewExternalMetadataRepository(db *database.DB) *ExternalMetadataRepository {
	return &ExternalMetadataRepository{db: db}
}

// Create inserts external metadata and returns its ID.
func (r *ExternalMetadataRepository) Create(ctx context.Context, em *models.ExternalMetadata) (int64, error) {
	query := `INSERT INTO external_metadata (
		media_item_id, provider, external_id, data, rating,
		review_url, cover_url, trailer_url, last_fetched
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	id, err := r.db.InsertReturningID(ctx, query,
		em.MediaItemID, em.Provider, em.ExternalID, em.Data, em.Rating,
		em.ReviewURL, em.CoverURL, em.TrailerURL, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert external metadata: %w", err)
	}
	em.ID = id
	em.LastFetched = now
	return id, nil
}

// GetByItem returns all external metadata for a media item.
func (r *ExternalMetadataRepository) GetByItem(ctx context.Context, mediaItemID int64) ([]*models.ExternalMetadata, error) {
	query := `SELECT id, media_item_id, provider, external_id, data, rating,
		review_url, cover_url, trailer_url, last_fetched
	FROM external_metadata WHERE media_item_id = ?
	ORDER BY provider`

	rows, err := r.db.QueryContext(ctx, query, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("get external metadata by item: %w", err)
	}
	defer rows.Close()

	items := []*models.ExternalMetadata{}
	for rows.Next() {
		em := &models.ExternalMetadata{}
		if err := rows.Scan(
			&em.ID, &em.MediaItemID, &em.Provider, &em.ExternalID, &em.Data,
			&em.Rating, &em.ReviewURL, &em.CoverURL, &em.TrailerURL, &em.LastFetched,
		); err != nil {
			return nil, err
		}
		items = append(items, em)
	}
	return items, rows.Err()
}

// GetByProvider returns external metadata for a specific provider and external ID.
func (r *ExternalMetadataRepository) GetByProvider(ctx context.Context, provider, externalID string) (*models.ExternalMetadata, error) {
	query := `SELECT id, media_item_id, provider, external_id, data, rating,
		review_url, cover_url, trailer_url, last_fetched
	FROM external_metadata WHERE provider = ? AND external_id = ? LIMIT 1`

	em := &models.ExternalMetadata{}
	err := r.db.QueryRowContext(ctx, query, provider, externalID).Scan(
		&em.ID, &em.MediaItemID, &em.Provider, &em.ExternalID, &em.Data,
		&em.Rating, &em.ReviewURL, &em.CoverURL, &em.TrailerURL, &em.LastFetched,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get by provider: %w", err)
	}
	return em, nil
}

// Upsert creates or updates external metadata keyed by (media_item_id, provider).
//
// A UNIQUE(media_item_id, provider) index (migration v20) guarantees at most one
// row per pair. Under concurrent enrichment two callers can both observe no
// existing row and both attempt an INSERT; the index lets only one win. The
// loser's INSERT fails with a unique-constraint violation, which we recover by
// re-reading the now-present row and switching to UPDATE — so the net result is
// exactly one row, never a duplicate, and the race never surfaces as an error to
// the caller. The index is the real guarantee; this fallback keeps the contended
// path correct and silent.
func (r *ExternalMetadataRepository) Upsert(ctx context.Context, em *models.ExternalMetadata) error {
	existing, err := r.findByItemAndProvider(ctx, em.MediaItemID, em.Provider)
	if err != nil {
		return err
	}
	if existing != nil {
		return r.update(ctx, em, existing.ID)
	}

	if _, err := r.Create(ctx, em); err != nil {
		if !isUniqueViolation(err) {
			return err
		}
		// Lost the INSERT race: a concurrent caller created the row first. Re-read
		// it and UPDATE so this caller's data still lands and no duplicate remains.
		existing, ferr := r.findByItemAndProvider(ctx, em.MediaItemID, em.Provider)
		if ferr != nil {
			return ferr
		}
		if existing == nil {
			// Row vanished between the conflict and the re-read (e.g. a concurrent
			// delete); surface the original insert error rather than guessing.
			return err
		}
		return r.update(ctx, em, existing.ID)
	}
	return nil
}

// update writes em over the existing row identified by id and refreshes its
// last_fetched timestamp.
func (r *ExternalMetadataRepository) update(ctx context.Context, em *models.ExternalMetadata, id int64) error {
	query := `UPDATE external_metadata SET
		external_id = ?, data = ?, rating = ?, review_url = ?,
		cover_url = ?, trailer_url = ?, last_fetched = ?
	WHERE id = ?`

	now := time.Now()
	if _, err := r.db.ExecContext(ctx, query,
		em.ExternalID, em.Data, em.Rating, em.ReviewURL,
		em.CoverURL, em.TrailerURL, now, id,
	); err != nil {
		return fmt.Errorf("update external metadata: %w", err)
	}
	em.ID = id
	em.LastFetched = now
	return nil
}

// isUniqueViolation reports whether err is a unique-constraint violation from
// either SQLite ("UNIQUE constraint failed: ...") or PostgreSQL ("duplicate key
// value violates unique constraint ..."). Matches the portable detection already
// used in the handlers layer (e.g. handlers/user_handler.go).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "violates unique constraint")
}

// Delete removes external metadata by ID.
func (r *ExternalMetadataRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM external_metadata WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete external metadata: %w", err)
	}
	return nil
}

func (r *ExternalMetadataRepository) findByItemAndProvider(ctx context.Context, mediaItemID int64, provider string) (*models.ExternalMetadata, error) {
	query := `SELECT id, media_item_id, provider, external_id, data, rating,
		review_url, cover_url, trailer_url, last_fetched
	FROM external_metadata WHERE media_item_id = ? AND provider = ? LIMIT 1`

	em := &models.ExternalMetadata{}
	err := r.db.QueryRowContext(ctx, query, mediaItemID, provider).Scan(
		&em.ID, &em.MediaItemID, &em.Provider, &em.ExternalID, &em.Data,
		&em.Rating, &em.ReviewURL, &em.CoverURL, &em.TrailerURL, &em.LastFetched,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find by item and provider: %w", err)
	}
	return em, nil
}
