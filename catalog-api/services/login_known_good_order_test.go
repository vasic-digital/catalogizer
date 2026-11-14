package services

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-LOGIN-LOCK-1 (N8): a login refused because the account changed while it ran (the loginPostSessionHook seam) must not
// leave the client address marked known-good; only an ACCEPTED login earns the account-block bypass. Unit-class evidence.
func TestLogin_RefusedByConcurrentChangeDoesNotMarkAddressKnownGood(t *testing.T) {
	const pw = "Vict1m!Throwaway#9"
	for _, accepted := range []bool{false, true} {
		t.Run(fmt.Sprintf("accepted=%v", accepted), func(t *testing.T) {
			sqlDB, err := sql.Open("sqlite3", ":memory:")
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			sqlDB.SetMaxOpenConns(1)
			db := database.WrapDB(sqlDB, database.DialectSQLite)
			require.NoError(t, db.RunMigrations(context.Background()))
			repo := repository.NewUserRepository(db) // default policy: 5 per address, 20 per account
			svc := NewAuthService(repo, "test-jwt-secret-throwaway")
			id, err := repo.Create(&models.User{Username: "nk", Email: "nk@example.test", RoleID: 2, IsActive: true, Salt: "0123456789abcdef0123456789abcdef"})
			require.NoError(t, err)
			require.NoError(t, svc.ResetPassword(id, pw))

			if !accepted {
				// A deactivation (a password change would also clear the back-off rows, which would hide the ordering).
				loginPostSessionHook = func() {
					_, uerr := sqlDB.Exec(`UPDATE users SET is_active = 0 WHERE id = ?`, id)
					require.NoError(t, uerr)
				}
				t.Cleanup(func() { loginPostSessionHook = nil })
			}
			_, err = svc.Login(models.LoginRequest{Username: "nk", Password: pw}, "198.51.100.7", "test")
			loginPostSessionHook = nil
			assert.Equal(t, accepted, err == nil)

			// Block the account from other addresses, then try this address with a wrong password.
			for i := 0; i < 20; i++ {
				_, _ = svc.Login(models.LoginRequest{Username: "nk", Password: "wrong-guess"}, fmt.Sprintf("10.9.%d.1", i), "test")
			}
			_, err = svc.Login(models.LoginRequest{Username: "nk", Password: "wrong-again"}, "198.51.100.7", "test")
			var throttled *ErrLoginThrottled
			if accepted {
				assert.ErrorIs(t, err, ErrInvalidCredentials, "an accepted login made the address known-good: it passes the account block")
			} else {
				assert.ErrorAs(t, err, &throttled, "a refused login must not make the address known-good")
			}
		})
	}
}
