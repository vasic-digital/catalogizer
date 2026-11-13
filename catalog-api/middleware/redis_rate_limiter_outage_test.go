package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catalogizer/internal/logging"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Follow-ups OPA-1..OPA-5 of CAT-OPDEC-A. UNIT-class where miniredis or an
// observer logger is involved (§11.4.289); the black-hole test uses a real
// TCP socket that accepts and never answers (no double of the limiter).

func withObserverLogger(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	prev := logging.Logger
	logging.Logger = zap.New(core)
	t.Cleanup(func() { logging.Logger = prev })
	return logs
}

func outageCount(logs *observer.ObservedLogs) int {
	n := 0
	for _, e := range logs.All() {
		if strings.Contains(e.Message, "using in-memory limiter fallback") {
			n++
		}
	}
	return n
}

func swRouter(cfg RedisRateLimiterConfig, mem gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg.Fallback = mem
	r := gin.New()
	r.GET("/x", SlidingWindowRedisRateLimit(cfg), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// OPA-1 (M2): the sliding-window limiter falls back too.
func TestSlidingWindowRedisRateLimit_OutageFallsBackToInMemory(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	r := swRouter(DefaultRedisRateLimiterConfig(client), memLimit(2))
	assert.Equal(t, http.StatusOK, fbGet(r), "outage must not return 503 when a fallback is configured")
	assert.Equal(t, http.StatusOK, fbGet(r))
	assert.Equal(t, http.StatusTooManyRequests, fbGet(r), "in-memory budget is enforced during the outage")
}

func TestSlidingWindowRedisRateLimit_OutageWithoutFallbackFailsClosed(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	r := swRouter(DefaultRedisRateLimiterConfig(client), nil)
	assert.Equal(t, http.StatusServiceUnavailable, fbGet(r))
}

// OPA-1 (M9): the main.go wiring is the exported helper; each tier gets ITS
// OWN fallback (auth tier -> auth in-memory, default tier -> default in-memory).
func TestRedisTierLimiters_WireTheirOwnFallbackPerTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	authMem := memLimit(1)      // auth tier: budget 1
	defMem := memLimit(1 << 20) // default tier: effectively unlimited
	auth, def := NewRedisTierLimiters(client, authMem, defMem)
	ra := gin.New()
	ra.GET("/x", auth, func(c *gin.Context) { c.Status(http.StatusOK) })
	rd := gin.New()
	rd.GET("/x", def, func(c *gin.Context) { c.Status(http.StatusOK) })
	assert.Equal(t, http.StatusOK, fbGet(ra))
	assert.Equal(t, http.StatusTooManyRequests, fbGet(ra), "auth tier answers from the auth fallback")
	assert.Equal(t, http.StatusOK, fbGet(rd))
	assert.Equal(t, http.StatusOK, fbGet(rd), "default tier answers from the default fallback")
}

// OPA-2 (M3/M4): the outage log line is throttled to one per interval.
func TestRedisRateLimit_OutageLogThrottledToOneLine(t *testing.T) {
	logs := withObserverLogger(t)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.OutageBackoff = -1 // breaker off so every request reaches useFallback and only the throttle limits the log
	r := fbRouter(cfg, memLimit(1000))
	for i := 0; i < 6; i++ {
		require.Equal(t, http.StatusOK, fbGet(r))
	}
	assert.Equal(t, 1, outageCount(logs), "6 outage requests inside the interval log exactly one line")
}

// OPA-5: the throttle state is per limiter, so a second tier is not silenced
// by the first tier's log line.
func TestRedisRateLimit_OutageThrottleIsPerLimiter(t *testing.T) {
	logs := withObserverLogger(t)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.OutageBackoff = -1
	r1 := fbRouter(cfg, memLimit(1000))
	r2 := fbRouter(cfg, memLimit(1000))
	require.Equal(t, http.StatusOK, fbGet(r1))
	require.Equal(t, http.StatusOK, fbGet(r2))
	assert.Equal(t, 2, outageCount(logs), "each limiter logs its own first outage line")
}

// OPA-3: against a Redis that accepts but never answers, only the first
// request pays the 500 ms timeout; the following ones are short-circuited.
func TestRedisRateLimit_BlackholeRedisDoesNotAddLatencyPerRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() })
		}
	}()
	client := redis.NewClient(&redis.Options{Addr: ln.Addr().String()})
	r := fbRouter(DefaultRedisRateLimiterConfig(client), memLimit(1000))

	t0 := time.Now()
	require.Equal(t, http.StatusOK, fbGet(r))
	first := time.Since(t0)
	assert.GreaterOrEqual(t, first, 400*time.Millisecond, "control: the first request really waits for the timeout")

	t1 := time.Now()
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusOK, fbGet(r))
	}
	assert.Less(t, time.Since(t1), 250*time.Millisecond, "5 further requests must not each wait 500 ms")
}

// Breaker closes again: after the backoff a healthy Redis is used again.
func TestRedisRateLimit_BreakerClosesAfterBackoff(t *testing.T) {
	mr, client := setupMiniredis(t)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 1
	cfg.OutageBackoff = 60 * time.Millisecond
	r := fbRouter(cfg, memLimit(1000))
	require.Equal(t, http.StatusOK, fbGet(r))
	mr.Close()
	require.Equal(t, http.StatusOK, fbGet(r), "falls back")
	require.NoError(t, mr.Restart())
	time.Sleep(120 * time.Millisecond)
	assert.Equal(t, http.StatusTooManyRequests, fbGet(r), "Redis (budget used) is authoritative again after the backoff")
}

// While the breaker is open the limiter does not ask Redis, for both
// algorithms: a recovered Redis (budget already used) is NOT consulted until
// the backoff has passed.
func TestBreakerStaysOpenWithinBackoff_BothAlgorithms(t *testing.T) {
	for name, mk := range map[string]func(RedisRateLimiterConfig) gin.HandlerFunc{
		"fixed":   RedisRateLimit,
		"sliding": SlidingWindowRedisRateLimit,
	} {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			mr, client := setupMiniredis(t)
			cfg := DefaultRedisRateLimiterConfig(client)
			cfg.Requests = 1
			cfg.Fallback = memLimit(1000)
			r := gin.New()
			r.GET("/x", mk(cfg), func(c *gin.Context) { c.Status(http.StatusOK) })
			require.Equal(t, http.StatusOK, fbGet(r))              // uses Redis budget
			require.Equal(t, http.StatusTooManyRequests, fbGet(r)) // control: Redis answers 429
			mr.Close()
			require.Equal(t, http.StatusOK, fbGet(r)) // outage: fallback, breaker opens
			require.NoError(t, mr.Restart())
			assert.Equal(t, http.StatusOK, fbGet(r), "Redis is back but the breaker is still open, fallback answers")
		})
	}
}

var _ = httptest.NewRecorder
