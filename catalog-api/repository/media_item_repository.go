package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"
)

// MediaItemRepository handles media_items database operations.
type MediaItemRepository struct {
	db *database.DB
}

// NewMediaItemRepository creates a new media item repository.
func NewMediaItemRepository(db *database.DB) *MediaItemRepository {
	return &MediaItemRepository{db: db}
}

// Create inserts a new media item and returns the generated ID.
func (r *MediaItemRepository) Create(ctx context.Context, item *models.MediaItem) (int64, error) {
	genreJSON, err := marshalJSONField(item.Genre)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal genre: %w", err)
	}

	castCrewJSON, err := marshalJSONField(item.CastCrew)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal cast_crew: %w", err)
	}

	query := `INSERT INTO media_items (
		media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	if item.FirstDetected.IsZero() {
		item.FirstDetected = now
	}
	if item.LastUpdated.IsZero() {
		item.LastUpdated = now
	}

	id, err := r.db.InsertReturningID(ctx, query,
		item.MediaTypeID, item.Title, item.OriginalTitle, item.Year,
		item.Description, genreJSON, item.Director, castCrewJSON,
		item.Rating, item.Runtime, item.Language, item.Country,
		item.Status, item.ParentID, item.SeasonNumber, item.EpisodeNumber, item.TrackNumber,
		item.FirstDetected, item.LastUpdated,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create media item: %w", err)
	}

	item.ID = id
	return id, nil
}

// GetByID retrieves a media item by its ID.
func (r *MediaItemRepository) GetByID(ctx context.Context, id int64) (*models.MediaItem, error) {
	query := `SELECT id, media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	FROM media_items WHERE id = ?`

	item, err := r.scanItem(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("media item not found")
		}
		return nil, fmt.Errorf("failed to get media item: %w", err)
	}
	return item, nil
}

// GetByTitle retrieves a media item by title and media type ID.
func (r *MediaItemRepository) GetByTitle(ctx context.Context, title string, mediaTypeID int64) (*models.MediaItem, error) {
	query := `SELECT id, media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	FROM media_items WHERE title = ? AND media_type_id = ? LIMIT 1`

	item, err := r.scanItem(r.db.QueryRowContext(ctx, query, title, mediaTypeID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get media item by title: %w", err)
	}
	return item, nil
}

// GetByType retrieves media items by type with pagination. Returns items and total count.
func (r *MediaItemRepository) GetByType(ctx context.Context, mediaTypeID int64, limit, offset int) ([]*models.MediaItem, int64, error) {
	countQuery := `SELECT COUNT(*) FROM media_items WHERE media_type_id = ?`
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, mediaTypeID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count media items by type: %w", err)
	}

	query := `SELECT id, media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	FROM media_items WHERE media_type_id = ?
	ORDER BY title ASC LIMIT ? OFFSET ?`

	items, err := r.queryItems(ctx, query, mediaTypeID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetChildren retrieves all child media items for a given parent ID.
func (r *MediaItemRepository) GetChildren(ctx context.Context, parentID int64) ([]*models.MediaItem, error) {
	query := `SELECT id, media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	FROM media_items WHERE parent_id = ?
	ORDER BY season_number ASC, episode_number ASC, track_number ASC, title ASC`

	return r.queryItems(ctx, query, parentID)
}

// Search performs a full-text LIKE search with optional type filter. Returns items and total count.
func (r *MediaItemRepository) Search(ctx context.Context, query string, mediaTypes []int64, limit, offset int) ([]*models.MediaItem, int64, error) {
	baseWhere := `WHERE (title LIKE ? OR original_title LIKE ? OR description LIKE ?)`
	searchPattern := "%" + query + "%"
	args := []interface{}{searchPattern, searchPattern, searchPattern}

	if len(mediaTypes) > 0 {
		placeholders := make([]string, len(mediaTypes))
		for i := range mediaTypes {
			placeholders[i] = "?"
		}
		baseWhere += " AND media_type_id IN (" + strings.Join(placeholders, ",") + ")"
		for _, mt := range mediaTypes {
			args = append(args, mt)
		}
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM media_items " + baseWhere
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	selectQuery := `SELECT id, media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	FROM media_items ` + baseWhere + `
	ORDER BY title ASC
	LIMIT ? OFFSET ?`

	paginatedArgs := make([]interface{}, len(args))
	copy(paginatedArgs, args)
	paginatedArgs = append(paginatedArgs, limit, offset)

	items, err := r.queryItems(ctx, selectQuery, paginatedArgs...)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Update updates an existing media item.
func (r *MediaItemRepository) Update(ctx context.Context, item *models.MediaItem) error {
	genreJSON, err := marshalJSONField(item.Genre)
	if err != nil {
		return fmt.Errorf("failed to marshal genre: %w", err)
	}

	castCrewJSON, err := marshalJSONField(item.CastCrew)
	if err != nil {
		return fmt.Errorf("failed to marshal cast_crew: %w", err)
	}

	query := `UPDATE media_items SET
		media_type_id = ?, title = ?, original_title = ?, year = ?, description = ?,
		genre = ?, director = ?, cast_crew = ?, rating = ?, runtime = ?,
		language = ?, country = ?, status = ?, parent_id = ?,
		season_number = ?, episode_number = ?, track_number = ?,
		last_updated = ?
	WHERE id = ?`

	item.LastUpdated = time.Now()

	_, err = r.db.ExecContext(ctx, query,
		item.MediaTypeID, item.Title, item.OriginalTitle, item.Year,
		item.Description, genreJSON, item.Director, castCrewJSON,
		item.Rating, item.Runtime, item.Language, item.Country,
		item.Status, item.ParentID,
		item.SeasonNumber, item.EpisodeNumber, item.TrackNumber,
		item.LastUpdated, item.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update media item: %w", err)
	}
	return nil
}

// Delete removes a media item by ID.
func (r *MediaItemRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM media_items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete media item: %w", err)
	}
	return nil
}

// GetDuplicates finds media items with the same title, type, and optionally year.
func (r *MediaItemRepository) GetDuplicates(ctx context.Context, title string, mediaTypeID int64, year *int) ([]*models.MediaItem, error) {
	baseQuery := `SELECT id, media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	FROM media_items WHERE title = ? AND media_type_id = ?`

	args := []interface{}{title, mediaTypeID}

	if year != nil {
		baseQuery += " AND year = ?"
		args = append(args, *year)
	}

	baseQuery += " ORDER BY first_detected ASC"

	return r.queryItems(ctx, baseQuery, args...)
}

// Count returns the total number of media items.
func (r *MediaItemRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_items").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count media items: %w", err)
	}
	return count, nil
}

// ListDuplicateGroups returns groups of entities that share the same title and type.
func (r *MediaItemRepository) ListDuplicateGroups(ctx context.Context, limit, offset int) ([]DuplicateGroup, int64, error) {
	countQuery := `SELECT COUNT(*) FROM (
		SELECT title, media_type_id FROM media_items
		GROUP BY title, media_type_id HAVING COUNT(*) > 1
	) sub`
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count duplicate groups: %w", err)
	}

	query := `SELECT mi.title, mi.media_type_id, mt.name, COUNT(*) as cnt
		FROM media_items mi
		JOIN media_types mt ON mt.id = mi.media_type_id
		GROUP BY mi.title, mi.media_type_id, mt.name
		HAVING COUNT(*) > 1
		ORDER BY cnt DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list duplicate groups: %w", err)
	}
	defer rows.Close()

	groups := []DuplicateGroup{}
	for rows.Next() {
		var g DuplicateGroup
		if err := rows.Scan(&g.Title, &g.MediaTypeID, &g.MediaTypeName, &g.Count); err != nil {
			return nil, 0, fmt.Errorf("scan duplicate group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, total, rows.Err()
}

// DuplicateGroup represents a group of entities with the same title and type.
type DuplicateGroup struct {
	Title         string `json:"title"`
	MediaTypeID   int64  `json:"media_type_id"`
	MediaTypeName string `json:"media_type"`
	Count         int    `json:"count"`
}

// CountByType returns counts of media items grouped by media type name.
func (r *MediaItemRepository) CountByType(ctx context.Context) (map[string]int64, error) {
	query := `SELECT mt.name, COUNT(mi.id)
		FROM media_types mt
		LEFT JOIN media_items mi ON mi.media_type_id = mt.id
		GROUP BY mt.name
		ORDER BY mt.name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count media items by type: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan type count: %w", err)
		}
		counts[name] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating type counts: %w", err)
	}

	return counts, nil
}

// GetByParent retrieves child media items for a parent with pagination. Returns items and total count.
func (r *MediaItemRepository) GetByParent(ctx context.Context, parentID int64, limit, offset int) ([]*models.MediaItem, int64, error) {
	countQuery := `SELECT COUNT(*) FROM media_items WHERE parent_id = ?`
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, parentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count children: %w", err)
	}

	query := `SELECT id, media_type_id, title, original_title, year, description,
		genre, director, cast_crew, rating, runtime, language, country,
		status, parent_id, season_number, episode_number, track_number,
		first_detected, last_updated
	FROM media_items WHERE parent_id = ?
	ORDER BY season_number ASC, episode_number ASC, track_number ASC, title ASC
	LIMIT ? OFFSET ?`

	items, err := r.queryItems(ctx, query, parentID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetMediaTypes returns all media types.
func (r *MediaItemRepository) GetMediaTypes(ctx context.Context) ([]models.MediaType, error) {
	query := `SELECT id, name, description, detection_patterns, metadata_providers,
		created_at, updated_at
	FROM media_types ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query media types: %w", err)
	}
	defer rows.Close()

	types := []models.MediaType{}
	for rows.Next() {
		var mt models.MediaType
		var detPat, metaProv sql.NullString

		if err := rows.Scan(&mt.ID, &mt.Name, &mt.Description, &detPat, &metaProv, &mt.CreatedAt, &mt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan media type: %w", err)
		}

		if detPat.Valid && detPat.String != "" {
			if err := json.Unmarshal([]byte(detPat.String), &mt.DetectionPatterns); err != nil {
				mt.DetectionPatterns = nil
			}
		}
		if metaProv.Valid && metaProv.String != "" {
			if err := json.Unmarshal([]byte(metaProv.String), &mt.MetadataProviders); err != nil {
				mt.MetadataProviders = nil
			}
		}

		types = append(types, mt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating media types: %w", err)
	}

	return types, nil
}

// GetMediaTypeByName retrieves a media type by its name. Returns the type and its ID.
func (r *MediaItemRepository) GetMediaTypeByName(ctx context.Context, name string) (*models.MediaType, int64, error) {
	query := `SELECT id, name, description, detection_patterns, metadata_providers,
		created_at, updated_at
	FROM media_types WHERE name = ?`

	var mt models.MediaType
	var detPat, metaProv sql.NullString

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&mt.ID, &mt.Name, &mt.Description, &detPat, &metaProv, &mt.CreatedAt, &mt.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, fmt.Errorf("media type not found: %s", name)
		}
		return nil, 0, fmt.Errorf("failed to get media type by name: %w", err)
	}

	if detPat.Valid && detPat.String != "" {
		if err := json.Unmarshal([]byte(detPat.String), &mt.DetectionPatterns); err != nil {
			mt.DetectionPatterns = nil
		}
	}
	if metaProv.Valid && metaProv.String != "" {
		if err := json.Unmarshal([]byte(metaProv.String), &mt.MetadataProviders); err != nil {
			mt.MetadataProviders = nil
		}
	}

	return &mt, mt.ID, nil
}

