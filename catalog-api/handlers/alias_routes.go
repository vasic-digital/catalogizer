package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// aliasDB is the slice of *database.DB the alias routes need.
type aliasDB interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// AliasHandler serves the "challenge-compatible alias" routes. CAT-001
// (§11.4.289): every value it returns is derived from real state (database
// rows). Where no real source exists the route answers 501, never a
// hard-coded payload.
type AliasHandler struct {
	db     aliasDB
	browse *BrowseHandler
	// syncGate guards the /sync/* alias GETs (SYNC-ADMIN-1: admin-only). Nil leaves them open
	// (unit tests that mount the handler without the production middleware chain).
	syncGate gin.HandlerFunc
}

// WithSyncGate installs the admin gate applied to the /sync/* alias routes.
func (h *AliasHandler) WithSyncGate(g gin.HandlerFunc) *AliasHandler {
	h.syncGate = g
	return h
}

// syncHandlers prepends the sync gate (when set) to a route's handler chain.
func (h *AliasHandler) syncHandlers(hs ...gin.HandlerFunc) []gin.HandlerFunc {
	if h.syncGate == nil {
		return hs
	}
	return append([]gin.HandlerFunc{h.syncGate}, hs...)
}

// NewAliasHandler builds the handler. browse may be nil (then /files is 501).
func NewAliasHandler(db aliasDB, browse *BrowseHandler) *AliasHandler {
	return &AliasHandler{db: db, browse: browse}
}

// RegisterAliasRoutes mounts the alias routes on the authenticated API group.
func (h *AliasHandler) RegisterAliasRoutes(api *gin.RouterGroup) {
	api.GET("/media-types", h.MediaTypes)
	api.GET("/stats/media-types", h.StatsMediaTypes)
	api.GET("/stats/scan-history", h.ScanHistory)
	api.GET("/sync/status", h.syncHandlers(h.SyncStatus)...)
	api.GET("/sync/providers", h.syncHandlers(h.SyncProviders)...)
	api.GET("/sync/devices", h.syncHandlers(h.notImplemented("no device registry exists"))...)
	api.GET("/sync/conflicts", h.syncHandlers(h.notImplemented("no sync conflict store exists"))...)
	api.GET("/files", h.Files)
	api.GET("/subtitles", h.Subtitles)
	api.GET("/translations", h.notImplemented("no translation catalog exists"))
	api.GET("/localization/translations", h.notImplemented("no localization catalog exists"))
	api.GET("/localization/stats", h.notImplemented("no localization catalog exists"))
	api.GET("/recommendations/by-type", h.notImplemented("no recommendation engine is wired"))
}

func (h *AliasHandler) notImplemented(reason string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented: " + reason})
	}
}

func (h *AliasHandler) fail(c *gin.Context, err error) {
	log.Printf("alias route %s %s: database error: %v", c.Request.Method, c.FullPath(), err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "database query failed"})
}

// MediaTypes lists the rows of the media_types table.
func (h *AliasHandler) MediaTypes(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(), `SELECT id, name FROM media_types ORDER BY id`)
	if err != nil {
		h.fail(c, err)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			h.fail(c, err)
			return
		}
		out = append(out, gin.H{"id": id, "name": name})
	}
	if err := rows.Err(); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"media_types": out})
}

// StatsMediaTypes counts media_items per media type.
func (h *AliasHandler) StatsMediaTypes(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(),
		`SELECT mt.name, COUNT(mi.id) FROM media_types mt LEFT JOIN media_items mi ON mi.media_type_id = mt.id GROUP BY mt.id, mt.name ORDER BY mt.id`)
	if err != nil {
		h.fail(c, err)
		return
	}
	defer rows.Close()
	dist := gin.H{}
	total := 0
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			h.fail(c, err)
			return
		}
		dist[name] = n
		total += n
	}
	if err := rows.Err(); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"distribution": dist, "total": total})
}

