package metrics

import (
	"catalogizer/internal/logredact"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GinMiddleware returns a Gin middleware that records HTTP request duration,
// request count, and active connection tracking for Prometheus metrics.
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip metrics endpoint itself to avoid self-referencing noise.
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		HTTPActiveConnections.Inc()
		start := time.Now()

		c.Next()

		HTTPActiveConnections.Dec()
		duration := time.Since(start)

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := normalizePath(c)

		HTTPRequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())
		HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	}
}

// normalizePath returns the matched route pattern if available,
// falling back to the raw path. Using route patterns prevents
// high-cardinality label values from path parameters.
func normalizePath(c *gin.Context) string {
	// FullPath returns the matched route pattern, e.g. "/api/v1/catalog/*path"
	if fp := c.FullPath(); fp != "" {
		return fp
	}
	return logredact.SecretPath(c.Request.URL.Path)
}
