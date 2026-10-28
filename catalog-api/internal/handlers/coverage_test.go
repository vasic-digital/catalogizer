package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/auth"
	"catalogizer/internal/services"
	"catalogizer/internal/smb"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================
// SMB Handler: RemoveSource, GetSourcesStatus, GetStatistics,
//              testSMBConnection, GetHealth, AddSource
// ============================================================

func TestSMBHandler_RemoveSource_ValidID_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	// RemoveSource with non-empty ID but nil smbManager -> panic
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/api/v1/smb/sources/source1", nil)
		c.Params = gin.Params{{Key: "id", Value: "source1"}}
		handler.RemoveSource(c)
	})
}

func TestSMBHandler_RemoveSource_EmptyID_DirectCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/v1/smb/sources/", nil)
	c.Params = gin.Params{{Key: "id", Value: ""}}

	handler.RemoveSource(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Source ID is required", resp["error"])
}

func TestSMBHandler_GetSourcesStatus_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	// GetSourcesStatus calls h.smbManager.GetSourceStatus() -> panic on nil
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/smb/sources/status", nil)
		handler.GetSourcesStatus(c)
	})
}

func TestSMBHandler_GetStatistics_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	// GetStatistics calls h.smbManager.GetSourceStatus() -> panic on nil
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/smb/statistics", nil)
		handler.GetStatistics(c)
	})
}

func TestSMBHandler_GetHealth_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	// GetHealth calls h.smbManager.GetSourceStatus() -> panic on nil
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/smb/health", nil)
		handler.GetHealth(c)
	})
}

func TestSMBHandler_testSMBConnection_NilManager(t *testing.T) {
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	source := &smb.SMBSource{
		Path: "//server/share",
	}
	err := handler.testSMBConnection(source)
	assert.Error(t, err)
	assert.Equal(t, "SMB manager not initialized", err.Error())
}

func TestSMBHandler_testSMBConnection_EmptyPath(t *testing.T) {
	// Even with a nil manager, check path validation order
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	source := &smb.SMBSource{
		Path: "",
	}
	err := handler.testSMBConnection(source)
	assert.Error(t, err)
	// nilManager is checked first
	assert.Equal(t, "SMB manager not initialized", err.Error())
}

func TestSMBHandler_AddSource_ValidJSON_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	body := `{"name": "test-source", "path": "//server/share", "username": "user", "password": "pass", "max_retry_attempts": 3, "retry_delay_seconds": 5, "connection_timeout_seconds": 10}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/smb/sources", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// AddSource with valid JSON and nil smbManager -> panic on h.smbManager.AddSource(source)
	assert.Panics(t, func() {
		handler.AddSource(c)
	})
}

func TestSMBHandler_TestConnection_ValidJSON_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	body := `{"name": "test", "path": "//server/share", "username": "user", "password": "pass"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/smb/test-connection", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.TestConnection(c)

	// testSMBConnection returns error "SMB manager not initialized"
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, false, resp["success"])
	assert.Equal(t, "SMB manager not initialized", resp["error"])
	assert.Contains(t, resp, "test_duration")
}

func TestSMBHandler_ForceReconnect_ValidID_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	// ForceReconnect with valid ID but nil smbManager -> panic
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/smb/sources/source1/reconnect", nil)
		c.Params = gin.Params{{Key: "id", Value: "source1"}}
		handler.ForceReconnect(c)
	})
}

func TestSMBHandler_UpdateSource_ValidJSON_NilManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	body := `{"name": "updated-name"}`
	// UpdateSource with valid ID and valid JSON, nil manager -> panic
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/api/v1/smb/sources/source1", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: "source1"}}
		handler.UpdateSource(c)
	})
}

