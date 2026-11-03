package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/handlers"
	"catalogizer/internal/middleware"
	"catalogizer/internal/services"
	rootmw "catalogizer/middleware"
	root_repository "catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type RecommendationTestSuite struct {
	suite.Suite
	router                *gin.Engine
	recommendationHandler *handlers.RecommendationHandler
	db                    *database.DB
	tmpDBPath             string
}

func (suite *RecommendationTestSuite) TearDownSuite() {
	if suite.db != nil {
		suite.db.Close()
	}
	if suite.tmpDBPath != "" {
		os.Remove(suite.tmpDBPath)
	}
}

func (suite *RecommendationTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Create a temporary test database file
	tmpFile, err := os.CreateTemp("", "catalogizer-rec-test-*.db")
	suite.Require().NoError(err)
	tmpFile.Close()
	suite.tmpDBPath = tmpFile.Name()

	dbConfig := &config.DatabaseConfig{
		Path: tmpFile.Name(),
	}
	db, err := database.NewConnection(dbConfig)
	suite.Require().NoError(err)
	// Don't close the database here - it will be closed in TearDownSuite
	suite.db = db

	// Use the canonical (migrated) schema, the one the real pipeline writes.
	// The handler reads media_items joined to media_types; a hand-made legacy
	// table makes the query fail with HTTP 500 (regression after the
	// recommendations fix moved the handler to the canonical schema).
	suite.Require().NoError(db.RunMigrations(context.Background()))
	var movieTypeID int
	suite.Require().NoError(db.DB.QueryRow(`SELECT id FROM media_types WHERE name = 'movie'`).Scan(&movieTypeID))
	_, err = db.DB.Exec(`
		INSERT INTO media_items (id, media_type_id, title, year, description, rating, runtime, language, country, director, genre, cast_crew)
		VALUES (123, ?, 'Test Movie', 2023, 'A test movie', 8.5, 120, 'en', 'US', 'Test Director', 'Drama', 'Actor1, Actor2')
	`, movieTypeID)
	suite.Require().NoError(err)

	// Create test logger
	logger, _ := zap.NewDevelopment()

	// Initialize services
	mediaRecognitionService := services.NewMediaRecognitionService(db, logger, nil, nil, "", "", "", "", "", "")
	duplicateDetectionService := services.NewDuplicateDetectionService(db, logger, nil)
	fileRepository := root_repository.NewFileRepository(db)
	recommendationService := services.NewRecommendationService(
		mediaRecognitionService,
		duplicateDetectionService,
		fileRepository,
		db,
	)

	// Create handlers
	suite.recommendationHandler = handlers.NewRecommendationHandler(recommendationService)

	// Setup router
	suite.router = gin.New()
	suite.router.Use(middleware.Logger(logger))

	// Add routes
	api := suite.router.Group("/api/v1")

	// Full recommendation routes
	recGroup := api.Group("/recommendations")
	recGroup.GET("/similar/:media_id", suite.recommendationHandler.GetSimilarItems)
	recGroup.GET("/trending", suite.recommendationHandler.GetTrendingItems)
	recGroup.GET("/personalized/:user_id", suite.recommendationHandler.GetPersonalizedRecommendations)
}

func (suite *RecommendationTestSuite) TestSimilarItemsUnknownIDIs404() {
	req, _ := http.NewRequest("GET", "/api/v1/recommendations/similar/999999", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)
	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *RecommendationTestSuite) TestSimilarItemsEndpoint() {
	req, _ := http.NewRequest("GET", "/api/v1/recommendations/similar/123", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Should return 200 even if no similar items found
	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *RecommendationTestSuite) TestTrendingItemsEndpoint() {
	req, _ := http.NewRequest("GET", "/api/v1/recommendations/trending", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// CAT-001: no trending source exists; the endpoint says so (501) instead
	// of inventing catalogue items.
	assert.Equal(suite.T(), http.StatusNotImplemented, w.Code)
}

func (suite *RecommendationTestSuite) TestPersonalizedRecommendationsEndpoint() {
	req, _ := http.NewRequest("GET", "/api/v1/recommendations/personalized/456", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Unauthenticated callers are refused (IDOR guard), never served data.
	assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)

	// Authenticated callers go through the REAL JWT middleware with a signed token (not c.Set).
	const secret = "rec-suite-secret-0123456789abcdef"
	jwtmw := rootmw.NewJWTMiddleware(secret)
	r := gin.New()
	r.Use(jwtmw.RequireAuth())
	r.GET("/api/v1/recommendations/personalized/:user_id", suite.recommendationHandler.GetPersonalizedRecommendations)
	serve := func(sub string, role int) int {
		claims := &rootmw.Claims{Username: "u" + sub, RoleID: role, RegisteredClaims: jwt.RegisteredClaims{
			Subject: sub, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		suite.Require().NoError(err)
		rq, _ := http.NewRequest("GET", "/api/v1/recommendations/personalized/456", nil)
		rq.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, rq)
		return rec.Code
	}
	// Owner: honest 501 (no engine wired), not fabricated titles.
	assert.Equal(suite.T(), http.StatusNotImplemented, serve("456", 2))
	// Another non-admin user: 403. An admin may ask for anyone: 501.
	assert.Equal(suite.T(), http.StatusForbidden, serve("7", 2))
	assert.Equal(suite.T(), http.StatusNotImplemented, serve("7", 1))
	// No token at all: refused by the middleware itself.
	rq, _ := http.NewRequest("GET", "/api/v1/recommendations/personalized/456", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, rq)
	assert.Equal(suite.T(), http.StatusUnauthorized, rec.Code)
}

func TestRecommendationSuite(t *testing.T) {
	suite.Run(t, new(RecommendationTestSuite))
}
