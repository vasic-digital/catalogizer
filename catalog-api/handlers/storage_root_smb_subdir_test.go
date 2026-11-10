package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// CAT-SMB-SUBDIR-1: POST /storage/roots accepts an smb path of the form share/sub/dir and refuses a path that could leave the subtree.
func TestCATSMBSUBDIR1_CreateStorageRootSMBSubdir(t *testing.T) {
	h := newAPIHarness(t, false)
	base := func(name, path string) map[string]interface{} {
		return map[string]interface{}{"name": name, "protocol": "smb", "host": "192.168.1.115", "path": path, "username": "u", "password": "p"}
	}
	for i, bad := range []string{"Public/../Music", "Public/..", `Public\Testing`, "Public//x", "Public/./x", "/Public/Testing"} {
		code, body := h.do("POST", "/storage/roots", base("bad-"+string(rune('a'+i)), bad))
		require.Equal(t, 400, code, "%q: %v", bad, body)
	}
	var n int
	require.NoError(t, h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	require.Equal(t, 0, n, "refused roots must not be stored")
	for i, good := range []string{"Public", "Public/Testing", "Public/a/b"} {
		code, body := h.do("POST", "/storage/roots", base("ok-"+string(rune('a'+i)), good))
		require.Equal(t, 201, code, "%q: %v", good, body)
	}
}
