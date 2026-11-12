package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"catalogizer/internal/media/models"
	"catalogizer/internal/smartrules"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// CollectionHandler handles media collection CRUD endpoints.
type CollectionHandler struct {
	repo *repository.MediaCollectionRepository
	// isAdmin decides, from the database, whether a user is an administrator.
	// Without it the role_id claim in the request context is used.
	isAdmin func(ctx context.Context, userID int) (bool, error)
	// smartCreateMu: one mutex per owner id, serialising the smart-collection cap check with the insert.
	// Entries are never pruned; the map is bounded by the number of users.
	smartCreateMu sync.Map // int64 -> *sync.Mutex
}

// lockSmartCreate takes the per-owner create lock and returns its release function.
func (h *CollectionHandler) lockSmartCreate(uid int64) func() {
	m, _ := h.smartCreateMu.LoadOrStore(uid, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// WithAdminChecker installs the live admin check (production wires UserRepository.IsAdmin).
func (h *CollectionHandler) WithAdminChecker(f func(ctx context.Context, userID int) (bool, error)) *CollectionHandler {
	h.isAdmin = f
	return h
}

// actor returns the authenticated user id (0, false when the request carries none).
func actor(c *gin.Context) (int64, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	switch id := v.(type) {
	case int:
		return int64(id), id > 0
	case int64:
		return id, id > 0
	}
	return 0, false
}

// admin reports whether the caller is an administrator; an undeterminable answer is "no".
func (h *CollectionHandler) admin(c *gin.Context, uid int64) bool {
	if uid <= 0 {
		return false
	}
	if h.isAdmin != nil {
		ok, err := h.isAdmin(c.Request.Context(), int(uid))
		return err == nil && ok
	}
	if r, ok := c.Get("role_id"); ok {
		if id, isInt := r.(int); isInt {
			return id == 1 // seeded admin role
		}
	}
	return false
}

// canRead: admin, the owner, a public collection, or a legacy shared (ownerless) one.
func (h *CollectionHandler) canRead(c *gin.Context, coll *models.MediaCollection, uid int64) bool {
	if coll.OwnerUserID == nil || coll.IsPublic || (uid > 0 && *coll.OwnerUserID == uid) {
		return true
	}
	return h.admin(c, uid)
}

// canEdit: the owner or an administrator. Ownerless legacy collections are admin-only.
func (h *CollectionHandler) canEdit(c *gin.Context, coll *models.MediaCollection, uid int64) bool {
	if coll.OwnerUserID != nil && uid > 0 && *coll.OwnerUserID == uid {
		return true
	}
	return h.admin(c, uid)
}

// loadAccessible loads a collection and answers 404 (not visible / missing)
// without revealing that a private collection of another user exists.
func (h *CollectionHandler) loadAccessible(c *gin.Context, edit bool) (*models.MediaCollection, int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid collection ID", err)
		return nil, 0, false
	}
	coll, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Collection not found", err)
		return nil, 0, false
	}
	uid, _ := actor(c)
	if !h.canRead(c, coll, uid) {
		utils.SendErrorResponse(c, http.StatusNotFound, "Collection not found", nil)
		return nil, 0, false
	}
	if edit && !h.canEdit(c, coll, uid) {
		utils.SendErrorResponse(c, http.StatusForbidden, "Only the owner or an administrator can change this collection", nil)
		return nil, 0, false
	}
	return coll, uid, true
}

// evalOpts builds the rule-evaluation options for the viewer.
func (h *CollectionHandler) evalOpts(uid int64) smartrules.Options {
	return smartrules.Options{Postgres: h.repo.IsPostgres(), UserID: uid}
}

// liveCount evaluates a smart collection and returns its current size.
func (h *CollectionHandler) liveCount(c *gin.Context, coll *models.MediaCollection, uid int64) (int, error) {
	rs, err := smartrules.ParseStored(string(coll.SmartRules))
	if err != nil {
		return 0, err
	}
	_, total, err := smartrules.Evaluate(c.Request.Context(), h.repo.Querier(), rs, h.evalOpts(uid), 1, 0)
	return total, err
}

