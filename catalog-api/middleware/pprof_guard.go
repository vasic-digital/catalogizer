package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// PprofSamplingGuard bounds every pprof request that holds the connection open for a sampling window (SECQ-3 / AZ2-5).
// Two kinds exist in net/http/pprof: /profile (CPU) and /trace always sample; and every named profile (allocs, block,
// goroutine, heap, mutex, threadcreate) turns into a delta profile when ?seconds=N is present (snapshot, sleep N seconds,
// snapshot). Without the guard each such request may hold a connection for as long as the server's WriteTimeout allows
// (900 s by default) with no concurrency limit.
//
//   - seconds, when present on any pprof path, must be a positive INTEGER not above maxSeconds, otherwise 400. Fractions
//     such as 0.5 are refused (the stdlib parses an integer and silently falls back to its default otherwise, a hold the
//     caller never asked for);
//   - an absent seconds on /profile or /trace is rewritten to the stdlib default (30 and 1) limited to maxSeconds;
//   - only one sampling window runs at a time across all of these paths; a second one is refused with 429.
//
// Plain snapshots (no seconds) pass through untouched. The slot is per process, which is what the runtime profiler
// itself allows.
func PprofSamplingGuard(maxSeconds int) gin.HandlerFunc {
	if maxSeconds < 1 {
		maxSeconds = 1
	}
	slot := make(chan struct{}, 1)
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		q := c.Request.URL.Query()
		_, hasSeconds := q["seconds"]
		def := 0
		switch {
		case strings.HasSuffix(p, "/profile"):
			def = 30
		case strings.HasSuffix(p, "/trace"):
			def = 1
		case !hasSeconds:
			c.Next() // a plain snapshot
			return
		}
		raw := q.Get("seconds")
		secs := def
		if hasSeconds {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > maxSeconds {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "seconds must be an integer between 1 and " + strconv.Itoa(maxSeconds),
				})
				return
			}
			secs = n
		}
		if secs > maxSeconds {
			secs = maxSeconds
		}
		select {
		case slot <- struct{}{}:
			defer func() { <-slot }()
		default:
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "another profile or trace is already running"})
			return
		}
		q.Set("seconds", strconv.Itoa(secs))
		c.Request.URL.RawQuery = q.Encode()
		c.Next()
	}
}
