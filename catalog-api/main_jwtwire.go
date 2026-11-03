package main

import (
	"catalogizer/database"
	root_middleware "catalogizer/middleware"
	root_repository "catalogizer/repository"
)

// newAuthMiddleware is the one place the production JWT middleware is built:
// session revocation (CAT-021) and the live admin check (CAT-051 F4) are
// installed together, so the /api/v1/admin gate can never silently fall back
// to trusting the role_id claim. The same goes for RequirePermission
// (system.configure wizard routes): it is decided from the database, never the claim. main.go must use it (see
// TestMainGoWiresJWTMiddlewareThroughHelper).
func newAuthMiddleware(secret string, db *database.DB, userRepo *root_repository.UserRepository) *root_middleware.JWTMiddleware {
	m := root_middleware.NewJWTMiddleware(secret)
	m.SetSessionChecker(newSessionChecker(db))
	m.SetAdminChecker(userRepo.IsAdmin)
	m.SetPermissionChecker(userRepo.HasPermission)
	return m
}
