package services

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"catalogizer/models"
	"catalogizer/repository"
)

type FavoritesService struct {
	favoritesRepo *repository.FavoritesRepository
	authService   *AuthService
}

func NewFavoritesService(favoritesRepo *repository.FavoritesRepository, authService *AuthService) *FavoritesService {
	return &FavoritesService{
		favoritesRepo: favoritesRepo,
		authService:   authService,
	}
}

func (s *FavoritesService) AddFavorite(userID int, favorite *models.Favorite) (*models.Favorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	existing, err := s.favoritesRepo.GetFavorite(userID, favorite.EntityType, favorite.EntityID)
	if err == nil && existing != nil {
		return existing, fmt.Errorf("item already in favorites")
	}

	favorite.UserID = userID
	favorite.CreatedAt = time.Now()

	id, err := s.favoritesRepo.CreateFavorite(favorite)
	if err != nil {
		return nil, fmt.Errorf("failed to add favorite: %w", err)
	}

	favorite.ID = id
	return favorite, nil
}

func (s *FavoritesService) RemoveFavorite(userID int, entityType string, entityID int) error {
	if s.favoritesRepo == nil {
		return fmt.Errorf("favorites repository not configured")
	}
	favorite, err := s.favoritesRepo.GetFavorite(userID, entityType, entityID)
	if err != nil {
		return fmt.Errorf("favorite not found: %w", err)
	}
	// Article XI §11.5: GetFavorite returns (nil, nil) on sql.ErrNoRows
	// (see repository/favorites_repository.go::GetFavorite). The previous
	// nil-unsafe `favorite.UserID` deref panicked, the gin recovery
	// middleware swallowed it as 500-with-empty-body. Caught by
	// FQA-API-218 in the 2026-04-29 real-binary bank verification:
	// `DELETE /favorites/movie/999999 -> HTTP 500 (body empty)` with
	// `runtime error: invalid memory address or nil pointer
	// dereference` at favorites_service.go:57.
	if favorite == nil {
		return fmt.Errorf("favorite not found")
	}
	if favorite.UserID != userID {
		return fmt.Errorf("unauthorized to remove this favorite")
	}

	return s.favoritesRepo.DeleteFavorite(favorite.ID)
}

func (s *FavoritesService) GetUserFavorites(userID int, entityType *string, category *string, limit, offset int) ([]models.Favorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	return s.favoritesRepo.GetUserFavorites(userID, entityType, category, limit, offset)
}

func (s *FavoritesService) GetFavoritesByEntity(userID int, entityType string, entityID int) (*models.Favorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	return s.favoritesRepo.GetFavorite(userID, entityType, entityID)
}

func (s *FavoritesService) IsFavorite(userID int, entityType string, entityID int) (bool, error) {
	if s.favoritesRepo == nil {
		return false, fmt.Errorf("favorites repository not configured")
	}
	favorite, err := s.favoritesRepo.GetFavorite(userID, entityType, entityID)
	if err != nil {
		return false, nil
	}
	return favorite != nil, nil
}

func (s *FavoritesService) UpdateFavorite(userID int, favoriteID int, updates *models.UpdateFavoriteRequest) (*models.Favorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	favorite, err := s.favoritesRepo.GetFavoriteByID(favoriteID)
	if err != nil {
		return nil, fmt.Errorf("favorite not found: %w", err)
	}

	if favorite.UserID != userID {
		return nil, fmt.Errorf("unauthorized to update this favorite")
	}

	if updates.Category != nil {
		favorite.Category = updates.Category
	}

	if updates.Notes != nil {
		favorite.Notes = updates.Notes
	}

	if updates.Tags != nil {
		favorite.Tags = updates.Tags
	}

	if updates.IsPublic != nil {
		favorite.IsPublic = *updates.IsPublic
	}

	favorite.UpdatedAt = &time.Time{}
	*favorite.UpdatedAt = time.Now()

	err = s.favoritesRepo.UpdateFavorite(favorite)
	if err != nil {
		return nil, fmt.Errorf("failed to update favorite: %w", err)
	}

	return favorite, nil
}

func (s *FavoritesService) GetFavoriteCategories(userID int, entityType *string) ([]models.FavoriteCategory, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	return s.favoritesRepo.GetFavoriteCategories(userID, entityType)
}

func (s *FavoritesService) CreateFavoriteCategory(userID int, category *models.FavoriteCategory) (*models.FavoriteCategory, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	category.UserID = userID
	category.CreatedAt = time.Now()

	id, err := s.favoritesRepo.CreateFavoriteCategory(category)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	category.ID = id
	return category, nil
}

