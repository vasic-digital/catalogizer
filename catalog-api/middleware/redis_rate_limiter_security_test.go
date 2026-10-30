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

func TestRedisRateLimit_SecurityBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a Redis client that will always fail
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "invalid-host:6379", // Invalid address to force failures
		Password: "",
		DB:       0,
	})

	config := DefaultRedisRateLimiterConfig(redisClient)
	config.Requests = 5
	config.Window = time.Minute

	router := gin.New()
	router.Use(RedisRateLimit(config))

	requestCount := 0
	router.GET("/test", func(c *gin.Context) {
		requestCount++
		c.JSON(http.StatusOK, gin.H{"count": requestCount})
	})

	// Make many requests - they should all be rejected due to Redis failure (fail closed)
	for i := 0; i < 10; i++ { // More than the configured limit of 5
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// All requests should fail with service unavailable due to Redis failure (security fix)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, "Request %d should be rejected with 503 due to Redis failure", i+1)
	}

	// This demonstrates the security fix: when Redis fails, all requests are blocked
	assert.Equal(t, 0, requestCount, "No requests should be processed when Redis fails")
}

// Unit-class (miniredis double): proves limiter logic only.
func TestRedisRateLimit_Unit_Miniredis_FixedBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// miniredis is an in-memory Redis double, not a real Redis server.
	_, client := setupMiniredis(t)

	config := DefaultRedisRateLimiterConfig(client)
	config.Requests = 5
	config.Window = time.Minute

	router := gin.New()
	router.Use(RedisRateLimit(config))

	requestCount := 0
	router.GET("/test", func(c *gin.Context) {
		requestCount++
		c.JSON(http.StatusOK, gin.H{"count": requestCount})
	})

	// Make 5 requests - they should all pass (within rate limit)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should pass (within limit)", i+1)
	}

	// 6th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request 6 should be rate limited")

	// Verify that exactly 5 requests were processed
	assert.Equal(t, 5, requestCount, "Exactly 5 requests should be processed")
}
