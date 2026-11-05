package handlers

import (
	"testing"

	"catalogizer/filesystem"
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

// CAT-S3-1 F7: an s3 root is an operator-chosen destination like a WebDAV one; the metadata / loopback URL is refused at creation, a LAN endpoint is created.
func TestCATS31_F7_CreateStorageRootRefusesS3MetadataURL(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	require.NoError(t, filesystem.RegisterS3Protocol())
	defer filesystem.UnregisterProtocol("s3")
	h := newAPIHarness(t, false)
	for _, u := range []string{"http://169.254.169.254/", "http://127.0.0.1:9000", "http://[::ffff:169.254.169.254]/", "http://0177.0.0.1/", "ftp://192.168.1.2/"} {
		code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "s3-ssrf-" + u, "protocol": "s3", "url": u, "path": "bucket/prefix"})
		require.Equal(t, 400, code, "%s: %v", u, body)
	}
	var n int
	require.NoError(t, h.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	require.Equal(t, 0, n, "refused roots must not be stored")
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lan-minio", "protocol": "s3", "url": "http://192.168.1.20:9000", "path": "bucket/prefix", "username": "k", "password": "s"})
	require.Equal(t, 201, code, "LAN S3 root must still be created: %v", body)
}
