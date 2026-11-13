package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type lrig struct {
	r       *gin.Engine
	lim     *FailedAttemptLimiter
	clock   time.Time
	reached int
	mu      sync.Mutex
	limited []string
	block   chan struct{} // when non-nil the handler blocks on it
}

// newRig wires the production RegisterChangePassword. The stub auth sets user_id
// from X-U (X-U-Raw sets a non-int value) and session_id from X-S; the handler
// answers the status in X-Status (default 401) or panics when X-Panic is set.
func newRig(maxSession, maxUser int, window time.Duration) *lrig {
	gin.SetMode(gin.TestMode)
	g := &lrig{clock: time.Unix(1_700_000_000, 0)}
	g.lim = NewFailedAttemptLimiter(maxSession, maxUser, window)
	g.lim.now = func() time.Time { return g.clock }
	g.lim.OnSessionLimit = func(uid int, sid string) {
		g.mu.Lock()
		g.limited = append(g.limited, strconv.Itoa(uid)+"/"+sid)
		g.mu.Unlock()
	}
	g.r = gin.New()
	g.r.Use(gin.Recovery())
	pass := func(c *gin.Context) { c.Next() }
	auth := func(c *gin.Context) {
		if raw := c.GetHeader("X-U-Raw"); raw != "" {
			c.Set("user_id", raw)
		} else if u := c.GetHeader("X-U"); u != "" {
			n, _ := strconv.Atoi(u)
			c.Set("user_id", n)
		}
		if s := c.GetHeader("X-S"); s != "" {
			c.Set("session_id", s)
		}
		c.Next()
	}
	h := func(c *gin.Context) {
		g.mu.Lock()
		g.reached++
		g.mu.Unlock()
		if c.GetHeader("X-Panic") != "" {
			panic("boom")
		}
		if g.block != nil && c.GetHeader("X-Block") != "" {
			<-g.block
		}
		st := http.StatusUnauthorized
		if s := c.GetHeader("X-Status"); s != "" {
			st, _ = strconv.Atoi(s)
		}
		c.Status(st)
	}
	RegisterChangePassword(g.r.Group("/a"), "/cp", pass, auth, h, g.lim)
	return g
}

func (g *lrig) do(u, s string, extra ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/a/cp", nil)
	if u != "" {
		req.Header.Set("X-U", u)
	}
	if s != "" {
		req.Header.Set("X-S", s)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		req.Header.Set(extra[i], extra[i+1])
	}
	w := httptest.NewRecorder()
	g.r.ServeHTTP(w, req)
	return w
}

func (g *lrig) fails(u, s string, n int) {
	for i := 0; i < n; i++ {
		_ = g.do(u, s)
	}
}

func (g *lrig) adv(d time.Duration) { g.clock = g.clock.Add(d) }

func TestRetryAfterExactValues(t *testing.T) {
	g := newRig(3, 100, 10*time.Minute)
	g.fails("1", "a", 3)
	w := g.do("1", "a")
	require.Equal(t, 429, w.Code)
	require.Equal(t, "600", w.Header().Get("Retry-After"))
	g.adv(100 * time.Second)
	require.Equal(t, "500", g.do("1", "a").Header().Get("Retry-After"))
	g.adv(500*time.Second - time.Second)
	require.Equal(t, "1", g.do("1", "a").Header().Get("Retry-After"))
	g.adv(time.Second)
	require.Equal(t, 401, g.do("1", "a").Code, "window elapsed: handler reachable again")
}

// The lock lifts when the OLDEST failure that holds the budget expires, not the newest.
func TestRetryAfterUsesOldestHoldingFailure(t *testing.T) {
	g := newRig(3, 100, 10*time.Minute)
	g.fails("1", "a", 1)
	g.adv(2 * time.Minute)
	g.fails("1", "a", 1)
	g.adv(2 * time.Minute)
	g.fails("1", "a", 1)
	w := g.do("1", "a")
	require.Equal(t, 429, w.Code)
	require.Equal(t, "360", w.Header().Get("Retry-After")) // 600s - 240s (oldest), not 600
}

