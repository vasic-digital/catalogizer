package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// AndroidTVMediaHandler handles media-specific operations for Android TV
type AndroidTVMediaHandler struct {
	db *database.DB
}

// NewAndroidTVMediaHandler creates a new Android TV media handler
func NewAndroidTVMediaHandler(db *database.DB) *AndroidTVMediaHandler {
	return &AndroidTVMediaHandler{
		db: db,
	}
}

// GetMediaByID godoc
// @Summary Get media by ID
// @Description Retrieve media item details by ID
// @Tags media
// @Accept json
// @Produce json
// @Param id path int true "Media ID"
// @Success 200 {object} models.MediaCatalogItem
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id} [get]
func (h *AndroidTVMediaHandler) GetMediaByID(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse media ID from path
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media ID", err)
		return
	}

	// DEFECT-D (2026-06-25): the prior query selected a FLAT/denormalized
	// media_items schema (`media_type`, `cover_image`, `quality`,
	// `file_size`, `directory_path`, `smb_path`, `created_at`,
	// `external_metadata`, `versions`, `watch_progress`, `last_watched`,
	// `is_downloaded`) that only ever existed in the in-memory test
	// helper (internal/tests/test_helper.go). The PRODUCTION schema
	// (database/migrations_sqlite.go media_items + migration v18) is
	// NORMALIZED: the type name lives in media_types(name) reached via
	// media_type_id, and none of the other flat columns exist. Every
	// real call returned HTTP 500 `no such column: media_type`. This
	// query is rewritten to the real schema, mirroring the working
	// entities path (handlers/stub_handler.go, handlers/media_entity_handler.go):
	// JOIN media_types mt for the type-name string, and select only
	// columns that actually exist on media_items after migrations.
	query := `
		SELECT
			mi.id, mt.name, mi.title, mi.year, mi.description, mi.rating,
			mi.duration, mi.first_detected, mi.updated_at,
			EXISTS(SELECT 1 FROM user_favorites uf WHERE uf.media_id = mi.id AND uf.user_id = ?)
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		WHERE mi.id = ?
	`
	// CAT-HAUTHZ-F2: is_favorite is the CALLER's own state (user_favorites row for the authenticated user), never the
	// legacy global media_items.is_favorite flag. No authenticated user (should not happen behind the auth middleware)
	// means user 0, which owns no rows, so the flag is false.
	callerID, _ := callerUserID(c)

	var mediaItem models.MediaCatalogItem
	// duration and updated_at are nullable in the real schema
	// (v18 added them with NULL-permitting defaults; runtime is NULL for
	// non-video types), so scan through sql.Null* and convert.
	var duration sql.NullInt64
	var updatedAt sql.NullString
	err = h.db.QueryRowContext(ctx, query, callerID, id).Scan(
		&mediaItem.ID, &mediaItem.MediaType, &mediaItem.Title,
		&mediaItem.Year, &mediaItem.Description, &mediaItem.Rating,
		&duration, &mediaItem.CreatedAt, &updatedAt, &mediaItem.IsFavorite,
	)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			utils.SendErrorResponse(c, http.StatusNotFound, "Media not found", err)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve media", err)
		}
		return
	}

	// Map nullable real-schema columns onto the response model.
	if duration.Valid {
		mediaItem.Duration = &duration.Int64
	}
	if updatedAt.Valid {
		mediaItem.UpdatedAt = updatedAt.String
	} else {
		// Real schema backfills updated_at from last_updated; when still
		// NULL, fall back to the creation timestamp for a stable value.
		mediaItem.UpdatedAt = mediaItem.CreatedAt
	}

	// DEFECT-D: the following response fields have NO column on the real
	// normalized media_items table and are populated from other tables
	// elsewhere in the system. GetMediaByID has no user context wired
	// (route is /api/v1/media/:id with no auth-derived user), so the
	// per-user fields cannot be resolved here. Each is defaulted to a
	// safe zero value rather than fabricating wrong data (§11.4.6).
	//   CoverImage       -> external_metadata.cover_url / cover_art table (per-item)        : default nil
	//   Quality          -> media_files.quality_info (per-file)                              : default nil
	//   FileSize         -> files.size via media_files (per-file)                            : default nil
	//   DirectoryPath    -> not present in normalized schema                                 : default ""
	//   SMBPath          -> not present in normalized schema                                 : default nil
	//   ExternalMetadata -> external_metadata table (provider rows, fetched separately)      : default []
	//   Versions         -> media_files rows (fetched separately)                            : default []
	//   WatchProgress    -> playback_positions.percent_complete (per-user, needs user_id)    : default 0
	//   LastWatched      -> playback_positions.last_played (per-user, needs user_id)         : default nil
	//   IsDownloaded     -> no equivalent concept in the normalized schema                   : default false
	mediaItem.ExternalMetadata = []models.ExternalMetadata{}
	mediaItem.Versions = []models.MediaVersion{}

	c.JSON(http.StatusOK, mediaItem)
}

// watchProgressRequest is the body of PUT /media/:id/progress. `progress` is a fraction 0.0-1.0 (all shipped clients send
// it). `timestamp` is accepted as a string (Android sends epoch millis as a string) or a number and is otherwise ignored:
// the server stamps updated_at itself. `position` is an absolute position in seconds; it is used only when the caller's
// stored row is in seconds. Unknown fields (the TS client also sends media_id, duration) are ignored.
type watchProgressRequest struct {
	Progress  *float64        `json:"progress"`
	Position  *int64          `json:"position"`
	Timestamp json.RawMessage `json:"timestamp"`
}

