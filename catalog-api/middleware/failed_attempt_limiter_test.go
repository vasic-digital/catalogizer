package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newClockRoute wires the production RegisterChangePassword with a handler
// whose status the test controls and a limiter on a fake clock.
func newClockRoute(max int, window time.Duration, status *int) (*gin.Engine, *time.Time) {
	gin.SetMode(gin.TestMode)
	clock := time.Unix(1_700_000_000, 0)
	lim := NewFailedAttemptLimiter(max, window)
	lim.now = func() time.Time { return clock }
	r := gin.New()
	auth := func(c *gin.Context) { c.Set("user_id", 1); c.Next() }
	pass := func(c *gin.Context) { c.Next() }
	RegisterChangePassword(r.Group("/a"), "/cp", pass, auth, func(c *gin.Context) { c.Status(*status) }, lim)
	return r, &clock
}

func hit(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/a/cp", nil))
	return w
}

func TestFailedAttemptLimiter_WindowExpiryRestoresAccess(t *testing.T) {
	st := http.StatusUnauthorized
	r, clock := newClockRoute(3, 10*time.Minute, &st)
	for i := 0; i < 3; i++ {
		require.Equal(t, 401, hit(r).Code)
	}
	w := hit(r)
	require.Equal(t, 429, w.Code)
	require.NotEmpty(t, w.Header().Get("Retry-After"))
	*clock = clock.Add(10*time.Minute - time.Second)
	require.Equal(t, 429, hit(r).Code, "still inside the window")
	*clock = clock.Add(2 * time.Second)
	require.Equal(t, 401, hit(r).Code, "window elapsed: handler reachable again")
}

func TestFailedAttemptLimiter_FailuresAgeOutIndividually(t *testing.T) {
	st := http.StatusUnauthorized
	r, clock := newClockRoute(3, 10*time.Minute, &st)
	require.Equal(t, 401, hit(r).Code)
	*clock = clock.Add(6 * time.Minute)
	require.Equal(t, 401, hit(r).Code)
	require.Equal(t, 401, hit(r).Code)
	require.Equal(t, 429, hit(r).Code)
	*clock = clock.Add(5 * time.Minute) // first failure (t0) aged out, two remain
	require.Equal(t, 401, hit(r).Code)
}

func TestFailedAttemptLimiter_OtherStatusesNotCounted(t *testing.T) {
	st := http.StatusBadRequest
	r, _ := newClockRoute(2, time.Minute, &st)
	for i := 0; i < 10; i++ {
		require.Equal(t, 400, hit(r).Code)
	}
}

func TestFailedAttemptLimiter_MapPrunedWhenWindowPasses(t *testing.T) {
	st := http.StatusUnauthorized
	lim := NewFailedAttemptLimiter(3, time.Minute)
	now := time.Unix(1, 0)
	lim.now = func() time.Time { return now }
	for uid := 0; uid < 50; uid++ {
		lim.fail(uid)
	}
	_ = st
	now = now.Add(2 * time.Minute)
	for uid := 0; uid < 50; uid++ {
		require.Zero(t, lim.blockedFor(uid))
	}
	require.Empty(t, lim.failures)
}
