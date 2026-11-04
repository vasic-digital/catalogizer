package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"catalogizer/database"
	"catalogizer/models"
)

type FavoritesRepository struct {
	db *database.DB
}

func NewFavoritesRepository(db *database.DB) *FavoritesRepository {
	return &FavoritesRepository{db: db}
}

func (r *FavoritesRepository) CreateFavorite(favorite *models.Favorite) (int, error) {
	query := `
		INSERT INTO favorites (user_id, entity_type, entity_id, category, notes, tags, is_public, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	var tagsJSON *string
	if favorite.Tags != nil && len(*favorite.Tags) > 0 {
		data, err := json.Marshal(*favorite.Tags)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal tags: %w", err)
		}
		tagsStr := string(data)
		tagsJSON = &tagsStr
	}

	id, err := r.db.InsertReturningID(context.Background(), query,
		favorite.UserID, favorite.EntityType, favorite.EntityID,
		favorite.Category, favorite.Notes, tagsJSON, favorite.IsPublic, favorite.CreatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create favorite: %w", err)
	}

	return int(id), nil
}

func (r *FavoritesRepository) GetFavorite(userID int, entityType string, entityID int) (*models.Favorite, error) {
	query := `
		SELECT id, user_id, entity_type, entity_id, category, notes, tags, is_public, created_at, updated_at
		FROM favorites
		WHERE user_id = ? AND entity_type = ? AND entity_id = ?
	`

	favorite := &models.Favorite{}
	var tagsJSON sql.NullString
	var updatedAt sql.NullTime

	err := r.db.QueryRow(query, userID, entityType, entityID).Scan(
		&favorite.ID, &favorite.UserID, &favorite.EntityType, &favorite.EntityID,
		&favorite.Category, &favorite.Notes, &tagsJSON, &favorite.IsPublic,
		&favorite.CreatedAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get favorite: %w", err)
	}

	if tagsJSON.Valid {
		tags := []string{}
		if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err == nil {
			if tags == nil { // stored JSON "null"
				tags = []string{}
			}
			favorite.Tags = &tags
		}
	}

	if updatedAt.Valid {
		favorite.UpdatedAt = &updatedAt.Time
	}

	return favorite, nil
}

func (r *FavoritesRepository) GetFavoriteByID(favoriteID int) (*models.Favorite, error) {
	query := `
		SELECT id, user_id, entity_type, entity_id, category, notes, tags, is_public, created_at, updated_at
		FROM favorites
		WHERE id = ?
	`

	favorite := &models.Favorite{}
	var tagsJSON sql.NullString
	var updatedAt sql.NullTime

	err := r.db.QueryRow(query, favoriteID).Scan(
		&favorite.ID, &favorite.UserID, &favorite.EntityType, &favorite.EntityID,
		&favorite.Category, &favorite.Notes, &tagsJSON, &favorite.IsPublic,
		&favorite.CreatedAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("favorite not found")
		}
		return nil, fmt.Errorf("failed to get favorite: %w", err)
	}

	if tagsJSON.Valid {
		tags := []string{}
		if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err == nil {
			if tags == nil { // stored JSON "null"
				tags = []string{}
			}
			favorite.Tags = &tags
		}
	}

	if updatedAt.Valid {
		favorite.UpdatedAt = &updatedAt.Time
	}

	return favorite, nil
}

func (r *FavoritesRepository) UpdateFavorite(favorite *models.Favorite) error {
	query := `
		UPDATE favorites
		SET category = ?, notes = ?, tags = ?, is_public = ?, updated_at = ?
		WHERE id = ?
	`

	var tagsJSON *string
	if favorite.Tags != nil && len(*favorite.Tags) > 0 {
		data, err := json.Marshal(*favorite.Tags)
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
		tagsStr := string(data)
		tagsJSON = &tagsStr
	}

	_, err := r.db.Exec(query,
		favorite.Category, favorite.Notes, tagsJSON, favorite.IsPublic,
		favorite.UpdatedAt, favorite.ID)

	return err
}

func (r *FavoritesRepository) DeleteFavorite(favoriteID int) error {
	query := `DELETE FROM favorites WHERE id = ?`
	_, err := r.db.Exec(query, favoriteID)
	return err
}

func (r *FavoritesRepository) GetUserFavorites(userID int, entityType *string, category *string, limit, offset int) ([]models.Favorite, error) {
	whereClause := "WHERE user_id = ?"
	args := []interface{}{userID}

	if entityType != nil {
		whereClause += " AND entity_type = ?"
		args = append(args, *entityType)
	}

	if category != nil {
		whereClause += " AND category = ?"
		args = append(args, *category)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, entity_type, entity_id, category, notes, tags, is_public, created_at, updated_at
		FROM favorites
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get user favorites: %w", err)
	}
	defer rows.Close()

	return r.scanFavorites(rows)
}

func (r *FavoritesRepository) GetPublicFavorites(entityType *string, category *string, limit, offset int) ([]models.Favorite, error) {
	whereClause := "WHERE is_public = 1"
	args := []interface{}{}

	if entityType != nil {
		whereClause += " AND entity_type = ?"
		args = append(args, *entityType)
	}

	if category != nil {
		whereClause += " AND category = ?"
		args = append(args, *category)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, entity_type, entity_id, category, notes, tags, is_public, created_at, updated_at
		FROM favorites
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get public favorites: %w", err)
	}
	defer rows.Close()

	return r.scanFavorites(rows)
}

func (r *FavoritesRepository) SearchFavorites(userID int, query string, entityType *string, limit, offset int) ([]models.Favorite, error) {
	searchPattern := "%" + query + "%"
	whereClause := "WHERE user_id = ? AND (notes LIKE ? OR tags LIKE ?)"
	args := []interface{}{userID, searchPattern, searchPattern}

	if entityType != nil {
		whereClause += " AND entity_type = ?"
		args = append(args, *entityType)
	}

	sqlQuery := fmt.Sprintf(`
		SELECT id, user_id, entity_type, entity_id, category, notes, tags, is_public, created_at, updated_at
		FROM favorites
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)

	rows, err := r.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search favorites: %w", err)
	}
	defer rows.Close()

	return r.scanFavorites(rows)
}

