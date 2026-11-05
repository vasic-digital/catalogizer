package handlers

import (
	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/netguard"
	"catalogizer/internal/services"
	"catalogizer/models"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// scannerInterface defines the scanner methods used by ScanHandler
type scannerInterface interface {
	QueueScan(job services.ScanJob) error
	GetAllActiveScanStatuses() map[string]*services.ScanStatus
	GetActiveScanStatus(jobID string) (*services.ScanStatus, bool)
}

// ScanHandler wraps UniversalScanner with REST API endpoints for
// managing storage roots and triggering scan operations.
type ScanHandler struct {
	scanner scannerInterface
	db      *database.DB
	// clientFactory builds the real protocol client used to probe a storage root's connectivity (GetStorageRootStatus).
	clientFactory filesystem.ClientFactory
	// listMaxEntries bounds one /storage/list response (0 = defaultListMaxEntries).
	listMaxEntries int
}

// defaultListMaxEntries bounds the entries one /storage/list call may return; more are reported with "truncated": true.
const defaultListMaxEntries = 1000

// storageDisconnectTimeout bounds the Disconnect of a probe/list client so a hung server cannot hold the request goroutine.
const storageDisconnectTimeout = 5 * time.Second

// disconnectBounded closes client under storageDisconnectTimeout.
func disconnectBounded(client filesystem.FileSystemClient) {
	dctx, cancel := context.WithTimeout(context.Background(), storageDisconnectTimeout)
	defer cancel()
	_ = client.Disconnect(dctx)
}

// NewScanHandler creates a new ScanHandler.
func NewScanHandler(scanner scannerInterface, db *database.DB) *ScanHandler {
	return &ScanHandler{scanner: scanner, db: db, clientFactory: filesystem.NewDefaultClientFactory()}
}

// clientForRoot builds the real protocol client of a storage root: the same settings mapping (filesystem.SettingsFromRoot) and factory the
// scanner uses, so a probe or listing meets exactly what a scan would meet.
func (h *ScanHandler) clientForRoot(root *models.StorageRoot) (filesystem.FileSystemClient, error) {
	// A local root is re-checked on every use: the allowed bases (or a symlink on disk) may have changed since it was stored.
	if root.Protocol == "local" {
		if err := filesystem.CheckLocalRoot(derefString(root.Path), filesystem.AllowedLocalBases()); err != nil {
			return nil, err
		}
	}
	settings, err := filesystem.SettingsFromRoot(root, services.ResolveSMBIdentity)
	if err != nil {
		return nil, err
	}
	return h.clientFactory.CreateClient(&filesystem.StorageConfig{ID: root.Name, Name: root.Name, Protocol: root.Protocol, Settings: settings})
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// storageRootProbeTimeout bounds one connectivity probe so a black-holed host cannot hold a status request open.
const storageRootProbeTimeout = 10 * time.Second

// createStorageRootRequest is the JSON body for POST /storage/roots.
type createStorageRootRequest struct {
	Name     string  `json:"name" binding:"required"`
	Protocol string  `json:"protocol" binding:"required"`
	Host     *string `json:"host"`
	Port     *int    `json:"port"`
	Path     *string `json:"path"`
	Username *string `json:"username"`
	Password *string `json:"password"`
	Domain   *string `json:"domain"`
	MaxDepth int     `json:"max_depth"`
	// URL is the address of a WebDAV root (https://host/dav); MountPoint and Options are the NFS local mount point and mount options
	// (for the other protocols Options is a JSON object with the tls / pin / credential-reference keys). WF22 H2: without these fields an
	// sftp / webdav / nfs root could not be created through the API, so no scan of one could ever be queued.
	URL        *string `json:"url"`
	MountPoint *string `json:"mount_point"`
	Options    *string `json:"options"`
	// AllowEmpty marks a root that is expected to be empty: its scan may complete with 0 files instead of failing (default false).
	AllowEmpty bool `json:"allow_empty"`
}

// supportedStorageProtocol reports whether a CreateStorageRoot request may declare protocol. The set IS the set the client factory builds
// (filesystem.DefaultClientFactory.SupportedProtocols: the built-ins plus every protocol registered through filesystem.RegisterProtocol), so
// a protocol the scanner advertises is creatable and one it cannot drive is refused (CATAPI-DEFECT-004 closed against a single source, WF22 H2).
func supportedStorageProtocol(protocol string) bool {
	for _, p := range filesystem.NewDefaultClientFactory().SupportedProtocols() {
		if p == protocol {
			return true
		}
	}
	return false
}

// validateWebDAVRootURL applies the operator-class outbound policy (netguard.CheckOperatorURL) to the URL of a WebDAV storage root.
func validateWebDAVRootURL(raw *string) error { return validateRootURL("webdav", raw) }

// validateRootURL is the operator-class check for every protocol whose root is an operator-chosen URL (webdav, s3).
func validateRootURL(protocol string, raw *string) error {
	if raw == nil || *raw == "" {
		return fmt.Errorf("%s storage root needs a url", protocol)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// WebDAV roots may carry userinfo (the stored URL is redacted on every read path), which says nothing about where the connection
	// goes: check the destination without it.
	target := *raw
	if u, perr := url.Parse(target); perr == nil && u.User != nil {
		u.User = nil
		target = u.String()
	}
	if _, err := netguard.CheckOperatorURL(ctx, target, "http", "https"); err != nil {
		return fmt.Errorf("%s url refused: %w", protocol, err)
	}
	return nil
}

// CreateStorageRoot handles POST /api/v1/storage/roots.
// Strictly creates a new storage root. If a row with the same name
// already exists, returns 409 Conflict (CATAPI-DEFECT-005). For
// idempotent upsert semantics the caller should use PUT
// /api/v1/storage/roots/{id} instead.
//
// Validates Protocol against supportedStorageProtocol
// (CATAPI-DEFECT-004). Unknown protocols return 400 with an
// explanatory message and the list of accepted values.
func (h *ScanHandler) CreateStorageRoot(c *gin.Context) {
	var req createStorageRootRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// CATAPI-DEFECT-004: enforce protocol allowlist.
	if !supportedStorageProtocol(req.Protocol) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "unsupported protocol",
			"protocol": req.Protocol,
			"accepted": filesystem.NewDefaultClientFactory().SupportedProtocols(),
		})
		return
	}

	// The root is validated as a ROW (filesystem.ValidateRoot, WF22 N3): every column it carries must be one the protocol consumes, every key the
	// protocol requires must be there, the options must parse and be honourable, the port must be 1-65535. A root that could never be scanned is
	// refused here instead of being stored and silently ignored.
	candidate := &models.StorageRoot{Protocol: req.Protocol, Host: req.Host, Port: req.Port, Path: req.Path, Username: req.Username, Password: req.Password,
		Domain: req.Domain, URL: req.URL, MountPoint: req.MountPoint, Options: req.Options}
	if req.Port != nil && (*req.Port < 1 || *req.Port > 65535) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "port must be 1-65535"})
		return
	}
	if err := filesystem.ValidateRoot(candidate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// SECQ-1/W5: early refusal of a WebDAV root aimed at loopback / link-local / cloud-metadata addresses. The dial of the client checks the
	// resolved address again on every connection and redirect hop, so this is for a clear error, not the only line of defence.
	if req.Protocol == "webdav" || req.Protocol == "s3" {
		if err := validateRootURL(req.Protocol, req.URL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if req.Protocol == "local" {
		resolved, err := filesystem.ResolveLocalRoot(derefString(req.Path), filesystem.AllowedLocalBases())
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		req.Path = &resolved // the resolved path is stored, not the symlink the caller named
	}

	if req.MaxDepth <= 0 {
		req.MaxDepth = 10
	}

	// CATAPI-DEFECT-005: refuse duplicate names with 409. Previous
	// behavior silently upserted and returned 201, hiding the
	// conflict from clients that genuinely meant "create".
	var existingID int64
	err := h.db.QueryRowContext(c.Request.Context(),
		"SELECT id FROM storage_roots WHERE name = ?", req.Name,
	).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":       "storage root with this name already exists",
			"name":        req.Name,
			"existing_id": existingID,
			"hint":        "use PUT /api/v1/storage/roots/{id} to update an existing root",
		})
		return
	}

	newID, insertErr := h.db.InsertReturningID(c.Request.Context(),
		`INSERT INTO storage_roots (name, protocol, host, port, path, username, password, domain, url, mount_point, options, allow_empty, enabled, max_depth)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Protocol, req.Host, req.Port, req.Path,
		req.Username, req.Password, req.Domain, req.URL, req.MountPoint, req.Options, req.AllowEmpty, true, req.MaxDepth,
	)
	if insertErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create storage root: %v", insertErr)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       newID,
		"name":     req.Name,
		"protocol": req.Protocol,
		"message":  "storage root created",
	})
}

// GetStorageRoots handles GET /api/v1/storage/roots.
// Returns all storage roots from the database.
func (h *ScanHandler) GetStorageRoots(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(),
		`SELECT id, name, protocol, host, port, path, username, domain, url, mount_point, allow_empty, enabled, max_depth,
		        created_at, updated_at, last_scan_at
		 FROM storage_roots ORDER BY id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to query storage roots: %v", err)})
		return
	}
	defer rows.Close()

	var roots []gin.H
	for rows.Next() {
		var (
			id                   int64
			name, protocol       string
			host, path, username *string
			domain, rawURL       *string
			mountPoint           *string
			port                 *int
			allowEmpty, enabled  bool
			maxDepth             int
			createdAt, updatedAt time.Time
			lastScanAt           *time.Time
		)
		if err := rows.Scan(&id, &name, &protocol, &host, &port, &path, &username, &domain, &rawURL, &mountPoint, &allowEmpty, &enabled, &maxDepth, &createdAt, &updatedAt, &lastScanAt); err != nil {
			continue
		}
		// The url of a WebDAV root may carry credentials in its userinfo: it is never returned with them.
		var shownURL *string
		if rawURL != nil {
			if u, perr := url.Parse(*rawURL); perr == nil {
				r := u.Redacted()
				shownURL = &r
			}
		}
		roots = append(roots, gin.H{
			"id":           id,
			"name":         name,
			"protocol":     protocol,
			"host":         host,
			"port":         port,
			"path":         path,
			"username":     username,
			"domain":       domain,
			"url":          shownURL,
			"mount_point":  mountPoint,
			"allow_empty":  allowEmpty,
			"enabled":      enabled,
			"max_depth":    maxDepth,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
			"last_scan_at": lastScanAt,
		})
	}

	if roots == nil {
		roots = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{"roots": roots})
}

