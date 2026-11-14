package main

import (
	"net/http"
	"strings"
	"testing"

	"catalogizer/internal/auth"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type ginEngineBox struct{ *gin.Engine }

// CAT-MW-TIERTEST-1. Unit-class (miniredis is a test double, §11.4.289) but the router
// is the one buildRouter returns and the limiters are those newRouterDeps builds with
// the Redis tiers ON: the real wiring path, not a source grep.
//
// Property: with Redis rate limiting enabled, /login and /register stay on the
// in-memory login tier (30/min, 429 on the 31st, NO Redis key ever written), while the
// auth tier (/me) and default tier (/init-status) write their own Redis namespaces.
func TestLoginRegisterAreNotRedisTiers(t *testing.T) {
	newEngine := func(t *testing.T) (*miniredis.Miniredis, *ginEngineBox) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		am := auth.NewAuthMiddleware(nil, zap.NewNop())
		e := buildRouter(newRouterDeps(routerTestDeps(), am, client, true))
		return mr, &ginEngineBox{e}
	}

	for _, route := range []string{"login", "register"} {
		route := route
		t.Run(route, func(t *testing.T) {
			mr, e := newEngine(t)
			for i := 1; i <= 30; i++ {
				w := rlHit(e.Engine, "POST", route, "192.0.2.50")
				require.NotEqual(t, http.StatusTooManyRequests, w.Code, "request %d must pass the login tier", i)
			}
			w := rlHit(e.Engine, "POST", route, "192.0.2.50")
			require.Equal(t, http.StatusTooManyRequests, w.Code, "31st request is limited by the in-memory 30/min login tier")
			require.Empty(t, mr.Keys(), "%s must never touch a Redis tier", route)
		})
	}

	t.Run("auth and default tiers ARE Redis-backed (positive control)", func(t *testing.T) {
		mr, e := newEngine(t)
		rlHit(e.Engine, "GET", "me", "192.0.2.51")
		rlHit(e.Engine, "GET", "init-status", "192.0.2.51")
		keys := strings.Join(mr.Keys(), ",")
		require.Contains(t, keys, "rate_limit:auth:192.0.2.51")
		require.Contains(t, keys, "rate_limit:default:192.0.2.51")
	})
}
