package handlers

// Contract tests for the endpoints behind the catalog-web pages that crashed
// against a real backend (responsive audit 2026-11-03). They run the REAL
// handlers over a REAL migrated SQLite database through a gin router (no
// mocks, no stubs).

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mediaModels "catalogizer/internal/media/models"
	"catalogizer/internal/services"
	"catalogizer/repository"
	rootServices "catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func getJSON(t *testing.T, r *gin.Engine, path string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body=%s", w.Body.String())
	return w.Code, body
}

// A user with no favorites must get "favorites": [] (never null): catalog-web
// iterates the payload and a null list is indistinguishable from a broken
// response.
func TestFavoritesList_EmptyIsEmptyArrayNotNull(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()

	favSvc := rootServices.NewFavoritesService(repository.NewFavoritesRepository(db), nil)
	h := NewFavoritesHandler(favSvc, zap.NewNop())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/favorites", func(c *gin.Context) { c.Set("user_id", 1); h.ListFavorites(c) })

	code, body := getJSON(t, r, "/api/v1/favorites")
	require.Equal(t, http.StatusOK, code)
	list, isArray := body["favorites"].([]interface{})
	assert.True(t, isArray, "favorites must be a JSON array, got %T (%v)", body["favorites"], body["favorites"])
	assert.Len(t, list, 0)
}

func newRecommendationRouter(t *testing.T) (*gin.Engine, *repository.MediaItemRepository, func()) {
	t.Helper()
	db, cleanup := setupMediaQueryTestDB(t)
	svc := services.NewRecommendationService(nil, nil, repository.NewFileRepository(db), db)
	h := NewRecommendationHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/recommendations/similar/:media_id", h.GetSimilarItems)
	return r, repository.NewMediaItemRepository(db), cleanup
}

// GET /recommendations/similar/:id answered 500 ("converting NULL to string is
// unsupported") for every item created by the real pipeline, because the
// handler selected legacy nullable/absent columns (media_type, producer,
// "cast", resolution, file_size) into non-nullable Go fields. An existing item
// must yield 200, an unknown id 404.
func TestRecommendationsSimilar_ExistingItemIs200(t *testing.T) {
	r, itemRepo, cleanup := newRecommendationRouter(t)
	defer cleanup()
	ctx := context.Background()

	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)
	id, err := itemRepo.Create(ctx, &mediaModels.MediaItem{MediaTypeID: typeID, Title: "Real Movie", Status: "detected"})
	require.NoError(t, err)

	code, body := getJSON(t, r, "/api/v1/recommendations/similar/"+itoa(id))
	assert.Equal(t, http.StatusOK, code, "body=%v", body)
	_, hasLocal := body["local_items"]
	assert.True(t, hasLocal, "response must carry local_items, got %v", body)
}

func TestRecommendationsSimilar_UnknownItemIs404(t *testing.T) {
	r, _, cleanup := newRecommendationRouter(t)
	defer cleanup()
	code, _ := getJSON(t, r, "/api/v1/recommendations/similar/987654")
	assert.Equal(t, http.StatusNotFound, code)
}

// getMediaMetadata must select exactly the requested row: with several items
// present the right one comes back, and an unknown id is sql.ErrNoRows even
// though the table is non-empty (a widened WHERE such as "OR mi.id > 0"
// would return an arbitrary row instead).
func TestGetMediaMetadata_SelectsRequestedRowOnly(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()
	ctx := context.Background()
	itemRepo := repository.NewMediaItemRepository(db)
	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)

	var ids []int64
	director := "Ridley Scott"
	for _, title := range []string{"First", "Second", "Third"} {
		id, err := itemRepo.Create(ctx, &mediaModels.MediaItem{MediaTypeID: typeID, Title: title, Status: "detected", Genre: []string{"Sci-Fi"}, Director: &director})
		require.NoError(t, err)
		ids = append(ids, id)
	}

	svc := services.NewRecommendationService(nil, nil, repository.NewFileRepository(db), db)
	h := NewRecommendationHandler(svc)

	for i, want := range []string{"First", "Second", "Third"} {
		md, err := h.getMediaMetadata(ctx, ids[i])
		require.NoError(t, err)
		assert.Equal(t, want, md.Title)
		assert.Equal(t, ids[i], md.ID)
		assert.Equal(t, "movie", md.MediaType)
		assert.Equal(t, "Ridley Scott", md.Director)
		assert.Contains(t, md.Genre, "Sci-Fi", "genre must come from the genre column, not director")
		assert.NotContains(t, md.Genre, "Ridley")
	}

	_, err = h.getMediaMetadata(ctx, 987654)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
