package main

import (
	"context"
	"net/http"
	"time"

	"catalogizer/internal/metrics"
	root_repository "catalogizer/repository"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PXF23-R5a / PXF23-F4: the admin image-quality revalidation route, the admin video-thumbnail route and the
// /health/deep route used to be inline closures in main.go with no HTTP-level test. They are now these handler
// constructors (same behaviour, byte for byte), taking narrow interfaces so a test can drive the real handler over
// real HTTP (httptest) with a store that fails on demand.

// revalidationStore is the part of the image-quality repository the revalidation route needs.
type revalidationStore interface {
	SampleForRevalidation(ctx context.Context, olderThan time.Time, limit int) ([]root_repository.ImageQualityAssessment, error)
	TouchLastChecked(ctx context.Context, id int64) error
}

// thumbnailGenerator is the part of the cover-art service the thumbnail route needs.
type thumbnailGenerator interface {
	GenerateMissingVideoThumbnails(ctx context.Context, limit int) (int, error)
}

// deepHealthChecker is the part of the health checker the deep health route needs.
type deepHealthChecker interface {
	Check(ctx context.Context) metrics.HealthCheckResponse
}

// imageQualityRevalidateHandler sweeps stale image-quality rows now. A failed sample is logged server-side only
// and answers a fixed 500 body (the repository error never reaches the client).
func imageQualityRevalidateHandler(store revalidationStore, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			StaleAgeSeconds int `json:"stale_age_seconds"`
			Limit           int `json:"limit"`
		}
		if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
			// Empty or malformed body → use defaults below
		}
		staleAge := 7 * 24 * 3600
		if req.StaleAgeSeconds > 0 {
			staleAge = req.StaleAgeSeconds
		}
		limit := 256
		if req.Limit > 0 && req.Limit <= 4096 {
			limit = req.Limit
		}
		ctx := c.Request.Context()
		cutoff := time.Now().Add(-time.Duration(staleAge) * time.Second)
		rows, err := store.SampleForRevalidation(ctx, cutoff, limit)
		if err != nil {
			logger.Warn("image quality revalidation sample failed (not returned to client)", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sample images for revalidation"})
			return
		}
		touched := 0
		for _, row := range rows {
			if err := store.TouchLastChecked(ctx, row.ID); err != nil {
				continue
			}
			touched++
		}
		c.JSON(http.StatusOK, gin.H{
			"sampled":           len(rows),
			"touched":           touched,
			"stale_age_seconds": staleAge,
			"limit":             limit,
		})
	}
}

// generateThumbnailsHandler creates video-frame thumbnails for items without cover art. A generator failure is
// logged server-side only and answers a fixed 500 body.
func generateThumbnailsHandler(svc thumbnailGenerator, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Limit int `json:"limit" binding:"min=1,max=500"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			// default limit when body is empty
			req.Limit = 50
		}
		generated, err := svc.GenerateMissingVideoThumbnails(c.Request.Context(), req.Limit)
		if err != nil {
			logger.Warn("video thumbnail generation failed (not returned to client)", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate video thumbnails"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"generated": generated,
			"message":   "thumbnail generation completed",
		})
	}
}

// deepHealthHandler runs the checker with a hard timeout so a slow dependency never blocks callers: a late check answers
// 200 "degraded"; an unhealthy check answers 503. The response carries the checker's own (already redacted) report.
func deepHealthHandler(checker deepHealthChecker, version string, deepTimeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		type result struct {
			resp metrics.HealthCheckResponse
		}
		ch := make(chan result, 1)
		go func() {
			ch <- result{resp: checker.Check(c.Request.Context())}
		}()
		select {
		case r := <-ch:
			status := http.StatusOK
			if r.resp.Status == metrics.HealthStatusUnhealthy {
				status = http.StatusServiceUnavailable
			}
			c.JSON(status, r.resp)
		case <-time.After(deepTimeout):
			c.JSON(http.StatusOK, gin.H{
				"status":  "degraded",
				"time":    time.Now().UTC(),
				"version": version,
				"message": "health check exceeded 100ms timeout",
				"components": gin.H{
					"timeout": gin.H{
						"status":  "degraded",
						"message": "deep health check took too long",
					},
				},
			})
		}
	}
}
