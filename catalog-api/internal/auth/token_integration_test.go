package auth

import (
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// setupRealDBService creates an AuthService backed by an in-memory SQLite database.
func setupRealDBService(t *testing.T) *AuthService {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewAuthService(db, "test-jwt-secret-key", logger)
	err = service.Initialize()
	require.NoError(t, err)

	return service
}

// insertTestUser inserts a user directly into the DB and returns the user ID.
func insertTestUser(t *testing.T, service *AuthService, username, password, role string) int64 {
	t.Helper()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	result, err := service.db.Exec(
		`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		username, username+"@test.com", string(passwordHash), "Test", "User", role, true,
	)
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)
	return id
}

// TestTokenGeneration_AccessAndRefreshDifferentExpiry verifies that access and refresh tokens
// have different expiry times.
func TestTokenGeneration_AccessAndRefreshDifferentExpiry(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	user := &User{
		ID:          1,
		Username:    "testuser",
		Role:        RoleUser,
		Permissions: GetRolePermissions(RoleUser),
	}

	accessToken, err := service.generateToken(user, "access")
	require.NoError(t, err)

	refreshToken, err := service.generateToken(user, "refresh")
	require.NoError(t, err)

	accessClaims, err := service.validateToken(accessToken)
	require.NoError(t, err)

	refreshClaims, err := service.validateToken(refreshToken)
	require.NoError(t, err)

	assert.Equal(t, "access", accessClaims.Type)
	assert.Equal(t, "refresh", refreshClaims.Type)

	// Refresh token should expire later than access token
	assert.Greater(t, refreshClaims.ExpiresAt, accessClaims.ExpiresAt)

	// Access token: ~24h from now; refresh token: ~7 days from now
	now := time.Now().Unix()
	accessDuration := accessClaims.ExpiresAt - now
	refreshDuration := refreshClaims.ExpiresAt - now

	assert.InDelta(t, 24*3600, accessDuration, 5)    // 24h +/- 5s
	assert.InDelta(t, 7*24*3600, refreshDuration, 5) // 7d +/- 5s
}

// TestTokenGeneration_ClaimsContainCorrectData verifies that JWT claims carry proper user data.
func TestTokenGeneration_ClaimsContainCorrectData(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	user := &User{
		ID:          42,
		Username:    "claimsuser",
		Role:        RoleAdmin,
		Permissions: GetRolePermissions(RoleAdmin),
	}

	token, err := service.generateToken(user, "access")
	require.NoError(t, err)

	claims, err := service.validateToken(token)
	require.NoError(t, err)

	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, "claimsuser", claims.Username)
	assert.Equal(t, RoleAdmin, claims.Role)
	assert.Contains(t, claims.Permissions, PermissionSystemAdmin)
	assert.Contains(t, claims.Permissions, PermissionReadMedia)
}

// TestTokenValidation_WrongSecret verifies that a token signed with a different secret is rejected.
func TestTokenValidation_WrongSecret(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()
	service1 := NewAuthService(db, "secret-one", logger)
	service2 := NewAuthService(db, "secret-two", logger)

	user := &User{
		ID:       1,
		Username: "testuser",
		Role:     RoleUser,
	}

	token, err := service1.generateToken(user, "access")
	require.NoError(t, err)

	// Validating with a different secret should fail
	claims, err := service2.validateToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// TestTokenValidation_TamperedToken verifies that a modified token payload is rejected.
func TestTokenValidation_TamperedToken(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	user := &User{
		ID:       1,
		Username: "testuser",
		Role:     RoleUser,
	}

	token, err := service.generateToken(user, "access")
	require.NoError(t, err)

	// Tamper with the token by changing a character in the payload
	tampered := token[:len(token)/2] + "X" + token[len(token)/2+1:]

	claims, err := service.validateToken(tampered)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// TestTokenValidation_EmptyToken verifies empty string is rejected.
func TestTokenValidation_EmptyToken(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	claims, err := service.validateToken("")
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// TestFullLoginFlow_Integration tests the complete login-validate-logout cycle
// with a real database.
func TestFullLoginFlow_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	insertTestUser(t, service, "flowuser", "mypassword", RoleUser)

	// Step 1: Login
	loginResp, err := service.Login("flowuser", "mypassword", "10.0.0.1", "Mozilla/5.0")
	require.NoError(t, err)
	require.NotNil(t, loginResp)
	assert.NotEmpty(t, loginResp.Token)
	assert.NotEmpty(t, loginResp.RefreshToken)
	assert.Equal(t, "flowuser", loginResp.User.Username)
	assert.Equal(t, RoleUser, loginResp.User.Role)
	assert.Equal(t, int64(86400), loginResp.ExpiresIn)

	// Step 2: Validate token (should succeed because session exists)
	user, err := service.ValidateToken(loginResp.Token)
	require.NoError(t, err)
	assert.Equal(t, "flowuser", user.Username)

	// Step 3: Logout
	err = service.Logout(loginResp.Token)
	require.NoError(t, err)

	// Step 4: Validate token again (should fail because session is deleted)
	user, err = service.ValidateToken(loginResp.Token)
	assert.Error(t, err)
	assert.Nil(t, user)
}

// TestLogin_ByEmail_Integration verifies that login works with email instead of username.
func TestLogin_ByEmail_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	insertTestUser(t, service, "emailuser", "pass1234", RoleUser)

	loginResp, err := service.Login("emailuser@test.com", "pass1234", "127.0.0.1", "test")
	require.NoError(t, err)
	assert.Equal(t, "emailuser", loginResp.User.Username)
}

// TestLogin_WrongPassword_Integration verifies that wrong password is rejected.
func TestLogin_WrongPassword_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	insertTestUser(t, service, "wrongpwuser", "correctpass", RoleUser)

	resp, err := service.Login("wrongpwuser", "wrongpass", "127.0.0.1", "test")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid credentials")
}

// TestLogin_InactiveUser_Integration verifies that inactive users cannot login.
func TestLogin_InactiveUser_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("pass1234"), bcrypt.MinCost)
	_, err := service.db.Exec(
		`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"inactiveuser", "inactive@test.com", string(passwordHash), "In", "Active", RoleUser, false,
	)
	require.NoError(t, err)

	resp, err := service.Login("inactiveuser", "pass1234", "127.0.0.1", "test")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "account is disabled")
}

