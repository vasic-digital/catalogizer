package handlers

import (
	"net/http"
	"strconv"

	"catalogizer/models"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PlaylistHandler handles HTTP requests for playlist operations.
type PlaylistHandler struct {
	service *services.PlaylistService
	logger  *zap.Logger
}

// NewPlaylistHandler creates a new playlist handler.
func NewPlaylistHandler(service *services.PlaylistService, logger *zap.Logger) *PlaylistHandler {
	return &PlaylistHandler{
		service: service,
		logger:  logger,
	}
}

// ListPlaylists handles GET /api/v1/playlists.
func (h *PlaylistHandler) ListPlaylists(c *gin.Context) {
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

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	playlists, err := h.service.GetUserPlaylists(uid, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list playlists", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list playlists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"playlists": playlists,
		"count":     len(playlists),
		"limit":     limit,
		"offset":    offset,
	})
}

// CreatePlaylist handles POST /api/v1/playlists.
func (h *PlaylistHandler) CreatePlaylist(c *gin.Context) {
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

	var req models.CreatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// CAT-001 (R5): smart playlists are not stored or evaluated; refuse instead of creating a plain empty one.
	if (req.IsSmart != nil && *req.IsSmart) || len(req.SmartRules) > 0 {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "smart playlists are not supported: is_smart and smart_rules are neither stored nor evaluated. Nothing was created.",
		})
		return
	}

	playlist, err := h.service.CreatePlaylist(uid, &req)
	if err != nil {
		h.logger.Error("Failed to create playlist", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create playlist"})
		return
	}

	c.JSON(http.StatusCreated, playlist)
}

// GetPlaylist handles GET /api/v1/playlists/:id.
func (h *PlaylistHandler) GetPlaylist(c *gin.Context) {
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

	playlistID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid playlist ID"})
		return
	}

	playlist, err := h.service.GetPlaylist(uid, playlistID)
	if err != nil {
		h.logger.Error("Failed to get playlist", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "playlist not found"})
		return
	}

	c.JSON(http.StatusOK, playlist)
}

// UpdatePlaylist handles PUT /api/v1/playlists/:id.
func (h *PlaylistHandler) UpdatePlaylist(c *gin.Context) {
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

	playlistID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid playlist ID"})
		return
	}

	var req models.UpdatePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	playlist, err := h.service.UpdatePlaylist(uid, playlistID, &req)
	if err != nil {
		// Article XI §11.5: distinguish "not found" from real internal
		// errors so PUT on a missing ID returns 404, not 500.
		// Caught by FQA-API-205 in the 2026-04-29 real-binary bank
		// verification.
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "playlist not found"})
			return
		}
		h.logger.Error("Failed to update playlist", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update playlist"})
		return
	}

	c.JSON(http.StatusOK, playlist)
}

// DeletePlaylist handles DELETE /api/v1/playlists/:id.
func (h *PlaylistHandler) DeletePlaylist(c *gin.Context) {
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

	playlistID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid playlist ID"})
		return
	}

	if err := h.service.DeletePlaylist(uid, playlistID); err != nil {
		h.logger.Error("Failed to delete playlist", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete playlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "playlist deleted"})
}

// AddItem handles POST /api/v1/playlists/:id/items.
func (h *PlaylistHandler) AddItem(c *gin.Context) {
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

	playlistID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid playlist ID"})
		return
	}

	var req models.AddPlaylistItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	item, err := h.service.AddItem(uid, playlistID, &req)
	if err != nil {
		h.logger.Error("Failed to add item to playlist", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add item to playlist"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// RemoveItem handles DELETE /api/v1/playlists/:id/items/:item_id.
func (h *PlaylistHandler) RemoveItem(c *gin.Context) {
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

	playlistID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid playlist ID"})
		return
	}

	itemID, err := strconv.Atoi(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item ID"})
		return
	}

	if err := h.service.RemoveItem(uid, playlistID, itemID); err != nil {
		h.logger.Error("Failed to remove item from playlist", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove item from playlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "item removed"})
}
