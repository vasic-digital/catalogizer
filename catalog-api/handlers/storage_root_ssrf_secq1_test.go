package handlers

import (
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

func TestSECQ1_StorageRootURLEarlyCheck(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	s := func(v string) *string { return &v }
	for _, u := range []string{"http://127.0.0.1:5005/", "http://169.254.169.254/", "https://[fe80::1]/dav", "ftp://192.168.1.2/", "http://user:pw@169.254.169.254/"} {
		require.Error(t, validateWebDAVRootURL(s(u)), u)
	}
	for _, u := range []string{"http://192.168.1.20:5005/", "https://webdav.yandex.ru/", "http://user:pw@192.168.1.20/dav"} {
		require.NoError(t, validateWebDAVRootURL(s(u)), u)
	}
	require.Error(t, validateWebDAVRootURL(nil))
}

// F4: the request path itself. POST /storage/roots with a metadata URL is refused with 400 and nothing is stored; a LAN URL is created.
func TestSECQ1_F4_CreateStorageRootRefusesMetadataURL(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := newAPIHarness(t, false)
	for _, u := range []string{"http://169.254.169.254/", "http://127.0.0.1:5005/dav", "http://[::ffff:169.254.169.254]/", "http://0177.0.0.1/"} {
		code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "ssrf-" + u, "protocol": "webdav", "url": u, "path": "/"})
		require.Equal(t, 400, code, "%s: %v", u, body)
	}
	var n int
	require.NoError(t, h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	require.Equal(t, 0, n, "refused roots must not be stored")
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lan-nas", "protocol": "webdav", "url": "http://192.168.1.20:5005/", "path": "/"})
	require.Equal(t, 201, code, "LAN NAS root must still be created: %v", body)
}