func TestFailuresAgeOutIndividually(t *testing.T) {
	g := newRig(3, 100, 10*time.Minute)
	g.fails("1", "a", 1)
	g.adv(6 * time.Minute)
	g.fails("1", "a", 2)
	require.Equal(t, 429, g.do("1", "a").Code)
	g.adv(5 * time.Minute) // first failure aged out, two remain
	require.Equal(t, 401, g.do("1", "a").Code)
}

func TestPruneBoundaryFailureExactlyWindowOldIsDropped(t *testing.T) {
	g := newRig(1, 100, 10*time.Minute)
	g.fails("1", "a", 1)
	require.Equal(t, 429, g.do("1", "a").Code)
	g.adv(10*time.Minute - time.Nanosecond)
	require.Equal(t, 429, g.do("1", "a").Code)
	g.adv(time.Nanosecond) // age == window exactly: no longer counts
	require.Equal(t, 401, g.do("1", "a").Code)
}

func TestOtherStatusesNotCounted(t *testing.T) {
	g := newRig(2, 100, time.Minute)
	for i := 0; i < 10; i++ {
		require.Equal(t, 400, g.do("1", "a", "X-Status", "400").Code)
		require.Equal(t, 404, g.do("1", "a", "X-Status", "404").Code)
		require.Equal(t, 403, g.do("1", "a", "X-Status", "403").Code)
		require.Equal(t, 500, g.do("1", "a", "X-Status", "500").Code)
	}
}

func TestHandlerPanicReleasesSlot(t *testing.T) {
	g := newRig(2, 100, time.Minute)
	require.Equal(t, 500, g.do("1", "a", "X-Panic", "1").Code)
	// If the slot leaked, the budget of 2 would be down to 1 for ever.
	require.Equal(t, 401, g.do("1", "a").Code)
	require.Equal(t, 401, g.do("1", "a").Code)
	require.Equal(t, 429, g.do("1", "a").Code)
}

// The attacker's stolen session is locked; the victim's own session of the same user is not.
func TestPerSessionIsolationAttackerLockedVictimNot(t *testing.T) {
	g := newRig(5, 25, 15*time.Minute)
	g.fails("1", "attacker", 5)
	require.Equal(t, 429, g.do("1", "attacker").Code)
	require.Equal(t, []string{"1/attacker"}, g.limited, "revoke hook fired once, for the attacker session")
	require.Equal(t, 200, g.do("1", "victim", "X-Status", "200").Code)
	require.Equal(t, 401, g.do("1", "victim").Code, "victim still has a fresh budget")
}

func TestPerSessionBoundaryExact(t *testing.T) {
	g := newRig(5, 25, 15*time.Minute)
	for i := 1; i <= 5; i++ {
		require.Equal(t, 401, g.do("1", "a").Code, "failure %d is answered", i)
	}
	require.Equal(t, 429, g.do("1", "a").Code)
	require.Len(t, g.limited, 1)
}

func TestUserCeilingAcrossSessions(t *testing.T) {
	g := newRig(5, 25, 15*time.Minute)
	for s := 0; s < 4; s++ { // 20 failures spread over 4 sessions
		g.fails("1", "s"+strconv.Itoa(s), 5)
	}
	require.Equal(t, 401, g.do("1", "s4").Code, "21st failure still answered")
	g.fails("1", "s4", 4) // total 25
	require.Equal(t, 429, g.do("1", "fresh").Code, "ceiling reached: a fresh session is refused")
	require.Equal(t, 200, g.do("2", "x", "X-Status", "200").Code, "another user unaffected")
}

func TestSuccessResetsOnlyThatUser(t *testing.T) {
	g := newRig(2, 25, time.Minute)
	g.fails("1", "a", 2) // alice locked
	require.Equal(t, 429, g.do("1", "a").Code)
	g.fails("2", "b", 1)
	require.Equal(t, 200, g.do("2", "b", "X-Status", "200").Code) // bob succeeds
	require.Equal(t, 429, g.do("1", "a").Code, "alice still locked")
	// bob's own counter was reset by his success: a fresh budget of 2
	require.Equal(t, 401, g.do("2", "b").Code)
	require.Equal(t, 401, g.do("2", "b").Code)
	require.Equal(t, 429, g.do("2", "b").Code)
}

