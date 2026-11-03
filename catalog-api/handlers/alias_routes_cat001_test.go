package handlers

// CAT-001 review round 1, F2/F7: the alias routes answer from real database state or 501, never canned data. The tests drive
// a real gin router with the real JWT middleware and the production-migrated database.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAliasRouter(t *testing.T) (*gin.Engine, *apiHarness) {
	t.Helper()
	h := newAPIHarness(t, false)
	jwtmw := middleware.NewJWTMiddleware(accessSecret)
	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(jwtmw.RequireAuth())
	NewAliasHandler(h.db, nil).RegisterAliasRoutes(api)
	return r, h
}

func aliasGet(r *gin.Engine, tok, path string) (int, map[string]interface{}) {
	req := httptest.NewRequest("GET", "/api/v1"+path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, decodeBody(w.Body.Bytes())
}

func TestCAT001_F2_StatsMediaTypesCountsRealRows(t *testing.T) {
	r, h := newAliasRouter(t)
	tok := cat001Token(t, 2, "5")
	var movieID int
	require.NoError(t, h.db.QueryRow(`SELECT id FROM media_types WHERE name = 'movie'`).Scan(&movieID))
	for _, title := range []string{"A", "B", "C"} {
		_, err := h.db.Exec(`INSERT INTO media_items (media_type_id, title) VALUES (?, ?)`, movieID, title)
		require.NoError(t, err)
	}
	code, body := aliasGet(r, tok, "/stats/media-types")
	require.Equal(t, 200, code)
	assert.EqualValues(t, 3, body["total"])
	assert.EqualValues(t, 3, body["distribution"].(map[string]interface{})["movie"])

	code, body = aliasGet(r, tok, "/media-types")
	require.Equal(t, 200, code)
	assert.NotEmpty(t, body["media_types"])
}

func TestCAT001_F2_SyncStatusAndProvidersComeFromTheCallersRows(t *testing.T) {
	r, h := newAliasRouter(t)
	_, err := h.db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (5, 'u5', 'u5@x', 'h', 's', 2, 1)`)
	require.NoError(t, err)
	tok := cat001Token(t, 2, "5")

	// nothing configured: truthfully empty, derived from the empty tables
	code, body := aliasGet(r, tok, "/sync/providers")
	require.Equal(t, 200, code)
	assert.Empty(t, body["providers"], "s3/gcs must not be advertised when no endpoint is configured")
	code, body = aliasGet(r, tok, "/sync/status")
	require.Equal(t, 200, code)
	assert.Equal(t, "idle", body["status"])

	_, err = h.db.Exec(`INSERT INTO sync_endpoints (id, user_id, name, type, url) VALUES (1, 5, 'e', 'webdav', 'https://x')`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO sync_sessions (endpoint_id, user_id, status) VALUES (1, 5, 'running')`)
	require.NoError(t, err)

	_, body = aliasGet(r, tok, "/sync/providers")
	provs := body["providers"].([]interface{})
	require.Len(t, provs, 1)
	assert.Equal(t, "webdav", provs[0].(map[string]interface{})["id"])
	_, body = aliasGet(r, tok, "/sync/status")
	assert.Equal(t, "running", body["status"])
	assert.EqualValues(t, 1, body["active_sessions"])

	// another user sees none of it
	_, body = aliasGet(r, cat001Token(t, 2, "6"), "/sync/status")
	assert.Equal(t, "idle", body["status"])
}

func TestCAT001_F2_ScanHistoryAndSubtitlesComeFromRows(t *testing.T) {
	r, h := newAliasRouter(t)
	tok := cat001Token(t, 2, "5")
	_, err := h.db.Exec(`INSERT INTO storage_roots (id, name, protocol, path, enabled) VALUES (1, 'r', 'local', '/x', 1)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO scan_history (storage_root_id, scan_type, status, start_time, files_processed) VALUES (1, 'full', 'completed', CURRENT_TIMESTAMP, 42)`)
	require.NoError(t, err)
	code, body := aliasGet(r, tok, "/stats/scan-history")
	require.Equal(t, 200, code)
	assert.EqualValues(t, 1, body["count"])
	assert.EqualValues(t, 42, body["history"].([]interface{})[0].(map[string]interface{})["files_processed"])

	code, body = aliasGet(r, tok, "/subtitles")
	require.Equal(t, 200, code)
	assert.EqualValues(t, 0, body["count"])
}

func TestCAT001_F2_RoutesWithoutARealSourceAre501(t *testing.T) {
	r, _ := newAliasRouter(t)
	tok := cat001Token(t, 2, "5")
	for _, p := range []string{"/translations", "/localization/stats", "/localization/translations", "/recommendations/by-type", "/sync/devices", "/sync/conflicts", "/files"} {
		code, _ := aliasGet(r, tok, p)
		assert.Equal(t, http.StatusNotImplemented, code, p)
	}
}

func TestCAT001_F2_AliasRoutesRequireAuth(t *testing.T) {
	r, _ := newAliasRouter(t)
	req := httptest.NewRequest("GET", "/api/v1/stats/media-types", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 401, w.Code)
}

// F7 (reviewer mutation RM6): dropping RequireAdmin from POST /scans must be caught.
func TestCAT001_F7_QueueScanIsAdminOnly(t *testing.T) {
	h := newAPIHarness(t, false)
	jwtmw := middleware.NewJWTMiddleware(accessSecret)
	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(jwtmw.RequireAuth())
	RegisterScanRoutes(api.Group("/scans"), h.handle, jwtmw)
	post := func(tok string) int {
		req := httptest.NewRequest("POST", "/api/v1/scans", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	assert.Equal(t, http.StatusForbidden, post(cat001Token(t, 2, "2")))
	assert.NotEqual(t, http.StatusForbidden, post(cat001Token(t, 1, "1")), "an admin passes the gate (then fails body validation)")
	// listing stays open to normal users
	req := httptest.NewRequest("GET", "/api/v1/scans", nil)
	req.Header.Set("Authorization", "Bearer "+cat001Token(t, 2, "2"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

func decodeBody(b []byte) map[string]interface{} {
	var out map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out
}
