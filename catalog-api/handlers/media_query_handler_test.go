package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	mediaModels "catalogizer/internal/media/models"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// setupMediaQueryTestDB creates a temporary SQLite DB with full migrations
// so all real tables (media_items, media_types, files, media_files, favorites, users) exist.
func setupMediaQueryTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "media_query_handler_test_*.db")
	require.NoError(t, err)
	tmpFile.Close()

	cfg := &config.DatabaseConfig{
		Type:               "sqlite",
		Path:               tmpFile.Name(),
		MaxOpenConnections: 1,
		MaxIdleConnections: 1,
		ConnMaxLifetime:    3600,
		EnableWAL:          true,
		CacheSize:          2000,
		BusyTimeout:        5000,
	}

	db, err := database.NewConnection(cfg)
	require.NoError(t, err)

	err = db.RunMigrations(context.Background())
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}
	return db, cleanup
}

func setupMediaQueryHandler(t *testing.T, db *database.DB) (*MediaQueryHandler, *repository.MediaItemRepository, *repository.UserRepository) {
	t.Helper()
	itemRepo := repository.NewMediaItemRepository(db)
	userRepo := repository.NewUserRepository(db)
	handler := NewMediaQueryHandler(db, itemRepo, userRepo)
	return handler, itemRepo, userRepo
}

func TestGetRecentMedia(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, itemRepo, _ := setupMediaQueryHandler(t, db)
	ctx := context.Background()

	// Get the movie type ID from seeded data
	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	tests := []struct {
		name           string
		setup          func()
		query          string
		expectedStatus int
		checkItems     func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "empty database returns empty items",
			setup:          func() {},
			query:          "/api/v1/media/recent",
			expectedStatus: http.StatusOK,
			checkItems: func(t *testing.T, body map[string]interface{}) {
				items := body["items"].([]interface{})
				assert.Empty(t, items)
				assert.Equal(t, float64(0), body["total"])
			},
		},
		{
			name: "returns items ordered by last_updated desc",
			setup: func() {
				_, _ = itemRepo.Create(ctx, &mediaModels.MediaItem{
					MediaTypeID: typeID, Title: "Old Movie", Status: "detected",
				})
				_, _ = itemRepo.Create(ctx, &mediaModels.MediaItem{
					MediaTypeID: typeID, Title: "New Movie", Status: "detected",
				})
			},
			query:          "/api/v1/media/recent",
			expectedStatus: http.StatusOK,
			checkItems: func(t *testing.T, body map[string]interface{}) {
				items := body["items"].([]interface{})
				assert.GreaterOrEqual(t, len(items), 2)
				// Most recent should be first
				first := items[0].(map[string]interface{})
				assert.Equal(t, "New Movie", first["title"])
			},
		},
		{
			name:           "respects limit parameter",
			setup:          func() {},
			query:          "/api/v1/media/recent?limit=1",
			expectedStatus: http.StatusOK,
			checkItems: func(t *testing.T, body map[string]interface{}) {
				items := body["items"].([]interface{})
				assert.Equal(t, 1, len(items))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", tt.query, nil)

			handler.GetRecentMedia(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var body map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &body)
			require.NoError(t, err)
			tt.checkItems(t, body)
		})
	}
}

