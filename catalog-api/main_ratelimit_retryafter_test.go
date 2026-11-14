package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// RLW-FU2 (in-memory tier): the 429 of every real auth-tier limiter carries a
// Retry-After header in integer seconds (the 1m window = "60"); non-limited
// responses carry none. Real limiter + real router, as in
// TestAuthRoutes_RealLimiterTierEnforced429.
func TestAuthRoutes_429CarriesRetryAfterSeconds(t *testing.T) {
	for _, r := range rlRoutes {
		r := r
		t.Run(r.method+"_"+r.path, func(t *testing.T) {
			e := realRateLimitedEngine(t)
			for i := 1; i <= r.limit; i++ {
				w := rlHit(e, r.method, r.path, "192.0.2.20")
				require.NotEqual(t, http.StatusTooManyRequests, w.Code)
				require.Empty(t, w.Header().Get("Retry-After"), "request %d is not limited: no Retry-After", i)
			}
			w := rlHit(e, r.method, r.path, "192.0.2.20")
			require.Equal(t, http.StatusTooManyRequests, w.Code)
			require.Equal(t, "60", w.Header().Get("Retry-After"))
		})
	}
}
