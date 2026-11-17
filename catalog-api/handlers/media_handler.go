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

// watchProgressRequest is the body of PUT /media/:id/progress. It accepts every shape the shipped clients send:
//   - {progress}                                    (Android TV; fraction 0.0-1.0)
//   - {progress, timestamp(string), position}       (Android WatchProgressRequest; not wired to this route)
//   - {media_id, position, duration, timestamp}     (desktop / api-client PlaybackProgress; floats, no `progress`)
//
// `progress` is optional when `position` and `duration` are both present (the fraction is then position/duration).
// position and duration are seconds and may be fractional (rounded to whole seconds). `timestamp` (string or number) and
// `media_id` are ignored: the server stamps updated_at itself and takes the id from the path.
type watchProgressRequest struct {
	Progress  *float64        `json:"progress"`
	Position  *float64        `json:"position"`
	Duration  *float64        `json:"duration"`
	Timestamp json.RawMessage `json:"timestamp"`
}

// watchProgressMax bounds position/duration so rounding to int64 can never overflow.
const watchProgressMax = 1e12

// UpdateWatchProgress godoc
// @Summary Update watch progress
// @Description Store the authenticated user's watch progress for a media item (per user, in media_progress). Rows are only written in seconds with a known duration; a request that cannot be stored without guessing a unit or duration answers 200 with persisted=false and a reason.
// @Tags media
// @Accept json
// @Produce json
// @Param id path int true "Media ID"
// @Param body body watchProgressRequest true "progress 0.0-1.0, or position+duration in seconds (floats allowed)"
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
	if req.Progress != nil && !(*req.Progress >= 0.0 && *req.Progress <= 1.0) {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Progress must be between 0.0 and 1.0", nil)
		return
	}
	if req.Position != nil && !(*req.Position >= 0 && *req.Position <= watchProgressMax) {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Position must be between 0 and 1e12", nil)
		return
	}
	if req.Duration != nil && !(*req.Duration >= 0 && *req.Duration <= watchProgressMax) {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Duration must be between 0 and 1e12", nil)
		return
	}
	if req.Progress == nil && (req.Position == nil || req.Duration == nil) {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Progress field is required (or position and duration)", nil)
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
	// ON CONFLICT DO UPDATE). A first insert that loses a race (or otherwise fails) is retried once from the SELECT, which
	// then finds the competing row and updates it.
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
		exists := selErr == nil
		pos, dur, reason := watchProgressPlan(req, exists, unit, duration)
		if reason != "" {
			c.JSON(http.StatusOK, gin.H{
				"success":   true,
				"message":   "Watch progress not stored",
				"persisted": false,
				"reason":    reason,
			})
			return
		}
		now := time.Now().UTC()
		if exists {
			// never changes the unit of an existing row; duration is only filled in when it was unknown
			if _, err := h.db.ExecContext(ctx,
				`UPDATE media_progress SET duration_total = ?, last_position = ?, updated_at = ?
				 WHERE user_id = ? AND media_item_id = ?`,
				dur, pos, now, callerID, id); err != nil {
				utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update watch progress", err)
				return
			}
			break
		}
		if _, err := h.db.ExecContext(ctx,
			`INSERT INTO media_progress (user_id, media_item_id, position_unit, duration_total, last_position, updated_at)
			 VALUES (?, ?, 'seconds', ?, ?, ?)`,
			callerID, id, dur, pos, now); err != nil {
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

// watchProgressPlan decides what to store and returns (last_position, duration_total, reason). A non-empty reason means
// the request is not stored. Rules: the unit of an existing row is never changed (the caller does not write it); a new
// row is only created in seconds and only when the duration is known from the body; an existing row with a known duration
// keeps it (converted fraction, or `position` seconds when the unit is seconds, clamped); an existing seconds row with an
// unknown duration only learns the duration from the body, otherwise the request is ignored.
func watchProgressPlan(req watchProgressRequest, exists bool, unit string, duration sql.NullInt64) (int64, int64, string) {
	bodyDur := int64(0)
	if req.Duration != nil {
		bodyDur = int64(math.Round(*req.Duration))
	}
	fraction := func(dur int64) (float64, bool) {
		if req.Progress != nil {
			return *req.Progress, true
		}
		if req.Position != nil && bodyDur > 0 {
			f := *req.Position / float64(bodyDur)
			if f > 1 {
				f = 1
			}
			return f, true
		}
		return 0, false
	}
	secondsPos := func(dur int64) (int64, bool) {
		if req.Position != nil {
			p := int64(math.Round(*req.Position))
			if p > dur {
				p = dur
			}
			return p, true
		}
		if f, ok := fraction(dur); ok {
			return int64(math.Round(f * float64(dur))), true
		}
		return 0, false
	}

	switch {
	case exists && duration.Valid && duration.Int64 > 0:
		dur := duration.Int64
		if unit == "seconds" {
			if p, ok := secondsPos(dur); ok {
				return p, dur, ""
			}
			return 0, 0, "no_position"
		}
		// pages/events/other units: only the fraction is meaningful
		if f, ok := fraction(dur); ok {
			return int64(math.Round(f * float64(dur))), dur, ""
		}
		return 0, 0, "no_fraction_for_unit_" + unit
	case exists:
		if unit != "seconds" {
			return 0, 0, "unknown_duration_unit_" + unit
		}
		if bodyDur <= 0 {
			return 0, 0, "unknown_duration"
		}
		if p, ok := secondsPos(bodyDur); ok {
			return p, bodyDur, ""
		}
		return 0, 0, "no_position"
	default:
		if bodyDur <= 0 {
			return 0, 0, "unknown_duration_no_row"
		}
		if p, ok := secondsPos(bodyDur); ok {
			return p, bodyDur, ""
		}
		return 0, 0, "no_position"
	}
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
