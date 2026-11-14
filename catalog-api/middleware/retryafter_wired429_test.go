package middleware

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

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
	// TIERTEST-RLW-F2: bracket the elapsed time instead of assuming the host never stalls.
	// The refused request sees 60 s minus the time elapsed since the token was spent, and
	// that elapsed time lies in [0, t1-t0].
	t0 := time.Now()
	w := raGet(r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Header().Get("Retry-After"))
	w = raGet(r)
	t1 := time.Now()
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	n, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err, "Retry-After must be integer seconds")
	require.GreaterOrEqual(t, n, int(math.Ceil(60-t1.Sub(t0).Seconds())))
	require.LessOrEqual(t, n, 60)
	require.GreaterOrEqual(t, n, 1)
}

// Token bucket, 60/minute (1 token/s): empty bucket -> next token <= 1 s away -> "1".
func TestRateLimiter_429RetryAfterTracksRefillRate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RateLimiter(60), func(c *gin.Context) { c.Status(http.StatusOK) })
	// TIERTEST-RLW-F1: the bucket refills 1 token/s, so a stall of >1 s inside a fixed
	// 60-request loop could leave a token and make request 61 pass. Loop (bounded) until
	// the first 429 instead; whatever was refilled, the wait for the next whole token is
	// in (0, 1] s, so Retry-After is exactly "1".
	var w *httptest.ResponseRecorder
	for i := 0; i < 10000; i++ {
		w = raGet(r)
		if w.Code == http.StatusTooManyRequests {
			break
		}
		require.Equal(t, http.StatusOK, w.Code)
	}
	require.Equal(t, http.StatusTooManyRequests, w.Code, "bucket never emptied")
	require.Equal(t, "1", w.Header().Get("Retry-After"))
}

// Busy guard: a sampling window of 10 s holds the slot; a second request is refused 429
// with Retry-After = remaining seconds of the running window (9..10), not a constant.
func TestPprofSamplingGuard_Busy429HasRetryAfterRemaining(t *testing.T) {
	gin.SetMode(gin.TestMode)
	started, release := make(chan struct{}), make(chan struct{})
	t0 := time.Now()
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
	// TIERTEST-RLW-F2: the window end was recorded after t0 and before this request, so the
	// remaining time is in [10-(t1-t0), 10] whatever stalls the host suffers.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/profile?seconds=10", nil))
	t1 := time.Now()
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	n, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err, "Retry-After must be integer seconds, got %q", w.Header().Get("Retry-After"))
	require.GreaterOrEqual(t, n, int(math.Ceil(10-t1.Sub(t0).Seconds())))
	require.LessOrEqual(t, n, 10)
	require.GreaterOrEqual(t, n, 1)
	close(release)
	<-done
}
