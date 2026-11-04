package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"catalogizer/database"
	"catalogizer/internal/logging"
	"catalogizer/internal/media/models"
	"catalogizer/internal/media/providers"
	"catalogizer/internal/services"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/proxy"
)

// SearchEntities handles GET /api/v1/entities/search — entity-
// level full-text search that returns structured media items
// (movies, tv shows, albums, books, ...) instead of raw files.
// The TV and mobile clients previously hit this path and got a
// "Invalid entity ID" error because the router matched
// /entities/:id with id="search". Registering a dedicated route
// before /:id (see main.go) fixes it, and this handler wraps the
// existing repository Search with the same q/query/search param
// aliases that ListEntities accepts.
func (h *MediaEntityHandler) SearchEntities(c *gin.Context) {
	ctx := c.Request.Context()

	query := c.Query("q")
	if query == "" {
		query = c.Query("query")
	}
	if query == "" {
		query = c.Query("search")
	}
	mediaType := c.Query("type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 24
	}
	if offset < 0 {
		offset = 0
	}

	// Empty query: return empty result set with 200 so autocomplete
	// UIs can fire the request as the user types without getting a
	// 400.
	if query == "" {
		c.JSON(http.StatusOK, gin.H{
			"items":  []interface{}{},
			"total":  0,
			"limit":  limit,
			"offset": offset,
			"query":  "",
		})
		return
	}

	var mediaTypeIDs []int64
	if mediaType != "" {
		_, typeID, err := h.itemRepo.GetMediaTypeByName(ctx, mediaType)
		if err != nil {
			utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media type", err)
			return
		}
		mediaTypeIDs = append(mediaTypeIDs, typeID)
	}

	items, total, err := h.itemRepo.Search(ctx, query, mediaTypeIDs, limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Entity search failed", err)
		return
	}

	jsonItems := itemsToJSON(items)
	h.enrichItemsWithCoverURLs(ctx, items, jsonItems)

	c.JSON(http.StatusOK, gin.H{
		"items":  jsonItems,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"query":  query,
	})
}

// tmdbCooldown tracks consecutive TMDB failures and backoffs.
var tmdbCooldown struct {
	mu       sync.RWMutex
	failures int
	until    time.Time
}

func isTMDBOnCooldown() bool {
	tmdbCooldown.mu.RLock()
	defer tmdbCooldown.mu.RUnlock()
	return time.Now().Before(tmdbCooldown.until)
}

func recordTMDBFailure() {
	tmdbCooldown.mu.Lock()
	defer tmdbCooldown.mu.Unlock()
	tmdbCooldown.failures++
	backoff := time.Duration(1<<min(tmdbCooldown.failures-1, 5)) * time.Minute
	if backoff > 30*time.Minute {
		backoff = 30 * time.Minute
	}
	tmdbCooldown.until = time.Now().Add(backoff)
}

func recordTMDBSuccess() {
	tmdbCooldown.mu.Lock()
	defer tmdbCooldown.mu.Unlock()
	tmdbCooldown.failures = 0
	tmdbCooldown.until = time.Time{}
}

// Note: primary-file selection logic moved to
// repository.MediaFileRepository.GetPrimaryStreamableFile so it
// can JOIN media_files against the files table in a single query
// and score candidates by real name / extension / size. The
// StreamEntity and DownloadEntity handlers call that method
// directly instead of filtering an in-memory MediaFileRecord
// slice that only carries file_id + is_primary.

// MediaEntityHandler handles entity-level media browsing endpoints.
type MediaEntityHandler struct {
	itemRepo        *repository.MediaItemRepository
	fileRepo        *repository.MediaFileRepository
	extMetaRepo     *repository.ExternalMetadataRepository
	userMetaRepo    *repository.UserMetadataRepository
	coverArtService *services.CoverArtService
	db              *database.DB
	enrichWg        sync.WaitGroup // tracks background TMDB enrichment goroutines
	proxyCfg        providers.ProxyConfiger
	llmProvider     *providers.LLMProvider
}

// NewMediaEntityHandler creates a new media entity handler.
func NewMediaEntityHandler(
	itemRepo *repository.MediaItemRepository,
	fileRepo *repository.MediaFileRepository,
	extMetaRepo *repository.ExternalMetadataRepository,
	userMetaRepo *repository.UserMetadataRepository,
	dbArgs ...*database.DB,
) *MediaEntityHandler {
	h := &MediaEntityHandler{
		itemRepo:     itemRepo,
		fileRepo:     fileRepo,
		extMetaRepo:  extMetaRepo,
		userMetaRepo: userMetaRepo,
	}
	if len(dbArgs) > 0 && dbArgs[0] != nil {
		h.db = dbArgs[0]
		if logging.Logger != nil {
			logging.With(logging.String("handler", "MediaEntityHandler")).Info("Database connected for metadata enrichment")
		}
	} else {
		if logging.Logger != nil {
			logging.With(logging.String("handler", "MediaEntityHandler")).Warn("No database provided, enrichment disabled")
		}
	}
	return h
}

// SetCoverArtService sets the cover art service for cover URL enrichment.
func (h *MediaEntityHandler) SetCoverArtService(cas *services.CoverArtService) {
	h.coverArtService = cas
}

// SetProxyConfig sets the proxy configuration for external API calls.
func (h *MediaEntityHandler) SetProxyConfig(cfg providers.ProxyConfiger) {
	h.proxyCfg = cfg
}

// SetLLMProvider sets the LLM provider for metadata fallback.
func (h *MediaEntityHandler) SetLLMProvider(lp *providers.LLMProvider) {
	h.llmProvider = lp
}

