package repository

// PGLA-X1: three last-administrator holes left by CAT-PG-LASTADMIN-1.
//   1. UpdateRole could strip system.admin from the custom role of the last administrator.
//   2. UpdateUserSettings saved a stale full-row read through the unguarded Update (reverting a concurrent role change).
//   3. Every guarded write read adminRoleIDs before taking the PostgreSQL advisory lock.
// Items 1 and 2 run on real SQLite with the real schema; item 3 asserts statement ORDER against the PostgreSQL dialect (unit class).
import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func pglaX1Repo(t *testing.T) (*UserRepository, *database.DB) {
	db, err := database.NewConnection(&config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "x1.db")})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.RunMigrations(context.Background()))
	_, err = db.Exec(`UPDATE users SET role_id = 2`) // seeded administrators would mask the invariant
	require.NoError(t, err)
	return NewUserRepository(db), db
}

func pglaX1Role(t *testing.T, repo *UserRepository, name string, perms ...string) int {
	id, err := repo.CreateRole(&models.Role{Name: name, Permissions: models.Permissions(perms)})
	require.NoError(t, err)
	return id
}

func pglaX1User(t *testing.T, repo *UserRepository, name string, roleID int, active bool) int {
	id, err := repo.Create(&models.User{Username: name, Email: name + "@x.test", RoleID: roleID, IsActive: active, Salt: "s", PasswordHash: "h"})
	require.NoError(t, err)
	return id
}

func pglaX1Perms(t *testing.T, repo *UserRepository, roleID int) models.Permissions {
	r, err := repo.GetRole(roleID)
	require.NoError(t, err)
	return r.Permissions
}

func TestPGLAX1_UpdateRole_CannotStripLastAdministrator(t *testing.T) {
	repo, _ := pglaX1Repo(t)
	ops := pglaX1Role(t, repo, "ops", "system.admin")
	pglaX1User(t, repo, "only", ops, true)

	err := repo.UpdateRole(&models.Role{ID: ops, Name: "ops", Permissions: models.Permissions{"media.view"}})
	require.ErrorIs(t, err, ErrLastAdmin)
	require.Equal(t, models.Permissions{"system.admin"}, pglaX1Perms(t, repo, ops), "the refused write changed nothing")
}

// negative controls: behaviour for everything that is not "removes the last administrator" is unchanged.
func TestPGLAX1_UpdateRole_NegativeControls(t *testing.T) {
	t.Run("another active administrator in a different admin role exists", func(t *testing.T) {
		repo, _ := pglaX1Repo(t)
		ops := pglaX1Role(t, repo, "ops", "system.admin")
		other := pglaX1Role(t, repo, "other", "system.admin")
		pglaX1User(t, repo, "a", ops, true)
		pglaX1User(t, repo, "b", other, true)
		require.NoError(t, repo.UpdateRole(&models.Role{ID: ops, Name: "ops", Permissions: models.Permissions{"media.view"}}))
		require.Equal(t, models.Permissions{"media.view"}, pglaX1Perms(t, repo, ops))
	})
	t.Run("the only other administrator is inactive", func(t *testing.T) {
		repo, _ := pglaX1Repo(t)
		ops := pglaX1Role(t, repo, "ops", "system.admin")
		other := pglaX1Role(t, repo, "other", "system.admin")
		pglaX1User(t, repo, "a", ops, true)
		pglaX1User(t, repo, "b", other, false)
		require.ErrorIs(t, repo.UpdateRole(&models.Role{ID: ops, Name: "ops", Permissions: models.Permissions{"media.view"}}), ErrLastAdmin)
	})
	t.Run("role keeps system.admin", func(t *testing.T) {
		repo, _ := pglaX1Repo(t)
		ops := pglaX1Role(t, repo, "ops", "system.admin")
		pglaX1User(t, repo, "a", ops, true)
		require.NoError(t, repo.UpdateRole(&models.Role{ID: ops, Name: "ops2", Permissions: models.Permissions{"system.admin", "media.view"}}))
	})
	t.Run("role is not an admin role, no administrator exists at all", func(t *testing.T) {
		repo, _ := pglaX1Repo(t)
		plain := pglaX1Role(t, repo, "plain", "media.view")
		pglaX1User(t, repo, "a", plain, true)
		require.NoError(t, repo.UpdateRole(&models.Role{ID: plain, Name: "plain", Permissions: models.Permissions{"media.edit"}}))
	})
	t.Run("admin role with no active member", func(t *testing.T) {
		repo, _ := pglaX1Repo(t)
		ops := pglaX1Role(t, repo, "ops", "system.admin")
		pglaX1User(t, repo, "a", ops, false)
		require.NoError(t, repo.UpdateRole(&models.Role{ID: ops, Name: "ops", Permissions: models.Permissions{"media.view"}}))
	})
	t.Run("system role stays refused with the old message", func(t *testing.T) {
		repo, _ := pglaX1Repo(t)
		err := repo.UpdateRole(&models.Role{ID: 1, Name: "x", Permissions: models.Permissions{"media.view"}})
		require.Error(t, err)
		require.False(t, errors.Is(err, ErrLastAdmin))
		require.Contains(t, err.Error(), "role not found or is system role")
	})
	t.Run("missing role keeps the old message", func(t *testing.T) {
		repo, _ := pglaX1Repo(t)
		err := repo.UpdateRole(&models.Role{ID: 9999, Name: "x", Permissions: models.Permissions{"media.view"}})
		require.Error(t, err)
		require.False(t, errors.Is(err, ErrLastAdmin))
	})
}

