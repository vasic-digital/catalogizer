package services

// CAT-SECQ1-C follow-ups SECQ1C-M2 / M3: the class planned for a destination is the class its connection test runs under, on update AND on
// create. classifyEndpointForPlan is replaced to make a destination plan as PUBLIC while the fixture is an allow-listed loopback listener:
// a public-class test must be refused by the dial (0 hits), a private-class test would reach it. Unit class for the seam; the transport,
// the repository and the service are real.

import (
	"context"
	"testing"

	"catalogizer/internal/netguard"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

func secq1cFUForceClass(t *testing.T, c netguard.Class) {
	t.Helper()
	old := classifyEndpointForPlan
	classifyEndpointForPlan = func(context.Context, string, string) (netguard.Class, error) { return c, nil }
	t.Cleanup(func() { classifyEndpointForPlan = old })
}

// M2: UpdateEndpoint tests the NEW destination under plan.class.
func TestSECQ1CFU_M2_UpdateTestsUnderPlannedClass(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	oldURL, _ := countingListener(t)
	newURL, hits := countingListener(t)
	ep := secq1cStoredEndpoint(t, repo, root, oldURL)

	secq1cFUForceClass(t, netguard.ClassPublic)
	_, err := svc.UpdateEndpoint(ep.ID, 1, &models.UpdateSyncEndpointRequest{URL: newURL})
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection test failed", "a destination planned public is tested under the public class")
	require.Zero(t, *hits, "the allow-listed loopback fixture was never reached")

	// control: planned private (confirmed), the same fixture is reached
	secq1cFUForceClass(t, netguard.ClassPrivate)
	_, err = svc.UpdateEndpoint(ep.ID, 1, &models.UpdateSyncEndpointRequest{URL: newURL, ConfirmPrivateNetwork: true})
	require.NoError(t, err)
	require.NotZero(t, *hits)
}

// M3: CreateSyncEndpoint tests a WebDAV destination under plan.class too (before the fix it used testConnection: ID 0, private default).
func TestSECQ1CFU_M3_CreateTestsUnderPlannedClass(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	url, hits := countingListener(t)
	mk := func() *models.SyncEndpoint {
		return &models.SyncEndpoint{Name: "c", Type: models.SyncTypeWebDAV, URL: url, SyncDirection: models.SyncDirectionDownload, LocalPath: root}
	}

	secq1cFUForceClass(t, netguard.ClassPublic)
	_, err := svc.CreateSyncEndpoint(1, mk())
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection test failed", "planned public: tested under the public class")
	require.Zero(t, *hits, "the allow-listed loopback fixture was never reached")
	eps, gerr := repo.GetUserEndpoints(1)
	require.NoError(t, gerr)
	require.Empty(t, eps, "nothing stored")

	// control: planned private and confirmed reaches the same fixture
	secq1cFUForceClass(t, netguard.ClassPrivate)
	e := mk()
	e.ConfirmPrivateNetwork = true
	_, err = svc.CreateSyncEndpoint(1, e)
	require.NoError(t, err)
	require.NotZero(t, *hits)
}