// ListEntities handles GET /api/v1/entities — list entities with filters and pagination.
func (h *MediaEntityHandler) ListEntities(c *gin.Context) {
	ctx := c.Request.Context()

	// Accept both ?query= (original) and ?q= (short form used by
	// the TV/mobile clients and by HelixQA banks) as the search
	// term. Without this alias the TV search box silently
	// returned every entity because the server ignored ?q=.
	query := c.Query("query")
	if query == "" {
		query = c.Query("q")
	}
	if query == "" {
		query = c.Query("search")
	}
	mediaType := c.Query("type")
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

	var mediaTypeIDs []int64
	if mediaType != "" {
		_, typeID, err := h.itemRepo.GetMediaTypeByName(ctx, mediaType)
		if err != nil {
			utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media type", err)
			return
		}
		mediaTypeIDs = append(mediaTypeIDs, typeID)
	}

	if query == "" {
		query = "%"
	}

	items, total, err := h.itemRepo.Search(ctx, query, mediaTypeIDs, limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to list entities", err)
		return
	}

	jsonItems := itemsToJSON(items)
	h.enrichItemsWithCoverURLs(ctx, items, jsonItems)

	c.JSON(http.StatusOK, gin.H{
		"items":  jsonItems,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetEntity handles GET /api/v1/entities/:id — get entity with details.
func (h *MediaEntityHandler) GetEntity(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	item, err := h.itemRepo.GetByID(ctx, id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Entity not found", err)
		return
	}

	// Get file count
	fileCount, _ := h.fileRepo.CountByItem(ctx, id)

	// Get children count
	children, _ := h.itemRepo.GetChildren(ctx, id)
	childrenCount := len(children)

	// Get external metadata
	extMeta, _ := h.extMetaRepo.GetByItem(ctx, id)

	// Get media type name
	types, _ := h.itemRepo.GetMediaTypes(ctx)
	typeName := ""
	for _, mt := range types {
		if mt.ID == item.MediaTypeID {
			typeName = mt.Name
			break
		}
	}

	result := entityDetailJSON(item, typeName, fileCount, int64(childrenCount), extMeta)

	// Enrich with cover URL
	if h.coverArtService != nil {
		result["cover_url"] = h.coverArtService.GetCoverURL(ctx, item.ID, typeName)
	}

	c.JSON(http.StatusOK, result)
}

// GetEntityChildren handles GET /api/v1/entities/:id/children.
func (h *MediaEntityHandler) GetEntityChildren(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, total, err := h.itemRepo.GetByParent(ctx, id, limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get children", err)
		return
	}

	jsonItems := itemsToJSON(items)
	h.enrichItemsWithCoverURLs(ctx, items, jsonItems)

	c.JSON(http.StatusOK, gin.H{
		"items":  jsonItems,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetEntityFiles handles GET /api/v1/entities/:id/files.
func (h *MediaEntityHandler) GetEntityFiles(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	files, err := h.fileRepo.GetFilesByItem(ctx, id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get files", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"total": len(files),
	})
}

// GetEntityMetadata handles GET /api/v1/entities/:id/metadata.
func (h *MediaEntityHandler) GetEntityMetadata(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	metadata, err := h.extMetaRepo.GetByItem(ctx, id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get metadata", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metadata": metadata,
	})
}

// GetEntityDuplicates handles GET /api/v1/entities/:id/duplicates.
func (h *MediaEntityHandler) GetEntityDuplicates(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	item, err := h.itemRepo.GetByID(ctx, id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Entity not found", err)
		return
	}

	dups, err := h.itemRepo.GetDuplicates(ctx, item.Title, item.MediaTypeID, item.Year)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to find duplicates", err)
		return
	}

	// Exclude self
	var filtered []*models.MediaItem
	for _, d := range dups {
		if d.ID != id {
			filtered = append(filtered, d)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"duplicates": itemsToJSON(filtered),
		"total":      len(filtered),
	})
}

// GetEntityTypes handles GET /api/v1/entities/types — list media types with counts.
func (h *MediaEntityHandler) GetEntityTypes(c *gin.Context) {
	ctx := c.Request.Context()

	types, err := h.itemRepo.GetMediaTypes(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get media types", err)
		return
	}

	counts, _ := h.itemRepo.CountByType(ctx)

	result := []gin.H{}
	for _, mt := range types {
		count := int64(0)
		if c, ok := counts[mt.Name]; ok {
			count = c
		}
		result = append(result, gin.H{
			"id":          mt.ID,
			"name":        mt.Name,
			"description": mt.Description,
			"count":       count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"types": result,
	})
}

// BrowseByType handles GET /api/v1/entities/browse/:type.
func (h *MediaEntityHandler) BrowseByType(c *gin.Context) {
	ctx := c.Request.Context()

	typeName := c.Param("type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 || limit > 200 {
		limit = 24
	}
	if offset < 0 {
		offset = 0
	}

	_, typeID, err := h.itemRepo.GetMediaTypeByName(ctx, typeName)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	items, total, err := h.itemRepo.GetByType(ctx, typeID, limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to browse type", err)
		return
	}

	// Enrich items with cover URLs via the cover art service fallback chain
	jsonItems := itemsToJSON(items)
	h.enrichItemsWithCoverURLs(ctx, items, jsonItems)

	// Trigger lazy enrichment for items without metadata
	var unenrichedIDs []int64
	for _, item := range items {
		if item != nil {
			unenrichedIDs = append(unenrichedIDs, item.ID)
		}
	}
	h.lazyEnrichEntities(unenrichedIDs)

	c.JSON(http.StatusOK, gin.H{
		"items":  jsonItems,
		"total":  total,
		"type":   typeName,
		"limit":  limit,
		"offset": offset,
	})
}

// RefreshEntityMetadata handles POST /api/v1/entities/:id/metadata/refresh.
// Finds cover images in the entity's directory and stores as external metadata.
func (h *MediaEntityHandler) RefreshEntityMetadata(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	if h.db == nil {
		// CAT-001: without a database nothing can be refreshed; never answer 202 "queued".
		c.JSON(http.StatusNotImplemented, gin.H{"error": "metadata refresh is unavailable: no database is configured. Nothing was queued.", "entity_id": id})
		return
	}

	if !h.entityMustExist(c, id) {
		return
	}

	// Find the directory path for this entity via directory_analyses
	var dirPath string
	err = h.db.QueryRowContext(ctx,
		`SELECT da.directory_path FROM directory_analyses da
		 WHERE da.media_item_id = ? LIMIT 1`, id).Scan(&dirPath)
	if err != nil || dirPath == "" {
		// Fallback: try media_files junction
		err = h.db.QueryRowContext(ctx,
			`SELECT f.directory_path FROM files f
			 JOIN media_files mf ON mf.file_id = f.id
			 WHERE mf.media_item_id = ? LIMIT 1`, id).Scan(&dirPath)
	}
	if err != nil || dirPath == "" {
		c.JSON(http.StatusOK, gin.H{"message": "No directory found", "entity_id": id})
		return
	}

	parentDir := dirPath

	// Search for cover images: directory_path stores full path like "Dir/Sub/cover.jpg"
	// So we look for files where the path starts with the entity dir and ends with a cover name
	coverNames := []string{"cover.jpg", "folder.jpg", "poster.jpg", "cover.png", "folder.png", "poster.png"}
	var coverFileID int64
	var coverTitle string
	for _, name := range coverNames {
		err := h.db.QueryRowContext(ctx,
			`SELECT id, title FROM files
			 WHERE directory_path LIKE ? LIMIT 1`,
			parentDir+"/%"+name).Scan(&coverFileID, &coverTitle)
		if err == nil && coverFileID > 0 {
			break
		}
		coverFileID = 0
	}

	if coverFileID > 0 {
		coverURL := fmt.Sprintf("/api/v1/download/file/%d", coverFileID)
		meta := &models.ExternalMetadata{
			MediaItemID: id,
			Provider:    "local_scan",
			ExternalID:  fmt.Sprintf("local:%d", id),
			CoverURL:    &coverURL,
		}
		if err := h.extMetaRepo.Upsert(ctx, meta); err != nil {
			// Log but don't fail - the cover was still found
			logging.Warnf("Failed to upsert external metadata: %v", err)
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Cover art found (local)", "entity_id": id, "cover_url": coverURL,
		})
		return
	}

	// Fallback: try TMDB for movies/TV
	entity, err := h.itemRepo.GetByID(ctx, id)
	if err == nil && entity != nil {
		year := 0
		if entity.Year != nil {
			year = *entity.Year
		}
		tmdbResult := h.fetchTMDBMetadata(ctx, entity.Title, year, int(entity.MediaTypeID))
		if tmdbResult != nil {
			meta := &models.ExternalMetadata{
				MediaItemID: id,
				Provider:    "tmdb",
				ExternalID:  fmt.Sprintf("tmdb:%d", tmdbResult.tmdbID),
				CoverURL:    &tmdbResult.posterURL,
				Data:        tmdbResult.overview,
				Rating:      tmdbResult.rating,
			}
			if err := h.extMetaRepo.Upsert(ctx, meta); err != nil {
				// Log but don't fail - the metadata was still fetched
				logging.Warnf("Failed to upsert TMDB metadata: %v", err)
			}

			// Also update the entity description/rating if empty
			if (entity.Description == nil || *entity.Description == "") && tmdbResult.overview != "" {
				_, err := h.db.ExecContext(ctx,
					`UPDATE media_items SET description = ? WHERE id = ? AND (description IS NULL OR description = '')`,
					tmdbResult.overview, id)
				if err != nil {
					logging.Warnf("Failed to update entity description: %v", err)
				}
			}
			if (entity.Rating == nil || *entity.Rating == 0) && tmdbResult.rating != nil {
				_, err := h.db.ExecContext(ctx,
					`UPDATE media_items SET rating = ? WHERE id = ? AND (rating IS NULL OR rating = 0)`,
					*tmdbResult.rating, id)
				if err != nil {
					logging.Warnf("Failed to update entity rating: %v", err)
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Metadata enriched from TMDB", "entity_id": id,
				"cover_url": tmdbResult.posterURL, "title": tmdbResult.title,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "No metadata sources available", "entity_id": id, "directory": parentDir,
	})
}

// tmdbSearchResult holds parsed TMDB API response data.
type tmdbSearchResult struct {
	tmdbID    int
	title     string
	overview  string
	posterURL string
	rating    *float64
	year      string
}

// buildProxyHTTPClient returns an HTTP client that routes through the
// configured proxy (SOCKS5 or HTTP) when enabled.
func (h *MediaEntityHandler) buildProxyHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{}
	if h.proxyCfg != nil && h.proxyCfg.IsEnabled() {
		if h.proxyCfg.GetURL() != "" {
			parsedProxy, err := url.Parse(h.proxyCfg.GetURL())
			if err == nil && parsedProxy.Scheme == "socks5" {
				var auth *proxy.Auth
				if h.proxyCfg.GetUsername() != "" || h.proxyCfg.GetPassword() != "" {
					auth = &proxy.Auth{User: h.proxyCfg.GetUsername(), Password: h.proxyCfg.GetPassword()}
				}
				SOCKS5Dialer, err := proxy.SOCKS5("tcp", parsedProxy.Host, auth, proxy.Direct)
				if err == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return SOCKS5Dialer.Dial(network, addr)
					}
					return &http.Client{Timeout: timeout, Transport: transport}
				}
			}
		}
		if h.proxyCfg.GetHTTPURL() != "" {
			parsedHTTPProxy, err := url.Parse(h.proxyCfg.GetHTTPURL())
			if err == nil {
				transport.Proxy = http.ProxyURL(parsedHTTPProxy)
				return &http.Client{Timeout: timeout, Transport: transport}
			}
		}
	}
	return &http.Client{Timeout: timeout}
}

// Title-cleaning patterns, compiled once at package load for efficiency.
// They turn a noisy catalog filename into a TMDB-search-friendly query.
var (
	// leadingSeqNumRe strips a leading "NNN <sep>" catalog sequence prefix,
	// e.g. "001 - ", "32 - ", "003 - ", "01. ", "07) ". A separator char is
	// REQUIRED so real titles that legitimately start with a number
	// ("21 Jump Street", "1984", "300") are left untouched.
	leadingSeqNumRe = regexp.MustCompile(`^\s*\d{1,4}\s*[-._)\]]\s*`)
	// balancedParensRe / balancedBracketsRe remove release/scene tags fully
	// enclosed in () or [], e.g. "(2003)", "(Digital-Empire)", "[x264]",
	// "[Canada, DCC 24K Gold GZS-1045]".
	balancedParensRe   = regexp.MustCompile(`\([^)]*\)`)
	balancedBracketsRe = regexp.MustCompile(`\[[^\]]*\]`)
	// danglingParenRe / danglingBracketRe strip an unbalanced trailing tag with
	// no closing char, e.g. "...(WebP by Doc MaKS" or "...(Minutemen-Faessla".
	danglingParenRe   = regexp.MustCompile(`\([^)]*$`)
	danglingBracketRe = regexp.MustCompile(`\[[^\]]*$`)
	// dashSeparatorRe collapses " - " word separators to a single space —
	// TMDB matches "Captain America The First Avenger" better than the
	// "Captain America - The First Avenger" filename form. A hyphen WITHOUT
	// surrounding spaces ("Spider-Man") is deliberately not matched.
	dashSeparatorRe = regexp.MustCompile(`\s+-\s+`)
	// multiSpaceRe collapses any run of whitespace left behind into one space.
	multiSpaceRe = regexp.MustCompile(`\s+`)
)

// cleanTitleForSearch normalizes a catalog media title into a TMDB-search-
// friendly query. It strips the catalog's "NNN - " sequence-number prefixes,
// release/scene tags in () and [] (including an unbalanced dangling tag), and
// collapses " - " separators that TMDB matches worse than plain spaces.
//
// It is conservative by design — it never strips a leading number that is part
// of a real title (a separator char is required after the sequence number) and
// it falls back to the raw title whenever cleaning would yield an empty query,
// so the caller never searches TMDB for "".
//
// The cleaned value is used ONLY to build the search query; the stored title and
// metadata still come from the TMDB response or the original entity. The dropped
// in-title year (e.g. "(2003)") is intentional — year is passed as a separate
// search parameter and must not be lost from it.
func cleanTitleForSearch(raw string) string {
	cleaned := leadingSeqNumRe.ReplaceAllString(raw, "")
	cleaned = balancedParensRe.ReplaceAllString(cleaned, " ")
	cleaned = balancedBracketsRe.ReplaceAllString(cleaned, " ")
	cleaned = danglingParenRe.ReplaceAllString(cleaned, " ")
	cleaned = danglingBracketRe.ReplaceAllString(cleaned, " ")
	cleaned = dashSeparatorRe.ReplaceAllString(cleaned, " ")
	cleaned = multiSpaceRe.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return raw
	}
	return cleaned
}

// fetchTMDBMetadata searches TMDB for a movie/TV show and returns poster + metadata.
// Retries without year if the initial year-qualified search returns no results,
// matching the aggregation_service.go enrichment path. Falls back to LLM on failure.
func (h *MediaEntityHandler) fetchTMDBMetadata(ctx context.Context, title string, year int, mediaTypeID int) *tmdbSearchResult {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		return h.tryLLMFallback(ctx, title, year, mediaTypeID)
	}

	if isTMDBOnCooldown() {
		logging.Warnf("TMDB: on cooldown due to previous failures; skipping '%s'", title)
		return h.tryLLMFallback(ctx, title, year, mediaTypeID)
	}

	// Determine search type
	searchType := "movie"
	if mediaTypeID == 2 { // tv_show
		searchType = "tv"
	}

	// Validate year range — skip obviously wrong years (before first film or future)
	yearParam := year
	currentYear := time.Now().Year()
	if yearParam > currentYear || yearParam < 1888 {
		yearParam = 0
	}

	// Clean catalog filename noise (sequence prefixes, release/scene tags,
	// " - " separators) out of the SEARCH QUERY only — stored title/metadata
	// still come from the TMDB response or the original entity. Done once and
	// reused for both the with-year and without-year (retry) queries below.
	searchTitle := cleanTitleForSearch(title)

	// Build base search URL with year
	baseURL := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s",
		searchType, apiKey, url.QueryEscape(searchTitle))
	searchURL := baseURL
	if yearParam > 0 {
		if searchType == "movie" {
			searchURL += fmt.Sprintf("&year=%d", yearParam)
		} else {
			searchURL += fmt.Sprintf("&first_air_date_year=%d", yearParam)
		}
	}

	client := h.buildProxyHTTPClient(10 * time.Second)

	var searchResp struct {
		Results []struct {
			ID           int     `json:"id"`
			Title        string  `json:"title"`
			Name         string  `json:"name"`
			Overview     string  `json:"overview"`
			PosterPath   string  `json:"poster_path"`
			VoteAverage  float64 `json:"vote_average"`
			ReleaseDate  string  `json:"release_date"`
			FirstAirDate string  `json:"first_air_date"`
		} `json:"results"`
	}

	tmdbFailed := false

	// Try with year first, retry without year if no results
	for attempt := 0; attempt < 2; attempt++ {
		reqURL := searchURL
		if attempt == 1 {
			if yearParam == 0 {
				break // No year was used in first attempt, retry won't help
			}
			reqURL = baseURL // Retry without year
			logging.Warnf("TMDB: retrying '%s' without year (year=%d returned no results)", title, yearParam)
		}

		if err := services.GuardProviderURL(reqURL, services.SSRFGuardConfig{}); err != nil {
			logging.Warnf("TMDB: unsafe URL for '%s': %v", title, err)
			tmdbFailed = true
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil) // #nosec G704 — URL validated by GuardProviderURL above
		if err != nil {
			logging.Warnf("TMDB: request build failed for '%s': %v", title, err)
			tmdbFailed = true
			break
		}

		resp, err := client.Do(req) // #nosec G704 — URL validated by GuardProviderURL above
		if err != nil {
			logging.Warnf("TMDB: request failed for '%s': %v", title, err)
			tmdbFailed = true
			break
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			logging.Warnf("TMDB: HTTP %d for '%s'", resp.StatusCode, title)
			tmdbFailed = true
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			tmdbFailed = true
			break
		}

		if err := json.Unmarshal(body, &searchResp); err != nil {
			logging.Warnf("TMDB: JSON parse error for '%s': %v", title, err)
			tmdbFailed = true
			break
		}
		if len(searchResp.Results) > 0 {
			recordTMDBSuccess()
			break // Found results
		}
		// No results with year — retry without
		time.Sleep(300 * time.Millisecond)
	}

	if tmdbFailed {
		recordTMDBFailure()
		return h.tryLLMFallback(ctx, title, year, mediaTypeID)
	}

	if len(searchResp.Results) == 0 {
		logging.Warnf("TMDB: no results for '%s' (year=%d, type=%s)", title, year, searchType)
		return h.tryLLMFallback(ctx, title, year, mediaTypeID)
	}

	best := searchResp.Results[0]
	resultTitle := best.Title
	if resultTitle == "" {
		resultTitle = best.Name
	}

	var posterURL string
	if best.PosterPath != "" {
		posterURL = "/api/v1/image-proxy?url=https://image.tmdb.org/t/p/w500" + best.PosterPath
	}

	rating := best.VoteAverage
	return &tmdbSearchResult{
		tmdbID:    best.ID,
		title:     resultTitle,
		overview:  best.Overview,
		posterURL: posterURL,
		rating:    &rating,
		year:      best.ReleaseDate,
	}
}

