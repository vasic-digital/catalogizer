package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"catalogizer/database"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
)

// MediaQueryHandler provides media query and auth endpoints that were previously stubs.
// It replaces the old StubHandler with real database-backed implementations.
type MediaQueryHandler struct {
	db       *database.DB
	itemRepo *repository.MediaItemRepository
	userRepo *repository.UserRepository
}

// NewMediaQueryHandler creates a new media query handler with database dependencies.
func NewMediaQueryHandler(db *database.DB, itemRepo *repository.MediaItemRepository, userRepo *repository.UserRepository) *MediaQueryHandler {
	return &MediaQueryHandler{
		db:       db,
		itemRepo: itemRepo,
		userRepo: userRepo,
	}
}

// GetRecentMedia returns media items ordered by last_updated descending.
// GET /api/v1/media/recent
func (h *MediaQueryHandler) GetRecentMedia(c *gin.Context) {
	ctx := c.Request.Context()

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM media_items`
	var total int64
	if err := h.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count media items"})
		return
	}

	query := `SELECT mi.id, mi.media_type_id, mt.name, mi.title, mi.original_title,
		mi.year, mi.description, mi.rating, mi.runtime, mi.status, mi.last_updated
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		ORDER BY mi.last_updated DESC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query recent media"})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id, mediaTypeID int64
		var typeName, title, status string
		var originalTitle, description sql.NullString
		var year, runtime sql.NullInt64
		var rating sql.NullFloat64
		var lastUpdated time.Time

		if err := rows.Scan(&id, &mediaTypeID, &typeName, &title, &originalTitle,
			&year, &description, &rating, &runtime, &status, &lastUpdated); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan media item"})
			return
		}

		item := gin.H{
			"id":            id,
			"media_type_id": mediaTypeID,
			"type_name":     typeName,
			"title":         title,
			"status":        status,
			"last_updated":  lastUpdated,
		}
		if originalTitle.Valid {
			item["original_title"] = originalTitle.String
		}
		if year.Valid {
			item["year"] = year.Int64
		}
		if description.Valid {
			item["description"] = description.String
		}
		if rating.Valid {
			item["rating"] = rating.Float64
		}
		if runtime.Valid {
			item["runtime"] = runtime.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to iterate media items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

// GetPopularMedia returns media items ordered by number of favorites.
// GET /api/v1/media/popular
func (h *MediaQueryHandler) GetPopularMedia(c *gin.Context) {
	ctx := c.Request.Context()

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM media_items`
	var total int64
	if err := h.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count media items"})
		return
	}

	query := `SELECT mi.id, mi.media_type_id, mt.name, mi.title, mi.original_title,
		mi.year, mi.description, mi.rating, mi.runtime, mi.status, mi.last_updated,
		COUNT(f.id) as fav_count
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		LEFT JOIN favorites f ON f.entity_type = 'media_item' AND f.entity_id = mi.id
		GROUP BY mi.id, mi.media_type_id, mt.name, mi.title, mi.original_title,
			mi.year, mi.description, mi.rating, mi.runtime, mi.status, mi.last_updated
		ORDER BY fav_count DESC, mi.last_updated DESC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query popular media"})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id, mediaTypeID int64
		var typeName, title, status string
		var originalTitle, description sql.NullString
		var year, runtime sql.NullInt64
		var rating sql.NullFloat64
		var lastUpdated time.Time
		var favCount int64

		if err := rows.Scan(&id, &mediaTypeID, &typeName, &title, &originalTitle,
			&year, &description, &rating, &runtime, &status, &lastUpdated, &favCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan media item"})
			return
		}

		item := gin.H{
			"id":             id,
			"media_type_id":  mediaTypeID,
			"type_name":      typeName,
			"title":          title,
			"status":         status,
			"last_updated":   lastUpdated,
			"favorite_count": favCount,
		}
		if originalTitle.Valid {
			item["original_title"] = originalTitle.String
		}
		if year.Valid {
			item["year"] = year.Int64
		}
		if description.Valid {
			item["description"] = description.String
		}
		if rating.Valid {
			item["rating"] = rating.Float64
		}
		if runtime.Valid {
			item["runtime"] = runtime.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to iterate media items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

// GetMediaByPath returns media items whose associated files match a path prefix.
// GET /api/v1/media/by-path
func (h *MediaQueryHandler) GetMediaByPath(c *gin.Context) {
	ctx := c.Request.Context()

	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path parameter is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	pathPattern := path + "%"

	countQuery := `SELECT COUNT(DISTINCT mi.id) FROM media_items mi
		JOIN media_files mf ON mf.media_item_id = mi.id
		JOIN files f ON mf.file_id = f.id
		WHERE f.path LIKE ?`
	var total int64
	if err := h.db.QueryRowContext(ctx, countQuery, pathPattern).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count media by path"})
		return
	}

	query := `SELECT DISTINCT mi.id, mi.media_type_id, mt.name, mi.title, mi.original_title,
		mi.year, mi.description, mi.rating, mi.runtime, mi.status, mi.last_updated
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		JOIN media_files mf ON mf.media_item_id = mi.id
		JOIN files f ON mf.file_id = f.id
		WHERE f.path LIKE ?
		ORDER BY mi.title ASC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query, pathPattern, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query media by path"})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id, mediaTypeID int64
		var typeName, title, status string
		var originalTitle, description sql.NullString
		var year, runtime sql.NullInt64
		var rating sql.NullFloat64
		var lastUpdated time.Time

		if err := rows.Scan(&id, &mediaTypeID, &typeName, &title, &originalTitle,
			&year, &description, &rating, &runtime, &status, &lastUpdated); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan media item"})
			return
		}

		item := gin.H{
			"id":            id,
			"media_type_id": mediaTypeID,
			"type_name":     typeName,
			"title":         title,
			"status":        status,
			"last_updated":  lastUpdated,
		}
		if originalTitle.Valid {
			item["original_title"] = originalTitle.String
		}
		if year.Valid {
			item["year"] = year.Int64
		}
		if description.Valid {
			item["description"] = description.String
		}
		if rating.Valid {
			item["rating"] = rating.Float64
		}
		if runtime.Valid {
			item["runtime"] = runtime.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to iterate media items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

// RefreshMediaMetadata answers 501: no metadata refresher exists for media items.
// POST /api/v1/media/:id/refresh
//
// CAT-001: this handler used to bump last_updated and answer 202 "Metadata
// refresh queued", but nothing reads last_updated to re-enrich, so no refresh
// was ever queued. The id is still validated (400/404) so callers get the real
// error for a bad request; a valid request gets an honest 501.
func (h *MediaQueryHandler) RefreshMediaMetadata(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid media item ID"})
		return
	}

	item, err := h.itemRepo.GetByID(ctx, id)
	if err != nil || item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "media item not found"})
		return
	}

	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "metadata refresh is not implemented: no component re-fetches metadata for a media item. " +
			"Nothing was queued or changed. Use POST /api/v1/entities/:id/metadata/refresh for entity cover discovery.",
	})
}

// GetMediaQuality returns quality information about a media item's associated files.
// GET /api/v1/media/:id/quality
func (h *MediaQueryHandler) GetMediaQuality(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid media item ID"})
		return
	}

	// Check if the media item exists
	item, err := h.itemRepo.GetByID(ctx, id)
	if err != nil || item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "media item not found"})
		return
	}

	// Query associated files
	query := `SELECT f.path, f.size, f.mime_type
		FROM files f
		JOIN media_files mf ON mf.file_id = f.id
		WHERE mf.media_item_id = ?`

	rows, err := h.db.QueryContext(ctx, query, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query media files"})
		return
	}
	defer rows.Close()

	files := make([]gin.H, 0)
	var totalSize int64
	for rows.Next() {
		var path string
		var size int64
		var mimeType sql.NullString

		if err := rows.Scan(&path, &size, &mimeType); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan file info"})
			return
		}

		fileInfo := gin.H{
			"path": path,
			"size": size,
		}
		if mimeType.Valid {
			fileInfo["mime_type"] = mimeType.String
		}
		files = append(files, fileInfo)
		totalSize += size
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to iterate files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id": id,
		"quality": gin.H{
			"files":      files,
			"total_size": totalSize,
		},
	})
}

// analyzeRequest represents the request body for the AnalyzeMedia endpoint.
type analyzeRequest struct {
	Path string `json:"path"`
	ID   *int64 `json:"id"`
}

// AnalyzeMedia analyzes a media item by path or ID and returns file and metadata information.
// POST /api/v1/media/analyze
func (h *MediaQueryHandler) AnalyzeMedia(c *gin.Context) {
	ctx := c.Request.Context()

	var req analyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: path or id required"})
		return
	}

	if req.Path == "" && req.ID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "either path or id must be provided"})
		return
	}

	if req.ID != nil {
		// Analyze by media item ID
		item, err := h.itemRepo.GetByID(ctx, *req.ID)
		if err != nil || item == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "media item not found"})
			return
		}

		// Get associated files
		filesQuery := `SELECT f.path, f.size, f.mime_type
			FROM files f
			JOIN media_files mf ON mf.file_id = f.id
			WHERE mf.media_item_id = ?`

		rows, err := h.db.QueryContext(ctx, filesQuery, *req.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query files"})
			return
		}
		defer rows.Close()

		files := make([]gin.H, 0)
		for rows.Next() {
			var path string
			var size int64
			var mimeType sql.NullString
			if err := rows.Scan(&path, &size, &mimeType); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan file"})
				return
			}
			fileInfo := gin.H{"path": path, "size": size}
			if mimeType.Valid {
				fileInfo["mime_type"] = mimeType.String
			}
			files = append(files, fileInfo)
		}

		// Get media type name
		var typeName string
		typeQuery := `SELECT name FROM media_types WHERE id = ?`
		if err := h.db.QueryRowContext(ctx, typeQuery, item.MediaTypeID).Scan(&typeName); err != nil {
			typeName = "unknown"
		}

		c.JSON(http.StatusOK, gin.H{
			"analysis": gin.H{
				"type":  typeName,
				"files": files,
				"metadata": gin.H{
					"id":             item.ID,
					"title":          item.Title,
					"media_type_id":  item.MediaTypeID,
					"status":         item.Status,
					"first_detected": item.FirstDetected,
					"last_updated":   item.LastUpdated,
				},
			},
		})
		return
	}

	// Analyze by path
	fileQuery := `SELECT id, path, size, mime_type FROM files WHERE path = ?`
	var fileID int64
	var filePath string
	var fileSize int64
	var fileMimeType sql.NullString

	err := h.db.QueryRowContext(ctx, fileQuery, req.Path).Scan(&fileID, &filePath, &fileSize, &fileMimeType)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found at specified path"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query file"})
		return
	}

	fileInfo := gin.H{
		"path": filePath,
		"size": fileSize,
	}
	if fileMimeType.Valid {
		fileInfo["mime_type"] = fileMimeType.String
	}

	// Check if this file is linked to a media item
	var mediaItemID sql.NullInt64
	linkQuery := `SELECT media_item_id FROM media_files WHERE file_id = ? LIMIT 1`
	h.db.QueryRowContext(ctx, linkQuery, fileID).Scan(&mediaItemID)

	metadata := gin.H{}
	detectedType := "unknown"
	if fileMimeType.Valid {
		detectedType = fileMimeType.String
	}

	if mediaItemID.Valid {
		item, err := h.itemRepo.GetByID(ctx, mediaItemID.Int64)
		if err == nil && item != nil {
			metadata["id"] = item.ID
			metadata["title"] = item.Title
			metadata["media_type_id"] = item.MediaTypeID
			metadata["status"] = item.Status

			var typeName string
			typeQuery := `SELECT name FROM media_types WHERE id = ?`
			if err := h.db.QueryRowContext(ctx, typeQuery, item.MediaTypeID).Scan(&typeName); err == nil {
				detectedType = typeName
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"analysis": gin.H{
			"type":     detectedType,
			"files":    []gin.H{fileInfo},
			"metadata": metadata,
		},
	})
}

// changePasswordRequest represents the request body for password change.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword changes the authenticated user's password.
// POST /api/v1/auth/change-password
func (h *MediaQueryHandler) ChangePassword(c *gin.Context) {
	// Get user ID from JWT context (set by auth middleware as int)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userIDVal.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID in context"})
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current_password and new_password (min 8 chars) are required"})
		return
	}

	// Get user from DB
	user, err := h.userRepo.GetByID(uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Verify current password: bcrypt(password + salt)
	if ok, _ := services.CheckPassword(req.CurrentPassword, user.Salt, user.PasswordHash); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}

	// Generate new salt
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate salt"})
		return
	}
	newSalt := hex.EncodeToString(saltBytes)

	// Hash new password with new salt
	newHash, err := services.HashPasswordWithSalt(req.NewPassword, newSalt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash new password"})
		return
	}

	// Update password in DB
	if err := h.userRepo.UpdatePassword(uid, newHash, newSalt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}
	// Same as AuthService.ChangePassword: sessions issued under the old
	// password are revoked (the user logs in again with the new one).
	if err := h.userRepo.DeactivateAllUserSessions(uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password changed but revoking sessions failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Password changed successfully",
	})
}

// GetInitStatus returns the initialization status. This endpoint is public
// (no auth required) and always reports the system as initialized.
// GET /api/v1/auth/init-status
func (h *MediaQueryHandler) GetInitStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"initialized": true,
	})
}