// TestLogin_NonexistentUser_Integration verifies that nonexistent users are rejected.
func TestLogin_NonexistentUser_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	resp, err := service.Login("nobody", "pass", "127.0.0.1", "test")
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// TestPasswordHashing_Integration verifies password hashing and comparison.
func TestPasswordHashing_Integration(t *testing.T) {
	password := "MyS3cur3P@ss!"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	// Correct password should match
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	assert.NoError(t, err)

	// Wrong password should not match
	err = bcrypt.CompareHashAndPassword(hash, []byte("wrong"))
	assert.Error(t, err)

	// Same password generates different hashes (salt)
	hash2, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	assert.NotEqual(t, string(hash), string(hash2))

	// Both hashes still validate against the original password
	err = bcrypt.CompareHashAndPassword(hash2, []byte(password))
	assert.NoError(t, err)
}

// TestChangePassword_Integration verifies the full change password flow.
func TestChangePassword_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	userID := insertTestUser(t, service, "changepwuser", "oldpassword", RoleUser)

	// Change password
	err := service.ChangePassword(userID, "oldpassword", "newpassword123")
	require.NoError(t, err)

	// Old password should no longer work
	_, err = service.Login("changepwuser", "oldpassword", "127.0.0.1", "test")
	assert.Error(t, err)

	// New password should work
	loginResp, err := service.Login("changepwuser", "newpassword123", "127.0.0.1", "test")
	require.NoError(t, err)
	assert.NotNil(t, loginResp)
}

// TestChangePassword_WrongCurrent_Integration verifies that wrong current password is rejected.
func TestChangePassword_WrongCurrent_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	userID := insertTestUser(t, service, "changepw2", "realpass", RoleUser)

	err := service.ChangePassword(userID, "wrongcurrent", "newpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "current password is incorrect")
}

// TestCreateUser_Integration verifies user creation through the service.
func TestCreateUser_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	req := &RegisterRequest{
		Username:  "newintegrationuser",
		Email:     "newint@test.com",
		Password:  "securepass123",
		FirstName: "New",
		LastName:  "User",
	}

	user, err := service.CreateUser(req)
	require.NoError(t, err)
	assert.Equal(t, "newintegrationuser", user.Username)
	assert.Equal(t, "newint@test.com", user.Email)
	assert.Equal(t, RoleUser, user.Role)
	assert.True(t, user.IsActive)

	// Should be able to login with the new user
	loginResp, err := service.Login("newintegrationuser", "securepass123", "127.0.0.1", "test")
	require.NoError(t, err)
	assert.NotNil(t, loginResp)
}