func (r *FavoritesRepository) CountUserFavorites(userID int, entityType *string) (int, error) {
	whereClause := "WHERE user_id = ?"
	args := []interface{}{userID}

	if entityType != nil {
		whereClause += " AND entity_type = ?"
		args = append(args, *entityType)
	}

	// Safe: whereClause is built from hardcoded SQL fragments with parameterized ? placeholders; user values are passed via args
	query := fmt.Sprintf("SELECT COUNT(*) FROM favorites %s", whereClause)

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func (r *FavoritesRepository) GetFavoritesCountByEntityType(userID int) (map[string]int, error) {
	query := `
		SELECT entity_type, COUNT(*) as count
		FROM favorites
		WHERE user_id = ?
		GROUP BY entity_type
		ORDER BY count DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorites count by entity type: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var entityType string
		var count int
		err := rows.Scan(&entityType, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entity type count: %w", err)
		}
		counts[entityType] = count
	}

	return counts, nil
}

func (r *FavoritesRepository) GetFavoritesCountByCategory(userID int) (map[string]int, error) {
	query := `
		SELECT COALESCE(category, 'uncategorized') as category, COUNT(*) as count
		FROM favorites
		WHERE user_id = ?
		GROUP BY category
		ORDER BY count DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorites count by category: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		err := rows.Scan(&category, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category count: %w", err)
		}
		counts[category] = count
	}

	return counts, nil
}

