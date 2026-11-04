package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"
)

// UserMetadataRepository handles user_metadata table operations.
type UserMetadataRepository struct {
	db *database.DB
}

// NewUserMetadataRepository creates a new user metadata repository.
func NewUserMetadataRepository(db *database.DB) *UserMetadataRepository {
	return &UserMetadataRepository{db: db}
}

// Create inserts user metadata and returns its ID.
func (r *UserMetadataRepository) Create(ctx context.Context, um *models.UserMetadata) (int64, error) {
	tagsJSON, _ := json.Marshal(um.Tags)
	now := time.Now()

	query := `INSERT INTO user_metadata (
		media_item_id, user_id, user_rating, watched_status, watched_date,
		personal_notes, tags, favorite, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(ctx, query,
		um.MediaItemID, um.UserID, um.UserRating, um.WatchedStatus, um.WatchedDate,
		um.PersonalNotes, string(tagsJSON), um.Favorite, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert user metadata: %w", err)
	}
	um.ID = id
	um.CreatedAt = now
	um.UpdatedAt = now
	return id, nil
}

// GetByItemAndUser returns user metadata for a specific item and user.
func (r *UserMetadataRepository) GetByItemAndUser(ctx context.Context, mediaItemID, userID int64) (*models.UserMetadata, error) {
	query := `SELECT id, media_item_id, user_id, user_rating, watched_status, watched_date,
		personal_notes, tags, favorite, created_at, updated_at
	FROM user_metadata WHERE media_item_id = ? AND user_id = ?`

	um, err := r.scanUserMetadata(r.db.QueryRowContext(ctx, query, mediaItemID, userID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user metadata: %w", err)
	}
	return um, nil
}

// Update updates user metadata.
func (r *UserMetadataRepository) Update(ctx context.Context, um *models.UserMetadata) error {
	tagsJSON, _ := json.Marshal(um.Tags)
	now := time.Now()

	query := `UPDATE user_metadata SET
		user_rating = ?, watched_status = ?, watched_date = ?,
		personal_notes = ?, tags = ?, favorite = ?, updated_at = ?
	WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query,
		um.UserRating, um.WatchedStatus, um.WatchedDate,
		um.PersonalNotes, string(tagsJSON), um.Favorite, now, um.ID,
	)
	if err != nil {
		return fmt.Errorf("update user metadata: %w", err)
	}
	um.UpdatedAt = now
	return nil
}

// Upsert creates or updates user metadata for a media item and user.
func (r *UserMetadataRepository) Upsert(ctx context.Context, um *models.UserMetadata) error {
	existing, err := r.GetByItemAndUser(ctx, um.MediaItemID, um.UserID)
	if err != nil {
		return err
	}
	if existing != nil {
		um.ID = existing.ID
		return r.Update(ctx, um)
	}
	_, err = r.Create(ctx, um)
	return err
}

// GetFavorites returns all favorite items for a user.
func (r *UserMetadataRepository) GetFavorites(ctx context.Context, userID int64) ([]*models.UserMetadata, error) {
	query := `SELECT id, media_item_id, user_id, user_rating, watched_status, watched_date,
		personal_notes, tags, favorite, created_at, updated_at
	FROM user_metadata WHERE user_id = ? AND favorite = 1
	ORDER BY updated_at DESC`

	return r.queryUserMetadata(ctx, query, userID)
}

// GetByWatchedStatus returns user metadata filtered by watched status.
func (r *UserMetadataRepository) GetByWatchedStatus(ctx context.Context, userID int64, status string) ([]*models.UserMetadata, error) {
	query := `SELECT id, media_item_id, user_id, user_rating, watched_status, watched_date,
		personal_notes, tags, favorite, created_at, updated_at
	FROM user_metadata WHERE user_id = ? AND watched_status = ?
	ORDER BY watched_date DESC`

	return r.queryUserMetadata(ctx, query, userID, status)
}

// --- internal helpers ---

func (r *UserMetadataRepository) queryUserMetadata(ctx context.Context, query string, args ...interface{}) ([]*models.UserMetadata, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query user metadata: %w", err)
	}
	defer rows.Close()

	items := []*models.UserMetadata{}
	for rows.Next() {
		um := &models.UserMetadata{}
		var tagsJSON sql.NullString
		if err := rows.Scan(
			&um.ID, &um.MediaItemID, &um.UserID, &um.UserRating, &um.WatchedStatus,
			&um.WatchedDate, &um.PersonalNotes, &tagsJSON, &um.Favorite,
			&um.CreatedAt, &um.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if tagsJSON.Valid {
			json.Unmarshal([]byte(tagsJSON.String), &um.Tags)
		}
		items = append(items, um)
	}
	return items, rows.Err()
}

func (r *UserMetadataRepository) scanUserMetadata(row *sql.Row) (*models.UserMetadata, error) {
	um := &models.UserMetadata{}
	var tagsJSON sql.NullString
	err := row.Scan(
		&um.ID, &um.MediaItemID, &um.UserID, &um.UserRating, &um.WatchedStatus,
		&um.WatchedDate, &um.PersonalNotes, &tagsJSON, &um.Favorite,
		&um.CreatedAt, &um.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if tagsJSON.Valid {
		json.Unmarshal([]byte(tagsJSON.String), &um.Tags)
	}
	return um, nil
}
