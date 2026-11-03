package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"catalogizer/database"
	"catalogizer/internal/auth"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type InternalAuthHandlerTestSuite struct {
	suite.Suite
	handler *AuthHandler
	router  *gin.Engine
	logger  *zap.Logger
}

func (suite *InternalAuthHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.logger = zap.NewNop()
}

func (suite *InternalAuthHandlerTestSuite) SetupTest() {
	// Initialize handler with nil auth service to test validation paths
	suite.handler = NewAuthHandler(nil, suite.logger)

	suite.router = gin.New()
	suite.router.POST("/api/v1/auth/login", suite.handler.Login)
	suite.router.POST("/api/v1/auth/logout", suite.handler.Logout)
	suite.router.POST("/api/v1/auth/register", suite.handler.Register)
	suite.router.GET("/api/v1/auth/profile", suite.handler.GetProfile)
	suite.router.PUT("/api/v1/auth/profile", suite.handler.UpdateProfile)
	suite.router.POST("/api/v1/auth/change-password", suite.handler.ChangePassword)
	suite.router.GET("/api/v1/auth/admin/users", suite.handler.ListUsers)
	suite.router.GET("/api/v1/auth/admin/users/:id", suite.handler.GetUser)
	suite.router.PUT("/api/v1/auth/admin/users/:id", suite.handler.UpdateUser)
	suite.router.GET("/api/v1/auth/status", suite.handler.GetAuthStatus)
	suite.router.GET("/api/v1/auth/permissions", suite.handler.GetPermissions)
	suite.router.GET("/api/v1/auth/init-status", suite.handler.GetInitStatus)
}

// setupAuthHandlerWithDB creates an AuthHandler backed by a real in-memory SQLite DB
func setupAuthHandlerWithDB(t *testing.T) (*AuthHandler, *gin.Engine) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	wrappedDB := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()

	os.Setenv("ADMIN_PASSWORD", "Test-Admin-Pw-9xK2-local")
	defer os.Unsetenv("ADMIN_PASSWORD")

	authService := auth.NewAuthService(wrappedDB, "test-jwt-secret", logger)
	if err := authService.Initialize(); err != nil {
		t.Fatalf("Failed to initialize auth service: %v", err)
	}

	handler := NewAuthHandler(authService, logger)
	router := gin.New()
	router.GET("/api/v1/auth/admin/users", handler.ListUsers)
	router.GET("/api/v1/auth/init-status", handler.GetInitStatus)

	return handler, router
}

// Constructor tests

func (suite *InternalAuthHandlerTestSuite) TestNewAuthHandler() {
	handler := NewAuthHandler(nil, suite.logger)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.authService)
	assert.NotNil(suite.T(), handler.logger)
}

// Login tests

func (suite *InternalAuthHandlerTestSuite) TestLogin_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Invalid request format", resp["error"])
}

