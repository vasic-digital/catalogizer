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

// A connection error quotes the URL it dialled; a password carried in the URL userinfo must be scrubbed from the reported reason.
func TestCAT001_StorageRootStatus_UnreachableUrlWithUserinfoDoesNotLeakPassword(t *testing.T) {
	srv := davServer(t, "alice", "pw", map[string][]string{"/": {}})
	hostport := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	h := statusHarness(t)
	id := createDavRoot(t, h, "leaky", "http://alice:UserinfoPw-77@"+hostport, "", "")
	code, body := h.do("GET", statusURL(id), nil)
	require.Equal(t, http.StatusOK, code, "%v", body)
	assert.Equal(t, "offline", body["status"])
	reason, _ := body["error"].(string)
	assert.NotEmpty(t, reason)
	assert.False(t, strings.Contains(reason, "UserinfoPw-77"), "reason leaks the URL password: %s", reason)
}

// Every protocol whose server this test can stand up or refuse without privileges goes through the single settings mapping and the real
// client: a root pointing at a closed port / a missing directory must come back offline (never online, never a 5xx). NFS is not exercised
// here: its Connect performs a real mount(2), which needs privileges and a server (honest gap, tracked in the CAT-001 report).
func TestCAT001_StorageRootStatus_SettingsGoThroughTheSingleMapping_AllProtocols(t *testing.T) {
	srv := davServer(t, "u", "p", map[string][]string{"/": {}})
	hostport := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	host, port, _ := strings.Cut(hostport, ":")
	portN, _ := strconv.Atoi(port)
	h := statusHarness(t)
	roots := []map[string]interface{}{
		{"name": "p-smb", "protocol": "smb", "host": host, "port": portN, "path": "share", "username": "u", "password": "p"},
		{"name": "p-ftp", "protocol": "ftp", "host": host, "port": portN, "path": "/", "username": "u", "password": "p"},
		{"name": "p-dav", "protocol": "webdav", "url": "http://" + hostport, "username": "u", "password": "p", "path": "/"},
		{"name": "p-local", "protocol": "local", "path": t.TempDir() + "/does-not-exist"},
	}
	for _, r := range roots {
		code, body := h.do("POST", "/storage/roots", r)
		require.Equal(t, http.StatusCreated, code, "%v: %v", r["name"], body)
		id := int64(body["id"].(float64))
		code, st := h.do("GET", statusURL(id), nil)
		require.Equal(t, http.StatusOK, code, "%v: %v", r["name"], st)
		assert.Equal(t, false, st["connected"], "%v must not be reported connected: %v", r["name"], st)
		assert.Contains(t, []interface{}{"offline", "misconfigured"}, st["status"], "%v: %v", r["name"], st)
	}
	// and the positive local case: an existing directory is online
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "p-local-ok", "protocol": "local", "path": t.TempDir()})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	code, st := h.do("GET", statusURL(int64(body["id"].(float64))), nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "online", st["status"], "%v", st)
}
