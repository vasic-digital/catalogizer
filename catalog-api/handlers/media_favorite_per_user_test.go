package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/database"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-HAUTHZ-F2: the favorite flag of PUT /media/:id/favorite and GET /media/:id is per authenticated user. Real migrated
// SQLite schema, real handler, no stand-ins; the only injected thing is the user id the auth middleware would set.
func favRouter(t *testing.T, h *AndroidTVMediaHandler, uid int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", uid) })
	r.GET("/api/v1/media/:id", h.GetMediaByID)
	r.PUT("/api/v1/media/:id/favorite", h.UpdateFavoriteStatus)
	return r
}

func favCall(r *gin.Engine, method, path, body string) (int, map[string]interface{}) {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestFavoriteIsPerUser(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	ctx := context.Background()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(ctx))

	for _, u := range []int{1, 2, 7} {
		_, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 2)`, u, fmt.Sprintf("u%d", u), fmt.Sprintf("u%d@example.test", u))
		require.NoError(t, err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item', 'detected')`)
	require.NoError(t, err)
	// legacy global flag set: must be ignored (decision recorded in migration v30)
	_, err = db.ExecContext(ctx, `UPDATE media_items SET is_favorite = 1 WHERE id = 1`)
	require.NoError(t, err)

	h := NewAndroidTVMediaHandler(db)
	a, b := favRouter(t, h, 1), favRouter(t, h, 2)

	_, got := favCall(a, "GET", "/api/v1/media/1", "")
	assert.Equal(t, false, got["is_favorite"], "legacy global flag must not leak to anyone")

	code, _ := favCall(b, "PUT", "/api/v1/media/1/favorite", `{"favorite": true}`)
	require.Equal(t, http.StatusOK, code)
	_, got = favCall(b, "GET", "/api/v1/media/1", "")
	assert.Equal(t, true, got["is_favorite"], "B sees its own favorite")
	_, got = favCall(a, "GET", "/api/v1/media/1", "")
	assert.Equal(t, false, got["is_favorite"], "A must not see B's favorite")

	// A un-favoriting (never favorited) must not clear B's; A favoriting then removing leaves B's intact
	code, _ = favCall(a, "PUT", "/api/v1/media/1/favorite", `{"favorite": false}`)
	require.Equal(t, http.StatusOK, code)
	_, got = favCall(b, "GET", "/api/v1/media/1", "")
	assert.Equal(t, true, got["is_favorite"], "A's toggle-off must not change B's state")

	// idempotent favorite twice
	for i := 0; i < 2; i++ {
		code, _ = favCall(b, "PUT", "/api/v1/media/1/favorite", `{"favorite": true}`)
		require.Equal(t, http.StatusOK, code)
	}
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_favorites`).Scan(&n))
	assert.Equal(t, 1, n)

	// no authenticated user: write is refused
	anon := favRouter(t, h, 0)
	code, _ = favCall(anon, "PUT", "/api/v1/media/1/favorite", `{"favorite": true}`)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func favSetup(t *testing.T, items int) (*database.DB, *AndroidTVMediaHandler) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	ctx := context.Background()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(ctx))
	_, err = db.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (1, 'u1', 'u1@example.test', 'x', 'x', 2)`)
	require.NoError(t, err)
	for i := 0; i < items; i++ {
		_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, ?, 'detected')`, fmt.Sprintf("Item%d", i+1))
		require.NoError(t, err)
	}
	return db, NewAndroidTVMediaHandler(db)
}

// Favoriting item 1 must not make item 2 read as favorite for the same caller (the GET EXISTS must join on media_id).
func TestFavoriteIsPerItem(t *testing.T) {
	_, h := favSetup(t, 2)
	r := favRouter(t, h, 1)
	code, _ := favCall(r, "PUT", "/api/v1/media/1/favorite", `{"favorite": true}`)
	require.Equal(t, http.StatusOK, code)
	_, got := favCall(r, "GET", "/api/v1/media/1", "")
	assert.Equal(t, true, got["is_favorite"])
	_, got = favCall(r, "GET", "/api/v1/media/2", "")
	assert.Equal(t, false, got["is_favorite"], "item 2 was never favorited by the caller")
}

// Deleting a media item removes its favorite rows (FK cascade), for every user.
func TestFavoriteRowsRemovedWithMediaItem(t *testing.T) {
	db, h := favSetup(t, 2)
	ctx := context.Background()
	r := favRouter(t, h, 1)
	for _, id := range []string{"1", "2"} {
		code, _ := favCall(r, "PUT", "/api/v1/media/"+id+"/favorite", `{"favorite": true}`)
		require.Equal(t, http.StatusOK, code)
	}
	_, err := db.ExecContext(ctx, `DELETE FROM media_items WHERE id = 1`)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_favorites WHERE media_id = 1`).Scan(&n))
	assert.Zero(t, n, "favorite rows of the deleted item must be gone")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_favorites WHERE media_id = 2`).Scan(&n))
	assert.Equal(t, 1, n, "other items' favorites stay")
}

// Anonymous caller gets 401 even when the body is invalid (auth is checked before parsing).
func TestFavoriteAnonymousIs401BeforeBodyParsing(t *testing.T) {
	_, h := favSetup(t, 1)
	anon := favRouter(t, h, 0)
	code, _ := favCall(anon, "PUT", "/api/v1/media/1/favorite", `not json`)
	assert.Equal(t, http.StatusUnauthorized, code)
}
