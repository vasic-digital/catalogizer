package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CopyHandlerTestSuite struct {
	suite.Suite
	handler  *CopyHandler
	fileRepo *repository.FileRepository
	router   *gin.Engine
}

func (suite *CopyHandlerTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
}

func (suite *CopyHandlerTestSuite) SetupTest() {
	suite.fileRepo = nil
	suite.handler = NewCopyHandler(suite.fileRepo, "/tmp")

	suite.router = gin.New()
	suite.router.POST("/api/copy/smb", suite.handler.CopyToSmb)
	suite.router.POST("/api/copy/local", suite.handler.CopyToLocal)
	suite.router.POST("/api/copy/upload", suite.handler.CopyFromLocal)
}

// Test handler initialization
func (suite *CopyHandlerTestSuite) TestNewCopyHandler() {
	handler := NewCopyHandler(nil, "/tmp")
	assert.NotNil(suite.T(), handler)
	assert.Nil(suite.T(), handler.fileRepo)
	assert.NotNil(suite.T(), handler.smbPool)
	assert.Equal(suite.T(), "/tmp", handler.tempDir)
}

func (suite *CopyHandlerTestSuite) TestNewCopyHandler_WithRepository() {
	fileRepo := &repository.FileRepository{}
	handler := NewCopyHandler(fileRepo, "/custom")
	assert.NotNil(suite.T(), handler)
	assert.Equal(suite.T(), fileRepo, handler.fileRepo)
	assert.Equal(suite.T(), "/custom", handler.tempDir)
}

