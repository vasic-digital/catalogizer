package middleware

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Breaker tests for F1 (error classes), F2 (single half-open probe), F3 (tier
// budgets) and F4 (injectable clock). UNIT-class where miniredis or the
// scripted RESP server is used (§11.4.289); the transport cases use real
// sockets (closed port, black hole, accept-and-close).

// fakeClock is the injectable breaker clock (F4).
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }
func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}
func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

// atomicFallback is a race-clean in-memory stand-in that answers 200 and counts calls.
type atomicFallback struct{ n atomic.Int64 }

func (a *atomicFallback) handler() gin.HandlerFunc {
	return func(c *gin.Context) { a.n.Add(1); c.Next() }
}

// respServer is a real TCP server speaking just enough RESP to answer every
// command with a fixed error line (reply errors produced by real go-redis
// parsing), or to close the connection immediately (EOF), or never to answer.
type respServer struct {
	ln       net.Listener
	accepted atomic.Int64
}

func startRESP(t *testing.T, mode string) *respServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &respServer{ln: ln}
	var conns sync.Map
	t.Cleanup(func() {
		_ = ln.Close()
		conns.Range(func(k, _ any) bool { _ = k.(net.Conn).Close(); return true })
	})
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			s.accepted.Add(1)
			conns.Store(c, struct{}{})
			go func() {
				switch mode {
				case "close":
					_ = c.Close()
				case "blackhole":
					_, _ = io.Copy(io.Discard, c)
				default: // error line, e.g. "LOADING Redis is loading"
					br := bufio.NewReader(c)
					for {
						line, err := br.ReadString('\n')
						if err != nil {
							return
						}
						if !strings.HasPrefix(line, "*") {
							continue
						}
						n, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
						for i := 0; i < n; i++ {
							l, err := br.ReadString('\n') // $len
							if err != nil {
								return
							}
							sz, _ := strconv.Atoi(strings.TrimSpace(l[1:]))
							if _, err := io.CopyN(io.Discard, br, int64(sz)+2); err != nil {
								return
							}
						}
						if _, err := fmt.Fprintf(c, "-%s\r\n", mode); err != nil {
							return
						}
					}
				}
			}()
		}
	}()
	return s
}

func (s *respServer) client() *redis.Client {
	return redis.NewClient(&redis.Options{Addr: s.ln.Addr().String(), MaxRetries: -1, DisableIdentity: true})
}

type algo struct {
	name string
	mk   func(RedisRateLimiterConfig) (gin.HandlerFunc, *outageState)
	key  string
}

var algos = []algo{
	{"fixed", redisRateLimitWithState, "rate_limit:10.1.2.3"},
	{"sliding", slidingWindowWithState, "sliding_rate_limit:10.1.2.3"},
}

func newBreakerRouter(a algo, cfg RedisRateLimiterConfig, fb gin.HandlerFunc) (*gin.Engine, *outageState) {
	gin.SetMode(gin.TestMode)
	cfg.Fallback = fb
	h, st := a.mk(cfg)
	r := gin.New()
	r.GET("/x", h, func(c *gin.Context) { c.Status(http.StatusOK) })
	return r, st
}

