package handlers

// CAT-001: GET /storage-roots/:id/status must PROBE the storage root through the real client factory and the real protocol client
// (here: the real WebDAV client against a real HTTP server, a real SQLite database with the production migrations). The former handler
// answered {connected:true,status:online} for any row that existed ("For now, return dummy connectivity status").

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statusHarness(t *testing.T) *apiHarness {
	t.Helper()
	h := newAPIHarness(t, false)
	h.r.GET("/storage-roots/:id/status", h.handle.GetStorageRootStatus)
	return h
}

func createDavRoot(t *testing.T, h *apiHarness, name, url, user, pass string) int64 {
	t.Helper()
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": name, "protocol": "webdav", "url": url, "username": user, "password": pass, "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	return int64(body["id"].(float64))
}

func statusURL(id int64) string { return "/storage-roots/" + strconv.FormatInt(id, 10) + "/status" }

func TestCAT001_StorageRootStatus_ReachableRootIsOnline(t *testing.T) {
	srv := davServer(t, "alice", "S3cr3t-Pw", map[string][]string{"/": {"a.mkv"}})
	h := statusHarness(t)
	id := createDavRoot(t, h, "up", srv.URL, "alice", "S3cr3t-Pw")
	code, body := h.do("GET", statusURL(id), nil)
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, true, body["connected"])
	assert.Equal(t, "online", body["status"])
}

func TestCAT001_StorageRootStatus_WrongCredentialsIsOfflineWithReasonAndNoSecretLeak(t *testing.T) {
	srv := davServer(t, "alice", "S3cr3t-Pw", map[string][]string{"/": {"a.mkv"}})
	h := statusHarness(t)
	id := createDavRoot(t, h, "badcred", srv.URL, "alice", "Wr0ng-Passw0rd")
	code, body := h.do("GET", statusURL(id), nil)
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, false, body["connected"], "a root the server rejects must not be reported connected: %v", body)
	assert.Equal(t, "offline", body["status"])
	reason, _ := body["error"].(string)
	assert.NotEmpty(t, reason, "an offline root states why")
	assert.False(t, strings.Contains(reason, "Wr0ng-Passw0rd"), "the stored password must never be echoed")
}

func TestCAT001_StorageRootStatus_UnreachableHostIsOffline(t *testing.T) {
	srv := davServer(t, "alice", "pw", map[string][]string{"/": {}})
	url := srv.URL
	srv.Close() // nothing listens any more
	h := statusHarness(t)
	id := createDavRoot(t, h, "gone", url, "alice", "pw")
	code, body := h.do("GET", statusURL(id), nil)
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, false, body["connected"])
	assert.Equal(t, "offline", body["status"])
}

func TestCAT001_StorageRootStatus_DisabledRootIsNotProbedNorOnline(t *testing.T) {
	srv := davServer(t, "alice", "pw", map[string][]string{"/": {}})
	h := statusHarness(t)
	id := createDavRoot(t, h, "off", srv.URL, "alice", "pw")
	_, err := h.db.Exec("UPDATE storage_roots SET enabled = 0 WHERE id = ?", id)
	require.NoError(t, err)
	code, body := h.do("GET", statusURL(id), nil)
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, false, body["connected"])
	assert.Equal(t, "disabled", body["status"])
}

func TestCAT001_StorageRootStatus_UnknownRootIs404(t *testing.T) {
	h := statusHarness(t)
	code, _ := h.do("GET", statusURL(9999), nil)
	assert.Equal(t, http.StatusNotFound, code)
}

var _ = gin.TestMode
