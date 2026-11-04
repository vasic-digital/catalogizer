package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockConversionService for testing
type MockConversionService struct {
	mock.Mock
}

func (m *MockConversionService) CreateConversionJob(userID int, request *models.ConversionRequest) (*models.ConversionJob, error) {
	args := m.Called(userID, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversionJob), args.Error(1)
}

func (m *MockConversionService) GetJob(jobID int, userID int) (*models.ConversionJob, error) {
	args := m.Called(jobID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversionJob), args.Error(1)
}

func (m *MockConversionService) GetUserJobs(userID int, status *string, limit, offset int) ([]models.ConversionJob, error) {
	args := m.Called(userID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ConversionJob), args.Error(1)
}

func (m *MockConversionService) CancelJob(jobID int, userID int) error {
	args := m.Called(jobID, userID)
	return args.Error(0)
}

func (m *MockConversionService) RetryJob(jobID int, userID int) error {
	args := m.Called(jobID, userID)
	return args.Error(0)
}

func (m *MockConversionService) DeleteJob(jobID int, userID int) error {
	args := m.Called(jobID, userID)
	return args.Error(0)
}

func (m *MockConversionService) GetSupportedFormats() *models.SupportedFormats {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*models.SupportedFormats)
}

// MockConversionAuthService for testing
type MockConversionAuthService struct {
	mock.Mock
}

func (m *MockConversionAuthService) CheckPermission(userID int, permission string) (bool, error) {
	args := m.Called(userID, permission)
	return args.Bool(0), args.Error(1)
}

func (m *MockConversionAuthService) GetCurrentUser(token string) (*models.User, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// TestCreateJob tests the conversion handler's CreateJob method
func TestCreateJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         int
		hasPermission  bool
		permissionErr  error
		requestData    *models.ConversionRequest
		mockResponse   *models.ConversionJob
		serviceError   error
		expectedStatus int
		expectedError  bool
	}{
		{
			name:          "Success",
			userID:        1,
			hasPermission: true,
			permissionErr: nil,
			requestData: &models.ConversionRequest{
				SourcePath:   "/input/test.pdf",
				TargetPath:   "/output/test.docx",
				SourceFormat: "pdf",
				TargetFormat: "docx",
				Quality:      "high",
			},
			mockResponse:   &models.ConversionJob{ID: 123, Status: "pending"},
			serviceError:   nil,
			expectedStatus: 200,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockConversionService := &MockConversionService{}
			mockAuthService := &MockConversionAuthService{}

			// Setup expectations
			mockAuthService.On("CheckPermission", tt.userID, models.PermissionConversionCreate).Return(tt.hasPermission, tt.permissionErr)
			mockAuthService.On("GetCurrentUser", "test-token").Return(&models.User{ID: tt.userID}, nil)
			mockConversionService.On("CreateConversionJob", tt.userID, tt.requestData).Return(tt.mockResponse, tt.serviceError)

			// Create handler with mocks
			handler := NewConversionHandler(mockConversionService, mockAuthService)

			// Setup request
			body, _ := json.Marshal(tt.requestData)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("POST", "/conversion/jobs", bytes.NewBuffer(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Authorization", "Bearer test-token")
			c.Set("user_id", tt.userID)

			// Call handler
			handler.CreateJob(c)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectedError && tt.mockResponse != nil {
				var responseJob models.ConversionJob
				err := json.Unmarshal(w.Body.Bytes(), &responseJob)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResponse.ID, responseJob.ID)
			}

			mockAuthService.AssertExpectations(t)
			mockConversionService.AssertExpectations(t)
		})
	}
}

func TestGetJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         int
		jobID          string
		mockJob        *models.ConversionJob
		serviceError   error
		expectedStatus int
		setupMock      bool
	}{
		{
			name:   "Success",
			userID: 1,
			jobID:  "123",
			mockJob: &models.ConversionJob{
				ID:     123,
				Status: "completed",
			},
			serviceError:   nil,
			expectedStatus: 200,
			setupMock:      true,
		},
		{
			name:           "InvalidJobID",
			userID:         1,
			jobID:          "invalid",
			mockJob:        nil,
			serviceError:   nil,
			expectedStatus: 400,
			setupMock:      false,
		},
		{
			name:           "JobNotFound",
			userID:         1,
			jobID:          "999",
			mockJob:        nil,
			serviceError:   assert.AnError,
			expectedStatus: 500,
			setupMock:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConversionService := &MockConversionService{}
			mockAuthService := &MockConversionAuthService{}

			mockAuthService.On("GetCurrentUser", "test-token").Return(&models.User{ID: tt.userID}, nil)
			if tt.setupMock {
				mockConversionService.On("GetJob", mock.AnythingOfType("int"), tt.userID).Return(tt.mockJob, tt.serviceError)
			}

			handler := NewConversionHandler(mockConversionService, mockAuthService)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/conversion/jobs/"+tt.jobID, nil)
			c.Request.Header.Set("Authorization", "Bearer test-token")
			c.Params = gin.Params{{Key: "id", Value: tt.jobID}}

			handler.GetJob(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockAuthService.AssertExpectations(t)
		})
	}
}

func TestListJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         int
		hasPermission  bool
		mockJobs       []models.ConversionJob
		serviceError   error
		expectedStatus int
	}{
		{
			name:          "Success",
			userID:        1,
			hasPermission: true,
			mockJobs: []models.ConversionJob{
				{ID: 1, Status: "completed"},
				{ID: 2, Status: "pending"},
			},
			serviceError:   nil,
			expectedStatus: 200,
		},
		{
			name:           "NoPermission",
			userID:         1,
			hasPermission:  false,
			mockJobs:       nil,
			serviceError:   nil,
			expectedStatus: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConversionService := &MockConversionService{}
			mockAuthService := &MockConversionAuthService{}

			mockAuthService.On("GetCurrentUser", "test-token").Return(&models.User{ID: tt.userID}, nil)
			mockAuthService.On("CheckPermission", tt.userID, models.PermissionConversionView).Return(tt.hasPermission, nil)
			if tt.hasPermission {
				mockConversionService.On("GetUserJobs", tt.userID, mock.AnythingOfType("*string"), 50, 0).Return(tt.mockJobs, tt.serviceError)
			}

			handler := NewConversionHandler(mockConversionService, mockAuthService)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/conversion/jobs", nil)
			c.Request.Header.Set("Authorization", "Bearer test-token")

			handler.ListJobs(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockAuthService.AssertExpectations(t)
		})
	}
}

func TestCancelJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         int
		jobID          string
		hasPermission  bool
		serviceError   error
		expectedStatus int
	}{
		{
			name:           "Success",
			userID:         1,
			jobID:          "123",
			hasPermission:  true,
			serviceError:   nil,
			expectedStatus: 200,
		},
		{
			name:           "NoPermission",
			userID:         1,
			jobID:          "123",
			hasPermission:  false,
			serviceError:   nil,
			expectedStatus: 403,
		},
		{
			name:           "InvalidJobID",
			userID:         1,
			jobID:          "invalid",
			hasPermission:  true,
			serviceError:   nil,
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConversionService := &MockConversionService{}
			mockAuthService := &MockConversionAuthService{}

			mockAuthService.On("GetCurrentUser", "test-token").Return(&models.User{ID: tt.userID}, nil)
			mockAuthService.On("CheckPermission", tt.userID, models.PermissionConversionManage).Return(tt.hasPermission, nil)
			if tt.hasPermission && tt.jobID == "123" {
				mockConversionService.On("CancelJob", 123, tt.userID).Return(tt.serviceError)
			}

			handler := NewConversionHandler(mockConversionService, mockAuthService)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("DELETE", "/conversion/jobs/"+tt.jobID, nil)
			c.Request.Header.Set("Authorization", "Bearer test-token")
			c.Params = gin.Params{{Key: "id", Value: tt.jobID}}

			handler.CancelJob(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockAuthService.AssertExpectations(t)
		})
	}
}

func TestGetSupportedFormats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		userID         int
		hasPermission  bool
		mockFormats    *models.SupportedFormats
		expectedStatus int
	}{
		{
			name:          "Success",
			userID:        1,
			hasPermission: true,
			mockFormats: &models.SupportedFormats{
				Audio: models.AudioFormats{
					Input:  []string{"mp3", "wav"},
					Output: []string{"mp3", "wav"},
				},
				Video: models.VideoFormats{
					Input:  []string{"mp4", "mkv"},
					Output: []string{"mp4", "mkv"},
				},
			},
			expectedStatus: 200,
		},
		{
			name:           "NoPermission",
			userID:         1,
			hasPermission:  false,
			mockFormats:    nil,
			expectedStatus: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConversionService := &MockConversionService{}
			mockAuthService := &MockConversionAuthService{}

			mockAuthService.On("GetCurrentUser", "test-token").Return(&models.User{ID: tt.userID}, nil)
			mockAuthService.On("CheckPermission", tt.userID, models.PermissionConversionView).Return(tt.hasPermission, nil)
			if tt.hasPermission {
				mockConversionService.On("GetSupportedFormats").Return(tt.mockFormats)
			}

			handler := NewConversionHandler(mockConversionService, mockAuthService)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/conversion/formats", nil)
			c.Request.Header.Set("Authorization", "Bearer test-token")

			handler.GetSupportedFormats(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockAuthService.AssertExpectations(t)
		})
	}
}

func TestConversionHandler_GetCurrentUser_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConversionService := &MockConversionService{}
	mockAuthService := &MockConversionAuthService{}

	handler := NewConversionHandler(mockConversionService, mockAuthService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs", nil)

	user, err := handler.getCurrentUser(c)

	assert.Nil(t, user)
	assert.Error(t, err)
}

func TestConversionHandler_GetCurrentUser_WithToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockConversionService := &MockConversionService{}
	mockAuthService := &MockConversionAuthService{}

	mockAuthService.On("GetCurrentUser", "valid-token").Return(&models.User{ID: 1}, nil)

	handler := NewConversionHandler(mockConversionService, mockAuthService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/conversion/jobs", nil)
	c.Request.Header.Set("Authorization", "Bearer valid-token")

	user, err := handler.getCurrentUser(c)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, 1, user.ID)
	mockAuthService.AssertExpectations(t)
}