// GetStorageRootStatus handles GET /api/v1/storage-roots/:id/status.
// Returns connectivity status for a storage root.
func (h *ScanHandler) GetStorageRootStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid storage root ID"})
		return
	}

	root, err := h.loadStorageRoot(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage root not found"})
		return
	}

	resp := gin.H{"id": id, "checked_at": time.Now().Format(time.RFC3339)}
	if !root.Enabled {
		// A disabled root is never scanned or probed: it is reported as such, not as online.
		resp["connected"], resp["status"] = false, "disabled"
		c.JSON(http.StatusOK, resp)
		return
	}

	// Probe the root with the same settings mapping and client factory the scanner uses, so the answer is the one a scan would meet.
	fsClient, err := h.clientForRoot(root)
	if err != nil {
		resp["connected"], resp["status"], resp["error"] = false, "misconfigured", services.ScrubErrorFor(err, root)
		c.JSON(http.StatusOK, resp)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), storageRootProbeTimeout)
	defer cancel()
	if err := fsClient.Connect(ctx); err != nil {
		resp["connected"], resp["status"], resp["error"] = false, "offline", services.ScrubErrorFor(err, root)
		c.JSON(http.StatusOK, resp)
		return
	}
	defer disconnectBounded(fsClient)
	if err := fsClient.TestConnection(ctx); err != nil {
		resp["connected"], resp["status"], resp["error"] = false, "offline", services.ScrubErrorFor(err, root)
		c.JSON(http.StatusOK, resp)
		return
	}
	resp["connected"], resp["status"] = true, "online"
	c.JSON(http.StatusOK, resp)
}