// tryLLMFallback attempts to generate metadata via LLM when TMDB is unavailable.
func (h *MediaEntityHandler) tryLLMFallback(ctx context.Context, title string, year int, mediaTypeID int) *tmdbSearchResult {
	if h.llmProvider == nil || !h.llmProvider.IsEnabled() {
		return nil
	}
	mediaType := "movie"
	if mediaTypeID == 2 {
		mediaType = "tv"
	}
	yearPtr := &year
	if year == 0 {
		yearPtr = nil
	}
	results, err := h.llmProvider.Search(ctx, title, mediaType, yearPtr)
	if err != nil || len(results) == 0 {
		logging.Warnf("LLM fallback failed for '%s': %v", title, err)
		return nil
	}
	best := results[0]
	var posterURL string
	if best.CoverURL != nil && *best.CoverURL != "" {
		posterURL = "/api/v1/image-proxy?url=" + url.QueryEscape(*best.CoverURL)
	}
	return &tmdbSearchResult{
		tmdbID:    0,
		title:     best.Title,
		overview:  "",
		posterURL: posterURL,
		rating:    best.Rating,
		year:      "",
	}
}

// entityDir pairs a media item id with its best-known directory path.
// DirPath is the MIN(directory_analyses.directory_path) for the item, or
// "" when the item has no directory_analyses row (the LEFT JOIN miss).
type entityDir struct {
	ID      int64
	DirPath string
}

