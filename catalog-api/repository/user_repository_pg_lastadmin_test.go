// CAT-PG-LASTADMIN-1: live PostgreSQL concurrency test of the last-administrator
// invariant across ALL admin-removing writes (demote, suspend, timed lock, delete).
// Gated on CATALOGIZER_PG_TEST_{HOST,PORT,USER,PASSWORD}; honest SKIP otherwise.
package repository

import (
	"context"
	"database/sql"
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
	a1, a2 := mk("la1"), mk("la2")
	effective := func() int {
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users WHERE role_id = 1 AND is_active = TRUE AND (is_locked IS NULL OR is_locked = FALSE OR (locked_until IS NOT NULL AND locked_until <= NOW()))`).Scan(&n))
		return n
	}
	reset := func() {
		for _, id := range []int{a1, a2} {
			if _, gerr := repo.GetByID(id); gerr != nil {
				// deleted by the previous round: recreate under the same name
				nn := "la" + strconv.Itoa(id)
				nid, cerr := repo.Create(&models.User{Username: nn + strconv.FormatInt(time.Now().UnixNano(), 10), Email: nn + strconv.FormatInt(time.Now().UnixNano(), 10) + "@x.test", RoleID: 1, IsActive: true, Salt: "s", PasswordHash: "h"})
				require.NoError(t, cerr)
				if id == a1 {
					a1 = nid
				} else {
					a2 = nid
				}
			}
		}
		_, rerr := db.Exec(`UPDATE users SET role_id = 1, is_active = TRUE, is_locked = FALSE, locked_until = NULL WHERE id IN (?, ?)`, a1, a2)
		require.NoError(t, rerr)
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
	type op func(id int) error
	ops := map[string]op{
		"lock":    func(id int) error { return repo.LockAccount(id, time.Now().Add(time.Hour)) },
		"suspend": func(id int) error { return repo.SetSuspended(id, true) },
		"demote":  demote,
		"delete":  func(id int) error { return repo.Delete(id) },
	}
	for _, pair := range [][2]string{{"lock", "lock"}, {"lock", "suspend"}, {"lock", "demote"}, {"lock", "delete"}, {"suspend", "suspend"}, {"demote", "demote"}, {"delete", "delete"}, {"demote", "delete"}} {
		pair := pair
		t.Run(pair[0]+"_x_"+pair[1], func(t *testing.T) {
			violations := 0
			for i := 0; i < rounds; i++ {
				reset()
				require.Equal(t, 2, effective())
				var wg sync.WaitGroup
				start := make(chan struct{})
				wg.Add(2)
				go func() { defer wg.Done(); <-start; _ = ops[pair[0]](a1) }()
				go func() { defer wg.Done(); <-start; _ = ops[pair[1]](a2) }()
				close(start)
				wg.Wait()
				if effective() == 0 {
					violations++
				}
			}
			t.Logf("RESULT pair=%s_x_%s rounds=%d zero_admin_violations=%d", pair[0], pair[1], rounds, violations)
			require.Zero(t, violations, "concurrent %s x %s left zero effective administrators in %d/%d rounds", pair[0], pair[1], violations, rounds)
		})
	}
}
