package handlers

// CAT-001: the admin backup endpoint must produce an ENCRYPTED, restorable
// backup of the real (SQLCipher) database - not a raw plaintext file copy.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminHandler_CreateBackup_EncryptedAndRestorable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	work := t.TempDir()
	t.Chdir(work) // defaultBackupDir is ./backups

	dbPath := filepath.Join(work, "live.db")
	db, err := database.NewConnection(&config.DatabaseConfig{
		Path:              dbPath,
		EncryptionKeyFile: filepath.Join(work, "keys", "db.key"),
	})
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE bk_t (id INTEGER PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		_, err = db.Exec(`INSERT INTO bk_t (v) VALUES ('BACKUPMARKER')`)
		require.NoError(t, err)
	}

	auth := &mockAdminAuthService{
		getCurrentUserFn:  func(string) (*models.User, error) { return &models.User{ID: 1}, nil },
		checkPermissionFn: func(int, string) (bool, error) { return true, nil },
	}
	h := NewAdminHandler(auth, &mockAdminUserRepo{}, db, "1.0.0-test")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/admin/backups", strings.NewReader(`{"type":"full"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer token")
	h.CreateBackup(c)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	files, err := filepath.Glob(filepath.Join(work, "backups", "backup_*.db"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	raw, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(string(raw), "SQLite format 3\x00"), "backup must be encrypted")
	assert.NotContains(t, string(raw), "BACKUPMARKER")
	st, _ := os.Stat(files[0])
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	// restore into a new location and read it back with the same key file
	target := filepath.Join(work, "restored.db")
	require.NoError(t, database.RestoreBackup(files[0], target))
	rdb, err := database.NewConnection(&config.DatabaseConfig{
		Path:              target,
		EncryptionKeyFile: filepath.Join(work, "keys", "db.key"),
	})
	require.NoError(t, err)
	defer rdb.Close()
	var n int
	require.NoError(t, rdb.QueryRow(`SELECT COUNT(*) FROM bk_t`).Scan(&n))
	assert.Equal(t, 10, n)
}