// enrichmentAttemptedProvider is the sentinel external_metadata provider written
// for an item that was processed for enrichment but yielded no cover/metadata
// (TMDB returned no result AND the LLM fallback was empty). It is a pure
// progress marker — it makes em.id non-NULL for the item so
// selectEntitiesNeedingEnrichment (which selects `WHERE em.id IS NULL`) stops
// returning the SAME leading unmatchable items on every batch and the queue
// advances to the real movies deeper in the catalog. A future "force
// re-enrich" can retry these by filtering `WHERE em.provider != 'enrichment_attempted'`.
const enrichmentAttemptedProvider = "enrichment_attempted"

// markEnrichmentAttempted writes the sentinel external_metadata row for an item
// whose enrichment found nothing. The row carries a NULL cover_url, so the
// cover lookup in cover_art_service.GetCoverURL (which filters `cover_url IS NOT
// NULL AND cover_url != ”`) never mistakes it for a real cover and still falls
// through to the type placeholder. The successful enrichment paths (local_scan,
// tmdb) are unaffected — they keep writing their own real rows.
func (h *MediaEntityHandler) markEnrichmentAttempted(ctx context.Context, itemID int64) error {
	return h.extMetaRepo.Upsert(ctx, &models.ExternalMetadata{
		MediaItemID: itemID,
		Provider:    enrichmentAttemptedProvider,
		ExternalID:  fmt.Sprintf("attempted:%d", itemID),
		CoverURL:    nil,
	})
}

