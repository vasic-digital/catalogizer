package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// PKGC-FU4-F1 / CHPW-RL-1-F1 / CHPW-RL-1-F2 (behavioural tier). Unit-class (no
// listener, no external service) but NOTHING is faked on the limiting path: the
// engine is the one buildRouter returns (the function main() uses), the limiters
// are the real ones newAuthRateTiers builds (the function main() calls), the JWT
// middleware and the engine middleware chain are production code. Only the
// non-limiter handlers are inert. The test sends limit+1 requests from one client
// address to each /api/v1/auth route and asserts that exactly the (limit+1)th is
// answered 429, i.e. that the route is behind the intended tier, runs the limiter
// BEFORE auth and handler, and shares the bucket with the routes of its tier.

type rlRoute struct {
	method, path string
	limit        int
	tier         string
}

var rlRoutes = []rlRoute{
	{"POST", "login", 30, "login"},
	{"POST", "register", 30, "login"},
	{"POST", "refresh", 600, "auth"},
	{"POST", "logout", 600, "auth"},
	{"GET", "me", 600, "auth"},
	{"GET", "status", 600, "auth"},
	{"GET", "permissions", 600, "auth"},
	{"GET", "profile", 600, "auth"},
	{"GET", "init-status", 2000, "default"},
	{"POST", "change-password", 2000, "default"},
}

func realRateLimitedEngine(t *testing.T) *gin.Engine {
	t.Helper()
	am := auth.NewAuthMiddleware(nil, zap.NewNop()) // fresh limiters per engine
	// newRouterDeps is the function main() calls to fill the limiter fields (RLW-FU1).
	return buildRouter(newRouterDeps(routerTestDeps(), am, nil, false))
}

func rlHit(e *gin.Engine, method, path, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/auth/"+path, nil)
	req.RemoteAddr = ip + ":4000"
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

// n requests, none may be 429 (the limiter lets the first `limit` through; the
// protected routes then answer 401/other from the inert or auth handlers).
func rlFill(t *testing.T, e *gin.Engine, r rlRoute, n int, ip string) {
	t.Helper()
	for i := 1; i <= n; i++ {
		w := rlHit(e, r.method, r.path, ip)
		require.NotEqual(t, http.StatusTooManyRequests, w.Code, "%s %s request %d/%d must not be limited", r.method, r.path, i, r.limit)
	}
}

func TestAuthRoutes_RealLimiterTierEnforced429(t *testing.T) {
	for _, r := range rlRoutes {
		r := r
		t.Run(r.method+"_"+r.path, func(t *testing.T) {
			e := realRateLimitedEngine(t)
			rlFill(t, e, r, r.limit, "192.0.2.10")
			w := rlHit(e, r.method, r.path, "192.0.2.10")
			require.Equal(t, http.StatusTooManyRequests, w.Code, "%s %s request %d must be 429 (tier %s, %d/min)", r.method, r.path, r.limit+1, r.tier, r.limit)
			var body struct {
				Error      string  `json:"error"`
				RetryAfter float64 `json:"retry_after"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Contains(t, body.Error, "Rate limit exceeded")
			require.Equal(t, 60.0, body.RetryAfter, "retry_after (seconds) is the 1m window")
			// Another client address has its own bucket: the 429 is per client, not global.
			require.NotEqual(t, http.StatusTooManyRequests, rlHit(e, r.method, r.path, "192.0.2.11").Code)
		})
	}
}

// Routes of one tier share ONE bucket (login+register on the 30/min tier), and
// the tiers do not share with each other. A route moved to another tier, or given
// a private limiter, breaks one of the two halves.
func TestAuthRoutes_TierBucketsSharedWithinAndIsolatedAcross(t *testing.T) {
	for _, tier := range []string{"login", "auth", "default"} {
		tier := tier
		t.Run("shared_"+tier, func(t *testing.T) {
			var rs []rlRoute
			for _, r := range rlRoutes {
				if r.tier == tier {
					rs = append(rs, r)
				}
			}
			e := realRateLimitedEngine(t)
			limit := rs[0].limit
			// spread `limit` requests round-robin over every route of the tier
			for i := 0; i < limit; i++ {
				r := rs[i%len(rs)]
				require.NotEqual(t, http.StatusTooManyRequests, rlHit(e, r.method, r.path, "192.0.2.20").Code, "request %d", i+1)
			}
			for _, r := range rs { // the very next request on ANY route of the tier is 429
				require.Equal(t, http.StatusTooManyRequests, rlHit(e, r.method, r.path, "192.0.2.20").Code,
					"%s %s shares the %s-tier bucket", r.method, r.path, tier)
			}
			for _, o := range rlRoutes { // routes of the other tiers are untouched
				if o.tier != tier {
					require.NotEqual(t, http.StatusTooManyRequests, rlHit(e, o.method, o.path, "192.0.2.20").Code,
						"%s %s (tier %s) must not be limited by tier %s", o.method, o.path, o.tier, tier)
				}
			}
		})
	}
}

// Every route registered under /api/v1/auth is in the table (a new auth route
// with no pinned tier fails here), and the control: a path outside the table is 404.
func TestAuthRoutes_EveryRegisteredAuthRouteHasAPinnedTier(t *testing.T) {
	e := realRateLimitedEngine(t)
	pinned := map[string]bool{}
	for _, r := range rlRoutes {
		pinned[fmt.Sprintf("%s /api/v1/auth/%s", r.method, r.path)] = true
	}
	n := 0
	for _, ri := range e.Routes() {
		if len(ri.Path) >= 12 && ri.Path[:12] == "/api/v1/auth" {
			n++
			require.True(t, pinned[ri.Method+" "+ri.Path], "auth route without a pinned limiter tier: %s %s", ri.Method, ri.Path)
		}
	}
	require.Equal(t, len(rlRoutes), n)
	require.Equal(t, http.StatusNotFound, rlHit(e, "GET", "never-registered", "192.0.2.30").Code)
}