// NewCollectionHandler creates a new collection handler.
func NewCollectionHandler(repo *repository.MediaCollectionRepository) *CollectionHandler {
	return &CollectionHandler{repo: repo}
}

// ListCollections handles GET /api/v1/collections. Only collections the caller
// may see are returned (own, public, legacy shared; everything for an admin).
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

	uid, _ := actor(c)
	collections, total, err := h.repo.ListVisible(ctx, uid, h.admin(c, uid), limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to list collections", err)
		return
	}
	status := h.fillLiveCounts(c, collections, uid)
	list := collectionsToJSON(collections)
	for i, coll := range collections {
		if st, ok := status[coll.ID]; ok {
			list[i]["total_items_status"] = st
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items":  list,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// MaxSmartCollectionsPerUser bounds the rule evaluations a single user can accumulate.
const MaxSmartCollectionsPerUser = 50

const (
	// listCountCap bounds the work of one count shown in a LIST: counts above it are reported as "N+".
	listCountCap = 1000
	// listCountBudget is how many smart collections one list request evaluates; the rest keep the stored count.
	listCountBudget = 10
	countCacheTTL   = 30 * time.Second
	countCacheMax   = 2048
)

type cachedCount struct {
	n      int
	capped bool
	at     time.Time
}

// countCache remembers recent bounded counts per (viewer, collection, last update) so repeated list
// views do not re-run the rules; an edit changes updated_at and therefore the key.
var countCache = struct {
	sync.Mutex
	m map[string]cachedCount
}{m: map[string]cachedCount{}}

func countKey(uid int64, coll *models.MediaCollection) string {
	return fmt.Sprintf("%d/%d/%d", uid, coll.ID, coll.UpdatedAt.UnixNano())
}

// fillLiveCounts sets TotalItems of smart collections to their current size for the viewer, bounded:
// each count stops at listCountCap, at most listCountBudget collections are evaluated per request and
// results are cached for countCacheTTL. It returns, per collection id, "exact", "capped" (TotalItems is
// a lower bound) or "stored" (not evaluated: budget exhausted or the rules failed).
func (h *CollectionHandler) fillLiveCounts(c *gin.Context, collections []*models.MediaCollection, uid int64) map[int64]string {
	status := map[int64]string{}
	budget := listCountBudget
	for _, coll := range collections {
		if !coll.IsSmart {
			continue
		}
		status[coll.ID] = "stored"
		key := countKey(uid, coll)
		countCache.Lock()
		hit, ok := countCache.m[key]
		countCache.Unlock()
		if ok && time.Since(hit.at) < countCacheTTL {
			coll.TotalItems = hit.n
			status[coll.ID] = countStatus(hit.capped)
			continue
		}
		if budget == 0 {
			continue
		}
		budget--
		rs, err := smartrules.ParseStored(string(coll.SmartRules))
		if err != nil {
			continue
		}
		n, capped, err := smartrules.CountUpTo(c.Request.Context(), h.repo.Querier(), rs, h.evalOpts(uid), listCountCap)
		if err != nil {
			continue
		}
		coll.TotalItems = n
		status[coll.ID] = countStatus(capped)
		countCache.Lock()
		if len(countCache.m) >= countCacheMax {
			countCache.m = map[string]cachedCount{}
		}
		countCache.m[key] = cachedCount{n: n, capped: capped, at: time.Now()}
		countCache.Unlock()
	}
	return status
}

func countStatus(capped bool) string {
	if capped {
		return "capped"
	}
	return "exact"
}

// GetCollection handles GET /api/v1/collections/:id.
func (h *CollectionHandler) GetCollection(c *gin.Context) {
	coll, uid, ok := h.loadAccessible(c, false)
	if !ok {
		return
	}
	if coll.IsSmart {
		n, err := h.liveCount(c, coll, uid)
		if err != nil {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to evaluate smart collection", err)
			return
		}
		coll.TotalItems = n
	}
	c.JSON(http.StatusOK, coll)
}

// ListCollectionItems handles GET /api/v1/collections/:id/items.
// A smart collection is evaluated NOW against the catalogue (it is always live);
// a manual collection lists its stored members. Paging: limit (default 50, max
// 500) with offset, or page (1-based).
func (h *CollectionHandler) ListCollectionItems(c *gin.Context) {
	coll, uid, ok := h.loadAccessible(c, false)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, offset = smartrules.NormalizePage(limit, offset)
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p >= 1 && c.Query("offset") == "" {
		if p > 1_000_000 {
			p = 1_000_000
		}
		page = p
		offset = (p - 1) * limit
	} else {
		page = offset/limit + 1
	}

	var (
		items []smartrules.Item
		total int
		err   error
	)
	if coll.IsSmart {
		var rs *smartrules.RuleSet
		rs, err = smartrules.ParseStored(string(coll.SmartRules))
		if err == nil {
			items, total, err = smartrules.Evaluate(c.Request.Context(), h.repo.Querier(), rs, h.evalOpts(uid), limit, offset)
		}
	} else {
		items, total, err = h.repo.ListManualItems(c.Request.Context(), coll.ID, limit, offset)
	}
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to list collection items", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":    items,
		"total":    total,
		"page":     page,
		"limit":    limit,
		"offset":   offset,
		"is_smart": coll.IsSmart,
	})
}

