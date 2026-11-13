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
