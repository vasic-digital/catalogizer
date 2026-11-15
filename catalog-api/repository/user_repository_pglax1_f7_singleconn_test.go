package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/require"
)

// PGLAX1-F7-FU1: with MaxOpenConnections=1 the guarded write's transaction owns the only connection, so any
// admin-set read through the pool (instead of the tx) after the lock waits forever. This pins that the 4 SQLite
// user-write paths (UpdateGuarded, LockAccount, SetSuspended, Delete) read through the tx (reviewer mutants R2, R7).
func TestPGLAX1F7_UserWriteGuards_SingleConnectionPoolDoesNotHang(t *testing.T) {
	for _, name := range []string{"Delete", "SetSuspended", "LockAccount", "UpdateGuarded"} {
		t.Run(name, func(t *testing.T) {
			db, err := database.NewConnection(&config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "s.db"), MaxOpenConnections: 1})
			require.NoError(t, err)
			defer db.Close()
			require.NoError(t, db.RunMigrations(context.Background()))
			_, err = db.Exec(`UPDATE users SET role_id = 2`)
			require.NoError(t, err)
			repo := NewUserRepository(db)
			a := pglaX1Role(t, repo, "rA", "system.admin")
			pglaX1User(t, repo, "keep", a, true)
			victim := pglaX1User(t, repo, "victim", a, true)
			done := make(chan error, 1)
			go func() {
				switch name {
				case "Delete":
					done <- repo.Delete(victim)
				case "SetSuspended":
					done <- repo.SetSuspended(victim, true)
				case "LockAccount":
					done <- repo.LockAccount(victim, time.Now().Add(time.Hour))
				default:
					u, err := repo.GetByID(victim)
					if err != nil {
						done <- err
						return
					}
					u.IsActive = false
					_, err = repo.UpdateGuarded(u, u.RoleID, true)
					done <- err
				}
			}()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(15 * time.Second):
				t.Fatal("deadlock: guarded write did not finish on a single-connection pool")
			}
		})
	}
}

// TH2-MINOR-1: LockAccount is the only guarded write whose refusal read (refusedByGuard) runs inside the
// transaction, before Commit. With MaxOpenConnections=1 a refusal read through the pool (r.db) deadlocks. This pins
// the sole-administrator refusal path completing on one connection and releasing that connection afterwards. It does
// not pin the position of the refusal read relative to Commit (a read after Commit through the pool still passes).
func TestPGLAX1F7_LockAccount_SoleAdminRefusedOnSingleConnectionPool(t *testing.T) {
	db, err := database.NewConnection(&config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "s.db"), MaxOpenConnections: 1})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.RunMigrations(context.Background()))
	_, err = db.Exec(`UPDATE users SET role_id = 2`)
	require.NoError(t, err)
	repo := NewUserRepository(db)
	a := pglaX1Role(t, repo, "rA", "system.admin")
	sole := pglaX1User(t, repo, "sole", a, true)
	done := make(chan error, 1)
	go func() { done <- repo.LockAccount(sole, time.Now().Add(time.Hour)) }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrLastAdmin)
	case <-time.After(15 * time.Second):
		t.Fatal("deadlock: refused LockAccount did not finish on a single-connection pool")
	}
	// TH2M1-FU1: if the refused path leaked the transaction's only connection this read would block until the
	// package timeout; the bounded context makes it fail fast instead.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var locked bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(is_locked, '0') FROM users WHERE id = ?`, sole).Scan(&locked),
		"the connection was not released after the refused LockAccount")
	require.False(t, locked, "the refused lock must not have been applied")
}