// F1: EVERY member of the error class table, for BOTH algorithms. The row
// states whether the breaker may open; in every row the failing request is
// still answered by the fallback (200), as before the breaker existed.
func TestBreaker_ErrorClassTable(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel2()

	type row struct {
		name     string
		client   func(t *testing.T, a algo) *redis.Client
		ctx      context.Context
		wantTrip bool
	}
	respRow := func(line string) func(*testing.T, algo) *redis.Client {
		return func(t *testing.T, _ algo) *redis.Client { return startRESP(t, line).client() }
	}
	rows := []row{
		{"request ctx cancelled (miniredis healthy)", func(t *testing.T, _ algo) *redis.Client { _, c := setupMiniredis(t); return c }, cancelled, false},
		{"request ctx deadline already passed (miniredis healthy)", func(t *testing.T, _ algo) *redis.Client { _, c := setupMiniredis(t); return c }, expired, false},
		{"reply error WRONGTYPE (miniredis, poisoned key)", func(t *testing.T, a algo) *redis.Client {
			mr, c := setupMiniredis(t)
			if a.name == "fixed" {
				_, err := mr.Push(a.key, "x") // list under a counter key
				require.NoError(t, err)
			} else {
				require.NoError(t, mr.Set(a.key, "x")) // string under a zset key
			}
			return c
		}, context.Background(), false},
		{"reply error NOSCRIPT/ERR (scripted RESP server)", respRow("NOSCRIPT No matching script"), context.Background(), false},
		{"reply error ERR generic (scripted RESP server)", respRow("ERR something else"), context.Background(), false},
		{"reply error NOAUTH (scripted RESP server)", respRow("NOAUTH Authentication required"), context.Background(), false},
		{"reply error LOADING (server up, not serving)", respRow("LOADING Redis is loading the dataset in memory"), context.Background(), true},
		{"reply error MASTERDOWN", respRow("MASTERDOWN Link with MASTER is down"), context.Background(), true},
		{"reply error CLUSTERDOWN", respRow("CLUSTERDOWN The cluster is down"), context.Background(), true},
		{"transport: connection refused (closed port)", func(t *testing.T, _ algo) *redis.Client {
			return redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
		}, context.Background(), true},
		{"transport: EOF (server accepts then closes)", func(t *testing.T, _ algo) *redis.Client { return startRESP(t, "close").client() }, context.Background(), true},
		{"transport: 500 ms Redis-call timeout (black hole)", func(t *testing.T, _ algo) *redis.Client { return startRESP(t, "blackhole").client() }, context.Background(), true},
		{"client closed (redis.ErrClosed)", func(t *testing.T, _ algo) *redis.Client {
			_, c := setupMiniredis(t)
			require.NoError(t, c.Close())
			return c
		}, context.Background(), true},
	}
	for _, a := range algos {
		for _, rw := range rows {
			a, rw := a, rw
			t.Run(a.name+"/"+rw.name, func(t *testing.T) {
				cfg := DefaultRedisRateLimiterConfig(rw.client(t, a))
				fb := &atomicFallback{}
				r, st := newBreakerRouter(a, cfg, fb.handler())
				code := getWithCtx(r, rw.ctx)
				assert.Equal(t, http.StatusOK, code, "the failing request is answered by the fallback")
				assert.EqualValues(t, 1, fb.n.Load(), "fallback consulted exactly once")
				assert.Equal(t, rw.wantTrip, st.tripped(), "breaker open?")
			})
		}
	}
}

// Classifier in isolation for the members that need no server.
func TestIsRedisOutage_Members(t *testing.T) {
	live := context.Background()
	done, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{"nil", live, nil, false},
		{"context.Canceled, ctx live", live, context.Canceled, false},
		{"wrapped Canceled", live, fmt.Errorf("x: %w", context.Canceled), false},
		{"DeadlineExceeded, ctx live (our 500 ms)", live, context.DeadlineExceeded, true},
		{"DeadlineExceeded, request ctx done", done, context.DeadlineExceeded, false},
		{"net.Error timeout", live, &net.OpError{Op: "read", Err: errors.New("i/o timeout")}, true},
		{"io.EOF", live, io.EOF, true},
		{"io.ErrUnexpectedEOF", live, io.ErrUnexpectedEOF, true},
		{"redis.ErrClosed", live, redis.ErrClosed, true},
		{"redis.ErrPoolTimeout", live, redis.ErrPoolTimeout, true},
		{"unexpected-reply shape error", live, errors.New("unexpected fixed-window script reply"), false},
		{"unclassified plain error", live, errors.New("boom"), false},
		{"transport error but request ctx done", done, io.EOF, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, isRedisOutage(c.ctx, c.err), c.name)
	}
}

