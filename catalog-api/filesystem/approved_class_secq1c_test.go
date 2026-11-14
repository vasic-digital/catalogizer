package filesystem

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/internal/netguard"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

// CAT-SECQ1-C: the stored approved class reaches the WebDAV / S3 operator transport. A loopback fixture server is allow-listed (class
// private, like a LAN NAS); a root approved for "public" must refuse it, a root approved for "private" (or with no stored class) reaches it.
func davLoopback(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusMultiStatus) }))
	t.Cleanup(srv.Close)
	return srv
}

func TestSECQ1C_SettingsFromRootCarriesApprovedClass(t *testing.T) {
	u, cls := "https://nas.example/dav", "public"
	for _, proto := range []string{"webdav"} {
		root := &models.StorageRoot{Protocol: proto, URL: &u, ApprovedClass: cls}
		s, err := SettingsFromRoot(root, nil)
		require.NoError(t, err)
		require.Equal(t, "public", s[KeyApprovedClass])
		require.NoError(t, ValidateSettings(proto, s))
	}
	root := &models.StorageRoot{Protocol: "webdav", URL: &u}
	s, err := SettingsFromRoot(root, nil)
	require.NoError(t, err)
	_, has := s[KeyApprovedClass]
	require.False(t, has, "a root loaded without a class carries no key (default private)")
}

func TestSECQ1C_WebDAVClientPinnedToApprovedClass(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := davLoopback(t)
	mk := func(cls string) FileSystemClient {
		st := map[string]interface{}{KeyURL: srv.URL}
		if cls != "" {
			st[KeyApprovedClass] = cls
		}
		c, err := NewDefaultClientFactory().CreateClient(&StorageConfig{ID: "x", Name: "x", Protocol: "webdav", Settings: st})
		require.NoError(t, err, cls)
		return c
	}
	require.NoError(t, mk("private").Connect(context.Background()), "private approval reaches an allow-listed LAN address")
	require.NoError(t, mk("").Connect(context.Background()), "no stored class: default private")
	err := mk("public").Connect(context.Background())
	require.Error(t, err, "a public approval must refuse a private-class destination")
}

func TestSECQ1C_BadApprovedClassRefused(t *testing.T) {
	for _, bad := range []string{"never", "PUBLIC ", "x", "internal"} {
		_, err := NewDefaultClientFactory().CreateClient(&StorageConfig{ID: "x", Name: "x", Protocol: "webdav",
			Settings: map[string]interface{}{KeyURL: "https://nas.example/dav", KeyApprovedClass: bad}})
		require.Error(t, err, bad)
	}
}

func TestSECQ1C_S3ClientPinnedToApprovedClass(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := davLoopback(t)
	c, err := NewS3Client(&S3Config{Endpoint: srv.URL, Bucket: "b", PathStyle: true, ApprovedClass: netguard.ClassPublic})
	require.NoError(t, err)
	require.Error(t, c.Connect(context.Background()), "public approval refuses the private-class endpoint before any bucket call")
}
