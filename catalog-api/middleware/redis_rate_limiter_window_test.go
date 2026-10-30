package middleware

// Unit-class tests (§11.4.27 / §11.4.289 clause C(1)): these run against
// miniredis, an in-memory Redis double. They prove the middleware's own
// window/fail-closed logic; they are NOT evidence that rate limiting works
// against a real Redis server. The live-class counterpart is
// TestRedisRateLimit_LiveRedis_WindowExpiresFromFirstRequest in
// redis_rate_limiter_live_test.go.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedWindowRouter builds a router guarded by RedisRateLimit with a constant key.
func fixedWindowRouter(cfg RedisRateLimiterConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg.KeyGenerator = func(c *gin.Context) string { return "window_test" }
	r := gin.New()
	r.Use(RedisRateLimit(cfg))
	r.GET("/t", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doGet(r *gin.Engine) int {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/t", nil))
	return w.Code
}

// TestRedisRateLimit_Unit_WindowExpiresFromFirstRequest is the regression test
// for the sliding-TTL defect: the fixed window MUST end Window after the FIRST
// request in that window, even when requests keep arriving inside it.
func TestRedisRateLimit_Unit_WindowExpiresFromFirstRequest(t *testing.T) {
	mr, client := setupMiniredis(t)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 2
	cfg.Window = 2 * time.Second
	r := fixedWindowRouter(cfg)

	require.Equal(t, http.StatusOK, doGet(r), "request 1 at t=0")
	mr.FastForward(1500 * time.Millisecond)
	require.Equal(t, http.StatusOK, doGet(r), "request 2 at t=1.5s")

	// TTL must still be measured from the first request: at most 0.5s left.
	ttl := mr.TTL("rate_limit:window_test")
	assert.LessOrEqual(t, ttl, 500*time.Millisecond,
		"TTL was reset by the second request (got %v); the window must not slide", ttl)

	// t=2.1s: the first window has ended, so a fresh window must admit this request.
	mr.FastForward(600 * time.Millisecond)
	assert.Equal(t, http.StatusOK, doGet(r),
		"request at t=2.1s must start a new window, not be throttled by a slid TTL")
}

// TestRedisRateLimit_Unit_CountComesFromIncr asserts the count is the atomic
// INCR result: after N requests the stored counter equals N and remaining
// matches it.
func TestRedisRateLimit_Unit_CountComesFromIncr(t *testing.T) {
	mr, client := setupMiniredis(t)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 5
	cfg.Window = time.Minute
	r := fixedWindowRouter(cfg)
	for i := 1; i <= 3; i++ {
		require.Equal(t, http.StatusOK, doGet(r))
	}
	v, err := mr.Get("rate_limit:window_test")
	require.NoError(t, err)
	assert.Equal(t, "3", v)
	assert.Greater(t, mr.TTL("rate_limit:window_test"), time.Duration(0), "key must carry a TTL")
}

// TestSlidingWindowRedisRateLimit_Unit_FailsClosed asserts the sliding-window
// limiter rejects with 503 (like RedisRateLimit) when Redis is unreachable,
// instead of letting the request through.
func TestSlidingWindowRedisRateLimit_Unit_FailsClosed(t *testing.T) {
	mr, client := setupMiniredis(t)
	mr.Close() // Redis now unreachable

	gin.SetMode(gin.TestMode)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.KeyGenerator = func(c *gin.Context) string { return "sw_down" }
	handled := 0
	r := gin.New()
	r.Use(SlidingWindowRedisRateLimit(cfg))
	r.GET("/t", func(c *gin.Context) { handled++; c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/t", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "Rate limiting service temporarily unavailable")
	assert.Equal(t, 0, handled, "handler must not run when Redis is down")
}

// TestRedisRateLimit_Unit_RepairsKeyWithoutTTL asserts that a counter left
// without an expiry (e.g. written by an older build) gets one, so it cannot
// throttle a client forever.
func TestRedisRateLimit_Unit_RepairsKeyWithoutTTL(t *testing.T) {
	mr, client := setupMiniredis(t)
	require.NoError(t, mr.Set("rate_limit:window_test", "7")) // no TTL
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 100
	cfg.Window = time.Minute
	r := fixedWindowRouter(cfg)
	require.Equal(t, http.StatusOK, doGet(r))
	ttl := mr.TTL("rate_limit:window_test")
	assert.Greater(t, ttl, time.Duration(0), "a TTL-less counter must be given the window expiry")
	assert.LessOrEqual(t, ttl, time.Minute)
}

// TestRedisRateLimit_Unit_HeadersAndRetryAfter pins the response headers and
// the 429 retry_after against the real window state (unit-class, miniredis).
func TestRedisRateLimit_Unit_HeadersAndRetryAfter(t *testing.T) {
	mr, client := setupMiniredis(t)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 3
	cfg.Window = time.Minute
	r := fixedWindowRouter(cfg)

	get := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/t", nil))
		return w
	}

	// First request; then let 20s of the window elapse so ttl < Window.
	w := get()
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2", w.Header().Get("X-RateLimit-Remaining"))
	mr.FastForward(20 * time.Second)

	for _, want := range []string{"1", "0"} {
		w = get()
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, want, w.Header().Get("X-RateLimit-Remaining"),
			"remaining must decrement by exactly one per request")
	}

	// Reset must be now+ttl (ttl ~40s), not now+Window (60s).
	reset, err := time.Parse(time.RFC3339, w.Header().Get("X-RateLimit-Reset"))
	require.NoError(t, err)
	now := time.Now()
	ttl := mr.TTL("rate_limit:window_test")
	require.Greater(t, ttl, time.Duration(0))
	assert.False(t, reset.Before(now.Add(ttl-time.Second).Truncate(time.Second)),
		"reset %v earlier than now+ttl-1s (ttl %v)", reset, ttl)
	assert.False(t, reset.After(now.Add(ttl+time.Second)),
		"reset %v later than now+ttl+1s (ttl %v): must use remaining TTL, not Window", reset, ttl)
	assert.False(t, reset.After(now.Add(cfg.Window)), "reset beyond now+Window")

	// Fourth request exceeds the limit: retry_after is the remaining TTL.
	w = get()
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	var body struct {
		RetryAfter string `json:"retry_after"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	ra, err := time.ParseDuration(body.RetryAfter)
	require.NoError(t, err)
	assert.Greater(t, ra, time.Duration(0), "retry_after must be positive")
	assert.LessOrEqual(t, ra, cfg.Window, "retry_after must not exceed the window")
	assert.InDelta(t, float64(ttl), float64(ra), float64(time.Second),
		"retry_after must equal the remaining TTL")
}
