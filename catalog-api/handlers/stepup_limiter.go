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
	count int
	first time.Time // start of the window this count belongs to
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

// purge drops expired entries and, when the table is still full, the oldest ones. Caller holds mu.
func (l *stepUpLimiter) purge(now time.Time) {
	for id, f := range l.m {
		if now.Sub(f.first) >= stepUpWindow {
			delete(l.m, id)
		}
	}
	for len(l.m) >= stepUpMaxUsers {
		var oldest int
		var ot time.Time
		first := true
		for id, f := range l.m {
			if first || f.first.Before(ot) {
				oldest, ot, first = id, f.first, false
			}
		}
		delete(l.m, oldest)
	}
}

// blocked reports whether user may not try a step-up now, and for how long.
func (l *stepUpLimiter) blocked(user int) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f, ok := l.m[user]
	if !ok {
		return 0, false
	}
	if now.Sub(f.first) >= stepUpWindow {
		delete(l.m, user)
		return 0, false
	}
	if f.count >= stepUpMaxFailures {
		return stepUpWindow - now.Sub(f.first), true
	}
	return 0, false
}

// fail records one failed step-up of user.
func (l *stepUpLimiter) fail(user int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if f, ok := l.m[user]; ok && now.Sub(f.first) < stepUpWindow {
		f.count++
		return
	}
	l.purge(now)
	l.m[user] = &stepUpFailure{count: 1, first: now}
}

// ok clears the failures of user after a successful step-up.
func (l *stepUpLimiter) ok(user int) {
	l.mu.Lock()
	delete(l.m, user)
	l.mu.Unlock()
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
