package services

import (
	"context"
	"database/sql"
	"testing"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// G1: Login reads the user, spends ~57 ms in bcrypt, then inserts the session.
// Anything that revokes sessions (role change, deactivation, password change,
// admin reset) during that window revokes nothing the login holds, because the
// session does not exist yet. Login must re-read the row after creating the
// session and refuse (and deactivate) it when the account changed. The hook
// forces the interleaving deterministically on a real SQLite database.
func TestLogin_RefusesSessionMintedFromStaleUserState(t *testing.T) {
	const pw = "Stale!Throwaway#1aa"
	cases := []struct {
		name   string
		change func(t *testing.T, db *sql.DB, svc *AuthService, id int)
	}{
		{"role demotion", func(t *testing.T, db *sql.DB, _ *AuthService, id int) {
			_, err := db.Exec(`UPDATE users SET role_id = 3 WHERE id = ?`, id)
			require.NoError(t, err)
		}},
		{"deactivation", func(t *testing.T, db *sql.DB, _ *AuthService, id int) {
			_, err := db.Exec(`UPDATE users SET is_active = 0 WHERE id = ?`, id)
			require.NoError(t, err)
		}},
		{"lock", func(t *testing.T, db *sql.DB, _ *AuthService, id int) {
			_, err := db.Exec(`UPDATE users SET is_locked = 1 WHERE id = ?`, id)
			require.NoError(t, err)
		}},
		{"password change / admin reset", func(t *testing.T, _ *sql.DB, svc *AuthService, id int) {
			require.NoError(t, svc.ResetPassword(id, "Other!Throwaway#2bb"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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

			loginPostSessionHook = func() { tc.change(t, sqlDB, svc, id) }
			t.Cleanup(func() { loginPostSessionHook = nil })

			res, err := svc.Login(models.LoginRequest{Username: "stale", Password: pw}, "127.0.0.1", "test")
			loginPostSessionHook = nil
			require.Error(t, err)
			assert.Nil(t, res)

			var active int
			require.NoError(t, sqlDB.QueryRow(
				`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = 1`, id).Scan(&active))
			assert.Equal(t, 0, active, "the session minted from stale state must be deactivated")
		})
	}
}

// Negative control: with no concurrent change the same login succeeds and its
// session stays active (the re-check must not be a blanket refusal).
func TestLogin_UnchangedUserStillLogsIn(t *testing.T) {
	const pw = "Stale!Throwaway#1aa"
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := repository.NewUserRepository(db)
	svc := NewAuthService(repo, "test-jwt-secret-throwaway")
	id, err := repo.Create(&models.User{Username: "ok", Email: "ok@example.test",
		RoleID: 2, IsActive: true, Salt: "0123456789abcdef0123456789abcdef"})
	require.NoError(t, err)
	require.NoError(t, svc.ResetPassword(id, pw))

	hooked := false
	loginPostSessionHook = func() { hooked = true }
	t.Cleanup(func() { loginPostSessionHook = nil })
	res, err := svc.Login(models.LoginRequest{Username: "ok", Password: pw}, "127.0.0.1", "test")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, hooked)
	var active int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = 1`, id).Scan(&active))
	assert.Equal(t, 1, active)
}
