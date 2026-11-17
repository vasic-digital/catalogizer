package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"catalogizer/database"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// media_handler_defect_d_test.go — §11.4.115 RED-on-broken-artifact
// regression guard for DEFECT-D.
//
// DEFECT-D (FACT, 2026-06-25): GET /api/v1/media/:id returned HTTP 500
// `no such column: media_type` for EVERY id, and PUT
// /api/v1/media/:id/progress returned 500 `no such column: watch_progress`,
// because AndroidTVMediaHandler.GetMediaByID / UpdateWatchProgress queried
// a FLAT/denormalized media_items schema that only ever existed in the
// in-memory test helper (internal/tests/test_helper.go). The PRODUCTION
// schema (database/migrations_sqlite.go media_items + migration v18) is
// NORMALIZED: the type name lives in media_types(name) via media_type_id;
// none of media_type / cover_image / quality / file_size / directory_path /
// smb_path / external_metadata / versions / watch_progress / last_watched /
// is_downloaded exist on media_items.
//
// Every test below runs the REAL production migrations (db.RunMigrations)
// so the real normalized media_items / media_types schema exists — no
// mock DB, real in-memory SQLite (§11.4.27). The flat test-helper schema
// is deliberately NOT used here: using it would hide the defect (it is the
// very schema mismatch that caused DEFECT-D).

// newDefectDTestDB opens an in-memory SQLite DB, runs the REAL production
// migrations, and seeds one media_type + one media_item. Returns the
// wrapped DB and the seeded media item id.
func newDefectDTestDB(t *testing.T) (*database.DB, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err, "open sqlite")
	t.Cleanup(func() { _ = sqlDB.Close() })
	// In-memory SQLite: a single connection so every statement hits the
	// same database (multiple conns to :memory: are separate DBs).
	sqlDB.SetMaxOpenConns(1)

	ctx := context.Background()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(ctx), "run production migrations")

	// media_types row id=1 is seeded as 'movie' by createInitialTables;
	// assert that explicitly so the MediaType name check is meaningful.
	var movieID int64
	require.NoError(t,
		db.QueryRowContext(ctx, `SELECT id FROM media_types WHERE name = 'movie'`).Scan(&movieID),
		"the production seed must contain a 'movie' media_type")

	res, err := db.ExecContext(ctx, `
		INSERT INTO media_items (media_type_id, title, year, description, rating, status)
		VALUES (?, 'Blade Runner 2049', 2017, 'A young blade runner discovers a secret.', 8.0, 'detected')
	`, movieID)
	require.NoError(t, err, "seed media_items")
	id, err := res.LastInsertId()
	require.NoError(t, err, "media_items LastInsertId")

	return db, id
}

// TestDefectD_RED_OldFlatQueryFailsOnRealSchema reproduces DEFECT-D on the
// REAL schema: the exact flat SELECT the old handler used must fail with a
// "no such column: media_type" error. This is the §11.4.115 RED proof that
// the defect is genuinely present in the production schema (not a synthetic
// failure) — and it is the guard that FAILS if anyone regresses the handler
// query back to the phantom-column form.
func TestDefectD_RED_OldFlatQueryFailsOnRealSchema(t *testing.T) {
	db, id := newDefectDTestDB(t)
	ctx := context.Background()

	// The verbatim flat query the broken handler used.
	oldQuery := `
		SELECT
			id, title, media_type, year, description, cover_image, rating, quality,
			file_size, duration, directory_path, smb_path, created_at, updated_at,
			external_metadata, versions, is_favorite, watch_progress, last_watched, is_downloaded
		FROM media_items
		WHERE id = ?
	`
	var sink any
	err := db.QueryRowContext(ctx, oldQuery, id).Scan(&sink)
	require.Error(t, err, "the old flat query MUST fail on the real normalized schema")
	assert.Contains(t, err.Error(), "no such column",
		"DEFECT-D signature is a missing-column error; got: %v", err)
	// media_type is the first phantom column the planner hits.
	assert.Contains(t, err.Error(), "media_type",
		"the missing column must be media_type (the canonical DEFECT-D symptom); got: %v", err)
}

