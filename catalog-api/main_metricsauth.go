package main

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// metricsScrapeTokenEnv names the dedicated Prometheus scrape credential. A scraper
// cannot log in, so GET /metrics (and nothing else) also accepts this bearer token.
const metricsScrapeTokenEnv = "METRICS_SCRAPE_TOKEN"

// minMetricsScrapeTokenLen is the shortest token accepted; anything shorter is ignored
// (administrator JWT only).
const minMetricsScrapeTokenLen = 32

const metricsScrapeOKKey = "metrics_scrape_ok"

// metricsScrapeToken returns the configured token, or "" when unset or too short.
func metricsScrapeToken() string {
	t := os.Getenv(metricsScrapeTokenEnv)
	if len(t) < minMetricsScrapeTokenLen {
		return ""
	}
	return t
}

// metricsScrapeDetect marks the request as a scrape when its bearer token equals token
// (constant-time compare). It never aborts and never logs the credential.
func metricsScrapeDetect(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token != "" {
			if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") &&
				subtle.ConstantTimeCompare([]byte(h[len("Bearer "):]), []byte(token)) == 1 {
				c.Set(metricsScrapeOKKey, true)
			}
		}
		c.Next()
	}
}

// metricsSkipIfScrape runs h (RequireAuth / RequireAdmin) unless the scrape token matched.
func metricsSkipIfScrape(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool(metricsScrapeOKKey) {
			c.Next()
			return
		}
		h(c)
	}
}
