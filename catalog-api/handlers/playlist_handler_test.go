package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/database"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func setupPlaylistTestDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT,
			salt TEXT,
			role_id INTEGER NOT NULL DEFAULT 1,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT INTO users (id, username, email, password_hash, salt) VALUES (1, 'testuser', 'test@example.com', '', '')`,
		`INSERT INTO users (id, username, email, password_hash, salt) VALUES (2, 'otheruser', 'other@example.com', '', '')`,
		`CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			is_public INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			playlist_id INTEGER NOT NULL,
			entity_id INTEGER NOT NULL,
			entity_type TEXT NOT NULL,
			position INTEGER DEFAULT 0,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE
		)`,
	}

	for _, s := range stmts {
		_, err := sqlDB.Exec(s)
		require.NoError(t, err)
	}

	return database.WrapDB(sqlDB, database.DialectSQLite)
}

func newPlaylistHandler(t *testing.T) (*PlaylistHandler, *database.DB) {
	t.Helper()
	db := setupPlaylistTestDB(t)
	repo := repository.NewPlaylistRepository(db)
	svc := services.NewPlaylistService(repo)
	logger, _ := zap.NewDevelopment()
	h := NewPlaylistHandler(svc, logger)
	return h, db
}

func playlistRouter(handler *PlaylistHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/playlists", handler.ListPlaylists)
	r.POST("/playlists", handler.CreatePlaylist)
	r.GET("/playlists/:id", handler.GetPlaylist)
	r.PUT("/playlists/:id", handler.UpdatePlaylist)
	r.DELETE("/playlists/:id", handler.DeletePlaylist)
	r.POST("/playlists/:id/items", handler.AddItem)
	r.DELETE("/playlists/:id/items/:item_id", handler.RemoveItem)
	return r
}

// playlistAuthMiddleware sets user_id in gin context.
func playlistAuthMiddleware(uid int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", uid)
		c.Next()
	}
}

func playlistRouterWithAuth(handler *PlaylistHandler, userID int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	auth := playlistAuthMiddleware(userID)
	r.GET("/playlists", auth, handler.ListPlaylists)
	r.POST("/playlists", auth, handler.CreatePlaylist)
	r.GET("/playlists/:id", auth, handler.GetPlaylist)
	r.PUT("/playlists/:id", auth, handler.UpdatePlaylist)
	r.DELETE("/playlists/:id", auth, handler.DeletePlaylist)
	r.POST("/playlists/:id/items", auth, handler.AddItem)
	r.DELETE("/playlists/:id/items/:item_id", auth, handler.RemoveItem)
	return r
}

// intToStr converts an int to its string representation for URL building.
func intToStr(i int) string {
	return fmt.Sprintf("%d", i)
}

// ---------------------------------------------------------------------------
// NewPlaylistHandler
// ---------------------------------------------------------------------------

func TestNewPlaylistHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	h := NewPlaylistHandler(nil, logger)
	assert.NotNil(t, h)
	assert.Equal(t, logger, h.logger)
}

// ---------------------------------------------------------------------------
// ListPlaylists
// ---------------------------------------------------------------------------

func TestListPlaylists_Unauthorized(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/playlists", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListPlaylists_InvalidUserIDType(t *testing.T) {
	handler, _ := newPlaylistHandler(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/playlists", func(c *gin.Context) {
		c.Set("user_id", "not-an-int")
		c.Next()
	}, handler.ListPlaylists)

	req := httptest.NewRequest(http.MethodGet, "/playlists", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListPlaylists_Empty(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodGet, "/playlists", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["count"])
}

func TestListPlaylists_WithLimitOffset(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	// Create a playlist first
	body, _ := json.Marshal(map[string]string{"name": "Test Playlist"})
	createReq := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	createReq.Header.Set("Content-Type", "application/json")
	cw := httptest.NewRecorder()
	router.ServeHTTP(cw, createReq)
	require.Equal(t, http.StatusCreated, cw.Code)

	// List with explicit limit/offset
	req := httptest.NewRequest(http.MethodGet, "/playlists?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["count"])
	assert.Equal(t, float64(10), resp["limit"])
	assert.Equal(t, float64(0), resp["offset"])
}

func TestListPlaylists_InvalidLimitResets(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodGet, "/playlists?limit=-1&offset=-5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(50), resp["limit"])
	assert.Equal(t, float64(0), resp["offset"])
}

// ---------------------------------------------------------------------------
// CreatePlaylist
// ---------------------------------------------------------------------------

func TestCreatePlaylist_Unauthorized(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouter(handler)

	body, _ := json.Marshal(map[string]string{"name": "My Playlist"})
	req := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreatePlaylist_InvalidJSON(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePlaylist_Success(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	body, _ := json.Marshal(map[string]string{
		"name":        "My Playlist",
		"description": "Test description",
	})
	req := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "My Playlist", resp["name"])
	assert.NotZero(t, resp["id"])
}

// CAT-001 R5: is_smart / smart_rules used to be dropped by the request model and a plain empty playlist was
// created and returned 201. A smart playlist is refused honestly and nothing is stored.
func TestCreatePlaylist_SmartIsRefusedNotSilentlyDropped(t *testing.T) {
	handler, db := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)
	for _, raw := range []string{
		`{"name":"S","is_smart":true}`,
		`{"name":"S","smart_rules":[{"field":"genre","operator":"equals","value":"rock"}]}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBufferString(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotImplemented, w.Code, raw)
		assert.Contains(t, w.Body.String(), "Nothing was created")
	}
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM playlists WHERE name = 'S'`).Scan(&n))
	assert.Equal(t, 0, n, "no playlist may be stored for a refused smart request")
}

// ---------------------------------------------------------------------------
// GetPlaylist
// ---------------------------------------------------------------------------

func TestGetPlaylist_Unauthorized(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/playlists/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetPlaylist_InvalidID(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodGet, "/playlists/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPlaylist_NotFound(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodGet, "/playlists/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPlaylist_Success(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	// Create first
	body, _ := json.Marshal(map[string]string{"name": "Readable"})
	cReq := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	cReq.Header.Set("Content-Type", "application/json")
	cw := httptest.NewRecorder()
	router.ServeHTTP(cw, cReq)
	require.Equal(t, http.StatusCreated, cw.Code)

	var created map[string]interface{}
	json.Unmarshal(cw.Body.Bytes(), &created)
	id := int(created["id"].(float64))

	req := httptest.NewRequest(http.MethodGet, "/playlists/"+intToStr(id), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Readable", resp["name"])
}

// ---------------------------------------------------------------------------
// UpdatePlaylist
// ---------------------------------------------------------------------------

func TestUpdatePlaylist_Unauthorized(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouter(handler)

	body, _ := json.Marshal(map[string]string{"name": "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/playlists/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdatePlaylist_InvalidID(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	body, _ := json.Marshal(map[string]string{"name": "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/playlists/abc", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePlaylist_InvalidJSON(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodPut, "/playlists/1", bytes.NewBufferString("bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePlaylist_Success(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	// Create
	body, _ := json.Marshal(map[string]string{"name": "Original"})
	cReq := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	cReq.Header.Set("Content-Type", "application/json")
	cw := httptest.NewRecorder()
	router.ServeHTTP(cw, cReq)
	require.Equal(t, http.StatusCreated, cw.Code)

	var created map[string]interface{}
	json.Unmarshal(cw.Body.Bytes(), &created)
	id := int(created["id"].(float64))

	// Update
	updatedName := "Updated Name"
	updateBody, _ := json.Marshal(map[string]*string{"name": &updatedName})
	uReq := httptest.NewRequest(http.MethodPut, "/playlists/"+intToStr(id), bytes.NewBuffer(updateBody))
	uReq.Header.Set("Content-Type", "application/json")
	uw := httptest.NewRecorder()
	router.ServeHTTP(uw, uReq)

	assert.Equal(t, http.StatusOK, uw.Code)
}

// ---------------------------------------------------------------------------
// DeletePlaylist
// ---------------------------------------------------------------------------

func TestDeletePlaylist_Unauthorized(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/playlists/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeletePlaylist_InvalidID(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodDelete, "/playlists/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeletePlaylist_Success(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	// Create
	body, _ := json.Marshal(map[string]string{"name": "Deletable"})
	cReq := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	cReq.Header.Set("Content-Type", "application/json")
	cw := httptest.NewRecorder()
	router.ServeHTTP(cw, cReq)
	require.Equal(t, http.StatusCreated, cw.Code)

	var created map[string]interface{}
	json.Unmarshal(cw.Body.Bytes(), &created)
	id := int(created["id"].(float64))

	req := httptest.NewRequest(http.MethodDelete, "/playlists/"+intToStr(id), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// AddItem
// ---------------------------------------------------------------------------

func TestAddItem_Unauthorized(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouter(handler)

	body, _ := json.Marshal(map[string]interface{}{"entity_id": 10, "entity_type": "movie"})
	req := httptest.NewRequest(http.MethodPost, "/playlists/1/items", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAddItem_InvalidPlaylistID(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	body, _ := json.Marshal(map[string]interface{}{"entity_id": 10, "entity_type": "movie"})
	req := httptest.NewRequest(http.MethodPost, "/playlists/abc/items", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddItem_InvalidJSON(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodPost, "/playlists/1/items", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddItem_Success(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	// Create playlist
	body, _ := json.Marshal(map[string]string{"name": "With Items"})
	cReq := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	cReq.Header.Set("Content-Type", "application/json")
	cw := httptest.NewRecorder()
	router.ServeHTTP(cw, cReq)
	require.Equal(t, http.StatusCreated, cw.Code)

	var created map[string]interface{}
	json.Unmarshal(cw.Body.Bytes(), &created)
	id := int(created["id"].(float64))

	// Add item
	itemBody, _ := json.Marshal(map[string]interface{}{"entity_id": 42, "entity_type": "movie"})
	iReq := httptest.NewRequest(http.MethodPost, "/playlists/"+intToStr(id)+"/items", bytes.NewBuffer(itemBody))
	iReq.Header.Set("Content-Type", "application/json")
	iw := httptest.NewRecorder()
	router.ServeHTTP(iw, iReq)

	assert.Equal(t, http.StatusCreated, iw.Code)
}

// ---------------------------------------------------------------------------
// RemoveItem
// ---------------------------------------------------------------------------

func TestRemoveItem_Unauthorized(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/playlists/1/items/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRemoveItem_InvalidPlaylistID(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodDelete, "/playlists/abc/items/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveItem_InvalidItemID(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	req := httptest.NewRequest(http.MethodDelete, "/playlists/1/items/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveItem_Success(t *testing.T) {
	handler, _ := newPlaylistHandler(t)
	router := playlistRouterWithAuth(handler, 1)

	// Create playlist
	body, _ := json.Marshal(map[string]string{"name": "With Removable Items"})
	cReq := httptest.NewRequest(http.MethodPost, "/playlists", bytes.NewBuffer(body))
	cReq.Header.Set("Content-Type", "application/json")
	cw := httptest.NewRecorder()
	router.ServeHTTP(cw, cReq)
	require.Equal(t, http.StatusCreated, cw.Code)

	var created map[string]interface{}
	json.Unmarshal(cw.Body.Bytes(), &created)
	playlistID := int(created["id"].(float64))

	// Add item
	itemBody, _ := json.Marshal(map[string]interface{}{"entity_id": 10, "entity_type": "song"})
	iReq := httptest.NewRequest(http.MethodPost, "/playlists/"+intToStr(playlistID)+"/items", bytes.NewBuffer(itemBody))
	iReq.Header.Set("Content-Type", "application/json")
	iw := httptest.NewRecorder()
	router.ServeHTTP(iw, iReq)
	require.Equal(t, http.StatusCreated, iw.Code)

	var item map[string]interface{}
	json.Unmarshal(iw.Body.Bytes(), &item)
	itemID := int(item["id"].(float64))

	// Remove item
	req := httptest.NewRequest(http.MethodDelete, "/playlists/"+intToStr(playlistID)+"/items/"+intToStr(itemID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
