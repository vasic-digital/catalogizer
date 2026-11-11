package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCopyHandler_CopyToStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	handler := &CopyHandler{
		logger: logger,
	}

	tests := []struct {
		name       string
		body       map[string]string
		wantStatus int
		wantError  bool
	}{
		{
			name: "Valid copy request",
			body: map[string]string{
				"source_path": "/tmp/test.txt",
				"dest_path":   "/storage/test.txt",
				"storage_id":  "local",
			},
			wantStatus: http.StatusNotImplemented, // CAT-001: no storage copy backend; honest 501, never a fake "copied"
			wantError:  true,
		},
		{
			name: "Missing source_path",
			body: map[string]string{
				"dest_path":  "/storage/test.txt",
				"storage_id": "local",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "Missing dest_path",
			body: map[string]string{
				"source_path": "/tmp/test.txt",
				"storage_id":  "local",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "Missing storage_id",
			body: map[string]string{
				"source_path": "/tmp/test.txt",
				"dest_path":   "/storage/test.txt",
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "Empty request body",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			jsonBody, _ := json.Marshal(tt.body)
			c.Request = httptest.NewRequest("POST", "/copy/storage", bytes.NewBuffer(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.CopyToStorage(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.wantError {
				assert.Contains(t, response, "error")
			} else {
				assert.NotContains(t, response, "message")
			}
		})
	}
}

func TestCopyHandler_CopyToStorage_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	handler := &CopyHandler{
		logger: logger,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/copy/storage", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CopyToStorage(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}

func TestCopyHandler_ListStoragePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	handler := &CopyHandler{
		logger: logger,
	}

	tests := []struct {
		name       string
		path       string
		storageID  string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "Valid list request",
			path:       "test",
			storageID:  "local",
			wantStatus: http.StatusNotImplemented, // CAT-001: no fabricated listing
			wantError:  true,
		},
		{
			name:       "Valid list request with nested path",
			path:       "test/subdir",
			storageID:  "local",
			wantStatus: http.StatusNotImplemented,
			wantError:  true,
		},
		{
			name:       "Missing storage_id",
			path:       "test",
			storageID:  "",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "Empty path with storage_id",
			path:       "",
			storageID:  "local",
			wantStatus: http.StatusNotImplemented,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			url := "/storage/list/" + tt.path
			if tt.storageID != "" {
				url += "?storage_id=" + tt.storageID
			}
			c.Request = httptest.NewRequest("GET", url, nil)
			c.Params = gin.Params{{Key: "path", Value: tt.path}}

			// Manually set query parameters
			if tt.storageID != "" {
				c.Request.URL.RawQuery = "storage_id=" + tt.storageID
			}

			handler.ListStoragePath(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.wantError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "path")
				assert.Contains(t, response, "storage_id")
				assert.Contains(t, response, "files")
				assert.Equal(t, tt.path, response["path"])
				assert.Equal(t, tt.storageID, response["storage_id"])
			}
		})
	}
}

// CAT-001: the CopyHandler holds no storage-root registry; it must not serve a hard-coded list.
func TestCopyHandler_GetStorageRoots_IsNotAFabricatedList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &CopyHandler{logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/storage/roots", nil)

	handler.GetStorageRoots(c)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	var response map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.NotContains(t, response, "roots")
	assert.Contains(t, response, "error")
}

func TestCopyHandler_NewCopyHandler(t *testing.T) {
	logger := zap.NewNop()
	tempDir := "/tmp/test"

	handler := NewCopyHandler(nil, nil, tempDir, logger)

	assert.NotNil(t, handler)
	assert.Equal(t, logger, handler.logger)
	assert.Equal(t, tempDir, handler.tempDir)
}

// --- parseHostPath tests ---

func TestCopyHandler_ParseHostPath(t *testing.T) {
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	tests := []struct {
		name         string
		hostPath     string
		expectedHost string
		expectedPath string
	}{
		{
			name:         "valid host:path",
			hostPath:     "server:/share/path",
			expectedHost: "server",
			expectedPath: "/share/path",
		},
		{
			name:         "host only, no path",
			hostPath:     "server:",
			expectedHost: "server",
			expectedPath: "",
		},
		{
			name:         "no colon separator",
			hostPath:     "no-colon",
			expectedHost: "",
			expectedPath: "",
		},
		{
			name:         "empty string",
			hostPath:     "",
			expectedHost: "",
			expectedPath: "",
		},
		{
			name:         "multiple colons",
			hostPath:     "server:path:with:colons",
			expectedHost: "server",
			expectedPath: "path:with:colons",
		},
		{
			name:         "colon at start",
			hostPath:     ":/path",
			expectedHost: "",
			expectedPath: "/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, path := handler.parseHostPath(tt.hostPath)
			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}

// --- CopyToLocal validation tests ---

func TestCopyHandler_CopyToLocal_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/copy/local", bytes.NewBufferString("not json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CopyToLocal(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCopyHandler_CopyToLocal_EmptySourcePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	body := `{"source_path":"","destination_path":"/local/dest"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/copy/local", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CopyToLocal(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCopyHandler_CopyToLocal_InvalidSourceFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	body := `{"source_path":"no-colon","destination_path":"/local/dest"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/copy/local", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CopyToLocal(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- CopyFromLocal validation tests ---

func TestCopyHandler_CopyFromLocal_NoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	handler := &CopyHandler{logger: logger}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/copy/upload", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data")

	handler.CopyFromLocal(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCopyHandler_StorageOperations_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()

	handler := &CopyHandler{
		logger: logger,
	}

	// Test 1: Get storage roots
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest("GET", "/storage/roots", nil)
	handler.GetStorageRoots(c1)
	assert.Equal(t, http.StatusNotImplemented, w1.Code) // CAT-001: no fabricated root list
	storageID := "local"

	// Test 2: List storage path with the storage ID from roots
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/storage/list/test?storage_id="+storageID, nil)
	c2.Params = gin.Params{{Key: "path", Value: "test"}}
	c2.Request.URL.RawQuery = "storage_id=" + storageID
	handler.ListStoragePath(c2)
	assert.Equal(t, http.StatusNotImplemented, w2.Code) // CAT-001: no fabricated listing

	var listResponse map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &listResponse)
	assert.NoError(t, err)
	assert.Equal(t, storageID, listResponse["storage_id"])

	// Test 3: Copy to storage
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	copyBody := map[string]string{
		"source_path": "/tmp/test.txt",
		"dest_path":   "/storage/test.txt",
		"storage_id":  storageID,
	}
	jsonBody, _ := json.Marshal(copyBody)
	c3.Request = httptest.NewRequest("POST", "/copy/storage", bytes.NewBuffer(jsonBody))
	c3.Request.Header.Set("Content-Type", "application/json")
	handler.CopyToStorage(c3)
	// CAT-001: there is no storage copy backend; the endpoint must not claim success.
	assert.Equal(t, http.StatusNotImplemented, w3.Code)

	var copyResponse map[string]interface{}
	err = json.Unmarshal(w3.Body.Bytes(), &copyResponse)
	assert.NoError(t, err)
	assert.Contains(t, copyResponse, "error")
}

// CAT-001: the CopyHandler has no storage backend to list, so it must say so instead of answering 200 with a fabricated empty listing.
func TestCAT001_CopyHandler_ListStoragePath_IsNotAFabricatedEmptyListing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &CopyHandler{logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/storage/list/x?storage_id=1", nil)
	c.Params = gin.Params{{Key: "path", Value: "x"}}
	handler.ListStoragePath(c)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}
