package handlers

import (
	"catalogizer/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterStorageRoutes mounts the storage-root endpoints on api (a group that already requires authentication).
func RegisterStorageRoutes(api *gin.RouterGroup, h *ScanHandler, auth *middleware.JWTMiddleware) {
	// Creating a root and listing a root's directories expose host/remote storage to the API, so both need the admin role
	// (CAT-001 review F1: a self-registered role-2 user could register "/" and read /etc). The root LIST carries host, path,
	// username and domain of every root, so it is admin-only too (SSE-OPQ-1, operator decision 2026-11-14); a root's connectivity
	// status stays available to any authenticated user. /browse/roots is the auth-only listing and carries no connection data.
	api.GET("/storage/list/*path", auth.RequireAdmin(), h.ListStoragePath)
	api.GET("/storage/roots", auth.RequireAdmin(), h.GetStorageRoots)
	api.POST("/storage/roots", auth.RequireAdmin(), h.CreateStorageRoot)
	api.GET("/storage-roots", auth.RequireAdmin(), h.GetStorageRoots)
	api.GET("/storage-roots/:id/status", h.GetStorageRootStatus)
}

// RegisterScanRoutes mounts the scan-job endpoints on g (the /scans group). Queueing a scan makes the server read a
// configured storage root, so it is admin-only; listing and status stay available to any authenticated user (a non-admin gets the failure text replaced by a fixed class text).
func RegisterScanRoutes(g *gin.RouterGroup, h *ScanHandler, auth *middleware.JWTMiddleware) {
	g.POST("", auth.RequireAdmin(), h.QueueScan)
	g.GET("", h.ListScans)
	g.GET("/:job_id", h.GetScanStatus)
}
