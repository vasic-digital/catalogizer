package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// SECQ-1/W5: the sync WebDAV client (gowebdav and GetQuota) must refuse loopback / metadata destinations.
func TestSECQ1_SyncWebDAVRefusesLoopbackAndMetadata(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	defer srv.Close()

	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32") // positive control
	require.NoError(t, NewWebDAVClient(srv.URL, "u", "p").TestConnection())
	require.Greater(t, hits.Load(), int32(0))
	before := hits.Load()

	t.Setenv(netguard.AllowCIDRsEnv, "")
	c := NewWebDAVClient(srv.URL, "u", "p")
	err := c.TestConnection()
	require.Error(t, err)
	require.True(t, errors.Is(err, netguard.ErrBlocked), "TestConnection: %v", err)
	_, err = c.GetQuota()
	require.Error(t, err)
	require.True(t, errors.Is(err, netguard.ErrBlocked), "GetQuota: %v", err)
	require.Equal(t, before, hits.Load(), "refused clients must not reach the server")

	m := NewWebDAVClient("http://169.254.169.254/", "u", "p")
	require.True(t, errors.Is(m.TestConnection(), netguard.ErrBlocked))
}
