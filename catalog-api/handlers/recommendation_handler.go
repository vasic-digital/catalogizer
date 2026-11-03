// This is the new recommendation handler for Gin
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"catalogizer/internal/services"
	"catalogizer/models"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

type RecommendationHandler struct {
	recommendationService *services.RecommendationService
}

func NewRecommendationHandler(
	recommendationService *services.RecommendationService,
) *RecommendationHandler {
	return &RecommendationHandler{
		recommendationService: recommendationService,
	}
}

// GetSimilarItems retrieves similar items for a given media
// @Summary Get similar media items
// @Description Retrieves similar media items based on content similarity and external APIs
// @Tags recommendations
// @Accept json
// @Produce json
// @Param media_id path int true "Media ID"
// @Param max_local_items query int false "Maximum local items to return (default: 10)"
// @Param max_external_items query int false "Maximum external items to return (default: 5)"
// @Param include_external query bool false "Include external recommendations (default: false)"
// @Param similarity_threshold query number false "Minimum similarity threshold (default: 0.3)"
// @Success 200 {object} services.SimilarItemsResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/recommendations/similar/{media_id} [get]
func (h *RecommendationHandler) GetSimilarItems(c *gin.Context) {
	ctx := c.Request.Context()

	// Get media ID from URL
	mediaIDStr := c.Param("media_id")
	mediaID, err := strconv.ParseInt(mediaIDStr, 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media ID", err)
		return
	}

	// Parse query parameters
	maxLocalItems, _ := strconv.Atoi(c.DefaultQuery("max_local_items", "10"))
	maxExternalItems, _ := strconv.Atoi(c.DefaultQuery("max_external_items", "5"))
	includeExternal := c.DefaultQuery("include_external", "false") == "true"
	similarityThreshold, _ := strconv.ParseFloat(c.DefaultQuery("similarity_threshold", "0.3"), 64)

	// Get media metadata from database
	mediaMetadata, err := h.getMediaMetadata(ctx, mediaID)
	if err != nil {
		// Article XI §11.5: getMediaMetadata uses sql.QueryRow which
		// returns sql.ErrNoRows on a missing media id. The previous
		// equality check ("media not found") missed that case and
		// the handler returned 500 instead of 404. Caught by
		// FQA-API-099 in the 2026-04-29 real-binary bank verification.
		if isNotFoundError(err) {
			utils.SendErrorResponse(c, http.StatusNotFound, "Media not found", nil)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get media metadata", err)
		return
	}

	// Create recommendation request
	req := &services.SimilarItemsRequest{
		MediaID:             mediaIDStr,
		MediaMetadata:       mediaMetadata,
		MaxLocalItems:       maxLocalItems,
		MaxExternalItems:    maxExternalItems,
		IncludeExternal:     includeExternal,
		SimilarityThreshold: similarityThreshold,
	}

	// Get recommendations
	response, err := h.recommendationService.GetSimilarItems(ctx, req)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get recommendations", err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetTrendingItems retrieves trending media items
// @Summary Get trending media
// @Description Retrieves trending media items based on recent activity and ratings
// @Tags recommendations
// @Accept json
// @Produce json
// @Param media_type query string false "Filter by media type"
// @Param limit query int false "Maximum items to return (default: 20)"
// @Param time_range query string false "Time range: day, week, month, year (default: week)"
// @Success 200 {object} TrendingResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/recommendations/trending [get]
func (h *RecommendationHandler) GetTrendingItems(c *gin.Context) {
	// No trending-analytics source exists yet. Say so instead of inventing
	// titles (CAT-001): an honest 501 beats fabricated catalogue items.
	utils.SendErrorResponse(c, http.StatusNotImplemented, "Trending recommendations are not implemented", ErrRecommendationsNotImplemented)
}

// PersonalizedRecommendations retrieves personalized recommendations for a user
// @Summary Get personalized recommendations
// @Description Retrieves personalized recommendations based on user's viewing history and preferences
// @Tags recommendations
// @Accept json
// @Produce json
// @Param user_id path int true "User ID"
// @Param limit query int false "Maximum items to return (default: 20)"
// @Success 200 {object} PersonalizedResponse
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/recommendations/personalized/{user_id} [get]
func (h *RecommendationHandler) GetPersonalizedRecommendations(c *gin.Context) {
	// Get user ID from URL
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid user ID", err)
		return
	}

	// IDOR guard: only the account owner (or an admin) may ask for a user's
	// recommendations.
	callerID, ok := callerUserID(c)
	if !ok {
		utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}
	if callerID != userID && !callerIsAdmin(c) {
		utils.SendErrorResponse(c, http.StatusForbidden, "Cannot access another user's recommendations", nil)
		return
	}

	// No personalisation engine is wired yet; never invent titles (CAT-001).
	utils.SendErrorResponse(c, http.StatusNotImplemented, "Personalized recommendations are not implemented", ErrRecommendationsNotImplemented)
}

// ErrRecommendationsNotImplemented marks recommendation endpoints that have no
// real data source behind them yet.
var ErrRecommendationsNotImplemented = errors.New("recommendation source not implemented")

// callerUserID reads the authenticated user id set by the JWT middleware.
func callerUserID(c *gin.Context) (int64, bool) {
	v, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	switch id := v.(type) {
	case int:
		return int64(id), true
	case int64:
		return id, true
	case string:
		n, err := strconv.ParseInt(id, 10, 64)
		return n, err == nil
	}
	return 0, false
}

func callerIsAdmin(c *gin.Context) bool {
	v, exists := c.Get("role_id")
	if !exists {
		return false
	}
	id, ok := v.(int)
	return ok && id == 1 // middleware.RoleAdminID
}

// getMediaMetadata retrieves media metadata from database
func (h *RecommendationHandler) getMediaMetadata(ctx context.Context, mediaID int64) (*models.MediaMetadata, error) {
	// Query the canonical media_items schema (identical on SQLite and
	// PostgreSQL). The legacy columns this handler used to select
	// (media_type, producer, "cast", resolution, file_size) only exist on
	// PostgreSQL via best-effort ALTERs and are NULL for every row written by
	// the real pipeline, which made every request fail with
	// "converting NULL to string is unsupported" (HTTP 500). The type name
	// comes from media_types; every nullable column is COALESCEd.
	query := `
		SELECT
			mi.id, mi.title, COALESCE(mt.name, ''), mi.year,
			COALESCE(mi.description, ''), mi.rating, mi.runtime,
			COALESCE(mi.language, ''), COALESCE(mi.country, ''),
			COALESCE(mi.director, ''), COALESCE(mi.genre, ''),
			COALESCE(mi.cast_crew, ''),
			mi.first_detected, mi.last_updated
		FROM media_items mi
		LEFT JOIN media_types mt ON mt.id = mi.media_type_id
		WHERE mi.id = ?
	`

	var metadata models.MediaMetadata
	var castStr string
	var firstDetected, lastUpdated sql.NullTime
	err := h.recommendationService.GetDB().QueryRowContext(ctx, query, mediaID).Scan(
		&metadata.ID, &metadata.Title, &metadata.MediaType, &metadata.Year,
		&metadata.Description, &metadata.Rating, &metadata.Duration,
		&metadata.Language, &metadata.Country, &metadata.Director,
		&metadata.Genre, &castStr, &firstDetected, &lastUpdated,
	)

	if err != nil {
		return nil, err
	}
	if firstDetected.Valid {
		metadata.CreatedAt = firstDetected.Time
	}
	if lastUpdated.Valid {
		metadata.UpdatedAt = lastUpdated.Time
	}

	// Parse cast array if present
	if castStr != "" {
		// Simple parsing - in production, use proper JSON parsing
		metadata.Cast = []string{castStr}
	}

	return &metadata, nil
}

// TrendingResponse represents the response for trending items
type TrendingResponse struct {
	Items       []*models.MediaCatalogItem `json:"items"`
	MediaType   string                     `json:"media_type"`
	TimeRange   string                     `json:"time_range"`
	GeneratedAt time.Time                  `json:"generated_at"`
}

// PersonalizedResponse represents the response for personalized recommendations
type PersonalizedResponse struct {
	UserID      int64                      `json:"user_id"`
	Items       []*models.MediaCatalogItem `json:"items"`
	GeneratedAt time.Time                  `json:"generated_at"`
}
