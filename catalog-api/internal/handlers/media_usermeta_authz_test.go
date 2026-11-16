package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HAUTHZ-O1: the legacy MediaHandler user-metadata read is caller-scoped. User 2's row must never be returned for user 1
// (or for an anonymous caller), on the real encrypted SQLite media database.
func TestMediaHandler_UserMetadata_CallerScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := setupMediaHandlerWithDB(t)
	defer cleanup()

	db := h.mediaDB.GetDB()
	_, err := db.Exec(`INSERT INTO media_items (id, media_type_id, title, year, genre, cast_crew, status)
		VALUES (1, 1, 'T', 2024, '[]', '{}', 'active')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_metadata (media_item_id, user_id, personal_notes, tags, favorite)
		VALUES (1, 2, 'hzmark-b-note', '[]', 1)`)
	require.NoError(t, err)
	// a legacy/ownerless row (user_id defaults to 0) must not be handed to an anonymous caller either
	_, err = db.Exec(`INSERT INTO user_metadata (media_item_id, personal_notes, tags) VALUES (1, 'hzmark-ownerless-note', '[]')`)
	require.NoError(t, err)

	own, err := h.getUserMetadata(1, 2)
	require.NoError(t, err)
	require.NotNil(t, own, "owner reads its own row")
	require.NotNil(t, own.PersonalNotes)
	assert.Equal(t, "hzmark-b-note", *own.PersonalNotes)

	foreign, err := h.getUserMetadata(1, 1)
	require.NoError(t, err)
	assert.Nil(t, foreign, "another user's row must not be returned")

	anon, err := h.getUserMetadata(1, 0)
	require.NoError(t, err)
	assert.Nil(t, anon)

	// through the handler: caller 1 must not see user 2's note in the body
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest("GET", "/api/v1/media/1", nil)
	c.Set("user_id", int64(1))
	h.GetMediaItem(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "hzmark-b-note")
}
