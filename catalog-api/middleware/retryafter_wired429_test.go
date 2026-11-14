package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// RLW-FU2-F3. Unit-class: the production handlers, driven through gin with no doubles
// on the limiting path.

// Token bucket, 1 request/minute: after the single token is spent the next token is
// ~60 s away, so Retry-After is the whole-second ceiling of that wait (59..60).
func TestRateLimiter_429HasRetryAfterSeconds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RateLimiter(1), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := raGet(r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Header().Get("Retry-After"))
	w = raGet(r)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	n, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err, "Retry-After must be integer seconds")
	require.GreaterOrEqual(t, n, 58)
	require.LessOrEqual(t, n, 60)
}

// Token bucket, 60/minute (1 token/s): empty bucket -> next token <= 1 s away -> "1".
func TestRateLimiter_429RetryAfterTracksRefillRate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RateLimiter(60), func(c *gin.Context) { c.Status(http.StatusOK) })
	for i := 0; i < 60; i++ {
		require.Equal(t, http.StatusOK, raGet(r).Code)
	}
	w := raGet(r)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "1", w.Header().Get("Retry-After"))
}

// Busy guard: a sampling window of 10 s holds the slot; a second request is refused 429
// with Retry-After = remaining seconds of the running window (9..10), not a constant.
func TestPprofSamplingGuard_Busy429HasRetryAfterRemaining(t *testing.T) {
	gin.SetMode(gin.TestMode)
	started, release := make(chan struct{}), make(chan struct{})
	r := gin.New()
	r.GET("/debug/pprof/profile", PprofSamplingGuard(30), func(c *gin.Context) {
		close(started)
		<-release
		c.Status(http.StatusOK)
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest("GET", "/debug/pprof/profile?seconds=10", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
	}()
	<-started
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/profile?seconds=10", nil))
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	n, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err, "Retry-After must be integer seconds, got %q", w.Header().Get("Retry-After"))
	require.GreaterOrEqual(t, n, 9)
	require.LessOrEqual(t, n, 10)
	close(release)
	<-done
}
