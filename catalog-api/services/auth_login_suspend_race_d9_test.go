// Adopted from the independent reviewer round-2 probe RV2-B1 (CAT-D9-SUSPEND).
package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RV2-B1: user with an EXPIRED timed lock (is_locked=1, locked_until in the past)
// logs in; the admin suspension lands during bcrypt. is_locked stays 1, only
// locked_until changes to NULL, so verifyUserUnchanged must still refuse.
func TestD9S_ExpiredLockThenSuspendDuringLogin(t *testing.T) {
	const pw = "Stale!Throwaway#1aa"
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := repository.NewUserRepository(db)
	svc := NewAuthService(repo, "test-jwt-secret-throwaway")
	id, err := repo.Create(&models.User{Username: "stale", Email: "stale@example.test",
		RoleID: 2, IsActive: true, Salt: "0123456789abcdef0123456789abcdef"})
	require.NoError(t, err)
	require.NoError(t, svc.ResetPassword(id, pw))
	require.NoError(t, repo.LockAccount(id, time.Now().Add(-time.Hour)))

	loginPostSessionHook = func() {
		require.NoError(t, repo.SetSuspended(id, true))
		// admin revokes happened before this login's session existed
	}
	t.Cleanup(func() { loginPostSessionHook = nil })
	res, err := svc.Login(models.LoginRequest{Username: "stale", Password: pw}, "127.0.0.1", "test")
	loginPostSessionHook = nil
	var active int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = 1`, id).Scan(&active))
	assert.Error(t, err, "login must be refused")
	assert.Nil(t, res)
	assert.Equal(t, 0, active, "no active session may survive a suspension")
}
