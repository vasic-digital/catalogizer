package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/handlers"
	"catalogizer/internal/middleware"
	"catalogizer/internal/services"
	root_repository "catalogizer/repository"

	"github.com/gin-gonic/gin"
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

	// Should return 200 even if no trending items found
	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func (suite *RecommendationTestSuite) TestPersonalizedRecommendationsEndpoint() {
	req, _ := http.NewRequest("GET", "/api/v1/recommendations/personalized/456", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Should return 200 even if no recommendations found
	assert.Equal(suite.T(), http.StatusOK, w.Code)
}

func TestRecommendationSuite(t *testing.T) {
	suite.Run(t, new(RecommendationTestSuite))
}
