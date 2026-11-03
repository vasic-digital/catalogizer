package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// S1: the limiter kept one entry per distinct client forever (sync.Map, no eviction), so a public
// endpoint's memory grew with the number of distinct source addresses ever seen.
func TestSlidingWindowLimiter_EvictsIdleKeys(t *testing.T) {
	l := newSlidingWindowLimiter(10, time.Minute)
	t0 := time.Unix(1_700_000_000, 0)
	for i := 0; i < 5000; i++ {
		ok, _ := l.allow(fmt.Sprintf("ratelimit:ip:10.%d.%d.%d", i/65536, (i/256)%256, i%256), t0)
		require.True(t, ok)
	}
	require.Equal(t, 5000, l.size(), "all keys are live inside the window")

	// two windows later every old key is idle; a single new request must trigger the sweep
	later := t0.Add(2*time.Minute + time.Second)
	ok, _ := l.allow("ratelimit:ip:fresh", later)
	require.True(t, ok)
	assert.LessOrEqual(t, l.size(), 2, "idle keys must be evicted (retained %d)", l.size())
}

func TestSlidingWindowLimiter_EvictionKeepsLiveKeysLimited(t *testing.T) {
	l := newSlidingWindowLimiter(3, time.Minute)
	t0 := time.Unix(1_700_000_000, 0)
	l.allow("seed", t0) // starts the sweep clock
	for i := 0; i < 3; i++ {
		ok, _ := l.allow("busy", t0.Add(50*time.Second+time.Duration(i)*time.Second))
		require.True(t, ok)
	}
	l.allow("other", t0.Add(61*time.Second)) // >= one window since t0: the sweep runs and evicts only idle "seed"
	assert.Equal(t, 2, l.size(), "busy (live) and other are retained, seed (idle) is evicted")
	ok, _ := l.allow("busy", t0.Add(62*time.Second))
	assert.False(t, ok, "a key still inside its window must stay limited after a sweep")
}

func TestRateLimitByUser_BoundedAcrossManyClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := NewAuthMiddleware(nil, zap.NewNop())
	router := gin.New()
	router.GET("/x", mw.RateLimitByUser(5, "50ms"), func(c *gin.Context) { c.Status(http.StatusOK) })
	hit := func(addr string) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = addr
		router.ServeHTTP(w, req)
	}
	for i := 0; i < 2000; i++ {
		hit(fmt.Sprintf("10.%d.%d.%d:1000", i/65536, (i/256)%256, i%256))
	}
	time.Sleep(120 * time.Millisecond)
	hit("192.0.2.9:1000")
	assert.LessOrEqual(t, mw.limiter.size(), 5, "retained keys after idle window: %d", mw.limiter.size())
}
