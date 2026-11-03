package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/internal/media/models"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CollectionHandlerTestSuite struct {
	suite.Suite
	handler *CollectionHandler
	router  *gin.Engine
}

func (suite *CollectionHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
}

func (suite *CollectionHandlerTestSuite) SetupTest() {
	suite.handler = NewCollectionHandler(nil)
	suite.router = gin.New()
	suite.router.GET("/api/v1/collections", suite.handler.ListCollections)
	suite.router.GET("/api/v1/collections/:id", suite.handler.GetCollection)
	suite.router.POST("/api/v1/collections", suite.handler.CreateCollection)
	suite.router.PUT("/api/v1/collections/:id", suite.handler.UpdateCollection)
	suite.router.DELETE("/api/v1/collections/:id", suite.handler.DeleteCollection)
}

// --- Constructor tests ---

func (suite *CollectionHandlerTestSuite) TestNewCollectionHandler_NilRepo() {
	handler := NewCollectionHandler(nil)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.repo)
}

func (suite *CollectionHandlerTestSuite) TestNewCollectionHandler_WithRepo() {
	repo := &repository.MediaCollectionRepository{}
	handler := NewCollectionHandler(repo)
	assert.NotNil(suite.T(), handler)
	assert.Equal(suite.T(), repo, handler.repo)
}

// --- GetCollection validation tests ---

