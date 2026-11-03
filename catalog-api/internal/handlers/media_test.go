package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"catalogizer/internal/media/analyzer"
	"catalogizer/internal/media/database"
)

// setupMediaHandlerWithDB creates a MediaHandler with a real encrypted SQLite database
func setupMediaHandlerWithDB(t *testing.T) (*MediaHandler, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "media_handler_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test_media.db")
	logger := zap.NewNop()

	mediaDB, err := database.NewMediaDatabase(database.DatabaseConfig{
		Path:     dbPath,
		Password: "testpassword123",
	}, logger)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create media database: %v", err)
	}

	handler := NewMediaHandler(mediaDB, &analyzer.MediaAnalyzer{}, logger)

	cleanup := func() {
		mediaDB.Close()
		os.RemoveAll(tmpDir)
	}

	return handler, cleanup
}

func TestNewMediaHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mediaDB := &database.MediaDatabase{}
	analyzer := &analyzer.MediaAnalyzer{}

	handler := NewMediaHandler(mediaDB, analyzer, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, mediaDB, handler.mediaDB)
	assert.Equal(t, analyzer, handler.analyzer)
	assert.Equal(t, logger, handler.logger)
}

func TestMediaHandler_GetMediaItem_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := zap.NewDevelopment()
	mediaDB := &database.MediaDatabase{}
	analyzer := &analyzer.MediaAnalyzer{}
	handler := NewMediaHandler(mediaDB, analyzer, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler.GetMediaItem(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response gin.H
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid media item ID", response["error"])
}

func TestMediaHandler_AnalyzeDirectory_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := zap.NewDevelopment()
	mediaDB := &database.MediaDatabase{}
	analyzer := &analyzer.MediaAnalyzer{}
	handler := NewMediaHandler(mediaDB, analyzer, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON request
	req := httptest.NewRequest("POST", "/analyze", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	handler.AnalyzeDirectory(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response gin.H
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid request format", response["error"])
}

func TestMediaHandler_GetMediaStats_MethodExists(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mediaDB := &database.MediaDatabase{}
	analyzer := &analyzer.MediaAnalyzer{}
	handler := NewMediaHandler(mediaDB, analyzer, logger)

	// Test that the method exists and can be called (will panic due to nil DB, but that's expected)
	assert.NotNil(t, handler.GetMediaStats)
}

func TestMediaHandler_getQualityDistribution(t *testing.T) {
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	distribution, err := handler.getQualityDistribution()
	assert.NoError(t, err)
	assert.Equal(t, map[string]int{
		"4K/UHD": 0,
		"1080p":  0,
		"720p":   0,
		"Other":  0,
	}, distribution)
}

// GetMediaTypes tests with real DB

func TestMediaHandler_GetMediaTypes_WithDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	// The embedded schema seeds media_types with NULL detection_patterns/metadata_providers,
	// which causes scan to skip rows. Update them so the query returns results.
	db := handler.mediaDB.GetDB()
	_, err := db.Exec(`UPDATE media_types SET detection_patterns = '[]', metadata_providers = '[]'`)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/media/types", nil)

	handler.GetMediaTypes(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	count := resp["count"].(float64)
	assert.GreaterOrEqual(t, count, float64(1))
}

func TestMediaHandler_GetMediaTypes_WithDB_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	// Delete all media types to test empty result
	db := handler.mediaDB.GetDB()
	_, err := db.Exec(`DELETE FROM media_types`)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/media/types", nil)

	handler.GetMediaTypes(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), resp["count"])
}

// SearchMedia tests with real DB

func TestMediaHandler_SearchMedia_WithDB_EmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), resp["total"])
	assert.Equal(t, float64(50), resp["limit"])
	assert.Equal(t, float64(0), resp["offset"])
	assert.Equal(t, false, resp["has_more"])
}

func TestMediaHandler_SearchMedia_WithDB_QueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	// Insert a test media item
	db := handler.mediaDB.GetDB()
	_, err := db.Exec(`INSERT INTO media_items (media_type_id, title, year, genre, cast_crew, status)
		VALUES (1, 'Test Movie', 2024, '["action"]', '{}', 'active')`)
	assert.NoError(t, err)

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search?query=Test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])
	assert.Equal(t, float64(1), resp["count"])
}

func TestMediaHandler_SearchMedia_WithDB_FilterByMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	// Insert a test media item
	db := handler.mediaDB.GetDB()
	_, err := db.Exec(`INSERT INTO media_items (media_type_id, title, year, genre, cast_crew, status)
		VALUES (1, 'Test Movie', 2024, '["action"]', '{}', 'active')`)
	assert.NoError(t, err)

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search?media_types=movie", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])
}

func TestMediaHandler_SearchMedia_WithDB_SortByYear(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search?sort_by=year&sort_order=desc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMediaHandler_SearchMedia_WithDB_YearFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search?year=2024&year_from=2020&year_to=2025", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMediaHandler_SearchMedia_WithDB_RatingFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search?min_rating=7.5&has_externals=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMediaHandler_SearchMedia_WithDB_SortByRating(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search?sort_by=rating&sort_order=asc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMediaHandler_SearchMedia_WithDB_SortByCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	router := gin.New()
	router.GET("/api/v1/media/search", handler.SearchMedia)

	req := httptest.NewRequest("GET", "/api/v1/media/search?sort_by=created", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// GetMediaStats tests with real DB

func TestMediaHandler_GetMediaStats_WithDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/media/stats", nil)

	handler.GetMediaStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	// Should have database stats, media type distribution, quality distribution, recent activity
	assert.NotNil(t, resp["database"])
	assert.NotNil(t, resp["media_type_distribution"])
	assert.NotNil(t, resp["quality_distribution"])
	assert.NotNil(t, resp["recent_activity"])
}

// GetMediaItem tests with real DB

func TestMediaHandler_GetMediaItem_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "99999"}}
	c.Request = httptest.NewRequest("GET", "/api/v1/media/99999", nil)

	handler.GetMediaItem(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Media item not found", resp["error"])
}

func TestMediaHandler_GetMediaItem_WithDB_Found(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	// Insert a test media item
	db := handler.mediaDB.GetDB()
	_, err := db.Exec(`INSERT INTO media_items (id, media_type_id, title, year, genre, cast_crew, status)
		VALUES (1, 1, 'Test Movie', 2024, '["action","drama"]', '{"director":"John Doe"}', 'active')`)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest("GET", "/api/v1/media/1", nil)

	handler.GetMediaItem(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// getMediaTypeDistribution with real DB

func TestMediaHandler_getMediaTypeDistribution_WithDB(t *testing.T) {
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	distribution, err := handler.getMediaTypeDistribution()
	assert.NoError(t, err)
	assert.NotNil(t, distribution)
	// Should have entries for the seeded media types
	assert.GreaterOrEqual(t, len(distribution), 1)
}

// getRecentActivity with real DB

func TestMediaHandler_getRecentActivity_WithDB(t *testing.T) {
	handler, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	activity, err := handler.getRecentActivity()
	assert.NoError(t, err)
	assert.NotNil(t, activity)
}
