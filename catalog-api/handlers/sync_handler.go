package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/models"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
)

// SyncHandler handles sync-related HTTP endpoints.
type SyncHandler struct {
	syncService *services.SyncService
	authService *services.AuthService
}

// NewSyncHandler creates a new SyncHandler.
func NewSyncHandler(syncService *services.SyncService, authService *services.AuthService) *SyncHandler {
	return &SyncHandler{
		syncService: syncService,
		authService: authService,
	}
}

// CreateEndpoint handles POST /sync/endpoints.
func (h *SyncHandler) CreateEndpoint(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	var endpoint models.SyncEndpoint
	if err := c.ShouldBindJSON(&endpoint); err != nil {
		syncFail(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	created, err := h.syncService.CreateSyncEndpoint(currentUser.ID, &endpoint)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "connection test failed") {
			status = http.StatusBadGateway
		}
		syncFail(c, status, "Failed to create sync endpoint", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": created})
}

// GetUserEndpoints handles GET /sync/endpoints.
func (h *SyncHandler) GetUserEndpoints(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	endpoints, err := h.syncService.GetUserEndpoints(currentUser.ID)
	if err != nil {
		syncFail(c, http.StatusInternalServerError, "Failed to get sync endpoints", err)
		return
	}

	// A user with no endpoints must get [] (not null): clients iterate `data`.
	if endpoints == nil {
		endpoints = []models.SyncEndpoint{}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoints})
}

// GetEndpoint handles GET /sync/endpoints/:id.
func (h *SyncHandler) GetEndpoint(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	endpointID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid endpoint ID"})
		return
	}

	endpoint, err := h.syncService.GetEndpoint(endpointID, currentUser.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		syncFail(c, status, "Failed to get sync endpoint", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoint})
}

// UpdateEndpoint handles PUT /sync/endpoints/:id.
func (h *SyncHandler) UpdateEndpoint(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	endpointID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid endpoint ID"})
		return
	}

	var updates models.UpdateSyncEndpointRequest
	if err := c.ShouldBindJSON(&updates); err != nil {
		syncFail(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	updated, err := h.syncService.UpdateEndpoint(endpointID, currentUser.ID, &updates)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "connection test failed") {
			status = http.StatusBadGateway
		}
		syncFail(c, status, "Failed to update sync endpoint", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

// DeleteEndpoint handles DELETE /sync/endpoints/:id.
func (h *SyncHandler) DeleteEndpoint(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	endpointID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid endpoint ID"})
		return
	}

	err = h.syncService.DeleteEndpoint(endpointID, currentUser.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		syncFail(c, status, "Failed to delete sync endpoint", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": "Sync endpoint deleted"}})
}

// StartSync handles POST /sync/endpoints/:id/sync.
func (h *SyncHandler) StartSync(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	endpointID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid endpoint ID"})
		return
	}

	session, err := h.syncService.StartSync(endpointID, currentUser.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "not active") {
			status = http.StatusConflict
		}
		syncFail(c, status, "Failed to start sync", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

// GetUserSessions handles GET /sync/sessions.
func (h *SyncHandler) GetUserSessions(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	limit := 50
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	sessions, err := h.syncService.GetUserSessions(currentUser.ID, limit, offset)
	if err != nil {
		syncFail(c, http.StatusInternalServerError, "Failed to get sync sessions", err)
		return
	}

	// No sessions yet must be [] (not null): clients iterate `data`.
	if sessions == nil {
		sessions = []models.SyncSession{}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": sessions})
}

// GetSession handles GET /sync/sessions/:id.
func (h *SyncHandler) GetSession(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid session ID"})
		return
	}

	session, err := h.syncService.GetSession(sessionID, currentUser.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		syncFail(c, status, "Failed to get sync session", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

// ScheduleSync handles POST /sync/schedules.
func (h *SyncHandler) ScheduleSync(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	var req struct {
		EndpointID int    `json:"endpoint_id" binding:"required"`
		Frequency  string `json:"frequency" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		syncFail(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	schedule := &models.SyncSchedule{
		Frequency: req.Frequency,
	}

	created, err := h.syncService.ScheduleSync(req.EndpointID, currentUser.ID, schedule)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unauthorized") {
			status = http.StatusForbidden
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		syncFail(c, status, "Failed to schedule sync", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": created})
}

// GetSyncStatistics handles GET /sync/statistics.
func (h *SyncHandler) GetSyncStatistics(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	startDate := time.Now().AddDate(0, -1, 0) // default: last 30 days
	endDate := time.Now()

	if startStr := c.Query("start_date"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startDate = parsed
		} else if parsed, err := time.Parse("2006-01-02", startStr); err == nil {
			startDate = parsed
		}
	}

	if endStr := c.Query("end_date"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endDate = parsed
		} else if parsed, err := time.Parse("2006-01-02", endStr); err == nil {
			endDate = parsed
		}
	}

	userID := currentUser.ID
	stats, err := h.syncService.GetSyncStatistics(&userID, startDate, endDate)
	if err != nil {
		syncFail(c, http.StatusInternalServerError, "Failed to get sync statistics", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// CleanupOldSessions handles POST /sync/cleanup.
func (h *SyncHandler) CleanupOldSessions(c *gin.Context) {
	_, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	var req struct {
		OlderThanDays int `json:"older_than_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to 30 days if no body provided
		req.OlderThanDays = 30
	}

	if req.OlderThanDays <= 0 {
		req.OlderThanDays = 30
	}

	olderThan := time.Now().AddDate(0, 0, -req.OlderThanDays)
	err = h.syncService.CleanupOldSessions(olderThan)
	if err != nil {
		syncFail(c, http.StatusInternalServerError, "Failed to cleanup old sessions", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": fmt.Sprintf("Cleaned up sessions older than %d days", req.OlderThanDays)}})
}

// getCurrentUser extracts the current user from the Authorization header.
func (h *SyncHandler) getCurrentUser(c *gin.Context) (*models.User, error) {
	token := c.GetHeader("Authorization")
	if token == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	// Remove "Bearer " prefix if present
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	user, err := h.authService.GetCurrentUser(token)
	if err != nil {
		return nil, fmt.Errorf("auth error: %w", err)
	}
	return user, nil
}
