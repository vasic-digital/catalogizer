package handlers

import (
	"testing"

	"catalogizer/filesystem"
	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// SEC-DIAL-1 for nfs3: POST /storage/roots refuses a metadata / loopback host, a malformed host and a '-'-leading export with 400 and stores
// nothing; a LAN host is created.
func TestNFS3WIRE1_CreateStorageRootRefusesUnsafeHosts(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	require.NoError(t, filesystem.RegisterNFS3Protocol())
	defer filesystem.UnregisterProtocol("nfs3")
	h := newAPIHarness(t, false)
	for _, c := range []struct{ host, path string }{
		{"169.254.169.254", "/export"}, {"127.0.0.1", "/export"}, {"evil:1234", "/export"}, {"nas.example", "-o x"},
	} {
		code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "nfs3-" + c.host + c.path, "protocol": "nfs3", "host": c.host, "path": c.path})
		require.Equal(t, 400, code, "%+v: %v", c, body)
	}
	var n int
	require.NoError(t, h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	require.Equal(t, 0, n, "refused roots must not be stored")
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lan-nfs3", "protocol": "nfs3", "host": "192.168.1.20", "path": "/volume1/video"})
	require.Equal(t, 201, code, "LAN nfs3 root must be created: %v", body)
}
