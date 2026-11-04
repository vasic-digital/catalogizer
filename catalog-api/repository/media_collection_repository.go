package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"
	"catalogizer/internal/smartrules"
)

// MediaCollectionRepository handles media_collections database operations.
type MediaCollectionRepository struct {
	db *database.DB
}

// NewMediaCollectionRepository creates a new media collection repository.
func NewMediaCollectionRepository(db *database.DB) *MediaCollectionRepository {
	return &MediaCollectionRepository{db: db}
}

// marshalJSONFieldString is a helper to marshal JSON fields to string.
func marshalJSONFieldString(v interface{}) (string, error) {
	s, err := marshalJSONField(v)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "null", nil
	}
	return *s, nil
}

// unmarshalJSONFieldString is a helper to unmarshal JSON fields from string.
func unmarshalJSONFieldString(data string, v interface{}) error {
	if data == "" || data == "null" {
		return nil
	}
	return json.Unmarshal([]byte(data), v)
}

// Create inserts a new media collection and returns the generated ID.
func (r *MediaCollectionRepository) Create(ctx context.Context, coll *models.MediaCollection) (int64, error) {
	externalIDsJSON, err := marshalJSONFieldString(coll.ExternalIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal external_ids: %w", err)
	}

	now := time.Now()
	if coll.CreatedAt.IsZero() {
		coll.CreatedAt = now
	}
	if coll.UpdatedAt.IsZero() {
		coll.UpdatedAt = now
	}

	query := `INSERT INTO media_collections (
		name, collection_type, description, total_items,
		external_ids, cover_url, created_at, updated_at,
		is_smart, smart_rules, owner_user_id, is_public
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(ctx, query,
		coll.Name, coll.CollectionType, coll.Description, coll.TotalItems,
		externalIDsJSON, coll.CoverURL, coll.CreatedAt, coll.UpdatedAt,
		coll.IsSmart, nullableRules(coll.SmartRules), coll.OwnerUserID, coll.IsPublic,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create media collection: %w", err)
	}

	coll.ID = id
	return id, nil
}

// GetByID retrieves a media collection by its ID.
func (r *MediaCollectionRepository) GetByID(ctx context.Context, id int64) (*models.MediaCollection, error) {
	query := `SELECT id, name, collection_type, description, total_items,
		external_ids, cover_url, created_at, updated_at,
		is_smart, smart_rules, owner_user_id, is_public
		FROM media_collections WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	coll, err := r.scanCollection(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("media collection not found")
		}
		return nil, fmt.Errorf("failed to get media collection: %w", err)
	}
	return coll, nil
}

// List returns all media collections with optional pagination.
func (r *MediaCollectionRepository) List(ctx context.Context, limit, offset int) ([]*models.MediaCollection, int, error) {
	// Count total
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_collections").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count media collections: %w", err)
	}

	query := `SELECT id, name, collection_type, description, total_items,
		external_ids, cover_url, created_at, updated_at,
		is_smart, smart_rules, owner_user_id, is_public
		FROM media_collections ORDER BY id LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list media collections: %w", err)
	}
	defer rows.Close()

	collections := []*models.MediaCollection{}
	for rows.Next() {
		coll, err := r.scanCollection(rows)
		if err != nil {
			continue
		}
		collections = append(collections, coll)
	}

	return collections, total, nil
}

// Update updates an existing media collection.
func (r *MediaCollectionRepository) Update(ctx context.Context, coll *models.MediaCollection) error {
	externalIDsJSON, err := marshalJSONFieldString(coll.ExternalIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal external_ids: %w", err)
	}

	coll.UpdatedAt = time.Now()

	query := `UPDATE media_collections SET
		name = ?, collection_type = ?, description = ?, total_items = ?,
		external_ids = ?, cover_url = ?, updated_at = ?,
		smart_rules = ?, is_public = ?
		WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query,
		coll.Name, coll.CollectionType, coll.Description, coll.TotalItems,
		externalIDsJSON, coll.CoverURL, coll.UpdatedAt,
		nullableRules(coll.SmartRules), coll.IsPublic, coll.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update media collection: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("media collection not found")
	}
	return nil
}

// Delete removes a media collection by ID.
func (r *MediaCollectionRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM media_collections WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete media collection: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("media collection not found")
	}
	return nil
}

