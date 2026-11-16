package handlers

import (
	"context"
	"fmt"
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

// NFS3-WIRE-1-m1: with the classifier held at "public" (so the approval classifier cannot refuse) and a syntactically valid root, only the
// VetStorageRoot leg of CreateStorageRoot can refuse a metadata-address nfs3 host. Dropping nfs3 from that gate must make this fail.
func TestNFS3WIRE1_OnlyVetStorageRootLegRefusesMetadataHost(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	require.NoError(t, filesystem.RegisterNFS3Protocol())
	defer filesystem.UnregisterProtocol("nfs3")
	was := classifyDestination
	classifyDestination = func(ctx context.Context, host, rawURL string) (netguard.Class, error) {
		return netguard.ClassPublic, nil
	}
	defer func() { classifyDestination = was }()
	h := newAPIHarness(t, false)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "nfs3-meta-only-vet", "protocol": "nfs3", "host": "169.254.169.254", "path": "/export"})
	require.Equal(t, 400, code, "only the VetStorageRoot leg can refuse here: %v", body)
	require.Contains(t, fmt.Sprint(body["error"]), "storage host is not allowed", "the refusal must come from the host policy: %v", body)
	var n int
	require.NoError(t, h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	require.Equal(t, 0, n)
}