// selectEntitiesNeedingEnrichment returns up to `limit` media items that do
// not yet have an external_metadata row. directory_analyses is LEFT JOINed
// so items that lack a directory_analyses row are STILL reachable (DirPath
// is "" for those); callers MUST skip the local-cover-file lookup when
// DirPath is empty and fall through to the TMDB/LLM path, which only needs
// the item's title/year. An inner JOIN here silently excluded every item
// without a directory_analyses row from enrichment.
//
// Ordering: TMDB-matchable types (media_type_id IN (1,2) — movie, tv_show)
// are returned FIRST, then everything else. Only ~1369 of the catalog's 27750
// items are movie/tv_show; the rest are music albums, comics, episodes, seasons
// and software that TMDB (a movie/TV database) can never match. Without this
// ORDER BY, enrichment processed items in arbitrary id order and burned
// thousands of batches marking unmatchable items `enrichment_attempted` before
// reaching the movies — so movie cover art loaded last.
//
// WITHIN each tier, items are ordered by first_detected DESCENDING (newest
// first), with mi.id as the final deterministic tiebreak. The visible
// "Recently Added Movies" shelf surfaces the highest-first_detected movies, so
// recency-first ordering populates that shelf quickly; the previous tiebreak
// (mi.id ascending) covered the OLDEST-detected movies first and left the
// visible newest-movie shelf for last (operator symptom). The CASE expression
// + GROUP BY (mi.id, mi.media_type_id, mi.first_detected) are portable across
// SQLite (dev) and PostgreSQL (prod); media_type_id and first_detected are both
// functionally dependent on the primary key mi.id, so adding them to GROUP BY
// does not change the result cardinality.
func (h *MediaEntityHandler) selectEntitiesNeedingEnrichment(ctx context.Context, limit int) ([]entityDir, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT mi.id, COALESCE(MIN(da.directory_path), '')
		 FROM media_items mi
		 LEFT JOIN directory_analyses da ON da.media_item_id = mi.id
		 LEFT JOIN external_metadata em ON em.media_item_id = mi.id
		 WHERE em.id IS NULL
		 GROUP BY mi.id, mi.media_type_id, mi.first_detected
		 ORDER BY (CASE WHEN mi.media_type_id IN (1, 2) THEN 0 ELSE 1 END), mi.first_detected DESC, mi.id
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []entityDir
	for rows.Next() {
		var ed entityDir
		if err := rows.Scan(&ed.ID, &ed.DirPath); err != nil {
			continue
		}
		entities = append(entities, ed)
	}
	return entities, rows.Err()
}

