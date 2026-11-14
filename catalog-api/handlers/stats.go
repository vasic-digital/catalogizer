package handlers

import (
	"context"
	"net/http"
	"strconv"

	"catalogizer/internal/models"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// StatsHandler handles statistics and analytics operations
type StatsHandler struct {
	fileRepo  *repository.FileRepository
	statsRepo *repository.StatsRepository
	isAdmin   func(ctx context.Context, userID int) (bool, error)
}

// WithAdminChecker installs the live admin check (production wires UserRepository.IsAdmin).
// Without it the role_id claim in the request context is used (role 1 is the seeded admin role).
func (h *StatsHandler) WithAdminChecker(f func(ctx context.Context, userID int) (bool, error)) *StatsHandler {
	h.isAdmin = f
	return h
}

// scanErrorClassText is what non-administrators see instead of a raw scan error message
// (which can carry host, share and account names).
const scanErrorClassText = "scan error (details restricted to administrators)"

// callerIsAdmin reports whether the caller is an administrator; any failure fails closed (false).
func (h *StatsHandler) callerIsAdmin(c *gin.Context) bool {
	if h.isAdmin != nil {
		uid, ok := statsActorID(c)
		if !ok {
			return false
		}
		admin, err := h.isAdmin(c.Request.Context(), uid)
		return err == nil && admin
	}
	if r, ok := c.Get("role_id"); ok {
		if id, isInt := r.(int); isInt {
			return id == 1
		}
	}
	return false
}

func statsActorID(c *gin.Context) (int, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	switch id := v.(type) {
	case int:
		return id, id > 0
	case int64:
		return int(id), id > 0
	}
	return 0, false
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(fileRepo *repository.FileRepository, statsRepo *repository.StatsRepository) *StatsHandler {
	return &StatsHandler{
		fileRepo:  fileRepo,
		statsRepo: statsRepo,
	}
}

// GetOverallStats godoc
// @Summary Get overall catalog statistics
// @Description Get comprehensive statistics about the entire catalog
// @Tags stats
// @Accept json
// @Produce json
// @Success 200 {object} OverallStats
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/overall [get]
func (h *StatsHandler) GetOverallStats(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.statsRepo.GetOverallStats(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get overall stats", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetSmbRootStats godoc
// @Summary Get SMB root statistics
// @Description Get statistics for a specific SMB root
// @Tags stats
// @Accept json
// @Produce json
// @Param smb_root path string true "SMB root name"
// @Success 200 {object} SmbRootStats
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/smb/{smb_root} [get]
func (h *StatsHandler) GetSmbRootStats(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Param("smb_root")
	if smbRootName == "" {
		utils.SendErrorResponse(c, http.StatusBadRequest, "SMB root name is required", nil)
		return
	}

	stats, err := h.statsRepo.GetStorageRootStats(ctx, smbRootName)
	if err != nil {
		// Article XI §11.5: distinguish "not found" — repository
		// surfaces both literal "smb root not found" and the
		// generic "storage root not found" depending on its
		// state. Either form is a 404, not 500. Caught by
		// FQA-API-067 in the 2026-04-29 real-binary bank
		// verification.
		if isNotFoundError(err) {
			utils.SendErrorResponse(c, http.StatusNotFound, "SMB root not found", err)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get SMB root stats", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetFileTypeStats godoc
// @Summary Get file type statistics
// @Description Get statistics about different file types across all or specific SMB roots
// @Tags stats
// @Accept json
// @Produce json
// @Param smb_root query string false "SMB root name filter"
// @Param limit query int false "Maximum number of results" default(50)
// @Success 200 {array} FileTypeStats
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/filetypes [get]
func (h *StatsHandler) GetFileTypeStats(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Query("smb_root")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	stats, err := h.statsRepo.GetFileTypeStats(ctx, smbRootName, limit)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get file type stats", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetSizeDistribution godoc
// @Summary Get file size distribution
// @Description Get statistics about file size distribution
// @Tags stats
// @Accept json
// @Produce json
// @Param smb_root query string false "SMB root name filter"
// @Success 200 {object} SizeDistribution
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/sizes [get]
func (h *StatsHandler) GetSizeDistribution(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Query("smb_root")

	distribution, err := h.statsRepo.GetSizeDistribution(ctx, smbRootName)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get size distribution", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    distribution,
	})
}

// GetDuplicateStats godoc
// @Summary Get duplicate file statistics
// @Description Get statistics about duplicate files
// @Tags stats
// @Accept json
// @Produce json
// @Param smb_root query string false "SMB root name filter"
// @Success 200 {object} DuplicateStats
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/duplicates [get]
func (h *StatsHandler) GetDuplicateStats(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Query("smb_root")

	stats, err := h.statsRepo.GetDuplicateStats(ctx, smbRootName)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get duplicate stats", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetTopDuplicateGroups godoc
// @Summary Get top duplicate groups
// @Description Get the largest duplicate groups by file count or total size
// @Tags stats
// @Accept json
// @Produce json
// @Param sort_by query string false "Sort by: count or size" default("count")
// @Param limit query int false "Maximum number of results" default(20)
// @Param smb_root query string false "SMB root name filter"
// @Success 200 {array} DuplicateGroupStats
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/duplicates/groups [get]
func (h *StatsHandler) GetTopDuplicateGroups(c *gin.Context) {
	ctx := c.Request.Context()

	sortBy := c.DefaultQuery("sort_by", "count")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	smbRootName := c.Query("smb_root")

	if sortBy != "count" && sortBy != "size" {
		utils.SendErrorResponse(c, http.StatusBadRequest, "sort_by must be 'count' or 'size'", nil)
		return
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	groups, err := h.statsRepo.GetTopDuplicateGroups(ctx, sortBy, limit, smbRootName)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get top duplicate groups", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
	})
}

// GetAccessPatterns godoc
// @Summary Get file access patterns
// @Description Get statistics about file access patterns over time
// @Tags stats
// @Accept json
// @Produce json
// @Param smb_root query string false "SMB root name filter"
// @Param days query int false "Number of days to analyze" default(30)
// @Success 200 {object} AccessPatterns
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/access [get]
func (h *StatsHandler) GetAccessPatterns(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Query("smb_root")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	if days <= 0 || days > 365 {
		days = 30
	}

	patterns, err := h.statsRepo.GetAccessPatterns(ctx, smbRootName, days)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get access patterns", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    patterns,
	})
}

// GetGrowthTrends godoc
// @Summary Get storage growth trends
// @Description Get statistics about storage growth over time
// @Tags stats
// @Accept json
// @Produce json
// @Param smb_root query string false "SMB root name filter"
// @Param months query int false "Number of months to analyze" default(12)
// @Success 200 {object} GrowthTrends
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/growth [get]
func (h *StatsHandler) GetGrowthTrends(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Query("smb_root")
	months, _ := strconv.Atoi(c.DefaultQuery("months", "12"))

	if months <= 0 || months > 60 {
		months = 12
	}

	trends, err := h.statsRepo.GetGrowthTrends(ctx, smbRootName, months)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get growth trends", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    trends,
	})
}

// GetScanHistory godoc
// @Summary Get scan history
// @Description Get history of scan operations
// @Tags stats
// @Accept json
// @Produce json
// @Param smb_root query string false "SMB root name filter"
// @Param limit query int false "Maximum number of results" default(50)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} ScanHistoryResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/stats/scans [get]
func (h *StatsHandler) GetScanHistory(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Query("smb_root")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	history, totalCount, err := h.statsRepo.GetScanHistory(ctx, smbRootName, limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get scan history", err)
		return
	}

	if !h.callerIsAdmin(c) {
		for i := range history {
			if history[i].ErrorMessage != nil && *history[i].ErrorMessage != "" {
				msg := scanErrorClassText
				history[i].ErrorMessage = &msg
			}
		}
	}

	response := ScanHistoryResponse{
		Scans:      history,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// Response types for statistics endpoints
type ScanHistoryResponse struct {
	Scans      []models.ScanHistoryItem `json:"scans"`
	TotalCount int64                    `json:"total_count"`
	Limit      int                      `json:"limit"`
	Offset     int                      `json:"offset"`
}

// These types would typically be defined in a separate stats models package
type OverallStats struct {
	TotalFiles         int64 `json:"total_files"`
	TotalDirectories   int64 `json:"total_directories"`
	TotalSize          int64 `json:"total_size"`
	TotalDuplicates    int64 `json:"total_duplicates"`
	DuplicateGroups    int64 `json:"duplicate_groups"`
	StorageRootsCount  int64 `json:"storage_roots_count"`
	ActiveStorageRoots int64 `json:"active_storage_roots"`
	LastScanTime       int64 `json:"last_scan_time"`
}

type StorageRootStats struct {
	Name             string `json:"name"`
	TotalFiles       int64  `json:"total_files"`
	TotalDirectories int64  `json:"total_directories"`
	TotalSize        int64  `json:"total_size"`
	DuplicateFiles   int64  `json:"duplicate_files"`
	DuplicateGroups  int64  `json:"duplicate_groups"`
	LastScanTime     int64  `json:"last_scan_time"`
	IsOnline         bool   `json:"is_online"`
}

type FileTypeStats struct {
	FileType    string `json:"file_type"`
	Extension   string `json:"extension"`
	Count       int64  `json:"count"`
	TotalSize   int64  `json:"total_size"`
	AverageSize int64  `json:"average_size"`
}

type SizeDistribution struct {
	Tiny    int64 `json:"tiny"`    // < 1KB
	Small   int64 `json:"small"`   // 1KB - 1MB
	Medium  int64 `json:"medium"`  // 1MB - 10MB
	Large   int64 `json:"large"`   // 10MB - 100MB
	Huge    int64 `json:"huge"`    // 100MB - 1GB
	Massive int64 `json:"massive"` // > 1GB
}

type DuplicateStats struct {
	TotalDuplicates       int64   `json:"total_duplicates"`
	DuplicateGroups       int64   `json:"duplicate_groups"`
	WastedSpace           int64   `json:"wasted_space"`
	LargestDuplicateGroup int     `json:"largest_duplicate_group"`
	AverageGroupSize      float64 `json:"average_group_size"`
}

type DuplicateGroupStats struct {
	GroupID     int64  `json:"group_id"`
	FileCount   int    `json:"file_count"`
	TotalSize   int64  `json:"total_size"`
	WastedSpace int64  `json:"wasted_space"`
	SamplePath  string `json:"sample_path"`
}

type AccessPatterns struct {
	RecentlyAccessed   int64    `json:"recently_accessed"`
	NeverAccessed      int64    `json:"never_accessed"`
	AccessFrequency    []int64  `json:"access_frequency"` // Daily access counts
	PopularExtensions  []string `json:"popular_extensions"`
	PopularDirectories []string `json:"popular_directories"`
}

type GrowthTrends struct {
	MonthlyGrowth   []MonthlyGrowth `json:"monthly_growth"`
	TotalGrowthRate float64         `json:"total_growth_rate"`
	FileGrowthRate  float64         `json:"file_growth_rate"`
	SizeGrowthRate  float64         `json:"size_growth_rate"`
}

type MonthlyGrowth struct {
	Month      string `json:"month"`
	FilesAdded int64  `json:"files_added"`
	SizeAdded  int64  `json:"size_added"`
	TotalFiles int64  `json:"total_files"`
	TotalSize  int64  `json:"total_size"`
}
