package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"catalogizer/internal/media/models"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// CollectionHandler handles media collection CRUD endpoints.
type CollectionHandler struct {
	repo *repository.MediaCollectionRepository
}

// NewCollectionHandler creates a new collection handler.
func NewCollectionHandler(repo *repository.MediaCollectionRepository) *CollectionHandler {
	return &CollectionHandler{repo: repo}
}

// ListCollections handles GET /api/v1/collections.
func (h *CollectionHandler) ListCollections(c *gin.Context) {
	ctx := c.Request.Context()

	limitStr := c.DefaultQuery("limit", "24")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	if limit <= 0 || limit > 200 {
		limit = 24
	}
	if offset < 0 {
		offset = 0
	}

	collections, total, err := h.repo.List(ctx, limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to list collections", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":  collectionsToJSON(collections),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetCollection handles GET /api/v1/collections/:id.
func (h *CollectionHandler) GetCollection(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid collection ID", err)
		return
	}

	coll, err := h.repo.GetByID(ctx, id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Collection not found", err)
		return
	}

	c.JSON(http.StatusOK, coll)
}

// CreateCollection handles POST /api/v1/collections.
func (h *CollectionHandler) CreateCollection(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Name           string            `json:"name" binding:"required"`
		CollectionType string            `json:"collection_type,omitempty" binding:"-"`
		Description    *string           `json:"description,omitempty"`
		IsPublic       *bool             `json:"is_public,omitempty"`
		IsSmart        *bool             `json:"is_smart,omitempty"`
		SmartRules     []json.RawMessage `json:"smart_rules,omitempty"`
		ExternalIDs    map[string]string `json:"external_ids,omitempty"`
		CoverURL       *string           `json:"cover_url,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// CAT-001 (R4-4/R4-9): smart collections are not stored or evaluated. Creating a plain
	// collection and answering 201 would silently drop the rules, so refuse honestly.
	if (req.IsSmart != nil && *req.IsSmart) || len(req.SmartRules) > 0 {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "smart collections are not supported: is_smart and smart_rules are neither stored nor evaluated. Nothing was created.",
		})
		return
	}

	// Article XI §11.5: enforce a sane upper bound on the collection
	// name. Without it, a 500-character or 5000-character name is
	// accepted and stored — a UI rendering / database-index hazard.
	// 200 chars is well above the longest legitimate movie/album/
	// series name on TMDB, OMDB or MusicBrainz. Caught by FQA-API-171.
	const maxCollectionNameLen = 200
	if len(req.Name) > maxCollectionNameLen {
		utils.SendErrorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("name too long (max %d characters)", maxCollectionNameLen),
			nil)
		return
	}

	// Set default collection type if empty
	collectionType := req.CollectionType
	if collectionType == "" {
		collectionType = "custom"
	}
	// Map request to model
	coll := &models.MediaCollection{
		Name:           req.Name,
		CollectionType: collectionType,
		Description:    req.Description,
		ExternalIDs:    req.ExternalIDs,
		CoverURL:       req.CoverURL,
		TotalItems:     0,
	}
	// is_public and is_smart are not stored in current schema; ignore for now

	id, err := h.repo.Create(ctx, coll)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to create collection", err)
		return
	}

	coll.ID = id
	c.JSON(http.StatusCreated, coll)
}

// UpdateCollection handles PUT /api/v1/collections/:id.
func (h *CollectionHandler) UpdateCollection(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid collection ID", err)
		return
	}

	var req struct {
		Name           *string           `json:"name,omitempty"`
		CollectionType *string           `json:"collection_type,omitempty"`
		Description    *string           `json:"description,omitempty"`
		IsPublic       *bool             `json:"is_public,omitempty"`
		IsSmart        *bool             `json:"is_smart,omitempty"`
		ExternalIDs    map[string]string `json:"external_ids,omitempty"`
		CoverURL       *string           `json:"cover_url,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.IsSmart != nil && *req.IsSmart {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "smart collections are not supported: is_smart is neither stored nor evaluated. Nothing was changed.",
		})
		return
	}

	// Fetch existing collection
	existing, err := h.repo.GetByID(ctx, id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Collection not found", err)
		return
	}

	// Apply updates
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.CollectionType != nil {
		existing.CollectionType = *req.CollectionType
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.ExternalIDs != nil {
		existing.ExternalIDs = req.ExternalIDs
	}
	if req.CoverURL != nil {
		existing.CoverURL = req.CoverURL
	}
	// is_public and is_smart ignored

	if err := h.repo.Update(ctx, existing); err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update collection", err)
		return
	}

	c.JSON(http.StatusOK, existing)
}

// DeleteCollection handles DELETE /api/v1/collections/:id.
func (h *CollectionHandler) DeleteCollection(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid collection ID", err)
		return
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		// Article XI §11.5: distinguish "not found" from real
		// internal errors so DELETE on a missing ID returns 404, not
		// 500. Caught by FQA-API-168 in the 2026-04-29 real-binary
		// bank verification.
		if isNotFoundError(err) {
			utils.SendErrorResponse(c, http.StatusNotFound, "Collection not found", err)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to delete collection", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Collection deleted",
	})
}

// isNotFoundError returns true when the given error is a database
// "not found" sentinel — sql.ErrNoRows or repository-level error
// strings containing "not found". Used by handlers to map 500 → 404
// for missing-resource cases.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if err == sql.ErrNoRows {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no rows in result set")
}

// validEntityTypes is the closed set of media-entity types favorites
// (and similar entity-keyed APIs) can attach to. Mirrors the seed in
// `database/migrations/*` for the `media_types` table. Any caller
// passing an out-of-set entity_type is sending invalid input and the
// handler MUST reject it with 400 — silently returning is_favorite or
// 200 lets stale / corrupt rows masquerade as legitimate state.
// Caught by FQA-API-220 in the 2026-04-29 real-binary bank
// verification: GET /favorites/check/invalid_type/1 returned
// HTTP 200 {"is_favorite":true}.
var validEntityTypes = map[string]struct{}{
	"movie":        {},
	"tv_show":      {},
	"tv_season":    {},
	"tv_episode":   {},
	"music_artist": {},
	"music_album":  {},
	"song":         {},
	"game":         {},
	"software":     {},
	"book":         {},
	"comic":        {},
}

// isValidEntityType returns true iff the entity_type is one of the
// closed set defined above.
func isValidEntityType(entityType string) bool {
	_, ok := validEntityTypes[entityType]
	return ok
}

// collectionsToJSON converts slice of MediaCollection to JSON-friendly slice.
func collectionsToJSON(collections []*models.MediaCollection) []gin.H {
	result := make([]gin.H, 0, len(collections))
	for _, coll := range collections {
		result = append(result, gin.H{
			"id":              coll.ID,
			"name":            coll.Name,
			"collection_type": coll.CollectionType,
			"description":     coll.Description,
			"total_items":     coll.TotalItems,
			"external_ids":    coll.ExternalIDs,
			"cover_url":       coll.CoverURL,
			"created_at":      coll.CreatedAt,
			"updated_at":      coll.UpdatedAt,
		})
	}
	return result
}