func (s *FavoritesService) UpdateFavoriteCategory(userID int, categoryID int, updates *models.UpdateFavoriteCategoryRequest) (*models.FavoriteCategory, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	category, err := s.favoritesRepo.GetFavoriteCategoryByID(categoryID)
	if err != nil {
		return nil, fmt.Errorf("category not found: %w", err)
	}

	if category.UserID != userID {
		return nil, fmt.Errorf("unauthorized to update this category")
	}

	if updates.Name != "" {
		category.Name = updates.Name
	}

	if updates.Description != nil {
		category.Description = updates.Description
	}

	if updates.Color != nil {
		category.Color = updates.Color
	}

	if updates.Icon != nil {
		category.Icon = updates.Icon
	}

	if updates.IsPublic != nil {
		category.IsPublic = *updates.IsPublic
	}

	category.UpdatedAt = &time.Time{}
	*category.UpdatedAt = time.Now()

	err = s.favoritesRepo.UpdateFavoriteCategory(category)
	if err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	return category, nil
}

func (s *FavoritesService) DeleteFavoriteCategory(userID int, categoryID int) error {
	if s.favoritesRepo == nil {
		return fmt.Errorf("favorites repository not configured")
	}
	category, err := s.favoritesRepo.GetFavoriteCategoryByID(categoryID)
	if err != nil {
		return fmt.Errorf("category not found: %w", err)
	}

	if category.UserID != userID {
		return fmt.Errorf("unauthorized to delete this category")
	}

	favoritesCount, err := s.favoritesRepo.CountFavoritesByCategory(categoryID)
	if err != nil {
		return fmt.Errorf("failed to check category usage: %w", err)
	}

	if favoritesCount > 0 {
		return fmt.Errorf("cannot delete category with existing favorites")
	}

	return s.favoritesRepo.DeleteFavoriteCategory(categoryID)
}

func (s *FavoritesService) GetPublicFavorites(entityType *string, category *string, limit, offset int) ([]models.Favorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	return s.favoritesRepo.GetPublicFavorites(entityType, category, limit, offset)
}

func (s *FavoritesService) SearchFavorites(userID int, query string, entityType *string, limit, offset int) ([]models.Favorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	return s.favoritesRepo.SearchFavorites(userID, query, entityType, limit, offset)
}

func (s *FavoritesService) GetFavoriteStatistics(userID int) (*models.FavoriteStatistics, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	stats := &models.FavoriteStatistics{
		UserID: userID,
	}

	totalCount, err := s.favoritesRepo.CountUserFavorites(userID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get total favorites count: %w", err)
	}
	stats.TotalFavorites = totalCount

	entityTypeCounts, err := s.favoritesRepo.GetFavoritesCountByEntityType(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity type counts: %w", err)
	}
	stats.FavoritesByEntityType = entityTypeCounts

	categoryCounts, err := s.favoritesRepo.GetFavoritesCountByCategory(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get category counts: %w", err)
	}
	stats.FavoritesByCategory = categoryCounts

	recentFavorites, err := s.favoritesRepo.GetRecentFavorites(userID, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent favorites: %w", err)
	}
	stats.RecentFavorites = recentFavorites

	return stats, nil
}

func (s *FavoritesService) GetRecommendedFavorites(userID int, limit int) ([]models.RecommendedFavorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	userFavorites, err := s.favoritesRepo.GetUserFavorites(userID, nil, nil, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get user favorites: %w", err)
	}

	var entityTypes []string

	for _, favorite := range userFavorites {
		entityTypes = append(entityTypes, favorite.EntityType)
	}

	entityTypes = s.removeDuplicateStrings(entityTypes)

	recommendations := []models.RecommendedFavorite{}

	for _, entityType := range entityTypes {
		similarFavorites, err := s.favoritesRepo.GetSimilarFavorites(userID, entityType, limit/len(entityTypes))
		if err != nil {
			continue
		}

		for _, favorite := range similarFavorites {
			recommendation := models.RecommendedFavorite{
				Favorite:        favorite,
				RecommendReason: fmt.Sprintf("Based on your interest in %s", entityType),
				RecommendScore:  0.8,
				RecommendedAt:   time.Now(),
			}
			recommendations = append(recommendations, recommendation)
		}
	}

	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	return recommendations, nil
}

