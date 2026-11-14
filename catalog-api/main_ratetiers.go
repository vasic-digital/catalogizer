package main

import (
	"catalogizer/internal/auth"

	"github.com/gin-gonic/gin"
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
