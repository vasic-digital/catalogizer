package handlers

import (
	"catalogizer/internal/pathguard"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/semaphore"
)

// backupSem limits backup operations to one at a time. Both CreateBackup and
// RestoreBackup acquire this semaphore so that concurrent requests receive an
// immediate 409 Conflict rather than competing for I/O on the same database file.
var backupSem = semaphore.NewWeighted(1)

const defaultBackupDir = "./backups"

// AdminAuthServiceInterface defines the auth operations used by AdminHandler.
type AdminAuthServiceInterface interface {
	CheckPermission(userID int, permission string) (bool, error)
	GetCurrentUser(token string) (*models.User, error)
}

// AdminUserRepoInterface defines user listing operations for admin.
type AdminUserRepoInterface interface {
	List(limit, offset int) ([]models.User, error)
	GetByID(id int) (*models.User, error)
	Update(user *models.User) error
	Count() (int, error)
	DeactivateAllUserSessions(userID int) error
}

// AdminHandler provides administrative API endpoints for system info,
// user management, storage overview, and backup operations.
type AdminHandler struct {
	authService AdminAuthServiceInterface
	userRepo    AdminUserRepoInterface
	db          *database.DB
	startTime   time.Time
	version     string
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(
	authService AdminAuthServiceInterface,
	userRepo AdminUserRepoInterface,
	db *database.DB,
	version string,
) *AdminHandler {
	return &AdminHandler{
		authService: authService,
		userRepo:    userRepo,
		db:          db,
		startTime:   time.Now(),
		version:     version,
	}
}

// requireAdmin extracts the current user from the Authorization header and
// verifies the user holds system.admin permission. Returns the user on
// success or writes an error response and returns nil on failure.
func (h *AdminHandler) requireAdmin(c *gin.Context) *models.User {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil
	}
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	user, err := h.authService.GetCurrentUser(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil
	}

	allowed, err := h.authService.CheckPermission(user.ID, models.PermissionSystemAdmin)
	if err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return nil
	}

	return user
}

// GetSystemInfo returns system information matching the frontend SystemInfo type.
// GET /api/v1/admin/system-info
func (h *AdminHandler) GetSystemInfo(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptimeSeconds := int(time.Since(h.startTime).Seconds())

	// Calculate memory usage as a percentage (heap in use vs system total alloc)
	memoryUsage := float64(0)
	if memStats.Sys > 0 {
		memoryUsage = float64(memStats.HeapInuse) / float64(memStats.Sys) * 100
	}

	// Measured where the host can provide the figure; JSON null plus the "unavailable" list otherwise (never a fabricated 0, CAT-001).
	var unavailable []string
	var cpuUsage interface{}
	if v, ok := hostCPUPercent(); ok {
		cpuUsage = v
	} else {
		unavailable = append(unavailable, "cpuUsage")
	}
	var diskUsage interface{}
	// The disk that holds the data, not whatever the process cwd happens to be.
	var diskOK bool
	var dTotal, dUsed, dAvail int64
	if dir := h.db.DataDir(); dir != "" {
		dTotal, dUsed, dAvail, diskOK = diskUsageOf(dir)
	}
	if total, used, avail, ok := dTotal, dUsed, dAvail, diskOK; ok {
		diskUsage = gin.H{"total": total, "used": used, "free": avail}
	} else {
		unavailable = append(unavailable, "diskUsage")
	}
	// No request counter exists in this process.
	unavailable = append(unavailable, "totalRequests")

	c.JSON(http.StatusOK, gin.H{
		"version":           h.version,
		"uptime":            uptimeSeconds,
		"cpuUsage":          cpuUsage,
		"memoryUsage":       memoryUsage,
		"diskUsage":         diskUsage,
		"activeConnections": runtime.NumGoroutine(),
		"totalRequests":     nil,
		"unavailable":       unavailable,
	})
}

