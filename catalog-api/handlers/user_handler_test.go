package handlers

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserService for testing
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) Create(user *models.User) (int, error) {
	args := m.Called(user)
	return args.Int(0), args.Error(1)
}

func (m *MockUserService) GetByID(id int) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) Update(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserService) Delete(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserService) List(limit, offset int) ([]models.User, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.User), args.Error(1)
}

func (m *MockUserService) GetRole(roleID int) (*models.Role, error) {
	args := m.Called(roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}

func (m *MockUserService) Count() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

// MockUserAuthService for testing
type MockUserAuthService struct {
	mock.Mock
}

func (m *MockUserAuthService) CheckPermission(userID int, permission string) (bool, error) {
	args := m.Called(userID, permission)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserAuthService) GetCurrentUser(token string) (*models.User, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserAuthService) HashPassword(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *MockUserAuthService) ValidatePassword(password string) error {
	args := m.Called(password)
	return args.Error(0)
}

func (m *MockUserAuthService) GenerateSecureToken(length int) (string, error) {
	args := m.Called(length)
	return args.String(0), args.Error(1)
}

func (m *MockUserAuthService) ResetPassword(userID int, newPassword string) error {
	args := m.Called(userID, newPassword)
	return args.Error(0)
}

func (m *MockUserAuthService) LockAccount(userID int, lockUntil time.Time) error {
	args := m.Called(userID, lockUntil)
	return args.Error(0)
}

func (m *MockUserAuthService) UnlockAccount(userID int) error {
	args := m.Called(userID)
	return args.Error(0)
}

func (m *MockUserAuthService) HashData(data string) string {
	args := m.Called(data)
	return args.String(0)
}

// TestUserHandler_NewUserHandler
func TestUserHandler_NewUserHandler(t *testing.T) {
	mockUserService := new(MockUserService)
	mockAuthService := new(MockUserAuthService)

	handler := NewUserHandler(mockUserService, mockAuthService)

	assert.NotNil(t, handler)
	assert.Equal(t, mockUserService, handler.userRepo)
	assert.Equal(t, mockAuthService, handler.authService)
}

// TestUserHandler_CreateUser tests CreateUser handler method
func TestUserHandler_CreateUser(t *testing.T) {
	tests := []struct {
		name               string
		requestBody        string
		mockSetup          func(*MockUserService, *MockUserAuthService)
		expectedStatusCode int
		expectedBody       string
	}{
		{
			name: "successful user creation",
			requestBody: `{
				"username": "testuser",
				"email": "test@example.com",
				"password": "password123",
				"role_id": 1,
				"first_name": "Test",
				"last_name": "User"
			}`,
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserCreate).Return(true, nil)
				authService.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil).Maybe()
				authService.On("ValidatePassword", "password123").Return(nil)
				authService.On("GenerateSecureToken", 16).Return("random-salt-123", nil)
				authService.On("HashData", mock.AnythingOfType("string")).Return("hashed-combined")

				userRepo.On("Create", mock.AnythingOfType("*models.User")).Return(1, nil)
			},
			expectedStatusCode: http.StatusCreated,
		},
		{
			name:               "unauthorized - no token",
			requestBody:        `{"username": "test", "password": "password123"}`,
			mockSetup:          func(userRepo *MockUserService, authService *MockUserAuthService) {},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "insufficient permissions",
			requestBody: `{
				"username": "testuser",
				"password": "password123"
			}`,
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserCreate).Return(false, nil)
			},
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "invalid request body",
			requestBody:        `{invalid json}`,
			mockSetup:          func(userRepo *MockUserService, authService *MockUserAuthService) {},
			expectedStatusCode: http.StatusUnauthorized, // Handler tries to extract token first
		},
		{
			// Article XI §11.5: payload is now COMPLETE (username +
			// email + role_id + password). The handler-level
			// required-field validation (added 2026-04-29 alongside
			// FQA-API-244 fix) rejects incomplete payloads with 400
			// BEFORE reaching ValidatePassword, so we have to send a
			// complete payload to actually exercise the
			// password-validation code path.
			name: "password validation error",
			requestBody: `{
				"username": "testuser",
				"email": "testuser@example.com",
				"role_id": 2,
				"password": "123"
			}`,
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserCreate).Return(true, nil)
				authService.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil).Maybe()
				authService.On("ValidatePassword", "123").Return(assert.AnError)
			},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			// Same Article XI §11.5 reasoning: send a complete
			// payload so the handler reaches userRepo.Create where
			// the duplicate-username conflict surfaces.
			name: "username already exists",
			requestBody: `{
				"username": "existinguser",
				"email": "existinguser@example.com",
				"role_id": 2,
				"password": "password123"
			}`,
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserCreate).Return(true, nil)
				authService.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil).Maybe()
				authService.On("ValidatePassword", "password123").Return(nil)
				authService.On("GenerateSecureToken", 16).Return("random-salt-123", nil)
				authService.On("HashData", mock.AnythingOfType("string")).Return("hashed-combined")

				userRepo.On("Create", mock.AnythingOfType("*models.User")).Return(0, assert.AnError)
			},
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			// Article XI §11.5 regression guard for FQA-API-244 /
			// FQA-API-248: incomplete payload (missing email,
			// role_id) MUST be rejected with 400 + diagnostic body
			// — NOT 500 "Failed to create user". Caught by the
			// 2026-04-29 real-binary bank verification.
			name: "incomplete payload returns 400 with diagnostic",
			requestBody: `{
				"username": "incomplete",
				"password": "Password123!",
				"role": "user"
			}`,
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserCreate).Return(true, nil)
				authService.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil).Maybe()
				// no ValidatePassword / Create expectations — handler
				// must short-circuit on missing required fields.
			},
			expectedStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh mocks for each test
			mockUserService := new(MockUserService)
			mockAuthService := new(MockUserAuthService)

			// Setup mocks
			if tt.mockSetup != nil {
				tt.mockSetup(mockUserService, mockAuthService)
			}

			handler := NewUserHandler(mockUserService, mockAuthService)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Add token for authenticated requests
			if strings.Contains(tt.requestBody, "password") && tt.name != "unauthorized - no token" {
				req.Header.Set("Authorization", "Bearer valid-token")
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.CreateUser(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatusCode, rr.Code)

			// Article XI §11.5: diagnostic-body assertion. The
			// "incomplete payload" case ships back a JSON body with
			// `error: missing required fields` and a `details` list
			// of which fields are missing. That body must persist —
			// regressing it (e.g. http.Error(w, "Failed to create
			// user", 500)) is the exact bluff pattern we just fixed.
			if tt.name == "incomplete payload returns 400 with diagnostic" {
				body := rr.Body.String()
				assert.Contains(t, body, "missing required fields",
					"diagnostic body must say which class of error fired")
				assert.Contains(t, body, "email",
					"diagnostic body must enumerate the missing field 'email'")
				assert.Contains(t, body, "role_id",
					"diagnostic body must enumerate the missing field 'role_id'")
			}

			// Check expectations
			mockUserService.AssertExpectations(t)
			mockAuthService.AssertExpectations(t)
		})
	}
}

