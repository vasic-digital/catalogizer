package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/tests"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const testJWTSecret = "test-sync-handler-secret-key"

// SyncHandlerTestSuite is the test suite for SyncHandler
type SyncHandlerTestSuite struct {
	suite.Suite
	handler     *SyncHandler
	router      *gin.Engine
	db          *database.DB
	syncService *services.SyncService
	authService *services.AuthService
	authToken   string
	testUserID  int
}

func (s *SyncHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
}

func (s *SyncHandlerTestSuite) SetupTest() {
	// Local sync paths are confined to configured roots (pathguard); these tests use /tmp.
	// TestPathConfine_* override this with their own narrow root.
	s.T().Setenv("CATALOGIZER_SYNC_ROOTS", "/tmp")
	// Create in-memory test database with all required tables
	sqlDB := tests.SetupTestDB(s.T())
	s.db = database.WrapDB(sqlDB, database.DialectSQLite)

	// Create additional tables needed for sync and auth
	s.createSyncTables()
	s.createAuthTables()

	// Insert test user with role
	s.testUserID = 1
	s.insertTestRole()
	s.insertTestUserSession()

	// Create services
	userRepo := repository.NewUserRepository(s.db)
	syncRepo := repository.NewSyncRepository(s.db)
	s.authService = services.NewAuthService(userRepo, testJWTSecret)
	s.syncService = services.NewSyncService(syncRepo, userRepo, s.authService)

	// Generate a valid JWT token for the test user
	s.authToken = s.generateTestToken(s.testUserID, "testuser", 1, "1")

	// Create handler
	s.handler = NewSyncHandler(s.syncService, s.authService)

	// Setup router
	s.router = gin.New()
	s.router.POST("/sync/endpoints", s.handler.CreateEndpoint)
	s.router.GET("/sync/endpoints", s.handler.GetUserEndpoints)
	s.router.GET("/sync/endpoints/:id", s.handler.GetEndpoint)
	s.router.PUT("/sync/endpoints/:id", s.handler.UpdateEndpoint)
	s.router.DELETE("/sync/endpoints/:id", s.handler.DeleteEndpoint)
	s.router.POST("/sync/endpoints/:id/sync", s.handler.StartSync)
	s.router.GET("/sync/sessions", s.handler.GetUserSessions)
	s.router.GET("/sync/sessions/:id", s.handler.GetSession)
	s.router.POST("/sync/schedules", s.handler.ScheduleSync)
	s.router.GET("/sync/statistics", s.handler.GetSyncStatistics)
	s.router.POST("/sync/cleanup", s.handler.CleanupOldSessions)
}

func (s *SyncHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *SyncHandlerTestSuite) createSyncTables() {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS sync_endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			url TEXT NOT NULL,
			username TEXT,
			password TEXT,
			sync_direction TEXT NOT NULL,
			local_path TEXT,
			remote_path TEXT,
			sync_settings TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_sync_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			sync_type TEXT NOT NULL,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			duration INTEGER,
			total_files INTEGER DEFAULT 0,
			synced_files INTEGER DEFAULT 0,
			failed_files INTEGER DEFAULT 0,
			skipped_files INTEGER DEFAULT 0,
			error_message TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			frequency TEXT NOT NULL,
			last_run DATETIME,
			next_run DATETIME,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, table := range tables {
		_, err := s.db.Exec(table)
		if err != nil {
			s.T().Fatalf("Failed to create sync table: %v", err)
		}
	}
}

func (s *SyncHandlerTestSuite) createAuthTables() {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			permissions TEXT DEFAULT '[]',
			is_system BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			session_token TEXT,
			refresh_token TEXT,
			device_info TEXT DEFAULT '{}',
			ip_address TEXT,
			user_agent TEXT,
			is_active BOOLEAN DEFAULT 1,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_activity_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, table := range tables {
		_, err := s.db.Exec(table)
		if err != nil {
			s.T().Fatalf("Failed to create auth table: %v", err)
		}
	}
}

