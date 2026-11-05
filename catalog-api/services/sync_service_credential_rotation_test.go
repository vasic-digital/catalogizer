package services

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R4-1: a WebDAV client cached for an endpoint must be rebuilt when the USERNAME (or password)
// changes, otherwise requests keep going out under the previous account. The fixture server
// records the basic-auth pair of every request it really receives.
func TestSyncService_WebDAVClientIsRebuiltOnUsernameOrPasswordRotation(t *testing.T) {
	svc, repo, root := newGuardedSyncService(t)
	t.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8")

	var mu sync.Mutex
	var seen [][2]string
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok { // like a real WebDAV server: challenge first, the client answers with its credentials
			w.Header().Set("WWW-Authenticate", `Basic realm="dav"`)
			w.WriteHeader(401)
			return
		}
		mu.Lock()
		seen = append(seen, [2]string{u, p})
		mu.Unlock()
		w.WriteHeader(207)
	})}}
	srv.Start()
	t.Cleanup(srv.Close)
	last := func() [2]string {
		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, seen, "the fixture server saw no request")
		return seen[len(seen)-1]
	}

	now := time.Now()
	id, err := repo.CreateEndpoint(&models.SyncEndpoint{
		UserID: 1, Name: "rot", Type: models.SyncTypeWebDAV, URL: srv.URL, Username: "alice", Password: "pw-one",
		SyncDirection: models.SyncDirectionDownload, LocalPath: root,
		Status: models.SyncStatusActive, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	ep, err := repo.GetEndpoint(id)
	require.NoError(t, err)
	require.NoError(t, svc.testConnection(ep))
	require.Equal(t, [2]string{"alice", "pw-one"}, last(), "control: the first account is used")

	// username changes, password stays
	ep.Username = "bob"
	require.NoError(t, svc.testConnection(ep))
	assert.Equal(t, [2]string{"bob", "pw-one"}, last(), "after a username rotation requests must carry the new username")

	// password changes, username stays
	ep.Password = "pw-two"
	require.NoError(t, svc.testConnection(ep))
	assert.Equal(t, [2]string{"bob", "pw-two"}, last(), "after a password rotation requests must carry the new password")
}
