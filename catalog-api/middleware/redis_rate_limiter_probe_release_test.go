package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Round-3 N1/N4: the half-open probe slot must be released BEFORE the
// downstream chain runs on the fallback path, and on a panic anywhere after
// admit(). UNIT-class (miniredis, fake clock, in-process hooks).

func probeRouter(a algo, cfg RedisRateLimiterConfig, down gin.HandlerFunc) (*gin.Engine, *outageState) {
	gin.SetMode(gin.TestMode)
	cfg.Fallback = func(c *gin.Context) { c.Next() } // the production shape: RateLimitByUser calls c.Next()
	h, st := a.mk(cfg)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/x", h, down)
	return r, st
}

func probeCfg(t *testing.T, clk *fakeClock, client *redis.Client) RedisRateLimiterConfig {
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Requests = 1
	cfg.Clock = clk.Now
	cfg.OutageBackoff = 5 * time.Second
	return cfg
}

func TestRV1_ProbeSlotLeakOnDownstreamPanic(t *testing.T) {
	for _, a := range algos {
		a := a
		t.Run(a.name, func(t *testing.T) {
			mr, client := setupMiniredis(t)
			clk := newFakeClock()
			var boom atomic.Bool
			r, st := probeRouter(a, probeCfg(t, clk, client), func(c *gin.Context) {
				if boom.Load() {
					panic("downstream boom")
				}
				c.Status(http.StatusOK)
			})
			require.Equal(t, http.StatusOK, getWithCtx(r, context.Background())) // uses the 1-request budget
			mr.Close()
			require.Equal(t, http.StatusOK, getWithCtx(r, context.Background())) // trips
			require.True(t, st.tripped())

			clk.Advance(6 * time.Second) // half-open; Redis still down so the probe fails
			boom.Store(true)
			assert.Equal(t, http.StatusInternalServerError, getWithCtx(r, context.Background()), "probe request's downstream panics (Recovery -> 500)")
			boom.Store(false)
			assert.False(t, st.probing, "probe slot must not leak when downstream panics")

			require.NoError(t, mr.Restart())
			clk.Advance(60 * time.Second)
			assert.Equal(t, http.StatusTooManyRequests, getWithCtx(r, context.Background()), "Redis takes over again: budget already used")
			assert.False(t, st.tripped(), "breaker closed")
		})
	}
}

func TestRV1_SlowDownstreamDoesNotHoldProbeSlot(t *testing.T) {
	for _, a := range algos {
		a := a
		t.Run(a.name, func(t *testing.T) {
			mr, client := setupMiniredis(t)
			clk := newFakeClock()
			entered := make(chan struct{})
			gate := make(chan struct{})
			var slow atomic.Bool
			r, st := probeRouter(a, probeCfg(t, clk, client), func(c *gin.Context) {
				if slow.CompareAndSwap(true, false) {
					close(entered)
					<-gate
				}
				c.Status(http.StatusOK)
			})
			require.Equal(t, http.StatusOK, getWithCtx(r, context.Background()))
			mr.Close()
			require.Equal(t, http.StatusOK, getWithCtx(r, context.Background()))
			require.True(t, st.tripped())
			clk.Advance(6 * time.Second)
			slow.Store(true)

			done := make(chan int, 1)
			go func() { done <- getWithCtx(r, context.Background()) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("slow downstream never entered")
			}
			// the failed probe's request is now executing downstream
			st.mu.Lock()
			held := st.probing
			st.mu.Unlock()
			assert.False(t, held, "slot must not be held while downstream runs")

			require.NoError(t, mr.Restart())
			clk.Advance(60 * time.Second)
			assert.Equal(t, http.StatusTooManyRequests, getWithCtx(r, context.Background()), "a new probe reaches Redis while the slow request still runs")
			assert.False(t, st.tripped(), "breaker closed while the slow request runs")
			close(gate)
			select {
			case code := <-done:
				assert.Equal(t, http.StatusOK, code)
			case <-time.After(5 * time.Second):
				t.Fatal("slow request did not finish")
			}
		})
	}
}

// panicHook makes every Redis command / pipeline panic while armed.
type panicHook struct{ armed atomic.Bool }

func (h *panicHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *panicHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.armed.Load() {
			panic("script call boom")
		}
		return next(ctx, cmd)
	}
}
func (h *panicHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if h.armed.Load() {
			panic("script call boom")
		}
		return next(ctx, cmds)
	}
}

// N4: the deferred release in the handlers must free the slot when something
// after admit() panics (KeyGenerator, or the Redis call itself).
func TestN4_PanicAfterAdmitReleasesProbeSlot(t *testing.T) {
	for _, a := range algos {
		for _, where := range []string{"keygen", "script"} {
			a, where := a, where
			t.Run(a.name+"/"+where, func(t *testing.T) {
				mr, client := setupMiniredis(t)
				hook := &panicHook{}
				client.AddHook(hook)
				clk := newFakeClock()
				cfg := probeCfg(t, clk, client)
				var kgPanic atomic.Bool
				cfg.KeyGenerator = func(c *gin.Context) string {
					if kgPanic.Load() {
						panic("keygen boom")
					}
					return "10.1.2.3"
				}
				r, st := probeRouter(a, cfg, func(c *gin.Context) { c.Status(http.StatusOK) })
				mr.Close()
				require.Equal(t, http.StatusOK, getWithCtx(r, context.Background())) // trips
				require.True(t, st.tripped())
				clk.Advance(6 * time.Second)
				if where == "keygen" {
					kgPanic.Store(true)
				} else {
					hook.armed.Store(true)
				}
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/x", nil)
				req.RemoteAddr = "10.1.2.3:4567"
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusInternalServerError, w.Code, "panic recovered by gin.Recovery")
				assert.False(t, st.probing, "probe slot released by the deferred release")
			})
		}
	}
}

// N2 evidence: against a black-hole server, WITHOUT ContextTimeoutEnabled the
// 500 ms handler context does not bound the call (ReadTimeout governs); WITH it
// the call fails at ~500 ms. Real socket, no fake.
func TestN2_ContextTimeoutEnabledBoundsBlackHoleCall(t *testing.T) {
	srv := startRESP(t, "blackhole")
	measure := func(enabled bool) time.Duration {
		cl := redis.NewClient(&redis.Options{Addr: srv.ln.Addr().String(), MaxRetries: -1, DisableIdentity: true,
			ReadTimeout: 1500 * time.Millisecond, ContextTimeoutEnabled: enabled})
		defer cl.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		t0 := time.Now()
		err := cl.Ping(ctx).Err()
		require.Error(t, err)
		return time.Since(t0)
	}
	off, on := measure(false), measure(true)
	t.Logf("ContextTimeoutEnabled=false: %v, true: %v", off, on)
	assert.Greater(t, off, 1200*time.Millisecond, "ReadTimeout governs when disabled")
	assert.Less(t, on, 900*time.Millisecond, "context deadline governs when enabled")
}
