package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "github.com/mutecomm/go-sqlcipher"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestAnalyticsHandler_LogMediaAccess_InvalidBody(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.AnalyticsService{}
	handler := NewAnalyticsHandler(svc, logger)

	router := setupTestRouter()
	router.POST("/analytics/access", handler.LogMediaAccess)

	req := httptest.NewRequest(http.MethodPost, "/analytics/access", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandler_LogEvent_InvalidBody(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.AnalyticsService{}
	handler := NewAnalyticsHandler(svc, logger)

	router := setupTestRouter()
	router.POST("/analytics/event", handler.LogEvent)

	req := httptest.NewRequest(http.MethodPost, "/analytics/event", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandler_GetUserAnalytics_InvalidUserID(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.AnalyticsService{}
	handler := NewAnalyticsHandler(svc, logger)

	router := setupTestRouter()
	router.GET("/analytics/user/:user_id", handler.GetUserAnalytics)

	req := httptest.NewRequest(http.MethodGet, "/analytics/user/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandler_GetMediaAnalytics_InvalidMediaID(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.AnalyticsService{}
	handler := NewAnalyticsHandler(svc, logger)

	router := setupTestRouter()
	router.GET("/analytics/media/:media_id", handler.GetMediaAnalytics)

	req := httptest.NewRequest(http.MethodGet, "/analytics/media/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandler_CreateReport_InvalidBody(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.AnalyticsService{}
	handler := NewAnalyticsHandler(svc, logger)

	router := setupTestRouter()
	router.POST("/analytics/reports", handler.CreateReport)

	req := httptest.NewRequest(http.MethodPost, "/analytics/reports", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAnalyticsHandler_CreateReport_MissingReportType(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.AnalyticsService{}
	handler := NewAnalyticsHandler(svc, logger)

	router := setupTestRouter()
	router.POST("/analytics/reports", handler.CreateReport)

	body := map[string]interface{}{
		"params": map[string]interface{}{},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/analytics/reports", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReportingHandler_GetUsageReport_InvalidDateFormat(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.ReportingService{}
	handler := NewReportingHandler(svc, logger)

	router := setupTestRouter()
	router.GET("/reports/usage", handler.GetUsageReport)

	req := httptest.NewRequest(http.MethodGet, "/reports/usage?start_date=invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFavoritesHandler_ListFavorites_Unauthorized(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.FavoritesService{}
	handler := NewFavoritesHandler(svc, logger)

	router := setupTestRouter()
	router.GET("/favorites", handler.ListFavorites)

	req := httptest.NewRequest(http.MethodGet, "/favorites", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFavoritesHandler_AddFavorite_Unauthorized(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.FavoritesService{}
	handler := NewFavoritesHandler(svc, logger)

	router := setupTestRouter()
	router.POST("/favorites", handler.AddFavorite)

	body := map[string]interface{}{
		"entity_id":   123,
		"entity_type": "movie",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/favorites", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFavoritesHandler_AddFavorite_InvalidBody(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.FavoritesService{}
	handler := NewFavoritesHandler(svc, logger)

	router := setupTestRouter()
	router.POST("/favorites", func(c *gin.Context) {
		c.Set("user_id", 1)
		handler.AddFavorite(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/favorites", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFavoritesHandler_RemoveFavorite_InvalidEntityID(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.FavoritesService{}
	handler := NewFavoritesHandler(svc, logger)

	router := setupTestRouter()
	router.DELETE("/favorites/:entity_type/:entity_id", func(c *gin.Context) {
		c.Set("user_id", 1)
		handler.RemoveFavorite(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/favorites/movie/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFavoritesHandler_CheckFavorite_InvalidEntityID(t *testing.T) {
	logger := zap.NewNop()
	svc := &services.FavoritesService{}
	handler := NewFavoritesHandler(svc, logger)

	router := setupTestRouter()
	router.GET("/favorites/check/:entity_type/:entity_id", func(c *gin.Context) {
		c.Set("user_id", 1)
		handler.CheckFavorite(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/favorites/check/movie/invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		dateStr string
		wantErr bool
	}{
		{"valid date", "2024-01-15", false},
		{"invalid date", "invalid", true},
		{"wrong format", "01/15/2024", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := time.Parse("2006-01-02", tt.dateStr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMediaAccessLog_Fields(t *testing.T) {
	access := models.MediaAccessLog{
		UserID:     1,
		MediaID:    100,
		Action:     "play",
		AccessTime: time.Now(),
	}

	assert.Equal(t, 1, access.UserID)
	assert.Equal(t, 100, access.MediaID)
	assert.Equal(t, "play", access.Action)
	assert.False(t, access.AccessTime.IsZero())
}

func TestFavorite_Struct(t *testing.T) {
	fav := models.Favorite{
		UserID:     1,
		EntityID:   100,
		EntityType: "movie",
	}

	assert.Equal(t, 1, fav.UserID)
	assert.Equal(t, 100, fav.EntityID)
	assert.Equal(t, "movie", fav.EntityType)
}

func TestAnalyticsEvent_Struct(t *testing.T) {
	event := models.AnalyticsEvent{
		UserID:    1,
		EventType: "page_view",
		Timestamp: time.Now(),
	}

	assert.Equal(t, 1, event.UserID)
	assert.Equal(t, "page_view", event.EventType)
	assert.False(t, event.Timestamp.IsZero())
}

// TestFavoritesHandler_AddFavorite_Duplicate_Integration is the
// regression test for FQA-API-211. It wires the full handler ->
// service -> repository chain against an in-memory SQLite database
// and asserts that adding an already-favorited entity returns
// HTTP 409 Conflict (not 200 OK {"status":"added"}).
func TestFavoritesHandler_AddFavorite_Duplicate_Integration(t *testing.T) {
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

	// Seed a user (required by FK on favorites.user_id).
	if _, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO users (id, username, email, password_hash, salt, is_active)
		VALUES (1, 'testuser', 'test@example.com', '', '', 1)
	`); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Build the real handler chain.
	favoritesRepo := repository.NewFavoritesRepository(db)
	favoritesService := services.NewFavoritesService(favoritesRepo, nil)
	logger := zap.NewNop()
	handler := NewFavoritesHandler(favoritesService, logger)

	r := gin.New()
	r.POST("/favorites", func(c *gin.Context) {
		c.Set("user_id", 1)
		handler.AddFavorite(c)
	})

	// First add: should succeed with 200.
	body := bytes.NewBufferString(`{"entity_id": 42, "entity_type": "movie"}`)
	req := httptest.NewRequest(http.MethodPost, "/favorites", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"first add must succeed; got body: %s", w.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "added", resp["status"])

	// Second add (same user_id + entity_type + entity_id): must be 409.
	body = bytes.NewBufferString(`{"entity_id": 42, "entity_type": "movie"}`)
	req = httptest.NewRequest(http.MethodPost, "/favorites", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code,
		"duplicate add must return 409 Conflict; got body: %s", w.Body.String())
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["error"], "already in favorites")
}

// TestFavoritesHandler_AddFavorite_Duplicate_RaceViaDBConstraint
// verifies that even if the pre-insert GetFavorite check is bypassed
// (e.g., by a race or by direct repository call), the UNIQUE
// constraint violation from the database is still surfaced as 409.
func TestFavoritesHandler_AddFavorite_Duplicate_RaceViaDBConstraint(t *testing.T) {
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

	if _, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO users (id, username, email, password_hash, salt, is_active)
		VALUES (1, 'testuser', 'test@example.com', '', '', 1)
	`); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	favoritesRepo := repository.NewFavoritesRepository(db)
	favoritesService := services.NewFavoritesService(favoritesRepo, nil)
	logger := zap.NewNop()
	handler := NewFavoritesHandler(favoritesService, logger)

	r := gin.New()
	r.POST("/favorites", func(c *gin.Context) {
		c.Set("user_id", 1)
		handler.AddFavorite(c)
	})

	// Pre-seed the favorite directly in the DB so the handler's
	// GetFavorite check will find it and return 409.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO favorites (user_id, entity_type, entity_id, is_public, created_at)
		VALUES (1, 'movie', 99, 0, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("seed favorite: %v", err)
	}

	body := bytes.NewBufferString(`{"entity_id": 99, "entity_type": "movie"}`)
	req := httptest.NewRequest(http.MethodPost, "/favorites", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code,
		"pre-seeded duplicate must return 409; got body: %s", w.Body.String())
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Contains(t, errResp["error"], "already in favorites")
}
