package services

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// credCacheFixture starts a real WebDAV-like listener that accepts exactly one password
// (switchable) and a stored endpoint whose client is already cached with the old one.
func credCacheFixture(t *testing.T) (svc *SyncService, id int, setPassword func(string)) {
	t.Helper()
	svc, repo, root := newGuardedSyncService(t)
	var want atomic.Value
	want.Store("old-throwaway")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, p, ok := r.BasicAuth()
		if !ok || p != want.Load().(string) {
			w.Header().Set("WWW-Authenticate", `Basic realm="x"`)
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(207)
	})}}
	srv.Start()
	t.Cleanup(srv.Close)
	t.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8")
	now := time.Now()
	id, err = repo.CreateEndpoint(&models.SyncEndpoint{
		UserID: 1, Name: "cred", Type: models.SyncTypeWebDAV, URL: srv.URL, Username: "u", Password: "old-throwaway",
		SyncDirection: models.SyncDirectionDownload, LocalPath: root,
		Status: models.SyncStatusActive, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	ep, err := repo.GetEndpoint(id)
	require.NoError(t, err)
	require.NoError(t, svc.testConnection(ep)) // caches a client holding the old password
	return svc, id, func(p string) { want.Store(p) }
}

// A credentials-only update is tested with the NEW credentials, not the cached client.
func TestSyncService_CredentialOnlyUpdateUsesNewCredentials(t *testing.T) {
	svc, id, setPassword := credCacheFixture(t)
	setPassword("new-throwaway") // server-side rotation
	_, err := svc.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{Password: "new-throwaway"})
	require.NoError(t, err, "updating to the correct new password must succeed")
}

// Reverse: a wrong new password must be refused (it is not tested against the old one).
func TestSyncService_WrongNewPasswordIsRefusedNotTestedWithOldCredentials(t *testing.T) {
	svc, id, _ := credCacheFixture(t) // server still accepts only the old password
	_, err := svc.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{Password: "typo-throwaway"})
	assert.Error(t, err, "a password the server rejects must not be stored")
}
