package repository

import (
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockUserRepo(t *testing.T) (*UserRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewUserRepository(db), mock
}

// userColumns returns the standard column set for user queries.
var userColumns = []string{
	"id", "username", "email", "password_hash", "salt", "role_id",
	"first_name", "last_name", "display_name", "avatar_url",
	"time_zone", "language", "is_active", "is_locked",
	"locked_until", "failed_login_attempts", "last_login_at", "last_login_ip",
	"created_at", "updated_at", "settings",
}

func sampleUserRow(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(userColumns).AddRow(
		1, "testuser", "test@example.com", "hashvalue", "saltvalue", 1,
		"John", "Doe", "JohnD", nil,
		"UTC", "en", true, false,
		nil, 0, nil, nil,
		now, now, nil,
	)
}

func sampleRoleRow(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "name", "description", "permissions", "is_system", "created_at", "updated_at",
	}).AddRow(
		1, "admin", "Administrator role", `["users.create","users.delete","media.manage"]`, true, now, now,
	)
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func TestUserRepository_GetByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		id      int
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		check   func(t *testing.T, user *models.User)
	}{
		{
			name: "success",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE id").
					WithArgs(1).
					WillReturnRows(sampleUserRow(now))
			},
			check: func(t *testing.T, user *models.User) {
				assert.Equal(t, 1, user.ID)
				assert.Equal(t, "testuser", user.Username)
				assert.Equal(t, "test@example.com", user.Email)
				assert.True(t, user.IsActive)
			},
		},
		{
			name: "not found",
			id:   999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE id").
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
		},
		{
			name: "with settings JSON",
			id:   2,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE id").
					WithArgs(2).
					WillReturnRows(sqlmock.NewRows(userColumns).AddRow(
						2, "admin", "admin@example.com", "hash", "salt", 1,
						"Admin", "User", "Admin", nil,
						"UTC", "en", true, false,
						nil, 0, nil, nil,
						now, now, `{"theme":"dark"}`,
					))
			},
			check: func(t *testing.T, user *models.User) {
				assert.Equal(t, 2, user.ID)
				assert.Equal(t, `{"theme":"dark"}`, user.Settings)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			user, err := repo.GetByID(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			tt.check(t, user)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByUsername
// ---------------------------------------------------------------------------

func TestUserRepository_GetByUsername(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		username string
		setup    func(mock sqlmock.Sqlmock)
		wantErr  bool
		check    func(t *testing.T, user *models.User)
	}{
		{
			name:     "success",
			username: "testuser",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE username").
					WithArgs("testuser").
					WillReturnRows(sampleUserRow(now))
			},
			check: func(t *testing.T, user *models.User) {
				assert.Equal(t, "testuser", user.Username)
			},
		},
		{
			name:     "not found",
			username: "nonexistent",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE username").
					WithArgs("nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			user, err := repo.GetByUsername(tt.username)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			tt.check(t, user)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestUserRepository_Create(t *testing.T) {
	tests := []struct {
		name    string
		user    *models.User
		setup   func(mock sqlmock.Sqlmock)
		wantID  int
		wantErr bool
	}{
		{
			name: "success",
			user: &models.User{
				Username:     "newuser",
				Email:        "new@example.com",
				PasswordHash: "hash",
				Salt:         "salt",
				RoleID:       1,
				IsActive:     true,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO users").
					WithArgs(
						"newuser", "new@example.com", "hash", "salt", 1,
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), true,
						sqlmock.AnyArg(), sqlmock.AnyArg(),
					).
					WillReturnResult(sqlmock.NewResult(42, 1))
			},
			wantID: 42,
		},
		{
			name: "database error",
			user: &models.User{
				Username: "dupuser",
				Email:    "dup@example.com",
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO users").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			id, err := repo.Create(tt.user)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestUserRepository_Update(t *testing.T) {
	tests := []struct {
		name    string
		user    *models.User
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			user: &models.User{
				ID:       1,
				Username: "updated",
				Email:    "updated@example.com",
				IsActive: true,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE users SET").
					WithArgs(
						"updated", "updated@example.com",
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
						true, "",
						0, sqlmock.AnyArg(), 1,
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "database error",
			user: &models.User{ID: 1},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE users SET").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			err := repo.Update(tt.user)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestUserRepository_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM users WHERE id").
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "database error",
			id:   999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM users WHERE id").
					WithArgs(999).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			err := repo.Delete(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdatePassword
// ---------------------------------------------------------------------------

func TestUserRepository_UpdatePassword(t *testing.T) {
	tests := []struct {
		name    string
		userID  int
		hash    string
		salt    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:   "success",
			userID: 1,
			hash:   "newhash",
			salt:   "newsalt",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE users SET password_hash").
					WithArgs("newhash", "newsalt", sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			err := repo.UpdatePassword(tt.userID, tt.hash, tt.salt)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Count
// ---------------------------------------------------------------------------

func TestUserRepository_Count(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "returns count",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
			},
			wantCount: 5,
		},
		{
			name: "empty table",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			count, err := repo.Count()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, count)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestUserRepository_List(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		limit   int
		offset  int
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:   "returns users",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(userColumns).
					AddRow(1, "user1", "u1@example.com", "hash", "salt", 1,
						"First", "Last", "User1", nil, "UTC", "en", true, false,
						nil, 0, nil, nil, now, now, nil).
					AddRow(2, "user2", "u2@example.com", "hash", "salt", 1,
						"First2", "Last2", "User2", nil, "UTC", "en", true, false,
						nil, 0, nil, nil, now, now, nil)
				mock.ExpectQuery("SELECT .+ FROM users").
					WithArgs(10, 0).
					WillReturnRows(rows)
			},
			want: 2,
		},
		{
			name:   "empty result",
			limit:  10,
			offset: 100,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users").
					WithArgs(10, 100).
					WillReturnRows(sqlmock.NewRows(userColumns))
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			users, err := repo.List(tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, users, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// LockAccount / UnlockAccount
// ---------------------------------------------------------------------------

func TestUserRepository_LockUnlockAccount(t *testing.T) {
	t.Run("lock account", func(t *testing.T) {
		repo, mock := newMockUserRepo(t)
		lockUntil := time.Now().Add(1 * time.Hour)
		mock.ExpectExec("UPDATE users SET is_locked = 1").
			WithArgs(lockUntil, 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.LockAccount(1, lockUntil)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("unlock account", func(t *testing.T) {
		repo, mock := newMockUserRepo(t)
		mock.ExpectExec("UPDATE users SET is_locked = 0").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.UnlockAccount(1)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// IncrementFailedLoginAttempts / ResetFailedLoginAttempts
// ---------------------------------------------------------------------------

func TestUserRepository_FailedLoginAttempts(t *testing.T) {
	t.Run("increment", func(t *testing.T) {
		repo, mock := newMockUserRepo(t)
		mock.ExpectExec("UPDATE users SET failed_login_attempts = failed_login_attempts").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.IncrementFailedLoginAttempts(1)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("reset", func(t *testing.T) {
		repo, mock := newMockUserRepo(t)
		mock.ExpectExec("UPDATE users SET failed_login_attempts = 0").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.ResetFailedLoginAttempts(1)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// GetByEmail
// ---------------------------------------------------------------------------

func TestUserRepository_GetByEmail(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		email   string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		check   func(t *testing.T, user *models.User)
	}{
		{
			name:  "success",
			email: "test@example.com",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE email").
					WithArgs("test@example.com").
					WillReturnRows(sampleUserRow(now))
			},
			check: func(t *testing.T, user *models.User) {
				assert.Equal(t, "test@example.com", user.Email)
			},
		},
		{
			name:  "not found",
			email: "missing@example.com",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE email").
					WithArgs("missing@example.com").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			user, err := repo.GetByEmail(tt.email)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			tt.check(t, user)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByUsernameOrEmail
// ---------------------------------------------------------------------------

func TestUserRepository_GetByUsernameOrEmail(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name            string
		usernameOrEmail string
		setup           func(mock sqlmock.Sqlmock)
		wantErr         bool
		check           func(t *testing.T, user *models.User)
	}{
		{
			name:            "success with username",
			usernameOrEmail: "testuser",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE username = \\? OR email = \\?").
					WithArgs("testuser", "testuser").
					WillReturnRows(sampleUserRow(now))
			},
			check: func(t *testing.T, user *models.User) {
				assert.Equal(t, "testuser", user.Username)
				assert.Equal(t, "test@example.com", user.Email)
			},
		},
		{
			name:            "success with email",
			usernameOrEmail: "test@example.com",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE username = \\? OR email = \\?").
					WithArgs("test@example.com", "test@example.com").
					WillReturnRows(sampleUserRow(now))
			},
			check: func(t *testing.T, user *models.User) {
				assert.Equal(t, "testuser", user.Username)
				assert.Equal(t, "test@example.com", user.Email)
			},
		},
		{
			name:            "not found",
			usernameOrEmail: "nonexistent",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE username = \\? OR email = \\?").
					WithArgs("nonexistent", "nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
		},
		{
			name:            "database error",
			usernameOrEmail: "testuser",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM users WHERE username = \\? OR email = \\?").
					WithArgs("testuser", "testuser").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			user, err := repo.GetByUsernameOrEmail(tt.usernameOrEmail)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			tt.check(t, user)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateLastLogin
// ---------------------------------------------------------------------------

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	tests := []struct {
		name      string
		userID    int
		ipAddress string
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:      "success",
			userID:    1,
			ipAddress: "192.168.1.100",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE users SET last_login_at = \\?, last_login_ip = \\? WHERE id = \\?").
					WithArgs(sqlmock.AnyArg(), "192.168.1.100", 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:      "database error",
			userID:    1,
			ipAddress: "192.168.1.100",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE users SET last_login_at = \\?, last_login_ip = \\? WHERE id = \\?").
					WithArgs(sqlmock.AnyArg(), "192.168.1.100", 1).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			err := repo.UpdateLastLogin(tt.userID, tt.ipAddress)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetRole
// ---------------------------------------------------------------------------

func TestUserRepository_GetRole(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		roleID  int
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
		check   func(t *testing.T, role *models.Role)
	}{
		{
			name:   "success",
			roleID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM roles WHERE id = \\?").
					WithArgs(1).
					WillReturnRows(sampleRoleRow(now))
			},
			check: func(t *testing.T, role *models.Role) {
				assert.Equal(t, 1, role.ID)
				assert.Equal(t, "admin", role.Name)
				assert.Equal(t, "Administrator role", *role.Description)
				assert.Equal(t, true, role.IsSystem)
				assert.Len(t, role.Permissions, 3)
				assert.Contains(t, role.Permissions, "users.create")
				assert.Contains(t, role.Permissions, "users.delete")
				assert.Contains(t, role.Permissions, "media.manage")
			},
		},
		{
			name:   "not found",
			roleID: 999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM roles WHERE id = \\?").
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
			errMsg:  "role not found",
		},
		{
			name:   "database error",
			roleID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM roles WHERE id = \\?").
					WithArgs(1).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
		{
			name:   "invalid permissions JSON",
			roleID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "permissions", "is_system", "created_at", "updated_at",
				}).AddRow(
					1, "admin", "Administrator role", "invalid json", true, now, now,
				)
				mock.ExpectQuery("SELECT .+ FROM roles WHERE id = \\?").
					WithArgs(1).
					WillReturnRows(rows)
			},
			wantErr: true,
			errMsg:  "failed to unmarshal permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			role, err := repo.GetRole(tt.roleID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, role)
			tt.check(t, role)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
