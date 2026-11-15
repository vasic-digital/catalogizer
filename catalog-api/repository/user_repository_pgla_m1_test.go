// PGLA-M1: live PostgreSQL pin of the "disambiguation re-read happens INSIDE the transaction" claim of
// LockAccount (user_repository.go, refusedByGuard(tx, ...)). The classification of a zero-row guarded UPDATE
// must read the same state the UPDATE saw, i.e. while the shared advisory lock is still held.
//
// How it is observed on the real code path: the repository connects as a non-superuser role, and the users
// table carries a row-level-security policy whose predicate is a real SQL function. For every scan of a users
// row that function records, in the scanning backend, the statement text and whether THAT backend holds the
// advisory lock at that moment. The disambiguation re-read is the only `SELECT role_id, COALESCE(is_locked`
// statement; it must have run with the lock held. A re-read after Commit (or on another pooled connection)
// runs without it and is recorded held=false. Gated on CATALOGIZER_PG_TEST_{HOST,PORT,USER,PASSWORD};
// honest SKIP otherwise.
package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

func TestPGLAM1_ReReadRunsInsideTxWhileAdvisoryLockHeld(t *testing.T) {
	host := os.Getenv("CATALOGIZER_PG_TEST_HOST")
	rawPort := os.Getenv("CATALOGIZER_PG_TEST_PORT")
	port := 0
	if rawPort != "" {
		var perr error
		port, perr = strconv.Atoi(rawPort)
		// PGLA-M1-N2: a set-but-unparsable port is a configuration error, never an honest "not set" SKIP.
		if perr != nil || port <= 0 {
			t.Fatalf("CATALOGIZER_PG_TEST_PORT is set but not a positive integer: %q", rawPort)
		}
	}
	user := os.Getenv("CATALOGIZER_PG_TEST_USER")
	pass := os.Getenv("CATALOGIZER_PG_TEST_PASSWORD")
	if host == "" || port == 0 {
		t.Skip("CATALOGIZER_PG_TEST_HOST/PORT not set: PostgreSQL in-tx re-read test skipped (honest SKIP, not a pass)")
	}
	ctx := context.Background()
	adm, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable", user, pass, host, port))
	require.NoError(t, err)
	defer adm.Close()
	name := fmt.Sprintf("tmp_pgla_m1_%d", time.Now().UnixNano())
	_, err = adm.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	rb := make([]byte, 9)
	_, err = rand.Read(rb)
	require.NoError(t, err)
	appPass := hex.EncodeToString(rb)
	appRole := "pgla_m1_app_" + hex.EncodeToString(rb[:4])
	// PGLA-M1-F1: the database (which owns objects the role has privileges on) is dropped FIRST, then the
	// role; both errors are checked and logged, never swallowed.
	defer func() {
		if _, derr := adm.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); derr != nil {
			t.Errorf("cleanup: DROP DATABASE %s: %v", name, derr)
		}
		if _, derr := adm.ExecContext(ctx, "DROP ROLE IF EXISTS "+appRole); derr != nil {
			t.Errorf("cleanup: DROP ROLE %s: %v", appRole, derr)
		}
	}()

	owner, err := database.NewConnection(&config.DatabaseConfig{Type: "postgres", Host: host, Port: port, Name: name, User: user, Password: pass, SSLMode: "disable"})
	require.NoError(t, err)
	defer owner.Close()
	require.NoError(t, owner.RunMigrations(ctx))

	for _, stmt := range []string{
		fmt.Sprintf("CREATE ROLE %s LOGIN NOSUPERUSER PASSWORD '%s'", appRole, appPass),
		`CREATE TABLE pgla_m1_probe (id BIGSERIAL PRIMARY KEY, q TEXT NOT NULL, held BOOLEAN NOT NULL)`,
		// SECURITY DEFINER: the probe writes as the owner; it runs inside the scanning backend, so
		// pg_backend_pid() and pg_locks describe the very transaction doing the read.
		`CREATE FUNCTION pgla_m1_users_probe() RETURNS boolean LANGUAGE sql VOLATILE SECURITY DEFINER AS $$
			INSERT INTO pgla_m1_probe(q, held)
			SELECT current_query(), EXISTS (SELECT 1 FROM pg_locks WHERE locktype = 'advisory' AND pid = pg_backend_pid() AND granted AND classid = 0 AND objsubid = 1 AND objid = 7340001);
			SELECT true $$`,
		`GRANT EXECUTE ON FUNCTION pgla_m1_users_probe() TO PUBLIC`,
		`ALTER TABLE users ENABLE ROW LEVEL SECURITY`,
		`CREATE POLICY pgla_m1_policy ON users USING (pgla_m1_users_probe())`,
		fmt.Sprintf("GRANT ALL ON ALL TABLES IN SCHEMA public TO %s", appRole),
		fmt.Sprintf("GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO %s", appRole),
	} {
		_, xerr := owner.Exec(stmt)
		require.NoError(t, xerr, stmt)
	}
	app, err := database.NewConnection(&config.DatabaseConfig{Type: "postgres", Host: host, Port: port, Name: name, User: appRole, Password: appPass, SSLMode: "disable"})
	require.NoError(t, err)
	defer app.Close()
	repo := NewUserRepository(app)

	// Exactly one effective administrator: locking it must be refused by the guarded UPDATE (zero rows).
	_, err = owner.Exec(`UPDATE users SET role_id = 2`)
	require.NoError(t, err)
	only, err := repo.Create(&models.User{Username: "m1_only", Email: "m1_only@x.test", RoleID: 1, IsActive: true, Salt: "s", PasswordHash: "h"})
	require.NoError(t, err)
	_, err = repo.Create(&models.User{Username: "m1_plain", Email: "m1_plain@x.test", RoleID: 2, IsActive: true, Salt: "s", PasswordHash: "h"})
	require.NoError(t, err)

	const reread = `SELECT role_id, COALESCE(is_locked%`
	count := func(where string) int {
		var n int
		require.NoError(t, owner.QueryRow(`SELECT COUNT(*) FROM pgla_m1_probe WHERE `+where).Scan(&n))
		return n
	}

	// Control needle (positive): a read outside any advisory lock IS recorded, with held = false.
	var rid int
	require.NoError(t, app.QueryRow(`SELECT role_id FROM users WHERE id = ?`, only).Scan(&rid))
	require.Equal(t, 1, rid)
	require.GreaterOrEqual(t, count(`held = FALSE AND q LIKE 'SELECT role_id FROM users%'`), 1, "probe must see an un-locked read (instrument is not blind)")
	// Control needle (negative): no re-read has happened yet.
	require.Zero(t, count(`q LIKE '`+reread+`'`), "no disambiguation re-read before LockAccount")

	err = repo.LockAccount(only, time.Now().Add(time.Hour))
	require.True(t, errors.Is(err, ErrLastAdmin), "locking the last administrator must be refused, got %v", err)

	total := count(`q LIKE '` + reread + `'`)
	require.Equal(t, 1, total, "exactly one disambiguation re-read per refused LockAccount")
	require.Equal(t, 1, count(`held = TRUE AND q LIKE '`+reread+`'`),
		"the re-read must run inside the transaction, while the advisory lock is held (mutation: re-read after Commit / on another connection)")
	require.Zero(t, count(`held = FALSE AND q LIKE '`+reread+`'`))
}
