package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-D9-SUSPEND review round 3 (D9S-R3-1..4), PROVISIONAL. Real SQLite, real
// migrations, real repository and service.

const d9sPw = "D9sR3!Throwaway#1aa"

type d9sEnv struct {
	sqlDB *sql.DB
	repo  *repository.UserRepository
	svc   *AuthService
}

func newD9sEnv(t *testing.T) *d9sEnv {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := repository.NewUserRepository(db)
	// These tests probe login timing and state, not the back-off (CAT-LOGIN-LOCK-1): lift its free budget.
	repo.LoginThrottle().SetPolicy(repository.LoginThrottlePolicy{BucketFree: 100000, AccountFree: 100000})
	return &d9sEnv{sqlDB, repo, NewAuthService(repo, "test-jwt-secret-throwaway")}
}

func (e *d9sEnv) mkUser(t *testing.T, name string, role int) int {
	t.Helper()
	id, err := e.repo.Create(&models.User{Username: name, Email: name + "@example.test",
		RoleID: role, IsActive: true, Salt: "0123456789abcdef0123456789abcdef"})
	require.NoError(t, err)
	require.NoError(t, e.svc.ResetPassword(id, d9sPw))
	return id
}

func (e *d9sEnv) login(name, pw string) (*AuthResult, error) {
	return e.svc.Login(models.LoginRequest{Username: name, Password: pw}, "127.0.0.1", "test")
}

// R3-4: the account state must not be visible to someone who does not know the password.
func TestD9SR3_LoginStateNotEnumerable(t *testing.T) {
	cases := []struct {
		name  string
		setup string
		want  error
	}{
		{"suspended", `UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = ?`, ErrAccountSuspended},
		{"timed lock", `UPDATE users SET is_locked = 1, locked_until = '2999-01-01 00:00:00' WHERE id = ?`, ErrAccountLocked},
		{"deactivated", `UPDATE users SET is_active = 0 WHERE id = ?`, ErrAccountDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newD9sEnv(t)
			id := e.mkUser(t, "victim", 2)
			_, err := e.sqlDB.Exec(tc.setup, id)
			require.NoError(t, err)

			_, wrong := e.login("victim", "wrong-password-xx")
			assert.ErrorIs(t, wrong, ErrInvalidCredentials, "unauthenticated caller must not learn the account state")

			_, unknown := e.login("nobody-here", "wrong-password-xx")
			assert.Equal(t, unknown, wrong, "state account must be indistinguishable from an unknown user")

			_, right := e.login("victim", d9sPw)
			assert.ErrorIs(t, right, tc.want, "the password holder still gets the specific reason")
		})
	}
}

