package repository

import (
	"catalogizer/database"
	"context"
	"fmt"
)

// DuplicateEntityGroup represents a group of duplicate media items.
type DuplicateEntityGroup struct {
	Title     string  `json:"title"`
	MediaType string  `json:"media_type"`
	Year      *int    `json:"year,omitempty"`
	Count     int     `json:"count"`
	EntityIDs []int64 `json:"entity_ids"`
}

// DuplicateEntityRepository handles duplicate entity detection.
type DuplicateEntityRepository struct {
	db *database.DB
}

// NewDuplicateEntityRepository creates a new duplicate entity repository.
func NewDuplicateEntityRepository(db *database.DB) *DuplicateEntityRepository {
	return &DuplicateEntityRepository{db: db}
}

// GetDuplicateGroups finds all groups of media items with matching title + type.
func (r *DuplicateEntityRepository) GetDuplicateGroups(ctx context.Context, limit, offset int) ([]DuplicateEntityGroup, int64, error) {
	// Count total groups
	countQuery := `SELECT COUNT(*) FROM (
		SELECT mi.title, mt.name
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		GROUP BY mi.title, mt.name, mi.year
		HAVING COUNT(*) > 1
	) AS dup_groups`

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count duplicate groups: %w", err)
	}

	// Get groups
	query := `SELECT mi.title, mt.name, mi.year, COUNT(*) as cnt
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		GROUP BY mi.title, mt.name, mi.year
		HAVING COUNT(*) > 1
		ORDER BY cnt DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query duplicate groups: %w", err)
	}
	defer rows.Close()

	// Buffer the page and release the cursor (and its pooled connection) BEFORE the per-group
	// id queries: a query issued while this cursor is open needs a second pooled connection, so
	// concurrent calls exceeding the pool would all hold one and wait for another (pool deadlock).
	page := []DuplicateEntityGroup{}
	for rows.Next() {
		var g DuplicateEntityGroup
		if err := rows.Scan(&g.Title, &g.MediaType, &g.Year, &g.Count); err != nil {
			return nil, 0, fmt.Errorf("failed to scan duplicate group: %w", err)
		}
		page = append(page, g)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()

	groups := []DuplicateEntityGroup{}
	for _, g := range page {
		// Get entity IDs for this group
		idQuery := `SELECT mi.id FROM media_items mi
			JOIN media_types mt ON mi.media_type_id = mt.id
			WHERE mi.title = ? AND mt.name = ?`
		args := []interface{}{g.Title, g.MediaType}
		if g.Year != nil {
			idQuery += " AND mi.year = ?"
			args = append(args, *g.Year)
		} else {
			idQuery += " AND mi.year IS NULL"
		}

		idRows, err := r.db.QueryContext(ctx, idQuery, args...)
		if err != nil {
			continue
		}
		for idRows.Next() {
			var id int64
			if err := idRows.Scan(&id); err != nil {
				continue
			}
			g.EntityIDs = append(g.EntityIDs, id)
		}
		idRows.Close()

		groups = append(groups, g)
	}

	return groups, total, nil
}

// CountDuplicates returns the total number of entities that have duplicates.
func (r *DuplicateEntityRepository) CountDuplicates(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM (
		SELECT mi.title, mt.name
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		GROUP BY mi.title, mt.name, mi.year
		HAVING COUNT(*) > 1
	) AS dup_groups`

	var count int64
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count duplicates: %w", err)
	}
	return count, nil
}
