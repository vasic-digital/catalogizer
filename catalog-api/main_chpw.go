package main

import (
	"strconv"

	root_middleware "catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// sessionDeactivator revokes one session row (repository.UserRepository does).
type sessionDeactivator interface {
	DeactivateSession(sessionID int) error
}

// registerChangePasswordRoute wires POST /change-password on the auth group
// with the per-session failed-attempt limiter (CAT-CHPW-RL-1). main.go MUST
// register this route only through here (TestMainGoRegistersChangePasswordOnlyViaHelper).
// When a session exhausts its failure budget the session is revoked, so the
// RequireAuth session check rejects that token from then on.
func registerChangePasswordRoute(authGroup gin.IRoutes, rate, requireAuth, handler gin.HandlerFunc, sessions sessionDeactivator, logger *zap.Logger) {
	lim := root_middleware.NewFailedAttemptLimiter(root_middleware.CredentialMaxFailures,
		root_middleware.CredentialMaxUserFailures, root_middleware.CredentialFailureWindow)
	lim.OnSessionLimit = func(userID int, sessionID string) {
		sid, err := strconv.Atoi(sessionID)
		if err != nil {
			logger.Warn("change-password failure budget exhausted; session id not numeric, cannot revoke",
				zap.Int("user_id", userID))
			return
		}
		if err := sessions.DeactivateSession(sid); err != nil {
			logger.Error("change-password failure budget exhausted; session revoke failed",
				zap.Int("user_id", userID), zap.Int("session_id", sid), zap.Error(err))
			return
		}
		logger.Warn("change-password failure budget exhausted; session revoked",
			zap.Int("user_id", userID), zap.Int("session_id", sid))
	}
	root_middleware.RegisterChangePassword(authGroup, "/change-password", rate, requireAuth, handler, lim)
}

// authRoutesDeps are the handlers and middleware of the /api/v1/auth group.
type authRoutesDeps struct {
	LoginRate, AuthRate, DefaultRate gin.HandlerFunc
	RequireAuth                      func() gin.HandlerFunc
	Login, Register, Refresh, Logout gin.HandlerFunc
	Me, Status, Permissions          gin.HandlerFunc
	InitStatus, ChangePassword       gin.HandlerFunc
	Sessions                         sessionDeactivator
	Logger                           *zap.Logger
}

// registerAuthRoutes registers the whole /api/v1/auth group. It is the ONLY place
// that registers the change-password handler. Unit-class limit: the route-table
// test walks an engine built through this function, not main()'s router; main()
// still builds all other routes inline, and that wiring is covered only by the
// AST source guards (TestMainPackageChangePasswordWiringSource).
func registerAuthRoutes(g *gin.RouterGroup, d authRoutesDeps) {
	// Strict rate limiting for brute-force-prone write operations (30 rpm per IP).
	g.POST("/login", d.LoginRate, d.Login)
	g.POST("/register", d.LoginRate, d.Register)
	// Medium rate limiting for token operations (600 rpm)
	g.POST("/refresh", d.AuthRate, d.Refresh)
	g.POST("/logout", d.AuthRate, d.Logout)
	g.GET("/me", d.AuthRate, d.RequireAuth(), d.Me)
	g.GET("/status", d.AuthRate, d.Status)
	g.GET("/permissions", d.AuthRate, d.RequireAuth(), d.Permissions)
	g.GET("/profile", d.AuthRate, d.RequireAuth(), d.Me)
	g.GET("/init-status", d.DefaultRate, d.InitStatus)
	// change-password verifies the current password (bcrypt compare) behind a
	// JWT: failed verifications are limited per session/user (CAT-CHPW-RL-1).
	registerChangePasswordRoute(g, d.DefaultRate, d.RequireAuth(), d.ChangePassword, d.Sessions, d.Logger)
}
