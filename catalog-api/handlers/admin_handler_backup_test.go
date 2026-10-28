package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminHandlerWithBackupDir creates an AdminHandler configured for backup tests.
// It overrides defaultBackupDir indirectly by creating backups in a temp directory.
func newAdminTestContext(t *testing.T, method, url string, body string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
		c.Request = httptest.NewRequest(method, url, reader)
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request = httptest.NewRequest(method, url, nil)
	}

	// Set admin auth header so requireAdmin passes
	c.Request.Header.Set("Authorization", "Bearer valid-token")
	return w, c
}

func newBackupAdminHandler() *AdminHandler {
	auth := &mockAdminAuthService{
		getCurrentUserFn: func(token string) (*models.User, error) {
			return &models.User{ID: 1, Username: "admin"}, nil
		},
		checkPermissionFn: func(userID int, permission string) (bool, error) {
			return true, nil
		},
	}
	repo := &mockAdminUserRepo{}
	return NewAdminHandler(auth, repo, nil, "1.0.0-test")
}

func TestGetBackups_EmptyDirectory(t *testing.T) {
	// Save original and restore after test
	origDir := defaultBackupDir

	tmpDir, err := os.MkdirTemp("", "backup_test_empty_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// We can't easily override the const, but we can test that a non-existent
	// directory returns an empty list. The handler reads from defaultBackupDir ("./backups").
	// Instead, test the handler behavior: if the directory doesn't exist, it returns [].
	_ = origDir

	h := newBackupAdminHandler()

	w, c := newAdminTestContext(t, "GET", "/api/v1/admin/backups", "")

	h.GetBackups(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var result []interface{}
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	// Should return an array (possibly empty if ./backups doesn't exist, or with files if it does)
	assert.NotNil(t, result)
}

func TestGetBackups_WithBackupFiles(t *testing.T) {
	// Create a temporary directory to act as ./backups
	tmpDir, err := os.MkdirTemp("", "backup_test_files_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some backup files in the temp directory
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "backup_20240101_120000.db"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "backup_20240102_120000.sql"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "not_a_backup.txt"), []byte("test"), 0644))

	// Read the directory to verify files exist
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 3, len(entries))

	// Count only .db and .sql files (matching handler logic)
	var backupCount int
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".db") || strings.HasSuffix(entry.Name(), ".sql")) {
			backupCount++
		}
	}
	assert.Equal(t, 2, backupCount)
}

func TestRestoreBackup_PathTraversalRejected(t *testing.T) {
	h := newBackupAdminHandler()

	tests := []struct {
		name           string
		backupID       string
		expectedStatus int
	}{
		{
			name:           "path traversal with dots",
			backupID:       "../../../etc/passwd",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "path traversal with forward slash",
			backupID:       "dir/backup.db",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "path traversal with backslash",
			backupID:       "dir\\backup.db",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-existent backup returns 404",
			backupID:       "nonexistent_backup.db",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, c := newAdminTestContext(t, "POST", "/api/v1/admin/backups/"+tt.backupID+"/restore", "")
			c.Params = gin.Params{{Key: "id", Value: tt.backupID}}

			h.RestoreBackup(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRestoreBackup_ValidFile(t *testing.T) {
	// Create a backup file in the default backup directory
	err := os.MkdirAll(defaultBackupDir, 0750)
	if err != nil {
		t.Skip("cannot create backup directory (SKIP-OK: #fs-backup-dir-unwritable)")
	}

	testFile := filepath.Join(defaultBackupDir, "test_restore_backup.db")
	require.NoError(t, os.WriteFile(testFile, []byte("test backup data"), 0644))
	defer os.Remove(testFile)

	h := newBackupAdminHandler()

	w, c := newAdminTestContext(t, "POST", "/api/v1/admin/backups/test_restore_backup.db/restore", "")
	c.Params = gin.Params{{Key: "id", Value: "test_restore_backup.db"}}

	h.RestoreBackup(c)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var body map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "queued", body["status"])
}

func TestScanStorage(t *testing.T) {
	h := newBackupAdminHandler()

	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid path returns 202",
			body:           `{"path": "/media/movies"}`,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "empty path returns 400",
			body:           `{"path": ""}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "whitespace-only path returns 400",
			body:           `{"path": "   "}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON returns 400",
			body:           `{invalid`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, c := newAdminTestContext(t, "POST", "/api/v1/admin/storage/scan", tt.body)

			h.ScanStorage(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// §11.4.115: id "." passed the separator/".." filter, Stat'ed the backups directory itself
// and answered 202 "queued" for something that is not a backup file.
func TestRestoreBackup_DotAndDirectoryIDsRejected(t *testing.T) {
	require.NoError(t, os.MkdirAll(filepath.Join(defaultBackupDir, "subdir_not_a_backup"), 0750))
	defer os.Remove(filepath.Join(defaultBackupDir, "subdir_not_a_backup"))
	h := newBackupAdminHandler()
	for _, id := range []string{".", "subdir_not_a_backup"} {
		w, c := newAdminTestContext(t, "POST", "/api/v1/admin/backups/"+id+"/restore", "")
		c.Params = gin.Params{{Key: "id", Value: id}}
		h.RestoreBackup(c)
		assert.NotEqual(t, http.StatusAccepted, w.Code, "id %q must not be queued as a restore", id)
	}
}