// TestUserHandler_GetUser tests GetUser handler method
func TestUserHandler_GetUser(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		mockSetup          func(*MockUserService, *MockUserAuthService)
		expectedStatusCode int
	}{
		{
			name: "successful get user - self",
			url:  "/api/users/1",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "testuser"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)

				targetUser := &models.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
					RoleID:   1,
				}
				userRepo.On("GetByID", 1).Return(targetUser, nil)

				role := &models.Role{ID: 1, Name: "User"}
				userRepo.On("GetRole", 1).Return(role, nil)
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "successful get user - admin with permission",
			url:  "/api/users/2",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserView).Return(true, nil)

				targetUser := &models.User{
					ID:       2,
					Username: "otheruser",
					Email:    "other@example.com",
					RoleID:   1,
				}
				userRepo.On("GetByID", 2).Return(targetUser, nil)

				role := &models.Role{ID: 1, Name: "User"}
				userRepo.On("GetRole", 1).Return(role, nil)
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "unauthorized - no token",
			url:  "/api/users/1",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				// No mocks needed - handler returns ErrUnauthorized before calling GetCurrentUser
			},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "insufficient permissions - different user without permission",
			url:  "/api/users/2",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "testuser"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserView).Return(false, nil)
			},
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name: "invalid user ID",
			url:  "/api/users/invalid",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "testuser"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
			},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "user not found",
			url:  "/api/users/999",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserView).Return(true, nil)

				userRepo.On("GetByID", 999).Return(nil, errors.New("user not found"))
			},
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh mocks for each test
			mockUserService := new(MockUserService)
			mockAuthService := new(MockUserAuthService)

			// Setup mocks
			if tt.mockSetup != nil {
				tt.mockSetup(mockUserService, mockAuthService)
			}

			handler := NewUserHandler(mockUserService, mockAuthService)

			// Create request
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if tt.name != "unauthorized - no token" {
				req.Header.Set("Authorization", "Bearer valid-token")
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.GetUser(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatusCode, rr.Code)

			// Check expectations
			mockUserService.AssertExpectations(t)
			mockAuthService.AssertExpectations(t)
		})
	}
}

