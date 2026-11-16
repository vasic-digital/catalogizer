package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalogizer/repository"
)

// CAT-HAUTHZ-1 follow-ups (M-T1, M-T2, M-T3). These drive the REAL handlers on the REAL production-migrated database
// (SQLite, newRTEnv); the caller identity comes from the same context key the JWT middleware sets. No test doubles.

// hzUnitRouter wires the playback and user-metadata handlers; the acting user is the X-Test-User header.
func hzUnitRouter(e *rtEnv) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if v := c.GetHeader("X-Test-User"); v != "" {
			n, _ := strconv.ParseInt(v, 10, 64)
			c.Set("user_id", n)
		}
		c.Next()
	})
	ph := NewPlaybackHandler(repository.NewPlaybackSessionRepository(e.db))
	r.POST("/pb/start", ph.StartSession)
	r.POST("/pb/progress", ph.ProgressSession)
	r.POST("/pb/end", ph.EndSession)
	mh := NewMediaEntityHandler(nil, nil, nil, repository.NewUserMetadataRepository(e.db), e.db)
	r.PUT("/entities/:id/user-metadata", mh.UpdateUserMetadata)
	return r
}

func hzUnitDo(r http.Handler, user, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// M-T1: the user-metadata write takes the owner from the token, never from the body. Read back through the user_metadata
// store (the same repository the handler writes through): A's note lands on A's row; B's row is not created or changed.
func TestHorizontalAuthzUnit_UserMetadataOwnerFromTokenNotBody(t *testing.T) {
	e := newRTEnv(t)
	r := hzUnitRouter(e)
	repo := repository.NewUserMetadataRepository(e.db)
	const item = 7

	// B writes its own row first
	w := hzUnitDo(r, "2", "PUT", "/entities/7/user-metadata", `{"personal_notes":"hzmark-b-note","user_rating":9}`)
	require.Equal(t, 200, w.Code, w.Body.String())

	// A names B in the body (user_id) and tries to overwrite it
	w = hzUnitDo(r, "1", "PUT", "/entities/7/user-metadata", `{"user_id":2,"personal_notes":"hzmark-a-note","user_rating":1}`)
	require.Equal(t, 200, w.Code, w.Body.String())

	b, err := repo.GetByItemAndUser(t.Context(), item, 2)
	require.NoError(t, err)
	require.NotNil(t, b, "B's row exists")
	require.NotNil(t, b.PersonalNotes)
	assert.Equal(t, "hzmark-b-note", *b.PersonalNotes, "A's write naming B in the body must not change B's row")
	require.NotNil(t, b.UserRating)
	assert.Equal(t, 9.0, *b.UserRating)

	a, err := repo.GetByItemAndUser(t.Context(), item, 1)
	require.NoError(t, err)
	require.NotNil(t, a, "A's write lands on A's own row")
	require.NotNil(t, a.PersonalNotes)
	assert.Equal(t, "hzmark-a-note", *a.PersonalNotes)

	// unauthenticated: refused, nothing written for user 3
	w = hzUnitDo(r, "", "PUT", "/entities/8/user-metadata", `{"user_id":3,"personal_notes":"x"}`)
	assert.Equal(t, 401, w.Code, w.Body.String())
	none, err := repo.GetByItemAndUser(t.Context(), 8, 3)
	require.NoError(t, err)
	assert.Nil(t, none)
}

// M-T2: a foreign session and a nonexistent session answer with the same status AND the same body (no oracle for ids).
func TestHorizontalAuthzUnit_ForeignSessionIndistinguishableFromMissing(t *testing.T) {
	e := newRTEnv(t)
	r := hzUnitRouter(e)
	w := hzUnitDo(r, "2", "POST", "/pb/start", `{"media_item_id":1,"file_id":1,"position_unit":"seconds","start_position":0}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	sid := hzJSONField(t, w.Body.Bytes(), "session_id")

	for _, ep := range []string{"/pb/progress", "/pb/end"} {
		foreign := hzUnitDo(r, "1", "POST", ep, `{"session_id":`+sid+`,"end_position":5,"total_amount":9}`)
		missing := hzUnitDo(r, "1", "POST", ep, `{"session_id":987654321,"end_position":5,"total_amount":9}`)
		assert.Equal(t, 404, missing.Code, ep+" missing: "+missing.Body.String())
		assert.Equal(t, 404, foreign.Code, ep+" foreign must be 404, not 403: "+foreign.Body.String())
		assert.Equal(t, missing.Body.String(), foreign.Body.String(), ep+": foreign and missing bodies must be identical")
	}
	// the owner is still served (positive control) and B's row is unchanged by A's attempts
	w = hzUnitDo(r, "2", "POST", "/pb/progress", `{"session_id":`+sid+`,"end_position":5,"total_amount":9}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
}

// M-T3: fail closed. When the ownership lookup itself fails the answer is 500 and nothing is written; it must never fall
// through to the write. The lookup (repository.Get) is made to fail by an unreadable start_position cell, a column that
// Get scans but that the UPDATE and the in-transaction reload of repository.Progress/End never read. So with a fail-open
// ownsSession the write path would run to completion and answer 200 and persist end_position=777; only the ownership
// check can produce the 500 here. (An earlier version corrupted user_id, which End re-reads itself, so End answered 500
// regardless of ownsSession and the End leg did not discriminate.)
func TestHorizontalAuthzUnit_OwnershipLookupErrorFailsClosed(t *testing.T) {
	e := newRTEnv(t)
	r := hzUnitRouter(e)
	w := hzUnitDo(r, "2", "POST", "/pb/start", `{"media_item_id":1,"file_id":1,"position_unit":"seconds","start_position":0}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	sid := hzJSONField(t, w.Body.Bytes(), "session_id")
	_, err := e.db.Exec("UPDATE playback_sessions SET start_position = 'unreadable-cell' WHERE id = " + sid)
	require.NoError(t, err)

	for _, ep := range []string{"/pb/progress", "/pb/end"} {
		// the caller IS the owner (user 2): only the lookup failure can stop the write
		w := hzUnitDo(r, "2", "POST", ep, `{"session_id":`+sid+`,"end_position":777,"total_amount":777}`)
		assert.Equal(t, 500, w.Code, ep+": lookup error must be 500 (fail closed): "+w.Body.String())
		var end interface{}
		var ended interface{}
		require.NoError(t, e.db.QueryRow("SELECT end_position, ended_at FROM playback_sessions WHERE id = "+sid).Scan(&end, &ended))
		assert.NotEqual(t, int64(777), end, ep+": no write happened after the failed ownership lookup")
		assert.Nil(t, ended, ep+": session not ended after the failed ownership lookup")
	}
}

func hzJSONField(t *testing.T, raw []byte, key string) string {
	t.Helper()
	m := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(raw, &m), "%s", raw)
	v, ok := m[key].(float64)
	require.True(t, ok, "%s in %s", key, raw)
	return fmt.Sprintf("%d", int64(v))
}
