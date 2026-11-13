package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F1 (round-1 review, reproduce-first): a client cancelling its own request
// must not switch the distributed limiter off for everyone. UNIT-class
// (miniredis, §11.4.289); the public API only, so it also runs on the code
// before the fix.

func getWithCtx(r *gin.Engine, ctx context.Context) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil).WithContext(ctx)
	req.RemoteAddr = "10.1.2.3:4567"
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRedisRateLimit_ClientCancelDoesNotSwitchLimiterOff(t *testing.T) {
	_, client := setupMiniredis(t)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 1
	r := fbRouter(cfg, memLimit(1000))
	require.Equal(t, http.StatusOK, fbGet(r))              // uses the Redis budget
	require.Equal(t, http.StatusTooManyRequests, fbGet(r)) // control: Redis is authoritative

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Equal(t, http.StatusOK, getWithCtx(r, cctx), "the cancelled request itself is served by the fallback (unchanged per-request behaviour)")

	assert.Equal(t, http.StatusTooManyRequests, fbGet(r),
		"the next normal request must still be limited by Redis, not by the fallback")
}
