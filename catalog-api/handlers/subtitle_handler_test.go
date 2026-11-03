package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type SubtitleHandlerTestSuite struct {
	suite.Suite
	handler *SubtitleHandler
	router  *gin.Engine
	logger  *zap.Logger
}

func (suite *SubtitleHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.logger = zap.NewNop()
}

func (suite *SubtitleHandlerTestSuite) SetupTest() {
	// Initialize handler with nil service to test validation paths
	suite.handler = NewSubtitleHandler(nil, suite.logger)

	suite.router = gin.New()
	suite.router.GET("/api/v1/subtitles/search", suite.handler.SearchSubtitles)
	suite.router.POST("/api/v1/subtitles/download", suite.handler.DownloadSubtitle)
	suite.router.GET("/api/v1/subtitles/media/:media_id", suite.handler.GetSubtitles)
	suite.router.GET("/api/v1/subtitles/:subtitle_id/verify-sync/:media_id", suite.handler.VerifySubtitleSync)
	suite.router.POST("/api/v1/subtitles/translate", suite.handler.TranslateSubtitle)
	suite.router.GET("/api/v1/subtitles/languages", suite.handler.GetSupportedLanguages)
	suite.router.GET("/api/v1/subtitles/providers", suite.handler.GetSupportedProviders)
	suite.router.POST("/api/v1/subtitles/upload", suite.handler.UploadSubtitle)
}

// Constructor tests

func (suite *SubtitleHandlerTestSuite) TestNewSubtitleHandler() {
	handler := NewSubtitleHandler(nil, suite.logger)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.subtitleService)
	assert.NotNil(suite.T(), handler.logger)
}

func (suite *SubtitleHandlerTestSuite) TestNewSubtitleHandler_NilLogger() {
	handler := NewSubtitleHandler(nil, nil)
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.logger)
}

// SearchSubtitles tests

func (suite *SubtitleHandlerTestSuite) TestSearchSubtitles_MissingMediaPath() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/search", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), false, resp.Success)
	assert.Equal(suite.T(), "MISSING_MEDIA_PATH", resp.Code)
	assert.Contains(suite.T(), resp.Error, "media_path is required")
}

func (suite *SubtitleHandlerTestSuite) TestSearchSubtitles_EmptyMediaPath() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/search?media_path=", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *SubtitleHandlerTestSuite) TestSearchSubtitles_MethodNotAllowed() {
	req := httptest.NewRequest("POST", "/api/v1/subtitles/search", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// DownloadSubtitle tests

func (suite *SubtitleHandlerTestSuite) TestDownloadSubtitle_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/subtitles/download", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INVALID_REQUEST", resp.Code)
}

func (suite *SubtitleHandlerTestSuite) TestDownloadSubtitle_MissingRequiredFields() {
	body := `{"media_item_id": 0, "result_id": "", "language": ""}`
	req := httptest.NewRequest("POST", "/api/v1/subtitles/download", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "MISSING_REQUIRED_FIELDS", resp.Code)
}

func (suite *SubtitleHandlerTestSuite) TestDownloadSubtitle_MissingMediaItemID() {
	body := `{"result_id": "abc", "language": "en"}`
	req := httptest.NewRequest("POST", "/api/v1/subtitles/download", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *SubtitleHandlerTestSuite) TestDownloadSubtitle_EmptyBody() {
	req := httptest.NewRequest("POST", "/api/v1/subtitles/download", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// GetSubtitles tests

func (suite *SubtitleHandlerTestSuite) TestGetSubtitles_InvalidMediaID() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/media/abc", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INVALID_MEDIA_ID", resp.Code)
}

// VerifySubtitleSync tests

func (suite *SubtitleHandlerTestSuite) TestVerifySubtitleSync_InvalidMediaID() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/sub1/verify-sync/abc", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INVALID_MEDIA_ID", resp.Code)
}

// TranslateSubtitle tests

func (suite *SubtitleHandlerTestSuite) TestTranslateSubtitle_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/v1/subtitles/translate", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INVALID_REQUEST", resp.Code)
}

