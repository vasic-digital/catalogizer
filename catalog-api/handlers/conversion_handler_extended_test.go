package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// RetryJob Tests
// =============================================================================

func TestRetryJob_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(true, nil)
	mockConvSvc.On("RetryJob", 42, 1).Return(nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs/42/retry", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.RetryJob(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Job retry started")
	mockAuthSvc.AssertExpectations(t)
	mockConvSvc.AssertExpectations(t)
}

func TestRetryJob_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs/42/retry", nil)
	// No Authorization header

	handler.RetryJob(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRetryJob_NoPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(false, nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs/42/retry", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.RetryJob(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockAuthSvc.AssertExpectations(t)
}

func TestRetryJob_PermissionCheckError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(false, errors.New("db error"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs/42/retry", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.RetryJob(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockAuthSvc.AssertExpectations(t)
}

func TestRetryJob_InvalidJobID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(true, nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs/abc/retry", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	handler.RetryJob(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid job ID")
}

func TestRetryJob_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(true, nil)
	mockConvSvc.On("RetryJob", 999, 1).Return(errors.New("job not found"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs/999/retry", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "999"}}

	handler.RetryJob(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockConvSvc.AssertExpectations(t)
}

func TestRetryJob_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(true, nil)
	mockConvSvc.On("RetryJob", 42, 1).Return(errors.New("internal failure"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs/42/retry", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.RetryJob(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to retry job")
}

// =============================================================================
// DownloadJobFile Tests
// =============================================================================

func TestDownloadJobFile_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/1/download", nil)

	handler.DownloadJobFile(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDownloadJobFile_InvalidJobID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/abc/download", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	handler.DownloadJobFile(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDownloadJobFile_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockConvSvc.On("GetJob", 999, 1).Return(nil, errors.New("job not found"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/999/download", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "999"}}

	handler.DownloadJobFile(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDownloadJobFile_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockConvSvc.On("GetJob", 42, 1).Return(nil, errors.New("database error"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/42/download", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.DownloadJobFile(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDownloadJobFile_JobNotCompleted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockConvSvc.On("GetJob", 42, 1).Return(&models.ConversionJob{
		ID:     42,
		Status: "pending",
	}, nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/42/download", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.DownloadJobFile(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Job is not completed")
}

func TestDownloadJobFile_NoOutputFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockConvSvc.On("GetJob", 42, 1).Return(&models.ConversionJob{
		ID:         42,
		Status:     "completed",
		TargetPath: "",
	}, nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/42/download", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.DownloadJobFile(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "No output file available")
}

// =============================================================================
// getCurrentUser Extended Tests
// =============================================================================

func TestConversionHandler_GetCurrentUser_TokenWithoutBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	// When token is sent without Bearer prefix, it should be passed as-is
	mockAuthSvc.On("GetCurrentUser", "raw-token").Return(&models.User{ID: 5}, nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "raw-token")

	user, err := handler.getCurrentUser(c)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, 5, user.ID)
}

func TestConversionHandler_GetCurrentUser_AuthServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "bad-token").Return(nil, errors.New("invalid token"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("Authorization", "Bearer bad-token")

	user, err := handler.getCurrentUser(c)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "auth error")
}

// =============================================================================
// DeleteJob Extended Tests
// =============================================================================

func TestDeleteJob_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(true, nil)
	mockConvSvc.On("DeleteJob", 999, 1).Return(errors.New("job not found"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/conversion/jobs/999", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "999"}}

	handler.DeleteJob(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteJob_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(true, nil)
	mockConvSvc.On("DeleteJob", 42, 1).Return(errors.New("internal error"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/conversion/jobs/42", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.DeleteJob(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to delete job")
}

func TestDeleteJob_PermissionCheckError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionManage).Return(false, errors.New("db fail"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/conversion/jobs/42", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	handler.DeleteJob(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// CreateJob Extended Tests
// =============================================================================

func TestCreateJob_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs", nil)

	handler.CreateJob(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateJob_PermissionCheckError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionCreate).Return(false, errors.New("db fail"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")

	handler.CreateJob(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateJob_NoPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockAuthSvc.On("CheckPermission", 1, models.PermissionConversionCreate).Return(false, nil)

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/conversion/jobs", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")

	handler.CreateJob(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// GetJob Extended Tests
// =============================================================================

func TestGetJob_NotFoundError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	mockAuthSvc.On("GetCurrentUser", "test-token").Return(&models.User{ID: 1}, nil)
	mockConvSvc.On("GetJob", 999, 1).Return(nil, errors.New("job not found"))

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/999", nil)
	c.Request.Header.Set("Authorization", "Bearer test-token")
	c.Params = gin.Params{{Key: "id", Value: "999"}}

	handler.GetJob(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetJob_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConvSvc := &MockConversionService{}
	mockAuthSvc := &MockConversionAuthService{}

	handler := NewConversionHandler(mockConvSvc, mockAuthSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs/1", nil)

	handler.GetJob(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
