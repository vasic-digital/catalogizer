package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// RLW-FU2-F2. Unit-class (miniredis double); the handler under test is production code.
// The sliding-window key lives 2x the window, so the body's retry_after (the raw key TTL)
// must be capped at the window exactly like the Retry-After header, and the two must agree.
func TestSlidingWindowRedisRateLimit_BodyRetryAfterMatchesHeaderCap(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := DefaultRedisRateLimiterConfig(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	cfg.Requests, cfg.Window = 2, 10*time.Second
	r := raEngine(SlidingWindowRedisRateLimit(cfg))
	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusOK, raGet(r).Code)
	}
	w := raGet(r)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	var body struct {
		RetryAfter string `json:"retry_after"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	d, err := time.ParseDuration(body.RetryAfter)
	require.NoError(t, err, "retry_after %q", body.RetryAfter)
	require.LessOrEqual(t, d, cfg.Window, "body retry_after %v exceeds the window (raw 2x key TTL)", d)
	hdr, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.Equal(t, retryAfterSeconds(d), strconv.Itoa(hdr), "body %v and header %d disagree", d, hdr)
	require.Equal(t, 10, hdr)
}

// RLW-FU2-F4. Retry-After and the X-RateLimit-* headers are invisible to a cross-origin browser
// client unless listed in Access-Control-Expose-Headers. Real CORS() handler through gin.
func TestCORS_ExposesRateLimitHeadersToAllowedOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://app.example")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	for _, method := range []string{"GET", "OPTIONS"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/x", nil)
		req.Header.Set("Origin", "http://app.example")
		r.ServeHTTP(w, req)
		exposed := map[string]bool{}
		for _, h := range splitCSV(w.Header().Get("Access-Control-Expose-Headers")) {
			exposed[h] = true
		}
		for _, want := range []string{"Retry-After", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
			require.True(t, exposed[want], "%s: %q not in Access-Control-Expose-Headers %q", method, want, w.Header().Get("Access-Control-Expose-Headers"))
		}
	}
}

// TIERTEST-N1. X-RateLimit-Reset of the token bucket must follow the injected clock, not wall time.
func TestTokenBucketRedisRateLimit_ResetHeaderFollowsInjectedClock(t *testing.T) {
	mr := miniredis.RunT(t)
	clock := time.Date(2031, 5, 6, 7, 8, 9, 0, time.UTC)
	h := tokenBucketRedisRateLimit(redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		func(*gin.Context) string { return "k" }, 3, 1, 30*time.Second, func() time.Time { return clock })
	w := raGet(raEngine(h))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, clock.Add(30*time.Second).Format(time.RFC3339), w.Header().Get("X-RateLimit-Reset"))
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