func (r *FavoritesRepository) GetRecentFavorites(userID int, limit int) ([]models.Favorite, error) {
	query := `
		SELECT id, user_id, entity_type, entity_id, category, notes, tags, is_public, created_at, updated_at
		FROM favorites
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent favorites: %w", err)
	}
	defer rows.Close()

	return r.scanFavorites(rows)
}

func (r *FavoritesRepository) GetSimilarFavorites(userID int, entityType string, limit int) ([]models.Favorite, error) {
	query := `
		SELECT id, user_id, entity_type, entity_id, category, notes, tags, is_public, created_at, updated_at
		FROM favorites
		WHERE user_id != ? AND entity_type = ? AND is_public = 1
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, userID, entityType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get similar favorites: %w", err)
	}
	defer rows.Close()

	return r.scanFavorites(rows)
}

func (r *FavoritesRepository) CreateFavoriteCategory(category *models.FavoriteCategory) (int, error) {
	query := `
		INSERT INTO favorite_categories (user_id, name, description, color, icon, is_public, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	id, err := r.db.InsertReturningID(context.Background(), query,
		category.UserID, category.Name, category.Description, category.Color,
		category.Icon, category.IsPublic, category.CreatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create favorite category: %w", err)
	}

	return int(id), nil
}

func (r *FavoritesRepository) GetFavoriteCategories(userID int, entityType *string) ([]models.FavoriteCategory, error) {
	whereClause := "WHERE user_id = ?"
	args := []interface{}{userID}

	if entityType != nil {
		whereClause += " AND entity_type = ?"
		args = append(args, *entityType)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, name, description, color, icon, is_public, created_at, updated_at
		FROM favorite_categories
		%s
		ORDER BY name ASC
	`, whereClause)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get favorite categories: %w", err)
	}
	defer rows.Close()

	return r.scanFavoriteCategories(rows)
}

func (r *FavoritesRepository) GetFavoriteCategoryByID(categoryID int) (*models.FavoriteCategory, error) {
	query := `
		SELECT id, user_id, name, description, color, icon, is_public, created_at, updated_at
		FROM favorite_categories
		WHERE id = ?
	`

	category := &models.FavoriteCategory{}
	var updatedAt sql.NullTime

	err := r.db.QueryRow(query, categoryID).Scan(
		&category.ID, &category.UserID, &category.Name, &category.Description,
		&category.Color, &category.Icon, &category.IsPublic, &category.CreatedAt, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found")
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	if updatedAt.Valid {
		category.UpdatedAt = &updatedAt.Time
	}

	return category, nil
}

func (r *FavoritesRepository) UpdateFavoriteCategory(category *models.FavoriteCategory) error {
	query := `
		UPDATE favorite_categories
		SET name = ?, description = ?, color = ?, icon = ?, is_public = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		category.Name, category.Description, category.Color, category.Icon,
		category.IsPublic, category.UpdatedAt, category.ID)

	return err
}

func (r *FavoritesRepository) DeleteFavoriteCategory(categoryID int) error {
	query := `DELETE FROM favorite_categories WHERE id = ?`
	_, err := r.db.Exec(query, categoryID)
	return err
}

func (r *FavoritesRepository) CountFavoritesByCategory(categoryID int) (int, error) {
	query := `SELECT COUNT(*) FROM favorites WHERE category = (SELECT name FROM favorite_categories WHERE id = ?)`
	var count int
	err := r.db.QueryRow(query, categoryID).Scan(&count)
	return count, err
}

func (r *FavoritesRepository) CreateFavoriteShare(share *models.FavoriteShare) (int, error) {
	query := `
		INSERT INTO favorite_shares (favorite_id, shared_by_user, shared_with, permissions, created_at, is_active)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	sharedWithJSON, err := json.Marshal(share.SharedWith)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal shared_with: %w", err)
	}

	permissionsJSON, err := json.Marshal(share.Permissions)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal permissions: %w", err)
	}

	id, err := r.db.InsertReturningID(context.Background(), query,
		share.FavoriteID, share.SharedByUser, string(sharedWithJSON),
		string(permissionsJSON), share.CreatedAt, share.IsActive)

	if err != nil {
		return 0, fmt.Errorf("failed to create favorite share: %w", err)
	}

	return int(id), nil
}

