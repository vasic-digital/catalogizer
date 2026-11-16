package handlers

// CAT-PG-LASTADMIN-1: the timed lock removes an administrator exactly like a
// suspension, so LockAccount must refuse to lock the last effective admin
// (SQLite path; the PostgreSQL concurrency test lives in repository/).

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/require"
)

// runWithin runs fn in a goroutine and returns its result, or ok=false after d. The result travels over a buffered
// channel, so a goroutine abandoned after a timeout finishes without blocking and without writing into any variable
// the caller still reads (TH3-n4); callers must therefore return values from fn instead of assigning captured ones.
func runWithin[T any](d time.Duration, fn func() T) (T, bool) {
	ch := make(chan T, 1)
	go func() { ch <- fn() }()
	select {
	case v := <-ch:
		return v, true
	case <-time.After(d):
		var zero T
		return zero, false
	}
}

// withinDeadlockGuard fails the test after d instead of waiting for the go test timeout. The test environment uses a
// single-connection SQLite pool, so a refusal read taken through the pool while the lock transaction holds that
// connection blocks forever (PGLAST-F6). On timeout the stuck goroutine is leaked (the process exits when the test
// binary ends); only the failure is made fast.
func withinDeadlockGuard[T any](t *testing.T, d time.Duration, what string, fn func() T) T {
	t.Helper()
	v, ok := runWithin(d, fn)
	if !ok {
		t.Fatalf("deadlock guard: %s did not finish within %s on a single-connection pool", what, d)
	}
	return v
}

const lockGuardTimeout = 15 * time.Second

func TestLockAccount_LastAdminRefused_ThenAllowedWithSecondAdmin(t *testing.T) {
	e := newD9Env(t)
	adm, err := e.repo.GetByUsername("rtadmin")
	require.NoError(t, err)
	err = withinDeadlockGuard(t, lockGuardTimeout, "refused LockAccount", func() error { return e.repo.LockAccount(adm.ID, time.Now().Add(time.Hour)) })
	require.True(t, errors.Is(err, repository.ErrLastAdmin), "sole admin lock must be refused, got %v", err)
	u, _ := e.repo.GetByID(adm.ID)
	require.False(t, u.IsLocked, "refused lock must leave the account unlocked")

	hash, salt, herr := e.authSvc.HashPasswordForUser(rtUserPass)
	require.NoError(t, herr)
	_, cerr := e.repo.Create(&models.User{Username: "second_admin", Email: "sa@example.test", PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
	require.NoError(t, cerr)
	err = withinDeadlockGuard(t, lockGuardTimeout, "allowed LockAccount", func() error { return e.repo.LockAccount(adm.ID, time.Now().Add(time.Hour)) })
	require.NoError(t, err, "lock allowed while another active admin exists")
	// ...and the new sole effective admin can no longer be locked.
	sa, _ := e.repo.GetByUsername("second_admin")
	err = withinDeadlockGuard(t, lockGuardTimeout, "second refused LockAccount", func() error { return e.repo.LockAccount(sa.ID, time.Now().Add(time.Hour)) })
	require.True(t, errors.Is(err, repository.ErrLastAdmin))
}

// lockRacer simulates the concurrent writer of the race: just before the
// handler's own lock write reaches the real repository, the other administrator
// is locked (through the real repository), so the handler's target is by then
// the only effective administrator. Everything else is the real handler,
// service adapter and repository.
type lockRacer struct {
	*AuthServiceAdapter
	e       *d9Env
	otherID int
}

func (l *lockRacer) LockAccount(userID int, until time.Time) error {
	if err := l.e.repo.LockAccount(l.otherID, until); err != nil {
		return err
	}
	return l.AuthServiceAdapter.LockAccount(userID, until)
}

func TestLockAccountHTTP_LastAdminIs409_OthersUnchanged(t *testing.T) {
	e := newD9Env(t)
	hash, salt, herr := e.authSvc.HashPasswordForUser(rtUserPass)
	require.NoError(t, herr)
	ad2ID, cerr := e.repo.Create(&models.User{Username: "second_admin", Email: "sa@example.test", PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
	require.NoError(t, cerr)
	adm, err := e.repo.GetByUsername("rtadmin")
	require.NoError(t, err)

	base := &AuthServiceAdapter{Inner: e.authSvc}
	e.r.POST("/lk/users/:id/lock", WrapHTTPHandler(NewUserHandler(e.repo, base).LockAccount))
	e.r.POST("/lkrace/users/:id/lock", WrapHTTPHandler(NewUserHandler(e.repo, &lockRacer{AuthServiceAdapter: base, e: e, otherID: adm.ID}).LockAccount))
	body := `{"lock_until":"2099-01-01T00:00:00Z"}`

	// Non-admin / no user.manage permission: refused, state unchanged.
	w := e.do(e.userTok, "POST", "/lk/users/"+rtItoa(ad2ID)+"/lock", body)
	require.Contains(t, []int{401, 403}, w.Code, w.Body.String())
	u, _ := e.repo.GetByID(ad2ID)
	require.False(t, u.IsLocked)

	// Admin locking a non-admin: unchanged behaviour, 200.
	w = e.do(e.adminTok, "POST", "/lk/users/"+rtItoa(e.userID)+"/lock", body)
	require.Equal(t, 200, w.Code, w.Body.String())

	// The race: the target became the last effective administrator -> 409, not 500, and it stays unlocked.
	w = withinDeadlockGuard(t, lockGuardTimeout, "HTTP lock of the last administrator", func() *httptest.ResponseRecorder {
		return e.do(e.adminTok, "POST", "/lkrace/users/"+rtItoa(ad2ID)+"/lock", body)
	})
	require.Equal(t, 409, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "last administrator")
	u, _ = e.repo.GetByID(ad2ID)
	require.False(t, u.IsLocked, "refused lock must leave the last administrator unlocked")
}