func TestGetPopularMedia(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, itemRepo, _ := setupMediaQueryHandler(t, db)
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	tests := []struct {
		name           string
		setup          func()
		expectedStatus int
		checkItems     func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "empty database returns empty items",
			setup:          func() {},
			expectedStatus: http.StatusOK,
			checkItems: func(t *testing.T, body map[string]interface{}) {
				items := body["items"].([]interface{})
				assert.Empty(t, items)
			},
		},
		{
			name: "returns items with favorite count",
			setup: func() {
				id1, _ := itemRepo.Create(ctx, &mediaModels.MediaItem{
					MediaTypeID: typeID, Title: "Popular Movie", Status: "detected",
				})
				id2, _ := itemRepo.Create(ctx, &mediaModels.MediaItem{
					MediaTypeID: typeID, Title: "Less Popular", Status: "detected",
				})

				// Add favorites for Popular Movie (more favorites = higher rank)
				db.ExecContext(ctx, `INSERT INTO favorites (user_id, entity_type, entity_id) VALUES (?, ?, ?)`, 1, "media_item", id1)
				db.ExecContext(ctx, `INSERT INTO favorites (user_id, entity_type, entity_id) VALUES (?, ?, ?)`, 2, "media_item", id1)
				// One favorite for Less Popular
				db.ExecContext(ctx, `INSERT INTO favorites (user_id, entity_type, entity_id) VALUES (?, ?, ?)`, 1, "media_item", id2)
			},
			expectedStatus: http.StatusOK,
			checkItems: func(t *testing.T, body map[string]interface{}) {
				items := body["items"].([]interface{})
				require.GreaterOrEqual(t, len(items), 2)
				// Popular Movie should come first (2 favorites > 1)
				first := items[0].(map[string]interface{})
				assert.Equal(t, "Popular Movie", first["title"])
				assert.Equal(t, float64(2), first["favorite_count"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/api/v1/media/popular", nil)

			handler.GetPopularMedia(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var body map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &body)
			require.NoError(t, err)
			tt.checkItems(t, body)
		})
	}
}

func TestGetMediaByPath(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, itemRepo, _ := setupMediaQueryHandler(t, db)
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	tests := []struct {
		name           string
		setup          func()
		query          string
		expectedStatus int
		checkBody      func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "missing path returns 400",
			setup:          func() {},
			query:          "/api/v1/media/by-path",
			expectedStatus: http.StatusBadRequest,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "path parameter is required", body["error"])
			},
		},
		{
			name: "returns items matching path prefix",
			setup: func() {
				// Create a storage root first
				db.ExecContext(ctx, `INSERT INTO storage_roots (name, protocol, path) VALUES (?, ?, ?)`,
					"test-root", "local", "/media")

				// Get the storage root ID
				var rootID int64
				db.QueryRowContext(ctx, `SELECT id FROM storage_roots WHERE name = ?`, "test-root").Scan(&rootID)

				// Create a file
				db.ExecContext(ctx, `INSERT INTO files (storage_root_id, path, name, size, modified_at) VALUES (?, ?, ?, ?, datetime('now'))`,
					rootID, "/media/movies/test.mkv", "test.mkv", 1000)

				var fileID int64
				db.QueryRowContext(ctx, `SELECT id FROM files WHERE path = ?`, "/media/movies/test.mkv").Scan(&fileID)

				// Create a media item and link it
				itemID, _ := itemRepo.Create(ctx, &mediaModels.MediaItem{
					MediaTypeID: typeID, Title: "Path Movie", Status: "detected",
				})

				db.ExecContext(ctx, `INSERT INTO media_files (media_item_id, file_id) VALUES (?, ?)`, itemID, fileID)
			},
			query:          "/api/v1/media/by-path?path=/media/movies",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				items := body["items"].([]interface{})
				assert.Equal(t, 1, len(items))
				first := items[0].(map[string]interface{})
				assert.Equal(t, "Path Movie", first["title"])
			},
		},
		{
			name:           "non-matching path returns empty",
			setup:          func() {},
			query:          "/api/v1/media/by-path?path=/nonexistent",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				items := body["items"].([]interface{})
				assert.Empty(t, items)
				assert.Equal(t, float64(0), body["total"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", tt.query, nil)

			handler.GetMediaByPath(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var body map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &body)
			require.NoError(t, err)
			tt.checkBody(t, body)
		})
	}
}

func TestRefreshMediaMetadata(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, itemRepo, _ := setupMediaQueryHandler(t, db)
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	itemID, _ := itemRepo.Create(ctx, &mediaModels.MediaItem{
		MediaTypeID: typeID, Title: "Refreshable Movie", Status: "detected",
	})

	// R4-7: pin a known value and snapshot BEFORE any subtest calls the handler, so a handler
	// that writes any constant (not only a time.Now()) is caught.
	_, err = db.Exec(`UPDATE media_items SET last_updated = ? WHERE id = ?`, "2001-02-03 04:05:06", itemID)
	require.NoError(t, err)
	var before string
	require.NoError(t, db.QueryRow(`SELECT CAST(last_updated AS TEXT) FROM media_items WHERE id = ?`, itemID).Scan(&before))

	tests := []struct {
		name           string
		paramID        string
		expectedStatus int
	}{
		{
			name:           "valid ID returns 501 (no refresher exists; CAT-001)",
			paramID:        fmt.Sprintf("%d", itemID),
			expectedStatus: http.StatusNotImplemented,
		},
		{
			name:           "non-existent ID returns 404",
			paramID:        "99999",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid ID returns 400",
			paramID:        "abc",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/v1/media/"+tt.paramID+"/refresh", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.paramID}}

			handler.RefreshMediaMetadata(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusNotImplemented {
				assert.NotContains(t, w.Body.String(), "queued\"")
				assert.NotContains(t, w.Body.String(), "accepted")
			}
		})
	}

	// The 501 must not touch the row: no fake "refresh" side effect.
	var after string
	require.NoError(t, db.QueryRow(`SELECT CAST(last_updated AS TEXT) FROM media_items WHERE id = ?`, itemID).Scan(&after))
	assert.Equal(t, before, after, "a 501 refresh must not modify the media row")
	assert.Contains(t, after, "2001-02-03")
}

