package handlers

// CAT-D9-SUSPEND: suspending a user through PUT /api/v1/admin/users/:id must
// persist the lock, revoke the user's sessions and keep the user out of login,
// refresh and every session-bound route; "active" must lift the lock again.
// Real SQLite with the production migrations, real repository, AuthService
// and handlers (no test doubles).

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type suspendSess struct{ tok, refresh string }

func (e *d9Env) loginFull(t *testing.T, user, pass string) suspendSess {
	t.Helper()
	w := e.login(user, pass)
	require.Equal(t, 200, w.Code, w.Body.String())
	var res struct {
		SessionToken string `json:"session_token"`
		RefreshToken string `json:"refresh_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	return suspendSess{res.SessionToken, res.RefreshToken}
}

func (e *d9Env) storedLock(t *testing.T, id int) (locked bool, until *string) {
	t.Helper()
	require.NoError(t, e.db.QueryRow(`SELECT is_locked, locked_until FROM users WHERE id = ?`, id).Scan(&locked, &until))
	return
}

func (e *d9Env) activeSessions(t *testing.T, id int) int {
	t.Helper()
	var n int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = 1`, id).Scan(&n))
	return n
}

func (e *d9Env) refresh(rt string) int {
	return e.do("", "POST", "/api/v1/auth/refresh", `{"refresh_token":"`+rt+`"}`).Code
}

func TestD9S_SuspendPersistsLockAndRevokesSessions(t *testing.T) {
	e := newD9Env(t)
	e.r.POST("/api/v1/auth/refresh", NewAuthHandler(e.authSvc).RefreshTokenGin)
	s := e.loginFull(t, "dave", rtUserPass)
	s2 := e.loginFull(t, "dave", rtUserPass) // a second device
	require.Equal(t, 200, e.do(s.tok, "GET", "/api/v1/users/"+rtItoa(e.userID), "").Code, "baseline: token works")
	require.Equal(t, 3, e.activeSessions(t, e.userID), "setup: fixture login + 2 logins")

	w := put(e, e.adminTok, e.userID, `{"status":"suspended"}`)
	require.Equal(t, 200, w.Code, w.Body.String())

	locked, until := e.storedLock(t, e.userID)
	assert.True(t, locked, "is_locked must be persisted")
	assert.Nil(t, until, "suspension is permanent (no locked_until)")
	assert.Equal(t, 0, e.activeSessions(t, e.userID), "all sessions revoked")
	assert.Equal(t, 401, e.do(s.tok, "GET", "/api/v1/users/"+rtItoa(e.userID), "").Code, "old token rejected")
	assert.Equal(t, 401, e.do(s2.tok, "GET", "/api/v1/users/"+rtItoa(e.userID), "").Code, "second device token rejected")
	assert.Equal(t, 401, e.refresh(s.refresh), "refresh rejected")
	lw := e.login("dave", rtUserPass)
	assert.Equal(t, 401, lw.Code, "login rejected")
	assert.Contains(t, lw.Body.String(), "account is temporarily locked", "existing intentional message")
}

func TestD9S_UnsuspendRestores(t *testing.T) {
	e := newD9Env(t)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"suspended"}`).Code)
	require.Equal(t, 401, e.login("dave", rtUserPass).Code)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"active"}`).Code)
	locked, until := e.storedLock(t, e.userID)
	assert.False(t, locked)
	assert.Nil(t, until)
	s := e.loginFull(t, "dave", rtUserPass)
	assert.Equal(t, 200, e.do(s.tok, "GET", "/api/v1/users/"+rtItoa(e.userID), "").Code)
}

