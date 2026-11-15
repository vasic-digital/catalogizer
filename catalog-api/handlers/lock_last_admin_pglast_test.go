package handlers

// CAT-PG-LASTADMIN-1: the timed lock removes an administrator exactly like a
// suspension, so LockAccount must refuse to lock the last effective admin
// (SQLite path; the PostgreSQL concurrency test lives in repository/).

import (
	"errors"
	"testing"
	"time"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/require"
)

func TestLockAccount_LastAdminRefused_ThenAllowedWithSecondAdmin(t *testing.T) {
	e := newD9Env(t)
	adm, err := e.repo.GetByUsername("rtadmin")
	require.NoError(t, err)
	err = e.repo.LockAccount(adm.ID, time.Now().Add(time.Hour))
	require.True(t, errors.Is(err, repository.ErrLastAdmin), "sole admin lock must be refused, got %v", err)
	u, _ := e.repo.GetByID(adm.ID)
	require.False(t, u.IsLocked, "refused lock must leave the account unlocked")

	hash, salt, herr := e.authSvc.HashPasswordForUser(rtUserPass)
	require.NoError(t, herr)
	_, cerr := e.repo.Create(&models.User{Username: "second_admin", Email: "sa@example.test", PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
	require.NoError(t, cerr)
	require.NoError(t, e.repo.LockAccount(adm.ID, time.Now().Add(time.Hour)), "lock allowed while another active admin exists")
	// ...and the new sole effective admin can no longer be locked.
	sa, _ := e.repo.GetByUsername("second_admin")
	require.True(t, errors.Is(e.repo.LockAccount(sa.ID, time.Now().Add(time.Hour)), repository.ErrLastAdmin))
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
	w = e.do(e.adminTok, "POST", "/lkrace/users/"+rtItoa(ad2ID)+"/lock", body)
	require.Equal(t, 409, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "last administrator")
	u, _ = e.repo.GetByID(ad2ID)
	require.False(t, u.IsLocked, "refused lock must leave the last administrator unlocked")
}