func TestGetMediaQuality(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, itemRepo, _ := setupMediaQueryHandler(t, db)
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	// Create a media item with associated files
	itemID, _ := itemRepo.Create(ctx, &mediaModels.MediaItem{
		MediaTypeID: typeID, Title: "Quality Movie", Status: "detected",
	})

	// Also create a media item with no files
	emptyItemID, _ := itemRepo.Create(ctx, &mediaModels.MediaItem{
		MediaTypeID: typeID, Title: "Empty Movie", Status: "detected",
	})

	// Insert a storage root and files
	db.ExecContext(ctx, `INSERT INTO storage_roots (name, protocol, path) VALUES (?, ?, ?)`,
		"quality-root", "local", "/quality")
	var rootID int64
	db.QueryRowContext(ctx, `SELECT id FROM storage_roots WHERE name = ?`, "quality-root").Scan(&rootID)

	db.ExecContext(ctx, `INSERT INTO files (storage_root_id, path, name, size, mime_type, modified_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		rootID, "/quality/movie.mkv", "movie.mkv", 5000, "video/x-matroska")
	db.ExecContext(ctx, `INSERT INTO files (storage_root_id, path, name, size, mime_type, modified_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		rootID, "/quality/movie.srt", "movie.srt", 200, "text/plain")

	var fileID1, fileID2 int64
	db.QueryRowContext(ctx, `SELECT id FROM files WHERE path = ?`, "/quality/movie.mkv").Scan(&fileID1)
	db.QueryRowContext(ctx, `SELECT id FROM files WHERE path = ?`, "/quality/movie.srt").Scan(&fileID2)

	db.ExecContext(ctx, `INSERT INTO media_files (media_item_id, file_id) VALUES (?, ?)`, itemID, fileID1)
	db.ExecContext(ctx, `INSERT INTO media_files (media_item_id, file_id) VALUES (?, ?)`, itemID, fileID2)

	tests := []struct {
		name           string
		paramID        string
		expectedStatus int
		checkBody      func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "returns quality with files and total size",
			paramID:        fmt.Sprintf("%d", itemID),
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				quality := body["quality"].(map[string]interface{})
				files := quality["files"].([]interface{})
				assert.Equal(t, 2, len(files))
				assert.Equal(t, float64(5200), quality["total_size"])
			},
		},
		{
			name:           "returns empty files array when no files",
			paramID:        fmt.Sprintf("%d", emptyItemID),
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				quality := body["quality"].(map[string]interface{})
				files := quality["files"].([]interface{})
				assert.Empty(t, files)
				assert.Equal(t, float64(0), quality["total_size"])
			},
		},
		{
			name:           "non-existent ID returns 404",
			paramID:        "99999",
			expectedStatus: http.StatusNotFound,
			checkBody:      func(t *testing.T, body map[string]interface{}) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/api/v1/media/"+tt.paramID+"/quality", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.paramID}}

			handler.GetMediaQuality(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var body map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &body)
			require.NoError(t, err)
			tt.checkBody(t, body)
		})
	}
}