func TestD9S_UnsuspendClearsTimedLock(t *testing.T) {
	e := newD9Env(t)
	_, err := e.db.Exec(`UPDATE users SET is_locked = 1, locked_until = datetime('now','+1 hour') WHERE id = ?`, e.userID)
	require.NoError(t, err)
	require.Equal(t, 401, e.login("dave", rtUserPass).Code)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"active"}`).Code)
	locked, until := e.storedLock(t, e.userID)
	assert.False(t, locked)
	assert.Nil(t, until, "active status must clear locked_until too")
	assert.Equal(t, 200, e.login("dave", rtUserPass).Code)
}

// A non-status update (e.g. email) must not clear or change the persisted lock.
func TestD9S_UnrelatedUpdateKeepsLock(t *testing.T) {
	e := newD9Env(t)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"suspended"}`).Code)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"email":"dave2@example.test"}`).Code)
	locked, _ := e.storedLock(t, e.userID)
	assert.True(t, locked)
	assert.Equal(t, 401, e.login("dave", rtUserPass).Code)
}

// The session-bound user lookup refuses a locked account even when a session
// row is still active (lock written by another writer, e.g. brute-force lockout).
func TestD9S_GetCurrentUserRefusesLockedAccount(t *testing.T) {
	e := newD9Env(t)
	s := e.loginFull(t, "dave", rtUserPass)
	require.NoError(t, e.repo.LockAccount(e.userID, farFuture()))
	assert.Equal(t, 401, e.do(s.tok, "GET", "/api/v1/users/"+rtItoa(e.userID), "").Code)
}

func TestD9S_AdminCannotSuspendSelf(t *testing.T) {
	e := newD9Env(t)
	a, err := e.repo.GetByUsername("rtadmin")
	require.NoError(t, err)
	assert.Equal(t, 400, put(e, e.adminTok, a.ID, `{"status":"suspended"}`).Code)
	locked, _ := e.storedLock(t, a.ID)
	assert.False(t, locked)
}

// Last-admin rule extends to suspension at the repository write: a suspended
// admin is not an active admin, so the only active admin cannot be locked.
func TestD9S_RepoRefusesLockingLastActiveAdmin(t *testing.T) {
	e := newD9Env(t)
	a, err := e.repo.GetByUsername("rtadmin")
	require.NoError(t, err)
	require.Equal(t, 1, e.activeAdmins(t))
	a.IsLocked = true
	applied, gerr := e.repo.UpdateGuarded(a, a.RoleID, a.IsActive)
	assert.False(t, applied)
	assert.ErrorIs(t, gerr, repository.ErrLastAdmin)
	locked, _ := e.storedLock(t, a.ID)
	assert.False(t, locked)
	// with a second active admin the same write is allowed
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"role":"admin"}`).Code)
	applied, gerr = e.repo.UpdateGuarded(a, a.RoleID, a.IsActive)
	assert.True(t, applied)
	assert.NoError(t, gerr)
}

func TestD9S_ConcurrentMutualSuspendNeverZeroAdmins(t *testing.T) {
	e := newD9Env(t)
	for i := 0; i < 20; i++ {
		aTok, bTok, aID, bID := e.resetTwoAdmins(t)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); put(e, aTok, bID, `{"status":"suspended"}`) }()
		go func() { defer wg.Done(); put(e, bTok, aID, `{"status":"suspended"}`) }()
		wg.Wait()
		assert.GreaterOrEqual(t, e.activeAdmins(t), 1, fmt.Sprintf("iteration %d: at least one active admin must remain", i))
	}
}

func farFuture() time.Time { return time.Now().Add(24 * time.Hour) }

// Both repository write variants persist the lock columns (the plain Update is
// used when the repository does not implement the guarded form).
func TestD9S_RepoUpdatePersistsLock(t *testing.T) {
	e := newD9Env(t)
	u, err := e.repo.GetByID(e.userID)
	require.NoError(t, err)
	u.IsLocked = true
	require.NoError(t, e.repo.Update(u))
	locked, until := e.storedLock(t, e.userID)
	assert.True(t, locked)
	assert.Nil(t, until)
	u.IsLocked = false
	ts := farFuture()
	u.LockedUntil = &ts
	require.NoError(t, e.repo.Update(u))
	locked, until = e.storedLock(t, e.userID)
	assert.False(t, locked)
	assert.NotNil(t, until)
}