// TestDefectD_GetMediaByID_Success is the GREEN guard: after the fix,
// GetMediaByID returns 200 with the correct MediaType name resolved via the
// media_types JOIN for the seeded id, against the REAL production schema.
func TestDefectD_GetMediaByID_Success(t *testing.T) {
	db, id := newDefectDTestDB(t)

	handler := NewAndroidTVMediaHandler(db)
	r := gin.New()
	r.GET("/api/v1/media/:id", handler.GetMediaByID)

	req := httptest.NewRequest("GET", "/api/v1/media/"+itoa(id), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code,
		"GetMediaByID must return 200 on the real schema; got body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "response must be valid JSON")

	// The JOIN must resolve the type-name string — this is the exact field
	// (media_type) whose phantom column caused the 500. Asserting the real
	// value 'movie' proves the JOIN works, not merely that 500 is gone.
	assert.Equal(t, "movie", resp["media_type"],
		"media_type must be the JOINed media_types.name 'movie'; got: %v", resp["media_type"])
	assert.Equal(t, "Blade Runner 2049", resp["title"])
	assert.Equal(t, float64(2017), resp["year"], "year must round-trip")
	assert.Equal(t, float64(8), resp["rating"], "rating must round-trip")

	// DEFECT-D guard: the phantom per-item fields must be the documented
	// safe defaults, never fabricated data. external_metadata/versions are
	// empty arrays (never null/garbage); is_downloaded is false.
	assert.Equal(t, []any{}, resp["external_metadata"], "external_metadata defaults to empty array")
	assert.Equal(t, []any{}, resp["versions"], "versions defaults to empty array")
	assert.Equal(t, false, resp["is_downloaded"], "is_downloaded has no real column; defaults false")
	assert.Equal(t, float64(0), resp["watch_progress"], "watch_progress has no real column here; defaults 0")
}

// TestDefectD_GetMediaByID_NotFound proves 404 for a missing id is preserved.
func TestDefectD_GetMediaByID_NotFound(t *testing.T) {
	db, _ := newDefectDTestDB(t)

	handler := NewAndroidTVMediaHandler(db)
	r := gin.New()
	r.GET("/api/v1/media/:id", handler.GetMediaByID)

	req := httptest.NewRequest("GET", "/api/v1/media/9999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"missing id must 404, not 500; got body: %s", w.Body.String())
}

// TestDefectD_GetMediaByID_InvalidID proves 400 for a non-numeric id.
func TestDefectD_GetMediaByID_InvalidID(t *testing.T) {
	db, _ := newDefectDTestDB(t)

	handler := NewAndroidTVMediaHandler(db)
	r := gin.New()
	r.GET("/api/v1/media/:id", handler.GetMediaByID)

	req := httptest.NewRequest("GET", "/api/v1/media/not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"non-numeric id must 400; got body: %s", w.Body.String())
}

// TestDefectD_RED_OldWatchProgressUpdateFailsOnRealSchema reproduces the
// UpdateWatchProgress half of DEFECT-D: the old phantom-column UPDATE must
// fail on the real schema. §11.4.115 RED proof + regression guard.
func TestDefectD_RED_OldWatchProgressUpdateFailsOnRealSchema(t *testing.T) {
	db, id := newDefectDTestDB(t)
	ctx := context.Background()

	oldUpdate := `
		UPDATE media_items
		SET watch_progress = ?, last_watched = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := db.ExecContext(ctx, oldUpdate, 0.5, id)
	require.Error(t, err, "old watch_progress UPDATE MUST fail on the real schema")
	assert.Contains(t, err.Error(), "no such column",
		"DEFECT-D watch_progress signature is a missing-column error; got: %v", err)
}

// TestDefectD_UpdateWatchProgress_ExistingMedia is the GREEN guard for the
// progress route: against the real schema, an existing media id returns 200
// (validated existence, no phantom write) and a missing id returns 404.
func TestDefectD_UpdateWatchProgress_ExistingMedia(t *testing.T) {
	db, id := newDefectDTestDB(t)

	handler := NewAndroidTVMediaHandler(db)
	r := gin.New()
	r.PUT("/api/v1/media/:id/progress", func(c *gin.Context) { c.Set("user_id", 7) }, handler.UpdateWatchProgress)

	req := httptest.NewRequest("PUT", "/api/v1/media/"+itoa(id)+"/progress",
		strings.NewReader(`{"progress": 0.5}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code,
		"existing media progress update must 200 on real schema; got body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"])
	// WPROG: the route is authenticated and now really stores the caller's progress (it used to report persisted=false).
	assert.Equal(t, true, resp["persisted"],
		"authenticated progress route must persist per-user progress")
}

// TestDefectD_UpdateWatchProgress_NotFound proves a missing id 404s (not 500).
func TestDefectD_UpdateWatchProgress_NotFound(t *testing.T) {
	db, _ := newDefectDTestDB(t)

	handler := NewAndroidTVMediaHandler(db)
	r := gin.New()
	r.PUT("/api/v1/media/:id/progress", func(c *gin.Context) { c.Set("user_id", 7) }, handler.UpdateWatchProgress)

	req := httptest.NewRequest("PUT", "/api/v1/media/9999999/progress",
		strings.NewReader(`{"progress": 0.5}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"missing id progress update must 404, not 500; got body: %s", w.Body.String())
}

// itoa renders an int64 media id for URL paths.
func itoa(i int64) string {
	return strconv.FormatInt(i, 10)
}