// Watch progress without a known duration is stored as a fraction in permille (0..1000) against a fixed duration_total
// of 1000, so media_progress readers (last_position / duration_total) still compute the right fraction.
const (
	progressUnitPermille = "permille"
	progressPermilleBase = 1000
)

// UpdateWatchProgress godoc
// @Summary Update watch progress
// @Description Store the authenticated user's watch progress for a media item (per user, in media_progress)
// @Tags media
// @Accept json
// @Produce json
// @Param id path int true "Media ID"
// @Param body body watchProgressRequest true "progress 0.0-1.0 (required), optional position in seconds"
// @Success 200 {object} utils.SuccessResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 401 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id}/progress [put]
func (h *AndroidTVMediaHandler) UpdateWatchProgress(c *gin.Context) {
	ctx := c.Request.Context()

	// Authentication is checked first, before any input is parsed (an anonymous caller gets 401, never 400).
	callerID, _ := callerUserID(c)
	if callerID == 0 {
		utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media ID", err)
		return
	}

	var req watchProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if req.Progress == nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Progress field is required", nil)
		return
	}
	progress := *req.Progress
	if !(progress >= 0.0 && progress <= 1.0) {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Progress must be between 0.0 and 1.0", nil)
		return
	}
	if req.Position != nil && *req.Position < 0 {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Position must not be negative", nil)
		return
	}

	var one int
	if err := h.db.QueryRowContext(ctx, `SELECT 1 FROM media_items WHERE id = ?`, id).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.SendErrorResponse(c, http.StatusNotFound, "Media not found", nil)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update watch progress", err)
		}
		return
	}

	// Portable SELECT / UPDATE / INSERT upsert (same pattern as PlaybackSessionRepository: the SQLCipher build predates
	// ON CONFLICT DO UPDATE). A concurrent first insert that loses the primary-key race is retried once as an update.
	for attempt := 0; attempt < 2; attempt++ {
		var unit string
		var duration sql.NullInt64
		selErr := h.db.QueryRowContext(ctx,
			`SELECT position_unit, duration_total FROM media_progress WHERE user_id = ? AND media_item_id = ?`,
			callerID, id).Scan(&unit, &duration)
		if selErr != nil && !errors.Is(selErr, sql.ErrNoRows) {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update watch progress", selErr)
			return
		}
		now := time.Now().UTC()
		if selErr == nil {
			newUnit, pos, dur := progressToPosition(progress, req.Position, unit, duration)
			if _, err := h.db.ExecContext(ctx,
				`UPDATE media_progress SET position_unit = ?, duration_total = ?, last_position = ?, updated_at = ?
				 WHERE user_id = ? AND media_item_id = ?`,
				newUnit, dur, pos, now, callerID, id); err != nil {
				utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update watch progress", err)
				return
			}
			break
		}
		newUnit, pos, dur := progressToPosition(progress, req.Position, progressUnitPermille, sql.NullInt64{})
		if _, err := h.db.ExecContext(ctx,
			`INSERT INTO media_progress (user_id, media_item_id, position_unit, duration_total, last_position, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			callerID, id, newUnit, dur, pos, now); err != nil {
			if attempt == 0 {
				continue
			}
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update watch progress", err)
			return
		}
		break
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Watch progress updated",
		"persisted": true,
	})
}

// progressToPosition converts the client's fraction into the unit of the stored row and returns (unit, last_position,
// duration_total). A row with a known duration keeps its unit; an explicit position wins when the unit is seconds
// (clamped to the duration). A row with an unknown duration, or a new row, is stored as permille of 1000.
func progressToPosition(progress float64, position *int64, unit string, duration sql.NullInt64) (string, int64, int64) {
	if duration.Valid && duration.Int64 > 0 {
		if unit == "seconds" && position != nil {
			p := *position
			if p > duration.Int64 {
				p = duration.Int64
			}
			return unit, p, duration.Int64
		}
		return unit, int64(math.Round(progress * float64(duration.Int64))), duration.Int64
	}
	return progressUnitPermille, int64(math.Round(progress * progressPermilleBase)), progressPermilleBase
}

// UpdateFavoriteStatus godoc
// @Summary Update favorite status
// @Description Update the favorite status for a media item
// @Tags media
// @Accept json
// @Produce json
// @Param id path int true "Media ID"
// @Param body body map[string]bool true "Favorite status"
// @Success 200 {object} utils.SuccessResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 401 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id}/favorite [put]
func (h *AndroidTVMediaHandler) UpdateFavoriteStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// Authentication is checked first, before any input is parsed (an anonymous caller gets 401, never 400).
	callerID, _ := callerUserID(c)
	if callerID == 0 {
		utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Parse media ID from path
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media ID", err)
		return
	}

	// Parse request body
	var requestBody map[string]bool
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	isFavorite, exists := requestBody["favorite"]
	if !exists {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Favorite field is required", nil)
		return
	}

	// CAT-HAUTHZ-F2: the favorite flag is per user. It used to UPDATE media_items.is_favorite (one flag per item shared by
	// every account); now it adds or removes the caller's own user_favorites row, so it can neither show nor change what
	// another user favorited. The item must exist (404 otherwise, as before); both toggles are idempotent.
	var one int
	if err := h.db.QueryRowContext(ctx, `SELECT 1 FROM media_items WHERE id = ?`, id).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.SendErrorResponse(c, http.StatusNotFound, "Media not found", nil)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update favorite status", err)
		}
		return
	}
	if isFavorite {
		_, err = h.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO user_favorites (user_id, media_id) VALUES (?, ?)`,
			callerID, id)
	} else {
		_, err = h.db.ExecContext(ctx, `DELETE FROM user_favorites WHERE user_id = ? AND media_id = ?`, callerID, id)
	}
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update favorite status", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Favorite status updated successfully",
	})
}
