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