// GetUsers returns the list of registered users.
// GET /api/v1/admin/users
func (h *AdminHandler) GetUsers(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	users, err := h.userRepo.List(1000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	// Map to the shape the frontend expects.
	type adminUser struct {
		ID        string  `json:"id"`
		Username  string  `json:"username"`
		Email     string  `json:"email"`
		Role      string  `json:"role"`
		Status    string  `json:"status"`
		LastLogin *string `json:"lastLogin,omitempty"`
		CreatedAt string  `json:"createdAt"`
	}

	result := make([]adminUser, 0, len(users))
	for _, u := range users {
		status := "active"
		if !u.IsActive {
			status = "inactive"
		}
		if u.IsLocked {
			status = "suspended"
		}

		roleName := "user"
		if u.Role != nil {
			roleName = u.Role.Name
		}

		au := adminUser{
			ID:        strconv.Itoa(u.ID),
			Username:  u.Username,
			Email:     u.Email,
			Role:      roleName,
			Status:    status,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		}

		if u.LastLoginAt != nil {
			formatted := u.LastLoginAt.Format(time.RFC3339)
			au.LastLogin = &formatted
		}

		result = append(result, au)
	}

	c.JSON(http.StatusOK, result)
}

// UpdateUser updates a user's role or status.
// PUT /api/v1/admin/users/:id
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userRepo.GetByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	var updates struct {
		Role   *string `json:"role"`
		Status *string `json:"status"`
		Email  *string `json:"email"`
	}

	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	wasActive := user.IsActive
	if updates.Status != nil {
		switch *updates.Status {
		case "active":
			user.IsActive = true
			user.IsLocked = false
		case "inactive":
			user.IsActive = false
		case "suspended":
			user.IsLocked = true
		}
	}

	if updates.Email != nil {
		user.Email = *updates.Email
	}

	// Revoke first (fail closed): a failed revocation leaves the user unchanged,
	// so a retry still revokes.
	if wasActive && !user.IsActive || (updates.Status != nil && !user.IsActive) {
		if err := revokeUserSessions(h.userRepo, user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke sessions; user not updated"})
			return
		}
	}
	if err := h.userRepo.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	// Second revoke closes the window for a login between the first revoke and
	// the update (that session carries the old claims).
	if wasActive && !user.IsActive || (updates.Status != nil && !user.IsActive) {
		if err := revokeUserSessions(h.userRepo, user.ID); err != nil {
			log.Printf("handlers: user %d updated but the post-update session revoke failed: %v", user.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "User updated but revoking sessions failed; repeat the request"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// GetStorageInfo returns storage root information.
// GET /api/v1/admin/storage
func (h *AdminHandler) GetStorageInfo(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	type storageInfo struct {
		Path           string  `json:"path"`
		TotalSpace     *int64  `json:"totalSpace"` // nil = not measurable (remote protocols), never a fake 0
		UsedSpace      *int64  `json:"usedSpace"`
		AvailableSpace *int64  `json:"availableSpace"`
		MediaCount     int     `json:"mediaCount"`
		LastScan       *string `json:"lastScan,omitempty"`
	}

	// Query storage roots from database.
	rows, err := h.db.Query(`
		SELECT id, name, path, protocol, last_scan_at
		FROM storage_roots
		ORDER BY name
	`)
	if err != nil {
		// CAT-001 (R4-9): a database failure is not "no storage". Answering 200 [] made the
		// UI show an empty, healthy-looking list; report the failure instead.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read storage roots"})
		return
	}
	defer rows.Close()

	var result []storageInfo
	for rows.Next() {
		var id int
		var name, path, protocol string
		var lastScan *time.Time

		if err := rows.Scan(&id, &name, &path, &protocol, &lastScan); err != nil {
			continue
		}

		// Count files for this storage root.
		var mediaCount int
		_ = h.db.QueryRow(`SELECT COUNT(*) FROM files WHERE storage_root_id = ?`, id).Scan(&mediaCount)

		si := storageInfo{
			Path:       fmt.Sprintf("%s://%s", protocol, path),
			MediaCount: mediaCount,
		}
		// Space is measured for a local root (statfs of its directory); the capacity of a remote share is not known here.
		if protocol == "local" {
			if total, used, avail, ok := diskUsageOf(path); ok {
				si.TotalSpace, si.UsedSpace, si.AvailableSpace = &total, &used, &avail
			}
		}

		if lastScan != nil {
			formatted := lastScan.Format(time.RFC3339)
			si.LastScan = &formatted
		}

		result = append(result, si)
	}

	if result == nil {
		result = []storageInfo{}
	}

	c.JSON(http.StatusOK, result)
}

// GetBackups returns a list of backups found in the backups directory.
// GET /api/v1/admin/backups
func (h *AdminHandler) GetBackups(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	entries, err := os.ReadDir(defaultBackupDir)
	if err != nil {
		// Directory doesn't exist or can't be read; return empty list.
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	type backupInfo struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Size      int64  `json:"size"`
		CreatedAt string `json:"created_at"`
	}

	var backups []backupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".db") && !strings.HasSuffix(name, ".sql") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		bType := "full"
		if strings.HasSuffix(name, ".sql") {
			bType = "sql"
		}

		backups = append(backups, backupInfo{
			ID:        name,
			Type:      bType,
			Size:      info.Size(),
			CreatedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	if backups == nil {
		backups = []backupInfo{}
	}

	c.JSON(http.StatusOK, backups)
}

// CreateBackup creates a new database backup.
// POST /api/v1/admin/backups
func (h *AdminHandler) CreateBackup(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	// Only one backup/restore operation at a time.
	if !backupSem.TryAcquire(1) {
		c.JSON(http.StatusConflict, gin.H{"error": "Another backup operation is in progress"})
		return
	}
	defer backupSem.Release(1)

	var req struct {
		Type string `json:"type"` // "full" or "incremental"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not configured"})
		return
	}

	if h.db.DatabaseType() != "sqlite" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Backup currently supported only for SQLite databases"})
		return
	}

	dbPath := h.db.DBPath()
	if dbPath == "" {
		dbPath = "./data/catalogizer.db"
	}

	// Verify the source database file exists.
	if _, err := os.Stat(dbPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database file not found"})
		return
	}

	// Create backup directory if it doesn't exist.
	if err := os.MkdirAll(defaultBackupDir, 0750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backup directory"})
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s.db", timestamp)
	destPath := filepath.Join(defaultBackupDir, filename)
	// Two backups in the same second must not collide: BackupTo never
	// overwrites, so pick the next free name.
	for i := 1; ; i++ {
		if _, err := os.Lstat(destPath); os.IsNotExist(err) {
			break
		}
		filename = fmt.Sprintf("backup_%s_%d.db", timestamp, i)
		destPath = filepath.Join(defaultBackupDir, filename)
	}

	// CAT-001: use the database-aware backup (WAL checkpoint + sidecar copy).
	// A raw single-file copy loses commits still in the -wal file and, for the
	// encrypted database, must still be an encrypted, 0600 file.
	if err := h.db.BackupTo(destPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write backup file"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"id":      filename,
		"status":  "completed",
		"message": "Backup created",
	})
}

// RestoreBackup validates the backup id and answers 501: no restore is implemented.
// POST /api/v1/admin/backups/:id/restore
func (h *AdminHandler) RestoreBackup(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	// Only one backup/restore operation at a time.
	if !backupSem.TryAcquire(1) {
		c.JSON(http.StatusConflict, gin.H{"error": "Another backup operation is in progress"})
		return
	}
	defer backupSem.Release(1)

	id := c.Param("id")

	// Sanitize: only allow filenames, no path traversal.
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}

	// "." resolves to the backup directory itself; require a plain single-element id
	// that names a regular file (a symlink or directory is not a backup).
	if !pathguard.SafeComponent(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}
	backupPath := filepath.Join(defaultBackupDir, id)
	if fi, err := os.Lstat(backupPath); err != nil || !fi.Mode().IsRegular() {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Backup '%s' not found", id)})
		return
	}

	// No restore is queued anywhere: nothing reads a restore marker on
	// restart. Answering 202 "queued" would be a fabricated success (CAT-001),
	// so say so, consistent with DeleteBackup.
	c.JSON(http.StatusNotImplemented, gin.H{
		"id":    id,
		"error": "restoring a backup through the API is not implemented; restore offline with the database restore tooling",
	})
}

// ScanStorage validates the request and answers 501: the real trigger is POST /api/v1/scans.
// POST /api/v1/admin/storage/scan
func (h *AdminHandler) ScanStorage(c *gin.Context) {
	if h.requireAdmin(c) == nil {
		return
	}

	var req struct {
		Path          string `json:"path"`
		StorageRootID *int   `json:"storage_root_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if strings.TrimSpace(req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path is required"})
		return
	}

	// If a storage root ID is provided, verify it exists.
	if req.StorageRootID != nil {
		var count int
		err := h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots WHERE id = ?`, *req.StorageRootID).Scan(&count)
		if err != nil || count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Storage root %d not found", *req.StorageRootID)})
			return
		}
	}

	// This endpoint never started a scan (CAT-001). The real, admin-gated
	// scan trigger is POST /api/v1/scans.
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "this endpoint does not start a scan; use POST /api/v1/scans",
		"path":  req.Path,
	})
}