// ListStoragePath handles GET /api/v1/storage/list/*path?storage_id=<storage root id>.
// Lists the directory through the same settings mapping and real protocol client the scanner uses; it never answers with a fabricated listing
// (CAT-001: the former handler returned an always-empty "mock" list).
func (h *ScanHandler) ListStoragePath(c *gin.Context) {
	idStr := c.Query("storage_id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "storage_id is required"})
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "storage_id must be a storage root id"})
		return
	}
	root, err := h.loadStorageRoot(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Storage root not found"})
		return
	}
	if !root.Enabled {
		c.JSON(http.StatusConflict, gin.H{"error": "storage root is disabled"})
		return
	}
	fsClient, err := h.clientForRoot(root)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": services.ScrubErrorFor(err, root)})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), storageRootProbeTimeout)
	defer cancel()
	if err := fsClient.Connect(ctx); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": services.ScrubErrorFor(err, root)})
		return
	}
	defer disconnectBounded(fsClient)

	dir := "/" + strings.TrimLeft(c.Param("path"), "/")
	limit := h.listMaxEntries
	if limit < 1 {
		limit = defaultListMaxEntries
	}
	// Ask for limit+1 so that "more than the bound" stays detectable (the client may stop reading early).
	entries, err := fsClient.ListDirectory(filesystem.WithListLimit(ctx, limit+1), dir)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": services.ScrubErrorFor(err, root)})
		return
	}
	truncated := len(entries) > limit
	if truncated {
		entries = entries[:limit]
	}
	files := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		files = append(files, gin.H{"name": e.Name, "path": e.Path, "size": e.Size, "is_dir": e.IsDir, "modified_at": e.ModTime.Format(time.RFC3339)})
	}
	c.JSON(http.StatusOK, gin.H{"path": dir, "storage_id": id, "files": files, "truncated": truncated})
}

