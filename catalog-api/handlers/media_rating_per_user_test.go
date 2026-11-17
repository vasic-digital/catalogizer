package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"catalogizer/database"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-RATING-1: per-user 0..10 integer rating, PUT/GET/DELETE /api/v1/media/:id/rating. Real migrated SQLite schema, real
// handlers, no stand-ins; the only injected thing is the user id the auth middleware would set (as in favRouter).
func ratingRouter(t *testing.T, h *AndroidTVMediaHandler, uid int, anon bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if !anon {
		r.Use(func(c *gin.Context) { c.Set("user_id", uid) })
	}
	r.PUT("/api/v1/media/:id/rating", h.PutMediaRating)
	r.GET("/api/v1/media/:id/rating", h.GetMediaRating)
	r.DELETE("/api/v1/media/:id/rating", h.DeleteMediaRating)
	return r
}

func ratingDB(t *testing.T) (*database.DB, *sql.DB) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	ctx := context.Background()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(ctx))
	for _, u := range []int{1, 2} {
		_, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 2)`, u, fmt.Sprintf("u%d", u), fmt.Sprintf("u%d@example.test", u))
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item', 'detected')`)
		require.NoError(t, err)
	}
	return db, sqlDB
}

func TestRatingIsPerUser_HorizontalAuthz(t *testing.T) {
	db, _ := ratingDB(t)
	h := NewAndroidTVMediaHandler(db)
	a, b := ratingRouter(t, h, 1, false), ratingRouter(t, h, 2, false)

	code, got := favCall(a, "GET", "/api/v1/media/1/rating", "")
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, got["rating"], "unrated: rating is null")

	code, _ = favCall(a, "PUT", "/api/v1/media/1/rating", `{"rating": 8}`)
	require.Equal(t, http.StatusOK, code)

	_, got = favCall(a, "GET", "/api/v1/media/1/rating", "")
	assert.Equal(t, float64(8), got["rating"])
	// B must not see A's rating
	_, got = favCall(b, "GET", "/api/v1/media/1/rating", "")
	assert.Nil(t, got["rating"], "B must not see A's rating")

	// B writes its own; A's stays
	code, _ = favCall(b, "PUT", "/api/v1/media/1/rating", `{"rating": 3}`)
	require.Equal(t, http.StatusOK, code)
	_, got = favCall(a, "GET", "/api/v1/media/1/rating", "")
	assert.Equal(t, float64(8), got["rating"], "B's write must not change A's rating")
	_, got = favCall(b, "GET", "/api/v1/media/1/rating", "")
	assert.Equal(t, float64(3), got["rating"])

	// B deletes: A's survives
	code, _ = favCall(b, "DELETE", "/api/v1/media/1/rating", "")
	require.Equal(t, http.StatusOK, code)
	_, got = favCall(a, "GET", "/api/v1/media/1/rating", "")
	assert.Equal(t, float64(8), got["rating"], "B's delete must not remove A's rating")
	_, got = favCall(b, "GET", "/api/v1/media/1/rating", "")
	assert.Nil(t, got["rating"])

	// per item: rating of item 1 does not appear on item 2
	_, got = favCall(a, "GET", "/api/v1/media/2/rating", "")
	assert.Nil(t, got["rating"])

	// update (upsert) keeps a single row
	_, _ = favCall(a, "PUT", "/api/v1/media/1/rating", `{"rating": 0}`)
	_, got = favCall(a, "GET", "/api/v1/media/1/rating", "")
	assert.Equal(t, float64(0), got["rating"], "0 is a valid rating, distinct from unrated")
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM user_media_ratings WHERE user_id = 1 AND media_id = 1`).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestRatingValidation(t *testing.T) {
	db, _ := ratingDB(t)
	h := NewAndroidTVMediaHandler(db)
	a := ratingRouter(t, h, 1, false)

	for _, body := range []string{`{"rating": 11}`, `{"rating": -1}`, `{"rating": 7.5}`, `{"rating": "5"}`, `{"rating": null}`, `{}`, `{"rating": 1e3}`, `not json`, `{"rating": true}`} {
		code, _ := favCall(a, "PUT", "/api/v1/media/1/rating", body)
		assert.Equal(t, http.StatusBadRequest, code, "body %s", body)
	}
	for _, body := range []string{`{"rating": 0}`, `{"rating": 10}`, `{"rating": 7.0}`} {
		code, _ := favCall(a, "PUT", "/api/v1/media/1/rating", body)
		assert.Equal(t, http.StatusOK, code, "body %s", body)
	}
	// a rejected write stored nothing new: last accepted value was 7
	_, got := favCall(a, "GET", "/api/v1/media/1/rating", "")
	assert.Equal(t, float64(7), got["rating"])

	for _, m := range []string{"PUT", "GET", "DELETE"} {
		code, _ := favCall(a, m, "/api/v1/media/abc/rating", `{"rating": 5}`)
		assert.Equal(t, http.StatusBadRequest, code, "%s non-numeric id", m)
		code, _ = favCall(a, m, "/api/v1/media/9999/rating", `{"rating": 5}`)
		assert.Equal(t, http.StatusNotFound, code, "%s unknown media", m)
	}
}

func TestRatingRequiresAuthBeforeParsing(t *testing.T) {
	db, _ := ratingDB(t)
	h := NewAndroidTVMediaHandler(db)
	anon := ratingRouter(t, h, 0, true)
	for _, m := range []string{"PUT", "GET", "DELETE"} {
		code, _ := favCall(anon, m, "/api/v1/media/abc/rating", `not json`)
		assert.Equal(t, http.StatusUnauthorized, code, m)
	}
}

func TestRatingCascadesOnMediaDelete(t *testing.T) {
	db, raw := ratingDB(t)
	_, err := raw.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)
	h := NewAndroidTVMediaHandler(db)
	a := ratingRouter(t, h, 1, false)
	_, _ = favCall(a, "PUT", "/api/v1/media/1/rating", `{"rating": 5}`)
	_, err = raw.Exec(`DELETE FROM media_items WHERE id = 1`)
	require.NoError(t, err)
	var n int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_media_ratings`).Scan(&n))
	assert.Zero(t, n)
}

// Order auth (401), id (400), body (400), existence (404): a malformed body on an unknown id is 400, like PUT .../favorite.
func TestRatingErrorOrder(t *testing.T) {
	db, _ := ratingDB(t)
	a := ratingRouter(t, NewAndroidTVMediaHandler(db), 1, false)
	code, _ := favCall(a, "PUT", "/api/v1/media/9999/rating", `not json`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = favCall(a, "PUT", "/api/v1/media/9999/rating", `{"rating": 5}`)
	assert.Equal(t, http.StatusNotFound, code)
}
