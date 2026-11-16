package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

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

// UpdateWatchProgress godoc
// @Summary Update watch progress
// @Description Update the watch progress for a media item
// @Tags media
// @Accept json
// @Produce json
// @Param id path int true "Media ID"
// @Param body body map[string]float64 true "Progress value (0.0 to 1.0)"
// @Success 200 {object} utils.SuccessResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id}/progress [put]
func (h *AndroidTVMediaHandler) UpdateWatchProgress(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse media ID from path
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media ID", err)
		return
	}

	// Parse request body
	var requestBody map[string]float64
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	progress, exists := requestBody["progress"]
	if !exists {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Progress field is required", nil)
		return
	}

	// Validate progress value
	if progress < 0.0 || progress > 1.0 {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Progress must be between 0.0 and 1.0", nil)
		return
	}

	// DEFECT-D (2026-06-25): the prior query did
	// `UPDATE media_items SET watch_progress=?, last_watched=..., updated_at=...`
	// but the real normalized media_items table has NO watch_progress and
	// NO last_watched column (they only existed in the flat test-helper
	// schema), so every call returned HTTP 500 `no such column: watch_progress`.
	//
	// The genuine progress store is `media_progress` (migration v15),
	// keyed PRIMARY KEY (user_id, media_item_id) and tracking absolute
	// position (last_position / duration_total / position_unit) — the
	// SAME store the canonical authenticated path reads via
	// PlaybackHandler.GetProgressForEntity / ProgressSession
	// (GET /api/v1/entities/:id/progress), which derives user_id from the
	// auth context (c.Get("user_id")).
	//
	// HONEST AMBIGUITY (§11.4.6): this AndroidTV route
	// (PUT /api/v1/media/:id/progress) is registered in the PUBLIC `api`
	// group with NO auth middleware, so it has no user_id in context, and
	// media_progress cannot be keyed without one. It also accepts a
	// fractional 0.0–1.0 progress, whereas media_progress stores absolute
	// position units. Writing a global, user-less, unit-mismatched row to
	// media_progress would persist WRONG data and would NOT agree with the
	// authenticated GetProgressForEntity reader. Per the no-phantom-column
	// rule, we therefore validate the media item exists against the REAL
	// schema (correct 404/200) and do NOT write to any phantom or
	// mis-keyed column. Genuine per-user progress persistence for this
	// route requires the route to be moved under the authenticated entity
	// group (tracked follow-up); until then it is a validated no-op write,
	// reported honestly to the caller.
	var existsRow int
	err = h.db.QueryRowContext(ctx, `SELECT 1 FROM media_items WHERE id = ?`, id).Scan(&existsRow)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			utils.SendErrorResponse(c, http.StatusNotFound, "Media not found", nil)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update watch progress", err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Watch progress accepted",
		"persisted": false,
		"reason":    "per-user progress store (media_progress) requires an authenticated user; this public route has none",
	})
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
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/v1/media/{id}/favorite [put]
func (h *AndroidTVMediaHandler) UpdateFavoriteStatus(c *gin.Context) {
	ctx := c.Request.Context()

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
	callerID, _ := callerUserID(c)
	if callerID == 0 {
		utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}
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