// R3-2/R3-3: RefreshToken on a suspended account.
func TestD9SR3_RefreshTokenSuspendedAndLocked(t *testing.T) {
	cases := []struct {
		name  string
		setup string
		want  error
	}{
		{"suspended", `UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = ?`, ErrAccountSuspended},
		{"timed lock", `UPDATE users SET is_locked = 1, locked_until = '2999-01-01 00:00:00' WHERE id = ?`, ErrAccountLocked},
		{"deactivated", `UPDATE users SET is_active = 0 WHERE id = ?`, ErrAccountDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newD9sEnv(t)
			id := e.mkUser(t, "refr", 2)
			res, err := e.login("refr", d9sPw)
			require.NoError(t, err)
			_, err = e.sqlDB.Exec(tc.setup, id)
			require.NoError(t, err)

			out, err := e.svc.RefreshToken(res.RefreshToken)
			assert.Nil(t, out)
			assert.True(t, errors.Is(err, tc.want), "got %v want %v", err, tc.want)
			var active int
			require.NoError(t, e.sqlDB.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = 1`, id).Scan(&active))
			assert.Equal(t, 0, active, "refusal must deactivate the session")
		})
	}
}

// Negative control: an unlocked user refreshes.
func TestD9SR3_RefreshTokenUnlockedWorks(t *testing.T) {
	e := newD9sEnv(t)
	e.mkUser(t, "okr", 2)
	res, err := e.login("okr", d9sPw)
	require.NoError(t, err)
	out, err := e.svc.RefreshToken(res.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, out)
}

// R3-1: a NULL is_locked (legacy / hand-edited row) must mean "unlocked" on every read and write path.
func TestD9SR3_NullIsLockedTreatedAsUnlockedEverywhere(t *testing.T) {
	e := newD9sEnv(t)
	id := e.mkUser(t, "nulllock", 2)
	_, err := e.sqlDB.Exec(`UPDATE users SET is_locked = NULL WHERE id = ?`, id)
	require.NoError(t, err)

	u, err := e.repo.GetByID(id)
	require.NoError(t, err, "GetByID must tolerate NULL is_locked")
	assert.False(t, u.IsLocked)
	_, err = e.repo.GetByUsernameOrEmail("nulllock")
	require.NoError(t, err)
	_, err = e.repo.GetByUsername("nulllock")
	require.NoError(t, err)
	_, err = e.repo.GetByEmail("nulllock@example.test")
	require.NoError(t, err)
	list, err := e.repo.List(10, 0)
	require.NoError(t, err, "List must tolerate NULL is_locked")
	assert.NotEmpty(t, list)

	res, err := e.login("nulllock", d9sPw)
	require.NoError(t, err, "NULL lock must not block login")
	require.NotNil(t, res)
	out, err := e.svc.RefreshToken(res.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, out)

	// the lock writers must not no-op on a NULL row (NOT (NULL = 1) is NULL in SQL)
	require.NoError(t, e.repo.LockAccount(id, time.Now().Add(time.Hour)))
	u, err = e.repo.GetByID(id)
	require.NoError(t, err)
	assert.True(t, u.IsAccountLocked(), "LockAccount must lock a NULL-lock row")
	require.NoError(t, e.repo.UnlockAccount(id))
	u, _ = e.repo.GetByID(id)
	assert.False(t, u.IsAccountLocked())

	_, err = e.sqlDB.Exec(`UPDATE users SET is_locked = NULL WHERE id = ?`, id)
	require.NoError(t, err)
	require.NoError(t, e.repo.UnlockAccount(id), "UnlockAccount on NULL row is not a suspension")
}

func TestD9SR3_NullLockPermissionsResolve(t *testing.T) {
	e := newD9sEnv(t)
	id := e.mkUser(t, "nullperm", 1)
	_, err := e.sqlDB.Exec(`UPDATE users SET is_locked = NULL WHERE id = ?`, id)
	require.NoError(t, err)
	ok, err := e.repo.HasPermission(context.Background(), id, "admin_all")
	require.NoError(t, err, "permission resolution must tolerate NULL is_locked")
	assert.True(t, ok, "a NULL is_locked row is not locked: its admin permission must resolve to true (D9SR3-F1)")
}

// R3-1: suspension counts usable admins consistently with NULL-lock peers.
func TestD9SR3_LastAdminGuardWithNullLockPeer(t *testing.T) {
	e := newD9sEnv(t)
	a := e.mkUser(t, "adm1", 1)
	b := e.mkUser(t, "adm2", 1)
	_, err := e.sqlDB.Exec(`UPDATE users SET is_locked = NULL WHERE id = ?`, b)
	require.NoError(t, err)
	// peer b is usable (NULL = unlocked): suspending a is allowed
	require.NoError(t, e.repo.SetSuspended(a, true))
	// now b is the last usable admin: must be refused
	assert.ErrorIs(t, e.repo.SetSuspended(b, true), repository.ErrLastAdmin)
}

// D9SR3-F2: a wrong password on a suspended / timed-locked / deactivated account still
// counts as a failed attempt (exactly +1); the correct password does not.
func TestD9SR3_WrongPasswordCountsOnInactiveAccounts(t *testing.T) {
	cases := []struct{ name, setup string }{
		{"suspended", `UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = ?`},
		{"timed lock", `UPDATE users SET is_locked = 1, locked_until = '2999-01-01 00:00:00' WHERE id = ?`},
		{"deactivated", `UPDATE users SET is_active = 0 WHERE id = ?`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newD9sEnv(t)
			id := e.mkUser(t, "counted", 2)
			_, err := e.sqlDB.Exec(tc.setup, id)
			require.NoError(t, err)
			count := func() int {
				var n int
				require.NoError(t, e.sqlDB.QueryRow(`SELECT failed_login_attempts FROM users WHERE id = ?`, id).Scan(&n))
				return n
			}
			before := count()
			_, werr := e.login("counted", "wrong-password-xx")
			assert.ErrorIs(t, werr, ErrInvalidCredentials)
			assert.Equal(t, before+1, count(), "wrong password must raise failed_login_attempts by exactly 1")
			_, _ = e.login("counted", d9sPw)
			assert.Equal(t, before+1, count(), "the correct password must not raise failed_login_attempts")
		})
	}
}

// D9SR3-F4: the failed_login_inactive audit event is written AFTER the password check and
// carries its outcome; exactly one event per attempt.
func TestD9SR3_InactiveAuditWrittenWithOutcome(t *testing.T) {
	e := newD9sEnv(t)
	id := e.mkUser(t, "audited", 2)
	_, err := e.sqlDB.Exec(`UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = ?`, id)
	require.NoError(t, err)
	type ev struct {
		uid          int
		typ, details string
	}
	var got []ev
	e.svc.audit = func(uid int, typ, ip, ua, details string) { got = append(got, ev{uid, typ, details}) }

	_, _ = e.login("audited", "wrong-password-xx")
	require.Len(t, got, 1, "exactly one audit event per attempt")
	assert.Equal(t, ev{id, "failed_login_inactive", "account not loginable: invalid password"}, got[0])

	got = nil
	_, _ = e.login("audited", d9sPw)
	require.Len(t, got, 1, "exactly one audit event per attempt")
	assert.Equal(t, ev{id, "failed_login_inactive", "account not loginable: password correct"}, got[0])
}