func (r *FavoritesRepository) GetFavoriteShareByID(shareID int) (*models.FavoriteShare, error) {
	query := `
		SELECT id, favorite_id, shared_by_user, shared_with, permissions, created_at, is_active
		FROM favorite_shares
		WHERE id = ?
	`

	share := &models.FavoriteShare{}
	var sharedWithJSON, permissionsJSON string

	err := r.db.QueryRow(query, shareID).Scan(
		&share.ID, &share.FavoriteID, &share.SharedByUser, &sharedWithJSON,
		&permissionsJSON, &share.CreatedAt, &share.IsActive)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("share not found")
		}
		return nil, fmt.Errorf("failed to get share: %w", err)
	}

	if err := json.Unmarshal([]byte(sharedWithJSON), &share.SharedWith); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shared_with: %w", err)
	}

	if err := json.Unmarshal([]byte(permissionsJSON), &share.Permissions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
	}

	return share, nil
}

func (r *FavoritesRepository) GetSharedFavorites(userID int, limit, offset int) ([]models.Favorite, error) {
	query := `
		SELECT f.id, f.user_id, f.entity_type, f.entity_id, f.category, f.notes, f.tags, f.is_public, f.created_at, f.updated_at
		FROM favorites f
		INNER JOIN favorite_shares fs ON f.id = fs.favorite_id
		WHERE JSON_EXTRACT(fs.shared_with, '$') LIKE ? AND fs.is_active = 1
		ORDER BY f.created_at DESC
		LIMIT ? OFFSET ?
	`

	userIDPattern := fmt.Sprintf("%%\"%d\"%%", userID)

	rows, err := r.db.Query(query, userIDPattern, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared favorites: %w", err)
	}
	defer rows.Close()

	return r.scanFavorites(rows)
}

func (r *FavoritesRepository) RevokeFavoriteShare(shareID int) error {
	query := `UPDATE favorite_shares SET is_active = 0 WHERE id = ?`
	_, err := r.db.Exec(query, shareID)
	return err
}

func (r *FavoritesRepository) scanFavorites(rows *sql.Rows) ([]models.Favorite, error) {
	favorites := []models.Favorite{}

	for rows.Next() {
		var favorite models.Favorite
		var tagsJSON sql.NullString
		var updatedAt sql.NullTime

		err := rows.Scan(
			&favorite.ID, &favorite.UserID, &favorite.EntityType, &favorite.EntityID,
			&favorite.Category, &favorite.Notes, &tagsJSON, &favorite.IsPublic,
			&favorite.CreatedAt, &updatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan favorite: %w", err)
		}

		if tagsJSON.Valid {
			tags := []string{}
			if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err == nil {
				if tags == nil { // stored JSON "null"
					tags = []string{}
				}
				favorite.Tags = &tags
			}
		}

		if updatedAt.Valid {
			favorite.UpdatedAt = &updatedAt.Time
		}

		favorites = append(favorites, favorite)
	}

	return favorites, nil
}

func (r *FavoritesRepository) scanFavoriteCategories(rows *sql.Rows) ([]models.FavoriteCategory, error) {
	categories := []models.FavoriteCategory{}

	for rows.Next() {
		var category models.FavoriteCategory
		var updatedAt sql.NullTime

		err := rows.Scan(
			&category.ID, &category.UserID, &category.Name, &category.Description,
			&category.Color, &category.Icon, &category.IsPublic, &category.CreatedAt, &updatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		if updatedAt.Valid {
			category.UpdatedAt = &updatedAt.Time
		}

		categories = append(categories, category)
	}

	return categories, nil
}
