package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// createTestRequest creates a simple HTTP request for testing
func createTestRequest() *http.Request {
	return httptest.NewRequest("GET", "/", nil)
}

// setupMiniredis creates an in-memory Redis server (miniredis) and returns a
// client connected to it. miniredis is a test double: every test that uses it
// is UNIT-class (§11.4.27 / §11.4.289) and proves only the middleware logic,
// not behaviour against a real Redis server. Live-class coverage lives in
// redis_rate_limiter_live_test.go.
func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	t.Cleanup(func() { mr.Close() })

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, client
}

func TestDefaultRedisRateLimiterConfig(t *testing.T) {
	_, client := setupMiniredis(t)
	config := DefaultRedisRateLimiterConfig(client)

	assert.Equal(t, 100, config.Requests)
	assert.Equal(t, time.Minute, config.Window)
	assert.NotEmpty(t, config.Message)
	assert.NotNil(t, config.Client)
	assert.NotNil(t, config.KeyGenerator)
}

func TestStrictRedisRateLimiterConfig(t *testing.T) {
	_, client := setupMiniredis(t)
	config := StrictRedisRateLimiterConfig(client)

	assert.Equal(t, 10, config.Requests)
	assert.Equal(t, time.Minute, config.Window)
	assert.NotEmpty(t, config.Message)
	assert.NotNil(t, config.Client)
	assert.NotNil(t, config.KeyGenerator)
}

func TestAuthRedisRateLimiterConfig(t *testing.T) {
	_, client := setupMiniredis(t)
	config := AuthRedisRateLimiterConfig(client)

	assert.Equal(t, 5, config.Requests)
	assert.Equal(t, time.Minute, config.Window)
	assert.NotEmpty(t, config.Message)
	assert.NotNil(t, config.Client)
	assert.NotNil(t, config.KeyGenerator)
}

func TestUserRedisRateLimiterConfig(t *testing.T) {
	_, client := setupMiniredis(t)
	config := UserRedisRateLimiterConfig(client)

	assert.Equal(t, 200, config.Requests)
	assert.Equal(t, time.Minute, config.Window)
	assert.NotEmpty(t, config.Message)
	assert.NotNil(t, config.Client)
	assert.NotNil(t, config.KeyGenerator)
}

// Unit-class (miniredis double).
func TestRedisRateLimit_Unit_Miniredis_LimitEnforced(t *testing.T) {
	_, client := setupMiniredis(t)

	gin.SetMode(gin.TestMode)
	config := DefaultRedisRateLimiterConfig(client)
	config.Requests = 3             // Low limit for testing
	config.Window = time.Second * 2 // Short window for testing
	config.KeyGenerator = func(c *gin.Context) string {
		return "test_ip"
	}

	middleware := RedisRateLimit(config)

	// First request should pass
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = createTestRequest()
	c.Set("test_key", "test_value")

	middleware(c)
	assert.False(t, c.IsAborted())

	// Second request should pass
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = createTestRequestWithIP()

	middleware(c2)
	assert.False(t, c2.IsAborted())

	// Third request should pass
	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	c3.Request = createTestRequest()

	middleware(c3)
	assert.False(t, c3.IsAborted())

	// Fourth request should be rate limited
	c4, _ := gin.CreateTestContext(httptest.NewRecorder())
	c4.Request = createTestRequest()

	middleware(c4)
	assert.True(t, c4.IsAborted())
	assert.Equal(t, 429, c4.Writer.Status())
}

// Unit-class (miniredis double).
func TestSlidingWindowRedisRateLimit_Unit_Miniredis_LimitEnforced(t *testing.T) {
	_, client := setupMiniredis(t)

	gin.SetMode(gin.TestMode)
	config := DefaultRedisRateLimiterConfig(client)
	config.Requests = 3
	config.Window = time.Second
	config.KeyGenerator = func(c *gin.Context) string {
		return "test_ip"
	}

	middleware := SlidingWindowRedisRateLimit(config)

	// Make requests that should pass
	for i := 0; i < 3; i++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = createTestRequest()

		middleware(c)
		assert.False(t, c.IsAborted(), "Request %d should pass", i+1)
	}

	// Next request should be rate limited
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = createTestRequest()

	middleware(c)
	assert.True(t, c.IsAborted())
	assert.Equal(t, 429, c.Writer.Status())
}

// Unit-class (miniredis double).
func TestTokenBucketRedisRateLimit_Unit_Miniredis_RefillAfterEmpty(t *testing.T) {
	mr, client := setupMiniredis(t)

	gin.SetMode(gin.TestMode)

	middleware := TokenBucketRedisRateLimit(
		client,
		func(c *gin.Context) string { return "test_ip" },
		5,           // Max tokens
		1,           // Refill rate: 1 token per second
		time.Second, // Refill interval
	)

	// Make requests that should consume tokens
	for i := 0; i < 5; i++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = createTestRequest()

		middleware(c)
		assert.False(t, c.IsAborted(), "Request %d should pass", i+1)
	}

	// Next request should be rate limited (bucket empty)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = createTestRequest()

	middleware(c)
	assert.True(t, c.IsAborted())
	assert.Equal(t, 429, c.Writer.Status())

	// Fast-forward time in miniredis to simulate token refill
	mr.FastForward(time.Second * 2)

	// Request should pass again after refill
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = createTestRequestWithIP()

	middleware(c2)
	assert.False(t, c2.IsAborted())
}

func TestRedisRateLimit_ErrorHandling(t *testing.T) {
	// Use invalid Redis client to test error handling
	client := redis.NewClient(&redis.Options{
		Addr: "invalid-host:6379",
	})

	gin.SetMode(gin.TestMode)
	config := DefaultRedisRateLimiterConfig(client)
	middleware := RedisRateLimit(config)

	// Request should fail due to Redis error (fail closed)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createTestRequest()

	middleware(c)
	assert.True(t, c.IsAborted()) // Should abort due to Redis error
}

func TestAuthRateLimiterKeyGeneration(t *testing.T) {
	_, client := setupMiniredis(t)
	config := AuthRedisRateLimiterConfig(client)

	// Test key generation without username
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createTestRequest()
	key := config.KeyGenerator(c)
	assert.Contains(t, key, ".")

	// Test key generation with username
	c2, _ := gin.CreateTestContext(w)
	c2.Request = createTestRequestWithIP()
	c2.Set("username", "testuser")
	key2 := config.KeyGenerator(c2)
	assert.Contains(t, key2, "testuser:")
}

func TestUserRateLimiterKeyGeneration(t *testing.T) {
	_, client := setupMiniredis(t)
	config := UserRedisRateLimiterConfig(client)

	// Test key generation without user ID (should fall back to IP)
	c, _ := gin.CreateTestContext(nil)
	c.Request = createTestRequest()
	key := config.KeyGenerator(c)
	assert.Contains(t, key, "ip:")

	// Test key generation with user ID
	c2, _ := gin.CreateTestContext(nil)
	c2.Request = createTestRequestWithIP()
	c2.Set("user_id", "123")
	key2 := config.KeyGenerator(c2)
	assert.Equal(t, "user:123", key2)
}

// Helper function to create test HTTP request (using existing function)
func createTestRequestWithIP() *http.Request {
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	return req
}
