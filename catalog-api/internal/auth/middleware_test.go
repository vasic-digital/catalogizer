package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"catalogizer/database"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupMiddlewareTest creates an AuthService with a real SQLite in-memory DB
// for integration-style tests where session lookups are needed.
func setupMiddlewareTestWithRealDB(t *testing.T) (*AuthService, *AuthMiddleware) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()
	// Set admin credentials for tests (matches hardcoded values in test expectations)
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "Test-Admin-Pw-9xK2-local")
	service := NewAuthService(db, "integration-test-secret", logger)
	err = service.Initialize()
	require.NoError(t, err)

	mw := NewAuthMiddleware(service, logger)
	return service, mw
}

// createTestUserAndLogin creates a user, logs in, and returns the login response.
func createTestUserAndLogin(t *testing.T, service *AuthService, username, password string) *LoginResponse {
	t.Helper()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)

	_, err = service.db.Exec(
		`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		username, username+"@test.com", string(passwordHash), "Test", "User", "user", true,
	)
	require.NoError(t, err)

	resp, err := service.Login(username, password, "127.0.0.1", "test-agent")
	require.NoError(t, err)
	return resp
}

// TestRequireAuth_NoToken verifies that a request without a token returns 401.
func TestRequireAuth_NoToken(t *testing.T) {
	_, mw := setupMiddlewareTestWithRealDB(t)

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body["error"], "Missing")
}

// TestRequireAuth_InvalidToken verifies that an invalid token returns 401.
func TestRequireAuth_InvalidToken(t *testing.T) {
	_, mw := setupMiddlewareTestWithRealDB(t)

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireAuth_ValidToken verifies that a valid token with a session returns 200.
func TestRequireAuth_ValidToken(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "authuser", "password123")

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		username, _ := c.Get("username")
		c.JSON(http.StatusOK, gin.H{"username": username})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "authuser", body["username"])
}

// TestRequireAuth_ExpiredToken verifies that an expired token returns 401.
func TestRequireAuth_ExpiredToken(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	// Temporarily set TTL to something extremely short
	service.tokenTTL = 1 * time.Millisecond

	loginResp := createTestUserAndLogin(t, service, "expireduser", "password123")

	// Wait for the token to expire
	time.Sleep(50 * time.Millisecond)

	// Restore TTL
	service.tokenTTL = 24 * time.Hour

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireAuth_MalformedAuthorizationHeader verifies various malformed header formats.
func TestRequireAuth_MalformedAuthorizationHeader(t *testing.T) {
	_, mw := setupMiddlewareTestWithRealDB(t)

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name   string
		header string
	}{
		{"empty header", ""},
		{"no Bearer prefix", "Token abc123"},
		{"bearer lowercase", "bearer abc123"},
		{"only Bearer", "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// TestRequireAuth_TokenWithoutSession verifies that a valid JWT token with no session record returns 401.
func TestRequireAuth_TokenWithoutSession(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	// Generate a token directly (bypassing Login, so no session is created)
	user := &User{
		ID:          999,
		Username:    "nosession",
		Role:        "user",
		Permissions: GetRolePermissions("user"),
	}
	token, err := service.generateToken(user, "access")
	require.NoError(t, err)

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequirePermission_HasPermission verifies access when the user has the required permission.
func TestRequirePermission_HasPermission(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "permuser", "password123")

	router := gin.New()
	router.GET("/media", mw.RequireAuth(), mw.RequirePermission(PermissionReadMedia), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequirePermission_MissingPermission verifies 403 when the user lacks the required permission.
func TestRequirePermission_MissingPermission(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	// Create a viewer user (does not have manage:users)
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	_, err := service.db.Exec(
		`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"vieweruser", "viewer@test.com", string(passwordHash), "View", "User", "viewer", true,
	)
	require.NoError(t, err)

	loginResp, err := service.Login("vieweruser", "password123", "127.0.0.1", "test")
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin", mw.RequireAuth(), mw.RequirePermission(PermissionManageUsers), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRequirePermission_NoUser verifies 401 when RequirePermission is used without RequireAuth.
func TestRequirePermission_NoUser(t *testing.T) {
	_, mw := setupMiddlewareTestWithRealDB(t)

	router := gin.New()
	// Intentionally skip RequireAuth to test RequirePermission alone
	router.GET("/check", mw.RequirePermission(PermissionReadMedia), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireRole_MatchingRole verifies access when the user has the required role.
func TestRequireRole_MatchingRole(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	// admin user is created by Initialize; login as admin
	loginResp, err := service.Login("admin", "Test-Admin-Pw-9xK2-local", "127.0.0.1", "test")
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin-only", mw.RequireAuth(), mw.RequireRole(RoleAdmin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireRole_WrongRole verifies 403 when the user does not have the required role.
func TestRequireRole_WrongRole(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "normaluser", "password123")

	router := gin.New()
	router.GET("/mod-only", mw.RequireAuth(), mw.RequireRole(RoleModerator), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mod-only", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRequireRole_AdminBypassesRoleCheck verifies that admin can access any role-gated route.
func TestRequireRole_AdminBypassesRoleCheck(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp, err := service.Login("admin", "Test-Admin-Pw-9xK2-local", "127.0.0.1", "test")
	require.NoError(t, err)

	router := gin.New()
	router.GET("/mod-route", mw.RequireAuth(), mw.RequireRole(RoleModerator), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mod-route", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireResourceAccess_Granted verifies resource access when user has the right permission.
func TestRequireResourceAccess_Granted(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "resuser", "password123")

	router := gin.New()
	router.GET("/catalog", mw.RequireAuth(), mw.RequireResourceAccess("media", "read"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireResourceAccess_Denied verifies 403 when user lacks the resource permission.
func TestRequireResourceAccess_Denied(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	// Create a viewer: has read:media but not delete:media
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	_, err := service.db.Exec(
		`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"viewer2", "viewer2@test.com", string(passwordHash), "View", "User", "viewer", true,
	)
	require.NoError(t, err)

	loginResp, err := service.Login("viewer2", "password123", "127.0.0.1", "test")
	require.NoError(t, err)

	router := gin.New()
	router.DELETE("/media/:id", mw.RequireAuth(), mw.RequireResourceAccess("media", "delete"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/media/1", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestOptionalAuth_WithToken verifies that OptionalAuth sets user when a valid token is present.
func TestOptionalAuth_WithToken(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "optuser", "password123")

	router := gin.New()
	router.GET("/public", mw.OptionalAuth(), func(c *gin.Context) {
		user, exists := GetCurrentUser(c)
		if exists {
			c.JSON(http.StatusOK, gin.H{"username": user.Username})
		} else {
			c.JSON(http.StatusOK, gin.H{"username": "anonymous"})
		}
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "optuser", body["username"])
}

// TestOptionalAuth_WithoutToken verifies that OptionalAuth still allows the request without a token.
func TestOptionalAuth_WithoutToken(t *testing.T) {
	_, mw := setupMiddlewareTestWithRealDB(t)

	router := gin.New()
	router.GET("/public", mw.OptionalAuth(), func(c *gin.Context) {
		_, exists := GetCurrentUser(c)
		if exists {
			c.JSON(http.StatusOK, gin.H{"authenticated": true})
		} else {
			c.JSON(http.StatusOK, gin.H{"authenticated": false})
		}
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, false, body["authenticated"])
}

// TestOptionalAuth_WithInvalidToken verifies that OptionalAuth proceeds without setting user on invalid token.
func TestOptionalAuth_WithInvalidToken(t *testing.T) {
	_, mw := setupMiddlewareTestWithRealDB(t)

	router := gin.New()
	router.GET("/public", mw.OptionalAuth(), func(c *gin.Context) {
		_, exists := GetCurrentUser(c)
		c.JSON(http.StatusOK, gin.H{"authenticated": exists})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	req.Header.Set("Authorization", "Bearer garbage.token.here")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, false, body["authenticated"])
}

// TestAdminOnly_AdminUser verifies that AdminOnly allows admin users.
func TestAdminOnly_AdminUser(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp, err := service.Login("admin", "Test-Admin-Pw-9xK2-local", "127.0.0.1", "test")
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin", mw.RequireAuth(), mw.AdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAdminOnly_RegularUser verifies that AdminOnly rejects non-admin users.
func TestAdminOnly_RegularUser(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "regularuser", "password123")

	router := gin.New()
	router.GET("/admin", mw.RequireAuth(), mw.AdminOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestLogout_InvalidatesSession verifies that after logout, the token no longer works.
func TestLogout_InvalidatesSession(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "logoutuser", "password123")

	// Verify token works before logout
	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Perform logout
	err := service.Logout(loginResp.Token)
	require.NoError(t, err)

	// Verify token no longer works after logout
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

// TestContextHelpers verifies GetCurrentUser, GetCurrentUserID, HasPermission, IsAdmin, CanAccessResource.
func TestContextHelpers(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "helperuser", "password123")

	router := gin.New()
	router.GET("/helpers", mw.RequireAuth(), func(c *gin.Context) {
		user, ok := GetCurrentUser(c)
		assert.True(t, ok)
		assert.Equal(t, "helperuser", user.Username)

		userID, ok := GetCurrentUserID(c)
		assert.True(t, ok)
		assert.Greater(t, userID, int64(0))

		assert.True(t, HasPermission(c, PermissionReadMedia))
		assert.False(t, HasPermission(c, PermissionSystemAdmin))
		assert.False(t, IsAdmin(c))
		assert.True(t, CanAccessResource(c, "media", "read"))
		assert.False(t, CanAccessResource(c, "system", "admin"))

		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/helpers", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestContextHelpers_NoUser verifies helpers return false when no user in context.
func TestContextHelpers_NoUser(t *testing.T) {
	router := gin.New()
	router.GET("/no-user", func(c *gin.Context) {
		_, ok := GetCurrentUser(c)
		assert.False(t, ok)

		_, ok = GetCurrentUserID(c)
		assert.False(t, ok)

		assert.False(t, HasPermission(c, PermissionReadMedia))
		assert.False(t, IsAdmin(c))
		assert.False(t, CanAccessResource(c, "media", "read"))

		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/no-user", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestModeratorOrAdmin_ModeratorUser verifies ModeratorOrAdmin allows moderators.
func TestModeratorOrAdmin_ModeratorUser(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	_, err := service.db.Exec(
		`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"moduser", "mod@test.com", string(passwordHash), "Mod", "User", "moderator", true,
	)
	require.NoError(t, err)

	loginResp, err := service.Login("moduser", "password123", "127.0.0.1", "test")
	require.NoError(t, err)

	router := gin.New()
	router.GET("/modadmin", mw.RequireAuth(), mw.ModeratorOrAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/modadmin", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestModeratorOrAdmin_RegularUserDenied verifies ModeratorOrAdmin rejects regular users.
func TestModeratorOrAdmin_RegularUserDenied(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "reguser2", "password123")

	router := gin.New()
	router.GET("/modadmin", mw.RequireAuth(), mw.ModeratorOrAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/modadmin", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestLogUserActivity verifies the activity logging middleware does not interfere with requests.
func TestLogUserActivity_AuthenticatedUser(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "loguser", "password123")

	router := gin.New()
	router.GET("/activity", mw.RequireAuth(), mw.LogUserActivity(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "logged"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/activity", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogUserActivity_UnauthenticatedUser(t *testing.T) {
	logger := zap.NewNop()
	mw := NewAuthMiddleware(nil, logger)

	router := gin.New()
	router.GET("/activity", mw.LogUserActivity(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/activity", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRateLimitByUser verifies per-user rate limiting.
func TestRateLimitByUser_AllowsUnderLimit(t *testing.T) {
	logger := zap.NewNop()
	mw := NewAuthMiddleware(nil, logger)

	router := gin.New()
	router.GET("/limited", mw.RateLimitByUser(5, "1m"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// Send 5 requests (at the limit)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.0.2.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestRateLimitByUser_RejectsOverLimit(t *testing.T) {
	logger := zap.NewNop()
	mw := NewAuthMiddleware(nil, logger)

	router := gin.New()
	router.GET("/limited", mw.RateLimitByUser(3, "1m"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// Send 3 requests (at the limit)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.0.2.2:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 4th request should be rejected
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "192.0.2.2:12345"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Contains(t, body["error"], "Rate limit exceeded")
}

func TestRateLimitByUser_InvalidWindow(t *testing.T) {
	logger := zap.NewNop()
	mw := NewAuthMiddleware(nil, logger)

	// Invalid window should default to 1 minute
	router := gin.New()
	router.GET("/limited", mw.RateLimitByUser(100, "invalid"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "192.0.2.3:12345"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitByUser_AuthenticatedUser(t *testing.T) {
	service, mw := setupMiddlewareTestWithRealDB(t)

	loginResp := createTestUserAndLogin(t, service, "ratelimituser", "password123")

	router := gin.New()
	router.GET("/limited", mw.RequireAuth(), mw.RateLimitByUser(2, "1m"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// 2 requests should succeed
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 3rd should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimitByUser_DifferentClientsIndependent(t *testing.T) {
	logger := zap.NewNop()
	mw := NewAuthMiddleware(nil, logger)

	router := gin.New()
	router.GET("/limited", mw.RateLimitByUser(2, "1m"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// Client A: 2 requests
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Client A rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Client B should NOT be rate limited
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req2.RemoteAddr = "10.0.0.2:12345"
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}
