package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// MediaBrowseHandler handles media browsing endpoints backed by the files database.
type MediaBrowseHandler struct {
	fileRepo  *repository.FileRepository
	statsRepo *repository.StatsRepository
	db        *database.DB
}

// NewMediaBrowseHandler creates a new media browse handler.
func NewMediaBrowseHandler(fileRepo *repository.FileRepository, statsRepo *repository.StatsRepository, db *database.DB) *MediaBrowseHandler {
	return &MediaBrowseHandler{
		fileRepo:  fileRepo,
		statsRepo: statsRepo,
		db:        db,
	}
}

// mediaItemJSON is the JSON shape the web frontend expects for each media item.
type mediaItemJSON struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	MediaType       string `json:"media_type"`
	Quality         string `json:"quality,omitempty"`
	FileSize        int64  `json:"file_size,omitempty"`
	DirectoryPath   string `json:"directory_path"`
	StorageRootName string `json:"storage_root_name,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	// EntityID/EntityType identify the media entity (media_items row) this file
	// belongs to, via media_files. ID above is a files.id and must never be used
	// as an entity id; both are omitted when the file has no linked entity.
	EntityID   *int64 `json:"entity_id,omitempty"`
	EntityType string `json:"entity_type,omitempty"`
}

// SearchMedia handles GET /api/v1/media/search — replaces the hardcoded stub.
func (h *MediaBrowseHandler) SearchMedia(c *gin.Context) {
	ctx := c.Request.Context()

	query := c.Query("query")
	mediaType := c.Query("media_type")
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 || limit > 200 {
		limit = 24
	}
	if offset < 0 {
		offset = 0
	}

	// Convert offset-based pagination to page-based (FileRepository uses 1-indexed pages).
	page := (offset / limit) + 1

	filter := models.SearchFilter{
		Query:    query,
		FileType: mediaType,
	}

	pagination := models.PaginationOptions{
		Page:  page,
		Limit: limit,
	}

	sort := models.SortOptions{
		Field: sortBy,
		Order: sortOrder,
	}

	result, err := h.fileRepo.SearchFiles(ctx, filter, pagination, sort)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to search media", err)
		return
	}

	items := make([]mediaItemJSON, 0, len(result.Files))
	for _, f := range result.Files {
		items = append(items, fileToMediaItem(f))
	}
	h.attachEntityLinks(ctx, items)

	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"total":  result.TotalCount,
		"limit":  limit,
		"offset": offset,
	})
}

// attachEntityLinks fills EntityID/EntityType from media_files for the given file
// rows in one query. The primary link wins, then the lowest entity id. A lookup
// failure leaves the fields unset (the list itself is still valid); it never
// guesses an id.
func (h *MediaBrowseHandler) attachEntityLinks(ctx context.Context, items []mediaItemJSON) {
	if h.db == nil || len(items) == 0 {
		return
	}
	args := make([]interface{}, 0, len(items))
	ph := make([]string, 0, len(items))
	pos := make(map[int64]int, len(items))
	for i, it := range items {
		args = append(args, it.ID)
		ph = append(ph, "?")
		pos[it.ID] = i
	}
	rows, err := h.db.QueryContext(ctx,
		`SELECT mf.file_id, mi.id, mt.name
		 FROM media_files mf
		 JOIN media_items mi ON mi.id = mf.media_item_id
		 JOIN media_types mt ON mt.id = mi.media_type_id
		 WHERE mf.file_id IN (`+strings.Join(ph, ",")+`)
		 ORDER BY mf.file_id, mf.is_primary DESC, mi.id ASC`, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var fileID, entityID int64
		var typeName string
		if rows.Scan(&fileID, &entityID, &typeName) != nil {
			continue
		}
		i, ok := pos[fileID]
		if !ok || items[i].EntityID != nil {
			continue // first row per file is the best link
		}
		id := entityID
		items[i].EntityID = &id
		items[i].EntityType = typeName
	}
}

// GetMediaStats handles GET /api/v1/media/stats — replaces the hardcoded stub.
func (h *MediaBrowseHandler) GetMediaStats(c *gin.Context) {
	ctx := c.Request.Context()

	overallStats, err := h.statsRepo.GetOverallStats(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get media stats", err)
		return
	}

	// Build by_type breakdown from the files table.
	byType := map[string]int{}
	rows, err := h.db.QueryContext(ctx,
		`SELECT COALESCE(file_type, 'other') AS ft, COUNT(*) AS cnt
		 FROM files
		 WHERE is_directory = 0 AND deleted = 0
		 GROUP BY ft`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ft string
			var cnt int
			if scanErr := rows.Scan(&ft, &cnt); scanErr == nil {
				byType[ft] = cnt
			}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			c.Error(rowsErr)
		}
	}

	// Count files added in the last 7 days as recent_additions.
	var recentAdditions int
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	_ = h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM files WHERE is_directory = 0 AND deleted = 0 AND created_at >= ?`,
		sevenDaysAgo,
	).Scan(&recentAdditions)

	// Build by_quality breakdown by file extension (proxy for quality/format).
	byQuality := map[string]int{}
	qualityRows, qualityErr := h.db.QueryContext(ctx,
		`SELECT COALESCE(LOWER(extension), 'unknown') AS ext, COUNT(*) AS cnt
		 FROM files
		 WHERE is_directory = 0 AND deleted = 0 AND extension IS NOT NULL AND extension != ''
		 GROUP BY ext
		 ORDER BY cnt DESC
		 LIMIT 20`)
	if qualityErr == nil {
		defer qualityRows.Close()
		for qualityRows.Next() {
			var ext string
			var cnt int
			if scanErr := qualityRows.Scan(&ext, &cnt); scanErr == nil {
				byQuality[ext] = cnt
			}
		}
		if rowsErr := qualityRows.Err(); rowsErr != nil {
			c.Error(rowsErr)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_items":      overallStats.TotalFiles,
		"by_type":          byType,
		"by_quality":       byQuality,
		"total_size":       overallStats.TotalSize,
		"recent_additions": recentAdditions,
	})
}

// fileToMediaItem converts a FileWithMetadata to the JSON shape expected by the frontend.
func fileToMediaItem(f models.FileWithMetadata) mediaItemJSON {
	mediaType := "other"
	if f.FileType != nil {
		mediaType = *f.FileType
	}

	quality := ""
	if f.Extension != nil {
		quality = *f.Extension
	}

	return mediaItemJSON{
		ID:              f.ID,
		Title:           f.Name,
		MediaType:       mediaType,
		Quality:         quality,
		FileSize:        f.Size,
		DirectoryPath:   f.Path,
		StorageRootName: f.StorageRootName,
		CreatedAt:       f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       f.ModifiedAt.Format(time.RFC3339),
	}
}