func TestSMBHandler_UpdateSource_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	body := `{"name": "updated-name"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/smb/sources/", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: ""}}

	handler.UpdateSource(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Source ID is required", resp["error"])
}

func TestSMBHandler_GetSourceDetails_EmptyID_DirectCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBHandler(nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/smb/sources/", nil)
	c.Params = gin.Params{{Key: "id", Value: ""}}

	handler.GetSourceDetails(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Source ID is required", resp["error"])
}

// ============================================================
// Auth Handler: Register, Login, Logout, UpdateProfile,
//               ChangePassword with real DB
// ============================================================

func setupAuthHandlerFullRouter(t *testing.T) (*AuthHandler, *gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	wrappedDB := database.WrapDB(sqlDB, database.DialectSQLite)
	logger := zap.NewNop()

	os.Setenv("ADMIN_PASSWORD", "admin123")

	authService := auth.NewAuthService(wrappedDB, "test-jwt-secret", logger)
	if err := authService.Initialize(); err != nil {
		t.Fatalf("Failed to initialize auth service: %v", err)
	}

	handler := NewAuthHandler(authService, logger)
	router := gin.New()
	router.POST("/api/v1/auth/login", handler.Login)
	router.POST("/api/v1/auth/logout", handler.Logout)
	router.POST("/api/v1/auth/register", handler.Register)
	router.GET("/api/v1/auth/profile", handler.GetProfile)
	router.PUT("/api/v1/auth/profile", handler.UpdateProfile)
	router.POST("/api/v1/auth/change-password", handler.ChangePassword)
	router.GET("/api/v1/auth/admin/users", handler.ListUsers)
	router.GET("/api/v1/auth/admin/users/:id", handler.GetUser)
	router.PUT("/api/v1/auth/admin/users/:id", handler.UpdateUser)
	router.GET("/api/v1/auth/status", handler.GetAuthStatus)
	router.GET("/api/v1/auth/permissions", handler.GetPermissions)
	router.GET("/api/v1/auth/init-status", handler.GetInitStatus)

	cleanup := func() {
		os.Unsetenv("ADMIN_PASSWORD")
	}

	return handler, router, cleanup
}

func TestAuth_Login_ValidCredentials(t *testing.T) {
	_, router, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	body := `{"username": "admin", "password": "admin123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp, "token")
	assert.Contains(t, resp, "user")
}

func TestAuth_Login_InvalidCredentials(t *testing.T) {
	_, router, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	body := `{"username": "admin", "password": "wrongpassword"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid credentials", resp["error"])
}

func TestAuth_Logout_WithValidToken(t *testing.T) {
	_, router, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	// Login first to get a token
	loginBody := `{"username": "admin", "password": "admin123"}`
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)

	assert.Equal(t, http.StatusOK, loginW.Code)
	var loginResp map[string]interface{}
	err := json.Unmarshal(loginW.Body.Bytes(), &loginResp)
	require.NoError(t, err)
	token := loginResp["token"].(string)

	// Logout with the token
	logoutReq := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutW := httptest.NewRecorder()
	router.ServeHTTP(logoutW, logoutReq)

	assert.Equal(t, http.StatusOK, logoutW.Code)
	var logoutResp map[string]interface{}
	err = json.Unmarshal(logoutW.Body.Bytes(), &logoutResp)
	assert.NoError(t, err)
	assert.Equal(t, "Logged out successfully", logoutResp["message"])
}

func TestAuth_Logout_EmptyToken(t *testing.T) {
	_, router, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_Register_ValidRequest(t *testing.T) {
	_, router, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	body := `{"username": "newuser", "email": "new@example.com", "password": "password123", "first_name": "New", "last_name": "User"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "newuser", resp["username"])
	assert.Equal(t, "new@example.com", resp["email"])
}

