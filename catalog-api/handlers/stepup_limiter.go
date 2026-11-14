package handlers

// CAT-SECQ1-C (review finding 5): the step-up check re-presents the acting user's password, which makes the two step-up routes an
// online password oracle for anyone holding a stolen session token. This limiter counts FAILED step-ups per USER (never per client
// address, so a spoofed X-Forwarded-For buys nothing). After stepUpMaxFailures failures inside stepUpWindow the user gets 429 with a
// Retry-After until the window of the first failure ends; a success clears the count. The state is bounded: expired entries are dropped
// on every call and the table never holds more than stepUpMaxUsers users (the oldest are evicted first).

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	stepUpMaxFailures = 5
	stepUpWindow      = 15 * time.Minute
	stepUpMaxUsers    = 10000

	codeStepUpThrottled = "step_up_throttled"
)

type stepUpFailure struct {
	count    int       // failed attempts in the current window
	inflight int       // attempts that hold a slot and have not finished (their password check is running)
	first    time.Time // start of the window this entry belongs to
}

type stepUpLimiter struct {
	mu  sync.Mutex
	now func() time.Time
	m   map[int]*stepUpFailure
}

func newStepUpLimiter() *stepUpLimiter {
	return &stepUpLimiter{now: time.Now, m: make(map[int]*stepUpFailure)}
}

// stepUpThrottle is the process-wide limiter shared by every step-up route.
var stepUpThrottle = newStepUpLimiter()

// purge drops expired entries that have no attempt in flight and, when the table is still full, the oldest ones that have none either.
// Caller holds mu.
func (l *stepUpLimiter) purge(now time.Time) {
	for id, f := range l.m {
		if f.inflight == 0 && now.Sub(f.first) >= stepUpWindow {
			delete(l.m, id)
		}
	}
	for len(l.m) >= stepUpMaxUsers {
		var oldest int
		var ot time.Time
		found := false
		for id, f := range l.m {
			if f.inflight == 0 && (!found || f.first.Before(ot)) {
				oldest, ot, found = id, f.first, true
			}
		}
		if !found {
			return
		}
		delete(l.m, oldest)
	}
}

// acquire reserves one step-up attempt of user BEFORE the password is checked. The reservation counts failures and attempts that are still
// running together, so N concurrent requests cannot all pass the limit and all run the password check (the check is slow and not atomic
// with the failure count). When it returns ok the caller MUST call release exactly once. When it does not, wait is the time to advertise.
func (l *stepUpLimiter) acquire(user int) (wait time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f, have := l.m[user]
	if !have {
		l.purge(now)
		f = &stepUpFailure{first: now}
		l.m[user] = f
	} else if now.Sub(f.first) >= stepUpWindow {
		f.count, f.first = 0, now // the window ended; attempts still in flight are carried over
	}
	if f.count >= stepUpMaxFailures {
		return stepUpWindow - now.Sub(f.first), false
	}
	if f.count+f.inflight >= stepUpMaxFailures {
		return time.Second, false // the remaining attempts are all running; try again when they have finished
	}
	f.inflight++
	return 0, true
}

// release ends an attempt that acquire admitted. A success clears the failures of the user; a failure counts against the window.
func (l *stepUpLimiter) release(user int, success bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, have := l.m[user]
	if !have {
		return
	}
	if f.inflight > 0 {
		f.inflight--
	}
	if success {
		f.count = 0
		if f.inflight == 0 {
			delete(l.m, user)
		} else {
			f.first = l.now()
		}
		return
	}
	f.count++
}

func (l *stepUpLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.m)
}

// respondStepUpThrottled answers 429 with Retry-After (whole seconds, at least 1).
func respondStepUpThrottled(c *gin.Context, wait time.Duration, syncShape bool) {
	secs := int(wait.Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	body := gin.H{"error": "too many failed step-up attempts; try again later", "code": codeStepUpThrottled}
	if syncShape {
		body["success"] = false
	}
	c.JSON(http.StatusTooManyRequests, body)
}

// stepUpAttempt runs one throttled step-up: it reserves the attempt (429 with Retry-After when the user has no attempts left), runs check,
// and records the outcome (a panic in check counts as a failure). It returns true only for a verified step-up; otherwise the response has
// been written (429 throttled, or 403 step_up_failed).
func stepUpAttempt(c *gin.Context, user int, syncShape bool, check func() bool) bool {
	wait, ok := stepUpThrottle.acquire(user)
	if !ok {
		respondStepUpThrottled(c, wait, syncShape)
		return false
	}
	verified := func() (v bool) {
		defer func() { stepUpThrottle.release(user, v) }()
		return check()
	}()
	if !verified {
		body := gin.H{"error": "step-up authentication failed", "code": codeStepUpFailed}
		if syncShape {
			body["success"] = false
		}
		c.JSON(http.StatusForbidden, body)
		return false
	}
	return true
}