// queueScanRequest is the JSON body for POST /scans.
type queueScanRequest struct {
	StorageRootID int64  `json:"storage_root_id" binding:"required"`
	Path          string `json:"path"`
	ScanType      string `json:"scan_type"`
	MaxDepth      int    `json:"max_depth"`
}

// QueueScan handles POST /api/v1/scans.
// Looks up the storage root, builds a ScanJob, and queues it.
func (h *ScanHandler) QueueScan(c *gin.Context) {
	var req queueScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ScanType == "" {
		req.ScanType = "full"
	}
	if req.MaxDepth <= 0 {
		req.MaxDepth = 10
	}

	// Load storage root from DB
	root, err := h.loadStorageRoot(c.Request.Context(), req.StorageRootID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("storage root not found: %v", err)})
		return
	}

	if root.Protocol == "local" {
		if err := filesystem.CheckLocalRoot(derefString(root.Path), filesystem.AllowedLocalBases()); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
	}

	jobID := uuid.New().String()
	job := services.ScanJob{
		ID:          jobID,
		StorageRoot: root,
		Path:        req.Path,
		ScanType:    req.ScanType,
		MaxDepth:    req.MaxDepth,
		Context:     context.Background(),
	}

	if err := h.scanner.QueueScan(job); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("failed to queue scan: %v", err)})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":          jobID,
		"storage_root_id": req.StorageRootID,
		"scan_type":       req.ScanType,
		"status":          "queued",
		"message":         "scan job queued",
	})
}

// ListScans handles GET /api/v1/scans.
// Returns all active scan statuses.
func (h *ScanHandler) ListScans(c *gin.Context) {
	statuses := h.scanner.GetAllActiveScanStatuses()
	scans := make([]gin.H, 0, len(statuses))
	for id, s := range statuses {
		scans = append(scans, scanStatusToJSON(id, s))
	}
	c.JSON(http.StatusOK, gin.H{"scans": scans})
}

// GetScanStatus handles GET /api/v1/scans/:job_id.
// Returns the status of a specific scan job.
func (h *ScanHandler) GetScanStatus(c *gin.Context) {
	jobID := c.Param("job_id")
	status, exists := h.scanner.GetActiveScanStatus(jobID)
	if !exists {
		// Check completed scans table if we add one later;
		// for now return a synthetic "completed" or "not found"
		c.JSON(http.StatusNotFound, gin.H{
			"job_id": jobID,
			"status": "not_found",
			"error":  "scan job not found (may have already completed)",
		})
		return
	}
	snapshot := status.GetSnapshot()
	c.JSON(http.StatusOK, scanStatusToJSON(jobID, &snapshot))
}

// loadStorageRoot reads a StorageRoot from the database by ID. It selects every column the settings contract consumes
// (models.StorageRootConnColumns): the url of a WebDAV root and the mount point of an NFS one included - the former column list left them out, so a
// WebDAV or NFS scan queued through the API reached the client factory without its address (WF22 H1).
func (h *ScanHandler) loadStorageRoot(ctx context.Context, id int64) (*models.StorageRoot, error) {
	row := h.db.QueryRowContext(ctx,
		`SELECT id, name, enabled, max_depth, allow_empty, `+models.StorageRootConnColumns+`
		 FROM storage_roots WHERE id = ?`, id)

	var root models.StorageRoot
	dest := append([]interface{}{&root.ID, &root.Name, &root.Enabled, &root.MaxDepth, &root.AllowEmpty}, root.ConnScanTargets()...)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	return &root, nil
}

// scanStatusToJSON converts a ScanStatus to a JSON-friendly map.
func scanStatusToJSON(jobID string, s *services.ScanStatus) gin.H {
	elapsed := time.Since(s.StartTime).Milliseconds()
	return gin.H{
		"job_id":          jobID,
		"storage_root":    s.StorageRootName,
		"protocol":        s.Protocol,
		"status":          s.Status,
		"start_time":      s.StartTime,
		"elapsed_ms":      elapsed,
		"current_path":    s.CurrentPath,
		"files_processed": s.FilesProcessed,
		"files_found":     s.FilesFound,
		"files_updated":   s.FilesUpdated,
		"files_deleted":   s.FilesDeleted,
		"error_count":     s.ErrorCount,
		"reason":          s.Reason,
	}
}
