package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock implementations for AdminHandler dependencies ---

type mockAdminAuthService struct {
	checkPermissionFn func(userID int, permission string) (bool, error)
	getCurrentUserFn  func(token string) (*models.User, error)
}

func (m *mockAdminAuthService) CheckPermission(userID int, permission string) (bool, error) {
	if m.checkPermissionFn != nil {
		return m.checkPermissionFn(userID, permission)
	}
	return true, nil
}

func (m *mockAdminAuthService) GetCurrentUser(token string) (*models.User, error) {
	if m.getCurrentUserFn != nil {
		return m.getCurrentUserFn(token)
	}
	return &models.User{ID: 1, Username: "admin"}, nil
}

type mockAdminUserRepo struct {
	listFn    func(limit, offset int) ([]models.User, error)
	getByIDFn func(id int) (*models.User, error)
	updateFn  func(user *models.User) error
	countFn   func() (int, error)
	revoked   []int
	revokeErr error
}

func (m *mockAdminUserRepo) List(limit, offset int) ([]models.User, error) {
	if m.listFn != nil {
		return m.listFn(limit, offset)
	}
	return nil, nil
}

func (m *mockAdminUserRepo) GetByID(id int) (*models.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(id)
	}
	return nil, errors.New("not found")
}

func (m *mockAdminUserRepo) Update(user *models.User) error {
	if m.updateFn != nil {
		return m.updateFn(user)
	}
	return nil
}

func (m *mockAdminUserRepo) Count() (int, error) {
	if m.countFn != nil {
		return m.countFn()
	}
	return 0, nil
}

// DeactivateAllUserSessions records the revocation (the handlers require it).
func (m *mockAdminUserRepo) DeactivateAllUserSessions(userID int) error {
	m.revoked = append(m.revoked, userID)
	return m.revokeErr
}

func newTestAdminHandler(auth AdminAuthServiceInterface, userRepo AdminUserRepoInterface) *AdminHandler {
	return NewAdminHandler(auth, userRepo, nil, "1.0.0-test")
}

// --- Tests ---

func TestAdminHandler_requireAdmin_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{}
	repo := &mockAdminUserRepo{}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/system-info", nil)

	user := h.requireAdmin(c)
	assert.Nil(t, user)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminHandler_requireAdmin_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return nil, errors.New("invalid token")
		},
	}
	repo := &mockAdminUserRepo{}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/system-info", nil)
	c.Request.Header.Set("Authorization", "Bearer bad-token")

	user := h.requireAdmin(c)
	assert.Nil(t, user)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminHandler_requireAdmin_NoPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1, Username: "user"}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return false, nil
		},
	}
	repo := &mockAdminUserRepo{}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/system-info", nil)
	c.Request.Header.Set("Authorization", "Bearer good-token")

	user := h.requireAdmin(c)
	assert.Nil(t, user)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminHandler_requireAdmin_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1, Username: "admin"}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/system-info", nil)
	c.Request.Header.Set("Authorization", "Bearer good-token")

	user := h.requireAdmin(c)
	require.NotNil(t, user)
	assert.Equal(t, "admin", user.Username)
}

func TestAdminHandler_requireAdmin_BearerPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			assert.Equal(t, "my-token", token)
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer my-token")

	user := h.requireAdmin(c)
	require.NotNil(t, user)
}

func TestAdminHandler_GetSystemInfo_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/system-info", nil)
	c.Request.Header.Set("Authorization", "Bearer token")

	h.GetSystemInfo(c)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0-test", resp["version"])
	assert.NotNil(t, resp["uptime"])
	assert.NotNil(t, resp["memoryUsage"])
	assert.NotNil(t, resp["activeConnections"])
}

func TestAdminHandler_GetSystemInfo_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return nil, errors.New("invalid")
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/system-info", nil)
	c.Request.Header.Set("Authorization", "Bearer bad")

	h.GetSystemInfo(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminHandler_GetUsers_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	roleName := "admin"
	repo := &mockAdminUserRepo{
		listFn: func(limit, offset int) ([]models.User, error) {
			return []models.User{
				{ID: 1, Username: "alice", Email: "alice@test.com", IsActive: true, Role: &models.Role{Name: roleName}},
				{ID: 2, Username: "bob", Email: "bob@test.com", IsActive: false, IsLocked: true},
			}, nil
		},
	}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/users", nil)
	c.Request.Header.Set("Authorization", "Bearer token")

	h.GetUsers(c)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "alice", result[0]["username"])
	assert.Equal(t, "admin", result[0]["role"])
	assert.Equal(t, "active", result[0]["status"])
	assert.Equal(t, "suspended", result[1]["status"])
}