// scanCollection scans a database row into a MediaCollection struct.
func (r *MediaCollectionRepository) scanCollection(row interface{ Scan(...interface{}) error }) (*models.MediaCollection, error) {
	var (
		id                   int64
		name, collType       string
		description          *string
		totalItems           int
		externalIDsJSON      string
		coverURL             *string
		createdAt, updatedAt time.Time
		isSmart, isPublic    bool
		smartRules           sql.NullString
		ownerID              sql.NullInt64
	)
	err := row.Scan(&id, &name, &collType, &description, &totalItems,
		&externalIDsJSON, &coverURL, &createdAt, &updatedAt,
		&isSmart, &smartRules, &ownerID, &isPublic)
	if err != nil {
		return nil, err
	}

	var externalIDs map[string]string
	if err := unmarshalJSONFieldString(externalIDsJSON, &externalIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal external_ids: %w", err)
	}

	coll := &models.MediaCollection{
		ID:             id,
		Name:           name,
		CollectionType: collType,
		Description:    description,
		TotalItems:     totalItems,
		ExternalIDs:    externalIDs,
		CoverURL:       coverURL,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		IsSmart:        isSmart,
		IsPublic:       isPublic,
	}
	if smartRules.Valid && smartRules.String != "" {
		coll.SmartRules = json.RawMessage(smartRules.String)
	}
	if ownerID.Valid {
		v := ownerID.Int64
		coll.OwnerUserID = &v
	}
	return coll, nil
}

// nullableRules maps an empty rule document to SQL NULL.
func nullableRules(r json.RawMessage) interface{} {
	if len(r) == 0 {
		return nil
	}
	return string(r)
}

// ListVisible returns the collections the viewer may see: an admin sees all;
// anyone else sees legacy shared collections (no owner), public collections and
// their own. Private collections of other users are never returned.
func (r *MediaCollectionRepository) ListVisible(ctx context.Context, viewerID int64, admin bool, limit, offset int) ([]*models.MediaCollection, int, error) {
	where := ""
	var args []interface{}
	if !admin {
		where = " WHERE (owner_user_id IS NULL OR owner_user_id = ? OR is_public = ?)"
		args = append(args, viewerID, true)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_collections"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count media collections: %w", err)
	}
	query := `SELECT id, name, collection_type, description, total_items,
		external_ids, cover_url, created_at, updated_at,
		is_smart, smart_rules, owner_user_id, is_public
		FROM media_collections` + where + ` ORDER BY id LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, append(append([]interface{}{}, args...), limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list media collections: %w", err)
	}
	defer rows.Close()
	out := []*models.MediaCollection{}
	for rows.Next() {
		coll, err := r.scanCollection(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan media collection: %w", err)
		}
		out = append(out, coll)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// CountSmartByOwner returns how many smart collections the user owns.
func (r *MediaCollectionRepository) CountSmartByOwner(ctx context.Context, ownerID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_collections WHERE is_smart = ? AND owner_user_id = ?", true, ownerID).Scan(&n)
	return n, err
}

// Querier exposes the underlying database for rule evaluation.
func (r *MediaCollectionRepository) Querier() smartrules.Querier { return r.db }

// IsPostgres reports the dialect (the rule compiler formats dates per dialect).
func (r *MediaCollectionRepository) IsPostgres() bool { return r.db.Dialect().IsPostgres() }

// ListManualItems returns one page of the members of a manual collection.
func (r *MediaCollectionRepository) ListManualItems(ctx context.Context, collectionID int64, limit, offset int) ([]smartrules.Item, int, error) {
	limit, offset = smartrules.NormalizePage(limit, offset)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_collection_items ci JOIN media_items mi ON mi.id = ci.media_item_id WHERE ci.collection_id = ?`, collectionID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count collection items: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT mi.id, mi.title, mt.name, mi.year, mi.rating, mi.genre, mi.first_detected
		FROM media_collection_items ci
		JOIN media_items mi ON mi.id = ci.media_item_id
		JOIN media_types mt ON mt.id = mi.media_type_id
		WHERE ci.collection_id = ? ORDER BY ci.id LIMIT ? OFFSET ?`, collectionID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list collection items: %w", err)
	}
	defer rows.Close()
	items := make([]smartrules.Item, 0, limit)
	for rows.Next() {
		var (
			it     smartrules.Item
			year   sql.NullInt64
			rating sql.NullFloat64
			genre  sql.NullString
			added  sql.NullTime
		)
		if err := rows.Scan(&it.ID, &it.Title, &it.MediaType, &year, &rating, &genre, &added); err != nil {
			return nil, 0, fmt.Errorf("failed to scan collection item: %w", err)
		}
		if year.Valid {
			y := int(year.Int64)
			it.Year = &y
		}
		if rating.Valid {
			it.Rating = &rating.Float64
		}
		if genre.Valid {
			it.Genre = &genre.String
		}
		if added.Valid {
			t := added.Time
			it.AddedAt = &t
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
