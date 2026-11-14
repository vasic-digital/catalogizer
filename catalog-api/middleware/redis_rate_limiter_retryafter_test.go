package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// RLW-FU2 (Redis tier). UNIT-class: miniredis is a test double (§11.4.289);
// the limiter code under test is the production handler.

func raEngine(h gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", h, func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func raGet(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = "10.9.8.7:1234"
	r.ServeHTTP(w, req)
	return w
}

func raCheck(t *testing.T, r *gin.Engine, limit int, min, max int) {
	t.Helper()
	for i := 1; i <= limit; i++ {
		w := raGet(r)
		require.Equal(t, http.StatusOK, w.Code)
		require.Empty(t, w.Header().Get("Retry-After"), "allowed request carries no Retry-After")
	}
	w := raGet(r)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	v := w.Header().Get("Retry-After")
	n, err := strconv.Atoi(v)
	require.NoError(t, err, "Retry-After must be integer seconds, got %q", v)
	require.GreaterOrEqual(t, n, min)
	require.LessOrEqual(t, n, max)
}

func raClient(t *testing.T) *redis.Client {
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRedisRateLimit_429HasRetryAfterSeconds(t *testing.T) {
	cfg := DefaultRedisRateLimiterConfig(raClient(t))
	cfg.Requests, cfg.Window = 3, time.Minute
	raCheck(t, raEngine(RedisRateLimit(cfg)), 3, 1, 60)
}

// RLW-FU2-F1: exact value. Fixed window of 1 minute; the window TTL starts at the first
// request, miniredis time is advanced 20 s, so exactly 40 s remain -> Retry-After "40".
// Kills a "configured window" (60) and a "half TTL" (20) mutation.
func TestRedisRateLimit_429RetryAfterIsExactRemainingTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := DefaultRedisRateLimiterConfig(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	cfg.Requests, cfg.Window = 3, time.Minute
	r := raEngine(RedisRateLimit(cfg))
	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, raGet(r).Code)
	}
	mr.FastForward(20 * time.Second)
	w := raGet(r)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "40", w.Header().Get("Retry-After"))
	mr.FastForward(30*time.Second + 500*time.Millisecond)
	w = raGet(r)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "10", w.Header().Get("Retry-After"), "9.5 s remaining rounds UP")
}

func TestSlidingWindowRedisRateLimit_429HasRetryAfterSeconds(t *testing.T) {
	cfg := DefaultRedisRateLimiterConfig(raClient(t))
	cfg.Requests, cfg.Window = 3, time.Minute
	raCheck(t, raEngine(SlidingWindowRedisRateLimit(cfg)), 3, 1, 60)
}

func TestTokenBucketRedisRateLimit_429HasRetryAfterSeconds(t *testing.T) {
	h := TokenBucketRedisRateLimit(raClient(t), func(*gin.Context) string { return "k" }, 3, 1, 30*time.Second)
	raCheck(t, raEngine(h), 3, 1, 30)
}

func TestRetryAfterSeconds_RoundsUpMinOne(t *testing.T) {
	require.Equal(t, "1", retryAfterSeconds(0))
	require.Equal(t, "1", retryAfterSeconds(-time.Second))
	require.Equal(t, "1", retryAfterSeconds(time.Millisecond))
	require.Equal(t, "2", retryAfterSeconds(1001*time.Millisecond))
	require.Equal(t, "60", retryAfterSeconds(time.Minute))
}