// --- internal helpers ---

// queryItems executes a query and returns a slice of media items.
func (r *MediaItemRepository) queryItems(ctx context.Context, query string, args ...interface{}) ([]*models.MediaItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query media items: %w", err)
	}
	defer rows.Close()

	items := []*models.MediaItem{}
	for rows.Next() {
		item, err := r.scanItemFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating media items: %w", err)
	}

	return items, nil
}

// scanItem scans a single sql.Row into a MediaItem.
func (r *MediaItemRepository) scanItem(row *sql.Row) (*models.MediaItem, error) {
	item := &models.MediaItem{}
	var genreJSON, castCrewJSON sql.NullString

	err := row.Scan(
		&item.ID, &item.MediaTypeID, &item.Title, &item.OriginalTitle, &item.Year,
		&item.Description, &genreJSON, &item.Director, &castCrewJSON,
		&item.Rating, &item.Runtime, &item.Language, &item.Country,
		&item.Status, &item.ParentID, &item.SeasonNumber, &item.EpisodeNumber, &item.TrackNumber,
		&item.FirstDetected, &item.LastUpdated,
	)
	if err != nil {
		return nil, err
	}

	if genreJSON.Valid && genreJSON.String != "" {
		if err := json.Unmarshal([]byte(genreJSON.String), &item.Genre); err != nil {
			item.Genre = nil
		}
	}
	if castCrewJSON.Valid && castCrewJSON.String != "" {
		if err := json.Unmarshal([]byte(castCrewJSON.String), &item.CastCrew); err != nil {
			item.CastCrew = nil
		}
	}

	return item, nil
}

