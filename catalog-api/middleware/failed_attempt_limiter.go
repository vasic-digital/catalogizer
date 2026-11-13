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
	// CredentialMaxFailures is the number of wrong-secret answers ONE SESSION of
	// one user may cause inside CredentialFailureWindow before that session is
	// refused (and revoked, see OnSessionLimit).
	CredentialMaxFailures = 5
	// CredentialMaxUserFailures bounds the failures summed over ALL sessions of
	// one user in the window. It is 5x the session budget: a legitimate user
	// with a few devices never gets near it (it is reached only when >=5
	// distinct sessions were each driven to their own limit, i.e. several stolen
	// tokens), while distributed guessing across many sessions stays bounded.
	CredentialMaxUserFailures = 25
	// CredentialFailureWindow is how long failures count.
	CredentialFailureWindow = 15 * time.Minute
)

type attemptBucket struct {
	failures []time.Time
	inflight int
}

// FailedAttemptLimiter counts FAILED secret verifications per (user id,
// session) and, as a ceiling, per user id. Keying on the session means a thief
// holding a stolen token exhausts only that token's own budget, never the
// budget of the owner's other sessions (so the owner can still change the
// password from his own session). Successful attempts reset only that user's
// counters. In-memory and per-process, like the login limiter.
type FailedAttemptLimiter struct {
	mu         sync.Mutex
	maxSession int
	maxUser    int
	window     time.Duration
	now        func() time.Time
	buckets    map[string]*attemptBucket
	lastSweep  time.Time

	// OnSessionLimit, when set, is called (outside the lock) the moment a
	// failure exhausts one session's budget, so the caller can revoke that
	// session. May be nil.
	OnSessionLimit func(userID int, sessionID string)
}

// NewFailedAttemptLimiter builds a limiter: maxSession failures per session,
// maxUser per user (maxUser should be >= maxSession), per window.
func NewFailedAttemptLimiter(maxSession, maxUser int, window time.Duration) *FailedAttemptLimiter {
	return &FailedAttemptLimiter{maxSession: maxSession, maxUser: maxUser, window: window, now: time.Now, buckets: map[string]*attemptBucket{}}
}

func userKey(uid int) string { return "u:" + strconv.Itoa(uid) }
func sessKey(uid int, sid string) string {
	return "s:" + strconv.Itoa(uid) + ":" + sid
}

// pruneFailures drops the failures that are a full window old or older.
func (l *FailedAttemptLimiter) pruneFailures(b *attemptBucket, now time.Time) []time.Time {
	kept := b.failures[:0]
	for _, t := range b.failures {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	b.failures = kept
	return kept
}

// bucket returns the pruned bucket for key (nil when empty). Caller holds mu.
func (l *FailedAttemptLimiter) bucket(key string, now time.Time) *attemptBucket {
	b := l.buckets[key]
	if b == nil {
		return nil
	}
	kept := l.pruneFailures(b, now)
	if len(kept) == 0 && b.inflight == 0 {
		delete(l.buckets, key)
		return nil
	}
	return b
}

// sweep drops every expired, idle bucket (revoked or abandoned sessions never
// call acquire again, so their buckets would otherwise stay for ever). It runs
// at most once per window, from acquire. Caller holds mu.
func (l *FailedAttemptLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key := range l.buckets {
		l.bucket(key, now) // prunes and deletes when empty
	}
}

// wait returns how long until bucket b admits one more attempt (0 = admits now).
// Budget held only by in-flight attempts is reported as 1 s: those attempts
// resolve within a request, not within the window.
func (l *FailedAttemptLimiter) wait(b *attemptBucket, max int, now time.Time) time.Duration {
	if b == nil || len(b.failures)+b.inflight < max {
		return 0
	}
	if len(b.failures) >= max {
		// the lock lifts when the failure that leaves max-1 behind expires
		return l.window - now.Sub(b.failures[len(b.failures)-max])
	}
	return time.Second
}

func (l *FailedAttemptLimiter) hold(key string) {
	b := l.buckets[key]
	if b == nil {
		b = &attemptBucket{}
		l.buckets[key] = b
	}
	b.inflight++
}

// acquire reserves one verification slot in the session and the user bucket.
// In-flight attempts count against both budgets, so a concurrent burst cannot
// pass the check together (check-then-act race). Returns 0 when granted.
func (l *FailedAttemptLimiter) acquire(uid int, sid string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	sk, uk := sessKey(uid, sid), userKey(uid)
	w := l.wait(l.bucket(sk, now), l.maxSession, now)
	if uw := l.wait(l.bucket(uk, now), l.maxUser, now); uw > w {
		w = uw
	}
	if w > 0 {
		return w
	}
	l.hold(sk)
	l.hold(uk)
	return 0
}

func (l *FailedAttemptLimiter) release(uid int, sid string, failed, ok bool) {
	l.mu.Lock()
	now := l.now()
	hitSession := false
	for _, key := range []string{sessKey(uid, sid), userKey(uid)} {
		b := l.buckets[key]
		if b == nil {
			continue
		}
		if b.inflight--; b.inflight < 0 {
			b.inflight = 0
		}
		// failures that expired while the request was in flight must not count
		l.pruneFailures(b, now)
		switch {
		case failed:
			b.failures = append(b.failures, now)
			if key[0] == 's' && len(b.failures) == l.maxSession {
				hitSession = true
			}
		case ok:
			b.failures = nil
		}
		if len(b.failures) == 0 && b.inflight == 0 {
			delete(l.buckets, key)
		}
	}
	cb := l.OnSessionLimit
	l.mu.Unlock()
	if hitSession && cb != nil {
		cb(uid, sid)
	}
}

// Middleware must run AFTER the authentication middleware (it reads "user_id"
// and "session_id"). A 401 answer from the downstream handler counts as a
// failed verification, a 2xx answer resets the counters of that user; every
// other status leaves them unchanged. A refused caller gets 429 without the
// handler (so without a bcrypt compare). It fails closed: a user_id that is
// not an int (or missing) never reaches the handler, and a missing session id
// is counted under the shared "no session" session key.
func (l *FailedAttemptLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, _ := c.Get("user_id")
		uid, isInt := v.(int)
		if !isInt {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		sid, _ := c.Get("session_id")
		sidStr, _ := sid.(string)
		if wait := l.acquire(uid, sidStr); wait > 0 {
			secs := int((wait + time.Second - 1) / time.Second)
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
				l.release(uid, sidStr, false, false)
			}
		}()
		c.Next()
		st := c.Writer.Status()
		released = true
		l.release(uid, sidStr, st == http.StatusUnauthorized, st >= 200 && st < 300)
	}
}

// RegisterChangePassword wires POST path with the full chain: the generic rate
// limiter, authentication, the failed-attempt limiter, then the handler.
func RegisterChangePassword(rg gin.IRoutes, path string, rate, requireAuth, handler gin.HandlerFunc, lim *FailedAttemptLimiter) {
	rg.POST(path, rate, requireAuth, lim.Middleware(), handler)
}