// Item 2: settings are written without carrying role_id / is_active from a possibly stale read.
func TestPGLAX1_UpdateSettings_DoesNotRevertConcurrentRoleChange(t *testing.T) {
	repo, db := pglaX1Repo(t)
	ops := pglaX1Role(t, repo, "ops", "system.admin")
	id := pglaX1User(t, repo, "u", ops, true)
	stale, err := repo.GetByID(id) // the handler's read
	require.NoError(t, err)
	// a concurrent administrator demotes and deactivates the user
	_, err = db.Exec(`UPDATE users SET role_id = 2, is_active = 0 WHERE id = ?`, id)
	require.NoError(t, err)

	require.NoError(t, repo.UpdateSettings(stale.ID, `{"theme":"dark"}`))

	var rid int
	var act bool
	var settings string
	require.NoError(t, db.QueryRow(`SELECT role_id, is_active, COALESCE(settings,'') FROM users WHERE id = ?`, id).Scan(&rid, &act, &settings))
	require.Equal(t, 2, rid, "role not reverted")
	require.False(t, act, "deactivation not reverted")
	require.Equal(t, `{"theme":"dark"}`, settings)

	// control: the old path (Update with the stale row) DID revert both, which is the defect.
	stale.Settings = `{"theme":"light"}`
	require.NoError(t, repo.Update(stale))
	require.NoError(t, db.QueryRow(`SELECT role_id, is_active FROM users WHERE id = ?`, id).Scan(&rid, &act))
	require.Equal(t, ops, rid, "documents the defect of the unguarded Update")

	require.ErrorIs(t, repo.UpdateSettings(99999, "{}"), sql.ErrNoRows)
}