func (s *SyncHandlerTestSuite) insertTestRole() {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO roles (id, name, description, permissions, is_system) VALUES (?, ?, ?, ?, ?)`,
		1, "admin", "Administrator", `["*"]`, true,
	)
	if err != nil {
		s.T().Fatalf("Failed to insert test role: %v", err)
	}

	// Update the test user (created by SetupTestDB) to have non-NULL password_hash and salt,
	// which are required by UserRepository.GetByID (scans into non-nullable string fields)
	_, err = s.db.Exec(
		`UPDATE users SET password_hash = ?, salt = ?, role_id = ? WHERE id = ?`,
		"$2a$10$fakehashfortest", "fakesalt", 1, s.testUserID,
	)
	if err != nil {
		s.T().Fatalf("Failed to update test user: %v", err)
	}
}

func (s *SyncHandlerTestSuite) insertTestUserSession() {
	expires := time.Now().Add(24 * time.Hour)
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO user_sessions (id, user_id, session_token, refresh_token, device_info, is_active, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1, s.testUserID, "test-token", "test-refresh", "{}", true, expires,
	)
	if err != nil {
		s.T().Fatalf("Failed to insert test user session: %v", err)
	}
}

func (s *SyncHandlerTestSuite) generateTestToken(userID int, username string, roleID int, sessionID string) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":    userID,
		"username":   username,
		"role_id":    roleID,
		"session_id": sessionID,
		"exp":        jwt.NewNumericDate(now.Add(24 * time.Hour)),
		"iat":        jwt.NewNumericDate(now),
		"iss":        "catalogizer",
		"sub":        fmt.Sprintf("%d", userID),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		s.T().Fatalf("Failed to generate test token: %v", err)
	}
	return tokenString
}

// createTestEndpointInDB inserts a sync endpoint directly in the DB and returns its ID.
func (s *SyncHandlerTestSuite) createTestEndpointInDB(userID int, name, endpointType, status string) int {
	now := time.Now()
	result, err := s.db.Exec(
		`INSERT INTO sync_endpoints (user_id, name, type, url, username, password, sync_direction, local_path, remote_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, name, endpointType, "http://example.com", "user", "pass", "upload", "/tmp/local", "/tmp/remote", status, now, now,
	)
	if err != nil {
		s.T().Fatalf("Failed to create test endpoint: %v", err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

// createTestSessionInDB inserts a sync session directly in the DB and returns its ID.
func (s *SyncHandlerTestSuite) createTestSessionInDB(endpointID, userID int, status string) int {
	now := time.Now()
	result, err := s.db.Exec(
		`INSERT INTO sync_sessions (endpoint_id, user_id, status, sync_type, started_at, total_files, synced_files, failed_files, skipped_files)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		endpointID, userID, status, "manual", now, 10, 5, 1, 4,
	)
	if err != nil {
		s.T().Fatalf("Failed to create test session: %v", err)
	}
	id, _ := result.LastInsertId()
	return int(id)
}

func (s *SyncHandlerTestSuite) doRequest(method, path string, body interface{}, withAuth bool) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+s.authToken)
	}

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *SyncHandlerTestSuite) parseResponse(w *httptest.ResponseRecorder) map[string]interface{} {
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(s.T(), err)
	return resp
}

// --- Constructor tests ---

func (s *SyncHandlerTestSuite) TestNewSyncHandler() {
	handler := NewSyncHandler(nil, nil)
	assert.NotNil(s.T(), handler)
	assert.Nil(s.T(), handler.syncService)
	assert.Nil(s.T(), handler.authService)
}

func (s *SyncHandlerTestSuite) TestNewSyncHandler_WithServices() {
	handler := NewSyncHandler(s.syncService, s.authService)
	assert.NotNil(s.T(), handler)
	assert.Equal(s.T(), s.syncService, handler.syncService)
	assert.Equal(s.T(), s.authService, handler.authService)
}

// --- CreateEndpoint tests ---

func (s *SyncHandlerTestSuite) TestCreateEndpoint_Unauthorized_NoHeader() {
	w := s.doRequest("POST", "/sync/endpoints", map[string]string{"name": "test"}, false)

	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), false, resp["success"])
	assert.Equal(s.T(), "Unauthorized", resp["error"])
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_Unauthorized_InvalidToken() {
	req := httptest.NewRequest("POST", "/sync/endpoints", bytes.NewBufferString(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token-value")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_InvalidJSON() {
	req := httptest.NewRequest("POST", "/sync/endpoints", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), false, resp["success"])
	assert.Contains(s.T(), resp["error"], "Invalid request body")
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_EmptyBody() {
	w := s.doRequest("POST", "/sync/endpoints", map[string]string{}, true)

	// The service will validate and return error for missing fields
	assert.True(s.T(), w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_InvalidEndpoint_MissingName() {
	body := map[string]interface{}{
		"type":           "local",
		"url":            "http://example.com",
		"sync_direction": "upload",
		"local_path":     "/tmp/test",
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)

	// Service validation returns "invalid" error -> 400
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), false, resp["success"])
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_InvalidEndpoint_MissingURL() {
	body := map[string]interface{}{
		"name":           "Test Endpoint",
		"type":           "local",
		"sync_direction": "upload",
		"local_path":     "/tmp/test",
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_InvalidEndpoint_MissingType() {
	body := map[string]interface{}{
		"name":           "Test Endpoint",
		"url":            "http://example.com",
		"sync_direction": "upload",
		"local_path":     "/tmp/test",
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_InvalidSyncType() {
	body := map[string]interface{}{
		"name":           "Test Endpoint",
		"type":           "unsupported_type",
		"url":            "http://example.com",
		"sync_direction": "upload",
		"local_path":     "/tmp/test",
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["details"], "invalid")
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_InvalidSyncDirection() {
	body := map[string]interface{}{
		"name":           "Test Endpoint",
		"type":           "local",
		"url":            "http://example.com",
		"sync_direction": "sideways",
		"local_path":     "/tmp/test",
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_ValidLocalEndpoint() {
	body := map[string]interface{}{
		"name":           "Local Sync",
		"type":           "local",
		"url":            "file:///tmp/sync",
		"sync_direction": "upload",
		"local_path":     "/tmp/local",
		"remote_path":    "/tmp/remote",
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)

	assert.Equal(s.T(), http.StatusCreated, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "Local Sync", data["name"])
	assert.Equal(s.T(), "local", data["type"])
	assert.Equal(s.T(), "active", data["status"])
}

func (s *SyncHandlerTestSuite) TestCreateEndpoint_AuthHeaderWithoutBearerPrefix() {
	req := httptest.NewRequest("POST", "/sync/endpoints", bytes.NewBufferString(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", s.authToken) // No "Bearer " prefix

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	// The getCurrentUser method only strips "Bearer " prefix if present.
	// Without it, the full JWT string is still a valid token, so auth succeeds.
	// The request then fails on service validation (missing required fields).
	assert.True(s.T(), w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError,
		"Expected 400 or 500 since auth succeeds but body is incomplete, got %d", w.Code)
}

// --- GetUserEndpoints tests ---

func (s *SyncHandlerTestSuite) TestGetUserEndpoints_Unauthorized() {
	w := s.doRequest("GET", "/sync/endpoints", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetUserEndpoints_Empty() {
	w := s.doRequest("GET", "/sync/endpoints", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	// When no endpoints exist, the repo returns nil which serializes as JSON null.
	// We only verify the request succeeds and the success flag is true.
}

func (s *SyncHandlerTestSuite) TestGetUserEndpoints_WithEndpoints() {
	// Create some test endpoints directly in DB
	s.createTestEndpointInDB(s.testUserID, "Endpoint 1", "local", "active")
	s.createTestEndpointInDB(s.testUserID, "Endpoint 2", "local", "active")

	w := s.doRequest("GET", "/sync/endpoints", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].([]interface{})
	assert.Equal(s.T(), 2, len(data))
}

func (s *SyncHandlerTestSuite) TestGetUserEndpoints_DoesNotReturnOtherUserEndpoints() {
	// Create endpoint for user 1 and user 2
	s.createTestEndpointInDB(s.testUserID, "My Endpoint", "local", "active")
	s.createTestEndpointInDB(2, "Other User Endpoint", "local", "active")

	w := s.doRequest("GET", "/sync/endpoints", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].([]interface{})
	assert.Equal(s.T(), 1, len(data))
	firstEndpoint := data[0].(map[string]interface{})
	assert.Equal(s.T(), "My Endpoint", firstEndpoint["name"])
}

// --- GetEndpoint tests ---

func (s *SyncHandlerTestSuite) TestGetEndpoint_Unauthorized() {
	w := s.doRequest("GET", "/sync/endpoints/1", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetEndpoint_InvalidID_NotNumber() {
	w := s.doRequest("GET", "/sync/endpoints/abc", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["error"], "Invalid endpoint ID")
}

func (s *SyncHandlerTestSuite) TestGetEndpoint_InvalidID_Float() {
	w := s.doRequest("GET", "/sync/endpoints/1.5", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetEndpoint_InvalidID_Special() {
	specialIDs := []string{"!@#", "id-abc", "--1"}
	for _, id := range specialIDs {
		w := s.doRequest("GET", "/sync/endpoints/"+id, nil, true)
		assert.Equal(s.T(), http.StatusBadRequest, w.Code, "ID %s should be rejected", id)
	}
}

func (s *SyncHandlerTestSuite) TestGetEndpoint_NotFound() {
	w := s.doRequest("GET", "/sync/endpoints/9999", nil, true)
	assert.Equal(s.T(), http.StatusNotFound, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), false, resp["success"])
}

func (s *SyncHandlerTestSuite) TestGetEndpoint_Valid() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "My Endpoint", "local", "active")

	w := s.doRequest("GET", fmt.Sprintf("/sync/endpoints/%d", endpointID), nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "My Endpoint", data["name"])
}

func (s *SyncHandlerTestSuite) TestGetEndpoint_OtherUserEndpoint_AdminCanView() {
	// Admin user (role with "*" permission) should be able to view other user endpoints
	endpointID := s.createTestEndpointInDB(2, "Other User EP", "local", "active")

	w := s.doRequest("GET", fmt.Sprintf("/sync/endpoints/%d", endpointID), nil, true)

	// Admin has "*" permission which includes share.view
	assert.Equal(s.T(), http.StatusOK, w.Code)
}

// --- UpdateEndpoint tests ---

func (s *SyncHandlerTestSuite) TestUpdateEndpoint_Unauthorized() {
	w := s.doRequest("PUT", "/sync/endpoints/1", map[string]string{"name": "updated"}, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestUpdateEndpoint_InvalidID() {
	w := s.doRequest("PUT", "/sync/endpoints/abc", map[string]string{"name": "updated"}, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["error"], "Invalid endpoint ID")
}

func (s *SyncHandlerTestSuite) TestUpdateEndpoint_InvalidJSON() {
	req := httptest.NewRequest("PUT", "/sync/endpoints/1", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["error"], "Invalid request body")
}

func (s *SyncHandlerTestSuite) TestUpdateEndpoint_NotFound() {
	w := s.doRequest("PUT", "/sync/endpoints/9999", map[string]string{"name": "updated"}, true)
	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SyncHandlerTestSuite) TestUpdateEndpoint_ValidUpdate() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Original Name", "local", "active")

	body := map[string]interface{}{
		"name": "Updated Name",
	}
	w := s.doRequest("PUT", fmt.Sprintf("/sync/endpoints/%d", endpointID), body, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "Updated Name", data["name"])
}

func (s *SyncHandlerTestSuite) TestUpdateEndpoint_UpdateSyncDirection() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Dir Endpoint", "local", "active")

	body := map[string]interface{}{
		"sync_direction": "download",
	}
	w := s.doRequest("PUT", fmt.Sprintf("/sync/endpoints/%d", endpointID), body, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "download", data["sync_direction"])
}

func (s *SyncHandlerTestSuite) TestUpdateEndpoint_DeactivateEndpoint() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Active EP", "local", "active")

	isActive := false
	body := map[string]interface{}{
		"is_active": isActive,
	}
	w := s.doRequest("PUT", fmt.Sprintf("/sync/endpoints/%d", endpointID), body, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "inactive", data["status"])
}

// --- DeleteEndpoint tests ---

func (s *SyncHandlerTestSuite) TestDeleteEndpoint_Unauthorized() {
	w := s.doRequest("DELETE", "/sync/endpoints/1", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestDeleteEndpoint_InvalidID() {
	w := s.doRequest("DELETE", "/sync/endpoints/abc", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["error"], "Invalid endpoint ID")
}

func (s *SyncHandlerTestSuite) TestDeleteEndpoint_NotFound() {
	w := s.doRequest("DELETE", "/sync/endpoints/9999", nil, true)
	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SyncHandlerTestSuite) TestDeleteEndpoint_Valid() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Delete Me", "local", "active")

	w := s.doRequest("DELETE", fmt.Sprintf("/sync/endpoints/%d", endpointID), nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Contains(s.T(), data["message"], "deleted")

	// Verify it's actually deleted
	w2 := s.doRequest("GET", fmt.Sprintf("/sync/endpoints/%d", endpointID), nil, true)
	assert.Equal(s.T(), http.StatusNotFound, w2.Code)
}

// --- StartSync tests ---

func (s *SyncHandlerTestSuite) TestStartSync_Unauthorized() {
	w := s.doRequest("POST", "/sync/endpoints/1/sync", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestStartSync_InvalidID() {
	w := s.doRequest("POST", "/sync/endpoints/abc/sync", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["error"], "Invalid endpoint ID")
}

func (s *SyncHandlerTestSuite) TestStartSync_NotFound() {
	w := s.doRequest("POST", "/sync/endpoints/9999/sync", nil, true)
	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SyncHandlerTestSuite) TestStartSync_InactiveEndpoint() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Inactive EP", "local", "inactive")

	w := s.doRequest("POST", fmt.Sprintf("/sync/endpoints/%d/sync", endpointID), nil, true)

	assert.Equal(s.T(), http.StatusConflict, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["details"], "not active")
}

func (s *SyncHandlerTestSuite) TestStartSync_ValidActiveEndpoint() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Active Sync EP", "local", "active")

	w := s.doRequest("POST", fmt.Sprintf("/sync/endpoints/%d/sync", endpointID), nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "running", data["status"])
	assert.Equal(s.T(), "manual", data["sync_type"])
}

// --- GetUserSessions tests ---

func (s *SyncHandlerTestSuite) TestGetUserSessions_Unauthorized() {
	w := s.doRequest("GET", "/sync/sessions", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_Empty() {
	w := s.doRequest("GET", "/sync/sessions", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_WithSessions() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Session EP", "local", "active")
	s.createTestSessionInDB(endpointID, s.testUserID, "completed")
	s.createTestSessionInDB(endpointID, s.testUserID, "failed")

	w := s.doRequest("GET", "/sync/sessions", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].([]interface{})
	assert.Equal(s.T(), 2, len(data))
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_DefaultPagination() {
	// Default limit is 50, offset is 0
	w := s.doRequest("GET", "/sync/sessions", nil, true)
	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_CustomPagination() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Paged EP", "local", "active")
	for i := 0; i < 5; i++ {
		s.createTestSessionInDB(endpointID, s.testUserID, "completed")
	}

	w := s.doRequest("GET", "/sync/sessions?limit=2&offset=0", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].([]interface{})
	assert.Equal(s.T(), 2, len(data))
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_LimitCapped() {
	// Limit > 200 should be ignored (uses default 50)
	w := s.doRequest("GET", "/sync/sessions?limit=500", nil, true)
	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_InvalidLimitIgnored() {
	// Non-numeric limit should be silently ignored
	w := s.doRequest("GET", "/sync/sessions?limit=abc", nil, true)
	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_NegativeLimitIgnored() {
	w := s.doRequest("GET", "/sync/sessions?limit=-5", nil, true)
	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_NegativeOffsetIgnored() {
	w := s.doRequest("GET", "/sync/sessions?offset=-1", nil, true)
	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetUserSessions_ZeroLimitIgnored() {
	w := s.doRequest("GET", "/sync/sessions?limit=0", nil, true)
	assert.Equal(s.T(), http.StatusOK, w.Code)
}

// --- GetSession tests ---

func (s *SyncHandlerTestSuite) TestGetSession_Unauthorized() {
	w := s.doRequest("GET", "/sync/sessions/1", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetSession_InvalidID() {
	w := s.doRequest("GET", "/sync/sessions/abc", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
	resp := s.parseResponse(w)
	assert.Contains(s.T(), resp["error"], "Invalid session ID")
}

func (s *SyncHandlerTestSuite) TestGetSession_NotFound() {
	w := s.doRequest("GET", "/sync/sessions/9999", nil, true)
	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetSession_Valid() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Session Detail EP", "local", "active")
	sessionID := s.createTestSessionInDB(endpointID, s.testUserID, "completed")

	w := s.doRequest("GET", fmt.Sprintf("/sync/sessions/%d", sessionID), nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "completed", data["status"])
	assert.Equal(s.T(), float64(10), data["total_files"])
	assert.Equal(s.T(), float64(5), data["synced_files"])
}

func (s *SyncHandlerTestSuite) TestGetSession_FloatID() {
	w := s.doRequest("GET", "/sync/sessions/1.5", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// --- ScheduleSync tests ---

func (s *SyncHandlerTestSuite) TestScheduleSync_Unauthorized() {
	body := map[string]interface{}{
		"endpoint_id": 1,
		"frequency":   "daily",
	}
	w := s.doRequest("POST", "/sync/schedules", body, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestScheduleSync_InvalidJSON() {
	req := httptest.NewRequest("POST", "/sync/schedules", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestScheduleSync_MissingEndpointID() {
	body := map[string]interface{}{
		"frequency": "daily",
	}
	w := s.doRequest("POST", "/sync/schedules", body, true)

	// endpoint_id is required via binding:"required"
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestScheduleSync_MissingFrequency() {
	body := map[string]interface{}{
		"endpoint_id": 1,
	}
	w := s.doRequest("POST", "/sync/schedules", body, true)

	// frequency is required via binding:"required"
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestScheduleSync_EndpointNotFound() {
	body := map[string]interface{}{
		"endpoint_id": 9999,
		"frequency":   "daily",
	}
	w := s.doRequest("POST", "/sync/schedules", body, true)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SyncHandlerTestSuite) TestScheduleSync_Valid() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Scheduled EP", "local", "active")

	body := map[string]interface{}{
		"endpoint_id": endpointID,
		"frequency":   "daily",
	}
	w := s.doRequest("POST", "/sync/schedules", body, true)

	assert.Equal(s.T(), http.StatusCreated, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "daily", data["frequency"])
	assert.Equal(s.T(), true, data["is_active"])
}

func (s *SyncHandlerTestSuite) TestScheduleSync_HourlyFrequency() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Hourly EP", "local", "active")

	body := map[string]interface{}{
		"endpoint_id": endpointID,
		"frequency":   "hourly",
	}
	w := s.doRequest("POST", "/sync/schedules", body, true)

	assert.Equal(s.T(), http.StatusCreated, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.Equal(s.T(), "hourly", data["frequency"])
}

func (s *SyncHandlerTestSuite) TestScheduleSync_WeeklyFrequency() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Weekly EP", "local", "active")

	body := map[string]interface{}{
		"endpoint_id": endpointID,
		"frequency":   "weekly",
	}
	w := s.doRequest("POST", "/sync/schedules", body, true)

	assert.Equal(s.T(), http.StatusCreated, w.Code)
}

func (s *SyncHandlerTestSuite) TestScheduleSync_MonthlyFrequency() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Monthly EP", "local", "active")

	body := map[string]interface{}{
		"endpoint_id": endpointID,
		"frequency":   "monthly",
	}
	w := s.doRequest("POST", "/sync/schedules", body, true)

	assert.Equal(s.T(), http.StatusCreated, w.Code)
}

// --- GetSyncStatistics tests ---

func (s *SyncHandlerTestSuite) TestGetSyncStatistics_Unauthorized() {
	w := s.doRequest("GET", "/sync/statistics", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetSyncStatistics_DefaultDateRange() {
	w := s.doRequest("GET", "/sync/statistics", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.NotNil(s.T(), data["start_date"])
	assert.NotNil(s.T(), data["end_date"])
}

func (s *SyncHandlerTestSuite) TestGetSyncStatistics_CustomDateRange_RFC3339() {
	start := time.Now().AddDate(0, -2, 0).Format(time.RFC3339)
	end := time.Now().Format(time.RFC3339)

	w := s.doRequest("GET", fmt.Sprintf("/sync/statistics?start_date=%s&end_date=%s", start, end), nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
}

func (s *SyncHandlerTestSuite) TestGetSyncStatistics_CustomDateRange_DateOnly() {
	w := s.doRequest("GET", "/sync/statistics?start_date=2025-01-01&end_date=2025-12-31", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
}

func (s *SyncHandlerTestSuite) TestGetSyncStatistics_InvalidDateIgnored() {
	// Invalid dates should be silently ignored and defaults used
	w := s.doRequest("GET", "/sync/statistics?start_date=not-a-date&end_date=also-invalid", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
}

func (s *SyncHandlerTestSuite) TestGetSyncStatistics_WithSessionData() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Stats EP", "local", "active")
	s.createTestSessionInDB(endpointID, s.testUserID, "completed")
	s.createTestSessionInDB(endpointID, s.testUserID, "failed")

	w := s.doRequest("GET", "/sync/statistics", nil, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.NotNil(s.T(), data["by_status"])
	assert.NotNil(s.T(), data["by_type"])
}

// --- CleanupOldSessions tests ---

func (s *SyncHandlerTestSuite) TestCleanupOldSessions_Unauthorized() {
	w := s.doRequest("POST", "/sync/cleanup", nil, false)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestCleanupOldSessions_DefaultDays() {
	// No body or empty body should default to 30 days
	req := httptest.NewRequest("POST", "/sync/cleanup", nil)
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	assert.Equal(s.T(), true, resp["success"])
	data := resp["data"].(map[string]interface{})
	assert.Contains(s.T(), data["message"], "30 days")
}

func (s *SyncHandlerTestSuite) TestCleanupOldSessions_CustomDays() {
	body := map[string]interface{}{
		"older_than_days": 60,
	}
	w := s.doRequest("POST", "/sync/cleanup", body, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.Contains(s.T(), data["message"], "60 days")
}

func (s *SyncHandlerTestSuite) TestCleanupOldSessions_ZeroDays_DefaultsTo30() {
	body := map[string]interface{}{
		"older_than_days": 0,
	}
	w := s.doRequest("POST", "/sync/cleanup", body, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.Contains(s.T(), data["message"], "30 days")
}

func (s *SyncHandlerTestSuite) TestCleanupOldSessions_NegativeDays_DefaultsTo30() {
	body := map[string]interface{}{
		"older_than_days": -10,
	}
	w := s.doRequest("POST", "/sync/cleanup", body, true)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.Contains(s.T(), data["message"], "30 days")
}

func (s *SyncHandlerTestSuite) TestCleanupOldSessions_InvalidJSON_DefaultsTo30() {
	req := httptest.NewRequest("POST", "/sync/cleanup", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := s.parseResponse(w)
	data := resp["data"].(map[string]interface{})
	assert.Contains(s.T(), data["message"], "30 days")
}

func (s *SyncHandlerTestSuite) TestCleanupOldSessions_ActuallyDeletesOldSessions() {
	endpointID := s.createTestEndpointInDB(s.testUserID, "Cleanup EP", "local", "active")

	// Create an old completed session (90 days ago)
	oldTime := time.Now().AddDate(0, 0, -90)
	_, err := s.db.Exec(
		`INSERT INTO sync_sessions (endpoint_id, user_id, status, sync_type, started_at, completed_at, total_files, synced_files, failed_files, skipped_files)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		endpointID, s.testUserID, "completed", "manual", oldTime, oldTime, 10, 10, 0, 0,
	)
	assert.NoError(s.T(), err)

	// Create a recent session
	s.createTestSessionInDB(endpointID, s.testUserID, "completed")

	// Cleanup sessions older than 30 days
	body := map[string]interface{}{
		"older_than_days": 30,
	}
	w := s.doRequest("POST", "/sync/cleanup", body, true)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	// The old session should have been cleaned up; the recent one should remain
	w2 := s.doRequest("GET", "/sync/sessions", nil, true)
	assert.Equal(s.T(), http.StatusOK, w2.Code)
	resp := s.parseResponse(w2)
	data := resp["data"].([]interface{})
	assert.Equal(s.T(), 1, len(data), "Only the recent session should remain after cleanup")
}

// --- getCurrentUser edge cases ---

func (s *SyncHandlerTestSuite) TestGetCurrentUser_EmptyAuthHeader() {
	req := httptest.NewRequest("GET", "/sync/endpoints", nil)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetCurrentUser_BearerOnly() {
	req := httptest.NewRequest("GET", "/sync/endpoints", nil)
	req.Header.Set("Authorization", "Bearer ")

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetCurrentUser_ExpiredToken() {
	// Generate an expired token
	now := time.Now().Add(-2 * time.Hour)
	claims := jwt.MapClaims{
		"user_id":    s.testUserID,
		"username":   "testuser",
		"role_id":    1,
		"session_id": "1",
		"exp":        jwt.NewNumericDate(now.Add(-1 * time.Hour)), // already expired
		"iat":        jwt.NewNumericDate(now),
		"iss":        "catalogizer",
		"sub":        "1",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, _ := token.SignedString([]byte(testJWTSecret))

	req := httptest.NewRequest("GET", "/sync/endpoints", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)
}

// --- End-to-end workflow tests ---

func (s *SyncHandlerTestSuite) TestWorkflow_CreateGetUpdateDelete() {
	// 1. Create endpoint
	createBody := map[string]interface{}{
		"name":           "Workflow EP",
		"type":           "local",
		"url":            "file:///tmp/workflow",
		"sync_direction": "upload",
		"local_path":     "/tmp/workflow-local",
		"remote_path":    "/tmp/workflow-remote",
	}
	w1 := s.doRequest("POST", "/sync/endpoints", createBody, true)
	assert.Equal(s.T(), http.StatusCreated, w1.Code)
	resp1 := s.parseResponse(w1)
	data1 := resp1["data"].(map[string]interface{})
	endpointID := int(data1["id"].(float64))

	// 2. Get endpoint
	w2 := s.doRequest("GET", fmt.Sprintf("/sync/endpoints/%d", endpointID), nil, true)
	assert.Equal(s.T(), http.StatusOK, w2.Code)
	resp2 := s.parseResponse(w2)
	data2 := resp2["data"].(map[string]interface{})
	assert.Equal(s.T(), "Workflow EP", data2["name"])

	// 3. Update endpoint
	updateBody := map[string]interface{}{
		"name":           "Updated Workflow EP",
		"sync_direction": "bidirectional",
	}
	w3 := s.doRequest("PUT", fmt.Sprintf("/sync/endpoints/%d", endpointID), updateBody, true)
	assert.Equal(s.T(), http.StatusOK, w3.Code)
	resp3 := s.parseResponse(w3)
	data3 := resp3["data"].(map[string]interface{})
	assert.Equal(s.T(), "Updated Workflow EP", data3["name"])
	assert.Equal(s.T(), "bidirectional", data3["sync_direction"])

	// 4. List endpoints
	w4 := s.doRequest("GET", "/sync/endpoints", nil, true)
	assert.Equal(s.T(), http.StatusOK, w4.Code)
	resp4 := s.parseResponse(w4)
	data4 := resp4["data"].([]interface{})
	assert.GreaterOrEqual(s.T(), len(data4), 1)

	// 5. Delete endpoint
	w5 := s.doRequest("DELETE", fmt.Sprintf("/sync/endpoints/%d", endpointID), nil, true)
	assert.Equal(s.T(), http.StatusOK, w5.Code)

	// 6. Confirm deleted
	w6 := s.doRequest("GET", fmt.Sprintf("/sync/endpoints/%d", endpointID), nil, true)
	assert.Equal(s.T(), http.StatusNotFound, w6.Code)
}

func (s *SyncHandlerTestSuite) TestWorkflow_CreateEndpointStartSyncGetSession() {
	// 1. Create active endpoint
	createBody := map[string]interface{}{
		"name":           "Sync Workflow EP",
		"type":           "local",
		"url":            "file:///tmp/sync-wf",
		"sync_direction": "upload",
		"local_path":     "/tmp/sync-wf-local",
	}
	w1 := s.doRequest("POST", "/sync/endpoints", createBody, true)
	assert.Equal(s.T(), http.StatusCreated, w1.Code)
	resp1 := s.parseResponse(w1)
	data1 := resp1["data"].(map[string]interface{})
	endpointID := int(data1["id"].(float64))

	// 2. Start sync
	w2 := s.doRequest("POST", fmt.Sprintf("/sync/endpoints/%d/sync", endpointID), nil, true)
	assert.Equal(s.T(), http.StatusOK, w2.Code)
	resp2 := s.parseResponse(w2)
	data2 := resp2["data"].(map[string]interface{})
	sessionID := int(data2["id"].(float64))
	assert.Equal(s.T(), "running", data2["status"])

	// 3. Get session
	w3 := s.doRequest("GET", fmt.Sprintf("/sync/sessions/%d", sessionID), nil, true)
	assert.Equal(s.T(), http.StatusOK, w3.Code)
	resp3 := s.parseResponse(w3)
	data3 := resp3["data"].(map[string]interface{})
	assert.Equal(s.T(), float64(endpointID), data3["endpoint_id"])

	// 4. List user sessions
	w4 := s.doRequest("GET", "/sync/sessions", nil, true)
	assert.Equal(s.T(), http.StatusOK, w4.Code)
	resp4 := s.parseResponse(w4)
	data4 := resp4["data"].([]interface{})
	assert.GreaterOrEqual(s.T(), len(data4), 1)
}

// --- OverflowID edge case ---

func (s *SyncHandlerTestSuite) TestGetEndpoint_OverflowID() {
	w := s.doRequest("GET", "/sync/endpoints/99999999999999999999", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestDeleteEndpoint_OverflowID() {
	w := s.doRequest("DELETE", "/sync/endpoints/99999999999999999999", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestStartSync_OverflowID() {
	w := s.doRequest("POST", "/sync/endpoints/99999999999999999999/sync", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SyncHandlerTestSuite) TestGetSession_OverflowID() {
	w := s.doRequest("GET", "/sync/sessions/99999999999999999999", nil, true)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// Run the test suite
func TestSyncHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(SyncHandlerTestSuite))
}
