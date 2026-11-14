package services

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// CAT-SECQ1-C: the sync WebDAV client is built for the stored class of its endpoint. A loopback fixture (allow-listed, class private)
// is reachable for a private approval and refused for a public one, on the connection test and on GetQuota alike.
func TestSECQ1C_SyncWebDAVClientPinnedToApprovedClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype><D:quota-used-bytes>5</D:quota-used-bytes><D:quota-available-bytes>10</D:quota-available-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	defer srv.Close()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")

	priv := NewWebDAVClientWithClass(srv.URL, "u", "p", netguard.ClassPrivate)
	require.NoError(t, priv.TestConnection())
	_, err := priv.GetQuota()
	require.NoError(t, err)

	pub := NewWebDAVClientWithClass(srv.URL, "u", "p", netguard.ClassPublic)
	require.Error(t, pub.TestConnection(), "public approval refuses the private-class destination")
	_, err = pub.GetQuota()
	require.Error(t, err)

	// the unset zero value is the default, private
	def := NewWebDAVClientWithClass(srv.URL, "u", "p", netguard.ClassNever)
	require.NoError(t, def.TestConnection())
}
