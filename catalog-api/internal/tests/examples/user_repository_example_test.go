package examples

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalogizer/database"
	"catalogizer/internal/tests/testutils"
	"catalogizer/models"
	"catalogizer/repository"
)

// TestUserRepositoryExample demonstrates how to test a repository
func TestUserRepositoryExample(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Wrap with database.DB
	dialect := database.DialectSQLite
	wrappedDB := database.WrapDB(db, dialect)

	// Create repository with mock database
	userRepo := repository.NewUserRepository(wrappedDB)

	// Test 1: Create user
	t.Run("CreateUser", func(t *testing.T) {
		// Setup expectations - note the SQLite dialect will rewrite placeholders
		// For SQLite, we expect '?' placeholders
		mock.ExpectExec(`INSERT INTO users`).
			WithArgs("testuser", "test@example.com", "hashed_password", "salt", 1, "Test", "User", "Test User", nil, nil, nil, true, sqlmock.AnyArg(), sqlmock.AnyArg(), "{}").
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Execute
		user := &models.User{
			ID:           0, // Will be set by Create
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hashed_password",
			Salt:         "salt",
			RoleID:       1,
			FirstName:    strPtr("Test"),
			LastName:     strPtr("User"),
			DisplayName:  strPtr("Test User"),
			IsActive:     true,
		}

		id, err := userRepo.Create(user)
		assert.NoError(t, err)
		assert.Equal(t, 1, id)

		// Verify expectations
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Test 2: Get user by ID
	t.Run("GetByID", func(t *testing.T) {
		// Setup expectations
		createdAt := time.Now()
		updatedAt := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "username", "email", "password_hash", "salt", "role_id", "first_name", "last_name",
			"display_name", "avatar_url", "time_zone", "language", "is_active", "is_locked",
			"locked_until", "failed_login_attempts", "last_login_at", "last_login_ip",
			"created_at", "updated_at", "settings",
		}).
			AddRow(
				1, "testuser", "test@example.com", "hashed_password", "salt", 1,
				"Test", "User", "Test User", nil, nil, nil,
				true, false, nil, 0, nil, nil,
				createdAt, updatedAt, "{}",
			)

		mock.ExpectQuery(`SELECT .* FROM users WHERE id = \?`).
			WithArgs(1).
			WillReturnRows(rows)

		// Execute
		user, err := userRepo.GetByID(1)
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, 1, user.ID)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, "test@example.com", user.Email)

		// Verify expectations
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Test 3: Get user by username
	t.Run("GetByUsername", func(t *testing.T) {
		// Setup expectations
		createdAt := time.Now()
		updatedAt := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "username", "email", "password_hash", "salt", "role_id", "first_name", "last_name",
			"display_name", "avatar_url", "time_zone", "language", "is_active", "is_locked",
			"locked_until", "failed_login_attempts", "last_login_at", "last_login_ip",
			"created_at", "updated_at", "settings",
		}).
			AddRow(
				1, "testuser", "test@example.com", "hashed_password", "salt", 1,
				"Test", "User", "Test User", nil, nil, nil,
				true, false, nil, 0, nil, nil,
				createdAt, updatedAt, "{}",
			)

		mock.ExpectQuery(`SELECT .* FROM users WHERE username = \?`).
			WithArgs("testuser").
			WillReturnRows(rows)

		// Execute
		user, err := userRepo.GetByUsername("testuser")
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "testuser", user.Username)

		// Verify expectations
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Test 4: Update user
	t.Run("UpdateUser", func(t *testing.T) {
		// Setup expectations
		mock.ExpectExec(`UPDATE users SET`).
			WithArgs(
				"testuser", "updated@example.com", "Test", "User", "Test User",
				nil, nil, nil, true, "{}", 0, sqlmock.AnyArg(), 1,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Execute
		user := &models.User{
			ID:          1,
			Username:    "testuser",
			Email:       "updated@example.com",
			FirstName:   strPtr("Test"),
			LastName:    strPtr("User"),
			DisplayName: strPtr("Test User"),
			IsActive:    true,
			Settings:    "{}",
		}

		err := userRepo.Update(user)
		assert.NoError(t, err)

		// Verify expectations
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Test 5: Delete user
	t.Run("DeleteUser", func(t *testing.T) {
		// Setup expectations
		// Delete reads the admin roles first (outside the transaction), then in one
		// transaction stamps and detaches the audit rows before removing the user.
		mock.ExpectQuery(`SELECT id, name, description, permissions, is_system, created_at, updated_at\s+FROM roles`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "permissions", "is_system", "created_at", "updated_at"}))
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT username FROM users WHERE id = \?`).
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("testuser"))
		mock.ExpectExec(`UPDATE auth_audit_log SET details`).
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(`UPDATE auth_audit_log SET user_id = NULL WHERE user_id = \?`).
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(`UPDATE user_permissions SET granted_by = NULL WHERE granted_by = \?`).
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`SELECT role_id, is_active FROM users WHERE id = \?`).
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"role_id", "is_active"}).AddRow(2, true))
		mock.ExpectExec(`DELETE FROM users WHERE id = \?`).
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		// Execute
		err := userRepo.Delete(1)
		assert.NoError(t, err)

		// Verify expectations
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Test 5b: Delete of the last active administrator is refused and rolled back (PX-F4)
	t.Run("DeleteUser_ProtectedLastAdmin", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, name, description, permissions, is_system, created_at, updated_at\s+FROM roles`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "permissions", "is_system", "created_at", "updated_at"}).
				AddRow(1, "admin", "Administrator", `["*"]`, true, time.Now(), time.Now()))
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT username FROM users WHERE id = \?`).
			WithArgs(7).
			WillReturnRows(sqlmock.NewRows([]string{"username"}).AddRow("root"))
		mock.ExpectExec(`UPDATE auth_audit_log SET details`).
			WithArgs(sqlmock.AnyArg(), 7).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(`UPDATE auth_audit_log SET user_id = NULL WHERE user_id = \?`).
			WithArgs(7).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(`UPDATE user_permissions SET granted_by = NULL WHERE granted_by = \?`).
			WithArgs(7).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`SELECT role_id, is_active FROM users WHERE id = \?`).
			WithArgs(7).
			WillReturnRows(sqlmock.NewRows([]string{"role_id", "is_active"}).AddRow(1, true))
		// the guarded DELETE matches no row when no other active administrator exists
		mock.ExpectExec(`DELETE FROM users WHERE id = \? AND \(SELECT COUNT\(\*\) FROM users o`).
			WithArgs(7, 7, true, false, 1).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		err := userRepo.Delete(7)
		require.ErrorIs(t, err, repository.ErrLastAdmin)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Test 6: List users
	t.Run("ListUsers", func(t *testing.T) {
		// Setup expectations
		createdAt := time.Now()
		updatedAt := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "username", "email", "password_hash", "salt", "role_id", "first_name", "last_name",
			"display_name", "avatar_url", "time_zone", "language", "is_active", "is_locked",
			"locked_until", "failed_login_attempts", "last_login_at", "last_login_ip",
			"created_at", "updated_at", "settings",
		}).
			AddRow(
				1, "user1", "user1@example.com", "hash1", "salt1", 1,
				"First1", "Last1", "User One", nil, nil, nil,
				true, false, nil, 0, nil, nil,
				createdAt, updatedAt, "{}",
			).
			AddRow(
				2, "user2", "user2@example.com", "hash2", "salt2", 2,
				"First2", "Last2", "User Two", nil, nil, nil,
				true, false, nil, 0, nil, nil,
				createdAt, updatedAt, "{}",
			)

		mock.ExpectQuery(`SELECT .* FROM users ORDER BY created_at DESC LIMIT \? OFFSET \?`).
			WithArgs(10, 0).
			WillReturnRows(rows)

		// Execute
		users, err := userRepo.List(10, 0)
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, "user1", users[0].Username)
		assert.Equal(t, "user2", users[1].Username)

		// Verify expectations
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestWithTestUtilsExample demonstrates using testutils package
func TestWithTestUtilsExample(t *testing.T) {
	// Create test template
	template := testutils.NewRepositoryTestTemplate(t)
	defer template.Cleanup()

	// Wrap with database.DB
	dialect := database.DialectSQLite
	wrappedDB := database.WrapDB(template.DB, dialect)

	// Create repository with mock database
	userRepo := repository.NewUserRepository(wrappedDB)

	t.Run("CreateUserWithTemplate", func(t *testing.T) {
		// Setup expectations using template
		template.ExpectExec(`INSERT INTO users`).
			WithArgs("templateuser", "template@example.com", "hashed_password", "salt", 1, "Template", "User", "Template User", nil, nil, nil, true, sqlmock.AnyArg(), sqlmock.AnyArg(), "{}").
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Execute
		user := &models.User{
			Username:     "templateuser",
			Email:        "template@example.com",
			PasswordHash: "hashed_password",
			Salt:         "salt",
			RoleID:       1,
			FirstName:    strPtr("Template"),
			LastName:     strPtr("User"),
			DisplayName:  strPtr("Template User"),
			IsActive:     true,
		}

		id, err := userRepo.Create(user)
		assert.NoError(t, err)
		assert.Equal(t, 1, id)

		// Verify all expectations
		template.VerifyAll()
	})
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

// TestIntegrationExample demonstrates integration test with real database
func TestIntegrationExample(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO users (username, email) VALUES (?, ?)`, "demo", "demo@test.com")
	require.NoError(t, err)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
