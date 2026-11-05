package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// SearchHandler handles search operations
type SearchHandler struct {
	fileRepo *repository.FileRepository
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(fileRepo *repository.FileRepository) *SearchHandler {
	return &SearchHandler{
		fileRepo: fileRepo,
	}
}

// SearchFiles godoc
// @Summary Search files
// @Description Perform advanced file search with filters and sorting
// @Tags search
// @Accept json
// @Produce json
// @Param q query string false "Search query (substring match on filename, path, and the extracted text of pdf, txt and md files of local storage roots)"
// @Param path query string false "Path filter (partial match)"
// @Param name query string false "Name filter (partial match)"
// @Param extension query string false "File extension filter (exact match)"
// @Param file_type query string false "File type filter (exact match)"
// @Param mime_type query string false "MIME type filter (exact match)"
// @Param smb_roots query string false "SMB roots filter (comma-separated list)"
// @Param min_size query int false "Minimum file size in bytes"
// @Param max_size query int false "Maximum file size in bytes"
// @Param modified_after query string false "Modified after date (RFC3339 format)"
// @Param modified_before query string false "Modified before date (RFC3339 format)"
// @Param include_deleted query bool false "Include deleted files" default(false)
// @Param only_duplicates query bool false "Only show duplicate files" default(false)
// @Param exclude_duplicates query bool false "Exclude duplicate files" default(false)
// @Param include_directories query bool false "Include directories" default(true)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(100)
// @Param sort_by query string false "Sort field (name, size, modified_at, created_at, path, extension)" default("name")
// @Param sort_order query string false "Sort order (asc, desc)" default("asc")
// @Success 200 {object} models.SearchResult
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/search [get]
func (h *SearchHandler) SearchFiles(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse search filters
	filter := models.SearchFilter{
		Query:              c.Query("q"),
		Path:               c.Query("path"),
		Name:               c.Query("name"),
		Extension:          c.Query("extension"),
		FileType:           c.Query("file_type"),
		MimeType:           c.Query("mime_type"),
		IncludeDeleted:     parseBool(c.Query("include_deleted"), false),
		OnlyDuplicates:     parseBool(c.Query("only_duplicates"), false),
		ExcludeDuplicates:  parseBool(c.Query("exclude_duplicates"), false),
		IncludeDirectories: parseBool(c.Query("include_directories"), true),
	}

	// Parse SMB roots filter
	if smbRootsStr := c.Query("smb_roots"); smbRootsStr != "" {
		filter.StorageRoots = strings.Split(smbRootsStr, ",")
		// Trim whitespace from each root name
		for i := range filter.StorageRoots {
			filter.StorageRoots[i] = strings.TrimSpace(filter.StorageRoots[i])
		}
	}

	// Parse size filters
	if minSizeStr := c.Query("min_size"); minSizeStr != "" {
		if minSize, err := strconv.ParseInt(minSizeStr, 10, 64); err == nil {
			filter.MinSize = &minSize
		}
	}
	if maxSizeStr := c.Query("max_size"); maxSizeStr != "" {
		if maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64); err == nil {
			filter.MaxSize = &maxSize
		}
	}

	// Parse date filters
	if modifiedAfterStr := c.Query("modified_after"); modifiedAfterStr != "" {
		if modifiedAfter, err := time.Parse(time.RFC3339, modifiedAfterStr); err == nil {
			filter.ModifiedAfter = &modifiedAfter
		} else {
			utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid modified_after date format. Use RFC3339 format", err)
			return
		}
	}
	if modifiedBeforeStr := c.Query("modified_before"); modifiedBeforeStr != "" {
		if modifiedBefore, err := time.Parse(time.RFC3339, modifiedBeforeStr); err == nil {
			filter.ModifiedBefore = &modifiedBefore
		} else {
			utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid modified_before date format. Use RFC3339 format", err)
			return
		}
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 100
	}

	pagination := models.PaginationOptions{
		Page:  page,
		Limit: limit,
	}

	// Parse sorting
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")

	validSortFields := map[string]bool{
		"name":        true,
		"size":        true,
		"modified_at": true,
		"created_at":  true,
		"path":        true,
		"extension":   true,
	}
	if !validSortFields[sortBy] {
		sortBy = "name"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	sort := models.SortOptions{
		Field: sortBy,
		Order: sortOrder,
	}

	// Perform search
	result, err := h.fileRepo.SearchFiles(ctx, filter, pagination, sort)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Search failed", err)
		return
	}
	if result != nil && result.Files == nil {
		result.Files = []models.FileWithMetadata{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// SearchDuplicates godoc
// @Summary Search duplicate files
// @Description Find duplicate files across all SMB roots or within specific roots
// @Tags search
// @Accept json
// @Produce json
// @Param smb_roots query string false "SMB roots filter (comma-separated list)"
// @Param min_size query int false "Minimum file size in bytes"
// @Param max_size query int false "Maximum file size in bytes"
// @Param file_type query string false "File type filter"
// @Param extension query string false "File extension filter"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(100)
// @Param sort_by query string false "Sort field (name, size, modified_at, path)" default("name")
// @Param sort_order query string false "Sort order (asc, desc)" default("asc")
// @Success 200 {object} models.SearchResult
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/search/duplicates [get]
func (h *SearchHandler) SearchDuplicates(c *gin.Context) {
	ctx := c.Request.Context()

	// Create search filter for duplicates only
	filter := models.SearchFilter{
		OnlyDuplicates:     true,
		IncludeDirectories: false, // Duplicates only apply to files
		FileType:           c.Query("file_type"),
		Extension:          c.Query("extension"),
	}

	// Parse SMB roots filter
	if smbRootsStr := c.Query("smb_roots"); smbRootsStr != "" {
		filter.StorageRoots = strings.Split(smbRootsStr, ",")
		for i := range filter.StorageRoots {
			filter.StorageRoots[i] = strings.TrimSpace(filter.StorageRoots[i])
		}
	}

	// Parse size filters
	if minSizeStr := c.Query("min_size"); minSizeStr != "" {
		if minSize, err := strconv.ParseInt(minSizeStr, 10, 64); err == nil {
			filter.MinSize = &minSize
		}
	}
	if maxSizeStr := c.Query("max_size"); maxSizeStr != "" {
		if maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64); err == nil {
			filter.MaxSize = &maxSize
		}
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 100
	}

	pagination := models.PaginationOptions{
		Page:  page,
		Limit: limit,
	}

	// Parse sorting
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")

	validSortFields := map[string]bool{
		"name":        true,
		"size":        true,
		"modified_at": true,
		"path":        true,
	}
	if !validSortFields[sortBy] {
		sortBy = "name"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	sort := models.SortOptions{
		Field: sortBy,
		Order: sortOrder,
	}

	// Perform search
	result, err := h.fileRepo.SearchFiles(ctx, filter, pagination, sort)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Duplicate search failed", err)
		return
	}
	if result != nil && result.Files == nil {
		result.Files = []models.FileWithMetadata{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// AdvancedSearch godoc
// @Summary Advanced search with POST body
// @Description Perform advanced file search using POST body for complex filters
// @Tags search
// @Accept json
// @Produce json
// @Param body body SearchRequest true "Search request"
// @Success 200 {object} models.SearchResult
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/search/advanced [post]
func (h *SearchHandler) AdvancedSearch(c *gin.Context) {
	ctx := c.Request.Context()

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate pagination
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 1000 {
		req.Limit = 100
	}

	// Validate sorting
	validSortFields := map[string]bool{
		"name":        true,
		"size":        true,
		"modified_at": true,
		"created_at":  true,
		"path":        true,
		"extension":   true,
	}
	if !validSortFields[req.SortBy] {
		req.SortBy = "name"
	}
	if req.SortOrder != "asc" && req.SortOrder != "desc" {
		req.SortOrder = "asc"
	}

	pagination := models.PaginationOptions{
		Page:  req.Page,
		Limit: req.Limit,
	}

	sort := models.SortOptions{
		Field: req.SortBy,
		Order: req.SortOrder,
	}

	// Perform search
	result, err := h.fileRepo.SearchFiles(ctx, req.Filter, pagination, sort)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Advanced search failed", err)
		return
	}
	if result != nil && result.Files == nil {
		result.Files = []models.FileWithMetadata{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// SearchRequest represents the request body for advanced search
type SearchRequest struct {
	Filter    models.SearchFilter `json:"filter"`
	Page      int                 `json:"page"`
	Limit     int                 `json:"limit"`
	SortBy    string              `json:"sort_by"`
	SortOrder string              `json:"sort_order"`
}

// Helper function to parse boolean query parameters
func parseBool(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return result
}
