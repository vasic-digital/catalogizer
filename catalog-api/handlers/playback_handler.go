package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"catalogizer/repository"

	"github.com/gin-gonic/gin"
)

// PlaybackHandler exposes the /api/v1/playback/sessions endpoints
// and the entity-level progress / history views that render the
// "duration / current / last session / total reproductions"
// indicator on every media card across apps.
type PlaybackHandler struct {
	repo *repository.PlaybackSessionRepository
}

// NewPlaybackHandler wires a handler over the given repository.
func NewPlaybackHandler(repo *repository.PlaybackSessionRepository) *PlaybackHandler {
	return &PlaybackHandler{repo: repo}
}

// startPlaybackRequest is the JSON body of POST /sessions/start.
type startPlaybackRequest struct {
	MediaItemID   int64  `json:"media_item_id" binding:"required"`
	FileID        *int64 `json:"file_id,omitempty"`
	PositionUnit  string `json:"position_unit" binding:"required,oneof=seconds pages events"`
	StartPosition int64  `json:"start_position"`
}

// progressPlaybackRequest is the JSON body of POST /sessions/progress.
type progressPlaybackRequest struct {
	SessionID   int64 `json:"session_id" binding:"required"`
	EndPosition int64 `json:"end_position"`
	TotalAmount int64 `json:"total_amount"`
}

// endPlaybackRequest is the JSON body of POST /sessions/end.
// DurationTotal is optional — players only know the total
// media duration once they've parsed it from the stream, so
// clients MAY pass it on end() even if they couldn't on start().
type endPlaybackRequest struct {
	SessionID     int64  `json:"session_id" binding:"required"`
	EndPosition   int64  `json:"end_position"`
	TotalAmount   int64  `json:"total_amount"`
	DurationTotal *int64 `json:"duration_total,omitempty"`
	Completed     bool   `json:"completed"`
}

// userIDFromContext extracts the authenticated user id set by
// JWTMiddleware.RequireAuth. Returns 0 and false if unset.
func userIDFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case string:
		if n, err := strconv.ParseInt(x, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// StartSession handles POST /api/v1/playback/sessions/start.
func (h *PlaybackHandler) StartSession(c *gin.Context) {
	var req startPlaybackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadBody(c, err)
		return
	}
	uid, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	id, err := h.repo.Start(c.Request.Context(), repository.PlaybackStart{
		UserID:        uid,
		MediaItemID:   req.MediaItemID,
		FileID:        req.FileID,
		PositionUnit:  req.PositionUnit,
		StartPosition: req.StartPosition,
		StartedAt:     time.Now().UTC(),
	})
	if err != nil {
		respondInternal(c, http.StatusInternalServerError, "internal error", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": id})
}

// ProgressSession handles POST /api/v1/playback/sessions/progress.
func (h *PlaybackHandler) ProgressSession(c *gin.Context) {
	var req progressPlaybackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadBody(c, err)
		return
	}
	if err := h.repo.Progress(c.Request.Context(), repository.PlaybackProgress{
		SessionID:   req.SessionID,
		EndPosition: req.EndPosition,
		TotalAmount: req.TotalAmount,
	}); err != nil {
		if errors.Is(err, repository.ErrPlaybackSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "playback session not found"})
			return
		}
		respondInternal(c, http.StatusInternalServerError, "internal error", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// EndSession handles POST /api/v1/playback/sessions/end.
func (h *PlaybackHandler) EndSession(c *gin.Context) {
	var req endPlaybackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadBody(c, err)
		return
	}
	if err := h.repo.End(c.Request.Context(), repository.PlaybackEnd{
		SessionID:     req.SessionID,
		EndPosition:   req.EndPosition,
		TotalAmount:   req.TotalAmount,
		DurationTotal: req.DurationTotal,
		EndedAt:       time.Now().UTC(),
		Completed:     req.Completed,
	}); err != nil {
		if errors.Is(err, repository.ErrPlaybackSessionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "playback session not found"})
			return
		}
		respondInternal(c, http.StatusInternalServerError, "internal error", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetProgressForEntity handles GET /api/v1/entities/:id/progress.
// Returns the per-user summary the card UI needs. A never-
// reproduced entity returns `{"progress": null}` with status 200
// so clients don't have to special-case a 404 on first load.
func (h *PlaybackHandler) GetProgressForEntity(c *gin.Context) {
	mediaItemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity id"})
		return
	}
	uid, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	prog, err := h.repo.GetProgress(c.Request.Context(), uid, mediaItemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"progress": nil})
			return
		}
		respondInternal(c, http.StatusInternalServerError, "internal error", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"progress": prog})
}

// ListHistoryForEntity handles GET /api/v1/entities/:id/history?limit=N.
// Returns the per-user session list newest-first for the history
// drawer.
func (h *PlaybackHandler) ListHistoryForEntity(c *gin.Context) {
	mediaItemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity id"})
		return
	}
	uid, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	sessions, err := h.repo.ListHistory(c.Request.Context(), uid, mediaItemID, limit)
	if err != nil {
		respondInternal(c, http.StatusInternalServerError, "internal error", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "count": len(sessions)})
}
