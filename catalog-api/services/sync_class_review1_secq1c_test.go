package services

// CAT-SECQ1-C review round 1: unresolvable names, URL-change supersede, the stored class reaching the live client, cache eviction and
// fail-closed class lookup. Real SQLite (the sync test DB), real WebDAV fixture listeners.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

const secq1cUnresolvable = "http://nx-secq1c.invalid/dav" // RFC 2606 .invalid never resolves

func secq1cQuotaServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href><D:propstat><D:prop><D:quota-used-bytes>5</D:quota-used-bytes><D:quota-available-bytes>10</D:quota-available-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func secq1cStoredEndpoint(t *testing.T, repo interface {
	CreateEndpoint(*models.SyncEndpoint) (int, error)
}, root, url string) *models.SyncEndpoint {
	t.Helper()
	now := time.Now()
	ep := &models.SyncEndpoint{UserID: 1, Name: "e", Type: models.SyncTypeWebDAV, URL: url, SyncDirection: models.SyncDirectionDownload,
		LocalPath: root, Status: models.SyncStatusActive, CreatedAt: now, UpdatedAt: now}
	id, err := repo.CreateEndpoint(ep)
	require.NoError(t, err)
	ep.ID = id
	return ep
}

// M2 (planning): an address that does not resolve is networked and plans as PRIVATE, never public.
func TestSECQ1C_R1_UnresolvablePlansPrivate(t *testing.T) {
	plan, err := planSyncClass(&models.SyncEndpoint{Type: models.SyncTypeWebDAV, URL: secq1cUnresolvable})
	require.NoError(t, err)
	require.True(t, plan.networked)
	require.Equal(t, netguard.ClassPrivate, plan.class)
}

// M2 (create): an unresolvable destination needs the operator's confirmation before anything is stored.
func TestSECQ1C_R1_CreateUnresolvableNeedsConfirmation(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	_, err := svc.CreateSyncEndpoint(1, &models.SyncEndpoint{Name: "u", Type: models.SyncTypeWebDAV, URL: secq1cUnresolvable,
		SyncDirection: models.SyncDirectionDownload, LocalPath: root})
	require.ErrorIs(t, err, ErrPrivateConfirmationRequired)
	eps, err := repo.GetUserEndpoints(1)
	require.NoError(t, err)
	require.Empty(t, eps, "nothing is stored for an unconfirmed destination")
}

// M2 (approveCreated): when the destination cannot be classified at approval time, the stored row stands only for a confirmed endpoint.
func TestSECQ1C_R1_ApproveCreatedUnresolvableBranch(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	plan := syncClassPlan{networked: true, class: netguard.ClassPrivate}

	ep := secq1cStoredEndpoint(t, repo, root, secq1cUnresolvable)
	require.ErrorIs(t, svc.approveCreated(ep, plan), ErrPrivateConfirmationRequired, "unconfirmed")
	_, err := repo.GetEndpoint(ep.ID)
	require.Error(t, err, "the unconfirmed row is removed again")

	ep2 := secq1cStoredEndpoint(t, repo, root, secq1cUnresolvable)
	ep2.ConfirmPrivateNetwork = true
	require.NoError(t, svc.approveCreated(ep2, plan))
	cls, src, err := repo.GetApprovedClass(svc.ctx, ep2.ID)
	require.NoError(t, err)
	require.Equal(t, netguard.ClassPrivate, cls)
	require.Equal(t, database.ApprovedSourceOperator, src)

	// a destination planned public that no longer classifies public is refused even when confirmed (rebinding)
	ep3 := secq1cStoredEndpoint(t, repo, root, secq1cUnresolvable)
	ep3.ConfirmPrivateNetwork = true
	require.ErrorIs(t, svc.approveCreated(ep3, syncClassPlan{networked: true, class: netguard.ClassPublic}), ErrPrivateConfirmationRequired)
}

// M5: an operator-set private approval is superseded when the URL moves to a public destination.
func TestSECQ1C_R1_URLChangeSupersedesOperatorClass(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	ep := secq1cStoredEndpoint(t, repo, root, "http://old.example/dav")
	require.NoError(t, repo.SetApprovedClass(svc.ctx, ep.ID, netguard.ClassPrivate, database.ApprovedSourceOperator))

	require.NoError(t, svc.storeURLChangeClass(ep, syncClassPlan{networked: true, class: netguard.ClassPublic}))
	cls, src, err := repo.GetApprovedClass(svc.ctx, ep.ID)
	require.NoError(t, err)
	require.Equal(t, netguard.ClassPublic, cls, "the operator decision was about the old URL")
	require.Equal(t, database.ApprovedSourceAuto, src)
	require.Equal(t, "public", ep.ApprovedClass)
}

// M6: the class STORED for the endpoint reaches the client getWebDAVClient returns (observed through GetQuota against a loopback fixture,
// reachable only for a private approval), and a changed class replaces the cached client.
func TestSECQ1C_R1_GetWebDAVClientUsesStoredClass(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	ep := secq1cStoredEndpoint(t, repo, root, secq1cQuotaServer(t))

	require.NoError(t, repo.SetApprovedClass(svc.ctx, ep.ID, netguard.ClassPrivate, database.ApprovedSourceOperator))
	c, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	_, err = c.GetQuota()
	require.NoError(t, err, "private approval reaches the allow-listed loopback fixture")

	require.NoError(t, repo.SetApprovedClass(svc.ctx, ep.ID, netguard.ClassPublic, database.ApprovedSourceOperator))
	c2, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	require.NotSame(t, c, c2, "a client cached under the previous class is not reused")
	_, err = c2.GetQuota()
	require.Error(t, err, "public approval refuses the private-class destination")
}

// Finding 4a: the class write evicts the cached client.
func TestSECQ1C_R1_ClassWriteEvictsCachedClient(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	ep := secq1cStoredEndpoint(t, repo, root, "http://old.example/dav")
	_, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)
	svc.webdavMu.Lock()
	_, cached := svc.webdavClients[ep.ID]
	svc.webdavMu.Unlock()
	require.True(t, cached, "control: the client is cached")

	require.NoError(t, svc.storeURLChangeClass(ep, syncClassPlan{networked: true, class: netguard.ClassPublic}))
	svc.webdavMu.Lock()
	_, cached = svc.webdavClients[ep.ID]
	svc.webdavMu.Unlock()
	require.False(t, cached, "the cached client does not outlive the class write")
}

// Finding 4b: a stored endpoint whose class cannot be read gets no client (fail closed), and a cached one is dropped.
func TestSECQ1C_R1_UnreadableClassFailsClosed(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	ep := secq1cStoredEndpoint(t, repo, root, "http://old.example/dav")
	_, err := svc.getWebDAVClient(ep)
	require.NoError(t, err)

	require.NoError(t, repo.DeleteEndpoint(ep.ID)) // the class lookup now fails (not found)
	c, err := svc.getWebDAVClient(ep)
	require.Error(t, err)
	require.Nil(t, c)
	svc.webdavMu.Lock()
	_, cached := svc.webdavClients[ep.ID]
	svc.webdavMu.Unlock()
	require.False(t, cached)

	// a not-yet-stored endpoint (ID 0, create-time connection test) still gets a default-private client that is never cached
	c0, err := svc.getWebDAVClient(&models.SyncEndpoint{URL: "http://new.example/dav"})
	require.NoError(t, err)
	require.NotNil(t, c0)
}
