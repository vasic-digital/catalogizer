package handlers

// Contract tests for the endpoints behind the catalog-web pages that crashed
// against a real backend (responsive audit 2026-11-03). They run the REAL
// handlers over a REAL migrated SQLite database through a gin router (no
// mocks, no stubs).

import (
	"context"
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