func (suite *CollectionHandlerTestSuite) TestGetCollection_InvalidID_NotNumber() {
	req := httptest.NewRequest("GET", "/api/v1/collections/abc", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Contains(suite.T(), w.Body.String(), "Invalid collection ID")
}

func (suite *CollectionHandlerTestSuite) TestGetCollection_InvalidID_Decimal() {
	req := httptest.NewRequest("GET", "/api/v1/collections/1.5", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CollectionHandlerTestSuite) TestGetCollection_InvalidID_Special() {
	invalidIDs := []string{"!@#", "id-abc", "12e3", "--1"}
	for _, id := range invalidIDs {
		req := httptest.NewRequest("GET", "/api/v1/collections/"+id, nil)
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(suite.T(), http.StatusBadRequest, w.Code,
			"ID %s should be rejected", id)
	}
}

func (suite *CollectionHandlerTestSuite) TestGetCollection_OverflowID() {
	// ID that exceeds int64 max
	req := httptest.NewRequest("GET", "/api/v1/collections/99999999999999999999", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// --- CreateCollection validation tests ---

func (suite *CollectionHandlerTestSuite) TestCreateCollection_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/collections", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Contains(suite.T(), w.Body.String(), "Invalid request body")
}

func (suite *CollectionHandlerTestSuite) TestCreateCollection_MissingName() {
	body := `{"collection_type": "custom"}`
	req := httptest.NewRequest("POST", "/api/v1/collections", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// CAT-001 (R4-4/R4-9): is_smart / smart_rules were silently dropped and the collection was created
// as a plain one. The nil repo means any path that reaches repo.Create panics, so these tests
// also prove the request is refused before storage.
func (suite *CollectionHandlerTestSuite) TestCreateCollection_SmartIsRefusedNotSilentlyDropped() {
	for _, body := range []string{
		`{"name":"S","is_smart":true,"smart_rules":[{"field":"genre"}]}`,
		`{"name":"S","is_smart":true}`,
		`{"name":"S","smart_rules":[{"field":"genre"}]}`,
	} {
		req := httptest.NewRequest("POST", "/api/v1/collections", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		assert.NotPanics(suite.T(), func() { suite.router.ServeHTTP(w, req) }, body)
		assert.Equal(suite.T(), http.StatusNotImplemented, w.Code, body)
		assert.Contains(suite.T(), w.Body.String(), "Nothing was created")
	}
}

func (suite *CollectionHandlerTestSuite) TestUpdateCollection_SmartIsRefused() {
	req := httptest.NewRequest("PUT", "/api/v1/collections/1", bytes.NewBufferString(`{"is_smart":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	assert.NotPanics(suite.T(), func() { suite.router.ServeHTTP(w, req) })
	assert.Equal(suite.T(), http.StatusNotImplemented, w.Code)
}

func (suite *CollectionHandlerTestSuite) TestCreateCollection_EmptyBody() {
	req := httptest.NewRequest("POST", "/api/v1/collections", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// name is required, so empty body should fail
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CollectionHandlerTestSuite) TestCreateCollection_MethodNotAllowed() {
	// Test that DELETE is not allowed on the collection list endpoint
	req := httptest.NewRequest("DELETE", "/api/v1/collections", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *CollectionHandlerTestSuite) TestCreateCollection_PUTMethodNotAllowed() {
	// PUT without ID goes to collection list, which doesn't support PUT
	req := httptest.NewRequest("PUT", "/api/v1/collections", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// --- UpdateCollection validation tests ---

func (suite *CollectionHandlerTestSuite) TestUpdateCollection_InvalidID() {
	body := `{"name": "updated"}`
	req := httptest.NewRequest("PUT", "/api/v1/collections/abc", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Contains(suite.T(), w.Body.String(), "Invalid collection ID")
}

func (suite *CollectionHandlerTestSuite) TestUpdateCollection_InvalidJSON() {
	req := httptest.NewRequest("PUT", "/api/v1/collections/1", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CollectionHandlerTestSuite) TestUpdateCollection_DecimalID() {
	body := `{"name": "updated"}`
	req := httptest.NewRequest("PUT", "/api/v1/collections/1.5", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// --- DeleteCollection validation tests ---

func (suite *CollectionHandlerTestSuite) TestDeleteCollection_InvalidID() {
	req := httptest.NewRequest("DELETE", "/api/v1/collections/abc", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	assert.Contains(suite.T(), w.Body.String(), "Invalid collection ID")
}

func (suite *CollectionHandlerTestSuite) TestDeleteCollection_DecimalID() {
	req := httptest.NewRequest("DELETE", "/api/v1/collections/1.5", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CollectionHandlerTestSuite) TestDeleteCollection_SpecialCharID() {
	req := httptest.NewRequest("DELETE", "/api/v1/collections/!@#", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// --- collectionsToJSON tests ---

func (suite *CollectionHandlerTestSuite) TestCollectionsToJSON_Empty() {
	result := collectionsToJSON([]*models.MediaCollection{})
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result)
}

func (suite *CollectionHandlerTestSuite) TestCollectionsToJSON_Nil() {
	result := collectionsToJSON(nil)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result)
}

func (suite *CollectionHandlerTestSuite) TestCollectionsToJSON_SingleCollection() {
	desc := "Test description"
	coverURL := "https://example.com/cover.jpg"
	now := time.Now()

	collections := []*models.MediaCollection{
		{
			ID:             1,
			Name:           "Test Collection",
			CollectionType: "custom",
			Description:    &desc,
			TotalItems:     10,
			ExternalIDs:    map[string]string{"tmdb": "123"},
			CoverURL:       &coverURL,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	result := collectionsToJSON(collections)
	assert.Len(suite.T(), result, 1)

	item := result[0]
	assert.Equal(suite.T(), int64(1), item["id"])
	assert.Equal(suite.T(), "Test Collection", item["name"])
	assert.Equal(suite.T(), "custom", item["collection_type"])
	assert.Equal(suite.T(), &desc, item["description"])
	assert.Equal(suite.T(), 10, item["total_items"])
	assert.Equal(suite.T(), map[string]string{"tmdb": "123"}, item["external_ids"])
	assert.Equal(suite.T(), &coverURL, item["cover_url"])
	assert.Equal(suite.T(), now, item["created_at"])
	assert.Equal(suite.T(), now, item["updated_at"])
}

func (suite *CollectionHandlerTestSuite) TestCollectionsToJSON_MultipleCollections() {
	collections := []*models.MediaCollection{
		{ID: 1, Name: "First", CollectionType: "playlist"},
		{ID: 2, Name: "Second", CollectionType: "custom"},
		{ID: 3, Name: "Third", CollectionType: "smart"},
	}

	result := collectionsToJSON(collections)
	assert.Len(suite.T(), result, 3)

	assert.Equal(suite.T(), "First", result[0]["name"])
	assert.Equal(suite.T(), "Second", result[1]["name"])
	assert.Equal(suite.T(), "Third", result[2]["name"])
}

func (suite *CollectionHandlerTestSuite) TestCollectionsToJSON_NilOptionalFields() {
	collections := []*models.MediaCollection{
		{
			ID:             1,
			Name:           "Minimal",
			CollectionType: "custom",
			Description:    nil,
			ExternalIDs:    nil,
			CoverURL:       nil,
		},
	}

	result := collectionsToJSON(collections)
	assert.Len(suite.T(), result, 1)

	item := result[0]
	assert.Nil(suite.T(), item["description"])
	assert.Nil(suite.T(), item["external_ids"])
	assert.Nil(suite.T(), item["cover_url"])
}

func TestCollectionHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(CollectionHandlerTestSuite))
}
