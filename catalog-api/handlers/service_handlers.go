package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/models"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AnalyticsHandler struct {
	service *services.AnalyticsService
	logger  *zap.Logger
}

func NewAnalyticsHandler(service *services.AnalyticsService, logger *zap.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{
		service: service,
		logger:  logger,
	}
}

// bindWriteUserID attributes a write to the caller. A body user_id that names another
// account is refused with 403 (forged rows would feed the admin reports); an absent (0)
// or matching one is replaced by the token's id. It reports whether the handler may go on.
func bindWriteUserID(c *gin.Context, bodyID *int) bool {
	id64, ok := callerUserID(c)
	uid := int(id64)
	if !ok || uid <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return false
	}
	if *bodyID != 0 && *bodyID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "user_id must be the authenticated user"})
		return false
	}
	*bodyID = uid
	return true
}

func (h *AnalyticsHandler) LogMediaAccess(c *gin.Context) {
	var access models.MediaAccessLog
	if err := c.ShouldBindJSON(&access); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if !bindWriteUserID(c, &access.UserID) {
		return
	}

	if access.AccessTime.IsZero() {
		access.AccessTime = time.Now()
	}

	if err := h.service.LogMediaAccess(&access); err != nil {
		h.logger.Error("Failed to log media access", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log access"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "logged"})
}

func (h *AnalyticsHandler) LogEvent(c *gin.Context) {
	var event models.AnalyticsEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if !bindWriteUserID(c, &event.UserID) {
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if err := h.service.LogEvent(&event); err != nil {
		h.logger.Error("Failed to log analytics event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "logged"})
}

func (h *AnalyticsHandler) GetUserAnalytics(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format"})
		return
	}

	analytics, err := h.service.GetUserAnalytics(userID, startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get user analytics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

func (h *AnalyticsHandler) GetSystemAnalytics(c *gin.Context) {
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format"})
		return
	}

	analytics, err := h.service.GetSystemAnalytics(startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get system analytics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

func (h *AnalyticsHandler) GetMediaAnalytics(c *gin.Context) {
	mediaID, err := strconv.Atoi(c.Param("media_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid media ID"})
		return
	}

	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format"})
		return
	}

	analytics, err := h.service.GetMediaAnalytics(mediaID, startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get media analytics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

func (h *AnalyticsHandler) CreateReport(c *gin.Context) {
	var req struct {
		ReportType string                 `json:"report_type" binding:"required"`
		Params     map[string]interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	report, err := h.service.CreateReport(req.ReportType, req.Params)
	if err != nil {
		if errors.Is(err, services.ErrUnsupportedReportType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("Failed to create report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

type ReportingHandler struct {
	service *services.ReportingService
	logger  *zap.Logger
}

func NewReportingHandler(service *services.ReportingService, logger *zap.Logger) *ReportingHandler {
	return &ReportingHandler{
		service: service,
		logger:  logger,
	}
}

func (h *ReportingHandler) GetUsageReport(c *gin.Context) {
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format"})
		return
	}

	// Article XI §11.5: pass user_id from the authenticated context
	// so the service has the param it requires. Caught by FQA-API-277:
	// the report service threw "user_id parameter required" and the
	// handler wrapped that as 500 (it's a 400 from the user's
	// perspective, but it shouldn't happen at all when called via
	// auth-required routes — the user IS authenticated).
	params := map[string]interface{}{
		"start_date": startDate,
		"end_date":   endDate,
	}
	if uid, exists := c.Get("user_id"); exists {
		params["user_id"] = uid
	}

	report, err := h.service.GenerateReport("user_analytics", "json", params)
	if err != nil {
		// Missing-required-param errors from the service layer are
		// 400, not 500.
		if strings.Contains(err.Error(), "parameter required") ||
			strings.Contains(err.Error(), "required parameter") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("Failed to generate usage report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (h *ReportingHandler) GetPerformanceReport(c *gin.Context) {
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format"})
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format"})
		return
	}

	params := map[string]interface{}{
		"start_date": startDate,
		"end_date":   endDate,
	}

	report, err := h.service.GenerateReport("system_overview", "json", params)
	if err != nil {
		// Article XI §11.5: missing-required-param → 400, not 500.
		// FQA-API-278: the service threw "start_date parameter
		// required" and the handler returned 500. Now: validate the
		// parse errors above (was previously silently ignored with
		// `_`), and downgrade service-level missing-param errors.
		if strings.Contains(err.Error(), "parameter required") ||
			strings.Contains(err.Error(), "required parameter") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("Failed to generate performance report", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

type FavoritesHandler struct {
	service *services.FavoritesService
	logger  *zap.Logger
}

func NewFavoritesHandler(service *services.FavoritesService, logger *zap.Logger) *FavoritesHandler {
	return &FavoritesHandler{
		service: service,
		logger:  logger,
	}
}

func (h *FavoritesHandler) ListFavorites(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	mediaType := c.Query("media_type")
	category := c.Query("category")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var entityType *string
	if mediaType != "" {
		entityType = &mediaType
	}
	var categoryPtr *string
	if category != "" {
		categoryPtr = &category
	}

	favorites, err := h.service.GetUserFavorites(uid, entityType, categoryPtr, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get favorites", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get favorites"})
		return
	}

	// No favorites must serialise as [] (not null): clients iterate it.
	if favorites == nil {
		favorites = []models.Favorite{}
	}

	c.JSON(http.StatusOK, gin.H{
		"favorites": favorites,
		"count":     len(favorites),
		"limit":     limit,
		"offset":    offset,
	})
}

func (h *FavoritesHandler) AddFavorite(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	var req struct {
		EntityID   int    `json:"entity_id" binding:"required"`
		EntityType string `json:"entity_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	favorite := &models.Favorite{
		UserID:     uid,
		EntityID:   req.EntityID,
		EntityType: req.EntityType,
	}

	// Article XI §11.5: validate entity_type against the small set
	// of types favorites supports BEFORE the service call so an
	// invalid type returns 400 (not 500). Caught by FQA-API-219.
	if !isValidEntityType(req.EntityType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_type"})
		return
	}

	_, err := h.service.AddFavorite(uid, favorite)
	if err != nil {
		// Distinguish "not found" / FK-violation from real internal
		// errors. FQA-API-211/217/218 caught this returning 500
		// when a not-yet-existing entity was favorited.
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "entity not found"})
			return
		}
		// "item already in favorites" → 409 Conflict, not 500.
		// Caught by FQA-API-211 (Add second time) where the API
		// previously returned 200 for the duplicate-add case.
		// Also catches DB-level UNIQUE constraint violations from
		// both SQLite ("UNIQUE constraint failed") and PostgreSQL
		// ("duplicate key value violates unique constraint").
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "already in favorites") ||
			strings.Contains(errStr, "already exists") ||
			strings.Contains(errStr, "duplicate") ||
			strings.Contains(errStr, "unique constraint") {
			c.JSON(http.StatusConflict, gin.H{"error": "already in favorites"})
			return
		}
		h.logger.Error("Failed to add favorite", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add favorite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "added"})
}

func (h *FavoritesHandler) RemoveFavorite(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	entityID, err := strconv.Atoi(c.Param("entity_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity ID"})
		return
	}

	entityType := c.Param("entity_type")
	if entityType == "" {
		entityType = c.Query("entity_type")
	}
	// Article XI §11.5: reject invalid entity_type before the
	// service call so the caller gets a deterministic 400 instead
	// of a 404 that hides the real problem.
	if !isValidEntityType(entityType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_type"})
		return
	}

	if err := h.service.RemoveFavorite(uid, entityType, entityID); err != nil {
		// Article XI §11.5: a remove of a non-favorited entity is
		// NOT an internal error — it's a 404. Caught by
		// FQA-API-218.
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "favorite not found"})
			return
		}
		h.logger.Error("Failed to remove favorite", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove favorite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

func (h *FavoritesHandler) CheckFavorite(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	entityID, err := strconv.Atoi(c.Param("entity_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity ID"})
		return
	}

	entityType := c.Param("entity_type")
	if entityType == "" {
		entityType = c.Query("entity_type")
	}
	// Article XI §11.5: reject invalid entity_type. Otherwise stale
	// rows from earlier broken inserts (when entity_type was not
	// validated on AddFavorite) would surface as `is_favorite:true`
	// for nonsense types — letting bad data corrupt client logic.
	// Caught by FQA-API-220.
	if !isValidEntityType(entityType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_type"})
		return
	}

	isFavorite, err := h.service.IsFavorite(uid, entityType, entityID)
	if err != nil {
		h.logger.Error("Failed to check favorite", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check favorite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"is_favorite": isFavorite})
}
