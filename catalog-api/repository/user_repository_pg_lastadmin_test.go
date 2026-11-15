// CAT-PG-LASTADMIN-1: live PostgreSQL concurrency test of the last-administrator
// invariant across ALL admin-removing writes (demote, suspend, timed lock, delete).
// Gated on CATALOGIZER_PG_TEST_{HOST,PORT,USER,PASSWORD}; honest SKIP otherwise.
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

	"github.com/stretchr/testify/require"
)

func TestPGLastAdmin_ConcurrentRemovalsNeverLeaveZero(t *testing.T) {
	host := os.Getenv("CATALOGIZER_PG_TEST_HOST")
	port, _ := strconv.Atoi(os.Getenv("CATALOGIZER_PG_TEST_PORT"))
	user := os.Getenv("CATALOGIZER_PG_TEST_USER")
	pass := os.Getenv("CATALOGIZER_PG_TEST_PASSWORD")
	if host == "" || port == 0 {
		t.Skip("CATALOGIZER_PG_TEST_HOST/PORT not set: PostgreSQL last-admin race test skipped (honest SKIP, not a pass)")
	}
	rounds := 50
	if v, _ := strconv.Atoi(os.Getenv("CATALOGIZER_PG_LASTADMIN_ROUNDS")); v > 0 {
		rounds = v
	}
	ctx := context.Background()
	adm, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable", user, pass, host, port))
	require.NoError(t, err)
	defer adm.Close()
	name := fmt.Sprintf("tmp_lastadmin_%d", time.Now().UnixNano())
	_, err = adm.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	defer func() { _, _ = adm.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name) }()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "postgres", Host: host, Port: port, Name: name, User: user, Password: pass, SSLMode: "disable"})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.RunMigrations(ctx))
	repo := NewUserRepository(db)
	// Seeded administrators would mask the invariant: demote them out of the way.
	_, err = db.Exec(`UPDATE users SET role_id = 2`)
	require.NoError(t, err)
	mk := func(n string) int {
		id, cerr := repo.Create(&models.User{Username: n, Email: n + "@x.test", RoleID: 1, IsActive: true, Salt: "s", PasswordHash: "h"})
		require.NoError(t, cerr)
		return id
	}
	seq := 0
	var admins []int
	effective := func() int {
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users WHERE role_id = 1 AND is_active = TRUE AND (is_locked IS NULL OR is_locked = FALSE OR (locked_until IS NOT NULL AND locked_until <= NOW()))`).Scan(&n))
		return n
	}
	// reset restores exactly n active, unlocked administrators (recreating deleted ones).
	reset := func(n int) {
		for len(admins) < n {
			seq++
			admins = append(admins, mk(fmt.Sprintf("la%d", seq)))
		}
		for k := 0; k < n; k++ {
			if _, gerr := repo.GetByID(admins[k]); gerr != nil {
				seq++
				admins[k] = mk(fmt.Sprintf("la%d", seq))
			}
			_, rerr := db.Exec(`UPDATE users SET role_id = 1, is_active = TRUE, is_locked = FALSE, locked_until = NULL WHERE id = ?`, admins[k])
			require.NoError(t, rerr)
		}
		for _, id := range admins[n:] {
			_, rerr := db.Exec(`UPDATE users SET role_id = 2 WHERE id = ?`, id)
			require.NoError(t, rerr)
		}
	}
	demote := func(id int) error {
		u, gerr := repo.GetByID(id)
		if gerr != nil {
			return gerr
		}
		u.RoleID = 2
		_, uerr := repo.UpdateGuarded(u, 1, true)
		return uerr
	}
	// PGLAX1-F3: deactivation through UpdateGuarded (role unchanged, IsActive=false).
	deactivate := func(id int) error {
		u, gerr := repo.GetByID(id)
		if gerr != nil {
			return gerr
		}
		u.IsActive = false
		_, uerr := repo.UpdateGuarded(u, 1, true)
		return uerr
	}
	type op func(id int) error
	ops := map[string]op{
		"lock":       func(id int) error { return repo.LockAccount(id, time.Now().Add(time.Hour)) },
		"suspend":    func(id int) error { return repo.SetSuspended(id, true) },
		"demote":     demote,
		"delete":     func(id int) error { return repo.Delete(id) },
		"deactivate": deactivate,
	}
	variants := [][]string{
		{"lock", "lock"}, {"lock", "suspend"}, {"lock", "demote"}, {"lock", "delete"},
		{"suspend", "suspend"}, {"demote", "demote"}, {"delete", "delete"}, {"demote", "delete"},
		{"lock", "lock", "lock"}, {"lock", "suspend", "demote"}, {"lock", "demote", "delete"}, {"suspend", "suspend", "lock"},
		{"deactivate", "deactivate"}, {"deactivate", "lock"}, {"deactivate", "delete"}, {"deactivate", "demote"},
	}
	for _, v := range variants {
		v := v
		name := ""
		for k, o := range v {
			if k > 0 {
				name += "_x_"
			}
			name += o
		}
		t.Run(name, func(t *testing.T) {
			n := len(v)
			violations := 0
			for i := 0; i < rounds; i++ {
				reset(n)
				require.Equal(t, n, effective(), "round %d precondition", i)
				errs := make([]error, n)
				var wg sync.WaitGroup
				start := make(chan struct{})
				wg.Add(n)
				for k := 0; k < n; k++ {
					k := k
					go func() { defer wg.Done(); <-start; errs[k] = ops[v[k]](admins[k]) }()
				}
				close(start)
				wg.Wait()
				lastAdmin, other := 0, 0
				for _, e := range errs {
					switch {
					case e == nil:
					case errors.Is(e, ErrLastAdmin):
						lastAdmin++
					default:
						other++
						t.Errorf("round %d: unexpected error %v", i, e)
					}
				}
				// Exactly one administrator survives, exactly one write is refused with
				// ErrLastAdmin (positive control: the guard really fired), nothing else failed.
				if got := effective(); got != 1 || lastAdmin != 1 || other != 0 {
					violations++
					t.Errorf("round %d: effective=%d last_admin_errors=%d other_errors=%d errs=%v", i, got, lastAdmin, other, errs)
				}
			}
			t.Logf("RESULT pair=%s rounds=%d violations=%d", name, rounds, violations)
			require.Zero(t, violations)
		})
	}
}
