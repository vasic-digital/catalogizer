package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// PprofSamplingGuard bounds the two pprof handlers that hold a request open for a sampling window, /profile (CPU) and
// /trace (SECQ-3 / AZ2-5). Without it an administrator request may hold a connection for as long as the server's
// WriteTimeout allows (900 s by default), and several can run at once, each one a CPU profiler or tracer.
//
//   - seconds must be a positive integer not above maxSeconds, otherwise 400 (the stdlib would silently fall back to its
//     30 s default on a malformed value, which is a hold the caller never asked for);
//   - an absent seconds is rewritten to the stdlib default (30 for profile, 1 for trace) limited to maxSeconds;
//   - only one sampling window runs at a time, a second one is refused with 429 (the runtime allows a single CPU
//     profile anyway, and a trace excludes it).
//
// Snapshot endpoints (heap, goroutine, ...) pass through untouched.
func PprofSamplingGuard(maxSeconds int) gin.HandlerFunc {
	if maxSeconds < 1 {
		maxSeconds = 1
	}
	slot := make(chan struct{}, 1)
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		var def int
		switch {
		case strings.HasSuffix(p, "/profile"):
			def = 30
		case strings.HasSuffix(p, "/trace"):
			def = 1
		default:
			c.Next()
			return
		}
		q := c.Request.URL.Query()
		raw := q.Get("seconds")
		secs := def
		if raw != "" {
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
