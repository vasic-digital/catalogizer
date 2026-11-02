package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type RecommendationHandlerTestSuite struct {
	suite.Suite
	handler *RecommendationHandler
	router  *gin.Engine
}

func (suite *RecommendationHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
}

func (suite *RecommendationHandlerTestSuite) SetupTest() {
	// Initialize handler with nil service to test validation paths
	suite.handler = NewRecommendationHandler(nil)

	suite.router = gin.New()
	suite.router.GET("/api/v1/recommendations/similar/:media_id", suite.handler.GetSimilarItems)
	suite.router.GET("/api/v1/recommendations/trending", suite.handler.GetTrendingItems)
	suite.router.GET("/api/v1/recommendations/personalized/:user_id", suite.handler.GetPersonalizedRecommendations)
}

// Constructor tests

func (suite *RecommendationHandlerTestSuite) TestNewRecommendationHandler() {
	handler := NewRecommendationHandler(nil)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.recommendationService)
}

// GetSimilarItems tests

func (suite *RecommendationHandlerTestSuite) TestGetSimilarItems_InvalidMediaID() {
	req := httptest.NewRequest("GET", "/api/v1/recommendations/similar/abc", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"], "Invalid media ID")
}

func (suite *RecommendationHandlerTestSuite) TestGetSimilarItems_FloatMediaID() {
	req := httptest.NewRequest("GET", "/api/v1/recommendations/similar/1.5", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *RecommendationHandlerTestSuite) TestGetSimilarItems_EmptyStringID() {
	// This gets captured by the router as 404 since the param is required
	req := httptest.NewRequest("GET", "/api/v1/recommendations/similar/", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code == http.StatusMovedPermanently || w.Code == http.StatusNotFound)
}

func (suite *RecommendationHandlerTestSuite) TestGetSimilarItems_MethodNotAllowed() {
	req := httptest.NewRequest("POST", "/api/v1/recommendations/similar/1", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// GetTrendingItems tests

func (suite *RecommendationHandlerTestSuite) TestGetTrendingItems_NotImplementedNeverFabricates() {
	for _, q := range []string{"", "?media_type=movie", "?time_range=month", "?limit=3"} {
		req := httptest.NewRequest("GET", "/api/v1/recommendations/trending"+q, nil)
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(suite.T(), http.StatusNotImplemented, w.Code, q)
		assert.NotContains(suite.T(), w.Body.String(), `"items"`, q)
		assert.NotContains(suite.T(), w.Body.String(), "/trending/path", q)
	}
}

// GetPersonalizedRecommendations tests

func (suite *RecommendationHandlerTestSuite) personalizedRouter(callerID interface{}, role int) *gin.Engine {
	r := gin.New()
	r.GET("/p/:user_id", func(c *gin.Context) {
		if callerID != nil {
			c.Set("user_id", callerID)
			c.Set("role_id", role)
		}
		suite.handler.GetPersonalizedRecommendations(c)
	})
	return r
}

func (suite *RecommendationHandlerTestSuite) TestGetPersonalizedRecommendations_InvalidUserID() {
	req := httptest.NewRequest("GET", "/p/abc", nil)
	w := httptest.NewRecorder()
	suite.personalizedRouter(1, 2).ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	assert.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(suite.T(), resp["error"], "Invalid user ID")
}

func (suite *RecommendationHandlerTestSuite) TestGetPersonalizedRecommendations_FloatUserID() {
	req := httptest.NewRequest("GET", "/p/1.5", nil)
	w := httptest.NewRecorder()
	suite.personalizedRouter(1, 2).ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *RecommendationHandlerTestSuite) TestGetPersonalizedRecommendations_RequiresAuthentication() {
	req := httptest.NewRequest("GET", "/p/1", nil)
	w := httptest.NewRecorder()
	suite.personalizedRouter(nil, 0).ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
}

// IDOR: a normal user must not request another user's recommendations.
func (suite *RecommendationHandlerTestSuite) TestGetPersonalizedRecommendations_OtherUserForbidden() {
	req := httptest.NewRequest("GET", "/p/2", nil)
	w := httptest.NewRecorder()
	suite.personalizedRouter(1, 2).ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusForbidden, w.Code)
}

// The caller's own id (and admin for anyone) passes the IDOR check and then
// gets an honest 501, never invented titles.
func (suite *RecommendationHandlerTestSuite) TestGetPersonalizedRecommendations_NotImplementedNeverFabricates() {
	for _, tc := range []struct {
		caller interface{}
		role   int
	}{{1, 2}, {int64(1), 2}, {99, 1}} {
		req := httptest.NewRequest("GET", "/p/1", nil)
		w := httptest.NewRecorder()
		suite.personalizedRouter(tc.caller, tc.role).ServeHTTP(w, req)
		assert.Equal(suite.T(), http.StatusNotImplemented, w.Code)
		assert.NotContains(suite.T(), w.Body.String(), "Personalized Media")
	}
}

func TestRecommendationHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(RecommendationHandlerTestSuite))
}
