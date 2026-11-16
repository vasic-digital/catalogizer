package handlers

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// unownedPrivateHost returns an RFC 1918 address that no interface of THIS machine carries. The root-creation policy refuses the server's own
// addresses (SEC-DIAL-1), so a hard-coded LAN address passes on one machine and fails on the machine that happens to own it (ENV-F1: the
// developer host owns 192.168.1.115). Nothing is dialled by this test, only classified.
func unownedPrivateHost(t *testing.T) string {
	t.Helper()
	owned := map[string]bool{}
	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			owned[ipn.IP.String()] = true
		}
	}
	for _, c := range []string{"192.168.1.115", "10.254.254.1", "172.31.254.1", "192.168.254.1"} {
		if !owned[c] {
			return c
		}
	}
	t.Fatal("no candidate private address is free of this machine's interfaces")
	return ""
}

// CAT-SMB-SUBDIR-1: POST /storage/roots accepts an smb path of the form share/sub/dir and refuses a path that could leave the subtree.
func TestCATSMBSUBDIR1_CreateStorageRootSMBSubdir(t *testing.T) {
	h := newAPIHarness(t, false)
	host := unownedPrivateHost(t)
	base := func(name, path string) map[string]interface{} {
		return map[string]interface{}{"name": name, "protocol": "smb", "host": host, "path": path, "username": "u", "password": "p"}
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