// TestCreateUser_Duplicate_Integration verifies that duplicate username/email is rejected.
func TestCreateUser_Duplicate_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	req := &RegisterRequest{
		Username:  "dupuser",
		Email:     "dup@test.com",
		Password:  "password123",
		FirstName: "Dup",
		LastName:  "User",
	}

	_, err := service.CreateUser(req)
	require.NoError(t, err)

	// Try to create the same user again
	_, err = service.CreateUser(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestSessionManagement_Integration verifies session creation and invalidation.
func TestSessionManagement_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	insertTestUser(t, service, "sessuser", "password123", RoleUser)

	// Login creates a session
	loginResp, err := service.Login("sessuser", "password123", "10.0.0.1", "Chrome")
	require.NoError(t, err)

	// Verify session exists
	var count int
	err = service.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE token = ?", loginResp.Token).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Logout deletes the session
	err = service.Logout(loginResp.Token)
	require.NoError(t, err)

	err = service.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE token = ?", loginResp.Token).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestMultipleSessions_Integration verifies a user can have multiple active sessions.
// Note: tokens generated in the same second for the same user produce identical JWTs
// (no jti/nonce claim), so we introduce a 1-second delay between logins to ensure
// distinct tokens and sessions.
func TestMultipleSessions_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	insertTestUser(t, service, "multiuser", "password123", RoleUser)

	// Login from first "device"
	resp1, err := service.Login("multiuser", "password123", "10.0.0.1", "Chrome")
	require.NoError(t, err)

	// Wait so the second token has a different iat/exp (tokens have no jti)
	time.Sleep(1100 * time.Millisecond)

	// Login from second "device"
	resp2, err := service.Login("multiuser", "password123", "10.0.0.2", "Firefox")
	require.NoError(t, err)

	// Tokens should be different
	require.NotEqual(t, resp1.Token, resp2.Token, "tokens should differ across seconds")

	// Both tokens should be valid
	user1, err := service.ValidateToken(resp1.Token)
	require.NoError(t, err, "resp1 token should validate")
	assert.Equal(t, "multiuser", user1.Username)

	user2, err := service.ValidateToken(resp2.Token)
	require.NoError(t, err, "resp2 token should validate")
	assert.Equal(t, "multiuser", user2.Username)

	// Logout first session; second should still work
	err = service.Logout(resp1.Token)
	require.NoError(t, err)

	_, err = service.ValidateToken(resp1.Token)
	assert.Error(t, err)

	user2Again, err := service.ValidateToken(resp2.Token)
	require.NoError(t, err)
	assert.Equal(t, "multiuser", user2Again.Username)
}

// TestAuditLog_Integration verifies that login events are recorded in the audit log.
func TestAuditLog_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	insertTestUser(t, service, "audituser", "password123", RoleUser)

	// Successful login
	_, err := service.Login("audituser", "password123", "10.0.0.1", "Chrome")
	require.NoError(t, err)

	// Failed login
	_, _ = service.Login("audituser", "wrongpass", "10.0.0.2", "Firefox")

	// Check audit log entries
	var successCount, failCount int
	err = service.db.QueryRow(
		"SELECT COUNT(*) FROM auth_audit_log WHERE event_type = 'login_success'").Scan(&successCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, successCount, 1)

	err = service.db.QueryRow(
		"SELECT COUNT(*) FROM auth_audit_log WHERE event_type = 'failed_login'").Scan(&failCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, failCount, 1)
}

// TestDefaultAdminUser_Integration verifies the default admin user is created on Initialize.
func TestDefaultAdminUser_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	// Admin should have been created by Initialize
	loginResp, err := service.Login("admin", "Test-Admin-Pw-9xK2-local", "127.0.0.1", "test")
	require.NoError(t, err)
	assert.Equal(t, "admin", loginResp.User.Username)
	assert.Equal(t, RoleAdmin, loginResp.User.Role)
}

// TestListUsers_Integration verifies listing users with pagination.
func TestListUsers_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	// admin already exists from Initialize
	insertTestUser(t, service, "listuser1", "pass", RoleUser)
	insertTestUser(t, service, "listuser2", "pass", RoleViewer)

	users, total, err := service.ListUsers(10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total) // admin + 2 created users
	assert.Len(t, users, 3)

	// Test pagination
	users, total, err = service.ListUsers(1, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, users, 1)
}

// TestUpdateUser_Integration verifies user profile update.
func TestUpdateUser_Integration(t *testing.T) {
	service := setupRealDBService(t)
	defer service.db.Close()

	userID := insertTestUser(t, service, "updateuser", "pass", RoleUser)

	newFirst := "Updated"
	newEmail := "updated@test.com"
	user, err := service.UpdateUser(userID, &UpdateUserRequest{
		FirstName: &newFirst,
		Email:     &newEmail,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", user.FirstName)
	assert.Equal(t, "updated@test.com", user.Email)
}
