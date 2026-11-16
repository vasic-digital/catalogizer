package handlers

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// firstUnownedAddr returns the first candidate that none of owned carries. The root-creation policy refuses the server's own addresses
// (SEC-DIAL-1), so a hard-coded LAN address passes on one machine and fails on the machine that owns it (ENV-F1: the developer host owns
// 192.168.1.115). Addresses are compared by value (IP.Equal), for *net.IPNet and *net.IPAddr alike, as the product's ownAddr does.
func firstUnownedAddr(owned []net.Addr, candidates []string) (string, bool) {
	var ips []net.IP
	for _, a := range owned {
		switch v := a.(type) {
		case *net.IPNet:
			ips = append(ips, v.IP)
		case *net.IPAddr:
			ips = append(ips, v.IP)
		}
	}
	for _, c := range candidates {
		cip := net.ParseIP(c)
		if cip == nil {
			continue
		}
		free := true
		for _, ip := range ips {
			if ip.Equal(cip) {
				free = false
				break
			}
		}
		if free {
			return c, true
		}
	}
	return "", false
}

// unownedHost picks, from candidates of one address class, one that no interface of THIS machine carries. Nothing is dialled.
func unownedHost(t *testing.T, candidates ...string) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	h, ok := firstUnownedAddr(addrs, candidates)
	require.True(t, ok, "every candidate %v is carried by this machine's interfaces", candidates)
	return h
}

func unownedPrivateHost(t *testing.T) string {
	t.Helper()
	return unownedHost(t, "192.168.1.115", "10.254.254.1", "172.31.254.1", "192.168.254.1")
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