// Test HTTP method restrictions
func (suite *CopyHandlerTestSuite) TestCopyToSmb_MethodNotAllowed() {
	req := httptest.NewRequest("GET", "/api/copy/smb", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyToLocal_MethodNotAllowed() {
	req := httptest.NewRequest("GET", "/api/copy/local", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyFromLocal_MethodNotAllowed() {
	req := httptest.NewRequest("GET", "/api/copy/upload", nil)
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusNotFound, w.Code)
}

// Test CopyToSmb input validation
func (suite *CopyHandlerTestSuite) TestCopyToSmb_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/copy/smb", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyToSmb_MissingSourceFileID() {
	requestBody := SmbCopyRequest{
		SourceFileID:       0, // Invalid: must be > 0
		DestinationSmbRoot: "main",
		DestinationPath:    "/destination",
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("POST", "/api/copy/smb", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyToSmb_MissingDestinationSmbRoot() {
	requestBody := SmbCopyRequest{
		SourceFileID:       123,
		DestinationSmbRoot: "", // Invalid: required
		DestinationPath:    "/destination",
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("POST", "/api/copy/smb", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyToSmb_MissingDestinationPath() {
	requestBody := SmbCopyRequest{
		SourceFileID:       123,
		DestinationSmbRoot: "main",
		DestinationPath:    "", // Invalid: required
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("POST", "/api/copy/smb", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test CopyToLocal input validation
func (suite *CopyHandlerTestSuite) TestCopyToLocal_InvalidJSON() {
	req := httptest.NewRequest("POST", "/api/copy/local", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyToLocal_MissingSourceFileID() {
	requestBody := LocalCopyRequest{
		SourceFileID:    0, // Invalid: must be > 0
		DestinationPath: "/local/destination",
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("POST", "/api/copy/local", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyToLocal_MissingDestinationPath() {
	requestBody := LocalCopyRequest{
		SourceFileID:    123,
		DestinationPath: "", // Invalid: required
	}
	body, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("POST", "/api/copy/local", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Test CopyFromLocal input validation
func (suite *CopyHandlerTestSuite) TestCopyFromLocal_MissingRequiredFields() {
	// Create empty multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/copy/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	// Should fail validation for missing required fields
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

func (suite *CopyHandlerTestSuite) TestCopyFromLocal_MissingFile() {
	// Create form with fields but no file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("destination_smb_root", "main")
	writer.WriteField("destination_path", "/destination")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/copy/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	suite.router.ServeHTTP(w, req)

	// Should fail validation for missing file
	assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
}

// Note: Tests that would pass validation but fail at repository/SMB level are omitted.
// These tests focus only on HTTP method restrictions, JSON validation, and input validation.

// Run the test suite
func TestCopyHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(CopyHandlerTestSuite))
}

// --- validateSmbCopyRequest tests ---

func TestValidateSmbCopyRequest(t *testing.T) {
	handler := NewCopyHandler(nil, "/tmp")

	tests := []struct {
		name      string
		req       SmbCopyRequest
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid request",
			req:       SmbCopyRequest{SourceFileID: 1, DestinationSmbRoot: "main", DestinationPath: "/dest"},
			expectErr: false,
		},
		{
			name:      "zero source file ID",
			req:       SmbCopyRequest{SourceFileID: 0, DestinationSmbRoot: "main", DestinationPath: "/dest"},
			expectErr: true,
			errMsg:    "source_file_id is required",
		},
		{
			name:      "negative source file ID",
			req:       SmbCopyRequest{SourceFileID: -1, DestinationSmbRoot: "main", DestinationPath: "/dest"},
			expectErr: true,
			errMsg:    "source_file_id is required",
		},
		{
			name:      "empty destination smb root",
			req:       SmbCopyRequest{SourceFileID: 1, DestinationSmbRoot: "", DestinationPath: "/dest"},
			expectErr: true,
			errMsg:    "destination_smb_root is required",
		},
		{
			name:      "empty destination path",
			req:       SmbCopyRequest{SourceFileID: 1, DestinationSmbRoot: "main", DestinationPath: ""},
			expectErr: true,
			errMsg:    "destination_path is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := handler.validateSmbCopyRequest(&tc.req)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLocalCopyRequest(t *testing.T) {
	handler := NewCopyHandler(nil, "/tmp")

	tests := []struct {
		name      string
		req       LocalCopyRequest
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid request inside the local root",
			req:       LocalCopyRequest{SourceFileID: 1, DestinationPath: "/tmp/dest"},
			expectErr: false,
		},
		{
			name:      "absolute path outside the local root is rejected",
			req:       LocalCopyRequest{SourceFileID: 1, DestinationPath: "/local/dest"},
			expectErr: true,
			errMsg:    "invalid destination_path",
		},
		{
			name:      "zero source file ID",
			req:       LocalCopyRequest{SourceFileID: 0, DestinationPath: "/tmp/dest"},
			expectErr: true,
			errMsg:    "source_file_id is required",
		},
		{
			name:      "negative source file ID",
			req:       LocalCopyRequest{SourceFileID: -1, DestinationPath: "/tmp/dest"},
			expectErr: true,
			errMsg:    "source_file_id is required",
		},
		{
			name:      "empty destination path",
			req:       LocalCopyRequest{SourceFileID: 1, DestinationPath: ""},
			expectErr: true,
			errMsg:    "destination_path is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := handler.validateLocalCopyRequest(&tc.req)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- CopyResponse struct test ---

func TestCopyResponse_Fields(t *testing.T) {
	resp := CopyResponse{
		Success:     true,
		BytesCopied: 1024,
		FilesCount:  5,
		TimeTaken:   100 * time.Millisecond,
		SourcePath:  "/source/path",
		DestPath:    "/dest/path",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, int64(1024), resp.BytesCopied)
	assert.Equal(t, 5, resp.FilesCount)
	assert.Equal(t, 100*time.Millisecond, resp.TimeTaken)
	assert.Equal(t, "/source/path", resp.SourcePath)
	assert.Equal(t, "/dest/path", resp.DestPath)
}

// --- Close method test ---

func TestCopyHandler_Close(t *testing.T) {
	handler := NewCopyHandler(nil, "/tmp")
	// Close should not panic with a fresh pool
	assert.NotPanics(t, func() {
		handler.Close()
	})
}