func TestAnalyzeMedia(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, itemRepo, _ := setupMediaQueryHandler(t, db)
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	itemID, _ := itemRepo.Create(ctx, &mediaModels.MediaItem{
		MediaTypeID: typeID, Title: "Analyzable Movie", Status: "detected",
	})

	// Create a file linked to this item
	db.ExecContext(ctx, `INSERT INTO storage_roots (name, protocol, path) VALUES (?, ?, ?)`,
		"analyze-root", "local", "/analyze")
	var rootID int64
	db.QueryRowContext(ctx, `SELECT id FROM storage_roots WHERE name = ?`, "analyze-root").Scan(&rootID)

	db.ExecContext(ctx, `INSERT INTO files (storage_root_id, path, name, size, mime_type, modified_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		rootID, "/analyze/movie.mkv", "movie.mkv", 3000, "video/x-matroska")
	var fileID int64
	db.QueryRowContext(ctx, `SELECT id FROM files WHERE path = ?`, "/analyze/movie.mkv").Scan(&fileID)
	db.ExecContext(ctx, `INSERT INTO media_files (media_item_id, file_id) VALUES (?, ?)`, itemID, fileID)

	tests := []struct {
		name           string
		body           interface{}
		expectedStatus int
		checkBody      func(t *testing.T, body map[string]interface{})
	}{
		{
			name:           "analyze by ID",
			body:           map[string]interface{}{"id": itemID},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				analysis := body["analysis"].(map[string]interface{})
				assert.Equal(t, "movie", analysis["type"])
				files := analysis["files"].([]interface{})
				assert.Equal(t, 1, len(files))
			},
		},
		{
			name:           "analyze by path",
			body:           map[string]string{"path": "/analyze/movie.mkv"},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				analysis := body["analysis"].(map[string]interface{})
				files := analysis["files"].([]interface{})
				assert.Equal(t, 1, len(files))
			},
		},
		{
			name:           "missing path and id returns 400",
			body:           map[string]string{},
			expectedStatus: http.StatusBadRequest,
			checkBody:      func(t *testing.T, body map[string]interface{}) {},
		},
		{
			name:           "non-existent ID returns 404",
			body:           map[string]interface{}{"id": 99999},
			expectedStatus: http.StatusNotFound,
			checkBody:      func(t *testing.T, body map[string]interface{}) {},
		},
		{
			name:           "non-existent path returns 404",
			body:           map[string]string{"path": "/nonexistent/file.mkv"},
			expectedStatus: http.StatusNotFound,
			checkBody:      func(t *testing.T, body map[string]interface{}) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/v1/media/analyze", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.AnalyzeMedia(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var body map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &body)
			require.NoError(t, err)
			tt.checkBody(t, body)
		})
	}
}

func TestChangePassword(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, _, userRepo := setupMediaQueryHandler(t, db)

	// Create a test user with a known password
	salt := "testsalt1234567890abcdef01234567"
	password := "oldpassword123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password+salt), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID, err := userRepo.Create(&models.User{
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Salt:         salt,
		RoleID:       1,
		IsActive:     true,
	})
	require.NoError(t, err)

	tests := []struct {
		name           string
		userIDInCtx    interface{}
		body           map[string]string
		expectedStatus int
		checkBody      func(t *testing.T, body map[string]interface{})
	}{
		{
			name:        "valid password change",
			userIDInCtx: userID,
			body: map[string]string{
				"current_password": "oldpassword123",
				"new_password":     "newpassword123",
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "ok", body["status"])
			},
		},
		{
			name:        "wrong current password",
			userIDInCtx: userID,
			body: map[string]string{
				"current_password": "wrongpassword",
				"new_password":     "newpassword123",
			},
			expectedStatus: http.StatusUnauthorized,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "current password is incorrect", body["error"])
			},
		},
		{
			name:        "new password too short",
			userIDInCtx: userID,
			body: map[string]string{
				"current_password": "newpassword123",
				"new_password":     "short",
			},
			expectedStatus: http.StatusBadRequest,
			checkBody:      func(t *testing.T, body map[string]interface{}) {},
		},
		{
			name:           "missing user_id in context",
			userIDInCtx:    nil,
			body:           map[string]string{},
			expectedStatus: http.StatusUnauthorized,
			checkBody:      func(t *testing.T, body map[string]interface{}) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/v1/auth/change-password", bytes.NewBuffer(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			if tt.userIDInCtx != nil {
				c.Set("user_id", tt.userIDInCtx)
			}

			handler.ChangePassword(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var body map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &body)
			tt.checkBody(t, body)
		})
	}
}

func TestGetInitStatus(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	handler, _, _ := setupMediaQueryHandler(t, db)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auth/init-status", nil)

	handler.GetInitStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, true, body["initialized"])
}
