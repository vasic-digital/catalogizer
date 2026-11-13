package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// Operator decision 2026-11-12 (CAT-OPDEC-A): after startup, a Redis outage
// must fall back to the in-memory limiter instead of answering 503.
// UNIT-class: miniredis is a test double (§11.4.289).

func fbRouter(cfg RedisRateLimiterConfig, mem gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg.Fallback = mem
	r := gin.New()
	r.GET("/x", RedisRateLimit(cfg), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func fbGet(r *gin.Engine) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = "10.1.2.3:4567"
	r.ServeHTTP(w, req)
	return w.Code
}

// memLimit is a tiny in-memory stand-in with a fixed budget, used to prove
// the fallback handler (not Redis) decides the answer.
func memLimit(budget int) gin.HandlerFunc {
	n := 0
	return func(c *gin.Context) {
		n++
		if n > budget {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limit_exceeded"})
			return
		}
		c.Next()
	}
}

func TestRedisRateLimit_OutageFallsBackToInMemory(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}) // nothing listens
	r := fbRouter(DefaultRedisRateLimiterConfig(client), memLimit(2))
	assert.Equal(t, http.StatusOK, fbGet(r), "outage must not return 503 when a fallback is configured")
	assert.Equal(t, http.StatusOK, fbGet(r))
	assert.Equal(t, http.StatusTooManyRequests, fbGet(r), "in-memory budget is enforced during the outage")
}

func TestRedisRateLimit_OutageWithoutFallbackStillFailsClosed(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	r := fbRouter(DefaultRedisRateLimiterConfig(client), nil)
	assert.Equal(t, http.StatusServiceUnavailable, fbGet(r))
}

func TestRedisRateLimit_RecoversToRedisAfterOutage(t *testing.T) {
	mr, client := setupMiniredis(t)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 1
	cfg.OutageBackoff = 50 * time.Millisecond // short breaker (CAT-OPA-3) so recovery is observable
	r := fbRouter(cfg, memLimit(1000))
	assert.Equal(t, http.StatusOK, fbGet(r))
	mr.Close() // outage
	assert.Equal(t, http.StatusOK, fbGet(r), "falls back while Redis is down")
	assert.NoError(t, mr.Restart())
	time.Sleep(100 * time.Millisecond)
	// miniredis keeps its data across Restart: the pre-outage request already used
	// the budget of 1, and the in-memory fallback (budget 1000) would allow this.
	assert.Equal(t, http.StatusTooManyRequests, fbGet(r), "Redis limiter is authoritative again after recovery")
}
