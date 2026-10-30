package middleware

// Live-class test (§11.4.289): runs RedisRateLimit against a REAL Redis server
// (localhost:6379, DB 15) via tests.SetupTestRedis. When no Redis is reachable
// the test SKIPs with a reason; it never falls back to a double.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"catalogizer/internal/tests"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisRateLimit_LiveRedis_WindowExpiresFromFirstRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("live Redis test skipped in -short mode (needs real wall-clock window)")
	}
	// Probe first so the SKIP carries a closed reason code (§11.4.69).
	probe := redis.NewClient(&redis.Options{Addr: liveRedisAddr, DialTimeout: 2 * time.Second})
	pctx, pcancel := context.WithTimeout(context.Background(), 2*time.Second)
	perr := probe.Ping(pctx).Err()
	pcancel()
	_ = probe.Close()
	if perr != nil {
		t.Skipf("SKIP reason=hardware_not_present: no real Redis reachable at %s (%v); live-class run owed", liveRedisAddr, perr)
	}

	h := tests.SetupTestRedis(t)
	defer h.Close()

	// §11.4.289(D) identity of the server that answered.
	info, err := h.GetClient().Info(context.Background(), "server").Result()
	require.NoError(t, err)
	t.Logf("live redis identity: addr=%s redis_version=%s run_id=%s",
		liveRedisAddr, infoField(info, "redis_version"), infoField(info, "run_id"))
	require.NotEmpty(t, infoField(info, "run_id"), "INFO server must report a run_id")

	cfg := DefaultRedisRateLimiterConfig(h.GetClient())
	cfg.Requests = 2
	cfg.Window = 2 * time.Second
	r := fixedWindowRouter(cfg)
	const key = "rate_limit:window_test"

	start := time.Now()
	require.Equal(t, http.StatusOK, doGet(r), "request 1")
	time.Sleep(1200 * time.Millisecond)
	require.Equal(t, http.StatusOK, doGet(r), "request 2 inside the window")

	// PTTL, not TTL: TTL has whole-second resolution and rounds 0.8s up to 1s.
	ttl, err := h.GetClient().PTTL(context.Background(), key).Result()
	require.NoError(t, err)
	t.Logf("live redis: TTL after second request = %v (elapsed %v)", ttl, time.Since(start))
	assert.LessOrEqual(t, ttl, 900*time.Millisecond,
		"TTL was reset by the second request; the window must not slide")
	assert.Equal(t, http.StatusTooManyRequests, doGet(r), "request 3 inside the window exceeds the limit")

	// Wait until Window has passed since the FIRST request (plus margin).
	time.Sleep(time.Until(start.Add(cfg.Window + 300*time.Millisecond)))
	assert.False(t, h.Exists(key), "key must have expired Window after the first request")
	assert.Equal(t, http.StatusOK, doGet(r), "a new window must admit the request")
}

// liveRedisAddr matches tests.SetupTestRedis.
const liveRedisAddr = "localhost:6379"

// infoField returns one "key:value" field from a Redis INFO reply.
func infoField(info, key string) string {
	for _, line := range strings.Split(info, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+":"); ok {
			return v
		}
	}
	return ""
}
