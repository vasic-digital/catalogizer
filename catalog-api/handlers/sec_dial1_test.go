package handlers

import (
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// SEC-DIAL-1 (unit-class evidence, §11.4.289: the real handler + ValidateRoot + netguard run in-process against a real sqlite DB; no network
// is involved): an smb / ftp / nfs storage root aimed at a loopback, link-local, metadata or reserved host is refused at creation with a
// generic 400 and nothing stored; a private LAN NAS host is still created.
func secDialRoot(protocol, host, name string) map[string]interface{} {
	m := map[string]interface{}{"name": name, "protocol": protocol, "host": host}
	switch protocol {
	case "smb":
		m["path"] = "share"
	case "ftp":
		m["path"] = "/"
	case "nfs":
		m["path"] = "/export"
		m["mount_point"] = "/mnt/secdial-" + name
	}
	return m
}

var secDialBadHosts = []string{
	"127.0.0.1", "100.100.100.200", "168.63.129.16", "fd20:ce::254", "198.18.0.1", "169.254.169.254", "fe80::1", "0.0.0.0", "::1",
	"::ffff:169.254.169.254", "0177.0.0.1", "localhost",
}

func TestSECDIAL1_CreateStorageRootRefusesUnvettedHost(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := newAPIHarness(t, false)
	for _, proto := range []string{"smb", "ftp", "nfs"} {
		for i, host := range secDialBadHosts {
			code, body := h.do("POST", "/storage/roots", secDialRoot(proto, host, proto+"-bad-"+string(rune('a'+i))))
			require.Equal(t, 400, code, "%s %s: %v", proto, host, body)
			require.NotContains(t, toStr(body), host, "the refusal must not echo the host or its resolution")
		}
	}
	var n int
	require.NoError(t, h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	require.Equal(t, 0, n, "refused roots must not be stored")
}

func TestSECDIAL1_CreateStorageRootAllowsPrivateLANHost(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := newAPIHarness(t, false)
	for i, host := range []string{"192.168.1.20", "10.0.0.5", "172.16.3.4", "100.64.1.2", "fd12:3456::1"} {
		for _, proto := range []string{"smb", "ftp", "nfs"} {
			code, body := h.do("POST", "/storage/roots", secDialRoot(proto, host, proto+"-lan-"+string(rune('a'+i))))
			require.Equal(t, 201, code, "%s %s: %v", proto, host, body)
		}
	}
}

func TestSECDIAL1_CreateStorageRootRefusesMalformedHostAndNFSInjection(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := newAPIHarness(t, false)
	for i, host := range []string{"nas.lan,addr=127.0.0.1", "-o", "nas lan", "nas\nlan", "nas.lan:445", "user@nas.lan", "[::1]", "nas.lan/x"} {
		for _, proto := range []string{"smb", "ftp", "nfs"} {
			code, body := h.do("POST", "/storage/roots", secDialRoot(proto, host, proto+"-mal-"+string(rune('a'+i))))
			require.Equal(t, 400, code, "%s %q: %v", proto, host, body)
		}
	}
	for i, opt := range []string{"vers=3,addr=127.0.0.1", "vers=3,mountaddr=169.254.169.254", "vers=3\n,addr=1.2.3.4", "vers=3 addr=1", "-o vers=3", "vers=3,,x"} {
		r := secDialRoot("nfs", "192.168.1.20", "nfs-opt-"+string(rune('a'+i)))
		r["options"] = opt
		code, body := h.do("POST", "/storage/roots", r)
		require.Equal(t, 400, code, "options %q: %v", opt, body)
	}
	// An export path with a control character or a leading '-' is refused. A space or a ',' is a legal part of a share name
	// ("/volume1/My Videos"): ValidateNFSExport (SEC-DIAL-1-FU, b001ac4bf) lets them through because the export only ever forms the SOURCE of
	// mount(2), never an option string, so it cannot inject an option or another address. The pre-FU expectation (400) is stale.
	for i, exp := range []string{"/ex\nport", "-o"} {
		r := secDialRoot("nfs", "192.168.1.20", "nfs-exp-"+string(rune('a'+i)))
		r["path"] = exp
		code, body := h.do("POST", "/storage/roots", r)
		require.Equal(t, 400, code, "export %q: %v", exp, body)
	}
	for i, exp := range []string{"/export,addr=127.0.0.1", "/ex port"} {
		r := secDialRoot("nfs", "192.168.1.20", "nfs-exp-legal-"+string(rune('a'+i)))
		r["path"] = exp
		code, body := h.do("POST", "/storage/roots", r)
		require.Equal(t, 201, code, "export %q is a legal share name and must be accepted: %v", exp, body)
	}
	r := secDialRoot("nfs", "192.168.1.20", "nfs-good-opts")
	r["options"] = "vers=3,ro,nolock,port=2049"
	code, body := h.do("POST", "/storage/roots", r)
	require.Equal(t, 201, code, "plain options must still be accepted: %v", body)
}
