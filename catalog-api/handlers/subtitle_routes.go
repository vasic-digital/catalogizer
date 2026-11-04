package handlers

import "github.com/gin-gonic/gin"

// RegisterSubtitleRoutes registers the subtitle route table under api (e.g. /api/v1). main.go calls
// this exact function, and the handler tests build their router with it, so the production table and
// the tested table cannot drift apart. The handler is resolved lazily on first use (get), and
// adminGate guards the provider-facing operations (search, download, translate).
func RegisterSubtitleRoutes(api *gin.RouterGroup, get func() *SubtitleHandler, adminGate gin.HandlerFunc) {
	sub := api.Group("/subtitles")
	sub.GET("/search", adminGate, func(c *gin.Context) { get().SearchSubtitles(c) })
	sub.POST("/download", adminGate, func(c *gin.Context) { get().DownloadSubtitle(c) })
	sub.GET("/media/:media_id", func(c *gin.Context) { get().GetSubtitles(c) })
	sub.GET("/:subtitle_id/verify-sync/:media_id", func(c *gin.Context) { get().VerifySubtitleSync(c) })
	sub.POST("/translate", adminGate, func(c *gin.Context) { get().TranslateSubtitle(c) })
	sub.POST("/upload", func(c *gin.Context) { get().UploadSubtitle(c) })
	sub.DELETE("/:subtitle_id", func(c *gin.Context) { get().DeleteSubtitle(c) })
	sub.GET("/languages", func(c *gin.Context) { get().GetSupportedLanguages(c) })
	sub.GET("/providers", func(c *gin.Context) { get().GetSupportedProviders(c) })
}
