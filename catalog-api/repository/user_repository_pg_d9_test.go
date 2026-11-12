// PostgreSQL behaviour of SetSuspended/LockAccount/UnlockAccount; gated on CATALOGIZER_PG_TEST_{HOST,PORT,USER,PASSWORD}. Adopted from reviewer probe RV2 (M3, CAT-D9-SUSPEND).
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestD9S_PG_SuspendLockUnlock(t *testing.T) {
	host := os.Getenv("CATALOGIZER_PG_TEST_HOST")
	port, _ := strconv.Atoi(os.Getenv("CATALOGIZER_PG_TEST_PORT"))
	user := os.Getenv("CATALOGIZER_PG_TEST_USER")
	pass := os.Getenv("CATALOGIZER_PG_TEST_PASSWORD")
	if host == "" || port == 0 {
		t.Skip("CATALOGIZER_PG_TEST_HOST/PORT not set: PostgreSQL suspend/lock test skipped (honest SKIP, not a pass)")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable", user, pass, host, port))
	require.NoError(t, err)
	defer admin.Close()
	name := fmt.Sprintf("tmp_d9s_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	defer func() { _, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name) }()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "postgres", Host: host, Port: port, Name: name, User: user, Password: pass, SSLMode: "disable"})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.RunMigrations(ctx))
	repo := NewUserRepository(db)
	adminRole := 1
	mk := func(n string, role int) int {
		id, err := repo.Create(&models.User{Username: n, Email: n + "@x.test", RoleID: role, IsActive: true, Salt: "s", PasswordHash: "h"})
		require.NoError(t, err)
		return id
	}
	// count pre-existing active admins (seeded?)
	var seeded int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users WHERE role_id = 1 AND is_active = TRUE`).Scan(&seeded))
	t.Logf("seeded active admins: %d", seeded)
	u := mk("plain", 2)
	lockedState := func(id int) (bool, bool) {
		var l bool
		var until *time.Time
		require.NoError(t, db.QueryRow(`SELECT is_locked, locked_until FROM users WHERE id = ?`, id).Scan(&l, &until))
		return l, until != nil
	}
	require.NoError(t, repo.SetSuspended(u, true))
	l, hasUntil := lockedState(u)
	assert.True(t, l)
	assert.False(t, hasUntil)
	require.NoError(t, repo.LockAccount(u, time.Now().Add(time.Hour)))
	l, hasUntil = lockedState(u)
	assert.True(t, l)
	assert.False(t, hasUntil, "timed lock must not replace suspension (PG)")
	assert.True(t, errors.Is(repo.UnlockAccount(u), ErrAccountSuspended), "PG unlock refuses suspension")
	require.NoError(t, repo.SetSuspended(u, false))
	l, _ = lockedState(u)
	assert.False(t, l)
	require.NoError(t, repo.LockAccount(u, time.Now().Add(time.Hour)))
	require.NoError(t, repo.UnlockAccount(u))
	// last-admin guard: two admins suspending each other concurrently
	if seeded == 0 {
		a1, a2 := mk("a1", adminRole), mk("a2", adminRole)
		for i := 0; i < 20; i++ {
			require.NoError(t, repo.SetSuspended(a1, false))
			require.NoError(t, repo.SetSuspended(a2, false))
			var wg sync.WaitGroup
			errs := make([]error, 2)
			wg.Add(2)
			go func() { defer wg.Done(); errs[0] = repo.SetSuspended(a1, true) }()
			go func() { defer wg.Done(); errs[1] = repo.SetSuspended(a2, true) }()
			wg.Wait()
			var free int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users WHERE role_id = 1 AND is_active = TRUE AND is_locked = FALSE`).Scan(&free))
			require.GreaterOrEqual(t, free, 1, "iteration %d errs=%v", i, errs)
		}
		// expired timed lock admin counts as unlocked
		require.NoError(t, repo.SetSuspended(a1, false))
		require.NoError(t, repo.SetSuspended(a2, false))
		require.NoError(t, repo.SetSuspended(a2, true))
		require.NoError(t, repo.SetSuspended(a1, false))
		require.NoError(t, repo.LockAccount(a1, time.Now().Add(-time.Hour)))
		require.NoError(t, repo.SetSuspended(a2, false))
		assert.NoError(t, repo.SetSuspended(a2, true), "a1 expired lock counts as unlocked; suspending a2 allowed")
		assert.True(t, errors.Is(repo.SetSuspended(a1, true), ErrLastAdmin))
	}
}