func TestAdminHandler_GetUsers_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{
		listFn: func(limit, offset int) ([]models.User, error) {
			return nil, errors.New("db error")
		},
	}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/users", nil)
	c.Request.Header.Set("Authorization", "Bearer token")

	h.GetUsers(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminHandler_UpdateUser_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{
		getByIDFn: func(id int) (*models.User, error) {
			return &models.User{ID: id, Username: "alice", Email: "alice@test.com"}, nil
		},
		updateFn: func(user *models.User) error { return nil },
	}
	h := newTestAdminHandler(auth, repo)

	body := `{"status":"active","email":"newemail@test.com"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/admin/users/5", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Params = gin.Params{{Key: "id", Value: "5"}}

	h.UpdateUser(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminHandler_UpdateUser_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/admin/users/abc", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	h.UpdateUser(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_UpdateUser_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{
		getByIDFn: func(id int) (*models.User, error) {
			return nil, errors.New("user not found")
		},
	}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/admin/users/99", strings.NewReader(`{"status":"active"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Params = gin.Params{{Key: "id", Value: "99"}}

	h.UpdateUser(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminHandler_UpdateUser_UpdateError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{
		getByIDFn: func(id int) (*models.User, error) {
			return &models.User{ID: id, Username: "alice"}, nil
		},
		updateFn: func(user *models.User) error {
			return errors.New("update failed")
		},
	}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/admin/users/5", strings.NewReader(`{"status":"inactive"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Params = gin.Params{{Key: "id", Value: "5"}}

	h.UpdateUser(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminHandler_UpdateUser_StatusVariants(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}

	for _, status := range []string{"active", "inactive", "suspended"} {
		t.Run("status_"+status, func(t *testing.T) {
			var updatedUser *models.User
			repo := &mockAdminUserRepo{
				getByIDFn: func(id int) (*models.User, error) {
					return &models.User{ID: id, Username: "test"}, nil
				},
				updateFn: func(user *models.User) error {
					updatedUser = user
					return nil
				},
			}
			h := newTestAdminHandler(auth, repo)

			body := `{"status":"` + status + `"}`
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("PUT", "/admin/users/1", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Authorization", "Bearer token")
			c.Params = gin.Params{{Key: "id", Value: "1"}}

			h.UpdateUser(c)
			assert.Equal(t, http.StatusOK, w.Code)
			require.NotNil(t, updatedUser)

			switch status {
			case "active":
				assert.True(t, updatedUser.IsActive)
				assert.False(t, updatedUser.IsLocked)
			case "inactive":
				assert.False(t, updatedUser.IsActive)
			case "suspended":
				assert.True(t, updatedUser.IsLocked)
			}
		})
	}
}

func TestAdminHandler_GetBackups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/backups", nil)
	c.Request.Header.Set("Authorization", "Bearer token")

	h.GetBackups(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminHandler_CreateBackup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/admin/backups", strings.NewReader(`{"type":"full"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")

	h.CreateBackup(c)
	// With nil db, the handler returns 500 "Database not configured"
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "Database not configured")
}

func TestAdminHandler_CreateBackup_BadBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/admin/backups", strings.NewReader("not json"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")

	h.CreateBackup(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_RestoreBackup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/admin/backups/42/restore", nil)
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	h.RestoreBackup(c)
	// Backup file "42" does not exist, so the handler returns 404.
	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "not found")
}

func TestAdminHandler_ScanStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/admin/storage/scan", strings.NewReader(`{"path":"/media"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")

	h.ScanStorage(c)
	// CAT-001: this endpoint starts no scan; it must say so (501) instead of 202 "Scan initiated".
	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEqual(t, "accepted", resp["status"])
	assert.Contains(t, resp["error"], "POST /api/v1/scans")
}

func TestAdminHandler_ScanStorage_BadBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	h := newTestAdminHandler(auth, &mockAdminUserRepo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/admin/storage/scan", strings.NewReader("invalid"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")

	h.ScanStorage(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_UpdateUser_BadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{
		getByIDFn: func(id int) (*models.User, error) {
			return &models.User{ID: id, Username: "test"}, nil
		},
	}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/admin/users/1", strings.NewReader("not json"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.UpdateUser(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_UpdateUser_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{
		getByIDFn: func(id int) (*models.User, error) {
			return nil, errors.New("database connection failed")
		},
	}
	h := newTestAdminHandler(auth, repo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/admin/users/1", strings.NewReader(`{"status":"active"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.UpdateUser(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