// ScanHistory returns the most recent scan_history rows.
func (h *AliasHandler) ScanHistory(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(),
		`SELECT id, storage_root_id, scan_type, status, start_time, end_time, files_processed, error_count FROM scan_history ORDER BY id DESC LIMIT 100`)
	if err != nil {
		h.fail(c, err)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int
		var root, processed, errs sql.NullInt64
		var typ, status sql.NullString
		var start interface{}
		var end interface{}
		if err := rows.Scan(&id, &root, &typ, &status, &start, &end, &processed, &errs); err != nil {
			h.fail(c, err)
			return
		}
		out = append(out, gin.H{"id": id, "storage_root_id": nullInt(root), "scan_type": nullStr(typ), "status": nullStr(status),
			"start_time": start, "end_time": end, "files_processed": nullInt(processed), "error_count": nullInt(errs)})
	}
	if err := rows.Err(); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": out, "count": len(out)})
}

// SyncStatus summarises the caller's sync_sessions.
func (h *AliasHandler) SyncStatus(c *gin.Context) {
	uid, ok := callerUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	var active int
	if err := h.db.QueryRowContext(c.Request.Context(),
		`SELECT COUNT(*) FROM sync_sessions WHERE user_id = ? AND status = 'running'`, uid).Scan(&active); err != nil {
		h.fail(c, err)
		return
	}
	var last sql.NullString
	if err := h.db.QueryRowContext(c.Request.Context(),
		`SELECT MAX(completed_at) FROM sync_sessions WHERE user_id = ? AND completed_at IS NOT NULL`, uid).Scan(&last); err != nil {
		h.fail(c, err)
		return
	}
	status := "idle"
	if active > 0 {
		status = "running"
	}
	var lastSync interface{}
	if last.Valid {
		lastSync = last.String
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "active_sessions": active, "last_sync": lastSync})
}

// SyncProviders lists the endpoint types the caller has actually configured.
func (h *AliasHandler) SyncProviders(c *gin.Context) {
	uid, ok := callerUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	rows, err := h.db.QueryContext(c.Request.Context(),
		`SELECT type, COUNT(*) FROM sync_endpoints WHERE user_id = ? GROUP BY type ORDER BY type`, uid)
	if err != nil {
		h.fail(c, err)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var typ string
		var n int
		if err := rows.Scan(&typ, &n); err != nil {
			h.fail(c, err)
			return
		}
		out = append(out, gin.H{"id": typ, "endpoints": n})
	}
	if err := rows.Err(); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"providers": out})
}

// Files forwards to the real directory browser (requires storage_root).
func (h *AliasHandler) Files(c *gin.Context) {
	if h.browse == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented: no file browser is wired"})
		return
	}
	root := c.Query("storage_root")
	if root == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "storage_root query parameter is required"})
		return
	}
	c.Params = append(c.Params, gin.Param{Key: "storage_root", Value: root})
	h.browse.BrowseDirectory(c)
}

// Subtitles lists stored subtitle tracks.
func (h *AliasHandler) Subtitles(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(),
		`SELECT id, media_item_id, language_code, format, source FROM subtitle_tracks ORDER BY id DESC LIMIT 100`)
	if err != nil {
		h.fail(c, err)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int
		var media sql.NullInt64
		var lang, format, source sql.NullString
		if err := rows.Scan(&id, &media, &lang, &format, &source); err != nil {
			h.fail(c, err)
			return
		}
		out = append(out, gin.H{"id": id, "media_item_id": nullInt(media), "language_code": nullStr(lang), "format": nullStr(format), "source": nullStr(source)})
	}
	if err := rows.Err(); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"subtitles": out, "count": len(out)})
}

// nullInt and nullStr render a possibly-NULL column as JSON null instead of failing the request.
func nullInt(v sql.NullInt64) interface{} {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func nullStr(v sql.NullString) interface{} {
	if v.Valid {
		return v.String
	}
	return nil
}