func (suite *SubtitleHandlerTestSuite) TestTranslateSubtitle_MissingRequiredFields() {
	body := `{"subtitle_id": "", "source_language": "", "target_language": ""}`
	req := httptest.NewRequest("POST", "/api/v1/subtitles/translate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "MISSING_REQUIRED_FIELDS", resp.Code)
}

func (suite *SubtitleHandlerTestSuite) TestTranslateSubtitle_PartialFields() {
	body := `{"subtitle_id": "sub1", "source_language": "en", "target_language": ""}`
	req := httptest.NewRequest("POST", "/api/v1/subtitles/translate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// GetSupportedLanguages tests

func (suite *SubtitleHandlerTestSuite) TestGetSupportedLanguages_Success() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/languages", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	var resp LanguageListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), resp.Success)
	assert.Equal(suite.T(), 19, resp.Count)
	assert.Len(suite.T(), resp.Languages, 19)
}

func (suite *SubtitleHandlerTestSuite) TestGetSupportedLanguages_ContainsEnglish() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/languages", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	var resp LanguageListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)

	foundEnglish := false
	for _, lang := range resp.Languages {
		if lang.Code == "en" {
			foundEnglish = true
			assert.Equal(suite.T(), "English", lang.Name)
			assert.Equal(suite.T(), "English", lang.NativeName)
		}
	}
	assert.True(suite.T(), foundEnglish, "English should be in supported languages")
}

// GetSupportedProviders tests

func (suite *SubtitleHandlerTestSuite) TestGetSupportedProviders_Success() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/providers", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	var resp ProviderListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), resp.Success)
	// CAT-001: only providers with a working client are listed; none of the five has one yet.
	assert.Equal(suite.T(), 0, resp.Count)
	assert.NotNil(suite.T(), resp.Providers)
	assert.Len(suite.T(), resp.Providers, 0)
}

func (suite *SubtitleHandlerTestSuite) TestGetSupportedProviders_AllSupported() {
	req := httptest.NewRequest("GET", "/api/v1/subtitles/providers", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	var resp ProviderListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)

	for _, provider := range resp.Providers {
		assert.True(suite.T(), provider.Supported, "All providers should be marked as supported")
		assert.NotEmpty(suite.T(), provider.Name, "Provider name should not be empty")
		assert.NotEmpty(suite.T(), provider.Description, "Provider description should not be empty")
	}
}

// CAT-001 F3: with no working provider client, a search is 501 NOT_IMPLEMENTED, not a 500 SEARCH_FAILED.
func TestCAT001_F3_SearchWithoutWorkingProviderIs501(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSubtitleHandler(services.NewSubtitleService(nil, zap.NewNop(), nil), zap.NewNop())
	r := gin.New()
	r.GET("/api/v1/subtitles/search", h.SearchSubtitles)
	for _, q := range []string{"", "&providers=subscene", "&providers=addic7ed,opensubtitles"} {
		req := httptest.NewRequest("GET", "/api/v1/subtitles/search?media_path=/m/x.mkv"+q, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotImplemented, w.Code, q)
		var resp ErrorResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "NOT_IMPLEMENTED", resp.Code)
	}
}

// UploadSubtitle tests

func (suite *SubtitleHandlerTestSuite) TestUploadSubtitle_MissingRequiredFields() {
	req := httptest.NewRequest("POST", "/api/v1/subtitles/upload", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "MISSING_REQUIRED_FIELDS", resp.Code)
}

func (suite *SubtitleHandlerTestSuite) TestUploadSubtitle_InvalidMediaID() {
	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"media_item_id\"\r\n\r\n")
	body.WriteString("not-a-number\r\n")
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"language\"\r\n\r\n")
	body.WriteString("English\r\n")
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"language_code\"\r\n\r\n")
	body.WriteString("en\r\n")
	body.WriteString("--boundary--\r\n")

	req := httptest.NewRequest("POST", "/api/v1/subtitles/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
	var resp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "INVALID_MEDIA_ID", resp.Code)
}

// Helper function tests

func (suite *SubtitleHandlerTestSuite) TestGetProviderDisplayName() {
	assert.Equal(suite.T(), "OpenSubtitles", getProviderDisplayName("opensubtitles"))
	assert.Equal(suite.T(), "SubDB", getProviderDisplayName("subdb"))
	assert.Equal(suite.T(), "YIFY Subtitles", getProviderDisplayName("yifysubtitles"))
	assert.Equal(suite.T(), "Subscene", getProviderDisplayName("subscene"))
	assert.Equal(suite.T(), "Addic7ed", getProviderDisplayName("addic7ed"))
	assert.Equal(suite.T(), "unknown", getProviderDisplayName("unknown"))
}

func (suite *SubtitleHandlerTestSuite) TestGetProviderDescription() {
	desc := getProviderDescription("opensubtitles")
	assert.Contains(suite.T(), desc, "subtitle database")

	desc = getProviderDescription("subdb")
	assert.Contains(suite.T(), desc, "Hash-based")

	desc = getProviderDescription("unknown")
	assert.Equal(suite.T(), "Subtitle provider", desc)
}

func TestSubtitleHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(SubtitleHandlerTestSuite))
}
