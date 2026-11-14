package services

// CAT-SECQ1-C review round 2: the approved class is enforced on the LIVE gowebdav path (TestConnection, ReadDir, the sync request path),
// observed by the number of requests that reach a fixture, not only through GetQuota.

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

func secq1cCountingDav(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"></d:multistatus>`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// F2: a PUBLIC approval refuses an allow-listed loopback destination on every request of the live path.
func TestSECQ1C_R2_PublicClassRefusedOnLiveDavPath(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	srv, hits := secq1cCountingDav(t)
	ep := secq1cStoredEndpoint(t, repo, root, srv.URL)
	require.NoError(t, repo.SetApprovedClass(svc.ctx, ep.ID, netguard.ClassPublic, database.ApprovedSourceOperator))

	c, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	require.Error(t, c.TestConnection(), "TestConnection (the live gowebdav path) refuses a private-class destination for a public approval")
	_, lerr := c.ListFiles("/")
	require.Error(t, lerr, "ListFiles (the sync listing path) refuses it too")
	require.Error(t, svc.testConnection(ep), "and so does the service's own connection test")
	require.Zero(t, atomic.LoadInt32(hits), "no request reached the private fixture")
}

// control: the same fixture is reached under a PRIVATE approval with the allow-list (so the refusal above is the class, not a dead fixture)
func TestSECQ1C_R2_PrivateClassReachesAllowListedFixture(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	srv, hits := secq1cCountingDav(t)
	ep := secq1cStoredEndpoint(t, repo, root, srv.URL)
	require.NoError(t, repo.SetApprovedClass(svc.ctx, ep.ID, netguard.ClassPrivate, database.ApprovedSourceOperator))
	c, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	require.NoError(t, c.TestConnection())
	require.NotZero(t, atomic.LoadInt32(hits))
}

// never looser than the strict transport: without the allow-list even a PRIVATE approval refuses loopback.
func TestSECQ1C_R2_PrivateClassStillNeedsTheAllowList(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	t.Setenv(netguard.AllowCIDRsEnv, "")
	srv, hits := secq1cCountingDav(t)
	ep := secq1cStoredEndpoint(t, repo, root, srv.URL)
	require.NoError(t, repo.SetApprovedClass(svc.ctx, ep.ID, netguard.ClassPrivate, database.ApprovedSourceOperator))
	c, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	require.Error(t, c.TestConnection())
	require.Zero(t, atomic.LoadInt32(hits))
}

// survivor M: a not-yet-stored endpoint (ID 0) gets a client for the default class and it is never cached.
func TestSECQ1C_R2_UnstoredEndpointClientIsNotCached(t *testing.T) {
	svc, _, _ := newGuardedSyncService(t)
	ep := &models.SyncEndpoint{ID: 0, URL: "https://example.com/dav"}
	c1, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	c2, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	require.NotSame(t, c1, c2)
	svc.webdavMu.Lock()
	_, cached := svc.webdavClients[0]
	svc.webdavMu.Unlock()
	require.False(t, cached)
}
