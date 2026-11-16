package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ProbeAndIngestHandlerTestSuite struct {
	suite.Suite
	handler *ProbeAndIngestHandler
	router  *gin.Engine
	logger  *zap.Logger
}

func (suite *ProbeAndIngestHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.logger = zap.NewNop()
}

func (suite *ProbeAndIngestHandlerTestSuite) SetupTest() {
	// Initialize handler with nil services to test validation paths only.
	// Real SMB probing requires network infrastructure. Nil params let us
	// verify that validation (missing host, bad JSON, etc.) is bulletproof.
	suite.handler = NewProbeAndIngestHandler(nil, nil, suite.logger)

	suite.router = gin.New()
	suite.router.POST("/api/v1/smb/probe-and-ingest", suite.handler.ProbeAndIngest)
}

// TestNewProbeAndIngestHandler verifies the constructor returns a non-nil handler.
func (suite *ProbeAndIngestHandlerTestSuite) TestNewProbeAndIngestHandler() {
	handler := NewProbeAndIngestHandler(nil, nil, suite.logger)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.discoveryService)
	assert.Nil(suite.T(), handler.bindingIngester)
	assert.NotNil(suite.T(), handler.logger)
}

// TestProbeAndIngest_InvalidJSON verifies that non-JSON request body returns 400.
func (suite *ProbeAndIngestHandlerTestSuite) TestProbeAndIngest_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/smb/probe-and-ingest",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"].(string), "Invalid request")
}

// TestProbeAndIngest_MissingHost verifies that omitting the required 'host' field
// returns 400 (bind:"required" validation on ProbeAndIngestRequest.Host).
func (suite *ProbeAndIngestHandlerTestSuite) TestProbeAndIngest_MissingHost() {
	body := `{"host": ""}`
	req := httptest.NewRequest("POST", "/api/v1/smb/probe-and-ingest",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// TestProbeAndIngest_EmptyBody verifies that sending no body at all returns 400.
func (suite *ProbeAndIngestHandlerTestSuite) TestProbeAndIngest_EmptyBody() {
	req := httptest.NewRequest("POST", "/api/v1/smb/probe-and-ingest", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// TestProbeAndIngest_NoBody verifies that sending a request without Content-Type
// still returns 400 because gin binding fails on missing body.
func (suite *ProbeAndIngestHandlerTestSuite) TestProbeAndIngest_NoBody() {
	req := httptest.NewRequest("POST", "/api/v1/smb/probe-and-ingest", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// TestProbeAndIngest_ValidRequestShape verifies that a structurally valid request
// (non-empty host) passes Gin's binding validation. With nil discoveryService
// this will panic at runtime — we only verify the 400 does NOT come from
// binding rejection. The real e2e path is tested via the service-level tests.
func (suite *ProbeAndIngestHandlerTestSuite) TestProbeAndIngest_ValidRequestShape() {
	body := `{"host": "192.168.1.100", "confirm_private_network": true}`
	req := httptest.NewRequest("POST", "/api/v1/smb/probe-and-ingest",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// With nil discoveryService the handler panics. We recover and assert
	// that the panic is FROM the nil-deref (binding passed), not a 400.
	defer func() {
		r := recover()
		assert.NotNil(suite.T(), r, "handler should panic on nil discoveryService — binding validation passed")
	}()
	suite.router.ServeHTTP(w, req)
}

func TestProbeAndIngestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ProbeAndIngestHandlerTestSuite))
}
