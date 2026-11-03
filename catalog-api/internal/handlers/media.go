package handlers

import (
	"catalogizer/internal/media/analyzer"
	"catalogizer/internal/media/database"
	"catalogizer/internal/media/models"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MediaHandler handles media metadata endpoints
type MediaHandler struct {
	mediaDB  *database.MediaDatabase
	analyzer *analyzer.MediaAnalyzer
	logger   *zap.Logger
}

// NewMediaHandler creates a new media handler
func NewMediaHandler(mediaDB *database.MediaDatabase, analyzer *analyzer.MediaAnalyzer, logger *zap.Logger) *MediaHandler {
	return &MediaHandler{
		mediaDB:  mediaDB,
		analyzer: analyzer,
		logger:   logger,
	}
}

// @Summary Get all media types
// @Description Get list of all supported media types
// @Tags media
// @Produce json
// @Success 200 {array} models.MediaType
// @Failure 500 {object} map[string]string
// @Router /api/v1/media/types [get]
func (h *MediaHandler) GetMediaTypes(c *gin.Context) {
	query := `
		SELECT id, name, description, detection_patterns, metadata_providers, created_at, updated_at
		FROM media_types
		ORDER BY name
	`

	rows, err := h.mediaDB.GetDB().Query(query)
	if err != nil {
		h.logger.Error("Failed to get media types", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve media types"})
		return
	}
	defer rows.Close()

	var mediaTypes []models.MediaType
	for rows.Next() {
		var mt models.MediaType
		var patternsJSON, providersJSON string

		err := rows.Scan(
			&mt.ID, &mt.Name, &mt.Description, &patternsJSON, &providersJSON,
			&mt.CreatedAt, &mt.UpdatedAt,
		)
		if err != nil {
			h.logger.Error("Failed to scan media type", zap.Error(err))
			continue
		}

		// Parse JSON fields
		json.Unmarshal([]byte(patternsJSON), &mt.DetectionPatterns)
		json.Unmarshal([]byte(providersJSON), &mt.MetadataProviders)

		mediaTypes = append(mediaTypes, mt)
	}

	c.JSON(http.StatusOK, gin.H{
		"media_types": mediaTypes,
		"count":       len(mediaTypes),
	})
}

// @Summary Search media items
// @Description Search for media items with various filters
// @Tags media
// @Param query query string false "Search query"
// @Param media_types query string false "Comma-separated media type names"
// @Param year query int false "Specific year"
// @Param year_from query int false "Year range from"
// @Param year_to query int false "Year range to"
// @Param genre query string false "Comma-separated genres"
// @Param min_rating query number false "Minimum rating"
// @Param has_externals query bool false "Has external metadata"
// @Param quality query string false "Comma-separated quality levels"
// @Param smb_roots query string false "Comma-separated SMB roots"
// @Param watched_status query string false "Watched status"
// @Param sort_by query string false "Sort field" default(title)
// @Param sort_order query string false "Sort order" default(asc)
// @Param limit query int false "Limit results" default(50)
// @Param offset query int false "Offset for pagination" default(0)
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/media/search [get]
func (h *MediaHandler) SearchMedia(c *gin.Context) {
	var req models.MediaSearchRequest

	// Parse query parameters
	req.Query = c.Query("query")
	req.SortBy = c.DefaultQuery("sort_by", "title")
	req.SortOrder = c.DefaultQuery("sort_order", "asc")
	req.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	req.Offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Parse media types
	if mediaTypesStr := c.Query("media_types"); mediaTypesStr != "" {
		req.MediaTypes = strings.Split(mediaTypesStr, ",")
	}

	// Parse year
	if yearStr := c.Query("year"); yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil {
			req.Year = &year
		}
	}

	// Parse year range
	if yearFromStr := c.Query("year_from"); yearFromStr != "" {
		if yearFrom, err := strconv.Atoi(yearFromStr); err == nil {
			if req.YearRange == nil {
				req.YearRange = &models.YearRange{}
			}
			req.YearRange.From = yearFrom
		}
	}
	if yearToStr := c.Query("year_to"); yearToStr != "" {
		if yearTo, err := strconv.Atoi(yearToStr); err == nil {
			if req.YearRange == nil {
				req.YearRange = &models.YearRange{}
			}
			req.YearRange.To = yearTo
		}
	}

	// Parse other filters
	if genreStr := c.Query("genre"); genreStr != "" {
		req.Genre = strings.Split(genreStr, ",")
	}
	if qualityStr := c.Query("quality"); qualityStr != "" {
		req.Quality = strings.Split(qualityStr, ",")
	}
	if smbRootsStr := c.Query("smb_roots"); smbRootsStr != "" {
		req.SmbRoots = strings.Split(smbRootsStr, ",")
	}

	if minRatingStr := c.Query("min_rating"); minRatingStr != "" {
		if minRating, err := strconv.ParseFloat(minRatingStr, 64); err == nil {
			req.MinRating = &minRating
		}
	}

	if hasExternalsStr := c.Query("has_externals"); hasExternalsStr != "" {
		hasExternals := hasExternalsStr == "true"
		req.HasExternals = &hasExternals
	}

	req.WatchedStatus = &[]string{c.Query("watched_status")}[0]

	// Build query
	baseQuery := `
		SELECT mi.id, mi.media_type_id, mi.title, mi.original_title, mi.year, mi.description,
		       mi.genre, mi.director, mi.cast_crew, mi.rating, mi.runtime, mi.language, mi.country,
		       mi.status, mi.first_detected, mi.last_updated,
		       mt.name as media_type_name, mt.description as media_type_description
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		WHERE 1=1
	`

	countQuery := `
		SELECT COUNT(*)
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		WHERE 1=1
	`

	var conditions []string
	var args []interface{}

	// Add search conditions
	if req.Query != "" {
		conditions = append(conditions, "(mi.title LIKE ? OR mi.original_title LIKE ?)")
		searchTerm := "%" + req.Query + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if len(req.MediaTypes) > 0 {
		placeholders := strings.Repeat("?,", len(req.MediaTypes))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, "mt.name IN ("+placeholders+")")
		for _, mt := range req.MediaTypes {
			args = append(args, mt)
		}
	}

	if req.Year != nil {
		conditions = append(conditions, "mi.year = ?")
		args = append(args, *req.Year)
	}

	if req.YearRange != nil {
		if req.YearRange.From > 0 {
			conditions = append(conditions, "mi.year >= ?")
			args = append(args, req.YearRange.From)
		}
		if req.YearRange.To > 0 {
			conditions = append(conditions, "mi.year <= ?")
			args = append(args, req.YearRange.To)
		}
	}

	if req.MinRating != nil {
		conditions = append(conditions, "mi.rating >= ?")
		args = append(args, *req.MinRating)
	}

	if req.HasExternals != nil && *req.HasExternals {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM external_metadata em WHERE em.media_item_id = mi.id)")
	}

	// Build final queries
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	// Get total count
	var total int64
	err := h.mediaDB.GetDB().QueryRow(countQuery+whereClause, args...).Scan(&total)
	if err != nil {
		h.logger.Error("Failed to count search results", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	// Add sorting and pagination
	finalQuery := baseQuery + whereClause
	switch req.SortBy {
	case "title":
		finalQuery += " ORDER BY mi.title"
	case "year":
		finalQuery += " ORDER BY mi.year"
	case "rating":
		finalQuery += " ORDER BY mi.rating"
	case "created":
		finalQuery += " ORDER BY mi.first_detected"
	default:
		finalQuery += " ORDER BY mi.title"
	}

	if req.SortOrder == "desc" {
		finalQuery += " DESC"
	} else {
		finalQuery += " ASC"
	}

	finalQuery += " LIMIT ? OFFSET ?"
	args = append(args, req.Limit, req.Offset)

	// Execute search
	rows, err := h.mediaDB.GetDB().Query(finalQuery, args...)
	if err != nil {
		h.logger.Error("Failed to execute search", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}
	defer rows.Close()

	var mediaItems []models.MediaItem
	for rows.Next() {
		var item models.MediaItem
		var mediaType models.MediaType
		var genreJSON, castCrewJSON string

		err := rows.Scan(
			&item.ID, &item.MediaTypeID, &item.Title, &item.OriginalTitle, &item.Year,
			&item.Description, &genreJSON, &item.Director, &castCrewJSON, &item.Rating,
			&item.Runtime, &item.Language, &item.Country, &item.Status,
			&item.FirstDetected, &item.LastUpdated,
			&mediaType.Name, &mediaType.Description,
		)
		if err != nil {
			h.logger.Error("Failed to scan media item", zap.Error(err))
			continue
		}

		// Parse JSON fields
		json.Unmarshal([]byte(genreJSON), &item.Genre)
		json.Unmarshal([]byte(castCrewJSON), &item.CastCrew)

		item.MediaType = &mediaType
		mediaItems = append(mediaItems, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"media_items": mediaItems,
		"total":       total,
		"count":       len(mediaItems),
		"limit":       req.Limit,
		"offset":      req.Offset,
		"has_more":    int64(req.Offset+req.Limit) < total,
	})
}

// @Summary Get media item details
// @Description Get detailed information about a specific media item
// @Tags media
// @Param id path int true "Media Item ID"
// @Produce json
// @Success 200 {object} models.MediaItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/media/{id} [get]
func (h *MediaHandler) GetMediaItem(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media item ID"})
		return
	}

	mediaItem, err := h.getMediaItemWithDetails(id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Media item not found"})
			return
		}
		h.logger.Error("Failed to get media item", zap.Int64("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve media item"})
		return
	}

	c.JSON(http.StatusOK, mediaItem)
}

// @Summary Analyze directory
// @Description Trigger analysis of a specific directory
// @Tags media
// @Accept json
// @Param request body object true "Analysis request"
// @Produce json
// @Success 202 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/media/analyze [post]
func (h *MediaHandler) AnalyzeDirectory(c *gin.Context) {
	var request struct {
		DirectoryPath string `json:"directory_path" binding:"required"`
		SmbRoot       string `json:"smb_root" binding:"required"`
		Priority      int    `json:"priority"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if request.Priority == 0 {
		request.Priority = 5 // Default priority
	}

	err := h.analyzer.AnalyzeDirectory(c.Request.Context(), request.DirectoryPath, request.SmbRoot, request.Priority)
	if err != nil {
		h.logger.Error("Failed to queue directory analysis",
			zap.String("directory", request.DirectoryPath),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue analysis"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":        "Directory analysis queued",
		"directory_path": request.DirectoryPath,
		"smb_root":       request.SmbRoot,
		"priority":       request.Priority,
	})
}

// @Summary Get media statistics
// @Description Get statistics about media items and analysis
// @Tags media
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /api/v1/media/stats [get]
func (h *MediaHandler) GetMediaStats(c *gin.Context) {
	stats := make(map[string]interface{})

	// Database stats
	dbStats, err := h.mediaDB.GetStats()
	if err != nil {
		h.logger.Error("Failed to get database stats", zap.Error(err))
	} else {
		stats["database"] = dbStats
	}

	// Media type distribution
	typeDistribution, err := h.getMediaTypeDistribution()
	if err != nil {
		h.logger.Error("Failed to get media type distribution", zap.Error(err))
	} else {
		stats["media_type_distribution"] = typeDistribution
	}

	// Quality distribution
	qualityDistribution, err := h.getQualityDistribution()
	if err != nil {
		h.logger.Error("Failed to get quality distribution", zap.Error(err))
	} else {
		stats["quality_distribution"] = qualityDistribution
	}

	// Recent activity
	recentActivity, err := h.getRecentActivity()
	if err != nil {
		h.logger.Error("Failed to get recent activity", zap.Error(err))
	} else {
		stats["recent_activity"] = recentActivity
	}

	c.JSON(http.StatusOK, stats)
}

// Helper methods

func (h *MediaHandler) getMediaItemWithDetails(id int64) (*models.MediaItem, error) {
	// Get basic media item
	query := `
		SELECT mi.id, mi.media_type_id, mi.title, mi.original_title, mi.year, mi.description,
		       mi.genre, mi.director, mi.cast_crew, mi.rating, mi.runtime, mi.language, mi.country,
		       mi.status, mi.first_detected, mi.last_updated,
		       mt.name, mt.description
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		WHERE mi.id = ?
	`

	var item models.MediaItem
	var mediaType models.MediaType
	var genreJSON, castCrewJSON string

	err := h.mediaDB.GetDB().QueryRow(query, id).Scan(
		&item.ID, &item.MediaTypeID, &item.Title, &item.OriginalTitle, &item.Year,
		&item.Description, &genreJSON, &item.Director, &castCrewJSON, &item.Rating,
		&item.Runtime, &item.Language, &item.Country, &item.Status,
		&item.FirstDetected, &item.LastUpdated,
		&mediaType.Name, &mediaType.Description,
	)
	if err != nil {
		return nil, err
	}

	// Parse JSON fields
	if err := json.Unmarshal([]byte(genreJSON), &item.Genre); err != nil {
		h.logger.Warn("Failed to parse genre JSON", zap.Int64("media_id", id), zap.Error(err))
	}
	if err := json.Unmarshal([]byte(castCrewJSON), &item.CastCrew); err != nil {
		h.logger.Warn("Failed to parse cast/crew JSON", zap.Int64("media_id", id), zap.Error(err))
	}
	item.MediaType = &mediaType

	// Get external metadata (non-fatal — item is valid without enrichment)
	if extMeta, err := h.getExternalMetadata(id); err != nil {
		h.logger.Warn("Failed to get external metadata", zap.Int64("media_id", id), zap.Error(err))
	} else {
		item.ExternalMetadata = extMeta
	}

	// Get files
	if files, err := h.getMediaFiles(id); err != nil {
		h.logger.Warn("Failed to get media files", zap.Int64("media_id", id), zap.Error(err))
	} else {
		item.Files = files
	}

	// Get user metadata
	if userMeta, err := h.getUserMetadata(id); err != nil {
		h.logger.Warn("Failed to get user metadata", zap.Int64("media_id", id), zap.Error(err))
	} else {
		item.UserMetadata = userMeta
	}

	return &item, nil
}

func (h *MediaHandler) getExternalMetadata(mediaItemID int64) ([]models.ExternalMetadata, error) {
	query := `
		SELECT id, media_item_id, provider, external_id, data, rating, review_url, cover_url, trailer_url, last_fetched
		FROM external_metadata
		WHERE media_item_id = ?
		ORDER BY provider
	`

	rows, err := h.mediaDB.GetDB().Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metadata []models.ExternalMetadata
	for rows.Next() {
		var em models.ExternalMetadata
		err := rows.Scan(
			&em.ID, &em.MediaItemID, &em.Provider, &em.ExternalID, &em.Data,
			&em.Rating, &em.ReviewURL, &em.CoverURL, &em.TrailerURL, &em.LastFetched,
		)
		if err != nil {
			continue
		}
		metadata = append(metadata, em)
	}

	return metadata, nil
}

func (h *MediaHandler) getMediaFiles(mediaItemID int64) ([]models.MediaFile, error) {
	query := `
		SELECT id, media_item_id, file_path, smb_root, filename, file_size, file_extension,
		       quality_info, language, subtitle_tracks, audio_tracks, duration, checksum,
		       virtual_smb_link, direct_smb_link, last_verified, created_at
		FROM media_files
		WHERE media_item_id = ?
		ORDER BY filename
	`

	rows, err := h.mediaDB.GetDB().Query(query, mediaItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.MediaFile
	for rows.Next() {
		var mf models.MediaFile
		var qualityInfoJSON, subtitleTracksJSON, audioTracksJSON string

		err := rows.Scan(
			&mf.ID, &mf.MediaItemID, &mf.FilePath, &mf.SmbRoot, &mf.Filename,
			&mf.FileSize, &mf.FileExtension, &qualityInfoJSON, &mf.Language,
			&subtitleTracksJSON, &audioTracksJSON, &mf.Duration, &mf.Checksum,
			&mf.VirtualSmbLink, &mf.DirectSmbLink, &mf.LastVerified, &mf.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Parse JSON fields
		json.Unmarshal([]byte(qualityInfoJSON), &mf.QualityInfo)
		json.Unmarshal([]byte(subtitleTracksJSON), &mf.SubtitleTracks)
		json.Unmarshal([]byte(audioTracksJSON), &mf.AudioTracks)

		files = append(files, mf)
	}

	return files, nil
}

func (h *MediaHandler) getUserMetadata(mediaItemID int64) (*models.UserMetadata, error) {
	query := `
		SELECT id, media_item_id, user_rating, watched_status, watched_date, personal_notes, tags, favorite, created_at, updated_at
		FROM user_metadata
		WHERE media_item_id = ?
	`

	var um models.UserMetadata
	var tagsJSON string

	err := h.mediaDB.GetDB().QueryRow(query, mediaItemID).Scan(
		&um.ID, &um.MediaItemID, &um.UserRating, &um.WatchedStatus, &um.WatchedDate,
		&um.PersonalNotes, &tagsJSON, &um.Favorite, &um.CreatedAt, &um.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	json.Unmarshal([]byte(tagsJSON), &um.Tags)
	return &um, nil
}

func (h *MediaHandler) getMediaTypeDistribution() (map[string]int, error) {
	query := `
		SELECT mt.name, COUNT(mi.id) as count
		FROM media_types mt
		LEFT JOIN media_items mi ON mt.id = mi.media_type_id
		GROUP BY mt.id, mt.name
		ORDER BY count DESC
	`

	rows, err := h.mediaDB.GetDB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := make(map[string]int)
	for rows.Next() {
		var mediaType string
		var count int
		if err := rows.Scan(&mediaType, &count); err != nil {
			continue
		}
		distribution[mediaType] = count
	}

	return distribution, nil
}

func (h *MediaHandler) getQualityDistribution() (map[string]int, error) {
	return models.QualityDistribution(h.mediaDB.GetDB(), false)
}

func (h *MediaHandler) getRecentActivity() (map[string]interface{}, error) {
	activity := make(map[string]interface{})
	cutoffTime := time.Now().Add(-24 * time.Hour)

	// Recent analyses
	var recentAnalyses int
	err := h.mediaDB.GetDB().QueryRow(
		"SELECT COUNT(*) FROM directory_analysis WHERE last_analyzed > ?",
		cutoffTime,
	).Scan(&recentAnalyses)
	if err == nil {
		activity["analyses_24h"] = recentAnalyses
	}

	// Recent media items
	var recentItems int
	err = h.mediaDB.GetDB().QueryRow(
		"SELECT COUNT(*) FROM media_items WHERE first_detected > ?",
		cutoffTime,
	).Scan(&recentItems)
	if err == nil {
		activity["new_items_24h"] = recentItems
	}

	// Recent metadata updates
	var recentMetadata int
	err = h.mediaDB.GetDB().QueryRow(
		"SELECT COUNT(*) FROM external_metadata WHERE last_fetched > ?",
		cutoffTime,
	).Scan(&recentMetadata)
	if err == nil {
		activity["metadata_updates_24h"] = recentMetadata
	}

	return activity, nil
}