// scanItemFromRows scans a single row from sql.Rows into a MediaItem.
func (r *MediaItemRepository) scanItemFromRows(rows *sql.Rows) (*models.MediaItem, error) {
	item := &models.MediaItem{}
	var genreJSON, castCrewJSON sql.NullString

	err := rows.Scan(
		&item.ID, &item.MediaTypeID, &item.Title, &item.OriginalTitle, &item.Year,
		&item.Description, &genreJSON, &item.Director, &castCrewJSON,
		&item.Rating, &item.Runtime, &item.Language, &item.Country,
		&item.Status, &item.ParentID, &item.SeasonNumber, &item.EpisodeNumber, &item.TrackNumber,
		&item.FirstDetected, &item.LastUpdated,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan media item: %w", err)
	}

	if genreJSON.Valid && genreJSON.String != "" {
		if err := json.Unmarshal([]byte(genreJSON.String), &item.Genre); err != nil {
			item.Genre = nil
		}
	}
	if castCrewJSON.Valid && castCrewJSON.String != "" {
		if err := json.Unmarshal([]byte(castCrewJSON.String), &item.CastCrew); err != nil {
			item.CastCrew = nil
		}
	}

	return item, nil
}

// marshalJSONField marshals a value to a JSON string pointer for database storage.
// Returns nil if the value is nil or represents an empty collection.
func marshalJSONField(v interface{}) (*string, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case []string:
		if len(val) == 0 {
			return nil, nil
		}
	case *models.CastCrew:
		if val == nil {
			return nil, nil
		}
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	s := string(data)
	return &s, nil
}
