package handlers

// CAT-001 R4 sweep: GET download info for a directory reported estimated_archive_size = the directory entry's own size
// ("Placeholder"), a figure that is not an estimate of anything. It must be absent when it was not computed.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCAT001_DownloadInfo_DirectoryHasNoInventedArchiveSize(t *testing.T) {
	db, cleanup := setupMediaQueryTestDB(t)
	defer cleanup()
	_, err := db.Exec(`INSERT INTO storage_roots (name, protocol, path) VALUES ('dl-root', 'local', '/m')`)
	require.NoError(t, err)
	var rootID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM storage_roots WHERE name = 'dl-root'`).Scan(&rootID))
	_, err = db.Exec(`INSERT INTO files (storage_root_id, path, name, size, is_directory, modified_at) VALUES (?, '/m/dir', 'dir', 4096, 1, datetime('now'))`, rootID)
	require.NoError(t, err)
	var fileID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM files WHERE name = 'dir'`).Scan(&fileID))

	h := NewDownloadHandler(repository.NewFileRepository(db), t.TempDir(), 1<<30, 1024)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/info/:id", h.GetDownloadInfo)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/info/"+strconv.FormatInt(fileID, 10), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body.Data["is_directory"])
	_, present := body.Data["estimated_archive_size"]
	assert.False(t, present, "no archive estimate was computed, so none may be reported: %v", body.Data)
}
