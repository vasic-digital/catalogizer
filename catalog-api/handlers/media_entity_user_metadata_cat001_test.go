package handlers

// CAT-001: UpdateUserMetadata attributed every write to user 1 ("default to 1 for now"): the JWT middleware sets user_id as an int (or a
// string), the handler only accepted int64, so the real user id was dropped and another account's data was overwritten. Real SQLite
// database with the production migrations, real repositories.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"catalogizer/database"
	"catalogizer/internal/media/models"
	"catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedUsers(t *testing.T, db *database.DB, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		_, err := db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 1)`,
			id, "u"+strconv.FormatInt(id, 10), "u"+strconv.FormatInt(id, 10)+"@example.test")
		require.NoError(t, err)
	}
}

func putUserMetadata(t *testing.T, h *MediaEntityHandler, itemID int64, userID interface{}, set bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/entities/"+strconv.FormatInt(itemID, 10)+"/user-metadata", bytes.NewBufferString(`{"user_rating":7.5,"is_favorite":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(itemID, 10)}}
	if set {
		c.Set("user_id", userID)
	}
	h.UpdateUserMetadata(c)
	return w
}

func TestCAT001_UpdateUserMetadata_WritesForTheAuthenticatedUser(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()
	h, itemRepo := setupEntityHandler(t, db)
	ctx := context.Background()
	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)
	itemID, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "M", Status: "detected"})
	require.NoError(t, err)

	seedUsers(t, db, 1, 42, 43, 44)
	for _, tc := range []struct {
		name string
		uid  interface{}
		want int64
	}{{"int (JWT middleware)", int(42), 42}, {"int64", int64(43), 43}, {"string subject", "44", 44}} {
		w := putUserMetadata(t, h, itemID, tc.uid, true)
		require.Equal(t, http.StatusOK, w.Code, "%s: %s", tc.name, w.Body.String())
		um, err := h.userMetaRepo.GetByItemAndUser(ctx, itemID, tc.want)
		require.NoError(t, err, tc.name)
		require.NotNil(t, um, "%s: row for user %d must exist", tc.name, tc.want)
		assert.Equal(t, tc.want, um.UserID, tc.name)
	}
	// nothing was written for the old hard-coded user
	um, _ := h.userMetaRepo.GetByItemAndUser(ctx, itemID, 1)
	assert.Nil(t, um, "no row may be attributed to user 1")
}

func TestCAT001_UpdateUserMetadata_UnauthenticatedIs401NotUserOne(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()
	h, itemRepo := setupEntityHandler(t, db)
	ctx := context.Background()
	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)
	itemID, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "M", Status: "detected"})
	require.NoError(t, err)
	seedUsers(t, db, 1)
	w := putUserMetadata(t, h, itemID, nil, false)
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	um, _ := h.userMetaRepo.GetByItemAndUser(ctx, itemID, 1)
	assert.Nil(t, um)
}

// F8 (review round 1): the same contract through a real gin router with the real JWT middleware and a signed token, so the
// user id type that production actually puts in the context (int, from the token subject) is what the handler sees.
// On the base commit this fails: the handler only accepted int64 and wrote the row under user 1.
func TestCAT001_F8_UpdateUserMetadata_ThroughRealJWTRouter(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()
	h, itemRepo := setupEntityHandler(t, db)
	ctx := context.Background()
	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)
	itemID, err := itemRepo.Create(ctx, &models.MediaItem{MediaTypeID: typeID, Title: "M", Status: "detected"})
	require.NoError(t, err)
	seedUsers(t, db, 1, 42)

	gin.SetMode(gin.TestMode)
	jwtmw := middleware.NewJWTMiddleware(accessSecret)
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(jwtmw.RequireAuth())
	g.PUT("/entities/:id/user-metadata", h.UpdateUserMetadata)

	do := func(tok string) int {
		req := httptest.NewRequest("PUT", "/api/v1/entities/"+strconv.FormatInt(itemID, 10)+"/user-metadata", bytes.NewBufferString(`{"user_rating":7.5,"is_favorite":true}`))
		req.Header.Set("Content-Type", "application/json")
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	assert.Equal(t, http.StatusUnauthorized, do(""))
	require.Equal(t, http.StatusOK, do(cat001Token(t, 2, "42")))
	um, err := h.userMetaRepo.GetByItemAndUser(ctx, itemID, 42)
	require.NoError(t, err)
	require.NotNil(t, um, "the row must be written for the token's user")
	other, _ := h.userMetaRepo.GetByItemAndUser(ctx, itemID, 1)
	assert.Nil(t, other, "nothing may be attributed to user 1")
}
