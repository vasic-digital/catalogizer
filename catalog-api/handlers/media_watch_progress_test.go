package handlers

// WPROG: PUT /api/v1/media/:id/progress persists a per-user watch position in media_progress (the store the authenticated
// GET /entities/:id/progress reads). Host-fixture tests: real migrated in-memory SQLite and the real handler; the only
// injected thing is the user id the RequireAuth middleware would put in the context. Not a live-device result.

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
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wprogDB(t *testing.T) *database.DB {
	t.Helper()
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
	_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item', 'detected')`)
	require.NoError(t, err)
	return db
}

// wprogRouter: uid<0 means no authenticated user (route reached without RequireAuth).
func wprogRouter(h *AndroidTVMediaHandler, uid int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if uid >= 0 {
		r.Use(func(c *gin.Context) { c.Set("user_id", uid) })
	}
	r.PUT("/api/v1/media/:id/progress", h.UpdateWatchProgress)
	return r
}

func wprogPut(r *gin.Engine, id, body string) (int, map[string]interface{}) {
	req := httptest.NewRequest("PUT", "/api/v1/media/"+id+"/progress", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func fraction(p *repository.MediaProgress) float64 {
	return float64(p.LastPosition) / float64(*p.DurationTotal)
}

func TestWatchProgress_PersistsPerUser(t *testing.T) {
	db := wprogDB(t)
	repo := repository.NewPlaybackSessionRepository(db)
	h := NewAndroidTVMediaHandler(db)
	a, b := wprogRouter(h, 1), wprogRouter(h, 2)

	code, resp := wprogPut(a, "1", `{"progress": 0.25, "duration": 1000}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, resp["persisted"], "the write must really be stored")

	p, err := repo.GetProgress(context.Background(), 1, 1)
	require.NoError(t, err, "user 1 must have a media_progress row")
	assert.InDelta(t, 0.25, fraction(p), 0.001)

	_, err = repo.GetProgress(context.Background(), 2, 1)
	assert.ErrorIs(t, err, sql.ErrNoRows, "user 2 must not see user 1's progress")

	// B writes its own value; A's must be unchanged (upsert is keyed per user)
	code, _ = wprogPut(b, "1", `{"progress": 0.9, "duration": 1000}`)
	require.Equal(t, http.StatusOK, code)
	pa, _ := repo.GetProgress(context.Background(), 1, 1)
	pb, _ := repo.GetProgress(context.Background(), 2, 1)
	assert.InDelta(t, 0.25, fraction(pa), 0.001)
	assert.InDelta(t, 0.9, fraction(pb), 0.001)

	// upsert: second write by A replaces, exactly one row for (A,item)
	code, _ = wprogPut(a, "1", `{"progress": 0.5}`)
	require.Equal(t, http.StatusOK, code)
	pa, _ = repo.GetProgress(context.Background(), 1, 1)
	assert.InDelta(t, 0.5, fraction(pa), 0.001)
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM media_progress WHERE user_id = 1 AND media_item_id = 1`).Scan(&n))
	assert.Equal(t, 1, n)

	// boundaries 0 and 1 are valid
	for _, v := range []string{"0", "1", "0.0", "1.0"} {
		code, _ = wprogPut(a, "1", `{"progress": `+v+`}`)
		assert.Equal(t, http.StatusOK, code, v)
	}
}

// Body shapes accepted without a 400. Android TV sends {"progress": x} and the Android WatchProgressRequest adds a
// string timestamp and position, but neither calls this route today (Android calls the nonexistent user/progress/{id});
// they are accepted for when a client is pointed here, and without a duration they are answered persisted:false. The
// desktop / api-client PlaybackProgress shape (floats, no progress) is covered in media_watch_progress_integrity_test.go.
func TestWatchProgress_AcceptedBodyShapes(t *testing.T) {
	db := wprogDB(t)
	h := NewAndroidTVMediaHandler(db)
	r := wprogRouter(h, 1)
	cases := map[string]string{
		"progress only":                    `{"progress":0.42}`,
		"progress + string timestamp":      `{"progress":0.42,"timestamp":"1700000000000"}`,
		"progress + timestamp + position":  `{"progress":0.42,"timestamp":"1700000000000","position":3012}`,
		"desktop floats":                   `{"media_id":1,"position":12.5,"duration":7200.25,"timestamp":1700000000000}`,
		"progress + numeric ts + duration": `{"progress":0.42,"media_id":1,"duration":7200,"timestamp":1700000000000}`,
	}
	for name, body := range cases {
		code, resp := wprogPut(r, "1", body)
		assert.Equal(t, http.StatusOK, code, "%s: %v", name, resp)
	}
}

func TestWatchProgress_StatusCodes(t *testing.T) {
	db := wprogDB(t)
	h := NewAndroidTVMediaHandler(db)
	r := wprogRouter(h, 1)

	code, _ := wprogPut(wprogRouter(h, -1), "1", `{"progress": 0.5}`)
	assert.Equal(t, http.StatusUnauthorized, code, "no user in context -> 401 before anything is parsed")
	code, _ = wprogPut(wprogRouter(h, -1), "abc", `not json`)
	assert.Equal(t, http.StatusUnauthorized, code)

	code, _ = wprogPut(r, "999", `{"progress": 0.5}`)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = wprogPut(r, "abc", `{"progress": 0.5}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = wprogPut(r, "1", `not json`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = wprogPut(r, "1", `{}`)
	assert.Equal(t, http.StatusBadRequest, code, "progress required")
	code, _ = wprogPut(r, "1", `{"progress": null}`)
	assert.Equal(t, http.StatusBadRequest, code, "null progress is missing")
	code, _ = wprogPut(r, "1", `{"progress": "0.5"}`)
	assert.Equal(t, http.StatusBadRequest, code, "progress must be a number")
	code, _ = wprogPut(r, "1", `{"progress": 1.01}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = wprogPut(r, "1", `{"progress": -0.01}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = wprogPut(r, "1", `{"progress": 0.5, "position": -5}`)
	assert.Equal(t, http.StatusBadRequest, code, "negative position")

	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM media_progress`).Scan(&n))
	assert.Equal(t, 0, n, "no refused request may leave a row")
}

// A row created by the playback-session path (unit seconds, known duration) keeps its unit; the fraction is converted
// into that unit, and an explicit position (seconds) is honoured.
func TestWatchProgress_KeepsSecondsRow(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `INSERT INTO media_progress (user_id, media_item_id, position_unit, duration_total, last_position, updated_at) VALUES (1, 1, 'seconds', 7200, 100, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	h := NewAndroidTVMediaHandler(db)
	r := wprogRouter(h, 1)
	repo := repository.NewPlaybackSessionRepository(db)

	code, _ := wprogPut(r, "1", `{"progress": 0.5}`)
	require.Equal(t, http.StatusOK, code)
	p, _ := repo.GetProgress(ctx, 1, 1)
	assert.Equal(t, "seconds", p.PositionUnit)
	assert.Equal(t, int64(3600), p.LastPosition)
	assert.Equal(t, int64(7200), *p.DurationTotal)

	code, _ = wprogPut(r, "1", `{"progress": 0.5, "position": 4000}`)
	require.Equal(t, http.StatusOK, code)
	p, _ = repo.GetProgress(ctx, 1, 1)
	assert.Equal(t, int64(4000), p.LastPosition)

	code, _ = wprogPut(r, "1", `{"progress": 0.5, "position": 99999}`)
	require.Equal(t, http.StatusOK, code)
	p, _ = repo.GetProgress(ctx, 1, 1)
	assert.Equal(t, int64(7200), p.LastPosition, "position is clamped to the duration")
}
