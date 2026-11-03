package services

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/netguard"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGuardedSyncService(t *testing.T) (*SyncService, *repository.SyncRepository, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("CATALOGIZER_SYNC_ROOTS", root)
	t.Setenv("CATALOGIZER_READ_ROOTS", root)
	t.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "")
	db, cleanup := newSyncTestDB(t)
	t.Cleanup(cleanup)
	repo := repository.NewSyncRepository(db)
	return NewSyncService(repo, nil, nil), repo, root
}

func countingListener(t *testing.T) (string, *int64) {
	t.Helper()
	var hits int64
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(207)
	})}}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.URL, &hits
}

// F-R2: an endpoint stored before the create-time validation existed (or written
// straight to the database) must still be refused at dial time, and the listener must
// see no connection (kills the mutation that drops SetTransport(netguard.Transport())).
func TestSyncService_StoredLoopbackEndpointIsBlockedAtDialTime(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	url, hits := countingListener(t)
	now := time.Now()
	id, err := repo.CreateEndpoint(&models.SyncEndpoint{
		UserID: 1, Name: "stored", Type: models.SyncTypeWebDAV, URL: url,
		SyncDirection: models.SyncDirectionDownload, LocalPath: root,
		Status: models.SyncStatusActive, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	ep, err := repo.GetEndpoint(id)
	require.NoError(t, err)

	err = svc.testConnection(ep)
	require.Error(t, err)
	assert.True(t, errors.Is(err, netguard.ErrBlocked) || strings.Contains(err.Error(), netguard.ErrBlocked.Error()),
		"testConnection must fail with the netguard refusal, got: %v", err)
	assert.EqualValues(t, 0, atomic.LoadInt64(hits), "the loopback listener must receive zero connections")

	// StartSync reaches the same client; the session it creates must not connect either.
	_, _ = svc.StartSync(id, 1)
	svc.Close()
	assert.EqualValues(t, 0, atomic.LoadInt64(hits), "StartSync must not connect to a blocked destination")
}

// M10: after the URL changes, the cached client for the old URL must not be reused.
func TestSyncService_UpdateEndpointEvictsClientForOldURL(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	urlA, hitsA := countingListener(t)
	urlB, hitsB := countingListener(t)
	t.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8")
	now := time.Now()
	id, err := repo.CreateEndpoint(&models.SyncEndpoint{
		UserID: 1, Name: "ev", Type: models.SyncTypeWebDAV, URL: urlA,
		SyncDirection: models.SyncDirectionDownload, LocalPath: root,
		Status: models.SyncStatusActive, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	ep, err := repo.GetEndpoint(id)
	require.NoError(t, err)
	require.NoError(t, svc.testConnection(ep)) // caches a client for A
	require.Greater(t, atomic.LoadInt64(hitsA), int64(0))

	_, err = svc.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{URL: urlB})
	require.NoError(t, err)
	beforeA := atomic.LoadInt64(hitsA)
	beforeB := atomic.LoadInt64(hitsB)
	require.Greater(t, beforeB, int64(0), "the new URL must have been contacted")

	stored, err := repo.GetEndpoint(id)
	require.NoError(t, err)
	require.Equal(t, urlB, stored.URL)
	require.NoError(t, svc.testConnection(stored))
	assert.Equal(t, beforeA, atomic.LoadInt64(hitsA), "the old URL must not be contacted again")
	assert.Greater(t, atomic.LoadInt64(hitsB), beforeB, "the new URL must be contacted")
}

// A rejected update must not leave a client for the rejected URL cached under the
// endpoint's id (the stored URL is unchanged).
func TestSyncService_FailedUpdateDoesNotCacheRejectedURLClient(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	urlA, hitsA := countingListener(t)
	t.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8")
	now := time.Now()
	id, err := repo.CreateEndpoint(&models.SyncEndpoint{
		UserID: 1, Name: "rej", Type: models.SyncTypeWebDAV, URL: urlA,
		SyncDirection: models.SyncDirectionDownload, LocalPath: root,
		Status: models.SyncStatusActive, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	// A URL that validates but is unreachable: the update fails at the connection test.
	_, err = svc.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{URL: "http://127.0.0.1:1/"})
	require.Error(t, err)

	stored, err := repo.GetEndpoint(id)
	require.NoError(t, err)
	require.Equal(t, urlA, stored.URL)
	before := atomic.LoadInt64(hitsA)
	require.NoError(t, svc.testConnection(stored))
	assert.Greater(t, atomic.LoadInt64(hitsA), before, "the stored URL must be used, not the rejected one")
}

// M3: only a string that STARTS with a bucket scheme is exempt from the network check.
func TestBucketURL_PrefixOnly(t *testing.T) {
	for _, ok := range []string{"s3://b", "GS://b", "gcs://b/x"} {
		assert.True(t, bucketURL(ok), ok)
	}
	for _, bad := range []string{"http://127.0.0.1/?u=s3://b", "https://127.0.0.1/gs://b", "http://x/gcs://"} {
		assert.False(t, bucketURL(bad), bad)
	}
}

func TestValidateSyncEndpoint_CloudStorageHTTPLoopbackWithBucketSubstringIsRefused(t *testing.T) {
	svc, _, root := newGuardedSyncService(t)
	err := svc.validateSyncEndpoint(&models.SyncEndpoint{
		Name: "c", Type: models.SyncTypeCloudStorage, URL: "http://127.0.0.1:9/?u=s3://bucket",
		SyncDirection: models.SyncDirectionUpload, LocalPath: root,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, netguard.ErrBlocked)
}

// F-R5: the client cache is shared between request goroutines and sync workers.
func TestSyncService_WebDAVClientCacheIsRaceFree(t *testing.T) {
	svc := NewSyncService(nil, nil, nil)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 1; i <= 200; i++ {
				ep := &models.SyncEndpoint{ID: i, URL: "https://example.invalid/"}
				_, _ = svc.getWebDAVClient(ep)
				svc.evictWebDAVClient(i)
			}
		}(g)
	}
	wg.Wait()
}