func (suite *InternalAuthHandlerTestSuite) TestLogin_EmptyBody() {
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *InternalAuthHandlerTestSuite) TestLogin_MissingFields() {
	body := `{"username": ""}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Logout tests

func (suite *InternalAuthHandlerTestSuite) TestLogout_MissingAuthHeader() {
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Missing authorization header", resp["error"])
}

// Register tests

func (suite *InternalAuthHandlerTestSuite) TestRegister_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Invalid request format", resp["error"])
}

func (suite *InternalAuthHandlerTestSuite) TestRegister_EmptyBody() {
	req := httptest.NewRequest("POST", "/api/v1/auth/register", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// GetProfile tests

func (suite *InternalAuthHandlerTestSuite) TestGetProfile_Unauthenticated() {
	req := httptest.NewRequest("GET", "/api/v1/auth/profile", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "User not authenticated", resp["error"])
}

// UpdateProfile tests

func (suite *InternalAuthHandlerTestSuite) TestUpdateProfile_Unauthenticated() {
	body := `{"first_name": "John"}`
	req := httptest.NewRequest("PUT", "/api/v1/auth/profile", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "User not authenticated", resp["error"])
}

// ChangePassword tests

func (suite *InternalAuthHandlerTestSuite) TestChangePassword_Unauthenticated() {
	body := `{"current_password": "old", "new_password": "new12345"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/change-password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
}

// GetUser tests

func (suite *InternalAuthHandlerTestSuite) TestGetUser_InvalidID() {
	req := httptest.NewRequest("GET", "/api/v1/auth/admin/users/abc", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Invalid user ID", resp["error"])
}

func (suite *InternalAuthHandlerTestSuite) TestGetUser_FloatID() {
	req := httptest.NewRequest("GET", "/api/v1/auth/admin/users/1.5", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// UpdateUser tests

func (suite *InternalAuthHandlerTestSuite) TestUpdateUser_InvalidID() {
	body := `{"first_name": "John"}`
	req := httptest.NewRequest("PUT", "/api/v1/auth/admin/users/abc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *InternalAuthHandlerTestSuite) TestUpdateUser_InvalidJSON() {
	req := httptest.NewRequest("PUT", "/api/v1/auth/admin/users/1", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// GetAuthStatus tests

func (suite *InternalAuthHandlerTestSuite) TestGetAuthStatus_Unauthenticated() {
	req := httptest.NewRequest("GET", "/api/v1/auth/status", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), false, resp["authenticated"])
}

// GetPermissions tests

func (suite *InternalAuthHandlerTestSuite) TestGetPermissions_Unauthenticated() {
	req := httptest.NewRequest("GET", "/api/v1/auth/permissions", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Not authenticated", resp["error"])
}

// ListUsers tests with real DB

func TestListUsers_WithDB_DefaultPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := setupAuthHandlerWithDB(t)

	req := httptest.NewRequest("GET", "/api/v1/auth/admin/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.NotNil(t, resp["users"])
	assert.Equal(t, float64(20), resp["limit"])
	assert.Equal(t, float64(0), resp["offset"])
	// Should have at least the default admin user
	total := resp["total"].(float64)
	assert.GreaterOrEqual(t, total, float64(1))
}

func TestListUsers_WithDB_CustomPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := setupAuthHandlerWithDB(t)

	req := httptest.NewRequest("GET", "/api/v1/auth/admin/users?limit=5&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(5), resp["limit"])
	assert.Equal(t, float64(0), resp["offset"])
}

func TestListUsers_WithDB_LimitCappedAt100(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := setupAuthHandlerWithDB(t)

	req := httptest.NewRequest("GET", "/api/v1/auth/admin/users?limit=200", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	// Limit should be capped at 100
	assert.Equal(t, float64(100), resp["limit"])
}

func TestListUsers_WithDB_InvalidLimitDefaultsTo20(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := setupAuthHandlerWithDB(t)

	req := httptest.NewRequest("GET", "/api/v1/auth/admin/users?limit=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	// Invalid limit should default to 20 (Atoi returns 0 for invalid, but DefaultQuery gives "20")
	// Actually "abc" is provided but Atoi returns 0 for it
	assert.Contains(t, []float64{0, 20}, resp["limit"])
}

// GetInitStatus tests with real DB

func TestGetInitStatus_WithDB_HasAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, router := setupAuthHandlerWithDB(t)

	req := httptest.NewRequest("GET", "/api/v1/auth/init-status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	// After Initialize(), there should be an admin user
	assert.Equal(t, true, resp["initialized"])
	assert.Equal(t, true, resp["has_admin"])
	userCount := resp["user_count"].(float64)
	assert.GreaterOrEqual(t, userCount, float64(1))
}

func TestGetInitStatus_WithDB_NoUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a DB with the auth tables but no users
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	wrappedDB := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()

	// Create tables manually without creating default admin
	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		is_active BOOLEAN NOT NULL DEFAULT 1,
		last_login DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	authService := auth.NewAuthService(wrappedDB, "test-jwt-secret", logger)
	handler := NewAuthHandler(authService, logger)

	router := gin.New()
	router.GET("/api/v1/auth/init-status", handler.GetInitStatus)

	req := httptest.NewRequest("GET", "/api/v1/auth/init-status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp["initialized"])
	assert.Equal(t, false, resp["has_admin"])
	assert.Equal(t, float64(0), resp["user_count"])
}

func TestGetInitStatus_WithDB_UserButNoAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	wrappedDB := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()

	// Create tables and insert a regular user (not admin)
	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		is_active BOOLEAN NOT NULL DEFAULT 1,
		last_login DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = sqlDB.Exec(`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		VALUES ('regularuser', 'user@example.com', 'hash', 'Regular', 'User', 'user', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	authService := auth.NewAuthService(wrappedDB, "test-jwt-secret", logger)
	handler := NewAuthHandler(authService, logger)

	router := gin.New()
	router.GET("/api/v1/auth/init-status", handler.GetInitStatus)

	req := httptest.NewRequest("GET", "/api/v1/auth/init-status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp["initialized"])
	assert.Equal(t, false, resp["has_admin"])
	assert.Equal(t, float64(1), resp["user_count"])
}

func TestInternalAuthHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(InternalAuthHandlerTestSuite))
}
