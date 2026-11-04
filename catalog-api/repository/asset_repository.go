package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"

	"digital.vasic.assets/pkg/asset"
)

// AssetRecord is the database representation of an asset.
type AssetRecord struct {
	ID          string
	Type        string
	Status      string
	ContentType sql.NullString
	Size        int64
	SourceHint  sql.NullString
	EntityType  sql.NullString
	EntityID    sql.NullString
	Metadata    sql.NullString
	LocalPath   sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ResolvedAt  sql.NullTime
	ExpiresAt   sql.NullTime
}

// AssetRepository handles asset-related database operations.
type AssetRepository struct {
	db *database.DB
}

// NewAssetRepository creates a new asset repository.
func NewAssetRepository(db *database.DB) *AssetRepository {
	return &AssetRepository{db: db}
}

// CreateAsset inserts a new asset record.
func (r *AssetRepository) CreateAsset(ctx context.Context, a *asset.Asset) error {
	metadataJSON, _ := json.Marshal(a.Metadata)

	query := `
		INSERT INTO assets (id, type, status, content_type, size, source_hint,
			entity_type, entity_id, metadata, created_at, updated_at, resolved_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		string(a.ID), string(a.Type), string(a.Status),
		a.ContentType, a.Size, a.SourceHint,
		a.EntityType, a.EntityID, string(metadataJSON),
		a.CreatedAt, a.UpdatedAt, a.ResolvedAt, a.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert asset: %w", err)
	}
	return nil
}

// GetAsset retrieves an asset by ID.
func (r *AssetRepository) GetAsset(ctx context.Context, id asset.ID) (*asset.Asset, error) {
	query := `
		SELECT id, type, status, content_type, size, source_hint,
			entity_type, entity_id, metadata, created_at, updated_at,
			resolved_at, expires_at
		FROM assets WHERE id = ?`

	var rec AssetRecord
	err := r.db.QueryRowContext(ctx, query, string(id)).Scan(
		&rec.ID, &rec.Type, &rec.Status, &rec.ContentType, &rec.Size,
		&rec.SourceHint, &rec.EntityType, &rec.EntityID, &rec.Metadata,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.ResolvedAt, &rec.ExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get asset: %w", err)
	}

	return r.recordToAsset(&rec), nil
}

// UpdateAsset updates an existing asset record.
func (r *AssetRepository) UpdateAsset(ctx context.Context, a *asset.Asset) error {
	metadataJSON, _ := json.Marshal(a.Metadata)

	query := `
		UPDATE assets SET type = ?, status = ?, content_type = ?, size = ?,
			source_hint = ?, entity_type = ?, entity_id = ?, metadata = ?,
			updated_at = ?, resolved_at = ?, expires_at = ?
		WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query,
		string(a.Type), string(a.Status), a.ContentType, a.Size,
		a.SourceHint, a.EntityType, a.EntityID, string(metadataJSON),
		a.UpdatedAt, a.ResolvedAt, a.ExpiresAt, string(a.ID),
	)
	if err != nil {
		return fmt.Errorf("update asset: %w", err)
	}
	return nil
}

// FindByEntity returns all assets for the given entity.
func (r *AssetRepository) FindByEntity(ctx context.Context, entityType, entityID string) ([]*asset.Asset, error) {
	query := `
		SELECT id, type, status, content_type, size, source_hint,
			entity_type, entity_id, metadata, created_at, updated_at,
			resolved_at, expires_at
		FROM assets WHERE entity_type = ? AND entity_id = ?
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("find assets by entity: %w", err)
	}
	defer rows.Close()

	assets := []*asset.Asset{}
	for rows.Next() {
		var rec AssetRecord
		if err := rows.Scan(
			&rec.ID, &rec.Type, &rec.Status, &rec.ContentType, &rec.Size,
			&rec.SourceHint, &rec.EntityType, &rec.EntityID, &rec.Metadata,
			&rec.CreatedAt, &rec.UpdatedAt, &rec.ResolvedAt, &rec.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		assets = append(assets, r.recordToAsset(&rec))
	}

	return assets, rows.Err()
}

// FindPending returns assets in pending status, limited by count.
func (r *AssetRepository) FindPending(ctx context.Context, limit int) ([]*asset.Asset, error) {
	query := `
		SELECT id, type, status, content_type, size, source_hint,
			entity_type, entity_id, metadata, created_at, updated_at,
			resolved_at, expires_at
		FROM assets WHERE status = 'pending'
		ORDER BY created_at ASC LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("find pending assets: %w", err)
	}
	defer rows.Close()

	assets := []*asset.Asset{}
	for rows.Next() {
		var rec AssetRecord
		if err := rows.Scan(
			&rec.ID, &rec.Type, &rec.Status, &rec.ContentType, &rec.Size,
			&rec.SourceHint, &rec.EntityType, &rec.EntityID, &rec.Metadata,
			&rec.CreatedAt, &rec.UpdatedAt, &rec.ResolvedAt, &rec.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		assets = append(assets, r.recordToAsset(&rec))
	}

	return assets, rows.Err()
}

func (r *AssetRepository) recordToAsset(rec *AssetRecord) *asset.Asset {
	a := &asset.Asset{
		ID:        asset.ID(rec.ID),
		Type:      asset.Type(rec.Type),
		Status:    asset.Status(rec.Status),
		Size:      rec.Size,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}

	if rec.ContentType.Valid {
		a.ContentType = rec.ContentType.String
	}
	if rec.SourceHint.Valid {
		a.SourceHint = rec.SourceHint.String
	}
	if rec.EntityType.Valid {
		a.EntityType = rec.EntityType.String
	}
	if rec.EntityID.Valid {
		a.EntityID = rec.EntityID.String
	}
	if rec.ResolvedAt.Valid {
		a.ResolvedAt = &rec.ResolvedAt.Time
	}
	if rec.ExpiresAt.Valid {
		a.ExpiresAt = &rec.ExpiresAt.Time
	}

	a.Metadata = make(map[string]string)
	if rec.Metadata.Valid && rec.Metadata.String != "" {
		json.Unmarshal([]byte(rec.Metadata.String), &a.Metadata)
	}

	return a
}