const maxCollectionNameLen = 200

// parseSmartRules validates the rule list of a request and answers 400 itself.
func parseSmartRules(c *gin.Context, raws []json.RawMessage) (*smartrules.RuleSet, bool) {
	rs, err := smartrules.Parse(raws)
	if err != nil {
		if smartrules.IsValidation(err) {
			utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid smart_rules: "+err.Error(), nil)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to process smart_rules", err)
		}
		return nil, false
	}
	return rs, true
}

// verifyRules runs the rules once: a rule set that cannot be evaluated is
// refused instead of being stored and failing at every read.
func (h *CollectionHandler) verifyRules(c *gin.Context, rs *smartrules.RuleSet, uid int64) (int, bool) {
	_, total, err := smartrules.Evaluate(c.Request.Context(), h.repo.Querier(), rs, h.evalOpts(uid), 1, 0)
	if err != nil {
		if smartrules.IsValidation(err) {
			// ValidationError text is fixed-format and client-correctable (like parseSmartRules).
			utils.SendErrorResponse(c, http.StatusBadRequest, "Smart collection rules could not be evaluated: "+err.Error(), err)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Smart collection rules could not be evaluated", err)
		}
		return 0, false
	}
	return total, true
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

	// Article XI §11.5: enforce a sane upper bound on the collection
	// name. Without it, a 500-character or 5000-character name is
	// accepted and stored — a UI rendering / database-index hazard.
	// 200 chars is well above the longest legitimate movie/album/
	// series name on TMDB, OMDB or MusicBrainz. Caught by FQA-API-171.
	if len(req.Name) > maxCollectionNameLen {
		utils.SendErrorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("name too long (max %d characters)", maxCollectionNameLen),
			nil)
		return
	}

	if req.IsSmart != nil && !*req.IsSmart && len(req.SmartRules) > 0 {
		utils.SendErrorResponse(c, http.StatusBadRequest, "smart_rules given but is_smart is false", nil)
		return
	}
	wantSmart := (req.IsSmart != nil && *req.IsSmart) || len(req.SmartRules) > 0

	uid, authed := actor(c)
	coll := &models.MediaCollection{
		Name:        req.Name,
		Description: req.Description,
		ExternalIDs: req.ExternalIDs,
		CoverURL:    req.CoverURL,
		IsPublic:    req.IsPublic != nil && *req.IsPublic,
	}
	if authed {
		coll.OwnerUserID = &uid
	}

	if wantSmart {
		if !authed {
			utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required for smart collections", nil)
			return
		}
		rs, ok := parseSmartRules(c, req.SmartRules)
		if !ok {
			return
		}
		// run the rules once, OUTSIDE the lock (slow, independent of the count)
		total, ok := h.verifyRules(c, rs, uid)
		if !ok {
			return
		}
		// SECQ-4b: count and insert must be one atomic step per owner, or concurrent creates from cap-1 all pass the
		// check. The lock is held through the insert below (defer). CAVEAT: in-process only; several server instances
		// sharing one database (or PostgreSQL behind a load balancer) still need a DB-level guarantee.
		unlock := h.lockSmartCreate(uid)
		defer unlock()
		n, err := h.repo.CountSmartByOwner(ctx, uid)
		if err != nil {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to create collection", err)
			return
		}
		if n >= MaxSmartCollectionsPerUser {
			utils.SendErrorResponse(c, http.StatusConflict,
				fmt.Sprintf("smart collection limit reached (max %d per user)", MaxSmartCollectionsPerUser), nil)
			return
		}
		coll.IsSmart = true
		coll.SmartRules = json.RawMessage(rs.Raw)
		coll.TotalItems = total
		coll.CollectionType = "smart"
	}

	// Set default collection type if empty
	if req.CollectionType != "" {
		coll.CollectionType = req.CollectionType
	} else if coll.CollectionType == "" {
		coll.CollectionType = "custom"
	}

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

	var req struct {
		Name           *string           `json:"name,omitempty"`
		CollectionType *string           `json:"collection_type,omitempty"`
		Description    *string           `json:"description,omitempty"`
		IsPublic       *bool             `json:"is_public,omitempty"`
		IsSmart        *bool             `json:"is_smart,omitempty"`
		SmartRules     []json.RawMessage `json:"smart_rules,omitempty"`
		ExternalIDs    map[string]string `json:"external_ids,omitempty"`
		CoverURL       *string           `json:"cover_url,omitempty"`
	}
	// An invalid id is answered before the body is read.
	if _, err := strconv.ParseInt(c.Param("id"), 10, 64); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid collection ID", err)
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	existing, uid, ok := h.loadAccessible(c, true)
	if !ok {
		return
	}

	if req.Name != nil && len(*req.Name) > maxCollectionNameLen {
		utils.SendErrorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("name too long (max %d characters)", maxCollectionNameLen), nil)
		return
	}
	// A collection is smart or manual for its whole life: converting loses either
	// the rules or the members, so it is refused instead of guessed.
	if existing.IsSmart {
		if req.IsSmart != nil && !*req.IsSmart {
			utils.SendErrorResponse(c, http.StatusBadRequest, "a smart collection cannot be converted to a manual one; create a new collection", nil)
			return
		}
	} else if (req.IsSmart != nil && *req.IsSmart) || len(req.SmartRules) > 0 {
		utils.SendErrorResponse(c, http.StatusBadRequest, "a manual collection cannot become smart; create a new smart collection", nil)
		return
	}
	if existing.IsSmart && req.SmartRules != nil {
		rs, ok := parseSmartRules(c, req.SmartRules)
		if !ok {
			return
		}
		total, ok := h.verifyRules(c, rs, uid)
		if !ok {
			return
		}
		existing.SmartRules = json.RawMessage(rs.Raw)
		existing.TotalItems = total
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
	if req.IsPublic != nil {
		existing.IsPublic = *req.IsPublic
	}

	if err := h.repo.Update(ctx, existing); err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update collection", err)
		return
	}

	c.JSON(http.StatusOK, existing)
}

// DeleteCollection handles DELETE /api/v1/collections/:id.
func (h *CollectionHandler) DeleteCollection(c *gin.Context) {
	ctx := c.Request.Context()

	coll, _, ok := h.loadAccessible(c, true)
	if !ok {
		return
	}

	if err := h.repo.Delete(ctx, coll.ID); err != nil {
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
			"is_smart":        coll.IsSmart,
			"smart_rules":     smartRulesJSON(coll.SmartRules),
			"is_public":       coll.IsPublic,
			"owner_id":        coll.OwnerUserID,
			"created_at":      coll.CreatedAt,
			"updated_at":      coll.UpdatedAt,
		})
	}
	return result
}

// smartRulesJSON returns the stored rules as raw JSON, or an empty list.
func smartRulesJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}