func (s *FavoritesService) ShareFavorite(userID int, favoriteID int, shareWith []int, permissions models.SharePermissions) (*models.FavoriteShare, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	favorite, err := s.favoritesRepo.GetFavoriteByID(favoriteID)
	if err != nil {
		return nil, fmt.Errorf("favorite not found: %w", err)
	}

	if favorite.UserID != userID {
		return nil, fmt.Errorf("unauthorized to share this favorite")
	}

	share := &models.FavoriteShare{
		FavoriteID:   favoriteID,
		SharedByUser: userID,
		SharedWith:   shareWith,
		Permissions:  permissions,
		CreatedAt:    time.Now(),
		IsActive:     true,
	}

	id, err := s.favoritesRepo.CreateFavoriteShare(share)
	if err != nil {
		return nil, fmt.Errorf("failed to create favorite share: %w", err)
	}

	share.ID = id
	return share, nil
}

func (s *FavoritesService) GetSharedFavorites(userID int, limit, offset int) ([]models.Favorite, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	return s.favoritesRepo.GetSharedFavorites(userID, limit, offset)
}

func (s *FavoritesService) RevokeFavoriteShare(userID int, shareID int) error {
	if s.favoritesRepo == nil {
		return fmt.Errorf("favorites repository not configured")
	}
	share, err := s.favoritesRepo.GetFavoriteShareByID(shareID)
	if err != nil {
		return fmt.Errorf("share not found: %w", err)
	}

	if share.SharedByUser != userID {
		return fmt.Errorf("unauthorized to revoke this share")
	}

	return s.favoritesRepo.RevokeFavoriteShare(shareID)
}

func (s *FavoritesService) BulkAddFavorites(userID int, favorites []models.BulkFavoriteRequest) ([]models.Favorite, error) {
	results := []models.Favorite{}
	var errors []error

	for _, req := range favorites {
		favorite := &models.Favorite{
			EntityType: req.EntityType,
			EntityID:   req.EntityID,
			Category:   req.Category,
			Notes:      req.Notes,
			Tags:       req.Tags,
			IsPublic:   req.IsPublic,
		}

		result, err := s.AddFavorite(userID, favorite)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		results = append(results, *result)
	}

	if len(errors) > 0 && len(results) == 0 {
		return nil, fmt.Errorf("failed to add any favorites: %v", errors)
	}

	return results, nil
}

func (s *FavoritesService) BulkRemoveFavorites(userID int, favorites []models.BulkFavoriteRemoveRequest) error {
	var errors []error

	for _, req := range favorites {
		err := s.RemoveFavorite(userID, req.EntityType, req.EntityID)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to remove some favorites: %v", errors)
	}

	return nil
}

