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

type SMBDiscoveryHandlerTestSuite struct {
	suite.Suite
	handler *SMBDiscoveryHandler
	router  *gin.Engine
	logger  *zap.Logger
}

func (suite *SMBDiscoveryHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.logger = zap.NewNop()
}

func (suite *SMBDiscoveryHandlerTestSuite) SetupTest() {
	// Initialize handler with nil service to test validation paths
	suite.handler = NewSMBDiscoveryHandler(nil, suite.logger)

	suite.router = gin.New()
	suite.router.POST("/api/v1/smb/discover", suite.handler.DiscoverShares)
	suite.router.POST("/api/v1/smb/test", suite.handler.TestConnection)
	suite.router.POST("/api/v1/smb/browse", suite.handler.BrowseShare)
	suite.router.GET("/api/v1/smb/discover", suite.handler.DiscoverSharesGET)
	suite.router.GET("/api/v1/smb/test", suite.handler.TestConnectionGET)
}

// Constructor tests

func (suite *SMBDiscoveryHandlerTestSuite) TestNewSMBDiscoveryHandler() {
	handler := NewSMBDiscoveryHandler(nil, suite.logger)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.service)
	assert.NotNil(suite.T(), handler.logger)
}

// DiscoverShares (POST) tests

func (suite *SMBDiscoveryHandlerTestSuite) TestDiscoverShares_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/smb/discover", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), resp["error"].(string), "Invalid request")
}

func (suite *SMBDiscoveryHandlerTestSuite) TestDiscoverShares_MissingRequiredFields() {
	body := `{"host": ""}`
	req := httptest.NewRequest("POST", "/api/v1/smb/discover", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *SMBDiscoveryHandlerTestSuite) TestDiscoverShares_EmptyBody() {
	req := httptest.NewRequest("POST", "/api/v1/smb/discover", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// TestConnection (POST) tests

func (suite *SMBDiscoveryHandlerTestSuite) TestTestConnection_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/smb/test", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *SMBDiscoveryHandlerTestSuite) TestTestConnection_MissingFields() {
	body := `{"host": "192.168.1.1"}`
	req := httptest.NewRequest("POST", "/api/v1/smb/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// BrowseShare tests

func (suite *SMBDiscoveryHandlerTestSuite) TestBrowseShare_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/smb/browse", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *SMBDiscoveryHandlerTestSuite) TestBrowseShare_MissingFields() {
	body := `{"host": "192.168.1.1"}`
	req := httptest.NewRequest("POST", "/api/v1/smb/browse", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// SECQ-5: the credential-in-URL GET forms are retired. A request that carries credentials in the query string must be
// refused with 410 Gone before any parameter is read, the response must never echo the secret, and the service (nil here,
// so any use would panic) must never be reached.

const secq5Secret = "s3cr3t-SECQ5-pw"

func (suite *SMBDiscoveryHandlerTestSuite) assertGone(path string) {
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusGone, w.Code)
	assert.NotContains(suite.T(), w.Body.String(), secq5Secret)
	var resp map[string]interface{}
	assert.NoError(suite.T(), json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(suite.T(), resp["error"].(string), "POST")
}

func (suite *SMBDiscoveryHandlerTestSuite) TestDiscoverSharesGET_RetiredWithCredentials() {
	suite.assertGone("/api/v1/smb/discover?host=192.168.1.1&username=user&password=" + secq5Secret)
}

func (suite *SMBDiscoveryHandlerTestSuite) TestDiscoverSharesGET_RetiredWithoutParams() {
	suite.assertGone("/api/v1/smb/discover")
}

func (suite *SMBDiscoveryHandlerTestSuite) TestTestConnectionGET_RetiredWithCredentials() {
	suite.assertGone("/api/v1/smb/test?host=192.168.1.1&share=s&username=user&password=" + secq5Secret)
}

func (suite *SMBDiscoveryHandlerTestSuite) TestTestConnectionGET_RetiredWithoutParams() {
	suite.assertGone("/api/v1/smb/test")
}

func TestSMBDiscoveryHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(SMBDiscoveryHandlerTestSuite))
}
