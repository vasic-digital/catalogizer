package handlers

// Adopted from the independent reviewer's round-1 probes (RV1..RV5, authored by
// the reviewer, adopted unchanged in behaviour as regression tests for CAT-D9-SUSPEND).

import (
	"context"
	"errors"
	"testing"
	"time"

	"catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A plain JWT route with the production-equivalent session checker
// (main_sessioncheck.go reads only user_sessions).
func (e *d9Env) jwtOnlyRoute() {
	mw := middleware.NewJWTMiddleware("test-jwt-secret-throwaway")
	db := e.db
	mw.SetSessionChecker(func(ctx context.Context, sessionID, userID string) error {
		var active bool
		var owner int
		var exp time.Time
		if err := db.QueryRowContext(ctx, "SELECT is_active, expires_at, user_id FROM user_sessions WHERE id = ?", sessionID).Scan(&active, &exp, &owner); err != nil {
			return err
		}
		if !active || exp.Before(time.Now()) || rtItoa(owner) != userID {
			return errors.New("session inactive")
		}
		return nil
	})
	e.r.GET("/api/v1/zz/jwtonly", mw.RequireAuth(), func(c *gin.Context) { c.String(200, "ok") })
}

// RV1: suspend via admin PUT; the access token is rejected on a JWT-only route.
func TestRV_SuspendRejectsJWTOnlyRoute(t *testing.T) {
	e := newD9Env(t)
	e.jwtOnlyRoute()
	s := e.loginFull(t, "dave", rtUserPass)
	require.Equal(t, 200, e.do(s.tok, "GET", "/api/v1/zz/jwtonly", "").Code)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"suspended"}`).Code)
	assert.Equal(t, 401, e.do(s.tok, "GET", "/api/v1/zz/jwtonly", "").Code)
}

// RV2: a lock written by LockAccount (POST /users/:id/lock path) leaves the
// token usable on a JWT-only route (D9S-F1 class).
func TestRV_LockAccountKeepsJWTOnlyAccess(t *testing.T) {
	e := newD9Env(t)
	e.jwtOnlyRoute()
	s := e.loginFull(t, "dave", rtUserPass)
	require.NoError(t, e.repo.LockAccount(e.userID, farFuture()))
	assert.Equal(t, 401, e.do(s.tok, "GET", "/api/v1/zz/jwtonly", "").Code, "locked user still has API access")
}

// RV3: stale-read overwrite. A writer that read the row before the suspension
// (self settings/profile save) clears the lock with plain Update.
func TestRV_StaleUpdateClearsSuspension(t *testing.T) {
	e := newD9Env(t)
	stale, err := e.repo.GetByID(e.userID)
	require.NoError(t, err)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"suspended"}`).Code)
	stale.Settings = `{"theme":"dark"}`
	require.NoError(t, e.repo.Update(stale))
	locked, _ := e.storedLock(t, e.userID)
	assert.True(t, locked, "suspension lost to a stale Update")
}

// RV3b: same through the guarded writer (CAS covers role/active only).
func TestRV_StaleGuardedUpdateClearsSuspension(t *testing.T) {
	e := newD9Env(t)
	stale, err := e.repo.GetByID(e.userID)
	require.NoError(t, err)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"suspended"}`).Code)
	applied, gerr := e.repo.UpdateGuarded(stale, stale.RoleID, stale.IsActive)
	require.NoError(t, gerr)
	_ = applied
	locked, _ := e.storedLock(t, e.userID)
	assert.True(t, locked, "suspension lost to a stale guarded update")
}

// RV4: a timed lock written over a permanent suspension turns it into a timed
// one; after expiry the suspended user can log in again.
func TestRV_TimedLockOverridesSuspension(t *testing.T) {
	e := newD9Env(t)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"suspended"}`).Code)
	require.NoError(t, e.repo.LockAccount(e.userID, time.Now().Add(-time.Minute)))
	assert.Equal(t, 401, e.login("dave", rtUserPass).Code, "suspended user logs in after a timed lock expired")
}

// RV5: list shows "suspended" for an expired timed lock (D9S-F4).
func TestRV_ListStatusExpiredLock(t *testing.T) {
	e := newD9Env(t)
	require.NoError(t, e.repo.LockAccount(e.userID, time.Now().Add(-time.Minute)))
	w := e.do(e.adminTok, "GET", "/api/v1/admin/users?status=suspended", "")
	require.Equal(t, 200, w.Code)
	assert.NotContains(t, w.Body.String(), `"dave"`, "expired lock listed as suspended")
	assert.Equal(t, 200, e.login("dave", rtUserPass).Code, "expired lock can log in")
}
