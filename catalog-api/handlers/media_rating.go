package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// CAT-RATING-1: per-user rating of a media item, 0..10 integer (decision recorded in migration v31). A rating is the
// caller's own user_media_ratings row; no query here is ever built from a user id other than the authenticated one, so a
// user can neither read, overwrite nor delete another user's rating. Authentication is checked before the id or body is
// parsed (an anonymous caller gets 401, never 400/404).

const (
	minMediaRating = 0
	maxMediaRating = 10
)

// ratingPrelude authenticates, parses the media id and (when body is nil) checks that the media item exists; PUT passes its
// body validator so the order is auth (401), id (400), body (400), existence (404), the same as PUT /media/:id/favorite.
// It writes the error response itself and returns ok=false when the request must stop.
func (h *AndroidTVMediaHandler) ratingPrelude(c *gin.Context, what string, validateBody func() bool) (callerID, mediaID int64, ok bool) {
	callerID, _ = callerUserID(c)
	if callerID == 0 {
		utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return 0, 0, false
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media ID", err)
		return 0, 0, false
	}
	if validateBody != nil && !validateBody() {
		utils.SendErrorResponse(c, http.StatusBadRequest, "rating must be an integer from 0 to 10", nil)
		return 0, 0, false
	}
	var one int
	if err := h.db.QueryRowContext(c.Request.Context(), `SELECT 1 FROM media_items WHERE id = ?`, id).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.SendErrorResponse(c, http.StatusNotFound, "Media not found", nil)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to "+what, err)
		}
		return 0, 0, false
	}
	return callerID, id, true
}

// parseRatingBody accepts exactly {"rating": N} where N is a JSON number with an integral value in 0..10 (7 and 7.0 are
// accepted, which keeps the Android RatingRequest(Double) wire shape working; 7.5, "7", null, true and out-of-range values
// are rejected).
func parseRatingBody(c *gin.Context) (int, bool) {
	var body map[string]json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		return 0, false
	}
	raw, present := body["rating"]
	if !present {
		return 0, false
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == '"' || raw[0] == 'n' || raw[0] == 't' || raw[0] == 'f' {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f != math.Trunc(f) || f < minMediaRating || f > maxMediaRating {
		return 0, false
	}
	return int(f), true
}

// PutMediaRating godoc
// @Summary Set the caller's rating of a media item
// @Description Stores the authenticated user's own 0..10 integer rating (upsert). Never touches another user's rating.
// @Tags media
// @Accept json
// @Produce json
// @Param id path int true "Media ID"
// @Param body body map[string]int true "Rating (0..10 integer)"
// @Success 200 {object} utils.SuccessResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 401 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id}/rating [put]
func (h *AndroidTVMediaHandler) PutMediaRating(c *gin.Context) {
	var rating int
	callerID, id, ok := h.ratingPrelude(c, "update rating", func() bool {
		var valid bool
		rating, valid = parseRatingBody(c)
		return valid
	})
	if !ok {
		return
	}
	// Portable upsert: PostgreSQL gets ON CONFLICT ... DO UPDATE; the bundled SQLite build predates 3.24 and gets INSERT OR
	// REPLACE (which also refreshes created_at there; updated_at is the column to trust).
	q := h.db.Dialect().UpsertSQL("user_media_ratings", []string{"user_id", "media_id", "rating", "updated_at"}, []string{"user_id", "media_id"})
	if _, err := h.db.ExecContext(c.Request.Context(), q, callerID, id, rating, time.Now().UTC()); err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update rating", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "media_id": id, "rating": rating})
}

// GetMediaRating godoc
// @Summary Get the caller's rating of a media item
// @Description Returns the authenticated user's own rating (0..10) or null when they have not rated the item.
// @Tags media
// @Produce json
// @Param id path int true "Media ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 401 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id}/rating [get]
func (h *AndroidTVMediaHandler) GetMediaRating(c *gin.Context) {
	callerID, id, ok := h.ratingPrelude(c, "read rating", nil)
	if !ok {
		return
	}
	var rating int
	err := h.db.QueryRowContext(c.Request.Context(),
		`SELECT rating FROM user_media_ratings WHERE user_id = ? AND media_id = ?`, callerID, id).Scan(&rating)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		c.JSON(http.StatusOK, gin.H{"media_id": id, "rating": nil})
	case err != nil:
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to read rating", err)
	default:
		c.JSON(http.StatusOK, gin.H{"media_id": id, "rating": rating})
	}
}

// DeleteMediaRating godoc
// @Summary Remove the caller's rating of a media item
// @Description Deletes the authenticated user's own rating; idempotent (200 even when there was none).
// @Tags media
// @Produce json
// @Param id path int true "Media ID"
// @Success 200 {object} utils.SuccessResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 401 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id}/rating [delete]
func (h *AndroidTVMediaHandler) DeleteMediaRating(c *gin.Context) {
	callerID, id, ok := h.ratingPrelude(c, "remove rating", nil)
	if !ok {
		return
	}
	if _, err := h.db.ExecContext(c.Request.Context(),
		`DELETE FROM user_media_ratings WHERE user_id = ? AND media_id = ?`, callerID, id); err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to remove rating", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "media_id": id})
}
