package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Defaults for credential-verifying routes behind authentication (CAT-CHPW-RL-1).
const (
	// CredentialMaxFailures is the number of wrong-secret answers one user may
	// cause inside CredentialFailureWindow before the route refuses that user.
	CredentialMaxFailures = 5
	// CredentialFailureWindow is how long a user stays locked once the limit is reached.
	CredentialFailureWindow = 15 * time.Minute
)

// FailedAttemptLimiter counts FAILED secret verifications per authenticated
// user id and locks that user out of the guarded route once max failures were
// recorded inside the window. Successful attempts reset only that user's
// counter. It is keyed on the authenticated user id (never on a guessed
// username), so a caller can only ever lock the account its own token belongs
// to. In-memory and per-process, like the login limiter.
type FailedAttemptLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	now      func() time.Time
	failures map[int][]time.Time
	inflight map[int]int
}

// NewFailedAttemptLimiter builds a limiter allowing max failures per window.
func NewFailedAttemptLimiter(max int, window time.Duration) *FailedAttemptLimiter {
	return &FailedAttemptLimiter{max: max, window: window, now: time.Now, failures: map[int][]time.Time{}, inflight: map[int]int{}}
}

func (l *FailedAttemptLimiter) prune(uid int, now time.Time) []time.Time {
	kept := l.failures[uid][:0]
	for _, t := range l.failures[uid] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, uid)
		return nil
	}
	l.failures[uid] = kept
	return kept
}

// acquire reserves one verification slot for uid. In-flight attempts count
// against the budget, so a concurrent burst cannot pass the check together
// (check-then-act race): at most max attempts are ever evaluated per window.
// It returns 0 when the slot was granted, else how long uid stays refused.
func (l *FailedAttemptLimiter) acquire(uid int) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f := l.prune(uid, now)
	if len(f)+l.inflight[uid] >= l.max {
		if len(f) > 0 {
			return l.window - now.Sub(f[0])
		}
		return time.Second // only in-flight attempts hold the budget
	}
	l.inflight[uid]++
	return 0
}

// blockedFor reports the remaining lock time for uid without reserving a slot.
func (l *FailedAttemptLimiter) blockedFor(uid int) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f := l.prune(uid, now)
	if len(f) < l.max {
		return 0
	}
	return l.window - now.Sub(f[0])
}

func (l *FailedAttemptLimiter) release(uid int, failed, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inflight[uid]--; l.inflight[uid] <= 0 {
		delete(l.inflight, uid)
	}
	switch {
	case failed:
		l.failures[uid] = append(l.failures[uid], l.now())
	case ok:
		delete(l.failures, uid)
	}
}

func (l *FailedAttemptLimiter) fail(uid int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.prune(uid, now)
	l.failures[uid] = append(l.failures[uid], now)
}

// Middleware must run AFTER the authentication middleware (it reads "user_id").
// A 401 answer from the downstream handler counts as a failed verification, a
// 2xx answer resets the counter; every other status leaves it unchanged. A
// locked user gets 429 without the handler (and so without a bcrypt compare).
func (l *FailedAttemptLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get("user_id")
		uid, isInt := v.(int)
		if !ok || !isInt {
			c.Next() // handler rejects the missing identity itself
			return
		}
		if wait := l.acquire(uid); wait > 0 {
			secs := int(wait.Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(secs))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "too many failed attempts; try again later",
				"retry_after": secs,
			})
			return
		}
		released := false
		defer func() { // also runs when the handler panics: the slot is never leaked
			if !released {
				l.release(uid, false, false)
			}
		}()
		c.Next()
		st := c.Writer.Status()
		released = true
		l.release(uid, st == http.StatusUnauthorized, st >= 200 && st < 300)
	}
}

// RegisterChangePassword wires POST path with the full chain: the generic rate
// limiter, authentication, the failed-attempt limiter, then the handler. main
// and the tests use this one function so the wiring is the tested wiring.
func RegisterChangePassword(rg gin.IRoutes, path string, rate, requireAuth, handler gin.HandlerFunc, lim *FailedAttemptLimiter) {
	rg.POST(path, rate, requireAuth, lim.Middleware(), handler)
}
