package handlers

import (
	"catalogizer/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterStorageRoutes mounts the storage-root endpoints on api (a group that already requires authentication).
func RegisterStorageRoutes(api *gin.RouterGroup, h *ScanHandler, auth *middleware.JWTMiddleware) {
	// Creating a root and listing a root's directories expose host/remote storage to the API, so both need the admin role
	// (CAT-001 review F1: a self-registered role-2 user could register "/" and read /etc). Reading the root list and a root's
	// connectivity status stays available to any authenticated user.
	api.GET("/storage/list/*path", auth.RequireAdmin(), h.ListStoragePath)
	api.GET("/storage/roots", h.GetStorageRoots)
	api.POST("/storage/roots", auth.RequireAdmin(), h.CreateStorageRoot)
	api.GET("/storage-roots", h.GetStorageRoots)
	api.GET("/storage-roots/:id/status", h.GetStorageRootStatus)
}

// RegisterScanRoutes mounts the scan-job endpoints on g (the /scans group). Queueing a scan makes the server read a
// configured storage root, so it is admin-only; listing and status stay available to any authenticated user.
func RegisterScanRoutes(g *gin.RouterGroup, h *ScanHandler, auth *middleware.JWTMiddleware) {
	g.POST("", auth.RequireAdmin(), h.QueueScan)
	g.GET("", h.ListScans)
	g.GET("/:job_id", h.GetScanStatus)
}