// EnrichAllEntities handles POST /api/v1/entities/enrich — batch metadata enrichment.
// Returns 202 Accepted immediately and processes enrichment asynchronously in the
// background. The caller can poll GET /api/v1/entities/stats to observe progress.
// Query params:
//   - limit: max entities to process (default 50, max 200)
func (h *MediaEntityHandler) EnrichAllEntities(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	// Parse optional limit (default 50, max 200) to keep response time reasonable
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// Fast existence check so we can return 200 when there is no work.
	var pending int
	_ = h.db.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM media_items mi
		 LEFT JOIN external_metadata em ON em.media_item_id = mi.id
		 WHERE em.id IS NULL
		 LIMIT 1`).Scan(&pending)
	if pending == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":  "No entities need enrichment",
			"queued":   0,
			"accepted": false,
		})
		return
	}

	// Return 200 OK immediately; enrichment is performed in the background.
	c.JSON(http.StatusOK, gin.H{
		"message":       "Batch enrichment queued",
		"queued":        -1,
		"accepted":      true,
		"limit_applied": limit,
	})

	// Spawn background enrichment goroutine
	h.enrichWg.Add(1)
	go func(limit int) {
		defer h.enrichWg.Done()
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Get all entities that don't have external metadata yet. Uses a
		// LEFT JOIN on directory_analyses so items without a
		// directory_analyses row are still reachable (DirPath == "").
		entities, err := h.selectEntitiesNeedingEnrichment(bgCtx, limit)
		if err != nil {
			logging.Warnf("EnrichAllEntities: background query failed: %v", err)
			return
		}

		if len(entities) == 0 {
			logging.Infof("EnrichAllEntities: no entities need enrichment after fast check indicated work")
			return
		}

		enriched := 0
		tmdbEnriched := 0
		localEnriched := 0
		coverNames := []string{"cover.jpg", "folder.jpg", "poster.jpg", "cover.png"}

		for _, ent := range entities {
			// Check context cancellation before each entity
			select {
			case <-bgCtx.Done():
				logging.Warnf("EnrichAllEntities: background job cancelled after %d entities", enriched)
				return
			default:
			}

			// Try local cover first — only when we know the entity's
			// directory. Items without a directory_analyses row have an
			// empty DirPath (LEFT JOIN miss); skipping the lookup avoids a
			// degenerate `LIKE '/%cover.jpg'` pattern that would match
			// unrelated files, and falls straight through to TMDB/LLM.
			parentDir := ent.DirPath
			found := false
			if parentDir != "" {
				for _, name := range coverNames {
					var coverFileID int64
					err := h.db.QueryRowContext(bgCtx,
						`SELECT id FROM files WHERE directory_path LIKE ? LIMIT 1`,
						parentDir+"/%"+name).Scan(&coverFileID)
					if err == nil && coverFileID > 0 {
						coverURL := fmt.Sprintf("/api/v1/download/file/%d", coverFileID)
						meta := &models.ExternalMetadata{
							MediaItemID: ent.ID,
							Provider:    "local_scan",
							ExternalID:  fmt.Sprintf("local:%d", ent.ID),
							CoverURL:    &coverURL,
						}
						if err := h.extMetaRepo.Upsert(bgCtx, meta); err != nil {
							// Log but continue - cover was still found
						}
						localEnriched++
						enriched++
						found = true
						break
					}
				}
			}

			// Try TMDB if no local cover
			if !found {
				var title string
				var year int
				var mediaTypeID int
				err := h.db.QueryRowContext(bgCtx,
					`SELECT title, COALESCE(year, 0), media_type_id FROM media_items WHERE id = ?`,
					ent.ID).Scan(&title, &year, &mediaTypeID)
				if err == nil && title != "" {
					result := h.fetchTMDBMetadata(bgCtx, title, year, mediaTypeID)
					if result != nil && result.posterURL != "" {
						meta := &models.ExternalMetadata{
							MediaItemID: ent.ID,
							Provider:    "tmdb",
							ExternalID:  fmt.Sprintf("tmdb:%d", result.tmdbID),
							CoverURL:    &result.posterURL,
							Data:        result.overview,
							Rating:      result.rating,
						}
						if err := h.extMetaRepo.Upsert(bgCtx, meta); err != nil {
							// Log but continue - metadata was still fetched
						}
						// Update entity description
						if result.overview != "" {
							_, _ = h.db.ExecContext(bgCtx,
								`UPDATE media_items SET description = ? WHERE id = ? AND (description IS NULL OR description = '')`,
								result.overview, ent.ID)
						}
						if result.rating != nil && *result.rating > 0 {
							_, _ = h.db.ExecContext(bgCtx,
								`UPDATE media_items SET rating = ? WHERE id = ? AND (rating IS NULL OR rating = 0)`,
								*result.rating, ent.ID)
						}
						tmdbEnriched++
						enriched++
						found = true
						// Rate limit TMDB API (40 requests per 10 seconds)
						time.Sleep(250 * time.Millisecond)
					}
				}
			}

			// Progress marker: the item was processed but neither a local
			// cover nor TMDB/LLM produced any metadata. Write a sentinel
			// external_metadata row so em.id is no longer NULL for it —
			// without this the `WHERE em.id IS NULL` query returns the SAME
			// leading unmatchable items on every batch and the queue never
			// reaches the real movies deeper in the catalog.
			if !found {
				if err := h.markEnrichmentAttempted(bgCtx, ent.ID); err != nil {
					logging.Warnf("EnrichAllEntities: failed to mark item %d enrichment-attempted: %v", ent.ID, err)
				}
			}
		}

		logging.Infof("EnrichAllEntities: completed — %d enriched (%d local, %d TMDB) out of %d queued",
			enriched, localEnriched, tmdbEnriched, len(entities))
	}(limit)
}

// entityMustExist answers 404 (and returns false) when no entity has the id, so a write aimed at an unknown entity is
// reported as such instead of surfacing a foreign-key failure as 500. It returns true when the repository is not wired
// (those handlers are exercised without one) or the entity exists; a lookup failure other than "not found" is a 500.
func (h *MediaEntityHandler) entityMustExist(c *gin.Context, id int64) bool {
	if h.itemRepo == nil {
		return true
	}
	if _, err := h.itemRepo.GetByID(c.Request.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			utils.SendErrorResponse(c, http.StatusNotFound, "Entity not found", err)
		} else {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to look up entity", err)
		}
		return false
	}
	return true
}

// UpdateUserMetadata handles PUT /api/v1/entities/:id/user-metadata.
func (h *MediaEntityHandler) UpdateUserMetadata(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	var req struct {
		UserRating    *float64 `json:"user_rating"`
		WatchedStatus *string  `json:"watched_status"`
		Favorite      *bool    `json:"favorite"`
		IsFavorite    *bool    `json:"is_favorite"`
		PersonalNotes *string  `json:"personal_notes"`
		Tags          []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// The authenticated user (JWT middleware sets int, int64 or a string subject). Never default to another account: an
	// unauthenticated request is refused instead of being written under user 1 (CAT-001).
	userID, ok := userIDFromContext(c)
	if !ok {
		utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	um := &models.UserMetadata{
		MediaItemID:   id,
		UserID:        userID,
		UserRating:    req.UserRating,
		WatchedStatus: req.WatchedStatus,
		PersonalNotes: req.PersonalNotes,
		Tags:          req.Tags,
	}
	// Determine favorite value from either field
	if req.IsFavorite != nil {
		um.Favorite = *req.IsFavorite
	} else if req.Favorite != nil {
		um.Favorite = *req.Favorite
	}

	if !h.entityMustExist(c, id) {
		return
	}
	if err := h.userMetaRepo.Upsert(ctx, um); err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to update user metadata", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "User metadata updated",
		"is_favorite": um.Favorite,
	})
}

// GetEntityStats handles GET /api/v1/entities/stats.
func (h *MediaEntityHandler) GetEntityStats(c *gin.Context) {
	ctx := c.Request.Context()

	totalCount, _ := h.itemRepo.Count(ctx)
	countByType, _ := h.itemRepo.CountByType(ctx)

	c.JSON(http.StatusOK, gin.H{
		"total_entities": totalCount,
		"by_type":        countByType,
	})
}

// StreamEntity handles GET /api/v1/entities/:id/stream — returns streaming info for primary file.
func (h *MediaEntityHandler) StreamEntity(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	// GetPrimaryStreamableFile JOINs media_files against files,
	// filters out metadata scratch files (.DS_Store, Thumbs.db,
	// ._*, desktop.ini, .nfo, .srt, ...) and returns the largest
	// file with a known playable extension. Without this, the
	// earlier implementation handed libVLC a 6 KB .DS_Store and
	// crashed the Android TV client inside
	// Media.nativeNewFromLocation.
	primary, err := h.fileRepo.GetPrimaryStreamableFile(ctx, id)
	if err != nil {
		if err == repository.ErrNoStreamableFile {
			utils.SendErrorResponse(c, http.StatusNotFound, "No streamable file available", err)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to resolve streamable file", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entity_id":  id,
		"file_id":    primary.FileID,
		"stream_url": fmt.Sprintf("/api/v1/stream/%d", primary.FileID),
		"filename":   primary.Filename,
		"size":       primary.Size,
		"mime_type":  primary.MimeType,
	})
}

// DownloadEntity handles GET /api/v1/entities/:id/download — returns download info.
func (h *MediaEntityHandler) DownloadEntity(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	fileIDStr := c.Query("file_id")

	files, err := h.fileRepo.GetFilesByItem(ctx, id)
	if err != nil || len(files) == 0 {
		utils.SendErrorResponse(c, http.StatusNotFound, "No files available for download", err)
		return
	}

	// Explicit file_id always wins — the client has asked for a
	// specific file (subtitle, trailer, alternate cut).
	var targetFileID int64 = -1
	if fileIDStr != "" {
		if parsed, err := strconv.ParseInt(fileIDStr, 10, 64); err == nil {
			for _, f := range files {
				if f.FileID == parsed {
					targetFileID = f.FileID
					break
				}
			}
			if targetFileID < 0 {
				utils.SendErrorResponse(c, http.StatusNotFound, "Requested file_id not linked to this entity", nil)
				return
			}
		}
	}

	// No explicit file_id: use the same smart primary-picker as
	// StreamEntity so `Download` and `Play Now` both hand back
	// the real movie instead of a 6 KB .DS_Store.
	if targetFileID < 0 {
		primary, err := h.fileRepo.GetPrimaryStreamableFile(ctx, id)
		if err != nil {
			if err == repository.ErrNoStreamableFile {
				utils.SendErrorResponse(c, http.StatusNotFound, "No downloadable file available", err)
				return
			}
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to resolve downloadable file", err)
			return
		}
		targetFileID = primary.FileID
	}

	c.JSON(http.StatusOK, gin.H{
		"entity_id":    id,
		"file_id":      targetFileID,
		"download_url": fmt.Sprintf("/api/v1/download/file/%d", targetFileID),
		"total_files":  len(files),
	})
}

// GetInstallInfo handles GET /api/v1/entities/:id/install-info — software installation details.
func (h *MediaEntityHandler) GetInstallInfo(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return
	}

	item, err := h.itemRepo.GetByID(ctx, id)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Entity not found", err)
		return
	}

	// Verify this is a software entity
	types, _ := h.itemRepo.GetMediaTypes(ctx)
	typeName := ""
	for _, mt := range types {
		if mt.ID == item.MediaTypeID {
			typeName = mt.Name
			break
		}
	}
	if typeName != "software" {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Install info is only available for software entities", nil)
		return
	}

	files, err := h.fileRepo.GetFilesByItem(ctx, id)
	if err != nil || len(files) == 0 {
		utils.SendErrorResponse(c, http.StatusNotFound, "No files available", err)
		return
	}

	primary := files[0]
	for _, f := range files {
		if f.IsPrimary {
			primary = f
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"entity_id":    id,
		"title":        item.Title,
		"file_id":      primary.FileID,
		"download_url": fmt.Sprintf("/api/v1/entities/%d/download", id),
		"total_files":  len(files),
	})
}

// ListDuplicateGroups handles GET /api/v1/entities/duplicates — global duplicate listing.
func (h *MediaEntityHandler) ListDuplicateGroups(c *gin.Context) {
	ctx := c.Request.Context()

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	groups, total, err := h.itemRepo.ListDuplicateGroups(ctx, limit, offset)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to list duplicates", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// --- JSON helpers ---

func itemsToJSON(items []*models.MediaItem) []gin.H {
	if items == nil {
		return []gin.H{}
	}
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		result = append(result, itemToJSON(item))
	}
	return result
}

func itemToJSON(item *models.MediaItem) gin.H {
	h := gin.H{
		"id":             item.ID,
		"media_type_id":  item.MediaTypeID,
		"title":          item.Title,
		"status":         item.Status,
		"first_detected": item.FirstDetected,
		"last_updated":   item.LastUpdated,
	}
	if item.OriginalTitle != nil {
		h["original_title"] = *item.OriginalTitle
	}
	if item.Year != nil {
		h["year"] = *item.Year
	}
	if item.Description != nil {
		h["description"] = *item.Description
	}
	if len(item.Genre) > 0 {
		h["genre"] = item.Genre
	}
	if item.Director != nil {
		h["director"] = *item.Director
	}
	if item.Rating != nil {
		h["rating"] = *item.Rating
	}
	if item.Runtime != nil {
		h["runtime"] = *item.Runtime
	}
	if item.Language != nil {
		h["language"] = *item.Language
	}
	if item.ParentID != nil {
		h["parent_id"] = *item.ParentID
	}
	if item.SeasonNumber != nil {
		h["season_number"] = *item.SeasonNumber
	}
	if item.EpisodeNumber != nil {
		h["episode_number"] = *item.EpisodeNumber
	}
	if item.TrackNumber != nil {
		h["track_number"] = *item.TrackNumber
	}
	return h
}

func entityDetailJSON(item *models.MediaItem, typeName string, fileCount, childrenCount int64, extMeta []*models.ExternalMetadata) gin.H {
	h := itemToJSON(item)
	h["media_type"] = typeName
	h["file_count"] = fileCount
	h["children_count"] = childrenCount
	if extMeta != nil {
		h["external_metadata"] = extMeta
	} else {
		h["external_metadata"] = []interface{}{}
	}
	return h
}

func strPtr(s string) *string {
	return &s
}

// enrichItemsWithCoverURLs adds a cover_url field to each item in jsonItems.
// Uses the CoverArtService batch method for efficiency, falling back to
// per-item DB queries or placeholder URLs.
func (h *MediaEntityHandler) enrichItemsWithCoverURLs(ctx context.Context, items []*models.MediaItem, jsonItems []gin.H) {
	if h.coverArtService == nil || len(items) == 0 {
		// No cover art service — use inline external_metadata fallback (legacy path)
		if h.db != nil {
			for i, itemMap := range jsonItems {
				if i >= len(items) || items[i] == nil {
					continue
				}
				id := items[i].ID
				var coverURL *string
				_ = h.db.QueryRowContext(ctx,
					`SELECT cover_url FROM external_metadata WHERE media_item_id = ? AND cover_url IS NOT NULL LIMIT 1`,
					id).Scan(&coverURL)
				if coverURL != nil && *coverURL != "" {
					itemMap["cover_url"] = *coverURL
				}
			}
		}
		return
	}

	// Build media type name lookup
	typeNames := h.getMediaTypeNames(ctx)

	// Build batch request
	reqs := make([]services.CoverURLRequest, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		typeName := typeNames[item.MediaTypeID]
		if typeName == "" {
			typeName = "movie"
		}
		reqs = append(reqs, services.CoverURLRequest{
			ID:            item.ID,
			MediaTypeName: typeName,
		})
	}

	coverURLs := h.coverArtService.GetCoverURLsBatch(ctx, reqs)

	// Apply cover URLs to JSON items
	for i, itemMap := range jsonItems {
		if i >= len(items) || items[i] == nil {
			continue
		}
		if url, ok := coverURLs[items[i].ID]; ok {
			itemMap["cover_url"] = url
		}
	}
}

// getMediaTypeNames returns a map of media type ID to name.
func (h *MediaEntityHandler) getMediaTypeNames(ctx context.Context) map[int64]string {
	types, err := h.itemRepo.GetMediaTypes(ctx)
	if err != nil {
		return map[int64]string{}
	}
	result := make(map[int64]string, len(types))
	for _, mt := range types {
		result[mt.ID] = mt.Name
	}
	return result
}

// Close waits for all background enrichment goroutines to finish.
// Must be called during shutdown to avoid goroutine leaks.
func (h *MediaEntityHandler) Close() {
	h.enrichWg.Wait()
}

// lazyEnrichEntities triggers background TMDB enrichment for entities without metadata.
// Called from browse/detail handlers to populate data on first access.
func (h *MediaEntityHandler) lazyEnrichEntities(entityIDs []int64) {
	if h.db == nil || os.Getenv("TMDB_API_KEY") == "" {
		return
	}
	h.enrichWg.Add(1)
	go func() {
		defer h.enrichWg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		for _, id := range entityIDs {
			// Check if already has metadata
			var count int
			_ = h.db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM external_metadata WHERE media_item_id = ?`, id).Scan(&count)
			if count > 0 {
				continue // Already enriched
			}
			// Check cache staleness (re-enrich if older than 30 days)
			var title string
			var year int
			var mediaTypeID int
			err := h.db.QueryRowContext(ctx,
				`SELECT title, COALESCE(year, 0), media_type_id FROM media_items WHERE id = ?`,
				id).Scan(&title, &year, &mediaTypeID)
			if err != nil || title == "" {
				continue
			}
			result := h.fetchTMDBMetadata(ctx, title, year, mediaTypeID)
			if result != nil && result.posterURL != "" {
				meta := &models.ExternalMetadata{
					MediaItemID: id,
					Provider:    "tmdb",
					ExternalID:  fmt.Sprintf("tmdb:%d", result.tmdbID),
					CoverURL:    &result.posterURL,
					Data:        result.overview,
					Rating:      result.rating,
				}
				if err := h.extMetaRepo.Upsert(ctx, meta); err != nil {
					// Log but continue
				}
				if result.overview != "" {
					if _, execErr := h.db.ExecContext(ctx,
						`UPDATE media_items SET description = ? WHERE id = ? AND (description IS NULL OR description = '')`,
						result.overview, id); execErr != nil {
						logging.Warnf("Failed to update media description from TMDB (id=%d): %v", id, execErr)
					}
				}
				if result.rating != nil && *result.rating > 0 {
					if _, execErr := h.db.ExecContext(ctx,
						`UPDATE media_items SET rating = ? WHERE id = ? AND (rating IS NULL OR rating = 0)`,
						*result.rating, id); execErr != nil {
						logging.Warnf("Failed to update media rating from TMDB (id=%d): %v", id, execErr)
					}
				}
				time.Sleep(250 * time.Millisecond) // TMDB rate limit
			}
		}
	}()
}
