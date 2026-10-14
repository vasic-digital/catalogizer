package handlers

// WF22 round 3 (structural fix round): the API half of the class inventories.
//   class C  CreateStorageRoot validates the ROW (filesystem.ValidateRoot), not the vacuous output of the mapping (N3, review P5);
//   class B  no read endpoint returns a storage root's credential (N5, review P7).
// The database is opened through the PRODUCTION path (database.NewConnection + RunMigrations on a file), owner decision OD-23.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openProductionFileDB opens a SQLite FILE database exactly as main does (NewConnection) and applies the production migration chain.
func openProductionFileDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(t.TempDir(), "catalog.db"), EnableWAL: true})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.RunMigrations(context.Background()))
	return db
}

func TestWF22R3_ProductionDBHarnessHasTheMigratedSchema(t *testing.T) {
	db := openProductionFileDB(t)
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('storage_roots') WHERE name = 'allow_empty'`).Scan(&n))
	assert.Equal(t, 1, n, "the production migrations (v21) created storage_roots.allow_empty")
}

// N3 (review P5/P5b) + the create matrix.
func TestWF22R3_N3_CreateStorageRootValidatesTheRow(t *testing.T) {
	h := newAPIHarness(t, false)
	cases := []struct {
		name string
		body map[string]interface{}
		want int
	}{
		{"ftp with url + mount_point", map[string]interface{}{"name": "c1", "protocol": "ftp", "host": "nas", "path": "/media", "url": "https://user:pw@elsewhere/dav", "mount_point": "/mnt/x"}, 400},
		{"nfs empty", map[string]interface{}{"name": "c2", "protocol": "nfs"}, 400},
		{"smb without share", map[string]interface{}{"name": "c3", "protocol": "smb", "host": "nas"}, 400},
		{"webdav without url", map[string]interface{}{"name": "c4", "protocol": "webdav"}, 400},
		{"ftp host-key pin it cannot honour", map[string]interface{}{"name": "c5", "protocol": "ftp", "host": "nas", "options": `{"host_key_fingerprint":"SHA256:x"}`}, 400},
		{"ftp options not json", map[string]interface{}{"name": "c6", "protocol": "ftp", "host": "nas", "options": "vers=3"}, 400},
		{"port out of range", map[string]interface{}{"name": "c7", "protocol": "ftp", "host": "nas", "port": 70000}, 400},
		{"ftp ok", map[string]interface{}{"name": "ok-ftp", "protocol": "ftp", "host": "nas", "path": "/media", "username": "u", "password": "p"}, 201},
		{"nfs ok", map[string]interface{}{"name": "ok-nfs", "protocol": "nfs", "host": "nas", "path": "/export", "mount_point": "/mnt/nas", "options": "vers=4"}, 201},
		{"webdav ok", map[string]interface{}{"name": "ok-dav", "protocol": "webdav", "url": "https://nas/dav", "username": "u", "password": "p"}, 201},
		{"smb ok with identity", map[string]interface{}{"name": "ok-smb", "protocol": "smb", "host": "nas", "path": "media", "options": `{"identity_index": 1}`}, 201},
		{"local ok", map[string]interface{}{"name": "ok-local", "protocol": "local", "path": "/data"}, 201},
	}
	for _, c := range cases {
		code, body := h.do("POST", "/storage/roots", c.body)
		assert.Equal(t, c.want, code, "%s: %v", c.name, body)
		if c.want == 400 {
			_, stored := body["id"]
			assert.False(t, stored, "%s: a refused root is not stored", c.name)
		}
	}
	var n int
	require.NoError(t, h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	assert.Equal(t, 5, n, "exactly the five valid roots were stored")
}

// N5 (review P7): the browse endpoints do not return a credential, and neither does the scan handler's listing.
func TestWF22R3_N5_NoReadEndpointReturnsACredential(t *testing.T) {
	h := newAPIHarness(t, false)
	code, _ := h.do("POST", "/storage/roots", map[string]interface{}{
		"name": "r3-dav", "protocol": "webdav", "url": "https://alice:urlSecret77@nas/dav", "username": "bob", "password": "colSecret88",
	})
	require.Equal(t, http.StatusCreated, code)
	code, _ = h.do("POST", "/storage/roots", map[string]interface{}{
		"name": "r3-ftp", "protocol": "ftp", "host": "nas", "username": "u", "password": "ftpSecret55",
	})
	require.Equal(t, http.StatusCreated, code)

	bh := NewBrowseHandler(repository.NewFileRepository(h.db))
	h.r.GET("/browse/roots", bh.GetStorageRoots)
	h.r.GET("/browse", bh.GetStorageRoots)
	for _, ep := range []string{"/browse/roots", "/browse", "/storage/roots"} {
		req := httptest.NewRequest("GET", ep, nil)
		w := httptest.NewRecorder()
		h.r.ServeHTTP(w, req)
		body := w.Body.String()
		require.Equal(t, http.StatusOK, w.Code, ep)
		require.True(t, strings.Contains(body, "r3-dav") && strings.Contains(body, "r3-ftp"), "control needle: both roots are listed at %s: %s", ep, body)
		for _, secret := range []string{"colSecret88", "urlSecret77", "ftpSecret55"} {
			assert.NotContains(t, body, secret, "%s returns a storage-root credential", ep)
		}
		assert.Contains(t, body, "bob", "%s: control: the username is still shown", ep)
	}
}
