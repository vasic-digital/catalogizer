package handlers

import (
	"io"
	"net/http"
	"strconv"

	"catalogizer/repository"

	"digital.vasic.assets/pkg/asset"
	"digital.vasic.assets/pkg/manager"
	"digital.vasic.assets/pkg/resolver"
	"github.com/gin-gonic/gin"
)

// AssetHandler serves asset content and manages asset requests.
type AssetHandler struct {
	manager *manager.Manager
	repo    *repository.AssetRepository
}

// NewAssetHandler creates a new asset handler.
func NewAssetHandler(mgr *manager.Manager, repo *repository.AssetRepository) *AssetHandler {
	return &AssetHandler{manager: mgr, repo: repo}
}

// ServeAsset serves asset content by ID.
// GET /api/v1/assets/:id
func (h *AssetHandler) ServeAsset(c *gin.Context) {
	id := asset.ID(c.Param("id"))

	// Look up asset metadata from DB for type-aware defaults
	dbAsset, _ := h.repo.GetAsset(c.Request.Context(), id)

	var (
		content   io.ReadCloser
		ct        string
		size      int64
		isDefault bool
		err       error
	)

	if dbAsset != nil {
		rc, si, def, e := h.manager.GetTyped(c.Request.Context(), id, dbAsset.Type)
		content, isDefault, err = rc, def, e
		if e == nil {
			ct, size = si.ContentType, si.Size
		}
	} else {
		rc, si, def, e := h.manager.Get(c.Request.Context(), id)
		content, isDefault, err = rc, def, e
		if e == nil {
			ct, size = si.ContentType, si.Size
		}
	}

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}
	defer content.Close()

	if isDefault {
		c.Header("X-Asset-Status", "pending")
		c.Header("Cache-Control", "no-cache")
	} else {
		c.Header("X-Asset-Status", "ready")
		c.Header("Cache-Control", "public, max-age=86400")
	}

	c.Header("Content-Type", ct)
	if size > 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}

	c.Status(http.StatusOK)
	io.Copy(c.Writer, content)
}

// RequestAsset handles asset resolution requests.
// POST /api/v1/assets/request
func (h *AssetHandler) RequestAsset(c *gin.Context) {
	var req struct {
		Type       string `json:"type" binding:"required"`
		SourceHint string `json:"source_hint"`
		EntityType string `json:"entity_type" binding:"required"`
		EntityID   string `json:"entity_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadBody(c, err)
		return
	}

	// Create asset record in DB
	a := asset.New(asset.Type(req.Type), req.EntityType, req.EntityID)
	a.SourceHint = req.SourceHint

	if err := h.repo.CreateAsset(c.Request.Context(), a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create asset"})
		return
	}

	// Submit for background resolution
	resolveReq := &resolver.ResolveRequest{
		AssetID:    a.ID,
		AssetType:  a.Type,
		SourceHint: req.SourceHint,
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
	}

	h.manager.Request(c.Request.Context(), resolveReq)

	c.JSON(http.StatusOK, gin.H{
		"asset_id": string(a.ID),
		"status":   string(a.Status),
	})
}

// GetByEntity returns asset metadata for an entity.
// GET /api/v1/assets/by-entity/:type/:id
func (h *AssetHandler) GetByEntity(c *gin.Context) {
	entityType := c.Param("type")
	entityID := c.Param("id")

	assets, err := h.repo.FindByEntity(c.Request.Context(), entityType, entityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query assets"})
		return
	}

	result := make([]gin.H, 0, len(assets))
	for _, a := range assets {
		item := gin.H{
			"id":           string(a.ID),
			"type":         string(a.Type),
			"status":       string(a.Status),
			"content_type": a.ContentType,
			"size":         a.Size,
			"entity_type":  a.EntityType,
			"entity_id":    a.EntityID,
			"created_at":   a.CreatedAt,
			"updated_at":   a.UpdatedAt,
		}
		if a.ResolvedAt != nil {
			item["resolved_at"] = a.ResolvedAt
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, result)
}
