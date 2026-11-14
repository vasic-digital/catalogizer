package main

import (
	"catalogizer/internal/auth"
	root_middleware "catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// newAuthRateTiers builds the three in-memory rate-limit tiers (per client IP,
// 1 minute window) main() hands to buildRouter. Extracted from main() so a test
// can drive the REAL limiters through the REAL router (PKGC-FU4-F1).
//
//   - login   (30 rpm):   /login and /register only (brute-force flooders).
//   - authTier (600 rpm): token operations (/refresh, /logout, /status, /me ...).
//   - def    (2000 rpm):  everything else.
func newAuthRateTiers(authMiddleware *auth.AuthMiddleware) (login, authTier, def gin.HandlerFunc) {
	login = authMiddleware.RateLimitByUser(30, "1m")
	authTier = authMiddleware.RateLimitByUser(600, "1m")
	def = authMiddleware.RateLimitByUser(2000, "1m")
	return login, authTier, def
}

// newRouterDeps returns d with the three limiter tiers filled in. It is the ONLY place that
// maps newAuthRateTiers' results to routerDeps.LoginRate / AuthRate / DefaultRate and that
// applies the optional Redis tiers, and it is called by BOTH main() and the wiring test
// (RLW-FU1), so a swap of the login and auth tiers here fails the behavioural test instead of
// only a source pin. main() keeps no limiter variable of its own: it reads d.DefaultRate.
//
// Redis (operator decision 2026-11-12): when redisRateLimit is true and redisClient is not nil
// the auth and default tiers become Redis-backed with their own in-memory tier as fallback
// (a Redis outage after startup falls back to 600/2000 per minute, not 503); the login tier
// always stays in memory.
func newRouterDeps(d routerDeps, am *auth.AuthMiddleware, redisClient *redis.Client, redisRateLimit bool) routerDeps {
	login, authTier, def := newAuthRateTiers(am)
	if redisClient != nil && redisRateLimit {
		authTier, def = root_middleware.NewRedisTierLimiters(redisClient, authTier, def)
	}
	d.LoginRate, d.AuthRate, d.DefaultRate = login, authTier, def
	return d
}
