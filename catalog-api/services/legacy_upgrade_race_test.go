package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"testing"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F5: the upgrade-on-login write is a compare-and-swap on the legacy hash. A
// password reset that lands between the legacy verification and the upgrade
// write must NOT be reverted to the old password.
func TestLegacyUpgrade_DoesNotRevertConcurrentReset(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := repository.NewUserRepository(db)
	svc := NewAuthService(repo, "test-jwt-secret-throwaway")

	const oldPw, newPw = "Old!Throwaway#1aa", "New!Throwaway#2bb"
	salt := "0123456789abcdef0123456789abcdef"
	sum := sha256.Sum256([]byte(oldPw + salt))
	id, err := repo.Create(&models.User{Username: "racer", Email: "racer@example.test",
		PasswordHash: hex.EncodeToString(sum[:]), Salt: salt, RoleID: 2, IsActive: true})
	require.NoError(t, err)

	legacyUpgradeHook = func() { require.NoError(t, svc.ResetPassword(id, newPw)) }
	t.Cleanup(func() { legacyUpgradeHook = nil })

	_, err = svc.Login(models.LoginRequest{Username: "racer", Password: oldPw}, "127.0.0.1", "test")
	require.NoError(t, err, "the login itself succeeded on the legacy hash")
	legacyUpgradeHook = nil

	u, err := repo.GetByID(id)
	require.NoError(t, err)
	ok, _ := CheckPassword(newPw, u.Salt, u.PasswordHash)
	assert.True(t, ok, "the reset password must survive the racing upgrade")
	ok, _ = CheckPassword(oldPw, u.Salt, u.PasswordHash)
	assert.False(t, ok, "the old password must NOT be restored")
	assert.True(t, strings.HasPrefix(u.PasswordHash, "$cz1$"))
}

func TestUpdatePasswordIfHash_OnlyWhenHashMatches(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := repository.NewUserRepository(db)
	id, err := repo.Create(&models.User{Username: "cas", Email: "cas@example.test", PasswordHash: "h1", Salt: "s", RoleID: 2, IsActive: true})
	require.NoError(t, err)
	swapped, err := repo.UpdatePasswordIfHash(id, "h2", "s", "stale")
	require.NoError(t, err)
	assert.False(t, swapped)
	swapped, err = repo.UpdatePasswordIfHash(id, "h2", "s", "h1")
	require.NoError(t, err)
	assert.True(t, swapped)
	u, _ := repo.GetByID(id)
	assert.Equal(t, "h2", u.PasswordHash)
}
