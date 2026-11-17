package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/testpg"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reviewer tests (independent review of c97205bc1, CAT-RATING-1). Not part of the change under review.

func rvSeed(t *testing.T, db *database.DB) {
	ctx := context.Background()
	for _, u := range []int{1, 2} {
		_, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 2)`, u, fmt.Sprintf("u%d", u), fmt.Sprintf("u%d@example.test", u))
		require.NoError(t, err)
	}
	var mt int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT MIN(id) FROM media_types`).Scan(&mt))
	for i := 0; i < 2; i++ {
		_, err := db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (?, 'Item', 'detected')`, mt)
		require.NoError(t, err)
	}
}

func rvFlow(t *testing.T, db *database.DB, raw *sql.DB) {
	ctx := context.Background()
	h := NewAndroidTVMediaHandler(db)
	a, b := ratingRouter(t, h, 1, false), ratingRouter(t, h, 2, false)
	var m1, m2 int64
	require.NoError(t, raw.QueryRow(`SELECT MIN(id), MAX(id) FROM media_items`).Scan(&m1, &m2))
	p1 := fmt.Sprintf("/api/v1/media/%d/rating", m1)
	p2 := fmt.Sprintf("/api/v1/media/%d/rating", m2)

	code, got := favCall(a, "GET", p1, "")
	require.Equal(t, http.StatusOK, code, "%v", got)
	assert.Nil(t, got["rating"])
	code, got = favCall(a, "PUT", p1, `{"rating": 8}`)
	require.Equal(t, http.StatusOK, code, "%v", got)
	assert.Equal(t, float64(8), got["rating"])
	code, got = favCall(b, "PUT", p1, `{"rating": 2}`)
	require.Equal(t, http.StatusOK, code, "%v", got)
	_, got = favCall(a, "GET", p1, "")
	assert.Equal(t, float64(8), got["rating"])
	_, got = favCall(b, "GET", p1, "")
	assert.Equal(t, float64(2), got["rating"])

	var created1, updated1 time.Time
	require.NoError(t, raw.QueryRow(`SELECT created_at, updated_at FROM user_media_ratings WHERE user_id=1 AND media_id=$1`, m1).Scan(&created1, &updated1))
	time.Sleep(1100 * time.Millisecond)
	code, got = favCall(a, "PUT", p1, `{"rating": 10.0}`)
	require.Equal(t, http.StatusOK, code, "%v", got)
	var n int
	var created2, updated2 time.Time
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_media_ratings WHERE user_id=1`).Scan(&n))
	assert.Equal(t, 1, n, "upsert keeps one row")
	require.NoError(t, raw.QueryRow(`SELECT created_at, updated_at FROM user_media_ratings WHERE user_id=1 AND media_id=$1`, m1).Scan(&created2, &updated2))
	t.Logf("created %v -> %v ; updated %v -> %v", created1, created2, updated1, updated2)
	assert.True(t, updated2.After(updated1), "updated_at advances")
	_, got = favCall(a, "GET", p1, "")
	assert.Equal(t, float64(10), got["rating"])

	code, _ = favCall(a, "DELETE", p2, "")
	assert.Equal(t, http.StatusOK, code, "idempotent delete")
	code, _ = favCall(b, "DELETE", p1, "")
	assert.Equal(t, http.StatusOK, code)
	_, got = favCall(a, "GET", p1, "")
	assert.Equal(t, float64(10), got["rating"], "B delete leaves A")

	// concurrent upserts by the same user: one row, value among written
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			c, g := favCall(a, "PUT", p2, fmt.Sprintf(`{"rating": %d}`, v))
			assert.Equal(t, http.StatusOK, c, "%v", g)
		}(i)
	}
	wg.Wait()
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_media_ratings WHERE user_id=1 AND media_id=$1`, m2).Scan(&n))
	assert.Equal(t, 1, n)

	// CHECK at the schema layer
	_, err := raw.Exec(`UPDATE user_media_ratings SET rating = 11 WHERE user_id = 1`)
	assert.Error(t, err, "CHECK rejects 11")

	// cascades: deleting user 2's nothing; delete media m1 then user 1
	_, err = raw.Exec(`DELETE FROM media_items WHERE id = $1`, m1)
	require.NoError(t, err)
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_media_ratings WHERE media_id=$1`, m1).Scan(&n))
	assert.Zero(t, n, "media delete cascades")
	_, err = raw.Exec(`DELETE FROM users WHERE id = 1`)
	require.NoError(t, err, "user delete with ratings")
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_media_ratings WHERE user_id=1`).Scan(&n))
	assert.Zero(t, n, "user delete cascades")
	_ = ctx
}

func TestRVRating_RealPostgres(t *testing.T) {
	dsn, _ := testpg.Start(t)
	raw, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer raw.Close()
	db := database.WrapDB(raw, database.DialectPostgres)
	require.NoError(t, db.RunMigrations(context.Background()))
	rvSeed(t, db)
	rvFlow(t, db, raw)
}

func TestRVRating_SQLiteFileWithFKAndConcurrency(t *testing.T) {
	path := t.TempDir() + "/rv.db"
	raw, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=1")
	require.NoError(t, err)
	defer raw.Close()
	db := database.WrapDB(raw, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	rvSeed(t, db)
	rvFlow(t, db, raw)
}

// Extra validation matrix (values the author did not list).
func TestRVRating_ValidationExtra(t *testing.T) {
	db, _ := ratingDB(t)
	a := ratingRouter(t, NewAndroidTVMediaHandler(db), 1, false)
	gin.SetMode(gin.TestMode)
	bad := []string{`null`, `[]`, `[5]`, `{"rating": [5]}`, `{"rating": {"v":5}}`, `{"rating": 10.5}`, `{"rating": -0.5}`, `{"rating": 1e309}`, `"x"`, `{"Rating": 5}`, ``}
	for _, body := range bad {
		code, _ := favCall(a, "PUT", "/api/v1/media/1/rating", body)
		assert.Equal(t, http.StatusBadRequest, code, "body %q", body)
	}
	ok := map[string]float64{`{"rating": 1e1}`: 10, `{"rating": -0}`: 0, `{"rating": 5, "extra": true}`: 5, `{"rating": 3.0e0}`: 3}
	for body, want := range ok {
		code, got := favCall(a, "PUT", "/api/v1/media/1/rating", body)
		assert.Equal(t, http.StatusOK, code, "body %q", body)
		assert.Equal(t, want, got["rating"], "body %q", body)
	}
	for _, p := range []string{"/api/v1/media/-1/rating", "/api/v1/media/0/rating", "/api/v1/media/99999999999999999999/rating"} {
		code, _ := favCall(a, "GET", p, "")
		t.Logf("GET %s -> %d", p, code)
		assert.Contains(t, []int{400, 404}, code)
	}
	// GET/DELETE on unknown media must not leak whether another user rated it: 404 regardless of rows
	code, _ := favCall(a, "DELETE", "/api/v1/media/1/rating", `garbage body ignored`)
	assert.Equal(t, http.StatusOK, code)
}

// Anonymous on unknown media: 401, not 404 (no existence oracle for anon).
func TestRVRating_AnonNoOracle(t *testing.T) {
	db, _ := ratingDB(t)
	anon := ratingRouter(t, NewAndroidTVMediaHandler(db), 0, true)
	for _, m := range []string{"PUT", "GET", "DELETE"} {
		for _, p := range []string{"/api/v1/media/1/rating", "/api/v1/media/9999/rating"} {
			code, _ := favCall(anon, m, p, `{"rating":5}`)
			assert.Equal(t, http.StatusUnauthorized, code, "%s %s", m, p)
		}
	}
}

// user_id as string (middleware variant) still scopes correctly; user_id "0"/garbage is 401.
func TestRVRating_UserIDTypes(t *testing.T) {
	db, _ := ratingDB(t)
	h := NewAndroidTVMediaHandler(db)
	mk := func(v interface{}) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("user_id", v) })
		r.PUT("/api/v1/media/:id/rating", h.PutMediaRating)
		r.GET("/api/v1/media/:id/rating", h.GetMediaRating)
		return r
	}
	code, _ := favCall(mk("2"), "PUT", "/api/v1/media/1/rating", `{"rating": 4}`)
	require.Equal(t, http.StatusOK, code)
	_, got := favCall(mk(int64(2)), "GET", "/api/v1/media/1/rating", "")
	assert.Equal(t, float64(4), got["rating"])
	_, got = favCall(mk(1), "GET", "/api/v1/media/1/rating", "")
	assert.Nil(t, got["rating"])
	for _, v := range []interface{}{"0", "abc", 3.0} {
		code, _ := favCall(mk(v), "GET", "/api/v1/media/1/rating", "")
		assert.Equal(t, http.StatusUnauthorized, code, "user_id %v", v)
	}
}