func TestNonIntOrMissingUserFailsClosed(t *testing.T) {
	g := newRig(2, 25, time.Minute)
	require.Equal(t, 401, g.do("", "a", "X-U-Raw", "alice-uuid").Code)
	require.Equal(t, 401, g.do("", "a").Code) // no user_id at all
	require.Zero(t, g.reached, "handler (bcrypt) never reached")
}

func TestMissingSessionIdSharesOneBucketAndIsCounted(t *testing.T) {
	g := newRig(3, 25, time.Minute)
	g.fails("1", "", 3)
	require.Equal(t, 429, g.do("1", "").Code)
}

func TestInflightHeldBudgetRetryAfterIsOneSecond(t *testing.T) {
	g := newRig(1, 25, 15*time.Minute)
	g.block = make(chan struct{})
	done := make(chan struct{})
	go func() { g.do("1", "a", "X-Block", "1"); close(done) }()
	for { // wait until the first request is inside the handler
		g.mu.Lock()
		n := g.reached
		g.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	w := g.do("1", "a")
	require.Equal(t, 429, w.Code)
	require.Equal(t, "1", w.Header().Get("Retry-After"), "held by an in-flight attempt, not by a 15 min lock")
	close(g.block)
	<-done
}

func TestConcurrentGuessesStopAtSessionLimit(t *testing.T) {
	g := newRig(5, 25, 15*time.Minute)
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[int]int{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := g.do("1", "a").Code
			mu.Lock()
			counts[c]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	require.Equal(t, 5, counts[401])
	require.Equal(t, 95, counts[429])
}

func TestMapPrunedWhenWindowPasses(t *testing.T) {
	lim := NewFailedAttemptLimiter(3, 25, time.Minute)
	now := time.Unix(1, 0)
	lim.now = func() time.Time { return now }
	for uid := 0; uid < 50; uid++ {
		require.Zero(t, lim.acquire(uid, "s"))
		lim.release(uid, "s", true, false)
	}
	require.NotEmpty(t, lim.buckets)
	now = now.Add(2 * time.Minute)
	for uid := 0; uid < 50; uid++ {
		require.Zero(t, lim.acquire(uid, "s"))
		lim.release(uid, "s", false, false)
	}
	require.Empty(t, lim.buckets)
}

// A failure exactly window-old no longer counts: it must not add to the next one.
func TestPruneBoundaryDroppedFailureDoesNotCountTowardsNext(t *testing.T) {
	g := newRig(2, 100, 10*time.Minute)
	g.fails("1", "a", 1)
	g.adv(10 * time.Minute)
	require.Equal(t, 401, g.do("1", "a").Code) // counted: 1 (old one dropped)
	require.Equal(t, 401, g.do("1", "a").Code) // counted: 2
	require.Equal(t, 429, g.do("1", "a").Code)
}

// M1: failures spread over two sessions reach the user bucket's size of 5 but
// no single session's budget: nobody may be revoked.
func TestUserBucketReachingSessionSizeDoesNotRevokeAnySession(t *testing.T) {
	g := newRig(5, 25, 15*time.Minute)
	g.fails("1", "A", 3)
	g.fails("1", "B", 2)
	require.Empty(t, g.limited, "no session budget exhausted: no revoke")
	require.Equal(t, 401, g.do("1", "A").Code)
	require.Equal(t, 401, g.do("1", "B").Code)
}

// M2: a success resets the USER bucket too (fresh session admitted afterwards).
func TestSuccessResetsUserCeilingBucket(t *testing.T) {
	g := newRig(5, 6, 15*time.Minute)
	g.fails("1", "s1", 4)
	g.fails("1", "s2", 1) // user bucket at 5 of 6
	require.Equal(t, 200, g.do("1", "s3", "X-Status", "200").Code)
	require.Equal(t, 401, g.do("1", "s4").Code)
	require.Equal(t, 401, g.do("1", "s4").Code) // 2 of 6, not 6 of 6
	require.Equal(t, 401, g.do("1", "s4").Code)
}

// M5: Retry-After is the CEILING of the remaining time, never 0 while locked.
func TestRetryAfterRoundsUpFractionalSeconds(t *testing.T) {
	g := newRig(1, 25, 10*time.Minute)
	g.fails("1", "a", 1)
	g.adv(10*time.Minute - 100*time.Millisecond)
	w := g.do("1", "a")
	require.Equal(t, 429, w.Code)
	require.Equal(t, "1", w.Header().Get("Retry-After"))
	g.adv(-(10*time.Minute - 100*time.Millisecond) + 100*time.Second + 500*time.Millisecond)
	require.Equal(t, "500", g.do("1", "a").Header().Get("Retry-After")) // 499.5 s remaining -> 500
}

// Buckets of revoked/idle sessions are swept by a later acquire of ANOTHER user.
func TestIdleBucketsOfOtherUsersAreSwept(t *testing.T) {
	g := newRig(3, 25, time.Minute)
	for u := 1; u <= 50; u++ {
		g.fails(strconv.Itoa(u), "dead", 1)
	}
	require.Len(t, g.lim.buckets, 100)
	g.adv(2 * time.Minute)
	require.Equal(t, 401, g.do("999", "x").Code)
	require.Len(t, g.lim.buckets, 2, "only the live user's session+user buckets remain")
}

// R4 boundary probe: the 5th recorded failure lands exactly when the 1st is window old.
func TestReviewR4BoundaryNoRevokeWithFourInWindow(t *testing.T) {
	g := newRig(5, 25, time.Minute)
	g.fails("1", "A", 1)
	g.adv(time.Second)
	g.fails("1", "A", 3)
	g.adv(time.Minute - time.Second) // first failure now exactly window old
	require.Equal(t, 401, g.do("1", "A").Code)
	require.Empty(t, g.limited, "only 4 failures inside the window")
}

// Sweep must never drop a lockout still inside its window.
func TestReviewSweepKeepsActiveLockout(t *testing.T) {
	g := newRig(5, 25, time.Minute)
	g.lim.OnSessionLimit = nil
	g.fails("2", "z", 1) // lastSweep = t0
	g.adv(10 * time.Second)
	g.fails("1", "A", 5)
	g.adv(time.Minute - 5*time.Second)         // t0+65s: sweep due, A's failures 55s old
	require.Equal(t, 401, g.do("3", "q").Code) // triggers sweep
	require.Equal(t, 429, g.do("1", "A").Code, "lockout survives sweep")
}

// A failure that expires while its request is in flight must not count as the 5th.
func TestReviewExpiryDuringRequestDoesNotRevoke(t *testing.T) {
	g := newRig(5, 25, time.Minute)
	g.fails("1", "A", 1)
	g.adv(time.Second)
	g.fails("1", "A", 3)
	g.adv(time.Minute - time.Second - time.Nanosecond) // first failure 1ns short of expiry
	g.block = make(chan struct{})
	done := make(chan int)
	go func() { done <- g.do("1", "A", "X-Block", "1").Code }()
	for {
		g.lim.mu.Lock()
		b := g.lim.buckets["s:1:A"]
		inflight := b != nil && b.inflight == 1
		g.lim.mu.Unlock()
		if inflight {
			break
		}
		time.Sleep(time.Millisecond)
	}
	g.lim.mu.Lock()
	g.adv(2 * time.Nanosecond)
	g.lim.mu.Unlock()
	close(g.block)
	require.Equal(t, 401, <-done)
	require.Empty(t, g.limited, "only 4 failures inside the window at release time")
}

// Sweep is throttled to once per window: a bucket that expires between two
// sweeps stays until the next due one (performance-only behaviour, documented).
func TestSweepRunsAtMostOncePerWindow(t *testing.T) {
	g := newRig(5, 25, time.Minute)
	require.Equal(t, 401, g.do("1", "p").Code) // first acquire: sweep, lastSweep = t0
	g.adv(10 * time.Second)
	g.fails("5", "D", 1) // D fails at t0+10s, expires t0+70s
	g.adv(50 * time.Second)
	require.Equal(t, 401, g.do("1", "p").Code) // t0+60s: sweep due, D (50s old) kept
	g.adv(30 * time.Second)                    // t0+90s: D expired, next sweep due t0+120s
	require.Equal(t, 401, g.do("1", "p").Code) // throttled: no sweep
	require.Contains(t, g.lim.buckets, "s:5:D", "expired bucket stays until the next due sweep")
	g.adv(31 * time.Second) // t0+121s: due
	require.Equal(t, 401, g.do("1", "p").Code)
	require.NotContains(t, g.lim.buckets, "s:5:D")
}