// F2: after the backoff only ONE request probes Redis; the rest keep using
// the fallback. Real black-hole socket; the clock is faked, so the only wait
// is the single 500 ms probe timeout.
func TestBreaker_HalfOpenSingleProbe(t *testing.T) {
	for _, a := range algos {
		a := a
		t.Run(a.name, func(t *testing.T) {
			srv := startRESP(t, "blackhole")
			clk := newFakeClock()
			cfg := DefaultRedisRateLimiterConfig(srv.client())
			cfg.Clock = clk.Now
			cfg.OutageBackoff = 5 * time.Second
			fb := &atomicFallback{}
			r, st := newBreakerRouter(a, cfg, fb.handler())

			require.Equal(t, http.StatusOK, getWithCtx(r, context.Background())) // trips (500 ms)
			require.True(t, st.tripped())
			// open: no Redis traffic at all
			before := srv.accepted.Load()
			for i := 0; i < 3; i++ {
				require.Equal(t, http.StatusOK, getWithCtx(r, context.Background()))
			}
			assert.Equal(t, before, srv.accepted.Load(), "breaker open: Redis not asked")

			clk.Advance(6 * time.Second) // half-open
			const n = 20
			var wg sync.WaitGroup
			var ok atomic.Int64
			start := make(chan struct{})
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					if getWithCtx(r, context.Background()) == http.StatusOK {
						ok.Add(1)
					}
				}()
			}
			close(start)
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("concurrent half-open requests did not finish")
			}
			assert.EqualValues(t, n, ok.Load())
			assert.EqualValues(t, before+1, srv.accepted.Load(), "exactly one probe connection reached Redis while half-open")
			assert.True(t, st.tripped(), "failed probe re-opens the breaker")
			assert.False(t, st.probing, "probe slot released")
		})
	}
}

// F2: a successful probe closes the breaker; a probe that ends neutrally
// (client cancelled) releases the slot and leaves the breaker half-open so the
// next request probes.
func TestBreaker_ProbeOutcomes(t *testing.T) {
	for _, a := range algos {
		a := a
		t.Run(a.name, func(t *testing.T) {
			mr, client := setupMiniredis(t)
			clk := newFakeClock()
			cfg := DefaultRedisRateLimiterConfig(client)
			cfg.Requests = 1
			cfg.Clock = clk.Now
			cfg.OutageBackoff = 5 * time.Second
			r, st := newBreakerRouter(a, cfg, (&atomicFallback{}).handler())
			require.Equal(t, http.StatusOK, getWithCtx(r, context.Background()))
			mr.Close()
			require.Equal(t, http.StatusOK, getWithCtx(r, context.Background()))
			require.True(t, st.tripped())
			require.NoError(t, mr.Restart())

			// still inside backoff: Redis (healthy, budget used) NOT asked
			assert.Equal(t, http.StatusOK, getWithCtx(r, context.Background()))
			clk.Advance(6 * time.Second)

			cctx, cancel := context.WithCancel(context.Background())
			cancel()
			assert.Equal(t, http.StatusOK, getWithCtx(r, cctx), "cancelled probe served by fallback")
			assert.True(t, st.tripped(), "neutral probe leaves the breaker half-open")
			assert.False(t, st.probing, "neutral probe releases the slot")

			assert.Equal(t, http.StatusTooManyRequests, getWithCtx(r, context.Background()), "next request probes and Redis answers")
			assert.False(t, st.tripped(), "successful probe closes the breaker")
			assert.Equal(t, http.StatusTooManyRequests, getWithCtx(r, context.Background()))
		})
	}
}

// F3: each tier enforces ITS OWN Redis budget (auth 5/min, default 100/min).
// Separate miniredis per tier because both tiers use the same key prefix.
func TestRedisTierLimiters_EachTierEnforcesItsOwnBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(t *testing.T, pick func(auth, def gin.HandlerFunc) gin.HandlerFunc, budget int) {
		var mr *miniredis.Miniredis
		var client *redis.Client
		mr, client = setupMiniredis(t)
		_ = mr
		auth, def := NewRedisTierLimiters(client, nil, nil)
		r := gin.New()
		r.GET("/x", pick(auth, def), func(c *gin.Context) { c.Status(http.StatusOK) })
		for i := 1; i <= budget; i++ {
			require.Equal(t, http.StatusOK, fbGet(r), "request %d inside the budget", i)
		}
		assert.Equal(t, http.StatusTooManyRequests, fbGet(r), "request %d exceeds the budget", budget+1)
	}
	t.Run("auth tier = 5", func(t *testing.T) { run(t, func(a, _ gin.HandlerFunc) gin.HandlerFunc { return a }, 5) })
	t.Run("default tier = 100", func(t *testing.T) { run(t, func(_, d gin.HandlerFunc) gin.HandlerFunc { return d }, 100) })
}

var _ = httptest.NewRecorder
