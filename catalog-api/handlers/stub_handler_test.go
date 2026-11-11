package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func setupMediaQueryHandlerTest(t *testing.T) (*MediaQueryHandler, *database.DB, sqlmock.Sqlmock, func()) {
	gin.SetMode(gin.TestMode)

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	itemRepo := repository.NewMediaItemRepository(db)
	userRepo := repository.NewUserRepository(db)

	handler := NewMediaQueryHandler(db, itemRepo, userRepo)

	cleanup := func() {
		sqlDB.Close()
	}

	return handler, db, mock, cleanup
}

func TestMediaQueryHandler_GetRecentMedia_DefaultPagination(t *testing.T) {
	handler, _, mock, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	// Setup mock expectations
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM media_items").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery("SELECT mi.id, mi.media_type_id").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "media_type_id", "name", "title", "original_title",
			"year", "description", "rating", "runtime", "status", "last_updated",
		}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/media/recent", nil)

	handler.GetRecentMedia(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	items, ok := response["items"].([]interface{})
	require.True(t, ok)
	assert.Len(t, items, 0)

	_, hasTotal := response["total"]
	assert.True(t, hasTotal)
}

func TestMediaQueryHandler_GetPopularMedia_DefaultPagination(t *testing.T) {
	handler, _, mock, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	// Setup mock expectations
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM media_items").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery("SELECT mi.id, mi.media_type_id").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "media_type_id", "name", "title", "original_title",
			"year", "description", "rating", "runtime", "status", "last_updated", "fav_count",
		}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/media/popular", nil)

	handler.GetPopularMedia(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	_, hasItems := response["items"]
	assert.True(t, hasItems)
	_, hasTotal := response["total"]
	assert.True(t, hasTotal)
}

func TestMediaQueryHandler_GetMediaByPath_MissingPath(t *testing.T) {
	handler, _, _, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/media/by-path", nil)

	handler.GetMediaByPath(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "path parameter is required")
}

func TestMediaQueryHandler_GetMediaByPath_ValidPath(t *testing.T) {
	handler, _, mock, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	// Setup mock expectations
	mock.ExpectQuery("SELECT COUNT\\(DISTINCT mi.id\\)").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery("SELECT DISTINCT mi.id, mi.media_type_id").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "media_type_id", "name", "title", "original_title",
			"year", "description", "rating", "runtime", "status", "last_updated",
		}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/media/by-path?path=/media/movies", nil)

	handler.GetMediaByPath(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMediaQueryHandler_ChangePassword_Unauthorized_NoUserID(t *testing.T) {
	handler, _, _, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	bodyJSON, _ := json.Marshal(map[string]string{
		"current_password": "old",
		"new_password":     "newpass123",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password",
		bytes.NewReader(bodyJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	// No user_id set

	handler.ChangePassword(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "unauthorized")
}

func TestMediaQueryHandler_ChangePassword_NotFound(t *testing.T) {
	handler, _, mock, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	userID := 99999

	// Setup mock for user not found
	mock.ExpectQuery("SELECT id, username, password_hash, salt, role").
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	bodyJSON, _ := json.Marshal(map[string]string{
		"current_password": "old",
		"new_password":     "newpass123",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password",
		bytes.NewReader(bodyJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", userID)

	handler.ChangePassword(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "not found")
}

func TestMediaQueryHandler_ChangePassword_WrongPassword(t *testing.T) {
	handler, _, mock, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	userID := 1
	salt := "testsalt123"
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"+salt), bcrypt.DefaultCost)

	// Setup mock for user found with correct column names matching the repository
	mock.ExpectQuery("SELECT id, username, email, password_hash, salt").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "email", "password_hash", "salt", "role_id",
			"first_name", "last_name", "display_name", "avatar_url", "time_zone",
			"language", "is_active", "is_locked", "locked_until", "failed_login_attempts",
			"last_login_at", "last_login_ip", "created_at", "updated_at", "settings",
		}).AddRow(userID, "testuser", "test@example.com", string(hash), salt, 1,
			nil, nil, nil, nil, nil,
			nil, true, false, nil, 0,
			nil, nil, time.Now(), time.Now(), nil))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	bodyJSON, _ := json.Marshal(map[string]string{
		"current_password": "wrongpassword",
		"new_password":     "newpass123",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password",
		bytes.NewReader(bodyJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", userID)

	handler.ChangePassword(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "incorrect")
}

func TestMediaQueryHandler_ChangePassword_Success(t *testing.T) {
	handler, _, mock, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	userID := 1
	salt := "testsalt123"
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"+salt), bcrypt.DefaultCost)

	// Setup mock for user found with correct column names
	mock.ExpectQuery("SELECT id, username, email, password_hash, salt").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "email", "password_hash", "salt", "role_id",
			"first_name", "last_name", "display_name", "avatar_url", "time_zone",
			"language", "is_active", "is_locked", "locked_until", "failed_login_attempts",
			"last_login_at", "last_login_ip", "created_at", "updated_at", "settings",
		}).AddRow(userID, "testuser", "test@example.com", string(hash), salt, 1,
			nil, nil, nil, nil, nil,
			nil, true, false, nil, 0,
			nil, nil, time.Now(), time.Now(), nil))

	// Setup mock for password update - match the actual query
	mock.ExpectExec("UPDATE users SET password_hash").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// session revocation after a password change (F3)
	mock.ExpectExec("UPDATE user_sessions").
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	bodyJSON, _ := json.Marshal(map[string]string{
		"current_password": "oldpassword",
		"new_password":     "newpass123",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password",
		bytes.NewReader(bodyJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", userID)

	handler.ChangePassword(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestMediaQueryHandler_GetInitStatus(t *testing.T) {
	handler, _, _, cleanup := setupMediaQueryHandlerTest(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/init-status", nil)

	handler.GetInitStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	initialized, ok := response["initialized"].(bool)
	require.True(t, ok)
	assert.True(t, initialized)
}

func TestNewMediaQueryHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	itemRepo := repository.NewMediaItemRepository(db)
	userRepo := repository.NewUserRepository(db)

	handler := NewMediaQueryHandler(db, itemRepo, userRepo)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.NotNil(t, handler.itemRepo)
	assert.NotNil(t, handler.userRepo)
}
