package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SSE-OPQ-1 (operator decision 2026-11-14): GET /storage/roots and /storage-roots are admin-only; /browse/roots and /browse stay
// open to any authenticated user and carry no host/path/username/domain. Real binary, real router, real SQLCipher DB, real register+login.
func TestRoutePerms_RealBinary_SSEOPQ1_StorageRootsAdminOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	media := t.TempDir()
	c, adm, stop := startPermServer(t, media)
	defer stop()
	plain := selfRegister(t, c, "opqplain")

	code, raw := c.do("POST", "/api/v1/storage/roots", adm, map[string]interface{}{"name": "opqroot", "protocol": "local", "path": media})
	require.Equal(t, 201, code, "%s", raw)

	for _, p := range []string{"/api/v1/storage/roots", "/api/v1/storage-roots"} {
		code, _ = c.do("GET", p, "", nil)
		assert.Equal(t, 401, code, "anonymous GET "+p)
		code, raw = c.do("GET", p, adm, nil)
		require.Equal(t, 200, code, "admin GET %s: %s", p, raw)
		require.Contains(t, string(raw), media, "control needle: admin sees the root path at %s", p)
		code, raw = c.do("GET", p, plain, nil)
		assert.Equal(t, 403, code, "non-admin GET "+p)
		assert.NotContains(t, string(raw), media)
	}
	for _, p := range []string{"/api/v1/browse/roots", "/api/v1/browse"} {
		code, raw = c.do("GET", p, plain, nil)
		require.Equal(t, 200, code, "non-admin GET %s: %s", p, raw)
		require.Contains(t, string(raw), "opqroot", "control needle: the root is listed at %s", p)
		assert.NotContains(t, string(raw), media, "%s leaks the root path", p)
		assert.NotContains(t, string(raw), `"path"`)
	}
}
