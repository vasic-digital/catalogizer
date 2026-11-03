package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const permQuery = `SELECT u.is_active, ro.permissions`

func TestUserRepository_HasPermission(t *testing.T) {
	cases := []struct {
		name       string
		active     bool
		perms      string
		permission string
		want       bool
	}{
		{"role carries the permission", true, `["storage.manage"]`, "storage.manage", true},
		{"wildcard role", true, `["*"]`, "storage.manage", true},
		{"system.admin passes any permission", true, `["system.admin"]`, "storage.manage", true},
		{"prefix wildcard on a segment boundary", true, `["storage.*"]`, "storage.manage", true},
		{"default user role is refused", true, `["media.view","media.download"]`, "storage.manage", false},
		{"similar permission is not enough", true, `["storage.view"]`, "storage.manage", false},
		{"unsafe prefix wildcard is refused", true, `["stor*"]`, "storage.manage", false},
		{"deactivated user with the permission is refused", false, `["storage.manage"]`, "storage.manage", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			mock.ExpectQuery(permQuery).WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"is_active", "permissions", "is_locked", "locked_until"}).AddRow(c.active, c.perms, false, nil))
			got, err := repo.HasPermission(context.Background(), 7, c.permission)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserRepository_HasPermission_UnknownUserAndFailuresFailClosed(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	mock.ExpectQuery(permQuery).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"is_active", "permissions", "is_locked", "locked_until"}))
	got, err := repo.HasPermission(context.Background(), 9, "storage.manage")
	require.NoError(t, err)
	assert.False(t, got, "unknown user or dangling role is not entitled")

	repo, mock = newMockUserRepo(t)
	mock.ExpectQuery(permQuery).WithArgs(9).WillReturnError(errors.New("db down"))
	got, err = repo.HasPermission(context.Background(), 9, "storage.manage")
	assert.Error(t, err, "a read failure is reported so the gate answers 503, never allows")
	assert.False(t, got)

	repo, mock = newMockUserRepo(t)
	mock.ExpectQuery(permQuery).WithArgs(9).WillReturnRows(sqlmock.NewRows([]string{"is_active", "permissions", "is_locked", "locked_until"}).AddRow(true, `not json`, false, nil))
	got, err = repo.HasPermission(context.Background(), 9, "storage.manage")
	assert.Error(t, err)
	assert.False(t, got)
}

// F11: a locked account is refused like a deactivated one (permanent lock, or lock with an
// unexpired locked_until); an expired lock no longer blocks. Applies to IsAdmin too.
func TestUserRepository_HasPermission_LockedAccount(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	cases := []struct {
		name   string
		locked bool
		until  interface{}
		want   bool
	}{
		{"permanent lock", true, nil, false},
		{"lock not yet expired", true, future, false},
		{"lock expired", true, past, true},
		{"not locked, stale locked_until", false, future, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			rows := func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"is_active", "permissions", "is_locked", "locked_until"}).AddRow(true, `["*"]`, c.locked, c.until)
			}
			mock.ExpectQuery(permQuery).WithArgs(7).WillReturnRows(rows())
			got, err := repo.HasPermission(context.Background(), 7, "storage.manage")
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
			mock.ExpectQuery(permQuery).WithArgs(7).WillReturnRows(rows())
			adm, err := repo.IsAdmin(context.Background(), 7)
			require.NoError(t, err)
			assert.Equal(t, c.want, adm, "IsAdmin must agree")
		})
	}
}
