package auth

import (
	"sync"
	"sync/atomic"
	"time"
)

// slidingWindowLimiter is the in-memory per-key sliding-window limiter behind RateLimitByUser.
type slidingWindowLimiter struct {
	requests int
	window   time.Duration
	entries  sync.Map // key -> *windowEntry

	// lastSweep is the UnixNano of the last idle-key sweep. At most one sweep per window runs, so
	// the retained keys are bounded by the clients seen in the last ~2 windows (S1: previously
	// every client ever seen was retained for the life of the process).
	lastSweep atomic.Int64
}

type windowEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
	dead       bool // evicted by a sweep; a caller that raced the eviction must re-create the entry
}

func newSlidingWindowLimiter(requests int, window time.Duration) *slidingWindowLimiter {
	return &slidingWindowLimiter{requests: requests, window: window}
}

// size returns the number of retained keys (test introspection).
func (l *slidingWindowLimiter) size() int {
	n := 0
	l.entries.Range(func(_, _ interface{}) bool { n++; return true })
	return n
}

// allow records a request for key at now and reports whether it is within the limit; the second
// result is the number of in-window requests seen before this one.
func (l *slidingWindowLimiter) allow(key string, now time.Time) (bool, int) {
	l.maybeSweep(now)
	for {
		val, _ := l.entries.LoadOrStore(key, &windowEntry{})
		e := val.(*windowEntry)
		e.mu.Lock()
		if e.dead {
			e.mu.Unlock()
			l.entries.CompareAndDelete(key, e)
			continue
		}
		defer e.mu.Unlock()

		cutoff := now.Add(-l.window)
		valid := e.timestamps[:0]
		for _, ts := range e.timestamps {
			if ts.After(cutoff) {
				valid = append(valid, ts)
			}
		}
		e.timestamps = valid
		if len(e.timestamps) >= l.requests {
			return false, len(e.timestamps)
		}
		e.timestamps = append(e.timestamps, now)
		return true, len(e.timestamps) - 1
	}
}

// maybeSweep evicts every key whose newest timestamp is outside the window, at most once per window.
func (l *slidingWindowLimiter) maybeSweep(now time.Time) {
	last := l.lastSweep.Load()
	if last == 0 {
		l.lastSweep.CompareAndSwap(0, now.UnixNano())
		return
	}
	if now.UnixNano()-last < int64(l.window) || !l.lastSweep.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	cutoff := now.Add(-l.window)
	l.entries.Range(func(k, v interface{}) bool {
		e := v.(*windowEntry)
		if !e.mu.TryLock() {
			return true // in use right now, therefore not idle
		}
		if n := len(e.timestamps); n == 0 || !e.timestamps[n-1].After(cutoff) {
			e.dead = true
			l.entries.CompareAndDelete(k, v)
		}
		e.mu.Unlock()
		return true
	})
}