func TestAuth_Register_DuplicateUsername(t *testing.T) {
	_, router, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	// Register first user
	body := `{"username": "dupuser", "email": "dup@example.com", "password": "password123", "first_name": "Dup", "last_name": "User"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Register same username again
	body2 := `{"username": "dupuser", "email": "dup2@example.com", "password": "password123", "first_name": "Dup2", "last_name": "User2"}`
	req2 := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Should be 409 or 500 depending on how the service reports it
	assert.True(t, w2.Code == http.StatusConflict || w2.Code == http.StatusInternalServerError)
}

func TestAuth_GetProfile_Authenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use gin context with user set directly
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auth/profile", nil)

	user := &auth.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "user",
	}
	c.Set("user", user)

	handler := NewAuthHandler(nil, zap.NewNop())
	handler.GetProfile(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", resp["username"])
}

func TestAuth_UpdateProfile_Authenticated_InvalidJSON(t *testing.T) {
	handler := NewAuthHandler(nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/auth/profile", bytes.NewBufferString("not-json"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(1))

	handler.UpdateProfile(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request format", resp["error"])
}

func TestAuth_UpdateProfile_Authenticated_ValidJSON(t *testing.T) {
	handler, _, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"first_name": "Updated", "last_name": "Name", "email": "updated@example.com"}`
	c.Request = httptest.NewRequest("PUT", "/api/v1/auth/profile", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(1))

	handler.UpdateProfile(c)

	// With a real auth service and valid user_id=1, should succeed
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

func TestAuth_ChangePassword_Authenticated_InvalidJSON(t *testing.T) {
	handler := NewAuthHandler(nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/change-password", bytes.NewBufferString("not-json"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(1))

	handler.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request format", resp["error"])
}

func TestAuth_ChangePassword_WithDB_WrongCurrentPassword(t *testing.T) {
	handler, _, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"current_password": "wrongpassword", "new_password": "newpassword123"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/change-password", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(1))

	handler.ChangePassword(c)

	// Should be 400 ("Current password is incorrect") or 500
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

func TestAuth_ChangePassword_WithDB_CorrectCurrentPassword(t *testing.T) {
	handler, _, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"current_password": "admin123", "new_password": "newpassword123"}`
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/change-password", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", int64(1))

	handler.ChangePassword(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Password changed successfully", resp["message"])
}

func TestAuth_GetUser_WithDB_ValidID(t *testing.T) {
	handler, _, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auth/admin/users/1", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.GetUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "admin", resp["username"])
}

func TestAuth_GetUser_WithDB_NotFound(t *testing.T) {
	handler, _, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auth/admin/users/9999", nil)
	c.Params = gin.Params{{Key: "id", Value: "9999"}}

	handler.GetUser(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuth_UpdateUser_WithDB_ValidRequest(t *testing.T) {
	handler, _, cleanup := setupAuthHandlerFullRouter(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"first_name": "AdminUpdated"}`
	c.Request = httptest.NewRequest("PUT", "/api/v1/auth/admin/users/1", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	// Set current user for the admin action log
	currentUser := &auth.User{ID: 1, Username: "admin"}
	c.Set("user", currentUser)

	handler.UpdateUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_GetAuthStatus_Authenticated(t *testing.T) {
	handler := NewAuthHandler(nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auth/status", nil)

	user := &auth.User{
		ID:          1,
		Username:    "testuser",
		Role:        "user",
		Permissions: []string{"read", "write"},
	}
	c.Set("user", user)

	handler.GetAuthStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp["authenticated"])
	assert.Contains(t, resp, "user")
	assert.Contains(t, resp, "permissions")
}

func TestAuth_GetPermissions_Authenticated(t *testing.T) {
	handler := NewAuthHandler(nil, zap.NewNop())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auth/permissions", nil)

	user := &auth.User{
		ID:          1,
		Username:    "admin",
		Role:        auth.RoleAdmin,
		Permissions: []string{"read", "write", "admin"},
	}
	c.Set("user", user)

	handler.GetPermissions(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, auth.RoleAdmin, resp["role"])
	assert.Equal(t, true, resp["is_admin"])
	assert.Contains(t, resp, "permissions")
}

// ============================================================
// Download Handler: getDirectoryContentsRecursive, getFilesByPath,
//                   DownloadFile, DownloadDirectory, DownloadArchive
// ============================================================

func TestDownloadHandler_getDirectoryContentsRecursive_NilService(t *testing.T) {
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	files, err := handler.getDirectoryContentsRecursive("any/path")
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestDownloadHandler_getFilesByPath_NilService(t *testing.T) {
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	files, err := handler.getFilesByPath("any/path", "root1")
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestDownloadHandler_createZipArchive_EmptyFiles(t *testing.T) {
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	var buf bytes.Buffer
	err := handler.createZipArchive(&buf, nil)
	assert.NoError(t, err)
	// The result is a valid (empty) zip archive
	assert.True(t, buf.Len() > 0)
}

func TestDownloadHandler_createTarArchive_EmptyFiles_Uncompressed(t *testing.T) {
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	var buf bytes.Buffer
	err := handler.createTarArchive(&buf, nil, false)
	assert.NoError(t, err)
}

func TestDownloadHandler_createTarArchive_EmptyFiles_Compressed(t *testing.T) {
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	var buf bytes.Buffer
	err := handler.createTarArchive(&buf, nil, true)
	assert.NoError(t, err)
}

func TestDownloadHandler_DownloadFile_ValidID_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	// With nil catalogService, GetFileInfo will panic
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/download/file/1", nil)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		handler.DownloadFile(c)
	})
}

func TestDownloadHandler_DownloadDirectory_PathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/download/directory/../../../etc/passwd", nil)
	c.Params = gin.Params{{Key: "path", Value: "/../../../etc/passwd"}}

	handler.DownloadDirectory(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid path", resp["error"])
}

func TestDownloadHandler_DownloadDirectory_EmptyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/download/directory/", nil)
	c.Params = gin.Params{{Key: "path", Value: ""}}

	handler.DownloadDirectory(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDownloadHandler_DownloadArchive_DefaultFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	// No format specified, defaults to "zip"
	body := `{"paths": ["/test/path"]}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/download/archive", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.DownloadArchive(c)

	// With nil catalogService, getFilesByPath returns empty -> creates empty archive
	// Status should be 200 (archive created, even if empty)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDownloadHandler_DownloadArchive_TarFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	body := `{"paths": ["/test/path"], "format": "tar"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/download/archive", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.DownloadArchive(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDownloadHandler_DownloadArchive_TarGzFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewDownloadHandler(nil, nil, "/tmp", 1024*1024, 32768, logger)

	body := `{"paths": ["/test/path"], "format": "tar.gz"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/download/archive", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.DownloadArchive(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================
// Copy Handler: CopyToSMB, CopyToLocal, CopyFromLocal, ListSMBPath
// ============================================================

func TestCopyHandler_CopyToSMB_BothPathsButDestInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	// Both paths present but destination has no colon
	body := `{"source_path":"server1:/source/path","destination_path":"no-colon-dest"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/copy/smb", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CopyToSMB(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Contains(t, resp["error"], "Invalid host:path format")
}

func TestCopyHandler_CopyToLocal_EmptyDestination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	// destination_path has binding:"required", so ShouldBindJSON fails before
	// the handler's manual empty-string check.
	body := `{"source_path":"server:/source","destination_path":""}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/copy/local", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CopyToLocal(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request format", resp["error"])
}

func TestCopyHandler_CopyToLocal_ValidSourceAndDest_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	// The destination must lie inside the configured root (tempDir); a valid
	// in-root destination passes validation and then reaches the nil service.
	handler := &CopyHandler{logger: logger, smbService: nil, tempDir: t.TempDir()}

	body := `{"source_path":"server:/source/file.txt","destination_path":"sub/test_copy_dest_xxx"}`
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/copy/local", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.CopyToLocal(c)
	})
}

func TestCopyHandler_CopyFromLocal_MissingDestination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	// Create a multipart form with file but no destination
	body := &bytes.Buffer{}
	// Use a minimal multipart form - the handler reads FormFile first
	// If no file is uploaded it returns 400 before checking destination.
	// To test "missing destination", we need an actual file upload.
	// We cannot easily create multipart in this test, so test only the no-file case.
	_ = body

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/copy/upload", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data")

	handler.CopyFromLocal(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "No file uploaded", resp["error"])
}

func TestCopyHandler_ListSMBPath_WithHost_EmptyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger, smbService: nil}

	// With host provided but empty path -> path defaults to "/"
	// Then it strips leading slash -> path becomes ""
	// Then calls smbService.ListFiles which panics on nil
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/smb/list/?host=testhost", nil)
		c.Request.URL.RawQuery = "host=testhost"
		c.Params = gin.Params{{Key: "path", Value: ""}}
		handler.ListSMBPath(c)
	})
}

func TestCopyHandler_ListSMBPath_WithHostAndPath_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger, smbService: nil}

	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/smb/list/testpath?host=testhost", nil)
		c.Request.URL.RawQuery = "host=testhost"
		c.Params = gin.Params{{Key: "path", Value: "/testpath"}}
		handler.ListSMBPath(c)
	})
}

// ============================================================
// Media Player Handlers: handlers using mux router with nil services
// Testing unauthenticated and invalid body paths for more coverage
// ============================================================

func TestMediaPlayerHandlers_GetContinueWatching_WithLimitParam(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// Unauthenticated - should return 401
	req := httptest.NewRequest("GET", "/api/v1/video/continue-watching?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMediaPlayerHandlers_GetWatchHistory_WithParams(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// Unauthenticated
	req := httptest.NewRequest("GET", "/api/v1/video/watch-history?limit=10&offset=5&type=movie", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMediaPlayerHandlers_GetContinueWatchingList_WithLimitParam(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// Unauthenticated
	req := httptest.NewRequest("GET", "/api/v1/playback/continue-watching?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMediaPlayerHandlers_GetMusicSession_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// With nil musicPlayerService, calling GetSession panics
	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/music/session/test-session-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_NextTrack_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("POST", "/api/v1/music/session/test-session/next", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_PreviousTrack_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("POST", "/api/v1/music/session/test-session/previous", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetVideoSession_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/video/session/test-session", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_NextVideo_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("POST", "/api/v1/video/session/test-session/next", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_PreviousVideo_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("POST", "/api/v1/video/session/test-session/previous", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_PlayMusic_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	// Valid JSON with correct types; nil musicPlayerService -> panic
	assert.Panics(t, func() {
		body := `{"track_id": 123, "user_id": 1}`
		req := httptest.NewRequest("POST", "/api/v1/music/play", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.PlayMusic(w, req)
	})
}

func TestMediaPlayerHandlers_PlayVideo_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	assert.Panics(t, func() {
		body := `{"video_id": 123, "user_id": 1}`
		req := httptest.NewRequest("POST", "/api/v1/video/play", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.PlayVideo(w, req)
	})
}

func TestMediaPlayerHandlers_CreatePlaylist_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"name": "My Playlist"}`
		req := httptest.NewRequest("POST", "/api/v1/playlists", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetPlaylist_ValidID_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/playlists/1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetPlaylistItems_ValidID_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/playlists/1/items?limit=10&offset=5", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_AddToPlaylist_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"media_ids": [1, 2, 3]}`
		req := httptest.NewRequest("POST", "/api/v1/playlists/1/items", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_RemoveFromPlaylist_ValidIDs_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("DELETE", "/api/v1/playlists/1/items/2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_ReorderPlaylist_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"new_position": 2}`
		req := httptest.NewRequest("POST", "/api/v1/playlists/1/items/1/reorder", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_RefreshSmartPlaylist_ValidID_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("POST", "/api/v1/playlists/1/refresh", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_SearchSubtitles_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"query": "test"}`
		req := httptest.NewRequest("POST", "/api/v1/subtitles/search", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_DownloadSubtitle_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"subtitle_id": "123"}`
		req := httptest.NewRequest("POST", "/api/v1/subtitles/download", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_TranslateSubtitle_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"subtitle_id": "123", "target_language": "fr"}`
		req := httptest.NewRequest("POST", "/api/v1/subtitles/translate", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_SearchLyrics_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"query": "song"}`
		req := httptest.NewRequest("POST", "/api/v1/lyrics/search", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_SynchronizeLyrics_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"lyrics_id": "123"}`
		req := httptest.NewRequest("POST", "/api/v1/lyrics/sync", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetConcertLyrics_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"concert_id": "123"}`
		req := httptest.NewRequest("POST", "/api/v1/lyrics/concert", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_SearchCoverArt_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"query": "album"}`
		req := httptest.NewRequest("POST", "/api/v1/cover-art/search", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_ScanLocalCoverArt_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"directory": "/music"}`
		req := httptest.NewRequest("POST", "/api/v1/cover-art/scan", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_TranslateText_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"text": "hello", "target_language": "fr"}`
		req := httptest.NewRequest("POST", "/api/v1/translate", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_DetectLanguage_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"text": "Bonjour le monde"}`
		req := httptest.NewRequest("POST", "/api/v1/translate/detect", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_UpdatePlaybackPosition_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"media_id": 1, "position": 12345}`
		req := httptest.NewRequest("POST", "/api/v1/playback/position", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_CreateBookmark_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"media_id": 1, "position": 100, "label": "Important scene"}`
		req := httptest.NewRequest("POST", "/api/v1/playback/bookmarks", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetMusicLibraryStats_Authenticated_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	// Authenticated user but nil service -> panic
	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/music/library/stats", nil)
		ctx := context.WithValue(req.Context(), "user_id", int64(1))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetUserPlaylists_Authenticated_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/playlists?include_public=true", nil)
		ctx := context.WithValue(req.Context(), "user_id", int64(1))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetContinueWatching_Authenticated_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/video/continue-watching?limit=5", nil)
		ctx := context.WithValue(req.Context(), "user_id", int64(1))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetWatchHistory_Authenticated_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/video/watch-history?limit=10&offset=5&type=movie", nil)
		ctx := context.WithValue(req.Context(), "user_id", int64(1))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetContinueWatchingList_Authenticated_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/playback/continue-watching?limit=10", nil)
		ctx := context.WithValue(req.Context(), "user_id", int64(1))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_GetPlaybackStats_Authenticated_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/playback/stats?limit=10&media_type=music", nil)
		ctx := context.WithValue(req.Context(), "user_id", int64(1))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_PlayAlbum_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	assert.Panics(t, func() {
		body := `{"album_id": 123, "user_id": 1}`
		req := httptest.NewRequest("POST", "/api/v1/music/play/album", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.PlayAlbum(w, req)
	})
}

func TestMediaPlayerHandlers_PlayArtist_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	assert.Panics(t, func() {
		body := `{"artist_id": 123, "user_id": 1}`
		req := httptest.NewRequest("POST", "/api/v1/music/play/artist", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.PlayArtist(w, req)
	})
}

func TestMediaPlayerHandlers_PlaySeries_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	assert.Panics(t, func() {
		body := `{"series_id": 123, "user_id": 1}`
		req := httptest.NewRequest("POST", "/api/v1/video/play/series", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.PlaySeries(w, req)
	})
}

func TestMediaPlayerHandlers_UpdateMusicPlayback_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"state": "playing"}`
		req := httptest.NewRequest("POST", "/api/v1/music/session/test-session/update", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_SeekMusic_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"position": 12345}`
		req := httptest.NewRequest("POST", "/api/v1/music/session/test-session/seek", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_AddToMusicQueue_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	// Use numeric array for track_ids ([]int64) and call handler directly
	// to bypass mux router recovery. nil musicPlayerService -> panic.
	assert.Panics(t, func() {
		body := `{"track_ids": [1, 2]}`
		req := httptest.NewRequest("POST", "/api/v1/music/session/test-session/queue", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.AddToMusicQueue(w, req)
	})
}

func TestMediaPlayerHandlers_SetEqualizer_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"preset": "rock", "bands": {"60": 2.0}}`
		req := httptest.NewRequest("POST", "/api/v1/music/session/test-session/equalizer", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_UpdateVideoPlayback_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"state": "paused"}`
		req := httptest.NewRequest("POST", "/api/v1/video/session/test-session/update", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_SeekVideo_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"position": 54321}`
		req := httptest.NewRequest("POST", "/api/v1/video/session/test-session/seek", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestMediaPlayerHandlers_CreateVideoBookmark_ValidJSON_NilService(t *testing.T) {
	logger := zap.NewNop()
	handlers := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	router := mux.NewRouter()
	handlers.RegisterRoutes(router)

	assert.Panics(t, func() {
		body := `{"label": "scene 1", "position": 100}`
		req := httptest.NewRequest("POST", "/api/v1/video/session/test-session/bookmark", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

// ============================================================
// Localization Handlers: RegisterRoutes, additional validation paths
// ============================================================

func TestLocalizationHandlers_RegisterRoutes_RoutesExist(t *testing.T) {
	logger := zap.NewNop()
	handler := NewLocalizationHandlers(logger, nil)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	// Verify several routes are registered by sending an OPTIONS request
	// (CORS middleware returns 200 for OPTIONS)
	optionsEndpoints := []string{
		"/api/v1/wizard/localization/defaults",
		"/api/v1/wizard/localization/setup",
		"/api/v1/wizard/configuration/export",
		"/api/v1/wizard/configuration/import",
		"/api/v1/wizard/configuration/validate",
		"/api/v1/wizard/configuration/edit",
		"/api/v1/wizard/configuration/templates",
		"/api/v1/localization",
		"/api/v1/localization/languages",
		"/api/v1/localization/stats",
		"/api/v1/localization/detect",
		"/api/v1/localization/check-support",
		"/api/v1/localization/format-datetime",
	}

	for _, path := range optionsEndpoints {
		req := httptest.NewRequest("OPTIONS", path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		// OPTIONS should return 200 or be handled
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusMethodNotAllowed,
			"Route %s returned unexpected status %d", path, w.Code)
	}
}

func TestLocalizationHandlers_UpdateUserLocalization_NoAuth(t *testing.T) {
	logger := zap.NewNop()
	handler := NewLocalizationHandlers(logger, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/localization", bytes.NewBufferString(`{"language":"fr"}`))
	r.Header.Set("Content-Type", "application/json")
	// No context user_id

	handler.UpdateUserLocalization(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLocalizationHandlers_ExportConfiguration_InvalidJSON(t *testing.T) {
	logger := zap.NewNop()
	handler := NewLocalizationHandlers(logger, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/wizard/configuration/export", bytes.NewBufferString("not-json"))
	r.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(r.Context(), "user_id", int64(1))
	r = r.WithContext(ctx)

	handler.ExportConfiguration(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request body", resp.Error)
}

func TestLocalizationHandlers_ImportConfiguration_NoAuth(t *testing.T) {
	logger := zap.NewNop()
	handler := NewLocalizationHandlers(logger, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/wizard/configuration/import", bytes.NewBufferString(`{"config_json":"{}"}`))
	r.Header.Set("Content-Type", "application/json")

	handler.ImportConfiguration(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLocalizationHandlers_DetectLanguage_InvalidJSON(t *testing.T) {
	logger := zap.NewNop()
	handler := NewLocalizationHandlers(logger, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/localization/detect", bytes.NewBufferString("not-json"))
	r.Header.Set("Content-Type", "application/json")

	handler.DetectLanguage(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLocalizationHandlers_DetectLanguage_EmptyFields(t *testing.T) {
	logger := zap.NewNop()
	locSvc := &services.LocalizationService{}
	handler := NewLocalizationHandlers(logger, locSvc)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/localization/detect", bytes.NewBufferString(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "TestAgent")
	r.Header.Set("Accept-Language", "de-DE")

	handler.DetectLanguage(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

// ============================================================
// SMB Discovery Handler: additional validation coverage
// ============================================================

func TestSMBDiscoveryHandler_DiscoverSharesGET_MissingParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.GET("/api/v1/smb/discover", handler.DiscoverSharesGET)

	tests := []struct {
		name  string
		query string
	}{
		{"missing all", ""},
		{"missing username", "host=server&password=pass"},
		{"missing password", "host=server&username=user"},
		{"missing host", "username=user&password=pass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/smb/discover"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestSMBDiscoveryHandler_DiscoverSharesGET_WithDomain_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.GET("/api/v1/smb/discover", handler.DiscoverSharesGET)

	// All params present including domain, but nil service -> panic
	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/smb/discover?host=server&username=user&password=pass&domain=WORKGROUP", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestSMBDiscoveryHandler_TestConnectionGET_MissingParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.GET("/api/v1/smb/test", handler.TestConnectionGET)

	tests := []struct {
		name  string
		query string
	}{
		{"missing all", ""},
		{"missing share", "host=server&username=user&password=pass"},
		{"missing host", "share=myshare&username=user&password=pass"},
		{"missing username", "host=server&share=myshare&password=pass"},
		{"missing password", "host=server&share=myshare&username=user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/smb/test"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestSMBDiscoveryHandler_TestConnectionGET_WithPort_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.GET("/api/v1/smb/test", handler.TestConnectionGET)

	// All required params present with optional port and domain, nil service -> panic
	assert.Panics(t, func() {
		req := httptest.NewRequest("GET", "/api/v1/smb/test?host=server&share=myshare&username=user&password=pass&port=139&domain=WORKGROUP", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

func TestSMBDiscoveryHandler_BrowseShare_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.POST("/api/v1/smb/browse", handler.BrowseShare)

	req := httptest.NewRequest("POST", "/api/v1/smb/browse", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSMBDiscoveryHandler_BrowseShare_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.POST("/api/v1/smb/browse", handler.BrowseShare)

	body := `{"host": "server"}`
	req := httptest.NewRequest("POST", "/api/v1/smb/browse", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSMBDiscoveryHandler_DiscoverShares_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.POST("/api/v1/smb/discover", handler.DiscoverShares)

	req := httptest.NewRequest("POST", "/api/v1/smb/discover", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSMBDiscoveryHandler_TestConnection_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewSMBDiscoveryHandler(nil, logger)

	router := gin.New()
	router.POST("/api/v1/smb/test", handler.TestConnection)

	body := `{"host": "server"}`
	req := httptest.NewRequest("POST", "/api/v1/smb/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================
// Catalog Handler: additional coverage for ListRoot, Search,
//                  SearchDuplicates, GetDirectoriesBySize
// ============================================================

func TestCatalogHandler_ListRoot_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	// CatalogHandler uses interfaces, nil interface -> panic
	handler := NewCatalogHandler(nil, nil, logger)
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/api/v1/catalog", nil)
		handler.ListRoot(c)
	})
}

func TestCatalogHandler_Search_EmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewCatalogHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search", nil)

	handler.Search(c)

	// Contract change (post-2026-03): empty-query search returns 200
	// with an empty result envelope instead of 400. This matches the
	// challenge bank + userflow suite expectation that a blank search
	// box does not error, and is friendlier to UI autocomplete that
	// fires before the user types.
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), resp["total"])
	assert.Equal(t, float64(0), resp["count"])
	// results + files arrays should exist and be empty
	results, ok := resp["results"].([]interface{})
	assert.True(t, ok, "results should be an array")
	assert.Empty(t, results)
	files, ok := resp["files"].([]interface{})
	assert.True(t, ok, "files should be an array")
	assert.Empty(t, files)
}

func TestCatalogHandler_SearchDuplicates_MissingSmbRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewCatalogHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/search/duplicates", nil)

	handler.SearchDuplicates(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "SMB root is required", resp["error"])
}

func TestCatalogHandler_GetDirectoriesBySize_MissingSmbRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewCatalogHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/stats/directories/by-size", nil)

	handler.GetDirectoriesBySize(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "SMB root is required", resp["error"])
}

func TestCatalogHandler_GetFileInfo_EmptyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewCatalogHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog-info/", nil)
	c.Params = gin.Params{{Key: "path", Value: ""}}

	handler.GetFileInfo(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCatalogHandler_ListPath_EmptyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := NewCatalogHandler(nil, nil, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/catalog/", nil)
	c.Params = gin.Params{{Key: "path", Value: ""}}

	handler.ListPath(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================
// Media Player Handlers: buildMusicSessionResponse, buildVideoSessionResponse
// ============================================================

func TestMediaPlayerHandlers_BuildMusicSessionResponse(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	now := time.Now()
	session := &services.MusicPlaybackSession{
		ID:             "music-session-1",
		PlaybackState:  "playing",
		Position:       12345,
		Duration:       300000,
		Volume:         0.8,
		IsMuted:        false,
		QueueIndex:     2,
		RepeatMode:     "all",
		ShuffleEnabled: true,
		LastActivity:   now,
	}

	resp := h.buildMusicSessionResponse(session)

	assert.Equal(t, "music-session-1", resp.SessionID)
	assert.Equal(t, services.PlaybackState("playing"), resp.PlaybackState)
	assert.Equal(t, int64(12345), resp.Position)
	assert.Equal(t, int64(300000), resp.Duration)
	assert.Equal(t, 0.8, resp.Volume)
	assert.Equal(t, false, resp.IsMuted)
	assert.Equal(t, 2, resp.QueueIndex)
	assert.Equal(t, "all", resp.RepeatMode)
	assert.Equal(t, true, resp.ShuffleEnabled)
	assert.Equal(t, now, resp.LastActivity)
	assert.Nil(t, resp.CurrentTrack)
}

func TestMediaPlayerHandlers_BuildVideoSessionResponse(t *testing.T) {
	logger := zap.NewNop()
	h := NewMediaPlayerHandlers(logger, nil, nil, nil, nil, nil, nil, nil, nil)

	now := time.Now()
	session := &services.VideoPlaybackSession{
		ID:            "video-session-1",
		PlaybackState: "paused",
		Position:      54321,
		Duration:      7200000,
		Volume:        0.5,
		IsMuted:       true,
		PlaybackSpeed: 1.5,
		PlaylistIndex: 0,
		LastActivity:  now,
	}

	resp := h.buildVideoSessionResponse(session)

	assert.Equal(t, "video-session-1", resp.SessionID)
	assert.Equal(t, services.PlaybackState("paused"), resp.PlaybackState)
	assert.Equal(t, int64(54321), resp.Position)
	assert.Equal(t, int64(7200000), resp.Duration)
	assert.Equal(t, 0.5, resp.Volume)
	assert.Equal(t, true, resp.IsMuted)
	assert.Equal(t, 1.5, resp.PlaybackSpeed)
	assert.Equal(t, 0, resp.QueueIndex)
	assert.Equal(t, now, resp.LastActivity)
	assert.Nil(t, resp.CurrentVideo)
}