// TestUserHandler_UpdateUser tests UpdateUser handler method
func TestUserHandler_UpdateUser(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		requestBody        string
		mockSetup          func(*MockUserService, *MockUserAuthService)
		expectedStatusCode int
	}{
		{
			name: "successful update - self",
			url:  "/api/users/1",
			requestBody: `{
				"first_name": "Updated",
				"last_name": "Name"
			}`,
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "testuser"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)

				existingUser := &models.User{
					ID:       1,
					Username: "testuser",
					Email:    "test@example.com",
					RoleID:   1,
				}
				userRepo.On("GetByID", 1).Return(existingUser, nil)
				userRepo.On("Update", mock.AnythingOfType("*models.User")).Return(nil)
				userRepo.On("GetRole", 1).Return(&models.Role{ID: 1, Name: "User"}, nil)
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "successful update - admin with permissions",
			url:  "/api/users/2",
			requestBody: `{
				"first_name": "Admin Updated",
				"is_active": false
			}`,
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserUpdate).Return(true, nil)
				authService.On("CheckPermission", 1, models.PermissionUserManage).Return(true, nil)
				authService.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil)

				existingUser := &models.User{
					ID:       2,
					Username: "otheruser",
					Email:    "other@example.com",
					RoleID:   1,
				}
				userRepo.On("GetByID", 2).Return(existingUser, nil)
				userRepo.On("Update", mock.AnythingOfType("*models.User")).Return(nil)
				userRepo.On("GetRole", 1).Return(&models.Role{ID: 1, Name: "User"}, nil)
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "unauthorized - no token",
			url:                "/api/users/1",
			requestBody:        `{"first_name": "Updated"}`,
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh mocks for each test
			mockUserService := new(MockUserService)
			mockAuthService := new(MockUserAuthService)

			// Setup mocks
			if tt.mockSetup != nil {
				tt.mockSetup(mockUserService, mockAuthService)
			}

			handler := NewUserHandler(mockUserService, mockAuthService)

			// Create request
			req := httptest.NewRequest(http.MethodPut, tt.url, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.name != "unauthorized - no token" {
				req.Header.Set("Authorization", "Bearer valid-token")
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.UpdateUser(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatusCode, rr.Code)

			// Check expectations
			mockUserService.AssertExpectations(t)
			mockAuthService.AssertExpectations(t)
		})
	}
}

// TestUserHandler_ListUsers tests ListUsers handler method
func TestUserHandler_ListUsers(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		mockSetup          func(*MockUserService, *MockUserAuthService)
		expectedStatusCode int
	}{
		{
			name: "successful list users",
			url:  "/api/users?limit=10&offset=0",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "admin"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserView).Return(true, nil)

				users := []models.User{
					{ID: 1, Username: "user1", RoleID: 1},
					{ID: 2, Username: "user2", RoleID: 1},
				}
				userRepo.On("List", 10, 0).Return(users, nil)
				userRepo.On("Count").Return(2, nil)
				userRepo.On("GetRole", 1).Return(&models.Role{ID: 1, Name: "User"}, nil)
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "unauthorized - no token",
			url:                "/api/users",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "insufficient permissions",
			url:  "/api/users",
			mockSetup: func(userRepo *MockUserService, authService *MockUserAuthService) {
				authUser := &models.User{ID: 1, Username: "testuser"}
				authService.On("GetCurrentUser", "valid-token").Return(authUser, nil)
				authService.On("CheckPermission", 1, models.PermissionUserView).Return(false, nil)
			},
			expectedStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh mocks for each test
			mockUserService := new(MockUserService)
			mockAuthService := new(MockUserAuthService)

			// Setup mocks
			if tt.mockSetup != nil {
				tt.mockSetup(mockUserService, mockAuthService)
			}

			handler := NewUserHandler(mockUserService, mockAuthService)

			// Create request
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if tt.name != "unauthorized - no token" {
				req.Header.Set("Authorization", "Bearer valid-token")
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.ListUsers(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatusCode, rr.Code)

			// Check expectations
			mockUserService.AssertExpectations(t)
			mockAuthService.AssertExpectations(t)
		})
	}
}