func (s *FavoritesService) ExportFavorites(userID int, format string) ([]byte, error) {
	if s.favoritesRepo == nil {
		return nil, fmt.Errorf("favorites repository not configured")
	}
	favorites, err := s.favoritesRepo.GetUserFavorites(userID, nil, nil, 10000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get user favorites: %w", err)
	}

	switch format {
	case "json":
		return s.exportFavoritesToJSON(favorites)
	case "csv":
		return s.exportFavoritesToCSV(favorites)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

func (s *FavoritesService) ImportFavorites(userID int, data []byte, format string) ([]models.Favorite, error) {
	switch format {
	case "json":
		return s.importFavoritesFromJSON(userID, data)
	case "csv":
		return s.importFavoritesFromCSV(userID, data)
	default:
		return nil, fmt.Errorf("unsupported import format: %s", format)
	}
}

func (s *FavoritesService) removeDuplicateStrings(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

func (s *FavoritesService) exportFavoritesToJSON(favorites []models.Favorite) ([]byte, error) {
	// Create export structure with metadata
	export := struct {
		Version    string                   `json:"version"`
		ExportedAt time.Time                `json:"exported_at"`
		Count      int                      `json:"count"`
		Favorites  []models.Favorite        `json:"favorites"`
		Categories []map[string]interface{} `json:"categories,omitempty"`
	}{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Count:      len(favorites),
		Favorites:  favorites,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal favorites to JSON: %w", err)
	}

	return data, nil
}

func (s *FavoritesService) exportFavoritesToCSV(favorites []models.Favorite) ([]byte, error) {
	// Create CSV writer
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)

	// Write header
	headers := []string{
		"ID", "UserID", "EntityType", "EntityID", "Category", "Notes",
		"Tags", "IsPublic", "CreatedAt", "UpdatedAt",
	}
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write data rows
	for _, favorite := range favorites {
		// Handle nullable fields
		category := ""
		if favorite.Category != nil {
			category = *favorite.Category
		}

		notes := ""
		if favorite.Notes != nil {
			notes = *favorite.Notes
		}

		tags := ""
		if favorite.Tags != nil {
			tags = fmt.Sprintf("%v", *favorite.Tags)
		}

		updatedAt := ""
		if favorite.UpdatedAt != nil {
			updatedAt = favorite.UpdatedAt.Format(time.RFC3339)
		}

		record := []string{
			fmt.Sprintf("%d", favorite.ID),
			fmt.Sprintf("%d", favorite.UserID),
			favorite.EntityType,
			fmt.Sprintf("%d", favorite.EntityID),
			category,
			notes,
			tags,
			fmt.Sprintf("%t", favorite.IsPublic),
			favorite.CreatedAt.Format(time.RFC3339),
			updatedAt,
		}

		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	// Flush writer
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return buffer.Bytes(), nil
}

func (s *FavoritesService) importFavoritesFromJSON(userID int, data []byte) ([]models.Favorite, error) {
	// Parse import data
	var importData struct {
		Version    string            `json:"version"`
		ExportedAt time.Time         `json:"exported_at"`
		Count      int               `json:"count"`
		Favorites  []models.Favorite `json:"favorites"`
	}

	if err := json.Unmarshal(data, &importData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	if len(importData.Favorites) == 0 {
		return []models.Favorite{}, nil
	}

	// Process each favorite
	var importedFavorites []models.Favorite
	var errors []error

	for _, fav := range importData.Favorites {
		// Create new favorite with current user ID
		newFavorite := &models.Favorite{
			UserID:     userID, // Override with current user
			EntityType: fav.EntityType,
			EntityID:   fav.EntityID,
			Category:   fav.Category,
			Notes:      fav.Notes,
			Tags:       fav.Tags,
			IsPublic:   fav.IsPublic,
		}

		// Try to add favorite (will skip duplicates)
		result, err := s.AddFavorite(userID, newFavorite)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to import favorite %s %d: %w", fav.EntityType, fav.EntityID, err))
			continue
		}

		importedFavorites = append(importedFavorites, *result)
	}

	// If all failed, return error
	if len(importedFavorites) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("failed to import any favorites: %v", errors)
	}

	return importedFavorites, nil
}

func (s *FavoritesService) importFavoritesFromCSV(userID int, data []byte) ([]models.Favorite, error) {
	// Parse CSV data
	reader := csv.NewReader(bytes.NewReader(data))

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Validate headers (basic check)
	if len(headers) < 9 {
		return nil, fmt.Errorf("invalid CSV format: expected at least 9 columns, got %d", len(headers))
	}

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV records: %w", err)
	}

	var importedFavorites []models.Favorite
	var errors []error

	for i, record := range records {
		if len(record) < 9 {
			errors = append(errors, fmt.Errorf("record %d: insufficient columns", i+1))
			continue
		}

		// Parse entity type and ID
		entityType := record[2]
		entityIDStr := record[3]

		var entityID int
		if _, err := fmt.Sscanf(entityIDStr, "%d", &entityID); err != nil {
			errors = append(errors, fmt.Errorf("record %d: invalid entity ID: %s", i+1, entityIDStr))
			continue
		}

		// Parse optional fields
		var category *string
		if record[4] != "" {
			category = &record[4]
		}

		var notes *string
		if record[5] != "" {
			notes = &record[5]
		}

		var tags *[]string
		if record[6] != "" {
			// Simple comma-separated tags parsing
			tagList := strings.Split(record[6], ",")
			for i := range tagList {
				tagList[i] = strings.TrimSpace(tagList[i])
			}
			tags = &tagList
		}

		var isPublic bool
		if record[7] != "" {
			if _, err := fmt.Sscanf(record[7], "%t", &isPublic); err != nil {
				errors = append(errors, fmt.Errorf("record %d: invalid is_public value: %s", i+1, record[7]))
				continue
			}
		}

		// Create favorite
		favorite := &models.Favorite{
			UserID:     userID,
			EntityType: entityType,
			EntityID:   entityID,
			Category:   category,
			Notes:      notes,
			Tags:       tags,
			IsPublic:   isPublic,
		}

		// Add favorite
		result, err := s.AddFavorite(userID, favorite)
		if err != nil {
			errors = append(errors, fmt.Errorf("record %d: failed to add favorite: %w", i+1, err))
			continue
		}

		importedFavorites = append(importedFavorites, *result)
	}

	// If all failed, return error
	if len(importedFavorites) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("failed to import any favorites: %v", errors)
	}

	return importedFavorites, nil
}
