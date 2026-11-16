package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/database"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type MediaHandlerTestSuite struct {
	suite.Suite
	handler *AndroidTVMediaHandler
	router  *gin.Engine
}

func (suite *MediaHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
}

func (suite *MediaHandlerTestSuite) SetupTest() {
	// Initialize handler with nil database to test guard paths
	suite.handler = NewAndroidTVMediaHandler(nil)

	suite.router = gin.New()
	suite.router.GET("/api/v1/media/:id", suite.handler.GetMediaByID)
	suite.router.PUT("/api/v1/media/:id/progress", suite.handler.UpdateWatchProgress)
	suite.router.PUT("/api/v1/media/:id/favorite", suite.handler.UpdateFavoriteStatus)
}

// Constructor tests

func (suite *MediaHandlerTestSuite) TestNewAndroidTVMediaHandler() {
	handler := NewAndroidTVMediaHandler(nil)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.db)
}

// GetMediaByID tests

func (suite *MediaHandlerTestSuite) TestGetMediaByID_InvalidID() {
	req := httptest.NewRequest("GET", "/api/v1/media/abc", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), false, body["success"])
	assert.Contains(suite.T(), body["error"], "Invalid media ID")
}

func (suite *MediaHandlerTestSuite) TestGetMediaByID_NegativeID() {
	req := httptest.NewRequest("GET", "/api/v1/media/-1", nil)
	w := httptest.NewRecorder()

	// Negative ID parses fine as int64, but db is nil so it will panic
	assert.Panics(suite.T(), func() {
		suite.router.ServeHTTP(w, req)
	})
}

func (suite *MediaHandlerTestSuite) TestGetMediaByID_EmptyID() {
	// With the route pattern :id, an empty ID results in 404 from the router
	req := httptest.NewRequest("GET", "/api/v1/media/", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Router returns 301 redirect or 404 for trailing slash
	assert.True(suite.T(), w.Code == http.StatusMovedPermanently || w.Code == http.StatusNotFound)
}

func (suite *MediaHandlerTestSuite) TestGetMediaByID_FloatID() {
	req := httptest.NewRequest("GET", "/api/v1/media/1.5", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *MediaHandlerTestSuite) TestGetMediaByID_MethodNotAllowed() {
	req := httptest.NewRequest("POST", "/api/v1/media/1", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// UpdateWatchProgress tests

func (suite *MediaHandlerTestSuite) TestUpdateWatchProgress_InvalidID() {
	body := bytes.NewBufferString(`{"progress": 0.5}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/abc/progress", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"], "Invalid media ID")
}

func (suite *MediaHandlerTestSuite) TestUpdateWatchProgress_InvalidJSON() {
	req := httptest.NewRequest("PUT", "/api/v1/media/1/progress", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"], "Invalid request body")
}

func (suite *MediaHandlerTestSuite) TestUpdateWatchProgress_MissingProgressField() {
	body := bytes.NewBufferString(`{"something": 0.5}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/1/progress", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"], "Progress field is required")
}

func (suite *MediaHandlerTestSuite) TestUpdateWatchProgress_ProgressTooHigh() {
	body := bytes.NewBufferString(`{"progress": 1.5}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/1/progress", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"], "Progress must be between 0.0 and 1.0")
}

func (suite *MediaHandlerTestSuite) TestUpdateWatchProgress_ProgressNegative() {
	body := bytes.NewBufferString(`{"progress": -0.1}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/1/progress", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *MediaHandlerTestSuite) TestUpdateWatchProgress_EmptyBody() {
	req := httptest.NewRequest("PUT", "/api/v1/media/1/progress", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// UpdateFavoriteStatus tests

func (suite *MediaHandlerTestSuite) TestUpdateFavoriteStatus_InvalidID() {
	body := bytes.NewBufferString(`{"favorite": true}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/abc/favorite", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"], "Invalid media ID")
}

func (suite *MediaHandlerTestSuite) TestUpdateFavoriteStatus_InvalidJSON() {
	req := httptest.NewRequest("PUT", "/api/v1/media/1/favorite", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *MediaHandlerTestSuite) TestUpdateFavoriteStatus_MissingFavoriteField() {
	body := bytes.NewBufferString(`{"something": true}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/1/favorite", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"], "Favorite field is required")
}

func (suite *MediaHandlerTestSuite) TestUpdateFavoriteStatus_EmptyBody() {
	req := httptest.NewRequest("PUT", "/api/v1/media/1/favorite", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// TestUpdateFavoriteStatus_HappyPath is the regression test for
// FIX-QA-2026-04-21-001. Earlier tests only exercised 400-path
// branches (invalid ID, invalid JSON, missing field), so the real
// UPDATE never ran — which is how the missing-column 500 slipped
// through into production unnoticed until the 2026-04-20 RunAll log
// analysis surfaced it. This test runs the handler against an
// in-memory SQLite schema with the migration-v18 columns and asserts
// 200 + affected row.
func TestUpdateFavoriteStatus_HappyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)

	ctx := context.Background()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (7, 'u7', 'u7@example.test', 'x', 'x', 2)`); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	// Seed one media_item via the migration-v8 entity schema.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO media_items (media_type_id, title, status)
		VALUES (1, 'Test Movie', 'detected')
	`); err != nil {
		t.Fatalf("seed media_items: %v", err)
	}

	handler := NewAndroidTVMediaHandler(db)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", 7) })
	r.PUT("/api/v1/media/:id/favorite", handler.UpdateFavoriteStatus)

	body := bytes.NewBufferString(`{"favorite": true}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/1/favorite", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"expected 200 after migration-v18 added is_favorite + updated_at; got body: %s", w.Body.String())

	var got int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_favorites WHERE user_id = 7 AND media_id = 1`,
	).Scan(&got); err != nil {
		t.Fatalf("read back is_favorite: %v", err)
	}
	assert.Equal(t, 1, got, "user 7 must have a user_favorites row after favoriting")

	body = bytes.NewBufferString(`{"favorite": false}`)
	req = httptest.NewRequest("PUT", "/api/v1/media/1/favorite", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_favorites WHERE user_id = 7 AND media_id = 1`,
	).Scan(&got); err != nil {
		t.Fatalf("read back is_favorite: %v", err)
	}
	assert.Equal(t, 0, got, "user 7 row must be gone after toggle-off")
}

func TestUpdateFavoriteStatus_MediaNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	handler := NewAndroidTVMediaHandler(db)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", 7) })
	r.PUT("/api/v1/media/:id/favorite", handler.UpdateFavoriteStatus)

	body := bytes.NewBufferString(`{"favorite": true}`)
	req := httptest.NewRequest("PUT", "/api/v1/media/9999999/favorite", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMediaHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(MediaHandlerTestSuite))
}
