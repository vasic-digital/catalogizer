package handlers

// WF22 independent-review probes. Polarity: assert the correct behaviour; FAIL = defect present.
// ADAPTATION NOTE (WF22 fix round 2): this file is the reviewer's probe adopted as a permanent test. The verbatim original is kept in
// specs/001-full-project-audit-remediation/evidence/wp12/scanner/fix-r2-reviewer-probes-verbatim/ with its RED log. Differences from the
// original are listed here and nowhere else:
//   - H2: "sftp" is only creatable when a protocol of that name is registered (the allow-list IS the factory's SupportedProtocols, WF22 H2);
//     the probe registers a test "sftp" protocol for its duration and adds the control that an unregistered name is still refused.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/tests"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// H1: the ONLY production path that queues a scan (POST /api/v1/scans -> loadStorageRoot) never reads url / mount_point, so a WebDAV root
// reaches SettingsFromRoot without its URL (and an NFS root without its mount point): the newly implemented WebDAV/NFS scanning is
// unreachable for an end user.
func TestReviewProbe_H1_LoadStorageRootDropsURLAndMountPoint(t *testing.T) {
	db := &database.DB{DB: tests.SetupTestDB(t)}
	defer db.Close()
	_, err := db.Exec(`INSERT INTO storage_roots (name, protocol, host, url, mount_point, path) VALUES ('dav','webdav','nas','http://nas:5005/dav','/mnt/x','/movies')`)
	require.NoError(t, err)
	var id int64
	require.NoError(t, db.QueryRow(`SELECT id FROM storage_roots WHERE name='dav'`).Scan(&id))
	h := NewScanHandler(nil, db)
	root, err := h.loadStorageRoot(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, root.Host, "control: the loader reads the row (host is loaded)")
	settings := filesystem.SettingsFromRoot(root, nil)
	t.Logf("H1 loaded root url=%v mount_point=%v -> settings keys %v", root.URL, root.MountPoint, keys(settings))
	assert.NotNil(t, root.URL, "the WebDAV URL stored in the row must reach the scanner")
	assert.NotNil(t, root.MountPoint, "the NFS mount point stored in the row must reach the scanner")
	_, hasURL := settings["url"]
	assert.True(t, hasURL)
}

func keys(m map[string]interface{}) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// H2: POST /storage/roots cannot store a WebDAV URL (no field) and refuses every registered protocol (sftp/ftps/nfs3), so the "pluggable
// protocols are scanned with no further code" claim has no user path.
func TestReviewProbe_H2_CreateStorageRootCannotCreateWebDAVOrPluggable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &database.DB{DB: tests.SetupTestDB(t)}
	defer db.Close()
	h := NewScanHandler(nil, db)
	r := gin.New()
	r.POST("/r", h.CreateStorageRoot)
	post := func(body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/r", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, http.StatusCreated, post(`{"name":"d1","protocol":"webdav","url":"http://nas:5005/dav","path":"/movies"}`), "control: webdav accepted")
	var url *string
	require.NoError(t, db.QueryRow(`SELECT url FROM storage_roots WHERE name='d1'`).Scan(&url))
	t.Logf("H2 stored url=%v", url)
	assert.NotNil(t, url, "the URL a WebDAV root needs must be stored")
	require.Equal(t, http.StatusBadRequest, post(`{"name":"g0","protocol":"gopher","host":"nas"}`), "control: a protocol nobody registered is still refused")
	require.NoError(t, filesystem.RegisterProtocol(filesystem.ProtocolSpec{
		Name: "sftp",
		Keys: map[string]filesystem.KeyType{filesystem.KeyHost: filesystem.KeyString, filesystem.KeyPath: filesystem.KeyString},
		New:  func(map[string]interface{}) (filesystem.FileSystemClient, error) { return nil, nil },
	}))
	t.Cleanup(func() { filesystem.UnregisterProtocol("sftp") })
	code := post(`{"name":"s1","protocol":"sftp","host":"nas","path":"/x"}`)
	t.Logf("H2 sftp create -> HTTP %d", code)
	assert.Equal(t, http.StatusCreated, code, "a registered protocol must be creatable if the scanner advertises it")
}