// Item 3: on PostgreSQL the administrator-role set is read INSIDE the transaction, after the advisory lock.
func TestPGLAX1_AdminRoleIDsReadAfterAdvisoryLock(t *testing.T) {
	newPG := func(t *testing.T) (*UserRepository, sqlmock.Sqlmock) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		return NewUserRepository(database.WrapDB(sqlDB, database.DialectPostgres)), mock
	}
	roles := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "permissions"}).AddRow(1, `["system.admin"]`)
	}
	t.Run("UpdateGuarded", func(t *testing.T) {
		repo, mock := newPG(t)
		mock.ExpectBegin()
		mock.ExpectExec("pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("FROM roles").WillReturnRows(roles())
		mock.ExpectExec("UPDATE users SET").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		ok, err := repo.UpdateGuarded(&models.User{ID: 7, RoleID: 2, IsActive: true}, 1, true)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("LockAccount", func(t *testing.T) {
		repo, mock := newPG(t)
		mock.ExpectBegin()
		mock.ExpectExec("pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("FROM roles").WillReturnRows(roles())
		mock.ExpectExec("UPDATE users SET is_locked").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectExec("UPDATE user_sessions").WillReturnResult(sqlmock.NewResult(0, 0))
		require.NoError(t, repo.LockAccount(7, time.Now().Add(time.Hour)))
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("SetSuspended", func(t *testing.T) {
		repo, mock := newPG(t)
		mock.ExpectBegin()
		mock.ExpectExec("pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("FROM roles").WillReturnRows(roles())
		mock.ExpectExec("UPDATE users SET is_locked").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		require.NoError(t, repo.SetSuspended(7, true))
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("Delete", func(t *testing.T) {
		repo, mock := newPG(t)
		mock.ExpectBegin()
		mock.ExpectExec("pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("FROM roles").WillReturnRows(roles())
		mock.ExpectQuery("SELECT username FROM users").WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("u"))
		mock.ExpectExec("UPDATE auth_audit_log SET details").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("UPDATE auth_audit_log SET user_id").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("UPDATE user_permissions SET granted_by").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("SELECT role_id, is_active FROM users").WillReturnRows(sqlmock.NewRows([]string{"role_id", "is_active"}).AddRow(2, true))
		mock.ExpectExec("DELETE FROM users").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		require.NoError(t, repo.Delete(7))
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("UpdateRole", func(t *testing.T) {
		repo, mock := newPG(t)
		mock.ExpectBegin()
		mock.ExpectExec("pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("FROM roles").WillReturnRows(roles())
		mock.ExpectQuery("FROM roles WHERE id").WithArgs(5).WillReturnRows(
			sqlmock.NewRows([]string{"permissions", "is_system"}).AddRow(`["system.admin"]`, false))
		mock.ExpectExec("UPDATE roles SET").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		require.NoError(t, repo.UpdateRole(&models.Role{ID: 5, Name: "n", Permissions: models.Permissions{"media.view"}}))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// PGLAX1-F1: locked members are not counted as effective administrators (SQLite, real schema).
func TestPGLAX1F1_UpdateRole_LockedMembers(t *testing.T) {
	lock := func(t *testing.T, db *database.DB, id int) {
		_, err := db.Exec(`UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = ?`, id)
		require.NoError(t, err)
	}
	t.Run("the only active member of the stripped role is locked: strip allowed", func(t *testing.T) {
		repo, db := pglaX1Repo(t)
		ops := pglaX1Role(t, repo, "ops", "system.admin")
		other := pglaX1Role(t, repo, "other", "system.admin")
		locked := pglaX1User(t, repo, "locked", ops, true)
		lock(t, db, locked)
		pglaX1User(t, repo, "keeper", other, true)
		require.NoError(t, repo.UpdateRole(&models.Role{ID: ops, Name: "ops", Permissions: models.Permissions{"media.view"}}))
		require.Equal(t, models.Permissions{"media.view"}, pglaX1Perms(t, repo, ops))
	})
	t.Run("the only other administrator is locked: strip refused", func(t *testing.T) {
		repo, db := pglaX1Repo(t)
		ops := pglaX1Role(t, repo, "ops", "system.admin")
		other := pglaX1Role(t, repo, "other", "system.admin")
		pglaX1User(t, repo, "a", ops, true)
		lockedOther := pglaX1User(t, repo, "b", other, true)
		lock(t, db, lockedOther)
		require.ErrorIs(t, repo.UpdateRole(&models.Role{ID: ops, Name: "ops", Permissions: models.Permissions{"media.view"}}), ErrLastAdmin)
		require.Equal(t, models.Permissions{"system.admin"}, pglaX1Perms(t, repo, ops))
	})
}

// PGLAX1-F5: an unreadable role row must not read as "not an administrator role" (fail closed, section 11.4.252).
func TestPGLAX1F5_UpdateRole_UnparsablePermissionsFailsClosed(t *testing.T) {
	repo, db := pglaX1Repo(t)
	ops := pglaX1Role(t, repo, "ops", "system.admin")
	pglaX1User(t, repo, "only", ops, true)
	_, err := db.Exec(`UPDATE roles SET permissions = ? WHERE id = ?`, "not json", ops)
	require.NoError(t, err)

	err = repo.UpdateRole(&models.Role{ID: ops, Name: "ops", Permissions: models.Permissions{"media.view"}})
	require.Error(t, err, "an unreadable current role must refuse the strip, not proceed unguarded")
	var got string
	require.NoError(t, db.QueryRow(`SELECT permissions FROM roles WHERE id = ?`, ops).Scan(&got))
	require.Equal(t, "not json", got, "the refused write changed nothing")

	// control: a role id that does not exist is still the old not-found path (ErrNoRows means not-admin).
	err = repo.UpdateRole(&models.Role{ID: 9999, Name: "x", Permissions: models.Permissions{"media.view"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "role not found or is system role")
}
